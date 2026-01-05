package claude

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"feishu-bot/internal/bot/client"
	"feishu-bot/internal/utils"
)

// StreamingTextHandler 流式文本处理器（不使用 CardKit，节省 API 调用）
type StreamingTextHandler struct {
	feishuClient  *client.FeishuClient
	claudeManager *ClaudeManager
	lastSessionID string
	logger        *log.Logger

	// 流式发送状态
	buffer       []rune    // 累积缓冲区
	bufferMu     sync.Mutex
	receiveID    string
	receiveIDType string
	lastFullLen  int       // 上次完整文本的长度（用于计算增量）

	// 时间分段配置
	idleTimeout     time.Duration // 空闲超时：N毫秒无新数据则发送
	maxDuration     time.Duration // 最大持续时间：连续输出N秒后强制分段
	maxBufferSize   int           // 最大缓冲区大小：超过此大小强制分段（防止超过飞书150KB限制）

	// 定时器控制
	lastDataTime    time.Time     // 最后一次收到数据的时间
	durationTimer   *time.Timer   // 持续时间定时器
	idleTimer       *time.Timer   // 空闲超时定时器
	stopTimers      chan struct{} // 停止定时器信号
	wg              sync.WaitGroup
}

// NewStreamingTextHandler 创建流式文本处理器
func NewStreamingTextHandler(feishuClient *client.FeishuClient) *StreamingTextHandler {
	// 使用统一超时配置
	timeoutConfig := utils.DefaultTimeoutConfig()

	return &StreamingTextHandler{
		feishuClient:  feishuClient,
		idleTimeout:   timeoutConfig.StreamIdleTimeout,
		maxDuration:   timeoutConfig.StreamMaxDuration,
		maxBufferSize: timeoutConfig.StreamMaxBufferSize,
		logger:        log.New(os.Stdout, "[StreamingTextHandler] ", log.LstdFlags),
		stopTimers:    make(chan struct{}),
	}
}

