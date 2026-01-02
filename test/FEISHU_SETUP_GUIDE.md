# 飞书应用配置指南 - CardKit 2.0 流式更新验证

## 第一步：创建飞书自建应用

### 1.1 访问飞书开放平台

打开浏览器，访问：https://open.feishu.cn/app

### 1.2 创建应用

1. 点击"创建自建应用"
2. 填写应用信息：
   - **应用名称**：Claude Stream Bot（或你喜欢的名字）
   - **应用描述**：Claude Code 流式对话机器人
   - **应用图标**：可选（上传一个你喜欢的图标）

### 1.3 获取凭证

创建成功后，进入应用详情页：

1. 在左侧菜单找到 **"凭证与基础信息"**
2. 复制以下信息（后续需要用到）：
   - **App ID**：格式如 `cli_a8058428d478501c`
   - **App Secret**：点击"查看"并复制

**保存这些信息！** 我们稍后会在 `.env` 文件中使用。

---

## 第二步：配置应用权限

### 2.1 打开权限管理

在应用详情页，找到左侧菜单的 **"权限管理"**

### 2.2 搜索并添加以下权限

**必需权限**：

1. **im:message** - 获取与发送消息
   - 勾选：`im:message` (获取与发送消息)
   - 勾选：`im:message:group_at_msg` (接收群组 @ 消息)
   - 勾选：`im:message:send_as_bot` (以应用身份发送消息)

2. **im:chat** - 聊天信息
   - 勾选：`im:chat` (获取群聊信息)

3. **contact:user.base:readonly** - 用户信息（可选）
   - 勾选：`contact:user.base:readonly` (获取用户基本信息)

4. **card:card** - 卡片操作（CardKit 2.0 需要的权限）
   - 搜索 `card`
   - 勾选所有相关权限

### 2.3 发布权限

1. 点击右上角 **"发布"** 或 **"申请权限"**
2. 选择 **"全员可使用"** 或指定测试用户
3. 点击 **"确定"**

---

## 第三步：配置事件订阅

### 3.1 打开事件订阅

在应用详情页，找到左侧菜单的 **"事件订阅"**

### 3.2 订阅消息事件

1. 点击 **"添加事件"**
2. 勾选以下事件：
   - **im.message.receive_v1** - 接收消息事件
   - **im.message.message_read_v1** - 消息已读（可选）

3. 配置 **请求地址**：
   - 填写：`https://your-domain.com/webhook`
   - 或者暂时使用 ngrok 等内网穿透工具进行测试

### 3.3 加密验证（可选但推荐）

1. 勾选 **"验证加密"**
2. 系统会生成一个 **Encrypt Key**（加密密钥）
3. 复制并保存这个密钥（用于验证 Webhook 签名）

---

## 第四步：配置环境变量

### 4.1 创建 .env 文件

在项目根目录创建 `.env` 文件：

```bash
cd /Users/wen/Desktop/code/18feishu/feishu-bot/feishu-bot
cp .env.example .env
```

### 4.2 填写配置

编辑 `.env` 文件，填入之前保存的信息：

```bash
# Feishu Application Configuration
FEISHU_APP_ID=cli_a8058428d478501c  # 替换为你的 App ID
FEISHU_APP_SECRET=your_app_secret_here  # 替换为你的 App Secret

# Claude Code Hook Configuration
FEISHU_USER_ID=xxxx  # 可选，先留空
FEISHU_OPEN_ID=ou_xxxxx  # 可选，先留空

# Server Configuration
PORT=8080

# CardKit Test Configuration（用于测试）
FEISHU_TEST_CHAT_ID=oc_xxxxx  # 测试群聊的 Chat ID，后续会说明如何获取
```

### 4.3 保存文件

保存 `.env` 文件并确保它在 `.gitignore` 中（不会提交到 Git）。

---

## 第五步：获取测试群聊 ID

### 5.1 创建测试群聊

1. 在飞书客户端创建一个测试群聊
2. 邀请你的机器人应用加入群聊
3. 找到群聊设置 → 群信息 → 群 ID

**方法 1：通过飞书客户端**
- 右键点击群聊 → 群设置 → 群信息
- 找到 **"群 ID"**（格式：`oc_xxxxxxxx`）

**方法 2：通过 API**
- 使用飞书 API 列出所有群聊
- 找到测试群的 Chat ID

### 5.2 保存 Chat ID

将 Chat ID 添加到 `.env` 文件：
```bash
FEISHU_TEST_CHAT_ID=oc_xxxxxxxx  # 你的测试群聊 ID
```

---

## 第六步：验证配置

### 6.1 测试程序

我已为你创建了测试程序 `test/poc_feishu_api.go`。

**注意**：这个程序需要一些修复（导入问题），让我创建一个简化版本：

### 6.2 使用 curl 快速测试

**测试 1：获取 tenant_access_token**

```bash
curl -X POST "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal" \
  -H "Content-Type: application/json" \
  -d '{
    "app_id": "cli_a8058428d478501c",
    "app_secret": "your_app_secret_here"
  }'
```

