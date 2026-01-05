# 飞书机器人配置完整指南

本文档描述当前实现（Claude CLI + 文本分段发送）的配置步骤，适用于企业自建应用。

## 📋 配置清单

### 1. 创建应用

**访问地址**：https://open.feishu.cn/app

**步骤**：
1. 点击"创建企业自建应用"
2. 选择应用类型：企业自建应用
3. 填写应用信息：
   - 应用名称：Claude Stream Bot（或自定义名称）
   - 应用描述：Claude CLI 流式对话机器人
4. 创建后记录以下信息：
   - **App ID**：`cli_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`（示例）
   - **App Secret**：从"凭证与基础信息"页面获取

### 2. 配置权限

**访问地址**：https://open.feishu.cn/app/cli_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx/auth

**必需权限**：

| 权限名称 | 权限ID | 用途 | 状态 |
|---------|--------|------|------|
| 获取与发送单聊、群组消息 | `im:message` | 接收和发送消息 | ✅ 必需 |
| 群聊中@消息的接收 | `im:message.group_at_msg` | 群聊中识别 @ 及命令 | ✅ 建议 |
| 访问群聊信息 | `im:chat` | 仅在需要读取群聊信息时使用 | ⭕️ 可选 |

**配置步骤**：
1. 访问权限管理页面
2. 搜索并启用上述权限
3. 点击"保存"
4. （重要）**发布版本**：权限修改后需要创建版本才能生效

### 3. 配置事件订阅

**访问地址**：https://open.feishu.cn/app/cli_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx/event

**订阅方式**：使用长连接接收事件

**配置步骤**：

#### 步骤 1：选择订阅方式
1. 点击"订阅方式"按钮
2. 选择"**使用长连接接收事件**"
3. 点击"保存"

**说明**：
- 长连接模式无需公网域名或 Webhook
- 需要使用飞书官方 SDK 启动长连接客户端
- 适合本地开发和测试

#### 步骤 2：添加事件
1. 点击右上角"**添加事件**"按钮
2. 在搜索框中输入：`message` 或 `im.message`
3. 找到"**im.message.receive_v1**"（接收消息）
4. 勾选该事件
5. 点击"确认添加"
6. 等待页面刷新，确认事件已添加

**事件详情**：
- **事件名称**：im.message.receive_v1
- **事件类型**：消息
- **订阅类型**：应用身份订阅
- **所需权限**：`im:message`
- **用途**：接收用户发送的消息（单聊和群聊）

#### 步骤 3：验证配置
确认页面显示：
- ✅ 订阅方式：长连接
- ✅ 已添加事件：im.message.receive_v1
- ✅ 不再显示"暂无数据"

### 4. 配置环境变量

```bash
# 飞书应用配置
FEISHU_APP_ID=cli_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
FEISHU_APP_SECRET=your_app_secret_here

# Anthropic API 配置（必需）
ANTHROPIC_API_KEY=your_api_key_here
ANTHROPIC_AUTH_TOKEN=your_auth_token_here
ANTHROPIC_BASE_URL=https://api.anthropic.com

# Claude CLI（可选，默认 claude）
CLAUDE_CLI_PATH=claude

# 其他可选项
LOG_LEVEL=info
BASE_DIR=/Users/wen/Desktop/code/
```

### 5. 启动机器人

**前提条件**：
- ✅ 已完成应用配置
- ✅ 已配置权限
- ✅ 已配置事件订阅
- ✅ 飞书机器人已加入测试群

**启动命令**：
```bash
# 直接运行
cd /path/to/feishu-bot
go run ./cmd/bot

# 或使用脚本
./scripts/start-bot.sh
```

**验证连接**：
```bash
# 查看日志
log_file=/tmp/feishu-bot-latest.log
[[ -f /tmp/feishu-bot.log ]] && log_file=/tmp/feishu-bot.log

tail -f "$log_file"

# 应该看到
[Info] connected to wss://msg-frontier.feishu.cn/ws/v2
```

### 6. 测试机器人

**在测试群中测试**：
```
你好
```

