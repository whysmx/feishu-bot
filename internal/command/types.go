package command

import "time"

// CommandRequest 命令执行请求
type CommandRequest struct {
	Token   string `json:"token" binding:"required"`
	Command string `json:"command" binding:"required"`
	UserID  string `json:"user_id" binding:"required"`
	OpenID  string `json:"open_id" binding:"required"`
}

// CommandResult 命令执行结果
type CommandResult struct {
	Token     string    `json:"token"`
	Command   string    `json:"command"`
	Success   bool      `json:"success"`
	Method    string    `json:"method"`  // "tmux" 或 "fallback"
	Output    string    `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	ExecTime  int64     `json:"exec_time"` // 执行时间(毫秒)
	Timestamp time.Time `json:"timestamp"`
}

// ExecutionMethod 执行方法常量
const (
	MethodTmux     = "tmux"
	MethodFallback = "fallback"
	MethodFailed   = "failed"
)

// MessageCommand 从飞书消息解析的命令
type MessageCommand struct {
	Token   string `json:"token"`
	Command string `json:"command"`
	Raw     string `json:"raw"` // 原始消息内容
}

// CommandParser 命令解析器接口
type CommandParser interface {
	ParseMessage(content string) (*MessageCommand, error)
	ValidateCommand(command string) error
	ExtractToken(content string) (string, error)
}

// CommandExecutor 命令执行器接口
type CommandExecutor interface {
	ExecuteCommand(req *CommandRequest) (*CommandResult, error)
	ExecuteInTmux(sessionName, command string) error
	ValidateSession(token string) error
}

// CommandValidator 命令验证器接口
type CommandValidator interface {
	ValidateCommand(command string) error
	ValidateUser(userID string) error
	ValidateRateLimit(userID string) error
}

// TmuxCommand tmux命令结构
type TmuxCommand struct {
	SessionName string
	Command     string
	Args        []string
}

// SecurityPolicy 安全策略
type SecurityPolicy struct {
	MaxCommandLength    int      `json:"max_command_length"`
	AllowedCommands     []string `json:"allowed_commands"`
	ForbiddenCommands   []string `json:"forbidden_commands"`
	RequireConfirmation []string `json:"require_confirmation"`
}