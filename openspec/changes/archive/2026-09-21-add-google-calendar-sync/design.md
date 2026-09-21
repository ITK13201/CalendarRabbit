## Context

CalendarRabbit は単一ユーザー・認証なし（VPN 前提）の構成で、Go バックエンド（gin + ent + goose マイグレーション、MySQL）と PWA フロントエンドからなる。カレンダー永続化は既に `calendarprovider.CalendarProvider` インターフェースの背後に抽象化されており、現状は `DBProvider`（ent への薄い委譲）のみが実装されている。usecase 層（`usecase/calendar`）はこのインターフェースにのみ依存する。動機は proposal.md - Why を参照。

制約:
- 同期は一方向（CalendarRabbit → Google）、DB を正とする（proposal 決定事項）。
- 秘密情報は環境変数（1Password 経由 `.env.op`）で注入する既存慣習に従う。
- マイグレーションは `backend/db/migrations` の goose SQL として追加する。

## Goals / Non-Goals

**Goals:**
- usecase 層をほぼ変更せずに、`CalendarProvider` の背後で Google ミラー同期を差し込む。
- OAuth 連携・専用カレンダー作成・即時ミラー同期・初回バックフィル・連携解除・同期失敗耐性を提供する。
- Google 連携が未設定/未連携でも従来どおり DB のみで全機能が動作する（連携は任意機能）。

**Non-Goals:**
- Google → CalendarRabbit の取り込み（双方向同期）。Google 側の直接編集は取り込まない。
- Push 通知（Webhook）・定期ポーリングによる差分取得。
- 複数ユーザー・複数カレンダー・複数 Google アカウント。

## Decisions

### D1: ミラーリングは CalendarProvider のデコレータで実装する

`DBProvider` をラップする `MirroringProvider`（`CalendarProvider` を実装）を新設する。各メソッドは「まず内側の `DBProvider` に委譲して DB を確定 → 連携が有効なら Google へミラー」という順で動く。usecase 層は引き続き `CalendarProvider` にのみ依存し、`main.go` の組み立てで DB プロバイダを `MirroringProvider` で包むかどうかを切り替える。

- 代替案: usecase 層に同期ロジックを直接書く → 却下。抽象境界（provider）に閉じ込める既存設計方針（calendar-management の抽象化要件）に反する。
- 代替案: DB と Google の両プロバイダを束ねる合成プロバイダを対等に並べる → 却下。DB を正とし Google は失敗許容のミラーという非対称性があるため、デコレータの方が意図を表現できる。

### D2: OAuth 2.0 Authorization Code フロー（offline access）

`golang.org/x/oauth2` + `golang.org/x/oauth2/google` を用い、`access_type=offline` と（必要に応じ `prompt=consent`）で refresh token を取得する。CSRF 対策に `state` を用いる。エンドポイント（いずれも単一ユーザー前提）:
- `GET /api/google/auth`: 認可 URL を返す（フロントがリダイレクト）。
- `GET /api/google/callback`: 認可コードをトークンへ交換し、連携を確定してフロントの設定画面へ戻す。
- `GET /api/google/status`: 連携済み/未連携を返す。
- `DELETE /api/google/connection`: 連携解除。

スコープは専用カレンダーの作成とイベント読み書きに必要な最小限（`https://www.googleapis.com/auth/calendar`）とする。

### D3: 連携状態とイベントマッピングの永続化（ent + goose マイグレーション）

- 新規 ent スキーマ `GoogleConnection`（`app_setting` と同様に id=1 固定の単一レコード）: `refresh_token`（暗号化して保存）, `access_token`（キャッシュ）, `token_expiry`, `calendar_id`, `connected`(bool), `created_at`, `updated_at`。
- `CalendarEvent` にミラー用の列を追加: `google_event_id`（string, default ""）と `sync_pending`（bool, default false）。一対一・単一カレンダーのため専用テーブルは設けず列で表現する。
- 対応する goose マイグレーションを `backend/db/migrations` に追加する。

- 代替案: マッピングを別テーブル化 → 却下。単一カレンダー一対一では過剰。
- 代替案: refresh token を DB 以外（例: 1Password）に保存 → 却下。実行時に動的取得・更新される連携状態であり、DB 保存が自然（client_id/secret のような静的秘密のみ環境変数）。

### D3b: refresh token はアプリ層で暗号化して保存する

refresh token は DB 保存時に暗号化する（環境変数で与える鍵による対称暗号: AES-GCM を想定）。読み出し時に復号して利用する。鍵 `GOOGLE_TOKEN_ENC_KEY` は 1Password 管理（`.env.op`）。単一ユーザー・VPN 前提でも、DB 単体の漏えい時にトークンを保護する。

