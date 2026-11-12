# hedgehug 小計画 C2 画面: 同期カード・オフラインの帯・同期のトースト・画面への反映

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** C2 のサーバー(同期の API と Server-Sent Events)を画面につなぎ、サイドバーの同期カード、「Store ごとの同期状態」、860px 以下の点、オフラインの帯、手動の同期のトーストと Shift+Ctrl+S、自動の同期を待たせる知らせ、別の書き手の変更の 1 秒ほどでの反映を作る。

**Architecture:** A2・D2 画面と同じ形にする。カードの文言の組み立て・手動の同期の流れ・反映の溜め方・タブの知らせは `core/features/sync` に置き、DOM(フォーカス・ダイアログ・見えているか・オンラインか)は `app` が関数として core に渡す。`api` は同期の API を core の口(`SyncPort`)に合わせ、`GET /api/events` を fetch で読み続ける(EventSource はヘッダーを付けられない。決定 C2-20)。`ui` はカード・帯・点を出して events を返すだけにする。

**Tech Stack:** 土台と同じ(Go 1.27.3、huma v2、Svelte 5、orval、Vitest、Testing Library、Playwright)。新しい依存は足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`「手動で同期する」「自動で同期する」(待つ条件)「オフライン」「同期のエラー」(文言)「同期の結果」「同期の状態の表示」「画面への反映」〜「反映で変わる画面」
- 01「日付と時刻」(相対時間の表)、01「画面の移り方と、共通の表示」(トースト、860px 以下の点)
- `docs/superpowers/plans/2026-11-13-hedgehug-track-c2.md`(決定 C2-1・C2-16・C2-18・C2-18b・C2-20・C2-21・C2-24、「C2 のあと」の C2 画面の項、「実装で計画から変えたこと」)
- `docs/superpowers/plans/2026-11-13-hedgehug-track-a2.md`(画面の決定。とくに 24・26・29)
- 見た目: モック版 19(https://claude.ai/artifact/Qw2jqgdbYtbcA4VTMHGCVA)のサイドバーの同期カード。振る舞いは仕様に従い、モックの動きを写さない。

**ブランチ:** `origin/develop` から `feat/track-c2-screen`。PR の向き先は `develop`。計画書とすべてのタスクを 1 本の PR にまとめる。実装のあいだは push しない。

## Global Constraints

A2 の計画の Global Constraints をすべて引き継ぐ。再掲する:

- 画面の文言・トーストの文言は、仕様の文字どおりに書く。仕様にない文言は足さない。足すときは「この計画で決めたこと」に載せる。
- 新しい依存(npm・Go)は足さない。
- `api/openapi.yaml` と `web/packages/api/src/generated.ts` は手で直さない。API を変えたタスクで `make generate` と `cd web && pnpm gen` を実行し、同じコミットに入れる。
- 大きさの上限(Svelte 200 行・TS 300 行・Go ファイル 300 行・関数 60 行・認知的複雑度 15)を超えたら、上限を上げずに分ける。
- ui の部品は渡された値を出して events を返すだけ。値の計算・検査・操作の流れは core に置く(設計書 6 節)。core は DOM を import しない(`lib` は ES2023 だけ)。
- 計画やテストに、トークンらしい値(`ghp_`・`gho_`・`ghu_`・`ghs_`・`ghr_`・`github_pat_` で始まる文字列)を書かない。
- force push はしない。
- 各タスクは TDD で進める(失敗するテストを書く → 落ちるのを見る → 実装 → 通す → コミット)。コミットの前に、そのタスクで触ったパッケージのテスト・lint・型検査を通す。web は `cd web && pnpm lint && pnpm check && pnpm test`、Go は `make check`。

---

## 範囲

**作るもの:** サイドバーの同期カード(要約・最終同期・24 時間の警告・「すべて同期」と案内)、「Store ごとの同期状態」と、そこから 1 つの Store の同期、860px 以下のメニューのボタンの点、オフラインの帯(自動同期のオン・オフで文を替える)、手動の同期のトースト(1 つ・すべて・ミラーの警告・オフライン・同期する Store がない)、Shift+Ctrl+S、自動の同期を待たせる知らせ(`PUT /api/sync/hold`)とネットワークの知らせ(`PUT /api/sync/network`)、`GET /api/events` を受けて開いている画面を読み直すこと(待つ条件のあいだと、見えていないあいだは溜めて 1 回で)。

**作らないもの(入口がまだない。入口を作るトラックが足す):**

| 仕様の箇所 | 作るトラック |
|---|---|
| コマンドパレットの「すべての Store を同期」 | コマンドパレットのトラック(E) |
| 設定「一般」の「すべて同期」、設定「Store」の「今すぐ同期」と同期の状況 | G |
| サイドバーの Store メニューの「今すぐ同期」(remote なしは無効) | Store メニューを作るトラック(G) |
| 同期カードの時計のずれの行 | C5 |
| 他の書き手との重なりのトースト(削除の取り消し) | C3 |
| 待つ条件のうち「ドラッグしているとき」「説明やコメントを編集しているとき」「書きかけがあるとき」 | ドラッグ(A4)、説明・コメントの入力欄(A3 以降)を作るトラック。決定 S9 の `holdReasons` に足す |
| 「GitHub を使わない場合」の同期の無効の入口(D2 画面の決定 S2 で C2 画面に回したもの) | G。いま同期の入口は同期カードだけで、カードは remote がある Store があるときだけ出る(06)。remote を足す入口(G)と一緒に作る |

**本物のアプリでの見え方:** 本物の組み立ては `port.SyncTargets` に Store のない実装を使う(決定 C2-2)。そのため、G が入るまでは、本物のアプリでは同期カードもオフラインの帯も出ない(どちらも「remote がある表示中の Store」が要る)。e2e のビルドは 2 つの Store(e2e・readonly)に remote を持つので、カードが出る。

---

## この計画で決めたこと(既存の規則の当てはめ)

| # | 決めたこと | 根拠 |
|---|---|---|
| S1 | `GET /api/sync` の本文に `auto`(「裏で自動的に同期する」がオンか)を足す。画面はオフラインの帯の文を、これで選ぶ | 06「オフライン」(オンとオフで帯の文が違う)。画面に設定を読む API がまだない(設定「一般」は G) |
| S2 | サーバーのオフラインの知らせの期限を 30 秒から 2 分にする(待つ条件の知らせは 30 秒のまま)。タブは、オフラインのあいだ 10 秒ごとに送り直す | ブラウザは見えないタブのタイマーを 1 分に 1 回ほどに間引く(C2 の引き継ぎ)。期限が切れると、すべてのタブが見えないときに `Auto` が同期を試し、「接続できません」を記録する。06「オフラインは Store の同期の状態をエラーにしない」に反する |
| S3 | 待つ条件の知らせ(`hold`)は、見えているタブだけが送る。見えなくなったら、すぐ `hold: false` を送る。待つ条件にあるあいだは 10 秒ごとに `hold: true` を送り直す | 06「画面に見えているブラウザのタブの条件だけで決める」。決定 C2-18 |
| S4 | ネットワークの知らせ(`network`)は、見えているかを問わず、オフラインになったとき、オフラインのあいだ 10 秒ごと、つながったときに送る。オンラインのあいだは送らない(つながったときの 1 回だけ) | 決定 C2-18b |
| S5 | タブの識別子は、ページを開くたびに `crypto.randomUUID()` で作る。保存しない | 決定 C2-18(タブごとの知らせ)。再読み込みしたタブの古い知らせは期限で切れる |
| S6 | 同期カードは、状態の一覧のうち非表示でない Store が 1 つ以上あるときだけ出す。要約・2 行目・3 行目は 06 の文言どおり。「1 度も同期に成功していない Store」は、2 行目の時刻にも、3 行目の 24 時間にも数えない(最後に成功した時刻がないので、経過時間がない) | 06「同期の状態の表示」 |
| S7 | 相対時間は 01 の表どおり(たった今・N 分前・N 時間前・N 日前・日付)。7 日以上は `formatDay`(今年なら M/D)。カードの相対時間と 24 時間の判定は 30 秒ごとに数え直す | 01「日付と時刻」。06(経過時間で判定) |
| S8 | 「Store ごとの同期状態」は、同期カードの要約(ボタン)を押すと開くメニュー。行は「{Store 名}」と「{要約} · {相対時間 または 未同期}」。24 時間以上の行は相対時間を警告の色にする。行を選ぶと、その Store を同期する(1 つの Store の同期のトースト)。Esc・外のクリックで閉じる。ui の `Menu` は 1 行に 1 色の補足しか出せないので、カードの中に専用のメニューを持つ | 06「同期の状態の表示」 |
| S9 | 自動の同期と反映を待つ条件は、いま画面にあるものだけを見る: ダイアログ(`[aria-modal="true"]`)、メニュー(`[role="menu"]`)、入力欄にフォーカス(`input`・`textarea`・`select`・`[contenteditable="true"]`)。app の `holdReasons`(関数の配列)に並べ、ドラッグ・説明やコメント・書きかけは、それを作るトラックが足す | 06「自動で同期する」の 5 つの条件のうち、いまある部品の 3 つ。残りは部品がない |
| S10 | 反映: `store` の出来事を受けたら、Store の一覧(`catalog.load`)、開いているプロジェクトの画面(`projectScreen.reload`)、開いている詳細パネル(`detail.reload`)を読み直す。`sync` の出来事なら同期の状態の一覧を読み直す。待つ条件にあるあいだと見えていないあいだは溜め、条件が解けたら(0.5 秒ごとに確かめる)種類ごとに 1 回で読み直す。見えるようになったときも同じ | 06「反映を待つとき」「反映するもの」。決定 C2-20 |
| S11 | `GET /api/events` につないだとき(つなぎ直しを含む)は、溜めていなくても、両方の種類を 1 回読み直す。切れたら 1 秒あとにつなぎ直し、続けて失敗するたびに 2 倍(最大 10 秒)にする | サーバーは溢れた出来事を落とす(events.go の `buffer`)。切れているあいだの変化も取りこぼさない |
| S12 | 手動の同期の結果は、待たずに反映する(終わったら、溜めているかを問わず、両方の種類を読み直す)。詳細パネルのタイトルの入力欄は、フォーカスがあるあいだは読み直しで書き換えない(別のアイテムを開いたときは書き換える) | 06「手動の同期の結果は、待たずに反映する。ただし、編集中の入力欄の内容は書き換えない」 |
| S13 | 手動の同期のトースト: 1 つの Store は、`synced`・`skipped` を通常、`failed` をエラーで `message` を出し、`mirrorWarning` があれば続けて警告のトーストを出す。「すべて同期」は、`lines` を改行でつないで 1 つのトーストにし、`failed` ならエラー、`synced`・`nothing` は通常 | 06「同期の結果」「手動で同期する」。決定 C2-21。文言はサーバー(domain/syncstatus)が作り、画面は組み立てない |
| S14 | 警告のトーストは、警告の色(`--c-amber-*`)で、通常と同じ 4.5 秒で消える。トーストの文の改行は、そのまま行にする | 06 は「警告のトースト」と言うが、01 のトーストは通常とエラーの 2 つ。消える時間は 01 の通常に合わせ、色だけ分ける |
| S15 | オフライン(`navigator.onLine` が false)のときの手動の同期は、サーバーを呼ばずに、エラーのトースト「オフラインのため同期できません。つながったら同期します」を 1 つ出す(1 つの Store でも、すべてでも) | 06「オフライン」 |
| S16 | 手動の同期の通信の失敗(サーバーに届かない・500・404)は、画面に出さない | A2 の決定 29 |
| S17 | Shift+Ctrl+S(Mac は Shift+Cmd+S)は、window の keydown を capture で受け、入力中・ダイアログを開いているときも「すべて同期」を行う(ブラウザの既定の動きは止める)。キーの判定は core の `isSyncAllKey` | 06「手動で同期する」。サイドバーはいまのすべての画面にある |
| S18 | 「すべて同期」のボタンの案内は `title` 属性で出す。Mac(`navigator.platform` か `userAgent` に `Mac`)なら「Shift+Cmd+S」、それ以外は「Shift+Ctrl+S」 | 06「ポインターを置くと…案内を出す」 |
| S19 | 860px 以下のメニューのボタンの点は、`PageHead` の `alert` で出す。点は、ボタンの外に `role="img"`・`aria-label="同期エラー"` で置く(ボタンの名前「メニュー」は変えない) | 06「同期の状態の表示」、01。e2e の `sidebar()` はボタンを名前「メニュー」で探す |
| S20 | オフラインの帯は、本文の上(各画面の見出しの上)に、`role="status"` で出す。帯は、オフラインで、かつ同期カードを出す条件(S6)のときだけ | 06「オフライン」 |
| S21 | e2e のビルドは自動同期がオフ(`Auto: false`)なので、帯はオフの文になる。オンの文、同期エラーのカードと点、24 時間の警告、失敗のトーストは、`/api/sync`・`/api/stores/*/sync` の応答を Playwright で差し替えて確かめる。オフラインは `navigator.onLine` を差し替えて `offline`・`online` の出来事を送る(`context.setOffline` は localhost への通信も止める) | 3 つの幅が 1 つのサーバーを共有するので、サーバーの同期の状態を変えるとほかのテストに響く(D2 画面の決定 S23 と同じ考え) |

---

## ファイルの地図

```
Go(Task 1)
internal/adapter/httpapi/syncing/dto.go(変更)         SyncStatusList.Auto
internal/adapter/httpapi/syncing/syncing.go(変更)     一覧で Settings.Timing を読む
internal/adapter/httpapi/syncing/syncing_test.go(変更)
internal/app/syncing/auto.go(変更)                     offlineFor = 2 分
internal/app/syncing/auto_test.go(変更)
api/openapi.yaml・web/packages/api/src/generated.ts(生成物)

web/packages/core(Task 2〜6)
src/features/dates/relative.ts(+test)                 relativeTime・olderThanADay
src/model/sync.ts                                       SyncStatus ほかの型
src/port/sync.ts                                        SyncPort
src/features/sync/card.ts(+test)                      syncCard・hasSyncError・offlineBand・isSyncAllKey
src/features/sync/sync.ts(+test)                      createSync(状態・手動の同期・トースト)
src/features/sync/fixtures.test-util.ts                 偽の SyncPort と時計
src/features/sync/reflection.ts(+test)                createReflection(出来事の受け取り・溜める・つなぎ直し)
src/features/sync/signals.ts(+test)                   createTabSignals(hold・network の知らせ)
src/features/toasts/toasts.ts(+test、変更)            警告のトースト
src/features/items/detail.ts(+test、変更)             reload
src/index.ts(変更)

web/packages/api(Task 7)
src/fetcher.ts(変更)                                   authHeader
src/sync.ts(+test)                                    createSyncPort(watch を含む)
src/index.ts(変更)

web/packages/ui(Task 8)
src/patterns/SyncCard.svelte、OfflineBand.svelte(+sync.test.ts)
src/layout/PageHead.svelte(変更、+layout.test.ts)    alert の点
src/primitives/Toasts.svelte(変更、+overlays.test.ts) 警告・改行
src/primitives/Input.svelte(変更、+inputs.test.ts)   onfocus
src/types.ts・src/index.ts(変更)

web/app(Task 9)
src/tab.ts                                              DOM の関数(holdReasons・visible・online・isMac)
src/wire.ts(変更)                                      sync・reflection・signals の組み立て
src/App.svelte(変更)                                   帯・Shift+Ctrl+S・開始
src/shell/Sidebar.svelte(変更)                         同期カード
src/shell/Screen.svelte、screens/project/ProjectScreen.svelte、screens/workspace/WorkspaceScreen.svelte(変更)   alert
src/screens/item/DetailPanel.svelte(変更)              タイトルの入力欄を守る

web/e2e(Task 10)
sync.spec.ts
```

---

### Task 1: 同期の状態に自動同期のオン・オフを足し、オフラインの知らせを 2 分にする

**Files:**
- Modify: `internal/adapter/httpapi/syncing/dto.go`
- Modify: `internal/adapter/httpapi/syncing/syncing.go`
- Modify: `internal/adapter/httpapi/syncing/syncing_test.go`
- Modify: `internal/app/syncing/auto.go`
- Modify: `internal/app/syncing/auto_test.go`
- Regenerate: `api/openapi.yaml`、`web/packages/api/src/generated.ts`

**Interfaces:**
- Produces: `GET /api/sync` の本文 `{ auto: boolean, stores: SyncStoreStatus[] }`(生成物の `SyncStatusList.auto`)。

- [ ] **Step 1: 失敗するテストを書く(API)**

`internal/adapter/httpapi/syncing/syncing_test.go` の `TestSyncAllAndStatus` の、`st` を読んだ直後に足す:

```go
	if !st.Auto {
		t.Errorf("auto = false, want true (settings say on)")
	}
```

ファイルの末尾に足す:

```go
// 決定 S1(C2 画面): 一覧は「裏で自動的に同期する」を返す(オフラインの帯の文を選ぶ)。
func TestStatusReportsAutoOff(t *testing.T) {
	e := newEnv(t)
	e.auto.Settings = &apptest.SyncSettings{Value: port.SyncTiming{Auto: false}}
	res := e.api.Get("/api/sync")
	st := decode[syncing.SyncStatusList](t, res.Body.Bytes())
	if res.Code != http.StatusOK || st.Auto {
		t.Errorf("status: %d auto=%v", res.Code, st.Auto)
	}
}
```

- [ ] **Step 2: 失敗するテストを書く(オフラインの期限)**

`internal/app/syncing/auto_test.go` の `TestAutoWaitsWhileOfflineAndSyncsWhenBackOnline` の最後のかたまり(`a.Wrote("S1")` から関数の終わりまで)を次に置き換え、コメントの「オフラインの知らせは 30 秒で切れる」を「オフラインの知らせは 2 分で切れる(決定 S2)」に直す:

```go
	a.Wrote("S1")
	a.Network("tab-2", false)
	f.clock.Advance(time.Minute + 30*time.Second) // 見えないタブの間引き(1 分に 1 回)より長く持つ
	step(t, a)
	if n := f.syncs(); n["S1"] != 1 {
		t.Fatalf("offline expired too early: %v", n)
	}
	f.clock.Advance(31 * time.Second)
	step(t, a)
	if n := f.syncs(); n["S1"] != 2 {
		t.Errorf("expired offline still blocks: %v", n) // 書き込み後の契機(S1)で同期する
	}
}
```

- [ ] **Step 3: 落ちるのを確かめる**

Run: `go test ./internal/adapter/httpapi/syncing/ ./internal/app/syncing/ -run 'Status|Offline' -v`
Expected: FAIL(`st.Auto undefined` のコンパイルエラーと、`offline expired too early`)

- [ ] **Step 4: 実装する**

`internal/adapter/httpapi/syncing/dto.go` の `SyncStatusList` を次にする:

```go
// SyncStatusList は、同期の状態の一覧(サイドバーの並び。非表示の Store も含む)と、自動同期のオン・オフ。
type SyncStatusList struct {
	Auto   bool              `json:"auto" doc:"「裏で自動的に同期する」がオンか(オフラインの帯の文を選ぶ)"`
	Stores []SyncStoreStatus `json:"stores" nullable:"false"`
}
```

`internal/adapter/httpapi/syncing/syncing.go` の `list-sync-status` の処理を次にする:

```go
	}, func(ctx context.Context, _ *struct{}) (*statusOutput, error) {
		st, err := s.Sync.Status(ctx)
		if err != nil {
			return nil, err
		}
		timing, err := s.Auto.Settings.Timing(ctx)
		if err != nil {
			return nil, err
		}
		return &statusOutput{Body: toStatusList(st, timing.Auto)}, nil
	})
```

`toStatusList` を次にする(中の Store の詰め方は変えない):

```go
func toStatusList(st []appsyncing.StoreStatus, auto bool) SyncStatusList {
	out := SyncStatusList{Auto: auto, Stores: []SyncStoreStatus{}}
```

`internal/app/syncing/auto.go` の `holdFor` の下に足し、`Network` の `a.offline[tab] = now.Add(holdFor)` を `a.offline[tab] = now.Add(offlineFor)` にする:

```go
// offlineFor は、タブの「オフラインです」の知らせが有効な時間(C2 画面の決定 S2)。ブラウザは見えないタブの
// タイマーを 1 分に 1 回ほどに間引くので、待つ条件より長くする。
const offlineFor = 2 * time.Minute
```

`Network` のコメントの「(決定 C2-18b)」を「(決定 C2-18b。期限は C2 画面の決定 S2)」にする。

- [ ] **Step 5: 生成物を作り直す**

Run: `make generate && cd web && pnpm gen && cd ..`
Expected: `api/openapi.yaml` と `web/packages/api/src/generated.ts` の `SyncStatusList` に `auto` が入る。

- [ ] **Step 6: 通るのを確かめる**

Run: `make check`
Expected: PASS

- [ ] **Step 7: コミット**

```bash
git add internal/adapter/httpapi/syncing internal/app/syncing api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(adapter): 同期の状態に自動同期のオン・オフを足し、オフラインの知らせを 2 分持たせる(C2 画面 Task 1)"
```

---

### Task 2: 相対時間と 24 時間の判定

**Files:**
- Create: `web/packages/core/src/features/dates/relative.ts`
- Test: `web/packages/core/src/features/dates/relative.test.ts`
- Modify: `web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: `formatDay(ymd: string, thisYear: number): string`(`features/dates/date.ts`)
- Produces: `relativeTime(at: Date, now: Date): string`、`olderThanADay(at: Date, now: Date): boolean`

- [ ] **Step 1: 失敗するテストを書く**

`web/packages/core/src/features/dates/relative.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { olderThanADay, relativeTime } from "./relative";

// 時刻は、このマシンの時間帯の日付で表すので、テストは時間帯を持たない書き方(ローカル時刻)で作る。
const now = new Date(2026, 10, 13, 12, 0, 0);
const ago = (ms: number) => new Date(now.getTime() - ms);
const min = 60_000;
const hour = 60 * min;
const day = 24 * hour;

describe("relativeTime (01 の表)", () => {
  it.each([
    [0, "たった今"],
    [59_999, "たった今"],
    [min, "1 分前"],
    [59 * min, "59 分前"],
    [hour, "1 時間前"],
    [23 * hour + 59 * min, "23 時間前"],
    [day, "1 日前"],
    [6 * day, "6 日前"],
  ])("%d ms ago is %s", (ms, want) => {
    expect(relativeTime(ago(ms), now)).toBe(want);
  });

  it("shows the date from 7 days", () => {
    expect(relativeTime(new Date(2026, 10, 6, 12, 0, 0), now)).toBe("11/6");
    expect(relativeTime(new Date(2025, 11, 31, 9, 0, 0), now)).toBe("2025/12/31");
  });

  it("treats a time in the future as just now", () => {
    expect(relativeTime(new Date(now.getTime() + 5 * min), now)).toBe("たった今");
  });
});

describe("olderThanADay (06: 経過時間で判定する)", () => {
  it("is true from 24 hours", () => {
    expect(olderThanADay(ago(day - 1), now)).toBe(false);
    expect(olderThanADay(ago(day), now)).toBe(true);
  });
});
```

- [ ] **Step 2: 落ちるのを確かめる**

Run: `cd web/packages/core && pnpm vitest run src/features/dates/relative.test.ts`
Expected: FAIL(`Failed to resolve import "./relative"`)

- [ ] **Step 3: 実装する**

`web/packages/core/src/features/dates/relative.ts`:

```ts
import { formatDay } from "./date";

// 相対時間(01「日付と時刻」の表)。1 分未満は「たった今」、7 日以上は日付。未来の時刻は「たった今」にする。

const minute = 60_000;
const hour = 60 * minute;
const day = 24 * hour;

const pad = (n: number) => String(n).padStart(2, "0");

export function relativeTime(at: Date, now: Date): string {
  const elapsed = now.getTime() - at.getTime();
  if (elapsed < minute) return "たった今";
  if (elapsed < hour) return `${Math.floor(elapsed / minute)} 分前`;
  if (elapsed < day) return `${Math.floor(elapsed / hour)} 時間前`;
  if (elapsed < 7 * day) return `${Math.floor(elapsed / day)} 日前`;
  const ymd = `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}`;
  return formatDay(ymd, now.getFullYear());
}

// olderThanADay は、at から 24 時間以上たったか(06「24 時間以上同期されていません」)。
export function olderThanADay(at: Date, now: Date): boolean {
  return now.getTime() - at.getTime() >= day;
}
```

`web/packages/core/src/index.ts` の `export { formatDay } from "./features/dates/date";` の下に足す:

```ts
export { olderThanADay, relativeTime } from "./features/dates/relative";
```

- [ ] **Step 4: 通るのを確かめる**

Run: `cd web/packages/core && pnpm vitest run src/features/dates && pnpm check`
Expected: PASS。`formatDay` の 7 日以上の行が「11/6」「2025/12/31」にならないときは、`date.ts` の `formatDay` の綴りに合わせてテストの期待値でなく `relativeTime` の渡し方を直す(`formatDay` は 01 の書式の正)。

- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src/features/dates/relative.ts web/packages/core/src/features/dates/relative.test.ts web/packages/core/src/index.ts
git commit -m "feat(core): 相対時間と 24 時間の判定を足す(C2 画面 Task 2)"
```

---

### Task 3: 同期の型・口と、同期カードの文の組み立て

**Files:**
- Create: `web/packages/core/src/model/sync.ts`
- Create: `web/packages/core/src/port/sync.ts`
- Create: `web/packages/core/src/features/sync/card.ts`
- Test: `web/packages/core/src/features/sync/card.test.ts`
- Modify: `web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: `relativeTime`・`olderThanADay`(Task 2)
- Produces:

```ts
// model/sync.ts
export interface SyncStoreStatus { storeId: string; name: string; hidden: boolean; readOnly: boolean; error: string; unsent: number; lastSuccess: Date | null }
export interface SyncStatus { auto: boolean; stores: SyncStoreStatus[] }
export type SyncStoreResult = { result: "synced" | "skipped" | "failed"; message: string; mirrorWarning: string }
export type SyncAllResult = { result: "synced" | "failed" | "nothing"; lines: string[] }
export type SyncEventKind = "store" | "sync"
// port/sync.ts
export interface SyncPort { status(): Promise<SyncStatus>; syncStore(storeId: string): Promise<SyncStoreResult>; syncAll(): Promise<SyncAllResult>; hold(tab: string, hold: boolean): Promise<void>; network(tab: string, online: boolean): Promise<void>; watch(on: { open(): void; event(kind: SyncEventKind): void }, signal: AbortSignal): Promise<void> }
// features/sync/card.ts
export interface SyncRow { storeId: string; name: string; summary: string; time: string; stale: boolean }
export interface SyncCard { summary: string; error: boolean; last: string; stale: string | null; rows: SyncRow[] }
export function syncCard(status: SyncStatus | null, now: Date): SyncCard | null
export function hasSyncError(status: SyncStatus | null): boolean
export function offlineBand(auto: boolean): string
export const offlineSyncMessage: string
export function isSyncAllKey(e: { key: string; shiftKey: boolean; ctrlKey: boolean; metaKey: boolean; altKey: boolean }): boolean
export function syncAllHint(mac: boolean): string
```

- [ ] **Step 1: 型と口を書く(テストのための土台。振る舞いはない)**

`web/packages/core/src/model/sync.ts`:

```ts
// 同期の状態(06「同期の状態の表示」)。一覧は remote がある Store だけで、非表示の Store も含む。

export interface SyncStoreStatus {
  storeId: string;
  name: string;
  hidden: boolean;
  readOnly: boolean;
  // error は、同期エラーの短い理由(06「同期のエラー」の表の文言)。エラーでなければ空。
  error: string;
  unsent: number;
  // lastSuccess は、最後に成功した同期の時刻。1 度も成功していなければ null。
  lastSuccess: Date | null;
}

export interface SyncStatus {
  // auto は、「裏で自動的に同期する」がオンか(オフラインの帯の文を選ぶ)。
  auto: boolean;
  stores: SyncStoreStatus[];
}

// 手動の同期の結果(決定 C2-21)。文言はサーバーが作る。
export interface SyncStoreResult {
  result: "synced" | "skipped" | "failed";
  message: string;
  // mirrorWarning は、ミラーに送れなかったときの警告。なければ空。
  mirrorWarning: string;
}

export interface SyncAllResult {
  result: "synced" | "failed" | "nothing";
  lines: string[];
}

// 画面への反映の出来事の種類。store は Store の変化、sync は同期の状態の変化。
export type SyncEventKind = "store" | "sync";
```

`web/packages/core/src/port/sync.ts`:

```ts
import type { SyncAllResult, SyncEventKind, SyncStatus, SyncStoreResult } from "../model/sync";

// 同期の口。実装は api パッケージにある。存在しない Store の同期は NotFound を投げる。
export interface SyncPort {
  status(): Promise<SyncStatus>;
  syncStore(storeId: string): Promise<SyncStoreResult>;
  syncAll(): Promise<SyncAllResult>;
  // hold は、このタブが自動の同期を待たせる条件にあるかを知らせる(決定 C2-18)。
  hold(tab: string, hold: boolean): Promise<void>;
  // network は、このタブがネットワークにつながっているかを知らせる(決定 C2-18b)。
  network(tab: string, online: boolean): Promise<void>;
  // watch は、画面への反映の出来事を受け続ける。つながったら open を呼ぶ。流れが終わったら解決し、
  // つなげなければ投げる。signal で止める(止めたときは解決する)。
  watch(on: { open(): void; event(kind: SyncEventKind): void }, signal: AbortSignal): Promise<void>;
}
```

- [ ] **Step 2: 失敗するテストを書く**

`web/packages/core/src/features/sync/card.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { SyncStatus, SyncStoreStatus } from "../../model/sync";
import { hasSyncError, isSyncAllKey, offlineBand, syncAllHint, syncCard } from "./card";

const now = new Date(2026, 10, 13, 12, 0, 0);
const hoursAgo = (h: number) => new Date(now.getTime() - h * 3_600_000);

function store(over: Partial<SyncStoreStatus>): SyncStoreStatus {
  return {
    storeId: "S1",
    name: "team-a",
    hidden: false,
    readOnly: false,
    error: "",
    unsent: 0,
    lastSuccess: hoursAgo(1),
    ...over,
  };
}
const status = (...stores: SyncStoreStatus[]): SyncStatus => ({ auto: true, stores });

describe("syncCard (06「同期の状態の表示」)", () => {
  it("is absent without a visible store that has a remote", () => {
    expect(syncCard(null, now)).toBeNull();
    expect(syncCard(status(), now)).toBeNull();
    expect(syncCard(status(store({ hidden: true })), now)).toBeNull();
  });

  it("shows すべて最新 when nothing is unsent", () => {
    const c = syncCard(status(store({}), store({ storeId: "S2", name: "solo" })), now);
    expect(c).toMatchObject({ summary: "すべて最新", error: false, last: "最終同期 1 時間前 · Store 2 件", stale: null });
  });

  it("sums unsent saves", () => {
    const c = syncCard(status(store({ unsent: 2 }), store({ storeId: "S2", unsent: 3 })), now);
    expect(c?.summary).toBe("未送信 5");
  });

  it("counts sync errors before unsent, and does not count hidden stores", () => {
    const c = syncCard(
      status(
        store({ error: "認証に失敗しました", unsent: 2 }),
        store({ storeId: "S2", hidden: true, error: "認証に失敗しました" }),
      ),
      now,
    );
    expect(c).toMatchObject({ summary: "1 件の Store で同期エラー", error: true, last: "最終同期 1 時間前 · Store 1 件" });
  });

  it("uses the newest success and なし when no store has synced", () => {
    expect(syncCard(status(store({ lastSuccess: hoursAgo(5) }), store({ storeId: "S2", lastSuccess: hoursAgo(2) })), now)?.last).toBe(
      "最終同期 2 時間前 · Store 2 件",
    );
    expect(syncCard(status(store({ lastSuccess: null })), now)?.last).toBe("最終同期 なし · Store 1 件");
  });

  it("warns about stores not synced for 24 hours, even with an error (06 の例)", () => {
    const c = syncCard(status(store({ error: "認証に失敗しました", lastSuccess: hoursAgo(48) }), store({ storeId: "S2", lastSuccess: null })), now);
    expect(c?.summary).toBe("1 件の Store で同期エラー");
    expect(c?.stale).toBe("1 件の Store が 24 時間以上同期されていません");
  });

  it("lists the rows of 「Store ごとの同期状態」", () => {
    const c = syncCard(
      status(
        store({ error: "認証に失敗しました", lastSuccess: hoursAgo(2) }),
        store({ storeId: "S2", name: "solo", unsent: 3, lastSuccess: hoursAgo(30) }),
        store({ storeId: "S3", name: "docs", lastSuccess: null }),
        store({ storeId: "S4", name: "old", hidden: true }),
      ),
      now,
    );
    expect(c?.rows).toEqual([
      { storeId: "S1", name: "team-a", summary: "同期エラー: 認証に失敗しました", time: "2 時間前", stale: false },
      { storeId: "S2", name: "solo", summary: "未送信 3", time: "1 日前", stale: true },
      { storeId: "S3", name: "docs", summary: "最新", time: "未同期", stale: false },
    ]);
  });
});

describe("hasSyncError (860px 以下の点)", () => {
  it("counts visible stores only, read-only included", () => {
    expect(hasSyncError(null)).toBe(false);
    expect(hasSyncError(status(store({ readOnly: true, error: "認証に失敗しました" })))).toBe(true);
    expect(hasSyncError(status(store({ hidden: true, error: "認証に失敗しました" })))).toBe(false);
  });
});

describe("offlineBand (06「オフライン」)", () => {
  it("changes with the automatic sync setting", () => {
    expect(offlineBand(true)).toBe("オフラインです。変更はこのマシンにコミットし、つながったら push します。");
    expect(offlineBand(false)).toBe(
      "オフラインです。変更はこのマシンにコミットします。つながったら「すべて同期」で push してください。",
    );
  });
});

describe("isSyncAllKey (Shift+Ctrl+S、Mac は Shift+Cmd+S)", () => {
  const key = (over: Partial<Parameters<typeof isSyncAllKey>[0]>) => ({
    key: "S",
    shiftKey: true,
    ctrlKey: false,
    metaKey: false,
    altKey: false,
    ...over,
  });
  it("accepts Ctrl or Cmd with Shift", () => {
    expect(isSyncAllKey(key({ ctrlKey: true }))).toBe(true);
    expect(isSyncAllKey(key({ metaKey: true, key: "s" }))).toBe(true);
  });
  it("rejects other combinations", () => {
    expect(isSyncAllKey(key({ ctrlKey: true, shiftKey: false }))).toBe(false);
    expect(isSyncAllKey(key({}))).toBe(false);
    expect(isSyncAllKey(key({ ctrlKey: true, altKey: true }))).toBe(false);
    expect(isSyncAllKey(key({ ctrlKey: true, key: "a" }))).toBe(false);
  });
  it("names the shortcut for the platform", () => {
    expect(syncAllHint(true)).toBe("Shift+Cmd+S");
    expect(syncAllHint(false)).toBe("Shift+Ctrl+S");
  });
});
```

- [ ] **Step 3: 落ちるのを確かめる**

Run: `cd web/packages/core && pnpm vitest run src/features/sync/card.test.ts`
Expected: FAIL(`Failed to resolve import "./card"`)

- [ ] **Step 4: 実装する**

`web/packages/core/src/features/sync/card.ts`:

```ts
import type { SyncStatus, SyncStoreStatus } from "../../model/sync";
import { olderThanADay, relativeTime } from "../dates/relative";

// 同期カードと「Store ごとの同期状態」の文(06「同期の状態の表示」)。非表示の Store は数えない。

export interface SyncRow {
  storeId: string;
  name: string;
  // summary は「同期エラー: {短い理由}」「未送信 X」「最新」。
  summary: string;
  // time は、最後に成功した同期の相対時間か「未同期」。
  time: string;
  // stale は、最後の成功から 24 時間以上(相対時間を警告の色にする)。
  stale: boolean;
}

export interface SyncCard {
  summary: string;
  error: boolean;
  last: string;
  stale: string | null;
  rows: SyncRow[];
}

const visible = (status: SyncStatus | null): SyncStoreStatus[] =>
  status?.stores.filter((s) => !s.hidden) ?? [];

function rowOf(s: SyncStoreStatus, now: Date): SyncRow {
  let summary = s.unsent > 0 ? `未送信 ${s.unsent}` : "最新";
  if (s.error !== "") summary = `同期エラー: ${s.error}`;
  return {
    storeId: s.storeId,
    name: s.name,
    summary,
    time: s.lastSuccess ? relativeTime(s.lastSuccess, now) : "未同期",
    stale: s.lastSuccess !== null && olderThanADay(s.lastSuccess, now),
  };
}

function summaryOf(stores: SyncStoreStatus[]): { summary: string; error: boolean } {
  const errors = stores.filter((s) => s.error !== "").length;
  if (errors > 0) return { summary: `${errors} 件の Store で同期エラー`, error: true };
  const unsent = stores.reduce((n, s) => n + s.unsent, 0);
  return { summary: unsent > 0 ? `未送信 ${unsent}` : "すべて最新", error: false };
}

// syncCard は、カードの文。remote がある表示中の Store がなければ null(カードを出さない)。
export function syncCard(status: SyncStatus | null, now: Date): SyncCard | null {
  const stores = visible(status);
  if (stores.length === 0) return null;
  const times = stores.flatMap((s) => (s.lastSuccess ? [s.lastSuccess.getTime()] : []));
  const newest = times.length > 0 ? relativeTime(new Date(Math.max(...times)), now) : "なし";
  const rows = stores.map((s) => rowOf(s, now));
  const stale = rows.filter((r) => r.stale).length;
  return {
    ...summaryOf(stores),
    last: `最終同期 ${newest} · Store ${stores.length} 件`,
    stale: stale > 0 ? `${stale} 件の Store が 24 時間以上同期されていません` : null,
    rows,
  };
}

// hasSyncError は、860px 以下のメニューのボタンに点を付けるか。閲覧のみも数え、非表示は数えない。
export function hasSyncError(status: SyncStatus | null): boolean {
  return visible(status).some((s) => s.error !== "");
}

// offlineBand は、オフラインの帯の文(06「オフライン」)。
export function offlineBand(auto: boolean): string {
  return auto
    ? "オフラインです。変更はこのマシンにコミットし、つながったら push します。"
    : "オフラインです。変更はこのマシンにコミットします。つながったら「すべて同期」で push してください。";
}

// offlineSyncMessage は、オフラインで手動の同期をしたときのエラーのトースト。
export const offlineSyncMessage = "オフラインのため同期できません。つながったら同期します";

// isSyncAllKey は、「すべて同期」のキー(Shift+Ctrl+S、Mac は Shift+Cmd+S)か。
export function isSyncAllKey(e: {
  key: string;
  shiftKey: boolean;
  ctrlKey: boolean;
  metaKey: boolean;
  altKey: boolean;
}): boolean {
  return e.key.toLowerCase() === "s" && e.shiftKey && (e.ctrlKey || e.metaKey) && !e.altKey;
}

// syncAllHint は、「すべて同期」のボタンの案内。
export function syncAllHint(mac: boolean): string {
  return mac ? "Shift+Cmd+S" : "Shift+Ctrl+S";
}
```

`web/packages/core/src/index.ts` に足す(Biome が並べ替えを求めたら従う):

```ts
export {
  hasSyncError,
  isSyncAllKey,
  offlineBand,
  offlineSyncMessage,
  type SyncCard,
  type SyncRow,
  syncAllHint,
  syncCard,
} from "./features/sync/card";
export type {
  SyncAllResult,
  SyncEventKind,
  SyncStatus,
  SyncStoreResult,
  SyncStoreStatus,
} from "./model/sync";
export type { SyncPort } from "./port/sync";
```

- [ ] **Step 5: 通るのを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core exec vitest run src/features/sync && pnpm lint && pnpm check`
Expected: PASS

- [ ] **Step 6: コミット**

```bash
git add web/packages/core/src/model/sync.ts web/packages/core/src/port/sync.ts web/packages/core/src/features/sync/card.ts web/packages/core/src/features/sync/card.test.ts web/packages/core/src/index.ts
git commit -m "feat(core): 同期の型と口、同期カードとオフラインの帯の文を足す(C2 画面 Task 3)"
```

---

### Task 4: 警告のトーストと、手動の同期の流れ

**Files:**
- Modify: `web/packages/core/src/features/toasts/toasts.ts`
- Modify: `web/packages/core/src/features/toasts/toasts.test.ts`
- Create: `web/packages/core/src/features/sync/fixtures.test-util.ts`
- Create: `web/packages/core/src/features/sync/sync.ts`
- Test: `web/packages/core/src/features/sync/sync.test.ts`
- Modify: `web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: `SyncPort`・`SyncStatus`(Task 3)、`offlineSyncMessage`(Task 3)、`Toasts`
- Produces:

```ts
// toasts.ts(変更)
export interface ToastItem { id: string; text: string; error: boolean; warn: boolean }
show(text: string, opts?: { error?: boolean; warn?: boolean }): void
// sync.ts
export interface SyncDeps { port: SyncPort; toasts: Toasts; online(): boolean; onSynced(): void }
export interface SyncState { status: Readable<SyncStatus | null>; load(): Promise<void>; syncAll(): Promise<void>; syncStore(storeId: string): Promise<void> }
export function createSync(deps: SyncDeps): SyncState
// fixtures.test-util.ts
export function fakeSyncPort(over?: Partial<SyncPort>): SyncPort & { calls: string[] }
```

- [ ] **Step 1: 失敗するテストを書く(トースト)**

`web/packages/core/src/features/toasts/toasts.test.ts` の `describe("toasts", ...)` の中に足す。既存の `toEqual` の期待値に `warn: false` が要るようになるので、既存の期待値のオブジェクトにも `warn: false` を足す:

```ts
  it("shows a warning in 4.5 seconds like a normal toast (決定 S14)", () => {
    const { timer, advance } = fakeTimer();
    const toasts = createToasts(timer);
    toasts.show("ミラー 1 件に送れませんでした", { warn: true });
    expect(get(toasts.list)).toEqual([
      { id: "t1", text: "ミラー 1 件に送れませんでした", error: false, warn: true },
    ]);
    advance(4500);
    expect(get(toasts.list)).toEqual([]);
  });
```

- [ ] **Step 2: 落ちるのを確かめる**

Run: `cd web/packages/core && pnpm vitest run src/features/toasts`
Expected: FAIL(`warn` がない)

- [ ] **Step 3: トーストを直す**

`web/packages/core/src/features/toasts/toasts.ts` を直す:

```ts
export interface ToastItem {
  id: string;
  text: string;
  error: boolean;
  // warn は、警告の色で出す(消える時間は通常と同じ。C2 画面の決定 S14)。
  warn: boolean;
}
```

```ts
  show(text: string, opts?: { error?: boolean; warn?: boolean }): void;
```

```ts
      const toast: ToastItem = {
        id: `t${next}`,
        text,
        error: opts.error ?? false,
        warn: opts.warn ?? false,
      };
```

Run: `cd web/packages/core && pnpm vitest run src/features/toasts`
Expected: PASS

- [ ] **Step 4: 偽の口を書く**

`web/packages/core/src/features/sync/fixtures.test-util.ts`:

```ts
import type { SyncPort } from "../../port/sync";

// fakeSyncPort は、呼ばれた操作を calls に「名前 引数」で残す偽の SyncPort。over で応答を差し替える。
export function fakeSyncPort(over: Partial<SyncPort> = {}): SyncPort & { calls: string[] } {
  const calls: string[] = [];
  const port: SyncPort = {
    status: async () => ({ auto: true, stores: [] }),
    syncStore: async () => ({ result: "synced", message: "", mirrorWarning: "" }),
    syncAll: async () => ({ result: "synced", lines: [] }),
    hold: async () => {},
    network: async () => {},
    watch: async () => {},
    ...over,
  };
  return {
    calls,
    status: () => (calls.push("status"), port.status()),
    syncStore: (id) => (calls.push(`syncStore ${id}`), port.syncStore(id)),
    syncAll: () => (calls.push("syncAll"), port.syncAll()),
    hold: (tab, hold) => (calls.push(`hold ${tab} ${hold}`), port.hold(tab, hold)),
    network: (tab, online) => (calls.push(`network ${tab} ${online}`), port.network(tab, online)),
    watch: (on, signal) => (calls.push("watch"), port.watch(on, signal)),
  };
}
```

- [ ] **Step 5: 失敗するテストを書く(手動の同期)**

`web/packages/core/src/features/sync/sync.test.ts`:

```ts
import { describe, expect, it, vi } from "vitest";
import { get } from "../../reactive/store";
import { createToasts } from "../toasts/toasts";
import { fakeSyncPort } from "./fixtures.test-util";
import { createSync } from "./sync";

function setup(port = fakeSyncPort(), online = true) {
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const onSynced = vi.fn();
  const sync = createSync({ port, toasts, online: () => online, onSynced });
  const shown = () => get(toasts.list).map(({ text, error, warn }) => ({ text, error, warn }));
  return { port, sync, shown, onSynced };
}

describe("createSync", () => {
  it("loads the status", async () => {
    const at = new Date("2026-11-13T10:00:00Z");
    const { sync } = setup(
      fakeSyncPort({
        status: async () => ({
          auto: false,
          stores: [{ storeId: "S1", name: "team", hidden: false, readOnly: false, error: "", unsent: 1, lastSuccess: at }],
        }),
      }),
    );
    expect(get(sync.status)).toBeNull();
    await sync.load();
    expect(get(sync.status)?.stores[0]?.lastSuccess).toEqual(at);
  });

  it("keeps the last status when loading fails (A2 の決定 29)", async () => {
    let fail = false;
    const { sync } = setup(
      fakeSyncPort({
        status: async () => {
          if (fail) throw new Error("down");
          return { auto: true, stores: [] };
        },
      }),
    );
    await sync.load();
    fail = true;
    await sync.load();
    expect(get(sync.status)).toEqual({ auto: true, stores: [] });
  });

  it("shows one toast for 「すべて同期」 and reloads (決定 S12・S13)", async () => {
    const { sync, shown, onSynced, port } = setup(
      fakeSyncPort({
        syncAll: async () => ({
          result: "failed",
          lines: ["成功 1 件・失敗 1 件", "team: アカウントが設定されていません"],
        }),
      }),
    );
    await sync.syncAll();
    expect(shown()).toEqual([
      { text: "成功 1 件・失敗 1 件\nteam: アカウントが設定されていません", error: true, warn: false },
    ]);
    expect(onSynced).toHaveBeenCalledOnce();
    expect(port.calls).toEqual(["syncAll", "status"]);
  });

  it("shows a normal toast when there is nothing to sync", async () => {
    const { sync, shown } = setup(
      fakeSyncPort({
        syncAll: async () => ({
          result: "nothing",
          lines: ["同期する Store がありません(remote のある、表示中の Store が要ります)"],
        }),
      }),
    );
    await sync.syncAll();
    expect(shown()[0]).toMatchObject({ error: false });
  });

  it("shows the result of one store and the mirror warning after it", async () => {
    const { sync, shown } = setup(
      fakeSyncPort({
        syncStore: async () => ({
          result: "synced",
          message: "team を同期しました(送信 1 件・受信 2 件)",
          mirrorWarning: "ミラー 1 件に送れませんでした",
        }),
      }),
    );
    await sync.syncStore("S1");
    expect(shown()).toEqual([
      { text: "team を同期しました(送信 1 件・受信 2 件)", error: false, warn: false },
      { text: "ミラー 1 件に送れませんでした", error: false, warn: true },
    ]);
  });

  it("shows failed as an error and skipped as a normal toast", async () => {
    let result: "failed" | "skipped" = "failed";
    const { sync, shown } = setup(
      fakeSyncPort({ syncStore: async () => ({ result, message: result, mirrorWarning: "" }) }),
    );
    await sync.syncStore("S1");
    result = "skipped";
    await sync.syncStore("S1");
    expect(shown().map((t) => t.error)).toEqual([true, false]);
  });

  it("does not call the server while offline (決定 S15)", async () => {
    const { sync, shown, port, onSynced } = setup(fakeSyncPort(), false);
    await sync.syncAll();
    await sync.syncStore("S1");
    expect(port.calls).toEqual([]);
    expect(onSynced).not.toHaveBeenCalled();
    expect(shown()).toEqual([
      { text: "オフラインのため同期できません。つながったら同期します", error: true, warn: false },
      { text: "オフラインのため同期できません。つながったら同期します", error: true, warn: false },
    ]);
  });

  it("does not show a failed request (決定 S16)", async () => {
    const { sync, shown, onSynced } = setup(
      fakeSyncPort({
        syncAll: async () => {
          throw new Error("down");
        },
      }),
    );
    await sync.syncAll();
    expect(shown()).toEqual([]);
    expect(onSynced).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 6: 落ちるのを確かめる**

Run: `cd web/packages/core && pnpm vitest run src/features/sync/sync.test.ts`
Expected: FAIL(`Failed to resolve import "./sync"`)

- [ ] **Step 7: 実装する**

`web/packages/core/src/features/sync/sync.ts`:

```ts
import type { SyncStatus } from "../../model/sync";
import type { SyncPort } from "../../port/sync";
import { type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";
import { offlineSyncMessage } from "./card";

// 同期の状態と手動の同期(06「手動で同期する」「同期の結果」)。トーストの文言はサーバーが作る(決定 C2-21)。

export interface SyncDeps {
  port: SyncPort;
  toasts: Toasts;
  // online は、このタブがネットワークにつながっているか(navigator.onLine)。
  online(): boolean;
  // onSynced は、手動の同期が終わったあと(待たずに反映する。決定 S12)。
  onSynced(): void;
}

export interface SyncState {
  status: Readable<SyncStatus | null>;
  // load は、同期の状態を読み直す。失敗したら、いまの状態のまま(A2 の決定 29)。
  load(): Promise<void>;
  syncAll(): Promise<void>;
  syncStore(storeId: string): Promise<void>;
}

export function createSync(deps: SyncDeps): SyncState {
  const status = writable<SyncStatus | null>(null);
  let latest = 0;

  async function load() {
    latest += 1;
    const mine = latest;
    try {
      const next = await deps.port.status();
      if (mine === latest) status.set(next);
    } catch {
      // 通信の失敗は出さない(決定 S16)。
    }
  }

  // run は、オフラインなら送らずにトーストを出し、そうでなければ同期して、結果を出して読み直す。
  async function run(sync: () => Promise<void>) {
    if (!deps.online()) {
      deps.toasts.show(offlineSyncMessage, { error: true });
      return;
    }
    try {
      await sync();
    } catch {
      return; // 決定 S16
    }
    deps.onSynced();
    await load();
  }

  return {
    status: readonly(status),
    load,
    syncAll: () =>
      run(async () => {
        const r = await deps.port.syncAll();
        deps.toasts.show(r.lines.join("\n"), { error: r.result === "failed" });
      }),
    syncStore: (storeId) =>
      run(async () => {
        const r = await deps.port.syncStore(storeId);
        deps.toasts.show(r.message, { error: r.result === "failed" });
        if (r.mirrorWarning !== "") deps.toasts.show(r.mirrorWarning, { warn: true });
      }),
  };
}
```

`web/packages/core/src/index.ts` に足す:

```ts
export { createSync, type SyncDeps, type SyncState } from "./features/sync/sync";
```

- [ ] **Step 8: 通るのを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test && pnpm lint && pnpm check`
Expected: PASS。`pnpm check` で `ToastItem` の `warn` が ui の `Toast` 型と合わないと言われたら、Task 8 で ui に `warn?` を足すまで型の差はない(ui の `Toast.warn` は任意)ので、そのエラーは出ないはず。出たら内容を報告する。

- [ ] **Step 9: コミット**

```bash
git add web/packages/core/src/features/toasts web/packages/core/src/features/sync web/packages/core/src/index.ts
git commit -m "feat(core): 警告のトーストと、手動の同期の流れを足す(C2 画面 Task 4)"
```

---

### Task 5: 画面への反映(出来事の受け取り・溜める・つなぎ直し)と詳細パネルの読み直し

**Files:**
- Create: `web/packages/core/src/features/sync/reflection.ts`
- Test: `web/packages/core/src/features/sync/reflection.test.ts`
- Modify: `web/packages/core/src/features/items/detail.ts`
- Modify: `web/packages/core/src/features/items/detail.test.ts`
- Modify: `web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: `SyncPort.watch`(Task 3)、`Timer`(`features/toasts/toasts.ts`)、`fakeSyncPort`(Task 4)
- Produces:

```ts
export interface ReflectionDeps { port: SyncPort; timer: Timer; held(): boolean; visible(): boolean; apply(kinds: SyncEventKind[]): Promise<void> }
export interface Reflection { start(): () => void; now(): Promise<void>; wake(): void }
export function createReflection(deps: ReflectionDeps): Reflection
// detail.ts
Detail.reload(): Promise<void>
```

- [ ] **Step 1: 失敗するテストを書く(反映)**

`web/packages/core/src/features/sync/reflection.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { SyncEventKind } from "../../model/sync";
import type { SyncPort } from "../../port/sync";
import type { Timer } from "../toasts/toasts";
import { fakeSyncPort } from "./fixtures.test-util";
import { createReflection } from "./reflection";

// manualTimer は、時間を手で進める時計(toasts.test.ts の fakeTimer と同じ形)。
function manualTimer() {
  let now = 0;
  let seq = 0;
  const pending = new Map<number, { at: number; fn: () => void }>();
  const timer: Timer = {
    set(fn, ms) {
      seq += 1;
      pending.set(seq, { at: now + ms, fn });
      return seq;
    },
    clear: (h) => void pending.delete(h as number),
  };
  const advance = async (ms: number) => {
    now += ms;
    for (const [h, p] of [...pending].sort((a, b) => a[1].at - b[1].at)) {
      if (p.at <= now) {
        pending.delete(h);
        p.fn();
      }
    }
    await flush();
  };
  return { timer, advance };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

// stream は、watch の流れを外から動かす。open・send・end・fail で流れを進める。
function stream() {
  let on: Parameters<SyncPort["watch"]>[0] | undefined;
  let finish: (() => void) | undefined;
  let error: ((e: unknown) => void) | undefined;
  let connects = 0;
  const watch: SyncPort["watch"] = (o, signal) => {
    connects += 1;
    on = o;
    return new Promise<void>((resolve, reject) => {
      finish = resolve;
      error = reject;
      signal.addEventListener("abort", () => resolve());
    });
  };
  return {
    watch,
    connects: () => connects,
    open: () => on?.open(),
    send: (k: SyncEventKind) => on?.event(k),
    end: () => finish?.(),
    fail: () => error?.(new Error("down")),
  };
}

function setup() {
  const s = stream();
  const { timer, advance } = manualTimer();
  const applied: SyncEventKind[][] = [];
  let held = false;
  let visible = true;
  const r = createReflection({
    port: fakeSyncPort({ watch: s.watch }),
    timer,
    held: () => held,
    visible: () => visible,
    apply: async (kinds) => {
      applied.push(kinds);
    },
  });
  return {
    s,
    r,
    advance,
    applied,
    hold: (v: boolean) => {
      held = v;
    },
    show: (v: boolean) => {
      visible = v;
    },
  };
}

describe("createReflection (06「画面への反映」)", () => {
  it("reloads everything when it connects (決定 S11)", async () => {
    const t = setup();
    t.r.start();
    t.s.open();
    await t.advance(0);
    expect(t.applied).toEqual([["store", "sync"]]);
  });

  it("applies an event at once when nothing holds", async () => {
    const t = setup();
    t.r.start();
    t.s.open();
    await t.advance(0);
    t.s.send("store");
    await t.advance(0);
    expect(t.applied.at(-1)).toEqual(["store"]);
  });

  it("collects events while held and applies them once (決定 S10)", async () => {
    const t = setup();
    t.r.start();
    t.s.open();
    await t.advance(0);
    t.hold(true);
    t.s.send("store");
    t.s.send("sync");
    t.s.send("store");
    await t.advance(1000);
    expect(t.applied).toHaveLength(1);
    t.hold(false);
    await t.advance(500);
    expect(t.applied).toEqual([["store", "sync"], ["store", "sync"]]);
  });

  it("waits while the tab is not visible and applies on wake", async () => {
    const t = setup();
    t.r.start();
    t.s.open();
    await t.advance(0);
    t.show(false);
    t.s.send("sync");
    await t.advance(5000);
    expect(t.applied).toHaveLength(1);
    t.show(true);
    t.r.wake();
    await t.advance(0);
    expect(t.applied.at(-1)).toEqual(["sync"]);
  });

  it("applies now even while held (手動の同期。決定 S12)", async () => {
    const t = setup();
    t.r.start();
    t.s.open();
    await t.advance(0);
    t.hold(true);
    await t.r.now();
    expect(t.applied.at(-1)).toEqual(["store", "sync"]);
  });

  it("reconnects after 1 second, doubling up to 10 seconds", async () => {
    const t = setup();
    t.r.start();
    t.s.fail();
    await t.advance(999);
    expect(t.s.connects()).toBe(1);
    await t.advance(1);
    expect(t.s.connects()).toBe(2);
    t.s.fail();
    await t.advance(2000);
    expect(t.s.connects()).toBe(3);
    t.s.end();
    await t.advance(4000);
    expect(t.s.connects()).toBe(4);
    t.s.open(); // つながったら間隔を戻す
    t.s.end();
    await t.advance(1000);
    expect(t.s.connects()).toBe(5);
  });

  it("stops watching and reconnecting when stopped", async () => {
    const t = setup();
    const stop = t.r.start();
    stop();
    await t.advance(20_000);
    expect(t.s.connects()).toBe(1);
  });
});
```

- [ ] **Step 2: 落ちるのを確かめる**

Run: `cd web/packages/core && pnpm vitest run src/features/sync/reflection.test.ts`
Expected: FAIL(`Failed to resolve import "./reflection"`)

- [ ] **Step 3: 実装する(反映)**

`web/packages/core/src/features/sync/reflection.ts`:

```ts
import type { SyncEventKind } from "../../model/sync";
import type { SyncPort } from "../../port/sync";
import type { Timer } from "../toasts/toasts";

// 画面への反映(06「画面への反映」「反映を待つとき」。決定 S10〜S12)。出来事を種類ごとに溜め、待つ条件に
// なく、見えているときに 1 回で apply する。待っているあいだは 0.5 秒ごとに確かめる。

export interface ReflectionDeps {
  port: SyncPort;
  timer: Timer;
  // held は、このタブが待つ条件(ダイアログ・メニュー・入力欄のフォーカスほか)にあるか。
  held(): boolean;
  // visible は、このタブが見えているか。
  visible(): boolean;
  // apply は、溜めた種類の画面を読み直す。順は store、sync。
  apply(kinds: SyncEventKind[]): Promise<void>;
}

export interface Reflection {
  // start は、出来事を受け始め、止める関数を返す。
  start(): () => void;
  // now は、待つ条件を問わず、すべてをすぐに読み直す(手動の同期のあと)。
  now(): Promise<void>;
  // wake は、待つ条件が解けたかもしれないとき(見えるようになったときほか)に、すぐ確かめる。
  wake(): void;
}

const retryMs = 500;
const firstBackoff = 1000;
const maxBackoff = 10_000;
const order: SyncEventKind[] = ["store", "sync"];

export function createReflection(deps: ReflectionDeps): Reflection {
  const pending = new Set<SyncEventKind>();
  let retry: unknown;
  let applying = false;

  async function flush() {
    if (retry !== undefined) {
      deps.timer.clear(retry);
      retry = undefined;
    }
    if (pending.size === 0 || applying) return;
    if (deps.held() || !deps.visible()) {
      retry = deps.timer.set(() => void flush(), retryMs);
      return;
    }
    const kinds = order.filter((k) => pending.has(k));
    pending.clear();
    applying = true;
    try {
      await deps.apply(kinds);
    } finally {
      applying = false;
    }
    if (pending.size > 0) void flush();
  }

  function add(...kinds: SyncEventKind[]) {
    for (const k of kinds) pending.add(k);
    void flush();
  }

  return {
    start() {
      const abort = new AbortController();
      let backoff = firstBackoff;
      let again: unknown;
      const connect = async () => {
        try {
          await deps.port.watch(
            {
              open: () => {
                backoff = firstBackoff;
                add(...order);
              },
              event: (k) => add(k),
            },
            abort.signal,
          );
        } catch {
          // つなげなかった・切れた。下でつなぎ直す。
        }
        if (abort.signal.aborted) return;
        again = deps.timer.set(() => void connect(), backoff);
        backoff = Math.min(backoff * 2, maxBackoff);
      };
      void connect();
      return () => {
        abort.abort();
        if (again !== undefined) deps.timer.clear(again);
        if (retry !== undefined) deps.timer.clear(retry);
      };
    },
    async now() {
      for (const k of order) pending.delete(k);
      await deps.apply([...order]);
    },
    wake: () => void flush(),
  };
}
```

- [ ] **Step 4: 通るのを確かめる(反映)**

Run: `cd web/packages/core && pnpm vitest run src/features/sync/reflection.test.ts`
Expected: PASS

- [ ] **Step 5: 失敗するテストを書く(詳細パネルの読み直し)**

`web/packages/core/src/features/items/detail.test.ts` の `describe("createDetail", ...)` の中に足す。別の書き手の保存は、偽物の `items.all` の行を新しいオブジェクトに置き換えて作る(同じオブジェクトを書き換えると、開いているパネルの値も一緒に変わり、読み直しを確かめられない):

```ts
  it("reload reads the open item again (C2 画面の決定 S10)", async () => {
    const { detail, items, task } = await setup();
    await detail.open("S1", task.id);
    const i = items.all.findIndex((x) => x.id === task.id);
    items.all[i] = { ...task, title: "別の画面で変えたタイトル" };
    expect(get(detail.view)?.item.title).toBe("画面");
    await detail.reload();
    expect(get(detail.view)?.item.title).toBe("別の画面で変えたタイトル");
  });

  it("reload closes the panel when the item is gone (06「反映で変わる画面」)", async () => {
    const { detail, items, task } = await setup();
    await detail.open("S1", task.id);
    items.all.splice(
      items.all.findIndex((x) => x.id === task.id),
      1,
    );
    await detail.reload();
    expect(get(detail.view)).toBeNull();
  });

  it("reload does nothing when the panel is closed", async () => {
    const { detail } = await setup();
    await detail.reload();
    expect(get(detail.view)).toBeNull();
  });
```

2 つ目のテストは、偽物の `get` がないアイテムで `NotFound` を投げることに頼る。`fixtures.test-util.ts` の `find` が `NotFound` 以外を投げるなら、`NotFound` を投げるように直す(本物の口の約束: `port/items.ts`)。

- [ ] **Step 6: 落ちるのを確かめる**

Run: `cd web/packages/core && pnpm vitest run src/features/items/detail.test.ts`
Expected: FAIL(`detail.reload is not a function`)

- [ ] **Step 7: 実装する(詳細パネルの読み直し)**

`web/packages/core/src/features/items/detail.ts` の `Detail` に足す:

```ts
  // reload は、開いているアイテムを読み直す(画面への反映)。開いていなければ何もしない。
  // なくなっていたら、パネルを閉じる(06「反映で変わる画面」)。
  reload(): Promise<void>;
```

`createDetail` の返す値に足す:

```ts
    async reload() {
      const v = get(view);
      if (v) await load(v.store.id, v.item.id);
    },
```

`load` の `NotFound` 以外の失敗は投げる(既存のまま)。

`web/packages/core/src/index.ts` に足す:

```ts
export { createReflection, type Reflection, type ReflectionDeps } from "./features/sync/reflection";
```

- [ ] **Step 8: 通るのを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test && pnpm lint && pnpm check`
Expected: PASS

- [ ] **Step 9: コミット**

```bash
git add web/packages/core/src/features/sync/reflection.ts web/packages/core/src/features/sync/reflection.test.ts web/packages/core/src/features/items web/packages/core/src/index.ts
git commit -m "feat(core): 画面への反映を溜めて 1 回で読み直し、詳細パネルの読み直しを足す(C2 画面 Task 5)"
```

---

### Task 6: タブの知らせ(自動の同期を待たせる・オフライン)

**Files:**
- Create: `web/packages/core/src/features/sync/signals.ts`
- Test: `web/packages/core/src/features/sync/signals.test.ts`
- Modify: `web/packages/core/src/index.ts`

**Interfaces:**
- Consumes: `SyncPort.hold`・`SyncPort.network`(Task 3)、`fakeSyncPort`(Task 4)
- Produces:

```ts
export interface TabSignalsDeps { port: SyncPort; tab: string; held(): boolean; visible(): boolean; online(): boolean; now(): number }
export interface TabSignals { check(): void }
export function createTabSignals(deps: TabSignalsDeps): TabSignals
export const resendMs = 10_000
```

`check` は、app が 1 秒ごとと、フォーカス・見えるか・オンラインかが変わったときに呼ぶ。

- [ ] **Step 1: 失敗するテストを書く**

`web/packages/core/src/features/sync/signals.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { fakeSyncPort } from "./fixtures.test-util";
import { createTabSignals } from "./signals";

function setup() {
  const port = fakeSyncPort();
  const st = { held: false, visible: true, online: true, now: 0 };
  const s = createTabSignals({
    port,
    tab: "T",
    held: () => st.held,
    visible: () => st.visible,
    online: () => st.online,
    now: () => st.now,
  });
  return { port, st, s };
}

describe("hold (決定 S3・C2-18)", () => {
  it("sends hold when held and visible, resends every 10 seconds, and releases", () => {
    const { port, st, s } = setup();
    s.check();
    expect(port.calls).toEqual([]);
    st.held = true;
    s.check();
    st.now = 9_999;
    s.check();
    st.now = 10_000;
    s.check();
    st.held = false;
    s.check();
    s.check();
    expect(port.calls).toEqual(["hold T true", "hold T true", "hold T false"]);
  });

  it("releases at once when the tab is hidden, and does not send while hidden", () => {
    const { port, st, s } = setup();
    st.held = true;
    s.check();
    st.visible = false;
    s.check();
    st.now = 30_000;
    s.check();
    expect(port.calls).toEqual(["hold T true", "hold T false"]);
  });
});

describe("network (決定 S4・C2-18b)", () => {
  it("sends offline at once and every 10 seconds, even when hidden, and online once", () => {
    const { port, st, s } = setup();
    st.visible = false;
    st.online = false;
    s.check();
    st.now = 5_000;
    s.check();
    st.now = 10_000;
    s.check();
    st.online = true;
    s.check();
    st.now = 60_000;
    s.check();
    expect(port.calls).toEqual(["network T false", "network T false", "network T true"]);
  });

  it("does not send online when it has never been offline", () => {
    const { port, s } = setup();
    s.check();
    expect(port.calls).toEqual([]);
  });

  it("ignores a failed request and sends again on the next resend", async () => {
    const { st, s } = setup();
    const failing = fakeSyncPort({
      network: async () => {
        throw new Error("down");
      },
    });
    const t = createTabSignals({ port: failing, tab: "T", held: () => false, visible: () => true, online: () => false, now: () => st.now });
    t.check();
    await Promise.resolve();
    st.now = 10_000;
    t.check();
    expect(failing.calls).toEqual(["network T false", "network T false"]);
    expect(s).toBeDefined();
  });
});
```

- [ ] **Step 2: 落ちるのを確かめる**

Run: `cd web/packages/core && pnpm vitest run src/features/sync/signals.test.ts`
Expected: FAIL(`Failed to resolve import "./signals"`)

- [ ] **Step 3: 実装する**

`web/packages/core/src/features/sync/signals.ts`:

```ts
import type { SyncPort } from "../../port/sync";

// タブの知らせ(決定 C2-18・C2-18b、C2 画面の決定 S3・S4)。サーバーは知らせを期限で捨てるので、続くあいだは
// 10 秒ごとに送り直す。送る失敗は捨てる(次の送り直しで届く)。

export const resendMs = 10_000;

export interface TabSignalsDeps {
  port: SyncPort;
  tab: string;
  held(): boolean;
  visible(): boolean;
  online(): boolean;
  // now は、いまの時刻(ミリ秒)。
  now(): number;
}

export interface TabSignals {
  // check は、いまの状態を見て、要る知らせを送る。app が 1 秒ごとと、状態が変わったときに呼ぶ。
  check(): void;
}

// signal は、1 つの真偽の知らせ。on のあいだは 10 秒ごとに送り、off になったら 1 回送る。
function signal(send: (on: boolean) => Promise<void>) {
  let on = false;
  let sentAt = Number.NEGATIVE_INFINITY;
  return (next: boolean, now: number) => {
    if (next && (!on || now - sentAt >= resendMs)) {
      sentAt = now;
      void send(true).catch(() => {});
    } else if (!next && on) {
      void send(false).catch(() => {});
    }
    on = next;
  };
}

export function createTabSignals(deps: TabSignalsDeps): TabSignals {
  const hold = signal((on) => deps.port.hold(deps.tab, on));
  const offline = signal((on) => deps.port.network(deps.tab, !on));
  return {
    check() {
      const now = deps.now();
      hold(deps.visible() && deps.held(), now);
      offline(!deps.online(), now);
    },
  };
}
```

`web/packages/core/src/index.ts` に足す:

```ts
export {
  createTabSignals,
  resendMs,
  type TabSignals,
  type TabSignalsDeps,
} from "./features/sync/signals";
```

- [ ] **Step 4: 通るのを確かめる**

Run: `cd web && pnpm --filter @hedgehug/core test && pnpm lint && pnpm check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/core/src/features/sync/signals.ts web/packages/core/src/features/sync/signals.test.ts web/packages/core/src/index.ts
git commit -m "feat(core): 自動の同期を待たせる知らせとオフラインの知らせを足す(C2 画面 Task 6)"
```

---

### Task 7: api の同期の口と、出来事の読み続け

**Files:**
- Modify: `web/packages/api/src/fetcher.ts`
- Create: `web/packages/api/src/sync.ts`
- Test: `web/packages/api/src/sync.test.ts`
- Modify: `web/packages/api/src/index.ts`

**Interfaces:**
- Consumes: 生成物の `listSyncStatus`・`syncStore`・`syncAll`・`holdSync`・`syncNetwork`・`getWatchEventsUrl`(Task 1 で `auto` 入り)、`call`(`./errors`)、`SyncPort`(Task 3)
- Produces: `createSyncPort(): SyncPort`、`authHeader(): string`(fetcher.ts)

- [ ] **Step 1: 失敗するテストを書く**

`web/packages/api/src/sync.test.ts`:

```ts
import { NotFound } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createSyncPort } from "./sync";

afterEach(() => {
  vi.unstubAllGlobals();
});

function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async () =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

describe("createSyncPort", () => {
  it("reads the status with dates", async () => {
    respond(200, {
      auto: false,
      stores: [
        { store: "S1", name: "team", hidden: false, readOnly: true, error: "認証に失敗しました", unsent: 2, lastSuccess: "2026-11-13T10:00:00Z" },
        { store: "S2", name: "solo", hidden: true, readOnly: false, error: "", unsent: 0, lastSuccess: "" },
      ],
    });
    expect(await createSyncPort().status()).toEqual({
      auto: false,
      stores: [
        { storeId: "S1", name: "team", hidden: false, readOnly: true, error: "認証に失敗しました", unsent: 2, lastSuccess: new Date("2026-11-13T10:00:00Z") },
        { storeId: "S2", name: "solo", hidden: true, readOnly: false, error: "", unsent: 0, lastSuccess: null },
      ],
    });
  });

  it("syncs one store and turns 404 into NotFound", async () => {
    const fetch = respond(200, { result: "failed", message: "m", mirrorWarning: "" });
    expect(await createSyncPort().syncStore("S 1")).toEqual({ result: "failed", message: "m", mirrorWarning: "" });
    expect(fetch.mock.calls[0]?.[0]).toBe("/api/stores/S%201/sync");
    respond(404, { detail: "Store がありません" });
    await expect(createSyncPort().syncStore("S9")).rejects.toBeInstanceOf(NotFound);
  });

  it("sends hold and network", async () => {
    const fetch = respond(204, undefined);
    await createSyncPort().hold("T", true);
    await createSyncPort().network("T", false);
    const bodies = fetch.mock.calls.map((c) => JSON.parse(String((c[1] as RequestInit).body)));
    expect(bodies).toEqual([{ tab: "T", hold: true }, { tab: "T", online: false }]);
  });
});

describe("watch", () => {
  function streamOf(chunks: string[]) {
    const enc = new TextEncoder();
    return new ReadableStream<Uint8Array>({
      start(c) {
        for (const ch of chunks) c.enqueue(enc.encode(ch));
        c.close();
      },
    });
  }

  it("calls open, then each event, across chunk boundaries, with the token", async () => {
    const fetch = vi.fn(
      async () =>
        new Response(
          streamOf(["event: store\ndata: {\"store\":\"S1\"}\n\nevent: sy", "nc\ndata: {\"store\":\"S1\"}\n\n"]),
          { status: 200, headers: { "Content-Type": "text/event-stream" } },
        ),
    );
    vi.stubGlobal("fetch", fetch);
    setToken("tok");
    const seen: string[] = [];
    await createSyncPort().watch(
      { open: () => seen.push("open"), event: (k) => seen.push(k) },
      new AbortController().signal,
    );
    expect(seen).toEqual(["open", "store", "sync"]);
    const init = fetch.mock.calls[0]?.[1] as RequestInit;
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer tok");
  });

  it("throws when the server refuses", async () => {
    respond(401, undefined);
    await expect(
      createSyncPort().watch({ open: () => {}, event: () => {} }, new AbortController().signal),
    ).rejects.toThrow();
  });

  it("resolves when stopped", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (_url: string, init: RequestInit) => {
        throw Object.assign(new Error("aborted"), { name: "AbortError", signal: init.signal });
      }),
    );
    const abort = new AbortController();
    abort.abort();
    await expect(createSyncPort().watch({ open: () => {}, event: () => {} }, abort.signal)).resolves.toBeUndefined();
  });
});
```

- [ ] **Step 2: 落ちるのを確かめる**

Run: `cd web/packages/api && pnpm vitest run src/sync.test.ts`
Expected: FAIL(`Failed to resolve import "./sync"`)

- [ ] **Step 3: 実装する**

`web/packages/api/src/fetcher.ts` の `setToken` の下に足し、`fetcher` の中の `headers.set("Authorization", ...)` を `headers.set("Authorization", authHeader())` にする:

```ts
// authHeader は、API に付ける Authorization の値(出来事の読み続けも使う)。
export function authHeader(): string {
  return `Bearer ${token}`;
}
```

`web/packages/api/src/sync.ts`:

```ts
import type { SyncEventKind, SyncPort } from "@hedgehug/core";
import { call } from "./errors";
import { ApiError, authHeader } from "./fetcher";
import {
  getWatchEventsUrl,
  holdSync,
  listSyncStatus,
  syncAll,
  syncNetwork,
  syncStore,
} from "./generated";

// 同期の口の実装。出来事は、生成される watchEvents() が流れの終わりまで解決しないので、fetch で読み続ける
// (決定 C2-20、C2 の引き継ぎ)。

const kindOf = (name: string): SyncEventKind | null =>
  name === "store" || name === "sync" ? name : null;

async function watch(
  on: Parameters<SyncPort["watch"]>[0],
  signal: AbortSignal,
): Promise<void> {
  let res: Response;
  try {
    res = await fetch(getWatchEventsUrl(), {
      headers: { Authorization: authHeader(), Accept: "text/event-stream" },
      cache: "no-store",
      signal,
    });
  } catch (e) {
    if (signal.aborted) return;
    throw e;
  }
  if (!res.ok || !res.body) throw new ApiError(res.status, `${res.status} ${res.statusText}`);
  on.open();
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  let buf = "";
  let event = "";
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) return;
      buf += value;
      let nl = buf.indexOf("\n");
      while (nl >= 0) {
        const line = buf.slice(0, nl);
        buf = buf.slice(nl + 1);
        if (line.startsWith("event: ")) event = line.slice(7);
        if (line === "") {
          const k = kindOf(event);
          if (k) on.event(k);
          event = "";
        }
        nl = buf.indexOf("\n");
      }
    }
  } catch (e) {
    if (signal.aborted) return;
    throw e;
  }
}

export function createSyncPort(): SyncPort {
  return {
    status: () =>
      call(async () => {
        const body = await listSyncStatus();
        return {
          auto: body.auto,
          stores: body.stores.map((s) => ({
            storeId: s.store,
            name: s.name,
            hidden: s.hidden,
            readOnly: s.readOnly,
            error: s.error,
            unsent: s.unsent,
            lastSuccess: s.lastSuccess === "" ? null : new Date(s.lastSuccess),
          })),
        };
      }),
    syncStore: (storeId) =>
      call(async () => {
        const r = await syncStore(storeId);
        return { result: r.result, message: r.message, mirrorWarning: r.mirrorWarning };
      }),
    syncAll: () =>
      call(async () => {
        const r = await syncAll();
        return { result: r.result, lines: r.lines };
      }),
    hold: (tab, hold) => call(() => holdSync({ tab, hold })),
    network: (tab, online) => call(() => syncNetwork({ tab, online })),
    watch,
  };
}
```

404 は `call`(`./errors`)が `NotFound` にする。

`web/packages/api/src/index.ts` に足す:

```ts
export { createSyncPort } from "./sync";
```

- [ ] **Step 4: 通るのを確かめる**

Run: `cd web && pnpm --filter @hedgehug/api test && pnpm lint && pnpm check`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/packages/api/src
git commit -m "feat(api): 同期の口と、画面への反映の出来事を fetch で読み続ける処理を足す(C2 画面 Task 7)"
```

---

### Task 8: ui の同期カード・オフラインの帯・メニューのボタンの点・警告のトースト

**Files:**
- Create: `web/packages/ui/src/patterns/SyncCard.svelte`
- Create: `web/packages/ui/src/patterns/OfflineBand.svelte`
- Test: `web/packages/ui/src/patterns/sync.test.ts`
- Modify: `web/packages/ui/src/layout/PageHead.svelte`、`web/packages/ui/src/layout/layout.test.ts`
- Modify: `web/packages/ui/src/primitives/Toasts.svelte`、`web/packages/ui/src/primitives/overlays.test.ts`
- Modify: `web/packages/ui/src/primitives/Input.svelte`、`web/packages/ui/src/primitives/inputs.test.ts`
- Modify: `web/packages/ui/src/types.ts`、`web/packages/ui/src/index.ts`

**Interfaces:**
- Consumes: なし(ui は core を import しない。値は app が渡す)
- Produces:

```ts
// types.ts
export interface Toast { id: string; text: string; error?: boolean; warn?: boolean; action?: string }
export interface SyncCardView { summary: string; error: boolean; last: string; stale: string | null; rows: { storeId: string; name: string; summary: string; time: string; stale: boolean }[] }
// SyncCard.svelte props: { card: SyncCardView; hint: string; onsyncall: () => void; onsyncstore: (storeId: string) => void }
// OfflineBand.svelte props: { text: string }
// PageHead.svelte に alert?: boolean
// Input.svelte に onfocus?: (event: FocusEvent) => void
```

- [ ] **Step 1: 失敗するテストを書く**

`web/packages/ui/src/patterns/sync.test.ts`:

```ts
import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import type { SyncCardView } from "../types";
import OfflineBand from "./OfflineBand.svelte";
import SyncCard from "./SyncCard.svelte";

const card: SyncCardView = {
  summary: "1 件の Store で同期エラー",
  error: true,
  last: "最終同期 2 時間前 · Store 2 件",
  stale: "1 件の Store が 24 時間以上同期されていません",
  rows: [
    { storeId: "S1", name: "team-a", summary: "同期エラー: 認証に失敗しました", time: "2 時間前", stale: false },
    { storeId: "S2", name: "solo", summary: "未送信 3", time: "1 日前", stale: true },
  ],
};

describe("SyncCard", () => {
  it("shows the lines and the hint of 「すべて同期」", async () => {
    const onsyncall = vi.fn();
    render(SyncCard, { props: { card, hint: "Shift+Ctrl+S", onsyncall, onsyncstore: vi.fn() } });
    expect(screen.getByRole("button", { name: "1 件の Store で同期エラー" })).toBeInTheDocument();
    expect(screen.getByText("最終同期 2 時間前 · Store 2 件")).toBeInTheDocument();
    expect(screen.getByText("1 件の Store が 24 時間以上同期されていません")).toBeInTheDocument();
    const all = screen.getByRole("button", { name: "すべて同期" });
    expect(all).toHaveAttribute("title", "Shift+Ctrl+S");
    await fireEvent.click(all);
    expect(onsyncall).toHaveBeenCalledOnce();
  });

  it("opens 「Store ごとの同期状態」 and syncs the chosen store", async () => {
    const onsyncstore = vi.fn();
    render(SyncCard, { props: { card, hint: "", onsyncall: vi.fn(), onsyncstore } });
    await fireEvent.click(screen.getByRole("button", { name: "1 件の Store で同期エラー" }));
    const menu = screen.getByRole("menu", { name: "Store ごとの同期状態" });
    expect(menu).toHaveTextContent("team-a");
    expect(menu).toHaveTextContent("同期エラー: 認証に失敗しました · 2 時間前");
    await fireEvent.click(screen.getByRole("menuitem", { name: /solo/ }));
    expect(onsyncstore).toHaveBeenCalledWith("S2");
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("closes the menu with Escape", async () => {
    render(SyncCard, { props: { card, hint: "", onsyncall: vi.fn(), onsyncstore: vi.fn() } });
    await fireEvent.click(screen.getByRole("button", { name: "1 件の Store で同期エラー" }));
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });
});

describe("OfflineBand", () => {
  it("shows the text as a status", () => {
    render(OfflineBand, { props: { text: "オフラインです。" } });
    expect(screen.getByRole("status")).toHaveTextContent("オフラインです。");
  });
});
```

`web/packages/ui/src/layout/layout.test.ts` の `describe("PageHead", ...)` に足す:

```ts
  it("marks the drawer button with a dot while a sync error exists (決定 S19)", () => {
    const { rerender } = render(PageHead, { props: { title: "x", ondrawer: vi.fn() } });
    expect(screen.queryByRole("img", { name: "同期エラー" })).not.toBeInTheDocument();
    rerender({ title: "x", ondrawer: vi.fn(), alert: true });
    expect(screen.getByRole("img", { name: "同期エラー" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "メニュー" })).toBeInTheDocument();
  });
```

`web/packages/ui/src/primitives/overlays.test.ts` の末尾に足す(`Toasts` は既に import されている):

```ts
describe("Toasts warnings and lines (C2 画面の決定 S14)", () => {
  it("marks a warning and keeps the line breaks", () => {
    render(Toasts, {
      props: {
        toasts: [{ id: "t1", text: "成功 1 件・失敗 1 件\nteam: 認証に失敗しました", warn: true }],
        ondismiss: vi.fn(),
      },
    });
    const toast = screen.getByRole("status");
    expect(toast).toHaveClass("warn");
    expect(toast.textContent).toContain("成功 1 件・失敗 1 件\nteam: 認証に失敗しました");
  });
});
```

`web/packages/ui/src/primitives/inputs.test.ts` に足す(`Input` の import は既存のものを使う):

```ts
describe("Input onfocus", () => {
  it("reports focus", async () => {
    const onfocus = vi.fn();
    render(Input, { props: { label: "タイトル", onfocus } });
    await fireEvent.focus(screen.getByLabelText("タイトル"));
    expect(onfocus).toHaveBeenCalledOnce();
  });
});
```

- [ ] **Step 2: 落ちるのを確かめる**

Run: `cd web/packages/ui && pnpm vitest run`
Expected: FAIL(`SyncCard.svelte` がない、`alert`・`warn`・`onfocus` がない)

- [ ] **Step 3: 実装する**

`web/packages/ui/src/types.ts` の `Toast` を直し、末尾に足す:

```ts
export interface Toast {
  id: string;
  text: string;
  error?: boolean;
  // warn は、警告の色で出す。
  warn?: boolean;
  action?: string;
}
```

```ts
// SyncCardView は、同期カードに出す文(core の syncCard が作る)。
export interface SyncCardView {
  summary: string;
  error: boolean;
  last: string;
  stale: string | null;
  rows: { storeId: string; name: string; summary: string; time: string; stale: boolean }[];
}
```

`web/packages/ui/src/patterns/SyncCard.svelte`:

```svelte
<script lang="ts">
  import Button from "../primitives/Button.svelte";
  import type { SyncCardView } from "../types";

  // サイドバーの同期カード(06「同期の状態の表示」)。要約を押すと「Store ごとの同期状態」が開く。
  interface Props {
    card: SyncCardView;
    // hint は、「すべて同期」の案内(Shift+Ctrl+S か Shift+Cmd+S)。
    hint: string;
    onsyncall: () => void;
    onsyncstore: (storeId: string) => void;
  }

  let { card, hint, onsyncall, onsyncstore }: Props = $props();
  let open = $state(false);
  let element: HTMLDivElement | undefined = $state();

  function choose(storeId: string) {
    open = false;
    onsyncstore(storeId);
  }

  function onkeydown(event: KeyboardEvent) {
    if (open && event.key === "Escape") open = false;
  }

  function onpointerdown(event: PointerEvent) {
    if (open && element && !element.contains(event.target as Node)) open = false;
  }
</script>

<svelte:window {onkeydown} {onpointerdown} />

<div class="sync-card" bind:this={element}>
  <div class="row">
    <button
      type="button"
      class="summary"
      class:error={card.error}
      aria-haspopup="menu"
      aria-expanded={open}
      onclick={() => (open = !open)}>{card.summary}</button
    >
    <span title={hint}><Button size="sm" onclick={onsyncall}>すべて同期</Button></span>
  </div>
  <p class="line">{card.last}</p>
  {#if card.stale}
    <p class="line warn">{card.stale}</p>
  {/if}
  {#if open}
    <div class="menu" role="menu" aria-label="Store ごとの同期状態">
      {#each card.rows as row (row.storeId)}
        <button type="button" role="menuitem" class="item" onclick={() => choose(row.storeId)}>
          <span class="name">{row.name}</span>
          <span class="state">{row.summary} · <span class:warn={row.stale}>{row.time}</span></span>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .sync-card {
    position: relative;
    margin-top: auto;
    padding: 10px 8px;
    border-top: 1px solid var(--line);
    font-size: 12px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .summary {
    flex: 1;
    min-width: 0;
    text-align: left;
    border: 0;
    background: none;
    padding: 2px 0;
    color: var(--ink);
    font-weight: 500;
    cursor: pointer;
  }
  .summary.error {
    color: var(--danger);
  }
  .line {
    margin: 2px 0 0;
    color: var(--muted);
  }
  .warn {
    color: var(--c-amber-fg);
  }
  .menu {
    position: absolute;
    left: 8px;
    right: 8px;
    bottom: 100%;
    display: flex;
    flex-direction: column;
    padding: 4px;
    background: var(--raised);
    border: 1px solid var(--line);
    border-radius: 8px;
    box-shadow: var(--shadow-pop);
    z-index: 40;
  }
  .item {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    padding: 6px 8px;
    border: 0;
    border-radius: 6px;
    background: none;
    color: var(--ink);
    text-align: left;
    cursor: pointer;
  }
  .item:hover {
    background: var(--hover);
  }
  .name {
    font-weight: 500;
  }
  .state {
    color: var(--muted);
  }
</style>
```

色の変数は、`Menu.svelte`・`Toasts.svelte`・`ReadOnlyBand.svelte` が使っているもの(`--danger`・`--raised`・`--shadow-pop`・`--line`・`--hover`・`--muted`・`--ink`・`--c-amber-*`)だけを使う。新しい変数は作らない。

`web/packages/ui/src/patterns/OfflineBand.svelte`:

```svelte
<script lang="ts">
  // オフラインの帯(06「オフライン」)。文は core の offlineBand が作る。
  interface Props {
    text: string;
  }

  let { text }: Props = $props();
</script>

<p class="band" role="status">{text}</p>

<style>
  .band {
    margin: 0;
    padding: 8px 24px;
    background: var(--c-amber-bg);
    color: var(--c-amber-fg);
    font-size: 12.5px;
  }
</style>
```

`web/packages/ui/src/layout/PageHead.svelte` の `Props` に `alert?: boolean;` を足し、`let { title, ondrawer, crumbs, actions, alert = false }: Props = $props();` にして、`.drawer-btn` の中を次にする:

```svelte
  <span class="drawer-btn">
    <IconButton label="メニュー" onclick={ondrawer}>
      <svg class="ico" width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
        <path d="M4 6h16M4 12h16M4 18h16" />
      </svg>
    </IconButton>
    {#if alert}
      <span class="alert-dot" role="img" aria-label="同期エラー"></span>
    {/if}
  </span>
```

`<style>` に足し、`.drawer-btn` に `position: relative;` を足す:

```css
  .alert-dot {
    position: absolute;
    top: 4px;
    right: 4px;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--danger);
    pointer-events: none;
  }
```

`web/packages/ui/src/primitives/Toasts.svelte` の `.toast` の `div` に `class:warn={toast.warn}` を足し、`<style>` に足す(`.toast.error` の色の書き方に合わせる):

```css
  .toast span {
    white-space: pre-line;
  }
  .toast.warn {
    background: var(--c-amber-bg);
    color: var(--c-amber-fg);
  }
```

`web/packages/ui/src/primitives/Input.svelte` の `Props` に `onfocus?: (event: FocusEvent) => void;` を足し、分割代入に `onfocus` を足して、`<input>` に `{onfocus}` を付ける(`onblur` と同じ渡し方)。

`web/packages/ui/src/index.ts` に、ほかの patterns の書き方に合わせて足す:

```ts
export { default as OfflineBand } from "./patterns/OfflineBand.svelte";
export { default as SyncCard } from "./patterns/SyncCard.svelte";
```

`SyncCardView` も `types` から export する(既存の型の export の並びに足す)。

- [ ] **Step 4: 通るのを確かめる**

Run: `cd web && pnpm --filter @hedgehug/ui test && pnpm lint && pnpm check && pnpm size`
Expected: PASS(Svelte 200 行以内)

- [ ] **Step 5: コミット**

```bash
git add web/packages/ui/src
git commit -m "feat(ui): 同期カード・オフラインの帯・メニューのボタンの点・警告のトーストを足す(C2 画面 Task 8)"
```

---

### Task 9: app で組み立てる(カード・帯・点・Shift+Ctrl+S・知らせ・反映・タイトルの入力欄)

**Files:**
- Create: `web/app/src/tab.ts`
- Modify: `web/app/src/wire.ts`
- Modify: `web/app/src/App.svelte`
- Modify: `web/app/src/shell/Sidebar.svelte`
- Modify: `web/app/src/shell/Screen.svelte`
- Modify: `web/app/src/screens/project/ProjectScreen.svelte`
- Modify: `web/app/src/screens/workspace/WorkspaceScreen.svelte`
- Modify: `web/app/src/screens/item/DetailPanel.svelte`

app の Svelte と `wire.ts` には単体テストを置かない(A2 と同じ。振る舞いは core の単体テストと Task 10 の e2e で確かめる)。このタスクの「失敗するテスト」は Task 10 の e2e の一部を先に書いて使う。

**Interfaces:**
- Consumes: `createSync`(Task 4)、`createReflection`(Task 5)、`createTabSignals`(Task 6)、`createSyncPort`(Task 7)、`SyncCard`・`OfflineBand`・`PageHead.alert`・`Input.onfocus`(Task 8)、`syncCard`・`hasSyncError`・`offlineBand`・`isSyncAllKey`・`syncAllHint`(Task 3)、`detail.reload`(Task 5)
- Produces: `wire.ts` の `sync: SyncState`、`reflection: Reflection`、`syncNow: Readable<Date>`(30 秒ごと)、`online: Readable<boolean>`

- [ ] **Step 1: 先に e2e を 2 本書いて落ちるのを見る**

`web/e2e/sync.spec.ts` を作り、Task 10 の Step 1 のうち「the sync card shows the stores and syncs them all」と「Shift+Ctrl+S syncs while typing in the title」の 2 本だけを書く(コードは Task 10 の Step 1 のとおり)。

Run: `make e2e-bin && cd web && pnpm e2e sync.spec.ts`
Expected: FAIL(同期カードがない)

- [ ] **Step 2: DOM の関数を書く**

`web/app/src/tab.ts`:

```ts
// このタブの状態を、core に関数として渡す(core は DOM を知らない)。

// holdReasons は、自動の同期と画面への反映を待つ条件(06「自動で同期する」。C2 画面の決定 S9)。
// ドラッグ・説明やコメントの編集・書きかけは、それを作るトラックがここに足す。
export const holdReasons: (() => boolean)[] = [
  () => document.querySelector('[aria-modal="true"]') !== null,
  () => document.querySelector('[role="menu"]') !== null,
  () =>
    document.activeElement?.closest("input, textarea, select, [contenteditable='true']") != null,
];

export const held = () => holdReasons.some((r) => r());
export const visible = () => document.visibilityState === "visible";
export const online = () => navigator.onLine;
export const isMac = () => /Mac/.test(navigator.platform || navigator.userAgent);
export const tabId = crypto.randomUUID();
```

- [ ] **Step 3: 組み立てる**

`web/app/src/wire.ts` の import に、api の `createSyncPort` と、core の `createReflection`・`createSync`・`createTabSignals`・`readonly`・`writable`・`type SyncEventKind` を足す(`readonly`・`writable` は core の index が既に export している)。次のコードは、`detail` と `projectScreen` を作ったあと(ファイルの末尾の `void catalog.load()...` の前)に足す:

```ts
import { held, online as isOnline, tabId, visible } from "./tab";

const syncPort = createSyncPort();

// 画面への反映(06「画面への反映」)。store は一覧・開いているプロジェクト・詳細パネル、sync は同期の状態。
// 失敗は出さない(A2 の決定 29)。
async function reflect(kinds: SyncEventKind[]) {
  const jobs: Promise<unknown>[] = [];
  if (kinds.includes("store")) {
    jobs.push(catalog.load().then(() => Promise.all([projectScreen.reload(), detail.reload()])));
  }
  if (kinds.includes("sync")) jobs.push(sync.load());
  await Promise.allSettled(jobs);
}

export const reflection = createReflection({
  port: syncPort,
  timer: { set: (fn, ms) => setTimeout(fn, ms), clear: (h) => clearTimeout(h as ReturnType<typeof setTimeout>) },
  held,
  visible,
  apply: reflect,
});

export const sync = createSync({
  port: syncPort,
  toasts,
  online: isOnline,
  onSynced: () => void reflection.now(),
});

const signals = createTabSignals({
  port: syncPort,
  tab: tabId,
  held,
  visible,
  online: isOnline,
  now: () => Date.now(),
});

// syncNow は、相対時間と 24 時間の判定に使う時刻。30 秒ごとに進める(決定 S7)。
const now = writable(new Date());
export const syncNow = readonly(now);
const onlineState = writable(isOnline());
export const online = readonly(onlineState);

// startSync は、出来事の受け取り・知らせ・時刻の更新を始める。App が 1 回呼ぶ。
export function startSync() {
  reflection.start();
  void sync.load();
  const recheck = () => {
    signals.check();
    reflection.wake();
  };
  setInterval(() => signals.check(), 1000);
  setInterval(() => now.set(new Date()), 30_000);
  document.addEventListener("visibilitychange", recheck);
  document.addEventListener("focusout", () => setTimeout(recheck, 0));
  document.addEventListener("focusin", () => signals.check());
  const network = () => {
    onlineState.set(isOnline());
    recheck();
  };
  window.addEventListener("online", network);
  window.addEventListener("offline", network);
}
```

- [ ] **Step 4: 画面に出す**

`web/app/src/shell/Sidebar.svelte`: import に `hasSyncError` は要らない。次を足す:

```svelte
<script lang="ts">
  // (既存の import に足す)
  import { syncAllHint, syncCard } from "@hedgehug/core";
  import { SyncCard } from "@hedgehug/ui";
  import { isMac } from "../tab";
  import { sync, syncNow } from "../wire";

  const status = sync.status;
  const card = $derived(syncCard($status, $syncNow));
  const hint = syncAllHint(isMac());
</script>
```

ファイルの Store ごとのブロックの `{/each}` のあとに足す:

```svelte
{#if card}
  <SyncCard
    {card}
    {hint}
    onsyncall={() => void sync.syncAll()}
    onsyncstore={(id) => void sync.syncStore(id)}
  />
{/if}
```

`$derived` を使うので、既存の `$props()` の宣言と並べる。`<script>` を 2 つにせず、既存の `<script>` に足す。

`web/app/src/App.svelte`:

```svelte
<script lang="ts">
  // (既存の import に足す)
  import { hasSyncError, isSyncAllKey, offlineBand, syncCard } from "@hedgehug/core";
  import { OfflineBand } from "@hedgehug/ui";
  import { online, startSync, sync, syncNow } from "./wire";

  const syncStatus = sync.status;
  const isOnline = online;
  startSync();

  // オフラインの帯は、同期カードを出す条件のときだけ(06「オフライン」。決定 S20)。
  const band = $derived(
    !$isOnline && syncCard($syncStatus, $syncNow) ? offlineBand($syncStatus?.auto ?? true) : null,
  );
  const alert = $derived(hasSyncError($syncStatus));

  // Shift+Ctrl+S(Shift+Cmd+S)は、入力中とダイアログを開いているときも使える(06。決定 S17)。
  function onkeydowncapture(event: KeyboardEvent) {
    if (!isSyncAllKey(event)) return;
    event.preventDefault();
    event.stopPropagation();
    void sync.syncAll();
  }
</script>

<svelte:window {onkeydown} {onkeydowncapture} />
```

Svelte 5 の `on<event>capture` の属性で、capture の段階で受ける(詳細パネルの入力欄やダイアログが先に止めても届く)。

`<AppFrame>` の中の `<Screen>` の前に帯を置き、`Screen` に `alert` を渡す:

```svelte
  {#if band}
    <OfflineBand text={band} />
  {/if}
  <Screen current={$current} stores={$stores} version={$version} {alert} ondrawer={shell.openDrawer} />
```

`web/app/src/shell/Screen.svelte` の `Props` に `alert: boolean;` を足し、`ProjectScreen`・`WorkspaceScreen`・ホームの `PageHead` に `{alert}` を渡す。`ProjectScreen.svelte`・`WorkspaceScreen.svelte` の `Props` に `alert: boolean;` を足し、`<PageHead ... {ondrawer} {alert}>` にする。

- [ ] **Step 5: タイトルの入力欄を守る**

`web/app/src/screens/item/DetailPanel.svelte` の `title` の `$effect` を次にする(決定 S12):

```svelte
  // タイトルの入力欄。開いたアイテムが変わったら、そのタイトルに戻す。同じアイテムの読み直し(画面への
  // 反映)では、入力欄にフォーカスがあるあいだは書き換えない(06「編集中の入力欄の内容は書き換えない」)。
  let title = $state("");
  let editing = $state(false);
  let shownId: string | undefined;
  $effect(() => {
    const id = $view?.item.id;
    const next = $view?.item.title ?? "";
    if (id !== shownId || !editing) title = next;
    shownId = id;
  });
```

`<Input ...>` に `onfocus={() => (editing = true)}` を足し、`onblur` を `onblur={() => { editing = false; void save(); }}` にする。

- [ ] **Step 6: 通るのを確かめる**

Run: `cd web && pnpm lint && pnpm check && pnpm test && pnpm deps && pnpm size && cd .. && make e2e-bin && cd web && pnpm e2e sync.spec.ts`
Expected: PASS(2 本)

- [ ] **Step 7: コミット**

```bash
git add web/app/src web/e2e/sync.spec.ts
git commit -m "feat(app): 同期カード・オフラインの帯・点・Shift+Ctrl+S・タブの知らせ・画面への反映を組み立てる(C2 画面 Task 9)"
```

---

### Task 10: 通しのテスト

**Files:**
- Modify: `web/e2e/sync.spec.ts`

e2e のサーバーは、Store が 2 つ(e2e・readonly)とも remote を持ち、同期は偽物(結果は送信 0 件・受信 0 件)、自動同期はオフ(決定 S21)。3 つの幅が 1 つのサーバーを共有するので、サーバーの同期の状態に頼るのは「同期すると成功する」ことだけにし、エラー・24 時間・オンの文は応答を差し替えて確かめる。

**Interfaces:**
- Consumes: `tokenUrl`・`sidebar`・`unique`(`e2e/helpers.ts`)

- [ ] **Step 1: テストを書く**

`web/e2e/sync.spec.ts`:

```ts
import { expect, type Page, test } from "@playwright/test";
import { sidebar, tokenUrl, unique } from "./helpers";

// status は、GET /api/sync の応答を差し替える(決定 S21)。
async function status(page: Page, body: unknown) {
  await page.route("**/api/sync", (route) =>
    route.request().method() === "GET" ? route.fulfill({ json: body }) : route.fallback(),
  );
}

