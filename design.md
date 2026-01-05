# 飞书 × Claude CLI 实时对话系统概要设计（当前实现）

本文档描述当前代码实现：通过飞书长连接接收消息，调用本地 Claude CLI，按“文本分段”方式回传结果。

## 1. 设计目标

- 以飞书为唯一入口，支持群聊/单聊对话
- 调用本地 Claude CLI（`stream-json`），实现流式输出体验
- 通过分段文本消息减少 API 调用与超长消息风险
- 保持实现简单、可部署、可追踪

## 2. 核心流程概览

```
用户消息 -> Feishu WebSocket -> MessageHandler
 -> Claude CLI (stream-json) -> StreamingTextHandler -> 飞书文本消息
```

## 3. 组件与职责

### 3.1 事件接收与分发（`cmd/bot/main.go`）

- 使用飞书 SDK 建立 WebSocket 长连接
- 订阅 `im.message.receive_v1`
- 对消息事件进行异步处理（避免阻塞 ACK）

### 3.2 消息处理（`internal/bot/handlers/message.go`）

- 过滤非文本消息
- 去重（30 分钟窗口）
- 按 `chat_type` 分流：
  - `p2p`：使用 `open_id` 作为 `receive_id`
  - `group/private`：使用 `chat_id` 作为 `receive_id`
- 群聊命令：仅当 `@机器人` 且命令匹配 `ls/bind/help` 才执行

### 3.3 Claude CLI 管理（`internal/claude/manager.go`）

- 启动 `claude` 命令（`--output-format stream-json`）
- 解析 `stream_event`/`assistant` 输出
- 维护 `session_id`，支持 `--resume`
- 发生“无法 resume”时自动降级重试

### 3.4 流式分段发送（`internal/claude/streaming_text_handler.go`）

- 处理 Claude 的全量文本增量回调
- 计算新增内容，累积到缓冲区
- 按时间和大小策略分段发送文本消息

### 3.5 绑定配置（`internal/config/chat_config.go`）

- 配置文件：`configs/chat_config.json`
- 保存群聊 `chat_id -> project_path` 映射
- `BASE_DIR` 决定 `ls/bind` 的扫描目录

## 4. 消息处理行为

### 4.1 P2P 单聊

- 使用 `open_id` 回复
- 会话按用户维持（`open_id/union_id` 映射到 Claude `session_id`）

### 4.2 群聊

- 使用 `chat_id` 回复
- 命令仅在 `@机器人` 且匹配命令词时执行
- **普通对话**：即使未 @，仍会转发给 Claude（是否能收到取决于飞书事件推送策略）

### 4.3 命令列表

- `ls`：列出基础目录下的项目
- `bind <序号>`：绑定群聊到指定项目路径
- `help`：显示帮助与当前绑定状态

## 5. 流式分段策略

分段发送由 `internal/utils/timeout.go` 统一配置：

- `StreamIdleTimeout`：空闲多久发送一次缓冲内容（默认 8s）
- `StreamMaxDuration`：持续输出多久强制分段（默认 20s）
- `StreamMaxBufferSize`：缓冲区最大字符数（默认 30000）

触发任一条件就会发送一段文本消息；进程结束后会发送剩余内容。

## 6. 会话管理策略

- **P2P**：按用户维持会话，支持 `--resume`
- **群聊**：使用全局会话 ID（`global_group_session`），所有群聊共享上下文
  - 优点：实现简单
  - 风险：不同群聊上下文混用

## 7. 配置与运行时数据

- `.env`：运行配置，必需项见 `.env.example`
- `configs/chat_config.json`：群聊绑定配置（首次运行自动生成）
- 日志默认写入 `/tmp/feishu-bot-latest.log`（可用 `LOG_FILE` 覆盖）

## 8. 约束与注意事项

- 文本消息受飞书大小限制，分段策略用于规避超长消息
- Claude CLI 使用 `--dangerously-skip-permissions`，具备本地读写/命令能力
- 群聊是否能收到未 @ 消息，取决于飞书事件推送权限与配置

## 9. 可扩展方向

- 群聊按 `chat_id` 独立会话
- `bind` 支持路径输入与校验
- 命令别名/中文指令
- 更精确的 @ 机器人判定（检查 mention ID）
