# 飞书 × Claude Code CLI 实时流式对话系统概要设计（可跑通版）

## 1. 设计目标
构建一个中间件服务（Middleware），将飞书作为唯一的交互前端，通过 Webhook 或 WebSocket 接收用户指令，并在后端调用本地运行的 Claude Code CLI。系统利用 CardKit 2.0 的流式更新能力，将 Claude 的文本输出（Printing）实时推送到飞书卡片，实现“打字机”效果。不展示 Thinking。

## 2. 核心技术架构
### 2.1 架构分层
1. 交互层（Feishu Client）
   - 使用 CardKit 2.0 作为用户输入和流式输出界面。
   - 渲染 Markdown、代码块及交互按钮（如“继续”、“终止”）。
2. 网关与编排层（Middleware Server）
   - 消息接收：校验 X-Lark-Signature，解析用户消息。
   - 进程编排：管理 Claude CLI 子进程的生命周期（Spawn/Kill）。
   - 流式转换（Transcoder）：将 CLI 的 stream-json 输出转换为飞书 Stream Update API 请求。
   - 防抖缓冲（Throttling）：平衡 CLI 输出速度与飞书 API 频率限制。
3. 执行层（Agent Engine）
   - 运行在 Docker 容器中的 Claude Code CLI。
   - 通过 -p 参数运行在无头模式，执行文件读写与 Bash 命令。

### 2.2 关键数据流向
```
User --> Feishu --> Middleware --> Claude CLI
Claude CLI (stream-json) --> Middleware --> Stream Update API --> Feishu --> User
```

## 3. 关键模块详细设计
### 3.1 飞书卡片配置（CardKit 2.0）
为启用流式更新，卡片必须满足：
- `schema: "2.0"`
- `config.streaming_mode = true`
- `config.update_multi = true`
- 在卡片 Body 中定义一个 `element_id`（如 `reply_content`），用于局部流式更新

### 3.2 Claude CLI 进程编排
中间件通过子进程调用 Claude CLI，使用以下 Flag 组合输出机器可读的流式数据：
```
claude -p \
  --output-format stream-json \
  --include-partial-messages \
  --resume <FeishuSessionID>
```
关键参数：
- `-p / --print`：无头模式。
- `--output-format stream-json`：输出 NDJSON。
- `--include-partial-messages`：启用字符级增量输出。
- `--resume`：传入飞书用户 Session ID，以保持多轮上下文。

### 3.3 流式解析与缓冲策略（最小可跑通）
1. Parser（解析器）
   - 按行读取 CLI stdout，解析 JSON。
   - 仅处理 `content_block_delta` 里的 `text_delta`，将文本追加到输出缓冲区。
2. Buffer（缓冲与防抖）
   - 时间片合并：每 100ms~200ms 将 Buffer 合并为一次更新。
   - 维护 `sequence` 递增计数，保证卡片更新顺序。

### 3.4 会话与生命周期管理
- 会话映射：维护 `Feishu_OpenID -> Claude_Session_ID` 映射。
- 结束处理：CLI 输出 `message_stop` 或进程退出时，将卡片 `streaming_mode` 置为 `false`。

### 3.5 多用户扩展（多 Bot）
- 多用户通过部署多个 Bot 实现并行会话。
- 每个 Bot 独立维护会话映射与 Claude CLI 进程池。

## 4. 必须限制（最小集合）
为保证能跑通且不触发平台错误，需至少满足：
- Stream Update 接口限流：`1000 次/分钟、50 次/秒`
- 卡片体积限制：`≤ 30KB`
- `content` 长度：`1–100000` 字符
- `sequence` 必须严格递增
- `update_multi` 必须为 `true`

## 5. 缓冲与限流策略伪代码（最小可跑通）
下面伪代码只保证“能跑通”，在不触发秒/分钟限流前提下尽可能平滑输出：

