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
	welcomeText := `🎉 欢迎使用 Claude CLI 对话机器人！

使用方法：
• 直接发送任何消息即可开始对话

说明：
• 所有消息会直接透传给 Claude CLI
• 不做命令拦截或二次加工`

	return fns.feishuClient.SendTextMessage(openID, welcomeText)
}

// SendHelpMessage 发送帮助消息
func (fns *feishuNotificationSender) SendHelpMessage(openID string) error {
	helpText := `💡 使用说明

• 直接发送任何消息即可对话

说明：
• 所有消息会直接透传给 Claude CLI
• 不做命令拦截或二次加工`

	return fns.feishuClient.SendTextMessage(openID, helpText)
}
