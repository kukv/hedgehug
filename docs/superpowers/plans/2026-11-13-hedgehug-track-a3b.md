# トラック A3 後半(テンプレートと遷移の制約) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 作成ダイアログでテンプレートを当てはめ、子と Step を 1 回の保存でまとめて作れるようにし、ステータスを変えるすべての経路(メニュー・Step のチェック・ツリーのボタン)に遷移の制約を効かせる。

**Architecture:** 遷移の検査は `domain/workspace.CheckTransition`(B3)を、`app/items.SetProperty` の保存の中(読み直した設定とアイテム)から呼ぶ。`CheckStep` は `SetProperty` を通るので同じ検査を受ける。画面のメニューは core の `transitionRefusal`(B3)で項目を無効にし、理由を出す。テンプレートは、作成の入力にテンプレートの内部 ID を足し、サーバーが保存の前に読んだテンプレートから子と Step を深さ優先の番号で作る。作成ダイアログへの当てはめは core の純粋な関数(新しいファイル `createTemplate.ts`)に置き、`createDialog.ts` はそれを呼ぶだけにする。

**Tech Stack:** Go(huma・domain/app/infra の層)、TypeScript(core・api の orval)、Svelte 5(ui・app)、Vitest、Playwright。

**Spec:** `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md`(「ステータスの変更」「アイテムの作成」「子と Step」)、`03-workspace-settings.md`(「遷移の制約」「テンプレート」の「持てるもの」「適用するときの規則」「種類ごとの既定」)。

**前提として読むもの:** A3 前半の計画 `docs/superpowers/plans/2026-11-13-hedgehug-track-a3a.md`(決定 1・2・5・6・12)、B3 の計画 `docs/superpowers/plans/2026-11-13-hedgehug-track-b3.md`、PR #170 本文の「A3b への申し送り」。

## Global Constraints

