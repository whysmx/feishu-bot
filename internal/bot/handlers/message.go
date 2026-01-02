package handlers

import (
	"context"
	"encoding/json"
	"feishu-bot/internal/bot/client"
	"feishu-bot/internal/claude"
	"feishu-bot/internal/command"
	"feishu-bot/internal/notification"
	"feishu-bot/internal/session"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// MessageHandler 消息处理器
type MessageHandler struct {
	sessionManager      session.SessionManager
	commandExecutor     command.CommandExecutor
	notificationSender  notification.NotificationSender
	logger              *log.Logger
	feishuClient        *client.FeishuClient // 添加飞书客户端
	recentMessageIDs    map[string]time.Time
	recentMessageMu     sync.Mutex
}

// NewMessageHandler 创建消息处理器
func NewMessageHandler(
	sessionManager session.SessionManager,
	commandExecutor command.CommandExecutor,
	notificationSender notification.NotificationSender,
	feishuClient *client.FeishuClient,
) *MessageHandler {
	return &MessageHandler{
		sessionManager:      sessionManager,
		commandExecutor:     commandExecutor,
		notificationSender:  notificationSender,
		feishuClient:        feishuClient,
		logger:              log.New(log.Writer(), "[MessageHandler] ", log.LstdFlags),
		recentMessageIDs:    make(map[string]time.Time),
	}
}

// HandleP2PMessage 处理单聊消息
func (mh *MessageHandler) HandleP2PMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	mh.logger.Printf("Received P2P message: %s", larkcore.Prettify(event))
	_ = os.WriteFile("/tmp/feishu-last-p2p-event.json", []byte(larkcore.Prettify(event)), 0644)

	// 安全检查防止 nil 指针 - 只检查必需的字段
	if event == nil || event.Event == nil || event.Event.Sender == nil ||
		event.Event.Sender.SenderId == nil || event.Event.Sender.SenderId.OpenId == nil {
		mh.logger.Printf("Invalid event structure: missing required fields")
		return fmt.Errorf("invalid event structure")
	}

	if mh.shouldIgnoreMessage(event) {
		return nil
	}

	// 获取消息内容
	content, err := mh.extractTextContent(event.Event.Message)
	if err != nil {
		mh.logger.Printf("Failed to extract message content: %v", err)
		return err
	}
	messageID := ""
	if event.Event.Message != nil && event.Event.Message.MessageId != nil {
		messageID = *event.Event.Message.MessageId
	}
	chatID := ""
	if event.Event.Message != nil && event.Event.Message.ChatId != nil {
		chatID = *event.Event.Message.ChatId
	}
	mh.logger.Printf("[DEBUG] P2P content extracted: message_id=%s chat_id=%s len=%d content=%q", messageID, chatID, len(content), content)

	openID := *event.Event.Sender.SenderId.OpenId
	// 使用UnionId作为用户标识符，如果不存在则使用OpenId
	var userID string
	if event.Event.Sender.SenderId.UnionId != nil {
		userID = *event.Event.Sender.SenderId.UnionId
	} else {
		// 使用OpenId作为备选
		userID = openID
	}

	// P2P场景固定使用open_id，避免卡片发送到非成员chat导致230002
	receiveID := openID
	receiveIDType := "open_id"
	mh.logger.Printf("✅✅✅ P2P MODE: Using open_id=%s", openID) // 明确的标记
	return mh.processMessage(openID, userID, receiveID, receiveIDType, content)
}

func (mh *MessageHandler) shouldIgnoreMessage(event *larkim.P2MessageReceiveV1) bool {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return false
	}

	if event.Event.Message.MessageId != nil {
		mh.logger.Printf("[DEBUG] shouldIgnoreMessage: message_id=%s", *event.Event.Message.MessageId)
	}

	if event.Event.Sender != nil && event.Event.Sender.SenderType != nil {
		senderType := strings.ToLower(strings.TrimSpace(*event.Event.Sender.SenderType))
		if senderType != "" && senderType != "user" {
			mh.logger.Printf("[DEBUG] Ignoring message: sender_type=%s", senderType)
			return true
		}
	}

	if event.Event.Message.MessageType != nil {
		messageType := strings.ToLower(strings.TrimSpace(*event.Event.Message.MessageType))
		if messageType != "" && messageType != "text" {
			mh.logger.Printf("[DEBUG] Ignoring non-text message: message_type=%s", messageType)
			return true
		}
	}

	if event.Event.Message.MessageId != nil && *event.Event.Message.MessageId != "" {
		if mh.isDuplicateMessage(*event.Event.Message.MessageId) {
			mh.logger.Printf("[DEBUG] Ignoring duplicate message: message_id=%s", *event.Event.Message.MessageId)
			return true
		}
	}

	return false
}

