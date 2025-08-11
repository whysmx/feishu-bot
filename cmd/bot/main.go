package main

import (
	"context"
	"feishu-bot/internal/bot/client"
	"feishu-bot/internal/bot/handlers"
	"feishu-bot/internal/notification"
	"feishu-bot/internal/session"
	"log"
	"os"
	"strconv"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkapplication "github.com/larksuite/oapi-sdk-go/v3/service/application/v6"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

func main() {
	log.Println("Starting Feishu Bot...")

	// 获取配置
	appID := getEnv("FEISHU_APP_ID", "cli_a8058428d478501c")
	appSecret := getEnv("FEISHU_APP_SECRET", "BMcKHGIcA3BeS2WlIrIPpdPp0qoupyjK")
	if appSecret == "" {
		log.Fatal("FEISHU_APP_SECRET is required")
	}

	sessionStorageFile := getEnv("SESSION_STORAGE_FILE", "data/sessions.json")
	logLevel := getEnv("LOG_LEVEL", "info")

	// 设置日志级别
	if logLevel == "debug" {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// 初始化会话管理器
	sessionManager, err := session.NewSessionManager(sessionStorageFile, session.SessionConfig{
		TokenLength:            8,
		ExpirationHours:        getEnvInt("SESSION_EXPIRATION_HOURS", 24),
		CleanupIntervalMinutes: 60,
	})
	if err != nil {
		log.Fatalf("Failed to initialize session manager: %v", err)
	}

	// 初始化飞书客户端
	feishuClient := client.NewFeishuClient(client.FeishuConfig{
		AppID:     appID,
		AppSecret: appSecret,
		CardTemplates: client.CardTemplates{
			TaskCompleted: getEnv("TASK_COMPLETED_CARD_ID", "AAqz1Y1QyEzLF"),
			TaskWaiting:   getEnv("TASK_WAITING_CARD_ID", "AAqz1Y1p8y5Se"),
			CommandResult: getEnv("COMMAND_RESULT_CARD_ID", "AAqz1Y1TvQB25"),
			SessionList:   getEnv("SESSION_LIST_CARD_ID", ""),
		},
	})

	// 初始化通知发送器
	notificationSender := notification.NewFeishuNotificationSender(feishuClient)

	// 初始化消息处理器（暂时传入nil作为命令执行器）
	messageHandler := handlers.NewMessageHandler(sessionManager, nil, notificationSender)

	// 注册事件处理器
	eventHandler := dispatcher.NewEventDispatcher("", "").
		// 处理用户进入机器人单聊事件
		OnP2ChatAccessEventBotP2pChatEnteredV1(func(ctx context.Context, event *larkim.P2ChatAccessEventBotP2pChatEnteredV1) error {
			log.Printf("[OnP2ChatAccessEventBotP2pChatEnteredV1] User entered: %s", larkcore.Prettify(event))

			openID := *event.Event.OperatorId.OpenId
			if err := sendWelcomeMessage(notificationSender, openID); err != nil {
				log.Printf("Failed to send welcome message: %v", err)
			}
			return nil
		}).
		// 处理用户点击机器人菜单事件
		OnP2BotMenuV6(func(ctx context.Context, event *larkapplication.P2BotMenuV6) error {
			log.Printf("[OnP2BotMenuV6] Menu clicked: %s", larkcore.Prettify(event))

			openID := *event.Event.Operator.OperatorId.OpenId
			eventKey := *event.Event.EventKey

			switch eventKey {
			case "help":
				return sendHelpMessage(notificationSender, openID)
			case "sessions":
				userID := *event.Event.Operator.OperatorId.UserId
				return handleSessionsFromMenu(messageHandler, openID, userID)
			default:
				return sendHelpMessage(notificationSender, openID)
			}
		}).
		// 接收用户发送的消息
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			log.Printf("[OnP2MessageReceiveV1] Message received: %s", larkcore.Prettify(event))

			chatType := *event.Event.Message.ChatType

			if chatType == "p2p" {
				// 单聊消息
				return messageHandler.HandleP2PMessage(ctx, event)
			} else if chatType == "group" {
				// 群聊消息
				return messageHandler.HandleGroupMessage(ctx, event)
			}

			return nil
		})

	// 启动WebSocket长连接
	wsClient := larkws.NewClient(appID, appSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)

	log.Println("Starting WebSocket connection to Feishu...")
	err = wsClient.Start(context.Background())
	if err != nil {
		log.Fatalf("Failed to start WebSocket client: %v", err)
	}
}

// sendWelcomeMessage 发送欢迎消息
func sendWelcomeMessage(sender notification.NotificationSender, openID string) error {
	welcomeText := `🎉 欢迎使用 Claude Code 远程控制机器人！

主要功能：
• 📬 接收 Claude Code 任务完成通知
• ⌨️ 远程发送命令到 Claude Code 会话
• 📊 查看和管理活跃会话
• 🔒 安全的令牌验证机制

使用方法：
1. 当 Claude Code 完成任务或需要输入时，您将收到通知卡片和唯一令牌
2. 通过 "令牌: 命令" 格式发送消息来远程控制，例如：ABC12345: run tests
3. 使用 /sessions 查看所有活跃会话
4. 使用 /help 获取帮助信息

开始您的远程开发之旅吧！`

	// 尝试发送文本消息
	if textSender, ok := sender.(interface {
		SendTextNotification(openID, message string) error
	}); ok {
		return textSender.SendTextNotification(openID, welcomeText)
	}

	log.Printf("Sending welcome message to %s", openID)
	return nil
}

// sendHelpMessage 发送帮助消息
func sendHelpMessage(sender notification.NotificationSender, openID string) error {
	helpText := `💡 Claude Code 远程控制机器人帮助

命令格式：
• <令牌>: <命令> - 执行远程命令，例如：ABC12345: npm test
• /sessions - 查看所有活跃会话
• /help - 显示此帮助信息

令牌说明：
• 每个任务会生成一个8位唯一令牌（如：ABC12345）
• 令牌有效期为24小时
• 使用令牌可以安全地控制对应的Claude Code会话

支持的命令示例：
• ABC12345: run tests - 运行测试
• ABC12345: git status - 查看Git状态
• ABC12345: npm run build - 构建项目
• ABC12345: help - 获取Claude Code帮助`

	if textSender, ok := sender.(interface {
		SendTextNotification(openID, message string) error
	}); ok {
		return textSender.SendTextNotification(openID, helpText)
	}

	log.Printf("Sending help message to %s", openID)
	return nil
}

// handleSessionsFromMenu 处理菜单中的会话命令
func handleSessionsFromMenu(handler *handlers.MessageHandler, openID, userID string) error {
	// 这里需要调用handler的方法，但handler的方法是私有的
	// 在实际实现中需要添加公开的方法
	log.Printf("Handling sessions command from menu for user %s", userID)
	return nil
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
