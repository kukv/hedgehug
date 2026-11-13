# hedgehug トラック A(アイテムとプロジェクト)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 02 のアイテムとプロジェクトの操作(作成・編集・移動・種類の変更・削除・詳細パネル・プロジェクトの画面・コマンドパレット)を作る。

**Architecture:** 設計書のとおり、規則は `domain/item`・`domain/project` などに、操作は `app/items`・`app/projects` に 1 ファイル 1 操作で置き、`port.Stores` だけを通して保存する。API は `adapter/httpapi/items`・`adapter/httpapi/projects`、画面は `web/packages/core/features/items`・`features/projects` と `web/app/screens/project` に置く。トラックを 5 つの小計画(A1〜A5)に分け、この文書では一覧を全体分、A1 だけをコード入りで書く。A2 以降は、前の小計画が develop に入ってから書く。

**Tech Stack:** 土台と同じ(Go 1.27.3、huma v2、go-git、Svelte 5、orval、Vitest、Playwright)。

**Spec:**
- `docs/superpowers/specs/2026-11-12-hedgehug-architecture-design.md`(1〜3 節・8 節)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`(「ファイル構成」「変更記録」「移動・番号・旧 ID」「削除・復元・完全に削除」「アクティビティと履歴」)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md`(全文。このトラックの正本)
- 関わる節: 01「用語と階層」「「あなた」の決まり方」「書き込みの単位」「閲覧のみの Store」「画面のアドレス」、03「ステータス」「プロジェクト別のルール」「テンプレート」、04「ツリー」「4 つのレイアウトに共通すること」、06「同時に編集したとき」「番号の重なりの修復」「通知」
- 土台の計画: `docs/superpowers/plans/2026-11-12-hedgehug-foundation.md`(型・口・関数名はこれに合わせる)

**持ち場(設計書 8 節):** `internal/domain/item`・`internal/domain/project`・`internal/domain/numbering`・`internal/domain/rank`、`internal/app/items`・`internal/app/projects`、`internal/adapter/httpapi/items`・`internal/adapter/httpapi/projects`、`web/packages/core/src/features/items`・`features/projects`、`web/app/src/screens/project`。

**待つもの:**
- A1: 土台の Task 7(gitstore)と Task 8(httpapi・cmd)が develop に入っていること。
- A2: A1 と、土台の Task 12(画面の app と e2e)。
- A3: A2 と、トラック B のステータス・プロパティ・値の定義(`domain/workspace` に入るもの)。
- A4: A3。
- A5: A4 と、トラック C の同期の口(`Replicator`)。コマンドパレットの「このクエリで」の群はトラック E の検索が入ってから。

## Global Constraints

土台の計画の Global Constraints をすべて引き継ぐ。加えて:

- 利用者に見せる断りの文言は、仕様 02 の表の文言を一字一句そのまま使う。
- 通知は作らない。通知は、トラック C が変更記録から作る(06「通知」)。このトラックは、通知の判定に要る変更記録(担当者・ウォッチ中の人・説明の本文など)を正しく書くことだけを受け持つ。
- アクティビティの 1 行(保存のコミットの件名、`Operation.Summary`)は、仕様 02 の「アクティビティには…と残る」の文にする。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| 1 | 操作を断る理由は `domain/refusal` の `Refusal` で返し、`Error()` が仕様の文言そのものになる。API は 422 で `detail` に文言を入れる。画面は `detail` をそのまま出す | 文言を 1 か所に置き、あとで足す CLI も同じ文言を使える |
| 2 | 「あなた」のログイン名は口 `port.Identity` の `Me(ctx, store)` で得る。偽物は `apptest.Identity{Login}`。本物の実装(アカウントの割り当てとこのマシンのログイン名から決める。01)はトラック G が足す。それまで本物のサーバーは「ログイン名が未設定」のエラーを返す | 01「「あなた」の決まり方」は G の持ち場(08 の設定)にある。A〜D のどれもが保存で使うので、口だけを先に入れる |
| 3 | アイテム・プロジェクトの番号は、`domain/numbering` が、ワークスペースのログの `number` の行(保存形式「移動・番号・旧 ID」)の最大値の次にする。`number` は `change.Type` の新しい値で、`domain/numbering` に置く(`domain/change` は変えない) | 保存形式どおり。`change` は土台の共有部分なので触らない |
| 4 | 兄弟の並び順の値(`rank`)は、`domain/rank` の英数字 62 文字の辞書順の文字列にする。末尾に足すときは、最後の兄弟の値の次にする | 保存形式の例 `rank: "a0V"`。ツリーのドラッグ(トラック E)も同じ関数を使う |
| 5 | プロジェクトに `rank`・`owner`・`status`・`start`・`end` を、アイテムに `watchers` を足す(A1 の共有部分の PR)。ほかの項目(アイテムのステータス・担当者・プロパティ・説明など)は、使う小計画で足す | 02「プロジェクト」の固定の項目と、02「作成したあと」の「ウォッチ中の人は作成者だけ」 |
| 6 | 口に `ProjectReader.List(ws)`(並び順の値、同じなら内部 ID の順)と `ItemReader.List(project)`(番号の順)を足す | プロジェクトの番号・名前の重複・兄弟の末尾を決めるのに要る |
| 7 | API は内部 ID で指す(`/api/stores/{store}/projects/{project}` など)。画面の ID(`HH-12`)とアドレスの解決は A2 で足す | A1 はサーバーだけ |
| 8 | A1 の作成は、種類・プロジェクト・親・タイトルだけを受け取る。ステータス・優先度・プロパティ・テンプレートは A3(トラック B の定義が入ってから)で足す | B を待たずに A1 を始めるため |
| 9 | API の機能のパッケージ名は `adapter/httpapi/projects`・`adapter/httpapi/items` とし、app のパッケージは `appprojects`・`appitems` の別名で import する | 設計書 8 節の持ち場の名前 |

---

## 小計画の一覧

| 小計画 | 範囲(仕様 02 の節) | 待つもの | 完成の条件 |
|---|---|---|---|
| **A1 作成と読み出し(サーバー)** | 「アイテムの種類と親子」の置ける親の規則、「入力の検査」のタイトル・プロジェクト・親、「作成したあと」の番号・並び・ウォッチ中の人、「プロジェクト」の「作成」の検査と番号。API で作成・取得・一覧 | 土台 Task 7・8 | API でプロジェクトとアイテムを作って読め、断りの文言が仕様どおり。照合テストが偽物と gitstore の両方で通る |
| A2 プロジェクトの画面と詳細パネルの骨組み(計画: `2026-11-13-hedgehug-track-a2.md`) | 「プロジェクト」の「画面の見出しと構成」(概要・ツリーのタブ)、「作成」のダイアログ、アイテムの作成ダイアログ(種類・ワークスペース・プロジェクト・親・タイトル・ID の予告・続けて作成・閉じる)、「詳細パネル」の冒頭・「上から順の構成」・「タイトル」・「子と Step」(Step の追加と「上へ」)、閲覧のみの Store の無効化。画面のアドレス(01)のプロジェクトとアイテムの入口 | A1、土台 Task 12 | 3 つの幅の Playwright で、プロジェクトを作り、Epic・Task・Step を作り、詳細パネルでタイトルを変えられる |
| A3 プロパティとステータス(前半 A3a: `2026-11-13-hedgehug-track-a3a.md`。後半 A3b はテンプレートと遷移の制約で、B3 の後に別の計画) | 「プロパティの入力と保存」、「ステータスの変更」、作成ダイアログのプロパティ・テンプレート・「ダイアログの中での連動」・「一緒に作るもの」、Step のチェック。「複製」は A4 へ、作成ダイアログの説明は A4 へ、スプリントはトラック F へ(A3a の決定 4・6) | A2、トラック B のステータス・プロパティ・テンプレートの定義 | 詳細パネルと作成ダイアログで全プロパティを入れられ、遷移の制約の断りが仕様どおり(使わないステータスは B5。A3a の決定 3) |
| A4 編集・移動・削除と詳細パネルの残り | 「同じワークスペースの中での移動」、「種類の変更」、「削除」、「複製」、作成ダイアログの「説明」、詳細パネルの「説明」「Markdown」「メンション」「関連」「作業時間」「コメント」「ウォッチ」「履歴」、メニューの「上へ移動」「下へ移動」、「プロジェクト」の「概要の編集」「リポジトリの追加と削除」「名前の変更・移動・削除」のうち名前の変更と削除 | A3 | 各操作の保存・断り・アクティビティの件名が仕様どおり。説明の同時編集が行ごとに合わさる |
| A5 別のワークスペース・Store への移動とコマンドパレット | 「別のワークスペースと別の Store への移動」の全節、プロジェクトの移動、「コマンドパレット」。F1(スプリントとリリース)が先に develop に入ったので、A5 が合わせる(F1 の計画の F1-23・Task 19): A5a は別のワークスペースへの移動で、移したアイテムにスプリント・リリースがあれば同じ操作で `set sprint null`・`set release null` を書く(02 の 231 行)。A5c はパレットの「作成」の群に「スプリントを作成」「リリースを作成」を、ワークスペースかプロジェクトの画面を開いているときだけ出し、選ぶと開いているワークスペースの「スプリント」(「リリース」)のタブを開いて作成ダイアログを開く(02 の 619・652 行) | A4、トラック C の `Replicator`。パレットの「このクエリで」はトラック E、「作成」のスプリント・リリースはトラック F | 別の Store への移動が 2 回の保存で、失敗したら元のまま。旧 ID から辿れる。パレットの候補の群と上限が仕様どおり |

### 仕様 02 の節と小計画(漏れの確認)