func (mh *MessageHandler) isDuplicateMessage(messageID string) bool {
	const dedupWindow = 30 * time.Minute
	now := time.Now()

	mh.recentMessageMu.Lock()
	defer mh.recentMessageMu.Unlock()

	if lastSeen, ok := mh.recentMessageIDs[messageID]; ok {
		if now.Sub(lastSeen) < dedupWindow {
			mh.logger.Printf("[DEBUG] Duplicate detected: message_id=%s last_seen=%s", messageID, lastSeen.Format(time.RFC3339))
			return true
		}
	}

	mh.recentMessageIDs[messageID] = now
	mh.logger.Printf("[DEBUG] Dedup record added: message_id=%s", messageID)

	for id, ts := range mh.recentMessageIDs {
		if now.Sub(ts) >= dedupWindow {
			delete(mh.recentMessageIDs, id)
		}
	}

	return false
}

// processMessage 处理消息的通用逻辑
func (mh *MessageHandler) processMessage(openID, userID, receiveID, receiveIDType, content string) error {
	mh.logger.Printf("[DEBUG] processMessage: open_id=%s user_id=%s receive_id=%s receive_id_type=%s len=%d", openID, userID, receiveID, receiveIDType, len(content))
	return mh.handleStreamingChat(openID, userID, receiveID, receiveIDType, content)
}

// handleSessionsCommand 处理会话列表命令
func (mh *MessageHandler) handleSessionsCommand(openID, userID string) error {
	sessions, err := mh.sessionManager.ListSessions(userID)
	if err != nil {
		mh.logger.Printf("Failed to list sessions for user %s: %v", userID, err)
		return mh.sendTextMessage(openID, "❌ 获取会话列表失败，请稍后重试")
	}

	if sessions.Total == 0 {
		return mh.sendTextMessage(openID, "📋 您当前没有活跃的会话")
	}

	// 构建会话列表消息
	var message strings.Builder
	message.WriteString("📋 您的活跃会话列表：\n\n")

	for i, sess := range sessions.Sessions {
		statusEmoji := mh.getStatusEmoji(sess.Status)
		message.WriteString(
			fmt.Sprintf("%d. %s %s\n   令牌: %s\n   项目: %s\n   状态: %s\n\n",
				i+1, statusEmoji, sess.Description, sess.Token,
				sess.WorkingDir, sess.Status))
	}

	message.WriteString(fmt.Sprintf("总计: %d 个会话 | 活跃: %d 个",
		sessions.Total, sessions.ActiveCount))

	return mh.sendTextMessage(openID, message.String())
}

// handleRemoteCommand 处理远程命令
func (mh *MessageHandler) handleRemoteCommand(openID, userID, content string) error {
	// 解析命令
	token, command, err := mh.parseRemoteCommand(content)
	if err != nil {
		return mh.sendTextMessage(openID, "❌ 命令格式错误，请使用: <令牌>: <命令>")
	}

	// 检查命令执行器是否可用
	if mh.commandExecutor == nil {
		return mh.sendTextMessage(openID, "⚠️ 命令执行功能暂未启用")
	}

	// 暂时使用mock实现
	mh.logger.Printf("Mock: Would execute command %s for token %s", command, token)

	// 模拟成功响应
	resultMessage := fmt.Sprintf("✅ 命令执行成功\n\n令牌: %s\n命令: %s\n方法: mock\n耗时: 100ms",
		token, command)

	return mh.sendTextMessage(openID, resultMessage)
}

// isRemoteCommand 检查是否是远程命令
func (mh *MessageHandler) isRemoteCommand(content string) bool {
	// 匹配格式: TOKEN: command
	pattern := `^[A-Z0-9]{8}:\s*.+`
	matched, _ := regexp.MatchString(pattern, content)
	return matched
}

// parseRemoteCommand 解析远程命令
func (mh *MessageHandler) parseRemoteCommand(content string) (token, command string, err error) {
	// 匹配 TOKEN: command 格式
	re := regexp.MustCompile(`^([A-Z0-9]{8}):\s*(.+)$`)
	matches := re.FindStringSubmatch(content)

	if len(matches) != 3 {
		return "", "", fmt.Errorf("invalid command format")
	}

	return matches[1], strings.TrimSpace(matches[2]), nil
}

