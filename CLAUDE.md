# CLAUDE.md

此文件为 Claude Code (claude.ai/code) 使用此代码库中的代码提供指导。

## 项目概述

**飞书 Claude CLI 流式对话机器人** - 基于飞书平台和 Claude CLI 的智能对话机器人，支持流式文本输出和打字机效果。

这是一个 Go 项目，通过集成本地 Claude CLI 和飞书消息 API，实现实时流式对话功能。

## 当前状态

✅ **所有核心功能已完成**：
- Claude CLI 进程管理和 stream-json 解析
- 飞书 WebSocket 长连接
- 流式文本分段发送（智能缓冲）
- 直接消息触发流式对话
- 打字机效果
- **P2P（单聊）和群聊功能正常**

## 配置

### 飞书应用配置
应用配置存储在 `.env` 文件中：
- 应用 ID：`cli_a9dc39c0c2b8dbc8`
- 应用密钥：存储在 `.env` 中（已从配置文件移除）
- 基础域名：`https://open.feishu.cn`

### 权限要求
- `im:message` - 获取与发送消息
- `im:message:group_at_msg` - 群聊 @消息
- `im:chat` - 访问群聊信息

### 事件订阅
- 方式：WebSocket 长连接
- 事件：`im.message.receive_v1`

## 项目结构

```
./
├── cmd/
│   └── bot/
│       └── main.go                 # 主程序入口
├── internal/
│   ├── bot/
│   │   ├── client/
│   │   │   └── feishu.go           # 飞书客户端（含 token 管理）
│   │   └── handlers/
│   │       └── message.go          # 消息处理器
│   ├── claude/
│   │   ├── manager.go              # Claude CLI 进程管理
│   │   └── streaming_text_handler.go  # 流式文本处理
│   └── utils/
│       ├── paths.go               # 路径工具
│       └── timeout.go             # 超时配置
├── scripts/                        # 部署脚本
│   ├── start-bot.sh
│   ├── stop-bot.sh
│   └── restart-bot.sh
├── .env                            # 环境变量（敏感）
├── .env.example                    # 配置示例
├── go.mod                          # Go 模块配置
└── Makefile                        # 构建脚本
```

## 开发设置

### 运行项目

```bash
# 安装依赖
go mod download

# 运行机器人
go run cmd/bot/main.go

# 或后台运行
go run cmd/bot/main.go > /tmp/feishu-bot.log 2>&1 &
```

### 查看日志

```bash
# 实时查看日志
tail -f /tmp/feishu-bot.log

# 检查 WebSocket 连接
grep "connected" /tmp/feishu-bot.log

# 搜索错误
grep "ERROR" /tmp/feishu-bot.log
```

## 架构设计

### 核心组件

1. **Claude CLI Manager** (`internal/claude/manager.go`)
   - 启动和管理 Claude CLI 进程
   - 解析 stream-json 输出
   - 提供文本增量回调
   - **立即发送机制**：每次文本增量都触发回调

2. **Streaming Text Handler** (`internal/claude/streaming_text_handler.go`)
   - **基于时间的智能分段**：优化 API 调用次数
   - 8 秒空闲超时：无新数据时发送缓冲区内容
   - 20 秒最大持续时间：长时间输出强制分段
   - 30000 字符缓冲上限：防止超过飞书 150KB 限制

3. **Message Handler** (`internal/bot/handlers/message.go`)
   - 处理飞书消息
   - 路由用户消息到流式对话处理器
   - 集成 Claude CLI

### 数据流

```
用户消息 (@机器人 问题)
    ↓
飞书 WebSocket 接收
    ↓
Message Handler 处理
    ↓
启动 Claude CLI (cc1 -p 命令)
    ↓
解析 stream-json → 提取文本增量
    ↓
StreamingTextHandler 智能分段缓冲
    ├─ 8 秒空闲超时
    ├─ 20 秒持续时间
    └─ 30000 字符缓冲上限
    ↓
飞书消息 API 发送（打字机效果）
```

