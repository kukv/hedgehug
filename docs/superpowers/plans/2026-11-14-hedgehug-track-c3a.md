# hedgehug トラック C3a(同期の修復: 枠組み・削除と変更の重なり・(1)〜(4)・(5)のプロジェクト違い)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 同期で合わせたあと、remote へ送る前に、仕様 06 の「同期の修復」のうち、削除と変更の重なり(削除の取り消し)、(1) 完全な削除と重なったとき(祖先・関連)、(2) 親が削除されたとき、(3) 番号の重なり、(4) 親の循環、(5) のうちプロジェクトだけが親と違う子を、実行者「同期」の保存として書き、どちらのマシンで同期しても同じ値になるようにする。

**Architecture:** 修復は保存形式「同期」の手順 4・5 のとおり、合わせたコミットのあと、push の前に、実行者 `sync` の別のコミットとして書く。`infra/gitstore` の `Replicator.Sync` が、合わせたあと(早送り・合わせのコミット・何もしないのどれでも)、Store のロックを持ったまま、(1) を自分で直し(フォルダの削除と `purged.jsonl` はファイルの扱いなので gitstore の中)、続けて `port.Remote.Repair` の関数を呼ぶ。その関数は `app/repairing.Run` で、`port.RepairWriter`(ロックを取り直さない読み書きと、共通の祖先から見てどちらの側で足された行かの `repair.Facts`)を受け取り、`domain/repair` の規則で直す内容を決めて、修復ごとに 1 回ずつ書く。`app/syncing` は `Remote.Repair` に `Service.Repair` を渡すだけで、cmd が `repairing.Run` をつなぐ。削除の取り消しは、新しい行の種類 `restore` と、`change.Undo`(操作 1 つの行を戻す変更)で書く。

