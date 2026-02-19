package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xiaotiyanlove-star/clawmem/config"
)

// Client 封装 OpenAI 兼容的 API 客户端
type Client struct {
	embedBase  string
	embedKey   string
	embedModel string
	llmBase    string
	llmKey     string
	llmModel   string
	httpClient *http.Client
}

// NewClient 创建新的 LLM 客户端
func NewClient(cfg *config.Config) *Client {
	return &Client{
		embedBase:  cfg.EmbedAPIBase,
		embedKey:   cfg.EmbedAPIKey,
		embedModel: cfg.EmbedModel,
		llmBase:    cfg.LLMAPIBase,
		llmKey:     cfg.LLMAPIKey,
		llmModel:   cfg.LLMModel,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// ========================
// Embedding 相关
// ========================

type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed 将文本转换为向量
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := embeddingRequest{
		Model: c.embedModel,
		Input: text,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	url := c.embedBase + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.embedKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.embedKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Embedding API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Embedding API 返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 Embedding 响应失败: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("Embedding API 返回空结果")
	}

	return result.Data[0].Embedding, nil
}

// ========================
// LLM Chat 相关（用于记忆摘要提取）
// ========================

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Chat 通用聊天接口，支持自定义 system prompt 和 user message
// 可选传入自定义的 base/key/model，为空则使用默认配置
func (c *Client) Chat(ctx context.Context, systemPrompt, userMessage string, overrideBase, overrideKey, overrideModel string) (string, error) {
	base := c.llmBase
	key := c.llmKey
	model := c.llmModel
	if overrideBase != "" {
		base = overrideBase
	}
	if overrideKey != "" {
		key = overrideKey
	}
	if overrideModel != "" {
		model = overrideModel
	}

	reqBody := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	url := base + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 LLM API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API 返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析 LLM 响应失败: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("LLM API 返回空结果")
	}

	return result.Choices[0].Message.Content, nil
}

// Summarize 使用 LLM 从对话内容中提取关键记忆摘要
func (c *Client) Summarize(ctx context.Context, content string) (string, error) {
	systemPrompt := `你是一个记忆提取助手。请从以下对话内容中提取关键信息，生成简洁的记忆摘要。
规则：
1. 仅提取事实性信息（用户偏好、重要事件、决策等）
2. 忽略寒暄和无意义对话
3. 用简短的陈述句列出，每条一行
4. 如果没有值得记忆的内容，返回空字符串`

	reqBody := chatRequest{
		Model: c.llmModel,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: content},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	url := c.llmBase + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.llmKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.llmKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 LLM API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API 返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析 LLM 响应失败: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("LLM API 返回空结果")
	}

	return result.Choices[0].Message.Content, nil
}
