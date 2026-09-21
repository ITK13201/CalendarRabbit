## ADDED Requirements

### Requirement: Kubernetes デプロイでの Google 連携設定の注入

システムは Google Calendar 連携に必要な設定（OAuth クライアントID・OAuth クライアントシークレット・OAuth リダイレクトURL・トークン暗号鍵）を、Kubernetes デプロイ（Helm チャート）経由でバックエンドへ注入できなければならない（SHALL）。機密情報であるクライアントシークレットとトークン暗号鍵は Kubernetes Secret 経由で注入しなければならず（SHALL）、Deployment のマニフェストに平文で埋め込んではならない（MUST NOT）。非機密であるクライアントIDとリダイレクトURLは通常の設定値として注入してよい（MAY）。これらの設定が未指定（空）の場合、デプロイは成功し、Google 連携は休眠したまま既存のカレンダー機能が動作しなければならない（SHALL）。

#### Scenario: Helm 経由で Google 連携設定を注入して有効化する

- **WHEN** Helm リリースで Google 連携の4設定（クライアントID・クライアントシークレット・リダイレクトURL・トークン暗号鍵）に値を指定してデプロイする
- **THEN** バックエンドのコンテナに対応する環境変数が注入され、クライアントシークレットとトークン暗号鍵は Secret 経由で供給される

#### Scenario: Google 連携設定を指定せずにデプロイする

- **WHEN** Google 連携の設定を指定せずに Helm でデプロイする
- **THEN** デプロイは成功し、Google 連携は休眠したまま DB のみでカレンダー機能が動作する

#### Scenario: 機密情報は Secret 経由で供給される

- **WHEN** クライアントシークレットとトークン暗号鍵を指定してデプロイする
- **THEN** これらの値は Kubernetes Secret から参照され、Deployment マニフェストに平文では現れない
