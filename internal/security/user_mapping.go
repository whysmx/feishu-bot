package security

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"strings"
	"sync"
)

// UserMappingService 用户映射服务
type UserMappingService struct {
	configPath   string
	userMappings map[string]string // user_id -> open_id mapping
	adminUsers   map[string]bool
	mutex        sync.RWMutex
	logger       *log.Logger
}

// WhitelistConfig 白名单配置结构
type WhitelistConfig struct {
	AllowedUsers []AllowedUser  `yaml:"allowed_users"`
	AdminUsers   []string       `yaml:"admin_users"`
	GlobalLimits GlobalLimits   `yaml:"global_limits"`
}

// AllowedUser 允许的用户
type AllowedUser struct {
	UserID      string   `yaml:"user_id"`
	OpenID      string   `yaml:"open_id"`
	Name        string   `yaml:"name"`
	Permissions []string `yaml:"permissions"`
	MaxSessions int      `yaml:"max_sessions"`
}

// GlobalLimits 全局限制
type GlobalLimits struct {
	MaxTotalSessions        int `yaml:"max_total_sessions"`
	MaxSessionDurationHours int `yaml:"max_session_duration_hours"`
}

// NewUserMappingService 创建用户映射服务
func NewUserMappingService(configPath string) (*UserMappingService, error) {
	service := &UserMappingService{
		configPath:   configPath,
		userMappings: make(map[string]string),
		adminUsers:   make(map[string]bool),
		logger:       log.New(log.Writer(), "[UserMapping] ", log.LstdFlags),
	}

	if err := service.LoadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load user mapping config: %w", err)
	}

	return service, nil
}

// LoadConfig 加载配置文件
func (ums *UserMappingService) LoadConfig() error {
	ums.mutex.Lock()
	defer ums.mutex.Unlock()

	data, err := ioutil.ReadFile(ums.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", ums.configPath, err)
	}

	var config WhitelistConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// 清空现有映射
	ums.userMappings = make(map[string]string)
	ums.adminUsers = make(map[string]bool)

	// 构建用户映射
	for _, user := range config.AllowedUsers {
		if user.UserID == "" || user.OpenID == "" {
			ums.logger.Printf("Warning: skipping user with empty user_id or open_id")
			continue
		}
		ums.userMappings[user.UserID] = user.OpenID
		ums.logger.Printf("Loaded user mapping: %s -> %s (%s)", user.UserID, user.OpenID, user.Name)
	}

	// 构建管理员映射
	for _, adminUserID := range config.AdminUsers {
		ums.adminUsers[adminUserID] = true
	}

	ums.logger.Printf("Loaded %d user mappings and %d admin users", len(ums.userMappings), len(ums.adminUsers))
	return nil
}

// ResolveOpenID 解析OpenID
// 如果提供的openID是占位符或空字符串，则尝试通过userID查找真实的OpenID
func (ums *UserMappingService) ResolveOpenID(userID, openID string) (string, error) {
	ums.mutex.RLock()
	defer ums.mutex.RUnlock()

	// 如果openID看起来是有效的（不是占位符），直接返回
	if ums.isValidOpenID(openID) {
		return openID, nil
	}

	// 如果openID是占位符或空，尝试通过userID查找
	if realOpenID, exists := ums.userMappings[userID]; exists {
		ums.logger.Printf("Resolved placeholder openID '%s' to real openID '%s' for userID '%s'", 
			openID, realOpenID, userID)
		return realOpenID, nil
	}

	return "", fmt.Errorf("cannot resolve openID for userID '%s': user not found in whitelist", userID)
}

// isValidOpenID 检查OpenID是否有效（不是占位符）
func (ums *UserMappingService) isValidOpenID(openID string) bool {
	if openID == "" {
		return false
	}

	// 检查是否为占位符
	placeholders := []string{
		"your_open_id",
		"your_user_id", 
		"YOUR_OPEN_ID",
		"YOUR_USER_ID",
		"placeholder",
		"PLACEHOLDER",
	}

	openIDLower := strings.ToLower(openID)
	for _, placeholder := range placeholders {
		if openIDLower == strings.ToLower(placeholder) {
			return false
		}
	}

	// 飞书OpenID通常以'ou_'开头，长度较长
	if strings.HasPrefix(openID, "ou_") && len(openID) > 10 {
		return true
	}

	// 对于其他格式，如果不是明显的占位符，暂时认为有效
	// 实际验证将由飞书API进行
	return !strings.Contains(openIDLower, "placeholder") && 
		   !strings.Contains(openIDLower, "your_") &&
		   !strings.Contains(openIDLower, "example")
}

// GetUserByOpenID 根据OpenID获取用户信息
func (ums *UserMappingService) GetUserByOpenID(openID string) (*AllowedUser, error) {
	ums.mutex.RLock()
	defer ums.mutex.RUnlock()

	data, err := ioutil.ReadFile(ums.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config WhitelistConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	for _, user := range config.AllowedUsers {
		if user.OpenID == openID {
			return &user, nil
		}
	}

	return nil, fmt.Errorf("user with openID '%s' not found", openID)
}

// IsUserAllowed 检查用户是否被允许
func (ums *UserMappingService) IsUserAllowed(userID string) bool {
	ums.mutex.RLock()
	defer ums.mutex.RUnlock()

	_, exists := ums.userMappings[userID]
	return exists
}

// IsAdminUser 检查是否为管理员用户
func (ums *UserMappingService) IsAdminUser(userID string) bool {
	ums.mutex.RLock()
	defer ums.mutex.RUnlock()

	return ums.adminUsers[userID]
}

// GetAllMappings 获取所有用户映射（调试用）
func (ums *UserMappingService) GetAllMappings() map[string]string {
	ums.mutex.RLock()
	defer ums.mutex.RUnlock()

	mappings := make(map[string]string)
	for k, v := range ums.userMappings {
		mappings[k] = v
	}
	return mappings
}

// ReloadConfig 重新加载配置文件
func (ums *UserMappingService) ReloadConfig() error {
	ums.logger.Println("Reloading user mapping configuration...")
	return ums.LoadConfig()
}