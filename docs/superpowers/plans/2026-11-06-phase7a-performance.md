# フェーズ 7a 性能 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 1万タスクの Store で `hh ls` の6つの場面を測り、spec の目標を満たす。

**Architecture:** 既存の `historytest.Synth` で1万タスクの Store を合成する。場面の準備を `usecasetest` に1か所で書き、`browse` のベンチ（プロファイル用）と `cmd/hedgehug` のベンチ（実プロセス、判定用）の両方から使う。

**Tech Stack:** Go の testing.B、go-git

spec: `docs/superpowers/specs/2026-11-06-phase7-finish-design.md` の「7a 性能」

---

## ファイル

- 修正: `internal/app/storage/history/historytest/bench.go` 1万タスクの合成の入口
- 修正: `internal/app/usecase/usecasetest/bench.go` 環境の切り出しと場面の定義
- 作成: `internal/app/usecase/browse/bench_ls_test.go` usecase の層のベンチ
- 作成: `cmd/hedgehug/bench_test.go` 実プロセスのベンチ
- 修正: `Makefile` `bench-tasks`
- 修正: spec の末尾に「計測の結果」

### Task 1: 1万タスクの合成の入口

**Files:**
- Modify: `internal/app/storage/history/historytest/bench.go`

- [ ] **Step 1: 定数と入口を足す**

```go
// The store of the benchmarks of make bench-tasks: 20,000 commits after the
// 10,000 that add the tasks.
const (
	TaskBenchCommits = 30_000
	TaskBenchTasks   = 10_000
)

// TaskBenchSource is the synthesized store of make bench-tasks. It skips b
// unless HH_BENCH is set.
func TaskBenchSource(b *testing.B) string {
	b.Helper()
	if os.Getenv("HH_BENCH") == "" {
		b.Skip("set HH_BENCH=1 (make bench-tasks) to measure on a store of 10,000 tasks")
	}
	return Synth(b, TaskBenchCommits, TaskBenchTasks, benchSeed)
}
```

- [ ] **Step 2: ビルドを確かめる**

Run: `go build ./internal/app/storage/history/...`
Expected: 成功

### Task 2: 場面の定義と usecase の層のベンチ

**Files:**
- Modify: `internal/app/usecase/usecasetest/bench.go`
- Create: `internal/app/usecase/browse/bench_ls_test.go`

- [ ] **Step 1: `BenchSession` の環境を `BenchEnv` に切り出す**

`BenchSession` の冒頭の環境の用意と config の書き出しを `BenchEnv(b, dir, account) map[string]string` に移す。`BenchSession` は `BenchEnv` の結果で `Session` を作る。`Now` はこれまでどおり。動作は変えない。

- [ ] **Step 2: 場面を足す**

```go
// LsCase is a situation in which a command lists the tasks: Prepare changes
// the store at dir as what happened before the command.
type LsCase struct {
	Name    string
	Args    []string // of hedgehug
	Tree    bool     // ls --tree
	Prepare func(b *testing.B, dir string)
}

// LsCases are the situations of the targets of the spec of phase 7a.
func LsCases() []LsCase {
	return []LsCase{
		{Name: "cached", Args: []string{"ls"}, Prepare: func(*testing.B, string) {}},
		{Name: "tree", Args: []string{"ls", "--tree"}, Tree: true, Prepare: func(*testing.B, string) {}},
		{Name: "afterCommit", Args: []string{"ls"}, Prepare: commitTitle},
		{Name: "afterReparent", Args: []string{"ls"}, Prepare: commitReparent},
		{Name: "schema", Args: []string{"ls"}, Prepare: commitSchema},
		{Name: "rebuild", Args: []string{"ls"}, Prepare: removeIndex},
	}
}
```

`commitTitle` は最初に見つけたタスクのタイトルを変える。`commitReparent` は2番目のプロジェクトを `epic` にして1番目のプロジェクトの下に移す（プロジェクトはルートにしか置けない。約 100 の子孫の再検査になる）。`commitSchema` は `tx.PutFile("schema.yaml", 今の内容 + "# bench\n")` を行う。どれも `BenchmarkShowAfterOneCommit` と同じく `ss.OpenStore` と `ss.Update` で書き、`s.Repo().Close()` する。`removeIndex` は `<dir>/.hedgehug/local/index` を消す。

