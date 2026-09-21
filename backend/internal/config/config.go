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

	// LLM provider 選択（"deepseek" | "claude"）
	LLMProvider string

	// Claude
	ClaudeAPIKey string
	ClaudeModel  string

	// DeepSeek（OpenAI 互換 API）
	DeepSeekAPIKey  string
	DeepSeekModel   string
	DeepSeekBaseURL string

	// web 検索 API（DeepSeek 経路の事前検索用）
	SearchProvider   string
	SearchAPIKey     string
	SearchMaxResults int

	// Google Calendar 連携（任意機能。未設定でも Load は成功する）
	GoogleOAuthClientID     string
	GoogleOAuthClientSecret string
	GoogleOAuthRedirectURL  string
	GoogleTokenEncKey       string

	// CORS
	AllowedOrigins []string
}

// GoogleSyncEnabled は Google Calendar 連携が設定済み（利用可能）かどうかを返す。
// OAuth クライアント情報とトークン暗号鍵がすべて揃っている場合のみ有効とする。
func (c *Config) GoogleSyncEnabled() bool {
	return c.GoogleOAuthClientID != "" &&
		c.GoogleOAuthClientSecret != "" &&
		c.GoogleOAuthRedirectURL != "" &&
		c.GoogleTokenEncKey != ""
}

// LLM プロバイダ識別子。
const (
	ProviderDeepSeek = "deepseek"
	ProviderClaude   = "claude"
)

const (
	defaultPort             = "8080"
	defaultDBPort           = "3306"
	defaultClaudeModel      = "claude-sonnet-4-6"
	defaultLLMProvider      = ProviderDeepSeek
	defaultDeepSeekModel    = "deepseek-v4-pro"
	defaultDeepSeekBaseURL  = "https://api.deepseek.com"
	defaultSearchProvider   = "tavily"
	defaultSearchMaxResults = 10
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

	provider := strings.ToLower(envOrDefault("LLM_PROVIDER", defaultLLMProvider))

	cfg := &Config{
		Port:             envOrDefault("PORT", defaultPort),
		DBHost:           requireEnv("DB_HOST"),
		DBPort:           envOrDefault("DB_PORT", defaultDBPort),
		DBUser:           requireEnv("DB_USER"),
		DBPassword:       requireEnv("DB_PASSWORD"),
		DBName:           requireEnv("DB_NAME"),
		LLMProvider:      provider,
		ClaudeAPIKey:     os.Getenv("CLAUDE_API_KEY"),
		ClaudeModel:      envOrDefault("CLAUDE_MODEL", defaultClaudeModel),
		DeepSeekAPIKey:   os.Getenv("DEEPSEEK_API_KEY"),
		DeepSeekModel:    envOrDefault("DEEPSEEK_MODEL", defaultDeepSeekModel),
		DeepSeekBaseURL:  envOrDefault("DEEPSEEK_BASE_URL", defaultDeepSeekBaseURL),
		SearchProvider:   envOrDefault("SEARCH_PROVIDER", defaultSearchProvider),
		SearchAPIKey:     os.Getenv("SEARCH_API_KEY"),
		SearchMaxResults: envIntOrDefault("SEARCH_MAX_RESULTS", defaultSearchMaxResults),
		// Google Calendar 連携（任意機能）。未設定でも Load は失敗させない（design.md D6）。
		GoogleOAuthClientID:     os.Getenv("GOOGLE_OAUTH_CLIENT_ID"),
		GoogleOAuthClientSecret: os.Getenv("GOOGLE_OAUTH_CLIENT_SECRET"),
		GoogleOAuthRedirectURL:  os.Getenv("GOOGLE_OAUTH_REDIRECT_URL"),
		GoogleTokenEncKey:       os.Getenv("GOOGLE_TOKEN_ENC_KEY"),
		AllowedOrigins:          parseOrigins(envOrDefault("ALLOWED_ORIGINS", "http://localhost:5173")),
	}

	// プロバイダに応じて必須項目を切り替える（design.md D6）。
	switch provider {
	case ProviderDeepSeek:
		if cfg.DeepSeekAPIKey == "" {
			missing = append(missing, "DEEPSEEK_API_KEY")
		}
		if cfg.SearchAPIKey == "" {
			missing = append(missing, "SEARCH_API_KEY")
		}
	case ProviderClaude:
		if cfg.ClaudeAPIKey == "" {
			missing = append(missing, "CLAUDE_API_KEY")
		}
	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER: %q (want %q or %q)", provider, ProviderDeepSeek, ProviderClaude)
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

// envIntOrDefault は整数の環境変数を読み込む。未設定・パース不能なら既定値を返す。
func envIntOrDefault(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
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
