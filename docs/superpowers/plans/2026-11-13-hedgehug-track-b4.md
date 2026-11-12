# hedgehug トラック B4(自動化)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 03「自動化」を作る。ルールの作成・編集・削除・有効と無効、利用者のステータスの変更と Step の完了で動く実行、制約で止まったときの通知、「制約を無視して適用」の操作、最近の実行、ステータスの削除でのルールの追従を入れる。

**Architecture:**
- ルールの型・保存の行・検査・実行の規則は、新しい `domain/automation` に置く。ルールはワークスペースの `log.jsonl` の項目 `rules/<ID>/<項目>` に持ち、`automation.FromLog` で読む。`workspace.Settings` には載せない(決定 B4-2)。
- 操作は新しい `app/automations` に置く。アイテムの操作(`app/items`)は、保存の中で `automation.Run` を呼ぶ。
- 自動化の行は、操作と同じ保存の中に入る。行ごとに実行者を変えられるように、`change.Draft` に `Rule` を足す(決定 B4-1)。
- 通知は新しい口 `port.Inbox` に書く。本物の受信箱は C4 が作る。

**Tech Stack:** 土台と同じ(Go 1.27.3、huma v2、Svelte 5、orval、Vitest、Playwright)。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md`
  - この計画の正本は「自動化」の全節。
  - 冒頭の表のうち、「ルールの作成・編集・削除」「ルールの有効・無効」の行。
  - 「値の編集」の「削除」のうち、自動化の箇条。
  - 名前の重複の規則。
- 06「通知の種類」の「自動化」と「自動化が制約で止まった」の行、「自動化」の節、「アクティビティ」の「対象」の 538 行(人でない実行者の同じ変更)。
- 07「PR・Issue の状態で動く自動化」(きっかけの 4 つ。起こす側は D4)。
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`「変更記録」の `by`・`actor`、「アクティビティと履歴」。
- `docs/superpowers/plans/2026-11-12-hedgehug-track-b.md`(小計画の表の B4 の行、未決 3)。

**持ち場:**
- 新規: `internal/domain/automation`、`internal/app/automations`、`internal/adapter/httpapi/automations`、`web/packages/core/src/features/workspaces/automation.ts`、`web/app/src/screens/workspace-settings/AutomationPanel.svelte`・`RuleDialog.svelte`。
- 足すだけ: `change.Draft`・`change.Stamp`、`port.Inbox`(新しい口)、`apptest.Inbox`、`app/items` の `SetProperty`、`app/workspaces` の `DeleteValue`、gitstore の `workspace.yaml`、ui の `Switch`。

