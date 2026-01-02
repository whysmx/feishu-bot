# 飞书 × Claude Code 流式对话系统 - PoC 测试报告

## 测试日期
2026-01-01

## Phase 1: Claude CLI Stream-JSON 输出验证

### 测试结果: ✅ **通过**

### 测试过程
1. 创建了 Go 测试程序 `test/poc_claude.go`
2. 使用以下参数启动 Claude CLI：
   ```bash
   claude -p --output-format stream-json --include-partial-messages --verbose
   ```
3. 发送测试消息："Hello! Please say 'Hi there!' and nothing else."

### 关键发现

#### 1. JSON 结构格式
Claude CLI 的 stream-json 输出使用了 `stream_event` 包装：

```json
{
  "type": "stream_event",
  "event": {
    "type": "content_block_delta",
    "index": 0,
    "delta": {
      "type": "text_delta",
      "text": "Hi"
    }
  },
  "session_id": "...",
  "uuid": "..."
}
```

#### 2. 事件类型顺序
1. `system` - 系统初始化信息（包含session_id、工具列表等）
2. `stream_event` → `message_start` - 消息开始
3. `stream_event` → `content_block_start` - 内容块开始
4. `stream_event` → `content_block_delta` - 文本增量（**这是我们需要提取的**）
5. `stream_event` → `message_stop` - 消息结束

#### 3. NDJSON 解析器实现
成功实现了能够：
- 解析 `stream_event` 包装的 JSON
- 提取 `text_delta` 内容
- 过滤掉 `system` 等无关事件
- 实时输出文本（打字机效果）

### 测试输出
```
=== Claude CLI Output (stream-json) ===
Hi there!

=== Test Summary ===
Total lines processed: 10
Total output length: 9 characters
Time elapsed: 2.816437667s
Output: Hi there!
```

---

## Phase 2: 飞书 CardKit 流式更新 API 验证

### 测试状态: ⏳ **待验证（需要飞书配置）**

### 验证步骤

#### 步骤 1: 配置飞书应用

1. 在飞书开放平台创建自建应用
2. 获取 App ID 和 App Secret
3. 配置权限：
   - `im:message` - 发送消息
   - `im:message:group_at_msg` - 接收群消息
   - `card:card` - 卡片操作权限

#### 步骤 2: 创建流式卡片

使用以下配置创建 CardKit 2.0 卡片：

```json
{
  "schema": "2.0",
  "config": {
    "wide_screen_mode": true,
    "streaming_mode": true,
    "update_multi": true
  },
  "elements": [
    {
      "tag": "markdown",
      "element_id": "reply_content",
      "uuid": "<生成UUID>",
      "content": "思考中..."
    }
  ]
}
```

**关键配置说明**：
- `schema: "2.0"` - 必须使用 CardKit 2.0
- `streaming_mode: true` - 启用流式模式
- `update_multi: true` - 允许多次更新
- `element_id` - 用于局部更新的标识符
- `uuid` - 必须在创建时指定，后续更新需要使用

#### 步骤 3: 流式更新 API

**API 端点**：
```
PUT /open-apis/cardkit/v1/cards/{card_id}/elements/{element_id}/content
```

**请求体**：
```json
{
  "uuid": "<创建时指定的UUID>",
  "content": "更新后的完整文本",
  "sequence": 1
}
```

**参数说明**：
- `uuid` - 创建卡片时指定的 UUID
- `content` - **全量文本**（不是增量）
- `sequence` - 严格递增的整数

#### 步骤 4: 限流要求

- **频率限制**: 1000 次/分钟、50 次/秒
- **内容长度**: 1–100000 字符
- **卡片大小**: ≤ 30KB
- **Sequence**: 必须严格递增，重复或乱序会报错

### 手动测试方法

如果还没有配置飞书应用，可以使用以下方法手动测试：

1. **使用 curl 测试创建卡片**：
```bash
curl -X POST "https://open.feishu.cn/open-apis/im/v1/messages" \
  -H "Authorization: Bearer <tenant_access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "receive_id": "<chat_id>",
    "msg_type": "interactive",
    "content": "{\"schema\":\"2.0\",\"config\":{\"wide_screen_mode\":true,\"streaming_mode\":true,\"update_multi\":true},\"elements\":[{\"tag\":\"markdown\",\"element_id\":\"reply_content\",\"uuid\":\"test-uuid-123\",\"content\":\"思考中...\"}]}"
  }'
```

2. **使用 curl 测试流式更新**：
```bash
curl -X PUT "https://open.feishu.cn/open-apis/cardkit/v1/cards/<card_id>/elements/reply_content/content" \
  -H "Authorization: Bearer <tenant_access_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "uuid": "test-uuid-123",
    "content": "Hi there!",
    "sequence": 1
  }'
```

