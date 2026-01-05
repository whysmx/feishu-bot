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

#### 3.1 基础目录（用于 ls/bind）

```bash
# .env 文件
BASE_DIR=C:\Users\YourName\projects
```

**说明**：
- 作为群聊 `ls/bind` 的扫描目录
- 可使用绝对路径或磁盘盘符路径

#### 3.2 日志文件

```bash
# .env 文件
LOG_FILE=data\logs\bot.log
```

**说明**：
- 相对路径：`data\logs\bot.log`（相对于可执行文件目录）
- 或绝对路径：`C:\feishu-bot\logs\bot.log`

---

### 4. 日志级别

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

### 6. 基础目录配置（可选）

```bash
# .env 文件
BASE_DIR=C:\\Users\\YourName\\projects
```

**说明**：
- `BASE_DIR` 用于 `ls/bind` 扫描项目目录
- 未配置时使用默认值（代码内置）

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
- `BASE_DIR`（可选，扫描目录）
- `LOG_FILE`（可选，日志路径）

### 3. 确认配置目录可写

`configs/chat_config.json` 会在首次运行时自动生成并写入绑定信息，无需手动创建。

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
LOG_FILE=data\\logs\\bot.log
BASE_DIR=C:\\Users\\YourName\\projects

# ==================== 日志配置 ====================
LOG_LEVEL=info
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
