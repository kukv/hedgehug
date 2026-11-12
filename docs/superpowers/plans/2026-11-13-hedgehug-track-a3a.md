# hedgehug トラック A3 前半(プロパティとステータス)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 詳細パネルと作成ダイアログで、組み込みの 8 つのプロパティと追加プロパティの値を入れて保存でき、ステータスのメニューと Step のチェックでステータスを変えられるようにする(遷移の制約・テンプレート・使わないステータスを除く)。

**Architecture:** アイテムの値は `domain/item` の `Item` に足し、ログの項目(`status`・`assignee`・`fields/<ID>` など)で持つ。型ごとの読み分けは、ワークスペースの設定(`workspace.Property` の型)で決める。操作は `app/items` に 1 ファイル 1 操作で足し(プロパティの変更・複数の値の入り切り・Step のチェック・作成の初期値)、API は `adapter/httpapi/items` に足す。画面のロジック(プロパティの行、ステータスのメニュー、作成ダイアログの値の連動)は `core/features/items` に、見た目は `app/screens/item` と `app/screens/project` に置く。値を選ぶメニューとアバターは ui の共有部品にする。

**Tech Stack:** 土台と同じ(Go 1.27.3、huma v2、go-git、Svelte 5、orval、Vitest、Testing Library、Playwright)。新しい依存は足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md`(正本): 「プロパティの入力と保存」全節、「ステータスの変更」、「アイテムの作成」の「開き方と初期値」のステータス・優先度、「ダイアログの項目」のプロパティ、「ダイアログの中での連動」のテンプレート以外、「入力の検査」の日付・ポイント・見積もり時間、「詳細パネル」の「上から順の構成」のプロパティ、「子と Step」の行・見出しの数・Step の追加の値・Step のチェック、「操作ごとの記録と通知」
- 01「書き込みの単位」「閲覧のみの Store」「日付と時刻」「画面の移り方と、共通の表示」(値を文で出す形、「(削除された値)」)
- 03「プロパティ」(組み込みの 8 種・追加プロパティ・使う種類・非表示・並べ替え)、「値の編集」の「分類」「分類ごとの既定のステータス」「初期ステータス」「初期優先度」、「プロジェクト別のルール」の「使うステータス」(持ち場の確認のみ)
- 04「集計の定義」「ツリー」の「行」
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`「アイテムの例」「変更記録」
- トラック A の計画 `docs/superpowers/plans/2026-11-12-hedgehug-track-a.md`(A3 の行・漏れの表)、A2 の計画 `docs/superpowers/plans/2026-11-13-hedgehug-track-a2.md`(決定 10・14・17・18)、トラック B の計画 `docs/superpowers/plans/2026-11-12-hedgehug-track-b.md`(未決 3)、B2 の計画(`feat/track-b2` の `docs/superpowers/plans/2026-11-13-hedgehug-track-b2.md`。B2-1・B2-2・B2-12・漏れの表)、PR #159 の「A3 への申し送り」

## 持ち場・待つもの・前半と後半の境目

**持ち場:** トラック A の持ち場(`internal/domain/item`・`internal/app/items`・`internal/adapter/httpapi/items`・`web/packages/core/src/features/items`・`web/app/src/screens/item`・`web/app/src/screens/project`)。共有部分として、`domain/item` の中心の型と `item.Create`・`FromLog`(Task 1)、`gitstore` の `item.yaml`(Task 2)、`apptest` のサンプルと照合テスト(Task 1)、ui の部品 `ValueMenu`・`Avatar`(Task 7)を変える。共有部分は、その変更だけの小さな PR を先に develop へ入れる。

**待つもの:**

