package command

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// defaultCommandValidator 默认命令验证器
type defaultCommandValidator struct {
	allowedUsers   map[string]bool
	rateLimits     map[string]*rateLimitEntry
	mutex          sync.RWMutex
	securityPolicy *SecurityPolicy
}

// rateLimitEntry 速率限制条目
type rateLimitEntry struct {
	lastReset    time.Time
	commandCount int
}

// NewCommandValidator 创建命令验证器
func NewCommandValidator() CommandValidator {
	return &defaultCommandValidator{
		allowedUsers: make(map[string]bool),
		rateLimits:   make(map[string]*rateLimitEntry),
		securityPolicy: &SecurityPolicy{
			MaxCommandLength: 1000,
			AllowedCommands: []string{
				"ls", "pwd", "cat", "echo", "grep", "find", "git", "npm", "node", 
				"python", "pip", "go", "cargo", "docker", "kubectl", "make",
				"cd", "mkdir", "touch", "cp", "mv", "help", "man", "which",
			},
			ForbiddenCommands: []string{
				"rm -rf /", "sudo rm", "mkfs", "dd", "shutdown", "reboot",
				":(){ :|:& };:", "> /dev/", "chmod 777", "chown", "passwd",
			},
			RequireConfirmation: []string{
				"rm", "delete", "drop", "truncate", "format",
			},
		},
	}
}

// ValidateCommand 验证命令
func (dcv *defaultCommandValidator) ValidateCommand(command string) error {
	if command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	// 检查命令长度
	if len(command) > dcv.securityPolicy.MaxCommandLength {
		return fmt.Errorf("command too long (max %d characters)", dcv.securityPolicy.MaxCommandLength)
	}

	// 检查禁止的命令
	lowerCommand := strings.ToLower(command)
	for _, forbidden := range dcv.securityPolicy.ForbiddenCommands {
		if strings.Contains(lowerCommand, strings.ToLower(forbidden)) {
			return fmt.Errorf("forbidden command detected: %s", forbidden)
		}
	}

	// 检查是否包含shell注入风险
	if dcv.hasPotentialInjection(command) {
		return fmt.Errorf("potential command injection detected")
	}

	return nil
}

// ValidateUser 验证用户
func (dcv *defaultCommandValidator) ValidateUser(userID string) error {
	dcv.mutex.RLock()
	defer dcv.mutex.RUnlock()

	// 简单的用户验证，实际实现中应该从配置或数据库加载
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}

	// 这里可以添加用户黑名单检查
	// 当前实现允许所有非空用户ID
	return nil
}

// ValidateRateLimit 验证速率限制
func (dcv *defaultCommandValidator) ValidateRateLimit(userID string) error {
	dcv.mutex.Lock()
	defer dcv.mutex.Unlock()

	now := time.Now()
	entry, exists := dcv.rateLimits[userID]

	if !exists {
		dcv.rateLimits[userID] = &rateLimitEntry{
			lastReset:    now,
			commandCount: 1,
		}
		return nil
	}

	// 重置计数器（每分钟重置）
	if now.Sub(entry.lastReset) > time.Minute {
		entry.lastReset = now
		entry.commandCount = 1
		return nil
	}

	// 检查是否超过限制（每分钟30个命令）
	if entry.commandCount >= 30 {
		return fmt.Errorf("rate limit exceeded: too many commands per minute")
	}

	entry.commandCount++
	return nil
}

// hasPotentialInjection 检查潜在的命令注入
func (dcv *defaultCommandValidator) hasPotentialInjection(command string) bool {
	// 检查常见的命令注入模式
	dangerousPatterns := []string{
		";", "&", "|", "`", "$(",
		"$(", "${", "&&", "||", 
		"<(", ">(", "\n", "\r",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(command, pattern) {
			return true
		}
	}

	return false
}

// IsCommandAllowed 检查命令是否被允许
func (dcv *defaultCommandValidator) IsCommandAllowed(command string) bool {
	// 提取命令的第一个词（实际命令）
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false
	}

	baseCommand := parts[0]

	// 检查是否在允许列表中
	for _, allowed := range dcv.securityPolicy.AllowedCommands {
		if baseCommand == allowed || strings.HasPrefix(baseCommand, allowed) {
			return true
		}
	}

	// 如果没有允许列表，则检查是否在禁止列表中
	if len(dcv.securityPolicy.AllowedCommands) == 0 {
		for _, forbidden := range dcv.securityPolicy.ForbiddenCommands {
			if strings.Contains(strings.ToLower(command), strings.ToLower(forbidden)) {
				return false
			}
		}
		return true
	}

	return false
}

// RequiresConfirmation 检查命令是否需要确认
func (dcv *defaultCommandValidator) RequiresConfirmation(command string) bool {
	lowerCommand := strings.ToLower(command)
	
	for _, pattern := range dcv.securityPolicy.RequireConfirmation {
		if strings.Contains(lowerCommand, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// AddAllowedUser 添加允许的用户
func (dcv *defaultCommandValidator) AddAllowedUser(userID string) {
	dcv.mutex.Lock()
	defer dcv.mutex.Unlock()
	dcv.allowedUsers[userID] = true
}

// RemoveAllowedUser 移除允许的用户
func (dcv *defaultCommandValidator) RemoveAllowedUser(userID string) {
	dcv.mutex.Lock()
	defer dcv.mutex.Unlock()
	delete(dcv.allowedUsers, userID)
}

// UpdateSecurityPolicy 更新安全策略
func (dcv *defaultCommandValidator) UpdateSecurityPolicy(policy *SecurityPolicy) {
	dcv.mutex.Lock()
	defer dcv.mutex.Unlock()
	dcv.securityPolicy = policy
}

// GetSecurityPolicy 获取安全策略
func (dcv *defaultCommandValidator) GetSecurityPolicy() *SecurityPolicy {
	dcv.mutex.RLock()
	defer dcv.mutex.RUnlock()
	
	// 返回副本以避免并发修改
	return &SecurityPolicy{
		MaxCommandLength:    dcv.securityPolicy.MaxCommandLength,
		AllowedCommands:     append([]string{}, dcv.securityPolicy.AllowedCommands...),
		ForbiddenCommands:   append([]string{}, dcv.securityPolicy.ForbiddenCommands...),
		RequireConfirmation: append([]string{}, dcv.securityPolicy.RequireConfirmation...),
	}
}

// CleanupRateLimits 清理过期的速率限制条目
func (dcv *defaultCommandValidator) CleanupRateLimits() {
	dcv.mutex.Lock()
	defer dcv.mutex.Unlock()

	now := time.Now()
	for userID, entry := range dcv.rateLimits {
		// 删除1小时前的条目
		if now.Sub(entry.lastReset) > time.Hour {
			delete(dcv.rateLimits, userID)
		}
	}
}