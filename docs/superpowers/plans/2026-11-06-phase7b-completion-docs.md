# フェーズ 7b 補完と文書 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `hh completion` と動的な補完を足し、README と `docs/guide/` を書く。

**Architecture:** 候補の表を `internal/app/cli/completion.go` の1か所に置き、コマンドの木を歩いて `ValidArgsFunction` と `RegisterFlagCompletionFunc` に配線する。候補のうちメンバーとスキーマの値は `browse/complete.go` から読む。

**Tech Stack:** cobra 1.10

spec: `docs/superpowers/specs/2026-11-06-phase7-finish-design.md` の「7b 補完と文書」

spec から直した点（spec も直した）:

- `hh set` の担当者のキーは `assignees=`（`assignee=` はない）。値はアカウントの ID（`github.com/alice`）
- `hh use` の引数は Store ではなくプロジェクトなので、タスク ID を出す
- 自動化のルールの ID（`automation rm`・`enable`・`disable`）も補完する

---

### Task 1: `completion` コマンドと `beforeRun`

**Files:**
- Create: `internal/app/cli/completion.go`、`internal/app/cli/completion_test.go`
- Modify: `internal/app/cli/app.go`、`internal/app/cli/run.go`

- [ ] **Step 1: 失敗するテストを書く**

`hh completion bash|zsh|fish` の出力に実行名が入ること、`powershell` が使い方の誤りになること、アカウントなしで動くこと、`__complete` がアカウントなしで失敗しないこと、`__complete` が裏の同期を起動しないことを確かめる。

- [ ] **Step 2: 実装する**

`env.progName`（`Main` が `filepath.Base(os.Args[0])` を入れる）が `hh` なら root の名前を `hh` にして生成する。`beforeRun` の冒頭で `completion`、`__complete`、`__completeNoDesc` なら何もしない。`app.go` の「completion comes in phase 7」のコメントを消す。

- [ ] **Step 3: テストを通して Commit**

### Task 2: 動的な補完

**Files:**
- Create: `internal/app/usecase/browse/complete.go`
- Modify: `internal/app/cli/completion.go`、`internal/app/cli/completion_test.go`

- [ ] **Step 1: 失敗するテストを書く**

`__complete` の候補のテーブルテスト: `show`（ID と説明のタイトル）、`status <id>`（ステータス）、`set <id>`（`key=`）、`set <id> assignees=`、`ls assignee:`、`--store`、`--workspace`、`--parent`、`--sprint`、`--release`、`--template`、`view show`。Store がないときに候補なしで終了コード 0 であること。`<id>` と `<project>` と `<workspace>` を取るすべてのコマンドに補完があること（木を歩く検査）。

- [ ] **Step 2: 実装する**

表 `argCompleters`（コマンドのパス → 位置ごとの候補。最後の要素は残りの引数にも使う）と `flagCompleters`（フラグの名前 → 候補）を置き、`wireCompletion` で木を歩く。`store init` はフラグの補完の対象外（`--template` も `--workspace` も別の意味）。読み取りの失敗は候補なしにする。

- [ ] **Step 3: テストを通して Commit**

### Task 3: シェルでの手の確認

- [ ] **Step 1: bash と zsh で確かめる**

`make build` のバイナリで、Store を作り、`show <TAB>`、`status ID <TAB>`、`set ID assignees=<TAB>`、`ls assignee:<TAB>` を確かめる。`=` と `:` を含む語が bash と zsh で壊れないこと。

- [ ] **Step 2: fish で確かめる**

Debian のコンテナで、bash 5.2（bash-completion あり）と fish 4.0 を確かめる。手元の環境に fish を入れない。

### Task 4: 文書

**Files:**
- Modify: `README.md`、`README.ja.md`、`CONTRIBUTING.md`
- Create: `docs/guide/*.md`、`docs/guide/ja/*.md`（`getting-started`、`sync`、`stores`、`github`、`planning`、`maintenance`）

- [ ] **Step 1: 書く前に動作を確かめる**

文書に書くコマンドは、実際に実行して出力を確かめる。

- [ ] **Step 2: 日本語を書き、英語を書く**

spec の「文書」と「先送りされた文書」のとおり。README の開発の節は CONTRIBUTING に移す。

- [ ] **Step 3: Commit**

### Task 5: 仕上げ

- [ ] **Step 1: `make check` を通す**
- [ ] **Step 2: 引き継ぎ `2026-11-06-phase7b-handoff.md` を書き、前の回の引き継ぎの「次の回」を直す**
- [ ] **Step 3: PR を出す**
