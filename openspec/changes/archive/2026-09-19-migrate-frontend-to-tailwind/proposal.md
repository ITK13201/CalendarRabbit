## Why

フロントエンドのスタイルは現在 `src/styles.css`（約720行）に手書きのCSS変数・BEM風クラスとして集約されており、コンポーネントとスタイルが分離しているため変更時の見通しが悪い。ユーティリティファースト（Tailwind CSS）へ移行することで、スタイルをマークアップに近接させ、デザイントークンを一元管理し、今後のUI改修を高速化する。

## What Changes

- ビルド構成に Tailwind CSS v4 を導入する（`tailwindcss` + `@tailwindcss/vite` プラグイン、CSS-first 構成）。
- 既存の `:root` CSS変数（カラー・角丸・影・トランジション等）を Tailwind の `@theme` トークンとして移植し、デザインの一貫性を保つ。
- アクセントカラーをモダンな violet 系（`#7c3aed`）へ刷新し、PWA マニフェストの `theme_color` も合わせて更新する。
- 再利用されるスタイル（ボタン・入力・モーダル・バブル等）は Tailwind の `@layer components` でセマンティッククラスとして定義し、`className` の重複を避ける。
- 手書きクラスで書かれている以下の要素を、コンポーネント内の Tailwind ユーティリティクラスへ置き換える:
  - アプリシェル（`.app` / `.app-content` / `.screen` / `.screen-header`）
  - 下部タブ（`BottomTabs`）
  - ボタン（`.btn` 系 primary / secondary / danger）
  - チャット画面（バブル・入力欄・予定案カード・通知/エラー）
  - フォームとモーダル（`EventForm` / `EventDetail` / `.field` / `.modal` 系）
  - 設定画面
- **移行しない箇所（意図的なスコープ外）**: `react-big-calendar` のDOM内部を狙うオーバーライド（`.rbc-*`）と、その関連メディアクエリ（一覧のカード化）は、Tailwindでは表現しづらいため最小限のプレーンCSSとして残す。ただし色・角丸などは `@theme` トークン（CSS変数）と連動させる。
- 元デザインの完全再現は目標としない。トークンを引き継ぎつつ、視覚的に同等〜近い水準を許容する（**BREAKING** なし・機能不変）。
- 移植完了後、Tailwind化された分の記述を `styles.css` から削除し、残すのは RBC オーバーライドと最小のグローバル定義（`@import`、`@theme`、`html/body` リセット、`prefers-reduced-motion`）のみとする。

## Capabilities

これはスタイリング基盤（ツール）の移行であり、アプリの外部から観測可能な振る舞い（要件・シナリオ）は変更しない。`pwa-web-app` を含む既存 spec の要件はすべて維持されるため、新規・変更 capability は発生しない。`.openspec.yaml` に `skip_specs: true` を設定して spec デルタを省略する。

### New Capabilities
- （なし）

### Modified Capabilities
- （なし）

## Impact

- **依存関係**: `frontend/package.json` に `tailwindcss` と `@tailwindcss/vite`（devDependencies）を追加。
- **ビルド構成**: `frontend/vite.config.ts` に `@tailwindcss/vite` プラグインを追加し、PWA マニフェストの `theme_color` を新アクセント（`#7c3aed`）へ更新。
- **スタイル**: `frontend/src/styles.css` を大幅に整理（Tailwind の `@import` と `@theme` を先頭に、RBC 用オーバーライドと最小グローバルのみ残置）。
- **コンポーネント**: `App.tsx` / `main.tsx` / `components/BottomTabs.tsx` / `components/EventForm.tsx` / `components/EventDetail.tsx` / `screens/ChatScreen.tsx` / `screens/CalendarScreen.tsx` / `screens/SettingsScreen.tsx` の `className` をユーティリティへ置換。
- **テスト**: `BottomTabs.test.tsx` / `App.test.tsx` はクラス名ではなくロール/テキストに依存しているか確認が必要（クラス名依存があれば更新）。Vitest 設定は `css: false` のため単体テストへの影響は小さい。
- **維持事項**: セーフエリア対応（`env(safe-area-inset-bottom)`）、`100dvh` レイアウト、月表示が見切れないための `min-width:0`、`prefers-reduced-motion` の各挙動を移行後も保つ。
