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

	// 全局记忆存储总条数预算 (Phase 3)，超限则淘汰末尾
	MemoryMaxCount int

	// ========== Dream (记忆整合) 配置 ==========
	// 是否启用 Dream 功能（默认 false，不影响现有逻辑）
	DreamEnabled bool
	// Dream 执行间隔（如 "24h", "12h", "6h"），为空或 "0" 则禁用定时任务
	DreamInterval string
	// Dream 处理的时间窗口（如 "24h"），整合多久以内的记忆
	DreamWindow string
	// 触发 Dream 的最小记忆条数（低于此数不执行，避免浪费 Token）
	DreamMinCount int
	// Dream 使用的 LLM API 地址（为空则复用 LLMAPIBase）
	DreamLLMBase string
	// Dream 使用的 LLM API Key（为空则复用 LLMAPIKey）
	DreamLLMKey string
	// Dream 使用的 LLM 模型名（为空则复用 LLMModel）
	DreamLLMModel string
	// Dream 自定义 System Prompt（为空则使用内置默认 Prompt）
	DreamPrompt string
	// Dream 单次最大处理记忆条数（防止 Token 爆炸）
	DreamMaxItems int
}

// Load 从环境变量中加载配置
func Load() *Config {
	dim, _ := strconv.Atoi(getEnv("EMBED_DIMENSION", "1024"))
	disableSummary, _ := strconv.ParseBool(getEnv("DISABLE_LLM_SUMMARY", "true"))
	dreamEnabled, _ := strconv.ParseBool(getEnv("DREAM_ENABLED", "false"))
	dreamMinCount, _ := strconv.Atoi(getEnv("DREAM_MIN_COUNT", "10"))
	dreamMaxItems, _ := strconv.Atoi(getEnv("DREAM_MAX_ITEMS", "200"))
	memoryMaxCount, _ := strconv.Atoi(getEnv("MAX_MEMORY_COUNT", "5000"))

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

		// Dream 配置
		DreamEnabled:  dreamEnabled,
		DreamInterval: getEnv("DREAM_INTERVAL", "24h"),
		DreamWindow:   getEnv("DREAM_WINDOW", "24h"),
		DreamMinCount: dreamMinCount,
		DreamLLMBase:  getEnv("DREAM_LLM_BASE", ""),
		DreamLLMKey:   getEnv("DREAM_LLM_KEY", ""),
		DreamLLMModel: getEnv("DREAM_LLM_MODEL", ""),
		DreamPrompt:   getEnv("DREAM_PROMPT", ""),
		DreamMaxItems: dreamMaxItems,

		MemoryMaxCount: memoryMaxCount,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