**预期结果**：
1. 机器人按段发送多条文本消息
2. Claude 输出逐步显示（非卡片）
3. 机器人日志显示收到消息

**群聊命令测试**（需 @ 机器人）：
```
@机器人 ls
@机器人 bind 3
@机器人 help
```

**查看日志**：
```bash
# 查看消息接收
grep "Message received" "$log_file"

# 查看流式处理
grep "StreamingTextHandler" "$log_file"
```

## 📊 配置检查清单

部署新项目时，按以下顺序检查：

- [ ] **1. 应用创建**
  - [ ] 已创建企业自建应用
  - [ ] 已记录 App ID 和 App Secret

- [ ] **2. 权限配置**
  - [ ] `im:message` ✅
  - [ ] `im:message.group_at_msg` ✅（群聊命令）
  - [ ] `im:chat` ✅（可选）
  - [ ] 已发布版本（权限生效）

- [ ] **3. 事件订阅**
  - [ ] 订阅方式：长连接 ✅
  - [ ] 已添加：im.message.receive_v1 ✅
  - [ ] 配置已保存 ✅

- [ ] **4. 代码配置**
  - [ ] .env 文件已配置
  - [ ] App ID 和 Secret 正确
  - [ ] Claude CLI 可执行
  - [ ] Anthropic API Key/Auth Token 正确

- [ ] **5. 测试验证**
  - [ ] 机器人已加入测试群
  - [ ] 日志显示 WebSocket 已连接
  - [ ] 能接收并响应消息

## 🐛 常见问题

### 1. 平台显示"应用未建立长连接"

**症状**：
- 机器人日志显示 `connected`
- 但飞书平台显示"应用未建立长连接"
- 无法保存事件订阅配置

**解决方案**：
1. 确认机器人进程正在运行
2. 检查日志是否显示 `connected to wss://msg-frontier.feishu.cn`
3. 等待 2-3 分钟，刷新页面重试
4. 或直接继续配置事件订阅，可能是平台检测延迟

### 2. 机器人收不到消息

**检查清单**：
1. [ ] 事件是否添加：im.message.receive_v1
2. [ ] 权限是否开启：im:message
3. [ ] 机器人是否在群里
4. [ ] 群聊是否需要 @（取决于飞书设置）
5. [ ] 查看机器人日志是否有错误

**调试命令**：
```bash
# 检查机器人进程
ps aux | grep "bot/main"

# 查看日志
tail -50 "$log_file"

# 搜索错误
grep -i "error" "$log_file"
```

### 3. Claude CLI 启动失败

**可能原因**：
- Claude CLI 未安装或路径错误
- `CLAUDE_CLI_PATH` 未配置
- `ANTHROPIC_API_KEY` 或 `ANTHROPIC_AUTH_TOKEN` 缺失

**解决方案**：
- 确认 `claude` 可执行
- 检查 `.env` 中的 Anthropic 配置
- 查看日志中的 Claude CLI 错误输出

### 4. 消息发送失败

**可能原因**：
- tenant token 获取失败
- receive_id 为空或类型不匹配
- 飞书 API 返回错误码

**解决方案**：
- 检查日志中的 API error
- 重新获取 token 或重启机器人

## 📚 相关文档

- [飞书开放平台文档](https://open.feishu.cn/document)
- [事件订阅指南](https://open.feishu.cn/document/server-docs/event-subscription-guide)
- [Claude CLI 文档](https://docs.anthropic.com/claude-cli/overview)
- [飞书 Go SDK](https://github.com/larksuite/oapi-sdk-go)

## 🔄 配置模板

### 快速复制配置

**应用信息**：
```yaml
应用类型: 企业自建应用
App ID: cli_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
App Secret: [从飞书平台获取]
```

**必需权限**：
```yaml
permissions:
  - im:message
  - im:message.group_at_msg
  # 可选：im:chat
```

**事件订阅**：
```yaml
subscription_mode: 长连接
events:
  - im.message.receive_v1
```

---

**最后更新**：2026-01-05
**维护者**：Claude Code
**适用于**：飞书开放平台企业自建应用
