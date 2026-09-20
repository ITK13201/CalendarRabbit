## Context

動機は proposal.md（## Why）を参照。現状は `internal/logging`（`slog` の JSON ハンドラ + `LogContext` ヘルパ、context に `requestId` を伝播）と、middleware の単一 `access` ログのみ。`requestId` はトップレベルに出るが、`method`/`path`/`query` はイベントごとに手書きで、`extra` ネスト構造は存在しない。

制約:
- 単一ユーザー・認証なし・VPN前提のバックエンド（Go 1.26 / gin / ent / slog）。
- 標準 `log/slog` で完結させ、追加の外部ロギング依存は導入しない。
- レイヤ構成: `internal/usecase/*`（業務ロジック=本changeの「サービス層」）、`internal/service/*`（`persistence`=DB、`chat` の Tavily/DeepSeek/Claude=HTTP、`calendarprovider`=DB。本changeの「外部API/DB」）。
- 基盤パターンは [MoneyRabbit](https://github.com/ITK13201/MoneyRabbit) の `http.request`/`http.response` + `slog.Group("extra", ...)` を踏襲しつつ、`requestId`/`method`/`path`/`query` のトップレベル化は本プロジェクト向けに拡張する。

ユーザー確認済みの前提（planning 時の質疑）:
- 「サービス層」= `internal/usecase/*`、「外部API/DB」= `internal/service/*`。
- 計装は defer ベースのヘルパ方式（デコレータではない）。
- 機密ヘッダー/フィールドはマスクする。

## Goals / Non-Goals

**Goals:**
- 全ログで `time, level, msg, requestId, method, path, query` をトップレベル、それ以外を `extra` 配下に統一する仕組みを、呼び出し側の記述に依存せず（横断的に）実現する。
- HTTP リクエスト/レスポンス、usecase メソッド、外部API/DBメソッドの入出力を1つの `requestId` で追跡可能にする。
- 各メソッドへの計装追加を「先頭1行の defer」で完結させ、ボイラープレートを最小化する。

**Non-Goals:**
- メトリクス/トレーシング（OpenTelemetry 等）の導入。
- ent が発行する個々の SQL レベルのフック計装（計装境界は `persistence` リポジトリのメソッド）。
- ログ集約基盤・ローテーション・出力先（stdout 固定のまま）の変更。
- 外部から観測可能な API 仕様・機能振る舞いの変更。

## Decisions

### D1: トップレベル項目の promotion はカスタム `slog.Handler` で行う

`requestId`/`method`/`path`/`query` を全ログのトップレベルへ確実に出し、その他を `extra` にネストするため、`slog.JSONHandler` をラップするカスタムハンドラ `contextHandler` を `internal/logging` に実装する。`Handle(ctx, record)` で:
1. context から `requestId`/`method`/`path`/`query` を取り出しトップレベル属性として付与する。
2. レコードが持つその他の属性を `slog.Group("extra", ...)` に相当する形へ再構成し、`extra` 配下へ移す（`requestId`/`method`/`path`/`query` の予約キーは昇格側で扱い重複させない）。

- **代替案**: 呼び出し側が毎回 `slog.Group("extra", ...)` を書く MoneyRabbit 方式。→ usecase/外部層でも `method`/`path`/`query` をトップレベルに出す要件があり、context 由来値の昇格が必須。呼び出し側規約だけでは漏れる/冗長になるためハンドラ方式を採用。

### D2: `method`/`path`/`query` を context へ伝播

現状 `requestId` のみ context 伝播している。middleware（RequestID の後段）で `method`/`path`/`query` も context へ格納するヘルパ（`logging.WithHTTPInfo` 等）を追加し、D1 のハンドラがこれらを昇格できるようにする。usecase/外部層のログは同一 context を引き継ぐため、追加記述なしにトップレベルへ反映される。

- `requestId` のキー名は既存の `requestId`（camelCase）を踏襲（MoneyRabbit の `request_id` ではなく現行コードに合わせる）。

### D3: defer 計装ヘルパ `logging.Trace`

usecase・外部層の各メソッド先頭に1行だけ書く計装ヘルパを提供する。概形:

```go
func (u *UseCase) SendMessage(ctx context.Context, content string) (res *SendResult, err error) {
    defer logging.Trace(ctx, u.logger, "chat.UseCase.SendMessage",
        logging.Args{"content": content}, &res, &err)()
    ...
}
```

- `Trace` は即時に `[Name] started`（`extra` に `args`）を出力し、戻り値として「finished を出す関数」を返す。呼び出し側が `()` で defer 実行すると、`[Name] finished`（`extra` に `result`・`error`）を出力する。
- 戻り値・エラーはポインタ（`&res, &err`）で受け、defer 実行時点（return 確定後）の値を読む。名前付き戻り値を用いる。
- `Args` は `map[string]any`。`result` は任意の値を受ける汎用ヘルパ（`any` ラッパを用意し、CLAUDE.md の「`any` を避ける」方針とは計装基盤の内部境界として最小限に留める。公開シグネチャでは型付き引数を優先）。
- レベルは既定 `Info`、`err != nil` の finished は `Error`（またはメソッド性質に応じて呼び分け可能にする）。
- **args/result への丸め・マスク適用（確認済み）**: `Trace` は `args`（started/finished）と `result`（finished）の出力時に、D6 のマスクと `Truncate`（先頭1000文字）を一律適用する。usecase 層（サービス層）のユーザーメッセージ・検索結果などが肥大化・漏洩しないようにする。丸め粒度は「外部レスポンスボディ＝1000文字」に揃え、HTTP ボディのみ D5 の 4KB を用いる（HTTP と内部計装で上限が異なる点は意図的）。

- **代替案**: 各インターフェースを実装するデコレータ構造体。→ 対象メソッドが多くコード量・保守コストが大きい。defer 方式は変更が局所的で、名前付き戻り値さえ用意すれば戻り値/エラーを正確に捕捉できるため採用。

### D4: 外部API/DB のレスポンスボディ取り込みと丸め

- HTTP 外部呼び出し（Tavily/DeepSeek/Claude）: レスポンスボディ（またはパース済みの戻り値表現）を `finished` ログの `extra.responseBody` に含める。1000文字超は `logging.Truncate`（rune 単位、既存の `truncate` を昇格・共通化）で先頭1000文字＋省略記号に丸める。
- DB（persistence）: 「レスポンスボディ」に相当するのは戻り値エンティティ。戻り値を `result` として出力（同じく丸め対象）。
- **既存の個別ログの扱い（確認済み）**: DeepSeek/Tavily 内の個別ログ（`chat.deepseek.research`/`extracted`/`request_failed`/`parse_failed`、`chat.search.*` 等）は `Trace` の started/finished へ集約し、重複するもの（呼び出し成功/失敗・件数など started/finished で表現できるもの）は削除する。started/finished では表せない固有の診断情報（例: parse 失敗時の `finish_reason`・`raw` 抜粋、not_found 再検索の `attempt`/`query`）のみ `extra` に残す。

### D5: middleware の刷新

`middleware/logger.go` を `http.request`/`http.response` 2本立てへ刷新（MoneyRabbit の `bodyWriter` パターンを踏襲）。HTTP リクエスト/レスポンスボディの上限は定数 `logBodyMaxBytes = 4KB`（確認済み。MoneyRabbit 準拠）とし、超過時は `...(truncated)` を付す。ヘッダーは D6 のマスクを適用して出力。`requestid.go` は現行の `requestId` キー・`X-Request-Id` ヘッダを維持しつつ、D2 の HTTP 情報 context 格納を追加（責務が近ければ Logger 側で格納してもよい）。既存の `access` ログは撤去。

### D6: 機密情報マスク

`internal/logging` にマスクユーティリティを置く。既定のマスク対象キー集合（`authorization`, `x-api-key`, `cookie`, `set-cookie` 等、小文字正規化して比較）を持ち、ヘッダーマップ/引数マップを走査して該当値を `[REDACTED]` に置換する。middleware（ヘッダー）と `Trace`（`args` および `result`）の双方から利用する（D3 確認済み）。

## Risks / Trade-offs

- **ログ量・容量の増加** → ボディは 4KB（HTTP）/1000文字（外部レスポンス）で丸め、`result`/`args` も大きい場合は丸め対象にする。必要ならレベルやサンプリングで抑制。
- **機密情報の漏洩** → D6 のマスクを middleware と計装ヘルパの両方に適用。マスク対象キーは定数集合で一元管理し、漏れを避ける。ボディ内 JSON の機密値（APIキー等）は完全には防げない点をトレードオフとして受容（単一ユーザー・VPN前提）。
- **名前付き戻り値への依存** → `Trace` が戻り値/エラーをポインタで捕捉するため、対象メソッドは名前付き戻り値化が必要。シグネチャ変更に伴う軽微な差分が広範囲に発生する。→ 機械的変更で対応、テストで回帰確認。
- **ログキー構成の非互換**（`access`→`http.request`/`http.response`、`extra` ネスト化） → 既存ログ利用側があれば移行が必要。単一運用者・小規模のため影響は限定的。Migration で周知。
- **性能オーバーヘッド**（ボディバッファリング・リフレクション的な値整形） → 上限バイトでのバッファリングに留め、`extra` 整形は軽量な属性再構成に限定。

## Migration Plan

1. `internal/logging` に基盤（`contextHandler`、context 伝播ヘルパ、`Trace`、`Truncate`、マスク）を追加。
2. `cmd/server/main.go` の logger 初期化を新ハンドラへ差し替え。
3. middleware を `http.request`/`http.response` へ刷新し `access` を撤去、HTTP 情報の context 格納を追加。
4. usecase 各メソッド → 外部層各メソッドの順で `Trace` を付与（名前付き戻り値化）。
5. 既存の個別ログを新方式へ整理・重複排除。
6. `go build` / `go vet` / `go test ./...` で回帰確認。ログ出力を1リクエスト分手動確認し、トップレベル項目・`extra` ネスト・マスク・丸めを検証。

ロールバック: 本changeはログ出力形式の変更が中心で外部振る舞いに影響しないため、リバートで即時復旧可能。
