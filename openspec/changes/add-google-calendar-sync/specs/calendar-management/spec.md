## MODIFIED Requirements

### Requirement: 将来のカレンダープロバイダ連携を見据えた抽象化

システムはカレンダーの永続化を、外部プロバイダ（Google Calendar API 等）と組み合わせ可能な抽象境界の背後に置かなければならない（SHALL）。DB を正（source of truth）とし、Google Calendar への同期はその抽象境界の背後で行うミラーとして扱わなければならない（SHALL）。イベントの作成・更新・削除は、Google 連携が有効な場合に専用カレンダーへの即時ミラー同期を伴う（詳細は `google-calendar-sync` を参照）。Google 連携が無効な場合、または Google への同期が失敗した場合でも、CRUD は DB 上で成功として完結しなければならない（SHALL）。

#### Scenario: DBプロバイダでCRUDが完結する

- **WHEN** Google 連携が無効な状態でイベントの CRUD を行う
- **THEN** すべての操作は DB を通じて完結し、外部カレンダーAPIには依存しない

#### Scenario: 連携時は CRUD が Google ミラー同期を伴う

- **WHEN** Google 連携が有効な状態でイベントの作成・更新・削除を行う
- **THEN** システムは DB を更新したうえで、専用カレンダーへ即時にミラー同期する

#### Scenario: Google 同期が失敗しても CRUD は成功する

- **WHEN** Google 連携が有効だが Google への同期が失敗する
- **THEN** システムは DB 操作を成功として返し、CRUD の外部から見た結果は変わらない
