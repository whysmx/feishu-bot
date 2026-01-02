# 飞书机器人测试指南

## 📋 测试前检查清单

### 1. 确认配置已完成

- [ ] 飞书开放平台事件订阅已配置
  - 订阅方式：长连接
  - 已添加事件：im.message.receive_v1

- [ ] 权限已开启
  - im:message
  - im:message.group_at_msg
  - im:chat
  - cardkit:card:write

- [ ] 机器人已启动
  - 进程运行中
  - WebSocket 已连接

### 2. 机器人进程检查

```bash
# 检查进程
ps aux | grep "[b]ot/main"

# 查看日志
tail -f /tmp/feishu-bot.log

# 应该看到
[Info] connected to wss://msg-frontier.feishu.cn/ws/v2
```

## 🧪 测试方法

### 方法 1：飞书客户端测试（推荐）

#### 群聊测试
1. 打开测试群（cc1.0）
2. 输入：`@机器人 你好`
3. 观察是否创建卡片并流式回复

#### 单聊测试
1. 打开机器人对话
2. 输入：`你好`
3. 观察是否创建卡片并流式回复

### 方法 2：网页版 Messenger 测试

**访问地址**：https://feishu.cn/next/messenger

**步骤**：
1. 在浏览器中打开飞书网页版
2. 找到测试群或机器人对话
3. 发送：`@机器人 你好`（群聊）或 `你好`（单聊）

### 方法 3：飞书移动端测试

1. 打开飞书 App
2. 进入测试群
3. @机器人 发送：`你好`

## ✅ 成功的标志

### 机器人日志应该显示：
```
[OnP2MessageReceiveV1] Message received: ...
Processing streaming chat from user ...
Creating CardKit streaming card
[Content delta] sequence=1: 你
[Content delta] sequence=2: 你好
```

### 飞书界面应该显示：
1. 创建新的对话卡片
2. 卡片标题："Claude 对话"
3. 内容逐字显示（打字机效果）
4. 完整显示 AI 回复

## ❌ 失败的排查

### 情况 1：机器人无响应

**检查**：
```bash
# 1. 检查进程
ps aux | grep "[b]ot/main"

# 2. 检查 WebSocket 连接
grep "connected" /tmp/feishu-bot.log

# 3. 检查是否有错误
grep -i "error" /tmp/feishu-bot.log
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
tail -50 /tmp/feishu-bot.log

# 查找错误
grep -A 5 "Message received" /tmp/feishu-bot.log
```

**常见原因**：
- Claude CLI 未安装或配置错误
- chat_id 配置错误
- token 获取失败
- CardKit API 调用失败

### 情况 3：流式输出卡住

**检查**：
```bash
# 查看 Claude CLI 相关日志
grep "claude\|Claude\|stream" /tmp/feishu-bot.log

# 检查 CardKit 更新
grep "CardKit\|UpdateContent" /tmp/feishu-bot.log
```

**常见原因**：
- 超过 CardKit 限流（10 QPS）
- token 过期
- Claude CLI 进程卡死

## 🔧 调试命令

### 实时监控日志
```bash
tail -f /tmp/feishu-bot.log
```

### 搜索关键事件
```bash
# 消息接收
grep "Message received" /tmp/feishu-bot.log

# 流式对话
grep "streaming chat" /tmp/feishu-bot.log

# CardKit 操作
grep "CardKit" /tmp/feishu-bot.log

# 错误信息
grep -i "error\|fail" /tmp/feishu-bot.log
```

### 重启机器人
```bash
# 停止旧进程
pkill -f "bot/main"

# 启动新进程
cd /Users/wen/Desktop/code/18feishu/feishu-bot
go run cmd/bot/main.go > /tmp/feishu-bot.log 2>&1 &
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
- [ ] CardKit 打字机效果正常

### 问题描述
（如有问题，记录详细描述）

### 日志片段
（附上相关日志）

---

**下一步**：
- 如果所有测试通过 → 进入功能优化阶段
- 如果有失败 → 根据错误信息排查问题
