package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_USER", "root")
	t.Setenv("DB_PASSWORD", "password")
	t.Setenv("DB_NAME", "calendarrabbit")
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
	// デフォルト値
	assert.Equal(t, defaultPort, cfg.Port)
	assert.Equal(t, defaultDBPort, cfg.DBPort)
	assert.Equal(t, defaultClaudeModel, cfg.ClaudeModel)
}

func TestLoad_MissingRequired(t *testing.T) {
	// 必須の環境変数を空にする
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_NAME", "")
	t.Setenv("CLAUDE_API_KEY", "")

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "DB_HOST")
	assert.Contains(t, err.Error(), "CLAUDE_API_KEY")
}

func TestLoad_CustomModel(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CLAUDE_MODEL", "claude-opus-4-8")
	t.Setenv("PORT", "9090")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "claude-opus-4-8", cfg.ClaudeModel)
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
