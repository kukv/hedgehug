# hedgehug トラック C5c2(Issue のリンク・メンバーの重複の同期の修復)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 同期の修復 (11) のうち、Issue のリンク(1 つの Issue に 1 つのアイテム、1 つのアイテムに 1 つの Issue)とメンバー(ログイン名で 1 人)の重複を直す。あとから同期した方の Issue のリンクを外してリンクした人に「同期の修復」の通知を 1 回だけ出し、メンバーは大文字小文字だけが違う綴りの行を外して 1 行にする(通知なし)。

**Architecture:** 直すものの決め方は `internal/domain/repair/links.go` に置き、残す 1 件の比べ方は C5c1 の `keeper` と共通にする。値 1 つの今の状態を決めた行は `change.ListRow` で読み、GitHub の改名の追従で付け直したリンクは `itemlink.Origin` で人がリンクした行までたどる。修復の段 `unlinkIssues`・`mergeMembers` は `internal/app/repairing/links.go` に置き、`steps` の最後(`renameDuplicates` のあと)に足す。通知は `internal/app/noticing/repair_issues.go` の `issueUnlinks` を `kinds` の最後に足す。`port` と gitstore は変えない。

**Tech Stack:** Go 1.27.3(go-git v5、modernc.org/sqlite)。web は変えない。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`
  - `:254-266`(1 つしかないものが 2 つになったとき)
  - `:347-353`(修復の決まり・順)
  - `:519`・`:526`(アクティビティの文「{ID} の {リポジトリ}#{番号} のリンクを解除」「メンバー {ログイン名} の重複を統合」)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/07-github.md` `:244`(Issue の 1 対 1、`Same` の比べ方)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/08-stores-maintenance.md` `:284`(メンバーの重複)・`:426-427`(診断の同じ直し方)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` `:214`(同期の修復の表の (11))
- `2026-11-15-hedgehug-track-c5c1.md`(C5c1-4 残す 1 件、C5c1-9〜C5c1-11 通知の見分け方・宛先・鍵)、`2026-11-14-hedgehug-track-c4b2.md`(C4b2-6〜C4b2-12 通知の仕組み)
- `2026-11-12-hedgehug-track-c.md` の C5c の行

## Global Constraints

土台・トラック C・C4a・C4b1・C4b2・C5a・C5b1・C5b2・C5c1 の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限は `internal/archtest/size.go` のとおり。
  - Go ファイルは 300 行、テストは 600 行まで。関数は 60 行、認知的複雑度は 15 まで。
  - パッケージのテスト以外のファイルは 20 個まで。`internal/infra/gitstore`(直下)と `internal/app/port` は満杯で、この計画では触らない。
  - 足すのは `domain/repair/links.go`・`app/repairing/links.go`・`app/noticing/repair_issues.go` だけにする(`noticing` は 19 → 20 個で満杯になる)。
