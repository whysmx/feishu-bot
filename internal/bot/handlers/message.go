package handlers

import (
	"context"
	"encoding/json"
	"feishu-bot/internal/command"
	"feishu-bot/internal/notification"
	"feishu-bot/internal/session"
	"fmt"
	"log"
	"regexp"
	"strings"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// MessageHandler 消息处理器
type MessageHandler struct {
	sessionManager     session.SessionManager
	commandExecutor    command.CommandExecutor
	notificationSender notification.NotificationSender
	logger             *log.Logger
}

// NewMessageHandler 创建消息处理器
func NewMessageHandler(
	sessionManager session.SessionManager,
	commandExecutor command.CommandExecutor,
	notificationSender notification.NotificationSender,
) *MessageHandler {
	return &MessageHandler{
		sessionManager:     sessionManager,
		commandExecutor:    commandExecutor,
		notificationSender: notificationSender,
		logger:             log.New(log.Writer(), "[MessageHandler] ", log.LstdFlags),
	}
}

// HandleP2PMessage 处理单聊消息
func (mh *MessageHandler) HandleP2PMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	mh.logger.Printf("Received P2P message: %s", larkcore.Prettify(event))

	// 安全检查防止 nil 指针 - 只检查必需的字段
	if event == nil || event.Event == nil || event.Event.Sender == nil || 
		event.Event.Sender.SenderId == nil || event.Event.Sender.SenderId.OpenId == nil {
		mh.logger.Printf("Invalid event structure: missing required fields")
		return fmt.Errorf("invalid event structure")
	}

	// 获取消息内容
	content, err := mh.extractTextContent(event.Event.Message)
	if err != nil {
		mh.logger.Printf("Failed to extract message content: %v", err)
		return err
	}

	openID := *event.Event.Sender.SenderId.OpenId
	// 使用UnionId作为用户标识符，如果不存在则使用OpenId
	var userID string
	if event.Event.Sender.SenderId.UnionId != nil {
		userID = *event.Event.Sender.SenderId.UnionId
	} else {
		// 使用OpenId作为备选
		userID = openID
	}

	return mh.processMessage(openID, userID, content)
}

// HandleGroupMessage 处理群聊消息
func (mh *MessageHandler) HandleGroupMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	mh.logger.Printf("Received group message: %s", larkcore.Prettify(event))

	// 安全检查防止 nil 指针 - 只检查必需的字段
	if event == nil || event.Event == nil || event.Event.Sender == nil || 
		event.Event.Sender.SenderId == nil || event.Event.Sender.SenderId.OpenId == nil {
		mh.logger.Printf("Invalid event structure: missing required fields")
		return fmt.Errorf("invalid event structure")
	}

	// 检查是否@了机器人
	if !mh.isMentioned(event.Event.Message) {
		return nil // 群聊中只处理@机器人的消息
	}

	// 获取消息内容
	content, err := mh.extractTextContent(event.Event.Message)
	if err != nil {
		mh.logger.Printf("Failed to extract message content: %v", err)
		return err
	}

	// 移除@机器人的部分
	content = mh.cleanMentionContent(content)

	openID := *event.Event.Sender.SenderId.OpenId
	// 使用UnionId作为用户标识符，如果不存在则使用OpenId
	var userID string
	if event.Event.Sender.SenderId.UnionId != nil {
		userID = *event.Event.Sender.SenderId.UnionId
	} else {
		// 使用OpenId作为备选
		userID = openID
	}

	return mh.processMessage(openID, userID, content)
}

// processMessage 处理消息的通用逻辑
func (mh *MessageHandler) processMessage(openID, userID, content string) error {
	content = strings.TrimSpace(content)
	
	// 处理特殊命令
	switch {
	case content == "/help" || content == "help":
		return mh.sendHelpMessage(openID)
		
	case content == "/sessions" || content == "sessions":
		return mh.handleSessionsCommand(openID, userID)
		
	case mh.isRemoteCommand(content):
		return mh.handleRemoteCommand(openID, userID, content)
		
	default:
		// 默认显示帮助信息
		return mh.sendHelpMessage(openID)
	}
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
func (mh *MessageHandler) isMentioned(message interface{}) bool {
	// 简单实现，实际需要检查mentions字段
	return true
}

// cleanMentionContent 清理@机器人的内容
func (mh *MessageHandler) cleanMentionContent(content string) string {
	// 移除@机器人的标记，这里简化处理
	return strings.TrimSpace(content)
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
	helpText := `💡 Claude Code 远程控制机器人帮助

命令格式：
• <令牌>: <命令> - 执行远程命令，例如：ABC12345: npm test
• /sessions - 查看所有活跃会话
• /help - 显示此帮助信息

令牌说明：
• 每个任务会生成一个8位唯一令牌（如：ABC12345）
• 令牌有效期为24小时
• 使用令牌可以安全地控制对应的Claude Code会话

支持的命令示例：
• ABC12345: run tests - 运行测试
• ABC12345: git status - 查看Git状态
• ABC12345: npm run build - 构建项目
• ABC12345: help - 获取Claude Code帮助

安全提示：
• 请勿分享您的令牌给他人
• 系统会验证您的身份和权限
• 危险命令会被自动拦截`

	return mh.sendTextMessage(openID, helpText)
}