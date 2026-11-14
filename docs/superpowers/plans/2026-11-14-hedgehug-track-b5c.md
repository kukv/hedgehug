# トラック B5c(ワークスペースの別の Store への移動) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ワークスペースの設定「一般」の「別の Store へ移動」で、ワークスペースをまるごと(設定・プロジェクト・アイテム・スプリント・リリース・削除済みのものを含む)別の Store へ移し、転送記録を残して、移動元の古いアドレスからいまの場所を開けるようにする。

**Architecture:** A5a・A5b の「別の Store」の部品をそのまま使う。移すものの集まり・外との関連・人の綴りの置き換えは `internal/domain/transfer` に純粋な関数で足し、操作は `internal/app/transfers` に `WorkspaceTargets`・`WorkspacePreview`・`MoveWorkspace`・`LocateWorkspace` を足す。移動先は `Delivery.Deliver`(受け取り → 保存 → 送信を 1 つのロックで)、そのあと移動元を `Stores.Write`、移動元が失敗したら移動先を戻す。ワークスペースの中身は移動先でも同じ設定を使うので、値の引き直しとステータスの置き換えはない。保存の口は `WorkspaceWriter.Import`・`Remove` を足す。画面は「一般」に区画と確認ダイアログを足し、ワークスペースのアドレスが Store 名の Store で見つからないときはサーバーの `locateWorkspace` に聞く。

**Tech Stack:** Go(huma・domain/app/infra/adapter の層、go-git v5)、TypeScript(core・api の orval)、Svelte 5(ui・app)、Vitest、Playwright。

**Spec:** `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md`(「範囲と画面の構成」の操作の表の「別の Store へ移動」、「別の Store へ移動」の節、「プロジェクト別のルール」の「移動・削除・復元」、「変更が及ぶ範囲」の「別の Store へ移動」の行)、`02-items-projects.md`(「関連・転送記録・旧 ID」「保存」。ワークスペースの移動は 03 がここを参照する)、`01-overview.md`(「画面のアドレス」)、`06-sync-notifications.md`(「他の書き手との重なり」の削除の取り消し、「別の Store へ移動したものと重なったとき」)、`08-stores-maintenance.md`(「削除済み」の画面: 移したものは移動元に残らない)。保存形式は `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`(「ファイル構成」、「変更記録」の `moved_in`、「移動・番号・旧 ID」の「別の Store」、「アクティビティと履歴」)。

**実装担当にタスクを渡すときは、タスクの本文と一緒に「Global Constraints」「持ち場と境目」「決めたこと」「タスク間の名前」の節を必ず渡す。** 食い違ったときは、この 4 節と仕様が本文より優先する。

**前提として読むもの:** A5a の計画 `2026-11-14-hedgehug-track-a5a.md` と A5b の計画 `2026-11-14-hedgehug-track-a5b.md` の「決めたこと」「タスク間の名前」「実装で計画から変えたこと」、B5b の計画 `2026-11-14-hedgehug-track-b5b.md` の B5b-4(削除済みのワークスペースを名前・接頭辞の重複の検査から除く)。B5c は A5a・A5b の型と口をそのまま使い、同じ規則を繰り返して書かない。手本のコードは `internal/app/transfers/project_move_store.go`・`project_undo.go`・`project_locate.go`、`web/packages/core/src/features/transfers/projectDialog.ts`、`web/packages/core/src/features/workspaces/delete.ts`(名前の入力)。

## Global Constraints