- 依存は追加しない(足すときは版を明記し、公開から 7 日以上の版だけ)。
- 大きさの上限: Go ファイル 300 行・関数 60 行・認知的複雑度 15・パッケージ 20 ファイル、Svelte 200 行、TS 300 行。超えたら分ける。`internal/app/items` はいま 16 ファイル。
- 向き: adapter→app→domain、infra→app/port。app の機能どうしは import しない(テストも)。
- 画面の文言は仕様の文字どおり。断りの文言は `ワークフローの制約: {いま} から {移行先} には変更できません({移れる先}のみ)`(B3 の `CheckTransition` と `transitionRefusal` がすでに持つ)。
- 作成のアクティビティは `{種類}「{タイトル}」を作成`、一緒に作ったものがあれば `{種類}「{タイトル}」ほか N 件を作成`。トーストは `{ID} を作成しました` / `{ID} ほか N 件を作成しました`。
- 各タスクは TDD(失敗するテスト → 失敗を確かめる → 実装 → 通す → コミット)。タスクごとにレビューする。
- コミットの末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`。
- PR はトラックで 1 本(develop 向け)。force push は拒否されるので、載せ直すときは別名のブランチで出す。
- 計画やテストにトークンらしい値(ghp_ など)を書かない。

---

## 持ち場と境目

| 入れるもの | 仕様 |
|---|---|
| 遷移の制約: `SetProperty` のステータス(詳細パネル・ツリー・子の行・Step の行のステータスのボタン)、`CheckStep`、ステータスのメニューの項目の無効と理由 | 02「ステータスの変更」「Step のチェック」、03「遷移の制約」 |
| 作成ダイアログのテンプレートの欄、既定のテンプレートの当てはめ(開いたとき・種類を切り替えたとき・ワークスペースを変えたとき)、テンプレートを選んだときの当てはめ、「一緒に作るもの」、「続けて作成」で残す | 02「開き方と初期値」「ダイアログの項目」「ダイアログの中での連動」「作成したあと」、03「適用するときの規則」 |
| テンプレートでまとめて作る保存(番号は深さ優先、子の値、Step の担当者、アクティビティとトーストの「ほか N 件」) | 02「作成したあと」、03「適用するときの規則」 |

| 入れないもの | 回す先 | 理由 |
|---|---|---|
| 作成ダイアログの説明の欄、テンプレートの説明の当てはめ(「テンプレート由来」の判定を含む)、子の説明の保存 | A4 | A3a の決定 6。説明の保存(`description.md`)がまだない |
| プロジェクト別のルール(使うステータス・プロジェクトの遷移の制約・プロジェクトの使うテンプレートと既定)、「(変更できる先がありません)」 | B5 | A3a の決定 3、B3 の申し送り。この計画の関数はワークスペースの設定だけを見る |
| ツリーの Space キー | A4 | A2 の申し送り(ツリーのキー操作は A4) |
| スプリント(もとになるアイテムだけに入れる) | F | A3a の決定 6 |
| 担当者への通知(一緒に作った子と Step を含む) | C4・B4 | 受信箱はまだない |
| カンバンのドラッグ・テーブルのセル・一括変更・コマンドパレット・自動化 | E・F・B4 | まだその画面がない。サーバーの `SetProperty` を通れば同じ検査を受ける |

## 決めたこと

| # | 決定 | 根拠 |
|---|---|---|
| A3b-1 | 遷移の検査は `SetProperty` の保存の中で、読み直したアイテムのいまのステータスと、読み直した設定で行う。`writeValue` の行を作る関数を `func(tx port.Tx, it item.Item) ([]change.Draft, error)` に変え、断りを返せるようにする | A3a の決定 2(差し込み口を 1 か所)。保存の前に読んだ値で検査すると、同時の変更を見落とす |
| A3b-2 | `CheckTransition` は、いまのステータスがワークスペースの一覧にないとき(削除された値のまま)は通す。core の `transitionRefusal` も同じにする | 02「ステータスの変更」 |
| A3b-3 | 作成・Step の追加には遷移の検査をしない(`create` は `SetProperty` を通らないので、何もしない。テストで確かめる) | 03「遷移の制約」の「適用しない変更」 |
| A3b-4 | ステータスのメニューの無効は `statusOptions(settings, current, workflow)` の第 3 引数で選ぶ。`StatusButton`(詳細パネル・ツリー・子の行・Step の行)は `true`、作成ダイアログのセレクト(`choiceRow`)は既定の `false` | 02「入力の検査」(作成時は制約を適用しない)。作成ダイアログは `StatusButton` を使わない(`statusSelect`) |
| A3b-5 | メニューの無効な項目は、分類のヒントを残したまま、理由を項目の下に小さく出し、`title` にも入れる(ui の `ValueOption` に `disabled`・`reason` を足す) | 02「ヒントに理由を出す」。タッチでもホバーなしで読めるように、`Menu` と同じく画面に出す |
| A3b-6 | 作成の入力にテンプレートの内部 ID(`CreateInput.Template`、API の `template`)を足す。サーバーは保存の前にテンプレートを読み、その同じ中身から、アクティビティの件数と作る子・Step を決める | アクティビティの「ほか N 件」と作ったものの件数を同じものから数える |
| A3b-7 | テンプレートがない、または種類が作るアイテムと違うときは `port.ErrNotFound`。タイトル・プロジェクト・親・値の断りを先に出す(テンプレートの検査は保存の中で最後) | B2-12(画面からは起きない参照の誤りは ErrNotFound)。02「入力の検査」の表の順を崩さない |
| A3b-8 | もとになるアイテムの値は、作成ダイアログで当てはめて送った値(いまのまま)。サーバーはテンプレートの値をもとになるアイテムに入れ直さない | 02「テンプレートを変えたとき」(入力で上書きできる) |
| A3b-9 | 子の値は、テンプレートの値のうち、子の種類で使い、表示中で、値の参照が正しいものだけを入れる(`workspace.TemplateValuesFor`)。合わない値は黙って捨て、作成を断らない | 03「表示中でなくなったプロパティや、その種類で使わないプロパティは適用しない」。子はダイアログで直せない |
| A3b-10 | 子・Step のステータスは初期ステータス、子の優先度はテンプレートの値か初期優先度、Step の優先度は初期優先度。Step の担当者は `@parent` ならすぐ上のアイテムの担当者、`@none` なら担当者なし、ほかはそのログイン名。タイトルは `NormalizeTitle` で整える | 03「適用するときの規則」、02「入力の検査」 |
| A3b-11 | 番号は、もとになるアイテムの番号の次から、子を並び順に、子ごとにその子の Step を並び順に振る(Story・Task・Bug のテンプレートは、もとになるアイテムの Step を並び順に)。ワークスペースのログに番号の行を 1 つずつ書く。並び順の値は、同じ親の下で `rank.After` を続けて振る | 02「作成したあと」 |
| A3b-12 | 作成ダイアログのフォームは `templateId`(空は「なし」)を持つ。テンプレートの欄は、その種類で使えるテンプレートがあるときだけ「なし」と並べて出す。使えるテンプレートはワークスペースの設定だけで決める(B5 でプロジェクトの絞り込みを足す) | 02「ダイアログの項目」、03「プロジェクトのテンプレート」は B5 |
| A3b-13 | 当てはめは、タイトルが空のときだけ書き出しを入れ、テンプレートに値のある項目のうち、その種類で表示中のものだけを上書きする。テンプレートの数の値(JSON の数)は、入力欄の文字列にして入れる | 03「適用するときの規則」、A3a の決定 12 |
| A3b-14 | 既定のテンプレートの当てはめは、開いたとき・種類を切り替えたとき・ワークスペースを変えたときだけ。どれも、いまのテンプレートを「なし」にしてから既定を当てはめる。プロジェクトを変えたとき・作成のあとの読み直しでは当てはめない(テンプレートはそのまま残す) | 02「ダイアログの中での連動」。プロジェクトで使えるテンプレートが変わるのは B5 |
| A3b-15 | 「一緒に作るもの」は B3 の `templateStructure` で作り、「親と同じ」の担当者には、いまの入力の担当者を出す | 02「ダイアログの項目」 |
| A3b-16 | トーストの「ほか N 件」は、送ったときに選んでいたテンプレートの子と Step の数で数える | テンプレートが送る前と保存の中で変わるのは同期と重なったときだけ。そのときもアクティビティはサーバーの数で正しい |

## ファイルの構成

| ファイル | 役割 | タスク |
|---|---|---|
| `internal/domain/workspace/workflow.go` | `CheckTransition` に、一覧にないいまのステータスを通す条件を足す | 1 |
| `internal/app/items/setproperty.go` | `SetProperty` のステータスの検査、`writeValue` の型 | 2 |
| `internal/app/items/togglevalue.go` | `writeValue` の新しい型に合わせる | 2 |
| `internal/domain/workspace/template_check.go` | `TemplateValuesFor` | 3 |
| `internal/app/items/create.go` | `CreateInput.Template`、保存の前のテンプレートの読み込み、まとめて作る | 3 |
| `internal/app/items/createtemplate.go`(新規) | テンプレートを読む・子と Step を作る・件数 | 3 |
| `internal/adapter/httpapi/items/items.go` | `CreateItem.Template` | 4 |
| `web/packages/core/src/model/item.ts`・`web/packages/api/src/items.ts` | `NewItem.templateId` を API に渡す | 4 |
| `web/packages/core/src/features/workspaces/workflow.ts` | `transitionRefusal` の一覧にないステータス | 5 |
| `web/packages/core/src/features/items/properties.ts` | `statusOptions` の第 3 引数、`ValueOption` の `disabled`・`reason` | 5 |
| `web/packages/ui/src/types.ts`・`web/packages/ui/src/patterns/ValueMenu.svelte` | 無効な項目と理由 | 5 |
| `web/app/src/screens/item/StatusButton.svelte` | `statusOptions(settings, status, true)` | 5 |
| `web/packages/core/src/features/items/createTemplate.ts`(新規) | テンプレートの選択肢・当てはめ・既定・一緒に作るもの・件数 | 6 |
| `web/packages/core/src/features/items/createDialog.ts` | フォームの `templateId`、状態の `templates`・`structure`、連動、送信、トースト | 7 |
| `web/packages/core/src/features/items/createDialogOptions.ts`(新規、行数を超えたときだけ) | `createDialog.ts` から選択肢を作る関数を移す | 7 |
| `web/app/src/screens/item/CreateItemFields.svelte` | テンプレートのセレクト | 8 |
| `web/app/src/screens/item/CreateStructure.svelte`(新規) | 「一緒に作るもの」の読み取り専用のツリー | 8 |
| `web/e2e/create-template.spec.ts`(新規) | 通しのテスト | 9 |

---

### Task 1: `CheckTransition` が一覧にないいまのステータスを通す

**Files:**
- Modify: `internal/domain/workspace/workflow.go`
- Test: `internal/domain/workspace/workflow_test.go`

**Interfaces:**
- Produces: `workspace.CheckTransition(s Settings, from, to change.ID) error`(形は変えない。from が `s.Statuses` にないと nil)

- [ ] **Step 1: 失敗するテストを書く**

`workflow_test.go` の末尾に足す。既存のテストが使っている設定の作り方(同じファイルの先頭の補助関数)に合わせ、ステータス Todo・InProgress・Done の設定 `s` を作る部分は既存のものを使う。

```go
func TestCheckTransitionSkipsADeletedCurrentStatus(t *testing.T) {
	s := workflowSettings() // 既存の補助関数(Todo・InProgress・Done)。名前が違えば合わせる
	gone := change.ID("01GONE000000000000000000001")
	// 削除されたステータスの行が同期で残っていても、いまの値が一覧にないなら検査しない(02「ステータスの変更」)。
	s.Workflow = map[change.ID][]change.ID{gone: {s.Statuses[1].ID}}
	if err := workspace.CheckTransition(s, gone, s.Statuses[2].ID); err != nil {
		t.Errorf("CheckTransition from a deleted status = %v, want nil", err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/workspace/ -run TestCheckTransitionSkipsADeletedCurrentStatus`
Expected: FAIL(`ワークフローの制約:  から Done には…` の断りが返る)

- [ ] **Step 3: 実装する**

`CheckTransition` の先頭の条件を次にする。

```go
// CheckTransition は、from から to への変更を遷移の制約で検査する(B3-2)。同じステータス、
// from が空、from がいまの一覧にない(削除された値のまま。02「ステータスの変更」)、制限のない行は通す。
func CheckTransition(s Settings, from, to change.ID) error {
	if from == "" || from == to || !hasValue(s.Statuses, from) {
		return nil
	}
```

(`hasValue` は `template_check.go` にある同じパッケージの関数。)

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/domain/workspace/`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/workspace/workflow.go internal/domain/workspace/workflow_test.go
git commit -m "fix(domain): いまのステータスが一覧にないときは遷移の制約を検査しない(A3b-2)"
```

---

### Task 2: `SetProperty` と `CheckStep` に遷移の制約を効かせる

**Files:**
- Modify: `internal/app/items/setproperty.go`
- Modify: `internal/app/items/togglevalue.go:39-48`
- Modify: `internal/app/items/checkstep.go`(コメントだけ)
- Test: `internal/app/items/workflow_test.go`(新規。テストファイルはパッケージのファイル数に数えないことを `internal/archtest` で確かめる。数えるなら `values_test.go` に足す)

**Interfaces:**
- Consumes: `workspace.CheckTransition`(Task 1)
- Produces: `func (s Service) writeValue(ctx context.Context, store port.StoreID, id item.ID, summary string, drafts func(tx port.Tx, it item.Item) ([]change.Draft, error)) (item.Item, error)`

- [ ] **Step 1: 失敗するテストを書く**

```go
package items_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// restrict は、サンプルのワークスペースで、from から to にだけ変えられるようにする(B3 の行の形で書く)。
func restrict(t *testing.T, rec *recorder, from change.ID, to ...change.ID) {
	t.Helper()
	ds := make([]change.Draft, 0, len(to))
	for _, id := range to {
		ds = append(ds, change.NewAdd(workspace.WorkflowField(from), id))
	}
	err := rec.Stores.Write(t.Context(), store, port.Operation{Actor: change.User("hanako"), Summary: "ワークフローの遷移の制約を変更"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...)
	})
	if err != nil {
		t.Fatal(err)
	}
}

const refusedTodoToDone = "ワークフローの制約: Todo から Done には変更できません(InProgress のみ)"

func TestSetPropertyRefusesAStatusTheWorkflowForbids(t *testing.T) {
	svc, rec := setupRec(t)
	restrict(t, rec, apptest.SampleStatusTodo, apptest.SampleStatusInProgress)
	before := logLen(t, rec, apptest.SampleTask)
	_, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", quoted(apptest.SampleStatusDone))
	if message(err) != refusedTodoToDone || logLen(t, rec, apptest.SampleTask) != before {
		t.Errorf("SetProperty = %v; log %d → %d", err, before, logLen(t, rec, apptest.SampleTask))
	}
	if _, err := svc.SetProperty(t.Context(), store, apptest.SampleTask, "status", quoted(apptest.SampleStatusInProgress)); err != nil {
		t.Errorf("an allowed status must be saved: %v", err)
	}
}

func TestCheckStepRefusesAStatusTheWorkflowForbids(t *testing.T) {
	svc, rec := setupRec(t)
	step, err := svc.AddStep(t.Context(), store, apptest.SampleTask, "確認")
	if err != nil {
		t.Fatal(err)
	}
	restrict(t, rec, apptest.SampleStatusTodo, apptest.SampleStatusInProgress)
	if _, err := svc.CheckStep(t.Context(), store, step.ID, true); message(err) != refusedTodoToDone {
		t.Errorf("CheckStep = %v", err)
	}
}

// 作成と Step の追加には遷移の制約を適用しない(A3b-3)。初期ステータス以外でも作れる。
func TestCreateAndAddStepIgnoreTheWorkflow(t *testing.T) {
	svc, rec := setupRec(t)
	restrict(t, rec, apptest.SampleStatusTodo, apptest.SampleStatusInProgress)
	_, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: "Task", Title: "完了で作る",
		Values: map[change.ID]json.RawMessage{"status": quoted(apptest.SampleStatusDone)},
	})
	if err != nil {
		t.Errorf("Create with Done = %v", err)
	}
	if _, err := svc.AddStep(t.Context(), store, apptest.SampleTask, "確認"); err != nil {
		t.Errorf("AddStep = %v", err)
	}
}
```

(`json` を import に足す。`quoted`・`ptr`・`message`・`setupRec`・`logLen` は既存のテストの補助関数。)

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/app/items/ -run 'Workflow'`
Expected: FAIL(`SetProperty` と `CheckStep` が保存してしまう)。`TestCreateAndAddStepIgnoreTheWorkflow` は、この時点で通る(作成が検査しないことの確認)。

- [ ] **Step 3: 実装する**

`setproperty.go` の `SetProperty` の最後と `writeValue` を次にする。

```go
	summary := fmt.Sprintf("%s の%sを %s に変更", ec.display, p.Name, item.Display(p, to, ec.settings, ec.names, s.Clock.Now()))
	return s.writeValue(ctx, store, id, summary, func(tx port.Tx, it item.Item) ([]change.Draft, error) {
		from := item.Current(it, p)
		if bytes.Equal(from, to) {
			return nil, nil
		}
		if err := checkStatus(tx, it, p, to); err != nil {
			return nil, err
		}
		return []change.Draft{change.NewSet(item.Field(p), from, to)}, nil
	})
}

// checkStatus は、ステータスを変えるとき、保存の中で読み直したアイテムと設定で遷移の制約を検査する(A3b-1)。
func checkStatus(tx port.Tx, it item.Item, p workspace.Property, to json.RawMessage) error {
	if p.Type != workspace.TypeStatus {
		return nil
	}
	st, err := tx.Workspaces().Settings(it.Place.Workspace)
	if err != nil {
		return err
	}
	var next change.ID
	_ = json.Unmarshal(to, &next)
	return workspace.CheckTransition(st, it.Status, next)
}
```

```go
// writeValue は、保存の中でいまのアイテムを読み直し、drafts が返す行を 1 回で保存する。行がなければ何も残さない。
// drafts が断ったら何も保存しない。
func (s Service) writeValue(ctx context.Context, store port.StoreID, id item.ID, summary string, drafts func(tx port.Tx, it item.Item) ([]change.Draft, error)) (item.Item, error) {
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
		ds, err := drafts(tx, it)
		if err != nil {
			return err
		}
		return tx.WriteItems().Append(id, ds...)
	})
	if err != nil {
		return item.Item{}, err
	}
	return s.Get(ctx, store, id)
}
```

`SetProperty` のコメントの「遷移の制約は後半(A3b)でここに足す(決定 2)。」を「ステータスは、保存の中で遷移の制約を検査する(A3b-1)。」に替える。`checkstep.go` のコメントの「遷移の制約は後半(A3b)で SetProperty の中に足す(決定 2)。」を「遷移の制約は SetProperty の中で検査する(A3b-1)。」に替える。

`togglevalue.go` は型だけ合わせる。

```go
	return s.writeValue(ctx, store, id, summary, func(_ port.Tx, it item.Item) ([]change.Draft, error) {
		has := slices.Contains(currentIDs(it, p), value)
		switch {
		case on && !has:
			return []change.Draft{change.NewAdd(item.Field(p), value)}, nil
		case !on && has:
			return []change.Draft{change.NewRemove(item.Field(p), value)}, nil
		}
		return nil, nil
	})
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/app/items/ && make lint`(`make lint` がなければ `golangci-lint run ./internal/...`)
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/app/items/
git commit -m "feat(app): ステータスの変更と Step のチェックに遷移の制約を効かせる(A3b-1・A3b-3)"
```

---

### Task 3: テンプレートで子と Step をまとめて作る(サーバー)

**Files:**
- Modify: `internal/domain/workspace/template_check.go`(`TemplateValuesFor` を足す)
- Test: `internal/domain/workspace/template_check_test.go`
- Create: `internal/app/items/createtemplate.go`
- Modify: `internal/app/items/create.go`
- Test: `internal/app/items/template_test.go`(新規)

**Interfaces:**
- Produces:
  - `func TemplateValuesFor(s Settings, kind string, vs map[change.ID]json.RawMessage) map[change.ID]json.RawMessage`
  - `CreateInput.Template *change.ID`(nil は「なし」)
  - `func Others(t workspace.Template) int`(app/items の非公開 `others` でよい。外から使わない)

- [ ] **Step 1: domain の失敗するテストを書く**

`template_check_test.go` に足す(このファイルの既存の設定の作り方 `raw` と、追加プロパティを持つ設定の補助関数を使う。なければ `DefaultSettings` にプロパティを足して作る)。

```go
func TestTemplateValuesFor(t *testing.T) {
	s := workspace.DefaultSettings(ids()) // 既存のテストと同じ作り方に合わせる
	hidden := workspace.Property{ID: "01HIDDEN00000000000000001", Key: "memo", Name: "メモ", Type: workspace.TypeText, Kinds: []string{"Task"}, Hidden: true}
	storyOnly := workspace.Property{ID: "01STORY000000000000000001", Key: "area", Name: "領域", Type: workspace.TypeText, Kinds: []string{"Story"}}
	s.Properties = append(s.Properties, hidden, storyOnly)
	vs := map[change.ID]json.RawMessage{
		"priority":   raw(s.Priorities[0].ID),
		"systems":    raw([]change.ID{"01NOSUCHVALUE000000000001"}), // 消えた値の参照
		hidden.ID:    raw("非表示"),
		storyOnly.ID: raw("Story だけ"),
	}
	got := workspace.TemplateValuesFor(s, "Task", vs)
	want := map[change.ID]json.RawMessage{"priority": raw(s.Priorities[0].ID)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TemplateValuesFor = %s, want %s", got, want)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/workspace/ -run TestTemplateValuesFor`
Expected: FAIL(`undefined: workspace.TemplateValuesFor`)

- [ ] **Step 3: domain を実装する**

`template_check.go` の `keepValues` の下に足す。

```go
// TemplateValuesFor は、テンプレートの値のうち、kind の種類で使い、表示中で、値の参照が正しいものだけ
// (A3b-9。03「表示中でなくなったプロパティや、その種類で使わないプロパティは適用しない」)。
func TemplateValuesFor(s Settings, kind string, vs map[change.ID]json.RawMessage) map[change.ID]json.RawMessage {
	out := map[change.ID]json.RawMessage{}
	for id, v := range keepValues(s, kind, vs) {
		if p, _ := s.Property(id); !p.Hidden && validValue(s, p, v) {
			out[id] = v
		}
	}
	return out
}
```

Run: `go test ./internal/domain/workspace/` → PASS。コミット:

```bash
git add internal/domain/workspace/template_check.go internal/domain/workspace/template_check_test.go
git commit -m "feat(domain): テンプレートの値から、種類で使い表示中で正しいものだけを選ぶ(A3b-9)"
```

- [ ] **Step 4: app の失敗するテストを書く**

`internal/app/items/template_test.go`:

```go
package items_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// saveTemplate は、サンプルのワークスペースにテンプレートを書く(B3 の行の形。app の別のパッケージは使えない)。
func saveTemplate(t *testing.T, svc items.Service, rec *recorder, tp workspace.Template) workspace.Template {
	t.Helper()
	tp.ID = svc.IDs.New()
	for i := range tp.Children {
		tp.Children[i].ID = svc.IDs.New()
		for j := range tp.Children[i].Steps {
			tp.Children[i].Steps[j].ID = svc.IDs.New()
		}
	}
	for j := range tp.Steps {
		tp.Steps[j].ID = svc.IDs.New()
	}
	tp = tp.Ranked()
	err := rec.Stores.Write(t.Context(), store, port.Operation{Actor: change.User("hanako"), Summary: "テンプレートを作成"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, workspace.TemplateDrafts(workspace.Template{}, tp)...)
	})
	if err != nil {
		t.Fatal(err)
	}
	return tp
}

// byNumber は、プロジェクトのアイテムを番号で引く。
func byNumber(t *testing.T, svc items.Service) map[int]item.Item {
	t.Helper()
	list, err := svc.List(t.Context(), store, apptest.SampleProject)
	if err != nil {
		t.Fatal(err)
	}
	out := map[int]item.Item{}
	for _, it := range list {
		out[it.Place.Number] = it
	}
	return out
}

func TestCreateEpicFromATemplate(t *testing.T) {
	svc, rec := setupRec(t)
	tp := saveTemplate(t, svc, rec, workspace.Template{
		Name: "リリース", Kind: "Epic", Title: "リリース",
		Children: []workspace.TemplateChild{
			{Kind: "Story", Title: " QA\n 確認 ", Values: map[change.ID]json.RawMessage{
				"assignee": raw(`"taro"`), "systems": raw(`["` + string(apptest.SampleSystemWeb) + `"]`),
			}, Steps: []workspace.TemplateStep{
				{Title: "手順書", Assignee: workspace.StepParent},
				{Title: "記録", Assignee: workspace.StepNone},
			}},
			{Kind: "Task", Title: "本番反映"},
		},
	})
	root, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Kind: "Epic", Title: "11 月のリリース", Template: &tp.ID,
		Values: map[change.ID]json.RawMessage{"assignee": raw(`"hanako"`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := last(rec); got != "Epic「11 月のリリース」ほか 4 件を作成" {
		t.Errorf("summary = %q", got)
	}
	// 番号は深さ優先(02「作成したあと」): Epic 3、Story 4、Step 5・6、Task 7。
	all := byNumber(t, svc)
	qa, s1, s2, deploy := all[4], all[5], all[6], all[7]
	if root.Place.Number != 3 || qa.Title != "QA 確認" || qa.Kind != item.Story || deploy.Kind != item.Task {
		t.Fatalf("numbers: root %d, 4 %+v, 7 %+v", root.Place.Number, qa, deploy)
	}
	if *qa.Place.Parent != root.ID || *deploy.Place.Parent != root.ID || *s1.Place.Parent != qa.ID || *s2.Place.Parent != qa.ID {
		t.Errorf("parents: %v %v %v %v", qa.Place.Parent, deploy.Place.Parent, s1.Place.Parent, s2.Place.Parent)
	}
	if !(qa.Rank < deploy.Rank) || !(s1.Rank < s2.Rank) {
		t.Errorf("ranks: %q %q / %q %q", qa.Rank, deploy.Rank, s1.Rank, s2.Rank)
	}
	// 子はテンプレートの値、Step は「親と同じ」なら子の担当者、「未割り当て」なら担当者なし(03「適用するときの規則」)。
	if qa.Assignee != "taro" || len(qa.Systems) != 1 || s1.Assignee != "taro" || s2.Assignee != "" || deploy.Assignee != "" {
		t.Errorf("assignees: qa %q %v, steps %q %q, task %q", qa.Assignee, qa.Systems, s1.Assignee, s2.Assignee, deploy.Assignee)
	}
	for _, it := range []item.Item{qa, s1, s2, deploy} {
		if it.Status != apptest.SampleStatusTodo || it.Priority != apptest.SamplePriorityMedium || len(it.Watchers) != 1 || it.Watchers[0] != "hanako" {
			t.Errorf("%s: status %s priority %s watchers %v", it.Title, it.Status, it.Priority, it.Watchers)
		}
	}
	// 次の番号は、作ったものの次。
	if n, err := svc.NextNumber(t.Context(), store, apptest.SampleWorkspace); err != nil || n != 8 {
		t.Errorf("NextNumber = %d, %v", n, err)
	}
}

func TestCreateTaskFromATemplateTakesTheRootAssigneeForSteps(t *testing.T) {
	svc, rec := setupRec(t)
	tp := saveTemplate(t, svc, rec, workspace.Template{
		Name: "調査", Kind: "Task", Title: "調査",
		Steps: []workspace.TemplateStep{{Title: "まとめる", Assignee: workspace.StepParent}},
	})
	root, err := svc.Create(t.Context(), items.CreateInput{
		Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: "Task", Title: "調査", Template: &tp.ID,
		Values: map[change.ID]json.RawMessage{"assignee": raw(`"taro"`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	step := byNumber(t, svc)[root.Place.Number+1]
	if step.Kind != item.Step || *step.Place.Parent != root.ID || step.Assignee != "taro" || last(rec) != "Task「調査」ほか 1 件を作成" {
		t.Errorf("step = %+v, summary %q", step, last(rec))
	}
}

func TestCreateWithAMissingOrMismatchedTemplate(t *testing.T) {
	svc, rec := setupRec(t)
	tp := saveTemplate(t, svc, rec, workspace.Template{Name: "調査", Kind: "Task", Title: "調査"})
	missing := change.ID("01NOSUCHTEMPLATE000000001")
	for name, in := range map[string]items.CreateInput{
		"missing":  {Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: "Task", Title: "a", Template: &missing},
		"mismatch": {Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: "Bug", Title: "a", Template: &tp.ID},
	} {
		if _, err := svc.Create(t.Context(), in); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// タイトルの断りが先(02「入力の検査」の表の順。A3b-7)。
	_, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: "Task", Title: " ", Template: &missing})
	if message(err) != "タイトルを入力してください" {
		t.Errorf("empty title = %v", err)
	}
}
```

(`encoding/json` を import に足す。`raw` は `values_test.go` の `raw(s string) json.RawMessage`。`NextNumber` の名前と引数は `numbers.go` に合わせる。)

- [ ] **Step 5: 失敗を確かめる**

Run: `go test ./internal/app/items/ -run Template`
Expected: FAIL(`unknown field Template in struct literal`)

- [ ] **Step 6: app を実装する**

`create.go` の `CreateInput` に足す。

```go
// CreateInput は、作成ダイアログの入力。Values はプロパティの値(キーはプロパティの ID。組み込みはキー)。
// Template は、一緒に作る構成を持つテンプレート(nil は「なし」。A3b-6)。
type CreateInput struct {
	Store    port.StoreID
	Project  project.ID
	Parent   *item.ID
	Kind     item.Kind
	Title    string
	Values   map[change.ID]json.RawMessage
	Template *change.ID
}
```

`Create` と `create` を次にする。

```go
// Create は、アイテムを作る(02「作成したあと」)。テンプレートが子や Step を持つときは、それらも同じ 1 回の保存で作る。
func (s Service) Create(ctx context.Context, in CreateInput) (item.Item, error) {
	t, err := s.templateOf(ctx, in)
	if err != nil {
		return item.Item{}, err
	}
	summary := fmt.Sprintf("%s「%s」を作成", in.Kind, item.NormalizeTitle(in.Title))
	if t != nil && others(*t) > 0 {
		summary = fmt.Sprintf("%s「%s」ほか %d 件を作成", in.Kind, item.NormalizeTitle(in.Title), others(*t))
	}
	return s.create(ctx, in, t, summary)
}

// create は、作成と Step の追加が共用する本体。summary はアクティビティの 1 行。t は一緒に作るテンプレート(nil はなし)。
func (s Service) create(ctx context.Context, in CreateInput, t *workspace.Template, summary string) (item.Item, error) {
	me, err := s.Identity.Me(ctx, in.Store)
	if err != nil {
		return item.Item{}, err
	}
	title := item.NormalizeTitle(in.Title)
	op := port.Operation{Actor: change.User(me), Summary: summary}
	var id item.ID
	err = s.Stores.Write(ctx, in.Store, op, func(tx port.Tx) error {
		created, err := s.build(tx, in, title, me)
		if err != nil {
			return err
		}
		all, err := s.withTemplate(tx, in, t, created, me)
		if err != nil {
			return err
		}
		id = created.ID
		return writeCreated(tx, all)
	})
	if err != nil {
		return item.Item{}, err
	}
	return s.Get(ctx, in.Store, id)
}

// writeCreated は、作るアイテムを番号の順に書く。番号の行も 1 つずつ書く(A3b-11)。
func writeCreated(tx port.Tx, all []item.Item) error {
	var errs []error
	for _, it := range all {
		errs = append(errs,
			tx.WriteWorkspaces().Append(it.Place.Workspace, numbering.Draft(numbering.Items, it.Place.Number)),
			tx.WriteItems().Append(it.ID, item.Create(it)),
		)
	}
	return errors.Join(errs...)
}
```

`addstep.go` の呼び出しを `s.create(ctx, in, nil, fmt.Sprintf(...))` に替える。

`createtemplate.go`(新規):

```go
package items

import (
	"context"
	"encoding/json"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// templateOf は、作成で使うテンプレートを、保存の前に読む(A3b-6)。アクティビティの件数と作る構成は、
// この同じ中身から決める。テンプレートを選んでいない、またはプロジェクトがなければ nil(プロジェクトの
// 断りは保存の中で出す)。ないか種類が違うときも nil を返し、保存の中で ErrNotFound にする(A3b-7)。
func (s Service) templateOf(ctx context.Context, in CreateInput) (*workspace.Template, error) {
	if in.Template == nil || in.Project == "" {
		return nil, nil //nolint:nilnil // テンプレートなしは正しい結果
	}
	var found *workspace.Template
	err := s.Stores.Read(ctx, in.Store, func(r port.Reader) error {
		p, err := r.Projects().Get(in.Project)
		if err != nil {
			return err
		}
		st, err := r.Workspaces().Settings(p.Place.Workspace)
		if err != nil {
			return err
		}
		if t, ok := st.Template(*in.Template); ok && t.Kind == string(in.Kind) {
			found = &t
		}
		return nil
	})
	return found, err
}

// others は、テンプレートと一緒に作る子と Step の数。
func others(t workspace.Template) int {
	n := len(t.Children) + len(t.Steps)
	for _, c := range t.Children {
		n += len(c.Steps)
	}
	return n
}

// withTemplate は、もとになるアイテム root と、テンプレートの構成から作る子と Step を、番号の順に返す。
// テンプレートを選んだのに読めなかったときは、ほかの検査のあとで ErrNotFound にする(A3b-7)。
func (s Service) withTemplate(tx port.Tx, in CreateInput, t *workspace.Template, root item.Item, me string) ([]item.Item, error) {
	if in.Template == nil {
		return []item.Item{root}, nil
	}
	if t == nil {
		return nil, port.ErrNotFound
	}
	st, err := tx.Workspaces().Settings(root.Place.Workspace)
	if err != nil {
		return nil, err
	}
	e := expander{st: st, me: me, newID: s.IDs.New, next: root.Place.Number, out: []item.Item{root}}
	e.steps(root, t.Steps)
	r := ""
	for _, c := range t.Children {
		r = rank.After(r)
		child := e.add(item.Kind(c.Kind), root, c.Title, r, workspace.TemplateValuesFor(st, c.Kind, c.Values))
		e.steps(child, c.Steps)
	}
	return e.out, nil
}

// expander は、テンプレートの構成からアイテムを深さ優先の番号で作る(A3b-10・A3b-11)。
type expander struct {
	st    workspace.Settings
	me    string
	newID func() change.ID
	next  int
	out   []item.Item
}

func (e *expander) add(kind item.Kind, parent item.Item, title, r string, vals map[change.ID]json.RawMessage) item.Item {
	e.next++
	pid := parent.ID
	it := item.Item{
		ID:       e.newID(),
		Place:    item.Place{Workspace: parent.Place.Workspace, Project: parent.Place.Project, Parent: &pid, Number: e.next},
		Kind:     kind,
		Rank:     r,
		Title:    item.NormalizeTitle(title),
		Watchers: []string{e.me},
	}
	applyValues(&it, vals, e.st)
	e.out = append(e.out, it)
	return it
}

// steps は、parent の下に Step を並び順に作る。担当者は「親と同じ」なら parent の担当者、「未割り当て」ならなし。
func (e *expander) steps(parent item.Item, ss []workspace.TemplateStep) {
	r := ""
	for _, s := range ss {
		r = rank.After(r)
		who := s.Assignee
		switch who {
		case workspace.StepParent:
			who = parent.Assignee
		case workspace.StepNone:
			who = ""
		}
		var vals map[change.ID]json.RawMessage
		if who != "" {
			vals = map[change.ID]json.RawMessage{"assignee": change.Value(who)}
		}
		e.add(item.Step, parent, s.Title, r, vals)
	}
}
```

- [ ] **Step 7: 通ることを確かめる**

Run: `go test ./internal/app/items/ ./internal/domain/... && make lint`
Expected: PASS(関数 60 行・認知的複雑度 15 を超えたら、超えた関数を分ける)

- [ ] **Step 8: コミット**

```bash
git add internal/app/items/
git commit -m "feat(app): テンプレートの子と Step を、深さ優先の番号で同じ 1 回の保存で作る(A3b-6〜A3b-11)"
```

---

### Task 4: 作成の API と画面の口にテンプレートを足す

**Files:**
- Modify: `internal/adapter/httpapi/items/items.go`(`CreateItem`、作成のハンドラー)
- Test: `internal/adapter/httpapi/items/items_test.go`(作成のテストがあるファイル。名前は既存に合わせる)
- Modify: `web/packages/core/src/model/item.ts`(`NewItem.templateId`)
- Modify: `web/packages/api/src/items.ts`(作成で `template` を渡す)
- Test: `web/packages/api/src/items.test.ts`(作成の対応のテストがあれば足す。なければ作る)
- 生成物: `make generate`・`pnpm -C web gen`

**Interfaces:**
- Consumes: `CreateInput.Template`(Task 3)
- Produces: API 本文 `CreateItem.template`(省略可、空は「なし」)、`NewItem.templateId?: string`

- [ ] **Step 1: 失敗するテストを書く**

httpapi の作成のテストに足す(既存の作成のテストの準備とリクエストの送り方を使う)。テンプレートは、既存の httpapi のテストの準備でワークスペースの設定に書けないなら、存在しない ID を送って 404 になること(ID が app に届くこと)を確かめる。

```go
func TestCreateItemPassesTheTemplate(t *testing.T) {
	api := setupAPI(t) // 既存の準備に合わせる
	resp := api.Post("/api/stores/s1/items", map[string]any{
		"project": string(apptest.SampleProject), "parent": string(apptest.SampleEpic),
		"kind": "Task", "title": "調査", "template": "01NOSUCHTEMPLATE000000001",
	})
	if resp.Code != http.StatusNotFound {
		t.Errorf("status = %d, body %s", resp.Code, resp.Body)
	}
}
```

api の作成のテスト(Vitest。`createItem` を差し替える既存の書き方に合わせる):

```ts
it("sends the template id when creating", async () => {
  const sent: unknown[] = [];
  const port = itemsPort({ createItem: async (_store, body) => { sent.push(body); return okItem; } }); // 既存の作り方に合わせる
  await port.create("s1", { projectId: "P", parentId: null, kind: "Epic", title: "a", values: {}, templateId: "T1" });
  expect(sent[0]).toMatchObject({ template: "T1" });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/adapter/httpapi/items/` と `pnpm -C web --filter @hedgehug/api test`
Expected: FAIL(Go は 201 で作れてしまう。TS は `template` がない)

- [ ] **Step 3: 実装する**

```go
type CreateItem struct {
	Project string `json:"project" doc:"空はプロジェクトを選んでいない"`
	Parent  string `json:"parent,omitempty" doc:"空は親を選んでいない"`
	Kind    string `json:"kind" enum:"Epic,Story,Task,Bug,Step"`
	Title   string `json:"title"`

	Values   map[string]json.RawMessage `json:"values,omitempty" doc:"プロパティの値(キーはプロパティの内部 ID。組み込みはキー)"`
	Template string                     `json:"template,omitempty" doc:"一緒に作るテンプレートの内部 ID。空は「なし」"`
}
```

作成のハンドラーで `CreateInput` を作るところに足す。

```go
	if in.Body.Template != "" {
		tid := change.ID(in.Body.Template)
		ci.Template = &tid
	}
```

`make generate` と `pnpm -C web gen` で OpenAPI と orval を作り直す。`model/item.ts`:

```ts
export interface NewItem {
  projectId: string;
  parentId: string | null;
  kind: Kind;
  title: string;
  // values は、作成のときに書くプロパティの値(プロパティの ID ごと)。
  values: Record<string, PropertyInput>;
  // templateId は、一緒に作るテンプレート(空かなしは「なし」)。
  templateId?: string;
}
```

`api/src/items.ts` の作成で、本文に `...(input.templateId ? { template: input.templateId } : {})` を足す。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/adapter/... && pnpm -C web check && pnpm -C web test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add -A internal web/packages   # 生成した OpenAPI が別の場所にあれば、それも足す
git commit -m "feat(api): 作成の本文にテンプレートの内部 ID を足す(A3b-6)"
```

---

### Task 5: ステータスのメニューで移れない項目を無効にし、理由を出す

**Files:**
- Modify: `web/packages/core/src/features/workspaces/workflow.ts`(`transitionRefusal`)
- Test: `web/packages/core/src/features/workspaces/workflow.test.ts`
- Modify: `web/packages/core/src/features/items/properties.ts`(`ValueOption`・`statusOptions`)
- Test: `web/packages/core/src/features/items/properties.test.ts`
- Modify: `web/packages/ui/src/types.ts`・`web/packages/ui/src/patterns/ValueMenu.svelte`
- Test: `web/packages/ui/src/patterns/ValueMenu.test.ts`
- Modify: `web/app/src/screens/item/StatusButton.svelte`

**Interfaces:**
- Consumes: B3 の `transitionRefusal(settings, from, to): string | null`
- Produces: `statusOptions(settings: WorkspaceSettings, current: string, workflow = false): ValueOption[]`、`ValueOption.disabled?: boolean`、`ValueOption.reason?: string`(core と ui の両方)

- [ ] **Step 1: 失敗するテストを書く**

`workflow.test.ts`:

```ts
it("does not check a current status that is no longer in the list", () => {
  const s = { ...settings, workflow: { gone: ["st2"] } }; // このファイルの既存の設定に合わせる
  expect(transitionRefusal(s, "gone", "st3")).toBeNull();
});
```

`properties.test.ts`(設定は `settings.test-util.ts` の `settings`。Todo=st1・InProgress=st2・Done=st3・Cancel=st4):

```ts
describe("statusOptions with the workflow", () => {
  const restricted = { ...settings, workflow: { st1: ["st2"] } };
  it("disables statuses the workflow forbids and gives the reason", () => {
    const done = statusOptions(restricted, "st1", true).find((o) => o.id === "st3");
    expect(done).toMatchObject({
      disabled: true,
      hint: "完了",
      reason: "ワークフローの制約: Todo から Done には変更できません(InProgress のみ)",
    });
    expect(statusOptions(restricted, "st1", true).find((o) => o.id === "st2")?.disabled).toBeFalsy();
    expect(statusOptions(restricted, "st1", true).find((o) => o.id === "st1")?.disabled).toBeFalsy();
  });
  it("leaves every status selectable without the workflow (create dialog)", () => {
    expect(statusOptions(restricted, "st1").some((o) => o.disabled)).toBe(false);
    const row = propertyRows(restricted, "Task", (p) => (p.id === "status" ? "st1" : "")).find(
      (r) => r.property.id === "status",
    );
    expect(row?.options.some((o) => o.disabled)).toBe(false);
  });
});
```

(`hint` の分類名は `categoryLabels` の実際の値に合わせる。`propertyRows` の行の形も既存のテストに合わせる。)

`ValueMenu.test.ts`(既存のテストの描画の仕方に合わせる):

```ts
it("disables an option and shows its reason", async () => {
  const onselect = vi.fn();
  render(ValueMenu, {
    open: true, x: 0, y: 0, label: "ステータス", onselect, onclose: () => {},
    options: [
      { id: "a", label: "Todo", checked: true },
      { id: "b", label: "Done", checked: false, disabled: true, reason: "ワークフローの制約: Todo から Done には変更できません(InProgress のみ)" },
    ],
  });
  const done = screen.getByRole("menuitemcheckbox", { name: /Done/ });
  expect(done).toBeDisabled();
  expect(screen.getByText("ワークフローの制約: Todo から Done には変更できません(InProgress のみ)")).toBeInTheDocument();
  await fireEvent.click(done);
  expect(onselect).not.toHaveBeenCalled();
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web test -- workflow properties ValueMenu`
Expected: FAIL

- [ ] **Step 3: 実装する**

`workflow.ts` の `transitionRefusal` の先頭:

```ts
  // いまのステータスが一覧にない(削除された値のまま)ときは検査しない(02「ステータスの変更」、A3b-2)。
  if (from === "" || from === to || !settings.statuses.some((v) => v.id === from)) return null;
```

`properties.ts`:

```ts
export interface ValueOption {
  id: string;
  label: string;
  hint?: string;
  checked: boolean;
  // disabled は選べない項目、reason はその理由(遷移の制約。A3b-5)。
  disabled?: boolean;
  reason?: string;
}
```

```ts
// statusOptions は、ステータスのメニューの項目(決定 21)。workflow なら、遷移の制約で移れない項目を無効にし、
// 理由を添える(02「ステータスの変更」、A3b-4)。作成ダイアログは制約を適用しないので false。
// 使わないステータスの扱いは B5 の後(決定 3)。
export function statusOptions(
  settings: WorkspaceSettings,
  current: string,
  workflow = false,
): ValueOption[] {
  return settings.statuses.map((s) => {
    const reason = workflow ? transitionRefusal(settings, current, s.id) : null;
    return {
      id: s.id,
      label: s.name,
      hint: s.category ? categoryLabels[s.category] : undefined,
      checked: s.id === current,
      ...(reason ? { disabled: true, reason } : {}),
    };
  });
}
```

(`transitionRefusal` は `../workspaces/workflow` から import する。)

`ui/src/types.ts` の `ValueOption` に同じ 2 つを足す。`ValueMenu.svelte` のボタン:

```svelte
      <button
        type="button"
        role="menuitemcheckbox"
        aria-checked={o.checked}
        class="menu-item"
        disabled={o.disabled}
        title={o.reason}
        onclick={() => {
          if (!o.disabled) choose(o.id);
        }}
      >
        <span class="check" aria-hidden="true">{o.checked ? "✓" : ""}</span>
        <span class="menu-label">{o.label}</span>
        {#if o.hint}
          <span class="menu-hint">{o.hint}</span>
        {/if}
        {#if o.reason}
          <span class="menu-reason">{o.reason}</span>
        {/if}
      </button>
```

スタイルは `Menu.svelte` の `.menu-item:disabled` と同じ見た目にし、`.menu-reason` は `flex-basis: 100%; font-size: 11px; color: var(--muted); white-space: normal;`(`.menu-item` が `flex-wrap: wrap` でなければ足す)。

`StatusButton.svelte` の `options={statusOptions(settings, status)}` を `options={statusOptions(settings, status, true)}` にし、先頭のコメントに「遷移の制約で移れない項目は無効にする(A3b-4)」を足す。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web check && pnpm -C web test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages web/app
git commit -m "feat(app): ステータスのメニューで遷移の制約に反する項目を無効にし、理由を出す(A3b-4・A3b-5)"
```

---

### Task 6: 作成ダイアログのテンプレートの当てはめ(core の純粋な関数)

**Files:**
- Create: `web/packages/core/src/features/items/createTemplate.ts`
- Test: `web/packages/core/src/features/items/createTemplate.test.ts`
- Modify: `web/packages/core/src/index.ts`(画面が使うものを export)

**Interfaces:**
- Consumes: B3 の `usableTemplates`・`defaultTemplate`・`applyTemplate`・`templateStructure`(`../workspaces/templateApply`)、`visibleProperties`(`./properties`)、`Values`(`./createValues`)
- Produces:
  - `interface TemplateForm { kind: Kind; title: string; values: Values; templateId: string }`
  - `templateOptions(settings, kind): Option[]`(使えるものがなければ `[]`、あれば先頭に `{ value: "", label: "なし" }`)
  - `applyToForm<F extends TemplateForm>(form: F, settings, templateId: string): F`
  - `applyDefault<F extends TemplateForm>(form: F, settings): F`
  - `structureOf(settings, form: TemplateForm, members: Member[]): { heading: string; rows: StructureRow[] } | null`
  - `othersOf(settings, templateId: string): number`

- [ ] **Step 1: 失敗するテストを書く**

```ts
import { describe, expect, it } from "vitest";
import type { WorkspaceSettings, WorkspaceTemplate } from "../../model/workspace";
import {
  applyDefault,
  applyToForm,
  othersOf,
  structureOf,
  templateOptions,
  type TemplateForm,
} from "./createTemplate";
import { settings } from "./settings.test-util";

const tpl = (t: Partial<WorkspaceTemplate> & { id: string; kind: string }): WorkspaceTemplate => ({
  name: t.id,
  title: "",
  description: "",
  values: {},
  children: [],
  steps: [],
  ...t,
});

// T1: Task(書き出し・優先度・F2・ポイント)。T2: Task(担当者・Step 2 件)。E1: Epic(子 2 件・Step 1 件)。
const t1 = tpl({ id: "T1", kind: "Task", name: "調査", title: "調査: ", values: { priority: "pr1", F2: "メモ", points: 3 } });
const t2 = tpl({
  id: "T2",
  kind: "Task",
  name: "作業",
  values: { assignee: "hanako" },
  steps: [
    { id: "s1", title: "準備", assignee: "@parent" },
    { id: "s2", title: "記録", assignee: "@none" },
  ],
});
const e1 = tpl({
  id: "E1",
  kind: "Epic",
  name: "リリース",
  children: [
    { id: "c1", kind: "Story", title: "QA", description: "", values: { assignee: "taro" }, steps: [{ id: "s3", title: "確認", assignee: "@parent" }] },
    { id: "c2", kind: "Task", title: "本番反映", description: "", values: {}, steps: [] },
  ],
});
const withTemplates: WorkspaceSettings = {
  ...settings,
  templates: [t1, t2, e1],
  templateDefaults: { Task: "T1" },
};
const form = (f: Partial<TemplateForm> = {}): TemplateForm => ({
  kind: "Task",
  title: "",
  values: { status: "st1", priority: "pr2" },
  templateId: "",
  ...f,
});

describe("templateOptions", () => {
  it("lists the kind's templates after なし, and nothing when there are none", () => {
    expect(templateOptions(withTemplates, "Task")).toEqual([
      { value: "", label: "なし" },
      { value: "T1", label: "調査" },
      { value: "T2", label: "作業" },
    ]);
    expect(templateOptions(withTemplates, "Bug")).toEqual([]);
  });
});

describe("applyToForm", () => {
  it("fills an empty title and overwrites only the template's shown values", () => {
    const out = applyToForm(form({ values: { status: "st2", priority: "pr2", F2: "前" } }), withTemplates, "T1");
    // ポイントは Story だけのプロパティなので Task には当てはめない(03「その種類で使わないプロパティは適用しない」)。
    expect(out).toEqual(form({ title: "調査: ", templateId: "T1", values: { status: "st2", priority: "pr1", F2: "メモ" } }));
  });
  it("keeps a typed title", () => {
    expect(applyToForm(form({ title: "ログイン" }), withTemplates, "T1").title).toBe("ログイン");
  });
  it("turns template numbers into input text (A3a 決定 12)", () => {
    const out = applyToForm(form({ kind: "Story" }), { ...withTemplates, templates: [{ ...t1, kind: "Story" }] }, "T1");
    expect(out.values.points).toBe("3");
  });
  it("switching to なし keeps the input", () => {
    const applied = applyToForm(form(), withTemplates, "T1");
    expect(applyToForm(applied, withTemplates, "")).toEqual({ ...applied, templateId: "" });
  });
});

describe("applyDefault", () => {
  it("drops the current template and applies the kind's default", () => {
    expect(applyDefault(form({ templateId: "T2" }), withTemplates).templateId).toBe("T1");
    expect(applyDefault(form({ kind: "Bug", templateId: "T2" }), withTemplates).templateId).toBe("");
  });
});

describe("structureOf and othersOf", () => {
  it("shows the children and steps with the assignee each would get", () => {
    expect(structureOf(withTemplates, form({ templateId: "T2", values: { assignee: "taro" } }), [{ login: "taro", name: "太郎" }])).toEqual({
      heading: "一緒に作るもの: Step 2 件",
      rows: [
        { depth: 0, kind: "Step", title: "準備", assignee: "太郎" },
        { depth: 0, kind: "Step", title: "記録", assignee: "未割り当て" },
      ],
    });
    expect(structureOf(withTemplates, form({ templateId: "T1" }), [])).toBeNull();
    expect(othersOf(withTemplates, "E1")).toBe(3);
    expect(othersOf(withTemplates, "")).toBe(0);
  });
});
```

(`heading` の文言は B3 の `templateSummary` が返すもの。合わなければ `templateSummary` の実際の出力に合わせる。)

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- createTemplate`
Expected: FAIL(モジュールがない)

- [ ] **Step 3: 実装する**

```ts
import type { Kind, PropertyInput } from "../../model/item";
import type { Member } from "../../model/member";
import type { WorkspaceSettings } from "../../model/workspace";
import type { Option } from "../projects/createDialog";
import {
  applyTemplate,
  defaultTemplate,
  type StructureRow,
  templateStructure,
  usableTemplates,
} from "../workspaces/templateApply";
import type { Values } from "./createValues";
import { visibleProperties } from "./properties";

// 作成ダイアログのテンプレート(02「ダイアログの中での連動」、03「適用するときの規則」)。説明の当てはめは
// A4(A3a の決定 6)。使えるテンプレートはワークスペースの設定だけで決める(プロジェクトの絞り込みは B5)。

export interface TemplateForm {
  kind: Kind;
  title: string;
  values: Values;
  // templateId は、当てはめているテンプレート。空は「なし」。
  templateId: string;
}

const find = (settings: WorkspaceSettings, id: string) =>
  settings.templates.find((t) => t.id === id) ?? null;

// templateOptions は、テンプレートのセレクトの選択肢。使えるものがなければ空(欄を出さない)。
export function templateOptions(settings: WorkspaceSettings, kind: Kind): Option[] {
  const list = usableTemplates(settings, kind);
  if (list.length === 0) return [];
  return [{ value: "", label: "なし" }, ...list.map((t) => ({ value: t.id, label: t.name }))];
}

// toInput は、テンプレートの値を入力欄の値にする。数は文字列にする(A3a の決定 12、A3b-13)。
const toInput = (v: unknown): PropertyInput => (typeof v === "number" ? String(v) : (v as PropertyInput));

// applyToForm は、templateId のテンプレートを当てはめた入力。空なら「なし」にする(入力は残す)。
export function applyToForm<F extends TemplateForm>(
  form: F,
  settings: WorkspaceSettings,
  templateId: string,
): F {
  const shown = new Set(visibleProperties(settings, form.kind).map((p) => p.id));
  const out = applyTemplate(
    { title: form.title, description: "", values: {}, template: find(settings, form.templateId) },
    find(settings, templateId),
    "select",
    (id) => shown.has(id),
  );
  const values = { ...form.values };
  for (const [id, v] of Object.entries(out.values)) values[id] = toInput(v);
  return { ...form, title: out.title, values, templateId: out.template?.id ?? "" };
}

// applyDefault は、いまのテンプレートを「なし」にしてから、その種類の既定を当てはめる(A3b-14)。
export function applyDefault<F extends TemplateForm>(form: F, settings: WorkspaceSettings): F {
  const none = { ...form, templateId: "" };
  return applyToForm(none, settings, defaultTemplate(settings, form.kind)?.id ?? "");
}

// structureOf は、「一緒に作るもの」。「親と同じ」の担当者には、いまの入力の担当者を出す(A3b-15)。
export function structureOf(
  settings: WorkspaceSettings,
  form: TemplateForm,
  members: Member[],
): { heading: string; rows: StructureRow[] } | null {
  const t = find(settings, form.templateId);
  if (!t) return null;
  const assignee = typeof form.values.assignee === "string" ? form.values.assignee : "";
  return templateStructure(t, assignee, members);
}

// othersOf は、一緒に作る子と Step の数(トーストの「ほか N 件」。A3b-16)。
export function othersOf(settings: WorkspaceSettings, templateId: string): number {
  const t = find(settings, templateId);
  if (!t) return 0;
  return t.children.reduce((n, c) => n + 1 + c.steps.length, t.steps.length);
}
```

`visibleProperties(settings, kind)` が `Kind` でなく `string` を取るなら、そのまま渡す。`index.ts` に `templateOptions`・`structureOf` と型 `StructureRow` を足す(`StructureRow` は B3 で export 済みならそのまま)。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web check && pnpm -C web --filter @hedgehug/core test`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(core): 作成ダイアログにテンプレートを当てはめる関数(A3b-12〜A3b-16)"
```

---

### Task 7: 作成ダイアログにテンプレートを結び付ける

**Files:**
- Modify: `web/packages/core/src/features/items/createDialog.ts`
- Create(行数を超えたときだけ): `web/packages/core/src/features/items/createDialogOptions.ts`
- Test: `web/packages/core/src/features/items/createDialogTemplate.test.ts`(新規)
- Modify: `web/packages/core/src/features/items/fixtures.test-util.ts`(偽物の `create` が `templateId` を記録する)
- Modify: 既存の作成ダイアログのテスト(`form` に `templateId: ""` が増えるので、`toEqual` で form 全体を比べているものを直す)

**Interfaces:**
- Consumes: Task 6 の関数、Task 4 の `NewItem.templateId`
- Produces: `ItemForm.templateId: string`、`ItemDialogState.templates: Option[]`、`ItemDialogState.structure: { heading: string; rows: StructureRow[] } | null`、`dialog.update({ templateId })` で当てはめる

- [ ] **Step 1: 失敗するテストを書く**

偽物の `fakeItemsPort` に、`create` に渡された `templateId` を `createdTemplates: (string | undefined)[]` として記録する欄を足す(`createdValues` と同じ書き方)。テスト用の設定は `fakeWorkspaces({ "S1-W1": withTemplates, "S2-W1": { ...settings2, templates: [...], templateDefaults: {...} } })` のように渡す。

```ts
import { describe, expect, it } from "vitest";
import type { WorkspaceSettings } from "../../model/workspace";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createItemDialog } from "./createDialog";
import { fakeItemsPort, item } from "./fixtures.test-util";
import { fakeWorkspaces, settings, settings2 } from "./settings.test-util";

const steps = [{ id: "s1", title: "準備", assignee: "@parent" }];
const base = { description: "", children: [], steps: [] as typeof steps };
const ws1: WorkspaceSettings = {
  ...settings,
  templates: [
    { ...base, id: "T1", kind: "Task", name: "調査", title: "調査: ", values: { priority: "pr1" } },
    { ...base, id: "T2", kind: "Task", name: "作業", title: "", values: {}, steps },
    { ...base, id: "B1", kind: "Bug", name: "不具合", title: "不具合: ", values: {} },
  ],
  templateDefaults: { Task: "T1", Bug: "B1" },
};
const ws2: WorkspaceSettings = {
  ...settings2,
  templates: [{ ...base, id: "X1", kind: "Task", name: "別", title: "別: ", values: { assignee: "jiro" } }],
  templateDefaults: { Task: "X1" },
};

async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const items = fakeItemsPort([item("S1", "S1-P1", 1, "Epic", null, "認証")]);
  const shown: string[] = [];
  const dialog = createItemDialog({
    catalog,
    items,
    toasts: createToasts({ set: (t) => (shown.push(t.message), 0), clear: () => {} }), // 既存のテストのトーストの拾い方に合わせる
    here: () => ({ workspaceId: "S1-W1", projectId: "S1-P1" }),
    openProjectDialog: () => {},
    workspaces: fakeWorkspaces({ "S1-W1": ws1, "S2-W1": ws2 }),
    catalogPort: {
      ...fakeCatalogPort(),
      listMembers: async () => [{ login: "taro", name: "太郎" }, { login: "jiro", name: "次郎" }],
    },
    onCreated: () => {},
  });
  await dialog.open();
  return { dialog, items, shown };
}

describe("create dialog templates", () => {
  it("applies the kind's default when it opens", async () => {
    const { dialog } = await setup();
    const s = get(dialog.state);
    expect(s?.form).toMatchObject({ kind: "Task", templateId: "T1", title: "調査: " });
    expect(s?.form.values.priority).toBe("pr1");
    expect(s?.templates.map((o) => o.label)).toEqual(["なし", "調査", "作業"]);
  });

  it("switching the kind drops the template and applies the new default; the title stays", async () => {
    const { dialog } = await setup();
    await dialog.update({ kind: "Bug" });
    expect(get(dialog.state)?.form).toMatchObject({ templateId: "B1", title: "調査: " });
    await dialog.update({ kind: "Story" });
    expect(get(dialog.state)?.form.templateId).toBe("");
    expect(get(dialog.state)?.templates).toEqual([]);
  });

  it("selecting a template overwrites its values and shows what is made with it", async () => {
    const { dialog } = await setup();
    await dialog.update({ values: { ...get(dialog.state)!.form.values, assignee: "taro" } });
    await dialog.update({ templateId: "T2" });
    const s = get(dialog.state);
    expect(s?.form.templateId).toBe("T2");
    expect(s?.structure).toEqual({
      heading: "一緒に作るもの: Step 1 件",
      rows: [{ depth: 0, kind: "Step", title: "準備", assignee: "太郎" }],
    });
  });

  it("changing the workspace applies the new workspace's default after resetting values", async () => {
    const { dialog } = await setup();
    await dialog.update({ workspaceId: "S2-W1" });
    const s = get(dialog.state);
    expect(s?.form.templateId).toBe("X1");
    expect(s?.form.values.assignee).toBe("jiro");
    expect(s?.form.values.priority).toBe("np1");
  });

  it("sends the template, toasts the others and keeps it when continuing", async () => {
    const { dialog, items, shown } = await setup();
    await dialog.update({ templateId: "T2", title: "作業する", keepOpen: true });
    await dialog.submit();
    expect(items.createdTemplates.at(-1)).toBe("T2");
    expect(shown.at(-1)).toMatch(/^HH-\d+ ほか 1 件を作成しました$/);
    // 続けて作成: タイトルだけ空にし、テンプレートと「一緒に作るもの」は残す。書き出しは入れ直さない。
    const s = get(dialog.state);
    expect(s?.form).toMatchObject({ templateId: "T2", title: "" });
    expect(s?.structure?.rows).toHaveLength(1);
  });

  it("sends no template when none is chosen", async () => {
    const { dialog, items, shown } = await setup();
    await dialog.update({ templateId: "", title: "なしで作る" });
    await dialog.submit();
    expect(items.createdTemplates.at(-1)).toBeUndefined();
    expect(shown.at(-1)).toMatch(/^HH-\d+ を作成しました$/);
  });

  it("keeps the template when the project changes (B5 narrows it later)", async () => {
    const { dialog } = await setup();
    await dialog.update({ templateId: "T2" });
    await dialog.update({ projectId: "S1-P2" }); // カタログの偽物にある 2 つ目のプロジェクトに合わせる
    expect(get(dialog.state)?.form.templateId).toBe("T2");
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter @hedgehug/core test -- createDialogTemplate`
Expected: FAIL

- [ ] **Step 3: 実装する**

`createDialog.ts` を次のように変える(差分の要点。既存の流れはそのまま)。

1. import に `applyDefault, applyToForm, othersOf, structureOf, templateOptions` と `StructureRow` を足す。先頭のコメントの「テンプレート・スプリント・説明は A3 後半。」を「テンプレートは A3 後半(A3b)。スプリントは F、説明は A4。」に替える。
2. `ItemForm` に `templateId: string;`(コメント「当てはめているテンプレート。空は「なし」」)。`ItemDialogState` に `templates: Option[];` と `structure: { heading: string; rows: StructureRow[] } | null;`。
3. 行・テンプレートの欄・一緒に作るものを一度に作る関数を足し、`rowsFor(ctx, form)` を呼んでいる所(`refresh`・`reloaded`・種類の切り替え・値の変更)をこれに替える。

```ts
  // view は、form から作り直す欄(プロパティの行・テンプレートの欄・一緒に作るもの)。
  const view = (ctx: WorkspaceContext, form: ItemForm) => ({
    rows: rowsFor(ctx, form),
    templates: templateOptions(ctx.settings, form.kind),
    structure: structureOf(ctx.settings, form, ctx.people.members),
  });
```

4. `open`: `form` を `let` にし、`form.values = initialValues(loadedCtx.settings);` の次に `form = applyDefault(form, loadedCtx.settings);`。`form` の初期値に `templateId: ""`。
5. `reloaded`: ワークスペースが変わったとき(`now.form.workspaceId !== form.workspaceId`)だけ、`switchWorkspace` のあとで既定を当てはめる。

```ts
    const switched = now.form.workspaceId !== form.workspaceId;
    const values = switched
      ? switchWorkspace(now.form.values, loaded.ctx.settings, memberCheck(now.form.workspaceId, form.workspaceId, loaded.ctx))
      : now.form.values;
    let kept: ItemForm = { ...loaded.next.form, title, keepOpen, values, templateId: now.form.templateId };
    // ワークスペースを変えたら、テンプレートを「なし」にしてから新しいワークスペースの既定を当てはめる(A3b-14)。
    if (switched) kept = applyDefault(kept, loaded.ctx.settings);
    current = loaded.ctx;
    state.set({ ...loaded.next, form: kept, ...view(loaded.ctx, kept) });
```

6. `update` の種類の切り替え: 親を選び直したあと、`current` があれば `form = applyDefault(form, current.settings)`(`form` は `let` にする)。`state.set` の `rows` を `...view(current, form)` に替える。
7. `update` に、テンプレートを選んだときの枝を足す(種類の枝の次)。

```ts
      if (patch.templateId !== undefined && patch.templateId !== s.form.templateId && current) {
        // テンプレートを変えたら当てはめる(02「テンプレートを変えたとき」)。
        const applied = applyToForm({ ...s.form, ...patch, templateId: s.form.templateId }, current.settings, patch.templateId);
        state.set({ ...s, form: applied, ...view(current, applied) });
        return;
      }
```

8. 最後の値の変更の枝: `const rows = ...` を `const parts = current ? view(current, form) : {}; state.set({ ...s, form, ...parts });` に替える(担当者を変えると「一緒に作るもの」の「親と同じ」も変わる)。
9. `send`: 作成の入力に `...(s.form.templateId ? { templateId: s.form.templateId } : {})` を足す。トーストを次にする。

```ts
    const others = othersOf(s.settings, s.form.templateId);
    const id = itemDisplayId(ws.prefix, created.number);
    deps.toasts.show(others > 0 ? `${id} ほか ${others} 件を作成しました` : `${id} を作成しました`);
```

10. `refresh` が返す `next` に `...view(ctx, shown)` を入れ、`rows: rowsFor(ctx, shown)` を消す。
11. ファイルが 300 行を超えたら、`wsOptions`・`parentOptions`・`missing` を `createDialogOptions.ts` に純粋な関数として移す(`wsOptions(stores)`・`parentOptions(prefix, candidates)`・`missing(kind, candidates)`)。

既存のテストで `form` を `toEqual` で比べているものは、`templateId: ""`(既定のテンプレートがない設定なので空)を足して直す。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web check && pnpm -C web --filter @hedgehug/core test`
Expected: PASS(既存の作成ダイアログのテストも含む)

- [ ] **Step 5: コミット**

```bash
git add web/packages/core
git commit -m "feat(core): 作成ダイアログでテンプレートを当てはめ、一緒に作るものを送る(A3b-12〜A3b-16)"
```

---

### Task 8: 作成ダイアログの画面にテンプレートの欄と「一緒に作るもの」を出す

**Files:**
- Modify: `web/app/src/screens/item/CreateItemFields.svelte`
- Create: `web/app/src/screens/item/CreateStructure.svelte`
- Test: `web/app/src/screens/item/CreateStructure.test.ts`(app の部品のテストの置き方に合わせる。app に部品のテストがなければ、このタスクの確認は Task 9 の e2e で行い、ここでは `pnpm -C web check` だけ)

**Interfaces:**
- Consumes: `ItemDialogState.templates`・`structure`、`itemDialog.update({ templateId })`

- [ ] **Step 1: 失敗するテストを書く(app に部品のテストがあるとき)**

```ts
import { render, screen } from "@testing-library/svelte";
import { expect, it } from "vitest";
import CreateStructure from "./CreateStructure.svelte";

it("shows the heading and one row per child and step with its assignee", () => {
  render(CreateStructure, {
    structure: {
      heading: "一緒に作るもの: 子 1 件・Step 1 件",
      rows: [
        { depth: 0, kind: "Story", title: "QA", assignee: "太郎" },
        { depth: 1, kind: "Step", title: "確認", assignee: "未割り当て" },
      ],
    },
  });
  expect(screen.getByText("一緒に作るもの: 子 1 件・Step 1 件")).toBeInTheDocument();
  const rows = screen.getAllByRole("listitem");
  expect(rows).toHaveLength(2);
  expect(rows[1]).toHaveTextContent("確認");
  expect(rows[1]).toHaveTextContent("未割り当て");
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `pnpm -C web --filter app test -- CreateStructure`(app のパッケージ名に合わせる)
Expected: FAIL

- [ ] **Step 3: 実装する**

`CreateStructure.svelte`:

```svelte
<script lang="ts">
  import type { StructureRow } from "@hedgehug/core";
  import { KindIcon } from "@hedgehug/ui";

  // 作成ダイアログの「一緒に作るもの」(02「ダイアログの項目」)。読み取り専用のツリー。
  interface Props {
    structure: { heading: string; rows: StructureRow[] };
  }

  let { structure }: Props = $props();
</script>

<section class="structure" aria-label={structure.heading}>
  <h3>{structure.heading}</h3>
  <ul>
    {#each structure.rows as r, i (i)}
      <li style:padding-left="{r.depth * 20}px">
        <KindIcon kind={r.kind} />
        <span class="title">{r.title}</span>
        <span class="assignee">{r.assignee}</span>
      </li>
    {/each}
  </ul>
</section>

<style>
  .structure h3 {
    font-size: 12px;
    margin: 0 0 4px;
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  li {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
  }
  .title {
    flex: 1;
  }
  .assignee {
    color: var(--muted);
  }
</style>
```

(`KindIcon` の props の名前は ui の実際のものに合わせる。)

`CreateItemFields.svelte`: 親の欄のあと、グリッドの中に足す。

```svelte
  {#if s.templates.length > 0}
    <div class="field">
      <span>テンプレート</span>
      <Select
        label="テンプレート"
        options={s.templates}
        bind:value={() => s.form.templateId, (v) => void itemDialog.update({ templateId: v })}
      />
    </div>
  {/if}
```

`<CreatePropertyFields {s} />` のあとに:

```svelte
{#if s.structure}
  <CreateStructure structure={s.structure} />
{/if}
```

先頭のコメントを「作成ダイアログの項目(02「ダイアログの項目」のうち、種類・ワークスペース・プロジェクト・親・テンプレート・タイトル・一緒に作るもの)。」に替える。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web check && pnpm -C web test`
Expected: PASS(Svelte 200 行以内)

- [ ] **Step 5: コミット**

```bash
git add web/app
git commit -m "feat(app): 作成ダイアログにテンプレートの欄と一緒に作るものを出す(A3b-12・A3b-15)"
```

---

### Task 9: 通しのテスト(e2e)

**Files:**
- Create: `web/e2e/create-template.spec.ts`

**Interfaces:**
- Consumes: ここまでの全部。`web/e2e/helpers.ts`(`sidebar`・`tokenUrl`・`unique`)と、`templates.spec.ts` の `prefixOf`・`createWorkspace`・`openSettings`(このファイルに写すか、`helpers.ts` に移して両方から使う。移すなら `templates.spec.ts` の import も直す)

- [ ] **Step 1: テストを書く**

テンプレートとプロジェクトの作り方、Epic と Task の作り方は、`templates.spec.ts`・`project.spec.ts`・`create.spec.ts` の操作をそのまま使う。

```ts
import { expect, test } from "@playwright/test";
import { createWorkspace, openSettings, prefixOf, tokenUrl, unique } from "./helpers";

// テンプレートでまとめて作る・遷移の制約の通しのテスト(A3b)。サーバーは 3 つの幅で共有なので、
// 名前と接頭辞は毎回変える。

test("creates a task with its template's steps in one go", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "テンプレートで作る"), prefix);
  // プロジェクトと Epic を作る(project.spec.ts・create.spec.ts と同じ操作)。
  // …
  // テンプレート「手順つき」: 種類 Task、書き出し「手順: 」、Step「準備」(親と同じ)・「記録」(未割り当て)。Task の既定にする。
  await openSettings(page, prefix, "テンプレート");
  // …(templates.spec.ts の作成の操作)

  // 作成ダイアログを開くと、既定が当たっている。
  await page.keyboard.press("c");
  const dialog = page.getByRole("dialog", { name: /作成/ }); // 既存の作成ダイアログの名前に合わせる
  await expect(dialog.getByRole("combobox", { name: "テンプレート" })).toHaveValue(/.+/);
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toHaveValue("手順: ");
  await expect(dialog.getByRole("region", { name: "一緒に作るもの: Step 2 件" })).toBeVisible();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("手順: ログイン");
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(page.getByText(`${prefix}-2 ほか 2 件を作成しました`)).toBeVisible();
  // 詳細パネルの Step に 2 件が並ぶ(番号は深さ優先)。
  await expect(page.getByText(`${prefix}-3`)).toBeVisible();
  await expect(page.getByText(`${prefix}-4`)).toBeVisible();
});

test("the status menu disables a status the workflow forbids and tells why", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "制約"), prefix);
  await openSettings(page, prefix, "ワークフロー");
  await page.getByRole("checkbox", { name: "Todo から InProgress", exact: true }).click();
  // プロジェクト・Epic・Task(Todo)を作り、詳細パネルのステータスのボタンを押す。
  // …
  const menu = page.getByRole("menu", { name: "ステータス" });
  await expect(menu.getByRole("menuitemcheckbox", { name: /Done/ })).toBeDisabled();
  await expect(menu).toContainText("ワークフローの制約: Todo から Done には変更できません(InProgress のみ)");
  await menu.getByRole("menuitemcheckbox", { name: /InProgress/ }).click();
  await expect(page.getByRole("button", { name: "ステータス: InProgress" }).first()).toBeVisible();
});

test("checking a step the workflow forbids shows the refusal and changes nothing", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "Step の制約"), prefix);
  await openSettings(page, prefix, "ワークフロー");
  await page.getByRole("checkbox", { name: "Todo から InProgress", exact: true }).click();
  // プロジェクト・Epic・Task を作り、詳細パネルで Step「確認」を足す。
  // …
  const step = page.getByRole("checkbox", { name: /確認/ }); // StepRow のチェックの名前に合わせる
  await step.click();
  await expect(page.getByText("ワークフローの制約: Todo から Done には変更できません(InProgress のみ)")).toBeVisible();
  await expect(step).not.toBeChecked();
});
```

「…」の部分は、既存の spec の同じ操作の行を写して埋める(この計画を書いた時点で、それぞれの操作のロケーターは既存の spec にある。新しい操作はない)。

- [ ] **Step 2: 走らせる**

Run: `make e2e`(なければ `pnpm -C web exec playwright test create-template`)
Expected: PASS(3 幅)。落ちたら、画面の振る舞いが仕様どおりか先に確かめ、テストの誤りか実装の誤りかを見分ける。

- [ ] **Step 3: コミット**

```bash
git add web/e2e
git commit -m "test(e2e): テンプレートでまとめて作る・遷移の制約の通しのテスト(A3b)"
```

---

### Task 10: 仕上げ

- [ ] `make check` と、e2e の全部を走らせ、結果を記録する。
- [ ] 最終レビュー(superpowers:requesting-code-review、または `code-review high`)。指摘は、直すものと先送りするもの(理由つき)に分ける。
- [ ] この計画の末尾に「実装で計画から変えたこと」を足す。
- [ ] PR を 1 本出す(向き先 develop、タイトル「feat: トラック A3 後半(テンプレートと遷移の制約)」)。本文に、できること・決めたこと(A3b-1〜A3b-16)・ui の `ValueOption` と `ValueMenu` を変えたこと・回したもの(A4・B5・F・C4/B4・E)・確認の結果を書く。

## 自己点検の記録

- 仕様の対応: 02「ステータスの変更」→ Task 1・2・5。02「Step のチェック」→ Task 2・9。02「開き方と初期値」(既定のテンプレート)→ Task 6・7。02「ダイアログの項目」(テンプレート・一緒に作るもの)→ Task 7・8。02「ダイアログの中での連動」(種類・ワークスペース・プロジェクト・テンプレート)→ Task 7。02「入力の検査」(作成は制約を適用しない、子と Step のタイトルの整え方)→ Task 2・3。02「作成したあと」(一緒に作るもの・番号・アクティビティ・表示・ダイアログ)→ Task 3・7。03「適用するときの規則」→ Task 3・6。説明・プロジェクト別のルール・スプリント・通知は「持ち場と境目」の表で回した。
- 名前の一貫: `CreateInput.Template`(Task 3)= API `template`(Task 4)= `NewItem.templateId`(Task 4・7)。`statusOptions(settings, current, workflow)`(Task 5)。`TemplateForm.templateId`(Task 6)= `ItemForm.templateId`(Task 7)。
