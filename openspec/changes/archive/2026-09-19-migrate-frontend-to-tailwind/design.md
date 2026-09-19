## Context

現状のスタイルは `frontend/src/styles.css`（約720行）に集約され、`:root` の CSS 変数群（`--primary`, `--bg`, `--radius`, `--shadow-*`, `--transition` 等）と手書きの BEM 風クラス（`.btn`, `.bubble`, `.modal`, `.field` など）で構成される。コンポーネント側（`App.tsx` ほか）は文字列 `className` でこれらを参照している。ビルドは Vite 7 + `@vitejs/plugin-react`、テストは Vitest（`css: false`）。動機は proposal.md - Why を参照。

制約:
- `react-big-calendar` は自前 DOM に `.rbc-*` クラスを付与するため、Tailwind ユーティリティを要素へ直接付与できない。現行はこれを子孫セレクタでオーバーライドしている。
- モバイル表示の重要挙動を維持する必要がある: セーフエリア（`env(safe-area-inset-bottom)`）、`100dvh`、月表示が見切れないための `min-width:0`、`prefers-reduced-motion`。これらは `pwa-web-app` spec のモバイル表示品質要件に対応する。

## Goals / Non-Goals

**Goals:**
- Tailwind CSS v4 を CSS-first 構成で導入し、コンポーネントの `className` をユーティリティへ移行する。
- 既存デザイントークンを Tailwind の `@theme` へ移植し、Tailwind 化した箇所と残置 CSS（RBC）の双方から同じトークンを参照できるようにする。
- `styles.css` を「Tailwind の入口 + テーマ + 最小グローバル + RBC オーバーライド」だけの薄いファイルに縮小する。

**Non-Goals:**
- 元デザインのピクセル単位の再現（近い水準で可）。
- `react-big-calendar` 内部 DOM の Tailwind 化（プレーンCSSのまま残す）。
- 機能・API・データモデルの変更（本件はスタイル基盤のみ）。
- ダークモード等の新規テーマ追加。

## Decisions

### D1: Tailwind v4 + `@tailwindcss/vite`（CSS-first）を採用

`tailwind.config.js` を持つ v3 + PostCSS 構成ではなく、v4 のファーストパーティ Vite プラグインを使う。理由:
- Vite プロジェクトに最適化され、設定ファイルが `styles.css` 内の `@import "tailwindcss";` と `@theme { ... }` に集約されるため、トークンの単一情報源を保てる。
- content 検出（テンプレートスキャン）が自動で、設定量が少ない。

代替案: v3 + `postcss.config.js` + `tailwind.config.js`。枯れているが設定が分散し、CSS 変数とユーティリティの二重管理になりやすいため不採用。

### D2: デザイントークンは `@theme` に移植し、CSS 変数として RBC からも参照

既存 `:root` の値を `@theme` へ移す。v4 は `@theme` 内の各トークンを CSS 変数として公開し、同時に対応するユーティリティ（例: `--color-primary` → `bg-primary` / `text-primary`）を生成する。これにより:
- Tailwind 化したコンポーネントは `bg-primary` などのユーティリティを使用。
- 残置する `.rbc-*` オーバーライドは同じ CSS 変数（`var(--color-primary)` 等）を参照でき、色・角丸の一貫性を保てる。

命名は Tailwind の名前空間規約に合わせる（`--color-*`, `--radius-*`, `--shadow-*`, `--ease-*`）。既存の意味的トークン（`primary-soft`, `border-strong`, `danger-soft` 等）はそのまま `--color-*` として持ち込む。

### D2b: 繰り返し現れるスタイルは `@layer components` でクラス化

ボタン（`.btn` 系）・フォーム入力（`.field` 系）・モーダル・チャットバブルなど、複数箇所で再利用されるパターンは、Tailwind の `@layer components` 内で `@apply`（またはユーティリティ合成）を用いてセマンティッククラスとして定義する。JSX 側の `className` は `.btn btn-primary` のように短く保ち、重複を避ける。

代替案: React コンポーネント化（`<Button variant>`）や各要素への直接ユーティリティ直書き。前者はファイル増とリファクタ範囲拡大、後者は className の長大化・重複を招くため不採用。

### D2c: アクセントカラーを violet 系へ刷新

背景はニュートラル基調のまま、アクセントを既存インディゴ（`#4f46e5`）から violet 系へ更新する:
- `--color-primary: #7c3aed`（violet-600）
- hover: `#6d28d9`（violet-700）／ active: `#5b21b6`（violet-800）相当
- soft 面: `#f5f3ff`（violet-50）／ ring: 半透明の violet