| 仕様 02 の節 | 小計画 |
|---|---|
| この文書の範囲 | —(範囲の説明) |
| アイテムの種類と親子 | A1(置ける親)、A3(スプリント・リリースを持てる種類)、A4(移動の断りの文言) |
| プロパティの種類/入力の方法と保存 | A3 |
| ステータスの変更 | A3 |
| アイテムの作成: 開き方と初期値 | A2(開き方、ワークスペース・プロジェクト・種類・親の初期値)、A3(ステータス・優先度・テンプレートの初期値) |
| アイテムの作成: ダイアログの項目 | A2(種類・ワークスペース・プロジェクト・親・タイトル・ID の予告・続けて作成)、A3(テンプレート・プロパティ・一緒に作るもの)、A4(説明)、トラック F(スプリント) |
| アイテムの作成: ダイアログの中での連動 | A2(種類・ワークスペース・プロジェクトを変えたときの親の選び直し)、A3(テンプレートとプロパティの連動) |
| アイテムの作成: 入力の検査 | A1(タイトル・プロジェクト・親)、A3(日付・ポイント・見積もり時間) |
| アイテムの作成: 作成したあと | A1(番号・並び・ウォッチ中の人・件名)、A2(トースト・ダイアログ・閉じる)、A3(一緒に作るもの) |
| 複製 | A4(A3a の決定 4) |
| 同じワークスペースの中での移動 | A4 |
| 別のワークスペースと別の Store への移動(移す先の選び方・移すもの・値の引き直し・関連・転送記録・旧 ID・保存) | A5 |
| 種類の変更 | A4 |
| 削除 | A4 |
| 詳細パネル(冒頭・上から順の構成・タイトル) | A2 |
| 詳細パネル: 説明・Markdown・メンション | A4 |
| 詳細パネル: 子と Step | A2(一覧・Step の追加・「上へ」)、A3(Step のチェック・完了の数) |
| 詳細パネル: 関連 | A4 |
| 詳細パネル: GitHub | トラック D(区画の枠は A2 が置く) |
| 詳細パネル: 作業時間・コメント・ウォッチ・履歴 | A4 |
| プロジェクト: 冒頭・作成 | A1(検査・番号)、A2(ダイアログ・開く画面) |
| プロジェクト: 画面の見出しと構成 | A2(見出し・概要・ツリーのタブ)。カンバン・テーブル・ガントのタブはトラック E、設定タブのルールの区画はトラック B |
| プロジェクト: 概要の編集 | A4(Epic の一覧の進捗を含む) |
| プロジェクト: リポジトリの追加と削除 | A4 |
| プロジェクト: 名前の変更・移動・削除 | A4(名前の変更・削除)、A5(移動) |
| コマンドパレット | A5 |
| 操作ごとの記録と通知 | 各小計画(変更記録とアクティビティの件名)。通知はトラック C |
| モックからの変更点 | —(経緯の記録) |

---

## A1: 作成と読み出し(サーバー)

**完成の条件:** `make check` が通る。API で、プロジェクトを作って一覧・取得でき、アイテムを Epic → Task → Step と作って取得・一覧できる。番号はワークスペースの連番、並びは親の子の末尾、ウォッチ中の人は作成者だけ。断りの文言が仕様 02 の表どおりで、API は 422 を返す。

**ブランチ:** タスクごとに `origin/develop` から `feat/track-a1-<タスク番号>` を切り、PR の向き先は `develop`。Task 1 は共有部分の変更なので、ほかのタスクより先に develop に入れ、トラック B・C・D に取り込んでもらう(設計書 8 節「共有部分の変更」)。

### A1 のファイルの地図

```
internal/domain/refusal/refusal.go        Refusal(断りの理由)
internal/domain/numbering/numbering.go    number の行と、次の番号
internal/domain/rank/rank.go              並び順の値
internal/domain/project/project.go        (変更)rank・owner・status・start・end
internal/domain/project/check.go          作成の検査
internal/domain/item/item.go              (変更)watchers
internal/domain/item/check.go             タイトルの整え方と作成の検査
internal/app/port/stores.go               (変更)ProjectReader.List・ItemReader.List
internal/app/port/identity.go             Identity
internal/app/apptest/identity.go          Identity の偽物
internal/app/apptest/stores_read.go       (変更)List の偽物
internal/app/apptest/stores_contract.go   (変更)準備のデータ・sameItem・場面の追加
internal/app/apptest/stores_contract_lists.go 一覧の場面
internal/app/apptest/sample.go            (変更)number の行・新しい項目
internal/infra/gitstore/read.go           (変更)List
internal/infra/gitstore/yaml.go           (変更)新しい項目
internal/infra/gitstore/write_test.go     (変更)item.yaml の期待
internal/adapter/httpapi/apierr/apierr.go エラーを HTTP の状態に直す
internal/adapter/httpapi/routes.go        (変更)Deps と Register
cmd/hedgehug/wire.go・wire_e2e.go         (変更)IDs・Identity を差し込む
internal/app/projects/service.go・create.go・get.go・list.go
internal/app/items/service.go・create.go・get.go・list.go
internal/adapter/httpapi/projects/projects.go
internal/adapter/httpapi/items/items.go
api/openapi.yaml(生成物)
```

---

### Task 1: 共有部分の変更(口・中心の型・Identity・断りの型)

ほかのトラックも使う部分だけを、この PR で変える。中身の操作は Task 2 以降。

**Files:**
- Create: `internal/domain/refusal/refusal.go`、`internal/domain/numbering/numbering.go`、`internal/domain/rank/rank.go`
- Test: `internal/domain/refusal/refusal_test.go`、`internal/domain/numbering/numbering_test.go`、`internal/domain/rank/rank_test.go`
- Modify: `internal/domain/project/project.go`(全体を置き換え)、`internal/domain/project/project_test.go`
- Modify: `internal/domain/item/item.go`(`Item` と `Create`・`FromLog`)
- Modify: `internal/app/port/stores.go`、Create: `internal/app/port/identity.go`
- Create: `internal/app/apptest/identity.go`、`internal/app/apptest/stores_contract_lists.go`
- Modify: `internal/app/apptest/stores_read.go`、`stores_contract.go`、`sample.go`
- Modify: `internal/infra/gitstore/read.go`、`yaml.go`、`write_test.go`
- Create: `internal/adapter/httpapi/apierr/apierr.go`、Test: `apierr_test.go`
- Modify: `internal/adapter/httpapi/routes.go`(`Deps`)、`cmd/hedgehug/wire.go`、`cmd/hedgehug/wire_e2e.go`

**Interfaces:**
- Consumes: 土台の `change.*`・`port.*`・`apptest.*`・`gitstore.*`・`httpapi.Deps`
- Produces:
  - `refusal.New(message string) error`、`type refusal.Refusal struct{ Message string }`
  - `numbering.Type change.Type = "number"`、`numbering.Kind`(`Items`・`Projects`)、`numbering.Draft(k Kind, n int) change.Draft`、`numbering.Next(wsLog []change.Record, k Kind) int`
  - `rank.First = "V"`、`rank.After(s string) string`
  - `project.Status`(`Todo`・`InProgress`・`Done`・`Pending`・`Cancel`)、`project.ValidStatus(s Status) bool`、`project.Project{ID; Place; Rank, Title, Owner string; Status Status; Start, End string}`
  - `item.Item` に `Watchers []string`
  - `port.ProjectReader.List(ws workspace.ID) ([]project.Project, error)`、`port.ItemReader.List(p project.ID) ([]item.Item, error)`
  - `port.Identity{ Me(ctx context.Context, store StoreID) (string, error) }`、`apptest.Identity{Login string}`
  - `apierr.From(err error) error`
  - `httpapi.Deps{Version string; Stores port.Stores; IDs port.IDs; Identity port.Identity}`
  - `apptest.Sample` が `number` の行を書く(アイテム 1・2、プロジェクト 1)

- [ ] **Step 1: 失敗するテストを書く(domain)**

`internal/domain/refusal/refusal_test.go`:

```go
package refusal_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

func TestRefusalKeepsTheMessage(t *testing.T) {
	err := fmt.Errorf("create: %w", refusal.New("タイトルを入力してください"))
	var r *refusal.Refusal
	if !errors.As(err, &r) || r.Message != "タイトルを入力してください" {
		t.Errorf("As = %v, %+v", errors.As(err, &r), r)
	}
	if refusal.New("x").Error() != "x" {
		t.Error("Error() is not the message")
	}
}
```

`internal/domain/numbering/numbering_test.go`:

```go
package numbering_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/numbering"
)

func row(id string, d change.Draft) change.Record {
	return change.Record{ID: change.ID(id), At: time.Unix(0, 0), By: "t", Actor: change.User("t"), T: d.T, F: d.F, To: d.To}
}

func TestNextIsMaxPlusOne(t *testing.T) {
	log := []change.Record{
		row("c", change.NewCreate(map[string]any{"id": "W"})),
		row("a", numbering.Draft(numbering.Items, 3)),
		row("b", numbering.Draft(numbering.Items, 7)),
		row("p", numbering.Draft(numbering.Projects, 2)),
		row("x", change.Draft{T: numbering.Type, F: "items", To: json.RawMessage(`"bad"`)}),
	}
	if got := numbering.Next(log, numbering.Items); got != 8 {
		t.Errorf("Next(items) = %d, want 8", got)
	}
	if got := numbering.Next(log, numbering.Projects); got != 3 {
		t.Errorf("Next(projects) = %d, want 3", got)
	}
	if got := numbering.Next(nil, numbering.Items); got != 1 {
		t.Errorf("Next(empty) = %d, want 1", got)
	}
}
```

`internal/domain/rank/rank_test.go`:

