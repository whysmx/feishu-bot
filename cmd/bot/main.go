package main

import (
	"bufio"
	"context"
	"feishu-bot/internal/bot/client"
	"feishu-bot/internal/bot/handlers"
	"feishu-bot/internal/utils"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkapplication "github.com/larksuite/oapi-sdk-go/v3/service/application/v6"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

func main() {
	log.Printf("Starting Feishu Bot... version=%s build_time=%s commit=%s", buildVersion, buildTime, buildCommit)

	if path, loaded := loadDotEnv(".env"); loaded {
		log.Printf("Loaded .env from %s", path)
	} else {
		log.Println("No .env found; relying on existing environment variables")
	}

	// 验证必需的环境变量
	if err := validateConfig(); err != nil {
		log.Fatalf("配置验证失败: %v\n请检查 .env 文件是否配置正确", err)
	}

	// 获取配置
	appID := getEnv("FEISHU_APP_ID", "")
	appSecret := getEnv("FEISHU_APP_SECRET", "")
	log.Printf("Using FEISHU_APP_ID=%s", appID)

	logLevel := getEnv("LOG_LEVEL", "info")
	larkLogLevel := larkcore.LogLevelInfo
	if logLevel == "debug" {
		larkLogLevel = larkcore.LogLevelDebug
	}

	// 设置日志级别
	if logLevel == "debug" {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// 初始化飞书客户端
	feishuClient := client.NewFeishuClient(client.FeishuConfig{
		AppID:     appID,
		AppSecret: appSecret,
	})

	// 启动时做一次 token 自检，便于定位偶发 10014
	if token, err := feishuClient.GetTenantAccessToken(); err != nil {
		log.Printf("Tenant token self-check failed: %v", err)
	} else {
		log.Printf("Tenant token self-check ok: token=%s", token)
	}

	// 初始化消息处理器
	messageHandler := handlers.NewMessageHandler(feishuClient)

	// 注册事件处理器
	eventHandler := dispatcher.NewEventDispatcher("", "").
		// 处理用户进入机器人单聊事件
		OnP2ChatAccessEventBotP2pChatEnteredV1(func(ctx context.Context, event *larkim.P2ChatAccessEventBotP2pChatEnteredV1) error {
			log.Printf("[OnP2ChatAccessEventBotP2pChatEnteredV1] User entered: %s", larkcore.Prettify(event))
			return nil
		}).
		// 处理用户点击机器人菜单事件
		OnP2BotMenuV6(func(ctx context.Context, event *larkapplication.P2BotMenuV6) error {
			log.Printf("[OnP2BotMenuV6] Menu clicked: %s", larkcore.Prettify(event))
			return nil
		}).
		// 接收用户发送的消息
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			appendEventTrace("ws_recv", event)
			log.Printf("[OnP2MessageReceiveV1] Ack immediately; processing async")
			go func(ev *larkim.P2MessageReceiveV1) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[OnP2MessageReceiveV1] Panic recovered: %v", r)
					}
				}()

				appendEventTrace("handler_async", ev)
				eventID := ""
				if ev != nil && ev.EventV2Base != nil && ev.EventV2Base.Header != nil {
					eventID = ev.EventV2Base.Header.EventID
				}
				messageID := ""
				chatType := ""
				if ev != nil && ev.Event != nil && ev.Event.Message != nil {
					if ev.Event.Message.MessageId != nil {
						messageID = *ev.Event.Message.MessageId
					}
					if ev.Event.Message.ChatType != nil {
						chatType = *ev.Event.Message.ChatType
					}
				}
				log.Printf("[OnP2MessageReceiveV1] event_id=%s message_id=%s chat_type=%s", eventID, messageID, chatType)
				log.Printf("[OnP2MessageReceiveV1] Message received: %s", larkcore.Prettify(ev))
				if ev == nil || ev.Event == nil || ev.Event.Message == nil || ev.Event.Message.ChatType == nil {
					log.Printf("[OnP2MessageReceiveV1] Invalid event payload")
					return
				}

				chatType = *ev.Event.Message.ChatType
				if chatType == "p2p" {
					// 处理单聊消息
					if err := messageHandler.HandleP2PMessage(context.Background(), ev); err != nil {
						log.Printf("Failed to handle P2P message: %v", err)
					}
				} else if chatType == "group" || chatType == "private" {
					// 处理群聊消息
					if err := messageHandler.HandleGroupMessage(context.Background(), ev); err != nil {
						log.Printf("Failed to handle GROUP message: %v", err)
					}
				} else {
					log.Printf("[OnP2MessageReceiveV1] Unsupported chat_type=%s", chatType)
				}
			}(event)

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
							TemplateID: "", // 卡片功能已弃用，留空
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
	eventHandler.InitConfig(
		larkevent.WithLogger(prefixedLogger{prefix: "LARK-EVENT"}),
		larkevent.WithLogLevel(larkLogLevel),
	)

	wsLogLevel := larkLogLevel

	// 启动WebSocket长连接
	wsClient := larkws.NewClient(appID, appSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(wsLogLevel),
	)

	log.Println("Starting WebSocket connection to Feishu...")
	if err := wsClient.Start(context.Background()); err != nil {
		log.Fatalf("Failed to start WebSocket client: %v", err)
	}
}

