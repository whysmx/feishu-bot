package session

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileStorage 基于文件的会话存储
type FileStorage struct {
	filePath string
	mutex    sync.RWMutex
}

// NewFileStorage 创建文件存储
func NewFileStorage(filePath string) *FileStorage {
	return &FileStorage{
		filePath: filePath,
	}
}

// Load 加载会话数据
func (fs *FileStorage) Load() (*SessionStorage, error) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	// 确保目录存在
	dir := filepath.Dir(fs.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// 如果文件不存在，返回空存储
	if _, err := os.Stat(fs.filePath); os.IsNotExist(err) {
		return &SessionStorage{
			Sessions:  make(map[string]*Session),
			UpdatedAt: time.Now(),
		}, nil
	}

	data, err := ioutil.ReadFile(fs.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var storage SessionStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	// 确保sessions不为nil
	if storage.Sessions == nil {
		storage.Sessions = make(map[string]*Session)
	}

	return &storage, nil
}

// Save 保存会话数据
func (fs *FileStorage) Save(storage *SessionStorage) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	storage.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(fs.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// 写入临时文件，然后重命名（原子操作）
	tempFile := fs.filePath + ".tmp"
	if err := ioutil.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempFile, fs.filePath); err != nil {
		os.Remove(tempFile) // 清理临时文件
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// Backup 备份会话数据
func (fs *FileStorage) Backup() error {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	if _, err := os.Stat(fs.filePath); os.IsNotExist(err) {
		return nil // 文件不存在，无需备份
	}

	backupPath := fmt.Sprintf("%s.backup.%d", fs.filePath, time.Now().Unix())
	
	data, err := ioutil.ReadFile(fs.filePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	if err := ioutil.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

// CleanupOldBackups 清理旧备份文件
func (fs *FileStorage) CleanupOldBackups(maxAge time.Duration) error {
	dir := filepath.Dir(fs.filePath)
	baseName := filepath.Base(fs.filePath)
	
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)
	
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		// 检查是否是备份文件
		if filepath.Ext(file.Name()) == ".backup" || 
		   (len(file.Name()) > len(baseName) && file.Name()[:len(baseName)] == baseName && 
		    file.Name()[len(baseName):len(baseName)+8] == ".backup.") {
			
			if file.ModTime().Before(cutoff) {
				backupPath := filepath.Join(dir, file.Name())
				if err := os.Remove(backupPath); err != nil {
					// 记录错误但继续清理其他文件
					fmt.Printf("Warning: failed to remove backup file %s: %v\n", backupPath, err)
				}
			}
		}
	}

	return nil
}