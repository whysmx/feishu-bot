package handlers

import (
	"context"
	"encoding/json"
	"feishu-bot/internal/bot/client"
	"feishu-bot/internal/claude"
	"feishu-bot/internal/config"
	"feishu-bot/internal/utils"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// MessageHandler 消息处理器
type MessageHandler struct {
	logger           *log.Logger
	feishuClient     *client.FeishuClient
	recentMessageIDs map[string]time.Time
	recentMessageMu  sync.Mutex
	claudeSessions   map[string]string
	claudeSessionMu  sync.Mutex
}

// NewMessageHandler 创建消息处理器
func NewMessageHandler(feishuClient *client.FeishuClient) *MessageHandler {
	return &MessageHandler{
		feishuClient:     feishuClient,
		logger:           log.New(log.Writer(), "[MessageHandler] ", log.LstdFlags),
		recentMessageIDs: make(map[string]time.Time),
		claudeSessions:   make(map[string]string),
	}
}

// HandleP2PMessage 处理单聊消息
func (mh *MessageHandler) HandleP2PMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	appendP2PTrace(event, "handler_enter")
	mh.logger.Printf("Received P2P message: %s", larkcore.Prettify(event))
	_ = os.WriteFile(utils.GetTempFilePath("feishu-last-p2p-event.json"), []byte(larkcore.Prettify(event)), 0644)

	// 安全检查防止 nil 指针 - 只检查必需的字段
	if event == nil || event.Event == nil || event.Event.Sender == nil ||
		event.Event.Sender.SenderId == nil || event.Event.Sender.SenderId.OpenId == nil {
		mh.logger.Printf("Invalid event structure: missing required fields")
		return fmt.Errorf("invalid event structure")
	}

	if mh.shouldIgnoreMessage(event) {
		return nil
	}

	// 获取消息内容
	content, err := mh.extractTextContent(event.Event.Message)
	if err != nil {
		mh.logger.Printf("Failed to extract message content: %v", err)
		return err
	}
	messageID := ""
	if event.Event.Message != nil && event.Event.Message.MessageId != nil {
		messageID = *event.Event.Message.MessageId
	}
	chatID := ""
	if event.Event.Message != nil && event.Event.Message.ChatId != nil {
		chatID = *event.Event.Message.ChatId
	}
	mh.logger.Printf("[DEBUG] P2P content extracted: message_id=%s chat_id=%s len=%d content=%q", messageID, chatID, len(content), content)

	openID := *event.Event.Sender.SenderId.OpenId
	// 使用UnionId作为用户标识符，如果不存在则使用OpenId
	var userID string
	if event.Event.Sender.SenderId.UnionId != nil {
		userID = *event.Event.Sender.SenderId.UnionId
	} else {
		// 使用OpenId作为备选
		userID = openID
	}

	// P2P场景固定使用open_id，避免卡片发送到非成员chat导致230002
	receiveID := openID
	receiveIDType := "open_id"
	mh.logger.Printf("✅✅✅ P2P MODE: Using open_id=%s", openID) // 明确的标记
	return mh.processMessage(openID, userID, receiveID, receiveIDType, content)
}

