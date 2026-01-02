package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// StreamEvent Claude CLI stream-json 事件结构
type StreamEvent struct {
	Type      string                 `json:"type"`
	Event     map[string]interface{} `json:"event,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	UUID      string                 `json:"uuid,omitempty"`
}

// TextDelta 文本增量事件
type TextDelta struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
}

// ClaudeManager Claude CLI 进程管理器
type ClaudeManager struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stderr        io.ReadCloser
	cancel        context.CancelFunc
	currentText   strings.Builder
	textSequence  int
	lastMessageID string
	mu            sync.Mutex
	onTextDelta   func(text string, sequence int) error
	onComplete    func(finalText string) error
	onError       func(err error)
}

// ClaudeConfig Claude CLI 配置
type ClaudeConfig struct {
	ProjectDir    string
	InitialPrompt string
}

// NewClaudeManager 创建 Claude 管理器
func NewClaudeManager(config ClaudeConfig) *ClaudeManager {
	return &ClaudeManager{}
}

// SetTextDeltaCallback 设置文本增量回调
func (m *ClaudeManager) SetTextDeltaCallback(cb func(text string, sequence int) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onTextDelta = cb
}

// SetCompleteCallback 设置完成回调
func (m *ClaudeManager) SetCompleteCallback(cb func(finalText string) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onComplete = cb
}

// SetErrorCallback 设置错误回调
func (m *ClaudeManager) SetErrorCallback(cb func(err error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onError = cb
}

// Start 启动 Claude CLI 进程
func (m *ClaudeManager) Start(ctx context.Context, userMessage string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 创建带取消的上下文
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	// 构建 Claude CLI 命令
	// 使用项目目录作为工作目录
	args := []string{
		"-p",                             // 非交互模式
		"--output-format", "stream-json", // 流式 JSON 输出
		"--include-partial-messages", // 包含部分消息
		"--verbose",                  // 详细输出
	}

	m.cmd = exec.CommandContext(ctx, "/Users/wen/.npm-global/bin/claude", append([]string{"--dangerously-skip-permissions"}, args...)...)

	// 设置环境变量（从cc1 shell函数复制）
	m.cmd.Env = append(os.Environ(),
		"ANTHROPIC_BASE_URL=https://open.bigmodel.cn/api/anthropic",
		"ANTHROPIC_API_KEY=a944d4a96c5b4de0af8557409ebc8fd6.n9f4QJFeF0hvIrW3",
		"ANTHROPIC_AUTH_TOKEN=a944d4a96c5b4de0af8557409ebc8fd6.n9f4QJFeF0hvIrW3",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=true",
		"CLAUDE_CODE_ENABLE_UNIFIED_READ_TOOL=true",
	)

	// 设置工作目录为项目目录
	// m.cmd.Dir = config.ProjectDir

	// 创建管道
	stdin, err := m.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	m.stdin = stdin

	stdout, err := m.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	m.stdout = stdout

	stderr, err := m.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	m.stderr = stderr

	// 启动进程
	log.Printf("[ClaudeManager] Starting claude command: %s %v", m.cmd.Path, m.cmd.Args)
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start claude command: %w", err)
	}
	log.Printf("[ClaudeManager] Process started with PID: %d", m.cmd.Process.Pid)

	// 发送用户消息
	log.Printf("[ClaudeManager] Sending user message: %s", userMessage)
	if _, err := fmt.Fprintln(m.stdin, userMessage); err != nil {
		cancel()
		return fmt.Errorf("failed to send user message: %w", err)
	}

	// ⚠️ 重要：关闭 stdin 发送 EOF 信号，让 Claude CLI 知道输入结束
	// Claude CLI 在 -p 模式下需要 EOF 才会开始处理
	// 延迟关闭，给协程启动时间
	go func() {
		time.Sleep(100 * time.Millisecond)
		log.Printf("[ClaudeManager] Closing stdin to send EOF signal")
		m.stdin.Close()
	}()

	log.Printf("[ClaudeManager] User message sent, starting parse goroutines")

	// 重置状态
	m.currentText.Reset()
	m.textSequence = 0
	m.lastMessageID = ""

	// 启动输出解析协程
	go m.parseOutput()
	go m.parseError()

	return nil
}

// parseOutput 解析 Claude CLI 输出
func (m *ClaudeManager) parseOutput() {
	scanner := bufio.NewScanner(m.stdout)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		// 记录原始输出（前100行，方便调试）
		if lineCount <= 100 {
			log.Printf("[Claude CLI %d] %s", lineCount, line)
		}

		// 跳过空行
		if strings.TrimSpace(line) == "" {
			continue
		}

		// 解析 JSON
		var event StreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// 不是 JSON 格式，可能是纯文本输出
			log.Printf("[Claude CLI] Non-JSON line %d: %s", lineCount, line)
			continue
		}

		// 处理不同类型的事件
		switch event.Type {
		case "stream_event":
			m.handleStreamEvent(event)
		case "system":
			// 系统事件，记录但不处理
			log.Printf("[Claude CLI] System event: %s", line)
		case "error":
			m.handleError(fmt.Errorf("claude error: %s", line))
		}
	}

	log.Printf("[Claude CLI] Output ended, total lines: %d", lineCount)
	// 输出结束，通知完成
	m.notifyComplete()
}

// parseError 解析错误输出
func (m *ClaudeManager) parseError() {
	scanner := bufio.NewScanner(m.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			m.handleError(fmt.Errorf("claude stderr: %s", line))
		}
	}
}

// handleStreamEvent 处理流式事件
func (m *ClaudeManager) handleStreamEvent(event StreamEvent) {
	eventType, ok := event.Event["type"].(string)
	if !ok {
		return
	}

	switch eventType {
	case "message_start":
		// 消息开始，重置文本（避免重复 message_start 造成序列号回退）
		var messageID string
		if messageRaw, ok := event.Event["message"].(map[string]interface{}); ok {
			if id, ok := messageRaw["id"].(string); ok {
				messageID = id
			}
		}

		m.mu.Lock()
		prevSeq := m.textSequence
		prevMessageID := m.lastMessageID
		if messageID != "" && m.lastMessageID == messageID && m.textSequence > 0 {
			log.Printf("[ClaudeManager] message_start ignored: message_id=%s last_message_id=%s seq=%d", messageID, m.lastMessageID, m.textSequence)
			m.mu.Unlock()
			return
		}
		if messageID != "" {
			m.lastMessageID = messageID
		}
		m.currentText.Reset()
		m.textSequence = 1 // CardKit API 要求序列号从 1 开始
		log.Printf("[ClaudeManager] message_start: message_id=%s last_message_id=%s prev_seq=%d new_seq=%d", messageID, prevMessageID, prevSeq, m.textSequence)
		m.mu.Unlock()

	case "content_block_delta":
		// 文本增量事件
		m.handleTextDelta(event)

	case "message_stop":
		// 消息结束
		log.Printf("[ClaudeManager] message_stop received")
		m.notifyComplete()
	}
}

// handleTextDelta 处理文本增量
func (m *ClaudeManager) handleTextDelta(event StreamEvent) {
	// 解析 delta
	deltaData, ok := event.Event["delta"].(map[string]interface{})
	if !ok {
		return
	}

	deltaType, ok := deltaData["type"].(string)
	if !ok || deltaType != "text_delta" {
		return
	}

	text, ok := deltaData["text"].(string)
	if !ok {
		return
	}

	if text == "" {
		return
	}

	// 更新当前文本
	m.mu.Lock()
	m.currentText.WriteString(text)
	fullText := m.currentText.String()
	sequence := m.textSequence // 先使用当前序列号
	m.textSequence++           // 然后递增

	// 获取回调（复制引用避免死锁）
	callback := m.onTextDelta
	m.mu.Unlock()

	// 调用回调
	if callback != nil {
		if err := callback(fullText, sequence); err != nil {
			m.handleError(fmt.Errorf("failed to send text delta: %w", err))
		}
	}
}

// notifyComplete 通知完成
func (m *ClaudeManager) notifyComplete() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.onComplete != nil {
		finalText := m.currentText.String()
		if err := m.onComplete(finalText); err != nil {
			m.handleError(fmt.Errorf("failed to send complete: %w", err))
		}
	}
}

// handleError 处理错误
func (m *ClaudeManager) handleError(err error) {
	m.mu.Lock()
	callback := m.onError
	m.mu.Unlock()

	if callback != nil {
		callback(err)
	}
}

// Stop 停止 Claude CLI 进程
func (m *ClaudeManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
	}

	if m.stdin != nil {
		m.stdin.Close()
	}

	if m.cmd != nil && m.cmd.Process != nil {
		// 等待进程自然结束，最多 5 秒
		done := make(chan error, 1)
		go func() {
			done <- m.cmd.Wait()
		}()

		select {
		case <-done:
			// 进程已结束
		case <-time.After(5 * time.Second):
			// 超时，强制杀死
			m.cmd.Process.Kill()
		}
	}
}

// WaitForExit 等待进程退出
func (m *ClaudeManager) WaitForExit() error {
	if m.cmd != nil {
		return m.cmd.Wait()
	}
	return nil
}