---

## Phase 3: 端到端集成测试

### 测试状态: ⏳ **待实施**

### 测试计划

#### 测试场景
1. 用户在飞书群聊中 @机器人："帮我写一个排序算法"
2. Middleware 接收消息并创建流式卡片
3. 启动 Claude CLI 进程
4. 实时解析 stream-json 输出
5. 每 100-150ms 或 500 字符更新一次飞书卡片
6. 验证打字机效果是否流畅

#### 成功标准
- ✅ Claude CLI 输出能实时显示在飞书卡片上
- ✅ Sequence 严格递增，无冲突
- ✅ API 调用频率 ≤ 50次/秒
- ✅ 卡片更新延迟 ≤ 500ms

---

## 已完成的工作

1. ✅ 克隆并分析了 `tqtcloud/feishu-bot` 项目
2. ✅ 验证了 Claude CLI stream-json 输出格式
3. ✅ 实现了 NDJSON 解析器
4. ✅ 确认了 `stream_event` 包装结构
5. ✅ 成功提取了 `text_delta` 内容
6. ✅ 验证了实时输出的可行性

---

## 下一步行动

### 立即可做（无需飞书配置）

1. **完善 NDJSON 解析器**
   - 添加对 `thinking_block_delta` 的过滤
   - 处理多轮对话场景
   - 实现更完善的错误处理

2. **实现缓冲和限流器**
   - 内存实现限流器（50次/秒、1000次/分钟）
   - 自适应缓冲策略（500字符或150ms）
   - Sequence 递增管理

3. **会话管理**
   - Session ID 生成和映射
   - 会话状态维护
   - 进程生命周期管理

### 需要飞书配置后

4. **配置飞书应用**
   - 创建飞书自建应用
   - 配置 Webhook 和权限
   - 获取 App ID 和 App Secret

5. **测试 CardKit API**
   - 创建流式卡片
   - 验证流式更新 API
   - 确认限级行为

6. **端到端集成**
   - 实现完整的消息流
   - 测试多用户并发
   - 性能优化

---

## 关键技术点总结

### Claude CLI Stream-JSON

✅ **已验证可行**
- 使用 `-p --output-format stream-json --include-partial-messages --verbose`
- 输出格式为 NDJSON（每行一个 JSON）
- 使用 `stream_event` 包装
- `text_delta` 包含实际的文本增量

### 飞书 CardKit 2.0

⏳ **待验证**（需要配置）
- `streaming_mode: true` 启用流式模式
- `update_multi: true` 允许多次更新
- `element_id` 用于局部更新
- `sequence` 必须严格递增

### 限流策略

✅ **已设计方案**
- 内存实现滑动窗口
- 维护 1秒窗口和 60秒窗口
- 自适应缓冲：500字符或150ms

---

## 问题记录

### 已解决

1. **Claude CLI 参数问题**
   - 问题：`--resume` 需要 UUID 格式
   - 解决：对于新会话不使用 `--resume`

2. **JSON 结构解析问题**
   - 问题：实际 JSON 有 `stream_event` 包装
   - 解决：更新解析器处理外层包装

3. **网络依赖问题**
   - 问题：无法下载 `github.com/google/uuid`
   - 解决：使用硬编码 UUID 测试

### 待解决

1. **飞书 API 验证**
   - 需要配置飞书应用
   - 需要验证 CardKit 2.0 API 是否可用
   - 需要测试限级行为

2. **并发安全**
   - Sequence 严格递增的并发控制
   - 会话映射的并发访问
   - 进程池的管理

---

## 结论

### Phase 1 PoC 结论: ✅ **成功**

Claude CLI 的 stream-json 输出可以成功解析和提取，实时打字机效果可行。

### Phase 2 PoC 建议: **需要用户配置**

需要配置飞书应用来验证 CardKit 2.0 流式更新 API。如果 API 不可用或不稳定，可以回退到消息 patch 模式。

### 继续开发的建议

**可以继续开发核心模块**（不依赖飞书配置）：
1. 完善缓冲和限流器
2. 实现会话管理
3. Webhook 接收处理
4. Claude CLI 进程管理

**等待飞书配置后**：
5. 测试 CardKit API
6. 端到端集成
7. 性能优化

---

## 附录

### 测试文件

1. `test/poc_claude.go` - Claude CLI stream-json 验证
2. `test/poc_feishu_api.go` - 飞书 API 测试框架（待完善）

### 参考文档

- 飞书 CardKit 2.0: https://open.feishu.cn/document/cardkit-v1/streaming-updates-openapi-overview
- Claude Code 文档: https://code.claude.com/docs/en/headless
- 飞书 Go SDK: https://github.com/larksuite/oapi-sdk-go
