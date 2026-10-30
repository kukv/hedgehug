# フェーズ 1d-2: 編集・コメント・履歴・検査・devenv と持ち越しの直し Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** フェーズ1を閉じる。`edit`、`comment`（追加・編集・削除）、`log`、`schema check`、`schema migrate`、`doctor`、`show` の履歴、`tools/devenv` を作り、1a〜1d-1 の持ち越しをすべて直す。

**Architecture:** 形式（model / schema）、Store と gitrepo、config、CLI の既存の直しを先に入れ、その上に新しいコマンドを足す。`internal/doctor` は検査を Finding の列で返し、`cmd/hedgehug` が出力する。`schema.Migrate` は Schema の値だけを変え、タスクの書き換えと1コミットは CLI が `Store.Update` で行う。

**Tech Stack:** Go 1.27.3、cobra、go-git v5、`go.yaml.in/yaml/v4`、`golang.org/x/term`、`golang.org/x/text/width`（Task 6 で足す）

**Spec:** `docs/superpowers/specs/2026-10-30-phase1d2-maintenance-design.md`（実装前に必ず全体を読む。この plan は spec の細部を繰り返さない。食い違えば spec が正）

## Global Constraints

- モジュールは `github.com/kukv/hedgehug`、Go は `go.mod` のまま。新しい依存は `golang.org/x/text`（`width` パッケージ）だけ。
- 依存の向きは `cmd/hedgehug → doctor → query/index → store → gitrepo/schema → model`、`cmd/hedgehug → config`、`tools/devenv → config, store, schema, model`。`doctor` は `cmd/hedgehug` を、`config` は `store` を import しない。
- テストは標準の `testing` だけ。対象と同じパッケージに置き、既存のテストの書き方（`t.Fatal`、`t.Errorf("... = %v, want %v")`、各パッケージの `helpers_test.go` の補助関数）に合わせる。cmd のテストは `newTestEnv` / `te.run` / `te.ok` / `te.log` を使う。
- 環境変数は `getenv func(string) string` で受ける。`os.Getenv` を直接呼ばない（`main` の `env` だけが呼ぶ）。
- コード中のコメントと CLI の出力は英語。コミットメッセージは日本語の Conventional Commits で、末尾に空行を挟んで `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`。
- 各タスクの最後に `make check` を通す。`GOOS=windows go vet ./...` と `GOOS=darwin go vet ./...` も通す。gosec の指摘には具体的な理由付きの `//nolint:gosec // <理由>`。
- ファイルは 0600、ディレクトリは 0750。
- 終了コード: 成功 0、実行時のエラー 1、フラグや引数の誤り 2、`-h` は 0。`schema check` / `doctor` は問題があれば 1。
- 新しい Op 名: `edit`、`comment`、`migrate`。

## ファイル構成

```
internal/model/node.go          明示タグの拒否、Encoder の要素の対応づけ（Task 1・2）
internal/schema/validate.go     number の NaN/Inf（Task 1）、遷移を飛ばす ValidateWrite の指定（Task 3）
internal/schema/migrate.go      Migrate（Task 11）
internal/gitrepo/commit.go      ErrEmptyCommit の寄せ（Task 3）
internal/gitrepo/log.go status.go verify.go   （Task 4）
internal/store/tx.go            Update の戻り値、Op.SkipTransitions、RemoveTask（Task 3）
internal/store/read.go          TaskBytes、CommentIDs（Task 3）
internal/config/config.go author.go           （Task 5）
internal/doctor/finding.go schema.go doctor.go （Task 12・13）
cmd/hedgehug/                   既存の直し（Task 6）、editor.go edit.go（Task 8）、comment.go（Task 9）、log.go（Task 10）、migrate.go schema.go（Task 11・12）、doctor.go（Task 13）
tools/devenv/main.go            （Task 14）
```

---

### Task 1: 明示タグと NaN/Inf の拒否

**Files:**
- Modify: `internal/model/node.go`（`checkNode`）
- Modify: `internal/schema/validate.go`（number の検査）
- Modify: `cmd/hedgehug/assign.go`（`propValue` の number）
- Test: `internal/model/node_test.go`、`internal/schema/validate_test.go`、`cmd/hedgehug/set_test.go`

**Interfaces:**
- Produces: `ParseDoc` が明示タグの付いたノードで `*DecodeError`（`Line` 付き、メッセージ `YAML tags are not supported`）を返す。

- [ ] **Step 1: 失敗するテストを書く**
  - `ParseDoc` に `title: !!binary aGk=\n`、`x: !!str 12\n`、`x: !custom v\n`、`!!map {a: 1}` を渡すと `*DecodeError` になり、`Line` がタグの行。タグのない `x: 12` と `x: "12"` は通る。
  - `ValidateTask` で number の props に `NaN`、`.nan`、`Inf`、`-Inf`、`+Inf`、`.inf` を入れると `bad_value`。`1.5`、`-3`、`1e3` は通る。
  - `te.run("set", id, "points=NaN")`（既定スキーマの number の props は `points`）が終了コード 1 で、stderr が `is not a number` を含む。`points=Inf` も同じ。
