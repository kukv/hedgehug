# hedgehug トラック B(ワークスペースの設定)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 03 のワークスペースの設定(作成、一般、プロパティ、値、ワークフロー、テンプレート、自動化、プロジェクト別のルール)を、5 つの小計画に分けて作る。この文書は全体の一覧と、最初の小計画 B1 を細かく書く。B2 以降は、前の小計画が develop に入ってから書く。

**Architecture:** 設定の規則(値の定義、初期値、検査)は `domain/workspace` に、操作は `app/workspaces` に 1 ファイル 1 操作で置く。設定は保存形式のとおりワークスペースの `log.jsonl` の項目(`statuses/<値の内部 ID>/name` など)として持ち、`change.Resolve` でいまの値にする。HTTP は `adapter/httpapi/workspaces`、画面は `web/packages/core/features/workspaces` と `web/app/screens/workspace-settings`。

**Tech Stack:** 土台と同じ(Go 1.27.3、huma v2、go-git、Svelte 5、orval、Vitest、Playwright)。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md`(この計画の正本。全節)
- 01「用語と階層」「「あなた」の決まり方」「書き込みの単位」「閲覧のみの Store」
- 02「作成」(テンプレートの適用)、「名前の変更・移動・削除」「ステータスの置き換え」(プロジェクトの移動とルール)
- 06「同時に編集したとき」「修復の決まり」(同期の修復はトラック C の持ち場。下の漏れの表)
- `docs/superpowers/specs/2026-11-12-hedgehug-architecture-design.md`(パッケージ構成。8 節の持ち場と衝突を防ぐルール)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`(「変更記録」の `f` の書き方、「同期の修復」の表の (7)〜(9)、「移動・番号・旧 ID」)
- `docs/superpowers/plans/2026-11-12-hedgehug-foundation.md`(土台。型・口・関数名はこれに合わせる)

**持ち場:** `internal/domain/workspace`(土台の既存の型と関数は変えず、足すだけ)、`internal/domain/automation`、`internal/domain/projectrule`、`internal/app/workspaces`、`internal/adapter/httpapi/workspaces`、`web/packages/core/src/features/workspaces`、`web/packages/api/src/workspaces.ts`、`web/app/src/screens/workspace-settings`。

**待つもの:** 土台のすべて(Task 1〜12)と、トラック A の共有部分の PR(`docs/superpowers/plans/2026-11-12-hedgehug-track-a.md` の A1 Task 1)が develop に入っていること。A1 Task 1 は、`port.Identity`(偽物 `apptest.Identity`)、`domain/refusal`(断りの文言)、`httpapi/apierr`(422・404 への変換)、`domain/rank`(並び順の値)、`port.ItemReader.List`・`port.ProjectReader.List`、`httpapi.Deps` の `IDs`・`Identity` を入れる。B はこれらを自分で定義せず、そのまま使う。B1 が直接使うのは、Task 3(`change.Resolve`)・Task 4(`workspace.Workspace`・`workspace.Create`・`workspace.FromLog`)・Task 5(`port.Stores`・`apptest`)・Task 7(gitstore の Reader)・Task 8(`httpapi.Deps`・`routes.go`・`make generate`)・Task 10(`pnpm gen` と生成物の検査)。

## Global Constraints

土台の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 依存の向き: `app/workspaces` は `domain` と `app/port` だけを import する。ほかの `app/<機能>` は import しない。機能をまたぐ規則は `domain` に置く。
- 共有部分(`app/port`、`domain/workspace` の既存の型、`app/apptest`、`cmd`、`ui` の基本部品、`httpapi/routes.go` の行以外)を変えるときは、その変更だけの小さな PR を先に develop へ入れる(設計書 8 節)。
- 画面の文言・アクティビティの文・断りの文は、仕様 03 の文字どおりに書く。仕様にない文言を足さない。
- `api/openapi.yaml` と `web/packages/api/src/generated.ts` は手で直さない。API を変えたら `make generate` と `cd web && pnpm gen` で作り直し、同じコミットに入れる。

## この計画で決めたこと(仕様と設計書の規則の当てはめ)

| # | 決めたこと | 根拠 |
|---|---|---|
| 1 | ワークスペースの設定は、`workspace.Workspace`(ID・名前・接頭辞。土台の型のまま)とは別の型 `workspace.Settings` に持ち、口に `WorkspaceReader.Settings(id)` を足す | `Workspace` に一覧の欄を足すと `==` で比べられなくなり、土台の照合テスト(`w != cWorkspace`、`slices.Equal`)が壊れる。既存のフィールドは変えない |
| 2 | 設定はワークスペースの `log.jsonl` の項目で持つ。プロパティは `properties/<ID>/{key,name,type,builtin,hidden,rank}` と、値を複数持つ `properties/<ID>/kinds`。値は `statuses/<ID>/{name,color,category,rank}`・`priorities/<ID>/…`・`systems/<ID>/…`。ほかに `initial_status`・`initial_priority`・`category_default/<分類>` | 保存形式「変更記録」の `f` の例(`statuses/<値の内部 ID>/name`)。1 つの値ごとに 1 項目にして、項目ごとの後勝ちで合わせる(06) |
| 3 | 組み込みの 8 種のプロパティの ID は、キー(`status` など)と同じにする。追加プロパティの ID は内部 ID(ULID) | 組み込みはどのワークスペースにも同じものがあり、アイテムの側も `status` などの決まった項目で持つ(保存形式「アイテムの例」) |
| 4 | 作成時の初期値(組み込み 8 種、ステータス 5、優先度 5、関連システム 6、初期ステータス・初期優先度・分類の既定)は、作成の 1 行(`create` の `to`)にまとめて書く。並び(`rank`)は `domain/rank` で `rank.First` から順に振る(決定 13) | 01「書き込みの単位」(1 回の操作 = 1 回の保存)。並べ替えは隣と入れ替えるだけ(03「並べ替え」)なので、2 つの `rank` を交換すれば足りる |
| 5 | 色は、03「色」の 18 色を、画面のトークン `--c-<色>-*` の名前(`gray`・`slate`・`brown`・`red`・`rose`・`pink`・`magenta`・`purple`・`indigo`・`blue`・`sky`・`cyan`・`teal`・`green`・`olive`・`amber`・`yellow`・`orange`)で持つ。順は 03 の並び(グレー、青灰、茶、赤、ローズ、桃、赤紫、紫、藍、青、空色、水色、青緑、緑、オリーブ、山吹、黄、橙) | モック版 19 のトークンに同じ 18 色があり、順も一致する |
| 6 | 分類は `todo`・`doing`・`done`・`canceled` で持つ | 03「分類」の 4 つ。トークン `--cat-*` と同じ名前 |
| 7 | 接頭辞は、サーバーでも前後の空白を除いて大文字にしてから検査する。名前は前後の空白を除く | 03「接頭辞」(入力と同時に大文字にする)、03 の名前の重複の規則(保存する名前は前後の空白を除いた形) |
| 8 | 旧い接頭辞は、ワークスペースのログの `create` と `set prefix` の行に出たすべての接頭辞から、いまの接頭辞を除いたもの(辞書順)とし、`workspace.OldPrefixes(log)` で返す。別の記録は持たない | 保存形式「移動・番号・旧 ID」(旧い接頭辞はワークスペースの `log.jsonl` に残る `prefix` の過去の値すべて) |
| 9 | 操作が条件に合わないときは、A1 Task 1 の `refusal.New(文言)` を返し、HTTP では `apierr.From` で 422 と文言にする | 01「書き込みの単位」(何も変えずに理由を表示する)。文言は 03 の表のとおり。断りの型を A とそろえる |
| 13 | 値とプロパティの並び(`rank`)は、A1 Task 1 の `domain/rank` で振る(初期値は `rank.First` から `rank.After` を順に当てる) | アイテムの並びと同じ関数を使う。並べ替えは 2 つの値の `rank` を交換する |
| 10 | 同期で起きる修復(名前の「 (2)」、接頭辞の `WEB2`、キーの `customer2`、初期ステータスの戻し、初期値・既定にした値の削除の取り消し、削除と変更の重なり)は、トラック C の持ち場とし、B は作らない | 設計書 8 節のトラックの表(C: 同期の修復)。保存形式「同期の修復」の表 |
| 11 | `workspace.yaml` に設定を書き出すことと、`item.yaml` などの行末の値の名前の注釈は B2 で作る(土台の計画の決定 5) | 値の名前を変える操作(B2)と一緒に作るのが速い。B1 の間、`workspace.yaml` は名前と接頭辞だけ |
| 12 | 削除済みのワークスペースを名前・接頭辞の重複から除く規則は、ワークスペースの削除を作る B5 で足す。B1 の間は削除がないので、すべてを数える | 03「名前の重複」(削除済みのワークスペースは入れない)。削除がなければ結果は同じ |

---

## 小計画の一覧

