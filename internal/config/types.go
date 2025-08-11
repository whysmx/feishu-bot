package config

// Config 主配置结构
type Config struct {
	Feishu   FeishuConfig   `yaml:"feishu"`
	Cards    CardsConfig    `yaml:"cards"`
	Webhook  WebhookConfig  `yaml:"webhook"`
	Session  SessionConfig  `yaml:"session"`
	Command  CommandConfig  `yaml:"command"`
	Security SecurityConfig `yaml:"security"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// FeishuConfig 飞书相关配置
type FeishuConfig struct {
	BaseDomain string `yaml:"base_domain"`
	AppID      string `yaml:"app_id"`
	AppSecret  string `yaml:"app_secret"`
}

// CardsConfig 卡片模板配置
type CardsConfig struct {
	StopTaskCardID      string `yaml:"stop_task_card_id"`
	RunningTaskCardID   string `yaml:"running_task_card_id"`
	SuccessTaskCardID   string `yaml:"success_task_card_id"`
	TaskCompletedCardID string `yaml:"task_completed_card_id"`
	TaskWaitingCardID   string `yaml:"task_waiting_card_id"`
	CommandResultCardID string `yaml:"command_result_card_id"`
}

// WebhookConfig Webhook相关配置
type WebhookConfig struct {
	Port      int                `yaml:"port"`
	Endpoints WebhookEndpoints   `yaml:"endpoints"`
}

type WebhookEndpoints struct {
	Notification string `yaml:"notification"`
	Health       string `yaml:"health"`
}

// SessionConfig 会话管理配置
type SessionConfig struct {
	TokenLength           int    `yaml:"token_length"`
	ExpirationHours       int    `yaml:"expiration_hours"`
	CleanupIntervalMinutes int    `yaml:"cleanup_interval_minutes"`
	StorageFile           string `yaml:"storage_file"`
}

// CommandConfig 命令执行配置
type CommandConfig struct {
	MaxLength            int      `yaml:"max_length"`
	TimeoutSeconds       int      `yaml:"timeout_seconds"`
	AllowedTmuxCommands  []string `yaml:"allowed_tmux_commands"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	WhitelistFile string     `yaml:"whitelist_file"`
	RateLimit     RateLimit  `yaml:"rate_limit"`
}

type RateLimit struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
	CommandsPerHour   int `yaml:"commands_per_hour"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level      string `yaml:"level"`
	File       string `yaml:"file"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
}

// UserConfig 用户权限配置
type UserConfig struct {
	AllowedUsers []User   `yaml:"allowed_users"`
	AdminUsers   []string `yaml:"admin_users"`
	GlobalLimits GlobalLimits `yaml:"global_limits"`
}

type User struct {
	UserID      string   `yaml:"user_id"`
	OpenID      string   `yaml:"open_id"`
	Name        string   `yaml:"name,omitempty"`
	Permissions []string `yaml:"permissions"`
	MaxSessions int      `yaml:"max_sessions"`
}

type GlobalLimits struct {
	MaxTotalSessions        int `yaml:"max_total_sessions"`
	MaxSessionDurationHours int `yaml:"max_session_duration_hours"`
}