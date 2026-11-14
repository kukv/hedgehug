# トラック A5b(プロジェクトの別のワークスペース・別の Store への移動) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** プロジェクトの設定タブの「別のワークスペースへ移動」で、プロジェクトとその下のすべてのアイテム(削除済みを含む)を、同じ Store の別のワークスペースと別の Store のワークスペースへ仕様どおりに移す。プロジェクト別のルールと専用ルールを移動先の名前に合わせて引き継ぎ、転送記録を残して、プロジェクトの旧 ID のアドレスから辿れるようにする。

**Architecture:** A5a の部品をそのまま広げる。移す集まりの規則(プロジェクトの下のすべてのアイテムと外との関連、Epic ごとの番号、ルールの引き継ぎ)は `internal/domain/transfer` に純粋な関数で足し、操作は `internal/app/transfers` に `MoveProject` などを足す。同じ Store の中は 1 回の `Stores.Write`。別の Store は A5a と同じく、移動先を `Delivery.Deliver`(受け取り → 保存 → 送信を 1 つのロックで)、そのあと移動元を `Stores.Write`、移動元が失敗したら移動先を戻す。保存の口は `ProjectWriter.Import`・`Remove` を足す。旧 ID の解決は `transfers.Service.LocateProject` を足し、画面は `resolve.ts` のプロジェクトのアドレスがカタログで見つからないときにサーバーへ聞く。

**Tech Stack:** Go(huma・domain/app/infra/adapter の層、go-git v5)、TypeScript(core・api の orval)、Svelte 5(ui・app)、Vitest、Playwright。

**Spec:** `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md`(「名前の変更・移動・削除」の **移動**、「画面の見出しと構成」の設定タブの表、「別のワークスペースと別の Store への移動」のうち「値の引き直し」「関連・転送記録・旧 ID」「保存」。プロジェクトの移動は「アイテムと同じ」とする箇所が多い)、`03-workspace-settings.md`(「プロジェクト別のルール」の「移動・削除・復元」、「自動化」の動かない操作、「移動のステータスの置き換え」の行)、`06-sync-notifications.md`(「他の書き手との重なり」の削除の取り消し、「別の Store へ移動したものと重なったとき」)、`01-overview.md`(「画面のアドレス」)。保存形式は `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`(「移動・番号・旧 ID」の「プロジェクトを移すときは…」、「変更記録」の `moved_in`、「アクティビティと履歴」)。

**実装担当にタスクを渡すときは、タスクの本文と一緒に「Global Constraints」「持ち場と境目」「決めたこと」「タスク間の名前」の節を必ず渡す。** 食い違ったときは、この 4 節と仕様が本文より優先する。

**前提として読むもの:** A5a の計画 `2026-11-14-hedgehug-track-a5a.md` の「決めたこと」(A5a-1〜A5a-23)と「タスク間の名前」、PR #205 の本文の「計画から変えた主なもの」(`gh pr view 205 --json body -q .body`)。A5b は A5a の型と口をそのまま使い、同じ規則を繰り返して書かない。

## Global Constraints

