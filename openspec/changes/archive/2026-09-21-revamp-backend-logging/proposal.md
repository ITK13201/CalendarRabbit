## Why

現状のバックエンドのログは、単一の `access` ログ（middleware）と各所に散在する `LogContext` 呼び出しにとどまり、リクエスト/レスポンスの内容（ヘッダー・ボディ）や、業務処理・外部API/DB呼び出しの入出力を体系的に追跡できない。障害調査や挙動確認の際に「どのリクエストで・どのメソッドに・どんな引数を渡し・何が返ったか」を再構成できず、可観測性が不足している。姉妹プロジェクト [MoneyRabbit](https://github.com/ITK13201/MoneyRabbit) で確立したログ構造（トップレベル項目 + `extra` ネスト、`http.request`/`http.response`）を基盤として、CalendarRabbit のログ全体を統一・拡充する。

## What Changes

- **ログ全体構成の統一**: すべてのログで `time`, `level`, `msg`, `requestId`, `method`, `path`, `query` をトップレベルに配置し、それ以外の任意プロパティは `extra` 配下にネストする。これを実現する共通ログ基盤（slog カスタムハンドラ + context 伝播）を導入する。
- **リクエスト/レスポンスログ**: middleware を刷新し、リクエスト時に `msg='http.request'`（query・ヘッダー・ボディ）、レスポンス時に `msg='http.response'`（ステータスコード・ヘッダー・ボディ・`latency_ms`）を出力する。既存の単一 `access` ログは置き換える。
- **サービス層（usecase）の前後ログ**: `internal/usecase/*`（chat/calendar/settings の UseCase）のメソッド呼び出し前後で `msg='[StructName.MethodName] started'` / `finished` を出力し、requestId・メソッド名・引数・戻り値・エラー情報を含める。
- **外部API/DB呼び出しの前後ログ**: `internal/service/*`（persistence=DB、chat の Tavily/DeepSeek/Claude=HTTP、calendarprovider=DB）のメソッド呼び出し前後で `msg='[StructName.MethodName] started'` / `finished` を出力する。requestId・メソッド名・引数・戻り値・エラー情報に加え、レスポンスボディをできる限り含める（長すぎる場合は先頭1000文字に丸める）。
- **共通の計装ヘルパ**: defer ベースの計装ヘルパ（started を即時、finished を defer で出力し、戻り値・エラーをポインタ経由で取得）を導入し、各メソッド先頭1行で計装できるようにする。
- **機密情報のマスク**: `Authorization` / `X-Api-Key` / `Cookie` / `Set-Cookie` などの既知の機密ヘッダー・フィールドは値を `[REDACTED]` に置換して出力する。

## Capabilities

### New Capabilities
- `backend-observability`: バックエンドの構造化ロギングと可観測性。ログの全体構成（トップレベル項目 + `extra` ネスト）、HTTP リクエスト/レスポンスログ、サービス層および外部API/DB呼び出しの前後ログ、機密情報のマスク、ボディ長の丸めといった、外部から観測可能なログの振る舞いを規定する。

### Modified Capabilities
（なし。ログは横断的関心事であり、既存の機能キャパビリティの要求振る舞いは変更しない。）

## Impact

- **新規/刷新コード**:
  - `internal/logging`: 共通ログ基盤（カスタム slog ハンドラ、context への `method`/`path`/`query` 伝播、defer 計装ヘルパ、機密マスク・ボディ丸めユーティリティ）。
  - `internal/middleware/logger.go`・`requestid.go`: `http.request`/`http.response` ログへ刷新。
  - `internal/usecase/*`（chat/calendar/settings）: 各公開メソッドへ計装を追加。
  - `internal/service/*`（persistence、chat: tavily/deepseek/claude、calendarprovider）: 各外部呼び出しメソッドへ計装を追加。
  - `cmd/server/main.go`: 新ログ基盤の初期化。
- **ログ出力形式の変更（互換性）**: JSON ログのキー構成が変わる（`extra` ネスト化、`access` → `http.request`/`http.response`）。ログを集計・パースしている仕組みがあれば影響を受ける。
- **依存**: 追加の外部依存は基本的に不要（標準 `log/slog` と既存の gin/uuid で完結）。
- **パフォーマンス/容量**: リクエスト/レスポンスボディや外部レスポンスボディをログ化するため、ログ量が増える。ボディは上限バイト/1000文字で丸めて肥大化を抑える。