| 小計画 | 範囲(仕様 03 の節) | 待つもの | 完成の条件 |
|---|---|---|---|
| **B1 設定の型・作成・名前・接頭辞(サーバー)** | 「ワークスペースの作成」(初期値と検査。Store の候補とダイアログの画面は B2)、「一般」の「名前」「接頭辞」(変更と旧い接頭辞の記録。確認ダイアログの件数・例は B2)、冒頭の表のうち作成・名前・接頭辞のアクティビティの文、名前の重複の規則 | 土台、A1 Task 1 | 設定が照合テストで偽物と gitstore の両方で読み書きでき、作成・名前・接頭辞の操作が仕様の検査と文言どおり HTTP で動く。`make check` が緑 |
| **B2 プロパティと値、設定の画面** | 「範囲と画面の構成」(8 つのサブタブ、サイドバーのワークスペースのメニュー、閲覧のみの見た目)、「ワークスペースの作成」のダイアログ、「一般」の名前・接頭辞の画面(確認ダイアログの件数と ID の例)、「プロパティ」全節、「値の編集」全節(色・名前・分類・分類の既定・初期ステータス・初期優先度・並べ替え・削除と置き換え。WIP 上限の付け替えを含む)、冒頭の表のプロパティ・値の行、「変更が及ぶ範囲」のプロパティ・値の行、`workspace.yaml` の設定の書き出しと値の名前の注釈 | B1、トラック A の A2(画面のアドレス)とサイドバー(下の未決の点 1)。アイテムの件数と置き換えは A1 Task 1 の `ItemReader.List` を使う | 設定の画面で 8 つのサブタブが開き、プロパティと値の操作が仕様の文言どおりに動く。値の削除で、使っているアイテムが置き換わる。e2e が 3 幅で通る |
| **B3 ワークフローとテンプレート** | 「ワークフロー」(遷移の制約の表と、制約の判定の規則 `workspace.CanTransition`)、「テンプレート」全節(持てるもの、一覧と編集、アイテムから作る、適用の規則、種類ごとの既定)、冒頭の表の行 | B2、トラック A の作成ダイアログと「ステータスを変える操作」(判定を呼ぶ場所) | 遷移の制約を編集でき、判定の関数が 03 の文言を返す。テンプレートを作り・直し・消せ、作成ダイアログでの適用の規則を core の関数として持つ |
| **B4 自動化** | 「自動化」全節(ルールの一覧、きっかけとすること、実行の規則、制約で止まったとき、最近の実行)、冒頭の表の行 | B3、トラック A(利用者の値の変更の操作)、トラック C(受信箱の口 `Inbox`)、トラック D(GitHub から取り直した変更の保存) | ルールを作り・直し・消せ、6 つのきっかけで、操作と同じ保存の中で動く。止まったときに受信箱へ通知が出る |
| **B5 プロジェクト別のルール、削除と別の Store への移動** | 「プロジェクト別のルール」全節、「範囲と画面の構成」のサブタブの上の 1 行、「一般」の「削除」と「別の Store へ移動」、「変更が及ぶ範囲」の残りの行、同期の修復(C3a の、06「親が削除されたとき」(2) に削除済みのワークスペースの下の子を足す。06「親と別のワークスペース・プロジェクトにある子」(5) に、移動先のプロジェクトで使わないステータスの置き換えを足す) | B4、トラック A のプロジェクトの設定タブとプロジェクトの移動、トラック C の `Replicator`(移動先の同期と送信) | プロジェクトの設定タブの 4 区画が仕様どおり動き、ワークスペースの論理削除と別の Store への移動が 1 回ずつの保存でできる |

### 漏れの表(仕様 03 の節 → 小計画)

| 節 | 小計画 |
|---|---|
| 冒頭(設定の位置づけ) | B2 |
| 範囲と画面の構成(サブタブ、メニュー、閲覧のみ、アクティビティとトーストの表、名前の重複の規則) | B2(画面)、表の各行は該当する小計画、名前の重複の規則は B1(ワークスペース)・B2(プロパティ・値)・B3(テンプレート)・B4(ルール) |
| 範囲と画面の構成の「同期で同じ名前が 2 つになったとき」 | トラック C(決定 10) |
| ワークスペースの作成 | B1(初期値と検査)、B2(ダイアログ、Store の候補、作ったあとに概要を開く) |
| 一般 / 名前 | B1(操作)、B2(画面) |
| 一般 / 接頭辞(変更・旧い接頭辞) | B1(操作と記録)、B2(画面、確認ダイアログの件数と例)。旧い接頭辞での検索はトラック E、画面のアドレスでの解決はトラック A、同期の `WEB2` はトラック C |
| 一般 / 別の Store へ移動 | B5 |
| 一般 / 削除 | B5(削除)。復元はトラック G(08) |
| プロパティ / 一覧・組み込みの 8 種・追加プロパティ・使う種類・非表示・並べ替え・削除 | B1(組み込み 8 種の初期値)、B2(残り)。同期の `customer2` はトラック C |
| 値の編集 / 色・名前・分類・分類ごとの既定・初期ステータス・初期優先度・並べ替え・削除 | B1(初期値)、B2(操作)。同期での削除の取り消しと初期ステータスの戻しはトラック C |
| ワークフロー / 遷移の制約 | B3 |
| テンプレート / 持てるもの・一覧と編集・アイテムから作る・適用するときの規則・種類ごとの既定 | B3(作成ダイアログへのはめ込みはトラック A と調整) |
| 自動化 / ルールの一覧・きっかけとすること・実行の規則・制約で止まったとき・最近の実行 | B4 |
| プロジェクト別のルール / 使うステータス・遷移の制約・テンプレート・自動化・戻す・移動・削除・復元 | B5(プロジェクトの移動での引き継ぎは、トラック A の移動と同じ保存に入れる) |
| 変更が及ぶ範囲 | B2(プロパティ・値)、B3(遷移の制約)、B5(移動・削除・プロジェクト別のルール)。接頭辞の行は B1 |
| モックからの変更点 | 各行の中身は上の節に含まれる(この表はレビュー用で、作るものを足さない) |

---

## 小計画 B1: 設定の型・作成・名前・接頭辞(サーバー)

### ファイルの地図

```
internal/domain/workspace/settings.go        Settings・Value・Property・Category・Color・PropertyType
internal/domain/workspace/defaults.go        DefaultSettings(作成時の初期値)
internal/domain/workspace/settings_log.go    CreateWithSettings・SettingsFromLog
internal/domain/workspace/prefixes.go        OldPrefixes
internal/domain/workspace/*_test.go

internal/app/port/stores.go                  WorkspaceReader に Settings を足す(共有 PR)
internal/app/apptest/stores_read.go          偽物の Settings(共有 PR)
internal/app/apptest/stores_contract.go      照合テストの場面を 1 つ足す(共有 PR)
internal/app/apptest/stores_contract_settings.go
internal/app/apptest/sample.go               サンプルに初期値の設定を入れる(共有 PR)
internal/infra/gitstore/read.go              gitstore の Settings(共有 PR)
internal/adapter/httpapi/routes.go           workspaces.Register の 1 行

internal/app/workspaces/deps.go              Deps・断りの文言
internal/app/workspaces/create.go            Create
internal/app/workspaces/rename.go            Rename
internal/app/workspaces/prefix.go            ChangePrefix
internal/app/workspaces/get.go               List・Get
internal/app/workspaces/*_test.go

internal/adapter/httpapi/workspaces/workspaces.go  Register と HTTP の誤りへの変換
internal/adapter/httpapi/workspaces/dto.go         本文の形
internal/adapter/httpapi/workspaces/workspaces_test.go
api/openapi.yaml / web/packages/api/src/generated.ts(生成物)
```

---

### Task B1-1(共有 PR): ワークスペースの設定の型と、口の `Settings`

`domain/workspace` に設定の型と初期値を足し、口 `WorkspaceReader` に `Settings` を足して、偽物と gitstore の両方で照合テストを通す。トラック A もステータス・優先度を読むので、この PR だけを先に develop へ入れる。

**Files:**
- Create: `internal/domain/workspace/settings.go`、`internal/domain/workspace/defaults.go`、`internal/domain/workspace/settings_log.go`
- Test: `internal/domain/workspace/settings_test.go`
- Modify: `internal/domain/workspace/workspace.go`(`opts` に `Multi` を足すだけ。型と関数は変えない)
- Modify: `internal/app/port/stores.go`(`WorkspaceReader` に `Settings`)
- Modify: `internal/app/apptest/stores_read.go`、`internal/infra/gitstore/read.go`(`Settings` の実装)
- Modify: `internal/app/apptest/stores_contract.go`(場面を 1 つ足す)
- Create: `internal/app/apptest/stores_contract_settings.go`
- Modify: `internal/app/apptest/sample.go`(初期値の設定で作る。ID の定数を足す)

**Interfaces:**
- Consumes: 土台の `change.Resolve`・`change.NewCreate`・`change.Record`、`workspace.Workspace`・`workspace.Create`・`workspace.ValidPrefix`・`workspace.FromLog`、`port.WorkspaceReader`、`apptest.RunStoresContract`
- Produces:
  - `workspace.Category`(`CategoryTodo`・`CategoryDoing`・`CategoryDone`・`CategoryCanceled`)、`workspace.Categories []Category`
  - `workspace.Color`、`workspace.Colors []Color`(18 色、03 の順)
  - `workspace.PropertyType`(`TypeStatus`・`TypeUser`・`TypePriority`・`TypeDate`・`TypeNumber`・`TypeDuration`・`TypeSystems`・`TypeText`・`TypeSingle`・`TypeMulti`・`TypeCheck`・`TypeURL`)
  - `workspace.Kinds []string`(`Epic`・`Story`・`Task`・`Bug`・`Step`)
  - `workspace.Value{ID change.ID; Name string; Color Color; Category Category; Rank string}`
  - `workspace.Property{ID change.ID; Key, Name string; Type PropertyType; Builtin bool; Kinds []string; Hidden bool; Rank string}`
  - `workspace.Settings{Properties []Property; Statuses, Priorities, Systems []Value; InitialStatus, InitialPriority change.ID; CategoryDefaults map[Category]change.ID}`(一覧は `Rank` の順)
  - `workspace.DefaultSettings(newID func() change.ID) Settings`
  - `workspace.CreateWithSettings(w Workspace, s Settings) (change.Draft, error)`
  - `workspace.SettingsFromLog(log []change.Record) (Settings, bool)`
  - `port.WorkspaceReader.Settings(id workspace.ID) (workspace.Settings, error)`
  - `apptest.SampleStatusTodo`・`SampleStatusInProgress`・`SampleStatusDone`・`SampleStatusPending`・`SampleStatusCancel`・`SamplePriorityMedium`(サンプルの値の内部 ID)

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/workspace/settings_test.go`:

```go
package workspace_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func seqIDs() func() change.ID {
	n := 0
	return func() change.ID {
		n++
		return change.ID(fmt.Sprintf("01VAL%021d", n))
	}
}

func names(vs []workspace.Value) []string {
	var out []string
	for _, v := range vs {
		out = append(out, v.Name+":"+string(v.Color)+":"+string(v.Category))
	}
	return out
}

