# 飞书 Claude CLI 流式对话机器人

基于飞书平台和 Claude CLI 的智能对话机器人，支持流式输出和打字机效果。

## 项目概述

通过集成 Claude CLI 和飞书 CardKit 2.0，实现实时流式对话的飞书机器人：

- 🤖 **Claude CLI 集成**：使用本地 Claude CLI 进行对话
- ⚡ **流式输出**：实时显示 AI 回复，打字机效果
- 💬 **CardKit 2.0**：使用飞书卡片展示对话内容
- 🔌 **WebSocket 长连接**：实时接收用户消息
- 📝 **Markdown 支持**：支持格式化文本和代码高亮

## 技术栈

- **语言**：Go 1.22.6
- **飞书 SDK**：`github.com/larksuite/oapi-sdk-go/v3`
- **Claude CLI**：本地进程调用（使用 `cc1` 命令）
- **通信方式**：WebSocket 长连接
- **卡片展示**：CardKit v1 + JSON 2.0 Schema

## 项目结构

```
feishu-bot/
├── cmd/
│   └── bot/                      # 主程序入口
│       └── main.go
├── internal/
│   ├── bot/
│   │   ├── client/               # 飞书客户端封装
│   │   │   └── feishu.go
│   │   └── handlers/             # 消息处理器
│   │       └── message.go
│   ├── claude/                   # Claude CLI 集成
│   │   ├── manager.go            # CLI 进程管理
│   │   ├── cardkit_updater.go    # 卡片流式更新
│   │   └── handler.go            # 对话处理器
│   ├── command/                  # 命令处理
│   ├── notification/             # 通知服务
│   └── session/                  # 会话管理
├── configs/                      # 配置文件
├── .env                          # 环境变量
├── go.mod
└── Makefile
```

## 核心功能

### 1. 流式对话
- 调用本地 Claude CLI (`cc1` 命令)
- 解析 `stream-json` 格式输出
- 实时提取文本增量

### 2. CardKit 集成
- 创建卡片实体
- 流式更新卡片内容
- 限流控制（10 次/秒）
- 打字机效果配置

### 3. 消息处理
- WebSocket 长连接接收消息
- 直接消息发起对话（无需 /chat）
- 自动创建和更新卡片

## 快速开始

### 1. 环境准备

确保已安装：
- Go 1.22.6+
- Claude CLI（配置为 `cc1` 别名）

### 2. 配置飞书应用

1. 访问 [飞书开放平台](https://open.feishu.cn/app)
2. 创建自建应用，获取 App ID 和 App Secret
3. 配置权限：
   - `im:message` - 获取与发送消息
   - `im:message:group_at_msg` - 群聊 @消息
   - `im:chat` - 访问群聊信息
   - `cardkit:card:write` - 创建与更新卡片
4. 配置事件订阅：
   - 选择"使用长连接接收事件"
   - 添加事件：`im.message.receive_v1`

### 3. 配置项目

```bash
# 复制环境变量模板
cp .env.example .env

# 编辑 .env 文件
FEISHU_APP_ID=cli_a9dc39c0c2b8dbc8
FEISHU_APP_SECRET=your_app_secret
```

### 4. 运行机器人

```bash
# 安装依赖
go mod download

# 运行机器人
go run cmd/bot/main.go
```

## 使用方法

### 发起对话

在飞书群聊或私聊中：

```
@机器人 你的问题
```

机器人会：
1. 创建一个新的对话卡片
2. 显示"思考中..."
3. 逐字显示 AI 回复（打字机效果）

### 示例对话

```
用户: @机器人 如何用 Go 实现 HTTP 服务器？

机器人: [创建卡片]
      [流式更新内容]
      在 Go 中，可以使用标准库的 net/http 包...
```

## 技术实现

### Claude CLI 集成

```go
// 启动 Claude CLI 进程
cmd := exec.Command("cc1",
    "-p",                                // 非交互模式
    "--output-format", "stream-json",    // 流式 JSON 输出
    "--include-partial-messages",        // 包含部分消息
)

// 解析流式输出
// {"type": "stream_event", "event": {"type": "content_block_delta", "delta": {"text": "..."}}}
```

### CardKit 流式更新

```go
// 1. 创建卡片实体
POST /open-apis/cardkit/v1/cards

// 2. 发送卡片到群聊
POST /open-apis/im/v1/messages

// 3. 流式更新卡片内容（限流 100ms）
PUT /open-apis/cardkit/v1/cards/{card_id}/elements/{element_id}/content
```

### WebSocket 长连接

```go
wsClient := larkws.NewClient(appID, appSecret,
    larkws.WithEventHandler(eventHandler),
    larkws.WithLogLevel(larkcore.LogLevelInfo),
)
wsClient.Start(context.Background())
```

## 配置说明

### 环境变量

| 变量名 | 说明 | 示例 |
|--------|------|------|
| `FEISHU_APP_ID` | 飞书应用 ID | `cli_a9dc39c0c2b8dbc8` |
| `FEISHU_APP_SECRET` | 飞书应用密钥 | `Y0psnqB52LC50Svx...` |
| `GROUP_REQUIRE_MENTION` | 群聊是否必须@机器人（默认 false） | `false` |

### CardKit 打字机效果

```json
{
  "config": {
    "streaming_mode": true,
    "streaming_config": {
      "print_frequency_ms": {"default": 70},
      "print_step": {"default": 1},
      "print_strategy": "fast"
    }
  }
}
```

## 开发状态

✅ 已完成：
- [x] Claude CLI 进程管理
- [x] Stream-JSON 解析器
- [x] CardKit 流式更新（限流 10 QPS）
- [x] 飞书消息处理集成
- [x] WebSocket 长连接
- [x] 直接消息触发对话（无需 /chat）
- [x] 打字机效果

⏳ 调试中：
- [ ] 飞书平台事件订阅配置

## 常见问题

### 1. 平台显示"应用未建立长连接"

**症状**：机器人日志显示 `connected`，但飞书平台显示未建立长连接

**解决方案**：
1. 即使提示未建立连接，也强制保存事件订阅配置
2. 重启机器人
3. 等待 2-3 分钟刷新页面
4. 或直接在群里测试，看是否能收到消息

### 2. 机器人无响应

**检查清单**：
- [ ] 机器人进程是否运行
- [ ] WebSocket 日志是否显示 `connected`
- [ ] 事件订阅是否配置成功
- [ ] 权限是否已开启

### 3. CardKit 更新失败

**可能原因**：
- 超过 10 QPS 限流
- token 过期
- card_id 或 element_id 错误

## 日志查看

```bash
# 查看机器人日志
tail -f /tmp/feishu-bot.log

# 搜索错误
grep "ERROR" /tmp/feishu-bot.log

# 检查 WebSocket 连接
grep "connected" /tmp/feishu-bot.log
```

## 相关资源

- [飞书开放平台文档](https://open.feishu.cn/document)
- [CardKit 2.0 指南](https://open.feishu.cn/document/common-capabilities/message-card/card-components)
- [Claude CLI 文档](https://docs.anthropic.com/claude-cli/overview)
- [飞书 Go SDK](https://github.com/larksuite/oapi-sdk-go)

## 许可证

MIT License
