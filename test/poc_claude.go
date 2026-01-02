package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// StreamEvent 表示 Claude CLI stream-json 输出的事件
type StreamEvent struct {
	Type   string  `json:"type"`
	Event  *Event  `json:"event,omitempty"` // 外层包装
	Delta  *Delta  `json:"delta,omitempty"`  // 旧格式兼容
	ContentBlock *ContentBlock `json:"content_block,omitempty"`
}

// Event 实际的事件内容
type Event struct {
	Type          string       `json:"type"`
	Index         int          `json:"index,omitempty"`
	Delta         *Delta       `json:"delta,omitempty"`
	ContentBlock  *ContentBlock `json:"content_block,omitempty"`
}

// Delta 文本增量
type Delta struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ContentBlock 内容块
type ContentBlock struct {
	Type string `json:"type"`
}

// NDJSONParser 解析 NDJSON 流
type NDJSONParser struct {
	currentBlockType string
}

// Parse 解析一行 NDJSON
func (p *NDJSONParser) Parse(line string) (text string, stop bool, err error) {
	var event StreamEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return "", false, err
	}

	// 处理 stream_event 包装
	if event.Type == "stream_event" && event.Event != nil {
		switch event.Event.Type {
		case "content_block_start":
			if event.Event.ContentBlock != nil {
				p.currentBlockType = event.Event.ContentBlock.Type
				fmt.Printf("[DEBUG] Content block started: type=%s\n", p.currentBlockType)
			}

		case "content_block_delta":
			if p.currentBlockType == "text" && event.Event.Delta != nil && event.Event.Delta.Type == "text_delta" {
				text = event.Event.Delta.Text
				return text, false, nil
			}

		case "message_stop":
			fmt.Printf("[DEBUG] Message stopped\n")
			return "", true, nil
		}
	}

	// 处理直接事件（没有 stream_event 包装）
	switch event.Type {
	case "content_block_start":
		if event.ContentBlock != nil {
			p.currentBlockType = event.ContentBlock.Type
			fmt.Printf("[DEBUG] Content block started: type=%s\n", p.currentBlockType)
		}

	case "content_block_delta":
		if p.currentBlockType == "text" && event.Delta != nil && event.Delta.Type == "text_delta" {
			text = event.Delta.Text
			return text, false, nil
		}

	case "message_stop":
		fmt.Printf("[DEBUG] Message stopped\n")
		return "", true, nil
	}

	return "", false, nil
}

func main() {
	fmt.Println("=== Claude CLI Stream-JSON PoC Test ===")
	fmt.Println("Starting Claude CLI with stream-json output...")
	fmt.Println()

	// 创建 Claude CLI 命令（不使用 --resume，创建新会话）
	cmd := exec.Command("claude",
		"-p",
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--verbose",
	)

	// 获取 stdin 和 stdout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Printf("[ERROR] Failed to get stdin: %v\n", err)
		os.Exit(1)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("[ERROR] Failed to get stdout: %v\n", err)
		os.Exit(1)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("[ERROR] Failed to get stderr: %v\n", err)
		os.Exit(1)
	}

	// 启动进程
	if err := cmd.Start(); err != nil {
		fmt.Printf("[ERROR] Failed to start Claude CLI: %v\n", err)
		os.Exit(1)
	}

	// 发送测试消息
	testMessage := "Hello! Please say 'Hi there!' and nothing else."
	fmt.Printf("[INPUT] Sending message: %s\n", testMessage)
	stdin.Write([]byte(testMessage + "\n"))
	stdin.Close()

	// 创建解析器
	parser := &NDJSONParser{}

	// 启动 goroutine 读取 stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Printf("[STDERR] %s\n", scanner.Text())
		}
	}()

	// 逐行读取 stdout 并解析
	scanner := bufio.NewScanner(stdout)
	lineCount := 0
	var output strings.Builder

	fmt.Println("\n=== Claude CLI Output (stream-json) ===")
	fmt.Println("[DEBUG] Showing first 5 raw lines:")
	fmt.Println()

	startTime := time.Now()
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		// 打印前 5 行原始内容用于调试
		if lineCount <= 5 {
			fmt.Printf("[RAW LINE %d] %s\n", lineCount, line)
		}

		text, stop, err := parser.Parse(line)
		if err != nil {
			fmt.Printf("[ERROR] Failed to parse line %d: %v\n", lineCount, err)
			continue
		}

		if text != "" {
			output.WriteString(text)
			fmt.Print(text) // 实时输出
		}

		if stop {
			fmt.Printf("\n[INFO] Message completed after %d lines\n", lineCount)
			break
		}
	}

	elapsed := time.Since(startTime)

	// 等待进程结束
	if err := cmd.Wait(); err != nil {
		fmt.Printf("\n[ERROR] Claude CLI exited with error: %v\n", err)
	} else {
		fmt.Printf("\n[SUCCESS] Claude CLI exited successfully\n")
	}

	// 打印统计信息
	fmt.Println("\n=== Test Summary ===")
	fmt.Printf("Total lines processed: %d\n", lineCount)
	fmt.Printf("Total output length: %d characters\n", output.Len())
	fmt.Printf("Time elapsed: %v\n", elapsed)
	fmt.Printf("Output:\n%s\n", output.String())
}
