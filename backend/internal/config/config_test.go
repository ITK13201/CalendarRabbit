package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setRequiredDBEnv は DB 系の必須環境変数のみを設定する。
func setRequiredDBEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_USER", "root")
	t.Setenv("DB_PASSWORD", "password")
	t.Setenv("DB_NAME", "calendarrabbit")
}

// setRequiredEnv は既定プロバイダ（deepseek）で Load が成功する環境を用意する。
func setRequiredEnv(t *testing.T) {
	t.Helper()
	setRequiredDBEnv(t)
	t.Setenv("DEEPSEEK_API_KEY", "sk-deepseek")
	t.Setenv("SEARCH_API_KEY", "tvly-test")
	// 切り戻し用に Claude キーも設定しておく（既定では必須ではない）。
	t.Setenv("CLAUDE_API_KEY", "sk-test")
}

func TestLoad_Success(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "localhost", cfg.DBHost)
	assert.Equal(t, "root", cfg.DBUser)
	assert.Equal(t, "calendarrabbit", cfg.DBName)
	assert.Equal(t, "sk-test", cfg.ClaudeAPIKey)
	assert.Equal(t, "sk-deepseek", cfg.DeepSeekAPIKey)
	assert.Equal(t, "tvly-test", cfg.SearchAPIKey)
	// デフォルト値
	assert.Equal(t, defaultPort, cfg.Port)
	assert.Equal(t, defaultDBPort, cfg.DBPort)
	assert.Equal(t, defaultClaudeModel, cfg.ClaudeModel)
	assert.Equal(t, ProviderDeepSeek, cfg.LLMProvider)
	assert.Equal(t, defaultDeepSeekModel, cfg.DeepSeekModel)
	assert.Equal(t, defaultDeepSeekBaseURL, cfg.DeepSeekBaseURL)
	assert.Equal(t, defaultSearchProvider, cfg.SearchProvider)
	assert.Equal(t, defaultSearchMaxResults, cfg.SearchMaxResults)
}

func TestLoad_MissingRequired(t *testing.T) {
	// 必須の環境変数を空にする
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_NAME", "")
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("SEARCH_API_KEY", "")

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "DB_HOST")
	// 既定プロバイダ deepseek の必須項目
	assert.Contains(t, err.Error(), "DEEPSEEK_API_KEY")
	assert.Contains(t, err.Error(), "SEARCH_API_KEY")
}

func TestLoad_DeepSeekMissingKeys(t *testing.T) {
	setRequiredDBEnv(t)
	t.Setenv("LLM_PROVIDER", "deepseek")
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("SEARCH_API_KEY", "")

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "DEEPSEEK_API_KEY")
	assert.Contains(t, err.Error(), "SEARCH_API_KEY")
}

func TestLoad_ClaudeProviderMissingKey(t *testing.T) {
	setRequiredDBEnv(t)
	t.Setenv("LLM_PROVIDER", "claude")
	t.Setenv("CLAUDE_API_KEY", "")

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "CLAUDE_API_KEY")
}

func TestLoad_ClaudeProviderSuccess(t *testing.T) {
	setRequiredDBEnv(t)
	t.Setenv("LLM_PROVIDER", "claude")
	t.Setenv("CLAUDE_API_KEY", "sk-test")
	// claude 経路では DeepSeek/検索キーは不要
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("SEARCH_API_KEY", "")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, ProviderClaude, cfg.LLMProvider)
	assert.Equal(t, "sk-test", cfg.ClaudeAPIKey)
}

func TestLoad_UnknownProvider(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("LLM_PROVIDER", "gemini")

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "unknown LLM_PROVIDER")
}

func TestLoad_CustomModel(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CLAUDE_MODEL", "claude-opus-4-8")
	t.Setenv("DEEPSEEK_MODEL", "DeepSeek-V4-Pro-9999")
	t.Setenv("SEARCH_MAX_RESULTS", "8")
	t.Setenv("PORT", "9090")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "claude-opus-4-8", cfg.ClaudeModel)
	assert.Equal(t, "DeepSeek-V4-Pro-9999", cfg.DeepSeekModel)
	assert.Equal(t, 8, cfg.SearchMaxResults)
	assert.Equal(t, "9090", cfg.Port)
}

func TestDSN(t *testing.T) {
	setRequiredEnv(t)
	cfg, err := Load()
	require.NoError(t, err)

	dsn := cfg.DSN()
	assert.Contains(t, dsn, "root:password@tcp(localhost:3306)/calendarrabbit")
	assert.Contains(t, dsn, "parseTime=true")
	assert.Contains(t, dsn, "loc=UTC")
}

func TestParseOrigins(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5173, https://example.com ,")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"http://localhost:5173", "https://example.com"}, cfg.AllowedOrigins)
}
