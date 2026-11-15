# hedgehug トラック C5d(時計のずれの警告)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** GitHub の応答の日時とこのマシンの時計を比べ、5 分以上ずれていたら、設定の「一般」タブとサイドバーの同期カードに警告を出す。5 分未満に戻ったら消す。

**Architecture:** ずれは 2 つの通信で測る。`infra/github.Client` の既存の `OnSkew`(REST・デバイスフロー)と、`infra/gitstore` の git の HTTP 通信(同期・ミラー)の応答の `Date` である。測ったずれは `app/syncing.Skew` が受け取り、`domain/syncstatus.SkewMinutes` で警告の分に直す。値が変わったときだけ、state.db の表 `clock_skew` に書いて出来事 `sync` を出す。`GET /api/sync` は警告の分を `clockSkew` として返す。画面は core の `syncCard`・`clockWarning` で文を作り、同期カードと、新しく作る設定の「一般」タブに出す。

**Tech Stack:** Go 1.27.3(go-git v5、modernc.org/sqlite、huma v2)。web は Svelte 5・TypeScript・Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`
  - `:130`(同期カードの警告の行)
  - `:138-145`(「時計のずれの警告」)
  - `:189`(後勝ちは保存時刻に頼る。補正しない)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/08-stores-maintenance.md`
  - `:7-15`(設定のタブの並び。「一般」が先頭)
  - `:23`(「一般」のログイン名の下の警告。GitHub に問い合わせたことがないマシンには出さない)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/07-github.md` `:162`
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` `:286`(時計のずれは state.db に置く)
- `docs/superpowers/plans/2026-11-12-hedgehug-track-d.md` の決定 3(`infra/github` は `OnSkew` で知らせるだけ。記録と表示はトラック C)
- `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md` の C5d の行(`:79`)と未決の点 2(`:2079`)

## Global Constraints

土台・トラック C の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限は `internal/archtest/size.go` のとおり。
  - Go ファイルは 300 行、テストは 600 行まで。関数は 60 行、認知的複雑度は 15 まで。
  - パッケージのテスト以外のファイルは 20 個まで。`internal/app/port`・`internal/app/apptest`・`internal/infra/gitstore`(直下)は 20 個で満杯なので、ファイルを足さない。
  - 口は `port/clock.go`、偽物は `apptest/clock.go`、gitstore の測定は `sync_transport.go` に足す。
  - 足すファイルは `infra/statedb/skew.go`・`app/syncing/skew.go`・`cmd/hedgehug/skew_e2e.go`・`web/app/src/screens/settings/GeneralSection.svelte` と、それぞれのテストだけ。
- app の機能どうしは import しない。
- `port` の型の名前とコメントに「コミット」「ブランチ」「マージ」を書かない。
- e2e だけで使う入口は `//go:build e2e` のファイルに置く。本物の組み立てだけで使う関数は `//go:build !e2e` のファイル(`wire.go`)に置く。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 文言({N} は分の数。符号は付けない):
  - 設定「一般」(06 `:143`・08 `:23`): 「このマシンの時計が GitHub の時刻より {N} 分進んでいます。同期で後から届いた変更の勝ち負けと、削除済みを完全に削除する日が、他の人とずれます。OS の日時の設定を確かめてください。」。遅れているときは「進んで」を「遅れて」にする。
  - 同期カード(06 `:130`・`:144`): 「このマシンの時計が {N} 分ずれています」
