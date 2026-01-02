package claude

import (
	"context"
	"fmt"
	"log"
	"os"
)

// Handler 流式对话处理器
type Handler struct {
	claudeManager *ClaudeManager
}

// NewHandler 创建处理器
func NewHandler() *Handler {
	return &Handler{}
}

// HandleMessage 处理用户消息并流式返回 AI 回复
func (h *Handler) HandleMessage(ctx context.Context, token, receiveID, receiveIDType, userMessage string) error {
	log.Printf("[HandleMessage] Processing message receive_id=%s type=%s", receiveID, receiveIDType)

	// Step 1: 创建流式卡片
	cardID, elementID, uuid, initialSeq, err := CreateStreamingCard(token, receiveID, receiveIDType, "Claude 对话", "思考中...")
	if err != nil {
		return fmt.Errorf("failed to create streaming card: %w", err)
	}

	log.Printf("[HandleMessage] Card created: card_id=%s, initial_seq=%d", cardID, initialSeq)

	// Step 2: 初始化 Claude 管理器
	h.claudeManager = NewClaudeManager(ClaudeConfig{})

	// 创建 CardKit 更新器
	updater := NewCardKitUpdater(token, cardID, elementID, uuid)

	// ✅ 关键修复: 使用服务端返回的初始 sequence
	updater.SetCurrentSeq(initialSeq)

	defer updater.Stop()

	// 设置文本增量回调 - 实时更新卡片
	h.claudeManager.SetTextDeltaCallback(func(text string, sequence int) error {
		log.Printf("[TextDelta] seq=%d, text=%s", sequence, text)
		return updater.UpdateContent(text, sequence)
	})

	// 设置完成回调
	h.claudeManager.SetCompleteCallback(func(finalText string) error {
		log.Printf("[Complete] Final text length: %d", len(finalText))
		return updater.FinalizeContent(finalText)
	})

	// 设置错误回调
	h.claudeManager.SetErrorCallback(func(err error) {
		log.Printf("[Error] Claude error: %v", err)
	})

	// Step 3: 启动 Claude CLI
	log.Printf("[HandleMessage] About to start Claude CLI")
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[HandleMessage] Panic recovered: %v", r)
		}
	}()
	if err := h.claudeManager.Start(ctx, userMessage); err != nil {
		return fmt.Errorf("failed to start claude: %w", err)
	}
	log.Printf("[HandleMessage] Claude CLI started successfully")

	// Step 4: 等待完成
	if err := h.claudeManager.WaitForExit(); err != nil {
		log.Printf("Claude exited with error: %v", err)
	}

	log.Printf("[HandleMessage] Message processing completed")
	return nil
}

// ExampleMain 示例使用
func ExampleMain() {
	// 从环境变量读取配置
	receiveID := os.Getenv("FEISHU_TEST_CHAT_ID")

	// 这里需要先获取 tenant_access_token
	// 为了简化，假设已经有 token
	token := "your_tenant_access_token_here"

	// 创建处理器
	handler := NewHandler()

	// 处理用户消息
	userMessage := "Hello! Please say 'Hi there!' and nothing else."

	ctx := context.Background()
	if err := handler.HandleMessage(ctx, token, receiveID, "chat_id", userMessage); err != nil {
		log.Printf("Failed to handle message: %v", err)
	}
}