const hoursAgo = (h: number) => new Date(Date.now() - h * 3_600_000).toISOString();

// goOffline は、navigator.onLine を差し替えて offline の出来事を送る(setOffline は localhost も止める)。
async function goOffline(page: Page) {
  await page.evaluate(() => {
    Object.defineProperty(navigator, "onLine", { configurable: true, get: () => false });
    window.dispatchEvent(new Event("offline"));
  });
}

test("the sync card shows the stores and syncs them all", async ({ page }, info) => {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await expect(side.getByText(/最終同期 .+ · Store 2 件/)).toBeVisible();
  const all = side.getByRole("button", { name: "すべて同期" });
  await expect(all.locator("..")).toHaveAttribute("title", /Shift\+(Ctrl|Cmd)\+S/);
  await all.click();
  await expect(page.getByRole("status").filter({ hasText: "2 件の Store を同期しました" })).toBeVisible();
  await expect(side.getByText("すべて最新")).toBeVisible();
});

test("Shift+Ctrl+S syncs while typing in the title", async ({ page }) => {
  await page.goto(tokenUrl("/s/e2e/i/HH-2"));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  const title = panel.getByRole("textbox", { name: "タイトル" });
  await title.fill("入力中のタイトル");
  await page.keyboard.press("Shift+Control+S");
  await expect(page.getByRole("status").filter({ hasText: "2 件の Store を同期しました" })).toBeVisible();
  await expect(title).toHaveValue("入力中のタイトル");
  await expect(title).toBeFocused();
});