var (
	buildVersion = "dev"
	buildTime    = "unknown"
	buildCommit  = "unknown"
)

type prefixedLogger struct {
	prefix string
}

func (pl prefixedLogger) Debug(ctx context.Context, args ...interface{}) {
	log.Printf("[%s][Debug] %s", pl.prefix, fmt.Sprint(args...))
}

func (pl prefixedLogger) Info(ctx context.Context, args ...interface{}) {
	log.Printf("[%s][Info] %s", pl.prefix, fmt.Sprint(args...))
}

func (pl prefixedLogger) Warn(ctx context.Context, args ...interface{}) {
	log.Printf("[%s][Warn] %s", pl.prefix, fmt.Sprint(args...))
}

func (pl prefixedLogger) Error(ctx context.Context, args ...interface{}) {
	log.Printf("[%s][Error] %s", pl.prefix, fmt.Sprint(args...))
}

func appendEventTrace(tag string, ev *larkim.P2MessageReceiveV1) {
	eventID := ""
	messageID := ""
	chatType := ""
	openID := ""
	if ev != nil && ev.EventV2Base != nil && ev.EventV2Base.Header != nil {
		eventID = ev.EventV2Base.Header.EventID
	}
	if ev != nil && ev.Event != nil && ev.Event.Message != nil {
		if ev.Event.Message.MessageId != nil {
			messageID = *ev.Event.Message.MessageId
		}
		if ev.Event.Message.ChatType != nil {
			chatType = *ev.Event.Message.ChatType
		}
	}
	if ev != nil && ev.Event != nil && ev.Event.Sender != nil && ev.Event.Sender.SenderId != nil && ev.Event.Sender.SenderId.OpenId != nil {
		openID = *ev.Event.Sender.SenderId.OpenId
	}
	line := fmt.Sprintf("%s pid=%d tag=%s event_id=%s message_id=%s chat_type=%s open_id=%s\n",
		time.Now().Format(time.RFC3339), os.Getpid(), tag, eventID, messageID, chatType, openID)
	writeTraceLine(line)
}

func writeTraceLine(line string) {
	traceLogPath := utils.GetTempFilePath("feishu-event-trace.log")
	file, err := os.OpenFile(traceLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		_, _ = file.WriteString(line)
		_ = file.Close()
		return
	}
	errorLogPath := utils.GetTempFilePath("feishu-event-trace.err")
	_ = os.WriteFile(errorLogPath, []byte(fmt.Sprintf("%s open_error=%v\n", time.Now().Format(time.RFC3339), err)), 0644)
	_ = os.WriteFile(traceLogPath, []byte(line), 0644)
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

// validateConfig 验证必需的环境变量
func validateConfig() error {
	requiredVars := []struct {
		name     string
		hint     string
		validate func(string) bool
	}{
		{
			name: "FEISHU_APP_ID",
			hint: "飞书应用 ID，格式如 cli_xxxxxxxx",
			validate: func(v string) bool {
				return strings.HasPrefix(v, "cli_")
			},
		},
		{
			name: "FEISHU_APP_SECRET",
			hint: "飞书应用密钥",
			validate: func(v string) bool {
				return len(v) >= 10
			},
		},
		{
			name: "ANTHROPIC_API_KEY",
			hint: "Anthropic API Key",
			validate: func(v string) bool {
				return strings.Contains(v, ".")
			},
		},
		{
			name: "ANTHROPIC_AUTH_TOKEN",
			hint: "Anthropic Auth Token",
			validate: func(v string) bool {
				return strings.Contains(v, ".")
			},
		},
	}

	var missing []string
	for _, rv := range requiredVars {
		value := os.Getenv(rv.name)
		if value == "" {
			missing = append(missing, fmt.Sprintf("  - %s: %s", rv.name, rv.hint))
		} else if rv.validate != nil && !rv.validate(value) {
			missing = append(missing, fmt.Sprintf("  - %s: 格式不正确，期望 %s", rv.name, rv.hint))
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("缺少必需的环境变量或格式错误:\n%s\n\n请在 .env 文件中配置这些变量", strings.Join(missing, "\n"))
	}

	return nil
}
