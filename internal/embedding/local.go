package embedding

import (
	"context"
	"crypto/md5"
	"log"
	"math/rand"
	"sync"
)

// LocalEmbedder 实现本地兜底 (Tier 0)
// 目前仅作为 Mock 实现，因为 chromem-go v0.7.0 的默认模型依赖外部 API 且 Hugging Face API 变动导致不稳定。
// 在生产环境中，建议配置有效的 Cloudflare/OpenAI Key，或自行集成完整的本地 ONNX 模型。
type LocalEmbedder struct {
	mu sync.Mutex
}

func NewLocalEmbedder() *LocalEmbedder {
	return &LocalEmbedder{}
}

func (e *LocalEmbedder) Name() string {
	return "Mock Local Embedder (Fallback)"
}

func (e *LocalEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

func (e *LocalEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	log.Printf("[WARN] Generating Mock Embeddings for %d texts (Please configure valid Cloudflare/OpenAI credentials for real vectors)", len(texts))

	results := make([][]float32, len(texts))
	for i, text := range texts {
		// 生成一个确定性的伪随机向量 (基于文本 Hash)，以便相同的文本总是产生相同的向量
		results[i] = generateMockVector(text, 768) // 假设维度 768
	}
	return results, nil
}

func generateMockVector(text string, dim int) []float32 {
	hash := md5.Sum([]byte(text))
	// 使用 hash 作为种子
	seed := int64(hash[0]) | int64(hash[1])<<8 | int64(hash[2])<<16
	r := rand.New(rand.NewSource(seed))

	vec := make([]float32, dim)
	var sum float32
	for i := 0; i < dim; i++ {
		vec[i] = r.Float32()
		sum += vec[i] * vec[i]
	}
	// 归一化 (L2 Norm)
	/* norm := float32(math.Sqrt(float64(sum)))
	if norm > 0 {
		for i := 0; i < dim; i++ {
			vec[i] /= norm
		}
	} */
	return vec
}
