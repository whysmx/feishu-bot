# 开发任务清单

## 说明

- 项目已迁移至 Claude CLI + 文本分段发送方案，旧版 CardKit 相关任务已不适用。
- 当前没有统一的任务清单来源时，可在此文件追加记录。

## 当前待办

- [ ] 为 `internal/bot/handlers/message.go` 增加基础单元测试（解析、命令判断、去重）
- [ ] 为 `internal/config/chat_config.go` 增加读写配置测试

## 已归档（历史）

- CardKit 卡片创建/更新相关任务（已弃用）
- Webhook/relay 服务相关任务（当前仓库未包含）
- 旧版 mention 清理与 /chat 兼容逻辑（已简化）