| もの | 状態(2026-11-13 04:14 時点) | このトラックで使うもの |
|---|---|---|
| A2(#157) | develop に入っている | 作成ダイアログ・詳細パネル・ツリーの骨組み |
| B1(#149)・#159 | develop に入っている | `workspace.Settings`、`item.Item` の `Status`・`Priority`・`Systems`・`Choices` |
| **B2 本体(`feat/track-b2`)** | Task 15(e2e)まで済み、PR はまだ | `workspace.Property.Choices`・`Settings.Property`、`item.yaml` の書き出し(`yaml_notes.go`)、web の `model/workspace.ts`(`WorkspaceSettings` ほか)・`port/workspaces.ts`(`WorkspacesPort.get`)・`api/workspaces.ts`、プロパティを足す API(e2e で追加プロパティを作る) |

B2 本体との重なり(`git diff origin/develop...feat/track-b2` で確かめた):

| ファイル | 重なり | 扱い |
|---|---|---|
| `internal/domain/workspace/settings.go`・`lists.go` | B2 が `Property.Choices`・`Settings.Property` を足す | A3 前半は B2 のものを使い、同じものを作らない |
| `internal/infra/gitstore/yaml.go`・`yaml_notes.go` | B2 が `itemToYAML` を `yaml_notes.go` に移し、設定を受け取る形にする | Task 2 は B2 のマージ後に、B2 の `itemToYAML` に行を足す |
| `web/packages/core/src/model/workspace.ts`・`port/workspaces.ts`・`api/src/workspaces.ts` | B2 が設定の型と口を足す | A3 前半の画面は B2 の `WorkspaceSettings` と `WorkspacesPort.get` を使い、別名の型を作らない |
| `api/openapi.yaml`・`generated.ts`・`core/src/index.ts`・`app/src/wire.ts`・`DetailHeader.svelte`・`ProjectScreen.svelte` | B2 も変える | マージ後に取り込んでから触る(生成物は作り直す) |
| `internal/domain/item`・`internal/app/items`・`internal/adapter/httpapi/items` | B2 は触らない | 重なりなし |
| C2 画面(`feat/track-c2-screen`) | `core/features/toasts`・`sync`・`index.ts` | `index.ts` の export の追記だけ。マージの順で取り込む |

**進め方の順:** Task 1(共有 PR)はいま develop から始められる。Task 2 以降は、サーバーのタスクも B2 の `Settings.Property`・`Property.Choices` を使うので、B2 本体が develop に入ってから、develop を取り込んで進める。B2 は e2e(最後のタスク)まで済んでいるので、Task 1 の PR のあいだに入る見込み。入っていなければ、Task 1 の後で止めて知らせる。

**前半と後半の境目:**

| 区分 | 中身 |
|---|---|
| 前半(この計画) | 組み込み 8 つと追加プロパティ 8 型の値の持ち方と読み分け、詳細パネルでの入力と保存、ステータスのメニュー(遷移の制約と使わないステータスを除く)、作成ダイアログのプロパティ欄と初期値(初期ステータス・初期優先度)、ダイアログの中での連動のうちテンプレート以外、入力の検査(日付・ポイント・見積もり時間)、Step の追加の値、Step のチェック、ツリーの行と詳細パネルの子の行の値の表示、#159 と A2 からの申し送り |
| 後半(A3b。B3 が develop に入ってから別の計画) | テンプレート(作成ダイアログへの適用、連動のうちテンプレートの部分、「一緒に作るもの」)、遷移の制約(`workspace.CanTransition` を、ステータスのメニュー・Step のチェック・ツリーのステータスのボタンから呼ぶ) |
| ほかの持ち場 | 下の決定 3〜6 |

## Global Constraints

トラック A・A2 の計画の Global Constraints をすべて引き継ぐ(土台の Global Constraints を含む)。主なもの:

- 利用者に見せる断りの文言・アクティビティの文は、仕様 02 の表と文を一字一句そのまま使う。仕様にない文言は足さない。足すときは下の「この計画で決めたこと」に載せる。
- 通知は作らない(トラック C が変更記録から作る)。このトラックは、通知の判定に要る変更記録(担当者など)を正しく書くことだけを受け持つ。
- 依存は足さない。足すなら版を明記し、公開から 7 日以上たった版だけを使う(`@latest` は禁止)。
- `api/openapi.yaml` と `web/packages/api/src/generated.ts` は手で直さない。API を変えたタスクで `make generate` と `cd web && pnpm gen` を実行し、同じコミットに入れる。
- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージ 20 ファイル(テストを除く。`apptest` はいま 20 で上限)、Svelte 200 行、TS 300 行。超えたら上限を上げずに分ける。
- ui の部品は渡された値を出して events を返すだけ。値の計算・検査・操作の流れは core に置く(設計書 6 節)。
- モックの振る舞いを仕様として書き写さない。見た目(色・間隔)だけをモック版 19 に合わせる。
- 各タスクは TDD で進める(失敗するテスト → 失敗を確かめる → 実装 → 通す → コミット)。タスクの終わりに `make check` を通す(画面のタスクは `cd web && pnpm check` と該当の e2e も)。
- 計画やテストにトークンらしい値(`ghp_` など)を書かない。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| 1 | 前半と後半の境目は上の表のとおり。テンプレートと遷移の制約は後半(A3b)で、B3 が develop に入ってから別の計画で作る | ユーザー決定(2026-11-13)。トラック B の未決 3(判定は `domain` の関数で、A の操作が呼ぶ) |
| 2 | 遷移の制約を後半で足す場所は、ステータスを変える操作(`app/items` の `SetProperty` のステータス、`CheckStep`)の保存の中とし、前半ではそこに検査を置かない。画面のメニューの項目を無効にする処理も後半で足す | 02「ステータスの変更」。後半の差し込み口を 1 か所にする |
| 3 | 「使わないステータス」(メニュー・作成ダイアログに出さない、いまの値に「このプロジェクトでは使わない」、断りの「このプロジェクトでは「{ステータス}」を使いません」)は、トラック B の B5 の持ち場(B の計画の小計画の表「プロジェクト別のルール」全節、漏れの表の「プロジェクト別のルール / 使うステータス」)。判定の関数は B5 が `domain/workspace` に足し、A の操作と画面から呼ぶ部分は、B5 が入ったときに A 側のタスクとして足す。前半では、ワークスペースのすべてのステータスを出す | トラック B の計画の未決 3 の推奨(自動化の呼び出しを B4 が入ったときに A 側のタスクとして足す)と同じ分け方。B5 の前は使わないステータスのデータがない |
| 4 | 「複製」は前半にも後半にも入れず、A4 に移す。複製はテンプレートに依存しない(02「複製」の表の「子」の行は、テンプレートを別の手段として案内するだけ)が、clones の関連(A4 の「関連」)、アイテムのメニュー(A4)、説明の保存(A4)、スプリント・リリース(トラック F)に依存する | 02「複製」。トラック A の計画の A4 の行(関連・メニュー・説明) |
| 5 | 「一緒に作るもの」は後半(テンプレートが子や Step を持つときだけ出る) | 02「ダイアログの項目」の「一緒に作るもの」、03「適用するときの規則」 |
| 6 | 作成ダイアログの「説明」の欄は A4 に移す(説明の保存 `description.md` と `desc` の行、メンションは A4 で作る)。後半(A3b)のテンプレートの説明の適用は、A4 の説明の欄を待つ。「スプリント」の欄、詳細パネルのスプリント・リリースは、スプリントとリリースを作るトラック F が足す | トラック A の計画の A4 の行(説明・メンション)、A5 の行(作成のスプリント・リリースはトラック F) |
| 7 | 組み込みの値のログの項目名は `status`・`assignee`・`priority`・`start`・`end`・`points`・`estimate`・`systems`、追加プロパティは `fields/<プロパティの内部 ID>` とする。組み込みの項目名は、組み込みのプロパティの内部 ID(= キー。03「組み込みの 8 種」)と同じにする。保存形式の「アイテムの例」の `due` は `end` に直した(ユーザー決定 2026-11-13。下の Q1) | 03「組み込みの 8 種」のキー `end`、04 の検索のキー `end`。プロパティの内部 ID から項目名を 1 つの規則で引ける |
| 8 | 値の形: `status`・`priority`・単一選択は値の内部 ID か null(`set`)。`systems`・複数選択は値の内部 ID の `add`・`remove`。`assignee`・ユーザー型はログイン名か null。`start`・`end`・日付型は `YYYY-MM-DD` か null。`points`・`estimate`・数値型は JSON の数か null。テキスト・URL は前後の空白を除いた文字列か null(空は null)。チェックは true・false(作成時に値がなければ書かない。読むときは false) | 保存形式「アイテムの例」「変更記録」(1 つの値ごとに 1 項目、複数は add・remove) |
| 9 | 型ごとの読み分けは設定で決める。`FromLog` は、`fields/<ID>` の `set` の値をすべて生のまま `Item.Fields`(`map[change.ID]json.RawMessage`)に持つ。`Choices` は #159 のまま残す(B2 の yaml・値の削除が使う)。値を型どおりに読むのは `item.FieldValue(it, p workspace.Property)` で、単一選択・複数選択は `Choices`、それ以外は `Fields` を読む。内部 ID の形をしたテキストやログイン名も、テキスト・ユーザーとして読める | PR #159 の申し送り(`readValues` は内部 ID でない値を読み飛ばす)。`domain/item` は `domain/workspace` を import できる |
| 10 | 作成は、ステータスを指定がなければ初期ステータス、優先度を指定がなければ初期優先度にする。ほかの値は指定のあるものだけを作成の行に書く。Step の追加は、担当者を親と同じ、ステータスを初期ステータス、優先度を初期優先度にする | 02「開き方と初期値」、「子と Step」の「Step の追加」、A2 の決定 10 |
| 11 | `apptest.Sample` のアイテムに、ステータス Todo・優先度 Medium を入れる(作成の初期値と同じ)。A3 より前に作ったアイテム(ステータス・優先度が空)は、値がワークスペースの一覧にないものとして扱い、詳細パネルなどに薄い字で「(削除された値)」を出す。押すと選び直せる | 02「プロパティの入力と保存」(「必ず値がある」)、01「画面の移り方と、共通の表示」(値がワークスペースの一覧にないとき)。空の値は開発中のデータにしかない |
| 12 | プロパティの変更は `PUT /api/stores/{store}/items/{item}/properties/{property}` 本文 `{value}`(operationId `set-item-property`)→ 更新後のアイテム。`{property}` はプロパティの内部 ID(組み込みはキー)。`value` は JSON の値で、数(ポイント・見積もり時間・数値)は入力欄の文字のまま文字列で送り、サーバーが読む。空の文字列と null は「値を空にする」 | 02「入力の方法と保存」(1 つ変えるごとに 1 回の保存)。検査の文言を Go の 1 か所に置く(A2 の決定 14) |
| 13 | 関連システム・複数選択の入り切りは `POST …/items/{item}/properties/{property}/values/{value}`(`add-item-value`)と `DELETE` 同パス(`remove-item-value`)→ 更新後のアイテム。すでにある値の追加・ない値の取り除きは何も保存しない | 02「入力の方法と保存」(項目ごとに入り切りし、選ぶたびに保存)。保存形式(add・remove) |
| 14 | Step のチェックは `POST …/items/{item}/check` 本文 `{done}`(`check-step`)→ 更新後のアイテム。`done: true` は完了の既定、`false` は未着手の既定のステータスにする。前と同じなら何も保存しない。Step でなければ 404 | 02「Step のチェック」、03「分類ごとの既定のステータス」の表 |
| 15 | アクティビティは「{ID} の{プロパティ名}を {値} に変更」(Step のチェックも、ステータスの変更として同じ文)。{プロパティ名} はいまのワークスペースのプロパティ名。{値} は 01 の形: 空は「なし」、チェックは「オン」「オフ」、複数の値は「, 」でつなぐ(入り切りでは、変えたあとのすべての値)、見積もり時間は「N 時間」(小数第 2 位まで、末尾の 0 を省く)、日付は 01 の日付の表示(保存した日の年なら「M/D」、ほかの年は「YYYY/MM/DD」)、値(ステータス・優先度・選択肢・関連システム)は名前、ユーザーはメンバーの表示名(メンバーにいなければ「不明なユーザー」。01「画面の移り方と、共通の表示」に合わせる) | 02「入力の方法と保存」「時間の値」、01「画面の移り方と、共通の表示」「日付と時刻」。人の名前は 02「概要の編集」の「オーナーなら人の名前」に合わせた |
| 16 | 検査は 02「入力の検査」の表の順(タイトル・プロジェクト・親・日付・ポイント・見積もり時間)で、作成と詳細パネルで同じ関数を使う。詳細パネルの日付は、変える側と保存済みのもう片方で比べる。ポイントの「数でない値」は、ポイントの文言「ポイントは 0 以上の数で入力してください」で断る。数値型の追加プロパティの数でない値は、画面で送らずに元の値に戻す(サーバーに届いたら 422 で、仕様の文言は持たない) | 02「入力の方法と保存」の検査の表、「入力の検査」の表。数値型には仕様の文言がない |
| 17 | 詳細パネルの断りはエラーのトーストで出し、入力欄を保存済みの値に戻す。作成ダイアログの断りはダイアログの中に赤字で出し、入力は残す | 02「入力の方法と保存」(何も変えずに理由を表示)、「入力の検査」(赤字・入力を残す)。A2 の決定 13・14 |
| 18 | 値の選択肢がない値の指定(一覧にない値の内部 ID、Step でない `check-step`、ないプロパティ)は `port.ErrNotFound`(404)。使う種類から外れた・非表示のプロパティへの変更は断らない(値は残る規則なので保存してよい。画面には出さない) | B2-12 と同じ扱い。03「使う種類」「非表示」(値は消えない) |
| 19 | 詳細パネルのプロパティは、ワークスペースの設定の並び(`Settings.Properties`)のうち、非表示でなく、そのアイテムの種類で使うものを出す。追加プロパティには「追加」と添える。末尾に「プロパティを追加・変更」を置き、ワークスペースの設定タブ(B2 のアドレス `/s/{store}/w/{prefix}/settings`)を開く | 02「プロパティの種類」「上から順の構成」、03「並べ替え」 |
| 20 | 入力の形は 02 の表のとおり。ステータス・優先度・単一選択・関連システム・複数選択は ui の新しい部品 `ValueMenu`(ボタンからメニュー。いまの値にチェック、選択肢が 7 件以上で検索欄、複数は開いたまま)。担当者・ユーザーは `Select`(先頭に「未割り当て」、あなたに「(あなた)」)。日付は `<input type="date">`、数値は `<input type="number">`(見積もり時間は `step="0.25"`)、チェックは `Checkbox`、テキスト・URL は `Input`。保存のきっかけは、選ぶもの・チェックは選んだとき、入力欄はフォーカスが外れたとき(Enter でも外す。タイトルと同じ) | 02「入力の方法と保存」の表。保存のきっかけは仕様にないので、詳細パネルのタイトル(02「タイトル」)に合わせた |
| 21 | ステータスのメニューの項目は「{名前}」と分類(「未着手」「進行中」「完了」「中止」)を添え、いまのステータスにチェックを付ける。ステータスのボタンは、詳細パネルのプロパティ、詳細パネルの子の行(Epic の子・Step)、ツリーの行に置く | 02「ステータスの変更」、「子と Step」、04「ツリー」の「行」 |
| 22 | ツリーの行には、Step の進捗「完了/対象」(Story・Task・Bug で、対象が 0 件でないとき)、期限、ポイント、ステータスのボタン、担当者のアバターを出す。「ブロック中」とメニューは A4。期限の「期限切れ」「今日」は画面を出したときの日付で決め、日付が変わったときの自動の更新はトラック E(マイタスクと集計の表示の持ち主)が足す | 04「ツリー」の「行」「集計の定義」、A2 の決定 17。01「日付と時刻」の自動の更新は複数の画面にまたがる |
| 23 | 詳細パネルの子の見出しに「3/8」(完了/中止を除く総数)を添える。Epic の子の行にステータスのボタンと担当者のアバター、Step の行にチェックボックス・ステータスのボタン・担当者のアバターを足す。Step の削除の「×」は A4 | 02「子と Step」、A2 の決定 18 |
| 24 | アバターは ui の新しい部品 `Avatar`(表示名の頭の 1 文字を丸に出し、`title` に表示名)。メンバーにいない人は中身のない丸で、`title` は「不明なユーザー」。担当者がいなければ何も出さない | 01「画面の移り方と、共通の表示」(不明なユーザー)。仕様にアバターの中身の定めがない |
| 25 | 作成ダイアログのプロパティ欄は、その種類で使う表示中のすべてのプロパティを、設定の並びで出す。ステータスは初期ステータスを初期値にしたセレクト(詳細パネルのボタンとメニューではない)、優先度は初期優先度を初期値にしたボタンとメニュー。種類を切り替えたときは、使わない欄を隠し、入力済みの値はダイアログを開いているあいだ覚え、使える種類に戻すと値を戻す。「作成」では、表示している欄の値だけを送る。ワークスペースを変えたときは、担当者・開始日・終了日・ポイント・見積もり時間を残し(担当者は、Store が違えばログイン名が一致するメンバーだけ)、優先度・関連システム・追加プロパティ・ステータスを初期値に戻す | 02「ダイアログの中での連動」(テンプレート以外の部分) |
| 26 | 作成ダイアログの検査は、A2 の決定 14 のとおりサーバーに任せ、`Refused` を赤字で出す | A2 の決定 14 |
| 27 | A2 の申し送り「送信中の入力が断りのあと古い値に戻る」は、断りを受けたとき、送った時点の状態ではなく、いまの状態に `error` だけを足して直す。作ったあと(続けて作成)も、送信中に変えた入力を残す | A2 の最終レビューの残り。02「入力の検査」(入力はそのまま残す) |
| 28 | 閲覧のみの Store のアイテムでは、プロパティの入力・ステータスのボタン・Step のチェックを無効にし、「閲覧のみの Store です」を添える | 01「閲覧のみの Store」、02「詳細パネル」の冒頭 |
| 29 | 値の選択肢は、画面がワークスペースの詳細(B2 の `WorkspacesPort.get` の `settings`)から作る。詳細パネルとツリーは、ワークスペースごとに 1 回読み、プロジェクトの画面を読み直すときに読み直す | B2 の web の口を使う(別名の型を作らない) |
| 30 | e2e の追加プロパティは、B2 の API(`add-workspace-property`)でテストの初めに作る。値を変える e2e は、3 幅が同じサーバーを使うので、それぞれ自分の Task を作ってから変える | develop に追加プロパティを作る手段は B2 の API だけ。A2 の決定 21 |
| 31 | item.yaml の日付(`start: 2026-11-11`)を引用符で囲むかどうかは、yaml の書き出しの既定に任せる。保存形式の例は引用符なしだが、`*.yaml` は人が読むための写しで、読み戻した値が同じならよい | 保存形式「いまの値の決め方」(`*.yaml` は `log.jsonl` から作る)。書き出しの細工で 1 か所だけ形を変えない |
| 32 | 追加プロパティの削除(B2 の `DeleteProperty`)と件数(B2 の `Usage`)は、`Choices` だけを見ている。選択式でない値(テキスト・数値・日付・チェック・ユーザー・URL)もアイテムから外し、件数に数えるよう、Task 2 の共有 PR で B2 の 2 つの関数を広げる | 03「プロパティ」の「削除」(値が消える)、「値の入っている N 件」。選択式でない値を足すのはこの計画 |
| 33 | 値を選ぶメニューは ui の新しい部品 `ValueMenu` にする。B2 の `KindsMenu`(設定の画面の中の、開いたまま入り切りするメニュー)は使わない。共有の `Menu` は変えない | 詳細パネル・作成ダイアログ・ツリー、のちのカンバン・テーブル(E)が使う。B2 は `Menu` を変えずに画面の中の部品にした |

### ユーザーの回答(2026-11-13)

| # | 点 | 回答 |
|---|---|---|
| Q1 | 終了日のログの項目名(保存形式の例の `due` と、03・04 のキー `end`) | `end`(推奨どおり)。保存形式の「アイテムの例」の `due` を `end` に直した |
| Q2 | 「複製」を前半・後半のどちらに入れるか | どちらにも入れず A4(推奨どおり。決定 4) |

## 漏れの表(範囲 → タスク、範囲の外 → 持ち主)

| 仕様の節・項目 | 持ち場 |
|---|---|
| 02 プロパティの種類: 組み込み 8 つ・追加 8 型、非表示・使う種類で出さない | Task 1(値)・Task 8(行) |
| 02 同: スプリント・リリースの 2 項目 | トラック F(決定 6) |
| 02 同: 非表示・使う種類を変えても値は消えない | Task 1(値を消さない)・Task 8(出さないだけ) |
| 02 入力の方法と保存: 1 つ変えるごとに 1 回の保存、同じなら保存しない、アクティビティの文 | Task 4 |
| 02 同: 履歴「{プロパティ名}を 旧 → 新」 | 行は Task 4 が書く。履歴の画面は A4 |
| 02 同: 型ごとの入力の方法(表) | Task 7(部品)・Task 9(詳細パネル) |
| 02 同: 検査(終了日・ポイント・見積もり時間) | Task 3(検査)・Task 4・5(呼び出し) |
| 02 同: 時間の値を「N 時間」で出す | Task 3(アクティビティ)。一括変更の確認はトラック E、通知はトラック C |
| 02 同: 「(削除された値)」 | Task 8・9・10・11 |
| 02 同: 担当者の通知・ウォッチ中の人への通知 | トラック C(変更記録から作る) |
| 02 ステータスの変更: メニュー(分類・チェック) | Task 8・9・11 |
| 02 同: 使わないステータス(メニュー・断り) | B5 と、B5 が入ったときの A 側のタスク(決定 3) |
| 02 同: 遷移の制約(メニューの無効・断り) | 後半 A3b(決定 1・2) |
| 02 同: 自動化が変えるとき | B4 |
| 02 同: カンバン・テーブル・一括変更・コマンドパレットからの変更 | トラック E(04)、A5(パレット) |
| 02 アイテムの作成: 初期値のステータス・優先度 | Task 5(サーバー)・Task 10(ダイアログ) |
| 02 同: テンプレートの初期値と当てはめ | 後半 A3b |
| 02 ダイアログの項目: プロパティ | Task 10 |
| 02 同: テンプレート・一緒に作るもの | 後半 A3b |
| 02 同: スプリント | トラック F(決定 6) |
| 02 同: 説明 | A4(決定 6) |
| 02 ダイアログの中での連動: テンプレート以外 | Task 10 |
| 02 同: テンプレートの部分 | 後半 A3b |
| 02 同: プロジェクトを変えたときの、使わないステータスの戻し | B5 と A 側のタスク(決定 3) |
| 02 入力の検査: 日付・ポイント・見積もり時間 | Task 3・5 |
| 02 作成したあと: 一緒に作るもの | 後半 A3b |
| 02 複製 | A4(決定 4) |
| 02 詳細パネル: プロパティの区画、「追加」、「プロパティを追加・変更」 | Task 9 |
| 02 同: 子と Step の数「3/8」、子の行のステータス・担当者、Step のチェック | Task 6(Step のチェックの API)・Task 11(画面) |
| 02 同: Step の追加の担当者・ステータス・優先度 | Task 5 |
| 02 同: Step の削除の「×」 | A4 |
| 04 ツリーの行: 進捗・期限・ポイント・ステータス・担当者 | Task 11 |
| 04 ツリーの行: ブロック中・メニュー | A4 |
| 01 日付が変わったときの自動の更新 | トラック E(決定 22) |
| 保存形式: item.yaml の assignee・start・end・points・estimate・追加プロパティの値 | Task 2 |
| 03 プロパティの削除・件数(選択式でない値) | Task 2(決定 32) |
| PR #159 の申し送り(選択式でない値、型ごとの読み分け) | Task 1 |
| A2 の申し送り(送信中の入力) | Task 10 |


---

## ファイルの地図

```
Go 共有 PR(Task 1・2)
internal/domain/item/item.go              Item に Assignee・Start・End・Points・Estimate・Fields。Create が値を書く
internal/domain/item/values.go            readValues が新しい項目と Fields を読む
internal/domain/item/fields.go            Field・Current・FieldValue(型ごとの読み分け)
internal/app/apptest/sample.go            Sample のアイテムにステータス・優先度
internal/app/apptest/stores_contract_values.go  照合テストに新しい項目(ファイルは増やさない)
internal/infra/gitstore/yaml.go・yaml_notes.go   item.yaml に assignee・start・end・points・estimate と、選択式でない fields(B2 の後)

Go サーバー(Task 3〜6)
internal/domain/item/input.go             ParseInput(入力 → ログの値)、CheckDates
internal/domain/item/format.go            Display(アクティビティの {値})
internal/domain/item/check.go             (変更)CheckCreate の後ろに CheckInputs
internal/app/items/view.go                editContext(アイテム・ID・設定・メンバーを 1 回で読む)
internal/app/items/setproperty.go         SetProperty
internal/app/items/togglevalue.go         AddValue・RemoveValue
internal/app/items/checkstep.go           CheckStep
internal/app/items/create.go              (変更)CreateInput.Values、初期値
internal/app/items/addstep.go             (変更)担当者・ステータス・優先度
internal/adapter/httpapi/items/items.go   (変更)Item の値、CreateItem.Values
internal/adapter/httpapi/items/values.go  set-item-property・add-item-value・remove-item-value・check-step

web 共有 PR(Task 7)
web/packages/ui/src/patterns/ValueMenu.svelte   値を選ぶメニュー(チェック・検索欄・開いたまま)
web/packages/ui/src/patterns/Avatar.svelte      アバター

web(Task 8〜12)
web/packages/core/src/model/item.ts             (変更)値、PropertyInput
web/packages/core/src/port/items.ts             (変更)setProperty・addValue・removeValue・checkStep
web/packages/api/src/items.ts                   (変更)
web/packages/core/src/features/items/properties.ts   プロパティの行・表示・メニューの選択肢
web/packages/core/src/features/items/progress.ts     「完了/対象」と期限の表示
web/packages/core/src/features/items/detail.ts       (変更)設定・メンバー、値の保存、Step のチェック
web/packages/core/src/features/items/createValues.ts 作成ダイアログの値の初期値と連動
web/packages/core/src/features/items/createDialog.ts (変更)values、送信中の入力の扱い
web/packages/core/src/features/projects/screen.ts    (変更)設定を読む、ステータスの変更
web/app/src/screens/item/PropertyField.svelte   1 つのプロパティの入力(詳細パネルと作成ダイアログで共用)
web/app/src/screens/item/StatusButton.svelte    ステータスのボタンとメニュー
web/app/src/screens/item/DetailProperties.svelte 詳細パネルのプロパティの区画
web/app/src/screens/item/CreatePropertyFields.svelte 作成ダイアログのプロパティ欄
web/app/src/screens/item/DetailChildren.svelte  (変更)数・ステータス・担当者・Step のチェック
web/app/src/screens/project/TreeRow.svelte      (変更)進捗・期限・ポイント・ステータス・担当者
web/e2e/properties.spec.ts                      3 幅の通しのテスト
```

## 型と口(タスクをまたいで使う名前)

各タスクの実装者は、自分のタスクしか読まない。境目の名前と型は、ここに書いたものを正とする。

### Go

```go
// internal/domain/item/item.go(Task 1)
type Item struct {
	// …既存(ID・Place・Kind・Rank・Title・Created・Watchers・Status・Priority・Systems・Choices)
	Assignee string                        // ログイン名。なければ空
	Start    string                        // YYYY-MM-DD。なければ空
	End      string                        // YYYY-MM-DD。なければ空
	Points   *float64                      // なければ nil
	Estimate *float64                      // 時間。なければ nil
	Fields   map[change.ID]json.RawMessage // fields/<ID> の set の値(生)。複数選択の配列は入らない。なければ nil
}

// internal/domain/item/fields.go(Task 1)
func Field(p workspace.Property) string                       // 組み込みは p.Key、追加は FieldKey(p.ID)
func Current(it Item, p workspace.Property) json.RawMessage   // ログに書く形。なければ null、複数は配列
func FieldValue(it Item, p workspace.Property) any            // 追加プロパティを型で読む(決定 9)

// internal/domain/item/input.go(Task 3)
func ParseInput(p workspace.Property, in json.RawMessage, st workspace.Settings) (json.RawMessage, error)
func CheckDates(start, end string) error
func ParsePoints(s string) (*float64, error)
func ParseEstimate(s string) (*float64, error)

// internal/domain/item/format.go(Task 3)
type Names func(login string) string // 人の表示名
func Display(p workspace.Property, v json.RawMessage, st workspace.Settings, names Names, today time.Time) string

// internal/domain/item/check.go(Task 5)
func CheckInputs(st workspace.Settings, in map[change.ID]json.RawMessage) (map[change.ID]json.RawMessage, error)

// internal/app/items(Task 4〜5)
func (s Service) SetProperty(ctx context.Context, store port.StoreID, id item.ID, prop change.ID, value json.RawMessage) (item.Item, error)
func (s Service) AddValue(ctx context.Context, store port.StoreID, id item.ID, prop, value change.ID) (item.Item, error)
func (s Service) RemoveValue(ctx context.Context, store port.StoreID, id item.ID, prop, value change.ID) (item.Item, error)
func (s Service) CheckStep(ctx context.Context, store port.StoreID, id item.ID, done bool) (item.Item, error)
type CreateInput struct {
	// …既存(Store・Project・Parent・Kind・Title)
	Values map[change.ID]json.RawMessage // プロパティの内部 ID → 入力(set-item-property の value と同じ形。複数は ID の配列)
}
// Service に Clock を足す(アクティビティの日付の年を決める)
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
	Clock    port.Clock
}
```

API(Task 6):

| operationId | メソッドとパス | 本文 | 返すもの |
|---|---|---|---|
| `set-item-property` | `PUT /api/stores/{store}/items/{item}/properties/{property}` | `{value}`(any) | `Item` |
| `add-item-value` | `POST /api/stores/{store}/items/{item}/properties/{property}/values/{value}` | なし | `Item` |
| `remove-item-value` | `DELETE` 同じパス | なし | `Item` |
| `check-step` | `POST /api/stores/{store}/items/{item}/check` | `{done}` | `Item` |
| `create-item`(変更) | 既存 | `values`(`{プロパティの内部 ID: any}`)を足す | `Item` |

`Item` に足す項目: `status`・`priority`・`assignee`・`start`・`end`(string、空はなし)、`points`・`estimate`(number か null)、`systems`(string[])、`choices`(`{ID: string[]}`)、`fields`(`{ID: any}`)。

### TypeScript(core)

```ts
// model/item.ts(Task 8)
export type PropertyInput = string | boolean | string[] | null;
export interface Item {
  // …既存
  status: string; priority: string; assignee: string; start: string; end: string;
  points: number | null; estimate: number | null; systems: string[];
  choices: Record<string, string[]>; fields: Record<string, unknown>;
}
export interface NewItem { projectId: string; parentId: string | null; kind: Kind; title: string;
  values: Record<string, PropertyInput>; }

// port/items.ts(Task 8)
setProperty(storeId: string, itemId: string, propertyId: string, value: PropertyInput): Promise<Item>;
addValue(storeId: string, itemId: string, propertyId: string, valueId: string): Promise<Item>;
removeValue(storeId: string, itemId: string, propertyId: string, valueId: string): Promise<Item>;
checkStep(storeId: string, itemId: string, done: boolean): Promise<Item>;

// features/items/properties.ts(Task 8)
export interface ValueOption { id: string; label: string; hint?: string; checked: boolean }
export interface PropertyRow {
  property: WorkspaceProperty;   // B2 の型
  extra: boolean;                // 追加プロパティ(「追加」と添える)
  value: PropertyInput;          // 入力欄に入れる値
  text: string;                  // ボタンに出す文字(値の名前・「なし」・「未割り当て」)
  deleted: boolean;              // 「(削除された値)」を薄く出す
  options: ValueOption[];        // メニュー・セレクトの選択肢(選ぶ型だけ)
}
export function visibleProperties(settings: WorkspaceSettings, kind: Kind): WorkspaceProperty[];
export function valueOf(item: Pick<Item, …>, p: WorkspaceProperty): PropertyInput;
export function propertyRows(settings: WorkspaceSettings, kind: Kind, values: (p: WorkspaceProperty) => PropertyInput, people: People): PropertyRow[];
export function statusOptions(settings: WorkspaceSettings, current: string): ValueOption[];
export interface People { members: Member[]; me: string }
export const deletedValue = "(削除された値)";
export const categoryLabels: Record<Category, string>; // 未着手・進行中・完了・中止

// features/items/progress.ts(Task 11)
export function stepProgress(items: Item[], parentId: string, settings: WorkspaceSettings): { done: number; total: number } | null;
export function dueLabel(end: string, category: Category | undefined, today: Date): { text: string; emphasis: "overdue" | "today" | null } | null;
export function formatDate(iso: string, today: Date): string; // 01「日付と時刻」の M/D・YYYY/MM/DD
```

ui(Task 7):

```ts
// ValueMenu: open・x・y・options(ValueOption と同じ形)・multi・onselect(id)・onclose
// Avatar: name?: string(表示名。空なら中身のない丸)・title: string
```

## タスクの一覧と PR

| Task | 中身 | PR | 始められる時期 |
|---|---|---|---|
| 1 | アイテムの値の項目と型ごとの読み分け(共有) | 共有 PR ①(先に develop へ) | いま |
| 2 | item.yaml の新しい項目(共有) | 共有 PR ②(先に develop へ) | B2 のマージ後 |
| 3 | 入力の読み取り・検査・アクティビティの {値} | トラックの PR | Task 1 と B2 のマージの後 |
| 4 | プロパティの変更・複数の値の入り切り | トラックの PR | Task 3 の後 |
| 5 | 作成の値と初期値、Step の追加の値、Step のチェック | トラックの PR | Task 4 の後 |
| 6 | API | トラックの PR | Task 5 の後 |
| 7 | ui の `ValueMenu`・`Avatar`(共有) | 共有 PR ③(先に develop へ) | B2 のマージ後 |
| 8 | core の型・口・プロパティの行 | トラックの PR | Task 6・7 と B2 のマージの後 |
| 9 | 詳細パネルのプロパティ | トラックの PR | Task 8 の後 |
| 10 | 作成ダイアログのプロパティと連動、A2 の申し送り | トラックの PR | Task 8 の後 |
| 11 | ツリーの行と詳細パネルの子の行、Step のチェック | トラックの PR | Task 9 の後 |
| 12 | e2e(3 幅) | トラックの PR | Task 9〜11 の後 |

トラックの PR は 1 本(`feat/track-a3a`、向き先 develop)。共有 PR は `feat/track-a3a-item-values`・`feat/track-a3a-yaml`・`feat/track-a3a-ui` の名前で develop から切り、入ったらトラックのブランチに develop を取り込む。

---

### Task 1: 共有: アイテムの値の項目と、型ごとの読み分け

**ブランチ:** develop から `feat/track-a3a-item-values`。この PR だけで develop へ入れる。

**Files:**
- Modify: `internal/domain/item/item.go`(`Item` の項目、`Create`)
- Modify: `internal/domain/item/values.go`(`readValues`)
- Create: `internal/domain/item/fields.go`
- Test: `internal/domain/item/fields_test.go`、`internal/domain/item/values_test.go`(追記)
- Modify: `internal/app/apptest/sample.go`、`internal/app/apptest/stores_contract_values.go`(ファイルは増やさない。`apptest` は 20 ファイルで上限)

**Interfaces:**
- Consumes: `workspace.Property`・`workspace.Type*`(develop)、`item.FieldKey`・`MultiChoice`(#159)
- Produces: `Item.Assignee`・`Start`・`End`・`Points`・`Estimate`・`Fields`、`item.Field`・`item.Current`・`item.FieldValue`、`apptest.SamplePriorityMedium` を Sample のアイテムが持つ

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/item/fields_test.go`:

```go
package item_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	fNote  change.ID = "01PROP0000000000000000000C" // テキスト
	fCount change.ID = "01PROP0000000000000000000D" // 数値
	fOwner change.ID = "01PROP0000000000000000000E" // ユーザー
	fDone  change.ID = "01PROP0000000000000000000F" // チェック
)

func prop(id change.ID, typ workspace.PropertyType) workspace.Property {
	return workspace.Property{ID: id, Key: string(id), Type: typ}
}

func TestBuiltinValuesRoundTrip(t *testing.T) {
	three, half := 3.0, 1.5
	base := item.Item{
		ID: "01ITEM0000000000000000000A", Kind: item.Task, Title: "x",
		Status: sTodo, Priority: pMedium, Assignee: "taro", Start: "2026-11-11", End: "2026-11-15",
		Points: &three, Estimate: &half, Systems: []change.ID{yWeb},
	}
	got, ok := item.FromLog(rows(item.Create(base)))
	if !ok || got.Assignee != "taro" || got.Start != "2026-11-11" || got.End != "2026-11-15" ||
		*got.Points != 3 || *got.Estimate != 1.5 || got.Status != sTodo || got.Priority != pMedium {
		t.Errorf("FromLog = %+v", got)
	}
}

func TestEmptyValuesAreNotWritten(t *testing.T) {
	d := item.Create(item.Item{ID: "01ITEM0000000000000000000A", Kind: item.Task, Title: "x"})
	var to map[string]json.RawMessage
	_ = json.Unmarshal(d.To, &to)
	for _, k := range []string{"assignee", "start", "end", "points", "estimate", "systems"} {
		if _, ok := to[k]; ok {
			t.Errorf("create wrote %s", k)
		}
	}
}

// 内部 ID の形をしたテキストとログイン名も、選択肢ではなくテキスト・ユーザーとして読める(決定 9)。
func TestFieldValueReadsByTheSettingsType(t *testing.T) {
	idShaped := json.RawMessage(`"01CHOICE00000000000000000A"`)
	base := item.Item{ID: "01ITEM0000000000000000000A", Kind: item.Task, Title: "x",
		Fields: map[change.ID]json.RawMessage{
			fNote: idShaped, fCount: json.RawMessage(`2.5`), fOwner: json.RawMessage(`"hanako"`),
			fDone: json.RawMessage(`true`), fEnv: json.RawMessage(`"` + string(vProd) + `"`),
			fTags: json.RawMessage(`["` + string(vA) + `","` + string(vB) + `"]`),
		}}
	it, _ := item.FromLog(rows(item.Create(base)))
	cases := []struct {
		p    workspace.Property
		want any
	}{
		{prop(fNote, workspace.TypeText), "01CHOICE00000000000000000A"},
		{prop(fCount, workspace.TypeNumber), ptr(2.5)},
		{prop(fOwner, workspace.TypeUser), "hanako"},
		{prop(fDone, workspace.TypeCheck), true},
		{prop(fEnv, workspace.TypeSingle), vProd},
		{prop(fTags, workspace.TypeMulti), []change.ID{vA, vB}},
		{prop("01PROP0000000000000000000Z", workspace.TypeCheck), false},
	}
	for _, c := range cases {
		if got := item.FieldValue(it, c.p); !reflect.DeepEqual(got, c.want) {
			t.Errorf("FieldValue(%s) = %#v, want %#v", c.p.Type, got, c.want)
		}
	}
}

func TestCurrentIsTheLogForm(t *testing.T) {
	half := 1.5
	it := item.Item{Assignee: "taro", Estimate: &half, Systems: []change.ID{yWeb},
		Fields: map[change.ID]json.RawMessage{fNote: json.RawMessage(`"memo"`)}}
	cases := map[string]struct {
		p    workspace.Property
		want string
	}{
		"assignee": {workspace.Property{ID: "assignee", Key: "assignee", Builtin: true, Type: workspace.TypeUser}, `"taro"`},
		"start":    {workspace.Property{ID: "start", Key: "start", Builtin: true, Type: workspace.TypeDate}, `null`},
		"estimate": {workspace.Property{ID: "estimate", Key: "estimate", Builtin: true, Type: workspace.TypeDuration}, `1.5`},
		"systems":  {workspace.Property{ID: "systems", Key: "systems", Builtin: true, Type: workspace.TypeSystems}, `["` + string(yWeb) + `"]`},
		"text":     {prop(fNote, workspace.TypeText), `"memo"`},
		"check":    {prop(fDone, workspace.TypeCheck), `false`},
	}
	for name, c := range cases {
		if got := string(item.Current(it, c.p)); got != c.want {
			t.Errorf("%s: Current = %s, want %s", name, got, c.want)
		}
		if name == "text" && item.Field(c.p) != "fields/"+string(fNote) {
			t.Errorf("Field = %s", item.Field(c.p))
		}
	}
}

func ptr(f float64) *float64 { return &f }
```

`internal/app/apptest/stores_contract_values.go` の `contractItemValues` の書き込みに、`change.NewSet("assignee", nil, "taro")`、`change.NewSet("end", nil, "2026-11-15")`、`change.NewSet("estimate", nil, 1.5)`、`change.NewSet(item.FieldKey(note), nil, "memo")`(`note` は新しい定数 `01CONTRACTNT00000000000001`)を足し、読んだ `it` の `Assignee == "taro"`・`End == "2026-11-15"`・`*Estimate == 1.5`・`string(it.Fields[note]) == `"memo"`` も確かめる。

`internal/app/apptest/sample.go` の `epic` と `task` に `Status: SampleStatusTodo, Priority: SamplePriorityMedium` を足す(決定 11)。値の定数に `SampleSystemWeb change.ID = "01SAMPLEVAL000000000000011"`(関連システムの先頭。ステータス 5・優先度 5 の次)を足す。B2 本体のテストと e2e は自分で作ったワークスペースを使うので(`feat/track-b2` で確かめた)、Sample の値が増えても B2 の件数のテストは変わらない。`sample` を読むテスト(`apptest` の `stores_test.go`)が `Status` を確かめるように、1 行足す。

- [ ] **Step 2: テストが失敗するのを確かめる**

Run: `go test ./internal/domain/item/... ./internal/app/apptest/...`
Expected: FAIL(`Assignee` などの項目と `FieldValue` がない)

- [ ] **Step 3: 実装する**

`internal/domain/item/item.go` の `Item` に項目を足す(上の「型と口」のとおり)。`Create` を次にする:

```go
// Create は、アイテムの作成の変更。値のない項目は書かない(読むときは空として扱う)。
// 作成の時刻と人は、変更記録の at と by から取る。種類と親の組み合わせは検査しない。呼ぶ側が先に CanPlace で確かめる。
func Create(it Item) change.Draft {
	to := map[string]any{
		"id": it.ID, "place": it.Place, "kind": it.Kind, "rank": it.Rank, "title": it.Title,
		"watchers": it.Watchers,
	}
	put := func(k string, v any, ok bool) {
		if ok {
			to[k] = v
		}
	}
	put("status", it.Status, it.Status != "")
	put("priority", it.Priority, it.Priority != "")
	put("assignee", it.Assignee, it.Assignee != "")
	put("start", it.Start, it.Start != "")
	put("end", it.End, it.End != "")
	put("points", it.Points, it.Points != nil)
	put("estimate", it.Estimate, it.Estimate != nil)
	put("systems", it.Systems, len(it.Systems) > 0)
	for p, raw := range it.Fields {
		to[FieldKey(p)] = raw
	}
	return change.NewCreate(to)
}
```

`internal/domain/item/values.go` の `readValues` に足す:

```go
	it.Assignee = str(s.Values["assignee"])
	it.Start = str(s.Values["start"])
	it.End = str(s.Values["end"])
	it.Points = num(s.Values["points"])
	it.Estimate = num(s.Values["estimate"])
	for f, raw := range s.Values {
		if strings.HasPrefix(f, fieldsPrefix) && string(raw) != "null" {
			if it.Fields == nil {
				it.Fields = map[change.ID]json.RawMessage{}
			}
			it.Fields[change.ID(strings.TrimPrefix(f, fieldsPrefix))] = raw
		}
	}
```

```go
// str は、文字列の値。null や文字列でない値は空。
func str(raw json.RawMessage) string {
	var v string
	if json.Unmarshal(raw, &v) != nil {
		return ""
	}
	return v
}

// num は、数の値。null や数でない値は nil。
func num(raw json.RawMessage) *float64 {
	var v float64
	if len(raw) == 0 || string(raw) == "null" || json.Unmarshal(raw, &v) != nil {
		return nil
	}
	return &v
}
```

`internal/domain/item/fields.go`:

```go
package item

import (
	"encoding/json"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Field は、プロパティ p の値を持つログの項目名(決定 7)。組み込みはキー、追加は fields/<ID>。
func Field(p workspace.Property) string {
	if p.Builtin {
		return p.Key
	}
	return FieldKey(p.ID)
}

// Current は、p のいまの値を、ログに書く形(JSON)で返す。値がなければ null(チェックは false)。
// 関連システムと複数選択は、値の内部 ID の配列。
func Current(it Item, p workspace.Property) json.RawMessage {
	if p.Builtin {
		return builtinValue(it, p.Key)
	}
	switch p.Type {
	case workspace.TypeMulti:
		return change.Value(nonNil(it.Choices[p.ID]))
	case workspace.TypeCheck:
		return change.Value(FieldValue(it, p))
	}
	if raw, ok := it.Fields[p.ID]; ok {
		return raw
	}
	return json.RawMessage("null")
}

func builtinValue(it Item, key string) json.RawMessage {
	opt := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	switch key {
	case "status":
		return change.Value(opt(string(it.Status)))
	case "priority":
		return change.Value(opt(string(it.Priority)))
	case "assignee":
		return change.Value(opt(it.Assignee))
	case "start":
		return change.Value(opt(it.Start))
	case "end":
		return change.Value(opt(it.End))
	case "points":
		return change.Value(it.Points)
	case "estimate":
		return change.Value(it.Estimate)
	case "systems":
		return change.Value(nonNil(it.Systems))
	}
	return json.RawMessage("null")
}

// FieldValue は、追加プロパティ p の値を p の型で読む(決定 9)。テキスト・URL・日付・ユーザーは string、
// 数値は *float64、チェックは bool、単一選択は change.ID、複数選択は []change.ID。値がなければゼロ値。
func FieldValue(it Item, p workspace.Property) any {
	raw := it.Fields[p.ID]
	switch p.Type {
	case workspace.TypeSingle:
		if vs := it.Choices[p.ID]; len(vs) > 0 {
			return vs[0]
		}
		return change.ID("")
	case workspace.TypeMulti:
		return it.Choices[p.ID]
	case workspace.TypeNumber:
		return num(raw)
	case workspace.TypeCheck:
		var b bool
		_ = json.Unmarshal(raw, &b)
		return b
	}
	return str(raw)
}

func nonNil(ids []change.ID) []change.ID {
	if ids == nil {
		return []change.ID{}
	}
	return ids
}
```

`FieldValue` の複数選択で値がないときは `nil`(`[]change.ID(nil)`)を返す。テストの期待もそれに合わせる。

- [ ] **Step 4: テストが通るのを確かめる**

Run: `go test ./internal/domain/item/... ./internal/app/apptest/... ./internal/infra/gitstore/...`
Expected: PASS(gitstore は照合テストを持つ)

- [ ] **Step 5: `make check` を通してコミットし、PR を出す**

```bash
make check
git add internal/domain/item internal/app/apptest
git commit -m "feat(item): 組み込みの値と選択式でない追加プロパティの値を持ち、設定の型で読み分ける(A3 前半 Task 1)"
git push -u origin feat/track-a3a-item-values
gh pr create --base develop --title "アイテムの値の項目と型ごとの読み分け(トラック A3 前半の共有部分)" --body-file "$SCRATCH/pr-body.md"   # $SCRATCH はセッションのスクラッチパッド
```

PR の本文は、中身(`Item` の項目・`Field`・`Current`・`FieldValue`・Sample)、決めたこと(この計画の決定 7〜9・11)、`make check` の結果、末尾の生成の印。PR がマージされたら、`feat/track-a3a` を develop から切る(B2 が入っていれば B2 の後の develop)。

---

### Task 2: 共有: item.yaml の新しい項目と、B2 のプロパティの削除・件数

**ブランチ:** B2 本体が develop に入ってから、develop から `feat/track-a3a-yaml`。

**Files:**
- Modify: `internal/infra/gitstore/yaml.go`(`itemYAML` に項目)・`yaml_notes.go`(`itemToYAML`・`fieldsNode`)
- Test: `internal/infra/gitstore/yaml_test.go`(あるファイルに追記)
- Modify: `internal/app/workspaces/property_delete.go`(`clearItems`)・`usage.go`(`count`)。B の持ち場だが、選択式でない値を足すのはこの計画なので、同じ共有 PR で B2 の処理を広げる(決定 32)
- Test: `internal/app/workspaces/property_test.go`・`usage_test.go`(追記)

**Interfaces:**
- Consumes: Task 1 の `Item` の項目、B2 の `itemToYAML(it, log, settings)`・`fieldsNode`・`comment`
- Produces: item.yaml の `assignee`・`start`・`end`・`points`・`estimate`、`fields` の選択式でない値。追加プロパティの削除で選択式でない値も消え、件数にも入る

- [ ] **Step 1: 失敗するテストを書く**

B2 の `yaml_notes.go` のテスト(`TestItemYAMLNotes` など、B2 のマージ後の名前を使う)に倣い、次のアイテムの `renderYAML` の結果を比べるテストを足す。

```go
func TestItemYAMLHasPlainValues(t *testing.T) {
	three, half := 3.0, 1.5
	it := item.Item{ID: "01ITEM0000000000000000000A", Kind: item.Task, Title: "x",
		Assignee: "taro", Start: "2026-11-11", End: "2026-11-15", Points: &three, Estimate: &half,
		Fields: map[change.ID]json.RawMessage{"01PROP0000000000000000000C": json.RawMessage(`"memo"`)}}
	out, err := renderYAML(change.KindItem, rows(item.Create(it)), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"assignee": "taro", "start": "2026-11-11", "end": "2026-11-15", "points": 3, "estimate": 1.5}
	for k, v := range want {
		if fmt.Sprint(got[k]) != fmt.Sprint(v) {
			t.Errorf("%s = %v, want %v\n%s", k, got[k], v, out)
		}
	}
	if f, _ := got["fields"].(map[string]any); f["01PROP0000000000000000000C"] != "memo" {
		t.Errorf("fields = %v", got["fields"])
	}
}
```

日付を引用符で囲むかどうかは、yaml の書き出しの既定に任せる(決定 31)。テストは読み戻した値で比べる。

(`rows` がこのパッケージにないときは、`change.Stamp` で 1 行を作る小さな関数をテストに置く。)

B2 のテストに倣い、`internal/app/workspaces/property_test.go` に足す:

```go
// テキストの追加プロパティを消すと、選択式でない値も外れる(03「プロパティ」の「削除」、決定 32)。
func TestDeletePropertyClearsPlainValues(t *testing.T) {
	s := setup(t)
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	memo, _ := s.AddProperty(ctx, store, w.ID, workspaces.NewProperty{Name: "メモ", Key: "memo", Type: workspace.TypeText})
	id := putItem(t, s, w.ID, change.NewSet(item.FieldKey(memo.ID), nil, "下書き"))
	u, _ := s.Usage(ctx, store, w.ID)
	if u.Properties[memo.ID] != 1 {
		t.Errorf("usage = %+v", u.Properties)
	}
	if err := s.DeleteProperty(ctx, store, w.ID, memo.ID); err != nil {
		t.Fatal(err)
	}
	if it := itemOf(t, s, id); it.Fields != nil {
		t.Errorf("fields = %v", it.Fields)
	}
}
```

- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/infra/gitstore/ ./internal/app/workspaces/` / Expected: FAIL

- [ ] **Step 3: 実装する**

`itemYAML` に、保存形式「アイテムの例」の並び(status の次に assignee、priority の次に start・end・points)と、`estimate` を `relations` の位置(いまはないので `fields` の後)に足す:

```go
	Assignee *string  `yaml:"assignee"`
	Start    *string  `yaml:"start"`
	End      *string  `yaml:"end"`
	Points   *float64 `yaml:"points"`
	Estimate *float64 `yaml:"estimate"`
```

`itemToYAML` で、空の文字列は `nil`(`null` を出す)にして入れる。

`property_delete.go` の `clearItems` は、`it.Choices[p.ID]` に加えて、`it.Fields[p.ID]` に値があり選択式でないときに `change.NewSet(item.FieldKey(p.ID), it.Fields[p.ID], nil)` を書く。`usage.go` の `count` は、`it.Fields` のプロパティ(値が null でないもの)も `Properties` に数える(`Choices` と同じプロパティは 1 回だけ)。
`fieldsNode` は、`it.Choices` のプロパティに加えて、`it.Fields` にあり `it.Choices` にないプロパティを、生の値(`yaml.Unmarshal` した値のノード)で出す。並びはプロパティの ID の辞書順のまま。選択式でない値には注釈を付けない。

- [ ] **Step 4: 通るのを確かめる** — Run: `go test ./internal/infra/gitstore/... ./internal/app/workspaces/...` / Expected: PASS
- [ ] **Step 5: `make check`、コミット、PR**(タイトル「item.yaml の新しい値と、選択式でない追加プロパティの削除・件数(トラック A3 前半の共有部分)」。B2 の持ち主に、`property_delete.go`・`usage.go` を変えたことを本文で知らせる)。マージ後、`feat/track-a3a` に develop を取り込む。

---

### Task 3: 入力の読み取り・検査・アクティビティの {値}

**Files:**
- Create: `internal/domain/item/input.go`、`internal/domain/item/format.go`
- Test: `internal/domain/item/input_test.go`、`internal/domain/item/format_test.go`

**Interfaces:**
- Consumes: Task 1 の `Field`・`Current`、`workspace.Settings`・B2 の `Settings.Property`・`Property.Choices`
- Produces: `ParseInput`・`CheckDates`・`ParsePoints`・`ParseEstimate`・`Display`・`Names`

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/item/input_test.go`:

```go
package item_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func refusedWith(t *testing.T, err error, want string) {
	t.Helper()
	var r *refusal.Refusal
	if !errors.As(err, &r) || r.Message != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

func TestParseEstimate(t *testing.T) {
	for in, want := range map[string]string{"1.5": "1.5", "0": "0", "2.25": "2.25", "": "null", " 3 ": "3"} {
		v, err := item.ParseEstimate(in)
		if err != nil || string(mustJSON(v)) != want {
			t.Errorf("ParseEstimate(%q) = %v, %v", in, v, err)
		}
	}
	_, err := item.ParseEstimate("-1")
	refusedWith(t, err, "見積もり時間は 0 以上の数で入力してください")
	_, err = item.ParseEstimate("abc")
	refusedWith(t, err, "見積もり時間は 0 以上の数で入力してください")
	_, err = item.ParseEstimate("0.123")
	refusedWith(t, err, "見積もり時間は小数第 2 位までで入力してください")
}

func TestParsePoints(t *testing.T) {
	if v, err := item.ParsePoints("2.5"); err != nil || *v != 2.5 {
		t.Errorf("ParsePoints = %v, %v", v, err)
	}
	for _, in := range []string{"-0.5", "x"} {
		_, err := item.ParsePoints(in)
		refusedWith(t, err, "ポイントは 0 以上の数で入力してください")
	}
}

func TestCheckDates(t *testing.T) {
	if item.CheckDates("2026-11-11", "2026-11-11") != nil || item.CheckDates("", "2026-11-01") != nil {
		t.Error("same day and one-sided must pass")
	}
	refusedWith(t, item.CheckDates("2026-11-11", "2026-11-10"), "終了日は開始日以降にしてください")
}

// byKey は、設定の組み込みのプロパティ(キー k)。
func byKey(t *testing.T, st workspace.Settings, k string) workspace.Property {
	t.Helper()
	for _, p := range st.Properties {
		if p.Key == k {
			return p
		}
	}
	t.Fatalf("no %s", k)
	return workspace.Property{}
}

func TestParseInputNormalizes(t *testing.T) {
	st := workspace.DefaultSettings(seqIDs())
	byKey := func(k string) workspace.Property { return byKey(t, st, k) }
	cases := []struct {
		p    workspace.Property
		in   string
		want string
	}{
		{byKey("status"), `"` + string(st.Statuses[1].ID) + `"`, `"` + string(st.Statuses[1].ID) + `"`},
		{byKey("assignee"), `""`, `null`},
		{byKey("assignee"), `"taro"`, `"taro"`},
		{byKey("start"), `"2026-11-11"`, `"2026-11-11"`},
		{byKey("start"), `null`, `null`},
		{byKey("points"), `"3"`, `3`},
		{byKey("estimate"), `"1.50"`, `1.5`},
		{workspace.Property{ID: fNote, Type: workspace.TypeText}, `"  memo  "`, `"memo"`},
		{workspace.Property{ID: fNote, Type: workspace.TypeURL}, `"   "`, `null`},
		{workspace.Property{ID: fDone, Type: workspace.TypeCheck}, `true`, `true`},
	}
	for _, c := range cases {
		got, err := item.ParseInput(c.p, json.RawMessage(c.in), st)
		if err != nil || string(got) != c.want {
			t.Errorf("ParseInput(%s, %s) = %s, %v; want %s", c.p.Key, c.in, got, err, c.want)
		}
	}
}

func TestParseInputRejectsUnknownValues(t *testing.T) {
	st := workspace.DefaultSettings(seqIDs())
	status := st.Properties[0]
	if _, err := item.ParseInput(status, json.RawMessage(`"01NOPE00000000000000000000"`), st); !errors.Is(err, item.ErrNoSuchValue) {
		t.Errorf("err = %v", err)
	}
	if _, err := item.ParseInput(status, json.RawMessage(`null`), st); !errors.Is(err, item.ErrNoSuchValue) {
		t.Errorf("status cannot be empty: %v", err)
	}
	date := workspace.Property{Key: "start", Builtin: true, Type: workspace.TypeDate}
	if _, err := item.ParseInput(date, json.RawMessage(`"11/11"`), st); !errors.Is(err, item.ErrBadInput) {
		t.Errorf("err = %v", err)
	}
}
```

`input_test.go` に次の小さな関数も置く:

```go
func seqIDs() func() change.ID {
	n := 0
	return func() change.ID { n++; return change.ID(fmt.Sprintf("01SEQ%021d", n)) }
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func raw(s string) json.RawMessage { return json.RawMessage(s) }
```

`internal/domain/item/format_test.go`:

```go
func TestDisplay(t *testing.T) {
	st := workspace.DefaultSettings(seqIDs())
	names := func(login string) string {
		if login == "taro" {
			return "太郎"
		}
		return login
	}
	today := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	p := func(k string) workspace.Property { return byKey(t, st, k) }
	cases := []struct {
		p    workspace.Property
		v    string
		want string
	}{
		{p("status"), `"` + string(st.Statuses[2].ID) + `"`, "Done"},
		{p("status"), `"01GONE00000000000000000000"`, "(削除された値)"},
		{p("assignee"), `"taro"`, "太郎"},
		{p("assignee"), `null`, "なし"},
		{p("start"), `"2026-11-11"`, "11/11"},
		{p("end"), `"2027-01-05"`, "2027/01/05"},
		{p("estimate"), `1.5`, "1.5 時間"},
		{p("estimate"), `2`, "2 時間"},
		{p("points"), `3`, "3"},
		{p("systems"), `["` + string(st.Systems[0].ID) + `","` + string(st.Systems[1].ID) + `"]`, "Web, API"},
		{p("systems"), `[]`, "なし"},
		{workspace.Property{Type: workspace.TypeCheck}, `true`, "オン"},
		{workspace.Property{Type: workspace.TypeCheck}, `false`, "オフ"},
		{workspace.Property{Type: workspace.TypeText}, `"memo"`, "memo"},
	}
	for _, c := range cases {
		if got := item.Display(c.p, json.RawMessage(c.v), st, names, today); got != c.want {
			t.Errorf("Display(%s, %s) = %q, want %q", c.p.Key, c.v, got, c.want)
		}
	}
}
```

- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/domain/item/...` / Expected: FAIL(関数がない)

- [ ] **Step 3: 実装する**

`internal/domain/item/input.go`:

```go
package item

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ErrNoSuchValue は、選択肢にない値を指したとき(決定 18。API は 404)。
var ErrNoSuchValue = errors.New("no such value")

// ErrBadInput は、型に合わない入力(日付の形でない、数値型の数でない値など)。仕様の文言はない(決定 16)。
var ErrBadInput = errors.New("bad input")

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// CheckDates は、開始日と終了日が両方あり、終了日が開始日より前なら断る(02「入力の方法と保存」)。
func CheckDates(start, end string) error {
	if start != "" && end != "" && end < start {
		return refusal.New("終了日は開始日以降にしてください")
	}
	return nil
}

// ParsePoints は、ポイントの入力を読む。空は nil。0 未満と数でない値は断る(決定 16)。
func ParsePoints(s string) (*float64, error) {
	v, ok, empty := parseNumber(s)
	if empty {
		return nil, nil //nolint:nilnil // 空は値なし
	}
	if !ok || v < 0 {
		return nil, refusal.New("ポイントは 0 以上の数で入力してください")
	}
	return &v, nil
}

// ParseEstimate は、見積もり時間の入力を読む。空は nil(02「入力の方法と保存」の検査の表)。
func ParseEstimate(s string) (*float64, error) {
	v, ok, empty := parseNumber(s)
	if empty {
		return nil, nil //nolint:nilnil // 空は値なし
	}
	if !ok || v < 0 {
		return nil, refusal.New("見積もり時間は 0 以上の数で入力してください")
	}
	if math.Abs(v*100-math.Round(v*100)) > 1e-9 {
		return nil, refusal.New("見積もり時間は小数第 2 位までで入力してください")
	}
	return &v, nil
}

func parseNumber(s string) (v float64, ok, empty bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false, true
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil && !math.IsInf(v, 0) && !math.IsNaN(v), false
}

// ParseInput は、プロパティ p への入力 in を、ログに書く値にする(決定 8・12)。
// 関連システムと複数選択は入り切りの操作で扱うので、ここでは ErrBadInput。
func ParseInput(p workspace.Property, in json.RawMessage, st workspace.Settings) (json.RawMessage, error) {
	switch p.Type {
	case workspace.TypeStatus:
		return pickValue(in, st.Statuses, false)
	case workspace.TypePriority:
		return pickValue(in, st.Priorities, false)
	case workspace.TypeSingle:
		return pickValue(in, choicesOf(st, p.ID), true)
	case workspace.TypeDate:
		s := text(in)
		if s != "" && !validDate(s) {
			return nil, ErrBadInput
		}
		return orNull(s), nil
	case workspace.TypeNumber, workspace.TypeDuration:
		return parseNumberInput(p, in)
	case workspace.TypeCheck:
		var b bool
		if json.Unmarshal(in, &b) != nil {
			return nil, ErrBadInput
		}
		return change.Value(b), nil
	case workspace.TypeUser, workspace.TypeText, workspace.TypeURL:
		return orNull(text(in)), nil
	}
	return nil, ErrBadInput
}

func parseNumberInput(p workspace.Property, in json.RawMessage) (json.RawMessage, error) {
	s := text(in)
	var v *float64
	var err error
	switch {
	case p.Key == "estimate" && p.Builtin:
		v, err = ParseEstimate(s)
	case p.Key == "points" && p.Builtin:
		v, err = ParsePoints(s)
	default:
		n, ok, empty := parseNumber(s)
		if !empty && !ok {
			return nil, ErrBadInput
		}
		if !empty {
			v = &n
		}
	}
	if err != nil {
		return nil, err
	}
	return change.Value(v), nil
}

// pickValue は、in が vs のどれかの内部 ID なら、その ID の値を返す。nullable なら null と空も認める。
func pickValue(in json.RawMessage, vs []workspace.Value, nullable bool) (json.RawMessage, error) {
	id := change.ID(text(in))
	if id == "" && nullable {
		return json.RawMessage("null"), nil
	}
	for _, v := range vs {
		if v.ID == id {
			return change.Value(id), nil
		}
	}
	return nil, ErrNoSuchValue
}

func choicesOf(st workspace.Settings, p change.ID) []workspace.Value {
	if prop, ok := st.Property(p); ok {
		return prop.Choices
	}
	return nil
}

// text は、文字列の入力の前後の空白を除いたもの。null と文字列でない値は空。
func text(in json.RawMessage) string {
	var s string
	if json.Unmarshal(in, &s) != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func orNull(s string) json.RawMessage {
	if s == "" {
		return json.RawMessage("null")
	}
	return change.Value(s)
}

func validDate(s string) bool {
	if !datePattern.MatchString(s) {
		return false
	}
	_, err := time.Parse(time.DateOnly, s)
	return err == nil
}
```

`internal/domain/item/format.go`:

```go
package item

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Names は、ログイン名から人の表示名を返す(メンバーにいなければログイン名)。
type Names func(login string) string

const deletedValue = "(削除された値)"

// Display は、アクティビティの {値} を作る(決定 15、01「画面の移り方と、共通の表示」)。
// v はログに書く形(Current・ParseInput の結果)。today は保存する日(日付の年を決める)。
func Display(p workspace.Property, v json.RawMessage, st workspace.Settings, names Names, today time.Time) string {
	if string(v) == "null" || len(v) == 0 {
		if p.Type == workspace.TypeCheck {
			return "オフ"
		}
		return "なし"
	}
	switch p.Type {
	case workspace.TypeStatus:
		return valueName(st.Statuses, change.ID(text(v)))
	case workspace.TypePriority:
		return valueName(st.Priorities, change.ID(text(v)))
	case workspace.TypeSingle:
		return valueName(choicesOf(st, p.ID), change.ID(text(v)))
	case workspace.TypeSystems:
		return joinNames(st.Systems, v)
	case workspace.TypeMulti:
		return joinNames(choicesOf(st, p.ID), v)
	case workspace.TypeUser:
		return names(text(v))
	case workspace.TypeDate:
		return dateText(text(v), today)
	case workspace.TypeCheck:
		var b bool
		_ = json.Unmarshal(v, &b)
		return map[bool]string{true: "オン", false: "オフ"}[b]
	case workspace.TypeDuration:
		return Hours(num(v)) + " 時間"
	case workspace.TypeNumber:
		return strconv.FormatFloat(*num(v), 'f', -1, 64)
	}
	return text(v)
}

// Hours は、時間を小数第 2 位までで、末尾の 0 を省いて書く(02「時間の値」)。
func Hours(v *float64) string {
	return strconv.FormatFloat(float64(int64(*v*100+0.5))/100, 'f', -1, 64)
}

func valueName(vs []workspace.Value, id change.ID) string {
	for _, v := range vs {
		if v.ID == id {
			return v.Name
		}
	}
	return deletedValue
}

func joinNames(vs []workspace.Value, v json.RawMessage) string {
	var ids []change.ID
	_ = json.Unmarshal(v, &ids)
	if len(ids) == 0 {
		return "なし"
	}
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = valueName(vs, id)
	}
	return strings.Join(names, ", ")
}

// dateText は、01「日付と時刻」の表示(今年なら M/D、ほかの年は YYYY/MM/DD)。
func dateText(s string, today time.Time) string {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return s
	}
	if d.Year() == today.Year() {
		return strconv.Itoa(int(d.Month())) + "/" + strconv.Itoa(d.Day())
	}
	return d.Format("2006/01/02")
}
```

- [ ] **Step 4: 通るのを確かめる** — Run: `go test ./internal/domain/item/...` / Expected: PASS
- [ ] **Step 5: `make check`、コミット** — `git commit -m "feat(item): プロパティの入力の読み取り・検査と、アクティビティの値の表示を足す(A3 前半 Task 3)"`

---

### Task 4: プロパティの変更と、複数の値の入り切り

**Files:**
- Create: `internal/app/items/view.go`、`internal/app/items/setproperty.go`、`internal/app/items/togglevalue.go`
- Modify: `internal/app/items/service.go`(`Clock` を足す)、`internal/adapter/httpapi/routes.go`(`Clock: deps.Clock` の 1 語。共有のファイルだが A の Register の行だけを変える)
- Test: `internal/app/items/values_test.go`

**Interfaces:**
- Consumes: Task 1 の `Field`・`Current`、Task 3 の `ParseInput`・`CheckDates`・`Display`、`port.Clock`
- Produces: `Service.SetProperty`・`AddValue`・`RemoveValue`、`editContext`(Task 5 も使う)

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/items/values_test.go`(`edit_test.go` の `setupRec`・`logLen`・`last` を使う。`setupRec` の `items.Service` に `Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))` を足す。`items_test.go` の組み立ても同じ):

```go
package items_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

func raw(s string) json.RawMessage { return json.RawMessage(s) }

func TestSetPropertySavesOneChange(t *testing.T) {
	svc, rec := setupRec(t)
	it, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"`+string(apptest.SampleStatusDone)+`"`))
	if err != nil || it.Status != apptest.SampleStatusDone {
		t.Fatalf("SetProperty = %+v, %v", it, err)
	}
	if last(rec) != "HH-2 のステータスを Done に変更" {
		t.Errorf("summary = %q", last(rec))
	}
	n := logLen(t, rec, apptest.SampleTask)
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"`+string(apptest.SampleStatusDone)+`"`)); err != nil {
		t.Fatal(err)
	}
	if logLen(t, rec, apptest.SampleTask) != n {
		t.Error("the same value must not be saved")
	}
}

func TestSetPropertyWritesTheValueText(t *testing.T) {
	svc, rec := setupRec(t)
	cases := []struct{ prop, in, want string }{
		{"assignee", `"taro"`, "HH-2 の担当者を taro に変更"},
		{"assignee", `""`, "HH-2 の担当者を なし に変更"},
		{"start", `"2026-11-11"`, "HH-2 の開始日を 11/11 に変更"},
		{"estimate", `"1.25"`, "HH-2 の見積もり時間を 1.25 時間 に変更"},
		{"points", `"3"`, "HH-2 のポイントを 3 に変更"},
	}
	for _, c := range cases {
		if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, change.ID(c.prop), raw(c.in)); err != nil {
			t.Fatalf("%s: %v", c.prop, err)
		}
		if last(rec) != c.want {
			t.Errorf("summary = %q, want %q", last(rec), c.want)
		}
	}
}