```go
package rank_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/rank"
)

func TestAfter(t *testing.T) {
	for in, want := range map[string]string{"": "V", "V": "W", "a0V": "a0W", "9": "A", "Z": "a", "z": "zV", "azz": "azzV"} {
		if got := rank.After(in); got != want {
			t.Errorf("After(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAfterIsAlwaysLater(t *testing.T) {
	s := ""
	for range 500 {
		next := rank.After(s)
		if next <= s {
			t.Fatalf("After(%q) = %q is not later", s, next)
		}
		s = next
	}
}
```

`internal/domain/project/project_test.go` を次に置き換える:

```go
package project_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

func TestCreateAndFromLog(t *testing.T) {
	p := project.Project{
		ID: "P", Place: project.Place{Workspace: "W", Number: 1}, Rank: "V", Title: "ログイン",
		Owner: "taro", Status: project.InProgress, Start: "2026-11-01", End: "2026-11-30",
	}
	log := change.Stamp(nil, []change.Draft{project.Create(p)}, change.Meta{Op: "o", At: time.Unix(0, 0), Actor: change.User("taro")}, func() change.ID { return "c" })
	got, ok := project.FromLog(log)
	if !ok || got != p {
		t.Errorf("FromLog = %+v, %v", got, ok)
	}
	if got.DisplayID("HH") != "HH-P1" {
		t.Errorf("DisplayID = %s", got.DisplayID("HH"))
	}
}

func TestValidStatus(t *testing.T) {
	for _, s := range []project.Status{project.Todo, project.InProgress, project.Done, project.Pending, project.Cancel} {
		if !project.ValidStatus(s) {
			t.Errorf("%s is not valid", s)
		}
	}
	if project.ValidStatus("Review") {
		t.Error("Review is valid")
	}
}
```

`internal/domain/item/item_test.go` の `TestCreateAndFromLog` の `it` に `Watchers: []string{"hanako"},` を足し、最後に次を足す:

```go
	if len(got.Watchers) != 1 || got.Watchers[0] != "hanako" {
		t.Errorf("Watchers = %v", got.Watchers)
	}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/...`
Expected: FAIL(`undefined: refusal.New`・`numbering.Draft`・`rank.After`・`project.InProgress`・`Watchers` ほか)

- [ ] **Step 3: domain を実装する**

`internal/domain/refusal/refusal.go`:

```go
// Package refusal は、操作を断る理由を持つ。Message は利用者に見せる文言で、仕様の文言のとおりに書く。
package refusal

// Refusal は、操作を断る理由。
type Refusal struct {
	Message string
}

func (r *Refusal) Error() string { return r.Message }

// New は、message で断るエラーを返す。
func New(message string) error { return &Refusal{Message: message} }
```

`internal/domain/numbering/numbering.go`:

```go
// Package numbering は、ワークスペースの連番(アイテム用・プロジェクト用)を持つ。
// 番号を振るたびに、ワークスペースのログに number の行を書き、使った最大番号はその行の最大値で決める
// (保存形式「移動・番号・旧 ID」)。同時に振っても小さくならず、番号は使い回さない。
package numbering

import (
	"encoding/json"

	"github.com/kukv/hedgehug/internal/domain/change"
)

// Type は、番号を使った行の種類。
const Type change.Type = "number"

// Kind は、どの連番か。行の f に書く。
type Kind string

const (
	Items    Kind = "items"
	Projects Kind = "projects"
)

// Draft は、番号 n を使った行。
func Draft(k Kind, n int) change.Draft {
	return change.Draft{T: Type, F: string(k), To: change.Value(n)}
}

// Next は、ワークスペースのログから、次に振る番号を返す。
func Next(wsLog []change.Record, k Kind) int {
	highest := 0
	for _, r := range wsLog {
		if r.T != Type || r.F != string(k) {
			continue
		}
		var n int
		if json.Unmarshal(r.To, &n) == nil && n > highest {
			highest = n
		}
	}
	return highest + 1
}
```

`internal/domain/rank/rank.go`:

```go
// Package rank は、兄弟の並び順の値を持つ。値は英数字 62 文字の文字列で、辞書順に並ぶ。
package rank

import "strings"

const digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// First は、兄弟がないときの値。
const First = "V"

// After は、s よりあとに並ぶ値を返す。s が空なら First。最後の文字を 1 つ進め、進められなければ First を足す。
func After(s string) string {
	if s == "" {
		return First
	}
	i := strings.IndexByte(digits, s[len(s)-1])
	if i >= 0 && i < len(digits)-1 {
		return s[:len(s)-1] + string(digits[i+1])
	}
	return s + First
}
```

`internal/domain/project/project.go` を次に置き換える:

```go
// Package project は、プロジェクト(Epic をまとめる単位)を持つ。
package project

import (
	"encoding/json"
	"strconv"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// ID はプロジェクトの内部 ID。
type ID = change.ID

// Status は、プロジェクトのステータス。5 つに固定で、利用者は変えられない(01・02)。
type Status string

const (
	Todo       Status = "Todo"
	InProgress Status = "InProgress"
	Done       Status = "Done"
	Pending    Status = "Pending"
	Cancel     Status = "Cancel"
)

// ValidStatus は、5 つのどれかか。
func ValidStatus(s Status) bool {
	switch s {
	case Todo, InProgress, Done, Pending, Cancel:
		return true
	}
	return false
}

// Place は、プロジェクトの場所(ワークスペースと番号)。1 つの項目として変える。
type Place struct {
	Workspace workspace.ID `json:"workspace"`
	Number    int          `json:"number"`
}

// Project は、プロジェクトの固定の項目(02「プロジェクト」)。説明はトラック A の A4 で足す。
type Project struct {
	ID     ID
	Place  Place
	Rank   string
	Title  string
	Owner  string // ログイン名。空は「なし」
	Status Status
	Start  string // YYYY-MM-DD。空は「なし」
	End    string
}

// DisplayID は、画面の ID(HH-P1)。
func (p Project) DisplayID(prefix string) string {
	return prefix + "-P" + strconv.Itoa(p.Place.Number)
}

// Create は、プロジェクトの作成の変更。
func Create(p Project) change.Draft {
	return change.NewCreate(map[string]any{
		"id": p.ID, "place": p.Place, "rank": p.Rank, "title": p.Title,
		"owner": p.Owner, "status": p.Status, "start": p.Start, "end": p.End,
	})
}

// FromLog は、プロジェクトのログからいまの値を作る。作成の行がなければ false。
func FromLog(log []change.Record) (Project, bool) {
	s := change.Resolve(log, change.Options{})
	if s.Created == nil {
		return Project{}, false
	}
	var p Project
	for f, dst := range map[string]any{
		"id": &p.ID, "place": &p.Place, "rank": &p.Rank, "title": &p.Title,
		"owner": &p.Owner, "status": &p.Status, "start": &p.Start, "end": &p.End,
	} {
		if v, ok := s.Values[f]; ok {
			_ = json.Unmarshal(v, dst)
		}
	}
	return p, true
}
```

`internal/domain/item/item.go` の `Item` の末尾に `Watchers []string // ウォッチ中の人のログイン名` を足す。`Create` の `map` に `"watchers": it.Watchers` を足す。`FromLog` を次にする(`Resolve` に `opts` を渡し、`watchers` を読む):

```go
var opts = change.Options{Multi: func(f string) bool { return f == "watchers" }}

// FromLog は、アイテムのログからいまの値を作る。作成の行がなければ false。
func FromLog(log []change.Record) (Item, bool) {
	s := change.Resolve(log, opts)
	if s.Created == nil {
		return Item{}, false
	}
	it := Item{Created: Stamp{At: s.Created.At, By: s.Created.By}}
	_ = json.Unmarshal(s.Values["id"], &it.ID)
	_ = json.Unmarshal(s.Values["place"], &it.Place)
	_ = json.Unmarshal(s.Values["kind"], &it.Kind)
	_ = json.Unmarshal(s.Values["rank"], &it.Rank)
	_ = json.Unmarshal(s.Values["title"], &it.Title)
	for _, raw := range s.Lists["watchers"] {
		var login string
		if json.Unmarshal(raw, &login) == nil {
			it.Watchers = append(it.Watchers, login)
		}
	}
	return it, true
}
```

- [ ] **Step 4: domain のテストが通ることを確かめる**

Run: `go test ./internal/domain/...`
Expected: PASS

- [ ] **Step 5: 口と偽物・照合テストを変える**

`internal/app/port/stores.go` の `ProjectReader`・`ItemReader` を次にする:

```go
type ProjectReader interface {
	Get(id project.ID) (project.Project, error)
	// List は、ワークスペースのプロジェクトを、並び順の値の順(同じなら内部 ID の順)で返す。
	List(ws workspace.ID) ([]project.Project, error)
}

type ItemReader interface {
	Get(id item.ID) (item.Item, error)
	// List は、プロジェクトのすべてのアイテムを、番号の順で返す。
	List(p project.ID) ([]item.Item, error)
}
```

`internal/app/port/identity.go`:

```go
package port

import "context"

// Identity は、Store ごとの「あなた」のログイン名(01「「あなた」の決まり方」)。
// 本物の実装はトラック G が足す。
type Identity interface {
	Me(ctx context.Context, store StoreID) (string, error)
}
```

`internal/app/apptest/identity.go`:

```go
package apptest

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Identity は、どの Store でも Login を「あなた」とする偽物。
type Identity struct {
	Login string
}

func (i Identity) Me(context.Context, port.StoreID) (string, error) { return i.Login, nil }
```

`internal/app/apptest/stores_read.go` の `projects` と `items` に次を足す(import に `cmp` を足す):