- 依存は追加しない(足すときは `go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` で版を明記し、公開から 7 日以上の版だけ。Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 大きさの上限: Go ファイル 300 行(テスト 600 行)・関数 60 行・認知的複雑度 15・パッケージ 20 ファイル(テストを除いて数える)、Svelte 200 行、TS 300 行。超えたら分ける。いまの数: `internal/app/transfers` 17(Task 3 で `project_targets.go` を `targets.go` に、`project_locate.go` を `locate_kinds.go` にまとめて 15 にしてから 4 つ足し 19)、`internal/domain/transfer` 9、`internal/app/apptest` 20(新しいファイルを足さない)、`internal/infra/gitstore` 19(足さない)、`internal/adapter/httpapi/transfers` 4。
- 向き: adapter→app→domain、infra→app/port。app の機能どうしは import しない(テストも)。`app/transfers` は `app/workspaces`・`app/projects`・`app/items` を import しない。
- 画面の文言は仕様の文字どおり。仕様にない文言は B5c-14 の文を使い、Task 12 で仕様に書き足す。モックの動きは写さない。
- 通知は作らない。自動化は動かさない。保存したビューは変えない(03「変更が及ぶ範囲」)。
- 閲覧のみの Store のワークスペースでは、「別の Store へ移動」の区画のセレクトと「移動…」を無効にし「閲覧のみの Store です」を添える。閲覧のみの Store は移動先の候補に出さない。
- 各タスクは TDD(失敗するテスト → 失敗を確かめる → 実装 → 通す → コミット)。タスクごとにレビューする。
- コミットの末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`。
- PR はトラックで 1 本(develop 向け、ブランチ `feat/track-b5c`)。force push は拒否されるので、載せ直すときは別名のブランチで出す。push は HTTPS(`https://github.com/kukv/hedgehug.git`)で通る。
- 計画やテストにトークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e のビルドも CI で lint する。
- 確認のコマンド: Go は `go test ./internal/... ./cmd/...`、全体は `make check`、画面は `pnpm -C web test`・`pnpm -C web check`、API の型は `make generate` のあと `pnpm -C web gen`、e2e は `pnpm -C web e2e`。手元の検証は 1 回で足りる。
- `api/openapi.yaml`・`web/packages/api/src/generated.ts` は手でマージしない。API を変えたタスクで `make generate` → `pnpm -C web gen` で作り直してコミットする。
- 本文のテストの名前と期待は下書きで、実行していない。仕様と「決めたこと」が正。

## Review Focus

1. **`SyncSave` の試み直しで古い値を使う**: 移動先の保存の `fn` は、送信が重なると最大 3 回呼び直される(A5a-5)。移動先の同じ接頭辞・同じ名前の断り、ワークスペースの順位、足すメンバーと綴りの対応は、毎回 `fn` の `Tx` から読んで決める。テストは Task 5(偽の `Delivery` が `fn` を 2 回呼び、2 回目の前に移動先へ同じ接頭辞のワークスペースを足すと断る)。
2. **移動元で、あいだに足された・変えられたもの**: `prepare` で読んだあと、移動元の保存までのあいだに、同じマシンの別のタブがワークスペースの中にアイテムを作る・設定を変えると、移動先に写っていないものが移動元から消える。移動元の保存の直前に読み直し、違えば失敗して移動先を戻す(B5c-9)。テストは Task 6。
3. **削除済みのものも一緒に移る**: 削除済みのプロジェクト・アイテム・スプリント・リリース(ワークスペースのログの中)が、削除済みのまま元の削除日時で移り、移動元の「削除済み」に残らない。削除済みのプロジェクトの下のアイテムも移す。テストは Task 1(集まり)と Task 5(移動先で削除済みとして読める)。
4. **古いアドレスで、ほかの Store に同じ接頭辞がある**: 移動元 Store「work」の `WEB` を Store「team」へ移したあと、Store「other」にも `WEB` があるとき、`/s/work/w/WEB/overview` と `/s/work/i/WEB-12` は Store「team」のものを開く(Store 名の Store の転送記録を、ほかの Store のいまの接頭辞より先に見る。01「画面のアドレス」)。テストは Task 7。
5. **移したあとの画面**: 自分で移したときは、移動先の Store のワークスペースの「設定」タブへ移り、サイドバーの移動元からワークスペースが消え、移動先に出る。古いアドレスの 1 行は、タブを変えると消える。テストは Task 9・11 と Task 12 の e2e。

---

## 持ち場と境目

| 入れるもの | 仕様 |
|---|---|
| 「一般」の「別の Store へ移動」の区画(説明文・移動先のセレクト・「移動…」・候補がないときの 2 つの文・閲覧のみの無効) | 03「別の Store へ移動」の 1 段落目 |
| 移動できない条件(同じ接頭辞・同じ名前)のトースト | 03 の同じ節の 2 段落目 |
| 確認ダイアログ「別の Store へ移動」(本文の 5 つの点、名前の入力と「名前が一致しません」) | 03 の同じ節の 3・4 段落目 |
| 別の Store への移動(移動先の受け取り・保存・送信 → 移動元の保存、失敗で元のまま、`moved_in`、外との関連の解除、メンバーの追加と綴りの置き換え、戻し、転送記録) | 03 の同じ節の「移動すると」の点、02「保存」 |
| 別の画面で削除されたワークスペースを移すときの削除の取り消し | 06「他の書き手との重なり」(A5b-15 と同じ扱い) |
| 古いアドレスの解決(ワークスペース・その中のプロジェクトとアイテム)、見つからないときのトースト、ワークスペースの見出しの下の 1 行 | 03 の同じ節の転送の記録の点、01「画面のアドレス」 |
| アクティビティ・トースト | 03 の操作の表、B5c-11 |
| プロジェクトの履歴の、ワークスペースと一緒に移ったときの行 | B5c-12 |

| 入れないもの | 回す先 | 理由 |
|---|---|---|
| アクティビティの画面の「移動元にないワークスペースの内部 ID を指す行は、絞り込みの選択肢に出さない」(保存形式「アクティビティと履歴」)、「メンバー {ログイン名} を追加」の 1 人 1 行の表示 | C4 | アクティビティの画面は C4。B5c は保存の行を書くだけ |
| 移動元で他の人が同時に変えたものとの重なりの通知 | C4 | 同期の側のフォルダの扱いは C1(`underTransferred`)が祖先にワークスペースを含めて済んでいる。通知は C4 |
| コマンドパレット・検索の旧 ID | A5c・E | ワークスペースの移動では ID が変わらないので、いまの ID で見つかる。旧い接頭辞と転送記録の規則は `LocateWorkspace` と同じ |
| 「完全に削除」の書き込み、移動先での復元の名前の重なり | G | B5c は `purged.jsonl` を読むだけ |

## 決めたこと

| # | 決定 | 根拠 |
|---|---|---|
| B5c-1 | 1 本の計画・1 本の PR(サーバーと画面を分けない) | A5b-1 と同じ。トラック B の計画の B5c の行 |
| B5c-2 | 移動元の古いアドレス(`/s/{移動元}/w/{接頭辞}/{タブ}`)から開いたときは、開いたタブのまま移動先のワークスペースを開き、ワークスペースの見出しの下(タブの上)に「このワークスペースは {日時} に {Store 名} へ移動しました」を出す(そのあとさらに別の Store へ移したときは「このワークスペースは {日時} に {B の Store 名} へ移動し、その後 {いまの Store 名} へ移動しました」)。タブを変える・別の画面を開くと消える。普通に開いたときは出さない。リンクは出さない(もう移動先を開いているため)。仕様 03 の「表示して、移動先へのリンクを出す」を直す(Task 12) | ユーザーの決定(2026-11-14)。01「画面のアドレス」の「移動したものは、いまある場所を開く」と A5b-2 に揃える |
| B5c-3 | 移すものは `transfer.CollectWorkspace(ws workspace.ID, projects []project.Project, items []item.Item) WorkspaceSet`: ワークスペースのすべてのプロジェクトとアイテム(削除済みを含む。削除済みのプロジェクトの下のアイテムも)。「外」は、`items` のうち集まりにいないアイテム(移動元の Store のほかのワークスペースのもの)との関連(両方の側。A5a の `Cut`)。スプリント・リリース・プロジェクト別のルール・専用ルール・旧い接頭辞・`number` の行はワークスペースのログにあるので、ワークスペースのフォルダを写せば移る | 03「移動すると」の 1・3 つ目の点、保存形式「ファイル構成」 |
| B5c-4 | 件数: P = 削除されていないプロジェクト、I = 削除されていないプロジェクトの削除されていないアイテム、M = 削除済みのプロジェクトと、それ以外のアイテム(自分が削除済みか、削除済みのプロジェクトの下)。スプリント・リリースは数えない(本文が「プロジェクト {P} 件・アイテム {I} 件」の続きのため) | 03 の確認ダイアログの 2 つ目の点 |
| B5c-5 | 断り(移動先の Store の削除されていないワークスペースと比べる。B5b-4 と同じく削除済みは除く): 同じ接頭辞 → 「{移動先の Store 名}: この Store には同じ接頭辞のワークスペースがあります」、同じ名前 → 「{移動先の Store 名}: この Store には同じ名前のワークスペースがあります」(`workspace.PrefixTaken`・`NameTaken`。接頭辞を先に調べる。仕様の並び)。サーバーが Store 名を付けた文で `refusal.New` を返す。予告(区画の「移動…」)と送る前の `prepare` と移動先の保存の `fn` の中(`Tx` から読み直して)の 3 か所で調べる | 03 の 2 段落目、Review Focus 1 |
| B5c-6 | 移動先に書く行(1 回の `Delivery.Deliver`): ワークスペース・プロジェクト・アイテムのフォルダを `Import` で写し(行はそのまま)、それぞれに `moved_in`(`FromWorkspace` はそのワークスペース自身、`FromProject` はプロジェクトならそれ自身・アイテムならそのプロジェクト、`Old` はいまの ID(ワークスペースは接頭辞)、`Names` と `Moves` は A5a・A5b と同じ規則で引く。アイテムが前に同じ Store の別のワークスペースから来ていれば、その値の名前が要るため)、ワークスペースに `set rank`(移動先の Store のワークスペース(削除済みを含む。B5b-4)の最後の順位の後ろ)、外との関連の `remove`、人の綴りが変わるものの `set`(B5c-7)、足す人の `join`(A5a-11 と同じく同じ保存)。別の画面で削除されていたら、写した行のその削除の操作を `change.Undo` で取り消す(B5c-10) | 保存形式「移動・番号・旧 ID」の「別の Store」、03「移動すると」 |
| B5c-7 | 移動先に足す人と綴りは `transfer.People`(A5a)で決める。`items` はすべてのアイテム、`logs` はワークスペース・プロジェクト・アイテムのすべての行、`extra` は `transfer.SettingsPeople(settings, rules)`(テンプレートの本体・子の担当者とユーザー型の値、Step の担当者(`workspace.StepParent`・`StepNone` は除く)、ワークスペース全体と専用のルールの担当者の値(`automation.Unassigned` は除く))とプロジェクトのオーナー。綴りが移動先と違う人(大文字と小文字だけが違う)は、その値を `set` で移動先の綴りに書き直す: アイテムは `transfer.RemapPeople(it, settings, people)`(担当者・ユーザー型の値・ウォッチ。値の引き直しはしない)、設定は `transfer.RemapSettingsPeople(settings, rules, people)`(`workspace.TemplateDrafts(old, new)`・`automation.Drafts(old, new)` で差分の行)、プロジェクトは `set owner`。作成者と行の `by` は書き直さない(A5a と同じ) | 03「移動すると」の 2 つ目の点、02「値の引き直し」の担当者の行 |
| B5c-8 | 移動元に書く行(1 回の `Stores.Write`): ワークスペース・プロジェクト・アイテムのフォルダを `Remove`、外のアイテムに関連の `remove`、転送記録をワークスペースについて 1 行(`entity: workspace`、`old`・`new` は接頭辞、`to_store`・`to_store_name`・`to_remote`、`at` は移動先の `moved_in` と同じ)。プロジェクト・アイテムごとの転送記録は書かない(ID が変わらず、古いアドレスは B5c-13 でワークスペースの記録から辿るため。03「移すたびに 1 件ずつ」) | 03「移動すると」の 4 つ目の点 |
| B5c-9 | 移動元の保存の直前に、`Tx` から集まりを読み直し、`prepare` で読んだものと違えば `errSourceChanged` で失敗して移動先を戻す。比べるのは、ワークスペース・プロジェクト・アイテムの参照の集まりと、それぞれのログの行数と最後の行の `id`(`sourceStamp`)。外のアイテムとの関連は、外のアイテムのログが変わっても比べない(関連の `remove` は読み直した値から書く) | A5a-6・A5b-12、Review Focus 2 |
| B5c-10 | 別の画面で利用者が削除したワークスペース(`port.EditableWorkspace` を通るもの)を移すときは、削除を取り消してから移す。移動先の同じ保存で、写した行のその削除の操作を `change.Undo` で取り消し(ワークスペース・プロジェクト・アイテムのログすべて)、両方の保存が済んでから `port.NoteRevived(Revival{Kind: RevivedName, Text: 名前})`。名前と接頭辞の重なりは B5c-5 の移動先の検査で済む | 06「他の書き手との重なり」、A5b-15、`port.ReviveWorkspace` と同じ取り消しの範囲 |
| B5c-11 | アクティビティ: 移動元「ワークスペース「{名前}」を {移動先の Store 名} へ移動」(03 の表)、移動先「ワークスペース「{名前}」を {移動元の Store 名} から移動」(03「移動先にワークスペースの追加として残る」の文)。戻しは「ワークスペース「{名前}」の移動を取り消し」。トースト「{移動先の Store 名} へ移動しました」 | 03 の操作の表と「保存」の点、02「保存」の戻し |
| B5c-12 | ユーザーの決定(2026-11-14): 履歴に 1 行出す。プロジェクトの履歴は、`moved_in` の `FromWorkspace` がいまのワークスペースと同じ(ワークスペースと一緒に移った)とき、「{移動元の Store 名} から移動」と出す(A5b-21 の「{移動元のワークスペース名} から移動」では同じ名前が出て、何が起きたか分からないため)。アイテムの履歴は A5a のまま(「{旧 ID} から移動」と移動元の Store・ワークスペース・プロジェクト) | 02「履歴」、A5b-21 |
| B5c-13 | 古いアドレスの解決。ワークスペース: `transfers.Service.LocateWorkspace(ctx, storeName, prefix)`。Store 名の Store を先に (a) いまの接頭辞、(b) 旧い接頭辞、(c) その Store の転送記録の `entity: workspace` の行を `at` の新しい順に見て、`target` を表示中のすべての Store から探し、記録の `old` か、見つけたワークスペースのいまの接頭辞・旧い接頭辞のどれかが画面の接頭辞と一致すれば当たり(接頭辞を HH から WEB に変えてから移すと記録の `old` は WEB なので、`/s/work/w/HH` は見つけたワークスペースの旧い接頭辞で当てる。仕様 03「ワークスペースは旧い接頭辞で転送の記録をたどり」。A5a の `aliases` と同じ考え)。見つからなければ、ほかの Store で (a)(b)(c) を順に(A5b の `find` と同じ順)。削除済みのワークスペースは開かない。(c) で見つかったら `MovedFrom{Old: 接頭辞, At, Display: 記録の移動先の Store 名, Current: いまの Store 名}`(Store 名は `storeName` と同じくこのマシンの名前を優先)。見つからないときの `Gone` は A5a-15 の 4 つで、`NewDisplay` は記録の移動先の Store 名。アイテム・プロジェクト: `find` の (c) で、種類の記録に加えて、`entity: workspace` の記録を上と同じ一致の規則(記録の `old` か、`target` のワークスペースのいまの接頭辞・旧い接頭辞)で見て、`target` のワークスペースの中の同じ番号の削除されていないもの。この経路で見つかったときは `MovedFrom` を付けない(ID が変わっていないため)。開けないときの `Gone` は `Old` を画面の ID、`NewDisplay` を記録の移動先の Store 名にする(トーストは「{旧 ID} は {日時} に {Store 名} へ移動しましたが、削除されています…」と読める) | B5c-2、01「画面のアドレス」、02「見つからないとき」、Review Focus 4 |
| B5c-14 | 仕様にない文言: 区画のセレクトの名前「移動先の Store」、ボタン「移動…」、確認ダイアログのボタン「移動」「キャンセル」、名前の入力欄「確認のため、ワークスペースの名前を入力してください」(削除と同じ)、外との関連の点「ワークスペースの外にあるアイテムとの関連は、種類を問わず解除されます。ワークスペースの中のアイテム同士の関連は残ります。」、人の点「{表示名}さん・…の {N} 人が、移動先のメンバーに追加されます」(0 人なら出さない)、移動先のアクティビティ(B5c-11)、戻しのアクティビティ(B5c-11)、ワークスペースの見出しの下の 1 行の「その後」の形(B5c-2)、ワークスペースの古いアドレスのトースト(下)、失敗のトースト(A5a-12 と同じ: 「{移動先の Store 名} に送信できないため移動できません: {理由}」「オフラインのため {移動先の Store 名} へ移動できません」「移動できませんでした」、取り消しもできなかったとき「ワークスペース「{名前}」を {移動先の Store 名} へ移しましたが、移動元から取り除けず、取り消しもできませんでした。どちらかを削除してください」)。ワークスペースの古いアドレスで開けないときのトーストは、02「見つからないとき」の文の {旧 ID} を「このワークスペース」に、{新 ID} を {Store 名} にしたもの(例: 「このワークスペースは {日時} に team へ移動しましたが、削除されています。設定の「削除済み」から復元できます」)。Task 12 で仕様 03 に書き足し、PR の本文に一覧で出す | A5b-18 と同じ扱い |
| B5c-15 | 候補は `WorkspaceTargets`: 表示中の Store(`StoreList`)のうち閲覧のみでなく自分でないもの(Store の並び)。`HiddenOnly` は、候補が 0 件で、`SyncTargets` に非表示・閲覧のみでない・自分でない Store があるとき true。画面は、候補があればセレクト(初めの値は先頭)、0 件なら `HiddenOnly` で「移せる Store がありません(非表示の Store は、設定の「Store」で表示に戻してください)」か「移せる Store がありません」を出し「移動…」を無効にする | 03 の 1 段落目 |
| B5c-16 | 区画の流れ: 「移動…」を押すと予告を取り、断り(B5c-5)ならエラーのトーストを出してダイアログを開かない。断りでなければ確認ダイアログを開く。名前の照合は画面だけで行う(削除と同じ。`nameMismatch`)。「移動」は、入力が名前と一致しないあいだ「名前が一致しません」を出して送らない | 03 の 3・4 段落目、B5b の削除と同じ |
| B5c-17 | 移したあと、「移動」を押したときからアドレスが変わっていなければ、画面は移動先のワークスペースの「設定」タブのアドレス(`{ name: "workspace", store: 移動先の Store 名, prefix, tab: "settings" }`)へ移り(履歴に積まない `replace`)、そのあとでサイドバーの一覧を読み直す。押したあとに画面を移っていたら、一覧を読み直すだけにする。ほかの書き手(同期・別のタブ)が移したときは、いまの扱い(出していたワークスペースがなくなればマイタスク。01)のまま。`createLeaveMoved` と同じ形で `createLeaveWorkspaceMoved` を作る | A5b-19 |
| B5c-18 | 失敗の扱いは A5a-12・A5b-20 と同じ: 送れない・オフライン・断り・それ以外はトーストを出してダイアログを開いたまま。`NotFound` と取り消しもできなかったときは、ダイアログを閉じて画面を読み直す。オフラインは、移動先に remote があり `navigator.onLine === false` のとき画面が決める | A5a-12 |
| B5c-19 | API(`/api/…`): `GET stores/{store}/workspaces/{workspace}/transfer-targets` → `{targets: [{storeId, storeName}], hiddenOnly}`、`GET stores/{store}/workspaces/{workspace}/transfer-preview?toStore=` → B5c-20(断りは 422 の refusal。A5b の断りと同じ形)、`POST stores/{store}/workspaces/{workspace}/transfer`(本文 `{toStore}` → 200 `{toStore, toStoreName, workspaceId, prefix}`)。送れなかったときは 422、取り消しもできなかったときは 409(A5a-19 と同じ)。`GET locate-workspace?store=&prefix=` → `{kind: "found", storeId, workspaceId, movedFrom?}` か `{kind: "gone", …}` か 404 | A5b-22 に合わせる |
| B5c-21 | このマシンの `state.db` の GitHub の行(`github_checks`・`github_link_failures`・`github_scans`・`github_renames`。どれも Store ID が鍵)は移動先へ付け替えない。どれも GitHub から読んだものの覚えで、移動先の Store の次の読み取りで作り直される。A5a・A5b と同じ扱い | 03「変更が及ぶ範囲」(このマシンのものは影響なし) |
| B5c-20 | 予告 `WorkspacePreview`: `source`(移動元の Store 名)・`destination`(移動先の Store 名)・`projects`・`items`・`deleted`(B5c-4)・`people`(`[{login, name}]`。足す人、小文字のログイン名の順)・`remoteNotice`(移動先に remote がなく移動元にある)・`destinationHasRemote`。空の配列は `[]` | 03 の確認ダイアログの点 |

---

## ファイルの地図

| 場所 | 新規・変更 | 役割 |
|---|---|---|
| `internal/domain/transfer/workspace.go` | 新規 | `WorkspaceSet`・`CollectWorkspace`・`Counts` |
| `internal/domain/transfer/people_remap.go` | 新規 | `SettingsPeople`・`RemapPeople`・`RemapSettingsPeople` |
| `internal/domain/transfer/remap_fields.go` | 変更 | 人の置き換え(`assignee`・`person`・`watchers`)を `RemapPeople` と共有 |
| `internal/app/port/stores.go` | 変更 | `WorkspaceWriter.Import`・`Remove` |
| `internal/infra/gitstore/write.go`・`sync_tree_test.go`・`internal/app/apptest/stores.go`・`sync.go`・`stores_contract_revive.go` | 変更 | 上の口の実装と契約テスト(`idWriter` を共有。新しいファイルを足さない。契約テストのほかのファイルは 300 行に近い)、同期のワークスペースの記録のテスト |
| `internal/app/transfers/targets.go`・`locate_kinds.go` | 変更・新規 | `project_targets.go` をまとめる、`project_locate.go` を `LocateWorkspace` と一緒に置く(中身は変えない) |
| `internal/app/transfers/workspace_preview.go`・`workspace_move.go`・`workspace_write.go`・`workspace_undo.go` | 新規 | 候補と予告・移動の流れ・両方の保存の行・戻しと読み直し |
| `internal/app/transfers/locate.go`・`locate_read.go`・`locate_gone.go` | 変更 | ワークスペースの記録をアイテム・プロジェクトの (c) に足す、スナップショットに削除済みのワークスペース |
| `internal/domain/history/project.go` | 変更 | B5c-12 |
| `internal/adapter/httpapi/transfers/workspace.go` | 新規 | 4 つの API |
| `web/packages/core/src/model/transfer.ts`・`port/transfers.ts`・`web/packages/api/src/transfers.ts` | 変更 | 型・口・api の実装 |
| `web/packages/core/src/features/transfers/workspaceDialog.ts`・`workspaceDialogText.ts`・`leaveMoved.ts`・`movedText.ts` | 新規・変更 | 区画とダイアログのモデル・文、移したあとの移り先、「このワークスペース」の文 |
| `web/packages/core/src/features/router/resolve.ts`・`features/screen/screen.ts` | 変更 | ワークスペースのアドレスをサーバーに聞く、`workspaceMoved` |
| `web/app/src/screens/workspace-settings/WorkspaceTransfer.svelte`・`WorkspaceTransferDialog.svelte`・`GeneralPanel.svelte`、`web/app/src/screens/workspace/WorkspaceScreen.svelte`・`web/app/src/wireActions.ts` | 新規・変更 | 区画・ダイアログ・見出しの下の 1 行 |
| `web/e2e/workspace-transfer.spec.ts` | 新規 | 通しのテスト |

## タスクの一覧

| # | タスク | 待つもの |
|---|---|---|
| 1 | `domain/transfer`: `CollectWorkspace`・`Counts` | — |
| 2 | `domain/transfer`: `SettingsPeople`・`RemapPeople`・`RemapSettingsPeople` | — |
| 3 | 口と保存: `WorkspaceWriter.Import`・`Remove`、ワークスペースの `moved_in`、同期のワークスペースの記録、ファイルのまとめ | — |
| 4 | `app/transfers`: `WorkspaceTargets`・`WorkspacePreview`(断りを含む) | 1〜3 |
| 5 | `app/transfers`: `MoveWorkspace`(両方の保存・試み直し・削除の取り消し) | 4 |
| 6 | `app/transfers`: 戻しと移動元の読み直し | 5 |
| 7 | `LocateWorkspace`・アイテムとプロジェクトのワークスペースの記録からの解決・プロジェクトの履歴 | 5 |
| 8 | API と配線 | 4〜7 |
| 9 | 画面の口と区画・ダイアログのモデル(core) | 8 |
| 10 | 区画・ダイアログの部品と、移したあとの移り先(app) | 9 |
| 11 | ワークスペースの古いアドレスと見出しの下の 1 行 | 9 |
| 12 | 通しのテスト(e2e)・仕様の文言・PR | すべて |

## タスク間の名前

### Go

| 名前 | 置き場所 | 形 |
|---|---|---|
| `transfer.WorkspaceSet`・`CollectWorkspace` | `internal/domain/transfer/workspace.go` | `type WorkspaceSet struct { Workspace workspace.ID; Projects []project.Project; Items []item.Item; Outside []Cut }`(`Projects`・`Items` は引数の順のまま、そのワークスペースのもの)、`func CollectWorkspace(ws workspace.ID, projects []project.Project, items []item.Item) WorkspaceSet` |
| `transfer.Counts`・`(WorkspaceSet) Counts` | 同上 | `type Counts struct { Projects, Items, Deleted int }`(B5c-4) |
| `transfer.SettingsPeople` | `internal/domain/transfer/people_remap.go` | `func SettingsPeople(s workspace.Settings, rules []automation.Rule) []string` |
| `transfer.RemapPeople` | 同上 | `func RemapPeople(it item.Item, s workspace.Settings, people PeopleMap) []change.Draft`(綴りが変わる値だけ。担当者・ユーザー型の値・ウォッチ) |
| `transfer.RemapSettingsPeople` | 同上 | `func RemapSettingsPeople(s workspace.Settings, rules []automation.Rule, people PeopleMap) []change.Draft` |
| `port.WorkspaceWriter.Import`・`Remove` | `internal/app/port/stores.go` | `Import(id workspace.ID, log []change.Record) error`(説明はない)、`Remove(id workspace.ID) error`(下のプロジェクト・アイテムのフォルダは消さない。`ProjectWriter` と同じ約束) |
| `transfers.WorkspaceTargets`・`(Service) WorkspaceTargets` | `internal/app/transfers/workspace_preview.go` | `type WorkspaceTargets struct { Stores []port.StoreInfo; HiddenOnly bool }`、`WorkspaceTargets(ctx, store port.StoreID, id workspace.ID) (WorkspaceTargets, error)` |
| `transfers.WorkspacePreview`・`(Service) WorkspacePreview` | 同上 | `type WorkspacePreview struct { Source, Destination string; Counts transfer.Counts; People []member.Member; RemoteNotice, DestinationHasRemote bool }`、`WorkspacePreview(ctx, store port.StoreID, id workspace.ID, to port.StoreID) (WorkspacePreview, error)`(断りは refusal) |
| `transfers.WorkspaceMoved`・`(Service) MoveWorkspace` | `internal/app/transfers/workspace_move.go` | `type WorkspaceMoved struct { ToStore port.StoreID; ToStoreName string; Workspace workspace.ID; Prefix string }`、`MoveWorkspace(ctx, store port.StoreID, id workspace.ID, to port.StoreID) (WorkspaceMoved, error)`。失敗は A5a の `*SendError`・`*UndoError`(`UndoError.Old` はワークスペース名、`NewDisplay` は空) |
| `crossWorkspaceMove`・`writeDestination`・`writeSource` | `internal/app/transfers/workspace_move.go`・`workspace_write.go` | 内部。Task 5 |
| `rollbackWorkspace`・`sourceStamp` | `internal/app/transfers/workspace_undo.go` | 内部。B5c-9 |
| `transfers.LocatedWorkspace`・`(Service) LocateWorkspace` | `internal/app/transfers/locate_kinds.go` | `type LocatedWorkspace struct { Found bool; Store port.StoreID; Workspace workspace.ID; MovedFrom *MovedFrom; Gone *Gone }`、`LocateWorkspace(ctx, storeName, prefix string) (LocatedWorkspace, error)` |

### HTTP API

| operationId | メソッドとパス | 本文 → 応答 |
|---|---|---|
| `list-workspace-transfer-targets` | GET `/api/stores/{store}/workspaces/{workspace}/transfer-targets` | → `{targets: {storeId, storeName}[], hiddenOnly}` |
| `get-workspace-transfer-preview` | GET `/api/stores/{store}/workspaces/{workspace}/transfer-preview` | query `toStore` → `{source, destination, projects, items, deleted, people: {login, name}[], remoteNotice, destinationHasRemote}` |
| `transfer-workspace` | POST `/api/stores/{store}/workspaces/{workspace}/transfer` | `{toStore}` → `{toStore, toStoreName, workspaceId, prefix}` |
| `locate-workspace` | GET `/api/locate-workspace` | query `store, prefix` → `{kind: "found", storeId, workspaceId, movedFrom?}` か `{kind: "gone", reason, old, at, newDisplay, storeName}` |

### 画面

| 名前 | 置き場所 | 形 |
|---|---|---|
| `WorkspaceTransferTargets`・`WorkspaceTransferPreview`・`WorkspaceTransferResult`・`LocatedWorkspace` | `web/packages/core/src/model/transfer.ts` | API の形のまま |
| `TransfersPort` に足すもの | `web/packages/core/src/port/transfers.ts` | `workspaceTargets(storeId, workspaceId)`・`workspacePreview(storeId, workspaceId, toStoreId)`・`transferWorkspace(storeId, workspaceId, toStoreId)`・`locateWorkspace(storeName, prefix)` |
| `createWorkspaceTransfer` | `web/packages/core/src/features/transfers/workspaceDialog.ts` | 区画(候補・選んだ移動先)と確認ダイアログ(予告・名前の照合・送信)。Task 9 |
| `workspaceTransferLines`・`workspaceMovedMessage` | `web/packages/core/src/features/transfers/workspaceDialogText.ts` | Task 9 |
| `createLeaveWorkspaceMoved` | `web/packages/core/src/features/transfers/leaveMoved.ts` | B5c-17 |
| `workspaceMovedLine`・`workspaceGoneText` | `web/packages/core/src/features/transfers/movedText.ts` | `movedLine`・`goneText` の旧 ID を「このワークスペース」にしたもの(B5c-2・B5c-14) |
| `workspaceTransfer` | `web/app/src/wireActions.ts` | `createWorkspaceTransfer` の組み立て |
| `Screen.workspaceMoved` | `web/packages/core/src/features/screen/screen.ts` | `Readable<{ storeId: string; workspaceId: string; tab: WorkspaceTab; text: string } \| null>`(A5b-24 の `projectMoved` と同じ持ち方) |

---

### Task 1: `domain/transfer`: `CollectWorkspace`・`Counts`

**Files:**
- Create: `internal/domain/transfer/workspace.go`
- Test: `internal/domain/transfer/workspace_test.go`

**Interfaces:**
- Consumes: A5a の `transfer.Cut`(外との関連)と、それを作る関数(`collect.go`)
- Produces: `WorkspaceSet`・`CollectWorkspace`・`Counts`(タスク間の名前)

- [ ] **Step 1: 失敗するテストを書く**

- `TestCollectWorkspaceTakesAllIncludingDeleted`: ワークスペース W にプロジェクト P1(削除されていない)・P2(削除済み)、P1 の下に Epic E と Task T(削除済み)、P2 の下に Task U。ほかのワークスペース X のプロジェクトとアイテム V。`Projects` は P1・P2、`Items` は E・T・U、V は入らない。
- `TestCollectWorkspaceCutsOutside`: E と V に関連(E 側の `relations` と V 側の両方)、E と U に関連。`Outside` は E–V の両方の側の 2 つで、E–U は入らない(A5a の `Cut` の形)。
- `TestWorkspaceCounts`: 上の集まりで `Counts{Projects: 1, Items: 1, Deleted: 3}`(P2・T・U)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/transfer/ -run 'CollectWorkspace|WorkspaceCounts'`
Expected: FAIL(`CollectWorkspace` がない)

- [ ] **Step 3: 実装する**

`Outside` は A5a の `Collect` の外の関連の求め方を、集まりの判定だけ「ワークスペースが ws か」に替えて使う(同じ関数があれば引数で判定を渡す形にし、写さない)。

- [ ] **Step 4: 通す**

Run: `go test ./internal/domain/transfer/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/transfer
git commit -m "feat: 別の Store へ移すワークスペースの中身と、外との関連と件数を集める"
```

---

### Task 2: `domain/transfer`: `SettingsPeople`・`RemapPeople`・`RemapSettingsPeople`

**Files:**
- Create: `internal/domain/transfer/people_remap.go`
- Modify: `internal/domain/transfer/remap_fields.go`(人の置き換えを共有する)
- Test: `internal/domain/transfer/people_remap_test.go`

**Interfaces:**
- Consumes: `PeopleMap`、`workspace.TemplateDrafts`・`StepParent`・`StepNone`、`automation.Drafts`・`ActionAssignee`・`Unassigned`
- Produces: 3 つの関数(タスク間の名前)

- [ ] **Step 1: 失敗するテストを書く**

- `TestSettingsPeople`: テンプレートの本体の担当者 `taro`、子の担当者 `Hanako`、子の Step の担当者 `@parent` と `jiro`、本体のユーザー型の値 `saburo`、ルールの担当者の値 `shiro` と `automation.Unassigned` → `taro`・`Hanako`・`jiro`・`saburo`・`shiro`(順は問わない。`@parent`・未割り当ては入らない)。
- `TestRemapPeopleOnlySpelling`: `people = {"taro": "Taro", "hanako": "hanako"}`。担当者 `taro`・ウォッチ `[taro, hanako]`・ユーザー型の値 `taro` のアイテム → 担当者・ウォッチ(`remove taro`・`add Taro`)・ユーザー型の値の `set` だけが出る。`hanako` は綴りが同じなので行を出さない。ステータスや優先度の行は出さない。
- `TestRemapSettingsPeople`: テンプレートの子の Step の担当者 `taro` → `Taro` の差分の行(`workspace.TemplateDrafts` が出す形)、ルールの担当者の値 `taro` → `automation.Drafts` の差分の行。綴りが変わらない設定には行を出さない。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/transfer/ -run 'SettingsPeople|RemapPeople'`
Expected: FAIL

- [ ] **Step 3: 実装する**

`RemapPeople` は `remapper` の `assignee`・`person`・`watchers` を使う(`from == to` の設定で、値の引き直しの部分を呼ばない入口を作る)。`RemapSettingsPeople` は、テンプレートとルールを写して人を置き換え、元と違うものだけ `TemplateDrafts(old, new)`・`automation.Drafts(old, new)` に通す。

- [ ] **Step 4: 通す**

Run: `go test ./internal/domain/transfer/`
Expected: PASS(A5a・A5b の `Remap` のテストも通る)

- [ ] **Step 5: コミット**

```bash
git add internal/domain/transfer
git commit -m "feat: ワークスペースの設定とアイテムの人の値を、移動先のメンバーの綴りに書き直す行を作る"
```

---

### Task 3: 口と保存: `WorkspaceWriter.Import`・`Remove`、ワークスペースの `moved_in`、同期のワークスペースの記録、ファイルのまとめ

**Files:**
- Modify: `internal/app/port/stores.go`、`internal/infra/gitstore/write.go`・`sync_tree_test.go`、`internal/app/apptest/stores.go`・`sync.go`・`stores_contract_revive.go`、`internal/app/transfers/targets.go`
- Create: `internal/app/transfers/locate_kinds.go`(`project_locate.go` の中身)
- Delete: `internal/app/transfers/project_targets.go`(中身を `targets.go` へ)・`project_locate.go`

**Interfaces:**
- Produces: `WorkspaceWriter.Import(id workspace.ID, log []change.Record) error`、`Remove(id workspace.ID) error`

- [ ] **Step 1: テストを書く**

- 契約テスト(`stores_contract_revive.go`。gitstore と apptest の両方で流す): `checkWorkspaceImport`(ほかの Store のワークスペースの行を `Import` して読み戻すと、`id`・`op`・`at`・`by`・`actor`・`prev` が変わらず、`Workspaces().Get` と `Settings` で読める。同じ ID があれば `ErrInvalidOperation`)、`checkWorkspaceRemove`(`Remove` のあと `Get` が `ErrNotFound`、下のプロジェクトのフォルダは残る、同じ Tx で書いた行も捨てる)、`checkWorkspaceMovedIn`(ワークスペースのログに `transfer.NewMovedIn`(`FromProject` は空)を足しても、`Get`・`Settings` と `workspace.yaml` の中身が足す前と同じ)。
- `sync_tree_test.go` の `TestUnderTransferredWorkspace`: 片方の `transfers.jsonl` に `entity: workspace`・`target: W1`・ほかの Store の `to_store` の行があり `workspaces/W1` がない。他方が W1 に作ったプロジェクト P9 とアイテム I9 のフォルダは、合わせたあとに消える(C1 の `underTransferred` がワークスペースを祖先に含めていることの確認。通らなければ直す)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/infra/gitstore/ ./internal/app/apptest/ -run 'Contract|UnderTransferredWorkspace'`
Expected: FAIL(コンパイルエラー)。`TestUnderTransferredWorkspace` は先に通ってもよい

- [ ] **Step 3: 実装する**

gitstore は `idWriter` の `Import`・`Remove` を `workspaces/<id>` に使う(説明はないので空を渡す)。apptest は既存の `ProjectWriter` と同じ持ち方。`targets.go`・`locate_kinds.go` へのまとめは中身を変えない。

- [ ] **Step 4: 通す**

Run: `go test ./internal/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal
git commit -m "feat: ワークスペースのフォルダを別の Store から写す口と、取り除く口を足す"
```

---

### Task 4: `app/transfers`: 候補と予告

**Files:**
- Create: `internal/app/transfers/workspace_preview.go`
- Test: `internal/app/transfers/workspace_preview_test.go`

**Interfaces:**
- Consumes: Task 1〜3、A5a・A5b の `storeInfo`・`remoteOf`、`transfer.People`、`port.EditableWorkspace`・`LiveWorkspaces`、`workspace.PrefixTaken`・`NameTaken`
- Produces: `WorkspaceTargets`・`WorkspacePreview`(タスク間の名前)、内部の `refuseWorkspaceMove(dst []workspace.Workspace, w workspace.Workspace, storeName string) error`(B5c-5。Task 5 が `fn` の中でも呼ぶ)

- [ ] **Step 1: 失敗するテストを書く**

Store「e2e」(remote あり)のワークスペース「Web」(`WEB`)を Store「team」へ移す。

- `TestWorkspaceTargets`: 表示中の Store が e2e・team・ro(閲覧のみ)で、`SyncTargets` だけに hid(非表示)がある → `Stores` は team だけ、`HiddenOnly` は false。team がないと `Stores` は空で `HiddenOnly` は true。hid も閲覧のみなら false。
- `TestWorkspacePreview`: `Counts` が B5c-4 のとおり、`People` が移動先にいない人(表示名は移動元のメンバー。綴りだけ違う人は入らない)、移動先に remote がないとき `RemoteNotice` が true、`Source` が「e2e」・`Destination` が「team」。
- `TestWorkspacePreviewRefuses`: team に接頭辞 `WEB` の削除されていないワークスペース → `refusal`「team: この Store には同じ接頭辞のワークスペースがあります」。接頭辞は違い名前「Web」が同じ → 「team: この Store には同じ名前のワークスペースがあります」。両方なら接頭辞の文。削除済みのワークスペースとの重なりは断らない(B5b-4)。
- `TestWorkspacePreviewNotFound`: ないワークスペース・完全に削除したワークスペースは `port.ErrNotFound`。別の画面で削除された(`EditableWorkspace` が通る)ワークスペースは予告を返す。移動先が閲覧のみ・非表示・自分の Store なら `port.ErrNotFound`。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/transfers/ -run 'WorkspaceTargets|WorkspacePreview'`
Expected: FAIL

- [ ] **Step 3: 実装する**

移動元を読んでから移動先を読む(読み取りを入れ子にしない)。

- [ ] **Step 4: 通す**

Run: `go test ./internal/app/transfers/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/transfers
git commit -m "feat: ワークスペースを移せる Store の候補と、移動の確認の予告を返す"
```

---

### Task 5: `app/transfers`: 別の Store への移動

**Files:**
- Create: `internal/app/transfers/workspace_move.go`・`workspace_write.go`
- Test: `internal/app/transfers/workspace_move_test.go`

**Interfaces:**
- Consumes: Task 4、A5a・A5b の `Delivery`・`SendError`・`earlierMoves`・`vocabOf`・`namesOf`、`transfer.NewMovedIn`・`RemapPeople`・`RemapSettingsPeople`
- Produces: `MoveWorkspace`(タスク間の名前)、内部の `crossWorkspaceMove`(Task 6 が戻しを足す)

- [ ] **Step 1: 失敗するテストを書く**

A5a の `move_store_test.go` の偽の `Delivery` と Store ごとのロックを使う。

- `TestMoveWorkspaceWritesBothStores`: 移動先に、ワークスペース・プロジェクト・アイテム(削除済みを含む)のフォルダ(行はそのまま + `moved_in`)、ワークスペースの `set rank`(team のワークスペースの後ろ)、足した人の `join`。移動先で名前・接頭辞・スプリント・プロジェクト別のルールが移動元と同じで、削除済みのプロジェクト・アイテム・スプリントは元の削除日時で削除済み(Review Focus 3)。移動元は、3 種類のフォルダが消え、転送記録が 1 行(`entity: workspace`・`old`/`new` が `WEB`・`to_store_name` が「team」・`to_remote`・`at` は移動先の `moved_in` と同じ)。アクティビティは移動元「ワークスペース「Web」を team へ移動」、移動先「ワークスペース「Web」を e2e から移動」。
- `TestMoveWorkspaceCutsOutsideRelations`: 移動元のほかのワークスペースのアイテムとの関連が両方の側から消え、ワークスペースの中の関連は残る。
- `TestMoveWorkspaceRespellsPeople`: 移動先のメンバーに `Taro` がいて、移動元の担当者・ルールの担当者の値・プロジェクトのオーナーが `taro` → 移動先で `Taro` に書き直され、`taro` は `join` されない。
- `TestMoveWorkspaceReadsTxOnRetry`: 偽の `Delivery` が `fn` を 2 回呼び、2 回目の前に team へ接頭辞 `WEB` のワークスペースを足すと断る(Review Focus 1)。別の場合として、2 回目の前に team へワークスペースを足すと、`set rank` はその後ろ。
- `TestMoveWorkspaceSendFails`: 送れない → `*SendError`、移動元は変わらない。
- `TestMoveWorkspaceRevives`: 別の画面で削除されたワークスペースは、移動先で削除が取り消され(ワークスペース・その削除で一緒に削除されたプロジェクトとアイテム)、別に削除していたアイテムは削除済みのまま。両方の保存のあとで名前が積まれる。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/transfers/ -run MoveWorkspace`
Expected: FAIL

- [ ] **Step 3: 実装する**

`crossProjectMove` と同じ形(`prepare` で移動元を読み、移動先を読んで断りを調べ、`Delivery.Deliver(writeDestination)` → `Stores.Write(writeSource)`)。`writeDestination` は B5c-5・B5c-6・B5c-7・B5c-10 を毎回 `Tx` から決める。`writeSource` は B5c-8。戻しは Task 6 で足すので、ここでは移動元の保存が失敗したらそのエラーを返す。

- [ ] **Step 4: 通す**

Run: `go test ./internal/app/transfers/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/transfers
git commit -m "feat: ワークスペースを別の Store へ移し、移動元に転送記録を残す"
```

---

### Task 6: `app/transfers`: 戻しと移動元の読み直し

**Files:**
- Create: `internal/app/transfers/workspace_undo.go`
- Test: `internal/app/transfers/workspace_undo_test.go`

**Interfaces:**
- Consumes: Task 5 の `crossWorkspaceMove`、A5a の `UndoError`・`errSourceChanged`・`unjoinDrafts`・`lastMovedIn`、`port.ErrSaveNotDiscarded`
- Produces: `rollbackWorkspace`、`sourceStamp`

- [ ] **Step 1: 失敗するテストを書く**

- `TestMoveWorkspaceRollsBack`: 移動元の保存が失敗 → 移動先からワークスペース・プロジェクト・アイテムのフォルダと足したメンバーが消える。アクティビティは「ワークスペース「Web」の移動を取り消し」。移動元は変わらない。
- `TestMoveWorkspaceUndoFails`: 戻しも送れない → `*UndoError{Old: "Web", Store: "team"}`。
- `TestMoveWorkspaceSourceChanged`: `prepare` のあと、移動元のワークスペースのプロジェクトにアイテムを足す → 移動元の保存が失敗し、移動先を戻す(足したアイテムは移動元に残る)。設定の行(ステータスの改名)を足しても同じ(Review Focus 2)。
- `TestMoveWorkspaceSaveNotDiscarded`: `Deliver` が `port.ErrSaveNotDiscarded` → 移動元は保存せず戻す。
- `TestMoveWorkspaceOppositeDirections`: e2e → team と team → e2e のワークスペースの移動を同時に流しても止まらない(A5a の Review Focus 2 のテストと同じ組み立て)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/transfers/ -run 'MoveWorkspace(RollsBack|UndoFails|SourceChanged|SaveNotDiscarded|Opposite)'`
Expected: FAIL

- [ ] **Step 3: 実装する**

`rollbackProject` と同じ形。戻しの判定は、移動先のワークスペースのログの最後の `moved_in` の操作。`sourceStamp` は B5c-9。

- [ ] **Step 4: 通す**

Run: `go test ./internal/app/transfers/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/transfers
git commit -m "feat: ワークスペースの移動で移動元の保存が失敗したら、移動先を戻す"
```

---

### Task 7: 古いアドレスの解決とプロジェクトの履歴

**Files:**
- Modify: `internal/app/transfers/locate_kinds.go`・`locate.go`・`locate_read.go`・`locate_gone.go`、`internal/domain/history/project.go`
- Test: `internal/app/transfers/workspace_locate_test.go`、`internal/domain/history/project_test.go`

**Interfaces:**
- Consumes: Task 5(テストで移すため)、A5a の `find`・`snapshots`・`whyGone`・`storeName`
- Produces: `LocateWorkspace`(タスク間の名前)

- [ ] **Step 1: 失敗するテストを書く**

- `TestLocateWorkspaceMoved`: e2e の `WEB` を team へ移したあと、`LocateWorkspace(ctx, "e2e", "WEB")` は team のワークスペースで、`MovedFrom{Old: "WEB", Display: "team", Current: "team"}`。続けて team から other へ移すと `Display: "team", Current: "other"`。
- `TestLocateWorkspaceNamedRecordFirst`: e2e の `WEB` を team へ移し、Store「zzz」(サイドバーで team より前)にも `WEB` がある → team のもの(Review Focus 4)。`LocateWorkspace(ctx, "zzz", "WEB")` は zzz のもの(いまの接頭辞、`MovedFrom` なし)。
- `TestLocateWorkspaceRenamedBeforeMove`: e2e の `HH` を `WEB` に変えてから team へ移し、Store「zzz」にも `HH` がある → `LocateWorkspace(ctx, "e2e", "HH")` と `Locate(ctx, "e2e", "HH-1")` は team のもの(B5c-13 の一致の規則)。
- `TestLocateWorkspaceGone`: 移動先の Store を非表示 → `Gone{Reason: hidden, Old: "WEB", NewDisplay: "team", StoreName: "team"}`。移動先で削除 → `deleted`。
- `TestLocateItemViaWorkspaceRecord`: e2e の `WEB` を team へ移し、zzz にも `WEB` と `WEB-1` がある → `Locate(ctx, "e2e", "WEB-1")` は team の `WEB-1` で `MovedFrom` なし。`LocateProject(ctx, "e2e", "WEB-P1")` も同じ。移動先の Store を非表示 → `Gone{Reason: hidden, Old: "WEB-1", NewDisplay: "team"}`。
- `TestProjectHistoryMovedWithWorkspace`: `moved_in` の `FromWorkspace` がいまのワークスペースと同じ → 「e2e から移動」。違えば A5b-21 のまま「{ワークスペース名} から移動」。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/transfers/ ./internal/domain/history/ -run 'LocateWorkspace|ViaWorkspaceRecord|MovedWithWorkspace'`
Expected: FAIL

- [ ] **Step 3: 実装する**

B5c-13。スナップショットのワークスペースに削除済みも持たせ(`hasDeleted` の workspace の場合のため)、`kind` に `workspaceKind` を足して `find` の (a)(b)(c) を共有する。アイテム・プロジェクトの (c) は、種類の記録で見つからなかったあとにワークスペースの記録を見る。

- [ ] **Step 4: 通す**

Run: `go test ./internal/app/transfers/ ./internal/domain/history/`
Expected: PASS(A5a・A5b の解決のテストも通る)

- [ ] **Step 5: コミット**

```bash
git add internal
git commit -m "feat: 別の Store へ移したワークスペースと、その中のアイテム・プロジェクトを、移動元の古いアドレスから探す"
```

---

### Task 8: API と配線

**Files:**
- Create: `internal/adapter/httpapi/transfers/workspace.go`
- Modify: `internal/adapter/httpapi/transfers/transfers.go`(登録)、`api/openapi.yaml`・`web/packages/api/src/generated.ts`(生成)
- Test: `internal/adapter/httpapi/transfers/workspace_test.go`

**Interfaces:**
- Consumes: Task 4〜7
- Produces: B5c-19 の 4 つの API(タスク間の名前の表)

- [ ] **Step 1: 失敗するテストを書く**

`project_test.go` と同じ組み立て。`TestWorkspaceTransferTargets`(`hiddenOnly` と空の `targets` が `[]`)、`TestWorkspaceTransferPreview`(断りは 422 で文がそのまま)、`TestTransferWorkspace`(200 の本文、送れないとき 422、取り消しもできないとき 409、ないとき 404)、`TestLocateWorkspace`(found・gone・404)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/transfers/`
Expected: FAIL

- [ ] **Step 3: 実装して生成する**

Run: `make generate && pnpm -C web gen`

- [ ] **Step 4: 通す**

Run: `go test ./internal/... ./cmd/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal api web/packages/api/src/generated.ts
git commit -m "feat: ワークスペースの別の Store への移動と古いアドレスの解決の API を足す"
```

---

### Task 9: 画面の口と区画・ダイアログのモデル(core)

**Files:**
- Create: `web/packages/core/src/features/transfers/workspaceDialog.ts`・`workspaceDialogText.ts`
- Modify: `web/packages/core/src/model/transfer.ts`・`port/transfers.ts`・`features/transfers/leaveMoved.ts`・`actionsIndex.ts`、`web/packages/api/src/transfers.ts`
- Test: `workspaceDialog.test.ts`・`workspaceDialogText.test.ts`・`leaveMoved.test.ts`

**Interfaces:**
- Consumes: Task 8 の API、`nameMismatch`(`features/workspaces/delete.ts`)、`readOnlyNote`
- Produces: `createWorkspaceTransfer`・`createLeaveWorkspaceMoved`・`TransfersPort` の 4 つ(タスク間の名前)

- [ ] **Step 1: 失敗するテストを書く**

- `workspaceDialogText.test.ts`: 本文の行(03 の確認ダイアログの点の順)。`「Web」を e2e から team へ移動します。`、`プロジェクト 2 件・アイテム 5 件を移動します。削除済みの 3 件も一緒に移ります。`(削除済みが 0 件なら 2 文目なし)、外との関連の文(B5c-14)、`鈴木さん・高橋さんの 2 人が、移動先のメンバーに追加されます`(0 人なら行なし)、`移動先に remote がないため、ほかの人の Store からは見えなくなります。`(`remoteNotice` のときだけ)。説明文「ワークスペースをまるごと、別の git リポジトリへ移動します。移動先には、この Store の履歴は引き継がれません。」、候補がないときの 2 つの文(B5c-15)、トースト `team へ移動しました`。
- `workspaceDialog.test.ts`: 候補の読み込みと初めの値、閲覧のみで無効、「移動…」で断りならエラーのトーストでダイアログを開かない、名前が一致しないと「名前が一致しません」で送らない、一致すれば送ってトーストと `afterMoved`、送れない・オフライン(移動先に remote があり `navigator.onLine === false`)・「移動できませんでした」はダイアログを開いたまま、`NotFound` と取り消しもできなかったときは閉じて一覧を読み直す(B5c-18)。
- `leaveMoved.test.ts`: `createLeaveWorkspaceMoved` は、押したときからアドレスが変わっていなければ移動先の設定タブへ `replace` してから一覧を読み直し、変わっていれば読み直すだけ(B5c-17)。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web test -- workspaceDialog leaveMoved`
Expected: FAIL

- [ ] **Step 3: 実装する**

`createProjectTransfer` と `workspaceDelete` の形を合わせる(区画の状態・予告・名前の照合)。

- [ ] **Step 4: 通す**

Run: `pnpm -C web test && pnpm -C web check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat: ワークスペースの別の Store への移動の区画と確認ダイアログのモデルを足す"
```

---

### Task 10: 区画・ダイアログの部品と、移したあとの移り先(app)

**Files:**
- Create: `web/app/src/screens/workspace-settings/WorkspaceTransfer.svelte`・`WorkspaceTransferDialog.svelte`
- Modify: `web/app/src/screens/workspace-settings/GeneralPanel.svelte`(接頭辞と削除のあいだに区画)、`web/app/src/wireActions.ts`

**Interfaces:**
- Consumes: Task 9
- Produces: `workspaceTransfer`(`wireActions.ts`)

app の部品にはテストの置き場がない(A5b の `ProjectTransfer.svelte` と同じ)。動きは Task 9 のモデルのテストと Task 12 の e2e で確かめる。

- [ ] **Step 1: 部品を作る**

区画: 説明文・「移動先の Store」のセレクト・「移動…」、閲覧のみで無効と「閲覧のみの Store です」、候補がないときの文。ダイアログ: 本文の行・名前の入力欄・ボタン「キャンセル」「移動」(危険操作の見た目にしない)。`ProjectTransfer.svelte`・`DeleteWorkspaceDialog.svelte` と同じ作り。`afterMoved` に `createLeaveWorkspaceMoved` をそのまま渡す。

- [ ] **Step 2: 型と lint を通す**

Run: `pnpm -C web test && pnpm -C web check`
Expected: PASS

- [ ] **Step 3: コミット**

```bash
git add web
git commit -m "feat: ワークスペースの設定の「一般」に、別の Store への移動の区画と確認ダイアログを足す"
```

---

### Task 11: ワークスペースの古いアドレスと見出しの下の 1 行

**Files:**
- Modify: `web/packages/core/src/features/router/resolve.ts`・`features/screen/screen.ts`・`features/transfers/movedText.ts`、`web/app/src/screens/workspace/WorkspaceScreen.svelte`
- Test: `resolve.test.ts`・`screen.test.ts`・`movedText.test.ts`

**Interfaces:**
- Consumes: Task 8・9 の `locateWorkspace`
- Produces: `Screen.workspaceMoved`、`workspaceMovedLine`・`workspaceGoneText`

- [ ] **Step 1: 失敗するテストを書く**

- `resolve.test.ts`: ワークスペースのアドレスは、一覧ではアドレスの Store だけをいまの接頭辞・旧い接頭辞で探し、なければ `locateWorkspace` に聞く(A5b のプロジェクトと同じ)。found なら `{ name: "workspace", storeId, workspaceId, tab, movedFrom? }`、gone なら `{ name: "home", gone }`、404 なら home。`askServer` が false なら聞かない。
- `movedText.test.ts`: `workspaceMovedLine` = 「このワークスペースは {日時} に team へ移動しました」/「…team へ移動し、その後 other へ移動しました」、`workspaceGoneText` の 4 つの文(B5c-14)。
- `screen.test.ts`: `movedFrom` 付きで解決したら `workspaceMoved` に置き、アドレスを移動先の Store 名の形に替える(履歴に積まない)。タブを変えると null。いまのアドレスで開いたら null。ワークスペースの gone は `workspaceGoneText` のトースト。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web test -- resolve screen movedText`
Expected: FAIL

- [ ] **Step 3: 実装する**

`projectMoved`(A5b-24 と「実装で計画から変えたこと」の `text` の持ち方)と同じ。`WorkspaceScreen.svelte` は、出しているワークスペースとタブが `workspaceMoved` と同じときだけ、見出しの下・タブの上に `<p>` で出す。

- [ ] **Step 4: 通す**

Run: `pnpm -C web test && pnpm -C web check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat: 別の Store へ移したワークスペースの古いアドレスで、移動先を開き見出しの下に移動の 1 行を出す"
```

---

### Task 12: 通しのテスト(e2e)・仕様の文言・PR

**Files:**
- Create: `web/e2e/workspace-transfer.spec.ts`
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md`(B5c-2・B5c-14)、この計画の末尾(「実装で計画から変えたこと」)

**増やす数: spec 1 ファイル・テスト 2 件(3 幅で 6 回)。** 移すワークスペースは `createWorkspace` で作る(接頭辞は `prefixOf` の値。サンプルのワークスペースを移すと、ほかの spec が使えなくなる)。移動先は Store「team」。

| テスト | 確かめること |
|---|---|
| 1 件目 | e2e にワークスペース(接頭辞 `{prefix}`)・プロジェクト・Task を作り、設定の「一般」で「移動先の Store」に team を選び「移動…」→ 確認ダイアログの本文を見て、違う名前で「移動」→「名前が一致しません」。名前を入れて「移動」→ トースト「team へ移動しました」、アドレスが `/s/team/w/{prefix}/settings`、サイドバーの e2e にワークスペースがなく team にある。古いアドレス `/s/e2e/w/{prefix}/overview` が team の概要で開き、見出しの下に「このワークスペースは … に team へ移動しました」。タブを「ツリー」に変えると消える。Task の `/s/e2e/i/{prefix}-1` も開く |
| 2 件目 | 閲覧のみの Store のワークスペースの設定で、区画のセレクトと「移動…」が無効で「閲覧のみの Store です」が添う |

- [ ] **Step 1: テストを書き、1 回流す**

Run: `make e2e-bin && pnpm -C web e2e workspace-transfer.spec.ts`
Expected: 6 件が通る。

- [ ] **Step 2: 仕様に文言を足す**

03「別の Store へ移動」に B5c-2(移動先を開き、見出しの下に 1 行。リンクの記述を直す)と B5c-14 の文言を書き、差分を見せて承認を取る。

- [ ] **Step 3: 全体の検査**

Run: `make check`
Expected: 緑。

- [ ] **Step 4: 計画に実装で変えたことを足す**

この計画の末尾に「## 実装で計画から変えたこと」を足す(なければ「なし」)。

- [ ] **Step 5: コミットして PR を出す**

```bash
git add web docs
git commit -m "test: ワークスペースの別の Store への移動と古いアドレスの通しのテストを足す"
git push -u origin feat/track-b5c
gh pr create --base develop --head feat/track-b5c \
  --title "feat: トラック B5c(ワークスペースの別の Store への移動)" \
  --body-file <本文を書いたファイル>
```

本文は PR #208 の見出しに合わせる(「概要」「範囲」「ユーザーに決めてもらったこと」「仕様に足した文言」「計画から変えた主なもの」「develop の取り込み」「他のトラックへ渡すもの」「確かめたこと」)。末尾は `🤖 Generated with [Claude Code](https://claude.com/claude-code)`。「他のトラックへ渡すもの」には、C4 = アクティビティの絞り込みで移動元にないワークスペースを選択肢に出さないことと「メンバー {ログイン名} を追加」、A5c・E = 古いアドレスと同じ規則(`LocateWorkspace` とワークスペースの記録からのアイテム・プロジェクトの解決)、G = 移動先で復元するときの名前の重なり、を書く。

## 実装で計画から変えたこと

- **B5c-12 の「いまのワークスペース」は、その操作を終えたときのワークスペースで比べる。** 計画は「いまのワークスペースと同じ」としていたが、あとで同じ Store の中で別のワークスペースへ移すと、履歴の行が変わってしまうため。操作の最後の行までのログを `project.FromLog` に渡して求める(同じ操作の中の場所の set も反映される)。仕様 02「履歴」に書いた。
- **ワークスペースの記録で辿れたが、その番号のアイテム・プロジェクトが削除済みなら `Gone deleted` を返す。** 計画に書いていない。これがないと、ほかの Store の同じ ID を開いてしまう。移動先のワークスペースが削除済みのときも同じ。
- **検索の優先順を、種類の記録 → 種類の記録の理由 → ワークスペースの記録 → ワークスペースの記録の理由とした。** 計画では `find` の (c) を並列に読める書き方だったが、種類の記録が完全な削除を指すときに移動先の同じ ID を開かないよう、順に読むことにした。アドレスの Store 名の Store がないときも同じ順(`searchIn` に集約)。
- **ワークスペースの記録の照合で使う、見つけたワークスペースの接頭辞は、記録の `at` までにログに出たものに絞った(B5c-13)。** 計画は「いまの接頭辞・旧い接頭辞」だったが、それでは移したあとで移動先で使い始めた接頭辞(例: team で APP に変えた)でも、移動元に一度もなかった `/s/work/w/APP` が移動の 1 行付きで開いてしまうため。接頭辞ごとの最初の日時は `workspace.PrefixTimes` で求める。仕様 03 に書いた。
- **`MovedFrom.Old` は、記録の `old` ではなく画面の接頭辞にした。** `Gone.Old` と同じ意味に揃えたため。
- **移動先の Store が非表示で、移す前に接頭辞を変えていたときは、旧い接頭辞でたどれない。** 旧い接頭辞は、表示中の Store の中でだけ照合する。仕様 03 に書いた。
- **`UndoError` に `Workspace` の形を足した。** `NewDisplay` が空のとき、B5c-14 の文を返す。
- **`deliverUndo` と `moveError` のヘルパーを足した。** 計画はそれぞれのファイルに書く想定だった。`deliverUndo` は引数 7 個のまま共通化した。`moveError` は 409 の Undo・422 の Send・それ以外を振り分け、move.go・project.go・workspace.go が共有する。
- **ワークスペースの戻しは、移動先のワークスペースの下にあるすべてのフォルダ(ワークスペース・プロジェクト・アイテム。削除済みを含む)を `Remove` で消す。** `Remove` 自体は下のものを消さないので、`rollbackWorkspace` が順に消す。`ErrNotFound` は飛ばし、保存が実際には捨てられていた(`moved_in` がない)ときはメンバーの行を書かない。移動先の保存から移動元の保存までのごく短いあいだに、ほかの書き手が移動先の別のワークスペースから、移したワークスペースへプロジェクトやアイテムを移していると、戻しはそれも消す(下に残して親のないものにするより、まとめて消すほうを選んだ)。
- **移動元の読み直しを `gatherWorkspace` の再実行から、`sourceStamp`(ログごとの行数と最後の行の id)の比較に絞った(B5c-9)。** 説明・設定・前の移動・メンバーは読まない。外のアイテムのログは比べず、読み直した `Outside` から外との関連だけを外す。
- **ファイルのマージ。** `project_targets.go` を `targets.go` に、`project_locate.go` を `locate_kinds.go` に改め、`project_targets_test.go` を `targets_test.go` に統合した(transfers の非テストのファイルは 20 のまま)。`matching`・`aliases` は `locate.go` から `locate_read.go` に移した。`locate_kinds.go` の `withStoreName`・`viaWorkspace`・`deletedIn`・`prefixesOf` は計画の名前の一覧にない。契約テストの登録は `runWorkspaceContract` にまとめた(`stores_contract.go` を 300 行以内に保つため)。
- **既存のテスト `TestProjectRowsMoved` の O7 に、同じ操作の中の場所の set(W3 → W4)を足した。** O7 の元の形(`FromWorkspace` が移動先のワークスペースと同じで場所の set がない)は、B5c-12 ではワークスペースごと移った形そのもので、期待が「team から移動」になるため。A5b の実際の行の形に合わせ、期待する文は変えていない。
- **`createLeaveWorkspaceMoved` は、結果の移動先の Store 名と接頭辞から経路を作る(カタログを引かない)。** ホームへ落ちないようにするため。`createLeaveMoved` と共有する `leaving` ヘルパーを切り出した。オフラインの判定は `destinationHasRemote && !online` で、`crossStore` の項目は持たない(ワークスペースの移動は常に Store をまたぐ)。
- **ワークスペースの古いアドレスは、カタログではアドレスに名前のある Store だけを探し、見つからなければ `locateWorkspace` を呼ぶ。** 書き換えるアドレスは、カタログの Store 名と接頭辞だけから作る(`route.prefix` は使わない)。カタログに当のワークスペースがないときはアドレスを書き換えず、次のカタログの読み込みでもう一度サーバーに聞く。
- **ほかの書き手が移して、出していたワークスペースが一覧の読み直しで見つからなくなったときは、アドレスもホームに替える(B5c-17)。** 計画は「いまの扱いのまま」としていたが、画面だけをホームにしてアドレスを残すと、次の一覧の読み直しでそのアドレスをサーバーに聞き直し、移った先へひとりでに移ってしまうため。プロジェクトの画面が消えたときにワークスペースの概要へアドレスを替える(A4b-22)のと同じ形にした。
- **e2e の「タブを変えると消える」は、ワークスペースのタブに「ツリー」がないため「スプリント」で確かめた。** 計画の文は「ツリー」だった。アイテムの古いアドレスは、移動先のプロジェクトのツリーと詳細パネルを開くので、パネルのタイトルで確かめた(アドレスは `/s/team/i/…` ではなく `/s/team/p/{接頭辞}-P1/…`)。
- **Task 8 のテストは、実装と同時に書き、先に失敗させなかった(RED なし)。** 4 つの操作がプロジェクトの API と同じ形だったため。レビューで `moveError` の共通化、`hiddenOnly` の候補のテスト、取り消しもできなかったときの文の完全一致を足した。
- **計画にないテストを足した。** `TestMoveWorkspaceIgnoresOutsideChanges`(あいだの外のアイテムの変更で止まらない)、`TestMoveWorkspaceOppositeDirections`(逆向きの同時移動)、`TestMoveWorkspaceSourceChanged` のアイテムの編集、`TestProjectHistoryOfWorkspaceMove`(app 層)、`TestLocateItemRecordBeforeWorkspaceRecord(InOtherStores)`。
- **Task 10(部品)にはテストを足さず、検査と e2e で確かめた(計画どおり)。** 確認ダイアログと削除ダイアログの似た作り、Task 9 のプロジェクトのダイアログと似た送信の骨組みは、共通化しなかった。
- **実装で見つけて直した A5a・A5b の不具合(ユーザーの決定、2026-11-14)。** アイテムとプロジェクトの別の Store への移動の戻しは、prepare で読んだアイテムだけを消していたため、移動先の保存から戻しまでのあいだに移動先で移したアイテムの下・移したプロジェクトに作られたアイテムが、親のないものとして残っていた。ワークスペースの戻しと同じく、戻しの `Tx` から移動先のいまの中身を読んで消すようにした(アイテムは移動先のプロジェクトの一覧から移したアイテムとその下を `item.Descendants` で、プロジェクトは `removeProjectFolders` を共有して、削除済みを含めて消す)。テストは `TestMoveAcrossStoresRollbackRemovesChildrenAddedAtTheDestination`・`TestMoveProjectAcrossRollbackRemovesItemsAddedAtTheDestination`。