func TestSetPropertyRefusesBadDatesAndNumbers(t *testing.T) {
	svc, rec := setupRec(t)
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "start", raw(`"2026-11-11"`)); err != nil {
		t.Fatal(err)
	}
	n := logLen(t, rec, apptest.SampleTask)
	cases := []struct{ prop, in, want string }{
		{"end", `"2026-11-10"`, "終了日は開始日以降にしてください"},
		{"points", `"-1"`, "ポイントは 0 以上の数で入力してください"},
		{"estimate", `"0.123"`, "見積もり時間は小数第 2 位までで入力してください"},
	}
	for _, c := range cases {
		_, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, change.ID(c.prop), raw(c.in))
		var r *refusal.Refusal
		if !errors.As(err, &r) || r.Message != c.want {
			t.Errorf("%s: err = %v", c.prop, err)
		}
	}
	if logLen(t, rec, apptest.SampleTask) != n {
		t.Error("refused changes must not be saved")
	}
}

func TestSetPropertyUnknownIsNotFound(t *testing.T) {
	svc, _ := setupRec(t)
	_, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"01NOPE00000000000000000000"`))
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
	_, err = svc.SetProperty(t.Context(), store, apptest.SampleTask, "01NOPROP000000000000000000", raw(`null`))
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestToggleSystems(t *testing.T) {
	svc, rec := setupRec(t)
	web := apptest.SampleSystemWeb
	it, err := svc.AddValue(t.Context(), store, apptest.SampleTask, "systems", web)
	if err != nil || len(it.Systems) != 1 || last(rec) != "HH-2 の関連システムを Web に変更" {
		t.Fatalf("AddValue = %+v, %v, %q", it, err, last(rec))
	}
	n := logLen(t, rec, apptest.SampleTask)
	_, _ = svc.AddValue(t.Context(), store, apptest.SampleTask, "systems", web)
	if logLen(t, rec, apptest.SampleTask) != n {
		t.Error("adding a present value must not be saved")
	}
	it, err = svc.RemoveValue(t.Context(), store, apptest.SampleTask, "systems", web)
	if err != nil || len(it.Systems) != 0 || last(rec) != "HH-2 の関連システムを なし に変更" {
		t.Errorf("RemoveValue = %+v, %v, %q", it, err, last(rec))
	}
}
```

import には `errors` も入れる。担当者の表示名のテストは、Sample のメンバーの表示名がログイン名と同じなので、`taro` になる(決定 15)。

- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/items/...` / Expected: FAIL

- [ ] **Step 3: 実装する**

`internal/app/items/view.go`:

```go
package items

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// editContext は、値の操作が保存の前に読むもの(アクティビティの 1 行を保存の前に決めるため。withID と同じ)。
type editContext struct {
	item     item.Item
	display  string
	settings workspace.Settings
	names    item.Names
}

func (s Service) editContext(ctx context.Context, store port.StoreID, id item.ID) (editContext, error) {
	var ec editContext
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := r.Items().Get(id)
		if err != nil {
			return err
		}
		w, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return err
		}
		st, err := r.Workspaces().Settings(it.Place.Workspace)
		if err != nil {
			return err
		}
		ms, err := r.Members().List()
		if err != nil {
			return err
		}
		names := map[string]string{}
		for _, m := range ms {
			names[strings.ToLower(m.Login)] = m.Name
		}
		ec = editContext{item: it, display: it.DisplayID(w.Prefix), settings: st,
			names: func(login string) string {
				if n, ok := names[strings.ToLower(login)]; ok {
					return n
				}
				return login
			}}
		return nil
	})
	return ec, err
}

