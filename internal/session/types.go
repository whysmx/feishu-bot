package session

import "time"

// Session 会话信息
type Session struct {
	Token       string    `json:"token"`        // 8位令牌 (如: ABC12345)
	UserID      string    `json:"user_id"`      // 飞书用户ID
	OpenID      string    `json:"open_id"`      // 飞书OpenID
	TmuxSession string    `json:"tmux_session"` // tmux会话名
	WorkingDir  string    `json:"working_dir"`  // 工作目录
	Description string    `json:"description"`  // 任务描述
	Status      string    `json:"status"`       // completed/waiting/active
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	ExpiresAt   time.Time `json:"expires_at"`   // 过期时间(24小时)
	LastActiveAt *time.Time `json:"last_active_at,omitempty"` // 最后活跃时间
}

// SessionStatus 会话状态常量
const (
	StatusActive    = "active"
	StatusCompleted = "completed"
	StatusWaiting   = "waiting"
	StatusExpired   = "expired"
)

// SessionStorage 会话存储结构
type SessionStorage struct {
	Sessions map[string]*Session `json:"sessions"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// CreateSessionRequest 创建会话请求
type CreateSessionRequest struct {
	UserID      string `json:"user_id" binding:"required"`
	OpenID      string `json:"open_id" binding:"required"`
	TmuxSession string `json:"tmux_session" binding:"required"`
	WorkingDir  string `json:"working_dir"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

// UpdateSessionRequest 更新会话请求
type UpdateSessionRequest struct {
	Status      *string `json:"status,omitempty"`
	Description *string `json:"description,omitempty"`
}

// SessionListResponse 会话列表响应
type SessionListResponse struct {
	Sessions    []*Session `json:"sessions"`
	Total       int        `json:"total"`
	ActiveCount int        `json:"active_count"`
}

// TokenGenerator 令牌生成接口
type TokenGenerator interface {
	Generate() string
	Validate(token string) bool
}

// SessionManager 会话管理接口
type SessionManager interface {
	CreateSession(req *CreateSessionRequest) (*Session, error)
	GetSession(token string) (*Session, error)
	UpdateSession(token string, req *UpdateSessionRequest) (*Session, error)
	DeleteSession(token string) error
	ListSessions(userID string) (*SessionListResponse, error)
	ListAllSessions() (*SessionListResponse, error)
	CleanupExpiredSessions() (int, error)
	ValidateSession(token string) (*Session, error)
}