// HandleMessage 处理消息（基于时间的智能分段发送）
func (h *StreamingTextHandler) HandleMessage(ctx context.Context, token, receiveID, receiveIDType, userMessage, resumeSessionID, projectDir string) error {
	h.logger.Printf("Processing message with time-based streaming mode: receive_id=%s type=%s project_dir=%s", receiveID, receiveIDType, projectDir)

	// 初始化状态
	h.receiveID = receiveID
	h.receiveIDType = receiveIDType
	h.buffer = make([]rune, 0)
	h.lastDataTime = time.Now()
	h.stopTimers = make(chan struct{})

	// 初始化 Claude 管理器（带项目目录）
	config := ClaudeConfig{}
	if projectDir != "" {
		config.ProjectDir = projectDir
		h.logger.Printf("Using project directory: %s", projectDir)
	}
	h.claudeManager = NewClaudeManager(config)

	// 设置文本增量回调 - 基于时间智能分段
	h.claudeManager.SetTextDeltaCallback(func(text string, sequence int) error {
		h.logger.Printf("[TextDelta] seq=%d text_len=%d", sequence, len(text))
		return h.onTextDelta(text)
	})

	// 设置完成回调 - 发送剩余所有内容并停止定时器
	h.claudeManager.SetCompleteCallback(func(finalText string) error {
		h.logger.Printf("[Complete] final_text_len=%d", len(finalText))
		h.stopAllTimers()
		return h.sendRemaining()
	})

	// 设置错误回调
	h.claudeManager.SetErrorCallback(func(err error) {
		h.logger.Printf("[Error] Claude error: %v", err)
		h.stopAllTimers()
	})

	// 启动 Claude CLI
	h.logger.Printf("Starting Claude CLI...")
	if err := h.claudeManager.Start(ctx, userMessage, resumeSessionID); err != nil {
		h.stopAllTimers()
		return fmt.Errorf("failed to start claude: %w", err)
	}

	// 等待完成
	h.logger.Printf("Waiting for Claude output...")
	if err := h.claudeManager.WaitForOutput(ctx); err != nil {
		h.logger.Printf("Claude output wait error: %v", err)

		// 检测是否是 session resume 失败
		if strings.Contains(err.Error(), "No conversation found") && resumeSessionID != "" {
			h.logger.Printf("Session resume failed, retrying without resume...")
			h.stopAllTimers()

			// 重新初始化 manager，不使用 resume
			config := ClaudeConfig{}
			if projectDir != "" {
				config.ProjectDir = projectDir
			}
			h.claudeManager = NewClaudeManager(config)

			// 重新设置回调
			h.claudeManager.SetTextDeltaCallback(func(text string, sequence int) error {
				h.logger.Printf("[TextDelta] seq=%d text_len=%d", sequence, len(text))
				return h.onTextDelta(text)
			})

			h.claudeManager.SetCompleteCallback(func(finalText string) error {
				h.logger.Printf("[Complete] final_text_len=%d", len(finalText))
				h.stopAllTimers()
				return h.sendRemaining()
			})

			h.claudeManager.SetErrorCallback(func(err error) {
				h.logger.Printf("[Error] %v", err)
			})

			// 重新启动（不使用 resume）
			if err := h.claudeManager.Start(ctx, userMessage, ""); err != nil {
				h.stopAllTimers()
				return fmt.Errorf("failed to start claude (retry): %w", err)
			}

			// 重新等待
			if err := h.claudeManager.WaitForOutput(ctx); err != nil {
				h.logger.Printf("Claude output wait error (retry): %v", err)
			}

			if err := h.claudeManager.WaitForExit(); err != nil {
				h.logger.Printf("Claude exited with error (retry): %v", err)
			}
		}
	}
	h.stopAllTimers()

	if err := h.claudeManager.WaitForExit(); err != nil {
		h.logger.Printf("Claude exited with error: %v", err)
	}

	h.lastSessionID = h.claudeManager.GetSessionID()
	h.logger.Printf("Message processing completed, session_id=%s", h.lastSessionID)

	return nil
}

// onTextDelta 收到文本增量时的处理
func (h *StreamingTextHandler) onTextDelta(text string) error {
	h.bufferMu.Lock()
	defer h.bufferMu.Unlock()

	// 计算增量：text 是完整累积文本，我们需要只追加新增部分
	fullTextRunes := []rune(text)
	newLen := len(fullTextRunes)

	// 只追加新增部分
	if newLen > h.lastFullLen {
		newContent := fullTextRunes[h.lastFullLen:]
		h.buffer = append(h.buffer, newContent...)
		h.lastFullLen = newLen
		h.logger.Printf("[Buffer] accumulated=%d chars, new_increment=%d chars, full_text=%d chars",
			len(h.buffer), len(newContent), newLen)
	} else {
		// 可能是重复事件，忽略
		h.logger.Printf("[Buffer] Ignoring duplicate text: new_len=%d last_len=%d", newLen, h.lastFullLen)
		return nil
	}

	now := time.Now()
	h.lastDataTime = now

	// 检查缓冲区是否超过最大限制
	for len(h.buffer) >= h.maxBufferSize {
		// 强制分段发送
		chunk := string(h.buffer[:h.maxBufferSize])
		h.buffer = h.buffer[h.maxBufferSize:]

		h.logger.Printf("[Buffer] Max buffer size %d reached, force sending chunk", h.maxBufferSize)
		h.bufferMu.Unlock() // 临时解锁以发送消息
		if err := h.sendMessage(chunk); err != nil {
			h.logger.Printf("[Buffer] Failed to send forced chunk: %v", err)
			h.bufferMu.Lock()
			return err
		}
		h.bufferMu.Lock()
	}

	// 如果缓冲区为空（刚被清空），启动持续时间定时器
	if len(h.buffer) > 0 && len(h.buffer) == len([]rune(text))-h.lastFullLen+len(h.buffer) {
		h.startDurationTimer()
	}

	// 重置空闲超时定时器
	h.resetIdleTimer()

	return nil
}