- [ ] **Step 2: テストが失敗することを確かめる**（`go test ./internal/model/ ./internal/schema/ ./cmd/hedgehug/ -run 'Tag|NaN|Number'`）
- [ ] **Step 3: 実装する**
  - 書き手がタグを付けたかは `n.Style&yaml.TaggedStyle != 0` で判定する（yaml/v4 でタグ付きの入力に立つ。立たない場合は `n.Tag != "" && n.Tag != n.ShortTag()` ではなく、`yaml.Load` 後のノードの `Style` を実際に出力して確かめ、判定の根拠をコメントに書く）。
  - number は `strconv.ParseFloat` の後に `math.IsNaN(x) || math.IsInf(x, 0)` を誤りにする（`internal/query/eval.go:379` と同じ形）。
- [ ] **Step 4: テストが通ることを確かめ、`make check`**
- [ ] **Step 5: コミット** `fix: YAML の明示タグと number の NaN・Inf を拒否する`

### Task 2: Encoder で要素を対応づけ、未知のキーとコメントを残す

**Files:**
- Modify: `internal/model/node.go`（`Encoder.Put`、`reuseItems` を置き換える）
- Test: `internal/model/node_test.go`、`internal/schema/schema_test.go`

**Interfaces:**
- Consumes: なし
- Produces: `Encoder.Put(key, v, zero)` の外から見える形は変えない。`schema.Encode` はそのまま新しい規則で動く。

規則（spec の「持ち越しの直し」の「形式」）:
- `mergeNode(old, nv *yaml.Node, t reflect.Type) *yaml.Node` を作る。`NodeEqual(old, nv)` なら `old` を返す。
- 両方 mapping:
  - `t` が struct（ポインタは外す）なら、既知のキーは yaml タグの名前（`-` と空は除く。タグがなければ小文字のフィールド名）。`nv` の各キーについて、`old` に同じキーがあれば値を `mergeNode(old値, nv値, そのフィールドの型)` にし、キーのノードも `old` のものを使う。`old` にだけあり、既知でないキーは、`old` の並びの位置を保って残す。`old` にだけあり既知のキー（omitempty で消えた値）は消す。
  - `t` が struct でない mapping（`Props`、`Lists`、`map[string]string`、`Fields`）なら、キーはすべて利用者のデータ。`nv` のキーの順に、`old` の同じキーの値と `mergeNode` する。要素の型は、`Props` なら `Prop`、`Lists` なら `[]string`、map ならその値の型、`Fields` なら nil（nil は「すべて既知」として扱い、mapping でも消えたキーは消す）。`Props` と `Lists` の判定は、`t` が `yaml.Marshaler` を実装した slice で要素が struct なら「その struct の、yaml タグが `-` のフィールド（Name / Key）をキーにした mapping」とみなす一般の規則にする（schema を import しないため）。`List` の値は `Values` フィールドの型（`[]string`）。
- 両方 sequence で、要素の型が struct:
  - 識別のフィールドは、yaml タグが `name` か `where` のフィールド。あれば、`nv` の各要素をその値が同じ `old` の要素に対応させる（各 `old` の要素は1回だけ）。残った要素どうしは順番に対応させる。識別のフィールドがなければ、今の `reuseItems` と同じく値が同じものを対応させ、残りを順番に対応させる。
  - 対応した要素は `mergeNode` する。対応しない新しい要素はそのまま。
- そのほか（scalar、型の違い）は、今と同じく `styleLike(nv, old)` を当てた `nv`。
- `Put` は、`old` があり、`v` が `*yaml.Node` でなければ、`nv` をエンコードした後に `mergeNode(old, nv, reflect.TypeOf(v))` を通す。`decodesTo` で値が同じなら `old` を使う今の早道はそのまま残す。

- [ ] **Step 1: 失敗するテストを書く**（`internal/schema/schema_test.go`）
  - schema.yaml に、各 status / kind / prop / workflow.transitions の項目へコメントと未知のキー（`x-note: keep`）を付けたものを用意する。
  - 1つの prop の `label` を変えて `Encode` すると、ほかの prop のコメントと未知のキーがバイト単位で残り、変えた prop の未知のキーも残る。
  - status を1つ改名（`Name` を変える）して `Encode` すると、その status の未知のキーとコメントが残り、`name` だけが変わる。
  - status を末尾に足す・1つ消すと、ほかの要素はそのまま。
  - `workflow.transitions` の1つのキーの値に1つ足すと、ほかのキーのコメントが残る。
  - 何も変えずに `Encode` すると入力とバイト単位で同じ（既存のゴールデンテストが通り続けること）。
  - `internal/model/node_test.go`: タスクの `props` の1つを変えたとき、ほかの props の行末コメントが残る。
- [ ] **Step 2: 失敗を確かめる**（`go test ./internal/model/ ./internal/schema/`）
- [ ] **Step 3: 実装する**（上の規則。`reuseItems` は `mergeNode` に吸収して消す）
- [ ] **Step 4: 全パッケージのテスト（ゴールデンを含む）と `make check`**
- [ ] **Step 5: コミット** `fix: 書き換えで要素を対応づけ、ほかの要素と未知のキーのコメントを残す`

### Task 3: Store と gitrepo の直しと、新しい読み出し