test("「Store ごとの同期状態」 syncs one store", async ({ page }, info) => {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await side.getByRole("button", { name: /すべて最新|未送信/ }).click();
  const menu = page.getByRole("menu", { name: "Store ごとの同期状態" });
  await menu.getByRole("menuitem", { name: /^readonly/ }).click();
  await expect(
    page.getByRole("status").filter({ hasText: "readonly を同期しました(送信 0 件・受信 0 件)" }),
  ).toBeVisible();
});

test("a sync error shows on the card, the menu and the drawer button", async ({ page }, info) => {
  await status(page, {
    auto: true,
    stores: [
      { store: "S1", name: "team-a", hidden: false, readOnly: false, error: "認証に失敗しました", unsent: 0, lastSuccess: hoursAgo(48) },
      { store: "S2", name: "solo", hidden: false, readOnly: false, error: "", unsent: 3, lastSuccess: hoursAgo(1) },
    ],
  });
  await page.goto(tokenUrl());
  if (info.project.name !== "desktop") {
    await expect(page.getByRole("img", { name: "同期エラー" })).toBeVisible();
  }
  const side = await sidebar(page, info);
  await expect(side.getByRole("button", { name: "1 件の Store で同期エラー" })).toBeVisible();
  await expect(side.getByText("1 件の Store が 24 時間以上同期されていません")).toBeVisible();
  await side.getByRole("button", { name: "1 件の Store で同期エラー" }).click();
  await expect(page.getByRole("menu", { name: "Store ごとの同期状態" })).toContainText(
    "同期エラー: 認証に失敗しました · 2 日前",
  );
});

