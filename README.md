# 飞书 Claude Code 远程控制机器人

基于飞书平台的Claude Code远程控制机器人，实现类似Claude Code Remote的功能，通过飞书消息远程控制Claude Code会话。

## 项目概述

该项目将原本基于邮件的Claude Code远程控制系统改造为基于飞书的实时通信系统，提供：

- 📬 实时接收Claude Code任务完成/等待输入通知
- ⌨️ 通过飞书消息远程发送命令到Claude Code会话
- 📊 查看和管理活跃会话
- 🔒 基于飞书用户的安全验证机制
- 🎯 交互式卡片界面

## 架构设计

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Claude Code    │───▶│   Webhook服务    │───▶│   飞书机器人     │
│     会话        │    │   (接收通知)     │    │   (发送通知)     │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                 │                       │
                       ┌─────────▼──────────┐           │
                       │   会话管理服务      │           │
                       │ (令牌生成/映射)     │           │
                       └────────────────────┘           │
                                 ▲                       │
┌─────────────────┐    ┌─────────┴──────────┐           │
│    tmux会话     │◀───│   命令中继服务      │◀──────────┘
│                 │    │ (解析消息/执行命令) │
└─────────────────┘    └────────────────────┘
```

## 项目结构

```
feishu-bot/
├── cmd/                    # 应用程序入口
│   ├── bot/               # 主机器人服务
│   ├── webhook/           # Claude Code webhook接收器
│   └── relay/             # 命令中继服务
├── internal/              # 内部包
│   ├── bot/               # 机器人逻辑
│   │   ├── client/        # 飞书客户端封装
│   │   └── handlers/      # 事件处理器
│   ├── session/           # 会话管理
│   ├── command/           # 命令执行
│   ├── notification/      # 通知服务
│   ├── security/          # 安全验证
│   └── config/            # 配置管理
├── configs/               # 配置文件
│   ├── config.yaml        # 主配置
│   ├── cards/             # 卡片模板
│   └── security/          # 用户权限配置
└── data/                  # 运行时数据
    ├── sessions.json      # 会话映射
    └── logs/              # 日志文件
```

## 核心功能

### 1. 会话管理
- 8位唯一令牌生成 (如: `ABC12345`)
- 会话与 tmux 的映射关系
- 24小时自动过期机制
- 定期清理过期会话

### 2. 通知系统
- 接收 Claude Code 的任务状态通知
- 发送交互式飞书卡片
- 支持任务完成/等待输入/错误等状态

### 3. 命令执行
- 解析飞书消息中的远程命令
- 格式: `<令牌>: <命令>` (如: `ABC12345: run tests`)
- 多层回退执行策略 (tmux → fallback)
- 命令安全验证

### 4. 用户界面
- 交互式卡片 (任务通知、命令结果等)
- 便捷命令 (`/sessions`, `/help`)
- 实时状态反馈

## 快速开始

### 1. 环境配置

复制环境变量模板：
```bash
cp .env.example .env
```

编辑 `.env` 文件，配置飞书应用信息：
```bash
FEISHU_APP_ID=your_app_id
FEISHU_APP_SECRET=your_app_secret
WEBHOOK_PORT=8080
SESSION_STORAGE_FILE=data/sessions.json
LOG_LEVEL=info
```

### 2. 用户权限配置

#### 获取您的飞书 ID
1. 打开飞书客户端
2. 点击头像 -> 设置 -> 我的信息
3. 复制您的用户 ID（格式如：`ou_xxxxxxxx`）

#### 编辑用户白名单
编辑 `configs/security/whitelist.yaml`：
```yaml
allowed_users:
  - user_id: "g12da5gf"                               # 您的飞书用户ID（短格式）
    open_id: "ou_8a4ad14f0daec82d332888e5ee31ad82"      # 您的飞书OpenID（长格式）
    name: "您的姓名"                              # 可选，方便管理
    permissions:
      - "command_execute"                            # 命令执行权限
      - "session_manage"                             # 会话管理权限
    max_sessions: 5                                  # 最大并发会话数

# 管理员用户（拥有所有权限）
admin_users:
  - "ou_8a4ad14f0daec82d332888e5ee31ad82"           # 使用 OpenID

# 全局限制
global_limits:
  max_total_sessions: 50                             # 系统最大会话数
  max_session_duration_hours: 48                     # 最大会话持续时间
```

**重要提示：**
- `user_id` 和 `open_id` 可以相同（都使用 OpenID）
- 系统会自动处理 user_id 到 open_id 的映射
- 请确保 OpenID 正确，否则无法接收通知

### 3. 构建和运行

```bash
# 安装依赖
make deps

# 构建所有服务
make build

# 运行主机器人
make run-bot

# 运行webhook服务 (另一个终端)
make run-webhook

