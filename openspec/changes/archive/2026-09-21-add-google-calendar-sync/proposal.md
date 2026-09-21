## Why

現在 CalendarRabbit のイベントは自前 DB（MySQL）にのみ保存され、ユーザーが普段使う Google Calendar からは見えない。ユーザーが CalendarRabbit で登録・管理した予定を、追加のアプリを開かずに手持ちの Google Calendar 上で確認できるようにしたい。

## What Changes

- Google Calendar 連携（OAuth 2.0 ユーザー同意フロー）を追加する。ユーザーは自分の Google アカウントを一度連携し、以後は保存された refresh token でバックグラウンド同期する。
- 連携時に Google Calendar 上へ CalendarRabbit 専用カレンダーを1つ自動作成し、その calendarId を保持する。既存の他カレンダーには一切書き込まない。
- 同期方向は **一方向（CalendarRabbit → Google）**。DB を正（source of truth）とし、Google 専用カレンダーはそのミラーとして扱う。Google 側で直接行われた編集は取り込まない。
- イベントの作成・更新・削除時に、対応する Google イベントへ**即時ミラー同期**する。CalendarRabbit の各イベントに対応する Google イベントID（マッピング）を保持する。
- **初回連携時のバックフィル**: 連携完了時点で既に DB にあるイベントをすべて Google 専用カレンダーに作成する。
- 連携解除（disconnect）を提供する。保存トークン・calendarId・イベントマッピングを破棄する。解除時に Google 側の専用カレンダーを削除するか残すかをユーザーが選択できる。
- 同期失敗時も DB 操作は成功させ、失敗したイベントは未同期として記録し後続操作で再送する（DB を正とするため）。未同期イベントを一括再送する手動再同期も提供する。
- refresh token は DB 保存時にアプリ層で暗号化する（平文保存しない）。専用カレンダー名は「CalendarRabbit」固定。

## Capabilities

### New Capabilities
- `google-calendar-sync`: Google アカウントの OAuth 連携、専用カレンダーの作成、DB イベントの Google への一方向ミラー同期（即時同期・初回バックフィル・連携解除・同期失敗時の再送）を扱う。

### Modified Capabilities
- `calendar-management`: イベントの作成・更新・削除が、Google 連携が有効な場合に専用カレンダーへの即時ミラー同期を伴うよう振る舞いを拡張する（未連携時は従来どおり DB のみで完結）。

## Impact

- 影響コード（backend/Go）:
  - `internal/service/calendarprovider`: 既存 `CalendarProvider` 抽象の背後に Google ミラーを組み合わせる（DB 主 + ミラー）。
  - `internal/usecase/calendar`: CRUD 後のミラー同期の起動点。
  - 新規: Google OAuth・Google Calendar API クライアント、連携状態とイベントマッピングの永続化（ent スキーマ追加）。
  - `internal/handler`: OAuth 開始・コールバック・連携状態取得・連携解除の HTTP エンドポイント。
  - `internal/config`: Google OAuth クライアントID/シークレット・リダイレクトURI・スコープの設定追加。
- 影響コード（frontend）: 設定画面に「Google Calendar 連携」の接続/解除 UI と状態表示を追加。
- 依存関係: Google API 用ライブラリ（`google.golang.org/api/calendar/v3`, `golang.org/x/oauth2`）。
- 設定/秘密情報: Google OAuth の client_id / client_secret と refresh token 暗号鍵（`GOOGLE_TOKEN_ENC_KEY`）を環境変数（1Password 管理、`.env.op`）で注入。refresh token 等の連携状態は暗号化して DB に保存。
- DB: マイグレーション追加（連携状態テーブル・イベント↔Google イベントIDマッピング）。