**待つもの:** なし。B3(#170)と A3 前半(#171)は develop に入っている。

## Global Constraints

- 土台・トラック B の計画の Global Constraints をすべて引き継ぐ。
- 依存の向き: `app/automations` は `domain` と `app/port` だけを import する。`app/items`・`app/workspaces` は `domain/automation` を使ってよい(domain なので)。app の機能どうしは import しない。
- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。`app/workspaces`・`app/apptest`・`infra/gitstore` は 20 個で満杯なので、ファイルを足さない(決定 B4-13)。
- 画面の文言・アクティビティの文・断りの文は、仕様 03・06 の文字どおりに書く。
- `api/openapi.yaml` と `web/packages/api/src/generated.ts` は手で直さない。`make generate` と `cd web && pnpm gen` で作り直し、同じコミットに入れる。
- 依存は足さない(足すなら版を明記し、公開から 7 日以上のものだけ)。
- 計画やテストに `ghp_` などのトークンらしい値を書かない。
- PR はトラックで 1 本(向き先 develop)。force push は拒否されるので、載せ直すときは別名のブランチで出す。

## この計画で決めたこと(仕様と既存の規則の当てはめ)

| # | 決めたこと | 根拠 |
|---|---|---|
| B4-1 | `change.Draft` に `Rule change.ID` を足す。空でなければ、`Stamp` はその行の実行者を `automation:<Rule>`、`by` を操作の人(`Operation.Actor.Login`)にする。`Rule` を持つ行は、人の操作(`ActorUser`)の中でだけ書け、ルールの ID が `change.ValidID` でなければ `ErrInvalidOperation`(偽物と gitstore の両方で `port.CheckRuleDrafts`) | 保存形式「変更記録」(`actor` は `automation:<ルールの内部 ID>`、`by` はきっかけの操作をした人)。03「変更は、操作と同じ保存の中に入る」。いまの `Write` は 1 回に実行者が 1 つ |
| B4-2 | ルールは `rules/<ID>/{name,trigger,trigger_status,all_merged,action,value,enabled}` の `set` で持ち、削除は `delete rules/<ID>`。`workspace.Settings` には載せず、`automation.FromLog(log)` で読む | `domain/automation` が `workspace` と `item` を使うので、`Settings` に載せると import が輪になる。テンプレート(B3-4)と同じ項目の分け方 |
| B4-3 | ルールの並びは内部 ID の順(作った順)。並べ替えの操作はない | 03「作ったルールは…一覧の末尾に入る」。テンプレートの B3-10 と同じ |
| B4-4 | きっかけの値は `pr_open`・`review_requested`・`pr_merged`・`issue_closed`・`status`・`steps_done`。することは `status`・`assignee`。「未割り当てにする」の値は `@none`(`automation.Unassigned`) | 03「きっかけとすること」の 6 つと 2 つ。`@none` は B3 の Step の担当者(`workspace.StepNone`)と同じ書き方 |
| B4-5 | 編集では有効・無効を変えない。作ったルールは有効で始まる | 03「作ったルールは、有効で始まる」「直して保存すれば、有効のまま保存できる」 |
| B4-6 | 値の参照(きっかけ・することのステータス、担当者のログイン名)が設定やメンバーにないとき、きっかけ・することが知らない値のときは `port.ErrNotFound` にする | B2-12(画面からは起きないので 404) |
| B4-7 | アイテムの操作から動くのは `items.SetProperty`(ステータス)だけ。`CheckStep` は `SetProperty` を通るので動く。作成・Step の追加・値の削除による置き換え・「制約を無視して適用」では動かさない | 03「実行の規則」の 2 つ目の箇条。いま develop にある利用者の操作は詳細パネル・ツリーのステータスと Step のチェック。カンバン・一括変更・コマンドパレットは、作るトラック(E ほか)が `SetProperty` か `automation.Run` を使う |
| B4-8 | 「Step がすべて完了したとき」は、利用者が変えた Step の値で判定する。自動化がその Step のステータスを変えても、判定に入れない | 03「自動化が行ったステータスの変更は、ほかのルールのきっかけにならない」 |
| B4-9 | `automation.StepsDone` は、変えた Step の変更前の値を `map[item.ID]change.ID` で受ける | 一括変更(E)で、同じ親の Step を 1 回で複数変えたときにも 1 回だけ動くように |
| B4-10 | 自動化の通知の本文は「「{ルール名}」で{プロパティ名}を {値} に変更しました」、アクティビティ・最近の実行の文は「「{ルール名}」で{プロパティ名}を {値} に変更」。値は `item.Display`(利用者の操作のアクティビティと同じ。未割り当ては「なし」) | 06「通知の種類」の「「{ルール名}」で{変更の内容}しました」、06 の 538 行の例「「PR マージで完了」でステータスを Done に変更」 |
| B4-11 | 「制約を無視して適用」は 1 回で保存する。ステータスの行はそのルールが書いた行(`ByRule`。実行者 `automation:<ルール>`、`by` は押した人)、アクティビティの文は「「{ルール名}」の変更を制約を無視して適用」。自動化は動かさない。すでにそのステータスなら何も保存しない。消したルールの名前も使える | 03「受信箱の「制約を無視して…」で適用した変更も、自動化が行った変更として、きっかけにならない」、03・06 の文言 |
| B4-12 | `port.Inbox` は `Add(ctx, Notice) error` だけ。`Notice` は種類・Store・ワークスペース・アイテム・ルール・本文・止まった理由・変更先のステータスを持つ。既読と一覧は C4 が足す。本物のアプリでは C4 まで何もしない実装(`cmd` の `noInbox`)、e2e の組み立ては `apptest.Inbox` | ユーザー決定(2026-11-13: B4 は口越しに通知を書くだけ、本物の受信箱は C4)。土台の決定 1(口は 1 つ 1 ファイル) |
| B4-13 | `app/apptest` に `inbox.go` を足すため、`stores_contract_lists.go` を `stores_contract_values.go` にまとめる | 上限 20 個。#162 の前例(書いた値がそのまま読める契約どうしをまとめた) |
| B4-14 | 通知は保存が成功したあとに書く。通知が書けなかったときは、そのエラーを返す(保存は残る) | 通知は Store の外(state.db)で、保存の取り消しはできない。エラーを隠さない |
| B4-15 | 最近の実行は、ワークスペースのすべてのアイテムのログから、実行者が自動化の行を集めて作る。同じアイテム・項目・変更前・変更後の行が複数あれば、最も早い 1 行だけにする。新しい順に 8 件。行は、相対時刻・アイテムの ID・文 | 03「最近の実行」、06 の 538 行。索引(E)はまだないので、いまの口で読む。性能は E への申し送り |
| B4-16 | 最近の実行の文のルール名は、消したルールの名前も引く(`automation.Names`) | 削除したルールの実行も、アクティビティには残るため |
| B4-17 | ステータスの削除の追従は `app/workspaces` の `DeleteValue` に足す。書き換えた・無効にしたルールごとに、削除した人の受信箱へ通知を 1 件書く。ほかの人のマシンが受け取った削除から通知を作るのは C4 | 03「削除」の自動化の箇条(削除した人のマシンは自分の操作で作る) |
| B4-18 | 一覧の説明(「{きっかけ} → {すること}」)は画面(core)で作る。担当者は表示名(なければログイン名)、未割り当ては「未割り当て」 | 03 の例「ステータスが Done になったとき → 担当者を 未割り当て に」 |
| B4-19 | 作成ダイアログで、きっかけを「ステータスが次になったとき」にしたときのステータスの初期値は、一覧の先頭のステータス | 03 は、することを切り替えたときの初期値を「一覧の先頭」と定めている。同じ規則を当てはめた |
| B4-20 | ダイアログの題は「ルールを作成」「ルールの編集」、ボタンは「キャンセル」(閲覧のみは「閉じる」)と「保存」 | B3 のテンプレートのダイアログと同じ |
| B4-21 | 編集の保存のトーストは出さない(作成は「ルールを作成しました」、削除は「ルールを削除しました」) | 03 の表のトーストの列。B3 のテンプレートの編集と同じ読み方 |
| B4-22 | 設定の画面は、自動化のサブタブを開いているときだけ `GET .../automation` を読む | 最近の実行はすべてのアイテムのログを読むので、ほかのサブタブの操作のたびに読まない |
| B4-23 | ui の `Switch` に `onchange` を足す。渡したときは、押しても見た目を変えず、`onchange(次の値)` だけ呼ぶ(読み直しで変わる) | 03「ON にせず(OFF のまま)」。断ったときに見た目が ON に残らないように |

### 範囲の外(申し送り)

| 項目 | 持ち場 |
|---|---|
| GitHub の 4 つのきっかけを起こすこと(取り直しと自動リンクの保存の中で `automation.Run` を呼ぶ。`AllMerged` の材料を数える。自動化の行の `by` をどうするか) | D4(トラック D の計画 73・86 行) |
| 受信箱の画面、通知のボタン(「制約を無視して…」は B4 の API `apply-automation-ignoring-workflow` を呼ぶ、「変更しない」)、既読 | C4 |
| ほかの人のマシンが、受け取ったステータスの削除から自動化の通知を作ること。アクティビティの画面と「自動化だけ」の絞り込み | C4 |
| 専用ルール、「このプロジェクトで動かさない」、使わないステータスで止まること(「このプロジェクトでは「{ステータス}」を使いません」) | B5(`automation.Run` に判定を足す) |
| 同期で削除された値を指すルールを無効にする修復、同じ名前のルールの「 (2)」 | C(同期の修復) |
| カンバンのドラッグ、一括変更、コマンドパレット、ツリーの Space キーから動かすこと | それぞれの画面のトラック(`SetProperty` を通すか、`automation.Run` と `StepsDone` を呼ぶ) |
| 削除済みのアイテムで動かないこと | アイテムの削除を作るトラック(A4)。いまは削除がない |
| 最近の実行の性能(すべてのアイテムのログを読む) | E(索引) |

---

## ファイルの地図

```
internal/domain/change/draft.go              Draft.Rule
internal/domain/change/stamp.go              行ごとの実行者
internal/app/port/stores.go                  CheckRuleDrafts
internal/app/port/inbox.go                   Inbox・Notice(新しい口)
internal/app/apptest/inbox.go                偽物(通知を覚える)
internal/app/apptest/stores.go               append で CheckRuleDrafts
internal/app/apptest/stores_contract_rules.go 照合の場面を足す
internal/app/apptest/stores_contract_values.go  stores_contract_lists.go をまとめる
internal/infra/gitstore/write.go             append で CheckRuleDrafts
internal/infra/gitstore/yaml.go・yaml_settings.go・yaml_templates.go  workspace.yaml の rules

internal/domain/automation/rule.go           型・項目名
internal/domain/automation/log.go            FromLog・Names・Drafts・DeleteDraft
internal/domain/automation/check.go          Normalize・Valid・Check・CheckSameStatus
internal/domain/automation/run.go            Event・Outcome・Run
internal/domain/automation/steps.go          StepsDone
internal/domain/automation/follow.go         StatusDeleteDrafts
internal/domain/automation/text.go           文言
internal/domain/automation/*_test.go

internal/app/automations/service.go          Service・edit・rulesOf
internal/app/automations/rules.go            Create・Update・Delete・SetEnabled
internal/app/automations/view.go            List(ルールと最近の実行)
internal/app/automations/apply.go            ApplyIgnoringWorkflow
internal/app/automations/*_test.go

internal/app/items/service.go                Service.Inbox
internal/app/items/setproperty.go            保存の中で automate
internal/app/items/automate.go               automate・notify
internal/app/workspaces/service.go           Service.Inbox
internal/app/workspaces/value_delete.go      ルールの追従と通知

internal/adapter/httpapi/automations/automations.go  Register
internal/adapter/httpapi/automations/dto.go
internal/adapter/httpapi/automations/automations_test.go
internal/adapter/httpapi/routes.go           Deps.Inbox、Register の 1 行、items・workspaces の Inbox
cmd/hedgehug/wire.go・wire_e2e.go            noInbox・apptest.Inbox

web/packages/core/src/model/workspace.ts     AutomationRule ほか
web/packages/core/src/port/workspaces.ts     automation・createRule ほか
web/packages/api/src/workspaces.ts           実装
web/packages/core/src/features/workspaces/automation.ts(+ test)
web/packages/core/src/features/workspaces/settings.ts   automation の読み込み
web/packages/ui/src/primitives/Switch.svelte onchange
web/app/src/screens/workspace-settings/AutomationPanel.svelte・RuleDialog.svelte・SettingsTab.svelte
web/e2e/automation.spec.ts
```

---

### Task 1: 自動化の行を同じ保存に書けるようにする(`Draft.Rule`)

**Files:**
- Modify: `internal/domain/change/draft.go`、`internal/domain/change/stamp.go`
- Modify: `internal/app/port/stores.go`
- Modify: `internal/app/apptest/stores.go`、`internal/infra/gitstore/write.go`
- Modify: `internal/app/apptest/stores_contract_rules.go`(照合の場面)
- Test: `internal/domain/change/record_test.go` に足す(600 行を超えるなら `stamp_test.go` を新しく作る)

**Interfaces:**
- Produces: `change.Draft.Rule change.ID`、`func (d Draft) ByRule(rule ID) Draft`、`port.CheckRuleDrafts(actor change.Actor, drafts []change.Draft) error`

- [ ] **Step 1: 失敗するテストを書く(Stamp)**

```go
// 自動化の行は、実行者が automation:<ルール>、by は操作の人になる。ほかの行は操作の実行者のまま。
func TestStampWritesTheRuleActor(t *testing.T) {
	meta := change.Meta{Op: "OP1", At: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC), Actor: change.User("hanako")}
	n := 0
	newID := func() change.ID { n++; return change.ID(fmt.Sprintf("R%d", n)) }
	user := change.NewSet("status", "A", "B")
	auto := change.NewSet("assignee", "taro", nil).ByRule("RULE1")
	recs := change.Stamp(nil, []change.Draft{user, auto}, meta, newID)
	if recs[0].Actor != change.User("hanako") || recs[0].By != "hanako" {
		t.Errorf("user row = %+v", recs[0])
	}
	want := change.Actor{Kind: change.ActorAutomation, Login: "hanako", Rule: "RULE1"}
	if recs[1].Actor != want || recs[1].By != "hanako" || recs[1].Op != "OP1" {
		t.Errorf("rule row = %+v", recs[1])
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/change/ -run TestStampWritesTheRuleActor`
Expected: FAIL(`ByRule` がない)

- [ ] **Step 3: 実装する**

`draft.go` の `Draft` に足す:

```go
type Draft struct {
	T    Type
	F    string
	From json.RawMessage
	To   json.RawMessage
	Root *bool
	// Rule は、自動化のルールが書いた行のルール(B4-1)。空なら操作の実行者の行。
	Rule ID
}

// ByRule は、自動化のルール rule が書いた行にする。
func (d Draft) ByRule(rule ID) Draft {
	d.Rule = rule
	return d
}
```

`stamp.go` の `Stamp` のループを次にする:

```go
	for _, d := range drafts {
		actor := meta.Actor
		if d.Rule != "" {
			actor = Actor{Kind: ActorAutomation, Login: meta.Actor.Login, Rule: d.Rule}
		}
		r := Record{
			ID: newID(), Op: meta.Op, At: meta.At, By: actor.By(), Actor: actor,
			T: d.T, F: d.F, From: d.From, To: d.To, Root: d.Root,
		}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/change/`
Expected: PASS

- [ ] **Step 5: 照合テストの場面を書く**

`stores_contract_rules.go` の照合の一覧(既存の場面の並び)に、次の 2 つを足す。関数の登録の仕方は、同じファイルの既存の場面に合わせる。

```go
// 自動化の行は、同じ保存の中で automation:<ルール> の実行者として読める(B4-1)。
func contractRuleRows(t *testing.T, s port.Stores, store port.StoreID) {
	op := port.Operation{Actor: change.User("taro"), Summary: "HH-2 のステータスを Done に変更"}
	err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(SampleTask,
			change.NewSet("status", SampleStatusTodo, SampleStatusDone),
			change.NewSet("assignee", nil, "taro").ByRule("01RULE0000000000000000001"))
	})
	if err != nil {
		t.Fatal(err)
	}
	var recs []change.Record
	_ = s.Read(t.Context(), store, func(r port.Reader) error {
		recs, err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: SampleTask})
		return err
	})
	last := recs[len(recs)-1]
	if last.Actor.Kind != change.ActorAutomation || last.Actor.Rule != "01RULE0000000000000000001" || last.By != "taro" {
		t.Errorf("rule row = %+v", last)
	}
	if recs[len(recs)-2].Op != last.Op {
		t.Error("the rule row must be in the same operation")
	}
}

// 人でない実行者の操作と、使えないルールの ID では、自動化の行を書けない。
func contractRuleRowsRefused(t *testing.T, s port.Stores, store port.StoreID) {
	cases := []port.Operation{
		{Actor: change.Actor{Kind: change.ActorSync}, Summary: "同期の修復"},
		{Actor: change.User("taro"), Summary: "x"},
	}
	rules := []change.ID{"01RULE0000000000000000001", "bad/id"}
	for i, op := range cases {
		err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
			return tx.WriteItems().Append(SampleTask, change.NewSet("status", nil, "X").ByRule(rules[i]))
		})
		if !errors.Is(err, port.ErrInvalidOperation) {
			t.Errorf("case %d: err = %v", i, err)
		}
	}
}
```

- [ ] **Step 6: 失敗を確かめる**

Run: `go test ./internal/app/apptest/ ./internal/infra/gitstore/ -run Contract`
Expected: `contractRuleRowsRefused` が FAIL(断らない)

- [ ] **Step 7: `CheckRuleDrafts` を足して、両方の `append` から呼ぶ**

`port/stores.go`:

```go
// CheckRuleDrafts は、自動化の行(Draft.Rule)を書けるかを確かめる(B4-1)。自動化の行は人の操作の中でだけ書け、
// ルールの ID は change.ValidID。合わなければ ErrInvalidOperation を包んだエラー。
func CheckRuleDrafts(actor change.Actor, drafts []change.Draft) error {
	for _, d := range drafts {
		if d.Rule == "" {
			continue
		}
		if actor.Kind != change.ActorUser || !change.ValidID(d.Rule) {
			return fmt.Errorf("%w: rule row %q by %+v", ErrInvalidOperation, d.Rule, actor)
		}
	}
	return nil
}
```

`apptest/stores.go` と `gitstore/write.go` の `tx.append` の、`len(drafts) == 0` の検査の直後に足す:

```go
	if err := port.CheckRuleDrafts(t.meta.Actor, drafts); err != nil {
		return err
	}
```

- [ ] **Step 8: 通ることを確かめる**

Run: `go test ./internal/domain/change/ ./internal/app/... ./internal/infra/gitstore/`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/domain/change internal/app/port/stores.go internal/app/apptest internal/infra/gitstore/write.go
git commit -m "feat(change): 自動化のルールが書いた行を、操作と同じ保存に書けるようにする(B4 Task 1)"
```

---

### Task 2: 受信箱の口 `port.Inbox` と偽物

**Files:**
- Create: `internal/app/port/inbox.go`、`internal/app/apptest/inbox.go`
- Modify: `internal/app/apptest/stores_contract_values.go`(`stores_contract_lists.go` の中身を移す)
- Delete: `internal/app/apptest/stores_contract_lists.go`
- Test: `internal/app/apptest/fakes_test.go` に足す

**Interfaces:**
- Produces:

```go
type NoticeKind string
const (
	NoticeAutomation        NoticeKind = "automation"
	NoticeAutomationStopped NoticeKind = "automation_stopped"
)
type Notice struct {
	Kind      NoticeKind
	Store     StoreID
	Workspace workspace.ID
	Item      item.ID
	Rule      change.ID
	Text      string
	Reason    string
	Status    change.ID
}
type Inbox interface { Add(ctx context.Context, n Notice) error }
// apptest
type Inbox struct { mu sync.Mutex; notices []port.Notice; Err error }
func (i *Inbox) Add(ctx context.Context, n port.Notice) error
func (i *Inbox) Notices() []port.Notice
```

- [ ] **Step 1: ファイルをまとめる**

`stores_contract_lists.go` の関数を、そのまま `stores_contract_values.go` の末尾へ移し、`stores_contract_lists.go` を消す(import はまとめる)。

Run: `go test ./internal/app/apptest/ ./internal/infra/gitstore/ ./internal/archtest/`
Expected: PASS(振る舞いは変わらない)

- [ ] **Step 2: 失敗するテストを書く**

`fakes_test.go` に足す:

```go
func TestInboxKeepsNotices(t *testing.T) {
	var in apptest.Inbox
	n := port.Notice{Kind: port.NoticeAutomation, Store: "S1", Text: "「完了で外す」で担当者を なし に変更しました"}
	if err := in.Add(t.Context(), n); err != nil {
		t.Fatal(err)
	}
	if got := in.Notices(); len(got) != 1 || got[0] != n {
		t.Errorf("Notices = %+v", got)
	}
	in.Err = errors.New("full")
	if err := in.Add(t.Context(), n); err == nil || len(in.Notices()) != 1 {
		t.Errorf("Add with Err = %v, %d", err, len(in.Notices()))
	}
}
```

- [ ] **Step 3: 失敗を確かめる**

Run: `go test ./internal/app/apptest/ -run TestInboxKeepsNotices`
Expected: FAIL(`apptest.Inbox` がない)

- [ ] **Step 4: 実装する**

`port/inbox.go`:

```go
package port

import (
	"context"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// NoticeKind は、通知の種類(06「通知の種類」)。B4 は自動化の 2 つだけを書く。ほかは C4 が足す。
type NoticeKind string

const (
	NoticeAutomation        NoticeKind = "automation"         // 自動化が変えた。ステータスの削除でルールが変わった
	NoticeAutomationStopped NoticeKind = "automation_stopped" // 自動化が制約で止まった
)

// Notice は、あなた宛ての通知 1 件。差出人は種類で決まる(自動化は「hedgehug」)。
type Notice struct {
	Kind      NoticeKind
	Store     StoreID
	Workspace workspace.ID
	Item      item.ID   // 対象のアイテム。ルールが変わった通知では空(開く先は設定の「自動化」)
	Rule      change.ID // 自動化のルール
	Text      string    // 06 の本文
	Reason    string    // 止まった理由(NoticeAutomationStopped だけ)
	Status    change.ID // 止まった実行が変えようとしたステータス(「制約を無視して…」に使う)
}

// Inbox は、受信箱の口(B4-12)。実装は C4(infra/statedb)。既読と一覧は C4 が足す。
type Inbox interface {
	Add(ctx context.Context, n Notice) error
}
```

`apptest/inbox.go`:

```go
package apptest

import (
	"context"
	"slices"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Inbox は port.Inbox の偽物。足した通知を覚える。Err を入れると、Add はそれを返して覚えない。
type Inbox struct {
	mu      sync.Mutex
	notices []port.Notice
	Err     error
}

func (i *Inbox) Add(_ context.Context, n port.Notice) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.Err != nil {
		return i.Err
	}
	i.notices = append(i.notices, n)
	return nil
}

// Notices は、足した通知を足した順に返す。
func (i *Inbox) Notices() []port.Notice {
	i.mu.Lock()
	defer i.mu.Unlock()
	return slices.Clone(i.notices)
}
```

- [ ] **Step 5: 通ることを確かめる**

Run: `go test ./internal/app/... ./internal/archtest/`
Expected: PASS(apptest のファイルは 20 個のまま)

- [ ] **Step 6: Commit**

```bash
git add internal/app/port/inbox.go internal/app/apptest
git commit -m "feat(port): 受信箱の口 Inbox と偽物を足す(B4 Task 2)"
```

---

### Task 3: ルールの型と保存の行(`domain/automation`)

**Files:**
- Create: `internal/domain/automation/rule.go`、`internal/domain/automation/log.go`
- Test: `internal/domain/automation/log_test.go`

**Interfaces:**
- Produces:

```go
type Trigger string // TriggerPROpen "pr_open", TriggerReview "review_requested", TriggerPRMerged "pr_merged",
                    // TriggerIssueClosed "issue_closed", TriggerStatus "status", TriggerStepsDone "steps_done"
var Triggers []Trigger
type Action string  // ActionStatus "status", ActionAssignee "assignee"
const Unassigned = "@none"
type Rule struct {
	ID change.ID; Name string; Trigger Trigger; TriggerStatus change.ID; AllMerged bool
	Action Action; Value string; Enabled bool
}
func Field(id change.ID, name string) string     // rules/<id>/<name>
func FromLog(log []change.Record) []Rule          // 消したもの・名前のないものを除き、内部 ID の順
func Names(log []change.Record) map[change.ID]string // 消したものを含むすべての名前
func Drafts(old, r Rule) []change.Draft           // old がゼロ値なら作成
func DeleteDraft(id change.ID) change.Draft
```

- [ ] **Step 1: 失敗するテストを書く**

```go
package automation_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// stamp は、drafts をログの続きの行にする(テスト用)。
func stamp(log []change.Record, ds ...change.Draft) []change.Record {
	n := len(log)
	meta := change.Meta{Op: "OP", At: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC), Actor: change.User("taro")}
	return append(log, change.Stamp(log, ds, meta, func() change.ID {
		n++
		return change.ID("R" + string(rune('A'+n)))
	})...)
}

func done() automation.Rule {
	return automation.Rule{
		ID: "01RULE2", Name: "完了で外す", Trigger: automation.TriggerStatus, TriggerStatus: "DONE",
		Action: automation.ActionAssignee, Value: automation.Unassigned, Enabled: true,
	}
}

func TestRulesRoundTrip(t *testing.T) {
	merged := automation.Rule{
		ID: "01RULE1", Name: "PR マージで完了", Trigger: automation.TriggerPRMerged, AllMerged: true,
		Action: automation.ActionStatus, Value: "DONE", Enabled: true,
	}
	log := stamp(nil, automation.Drafts(automation.Rule{}, done())...)
	log = stamp(log, automation.Drafts(automation.Rule{}, merged)...)
	got := automation.FromLog(log)
	if len(got) != 2 || got[0] != merged || got[1] != done() {
		t.Fatalf("FromLog = %+v", got) // 内部 ID の順
	}
	edited := done()
	edited.Name, edited.Enabled = "完了で外す2", false
	ds := automation.Drafts(done(), edited)
	if len(ds) != 2 { // 変わった項目だけ
		t.Errorf("Drafts = %+v", ds)
	}
	log = stamp(log, ds...)
	log = stamp(log, automation.DeleteDraft(merged.ID))
	got = automation.FromLog(log)
	if len(got) != 1 || got[0] != edited {
		t.Errorf("after edit and delete = %+v", got)
	}
	names := automation.Names(log)
	if names[merged.ID] != "PR マージで完了" || names[edited.ID] != "完了で外す2" {
		t.Errorf("Names = %v", names)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/automation/`
Expected: FAIL(パッケージがない)

- [ ] **Step 3: 実装する**

`rule.go`:

```go
// Package automation は、自動化のルール(仕様 03「自動化」)の型・保存の行・検査・実行の規則を持つ。
// ルールはワークスペースの log.jsonl の項目 rules/<ID>/<項目> に持つ(B4-2)。
package automation

import "github.com/kukv/hedgehug/internal/domain/change"

// Trigger は、きっかけ(03「きっかけとすること」の 6 つ。B4-4)。
type Trigger string

const (
	TriggerPROpen      Trigger = "pr_open"          // PR がリンクされて Open のとき
	TriggerReview      Trigger = "review_requested" // レビューが依頼されたとき
	TriggerPRMerged    Trigger = "pr_merged"        // リンクした PR がマージされたとき
	TriggerIssueClosed Trigger = "issue_closed"     // リンクした Issue が閉じたとき
	TriggerStatus      Trigger = "status"           // ステータスが次になったとき
	TriggerStepsDone   Trigger = "steps_done"       // Step がすべて完了したとき
)

// Triggers は、きっかけの並び(作成ダイアログの選択肢の順)。
var Triggers = []Trigger{TriggerPROpen, TriggerReview, TriggerPRMerged, TriggerIssueClosed, TriggerStatus, TriggerStepsDone}

// Action は、すること。
type Action string

const (
	ActionStatus   Action = "status"   // ステータスを変更
	ActionAssignee Action = "assignee" // 担当者を変更
)

// Unassigned は、担当者を「未割り当てにする」の値。
const Unassigned = "@none"

// Rule は、自動化のルール 1 つ。比べられるように、値の型だけで持つ。
type Rule struct {
	ID            change.ID
	Name          string
	Trigger       Trigger
	TriggerStatus change.ID // TriggerStatus のときだけ
	AllMerged     bool      // TriggerPRMerged のときだけ。「リンク中の PR がすべてマージ済みのときだけ」
	Action        Action
	Value         string // ステータスの ID、メンバーのログイン名、Unassigned
	Enabled       bool
}

const prefix = "rules/"

// Field は、ルール id の項目名 rules/<id>/<name>。
func Field(id change.ID, name string) string { return prefix + string(id) + "/" + name }
```

`log.go`:

```go
package automation

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// collect は、ログのすべてのルール(消したものを含む)と、消したルールの ID。
func collect(log []change.Record) (map[change.ID]*Rule, map[change.ID]bool) {
	deleted := map[change.ID]bool{}
	for _, r := range log { // Resolve は削除の行を読み飛ばすので、生のログから集める
		if id, ok := strings.CutPrefix(r.F, prefix); ok && r.T == change.Delete && !strings.Contains(id, "/") {
			deleted[change.ID(id)] = true
		}
	}
	rules := map[change.ID]*Rule{}
	for f, raw := range change.Resolve(log, change.Options{}).Values {
		parts := strings.Split(f, "/")
		if len(parts) != 3 || parts[0]+"/" != prefix {
			continue
		}
		id := change.ID(parts[1])
		r, ok := rules[id]
		if !ok {
			r = &Rule{ID: id}
			rules[id] = r
		}
		set(r, parts[2], raw)
	}
	return rules, deleted
}

func set(r *Rule, field string, raw json.RawMessage) {
	switch field {
	case "name":
		_ = json.Unmarshal(raw, &r.Name)
	case "trigger":
		_ = json.Unmarshal(raw, &r.Trigger)
	case "trigger_status":
		_ = json.Unmarshal(raw, &r.TriggerStatus)
	case "all_merged":
		_ = json.Unmarshal(raw, &r.AllMerged)
	case "action":
		_ = json.Unmarshal(raw, &r.Action)
	case "value":
		_ = json.Unmarshal(raw, &r.Value)
	case "enabled":
		_ = json.Unmarshal(raw, &r.Enabled)
	}
}

// FromLog は、ワークスペースのログのルール。消したもの・名前のないものを除き、内部 ID の順(B4-3)。
func FromLog(log []change.Record) []Rule {
	rules, deleted := collect(log)
	var out []Rule
	for id, r := range rules {
		if r.Name != "" && !deleted[id] {
			out = append(out, *r)
		}
	}
	slices.SortFunc(out, func(a, b Rule) int { return strings.Compare(string(a.ID), string(b.ID)) })
	return out
}

// Names は、消したものを含むすべてのルールの名前(B4-16)。
func Names(log []change.Record) map[change.ID]string {
	rules, _ := collect(log)
	out := make(map[change.ID]string, len(rules))
	for id, r := range rules {
		out[id] = r.Name
	}
	return out
}

// Drafts は、old から r への行。old がゼロ値なら作成で、すべての項目を書く。ほかは変わった項目だけ。
func Drafts(old, r Rule) []change.Draft {
	fresh := old.ID == ""
	fields := []struct {
		name     string
		from, to any
	}{
		{"name", old.Name, r.Name},
		{"trigger", old.Trigger, r.Trigger},
		{"trigger_status", old.TriggerStatus, r.TriggerStatus},
		{"all_merged", old.AllMerged, r.AllMerged},
		{"action", old.Action, r.Action},
		{"value", old.Value, r.Value},
		{"enabled", old.Enabled, r.Enabled},
	}
	var ds []change.Draft
	for _, f := range fields {
		switch {
		case fresh:
			ds = append(ds, change.NewSet(Field(r.ID, f.name), nil, f.to))
		case f.from != f.to:
			ds = append(ds, change.NewSet(Field(r.ID, f.name), f.from, f.to))
		}
	}
	return ds
}

// DeleteDraft は、ルール id の削除の行。
func DeleteDraft(id change.ID) change.Draft { return change.NewDelete(prefix + string(id)) }
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/automation/ && go test ./internal/archtest/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/automation
git commit -m "feat(domain): 自動化のルールの型と保存の行を足す(B4 Task 3)"
```

---

### Task 4: ルールの検査と文言

**Files:**
- Create: `internal/domain/automation/check.go`、`internal/domain/automation/text.go`
- Test: `internal/domain/automation/check_test.go`

**Interfaces:**
- Consumes: Task 3 の `Rule`
- Produces:

```go
func Normalize(r Rule) Rule
func Valid(s workspace.Settings, logins []string, r Rule) bool
func Check(rules []Rule, r Rule) error           // 03 の保存の条件の表
func CheckSameStatus(r Rule) error               // 表の最後の行だけ
const SwitchRefusal = "きっかけと同じステータスには変更できません。ルールを編集してください"
func ChangeText(prop, value string) string       // 「{プロパティ名}を {値} に変更」
func ActivityText(rule, change string) string    // 「「{ルール名}」で{変更}」
func AppliedText(rule, change string) string     // 「「{ルール名}」で{変更}しました」
func StoppedText(rule string) string             // 「「{ルール名}」が制約で止まりました」
func IgnoreText(rule string) string              // 「「{ルール名}」の変更を制約を無視して適用」
func DisabledByDeleteText(rule string) string
func RewrittenText(rule, deleted, part, to string) string
```

- [ ] **Step 1: 失敗するテストを書く**

```go
package automation_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func settings() workspace.Settings {
	return workspace.Settings{Statuses: []workspace.Value{
		{ID: "TODO", Name: "Todo", Category: workspace.CategoryTodo},
		{ID: "DOING", Name: "InProgress", Category: workspace.CategoryDoing},
		{ID: "DONE", Name: "Done", Category: workspace.CategoryDone},
		{ID: "CANCEL", Name: "Cancel", Category: workspace.CategoryCanceled},
	}}
}

func TestNormalizeDropsUnusedFields(t *testing.T) {
	r := automation.Normalize(automation.Rule{Name: "  a  ", Trigger: automation.TriggerPROpen, TriggerStatus: "DONE", AllMerged: true})
	if r.Name != "a" || r.TriggerStatus != "" || r.AllMerged {
		t.Errorf("Normalize = %+v", r)
	}
}

func TestValid(t *testing.T) {
	s, logins := settings(), []string{"taro"}
	ok := []automation.Rule{
		{Trigger: automation.TriggerStatus, TriggerStatus: "DONE", Action: automation.ActionAssignee, Value: automation.Unassigned},
		{Trigger: automation.TriggerPRMerged, Action: automation.ActionStatus, Value: "DONE"},
		{Trigger: automation.TriggerStepsDone, Action: automation.ActionAssignee, Value: "Taro"}, // ログイン名は大文字と小文字を区別しない
	}
	for _, r := range ok {
		if !automation.Valid(s, logins, r) {
			t.Errorf("Valid(%+v) = false", r)
		}
	}
	bad := []automation.Rule{
		{Trigger: "nope", Action: automation.ActionStatus, Value: "DONE"},
		{Trigger: automation.TriggerStatus, TriggerStatus: "GONE", Action: automation.ActionStatus, Value: "DONE"},
		{Trigger: automation.TriggerPROpen, Action: automation.ActionStatus, Value: "GONE"},
		{Trigger: automation.TriggerPROpen, Action: automation.ActionAssignee, Value: "jiro"},
		{Trigger: automation.TriggerPROpen, Action: "nope", Value: "DONE"},
	}
	for _, r := range bad {
		if automation.Valid(s, logins, r) {
			t.Errorf("Valid(%+v) = true", r)
		}
	}
}

func TestCheck(t *testing.T) {
	rules := []automation.Rule{{ID: "R1", Name: "完了で外す"}}
	cases := []struct {
		r    automation.Rule
		want string
	}{
		{automation.Rule{ID: "R2", Name: ""}, "名前を入力してください"},
		{automation.Rule{ID: "R2", Name: "完了で外す"}, "同じ名前のルールがあります"},
		{automation.Rule{ID: "R2", Name: "x", Trigger: automation.TriggerStatus, TriggerStatus: "DONE", Action: automation.ActionStatus, Value: "DONE"},
			"きっかけと同じステータスには変更できません"},
		{automation.Rule{ID: "R1", Name: "完了で外す"}, ""}, // 自分の名前は重複にしない
	}
	for _, c := range cases {
		if got := message(automation.Check(rules, c.r)); got != c.want {
			t.Errorf("Check(%+v) = %q, want %q", c.r, got, c.want)
		}
	}
}

func TestTexts(t *testing.T) {
	c := automation.ChangeText("ステータス", "Done")
	got := []string{
		c,
		automation.ActivityText("PR マージで完了", c),
		automation.AppliedText("PR マージで完了", c),
		automation.StoppedText("PR マージで完了"),
		automation.IgnoreText("PR マージで完了"),
		automation.DisabledByDeleteText("X"),
		automation.RewrittenText("X", "Review", "すること", "Todo"),
	}
	want := []string{
		"ステータスを Done に変更",
		"「PR マージで完了」でステータスを Done に変更",
		"「PR マージで完了」でステータスを Done に変更しました",
		"「PR マージで完了」が制約で止まりました",
		"「PR マージで完了」の変更を制約を無視して適用",
		"自動化「X」は、ステータスの削除で、きっかけと同じステータスへの変更になったため、無効にしました",
		"自動化「X」は、ステータス「Review」の削除で、することのステータスを「Todo」に書き換えました",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: %q, want %q", i, got[i], want[i])
		}
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/automation/`
Expected: FAIL(`Normalize` などがない)

- [ ] **Step 3: 実装する**

`check.go`:

```go
package automation

import (
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// SwitchRefusal は、スイッチを ON にするときの断り(03「ルールの一覧」の下)。
const SwitchRefusal = "きっかけと同じステータスには変更できません。ルールを編集してください"

const sameStatus = "きっかけと同じステータスには変更できません"

// Normalize は、保存する形にする。名前の前後の空白を除き、きっかけで使わない項目を空にする
// (03「きっかけを他のものに変えると、条件の選択は保存しない」)。
func Normalize(r Rule) Rule {
	r.Name = workspace.NormalizeName(r.Name)
	if r.Trigger != TriggerStatus {
		r.TriggerStatus = ""
	}
	if r.Trigger != TriggerPRMerged {
		r.AllMerged = false
	}
	return r
}

// Valid は、きっかけ・すること・値の参照が正しいか。false は ErrNotFound にする(B4-6)。logins は Store のメンバー。
func Valid(s workspace.Settings, logins []string, r Rule) bool {
	if !slices.Contains(Triggers, r.Trigger) || (r.Trigger == TriggerStatus && !hasStatus(s, r.TriggerStatus)) {
		return false
	}
	switch r.Action {
	case ActionStatus:
		return hasStatus(s, change.ID(r.Value))
	case ActionAssignee:
		return r.Value == Unassigned || slices.ContainsFunc(logins, func(l string) bool { return strings.EqualFold(l, r.Value) })
	}
	return false
}

func hasStatus(s workspace.Settings, id change.ID) bool {
	return slices.ContainsFunc(s.Statuses, func(v workspace.Value) bool { return v.ID == id })
}

// Check は、保存の条件(03 の表の順)。r は Normalize のあと、rules はいまのルール。
func Check(rules []Rule, r Rule) error {
	if r.Name == "" {
		return refusal.New("名前を入力してください")
	}
	for _, o := range rules {
		if o.ID != r.ID && o.Name == r.Name {
			return refusal.New("同じ名前のルールがあります")
		}
	}
	return CheckSameStatus(r)
}

// CheckSameStatus は、きっかけのステータスへ変えるルールを断る(表の最後の行)。
func CheckSameStatus(r Rule) error {
	if r.Trigger == TriggerStatus && r.Action == ActionStatus && string(r.TriggerStatus) == r.Value {
		return refusal.New(sameStatus)
	}
	return nil
}
```

`text.go`:

```go
package automation

import "fmt"

// 自動化の文言(03「実行の規則」「制約で止まったとき」「削除」、06「通知の種類」。B4-10)。

// ChangeText は「{プロパティ名}を {値} に変更」。
func ChangeText(prop, value string) string { return fmt.Sprintf("%sを %s に変更", prop, value) }

// ActivityText は、アクティビティと最近の実行の文。
func ActivityText(rule, change string) string { return fmt.Sprintf("「%s」で%s", rule, change) }

// AppliedText は、自動化が変えたときの通知の本文。
func AppliedText(rule, change string) string { return fmt.Sprintf("「%s」で%sしました", rule, change) }

// StoppedText は、制約で止まったときの通知の本文。
func StoppedText(rule string) string { return fmt.Sprintf("「%s」が制約で止まりました", rule) }

// IgnoreText は、「制約を無視して…」の保存のアクティビティの文。
func IgnoreText(rule string) string { return fmt.Sprintf("「%s」の変更を制約を無視して適用", rule) }

// DisabledByDeleteText は、ステータスの削除で無効にしたルールの通知の本文。
func DisabledByDeleteText(rule string) string {
	return fmt.Sprintf("自動化「%s」は、ステータスの削除で、きっかけと同じステータスへの変更になったため、無効にしました", rule)
}

// RewrittenText は、ステータスの削除で書き換えたルールの通知の本文。part は「きっかけ」か「すること」。
func RewrittenText(rule, deleted, part, to string) string {
	return fmt.Sprintf("自動化「%s」は、ステータス「%s」の削除で、%sのステータスを「%s」に書き換えました", rule, deleted, part, to)
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/automation/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/automation
git commit -m "feat(domain): 自動化のルールの検査と文言を足す(B4 Task 4)"
```

---

### Task 5: 実行の規則(`automation.Run`・`StepsDone`)

**Files:**
- Create: `internal/domain/automation/run.go`、`internal/domain/automation/steps.go`
- Test: `internal/domain/automation/run_test.go`

**Interfaces:**
- Consumes: Task 3・4、`workspace.CheckTransition`、`item.ParseInput`・`item.Current`・`item.Field`
- Produces:

```go
type Event struct { Trigger Trigger; Status change.ID; AllMerged bool }
type Outcome struct {
	Rule    Rule
	Prop    workspace.Property
	To      json.RawMessage
	Draft   *change.Draft // Rule 付きの行。止まったときは nil
	Stopped string        // 止まった理由。変えたときは空
}
func Run(s workspace.Settings, rules []Rule, it item.Item, ev Event) []Outcome
func StepsDone(s workspace.Settings, steps []item.Item, before map[item.ID]change.ID) bool
```

- [ ] **Step 1: 失敗するテストを書く**

```go
package automation_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// runSettings は、ステータス 4 つと、Task・Step だけが使う担当者を持つ設定。
func runSettings() workspace.Settings {
	s := settings()
	s.Properties = []workspace.Property{
		{ID: "status", Key: "status", Name: "ステータス", Type: workspace.TypeStatus, Builtin: true, Kinds: workspace.Kinds},
		{ID: "assignee", Key: "assignee", Name: "担当者", Type: workspace.TypeUser, Builtin: true, Kinds: []string{"Task", "Step"}},
	}
	return s
}

func rule(id string, trig automation.Trigger, trigStatus change.ID, act automation.Action, v string) automation.Rule {
	return automation.Rule{ID: change.ID(id), Name: id, Trigger: trig, TriggerStatus: trigStatus, Action: act, Value: v, Enabled: true}
}

func TestRunAppliesMatchingRulesInOrder(t *testing.T) {
	s := runSettings()
	rules := []automation.Rule{
		rule("R1", automation.TriggerStatus, "DONE", automation.ActionAssignee, automation.Unassigned),
		rule("R2", automation.TriggerStatus, "DOING", automation.ActionAssignee, "taro"), // きっかけが違う
		rule("R3", automation.TriggerStatus, "DONE", automation.ActionAssignee, "taro"),
	}
	rules[0].Enabled = true
	off := rule("R4", automation.TriggerStatus, "DONE", automation.ActionStatus, "CANCEL")
	off.Enabled = false
	rules = append(rules, off)
	it := item.Item{ID: "I1", Kind: item.Task, Status: "DONE", Assignee: "hanako"}
	got := automation.Run(s, rules, it, automation.Event{Trigger: automation.TriggerStatus, Status: "DONE"})
	if len(got) != 2 || got[0].Rule.ID != "R1" || got[1].Rule.ID != "R3" {
		t.Fatalf("Run = %+v", got)
	}
	// R3 は R1 が外したあとの値(なし)から taro にする
	if got[1].Draft == nil || got[1].Draft.Rule != "R3" || string(got[1].Draft.From) != `null` || string(got[1].To) != `"taro"` {
		t.Errorf("R3 = %+v / %+v", got[1], got[1].Draft)
	}
}

func TestRunSkipsRulesThatChangeNothing(t *testing.T) {
	s := runSettings()
	rules := []automation.Rule{
		rule("R1", automation.TriggerStepsDone, "", automation.ActionAssignee, "taro"),
		rule("R2", automation.TriggerStepsDone, "", automation.ActionStatus, "DONE"),
	}
	epic := item.Item{ID: "E1", Kind: item.Epic, Status: "DONE"} // Epic は担当者を使わない・すでに Done
	if got := automation.Run(s, rules, epic, automation.Event{Trigger: automation.TriggerStepsDone}); len(got) != 0 {
		t.Errorf("Run = %+v", got)
	}
	task := item.Item{ID: "T1", Kind: item.Task, Status: "DOING", Assignee: "taro"}
	got := automation.Run(s, rules, task, automation.Event{Trigger: automation.TriggerStepsDone})
	if len(got) != 1 || got[0].Rule.ID != "R2" {
		t.Errorf("Run = %+v", got)
	}
}

func TestRunStopsOnTheWorkflowAndGoesOn(t *testing.T) {
	s := runSettings()
	s.Workflow = map[change.ID][]change.ID{"TODO": {"DOING"}}
	rules := []automation.Rule{
		rule("R1", automation.TriggerPROpen, "", automation.ActionStatus, "DONE"),
		rule("R2", automation.TriggerPROpen, "", automation.ActionStatus, "DOING"),
		rule("R3", automation.TriggerPROpen, "", automation.ActionStatus, "DONE"), // DOING からは通る
	}
	it := item.Item{ID: "T1", Kind: item.Task, Status: "TODO"}
	got := automation.Run(s, rules, it, automation.Event{Trigger: automation.TriggerPROpen})
	if len(got) != 3 {
		t.Fatalf("Run = %+v", got)
	}
	if got[0].Draft != nil || got[0].Stopped != "ワークフローの制約: Todo から Done には変更できません(InProgress のみ)" || string(got[0].To) != `"DONE"` {
		t.Errorf("R1 = %+v", got[0])
	}
	if got[1].Draft == nil || got[2].Draft == nil || string(got[2].Draft.From) != `"DOING"` {
		t.Errorf("R2, R3 = %+v, %+v", got[1], got[2])
	}
}

func TestRunMergedCondition(t *testing.T) {
	s := runSettings()
	r := rule("R1", automation.TriggerPRMerged, "", automation.ActionStatus, "DONE")
	r.AllMerged = true
	it := item.Item{ID: "T1", Kind: item.Task, Status: "DOING"}
	if got := automation.Run(s, []automation.Rule{r}, it, automation.Event{Trigger: automation.TriggerPRMerged}); len(got) != 0 {
		t.Errorf("not all merged: %+v", got)
	}
	if got := automation.Run(s, []automation.Rule{r}, it, automation.Event{Trigger: automation.TriggerPRMerged, AllMerged: true}); len(got) != 1 {
		t.Errorf("all merged: %+v", got)
	}
}

func TestStepsDone(t *testing.T) {
	s := settings()
	steps := []item.Item{{ID: "S1", Status: "DONE"}, {ID: "S2", Status: "CANCEL"}, {ID: "S3", Status: "DONE"}}
	if !automation.StepsDone(s, steps, map[item.ID]change.ID{"S3": "TODO"}) {
		t.Error("the last step completes the parent")
	}
	if automation.StepsDone(s, steps, map[item.ID]change.ID{"S3": "CANCEL"}) {
		t.Error("already all closed before the change")
	}
	steps[0].Status = "DOING"
	if automation.StepsDone(s, steps, map[item.ID]change.ID{"S3": "TODO"}) {
		t.Error("a step is still open")
	}
	if automation.StepsDone(s, nil, nil) {
		t.Error("no steps")
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/automation/`
Expected: FAIL(`Run` がない)

- [ ] **Step 3: 実装する**

`run.go`:

```go
package automation

import (
	"bytes"
	"encoding/json"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Event は、アイテムに起きたきっかけ。
type Event struct {
	Trigger   Trigger
	Status    change.ID // TriggerStatus: 変わったあとのステータス
	AllMerged bool      // TriggerPRMerged: リンク中の PR(マージされずに閉じたものを除く)が 1 件以上あり、すべてマージ済みか
}

// Outcome は、1 つのルールの実行の結果。Draft があれば変えた、Stopped があれば止まった。
type Outcome struct {
	Rule    Rule
	Prop    workspace.Property // ステータスか担当者
	To      json.RawMessage    // 変えたあと(止まったときは変えようとした)の値
	Draft   *change.Draft      // ルールが書く行(Rule 付き)
	Stopped string             // 止まった理由(03「制約で止まったとき」の文言)
}

// Run は、it に起きた ev で、有効なルールを一覧の順に実行する(03「実行の規則」)。it は利用者の操作のあとの値。
// 1 つのルールがステータスを変えたら、次のルールはそのあとの値から検査する。何もしないルール(同じ値、
// 担当者を使わない種類、なくなった値)は結果に入れない。自動化の変更は、ほかのきっかけを起こさない(呼ぶ側で起こさない)。
func Run(s workspace.Settings, rules []Rule, it item.Item, ev Event) []Outcome {
	var out []Outcome
	for _, r := range rules {
		if !r.Enabled || !matches(r, ev) {
			continue
		}
		if o, ok := apply(s, r, &it); ok {
			out = append(out, o)
		}
	}
	return out
}

func matches(r Rule, ev Event) bool {
	switch {
	case r.Trigger != ev.Trigger:
		return false
	case r.Trigger == TriggerStatus:
		return r.TriggerStatus == ev.Status
	case r.Trigger == TriggerPRMerged:
		return !r.AllMerged || ev.AllMerged
	}
	return true
}

// apply は、r のすることを it に当てる。変えたら it を変えたあとの値にする。
func apply(s workspace.Settings, r Rule, it *item.Item) (Outcome, bool) {
	p, ok := s.Property(change.ID(r.Action))
	if !ok || (r.Action == ActionAssignee && !slices.Contains(p.Kinds, string(it.Kind))) {
		return Outcome{}, false
	}
	value := r.Value
	if value == Unassigned {
		value = ""
	}
	to, err := item.ParseInput(p, change.Value(value), s)
	from := item.Current(*it, p)
	if err != nil || bytes.Equal(from, to) {
		return Outcome{}, false
	}
	o := Outcome{Rule: r, Prop: p, To: to}
	if r.Action == ActionStatus {
		if err := workspace.CheckTransition(s, it.Status, change.ID(value)); err != nil {
			o.Stopped = err.Error()
			return o, true
		}
		it.Status = change.ID(value)
	} else {
		it.Assignee = value
	}
	d := change.NewSet(item.Field(p), from, to).ByRule(r.ID)
	o.Draft = &d
	return o, true
}
```

注: 組み込みのステータスと担当者のプロパティの ID は、キーと同じ `status`・`assignee`(B1 の決定 3)なので、`change.ID(r.Action)` で引ける。`item.Current` が値のないときに返す形と `item.ParseInput` の返す形がそろっていることを、テストの `TestRunAppliesMatchingRulesInOrder`(`From` が `null`)で確かめる。そろっていなければ、`items.SetProperty` と同じ比べ方(`bytes.Equal(item.Current(...), to)`)に合わせて直す。

`steps.go`:

```go
package automation

import (
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// StepsDone は、1 回の操作で、親の Step がすべて完了・中止になったか(03「Step がすべて完了したとき」)。
// steps は親のすべての Step(操作のあとの値。自動化の変更は含めない。B4-8)、before は操作で変えた Step の
// 変更前のステータス(B4-9)。
func StepsDone(s workspace.Settings, steps []item.Item, before map[item.ID]change.ID) bool {
	return closed(s, steps, nil) && !closed(s, steps, before)
}

// closed は、Step が 1 つ以上あり、すべて完了・中止の分類か。override は値を差し替える Step。
func closed(s workspace.Settings, steps []item.Item, override map[item.ID]change.ID) bool {
	if len(steps) == 0 {
		return false
	}
	for _, st := range steps {
		v := st.Status
		if o, ok := override[st.ID]; ok {
			v = o
		}
		c := category(s, v)
		if c != workspace.CategoryDone && c != workspace.CategoryCanceled {
			return false
		}
	}
	return true
}

func category(s workspace.Settings, id change.ID) workspace.Category {
	for _, v := range s.Statuses {
		if v.ID == id {
			return v.Category
		}
	}
	return ""
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/automation/ && make lint`
Expected: PASS(`make lint` がなければ `make check` の lint の部分。認知的複雑度 15 以内)

- [ ] **Step 5: Commit**

```bash
git add internal/domain/automation
git commit -m "feat(domain): 自動化の実行の規則と Step の完了の判定を足す(B4 Task 5)"
```

---

### Task 6: ステータスの削除でのルールの追従(domain)

**Files:**
- Create: `internal/domain/automation/follow.go`
- Test: `internal/domain/automation/follow_test.go`

**Interfaces:**
- Produces:

```go
type RuleChange struct { Rule Rule; Disabled bool; Part string } // Part は「きっかけ」か「すること」
func StatusDeleteDrafts(rules []Rule, id, repl change.ID) ([]change.Draft, []RuleChange)
```

- [ ] **Step 1: 失敗するテストを書く**

```go
package automation_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/automation"
)

func TestStatusDeleteRewritesRules(t *testing.T) {
	rules := []automation.Rule{
		rule("R1", automation.TriggerStatus, "REVIEW", automation.ActionAssignee, "taro"),      // きっかけを書き換える
		rule("R2", automation.TriggerPROpen, "", automation.ActionStatus, "REVIEW"),            // することを書き換える
		rule("R3", automation.TriggerStatus, "TODO", automation.ActionStatus, "REVIEW"),        // 同じになる: 無効
		rule("R4", automation.TriggerStatus, "DONE", automation.ActionStatus, "TODO"),          // 関係ない
	}
	ds, changes := automation.StatusDeleteDrafts(rules, "REVIEW", "TODO")
	if len(changes) != 3 {
		t.Fatalf("changes = %+v", changes)
	}
	if changes[0].Part != "きっかけ" || changes[0].Rule.TriggerStatus != "TODO" || changes[0].Disabled {
		t.Errorf("R1 = %+v", changes[0])
	}
	if changes[1].Part != "すること" || changes[1].Rule.Value != "TODO" || changes[1].Disabled {
		t.Errorf("R2 = %+v", changes[1])
	}
	if !changes[2].Disabled || changes[2].Rule.Enabled || changes[2].Rule.Value != "TODO" {
		t.Errorf("R3 = %+v", changes[2])
	}
	if len(ds) != 4 { // R1: trigger_status、R2: value、R3: value と enabled
		t.Errorf("drafts = %+v", ds)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/automation/ -run TestStatusDelete`
Expected: FAIL

- [ ] **Step 3: 実装する**

```go
package automation

import "github.com/kukv/hedgehug/internal/domain/change"

// RuleChange は、ステータスの削除で変わったルール(通知の材料)。
type RuleChange struct {
	Rule     Rule   // 変えたあと
	Disabled bool   // きっかけと同じステータスへの変更になって無効にした
	Part     string // 書き換えた側(「きっかけ」か「すること」)。両方なら「すること」
}

// StatusDeleteDrafts は、ステータス id を repl に置き換えるルールの行と、変わったルール
// (03「値の編集」の「削除」の自動化)。書き換えてきっかけとすることが同じステータスになれば無効にする。
func StatusDeleteDrafts(rules []Rule, id, repl change.ID) ([]change.Draft, []RuleChange) {
	var ds []change.Draft
	var changes []RuleChange
	for _, r := range rules {
		n, part := r, ""
		if n.Trigger == TriggerStatus && n.TriggerStatus == id {
			n.TriggerStatus, part = repl, "きっかけ"
		}
		if n.Action == ActionStatus && n.Value == string(id) {
			n.Value, part = string(repl), "すること"
		}
		if part == "" {
			continue
		}
		disabled := CheckSameStatus(n) != nil
		if disabled {
			n.Enabled = false
		}
		ds = append(ds, Drafts(r, n)...)
		changes = append(changes, RuleChange{Rule: n, Disabled: disabled, Part: part})
	}
	return ds, changes
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/automation/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/automation
git commit -m "feat(domain): ステータスの削除で自動化のルールを書き換える規則を足す(B4 Task 6)"
```

---

### Task 7: ルールの操作(`app/automations`)

**Files:**
- Create: `internal/app/automations/service.go`、`internal/app/automations/rules.go`
- Test: `internal/app/automations/rules_test.go`

**Interfaces:**
- Consumes: Task 3・4
- Produces:

```go
type Service struct { Stores port.Stores; IDs port.IDs; Identity port.Identity; Clock port.Clock }
func (s Service) Create(ctx, store port.StoreID, ws workspace.ID, in automation.Rule) (automation.Rule, error)
func (s Service) Update(ctx, store, ws, id change.ID, in automation.Rule) error
func (s Service) Delete(ctx, store, ws, id change.ID) error
func (s Service) SetEnabled(ctx, store, ws, id change.ID, on bool) error
func rulesOf(r port.Reader, ws workspace.ID) ([]automation.Rule, []change.Record, error) // パッケージ内
```

- [ ] **Step 1: 失敗するテストを書く**

```go
package automations_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/automations"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const store port.StoreID = "S1"

// recorder は、保存のアクティビティの文を覚える。
type recorder struct {
	port.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func setup(t *testing.T) (automations.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return automations.Service{Stores: rec, IDs: ids, Identity: apptest.Identity{Login: "hanako"}, Clock: clock}, rec
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func doneRule() automation.Rule {
	return automation.Rule{
		Name: " 完了で外す ", Trigger: automation.TriggerStatus, TriggerStatus: apptest.SampleStatusDone,
		AllMerged: true, Action: automation.ActionAssignee, Value: automation.Unassigned,
	}
}

func TestCreateUpdateDelete(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "完了で外す" || !r.Enabled || r.AllMerged || r.ID == "" {
		t.Errorf("Create = %+v", r)
	}
	if _, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule()); message(err) != "同じ名前のルールがあります" {
		t.Errorf("duplicate: %v", err)
	}
	if err := svc.SetEnabled(ctx, store, apptest.SampleWorkspace, r.ID, false); err != nil {
		t.Fatal(err)
	}
	edit := doneRule()
	edit.Name = "完了で taro に"
	edit.Value = "taro"
	if err := svc.Update(ctx, store, apptest.SampleWorkspace, r.ID, edit); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, store, apptest.SampleWorkspace, r.ID); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"自動化「完了で外す」を作成",
		"自動化「完了で外す」を作成", // 断った保存も Write は呼ばれる(行は残らない)
		"自動化「完了で外す」を無効にする",
		"自動化「完了で taro に」を編集",
		"自動化「完了で taro に」を削除",
	}
	if len(rec.summaries) != len(want) {
		t.Fatalf("summaries = %q", rec.summaries)
	}
	for i := range want {
		if rec.summaries[i] != want[i] {
			t.Errorf("%d: %q, want %q", i, rec.summaries[i], want[i])
		}
	}
}

// 編集では有効・無効を変えない(B4-5)。
func TestUpdateKeepsEnabled(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	r, _ := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	_ = svc.SetEnabled(ctx, store, apptest.SampleWorkspace, r.ID, false)
	edit := doneRule()
	edit.Name = "別名"
	_ = svc.Update(ctx, store, apptest.SampleWorkspace, r.ID, edit)
	v, err := svc.List(ctx, store, apptest.SampleWorkspace)
	if err != nil || len(v.Rules) != 1 || v.Rules[0].Enabled || v.Rules[0].Name != "別名" {
		t.Errorf("List = %+v, %v", v, err)
	}
}

func TestSetEnabledRefusesTheSameStatus(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, _ := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	_ = svc.SetEnabled(ctx, store, apptest.SampleWorkspace, r.ID, false)
	// 同期で届いた形を作る: きっかけとすることが同じステータスで無効のルール
	same := automation.Rule{ID: "01RULESAME000000000000001", Name: "同じ", Trigger: automation.TriggerStatus,
		TriggerStatus: apptest.SampleStatusDone, Action: automation.ActionStatus, Value: string(apptest.SampleStatusDone)}
	err := rec.Stores.Write(ctx, store, port.Operation{Actor: change.User("taro"), Summary: "同期"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, automation.Drafts(automation.Rule{}, same)...)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetEnabled(ctx, store, apptest.SampleWorkspace, same.ID, true); message(err) != automation.SwitchRefusal {
		t.Errorf("SetEnabled = %v", err)
	}
	if err := svc.SetEnabled(ctx, store, apptest.SampleWorkspace, same.ID, false); err != nil { // OFF は通す
		t.Errorf("SetEnabled(false) = %v", err)
	}
}

func TestBadReferencesAreNotFound(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	bad := doneRule()
	bad.Value = "jiro" // メンバーにいない
	if _, err := svc.Create(ctx, store, apptest.SampleWorkspace, bad); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Create = %v", err)
	}
	if err := svc.Delete(ctx, store, apptest.SampleWorkspace, "01NORULE"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Delete = %v", err)
	}
	if err := svc.SetEnabled(ctx, store, apptest.SampleWorkspace, "01NORULE", true); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SetEnabled = %v", err)
	}
}
```

注: `TestUpdateKeepsEnabled` は Task 8 の `List` を使う。Task 7 では `List` を、`Rules` だけを返す形で先に置き(`Recent` は空)、Task 8 で最近の実行を足す。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/automations/`
Expected: FAIL(パッケージがない)

- [ ] **Step 3: 実装する**

`service.go`:

```go
// Package automations は、自動化のルールの操作(仕様 03「自動化」)を持つ。
package automations

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Service は、自動化の操作が使う口。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
	Clock    port.Clock
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// rulesOf は、ワークスペースのルールとログ。
func rulesOf(r port.Reader, ws workspace.ID) ([]automation.Rule, []change.Record, error) {
	if _, err := r.Workspaces().Get(ws); err != nil {
		return nil, nil, err
	}
	log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return nil, nil, err
	}
	return automation.FromLog(log), log, nil
}

func logins(r port.Reader) ([]string, error) {
	ms, err := r.Members().List()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Login)
	}
	return out, nil
}

// edit は、summary で決めた文で 1 回保存する。drafts は保存の中のいまのルールから行を作る。
func (s Service) edit(ctx context.Context, store port.StoreID, ws workspace.ID,
	summary func(rules []automation.Rule) (string, error),
	drafts func(st workspace.Settings, rules []automation.Rule, members []string) ([]change.Draft, error),
) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	var text string
	err = s.Stores.Read(ctx, store, func(r port.Reader) error {
		rules, _, err := rulesOf(r, ws)
		if err != nil {
			return err
		}
		text, err = summary(rules)
		return err
	})
	if err != nil {
		return err
	}
	return s.Stores.Write(ctx, store, port.Operation{Actor: change.User(me), Summary: text}, func(tx port.Tx) error {
		rules, _, err := rulesOf(tx, ws)
		if err != nil {
			return err
		}
		st, err := tx.Workspaces().Settings(ws)
		if err != nil {
			return err
		}
		ms, err := logins(tx)
		if err != nil {
			return err
		}
		ds, err := drafts(st, rules, ms)
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(ws, ds...)
	})
}

func find(rules []automation.Rule, id change.ID) (automation.Rule, error) {
	for _, r := range rules {
		if r.ID == id {
			return r, nil
		}
	}
	return automation.Rule{}, port.ErrNotFound
}
```

`rules.go`:

```go
package automations

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Create は、ワークスペース全体のルールを作る(03「ルールの一覧」の「ルールを作成」)。有効で始まる。
func (s Service) Create(ctx context.Context, store port.StoreID, ws workspace.ID, in automation.Rule) (automation.Rule, error) {
	in.ID, in.Enabled = s.IDs.New(), true
	in = automation.Normalize(in)
	err := s.edit(ctx, store, ws,
		func([]automation.Rule) (string, error) { return fmt.Sprintf("自動化「%s」を作成", oneLine(in.Name)), nil },
		func(st workspace.Settings, rules []automation.Rule, ms []string) ([]change.Draft, error) {
			if !automation.Valid(st, ms, in) {
				return nil, port.ErrNotFound
			}
			if err := automation.Check(rules, in); err != nil {
				return nil, err
			}
			return automation.Drafts(automation.Rule{}, in), nil
		})
	if err != nil {
		return automation.Rule{}, err
	}
	return in, nil
}

// Update は、ルールを直す。有効・無効は変えない(B4-5)。
func (s Service) Update(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, in automation.Rule) error {
	in.ID = id
	in = automation.Normalize(in)
	return s.edit(ctx, store, ws,
		func(rules []automation.Rule) (string, error) {
			if _, err := find(rules, id); err != nil {
				return "", err
			}
			return fmt.Sprintf("自動化「%s」を編集", oneLine(in.Name)), nil
		},
		func(st workspace.Settings, rules []automation.Rule, ms []string) ([]change.Draft, error) {
			old, err := find(rules, id)
			if err != nil {
				return nil, err
			}
			in.Enabled = old.Enabled
			if !automation.Valid(st, ms, in) {
				return nil, port.ErrNotFound
			}
			if err := automation.Check(rules, in); err != nil {
				return nil, err
			}
			return automation.Drafts(old, in), nil
		})
}

// Delete は、ルールを消す。
func (s Service) Delete(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID) error {
	return s.edit(ctx, store, ws,
		func(rules []automation.Rule) (string, error) {
			r, err := find(rules, id)
			return fmt.Sprintf("自動化「%s」を削除", oneLine(r.Name)), err
		},
		func(_ workspace.Settings, rules []automation.Rule, _ []string) ([]change.Draft, error) {
			if _, err := find(rules, id); err != nil {
				return nil, err
			}
			return []change.Draft{automation.DeleteDraft(id)}, nil
		})
}

// SetEnabled は、スイッチ(03)。ON にするときは、きっかけと同じステータスへのルールを断る。同じ値なら何も保存しない。
func (s Service) SetEnabled(ctx context.Context, store port.StoreID, ws workspace.ID, id change.ID, on bool) error {
	word := "無効"
	if on {
		word = "有効"
	}
	return s.edit(ctx, store, ws,
		func(rules []automation.Rule) (string, error) {
			r, err := find(rules, id)
			return fmt.Sprintf("自動化「%s」を%sにする", oneLine(r.Name), word), err
		},
		func(_ workspace.Settings, rules []automation.Rule, _ []string) ([]change.Draft, error) {
			old, err := find(rules, id)
			if err != nil {
				return nil, err
			}
			if on && automation.CheckSameStatus(old) != nil {
				return nil, refusal.New(automation.SwitchRefusal)
			}
			n := old
			n.Enabled = on
			return automation.Drafts(old, n), nil
		})
}
```

`List` の仮の形(`view.go`。Task 8 で最近の実行を足す):

```go
package automations

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// View は、自動化のサブタブの中身。
type View struct {
	Rules  []automation.Rule
	Recent []Run
}

// Run は、最近の実行の 1 行(Task 8)。
type Run struct{}

// List は、ワークスペースのルールと最近の実行。
func (s Service) List(ctx context.Context, store port.StoreID, ws workspace.ID) (View, error) {
	var v View
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		rules, _, err := rulesOf(r, ws)
		v.Rules = rules
		return err
	})
	return v, err
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/automations/ ./internal/archtest/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/automations
git commit -m "feat(app): 自動化のルールの作成・編集・削除・有効と無効を足す(B4 Task 7)"
```

---

### Task 8: 最近の実行と「制約を無視して適用」

**Files:**
- Modify: `internal/app/automations/view.go`
- Create: `internal/app/automations/apply.go`
- Test: `internal/app/automations/view_test.go`、`internal/app/automations/apply_test.go`

**Interfaces:**
- Consumes: Task 1(`ByRule`)、Task 3・4・7
- Produces:

```go
type Run struct { At time.Time; Item item.ID; Display string; Text string }
func (s Service) List(ctx, store, ws) (View, error) // Recent は新しい順に最大 8 件
func (s Service) ApplyIgnoringWorkflow(ctx, store port.StoreID, id item.ID, rule, status change.ID) error
```

- [ ] **Step 1: 失敗するテストを書く(最近の実行)**

`view_test.go`:

```go
package automations_test

import (
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// writeRuleRow は、ルール rule が担当者を to にした保存を 1 回作る(実行の操作は Task 9 なので、行を直接書く)。
func writeRuleRow(t *testing.T, s port.Stores, rule change.ID, it item.ID, from, to any) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "HH-2 のステータスを Done に変更"}
	err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(it, change.NewSet("assignee", from, to).ByRule(rule))
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestListShowsRecentRuns(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, _ := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	writeRuleRow(t, rec.Stores, r.ID, apptest.SampleTask, nil, "taro")
	writeRuleRow(t, rec.Stores, r.ID, apptest.SampleTask, nil, "taro") // 同じ変更: 最も早い 1 行だけ(B4-15)
	_ = svc.Delete(ctx, store, apptest.SampleWorkspace, r.ID)            // 消したルールの名前も引く(B4-16)
	v, err := svc.List(ctx, store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Recent) != 1 {
		t.Fatalf("Recent = %+v", v.Recent)
	}
	got := v.Recent[0]
	if got.Item != apptest.SampleTask || got.Display != "HH-2" || got.Text != "「完了で外す」で担当者を taro に変更" {
		t.Errorf("Recent[0] = %+v", got)
	}
}

func TestListKeepsTheNewestEight(t *testing.T) {
	svc, rec := setup(t)
	r, _ := svc.Create(t.Context(), store, apptest.SampleWorkspace, doneRule())
	for i := range 10 {
		writeRuleRow(t, rec.Stores, r.ID, apptest.SampleTask, nil, fmt.Sprintf("user%d", i))
	}
	v, _ := svc.List(t.Context(), store, apptest.SampleWorkspace)
	if len(v.Recent) != 8 || v.Recent[0].Text != "「完了で外す」で担当者を 不明なユーザー に変更" {
		t.Errorf("Recent = %+v", v.Recent)
	}
}
```

注: 偽の時計が保存のたびに進まないなら、`At` が同じになる。そのときの並びは、ログの新しい行が先(書いた順の逆)にする。`TestListKeepsTheNewestEight` の 1 行目は最後に書いた `user9` の行。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/automations/ -run TestList`
Expected: FAIL(`Run` に項目がない)

- [ ] **Step 3: 実装する**

`view.go` を次にする:

```go
package automations

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// recentMax は、最近の実行の件数(03「最近の実行」)。
const recentMax = 8

// View は、自動化のサブタブの中身。
type View struct {
	Rules  []automation.Rule
	Recent []Run
}

// Run は、最近の実行の 1 行(B4-15)。
type Run struct {
	At      time.Time
	Item    item.ID
	Display string // HH-12
	Text    string // 「{ルール名}」で{プロパティ名}を {値} に変更
}

// List は、ワークスペースのルールと最近の実行。
func (s Service) List(ctx context.Context, store port.StoreID, ws workspace.ID) (View, error) {
	var v View
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		rules, log, err := rulesOf(r, ws)
		if err != nil {
			return err
		}
		v.Rules = rules
		v.Recent, err = s.recent(r, ws, automation.Names(log))
		return err
	})
	return v, err
}

type ruleRow struct {
	rec  change.Record
	item item.Item
}

// recent は、ワークスペースのすべてのアイテムの、自動化の行(B4-15)。
func (s Service) recent(r port.Reader, ws workspace.ID, names map[change.ID]string) ([]Run, error) {
	rows, err := ruleRows(r, ws)
	if err != nil {
		return nil, err
	}
	w, err := r.Workspaces().Get(ws)
	if err != nil {
		return nil, err
	}
	st, err := r.Workspaces().Settings(ws)
	if err != nil {
		return nil, err
	}
	who, err := memberNames(r)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	out := make([]Run, 0, min(len(rows), recentMax))
	for _, row := range rows[:min(len(rows), recentMax)] {
		p := propertyOf(st, row.rec.F)
		value := item.Display(p, row.rec.To, st, who, now)
		out = append(out, Run{
			At: row.rec.At, Item: row.item.ID, Display: row.item.DisplayID(w.Prefix),
			Text: automation.ActivityText(names[row.rec.Actor.Rule], automation.ChangeText(p.Name, value)),
		})
	}
	return out, nil
}

// ruleRows は、自動化の行を新しい順に並べる。同じアイテム・項目・変更前・変更後の行は、最も早い 1 行だけ残す。
func ruleRows(r port.Reader, ws workspace.ID) ([]ruleRow, error) {
	ps, err := r.Projects().List(ws)
	if err != nil {
		return nil, err
	}
	var rows []ruleRow
	for _, p := range ps {
		its, err := r.Items().List(p.ID)
		if err != nil {
			return nil, err
		}
		for _, it := range its {
			log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: it.ID})
			if err != nil {
				return nil, err
			}
			rows = append(rows, earliest(it, log)...)
		}
	}
	slices.SortStableFunc(rows, func(a, b ruleRow) int { return b.rec.At.Compare(a.rec.At) })
	return rows, nil
}

// earliest は、アイテムのログの自動化の行。同じ変更は最も早い 1 行だけ。新しい行を先にする。
func earliest(it item.Item, log []change.Record) []ruleRow {
	var out []ruleRow
	for i := len(log) - 1; i >= 0; i-- {
		rec := log[i]
		if rec.Actor.Kind != change.ActorAutomation {
			continue
		}
		dup := slices.IndexFunc(out, func(o ruleRow) bool {
			return o.rec.F == rec.F && bytes.Equal(o.rec.From, rec.From) && bytes.Equal(o.rec.To, rec.To)
		})
		if dup >= 0 {
			out[dup].rec = rec // 古いほうへ置き換える(ログは古い順なので、さかのぼるほど早い)
			continue
		}
		out = append(out, ruleRow{rec: rec, item: it})
	}
	return out
}

func propertyOf(st workspace.Settings, f string) workspace.Property {
	for _, p := range st.Properties {
		if item.Field(p) == f {
			return p
		}
	}
	return workspace.Property{Name: f}
}

// memberNames は、ログイン名から表示名を引く。メンバーにいなければ「不明なユーザー」(01)。
func memberNames(r port.Reader) (func(string) string, error) {
	ms, err := r.Members().List()
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, m := range ms {
		names[strings.ToLower(m.Login)] = m.Name
	}
	return func(login string) string {
		if n, ok := names[strings.ToLower(login)]; ok {
			return n
		}
		return "不明なユーザー"
	}, nil
}
```

注: `item.Display` の `names` の引数の型は `item.Names`。関数型でなければ、`app/items/view.go` の `editContext` の作り方に合わせる。`earliest` で、重なりを置き換えたあとの行の並びが新しい順でなくなるときは、`ruleRows` の並べ替え(`At` の新しい順、同じなら `rec.ID` の大きい順)で直す。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/automations/`
Expected: PASS

- [ ] **Step 5: 失敗するテストを書く(制約を無視して適用)**

`apply_test.go`:

```go
package automations_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

func TestApplyIgnoringWorkflow(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, _ := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, apptest.SampleStatusDone); err != nil {
		t.Fatal(err)
	}
	if last := rec.summaries[len(rec.summaries)-1]; last != "「完了で外す」の変更を制約を無視して適用" {
		t.Errorf("summary = %q", last)
	}
	var recs []change.Record
	_ = rec.Stores.Read(ctx, store, func(rd port.Reader) error {
		recs, _ = rd.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		return nil
	})
	got := recs[len(recs)-1]
	want := change.Actor{Kind: change.ActorAutomation, Login: "hanako", Rule: r.ID}
	if got.F != "status" || got.Actor != want || string(got.To) != `"`+string(apptest.SampleStatusDone)+`"` {
		t.Errorf("row = %+v", got) // 自動化が行った変更、by は押した人(B4-11)
	}
	n := len(recs)
	_ = svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, apptest.SampleStatusDone) // すでに Done: 何も保存しない
	_ = rec.Stores.Read(ctx, store, func(rd port.Reader) error {
		recs, _ = rd.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		return nil
	})
	if len(recs) != n {
		t.Error("the same status must not be saved")
	}
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, "01NORULE", apptest.SampleStatusDone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("no rule: %v", err)
	}
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, "01NOSTATUS"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("no status: %v", err)
	}
}
```

- [ ] **Step 6: 失敗を確かめる**

Run: `go test ./internal/app/automations/ -run TestApply`
Expected: FAIL

- [ ] **Step 7: 実装する**

`apply.go`:

```go
package automations

import (
	"bytes"
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// ApplyIgnoringWorkflow は、止まった自動化の通知の「制約を無視して {ステータス名} にする」(03「制約で止まったとき」)。
// 遷移の制約を検査せず、ルールが行った変更(by は押した人)として 1 回で保存する。自動化は動かさない(B4-11)。
func (s Service) ApplyIgnoringWorkflow(ctx context.Context, store port.StoreID, id item.ID, rule, status change.ID) error {
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	var name string
	err = s.Stores.Read(ctx, store, func(r port.Reader) error {
		it, err := r.Items().Get(id)
		if err != nil {
			return err
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: it.Place.Workspace})
		if err != nil {
			return err
		}
		n, ok := automation.Names(log)[rule]
		if !ok {
			return port.ErrNotFound
		}
		name = n
		return nil
	})
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: automation.IgnoreText(oneLine(name))}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		it, err := tx.Items().Get(id)
		if err != nil {
			return err
		}
		st, err := tx.Workspaces().Settings(it.Place.Workspace)
		if err != nil {
			return err
		}
		p, ok := st.Property("status")
		if !ok {
			return port.ErrNotFound
		}
		to, err := item.ParseInput(p, change.Value(status), st)
		if err != nil {
			return port.ErrNotFound
		}
		from := item.Current(it, p)
		if bytes.Equal(from, to) {
			return nil
		}
		return tx.WriteItems().Append(id, change.NewSet(item.Field(p), from, to).ByRule(rule))
	})
}
```

- [ ] **Step 8: 通ることを確かめる**

Run: `go test ./internal/app/automations/ ./internal/archtest/`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/app/automations
git commit -m "feat(app): 自動化の最近の実行と、制約を無視して適用する操作を足す(B4 Task 8)"
```

---

### Task 9: 利用者のステータスの変更で自動化を動かす(`app/items`)

**Files:**
- Modify: `internal/app/items/service.go`(`Inbox port.Inbox`)、`internal/app/items/setproperty.go`
- Create: `internal/app/items/automate.go`
- Test: `internal/app/items/automate_test.go`

**Interfaces:**
- Consumes: Task 1・2・3・5
- Produces: `items.Service.Inbox`。`SetProperty` がステータスの変更の保存の中で自動化を動かし、保存のあとに通知を書く。

- [ ] **Step 1: 失敗するテストを書く**

```go
package items_test

import (
	"context"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// setupAuto は、ルールを書いた Store と、通知を覚える受信箱。
func setupAuto(t *testing.T, rules ...automation.Rule) (items.Service, *recorder, *apptest.Inbox) {
	t.Helper()
	svc, rec := setupRec(t)
	inbox := &apptest.Inbox{}
	svc.Inbox = inbox
	op := port.Operation{Actor: change.User("taro"), Summary: "ルールを作成"}
	err := rec.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		for _, r := range rules {
			if err := tx.WriteWorkspaces().Append(apptest.SampleWorkspace, automation.Drafts(automation.Rule{}, r)...); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, rec, inbox
}

func autoRule(id string, trig automation.Trigger, trigStatus change.ID, act automation.Action, v string) automation.Rule {
	return automation.Rule{ID: change.ID(id), Name: "ルール" + id, Trigger: trig, TriggerStatus: trigStatus, Action: act, Value: v, Enabled: true}
}

func records(t *testing.T, s port.Stores, id item.ID) []change.Record {
	t.Helper()
	var recs []change.Record
	_ = s.Read(context.Background(), store, func(r port.Reader) error {
		var err error
		recs, err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		return err
	})
	return recs
}

func TestStatusChangeRunsRulesInTheSameSave(t *testing.T) {
	svc, rec, inbox := setupAuto(t, autoRule("01RULE1", automation.TriggerStatus, apptest.SampleStatusDone, automation.ActionAssignee, "taro"))
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"`+string(apptest.SampleStatusDone)+`"`)); err != nil {
		t.Fatal(err)
	}
	recs := records(t, rec.Stores, apptest.SampleTask)
	user, auto := recs[len(recs)-2], recs[len(recs)-1]
	if user.F != "status" || auto.F != "assignee" || auto.Op != user.Op || auto.Actor.Rule != "01RULE1" || auto.By != "hanako" {
		t.Errorf("rows = %+v / %+v", user, auto)
	}
	if last(rec) != "HH-2 のステータスを Done に変更" {
		t.Errorf("summary = %q", last(rec))
	}
	ns := inbox.Notices()
	if len(ns) != 1 || ns[0].Kind != port.NoticeAutomation || ns[0].Item != apptest.SampleTask ||
		ns[0].Text != "「ルール01RULE1」で担当者を taro に変更しました" {
		t.Errorf("notices = %+v", ns)
	}
}

func TestAutomationDoesNotChain(t *testing.T) {
	svc, rec, _ := setupAuto(t,
		autoRule("01RULE1", automation.TriggerStatus, apptest.SampleStatusInProgress, automation.ActionStatus, string(apptest.SampleStatusDone)),
		autoRule("01RULE2", automation.TriggerStatus, apptest.SampleStatusDone, automation.ActionAssignee, "taro"),
	)
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"`+string(apptest.SampleStatusInProgress)+`"`)); err != nil {
		t.Fatal(err)
	}
	it, _ := svc.Get(t.Context(), store, apptest.SampleTask)
	if it.Status != apptest.SampleStatusDone || it.Assignee != "" {
		t.Errorf("item = %+v (RULE2 must not run)", it)
	}
	_ = rec
}

func TestStoppedRuleNotifiesWithTheReason(t *testing.T) {
	svc, rec, inbox := setupAuto(t, autoRule("01RULE1", automation.TriggerStatus, apptest.SampleStatusInProgress, automation.ActionStatus, string(apptest.SampleStatusDone)))
	// InProgress からは Pending にだけ行ける
	op := port.Operation{Actor: change.User("taro"), Summary: "制約"}
	_ = rec.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace,
			change.NewAdd("workflow/"+string(apptest.SampleStatusInProgress), apptest.SampleStatusPending))
	})
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", raw(`"`+string(apptest.SampleStatusInProgress)+`"`)); err != nil {
		t.Fatal(err)
	}
	ns := inbox.Notices()
	if len(ns) != 1 || ns[0].Kind != port.NoticeAutomationStopped || ns[0].Text != "「ルール01RULE1」が制約で止まりました" ||
		ns[0].Reason != "ワークフローの制約: InProgress から Done には変更できません(Pending のみ)" || ns[0].Status != apptest.SampleStatusDone {
		t.Errorf("notices = %+v", ns)
	}
	if recs := records(t, rec.Stores, apptest.SampleTask); recs[len(recs)-1].Actor.Kind == change.ActorAutomation {
		t.Error("a stopped run saves nothing")
	}
}

func TestStepsDoneRunsOnTheParentOnce(t *testing.T) {
	svc, rec, inbox := setupAuto(t, autoRule("01RULE1", automation.TriggerStepsDone, "", automation.ActionStatus, string(apptest.SampleStatusDone)))
	a, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "設計")
	b, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "実装")
	if n := len(inbox.Notices()); n != 0 {
		t.Fatalf("AddStep must not run rules: %d", n)
	}
	if _, err := svc.CheckStep(t.Context(), store, a.ID, true); err != nil {
		t.Fatal(err)
	}
	if n := len(inbox.Notices()); n != 0 {
		t.Fatalf("one step is still open: %d", n)
	}
	if _, err := svc.CheckStep(t.Context(), store, b.ID, true); err != nil {
		t.Fatal(err)
	}
	task, _ := svc.Get(t.Context(), store, apptest.SampleTask)
	if task.Status != apptest.SampleStatusDone || len(inbox.Notices()) != 1 {
		t.Errorf("task = %+v, notices = %+v", task, inbox.Notices())
	}
	_ = rec
}

func TestCreateDoesNotRunRules(t *testing.T) {
	svc, _, inbox := setupAuto(t, autoRule("01RULE1", automation.TriggerStatus, apptest.SampleStatusTodo, automation.ActionAssignee, "taro"))
	if _, err := svc.Create(t.Context(), store, items.CreateInput{
		Project: apptest.SampleProject, Parent: apptest.SampleEpic, Kind: item.Task, Title: "新しい",
	}); err != nil {
		t.Fatal(err)
	}
	if n := len(inbox.Notices()); n != 0 {
		t.Errorf("create must not run rules: %d", n)
	}
}
```

注: `AddStep`・`Create`・`CreateInput` の引数と戻り値は、いまの `app/items` の形に合わせて直す(`addstep.go`・`create.go` を読む)。テストの意図(作成と Step の追加では動かない、最後の Step で親に 1 回だけ動く)は変えない。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/items/ -run 'Status|Automation|Stopped|StepsDone|CreateDoesNot'`
Expected: FAIL(`Inbox` がない)

- [ ] **Step 3: 実装する**

`service.go` の `Service` に足す:

```go
	Inbox    port.Inbox // 自動化の通知(B4)
```

`automate.go`:

```go
package items

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// fired は、1 つのアイテムで動いたルールの結果。
type fired struct {
	item     item.Item
	outcomes []automation.Outcome
}

// automate は、利用者がアイテム id のステータスを from から変えたあとに、同じ保存の中で自動化を動かす
// (03「実行の規則」)。止まった実行は何も書かない。結果は保存のあとの通知に使う。
func automate(tx port.Tx, st workspace.Settings, id item.ID, from change.ID) ([]fired, error) {
	it, err := tx.Items().Get(id)
	if err != nil {
		return nil, err
	}
	log, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: it.Place.Workspace})
	if err != nil {
		return nil, err
	}
	rules := automation.FromLog(log)
	out, err := runOn(tx, st, rules, it, automation.Event{Trigger: automation.TriggerStatus, Status: it.Status})
	if err != nil || it.Kind != item.Step || it.Place.Parent == nil {
		return out, err
	}
	steps, err := siblings(tx, it)
	if err != nil || !automation.StepsDone(st, steps, map[item.ID]change.ID{it.ID: from}) {
		return out, err
	}
	parent, err := tx.Items().Get(*it.Place.Parent)
	if err != nil {
		return nil, err
	}
	more, err := runOn(tx, st, rules, parent, automation.Event{Trigger: automation.TriggerStepsDone})
	return append(out, more...), err
}

// runOn は、it でルールを動かし、変えた行を書く。
func runOn(tx port.Tx, st workspace.Settings, rules []automation.Rule, it item.Item, ev automation.Event) ([]fired, error) {
	os := automation.Run(st, rules, it, ev)
	if len(os) == 0 {
		return nil, nil
	}
	var ds []change.Draft
	for _, o := range os {
		if o.Draft != nil {
			ds = append(ds, *o.Draft)
		}
	}
	return []fired{{item: it, outcomes: os}}, tx.WriteItems().Append(it.ID, ds...)
}

// siblings は、it と同じ親の Step。it は利用者が変えた値で数える(自動化の変更を入れない。B4-8)。
func siblings(tx port.Tx, it item.Item) ([]item.Item, error) {
	all, err := tx.Items().List(it.Place.Project)
	if err != nil {
		return nil, err
	}
	var out []item.Item
	for _, x := range all {
		if x.Kind != item.Step || x.Place.Parent == nil || *x.Place.Parent != *it.Place.Parent {
			continue
		}
		if x.ID == it.ID {
			x = it
		}
		out = append(out, x)
	}
	return out, nil
}

// notify は、保存のあとに、動いたルールの通知を書く(B4-14)。
func (s Service) notify(ctx context.Context, store port.StoreID, ec editContext, fs []fired) error {
	for _, f := range fs {
		for _, o := range f.outcomes {
			n := port.Notice{Store: store, Workspace: f.item.Place.Workspace, Item: f.item.ID, Rule: o.Rule.ID}
			if o.Draft == nil {
				n.Kind, n.Text, n.Reason = port.NoticeAutomationStopped, automation.StoppedText(o.Rule.Name), o.Stopped
				n.Status = change.ID(o.Rule.Value)
			} else {
				value := item.Display(o.Prop, o.To, ec.settings, ec.names, s.Clock.Now())
				n.Kind, n.Text = port.NoticeAutomation, automation.AppliedText(o.Rule.Name, automation.ChangeText(o.Prop.Name, value))
			}
			if err := s.Inbox.Add(ctx, n); err != nil {
				return err
			}
		}
	}
	return nil
}
```

注: `runOn` で `it` は自分の変更のあと(`tx.Items().Get` は同じ保存で書いたものを含む)。`automate` の 1 回目の `runOn` が Step の値を変えても、`siblings` は利用者の値の `it` を使う。

`setproperty.go` の `SetProperty` の `writeValue` の呼び出しを、ステータスのときは自動化を動かす形にする。`writeValue` に、保存の中で呼ぶ `after` を渡せるようにする:

```go
	var fs []fired
	isStatus := p.Builtin && p.Key == "status"
	it, err := s.writeValue(ctx, store, id, summary, func(it item.Item) []change.Draft {
		from := item.Current(it, p)
		if bytes.Equal(from, to) {
			return nil
		}
		return []change.Draft{change.NewSet(item.Field(p), from, to)}
	}, func(tx port.Tx, before item.Item) error {
		if !isStatus {
			return nil
		}
		var err error
		fs, err = automate(tx, ec.settings, id, before.Status)
		return err
	})
	if err != nil {
		return item.Item{}, err
	}
	if err := s.notify(ctx, store, ec, fs); err != nil {
		return item.Item{}, err
	}
	return s.Get(ctx, store, id)
```

`writeValue` は次にする(`after` は、行を書いたときだけ呼ぶ。`nil` なら呼ばない。ほかの呼び出し元は `nil` を渡す。戻り値はいまのまま `s.Get` の結果でよいが、`SetProperty` は通知のあとに読み直す):

```go
func (s Service) writeValue(ctx context.Context, store port.StoreID, id item.ID, summary string,
	drafts func(item.Item) []change.Draft, after func(tx port.Tx, before item.Item) error,
) (item.Item, error) {
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
		ds := drafts(it)
		if err := tx.WriteItems().Append(id, ds...); err != nil {
			return err
		}
		if len(ds) == 0 || after == nil {
			return nil
		}
		return after(tx, it)
	})
	if err != nil {
		return item.Item{}, err
	}
	return s.Get(ctx, store, id)
}
```

`after` は行を書いたとき(値が変わったとき)だけ呼ばれる。`writeValue` を呼ぶほかの箇所(`togglevalue.go` ほか)には `nil` を渡す。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/items/ && go test ./internal/archtest/`
Expected: PASS(既存のテストの `items.Service` は `Inbox` を持たないが、ルールがなければ `notify` は `Inbox` を呼ばないので落ちない)

- [ ] **Step 5: Commit**

```bash
git add internal/app/items
git commit -m "feat(app): ステータスの変更と Step の完了で、同じ保存の中で自動化を動かす(B4 Task 9)"
```

---

### Task 10: ステータスの削除でルールを追従させる(`app/workspaces`)

**Files:**
- Modify: `internal/app/workspaces/service.go`(`Inbox port.Inbox`)、`internal/app/workspaces/value_delete.go`
- Test: `internal/app/workspaces/value_test.go` に足す(600 行を超えるなら `follow_test.go` に足す)

**Interfaces:**
- Consumes: Task 2・3・4・6

- [ ] **Step 1: 失敗するテストを書く**

```go
// ステータスの削除で、ルールのきっかけ・することを書き換え、同じになれば無効にし、削除した人に通知する(B4-17)。
func TestDeleteStatusRewritesRules(t *testing.T) {
	s := setup(t)
	inbox := &apptest.Inbox{}
	s.Inbox = inbox
	ctx := t.Context()
	w, _ := s.Create(ctx, store, "Web", "HH")
	st := settingsOf(t, s, w.ID)
	todo, doing := st.Statuses[0].ID, st.Statuses[1].ID
	r1 := automation.Rule{ID: "01RULE1", Name: "着手で外す", Trigger: automation.TriggerStatus, TriggerStatus: doing,
		Action: automation.ActionAssignee, Value: automation.Unassigned, Enabled: true}
	r2 := automation.Rule{ID: "01RULE2", Name: "Todo で着手", Trigger: automation.TriggerStatus, TriggerStatus: todo,
		Action: automation.ActionStatus, Value: string(doing), Enabled: true}
	op := port.Operation{Actor: change.User("taro"), Summary: "ルール"}
	err := s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(w.ID, append(automation.Drafts(automation.Rule{}, r1), automation.Drafts(automation.Rule{}, r2)...)...)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteValue(ctx, store, w.ID, workspace.List{Kind: workspace.ListStatuses}, doing, todo); err != nil {
		t.Fatal(err)
	}
	var rules []automation.Rule
	_ = s.Stores.Read(ctx, store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
		rules = automation.FromLog(log)
		return err
	})
	if rules[0].TriggerStatus != todo || !rules[0].Enabled || rules[1].Value != string(todo) || rules[1].Enabled {
		t.Errorf("rules = %+v", rules)
	}
	ns := inbox.Notices()
	want := []string{
		"自動化「着手で外す」は、ステータス「InProgress」の削除で、きっかけのステータスを「Todo」に書き換えました",
		"自動化「Todo で着手」は、ステータスの削除で、きっかけと同じステータスへの変更になったため、無効にしました",
	}
	if len(ns) != 2 || ns[0].Text != want[0] || ns[1].Text != want[1] || ns[0].Kind != port.NoticeAutomation || ns[0].Item != "" {
		t.Errorf("notices = %+v", ns)
	}
}
```

注: `setup`・`settingsOf`・`workspace.List` の作り方は、`value_test.go` の既存のテストに合わせる。`s.Stores` を直接使えない形なら、テストの下ごしらえで Store を別に持つ。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/workspaces/ -run TestDeleteStatusRewritesRules`
Expected: FAIL

