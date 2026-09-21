## 1. データモデルとマイグレーション

- [x] 1.1 ent スキーマ `GoogleConnection`（id=1 固定・`refresh_token`/`access_token`/`token_expiry`/`calendar_id`/`connected`/`created_at`/`updated_at`）を追加し、`go generate ./...` が成功して生成コードにエンティティが現れることを確認する
- [x] 1.2 `CalendarEvent` に `google_event_id`(string,default "") と `sync_pending`(bool,default false) を追加し、再生成が成功することを確認する
- [x] 1.3 上記スキーマ変更に対応する goose マイグレーションを `backend/db/migrations` に追加し、`goose up`（またはサーバ起動時適用）でテーブル/列が作成されることを確認する

## 2. 設定（環境変数）

- [x] 2.1 `internal/config` に `GoogleOAuthClientID`/`GoogleOAuthClientSecret`/`GoogleOAuthRedirectURL`/`GoogleTokenEncKey` を追加し、未設定でも `Load()` が成功する（連携は任意機能）ことを config テストで確認する
- [x] 2.2 `docker-compose.yml` と `.env.op` に `GOOGLE_OAUTH_*` と `GOOGLE_TOKEN_ENC_KEY` を追加し、`op run --env-file=.env.op -- docker compose config` で値が展開されることを確認する

## 3. Google Calendar クライアントと永続化

- [x] 3.1 `google.golang.org/api/calendar/v3` と `golang.org/x/oauth2`/`.../google` を依存に追加し、`go mod tidy` と `go build ./...` が成功することを確認する
- [x] 3.2 refresh token をアプリ層で暗号化/復号するユーティリティ（`GOOGLE_TOKEN_ENC_KEY` による AES-GCM 等）を実装し、暗号化→復号の往復と鍵不一致時の失敗をユニットテストで確認する
- [x] 3.3 `GoogleConnection` の取得・保存・破棄を行う persistence リポジトリを追加し、refresh token が暗号化列として保存され復号して取り出せることをユニットテスト（testsupport DB）で確認する
- [x] 3.4 OAuth トークンの管理（refresh token からの token source 生成・失効検知）と、専用カレンダー（名称「CalendarRabbit」）作成・削除/イベント作成・更新・削除を行う Google クライアントラッパを実装し、失効時に判別可能なエラーを返すことをテストで確認する

## 4. ミラーリングプロバイダ（一方向同期）

- [x] 4.1 `DBProvider` をラップする `MirroringProvider`（`CalendarProvider` 実装）を追加し、`_ CalendarProvider = (*MirroringProvider)(nil)` でインターフェース充足を確認する
- [x] 4.2 Create/Update/Delete で「DB 確定 → 連携有効時に Google へミラー → `google_event_id`/`sync_pending` 更新」を実装し、連携有効・無効それぞれの分岐をユニットテスト（Google クライアントのフェイク）で確認する
- [x] 4.3 同期失敗時に DB 操作は成功で返しつつ `sync_pending=true` を立て、次回操作/再連携で再送されることをテストで確認する
- [x] 4.4 日時・終日フラグ・タイムゾーンの Google イベントへのマッピング（終日=date、時刻付き=dateTime+timeZone）をテストで確認する

## 5. 連携ライフサイクル（OAuth・バックフィル・解除）

- [x] 5.1 連携確立処理（コールバックのトークン交換 → 連携状態保存）を実装し、専用 calendar_id 未保持時に専用カレンダーを1件だけ作成することをテストで確認する
- [x] 5.2 初回連携時のバックフィル（DB 既存イベント＋`sync_pending` を専用カレンダーへ作成しマッピング保存）を実装し、既存イベント複数/0件の双方をテストで確認する
- [x] 5.3 連携解除を実装し、削除パラメータで「専用カレンダーごと削除」/「カレンダーを残す」を分岐（いずれもローカル状態は破棄）、解除後は Google API を呼ばないことをテストで確認する
- [x] 5.4 未同期（sync_pending）イベントの一括再送処理（バックフィルロジックの再利用）を実装し、成功分の sync_pending 解消・未同期0件時の正常完了をテストで確認する

## 6. HTTP エンドポイントと配線

- [x] 6.1 `GET /api/google/auth`（認可URL）・`GET /api/google/callback`・`GET /api/google/status`（未同期件数を含む）・`DELETE /api/google/connection`（削除/保持パラメータ）・`POST /api/google/resync`（手動再同期）を handler に追加し、`state` による CSRF 検証を含めてハンドラテストで確認する
- [x] 6.2 未設定時（`GOOGLE_OAUTH_*` 未指定）に連携系エンドポイントが「未設定」を返し、DB のみで CRUD が動作することをハンドラテストで確認する
- [x] 6.3 `main.go` で連携有効時に `DBProvider` を `MirroringProvider` で包むよう配線し、`go build ./...` とサーバ起動が成功することを確認する
- [x] 6.4 Swagger アノテーションを追加し、`make`（swagger 生成）後に新エンドポイントが `swagger.yaml` に現れることを確認する

## 7. フロントエンド（設定画面）

- [x] 7.1 設定画面に Google Calendar 連携の状態表示（連携済み/未連携・未同期件数）と「接続」「解除」「再同期」UI を追加し、`GET /api/google/status` の結果で表示が切り替わることを確認する
- [x] 7.2 「接続」で `GET /api/google/auth` の認可URLへ遷移 → コールバック後に設定画面へ戻り連携済み表示になる導線を実装し、手動で連携→解除の往復が動作することを確認する
- [x] 7.3 「解除」UI で専用カレンダーを削除するか残すかを選択させ、選択が `DELETE /api/google/connection` のパラメータに反映されることを確認する
- [x] 7.4 「再同期」ボタンで `POST /api/google/resync` を呼び、未同期件数が更新されることを確認する

## 8. 総合検証

- [x] 8.1 `go test ./...`（backend）とフロントのテスト/ビルドが通ることを確認する
- [ ] 8.2 ローカル（`op run --env-file=.env.op -- docker compose up`）で、連携→既存イベントのバックフィル→新規作成/更新/削除が専用カレンダーに反映され、同期失敗後の手動再同期で解消し、解除（削除/保持の双方）が期待どおり動作することを実機確認する
