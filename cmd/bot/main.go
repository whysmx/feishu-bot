package main

import (
	"bufio"
	"context"
	"feishu-bot/internal/bot/client"
	"feishu-bot/internal/bot/handlers"
	"feishu-bot/internal/notification"
	"feishu-bot/internal/session"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkapplication "github.com/larksuite/oapi-sdk-go/v3/service/application/v6"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

func main() {
	log.Printf("Starting Feishu Bot... version=%s build_time=%s commit=%s", buildVersion, buildTime, buildCommit)

	if path, loaded := loadDotEnv(".env", "feishu-bot/.env"); loaded {
		log.Printf("Loaded .env from %s", path)
	} else {
		log.Println("No .env found; relying on existing environment variables")
	}

	// 获取配置
	appID := getEnv("FEISHU_APP_ID", "")
	if appID == "" {
		log.Fatal("FEISHU_APP_ID is required")
	}
	appSecret := getEnv("FEISHU_APP_SECRET", "")
	if appSecret == "" {
		log.Fatal("FEISHU_APP_SECRET is required")
	}
	log.Printf("Using FEISHU_APP_ID=%s", appID)

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
	messageHandler := handlers.NewMessageHandler(sessionManager, nil, notificationSender, feishuClient)

	// 初始化卡片交互处理器
	cardHandler := handlers.NewCardActionHandler(sessionManager, nil, notificationSender)

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
			}

			return nil
		}).
		// 处理卡片交互事件 - 按照官方示例的方式处理
		OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
			log.Printf("[OnP2CardActionTrigger] Card action triggered: %s", larkcore.Prettify(event))

			// 读取action值
			if event.Event.Action.Value == nil {
				log.Printf("No action value in card event")
				return &callback.CardActionTriggerResponse{}, nil
			}

			action, ok := event.Event.Action.Value["action"].(string)
			if !ok {
				log.Printf("Cannot parse action from card event")
				return &callback.CardActionTriggerResponse{}, nil
			}

			log.Printf("Processing card action: %s", action)

			// 处理不同的action
			switch action {
			case "complete_alarm":
				// 读取表单输入值
				notes := ""
				if event.Event.Action.FormValue != nil {
					if n, ok := event.Event.Action.FormValue["notes_input"]; ok {
						if str, ok := n.(string); ok {
							notes = str
						} else {
							notes = fmt.Sprintf("%v", n)
						}
					}
				}

				// 构造响应，更新卡片为完成状态
				response := &callback.CardActionTriggerResponse{
					Toast: &callback.Toast{
						Type:    "info",
						Content: "处理完成！",
					},
					Card: &callback.Card{
						Type: "template",
						Data: &callback.TemplateCard{
							TemplateID: "AAqz1Y1QyEzLF", // 使用完成状态的卡片模板
							TemplateVariable: map[string]interface{}{
								"complete_time": time.Now().Format("2006-01-02 15:04:05 (UTC+8)"), // 动态完成时间
								"notes":         notes,                                            // 用户输入的备注
								"open_id":       event.Event.Operator.OpenID,                      // 处理人
							},
						},
					},
				}

				log.Printf("Card action processed successfully, notes: %s", notes)
				return response, nil

			case "send_command", "continue_work", "view_status", "view_session", "view_options", "end_session", "retry_command":
				// 对于我们系统的特定action，调用内部handler
				response, err := cardHandler.HandleCardAction(ctx, event)
				if err != nil {
					log.Printf("Failed to handle card action: %v", err)
					return &callback.CardActionTriggerResponse{
						Toast: &callback.Toast{
							Type:    "error",
							Content: "处理失败",
						},
					}, nil
				}
				return response, nil

			default:
				log.Printf("Unknown card action: %s", action)
				return &callback.CardActionTriggerResponse{
					Toast: &callback.Toast{
						Type:    "warning",
						Content: "未知的操作",
					},
				}, nil
			}
		})

	wsLogLevel := larkcore.LogLevelInfo
	if logLevel == "debug" {
		wsLogLevel = larkcore.LogLevelDebug
	}

	// 启动WebSocket长连接
	wsClient := larkws.NewClient(appID, appSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(wsLogLevel),
	)

	log.Println("Starting WebSocket connection to Feishu...")
	err = wsClient.Start(context.Background())
	if err != nil {
		log.Fatalf("Failed to start WebSocket client: %v", err)
	}
}

var (
	buildVersion = "dev"
	buildTime    = "unknown"
	buildCommit  = "unknown"
)

// sendWelcomeMessage 发送欢迎消息
func sendWelcomeMessage(sender notification.NotificationSender, openID string) error {
welcomeText := `🎉 欢迎使用 Claude CLI 对话机器人！

使用方法：
• 直接发送任何消息即可开始对话

说明：
• 所有消息会直接透传给 Claude CLI
• 不做命令拦截或二次加工`

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
	helpText := `💡 使用说明

• 直接发送任何消息即可对话

说明：
• 所有消息会直接透传给 Claude CLI
• 不做命令拦截或二次加工`

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

func loadDotEnv(paths ...string) (string, bool) {
	for _, candidate := range paths {
		path := candidate
		if !filepath.IsAbs(path) {
			if wd, err := os.Getwd(); err == nil {
				path = filepath.Join(wd, path)
			}
		}

		if _, err := os.Stat(path); err != nil {
			continue
		}

		file, err := os.Open(path)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "export ") {
				line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key == "" {
				continue
			}
			value = strings.Trim(value, `"'`)
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, value)
			}
		}

		return path, true
	}

	return "", false
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

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	}
	return defaultValue
}