// property は、id のプロパティ。なければ port.ErrNotFound。
func (ec editContext) property(id change.ID) (workspace.Property, error) {
	p, ok := ec.settings.Property(id)
	if !ok {
		return workspace.Property{}, port.ErrNotFound
	}
	return p, nil
}
```

(`change` の import を足す。)

`internal/app/items/setproperty.go`:

```go
package items

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// SetProperty は、1 つのプロパティの値を変える(02「入力の方法と保存」)。前と同じなら何も保存しない。
// アクティビティは「{ID} の{プロパティ名}を {値} に変更」。遷移の制約は後半(A3b)でここに足す(決定 2)。
func (s Service) SetProperty(ctx context.Context, store port.StoreID, id item.ID, prop change.ID, value json.RawMessage) (item.Item, error) {
	ec, err := s.editContext(ctx, store, id)
	if err != nil {
		return item.Item{}, err
	}
	p, err := ec.property(prop)
	if err != nil {
		return item.Item{}, err
	}
	if p.Type == workspace.TypeSystems || p.Type == workspace.TypeMulti {
		return item.Item{}, port.ErrNotFound // 入り切りは AddValue・RemoveValue
	}
	to, err := item.ParseInput(p, value, ec.settings)
	if errors.Is(err, item.ErrNoSuchValue) {
		return item.Item{}, port.ErrNotFound
	}
	if err != nil {
		return item.Item{}, err
	}
	if err := checkDatePair(ec.item, p, to); err != nil {
		return item.Item{}, err
	}
	summary := fmt.Sprintf("%s の%sを %s に変更", ec.display, p.Name, item.Display(p, to, ec.settings, ec.names, s.Clock.Now()))
	return s.writeValue(ctx, store, id, summary, func(it item.Item) []change.Draft {
		from := item.Current(it, p)
		if bytes.Equal(from, to) {
			return nil
		}
		return []change.Draft{change.NewSet(item.Field(p), from, to)}
	})
}

// checkDatePair は、開始日か終了日を変えるとき、保存済みのもう片方と比べる(決定 16)。
func checkDatePair(it item.Item, p workspace.Property, to json.RawMessage) error {
	if !p.Builtin || (p.Key != "start" && p.Key != "end") {
		return nil
	}
	var v string
	_ = json.Unmarshal(to, &v)
	if p.Key == "start" {
		return item.CheckDates(v, it.End)
	}
	return item.CheckDates(it.Start, v)
}

// writeValue は、保存の中でいまのアイテムを読み直し、drafts が返す行を 1 回で保存する。行がなければ何も残さない。
func (s Service) writeValue(ctx context.Context, store port.StoreID, id item.ID, summary string, drafts func(item.Item) []change.Draft) (item.Item, error) {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return item.Item{}, err
	}
	op := port.Operation{Actor: change.User(me), Summary: summary}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		it, err := tx.Items().Get(id)
		if err != nil {
			return err
		}
		return tx.WriteItems().Append(id, drafts(it)...)
	})
	if err != nil {
		return item.Item{}, err
	}
	return s.Get(ctx, store, id)
}
```

`internal/app/items/togglevalue.go`:

```go
package items

import (
	"context"
	"fmt"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// AddValue は、関連システムか複数選択に値を 1 つ足す(02「入力の方法と保存」)。すでにあれば何も保存しない。
func (s Service) AddValue(ctx context.Context, store port.StoreID, id item.ID, prop, value change.ID) (item.Item, error) {
	return s.toggle(ctx, store, id, prop, value, true)
}

// RemoveValue は、値を 1 つ取り除く。なければ何も保存しない。
func (s Service) RemoveValue(ctx context.Context, store port.StoreID, id item.ID, prop, value change.ID) (item.Item, error) {
	return s.toggle(ctx, store, id, prop, value, false)
}

func (s Service) toggle(ctx context.Context, store port.StoreID, id item.ID, prop, value change.ID, on bool) (item.Item, error) {
	ec, err := s.editContext(ctx, store, id)
	if err != nil {
		return item.Item{}, err
	}
	p, err := ec.property(prop)
	if err != nil {
		return item.Item{}, err
	}
	list, ok := listOf(ec.settings, p)
	if !ok || (on && !slices.ContainsFunc(list, func(v workspace.Value) bool { return v.ID == value })) {
		return item.Item{}, port.ErrNotFound
	}
	after := toggled(currentIDs(ec.item, p), value, on)
	summary := fmt.Sprintf("%s の%sを %s に変更", ec.display, p.Name, item.Display(p, change.Value(after), ec.settings, ec.names, s.Clock.Now()))
	return s.writeValue(ctx, store, id, summary, func(it item.Item) []change.Draft {
		has := slices.Contains(currentIDs(it, p), value)
		switch {
		case on && !has:
			return []change.Draft{change.NewAdd(item.Field(p), value)}
		case !on && has:
			return []change.Draft{change.NewRemove(item.Field(p), value)}
		}
		return nil
	})
}

func listOf(st workspace.Settings, p workspace.Property) ([]workspace.Value, bool) {
	switch p.Type {
	case workspace.TypeSystems:
		return st.Systems, true
	case workspace.TypeMulti:
		return p.Choices, true
	}
	return nil, false
}

func currentIDs(it item.Item, p workspace.Property) []change.ID {
	if p.Type == workspace.TypeSystems {
		return it.Systems
	}
	return it.Choices[p.ID]
}

func toggled(ids []change.ID, v change.ID, on bool) []change.ID {
	out := slices.DeleteFunc(slices.Clone(ids), func(x change.ID) bool { return x == v })
	if on {
		out = append(out, v)
	}
	return out
}
```

`service.go` の `Service` に `Clock port.Clock` を足し、`routes.go` の 70 行目 `appitems.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity}` に `Clock: deps.Clock` を足す。既存のテストで `items.Service` を組み立てている箇所(`edit_test.go` の `setupRec`、`items_test.go`、httpapi の `newAPI`)にも `Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))` を足す。

- [ ] **Step 4: 通るのを確かめる** — Run: `go test ./internal/app/items/... ./internal/adapter/httpapi/...` / Expected: PASS
- [ ] **Step 5: `make check`、コミット** — `git commit -m "feat(app): プロパティの変更と、関連システム・複数選択の入り切りを足す(A3 前半 Task 4)"`

---

### Task 5: 作成の値と初期値、Step の追加の値、Step のチェック

**Files:**
- Modify: `internal/app/items/create.go`、`internal/app/items/addstep.go`、`internal/domain/item/check.go`
- Create: `internal/app/items/checkstep.go`
- Test: `internal/app/items/items_test.go`(追記)、`internal/app/items/values_test.go`(追記)、`internal/domain/item/check_test.go`(追記)

**Interfaces:**
- Consumes: Task 3・4
- Produces: `CreateInput.Values`、`item.CheckInputs`、`Service.CheckStep`

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/item/check_test.go` に:

```go
// 検査は 02「入力の検査」の表の順。日付 → ポイント → 見積もり時間。
func TestCheckInputsOrder(t *testing.T) {
	st := workspace.DefaultSettings(seqIDs())
	in := map[change.ID]json.RawMessage{
		"start": raw(`"2026-11-11"`), "end": raw(`"2026-11-10"`),
		"points": raw(`"-1"`), "estimate": raw(`"0.123"`),
	}
	_, err := item.CheckInputs(st, in)
	refusedWith(t, err, "終了日は開始日以降にしてください")
	in["end"] = raw(`"2026-11-12"`)
	_, err = item.CheckInputs(st, in)
	refusedWith(t, err, "ポイントは 0 以上の数で入力してください")
	in["points"] = raw(`"1"`)
	_, err = item.CheckInputs(st, in)
	refusedWith(t, err, "見積もり時間は小数第 2 位までで入力してください")
	in["estimate"] = raw(`"0.5"`)
	got, err := item.CheckInputs(st, in)
	if err != nil || string(got["estimate"]) != "0.5" || string(got["points"]) != "1" {
		t.Errorf("CheckInputs = %v, %v", got, err)
	}
}
```

(`raw` は `input_test.go` に置く。)

`internal/app/items/items_test.go` に:

