package notification

import (
	"log"
)

// FeishuClientInterface 飞书客户端接口，避免循环引用
type FeishuClientInterface interface {
	SendTaskCompletedCard(openID string, cardData interface{}) error
	SendTaskWaitingCard(openID string, cardData interface{}) error
	SendCommandResultCard(openID, token, command, result string, success bool) error
	SendTextMessage(openID, text string) error
}

// feishuNotificationSender 基于飞书的通知发送器
type feishuNotificationSender struct {
	feishuClient FeishuClientInterface
	logger       *log.Logger
}

// NewFeishuNotificationSender 创建飞书通知发送器
func NewFeishuNotificationSender(feishuClient FeishuClientInterface) NotificationSender {
	return &feishuNotificationSender{
		feishuClient: feishuClient,
		logger:       log.New(log.Writer(), "[NotificationSender] ", log.LstdFlags),
	}
}

// SendTaskCompletedNotification 发送任务完成通知
func (fns *feishuNotificationSender) SendTaskCompletedNotification(notification *TaskNotification) error {
	fns.logger.Printf("Sending task completed notification for token: %s", notification.Token)

	// 创建卡片数据，使用interface{}避免循环引用
	cardData := map[string]interface{}{
		"token":        notification.Token,
		"project_name": notification.ProjectName,
		"description":  notification.Description,
		"status":       "completed",
		"timestamp":    notification.Timestamp.Format("2006-01-02 15:04:05"),
		"user_id":      notification.UserID,
		"open_id":      notification.OpenID,
	}

	if err := fns.feishuClient.SendTaskCompletedCard(notification.OpenID, cardData); err != nil {
		fns.logger.Printf("Failed to send task completed card: %v", err)
		return err
	}

	fns.logger.Printf("Task completed notification sent successfully for token: %s", notification.Token)
	return nil
}

// SendTaskWaitingNotification 发送等待输入通知
func (fns *feishuNotificationSender) SendTaskWaitingNotification(notification *TaskNotification) error {
	fns.logger.Printf("Sending task waiting notification for token: %s", notification.Token)

	// 创建卡片数据，使用interface{}避免循环引用
	cardData := map[string]interface{}{
		"token":        notification.Token,
		"project_name": notification.ProjectName,
		"description":  notification.Description,
		"status":       "waiting",
		"timestamp":    notification.Timestamp.Format("2006-01-02 15:04:05"),
		"user_id":      notification.UserID,
		"open_id":      notification.OpenID,
	}

	if err := fns.feishuClient.SendTaskWaitingCard(notification.OpenID, cardData); err != nil {
		fns.logger.Printf("Failed to send task waiting card: %v", err)
		return err
	}

	fns.logger.Printf("Task waiting notification sent successfully for token: %s", notification.Token)
	return nil
}

// SendCommandResultNotification 发送命令执行结果通知
func (fns *feishuNotificationSender) SendCommandResultNotification(token, command, result string, success bool) error {
	fns.logger.Printf("Sending command result notification for token: %s", token)

	// 这里需要获取用户的OpenID，在实际实现中需要通过token查找session获取
	// 暂时使用空字符串，实际使用时需要修改
	openID := "" // TODO: 通过token获取用户OpenID

	if err := fns.feishuClient.SendCommandResultCard(openID, token, command, result, success); err != nil {
		fns.logger.Printf("Failed to send command result card: %v", err)
		return err
	}

	fns.logger.Printf("Command result notification sent successfully for token: %s", token)
	return nil
}

// SendTextNotification 发送文本通知（便捷方法）
func (fns *feishuNotificationSender) SendTextNotification(openID, message string) error {
	return fns.feishuClient.SendTextMessage(openID, message)
}

// SendWelcomeMessage 发送欢迎消息
func (fns *feishuNotificationSender) SendWelcomeMessage(openID string) error {
	welcomeText := `🎉 欢迎使用 Claude Code 远程控制机器人！

主要功能：
• 📬 接收 Claude Code 任务完成通知
• ⌨️ 远程发送命令到 Claude Code 会话
• 📊 查看和管理活跃会话
• 🔒 安全的令牌验证机制

使用方法：
1. 当 Claude Code 完成任务或需要输入时，您将收到通知卡片和唯一令牌
2. 通过 "令牌: 命令" 格式发送消息来远程控制，例如：ABC12345: run tests
3. 使用 /sessions 查看所有活跃会话
4. 使用 /help 获取帮助信息

开始您的远程开发之旅吧！`

	return fns.feishuClient.SendTextMessage(openID, welcomeText)
}

// SendHelpMessage 发送帮助消息
func (fns *feishuNotificationSender) SendHelpMessage(openID string) error {
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

	return fns.feishuClient.SendTextMessage(openID, helpText)
}