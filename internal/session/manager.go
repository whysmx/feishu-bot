package session

import (
	"fmt"
	"sync"
	"time"
)

// sessionManager 会话管理器实现
type sessionManager struct {
	storage   *FileStorage
	generator TokenGenerator
	config    SessionConfig
	cache     map[string]*Session
	mutex     sync.RWMutex
}

// SessionConfig 会话配置
type SessionConfig struct {
	TokenLength           int
	ExpirationHours       int
	CleanupIntervalMinutes int
}

// NewSessionManager 创建会话管理器
func NewSessionManager(storagePath string, config SessionConfig) (SessionManager, error) {
	if config.TokenLength <= 0 {
		config.TokenLength = DefaultTokenLength
	}
	if config.ExpirationHours <= 0 {
		config.ExpirationHours = 24
	}

	sm := &sessionManager{
		storage:   NewFileStorage(storagePath),
		generator: NewTokenGenerator(config.TokenLength),
		config:    config,
		cache:     make(map[string]*Session),
	}

	// 加载现有会话
	if err := sm.loadSessions(); err != nil {
		return nil, fmt.Errorf("failed to load sessions: %w", err)
	}

	// 启动清理协程
	if config.CleanupIntervalMinutes > 0 {
		go sm.startCleanupRoutine()
	}

	return sm, nil
}

// CreateSession 创建新会话
func (sm *sessionManager) CreateSession(req *CreateSessionRequest) (*Session, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 生成唯一令牌
	existingTokens := make(map[string]bool)
	for token := range sm.cache {
		existingTokens[token] = true
	}

	token, err := GenerateUniqueToken(sm.generator, existingTokens, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to generate unique token: %w", err)
	}

	// 创建会话
	now := time.Now()
	session := &Session{
		Token:       token,
		UserID:      req.UserID,
		OpenID:      req.OpenID,
		TmuxSession: req.TmuxSession,
		WorkingDir:  req.WorkingDir,
		Description: req.Description,
		Status:      req.Status,
		CreatedAt:   now,
		ExpiresAt:   now.Add(time.Duration(sm.config.ExpirationHours) * time.Hour),
		LastActiveAt: &now,
	}

	if session.Status == "" {
		session.Status = StatusActive
	}

	// 添加到缓存
	sm.cache[token] = session

	// 保存到存储
	if err := sm.saveSessions(); err != nil {
		delete(sm.cache, token) // 回滚
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return session, nil
}

// GetSession 获取会话
func (sm *sessionManager) GetSession(token string) (*Session, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	session, exists := sm.cache[token]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", token)
	}

	// 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired: %s", token)
	}

	return session, nil
}

// UpdateSession 更新会话
func (sm *sessionManager) UpdateSession(token string, req *UpdateSessionRequest) (*Session, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.cache[token]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", token)
	}

	// 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		delete(sm.cache, token)
		return nil, fmt.Errorf("session expired: %s", token)
	}

	// 更新字段
	if req.Status != nil {
		session.Status = *req.Status
	}
	if req.Description != nil {
		session.Description = *req.Description
	}

	now := time.Now()
	session.LastActiveAt = &now

	// 保存到存储
	if err := sm.saveSessions(); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return session, nil
}

// DeleteSession 删除会话
func (sm *sessionManager) DeleteSession(token string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if _, exists := sm.cache[token]; !exists {
		return fmt.Errorf("session not found: %s", token)
	}

	delete(sm.cache, token)

	return sm.saveSessions()
}

// ListSessions 列出用户会话
func (sm *sessionManager) ListSessions(userID string) (*SessionListResponse, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var sessions []*Session
	activeCount := 0

	for _, session := range sm.cache {
		if session.UserID == userID {
			// 跳过过期会话
			if time.Now().After(session.ExpiresAt) {
				continue
			}
			sessions = append(sessions, session)
			if session.Status == StatusActive {
				activeCount++
			}
		}
	}

	return &SessionListResponse{
		Sessions:    sessions,
		Total:       len(sessions),
		ActiveCount: activeCount,
	}, nil
}

// ListAllSessions 列出所有会话
func (sm *sessionManager) ListAllSessions() (*SessionListResponse, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var sessions []*Session
	activeCount := 0

	for _, session := range sm.cache {
		// 跳过过期会话
		if time.Now().After(session.ExpiresAt) {
			continue
		}
		sessions = append(sessions, session)
		if session.Status == StatusActive {
			activeCount++
		}
	}

	return &SessionListResponse{
		Sessions:    sessions,
		Total:       len(sessions),
		ActiveCount: activeCount,
	}, nil
}

// CleanupExpiredSessions 清理过期会话
func (sm *sessionManager) CleanupExpiredSessions() (int, error) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	now := time.Now()
	cleanedCount := 0

	for token, session := range sm.cache {
		if now.After(session.ExpiresAt) {
			delete(sm.cache, token)
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		if err := sm.saveSessions(); err != nil {
			return cleanedCount, fmt.Errorf("failed to save after cleanup: %w", err)
		}
	}

	return cleanedCount, nil
}

// ValidateSession 验证会话
func (sm *sessionManager) ValidateSession(token string) (*Session, error) {
	// 验证令牌格式
	if !sm.generator.Validate(token) {
		return nil, fmt.Errorf("invalid token format: %s", token)
	}

	return sm.GetSession(token)
}

// loadSessions 从存储加载会话
func (sm *sessionManager) loadSessions() error {
	storage, err := sm.storage.Load()
	if err != nil {
		return err
	}

	sm.cache = storage.Sessions
	if sm.cache == nil {
		sm.cache = make(map[string]*Session)
	}

	return nil
}

// saveSessions 保存会话到存储
func (sm *sessionManager) saveSessions() error {
	storage := &SessionStorage{
		Sessions:  sm.cache,
		UpdatedAt: time.Now(),
	}

	return sm.storage.Save(storage)
}

// startCleanupRoutine 启动清理协程
func (sm *sessionManager) startCleanupRoutine() {
	ticker := time.NewTicker(time.Duration(sm.config.CleanupIntervalMinutes) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if cleaned, err := sm.CleanupExpiredSessions(); err != nil {
				fmt.Printf("Error during session cleanup: %v\n", err)
			} else if cleaned > 0 {
				fmt.Printf("Cleaned up %d expired sessions\n", cleaned)
			}
		}
	}
}