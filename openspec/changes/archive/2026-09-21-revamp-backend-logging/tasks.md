## 1. ログ共通基盤（internal/logging）

- [x] 1.1 カスタム `slog.Handler`（`contextHandler`）を実装し、context の `requestId`/`method`/`path`/`query` をトップレベルへ昇格、その他の属性を `extra` グループへネストする（design D1/D2）。単体テストで、任意属性が `extra` 配下に入り予約キーがトップレベルに出ることを検証する
- [x] 1.2 HTTP 情報（`method`/`path`/`query`）を context へ格納/取得するヘルパ（`WithHTTPInfo` 等）を追加し、既存の `WithRequestID`/`RequestIDFromContext` と整合させる。単体テストで往復（set→get）を検証する
- [x] 1.3 defer 計装ヘルパ `Trace(ctx, logger, name, args, &res, &err) func()` を実装（started 即時／finished を返り関数で出力、`extra` に args/result/error、error 時は Error レベル）。`args`・`result` には D6 マスクと `Truncate`（1000文字）を一律適用する（design D3・確認済み）。単体テストで started/finished 2本が出力され、result/error が反映され、長い値が丸め・マスクされることを検証する
- [x] 1.4 文字列丸めユーティリティ `Truncate`（rune 単位・先頭1000文字＋省略記号）を共通化し、`service/chat` 内の既存 `truncate` を置換する。単体テストで1000文字境界の丸めを検証する（HTTP ボディの上限 4KB は 2.1 の別定数）
- [x] 1.5 機密情報マスクユーティリティ（既定キー集合 `authorization`/`x-api-key`/`cookie`/`set-cookie` を小文字比較、値を `[REDACTED]` へ置換）を実装（design D6）。単体テストで大文字小文字非依存のマスクを検証する
- [x] 1.6 `logging.New`/`Default` を `contextHandler` を用いる構成へ更新する。`go test ./internal/logging/...` が通ることを確認する

## 2. HTTP ミドルウェアの刷新

- [x] 2.1 `middleware/logger.go` を `http.request`（query・ヘッダー・ボディを `extra`）/`http.response`（status・ヘッダー・ボディ・`latency_ms` を `extra`）の2本立てへ刷新し、`bodyWriter` でレスポンスボディを `logBodyMaxBytes = 4KB`（確認済み）まで捕捉、超過時は `...(truncated)` を付す。リクエストボディは読み取り後に復元する（design D5）。既存の単一 `access` ログを撤去する
- [x] 2.2 ミドルウェアでヘッダーへ D6 マスクを適用し、`method`/`path`/`query` を context へ格納（1.2 のヘルパ利用）する。RequestID ミドルウェアの後段で動作することを確認する
- [x] 2.3 `middleware/middleware_test.go` を更新し、`http.request`/`http.response` の出力・トップレベル項目・`extra` ネスト・機密ヘッダーのマスク・ボディがハンドラで再読み取り可能なことを検証する。`go test ./internal/middleware/...` が通ることを確認する

## 3. サービス層（internal/usecase）の計装

- [x] 3.1 `usecase/chat` の公開メソッド（`SendMessage`/`GetConversation`/`ClearConversation`/`ApproveProposal`/`RejectProposal`）を名前付き戻り値化し、先頭に `Trace` を付与する（`chat.UseCase.<Method>`）
- [x] 3.2 `usecase/calendar` の公開メソッド（`Create`/`Update`/`Delete`/`Get`/`List`/`ListByPeriod`）へ `Trace` を付与する（`calendar.UseCase.<Method>`）
- [x] 3.3 `usecase/settings` の公開メソッド（`Get`/`Update`）へ `Trace` を付与する（`settings.UseCase.<Method>`）
- [ ] 3.4 `go test ./internal/usecase/...` が通ることを確認し、送信フローを1回実行して各 usecase メソッドの started/finished が同一 `requestId` で出力されることを確認する

## 4. 外部API/DB（internal/service）の計装

- [x] 4.1 `service/persistence` の全リポジトリ公開メソッド（AppSetting/Message/Conversation/CalendarEvent/EventProposal の各メソッド）へ `Trace` を付与し、戻り値（エンティティ）を `result` として丸め対象で出力する（`persistence.<Repo>.<Method>`）
- [x] 4.2 `service/chat` の外部HTTP呼び出し（`TavilySearcher.Search`/`DeepSeekExtractor.Extract`/`ClaudeExtractor.Extract`）へ `Trace` を付与し、レスポンスボディ/戻り値を `extra.responseBody` として `Truncate`（先頭1000文字）で出力する。既存の個別診断ログ（`chat.deepseek.research`/`extracted`/`request_failed`/`parse_failed`、`chat.search.*`）は `Trace` へ集約し、started/finished で表せる重複ログは削除、固有の診断情報（parse 失敗時の `finish_reason`/`raw` 抜粋、再検索の `attempt`/`query` 等）のみ `extra` に残す（design D4・確認済み）
- [x] 4.3 `service/calendarprovider`（`DBProvider`）は `persistence` への薄い委譲であり二重ログを避けるため計装しない方針を明記する（DB境界は 4.1 の persistence に集約）。委譲構造をコメントで補足する
- [ ] 4.4 `go test ./internal/service/...` が通ることを確認し、外部呼び出しの started/finished とレスポンスボディの丸め・機密マスクが期待どおりであることを確認する

## 5. 初期化と統合検証

- [x] 5.1 `cmd/server/main.go` の logger 初期化を新ログ基盤へ差し替える（起動ログがトップレベル項目のみで出力されることを確認）
- [x] 5.2 `go build ./...` と `go vet ./...` が通ることを確認する
- [ ] 5.3 ローカル起動（`op run --env-file=.env.op -- docker compose ...`）でチャット送信→予定案生成の1フローを実行し、`http.request`/`http.response`・usecase・外部API/DB の各ログが単一 `requestId` で連鎖し、トップレベル構成・`extra` ネスト・機密マスク・ボディ丸めが仕様どおりであることを目視確認する
- [x] 5.4 `openspec validate revamp-backend-logging --strict` が通ることを確認する
