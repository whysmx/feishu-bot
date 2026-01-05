package utils

import (
	"os"
	"path/filepath"
)

// GetTempFilePath 获取临时目录下的文件路径（跨平台兼容）
func GetTempFilePath(filename string) string {
	return filepath.Join(os.TempDir(), filename)
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