- 警告の色は、同期カードの「24 時間以上同期されていません」の行と同じ(`SyncCard.svelte` の `.warn`)。
- 手元の検証は 1 回で足りる(CI でも流れる)。`make check`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)・`make e2e`。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| C5d-1 | 設定に「一般」タブを足し、時計のずれの警告だけを置く。ずれがなければ中は空。ログイン名と同期のタイミングは、トラック G がこのタブの警告の前後に足す(警告はログイン名の下、同期のタイミングの上)。 | ユーザーの決定(2026-11-15)。08 `:11` の「一般」の画面は G の持ち場(D4a の計画 `:15`) |
| C5d-2 | タブの並びは「一般」「アカウントとメンバー」。`/settings`(タブなし)・サイドバーの歯車・パレットの「設定を開く」の開く先は、今のまま「アカウントとメンバー」にする。パレットに「設定: 一般」は足さない。 | 08 `:9-15` の並び。開く先のタブは仕様に書かれていない。中が空のタブを開いた先にしないため。パレットの設定の群(02 `:634`)に「一般」はない G1(G1-13)で、中身が入ったので開く先を「一般」に変えた。 |
| C5d-3 | 「GitHub に問い合わせる」は、`infra/github.Client` の問い合わせ(REST・デバイスフロー)と、git の HTTP 通信(remote との同期・ミラーへの送信)の両方を指す。応答の状態コードを問わず、`Date` があれば測る。git の測定は相手のホストを見ない。 | 08 `:23` の「GitHub に問い合わせたことがないマシン(アカウントも remote もない)」は、remote との通信も問い合わせに数えている。アカウントもリンクもない Store では REST の問い合わせが起きず、時計を直しても警告が消えない。remote・ミラーは github.com の URL に限る(08 `:83`・`:231`)。テストの remote はこのマシンの時計で `Date` を返すので、ずれは 0 になる |
| C5d-4 | ずれの向き: `OnSkew` が受け取る d は「GitHub の時刻 − このマシンの時計」。警告の分は m = −d を使い、m が正なら「進んで」、負なら「遅れて」。 | `infra/github/client.go:116` の `observeSkew` は `t.Sub(c.Now())` を渡している |
| C5d-5 | 警告の判定は |d| ≥ 5 分(測った長さで比べる)。{N} は |d| を分で切り捨てた数(5 分 59 秒 → 5)。5 分未満なら警告の分は 0(警告なし)。 | 06 `:142`「5 分以上ずれたら警告を出し、5 分未満に戻ったら消す」。切り捨てなら、出る {N} は必ず 5 以上になる |
| C5d-6 | 記録は state.db の表 `clock_skew`(1 行だけ)に、警告の分(符号付き。0 は警告なし)を置く。行がなければ 0 として読む(問い合わせたことがないマシンには出さない)。再起動しても、次に問い合わせるまで前の警告を出す。 | 保存形式 `:286`。08 `:23` |
| C5d-7 | `Skew.Observe` は応答のたびに、いくつもの goroutine から ctx なしで呼ばれる。mutex で守り、警告の分が前の値と同じなら書かない・出来事を出さない。前の値は、最初の呼び出しで state.db から読んで覚える。書けなかったら `slog.Warn` だけにする(問い合わせの結果は変えない)。 | `OnSkew func(time.Duration)` の形(トラック D)。応答ごとに書くと、取り直しのたびに書き込みと画面の読み直しが起きる |
| C5d-8 | 値が変わったら出来事 `port.Event{Kind: port.EventSync}`(Store は空)を出す。画面は出来事を受けるたびに `sync.load()` で `GET /api/sync` を読み直す(`web/app/src/wire.ts:208`)。 | 06 `:130` の同期カードの行。新しい種類は足さない |
| C5d-9 | `GET /api/sync` の `SyncStatusList` の一番上に `clockSkew`(int、警告の分。正は進んでいる、負は遅れている、0 は警告なし)を足す。Store の行の中には入れない。 | 同期カードは remote がある表示中の Store がないと出ない(`syncCard` が null)が、設定「一般」には出す(08 `:23`、閲覧のみの Store だけでも同じ) |
| C5d-10 | 同期カードの警告の行は、「24 時間以上同期されていません」の行の次に置く。カードがないとき(remote がある表示中の Store がない)は出さない。 | 06 `:130`「その次の行に」 |
| C5d-11 | e2e のビルドには、入口 `POST /e2e/clock-skew`(本文 `{"seconds": n}`。n は「GitHub の時刻 − このマシンの時計」の秒)を足し、`Skew.Observe` に渡す。e2e のテストは、最後に `{"seconds": 0}` を送って警告を消す。 | e2e のビルドの GitHub は偽物で、`OnSkew` が呼ばれない。サーバーは Playwright の worker ごとに 1 つで、ほかのテストと共有する |
| C5d-12 | トラック C の計画の C5d の行の受け入れ条件「3 か所」を「2 か所(設定「一般」と同期カード)」に直す。未決の点 2(`:2079`)は、トラック D の決定 3(`OnSkew`)と C5d-3 に合わせて直す。`cmd/hedgehug/wire.go:49` のコメントも直す。 | 仕様の文は 3 か所(06 `:143`・`:144`、08 `:23`)にあるが、画面に出るのは 2 か所 |

