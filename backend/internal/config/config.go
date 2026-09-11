package config

import (
	"os"
	"strings"

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
	Port string
	// MySQL
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBDSN      string // 可选：完整 DSN，设置后覆盖上面分项
	// Security
	JWTSecret  string
	EncryptKey string // 用于 AES 加密 OKX 密钥（32字节hex或直接取字符串hash）
	// LLM
	LLMProvider string // openai / deepseek / azure 等
	LLMBaseURL  string
	LLMAPIKey   string
	LLMModel    string
	// Static 前端静态目录（可选，服务打包产物）
	StaticDir   string
	CORSOrigins []string
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Load 加载配置
func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8080"),
		DBHost:      getEnv("DB_HOST", "127.0.0.1"),
		DBPort:      getEnv("DB_PORT", "3306"),
		DBUser:      getEnv("DB_USER", "rjdj"),
		DBPassword:  getEnv("DB_PASSWORD", "rjdj_pass"),
		DBName:      getEnv("DB_NAME", "rjdj"),
		DBDSN:       getEnv("DB_DSN", ""),
		JWTSecret:   getEnv("JWT_SECRET", "okx-analytics-change-me-secret-9f2c8d1a7b6e"),
		EncryptKey:  getEnv("ENCRYPT_KEY", "okx-analytics-enc-key-32b-7f3a!@#"),
		LLMProvider: getEnv("LLM_PROVIDER", "openai"),
		LLMBaseURL:  getEnv("LLM_BASE_URL", ""),
		LLMAPIKey:   getEnv("LLM_API_KEY", ""),
		LLMModel:    getEnv("LLM_MODEL", ""),
		StaticDir:   getEnv("STATIC_DIR", ""),
		CORSOrigins: splitCSV(getEnv("CORS_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173,http://localhost:3000,http://localhost")),
	}
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func (c *Config) ServerAddr() string {
	return ":" + c.Port
}
