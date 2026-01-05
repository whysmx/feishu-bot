# 飞书机器人测试指南

## 📋 测试前检查清单

### 1. 确认配置已完成

- [ ] 飞书开放平台事件订阅已配置
  - 订阅方式：长连接
  - 已添加事件：im.message.receive_v1

- [ ] 权限已开启
  - im:message
  - im:message.group_at_msg（群聊命令）

- [ ] 机器人已启动
  - 进程运行中
  - WebSocket 已连接

### 2. 机器人进程检查

```bash
# 检查进程
ps aux | grep "[b]ot/main"

# 查看日志
log_file=/tmp/feishu-bot-latest.log
[[ -f /tmp/feishu-bot.log ]] && log_file=/tmp/feishu-bot.log

tail -f "$log_file"

# 应该看到
[Info] connected to wss://msg-frontier.feishu.cn/ws/v2
```

## 🧪 测试方法

### 方法 1：飞书客户端测试（推荐）

#### 群聊测试
1. 打开测试群
2. 输入：`你好`
3. 观察是否分段回复文本消息
4. 如果未收到回复，请改用 `@机器人 你好` 进行测试

#### 群聊命令测试（需 @ 机器人）
1. 输入：`@机器人 ls`
2. 输入：`@机器人 bind 1`
3. 输入：`@机器人 help`

#### 单聊测试
1. 打开机器人对话
2. 输入：`你好`
3. 观察是否分段回复文本消息

### 方法 2：网页版 Messenger 测试

**访问地址**：https://feishu.cn/next/messenger

**步骤**：
1. 在浏览器中打开飞书网页版
2. 找到测试群或机器人对话
3. 发送：`你好`（群聊或单聊）

### 方法 3：飞书移动端测试

1. 打开飞书 App
2. 进入测试群
3. 发送：`你好`

## ✅ 成功的标志

### 机器人日志应该显示：
```
[OnP2MessageReceiveV1] Message received: ...
[MessageHandler] Received GROUP message: ...
[StreamingTextHandler] Processing message with time-based streaming mode: receive_id=... type=chat_id project_dir=...
[ClaudeManager] Starting claude command: ...
```

### 飞书界面应该显示：
1. 机器人按段发送多条文本消息
2. 输出逐步显示，直到完整回复
3. 群聊命令返回文本结果

## ❌ 失败的排查

### 情况 1：机器人无响应

**检查**：
```bash
# 1. 检查进程
ps aux | grep "[b]ot/main"

# 2. 检查 WebSocket 连接
grep "connected" "$log_file"

# 3. 检查是否有错误
grep -i "error" "$log_file"
```

**常见原因**：
- 机器人进程未运行
- 事件未配置（im.message.receive_v1）
- 权限未开启
- WebSocket 连接失败

### 情况 2：收到消息但不回复

**检查**：
```bash
# 查看详细日志
tail -50 "$log_file"

# 查找错误
grep -A 5 "Message received" "$log_file"
```

**常见原因**：
- Claude CLI 未安装或路径错误
- `ANTHROPIC_API_KEY` 或 `ANTHROPIC_AUTH_TOKEN` 缺失
- tenant token 获取失败
- 消息发送失败

### 情况 3：流式输出卡住

**检查**：
```bash
# 查看 Claude CLI 相关日志
grep "ClaudeManager\|StreamingTextHandler" "$log_file"
```

**常见原因**：
- Claude CLI 进程卡死
- CLI 输出中断或网络异常
- 飞书消息发送失败导致中断

## 🔧 调试命令

### 实时监控日志
```bash
tail -f "$log_file"
```

### 搜索关键事件
```bash
# 消息接收
grep "Message received" "$log_file"

# 流式处理
grep "StreamingTextHandler" "$log_file"

# Claude CLI
grep "ClaudeManager" "$log_file"

# 错误信息
grep -i "error\|fail" "$log_file"
```

### 重启机器人
```bash
# 停止旧进程
pkill -f "bot/main"

# 启动新进程
cd /Users/wen/Desktop/code/18feishu

go run ./cmd/bot > /tmp/feishu-bot.log 2>&1 &
```

## 📝 测试报告模板

### 测试时间
- 日期：
- 测试人：

### 测试环境
- 机器人版本：
- 飞书应用 ID：
- WebSocket 连接状态：

### 测试结果
- [ ] 机器人启动成功
- [ ] WebSocket 连接成功
- [ ] 群聊消息接收成功
- [ ] 单聊消息接收成功
- [ ] 流式对话功能正常
- [ ] 群聊命令响应正常

### 问题描述
（如有问题，记录详细描述）

### 日志片段
（附上相关日志）

---

**下一步**：
- 如果所有测试通过 → 进入功能优化阶段
- 如果有失败 → 根据错误信息排查问题