```go
func TestCreateUsesInitialStatusAndPriority(t *testing.T) {
	svc, _ := setupRec(t)
	it, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: apptest.SampleProject, Kind: item.Epic, Title: "新しい"})
	if err != nil || it.Status != apptest.SampleStatusTodo || it.Priority != apptest.SamplePriorityMedium {
		t.Errorf("Create = %+v, %v", it, err)
	}
}

func TestCreateWritesGivenValues(t *testing.T) {
	svc, _ := setupRec(t)
	epic := apptest.SampleEpic
	it, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Parent: &epic, Kind: item.Task, Title: "x",
		Values: map[change.ID]json.RawMessage{
			"status":   raw(`"` + string(apptest.SampleStatusInProgress) + `"`),
			"assignee": raw(`"taro"`), "end": raw(`"2026-11-20"`), "estimate": raw(`"2"`),
			"systems":  raw(`["` + string(apptest.SampleSystemWeb) + `"]`),
		},
	})
	if err != nil || it.Status != apptest.SampleStatusInProgress || it.Assignee != "taro" ||
		it.End != "2026-11-20" || *it.Estimate != 2 || len(it.Systems) != 1 {
		t.Errorf("Create = %+v, %v", it, err)
	}
}

func TestCreateChecksValuesAfterTheParent(t *testing.T) {
	svc, _ := setupRec(t)
	_, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: apptest.SampleProject, Kind: item.Task, Title: "x",
		Values: map[change.ID]json.RawMessage{"points": raw(`"-1"`)}})
	var r *refusal.Refusal
	if !errors.As(err, &r) || r.Message != "親になる Epic を選んでください" {
		t.Errorf("err = %v", err)
	}
}

func TestAddStepTakesTheParentsAssignee(t *testing.T) {
	svc, _ := setupRec(t)
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "assignee", raw(`"taro"`)); err != nil {
		t.Fatal(err)
	}
	st, err := svc.AddStep(t.Context(), store, apptest.SampleTask, "確認")
	if err != nil || st.Assignee != "taro" || st.Status != apptest.SampleStatusTodo || st.Priority != apptest.SamplePriorityMedium {
		t.Errorf("AddStep = %+v, %v", st, err)
	}
}
```

`values_test.go` に:

```go
func TestCheckStep(t *testing.T) {
	svc, rec := setupRec(t)
	step, err := svc.AddStep(t.Context(), store, apptest.SampleTask, "確認")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetProperty(t.Context(), store, step.ID, "status", raw(`"`+string(apptest.SampleStatusInProgress)+`"`)); err != nil {
		t.Fatal(err)
	}
	it, err := svc.CheckStep(t.Context(), store, step.ID, true)
	if err != nil || it.Status != apptest.SampleStatusDone || last(rec) != "HH-3 のステータスを Done に変更" {
		t.Fatalf("check = %+v, %v, %q", it, err, last(rec))
	}
	// 進行中から完了にして外すと、未着手の既定に戻る(02「Step のチェック」)。
	it, err = svc.CheckStep(t.Context(), store, step.ID, false)
	if err != nil || it.Status != apptest.SampleStatusTodo {
		t.Errorf("uncheck = %+v, %v", it, err)
	}
	if _, err := svc.CheckStep(t.Context(), store, apptest.SampleTask, true); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("a Task is not a Step: %v", err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/domain/item/... ./internal/app/items/...` / Expected: FAIL

- [ ] **Step 3: 実装する**

`internal/domain/item/check.go` に:

```go
// CheckInputs は、作成の入力 in を 02「入力の検査」の表の順(日付・ポイント・見積もり時間)に確かめ、
// ほかの値も ParseInput で読んで、ログに書く値にして返す。関連システムと複数選択は ID の配列のまま返す。
// タイトル・プロジェクト・親の検査(CheckCreate)は、呼ぶ側が先に行う。
func CheckInputs(st workspace.Settings, in map[change.ID]json.RawMessage) (map[change.ID]json.RawMessage, error) {
	if err := CheckDates(text(in["start"]), text(in["end"])); err != nil {
		return nil, err
	}
	out := map[change.ID]json.RawMessage{}
	ordered := append([]change.ID{"points", "estimate"}, keysExcept(in, "points", "estimate")...)
	for _, id := range ordered {
		v, ok := in[id]
		if !ok {
			continue
		}
		p, ok := st.Property(id)
		if !ok {
			return nil, ErrNoSuchValue
		}
		parsed, err := parseCreateValue(p, v, st)
		if err != nil {
			return nil, err
		}
		out[id] = parsed
	}
	return out, nil
}

// parseCreateValue は、複数の値(関連システム・複数選択)を ID の配列で、ほかは ParseInput で読む。
func parseCreateValue(p workspace.Property, v json.RawMessage, st workspace.Settings) (json.RawMessage, error) {
	if p.Type != workspace.TypeSystems && p.Type != workspace.TypeMulti {
		return ParseInput(p, v, st)
	}
	var ids []change.ID
	if json.Unmarshal(v, &ids) != nil {
		return nil, ErrBadInput
	}
	list := st.Systems
	if p.Type == workspace.TypeMulti {
		list = p.Choices
	}
	for _, id := range ids {
		if _, err := pickValue(change.Value(id), list, false); err != nil {
			return nil, err
		}
	}
	return change.Value(ids), nil
}

// keysExcept は、in のキーのうち skip にないものを辞書順で返す(結果を決まった順にする)。
func keysExcept(in map[change.ID]json.RawMessage, skip ...change.ID) []change.ID {
	var out []change.ID
	for k := range in {
		if !slices.Contains(skip, k) {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}
```

`internal/app/items/create.go`:

- `CreateInput` に `Values map[change.ID]json.RawMessage` を足す。
- `build` の `CheckCreate` の後で、プロジェクトのワークスペースの設定を読み、`vals, err := item.CheckInputs(st, in.Values)` を呼ぶ(`ErrNoSuchValue` は `port.ErrNotFound` にする)。
- 作るアイテムに `applyValues(&created, vals, st)` を当てる:

```go
// applyValues は、読んだ値をアイテムに入れ、ステータスと優先度がなければ初期値にする(決定 10)。
func applyValues(it *item.Item, vals map[change.ID]json.RawMessage, st workspace.Settings) {
	var s string
	read := func(k string) string { s = ""; _ = json.Unmarshal(vals[change.ID(k)], &s); return s }
	it.Status = change.ID(read("status"))
	it.Priority = change.ID(read("priority"))
	it.Assignee, it.Start, it.End = read("assignee"), read("start"), read("end")
	it.Points, it.Estimate = number(vals["points"]), number(vals["estimate"])
	_ = json.Unmarshal(vals["systems"], &it.Systems)
	if it.Status == "" {
		it.Status = st.InitialStatus
	}
	if it.Priority == "" {
		it.Priority = st.InitialPriority
	}
	for id, v := range vals {
		if p, ok := st.Property(id); ok && !p.Builtin && string(v) != "null" {
			if it.Fields == nil {
				it.Fields = map[change.ID]json.RawMessage{}
			}
			it.Fields[id] = v
		}
	}
}

func number(v json.RawMessage) *float64 {
	var f float64
	if len(v) == 0 || string(v) == "null" || json.Unmarshal(v, &f) != nil {
		return nil
	}
	return &f
}
```

(`build` が 60 行を超えるときは、設定を読んで値を当てる部分を `withValues(tx, ws, in, &created)` に分ける。)

`internal/app/items/addstep.go`: 親の担当者を `Values: map[change.ID]json.RawMessage{"assignee": change.Value(p.Assignee)}` で渡す(空なら渡さない)。ステータスと優先度は `applyValues` の初期値で入る。

`internal/app/items/checkstep.go`:

```go
package items

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// CheckStep は、Step のチェックを入り切りする(02「Step のチェック」)。done なら完了の既定、そうでなければ
// 未着手の既定のステータスにする(03「分類ごとの既定のステータス」)。保存とアクティビティは SetProperty と同じ。
// 遷移の制約は後半(A3b)で SetProperty の中に足す(決定 2)。
func (s Service) CheckStep(ctx context.Context, store port.StoreID, id item.ID, done bool) (item.Item, error) {
	ec, err := s.editContext(ctx, store, id)
	if err != nil {
		return item.Item{}, err
	}
	if ec.item.Kind != item.Step {
		return item.Item{}, port.ErrNotFound
	}
	cat := workspace.CategoryTodo
	if done {
		cat = workspace.CategoryDone
	}
	return s.SetProperty(ctx, store, id, "status", change.Value(ec.settings.CategoryDefaults[cat]))
}
```

- [ ] **Step 4: 通るのを確かめる** — Run: `go test ./internal/domain/item/... ./internal/app/items/...` / Expected: PASS
- [ ] **Step 5: `make check`、コミット** — `git commit -m "feat(app): 作成の値と初期値、Step の追加の値、Step のチェックを足す(A3 前半 Task 5)"`

---

### Task 6: API

**Files:**
- Modify: `internal/adapter/httpapi/items/items.go`(`Item` の値、`CreateItem.Values`、`Register`)
- Create: `internal/adapter/httpapi/items/values.go`
- Test: `internal/adapter/httpapi/items/values_test.go`
- Regenerate: `api/openapi.yaml`、`web/packages/api/src/generated.ts`

**Interfaces:**
- Consumes: Task 4・5 の `Service` の関数
- Produces: 上の API の表

- [ ] **Step 1: 失敗するテストを書く**

`items_test.go` の `newAPI`(Store「S1」に Sample)を使う。(`newAPI` の `Clock` は Task 4 で足してある。)`values_test.go`:

```go
package items_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
)

const taskPath = "/api/stores/S1/items/" + string(apptest.SampleTask)

func TestSetItemPropertyAPI(t *testing.T) {
	api := newAPI(t)
	res := api.Put(taskPath+"/properties/points", map[string]any{"value": "3"})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"points":3`) {
		t.Fatalf("PUT = %d %s", res.Code, res.Body)
	}
	res = api.Put(taskPath+"/properties/points", map[string]any{"value": "-1"})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "ポイントは 0 以上の数で入力してください") {
		t.Errorf("PUT -1 = %d %s", res.Code, res.Body)
	}
}

// value は any。真偽・null・配列も届く(決定 12)。
func TestSetItemPropertyValueShapes(t *testing.T) {
	api := newAPI(t)
	if res := api.Put(taskPath+"/properties/assignee", map[string]any{"value": "taro"}); !strings.Contains(res.Body.String(), `"assignee":"taro"`) {
		t.Errorf("string: %d %s", res.Code, res.Body)
	}
	if res := api.Put(taskPath+"/properties/assignee", map[string]any{"value": nil}); !strings.Contains(res.Body.String(), `"assignee":""`) {
		t.Errorf("null: %d %s", res.Code, res.Body)
	}
	if res := api.Put(taskPath+"/properties/start", map[string]any{"value": true}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("bool for a date: %d %s", res.Code, res.Body)
	}
	if res := api.Put(taskPath+"/properties/systems", map[string]any{"value": []string{string(apptest.SampleSystemWeb)}}); res.Code != http.StatusNotFound {
		t.Errorf("array for systems must use the toggle API: %d %s", res.Code, res.Body)
	}
}

func TestToggleAndCheckAPI(t *testing.T) {
	api := newAPI(t)
	path := taskPath + "/properties/systems/values/" + string(apptest.SampleSystemWeb)
	if res := api.Post(path); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), string(apptest.SampleSystemWeb)) {
		t.Errorf("POST = %d %s", res.Code, res.Body)
	}
	if res := api.Delete(path); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"systems":[]`) {
		t.Errorf("DELETE = %d %s", res.Code, res.Body)
	}
	if res := api.Post(taskPath+"/check", map[string]any{"done": true}); res.Code != http.StatusNotFound {
		t.Errorf("check a Task = %d", res.Code)
	}
}

func TestItemHasValues(t *testing.T) {
	api := newAPI(t)
	res := api.Get(taskPath)
	for _, s := range []string{`"status":"` + string(apptest.SampleStatusTodo) + `"`, `"assignee":""`, `"points":null`, `"systems":[]`, `"choices":{}`, `"fields":{}`} {
		if !strings.Contains(res.Body.String(), s) {
			t.Errorf("GET has no %s: %s", s, res.Body)
		}
	}
}
```

- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/adapter/httpapi/items/...` / Expected: FAIL

- [ ] **Step 3: 実装する**

`items.go` の `Item` に:

```go
	Status   string                     `json:"status" doc:"ステータスの内部 ID。空は値なし"`
	Priority string                     `json:"priority"`
	Assignee string                     `json:"assignee" doc:"ログイン名。空は未割り当て"`
	Start    string                     `json:"start" doc:"YYYY-MM-DD。空は値なし"`
	End      string                     `json:"end"`
	Points   *float64                   `json:"points"`
	Estimate *float64                   `json:"estimate" doc:"時間"`
	Systems  []string                   `json:"systems" nullable:"false"`
	Choices  map[string][]string        `json:"choices" nullable:"false" doc:"選択式の追加プロパティの値(プロパティの内部 ID → 値の内部 ID)"`
	Fields   map[string]json.RawMessage `json:"fields" nullable:"false" doc:"追加プロパティの値(プロパティの内部 ID → 値)。型は設定で読む"`
```

`toAPI` で、`nil` は空の配列・空の辞書にする。`CreateItem` に `Values map[string]json.RawMessage `json:"values,omitempty"`` を足し、`create-item` の処理で `appitems.CreateInput.Values` に移す。

`values.go`:

```go
package items

// SetItemProperty は、プロパティの変更の本文。value は値(数は文字列のまま。決定 12)。
type SetItemProperty struct {
	Value json.RawMessage `json:"value" doc:"新しい値。null と空の文字列は値を空にする"`
}

// CheckStep は、Step のチェックの本文。
type CheckStep struct {
	Done bool `json:"done"`
}

type propertyInput struct {
	Store    string `path:"store"`
	Item     string `path:"item"`
	Property string `path:"property"`
	Body     SetItemProperty
}

type valueInput struct {
	Store    string `path:"store"`
	Item     string `path:"item"`
	Property string `path:"property"`
	Value    string `path:"value"`
}

type checkInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  CheckStep
}