## Review Focus

1. 時計を直した利用者: 7 分進んでいた時計を OS で直すと、次の同期(git の通信)か GitHub の読み取りで警告が消える。アカウントも Issue のリンクもない Store だけでも消える(C5d-3)。→ Task 3 の `TestSyncReportsSkewFromTheRemoteResponse` と、Task 6 の e2e の消える段。
2. 同時に届く応答: 同期と GitHub の読み取りが同時に走り、`Observe` が並んで呼ばれても、記録と出来事が壊れない・重ならない。→ Task 2 の `TestSkewObserveIsSafeWhenCalledTogether`(`go test -race`)。
3. 境目: 4 分 59 秒は警告なし、5 分ちょうどは「5 分」、5 分 59 秒も「5 分」。向きは両方。→ Task 1 の `TestSkewMinutes`。
4. 再起動: 警告が出たまま再起動しても、次に問い合わせるまで警告が残る。問い合わせたことがないマシンには出ない。→ Task 1 の `TestClockSkewsRoundTrip`(行がないと 0)と、Task 2 の `TestSkewLoadsTheRecordedValueFirst`。
5. Store がないマシン・閲覧のみの Store だけのマシン: 同期カードは出ないが、設定「一般」には警告が出る。→ Task 5 の `clockWarning` のテスト(status の Store が空でも文を返す)と、Task 4 の API のテスト(Store が 0 件でも `clockSkew` を返す)。

---

### Task 1: 警告の分の判定と state.db の記録

**Files:**
- Modify: `internal/domain/syncstatus/syncstatus.go`(末尾に足す)
- Test: `internal/domain/syncstatus/syncstatus_test.go`
- Modify: `internal/app/port/clock.go`
- Modify: `internal/app/apptest/clock.go`
- Create: `internal/infra/statedb/skew.go`
- Test: `internal/infra/statedb/skew_test.go`

**Interfaces:**
- Produces:
  - `syncstatus.SkewLimit = 5 * time.Minute`
  - `syncstatus.SkewMinutes(d time.Duration) int`(d は「GitHub の時刻 − このマシンの時計」。戻り値は C5d-4・C5d-5 の警告の分)
  - `port.ClockSkews` interface: `Skew(ctx context.Context) (int, error)`(記録がなければ 0)、`SetSkew(ctx context.Context, minutes int) error`
  - `apptest.ClockSkews`(mutex で守った `Minutes int` を持つ偽物。`NewClockSkews() *ClockSkews`)
  - `(*statedb.DB).ClockSkews() port.ClockSkews`

- [ ] **Step 1: 失敗するテストを書く**

`syncstatus_test.go` に表のテスト `TestSkewMinutes` を足す。入力 d と期待値:

| d | 期待値 |
|---|---|
| `0` | 0 |
| `-4*time.Minute - 59*time.Second` | 0 |
| `4*time.Minute + 59*time.Second` | 0 |
| `-5 * time.Minute` | 5(進んでいる) |
| `5 * time.Minute` | -5(遅れている) |
| `-5*time.Minute - 59*time.Second` | 5 |
| `-7*time.Minute - 30*time.Second` | 7 |
| `6*time.Minute + 10*time.Second` | -6 |
| `-3 * time.Hour` | 180 |

