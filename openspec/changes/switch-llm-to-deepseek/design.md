## Context

現状のイベント抽出は `backend/internal/service/chat/claude.go` の `ClaudeExtractor` に集約されており、Anthropic Go SDK を直接利用し、Anthropic ネイティブの `web_search` ツール（`WebSearchTool20250305`）で最新情報を取得している。抽象境界として `chat.Extractor` インターフェース（`Extract(ctx, history, userMessage) (*ExtractionResult, error)`）が既に存在する。設定は `backend/internal/config/config.go` が環境変数から読み込み、`main.go` が `NewClaudeExtractor(cfg.ClaudeAPIKey, cfg.ClaudeModel, logger)` で組み立てる。

DeepSeek には Anthropic ネイティブ検索に相当する機能が無いため、単なるモデル文字列差し替えでは成立しない。動機とコスト分析は proposal.md（Why / Impact）を参照。

## Goals / Non-Goals

**Goals:**

- `chat.Extractor` を実装する `DeepSeekExtractor` を追加し、DeepSeek 公式 API（OpenAI 互換）で動かす。
- LLM プロバイダを設定で切り替え可能にし、既定を `deepseek` にする。`ClaudeExtractor` は残置し切り戻し可能にする。
- DeepSeek 経路の web 検索を「事前検索＋コンテキスト注入（1 回呼び出し、not_found 時のみ 1 回再検索）」で実装する。
- 外部から観測可能なイベント抽出の契約（`ExtractionResult` の status と構造）をプロバイダ間で不変に保つ。

**Non-Goals:**

- Claude 経路の web 検索方式の変更（従来のネイティブ `web_search` を維持）。
- 検索方式 B（Function Calling によるモデル駆動の多段検索）の採用。
- プロンプトやスキーマ（`extractionDTO`/`eventDTO`）の再設計。既存契約を踏襲する。
- フロントエンド・DB スキーマの変更。

## Decisions

### D1: プロバイダ切替は既存 `Extractor` インターフェースで行う

`main.go` で `cfg.LLMProvider` に応じて `NewDeepSeekExtractor(...)` か `NewClaudeExtractor(...)` を選択して `Extractor` に代入する。usecase 層は `Extractor` にのみ依存するため変更不要。
- 代替案: `ClaudeExtractor` を DeepSeek 用に改造 → 切り戻し不可・責務混在で却下。
- 代替案: OpenRouter 等ゲートウェイ経由の単一実装 → 手数料が乗る・公式直の要件に反するため却下。

### D2: DeepSeek クライアントは OpenAI 互換 SDK ＋ base_url 差し替え

DeepSeek は OpenAI 互換 API のため、OpenAI 互換 Go SDK の `base_url` を DeepSeek のエンドポイント（既定 `https://api.deepseek.com`）に向けて利用する。具体的な SDK 選定と API 形状は実装時に Context7 で最新仕様を確認する。
- 代替案: 自前 HTTP クライアント → 再実装コスト・保守負担で却下。

### D3: web 検索は「事前検索＋注入（方式 A）」

DeepSeek 経路では、アプリが先に外部検索 API を呼び、整形済み結果（本文抜粋＋URL）をシステム／ユーザーコンテキストに注入して LLM を 1 回呼び出す。呼び出し往復を最小化してコストと失敗モードを抑え、DeepSeek の tool-calling 品質に依存しない。off_topic 判定用の事前分類は行わず、実装を単純化するため常に検索を実行する（off_topic 時の検索 API 無駄打ちは低頻度・低コストのため許容）。
- 代替案: 方式 B（Function Calling） → 主経路で往復が累積し割高、tool-calling 信頼性に依存するため却下（proposal.md の比較参照）。
- 代替案: 検索完全廃止 → コア機能（最新の開催日時・場所探索）が劣化するため却下。

### D4: 検索は `Searcher` 抽象を新設し、LLM 最適化 API を使う

`Searcher`（`Search(ctx, query) ([]SearchResult, error)` 程度）を定義し、Tavily/Exa 等 LLM 最適化された検索 API の実装を提供する。プロバイダと API キーは設定で与える。クエリはユーザー発話に年・場所などの文脈を補って組み立て、精度低下を抑える。

### D5: not_found 時の限定的 2 パス

