# Windows 部署配置指南

本文档说明将飞书机器人部署到 Windows 系统时需要修改的所有配置项。

## 📋 配置清单

### 1. 飞书应用配置（必须修改）

这些配置与你的飞书账号绑定，**必须修改为新账号的配置**。

```bash
# .env 文件
FEISHU_APP_ID=cli_xxxxxxxx           # 新账号的应用 ID
FEISHU_APP_SECRET=your_app_secret     # 新账号的应用密钥
```

**获取方式**：
1. 访问 [飞书开放平台](https://open.feishu.cn/)
2. 创建自建应用或使用现有应用
3. 在"凭证与基础信息"页面获取 APP_ID 和 APP_SECRET

**注意事项**：
- 应用 ID 格式：`cli_` 开头 + 32 位字符
- 应用密钥长度：至少 32 位
- **不同账号的应用 ID 和密钥完全不同**

---

### 2. Claude CLI 配置（必须配置）

#### 2.1 Claude CLI 可执行文件路径

Windows 系统需要指定完整的 Claude CLI 路径。

```bash
# .env 文件
CLAUDE_CLI_PATH=C:\Users\YourName\AppData\Roaming\npm\claude.cmd
```

**Windows 常见路径**：
- npm 全局安装：`C:\Users\<用户名>\AppData\Roaming\npm\claude.cmd`
- 或使用 `where claude` 命令查找路径

#### 2.2 Anthropic API 配置

```bash
# .env 文件
ANTHROPIC_API_KEY=sk-ant-xxxxx           # 你的 API Key
ANTHROPIC_AUTH_TOKEN=sk-ant-xxxxx        # 你的 Auth Token
ANTHROPIC_BASE_URL=https://api.anthropic.com  # API 基础 URL
```

**注意事项**：
- API Key 和 Auth Token 格式：包含点号 `.`
- 如果使用代理，修改 `ANTHROPIC_BASE_URL`
- 这些凭据与你的 Anthropic 账号绑定

---

### 3. 路径配置（Windows 特定）

以下路径需要修改为 Windows 风格：

#### 3.1 会话存储文件

```bash
# .env 文件
SESSION_STORAGE_FILE=data\sessions.json
```

**说明**：
- 相对路径：`data\sessions.json`（相对于可执行文件目录）
- 或绝对路径：`C:\feishu-bot\data\sessions.json`

#### 3.2 项目配置文件

```bash
# .env 文件
PROJECT_CONFIG_FILE=C:\Users\YourName\.feishu-bot\projects.json
```

**Windows 路径规则**：
- 用户主目录：`C:\Users\<用户名>\`
- 配置目录：`C:\Users\<用户名>\.feishu-bot\`
- 需要手动创建 `.feishu-bot` 目录

#### 3.3 日志文件

```bash
# .env 文件
LOG_FILE=data\logs\bot.log
```

**说明**：
- 相对路径：`data\logs\bot.log`
- 或绝对路径：`C:\feishu-bot\logs\bot.log`

---

### 4. 服务器配置

```bash
# .env 文件
PORT=8080
```

**说明**：
- Windows 默认端口：8080
- 确保防火墙允许该端口

---

### 5. 日志级别

```bash
# .env 文件
LOG_LEVEL=info
```

**可选值**：
- `debug` - 详细调试信息
- `info` - 一般信息（推荐生产环境）
- `warn` - 警告信息
- `error` - 仅错误信息

---

### 6. 群聊配置

```bash
# .env 文件
GROUP_REQUIRE_MENTION=false
```

**说明**：
- `false` - 群聊中@机器人才能触发对话（推荐）
- `true` - 群聊中任意消息都会触发（测试用）

---

### 7. CardKit 卡片模板 ID（可选）

如果使用 CardKit 功能，需要配置卡片模板 ID：

```bash
# .env 文件
TASK_COMPLETED_CARD_ID=AAqz1Y1QyEzLF
TASK_WAITING_CARD_ID=AAqz1Y1p8y5Se
COMMAND_RESULT_CARD_ID=AAqz1Y1TvQB25
SESSION_LIST_CARD_ID=
```

**说明**：
- 这些 ID 与飞书账号绑定
- 新账号需要重新创建卡片模板
- 如果不使用 CardKit，可以留空

---

## 🚀 Windows 部署步骤

### 1. 准备工作

```powershell
# 创建项目目录
mkdir C:\feishu-bot
cd C:\feishu-bot

# 克隆代码
git clone https://github.com/whysmx/feishu-bot.git .

# 创建数据目录
mkdir data
mkdir data\logs
```

### 2. 配置环境变量

```powershell
# 复制示例配置文件
copy .env.example .env

# 使用记事本编辑配置
notepad .env
```

**必须修改的配置**：
- `FEISHU_APP_ID`
- `FEISHU_APP_SECRET`
- `ANTHROPIC_API_KEY`
- `ANTHROPIC_AUTH_TOKEN`
- `CLAUDE_CLI_PATH`（Windows 完整路径）
- `PROJECT_CONFIG_FILE`（Windows 路径）
- `SESSION_STORAGE_FILE`（Windows 路径）

### 3. 创建项目配置文件

```powershell
# 创建配置目录
mkdir C:\Users\$env:USERNAME\.feishu-bot

# 创建 projects.json
notepad C:\Users\$env:USERNAME\.feishu-bot\projects.json
```

**projects.json 示例**：
```json
{
  "bindings": {
    "oc_群聊ID1": "C:\\path\\to\\project1",
    "oc_群聊ID2": "C:\\path\\to\\project2"
  }
}
```

### 4. 编译项目

```powershell
# 下载依赖
go mod download

# 编译 Windows 可执行文件
go build -o bot.exe cmd/bot/main.go
```

### 5. 运行机器人

```powershell
# 前台运行（测试）
.\bot.exe

# 后台运行（生产环境）
Start-Process -WindowStyle Hidden -FilePath .\bot.exe
```

### 6. 验证运行

```powershell
# 检查进程
Get-Process | Where-Object {$_.ProcessName -like "bot"}

# 查看日志
Get-Content data\logs\bot.log -Tail 50 -Wait
```

---

## 🔧 常见问题

### Q1: Claude CLI 路径找不到

**问题**：`exec: "claude": executable file not found`

**解决**：
```powershell
# 查找 Claude CLI 路径
where claude

# 在 .env 中配置完整路径
CLAUDE_CLI_PATH=C:\Users\YourName\AppData\Roaming\npm\claude.cmd
```

### Q2: 路径分隔符错误

**问题**：文件路径使用正斜杠 `/`，Windows 需要反斜杠 `\`

**解决**：
- 在 .env 文件中使用反斜杠：`data\sessions.json`
- 或使用正斜杠（Go 会自动转换）：`data/sessions.json`

### Q3: 权限问题

**问题**：无法写入 `data/sessions.json`

**解决**：
```powershell
# 检查目录权限
icacls data

# 授予写入权限
icacls data /grant "$($env:USERNAME):(OI)(CI)F"
```

### Q4: 防火墙阻止

**问题**：无法连接飞书 WebSocket

**解决**：
```powershell
# 添加防火墙规则
New-NetFirewallRule -DisplayName "Feishu Bot" -Direction Inbound -Program "C:\feishu-bot\bot.exe" -Action Allow
```

---

## 📝 配置示例（完整）

以下是 Windows 环境的完整 `.env` 示例：

```bash
# ==================== 飞书应用配置 ====================
FEISHU_APP_ID=cli_a1234567890abcdef
FEISHU_APP_SECRET=abcdefghijklmnopqrstuvwxyz123456

# ==================== Claude CLI 配置 ====================
# Windows 完整路径
CLAUDE_CLI_PATH=C:\Users\YourName\AppData\Roaming\npm\claude.cmd

# Anthropic API 配置
ANTHROPIC_API_KEY=sk-ant-api03-xxxxx
ANTHROPIC_AUTH_TOKEN=sk-ant-api03-xxxxx
ANTHROPIC_BASE_URL=https://api.anthropic.com

# Claude Code 特性开关
CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=true
CLAUDE_CODE_ENABLE_UNIFIED_READ_TOOL=true

# ==================== 路径配置（Windows）====================
SESSION_STORAGE_FILE=data\sessions.json
PROJECT_CONFIG_FILE=C:\Users\YourName\.feishu-bot\projects.json
LOG_FILE=data\logs\bot.log

# ==================== 服务器配置 ====================
PORT=8080

# ==================== 日志配置 ====================
LOG_LEVEL=info

# ==================== 群聊配置 ====================
GROUP_REQUIRE_MENTION=false

# ==================== CardKit 配置（可选）====================
TASK_COMPLETED_CARD_ID=
TASK_WAITING_CARD_ID=
COMMAND_RESULT_CARD_ID=
SESSION_LIST_CARD_ID=
```

---

## ✅ 部署检查清单

部署前请确认以下事项：

- [ ] 修改了飞书应用 ID 和密钥
- [ ] 配置了 Claude CLI 完整路径
- [ ] 配置了 Anthropic API 凭证
- [ ] 修改了所有文件路径为 Windows 格式
- [ ] 创建了 `data` 和 `data\logs` 目录
- [ ] 创建了 `.feishu-bot` 配置目录
- [ ] 配置了 `projects.json` 文件
- [ ] 编译了 Windows 可执行文件
- [ ] 测试运行并检查日志
- [ ] 配置了防火墙规则
- [ ] 设置了开机自启动（如需要）

---

## 🎯 关键差异总结

| 配置项 | macOS/Linux | Windows |
|--------|-------------|---------|
| Claude CLI 路径 | `/usr/local/bin/claude` | `C:\Users\<用户>\AppData\Roaming\npm\claude.cmd` |
| 用户主目录 | `~` 或 `/home/user` | `C:\Users\<用户>` |
| 路径分隔符 | `/` | `\`（但也支持 `/`） |
| 配置目录 | `~/.feishu-bot/` | `C:\Users\<用户>\.feishu-bot\` |
| 可执行文件 | `./bot` | `.\bot.exe` |
| 后台运行 | `./bot &` | `Start-Process -WindowStyle Hidden` |

---

## 📚 相关文档

- [飞书开放平台文档](https://open.feishu.cn/document)
- [Claude CLI 文档](https://docs.anthropic.com/claude-cli/overview)
- [项目设计文档](./docs/design.md)
- [迁移指南](./MIGRATION.md)
