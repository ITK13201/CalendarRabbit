## Purpose

バックエンドの構造化ロギングと可観測性を規定する。全ログに共通する構成（トップレベル項目 + `extra` ネスト）、HTTP リクエスト/レスポンスログ、サービス層および外部API/DB呼び出しの前後ログ、機密情報のマスク、ボディ長の丸めといった、外部から観測可能なログの振る舞いを定義する。

## Requirements

### Requirement: 統一されたログ全体構成

システムはすべてのログを JSON 形式の構造化ログとして出力しなければならない（SHALL）。各ログレコードは `time`, `level`, `msg`, `requestId`, `method`, `path`, `query` をトップレベルのプロパティとして持ち、それ以外の任意のプロパティは `extra` オブジェクト配下にネストしなければならない（SHALL）。`requestId`, `method`, `path`, `query` は、リクエストに紐づくログであればリクエストの文脈から解決してトップレベルに付与しなければならない（SHALL）。リクエストの文脈を持たないログ（起動時ログ等）では、これらのトップレベル項目は空または省略されてよい（MAY）。

#### Scenario: リクエストに紐づくログのトップレベル項目

- **WHEN** リクエスト処理中に任意のログが出力される
- **THEN** そのログは `time`, `level`, `msg`, `requestId`, `method`, `path`, `query` をトップレベルに持ち、当該リクエストの `requestId`・HTTP メソッド・パス・クエリがそれぞれ反映される

#### Scenario: 追加プロパティは extra 配下にネストされる

- **WHEN** ログにトップレベル項目以外のプロパティ（引数・戻り値・ボディ・ステータス等）が付与される
- **THEN** それらのプロパティは `extra` オブジェクト配下にネストして出力され、トップレベルには現れない

#### Scenario: リクエスト文脈を持たないログ

- **WHEN** サーバ起動時など、リクエストに紐づかない状況でログが出力される
- **THEN** システムは `time`, `level`, `msg` を出力し、`requestId`・`method`・`path`・`query` は空または省略してよい

### Requirement: リクエストID の採番と伝播

システムは各 HTTP リクエストに対して一意の `requestId` を採番しなければならず（SHALL）、リクエストに既存の requestId ヘッダが付与されている場合はそれを引き継がなければならない（SHALL）。採番した `requestId` はレスポンスヘッダに付与し（SHALL）、後続のすべてのログ（リクエスト/レスポンス・サービス層・外部API/DB）でトップレベルの `requestId` として追跡可能でなければならない（SHALL）。

#### Scenario: requestId を新規採番する

- **WHEN** requestId ヘッダを持たないリクエストを受信する
- **THEN** システムは一意の requestId を採番し、レスポンスヘッダに付与し、当該リクエストのすべてのログのトップレベル `requestId` に反映する

#### Scenario: 既存の requestId を引き継ぐ

- **WHEN** requestId ヘッダを持つリクエストを受信する
- **THEN** システムはその値を requestId として引き継ぎ、レスポンスヘッダおよび全ログに反映する

### Requirement: HTTP リクエスト/レスポンスログ

システムは各 HTTP リクエストの受信時に `msg='http.request'` のログを出力し、`extra` 配下にクエリ・リクエストヘッダー・リクエストボディを含めなければならない（SHALL）。各レスポンスの返却時には `msg='http.response'` のログを出力し、`extra` 配下にステータスコード・レスポンスヘッダー・レスポンスボディ・`latency_ms`（リクエスト受信からレスポンスまでの所要時間ミリ秒）を含めなければならない（SHALL）。リクエストボディのログ化は後続ハンドラの読み取りを妨げてはならない（SHALL NOT）。

#### Scenario: リクエスト受信時のログ

- **WHEN** システムが HTTP リクエストを受信する
- **THEN** システムは `msg='http.request'` のログを出力し、`extra` にクエリ・ヘッダー・ボディを含める

#### Scenario: レスポンス返却時のログ