// HandleGroupMessage 处理群聊消息
func (mh *MessageHandler) HandleGroupMessage(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	mh.logger.Printf("Received GROUP message: %s", larkcore.Prettify(event))
	_ = os.WriteFile(utils.GetTempFilePath("feishu-last-group-event.json"), []byte(larkcore.Prettify(event)), 0644)

	// 安全检查防止 nil 指针
	if event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Message.ChatId == nil {
		mh.logger.Printf("Invalid group event structure: missing required fields")
		return fmt.Errorf("invalid group event structure")
	}

	if mh.shouldIgnoreMessage(event) {
		return nil
	}

	// 获取消息内容
	content, err := mh.extractTextContent(event.Event.Message)
	if err != nil {
		mh.logger.Printf("Failed to extract group message content: %v", err)
		return err
	}

	chatID := *event.Event.Message.ChatId
	messageID := ""
	if event.Event.Message.MessageId != nil {
		messageID = *event.Event.Message.MessageId
	}
	mh.logger.Printf("[DEBUG] GROUP content extracted: message_id=%s chat_id=%s len=%d content=%q", messageID, chatID, len(content), content)

	// 获取发送者信息（用于日志）
	openID := ""
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil && event.Event.Sender.SenderId.OpenId != nil {
		openID = *event.Event.Sender.SenderId.OpenId
	}

	var userID string
	if event.Event.Sender != nil && event.Event.Sender.SenderId != nil {
		if event.Event.Sender.SenderId.UnionId != nil {
			userID = *event.Event.Sender.SenderId.UnionId
		} else if event.Event.Sender.SenderId.OpenId != nil {
			userID = *event.Event.Sender.SenderId.OpenId
		}
	}

	// 群聊场景使用固定的全局会话ID和chat_id
	groupSessionID := "global_group_session"
	receiveID := chatID
	receiveIDType := "chat_id"
	mh.logger.Printf("✅✅✅ GROUP MODE: Using chat_id=%s global_session=%s sender=%s", chatID, groupSessionID, openID)

	// 检查是否 @机器人
	isMentioned := mh.isMentioned(event.Event.Message)
	mh.logger.Printf("[DEBUG] GROUP message: chat_id=%s is_mentioned=%t content=%q", chatID, isMentioned, content)

	// 如果 @机器人，检查是否为特殊命令
	if isMentioned {
		trimmedContent := strings.TrimSpace(content)

		// 移除 @提及部分（@xxx 开头的都会被移除）
		// 简单处理：按空格分割，取第一个非空部分之后的内容
		parts := strings.Fields(trimmedContent)
		if len(parts) > 0 && strings.HasPrefix(parts[0], "@") {
			// 第一个部分是 @xxx，跳过它
			trimmedContent = strings.Join(parts[1:], " ")
		}
		trimmedContent = strings.TrimSpace(trimmedContent)

		// 空消息，提示使用
		if trimmedContent == "" {
			return mh.sendTextMessage(receiveID, receiveIDType,
				"💡 提及机器人后输入问题即可对话\n发送 'help' 查看命令列表")
		}

		// 解析是否为特殊命令
		cmdType, cmdArgs, isCmd := parseCommand(trimmedContent)
		if isCmd {
			// 处理特殊命令（不转发给 Claude）
			switch cmdType {
			case "ls":
				return mh.handleLsCommand(receiveID)
			case "bind":
				return mh.handleBindCommand(receiveID, cmdArgs)
			case "help":
				return mh.handleHelpCommand(receiveID)
			}
		}

		// 不是特殊命令，正常转发给 Claude CLI
		return mh.processGroupMessage(groupSessionID, userID, receiveID, receiveIDType, trimmedContent)
	}

	// 不是 @机器人，正常处理对话
	return mh.processGroupMessage(groupSessionID, userID, receiveID, receiveIDType, content)
}