// 03「ワークスペースの作成」の初期値。
func TestDefaultSettings(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	if got := names(s.Statuses); !reflect.DeepEqual(got, []string{
		"Todo:gray:todo", "InProgress:blue:doing", "Done:green:done", "Pending:yellow:todo", "Cancel:gray:canceled",
	}) {
		t.Errorf("statuses = %v", got)
	}
	if got := names(s.Priorities); !reflect.DeepEqual(got, []string{
		"Highest:red:", "High:orange:", "Medium:yellow:", "Low:blue:", "Lowest:gray:",
	}) {
		t.Errorf("priorities = %v", got)
	}
	if got := names(s.Systems); !reflect.DeepEqual(got, []string{
		"Web:blue:", "API:teal:", "iOS:gray:", "Android:green:", "DB:purple:", "CI:yellow:",
	}) {
		t.Errorf("systems = %v", got)
	}
	if s.InitialStatus != s.Statuses[0].ID || s.InitialPriority != s.Priorities[2].ID {
		t.Errorf("initial = %s / %s", s.InitialStatus, s.InitialPriority)
	}
	want := map[workspace.Category]change.ID{
		workspace.CategoryTodo: s.Statuses[0].ID, workspace.CategoryDoing: s.Statuses[1].ID,
		workspace.CategoryDone: s.Statuses[2].ID, workspace.CategoryCanceled: s.Statuses[4].ID,
	}
	if !reflect.DeepEqual(s.CategoryDefaults, want) {
		t.Errorf("category defaults = %v", s.CategoryDefaults)
	}
	var keys []string
	for _, p := range s.Properties {
		keys = append(keys, fmt.Sprintf("%s:%s:%s:%v", p.Key, p.Name, p.Type, p.Builtin))
		if string(p.ID) != p.Key || !reflect.DeepEqual(p.Kinds, workspace.Kinds) || p.Hidden {
			t.Errorf("property %+v", p)
		}
	}
	if !reflect.DeepEqual(keys, []string{
		"status:ステータス:status:true", "assignee:担当者:user:true", "priority:優先度:priority:true",
		"start:開始日:date:true", "end:終了日:date:true", "points:ポイント:number:true",
		"estimate:見積もり時間:duration:true", "systems:関連システム:systems:true",
	}) {
		t.Errorf("properties = %v", keys)
	}
}

func TestColorsAreTheEighteenOfSpec03(t *testing.T) {
	if len(workspace.Colors) != 18 || workspace.Colors[0] != "gray" || workspace.Colors[17] != "orange" {
		t.Errorf("colors = %v", workspace.Colors)
	}
}

func stamp(log []change.Record, d change.Draft, id string, sec int) []change.Record {
	meta := change.Meta{Op: change.ID(id), At: time.Date(2026, 11, 12, 9, 0, sec, 0, time.UTC), Actor: change.User("taro")}
	return append(log, change.Stamp(log, []change.Draft{d}, meta, func() change.ID { return change.ID(id) })...)
}

func TestSettingsRoundTripThroughTheLog(t *testing.T) {
	w := workspace.Workspace{ID: "01WS0000000000000000000001", Name: "Web", Prefix: "HH"}
	s := workspace.DefaultSettings(seqIDs())
	d, err := workspace.CreateWithSettings(w, s)
	if err != nil {
		t.Fatal(err)
	}
	log := stamp(nil, d, "c", 0)
	got, ok := workspace.SettingsFromLog(log)
	if !ok || !reflect.DeepEqual(got, s) {
		t.Fatalf("SettingsFromLog =\n%+v\nwant\n%+v", got, s)
	}
	if back, ok := workspace.FromLog(log); !ok || back != w {
		t.Errorf("FromLog = %+v", back)
	}
	// 値の名前を変え、使う種類から Step を外すと、その項目だけが変わる。
	todo := s.Statuses[0].ID
	log = stamp(log, change.NewSet("statuses/"+string(todo)+"/name", "Todo", "未着手"), "s1", 1)
	log = stamp(log, change.NewRemove("properties/assignee/kinds", "Step"), "s2", 2)
	got, _ = workspace.SettingsFromLog(log)
	if got.Statuses[0].Name != "未着手" || got.Statuses[1].Name != "InProgress" {
		t.Errorf("statuses = %v", names(got.Statuses))
	}
	if !reflect.DeepEqual(got.Properties[1].Kinds, []string{"Epic", "Story", "Task", "Bug"}) {
		t.Errorf("assignee kinds = %v", got.Properties[1].Kinds)
	}
}

func TestInvalidColorAndCategoryRowsAreIgnored(t *testing.T) {
	w := workspace.Workspace{ID: "01WS0000000000000000000001", Name: "Web", Prefix: "HH"}
	s := workspace.DefaultSettings(seqIDs())
	d, _ := workspace.CreateWithSettings(w, s)
	todo := "statuses/" + string(s.Statuses[0].ID)
	log := stamp(nil, d, "c", 0)
	log = stamp(log, change.NewSet(todo+"/color", "gray", "chartreuse"), "s1", 1)
	log = stamp(log, change.NewSet(todo+"/category", "todo", "someday"), "s2", 2)
	got, _ := workspace.SettingsFromLog(log)
	if got.Statuses[0].Color != "gray" || got.Statuses[0].Category != workspace.CategoryTodo {
		t.Errorf("todo = %+v", got.Statuses[0])
	}
}

