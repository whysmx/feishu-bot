package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ChatConfig 聊天配置
type ChatConfig struct {
	BaseDir      string            `json:"base_dir"`       // 基础目录（用于 ls 命令）
	ProjectPaths map[string]string `json:"project_paths"`  // 群聊/聊天 ID -> 项目路径
	mu           sync.RWMutex
}

// ConfigFile 默认配置文件路径
const ConfigFile = "configs/chat_config.json"

// Load 加载配置文件
func Load() (*ChatConfig, error) {
	cfg := &ChatConfig{
		ProjectPaths: make(map[string]string),
	}

	// 优先从环境变量读取基础目录
	if baseDir := os.Getenv("BASE_DIR"); baseDir != "" {
		cfg.BaseDir = baseDir
	}

	// 读取配置文件（如果存在，覆盖项目路径映射）
	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
		// 文件不存在，使用环境变量或默认值
		if cfg.BaseDir == "" {
			cfg.BaseDir = "/Users/wen/Desktop/code/"
		}
		if err := cfg.Save(); err != nil {
			return nil, fmt.Errorf("创建默认配置失败: %w", err)
		}
		return cfg, nil
	}

	// 解析 JSON（注意：不会覆盖环境变量的 BaseDir）
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 环境变量优先级高于文件
	if baseDir := os.Getenv("BASE_DIR"); baseDir != "" {
		cfg.BaseDir = baseDir
	} else if cfg.BaseDir == "" {
		cfg.BaseDir = "/Users/wen/Desktop/code/"
	}

	return cfg, nil
}

// Save 保存配置文件
func (cfg *ChatConfig) Save() error {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(ConfigFile), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	// 序列化为 JSON（带缩进）
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(ConfigFile, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// SetBaseDir 设置基础目录
func (cfg *ChatConfig) SetBaseDir(baseDir string) error {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	cfg.BaseDir = baseDir
	return nil
}

// GetBaseDir 获取基础目录
func (cfg *ChatConfig) GetBaseDir() string {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return cfg.BaseDir
}

// SetProjectPath 设置群聊绑定的项目路径
func (cfg *ChatConfig) SetProjectPath(chatID, projectPath string) error {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	cfg.ProjectPaths[chatID] = projectPath
	return nil
}

// GetProjectPath 获取群聊绑定的项目路径
func (cfg *ChatConfig) GetProjectPath(chatID string) string {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	return cfg.ProjectPaths[chatID]
}