**Files:**
- Modify: `internal/gitrepo/commit.go`、`internal/store/tx.go`、`internal/store/validate.go`、`internal/schema/validate.go`、`internal/store/read.go`、`cmd/hedgehug/app.go` と書き込みの各コマンド
- Test: `internal/gitrepo/commit_test.go`、`internal/store/tx_test.go`、`internal/store/read_test.go`、`internal/store/validate_test.go`、Update を呼ぶ全テスト

**Interfaces:**
- Produces:
  - `func (s *Store) Update(op Op, fn func(*Tx) error) (committed bool, err error)`（呼び出しをすべて直す。テストは `_, err :=` か `committed, err :=`）
  - `Op.SkipTransitions bool`: true なら `workflow.transitions` の検査をしない（必須項目はかける）
  - `func ValidateWrite(old *model.Task, oldParentKind string, oldE *Effective, cur *model.Task, parentKind string, e *Effective, skipTransitions bool) []Violation`（`ValidateTransition` に `from` を渡す代わりに、skip のときは遷移の判定だけを飛ばす。必須項目は今と同じ）
  - `func (s *Store) TaskBytes(id model.ID) ([]byte, error)`（なければ `ErrNotFound` を wrap）
  - `type CommentRef struct{ Task, Comment model.ID }`、`func (s *Store) CommentIDs() ([]CommentRef, error)`（パスの順）
  - `cmd/hedgehug`: `func (a *app) update(s *store.Store, op store.Op, fn func(*store.Tx) error) (bool, error)` に変え（作者は中で入れる）、各コマンドはコミットしなかったら `printTask` の見出しを `No changes` にする

- [ ] **Step 1: 失敗するテストを書く**
  - gitrepo: 作業ツリーのファイルを HEAD と同じ内容のまま `Commit` に渡すと（index も HEAD と同じ）、`errors.Is(err, ErrNothingToCommit)`。
  - store: 作業ツリーのタスクファイルを手で書き換え（コミットしない）、`Update` で HEAD と同じ内容を `PutTask` すると、エラーなし・`committed == false`・コミットが増えない。
  - store: 同じ Update で `PutComment` してから `RemoveTask` すると、コメントのファイルが残らず、コミットにも入らない。`RemoveTask` の後の `PutComment` / `PutWorklog` はエラー。
  - store: 変更があれば `committed == true`、内容が同じ `PutTask` だけなら false。
  - store: `workflow.transitions` で `todo → done` を禁じたスキーマで、`SkipTransitions: true` の Update は通り、false なら `transition` で拒否。`SkipTransitions` でも `required` は拒否する。
  - store: `TaskBytes` が壊れたファイル（`---\n: :\n---\n`）をそのまま返す。ないと `ErrNotFound`。
  - store: `CommentIDs` が2つのタスクの3つのコメントを返し、読めない名前（`comments/x/y/zzz/abc.md`）を飛ばす。
  - cmd: 同じ値の `set`（`te.ok("set", id, "title=same")` を2回）の2回目が `No changes` を出し、コミットが増えない。`status` も同じ。
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
  - `gitrepo.Commit`: `w.Commit` の `git.ErrEmptyCommit` を `ErrNothingToCommit` に変える（`fmt.Errorf("%w", ...)` ではなく置き換え）。index は defer で戻る。
  - `Store.Update`: `s.commit` が `ErrNothingToCommit` なら、書いたファイルは HEAD と同じなのでそのまま、`committed=false, err=nil`。
  - `RemoveTask`: `tx.order` のうち `path.Dir(model.CommentPath(id, zero))+"/"`、worklog の同じ接頭辞のものを `remove: true` にする。`tx.removed map[model.ID]bool` を持ち、`PutComment` / `PutWorklog` はそのタスクが removed ならエラー。
  - `TaskBytes` は `readFile(s.path(model.TaskPath(id)))`。`CommentIDs` は `comments/` を `filepath.WalkDir` で歩き、`model.ParseCommentPath` が通るものだけ。
- [ ] **Step 4: `make check`**
- [ ] **Step 5: コミット** `fix: 空のコミットと RemoveTask の積み残しを直し、Update がコミットしたかを返す`

### Task 4: gitrepo の Log・Status・Verify

**Files:**
- Create: `internal/gitrepo/log.go`、`status.go`、`verify.go`
- Test: `internal/gitrepo/log_test.go`、`status_test.go`、`verify_test.go`

**Interfaces:**
- Produces（spec の「gitrepo」の型そのまま）:
  - `type LogEntry struct{ Hash plumbing.Hash; Author Signature; When time.Time; Subject string; Trailers []Trailer }`、`func (r *Repo) Log(prefixes []string, limit int) ([]LogEntry, error)`
  - `type FileStatus struct{ Path string; Staged, Unstaged bool }`、`func (r *Repo) Status() ([]FileStatus, error)`
  - `func (r *Repo) Verify() error`
  - `func (e LogEntry) Trailer(key string) (string, bool)`（最初の一致）