test("a failed sync of all shows the lines in one error toast", async ({ page }, info) => {
  await page.route("**/api/sync", (route) =>
    route.request().method() === "POST"
      ? route.fulfill({ json: { result: "failed", lines: ["成功 1 件・失敗 1 件", "e2e: 認証に失敗しました"] } })
      : route.fallback(),
  );
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await side.getByRole("button", { name: "すべて同期" }).click();
  const toast = page.getByRole("status").filter({ hasText: "成功 1 件・失敗 1 件" });
  await expect(toast).toContainText("e2e: 認証に失敗しました");
  await expect(toast).toHaveClass(/error/);
});

test("offline shows the band, tells the server and refuses a manual sync", async ({ page }, info) => {
  const sent: unknown[] = [];
  await page.route("**/api/sync/network", async (route) => {
    sent.push(route.request().postDataJSON());
    await route.fulfill({ status: 204 });
  });
  await page.goto(tokenUrl());
  await goOffline(page);
  await expect(
    page.getByText("オフラインです。変更はこのマシンにコミットします。つながったら「すべて同期」で push してください。"),
  ).toBeVisible();
  await expect.poll(() => sent.length).toBeGreaterThan(0);
  expect(sent[0]).toMatchObject({ online: false });
  const side = await sidebar(page, info);
  await side.getByRole("button", { name: "すべて同期" }).click();
  await expect(
    page.getByRole("status").filter({ hasText: "オフラインのため同期できません。つながったら同期します" }),
  ).toBeVisible();
  await page.evaluate(() => {
    Object.defineProperty(navigator, "onLine", { configurable: true, get: () => true });
    window.dispatchEvent(new Event("online"));
  });
  await expect(page.getByText(/^オフラインです。/)).toBeHidden();
  await expect.poll(() => sent.at(-1)).toMatchObject({ online: true });
});