```go
func (r projects) List(ws workspace.ID) ([]project.Project, error) {
	var out []project.Project
	for _, id := range r.v.ids(change.KindProject) {
		if p, err := r.Get(id); err == nil && p.Place.Workspace == ws {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b project.Project) int {
		return cmp.Or(strings.Compare(a.Rank, b.Rank), strings.Compare(string(a.ID), string(b.ID)))
	})
	return out, nil
}

func (r items) List(p project.ID) ([]item.Item, error) {
	var out []item.Item
	for _, id := range r.v.ids(change.KindItem) {
		if it, err := r.Get(id); err == nil && it.Place.Project == p {
			out = append(out, it)
		}
	}
	slices.SortFunc(out, func(a, b item.Item) int { return cmp.Compare(a.Place.Number, b.Place.Number) })
	return out, nil
}
```

`internal/app/apptest/stores_contract.go` を次のとおり変える。
1. `cProject` に `Rank: "V", Status: project.Todo` を、`cItem` に `Watchers: []string{"taro"}` を足す。
2. `sameItem` を次に置き換え、import に `reflect` を足す(`Item` が `[]string` を持ち、`==` で比べられなくなるため):
   ```go
   func sameItem(a, b item.Item) bool {
   	a.Created, b.Created = item.Stamp{}, item.Stamp{}
   	return reflect.DeepEqual(a, b)
   }
   ```
3. `RunStoresContract` の `cases` の末尾に `{"projects and items are listed", contractLists},` を足す。

`internal/app/apptest/stores_contract_lists.go`:

```go
package apptest

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

func contractLists(t *testing.T, s port.Stores, id port.StoreID) {
	early := project.Project{ID: "01CONTRACTPR00000000000000", Place: project.Place{Workspace: cWorkspace.ID, Number: 2}, Rank: "A", Title: "先頭", Status: project.Todo}
	task := item.Item{ID: "01CONTRACTIT00000000000002", Place: item.Place{Workspace: cWorkspace.ID, Project: cProject.ID, Number: 3}, Kind: item.Epic, Rank: "W", Title: "あと"}
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if err := tx.WriteProjects().Append(early.ID, project.Create(early)); err != nil {
			return err
		}
		return tx.WriteItems().Append(task.ID, item.Create(task))
	})
	mustRead(t, s, id, func(r port.Reader) error {
		ps, err := r.Projects().List(cWorkspace.ID)
		if err != nil || len(ps) != 2 || ps[0] != early || ps[1] != cProject {
			t.Errorf("Projects().List = %+v, %v", ps, err)
		}
		its, err := r.Items().List(cProject.ID)
		if err != nil || len(its) != 2 || !sameItem(its[0], cItem) || !sameItem(its[1], task) {
			t.Errorf("Items().List = %+v, %v", its, err)
		}
		if none, err := r.Items().List("01NOPE00000000000000000000"); err != nil || len(none) != 0 {
			t.Errorf("Items().List(unknown) = %+v, %v", none, err)
		}
		return nil
	})
}
```

`internal/app/apptest/sample.go` を次のとおり変える(import に `numbering` を足す)。
1. `pr` に `Rank: "V", Owner: "taro", Status: project.Todo` を足す。
2. `epic` と `task` に `Watchers: []string{"taro"}` を足し、`Rank` を `rank.First`(`"V"`)にする(import に `rank` を足す)。
3. ワークスペースへの書き込みを、番号の行を含む次にする:
   ```go
   tx.WriteWorkspaces().Append(ws.ID, d,
   	numbering.Draft(numbering.Projects, 1),
   	numbering.Draft(numbering.Items, 1),
   	numbering.Draft(numbering.Items, 2)),
   ```

- [ ] **Step 6: gitstore の読み書きを変える**

`internal/infra/gitstore/read.go` の `projects` と `items` に、`apptest/stores_read.go` と同じ `List` を足す。違いは `ids` がエラーを返すことだけ(import に `cmp`・`strings` を足す):

```go
func (r projects) List(ws workspace.ID) ([]project.Project, error) {
	ids, err := r.v.ids(change.KindProject)
	if err != nil {
		return nil, err
	}
	var out []project.Project
	for _, id := range ids {
		if p, err := r.Get(id); err == nil && p.Place.Workspace == ws {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b project.Project) int {
		return cmp.Or(strings.Compare(a.Rank, b.Rank), strings.Compare(string(a.ID), string(b.ID)))
	})
	return out, nil
}

func (r items) List(p project.ID) ([]item.Item, error) {
	ids, err := r.v.ids(change.KindItem)
	if err != nil {
		return nil, err
	}
	var out []item.Item
	for _, id := range ids {
		if it, err := r.Get(id); err == nil && it.Place.Project == p {
			out = append(out, it)
		}
	}
	slices.SortFunc(out, func(a, b item.Item) int { return cmp.Compare(a.Place.Number, b.Place.Number) })
	return out, nil
}
```

`internal/infra/gitstore/yaml.go` を次のとおり変える。
1. `projectYAML` を次にし、`renderYAML` の `KindProject` の行で新しい項目も入れる:
   ```go
   type projectYAML struct {
   	ID        string `yaml:"id"`
   	Workspace string `yaml:"workspace"`
   	Number    int    `yaml:"number"`
   	Rank      string `yaml:"rank"`
   	Title     string `yaml:"title"`
   	Owner     string `yaml:"owner"`
   	Status    string `yaml:"status"`
   	Start     string `yaml:"start"`
   	End       string `yaml:"end"`
   }
   ```
   ```go
   v = projectYAML{
   	ID: string(p.ID), Workspace: string(p.Place.Workspace), Number: p.Place.Number, Rank: p.Rank,
   	Title: p.Title, Owner: p.Owner, Status: string(p.Status), Start: p.Start, End: p.End,
   }
   ```
2. `itemYAML` の `Created` のあとに `Watchers []string \`yaml:"watchers,flow"\`` を足し、`itemToYAML` で `Watchers: it.Watchers` を入れる。

`internal/infra/gitstore/write_test.go` の `TestYAMLIsRebuiltFromLog` の `want` を、Sample の変更に合わせて次にする(`rank` が `V`、末尾に `watchers`):

```go
	want := `id: 01SAMPLEIT0000000000000002
workspace: 01SAMPLEWS0000000000000001
project: 01SAMPLEPR0000000000000001
parent: 01SAMPLEIT0000000000000001
number: 2
kind: Task
rank: V
title: ログイン画面
created: {at: "2026-11-12T09:00:00.000+09:00", by: taro}
watchers: [taro]
`
```

(土台の Task 7 で `at` の引用符を書き出しに合わせて直していたら、そのままにする。)

- [ ] **Step 7: API のエラーの直し方と Deps を変える**

`internal/adapter/httpapi/apierr/apierr.go`:

```go
// Package apierr は、usecase のエラーを HTTP の状態に直す。機能の API のパッケージが使う。
package apierr

import (
	"errors"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// From は、断り(refusal)を 422(detail に仕様の文言)、見つからないものを 404 にする。
// ほかのエラーはそのまま返し、huma が 500 にする。
func From(err error) error {
	var r *refusal.Refusal
	switch {
	case err == nil:
		return nil
	case errors.As(err, &r):
		return huma.Error422UnprocessableEntity(r.Message)
	case errors.Is(err, port.ErrNotFound), errors.Is(err, port.ErrStoreNotFound):
		return huma.Error404NotFound("not found")
	}
	return err
}
```

`internal/adapter/httpapi/apierr/apierr_test.go`:

```go
package apierr_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

func TestFrom(t *testing.T) {
	var se huma.StatusError
	if err := apierr.From(fmt.Errorf("x: %w", refusal.New("タイトルを入力してください"))); !errors.As(err, &se) || se.GetStatus() != http.StatusUnprocessableEntity || se.Error() != "タイトルを入力してください" {
		t.Errorf("refusal: %v", err)
	}
	if err := apierr.From(port.ErrStoreNotFound); !errors.As(err, &se) || se.GetStatus() != http.StatusNotFound {
		t.Errorf("not found: %v", err)
	}
	boom := errors.New("boom")
	if err := apierr.From(boom); err != boom {
		t.Errorf("other: %v", err)
	}
	if apierr.From(nil) != nil {
		t.Error("nil")
	}
}
```

`internal/adapter/httpapi/routes.go` の `Deps` を次にする:

```go
// Deps は、API が使う口。トラックが、自分の API に要る口を足す。
type Deps struct {
	Version  string
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
}
```

`cmd/hedgehug/wire.go` の `wire` を次にし、同じファイルに `noIdentity` を足す(import に `errors`・`port` を足す):

```go
func wire(context.Context) (httpapi.Deps, error) {
	stores := gitstore.New(sysclock.Clock{}, sysclock.IDs{})
	return httpapi.Deps{Stores: stores, IDs: sysclock.IDs{}, Identity: noIdentity{}}, nil
}

// noIdentity は、「あなた」を決める仕組み(トラック G)が入るまでの仮。保存する操作は断る。
type noIdentity struct{}

func (noIdentity) Me(context.Context, port.StoreID) (string, error) {
	return "", errors.New("このマシンのログイン名が未設定です")
}
```

`cmd/hedgehug/wire_e2e.go` の `wire` で、採番を `ids := apptest.NewIDs()` に取り出して `apptest.NewStores(…, ids)` に渡し、`httpapi.Deps{Stores: stores, IDs: ids, Identity: apptest.Identity{Login: "taro"}}` を返す。

- [ ] **Step 8: 全体が通ることを確かめる**

Run: `make check`
Expected: すべて成功(照合テストの「projects and items are listed」が、偽物と gitstore の両方で通る)

- [ ] **Step 9: Commit と PR**

```bash
git add internal cmd
git commit -m "feat: トラック A の共有部分(一覧の口・Identity・断りの型・番号と並び順・プロジェクトの項目)を足す"
```

PR の説明に「共有部分の変更。トラック B・C・D は、これが develop に入ったら rebase する」と書く。

---

### Task 2: アイテムとプロジェクトの作成の検査(domain)