// processGroupMessage 处理群聊消息（使用全局共享会话）
func (mh *MessageHandler) processGroupMessage(sessionID, userID, receiveID, receiveIDType, content string) error {
	mh.logger.Printf("[DEBUG] processGroupMessage: session_id=%s user_id=%s receive_id=%s receive_id_type=%s len=%d", sessionID, userID, receiveID, receiveIDType, len(content))

	// 获取 tenant_access_token
	token, err := mh.feishuClient.GetTenantAccessToken()
	if err != nil {
		mh.logger.Printf("Failed to get tenant access token: %v", err)
		return fmt.Errorf("failed to get tenant access token: %w", err)
	}

	// 验证 receive_id 不为空
	if receiveID == "" {
		mh.logger.Printf("ERROR: receiveID is empty! receiveIDType=%s", receiveIDType)
		return fmt.Errorf("cannot send card: missing valid receive ID")
	}

	// 读取绑定的项目路径
	projectDir := ""
	cfg, err := config.Load()
	if err == nil {
		projectDir = cfg.GetProjectPath(receiveID)
		if projectDir != "" {
			mh.logger.Printf("[DEBUG] Using bound project path: %s", projectDir)
		}
	}

	// 创建 Claude 流式文本处理器（不使用 CardKit，节省 API 调用）
	streamingTextHandler := claude.NewStreamingTextHandler(mh.feishuClient)

	// 群聊使用固定的全局会话ID，实现所有群聊共享会话
	resumeSessionID := mh.getClaudeSession(sessionID)
	mh.logger.Printf("[DEBUG] Group chat using global session: %s (resume=%s)", sessionID, resumeSessionID)

	// 处理消息（流式分段发送，同步 CLI 输出节奏）
	ctx := context.Background()
	if err := streamingTextHandler.HandleMessage(ctx, token, receiveID, receiveIDType, content, resumeSessionID, projectDir); err != nil {
		mh.logger.Printf("Failed to handle group streaming text chat: %v", err)
		return fmt.Errorf("failed to handle group streaming text chat: %w", err)
	}

	// 保存全局会话ID
	if newSessionID := streamingTextHandler.SessionID(); newSessionID != "" {
		mh.setClaudeSession(sessionID, newSessionID)
		mh.logger.Printf("[DEBUG] Group chat session saved: %s -> %s", sessionID, newSessionID)
	}

	mh.logger.Printf("Group chat streaming text completed successfully for session %s", sessionID)
	return nil
}

