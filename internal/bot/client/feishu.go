package client

import (
	"context"
	"encoding/json"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// FeishuClient 飞书客户端封装
type FeishuClient struct {
	client *lark.Client
	config FeishuConfig
}

// FeishuConfig 飞书配置
type FeishuConfig struct {
	AppID     string
	AppSecret string
	CardTemplates CardTemplates
}

// CardTemplates 卡片模板配置
type CardTemplates struct {
	TaskCompleted string // 任务完成卡片模板ID
	TaskWaiting   string // 等待输入卡片模板ID
	CommandResult string // 命令结果卡片模板ID
	SessionList   string // 会话列表卡片模板ID
}

// NewFeishuClient 创建飞书客户端
func NewFeishuClient(config FeishuConfig) *FeishuClient {
	client := lark.NewClient(config.AppID, config.AppSecret)
	
	return &FeishuClient{
		client: client,
		config: config,
	}
}

// CardData 卡片数据
type CardData struct {
	Token       string                 `json:"token"`
	ProjectName string                 `json:"project_name"`
	Description string                 `json:"description"`
	Status      string                 `json:"status"`
	Timestamp   string                 `json:"timestamp"`
	UserID      string                 `json:"user_id"`
	OpenID      string                 `json:"open_id"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// SendTaskCompletedCard 发送任务完成卡片
func (fc *FeishuClient) SendTaskCompletedCard(openID string, cardData interface{}) error {
	var data map[string]interface{}
	
	// 尝试不同的类型转换
	switch v := cardData.(type) {
	case *CardData:
		data = map[string]interface{}{
			"token":        v.Token,
			"project_name": v.ProjectName,
			"description":  v.Description,
			"timestamp":    v.Timestamp,
			"status":       v.Status,
			"open_id":      v.OpenID,
		}
	case map[string]interface{}:
		data = v
	default:
		return fmt.Errorf("invalid card data type")
	}
	card := &callback.Card{
		Type: "template",
		Data: &callback.TemplateCard{
			TemplateID: fc.config.CardTemplates.TaskCompleted,
			TemplateVariable: map[string]interface{}{
				"token":        data["token"],
				"project_name": data["project_name"],
				"description":  data["description"],
				"timestamp":    data["timestamp"],
				"status":       "completed",
				"open_id":      data["open_id"],
			},
		},
	}

	return fc.sendCard(openID, card)
}

// SendTaskWaitingCard 发送等待输入卡片
func (fc *FeishuClient) SendTaskWaitingCard(openID string, cardData interface{}) error {
	var data map[string]interface{}
	
	// 尝试不同的类型转换
	switch v := cardData.(type) {
	case *CardData:
		data = map[string]interface{}{
			"token":        v.Token,
			"project_name": v.ProjectName,
			"description":  v.Description,
			"timestamp":    v.Timestamp,
			"status":       v.Status,
			"open_id":      v.OpenID,
		}
	case map[string]interface{}:
		data = v
	default:
		return fmt.Errorf("invalid card data type")
	}
	card := &callback.Card{
		Type: "template",
		Data: &callback.TemplateCard{
			TemplateID: fc.config.CardTemplates.TaskWaiting,
			TemplateVariable: map[string]interface{}{
				"token":        data["token"],
				"project_name": data["project_name"],
				"description":  data["description"],
				"timestamp":    data["timestamp"],
				"status":       "waiting",
				"open_id":      data["open_id"],
			},
		},
	}

	return fc.sendCard(openID, card)
}

// SendCommandResultCard 发送命令执行结果卡片
func (fc *FeishuClient) SendCommandResultCard(openID string, token, command, result string, success bool) error {
	status := "success"
	if !success {
		status = "failed"
	}

	card := &callback.Card{
		Type: "template",
		Data: &callback.TemplateCard{
			TemplateID: fc.config.CardTemplates.CommandResult,
			TemplateVariable: map[string]interface{}{
				"token":   token,
				"command": command,
				"result":  result,
				"status":  status,
				"open_id": openID,
			},
		},
	}

	return fc.sendCard(openID, card)
}

// SendTextMessage 发送文本消息
func (fc *FeishuClient) SendTextMessage(openID, text string) error {
	// 按照飞书文本消息格式要求，内容必须是 JSON 字符串
	textContent := map[string]string{"text": text}
	content, err := json.Marshal(textContent)
	if err != nil {
		return fmt.Errorf("failed to marshal text content: %w", err)
	}
	
	resp, err := fc.client.Im.Message.Create(context.Background(), larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("open_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType("text").
			ReceiveId(openID).
			Content(string(content)).
			Build()).
		Build())

	if err != nil {
		return err
	}

	if !resp.Success() {
		return &FeishuError{
			Code:      resp.Code,
			Message:   resp.Msg,
			RequestID: resp.RequestId(),
		}
	}

	return nil
}

// SendInteractiveMessage 发送交互式消息
func (fc *FeishuClient) SendInteractiveMessage(openID string, card *callback.Card) error {
	return fc.sendCard(openID, card)
}

// sendCard 发送卡片的通用方法
func (fc *FeishuClient) sendCard(openID string, card *callback.Card) error {
	content, err := json.Marshal(card)
	if err != nil {
		return err
	}

	resp, err := fc.client.Im.Message.Create(context.Background(), larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("open_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType("interactive").
			ReceiveId(openID).
			Content(string(content)).
			Build()).
		Build())

	if err != nil {
		return err
	}

	if !resp.Success() {
		return &FeishuError{
			Code:      resp.Code,
			Message:   resp.Msg,
			RequestID: resp.RequestId(),
		}
	}

	return nil
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