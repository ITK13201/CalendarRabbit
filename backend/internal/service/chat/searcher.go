package chat

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// SearchResult は外部検索 API から得た 1 件の検索結果（本文抜粋＋URL）。
type SearchResult struct {
	Title   string
	URL     string
	Content string
}

// Searcher は外部の web 検索 API を抽象化する（design.md D4）。
// DeepSeek 経路では、この検索結果をコンテキストに注入して LLM を呼び出す。
type Searcher interface {
	Search(ctx context.Context, query string) ([]SearchResult, error)
}

// yearPattern は 4 桁の西暦を検出する（例: 2026 / 2026年）。
var yearPattern = regexp.MustCompile(`\d{4}`)

// searchContextKeywords はイベント開催情報へ検索を寄せるための補助キーワード。
const searchContextKeywords = "開催日程 会場 場所"

// buildSearchQuery はユーザー発話に年などの文脈を補って検索クエリを組み立てる（task 2.3）。
// 発話に西暦が含まれない場合は、今年と来年の両方を検索範囲として補う。
func buildSearchQuery(userMessage string, now time.Time) string {
	q := strings.TrimSpace(userMessage)
	if q == "" {
		return q
	}
	if !yearPattern.MatchString(q) {
		q = fmt.Sprintf("%s %d年 %d年", q, now.Year(), now.Year()+1)
	}
	return fmt.Sprintf("%s %s", q, searchContextKeywords)
}

// refineSearchQuery は not_found 時の再検索用にクエリを見直す（design.md D5）。
// 公式情報へ寄せるキーワードを付与して recall を補う。
func refineSearchQuery(query string) string {
	return fmt.Sprintf("%s 公式サイト 最新情報", strings.TrimSpace(query))
}

// formatSearchResults は検索結果を LLM へ注入するためのテキストへ整形する。
func formatSearchResults(results []SearchResult) string {
	if len(results) == 0 {
		return "（検索結果は得られませんでした）"
	}
	var sb strings.Builder
	for i, r := range results {
		fmt.Fprintf(&sb, "[%d] %s\nURL: %s\n%s\n\n", i+1, r.Title, r.URL, r.Content)
	}
	return strings.TrimRight(sb.String(), "\n")
}