func appendP2PTrace(event *larkim.P2MessageReceiveV1, tag string) {
	eventID := ""
	messageID := ""
	chatType := ""
	openID := ""
	if event != nil && event.EventV2Base != nil && event.EventV2Base.Header != nil {
		eventID = event.EventV2Base.Header.EventID
	}
	if event != nil && event.Event != nil && event.Event.Message != nil {
		if event.Event.Message.MessageId != nil {
			messageID = *event.Event.Message.MessageId
		}
		if event.Event.Message.ChatType != nil {
			chatType = *event.Event.Message.ChatType
		}
	}
	if event != nil && event.Event != nil && event.Event.Sender != nil && event.Event.Sender.SenderId != nil && event.Event.Sender.SenderId.OpenId != nil {
		openID = *event.Event.Sender.SenderId.OpenId
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

func (mh *MessageHandler) getClaudeSession(openID string) string {
	if openID == "" {
		return ""
	}
	mh.claudeSessionMu.Lock()
	defer mh.claudeSessionMu.Unlock()
	return mh.claudeSessions[openID]
}

func (mh *MessageHandler) setClaudeSession(openID, sessionID string) {
	if openID == "" || sessionID == "" {
		return
	}
	mh.claudeSessionMu.Lock()
	mh.claudeSessions[openID] = sessionID
	mh.claudeSessionMu.Unlock()
}

func (mh *MessageHandler) shouldIgnoreMessage(event *larkim.P2MessageReceiveV1) bool {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return false
	}

	if event.Event.Message.MessageId != nil {
		mh.logger.Printf("[DEBUG] shouldIgnoreMessage: message_id=%s", *event.Event.Message.MessageId)
	}

	if event.Event.Sender != nil && event.Event.Sender.SenderType != nil {
		senderType := strings.ToLower(strings.TrimSpace(*event.Event.Sender.SenderType))
		if senderType != "" && senderType != "user" {
			mh.logger.Printf("[DEBUG] Ignoring message: sender_type=%s", senderType)
			return true
		}
	}

	if event.Event.Message.MessageType != nil {
		messageType := strings.ToLower(strings.TrimSpace(*event.Event.Message.MessageType))
		if messageType != "" && messageType != "text" {
			mh.logger.Printf("[DEBUG] Ignoring non-text message: message_type=%s", messageType)
			return true
		}
	}

	if event.Event.Message.MessageId != nil && *event.Event.Message.MessageId != "" {
		if mh.isDuplicateMessage(*event.Event.Message.MessageId) {
			mh.logger.Printf("[DEBUG] Ignoring duplicate message: message_id=%s", *event.Event.Message.MessageId)
			return true
		}
	}

	return false
}

func (mh *MessageHandler) isDuplicateMessage(messageID string) bool {
	const dedupWindow = 30 * time.Minute
	now := time.Now()

	mh.recentMessageMu.Lock()
	defer mh.recentMessageMu.Unlock()

	if lastSeen, ok := mh.recentMessageIDs[messageID]; ok {
		if now.Sub(lastSeen) < dedupWindow {
			mh.logger.Printf("[DEBUG] Duplicate detected: message_id=%s last_seen=%s", messageID, lastSeen.Format(time.RFC3339))
			return true
		}
	}

	mh.recentMessageIDs[messageID] = now
	mh.logger.Printf("[DEBUG] Dedup record added: message_id=%s", messageID)

	for id, ts := range mh.recentMessageIDs {
		if now.Sub(ts) >= dedupWindow {
			delete(mh.recentMessageIDs, id)
		}
	}

	return false
}

// processMessage 处理消息的通用逻辑
func (mh *MessageHandler) processMessage(openID, userID, receiveID, receiveIDType, content string) error {
	mh.logger.Printf("[DEBUG] processMessage: open_id=%s user_id=%s receive_id=%s receive_id_type=%s len=%d", openID, userID, receiveID, receiveIDType, len(content))
	return mh.handleStreamingChat(openID, userID, receiveID, receiveIDType, content)
}

// extractTextContent 提取文本内容
func (mh *MessageHandler) extractTextContent(message interface{}) (string, error) {
	if message == nil {
		return "", fmt.Errorf("message is nil")
	}

	// 尝试从消息中提取Content字段
	var messageMap map[string]interface{}

	// 尝试直接传递的map
	if m, ok := message.(map[string]interface{}); ok {
		messageMap = m
	} else {
		// 尝试JSON转换
		messageBytes, err := json.Marshal(message)
		if err != nil {
			return "", fmt.Errorf("failed to marshal message: %w", err)
		}

		if err := json.Unmarshal(messageBytes, &messageMap); err != nil {
			return "", fmt.Errorf("failed to unmarshal message: %w", err)
		}
	}

	// 提取Content字段
	content, exists := messageMap["Content"]
	if !exists {
		// 尝试content字段（小写）
		content, exists = messageMap["content"]
		if !exists {
			return "", fmt.Errorf("no content field found in message")
		}
	}

	// 将content转换为字符串
	contentStr, ok := content.(string)
	if !ok {
		return "", fmt.Errorf("content is not a string")
	}
	mh.logger.Printf("[DEBUG] Raw message content: len=%d content=%q", len(contentStr), contentStr)

	// 解析JSON格式的文本内容
	var textContent map[string]interface{}
	if err := json.Unmarshal([]byte(contentStr), &textContent); err != nil {
		// 如果不是JSON格式，直接返回原内容
		return contentStr, nil
	}

	// 提取text字段
	text, exists := textContent["text"]
	if !exists {
		return "", fmt.Errorf("no text field found in content")
	}

	textStr, ok := text.(string)
	if !ok {
		return "", fmt.Errorf("text is not a string")
	}

	return textStr, nil
}

// isMentioned 检查是否@了机器人
func (mh *MessageHandler) isMentioned(message *larkim.EventMessage) bool {
	if message == nil {
		return false
	}
	return len(message.Mentions) > 0
}


// sendTextMessage 发送文本消息的便捷方法
func (mh *MessageHandler) sendTextMessage(receiveID, receiveIDType, text string) error {
	return mh.feishuClient.SendMessage(receiveID, receiveIDType, text)
}

// handleStreamingChat 处理流式对话请求
func (mh *MessageHandler) handleStreamingChat(openID, userID, receiveID, receiveIDType, question string) error {
	mh.logger.Printf("[DEBUG] handleStreamingChat called with: openID=%s userID=%s receiveID=%s receiveIDType=%s question=%s", openID, userID, receiveID, receiveIDType, question)
	_ = os.WriteFile(utils.GetTempFilePath("feishu-last-streaming.txt"), []byte(fmt.Sprintf("receive_id_type=%s receive_id=%s", receiveIDType, receiveID)), 0644)

	// 获取 tenant_access_token
	token, err := mh.feishuClient.GetTenantAccessToken()
	if err != nil {
		mh.logger.Printf("Failed to get tenant access token: %v", err)
		return mh.sendTextMessage(receiveID, receiveIDType, "❌ 获取访问令牌失败")
	}

	// 验证 receive_id 不为空
	if receiveID == "" {
		mh.logger.Printf("ERROR: receiveID is empty! receiveIDType=%s", receiveIDType)
		return mh.sendTextMessage(receiveID, receiveIDType, "❌ 无法发送卡片：缺少有效的会话ID")
	}

	// 创建 Claude 流式文本处理器（不使用 CardKit，节省 API 调用）
	streamingTextHandler := claude.NewStreamingTextHandler(mh.feishuClient)
	resumeSessionID := mh.getClaudeSession(openID)

	// 处理消息（流式分段发送，同步 CLI 输出节奏）
	ctx := context.Background()
	if err := streamingTextHandler.HandleMessage(ctx, token, receiveID, receiveIDType, question, resumeSessionID, ""); err != nil {
		mh.logger.Printf("Failed to handle streaming text chat: %v", err)
		return mh.sendTextMessage(receiveID, receiveIDType, "❌ 对话处理失败: "+err.Error())
	}
	if sessionID := streamingTextHandler.SessionID(); sessionID != "" {
		mh.setClaudeSession(openID, sessionID)
	}

	mh.logger.Printf("Streaming text chat completed successfully for user %s", userID)
	return nil
}

// parseCommand 解析用户消息是否为特殊命令
// 返回：命令类型、参数、是否为命令
func parseCommand(content string) (cmdType string, args string, isCmd bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", "", false
	}

	// 提取第一个单词作为命令
	parts := strings.Fields(content)
	if len(parts) == 0 {
		return "", "", false
	}

	command := strings.ToLower(parts[0])
	switch command {
	case "ls", "bind", "help":
		args = strings.Join(parts[1:], " ")
		return command, args, true
	default:
		return "", "", false
	}
}