- [ ] **Step 3: 実装する**

`service.go` の `Service` に `Inbox port.Inbox // ステータスの削除でルールが変わった通知(B4-17)` を足す。

`value_delete.go` の `DeleteValue` を次にする(ルールの行を足し、保存のあとに通知する):

```go
func (s Service) DeleteValue(ctx context.Context, store port.StoreID, ws workspace.ID, l workspace.List, id, replacement change.ID) error {
	var changes []automation.RuleChange
	var names [2]string // 削除した値、置き換え先
	err := s.editValue(ctx, store, ws, l, id,
		func(st workspace.Settings, v workspace.Value) string {
			return fmt.Sprintf("%s「%s」を削除", st.Label(l), oneLine(v.Name))
		},
		func(tx port.Tx, st workspace.Settings, vs []workspace.Value, v workspace.Value) ([]change.Draft, error) {
			repl, err := deleteChecked(st, l, vs, id, replacement)
			if err != nil {
				return nil, err
			}
			if err := replaceItems(tx, st, l, ws, id, repl); err != nil {
				return nil, err
			}
			ds := []change.Draft{change.NewDelete(l.Ref(id))}
			if l.Kind == workspace.ListStatuses {
				ds = append(ds, workspace.WorkflowDeleteDrafts(st, id)...)
				rds, rcs, err := ruleDrafts(tx, ws, id, repl)
				if err != nil {
					return nil, err
				}
				ds, changes = append(ds, rds...), rcs
				names = [2]string{v.Name, valueName(vs, repl)}
			}
			return append(ds, workspace.TemplateValueDrafts(st, l, id, repl)...), nil
		})
	if err != nil {
		return err
	}
	return s.notifyRules(ctx, store, ws, changes, names)
}

// replaceItems は、使っているアイテムの値を置き換える・外す(B2-8)。
func replaceItems(tx port.Tx, st workspace.Settings, l workspace.List, ws workspace.ID, id, repl change.ID) error {
	its, err := workspaceItems(tx, ws)
	if err != nil {
		return err
	}
	for _, it := range its {
		if ds := replaceRows(st, l, it, id, repl); len(ds) > 0 {
			if err := tx.WriteItems().Append(it.ID, ds...); err != nil {
				return err
			}
		}
	}
	return nil
}

// ruleDrafts は、ステータス id を指すルールの書き換え(03「削除」の自動化)。
func ruleDrafts(tx port.Tx, ws workspace.ID, id, repl change.ID) ([]change.Draft, []automation.RuleChange, error) {
	log, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
	if err != nil {
		return nil, nil, err
	}
	ds, rcs := automation.StatusDeleteDrafts(automation.FromLog(log), id, repl)
	return ds, rcs, nil
}

func valueName(vs []workspace.Value, id change.ID) string {
	for _, v := range vs {
		if v.ID == id {
			return v.Name
		}
	}
	return ""
}

// notifyRules は、変わったルールごとに、削除した人の受信箱へ通知を 1 件書く(B4-17)。
func (s Service) notifyRules(ctx context.Context, store port.StoreID, ws workspace.ID, changes []automation.RuleChange, names [2]string) error {
	for _, c := range changes {
		text := automation.RewrittenText(c.Rule.Name, names[0], c.Part, names[1])
		if c.Disabled {
			text = automation.DisabledByDeleteText(c.Rule.Name)
		}
		n := port.Notice{Kind: port.NoticeAutomation, Store: store, Workspace: ws, Rule: c.Rule.ID, Text: text}
		if err := s.Inbox.Add(ctx, n); err != nil {
			return err
		}
	}
	return nil
}
```

