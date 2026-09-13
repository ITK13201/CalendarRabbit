## Context

新規プロジェクトの初期構築。動機・スコープは proposal.md を参照。確定済み前提（ユーザー確認済み）:

- イベント検索元 = **Claude API + `web_search`ツール**
- 会話履歴 = **DBに永続化**
- スコープ = **単一ユーザー・単一カレンダー**（認証なし、Tailscale等VPN前提）

参考実装 MoneyRabbit（https://github.com/ITK13201/MoneyRabbit）のクリーンアーキテクチャ粒度に合わせる。MoneyRabbitの実構成:

- `backend/internal/domain/entity/` … ドメインエンティティ
- `backend/internal/usecase/<feature>/` … ユースケース層（feature単位、`usecase.go` + 操作別ファイル）
- `backend/internal/service/<concern>/` … サービス層（`persistence/`, `classifier/claude.go` 等の関心ごと単位）
- `backend/internal/handler/` … ginハンドラ + `router.go`
- `backend/internal/middleware/` … `cors.go`, `logger.go`, `requestid.go`
- `backend/cmd/server/` … エントリポイント
- `backend/ent/` … entスキーマ・生成コード、`db/`（Atlas/goose）、`docs/swagger/`
- `charts/<app>/` … Helm chart、`.github/workflows/`（build.yml, helm-release.yml）
- ORM=ent、スキーマ管理=Atlas、マイグレーション=goose、DB=MySQL、frontend=React19+Vite（`/api/*`をproxy）

CalendarRabbitはこの構成を踏襲する（DBはMySQLを採用）。

## Goals / Non-Goals

**Goals:**

- MoneyRabbit準拠のクリーンアーキテクチャ（domain / usecase / service / handler / middleware）でバックエンドを構築する。
- チャット→検索→予定案→承認→登録のフローを、承認をゲートとしてサーバ側状態機械（pending→approved/rejected）で表現する。
- service層の全ログ出力・requestId追跡・JSON structured logging・extra fieldでのcontext付与を横断的に実現する。
- カレンダー永続化を将来のGoogle Calendar連携に備えてprovider抽象（interface）の背後に置く。
- TDDで進められるよう、usecase/serviceをinterface注入で単体テスト可能にする。
- swaggerでAPI仕様を自動生成、docker-compose（ローカル）/ Helm（k8s）/ GitHub Actions（GHCR + Helm release）を用意。

**Non-Goals:**

- Google Calendar API連携の実装（抽象境界のみ用意、実装は本changeでは行わない）。
- 認証・認可・マルチユーザー・複数カレンダー（VPN前提の単一ユーザー・単一カレンダー）。
- リアルタイム配信（WebSocket/SSE）。チャットは同期RESTで完結させる。
- 繰り返し予定（RRULE等の反復ルール）。本changeは単発予定のみ扱う。
- 複数会話スレッド・汎用チャット。チャットは予定登録特化の単一連続スレッド。
- CDによるk8sクラスタへの自動適用。CDはbuild/test・GHCR push・Helm chart releaseまで（クラスタへの`helm upgrade`は手動またはArgoCD等で別途）。
- ローカルでのk8s起動確認（ローカルはdocker-composeのみ）。

## Decisions

### D1. バックエンドのレイヤ構成（MoneyRabbit準拠）

```
backend/
  cmd/server/main.go
  ent/                      # entスキーマ + 生成コード
  db/                       # Atlas設定 + gooseマイグレーション
  docs/swagger/             # swag生成物
  internal/
    domain/entity/          # Conversation, Message, EventProposal, CalendarEvent, AppSetting
    usecase/
      chat/                 # send_message, generate_proposal, approve, reject, list
      calendar/             # create, update, delete, list, get
      settings/             # get, update
    service/
      persistence/          # ent経由の永続化（conversation, message, proposal, event, setting）
      chat/claude.go        # Claude API + web_searchツール呼び出し（イベント抽出）
      calendarprovider/     # CalendarProvider interface + db実装（将来 google 実装追加）
    handler/                # chat.go, calendar.go, settings.go, router.go, health.go
    middleware/             # cors.go, logger.go, requestid.go
    config/                 # env読み込み
```