// handleLsCommand 处理 ls 命令 - 列出基础目录下的所有项目
func (mh *MessageHandler) handleLsCommand(chatID string) error {
	baseDir := getBaseDir()

	// 列出目录内容
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return mh.sendTextMessage(chatID, "chat_id",
			fmt.Sprintf("❌ 无法读取目录: %v", err))
	}

	// 过滤目录并编号
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			dirs = append(dirs, entry.Name())
		}
	}

	// 读取当前绑定的项目路径
	currentBinding := ""
	cfg, err := config.Load()
	if err == nil {
		if path := cfg.GetProjectPath(chatID); path != "" {
			currentBinding = path
		}
	}

	// 构建回复消息
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("📂 基础目录: %s\n\n", baseDir))
	builder.WriteString("可绑定项目目录：\n")
	for i, dir := range dirs {
		builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, dir))
	}
	builder.WriteString(fmt.Sprintf("\n共 %d 个目录\n", len(dirs)))
	builder.WriteString("使用命令: bind <序号>")

	// 显示当前绑定
	if currentBinding != "" {
		builder.WriteString(fmt.Sprintf("\n\n✅ 当前绑定: %s", currentBinding))
	}

	return mh.sendTextMessage(chatID, "chat_id", builder.String())
}

// handleBindCommand 处理 bind 命令 - 绑定群聊到指定项目路径
func (mh *MessageHandler) handleBindCommand(chatID, args string) error {
	// 解析序号
	args = strings.TrimSpace(args)
	if args == "" {
		return mh.sendTextMessage(chatID, "chat_id",
			"❌ 请提供项目序号\n使用命令: bind <序号>")
	}

	// 尝试解析为数字
	index, err := strconv.Atoi(args)
	if err != nil || index < 1 {
		return mh.sendTextMessage(chatID, "chat_id",
			"❌ 无效的序号，请输入数字")
	}

	// 读取项目列表
	baseDir := getBaseDir()
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return mh.sendTextMessage(chatID, "chat_id",
			fmt.Sprintf("❌ 无法读取目录: %v", err))
	}

	// 过滤并查找指定目录
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			dirs = append(dirs, entry.Name())
		}
	}

	if index > len(dirs) {
		return mh.sendTextMessage(chatID, "chat_id",
			fmt.Sprintf("❌ 序号超出范围，最大序号: %d", len(dirs)))
	}

	// 绑定路径
	selectedDir := dirs[index-1]
	projectPath := baseDir + selectedDir

	// 保存到配置文件
	cfg, err := config.Load()
	if err != nil {
		return mh.sendTextMessage(chatID, "chat_id",
			fmt.Sprintf("❌ 加载配置失败: %v", err))
	}
	if err := cfg.SetProjectPath(chatID, projectPath); err != nil {
		return mh.sendTextMessage(chatID, "chat_id",
			fmt.Sprintf("❌ 保存配置失败: %v", err))
	}
	if err := cfg.Save(); err != nil {
		return mh.sendTextMessage(chatID, "chat_id",
			fmt.Sprintf("❌ 保存配置文件失败: %v", err))
	}

	return mh.sendTextMessage(chatID, "chat_id",
		fmt.Sprintf("✅ 已绑定项目路径: %s\n（配置已保存）", projectPath))
}