func registerValues(api huma.API, svc appitems.Service) {
	huma.Register(api, huma.Operation{OperationID: "set-item-property", Method: http.MethodPut, Path: "/api/stores/{store}/items/{item}/properties/{property}", Summary: "アイテムのプロパティの値を変える"},
		func(ctx context.Context, in *propertyInput) (*oneOutput, error) {
			return one(svc.SetProperty(ctx, port.StoreID(in.Store), item.ID(in.Item), change.ID(in.Property), in.Body.Value))
		})
	huma.Register(api, huma.Operation{OperationID: "add-item-value", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/properties/{property}/values/{value}", Summary: "関連システム・複数選択に値を足す"},
		func(ctx context.Context, in *valueInput) (*oneOutput, error) {
			return one(svc.AddValue(ctx, port.StoreID(in.Store), item.ID(in.Item), change.ID(in.Property), change.ID(in.Value)))
		})
	huma.Register(api, huma.Operation{OperationID: "remove-item-value", Method: http.MethodDelete, Path: "/api/stores/{store}/items/{item}/properties/{property}/values/{value}", Summary: "関連システム・複数選択から値を取り除く"},
		func(ctx context.Context, in *valueInput) (*oneOutput, error) {
			return one(svc.RemoveValue(ctx, port.StoreID(in.Store), item.ID(in.Item), change.ID(in.Property), change.ID(in.Value)))
		})
	huma.Register(api, huma.Operation{OperationID: "check-step", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/check", Summary: "Step のチェックを入り切りする"},
		func(ctx context.Context, in *checkInput) (*oneOutput, error) {
			return one(svc.CheckStep(ctx, port.StoreID(in.Store), item.ID(in.Item), in.Body.Done))
		})
}

func one(it item.Item, err error) (*oneOutput, error) {
	if errors.Is(err, item.ErrBadInput) {
		return nil, huma.Error422UnprocessableEntity(err.Error()) // 画面からは送らない(決定 16)
	}
	if err != nil {
		return nil, apierr.From(err)
	}
	return &oneOutput{Body: toAPI(it)}, nil
}
```

`Register` に `registerValues(api, svc)` を足す。

- [ ] **Step 4: 生成物を作り直し、通るのを確かめる**

```bash
make generate
cd web && pnpm gen && cd ..
go test ./internal/adapter/httpapi/...
```
Expected: PASS。`generated.ts` に `setItemProperty`・`addItemValue`・`removeItemValue`・`checkStep` ができ、`SetItemProperty` の `value` が `unknown` になっていることを目で確かめる(`json.RawMessage` は huma で型のないスキーマになる。違う型になったら、`Value any` にして `json.Marshal` で `json.RawMessage` に戻す)。

- [ ] **Step 5: `make check`(web の型検査を含む)、コミット** — `git commit -m "feat(adapter): アイテムの値・プロパティの変更・入り切り・Step のチェックの API を足す(A3 前半 Task 6)"`

---

### Task 7: 共有: ui の `ValueMenu` と `Avatar`

**ブランチ:** B2 本体が develop に入ってから、develop から `feat/track-a3a-ui`。この PR だけで develop へ入れる。

B2 は、共有の `Menu` を変えずに、画面の中の部品で `ColorMenu`(1 つを選ぶ)と `KindsMenu`(開いたまま入り切りする、`workspace-settings` の中)を作った。値を選ぶメニューは、詳細パネル・作成ダイアログ・ツリー・のちのカンバンとテーブル(トラック E)が使うので、画面の中ではなく ui の `patterns` に 1 つ足す。`Menu` は変えない。B2 の `KindsMenu` を `ValueMenu` に置き換えるかは、B2 の持ち主に任せる(この計画では触らない)。

**Files:**
- Create: `web/packages/ui/src/patterns/ValueMenu.svelte`、`web/packages/ui/src/patterns/Avatar.svelte`
- Modify: `web/packages/ui/src/index.ts`、`web/packages/ui/src/types.ts`(`ValueOption`)、`web/packages/ui/src/primitives/Select.svelte`(`onchange?: () => void` を足し、`<select onchange={onchange}>` に渡す。選んだときに保存するため)
- Test: `web/packages/ui/src/patterns/ValueMenu.test.ts`、`web/packages/ui/src/patterns/Avatar.test.ts`

**Interfaces:**
- Produces: `ValueMenu`(props: `open`・`x`・`y`・`label`・`options: ValueOption[]`・`multi?: boolean`・`onselect(id)`・`onclose()`)、`Avatar`(props: `name?: string`・`title: string`)、型 `ValueOption { id: string; label: string; hint?: string; checked: boolean }`

- [ ] **Step 1: 失敗するテストを書く**

まず `web/packages/ui/src/primitives/Menu.test.ts` があれば、その書き方(`@testing-library/svelte` の `render`・`fireEvent`)に合わせる。`ValueMenu.test.ts`:

```ts
import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import ValueMenu from "./ValueMenu.svelte";

const opts = (n: number) =>
  Array.from({ length: n }, (_, i) => ({ id: `v${i}`, label: `値${i}`, checked: i === 1 }));
const base = { open: true, x: 0, y: 0, label: "優先度" };

describe("ValueMenu", () => {
  it("marks the current value and reports the choice", async () => {
    const onselect = vi.fn();
    render(ValueMenu, { ...base, options: opts(3), onselect, onclose: vi.fn() });
    expect(screen.getByRole("menuitemcheckbox", { name: /値1/ })).toHaveAttribute("aria-checked", "true");
    await fireEvent.click(screen.getByRole("menuitemcheckbox", { name: /値2/ }));
    expect(onselect).toHaveBeenCalledWith("v2");
  });

  it("has no search field below 7 options", () => {
    render(ValueMenu, { ...base, options: opts(6), onselect: vi.fn(), onclose: vi.fn() });
    expect(screen.queryByRole("searchbox")).toBeNull();
  });

  it("filters by label from 7 options", async () => {
    render(ValueMenu, { ...base, options: opts(7), onselect: vi.fn(), onclose: vi.fn() });
    await fireEvent.input(screen.getByRole("searchbox"), { target: { value: "値6" } });
    expect(screen.getAllByRole("menuitemcheckbox")).toHaveLength(1);
  });

  it("closes after a choice", async () => {
    const onclose = vi.fn();
    render(ValueMenu, { ...base, options: opts(2), onselect: vi.fn(), onclose });
    await fireEvent.click(screen.getByRole("menuitemcheckbox", { name: /値0/ }));
    expect(onclose).toHaveBeenCalledTimes(1);
  });

  it("stays open for multi", async () => {
    const onclose = vi.fn();
    render(ValueMenu, { ...base, multi: true, options: opts(2), onselect: vi.fn(), onclose });
    await fireEvent.click(screen.getByRole("menuitemcheckbox", { name: /値0/ }));
    expect(onclose).not.toHaveBeenCalled();
  });
});
```

`Avatar.test.ts`:

```ts
import { render, screen } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import Avatar from "./Avatar.svelte";

describe("Avatar", () => {
  it("shows the first letter and the name as the title", () => {
    render(Avatar, { name: "太郎", title: "太郎" });
    const a = screen.getByTitle("太郎");
    expect(a).toHaveTextContent("太");
  });
  it("is empty for an unknown user", () => {
    render(Avatar, { title: "不明なユーザー" });
    expect(screen.getByTitle("不明なユーザー")).toHaveTextContent("");
  });
});
```

- [ ] **Step 2: 失敗を確かめる** — Run: `cd web && pnpm --filter @hedgehug/ui test` / Expected: FAIL(部品がない)

- [ ] **Step 3: 実装する**

`ValueMenu.svelte`:

```svelte
<script lang="ts">
  import type { ValueOption } from "../types";

  // 値を選ぶメニュー(02「入力の方法と保存」)。いまの値にチェックを付け、7 件以上で検索欄を出す。
  // multi なら選んでも閉じない(関連システム・複数選択)。外を押すか Esc で閉じる(Menu と同じ)。
  interface Props {
    open: boolean;
    x: number;
    y: number;
    label: string;
    options: ValueOption[];
    multi?: boolean;
    onselect: (id: string) => void;
    onclose: () => void;
  }

  let { open, x, y, label, options, multi = false, onselect, onclose }: Props = $props();
  let element: HTMLDivElement | undefined = $state();
  let query = $state("");
  const shown = $derived(
    options.filter((o) => o.label.toLowerCase().includes(query.trim().toLowerCase())),
  );

  $effect(() => {
    if (!open) query = "";
  });

  function onkeydown(event: KeyboardEvent) {
    if (open && event.key === "Escape" && !event.isComposing) onclose();
  }
  function onpointerdown(event: PointerEvent) {
    if (open && element && !element.contains(event.target as Node)) onclose();
  }
  function choose(id: string) {
    onselect(id);
    if (!multi) onclose();
  }
</script>

<svelte:window {onkeydown} {onpointerdown} />

{#if open}
  <div
    class="menu-pop"
    role="menu"
    aria-label={label}
    style:left="{x}px"
    style:top="{y}px"
    bind:this={element}
  >
    {#if options.length >= 7}
      <!-- 「検索」は読み上げの名前で、画面に出る文言ではない -->
      <input class="menu-search" type="search" aria-label="検索" bind:value={query} />
    {/if}
    {#each shown as o (o.id)}
      <button
        type="button"
        role="menuitemcheckbox"
        aria-checked={o.checked}
        class="menu-item"
        onclick={() => choose(o.id)}
      >
        <span class="check" aria-hidden="true">{o.checked ? "✓" : ""}</span>
        <span class="menu-label">{o.label}</span>
        {#if o.hint}<span class="menu-hint">{o.hint}</span>{/if}
      </button>
    {/each}
  </div>
{/if}
```

スタイルは `Menu.svelte` の `.menu-pop`・`.menu-item`・`.menu-label`・`.menu-hint` を写す。足すのは `.check { width: 14px; display: inline-block; }` と `.menu-search { width: 100%; margin-bottom: 4px; }` だけ。

`Avatar.svelte`:

```svelte
<script lang="ts">
  // アバター(決定 24)。name の頭の 1 文字を丸に出す。name がなければ中身のない丸(01「不明なユーザー」)。
  interface Props {
    name?: string;
    title: string;
  }

  let { name = "", title }: Props = $props();
</script>

<span class="avatar" class:empty={name === ""} {title} aria-label={title}>{name.slice(0, 1)}</span>

<style>
  .avatar {
    display: inline-grid;
    place-items: center;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: var(--c-slate-soft);
    font-size: 10.5px;
  }
  .empty {
    background: none;
    border: 1px dashed var(--muted);
  }
</style>
```

`types.ts` に `ValueOption` を足し、`index.ts` に `ValueMenu`・`Avatar`・`ValueOption` を export する。

- [ ] **Step 4: 通るのを確かめる** — Run: `cd web && pnpm --filter @hedgehug/ui test && pnpm check` / Expected: PASS
- [ ] **Step 5: コミットと PR** — `git commit -m "feat(ui): 値を選ぶメニューとアバターの部品を足す(A3 前半 Task 7)"`。PR のタイトル「ui: ValueMenu・Avatar(トラック A3 前半の共有部分)」。マージ後、`feat/track-a3a` に develop を取り込む。

---

### Task 8: core の型・口・プロパティの行

**Files:**
- Modify: `web/packages/core/src/model/item.ts`、`web/packages/core/src/port/items.ts`、`web/packages/api/src/items.ts`、`web/packages/core/src/index.ts`
- Modify: `web/packages/core/src/features/items/fixtures.test-util.ts`(`item()` の値、偽の口の 4 つ)
- Create: `web/packages/core/src/features/items/settings.test-util.ts`(テスト用の設定と、ワークスペースの偽の口)
- Create: `web/packages/core/src/features/items/properties.ts`
- Test: `web/packages/core/src/features/items/properties.test.ts`

**Interfaces:**
- Consumes: B2 の `WorkspaceSettings`・`WorkspaceProperty`・`WorkspaceValue`・`Category`(`model/workspace.ts`)・`WorkspacesPort`・`fakeWorkspacesPort`(`features/workspaces/fixtures.test-util.ts`)、Task 6 の生成物
- Produces: 「型と口」の `PropertyInput`・`Item` の値・`ItemsPort` の 4 つ・`properties.ts`。テスト用に `settings`・`settings2`・`fakeWorkspaces`、`FakeItems.refuseNext`・`holdCreate`・`createdValues`

- [ ] **Step 1: テストの道具を足す**

`settings.test-util.ts`:

```ts
import type { WorkspaceProperty, WorkspaceSettings, WorkspaceValue } from "../../model/workspace";
import type { WorkspacesPort } from "../../port/workspaces";
import { fakeWorkspacesPort } from "../workspaces/fixtures.test-util";

const all = ["Epic", "Story", "Task", "Bug", "Step"];
const v = (id: string, name: string, category?: WorkspaceValue["category"]): WorkspaceValue =>
  category ? { id, name, color: "gray", category } : { id, name, color: "gray" };
const prop = (id: string, name: string, type: WorkspaceProperty["type"], extra: Partial<WorkspaceProperty> = {}): WorkspaceProperty => ({
  id, key: id, name, type, builtin: true, kinds: all, hidden: false, choices: [], ...extra,
});

// テスト用の設定。終了日は非表示、ポイントは Story だけ、F1 は単一選択の追加プロパティ、F2 はテキストの追加プロパティ。
export const settings: WorkspaceSettings = {
  properties: [
    prop("status", "ステータス", "status"),
    prop("assignee", "担当者", "user"),
    prop("priority", "優先度", "priority"),
    prop("start", "開始日", "date"),
    prop("end", "終了日", "date", { hidden: true }),
    prop("points", "ポイント", "number", { kinds: ["Story"] }),
    prop("estimate", "見積もり時間", "duration"),
    prop("systems", "関連システム", "systems"),
    prop("F1", "環境", "single", { builtin: false, choices: [v("C1", "本番"), v("C2", "検証")] }),
    prop("F2", "メモ", "text", { builtin: false }),
  ],
  statuses: [v("st1", "Todo", "todo"), v("st2", "InProgress", "doing"), v("st3", "Done", "done"), v("st4", "Cancel", "canceled")],
  priorities: [v("pr1", "High"), v("pr2", "Medium")],
  systems: [v("sy1", "Web"), v("sy2", "API")],
  initialStatus: "st1",
  initialPriority: "pr2",
  categoryDefaults: { todo: "st1", doing: "st2", done: "st3", canceled: "st4" },
};

// settings2 は、別のワークスペースの設定(初期値が違う)。
export const settings2: WorkspaceSettings = {
  ...settings,
  statuses: [v("ns1", "新規", "todo"), v("ns2", "完了", "done")],
  priorities: [v("np1", "普通")],
  initialStatus: "ns1",
  initialPriority: "np1",
  categoryDefaults: { todo: "ns1", done: "ns2" },
};

// fakeWorkspaces は、get だけを byId(ワークスペースの内部 ID → 設定)で返す口。ないものは settings。
export function fakeWorkspaces(byId: Record<string, WorkspaceSettings> = {}): WorkspacesPort {
  return {
    ...fakeWorkspacesPort(),
    get: async (storeId, id) => ({ id, storeId, name: "Web", prefix: "HH", oldPrefixes: [], settings: byId[id] ?? settings }),
  };
}
```

(`WorkspaceDetail` の項目が B2 のマージ後に違っていたら、それに合わせる。)

`fixtures.test-util.ts`:

- `item()` の返す値に `status: "st1", priority: "pr2", assignee: "", start: "", end: "", points: null, estimate: null, systems: [], choices: {}, fields: {}` を足す。
- `fakeItemsPort` に次を足す。`FakeItems` に `refuseNext(message: string): void`・`holdCreate(): { release: () => void }`・`createdValues: Record<string, PropertyInput>[]` を足す。

```ts
  let refusal = "";
  let hold: Promise<void> | null = null;
  const createdValues: Record<string, PropertyInput>[] = [];
  // takeRefusal は、refuseNext で決めた断りを 1 回だけ投げる。
  const takeRefusal = () => {
    if (refusal === "") return;
    const m = refusal;
    refusal = "";
    throw new Refused(m);
  };
  // set は、プロパティの値をアイテムに入れる(テスト用の簡単な形。組み込みはキー、追加は fields)。
  const set = (it: Item, propertyId: string, value: PropertyInput) => {
    const builtin = ["status", "priority", "assignee", "start", "end"];
    if (builtin.includes(propertyId)) (it as unknown as Record<string, PropertyInput>)[propertyId] = value ?? "";
    else if (propertyId === "points" || propertyId === "estimate") it[propertyId] = value === "" || value === null ? null : Number(value);
    else it.fields = { ...it.fields, [propertyId]: value };
  };
```

```ts
    refuseNext(message) { refusal = message; },
    holdCreate() {
      const g = gate();
      hold = g.wait;
      return { release: g.release };
    },
    createdValues,
    async setProperty(_s, id, propertyId, value) {
      calls.push(`setProperty ${id} ${propertyId} ${JSON.stringify(value)}`);
      takeRefusal();
      const it = find(id);
      set(it, propertyId, value);
      return it;
    },
    async addValue(_s, id, propertyId, valueId) {
      calls.push(`addValue ${id} ${propertyId} ${valueId}`);
      const it = find(id);
      if (propertyId === "systems") it.systems = [...it.systems, valueId];
      return it;
    },
    async removeValue(_s, id, propertyId, valueId) {
      calls.push(`removeValue ${id} ${propertyId} ${valueId}`);
      const it = find(id);
      if (propertyId === "systems") it.systems = it.systems.filter((x) => x !== valueId);
      return it;
    },
    async checkStep(_s, id, done) {
      calls.push(`checkStep ${id} ${done}`);
      const it = find(id);
      it.status = done ? "st3" : "st1";
      return it;
    },
```

既存の `create` の先頭で `if (hold) { const h = hold; hold = null; await h; }`、続けて `takeRefusal()` を呼び、`createdValues.push(input.values)` をする。作るアイテムには `item()` と同じ値の初期値を入れ、`input.values` の `status`・`priority` があれば上書きする。

- [ ] **Step 2: 失敗するテストを書く**

`properties.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { item } from "./fixtures.test-util";
import { deletedValue, propertyRows, statusOptions, valueOf, visibleProperties } from "./properties";
import { settings } from "./settings.test-util";

const task = { ...item("S1", "S1-P1", 2, "Task", "S1-I1"), status: "st2", priority: "gone", assignee: "taro",
  estimate: 1.5, systems: ["sy2"], choices: { F1: ["C2"] }, fields: { F2: "01CHOICE00000000000000000A" } };
const people = { members: [{ login: "taro", name: "太郎" }, { login: "hanako", name: "花子" }], me: "hanako" };
const byId = (id: string) => settings.properties.find((p) => p.id === id)!;

describe("properties", () => {
  it("shows visible properties for the kind in settings order", () => {
    expect(visibleProperties(settings, "Task").map((p) => p.id)).toEqual(
      ["status", "assignee", "priority", "start", "estimate", "systems", "F1", "F2"],
    );
    expect(visibleProperties(settings, "Story").map((p) => p.id)).toContain("points");
  });

  it("reads values by type, an ID-shaped text as text", () => {
    expect(valueOf(task, byId("estimate"))).toBe("1.5");
    expect(valueOf(task, byId("systems"))).toEqual(["sy2"]);
    expect(valueOf(task, byId("F1"))).toBe("C2");
    expect(valueOf(task, byId("F2"))).toBe("01CHOICE00000000000000000A");
  });

  it("labels values and marks deleted ones", () => {
    const rows = propertyRows(settings, "Task", (p) => valueOf(task, p), people);
    const row = (id: string) => rows.find((r) => r.property.id === id)!;
    expect(row("status").text).toBe("InProgress");
    expect(row("priority")).toMatchObject({ text: deletedValue, deleted: true });
    expect(row("systems").text).toBe("API");
    expect(row("F1")).toMatchObject({ text: "検証", extra: true });
    expect(row("F1").options[0]).toMatchObject({ id: "", label: "なし" });
    expect(row("assignee").options.map((o) => o.label)).toEqual(["未割り当て", "太郎", "花子(あなた)"]);
    expect(row("assignee").text).toBe("太郎");
  });

  it("lists statuses with their category and checks the current one", () => {
    expect(statusOptions(settings, "st2").slice(0, 2)).toEqual([
      { id: "st1", label: "Todo", hint: "未着手", checked: false },
      { id: "st2", label: "InProgress", hint: "進行中", checked: true },
    ]);
  });
});
```

- [ ] **Step 3: 失敗を確かめる** — Run: `cd web && pnpm --filter @hedgehug/core test properties` / Expected: FAIL(`properties.ts` がない)

- [ ] **Step 4: 実装する**

`model/item.ts` と `port/items.ts` を「型と口」のとおりに変える。`api/src/items.ts` の `toItem` に値を写す(`status: it.status, priority: it.priority, assignee: it.assignee, start: it.start, end: it.end, points: it.points ?? null, estimate: it.estimate ?? null, systems: it.systems ?? [], choices: it.choices ?? {}, fields: it.fields ?? {}`)。口の 4 つを足す:

```ts
    setProperty: (storeId, itemId, propertyId, value) =>
      call(async () => toItem(storeId, await setItemProperty(storeId, itemId, propertyId, { value }))),
    addValue: (storeId, itemId, propertyId, valueId) =>
      call(async () => toItem(storeId, await addItemValue(storeId, itemId, propertyId, valueId))),
    removeValue: (storeId, itemId, propertyId, valueId) =>
      call(async () => toItem(storeId, await removeItemValue(storeId, itemId, propertyId, valueId))),
    checkStep: (storeId, itemId, done) =>
      call(async () => toItem(storeId, await checkStep(storeId, itemId, { done }))),
```

`create` の本文に `values: input.values` を足す。

`features/items/properties.ts`:

```ts
import type { Item, Kind, PropertyInput } from "../../model/item";
import type { Member } from "../../model/member";
import type { Category, WorkspaceProperty, WorkspaceSettings, WorkspaceValue } from "../../model/workspace";

// 詳細パネルと作成ダイアログのプロパティの行(02「プロパティの入力と保存」、決定 19〜21)。

export const deletedValue = "(削除された値)";
export const categoryLabels: Record<Category, string> = {
  todo: "未着手",
  doing: "進行中",
  done: "完了",
  canceled: "中止",
};

export interface ValueOption {
  id: string;
  label: string;
  hint?: string;
  checked: boolean;
}
export interface People {
  members: Member[];
  me: string;
}
export interface PropertyRow {
  property: WorkspaceProperty;
  extra: boolean;
  value: PropertyInput;
  text: string;
  deleted: boolean;
  options: ValueOption[];
}

export type ItemValues = Pick<
  Item,
  "status" | "priority" | "assignee" | "start" | "end" | "points" | "estimate" | "systems" | "choices" | "fields"
>;

const chooser = ["status", "priority", "systems", "single", "multi"];

// visibleProperties は、非表示でなく、その種類で使うプロパティ(設定の並び)。
export function visibleProperties(settings: WorkspaceSettings, kind: Kind): WorkspaceProperty[] {
  return settings.properties.filter((p) => !p.hidden && p.kinds.includes(kind));
}

// valueOf は、アイテムの値を、入力欄に入れる形で返す。数は文字列、複数は ID の配列、チェックは真偽、空は ""。
export function valueOf(it: ItemValues, p: WorkspaceProperty): PropertyInput {
  if (p.builtin) return builtinValue(it, p.key);
  if (p.type === "multi") return it.choices[p.id] ?? [];
  if (p.type === "single") return it.choices[p.id]?.[0] ?? "";
  const raw = it.fields[p.id];
  if (p.type === "check") return raw === true;
  return raw === undefined || raw === null ? "" : String(raw);
}

function builtinValue(it: ItemValues, key: string): PropertyInput {
  switch (key) {
    case "points":
      return it.points === null ? "" : String(it.points);
    case "estimate":
      return it.estimate === null ? "" : String(it.estimate);
    case "systems":
      return it.systems;
    case "status":
    case "priority":
    case "assignee":
    case "start":
    case "end":
      return it[key];
  }
  return "";
}

function listOf(settings: WorkspaceSettings, p: WorkspaceProperty): WorkspaceValue[] {
  if (p.type === "status") return settings.statuses;
  if (p.type === "priority") return settings.priorities;
  if (p.type === "systems") return settings.systems;
  return p.choices;
}

// statusOptions は、ステータスのメニューの項目(決定 21)。使わないステータスの扱いは B5 の後(決定 3)。
export function statusOptions(settings: WorkspaceSettings, current: string): ValueOption[] {
  return settings.statuses.map((s) => ({
    id: s.id,
    label: s.name,
    hint: s.category ? categoryLabels[s.category] : undefined,
    checked: s.id === current,
  }));
}

function peopleOptions(people: People, current: string): ValueOption[] {
  const me = people.me.toLowerCase();
  return [
    { id: "", label: "未割り当て", checked: current === "" },
    ...people.members.map((m) => ({
      id: m.login,
      label: m.login.toLowerCase() === me ? `${m.name}(あなた)` : m.name,
      checked: m.login === current,
    })),
  ];
}

function choiceRow(settings: WorkspaceSettings, p: WorkspaceProperty, value: PropertyInput) {
  const list = listOf(settings, p);
  const ids = Array.isArray(value) ? value : value ? [String(value)] : [];
  const names = ids.map((id) => list.find((v) => v.id === id)?.name);
  const required = p.type === "status" || p.type === "priority";
  const deleted = names.some((n) => n === undefined) || (required && ids.length === 0);
  const text = deleted ? deletedValue : names.length > 0 ? names.join(", ") : "なし";
  const options =
    p.type === "status"
      ? statusOptions(settings, ids[0] ?? "")
      : list.map((v) => ({ id: v.id, label: v.name, checked: ids.includes(v.id) }));
  if (p.type === "single") options.unshift({ id: "", label: "なし", checked: ids.length === 0 });
  return { text, deleted, options };
}

// propertyRows は、種類で使う表示中のプロパティの行。values はプロパティの入力欄の値を返す。
export function propertyRows(
  settings: WorkspaceSettings,
  kind: Kind,
  values: (p: WorkspaceProperty) => PropertyInput,
  people: People,
): PropertyRow[] {
  return visibleProperties(settings, kind).map((p) => {
    const value = values(p);
    const base = { property: p, extra: !p.builtin, value };
    if (chooser.includes(p.type)) return { ...base, ...choiceRow(settings, p, value) };
    if (p.type === "user") {
      const options = peopleOptions(people, String(value));
      // メンバーにいない人は、ログイン名をそのまま出す。
      const text = options.find((o) => o.checked)?.label ?? String(value);
      return { ...base, text, deleted: false, options };
    }
    return { ...base, text: String(value), deleted: false, options: [] };
  });
}
```

(150 行を超えたら、`choiceRow`・`peopleOptions`・`statusOptions` を `propertyOptions.ts` に分ける。)`index.ts` に `properties.ts` の export を足す。

- [ ] **Step 5: 通るのを確かめる** — Run: `cd web && pnpm --filter @hedgehug/core test && pnpm --filter @hedgehug/api test && pnpm check` / Expected: PASS
- [ ] **Step 6: コミット** — `git commit -m "feat(core): アイテムの値の型・口と、プロパティの行を足す(A3 前半 Task 8)"`

---

### Task 9: 詳細パネルのプロパティ

**Files:**
- Modify: `web/packages/core/src/features/items/detail.ts`
- Create: `web/app/src/screens/item/PropertyField.svelte`、`web/app/src/screens/item/DetailProperties.svelte`、`web/app/src/screens/item/StatusButton.svelte`
- Modify: `web/app/src/screens/item/DetailPanel.svelte`、`web/app/src/wire.ts`
- Test: `web/packages/core/src/features/items/detail.test.ts`(追記と `setup` の変更)

**Interfaces:**
- Consumes: Task 8、B2 の `WorkspacesPort.get`、`CatalogPort.listMembers`・`me`
- Produces: `DetailView.settings`・`people`・`rows`、`Detail.setProperty(itemId, propertyId, value): Promise<boolean>`、`Detail.toggleValue(propertyId, valueId, on): Promise<void>`、`Detail.checkStep(stepId, done): Promise<void>`、`StatusButton.svelte`(props: `status`・`settings`・`disabled`・`onchange(statusId)`)、`PropertyField.svelte`(props: `row`・`disabled`・`statusSelect?`・`onchange(value)`・`ontoggle(valueId, on)`)

- [ ] **Step 1: 失敗するテストを書く**

`detail.test.ts` の `setup` の `createDetail` に `workspaces: fakeWorkspaces()` と `catalogPort: fakeCatalogPort()` を足す(`import { fakeWorkspaces } from "./settings.test-util"`)。テストを足す:

```ts
  it("loads settings, members and property rows with the item", async () => {
    const { detail, task } = await setup();
    await detail.open("S1", task.id);
    const v = get(detail.view);
    expect(v?.rows.map((r) => r.property.id)).toEqual(
      ["status", "assignee", "priority", "start", "estimate", "systems", "F1", "F2"],
    );
    expect(v?.people).toEqual({ members: [{ login: "taro", name: "太郎" }], me: "taro" });
  });

  it("saves a property and reloads", async () => {
    const { detail, items, task, changed } = await setup();
    await detail.open("S1", task.id);
    expect(await detail.setProperty(task.id, "points", "3")).toBe(true);
    expect(items.calls).toEqual([`setProperty ${task.id} points "3"`]);
    expect(changed()).toBe(1);
  });

  it("shows a refusal as an error toast", async () => {
    const { detail, items, toasts, task } = await setup();
    await detail.open("S1", task.id);
    items.refuseNext("ポイントは 0 以上の数で入力してください");
    expect(await detail.setProperty(task.id, "points", "-1")).toBe(false);
    expect(get(toasts.list)[0]).toMatchObject({ text: "ポイントは 0 以上の数で入力してください", error: true });
  });

  it("toggles a multi value with add and remove", async () => {
    const { detail, items, task } = await setup();
    await detail.open("S1", task.id);
    await detail.toggleValue("systems", "sy1", true);
    await detail.toggleValue("systems", "sy1", false);
    expect(items.calls).toEqual([`addValue ${task.id} systems sy1`, `removeValue ${task.id} systems sy1`]);
  });

  it("checks a Step", async () => {
    const { detail, items, task, s1 } = await setup();
    await detail.open("S1", task.id);
    await detail.checkStep(s1.id, true);
    expect(items.calls).toEqual([`checkStep ${s1.id} true`]);
    expect(get(detail.view)?.children.find((c) => c.id === s1.id)?.status).toBe("st3");
  });
```

- [ ] **Step 2: 失敗を確かめる** — Run: `cd web && pnpm --filter @hedgehug/core test detail` / Expected: FAIL

- [ ] **Step 3: core を実装する**

`detail.ts`:

- `DetailDeps` に `workspaces: WorkspacesPort` と `catalogPort: CatalogPort` を足す。
- `DetailView` に `settings: WorkspaceSettings`・`people: People`・`rows: PropertyRow[]` を足す。
- `load` を次にする(`me` が決まらないときは空。A2 の決定 3):

```ts
  async function load(storeId: string, itemId: string) {
    latest += 1;
    const mine = latest;
    try {
      const item = await deps.items.get(storeId, itemId);
      const [all, ws, members, me] = await Promise.all([
        deps.items.list(storeId, item.projectId),
        deps.workspaces.get(storeId, item.workspaceId),
        deps.catalogPort.listMembers(storeId),
        deps.catalogPort.me(storeId).catch(() => ""),
      ]);
      const store = deps.catalog.store(storeId);
      const project = deps.catalog.project(storeId, item.projectId);
      const workspace = project && deps.catalog.workspace(storeId, project.workspaceId);
      if (mine !== latest) return;
      if (!store || !project || !workspace) {
        view.set(null);
        return;
      }
      const people = { members, me };
      view.set({
        store,
        workspace,
        project,
        item,
        displayId: itemDisplayId(workspace.prefix, item.number),
        ancestors: ancestorsOf(all, item.id),
        children: all.filter((it) => it.parentId === item.id).sort(compareSiblings),
        readOnly: store.readOnly,
        settings: ws.settings,
        people,
        rows: propertyRows(ws.settings, item.kind, (p) => valueOf(item, p), people),
      });
    } catch (e) {
      // 開いているアイテムがなくなったら、パネルを閉じる(02「詳細パネル」)。
      if (!(e instanceof NotFound)) throw e;
      if (mine === latest) view.set(null);
    }
  }
```

- 保存の 3 つを足す。どれも `rename` と同じく、断りはエラーのトースト(決定 17)、保存したら `saved(gen)`:

```ts
    async setProperty(itemId, propertyId, value) {
      const v = get(view);
      if (!v) return false;
      const gen = latest;
      try {
        await deps.items.setProperty(v.store.id, itemId, propertyId, value);
      } catch (e) {
        refusedToast(e);
        return false;
      }
      await saved(gen);
      return true;
    },
    async toggleValue(propertyId, valueId, on) {
      const v = get(view);
      if (!v) return;
      const gen = latest;
      const send = on ? deps.items.addValue : deps.items.removeValue;
      try {
        await send(v.store.id, v.item.id, propertyId, valueId);
      } catch (e) {
        refusedToast(e);
        return;
      }
      await saved(gen);
    },
    async checkStep(stepId, done) {
      const v = get(view);
      if (!v) return;
      const gen = latest;
      try {
        await deps.items.checkStep(v.store.id, stepId, done);
      } catch (e) {
        refusedToast(e);
        return;
      }
      await saved(gen);
    },
```

(`detail.ts` が 300 行を超えるなら、3 つを `detailValues.ts` の `valueActions(view, deps, saved, refusedToast)` に分け、`createDetail` の返す値に広げる。)

- [ ] **Step 4: 通るのを確かめる** — Run: `cd web && pnpm --filter @hedgehug/core test` / Expected: PASS

- [ ] **Step 5: 画面をつくる**

`StatusButton.svelte`:

```svelte
<script lang="ts">
  import { deletedValue, readOnlyNote, statusOptions, type WorkspaceSettings } from "@hedgehug/core";
  import { StatusDot, ValueMenu } from "@hedgehug/ui";

  // ステータスのボタンとメニュー(02「ステータスの変更」、決定 21)。いまの値を選び直しても保存しない。
  interface Props {
    status: string;
    settings: WorkspaceSettings;
    disabled: boolean;
    onchange: (statusId: string) => void;
  }

  let { status, settings, disabled, onchange }: Props = $props();
  let menu = $state<{ x: number; y: number } | null>(null);
  const current = $derived(settings.statuses.find((s) => s.id === status));
  const name = $derived(current?.name ?? deletedValue);

  function open(event: MouseEvent) {
    const r = (event.currentTarget as HTMLElement).getBoundingClientRect();
    menu = { x: r.left, y: r.bottom + 4 };
  }
</script>

<button
  type="button"
  class="status-btn"
  class:deleted={!current}
  {disabled}
  title={disabled ? readOnlyNote : undefined}
  aria-label="ステータス: {name}"
  onclick={open}
>
  {#if current}<StatusDot category={current.category ?? "todo"} label={current.name} />{/if}
  {name}
</button>
<ValueMenu
  open={menu !== null}
  x={menu?.x ?? 0}
  y={menu?.y ?? 0}
  label="ステータス"
  options={statusOptions(settings, status)}
  onselect={(id) => {
    if (id !== status) onchange(id);
  }}
  onclose={() => (menu = null)}
/>

<style>
  .status-btn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: 12px;
  }
  .deleted {
    color: var(--muted);
  }
</style>
```

(`aria-label` の「ステータス: {name}」は読み上げとテストのための名前で、画面に出る文言ではない。)

`PropertyField.svelte`(詳細パネルと作成ダイアログで共用。保存はせず、値を返す):

```svelte
<script lang="ts">
  import { type PropertyInput, type PropertyRow, readOnlyNote, type WorkspaceSettings } from "@hedgehug/core";
  import { Input, Select, ValueMenu } from "@hedgehug/ui";
  import StatusButton from "./StatusButton.svelte";

  // 1 つのプロパティの入力(02「入力の方法と保存」の表、決定 20)。onchange は、選ぶもの・チェックは選んだとき、
  // 入力欄はフォーカスが外れたとき(Enter でも)。ontoggle は関連システム・複数選択の入り切り。
  // statusSelect なら、ステータスをセレクトで出す(作成ダイアログ。02「ダイアログの項目」)。
  interface Props {
    row: PropertyRow;
    settings: WorkspaceSettings;
    disabled: boolean;
    statusSelect?: boolean;
    onchange: (value: PropertyInput) => void;
    ontoggle: (valueId: string, on: boolean) => void;
  }

  let { row, settings, disabled, statusSelect = false, onchange, ontoggle }: Props = $props();
  const p = $derived(row.property);
  const note = $derived(disabled ? readOnlyNote : undefined);
  const multi = $derived(p.type === "systems" || p.type === "multi");
  const builtinNumber = $derived(p.builtin && (p.key === "points" || p.key === "estimate"));
  let draft = $state("");
  let selected = $state("");
  let menu = $state<{ x: number; y: number } | null>(null);
  $effect(() => {
    draft = typeof row.value === "string" ? row.value : "";
    selected = draft;
  });

  function openMenu(event: MouseEvent) {
    const r = (event.currentTarget as HTMLElement).getBoundingClientRect();
    menu = { x: r.left, y: r.bottom + 4 };
  }
  function pick(id: string) {
    if (multi) ontoggle(id, !row.options.find((o) => o.id === id)?.checked);
    else if (id !== row.value) onchange(id);
  }
  // 数の欄に数でない文字があるとき: ポイント・見積もり時間はサーバーに断らせ(決定 16)、ほかは送らずに戻す。
  function commit(event: Event) {
    const bad = (event.currentTarget as HTMLInputElement).validity?.badInput ?? false;
    if (bad && !builtinNumber) {
      draft = String(row.value ?? "");
      return;
    }
    const value = bad ? "x" : draft;
    if (value !== row.value) onchange(value);
  }
  function enter(event: KeyboardEvent) {
    if (event.key === "Enter" && !event.isComposing) (event.currentTarget as HTMLElement).blur();
  }
</script>

{#if p.type === "status" && !statusSelect}
  <StatusButton status={String(row.value)} {settings} {disabled} onchange={(id) => onchange(id)} />
{:else if p.type === "status" || p.type === "user"}
  <Select
    label={p.name}
    {disabled}
    title={note}
    options={row.options.map((o) => ({ value: o.id, label: o.label }))}
    bind:value={selected}
    onchange={() => onchange(selected)}
  />
{:else if row.options.length > 0 || multi || p.type === "priority" || p.type === "single"}
  <button type="button" class="value-btn" class:deleted={row.deleted} {disabled} title={note}
    aria-label="{p.name}: {row.text}" onclick={openMenu}>{row.text}</button>
  <ValueMenu open={menu !== null} x={menu?.x ?? 0} y={menu?.y ?? 0} label={p.name}
    options={row.options} {multi} onselect={pick} onclose={() => (menu = null)} />
{:else if p.type === "check"}
  <input type="checkbox" aria-label={p.name} checked={row.value === true} {disabled} title={note}
    onchange={() => onchange(row.value !== true)} />
{:else if p.type === "date" || p.type === "number" || p.type === "duration"}
  <input
    class="field"
    type={p.type === "date" ? "date" : "number"}
    step={p.type === "duration" ? "0.25" : "any"}
    aria-label={p.name}
    {disabled}
    title={note}
    bind:value={draft}
    onblur={commit}
    onkeydown={enter}
  />
{:else}
  <Input label={p.name} {disabled} title={note} bind:value={draft} onblur={commit} onkeydown={enter} />
{/if}
```

(`Select` の `onchange` は Task 7 で足す。`Input` の `onblur`・`onkeydown`・`title`・`disabled` は A2 で足してある(Task 9 の実装の前に `Input.svelte` の props を見て、名前が違えば合わせる)。スタイル: `.value-btn` は `StatusButton` と同じ大きさ、`.deleted` は `color: var(--muted)`、`.field` は `Input` と同じ高さ。)

`DetailProperties.svelte`:

```svelte
<script lang="ts">
  import type { DetailView, PropertyInput } from "@hedgehug/core";
  import { detail, links } from "../../wire";
  import PropertyField from "./PropertyField.svelte";

  // 詳細パネルのプロパティの区画(02「上から順の構成」、決定 19)。保存できなかったら、入力を保存済みの値に戻す。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();
  let version = $state(0);
  // B2 のワークスペースの設定タブのアドレス(/s/{store}/w/{prefix}/settings)。Route の形は B2 のマージ後の型に合わせる。
  const settingsRoute = $derived({
    name: "workspace" as const,
    store: v.store.name,
    workspace: v.workspace.prefix,
    tab: "settings" as const,
  });

  async function change(propertyId: string, value: PropertyInput) {
    if (!(await detail.setProperty(v.item.id, propertyId, value))) version += 1;
  }
</script>

<section class="peek-sec" aria-label="プロパティ">
  <dl class="props">
    {#each v.rows as row (`${row.property.id}-${version}`)}
      <dt>
        {row.property.name}
        {#if row.extra}<span class="extra">追加</span>{/if}
      </dt>
      <dd>
        <PropertyField
          {row}
          settings={v.settings}
          disabled={v.readOnly}
          onchange={(value) => void change(row.property.id, value)}
          ontoggle={(id, on) => void detail.toggleValue(row.property.id, id, on)}
        />
      </dd>
    {/each}
  </dl>
  <a
    class="props-link"
    href={links.href(settingsRoute)}
    onclick={(e) => links.click(e, settingsRoute)}>プロパティを追加・変更</a
  >
</section>
```

(スタイル: `.props` は 2 列のグリッド(名前 96px と値)。全画面ではそのまま右の列に入る。`.extra` は小さい薄い字。「プロパティを追加・変更」は閲覧のみの Store でも押せる(02 の閲覧のみの例外)。)

`DetailPanel.svelte` のタイトルの `Input` の下に `<DetailProperties {v} />` を置く。`wire.ts` の `createDetail` の引数に `workspaces`(B2 が `wire.ts` に作った `WorkspacesPort` の実装)と `catalogPort` を渡す。

- [ ] **Step 6: 画面を確かめる** — Run: `cd web && pnpm check && pnpm e2e detail.spec.ts` / Expected: PASS(既存の詳細パネルの e2e が通る)
- [ ] **Step 7: コミット** — `git commit -m "feat(web): 詳細パネルでプロパティを入れて保存する(A3 前半 Task 9)"`

---

### Task 10: 作成ダイアログのプロパティと連動、A2 の申し送り

**Files:**
- Create: `web/packages/core/src/features/items/createValues.ts`、`web/packages/core/src/features/items/createDialogLoad.ts`、`web/app/src/screens/item/CreatePropertyFields.svelte`
- Modify: `web/packages/core/src/features/items/createDialog.ts`、`web/app/src/screens/item/CreateItemFields.svelte`、`web/app/src/wire.ts`
- Test: `web/packages/core/src/features/items/createValues.test.ts`、`web/packages/core/src/features/items/createDialog.test.ts`(追記と `setup` の変更)

**Interfaces:**
- Consumes: Task 8・9
- Produces: `initialValues(settings)`・`switchWorkspace(values, next, member)`・`shownValues(values, settings, kind)`、`ItemForm.values`、`ItemDialogState.rows`・`settings`

- [ ] **Step 1: 失敗するテストを書く**

`createValues.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { initialValues, shownValues, switchWorkspace } from "./createValues";
import { settings, settings2 } from "./settings.test-util";

describe("create values", () => {
  it("starts with the initial status and priority", () => {
    expect(initialValues(settings)).toEqual({ status: "st1", priority: "pr2" });
  });

  it("keeps assignee, dates, points and estimate when the workspace changes", () => {
    const values = { status: "st2", priority: "pr1", assignee: "taro", start: "2026-11-11", estimate: "2", systems: ["sy1"], F1: "C1" };
    expect(switchWorkspace(values, settings2, () => true)).toEqual({
      status: "ns1", priority: "np1", assignee: "taro", start: "2026-11-11", estimate: "2",
    });
  });

  it("drops the assignee who is not a member of the new store", () => {
    expect(switchWorkspace({ assignee: "taro" }, settings2, () => false).assignee).toBeUndefined();
  });

  it("sends only the values shown for the kind", () => {
    const values = { status: "st1", priority: "pr2", points: "3", end: "2026-11-20", F2: "" };
    expect(shownValues(values, settings, "Task")).toEqual({ status: "st1", priority: "pr2" });
    expect(shownValues(values, settings, "Story")).toEqual({ status: "st1", priority: "pr2", points: "3" });
  });
});
```

`createDialog.test.ts` の `setup` の `createItemDialog` に `workspaces: fakeWorkspaces()` と `catalogPort: fakeCatalogPort()` を足し、テストを足す:

```ts
  it("starts with the initial values and property rows", async () => {
    const { dialog } = await setup({ workspaceId: "S1-W1", projectId: "S1-P1" });
    await dialog.open();
    const s = get(dialog.state);
    expect(s?.form.values).toEqual({ status: "st1", priority: "pr2" });
    expect(s?.rows.map((r) => r.property.id)).toContain("assignee");
  });

  it("sends the shown values with the item", async () => {
    const { dialog, items } = await setup({ workspaceId: "S1-W1", projectId: "S1-P1" });
    await dialog.open();
    const values = { ...get(dialog.state)!.form.values, assignee: "taro" };
    await dialog.update({ title: "値つき", values });
    await dialog.submit();
    expect(items.createdValues[0]).toEqual({ status: "st1", priority: "pr2", assignee: "taro" });
  });

  it("hides a property the kind does not use and restores its value", async () => {
    const { dialog } = await setup({ workspaceId: "S1-W1", projectId: "S1-P1" });
    await dialog.open({ kind: "Story" });
    await dialog.update({ values: { ...get(dialog.state)!.form.values, points: "3" } });
    await dialog.update({ kind: "Task" });
    expect(get(dialog.state)?.rows.map((r) => r.property.id)).not.toContain("points");
    await dialog.update({ kind: "Story" });
    expect(get(dialog.state)?.form.values.points).toBe("3");
  });

  // A2 の申し送り(決定 27)。
  it("keeps what was typed while sending when the server refuses", async () => {
    const { dialog, items } = await setup({ workspaceId: "S1-W1", projectId: "S1-P1" });
    await dialog.open();
    await dialog.update({ title: "a" });
    const held = items.holdCreate();
    items.refuseNext("ポイントは 0 以上の数で入力してください");
    const sending = dialog.submit();
    await dialog.update({ title: "ab" });
    held.release();
    await sending;
    expect(get(dialog.state)?.form.title).toBe("ab");
    expect(get(dialog.state)?.error).toBe("ポイントは 0 以上の数で入力してください");
  });
```

- [ ] **Step 2: 失敗を確かめる** — Run: `cd web && pnpm --filter @hedgehug/core test create` / Expected: FAIL

- [ ] **Step 3: 実装する**

`createValues.ts`:

```ts
import type { Kind, PropertyInput } from "../../model/item";
import type { WorkspaceSettings } from "../../model/workspace";
import { visibleProperties } from "./properties";

// 作成ダイアログの値(02「ダイアログの中での連動」のテンプレート以外、決定 25)。values はプロパティの
// 内部 ID → 入力。隠した欄の値も覚えておく(使える種類に戻すと値が戻る)。

export type Values = Record<string, PropertyInput>;

const kept = ["assignee", "start", "end", "points", "estimate"];

export function initialValues(settings: WorkspaceSettings): Values {
  return { status: settings.initialStatus, priority: settings.initialPriority };
}

// switchWorkspace は、ワークスペースを変えたときの値。担当者・開始日・終了日・ポイント・見積もり時間を残し、
// ほかは初期値に戻す。担当者は member(login) が true のときだけ残す。
export function switchWorkspace(values: Values, next: WorkspaceSettings, member: (login: string) => boolean): Values {
  const out = initialValues(next);
  for (const k of kept) {
    const v = values[k];
    if (v === undefined || v === "") continue;
    if (k === "assignee" && !member(String(v))) continue;
    out[k] = v;
  }
  return out;
}

// shownValues は、その種類で表示している欄の値だけ(「作成」で送るもの)。空の値は送らない。
export function shownValues(values: Values, settings: WorkspaceSettings, kind: Kind): Values {
  const out: Values = {};
  for (const p of visibleProperties(settings, kind)) {
    const v = values[p.id];
    if (v === undefined || v === "" || (Array.isArray(v) && v.length === 0)) continue;
    out[p.id] = v;
  }
  return out;
}
```

`createDialogLoad.ts`:

```ts
import type { WorkspacesPort } from "../../port/workspaces";
import type { CatalogPort } from "../../port/catalog";
import type { WorkspaceSettings } from "../../model/workspace";
import type { People } from "./properties";

export interface WorkspaceContext {
  settings: WorkspaceSettings;
  people: People;
}

// createContextLoader は、ワークスペースの設定とメンバー・あなたを読み、ワークスペースごとに覚える。
export function createContextLoader(workspaces: WorkspacesPort, catalog: CatalogPort) {
  const cache = new Map<string, Promise<WorkspaceContext>>();
  return (storeId: string, workspaceId: string): Promise<WorkspaceContext> => {
    const key = `${storeId}/${workspaceId}`;
    let p = cache.get(key);
    if (!p) {
      p = Promise.all([
        workspaces.get(storeId, workspaceId),
        catalog.listMembers(storeId),
        catalog.me(storeId).catch(() => ""),
      ]).then(([d, members, me]) => ({ settings: d.settings, people: { members, me } }));
      cache.set(key, p);
    }
    return p;
  };
}
```

(覚えるのはダイアログを開いているあいだだけにする。`open` のたびに `createContextLoader` を作り直す。)

`createDialog.ts`:

- `ItemForm` に `values: Values`、`ItemDialogState` に `rows: PropertyRow[]` と `settings: WorkspaceSettings` を足す。`ItemDialogDeps` に `workspaces: WorkspacesPort` と `catalogPort: CatalogPort` を足す。
- `refresh` で `const ctx = await load(store.id, ws.id)` を読み、`rows: propertyRows(ctx.settings, form.kind, (p) => form.values[p.id] ?? "", ctx.people)`・`settings: ctx.settings` を返す。
- `open` で、読んだ設定から `values: initialValues(ctx.settings)` を入れる(`refresh` の前に `load` を 1 回呼ぶ)。
- `update`:
  - ワークスペースが変わったら、新しいワークスペースの `ctx` を読み、`form.values = switchWorkspace(s.form.values, next.settings, (login) => sameStore || next.people.members.some((m) => m.login.toLowerCase() === login.toLowerCase()))`(`sameStore` は前と同じ Store か)。
  - 種類を変えたときは、`values` を変えずに `rows` だけを新しい種類で作り直す(隠した値を覚えておく)。
  - `patch.values` が来たら、`rows` を作り直す(`propertyRows` を同じ設定で呼ぶ)。
- `send` で、`items.create` に `values: shownValues(s.form.values, s.settings, kind)` を渡す。
- 申し送りの直し(決定 27): 断りを受けたら `const now = get(state); if (now) state.set({ ...now, error: e.message });`(送った時点の `s` で上書きしない)。続けて作成のときも、`get(state)` のいまの入力を元に、タイトル・エラーだけを空にする。
- 300 行を超えないよう、`rows` を作る小さな関数 `rowsFor(ctx, form)` を `createDialogLoad.ts` に置く。

`CreatePropertyFields.svelte`:

```svelte
<script lang="ts">
  import type { ItemDialogState, PropertyInput } from "@hedgehug/core";
  import { itemDialog } from "../../wire";
  import PropertyField from "../item/PropertyField.svelte";

  // 作成ダイアログのプロパティ欄(02「ダイアログの項目」、決定 25)。ステータスはセレクトで出す。
  interface Props {
    s: ItemDialogState;
  }

  let { s }: Props = $props();

  function set(id: string, value: PropertyInput) {
    void itemDialog.update({ values: { ...s.form.values, [id]: value } });
  }
  function toggle(id: string, valueId: string, on: boolean) {
    const now = (s.form.values[id] as string[] | undefined) ?? [];
    set(id, on ? [...now, valueId] : now.filter((x) => x !== valueId));
  }
</script>

<div class="create-props">
  {#each s.rows as row (row.property.id)}
    <label class="create-prop">
      <span>{row.property.name}</span>
      <PropertyField
        {row}
        settings={s.settings}
        disabled={false}
        statusSelect
        onchange={(value) => set(row.property.id, value)}
        ontoggle={(valueId, on) => toggle(row.property.id, valueId, on)}
      />
    </label>
  {/each}
</div>
```

`CreateItemFields.svelte` のタイトルの欄の下に `<CreatePropertyFields {s} />` を置く(200 行を超えない)。`wire.ts` の `createItemDialog` に `workspaces`・`catalogPort` を渡す。

- [ ] **Step 4: 通るのを確かめる** — Run: `cd web && pnpm --filter @hedgehug/core test && pnpm check && pnpm e2e create.spec.ts` / Expected: PASS
- [ ] **Step 5: コミット** — `git commit -m "feat(web): 作成ダイアログのプロパティと値の連動を足し、送信中の入力を断りのあとも残す(A3 前半 Task 10)"`

---

### Task 11: ツリーの行と詳細パネルの子の行、Step のチェック

**Files:**
- Create: `web/packages/core/src/features/items/progress.ts`、`web/app/src/screens/item/StepRow.svelte`
- Modify: `web/packages/core/src/features/projects/screen.ts`、`web/app/src/screens/project/TreeRow.svelte`・`ProjectTree.svelte`・`ProjectScreen.svelte`、`web/app/src/screens/item/DetailChildren.svelte`、`web/app/src/wire.ts`
- Test: `web/packages/core/src/features/items/progress.test.ts`、`web/packages/core/src/features/projects/screen.test.ts`(追記と `setup` の変更)

**Interfaces:**
- Consumes: Task 7〜9
- Produces: `categoryOf`・`childCount`・`stepProgress`・`dueLabel`・`formatDate`、`ProjectView.settings`、`ProjectScreen.setStatus(itemId, statusId): Promise<void>`、`createProjectScreen(catalog, catalogPort, items, workspaces, toasts)`

- [ ] **Step 1: 失敗するテストを書く**

`progress.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { item } from "./fixtures.test-util";
import { childCount, dueLabel, formatDate, stepProgress } from "./progress";
import { settings } from "./settings.test-util";

const step = (n: number, status: string) => ({ ...item("S1", "S1-P1", n, "Step", "S1-I2"), status });

describe("progress", () => {
  const today = new Date(2026, 10, 13);

  it("counts done children and leaves canceled ones out", () => {
    const steps = [step(3, "st3"), step(4, "st1"), step(5, "st4")];
    expect(childCount(steps, settings)).toEqual({ done: 1, total: 2 });
    expect(stepProgress(steps, "S1-I2", settings)).toEqual({ done: 1, total: 2 });
    expect(stepProgress([], "S1-I2", settings)).toBeNull();
  });

  it("formats dates like 01", () => {
    expect(formatDate("2026-05-12", today)).toBe("5/12");
    expect(formatDate("2027-01-05", today)).toBe("2027/01/05");
  });

  it("labels the due date", () => {
    expect(dueLabel("", "todo", today)).toBeNull();
    expect(dueLabel("2026-11-12", "todo", today)).toEqual({ text: "期限切れ 11/12", emphasis: "overdue" });
    expect(dueLabel("2026-11-13", "doing", today)).toEqual({ text: "今日 11/13", emphasis: "today" });
    expect(dueLabel("2026-11-12", "done", today)).toEqual({ text: "11/12", emphasis: null });
  });
});
```

`screen.test.ts` の `setup` を `createProjectScreen(catalog, port, items, fakeWorkspaces(), toasts)` にし(`toasts` は `createToasts({ set: () => 0, clear: () => {} })`)、テストを足す:

```ts
  it("loads the workspace settings", async () => {
    const { screen } = await setup();
    await screen.load("S1", "S1-P1");
    expect(get(screen.view)?.settings.initialStatus).toBe("st1");
  });

  it("changes a status and reloads, showing a refusal as a toast", async () => {
    const { screen, items, toasts } = await setup();
    await screen.load("S1", "S1-P1");
    await screen.setStatus("S1-I2", "st2");
    expect(items.calls).toEqual(['setProperty S1-I2 status "st2"']);
    expect(get(screen.view)?.items.find((it) => it.id === "S1-I2")?.status).toBe("st2");
    items.refuseNext("テストの断り");
    await screen.setStatus("S1-I2", "st3");
    expect(get(toasts.list)[0]?.error).toBe(true);
  });
```

(「テストの断り」は偽の口が返すだけの値。前半には遷移の制約がないので、仕様の断りの文言は使わない。)

- [ ] **Step 2: 失敗を確かめる** — Run: `cd web && pnpm --filter @hedgehug/core test progress screen` / Expected: FAIL

- [ ] **Step 3: core を実装する**

`progress.ts`:

```ts
import type { Item } from "../../model/item";
import type { Category, WorkspaceSettings } from "../../model/workspace";

// 04「集計の定義」の「完了/対象」と、期限の表示。

export function categoryOf(settings: WorkspaceSettings, statusId: string): Category | undefined {
  return settings.statuses.find((s) => s.id === statusId)?.category;
}

// childCount は、子のうち完了の数と、中止を除いた数(詳細パネルの見出しの「3/8」)。
// 一覧にないステータスは未完了として数える(決定 11)。
export function childCount(children: Item[], settings: WorkspaceSettings) {
  const counted = children.filter((c) => categoryOf(settings, c.status) !== "canceled");
  const done = counted.filter((c) => categoryOf(settings, c.status) === "done").length;
  return { done, total: counted.length };
}

// stepProgress は、ツリーの Story・Task・Bug の行の「完了/対象」。対象が 0 件なら null。
export function stepProgress(items: Item[], parentId: string, settings: WorkspaceSettings) {
  const steps = items.filter((it) => it.parentId === parentId && it.kind === "Step");
  const c = childCount(steps, settings);
  return c.total === 0 ? null : c;
}

const pad = (n: number) => String(n).padStart(2, "0");

// formatDate は、01「日付と時刻」の日付の表示(今年なら M/D、ほかの年は YYYY/MM/DD)。
export function formatDate(iso: string, today: Date): string {
  const [y = 0, m = 0, d = 0] = iso.split("-").map(Number);
  return y === today.getFullYear() ? `${m}/${d}` : `${y}/${pad(m)}/${pad(d)}`;
}

// dueLabel は、期限の表示。未完了で期限切れ・今日なら強調する(04「集計の定義」)。
export function dueLabel(end: string, category: Category | undefined, today: Date) {
  if (end === "") return null;
  const text = formatDate(end, today);
  const open = category !== "done" && category !== "canceled";
  const t = `${today.getFullYear()}-${pad(today.getMonth() + 1)}-${pad(today.getDate())}`;
  if (open && end < t) return { text: `期限切れ ${text}`, emphasis: "overdue" as const };
  if (open && end === t) return { text: `今日 ${text}`, emphasis: "today" as const };
  return { text, emphasis: null };
}
```

`screen.ts`: 引数に `workspaces: WorkspacesPort` と `toasts: Toasts` を足し、`load` の `Promise.all` に `workspaces.get(storeId, workspace.id)` を足して `ProjectView.settings` に入れる。`setStatus` を足す:

```ts
    async setStatus(itemId, statusId) {
      if (!shown) return;
      try {
        await items.setProperty(shown.storeId, itemId, "status", statusId);
      } catch (e) {
        if (!(e instanceof Refused)) throw e;
        toasts.show(e.message, { error: true });
        return;
      }
      await reload();
    },
```

`wire.ts` の `createProjectScreen` の呼び出しに `workspaces`・`toasts` を足す。

- [ ] **Step 4: 通るのを確かめる** — Run: `cd web && pnpm --filter @hedgehug/core test` / Expected: PASS

- [ ] **Step 5: 画面をつくる**

`TreeRow.svelte` に props `settings: WorkspaceSettings`・`members: Member[]`・`items: Item[]`・`today: Date`・`onstatus: (item: Item, statusId: string) => void` を足し(下の行へもそのまま渡す)、タイトルの `<a>` の後ろに 04「行」の順で足す:

```svelte
  {#if it.kind !== "Epic" && it.kind !== "Step"}
    {@const pr = stepProgress(items, it.id, settings)}
    {#if pr}<span class="progress">{pr.done}/{pr.total}</span>{/if}
  {/if}
  {@const due = dueLabel(it.end, categoryOf(settings, it.status), today)}
  {#if due}<span class="due" class:overdue={due.emphasis === "overdue"} class:today={due.emphasis === "today"}>{due.text}</span>{/if}
  {#if it.points !== null}<span class="points">{it.points} pt</span>{/if}
  <StatusButton status={it.status} {settings} disabled={readOnly} onchange={(id) => onstatus(it, id)} />
  {#if it.assignee}
    {@const m = members.find((x) => x.login.toLowerCase() === it.assignee.toLowerCase())}
    <Avatar name={m?.name ?? ""} title={m?.name ?? "不明なユーザー"} />
  {/if}
```

(「{N} pt」は 02「作業時間」の見出しの「· {N} pt」と同じ書き方。`.overdue` は `color: var(--c-red-text)`、`.today` は `font-weight: 600`。TreeRow が 200 行を超えるなら、値の並びを `TreeRowValues.svelte` に分ける。)`ProjectTree.svelte` は `view.settings`・`view.members`・`view.items`・`new Date()`(画面を出したとき。決定 22)・`(it, id) => void projectScreen.setStatus(it.id, id)` を渡す。

`StepRow.svelte`(DetailChildren の Step の 1 行を分ける):

```svelte
<script lang="ts">
  import { categoryOf, type DetailView, type Item, itemDisplayId, readOnlyNote } from "@hedgehug/core";
  import { Avatar, IconButton } from "@hedgehug/ui";
  import { detail, links } from "../../wire";
  import StatusButton from "./StatusButton.svelte";

  // Step の 1 行(02「子と Step」の「Step の行」。「×」は A4)。チェックがオンなのは分類が完了のときだけ。
  interface Props {
    v: DetailView;
    c: Item;
    first: boolean;
  }

  let { v, c, first }: Props = $props();
  const done = $derived(categoryOf(v.settings, c.status) === "done");
  const id = $derived(itemDisplayId(v.workspace.prefix, c.number));
  const route = $derived({ name: "item" as const, store: v.store.name, item: id });
  const note = $derived(v.readOnly ? readOnlyNote : undefined);
  const m = $derived(v.people.members.find((x) => x.login.toLowerCase() === c.assignee.toLowerCase()));
</script>

<div class="child-row" role="group" aria-label={c.title}>
  <input type="checkbox" aria-label={c.title} checked={done} disabled={v.readOnly} title={note}
    onchange={() => void detail.checkStep(c.id, !done)} />
  <a class="row-title" href={links.href(route)}
    onclick={(e) => links.click(e, route, () => void detail.open(v.store.id, c.id))}
    onauxclick={(e) => links.click(e, route)}>{c.title}</a>
  <code class="id">{id}</code>
  <StatusButton status={c.status} settings={v.settings} disabled={v.readOnly}
    onchange={(s) => void detail.setProperty(c.id, "status", s)} />
  {#if c.assignee}<Avatar name={m?.name ?? ""} title={m?.name ?? "不明なユーザー"} />{/if}
  <IconButton label="上へ" size="xs" disabled={v.readOnly || first} title={note}
    onclick={() => void detail.moveUp(c.id)}>↑</IconButton>
</div>
```

(既存の DetailChildren の Step の行の「上へ」の書き方(`IconButton` の props)をそのまま移す。)

`DetailChildren.svelte`:

- 見出しの `h3` を、1 つの文字列 `` `${isEpic ? "Story・Task・Bug" : "Step"} ${count.done}/${count.total}` ``(`count = childCount(v.children, v.settings)`)にする。区画の `aria-label` は今のまま(「Story・Task・Bug」「Step」)。
- Epic の子の行の ID・タイトルの後ろに `StatusButton`(`onchange` で `detail.setProperty(c.id, "status", id)`)と担当者の `Avatar` を足す。
- Step の行は `StepRow` に置き換える。

- [ ] **Step 6: 画面を確かめる** — Run: `cd web && pnpm check && pnpm e2e project.spec.ts detail.spec.ts` / Expected: PASS
- [ ] **Step 7: コミット** — `git commit -m "feat(web): ツリーと詳細パネルの子の行に値を出し、ステータスの変更と Step のチェックを足す(A3 前半 Task 11)"`

---

### Task 12: e2e(3 幅)

**Files:**
- Create: `web/e2e/properties.spec.ts`
- Modify: `web/e2e/helpers.ts`(`newTask`・`addProperty`)

**Interfaces:**
- Consumes: Task 9〜11、B2 の `add-workspace-property`(`POST /api/stores/{store}/workspaces/{workspace}/properties`、本文 `{name, key, type, kinds}`)

3 つの幅は 1 つのサーバーを共有する(A2 の決定 21)。値を変えるテストは、それぞれ `newTask` で自分の Task を作ってから使う(Sample の HH-2 を変えると、ほかの幅の結果が変わる)。

- [ ] **Step 1: 道具を足す**

`helpers.ts`:

```ts
import { type APIRequestContext, expect, type Page, type TestInfo } from "@playwright/test";

// e2e のサーバーの Store「e2e」とサンプルのワークスペース「Web」の内部 ID(cmd/hedgehug/wire_e2e.go・apptest.Sample)。
const e2eStore = "01E2ESTORE0000000000000001";
const sampleWorkspace = "01SAMPLEWS0000000000000001";

// newTask は、Epic「認証」の下に Task を作り、その詳細パネルを開いた状態にする。
export async function newTask(page: Page, info: TestInfo, title: string) {
  const name = unique(info, title);
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/tree"));
  await page.getByRole("list", { name: "ツリー" }).getByRole("link", { name: "認証" }).click();
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await panel.getByRole("button", { name: "Task" }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(name);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(name);
  return { name, panel };
}

// addProperty は、サンプルのワークスペースに追加プロパティを作る(B2 の API)。
export async function addProperty(request: APIRequestContext, body: { name: string; key: string; type: string }) {
  const res = await request.post(`/api/stores/${e2eStore}/workspaces/${sampleWorkspace}/properties`, {
    headers: { Authorization: "Bearer e2e-token" },
    data: { ...body, kinds: [] },
  });
  if (!res.ok()) throw new Error(`addProperty: ${res.status()} ${await res.text()}`);
}
```

(`e2eStore` の値は `wire_e2e.go` の `e2eStore` の定数と同じにする。既存の `tokenUrl`・`unique` はこのファイルにある。)

- [ ] **Step 2: テストを書く**

```ts
import { expect, test } from "@playwright/test";
import { addProperty, newTask, tokenUrl, unique } from "./helpers";

test("sets properties in the detail panel", async ({ page }, info) => {
  const { panel } = await newTask(page, info, "値");
  const props = panel.getByRole("region", { name: "プロパティ" });
  await props.getByRole("button", { name: "ステータス: Todo" }).click();
  await page.getByRole("menuitemcheckbox", { name: /InProgress/ }).click();
  await expect(props.getByRole("button", { name: "ステータス: InProgress" })).toBeVisible();
  await props.getByRole("combobox", { name: "担当者" }).selectOption("taro");
  await expect(props.getByRole("combobox", { name: "担当者" })).toHaveValue("taro");
  await props.getByLabel("開始日").fill("2026-11-11");
  await props.getByLabel("開始日").blur();
  await props.getByLabel("終了日").fill("2026-11-10");
  await props.getByLabel("終了日").blur();
  await expect(page.getByText("終了日は開始日以降にしてください")).toBeVisible();
  await expect(props.getByLabel("終了日")).toHaveValue("");
  await props.getByLabel("見積もり時間").fill("0.123");
  await props.getByLabel("見積もり時間").press("Enter");
  await expect(page.getByText("見積もり時間は小数第 2 位までで入力してください")).toBeVisible();
  await props.getByRole("button", { name: /^関連システム:/ }).click();
  await page.getByRole("menuitemcheckbox", { name: "Web" }).click();
  await page.getByRole("menuitemcheckbox", { name: "API" }).click();
  await page.keyboard.press("Escape");
  await expect(props.getByRole("button", { name: "関連システム: Web, API" })).toBeVisible();
});

test("creates a task with properties", async ({ page }, info) => {
  const title = unique(info, "値つき");
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/tree"));
  await page.keyboard.press("c");
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await expect(dialog.getByRole("combobox", { name: "ステータス" })).toHaveValue(/.+/);
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(title);
  await dialog.getByLabel("見積もり時間").fill("-1");
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(dialog.getByText("見積もり時間は 0 以上の数で入力してください")).toBeVisible();
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toHaveValue(title);
  await dialog.getByLabel("見積もり時間").fill("2");
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(dialog).toBeHidden();
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByLabel("見積もり時間")).toHaveValue("2");
  await expect(panel.getByRole("button", { name: "ステータス: Todo" })).toBeVisible();
});

test("checks a Step and shows the count", async ({ page }, info) => {
  const { panel } = await newTask(page, info, "Step つき");
  const input = panel.getByRole("textbox", { name: "Step を追加" });
  await input.fill("確認");
  await input.press("Enter");
  const steps = panel.getByRole("region", { name: "Step" });
  const row = steps.getByRole("group", { name: "確認" });
  await row.getByRole("checkbox").check();
  await expect(row.getByRole("button", { name: "ステータス: Done" })).toBeVisible();
  await expect(steps.getByRole("heading")).toHaveText("Step 1/1");
  await row.getByRole("checkbox").uncheck();
  await expect(row.getByRole("button", { name: "ステータス: Todo" })).toBeVisible();
});

test("changes a status from the tree", async ({ page }, info) => {
  const { name } = await newTask(page, info, "ツリー");
  await page.keyboard.press("Escape");
  const tree = page.getByRole("list", { name: "ツリー" });
  const row = tree.getByRole("listitem").filter({ has: page.getByRole("link", { name }) }).last();
  await row.getByRole("button", { name: "ステータス: Todo" }).click();
  await page.getByRole("menuitemcheckbox", { name: /Done/ }).click();
  await expect(row.getByRole("button", { name: "ステータス: Done" })).toBeVisible();
});

test("sets an added text property", async ({ page, request }, info) => {
  const name = unique(info, "メモ");
  const key = `memo_${info.project.name}_${info.retry}`;
  await addProperty(request, { name, key, type: "text" });
  const { panel } = await newTask(page, info, "追加の値");
  const props = panel.getByRole("region", { name: "プロパティ" });
  await expect(props.getByText("追加").first()).toBeVisible();
  const field = props.getByRole("textbox", { name });
  await field.fill("  01CHOICE00000000000000000A  ");
  await field.blur();
  await page.reload();
  // 再読み込みでパネルは閉じる(01)。ツリーから開き直す。
  await page.getByRole("list", { name: "ツリー" }).getByRole("link", { name: unique(info, "追加の値") }).click();
  await expect(page.getByRole("region", { name: "プロパティ" }).getByRole("textbox", { name })).toHaveValue(
    "01CHOICE00000000000000000A",
  );
});

test("a read-only store disables properties", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/i/HH-2"));
  const props = page.getByRole("region", { name: "プロパティ" });
  await expect(props.getByRole("button", { name: /^ステータス:/ })).toBeDisabled();
  await expect(props.getByRole("combobox", { name: "担当者" })).toBeDisabled();
  await expect(props.getByRole("link", { name: "プロパティを追加・変更" })).toBeEnabled();
});
```

(ツリーの行は `<li class="tree-row">` で、`list` の中の `listitem` になる。子の行も同じ `li` なので、`filter` でタイトルのリンクを持つものを選び、入れ子の外側を避けるために `.last()` を取る。見出しは Task 11 のとおり `h3` に「Step 1/1」の 1 つの文字列で出す。)

- [ ] **Step 3: 通るのを確かめる** — Run: `cd web && pnpm e2e` / Expected: 全部の spec が 3 幅で PASS
- [ ] **Step 4: `make check` とコミット** — `git commit -m "test(e2e): A3 前半の完成の条件を 3 つの幅で通しで確かめる(A3 前半 Task 12)"`

---

## 最後に

- [ ] 最終レビュー(`code-review` を high で、`feat/track-a3a` の develop との差分全体)。指摘を直す。
- [ ] `make check` と e2e(3 幅)を通す。
- [ ] トラックの PR を 1 本出す(向き先 develop、タイトル「feat: トラック A3 前半(プロパティとステータス)」)。本文に、できること・決めたこと(この計画の表の番号)・後半(A3b)と A4・B5・E・F に回したもの・確認の結果を書く。force push は拒否されるので、載せ直すときは別名のブランチで出す。

## 完成の条件

- 詳細パネルと作成ダイアログで、組み込みの 8 つと追加プロパティの値を入れて保存できる(e2e)。
- 断りの文言(「終了日は開始日以降にしてください」「ポイントは 0 以上の数で入力してください」「見積もり時間は 0 以上の数で入力してください」「見積もり時間は小数第 2 位までで入力してください」)とアクティビティの文が仕様どおり(Go のテスト・e2e)。
- `make check` と e2e(3 幅)が通る。
