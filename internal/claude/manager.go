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
	Message   map[string]interface{} `json:"message,omitempty"`
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
	outputDone    chan struct{}
	outputDoneOnce sync.Once
	updateCh      chan textUpdate
	updateDone    chan struct{}
	updateOnce    sync.Once
	sessionID     string
	currentText   strings.Builder
	textSequence  int
	lastMessageID string
	config        ClaudeConfig  // 保存配置
	mu            sync.Mutex
	onTextDelta   func(text string, sequence int) error
	onComplete    func(finalText string) error
	onError       func(err error)
	pendingTool   *pendingToolCall // 当前待执行的工具

	// 批量发送优化
	lastUpdateLen int             // 上次发送时的文本长度
	lastUpdateTime time.Time      // 上次发送时间
	flushTimer    *time.Timer     // 定时刷新器
	flushTimerMu  sync.Mutex      // 定时器锁
}

// ClaudeConfig Claude CLI 配置
type ClaudeConfig struct {
	ProjectDir    string
	InitialPrompt string
}

type textUpdate struct {
	text     string
	sequence int
}

// NewClaudeManager 创建 Claude 管理器
func NewClaudeManager(config ClaudeConfig) *ClaudeManager {
	return &ClaudeManager{
		config: config,
	}
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
func (m *ClaudeManager) Start(ctx context.Context, userMessage, resumeSessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 创建带取消的上下文
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.outputDone = make(chan struct{})
	m.outputDoneOnce = sync.Once{}
	m.updateCh = make(chan textUpdate, 1)
	m.updateDone = make(chan struct{})
	m.updateOnce = sync.Once{}

	// 构建 Claude CLI 命令
	// 使用项目目录作为工作目录
	args := []string{
		"-p",                             // 非交互模式
		"--output-format", "stream-json", // 流式 JSON 输出
		"--include-partial-messages", // 包含部分消息
		"--verbose",                  // 详细输出
	}
	if resumeSessionID != "" {
		args = append(args, "--resume", resumeSessionID)
		m.sessionID = resumeSessionID
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
	if m.config.ProjectDir != "" {
		m.cmd.Dir = m.config.ProjectDir
		log.Printf("[ClaudeManager] Working directory set to: %s", m.config.ProjectDir)
	}

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

	// 立即关闭 stdin 发送 EOF 信号
	// Claude CLI 在 -p 模式下需要 EOF 才会开始处理用户消息
	// 注意：这会导致工具调用失败（无法返回结果），但这是 Claude CLI -p 模式的限制
	if err := m.stdin.Close(); err != nil {
		log.Printf("[ClaudeManager] Warning: failed to close stdin: %v", err)
	}
	m.stdin = nil

	log.Printf("[ClaudeManager] User message sent, stdin closed (EOF sent), starting parse goroutines")

	// 重置状态
	m.currentText.Reset()
	m.textSequence = 0
	m.lastMessageID = ""

	// 启动输出解析协程
	go m.processUpdates()
	go m.parseOutput()
	go m.parseError()

	return nil
}

// parseOutput 解析 Claude CLI 输出
func (m *ClaudeManager) parseOutput() {
	defer m.markOutputDone()
	defer m.closeUpdateCh()
	reader := bufio.NewReader(m.stdout)
	lineCount := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			log.Printf("[Claude CLI] Output read error: %v", err)
		}
		if len(line) == 0 && err != nil {
			break
		}
		lineCount++
		line = strings.TrimRight(line, "\r\n")

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
		case "assistant":
			m.handleAssistantMessage(event)
		case "system":
			if event.SessionID != "" {
				m.setSessionID(event.SessionID)
			}
			// 系统事件，记录但不处理
			log.Printf("[Claude CLI] System event: %s", line)
		case "error":
			m.handleError(fmt.Errorf("claude error: %s", line))
		}
		if err == io.EOF {
			break
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
		if m.textSequence > 0 {
			log.Printf("[ClaudeManager] message_start ignored (stream active): message_id=%s last_message_id=%s seq=%d", messageID, m.lastMessageID, m.textSequence)
			m.mu.Unlock()
			return
		}
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
		m.lastUpdateLen = 0       // 重置批量发送状态
		m.lastUpdateTime = time.Time{} // 重置发送时间
		log.Printf("[ClaudeManager] message_start: message_id=%s last_message_id=%s prev_seq=%d new_seq=%d", messageID, prevMessageID, prevSeq, m.textSequence)
		m.mu.Unlock()

		// 停止之前的定时器
		m.stopFlushTimer()

	case "content_block_start":
		// 检测工具调用
		if contentBlock, ok := event.Event["content_block"].(map[string]interface{}); ok {
			if blockType, ok := contentBlock["type"].(string); ok && blockType == "tool_use" {
				m.handleToolUseStart(contentBlock)
			}
		}

	case "content_block_delta":
		// 文本增量事件或工具输入增量
		m.handleContentBlockDelta(event)

	case "content_block_stop":
		// 内容块结束（可能是工具调用结束）
		m.handleContentBlockStop(event)

	case "message_stop":
		// 消息结束
		log.Printf("[ClaudeManager] message_stop received")
		m.notifyComplete()
	}
}

