package embedding

import (
	"context"
	"fmt"
	"sync"

	"github.com/philippgille/chromem-go"
)

// LocalEmbedder 实现本地兜底 (Tier 0)
// 使用 chromem-go 内置的 BERT 模型 (纯 Go 实现，无 CGO)
// 支持延迟加载 (Lazy Loading)
type LocalEmbedder struct {
	model    func(ctx context.Context, text string) ([]float32, error)
	initOnce sync.Once
	initErr  error
	mu       sync.Mutex
}

func NewLocalEmbedder() *LocalEmbedder {
	return &LocalEmbedder{}
}

func (e *LocalEmbedder) Name() string {
	return "Local (Chromem-go BERT) (Tier 0)"
}

// loadModel 延迟加载模型
func (e *LocalEmbedder) loadModel() error {
	e.initOnce.Do(func() {
		// 使用 chromem-go 提供的默认 embedding function
		// 这个过程会下载模型文件（如果不存在）并加载到内存
		e.model = chromem.NewEmbeddingFuncDefault()
	})
	return e.initErr
}

func (e *LocalEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := e.loadModel(); err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	return e.model(ctx, text)
}

func (e *LocalEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if err := e.loadModel(); err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	results := make([][]float32, len(texts))
	for i, text := range texts {
		vec, err := e.model(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("local embed failed at index %d: %w", i, err)
		}
		results[i] = vec
	}
	return results, nil
}
