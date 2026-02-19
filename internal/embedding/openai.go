package embedding

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

// OpenAIEmbedder 实现 OpenAI 兼容的 Embedder (Tier 2)
type OpenAIEmbedder struct {
	BaseURL    string
	APIKey     string
	Model      string
	httpClient *http.Client
}

func NewOpenAIEmbedder(cfg *config.Config) *OpenAIEmbedder {
	return &OpenAIEmbedder{
		BaseURL:    cfg.EmbedAPIBase,
		APIKey:     cfg.EmbedAPIKey,
		Model:      cfg.EmbedModel,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (e *OpenAIEmbedder) Name() string {
	return "OpenAI/SiliconFlow (Tier 2)"
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model": e.Model,
		"input": text,
	}

	body, _ := json.Marshal(reqBody)
	url := e.BaseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.APIKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API Error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}

	return result.Data[0].Embedding, nil
}

func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := map[string]interface{}{
		"model": e.Model,
		"input": texts, // OpenAI 支持字符串数组
	}

	body, _ := json.Marshal(reqBody)
	url := e.BaseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.APIKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API Error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}

	// 确保顺序正确 (OpenAI 通常按顺序返回，但根据 Index 排序更安全)
	// 这里简单实现，假设 data 长度等于 texts
	if len(result.Data) != len(texts) {
		return nil, fmt.Errorf("API returned %d embeddings, expected %d", len(result.Data), len(texts))
	}

	output := make([][]float32, len(texts))
	for _, item := range result.Data {
		if item.Index >= 0 && item.Index < len(output) {
			output[item.Index] = item.Embedding
		}
	}

	return output, nil
}