## 关键技术点

### 1. Claude CLI 集成

使用本地 `cc1` 命令（Claude CLI 别名）：

```go
cmd := exec.Command("cc1",
    "-p",                                // 非交互模式
    "--output-format", "stream-json",    // 流式 JSON
    "--include-partial-messages",        // 包含部分消息
)
```

### 2. Stream-JSON 解析

解析 Claude CLI 输出的事件流：

```json
{"type": "stream_event", "event": {"type": "content_block_delta", "delta": {"text": "..."}}}
```

### 3. 智能分段策略

**StreamingTextHandler** 实现基于时间的智能分段，优化 API 调用：

**分段条件**（满足任一即发送）：
1. **空闲超时**：8 秒无新数据 → 发送缓冲区
2. **持续时间**：连续输出 20 秒 → 强制分段
3. **缓冲区上限**：累积 30000 字符 → 强制分段
4. **消息结束**：Claude 回复完成 → 发送剩余内容

**实现细节**：
```go
idleTimeout:   8 * time.Second  // 空闲超时（进一步减少API调用）
maxDuration:   20 * time.Second  // 最大持续时间
maxBufferSize: 30000             // 字符缓冲上限
```

**优化目标**：
- 减少 API 调用次数（避免频繁的小批量发送）
- 保持流式输出体验（8 秒响应速度）
- 防止超过飞书 150KB 消息大小限制

## 安全注意事项

- ✅ App Secret 已移至 `.env` 文件
- ✅ `.env` 已加入 `.gitignore`
- ⚠️ 确保 `.env` 文件权限正确（chmod 600）
- ⚠️ 生产环境应使用环境变量或密钥管理服务

## 故障排查

### 常见问题

**Q: 修改代码后机器人行为没有变化**
A: 可能的原因：
1. Go 构建缓存 → 使用 `go build -a` 强制重新构建
2. 旧的机器人进程仍在运行 → `./scripts/stop-bot.sh`
3. 飞书平台连接到旧实例 → 检查平台事件日志的时间戳

**Q: 机器人收不到消息**
A: 诊断步骤：
1. 检查 WebSocket 连接：`grep "connected" /tmp/feishu-bot-latest.log`
2. 检查平台事件推送：飞书开放平台 → 日志检索 → 事件日志检索
3. 检查是否有多个实例：`ps aux | grep "\./bot"`

**Q: 如何确认飞书平台推送了事件**
A:
1. 访问飞书开放平台日志检索页面
2. 切换到"事件日志检索"标签
3. 查询 `im.message.receive_v1` 事件
4. 查看推送状态（SUCCESS/FAIL）和错误信息

## 开发指南

### 重启机器人流程

**重要**：修改代码后，务必按照以下步骤重启机器人：

```bash
# 1. 停止所有机器人实例
./scripts/stop-bot.sh

# 2. 强制重新编译（清除构建缓存）
go build -a -o bin/bot cmd/bot/main.go

# 3. 启动新实例
./scripts/start-bot.sh

# 4. 记录新进程 PID
echo "Bot PID: $!"

# 5. 等待 5-10 秒让飞书平台识别新连接

# 6. 验证 WebSocket 连接成功
tail -20 /tmp/feishu-bot-latest.log | grep "connected"
```

### 添加新功能

1. 在 `internal/` 下创建新包
2. 在 `cmd/bot/main.go` 中注册
3. 更新 `README.md` 和此文档
4. **按照上述流程重启机器人**

### 测试

在飞书单聊或群聊中：
```
@机器人 你好
```

查看日志：
```bash
tail -f /tmp/feishu-bot-latest.log
```

## 相关资源

- [飞书开放平台](https://open.feishu.cn/document)
- [飞书 Go SDK](https://github.com/larksuite/oapi-sdk-go)
- [Claude CLI 文档](https://docs.anthropic.com/claude-cli/overview)