```pseudo
state:
  buffer = ""
  seq = 0
  last_flush_at = 0
  sec_window = []   # 存放最近 1s 内的更新时间戳（毫秒）
  min_window = []   # 存放最近 60s 内的更新时间戳（毫秒）

on_cli_text_delta(text):
  buffer += text

flush_loop(every 100ms):
  if buffer is empty:
    return

  now = now_ms()
  # 清理过期窗口
  sec_window = [t for t in sec_window if now - t < 1000]
  min_window = [t for t in min_window if now - t < 60000]

  if len(sec_window) >= 50 or len(min_window) >= 1000:
    return  # 等下一个 tick 再试

  seq += 1
  content = buffer
  buffer = ""
  send_stream_update(content, seq)
  sec_window.append(now)
  min_window.append(now)

send_stream_update(content, seq):
  PUT /open-apis/cardkit/v1/cards/:card_id/elements/:element_id/content
  body: { "uuid": "<uuid>", "content": content, "sequence": seq }
```

## 6. Middleware 接口契约清单（最小可跑通）
1) Feishu Webhook（入站）
- 入口：`POST /webhook/feishu`
- 校验：`X-Lark-Signature` + `X-Lark-Request-Timestamp` + `X-Lark-Request-Nonce`
- 处理：解析用户输入，定位会话，启动或复用 Claude CLI 进程
- 返回：`200 OK`（快速响应）

2) Stream Update API（出站）
- 入口：`PUT https://open.feishu.cn/open-apis/cardkit/v1/cards/:card_id/elements/:element_id/content`
- 请求体：
  - `uuid`: 同卡片元素绑定 UUID
  - `content`: 全量文本（用于打字机效果）
  - `sequence`: 严格递增整数
- 约束：`update_multi = true`，`content <= 100000` 字符，卡片总大小 <= 30KB

3) Claude CLI 进程管理
- 启动：
  - `claude -p --output-format stream-json --include-partial-messages --resume <session>`
- 读取：
  - 按行解析 NDJSON
  - 只处理 `content_block_delta.text_delta`
- 终止：
  - 用户点击“终止”或会话超时则 kill 子进程

4) 会话映射
- `Feishu_OpenID -> Claude_Session_ID`
- 可存内存或 Redis（最小可跑通用内存）

## 7. 启动方式与处理思路（从 0 还是二开）
优先建议二开已有项目，先跑通流式链路；从 0 开始只在需要完全掌控架构或计划大规模重构时使用。

方案 A：二开现有项目（推荐）
- 基于 `tqtcloud/feishu-bot`：保留 Webhook 与会话管理，改造命令执行为 Claude CLI 的 `stream-json` 流式解析与缓冲更新。
- 核心改动点：
  - 命令执行替换为 `claude -p --output-format stream-json --include-partial-messages`
  - stdout 逐行解析 NDJSON，仅处理 `text_delta`
  - 引入 100ms~200ms 缓冲 + `sequence` 递增更新

方案 B：从 0 开始
- 自建轻量 Webhook 服务 + 子进程管理 + NDJSON 解析 + Stream Update API 调用。
- 适合需要定制路由、日志、扩展模块的场景，但开发周期更长。

## 8. 推荐参考项目与资源
### 8.1 基础脚手架
- tqtcloud/feishu-bot (Go)  
  https://github.com/tqtcloud/feishu-bot
- ClaudeAgentSDK (Elixir)  
  https://hexdocs.pm/claude_agent_sdk/ClaudeAgentSDK.Streaming.html

### 8.2 社区实现参考
- feishu-ai-chatbot-stream (Node.js)  
  https://github.com/87619639/feishu-ai-chatbot-stream

### 8.3 官方文档
- CardKit 2.0 变更日志  
  https://open.feishu.cn/changelog
- 飞书流式更新 API（Stream Update Text）  
  https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/cardkit-v1/card-element/content
  https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/cardkit-v1/card-element/content
- Claude Code CLI 参数手册  
  https://code.claude.com/docs/en/cli-reference
- Claude Code Headless Mode 指南  
  https://code.claude.com/docs/en/headless