// handleAssistantMessage 处理完整的 assistant 消息快照
func (m *ClaudeManager) handleAssistantMessage(event StreamEvent) {
	assistantText := extractAssistantText(event.Message)
	if assistantText == "" {
		return
	}

	m.mu.Lock()
	if len(assistantText) <= m.currentText.Len() {
		m.mu.Unlock()
		return
	}

	m.currentText.Reset()
	m.currentText.WriteString(assistantText)

	sequence := m.textSequence
	if sequence <= 0 {
		sequence = 1
		m.textSequence = 2
	} else {
		m.textSequence++
	}
	callback := m.onTextDelta
	m.mu.Unlock()

	if callback != nil {
		m.enqueueUpdate(assistantText, sequence)
	}
}

func extractAssistantText(message map[string]interface{}) string {
	if message == nil {
		return ""
	}

	contentRaw, ok := message["content"]
	if !ok {
		return ""
	}

	contentSlice, ok := contentRaw.([]interface{})
	if !ok {
		return ""
	}

	var builder strings.Builder
	for _, item := range contentSlice {
		contentItem, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		itemType, _ := contentItem["type"].(string)
		if itemType != "text" {
			continue
		}
		text, _ := contentItem["text"].(string)
		if text == "" {
			continue
		}
		builder.WriteString(text)
	}

	return builder.String()
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
	currentLen := len(fullText)

	// 批量发送策略：
	// 1. 每次文本增量都立即发送（保持流式效果）
	// 2. 不再批量累积（避免工具调用导致的内容丢失）
	// 3. handler.go 层面已经有缓冲机制，不需要这里再次批量
	shouldSend := true  // 立即发送

	if shouldSend {
		sequence := m.textSequence
		m.textSequence++
		m.lastUpdateLen = currentLen
		m.lastUpdateTime = time.Now()

		// 获取回调（复制引用避免死锁）
		callback := m.onTextDelta
		m.mu.Unlock()

		// 调用回调
		if callback != nil {
			m.enqueueUpdate(fullText, sequence)
		}
	}
}

// handleContentBlockDelta 处理 content_block_delta 事件（文本或工具输入）
func (m *ClaudeManager) handleContentBlockDelta(event StreamEvent) {
	deltaData, ok := event.Event["delta"].(map[string]interface{})
	if !ok {
		return
	}

	deltaType, ok := deltaData["type"].(string)
	if !ok {
		return
	}

	switch deltaType {
	case "text_delta":
		// 文本增量
		m.handleTextDelta(event)
	case "input_json_delta":
		// 工具输入增量（暂不处理，因为我们在 content_block_stop 时执行工具）
		log.Printf("[ClaudeManager] Tool input delta received")
	}
}

// handleToolUseStart 处理工具调用开始
func (m *ClaudeManager) handleToolUseStart(contentBlock map[string]interface{}) {
	toolName, _ := contentBlock["name"].(string)
	toolID, _ := contentBlock["id"].(string)
	log.Printf("[ClaudeManager] Tool use detected: name=%s id=%s", toolName, toolID)

	// 保存工具调用信息，等待 content_block_stop 时执行
	m.mu.Lock()
	m.pendingTool = &pendingToolCall{
		name: toolName,
		id:   toolID,
	}
	m.mu.Unlock()
}

// pendingToolCall 待执行的工具调用
type pendingToolCall struct {
	name     string
	id       string
	inputJSON string
}

// handleContentBlockStop 处理内容块结束（执行工具）
func (m *ClaudeManager) handleContentBlockStop(event StreamEvent) {
	m.mu.Lock()
	tool := m.pendingTool
	m.mu.Unlock()

	if tool == nil {
		return
	}

	// 从 assistant 消息中获取工具输入
	// 需要从最近的 assistant 消息中提取完整的 input
	log.Printf("[ClaudeManager] Content block stop, checking for tool execution: tool_name=%s", tool.name)

	// 等待一小段时间让完整的 input JSON 传输完成
	time.Sleep(100 * time.Millisecond)

	// 执行工具（简化版：只支持 Bash 工具）
	if tool.name == "Bash" {
		m.executeBashTool(tool)
	} else {
		log.Printf("[ClaudeManager] Unsupported tool: %s", tool.name)
		m.markToolCompleted(tool.id, `{"error": "unsupported tool"}`)
	}

	// 清除待执行工具
	m.mu.Lock()
	m.pendingTool = nil
	m.mu.Unlock()
}

// executeBashTool 执行 Bash 工具
func (m *ClaudeManager) executeBashTool(tool *pendingToolCall) {
	log.Printf("[ClaudeManager] Executing Bash tool...")

	// TODO: 从 assistant 消息中解析工具输入
	// 简化版：假设工具输入已经通过某种方式获取
	// 正确的做法是从 assistant 消息的 input 字段解析

	// 临时方案：返回一个占位符结果
	result := `{"output": "Tool execution not yet implemented"}`
	m.markToolCompleted(tool.id, result)
}

