# 飞书 Claude CLI 流式对话机器人

通过飞书长连接接收消息，调用本地 Claude CLI，并将 Claude 的输出按段发送为文本消息。

## 功能概览

- **Claude CLI 集成**：默认使用 `claude` 命令（可通过 `CLAUDE_CLI_PATH` 指定路径）
- **流式文本输出**：基于空闲超时/持续时间/最大缓冲区分段发送，避免超过飞书消息大小限制
- **会话管理**：P2P 按用户维持会话，群聊使用全局共享会话
- **群聊指令**：@ 机器人后支持 `ls` / `bind` / `help`（项目路径绑定）
- **长连接**：使用飞书 WebSocket 事件订阅接收消息

## 技术栈

- **语言**：Go 1.22.x
- **飞书 SDK**：`github.com/larksuite/oapi-sdk-go/v3`
- **Claude CLI**：本地 `claude` 命令（`--output-format stream-json`）
- **通信方式**：飞书长连接 WebSocket

## 项目结构

```
./
├── cmd/
│   └── bot/                  # 主程序入口
├── internal/
│   ├── bot/                  # 飞书客户端与消息处理
│   ├── claude/               # Claude CLI 管理与流式处理
│   ├── config/               # 项目绑定配置
│   └── utils/                # 工具函数（超时、路径）
├── configs/
│   └── chat_config.json      # 群聊绑定配置（运行时会更新）
├── scripts/                  # 启停脚本（macOS/Linux/Windows）
├── docs/                     # 设计/测试文档
├── .env.example              # 环境变量示例
└── README.md
```

## 快速开始

### 1. 依赖

- Go 1.22+
- Claude CLI（确保 `claude` 在 PATH 中，或设置 `CLAUDE_CLI_PATH`）
- Anthropic API Key 与 Auth Token

### 2. 配置飞书应用

1. 访问 [飞书开放平台](https://open.feishu.cn/app) 创建企业自建应用
2. 记录 App ID / App Secret
3. 开启权限（实际用到）：
   - `im:message`（收发消息）
   - `im:message.group_at_msg`（群聊 @ 消息）
4. 事件订阅：选择**长连接**并添加 `im.message.receive_v1`

### 3. 配置环境变量

```bash
cp .env.example .env
```

> 必填项请根据 `.env.example` 填写。

### 4. 运行

```bash
# 直接运行
go run ./cmd/bot

# 或构建后运行
make build
./scripts/start-bot.sh
```

## 使用方法

### 私聊（P2P）

直接发送消息即可触发 Claude 对话；会话会按用户维持。

### 群聊

- 普通消息会被转发给 Claude
- **@机器人**后可使用指令（不会转发给 Claude）：

```
@机器人 ls
@机器人 bind 3
@机器人 help
```

## 群聊指令

### 1) ls：列出基础目录

```
@机器人 ls
```

列出 `BASE_DIR` 下可绑定的项目目录，并显示当前绑定。

### 2) bind：绑定项目路径

```
@机器人 bind <序号>
```

将群聊绑定到指定项目目录。绑定后 Claude CLI 会以该目录作为工作目录启动。

### 3) help：查看指令

```
@机器人 help
```

显示指令列表与当前绑定。

## 配置说明

### 环境变量

| 变量名 | 必需 | 说明 | 默认值 |
|--------|------|------|--------|
| `FEISHU_APP_ID` | 是 | 飞书 App ID | - |
| `FEISHU_APP_SECRET` | 是 | 飞书 App Secret | - |
| `ANTHROPIC_API_KEY` | 是 | Anthropic API Key | - |
| `ANTHROPIC_AUTH_TOKEN` | 是 | Anthropic Auth Token | - |
| `ANTHROPIC_BASE_URL` | 否 | Anthropic API Base URL | `https://api.anthropic.com` |
| `CLAUDE_CLI_PATH` | 否 | Claude CLI 路径 | `claude` |
| `BASE_DIR` | 否 | `ls/bind` 的基础目录 | `/Users/wen/Desktop/code/` |
| `LOG_LEVEL` | 否 | 日志级别 | `info` |
| `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` | 否 | Claude Code 流量开关 | `true` |
| `CLAUDE_CODE_ENABLE_UNIFIED_READ_TOOL` | 否 | Claude Code 读取工具开关 | `true` |

### 群聊绑定配置

- 文件：`configs/chat_config.json`
- 运行时会自动读取/写入
- 未配置时会自动生成，并使用默认 `base_dir`

### 流式输出分段参数

分段策略由 `internal/utils/timeout.go` 统一配置：

- `StreamIdleTimeout`：空闲多久发送一次缓冲内容
- `StreamMaxDuration`：连续输出超过多久强制分段
- `StreamMaxBufferSize`：缓冲区最大字符数

## 日志与排查

- `LOG_LEVEL=debug` 可开启更详细日志
- 使用脚本启动时，日志默认写入 `/tmp/feishu-bot-latest.log`（可用 `LOG_FILE` 覆盖）
- 运行时会在系统临时目录输出最近事件快照（如 `feishu-last-*.json`、`feishu-event-trace.log`）

## 相关资源

- [飞书开放平台文档](https://open.feishu.cn/document)
- [Claude CLI 文档](https://docs.anthropic.com/claude-cli/overview)
- [飞书 Go SDK](https://github.com/larksuite/oapi-sdk-go)

## 许可证

MIT License
