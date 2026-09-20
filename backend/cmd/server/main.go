// Package main は CalendarRabbit バックエンドのエントリポイント。
//
// @title           CalendarRabbit API
// @version         1.0
// @description     チャット駆動の予定登録を行う CalendarRabbit のバックエンドAPI（単一ユーザー・認証なし・VPN前提）。
// @BasePath        /
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	dbmigrations "github.com/ITK13201/CalendarRabbit/backend/db"
	"github.com/ITK13201/CalendarRabbit/backend/internal/config"
	"github.com/ITK13201/CalendarRabbit/backend/internal/handler"
	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/calendarprovider"
	chatservice "github.com/ITK13201/CalendarRabbit/backend/internal/service/chat"
	"github.com/ITK13201/CalendarRabbit/backend/internal/service/persistence"
	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/calendar"
	chatuc "github.com/ITK13201/CalendarRabbit/backend/internal/usecase/chat"
	"github.com/ITK13201/CalendarRabbit/backend/internal/usecase/settings"

	_ "github.com/ITK13201/CalendarRabbit/backend/docs/swagger"
	_ "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
	swaggerfiles "github.com/swaggo/files"
	ginswagger "github.com/swaggo/gin-swagger"
)

func main() {
	logger := logging.Default()
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := runMigrations(cfg.DSN(), logger); err != nil {
		logger.Error("failed to run migrations", slog.String("error", err.Error()))
		os.Exit(1)
	}

	client, err := persistence.Open(cfg.DSN())
	if err != nil {
		logger.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = client.Close() }()

	// usecases
	calUC := calendar.New(calendarprovider.NewDBProvider(persistence.NewCalendarEventRepository(client, logger)), logger)
	setUC := settings.New(persistence.NewAppSettingRepository(client, logger), logger)
	// 両プロバイダの Extractor を構築し、実行時に設定（設定画面）で選択する。
	extractors := buildExtractors(cfg, logger)
	chUC := chatuc.New(client, extractors, setUC, cfg.LLMProvider, logger)

	h := handler.New(handler.Deps{
		Calendar: calUC,
		Settings: setUC,
		Chat:     chUC,
		Logger:   logger,
	})
	engine := handler.NewRouter(h, handler.RouterConfig{
		Logger:         logger,
		AllowedOrigins: cfg.AllowedOrigins,
	})

	// Swagger UI
	engine.GET("/swagger/*any", ginswagger.WrapHandler(swaggerfiles.Handler))

	addr := ":" + cfg.Port
	logger.Info("starting server",
		slog.String("addr", addr),
		slog.String("defaultLlmProvider", cfg.LLMProvider),
		slog.String("claudeModel", cfg.ClaudeModel),
		slog.String("deepseekModel", cfg.DeepSeekModel),
	)
	if err := engine.Run(addr); err != nil {
		logger.Error("server exited", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// buildExtractors は利用可能な各 LLM プロバイダの Extractor を構築して
// プロバイダ名 -> Extractor のマップで返す（design.md D1）。
// 実行時にどれを使うかは設定画面（app_settings.llm_provider）で選択する。
func buildExtractors(cfg *config.Config, logger *slog.Logger) map[string]chatservice.Extractor {
	searcher := chatservice.NewTavilySearcher(cfg.SearchAPIKey, cfg.SearchMaxResults, logger)
	return map[string]chatservice.Extractor{
		config.ProviderDeepSeek: chatservice.NewDeepSeekExtractor(cfg.DeepSeekAPIKey, cfg.DeepSeekModel, cfg.DeepSeekBaseURL, searcher, logger),
		config.ProviderClaude:   chatservice.NewClaudeExtractor(cfg.ClaudeAPIKey, cfg.ClaudeModel, logger),
	}
}

// runMigrations は埋め込んだ goose マイグレーションを適用する。
func runMigrations(dsn string, logger *slog.Logger) error {
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// DB 起動待ち（compose 環境での競合対策）。
	for i := 0; i < 30; i++ {
		if err = sqlDB.PingContext(ctx); err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return err
	}

	goose.SetBaseFS(dbmigrations.MigrationsFS)
	goose.SetLogger(gooseLogger{logger: logger})
	if err := goose.SetDialect("mysql"); err != nil {
		return err
	}
	return goose.Up(sqlDB, dbmigrations.MigrationsDir)
}

// gooseLogger は goose のログを slog へ橋渡しする。
type gooseLogger struct {
	logger *slog.Logger
}

func (g gooseLogger) Printf(format string, v ...interface{}) {
	g.logger.Info("goose", slog.String("detail", strings.TrimRight(fmt.Sprintf(format, v...), "\n")))
}

func (g gooseLogger) Fatalf(format string, v ...interface{}) {
	g.logger.Error("goose", slog.String("detail", strings.TrimRight(fmt.Sprintf(format, v...), "\n")))
	os.Exit(1)
}