- [ ] **Step 1: 失敗するテストを書く**
  - Log: 3コミット（`a.md`、`d/b.md`、`a.md`）を `Commit` で作り、`Log(nil, 0)` が新しい順に3件、`Log(nil, 2)` が2件、`Log([]string{"a.md"}, 0)` が2件、`Log([]string{"d/"}, 0)` が1件。trailer（`Hedgehug-Op: add`）が `Trailer("Hedgehug-Op")` で読める。コミットのない repo（`Init` 直後）は空で nil エラー。
  - Status: 変更したファイル（Unstaged）、`git add` 相当で stage したファイル（go-git の `Worktree.Add` で。Staged）、未追跡のファイル（Unstaged）、`.gitignore` に書いたファイル（含まれない）。パスの順。
  - Verify: 正常な repo で nil。`.git/objects` の下の blob のファイル（loose object）を1つ切り詰めると、エラーにそのハッシュが入る。
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
  - Log: `r.r.Log(&git.LogOptions{PathFilter: ...})`。`PathFilter` は接頭辞のどれかで始まるか。HEAD がない（`plumbing.ErrReferenceNotFound`）なら空。Subject はメッセージの1行目、Trailers は最後の段落の `Key: Value` 行（`message()` の逆）。
  - Status: `Worktree.Status()` の各 `FileStatus` で、`Staging != Unmodified` を Staged、`Worktree != Unmodified` を Unstaged。`Untracked` は Unstaged。
  - Verify: HEAD から `object.Commit` を親へたどり（`r.r.Log`）、各コミットの `Tree()` を `Files()` ではなくツリーを再帰で歩いてツリーと blob を `Storer.EncodedObject` で読み、`Reader` を最後まで読む。見たハッシュは map で飛ばす。
- [ ] **Step 4: `make check`**
- [ ] **Step 5: コミット** `feat: gitrepo にコミットの履歴、作業ツリーの状態、オブジェクトの検査を足す`

### Task 5: config の直し

**Files:**
- Modify: `internal/config/config.go`、`internal/config/author.go`
- Test: `internal/config/config_test.go`、`internal/config/author_test.go`

**Interfaces:** 外から見える関数は変えない。

- [ ] **Step 1: 失敗するテストを書く**
  - 字下げ: `stores:\n  - name: a\n    path: /x\n` を読んで `AddStore` → `Save` すると、両方の要素が `  - name` の字下げで書かれる。`stores:\n- name: a\n  path: /x\n` なら `- name` のまま。ファイルがなかったときは `  - name`。
  - symlink: `config.yaml` を別のファイルへの symlink にして `Save` すると、symlink が残り、リンク先が書き換わる（Windows では symlink を作れなければ `t.Skip`）。
  - コメントだけ: `# my config\n` だけのファイルに `AddStore` → `Save` するとコメントが残る。
  - 相対パス: `HEDGEHUG_HOME` の config.yaml に `path: stores/me` と書くと、`Select` の `Path` が `<HEDGEHUG_HOME>/stores/me`（カレントディレクトリではない）。
  - include: `GIT_CONFIG_GLOBAL` のファイルが `[include]\n\tpath = inc.conf` を持ち、同じディレクトリの `inc.conf` に `[user]` がある → 作者が読める。`~/x.conf` の展開。後に書かれたほうが勝つ（include の後に `[user] name` があればそれ）。自分を include するループで深さ 10 で止まりエラーにならない。`[includeIf "gitdir:..."]` は読まない。
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
  - 字下げ: 読み込んだ `stores` の列の最初の要素の列番号（`Column`）と `stores` キーの列番号の差で「列を字下げしているか」を決め、`yaml.Dump` に `yaml.WithIndent(2)` と、列を字下げする指定（yaml/v4 の `WithSeqIndent` 等。v4 の Dumper のオプションを `go doc go.yaml.in/yaml/v4` で確かめて使う）を渡す。
  - symlink: `filepath.EvalSymlinks(c.file)` が成功したら、そのパスに一時ファイルを作って rename する。
  - コメントだけ: `yaml.Load` が中身のない DocumentNode（コメントを `HeadComment` に持つ）を返したら、その DocumentNode に mapping を足し、コメントを保つ。
  - 相対パス: `expand` で `~/` でも絶対でもなければ `filepath.Join(filepath.Dir(c.file), p)`。
  - include: gitconfig を読む関数を再帰にし、`include` セクションの `path` を順に読む。深さを引数で持つ。
- [ ] **Step 4: `make check`**
- [ ] **Step 5: コミット** `fix: config.yaml の字下げ・symlink・コメント・相対パスと、gitconfig の include を扱う`

### Task 6: CLI の直し

**Files:**
- Modify: `cmd/hedgehug/store_init.go`、`dep.go`、`ls.go`、`output.go`（`--format` の実行）、`show.go`、`tree.go`、`go.mod`（`golang.org/x/text`）
- Test: `cmd/hedgehug/main_test.go`、`store_init_test.go`、`dep_test.go`、`ls_test.go`、`show_test.go`、`tree_test.go`

**Interfaces:** 出力の形だけが変わる（spec の「持ち越しの直し」の「CLI」）。

