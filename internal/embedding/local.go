package embedding

import (
	"context"
	"sync"

	"github.com/philippgille/chromem-go"
)

// LocalEmbedder 实现本地兜底 (Tier 0)
// 使用 chromem-go 内置的 BERT 模型 (纯 Go 实现，无 CGO)
type LocalEmbedder struct {
	model func(ctx context.Context, text string) ([]float32, error)
	mu    sync.Mutex
}

func NewLocalEmbedder() *LocalEmbedder {
	// 使用 chromem-go 提供的默认 embedding function (通常是 sentence-transformers/all-MiniLM-L6-v2 的导出版)
	// 这个是纯 Go 实现，不需要 CGO，模型文件会下载到本地缓存
	return &LocalEmbedder{
		model: chromem.NewEmbeddingFuncDefault(),
	}
}

func (e *LocalEmbedder) Name() string {
	return "Local (Chromem-go BERT) (Tier 0)"
}

func (e *LocalEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.model(ctx, text)
}