// handleHelpCommand 处理 help 命令 - 显示帮助信息
func (mh *MessageHandler) handleHelpCommand(chatID string) error {
	// 读取当前绑定的项目路径
	currentBinding := ""
	cfg, err := config.Load()
	if err == nil {
		if path := cfg.GetProjectPath(chatID); path != "" {
			currentBinding = path
		}
	}

	// 构建帮助信息
	var builder strings.Builder
	builder.WriteString(`🤖 飞书 Claude CLI 机器人命令说明

特殊命令：
• ls - 列出可绑定的项目目录
• bind <序号> - 绑定群聊到指定项目路径
• help - 显示此帮助信息

使用示例：
@机器人 ls
@机器人 bind 18
@机器人 help

注意：
- 特殊命令仅在群聊中有效
- 绑定后配置会持久化保存
- 其他消息将转发给 Claude 处理`)

	// 显示当前绑定
	if currentBinding != "" {
		builder.WriteString(fmt.Sprintf("\n\n✅ 当前绑定: %s", currentBinding))
	} else {
		builder.WriteString("\n\n⚠️ 当前未绑定项目路径")
	}

	return mh.sendTextMessage(chatID, "chat_id", builder.String())
}

// getBaseDir 获取基础目录配置
func getBaseDir() string {
	// 优先从环境变量读取
	if dir := os.Getenv("BASE_DIR"); dir != "" {
		// 确保路径末尾有斜杠
		if !strings.HasSuffix(dir, "/") {
			dir = dir + "/"
		}
		return dir
	}

	// 从配置文件读取
	cfg, err := config.Load()
	if err == nil && cfg.BaseDir != "" {
		baseDir := cfg.BaseDir
		if !strings.HasSuffix(baseDir, "/") {
			baseDir = baseDir + "/"
		}
		return baseDir
	}

	// 默认值
	return "/Users/wen/Desktop/code/"
}