- [ ] **Step 1: 失敗するテストを書く**
  - `te.run("store")` と `te.run("dep")` が終了コード 2 で、stderr に使い方。`te.run("store", "init", "Bad Name")` が 2。重複した名前は 1 のまま。
  - `te.run("ls", "--format", "{{.nosuch}}")` が 2。`{{.title}}` は通る。`--format '{{'`（解析の誤り）も 2。`ls --tree --format` も同じ。
  - show: タイトルや props の名前に全角の文字を含むとき、ラベルの列が表示幅で揃う（`show` の Related のラベルに全角の inverse 名を持つ task_ref を使う。既存の `show_test.go` の Related のテストに倣う）。
  - tree: A→B→A の parent の循環に、B の子 C（循環でない）を置く。`ls --tree` の `== in a parent cycle` の下に A と B が並び、C は B の子として字下げして出る。`--json` で A と B だけが `in_cycle: true`、C は B の `children` にあり `in_cycle` がない。`--format` の `in_cycle` も同じ。`--ids-only` は今の規則（一致したタスクだけ、描く順）。
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**
  - `store` / `dep`: `RunE` で使い方を stderr に出して `usageErrorf("a subcommand is needed")`（ルートと同じ形）。
  - `store init` の名前: `config.ValidStoreName` の誤りを `usageError{err}` で包む。
  - `--format`: `template.New("format").Option("missingkey=error")`。実行時の誤りも `usageError` で包む。
  - show の幅: `golang.org/x/text/width` の `LookupRune(r).Kind()` が `EastAsianWide` か `EastAsianFullwidth` なら 2 として数える `displayWidth(s string) int` を `output.go` に置き、`%-11s` の代わりに空白で埋める。
  - tree: `treeOf` の循環の部分を、`seen` に入らなかった `show` のタスクのうち「parent をたどると自分に戻る」ものを循環のタスクとし、それぞれを根にして `build` する（`seen` は共有するので二重に出ない）。循環のタスクの子のうち循環のタスクは `build` で `seen` 済みなら飛ぶ。循環のタスクには `treeNode.inCycle = true`。`flatten` とJSONと `--format` は `treeNode` から作る。
- [ ] **Step 4: `make check`**
- [ ] **Step 5: コミット** `fix: 使い方の誤りの終了コード、--format の未知のキー、show の表示幅、循環の入れ子を直す`

### Task 7: 書いていなかったテスト（1c・1d-1）

**Files:**
- Test: `internal/index/*_test.go`、`cmd/hedgehug/store_init_test.go`、`status_test.go`、`dep_test.go`、`mv_test.go`

テストだけのタスク。落ちたら原因を直す（直したらコミットの種類は `fix:`）。

- [ ] **Step 1: テストを書く**
  - index: A→B→C→A の parent の3つ組の循環で、3つとも `cycle` の問題を持ち、`Roots` に出ない。
  - index: 削除済みのタスクを含めて末尾4文字が重なる ID の組で、`ShortID` が削除済みも数に入れて伸びる。
  - index: 同じ Store を、キャッシュを消して開いた index と、キャッシュから開いた index で、`Tasks()` の全 `Entry` が `reflect.DeepEqual`（ID の順に並べて比べる）。
  - index: parent の循環を含む Store の `Open` が成功する。
  - index: `update` の途中で失敗させ（壊れたキャッシュではなく、テスト用の差し替えでツリーの読み出しを1回失敗させる。差し替えの口がなければ、`Refresh` の前に schema.yaml を壊したコミットを作って失敗させる）、`Open` し直すと正しい状態になる。
  - cmd: `store init` で config.yaml の書き込みを失敗させる（config.yaml のディレクトリを読み取り専用にする。Windows では `t.Skip`）と、Store のディレクトリが残らない。
  - cmd: `status` が遷移の規則で拒否されたとき、`dep add` が循環で拒否されたとき、どちらもコミットが増えない。
  - cmd: `mv A --before A` がエラー。削除済みのタスクの `mv` がエラー（`restore` を案内する）。`dep add A --blocked-by A` がエラー。
- [ ] **Step 2: 実行し、落ちるものは原因を直す**
- [ ] **Step 3: `make check`**
- [ ] **Step 4: コミット** `test: 1c と 1d-1 で足りなかった index と CLI のテストを足す`

### Task 8: エディタの共通処理と `edit`

**Files:**
- Create: `cmd/hedgehug/editor.go`、`cmd/hedgehug/edit.go`
- Modify: `cmd/hedgehug/app.go`（コマンドの登録）、`cmd/hedgehug/helpers_test.go`（テスト用エディタ）
- Test: `cmd/hedgehug/edit_test.go`