**Files:**
- Create: `internal/domain/item/check.go`、`internal/domain/project/check.go`
- Test: `internal/domain/item/check_test.go`、`internal/domain/project/check_test.go`

**Interfaces:**
- Consumes: Task 1 の `refusal.New`、`item.CanPlace`(土台)
- Produces:
  - `item.NormalizeTitle(s string) string`
  - `item.CheckCreate(kind Kind, title string, projectChosen bool, parent *Kind) error`
  - `item.PlaceMessage(child Kind, parent *Kind) string`
  - `project.CheckCreate(title, start, end string, siblings []Project) error`

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/item/check_test.go`:

```go
package item_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	if err != nil {
		return "unexpected: " + err.Error()
	}
	return ""
}

// 02「入力の検査」: 連続する空白と改行を 1 つの空白にまとめ、前後の空白を取り除く。
func TestNormalizeTitle(t *testing.T) {
	for in, want := range map[string]string{
		"ログイン画面": "ログイン画面", "  ログイン\n\n 画面  ": "ログイン 画面", " \t\n ": "", "a　　b": "a b",
	} {
		if got := item.NormalizeTitle(in); got != want {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

// 02「入力の検査」の表を上から順に検査し、最初の 1 つだけを返す。
func TestCheckCreate(t *testing.T) {
	epic, task := item.Epic, item.Task
	cases := []struct {
		name    string
		kind    item.Kind
		title   string
		project bool
		parent  *item.Kind
		want    string
	}{
		{"ok epic", item.Epic, "認証", true, nil, ""},
		{"ok task", item.Task, "画面", true, &epic, ""},
		{"ok step", item.Step, "下書き", true, &task, ""},
		{"empty title comes first", item.Task, "  \n ", false, nil, "タイトルを入力してください"},
		{"no project", item.Task, "画面", false, nil, "プロジェクトを選んでください"},
		{"step without parent", item.Step, "下書き", true, nil, "親になる Story・Task・Bug を選んでください"},
		{"task without parent", item.Task, "画面", true, nil, "親になる Epic を選んでください"},
		{"step under epic", item.Step, "下書き", true, &epic, "Step は Epic の下に置けません"},
		{"epic under task", item.Epic, "認証", true, &task, "Epic は Task の下に置けません"},
	}
	for _, c := range cases {
		if got := message(item.CheckCreate(c.kind, c.title, c.project, c.parent)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPlaceMessageAtProjectRoot(t *testing.T) {
	if got := item.PlaceMessage(item.Task, nil); got != "プロジェクトの直下には Epic だけを置けます" {
		t.Errorf("PlaceMessage = %q", got)
	}
}
```

`internal/domain/project/check_test.go`:

```go
package project_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

// 02「プロジェクト」の「作成」の表。名前は前後の空白を除き、完全に一致するものだけを重複とする。
func TestCheckCreate(t *testing.T) {
	siblings := []project.Project{{Title: "ログイン"}}
	cases := []struct {
		name, title, start, end, want string
	}{
		{"ok", "決済", "", "", ""},
		{"same day", "決済", "2026-11-01", "2026-11-01", ""},
		{"only start", "決済", "2026-11-01", "", ""},
		{"empty", "  ", "", "", "タイトルを入力してください"},
		{"end before start", "決済", "2026-11-02", "2026-11-01", "終了日は開始日以降にしてください"},
		{"duplicate after trim", " ログイン ", "", "", "このワークスペースには同じ名前のプロジェクトがあります"},
		{"case differs", "ログイン画面", "", "", ""},
	}
	for _, c := range cases {
		if got := message(project.CheckCreate(c.title, c.start, c.end, siblings)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	if got := message(project.CheckCreate("Login", "", "", []project.Project{{Title: "login"}})); got != "" {
		t.Errorf("upper and lower case must differ: %q", got)
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/item/ ./internal/domain/project/`
Expected: FAIL(`undefined: item.NormalizeTitle` ほか)

- [ ] **Step 3: 実装する**

`internal/domain/item/check.go`:

```go
package item

import (
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// NormalizeTitle は、連続する空白と改行を 1 つの空白にまとめ、前後の空白を取り除く(02「入力の検査」)。
// 作成・詳細パネル・テンプレートから作る子・複製のすべての経路で使う。
func NormalizeTitle(s string) string { return strings.Join(strings.Fields(s), " ") }

// PlaceMessage は、置けない場所へ置こうとしたときの文言(02「アイテムの種類と親子」)。
func PlaceMessage(child Kind, parent *Kind) string {
	if parent == nil {
		return "プロジェクトの直下には Epic だけを置けます"
	}
	return fmt.Sprintf("%s は %s の下に置けません", child, *parent)
}

// CheckCreate は、作成の検査を 02「入力の検査」の表の順に行い、最初に合わなかった 1 つを返す。
// parent は選んだ親の種類で、選んでいなければ nil。日付・ポイント・見積もり時間の検査は A3 で足す。
func CheckCreate(kind Kind, title string, projectChosen bool, parent *Kind) error {
	if NormalizeTitle(title) == "" {
		return refusal.New("タイトルを入力してください")
	}
	if !projectChosen {
		return refusal.New("プロジェクトを選んでください")
	}
	if kind != Epic && parent == nil {
		if kind == Step {
			return refusal.New("親になる Story・Task・Bug を選んでください")
		}
		return refusal.New("親になる Epic を選んでください")
	}
	if !CanPlace(kind, parent) {
		return refusal.New(PlaceMessage(kind, parent))
	}
	return nil
}
```

`internal/domain/project/check.go`:

```go
package project

import (
	"strings"

	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// CheckCreate は、プロジェクトの作成の検査(02「プロジェクト」の「作成」の表の順)。
// siblings は同じワークスペースのプロジェクト(削除済みを除く)。名前は前後の空白を除いて完全一致で比べる。
func CheckCreate(title, start, end string, siblings []Project) error {
	t := strings.TrimSpace(title)
	if t == "" {
		return refusal.New("タイトルを入力してください")
	}
	if start != "" && end != "" && end < start {
		return refusal.New("終了日は開始日以降にしてください")
	}
	for _, s := range siblings {
		if s.Title == t {
			return refusal.New("このワークスペースには同じ名前のプロジェクトがあります")
		}
	}
	return nil
}
```

日付は `YYYY-MM-DD` の形で来る(API の型で確かめる。Task 5)ので、文字列の比較で前後が分かる。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/... && make lint`
Expected: PASS、lint 0 件

- [ ] **Step 5: Commit**

```bash
git add internal/domain/item internal/domain/project
git commit -m "feat(domain): アイテムとプロジェクトの作成の検査を足す"
```

---

### Task 3: プロジェクトの作成・取得・一覧(app/projects)

**Files:**
- Create: `internal/app/projects/service.go`、`create.go`、`get.go`、`list.go`
- Test: `internal/app/projects/projects_test.go`

**Interfaces:**
- Consumes: Task 1 の `port.Identity`・`ProjectReader.List`・`numbering`・`rank`、Task 2 の `project.CheckCreate`
- Produces:
  - `projects.Service{Stores port.Stores; IDs port.IDs; Identity port.Identity}`
  - `projects.CreateInput{Store port.StoreID; Workspace workspace.ID; Title, Owner string; Status project.Status; Start, End string}`
  - `(Service) Create(ctx, CreateInput) (project.Project, error)`
  - `(Service) Get(ctx, store port.StoreID, id project.ID) (project.Project, error)`
  - `(Service) List(ctx, store port.StoreID, ws workspace.ID) ([]project.Project, error)`

**決め方:** 番号はワークスペースのプロジェクト用の連番の次(番号の行も同じ保存で書く)。並びはワークスペースの中の末尾(02「作成」)。ステータスが空なら Todo。保存の件名は「プロジェクト「{タイトル}」を作成」。タイトルは前後の空白を除いて保存する。

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/projects/projects_test.go`:

```go
package projects_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/projects"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const store port.StoreID = "S1"

func setup(t *testing.T) projects.Service {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	return projects.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "hanako"}}
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func TestCreateNumbersAndPlacesAtTheEnd(t *testing.T) {
	svc := setup(t)
	p, err := svc.Create(t.Context(), projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: "  決済 ", Owner: "taro", Start: "2026-11-01", End: "2026-11-30"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Place.Number != 2 || p.DisplayID("HH") != "HH-P2" || p.Title != "決済" || p.Status != project.Todo || p.Rank != "W" {
		t.Errorf("created %+v", p)
	}
	list, err := svc.List(t.Context(), store, apptest.SampleWorkspace)
	if err != nil || len(list) != 2 || list[1] != p {
		t.Errorf("List = %+v, %v", list, err)
	}
	got, err := svc.Get(t.Context(), store, p.ID)
	if err != nil || got != p {
		t.Errorf("Get = %+v, %v", got, err)
	}
}

func TestCreateWritesOneSaveByMe(t *testing.T) {
	svc := setup(t)
	p, err := svc.Create(t.Context(), projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: "決済"})
	if err != nil {
		t.Fatal(err)
	}
	err = svc.Stores.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindProject, ID: p.ID})
		if err != nil || len(log) != 1 || log[0].By != "hanako" {
			t.Errorf("project log = %+v, %v", log, err)
		}
		ws, _ := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
		last := ws[len(ws)-1]
		if last.T != "number" || last.F != "projects" || string(last.To) != "2" || last.Op != log[0].Op {
			t.Errorf("number row = %+v", last)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateRefuses(t *testing.T) {
	svc := setup(t)
	cases := []struct {
		in   projects.CreateInput
		want string
	}{
		{projects.CreateInput{Title: ""}, "タイトルを入力してください"},
		{projects.CreateInput{Title: "決済", Start: "2026-11-02", End: "2026-11-01"}, "終了日は開始日以降にしてください"},
		{projects.CreateInput{Title: "ログイン"}, "このワークスペースには同じ名前のプロジェクトがあります"},
	}
	for _, c := range cases {
		c.in.Store, c.in.Workspace = store, apptest.SampleWorkspace
		if _, err := svc.Create(t.Context(), c.in); message(err) != c.want {
			t.Errorf("%+v: %v, want %q", c.in, err, c.want)
		}
	}
	list, _ := svc.List(t.Context(), store, apptest.SampleWorkspace)
	if len(list) != 1 {
		t.Errorf("a refused create saved something: %+v", list)
	}
}

func TestCreateInUnknownWorkspace(t *testing.T) {
	svc := setup(t)
	_, err := svc.Create(t.Context(), projects.CreateInput{Store: store, Workspace: "01NOPE00000000000000000000", Title: "決済"})
	if !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/projects/`
Expected: FAIL(`no non-test Go files` / `undefined: projects.Service`)

- [ ] **Step 3: 実装する**

`internal/app/projects/service.go`:

```go
// Package projects は、プロジェクトの操作(02「プロジェクト」)を持つ。1 ファイル 1 操作。
package projects

import "github.com/kukv/hedgehug/internal/app/port"

// Service は、プロジェクトの操作が使う口。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
}
```

`internal/app/projects/create.go`:

```go
package projects

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// CreateInput は、プロジェクトの作成ダイアログの入力。Status が空なら Todo。
type CreateInput struct {
	Store     port.StoreID
	Workspace workspace.ID
	Title     string
	Owner     string
	Status    project.Status
	Start     string
	End       string
}

// Create は、プロジェクトを作る。番号はワークスペースの連番の次、並びはワークスペースの中の末尾。
func (s Service) Create(ctx context.Context, in CreateInput) (project.Project, error) {
	me, err := s.Identity.Me(ctx, in.Store)
	if err != nil {
		return project.Project{}, err
	}
	title := strings.TrimSpace(in.Title)
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("プロジェクト「%s」を作成", title)}
	var created project.Project
	err = s.Stores.Write(ctx, in.Store, op, func(tx port.Tx) error {
		if _, err := tx.Workspaces().Get(in.Workspace); err != nil {
			return err
		}
		siblings, err := tx.Projects().List(in.Workspace)
		if err != nil {
			return err
		}
		if err := project.CheckCreate(in.Title, in.Start, in.End, siblings); err != nil {
			return err
		}
		wsLog, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: in.Workspace})
		if err != nil {
			return err
		}
		created = newProject(s.IDs.New(), in, title, numbering.Next(wsLog, numbering.Projects), siblings)
		return errors.Join(
			tx.WriteWorkspaces().Append(in.Workspace, numbering.Draft(numbering.Projects, created.Place.Number)),
			tx.WriteProjects().Append(created.ID, project.Create(created)),
		)
	})
	return created, err
}

func newProject(id project.ID, in CreateInput, title string, number int, siblings []project.Project) project.Project {
	last := ""
	if len(siblings) > 0 {
		last = siblings[len(siblings)-1].Rank
	}
	status := in.Status
	if status == "" {
		status = project.Todo
	}
	return project.Project{
		ID: id, Place: project.Place{Workspace: in.Workspace, Number: number}, Rank: rank.After(last),
		Title: title, Owner: in.Owner, Status: status, Start: in.Start, End: in.End,
	}
}
```

`internal/app/projects/get.go`:

```go
package projects

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// Get は、プロジェクトを 1 つ返す。なければ port.ErrNotFound。
func (s Service) Get(ctx context.Context, store port.StoreID, id project.ID) (project.Project, error) {
	var p project.Project
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		p, err = r.Projects().Get(id)
		return err
	})
	return p, err
}
```

`internal/app/projects/list.go`:

```go
package projects

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// List は、ワークスペースのプロジェクトを並び順で返す。ワークスペースがなければ port.ErrNotFound。
func (s Service) List(ctx context.Context, store port.StoreID, ws workspace.ID) ([]project.Project, error) {
	var out []project.Project
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := r.Workspaces().Get(ws); err != nil {
			return err
		}
		var err error
		out, err = r.Projects().List(ws)
		return err
	})
	return out, err
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/app/projects/ && make lint && go test ./internal/archtest/`
Expected: PASS、lint 0 件、archtest PASS(`app/projects` は domain と port だけを import している)

- [ ] **Step 5: Commit**

```bash
git add internal/app/projects
git commit -m "feat(app): プロジェクトの作成・取得・一覧を足す"
```

---

### Task 4: アイテムの作成・取得・一覧(app/items)

**Files:**
- Create: `internal/app/items/service.go`、`create.go`、`get.go`、`list.go`
- Test: `internal/app/items/items_test.go`

**Interfaces:**
- Consumes: Task 1・2 の `item.CheckCreate`・`item.NormalizeTitle`・`numbering`・`rank`・`port.Identity`・`ItemReader.List`
- Produces:
  - `items.Service{Stores port.Stores; IDs port.IDs; Identity port.Identity}`
  - `items.CreateInput{Store port.StoreID; Project project.ID; Parent *item.ID; Kind item.Kind; Title string}`
  - `(Service) Create(ctx, CreateInput) (item.Item, error)`
  - `(Service) Get(ctx, store port.StoreID, id item.ID) (item.Item, error)`
  - `(Service) List(ctx, store port.StoreID, p project.ID) ([]item.Item, error)`
  - `items.ErrParentElsewhere`(親が別のプロジェクトにある。画面の候補は同じプロジェクトだけなので、API の誤った呼び方)

**決め方(02「作成したあと」):** 番号はワークスペースのアイテム用の連番の次。並びは親の子の末尾(Epic はプロジェクトの直下の Epic の末尾)。ウォッチ中の人は作成者だけ。件名は「{種類}「{タイトル}」を作成」。

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/items/items_test.go`:

```go
package items_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const store port.StoreID = "S1"

func setup(t *testing.T) items.Service {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	return items.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "hanako"}}
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func ptr(id item.ID) *item.ID { return &id }

func TestCreateTaskUnderEpic(t *testing.T) {
	svc := setup(t)
	it, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Task, Title: " API を\n 作る "})
	if err != nil {
		t.Fatal(err)
	}
	// サンプルは HH-1(Epic)と HH-2(Task, 並び V)を持つ。次の番号は 3、並びは兄弟の末尾。
	if it.DisplayID("HH") != "HH-3" || it.Title != "API を 作る" || it.Rank != "W" || it.Place.Workspace != apptest.SampleWorkspace {
		t.Errorf("created %+v", it)
	}
	if !slices.Equal(it.Watchers, []string{"hanako"}) || it.Created.By != "hanako" {
		t.Errorf("watchers %v, created %+v", it.Watchers, it.Created)
	}
	list, err := svc.List(t.Context(), store, apptest.SampleProject)
	if err != nil || len(list) != 3 || list[2].ID != it.ID {
		t.Errorf("List = %+v, %v", list, err)
	}
}

func TestCreateEpicAndStep(t *testing.T) {
	svc := setup(t)
	epic, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: apptest.SampleProject, Kind: item.Epic, Title: "決済"})
	if err != nil || epic.Place.Parent != nil || epic.Rank != "W" || epic.Place.Number != 3 {
		t.Fatalf("epic %+v, %v", epic, err)
	}
	step, err := svc.Create(t.Context(), items.CreateInput{Store: store, Project: apptest.SampleProject, Parent: ptr(apptest.SampleTask), Kind: item.Step, Title: "下書き"})
	if err != nil || step.Place.Number != 4 || step.Rank != "V" {
		t.Fatalf("step %+v, %v", step, err)
	}
}

func TestCreateRefuses(t *testing.T) {
	svc := setup(t)
	cases := []struct {
		in   items.CreateInput
		want string
	}{
		{items.CreateInput{Project: apptest.SampleProject, Kind: item.Task, Title: " \n "}, "タイトルを入力してください"},
		{items.CreateInput{Kind: item.Task, Title: "画面"}, "プロジェクトを選んでください"},
		{items.CreateInput{Project: apptest.SampleProject, Kind: item.Step, Title: "下書き"}, "親になる Story・Task・Bug を選んでください"},
		{items.CreateInput{Project: apptest.SampleProject, Kind: item.Task, Title: "画面"}, "親になる Epic を選んでください"},
		{items.CreateInput{Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Kind: item.Step, Title: "下書き"}, "Step は Epic の下に置けません"},
	}
	for _, c := range cases {
		c.in.Store = store
		if _, err := svc.Create(t.Context(), c.in); message(err) != c.want {
			t.Errorf("%+v: %v, want %q", c.in, err, c.want)
		}
	}
	if list, _ := svc.List(t.Context(), store, apptest.SampleProject); len(list) != 2 {
		t.Errorf("a refused create saved something: %d items", len(list))
	}
}

func TestGetUnknown(t *testing.T) {
	if _, err := setup(t).Get(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/app/items/`
Expected: FAIL(`undefined: items.Service` ほか)

- [ ] **Step 3: 実装する**

`internal/app/items/service.go`:

```go
// Package items は、アイテムの操作(02)を持つ。1 ファイル 1 操作。
package items

import "github.com/kukv/hedgehug/internal/app/port"

// Service は、アイテムの操作が使う口。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
}
```

`internal/app/items/create.go`:

```go
package items

import (
	"context"
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
)

// ErrParentElsewhere は、親が別のプロジェクトにあるとき。作成ダイアログの親の候補は同じプロジェクトだけ(02)。
var ErrParentElsewhere = errors.New("the parent is in another project")

// CreateInput は、作成ダイアログの入力のうち A1 で扱うもの。プロパティとテンプレートは A3 で足す。
type CreateInput struct {
	Store   port.StoreID
	Project project.ID
	Parent  *item.ID
	Kind    item.Kind
	Title   string
}

// Create は、アイテムを作る(02「作成したあと」)。
func (s Service) Create(ctx context.Context, in CreateInput) (item.Item, error) {
	me, err := s.Identity.Me(ctx, in.Store)
	if err != nil {
		return item.Item{}, err
	}
	title := item.NormalizeTitle(in.Title)
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("%s「%s」を作成", in.Kind, title)}
	var id item.ID
	err = s.Stores.Write(ctx, in.Store, op, func(tx port.Tx) error {
		created, err := s.build(tx, in, title, me)
		if err != nil {
			return err
		}
		id = created.ID
		return errors.Join(
			tx.WriteWorkspaces().Append(created.Place.Workspace, numbering.Draft(numbering.Items, created.Place.Number)),
			tx.WriteItems().Append(created.ID, item.Create(created)),
		)
	})
	if err != nil {
		return item.Item{}, err
	}
	return s.Get(ctx, in.Store, id)
}

// build は、検査をして、作るアイテムを決める。
func (s Service) build(tx port.Tx, in CreateInput, title, me string) (item.Item, error) {
	parentKind, err := kindOf(tx, in.Parent, in.Project)
	if err != nil {
		return item.Item{}, err
	}
	if err := item.CheckCreate(in.Kind, in.Title, in.Project != "", parentKind); err != nil {
		return item.Item{}, err
	}
	p, err := tx.Projects().Get(in.Project)
	if err != nil {
		return item.Item{}, err
	}
	all, err := tx.Items().List(in.Project)
	if err != nil {
		return item.Item{}, err
	}
	wsLog, err := tx.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: p.Place.Workspace})
	if err != nil {
		return item.Item{}, err
	}
	return item.Item{
		ID:       s.IDs.New(),
		Place:    item.Place{Workspace: p.Place.Workspace, Project: p.ID, Parent: in.Parent, Number: numbering.Next(wsLog, numbering.Items)},
		Kind:     in.Kind,
		Rank:     rank.After(lastRank(all, in.Parent)),
		Title:    title,
		Watchers: []string{me},
	}, nil
}

// kindOf は、親の種類を返す。親を選んでいなければ nil。
func kindOf(tx port.Tx, parent *item.ID, p project.ID) (*item.Kind, error) {
	if parent == nil {
		return nil, nil
	}
	it, err := tx.Items().Get(*parent)
	if err != nil {
		return nil, err
	}
	if it.Place.Project != p {
		return nil, ErrParentElsewhere
	}
	return &it.Kind, nil
}

// lastRank は、同じ親の子のうち、最後の並び順の値。親が nil なら、プロジェクトの直下(Epic)。
func lastRank(all []item.Item, parent *item.ID) string {
	last := ""
	for _, it := range all {
		same := (parent == nil && it.Place.Parent == nil) ||
			(parent != nil && it.Place.Parent != nil && *it.Place.Parent == *parent)
		if same && it.Rank > last {
			last = it.Rank
		}
	}
	return last
}
```

`kindOf` が `nil, nil` を返すのは「親を選んでいない」を表すためで、`nilnil` の lint が出たらその行に `//nolint:nilnil // 親なしは正しい結果` を付ける。

`internal/app/items/get.go`:

```go
package items

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// Get は、アイテムを 1 つ返す。なければ port.ErrNotFound。
func (s Service) Get(ctx context.Context, store port.StoreID, id item.ID) (item.Item, error) {
	var it item.Item
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		it, err = r.Items().Get(id)
		return err
	})
	return it, err
}
```

`internal/app/items/list.go`:

```go
package items

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// List は、プロジェクトのすべてのアイテムを番号の順で返す。プロジェクトがなければ port.ErrNotFound。
func (s Service) List(ctx context.Context, store port.StoreID, p project.ID) ([]item.Item, error) {
	var out []item.Item
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := r.Projects().Get(p); err != nil {
			return err
		}
		var err error
		out, err = r.Items().List(p)
		return err
	})
	return out, err
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/app/items/ && make lint && go test ./internal/archtest/`
Expected: PASS、lint 0 件、archtest PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/items
git commit -m "feat(app): アイテムの作成・取得・一覧を足す"
```

---

### Task 5: プロジェクトとアイテムの API

**Files:**
- Create: `internal/adapter/httpapi/projects/projects.go`、`internal/adapter/httpapi/items/items.go`
- Test: `internal/adapter/httpapi/projects/projects_test.go`、`internal/adapter/httpapi/items/items_test.go`
- Modify: `internal/adapter/httpapi/routes.go`(Register を 2 行)、`api/openapi.yaml`(生成物)、`web/packages/api/src/generated.ts`(画面のラインが入っていれば生成物)

**Interfaces:**
- Consumes: Task 3・4 の `projects.Service`・`items.Service`、Task 1 の `apierr.From`・`httpapi.Deps`
- Produces(operationId と経路):
  - `create-project` `POST /api/stores/{store}/workspaces/{workspace}/projects`
  - `list-projects` `GET /api/stores/{store}/workspaces/{workspace}/projects`
  - `get-project` `GET /api/stores/{store}/projects/{project}`
  - `create-item` `POST /api/stores/{store}/items`
  - `get-item` `GET /api/stores/{store}/items/{item}`
  - `list-project-items` `GET /api/stores/{store}/projects/{project}/items`
  - 本文の形: `Project{id, workspace, number, rank, title, owner, status, start, end}`、`Item{id, workspace, project, parent(null 可), number, kind, rank, title, watchers, createdAt, createdBy}`
  - 断りは 422 で、`detail` に仕様の文言

- [ ] **Step 1: 失敗するテストを書く**

`internal/adapter/httpapi/projects/projects_test.go`:

```go
package projects_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/projects"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appprojects "github.com/kukv/hedgehug/internal/app/projects"
)

func newAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	projects.Register(api, appprojects.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "taro"}})
	return api
}

const base = "/api/stores/S1/workspaces/" + string(apptest.SampleWorkspace) + "/projects"

func TestCreateAndList(t *testing.T) {
	api := newAPI(t)
	res := api.Post(base, map[string]any{"title": "決済", "owner": "taro", "status": "InProgress", "start": "2026-11-01", "end": ""})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"number":2`) || !strings.Contains(res.Body.String(), `"status":"InProgress"`) {
		t.Fatalf("create: %d %s", res.Code, res.Body)
	}
	res = api.Get(base)
	if res.Code != http.StatusOK || strings.Count(res.Body.String(), `"id":`) != 2 {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/stores/S1/projects/" + string(apptest.SampleProject))
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"title":"ログイン"`) {
		t.Errorf("get: %d %s", res.Code, res.Body)
	}
}