test("the band follows the automatic sync setting", async ({ page }) => {
  await status(page, {
    auto: true,
    stores: [{ store: "S1", name: "team-a", hidden: false, readOnly: false, error: "", unsent: 0, lastSuccess: "" }],
  });
  await page.goto(tokenUrl());
  await goOffline(page);
  await expect(page.getByText("オフラインです。変更はこのマシンにコミットし、つながったら push します。")).toBeVisible();
});

test("a change in another tab shows within a few seconds", async ({ page, context }, info) => {
  await page.goto(tokenUrl("/s/e2e/w/HH"));
  const side = await sidebar(page, info);
  const other = await context.newPage();
  await other.goto(tokenUrl("/s/e2e/w/HH"));
  const name = unique(info, "反映のプロジェクト");
  await other.getByRole("button", { name: "プロジェクト", exact: true }).click();
  const dialog = other.getByRole("dialog", { name: "プロジェクトを作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(name);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(dialog).toBeHidden();
  await expect(side.getByRole("link", { name })).toBeVisible({ timeout: 3000 });
});

test("a held tab tells the server and waits to apply", async ({ page }) => {
  const holds: unknown[] = [];
  await page.route("**/api/sync/hold", async (route) => {
    holds.push(route.request().postDataJSON());
    await route.fulfill({ status: 204 });
  });
  await page.goto(tokenUrl("/s/e2e/i/HH-2"));
  const title = page.getByRole("region", { name: "詳細パネル" }).getByRole("textbox", { name: "タイトル" });
  await title.focus();
  await expect.poll(() => holds.at(-1)).toMatchObject({ hold: true });
  await title.blur();
  await expect.poll(() => holds.at(-1)).toMatchObject({ hold: false });
});
```

「a change in another tab」の作成の手順は、`web/e2e/project.spec.ts` の「creates a project from the workspace screen」と同じ。`page` 側はドロワーを開いたまま(860px 以下)サイドバーを見る。

「the sync card shows the stores」の「すべて最新」は、偽物の `Unsent` が 0 を返すので成り立つ。

- [ ] **Step 2: 走らせる**

Run: `make e2e-bin && cd web && pnpm e2e`
Expected: すべて PASS(3 つの幅)。落ちたテストは、テストの期待値を仕様の文言から変えずに、実装を直す。セレクタの食い違い(既存の画面のボタン名など)だけはテストを直す。

- [ ] **Step 3: 全体を確かめる**

Run: `make check`
Expected: PASS

- [ ] **Step 4: コミット**

```bash
git add web/e2e/sync.spec.ts
git commit -m "test(e2e): 同期カード・オフライン・Shift+Ctrl+S・画面への反映を通しで確かめる(C2 画面 Task 10)"
```

---

## C2 画面のあと

- 作らなかった入口(「範囲」の表)は、それぞれのトラックが、`sync.syncAll`・`sync.syncStore` を呼んで足す。トーストと読み直しは `createSync` が持つので、入口は呼ぶだけでよい。
- 待つ条件(ドラッグ・説明やコメント・書きかけ)は、`web/app/src/tab.ts` の `holdReasons` に足す。
- G: `port.SyncTargets` の本物が入ると、本物のアプリにも同期カードが出る。アカウントの割り当て・非表示・remote が変わったときに `EventSync` を流せば、カードは `sync` の出来事で読み直す(C2 の引き継ぎ)。
- C3: 他の書き手との重なりのトーストは、反映(`reflect`)とは別に、保存の応答から出す。
- C5: 時計のずれの行は、`syncCard` に行を 1 つ足す。

## 実装で計画から変えたこと

(実装のあとで書く)
