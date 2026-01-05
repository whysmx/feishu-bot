# 服务器迁移部署指南

本文档说明如何将飞书机器人迁移到新服务器并更换飞书账号。

## 前置准备

### 1. 新飞书应用配置

在新的飞书开放平台创建应用：

1. 访问 https://open.feishu.cn/app
2. 创建新应用，记录以下信息：
   - **App ID**（格式：`cli_xxxxxxxx`）
   - **App Secret**（在"凭证与基础信息"页面获取）

3. 配置应用权限：
   - `im:message` - 获取与发送消息
   - `im:message:group_at_msg` - 群聊 @消息
   - `im:chat` - 访问群聊信息
   - `cardkit:card:write` - 创建与更新卡片

4. 配置事件订阅：
   - 方式：WebSocket 长连接
   - 事件：`im.message.receive_v1`

### 2. 新的 Anthropic API 密钥

如果更换 Anthropic 账号：

1. 访问 https://console.anthropic.com/
2. 获取新的 API Key（格式：`sk-ant-xxxxx.xxxxx`）
3. 记录 **API Key** 和 **Session Token**

### 3. 安装 Claude CLI

```bash
# 使用 npm 全局安装
npm install -g @anthropic-ai/claude-cli

# 或使用 yarn
yarn global add @anthropic-ai/claude-cli

# 验证安装
claude --version
```

## 部署步骤

### 1. 克隆代码到新服务器

```bash
git clone <your-repo-url> 18feishu
cd 18feishu
```

### 2. 创建配置文件

```bash
# 复制示例配置
cp .env.example .env

# 编辑配置文件
vim .env
```

### 3. 填写配置项

**必需配置**：

```bash
# ==================== 飞书应用配置 ====================
FEISHU_APP_ID=cli_xxxxxxxxxxxxxx        # 新飞书应用的 App ID
FEISHU_APP_SECRET=xxxxxx                 # 新飞书应用的 App Secret
FEISHU_BASE_DOMAIN=https://open.feishu.cn

# ==================== Claude CLI 配置 ====================
# Anthropic API 配置（必需）
ANTHROPIC_API_KEY=sk-ant-xxxxx.xxxxx     # 新账号的 API Key
ANTHROPIC_AUTH_TOKEN=xxxxx                # 新账号的 Session Token
ANTHROPIC_BASE_URL=https://api.anthropic.com

# Claude Code 特性开关（可选）
CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=true
CLAUDE_CODE_ENABLE_UNIFIED_READ_TOOL=true

# ==================== 服务器配置 ====================
PORT=8080
```

**可选配置**：

```bash
# 如果 Claude CLI 不在 PATH 中，指定完整路径
CLAUDE_CLI_PATH=/usr/local/bin/claude

# 自定义日志文件路径
LOG_FILE=/var/log/feishu-bot/bot.log

# 会话存储文件
SESSION_STORAGE_FILE=data/sessions.json
```

### 4. 创建必要目录

```bash
mkdir -p data logs configs/security
```

### 5. 构建项目

```bash
# 强制重新构建（清除缓存）
go build -a -o bin/bot cmd/bot/main.go
```

### 6. 测试运行

```bash
# 前台运行，查看日志
./bin/bot
```

**预期输出**：

```
Starting Feishu Bot... version=dev build_time=unknown commit=unknown
Loaded .env from /path/to/.env
Using FEISHU_APP_ID=cli_xxxxxxxxxxxxxx
Starting WebSocket connection to Feishu...
...
```

**如果配置错误**，会看到类似提示：

```
配置验证失败: 缺少必需的环境变量或格式错误:
  - ANTHROPIC_API_KEY: Anthropic API Key
  - ANTHROPIC_AUTH_TOKEN: Anthropic Auth Token

请在 .env 文件中配置这些变量
```

### 7. 后台运行

```bash
# 使用启动脚本
./scripts/start-bot.sh

# 或手动后台运行
nohup ./bin/bot > /tmp/feishu-bot.log 2>&1 &
echo $! > /tmp/feishu-bot.pid
```

### 8. 验证运行状态

```bash
# 检查进程
ps aux | grep "./bin/bot"

# 查看日志
tail -f /tmp/feishu-bot-latest.log

# 检查 WebSocket 连接
grep "connected" /tmp/feishu-bot-latest.log
```

## 配置项说明

### 飞书相关

| 环境变量 | 是否必需 | 说明 | 示例 |
|---------|---------|------|------|
| `FEISHU_APP_ID` | ✅ 必需 | 飞书应用 ID | `cli_xxxxxxxxxxxxxx` |
| `FEISHU_APP_SECRET` | ✅ 必需 | 飞书应用密钥 | 从开放平台获取 |
| `FEISHU_BASE_DOMAIN` | ❌ 可选 | 飞书 API 域名 | `https://open.feishu.cn` |

