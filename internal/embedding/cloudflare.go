package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CloudflareEmbedder 实现 Cloudflare Workers AI Embedder (Tier 1)
type CloudflareEmbedder struct {
	AccountID  string
	APIToken   string
	Model      string
	httpClient *http.Client
}

func NewCloudflareEmbedder(accountID, apiToken string) *CloudflareEmbedder {
	// 默认使用 bge-base-en-v1.5 或类似模型，这里假设用户会通过配置传递模型名，
	// 如果配置为空，则使用一个通用的推荐模型。
	// 但 CF 的 URL 结构是 /accounts/{id}/run/{model_name}
	model := "@cf/baai/bge-base-en-v1.5" // 默认模型，可被配置覆盖

	return &CloudflareEmbedder{
		AccountID:  accountID,
		APIToken:   apiToken,
		Model:      model,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (e *CloudflareEmbedder) Name() string {
	return "Cloudflare Workers AI (Tier 1)"
}

func (e *CloudflareEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// CF Workers AI 接口规范: https://developers.cloudflare.com/workers-ai/models/text-embeddings/
	reqBody := map[string]interface{}{
		"text": text,
	}
	body, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s", e.AccountID, e.Model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.APIToken)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CF API Error %d: %s", resp.StatusCode, string(respBody))
	}

	// CF 返回格式可能是 {"result": {"data": [[...]]}} 或直接 {"result": [...]}，需注意
	// 这里假设标准 response
	var result struct {
		Result struct {
			Data [][]float32 `json:"data"` // 有时候是 shape [1, 768]
		} `json:"result"`
		Errors []interface{} `json:"errors"`
	}
	// 注意：CF 的 response 格式可能随模型不同而略有差异，这里针对 bge 模型适配
	// 如果直接是 vector 数组：
	// var simpleResult struct { Result []float32 `json:"result"` }

	bodyBytes, _ := io.ReadAll(resp.Body)
	// 尝试解析通用结构
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("parse error: %w body: %s", err, string(bodyBytes))
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("CF API Errors: %v", result.Errors)
	}

	if len(result.Result.Data) > 0 && len(result.Result.Data[0]) > 0 {
		return result.Result.Data[0], nil
	}

	// 尝试 fallback 解析 (有些模型直接返回 vector)
	var fallback struct {
		Result []float32 `json:"result"`
	}
	if err := json.Unmarshal(bodyBytes, &fallback); err == nil && len(fallback.Result) > 0 {
		return fallback.Result, nil
	}

	return nil, fmt.Errorf("empty embedding from CF")
}

func (e *CloudflareEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := map[string]interface{}{
		"text": texts, // Workers AI 支持字符串数组
	}
	body, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s", e.AccountID, e.Model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.APIToken)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CF API Error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Result struct {
			Data [][]float32 `json:"data"`
		} `json:"result"`
		Errors []interface{} `json:"errors"`
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("parse error: %w body: %s", err, string(bodyBytes))
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("CF API Errors: %v", result.Errors)
	}

	if len(result.Result.Data) != len(texts) {
		return nil, fmt.Errorf("CF returned %d embeddings, expected %d", len(result.Result.Data), len(texts))
	}

	return result.Result.Data, nil
}
