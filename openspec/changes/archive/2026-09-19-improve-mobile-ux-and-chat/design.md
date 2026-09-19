## Context

現状（`See proposal.md - Why`）を踏まえた実装上の制約:

- フロントは React CSR。カレンダーは `react-big-calendar`（RBC）を `views={['month','agenda']}` で使用。高さは `.calendar-container`（flex, `min-height:0`）内で `style={{ height: '100%' }}` を与えている（`CalendarScreen.tsx`）。
- チャット送信は `ChatScreen.handleSend` が `await api.sendMessage` → `await reload()` の順で、reload 完了まで送信メッセージが描画されない。バックエンドは `SendMessage` 内で web_search を伴う Claude 呼び出しをブロッキング実行するため待ち時間が長い。
- バックエンドは単一会話スレッド。`buildHistory`（`send.go`）が `ListByConversation` の全メッセージを Turn 列にして毎回 Claude へ渡す。会話クリア用の API は存在しない（`router.go` の `/api/chat` に DELETE 系なし）。
- `index.html` は既に `viewport-fit=cover`。`.bottom-tabs` にセーフエリア考慮の padding は未設定（`styles.css`）。
- 予定タップは `onSelectEvent` で直接 `EventForm`（編集モード）を開いている。

## Goals / Non-Goals

**Goals:**

- モバイルでの描画・操作の破綻をなくす（カレンダー見切れ、下部タブ重なり、一覧の可読性）。
- チャットの体感応答性を上げ（即時表示）、会話のリセット手段とコンテキスト上限を用意する。
- 予定選択を「詳細（読み取り専用）→編集」の二段フローにする。

**Non-Goals:**

- UI 全面刷新・デザインシステム導入（memo2 の「モダン化」）は対象外。
- カレンダーの外部プロバイダ連携や `calendar-management`／`app-settings` の変更は対象外。
- ストリーミング応答や会話要約（summarization）による高度なコンテキスト圧縮は対象外（固定ウィンドウで足りる）。

## Decisions

### D1. チャット送信メッセージの楽観的表示

`handleSend` で送信前にローカル `messages` へ暫定ユーザーメッセージ（負値などの一時 id・`created_at=now`）を append し、`loading` 中はアシスタントの「処理中」表示を出す。`api.sendMessage` 成功後に `reload()` でサーバ正の履歴へ置換して整合を取る。失敗時は暫定メッセージを取り除き、入力内容を復元してエラー表示。

- 代替案: SSE/WebSocket ストリーミング → バックエンド改修が大きく本 change の範囲外。楽観的表示で体感課題は解消できる。

### D2. 会話クリア（API＋UI）

バックエンドに会話クリア用エンドポイントを追加する: `DELETE /api/chat/conversations`。単一スレッド前提なので id は不要。usecase に `ClearConversation` を追加し、スレッドにひも付く `event_proposal`→`message` の順（外部参照の依存順）で削除する。会話レコード自体は残しても消してもよいが、`GetOrCreate` で再生成されるため「メッセージ・予定案の全削除」を正とする。

フロントは `api.clearConversation()` を追加し、チャット画面ヘッダーにクリアボタンを置く。押下時は誤操作防止のため確認ダイアログ（「本当に削除しますか？」相当）を表示し、承諾された場合のみ API を呼びローカル状態（messages/proposals）を空にする。

- 代替案: `POST /api/chat/conversations/clear` → 破棄セマンティクスは DELETE が自然。既存の GET `/conversations` と同一パスに DELETE を追加する。

### D3. Claude へ渡す履歴の上限（固定ウィンドウ）

`buildHistory` の結果を直近 N ターンに切り詰める（末尾 N 件）。N は定数で定義（マジックナンバー禁止のため名前付き定数、初期値 `10`）。永続化履歴自体は保持し、送信範囲のみ制限する。

- 代替案: トークン数ベースの動的打ち切りや要約 → 実装コスト大。件数固定で「際限なく増える」問題は解消できる。将来必要になれば置換可能。

