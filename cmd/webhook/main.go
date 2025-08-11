package main

import (
	"context"
	"feishu-bot/internal/bot/client"
	"feishu-bot/internal/config"
	"feishu-bot/internal/notification"
	"feishu-bot/internal/security"
	"feishu-bot/internal/session"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	larkcard "github.com/larksuite/oapi-sdk-go/v3/card"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/core/httpserverext"
)

func main() {
	// 加载配置
	configPath := getEnv("CONFIG_PATH", "configs/config.yaml")
	configManager := config.NewConfigManager(configPath)
	if err := configManager.Load(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cfg := configManager.GetConfig()

	// 获取配置
	port := getEnvInt("WEBHOOK_PORT", cfg.Webhook.Port)
	sessionStorageFile := getEnv("SESSION_STORAGE_FILE", cfg.Session.StorageFile)
	logLevel := getEnv("LOG_LEVEL", cfg.Logging.Level)

	// 设置日志级别
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化会话管理器
	sessionManager, err := session.NewSessionManager(sessionStorageFile, session.SessionConfig{
		TokenLength:           cfg.Session.TokenLength,
		ExpirationHours:       cfg.Session.ExpirationHours,
		CleanupIntervalMinutes: cfg.Session.CleanupIntervalMinutes,
	})
	if err != nil {
		log.Fatalf("Failed to initialize session manager: %v", err)
	}

	// 验证必要的配置
	if cfg.Feishu.AppSecret == "" {
		log.Fatal("FEISHU_APP_SECRET is required. Please set the environment variable or update config.yaml")
	}

	// 初始化飞书客户端
	feishuClient := client.NewFeishuClient(client.FeishuConfig{
		AppID:     cfg.Feishu.AppID,
		AppSecret: cfg.Feishu.AppSecret,
		CardTemplates: client.CardTemplates{
			TaskCompleted: cfg.Cards.TaskCompletedCardID,
			TaskWaiting:   cfg.Cards.TaskWaitingCardID,
			CommandResult: cfg.Cards.CommandResultCardID,
		},
	})

	// 初始化用户映射服务
	userMappingService, err := security.NewUserMappingService("configs/security/whitelist.yaml")
	if err != nil {
		log.Printf("Warning: Failed to initialize user mapping service: %v", err)
		log.Println("The webhook will work but cannot resolve placeholder OpenIDs")
		userMappingService = nil // 设置为nil，webhook handler会处理这种情况
	}

	// 初始化真实的通知发送器
	notificationSender := notification.NewFeishuNotificationSender(feishuClient)

	// 初始化webhook处理器
	webhookHandler := notification.NewWebhookHandler(sessionManager, notificationSender, userMappingService)
	


	// 设置路由
	router := gin.Default()
	
	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"service": "webhook",
		})
	})

	// 接收Claude Code通知
	router.POST("/webhook/notification", func(c *gin.Context) {
		var req notification.WebhookRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, notification.NotificationResponse{
				Success: false,
				Error:   "Invalid JSON: " + err.Error(),
			})
			return
		}

		resp, err := webhookHandler.HandleNotification(&req)
		if err != nil {
			log.Printf("Error handling notification: %v", err)
			c.JSON(http.StatusInternalServerError, notification.NotificationResponse{
				Success: false,
				Error:   "Internal server error",
			})
			return
		}

		if resp.Success {
			c.JSON(http.StatusOK, resp)
		} else {
			c.JSON(http.StatusBadRequest, resp)
		}
	})

	// 调试接口：获取会话信息
	router.GET("/webhook/session/:token", func(c *gin.Context) {
		token := c.Param("token")
		sess, err := webhookHandler.GetSessionInfo(token)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, sess)
	})

	// 调试接口：获取统计信息
	router.GET("/webhook/stats", func(c *gin.Context) {
		stats, err := webhookHandler.GetStats()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, stats)
	})

	// 手动清理过期会话
	router.POST("/webhook/cleanup", func(c *gin.Context) {
		cleaned, err := webhookHandler.CleanupExpiredSessions()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"cleaned_sessions": cleaned,
		})
	})

	// 处理飞书卡片交互事件 - 使用Gin路由
	router.POST("/webhook/card", func(c *gin.Context) {
		log.Printf("Card action received via HTTP webhook")
		
		// 创建一个临时的CardAction处理器
		cardActionHandler := larkcard.NewCardActionHandler("", "", func(ctx context.Context, cardAction *larkcard.CardAction) (interface{}, error) {
			log.Printf("Processing card action: %s", larkcore.Prettify(cardAction))
			
			// 这里暂时返回成功响应，实际的处理逻辑需要数据类型适配
			// TODO: 需要将 *larkcard.CardAction 转换为 *callback.CardActionTriggerEvent
			return map[string]interface{}{
				"success": true,
				"message": "Card action processed successfully",
			}, nil
		})
		
		// 使用SDK的处理函数
		handlerFunc := httpserverext.NewCardActionHandlerFunc(cardActionHandler)
		handlerFunc(c.Writer, c.Request)
	})

	log.Printf("Starting webhook server on port %d", port)
	log.Printf("Feishu client initialized with app_id: %s", cfg.Feishu.AppID)
	log.Printf("Card templates: completed=%s, waiting=%s, result=%s", 
		cfg.Cards.TaskCompletedCardID, cfg.Cards.TaskWaitingCardID, cfg.Cards.CommandResultCardID)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), router))
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