`skew_test.go` に `TestClockSkewsRoundTrip` を書く(`openTemp(t).ClockSkews()`。`screens_test.go` と同じ形)。
- 空の state.db で `Skew` は `0, nil`
- `SetSkew(ctx, 7)` のあと `Skew` は 7
- `SetSkew(ctx, -6)` で上書きされ -6
- `SetSkew(ctx, 0)` のあと 0

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/domain/syncstatus/ ./internal/infra/statedb/ -run 'TestSkewMinutes|TestClockSkewsRoundTrip'`
Expected: FAIL(`SkewMinutes`・`ClockSkews` が未定義)

- [ ] **Step 3: 実装する**

- `SkewMinutes`: m := −d。|m| < `SkewLimit` なら 0。そうでなければ `int(m / time.Minute)`(0 に向けて切り捨て)。
- `port/clock.go` に `ClockSkews` を足す。コメントに「GitHub の時刻とこのマシンの時計のずれの警告の分(06「時計のずれの警告」)。正はこのマシンの時計が進んでいる、負は遅れている、0 は警告なし」と書く。
- `apptest/clock.go` に偽物を足す。
- `statedb/skew.go`: `register("sync/004", ...)` で表 `clock_skew (id INTEGER PRIMARY KEY CHECK (id = 1), minutes INTEGER NOT NULL)` を作る。`SetSkew` は `INSERT … ON CONFLICT (id) DO UPDATE`(`screens.go` の `Put` と同じ形)。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/domain/syncstatus/ ./internal/infra/statedb/`
Expected: PASS

- [ ] **Step 5: コミットする**

```bash
git add internal/domain/syncstatus internal/app/port/clock.go internal/app/apptest/clock.go internal/infra/statedb/skew.go internal/infra/statedb/skew_test.go
git commit -m "feat(sync): 時計のずれの警告の分を決め、state.db に記録する(C5d)"
```

---

### Task 2: ずれを受け取る `syncing.Skew`

**Files:**
- Create: `internal/app/syncing/skew.go`
- Test: `internal/app/syncing/skew_test.go`

**Interfaces:**
- Consumes: Task 1 の `syncstatus.SkewMinutes`、`port.ClockSkews`、`apptest.ClockSkews`、`apptest.Events`
- Produces:
  - `type Skew struct { Records port.ClockSkews; Events port.Events; mu sync.Mutex; loaded bool; last int }`
  - `func (s *Skew) Observe(d time.Duration)`(`OnSkew` にそのまま渡せる形)
  - `func (s *Skew) Minutes(ctx context.Context) (int, error)`(`Records.Skew` を読む)

- [ ] **Step 1: 失敗するテストを書く**

`skew_test.go`:
- `TestSkewObserveRecordsOnlyWhenTheWarningChanges`: 偽物の `Records`・`Events` で次の順に呼ぶ。`Observe(-7m30s)` → 記録 7・出来事 1 つ(`port.Event{Kind: port.EventSync}`)。`Observe(-7m10s)` → 記録 7 のまま・出来事は増えない。`Observe(6m)` → 記録 -6・出来事 2 つ。`Observe(1m)` → 記録 0・出来事 3 つ。`Observe(-2m)` → 出来事 3 つのまま。
- `TestSkewLoadsTheRecordedValueFirst`: `Records` に 7 を入れておき、`Observe(-7m)` で出来事が出ないこと。`Minutes` が 7 を返すこと。
- `TestSkewObserveIsSafeWhenCalledTogether`: 50 個の goroutine から `Observe(-8m)` を同時に呼び、記録が 8・出来事がちょうど 1 つ。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test -race ./internal/app/syncing/ -run TestSkew`
Expected: FAIL(`Skew` が未定義)

- [ ] **Step 3: `Observe`・`Minutes` を実装する**

C5d-7・C5d-8 のとおり。`Observe` は ctx を受け取らないので、`context.Background()` で読み書きする。最初の読み込みに失敗したら、前の値を 0 として続ける。読み書きの失敗は `slog.Warn("時計のずれを記録できませんでした", "error", err)` だけにする。書けなかったときは前の値を変えない(次の応答でもう一度書く)。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/app/syncing/`
Expected: PASS