期望返回：
```json
{
  "code": 0,
  "tenant_access_token": "t-xxxxxxxxxxxx",
  "expire": 7200
}
```

**测试 2：创建流式卡片**

```bash
curl -X POST "https://open.feishu.cn/open-apis/im/v1/messages" \
  -H "Authorization: Bearer <tenant_access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "receive_id": "oc_xxxxxxxx",
    "msg_type": "interactive",
    "content": "{\"schema\":\"2.0\",\"config\":{\"wide_screen_mode\":true,\"streaming_mode\":true,\"update_multi\":true},\"elements\":[{\"tag\":\"markdown\",\"element_id\":\"reply_content\",\"uuid\":\"test-uuid-123\",\"content\":\"思考中...\"}]}"
  }'
```

期望返回：
```json
{
  "code": 0,
  "data": {
    "message_id": "om_xxxxxxxx"
  }
}
```

**保存返回的 `message_id`，这是你的 Card ID！**

**测试 3：流式更新卡片**

```bash
curl -X PUT "https://open.feishu.cn/open-apis/cardkit/v1/cards/<card_id>/elements/reply_content/content" \
  -H "Authorization: Bearer <tenant_access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "uuid": "test-uuid-123",
    "content": "Hello from CardKit!",
    "sequence": 1
  }'
```

期望返回：
```json
{
  "code": 0
}
```

如果成功，你应该能在飞书群聊中看到卡片内容从"思考中..."变为 "Hello from CardKit!"

---

## 第七步：验证打字机效果

### 7.1 手动模拟流式更新

多次调用流式更新 API，每次增加一点内容：

```bash
# 更新 1
curl -X PUT "https://open.feishu.cn/open-apis/cardkit/v1/cards/<card_id>/elements/reply_content/content" \
  -H "Authorization: Bearer <tenant_access_token>" \
  -H "Content-Type: application/json" \
  -d '{"uuid":"test-uuid-123","content":"Hello","sequence":1}'

# 等待 0.5 秒

# 更新 2
curl -X PUT "https://open.feishu.cn/open-apis/cardkit/v1/cards/<card_id>/elements/reply_content/content" \
  -H "Authorization: Bearer <tenant_access_token>" \
  -H "Content-Type: application/json" \
  -d '{"uuid":"test-uuid-123","content":"Hello there","sequence":2}'

# 等待 0.5 秒

# 更新 3
curl -X PUT "https://open.feishu.cn/open-apis/cardkit/v1/cards/<card_id>/elements/reply_content/content" \
  -H "Authorization: Bearer <tenant_access_token>" \
  -H "Content-Type: application/json" \
  -d '{"uuid":"test-uuid-123","content":"Hello there!","sequence":3}'
```

### 7.2 观察效果

在飞书群聊中，你应该能看到：
1. 卡片初始显示"思考中..."
2. 第一次更新后显示 "Hello"
3. 第二次更新后显示 "Hello there"
4. 第三次更新后显示 "Hello there!"

**这就是打字机效果！** ✨

---

## 常见问题

### Q1: 权限不足错误

**错误信息**：`code 99991663: app has no permission`

**解决方案**：
1. 检查权限管理中是否勾选了所有必需权限
2. 确保已经点击"发布"按钮
3. 等待几分钟让权限生效

### Q2: CardKit API 不存在

**错误信息**：`code 404: api not found`

**可能原因**：
1. CardKit 2.0 API 可能还未完全开放
2. 需要申请白名单或特殊权限

**备选方案**：使用传统的 `im.message.patch` API（稍后说明）

### Q3: Sequence 冲突

**错误信息**：`sequence must be strictly increasing`

**解决方案**：
- 确保 sequence 从 1 开始
- 每次更新都必须 +1
- 不能重复或跳过数字

### Q4: UUID 不匹配

**错误信息**：`uuid mismatch`

**解决方案**：
- 确保更新时使用的 UUID 与创建卡片时一致
- UUID 必须在整个会话中保持不变

---

## 下一步

### 如果 CardKit API 可用 ✅

恭喜！你可以继续开发完整的流式对话系统：

1. 实现缓冲和限流器
2. 集成 Claude CLI 进程
3. 端到端测试

### 如果 CardKit API 不可用 ⚠️

不要担心！我们可以使用备选方案：

**方案 A：使用消息 Patch API**
```bash
PATCH /open-apis/im/v1/messages/{message_id}
```

虽然不如 CardKit 2.0 高效，但也能实现类似的流式效果。

**方案 B：创建多个消息**
- 每次更新创建新消息
- 适合短文本输出

---

## 快速检查清单

- [ ] 创建了飞书自建应用
- [ ] 获取了 App ID 和 App Secret
- [ ] 配置了必需权限（im:message, card:card 等）
- [ ] 发布了权限
- [ ] 创建了测试群聊
- [ ] 获取了群聊 Chat ID
- [ ] 配置了 .env 文件
- [ ] 测试了获取 token API
- [ ] 测试了创建卡片 API
- [ ] 测试了流式更新 API
- [ ] 验证了打字机效果

完成以上所有步骤后，告诉我结果，我们继续下一步！🚀