func TestRefusalIs422WithTheMessage(t *testing.T) {
	res := newAPI(t).Post(base, map[string]any{"title": "ログイン"})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "このワークスペースには同じ名前のプロジェクトがあります") {
		t.Errorf("%d %s", res.Code, res.Body)
	}
}

func TestBadInputIsRefusedByTheSchema(t *testing.T) {
	api := newAPI(t)
	for _, body := range []map[string]any{{"title": "x", "status": "Review"}, {"title": "x", "start": "11/01"}} {
		if res := api.Post(base, body); res.Code != http.StatusUnprocessableEntity {
			t.Errorf("%v: %d", body, res.Code)
		}
	}
}

func TestUnknownStoreIs404(t *testing.T) {
	if res := newAPI(t).Get("/api/stores/nope/projects/x"); res.Code != http.StatusNotFound {
		t.Errorf("%d", res.Code)
	}
}
```

`internal/adapter/httpapi/items/items_test.go`:

```go
package items_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/items"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appitems "github.com/kukv/hedgehug/internal/app/items"
)

func newAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	_, api := humatest.New(t)
	items.Register(api, appitems.Service{Stores: s, IDs: ids, Identity: apptest.Identity{Login: "taro"}})
	return api
}

func TestCreateGetList(t *testing.T) {
	api := newAPI(t)
	res := api.Post("/api/stores/S1/items", map[string]any{
		"project": string(apptest.SampleProject), "parent": string(apptest.SampleEpic), "kind": "Task", "title": "API",
	})
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, `"number":3`) || !strings.Contains(body, `"watchers":["taro"]`) || !strings.Contains(body, `"createdBy":"taro"`) {
		t.Fatalf("create: %d %s", res.Code, body)
	}
	if res := api.Get("/api/stores/S1/items/" + string(apptest.SampleEpic)); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"parent":null`) {
		t.Errorf("get: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/projects/" + string(apptest.SampleProject) + "/items"); res.Code != http.StatusOK || strings.Count(res.Body.String(), `"kind":`) != 3 {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
}

func TestRefusals(t *testing.T) {
	api := newAPI(t)
	res := api.Post("/api/stores/S1/items", map[string]any{"project": string(apptest.SampleProject), "kind": "Task", "title": "画面"})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "親になる Epic を選んでください") {
		t.Errorf("no parent: %d %s", res.Code, res.Body)
	}
	res = api.Post("/api/stores/S1/items", map[string]any{"project": "", "kind": "Task", "title": "画面"})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "プロジェクトを選んでください") {
		t.Errorf("no project: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items", map[string]any{"project": "x", "kind": "Saga", "title": "x"}); res.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad kind: %d", res.Code)
	}
	if res := api.Get("/api/stores/S1/items/01NOPE00000000000000000000"); res.Code != http.StatusNotFound {
		t.Errorf("unknown: %d", res.Code)
	}
}
```

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/adapter/httpapi/projects/ ./internal/adapter/httpapi/items/`
Expected: FAIL(`undefined: projects.Register` ほか)

- [ ] **Step 3: 実装する**

`internal/adapter/httpapi/projects/projects.go`:

```go
// Package projects は、プロジェクトの API を持つ。
package projects

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	"github.com/kukv/hedgehug/internal/app/port"
	appprojects "github.com/kukv/hedgehug/internal/app/projects"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// Project は、API が返すプロジェクト。
