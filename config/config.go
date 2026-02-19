package config

import (
	"os"
	"strconv"
)

// Config 全局配置结构
type Config struct {
	// 服务监听端口
	Port string

	// SQLite 数据库文件路径
	DBPath string

	// Chromem 向量库持久化目录
	VectorDBPath string

	// Embedding API 配置（兼容 OpenAI 接口）
	EmbedAPIBase string // 例如 https://api.siliconflow.cn/v1
	EmbedAPIKey  string
	EmbedModel   string // 例如 BAAI/bge-m3

	// LLM API 配置（用于记忆摘要/提取，兼容 OpenAI 接口）
	LLMAPIBase string
	LLMAPIKey  string
	LLMModel   string

	// 是否禁用 LLM 摘要（默认 true 以节省成本）
	DisableLLMSummary bool

	// 向量维度（需与 Embedding 模型输出一致）
	EmbedDimension int

	// Embedding 策略: cloud_first, accuracy_first, local_only
	EmbeddingStrategy string

	// Cloudflare 配置
	CFAccountID string
	CFAPIToken  string
}

// Load 从环境变量中加载配置
func Load() *Config {
	dim, _ := strconv.Atoi(getEnv("EMBED_DIMENSION", "1024"))
	disableSummary, _ := strconv.ParseBool(getEnv("DISABLE_LLM_SUMMARY", "true"))

	return &Config{
		Port:              getEnv("PORT", "8080"),
		DBPath:            getEnv("DB_PATH", "./data/clawmem.db"),
		VectorDBPath:      getEnv("VECTOR_DB_PATH", "./data/vectors"),
		EmbedAPIBase:      getEnv("EMBED_API_BASE", "https://api.openai.com/v1"),
		EmbedAPIKey:       getEnv("EMBED_API_KEY", ""),
		EmbedModel:        getEnv("EMBED_MODEL", "text-embedding-3-small"),
		LLMAPIBase:        getEnv("LLM_API_BASE", "https://api.openai.com/v1"),
		LLMAPIKey:         getEnv("LLM_API_KEY", ""),
		LLMModel:          getEnv("LLM_MODEL", "gpt-4o-mini"),
		DisableLLMSummary: disableSummary,
		EmbedDimension:    dim,
		EmbeddingStrategy: getEnv("EMBEDDING_STRATEGY", "cloud_first"),
		CFAccountID:       getEnv("CF_ACCOUNT_ID", ""),
		CFAPIToken:        getEnv("CF_API_TOKEN", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
