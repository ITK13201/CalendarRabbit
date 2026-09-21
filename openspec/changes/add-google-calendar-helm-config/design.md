## Context

Google Calendar 連携（`add-google-calendar-sync`、アーカイブ済み）は backend の `internal/config/config.go` で4つの環境変数を読み込む（L108-111）。これらは `docker-compose.yml`（L49-52）と `.env.op`（L47-50）には反映済みだが、Helm チャート `charts/calendarrabbit` には未反映。既存の Helm チャートは以下の慣習を持つ:

- 非機密の設定値（モデル名・URL・プロバイダ等）は `values.yaml` の `backend.env.*` に置き、`templates/backend.yaml` で `env` の `value` として直書きする。
- 機密情報（各種 API キー・DB パスワード）は `values.yaml` の `backend.secret.*` に置き、`templates/backend-secret.yaml` の `stringData` に入れ、`templates/backend.yaml` で `secretKeyRef` として参照する。
- `backend.secret.existingSecret` が指定された場合は Secret を生成せず既存 Secret を使う。

backend の `config.Load` は Google 4変数を `os.Getenv` で読むのみで、未設定でも失敗しない（config.go L107 コメント / design D6）。連携の有効・休眠は値の有無で決まる。

See proposal.md - Why.

## Goals / Non-Goals

**Goals:**
- 既存 Helm チャートの慣習（env=非機密 / Secret=機密 / existingSecret 対応）に完全に沿って Google 連携設定を追加する。
- 既定は空文字とし、未指定の既存リリースの挙動を変えない（後方互換）。

**Non-Goals:**
- backend アプリコードの変更（既に4変数を読み込み済みのため不要）。
- Google 連携そのものの挙動変更（休眠判定・同期ロジックは対象外）。
- `frontend` / `mysql` 系の変更。
- Ingress・TLS・OAuth リダイレクトURL のドメイン設計（運用者が `--set` で指定する）。

## Decisions

### D1: 機密2 / 非機密2 の分割（AskUserQuestion で確認済み）

`GOOGLE_OAUTH_CLIENT_SECRET` と `GOOGLE_TOKEN_ENC_KEY` を `backend.secret.*` → `backend-secret.yaml` の `stringData` → `backend.yaml` の `secretKeyRef` に。`GOOGLE_OAUTH_CLIENT_ID` と `GOOGLE_OAUTH_REDIRECT_URL` を `backend.env.*` → `backend.yaml` の `env` `value` に。

- **理由**: 既存の分割方針（API キー=Secret、モデル名/URL=値）と一致。クライアントIDとリダイレクトURLは秘匿性が低い。
- **代替案**: (a) 4つすべて Secret に集約 → 非機密の REDIRECT_URL まで Secret 経由になり既存慣習から外れる。(b) CLIENT_ID も Secret に → 秘匿性は低く不要。いずれも却下。

### D2: 常に空既定でレンダリング（トグルなし、AskUserQuestion で確認済み）

env / Secret キーを条件分岐なしで常に出力し、`values.yaml` の既定値を空文字（`""`）にする。

- **理由**: backend が dormant-when-empty なので、空文字を注入するだけで「休眠」を表現できる。テンプレートが単純で、既存の他キー（`claudeApiKey` 等がダミー既定を持つ）と同じ構造。
- **代替案**: `backend.google.enabled` トグルで条件レンダリング → マニフェストは綺麗になるが、既存チャートにトグルの前例がなく一貫性を欠く。却下。
- **注意**: 既存の secret 群はダミー文字列（`dummy-*-api-key`）を既定にしているが、Google の2機密は空文字を既定とする。ダミー値だと backend が「設定済み」と誤認して連携を起動しうるため、休眠を保つには空文字が正しい。

### D3: existingSecret 経路の踏襲

`backend-secret.yaml` は `{{- if not .Values.backend.secret.existingSecret }}` ガード内にあるため、Google の2機密キーもそのガード内に追加する。運用者が `existingSecret` を使う場合は、その Secret に `GOOGLE_OAUTH_CLIENT_SECRET` / `GOOGLE_TOKEN_ENC_KEY` を含める必要がある（既存の CLAUDE_API_KEY 等と同じ前提）。

## Risks / Trade-offs

- **[空文字の機密キーが Secret に常に存在する]** → 空の値は無害で backend は休眠する。運用者は `--set backend.secret.googleOAuthClientSecret=...` 等で上書きする。既存のダミー API キーと同じ運用モデル。
- **[existingSecret 利用者が Google キーを追加し忘れる]** → 連携を使わなければ空扱いで休眠するため実害なし。連携を使う場合のみ、既存 Secret にキー追加が必要。tasks / README への明記で緩和。
- **[REDIRECT_URL の既定空で連携有効化時に未設定]** → 連携を有効化する運用者が必ず `--set` する前提。空のままでは休眠するので誤起動はしない。
