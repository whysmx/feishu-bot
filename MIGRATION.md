# 服务器迁移部署指南

本文档说明如何将飞书机器人迁移到新服务器并更换飞书账号（当前实现为文本分段发送）。

## 前置准备

### 1. 新飞书应用配置

1. 访问 https://open.feishu.cn/app
2. 创建新应用，记录以下信息：
   - **App ID**（格式：`cli_xxxxxxxx`）
   - **App Secret**（在"凭证与基础信息"页面获取）

3. 配置应用权限：
   - `im:message` - 获取与发送消息
   - `im:message.group_at_msg` - 群聊 @ 消息（用于命令）
   - `im:chat` - 访问群聊信息（可选）

4. 配置事件订阅：
   - 方式：WebSocket 长连接
   - 事件：`im.message.receive_v1`

### 2. 新的 Anthropic API 密钥

如果更换 Anthropic 账号：

1. 访问 https://console.anthropic.com/
2. 获取新的 API Key
3. 记录 **API Key** 与 **Auth Token**

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
cp .env.example .env
vim .env
```

### 3. 填写配置项

**必需配置**：

```bash
# ==================== 飞书应用配置 ====================
FEISHU_APP_ID=cli_xxxxxxxxxxxxxx        # 新飞书应用的 App ID
FEISHU_APP_SECRET=xxxxxx                 # 新飞书应用的 App Secret

# ==================== Claude CLI 配置 ====================
ANTHROPIC_API_KEY=sk-ant-xxxxx.xxxxx     # 新账号的 API Key
ANTHROPIC_AUTH_TOKEN=xxxxx               # 新账号的 Auth Token
ANTHROPIC_BASE_URL=https://api.anthropic.com

# Claude Code 特性开关（可选）
CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=true
CLAUDE_CODE_ENABLE_UNIFIED_READ_TOOL=true
```

**可选配置**：

```bash
# 如果 Claude CLI 不在 PATH 中，指定完整路径
CLAUDE_CLI_PATH=/usr/local/bin/claude

# 自定义日志文件路径
LOG_FILE=/var/log/feishu-bot/bot.log

# 群聊绑定扫描目录
BASE_DIR=/data/projects/
```

### 4. 构建项目

```bash
# 强制重新构建（清除缓存）
go build -a -o bin/bot cmd/bot/main.go
```

### 5. 测试运行

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

### 6. 后台运行

```bash
# 使用启动脚本
./scripts/start-bot.sh

# 或手动后台运行
nohup ./bin/bot > /tmp/feishu-bot.log 2>&1 &
echo $! > /tmp/feishu-bot.pid
```

### 7. 验证运行状态

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

### Claude CLI 相关

| 环境变量 | 是否必需 | 说明 | 示例 |
|---------|---------|------|------|
| `CLAUDE_CLI_PATH` | ❌ 可选 | Claude CLI 路径 | `claude` 或 `/usr/local/bin/claude` |
| `ANTHROPIC_API_KEY` | ✅ 必需 | Anthropic API Key | `sk-ant-xxxxx.xxxxx` |
| `ANTHROPIC_AUTH_TOKEN` | ✅ 必需 | Auth Token | 从控制台获取 |
| `ANTHROPIC_BASE_URL` | ❌ 可选 | API 基础 URL | `https://api.anthropic.com` |

### 其他配置

| 环境变量 | 是否必需 | 说明 | 默认值 |
|---------|---------|------|--------|
| `LOG_FILE` | ❌ 可选 | 日志文件路径 | `/tmp/feishu-bot-latest.log` |
| `BASE_DIR` | ❌ 可选 | `ls/bind` 基础目录 | `/Users/wen/Desktop/code/` |

## 常见问题

### Q1: 启动时提示 "Claude CLI 未找到"

**原因**：Claude CLI 未安装或路径不正确

**解决方案**：
1. 运行 `claude --version` 确认可执行
2. 配置 `CLAUDE_CLI_PATH` 指向实际路径

### Q2: 收不到群聊消息

**原因**：权限或事件订阅不完整，或平台仅推送 @ 消息

**解决方案**：
1. 检查 `im:message` 权限
2. 确认 `im.message.receive_v1` 已订阅
3. 测试时用真实 @ mention
