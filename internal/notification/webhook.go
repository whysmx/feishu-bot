package notification

import (
	"feishu-bot/internal/security"
	"feishu-bot/internal/session"
	"fmt"
	"log"
	"time"
)

// WebhookHandler webhook处理器
type WebhookHandler struct {
	sessionManager session.SessionManager
	notificationSender NotificationSender
	userMappingService *security.UserMappingService
	logger *log.Logger
}

// NewWebhookHandler 创建webhook处理器
func NewWebhookHandler(sessionManager session.SessionManager, notificationSender NotificationSender, userMappingService *security.UserMappingService) *WebhookHandler {
	return &WebhookHandler{
		sessionManager: sessionManager,
		notificationSender: notificationSender,
		userMappingService: userMappingService,
		logger: log.New(log.Writer(), "[WebhookHandler] ", log.LstdFlags),
	}
}

// HandleNotification 处理Claude Code通知
func (wh *WebhookHandler) HandleNotification(req *WebhookRequest) (*NotificationResponse, error) {
	wh.logger.Printf("Received notification: type=%s, project=%s, user=%s", 
		req.Type, req.ProjectName, req.UserID)

	// 验证请求
	if err := wh.validateRequest(req); err != nil {
		return &NotificationResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid request: %v", err),
		}, nil
	}

	// 解析真实的OpenID（处理占位符情况）
	realOpenID, err := wh.resolveOpenID(req.UserID, req.OpenID)
	if err != nil {
		wh.logger.Printf("Failed to resolve OpenID for user %s: %v", req.UserID, err)
		return &NotificationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to resolve user OpenID: %v", err),
		}, nil
	}

	// 更新请求中的OpenID为解析后的真实OpenID
	req.OpenID = realOpenID

	// 创建会话
	sessionReq := &session.CreateSessionRequest{
		UserID:      req.UserID,
		OpenID:      req.OpenID,
		TmuxSession: req.TmuxSession,
		WorkingDir:  req.WorkingDir,
		Description: req.Description,
		Status:      wh.mapNotificationTypeToSessionStatus(req.Type),
	}

	sess, err := wh.sessionManager.CreateSession(sessionReq)
	if err != nil {
		return &NotificationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to create session: %v", err),
		}, nil
	}

	// 创建通知
	notification := &TaskNotification{
		Type:        req.Type,
		UserID:      req.UserID,
		OpenID:      req.OpenID,
		Token:       sess.Token,
		ProjectName: req.ProjectName,
		Description: req.Description,
		WorkingDir:  req.WorkingDir,
		TmuxSession: req.TmuxSession,
		Timestamp:   time.Now(),
	}

	// 发送通知
	if err := wh.sendNotification(notification); err != nil {
		wh.logger.Printf("Failed to send notification: %v", err)
		// 删除创建的会话
		wh.sessionManager.DeleteSession(sess.Token)
		return &NotificationResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to send notification: %v", err),
		}, nil
	}

	wh.logger.Printf("Successfully processed notification, token: %s", sess.Token)

	return &NotificationResponse{
		Success: true,
		Token:   sess.Token,
		Message: fmt.Sprintf("Notification sent successfully with token %s", sess.Token),
	}, nil
}

// validateRequest 验证webhook请求
func (wh *WebhookHandler) validateRequest(req *WebhookRequest) error {
	if req.Type == "" {
		return fmt.Errorf("notification type is required")
	}

	if req.Type != TypeCompleted && req.Type != TypeWaiting && req.Type != TypeError {
		return fmt.Errorf("invalid notification type: %s", req.Type)
	}

	if req.UserID == "" {
		return fmt.Errorf("user_id is required")
	}

	if req.OpenID == "" {
		return fmt.Errorf("open_id is required")
	}

	if req.TmuxSession == "" {
		return fmt.Errorf("tmux_session is required")
	}

	return nil
}

// mapNotificationTypeToSessionStatus 映射通知类型到会话状态
func (wh *WebhookHandler) mapNotificationTypeToSessionStatus(notificationType string) string {
	switch notificationType {
	case TypeCompleted:
		return session.StatusCompleted
	case TypeWaiting:
		return session.StatusWaiting
	case TypeError:
		return session.StatusActive // 错误状态仍然保持活跃，等待用户处理
	default:
		return session.StatusActive
	}
}

// sendNotification 发送通知
func (wh *WebhookHandler) sendNotification(notification *TaskNotification) error {
	switch notification.Type {
	case TypeCompleted:
		return wh.notificationSender.SendTaskCompletedNotification(notification)
	case TypeWaiting:
		return wh.notificationSender.SendTaskWaitingNotification(notification)
	case TypeError:
		// 错误通知可以复用任务完成通知，但修改消息内容
		return wh.notificationSender.SendTaskCompletedNotification(notification)
	default:
		return fmt.Errorf("unsupported notification type: %s", notification.Type)
	}
}

// GetSessionInfo 获取会话信息（用于调试）
func (wh *WebhookHandler) GetSessionInfo(token string) (*session.Session, error) {
	return wh.sessionManager.GetSession(token)
}

// CleanupExpiredSessions 清理过期会话
func (wh *WebhookHandler) CleanupExpiredSessions() (int, error) {
	return wh.sessionManager.CleanupExpiredSessions()
}

// GetStats 获取统计信息
func (wh *WebhookHandler) GetStats() (map[string]interface{}, error) {
	allSessions, err := wh.sessionManager.ListAllSessions()
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_sessions": allSessions.Total,
		"active_sessions": allSessions.ActiveCount,
		"timestamp": time.Now(),
	}

	// 按状态分组统计
	statusCounts := make(map[string]int)
	for _, sess := range allSessions.Sessions {
		statusCounts[sess.Status]++
	}
	stats["status_counts"] = statusCounts

	return stats, nil
}

// resolveOpenID 解析OpenID，处理占位符情况
func (wh *WebhookHandler) resolveOpenID(userID, openID string) (string, error) {
	if wh.userMappingService == nil {
		wh.logger.Printf("Warning: User mapping service not available, using provided OpenID as-is")
		return openID, nil
	}

	resolvedOpenID, err := wh.userMappingService.ResolveOpenID(userID, openID)
	if err != nil {
		return "", fmt.Errorf("user mapping failed: %w", err)
	}

	if resolvedOpenID != openID {
		wh.logger.Printf("Resolved OpenID: %s -> %s for user %s", openID, resolvedOpenID, userID)
	}

	return resolvedOpenID, nil
}