package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Manager 项目配置管理器
type Manager struct {
	configPath string           // 配置文件路径
	bindings   map[string]string // chat_id -> project_path
	mu         sync.RWMutex
	baseDir    string           // 基础目录：~/Desktop/code
}

// Config 配置文件结构
type Config struct {
	Bindings map[string]string `json:"bindings"`
}

// NewManager 创建项目配置管理器
func NewManager(configPath string) (*Manager, error) {
	// 展开路径中的 ~
	expandedPath := expandPath(configPath)

	// 确保配置目录存在
	configDir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	m := &Manager{
		configPath: expandedPath,
		bindings:   make(map[string]string),
		baseDir:    expandPath("~/Desktop/code"), // 默认基础目录
	}

	// 加载现有配置
	if err := m.Load(); err != nil {
		// 如果文件不存在，创建空配置
		if os.IsNotExist(err) {
			if err := m.Save(); err != nil {
				return nil, fmt.Errorf("failed to create config file: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}

	return m, nil
}

// Load 从文件加载配置
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	m.bindings = config.Bindings
	if m.bindings == nil {
		m.bindings = make(map[string]string)
	}

	return nil
}

// Save 保存配置到文件
func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config := Config{
		Bindings: m.bindings,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// GetProjectDir 获取群聊绑定的项目路径
func (m *Manager) GetProjectDir(chatID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.bindings[chatID]
}

// BindChat 绑定群聊到项目路径
func (m *Manager) BindChat(chatID, projectPath string) error {
	// 展开路径
	expandedPath := expandPath(projectPath)

	// 验证路径存在且是目录
	if !m.isValidPath(expandedPath) {
		return fmt.Errorf("路径不存在或不是目录: %s", projectPath)
	}

	// 转换为绝对路径
	absPath, err := filepath.Abs(expandedPath)
	if err != nil {
		return fmt.Errorf("无法解析路径: %w", err)
	}

	m.mu.Lock()
	m.bindings[chatID] = absPath
	m.mu.Unlock()

	return m.Save()
}

// UnbindChat 解绑群聊
func (m *Manager) UnbindChat(chatID string) error {
	m.mu.Lock()
	delete(m.bindings, chatID)
	m.mu.Unlock()

	return m.Save()
}

// ListBaseDirProjects 列出基础目录下的所有项目
func (m *Manager) ListBaseDirProjects() ([]string, error) {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // 目录不存在，返回空列表
		}
		return nil, fmt.Errorf("无法读取目录: %w", err)
	}

	var projects []string
	for _, entry := range entries {
		if entry.IsDir() {
			// 跳过隐藏目录
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			// 构造完整路径
			fullPath := filepath.Join(m.baseDir, entry.Name())
			projects = append(projects, fullPath)
		}
	}

	return projects, nil
}

// ListAllBindings 列出所有绑定（用于调试）
func (m *Manager) ListAllBindings() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本
	result := make(map[string]string, len(m.bindings))
	for k, v := range m.bindings {
		result[k] = v
	}
	return result
}

// isValidPath 验证路径是否有效且是目录
func (m *Manager) isValidPath(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// expandPath 展开路径中的 ~ 为用户主目录
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home := os.Getenv("HOME")
		if home != "" {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