あわせて PWA マニフェストの `theme_color`（`vite.config.ts`、現行 `#4f46e5`）も新アクセントへ合わせる。元デザインの完全再現は目標外（proposal 参照）のため、この刷新は許容範囲。

### D3: RBC オーバーライドと一覧カード化メディアクエリは残置

`.calendar-container .rbc-*` の子孫セレクタ群と、`@media (max-width: 600px)` の agenda カード化は Tailwind では表現しにくいため、`styles.css` にプレーンCSSとして残す。ただしハードコード色は `@theme` 由来の CSS 変数へ置換し、トークンと連動させる。`.calendar-container` のラッパ自体は Tailwind ユーティリティ化してよい（RBC の内部 DOM ではないため）。

### D4: モバイル挙動は Tailwind の任意値／プラグイン相当で再現

- `100dvh` → `h-[100dvh]`。
- `min-width:0`（`.screen`）→ `min-w-0`（横方向の見切れ防止、design の要）。
- セーフエリア → 任意値ユーティリティ（`pb-[env(safe-area-inset-bottom)]` 等）で表現。下部タブ高さは既存 `--tabbar-height` をトークン化し `h-[calc(theme(...)+env(safe-area-inset-bottom))]` 相当で再現するか、当該ノードのみ最小CSSを残す（実装時に可読性で判断）。
- `backdrop-filter`（下部タブ／モーダル）→ `backdrop-blur` + `backdrop-saturate` ユーティリティ。
- アニメーション（`bubble-in`, `modal-in`, `overlay-in`）→ 完全再現は不要（proposal 参照）のため簡素化を許容する。残すものは `@theme` の `--animate-*` + `@keyframes` として `styles.css` に定義し、過剰な演出は削除して実装を軽くする。
- `prefers-reduced-motion` のグローバル無効化は `styles.css` に残す（横断的挙動のため）。

特殊値（`env(safe-area-inset-bottom)`・`100dvh`・`calc(...)` を伴う tabbar 高さ等）は、任意値ユーティリティで冗長化するより、該当ノードのみ最小のプレーンCSSとして `styles.css` に残す方針とする（可読性とレイアウトの確実性を優先）。`min-w-0` 等の単純なものはユーティリティで表現する。

### D5: テストへの影響を確認して最小限に更新

`BottomTabs.test.tsx` / `App.test.tsx` がクラス名（`tab-active` 等）に依存していないか確認する。依存があればロール／`aria-current`／テキストベースへ寄せる。Vitest は `css: false` のため描画スタイルは検証されず、単体テストの主眼はロジック・アクセシビリティ属性であることを前提とする。

## Risks / Trade-offs

- [RBC の見た目がトークン移植時にズレる] → RBC オーバーライドの色をハードコードから CSS 変数へ置換する際、変数名の対応を1対1で確認。移行前後でカレンダー画面をスクリーンショット比較。
- [セーフエリア／`100dvh` 等の任意値ユーティリティが冗長で可読性を損なう] → 頻出パターンはコンポーネント内で共通化するか、当該箇所のみ最小CSSを残置する判断を許容（D4）。完全ユーティリティ化に固執しない。
- [クラス名に依存した既存テストの破損] → D5 で事前確認し、ロール／テキスト依存へ移行。
- [`@theme` のトークン命名規約ズレでユーティリティが生成されない] → v4 の名前空間（`--color-*` 等）に厳密に合わせ、`pnpm build` と目視でユーティリティ生成を確認。
- [PWA（vite-plugin-pwa）のアセット globbing に CSS 変更が影響] → 出力 CSS のハッシュ名が変わるだけで workbox の `**/*.css` に含まれるため実害なし。ビルド後に `dist` を確認。

## Migration Plan

1. `tailwindcss` と `@tailwindcss/vite` を devDependencies に追加し、`vite.config.ts` にプラグイン追加。
2. `styles.css` 冒頭に `@import "tailwindcss";` を追加し、`:root` を `@theme` へ移植（CSS 変数名を Tailwind 名前空間へ整理）。
3. コンポーネント単位で `className` をユーティリティへ置換（共通の `.btn` 系 → 小さな React ヘルパ or 直接ユーティリティ）。ボタン → タブ → チャット → フォーム/モーダル → 各画面の順。
4. Tailwind 化が完了したクラス定義を `styles.css` から削除。RBC オーバーライド・最小グローバル・アニメーション・`prefers-reduced-motion` のみ残す。
5. `pnpm typecheck` / `pnpm test` / `pnpm build` を通し、各画面（特にカレンダーのモバイル表示・セーフエリア）を目視確認。

ロールバック: 本変更は追加依存とスタイル記述の置換に限定されるため、コミットの revert で原状復帰可能。
