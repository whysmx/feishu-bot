package command

import (
	"feishu-bot/internal/session"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// tmuxCommandExecutor tmux命令执行器
type tmuxCommandExecutor struct {
	sessionManager session.SessionManager
	parser         CommandParser
	validator      CommandValidator
	logger         *log.Logger
}

// NewTmuxCommandExecutor 创建tmux命令执行器
func NewTmuxCommandExecutor(sessionManager session.SessionManager) CommandExecutor {
	return &tmuxCommandExecutor{
		sessionManager: sessionManager,
		parser:         NewCommandParser(),
		validator:      NewCommandValidator(),
		logger:         log.New(log.Writer(), "[CommandExecutor] ", log.LstdFlags),
	}
}

// ExecuteCommand 执行命令
func (tce *tmuxCommandExecutor) ExecuteCommand(req *CommandRequest) (*CommandResult, error) {
	startTime := time.Now()
	
	tce.logger.Printf("Executing command for token %s: %s", req.Token, req.Command)
	
	// 验证会话
	sess, err := tce.sessionManager.ValidateSession(req.Token)
	if err != nil {
		return &CommandResult{
			Token:     req.Token,
			Command:   req.Command,
			Success:   false,
			Method:    MethodFailed,
			Error:     fmt.Sprintf("session validation failed: %v", err),
			ExecTime:  time.Since(startTime).Milliseconds(),
			Timestamp: time.Now(),
		}, nil
	}
	
	// 验证用户权限
	if tce.validator != nil {
		if err := tce.validator.ValidateUser(req.UserID); err != nil {
			return &CommandResult{
				Token:     req.Token,
				Command:   req.Command,
				Success:   false,
				Method:    MethodFailed,
				Error:     fmt.Sprintf("user validation failed: %v", err),
				ExecTime:  time.Since(startTime).Milliseconds(),
				Timestamp: time.Now(),
			}, nil
		}
	}
	
	// 验证命令
	if err := tce.parser.ValidateCommand(req.Command); err != nil {
		return &CommandResult{
			Token:     req.Token,
			Command:   req.Command,
			Success:   false,
			Method:    MethodFailed,
			Error:     fmt.Sprintf("command validation failed: %v", err),
			ExecTime:  time.Since(startTime).Milliseconds(),
			Timestamp: time.Now(),
		}, nil
	}
	
	// 清理命令
	cleanCommand := SanitizeCommand(req.Command)
	
	// 尝试执行tmux命令
	result := tce.executeInTmux(sess.TmuxSession, cleanCommand)
	result.Token = req.Token
	result.Command = req.Command
	result.ExecTime = time.Since(startTime).Milliseconds()
	result.Timestamp = time.Now()
	
	// 更新会话活跃时间
	tce.sessionManager.UpdateSession(req.Token, &session.UpdateSessionRequest{})
	
	tce.logger.Printf("Command execution completed for token %s: success=%v, method=%s", 
		req.Token, result.Success, result.Method)
	
	return result, nil
}

// ExecuteInTmux 在tmux会话中执行命令
func (tce *tmuxCommandExecutor) ExecuteInTmux(sessionName, command string) error {
	result := tce.executeInTmux(sessionName, command)
	if !result.Success {
		return fmt.Errorf(result.Error)
	}
	return nil
}

// ValidateSession 验证会话
func (tce *tmuxCommandExecutor) ValidateSession(token string) error {
	_, err := tce.sessionManager.ValidateSession(token)
	return err
}

// executeInTmux 在tmux中执行命令
func (tce *tmuxCommandExecutor) executeInTmux(sessionName, command string) *CommandResult {
	tce.logger.Printf("Executing in tmux session '%s': %s", sessionName, command)
	
	// 首先检查tmux会话是否存在
	if !tce.tmuxSessionExists(sessionName) {
		return &CommandResult{
			Success: false,
			Method:  MethodFailed,
			Error:   fmt.Sprintf("tmux session '%s' does not exist", sessionName),
		}
	}
	
	// 使用tmux send-keys发送命令
	cmd := exec.Command("tmux", "send-keys", "-t", sessionName, command, "Enter")
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		tce.logger.Printf("tmux send-keys failed: %v, output: %s", err, string(output))
		
		// 尝试回退方法
		return tce.fallbackExecution(sessionName, command)
	}
	
	return &CommandResult{
		Success: true,
		Method:  MethodTmux,
		Output:  "Command sent to tmux session successfully",
	}
}

// fallbackExecution 回退执行方法
func (tce *tmuxCommandExecutor) fallbackExecution(sessionName, command string) *CommandResult {
	tce.logger.Printf("Attempting fallback execution for session %s", sessionName)
	
	// 方法1: 尝试创建新窗口并执行命令
	cmd := exec.Command("tmux", "new-window", "-t", sessionName, "-n", "claude-cmd", command)
	if err := cmd.Run(); err == nil {
		return &CommandResult{
			Success: true,
			Method:  MethodFallback,
			Output:  "Command executed in new tmux window",
		}
	}
	
	// 方法2: 尝试直接在会话中执行
	cmd = exec.Command("tmux", "send", "-t", sessionName, command, "C-m")
	if err := cmd.Run(); err == nil {
		return &CommandResult{
			Success: true,
			Method:  MethodFallback,
			Output:  "Command sent using alternative method",
		}
	}
	
	// 所有方法都失败
	return &CommandResult{
		Success: false,
		Method:  MethodFailed,
		Error:   "All execution methods failed",
	}
}

// tmuxSessionExists 检查tmux会话是否存在
func (tce *tmuxCommandExecutor) tmuxSessionExists(sessionName string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	err := cmd.Run()
	return err == nil
}

// listTmuxSessions 列出所有tmux会话
func (tce *tmuxCommandExecutor) listTmuxSessions() ([]string, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	sessions := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			sessions = append(sessions, line)
		}
	}
	
	return sessions, nil
}

// createTmuxSession 创建tmux会话
func (tce *tmuxCommandExecutor) createTmuxSession(sessionName, workingDir string) error {
	var cmd *exec.Cmd
	
	if workingDir != "" {
		cmd = exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", workingDir)
	} else {
		cmd = exec.Command("tmux", "new-session", "-d", "-s", sessionName)
	}
	
	return cmd.Run()
}

// killTmuxSession 杀死tmux会话
func (tce *tmuxCommandExecutor) killTmuxSession(sessionName string) error {
	cmd := exec.Command("tmux", "kill-session", "-t", sessionName)
	return cmd.Run()
}