### Claude CLI 相关

| 环境变量 | 是否必需 | 说明 | 示例 |
|---------|---------|------|------|
| `CLAUDE_CLI_PATH` | ❌ 可选 | Claude CLI 路径 | `claude` 或 `/usr/local/bin/claude` |
| `ANTHROPIC_API_KEY` | ✅ 必需 | Anthropic API Key | `sk-ant-xxxxx.xxxxx` |
| `ANTHROPIC_AUTH_TOKEN` | ✅ 必需 | Session Token | 从控制台获取 |
| `ANTHROPIC_BASE_URL` | ❌ 可选 | API 基础 URL | `https://api.anthropic.com` |

### 服务器相关

| 环境变量 | 是否必需 | 说明 | 默认值 |
|---------|---------|------|--------|
| `PORT` | ❌ 可选 | Webhook 端口 | `8080` |
| `LOG_FILE` | ❌ 可选 | 日志文件路径 | `/tmp/feishu-bot-latest.log` |
| `SESSION_STORAGE_FILE` | ❌ 可选 | 会话存储文件 | `data/sessions.json` |

## 常见问题

### Q1: 启动时提示 "Claude CLI 未找到"

**原因**：Claude CLI 未安装或路径不正确

**解决方案**：
```bash
# 检查 Claude CLI 是否在 PATH 中
which claude

# 如果不在，安装或指定路径
export CLAUDE_CLI_PATH=/usr/local/bin/claude
```

### Q2: 配置验证失败

**原因**：`.env` 文件中缺少必需的配置项

**解决方案**：
```bash
# 检查 .env 文件是否存在
ls -la .env

# 确保配置了所有必需项
grep -E "FEISHU_APP_ID|FEISHU_APP_SECRET|ANTHROPIC_API_KEY|ANTHROPIC_AUTH_TOKEN" .env
```

### Q3: WebSocket 连接失败

**原因**：飞书 App ID 或 Secret 配置错误

**解决方案**：
```bash
# 1. 验证 App ID 格式（应该以 cli_ 开头）
grep FEISHU_APP_ID .env

# 2. 检查日志中的错误信息
grep -i "error\|fail" /tmp/feishu-bot-latest.log

# 3. 在飞书开放平台检查应用配置
# https://open.feishu.cn/app/<YOUR_APP_ID>/event
```

### Q4: 机器人收不到消息

**原因**：
1. 飞书事件未订阅
2. 权限未配置
3. 有多个机器人实例在运行

**解决方案**：
```bash
# 1. 检查是否有多个实例
ps aux | grep "./bin/bot"

# 2. 停止所有旧实例
./scripts/stop-bot.sh

# 3. 检查飞书开放平台的事件推送状态
# 访问：https://open.feishu.cn/app/<YOUR_APP_ID>/logs
```

## 安全建议

### 1. 保护敏感信息

```bash
# 设置 .env 文件权限
chmod 600 .env

# 不要将 .env 提交到 Git
echo ".env" >> .gitignore
```

### 2. 使用环境变量（生产环境）

生产环境建议直接设置环境变量，而不是使用 `.env` 文件：

```bash
# systemd 服务文件示例
[Service]
Environment="FEISHU_APP_ID=cli_xxxxxxxxxxxxxx"
Environment="FEISHU_APP_SECRET=xxxxxx"
Environment="ANTHROPIC_API_KEY=sk-ant-xxxxx.xxxxx"
Environment="ANTHROPIC_AUTH_TOKEN=xxxxx"
ExecStart=/path/to/bin/bot
```

### 3. 定期更新密钥

建议每 3-6 个月更换一次 API 密钥。

## 迁移检查清单

- [ ] 创建新飞书应用并获取 App ID 和 Secret
- [ ] 获取新的 Anthropic API Key 和 Auth Token
- [ ] 在新服务器安装 Claude CLI
- [ ] 克隆代码到新服务器
- [ ] 创建并配置 `.env` 文件
- [ ] 创建必要目录（data、logs）
- [ ] 构建项目（`go build -a`）
- [ ] 测试运行（前台运行查看日志）
- [ ] 后台运行
- [ ] 在飞书中测试机器人（单聊和群聊）
- [ ] 检查日志确认 WebSocket 连接成功
- [ ] 设置开机自启动（可选）

## 完成部署后

1. **测试单聊**：在飞书中私聊机器人，发送测试消息
2. **测试群聊**：将机器人加入群聊，@机器人测试
3. **查看日志**：确认没有错误信息
4. **监控运行**：观察一段时间确保稳定性

---

如有问题，请查看日志文件：
```bash
tail -f /tmp/feishu-bot-latest.log
```