- [ ] **Step 5: コミットする**

```bash
git add internal/app/syncing/skew.go internal/app/syncing/skew_test.go
git commit -m "feat(sync): GitHub の応答のずれを受け取り、警告が変わったときだけ記録して知らせる(C5d)"
```

---

### Task 3: git の通信で測る・本物と e2e の組み立て

**Files:**
- Modify: `internal/infra/gitstore/sync_transport.go`
- Modify: `internal/infra/gitstore/replicator.go`(`Replicator` に `OnSkew`、`Sync` の入口)
- Modify: `internal/infra/gitstore/replicator_mirror.go`(`Mirror`・`SyncSave` の入口)
- Test: `internal/infra/gitstore/sync_errors_test.go`(`statusServer` の隣に足す)
- Modify: `internal/adapter/httpapi/routes.go`(`Deps` に `ClockSkew *appsyncing.Skew`)
- Modify: `cmd/hedgehug/wire.go`
- Modify: `cmd/hedgehug/wire_e2e.go`
- Create: `cmd/hedgehug/skew_e2e.go`
- Test: `cmd/hedgehug/skew_e2e_test.go`(`synclog_e2e_test.go` と同じ形)
- Modify: `cmd/hedgehug/device_e2e.go:139`(入口をつなぐ)

**Interfaces:**
- Consumes: Task 2 の `syncing.Skew`、Task 1 の `(*statedb.DB).ClockSkews()`
- Produces:
  - `gitstore.Replicator.OnSkew func(time.Duration)`(nil なら測らない。d は「応答の `Date` − このマシンの時計」)
  - `httpapi.Deps.ClockSkew *appsyncing.Skew`
  - e2e の入口 `POST /e2e/clock-skew`(本文 `{"seconds": n}`、API と同じトークン、成功は 204)

- [ ] **Step 1: 失敗するテストを書く**

`sync_errors_test.go` に `TestSyncReportsSkewFromTheRemoteResponse` を足す。`httptest` のサーバーは、どの要求にも `Date` を「いまから 10 分前」(`http.TimeFormat`)にして 404 を返す。`Replicator.OnSkew` に受け取った d を覚える関数を置いて `Sync` を呼ぶ(結果のエラーは問わない)。受け取った d のどれかが -10 分 ± 1 分に入ること。`OnSkew` が nil でも `Sync` が同じエラーを返すこと(パニックしない)。

