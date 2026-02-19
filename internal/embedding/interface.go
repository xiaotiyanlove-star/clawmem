package embedding

import "context"

// Embedder 定义 Embedding 接口
type Embedder interface {
	// Name 返回 Embedder 的名称（用于日志和区分）
	Name() string
	// Embed 将文本转换为向量
	Embed(ctx context.Context, text string) ([]float32, error)
}
