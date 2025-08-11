package security

import "time"

// UserPermission 用户权限
type UserPermission struct {
	UserID      string    `json:"user_id"`
	OpenID      string    `json:"open_id"`
	Name        string    `json:"name,omitempty"`
	Permissions []string  `json:"permissions"`
	MaxSessions int       `json:"max_sessions"`
	IsAdmin     bool      `json:"is_admin"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Permission 权限常量
const (
	PermissionCommandExecute = "command_execute"
	PermissionSessionManage  = "session_manage"
	PermissionUserManage     = "user_manage"
	PermissionSystemAdmin    = "system_admin"
)

// RateLimitEntry 速率限制条目
type RateLimitEntry struct {
	UserID      string    `json:"user_id"`
	LastReset   time.Time `json:"last_reset"`
	RequestCount int      `json:"request_count"`
	CommandCount int      `json:"command_count"`
}

// AuthResult 认证结果
type AuthResult struct {
	Allowed     bool     `json:"allowed"`
	User        *UserPermission `json:"user,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// SecurityValidator 安全验证器接口
type SecurityValidator interface {
	ValidateUser(userID, openID string) (*AuthResult, error)
	CheckPermission(userID, permission string) error
	CheckRateLimit(userID string) error
	ValidateCommand(command string) error
	IsAdmin(userID string) bool
}

// WhitelistManager 白名单管理器接口
type WhitelistManager interface {
	LoadWhitelist() error
	IsUserAllowed(userID string) bool
	GetUserPermissions(userID string) ([]string, error)
	AddUser(user *UserPermission) error
	RemoveUser(userID string) error
	UpdateUser(userID string, user *UserPermission) error
}

// AuditLog 审计日志
type AuditLog struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	Action    string                 `json:"action"`
	Resource  string                 `json:"resource"`
	Success   bool                   `json:"success"`
	Details   map[string]interface{} `json:"details,omitempty"`
	IP        string                 `json:"ip,omitempty"`
	UserAgent string                 `json:"user_agent,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// AuditActions 审计动作常量
const (
	ActionLogin         = "login"
	ActionCommandExecute = "command_execute"
	ActionSessionCreate = "session_create"
	ActionSessionDelete = "session_delete"
	ActionUserManage    = "user_manage"
	ActionConfigChange  = "config_change"
)