注: `editValue` の 4 つ目の引数(`workspace.Value`)が消す値でなければ、`vs` から `id` の名前を引く(`valueName(vs, id)`)。110 行 → 約 160 行で、ファイル 300 行の内。ファイルは足さない(B4-13 の満杯の件)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/workspaces/ ./internal/archtest/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/workspaces
git commit -m "feat(app): ステータスの削除で自動化のルールを書き換え、削除した人に通知する(B4 Task 10)"
```

---

### Task 11: `workspace.yaml` にルールを書き出す(gitstore)

**Files:**
- Modify: `internal/infra/gitstore/yaml.go`(76 行目の呼び出し)、`internal/infra/gitstore/yaml_settings.go`(`workspaceToYAML`)、`internal/infra/gitstore/yaml_templates.go`(`rulesNode`)
- Test: `internal/infra/gitstore/yaml_settings_test.go` に足す

- [ ] **Step 1: 失敗するテストを書く**

`TestWorkspaceYAMLHasWorkflowAndTemplates` と同じ作り方で、ルールを 1 つ書いた Store の `workspace.yaml` に次が含まれることを確かめる:

```go
func TestWorkspaceYAMLHasRules(t *testing.T) {
	// 既存の TestWorkspaceYAMLHasWorkflowAndTemplates と同じ下ごしらえで Store を作り、
	// automation.Drafts(automation.Rule{}, r) をワークスペースに書く。r は:
	r := automation.Rule{ID: "01RULE1", Name: "完了で外す", Trigger: automation.TriggerStatus, TriggerStatus: done,
		Action: automation.ActionAssignee, Value: automation.Unassigned, Enabled: true}
	// workspace.yaml を読み、次の文字が含まれることを確かめる(行末の注釈はステータスの名前)。
	want := []string{
		"rules:\n",
		"name: 完了で外す\n",
		"trigger: status\n",
		"trigger_status: " + string(done) + " # Done\n",
		"action: assignee\n",
		"value: '@none'\n",
		"enabled: true\n",
	}
	_ = r
	_ = want
}
```

`done` は既存のテストのステータスの ID の取り方に合わせる。ルールがないワークスペースでは `rules: []` を書く(テンプレートと同じ)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/infra/gitstore/ -run TestWorkspaceYAMLHasRules`
Expected: FAIL