type Project struct {
	ID        string `json:"id"`
	Workspace string `json:"workspace"`
	Number    int    `json:"number"`
	Rank      string `json:"rank"`
	Title     string `json:"title"`
	Owner     string `json:"owner" doc:"ログイン名。空は「なし」"`
	Status    string `json:"status" enum:"Todo,InProgress,Done,Pending,Cancel"`
	Start     string `json:"start" doc:"YYYY-MM-DD。空は「なし」"`
	End       string `json:"end"`
}

func toAPI(p project.Project) Project {
	return Project{
		ID: string(p.ID), Workspace: string(p.Place.Workspace), Number: p.Place.Number, Rank: p.Rank,
		Title: p.Title, Owner: p.Owner, Status: string(p.Status), Start: p.Start, End: p.End,
	}
}

type wsPath struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
}

type createInput struct {
	wsPath
	Body struct {
		Title  string `json:"title"`
		Owner  string `json:"owner,omitempty"`
		Status string `json:"status,omitempty" enum:"Todo,InProgress,Done,Pending,Cancel,"`
		Start  string `json:"start,omitempty" pattern:"^(\\d{4}-\\d{2}-\\d{2})?$"`
		End    string `json:"end,omitempty" pattern:"^(\\d{4}-\\d{2}-\\d{2})?$"`
	}
}

