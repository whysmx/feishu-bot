# CLAUDE.md

此文件为 Claude Code (claude.ai/code) 使用此代码库中的代码提供指导。

## 项目概述

**飞书 Claude CLI 流式对话机器人** - 基于飞书平台和 Claude CLI 的智能对话机器人，支持流式输出和打字机效果。

这是一个 Go 项目，通过集成本地 Claude CLI 和飞书 CardKit 2.0，实现实时流式对话功能。

## 当前状态

✅ **所有核心功能已完成**：
- Claude CLI 进程管理和 stream-json 解析
- CardKit 流式更新（限流 10 QPS）
- 飞书 WebSocket 长连接
- 直接消息触发流式对话
- 打字机效果
- **P2P（单聊）和群聊功能正常**
- **错误 230002 已修复**

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
- `cardkit:card:write` - 创建与更新卡片

### 事件订阅
- 方式：WebSocket 长连接
- 事件：`im.message.receive_v1`

## 项目结构

```
feishu-bot/
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
│   │   ├── cardkit_updater.go      # CardKit 流式更新
│   │   └── handler.go              # 流式对话处理器
│   ├── command/                    # 命令处理模块
│   ├── notification/               # 通知服务
│   └── session/                    # 会话管理
├── configs/                        # 配置文件目录
├── .env                            # 环境变量（敏感）
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

2. **CardKit Updater** (`internal/claude/cardkit_updater.go`)
   - 创建卡片实体
   - 流式更新卡片内容
   - 限流控制（100ms 间隔 = 10 QPS）

3. **Message Handler** (`internal/bot/handlers/message.go`)
   - 处理飞书消息
   - 路由用户消息到流式对话处理器
   - 集成 Claude CLI 和 CardKit

### 数据流

```
用户消息 (@机器人 问题)
    ↓
飞书 WebSocket 接收
    ↓
Message Handler 处理
    ↓
Claude Handler → 创建 CardKit 卡片
    ↓
启动 Claude CLI (cc1 命令)
    ↓
解析 stream-json → 提取文本增量
    ↓
CardKitUpdater 流式更新卡片（打字机效果）
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

### 3. CardKit 两步法

1. **创建卡片实体**：`POST /open-apis/cardkit/v1/cards`
2. **发送到聊天**：`POST /open-apis/im/v1/messages`
3. **流式更新**：`PUT /open-apis/cardkit/v1/cards/{id}/elements/{id}/content`

### 4. 限流控制

使用 `time.Ticker` 实现 100ms 间隔（10 QPS）：

```go
rateLimiter := time.NewTicker(100 * time.Millisecond)
<-rateLimiter.C  // 等待限流
```

## 安全注意事项

- ✅ App Secret 已移至 `.env` 文件
- ✅ `.env` 已加入 `.gitignore`
- ⚠️ 确保 `.env` 文件权限正确（chmod 600）
- ⚠️ 生产环境应使用环境变量或密钥管理服务

## 故障排查

### 错误 230002: "Bot/User can NOT be out of the chat"

**问题描述**：
机器人在 P2P（单聊）场景下发送消息时，飞书 API 返回错误：
```json
{
  "code": 230002,
  "msg": "Bot/User can NOT be out of the chat."
}
```

**根本原因**：
在 `internal/bot/handlers/message.go` 中，P2P 场景使用了错误的 `receive_id`：
- **错误做法**：使用 `FEISHU_TEST_CHAT_ID` 环境变量（群聊 ID）
- **正确做法**：使用用户的 `open_id`

**修复代码**：

```go
// HandleP2PMessage 处理单聊消息 (message.go:78-82)
func (mh *MessageHandler) HandleP2PMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
    // ... 安全检查代码 ...

    openID := *event.Event.Sender.SenderId.OpenId

    // ✅ P2P 场景固定使用 open_id，避免卡片发送到非成员 chat 导致 230002
    receiveID := openID
    receiveIDType := "open_id"
    mh.logger.Printf("✅✅✅ P2P MODE: Using open_id=%s", openID)

    return mh.processMessage(openID, userID, receiveID, receiveIDType, content)
}
```

**关键变更**（`message.go:427-431`）：
- **移除了**：`receiveID = os.Getenv("FEISHU_TEST_CHAT_ID")` 的回退逻辑
- **添加了**：空 `receiveID` 检查，提前返回错误提示

**receive_id_type 对比**：

| 场景 | receive_id_type | receive_id 值 | 示例 |
|------|----------------|---------------|------|
| **单聊 (P2P)** | `open_id` | 用户的 open_id | `ou_xxx` |
| **群聊** | `chat_id` | 群聊的 chat_id | `oc_xxx` |

**调试步骤**：

1. **检查日志中的模式标记**：
   ```bash
   grep "✅✅✅ P2P MODE" /tmp/feishu-bot-latest.log
   ```

2. **验证 API 调用参数**：
   ```bash
   grep "receive_id_type=open_id" /tmp/feishu-bot-latest.log
   ```

3. **检查飞书平台事件日志**：
   - 访问：https://open.feishu.cn/app/cli_a9dc39c0c2b8dbc8/logs
   - 切换到"事件日志检索"
   - 查看 `im.message.receive_v1` 事件推送状态

**常见陷阱**：

❌ **错误 1**：多个机器人实例同时运行
- 症状：代码修改后没有生效
- 诊断：`ps aux | grep "\./bot"`
- 解决：停止所有实例后重新编译启动

❌ **错误 2**：Go 构建缓存
- 症状：修改代码后行为不变
- 解决：`go build -a` 强制重新构建

❌ **错误 3**：旧实例 WebSocket 连接仍活跃
- 症状：新实例收不到事件
- 解决：`pkill -9 -f "./bot"` 完全停止旧进程

**验证修复成功**：

```bash
# 1. 重新编译并启动机器人
go build -a -o bot cmd/bot/main.go
nohup ./bot > /tmp/feishu-bot-latest.log 2>&1 &

# 2. 等待 5-10 秒让飞书平台识别新连接

# 3. 发送测试消息（单聊）
# 预期：机器人回复流式卡片，不报错

# 4. 检查日志
tail -f /tmp/feishu-bot-latest.log | grep "✅✅✅ P2P MODE"
```

**成功的日志输出**：
```
[MessageHandler] ✅✅✅ P2P MODE: Using open_id=ou_586ea0f2017246e8434e7b04ef739a9c
[DEBUG] Sending card: receive_id=ou_xxx receive_id_type=open_id
Card created and sent: card_id=7590672032577506233
```

### 其他常见问题

**Q: 修改代码后机器人行为没有变化**
A: 可能的原因：
1. Go 构建缓存 → 使用 `go build -a` 强制重新构建
2. 旧的机器人进程仍在运行 → `pkill -9 -f "./bot"`
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
pkill -9 -f "./bot"

# 2. 验证没有残留进程
ps aux | grep "\./bot" | grep -v grep

# 3. 强制重新编译（清除构建缓存）
go build -a -o bot cmd/bot/main.go

# 4. 启动新实例
nohup ./bot > /tmp/feishu-bot-latest.log 2>&1 &

# 5. 记录新进程 PID
echo "Bot PID: $!"

# 6. 等待 5-10 秒让飞书平台识别新连接

# 7. 验证 WebSocket 连接成功
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
- [CardKit 2.0 文档](https://open.feishu.cn/document/common-capabilities/message-card/card-components)
- [飞书 Go SDK](https://github.com/larksuite/oapi-sdk-go)
- [Claude CLI 文档](https://docs.anthropic.com/claude-cli/overview)
