## Why

イベント抽出に利用している LLM が Claude Sonnet（現行既定 `claude-sonnet-4-6`）で、単価が高く運用コストが嵩む。より安価な DeepSeek（`deepseek-v4-pro`）へ切り替えることで、同等の機能を保ちつつ推論コストを大幅に削減する。

## What Changes

- イベント抽出に使う既定 LLM を Claude Sonnet から DeepSeek（`deepseek-v4-pro`）へ切り替える。DeepSeek は OpenAI 互換の公式 API を直接利用する。
- LLM プロバイダを設定で切り替え可能にする（`deepseek` を既定、`claude` を残置）。既存の `Extractor` インターフェースを活かし、`ClaudeExtractor` はそのまま残し、新規に `DeepSeekExtractor` を追加する。後から切り戻せる状態を維持する。
- **BREAKING**: DeepSeek には Anthropic ネイティブの `web_search` ツール相当が無いため、DeepSeek 経路では web 検索の方式を変更する。アプリ側が先に外部検索 API（LLM 最適化された Tavily/Exa 等）で検索し、結果をコンテキストに注入して LLM を 1 回呼び出す方式（事前検索＋注入）に切り替える。`not_found` 時のみ 1 回だけ再検索する限定的 2 パスを許容する。
- 検索クエリはユーザー発話に年・場所などの文脈を補って組み立て、精度低下を抑える。
- Claude 経路は従来どおりネイティブ `web_search` を用いる実装を維持する（切り戻し用）。
- 設定（環境変数）を追加・変更する。プロバイダ選択に応じて必須項目を切り替える。

## Capabilities

### New Capabilities

（なし。既存の `chat-scheduling` の範囲で完結する。）

### Modified Capabilities

- `chat-scheduling`: 「Claudeとweb検索によるイベント特定」要件を、LLM プロバイダ非依存（DeepSeek 既定・Claude 残置）かつ web 検索を「事前検索＋コンテキスト注入（DeepSeek 経路）」に見直す。あわせて履歴上限要件の記述を特定プロバイダ名から「LLM」へ一般化する（振る舞いは不変）。

## Impact

- コード:
  - `backend/internal/service/chat/`: 新規 `DeepSeekExtractor`（OpenAI 互換クライアント）、web 検索の `Searcher` 抽象と実装（Tavily/Exa 等）、`ClaudeExtractor` は残置。
  - `backend/internal/config/config.go`: `LLM_PROVIDER`、`DEEPSEEK_API_KEY`、`DEEPSEEK_MODEL`、`DEEPSEEK_BASE_URL`、検索 API 用設定（`SEARCH_PROVIDER`/`SEARCH_API_KEY`/`SEARCH_MAX_RESULTS`）を追加。プロバイダに応じた必須検証。
  - `backend/cmd/server/main.go`: プロバイダ選択に応じた Extractor 組み立て。
- 依存: OpenAI 互換 Go SDK（DeepSeek 用、base_url 差し替え）、外部検索 API クライアント。`anthropic-sdk-go` は残置。
- 設定/デプロイ: `.env.op`（1Password 参照テンプレート）と Helm values の環境変数を更新。1Password item「Development/CalendarRabbit」へ新規フィールドを追加。
- 外部サービス: DeepSeek API、外部検索 API の課金が新たに発生（Anthropic のトークン単価＋検索従量課金は DeepSeek 経路では発生しなくなる）。