**Interfaces:**
- Consumes: `Store.TaskBytes`、`Store.Update` の `committed`（Task 3）
- Produces:
  - `func (a *app) editFile(name string, content []byte) (path string, edited []byte, err error)`: 一時ファイル `hedgehug-<name>-*.md` を作って書き、エディタで開き、読み戻す。ファイルは消さない（呼び出し側が成功時に `os.Remove`）。
  - `func (a *app) reopen(path string, content []byte) ([]byte, error)`: 同じパスにもう一度書いて開く。
  - `func (a *app) askRetry() bool`: stdin が TTY なら「Open the editor again? [Y/n] 」を stderr に出し、空か y/Y で true。TTY でなければ false。
  - `func (a *app) editor() ([]string, error)`: `$VISUAL`、`$EDITOR`（`a.getenv`）、なければ `vi`（Windows は `notepad`）を `strings.Fields` で分ける。
  - エディタの起動: `exec.Command(editor[0], append(editor[1:], path)...)`、`Env` は `os.Environ()`、stdin/stdout/stderr は `a.stdin` / `a.stderr` / `a.stderr`（stdout は JSON の出力を汚さないよう stderr にする）。エディタのコマンドは `a.getenv` で決める。
  - テスト用エディタ: `helpers_test.go` に `TestMain` を足す。プロセスの環境変数 `HH_TEST_EDITOR_SCRIPT` があれば、テストを走らせずにエディタ役として動いて終わる。スクリプトのファイルは `---next---` の行で区切った手順の列で、開くたびに先頭の1つを消費して残りを書き戻す。手順の中身は「最後の引数のファイルに書く内容」。特別な行: 先頭が `EXIT1` なら何も書かずに終了コード 1、`TOUCH <path>` の行は書いた後でそのパスに `x` を追記する。開いたときのファイルの中身は `<script>.seen` に追記する（`---next---` 区切り）。
  - `te.editWith(contents ...string) (seen func() []string)`: スクリプトを `t.TempDir()` に書き、`t.Setenv("HH_TEST_EDITOR_SCRIPT", path)`、`te` の `EDITOR` を `os.Args[0]` にする（`testEnv` に `env map[string]string` を足し、`getenv` が既存の分の後にこれを引く）。`t.Setenv` を使うので、エディタを使うテストは `t.Parallel()` しない。

- [ ] **Step 1: 失敗するテストを書く**（spec の `edit` の 1〜7 の各項目）
  - 変更なし → `No changes`、終了 0、コミットなし、一時ファイルが消えている。
  - タイトルを変える → 1コミット、`Hedgehug-Op: edit`、`show` で新しいタイトル。
  - `id` を変える → TTY でない: 終了 1、stderr に理由と一時ファイルのパス、ファイルが残る。TTY（`te.terminal = true`、`te.stdin = "y\n"`）で2回目に正しい内容 → 成功。2回目の開き直しは前回の編集内容（スクリプトが書いた内容）で開くこと（スクリプト側で、開いたときの中身を別ファイルに書き出して確かめる）。
  - `parent` を変える → `mv` を案内して拒否。
  - 検査の違反（未知のステータス）→ 再編集の確認。`n` で終了 1。
  - 壊れたタスク（ファイルを手で壊してコミット）→ 生のバイトで開き、正しい内容にすると直る。
  - 編集中に変更（エディタ役が、編集した後で Store のタスクファイルも書き換える。スクリプトの指示 `TOUCH <path>` で実装）→ 「changed while editing」、終了 1、確認は出ない。
  - 削除済み → `restore` を案内して終了 1、エディタを開かない。
  - エディタが終了コード 1 → 中止、終了 1。
- [ ] **Step 2: 失敗を確かめる**
- [ ] **Step 3: 実装する**（spec の `edit` のとおり。`Update` の fn の中で `TaskBytes` と開いたときのバイトを比べる。デコードは `model.DecodeTask`。拒否するキーは、元のタスクと比べる: `ID`、`Key`、`Created`、`Updated`、`DeletedAt`、`Parent`、`Rank`。壊れたタスクで元がデコードできないときは、パスの id と一致することだけを求める）
- [ ] **Step 4: `make check`**
- [ ] **Step 5: コミット** `feat: edit で $EDITOR からタスクを書き換える`

### Task 9: `comment`（追加・編集・削除）とコメントの短縮 ID

**Files:**
- Create: `cmd/hedgehug/comment.go`
- Modify: `cmd/hedgehug/show.go`（コメントの短縮 ID と author の `-`）、`app.go`
- Test: `cmd/hedgehug/comment_test.go`、`show_test.go`

**Interfaces:**
- Consumes: `editFile`（Task 8）、`Store.CommentIDs`（Task 3）、`index.Resolve` の規則
- Produces: `type commentIndex struct{...}`、`func loadComments(s *store.Store) (*commentIndex, error)`、`func (ci *commentIndex) Resolve(arg string) (store.CommentRef, error)`、`func (ci *commentIndex) Short(id model.ID) string`。短縮の規則は `internal/index/shortid.go` と同じ（最低4文字、末尾一致、大文字小文字を区別しない）。そのファイルの関数を流用できる形（ID の列を受ける関数）なら index から export して使い、できなければ同じ規則を `comment.go` に書く。曖昧なら `ambiguousError`（候補の短縮 ID とタスクのタイトル）。

- [ ] **Step 1: 失敗するテストを書く**（spec の `comment` の各項目）
  - `comment <id> hello world` → 本文 `hello world`、1コミット `Hedgehug-Op: comment`、`Hedgehug-Task` がタスク、出力にコメントの短縮 ID。`--json` の形。
  - 引数なし → エディタで本文。空白だけ → 終了 1、コミットなし。
  - 削除済みのタスク → `restore` を案内。
  - `comment edit <cid>` → 本文が変わる。変更なし → `No changes`。空 → `comment rm` を案内。
  - `comment rm <cid>` → `deleted_at` が付き、`show` に出ない。2回目 → エラー。削除済みの `edit` → エラー。
  - 末尾が重なる2つのコメントで、短い入力が曖昧のエラー、長い入力で解決。
  - `show` がコメントの短縮 ID と `-`（author 空）を出す。`show --json` の `comments[].short`。
