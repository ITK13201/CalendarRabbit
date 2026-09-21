## 1. values.yaml への設定追加

- [x] 1.1 `charts/calendarrabbit/values.yaml` の `backend.env` に `googleOAuthClientID: ""` と `googleOAuthRedirectURL: ""` を追加し、用途をコメントで明記する。`helm template` でエラーが出ないことを確認する
- [x] 1.2 `charts/calendarrabbit/values.yaml` の `backend.secret` に `googleOAuthClientSecret: ""` と `googleTokenEncKey: ""` を追加する（既定は空文字。休眠のためダミー値にしない旨をコメントで明記）

## 2. Secret テンプレートへの反映

- [x] 2.1 `charts/calendarrabbit/templates/backend-secret.yaml` の `stringData` に `GOOGLE_OAUTH_CLIENT_SECRET` と `GOOGLE_TOKEN_ENC_KEY` を追加する（既存の `existingSecret` ガード内に配置）。`helm template` の出力で両キーが `stringData` に現れることを確認する

## 3. Deployment テンプレートへの反映

- [x] 3.1 `charts/calendarrabbit/templates/backend.yaml` の `env` に非機密2項目（`GOOGLE_OAUTH_CLIENT_ID` / `GOOGLE_OAUTH_REDIRECT_URL`）を `value` として追加する
- [x] 3.2 `charts/calendarrabbit/templates/backend.yaml` の `env` に機密2項目（`GOOGLE_OAUTH_CLIENT_SECRET` / `GOOGLE_TOKEN_ENC_KEY`）を `secretKeyRef` として追加する。`helm template` の出力で機密値が平文で現れず secretKeyRef 参照になっていることを確認する

## 4. 検証

- [x] 4.1 値未指定で `helm template charts/calendarrabbit` を実行し、レンダリングが成功して Google 系 env が空文字で出力される（デプロイ可能）ことを確認する
- [x] 4.2 `--set backend.env.googleOAuthClientID=... --set backend.env.googleOAuthRedirectURL=... --set backend.secret.googleOAuthClientSecret=... --set backend.secret.googleTokenEncKey=...` で `helm template` を実行し、非機密は env の value に、機密は Secret の stringData と backend の secretKeyRef に反映されることを確認する
- [x] 4.3 `helm lint charts/calendarrabbit` が通ることを確認する
