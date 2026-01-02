package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/larksuiteoapi/sdk-go/aifunction/prompt"
	larkcard "github.com/larksuiteoapi/sdk-go/api/card/v1"
	larkim "github.com/larksuiteoapi/sdk-go/api/im/v1"
	"github.com/larksuiteoapi/sdk-go/core"
	"github.com/larksuiteoapi/sdk-go/core/errors"
)

// CardConfig 飞书卡片配置
type CardConfig struct {
	Schema string `json:"schema"`
	Config struct {
		WideScreenMode bool `json:"wide_screen_mode"`
		StreamingMode  bool `json:"streaming_mode"`
		UpdateMulti    bool `json:"update_multi"`
	} `json:"config"`
	Elements []CardElement `json:"elements"`
}

// CardElement 卡片元素
type CardElement struct {
	Tag       string `json:"tag"`
	ElementID string `json:"element_id"`
	UUID      string `json:"uuid,omitempty"`
	Content   string `json:"content"`
}

// StreamUpdateRequest 流式更新请求
type StreamUpdateRequest struct {
	UUID     string `json:"uuid"`
	Content  string `json:"content"`
	Sequence int64  `json:"sequence"`
}

func main() {
	fmt.Println("=== 飞书 CardKit 流式更新 API PoC 测试 ===")
	fmt.Println()

	// 从环境变量读取配置
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")
	chatID := os.Getenv("FEISHU_TEST_CHAT_ID")

	if appID == "" || appSecret == "" {
		fmt.Println("[ERROR] 缺少飞书配置！")
		fmt.Println("请设置以下环境变量：")
		fmt.Println("  export FEISHU_APP_ID=cli_xxxxxxxx")
		fmt.Println("  export FEISHU_APP_SECRET=your_app_secret_here")
		fmt.Println("  export FEISHU_TEST_CHAT_ID=oc_xxxxx (可选，用于测试群聊)")
		fmt.Println()
		fmt.Println("或者创建 .env 文件并填入配置。")
		os.Exit(1)
	}

	fmt.Printf("[INFO] App ID: %s\n", appID)
	if chatID != "" {
		fmt.Printf("[INFO] Test Chat ID: %s\n", chatID)
	}
	fmt.Println()

	// 创建飞书客户端
	client := core.NewClient(
		core.WithAppCredential(appID, appSecret),
		core.WithTimeout(10*time.Second),
	)

	fmt.Println("[INFO] 飞书客户端创建成功")
	fmt.Println()

	// 如果没有提供 chatID，只测试卡片创建
	if chatID == "" {
		fmt.Println("[INFO] 未指定 TEST_CHAT_ID，跳过 API 测试")
		fmt.Println("[INFO] 若要测试完整流程，请设置 FEISHU_TEST_CHAT_ID")
		printCardConfigExample()
		return
	}

	// 测试 1: 创建流式卡片
	fmt.Println("=== 测试 1: 创建流式卡片 ===")
	cardID, elementID, uuid, err := createStreamCard(client, chatID)
	if err != nil {
		fmt.Printf("[ERROR] 创建卡片失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[SUCCESS] 卡片创建成功\n")
	fmt.Printf("  Card ID: %s\n", cardID)
	fmt.Printf("  Element ID: %s\n", elementID)
	fmt.Printf("  UUID: %s\n", uuid)
	fmt.Println()

	// 测试 2: 流式更新卡片内容
	fmt.Println("=== 测试 2: 流式更新卡片内容 ===")

	// 模拟流式输出
	messages := []string{
		"Hello",
		" there",
		"!",
		" This",
		" is",
		" a",
		" streaming",
		" test",
		".",
	}

	for i, msg := range messages {
		sequence := int64(i + 1)
		content := joinMessages(messages[:i+1])

		fmt.Printf("[UPDATE %d] Sending: \"%s\" (total: %d chars)\n", sequence, msg, len(content))

		err := updateCardContent(client, cardID, elementID, uuid, content, sequence)
		if err != nil {
			fmt.Printf("[ERROR] 更新失败 (seq=%d): %v\n", sequence, err)
			os.Exit(1)
		}

		fmt.Printf("[SUCCESS] 更新成功\n")

		// 延迟以模拟真实输出
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println()
	fmt.Println("[SUCCESS] 所有测试通过！")
	fmt.Println()
	fmt.Println("[INFO] 请检查飞书群聊中的卡片，应该能看到打字机效果。")
}

// createStreamCard 创建流式卡片
func createStreamCard(client *core.Client, chatID string) (cardID, elementID, uuid string, err error) {
	// 生成 UUID
	uuid = generateUUID()
	elementID = "reply_content"

	// 创建卡片配置
	card := &CardConfig{
		Schema: "2.0",
		Config: struct {
			WideScreenMode bool `json:"wide_screen_mode"`
			StreamingMode  bool `json:"streaming_mode"`
			UpdateMulti    bool `json:"update_multi"`
		}{
			WideScreenMode: true,
			StreamingMode:  true,
			UpdateMulti:    true,
		},
		Elements: []CardElement{
			{
				Tag:       "markdown",
				ElementID: elementID,
				UUID:      uuid,
				Content:   "思考中...",
			},
		},
	}

	cardJSON, _ := json.Marshal(card)

	// 创建消息
	req := larkim.CreateMessageReq{
		ReceiveIdType: larkim.ReceiveIdTypeChat,
		ReceiveId:     chatID,
		MsgType:       larkim.MsgTypeInteractive,
		Content:       string(cardJSON),
	}

	resp, _, err := larkim.NewMessageService(client).Create(ctx, req)
	if err != nil {
		return "", "", "", fmt.Errorf("创建消息失败: %w", err)
	}

	if !resp.Success() {
		return "", "", "", fmt.Errorf("飞书API错误: %s", formatError(resp.CodeError))
	}

	return resp.Data.MessageId, elementID, uuid, nil
}

// updateCardContent 流式更新卡片内容
func updateCardContent(client *core.Client, cardID, elementID, uuid, content string, sequence int64) error {
	reqBody := StreamUpdateRequest{
		UUID:     uuid,
		Content:  content,
		Sequence: sequence,
	}

	bodyJSON, _ := json.Marshal(reqBody)

	// 构建请求 URL
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/cardkit/v1/cards/%s/elements/%s/content",
		cardID, elementID)

	// 创建 HTTP 请求
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// 使用飞书 SDK 的 HTTP 客户端
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("API 错误 (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// joinMessages 拼接消息
func joinMessages(messages []string) string {
	result := ""
	for _, msg := range messages {
		result += msg
	}
	return result
}

// generateUUID 生成简单的 UUID（简化版）
func generateUUID() string {
	return fmt.Sprintf("%d-%d-%d-%d",
		time.Now().UnixNano(),
		time.Now().UnixNano()%10000,
		time.Now().UnixNano()%10000,
		time.Now().UnixNano()%10000,
	)
}

// formatError 格式化错误信息
func formatError(err *errors.CodeError) string {
	if err == nil {
		return "unknown error"
	}
	return fmt.Sprintf("code=%d, msg=%s", err.Code, err.Msg)
}

// printCardConfigExample 打印卡片配置示例
func printCardConfigExample() {
	fmt.Println()
	fmt.Println("=== CardKit 2.0 流式卡片配置示例 ===")
	fmt.Println()

	card := &CardConfig{
		Schema: "2.0",
		Config: struct {
			WideScreenMode bool `json:"wide_screen_mode"`
			StreamingMode  bool `json:"streaming_mode"`
			UpdateMulti    bool `json:"update_multi"`
		}{
			WideScreenMode: true,
			StreamingMode:  true,
			UpdateMulti:    true,
		},
		Elements: []CardElement{
			{
				Tag:       "markdown",
				ElementID: "reply_content",
				UUID:      "your-uuid-here",
				Content:   "初始内容",
			},
		},
	}

	json, _ := json.MarshalIndent(card, "", "  ")
	fmt.Println(string(json))
	fmt.Println()

	fmt.Println("流式更新 API 调用示例：")
	fmt.Println("  PUT /open-apis/cardkit/v1/cards/{card_id}/elements/{element_id}/content")
	fmt.Println()
	fmt.Println("请求体：")
	fmt.Println(`{
  "uuid": "your-uuid-here",
  "content": "更新后的内容（全量文本）",
  "sequence": 1
}`)
	fmt.Println()
}

var ctx = core_ctx(ctx.Background())

// 简化的 context 包装
type core_ctx struct {
	context.Context
}

func ctx(c context.Context) core_ctx {
	return core_ctx{c}
}
