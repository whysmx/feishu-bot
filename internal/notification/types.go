package notification

import "time"

// TaskNotification 任务通知
type TaskNotification struct {
	Type        string    `json:"type"`         // "completed" 或 "waiting"
	UserID      string    `json:"user_id"`      // 目标用户ID
	OpenID      string    `json:"open_id"`      // 飞书OpenID
	Token       string    `json:"token"`        // 会话令牌
	ProjectName string    `json:"project_name"` // 项目名称
	Description string    `json:"description"`  // 任务描述
	WorkingDir  string    `json:"working_dir"`  // 工作目录
	TmuxSession string    `json:"tmux_session"` // tmux会话名
	Timestamp   time.Time `json:"timestamp"`    // 通知时间
}

// NotificationType 通知类型常量
const (
	TypeCompleted = "completed"
	TypeWaiting   = "waiting"
	TypeError     = "error"
)

// WebhookRequest Claude Code webhook请求
type WebhookRequest struct {
	Type        string            `json:"type" binding:"required"`
	ProjectName string            `json:"project_name"`
	Description string            `json:"description"`
	WorkingDir  string            `json:"working_dir"`
	TmuxSession string            `json:"tmux_session"`
	UserID      string            `json:"user_id"`
	OpenID      string            `json:"open_id"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NotificationResponse 通知响应
type NotificationResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// CardData 卡片数据模板
type CardData struct {
	Token       string            `json:"token"`
	ProjectName string            `json:"project_name"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Timestamp   string            `json:"timestamp"`
	UserID      string            `json:"user_id"`
	OpenID      string            `json:"open_id"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// NotificationSender 通知发送器接口
type NotificationSender interface {
	SendTaskCompletedNotification(notification *TaskNotification) error
	SendTaskWaitingNotification(notification *TaskNotification) error
	SendCommandResultNotification(token, command, result string, success bool) error
}