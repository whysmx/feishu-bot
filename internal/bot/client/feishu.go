package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// FeishuClient 飞书客户端封装
type FeishuClient struct {
	client            *lark.Client
	httpClient        *http.Client
	appID             string
	appSecret         string
	tenantAccessToken string
	tokenExpireTime   time.Time
	tokenMutex        sync.RWMutex
}

// FeishuConfig 飞书配置
type FeishuConfig struct {
	AppID     string
	AppSecret string
}

// NewFeishuClient 创建飞书客户端
func NewFeishuClient(config FeishuConfig) *FeishuClient {
	client := lark.NewClient(config.AppID, config.AppSecret)

	return &FeishuClient{
		client:          client,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		appID:           config.AppID,
		appSecret:       config.AppSecret,
		tokenExpireTime: time.Now(), // 初始化为过去时间，强制首次获取 token
	}
}

// GetClient 获取原始客户端（用于高级操作）
func (fc *FeishuClient) GetClient() *lark.Client {
	return fc.client
}

// FeishuError 飞书API错误
type FeishuError struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func (e *FeishuError) Error() string {
	return e.Message
}

// GetTenantAccessToken 获取 tenant_access_token（带缓存）
func (fc *FeishuClient) GetTenantAccessToken() (string, error) {
	// 先尝试读锁检查缓存
	fc.tokenMutex.RLock()
	if fc.tenantAccessToken != "" && time.Now().Before(fc.tokenExpireTime) {
		log.Printf("[FeishuClient] tenant token cache hit: expire_at=%s now=%s", fc.tokenExpireTime.Format(time.RFC3339), time.Now().Format(time.RFC3339))
		token := fc.tenantAccessToken
		fc.tokenMutex.RUnlock()
		return token, nil
	}
	fc.tokenMutex.RUnlock()
	log.Printf("[FeishuClient] tenant token cache miss: token_empty=%t expire_at=%s now=%s", fc.tenantAccessToken == "", fc.tokenExpireTime.Format(time.RFC3339), time.Now().Format(time.RFC3339))

	// 缓存失效或不存在，获取新 token
	fc.tokenMutex.Lock()
	defer fc.tokenMutex.Unlock()

	// 双重检查，防止并发获取
	if fc.tenantAccessToken != "" && time.Now().Before(fc.tokenExpireTime) {
		log.Printf("[FeishuClient] tenant token cache hit (double-check): expire_at=%s now=%s", fc.tokenExpireTime.Format(time.RFC3339), time.Now().Format(time.RFC3339))
		return fc.tenantAccessToken, nil
	}

	// 调用飞书 API 获取 token
	type tokenReq struct {
		AppID     string `json:"app_id"`
		AppSecret string `json:"app_secret"`
	}

	type tokenResp struct {
		Code              int    `json:"code"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}

	reqData := tokenReq{
		AppID:     fc.appID,
		AppSecret: fc.appSecret,
	}
	log.Printf("[FeishuClient] tenant token request: app_id=%s app_secret=%s at=%s", fc.appID, fc.appSecret, time.Now().Format(time.RFC3339))

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST",
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := fc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := httpMaxBytesReader(resp.Body, 1<<20) // 限制 1MB
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var result tokenResp
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[FeishuClient] tenant token response parse failed: http=%d cost_ms=%d body=%s", resp.StatusCode, time.Since(start).Milliseconds(), string(body))
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Code != 0 {
		log.Printf("[FeishuClient] tenant token response error: http=%d cost_ms=%d code=%d body=%s", resp.StatusCode, time.Since(start).Milliseconds(), result.Code, string(body))
		return "", fmt.Errorf("API error: code=%d", result.Code)
	}
	log.Printf("[FeishuClient] tenant token response: http=%d cost_ms=%d code=%d expire_s=%d", resp.StatusCode, time.Since(start).Milliseconds(), result.Code, result.Expire)

	// 缓存 token（提前 5 分钟过期）
	fc.tenantAccessToken = result.TenantAccessToken
	fc.tokenExpireTime = time.Now().Add(time.Duration(result.Expire-300) * time.Second)

	return result.TenantAccessToken, nil
}

// httpMaxBytesReader 限制读取最大字节数
func httpMaxBytesReader(r io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(r, maxBytes)
	return io.ReadAll(limited)
}

// SendMessage 发送消息（支持多种 receive_id_type）
func (fc *FeishuClient) SendMessage(receiveID, receiveIDType, content string) error {
	// 按照飞书文本消息格式要求，内容必须是 JSON 字符串
	textContent := map[string]string{"text": content}
	jsonContent, err := json.Marshal(textContent)
	if err != nil {
		return fmt.Errorf("failed to marshal text content: %w", err)
	}

	token, err := fc.GetTenantAccessToken()
	if err != nil {
		return err
	}

	// 根据不同的 receive_id_type 构建
	resp, err := fc.client.Im.Message.Create(context.Background(), larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType("text").
			ReceiveId(receiveID).
			Content(string(jsonContent)).
			Build()).
		Build(), larkcore.WithTenantAccessToken(token))

	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	if !resp.Success() {
		return &FeishuError{
			Code:      resp.Code,
			Message:   resp.Msg,
			RequestID: resp.RequestId(),
		}
	}

	log.Printf("[FeishuClient] Message sent: receive_id=%s receive_id_type=%s len=%d msg_id=%s",
		receiveID, receiveIDType, len(content), *resp.Data.MessageId)

	return nil
}