- [ ] **Step 3: 実装する**

`yaml.go` の呼び出しで、ログからルールを読んで渡す:

```go
		v = workspaceToYAML(w, workspace.RankOf(log), st, automation.FromLog(log))
```

`workspaceToYAML(w workspace.Workspace, rank string, s workspace.Settings, rules []automation.Rule)` の最後に `key("rules"), rulesNode(s, rules),` を足す。`yaml_templates.go` の末尾に:

```go
// rulesNode は、自動化のルール(作った順)。ステータスの値は行末に名前の注釈。
func rulesNode(s workspace.Settings, rules []automation.Rule) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode}
	for _, r := range rules {
		ts := idNode(r.TriggerStatus)
		ts.LineComment = comment(valueName(s.Statuses, r.TriggerStatus))
		v := strNode(r.Value)
		if r.Action == automation.ActionStatus {
			v.LineComment = comment(valueName(s.Statuses, change.ID(r.Value)))
		}
		n.Content = append(n.Content, mapping(false,
			key("id"), strNode(string(r.ID)),
			key("name"), strNode(r.Name),
			key("trigger"), strNode(string(r.Trigger)),
			key("trigger_status"), ts,
			key("all_merged"), boolNode(r.AllMerged),
			key("action"), strNode(string(r.Action)),
			key("value"), v,
			key("enabled"), boolNode(r.Enabled),
		))
	}
	return n
}
```

