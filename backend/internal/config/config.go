package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

func init() {
	// 尝试加载 backend 目录下的 .env（不存在则忽略）
	_ = godotenv.Load()
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")
}

// Config 应用配置，可通过环境变量或 .env 覆盖
type Config struct {
	// Server
	Port        string
	JWTSecret   string
	DBPath      string
	// Security
	EncryptKey  string // 用于 AES 加密 OKX 密钥（32字节hex或直接取字符串hash）
	// LLM
	LLMProvider string // openai / deepseek / azure 等
	LLMBaseURL  string
	LLMAPIKey   string
	LLMModel    string
	// Static 前端静态目录（可选，服务打包产物）
	StaticDir   string
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// Load 加载配置
func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8080"),
		JWTSecret:   getEnv("JWT_SECRET", "okx-analytics-change-me-secret-9f2c8d1a7b6e"),
		DBPath:      getEnv("DB_PATH", "data/app.db"),
		EncryptKey:  getEnv("ENCRYPT_KEY", "okx-analytics-enc-key-32b-7f3a!@#"),
		LLMProvider: getEnv("LLM_PROVIDER", "openai"),
		LLMBaseURL:  getEnv("LLM_BASE_URL", ""),
		LLMAPIKey:   getEnv("LLM_API_KEY", ""),
		LLMModel:    getEnv("LLM_MODEL", ""),
		StaticDir:   getEnv("STATIC_DIR", ""),
	}
}

func (c *Config) ServerAddr() string {
	return ":" + c.Port
}