**Tech Stack:** Go 1.27.3、go-git v5(gitstore)。画面の変更はない(他のタブとの重なりのトーストは C3a-screen)。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`「同時に編集したとき」の表の最後の行、「番号の重なりの修復」の最初の 2 段落、「親が循環したとき」、「親と別のワークスペース・プロジェクトにある子」の 2 つ目の箇条、「親が削除されたとき」、「完全な削除と重なったとき」、「修復の決まり」、「アクティビティ」の「対象」の表(実行者「同期」の文)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`「変更記録」(`restore`・`root`)、「同期」の手順 3〜6、「同期の修復」の表の (1)〜(5) と「順の指定なし: 削除と変更が重なった」の行、「削除・復元・完全に削除」、「移動・番号・旧 ID」の「使った最大番号」、「テスト」の「親と一緒の削除」
- 08「診断と修復」の表の「親がない」の行(親の循環の行き先)、「復元の単位」
- 04「ツリー」の「行」の「並び」(兄弟の並び。`item.SortSiblings`)
- `docs/superpowers/specs/2026-11-12-hedgehug-architecture-design.md` 2 節(依存の向き)・4 節(大きさの上限)
- `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(トラック C の小計画の一覧と漏れの表)、`2026-11-13-hedgehug-track-c2.md`(同期の実行。`syncing.Service.run`)、`2026-11-13-hedgehug-track-a4b.md`(削除の行。A4b-1・A4b-2)

## Global Constraints

土台とトラック C の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個。`infra/gitstore`・`app/apptest`・`app/port` は 20 個で満杯。gitstore は Task 5 で 2 つのファイルを既存のファイルへ移して空きを作る。apptest と port にはファイルを足さない(既存のファイルに書き足す。300 行を超えるなら、同じパッケージの短いファイルへ移す)。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(`go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 修復の保存は、`Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: …}`。`Summary` は下の「アクティビティの文」の表の文を、仕様の文のとおりに書く。修復 1 件(1 つの削除の取り消し、1 件の振り直し、1 件の移動)を 1 回の保存(1 コミット)にする。
- 修復は、どのマシンで同期しても同じ値になるよう、読む順・直す順を内部 ID の辞書順(番号を比べる規則では番号、同じなら内部 ID)で決める。マップの反復の順に頼らない。
- 修復を 2 回続けて行っても、2 回目は何も書かない(06「修復の決まり」の 3 つ目)。各 Task のテストに「もう一度 Run しても行が増えない」を入れる。
- 閲覧のみの Store(`Remote.ReceiveOnly`)では、(1) も `Remote.Repair` も行わない(06「修復の決まり」の 1 つ目)。
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test` と、e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)。

## アクティビティの文(修復の保存の Summary)

| 修復 | Summary | 根拠 |
|---|---|---|
| 削除と変更の重なり(アイテム・プロジェクト) | `{ID} の削除を取り消し` | 06「アクティビティ」の表 |
| 削除と変更の重なり(設定の値・プロパティ・テンプレート・ルール) | `「{値の名前}」の削除を取り消し` | 同上 |
| (1) 祖先が完全に削除されたアイテム | `{ID} を完全に削除(祖先が完全に削除)` | 06 の表に行がない。同じ表の「…(スプリントが完全に削除)」の書き方に合わせた(C3a-6)。Task 1 で 06 の表に足す |
| (1) 完全に削除された相手への関連 | `{ID} の関連を解除(相手が完全に削除)` | 同上 |
| (2) 親が削除された | `{子の ID} を削除` | 06「アクティビティ」の表 |
| (3) 番号の重なり | `{旧 ID} を {新 ID} に振り直し` | 06「番号の重なりの修復」 |
| (4) 親の循環 | `{ID} を {行き先の ID} の下へ移動(親の循環)` | 06「アクティビティ」の表。行き先がプロジェクトの直下(Epic を動かすとき)は `{行き先の ID}` をプロジェクトの ID(HH-P1)にする |
| (5) プロジェクトだけが違う子 | `{ID} をプロジェクト「{名前}」に移動(親に合わせる)` | 06「アクティビティ」の表 |

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C3a-1 | C3 を 3 本に分ける。C3a = 修復の枠組み・削除と変更の重なり(同期)・(1)〜(4)・(5) のプロジェクトだけが違う子。C3a-screen = 他のタブで削除されたアイテム・プロジェクトへの保存(06「他の書き手との重なり」の削除の取り消しとトースト)。C3b = (5) のワークスペースが違う子。トラック C の計画の一覧と漏れの表を直す(Task 1) | ユーザーの決定(2026-11-14)。C3b は A5 の「別のワークスペースへの移動」の書き込み(番号の振り直し・値の引き直し・`transfers.jsonl`)を使う。C3a-screen は C3a の `restore` の行を使う |
| C3a-2 | 修復は、合わせたあと・送る前に、`Replicator.Sync` の中で書く。C2 の申し送り(「`Service.run` の `Succeeded` の前に修復を足す」)と設計書 144 行(「`app/sync` が Stores.Write で保存する」)を、この形に直す(Task 1) | 保存形式「同期」の手順 4〜6(修復のコミットのあとに push)。送ったあとに書くと、相手のマシンが修復していない合わせた結果を受け取り、(3) で「remote に届いた方」の判定がマシンごとに変わって、2 台が別の番号に振り直す。`Sync` は Store のロックを持ったままなので、`Stores.Write` を中から呼ぶと止まる |
| C3a-3 | 修復の口: `port.Remote.Repair func(ctx context.Context, w RepairWriter) error`(nil なら呼ばない。`Mirror` では使わない)。`port.RepairWriter` は `Facts() repair.Facts`・`Read(fn func(Reader) error) error`・`Write(op Operation, fn func(Tx) error) error`。gitstore の実装は、ロックを取り直さずに読み書きし、`Write` は 1 回で 1 コミット | 修復の規則は app と domain に置き、診断(08、トラック G)が同じ規則を `Stores.Write`(実行者「あなた」)で使えるようにする。口は git の言葉を使わない |
| C3a-4 | `repair.Facts{Ours, Theirs map[change.ID]bool}` は、共通の祖先にない変更記録の ID(すべての `log.jsonl` の行)を、自分の側・remote の側に分けたもの。早送り・同じ先端・remote が空のときは両方とも空 | 保存形式「同期の修復」の表の「共通の祖先にない」「作成のコミットが remote の先端から辿れる」。早送りで受け取るものは、相手のマシンが修復を済ませて送っている |
| C3a-5 | 修復の順: (1)(gitstore) → `Remote.Repair` の中で、削除と変更の重なり → (2) → (3) → (4) → (5)。06 は削除と変更の重なりを (1) より前と決めているが、重なりの取り消しは完全に削除されたもの(フォルダがない)に行を書けず、(1) は削除済みかを見ないので、どちらを先にしても結果は同じ。(1) を gitstore に置くために、この順にする | 06「修復の決まり」の 5 つ目、保存形式「同期の修復」の 2 段落目 |
| C3a-6 | (1) の保存の文は、上の「アクティビティの文」の 2 行。`purged.jsonl` の行は `{"id","at","by":"hedgehug[bot]","actor":"sync","entity":"item","target","display"}`。`display` は、アイテムのワークスペースがあればその接頭辞の ID(HH-12)、なければ内部 ID | 06 の表に文がないので、同じ表の書き方に合わせる(規則の当てはめ)。保存形式「削除・復元・完全に削除」の例の形 |
| C3a-7 | `restore` の行: `{"t":"restore","f":<戻す delete の行の f>,"to":"<戻す削除の op>"}`。削除の op の行があるログごとに書く(1 つのログに 1 行)。`change.DeletionOf`・`workspace.SettingsFromLog`・`automation.collect` は、同じ `f` で `to` が自分の `op` の `restore` がある `delete` の行を、なかったものとして扱う。`restore` の行は履歴に出さない(`history` の除外は変えない。`restore` は `Resolve` が読まない種類) | 保存形式「変更記録」(`restore` は戻す削除の `op` を持つ)。`DeletionOf` は 1 つのログしか見ないので、ログごとに行が要る。08「復元の単位」の書き込みも同じ形(トラック G の画面がこの行を書く) |
| C3a-8 | 戻す変更は `change.Undo(log []Record, op ID) []Draft`: その `op` の `delete` の行には `restore`、`set` の行は、その行が項目の先端(`Tips` に入る)なら `set`(from と to を入れ替え)、`add` の行は先端なら `remove`、`remove` の行は先端なら `add`。ほかの種類(`create`・`desc`・`comment`・`worklog`・`number`)と、先端でない行は戻さない | 保存形式「削除・復元・完全に削除」の「復元」(値はその op の行の from から戻す。あとの操作で先端でなくなった行は戻さない)。値の削除で同じ op に書いた置き換え(B2 の `replaceRows`)も、ここで戻る |
| C3a-9 | 削除と変更の重なりの見つけ方(`repair.Overlaps`): ログの `delete` の行 D が `Facts` の片側にあり、D の `op` を戻す `restore` がまだなく、もう一方の側に、同じログの「変更」C がある。C は、`delete`・`restore` でなく、`op` が D と違う行。D の `f` が `deleted`(エンティティの削除)なら `root: true` の D だけを対象にし、C は同じログのどの行でもよい。D の `f` がそれ以外(`statuses/<ID>`・`properties/<ID>`・`choices/<P>/<V>`・テンプレート・ルールなど)なら、C は `f` が `D.f + "/"` で始まる行 | 保存形式「同期の修復」の「順の指定なし: 削除と変更が重なった」の行。`root: false` は (2) の対象。親の削除と子の変更は子のログだけに変更があるので、親は戻らない(保存形式「テスト」の「親と一緒の削除」)。値の名前・色の変更は `statuses/<ID>/name` のような `f` になる |
| C3a-10 | ワークスペースの削除は、いまどのトラックにもない(書く操作がない)ので、(2) の「削除済みのワークスペースの下」は扱わない。削除済みのアイテムとプロジェクトだけを親として見る。ワークスペースの削除を足すトラックが、(2) に削除済みのワークスペースを足す | ワークスペースの削除を書く usecase がない(調べた結果) |
| C3a-11 | (2) で子に書く `delete` は、`root: false`、`op` は親(アイテム、なければプロジェクト)の削除の op。行ごとに op を変えられるよう、`change.Draft.Op` と `Draft.InOp(op)` を足し、`Stamp` は `Op` があればそれを使う。`Op` を持つ行を書けるのは実行者 `sync` の操作だけ(`port.CheckOpDrafts`。`CheckRuleDrafts` と同じ場所で呼ぶ) | 保存形式「同期の修復」の表の (2)(op は親の削除と同じ。親を復元すれば一緒に戻る)。人の操作が別の操作の op を名乗ると、復元とアクティビティのまとめが崩れる |
| C3a-12 | (2) の親が、削除済みのアイテムと削除済みのプロジェクトの両方に当たるときは、親のアイテムの削除の op を使う。子の子は、子を削除済みにしたあとの次の回で見つかる(なくなるまで繰り返す) | 08「復元の単位」(アイテムを戻すとき、その子孫が一緒に戻る) |
| C3a-13 | (3) は、同じワークスペースの、番号が同じアイテム(削除済みを含む)とプロジェクト(削除済みを含む)を対象にする。残すのは、作成の行が `Facts.Ours` にないもの(remote に届いた方)のうち作成の `at` が最も古いもの(同じなら内部 ID の辞書順で前)。全部が `Ours` にあれば、全部のうち作成の `at` が最も古いもの。残さないものは、旧い番号の小さい順、同じなら内部 ID の順に、`numbering.Next` から振る(1 件ごとに `number` の行と `set place` を 1 回の保存で書く) | 06「番号の重なりの修復」。削除済みも ID を持ち、復元すると戻るので、重なったままにしない |
| C3a-14 | (4) は、削除されていないアイテムだけで輪を探す。輪が複数あれば、輪の中の最小の内部 ID の順で 1 つ目を直し、また探す。動かすものは、輪の中で番号が最も大きいもの(同じなら内部 ID の辞書順であと)。行き先は 08「親がない」: アイテムの `Place.Project` が読めない(完全に削除)なら直さない。動かすものが Epic なら、プロジェクトの直下(`parent` を nil)。それ以外は、そのプロジェクトの削除されていない、親のない Epic を `item.SortSiblings` で並べた先頭の下の末尾(`rank.After`)。Step は `set kind` で Task にする。Epic がなければ「修復で作った Epic」を作る(下の C3a-15)。輪を切ったあと、輪に入っていたほかのアイテムで、親が `item.CanPlace` に合わないものも、同じ修復で 1 件ずつ動かす | 06「親が循環したとき」、08「親がない」。Epic の下に Epic は置けない(01)ので、Epic は直下へ戻す。親のある Epic は輪の中の壊れたものなので、行き先に選ばない |
| C3a-15 | 「修復で作った Epic」: タイトル `修復で作った Epic`、内部 ID は `repair.EpicID(p project.ID, used func(change.ID) bool) change.ID`。プロジェクトの内部 ID の先頭 1 文字を `R` に替えたもの。その ID のアイテムがすでにあれば(削除済みを含む)、`S`、`T` … `Z` の順に替えたものを使う。番号は `numbering.Next`、順位はプロジェクトの直下の削除されていないものの末尾。作成と移動は 1 回の保存 | 保存形式「同期の修復」の (4)(内部 ID はプロジェクトの内部 ID から決まる値)。ULID は 0〜7 で始まるので `R`〜`Z` は利用者のものと重ならない。題名は 08 の呼び名をそのまま使う |
| C3a-16 | (5) は、削除されていない子と削除されていない親が同じワークスペースで、プロジェクトだけが違うものを、深さの浅い順(同じなら内部 ID の順)に 1 件ずつ直す。`set place` でプロジェクトを親のプロジェクトにする(番号・親は変えない)。ワークスペースが違う子は C3b が直すので、ここでは触らない。ステータスの置き換え(移動先のプロジェクトで使わないステータス)は、プロジェクト別のルールの「使わないステータス」を足すトラック B5 が、この修復に足す | 06「親と別のワークスペース・プロジェクトにある子」の 2 つ目の箇条。いまはプロジェクトが使わないステータスを持てない(B5 がまだ)ので、置き換える値がない |
| C3a-17 | (1) は gitstore の `sync_repair.go` で行う。すべての `items/*`(プロジェクト・ワークスペースのフォルダがないものも)を読み、祖先(親のアイテム・プロジェクト・ワークスペース)の内部 ID が `purged.jsonl` か、同じ回で消すアイテムにあるものを、内部 ID の順に 1 件ずつ、フォルダを消して `purged.jsonl` に 1 行足す。続けて、相手が `purged.jsonl` にある関連を、持ち主ごとに `remove` する | 保存形式「同期の修復」の (1)。`port.Tx` はフォルダの削除と `purged.jsonl` を扱えない(ファイルの扱いは gitstore だけ。設計書 3 節) |
| C3a-18 | 通知(C4)は、修復のコミット(実行者 `sync`)の件名・行と、対象のログから宛先を決める。C3a は通知の材料を別に記録しない。1 件の修復を 1 コミットにする(Global Constraints)ので、コミット 1 つが通知 1 件に対応する。(2) の行は op が親の削除のもので、コミットの `Hedgehug-Op` と違うので、C4 のアクティビティは修復のコミットを件名から作る(remote の履歴へののせ直しと同じ扱い) | 06「同期の修復」の通知は、受け取ったマシンが自分宛てに作る。受け取る側は自分で修復していないので、修復の結果だけから作れる必要がある |
| C3a-19 | `Replicator.Sync` の結果の送った数には、修復のコミットも入る(送った保存) | 修復は保存(01「書き込みの単位」)。06「同期の結果」の送った数は保存の数 |
| C3a-20 | 2 台の場面テストは `cmd/hedgehug/repair_scenario_test.go` に置く。本物の gitstore と `app/repairing` の両方を import できるのは cmd だけ(archtest)。remote は一時フォルダの bare リポジトリをパスで渡す(go-git のファイルの通信。試して動くことを確かめた) | 設計書 2 節の依存の向き。gitstore のテストの `https://remote.invalid` の仕組みは gitstore の _test にしかない |

## Review Focus

1. **送信が拒否されて受け取りからやり直したとき。** やり直しのたびに修復を行うが、前の試みで書いた修復のコミットが自分の側に残っているので、2 回目は何も書かない。番号の振り直しが 2 回進まない。Task 5 のテストに入れる(`beforeSend` で 1 回だけ拒否する)。
2. **3 台目のマシン・2 回目の同期。** 修復済みの remote を早送りで受け取ったマシンは何も書かない。2 台が同時に同じ修復をして合わせても、値は同じで、番号がずれない。Task 13 のテストに入れる。
3. **同じ側での削除と変更。** 自分で変更してから自分で削除した(どちらも `Ours`)ものは取り消さない。削除したあとに値の置き換えで書いた行(A4b-5。削除済みも対象)も、同じ側なので取り消さない。Task 6 のテストに入れる。
4. **戻す操作のあとに別の操作で変えた値。** 値を削除した op の置き換えの行のあと、別の操作でそのアイテムのステータスを変えていたら、そのアイテムのステータスは戻さない(先端でない)。Task 3 のテストに入れる。
5. **閲覧のみの Store。** 受け取った状態に修復が要っても、何も書かず、remote にも送らない。Task 5 のテストに入れる。

---

### Task 1: トラック C の計画・設計書・仕様の表を直す

**Files:**
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(小計画の一覧の C3 の行、漏れの表、C4 の「待つもの」)
- Modify: `docs/superpowers/specs/2026-11-12-hedgehug-architecture-design.md:144`
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`(「アクティビティ」の「対象」の表)

**Interfaces:** なし(文書だけ)。

- [ ] **Step 1: トラック C の小計画の一覧の C3 の行を 3 行にする**

C3a(この計画のファイル名を書く。範囲は C3a-1 の C3a)、C3a-screen(範囲: 06「他の書き手との重なり」の削除の取り消しとトースト。待つもの: C3a。完成の条件: 別のタブで削除したアイテム・プロジェクトへの保存で、削除が取り消され、トーストが出る)、C3b(範囲: 06「親と別のワークスペース・プロジェクトにある子」の 1 つ目の箇条。待つもの: C3a、A5。完成の条件: 場面テストで、子が親のワークスペースの次の番号に移り、転送記録が残る)。漏れの表の「他の書き手との重なり」を C3a-screen、「親と別のワークスペース・プロジェクトにある子」を C3a(プロジェクト)・C3b(ワークスペース)、ほかの C3 を C3a にする。C4・C5 の「待つもの」の C3 を C3a にする。

- [ ] **Step 2: 設計書 144 行を直す**

「合わせたあとの修復(親の循環など)は、`domain/repair` の規則を `app/repairing` が `port.RepairWriter`(実行者「同期」)で保存する。`infra/gitstore` の `Replicator.Sync` が、合わせたあと・送る前に、Store のロックを持ったまま呼ぶ。完全に削除したものと重なったときのフォルダの削除は gitstore が行う。項目ごとの後勝ちで合わせる処理は `infra/gitstore` の中に置く。」

- [ ] **Step 3: 06 の「アクティビティ」の表に 2 行足す**

「同期で、祖先が完全に削除されたアイテムを完全に削除した | 同期 | {ID} を完全に削除(祖先が完全に削除)」と「同期で、完全に削除された相手への関連を解除した | 同期 | {ID} の関連を解除(相手が完全に削除)」を、「同期で、削除された親の子を削除済みに入れた」の行の下に足す。

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/plans/2026-11-12-hedgehug-track-c.md docs/superpowers/specs/2026-11-12-hedgehug-architecture-design.md docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md docs/superpowers/plans/2026-11-14-hedgehug-track-c3a.md
git commit -m "docs: C3 を C3a・C3a-screen・C3b に分け、修復を送る前に書く形を設計書に書く"
```

---

### Task 2: `restore` の行と、取り消された削除を読まない

**Files:**
- Modify: `internal/domain/change/record.go`(`Restore Type = "restore"`)
- Modify: `internal/domain/change/draft.go`(`NewRestore`)
- Modify: `internal/domain/change/deletion.go`(`DeletionOf`、`Restored`)
- Modify: `internal/domain/workspace/settings_log.go:51-54`(削除の収集)
- Modify: `internal/domain/automation/log.go:12-18`(`collect`)
- Test: `internal/domain/change/deletion_test.go`、`internal/domain/workspace/settings_log_test.go`(既存のテストファイルの名前に合わせる)、`internal/domain/automation/log_test.go`
- Test: `internal/app/apptest/stores_contract_values.go`(`contractDeleted` に、取り消した削除のケースを足す)

**Interfaces:**
- Produces:
  - `const Restore Type = "restore"`
  - `func NewRestore(f string, op ID) Draft` → `Draft{T: Restore, F: f, To: Value(op)}`
  - `func Restored(log []Record) func(d Record) bool` — `d` が `delete` の行で、同じ `f`、`to` が `d.Op` の `restore` の行が log にあれば true
  - `DeletionOf` は、`Restored` が true の行を飛ばす

- [ ] **Step 1: 失敗するテストを書く**

```go
func TestDeletionOfSkipsRestored(t *testing.T) {
	del := rec("d1", "op1", t0, Delete, DeletedField) // root:true
	res := Record{ID: "r1", Op: "op2", At: t1, T: Restore, F: DeletedField, To: Value(ID("op1"))}
	if got := DeletionOf([]Record{del, res}); got != nil {
		t.Fatalf("restored deletion = %+v, want nil", got)
	}
	again := rec("d2", "op3", t2, Delete, DeletedField)
	if got := DeletionOf([]Record{del, res, again}); got == nil || got.Op != "op3" {
		t.Fatalf("deleted again = %+v, want op3", got)
	}
}

func TestRestoreOfOtherFieldDoesNotCount(t *testing.T) {
	del := rec("d1", "op1", t0, Delete, DeletedField)
	other := Record{ID: "r1", Op: "op2", At: t1, T: Restore, F: "statuses/X", To: Value(ID("op1"))}
	if DeletionOf([]Record{del, other}) == nil {
		t.Fatal("restore of another f must not undo the deletion")
	}
}
```

設定: `statuses/<ID>` の `delete`(op1)と、`f` が同じで `to` が op1 の `restore` があるログから `SettingsFromLog` で読むと、そのステータスが一覧に戻る。ルール: 同じ形で `automation` のルールが戻る。契約テスト: `apptest.DeleteItems` で消したアイテムのログに `NewRestore(DeletedField, op)` を足すと、`Items().Get` の `Deleted` が nil(本物の gitstore と偽物の両方で通る)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/change/ ./internal/domain/workspace/ ./internal/domain/automation/ ./internal/app/apptest/ ./internal/infra/gitstore/`
Expected: FAIL(`Restore`・`NewRestore` がない)

- [ ] **Step 3: 実装する**

`DeletionOf` のコメントの「restore の行は見ない」を、「同じ f で to が自分の op の restore がある行は飛ばす(C3a-7)」に直す。`SettingsFromLog` と `collect` は、`delete` の行を集めるところで `Restored(log)` が true の行を飛ばす。

- [ ] **Step 4: 通ることを確かめる**

Run: 同じコマンド
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git commit -am "feat: restore の行を足し、取り消した削除を読まない"
```

---

### Task 3: 操作 1 つの行を戻す `change.Undo`

**Files:**
- Create: `internal/domain/change/undo.go`
- Test: `internal/domain/change/undo_test.go`

**Interfaces:**
- Consumes: `NewRestore`、`Restored`(Task 2)、`Tips(log []Record, r Record) []ID`(stamp.go)
- Produces: `func Undo(log []Record, op ID) []Draft`(C3a-8)。返す順は、log の中の行の順

- [ ] **Step 1: 失敗するテストを書く**

```go
func TestUndoDeleteWritesRestore(t *testing.T) {
	log := []Record{create("c", "op0"), {ID: "d", Op: "op1", T: Delete, F: DeletedField, Root: ptr(true)}}
	got := Undo(log, "op1")
	want := []Draft{NewRestore(DeletedField, "op1")}
	if !reflect.DeepEqual(got, want) { t.Fatalf("got %+v", got) }
}

func TestUndoSetOnlyWhileTip(t *testing.T) {
	s1 := Record{ID: "s1", Op: "op1", T: Set, F: "status", From: Value("A"), To: Value("B")}
	if got := Undo([]Record{create("c", "op0"), s1}, "op1"); !reflect.DeepEqual(got, []Draft{NewSet("status", "B", "A")}) {
		t.Fatalf("tip set: got %+v", got)
	}
	s2 := Record{ID: "s2", Op: "op2", T: Set, F: "status", From: Value("B"), To: Value("C"), Prev: []ID{"s1"}}
	if got := Undo([]Record{create("c", "op0"), s1, s2}, "op1"); len(got) != 0 {
		t.Fatalf("overwritten set must not be undone: %+v", got)
	}
}

func TestUndoAddRemove(t *testing.T) {
	a := Record{ID: "a", Op: "op1", T: Add, F: "systems", To: Value("W")}
	r := Record{ID: "r", Op: "op1", T: Remove, F: "systems", From: Value("X")}
	got := Undo([]Record{create("c", "op0"), a, r}, "op1")
	want := []Draft{NewRemove("systems", "W"), NewAdd("systems", "X")}
	if !reflect.DeepEqual(got, want) { t.Fatalf("got %+v", got) }
}

func TestUndoSkipsAlreadyRestoredAndOtherKinds(t *testing.T) {
	d := Record{ID: "d", Op: "op1", T: Delete, F: DeletedField, Root: ptr(true)}
	res := Record{ID: "r", Op: "op2", T: Restore, F: DeletedField, To: Value(ID("op1"))}
	cm := Record{ID: "m", Op: "op1", T: Comment, F: "C1", To: Value(map[string]string{"body": "x"})}
	if got := Undo([]Record{create("c", "op0"), d, res, cm}, "op1"); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/change/ -run TestUndo`
Expected: FAIL(`Undo` がない)

- [ ] **Step 3: `Undo` を実装する**

先端の判定は、その行と同じ項目の「次に書く行」を仮に作って `Tips(log, next)` を呼び、行の ID が入っているかで決める(`key` が同じなら同じ項目)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/change/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/change/undo.go internal/domain/change/undo_test.go
git commit -m "feat: 操作 1 つの行を戻す change.Undo を足す"
```

---

### Task 4: 行ごとの op(`Draft.InOp`)と、書ける実行者の確認

**Files:**
- Modify: `internal/domain/change/draft.go`(`Draft.Op`、`InOp`)
- Modify: `internal/domain/change/stamp.go`(`Op` があれば使う)
- Modify: `internal/app/port/stores.go`(`CheckOpDrafts`)
- Modify: `CheckRuleDrafts` を呼んでいる 2 か所(`internal/infra/gitstore/write.go` の `tx.append`、`internal/app/apptest/stores.go` の `tx.append`)
- Test: `internal/domain/change/stamp_test.go`、`internal/app/port/stores_test.go`(なければ既存のテストファイルに足す)、`internal/app/apptest/stores_contract_rules.go`(契約テストに 1 ケース)

**Interfaces:**
- Produces:
  - `Draft.Op ID` — 空なら操作の op
  - `func (d Draft) InOp(op ID) Draft`
  - `func CheckOpDrafts(actor change.Actor, drafts []change.Draft) error` — `Op` を持つ行があり、実行者が `change.ActorSync` でないか、`Op` が `change.ValidID` でなければ `ErrInvalidOperation` を包んだエラー

- [ ] **Step 1: 失敗するテストを書く**

```go
func TestStampUsesDraftOp(t *testing.T) {
	meta := Meta{Op: "opW", At: t0, Actor: Actor{Kind: ActorSync}}
	got := Stamp(nil, []Draft{NewDeleteEntity(false).InOp("opP"), NewSet("title", "a", "b")}, meta, seqIDs())
	if got[0].Op != "opP" || got[1].Op != "opW" {
		t.Fatalf("ops = %s, %s", got[0].Op, got[1].Op)
	}
}
```

`CheckOpDrafts`: 実行者 `change.User("taro")` で `InOp` の行 → `ErrInvalidOperation`。実行者 sync で `InOp("bad id!")` → `ErrInvalidOperation`。実行者 sync で `InOp(有効な ID)` → nil。契約テスト: 人の操作で `InOp` の行を `Append` すると、`Write` が `ErrInvalidOperation` を返し、何も残らない。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/change/ ./internal/app/port/ ./internal/app/apptest/ ./internal/infra/gitstore/`
Expected: FAIL

- [ ] **Step 3: 実装する**

- [ ] **Step 4: 通ることを確かめる**

Run: 同じコマンド
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git commit -am "feat: 同期の修復の行が、親の削除と同じ op を持てるようにする"
```

---

### Task 5: 修復の口と、合わせたあと・送る前の呼び出し

**Files:**
- Create: `internal/domain/repair/facts.go`(`package repair` の最初のファイル。パッケージのコメントもここ)
- Modify: `internal/app/port/replicator.go`(`Remote.Repair`、`RepairWriter`。`Replicator` のコメントの「合わせたあとの修復は、app/syncing が Stores.Write で保存する」を直す)
- Move: `internal/infra/gitstore/replicator_seen.go` の中身を `replicator_mirror.go` の末尾へ、`lock.go` の中身を `stores.go` の末尾へ(ファイルを 2 つ空ける。中身は変えない)
- Create: `internal/infra/gitstore/sync_repair.go`(`Facts` の計算、`repairWriter`、`syncRun.repair`)
- Modify: `internal/infra/gitstore/write.go`(`Write` を `open` + `runTx` に分ける)
- Modify: `internal/infra/gitstore/replicator.go`(`integrate` が `repair.Facts` を返し、`once` が送る前に `repair` を呼ぶ)
- Modify: `internal/app/apptest/sync.go`(偽の `RepairWriter`)
- Test: `internal/infra/gitstore/sync_repair_test.go`

**Interfaces:**
- Produces:
  - `package repair`: `type Facts struct { Ours, Theirs map[change.ID]bool }`。`func (f Facts) Side(id change.ID) int`(Ours なら 1、Theirs なら 2、どちらでもなければ 0)
  - `port.Remote.Repair func(ctx context.Context, w RepairWriter) error`
  - `port.RepairWriter interface { Facts() repair.Facts; Read(fn func(r Reader) error) error; Write(op Operation, fn func(tx Tx) error) error }` — `Write` の約束は `Stores.Write` と同じ(行がなければ何も残さない、op.Check、fn のエラーで何も残さない)
  - gitstore: `func (s *Stores) runTx(root string, op port.Operation, fn func(port.Tx) error) error`(ロックを持っている前提)
  - gitstore: `func (s syncRun) repair(ctx context.Context, f repair.Facts, hook func(context.Context, port.RepairWriter) error) error` — Task 7 が (1) をこの先頭に足す
  - apptest: `type RepairWriter struct { Stores port.Stores; Store port.StoreID; F repair.Facts; Ops []port.Operation }`(`Facts`・`Read`・`Write` は `Stores` に渡すだけ。`Write` が行を 1 つ以上残したときだけ、その `op` を `Ops` に足す。app のテストが件名を確かめるため)
- `Sync` の流れ: `integrate` のあと、`receiveOnly` でなければ `repair`、そのあと `push`。`theirs` がゼロ(remote が空)でも `repair` を呼ぶ(`Facts` は空)

- [ ] **Step 1: 失敗するテストを書く**

`sync_repair_test.go`(gitstore の場面のヘルパー `newRemote`・`firstMachine`・`cloneMachine`・`sameFiles` を使う):

```go
func TestRepairRunsBeforeSendAndIsPushed(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0)
	a.write(t, "A の変更", set("title", "x", "A"))
	b.write(t, "B の変更", set("title", "x", "B"))
	var facts repair.Facts
	remote.Repair = func(_ context.Context, w port.RepairWriter) error {
		facts = w.Facts()
		return w.Write(port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: "修復"},
			func(tx port.Tx) error { return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "?", "R")) })
	}
	a.sync(t, remote)
	b.sync(t, remote) // b は合わせて、修復を書いて送る
	if len(facts.Ours) == 0 || len(facts.Theirs) == 0 {
		t.Fatalf("facts on merge = %+v", facts)
	}
	a.sync(t, port.Remote{URL: remote.URL}) // a は早送りで受け取る
	sameFiles(t, a, b)
	if got := title(t, a); got != "R" { t.Fatalf("title = %q", got) }
}
```

ほかのケース(同じファイル):
- `TestRepairFactsEmptyOnFastForward`: 早送りで受け取る同期では `Facts()` の両方が空。
- `TestRepairNotCalledWhenReceiveOnly`: `ReceiveOnly: true` の同期では `Repair` が呼ばれない(Review Focus 5)。
- `TestRepairWriteDoesNotDeadlock`: `Repair` の中の `Read` と `Write` が 5 秒以内に終わる(`context.WithTimeout` で包み、終わらなければ失敗)。
- `TestRepairOnRetryWritesOnce`: `beforeSend` で 1 回目だけ `errRejected` を返す。`Repair` は「タイトルが R でなければ R にする」(すでに R なら何も書かない)。同期のあと、ログの `title` の `set` で `to` が `"R"` のものが 1 行だけ(Review Focus 1)。
- `TestRepairErrorStopsSync`: `Repair` がエラーを返すと、`Sync` はそのエラーを返し、送らない(remote の先端が変わらない)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/infra/gitstore/ -run TestRepair`
Expected: FAIL(`Remote.Repair` がない)

- [ ] **Step 3: 実装する**

`Facts` は、`merge` で読んだ 3 つのツリーから、`log.jsonl` で終わるすべてのファイルの行の ID を集め、base にないものを ours・theirs に分ける(`parseLog` を使う)。早送り・同じ先端・remote が空なら空の `Facts`。`repairWriter.Write` は `s.r.stores.runTx(root, op, fn)`、`Read` は `fn(&view{root: root})`。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/infra/gitstore/ ./internal/app/... && go test ./internal/archtest/`
Expected: PASS(gitstore のテスト以外のファイルは 20 個のまま)

- [ ] **Step 5: Commit**

```bash
git add -A internal/domain/repair internal/app/port/replicator.go internal/infra/gitstore internal/app/apptest/sync.go
git commit -m "feat: 同期で合わせたあと、送る前に修復を書けるようにする"
```

---

### Task 6: 削除と変更の重なりを見つける `repair.Overlaps`

**Files:**
- Create: `internal/domain/repair/overlap.go`
- Test: `internal/domain/repair/overlap_test.go`

**Interfaces:**
- Consumes: `repair.Facts`(Task 5)、`change.Restored`(Task 2)
- Produces:
  - `type Overlap struct { Op change.ID; F string }` — 取り消す削除の op と、その `delete` の行の `f`
  - `func Overlaps(log []change.Record, f Facts) []Overlap` — C3a-9 の規則。`Op` の辞書順、同じ op は 1 つ

- [ ] **Step 1: 失敗するテストを書く**

表で書く。各ケースは、ログの行と、どの行が `Ours`・`Theirs` か、と期待する `[]Overlap`。
- アイテムの削除(root:true、Ours)とタイトルの変更(Theirs) → `{op, "deleted"}`
- 逆の側(削除が Theirs、変更が Ours) → 同じ
- 削除と変更が同じ側(どちらも Ours) → なし(Review Focus 3)
- 削除(Ours)と、その削除と同じ op の行(Theirs にはない) → なし
- `root: false` の削除(Ours)と変更(Theirs) → なし
- 削除と、もう取り消しの `restore` がある → なし
- コメントの追加(`t: comment`、Theirs)とアイテムの削除(Ours) → `{op, "deleted"}`
- ワークスペースのログ: `delete statuses/X`(Ours)と `set statuses/X/name`(Theirs) → `{op, "statuses/X"}`
- `delete statuses/X`(Ours)と `set statuses/XY/name`(Theirs) → なし(前方一致は `f + "/"`)
- 削除も変更も共通の祖先にある(Facts にない) → なし

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/repair/ -run TestOverlaps`
Expected: FAIL

- [ ] **Step 3: 実装する**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/repair/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/repair/overlap.go internal/domain/repair/overlap_test.go
git commit -m "feat: 同期で削除と変更が重なったものを見つける"
```

---

### Task 7: (1) 祖先が完全に削除されたアイテムと、完全に削除された相手への関連

**Files:**
- Create: `internal/domain/repair/purged.go`
- Test: `internal/domain/repair/purged_test.go`
- Modify: `internal/infra/gitstore/sync_repair.go`(`repair` の先頭で (1) を行う)
- Modify: `internal/infra/gitstore/write.go`・`commit.go`(`tx` にフォルダの削除と `purged.jsonl` の 1 行を持たせ、コミットで書く)
- Test: `internal/infra/gitstore/sync_repair_test.go`

**Interfaces:**
- Produces:
  - `func UnderPurged(items []item.Item, projectWorkspace map[project.ID]workspace.ID, purged map[change.ID]bool) []item.ID` — 親のアイテム(たどれる限り)・プロジェクト・ワークスペースのどれかが `purged` にあるか、すでにこの結果に入ったアイテムの下にあるもの。内部 ID の順。`projectWorkspace` にないプロジェクトは、そのワークスペースを見ない(プロジェクトの ID だけで判定)
  - `type Dangling struct { Owner item.ID; Relation item.Relation }`
  - `func PurgedRelations(items []item.Item, purged map[change.ID]bool) []Dangling` — 持ち主の内部 ID、関連の相手の内部 ID、種類の順
  - gitstore(内部): `func (t *tx) purge(ref change.Ref, display string) error`(そのエンティティのフォルダのファイルを消し、`purged.jsonl` に C3a-6 の行を足す。同じ tx の中で消したものを `Items().Get` は `port.ErrNotFound` にする)

- [ ] **Step 1: 失敗するテストを書く**

domain: 親の Epic が purged の Task と、その下の Step → 両方。プロジェクトが purged → そのプロジェクトのアイテム全部。ワークスペースが purged → そのワークスペースのプロジェクトのアイテム全部。関係ないアイテム → 入らない。関連の相手が purged → `Dangling` に入る。

gitstore(場面):
```go
func TestRepairPurgesChildOfPurgedParent(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0)
	a.purgeForTest(t, apptest.SampleEpic, "HH-1") // runTx の中で (*tx).purge
	b.write(t, "子を作る", createStepUnder(apptest.SampleTask)) // B が HH-2 の下に Step を作る
	a.sync(t, remote)
	b.sync(t, remote)
	a.sync(t, port.Remote{URL: remote.URL})
	sameFiles(t, a, b)
	// HH-2(Task)と、B が作った Step のフォルダがない。purged.jsonl に HH-2 と Step の行がある
	// コミットの件名に「HH-2 を完全に削除(祖先が完全に削除)」がある
}
```
ほか: 完全に削除された相手(Theirs の側で purge)に、もう一方が関連を足した → 同期のあと、関連が `remove` され、件名「{ID} の関連を解除(相手が完全に削除)」。閲覧のみの同期では何もしない。もう一度同期しても何も書かない。

`purgeForTest` は `sync_repair_test.go` に置くテストのヘルパー(machine のメソッド)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/repair/ ./internal/infra/gitstore/ -run 'Purge|Purged'`
Expected: FAIL

- [ ] **Step 3: 実装する**

`repair` の (1) は、`view.ids(change.KindItem)` で全アイテムを `item.FromLog` で読み、プロジェクトのフォルダのログからワークスペースを引き、`purged.jsonl` の `target` を集めて `UnderPurged` と `PurgedRelations` を呼ぶ。消すアイテム 1 件ごとに `runTx`(Summary `{ID} を完全に削除(祖先が完全に削除)`)、関連は持ち主ごとに `runTx`(`{ID} の関連を解除(相手が完全に削除)`。持ち主の関連を全部まとめて `remove`)。`display` は C3a-6。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/repair/ ./internal/infra/gitstore/ && go test ./internal/archtest/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git commit -am "feat: 同期で、祖先が完全に削除されたアイテムと、完全に削除された相手への関連を直す"
```

---

### Task 8: `app/repairing` と、削除と変更の重なりの取り消し

**Files:**
- Create: `internal/app/repairing/run.go`(`Run` と修復の順)
- Create: `internal/app/repairing/snapshot.go`(ワークスペース・プロジェクト・アイテム(削除済みを含む)とログを読む)
- Create: `internal/app/repairing/restore.go`
- Modify: `internal/domain/workspace/settings_log.go`(`NameOfDeleted`)
- Modify: `internal/app/syncing/service.go`(`Repair` の欄)、`internal/app/syncing/run.go`(`Remote.Repair` に渡す)
- Modify: `cmd/hedgehug/wire.go`(`Repair: repairing.Run`)
- Test: `internal/app/repairing/restore_test.go`、`internal/app/syncing/service_test.go`、`internal/domain/workspace/settings_log_test.go`

**Interfaces:**
- Consumes: `port.RepairWriter`、`apptest.RepairWriter`(Task 5)、`repair.Overlaps`(Task 6)、`change.Undo`(Task 3)
- Produces:
  - `func Run(ctx context.Context, w port.RepairWriter) error` — 修復の順(C3a-5)に、各段の関数を呼ぶ。Task 9〜12 が段を足す
  - `type step func(ctx context.Context, w port.RepairWriter) error`
  - `func restoreOverlaps(ctx context.Context, w port.RepairWriter) error`
  - `syncing.Service.Repair func(ctx context.Context, w port.RepairWriter) error`(nil なら `Remote.Repair` も nil)
  - `func NameOfDeleted(log []change.Record, f string) string` — `f` の値(ステータス・優先度・関連システム・選択肢・プロパティ・テンプレート・ルール)の名前を、削除の行がないものとして決める。見つからなければ空
- 取り消しの書き方: 重なり 1 つ(op)ごとに 1 回の `Write`。すべてのワークスペース・プロジェクト・アイテムのログのうち、その op の行があるものに `change.Undo(log, op)` を `Append` する。Summary は、`f` が `deleted` なら、`root: true` の削除の行があるログのエンティティの ID(アイテムは `DisplayID`、プロジェクトは `DisplayID`)で「{ID} の削除を取り消し」、それ以外は「「{NameOfDeleted}」の削除を取り消し」

- [ ] **Step 1: 失敗するテストを書く**

`restore_test.go`(`apptest.NewStores` と `apptest.Sample`。Facts は行の ID を手で入れる):
- アイテム HH-2 の削除(op1、Ours)と HH-2 のタイトルの変更(Theirs) → `Run` のあと HH-2 は削除済みでない、タイトルは変更のまま。`apptest.RepairWriter.Ops` の Summary が「HH-2 の削除を取り消し」の 1 件、実行者が `change.ActorSync`
- Epic HH-1 の削除(op1。HH-1 は root:true、HH-2 は root:false。Ours)と HH-1 のタイトルの変更(Theirs) → HH-1・HH-2 とも戻る
- HH-1 の削除(Ours)と HH-2 の変更(Theirs) → どちらも戻らない(保存形式「テスト」の「親と一緒の削除」)
- プロジェクト HH-P1 の削除と HH-P1 の名前の変更 → プロジェクトとアイテムが戻る。件名「HH-P1 の削除を取り消し」
- ステータス「Review」の削除(置き換えでアイテムの status を Todo にした行を含む)と「Review」の色の変更 → ステータスが戻り、アイテムの status も Review に戻る。件名「「Review」の削除を取り消し」
- もう一度 `Run` しても行が増えない

`syncing`: `Service.Repair` を置くと、`Replicator` の偽物が受けた `Remote.Repair` が nil でない。置かなければ nil。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/repairing/ ./internal/app/syncing/ ./internal/domain/workspace/`
Expected: FAIL

- [ ] **Step 3: 実装する**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/... ./internal/domain/... ./cmd/... && go test ./internal/archtest/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/repairing internal/app/syncing internal/domain/workspace cmd/hedgehug/wire.go
git commit -m "feat: 同期で削除と変更が重なったら、変更を残して削除を取り消す"
```

---

### Task 9: (2) 削除済みの親の下の子を削除済みに入れる

**Files:**
- Create: `internal/domain/repair/cascade.go`
- Create: `internal/app/repairing/cascade.go`
- Modify: `internal/app/repairing/run.go`(段を足す)
- Test: `internal/domain/repair/cascade_test.go`、`internal/app/repairing/cascade_test.go`

**Interfaces:**
- Produces:
  - `type Cascade struct { Child item.ID; Op change.ID }`
  - `func Cascades(items []item.Item, projects []project.Project) []Cascade` — 削除されていないアイテムで、親のアイテムが削除済み(op は親の `Deleted.Op`)か、親が削除済みでなくプロジェクトが削除済み(op はプロジェクトの `Deleted.Op`)のもの。内部 ID の順(C3a-12)
  - app: 1 件ごとに 1 回の `Write`。`Append(child, change.NewDeleteEntity(false).InOp(op))`、Summary「{子の ID} を削除」。なくなるまで読み直して繰り返す

- [ ] **Step 1: 失敗するテストを書く**

domain: 親が削除済みの Task → `{Task, 親の op}`。プロジェクトが削除済みで親が生きている Epic → `{Epic, プロジェクトの op}`。親もプロジェクトも削除済み → 親の op。削除済みの子 → 入らない。
app: Epic HH-1 を削除済み(apptest.DeleteItems で HH-1 だけ。HH-2 は残す)にしておく → `Run` のあと HH-2 は削除済みで `Deleted.Op` が HH-1 の op、`Deleted.Root` は false。HH-2 の下の Step も、同じ `Run` の中で削除済みになる。もう一度 `Run` しても行が増えない。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/repair/ ./internal/app/repairing/ -run Cascade`
Expected: FAIL

- [ ] **Step 3: 実装する**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/repair/ ./internal/app/repairing/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git commit -am "feat: 同期で、削除済みの親の下に残った子を削除済みに入れる"
```

---

### Task 10: (3) 番号の重なりを振り直す

**Files:**
- Create: `internal/domain/repair/numbers.go`
- Create: `internal/app/repairing/numbers.go`
- Modify: `internal/app/repairing/run.go`
- Test: `internal/domain/repair/numbers_test.go`、`internal/app/repairing/numbers_test.go`

**Interfaces:**
- Produces:
  - `type Numbered struct { ID change.ID; Number int; Create change.Record }`
  - `type Renumber struct { ID change.ID; From, To int }`
  - `func Renumbers(all []Numbered, f Facts, next int) []Renumber` — C3a-13。番号が重ならなければ空。`To` は `next` から 1 つずつ
  - app: ワークスペースごとに、アイテム(全プロジェクト、削除済みを含む)とプロジェクト(削除済みを含む)を別々に調べる。作成の行は `Changes().Of` の `t: create`。1 件ごとに 1 回の `Write`: ワークスペースに `numbering.Draft(kind, n)`、エンティティに `set place`(番号だけ変える)。`next` は書くたびに `numbering.Next` で読み直す。Summary「{旧 ID} を {新 ID} に振り直し」(アイテムは HH-3、プロジェクトは HH-P3)

- [ ] **Step 1: 失敗するテストを書く**

domain:
- HH-3 が 2 件、一方の作成が Ours → Ours の方を `next` へ
- 2 件とも Ours にない(両方 remote) → 作成の at が新しい方を振り直す
- at が同じ → 内部 ID が後の方を振り直す
- 3 件重なり、Ours が 2 件 → Ours の 2 件を、内部 ID の順に next・next+1
- 番号 3 と 5 がそれぞれ重なる → 旧い番号の小さい順(3 の方から)

app: サンプルのワークスペースに、番号 2 の Task をもう 1 つ作る(作成の行を Ours に入れる) → `Run` のあと、その Task は番号 3(サンプルが使った最大番号の次)、ワークスペースのログに `number` の 3 の行がある。プロジェクトの番号の重なりも同じ。もう一度 `Run` しても行が増えない。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/repair/ ./internal/app/repairing/ -run Renumber`
Expected: FAIL

- [ ] **Step 3: 実装する**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/repair/ ./internal/app/repairing/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git commit -am "feat: 同期で重なったアイテムとプロジェクトの番号を振り直す"
```

---

### Task 11: (4) 親の循環を切る

**Files:**
- Create: `internal/domain/repair/cycle.go`
- Create: `internal/app/repairing/cycle.go`
- Modify: `internal/app/repairing/run.go`
- Test: `internal/domain/repair/cycle_test.go`、`internal/app/repairing/cycle_test.go`

**Interfaces:**
- Produces:
  - `func Cycle(items []item.Item) (ring []item.ID, ok bool)` — 削除されていないアイテムだけで親をたどり、輪を 1 つ返す(C3a-14 の選び方。`ring` は内部 ID の順)
  - `func Mover(ring []item.ID, items []item.Item) item.ID` — 番号が最も大きいもの、同じなら内部 ID の辞書順であと
  - `type Destination struct { Parent *item.ID; NeedEpic bool }`
  - `func DestinationOf(it item.Item, items []item.Item) Destination` — 08「親がない」(C3a-14)。Epic なら `Parent: nil`。それ以外は、同じプロジェクトの削除されていない親のない Epic(自分を除く)を `item.SortSiblings` で並べた先頭。なければ `NeedEpic: true`
  - `func EpicID(p project.ID, used func(change.ID) bool) change.ID` — C3a-15
  - `const RepairEpicTitle = "修復で作った Epic"`
  - app: 輪を切る 1 件ごとに 1 回の `Write`: 必要なら「修復で作った Epic」の作成(`item.Create`、ワークスペースに `numbering.Draft(numbering.Items, n)`)、動かすものの `set place`(親だけ変える)と `set rank`(行き先の子の削除されていないものの末尾の `rank.After`)、Step なら `set kind`(Task)。プロジェクトが読めなければ(`port.ErrNotFound`)直さない。輪を切ったあと、輪のほかのアイテムで `!item.CanPlace(it.Kind, 親の種類)` のものも 1 件ずつ同じに動かす。Summary「{ID} を {行き先の ID} の下へ移動(親の循環)」

- [ ] **Step 1: 失敗するテストを書く**

domain:
- A(番号 5)の親が B(番号 7)、B の親が A → 輪 {A, B}、`Mover` は B
- 番号が同じ → 内部 ID が後の方
- 削除済みのアイテムを通る輪 → 輪とみなさない
- 輪が 2 つ → 内部 ID の小さいものを含む輪が先
- `DestinationOf`: プロジェクトに Epic E1(rank V)・E2(rank N) → E2(並びの先頭)。Epic がない → `NeedEpic`。動かすものが Epic → `Parent` nil
- `EpicID("01ABC…", 何もない)` → `"R1ABC…"`。`R…` が使われている → `"S1ABC…"`

app: サンプルの Epic HH-1 を生の行で Story に変え(`set kind`)、HH-1 の親を Task HH-2 にする(`set place`)。HH-2 の親は HH-1 なので輪 {HH-1, HH-2} ができ、親のない Epic はなくなる → `Run` のあと、番号の大きい HH-2 が「修復で作った Epic」(HH-3。内部 ID は `EpicID(SampleProject, …)`)の下へ動き、件名は「HH-2 を HH-3 の下へ移動(親の循環)」。続けて、輪に入っていた HH-1(Story)の親 HH-2 は Task で `CanPlace(Story, Task)` が false なので、HH-1 も HH-3 の下へ動く(件名「HH-1 を HH-3 の下へ移動(親の循環)」)。もう一度 `Run` しても行が増えない。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/repair/ ./internal/app/repairing/ -run 'Cycle|Destination|EpicID'`
Expected: FAIL

- [ ] **Step 3: 実装する**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/repair/ ./internal/app/repairing/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git commit -am "feat: 同期でできた親の循環を切る"
```

---

### Task 12: (5) 親とプロジェクトが違う子を、親のプロジェクトへ移す

**Files:**
- Create: `internal/domain/repair/place.go`
- Create: `internal/app/repairing/place.go`
- Modify: `internal/app/repairing/run.go`
- Test: `internal/domain/repair/place_test.go`、`internal/app/repairing/place_test.go`

**Interfaces:**
- Produces:
  - `type Mismatch struct { Child item.ID; Project project.ID }`
  - `func ProjectMismatch(items []item.Item) (Mismatch, bool)` — C3a-16。深さの浅い順、同じなら内部 ID の順の最初の 1 件。ワークスペースが違う子は返さない
  - app: 1 件ごとに 1 回の `Write`、`set place`(プロジェクトだけ親のものにする)。Summary「{ID} をプロジェクト「{名前}」に移動(親に合わせる)」({名前} はプロジェクトのタイトル)。なくなるまで読み直して繰り返す

- [ ] **Step 1: 失敗するテストを書く**

domain: 親が P2、子が P1 → `{子, P2}`。孫も P1 → 子が先(浅い順)。親と別のワークスペースの子 → 返さない。削除済みの子・削除済みの親 → 返さない。
app: サンプルにプロジェクト HH-P2 を作り、Epic HH-1 を HH-P2 へ `set place` で移す(Task HH-2 と、その下の Step は HH-P1 のまま) → `Run` のあと HH-2 と Step のプロジェクトが HH-P2、番号は変わらない。件名「HH-2 をプロジェクト「{HH-P2 のタイトル}」に移動(親に合わせる)」。もう一度 `Run` しても行が増えない。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/repair/ ./internal/app/repairing/ -run Mismatch`
Expected: FAIL

- [ ] **Step 3: 実装する**

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/repair/ ./internal/app/repairing/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git commit -am "feat: 同期で、親とプロジェクトが違う子を親のプロジェクトへ移す"
```

---

### Task 13: 2 台の場面テスト

**Files:**
- Create: `cmd/hedgehug/repair_scenario_test.go`
- Create: `cmd/hedgehug/repair_helpers_test.go`(2 台のマシンを作る、書く、同期する、ファイルを比べるヘルパー。gitstore の `sync_helpers_test.go` と同じ形。remote は `t.TempDir()` の bare リポジトリのパス(C3a-20)。`Remote.Repair` は `repairing.Run`)

**Interfaces:**
- Consumes: `gitstore.New`・`Create`・`(*Stores).Attach`・`NewReplicator`、`apptest.Sample`、`repairing.Run`

- [ ] **Step 1: 場面テストを書く**

各場面は、A と B が同じ remote から始め、それぞれ書いてから、A 同期 → B 同期 → A 同期の順に同期し、両方のファイルがバイト単位で同じで、作業ツリーがきれいなことを確かめる。続けて、B を先に同期する順でも同じ値になる(同じ場面をもう一度作って比べる)。
- 削除と変更: A が HH-2 を削除、B が HH-2 のタイトルを変える → HH-2 は削除済みでなく、タイトルは B の値
- 親の削除と親の変更: A が HH-1 を削除(HH-2 も一緒)、B が HH-1 のタイトルを変える → HH-1・HH-2 とも削除済みでない
- 親の削除と子の変更: A が HH-1 を削除、B が HH-2 のタイトルを変える → HH-1・HH-2 とも削除済み
- 親の削除と子の追加: A が HH-1 を削除、B が HH-1 の下に Task を作る → 作った Task も削除済みで、`Deleted.Op` が HH-1 の削除の op
- 番号の重なり: A と B がそれぞれ Task を作る(どちらも HH-3) → 先に送った方が HH-3、もう一方が HH-4。2 つの同期の順を入れ替えると、入れ替わる(先に送った方が残る)
- 親の循環: A が生の行で HH-1 を Story にして親を HH-2 にし(Task 11 の app のテストと同じ輪)、B は同じ時期に HH-2 のタイトルを変える → 同期のあと輪が切れ、「修復で作った Epic」の内部 ID・番号を含めて両方で同じ
- プロジェクトの違い: A が HH-1 を HH-P2 へ移し、B が HH-1 の下に Task を作る → B の Task も HH-P2
- 3 台目: C を clone して同期しても、何も書かない(コミットが増えない。Review Focus 2)
- 2 台が同時に同じ修復: A と B が同じ重なりを含む状態で、両方が合わせて修復し、片方が送れずにやり直す → 番号が 2 回進まない

- [ ] **Step 2: 走らせる**

Run: `go test ./cmd/hedgehug/ -run TestRepairScenario -v`
Expected: PASS。落ちたら、落ちた場面の修復の Task に戻って直す(テストを書き換えて通さない)。

- [ ] **Step 3: Commit**

```bash
git add cmd/hedgehug/repair_scenario_test.go cmd/hedgehug/repair_helpers_test.go
git commit -m "test: 同期の修復を 2 台の場面で確かめる"
```

---

### Task 14: 仕上げの検証と、計画の末尾の記録

**Files:**
- Modify: `docs/superpowers/plans/2026-11-14-hedgehug-track-c3a.md`(末尾に「実装で計画から変えたこと」)

- [ ] **Step 1: まとめて確かめる**

Run: `make lint && make fmt-check && make test && golangci-lint run --build-tags e2e ./...`
Expected: すべて成功

- [ ] **Step 2: 計画から変えたことを末尾に書く**

実装で計画と違えた点(名前・置き場所・決定の変更)を、理由と一緒に箇条書きにする。なければ「なし」と書く。

- [ ] **Step 3: Commit**

```bash
git commit -am "docs: C3a の実装で計画から変えたことを残す"
```