- app の機能どうしは import しない。`noticing` は `repairing` を import しない。
- 実行者は `change.ActorSync`。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(`go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 修復の保存の件名(06 `:519`・`:526`): Issue は「{ID} の {リポジトリ}#{番号} のリンクを解除」(`{リポジトリ}#{番号}` は `itemlink.Ref.Display()`)、メンバーは「メンバー {ログイン名} の重複を統合」。
- 通知の本文:
  - 1 つの Issue に 2 つのアイテム(06 `:266`): 「Issue {リポジトリ}#{番号} は {ID} にリンクされているため、{あなたの ID} からリンクを外しました」
  - 1 つのアイテムに 2 つの Issue(ユーザーの決定、2026-11-15。06 に足す): 「{ID} には Issue {残ったリポジトリ}#{番号} がリンクされているため、Issue {外したリポジトリ}#{番号} のリンクを外しました」
- C4d(アクティビティ)・C5d(時計のずれ)は並行で進む。`noticing` の `kinds` や `repairing` の `steps` が重なったら、develop にあとから入る方が合わせる。
- 手元の検証は 1 回で足りる(CI でも流れる)。`make lint`・`make test`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C5c2-1 | 範囲は Issue のリンクとメンバー。名前・接頭辞・キーは C5c1 で入った | ユーザーの決定(2026-11-15、C5c を 2 本に分ける) |
| C5c2-2 | Issue の重なりを数える範囲: Store の削除されていないアイテム(`item.FromLog` の `Deleted == nil`)のリンクのうち、種別が `itemlink.Issue` のもの。比べ方は `itemlink.Ref.Same`(番号と、大文字小文字を区別しないリポジトリ)。削除済みのアイテムと別の Store のアイテムは数えない | 保存形式 `:214`「削除済みは数えない」。同期は 1 つの Store しか合わせない。診断の `repair.issueTwice` と同じ範囲。07 `:244` の削除済み・別の Store も調べる検査は、書き込みのときの断り |
| C5c2-3 | 「リンクを決めた行」「綴りを決めた行」: 複数の値を持つ項目の値 1 つの、`add`・`remove` の先端のうち勝った行(`change.Resolve` の `resolveList` と同じ選び方)。いまある値なので `add` の行(作成の初期値にだけある値なら作成の行)。`change.ListRow` として足す | C5c1-3(名前を決めた行)の、複数の値を持つ項目への当てはめ。外して付け直したリンクは、付け直した行で決まる |
| C5c2-4 | 外す Issue のリンク: C5c2-2 のリンクを、C5c1-4 の `keeper` と同じ比べ方(リンクした行(C5c2-17)が自分の側(`Facts.Side == 1`)にないものが先、行の `at` が古い順、同じなら行の ID の小さい順)で並べ、先頭から、そのアイテムも、その Issue(`Same`)も、まだ残すと決まっていなければ残し、どちらかが決まっていれば外す。1 つの Issue に 2 つのアイテム、1 つのアイテムに 2 つの Issue、その組み合わせを 1 回の並びで決める。比べ方は `keeper` と共通の関数にする | 06 `:256`「番号の重なりと同じ決め方」。外したリンクは、どれも残したリンクの 1 本と重なる |
| C5c2-5 | Issue の書く行: アイテムのログに `itemlink.Set.Detach` の行(Issue は `remove links {Ref}` の 1 行。解除の記録には足さない)。外すリンク 1 本を 1 回の保存にし、C5c2-4 の並びの順に書く。件名の {ID} はアイテムの画面の ID | 保存形式 `:214`「`remove`」、C3a-18(1 件 1 保存)、07 `:244`(Issue は解除の記録を持たない) |
| C5c2-6 | メンバー: メンバーの一覧のログ(`change.Ref{Kind: change.KindMembers}`)のいまある綴り(`change.ListOf(log, "members")` のうち `member.ValidLogin` を通るもの)を小文字でまとめ、2 つ以上の組ごとに、綴りを決めた行(C5c2-3)に `keeper` を当てて残す綴りを決め(tie は行の ID)、ほかの綴りに `member` の `remove members {綴り}` を書く。1 組を 1 回の保存にし、組は小文字のログイン名の順。件名の {ログイン名} は残した綴り。表示名は `name/<小文字のログイン名>` で持つので書かない(06 `:264`「表示名は残した行のもの」は、書かずにそうなる) | 06 `:264`、08 `:284`、06 `:526`(アクティビティの 1 行が 1 組を指す)。ログイン名は英数字・ハイフン・アンダースコアだけなので、小文字にそろえると `strings.EqualFold` と同じになる |
| C5c2-7 | 段の順: `renameDuplicates` のあとに `unlinkIssues` → `mergeMembers` | 06 `:262-264` の表の順、06 `:353` |
| C5c2-8 | 通知の見分け方: 修復の保存(`Repair`)のうち `restore` の行を含まないもので、アイテムの行が `T == Remove`・`F == "links"`・値の種別が Issue のもの。同期の修復でリンクを `remove` するのは C5c2 だけ | C4b2-8・C5c1-9 |
| C5c2-9 | 宛先: `remove` の行の `prev` が指す `add` の行(修復を書いたときのリンクを決めた行。`prev` が複数の `add` の行を指すとき(2 台が同じアイテムに同じ Issue を足したとき)は、その行だけで `change.ListRow` の勝った行)から、`itemlink.Origin` でたどったリンクした行(C5c2-17)の `by`。実行者が人で、あなた(`same`)のときだけ作る。`prev` が指す `add` の行がなければ作らない | 06 `:266`「直したものをリンクした人」、C5c1-10 |
| C5c2-10 | 本文の選び方(判定のときの Store の状態で決める):<br>(a) 外したリンクと同じ Issue(`Same`・種別 Issue)をリンクした、削除されていない別のアイテムがあれば、06 `:266` の本文。{ID} はそのアイテム(2 つ以上なら内部 ID が最も小さいもの)、{あなたの ID} は外したアイテム<br>(b) (a) がなく、外したアイテムにいま Issue のリンクがあれば、1 つのアイテムに 2 つの Issue の本文。{ID} は外したアイテム、{残ったリポジトリ}#{番号} はいまのリンク<br>(c) どちらでもなければ作らない(修復のあとの別の保存で、残したリンクも外れたとき) | 06 `:266`、ユーザーの決定(2026-11-15)。(a) と (b) の両方に当たるとき(アイテム A の X を外したのが、B が X を持ち、A が Y を持つから)は、どちらも正しいので (a) にする |
| C5c2-11 | 鍵: `repair:issue:{アイテムの内部 ID}:{Ref.Key()}:{リンクした行(C5c2-9 で決めた行)の ID}`(修復の op を含めない。2 台が同じ修復を書いても 1 件) | C5c1-11 と同じ作り |
| C5c2-12 | 開く先: 外したアイテム(`entityOf` の `base`。`Workspace`・`Item`・`Display`) | 06 `:458`、C4b2-10 |
| C5c2-13 | メンバーの統合は通知を作らない。どの種類の判定も、メンバーの統合の保存から通知を作らないことをテストで確かめる | 06 `:266` |
| C5c2-14 | 閲覧のみの Store では修復しない(既存の `Run` の呼び方のまま) | 06 `:349` |
| C5c2-15 | e2e は足さない。2 台の場面は `cmd/hedgehug` の本物の gitstore と state.db で確かめる | C5c1-15 と同じ |
| C5c2-16 | 06 `:266` に (b) の本文を足し、保存形式 `:214` に Issue のリンクとメンバーの見つけ方と書く行を足す(Task 6) | ユーザーの決定(2026-11-15) |
| C5c2-17 | リンクした行: リンクを決めた行(C5c2-3)が、GitHub からの取り直しでリポジトリの改名・移管を追った行(`itemlink.Set.Rename`。同じ `op` に、種別と番号が同じで別のリポジトリの `remove links` がある `add links`)なら、その `remove` の行の `prev` が指す `add` の行(複数なら `change.ListRow` の勝った行)へ、改名でない行に着くまでたどる(`itemlink.Origin`)。C5c2-4 の並び・C5c2-9 の宛先・C5c2-11 の鍵は、この行で決める | 06 `:256`「あとから同期した方」、`:266`「リンクした人」。改名の追従はリンクではなく取り直し(07、D3-11)なので、リンクした時刻と人を変えない。改名前の名前で書き込みの検査(07 `:244` の `Same`)を通った同じ Issue が、取り直しのあとに 2 つのアイテムにそろって重なることがある |
| C5c2-18 | 診断の Issue の二重リンク(`repair.issueTwice`。検索の `is:invalid`)は、`Ref.Key()`(リポジトリの大文字小文字まで同じ)で比べていたのを、`Ref.Same` と同じ比べ方にする(Task 5) | ユーザーの決定(2026-11-15、案 1。C5c2 の PR で直す)。書き込みの検査(07 `:244`)・同期の修復(C5c2-2)と同じものを二重リンクとするため |

## Review Focus

1. **リポジトリの大文字小文字だけが違う Issue:** 太郎さんが HH-1 に `Kukv/Web#5`、花子さんが HH-2 に `kukv/web#5` をリンクした。同じ Issue なので、あとから届いた方を外す。
   - テスト: Task 2 の `TestIssueUnlinksSameIgnoresCase`
2. **削除済みのアイテムが持つ Issue:** 削除済みの HH-3 が kukv/web#5 を持ち、HH-1 にも kukv/web#5 が同期で届いた。数えないので外さない。
   - テスト: Task 3 の `TestUnlinkIssuesSkipsDeleted`
3. **重なりの組み合わせ:** HH-1 に自分の側で X、remote の側で Y、HH-2 に remote の側で X をリンクした。外すのは HH-1 の X だけで、HH-1 は Y、HH-2 は X が残る。
   - テスト: Task 2 の `TestIssueUnlinksChain`
4. **PR は数えない:** HH-1 と HH-2 が同じ PR kukv/web#7 にリンクし、HH-1 は Issue と PR を 1 本ずつ持つ。何も外さない。
   - テスト: Task 2 の `TestIssueUnlinksIgnoresPR`
5. **同じ綴りのメンバーを 2 台が足した:** 2 台が同時に「jiro」を取り込んだ。一覧は 1 行のままで、何も書かない。担当者「JIRO」のアイテムは、統合のあとも残した綴りのメンバーに結びつく。
   - テスト: Task 2 の `TestMemberMergesSameSpelling`、Task 3 の `TestMergeMembers`

---

### Task 1: 値 1 つの状態を決めた行(`change.ListRow`)・リンクした行(`itemlink.Origin`)と、残す 1 件の比べ方の共通化

**Files:**
- Modify: `internal/domain/change/current.go`(`ListRow` を足す)、Test: `internal/domain/change/current_test.go`
- Modify: `internal/domain/repair/duplicates.go`(`keeper` の比べ方を `older` に切り出す)

**Interfaces:**
- Produces(`change`): `func ListRow(log []Record, f string, v json.RawMessage) (Record, bool)`: C5c2-3。項目 f の値 v(比べるときは `canonical` にそろえる)の `add`・`remove` の先端のうち `winner` の行。v が作成の初期値にだけあれば作成の行。どちらもなければ false。
- Produces(`itemlink`、Create: `internal/domain/itemlink/origin.go`、Test: `internal/domain/itemlink/origin_test.go`): `func Origin(log []change.Record, add change.Record) change.Record`: C5c2-17。add が改名の行でなければ add のまま。たどった先が見つからなければ、そこまでの行。`prev` が輪になっていても止まる(たどった行を覚える)。
- Produces(`repair`、パッケージの中だけ): `func older[T any](row func(T) change.Record, tie func(T) string, f Facts) func(a, b T) int`: C5c2-4 の並び(自分の側でないものが先 → `at` → tie)。`keeper` は、これで並べた先頭を返す形にする(結果は変えない)。

- [ ] **Step 1: 失敗するテストを書く**
  - `TestListRow`: `add links A`(at 1)→ `remove links A`(at 2)→ `add links A`(at 3)の log で、A は at 3 の行。`add links B` の値の JSON のキーの順を変えた v でも同じ行。ない値は false。作成の初期値 `{"members":["x"]}` の x は作成の行。
  - `TestOrigin`: taro が `Kukv/Web#5` をリンク(`Attach`)→ GitHub の実行者で `Rename(…, "kukv/web2", …)` → さらに `Rename(…, "kukv/web3", …)`。`kukv/web3#5` の `add` の行から taro の `add` の行に着く。改名でない `add` はそのまま返る。PR の改名も同じ。
  - 既存の `TestKeeper…`・`TestRenumbers`・`TestNameRenames…` が変わらず通る。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/domain/change/` Expected: FAIL(`undefined: ListRow`)
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/domain/... && golangci-lint run ./internal/domain/...` Expected: PASS・指摘なし
- [ ] **Step 5: コミット**

```bash
git add internal/domain/change internal/domain/itemlink internal/domain/repair
git commit -m "feat(change): 値 1 つの状態を決めた行と、改名を追う前にリンクした行を返す(C5c2)"
```

---

### Task 2: 外す Issue のリンクと統合するメンバーの決め方(`domain/repair/links.go`)

**Files:**
- Create: `internal/domain/repair/links.go`、Test: `internal/domain/repair/links_test.go`

**Interfaces:**
- Consumes: Task 1 の `older`、`itemlink.Ref`・`Ref.Same`
- Produces(`repair`):
  - `type IssueLink struct { Item item.ID; Ref itemlink.Ref; Linking change.Record }`: C5c2-2 の範囲のリンク 1 本。Linking はリンクした行(C5c2-17。呼ぶ側が `itemlink.Origin` でたどって渡す)。
  - `func IssueUnlinks(all []IssueLink, f Facts) []IssueLink`: C5c2-4。外すリンクを、並びの順に返す。重ならなければ空。
  - `type Spelling struct { Login string; Row change.Record }`: メンバーの綴り 1 つと、綴りを決めた行。
  - `type MemberMerge struct { Keep string; Drop []string }`
  - `func MemberMerges(all []Spelling, f Facts) []MemberMerge`: C5c2-6。組は小文字のログイン名の順、Drop は綴りの辞書順。重ならなければ空。

- [ ] **Step 1: 失敗するテストを書く**
  - `TestIssueUnlinksTwoItems`: HH-1 と HH-2 が kukv/web#5。HH-2 の行が自分の側 → HH-2 のリンクだけを返す。`Facts` が空 → `at` があとの方。
  - `TestIssueUnlinksOneItem`: HH-1 が #5(remote の側)と #9(自分の側)→ #9 だけ。
  - `TestIssueUnlinksChain`: HH-1 が X(自分の側)と Y(remote の側、at 1)、HH-2 が X(remote の側、at 2)→ HH-1 の X だけ。
  - `TestIssueUnlinksSameIgnoresCase`: `Kukv/Web#5` と `kukv/web#5` → 1 本外す。
  - `TestIssueUnlinksIgnoresPR`: 呼ぶ側が Issue だけを渡す前提はなく、種別 PR の `IssueLink` が混じっても数えない(HH-1 の PR #7 と HH-2 の PR #7 → 空)。
  - `TestMemberMerges`: 「Jiro」(remote の側)と「jiro」(自分の側)→ `{Keep: "Jiro", Drop: ["jiro"]}`。3 つの綴り → Drop が 2 つ。2 組 → 小文字の順。
  - `TestMemberMergesSameSpelling`: 綴りが 1 つずつ(「jiro」「taro」)→ 空。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/domain/repair/` Expected: FAIL(`undefined: IssueUnlinks` ほか)
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/domain/... && golangci-lint run ./internal/domain/...` Expected: PASS・指摘なし
- [ ] **Step 5: コミット**

```bash
git add internal/domain/repair
git commit -m "feat(repair): 重なった Issue のリンクと大文字小文字だけが違うメンバーの直し方を決める規則を足す(C5c2)"
```

---

### Task 3: 修復の段(`unlinkIssues`・`mergeMembers`)

**Files:**
- Create: `internal/app/repairing/links.go`
- Modify: `internal/app/repairing/run.go`(`steps` の `renameDuplicates` のあとに `unlinkIssues`・`mergeMembers`)
- Modify: `internal/app/repairing/snapshot.go`(`appendTo` に `change.KindMembers` → `tx.WriteMembers().Append(drafts...)`)
- Test: `internal/app/repairing/links_test.go`(既存の `newEnv`・`e.write`・`e.run`・`wantOps` を使う)

**Interfaces:**
- Consumes: Task 1 の `change.ListRow`・`itemlink.Origin`、Task 2 の `repair.IssueUnlinks`・`MemberMerges`、`readSnapshot`(アイテムのログと画面の ID)、`itemlink.FromLog`・`Set.Detach`
- Produces:
  - `func unlinkIssues(ctx context.Context, w port.RepairWriter) error`(`step`): C5c2-2〜C5c2-5。
  - `func mergeMembers(ctx context.Context, w port.RepairWriter) error`(`step`): C5c2-6。

- [ ] **Step 1: 失敗するテストを書く**(どれも、一方を自分の側、他方を remote の側で書く)
  - `TestUnlinkIssuesTwoItems`: HH-1 に kukv/web#5(remote の側)、HH-2 に kukv/web#5(自分の側)→ 保存が 1 つで件名「HH-2 の kukv/web#5 のリンクを解除」、HH-2 のログの最後が `remove links`。HH-2 の `itemlink.FromLog` にリンクがない。もう一度 `Run` しても何も書かない。
  - `TestUnlinkIssuesOneItem`: HH-1 に #5(remote の側)と #9(自分の側)→ 件名「HH-1 の kukv/web#9 のリンクを解除」。
  - `TestUnlinkIssuesSkipsDeleted`: 削除済みの HH-2 が kukv/web#5、HH-1 に kukv/web#5 → 何も書かない。
  - `TestUnlinkIssuesRelinked`: 共通の祖先で HH-1 が #5 を持ち、自分の側で外して付け直し、remote の側で HH-2 に #5 → 付け直した行は自分の側なので、HH-1 の方を外す(C5c2-3)。
  - `TestUnlinkIssuesRenamed`(C5c2-17): 共通の祖先で taro が HH-1 に `kukv/web#5` をリンクし、自分の側で GitHub の実行者がそれを `kukv/web2#5` に付け直した(`Rename`)。remote の側で hanako が HH-2 に `kukv/web2#5` をリンクした → 外すのは HH-2 の方(HH-1 のリンクした行は共通の祖先の taro の行)。
  - `TestMergeMembers`: 自分の側で `member.Join("jiro")`、remote の側で `member.Join("Jiro")` → 保存が 1 つで件名「メンバー Jiro の重複を統合」、メンバーのログの最後が `remove members "jiro"`、`member.FromLog` が「Jiro」の 1 行。担当者が「JIRO」のアイテムはそのまま。2 台が同じ綴り「jiro」を足したときは何も書かない。もう一度 `Run` しても何も書かない。
  - `TestDuplicatesStepsOrder`: 1 回の `Run` で、ワークスペースの名前・Issue のリンク・メンバーの重なりがある → 件名が C5c2-7 の順。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/repairing/` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/repairing/ ./internal/infra/gitstore/... && golangci-lint run ./internal/app/...` Expected: PASS・指摘なし
- [ ] **Step 5: コミット**

```bash
git add internal/app/repairing
git commit -m "feat(repairing): 重なった Issue のリンクを外し、大文字小文字だけが違うメンバーを同期で統合する(C5c2)"
```

---

### Task 4: Issue のリンクを外した修復の通知(`issueUnlinks`)

**Files:**
- Create: `internal/app/noticing/repair_issues.go`
- Modify: `internal/app/noticing/repair.go`(`kinds` の `duplicates` のあとに `issueUnlinks`)
- Test: `internal/app/noticing/repair_issues_test.go`(`fixture` の `repairSave` を使う)

**Interfaces:**
- Consumes: Task 1 の `change.ListRow`・`itemlink.Origin`、既存の `hasRestoreRow`・`logOf`・`entityOf`・`same`、`itemlink.FromLog`、`port.LiveWorkspaces`・`port.LiveProjects`・`port.LiveItems`
- Produces: `func issueUnlinks(r port.Reader, b batch, i int, me string) ([]port.Notice, error)`: 見分け方は C5c2-8、宛先は C5c2-9、本文は C5c2-10、鍵は C5c2-11、開く先は C5c2-12。外した行ごとに 1 件。

- [ ] **Step 1: 失敗するテストを書く**(あなたは `hanako`)
  - `TestIssueUnlinksTwoItems`: hanako が HH-2 にリンクした kukv/web#5 を外した修復、HH-1 が kukv/web#5 を持つ → 本文「Issue kukv/web#5 は HH-1 にリンクされているため、HH-2 からリンクを外しました」・`Item == HH-2 の内部 ID`・`Display == "HH-2"`・`Source == "repair:issue:{HH-2 の内部 ID}:issue:kukv/web#5:{add の行の ID}"`
  - `TestIssueUnlinksOneItem`: hanako が HH-1 にリンクした #9 を外し、HH-1 に #5 が残る → 「HH-1 には Issue kukv/web#5 がリンクされているため、Issue kukv/web#9 のリンクを外しました」
  - `TestIssueUnlinksPrefersOtherItem`: HH-1 の X を外し、HH-2 が X、HH-1 が Y を持つ → (a) の本文
  - `TestIssueUnlinksNoHolder`: 外したあと、HH-1 にも別のアイテムにも Issue がない → 作らない
  - `TestIssueUnlinksRecipient`: リンクしたのが taro → 作らない。
  - `TestIssueUnlinksRenamed`(C5c2-17): hanako がリンクし、GitHub の実行者が改名を追って付け直したリンクを外した修復 → hanako に作る。鍵は hanako の `add` の行の ID
  - `TestIssueUnlinksBothSidesAdded`(C5c2-9): `remove` の行の `prev` が、taro と hanako の同じ Ref の `add` の行 2 つを指す → `ListRow` で勝った方の人にだけ作り、2 台で判定しても鍵は同じ
  - `TestIssueUnlinksSkipsOthers`(表): PR のリンクの `remove` を含む人の保存(修復でない)→ 作らない。`restore` の行と `remove links` を含む修復 → 作らない
  - `TestIssueUnlinksOnceAcrossMachines`: 行の ID が違う同じ修復の保存を 2 つ判定する → 受信箱は 1 行
  - `TestMemberMergeNoNotice`(C5c2-13): hanako が足した「jiro」を外した修復の保存を判定する → 受信箱は空(`wantNone`)
  - `TestIssueUnlinkSingleNotice`: Issue のリンクを外した修復の保存 1 つを判定する → 受信箱は 1 行(ほかの種類の判定が重ねて作らない)
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/app/noticing/ -run 'IssueUnlink|MemberMerge'` Expected: FAIL
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/app/... && golangci-lint run ./internal/...` Expected: PASS・指摘なし
- [ ] **Step 5: コミット**

```bash
git add internal/app/noticing
git commit -m "feat(noticing): 重なった Issue のリンクを外した修復を、リンクした人に知らせる(C5c2)"
```

---

### Task 5: 診断の Issue の二重リンクを、大文字小文字を区別せずに数える

C5c2-18 のとおり、検索の `is:invalid` が使う診断の Issue の二重リンクを、書き込みの検査・同期の修復と同じ比べ方(`itemlink.Ref.Same`)にする。

**Files:**
- Modify: `internal/domain/itemlink/itemlink.go`(`SameKey` を足す)、Test: `internal/domain/itemlink/itemlink_test.go`
- Modify: `internal/app/search/invalid.go:36`(`l.Key()` を `l.SameKey()` にする)
- Modify: `internal/domain/repair/problems.go:35`(`Issues` のコメントを「`itemlink` の `Ref.SameKey`」にする)
- Test: `internal/app/search/facts_test.go`

**Interfaces:**
- Produces(`itemlink`): `func (r Ref) SameKey() string`: 「{小文字のリポジトリ}#{番号}」。`a.Same(b)` と `a.SameKey() == b.SameKey()` が同じになる(種別は含めない)。

- [ ] **Step 1: 失敗するテストを書く**
  - `TestSameKey`: `{Issue, "Kukv/Web", 5}` と `{Issue, "kukv/web", 5}` が同じ。番号かリポジトリが違えば違う。
  - `TestInvalidIssueCase`(`facts_test.go`): HH-1 に Issue `Kukv/Web#5`、HH-2 に `kukv/web#5` をリンクした Store で、`is:invalid` が HH-1 と HH-2 を返す。
- [ ] **Step 2: 失敗を確かめる** — Run: `go test ./internal/domain/itemlink/ ./internal/app/search/` Expected: FAIL(`undefined: SameKey`)
- [ ] **Step 3: 実装する**
- [ ] **Step 4: 通ることを確かめる** — Run: `go test ./internal/domain/... ./internal/app/search/ && golangci-lint run ./internal/...` Expected: PASS・指摘なし
- [ ] **Step 5: コミット**

```bash
git add internal/domain/itemlink internal/domain/repair internal/app/search
git commit -m "fix(search): 診断の Issue の二重リンクを、リポジトリの大文字小文字を区別せずに数える(C5c2)"
```

---

### Task 6: 2 台の場面テストと、仕様・トラック C の計画の記録

**Files:**
- Create: `cmd/hedgehug/links_repair_scenario_test.go`
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md` `:266`
  - 「Issue のリンクでは「Issue {リポジトリ}#{番号} は {ID} にリンクされているため、{あなたの ID} からリンクを外しました」」のあとに「(1 つのアイテムに 2 つの Issue がリンクされたときは「{ID} には Issue {残ったリポジトリ}#{番号} がリンクされているため、Issue {外したリポジトリ}#{番号} のリンクを外しました」)」を足す。
- Modify: `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` `:214`
  - 見つけ方の最後に「Issue のリンクは、削除されていないアイテムの Issue のリンクを、リンクを決めた `add` の行で同じ順に並べ、アイテムか Issue がもう残すと決まったものと重なるリンクを外す。メンバーは、小文字にそろえて同じログイン名の綴りのうち、綴りを決めた `add` の行で 1 つを残す」を足す。
  - 書く行の最後に「Issue のリンクは `remove links`、メンバーは残さない綴りの `remove members`(1 組を 1 回の保存)」を足す。
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` の C5c の行
  - 「C5c2(Issue のリンク・メンバー)」を「C5c2(Issue のリンク・メンバー。計画: `2026-11-15-hedgehug-track-c5c2.md`)」にする。

**Interfaces:**
- Consumes: Task 1〜5、`cmd/hedgehug/repair_helpers_test.go` の `firstMachine`・`cloneMachine`・`writeAs`・`sync`・`noticed`・`sameFiles`・`item`、`repair_notice_scenario_test.go` の `wantRepair`・`wantNoRepair`、サンプルの HH-1(`apptest.SampleEpic`)・HH-2(`apptest.SampleTask`)、`itemlink.Set.Attach`・`member.Join`

- [ ] **Step 1: 場面テストを書く**(A = taro、B = hanako。本物の gitstore とテストごとの state.db)
  - `TestRepairIssueTwoItems`:
    1. A が最初の Store を送る。B が clone する。
    2. A が HH-1 に Issue kukv/web#5 をリンクする。B も同時に HH-2 に kukv/web#5 をリンクする。
    3. A が同期し、B が同期し、A がもう一度同期する。
    - 期待: どの同期もエラーにならない。2 台とも HH-1 だけが kukv/web#5 を持つ。2 台のファイルが同じ。B に「Issue kukv/web#5 は HH-1 にリンクされているため、HH-2 からリンクを外しました」が 1 行。A には出ない。
  - `TestRepairIssueOneItem`: 2 で A が HH-1 に #5、B が HH-1 に #9 をリンクする → HH-1 は #5 だけ。B に「HH-1 には Issue kukv/web#5 がリンクされているため、Issue kukv/web#9 のリンクを外しました」が 1 行。
  - `TestRepairMembers`: 2 で A が `member.Join("Jiro")`、B が `member.Join("jiro")` を書く → 2 台ともメンバーの一覧が「Jiro」の 1 行(taro・hanako を含めて 3 行)。2 台のファイルが同じ。どちらの受信箱にも同期の修復は出ない。B の修復の保存の件名が「メンバー Jiro の重複を統合」。
- [ ] **Step 2: 流して、足りない組み立てを直す** — Run: `go test ./cmd/hedgehug/ -run 'RepairIssue|RepairMembers'` Expected: PASS
- [ ] **Step 3: 仕様とトラック C の計画を書き直す**(上のとおり)。
- [ ] **Step 4: 全体を確かめる** — Run: `make lint && make test && golangci-lint run --build-tags e2e ./...` Expected: すべて PASS
- [ ] **Step 5: コミット**

```bash
git add cmd/hedgehug docs
git commit -m "test(sync): 2 台の場面で、重なった Issue のリンクとメンバーの修復を確かめ、仕様に書く(C5c2)"
```