func TestCreateWithSettingsChecksLikeCreate(t *testing.T) {
	s := workspace.DefaultSettings(seqIDs())
	if _, err := workspace.CreateWithSettings(workspace.Workspace{ID: "W", Name: "Web", Prefix: "hh"}, s); err == nil {
		t.Error("bad prefix accepted")
	}
	if _, err := workspace.CreateWithSettings(workspace.Workspace{ID: "W", Name: " ", Prefix: "HH"}, s); err == nil {
		t.Error("empty name accepted")
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/workspace/`
Expected: FAIL(`undefined: workspace.DefaultSettings` ほか)

- [ ] **Step 3: 型を書く**

`internal/domain/workspace/settings.go`:

```go
package workspace

import "github.com/kukv/hedgehug/internal/domain/change"

// Category はステータスの分類(03「分類」)。
type Category string

const (
	CategoryTodo     Category = "todo"
	CategoryDoing    Category = "doing"
	CategoryDone     Category = "done"
	CategoryCanceled Category = "canceled"
)

// Categories は分類の並び(未着手・進行中・完了・中止)。
var Categories = []Category{CategoryTodo, CategoryDoing, CategoryDone, CategoryCanceled}

// Color は値の色。名前は画面のトークン --c-<色>-* と同じ。
type Color string

// Colors は 03「色」の 18 色を、03 の並び(グレー、青灰、茶、赤、ローズ、桃、赤紫、紫、藍、青、
// 空色、水色、青緑、緑、オリーブ、山吹、黄、橙)で持つ。
var Colors = []Color{
	"gray", "slate", "brown", "red", "rose", "pink", "magenta", "purple", "indigo",
	"blue", "sky", "cyan", "teal", "green", "olive", "amber", "yellow", "orange",
}

// PropertyType はプロパティの型(03「組み込みの 8 種」「追加プロパティ」)。
type PropertyType string

const (
	TypeStatus   PropertyType = "status"
	TypeUser     PropertyType = "user"
	TypePriority PropertyType = "priority"
	TypeDate     PropertyType = "date"
	TypeNumber   PropertyType = "number"
	TypeDuration PropertyType = "duration" // 組み込みの見積もり時間だけが持つ
	TypeSystems  PropertyType = "systems"
	TypeText     PropertyType = "text"
	TypeSingle   PropertyType = "single"
	TypeMulti    PropertyType = "multi"
	TypeCheck    PropertyType = "check"
	TypeURL      PropertyType = "url"
)

// Kinds は、プロパティを使う種類に選べるアイテムの種類。item は workspace を import するので、
// item.Kind ではなく同じ文字で持つ。
var Kinds = []string{"Epic", "Story", "Task", "Bug", "Step"}

// Value は、ステータス・優先度・関連システムの値の 1 つ。Category はステータスだけが持つ。
type Value struct {
	ID       change.ID
	Name     string
	Color    Color
	Category Category
	Rank     string
}

// Property は、アイテムの項目の定義。組み込みの ID はキーと同じ。
type Property struct {
	ID      change.ID
	Key     string
	Name    string
	Type    PropertyType
	Builtin bool
	Kinds   []string // 使う種類(Kinds の並び)
	Hidden  bool
	Rank    string
}

// Settings は、ワークスペースの設定。一覧はどれも Rank の順。
type Settings struct {
	Properties       []Property
	Statuses         []Value
	Priorities       []Value
	Systems          []Value
	InitialStatus    change.ID
	InitialPriority  change.ID
	CategoryDefaults map[Category]change.ID
}

func validColor(c Color) bool {
	for _, x := range Colors {
		if x == c {
			return true
		}
	}
	return false
}

func validCategory(c Category) bool {
	for _, x := range Categories {
		if x == c {
			return true
		}
	}
	return false
}
```

`internal/domain/workspace/defaults.go`:

```go
package workspace

import (
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

type builtin struct {
	key, name string
	typ       PropertyType
}

// builtins は 03「組み込みの 8 種」の表の並び。
var builtins = []builtin{
	{"status", "ステータス", TypeStatus},
	{"assignee", "担当者", TypeUser},
	{"priority", "優先度", TypePriority},
	{"start", "開始日", TypeDate},
	{"end", "終了日", TypeDate},
	{"points", "ポイント", TypeNumber},
	{"estimate", "見積もり時間", TypeDuration},
	{"systems", "関連システム", TypeSystems},
}

// rankAt は、初期値の並びの i 番目の順位(rank.First から rank.After を i 回)。辞書順で並ぶ。
func rankAt(i int) string {
	r := rank.First
	for range i {
		r = rank.After(r)
	}
	return r
}

func values(newID func() change.ID, vs []Value) []Value {
	for i := range vs {
		vs[i].ID = newID()
		vs[i].Rank = rankAt(i)
	}
	return vs
}

// DefaultSettings は、作ったワークスペースの初期値(03「ワークスペースの作成」)。値の内部 ID は newID で振る。
// 振る順は、ステータス 5、優先度 5、関連システム 6。
func DefaultSettings(newID func() change.ID) Settings {
	s := Settings{}
	for i, b := range builtins {
		s.Properties = append(s.Properties, Property{
			ID: change.ID(b.key), Key: b.key, Name: b.name, Type: b.typ, Builtin: true,
			Kinds: slices.Clone(Kinds), Rank: rankAt(i),
		})
	}
	s.Statuses = values(newID, []Value{
		{Name: "Todo", Color: "gray", Category: CategoryTodo},
		{Name: "InProgress", Color: "blue", Category: CategoryDoing},
		{Name: "Done", Color: "green", Category: CategoryDone},
		{Name: "Pending", Color: "yellow", Category: CategoryTodo},
		{Name: "Cancel", Color: "gray", Category: CategoryCanceled},
	})
	s.Priorities = values(newID, []Value{
		{Name: "Highest", Color: "red"}, {Name: "High", Color: "orange"}, {Name: "Medium", Color: "yellow"},
		{Name: "Low", Color: "blue"}, {Name: "Lowest", Color: "gray"},
	})
	s.Systems = values(newID, []Value{
		{Name: "Web", Color: "blue"}, {Name: "API", Color: "teal"}, {Name: "iOS", Color: "gray"},
		{Name: "Android", Color: "green"}, {Name: "DB", Color: "purple"}, {Name: "CI", Color: "yellow"},
	})
	s.InitialStatus = s.Statuses[0].ID
	s.InitialPriority = s.Priorities[2].ID
	s.CategoryDefaults = map[Category]change.ID{
		CategoryTodo:     s.Statuses[0].ID,
		CategoryDoing:    s.Statuses[1].ID,
		CategoryDone:     s.Statuses[2].ID,
		CategoryCanceled: s.Statuses[4].ID,
	}
	return s
}
```

- [ ] **Step 4: ログとの読み書きを書く**

`internal/domain/workspace/settings_log.go`:

```go
package workspace

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// valueLists は、値の一覧の項目名の頭。
var valueLists = []string{"statuses", "priorities", "systems"}

// CreateWithSettings は、設定を含めたワークスペースの作成の変更。検査は Create と同じ。
// 設定は項目ごと(statuses/<ID>/name など)に、作成の 1 行の to に書く(この計画の決定 2・4)。
func CreateWithSettings(w Workspace, s Settings) (change.Draft, error) {
	if _, err := Create(w); err != nil {
		return change.Draft{}, err
	}
	m := map[string]any{"id": w.ID, "name": w.Name, "prefix": w.Prefix}
	for _, p := range s.Properties {
		base := "properties/" + string(p.ID) + "/"
		m[base+"key"], m[base+"name"], m[base+"type"] = p.Key, p.Name, p.Type
		m[base+"builtin"], m[base+"kinds"], m[base+"hidden"], m[base+"rank"] = p.Builtin, p.Kinds, p.Hidden, p.Rank
	}
	for i, vs := range [][]Value{s.Statuses, s.Priorities, s.Systems} {
		for _, v := range vs {
			base := valueLists[i] + "/" + string(v.ID) + "/"
			m[base+"name"], m[base+"color"], m[base+"rank"] = v.Name, v.Color, v.Rank
			if i == 0 {
				m[base+"category"] = v.Category
			}
		}
	}
	m["initial_status"], m["initial_priority"] = s.InitialStatus, s.InitialPriority
	for c, id := range s.CategoryDefaults {
		m["category_default/"+string(c)] = id
	}
	return change.NewCreate(m), nil
}

// SettingsFromLog は、ワークスペースのログから設定を作る。作成の行がなければ false。
func SettingsFromLog(log []change.Record) (Settings, bool) {
	st := change.Resolve(log, opts)
	if st.Created == nil {
		return Settings{}, false
	}
	b := newBuilder()
	for f, raw := range st.Values {
		b.set(f, raw)
	}
	for f, list := range st.Lists {
		b.setKinds(f, list)
	}
	return b.settings(), true
}

// builder は、項目ごとの値を集めて Settings にする。
type builder struct {
	props    map[change.ID]*Property
	vals     map[string]map[change.ID]*Value
	acc      Settings
}

func newBuilder() *builder {
	b := &builder{props: map[change.ID]*Property{}, vals: map[string]map[change.ID]*Value{}}
	for _, l := range valueLists {
		b.vals[l] = map[change.ID]*Value{}
	}
	b.acc.CategoryDefaults = map[Category]change.ID{}
	return b
}

func (b *builder) prop(id string) *Property {
	p, ok := b.props[change.ID(id)]
	if !ok {
		p = &Property{ID: change.ID(id)}
		b.props[change.ID(id)] = p
	}
	return p
}

func (b *builder) value(list, id string) *Value {
	v, ok := b.vals[list][change.ID(id)]
	if !ok {
		v = &Value{ID: change.ID(id)}
		b.vals[list][change.ID(id)] = v
	}
	return v
}

func (b *builder) set(f string, raw json.RawMessage) {
	parts := strings.Split(f, "/")
	switch {
	case len(parts) == 3 && parts[0] == "properties":
		setProperty(b.prop(parts[1]), parts[2], raw)
	case len(parts) == 3 && slices.Contains(valueLists, parts[0]):
		setValue(b.value(parts[0], parts[1]), parts[2], raw)
	case f == "initial_status":
		_ = json.Unmarshal(raw, &b.acc.InitialStatus)
	case f == "initial_priority":
		_ = json.Unmarshal(raw, &b.acc.InitialPriority)
	case len(parts) == 2 && parts[0] == "category_default":
		var id change.ID
		if json.Unmarshal(raw, &id) == nil {
			b.acc.CategoryDefaults[Category(parts[1])] = id
		}
	}
}

func (b *builder) setKinds(f string, list []json.RawMessage) {
	parts := strings.Split(f, "/")
	if len(parts) != 3 || parts[0] != "properties" || parts[2] != "kinds" {
		return
	}
	p := b.prop(parts[1])
	for _, k := range Kinds { // Kinds の並びにそろえる
		for _, raw := range list {
			var s string
			if json.Unmarshal(raw, &s) == nil && s == k {
				p.Kinds = append(p.Kinds, k)
			}
		}
	}
}

func setProperty(p *Property, field string, raw json.RawMessage) {
	switch field {
	case "key":
		_ = json.Unmarshal(raw, &p.Key)
	case "name":
		_ = json.Unmarshal(raw, &p.Name)
	case "type":
		_ = json.Unmarshal(raw, &p.Type)
	case "builtin":
		_ = json.Unmarshal(raw, &p.Builtin)
	case "hidden":
		_ = json.Unmarshal(raw, &p.Hidden)
	case "rank":
		_ = json.Unmarshal(raw, &p.Rank)
	}
}

func setValue(v *Value, field string, raw json.RawMessage) {
	switch field {
	case "name":
		_ = json.Unmarshal(raw, &v.Name)
	case "color":
		_ = json.Unmarshal(raw, &v.Color)
	case "category":
		_ = json.Unmarshal(raw, &v.Category)
	case "rank":
		_ = json.Unmarshal(raw, &v.Rank)
	}
}

func byRank[T any](m map[change.ID]*T, rank func(*T) (string, change.ID)) []T {
	out := make([]T, 0, len(m))
	for _, x := range m {
		out = append(out, *x)
	}
	slices.SortFunc(out, func(a, b T) int {
		ra, ia := rank(&a)
		rb, ib := rank(&b)
		if c := strings.Compare(ra, rb); c != 0 {
			return c
		}
		return strings.Compare(string(ia), string(ib))
	})
	return out
}

func (b *builder) settings() Settings {
	s := b.acc
	s.Properties = byRank(b.props, func(p *Property) (string, change.ID) { return p.Rank, p.ID })
	valueRank := func(v *Value) (string, change.ID) { return v.Rank, v.ID }
	s.Statuses = byRank(b.vals["statuses"], valueRank)
	s.Priorities = byRank(b.vals["priorities"], valueRank)
	s.Systems = byRank(b.vals["systems"], valueRank)
	return s
}
```

空の一覧は `nil` でなく長さ 0 になるが、`DefaultSettings` の一覧はどれも空でないので、往復のテストでは食い違わない。

`internal/domain/workspace/workspace.go` の `opts` を、次に置き換える(型・関数は変えない。`FromLog` と `SettingsFromLog` が同じ決め方を使う)。

```go
var opts = change.Options{
	Multi: func(f string) bool { return strings.HasSuffix(f, "/kinds") },
	Valid: valid,
}

func valid(r change.Record) bool {
	switch {
	case r.T == change.Set && r.F == "prefix":
		var p string
		return json.Unmarshal(r.To, &p) == nil && ValidPrefix(p)
	case r.T == change.Create:
		var v struct{ Prefix string }
		return json.Unmarshal(r.To, &v) == nil && ValidPrefix(v.Prefix)
	case r.T == change.Set && strings.HasSuffix(r.F, "/color"):
		var c Color
		return json.Unmarshal(r.To, &c) == nil && validColor(c)
	case r.T == change.Set && strings.HasSuffix(r.F, "/category"):
		var c Category
		return json.Unmarshal(r.To, &c) == nil && validCategory(c)
	}
	return true
}
```

- [ ] **Step 5: domain のテストが通ることを確かめる**

Run: `go test ./internal/domain/... && go test ./internal/app/... ./internal/infra/...`
Expected: PASS(土台のテストもそのまま通る)

- [ ] **Step 6: 口に `Settings` を足し、偽物と gitstore で実装する**

`internal/app/port/stores.go` の `WorkspaceReader` を次にする。

```go
type WorkspaceReader interface {
	Get(id workspace.ID) (workspace.Workspace, error)
	List() ([]workspace.Workspace, error) // 内部 ID の順
	Settings(id workspace.ID) (workspace.Settings, error)
}
```

`internal/app/apptest/stores_read.go` の `workspaces` に足す:

```go
func (r workspaces) Settings(id workspace.ID) (workspace.Settings, error) {
	s, ok := workspace.SettingsFromLog(r.v.log(change.Ref{Kind: change.KindWorkspace, ID: id}))
	if !ok {
		return workspace.Settings{}, port.ErrNotFound
	}
	return s, nil
}
```

`internal/infra/gitstore/read.go` の `workspaces` に足す:

```go
func (r workspaces) Settings(id workspace.ID) (workspace.Settings, error) {
	return get(r.v, change.Ref{Kind: change.KindWorkspace, ID: id}, workspace.SettingsFromLog)
}
```

- [ ] **Step 7: 照合テストに場面を足す**

`internal/app/apptest/stores_contract_settings.go`:

```go
package apptest

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// contractSettings は、設定を含めて作ったワークスペースの設定が、そのまま読めることを確かめる。
func contractSettings(t *testing.T, s port.Stores, id port.StoreID) {
	n := 0
	settings := workspace.DefaultSettings(func() change.ID {
		n++
		return change.ID("01CONTRACTVAL" + string(rune('A'+n)) + "000000000000")
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		d, err := workspace.CreateWithSettings(cWorkspace, settings)
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(cWorkspace.ID, d)
	})
	mustRead(t, s, id, func(r port.Reader) error {
		got, err := r.Workspaces().Settings(cWorkspace.ID)
		if err != nil || !reflect.DeepEqual(got, settings) {
			t.Errorf("Settings = %+v, %v", got, err)
		}
		if _, err := r.Workspaces().Settings("01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("unknown workspace: %v", err)
		}
		return nil
	})
}
```

値の内部 ID は 26 文字の英数字にする(gitstore のパスの検査に合わせる)。`"01CONTRACTVAL" + 1 文字 + "000000000000"` は 26 文字である。

`internal/app/apptest/stores_contract.go` の `cases` の末尾に 1 行足す:

```go
		{"workspace settings round trip", contractSettings},
```

- [ ] **Step 8: サンプルを初期値の設定で作る**

`internal/app/apptest/sample.go` の定数に足す:

```go
// サンプルのワークスペースの値の内部 ID(DefaultSettings が振る順: ステータス 5、優先度 5、関連システム 6)。
const (
	SampleStatusTodo       change.ID = "01SAMPLEVAL000000000000001"
	SampleStatusInProgress change.ID = "01SAMPLEVAL000000000000002"
	SampleStatusDone       change.ID = "01SAMPLEVAL000000000000003"
	SampleStatusPending    change.ID = "01SAMPLEVAL000000000000004"
	SampleStatusCancel     change.ID = "01SAMPLEVAL000000000000005"
	SamplePriorityMedium   change.ID = "01SAMPLEVAL000000000000008"
)

func sampleValueIDs() func() change.ID {
	n := 0
	return func() change.ID {
		n++
		return change.ID(fmt.Sprintf("01SAMPLEVAL%015d", n))
	}
}
```

`Sample`(A1 Task 1 で番号の行を足したあとの形)の `d, err := workspace.Create(ws)` を次にする(import に `fmt` を足す)。ワークスペースへの書き込み(`Append(ws.ID, d, numbering.Draft(…)…)`)はそのまま:

```go
		d, err := workspace.CreateWithSettings(ws, workspace.DefaultSettings(sampleValueIDs()))
```

`TestSample`(`stores_test.go`)に、設定が読めることを足す:

```go
	err = s.Read(t.Context(), "S1", func(r port.Reader) error {
		st, err := r.Workspaces().Settings(apptest.SampleWorkspace)
		if err != nil || st.InitialStatus != apptest.SampleStatusTodo || st.InitialPriority != apptest.SamplePriorityMedium {
			t.Errorf("settings = %+v, %v", st, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
```

(既存の `err := s.Read(...)` のあとに置くので、こちらは `err =` で代入する。)

- [ ] **Step 9: 全体を確かめる**

Run: `make check`
Expected: すべて成功。照合テスト「workspace settings round trip」が偽物と gitstore の両方で PASS。gitstore の `TestYAMLIsRebuiltFromLog` は `item.yaml` だけを見るので変わらない。

- [ ] **Step 10: Commit と PR**

```bash
git add internal/domain/workspace internal/app/port internal/app/apptest internal/infra/gitstore/read.go
git commit -m "feat(domain): ワークスペースの設定の型と初期値を足し、口で読めるようにする"
```

この PR は共有部分の変更なので、単独で develop に入れ、トラック A に取り込み(rebase)を知らせる。

---

### Task B1-2: ワークスペースの作成

**Files:**
- Create: `internal/app/workspaces/deps.go`、`internal/app/workspaces/create.go`
- Test: `internal/app/workspaces/create_test.go`、`internal/app/workspaces/helpers_test.go`

**Interfaces:**
- Consumes: B1-1 の `workspace.DefaultSettings`・`workspace.CreateWithSettings`、A1 Task 1 の `port.Identity`・`apptest.Identity`・`refusal.New`、土台の `port.Stores`・`port.IDs`・`apptest.*`
- Produces:
  - `workspaces.Deps{ Stores port.Stores; IDs port.IDs; Identity port.Identity }`
  - 断りは `refusal.Refusal`(A1 Task 1)で返す
  - `workspaces.Create(ctx context.Context, d Deps, store port.StoreID, name, prefix string) (workspace.Workspace, error)`

**仕様の検査(03「ワークスペースの作成」の表の順):** 名前が空 →「名前を入力してください」、同じ Store に同じ名前 →「この Store には同じ名前のワークスペースがあります」、接頭辞が空 →「接頭辞を入力してください」、接頭辞の形が違う →「接頭辞は英大文字で始まる 10 文字以内の英大文字と数字にしてください」、同じ Store に同じ接頭辞 →「この Store には同じ接頭辞のワークスペースがあります」。アクティビティの文は「ワークスペース「X」を作成」。

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/workspaces/helpers_test.go`:

```go
package workspaces_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const store port.StoreID = "S1"

func newDeps(t *testing.T) workspaces.Deps {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	return workspaces.Deps{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "taro"}}
}

// rejected は、err が画面に出す文言 msg の断りであることを確かめる。
func rejected(t *testing.T, err error, msg string) {
	t.Helper()
	var r *refusal.Refusal
	if !errors.As(err, &r) || r.Message != msg {
		t.Errorf("error = %v, want %q", err, msg)
	}
}
```

`internal/app/workspaces/create_test.go`:

```go
package workspaces_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
)

func TestCreate(t *testing.T) {
	d := newDeps(t)
	w, err := workspaces.Create(t.Context(), d, store, "  Web  ", " hh ")
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "Web" || w.Prefix != "HH" {
		t.Errorf("workspace = %+v", w)
	}
	err = d.Stores.Read(t.Context(), store, func(r port.Reader) error {
		st, err := r.Workspaces().Settings(w.ID)
		if err != nil || len(st.Statuses) != 5 || len(st.Properties) != 8 {
			t.Errorf("settings = %+v, %v", st, err)
		}
		log, _ := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
		if len(log) != 1 || log[0].T != change.Create || log[0].By != "taro" {
			t.Errorf("log = %+v", log)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateRefuses(t *testing.T) {
	d := newDeps(t)
	if _, err := workspaces.Create(t.Context(), d, store, "Web", "HH"); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, prefix, msg string }{
		{" ", "API", "名前を入力してください"},
		{"Web", "", "この Store には同じ名前のワークスペースがあります"},
		{"API", "", "接頭辞を入力してください"},
		{"API", "1API", "接頭辞は英大文字で始まる 10 文字以内の英大文字と数字にしてください"},
		{"API", "ABCDEFGHIJK", "接頭辞は英大文字で始まる 10 文字以内の英大文字と数字にしてください"},
		{"API", "hh", "この Store には同じ接頭辞のワークスペースがあります"},
	}
	for _, c := range cases {
		_, err := workspaces.Create(t.Context(), d, store, c.name, c.prefix)
		rejected(t, err, c.msg)
	}
	// 名前の比べ方は完全一致(大文字と小文字は区別する。03「名前の重複」)。
	if _, err := workspaces.Create(t.Context(), d, store, "web", "WEB"); err != nil {
		t.Errorf("a name differing only in case was refused: %v", err)
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/workspaces/`
Expected: FAIL(`undefined: workspaces.Deps`)

- [ ] **Step 3: 実装する**

`internal/app/workspaces/deps.go`:

```go
// Package workspaces は、ワークスペースの設定の操作(仕様 03)を持つ。1 ファイル 1 操作。
package workspaces

import (
	"context"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Deps は、操作が使う口。
type Deps struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
}

// reject は、操作が条件に合わないときの断り。msg は画面に出す文言(仕様 03 の表のとおり)。
func reject(msg string) error { return refusal.New(msg) }

// 仕様 03 の断りの文言。
const (
	msgNameEmpty       = "名前を入力してください"
	msgNameEmptyRename = "名前は空にできません"
	msgNameTaken       = "この Store には同じ名前のワークスペースがあります"
	msgPrefixEmpty     = "接頭辞を入力してください"
	msgPrefixForm      = "接頭辞は英大文字で始まる 10 文字以内の英大文字と数字にしてください"
	msgPrefixTaken     = "この Store には同じ接頭辞のワークスペースがあります"
)

// normalizeName は、保存する名前の形(前後の空白を除く。03「名前の重複」)。
func normalizeName(s string) string { return strings.TrimSpace(s) }

// normalizePrefix は、接頭辞の形(前後の空白を除いて大文字。03「接頭辞」)。
func normalizePrefix(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// me は、store の「あなた」を実行者にする。
func me(ctx context.Context, d Deps, store port.StoreID) (change.Actor, error) {
	login, err := d.Identity.Me(ctx, store)
	if err != nil {
		return change.Actor{}, err
	}
	return change.User(login), nil
}
```

`internal/app/workspaces/create.go`:

```go
package workspaces

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Create は、store にワークスペースを作る(03「ワークスペースの作成」)。初期値は DefaultSettings。
// 検査は 03 の表の順に行い、最初に当たったものの文言で断る。
func Create(ctx context.Context, d Deps, store port.StoreID, name, prefix string) (workspace.Workspace, error) {
	name, prefix = normalizeName(name), normalizePrefix(prefix)
	actor, err := me(ctx, d, store)
	if err != nil {
		return workspace.Workspace{}, err
	}
	w := workspace.Workspace{ID: d.IDs.New(), Name: name, Prefix: prefix}
	op := port.Operation{Actor: actor, Summary: fmt.Sprintf("ワークスペース「%s」を作成", name)}
	err = d.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		others, err := tx.Workspaces().List()
		if err != nil {
			return err
		}
		if err := checkNew(w, others); err != nil {
			return err
		}
		draft, err := workspace.CreateWithSettings(w, workspace.DefaultSettings(d.IDs.New))
		if err != nil {
			return err
		}
		return tx.WriteWorkspaces().Append(w.ID, draft)
	})
	if err != nil {
		return workspace.Workspace{}, err
	}
	return w, nil
}

func checkNew(w workspace.Workspace, others []workspace.Workspace) error {
	switch {
	case w.Name == "":
		return reject(msgNameEmpty)
	case nameTaken(w, others):
		return reject(msgNameTaken)
	case w.Prefix == "":
		return reject(msgPrefixEmpty)
	case !workspace.ValidPrefix(w.Prefix):
		return reject(msgPrefixForm)
	case prefixTaken(w, others):
		return reject(msgPrefixTaken)
	}
	return nil
}

// nameTaken は、w 以外に同じ名前のワークスペースがあるか(完全一致。03「名前の重複」)。
func nameTaken(w workspace.Workspace, others []workspace.Workspace) bool {
	for _, o := range others {
		if o.ID != w.ID && o.Name == w.Name {
			return true
		}
	}
	return false
}

// prefixTaken は、w 以外に同じ接頭辞のワークスペースがあるか。
func prefixTaken(w workspace.Workspace, others []workspace.Workspace) bool {
	for _, o := range others {
		if o.ID != w.ID && o.Prefix == w.Prefix {
			return true
		}
	}
	return false
}
```

`nameTaken`・`prefixTaken` は B1-3 の名前・接頭辞の変更でも使う。1 ファイル 1 操作の規則は「操作」の数で数え、操作どうしで使う小さな検査は、最初に使う操作のファイルに置く。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/app/workspaces/ && make lint && go test ./internal/archtest/`
Expected: PASS、lint 0 件、archtest PASS(`app/workspaces` が `domain` と `app/port` だけを import している)

- [ ] **Step 5: Commit**

```bash
git add internal/app/workspaces
git commit -m "feat(app): ワークスペースの作成を足す"
```

---

### Task B1-3: 名前と接頭辞の変更、旧い接頭辞、一覧と詳細

**Files:**
- Create: `internal/domain/workspace/prefixes.go`
- Test: `internal/domain/workspace/prefixes_test.go`
- Create: `internal/app/workspaces/rename.go`、`internal/app/workspaces/prefix.go`、`internal/app/workspaces/get.go`
- Test: `internal/app/workspaces/rename_test.go`、`internal/app/workspaces/prefix_test.go`、`internal/app/workspaces/get_test.go`

**Interfaces:**
- Consumes: B1-2 の `Deps`・`reject`・`nameTaken`・`prefixTaken`・`me`
- Produces:
  - `workspace.OldPrefixes(log []change.Record) []string`
  - `workspaces.Rename(ctx, d Deps, store port.StoreID, id workspace.ID, name string) error`
  - `workspaces.ChangePrefix(ctx, d Deps, store port.StoreID, id workspace.ID, prefix string) error`
  - `workspaces.Detail{ Workspace workspace.Workspace; Settings workspace.Settings; OldPrefixes []string }`
  - `workspaces.List(ctx, d Deps, store port.StoreID) ([]workspace.Workspace, error)`
  - `workspaces.Get(ctx, d Deps, store port.StoreID, id workspace.ID) (Detail, error)`

**仕様:** 名前は空 →「名前は空にできません」、同名 →「この Store には同じ名前のワークスペースがあります」、同じ名前に変えたときは何もしない(保存しない)。アクティビティの文は「ワークスペースの名前を「X」に変更」。接頭辞は作成と同じ検査と文言、同じ接頭辞のときは何もしない。アクティビティの文は「接頭辞を A から B に変更」。

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/workspace/prefixes_test.go`:

```go
package workspace_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 旧い接頭辞は、ログに出たすべての接頭辞から、いまのものを除いたもの(この計画の決定 8)。
func TestOldPrefixes(t *testing.T) {
	d, err := workspace.Create(workspace.Workspace{ID: "W", Name: "Web", Prefix: "HH"})
	if err != nil {
		t.Fatal(err)
	}
	log := stamp(nil, d, "c", 0)
	if got := workspace.OldPrefixes(log); len(got) != 0 {
		t.Errorf("fresh workspace: %v", got)
	}
	log = stamp(log, change.NewSet("prefix", "HH", "WEB"), "s1", 1)
	log = stamp(log, change.NewSet("prefix", "WEB", "APP"), "s2", 2)
	log = stamp(log, change.NewSet("prefix", "APP", "HH"), "s3", 3)
	log = stamp(log, change.NewSet("prefix", "HH", "bad!"), "s4", 4) // 形の違う行は数えない
	if got := workspace.OldPrefixes(log); !reflect.DeepEqual(got, []string{"APP", "WEB"}) {
		t.Errorf("OldPrefixes = %v", got)
	}
}
```

`internal/app/workspaces/rename_test.go`:

```go
package workspaces_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/change"
)

func TestRename(t *testing.T) {
	d := newDeps(t)
	w, _ := workspaces.Create(t.Context(), d, store, "Web", "HH")
	if _, err := workspaces.Create(t.Context(), d, store, "API", "API"); err != nil {
		t.Fatal(err)
	}
	rejected(t, workspaces.Rename(t.Context(), d, store, w.ID, "  "), "名前は空にできません")
	rejected(t, workspaces.Rename(t.Context(), d, store, w.ID, "API"), "この Store には同じ名前のワークスペースがあります")
	if err := workspaces.Rename(t.Context(), d, store, w.ID, " Web "); err != nil {
		t.Fatal(err)
	}
	if err := workspaces.Rename(t.Context(), d, store, w.ID, "フロント"); err != nil {
		t.Fatal(err)
	}
	err := d.Stores.Read(t.Context(), store, func(r port.Reader) error {
		got, _ := r.Workspaces().Get(w.ID)
		log, _ := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: w.ID})
		// 同じ名前に変えたときは何もしないので、ログは作成と 1 回の変更だけ。
		if got.Name != "フロント" || len(log) != 2 {
			t.Errorf("workspace = %+v, %d rows", got, len(log))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
```

`internal/app/workspaces/prefix_test.go`:

```go
package workspaces_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/workspaces"
)

func TestChangePrefix(t *testing.T) {
	d := newDeps(t)
	w, _ := workspaces.Create(t.Context(), d, store, "Web", "HH")
	if _, err := workspaces.Create(t.Context(), d, store, "API", "API"); err != nil {
		t.Fatal(err)
	}
	rejected(t, workspaces.ChangePrefix(t.Context(), d, store, w.ID, ""), "接頭辞を入力してください")
	rejected(t, workspaces.ChangePrefix(t.Context(), d, store, w.ID, "W-B"), "接頭辞は英大文字で始まる 10 文字以内の英大文字と数字にしてください")
	rejected(t, workspaces.ChangePrefix(t.Context(), d, store, w.ID, "api"), "この Store には同じ接頭辞のワークスペースがあります")
	if err := workspaces.ChangePrefix(t.Context(), d, store, w.ID, "hh"); err != nil {
		t.Fatal(err) // 同じ接頭辞: 何もしない
	}
	if err := workspaces.ChangePrefix(t.Context(), d, store, w.ID, "web"); err != nil {
		t.Fatal(err)
	}
	got, err := workspaces.Get(t.Context(), d, store, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Workspace.Prefix != "WEB" || !reflect.DeepEqual(got.OldPrefixes, []string{"HH"}) {
		t.Errorf("detail = %+v", got)
	}
}
```

`internal/app/workspaces/get_test.go`:

```go
package workspaces_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/workspaces"
)

func TestListAndGet(t *testing.T) {
	d := newDeps(t)
	a, _ := workspaces.Create(t.Context(), d, store, "Web", "HH")
	b, _ := workspaces.Create(t.Context(), d, store, "API", "API")
	list, err := workspaces.List(t.Context(), d, store)
	if err != nil || len(list) != 2 || list[0].ID != a.ID || list[1].ID != b.ID {
		t.Errorf("List = %+v, %v", list, err)
	}
	got, err := workspaces.Get(t.Context(), d, store, a.ID)
	if err != nil || got.Workspace != a || len(got.Settings.Priorities) != 5 || len(got.OldPrefixes) != 0 {
		t.Errorf("Get = %+v, %v", got, err)
	}
	if _, err := workspaces.Get(t.Context(), d, store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/workspace/ ./internal/app/workspaces/`
Expected: FAIL(`undefined: workspace.OldPrefixes`、`undefined: workspaces.Rename` ほか)

- [ ] **Step 3: 実装する**

`internal/domain/workspace/prefixes.go`:

```go
package workspace

import (
	"encoding/json"
	"slices"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// OldPrefixes は、旧い接頭辞(辞書順)。ログの create と set prefix の行に出た、使える形の接頭辞の
// すべてから、いまの接頭辞を除いたもの(保存形式「移動・番号・旧 ID」)。同期で負けた接頭辞も含む。
func OldPrefixes(log []change.Record) []string {
	w, ok := FromLog(log)
	if !ok {
		return nil
	}
	seen := map[string]bool{w.Prefix: true}
	var out []string
	for _, r := range log {
		p := prefixOf(r)
		if ValidPrefix(p) && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

func prefixOf(r change.Record) string {
	switch {
	case r.T == change.Create:
		var v struct{ Prefix string }
		_ = json.Unmarshal(r.To, &v)
		return v.Prefix
	case r.T == change.Set && r.F == "prefix":
		var p string
		_ = json.Unmarshal(r.To, &p)
		return p
	}
	return ""
}
```

`internal/app/workspaces/rename.go`:

```go
package workspaces

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Rename は、ワークスペースの名前を変える(03「一般」の「名前」)。同じ名前なら何もしない。
func Rename(ctx context.Context, d Deps, store port.StoreID, id workspace.ID, name string) error {
	name = normalizeName(name)
	actor, err := me(ctx, d, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: actor, Summary: fmt.Sprintf("ワークスペースの名前を「%s」に変更", name)}
	return d.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		w, err := tx.Workspaces().Get(id)
		if err != nil {
			return err
		}
		others, err := tx.Workspaces().List()
		if err != nil {
			return err
		}
		next := workspace.Workspace{ID: w.ID, Name: name, Prefix: w.Prefix}
		switch {
		case name == "":
			return reject(msgNameEmptyRename)
		case name == w.Name:
			return nil // 何も書かないので、保存もしない
		case nameTaken(next, others):
			return reject(msgNameTaken)
		}
		return tx.WriteWorkspaces().Append(id, change.NewSet("name", w.Name, name))
	})
}
```

`internal/app/workspaces/prefix.go`(アクティビティの文「接頭辞を A から B に変更」の A は、`Write` の前に読んだいまの接頭辞。`Operation` は `Write` の前に決めるため。読んでから書くまでに別のタブが接頭辞を変えたときも、書く値は `Write` の中で読み直したものから決まり、項目ごとの後勝ちに従う):

```go
package workspaces

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ChangePrefix は、接頭辞を変える(03「一般」の「接頭辞」)。番号は変わらない。旧い接頭辞はログに残る。
// 同じ接頭辞なら何もしない。確認ダイアログは画面(B2)が出す。
func ChangePrefix(ctx context.Context, d Deps, store port.StoreID, id workspace.ID, prefix string) error {
	prefix = normalizePrefix(prefix)
	actor, err := me(ctx, d, store)
	if err != nil {
		return err
	}
	var current workspace.Workspace
	if err := d.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		current, err = r.Workspaces().Get(id)
		return err
	}); err != nil {
		return err
	}
	op := port.Operation{Actor: actor, Summary: fmt.Sprintf("接頭辞を %s から %s に変更", current.Prefix, prefix)}
	return d.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		w, err := tx.Workspaces().Get(id)
		if err != nil {
			return err
		}
		others, err := tx.Workspaces().List()
		if err != nil {
			return err
		}
		return applyPrefix(tx, w, prefix, others)
	})
}

func applyPrefix(tx port.Tx, w workspace.Workspace, prefix string, others []workspace.Workspace) error {
	next := workspace.Workspace{ID: w.ID, Name: w.Name, Prefix: prefix}
	switch {
	case prefix == "":
		return reject(msgPrefixEmpty)
	case !workspace.ValidPrefix(prefix):
		return reject(msgPrefixForm)
	case prefix == w.Prefix:
		return nil
	case prefixTaken(next, others):
		return reject(msgPrefixTaken)
	}
	return tx.WriteWorkspaces().Append(w.ID, change.NewSet("prefix", w.Prefix, prefix))
}
```

`internal/app/workspaces/get.go`:

```go
package workspaces

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Detail は、設定の画面が読むワークスペースの中身。
type Detail struct {
	Workspace   workspace.Workspace
	Settings    workspace.Settings
	OldPrefixes []string
}

// List は、store のワークスペースを内部 ID の順に返す。
func List(ctx context.Context, d Deps, store port.StoreID) ([]workspace.Workspace, error) {
	var out []workspace.Workspace
	err := d.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		out, err = r.Workspaces().List()
		return err
	})
	return out, err
}

// Get は、ワークスペースと設定と旧い接頭辞を返す。
func Get(ctx context.Context, d Deps, store port.StoreID, id workspace.ID) (Detail, error) {
	var out Detail
	err := d.Stores.Read(ctx, store, func(r port.Reader) error {
		w, err := r.Workspaces().Get(id)
		if err != nil {
			return err
		}
		s, err := r.Workspaces().Settings(id)
		if err != nil {
			return err
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: id})
		if err != nil {
			return err
		}
		out = Detail{Workspace: w, Settings: s, OldPrefixes: workspace.OldPrefixes(log)}
		return nil
	})
	return out, err
}
```

`List`・`Get` は読むだけの操作で、1 ファイルに 2 つ置く。設計書 4 節の「1 ファイルに 1 操作」は保存する操作に当てる(レビューで確かめる規則)。レビューで分けるよう言われたら `list.go` を分ける。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/domain/workspace/ ./internal/app/workspaces/ && make lint`
Expected: PASS、lint 0 件

- [ ] **Step 5: Commit**

```bash
git add internal/domain/workspace/prefixes.go internal/domain/workspace/prefixes_test.go internal/app/workspaces
git commit -m "feat(app): ワークスペースの名前と接頭辞の変更、旧い接頭辞、一覧と詳細を足す"
```

---

### Task B1-4: ワークスペースの HTTP の API

**Files:**
- Create: `internal/adapter/httpapi/workspaces/workspaces.go`、`internal/adapter/httpapi/workspaces/dto.go`
- Test: `internal/adapter/httpapi/workspaces/workspaces_test.go`
- Modify: `internal/adapter/httpapi/routes.go`(`workspaces.Register` の 1 行)
- Modify(生成物): `api/openapi.yaml`、`web/packages/api/src/generated.ts`

**Interfaces:**
- Consumes: B1-2・B1-3 の `workspaces.*`、A1 Task 1 の `httpapi.Deps.IDs`・`Identity` と `apierr.From`
- Produces(すべて `Authorization: Bearer` が要る):
  - `GET /api/stores/{store}/workspaces` → `200 [{id, name, prefix}]`(operationId `list-workspaces`)
  - `POST /api/stores/{store}/workspaces` 本文 `{name, prefix}` → `201 {id, name, prefix}`(`create-workspace`)
  - `GET /api/stores/{store}/workspaces/{id}` → `200 {id, name, prefix, oldPrefixes, settings}`(`get-workspace`)
  - `PUT /api/stores/{store}/workspaces/{id}/name` 本文 `{name}` → `204`(`rename-workspace`)
  - `PUT /api/stores/{store}/workspaces/{id}/prefix` 本文 `{prefix}` → `204`(`change-workspace-prefix`)
  - 誤り: `apierr.From` で、断り → 422(`detail` が文言)、見つからない → 404
  - `httpapiworkspaces.Register(api huma.API, d workspaces.Deps)`

- [ ] **Step 1: 失敗するテストを書く**

`internal/adapter/httpapi/workspaces/workspaces_test.go`:

```go
package workspaces_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/app/apptest"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	ids := apptest.NewIDs()
	stores := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	stores.AddStore("S1")
	deps := httpapi.Deps{Version: "test", Stores: stores, IDs: ids, Identity: apptest.Identity{Login: "taro"}}
	srv := httptest.NewServer(httpapi.New("tok", fstest.MapFS{}, deps))
	t.Cleanup(srv.Close)
	return srv
}

func call(t *testing.T, srv *httptest.Server, method, path string, body any) (int, string) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequestWithContext(t.Context(), method, srv.URL+path, r)
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestCreateListAndGet(t *testing.T) {
	srv := newServer(t)
	code, body := call(t, srv, http.MethodPost, "/api/stores/S1/workspaces", map[string]string{"name": "Web", "prefix": "hh"})
	if code != http.StatusCreated || !strings.Contains(body, `"prefix":"HH"`) {
		t.Fatalf("create: %d %s", code, body)
	}
	var created struct{ ID string }
	_ = json.Unmarshal([]byte(body), &created)
	code, body = call(t, srv, http.MethodGet, "/api/stores/S1/workspaces", nil)
	if code != http.StatusOK || !strings.Contains(body, `"name":"Web"`) {
		t.Errorf("list: %d %s", code, body)
	}
	code, body = call(t, srv, http.MethodGet, "/api/stores/S1/workspaces/"+created.ID, nil)
	if code != http.StatusOK || !strings.Contains(body, `"initialStatus"`) || !strings.Contains(body, `"name":"InProgress"`) {
		t.Errorf("get: %d %s", code, body)
	}
}

func TestRejectionsAre422WithTheSpecText(t *testing.T) {
	srv := newServer(t)
	call(t, srv, http.MethodPost, "/api/stores/S1/workspaces", map[string]string{"name": "Web", "prefix": "HH"})
	code, body := call(t, srv, http.MethodPost, "/api/stores/S1/workspaces", map[string]string{"name": "Web", "prefix": "WEB"})
	if code != http.StatusUnprocessableEntity || !strings.Contains(body, "この Store には同じ名前のワークスペースがあります") {
		t.Errorf("dup: %d %s", code, body)
	}
}

func TestRenameAndPrefix(t *testing.T) {
	srv := newServer(t)
	_, body := call(t, srv, http.MethodPost, "/api/stores/S1/workspaces", map[string]string{"name": "Web", "prefix": "HH"})
	var w struct{ ID string }
	_ = json.Unmarshal([]byte(body), &w)
	if code, b := call(t, srv, http.MethodPut, "/api/stores/S1/workspaces/"+w.ID+"/name", map[string]string{"name": "フロント"}); code != http.StatusNoContent {
		t.Errorf("rename: %d %s", code, b)
	}
	if code, b := call(t, srv, http.MethodPut, "/api/stores/S1/workspaces/"+w.ID+"/prefix", map[string]string{"prefix": "WEB"}); code != http.StatusNoContent {
		t.Errorf("prefix: %d %s", code, b)
	}
	_, body = call(t, srv, http.MethodGet, "/api/stores/S1/workspaces/"+w.ID, nil)
	if !strings.Contains(body, `"name":"フロント"`) || !strings.Contains(body, `"oldPrefixes":["HH"]`) {
		t.Errorf("after: %s", body)
	}
}

func TestNotFound(t *testing.T) {
	srv := newServer(t)
	if code, _ := call(t, srv, http.MethodGet, "/api/stores/S1/workspaces/01NOPE00000000000000000000", nil); code != http.StatusNotFound {
		t.Errorf("unknown workspace: %d", code)
	}
	if code, _ := call(t, srv, http.MethodGet, "/api/stores/nope/workspaces", nil); code != http.StatusNotFound {
		t.Errorf("unknown store: %d", code)
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/adapter/httpapi/workspaces/`
Expected: FAIL(`/api/stores/S1/workspaces` が 404 で、`create` のテストが落ちる)

- [ ] **Step 3: 実装する**

`internal/adapter/httpapi/workspaces/dto.go`:

```go
package workspaces

import "github.com/kukv/hedgehug/internal/domain/workspace"

// 本文の形。domain の型をそのまま出さず、API の名前(小文字の始まり)にそろえる。

type summary struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
}

type value struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Category string `json:"category,omitempty" doc:"ステータスの分類(todo・doing・done・canceled)"`
}

type property struct {
	ID      string   `json:"id"`
	Key     string   `json:"key"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Builtin bool     `json:"builtin"`
	Kinds   []string `json:"kinds"`
	Hidden  bool     `json:"hidden"`
}

type settings struct {
	Properties       []property        `json:"properties"`
	Statuses         []value           `json:"statuses"`
	Priorities       []value           `json:"priorities"`
	Systems          []value           `json:"systems"`
	InitialStatus    string            `json:"initialStatus"`
	InitialPriority  string            `json:"initialPriority"`
	CategoryDefaults map[string]string `json:"categoryDefaults"`
}

type detail struct {
	summary
	OldPrefixes []string `json:"oldPrefixes"`
	Settings    settings `json:"settings"`
}

func toSummary(w workspace.Workspace) summary {
	return summary{ID: string(w.ID), Name: w.Name, Prefix: w.Prefix}
}

func toValues(vs []workspace.Value) []value {
	out := make([]value, 0, len(vs))
	for _, v := range vs {
		out = append(out, value{ID: string(v.ID), Name: v.Name, Color: string(v.Color), Category: string(v.Category)})
	}
	return out
}

func toSettings(s workspace.Settings) settings {
	out := settings{
		Statuses: toValues(s.Statuses), Priorities: toValues(s.Priorities), Systems: toValues(s.Systems),
		InitialStatus: string(s.InitialStatus), InitialPriority: string(s.InitialPriority),
		CategoryDefaults: map[string]string{}, Properties: []property{},
	}
	for c, id := range s.CategoryDefaults {
		out.CategoryDefaults[string(c)] = string(id)
	}
	for _, p := range s.Properties {
		out.Properties = append(out.Properties, property{
			ID: string(p.ID), Key: p.Key, Name: p.Name, Type: string(p.Type),
			Builtin: p.Builtin, Kinds: p.Kinds, Hidden: p.Hidden,
		})
	}
	return out
}
```

`internal/adapter/httpapi/workspaces/workspaces.go`:

```go
// Package workspaces は、ワークスペースの設定の API(仕様 03)を持つ。
package workspaces

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appws "github.com/kukv/hedgehug/internal/app/workspaces"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

type storePath struct {
	Store string `path:"store"`
}

type workspacePath struct {
	Store string `path:"store"`
	ID    string `path:"id"`
}

// Register は、ワークスペースの API を登録する。
func Register(api huma.API, d appws.Deps) {
	registerRead(api, d)
	registerWrite(api, d)
}

func registerRead(api huma.API, d appws.Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "list-workspaces", Method: http.MethodGet, Path: "/api/stores/{store}/workspaces",
		Summary: "Store のワークスペースの一覧",
	}, func(ctx context.Context, in *storePath) (*struct{ Body []summary }, error) {
		list, err := appws.List(ctx, d, port.StoreID(in.Store))
		if err != nil {
			return nil, apierr.From(err)
		}
		out := &struct{ Body []summary }{Body: []summary{}}
		for _, w := range list {
			out.Body = append(out.Body, toSummary(w))
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "get-workspace", Method: http.MethodGet, Path: "/api/stores/{store}/workspaces/{id}",
		Summary: "ワークスペースと設定",
	}, func(ctx context.Context, in *workspacePath) (*struct{ Body detail }, error) {
		got, err := appws.Get(ctx, d, port.StoreID(in.Store), workspace.ID(in.ID))
		if err != nil {
			return nil, apierr.From(err)
		}
		old := got.OldPrefixes
		if old == nil {
			old = []string{}
		}
		return &struct{ Body detail }{Body: detail{
			summary: toSummary(got.Workspace), OldPrefixes: old, Settings: toSettings(got.Settings),
		}}, nil
	})
}

type createInput struct {
	Store string `path:"store"`
	Body  struct {
		Name   string `json:"name"`
		Prefix string `json:"prefix"`
	}
}

type renameInput struct {
	workspacePath
	Body struct {
		Name string `json:"name"`
	}
}

type prefixInput struct {
	workspacePath
	Body struct {
		Prefix string `json:"prefix"`
	}
}

func registerWrite(api huma.API, d appws.Deps) {
	huma.Register(api, huma.Operation{
		OperationID: "create-workspace", Method: http.MethodPost, Path: "/api/stores/{store}/workspaces",
		Summary: "ワークスペースを作成", DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createInput) (*struct{ Body summary }, error) {
		w, err := appws.Create(ctx, d, port.StoreID(in.Store), in.Body.Name, in.Body.Prefix)
		if err != nil {
			return nil, apierr.From(err)
		}
		return &struct{ Body summary }{Body: toSummary(w)}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "rename-workspace", Method: http.MethodPut, Path: "/api/stores/{store}/workspaces/{id}/name",
		Summary: "ワークスペースの名前を変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *renameInput) (*struct{}, error) {
		return nil, apierr.From(appws.Rename(ctx, d, port.StoreID(in.Store), workspace.ID(in.ID), in.Body.Name))
	})
	huma.Register(api, huma.Operation{
		OperationID: "change-workspace-prefix", Method: http.MethodPut, Path: "/api/stores/{store}/workspaces/{id}/prefix",
		Summary: "接頭辞を変更", DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *prefixInput) (*struct{}, error) {
		return nil, apierr.From(appws.ChangePrefix(ctx, d, port.StoreID(in.Store), workspace.ID(in.ID), in.Body.Prefix))
	})
}
```

huma は、本文の構造体の `json` タグに `omitempty` がないフィールドを必須にする。`createInput.Body` の `name`・`prefix` が空文字のときも 422 の文言を操作から返したいので、huma の検査で先に断られないよう、空文字を受け付ける(`minLength` を付けない)。huma が空文字を断るなら、フィールドに `required:"false"` を付ける。

ハンドラーが `(nil, nil)` を返すと huma は `DefaultStatus` で本文なしの応答を返す。`rename-workspace` と `change-workspace-prefix` は、`apierr.From(nil)` が `nil` を返すのでそうなる。

`internal/adapter/httpapi/routes.go` の `NewAPI` の `system.Register(api, deps.Version)` の下に 1 行足す(import に `httpws "github.com/kukv/hedgehug/internal/adapter/httpapi/workspaces"` と `appws "github.com/kukv/hedgehug/internal/app/workspaces"` を足す):

```go
	httpws.Register(api, appws.Deps{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity})
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/adapter/...`
Expected: PASS

- [ ] **Step 5: 生成物を作り直す**

```bash
make generate
cd web && pnpm gen && cd ..
git diff --stat api/openapi.yaml web/packages/api/src/generated.ts
```

Expected: `api/openapi.yaml` に 5 つの operationId が足され、`generated.ts` に `listWorkspaces`・`createWorkspace`・`getWorkspace`・`renameWorkspace`・`changeWorkspacePrefix` ができる。画面の呼び出し(`web/packages/api/src/workspaces.ts`)は B2 で書く。

- [ ] **Step 6: 全体を確かめる**

Run: `make check`
Expected: すべて成功(生成物が最新、archtest が `httpapi/workspaces` → `app/workspaces` の向きを許し、infra を import していないことを確かめる)

- [ ] **Step 7: Commit**

```bash
git add internal/adapter/httpapi api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(adapter): ワークスペースの作成・名前・接頭辞・一覧・詳細の API を足す"
```

---

## 未決の点(B1 の実装より前に、親とユーザーで決めること)

| # | 点 | 推奨 |
|---|---|---|
| 1 | 画面のアドレス(01「画面のアドレス」)は A2 が入れるが、サイドバー(Store とワークスペースの一覧、ワークスペースのメニュー「設定を開く」「上へ移動」など。03「範囲と画面の構成」・04)の持ち主が A〜G のどこにも決まっていない | A2 でサイドバーの枠とワークスペースの一覧を作り、B2 はワークスペースのメニューの項目を足す |
| 2 | 「あなた」がメンバーにいなければ、次の書き込みと一緒にメンバーへ足す規則(01)は、すべての書き込みに掛かる。どこで行うか | `Identity` の実装を持つトラック G が、Store の追加のときに載せる処理と一緒に決める。それまで A〜D の操作は扱わない(Store の追加が先なので、普段は起きない) |
| 3 | 遷移の制約(B3)と自動化(B4)は、トラック A のステータスを変える操作(A3)から呼ぶ。A3 が B3・B4 より先に入ると、B が A の持ち場の操作を直すことになる | 判定(`workspace.CanTransition`)と実行(`automation.Run`)は domain に関数として置き、A の操作が呼ぶ。A3 は B3 を待ち、自動化の呼び出しは B4 が入ったときに A 側のタスクとして足す |
