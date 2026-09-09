package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Provider 大模型配置
type Provider struct {
	BaseURL string
	APIKey  string
	Model   string
}

// Message 对话消息（OpenAI 兼容格式）
type Message struct {
	Role       string     `json:"role"`
	Content    *string    `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

func StrPtr(s string) *string { return &s }

type ToolCall struct {
	ID       string     `json:"id"`
	Type     string     `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool OpenAI 工具定义
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
	Stream   bool      `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Client OpenAI 兼容客户端
type Client struct {
	cfg    Provider
	http   *http.Client
	tools  []Tool
}

func NewClient(cfg Provider) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 120 * time.Second},
	}
}

// RegisterTools 注册 AI 可用工具
func (c *Client) RegisterTools(tools ...Tool) {
	c.tools = append(c.tools, tools...)
}

// Chat 发起对话，返回模型回复。当模型要求调用工具时，executor 会被调用执行并自动继续多轮，直到模型给出最终文本。
func (c *Client) Chat(ctx context.Context, history []Message, execTool func(name string, args string) (string, error)) (string, []Message, error) {
	msgs := append([]Message{}, history...)
	maxRounds := 6
	for round := 0; round < maxRounds; round++ {
		req := chatRequest{Model: c.cfg.Model, Messages: msgs, Stream: false}
		if len(c.tools) > 0 {
			req.Tools = c.tools
		}
		body, _ := json.Marshal(req)
		resp, err := c.doRequest(ctx, body)
		if err != nil {
			return "", msgs, err
		}
		if resp.Error != nil {
			return "", msgs, fmt.Errorf("llm error: %s", resp.Error.Message)
		}
		if len(resp.Choices) == 0 {
			return "", msgs, fmt.Errorf("llm empty response")
		}
		assistantMsg := resp.Choices[0].Message
		msgs = append(msgs, assistantMsg)

		// 模型是否请求调用工具
		if len(assistantMsg.ToolCalls) == 0 {
			content := ""
			if assistantMsg.Content != nil {
				content = *assistantMsg.Content
			}
			return content, msgs, nil
		}

		// 依次执行工具调用
		for _, tc := range assistantMsg.ToolCalls {
			if tc.Function.Name == "" {
				continue
			}
			result, err := execTool(tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				result = fmt.Sprintf("工具执行错误: %v", err)
			}
			msgs = append(msgs, Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    StrPtr(result),
			})
		}
	}
	return "", msgs, fmt.Errorf("工具调用轮次过多，未得到最终回答")
}

// Ping 发送一条最小请求验证配置连通性
func (c *Client) Ping(ctx context.Context) (string, []Message, error) {
	msgs := []Message{{Role: "system", Content: StrPtr("ping")}}
	req := chatRequest{Model: c.cfg.Model, Messages: msgs, Stream: false}
	body, _ := json.Marshal(req)
	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return "", msgs, err
	}
	if resp.Error != nil {
		return "", msgs, fmt.Errorf("llm error: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", msgs, fmt.Errorf("llm empty response")
	}
	content := ""
	if resp.Choices[0].Message.Content != nil {
		content = *resp.Choices[0].Message.Content
	}
	return content, msgs, nil
}

// doRequest 发送请求
func (c *Client) doRequest(ctx context.Context, body []byte) (*chatResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求大模型失败: %v", err)
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("大模型返回异常状态 %d: %s", resp.StatusCode, string(respBytes))
	}
	var out chatResponse
	if err := json.Unmarshal(respBytes, &out); err != nil {
		return nil, fmt.Errorf("解析大模型响应失败: %v", err)
	}
	return &out, nil
}
