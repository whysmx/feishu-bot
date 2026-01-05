package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v2"
)

// ConfigManager 配置管理器
type ConfigManager struct {
	config     *Config
	configPath string
	userConfig *UserConfig
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configPath string) *ConfigManager {
	return &ConfigManager{
		configPath: configPath,
	}
}

// Load 加载配置
func (cm *ConfigManager) Load() error {
	// 加载主配置文件
	if err := cm.loadMainConfig(); err != nil {
		return fmt.Errorf("failed to load main config: %w", err)
	}

	// 加载用户权限配置
	if err := cm.loadUserConfig(); err != nil {
		return fmt.Errorf("failed to load user config: %w", err)
	}

	// 从环境变量覆盖配置
	cm.overrideFromEnv()

	return nil
}

// loadMainConfig 加载主配置文件
func (cm *ConfigManager) loadMainConfig() error {
	data, err := ioutil.ReadFile(cm.configPath)
	if err != nil {
		// 如果配置文件不存在，使用默认配置
		if os.IsNotExist(err) {
			cm.config = cm.getDefaultConfig()
			return nil
		}
		return err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	cm.config = &config
	return nil
}

// loadUserConfig 加载用户配置
func (cm *ConfigManager) loadUserConfig() error {
	userConfigPath := cm.config.Security.WhitelistFile
	if userConfigPath == "" {
		userConfigPath = "configs/security/whitelist.yaml"
	}

	// 如果是相对路径，基于配置文件目录
	if !filepath.IsAbs(userConfigPath) {
		configDir := filepath.Dir(cm.configPath)
		userConfigPath = filepath.Join(configDir, userConfigPath)
	}

	data, err := ioutil.ReadFile(userConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 创建默认用户配置
			cm.userConfig = &UserConfig{
				AllowedUsers: []User{},
				AdminUsers:   []string{},
				GlobalLimits: GlobalLimits{
					MaxTotalSessions:        50,
					MaxSessionDurationHours: 48,
				},
			}
			return nil
		}
		return err
	}

	var userConfig UserConfig
	if err := yaml.Unmarshal(data, &userConfig); err != nil {
		return err
	}

	cm.userConfig = &userConfig
	return nil
}

// overrideFromEnv 从环境变量覆盖配置
func (cm *ConfigManager) overrideFromEnv() {
	if appID := os.Getenv("FEISHU_APP_ID"); appID != "" {
		cm.config.Feishu.AppID = appID
	}

	if appSecret := os.Getenv("FEISHU_APP_SECRET"); appSecret != "" {
		cm.config.Feishu.AppSecret = appSecret
	}

	if webhookPort := os.Getenv("WEBHOOK_PORT"); webhookPort != "" {
		// 这里需要转换为int，简化处理
		if port := parseIntEnv(webhookPort, cm.config.Webhook.Port); port > 0 {
			cm.config.Webhook.Port = port
		}
	}

	if sessionFile := os.Getenv("SESSION_STORAGE_FILE"); sessionFile != "" {
		cm.config.Session.StorageFile = sessionFile
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		cm.config.Logging.Level = logLevel
	}

	if logFile := os.Getenv("LOG_FILE"); logFile != "" {
		cm.config.Logging.File = logFile
	}
}

// GetConfig 获取配置
func (cm *ConfigManager) GetConfig() *Config {
	return cm.config
}

// GetUserConfig 获取用户配置
func (cm *ConfigManager) GetUserConfig() *UserConfig {
	return cm.userConfig
}

// SaveConfig 保存配置
func (cm *ConfigManager) SaveConfig() error {
	data, err := yaml.Marshal(cm.config)
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(cm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return ioutil.WriteFile(cm.configPath, data, 0644)
}

// SaveUserConfig 保存用户配置
func (cm *ConfigManager) SaveUserConfig() error {
	userConfigPath := cm.config.Security.WhitelistFile
	if userConfigPath == "" {
		userConfigPath = "configs/security/whitelist.yaml"
	}

	// 如果是相对路径，基于配置文件目录
	if !filepath.IsAbs(userConfigPath) {
		configDir := filepath.Dir(cm.configPath)
		userConfigPath = filepath.Join(configDir, userConfigPath)
	}

	data, err := yaml.Marshal(cm.userConfig)
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(userConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return ioutil.WriteFile(userConfigPath, data, 0644)
}

// getDefaultConfig 获取默认配置
func (cm *ConfigManager) getDefaultConfig() *Config {
	return &Config{
		Feishu: FeishuConfig{
			BaseDomain: "https://open.feishu.cn",
			AppID:      "",
			AppSecret:  "",
		},
		Cards: CardsConfig{},
		Webhook: WebhookConfig{
			Port: 8080,
			Endpoints: WebhookEndpoints{
				Notification: "/webhook/notification",
				Health:       "/health",
			},
		},
		Session: SessionConfig{
			TokenLength:            8,
			ExpirationHours:        24,
			CleanupIntervalMinutes: 60,
			StorageFile:            "", // 从环境变量读取，默认为空
		},
		Command: CommandConfig{
			MaxLength:      1000,
			TimeoutSeconds: 300,
			AllowedTmuxCommands: []string{
				"send-keys",
				"list-sessions",
				"kill-session",
			},
		},
		Security: SecurityConfig{
			WhitelistFile: "configs/security/whitelist.yaml",
			RateLimit: RateLimit{
				RequestsPerMinute: 30,
				CommandsPerHour:   100,
			},
		},
		Logging: LoggingConfig{
			Level:      "info",
			File:       "", // 从环境变量读取，默认输出到 stdout
			MaxSizeMB:  100,
			MaxBackups: 5,
		},
	}
}

// IsUserAllowed 检查用户是否被允许
func (cm *ConfigManager) IsUserAllowed(userID string) bool {
	for _, user := range cm.userConfig.AllowedUsers {
		if user.UserID == userID {
			return true
		}
	}
	return false
}

// IsUserAdmin 检查用户是否是管理员
func (cm *ConfigManager) IsUserAdmin(userID string) bool {
	for _, adminUserID := range cm.userConfig.AdminUsers {
		if adminUserID == userID {
			return true
		}
	}
	return false
}

// GetUserPermissions 获取用户权限
func (cm *ConfigManager) GetUserPermissions(userID string) []string {
	for _, user := range cm.userConfig.AllowedUsers {
		if user.UserID == userID {
			return user.Permissions
		}
	}
	return []string{}
}

// AddUser 添加用户
func (cm *ConfigManager) AddUser(user User) error {
	// 检查用户是否已存在
	for i, existingUser := range cm.userConfig.AllowedUsers {
		if existingUser.UserID == user.UserID {
			// 更新现有用户
			cm.userConfig.AllowedUsers[i] = user
			return cm.SaveUserConfig()
		}
	}

	// 添加新用户
	cm.userConfig.AllowedUsers = append(cm.userConfig.AllowedUsers, user)
	return cm.SaveUserConfig()
}

// RemoveUser 移除用户
func (cm *ConfigManager) RemoveUser(userID string) error {
	for i, user := range cm.userConfig.AllowedUsers {
		if user.UserID == userID {
			cm.userConfig.AllowedUsers = append(
				cm.userConfig.AllowedUsers[:i],
				cm.userConfig.AllowedUsers[i+1:]...,
			)
			return cm.SaveUserConfig()
		}
	}
	return fmt.Errorf("user not found: %s", userID)
}

// parseIntEnv 解析整数环境变量
func parseIntEnv(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}
	
	if intValue, err := strconv.Atoi(value); err == nil {
		return intValue
	}
	
	return defaultValue
}