// extractTextContent 提取文本内容
func (mh *MessageHandler) extractTextContent(message interface{}) (string, error) {
	if message == nil {
		return "", fmt.Errorf("message is nil")
	}

	// 尝试从消息中提取Content字段
	var messageMap map[string]interface{}

	// 尝试直接传递的map
	if m, ok := message.(map[string]interface{}); ok {
		messageMap = m
	} else {
		// 尝试JSON转换
		messageBytes, err := json.Marshal(message)
		if err != nil {
			return "", fmt.Errorf("failed to marshal message: %w", err)
		}

		if err := json.Unmarshal(messageBytes, &messageMap); err != nil {
			return "", fmt.Errorf("failed to unmarshal message: %w", err)
		}
	}

	// 提取Content字段
	content, exists := messageMap["Content"]
	if !exists {
		// 尝试content字段（小写）
		content, exists = messageMap["content"]
		if !exists {
			return "", fmt.Errorf("no content field found in message")
		}
	}

	// 将content转换为字符串
	contentStr, ok := content.(string)
	if !ok {
		return "", fmt.Errorf("content is not a string")
	}
	mh.logger.Printf("[DEBUG] Raw message content: len=%d content=%q", len(contentStr), contentStr)

	// 解析JSON格式的文本内容
	var textContent map[string]interface{}
	if err := json.Unmarshal([]byte(contentStr), &textContent); err != nil {
		// 如果不是JSON格式，直接返回原内容
		return contentStr, nil
	}

	// 提取text字段
	text, exists := textContent["text"]
	if !exists {
		return "", fmt.Errorf("no text field found in content")
	}

	textStr, ok := text.(string)
	if !ok {
		return "", fmt.Errorf("text is not a string")
	}

	return textStr, nil
}

// isMentioned 检查是否@了机器人
func (mh *MessageHandler) isMentioned(message *larkim.EventMessage) bool {
	if message == nil {
		return false
	}
	return len(message.Mentions) > 0
}

// getStatusEmoji 获取状态对应的emoji
func (mh *MessageHandler) getStatusEmoji(status string) string {
	switch status {
	case session.StatusActive:
		return "🟢"
	case session.StatusCompleted:
		return "✅"
	case session.StatusWaiting:
		return "⏳"
	case session.StatusExpired:
		return "⚪"
	default:
		return "❓"
	}
}

// sendTextMessage 发送文本消息的便捷方法
func (mh *MessageHandler) sendTextMessage(openID, text string) error {
	// 这里假设notificationSender有一个SendTextNotification方法
	// 在实际实现中需要根据具体接口调整
	if sender, ok := mh.notificationSender.(interface {
		SendTextNotification(openID, message string) error
	}); ok {
		return sender.SendTextNotification(openID, text)
	}

	// 如果没有SendTextNotification方法，使用基本的发送方式
	mh.logger.Printf("Sending text message to %s: %s", openID, text)
	return nil
}

// sendHelpMessage 发送帮助消息的便捷方法
func (mh *MessageHandler) sendHelpMessage(openID string) error {
	helpText := `💡 使用说明

• 直接发送任何消息即可对话

说明：
• 所有消息会直接透传给 Claude CLI
• 不做命令拦截或二次加工`

	return mh.sendTextMessage(openID, helpText)
}

// handleStreamingChat 处理流式对话请求
func (mh *MessageHandler) handleStreamingChat(openID, userID, receiveID, receiveIDType, question string) error {
	mh.logger.Printf("[DEBUG] handleStreamingChat called with: openID=%s userID=%s receiveID=%s receiveIDType=%s question=%s", openID, userID, receiveID, receiveIDType, question)
	_ = os.WriteFile("/tmp/feishu-last-streaming.txt", []byte(fmt.Sprintf("receive_id_type=%s receive_id=%s", receiveIDType, receiveID)), 0644)

	// 获取 tenant_access_token
	token, err := mh.feishuClient.GetTenantAccessToken()
	if err != nil {
		mh.logger.Printf("Failed to get tenant access token: %v", err)
		return mh.sendTextMessage(openID, "❌ 获取访问令牌失败")
	}

	// 验证 receive_id 不为空
	if receiveID == "" {
		mh.logger.Printf("ERROR: receiveID is empty! receiveIDType=%s", receiveIDType)
		return mh.sendTextMessage(openID, "❌ 无法发送卡片：缺少有效的会话ID")
	}

	// 创建 Claude 流式对话处理器
	claudeHandler := claude.NewHandler()

	// 处理消息（会创建卡片并流式更新）
	ctx := context.Background()
	if err := claudeHandler.HandleMessage(ctx, token, receiveID, receiveIDType, question); err != nil {
		mh.logger.Printf("Failed to handle streaming chat: %v", err)
		return mh.sendTextMessage(openID, "❌ 对话处理失败: "+err.Error())
	}

	mh.logger.Printf("Streaming chat initiated successfully for user %s", userID)
	return nil
}