`skew_e2e_test.go` に `TestClockSkewHandler` を書く(`synclog_e2e_test.go` と同じ組み立て)。
- トークンなしは 401。GET は 405。
- `{"seconds": -420}` で 204。`Skew.Minutes` が 7、出来事 `sync` が 1 つ。
- ほかの道は `next` に渡す。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/infra/gitstore/ -run TestSyncReportsSkewFromTheRemoteResponse && go test -tags e2e ./cmd/hedgehug/ -run TestClockSkewHandler`
Expected: FAIL(`OnSkew`・`clockSkewHandler` が未定義)

- [ ] **Step 3: 実装する**

- `sync_transport.go`:
  - `withSkew(ctx context.Context, fn func(time.Duration), now func() time.Time) context.Context` を足す。fn が nil なら ctx をそのまま返す。
  - `watchedTransport.RoundTrip` は、応答を受け取ったら(見張りの有無を問わず)、ctx に fn があり `Date` を `http.ParseTime` で読めれば、`fn(date.Sub(now()))` を呼ぶ。
- `Replicator` の `Sync`・`Mirror`・`SyncSave` の入口で、`ctx = withSkew(ctx, r.OnSkew, r.stores.clock.Now)` にする。
- `wire.go`:
  - `hub` を `gh` より先に作る。
  - `skew := &syncing.Skew{Records: db.ClockSkews(), Events: hub}` を作り、`github.New(skew.Observe)` に渡す。
  - `newReplicator` で `rep.OnSkew = skew.Observe` にする(引数に `skew` を足す)。
  - `deps.ClockSkew = skew` にする。
  - 49 行目のコメントは「時計のずれは syncing.Skew が受け取る(C5d-3)」に直す。
- `wire_e2e.go`: `deps.ClockSkew = &syncing.Skew{Records: state.ClockSkews(), Events: hub}` にする。
- `skew_e2e.go`: `clockSkewHandler(token string, skew *syncing.Skew, next http.Handler) http.Handler`。形は `syncLogHandler` と同じ。`skew.Observe(time.Duration(b.Seconds) * time.Second)` を呼ぶ。出来事は `Observe` が出す。
- `device_e2e.go:139` の鎖に `clockSkewHandler(token, deps.ClockSkew, …)` を足す。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/infra/gitstore/ ./cmd/hedgehug/ && go test -tags e2e ./cmd/hedgehug/ && golangci-lint run --build-tags e2e ./...`
Expected: PASS、lint の指摘なし

- [ ] **Step 5: コミットする**

```bash
git add internal/infra/gitstore internal/adapter/httpapi/routes.go cmd/hedgehug
git commit -m "feat(sync): git の通信と GitHub の問い合わせの応答で時計のずれを測る(C5d)"
```

---

### Task 4: `GET /api/sync` の `clockSkew` と web の口

**Files:**
- Modify: `internal/adapter/httpapi/syncing/dto.go`(`SyncStatusList`)
- Modify: `internal/adapter/httpapi/syncing/syncing.go`(`Services` に `Skew *appsyncing.Skew`、`list-sync-status`、`toStatusList`)
- Modify: `internal/adapter/httpapi/routes.go`(`syncapi.Services{…, Skew: deps.ClockSkew}`)
- Test: `internal/adapter/httpapi/syncing/syncing_test.go`
- Regenerate: `api/openapi.yaml`(`make generate`)、`web/packages/api/src/generated.ts`(`cd web && pnpm gen`)
- Modify: `web/packages/core/src/model/sync.ts`(`SyncStatus` に `clockSkew: number`)
- Modify: `web/packages/api/src/sync.ts`
- Test: `web/packages/api/src/sync.test.ts`

**Interfaces:**
- Consumes: Task 3 の `Deps.ClockSkew`、Task 2 の `Skew.Minutes`
- Produces:
  - JSON `SyncStatusList.clockSkew`(int。doc は「時計のずれの警告の分。正はこのマシンの時計が進んでいる、負は遅れている。0 は警告なし」)
  - core `SyncStatus.clockSkew: number`(同じ意味)

- [ ] **Step 1: 失敗するテストを書く**

- `syncing_test.go` の `newEnv` で `Skew: &appsyncing.Skew{Records: apptest.NewClockSkews(), Events: …}` を渡す。
- `TestSyncStatusReportsClockSkew` を足す。
  - `Observe(-7m)` のあとの `GET /api/sync` は `clockSkew` が 7。
  - `Observe(6m)` のあとは -6。
  - 何も測っていない env では 0。
  - 対象の Store が 0 件(`Targets` が空)でも 200 で `clockSkew` を返す。
- `sync.test.ts` の既存の変換のテストの応答に `clockSkew: -6` を足し、結果の `clockSkew` が -6 であること。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `go test ./internal/adapter/httpapi/syncing/ && (cd web && pnpm vitest run packages/api/src/sync.test.ts)`
Expected: FAIL

- [ ] **Step 3: 実装する**