初回検索＋抽出で `not_found` になった場合のみ、クエリを見直して最大 1 回だけ再検索・再抽出する。上限を固定し暴走を防ぐ。方式 B の反復検索の利点を、コストを抑えつつ部分的に回収する。

### D7: 日時は日本標準時（JST）基準、出力は日本語

DeepSeek 経路では Claude のネイティブ検索と異なり現在日時やタイムゾーンを暗黙に把握しないため、抽出品質が劣化する。対策として、(1) 現在日時（Asia/Tokyo, UTC+09:00）をプロンプトへ注入して相対日付の基準を与える、(2) 出力する RFC3339 日時は必ず `+09:00` オフセットとし `Z`（UTC）を禁止する、(3) 終日・複数日イベントの `ends_at` は最終開催日そのもの（翌日にずらさない）とする、(4) `message`・`description` は情報源が他言語でも日本語で提示する、をシステムプロンプトで規定する。フロントエンドはブラウザ TZ（JST）で描画するため、UTC で返すと終日イベントの日付が翌日へずれる不具合が発生していた。
- 代替案: バックエンドで UTC↔JST を補正 → LLM が返す日時の意図（終日/時刻）を後段で判別できず却下。プロンプトで JST 出力を強制するのが単純かつ確実。

### D6: 設定はプロバイダ条件付き必須

`LLM_PROVIDER`（`deepseek`|`claude`、既定 `deepseek`）を追加。`deepseek` 選択時は `DEEPSEEK_API_KEY` と検索 API 用の `SEARCH_API_KEY` を必須、`claude` 選択時は従来どおり `CLAUDE_API_KEY` を必須とする。`DEEPSEEK_MODEL`（既定 `deepseek-v4-pro`）・`DEEPSEEK_BASE_URL`・`SEARCH_PROVIDER`・`SEARCH_MAX_RESULTS` を追加。既存の `CLAUDE_*` は残置。ハードコードは避け既定値は定数化する。

> 注: 当初 `DEEPSEEK_MODEL` の既定を `DeepSeek-V4-Pro-0813` と想定していたが、DeepSeek 公式 API が受け付けるモデル名は `deepseek-flash` / `deepseek-v4-pro` であり、実装時に既定を `deepseek-v4-pro` に修正した。

## Risks / Trade-offs

- [事前検索は検索クエリの質に精度が左右される] → 文脈補完クエリ＋LLM 最適化検索 API を用い、not_found 時に 1 回再検索して recall を補う。
- [多段の絞り込みが必要な難ケースでネイティブ検索より弱くなりうる] → D5 の限定的 2 パスで緩和。許容できない場合は将来的に方式 B を別 change で検討。
- [検索 API・DeepSeek API の障害が抽出全体を止める] → タイムアウトとエラーハンドリングを実装、ログ出力（既存 `logging` を踏襲）。
- [プロバイダ間で出力フォーマット遵守度が異なり JSON 抽出が不安定になりうる] → 既存 `extractJSON`/`parseExtraction` を再利用し、system プロンプトで「JSON のみ出力」を明示（既存踏襲）。
- [新たな外部課金（検索 API）] → 無料枠内で運用、`SEARCH_MAX_RESULTS` で件数を制御。モデル単価差で総額は現状より低下する見込み（proposal.md 参照）。

## Migration Plan

1. 依存追加（OpenAI 互換 SDK・検索 API クライアント）。`anthropic-sdk-go` は残置。
2. `Searcher` 抽象と実装、`DeepSeekExtractor` を追加。`ClaudeExtractor` は不変。
3. `config.go` に新設定とプロバイダ条件付き検証を追加、`main.go` で分岐。
4. `.env.op`・Helm values・1Password item「Development/CalendarRabbit」へ新規環境変数を追加。
5. 既定 `LLM_PROVIDER=deepseek` でデプロイ。
6. ロールバック: `LLM_PROVIDER=claude` に切り替えるだけで従来の Claude＋ネイティブ検索へ即時復帰（コード切り戻し不要）。

## Open Questions

- 検索 API は Tavily か Exa か。想定利用量は 1 日 5 回程度と低頻度のため、いずれの無料枠でも十分に収まる見込み。料金・無料枠・返却フォーマットを実装前に比較し、最も安価（無料枠内）なものを選ぶ。specs・方式・タスク分割には影響しない。