### D4. カレンダーのモバイル見切れ修正

原因は、flex コンテナ内で RBC 月表示が高さ計算する際、ナビゲーション遷移時に高さ再計算が正しく走らず、行の絶対配置がビューポート外へはみ出すこと。対策として:

1. `.calendar-container` に決定的な高さを与える（flex で確保した領域を `height` として RBC に渡す。`min-height:0` と併せ、コンテナ実高に追従させる）。
2. それでも遷移時の再計算漏れが残る場合の保険として、RBC を `key={`${view}-${startOfMonth(current)}`}` で期間切替時に再マウントし、レイアウト計算をやり直させる。

報告された再現環境は**デスクトップの狭い幅（レスポンシブ）**（iOS Safari / Android Chrome は未確認）。したがって主な再現・検証対象はブラウザ幅を狭めたデスクトップとし、モバイル実機/エミュレータは保険として確認する。実装時に「今日」「前」「次」の各遷移を確認し、1 だけで解消するなら 2 は入れない（`Risks` 参照）。

- 代替案: 固定 px 高さのハードコード → 端末差で破綻するため不可（プログラミング規約のハードコード回避にも反する）。

### D5. ナビゲーションラベル「<」「>」

RBC の `messages.previous`/`messages.next` を `'<'` / `'>'` に変更するのみ（`today` は「今日」を維持）。

### D6. 初期表示とタブ順序

`App.tsx` の初期 `tab` を `'calendar'` に変更。`BottomTabs.TABS` の並びを `calendar → chat → settings` に変更（`TabKey` 型は据え置き）。

### D7. 一覧（agenda）表示のモバイル対応

RBC の agenda はテーブル描画。モバイル幅（`max-width` メディアクエリ）で agenda テーブルをカード風の縦積みに CSS で再スタイルする（`display:block` 化＋各セルにラベル付け）。RBC のコンポーネント差し替えは行わず CSS のみで対応し、影響範囲を最小化する。

- 代替案: `components={{ agenda: ... }}` で独自レンダラ → 実装量が増える。まず CSS で十分。

### D8. 下部タブのセーフエリア対応

`.bottom-tabs` に `padding-bottom: env(safe-area-inset-bottom)` を追加し、高さは `calc(var(--tabbar-height) + env(safe-area-inset-bottom))` 相当で確保。`viewport-fit=cover` は設定済みのため CSS 変更のみ。

### D9. 予定の詳細→編集フロー

予定タップ（`onSelectEvent`）は編集フォームではなく、新規の読み取り専用「予定詳細」表示（`EventDetail` コンポーネント, モーダル）を開く。詳細には名称・日時・場所・概要・情報源を表示し、「編集」「削除」「閉じる」ボタンを置く。「編集」で既存の `EventForm`（編集モード）へ遷移し、「削除」は詳細から直接（確認のうえ）削除できる。状態は `viewing`（詳細対象）と `editing` を分離して管理する。編集フォーム内の削除ボタンは維持してもよいが、削除導線は詳細画面にも用意する。

## Risks / Trade-offs

- [D1: 楽観的表示とサーバ履歴の二重管理で表示がちらつく/重複する] → 一時 id を負値等で明確に分け、`reload()` 成功時にサーバ履歴で全置換して重複を防ぐ。失敗時はロールバック。
- [D4: 見切れの真因が環境依存で、CSS 高さ調整だけでは再発し得る] → 実機確認を必須化し、再マウント（key）を保険として用意。タスクに検証手順を含める。
- [D2: クリアの削除順序を誤ると外部キー制約違反] → `event_proposal`→`message` の依存順で削除。トランザクション内で実施。
- [D3: 履歴打ち切りで直前の重要な文脈が欠落し得る] → N=10 ターンは通常の予定登録対話には十分。将来はトークン基準へ置換可能な形（名前付き定数）にしておく。
- [D7: CSS のみの agenda 再スタイルは RBC のマークアップ変更に脆い] → RBC バージョン固定運用で許容。破綻時は D7 代替（独自レンダラ）へ移行。