`trigger_status` が空なら `null`(`idNode`)。空の一覧は `[]` で書かれることを、既存の `templatesNode` の書き方に合わせて確かめる(必要なら `Style: yaml.FlowStyle` を空のときだけ付ける)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/infra/gitstore/ ./internal/archtest/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infra/gitstore
git commit -m "feat(gitstore): workspace.yaml に自動化のルールを書き出す(B4 Task 11)"
```

---

### Task 12: 自動化の API と組み立て

**Files:**
- Create: `internal/adapter/httpapi/automations/automations.go`、`internal/adapter/httpapi/automations/dto.go`、`internal/adapter/httpapi/automations/automations_test.go`
- Modify: `internal/adapter/httpapi/routes.go`(`Deps.Inbox`、`automations.Register`、`items`・`workspaces` の `Inbox`)
- Modify: `cmd/hedgehug/wire.go`(`noInbox`)、`cmd/hedgehug/wire_e2e.go`(`&apptest.Inbox{}`)
- Regenerate: `api/openapi.yaml`、`web/packages/api/src/generated.ts`

**Interfaces:**
- Produces(operationId と形):

| operationId | メソッドとパス | 本文 | 応答 |
|---|---|---|---|
| `get-workspace-automation` | GET `/api/stores/{store}/workspaces/{workspace}/automation` | — | `WorkspaceAutomation{rules, recent}` |
| `create-automation-rule` | POST `.../automation/rules` | `AutomationRuleInput` | 201 `AutomationRule` |
| `update-automation-rule` | PUT `.../automation/rules/{rule}` | `AutomationRuleInput` | 204 |
| `delete-automation-rule` | DELETE `.../automation/rules/{rule}` | — | 204 |
| `set-automation-rule-enabled` | PUT `.../automation/rules/{rule}/enabled` | `SetAutomationRuleEnabled{enabled}` | 204 |
| `apply-automation-ignoring-workflow` | POST `/api/stores/{store}/items/{item}/automation/{rule}/apply` | `ApplyAutomationIgnoringWorkflow{status}` | 204 |

- [ ] **Step 1: 失敗するテストを書く**

`workspaces_test.go`(`httpapi/workspaces`)の作り方に合わせて、偽物の Store で API を立て、次を確かめる:

```go
// 作成 → 一覧 → スイッチ → 削除が通り、名前の重複は 422 で文言どおり、知らない値は 404。
func TestAutomationAPI(t *testing.T) {
	api := newTestAPI(t) // httpapi/workspaces のテストの下ごしらえを写す
	body := map[string]any{"name": "完了で外す", "trigger": "status", "triggerStatus": sampleDone,
		"allMerged": false, "action": "assignee", "value": "@none"}
	res := api.Post(wsPath+"/automation/rules", body)
	assertStatus(t, res, http.StatusCreated)
	var created struct{ ID string `json:"id"`; Enabled bool `json:"enabled"` }
	decode(t, res, &created)
	if !created.Enabled {
		t.Error("a new rule starts enabled")
	}
	res = api.Post(wsPath+"/automation/rules", body)
	assertStatus(t, res, http.StatusUnprocessableEntity) // 「同じ名前のルールがあります」
	res = api.Get(wsPath + "/automation")
	var view struct {
		Rules  []map[string]any `json:"rules"`
		Recent []map[string]any `json:"recent"`
	}
	decode(t, res, &view)
	if len(view.Rules) != 1 || view.Recent == nil {
		t.Errorf("view = %+v", view) // recent は空でも [] で返す
	}
	assertStatus(t, api.Put(wsPath+"/automation/rules/"+created.ID+"/enabled", map[string]any{"enabled": false}), http.StatusNoContent)
	assertStatus(t, api.Delete(wsPath+"/automation/rules/"+created.ID), http.StatusNoContent)
	bad := maps.Clone(body)
	bad["value"] = "jiro"
	assertStatus(t, api.Post(wsPath+"/automation/rules", bad), http.StatusNotFound)
}
```

`newTestAPI`・`assertStatus`・`decode`・`wsPath`・`sampleDone` は、`httpapi/workspaces` のテストにある同じ役目の関数・値の名前と形に置き換える(新しく作らず、そのパッケージのテストの書き方を写す)。

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/automations/`
Expected: FAIL

- [ ] **Step 3: 実装する**

`dto.go`:

```go
package automations

import (
	"time"

	appautomations "github.com/kukv/hedgehug/internal/app/automations"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// AutomationRuleInput は、ルールの作成と編集の本文。
type AutomationRuleInput struct {
	Name          string `json:"name"`
	Trigger       string `json:"trigger" enum:"pr_open,review_requested,pr_merged,issue_closed,status,steps_done"`
	TriggerStatus string `json:"triggerStatus" doc:"きっかけが status のときのステータスの ID。ほかは空"`
	AllMerged     bool   `json:"allMerged" doc:"きっかけが pr_merged のときの条件"`
	Action        string `json:"action" enum:"status,assignee"`
	Value         string `json:"value" doc:"ステータスの ID、ログイン名、または @none(未割り当て)"`
}

// AutomationRule は、ルール。
type AutomationRule struct {
	ID string `json:"id"`
	AutomationRuleInput
	Enabled bool `json:"enabled"`
}

// AutomationRun は、最近の実行の 1 行。
type AutomationRun struct {
	At      time.Time `json:"at"`
	Item    string    `json:"item"`
	Display string    `json:"display"`
	Text    string    `json:"text"`
}

// WorkspaceAutomation は、自動化のサブタブの中身。
type WorkspaceAutomation struct {
	Rules  []AutomationRule `json:"rules"`
	Recent []AutomationRun  `json:"recent"`
}

// SetAutomationRuleEnabled は、スイッチの本文。
type SetAutomationRuleEnabled struct {
	Enabled bool `json:"enabled"`
}

// ApplyAutomationIgnoringWorkflow は、「制約を無視して…」の本文。
type ApplyAutomationIgnoringWorkflow struct {
	Status string `json:"status" doc:"変えるステータスの ID(通知の Status)"`
}

func fromInput(in AutomationRuleInput) automation.Rule {
	return automation.Rule{
		Name: in.Name, Trigger: automation.Trigger(in.Trigger), TriggerStatus: change.ID(in.TriggerStatus),
		AllMerged: in.AllMerged, Action: automation.Action(in.Action), Value: in.Value,
	}
}

func toRule(r automation.Rule) AutomationRule {
	return AutomationRule{ID: string(r.ID), Enabled: r.Enabled, AutomationRuleInput: AutomationRuleInput{
		Name: r.Name, Trigger: string(r.Trigger), TriggerStatus: string(r.TriggerStatus),
		AllMerged: r.AllMerged, Action: string(r.Action), Value: r.Value,
	}}
}

func toView(v appautomations.View) WorkspaceAutomation {
	out := WorkspaceAutomation{Rules: []AutomationRule{}, Recent: []AutomationRun{}}
	for _, r := range v.Rules {
		out.Rules = append(out.Rules, toRule(r))
	}
	for _, r := range v.Recent {
		out.Recent = append(out.Recent, AutomationRun{At: r.At, Item: string(r.Item), Display: r.Display, Text: r.Text})
	}
	return out
}
```

注: huma が埋め込みの `AutomationRuleInput` を平らに出さないときは、`AutomationRule` に項目を並べ直す(A1 の「埋め込みにしない」の前例)。

`automations.go`:

