package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/ITK13201/CalendarRabbit/backend/internal/logging"
)

// defaultTavilyBaseURL は Tavily 検索 API のエンドポイント。
const defaultTavilyBaseURL = "https://api.tavily.com"

// defaultSearchTimeout は検索 API 呼び出しのタイムアウト。
const defaultSearchTimeout = 15 * time.Second

// httpDoer は http.Client を抽象化する（テストでモック化可能にする）。
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// TavilySearcher は Tavily 検索 API を用いた Searcher 実装。
type TavilySearcher struct {
	apiKey     string
	baseURL    string
	maxResults int
	httpClient httpDoer
	logger     *slog.Logger
}

var _ Searcher = (*TavilySearcher)(nil)

// NewTavilySearcher は API キーと最大件数から TavilySearcher を生成する。
func NewTavilySearcher(apiKey string, maxResults int, logger *slog.Logger) *TavilySearcher {
	if maxResults <= 0 {
		maxResults = 10
	}
	return &TavilySearcher{
		apiKey:     apiKey,
		baseURL:    defaultTavilyBaseURL,
		maxResults: maxResults,
		httpClient: &http.Client{Timeout: defaultSearchTimeout},
		logger:     logger,
	}
}

// tavilyRequest は Tavily /search のリクエストボディ。
type tavilyRequest struct {
	Query       string `json:"query"`
	MaxResults  int    `json:"max_results"`
	SearchDepth string `json:"search_depth"`
}

// tavilyResponse は Tavily /search のレスポンスボディ。
type tavilyResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}

// Search は Tavily で検索し、本文抜粋＋URL の結果を返す。
// started/finished（引数・戻り値・エラー）は Trace に集約し、request_failed/non_200/
// decode_failed/completed の個別ログは撤去する（design D4）。
func (s *TavilySearcher) Search(ctx context.Context, query string) (res []SearchResult, err error) {
	defer logging.Trace(ctx, s.logger, "chat.TavilySearcher.Search", logging.Args{"query": query}, &res, &err)()

	body, err := json.Marshal(tavilyRequest{
		Query:       query,
		MaxResults:  s.maxResults,
		SearchDepth: "advanced",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tavily request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to build tavily request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	callStart := time.Now()
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	callLatency := time.Since(callStart)
	if err != nil {
		return nil, fmt.Errorf("failed to read tavily response: %w", err)
	}
	// レスポンスボディを extra.responseBody として丸め、外部API実行の latency_ms を出力する（design D4）。
	logging.LogContext(ctx, s.logger, slog.LevelInfo, "chat.tavily.response",
		slog.Int("status", resp.StatusCode),
		slog.Float64("latency_ms", logging.DurationMillis(callLatency)),
		slog.String("responseBody", logging.Truncate(string(raw), logging.MaxTruncateRunes)))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tavily returned status %d", resp.StatusCode)
	}

	var parsed tavilyResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("failed to decode tavily response: %w", err)
	}

	results := make([]SearchResult, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		results = append(results, SearchResult{Title: r.Title, URL: r.URL, Content: r.Content})
	}
	return results, nil
}
