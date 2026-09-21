## Why

Google Calendar 連携機能（`add-google-calendar-sync`）はバックエンドの設定（`config.go`）・`docker-compose.yml`・`.env.op` には環境変数が反映されたが、Kubernetes デプロイに使う Helm チャート（`charts/calendarrabbit`）への反映が漏れている。このため Helm でデプロイした環境では Google 連携に必要な設定を注入する手段が存在せず、連携を有効化できない。

## What Changes

- Helm チャートに Google Calendar 連携用の4つの設定項目を追加する:
  - `GOOGLE_OAUTH_CLIENT_ID`（非機密 / 値として env に直書き）
  - `GOOGLE_OAUTH_REDIRECT_URL`（非機密 / 値として env に直書き）
  - `GOOGLE_OAUTH_CLIENT_SECRET`（機密 / Secret 経由）
  - `GOOGLE_TOKEN_ENC_KEY`（機密 / Secret 経由）
- `values.yaml`: `backend.env` に Google 連携の非機密設定、`backend.secret` に機密設定を追加する。いずれも既定は空文字とし、未設定なら連携は休眠する（backend 側の dormant-when-empty 挙動に合わせる）。
- `templates/backend-secret.yaml`: `GOOGLE_OAUTH_CLIENT_SECRET` と `GOOGLE_TOKEN_ENC_KEY` を `stringData` に追加する。
- `templates/backend.yaml`: 非機密2項目を `env`（`value`）として、機密2項目を `env`（`secretKeyRef`）として Deployment に追加する。
- 環境変数キーは常にレンダリングし、既定値を空文字とする（トグルは設けない）。空のときは backend 側が連携を休眠させる。

## Capabilities

### New Capabilities
（なし）

### Modified Capabilities
- `google-calendar-sync`: Google 連携に必要な設定を Kubernetes デプロイ（Helm チャート）経由で注入できること、未設定時は休眠すること、機密情報（クライアントシークレット・トークン暗号鍵）を Secret 経由で注入することを要件として追加する。

## Impact

- 影響コード（インフラ / Helm）:
  - `charts/calendarrabbit/values.yaml`: Google 連携の設定項目を追加。
  - `charts/calendarrabbit/templates/backend-secret.yaml`: 機密2キーを追加。
  - `charts/calendarrabbit/templates/backend.yaml`: env（値2 + secretKeyRef 2）を追加。
- backend アプリコードの変更は不要（`config.go` は既に4変数を読み込み済み）。
- 既存デプロイへの後方互換性: 追加項目の既定は空文字のため、既存の Helm リリースは値を指定しなければ従来どおり Google 連携なしで動作する（BREAKING ではない）。
- 秘密情報は 1Password / `--set` で本番注入する既存方針を踏襲する。