```go
// Package automations は、自動化の API(仕様 03「自動化」)。
package automations

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appautomations "github.com/kukv/hedgehug/internal/app/automations"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const base = "/api/stores/{store}/workspaces/{workspace}/automation"

type wsInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
}

type ruleInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Rule      string `path:"rule"`
}

type createInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Body      AutomationRuleInput
}

type updateInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Rule      string `path:"rule"`
	Body      AutomationRuleInput
}

type enabledInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Rule      string `path:"rule"`
	Body      SetAutomationRuleEnabled
}

type applyInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Rule  string `path:"rule"`
	Body  ApplyAutomationIgnoringWorkflow
}

type viewOutput struct{ Body WorkspaceAutomation }
type ruleOutput struct{ Body AutomationRule }

// Register は、自動化の API を登録する。
func Register(api huma.API, svc appautomations.Service) {
	huma.Register(api, huma.Operation{OperationID: "get-workspace-automation", Method: http.MethodGet, Path: base,
		Summary: "自動化のルールと最近の実行"},
		func(ctx context.Context, in *wsInput) (*viewOutput, error) {
			v, err := svc.List(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &viewOutput{Body: toView(v)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "create-automation-rule", Method: http.MethodPost, Path: base + "/rules",
		Summary: "ルールを作成", DefaultStatus: http.StatusCreated},
		func(ctx context.Context, in *createInput) (*ruleOutput, error) {
			r, err := svc.Create(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), fromInput(in.Body))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &ruleOutput{Body: toRule(r)}, nil
		})
	registerEdits(api, svc)
}

func registerEdits(api huma.API, svc appautomations.Service) {
	huma.Register(api, huma.Operation{OperationID: "update-automation-rule", Method: http.MethodPut, Path: base + "/rules/{rule}",
		Summary: "ルールを編集", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *updateInput) (*struct{}, error) {
			return nil, apierr.From(svc.Update(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Rule), fromInput(in.Body)))
		})
	huma.Register(api, huma.Operation{OperationID: "delete-automation-rule", Method: http.MethodDelete, Path: base + "/rules/{rule}",
		Summary: "ルールを削除", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *ruleInput) (*struct{}, error) {
			return nil, apierr.From(svc.Delete(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Rule)))
		})
	huma.Register(api, huma.Operation{OperationID: "set-automation-rule-enabled", Method: http.MethodPut, Path: base + "/rules/{rule}/enabled",
		Summary: "ルールを有効・無効にする", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *enabledInput) (*struct{}, error) {
			return nil, apierr.From(svc.SetEnabled(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), change.ID(in.Rule), in.Body.Enabled))
		})
	huma.Register(api, huma.Operation{OperationID: "apply-automation-ignoring-workflow", Method: http.MethodPost,
		Path: "/api/stores/{store}/items/{item}/automation/{rule}/apply", Summary: "止まった自動化の変更を、制約を無視して適用",
		DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *applyInput) (*struct{}, error) {
			return nil, apierr.From(svc.ApplyIgnoringWorkflow(ctx, port.StoreID(in.Store), item.ID(in.Item), change.ID(in.Rule), change.ID(in.Body.Status)))
		})
}
```

`apierr.From(nil)` が `nil` を返すことは既存の `templates.go` と同じ前提。

`routes.go`:
- `Deps` の「トラック B」として `Inbox port.Inbox // 受信箱(本物は C4)` を足す。
- `items.Register(api, appitems.Service{..., Inbox: deps.Inbox})`、`workspaces.Register(api, appworkspaces.Service{..., Inbox: deps.Inbox})`。
- `automations.Register(api, appautomations.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity, Clock: deps.Clock})` を 1 行足す。

`cmd/hedgehug/wire.go`(`noIdentity` の下):

```go
// noInbox は、受信箱(C4)が入るまでの仮。通知を捨てる(B4-12)。
type noInbox struct{}

func (noInbox) Add(context.Context, port.Notice) error { return nil }
```

`Deps` に `Inbox: noInbox{},`。`wire_e2e.go` の `Deps` に `Inbox: &apptest.Inbox{},`。

- [ ] **Step 4: 生成物を作り直し、確かめる**

```bash
make generate
cd web && pnpm gen && cd ..
go test ./internal/adapter/... ./cmd/... ./internal/archtest/
```

Expected: PASS。`api/openapi.yaml` に 6 つの operationId、`generated.ts` に `getWorkspaceAutomation`・`createAutomationRule`・`updateAutomationRule`・`deleteAutomationRule`・`setAutomationRuleEnabled`・`applyAutomationIgnoringWorkflow` ができる。

- [ ] **Step 5: Commit**

```bash
git add internal/adapter/httpapi cmd/hedgehug api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(adapter): 自動化の API を足し、受信箱の口を組み立てる(B4 Task 12)"
```

---

### Task 13: 画面の口と規則(core・api)

**Files:**
- Modify: `web/packages/core/src/model/workspace.ts`(型)、`web/packages/core/src/port/workspaces.ts`(口)、`web/packages/api/src/workspaces.ts`(実装)
- Create: `web/packages/core/src/features/workspaces/automation.ts`、`automation.test.ts`
- Modify: `web/packages/core/src/index.ts`(export)、`web/packages/core/src/features/workspaces/settings.ts`(読み込み)
- Modify: `web/packages/core/src/features/workspaces/fixtures.test-util.ts`(偽の口に足す)

**Interfaces:**
- Produces:

```ts
// model/workspace.ts
export type AutomationTrigger = "pr_open" | "review_requested" | "pr_merged" | "issue_closed" | "status" | "steps_done";
export type AutomationAction = "status" | "assignee";
export interface AutomationRuleInput {
  name: string; trigger: AutomationTrigger; triggerStatus: string; allMerged: boolean;
  action: AutomationAction; value: string;
}
export interface AutomationRule extends AutomationRuleInput { id: string; enabled: boolean; }
export interface AutomationRun { at: Date; item: string; display: string; text: string; }
export interface WorkspaceAutomation { rules: AutomationRule[]; recent: AutomationRun[]; }

// port/workspaces.ts の WorkspacesPort に足す
automation(storeId: string, workspaceId: string): Promise<WorkspaceAutomation>;
createRule(storeId: string, workspaceId: string, input: AutomationRuleInput): Promise<AutomationRule>;
updateRule(storeId: string, workspaceId: string, ruleId: string, input: AutomationRuleInput): Promise<void>;
deleteRule(storeId: string, workspaceId: string, ruleId: string): Promise<void>;
setRuleEnabled(storeId: string, workspaceId: string, ruleId: string, enabled: boolean): Promise<void>;

// settings.ts の SettingsState に足す
automation: WorkspaceAutomation | null; // 自動化のサブタブを開いているときだけ読む(B4-22)

// features/workspaces/automation.ts
export const automationHelp: string;
export const rulesEmpty = "ルールはありません";
export const recentEmpty = "まだ動いていません";
export const ruleCreated = "ルールを作成しました";
export const ruleDeleted = "ルールを削除しました";
export const deleteRuleTitle = "ルールを削除";
export const deleteRuleBody: (name: string) => string;
export const switchRefusal: string;
export const triggerOptions: { value: AutomationTrigger; label: string }[];
export const actionOptions: { value: AutomationAction; label: string }[];
export const unassignedOption = "未割り当てにする";
export const allMergedLabel = "リンク中の PR がすべてマージ済みのときだけ";
export function ruleSummary(settings: WorkspaceSettings, members: Member[], r: AutomationRuleInput): string;
export interface RuleRow { id: string; name: string; summary: string; enabled: boolean; }
export function ruleRows(a: WorkspaceAutomation, settings: WorkspaceSettings, members: Member[]): RuleRow[];
export function newRuleForm(settings: WorkspaceSettings): AutomationRuleInput;
export function withTrigger(form: AutomationRuleInput, trigger: AutomationTrigger, settings: WorkspaceSettings): AutomationRuleInput;
export function withAction(form: AutomationRuleInput, action: AutomationAction, settings: WorkspaceSettings): AutomationRuleInput;
export function ruleFormRefusal(form: AutomationRuleInput, rules: AutomationRule[], editingId: string): string | null;
export function switchOnRefusal(r: AutomationRule): string | null;
export function valueOptions(form: AutomationRuleInput, settings: WorkspaceSettings, members: Member[]): { value: string; label: string }[];
export interface RecentRow { display: string; text: string; time: string; }
export function recentRows(a: WorkspaceAutomation, now: Date): RecentRow[];
```

- [ ] **Step 1: 失敗するテストを書く**

`automation.test.ts`(設定は `fixtures.test-util.ts` の既存のもの。ステータスは Todo・InProgress・Done・Pending・Cancel、完了の既定は Done):

```ts
import { describe, expect, it } from "vitest";
import {
  newRuleForm,
  recentRows,
  ruleFormRefusal,
  ruleRows,
  ruleSummary,
  switchOnRefusal,
  withAction,
  withTrigger,
} from "./automation";
import { settingsFixture } from "./fixtures.test-util";

const settings = settingsFixture();
const id = (name: string) => settings.statuses.find((v) => v.name === name)?.id ?? "";
const members = [{ login: "tanaka", name: "田中" }];

describe("ruleSummary", () => {
  it("writes the trigger and the action (03 の例)", () => {
    const r = { name: "x", trigger: "status", triggerStatus: id("Done"), allMerged: false, action: "assignee", value: "@none" } as const;
    expect(ruleSummary(settings, members, r)).toBe("ステータスが Done になったとき → 担当者を 未割り当て に");
  });
  it("uses the merged-all wording only in the list", () => {
    const r = { name: "x", trigger: "pr_merged", triggerStatus: "", allMerged: true, action: "status", value: id("Done") } as const;
    expect(ruleSummary(settings, members, r)).toBe("リンクした PR がすべてマージされたとき → ステータスを Done に");
  });
  it("shows the member's name", () => {
    const r = { name: "x", trigger: "steps_done", triggerStatus: "", allMerged: false, action: "assignee", value: "tanaka" } as const;
    expect(ruleSummary(settings, members, r)).toBe("Step がすべて完了したとき → 担当者を 田中 に");
  });
});

describe("the rule form", () => {
  it("starts with PR merged, change status, and the done default", () => {
    expect(newRuleForm(settings)).toEqual({
      name: "", trigger: "pr_merged", triggerStatus: "", allMerged: false, action: "status", value: id("Done"),
    });
  });
  it("resets the value when the action changes", () => {
    const f = withAction(newRuleForm(settings), "assignee", settings);
    expect(f.value).toBe("@none");
    expect(withAction(f, "status", settings).value).toBe(settings.statuses[0]?.id);
  });
  it("gives the status trigger the first status and drops the condition", () => {
    const f = withTrigger({ ...newRuleForm(settings), allMerged: true }, "status", settings);
    expect(f.triggerStatus).toBe(settings.statuses[0]?.id);
    expect(f.allMerged).toBe(false);
  });
  it("refuses like the server", () => {
    const rules = [{ ...newRuleForm(settings), id: "R1", name: "完了で外す", enabled: true }];
    const f = newRuleForm(settings);
    expect(ruleFormRefusal({ ...f, name: " " }, rules, "")).toBe("名前を入力してください");
    expect(ruleFormRefusal({ ...f, name: " 完了で外す " }, rules, "")).toBe("同じ名前のルールがあります");
    expect(ruleFormRefusal({ ...f, name: "完了で外す" }, rules, "R1")).toBeNull();
    const same = { ...f, name: "y", trigger: "status" as const, triggerStatus: id("Done"), value: id("Done") };
    expect(ruleFormRefusal(same, rules, "")).toBe("きっかけと同じステータスには変更できません");
  });
});

describe("the list", () => {
  it("refuses to switch on a rule that changes to its trigger status", () => {
    const r = { name: "y", trigger: "status", triggerStatus: "S", allMerged: false, action: "status", value: "S", id: "R", enabled: false } as const;
    expect(switchOnRefusal(r)).toBe("きっかけと同じステータスには変更できません。ルールを編集してください");
  });
  it("lists rules and recent runs", () => {
    const a = {
      rules: [{ ...newRuleForm(settings), id: "R1", name: "PR マージで完了", enabled: true }],
      recent: [{ at: new Date("2026-11-13T08:55:00Z"), item: "I", display: "HH-2", text: "「PR マージで完了」でステータスを Done に変更" }],
    };
    expect(ruleRows(a, settings, members)[0]).toEqual({
      id: "R1", name: "PR マージで完了", summary: "リンクした PR がマージされたとき → ステータスを Done に", enabled: true,
    });
    expect(recentRows(a, new Date("2026-11-13T09:00:00Z"))).toEqual([
      { display: "HH-2", text: "「PR マージで完了」でステータスを Done に変更", time: "5 分前" },
    ]);
  });
});
```

`settingsFixture` と `Member` の型の名前は、`fixtures.test-util.ts` と `model/member.ts` の実際の名前に合わせる。

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test -- automation`
Expected: FAIL(`./automation` がない)

- [ ] **Step 3: 実装する**

`automation.ts`:

```ts
import type { Member } from "../../model/member";
import type {
  AutomationAction,
  AutomationRule,
  AutomationRuleInput,
  AutomationTrigger,
  WorkspaceAutomation,
  WorkspaceSettings,
} from "../../model/workspace";
import { relativeTime } from "../dates/relative";

// 自動化のタブ(03「自動化」)。文言は 03 の文字どおり。

export const automationHelp =
  "GitHub の出来事やステータスの変化で、アイテムを自動で変えます。自動化が変えたものは受信箱の「自動化」に出ます。";
export const rulesEmpty = "ルールはありません";
export const recentEmpty = "まだ動いていません";
export const ruleCreated = "ルールを作成しました";
export const ruleDeleted = "ルールを削除しました";
export const deleteRuleTitle = "ルールを削除";
export const deleteRuleBody = (name: string) => `「${name}」を削除します。`;
export const switchRefusal = "きっかけと同じステータスには変更できません。ルールを編集してください";
export const unassignedOption = "未割り当てにする";
export const allMergedLabel = "リンク中の PR がすべてマージ済みのときだけ";
const unassigned = "@none";
const sameStatus = "きっかけと同じステータスには変更できません";

export const triggerOptions: { value: AutomationTrigger; label: string }[] = [
  { value: "pr_open", label: "PR がリンクされて Open のとき" },
  { value: "review_requested", label: "レビューが依頼されたとき" },
  { value: "pr_merged", label: "リンクした PR がマージされたとき" },
  { value: "issue_closed", label: "リンクした Issue が閉じたとき" },
  { value: "status", label: "ステータスが次になったとき" },
  { value: "steps_done", label: "Step がすべて完了したとき" },
];

export const actionOptions: { value: AutomationAction; label: string }[] = [
  { value: "status", label: "ステータスを変更" },
  { value: "assignee", label: "担当者を変更" },
];

const statusName = (s: WorkspaceSettings, id: string) => s.statuses.find((v) => v.id === id)?.name ?? "";

function triggerText(s: WorkspaceSettings, r: AutomationRuleInput): string {
  if (r.trigger === "status") return `ステータスが ${statusName(s, r.triggerStatus)} になったとき`;
  if (r.trigger === "pr_merged" && r.allMerged) return "リンクした PR がすべてマージされたとき";
  return triggerOptions.find((o) => o.value === r.trigger)?.label ?? "";
}

function memberName(members: Member[], login: string): string {
  const m = members.find((x) => x.login.toLowerCase() === login.toLowerCase());
  return m?.name || login;
}

function actionText(s: WorkspaceSettings, members: Member[], r: AutomationRuleInput): string {
  if (r.action === "status") return `ステータスを ${statusName(s, r.value)} に`;
  return `担当者を ${r.value === unassigned ? "未割り当て" : memberName(members, r.value)} に`;
}

// ruleSummary は、一覧の説明「{きっかけ} → {すること}」(B4-18)。
export function ruleSummary(s: WorkspaceSettings, members: Member[], r: AutomationRuleInput): string {
  return `${triggerText(s, r)} → ${actionText(s, members, r)}`;
}

export interface RuleRow {
  id: string;
  name: string;
  summary: string;
  enabled: boolean;
}

export function ruleRows(a: WorkspaceAutomation, s: WorkspaceSettings, members: Member[]): RuleRow[] {
  return a.rules.map((r) => ({ id: r.id, name: r.name, summary: ruleSummary(s, members, r), enabled: r.enabled }));
}

const doneDefault = (s: WorkspaceSettings) => s.categoryDefaults.done ?? s.statuses[0]?.id ?? "";
const firstStatus = (s: WorkspaceSettings) => s.statuses[0]?.id ?? "";

// newRuleForm は、作成ダイアログの初期値(03)。
export function newRuleForm(s: WorkspaceSettings): AutomationRuleInput {
  return { name: "", trigger: "pr_merged", triggerStatus: "", allMerged: false, action: "status", value: doneDefault(s) };
}

// withTrigger は、きっかけを変える。条件は pr_merged のときだけ残し、ステータスのきっかけには先頭のステータス(B4-19)。
export function withTrigger(f: AutomationRuleInput, trigger: AutomationTrigger, s: WorkspaceSettings): AutomationRuleInput {
  return {
    ...f,
    trigger,
    triggerStatus: trigger === "status" ? f.triggerStatus || firstStatus(s) : "",
    allMerged: trigger === "pr_merged" ? f.allMerged : false,
  };
}

// withAction は、することを変え、値を初期化する(ステータスなら先頭、担当者なら未割り当て)。
export function withAction(f: AutomationRuleInput, action: AutomationAction, s: WorkspaceSettings): AutomationRuleInput {
  return { ...f, action, value: action === "status" ? firstStatus(s) : unassigned };
}

export function valueOptions(f: AutomationRuleInput, s: WorkspaceSettings, members: Member[]) {
  if (f.action === "status") return s.statuses.map((v) => ({ value: v.id, label: v.name }));
  return [{ value: unassigned, label: unassignedOption }, ...members.map((m) => ({ value: m.login, label: m.name || m.login }))];
}

const sameStatusRule = (r: AutomationRuleInput) =>
  r.trigger === "status" && r.action === "status" && r.triggerStatus === r.value;

// ruleFormRefusal は、Go の Check と同じ検査と文言(03 の保存の条件)。通るなら null。
export function ruleFormRefusal(f: AutomationRuleInput, rules: AutomationRule[], editingId: string): string | null {
  const name = f.name.trim();
  if (name === "") return "名前を入力してください";
  if (rules.some((r) => r.id !== editingId && r.name === name)) return "同じ名前のルールがあります";
  return sameStatusRule(f) ? sameStatus : null;
}

// switchOnRefusal は、スイッチを ON にする前の検査(03)。
export function switchOnRefusal(r: AutomationRule): string | null {
  return sameStatusRule(r) ? switchRefusal : null;
}

export interface RecentRow {
  display: string;
  text: string;
  time: string;
}