理由: MoneyRabbitと同じ「usecaseはfeature単位、serviceはconcern単位、handlerはフラット」の粒度に合わせることで、参照実装からの移植・レビューが容易。代替案（DDD重量級/ヘキサゴナルの厳密実装）はMVP規模に対して過剰。

### D2. 承認フローを状態機械で表現

`EventProposal` に `status`（`pending` / `approved` / `rejected`）を持たせ、承認APIは `pending` のみ受理する。承認usecaseは「proposal取得→status検証→CalendarEvent作成→proposalをapprovedに更新」をトランザクション内で行い、二重登録を防ぐ（spec: chat-scheduling「処理済みの予定案は再承認できない」）。承認時にユーザー編集後の内容を受け取れるよう、承認リクエストは編集済みフィールドを任意で受け付ける。

代替案: proposalを持たずチャット応答内に即登録 → 「承認前は登録されない」要件を満たせないため却下。

### D3. Claude連携（`service/chat/claude.go`）

Claude Messages API に `web_search` ツールを与え、システムプロンプトで「予定登録に特化したアシスタントとして振る舞い、イベントの名称・開始/終了日時・場所・概要・情報源URLを構造化して返す」よう指示（予定登録と無関係な入力にはその旨を案内し予定案を出さない）。抽出結果はTool useではなく最終的に構造化JSON（アプリ定義のスキーマ）として受け取り、パースして `EventProposal` にマップする。候補が複数/特定不能な場合はその旨をフラグで返し、handlerがそれに応じた応答を返す（spec: 候補複数の確認 / 特定不能）。APIキーは環境変数、Claudeモデルは既定 `claude-sonnet-4-6`（速度・コスト・知能のバランスが良くイベント抽出に十分）とし、設定で切替可能にする。

理由: web検索の鮮度要件（最新イベント日程）を満たすため、モデル知識のみに頼らずweb_searchを使う。プロンプトキャッシュを有効化してシステムプロンプト/ツール定義のトークンを再利用する。

### D4. データモデル（ent）

- `Conversation`(id, created_at, updated_at) … 単一連続スレッドのため実質1レコード（存在しなければ初回に生成）
- `Message`(id, conversation_id→FK, role[user|assistant], content, created_at)
- `EventProposal`(id, conversation_id→FK, message_id→FK, title, starts_at, ends_at, location, description, source_url, all_day, status[pending|approved|rejected], calendar_event_id nullable, created_at, updated_at)
- `CalendarEvent`(id, title, starts_at(UTC), ends_at(UTC), all_day, location, description, source_url, created_at, updated_at)
- `AppSetting`(id=1 固定の単一レコード, timezone, updated_at)

日時はDBにUTCで保存し、表示は `AppSetting.timezone`（既定 Asia/Tokyo）で解釈（spec: calendar-management/app-settings）。スキーマ管理はAtlas、マイグレーションはgoose。

### D5. カレンダーprovider抽象

`service/calendarprovider` に `CalendarProvider` interface（Create/Update/Delete/List/Get）を定義し、DB実装を提供。usecase/calendar と 承認usecase はこのinterface経由で登録する。将来 `google` 実装を追加してもusecaseを変更不要にする（spec: 将来プロバイダ連携を見据えた抽象化）。

### D6. ロギング（横断）

`middleware/requestid.go` でrequestIdを採番しcontextに格納、`middleware/logger.go` でJSONアクセスログ。service層は全メソッドでcontextからrequestIdを取り出し構造化ログを出力し、ロジック分岐等のcontext情報はextra field（key-value）で付与する。標準 `log/slog`（JSON handler）を採用。

理由: 追加依存を最小化しつつ構造化・context付与が可能。代替のzap/zerologも可だが、slogで要件を満たせる。

