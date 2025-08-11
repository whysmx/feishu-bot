package handlers

import (
	"context"
	"feishu-bot/internal/command"
	"feishu-bot/internal/notification"
	"feishu-bot/internal/session"
	"fmt"
	"log"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

// CardActionHandler 卡片交互处理器
type CardActionHandler struct {
	sessionManager     session.SessionManager
	commandExecutor    command.CommandExecutor
	notificationSender notification.NotificationSender
	logger             *log.Logger
}

// NewCardActionHandler 创建卡片交互处理器
func NewCardActionHandler(
	sessionManager session.SessionManager,
	commandExecutor command.CommandExecutor,
	notificationSender notification.NotificationSender,
) *CardActionHandler {
	return &CardActionHandler{
		sessionManager:     sessionManager,
		commandExecutor:    commandExecutor,
		notificationSender: notificationSender,
		logger:             log.New(log.Writer(), "[CardActionHandler] ", log.LstdFlags),
	}
}

// HandleCardAction 处理卡片交互
func (cah *CardActionHandler) HandleCardAction(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	cah.logger.Printf("Card action triggered: %s", larkcore.Prettify(event))

	if event.Event.Action.Value == nil {
		return cah.createErrorResponse("无效的卡片动作"), nil
	}

	action, ok := event.Event.Action.Value["action"].(string)
	if !ok {
		return cah.createErrorResponse("无法解析卡片动作"), nil
	}

	token, _ := event.Event.Action.Value["token"].(string)
	openID := event.Event.Operator.OpenID
	userID := ""
	if event.Event.Operator.UserID != nil {
		userID = *event.Event.Operator.UserID
	}

	cah.logger.Printf("Processing card action: %s for token: %s", action, token)

	switch action {
	case "send_command":
		return cah.handleSendCommand(event, openID, userID, token)
	case "continue_work":
		return cah.handleContinueWork(openID, userID, token)
	case "view_status":
		return cah.handleViewStatus(openID, userID, token)
	case "view_session":
		return cah.handleViewSession(openID, userID, token)
	case "view_options":
		return cah.handleViewOptions(openID, userID, token)
	case "end_session":
		return cah.handleEndSession(openID, userID, token)
	case "retry_command":
		return cah.handleRetryCommand(event, openID, userID, token)
	default:
		return cah.createErrorResponse(fmt.Sprintf("未知的卡片动作: %s", action)), nil
	}
}

// handleSendCommand 处理发送命令
func (cah *CardActionHandler) handleSendCommand(event *callback.CardActionTriggerEvent, openID, userID, token string) (*callback.CardActionTriggerResponse, error) {
	// 从表单输入获取命令
	command := ""
	if event.Event.Action.FormValue != nil {
		if cmdInput, ok := event.Event.Action.FormValue["command_input"]; ok {
			if cmdStr, ok := cmdInput.(string); ok {
				command = cmdStr
			}
		}
	}

	if command == "" {
		return cah.createErrorResponse("请输入命令内容"), nil
	}

	// 检查命令执行器是否可用
	if cah.commandExecutor == nil {
		return cah.createErrorResponse("命令执行功能暂未启用"), nil
	}
	
	// 暂时使用mock实现
	cah.logger.Printf("Mock: Would execute command %s for token %s", command, token)
	
	// 模拟成功响应
	return cah.createSuccessResponse(fmt.Sprintf("✅ 命令已发送: %s", command)), nil
}

// handleContinueWork 处理继续工作
func (cah *CardActionHandler) handleContinueWork(openID, userID, token string) (*callback.CardActionTriggerResponse, error) {
	// 验证会话
	sess, err := cah.sessionManager.ValidateSession(token)
	if err != nil {
		return cah.createErrorResponse("会话无效或已过期"), nil
	}

	// 发送提示消息
	message := fmt.Sprintf("会话 %s 已准备就绪，请发送命令格式: %s: <您的命令>", sess.Description, token)
	
	if textSender, ok := cah.notificationSender.(interface {
		SendTextNotification(openID, message string) error
	}); ok {
		textSender.SendTextNotification(openID, message)
	}

	return cah.createSuccessResponse("✅ 会话已激活，请发送命令"), nil
}

// handleViewStatus 处理查看状态
func (cah *CardActionHandler) handleViewStatus(openID, userID, token string) (*callback.CardActionTriggerResponse, error) {
	sess, err := cah.sessionManager.GetSession(token)
	if err != nil {
		return cah.createErrorResponse("获取会话信息失败"), nil
	}

	statusMessage := fmt.Sprintf(`📊 **会话状态信息**

**令牌:** %s
**状态:** %s
**描述:** %s
**工作目录:** %s
**创建时间:** %s
**过期时间:** %s`,
		sess.Token,
		sess.Status,
		sess.Description,
		sess.WorkingDir,
		sess.CreatedAt.Format("2006-01-02 15:04:05"),
		sess.ExpiresAt.Format("2006-01-02 15:04:05"))

	if textSender, ok := cah.notificationSender.(interface {
		SendTextNotification(openID, message string) error
	}); ok {
		textSender.SendTextNotification(openID, statusMessage)
	}

	return cah.createSuccessResponse("✅ 状态信息已发送"), nil
}

// handleViewSession 处理查看会话
func (cah *CardActionHandler) handleViewSession(openID, userID, token string) (*callback.CardActionTriggerResponse, error) {
	return cah.handleViewStatus(openID, userID, token)
}

// handleViewOptions 处理查看选项
func (cah *CardActionHandler) handleViewOptions(openID, userID, token string) (*callback.CardActionTriggerResponse, error) {
	optionsMessage := fmt.Sprintf(`🛠️ **可用命令选项**

**基础命令:**
• %s: help - 获取帮助
• %s: status - 查看当前状态
• %s: pwd - 显示当前目录
• %s: ls - 列出文件

**开发命令:**
• %s: git status - 查看Git状态
• %s: npm test - 运行测试
• %s: npm run build - 构建项目

**说明:** 将令牌替换为您的实际令牌使用`,
		token, token, token, token, token, token, token)

	if textSender, ok := cah.notificationSender.(interface {
		SendTextNotification(openID, message string) error
	}); ok {
		textSender.SendTextNotification(openID, optionsMessage)
	}

	return cah.createSuccessResponse("✅ 命令选项已发送"), nil
}

// handleEndSession 处理结束会话
func (cah *CardActionHandler) handleEndSession(openID, userID, token string) (*callback.CardActionTriggerResponse, error) {
	err := cah.sessionManager.DeleteSession(token)
	if err != nil {
		return cah.createErrorResponse("结束会话失败"), nil
	}

	return cah.createSuccessResponse("✅ 会话已结束"), nil
}

// handleRetryCommand 处理重试命令
func (cah *CardActionHandler) handleRetryCommand(event *callback.CardActionTriggerEvent, openID, userID, token string) (*callback.CardActionTriggerResponse, error) {
	// 获取原始命令
	command, _ := event.Event.Action.Value["command"].(string)
	if command == "" {
		return cah.createErrorResponse("无法获取原始命令"), nil
	}

	// 暂时使用mock实现
	cah.logger.Printf("Mock: Would retry command %s for token %s", command, token)
	
	// 模拟成功响应
	return cah.createSuccessResponse(fmt.Sprintf("✅ 命令重试成功: %s", command)), nil
}

// createSuccessResponse 创建成功响应
func (cah *CardActionHandler) createSuccessResponse(message string) *callback.CardActionTriggerResponse {
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{
			Type:    "success",
			Content: message,
			I18nContent: map[string]string{
				"zh_cn": message,
				"en_us": message,
			},
		},
	}
}

// createErrorResponse 创建错误响应
func (cah *CardActionHandler) createErrorResponse(message string) *callback.CardActionTriggerResponse {
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{
			Type:    "error",
			Content: message,
			I18nContent: map[string]string{
				"zh_cn": message,
				"en_us": message,
			},
		},
	}
}

// createInfoResponse 创建信息响应
func (cah *CardActionHandler) createInfoResponse(message string) *callback.CardActionTriggerResponse {
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{
			Type:    "info",
			Content: message,
			I18nContent: map[string]string{
				"zh_cn": message,
				"en_us": message,
			},
		},
	}
}