package claude

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"feishu-bot/internal/utils"
)

// CardKitUpdater CardKit 流式更新器
type CardKitUpdater struct {
	token       string
	cardID      string
	elementID   string
	uuid        string
	currentSeq  int
	mu          sync.Mutex
	client      *http.Client
	rateLimiter *time.Ticker
	lastUpdate  time.Time
}

// NewCardKitUpdater 创建 CardKit 更新器
func NewCardKitUpdater(token, cardID, elementID, uuid string) *CardKitUpdater {
	// 使用统一超时配置
	timeoutConfig := utils.DefaultTimeoutConfig()

	return &CardKitUpdater{
		token:       token,
		cardID:      cardID,
		elementID:   elementID,
		uuid:        uuid,
		client:      &http.Client{Timeout: timeoutConfig.HTTPClientTimeout},
		rateLimiter: time.NewTicker(timeoutConfig.CardKitRateLimitInterval),
		currentSeq:  0,
	}
}

// SetCurrentSeq 设置当前 sequence (用于同步服务端返回的初始值)
func (u *CardKitUpdater) SetCurrentSeq(seq int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.currentSeq = seq
	log.Printf("[CardKitUpdater] Set currentSeq to: %d", seq)
}

// UpdateContent 更新卡片内容
func (u *CardKitUpdater) UpdateContent(text string, sequence int) error {
	// 先获取锁，确保串行处理
	u.mu.Lock()
	defer u.mu.Unlock()

	log.Printf("[CardKitUpdater] Updating content: seq=%d, len=%d", sequence, len(text))

	if sequence <= u.currentSeq {
		log.Printf("[CardKitUpdater] Skipping stale sequence: seq=%d current=%d", sequence, u.currentSeq)
		return nil
	}

	// 等待限流器（在锁内等待，保证串行）
	<-u.rateLimiter.C

	// 构建 API 请求
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/cardkit/v1/cards/%s/elements/%s/content",
		u.cardID, u.elementID)

	log.Printf("[CardKitUpdater] API URL: %s", url)

	updateUUID, err := generateUUID()
	if err != nil {
		return fmt.Errorf("failed to generate update uuid: %w", err)
	}
	log.Printf("[CardKitUpdater] Update UUID: %s element_id=%s card_id=%s", updateUUID, u.elementID, u.cardID)

	reqBody := map[string]interface{}{
		"uuid":     updateUUID,
		"content":  text,
		"sequence": sequence,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	log.Printf("[CardKitUpdater] Request body: %s", string(jsonData))

	req, err := http.NewRequest("PUT", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+u.token)
	req.Header.Set("Content-Type", "application/json")

	log.Printf("[CardKitUpdater] Sending request: seq=%d", sequence)

	// 发送请求
	resp, err := u.client.Do(req)
	if err != nil {
		log.Printf("[CardKitUpdater] Request failed: %v", err)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[CardKitUpdater] Response status: %d", resp.StatusCode)

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// 解析响应
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// 检查错误
	code, ok := result["code"].(float64)
	if !ok || int(code) != 0 {
		log.Printf("[CardKitUpdater] API error response: %s", string(body))
		// 尝试解析错误详情
		if data, ok := result["data"].(map[string]interface{}); ok {
			if expectedSeq, ok := data["expected_sequence_number"].(float64); ok {
				log.Printf("[CardKitUpdater] Server expects sequence_number: %d", int(expectedSeq))
			}
		}
		return fmt.Errorf("API error: %s", string(body))
	}

	// 从响应中获取服务端返回的 sequence_number
	data, ok := result["data"].(map[string]interface{})
	if ok {
		if serverSeq, ok := data["sequence_number"].(float64); ok {
			log.Printf("[CardKitUpdater] Server returned sequence_number: %d", int(serverSeq))
			u.currentSeq = int(serverSeq)
		} else {
			// 如果服务端没有返回 sequence_number，使用本地序号
			log.Printf("[CardKitUpdater] Using local sequence: %d", sequence)
			u.currentSeq = sequence
		}
	} else {
		// 兼容旧格式，使用本地序号
		u.currentSeq = sequence
	}

	log.Printf("[CardKitUpdater] Update successful: seq=%d, currentSeq=%d", sequence, u.currentSeq)
	u.lastUpdate = time.Now()
	return nil
}

// CreateStreamingCard 创建流式更新卡片
func CreateStreamingCard(token, receiveID, receiveIDType, title, initialContent string) (string, string, string, int, error) {
	uuid, err := generateUUID()
	if err != nil {
		return "", "", "", 0, fmt.Errorf("failed to generate uuid: %w", err)
	}
	log.Printf("[CardKitUpdater] Create card element: element_id=%s uuid=%s initial_len=%d", "content_markdown", uuid, len(initialContent))

	// Step 1: 创建卡片实体
	cardJSON := map[string]interface{}{
		"schema": "2.0",
		"header": map[string]interface{}{
			"title": map[string]interface{}{
				"content": title,
				"tag":     "plain_text",
			},
		},
		"config": map[string]interface{}{
			"streaming_mode": true,
			"update_multi":   true,
			"summary": map[string]interface{}{
				"content": "",
			},
			"streaming_config": map[string]interface{}{
				"print_frequency_ms": map[string]interface{}{
					"default": 70,
					"android": 70,
					"ios":     70,
					"pc":      70,
				},
				"print_step": map[string]interface{}{
					"default": 1,
					"android": 1,
					"ios":     1,
					"pc":      1,
				},
				"print_strategy": "fast",
			},
		},
		"body": map[string]interface{}{
			"elements": []map[string]interface{}{
				{
					"tag":        "markdown",
					"content":    initialContent,
					"element_id": "content_markdown",
					"uuid":       uuid,
				},
			},
		},
	}

	cardJSONStr, err := json.Marshal(cardJSON)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("failed to marshal card JSON: %w", err)
	}
	log.Printf("[CardKitUpdater] Card JSON size: %d bytes", len(cardJSONStr))

	// 转义为字符串
	cardJSONEscaped, _ := json.Marshal(string(cardJSONStr))

	createReq := map[string]interface{}{
		"type": "card_json",
		"data": json.RawMessage(cardJSONEscaped),
	}

	createReqJSON, err := json.Marshal(createReq)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("failed to marshal create request: %w", err)
	}

	// 调用创建卡片实体 API
	req, err := http.NewRequest("POST",
		"https://open.feishu.cn/open-apis/cardkit/v1/cards",
		bytes.NewReader(createReqJSON))
	if err != nil {
		return "", "", "", 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	timeoutConfig := utils.DefaultTimeoutConfig()
	client := &http.Client{Timeout: timeoutConfig.HTTPClientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("failed to send create request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("failed to read create response: %w", err)
	}

	var createResult map[string]interface{}
	if err := json.Unmarshal(body, &createResult); err != nil {
		return "", "", "", 0, fmt.Errorf("failed to parse create response: %w", err)
	}

	// 检查错误
	code, ok := createResult["code"].(float64)
	if !ok || int(code) != 0 {
		return "", "", "", 0, fmt.Errorf("create card error: %s", string(body))
	}

	// 获取 card_id
	data, ok := createResult["data"].(map[string]interface{})
	if !ok {
		return "", "", "", 0, fmt.Errorf("invalid response data format")
	}

	cardID, ok := data["card_id"].(string)
	if !ok {
		return "", "", "", 0, fmt.Errorf("card_id not found in response")
	}

	log.Printf("[DEBUG] Card created successfully: card_id=%s", cardID)
	log.Printf("[DEBUG] Full create response: %s", string(body))

	// 获取初始 sequence_number,如果服务端没有返回,则使用默认值 0
	initialSeq := 0
	if initSeq, ok := data["sequence_number"].(float64); ok {
		initialSeq = int(initSeq)
		log.Printf("[DEBUG] Initial sequence_number from create: %d", initialSeq)
	} else {
		log.Printf("[DEBUG] No initial sequence_number from server, using default: 0")
	}

	// Step 2: 发送卡片到群聊
	log.Printf("[DEBUG] Sending card: receive_id=%s receive_id_type=%s card_id=%s", receiveID, receiveIDType, cardID)
	sendReq := map[string]interface{}{
		"receive_id": receiveID,
		"msg_type":   "interactive",
		"content":    fmt.Sprintf("{\"type\":\"card\",\"data\":{\"card_id\":\"%s\"}}", cardID),
	}

	sendReqJSON, err := json.Marshal(sendReq)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("failed to marshal send request: %w", err)
	}

	sendURL := "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=" + receiveIDType
	log.Printf("[DEBUG] API URL: %s", sendURL)
	log.Printf("[DEBUG] Request body: %s", string(sendReqJSON))
	req2, err := http.NewRequest("POST", sendURL, bytes.NewReader(sendReqJSON))
	if err != nil {
		return "", "", "", 0, fmt.Errorf("failed to create send request: %w", err)
	}

	req2.Header.Set("Authorization", "Bearer "+token)
	req2.Header.Set("Content-Type", "application/json")
	log.Printf("[DEBUG] Headers: Authorization=Bearer %s", token[:20]+"...")

	resp2, err := client.Do(req2)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("failed to send send request: %w", err)
	}
	defer resp2.Body.Close()

	body2, err := io.ReadAll(resp2.Body)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("failed to read send response: %w", err)
	}

	var sendResult map[string]interface{}
	if err := json.Unmarshal(body2, &sendResult); err != nil {
		return "", "", "", 0, fmt.Errorf("failed to parse send response: %w", err)
	}

	// 检查错误
	code2, ok := sendResult["code"].(float64)
	if !ok || int(code2) != 0 {
		return "", "", "", 0, fmt.Errorf("send card error: %s", string(body2))
	}

	log.Printf("Card created and sent: card_id=%s, initial_seq=%d", cardID, initialSeq)

	return cardID, "content_markdown", uuid, initialSeq, nil
}

// Stop 停止更新器
func (u *CardKitUpdater) Stop() {
	u.rateLimiter.Stop()
}

func (u *CardKitUpdater) FinalizeContent(text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	u.mu.Lock()
	nextSeq := u.currentSeq + 1
	u.mu.Unlock()

	return u.UpdateContent(text, nextSeq)
}

func generateUUID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80

	hexBytes := make([]byte, 32)
	hex.Encode(hexBytes, buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hexBytes[0:8],
		hexBytes[8:12],
		hexBytes[12:16],
		hexBytes[16:20],
		hexBytes[20:32],
	), nil
}