# 运行命令中继服务 (可选)
make run-relay
```

### 4. Claude Code Hooks 配置

#### 第一步：设置环境变量
在您的 shell 配置文件（`~/.bashrc`, `~/.zshrc` 或 `~/.profile`）中添加：
```bash
# 飞书机器人配置
export FEISHU_USER_ID="your_feishu_user_id"     # 从 whitelist.yaml 中查找
export FEISHU_OPEN_ID="your_feishu_open_id"     # 从 whitelist.yaml 中查找
```

**重要说明：**
- `FEISHU_USER_ID` 和 `FEISHU_OPEN_ID` 必须与 `configs/security/whitelist.yaml` 中的配置一致
- 如果不设置环境变量，系统会使用占位符值（如 "your_open_id"）
- 系统会自动将占位符解析为真实的 OpenID（通过 user_id 查找）

#### 第二步：修改 Claude Code 配置
修改 `~/.claude/settings.json`，添加 hooks：
```json
{
  "hooks": {
    "Stop": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "curl -X POST http://localhost:8080/webhook/notification -H 'Content-Type: application/json' -d '{\"type\":\"completed\",\"user_id\":\"'$FEISHU_USER_ID'\",\"open_id\":\"'$FEISHU_OPEN_ID'\",\"project_name\":\"{{project}}\",\"description\":\"Task completed\",\"working_dir\":\"{{cwd}}\",\"tmux_session\":\"claude-code\"}'\n\necho \"Feishu notification sent for user $FEISHU_USER_ID\""
      }]
    }],
    "SubagentStop": [{
      "matcher": "*",
      "hooks": [{
        "type": "command",
        "command": "curl -X POST http://localhost:8080/webhook/notification -H 'Content-Type: application/json' -d '{\"type\":\"waiting\",\"user_id\":\"'$FEISHU_USER_ID'\",\"open_id\":\"'$FEISHU_OPEN_ID'\",\"project_name\":\"{{project}}\",\"description\":\"Waiting for input\",\"working_dir\":\"{{cwd}}\",\"tmux_session\":\"claude-code\"}'\n\necho \"Feishu notification sent for user $FEISHU_USER_ID\""
      }]
    }]
  }
}
```

#### 错误排查
如果您遇到错误：
```
Invalid ids: [your_open_id] . Please see field_violations for details
```

说明：
1. 环境变量未正确设置，或
2. `whitelist.yaml` 中的 user_id 和 open_id 不匹配

解决方案：
- 检查 `echo $FEISHU_USER_ID $FEISHU_OPEN_ID` 确认环境变量正确
- 检查 `configs/security/whitelist.yaml` 中的用户配置
- 重新启动 Claude Code 以加载新的环境变量

## 使用方法

### 1. 基本命令

- **远程命令**: `ABC12345: git status`
- **查看会话**: `/sessions`
- **获取帮助**: `/help`

### 2. 工作流程

1. 启动 Claude Code 并执行长时间任务
2. 任务完成/等待输入时，自动收到飞书通知卡片
3. 卡片中包含8位令牌 (如: `ABC12345`)
4. 发送消息 `ABC12345: <你的命令>` 来远程控制
5. 实时接收命令执行结果

### 3. 交互式卡片

#### 任务完成通知
```
🎉 任务执行完成
项目: MyProject
完成时间: 2024-01-15 14:30:05
令牌: ABC12345

[📝 继续工作] [📊 查看状态] [❌ 结束]
```

#### 等待输入通知
```
⏳ 等待用户输入
项目: MyProject  
当前任务: 代码重构
令牌: XYZ67890

请输入下一步操作:
[输入框____________________]
[💬 发送命令] [📋 查看选项] [❌ 结束]
```

## 开发指南

### 构建命令

```bash
make build          # 构建所有应用
make run-bot         # 运行机器人服务
make run-webhook     # 运行webhook服务  
make run-relay       # 运行命令中继服务
make clean           # 清理构建文件
make test            # 运行测试
make dev-setup       # 设置开发环境
```

### 配置说明

主要配置文件：

- `configs/config.yaml`: 主配置文件
- `configs/security/whitelist.yaml`: 用户权限配置
- `.env`: 环境变量配置

### 日志系统

日志文件位置：
- `data/logs/bot.log`: 主机器人日志
- `data/logs/webhook.log`: Webhook服务日志
- `data/logs/command.log`: 命令执行日志

## 安全特性

1. **用户白名单**: 只有配置文件中的用户可以使用
2. **令牌验证**: 8位唯一令牌，24小时自动过期
3. **命令过滤**: 危险命令自动拦截
4. **速率限制**: 防止命令滥用
5. **会话隔离**: 每个用户的会话相互独立

## 故障排除

### 常见问题

1. **机器人无响应**
   - 检查飞书应用配置是否正确
   - 确认WebSocket连接状态
   - 查看日志文件

2. **命令执行失败**
   - 确认tmux会话是否存在
   - 检查用户权限配置
   - 验证令牌是否有效

3. **通知不发送**
   - 检查webhook服务是否运行
   - 确认Claude Code hooks配置
   - 查看webhook日志

### 日志排查

```bash
# 查看实时日志
tail -f data/logs/bot.log

# 搜索错误信息
grep "ERROR" data/logs/*.log

# 查看会话状态
curl http://localhost:8080/webhook/stats
```

## 开发状态

✅ 已完成:
- [x] 项目结构和基础配置
- [x] 核心数据结构设计
- [x] 会话管理模块
- [x] Webhook服务
- [x] 飞书Bot服务基础功能
- [x] 命令中继服务框架
- [x] 卡片模板和交互处理
- [x] 配置文件管理
- [x] 编译错误修复

⏳ 待完成:
- [ ] 安全和权限验证模块完整实现
- [ ] 完整的集成测试
- [ ] 生产环境部署配置
- [ ] 监控和告警系统

## 贡献指南

1. Fork 项目
2. 创建功能分支
3. 提交更改
4. 发起 Pull Request

## 许可证

MIT License

## 支持

如有问题或建议，请创建 Issue 或联系开发团队。