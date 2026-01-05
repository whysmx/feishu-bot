# Chrome MCP 测试指南

本文档记录使用 Chrome MCP 工具进行飞书 Bot 自动化测试的完整流程和经验总结。

## 目录
- [环境准备](#环境准备)
- [Chrome MCP 基础操作](#chrome-mcp-基础操作)
- [测试流程](#测试流程)
- [常见问题](#常见问题)
- [调试技巧](#调试技巧)

## 环境准备

### 1. 检查 Chrome MCP 连接

在开始测试前，确保 Chrome MCP 已正确连接：

```bash
# 在 Claude Code 中调用 Chrome MCP 工具
# 以下操作由 MCP 自动完成，无需手动执行
```

### 2. 准备测试环境

1. **启动 Bot**
```bash
# 编译 bot
go build -o /tmp/feishu-bot cmd/bot/main.go

# 启动 bot 并记录日志
/tmp/feishu-bot > /tmp/feishu-bot-test.log 2>&1 &

# 验证 bot 进程
ps aux | grep feishu-bot | grep -v grep
```

2. **打开飞书网页**
- 访问：https://icntc77e5rdp.feishu.cn/next/messenger
- 进入测试群聊

## Chrome MCP 基础操作

### 查看页面状态

```javascript
// 获取当前页面快照
mcp__chrome-devtools__take_snapshot()
```

**用途**：
- 查看页面元素和结构
- 获取元素的 uid 用于后续操作
- 检查消息是否发送成功

### 列出所有打开的页面

```javascript
// 列出所有打开的标签页
mcp__chrome-devtools__list_pages()
```

**返回示例**：
```
0: https://icntc77e5rdp.feishu.cn/next/messenger [selected]
1: https://app.feishu.cn/
2: https://open.feishu.cn/app/xxx/event
```

### 切换页面

```javascript
// 选择页面（pageIdx 从 0 开始）
mcp__chrome-devtools__select_page(pageIdx=0)
```

### 点击元素

```javascript
// 点击元素
mcp__chrome-devtools__click(uid=112_115)
```

**注意**：uid 需要先从 `take_snapshot()` 获取

### 输入文本

```javascript
// 在输入框中输入文本
mcp__chrome-devtools__fill(uid=112_115, value="你好")
```

### 按键操作

```javascript
// 发送消息
mcp__chrome-devtools__press_key(key="Enter")

// 全选
mcp__chrome-devtools__press_key(key="Control+A")

// 删除
mcp__chrome-devtools__press_key(key="Backspace")

// 输入 @ 符号
mcp__chrome-devtools__press_key(key="@")
```

### 执行 JavaScript

```javascript
// 设置输入框内容
mcp__chrome-devtools__evaluate_script(function=`() => {
  const input = document.querySelector('[contenteditable="true"]');
  if (input) {
    input.textContent = '@Claude Stream Bot 你好';
    input.focus();
    return true;
  }
  return false;
}`)
```

**用途**：
- 直接操作 DOM 元素
- 执行复杂的页面操作
- 设置特殊格式的内容

### 等待操作

```bash
# 等待页面加载或 bot 处理
sleep 3
```

## 测试流程

### 完整测试步骤

#### 1. 启动并连接

```javascript
// 1. 列出当前页面
mcp__chrome-devtools__list_pages()

// 2. 选择飞书聊天页面
mcp__chrome-devtools__select_page(pageIdx=0)

// 3. 查看页面状态
mcp__chrome-devtools__take_snapshot()
```

#### 2. 发送测试消息

**方法一：使用 fill + Enter**

```javascript
// 点击输入框
mcp__chrome-devtools__click(uid=112_115)

// 输入消息（注意：纯文本 @ 不会生成真实 mention）
mcp__chrome-devtools__fill(uid=112_115, value="@Claude Stream Bot 测试消息")

// 发送
mcp__chrome-devtools__press_key(key="Enter")
```

**⚠️ 重要提醒（事件推送必看）**

- 仅用 `fill` 或粘贴文本 `@Claude Stream Bot` 不会生成 mention 实体
- 没有 mention 实体时，`im.message.receive_v1` 不会触发
- 必须通过 **按 `@` 并从候选列表点击机器人** 的方式生成 mention

**方法二：使用 JavaScript**

```javascript
// 直接设置内容
mcp__chrome-devtools__evaluate_script(function=`() => {
  const input = document.querySelector('[contenteditable="true"]');
  if (input) {
    input.textContent = '@Claude Stream Bot 测试';
    input.focus();
    return true;
  }
  return false;
}`)

// 按 Enter 发送
mcp__chrome-devtools__press_key(key="Enter")
```

**方法三：模拟键盘输入（推荐，确保生成真实 mention）**

```javascript
// 清空输入
mcp__chrome-devtools__press_key(key="Control+A")
mcp__chrome-devtools__press_key(key="Delete")

// 输入 @
mcp__chrome-devtools__press_key(key="@")

// 点击机器人建议（从弹出的候选列表选择机器人）
mcp__chrome-devtools__click(uid=122_132)

// 输入空格和消息内容
mcp__chrome-devtools__press_key(key="Space")
// ... 继续输入
```

#### 3. 等待并检查结果

```javascript
// 等待 bot 处理
// (在 bash 中执行)
sleep 5

// 检查 bot 日志
// (在 bash 中执行)
tail -100 /tmp/feishu-bot-test.log

// 查看页面消息
mcp__chrome-devtools__take_snapshot()
```

## 查看和检查 Bot 日志

### 1. 实时查看日志

```bash
# 实时跟踪日志
tail -f /tmp/feishu-bot-test.log

# 查看最新日志
tail -100 /tmp/feishu-bot-test.log

# 查看特定时间的日志
tail -200 /tmp/feishu-bot-test.log | grep "11:58"
```

### 2. 检查关键信息

**启动日志**：
```
2026/01/02 11:59:51 Starting Feishu Bot...
2026/01/02 11:59:51 Loaded .env
2026/01/02 11:59:51 Starting WebSocket connection to Feishu...
[Info] connected to wss://msg-frontier.feishu.cn/ws/v2
```

**消息接收日志**：
```
[MessageHandler] Received group message: {...}
[DEBUG] Original extracted content: '@Claude Stream Bot 测试'
[DEBUG] After cleaning: content='测试'
```

**处理日志**：
```
[MessageHandler] Processing streaming chat from user xxx: 测试
[HandleMessage] Processing message from chat xxx
Card created and sent: card_id=xxx
```

### 3. 检查进程状态

```bash
# 查看 bot 进程
ps aux | grep feishu-bot | grep -v grep

# 查看所有 go run 进程
ps aux | grep "go run" | grep -v grep

# 杀死所有 bot 进程
pkill -9 -f feishu-bot
```

## 常见问题

### 问题 1: Bot 没有响应

**检查步骤**：

1. **确认 bot 进程运行**
```bash
ps aux | grep feishu-bot
```

2. **检查 WebSocket 连接**
```bash
tail -50 /tmp/feishu-bot-test.log | grep "connected"
```

应该看到：`[Info] connected to wss://msg-frontier.feishu.cn/ws/v2`

3. **检查事件订阅**
- 访问：https://open.feishu.cn/app/cli_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx/event
- 确认"已添加事件"中有 `im.message.receive_v1`

4. **确认是真实 @mention**
- 必须“按 `@` 并从候选列表选择机器人”
- 纯文本 `@Claude Stream Bot` 不会触发事件

5. **检查事件日志**
- 访问：https://open.feishu.cn/app/cli_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx/logs?tab=event
- 查看是否有事件推送记录

### 问题 2: 消息发送失败

**可能原因**：

1. **输入框未获得焦点**
```javascript
// 先点击输入框
mcp__chrome-devtools__click(uid=xxx)
```

2. **消息格式不正确**
- 确保包含 `@机器人` 前缀
- 检查是否有多余的空格或换行

3. **页面未加载完成**
```bash
# 等待页面加载
sleep 3
```

### 问题 3: @Mention 处理问题

**现象**：Bot 收到的消息内容不正确

**调试方法**：

1. 查看原始提取内容日志：
```bash
grep "Original extracted content" /tmp/feishu-bot-test.log
```

2. 查看清理后内容：
```bash
grep "After cleaning" /tmp/feishu-bot-test.log
```

3. 检查 `cleanMentionContent` 函数逻辑（`message.go:283`）

### 问题 4: 多个 Bot 进程冲突

**症状**：修改代码后没有生效

**解决方法**：

```bash
# 1. 杀死所有 bot 进程
pkill -9 -f feishu-bot
pkill -9 -f "go run"

# 2. 确认进程已清理
ps aux | grep feishu-bot

# 3. 重新编译并启动
go build -o /tmp/feishu-bot cmd/bot/main.go
/tmp/feishu-bot > /tmp/feishu-bot-new.log 2>&1 &

# 4. 验证新进程
ps aux | grep feishu-bot
tail -20 /tmp/feishu-bot-new.log
```

## 调试技巧

### 1. 添加调试日志

在 `message.go` 中添加：

```go
// 在 HandleGroupMessage 函数中
mh.logger.Printf("[DEBUG] Original extracted content: '%s'", content)
content = mh.cleanMentionContent(content)
mh.logger.Printf("[DEBUG] After cleaning: content='%s'", content)
```

### 2. 使用不同的日志文件

```bash
# 每次测试使用不同的日志文件
/tmp/feishu-bot > /tmp/feishu-bot-$(date +%H%M%S).log 2>&1 &
```

### 3. 实时对比

同时打开多个终端窗口：

```bash
# 终端 1: 实时查看日志
tail -f /tmp/feishu-bot-test.log

# 终端 2: 查看 WebSocket 连接
netstat -an | grep 7590

# 终端 3: 查看进程状态
watch -n 1 'ps aux | grep feishu-bot'
```

### 4. 截图对比

使用 Chrome MCP 截图功能：

```javascript
// 发送消息前
mcp__chrome-devtools__take_snapshot()

// 发送消息
// ...

// 等待响应
sleep 5

// 发送消息后
mcp__chrome-devtools__take_snapshot()
```

对比两次快照，查看是否有新消息出现。

### 5. 检查飞书平台状态

**事件订阅页面**：
- URL: https://open.feishu.cn/app/cli_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx/event
- 检查项：
  - 订阅方式：长连接
  - 已添加事件：im.message.receive_v1
  - 权限状态：已开通

**事件日志页面**：
- URL: https://open.feishu.cn/app/cli_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx/logs?tab=event
- 检查项：
  - 事件类型：im.message.receive_v1
  - 返回状态：SUCCESS
  - 事件推送耗时

**版本管理页面**：
- URL: https://open.feishu.cn/app/cli_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx/version
- 检查项：
  - 是否有发布版本
  - 当前修改状态

## 完整测试脚本示例

```javascript
// 1. 列出页面
mcp__chrome-devtools__list_pages()

// 2. 选择飞书页面
mcp__chrome-devtools__select_page(pageIdx=0)

// 3. 查看当前状态
mcp__chrome-devtools__take_snapshot()

// 4. 输入测试消息
mcp__chrome-devtools__evaluate_script(function=`() => {
  const input = document.querySelector('[contenteditable="true"]');
  if (input) {
    input.textContent = '@Claude Stream Bot 你好';
    input.focus();
    return true;
  }
  return false;
}`)

// 5. 发送消息
mcp__chrome-devtools__press_key(key="Enter")

// 6. 等待响应（在 bash 中）
sleep 5

// 7. 检查日志（在 bash 中）
tail -100 /tmp/feishu-bot-test.log

// 8. 查看页面状态
mcp__chrome-devtools__take_snapshot()
```

## 性能优化建议

1. **批量操作**：一次发送多条测试消息，避免频繁切换

2. **并行检查**：同时在多个终端窗口查看不同日志

3. **日志筛选**：使用 grep 快速定位关键信息
```bash
# 只看错误
tail -f log | grep ERROR

# 只看消息处理
tail -f log | grep "Processing"
```

4. **自动化测试**：可以编写简单的测试脚本自动发送消息

## 总结

Chrome MCP 为飞书 Bot 测试提供了强大的自动化能力：

- ✅ 无需手动操作浏览器
- ✅ 可以精确定位页面元素
- ✅ 支持复杂的交互操作
- ✅ 便于日志收集和问题定位

关键点：
1. 熟练使用 `take_snapshot()` 获取元素 uid
2. 善用 `evaluate_script()` 处理复杂操作
3. 结合 bash 命令查看日志和进程状态
4. 遇到问题时系统性地检查：进程 -> 连接 -> 事件订阅 -> 平台日志

## 参考资料

- [Chrome DevTools Protocol](https://chromedevtools.github.io/devtools-protocol/)
- [飞书开放平台文档](https://open.feishu.cn/document)
- [项目 README](../README.md)
