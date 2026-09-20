## 1. 依存関係と設定

- [x] 1.1 OpenAI 互換 Go SDK と検索 API クライアントの依存を追加し、`go mod tidy` が成功して `anthropic-sdk-go` も残っていることを確認する（Context7 で最新仕様を確認してから選定）
- [x] 1.2 `config.go` に `LLMProvider`/`DeepSeekAPIKey`/`DeepSeekModel`（既定 `deepseek-v4-pro`）/`DeepSeekBaseURL`（既定 `https://api.deepseek.com`）/`SearchProvider`/`SearchAPIKey`/`SearchMaxResults` を追加し、既定値を定数化する
- [x] 1.3 `LLM_PROVIDER=deepseek` 時は `DEEPSEEK_API_KEY` と `SEARCH_API_KEY` を必須、`claude` 時は `CLAUDE_API_KEY` を必須とする条件付き検証を実装し、`config_test.go` に各プロバイダの必須欠落ケースのテストを追加してパスさせる

## 2. web 検索サービス（Searcher）

- [x] 2.1 `chat` パッケージに `Searcher` インターフェース（`Search(ctx, query) ([]SearchResult, error)`）と `SearchResult` 型を定義する
- [x] 2.2 選定した検索 API の `Searcher` 実装（本文抜粋＋URL を返す）を追加し、タイムアウト・エラーハンドリング・`logging` 出力を組み込む
- [x] 2.3 ユーザー発話に年・場所などの文脈を補って検索クエリを組み立てるヘルパを実装し、代表入力でクエリが期待どおり組まれることをユニットテストで検証する

## 3. DeepSeekExtractor

- [x] 3.1 `DeepSeekExtractor` を追加し `chat.Extractor` を実装する（OpenAI 互換クライアント、base_url は `DeepSeekBaseURL`、モデルは `DeepSeekModel`）
- [x] 3.2 事前検索→結果をコンテキスト注入→LLM 1 回呼び出しのフローを実装し、既存 `extractJSON`/`parseExtraction` を再利用して `ExtractionResult` を返す
- [x] 3.3 初回抽出が `not_found` の場合にクエリを見直して最大 1 回だけ再検索・再抽出する限定的 2 パスを実装する（再検索回数の上限を固定）
- [x] 3.4 モック `Searcher` とモック LLM クライアントを注入し、event/multiple/not_found（再検索あり）/off_topic の各分岐を検証するユニットテストを追加してパスさせる

## 4. プロバイダ選択の組み立て

- [x] 4.1 `main.go` で `cfg.LLMProvider` に応じて `DeepSeekExtractor` または既存 `ClaudeExtractor` を生成し `Extractor` として usecase に渡すよう分岐する（`ClaudeExtractor` は不変）
- [x] 4.2 起動ログに選択中のプロバイダとモデルを出力し、`LLM_PROVIDER` を切り替えて起動すると対応する Extractor が使われることを確認する

## 5. 設定・デプロイ反映

- [x] 5.1 `.env.op` に新規環境変数の 1Password 参照を追記し、item「Development/CalendarRabbit」へ追加すべきフィールドをコメントで明記する
- [x] 5.2 Helm chart の values / テンプレートへ新規環境変数を追加し、`helm template`（または lint）が成功することを確認する

## 6. 総合検証

- [x] 6.1 `LLM_PROVIDER=deepseek` で実際に対話し、イベント登録メッセージから検索結果に基づく予定案が生成されることを確認する（chat-scheduling 仕様のシナリオ）
- [x] 6.2 `LLM_PROVIDER=claude` に切り替えると従来の Claude＋ネイティブ検索へコード変更なしで復帰できることを確認する（ロールバック手順）
- [x] 6.3 `go test ./...` と `openspec validate switch-llm-to-deepseek --strict` がともにパスすることを確認する