- [ ] **Step 2〜5:** 失敗を確かめ、実装し、`make check`、コミット `feat: comment でコメントを足し、書き換え、消す`

### Task 10: `log` と `show` の履歴

**Files:**
- Create: `cmd/hedgehug/log.go`
- Modify: `cmd/hedgehug/show.go`、`app.go`
- Test: `cmd/hedgehug/log_test.go`、`show_test.go`

**Interfaces:**
- Consumes: `gitrepo.Log`、`LogEntry.Trailer`（Task 4）
- Produces: `type logJSON struct{ Hash string; Date time.Time; Author struct{Name, Email string}; Op, Task *string; Subject string }`（JSON のキーは spec のとおり小文字）、`func (a *app) taskLog(s *store.Store, id model.ID, limit int) ([]gitrepo.LogEntry, error)`（接頭辞は `model.TaskPath(id)`、`path.Dir(model.CommentPath(id, model.ID{}))+"/"`、worklog の同じもの）

- [ ] **Step 1: 失敗するテストを書く**
  - `add` を2つ、片方に `set` と `comment` → `log` が5行（init を含む）で新しい順、`log <id>` がそのタスクの3行（add、set、comment）、`-n 1` が1行、`-n -1` が終了 2。行の形 `^[0-9a-f]{7}  \d{4}-\d{2}-\d{2} \d{2}:\d{2}  <name>  hedgehug: `。
  - `--json` の各キー、init の `task` が null。
  - `show` の `History` の節が最大5行、`show --json` の `history`。
  - 日時はローカル時刻（`te` の `TZ` を固定できなければ、`time.Local` で整形した期待値と比べる）。
- [ ] **Step 2〜5:** 失敗を確かめ、実装し、`make check`、コミット `feat: log と show の履歴でコミットを出す`

### Task 11: `schema migrate`

**Files:**
- Create: `internal/schema/migrate.go`、`cmd/hedgehug/migrate.go`、`cmd/hedgehug/schema.go`（`schema` の親コマンド。単体は使い方を出して 2）
- Test: `internal/schema/migrate_test.go`、`cmd/hedgehug/migrate_test.go`

**Interfaces:**
- Consumes: `Op.SkipTransitions`、`Update` の `committed`（Task 3）、Encoder の対応づけ（Task 2）
- Produces:
  - `type Target string`（`TargetStatus = "status"`、`TargetKind = "kind"`、`TargetProp = "prop"`、`TargetValue = "value"`）
  - `type Migration struct{ Target Target; Prop, Old, New string }`（`Prop` は `value` のときだけ）
  - `func (s *Schema) Migrate(m Migration) (*Schema, error)`: 新しい Schema を返す（`s` は変えない。`doc` は引き継ぐので Encode でコメントが残る）。spec の表の schema.yaml の欄を書き換え、改名か統合をする。`<old>` がない、同じ名前、prop の型の違い、value が enum でないときはエラー。
  - `func (m Migration) ApplyTask(t *model.Task, s *Schema) (changed bool, err error)`: タスクの欄を書き換える。prop の統合で両方に値があるときはエラー。
- [ ] **Step 1: 失敗するテストを書く**
  - schema: 4つの対象それぞれの改名と統合で、spec の表のすべての参照が書き換わる（transitions のキーと値、required のキー、scoped の statuses と workflow、kinds の parents、templates.default のキー、required の値、props の values）。統合での重複の除去。status の統合で既定の引き継ぎ（同じカテゴリ）と、違うカテゴリで `Check` が拒否すること。改名した status のコメントと未知のキーが Encode で残る。
  - schema: `ApplyTask` の各対象。multi の値の重複の除去。
  - cmd: `schema migrate status todo ready` → schema.yaml とタスクが1コミット（`Hedgehug-Op: migrate`）、`ls status:ready` で引ける。削除済みのタスクも変わる。壊れたタスクは警告に出て残る。遷移の規則（`todo` から `ready` に移れない）があっても通る。必須項目で新しい違反になるなら全体を拒否し、コミットなし。`--dry-run` は一覧だけでコミットなし。`--json` の形。引数の数の誤りは 2。
- [ ] **Step 2〜5:** 失敗を確かめ、実装する（cmd は `store.Tasks()` を回して `ApplyTask`、変わったものを `PutTask`、`PutSchema`、`Op{Name: "migrate", SkipTransitions: true}`。壊れたタスクは `Tasks()` のエラーから `model.DecodeError` を拾い、パスの ID を index の短縮 ID にする）、`make check`、コミット `feat: schema migrate で定義の名前と値を一括で変える`

### Task 12: `internal/doctor` の Finding と `schema check`

**Files:**
- Create: `internal/doctor/finding.go`、`internal/doctor/schema.go`、`cmd/hedgehug/schema.go` に `check` を足す（Task 11 で作った親コマンド）
- Test: `internal/doctor/schema_test.go`、`cmd/hedgehug/schema_test.go`

