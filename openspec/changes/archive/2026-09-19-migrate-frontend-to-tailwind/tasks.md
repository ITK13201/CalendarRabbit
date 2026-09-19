## 1. Tailwind 導入とテーマ移植

- [x] 1.1 `frontend/package.json` に `tailwindcss` と `@tailwindcss/vite` を devDependencies として追加し、`pnpm install` が成功することを確認する
- [x] 1.2 `frontend/vite.config.ts` に `@tailwindcss/vite` プラグインを追加し、`pnpm dev` 起動時にビルドエラーが出ないことを確認する
- [x] 1.3 `frontend/src/styles.css` 冒頭に `@import "tailwindcss";` を追加し、既存 `:root` のトークンを `@theme`（`--color-*` / `--radius-*` / `--shadow-*` / `--ease-*` / `--tabbar-height` 等）へ移植する。アクセントを violet 系（`--color-primary: #7c3aed`、hover `#6d28d9`、soft `#f5f3ff`、ring は半透明 violet）へ更新する。`pnpm build` が通り、`bg-primary` など代表ユーティリティが生成されることを確認する
- [x] 1.4 `frontend/vite.config.ts` の PWA マニフェスト `theme_color` を `#7c3aed` へ更新し、`pnpm build` 後に `dist/manifest.webmanifest` へ反映されることを確認する
- [x] 1.5 再利用スタイル用に `@layer components` セクションを `styles.css` に用意し、`.btn` 系など代表クラスを `@apply` で定義できる土台を作る
- [x] 1.6 残すアニメーション（過剰な演出は簡素化）を `@theme` の `--animate-*` + `@keyframes` として `styles.css` に定義し、対応ユーティリティが使える状態にする

## 2. 共通要素の Tailwind 化

- [x] 2.1 ボタン（`.btn` / `.btn-primary` / `.btn-secondary` / `.btn-danger`）を `@layer components` のセマンティッククラスとして定義する（新アクセント violet を使用）。フォーカスリング・disabled・active の各状態が保たれることを目視確認する
- [x] 2.2 アプリシェル（`App.tsx` の `.app` / `.app-content`、`.screen` / `.screen-header`）をユーティリティ化する。`100dvh` と `.screen` の `min-w-0`（月表示の見切れ防止）が維持されることを確認する
- [x] 2.3 `BottomTabs.tsx` をユーティリティ化する。アクティブタブのインジケータ・`backdrop-blur` はユーティリティ、セーフエリア余白と tabbar 高さの `calc(...)` は該当ノードのみ最小CSSとして残し、挙動が維持されることを確認する

## 3. 画面・コンポーネントの Tailwind 化

- [x] 3.1 `ChatScreen.tsx` のバブル・入力欄・予定案カード・通知/エラーをユーティリティ化する。ユーザー/アシスタントのバブル配置と入力欄フォーカスリングを目視確認する
- [x] 3.2 `EventForm.tsx` と `EventDetail.tsx` のモーダル（`.modal-overlay` / `.modal` / `.field` / `.modal-actions` / `.detail-*`）を移行する（再利用パターンは `@layer components`、単発はユーティリティ）。オーバーレイのブラー・モーダルのスクロール・入力フォーカスが保たれることを確認する
- [x] 3.3 `SettingsScreen.tsx`（`.field` / セレクト / 保存ボタン / 通知・エラー）をユーティリティ化する

## 4. カレンダー（残置スコープ）とクリーンアップ

- [x] 4.1 `CalendarScreen.tsx` のラッパ（`.calendar-container` 等 RBC 外側）をユーティリティ化する。RBC 内部（`.rbc-*`）オーバーライドと agenda カード化メディアクエリは `styles.css` に残しつつ、ハードコード色を `@theme` 由来 CSS 変数へ置換する。カレンダー画面の見た目が移行前と同等であることをスクリーンショット比較で確認する
- [x] 4.2 旧手書きクラス定義を `styles.css` から削除し、残置が「`@import` / `@theme` / `@layer components`（再利用スタイル）/ 最小グローバル（`html,body,#root` リセット）/ 特殊値の最小CSS（セーフエリア・tabbar 高さ）/ 簡素化後アニメーション / RBC オーバーライド / `prefers-reduced-motion`」のみであることを確認する

## 5. 検証

- [x] 5.1 クラス名依存の既存テスト（`BottomTabs.test.tsx` / `App.test.tsx`）を確認し、必要ならロール／`aria-current`／テキスト依存へ更新して `pnpm test` が通ることを確認する
- [x] 5.2 `pnpm typecheck` / `pnpm test` / `pnpm build` がすべて成功することを確認する
- [x] 5.3 モバイル幅で全画面を目視確認する: 月表示のナビゲーション後に見切れない・agenda がカードで読める・下部タブがセーフエリアと重ならない・`prefers-reduced-motion` でアニメーションが抑制される（`pwa-web-app` のモバイル表示品質要件の維持）
- [x] 5.4 新アクセント（violet）が全画面（ボタン・タブ・バブル・カレンダーの today/イベント・フォーカスリング）に一貫適用され、manifest の `theme_color` と揃っていることを目視確認する