- 代替案: 平文保存 → 却下（ユーザー選択により暗号化を採用）。
- 代替案: KMS 等の外部鍵管理 → 却下。単一ユーザー・自ホスト構成には過剰で、環境変数鍵で十分。

### D7: 専用カレンダー名は「CalendarRabbit」固定

専用カレンダーの summary は定数「CalendarRabbit」とする（設定変更不可）。名称変更機能は Non-Goal。

### D8: 連携解除はカレンダー削除の要否をユーザーが選ぶ

連携解除 API は「専用カレンダーを削除するか残すか」のパラメータを受け取る。削除指定時は Google の該当カレンダーを削除してからローカル状態を破棄し、残す指定時はローカル状態のみ破棄する。フロントの解除 UI で明示的に選択させる。

### D9: 未同期イベントの手動再同期

`sync_pending=true` のイベントを一括再送する専用エンドポイント（`POST /api/google/resync`）を設ける。バックフィルと同じ再送ロジックを再利用し、成功分の `sync_pending` を解消する。フロント設定画面に「再同期」ボタンと未同期件数の表示を置く。ポーリングは行わない（Non-Goal）方針を、手動トリガーで補う。

### D4: 即時同期と失敗耐性

`MirroringProvider` は DB 操作成功後に同期を試みる。Google 呼び出しが失敗しても DB 操作は成功として返し、当該イベントの `sync_pending=true` を立てる（同期成功時に `google_event_id` を保存し `sync_pending=false`）。再送は「対象イベントの次回ミラー操作」および「連携（再）確立時のバックフィル」で `sync_pending` のイベントも対象に含めることで解消する。ポーリングは行わない（Non-Goal）。

- トレードオフ: 定期再送が無いため、一時的失敗のイベントは次の編集または再連携まで Google に反映されない。DB を正とする方針上許容し、Risks に明記する。

### D5: 日時・終日のマッピング

DB は UTC 保存 + `app_setting.timezone` で表示する既存方針。Google へは、終日イベントは `start.date`/`end.date`（日付のみ）、時刻付きイベントは `start.dateTime`/`end.dateTime` + `timeZone`（設定タイムゾーン）としてマッピングする。

### D6: 設定（環境変数）

`GOOGLE_OAUTH_CLIENT_ID` / `GOOGLE_OAUTH_CLIENT_SECRET` / `GOOGLE_OAUTH_REDIRECT_URL` / `GOOGLE_TOKEN_ENC_KEY`（refresh token 暗号鍵、D3b）を追加する。連携は任意機能のため、これらが未設定でもサーバは起動し、連携系エンドポイントは「未設定」を返して DB のみで動作する（LLM プロバイダで既に採用している「必須項目をモード別に切り替える」方針と整合）。値は 1Password 管理（`.env.op`）。

## Risks / Trade-offs

- [一時的な同期失敗が再送されず drift する（D4）] → `sync_pending` を記録し、次回編集・再連携・手動再同期（D9）で解消。定期ポーリングは Non-Goal。
- [refresh token 失効・ユーザーによる権限取り消し] → API 401/invalid_grant を検知したら連携を未連携相当に落とし、status で再連携を促す。
- [refresh token の暗号鍵 `GOOGLE_TOKEN_ENC_KEY` の紛失/変更] → 復号不能となり再連携が必要。鍵は 1Password で確実に管理し、ローテーション時は再連携で再取得する運用とする。
- [連携解除時のカレンダー削除は不可逆] → 削除指定時は Google 上の専用カレンダーとその全イベントが失われる。UI で削除/保持を明示選択させ（D8）、既定は保持側に寄せて誤操作を防ぐ。保持時は再連携で新カレンダーが作られ旧カレンダーが残る点はトレードオフとして受容。
- [Google API のクォータ/レイテンシがリクエスト応答に影響] → 同期は DB 確定後に行い失敗は握りつぶすため CRUD 応答の正当性には影響しない。バックフィルは件数に比例した時間がかかりうる点に留意。
- [OAuth コールバックの公開経路] → 単一ユーザー・VPN 前提だが `state` による CSRF 対策と redirect URI 固定を行う。

## Migration Plan

1. ent スキーマ追加（`GoogleConnection`, `CalendarEvent` への列追加）→ 生成 → 対応する goose マイグレーションを追加。
2. デプロイ時にマイグレーション自動適用（既存の起動時 goose.Up）。列追加・新規テーブルのみで後方互換。
3. `GOOGLE_OAUTH_*` と `GOOGLE_TOKEN_ENC_KEY` を `.env.op` に追加。未設定でも既存機能は不変（機能は休眠）。
4. ロールバック: ユーザーは連携解除、環境変数を外せば休眠。マイグレーションは down で列・テーブルを削除可能（追加のみのため安全）。
