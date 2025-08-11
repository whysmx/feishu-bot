package main

import (
	"feishu-bot/internal/command"
	"feishu-bot/internal/session"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	log.Println("Starting Command Relay Service...")

	// 获取配置
	sessionStorageFile := getEnv("SESSION_STORAGE_FILE", "data/sessions.json")
	logLevel := getEnv("LOG_LEVEL", "info")
	cleanupInterval := getEnvInt("CLEANUP_INTERVAL_MINUTES", 30)

	// 设置日志
	if logLevel == "debug" {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// 初始化会话管理器
	sessionManager, err := session.NewSessionManager(sessionStorageFile, session.SessionConfig{
		TokenLength:            8,
		ExpirationHours:        getEnvInt("SESSION_EXPIRATION_HOURS", 24),
		CleanupIntervalMinutes: cleanupInterval,
	})
	if err != nil {
		log.Fatalf("Failed to initialize session manager: %v", err)
	}

	// 初始化命令执行器
	commandExecutor := command.NewTmuxCommandExecutor(sessionManager)

	// 创建中继服务
	relayService := &CommandRelayService{
		sessionManager:  sessionManager,
		commandExecutor: commandExecutor,
		logger:          log.New(os.Stdout, "[RelayService] ", log.LstdFlags),
	}

	// 启动服务
	if err := relayService.Start(); err != nil {
		log.Fatalf("Failed to start relay service: %v", err)
	}

	// 等待终止信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Command Relay Service is running... Press Ctrl+C to stop")
	<-sigChan

	log.Println("Shutting down Command Relay Service...")
	relayService.Stop()
}

// CommandRelayService 命令中继服务
type CommandRelayService struct {
	sessionManager  session.SessionManager
	commandExecutor command.CommandExecutor
	logger          *log.Logger
	running         bool
	stopChan        chan bool
}

// Start 启动服务
func (crs *CommandRelayService) Start() error {
	crs.running = true
	crs.stopChan = make(chan bool)

	crs.logger.Println("Command relay service started")

	// 启动定期清理协程
	go crs.startCleanupRoutine()

	// 启动命令处理协程（在实际实现中，这里会监听消息队列或其他输入源）
	go crs.startCommandProcessor()

	return nil
}

// Stop 停止服务
func (crs *CommandRelayService) Stop() {
	if !crs.running {
		return
	}

	crs.running = false
	close(crs.stopChan)
	crs.logger.Println("Command relay service stopped")
}

// startCleanupRoutine 启动清理协程
func (crs *CommandRelayService) startCleanupRoutine() {
	ticker := time.NewTicker(30 * time.Minute) // 每30分钟清理一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if cleaned, err := crs.sessionManager.CleanupExpiredSessions(); err != nil {
				crs.logger.Printf("Error during session cleanup: %v", err)
			} else if cleaned > 0 {
				crs.logger.Printf("Cleaned up %d expired sessions", cleaned)
			}

		case <-crs.stopChan:
			return
		}
	}
}

// startCommandProcessor 启动命令处理器（示例实现）
func (crs *CommandRelayService) startCommandProcessor() {
	crs.logger.Println("Command processor started")

	// 这里是一个示例实现，在实际使用中，这里会：
	// 1. 监听消息队列（如Redis、RabbitMQ）
	// 2. 监听Webhook回调
	// 3. 监听文件系统变化
	// 4. 或其他外部输入源

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 示例：检查是否有新的命令需要处理
			crs.processPendingCommands()

		case <-crs.stopChan:
			crs.logger.Println("Command processor stopped")
			return
		}
	}
}

// processPendingCommands 处理待处理的命令（示例实现）
func (crs *CommandRelayService) processPendingCommands() {
	// 这是一个示例实现，展示如何处理命令
	// 在实际实现中，命令会从消息队列或其他源获取

	sessions, err := crs.sessionManager.ListAllSessions()
	if err != nil {
		crs.logger.Printf("Failed to list sessions: %v", err)
		return
	}

	// 这里只是演示，不会实际执行任何命令
	if len(sessions.Sessions) > 0 {
		crs.logger.Printf("Monitoring %d active sessions", sessions.ActiveCount)
	}
}

// ProcessCommand 处理单个命令（公开方法，供其他服务调用）
func (crs *CommandRelayService) ProcessCommand(req *command.CommandRequest) (*command.CommandResult, error) {
	crs.logger.Printf("Processing command for token %s: %s", req.Token, req.Command)

	// 执行命令
	result, err := crs.commandExecutor.ExecuteCommand(req)
	if err != nil {
		crs.logger.Printf("Command execution failed: %v", err)
		return nil, err
	}

	crs.logger.Printf("Command execution completed: success=%v, method=%s", 
		result.Success, result.Method)

	return result, nil
}

// GetSessionInfo 获取会话信息
func (crs *CommandRelayService) GetSessionInfo(token string) (*session.Session, error) {
	return crs.sessionManager.GetSession(token)
}

// GetServiceStats 获取服务统计信息
func (crs *CommandRelayService) GetServiceStats() map[string]interface{} {
	sessions, err := crs.sessionManager.ListAllSessions()
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	return map[string]interface{}{
		"running":         crs.running,
		"total_sessions":  sessions.Total,
		"active_sessions": sessions.ActiveCount,
		"timestamp":       time.Now(),
	}
}

// getEnv 获取环境变量，如果不存在则使用默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt 获取整数环境变量
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}