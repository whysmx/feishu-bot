package command

import (
	"fmt"
	"regexp"
	"strings"
)

// defaultCommandParser 默认命令解析器
type defaultCommandParser struct {
	tokenPattern *regexp.Regexp
}

// NewCommandParser 创建命令解析器
func NewCommandParser() CommandParser {
	return &defaultCommandParser{
		tokenPattern: regexp.MustCompile(`^([A-Z0-9]{8}):\s*(.+)$`),
	}
}

// ParseMessage 解析消息内容
func (cp *defaultCommandParser) ParseMessage(content string) (*MessageCommand, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("empty message content")
	}

	// 清理消息内容
	cleanContent := cp.cleanMessageContent(content)
	
	// 提取令牌和命令
	token, err := cp.ExtractToken(cleanContent)
	if err != nil {
		return nil, err
	}

	command := cp.extractCommand(cleanContent)
	if command == "" {
		return nil, fmt.Errorf("empty command")
	}

	return &MessageCommand{
		Token:   token,
		Command: command,
		Raw:     content,
	}, nil
}

// ValidateCommand 验证命令
func (cp *defaultCommandParser) ValidateCommand(command string) error {
	if command == "" {
		return fmt.Errorf("command cannot be empty")
	}

	if len(command) > 1000 {
		return fmt.Errorf("command too long (max 1000 characters)")
	}

	// 检查危险命令
	dangerousCommands := []string{
		"rm -rf /",
		"sudo rm",
		"mkfs",
		"dd if=",
		":(){ :|:& };:",  // fork bomb
		"shutdown",
		"reboot",
		"> /dev/sda",
	}

	lowerCommand := strings.ToLower(command)
	for _, dangerous := range dangerousCommands {
		if strings.Contains(lowerCommand, strings.ToLower(dangerous)) {
			return fmt.Errorf("dangerous command detected: %s", dangerous)
		}
	}

	return nil
}

// ExtractToken 提取令牌
func (cp *defaultCommandParser) ExtractToken(content string) (string, error) {
	matches := cp.tokenPattern.FindStringSubmatch(content)
	if len(matches) != 3 {
		return "", fmt.Errorf("invalid command format, expected: TOKEN: command")
	}
	
	token := matches[1]
	if len(token) != 8 {
		return "", fmt.Errorf("invalid token length, expected 8 characters")
	}

	return token, nil
}

// extractCommand 提取命令部分
func (cp *defaultCommandParser) extractCommand(content string) string {
	matches := cp.tokenPattern.FindStringSubmatch(content)
	if len(matches) != 3 {
		return ""
	}
	
	return strings.TrimSpace(matches[2])
}

// cleanMessageContent 清理消息内容
func (cp *defaultCommandParser) cleanMessageContent(content string) string {
	lines := strings.Split(content, "\n")
	var cleanLines []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// 跳过空行
		if line == "" {
			continue
		}
		
		// 跳过引用内容（以 > 开头）
		if strings.HasPrefix(line, ">") {
			continue
		}
		
		// 跳过常见的邮件标记
		if strings.Contains(line, "--- Original Message ---") ||
		   strings.Contains(line, "wrote:") ||
		   strings.HasPrefix(line, "--") ||
		   strings.Contains(line, "Sent from") {
			break
		}
		
		// 跳过简单问候语
		lower := strings.ToLower(line)
		if lower == "hi" || lower == "hello" || lower == "thanks" || 
		   lower == "ok" || lower == "好的" || lower == "谢谢" {
			continue
		}
		
		cleanLines = append(cleanLines, line)
	}
	
	return strings.Join(cleanLines, " ")
}

// IsValidTokenFormat 检查令牌格式
func IsValidTokenFormat(token string) bool {
	if len(token) != 8 {
		return false
	}
	
	pattern := regexp.MustCompile(`^[A-Z0-9]{8}$`)
	return pattern.MatchString(token)
}

// SanitizeCommand 清理命令
func SanitizeCommand(command string) string {
	// 移除控制字符
	re := regexp.MustCompile(`[\x00-\x1F\x7F]`)
	command = re.ReplaceAllString(command, "")
	
	// 限制长度
	if len(command) > 1000 {
		command = command[:1000]
	}
	
	return strings.TrimSpace(command)
}