- **WHEN** システムが HTTP レスポンスを返却する
- **THEN** システムは `msg='http.response'` のログを出力し、`extra` にステータスコード・ヘッダー・ボディ・`latency_ms` を含める

#### Scenario: ボディログ化がハンドラ処理を妨げない

- **WHEN** リクエストボディをログ化する
- **THEN** 後続のハンドラは同じリクエストボディを通常どおり読み取れる

### Requirement: サービス層メソッドの前後ログ

システムはサービス層（アプリケーションの業務ロジック層）の各メソッド呼び出しの前後でログを出力しなければならない（SHALL）。呼び出し前に `msg='[StructName.MethodName] started'`、完了後に `msg='[StructName.MethodName] finished'` を出力し（StructName・MethodName は当該メソッドの構造体名・メソッド名）、両ログには `requestId`・メソッド名・引数を含め、`finished` ログには戻り値およびエラー情報を含めなければならない（SHALL）。エラーが発生した場合、`finished` ログはそのエラー情報を含めなければならない（SHALL）。

#### Scenario: 正常完了時の前後ログ

- **WHEN** サービス層のメソッドが呼び出され正常に完了する
- **THEN** システムは呼び出し前に `[StructName.MethodName] started`、完了後に `[StructName.MethodName] finished` を出力し、requestId・メソッド名・引数・戻り値を含める

#### Scenario: エラー発生時の前後ログ

- **WHEN** サービス層のメソッドがエラーを返す
- **THEN** `finished` ログはエラー情報を含めて出力される

### Requirement: 外部API/DB呼び出しの前後ログ

システムは外部API呼び出しおよびデータベース呼び出しの各メソッドの前後でログを出力しなければならない（SHALL）。呼び出し前に `msg='[StructName.MethodName] started'`、完了後に `msg='[StructName.MethodName] finished'` を出力し、両ログには `requestId`・メソッド名・引数を含め、`finished` ログには戻り値およびエラー情報を含めなければならない（SHALL）。外部API/DBのレスポンスボディは可能な限り `finished` ログに含めなければならない（SHALL）。ただしボディが長すぎる場合は、先頭1000文字に丸めて出力しなければならない（SHALL）。

#### Scenario: 外部API呼び出しの前後ログとレスポンスボディ

- **WHEN** 外部API（またはDB）呼び出しメソッドが実行される
- **THEN** システムは呼び出し前に `started`、完了後に `finished` を出力し、requestId・メソッド名・引数・戻り値・エラー情報を含め、可能な範囲でレスポンスボディを含める

#### Scenario: レスポンスボディが長すぎる場合の丸め

- **WHEN** ログに含めようとするレスポンスボディが1000文字を超える
- **THEN** システムはボディを先頭1000文字に丸めて出力する

#### Scenario: 外部呼び出しがエラーを返す場合

- **WHEN** 外部API/DB呼び出しがエラーを返す
- **THEN** `finished` ログはエラー情報を含めて出力される

### Requirement: 機密情報のマスク

システムはログ出力時に、既知の機密情報を含むヘッダーおよびフィールド（例: `Authorization`, `X-Api-Key`, `Cookie`, `Set-Cookie`）の値を `[REDACTED]` などのマスク値へ置換しなければならない（SHALL）。マスクはヘッダーのキー照合において大文字小文字を区別してはならない（SHALL NOT）。

#### Scenario: 機密ヘッダーがマスクされる

- **WHEN** リクエストまたはレスポンスのヘッダーに `Authorization` など既知の機密ヘッダーが含まれる
- **THEN** システムはその値をマスク値に置換してログ出力し、生の値をログに残さない

#### Scenario: マスク対象のキー照合は大文字小文字を区別しない

- **WHEN** 機密ヘッダーのキーが `authorization` のように異なる大文字小文字で現れる
- **THEN** システムは同一のヘッダーとして扱い、値をマスクする