- [ ] **Step 3: `BenchmarkLs` を足す**

`bench_ls_test.go`。場面ごとに `historytest.Indexed(b, historytest.TaskBenchSource(b))` を元に、繰り返しのたびに `Clone` し、`Prepare` し、新しい `BenchSession` で `browse.List(ss, "")`（Tree の場面は `browse.Tree(ss, "")`）を測る。List の件数が `TaskBenchTasks` であることを確かめる。

- [ ] **Step 4: 動かす**

Run: `HH_BENCH=1 go test ./internal/app/usecase/browse -run '^$' -bench BenchmarkLs -benchtime 3x -timeout 30m`
Expected: 6つの場面が PASS。`schema` と `rebuild` が `cached` より明らかに遅い（速ければ場面の準備が効いていない）

### Task 3: 実プロセスのベンチと Makefile

**Files:**
- Create: `cmd/hedgehug/bench_test.go`
- Modify: `Makefile`

- [ ] **Step 1: `BenchmarkLsProcess` を足す**

最初に `go build -o <tmp>/hedgehug .` でバイナリを作る。場面ごとに Task 2 と同じく `Clone` と `Prepare` をし、`usecasetest.BenchEnv` の環境に `PATH` を足して `exec.Command(bin, c.Args...)` を実行し、stdout を `io.Discard` に捨てる。終了コードが 0 でなければ `b.Fatal`。

- [ ] **Step 2: Makefile に足す**

```make
# Measure hedgehug ls on a synthesized store of 10,000 tasks, in the process
# and as a command (not part of check or CI; see the phase 7 spec). The store
# is kept in $HH_BENCH_CACHE as for bench-history.
bench-tasks:
	HH_BENCH=1 go test -p 1 ./internal/app/usecase/browse ./cmd/hedgehug -run '^$$' -bench 'BenchmarkLs' -benchtime 5x -timeout 60m
```

`.PHONY` に `bench-tasks` を足す。

- [ ] **Step 3: lint と通常のテストを確かめる**

Run: `golangci-lint run ./internal/app/... ./cmd/...` と `go test ./internal/app/usecase/browse ./cmd/hedgehug`
Expected: 成功。`HH_BENCH` なしでベンチは skip される。depguard に止められたら `.golangci.yaml` の該当の規則に許可を足す

- [ ] **Step 4: Commit**

```bash
git add Makefile cmd/hedgehug internal/app
git commit -m "test: 1万タスクの hh ls を測る make bench-tasks を足す"
```

### Task 4: 計測

- [ ] **Step 1: mac で測る**

Run: `make bench-tasks 2>&1 | tee <スクラッチパッド>/bench-tasks-mac.txt`
Expected: `BenchmarkLsProcess` の6場面の値が取れる

- [ ] **Step 2: 目標と比べる**

spec の表と比べる。超えた項目ごとに `BenchmarkLs/<場面>` を `-cpuprofile` 付きで流し、`go tool pprof -top -cum` で原因を調べる。

### Task 5: 目標を超えた項目の修正

WSL の全件の作り直しが 1,033ms で目標を超えた。mac のプロファイルでは、時間のほぼすべてがタスクのファイルを1件ずつ読む pack の `pread` だった。`internal/app/storage/index/build.go` に `readEach`（`errgroup`、`GOMAXPROCS` まで）を足し、`rebuild` と `revalidate` の読み込みと解析を並列にする。index への書き込みは順次のまま。`go test -race` と、実 pack の上の `-race` のベンチで確かめる。

### Task 6: 結果の記録と引き継ぎ

**Files:**
- Modify: `docs/superpowers/specs/2026-11-06-phase7-finish-design.md`
- Create: `docs/superpowers/plans/2026-11-06-phase7a-handoff.md`

- [ ] **Step 1: WSL の計測を頼む**

ブランチを push し、draft PR を作り、手順を PR にコメントする。利用者は WSL で `make bench-tasks` を流し、出力を `gh pr comment` で PR に載せる。

- [ ] **Step 2: spec に「計測の結果」を足す**

history-growth の 8 節と同じ形で、環境の表と、項目・目標・WSL・mac・判定の表を書く。

- [ ] **Step 3: 引き継ぎを書き、`make check` を通してコミットし、PR を出す**

Run: `make check`
Expected: 成功