`toStatusList` に分を渡す。`make generate` と `pnpm gen` で生成物を作り直す。`api/sync.ts` は `clockSkew: body.clockSkew` をそのまま写す。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/adapter/httpapi/... && (cd web && pnpm check && pnpm vitest run packages/api)`
Expected: PASS

- [ ] **Step 5: コミットする**

```bash
git add internal/adapter/httpapi api web/packages/api web/packages/core/src/model/sync.ts
git commit -m "feat(sync): 同期の状態の API で時計のずれの警告の分を返す(C5d)"
```

---

### Task 5: 同期カードの警告の行と、警告の文

**Files:**
- Modify: `web/packages/core/src/features/sync/card.ts`
- Test: `web/packages/core/src/features/sync/card.test.ts`
- Modify: `web/packages/core/src/index.ts`(`clockWarning` を出す)
- Modify: `web/packages/ui/src/types.ts`(`SyncCardView` に `clock: string | null`)
- Modify: `web/packages/ui/src/patterns/SyncCard.svelte`
- Test: `web/packages/ui/src/patterns/sync.test.ts`

**Interfaces:**
- Consumes: Task 4 の `SyncStatus.clockSkew`
- Produces:
  - core `SyncCard.clock: string | null`(「このマシンの時計が {N} 分ずれています」。0 なら null)
  - core `clockWarning(status: SyncStatus | null): string | null`(設定「一般」の文。status が null か `clockSkew` が 0 なら null)

- [ ] **Step 1: 失敗するテストを書く**

`card.test.ts`:
- `clockSkew: 7` の status の `syncCard(...).clock` は「このマシンの時計が 7 分ずれています」。`-6` なら「このマシンの時計が 6 分ずれています」。`0` なら `null`。
- `clockWarning` は次のとおり。
  - 7: 「このマシンの時計が GitHub の時刻より 7 分進んでいます。同期で後から届いた変更の勝ち負けと、削除済みを完全に削除する日が、他の人とずれます。OS の日時の設定を確かめてください。」
  - -6: 「このマシンの時計が GitHub の時刻より 6 分遅れています。同期で後から届いた変更の勝ち負けと、削除済みを完全に削除する日が、他の人とずれます。OS の日時の設定を確かめてください。」
  - 0 と null: `null`
  - `stores: []`(カードは null)でも、`clockSkew: 7` なら文を返す。

`sync.test.ts`(ui): `clock` に文があると、その文が `.line.warn` で「24 時間以上」の行の後に出る。`null` なら出ない。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `cd web && pnpm vitest run packages/core/src/features/sync packages/ui/src/patterns/sync.test.ts`
Expected: FAIL

- [ ] **Step 3: 実装する**

`syncCard` が `clock` を埋める。`SyncCard.svelte` は `{#if card.clock}<p class="line warn">{card.clock}</p>{/if}` を stale の行の次に置く。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `cd web && pnpm check && pnpm vitest run packages/core packages/ui`
Expected: PASS

- [ ] **Step 5: コミットする**

```bash
git add web/packages/core web/packages/ui
git commit -m "feat(sync): 同期カードに時計のずれの行を出し、設定の警告の文を作る(C5d)"
```

---

### Task 6: 設定の「一般」タブ・e2e・記録の直し

**Files:**
- Modify: `web/packages/core/src/features/router/route.ts`(`SettingsTab = "general" | "accounts"`、`settingsTabs`)
- Test: `web/packages/core/src/features/router/route.test.ts`
- Create: `web/app/src/screens/settings/GeneralSection.svelte`
- Modify: `web/app/src/screens/settings/SettingsScreen.svelte`
- Test: `web/e2e/settings.spec.ts`
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-c.md`(`:79`・`:2079`。C5d-12。「G への申し送り」の表に 1 行)

**Interfaces:**
- Consumes: Task 5 の `clockWarning`、`web/app/src/wire.ts` の `sync.status`、Task 3 の `POST /e2e/clock-skew`

- [ ] **Step 1: 失敗するテストを書く**

`route.test.ts`:
- `/settings/general` は `{ name: "settings", tab: "general" }`。往復で `/settings/general` に戻る。
- `/settings` は今までどおり `accounts`(C5d-2)。

`settings.spec.ts` に `the clock warning shows in the general tab and on the sync card, and goes away when the clock is back` を足す(要求は `web/e2e/search-table-helpers.ts` の `send(page, "post", path, data)` で送る)。
1. `/settings/accounts` を開く。タブの並びが「一般」「アカウントとメンバー」の順。
2. `send(page, "post", "/e2e/clock-skew", { seconds: -420 })` を送る。タブ「一般」を押す。
3. 「このマシンの時計が GitHub の時刻より 7 分進んでいます。同期で後から届いた変更の勝ち負けと、削除済みを完全に削除する日が、他の人とずれます。OS の日時の設定を確かめてください。」が見える。
4. サイドバー(`sidebar(page, info)`。860px 以下はドロワー)に「このマシンの時計が 7 分ずれています」が見える。
5. `{ seconds: 360 }` を送ると、「…6 分遅れています。…」に変わる。再読み込みしなくても変わる(出来事 `sync`)。
6. `{ seconds: 60 }` を送ると、両方とも消える。
7. 最後に必ず `{ seconds: 0 }` を送る(`try`/`finally`。C5d-11)。

既存の「歯車で設定が開く」のテストは、URL が `/settings/accounts` のまま通ること(C5d-2)。

- [ ] **Step 2: テストが落ちることを確かめる**

Run: `cd web && pnpm vitest run packages/core/src/features/router && cd .. && make e2e`
Expected: route のテストと、足した e2e が FAIL

- [ ] **Step 3: 実装する**

- `route.ts` に `general` を足す(先頭)。`/settings` の既定は `accounts` のまま。
- `SettingsScreen.svelte` の `tabs` を `[{ id: "general", label: "一般" }, { id: "accounts", label: "アカウントとメンバー" }]` にし、`general` のとき `GeneralSection` を出す。
- `GeneralSection.svelte`: `clockWarning($status)` が null でなければ、警告の色の段落を 1 つ出す(色は `SyncCard.svelte` の `.warn` と同じ変数)。コメントに C5d-1(G がログイン名を上に、同期のタイミングを下に足す)を書く。
- トラック C の計画の「G への申し送り」の表に、行「設定の「一般」タブ」を足す。内容は「C5d が作った「一般」タブ(`GeneralSection.svelte`)に、ログイン名を時計のずれの警告の上に、同期のタイミングを下に足す(C5d-1)。そのとき `/settings`・歯車・パレットの「設定を開く」の開く先を「一般」に変える(C5d-2)」。
- トラック C の計画の C5d の行に計画のパスを書き、受け入れ条件を「時計のずれの文言が 2 か所(設定「一般」と同期カード)に出る」に直す。未決の点 2 の答えを「D の `infra/github.Client.OnSkew` と、gitstore の `Replicator.OnSkew`(git の通信)で測る。C5d が `syncing.Skew` で受け取る」に直す。

- [ ] **Step 4: 全体を確かめる**

Run: `make check && golangci-lint run --build-tags e2e ./... && make e2e`
Expected: すべて緑

- [ ] **Step 5: コミットする**

```bash
git add web docs/superpowers/plans/2026-11-12-hedgehug-track-c.md
git commit -m "feat(settings): 設定に「一般」タブを足し、時計のずれの警告を出す(C5d)"
```

## PR に書くこと

- 範囲(06「時計のずれの警告」)と、決めたこと C5d-1〜C5d-12。
- G へ渡すもの(トラック C の計画の「G への申し送り」に書いた): 設定「一般」タブに、ログイン名(警告の上)と同期のタイミング(警告の下)を足す(C5d-1)。そのとき、`/settings`・歯車・「設定を開く」の開く先を「一般」に変える(タブの並びの先頭。中が空でなくなるので C5d-2 の理由がなくなる)。
