## Why

予定の登録は「イベント名を思い出す → 開催日・場所を調べる → カレンダーに手入力する」という手間が多く、特にTGSのような一般公開イベントは情報を検索する手間がかかる。チャットで「TGSの予定を追加して」と伝えるだけで、裏側でイベント情報を検索し、内容を確認して承認するだけでカレンダーに登録できる体験を、個人用（VPN内・認証なし）のPWAとして提供する。CalendarRabbitはその初期構築（MVP）を行う。

## What Changes

- **チャット駆動の予定登録フロー**を新設する。ユーザーの自然言語メッセージをClaude API（`web_search`ツール併用）で解釈し、イベント候補（名称・開催日時・場所・概要）を検索・抽出して**予定案（proposal）**として提示。ユーザーが承認して初めてカレンダーに登録する（承認前は登録しない）。
- **チャット会話履歴・予定案・承認状態をDB（MySQL）に永続化**し、後から参照できるようにする。
- **DB管理のカレンダー機能**を新設する。予定（イベント）のCRUDと、月表示・一覧表示向けの取得APIを提供する（単一ユーザー・単一カレンダー前提）。
- **設定機能**を新設する。タイムゾーンやチャット/検索の挙動などアプリ設定を保存・取得できる。
- **React（CSR）製PWAフロントエンド**を新設する。画面下部タブで「チャット」「カレンダー」「設定」の3画面を切替。PWA（モバイル/デスクトップ）としてホーム画面に追加可能。
- **Go（gin）バックエンド**をクリーンアーキテクチャ（MoneyRabbit準拠の粒度）で新設。ent（ORM）+ MySQL、RESTful API、structured logging（JSON・requestId追跡・service層は全ログ出力・extra fieldでcontext付与）、swaggerでAPI仕様書を自動生成。認証なし（Tailscale等のVPN前提）。
- **インフラ**を整備する。ローカルはdocker-compose、本番はk8s（Helm chart）。CI/CDはGitHub ActionsでテストとdockerイメージのGHCR push、Helm chartでマニフェスト管理。TDDで実装。
- 将来拡張として**Google Calendar API連携**を見据えた設計にする（本changeでは実装しない）。

## Capabilities

### New Capabilities
- `chat-scheduling`: チャットメッセージの解釈、Claude+Web検索によるイベント検索、予定案の生成・提示、ユーザー承認、承認後のカレンダー登録、会話履歴・予定案の永続化。
- `calendar-management`: 予定（イベント）のDB永続化とCRUD、月表示・一覧表示向けの取得。単一ユーザー・単一カレンダーのスコープ。
- `app-settings`: アプリ設定（タイムゾーン等）の取得・更新と永続化。
- `pwa-web-app`: React CSRのPWAシェル。下部タブによる3画面（チャット/カレンダー/設定）ナビゲーション、インストール（ホーム画面に追加）対応。

### Modified Capabilities
<!-- 既存specなし。新規プロジェクトのため変更対象の既存capabilityはない。 -->

## Impact

- **新規コードベース**（現状はopenspecのみ）。以下を新設:
  - `backend/`: Go + gin + ent、クリーンアーキテクチャ、swagger、structured logging。
  - `frontend/`: React + pnpm + Vite（想定）+ PWA。
  - `deploy/`: docker-compose（ローカル）、Helm chart（k8s）。
  - `.github/workflows/`: CI/CD（テスト、GHCRへのdocker push、Helmによるデプロイ）。
- **外部依存**: Claude API（`web_search`ツール利用、APIキー必須）、MySQL。
- **APIコントラクト**: RESTful APIを新規公開（チャット、予定案の承認、カレンダーCRUD、設定）。swaggerで仕様公開。
- **将来影響**: カレンダー永続化層は将来のGoogle Calendar API連携を見据え、抽象化（provider差し替え可能）を意識した設計とする。