- 依存は追加しない(足すときは `go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` で版を明記し、公開から 7 日以上の版だけ。Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 大きさの上限: Go ファイル 300 行(テスト 600 行)・関数 60 行・認知的複雑度 15・パッケージ 20 ファイル(テストを除いて数える)、Svelte 200 行、TS 300 行。超えたら分ける。いまの数: `internal/app/apptest` 20(新しいファイルを足さない)、`internal/infra/gitstore` 19(足さない)、`internal/app/transfers` 11、`internal/domain/transfer` 9。
- 向き: adapter→app→domain、infra→app/port。app の機能どうしは import しない(テストも)。`app/transfers` は `app/projects`・`app/items` を import しない(要る小さな関数は写す。A5a の `lastRank` と同じ)。
- 画面の文言は仕様の文字どおり。仕様にない文言は「決めたこと」の A5b-18 の文を使い、Task 12 で仕様に書き足す。モックの動きは写さない。
- 通知は作らない。自動化は動かさない(03「自動化」: プロジェクトの別のワークスペースへの移動での値の引き直しとステータスの置き換えでは動かない)。
- 閲覧のみの Store のプロジェクトでは、設定の「別のワークスペースへ移動」の区画のセレクトと「移動」を無効にし「閲覧のみの Store です」を添える。閲覧のみの Store は移動先の候補に出さない。
- 各タスクは TDD(失敗するテスト → 失敗を確かめる → 実装 → 通す → コミット)。タスクごとにレビューする。
- コミットの末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`。
- PR はトラックで 1 本(develop 向け、ブランチ `feat/track-a5b`)。force push は拒否されるので、載せ直すときは別名のブランチで出す。
- 計画やテストにトークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e のビルドも CI で lint する。
- 確認のコマンド: Go は `go test ./internal/... ./cmd/...`、全体は `make check`、画面は `pnpm -C web test`・`pnpm -C web check`、API の型は `make generate` のあと `pnpm -C web gen`、e2e は `pnpm -C web e2e`(`web/e2e/fixtures.ts` が worker ごとに空いた番号で `bin/hedgehug-e2e` を起動する)。手元の検証は 1 回で足りる。
- `api/openapi.yaml`・`web/packages/api/src/generated.ts` は手でマージしない。API を変えたタスクで `make generate` → `pnpm -C web gen` で作り直してコミットする。
- 本文のテストの名前と期待は下書きで、実行していない。仕様と「決めたこと」が正。

## Review Focus

1. **`SyncSave` の試み直しで古い値を使う**: 別の Store への移動の `fn` は、送信が重なると最大 3 回呼び直される(A5a-5)。移動先の同じ名前のプロジェクト・同じ名前のルールの断り、プロジェクトと各アイテムの番号、ルールの引き継ぎの対応、移動先のメンバーは、毎回 `fn` の `Tx` から読んで決める。テストは Task 6(偽の `Delivery` が `fn` を 2 回呼び、2 回目の前に移動先へ同じ名前のプロジェクトを足すと断る / 番号が進む)。
2. **戻しでルールが移動先に残る**: 移動先の保存のあと移動元の保存が失敗したとき、移動先から、写したプロジェクト・アイテムのフォルダと足したメンバーだけでなく、移動先のワークスペースのログに書いたプロジェクト別のルールの行と作った専用ルールも戻す。テストは Task 6。
3. **プロジェクトが隠しているステータスの置き換え**: プロジェクトが「使わない」にしているステータスをアイテムがまだ持っているとき、そのステータスの名前が移動先のワークスペースにあれば置き換えない(ルールと一緒に移るので、移動の前と同じ状態になる)。置き換えの対象は「移動先のワークスペースにない名前」だけ。テストは Task 2(`StatusRows`)と Task 5。
4. **アイテムのないプロジェクトと、削除済みの Epic**: アイテムが 0 件のプロジェクトも移せる(番号の行はプロジェクトの分だけ)。削除済みの Epic とその下は、削除済みのまま元の削除日時で移り、番号は削除されていない Epic の下をすべて振ったあと。テストは Task 1・5。
5. **旧 ID のアドレスのタブ**: `/s/{store}/p/{旧 ID}/tree` は、移したプロジェクトのツリーのタブを開き、見出しの下に 1 行を出す。タブを変えると 1 行は消える。いまの ID で開いたときは出さない。テストは Task 11(`screen.test.ts`)と Task 12。

---

## 持ち場と境目

| 入れるもの | 仕様 |
|---|---|
| 設定タブの「別のワークスペースへ移動」の区画(説明文・移動先のセレクト・「移動」・候補がないときの文・閲覧のみの無効) | 02「画面の見出しと構成」の表、「名前の変更・移動・削除」の **移動** の候補 |
| 確認ダイアログ(本文の 4 文、ルールの 1 文と「移らないもの」、「ステータスの置き換え」) | 02 **移動** の 1〜3 つ目の点 |
| 同じ Store の別のワークスペースへの移動(1 回の保存、プロジェクトとアイテムの番号の振り直し、値の引き直し、ルールの引き継ぎ、転送記録) | 02 **移動**、保存形式「移動・番号・旧 ID」の「プロジェクトを移すときは…」 |
| 別の Store への移動(移動先の受け取り・保存・送信 → 移動元の保存、失敗で元のまま、`moved_in`、外との関連の解除、メンバーの追加、戻し) | 02 **移動** の 4〜6 つ目の点、「保存」 |
| 移動先の同じ名前のプロジェクト・同じ名前のルールの断り | 02 **移動** の 2・3 つ目の点 |
| 別の画面で削除されたプロジェクトを移すときの削除の取り消し | 06「他の書き手との重なり」(A5a-12 と同じ扱い) |
| プロジェクトの旧 ID の解決(画面のアドレス、見つからないときのトースト、見出しの下の 1 行) | 02 **移動** の 5 つ目の点、01「画面のアドレス」 |
| プロジェクトの履歴の「{移動元のワークスペース名} から移動」 | 02 **移動** の 5 つ目の点、「履歴」の表の「プロジェクトの移動・削除」 |
| アクティビティ「プロジェクト「{タイトル}」を {移動元} から {移動先} へ移動」、トースト「{移動先} へ移動しました」 | 02 **移動** の 6・7 つ目の点 |

| 入れないもの | 回す先 | 理由 |
|---|---|---|
| コマンドパレット(プロジェクトの旧 ID の候補を含む) | A5c | A5 を 3 本に分けた決定(2026-11-14)。A5c は `LocateProject` と同じ規則で引く |
| スプリントとリリースを外す | F | アイテムにスプリント・リリースの項目がまだない。F が `transfer.Remap` に外す行を足せば、プロジェクトの移動も同じ `Remap` を通るので足りる(F1 の計画の Task 19・F1-23) |
| 検索(`project:` のキー)でのプロジェクトの旧 ID | E | 検索は E。E は `LocateProject` と同じ規則で引く |
| アクティビティの画面と受信箱のプロジェクトの旧 ID のリンク、「メンバー {ログイン名} を追加」の 1 人 1 行 | C4 | A5a-11 と同じ。リンクは `locate-project` に旧 ID を渡せば辿れる |
| 別の Store へ移したプロジェクトの下に、移動元で他の人が作ったもの・変えたものとの重なり、プロジェクト別のルールの持ち主がいなくなったときの修復 | C2(入っている: `underTransferred` はプロジェクトの祖先も辿る)、C3a(入っている: 修復(7)) | 同期の側はエンティティの種類を問わず扱っている |
| 「完全に削除」の書き込み | G | A5b は `purged.jsonl` を読むだけ |
| ワークスペースの別の Store への移動 | B5c | 転送記録の形(`entity: workspace`)は A5a-1 で決めてある |

## 決めたこと

| # | 決定 | 根拠 |
|---|---|---|
| A5b-1 | 1 本の計画・1 本の PR(サーバーと画面を分けない) | ユーザーの決定(2026-11-14) |
| A5b-2 | プロジェクトの旧 ID のアドレス(`/s/{store}/p/{旧 ID}/{タブ}`)から開いたときは、開いたタブのまま移したプロジェクトを開き、プロジェクトの見出し(ID とタイトル)の下・タブの上に「{旧 ID} は {日時} に {新 ID} へ移動しました」を出す(そのあとさらに移したときは A5a-16 の「…へ移動し、その後 {いまの ID} へ移動しました」)。タブを変える・別の画面を開くと消える。普通に開いたときは出さない。仕様 02 の「プロジェクトの概要の上部」を「プロジェクトの画面の見出しの下」に直す(Task 12) | ユーザーの決定(2026-11-14) |
| A5b-3 | 移す集まりは `transfer.CollectProject(p project.ID, all []item.Item) ProjectSet`: プロジェクトの下のすべてのアイテム(削除済みを含む)を、Epic の並び(削除されていない Epic を順位の順 → 削除済みの Epic を順位の順)ごとに、Epic → その下(`transfer.Number` と同じ深さ優先)の順に並べる。「外」は、`all` にいて集まりにいないアイテムとの関連(両方の側。A5a の `Cut`) | 02 **移動**「番号は、Epic を並び順に、それぞれ同じ規則で振る」「関連の解除(移すアイテムの集まりの外との関連)」、02「移すもの」の削除済みの子の規則を Epic にも当てる |
| A5b-4 | 番号は `transfer.NumberProject(set ProjectSet, next int) map[item.ID]int`: Epic ごとに `transfer.Number(epic, その下, next)` を呼び、`next` を使った数だけ進める。プロジェクトの番号は、移動先のワークスペースの `numbering.Next(log, numbering.Projects)` | 02 **移動**、保存形式「使った最大番号」 |
| A5b-5 | ステータスの置き換えの対象は、移すアイテム(削除済みを含む)のステータスと、このプロジェクトの専用ルールが使うステータス(`automation.Statuses`)のうち、名前が移動先のワークスペースにないもの。`transfer.Unmatched` と `transfer.Remap` に渡す `toRules` は空の `projectrule.Rules{}`(ルールは一緒に移るので、移動先のワークスペースのすべてのステータスが使える)。行の並びは移動元のワークスペースのステータスの並び。件数はアイテムの数で、ルールだけが使うものは 0(画面は件数を出さない)。選択肢は、移動先のワークスペースのステータスのうち、引き継ぐ「使わないステータス」(A5b-6)に入らないもの(`transfer.UsableStatuses(to, carried)`。使わないステータスへ置き換えると、移したあとのアイテムや専用ルールがプロジェクトの使わないステータスを持つため)。初めの値は同じ分類の移動先の既定(既定は使わないことにできないので、必ず選択肢にある)。送られた置き換え先が選択肢にないときは、初めの値にする(`Remap`・`CarryRules` に渡す前に、app が選択肢にないものを捨てる) | 02 **移動** の 3 つ目の点(A5a の「移動先のプロジェクトが使わないもの」はここでは対象にしない)、Review Focus 3 |
| A5b-6 | ルールの引き継ぎは `transfer.CarryRules(in CarryInput) Carried` の 1 か所。移動先に合わせる規則は 03「移動・削除・復元」のとおり: 使わないステータスは名前の一致(ただし、移動先の初期ステータスか分類ごとの既定のもの、つまり `projectrule` の `checkCanHide` が断るものは移さず、「移らないもの」の `hiddenFixed` に出す。B5a の「使わないことにできない」規則を移動でも守るため)、遷移のチェックは From と To の両方が名前で一致、使うテンプレートは名前と種類の一致(1 件も合わなければ何も書かない = 限定なし)、種類ごとの既定は名前と種類の一致(`projectrule.None` はそのまま、合わなければ書かない = ワークスペースと同じ)、動かさない指定は引き継がない。専用ルールはすべて移り、移動先では **新しい内部 ID**(`port.IDs`)で作る(同じ ID で作ると、前に同じワークスペースから移したときの `delete` の行が残っていて削除済みと読まれるため)。きっかけ・することのステータスは、名前が移動先にあればその値、なければ `statusChoice`、なければ同じ分類の移動先の既定。担当者の値は、別の Store なら `PeopleMap` で置き換える(`automation.Unassigned` はそのまま)。名前・有効・きっかけの種類はそのまま | 03「移動・削除・復元」、保存形式「移動・番号・旧 ID」 |
| A5b-7 | 移動元のルールの行は、移動の保存の中で消す: `projectrule.ResetDrafts` の 4 つの区画の行と、専用ルールごとの `automation.DeleteDraft`。移動先には、合ったものの `add`・`set` と専用ルールの作成の行(`automation.Drafts(automation.Rule{}, r)`)を書く。同じ Store では、両方を同じ `Write` で書く | 03「移動元の分は、移動の保存の中で削除する」、保存形式 |
| A5b-8 | 「移らないもの」は、サーバーが `[]transfer.NotCarried{Kind, Name, Missing []string, Count int}` で返し、文は画面が作る。`Kind` は `hidden`(「使わないステータス「{Name}」(移動先にありません)」)・`workflow`(Name は「{From} から {To}」、「遷移のチェック「{Name}」(移動先に「{Missing[0]}」がありません)」。両方ないときは「移動先に「{From}」「{To}」がありません」)・`template`(「使うテンプレート「{Name}」(移動先にありません)」)・`templateDefault`(Name は種類、「{種類} の既定のテンプレート「{テンプレート名}」(移動先にありません)」。テンプレート名は `Missing[0]`)・`hiddenFixed`(「使わないステータス「{Name}」(移動先では使わないことにできません)」)・`disabled`(「ルールを動かさない指定 {Count} 件(移動先のルールには引き継ぎません)」、0 件なら行を出さない)。並びは hidden・hiddenFixed(移動元の並びで混ぜる) → workflow → template → templateDefault → disabled、各種類の中は移動元の設定の並び | 02 **移動** の 3 つ目の点の例 |
| A5b-9 | 断り: 移動先のワークスペースに同じ名前の削除されていないプロジェクトがある → `refusal.New("{移動先} には同じ名前のプロジェクトがあります")`(比べ方は `project.CheckRename` と同じ)。移動先のワークスペースに、移す専用ルールと同じ名前の削除されていないルールがある → `refusal.New("{移動先} には同じ名前のルールがあります: {名前}。先にルールの名前を変えてください")`(複数なら移動元の並びで最初の 1 つ)。順はプロジェクト → ルール。別の Store では、`prepare`(送る前)と `fn` の中(`Tx` から読み直して)の両方で調べる。{移動先} は A5b-10 の書き方 | 02 **移動** の 2・3 つ目の点、Review Focus 1 |
| A5b-10 | {移動元}・{移動先} の書き方は A5a-11 と同じ: ワークスペース名、別の Store のときは「{Store 名} の {ワークスペース名}」。確認ダイアログの本文・断り・アクティビティ・トーストのすべてで使う。アクティビティは「プロジェクト「{タイトル}」を {移動元} から {移動先} へ移動」(移動先・移動元の両方の保存に同じ文)、戻しは「{新 ID} の移動を取り消し」(新 ID はプロジェクトの新 ID)、トーストは「{移動先} へ移動しました」 | 02 **移動** の 6・7 つ目の点、A5a-11 |
| A5b-11 | 同じ Store の中の移動の行(1 回の `Write`): プロジェクトに `set place`(ワークスペースと新しい番号)と、順位が変わるなら `set rank`(移動先のワークスペースの削除されていないプロジェクトの末尾)。アイテムごとに `set place`(ワークスペースと新しい番号。プロジェクト・親・順位は変えない)と `transfer.Remap` の行。移動先のワークスペースに `number` の行(プロジェクトの番号を `numbering.Projects`、アイテムの番号を `numbering.Items`)とルールの行(A5b-7)、移動元のワークスペースにルールを消す行。転送記録はプロジェクト(`entity: project`、`old` は `HH-P1`、`new` は `MB-P2`)とアイテムごとに 1 行(`to_store` は自分) | 保存形式「移動・番号・旧 ID」 |
| A5b-12 | 別の Store への移動の行: 移動先に、プロジェクトのフォルダ(`log.jsonl`・`description.md`)を `ProjectWriter.Import` で、アイテムのフォルダを `ItemWriter.Import` で写し、プロジェクトに `moved_in`(`FromProject` はそのプロジェクト自身、`Names` は空)・`set place`・`set rank`・オーナーが変わるなら `set owner`、アイテムに A5a と同じ `moved_in`・`set place`・`Remap` の行・外との関連の `remove`、移動先のワークスペースに `number` の行とルールの行、メンバーの `join`(A5a-11 と同じく同じ保存)。移動元は、アイテムとプロジェクトのフォルダを `Remove`、外のアイテムに関連の `remove`、移動元のワークスペースにルールを消す行、転送記録をプロジェクトとアイテムごとに 1 行。順と失敗の扱いは A5a-6(移動元の保存の直前に集まりを集め直し、違えば失敗して戻す)。移動元の保存では、プロジェクト別のルールと専用ルールも `Tx` から読み直し、`prepare` で読んだものと違えば(あいだに別の書き手がルールを変えた)同じく `errSourceChanged` で失敗して戻す(読み直さずに消すと、あいだに足された専用ルールが移動元に残り、移動先にも移らない) | 保存形式「移動・番号・旧 ID」の「別の Store」、02「保存」 |
| A5b-13 | 移動先に足す人は、A5a の `transfer.People` に `extra []string` を足して数える(プロジェクトのオーナー、専用ルールの担当者の値。`logs` にプロジェクトのログも入れるので、プロジェクトのコメントの書き手と履歴の操作者は `by` で数える)。A5a の呼び出しは `nil` を渡す | 02 **移動** の 4 つ目の点の「移すものに現れるすべての人」 |
| A5b-14 | 戻し(移動元の保存が失敗したとき)は A5a-6 に加えて、移動先のワークスペースのログに書いたルールの行を戻す: 足した `add` を `remove`、`set` した種類ごとの既定を `set`(なし)、作った専用ルールを `automation.DeleteDraft`。移動先にプロジェクトの `moved_in` がない(保存されていなかった)なら何も書かない(A5a の `unjoin` と同じ判定を、プロジェクトのログで行う) | Review Focus 2 |
| A5b-15 | 別の画面で利用者が削除したプロジェクト(`port.EditableProject` を通るもの)を移すときは、A5a-12 と同じく削除を取り消してから移し、06 のトースト「{旧 ID} は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました」を出す(応答の `Hedgehug-Restored`)。同じ Store は保存の中で `port.ReviveProject`、別の Store は写した行のその削除の操作の行を移動先の同じ保存で `change.Undo` で取り消し、両方の保存が済んでから `port.NoteRevived`(プロジェクトの旧 ID) | 06「他の書き手との重なり」、A5a-12 |
| A5b-16 | 候補は `ProjectTargets`: 表示中の Store のうち閲覧のみでないもの(`StoreList`)の、自分のワークスペースと削除済みのワークスペースを除くすべてのワークスペース(プロジェクトの有無を問わない)。並びは A5a-13 と同じ(Store の並び → `workspace.RankOf` → 内部 ID)。候補がないときは、セレクトの代わりに「移せるワークスペースがありません」を出し、「移動」を無効にする | 02 **移動** の候補(アイテムの「プロジェクトが 1 つ以上」は付けない) |
| A5b-17 | 旧 ID の解決は `transfers.Service.LocateProject(ctx, storeName, display string) (LocatedProject, error)`。A5a-15 と同じ順(Store 名の Store を先に、(a) いまの ID、(b) 旧い接頭辞、(c) その Store の転送記録の `entity: project` の行を `at` の新しい順に、`target` を表示中のすべての Store から探す)で、プロジェクトを探す。削除済みのプロジェクト・削除済みのワークスペースのプロジェクトは開かない。見つからないときの `Gone` は A5a-15 と同じ 4 つ(`deleted`・`purged`・`hidden`・`elsewhere`)。`display` の形は `{接頭辞}-P{番号}` | 02 **移動** の 5 つ目の点「解決の規則はアイテムと同じ」 |
| A5b-18 | 仕様にない文言: {移動元}・{移動先} の書き方(A5b-10)、「移らないもの」の `hiddenFixed` の文(A5b-8)、区画の説明文「このプロジェクトと、その下のすべてのアイテム(削除済みを含む)を、別のワークスペースへ移します。」、セレクトの名前「移動先のワークスペース」、確認ダイアログのタイトル「プロジェクトを移動」、ボタン「移動」「キャンセル」、「移らないもの」の `workflow` の両方ないときの文と `templateDefault` の文(A5b-8)、失敗のトースト(A5a-12 と同じ文: 「{移動先の Store 名} に送信できないため移動できません: {理由}」「オフラインのため {移動先の Store 名} へ移動できません」「移動できませんでした」、取り消しもできなかったとき「{旧 ID} を {移動先の Store 名} の {新 ID} に移しましたが、移動元から取り除けず、取り消しもできませんでした。どちらかを削除してください」)。Task 12 で仕様 02 に書き足し、PR の本文に一覧で出す | A5a-23 と同じ扱い |
| A5b-19 | 移したあと、「移動」を押したときからアドレスが変わっていなければ、画面は移したプロジェクトの新しいアドレスの「設定」タブへ移り(履歴に積まない `replace`)、**そのあとで**サイドバーの一覧を読み直す(先に読み直すと、古いアドレスが「出していたプロジェクトが消えた」と見られ、ワークスペースの概要へ移ってしまう。`screen.ts` の `shown`)。押したあとに画面を移っていたら、移らずに一覧を読み直すだけにする(A5a-18 と同じ)。移り方は core の `createLeaveMoved`(`() => (result) => Promise<void>`。押したときに呼び、移せたら結果を渡す。`createAfterMoved` と同じ形)で、`createProjectTransfer` の `afterMoved` にそのまま渡す。新しいアドレスは、一覧を読み直すまでは `locateProject` のいまの ID で解決するので、1 行は出ない。ほかの書き手(同期・別のタブ・別のプログラム)が移したときは、いまの扱い(A4b-22: ワークスペースの概要へ移る)のまま: `screen.ts` は、出していたプロジェクトのアドレスを読み直すときは `locateProject` を呼ばない | A5a-18(自分で移したときは移した先を開き、ほかの書き手のときは切り替えない)をプロジェクトの画面に当てた |
| A5b-20 | 失敗の扱いは A5a-12 と同じ: 送れない・オフライン・断り(A5b-9)・それ以外はトーストを出してダイアログを開いたまま。`NotFound` と取り消しもできなかったときは、ダイアログを閉じて画面を読み直す。オフラインは、移動先に remote があり `navigator.onLine === false` のとき画面が決める | A5a-12 |
| A5b-21 | プロジェクトの履歴は、`set place` の行でワークスペースが変わったもの(同じ Store)と `moved_in` の行(別の Store)を「{移動元のワークスペース名} から移動」と出す。同じ Store の名前は `ProjectContext.WorkspaceName`(削除済み(論理削除)のワークスペースも名前を出し、Store にないものだけ「(削除された値)」。アイテムの履歴の `readOrigin` と同じ)、別の Store は `moved_in` の `FromWorkspaceName`。プロジェクトの `moved_in` にも、アイテム(A5a-2)と同じく `Moves`(移動元の Store の中の前の移動)を入れ、前の移動の行のワークスペース名はそこから引く。同じ操作に `moved_in` があれば `set place` は出さない。`set owner`(人の置き換え)は、ほかの変更と同じく出す | 02 **移動** の 5 つ目の点、「履歴」の表 |
| A5b-22 | API(`/api/…`): `GET stores/{store}/projects/{project}/transfer-targets` → `[{storeId, storeName, workspaceId, workspaceName}]`、`GET stores/{store}/projects/{project}/transfer-preview?toStore=&toWorkspace=` → A5b-23、`POST stores/{store}/projects/{project}/transfer`(本文 `{toStore, toWorkspace, statusChoices}` → 200 `{toStore, projectId, display, destination}`。`destination` は {移動先} の文)。送れなかったときは 422、取り消しもできなかったときは 409(A5a-19 と同じ)。`GET locate-project?store=&id=` → `{kind: "found", storeId, projectId, movedFrom?}` か `{kind: "gone", …}`(A5a-19 と同じ形)か 404 | A5a-19 に合わせる |
| A5b-23 | 予告 `ProjectPreview`: `items`(削除されていないアイテムの数 N)・`deleted`(削除済みのアイテムの数 M)・`prefix`(移動先の接頭辞)・`source`・`destination`(A5b-10 の {移動元}・{移動先} の文)・`crossStore`・`remoteNotice`・`destinationHasRemote`・`hasRules`(プロジェクト別のルールか専用ルールがあるか)・`notCarried`(A5b-8)・`replacements`(A5a の `StatusRow` と同じ形。A5b-5)。空の配列は `[]` | 02 **移動** の 1〜3 つ目の点 |
| A5b-24 | 旧 ID の 1 行の持ち方: `resolveRoute` はプロジェクトの旧 ID の解決で `movedFrom` を添えて返し、`screen.ts` は、それを `Screen.projectMoved: Readable<{storeId, projectId, tab, moved: MovedFrom, text: string} \| null>`(`text` は解決したときの core の `now()` で作った `movedLine` の文)に置いてから、アドレスを新しいプロジェクトのもの(同じタブ)に替える(履歴に積まない)。`current` の `Resolved` には `movedFrom` を入れない(同じ画面の判定 `sameScreen` を変えないため)。`projectMoved` は、そのあと出す画面が別のプロジェクトか別のタブになったら null にする。`ProjectScreen.svelte` は、出しているプロジェクトとタブが `projectMoved` と同じときだけ、その `text` を `<p>` で出す | A5b-2。アドレスを替えると解決し直すので、route に持つと消える(アイテムは `movedNotice` に持つのと同じ考え) |

---

## ファイルの地図

| 場所 | 新規・変更 | 役割 |
|---|---|---|
| `internal/domain/transfer/project.go` | 新規 | `ProjectSet`・`CollectProject`・`NumberProject` |
| `internal/domain/transfer/rules.go` | 新規 | `CarryInput`・`Carried`・`NotCarried`・`CarryRules`・`RuleStatuses` |
| `internal/domain/transfer/people.go` | 変更 | `People` に `extra []string` |
| `internal/app/port/stores.go` | 変更 | `ProjectWriter.Import`・`Remove` |
| `internal/infra/gitstore/write.go`・`internal/app/apptest/stores.go`・`sync.go`・`stores_contract_*.go` | 変更 | 上の口の実装(`Import`・`Remove` を `idWriter` に移して共有。新しいファイルを足さない) |
| `internal/app/transfers/project_targets.go`・`project_preview.go`・`project_move.go`・`project_move_store.go`・`project_undo.go`・`project_locate.go` | 新規 | 候補・予告・同じ Store の移動・別の Store の移動・戻し・旧 ID |
| `internal/app/transfers/service.go`・`move_store.go`・`locate_read.go` | 変更 | `Service.IDs`、`People` の呼び出し、スナップショットにプロジェクト |
| `internal/domain/history/project.go`・`internal/app/history/service.go` | 変更 | 「{ワークスペース名} から移動」 |
| `internal/adapter/httpapi/transfers/project.go` | 新規 | 4 つの API |
| `cmd/hedgehug/wire.go`・`wire_e2e.go` | 変更 | `IDs` の配線 |
| `web/packages/core/src/model/transfer.ts`・`port/transfers.ts`・`web/packages/api/src/transfers.ts` | 変更 | 型・口・api の実装 |
| `web/packages/core/src/features/transfers/projectDialog.ts`・`projectDialogText.ts` | 新規 | 区画と確認ダイアログのモデル・文 |
| `web/packages/core/src/features/router/resolve.ts`・`features/screen/screen.ts` | 変更 | プロジェクトの旧 ID、`projectMoved`(A5b-24) |
| `web/app/src/screens/project/ProjectTransfer.svelte`・`ProjectTransferDialog.svelte`・`ProjectSettings.svelte`・`ProjectScreen.svelte`・`web/app/src/wireActions.ts` | 新規・変更 | 区画・ダイアログ・見出しの下の 1 行 |
| `web/e2e/project-transfer.spec.ts` | 新規 | 通しのテスト |

## タスクの一覧

| # | タスク | 待つもの |
|---|---|---|
| 1 | `domain/transfer`: `CollectProject`・`NumberProject`・`People` の `extra` | — |
| 2 | `domain/transfer`: `CarryRules`・`RuleStatuses` | 1(並べてよい) |
| 3 | 口と保存: `ProjectWriter.Import`・`Remove` | — |
| 4 | `app/transfers`: `ProjectTargets`・`ProjectPreview` | 1・2 |
| 5 | `app/transfers`: 同じ Store の中のプロジェクトの移動 | 3・4 |
| 6 | `app/transfers`: 別の Store へのプロジェクトの移動と戻し | 5 |
| 7 | `LocateProject` と、プロジェクトの履歴の「{ワークスペース名} から移動」 | 6(テストで別の Store へ移すため) |
| 8 | API と配線 | 4〜7 |
| 9 | 画面の口と区画・ダイアログのモデル(core) | 8 |
| 10 | 区画・ダイアログの部品と、移したあとの移り先(app) | 9 |
| 11 | プロジェクトの旧 ID のアドレスと見出しの下の 1 行 | 9 |
| 12 | 通しのテスト(e2e)・仕様の文言・PR | すべて |

## タスク間の名前

### Go

| 名前 | 置き場所 | 形 |
|---|---|---|
| `transfer.ProjectSet`・`CollectProject` | `internal/domain/transfer/project.go` | `type ProjectSet struct { Project project.ID; Epics []Set; Outside []Cut }`(`Epics` は A5b-3 の並び。各 `Set` の `Outside` は空で、外との関連は `ProjectSet.Outside` にまとめる)、`func (s ProjectSet) Items() []item.Item`(Epic ごとに Root → Below の順)、`func (s ProjectSet) Contains(id item.ID) bool`、`func CollectProject(p project.ID, all []item.Item) ProjectSet` |
| `transfer.NumberProject` | 同上 | `func NumberProject(s ProjectSet, next int) map[item.ID]int` |
| `transfer.People` | `internal/domain/transfer/people.go` | `func People(from workspace.Settings, items []item.Item, logs [][]change.Record, extra []string, fromMembers, toMembers []member.Member) ([]member.Member, PeopleMap)` |
| `transfer.CarryInput`・`Carried`・`NotCarried`・`CarryRules` | `internal/domain/transfer/rules.go` | `type CarryInput struct { Project project.ID; From, To workspace.Settings; Rules projectrule.Rules; Own []automation.Rule; StatusChoice map[change.ID]change.ID; People PeopleMap; NewID func() change.ID }`(`Own` はこのプロジェクトの専用ルール、`People == nil` は同じ Store)、`type NotCarried struct { Kind, Name string; Missing []string; Count int }`、`type Carried struct { Source, Destination []change.Draft; Created []automation.Rule; NotCarried []NotCarried }`(`Source` は移動元のワークスペースのログ、`Destination` は移動先のワークスペースのログに書く行、`Created` は移動先で作る専用ルール)、`func CarryRules(in CarryInput) Carried` |
| `transfer.RuleStatuses` | 同上 | `func RuleStatuses(own []automation.Rule) []change.ID`(専用ルールが使うステータス。重複なし) |
| `port.ProjectWriter.Import`・`Remove` | `internal/app/port/stores.go` | `Import(id project.ID, log []change.Record, description string) error`、`Remove(id project.ID) error`(`ItemWriter` と同じ約束) |
| `transfers.Service.IDs` | `internal/app/transfers/service.go` | `IDs port.IDs`(専用ルールの新しい内部 ID) |
| `transfers.ProjectTarget`・`(Service) ProjectTargets` | `internal/app/transfers/project_targets.go` | `type ProjectTarget struct { Store port.StoreInfo; Workspace workspace.Workspace }`、`ProjectTargets(ctx, store port.StoreID, id project.ID) ([]ProjectTarget, error)` |
| `transfers.ProjectMoveInput`・`ProjectPreview`・`(Service) ProjectPreview` | `internal/app/transfers/project_preview.go` | `type ProjectMoveInput struct { ToStore port.StoreID; ToWorkspace workspace.ID; StatusChoices map[change.ID]change.ID }`、`type ProjectPreview struct { Items, Deleted int; Prefix, Source, Destination string; CrossStore, RemoteNotice, DestinationHasRemote, HasRules bool; NotCarried []transfer.NotCarried; Replacements []StatusRow }`、`ProjectPreview(ctx, store, id, in ProjectMoveInput) (ProjectPreview, error)` |
| `transfers.ProjectMoved`・`(Service) MoveProject` | `internal/app/transfers/project_move.go`(同じ Store)・`project_move_store.go`(別の Store)・`project_undo.go`(戻し) | `type ProjectMoved struct { ToStore port.StoreID; Project project.ID; Display, Destination string }`、`MoveProject(ctx, store port.StoreID, id project.ID, in ProjectMoveInput) (ProjectMoved, error)`。失敗は A5a の `*SendError`・`*UndoError` をそのまま使う |
| `transfers.LocatedProject`・`(Service) LocateProject` | `internal/app/transfers/project_locate.go` | `type LocatedProject struct { Found bool; Store port.StoreID; Project project.ID; MovedFrom *MovedFrom; Gone *Gone }`、`LocateProject(ctx, storeName, display string) (LocatedProject, error)` |
| `history.ProjectContext.WorkspaceName` | `internal/domain/history/project.go` | `WorkspaceName func(workspace.ID) string`(ないものは空) |

### HTTP API

| operationId | メソッドとパス | 本文 → 応答 | タスク |
|---|---|---|---|
| `list-project-transfer-targets` | GET `/api/stores/{store}/projects/{project}/transfer-targets` | → `{storeId, storeName, workspaceId, workspaceName}[]` | 8 |
| `get-project-transfer-preview` | GET `/api/stores/{store}/projects/{project}/transfer-preview` | query `toStore, toWorkspace` → A5b-23 | 8 |
| `transfer-project` | POST `/api/stores/{store}/projects/{project}/transfer` | `{toStore, toWorkspace, statusChoices}` → 200 `{toStore, projectId, display, destination}`。422・409 は A5a-19 と同じ。断りは既存の `apierr` の形 | 8 |
| `locate-project` | GET `/api/locate-project` | query `store, id` → `{kind: "found", storeId, projectId, movedFrom?}` か `{kind: "gone", reason, old, at, newDisplay, storeName?}` か 404 | 8 |

### 画面

| 名前 | 置き場所 | 形 |
|---|---|---|
| `ProjectTransferTarget`・`ProjectTransferPreview`・`NotCarried`・`ProjectTransferInput`・`ProjectTransferResult`・`LocatedProject` | `web/packages/core/src/model/transfer.ts` | API の形のまま |
| `TransfersPort` に足すもの | `web/packages/core/src/port/transfers.ts` | `projectTargets(storeId, projectId)`・`projectPreview(storeId, projectId, to: {storeId, workspaceId})`・`transferProject(storeId, projectId, input)`・`locateProject(storeName, id)` |
| `createProjectTransfer` | `web/packages/core/src/features/transfers/projectDialog.ts` | 区画(候補・選んだ移動先)と確認ダイアログ(予告・置き換えの選択・送信)。Task 9 |
| `projectTransferIntro`・`notCarriedLine`・`projectMovedMessage` | `web/packages/core/src/features/transfers/projectDialogText.ts` | Task 9 |
| `projectTransfer` | `web/app/src/wireActions.ts` | `createProjectTransfer` の組み立て |
| `resolveRoute` のプロジェクトの `movedFrom`・`Screen.projectMoved` | `web/packages/core/src/features/router/resolve.ts`・`features/screen/screen.ts` | `{ name: "project"; storeId; projectId; tab; movedFrom?: MovedFrom }`(`resolveRoute` の戻り値だけ。`current` には入れない)、`projectMoved: Readable<{ storeId: string; projectId: string; tab: ProjectTab; moved: MovedFrom } \| null>` |

---

### Task 1: `domain/transfer`: `CollectProject`・`NumberProject`・`People` の `extra`

**Files:**
- Create: `internal/domain/transfer/project.go`
- Modify: `internal/domain/transfer/people.go`、`internal/app/transfers/move_store.go`(`People` の呼び出しに `nil`)
- Test: `internal/domain/transfer/project_test.go`(新規)、`people_test.go`

**Interfaces:**
- Consumes: A5a の `transfer.Set`・`Cut`・`Collect`・`Number`、`item.SortSiblings`
- Produces: 「タスク間の名前」の `ProjectSet`・`CollectProject`・`NumberProject`・`People`

- [ ] **Step 1: 失敗するテストを書く**

`project_test.go`:

- `TestCollectProjectOrdersEpicsLiveThenDeleted`: Epic E1(順位 b)・E2(順位 a)・削除済みの E3(順位 0)と、E2 の下の Task T1・削除済みの T2、E1 の下の Task T3。`Items()` の ID が `[E2, T1, T2, E1, T3, E3]`。
- `TestCollectProjectCutsOnlyOutside`: 集まりの T1 → 別のプロジェクトの X への blocks、X → T1 への relates、T1 → T3 への blocks。`Outside` が `{T1→X blocks}`・`{X→T1 relates}` の 2 つで、T1 → T3 は入らない。`all` にいない相手への関連は数えない。
- `TestCollectProjectWithoutItems`: アイテムが 0 件なら `Epics` は空、`Items()` は空、`Outside` は空。
- `TestNumberProjectChainsEpics`: 上の 1 つ目の集まりで `NumberProject(s, 13)` が `E2:13, T1:14, T2:15, E1:16, T3:17, E3:18`。

`people_test.go` に `TestPeopleCountsExtra`: `extra = ["Hanako", "@none", ""]` で、移動先にいない `hanako` が足す人に入り、`@none` と空は数えない(`member.ValidLogin` に合わない)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/transfer/`
Expected: FAIL(`CollectProject` が未定義、`People` の引数の数が違う)

- [ ] **Step 3: 実装する**

`CollectProject` は、プロジェクト `p` のアイテムのうち親のないもの(Epic)を、A5b-3 の順に並べ、それぞれ `Collect(epic.ID, all)` で集め、`Outside` を集まり全体で数え直す(Epic ごとの `Outside` には、ほかの Epic の下のアイテムが「外」として入るので使わない)。`NumberProject` は A5b-4。`People` は `mentioned` で `extra` も数える。

- [ ] **Step 4: 通す**

Run: `go test ./internal/domain/transfer/ ./internal/app/transfers/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/transfer internal/app/transfers/move_store.go
git commit -m "feat: プロジェクトの下のすべてのアイテムを移す集まりと番号の規則を足す"
```

---

### Task 2: `domain/transfer`: `CarryRules`・`RuleStatuses`

**Files:**
- Create: `internal/domain/transfer/rules.go`
- Test: `internal/domain/transfer/rules_test.go`(新規)

**Interfaces:**
- Consumes: `projectrule.Rules`・`HiddenField`・`WorkflowField`・`TemplatesField`・`TemplateDefaultField`・`ResetDrafts`・`None`、`automation.Rule`・`Drafts`・`DeleteDraft`・`Statuses`・`Unassigned`・`ActionStatus`・`ActionAssignee`、`workspace.Settings`
- Produces: 「タスク間の名前」の `CarryInput`・`Carried`・`NotCarried`・`CarryRules`・`RuleStatuses`

- [ ] **Step 1: 失敗するテストを書く**

移動元のワークスペースのステータス `Todo`・`InProgress`・`Review`・`Pending`・`Done`、移動先は `Todo`・`InProgress`・`Done`(名前は同じで内部 ID は違う)。テンプレートは移動元に `Bug 報告`(Bug)・`仕様`(Story)、移動先に `仕様`(Story)だけ。

- `TestCarryRulesHiddenByName`: 使わない `Pending`・`InProgress` → 移動先に `add hidden_statuses <移動先の InProgress>` の 1 行。`NotCarried` に `{hidden, "Pending"}`。
- `TestCarryRulesWorkflowNeedsBothEnds`: `InProgress → Done`・`InProgress → Review`・`Review → Pending` → 移動先は `InProgress → Done` だけ。`NotCarried` は `{workflow, "InProgress から Review", ["Review"]}`・`{workflow, "Review から Pending", ["Review", "Pending"]}`。
- `TestCarryRulesTemplatesByNameAndKind`: 使う `Bug 報告`・`仕様` → 移動先に `仕様` の `add`。`Bug 報告` は `{template, "Bug 報告"}`。使うものが 1 件も合わないときは移動先に `templates` の行を書かない。
- `TestCarryRulesTemplateDefaults`: Bug の既定 `Bug 報告`・Story の既定 `仕様`・Task の既定 `projectrule.None` → 移動先は Story `仕様` と Task `None` の `set`。Bug は `{templateDefault, "Bug", ["Bug 報告"]}`。
- `TestCarryRulesDisabledIsNotCarried`: 動かさない指定 2 件 → 移動先に行なし、`{disabled, Count: 2}`。0 件なら `disabled` の行を出さない。
- `TestCarryRulesOwnRulesGetNewIDsAndStatuses`: 専用ルール「レビュー待ち」(きっかけ: ステータスが `Review`、すること: ステータスを `Done`)、`StatusChoice{Review: 移動先の InProgress}` → `Created` は新しい ID(`NewID` の値)・`Scope` は同じプロジェクト・きっかけ `移動先の InProgress`・値 `移動先の Done`。`Destination` に `automation.Drafts(automation.Rule{}, created)` の行。選択がなければ同じ分類の移動先の既定。
- `TestCarryRulesOwnRuleAssigneeAcrossStores`: することが担当者 `Taro` で `People{"taro": "TARO"}` → 値は `TARO`。`automation.Unassigned` は変えない。`People == nil` なら変えない。
- `TestCarryRulesSourceRemovesEverything`: `Source` が `projectrule.ResetDrafts` の 4 区画の行と、専用ルールごとの `automation.DeleteDraft`。
- `TestCarryRulesHiddenThatCannotBeHidden`: 使わない `Todo` の名前が移動先の初期ステータス、`Done` の名前が移動先の「完了」の既定 → どちらも移動先に書かず、`NotCarried` に `{hiddenFixed, "Todo"}`・`{hiddenFixed, "Done"}`。
- `TestCarryRulesKeepsRuleWhenStatusesCollapse`: きっかけ `Review`・すること `Pending` の専用ルールで、両方を移動先の `InProgress` に置き換える(きっかけとすることが同じになる)→ ルールは削除も無効化もせず、そのまま作る(03「ルールは、削除も無効化もしない」。ステータスの削除の追従が無効にするのとは違う)。
- `TestRuleStatuses`: きっかけとすることのステータスを重複なく返す。担当者のルールの値は入らない。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/transfer/ -run 'CarryRules|RuleStatuses'`
Expected: FAIL(未定義)

- [ ] **Step 3: 実装する**

`CarryRules` は A5b-6〜A5b-8。ステータスの名前の対応は、A5a の `remap.go` の名前で引く関数を使う(新しく書かない)。

- [ ] **Step 4: 通す**

Run: `go test ./internal/domain/transfer/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/transfer
git commit -m "feat: プロジェクトの移動でプロジェクト別のルールと専用ルールを移動先の名前に合わせる規則を足す"
```

---

### Task 3: 口と保存: `ProjectWriter.Import`・`Remove`

**Files:**
- Modify: `internal/app/port/stores.go`、`internal/infra/gitstore/write.go`、`internal/app/apptest/stores.go`・`sync.go`、`internal/app/apptest/stores_contract.go`(268 行。いまの `contractImport` とアイテムの `Import` の断りのケースを、種類(アイテム・プロジェクト)で回す形にして、行を増やしすぎない。300 行を超えるなら、まとまりを同じパッケージの既存の `stores_contract_cases.go` へ移す)
- Test: 照合テスト(gitstore と偽物の両方で流れる)

**Interfaces:**
- Produces: `ProjectWriter.Import(id project.ID, log []change.Record, description string) error`(フォルダがあれば・同じ Tx で消したものなら `ErrInvalidOperation`)、`ProjectWriter.Remove(id project.ID) error`(なければ `ErrNotFound`)

- [ ] **Step 1: 失敗する照合テストを書く**

- `ImportProject`: 別のプロジェクトのログと説明を `Import` すると、コミットのあと `Projects().Get` と `Description` で読め、`project.yaml` が作り直される。同じ ID がすでにあれば `ErrInvalidOperation`。
- `ImportProjectThenAppend`: 写したプロジェクトに、同じ Tx で `moved_in`・`set place` の行を足せる(gitstore と偽物の両方)。
- `RemoveProject`: `Remove` すると、コミットのあと `Projects().Get` が `ErrNotFound`、`List` に出ない。下のアイテムのフォルダは消さない(呼ぶ側がアイテムごとに消す)。ないものは `ErrNotFound`。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/infra/gitstore/ ./internal/app/apptest/`
Expected: FAIL(`Import`・`Remove` がない)

- [ ] **Step 3: 実装する**

gitstore と偽物の両方で、`itemWriter` の `Import`・`Remove` を `idWriter` に移して種類(`w.kind`)で参照を作る。`places` を消すのはアイテムのときだけ。`ProjectWriter` を返す口(`WriteProjects`)を `idWriter` を埋め込む型にする。`apptest/sync.go` の行の印を付ける包み(`rowsItems`)と同じものをプロジェクトにも付ける。コミットのときのフォルダの作成と削除が、プロジェクトの参照でも働くことを照合テストで確かめる(`projects/<ID>/`)。新しいファイルは足さない。

- [ ] **Step 4: 通す**

Run: `go test ./internal/infra/gitstore/ ./internal/app/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/port internal/infra/gitstore internal/app/apptest
git commit -m "feat: プロジェクトのフォルダを写す口と消す口を足す"
```

---

### Task 4: `app/transfers`: `ProjectTargets`・`ProjectPreview`

**Files:**
- Create: `internal/app/transfers/project_targets.go`・`project_preview.go`
- Test: `internal/app/transfers/project_targets_test.go`・`project_preview_test.go`(新規)

**Interfaces:**
- Consumes: Task 1・2、A5a の `storeInfo`・`checkDestination`・`hasRemote`・`StatusRow`、`port.LiveWorkspaces`・`LiveWorkspace`・`EditableProject`、`automation.FromLog`、`projectrule.FromLog`
- Produces: 「タスク間の名前」の `ProjectTarget`・`ProjectTargets`・`ProjectMoveInput`・`ProjectPreview`

- [ ] **Step 1: 失敗するテストを書く**

偽物は A5a のテストの組み立て(`helpers_test.go`)を使う。

- `TestProjectTargetsIncludeWorkspacesWithoutProjects`: 同じ Store のプロジェクトのないワークスペースが候補に入る。自分のワークスペース・削除済みのワークスペース・閲覧のみの Store・非表示の Store は入らない。並びは A5a-13。
- `TestProjectTargetsRefuseUnknownProject`: ない・削除済み(root: false)のプロジェクトは `port.ErrNotFound`。
- `TestProjectPreviewCounts`: 削除されていないアイテム 3 件・削除済み 1 件 → `Items: 3, Deleted: 1`、`Prefix` は移動先の接頭辞、`Source`・`Destination` はワークスペース名(別の Store なら「{Store 名} の {ワークスペース名}」)。
- `TestProjectPreviewReplacements`: アイテムが `Review`(2 件、うち 1 件は削除済み)と `Todo`、専用ルールが `Pending` を使い、移動先に `Review`・`Pending` がない → 行は移動元の並びで `Review`(2)・`Pending`(0)。選択肢は移動先のすべてのステータス、初めの値は同じ分類の既定。
- `TestProjectPreviewKeepsHiddenStatusThatExists`: プロジェクトが `InProgress` を使わないにし、アイテムが `InProgress` を持ち、移動先に `InProgress` がある → 置き換えの行なし(Review Focus 3)。
- `TestProjectPreviewRules`: ルールがなければ `HasRules: false`・`NotCarried` は空。あれば `HasRules: true` と `CarryRules` の `NotCarried`。
- `TestProjectPreviewRemote`: A5a の `TestPreviewRemote…` と同じ組み合わせで `RemoteNotice`・`DestinationHasRemote`。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/transfers/ -run 'ProjectTargets|ProjectPreview'`
Expected: FAIL(未定義)

- [ ] **Step 3: 実装する**

移動元を読む関数 `readProjectSource(r port.Reader, id project.ID, get func(port.Reader, project.ID) (project.Project, error)) (projectSource, error)` を `project_preview.go` に置き、Task 5・6 も使う(中身: プロジェクト、ワークスペースと設定、`CollectProject` の集まり、プロジェクト別のルール、専用ルール)。移動先は `port.LiveWorkspace` と設定・ログ。移動元と移動先は 1 つずつ順に読む(読み取りを入れ子にしない)。移動先が移動元と同じワークスペースなら `port.ErrNotFound`。

- [ ] **Step 4: 通す**

Run: `go test ./internal/app/transfers/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/transfers
git commit -m "feat: プロジェクトの移動先の候補と確認ダイアログの予告を足す"
```

---

### Task 5: `app/transfers`: 同じ Store の中のプロジェクトの移動

**Files:**
- Create: `internal/app/transfers/project_move.go`
- Modify: `internal/app/transfers/service.go`(`IDs`)
- Test: `internal/app/transfers/project_move_test.go`(新規)

**Interfaces:**
- Consumes: Task 1・2・4、A5a の `numberLines`・`emptiedOf` と同じ形の行の作り方、`port.ReviveProject`・`NoteRevived`、`numbering.Next`・`Draft`、`rank.After`
- Produces: 「タスク間の名前」の `ProjectMoved`・`MoveProject`。移動先が別の Store のときは、Task 6 が `moveProjectAcross` を足すまで `errors.ErrUnsupported` を返す(Task 5 のテストは同じ Store だけを流す)

- [ ] **Step 1: 失敗するテストを書く**

- `TestMoveProjectWithinStoreWritesOneSave`: 保存は 1 回。プロジェクトに `set place {移動先, 次の番号}` と `set rank`(移動先の末尾)、アイテムごとに `set place`(ワークスペースと番号だけが変わる)、移動先のワークスペースに `number` の行(`projects` 1 行・`items` の件数分)、転送記録がプロジェクト 1 行(`entity: project`、`old: HH-P1`、`new: MB-P2`、`to_store` は自分)とアイテムごと。アクティビティの文は「プロジェクト「Web」を Web から Mobile へ移動」。結果は `{Display: "MB-P2", Destination: "Mobile"}`。
- `TestMoveProjectRemapsValues`: 優先度の名前が移動先にないアイテムは初期優先度、ステータスは `StatusChoices` の値。
- `TestMoveProjectCarriesRules`: 移動元のワークスペースのログからプロジェクト別のルールと専用ルールが消え(`projectrule.FromLog(...).Of(p)` が空、`automation.FromLog` に専用ルールがない)、移動先に合うものだけがある。専用ルールの ID は新しい。
- `TestMoveProjectRefusesSameNameProject`: 移動先に同じ名前のプロジェクト → 「Mobile には同じ名前のプロジェクトがあります」。削除済みの同じ名前は断らない。
- `TestMoveProjectRefusesSameNameRule`: 移動先に専用ルールと同じ名前のルール → 「Mobile には同じ名前のルールがあります: レビュー待ち。先にルールの名前を変えてください」。
- `TestMoveProjectWithoutItems`: アイテム 0 件のプロジェクトを移せる。`number` の行は `projects` の 1 行だけ。
- `TestMoveProjectRevivesDeletedElsewhere`: 別の画面で削除したプロジェクト(root: true)を移すと、プロジェクトと下のアイテムの削除が取り消され、`port.WithRevived` に旧 ID が積まれる。
- `TestMoveProjectRefusals`: 閲覧のみの Store → 「閲覧のみの Store です」。移動先が同じワークスペース・削除済みのワークスペース・閲覧のみの Store → `port.ErrNotFound`。表示中でない Store → `port.ErrStoreNotFound`。
- `TestMoveProjectRoundTripCarriesRulesBack`: Web → Mobile → Web と移すと、使わないステータス・遷移のチェック・使うテンプレートが Web に戻り(前の移動の `remove` のあとの `add` が効く)、専用ルールも Web に(3 つ目の新しい ID で)戻って削除済みと読まれない。
- `TestMoveProjectKeepsDeletedItemsDeleted`: 削除済みの Epic と下のものが、削除済みのまま、元の削除の操作のまま移り、番号は削除されていない Epic の下のあと。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/transfers/ -run MoveProject`
Expected: FAIL(未定義)

- [ ] **Step 3: 実装する**

`MoveProject` は A5a の `Move` と同じ形: 移動元の Store の確認 → 移動先の確認 → 別の Store なら Task 6 へ → 保存の前に読んでアクティビティの文を決める → `Write` の中で `port.ReviveProject` → 読み直して検査(A5b-9)→ 行を書く(A5b-11)。行を作る `projectPlan`(移動元・移動先・番号・`Remap`・`CarryRules` の結果)は、Task 6 も使うので `project_move.go` に置く。

- [ ] **Step 4: 通す**

Run: `go test ./internal/app/transfers/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/transfers
git commit -m "feat: プロジェクトを同じ Store の別のワークスペースへ移す"
```

---

### Task 6: `app/transfers`: 別の Store へのプロジェクトの移動と戻し

**Files:**
- Create: `internal/app/transfers/project_move_store.go`・`project_undo.go`
- Test: `internal/app/transfers/project_move_store_test.go`(新規)

**Interfaces:**
- Consumes: Task 3・5、A5a の `Delivery`・`SendError`・`UndoError`・`errSourceChanged`・`earlierMoves`・`vocabOf`・`namesOf`・`joinedLater`、`port.ErrSaveNotDiscarded`
- Produces: `moveProjectAcross`(`MoveProject` から呼ぶ)

- [ ] **Step 1: 失敗するテストを書く**

A5a の `move_store_test.go` の偽の `Delivery` と Store ごとのロックを使う。

- `TestMoveProjectAcrossWritesBothStores`: 移動先に、プロジェクトとアイテムのフォルダ(行はそのまま + `moved_in`・`set place`)、`number` の行、ルールの行、メンバーの `join`。移動元は、フォルダが消え、ルールの行が消え、転送記録がプロジェクトとアイテムごと(`to_store_name`・`to_remote` 入り、`at` は移動先の `moved_in` と同じ)。アクティビティは両方「プロジェクト「Web」を e2e の Web から team の Team へ移動」。
- `TestMoveProjectAcrossCutsOutsideRelations`: 移動元の別のプロジェクトのアイテムとの関連が両方の側から消え、プロジェクトの中の関連は残る。
- `TestMoveProjectAcrossAddsPeople`: オーナー・専用ルールの担当者・プロジェクトのコメントの書き手・アイテムの担当者のうち移動先にいない人が `join` され、オーナーが綴りの違う人なら `set owner` で移動先の綴りになる。
- `TestMoveProjectAcrossReadsTxOnRetry`: 偽の `Delivery` が `fn` を 2 回呼ぶ。2 回目の前に移動先へ同じ名前のプロジェクトを足すと断る(Review Focus 1)。別の場合として、2 回目の前に移動先の番号を進めると、2 回目の番号で書く。
- `TestMoveProjectAcrossSendFails`: 送れない → `*SendError`、移動元は変わらない。
- `TestMoveProjectAcrossRollsBack`: 移動元の保存が失敗 → 移動先からフォルダ・足したメンバー・ルールの行・作った専用ルールが消える(`projectrule.FromLog(...).Of(p)` が空、`automation.FromLog` にない)。アクティビティは「MB-P2 の移動を取り消し」。戻しも失敗 → `*UndoError{Old: "HH-P1", NewDisplay: "TM-P2"}`。
- `TestMoveProjectAcrossSourceChanged`: `prepare` のあと移動元のプロジェクトにアイテムを足すと、移動元の保存が失敗して戻す。
- `TestMoveProjectAcrossRulesChanged`: `prepare` のあと移動元のワークスペースにこのプロジェクトの専用ルールを足すと、移動元の保存が失敗して戻す(移動元にルールが残り、移動先にはない)。
- `TestMoveProjectAcrossRevives`: 別の画面で削除されたプロジェクトは、移動先で削除が取り消され、両方の保存のあとで旧 ID が積まれる。
- `TestMoveProjectAcrossOppositeDirections`: A → B と B → A のプロジェクトの移動を同時に流しても止まらない(A5a の Review Focus 2 のテストと同じ組み立て)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/transfers/ -run MoveProjectAcross`
Expected: FAIL

- [ ] **Step 3: 実装する**

A5a の `crossMove` と同じ形の `crossProjectMove`(A5b-12・A5b-14)。`fn` の中で、移動先の同じ名前の断り・番号・メンバー・ルールの対応を `Tx` から読み直す。移動元の保存の直前に `CollectProject` で集め直し、アイテムの ID の集まりが違えば `errSourceChanged`。戻しの判定は、移動先のプロジェクトのログの最後の `moved_in` の操作。

- [ ] **Step 4: 通す**

Run: `go test ./internal/app/transfers/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/transfers
git commit -m "feat: プロジェクトを別の Store のワークスペースへ移し、移動元の保存が失敗したら戻す"
```

---

### Task 7: `LocateProject` と、プロジェクトの履歴の「{ワークスペース名} から移動」

**Files:**
- Create: `internal/app/transfers/project_locate.go`
- Modify: `internal/app/transfers/locate_read.go`(スナップショットにプロジェクト)、`internal/domain/history/project.go`、`internal/app/history/service.go`
- Test: `internal/app/transfers/project_locate_test.go`(新規)、`internal/domain/history/project_test.go`

**Interfaces:**
- Consumes: A5a の `snapshot`・`whyGone`・`storeName`・`splitNamed`、Task 5・6(Task 6 のあとに始める)
- Produces: `LocatedProject`・`LocateProject`、`ProjectContext.WorkspaceName`

- [ ] **Step 1: 失敗するテストを書く**

- `TestLocateProjectCurrentAndOldPrefix`: いまの ID・旧い接頭辞の ID で開く(`MovedFrom` なし)。
- `TestLocateProjectAfterMove`: 同じ Store で HH-P1 → MB-P2 と移したあと、`HH-P1` が MB-P2 を開き `MovedFrom{Old: "HH-P1", Display: "MB-P2", Current: "MB-P2"}`。さらに別の Store の TM-P3 へ移すと、`HH-P1` が TM-P3 を開き `Current: "TM-P3"`。
- `TestLocateProjectGone`: 移したあと削除 → `deleted`、完全に削除(`AddPurged` の `entity: project`)→ `purged`、移動先の Store が非表示 → `hidden`。
- `TestLocateProjectIgnoresItemRecords`: アイテムの記録の `old` に同じ文字列があっても、プロジェクトの記録だけを見る。`HH-12` のようなアイテムの形の ID は 404(`port.ErrNotFound`)。
- `TestProjectRowsMoved`(domain/history): 同じ Store の `set place`(ワークスペースが変わる)が「Web から移動」、`moved_in` が「Web から移動」(同じ操作の `set place` は出さない)、ワークスペースがないときは「(削除された値) から移動」。番号だけが変わる `set place` は出さない。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/transfers/ ./internal/domain/history/ ./internal/app/history/`
Expected: FAIL

- [ ] **Step 3: 実装する**

`LocateProject` は A5b-17。`parseDisplay` と同じ形で `{接頭辞}-P{番号}` を読む関数を足す。スナップショットにはプロジェクト(削除済みを含む)とワークスペースの削除を足し、アイテムの解決の振る舞いは変えない。履歴は A5b-21。`app/history.Project` は、すべてのワークスペースの名前を引いて `WorkspaceName` に渡す。

- [ ] **Step 4: 通す**

Run: `go test ./internal/app/... ./internal/domain/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/transfers internal/domain/history internal/app/history
git commit -m "feat: プロジェクトの旧 ID の解決と、プロジェクトの履歴の移動の行を足す"
```

---

### Task 8: API と配線

**Files:**
- Create: `internal/adapter/httpapi/transfers/project.go`
- Modify: `internal/adapter/httpapi/transfers/transfers.go`(登録)、`cmd/hedgehug/wire.go`・`wire_e2e.go`(`IDs`)、`api/openapi.yaml`・`web/packages/api/src/generated.ts`(生成)
- Test: `internal/adapter/httpapi/transfers/project_test.go`(新規)

**Interfaces:**
- Consumes: Task 4〜7
- Produces: 「HTTP API」の 4 つ

- [ ] **Step 1: 失敗するテストを書く**

- `TestProjectTransferTargetsAPI`・`TestProjectTransferPreviewAPI`: 形と空の配列が `[]`。
- `TestTransferProjectAPI`: 200 の本文。断りは既存の断りの形。`*SendError` は 422 で `detail` が全文、`*UndoError` は 409。削除の取り消しは `Hedgehug-Restored`。
- `TestLocateProjectAPI`: `found`・`gone`・404。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/transfers/`
Expected: FAIL

- [ ] **Step 3: 実装して、生成する**

A5a の `move.go`・`locate.go` と同じ形。`Service.IDs` を配線する。

Run: `make generate && pnpm -C web gen`

- [ ] **Step 4: 通す**

Run: `go test ./internal/... ./cmd/...`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/adapter cmd api web/packages/api/src/generated.ts
git commit -m "feat: プロジェクトの移動と旧 ID の解決の API を足す"
```

---

### Task 9: 画面の口と区画・ダイアログのモデル(core)

**Files:**
- Modify: `web/packages/core/src/model/transfer.ts`・`port/transfers.ts`、`web/packages/api/src/transfers.ts`、偽物 `web/packages/core/src/features/transfers/transfers.test-util.ts`
- Create: `web/packages/core/src/features/transfers/projectDialog.ts`・`projectDialogText.ts`
- Test: `projectDialog.test.ts`・`projectDialogText.test.ts`(新規)

**Interfaces:**
- Consumes: Task 8 の API、A5a の `dialogText.ts` の `offlineMessage`・`transferFailedMessage`・`replacementsHeading`、`model/errors` の `Refused`・`NotFound`・`TransferUndoFailed`
- Produces: `createProjectTransfer(deps)` → `{ section: Readable<SectionState>; load(project); choose(key); open(); dialog: Readable<DialogState | null>; setChoice(status, to); submit(); cancel() }`。`deps` は `{ transfers, toasts, online: () => boolean, afterMoved(result, project) }`

- [ ] **Step 1: 失敗するテストを書く**

`projectDialogText.test.ts`:

- `projectTransferIntro`: 「「Web」を Web から Mobile へ移動します。3 件のアイテムは MB の番号になります。」、`deleted: 1` で「削除済みの 1 件も一緒に移ります。」を続ける。`crossStore` で「移さないアイテムとの関連を解除します。移すもの同士の関連は残ります。移動先のメンバーにいない人は、移動先のメンバーに加わります。」、`remoteNotice` で「移動先に remote がないため、ほかの人の Store からは見えなくなります。」、`hasRules` で「このプロジェクトのルールは、移動先の同じ名前のステータス・テンプレートに合わせて移ります。」。
- `notCarriedLine`: A5b-8 の 6 種類(`hiddenFixed` を含む)と、遷移のチェックの両方ないときの文。
- 置き換えの行の見出し: 件数があれば「Review(2 件)」、0 なら「Pending」。
- `projectMovedMessage("Mobile")` が「Mobile へ移動しました」。

`projectDialog.test.ts`:

- 候補がなければ区画は「移せるワークスペースがありません」で、`open` できない。選択肢は「{Store 名} / {ワークスペース名}」。
- `open` で予告を読み、ダイアログのタイトル「プロジェクトを移動」、本文・「移らないもの」・置き換えの行(初めの値は `default`)。
- `submit`: 送る本文に `statusChoices`、成功でトースト「Mobile へ移動しました」と `afterMoved`。断りはトーストに断りの文、ダイアログは開いたまま。オフライン(移動先に remote、`online()` が false)は送らずに `offlineMessage`。`NotFound`・`TransferUndoFailed` は閉じる。それ以外は「移動できませんでした」で開いたまま。
- 閲覧のみの Store では区画が `disabled`。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web test -- projectDialog`
Expected: FAIL

- [ ] **Step 3: 実装する**

A5a の `dialog.ts` と同じ作り(状態は `writable`、送信中は二重に送らない)。

- [ ] **Step 4: 通す**

Run: `pnpm -C web test && pnpm -C web check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages
git commit -m "feat: プロジェクトの移動の区画と確認ダイアログのモデルを足す"
```

---

### Task 10: 区画・ダイアログの部品と、移したあとの移り先(app)

**Files:**
- Create: `web/app/src/screens/project/ProjectTransfer.svelte`・`ProjectTransferDialog.svelte`
- Modify: `web/app/src/screens/project/ProjectSettings.svelte`(「自動化」と「削除」のあいだに区画を置く)、`web/app/src/wireActions.ts`、`web/app/src/App.svelte`(`DeleteProjectDialog` の隣にダイアログを置く)
- Test: `web/app` には部品のテストがない(A5a と同じ)。振る舞いは Task 9 のモデルのテストと Task 12 の e2e で確かめる

**Interfaces:**
- Consumes: Task 9 の `createProjectTransfer`、`leaveDeletedProject` と同じ移り方の部品(`navigation.replace`・`catalog.load`)
- Produces: `projectTransfer`(`wireActions.ts`)

- [ ] **Step 1: 部品を書く**

区画: 見出し「別のワークスペースへ移動」、説明文(A5b-18)、select「移動先のワークスペース」、button「移動」。閲覧のみは `disabled` と `title` に「閲覧のみの Store です」。ダイアログ: role `dialog` の名前「プロジェクトを移動」、本文の段落、見出し「移らないもの」と一覧、見出し「ステータスの置き換え」と select「{ステータス名} の置き換え先」、button「移動」「キャンセル」。`afterMoved` は A5b-19(新しいアドレスの設定タブへ `replace`、`catalog.load`)。

- [ ] **Step 2: 確かめる**

Run: `pnpm -C web test && pnpm -C web check`
Expected: PASS(型と既存のテスト)

- [ ] **Step 3: コミット**

```bash
git add web/app
git commit -m "feat: プロジェクトの設定に別のワークスペースへの移動の区画と確認ダイアログを足す"
```

---

### Task 11: プロジェクトの旧 ID のアドレスと見出しの下の 1 行

**Files:**
- Modify: `web/packages/core/src/features/router/resolve.ts`、`web/packages/core/src/features/screen/screen.ts`(`projectMoved`。A5b-19・A5b-24)、`web/app/src/wire.ts`(`screen` の組み立てに `locateProject`)、`web/app/src/screens/project/ProjectScreen.svelte`
- Test: `web/packages/core/src/features/router/resolve.test.ts`・`web/packages/core/src/features/screen/screen.test.ts

**Interfaces:**
- Consumes: `TransfersPort.locateProject`、A5a の `movedLine`・`goneText`
- Produces: `resolveRoute` のプロジェクトの結果の `movedFrom`、`Screen.projectMoved`(A5b-24)

- [ ] **Step 1: 失敗するテストを書く**

- カタログ(いまの接頭辞・旧い接頭辞)で見つかるプロジェクトのアドレスは、`locateProject` を呼ばない。
- 見つからなければ `locateProject` を呼び、`found` なら `{name: "project", storeId, projectId, tab: <アドレスのタブ>, movedFrom}`。`gone` は `goneText` のトーストとホーム。404 はホーム。
- `screen.test.ts`: 旧 ID のアドレスで開くと、`projectMoved` に `{storeId, projectId, tab: "tree", moved}` が入り、アドレスが新しいプロジェクトの同じタブに替わる(`replace`)。替えたあとの解決でも `projectMoved` は残り、タブを「概要」に変えると null になる。
- `screen.test.ts`: 出していたプロジェクトのアドレスを読み直したとき(一覧の読み直しで、そのプロジェクトがほかの書き手に移されて見つからない)は `locateProject` を呼ばず、いまの扱いどおりワークスペースの概要へ移る。
- `screen.test.ts`: 自分で移したあと(A5b-19 の順: 新しいアドレスへ `replace` してから一覧を読み直す)は、`projectMoved` が null のままで、ワークスペースの概要へも移らない。

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web test -- resolve`
Expected: FAIL

- [ ] **Step 3: 実装する**

A5b-24。`ProjectScreen.svelte` は、`projectMoved` が出しているプロジェクトとタブと同じときだけ、見出しの下・タブの上に `<p>{movedLine(m, now)}</p>` を出す(A5b-2)。

- [ ] **Step 4: 通す**

Run: `pnpm -C web test && pnpm -C web check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat: プロジェクトの旧 ID のアドレスから移したプロジェクトを開き、見出しの下に移動の 1 行を出す"
```

---

### Task 12: 通しのテスト(e2e)・仕様の文言・PR

**Files:**
- Create: `web/e2e/project-transfer.spec.ts`
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md`(A5b-2 の「プロジェクトの画面の見出しの下」と A5b-18 の文言)、この計画の末尾(「実装で計画から変えたこと」)

**増やす数: spec 1 ファイル・テスト 2 件(3 幅で 6 回)。** e2e のサーバーは worker ごとで、ほかの spec と Store を共有する。A5a の `transfer.spec.ts` はサンプルの「Mobile」の次の番号を `MB-2` と決め打ちしているので、**同じ Store の移動先には「Mobile」を使わず**、`createWorkspace` で移動元と移動先の 2 つのワークスペースを作って使う。別の Store の移動先は `team` の Store のワークスペース「Team」(A5a の spec は `TM-\d+` で受けるので重ならない)。

| テスト | 確かめること |
|---|---|
| 1 件目 | 移動元のワークスペース(接頭辞 `{prefix}`)にプロジェクト・Epic・Task を作り、設定の区画で「e2e / {移動先のワークスペース名}」を選び、確認ダイアログの本文を見て「移動」→ トースト「{移動先のワークスペース名} へ移動しました」、アドレスが `{移動先の接頭辞}-P1/settings` で、見出しの下に 1 行が出ない。続けて「team / Team」へ移す → トースト「team の Team へ移動しました」。旧 ID の `/s/e2e/p/{prefix}-P1/tree` がツリーのタブで開き、見出しの下に「{prefix}-P1 は … に {移動先の接頭辞}-P1 へ移動し、その後 TM-P… へ移動しました」。タブを「概要」に変えると消える。移したアイテムの旧 ID `/s/e2e/i/{prefix}-1` も開く |
| 2 件目 | 閲覧のみの Store のプロジェクトの設定で、区画のセレクトと「移動」が無効で「閲覧のみの Store です」が添う |

- [ ] **Step 1: 部品の名前を突き合わせる**

Run: `grep -rn "aria-label\|label=" web/app/src/screens/project/ProjectTransfer*.svelte`
Expected: 上の表の名前が見つかる。違うときはテストの側を直す。

- [ ] **Step 2: テストを書き、1 回流す**

Run: `make e2e-bin && pnpm -C web e2e project-transfer.spec.ts`
Expected: 6 件が通る。

- [ ] **Step 3: 仕様に文言を足す**

02「名前の変更・移動・削除」の **移動** に A5b-18 の文言と A5b-2 の場所を書き、差分を見せて承認を取る。

- [ ] **Step 4: 全体の検査**

Run: `make check`
Expected: 緑。

- [ ] **Step 5: 計画に実装で変えたことを足す**

この計画の末尾に「## 実装で計画から変えたこと」を足す(なければ「なし」)。

- [ ] **Step 6: コミットして PR を出す**

```bash
git add web docs
git commit -m "test: プロジェクトの別のワークスペース・別の Store への移動と旧 ID のアドレスの通しのテストを足す"
git push -u origin feat/track-a5b
gh pr create --base develop --head feat/track-a5b \
  --title "feat: トラック A5b(プロジェクトの別のワークスペース・別の Store への移動)" \
  --body-file <本文を書いたファイル>
```

本文は PR #205 の見出しに合わせる(「概要」「範囲」「ユーザーに決めてもらったこと」「仕様に足した文言」「計画から変えた主なもの」「develop の取り込み」「他のトラックへ渡すもの」「確かめたこと」)。末尾は `🤖 Generated with [Claude Code](https://claude.com/claude-code)`。「他のトラックへ渡すもの」には、A5c・E = `LocateProject` の規則、C4 = `locate-project`、F = `Remap` で外せばプロジェクトの移動も外れること、を書く。

## 実装で計画から変えたこと

- `transfer.Carried` に `Hidden []change.ID`(移動先で使わないにするステータスの内部 ID)を足し、置き換えの選択肢は `transfer.UsableStatuses(to, projectrule.Rules{Hidden: carried.Hidden})` で決める。A5b-5 の「引き継ぐ使わないステータス」を型で渡すため。
- 専用ルールの新しい内部 ID の `Service.IDs` は、`internal/adapter/httpapi/routes.go` で配線した。`httpapi.Deps` がすでに `IDs` を持っていて、`cmd/hedgehug/wire.go`・`wire_e2e.go` を変える必要がなかったため。
- 予告の空の `NotCarried`・`Replacements` は、app では nil のままにし、API の adapter で `[]` にする。
- プロジェクトの `moved_in` にも `Moves`(移動元の Store の中の前の移動)を入れ、履歴の前の移動の行のワークスペース名をそこから引く(A5b-21 を直した)。アイテム(A5a-2)と同じ規則で、入れないと前の移動が「(削除された値) から移動」になるため。
- プロジェクトの履歴のワークスペース名は、削除済み(論理削除)のワークスペースでも名前を出し、Store にないものだけ「(削除された値)」にする(A5b-21 を直した)。アイテムの履歴(`readOrigin`)と揃えるため。
- 画面がオフラインで送らない条件は、別の Store へ移し、移動先に remote があり、オフラインのとき(A5a と同じ)。同じ Store の移動はネットワークを使わないため。
- 「移らないもの」の `workflow` の行は、`Missing` が空のときの文を作らない。サーバーが必ず 1 つ以上返すため(A5b-8)。
- `createProjectTransfer` の deps に `catalog` を足した。閲覧のみの Store かを見るためと、なくなっていた・取り消しもできなかったときの一覧の読み直しのため。
- 移したあとの移り先は、「移動」を押したときからアドレスが変わっていなければ設定タブへ移り、変わっていれば一覧を読み直すだけにした(A5b-19 を直した)。A5a-18 と同じ規則。`createLeaveMoved` は包まずに `afterMoved` に渡す。送っているあいだに同じプロジェクトの別のタブへ移ったときも、同じ規則で移らない。
- `projectMoved` に `text`(解決したときの core の `now()` で作った 1 行の文)を持たせ、`ProjectScreen.svelte` はそれを出す(A5b-24 を直した)。部品が自分で時刻を取らず、ほかの時刻の文と同じ core の `now()` を使うため。
- `screen.ts` の deps の `transfers` は `Pick<TransfersPort, "locate" | "locateProject">` にした。
- プロジェクトのアドレスは、一覧(カタログ)ではアドレスの Store だけを、いまの接頭辞・旧い接頭辞で探し、なければサーバーの `locateProject` に聞く(A5a-15 の順で、ほかの Store まで探す)。一覧でほかの Store を探すと、別の Store へ移した直後に、ほかの Store の同じ ID のプロジェクトを一瞬開くため。サーバーがほかの Store で見つけたときは、アドレスをその Store の名前の形に替える(履歴に積まない)。替えないと、次の一覧の読み直しで、出していたプロジェクトが消えたと見られ、ワークスペースの概要へ移されるため。
- 契約テストの `checkRemoved` は `internal/app/apptest/stores_contract_rules.go` に置いた。apptest は 20 ファイルで新しいファイルを足せず、ほかの契約テストのファイルはどれも 300 行に近いため。
- e2e の移動先のワークスペースは、`createWorkspace` で作り、接頭辞は `prefixOf` の値の先頭の Z を Y に替えたものにした(`prefixOf` の値は Z で始まるので重ならない)。
- 実装で見つけて直した A5a の不具合(ユーザーの決定、2026-11-14):
  - 別の Store へのアイテムの移動で、別の画面で削除されたアイテムを移すと、同じ削除で消えた下のものが、削除済みとして番号を振られていた。同じ Store の移動(`revivedSet`)と同じく、削除を取り消したものとして番号を振る。
  - ワークスペースのアドレスで、ほかの Store のいまの接頭辞を、Store 名の Store の旧い接頭辞より先に見ていた。01「画面のアドレス」のとおり、Store 名の Store を、いまの接頭辞・旧い接頭辞の順に先に探す。03「接頭辞」の「旧い接頭辞をあとから別のワークスペースが使っているときは、いまの接頭辞のワークスペースを開く」は、同じ Store の中の規則だと書き足した。