**Interfaces:**
- Produces:
  - `type Finding struct{ Check, Where, Msg, Hint string }`（JSON タグ `check`、`where`、`msg`、`hint,omitempty`）
  - `func SchemaFile(data []byte) (*schema.Schema, []Finding)`: schema.yaml のデコードと `Check`（spec の 1・2）。デコードできなければ Schema は nil。
  - `func Schema(s *store.Store, ix *index.Index) ([]Finding, error)`: spec の 2〜6。Where のタスクは `ix.ShortID(id) + " " + title`。
  - `func Sort(fs []Finding)`: Check の順（spec の表の順）→ Where。
  - cmd: `func (a *app) printFindings(fs []doctor.Finding) error`（テキストと JSON。1件以上なら `errFindings`（終了 1、stderr には何も足さない番兵）を返す）。`run` で番兵を見て、メッセージを出さずに 1 で終わる。
- [ ] **Step 1: 失敗するテストを書く**（spec の `schema check` の 1〜6 をそれぞれ1件ずつ起こすケースと、問題なしのケース。cmd はテキストの形、`N problems`、`No problems`、JSON、終了コード、schema.yaml が壊れて `store.Open` できないときに 1 だけを出すこと）
- [ ] **Step 2〜5:** 失敗を確かめ、実装し、`make check`、コミット `feat: schema check でスキーマと既存タスクの不整合を出す`

### Task 13: `doctor`

**Files:**
- Create: `internal/doctor/doctor.go`、`cmd/hedgehug/doctor.go`
- Test: `internal/doctor/doctor_test.go`、`cmd/hedgehug/doctor_test.go`

**Interfaces:**
- Consumes: `SchemaFile`、`Schema`（Task 12）、`gitrepo.Status`・`Verify`（Task 4）
- Produces: `func Run(dir string) ([]Finding, error)`: spec の `doctor` の 1〜5。`store.Open` と `index.Open` は 1 で schema.yaml が読めたときだけ使う。`gitrepo.Open(dir)` は自分で開く。cmd は `c.Select(a.storeName)` で dir を決め、`store.Open` を通さずに `doctor.Run`。
- [ ] **Step 1: 失敗するテストを書く**（spec の doctor のテストの各ケース: 壊れた schema.yaml で 5 を飛ばした旨の Finding、id とパスの食い違い、規則に合わない名前、タスクのないコメント、未コミットの変更の stage あり・なしと Hint、Verify の失敗、index の問題（missing_ref）、問題なし。cmd は終了コードと出力）
- [ ] **Step 2〜5:** 失敗を確かめ、実装し、`make check`、コミット `feat: doctor で Store のファイル・作業ツリー・git・スキーマを検査する`

### Task 14: devenv

**Files:**
- Create: `tools/devenv/main.go`、`tools/devenv/main_test.go`
- Modify: `.gitignore`（`/.devenv/`）

**Interfaces:**
- Produces: `func run(args []string, stdout, stderr io.Writer, cwd string) int`（`main` は `os.Args[1:]` と `os.Getwd()` で呼ぶ）。サブコマンドは標準の `flag` で `init`、`reset`、`env`（cobra は使わない。開発用で引数が少ないため）。
- [ ] **Step 1: 失敗するテストを書く**（spec の devenv のテスト: `init` で `users/alice` と `users/bob` に `.gitconfig`、`config.yaml`、`stores/me` の Store（`store.Open` できる、init のコミットの作者が alice）。既存の dir で `init` が 1。`reset` で作り直し（中に置いた余計なファイルが消える）。目印のない dir の `reset` が 1 で何も消さない。`env alice` の2行と絶対パス。存在しない利用者が 1。dir は `--dir` で `t.TempDir()` を渡す）
- [ ] **Step 2〜5:** 失敗を確かめ、実装する（場所は `config.Resolve` に `HEDGEHUG_HOME=<dir>/users/<u>` を返す getenv を渡して決め、作者は `gitrepo.Signature{Name: u, Email: u + "@example.com"}`、`store.Create` → `config.Load` → `AddStore` → `Save`）、`make check`、コミット `feat: devenv で alice と bob の開発環境を作る`

### Task 15: シナリオと仕上げ

**Files:**
- Modify: `cmd/hedgehug/scenario_test.go`
- Modify: `docs/superpowers/specs/2026-10-30-phase1d2-maintenance-design.md`（実装で確定した細部があれば）

- [ ] **Step 1: シナリオを足す**（spec の「テスト」のシナリオ: 既存の流れに、`edit`（テスト用エディタ）、`comment` の追加・編集・削除、`log <id>`、`schema migrate status`、`schema check`（0）、最後に `doctor`（0、`No problems`）を足し、各書き込みのコミットが1つずつ増えることを確かめる）
- [ ] **Step 2: `make check`、`make build`、`GOOS=windows go vet ./...`、`GOOS=darwin go vet ./...`**
- [ ] **Step 3: コミット** `test: フェーズ1の完了条件を通しのシナリオで確かめる`

親設計ドキュメント（Claude Docs）のパッケージ表とコメントのパスの直しは、コントローラーが Docs のツールで行う（サブエージェントのタスクにしない）。
