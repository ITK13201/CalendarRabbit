package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config はアプリケーション全体の設定を保持する。
// 環境変数から読み込み、必須項目が欠けている場合はエラーを返す。
type Config struct {
	// Server
	Port string

	// Database (MySQL)
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Claude
	ClaudeAPIKey string
	ClaudeModel  string

	// CORS
	AllowedOrigins []string
}

const (
	defaultPort        = "8080"
	defaultDBPort      = "3306"
	defaultClaudeModel = "claude-sonnet-4-6"
)

// Load は環境変数から設定を読み込む。
// 必須の環境変数（DB接続情報・CLAUDE_API_KEY）が欠けている場合はエラーを返す。
func Load() (*Config, error) {
	var missing []string

	requireEnv := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := &Config{
		Port:           envOrDefault("PORT", defaultPort),
		DBHost:         requireEnv("DB_HOST"),
		DBPort:         envOrDefault("DB_PORT", defaultDBPort),
		DBUser:         requireEnv("DB_USER"),
		DBPassword:     requireEnv("DB_PASSWORD"),
		DBName:         requireEnv("DB_NAME"),
		ClaudeAPIKey:   requireEnv("CLAUDE_API_KEY"),
		ClaudeModel:    envOrDefault("CLAUDE_MODEL", defaultClaudeModel),
		AllowedOrigins: parseOrigins(envOrDefault("ALLOWED_ORIGINS", "http://localhost:5173")),
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("required environment variables are missing: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

// DSN はMySQL接続用のDSN文字列を返す。
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=UTC&charset=utf8mb4",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName,
	)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

// PortNumber はポート番号を整数として返す（バリデーション用途）。
func (c *Config) PortNumber() (int, error) {
	return strconv.Atoi(c.Port)
}
