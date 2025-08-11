package session

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"regexp"
	"time"
)

const (
	// DefaultTokenLength 默认令牌长度
	DefaultTokenLength = 8
	// TokenCharset 令牌字符集
	TokenCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// defaultTokenGenerator 默认令牌生成器
type defaultTokenGenerator struct {
	length int
}

// NewTokenGenerator 创建新的令牌生成器
func NewTokenGenerator(length int) TokenGenerator {
	if length <= 0 {
		length = DefaultTokenLength
	}
	return &defaultTokenGenerator{
		length: length,
	}
}

// Generate 生成新令牌
func (g *defaultTokenGenerator) Generate() string {
	token := make([]byte, g.length)
	charsetLen := len(TokenCharset)
	
	// 使用加密随机数生成器
	randomBytes := make([]byte, g.length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// 降级使用时间戳
		return g.generateFallback()
	}
	
	for i := 0; i < g.length; i++ {
		token[i] = TokenCharset[int(randomBytes[i])%charsetLen]
	}
	
	return string(token)
}

// generateFallback 降级令牌生成
func (g *defaultTokenGenerator) generateFallback() string {
	// 简单的后备方案，使用当前时间的哈希
	now := time.Now().UnixNano()
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d", now)))
	
	token := make([]byte, g.length)
	for i := 0; i < g.length; i++ {
		token[i] = TokenCharset[int(hash[i])%len(TokenCharset)]
	}
	
	return string(token)
}

// Validate 验证令牌格式
func (g *defaultTokenGenerator) Validate(token string) bool {
	if len(token) != g.length {
		return false
	}
	
	// 验证字符是否都在允许的字符集中
	pattern := fmt.Sprintf("^[%s]{%d}$", TokenCharset, g.length)
	matched, err := regexp.MatchString(pattern, token)
	if err != nil {
		return false
	}
	
	return matched
}

// GenerateUniqueToken 生成唯一令牌，检查重复
func GenerateUniqueToken(generator TokenGenerator, existingTokens map[string]bool, maxRetries int) (string, error) {
	if maxRetries <= 0 {
		maxRetries = 100
	}
	
	for i := 0; i < maxRetries; i++ {
		token := generator.Generate()
		if !existingTokens[token] {
			return token, nil
		}
	}
	
	return "", fmt.Errorf("failed to generate unique token after %d retries", maxRetries)
}