export function recentRows(a: WorkspaceAutomation, now: Date): RecentRow[] {
  return a.recent.map((r) => ({ display: r.display, text: r.text, time: relativeTime(r.at, now) }));
}
```

`settings.ts`:
- `SettingsState` に `automation: WorkspaceAutomation | null;` を足す。
- `reload` で、`tab === "automation"` のときだけ `port.automation(storeId, workspaceId)` を `Promise.all` に足し、ほかのときは `null` を入れる。
- `selectTab(next)` で、`next === "automation"` なら、`tab` を変えたあと `void reload()` する(ほかのサブタブはいまのまま)。

`settings.test.ts` に、「自動化のサブタブを選ぶと automation を読み、ほかのサブタブでは読まない」テストを足す(既存の `selectTab` のテストの書き方に合わせる)。

`api/workspaces.ts` の口の実装に足す(`generated.ts` の関数名は Task 12 のもの):

```ts
    automation: (s, w) =>
      call(async () => {
        const v = await getWorkspaceAutomation(s, w);
        return {
          rules: (v.rules ?? []).map(toRule),
          recent: (v.recent ?? []).map((r) => ({ ...r, at: new Date(r.at) })),
        };
      }),
    createRule: (s, w, input) => call(async () => toRule(await createAutomationRule(s, w, input))),
    updateRule: (s, w, id, input) => call(() => updateAutomationRule(s, w, id, input)),
    deleteRule: (s, w, id) => call(() => deleteAutomationRule(s, w, id)),
    setRuleEnabled: (s, w, id, enabled) => call(() => setAutomationRuleEnabled(s, w, id, { enabled })),
```

`toRule` は API の型(`trigger`・`action` が文字列の列挙)を core の型にそのまま写す関数として、同じファイルに置く。`fixtures.test-util.ts` の偽の口と、`packages/api` の口の照合テスト(`ports.test.ts` があればその形)に、5 つの関数を足す。

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test && pnpm --filter @hedgehug/api test && pnpm check`(`pnpm check` がなければ `make check` の web の部分)
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/packages
git commit -m "feat(core): 自動化のルールの一覧・ダイアログの規則と口を足す(B4 Task 13)"
```

---

### Task 14: 自動化のサブタブ(画面)

**Files:**
- Modify: `web/packages/ui/src/primitives/Switch.svelte`(`onchange`)、`web/packages/ui/src/primitives/inputs.test.ts`
- Create: `web/app/src/screens/workspace-settings/AutomationPanel.svelte`、`RuleDialog.svelte`
- Modify: `web/app/src/screens/workspace-settings/SettingsTab.svelte`

- [ ] **Step 1: 失敗するテストを書く(Switch)**

`inputs.test.ts` に、既存の部品のテストの書き方で:

```ts
it("Switch with onchange stays as given and reports the next value", async () => {
  const seen: boolean[] = [];
  render(Switch, { props: { label: "有効", checked: false, onchange: (v: boolean) => seen.push(v) } });
  const sw = screen.getByRole("switch", { name: "有効" });
  await fireEvent.click(sw);
  expect(seen).toEqual([true]);
  expect(sw).not.toBeChecked(); // 押しても見た目を変えない(B4-23)
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm --filter @hedgehug/ui test -- inputs`
Expected: FAIL

- [ ] **Step 3: `Switch` を直す**

```svelte
<script lang="ts">
  interface Props {
    checked?: boolean;
    label: string;
    disabled?: boolean;
    title?: string;
    // onchange を渡したら、押しても見た目を変えず、次の値を渡すだけにする(保存のあとの読み直しで変わる)。
    onchange?: (checked: boolean) => void;
  }

  let { checked = $bindable(false), label, disabled = false, title, onchange }: Props = $props();

  function changed(e: Event & { currentTarget: HTMLInputElement }) {
    if (!onchange) {
      checked = e.currentTarget.checked;
      return;
    }
    const next = e.currentTarget.checked;
    e.currentTarget.checked = checked;
    onchange(next);
  }
</script>

<span class="switch" {title}>
  <input type="checkbox" role="switch" aria-checked={checked} aria-label={label} {disabled} {checked} onchange={changed}>
  <span></span>
</span>
```

(`<style>` はいまのまま。)

- [ ] **Step 4: 画面を作る**

`AutomationPanel.svelte`(200 行以内):

```svelte
<script lang="ts">
  import {
    automationHelp,
    deleteRuleBody,
    deleteRuleTitle,
    readOnlyNote,
    recentEmpty,
    recentRows,
    ruleDeleted,
    ruleRows,
    rulesEmpty,
    type SettingsState,
    switchOnRefusal,
  } from "@hedgehug/core";
  import { Button, ConfirmDialog, IconButton, Switch } from "@hedgehug/ui";
  import { toasts, workspaceSettings } from "../../wire";
  import RuleDialog from "./RuleDialog.svelte";

  // 自動化のタブ(03「自動化」)。
  interface Props {
    state: SettingsState;
  }

  let { state: s }: Props = $props();

  let editing = $state<string | "new" | null>(null);
  let deleting = $state<string | null>(null);

  const automation = $derived(s.automation);
  const rows = $derived(automation ? ruleRows(automation, s.detail.settings, s.members) : []);
  const recent = $derived(automation ? recentRows(automation, new Date()) : []);
  const title = $derived(s.readOnly ? readOnlyNote : undefined);
  const editingRule = $derived(automation?.rules.find((r) => r.id === editing) ?? null);
  const deletingName = $derived(rows.find((r) => r.id === deleting)?.name ?? "");

  function toggle(id: string, on: boolean) {
    const rule = automation?.rules.find((r) => r.id === id);
    const refusal = on && rule ? switchOnRefusal(rule) : null;
    if (refusal) {
      toasts.show(refusal, { error: true });
      return;
    }
    void workspaceSettings.run((p, st, w) => p.setRuleEnabled(st, w, id, on));
  }

  function remove() {
    const id = deleting;
    deleting = null;
    if (id === null) return;
    void workspaceSettings.run((p, st, w) => p.deleteRule(st, w, id), ruleDeleted);
  }
</script>

{#if automation}
  <div class="panel">
    <p class="help">{automationHelp}</p>
    <div class="rows">
      {#each rows as row (row.id)}
        <div class="row">
          <Switch
            label={`${row.name} を有効にする`}
            checked={row.enabled}
            disabled={s.readOnly}
            {title}
            onchange={(on) => toggle(row.id, on)}
          />
          <span class="name">{row.name}</span>
          <span class="summary">{row.summary}</span>
          <span class="grow"></span>
          <Button onclick={() => (editing = row.id)}>編集</Button>
          <IconButton label="削除" size="sm" disabled={s.readOnly} {title} onclick={() => (deleting = row.id)}>🗑</IconButton>
        </div>
      {:else}
        <p class="empty">{rulesEmpty}</p>
      {/each}
    </div>
    <div>
      <Button disabled={s.readOnly} {title} onclick={() => (editing = "new")}>ルールを作成</Button>
    </div>
    <section aria-label="最近の実行">
      <h3>最近の実行</h3>
      {#each recent as r, i (i)}
        <div class="run">
          <span class="time">{r.time}</span>
          <span class="id">{r.display}</span>
          <span>{r.text}</span>
        </div>
      {:else}
        <p class="empty">{recentEmpty}</p>
      {/each}
    </section>
  </div>
{/if}

{#if editing !== null && automation}
  <RuleDialog state={s} rules={automation.rules} rule={editingRule} onclose={() => (editing = null)} />
{/if}

{#if deleting !== null}
  <ConfirmDialog
    title={deleteRuleTitle}
    lines={[deleteRuleBody(deletingName)]}
    ok="削除"
    danger
    onok={remove}
    oncancel={() => (deleting = null)}
  />
{/if}

<style>
  .panel { display: flex; flex-direction: column; gap: 16px; max-width: 720px; }
  .rows { display: flex; flex-direction: column; }
  .row { display: flex; align-items: center; gap: 8px; padding: 6px 0; border-bottom: 1px solid var(--line); flex-wrap: wrap; }
  .name { font-weight: 600; }
  .summary, .empty, .help, .time { font-size: 12.5px; color: var(--muted); }
  .empty, .help { margin: 0; }
  .grow { flex: 1; }
  h3 { font-size: 13px; margin: 0 0 6px; }
  .run { display: flex; gap: 8px; font-size: 12.5px; padding: 3px 0; }
  .id { font-family: var(--mono); }
</style>
```

`toasts` の import 先と名前は、`wire.ts` の実際の export に合わせる(B3 の画面の使い方を写す)。スイッチの `label` は e2e で使うので、この形にする。

`RuleDialog.svelte`(200 行以内。`TemplateDialog.svelte` の形を写す):

```svelte
<script lang="ts">
  import {
    actionOptions,
    allMergedLabel,
    type AutomationAction,
    type AutomationRule,
    type AutomationRuleInput,
    type AutomationTrigger,
    newRuleForm,
    readOnlyNote,
    ruleCreated,
    ruleFormRefusal,
    type SettingsState,
    triggerOptions,
    valueOptions,
    withAction,
    withTrigger,
  } from "@hedgehug/core";
  import { Button, Checkbox, Dialog, Input, Select } from "@hedgehug/ui";
  import { workspaceSettings } from "../../wire";

  // ルールの作成と編集のダイアログ(03「きっかけとすること」)。
  interface Props {
    state: SettingsState;
    rules: AutomationRule[];
    rule: AutomationRule | null; // null は新規
    onclose: () => void;
  }

  let { state: s, rules, rule, onclose }: Props = $props();

  const settings = $derived(s.detail.settings);
  let form = $state<AutomationRuleInput>(rule ? { ...rule } : newRuleForm(s.detail.settings));
  let error = $state("");
  const disabled = $derived(s.readOnly);
  const title = $derived(s.readOnly ? readOnlyNote : undefined);
  const statusOptions = $derived(settings.statuses.map((v) => ({ value: v.id, label: v.name })));

  async function save() {
    const refusal = ruleFormRefusal(form, rules, rule?.id ?? "");
    if (refusal) {
      error = refusal;
      return;
    }
    const input = { ...form, name: form.name.trim() };
    const ok = await workspaceSettings.run(
      (p, st, w) => (rule ? p.updateRule(st, w, rule.id, input) : p.createRule(st, w, input)),
      rule ? undefined : ruleCreated,
      (message) => (error = message),
    );
    if (ok) onclose();
  }
</script>

<Dialog open title={rule ? "ルールの編集" : "ルールを作成"} {onclose}>
  <div class="form">
    <Input label="名前" bind:value={form.name} {disabled} {title} focusOnMount />
    <Select
      label="きっかけ"
      value={form.trigger}
      options={triggerOptions}
      {disabled}
      {title}
      onchange={(v) => (form = withTrigger(form, v as AutomationTrigger, settings))}
    />
    {#if form.trigger === "status"}
      <Select label="ステータス" value={form.triggerStatus} options={statusOptions} {disabled} {title}
        onchange={(v) => (form = { ...form, triggerStatus: v })} />
    {/if}
    {#if form.trigger === "pr_merged"}
      <Checkbox label={allMergedLabel} bind:checked={form.allMerged} {disabled} {title} />
    {/if}
    <Select
      label="すること"
      value={form.action}
      options={actionOptions}
      {disabled}
      {title}
      onchange={(v) => (form = withAction(form, v as AutomationAction, settings))}
    />
    <Select label="値" value={form.value} options={valueOptions(form, settings, s.members)} {disabled} {title}
      onchange={(v) => (form = { ...form, value: v })} />
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
  </div>
  {#snippet footer()}
    <Button onclick={onclose}>{disabled ? "閉じる" : "キャンセル"}</Button>
    <Button variant="primary" {disabled} {title} onclick={save}>保存</Button>
  {/snippet}
</Dialog>

<style>
  .form { display: flex; flex-direction: column; gap: 12px; }
  .error { margin: 0; color: var(--danger); font-size: 12.5px; }
</style>
```

`Dialog` の `footer` の渡し方、エラーの色のトークン名、`Checkbox` の `bind:checked` の可否は、`TemplateDialog.svelte` の書き方に合わせる。

`SettingsTab.svelte` に `{:else if $settings.tab === "automation"}<AutomationPanel state={$settings} />` と import を足す。

- [ ] **Step 5: 確かめる**

Run: `cd web && pnpm --filter @hedgehug/ui test && pnpm check && pnpm lint`(名前は `web/package.json` の scripts に合わせる)
Expected: PASS。`make dev`(あれば)で設定の「自動化」を開き、作成・スイッチ・削除が動くことを目で確かめる。

- [ ] **Step 6: Commit**

```bash
git add web/packages/ui web/app
git commit -m "feat(web): ワークスペースの設定に「自動化」のサブタブを足す(B4 Task 14)"
```

---

### Task 15: 通しのテスト(e2e)

**Files:**
- Create: `web/e2e/automation.spec.ts`

ほかの e2e と同じサーバーを 3 つの幅で共有するので、ルールはサンプルのワークスペース「Web」に作らない(ほかのテストのステータスの変更で動いてしまう)。テストごとに新しいワークスペースを作り、プロジェクトとアイテムは API で作る。

- [ ] **Step 1: テストを書く**

```ts
import { expect, type Page, type TestInfo, test } from "@playwright/test";
import { sidebar, tokenUrl, unique } from "./helpers";

// 自動化の通しのテスト(B4)。ルールはテストごとのワークスペースに作る(ほかのテストの操作で動かないように)。

const e2eStore = "01E2ESTORE0000000000000001";
const auth = { Authorization: "Bearer e2e-token" };
const prefixOf = (info: TestInfo) =>
  `A${info.project.name[0]?.toUpperCase()}${info.retry}${Math.floor(Math.random() * 1e5)}`;

async function createWorkspace(page: Page, info: TestInfo, name: string, prefix: string) {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await side.getByRole("region", { name: "e2e" }).getByRole("button", { name: "ワークスペースを追加" }).click();
  const dialog = page.getByRole("dialog", { name: "ワークスペースを追加" });
  await dialog.getByRole("textbox", { name: "名前" }).fill(name);
  await dialog.getByRole("textbox", { name: "接頭辞" }).fill(prefix);
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(dialog).toBeHidden();
}

// newTaskByApi は、ワークスペース prefix にプロジェクト・Epic・Task を API で作り、Task の ID とステータスの ID を返す。
async function newTaskByApi(page: Page, prefix: string) {
  const req = page.request;
  const list = await (await req.get(`/api/stores/${e2eStore}/workspaces`, { headers: auth })).json();
  const ws = list.find((w: { prefix: string }) => w.prefix === prefix).id as string;
  const detail = await (await req.get(`/api/stores/${e2eStore}/workspaces/${ws}`, { headers: auth })).json();
  const status = (name: string) => detail.settings.statuses.find((v: { name: string }) => v.name === name).id as string;
  const project = await (
    await req.post(`/api/stores/${e2eStore}/workspaces/${ws}/projects`, { headers: auth, data: { title: "自動化" } })
  ).json();
  const epic = await (
    await req.post(`/api/stores/${e2eStore}/items`, { headers: auth, data: { project: project.id, kind: "Epic", title: "E" } })
  ).json();
  const task = await (
    await req.post(`/api/stores/${e2eStore}/items`, {
      headers: auth,
      data: { project: project.id, parent: epic.id, kind: "Task", title: "T", values: { assignee: "taro" } },
    })
  ).json();
  return { ws, task: task.id as string, status };
}

async function openAutomation(page: Page, prefix: string) {
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/settings`));
  await page.getByRole("tab", { name: "自動化", exact: true }).click();
}

test("a rule runs on a status change and shows in recent runs", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "自動化"), prefix);
  const { task, status } = await newTaskByApi(page, prefix);
  await openAutomation(page, prefix);
  await expect(page.getByText("ルールはありません")).toBeVisible();
  await expect(page.getByText("まだ動いていません")).toBeVisible();

  await page.getByRole("button", { name: "ルールを作成" }).click();
  const dialog = page.getByRole("dialog", { name: "ルールを作成" });
  await dialog.getByRole("textbox", { name: "名前" }).fill("完了で外す");
  await dialog.getByRole("combobox", { name: "きっかけ" }).selectOption({ label: "ステータスが次になったとき" });
  await dialog.getByRole("combobox", { name: "ステータス" }).selectOption({ label: "Done" });
  await dialog.getByRole("combobox", { name: "すること" }).selectOption({ label: "担当者を変更" });
  await expect(dialog.getByRole("combobox", { name: "値" })).toHaveValue("@none");
  await dialog.getByRole("button", { name: "保存" }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText("ルールを作成しました")).toBeVisible();
  await expect(page.getByText("ステータスが Done になったとき → 担当者を 未割り当て に")).toBeVisible();

  const res = await page.request.put(`/api/stores/${e2eStore}/items/${task}/properties/status`, {
    headers: auth,
    data: { value: status("Done") },
  });
  expect(res.ok()).toBeTruthy();
  const it = await (await page.request.get(`/api/stores/${e2eStore}/items/${task}`, { headers: auth })).json();
  expect(it.assignee ?? "").toBe("");

  await openAutomation(page, prefix);
  const recent = page.getByRole("region", { name: "最近の実行" });
  await expect(recent).toContainText(`${prefix}-2`);
  await expect(recent).toContainText("「完了で外す」で担当者を なし に変更");
});

test("refuses a duplicate name and switching on a same-status rule", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "自動化の断り"), prefix);
  await openAutomation(page, prefix);
  for (const name of ["A", "A"]) {
    await page.getByRole("button", { name: "ルールを作成" }).click();
    const dialog = page.getByRole("dialog", { name: "ルールを作成" });
    await dialog.getByRole("textbox", { name: "名前" }).fill(name);
    await dialog.getByRole("button", { name: "保存" }).click();
    if (name === "A" && (await dialog.isVisible())) {
      await expect(dialog.getByRole("alert")).toHaveText("同じ名前のルールがあります");
      await dialog.getByRole("button", { name: "キャンセル" }).click();
    }
  }
  const sw = page.getByRole("switch", { name: "A を有効にする" });
  await expect(sw).toBeChecked();
  await sw.click();
  await expect(sw).not.toBeChecked();
});
```

注: アイテムの番号は、プロジェクトの作成ではなくアイテム用の番号で数える。Epic が 1、Task が 2(`${prefix}-2`)。違えば `list-project-items` の `number` で確かめて合わせる。「同じステータスの ON の断り」は、画面からは作れない(保存で断る)ので、e2e では確かめず core のテスト(Task 13)で確かめる。2 つ目のテストの名前は「重複と OFF」の内容に合わせて付け直す。

- [ ] **Step 2: 走らせる**

Run: `make e2e`(または `cd web && pnpm e2e`。ほかの e2e の動かし方に合わせる)
Expected: すべて PASS(3 幅)

- [ ] **Step 3: Commit**

```bash
git add web/e2e/automation.spec.ts
git commit -m "test(e2e): 自動化の通しのテストを足す(B4 Task 15)"
```

---

### Task 16: 仕上げ

- [ ] `make check` と e2e がすべて通ることを確かめる。
- [ ] `superpowers:requesting-code-review` で、ブランチ全体の最終レビューをする(code-review high)。指摘は直すか、理由を書いて見送る。
- [ ] この計画の末尾に「実装で計画から変えたこと」を書く。
- [ ] トラックの PR を 1 本出す(向き先 develop、タイトル「feat: トラック B4(自動化)」)。本文に、できること・決めたこと(B4-1〜B4-23)・範囲の外の申し送り(D4・C4・B5・C・E・A4)・確認の結果を書く。