### D7. API（RESTful）とswagger

- `POST /api/chat/messages`（メッセージ送信、必要に応じ proposal を含む応答）
- `GET /api/chat/conversations` / `GET /api/chat/conversations/{id}`（履歴）
- `POST /api/chat/proposals/{id}/approve` / `POST /api/chat/proposals/{id}/reject`
- `GET /api/calendar/events`（`from`,`to`クエリで期間取得）/ `POST` / `GET/PUT/DELETE /api/calendar/events/{id}`
- `GET /api/settings` / `PUT /api/settings`
- `GET /api/health`

swag（swaggoアノテーション）で `docs/swagger` を生成。

### D8. フロントエンド（React CSR + PWA）

Vite + React + TypeScript + pnpm。下部タブで Chat / Calendar / Settings を切替（spec: pwa-web-app）。カレンダー画面は月表示/一覧のためにカレンダーUIライブラリ（FullCalendar または react-big-calendar）を採用し、画面から予定の手動作成・編集・削除も行える（既存のcalendar CRUD APIを利用）。チャット応答は同期REST（1リクエストで完結）とし、web検索で数秒かかる場合はローディング表示で対応（SSEストリーミングは非採用）。PWAは `vite-plugin-pwa` でmanifest + Service Worker（アプリシェルのprecache）を生成し「ホーム画面に追加」に対応。`/api/*` はvite proxy（開発）/ Nginx（本番）でバックエンドへ。型定義はswaggerからのクライアント生成を利用可能にする。

### D9. インフラ / CI-CD

- ローカル: `docker-compose.yml`（backend, frontend, mysql）。
- 本番: `charts/calendarrabbit`（backend/frontend/mysql の deployment/service/secret、frontend configmap）。
- CI/CD: `.github/workflows/build.yml`（lint + test + docker build & GHCR push）、`helm-release.yml`（Helm chartのpackage/release）。TDDに合わせCIでテスト必須。クラスタへの適用（`helm upgrade`）はCDのスコープ外とし、手動またはArgoCD等で行う。

## Risks / Trade-offs

- **[web_search結果の不正確さ]** 日程・場所が誤ることがある → 承認フローを必須ゲートにし、ユーザーが編集して承認できるようにする（誤情報のまま登録されない）。情報源URLを予定案に含め検証可能にする。
- **[Claude API依存・レイテンシ/コスト]** チャット応答が遅く/高コストになりうる → プロンプトキャッシュ活用、web_search回数の上限設定、タイムアウトとエラー時のフォールバック応答を用意。
- **[認証なし]** VPN前提のため公開ネットワークに晒すと危険 → デプロイ手順でVPN/内部ネットワーク限定を明記。Ingressは内部限定。
- **[タイムゾーン/複数日イベント]** UTC保存とTZ表示の不整合、期間重なり判定の漏れ → 保存はUTC統一、期間取得は「開始<to かつ 終了>from」の重なり条件で実装しテストで担保。
- **[ent + Atlas + goose の三点管理]** スキーマ変更手順が煩雑 → MoneyRabbitの手順を踏襲しREADME/Makefileに明文化。

## Migration Plan

新規構築のためデータ移行なし。デプロイ順:

1. リポジトリscaffold（backend/frontend/charts/.github）。
2. ローカル docker-compose で疎通（mysql起動→goose migrate→backend→frontend）。
3. GitHub Actions で GHCR にイメージpush、Helm chart release。
4. k8s（VPN内）へ Helm でデプロイ、Secret（Claude APIキー、DB認証情報）を投入。

ロールバック: Helmリリースのrollback、DBはgooseのdownマイグレーションで対応。

## Open Questions

- 既定Claudeモデルは `claude-sonnet-4-6`（設定で切替可能）。web_search利用回数の上限は運用しながら調整（specや構成には影響しないため後決めで可）。
- 予定案のリマインダ/通知（PWA push）は将来検討（本changeのspec範囲外）。
