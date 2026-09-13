## 1. リポジトリscaffold・共通基盤

- [x] 1.1 `backend/` `frontend/` `charts/calendarrabbit/` `.github/workflows/` の雛形を作成し、`tree`で想定ディレクトリが存在することを確認する
- [x] 1.2 backendのGoモジュール初期化（gin, ent, slog, swag, goose, Atlas依存を追加）し、`go build ./...` が通ることを確認する
- [x] 1.3 `internal/config` で環境変数（DB接続、CLAUDE_API_KEY、CLAUDEモデル、ポート）を読み込み、必須欠如時にエラーとなる単体テストが通ることを確認する
- [x] 1.4 `docker-compose.yml`（mysql/backend/frontend）を作成し、`docker compose up mysql` でMySQLが起動・疎通できることを確認する

## 2. ミドルウェア・ロギング（横断）

- [x] 2.1 `middleware/requestid.go` を実装し、requestIdがcontextに入りレスポンスヘッダに付与されることをテストで確認する
- [x] 2.2 slog(JSON)ベースのロガーと `middleware/logger.go`（アクセスログ）を実装し、出力がJSONでrequestIdを含むことをテストで確認する
- [x] 2.3 service層からcontextのrequestIdを取り出しextra field付きで構造化ログを出すヘルパを実装し、単体テストで確認する
- [x] 2.4 `middleware/cors.go` を実装し、想定オリジンでCORSヘッダが返ることをテストで確認する

## 3. データモデル（ent / マイグレーション）

- [x] 3.1 entスキーマ `Conversation` `Message` `EventProposal` `CalendarEvent` `AppSetting` を定義し `go generate ./ent` で生成物が作られることを確認する（design.md D4）
- [x] 3.2 Atlas設定とgooseマイグレーションを作成し、docker-composeのMySQLに対しmigrate up/downが成功することを確認する
- [x] 3.3 `service/persistence` で各エンティティのCRUDを実装し、testcontainers/ローカルMySQLに対する永続化テストが通ることを確認する

## 4. calendar-management（カレンダー）

- [x] 4.1 `service/calendarprovider` に `CalendarProvider` interfaceとDB実装を実装し、DB実装の単体テストが通ることを確認する（design.md D5）
- [x] 4.2 `usecase/calendar`（create/update/delete/get/list）を実装し、必須項目欠如・終了<開始のバリデーションエラーがテストで再現されることを確認する（spec: calendar-management）
- [x] 4.3 期間取得（from/to重なり判定、複数日イベント包含）を実装し、境界ケースのテストが通ることを確認する（spec: 期間指定によるイベント取得）
- [x] 4.4 `handler/calendar.go` とルーティングを実装し、events CRUD + 期間取得のハンドラテストが通ることを確認する

## 5. app-settings（設定）

- [x] 5.1 `usecase/settings`（get/update、未初期化時は既定TZ=Asia/Tokyo、無効TZ拒否）を実装し、単体テストが通ることを確認する（spec: app-settings）
- [x] 5.2 `handler/settings.go`（GET/PUT）を実装し、更新値が後続GETに反映されるハンドラテストが通ることを確認する

## 6. chat-scheduling（チャット・承認フロー）

- [x] 6.1 `service/chat/claude.go` を実装：予定登録特化のシステムプロンプト＋既定モデル`claude-sonnet-4-6`（設定で切替可）で、Claude Messages API + `web_search`ツールによりイベント（名称/開始/終了/場所/概要/情報源URL、複数候補・特定不能フラグ）を構造化抽出する。Claudeクライアントをモック化した単体テストが通ることを確認する（design.md D3）
- [x] 6.2 `usecase/chat` のメッセージ送信＋会話永続化（単一連続スレッドの初期化/追記/履歴取得、予定登録の役割外メッセージへの案内）を実装し、テストが通ることを確認する（spec: チャットメッセージの受付と会話の永続化）
- [x] 6.3 予定案生成（pending状態で保存、特定不能時は追加情報要求、複数候補時は候補提示）を実装し、各分岐のテストが通ることを確認する（spec: Claudeとweb検索によるイベント特定）
- [x] 6.4 承認usecaseを実装：pendingのみ受理、編集後内容を反映してCalendarProvider経由でイベント作成、proposalをapprovedに更新、処理済みは再承認不可（トランザクション）。テストが通ることを確認する（spec: 予定案のユーザー確認と承認フロー、design.md D2）
- [x] 6.5 却下usecase（rejectedに更新・未登録）を実装し、テストが通ることを確認する
- [x] 6.6 `handler/chat.go`（messages送信、conversations取得、proposals approve/reject）とルーティングを実装し、「承認前は登録されない／承認で登録される」統合的ハンドラテストが通ることを確認する

## 7. API仕様（swagger）

- [x] 7.1 全ハンドラにswaggoアノテーションを付与し `swag init` で `docs/swagger` が生成され、`/swagger` でUIが表示されることを確認する

## 8. フロントエンド（React CSR + PWA）

- [x] 8.1 Vite + React + TypeScript + pnpm プロジェクトを初期化し、カレンダーUIライブラリ（FullCalendar/react-big-calendar）を導入、`/api/*` のvite proxy設定と `pnpm build` の成功を確認する
- [x] 8.2 下部タブ（Chat/Calendar/Settings）ナビゲーションを実装し、タブ切替とアクティブ表示・初期表示=Chatを確認する（spec: 下部タブによる3画面ナビゲーション）
- [x] 8.3 チャット画面（メッセージ送受信・履歴表示・予定案の承認/却下UI、応答待ちのローディング表示）を実装し、送信〜承認完了までの表示を確認する（spec: チャット画面）
- [x] 8.4 カレンダー画面をライブラリで実装：当月イベント取得・月/一覧表示・詳細表示に加え、手動での作成/編集/削除UIを実装し、各操作が表示に反映されることを確認する（spec: カレンダー画面）
- [x] 8.5 設定画面（タイムゾーン表示・変更・保存）を実装し、変更が反映されることを確認する（spec: 設定画面）
- [x] 8.6 `vite-plugin-pwa` でmanifest + Service Worker（アプリシェルprecache）を構成し、Lighthouse/DevToolsでインストール可能・オフライン表示を確認する（spec: PWAとしてのインストール対応）

## 9. インフラ / CI-CD

- [x] 9.1 backend/frontend の Dockerfile を作成し、`docker compose up` で全サービスが起動しフロントからAPIへ疎通できることを確認する
- [x] 9.2 `charts/calendarrabbit`（backend/frontend/mysqlのdeployment/service/secret、frontend configmap）を作成し `helm template`/`helm lint` が通ることを確認する（ローカルk8s起動確認は不要）
- [x] 9.3 `.github/workflows/build.yml`（lint + test + docker build & GHCR push）を作成し、CIでテストが実行されることを確認する
- [x] 9.4 `.github/workflows/helm-release.yml`（Helm chartのpackage/release）を作成し、ワークフローが成功することを確認する（クラスタへの`helm upgrade`はCDスコープ外＝手動/ArgoCD等）

## 10. 総合検証

- [x] 10.1 docker-compose環境でE2Eシナリオ「『TGSの予定を追加して』→ 予定案提示 → 承認 → カレンダー画面に表示」が通ることを手動確認する
- [x] 10.2 `go test ./...` と frontendのテストがすべてグリーンであることを確認する
