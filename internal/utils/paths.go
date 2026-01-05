package utils

import (
	"os"
	"path/filepath"
	"runtime"
)

// GetTempDir 获取系统临时目录（跨平台兼容）
// macOS/Linux: /tmp
// Windows: C:\Users\<用户>\AppData\Local\Temp
func GetTempDir() string {
	return os.TempDir()
}

// GetTempFilePath 获取临时目录下的文件路径（跨平台兼容）
func GetTempFilePath(filename string) string {
	return filepath.Join(GetTempDir(), filename)
}

// GetLogFile 获取日志文件路径（跨平台兼容）
// 如果指定了绝对路径，直接使用
// 如果是相对路径，基于当前工作目录
func GetLogFile(filename string) string {
	if filepath.IsAbs(filename) {
		return filename
	}
	return filepath.Join(".", filename)
}

// GetConfigDir 获取配置目录（跨平台兼容）
// macOS/Linux: ~/.feishu-bot
// Windows: C:\Users\<用户>\.feishu-bot
func GetConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// 降级到临时目录
		return GetTempDir()
	}
	return filepath.Join(homeDir, ".feishu-bot")
}

// GetConfigFilePath 获取配置文件路径（跨平台兼容）
func GetConfigFilePath(filename string) string {
	configDir := GetConfigDir()
	return filepath.Join(configDir, filename)
}

// EnsureDir 确保目录存在，如果不存在则创建
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// IsWindows 判断是否是 Windows 系统
func IsWindows() bool {
	return runtime.GOOS == "windows"
}