// startDurationTimer 启动持续时间定时器（超长强制分段）
func (h *StreamingTextHandler) startDurationTimer() {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()

		if h.durationTimer != nil {
			h.durationTimer.Stop()
		}
		h.durationTimer = time.NewTimer(h.maxDuration)

		select {
		case <-h.durationTimer.C:
			h.logger.Printf("[DurationTimer] Max duration %v reached, force sending", h.maxDuration)
			h.bufferMu.Lock()
			if len(h.buffer) > 0 {
				chunk := string(h.buffer)
				h.buffer = make([]rune, 0)
				h.bufferMu.Unlock()

				if err := h.sendMessage(chunk); err != nil {
					h.logger.Printf("[DurationTimer] Failed to send: %v", err)
				}
			} else {
				h.bufferMu.Unlock()
			}
		case <-h.stopTimers:
			h.logger.Printf("[DurationTimer] Stopped")
			return
		}
	}()
}

// resetIdleTimer 重置空闲超时定时器
func (h *StreamingTextHandler) resetIdleTimer() {
	if h.idleTimer != nil {
		h.idleTimer.Stop()
	}

	h.idleTimer = time.NewTimer(h.idleTimeout)

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()

		select {
		case <-h.idleTimer.C:
			// 检查是否有新数据到达
			idleTime := time.Since(h.lastDataTime)
			if idleTime >= h.idleTimeout {
				h.logger.Printf("[IdleTimer] Idle timeout %v reached, sending buffer", h.idleTimeout)
				h.bufferMu.Lock()
				if len(h.buffer) > 0 {
					chunk := string(h.buffer)
					h.buffer = make([]rune, 0)
					h.bufferMu.Unlock()

					if err := h.sendMessage(chunk); err != nil {
						h.logger.Printf("[IdleTimer] Failed to send: %v", err)
					}
				} else {
					h.bufferMu.Unlock()
				}
			}
		case <-h.stopTimers:
			h.logger.Printf("[IdleTimer] Stopped")
			return
		}
	}()
}

// stopAllTimers 停止所有定时器
func (h *StreamingTextHandler) stopAllTimers() {
	close(h.stopTimers)

	if h.durationTimer != nil {
		h.durationTimer.Stop()
	}
	if h.idleTimer != nil {
		h.idleTimer.Stop()
	}

	h.wg.Wait()

	// 重新创建 stopTimers channel 以备下次使用
	h.stopTimers = make(chan struct{})
}

// sendRemaining 发送缓冲区剩余的所有内容
func (h *StreamingTextHandler) sendRemaining() error {
	h.bufferMu.Lock()
	defer h.bufferMu.Unlock()

	if len(h.buffer) == 0 {
		h.logger.Printf("No remaining content to send")
		return nil
	}

	chunk := string(h.buffer)
	h.logger.Printf("Sending remaining content: %d chars", len(chunk))
	if err := h.sendMessage(chunk); err != nil {
		return err
	}

	// 清空缓冲区
	h.buffer = make([]rune, 0)
	return nil
}

// sendMessage 发送文本消息到飞书
func (h *StreamingTextHandler) sendMessage(content string) error {
	h.logger.Printf("Sending message: len=%d", len(content))

	if err := h.feishuClient.SendMessage(h.receiveID, h.receiveIDType, content); err != nil {
		h.logger.Printf("Failed to send message: %v", err)
		return err
	}

	h.logger.Printf("Message sent successfully")
	return nil
}

// SessionID 返回会话 ID
func (h *StreamingTextHandler) SessionID() string {
	return h.lastSessionID
}

// SetIdleTimeout 设置空闲超时时间
func (h *StreamingTextHandler) SetIdleTimeout(timeout time.Duration) {
	h.idleTimeout = timeout
}

// SetMaxDuration 设置最大持续时间
func (h *StreamingTextHandler) SetMaxDuration(duration time.Duration) {
	h.maxDuration = duration
}