type oneOutput struct{ Body Project }

type listOutput struct{ Body []Project }

type getInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
}

// Register は、プロジェクトの API を登録する。
func Register(api huma.API, svc appprojects.Service) {
	huma.Register(api, huma.Operation{OperationID: "create-project", Method: http.MethodPost, Path: "/api/stores/{store}/workspaces/{workspace}/projects", Summary: "プロジェクトを作成"},
		func(ctx context.Context, in *createInput) (*oneOutput, error) {
			p, err := svc.Create(ctx, appprojects.CreateInput{
				Store: port.StoreID(in.Store), Workspace: workspace.ID(in.Workspace), Title: in.Body.Title,
				Owner: in.Body.Owner, Status: project.Status(in.Body.Status), Start: in.Body.Start, End: in.Body.End,
			})
			if err != nil {
				return nil, apierr.From(err)
			}
			return &oneOutput{Body: toAPI(p)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "list-projects", Method: http.MethodGet, Path: "/api/stores/{store}/workspaces/{workspace}/projects", Summary: "ワークスペースのプロジェクトを並び順で返す"},
		func(ctx context.Context, in *wsPath) (*listOutput, error) {
			ps, err := svc.List(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &listOutput{Body: []Project{}}
			for _, p := range ps {
				out.Body = append(out.Body, toAPI(p))
			}
			return out, nil
		})
	huma.Register(api, huma.Operation{OperationID: "get-project", Method: http.MethodGet, Path: "/api/stores/{store}/projects/{project}", Summary: "プロジェクトを返す"},
		func(ctx context.Context, in *getInput) (*oneOutput, error) {
			p, err := svc.Get(ctx, port.StoreID(in.Store), project.ID(in.Project))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &oneOutput{Body: toAPI(p)}, nil
		})
}
```

`internal/adapter/httpapi/items/items.go`:

```go
// Package items は、アイテムの API を持つ。
package items

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// Item は、API が返すアイテム。
type Item struct {
	ID        string   `json:"id"`
	Workspace string   `json:"workspace"`
	Project   string   `json:"project"`
	Parent    *string  `json:"parent" doc:"親の内部 ID。Epic は null"`
	Number    int      `json:"number"`
	Kind      string   `json:"kind" enum:"Epic,Story,Task,Bug,Step"`
	Rank      string   `json:"rank"`
	Title     string   `json:"title"`
	Watchers  []string `json:"watchers"`
	CreatedAt string   `json:"createdAt"`
	CreatedBy string   `json:"createdBy"`
}

func toAPI(it item.Item) Item {
	out := Item{
		ID: string(it.ID), Workspace: string(it.Place.Workspace), Project: string(it.Place.Project),
		Number: it.Place.Number, Kind: string(it.Kind), Rank: it.Rank, Title: it.Title, Watchers: it.Watchers,
		CreatedAt: it.Created.At.Format(change.TimeLayout), CreatedBy: it.Created.By,
	}
	if out.Watchers == nil {
		out.Watchers = []string{}
	}
	if it.Place.Parent != nil {
		p := string(*it.Place.Parent)
		out.Parent = &p
	}
	return out
}

type createInput struct {
	Store string `path:"store"`
	Body  struct {
		Project string `json:"project" doc:"空はプロジェクトを選んでいない"`
		Parent  string `json:"parent,omitempty" doc:"空は親を選んでいない"`
		Kind    string `json:"kind" enum:"Epic,Story,Task,Bug,Step"`
		Title   string `json:"title"`
	}
}

type oneOutput struct{ Body Item }

type listOutput struct{ Body []Item }

type getInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
}

type listInput struct {
	Store   string `path:"store"`
	Project string `path:"project"`
}

// Register は、アイテムの API を登録する。
func Register(api huma.API, svc appitems.Service) {
	huma.Register(api, huma.Operation{OperationID: "create-item", Method: http.MethodPost, Path: "/api/stores/{store}/items", Summary: "アイテムを作成"},
		func(ctx context.Context, in *createInput) (*oneOutput, error) {
			req := appitems.CreateInput{Store: port.StoreID(in.Store), Project: project.ID(in.Body.Project), Kind: item.Kind(in.Body.Kind), Title: in.Body.Title}
			if in.Body.Parent != "" {
				p := item.ID(in.Body.Parent)
				req.Parent = &p
			}
			it, err := svc.Create(ctx, req)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &oneOutput{Body: toAPI(it)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "get-item", Method: http.MethodGet, Path: "/api/stores/{store}/items/{item}", Summary: "アイテムを返す"},
		func(ctx context.Context, in *getInput) (*oneOutput, error) {
			it, err := svc.Get(ctx, port.StoreID(in.Store), item.ID(in.Item))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &oneOutput{Body: toAPI(it)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "list-project-items", Method: http.MethodGet, Path: "/api/stores/{store}/projects/{project}/items", Summary: "プロジェクトのすべてのアイテムを番号の順で返す"},
		func(ctx context.Context, in *listInput) (*listOutput, error) {
			its, err := svc.List(ctx, port.StoreID(in.Store), project.ID(in.Project))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &listOutput{Body: []Item{}}
			for _, it := range its {
				out.Body = append(out.Body, toAPI(it))
			}
			return out, nil
		})
}
```

`ErrParentElsewhere` は `apierr.From` が 500 にする(画面の候補は同じプロジェクトだけなので、起きるのは呼び出しの誤りだけ)。

`internal/adapter/httpapi/routes.go` の `NewAPI` の `system.Register(api, deps.Version)` の下に次の 2 行を足す(import に `appitems`・`appprojects` と、`httpapi/items`・`httpapi/projects` を足す):

```go
	projects.Register(api, appprojects.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity})
	items.Register(api, appitems.Service{Stores: deps.Stores, IDs: deps.IDs, Identity: deps.Identity})
```

- [ ] **Step 4: テストが通ることを確かめ、生成物を作り直す**

Run: `go test ./internal/adapter/... && make generate && git diff --stat api/openapi.yaml`
Expected: PASS。`api/openapi.yaml` に 6 つの operationId が足される。

画面のライン(土台の Task 10)が develop に入っていれば、続けて `cd web && pnpm gen` で `web/packages/api/src/generated.ts` も作り直す(CI が生成物の差分を確かめるため)。

humatest が 422 の本文に、仕様の文言を `detail` として入れていることを確かめる:

Run: `go test ./internal/adapter/httpapi/projects/ -run TestRefusalIs422WithTheMessage -v`
Expected: PASS

- [ ] **Step 5: 全体を確かめる**

Run: `make check`
Expected: すべて成功

- [ ] **Step 6: Commit**

```bash
git add internal/adapter api web/packages/api/src/generated.ts
git commit -m "feat(adapter): プロジェクトとアイテムの作成・取得・一覧の API を足す"
```

(`web/packages/api/src/generated.ts` がまだなければ、`git add` から外す。)

---

## 未決の点

仕様と既存の規則では決まらず、ほかのトラックと合わせる必要がある点。A1 は下の推奨案で進められる。

| # | 点 | 推奨案 |
|---|---|---|
| 1 | 「あなた」がメンバーにいないとき、次の書き込みと一緒に足す(01「「あなた」の決まり方」)処理を、どこが持つか。どの保存にも関わる | Stores の口の外の小さな共通の usecase の部品にすると機能どうしの import になるので、トラック G が `port.Identity` の本物と一緒に「保存の前にメンバーを足す」を `Identity` 側の約束として決め、A〜D は `Me` を呼ぶだけにする |
| 2 | 画面のアドレス(01)の仕組み(ルーター)を、どのトラックが先に入れるか。A2・B・C・D のどれもが画面を足す | A2 の先頭に、ルーターだけの小さな共有部分の PR を置く。B・C・D の画面がそれより先に要るなら、先に要るトラックが入れ、A2 はそれを使う(親のセッションで決めてほしい) |
| 3 | `domain/refusal`・`port.Identity`・`apierr` は B・C・D も使う。A1 の Task 1 と、ほかのトラックの計画が同じものを別々に足すと衝突する | A1 の Task 1 を最初に develop に入れ、B・C・D の計画はそれを前提にする(親のセッションで、ほかのトラックの計画と合わせてほしい) |