// markToolCompleted 标记工具完成并将结果返回给 Claude
func (m *ClaudeManager) markToolCompleted(toolID, result string) {
	log.Printf("[ClaudeManager] Tool completed: id=%s result_len=%d", toolID, len(result))

	// 构造工具结果响应并发送给 Claude
	// 格式：<tool_result>|<json>|<tool_id>
	response := fmt.Sprintf("<tool_result>|%s|%s\n", result, toolID)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stdin != nil && m.cmd != nil && m.cmd.Process != nil {
		log.Printf("[ClaudeManager] Sending tool result to Claude: %d bytes", len(response))
		if _, err := fmt.Fprint(m.stdin, response); err != nil {
			log.Printf("[ClaudeManager] Failed to send tool result: %v", err)
		}
	} else {
		log.Printf("[ClaudeManager] Cannot send tool result: stdin or process not available")
	}
}

// notifyComplete 通知完成
func (m *ClaudeManager) notifyComplete() {
	m.mu.Lock()

	// 停止定时器
	m.flushTimerMu.Lock()
	if m.flushTimer != nil {
		m.flushTimer.Stop()
		m.flushTimer = nil
	}
	m.flushTimerMu.Unlock()

	// 如果还有未发送的内容，强制发送一次
	finalText := m.currentText.String()
	if len(finalText) > m.lastUpdateLen {
		sequence := m.textSequence
		m.textSequence++
		m.lastUpdateLen = len(finalText)

		callback := m.onTextDelta
		m.mu.Unlock()

		if callback != nil {
			m.enqueueUpdate(finalText, sequence)
		}

		// 等待队列清空
		if m.updateDone != nil {
			<-m.updateDone
		}
	} else {
		m.mu.Unlock()
	}

	// 调用完成回调
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.onComplete != nil {
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

	m.closeUpdateCh()
	m.markOutputDone()
}

// WaitForExit 等待进程退出
func (m *ClaudeManager) WaitForExit() error {
	if m.cmd != nil {
		return m.cmd.Wait()
	}
	return nil
}

// WaitForOutput 等待输出解析完成
func (m *ClaudeManager) WaitForOutput(ctx context.Context) error {
	m.mu.Lock()
	ch := m.outputDone
	updateDone := m.updateDone
	m.mu.Unlock()

	if ch == nil {
		return nil
	}

	select {
	case <-ch:
		if updateDone == nil {
			return nil
		}
		select {
		case <-updateDone:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *ClaudeManager) markOutputDone() {
	m.outputDoneOnce.Do(func() {
		if m.outputDone != nil {
			close(m.outputDone)
		}
	})
}

func (m *ClaudeManager) setSessionID(sessionID string) {
	if sessionID == "" {
		return
	}
	m.mu.Lock()
	m.sessionID = sessionID
	m.mu.Unlock()
}

func (m *ClaudeManager) GetSessionID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessionID
}

func (m *ClaudeManager) enqueueUpdate(text string, sequence int) {
	if m.updateCh == nil {
		return
	}
	update := textUpdate{text: text, sequence: sequence}
	select {
	case m.updateCh <- update:
	default:
		select {
		case <-m.updateCh:
		default:
		}
		m.updateCh <- update
	}
}

func (m *ClaudeManager) closeUpdateCh() {
	m.updateOnce.Do(func() {
		if m.updateCh != nil {
			close(m.updateCh)
		}
	})
}

func (m *ClaudeManager) processUpdates() {
	defer func() {
		if m.updateDone != nil {
			close(m.updateDone)
		}
	}()

	for update := range m.updateCh {
		m.mu.Lock()
		callback := m.onTextDelta
		m.mu.Unlock()

		if callback == nil {
			continue
		}
		if err := callback(update.text, update.sequence); err != nil {
			m.handleError(fmt.Errorf("failed to send text delta: %w", err))
		}
	}
}

// resetFlushTimer 重置刷新定时器
func (m *ClaudeManager) resetFlushTimer(text string) {
	m.flushTimerMu.Lock()
	defer m.flushTimerMu.Unlock()

	// 停止旧定时器
	if m.flushTimer != nil {
		m.flushTimer.Stop()
	}

	// 创建新定时器（3 秒后强制发送）
	m.flushTimer = time.AfterFunc(3*time.Second, func() {
		m.mu.Lock()
		// 检查是否已经有新的发送
		currentLen := m.currentText.Len()
		if currentLen <= m.lastUpdateLen {
			m.mu.Unlock()
			return
		}

		sequence := m.textSequence
		m.textSequence++
		m.lastUpdateLen = currentLen
		m.lastUpdateTime = time.Now()

		callback := m.onTextDelta
		m.mu.Unlock()

		if callback != nil {
			m.enqueueUpdate(text, sequence)
		}
	})
}

// stopFlushTimer 停止刷新定时器
func (m *ClaudeManager) stopFlushTimer() {
	m.flushTimerMu.Lock()
	defer m.flushTimerMu.Unlock()

	if m.flushTimer != nil {
		m.flushTimer.Stop()
		m.flushTimer = nil
	}
}
