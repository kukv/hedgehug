# コードベースの構成見直し 設計

- 日付: 2026-11-04
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「パッケージ構成」
- 前段: 2026-11-03 の調査（依存図: https://claude.ai/artifact/GM5cKh5j1wKHn9mTPGxMkq）で出した提案 A〜I。E（doctor --fix の force push 復旧を同じディレクトリ内で直す方式）は別の spec で扱う

## 目的

後からコードを読んでも見通しがよい構成にする。具体的には次の 3 つ。

- パッケージを層に並べ、層の向きを lint で強制する
- CLI と server が usecase だけを通して動くようにする
- OS に触る処理を 1 か所に集め、いつか Windows 対応に戻るときに書く場所をそこだけにする

挙動は変えない。例外は 4.3 の `--json` の 2 か所だけ。Windows 版の実装は含めない（CI が遅い問題の解き方が決まっていないため。2026-11-04 利用者が決定）。

## 決定の一覧（2026-11-03〜04 に利用者が決定）

| # | 対象 | 決定 |
| --- | --- | --- |
| 1 | 大枠 | `internal/lib/`（hedgehug を知らない部品）と `internal/app/`（hedgehug 固有）に分け、境界を depguard で強制する |
| 2 | 層 | lib → domain → storage → remote → usecase → cli / server。各グループは自分・下の層・lib だけを import する |
| 3 | view | domain ではなく storage に置く（Store の中のファイルの形式と読み書きなので） |
| 4 | mount と current | 責務ごとに解体して既存の層に配る。新しい層は作らない |
| 5 | usecase の分け方 | 直下を土台にし、領域ごとのサブパッケージ 11 個に分ける（2.3） |
| 6 | 名前 | ops → usecase、sync → replicate、context → current。usecase の領域名は下の層と同じ名前にしない |
| 7 | JSON の形 | CLI の `--json` を server と同じ usecase の型に一本化する。CLI にしかない項目は足す |
| 8 | Windows | 実装は含めない。osfs の `_unix.go` に閉じ込めるところまで |
| 9 | spec の単位 | 構成（本 spec）と E を分ける |
| 10 | PR | 6 つに分け、機械的な移動を最初に行う。最後の PR 6 は、完了後のパッケージ構成を見て利用者が最終判断をする回（6 節。PR 6 は 2026-11-04 利用者が追加） |
| 11 | cli・server のテスト | テストも usecase を通す。データの準備と書き込み結果の確認を usecase の操作で書き直し、4.2 のルールをテストにもかける。回 4 で本体と一緒に行う（2026-11-04 利用者が決定） |
| 12 | テスト用の部品 | `lib/testenv`・`lib/fakegh`・`lib/gitrepo/gitrepotest`・`storage/history/historytest` は、どの層のテストからも import してよい（2026-11-04 利用者が決定） |

## 1. 今の状態（main f965465）

- `internal/` に 23 パッケージが平らに並んでいる。ops は 36 ファイル・1.09 万行（テスト 1.6 万行）。
- `cmd/hedgehug` は 37 ファイル・4,400 行で、CLI の本体がここにある。storage と remote のパッケージを 16 ファイルで直接使っている。server も 7 ファイルで直接使っている。
- 層を逆にまたぐ依存がある。
  - `github` → `config`（`config.Host` と `config.PlainHTTP` だけ）
  - `auth`・`config` → `store`（`store.WithFileLock` だけ）
  - `mount` → `sync`（`ReadParent`・`LeftoverIn`・`LeftoverError`）
  - `history` のテスト（`bench_test.go`）→ `ops`
- OS ごとに分けたファイルは `atomicfile_unix.go`・`store/lock_unix.go`・`sync/spawn_unix.go` の 3 つだけ。`os.Rename`・`os.Remove(All)`・`os.ReadFile` などの直接の呼び出しが 10 パッケージ以上に散らばっている（3 節）。

## 2. 最終的な配置

### 2.1 ディレクトリ

```
cmd/hedgehug/main.go        cli.Main(version) を呼ぶだけ
cmd/fakegh/                 今のまま
internal/lib/
  osfs/                     改名・削除・読み書き・ディレクトリ作成・ロック・プロセス起動・パスの比較（旧 atomicfile を統合）
  plural/
  gitrepo/ (+gitrepotest/)
  giturl/                   remote URL の分解と正規化（旧 context の Split・RepoID）
  github/
  fakegh/
  testenv/
internal/app/
  domain/
    model/
    schema/
    merge/
  storage/
    store/                  旧 store ＋ force push 復旧の途中残りの検出（旧 sync の ReadParent・LeftoverIn・LeftoverError）
    index/
    history/ (+historytest/)
    inbox/
    query/
    config/
    view/
    mount/                  マウント中の Store の集合の開け閉め（旧 mount の Open・Set・Mounted）
    current/                context.json の読み書き（旧 context の State・Load・Save）
  remote/
    replicate/              旧 sync
    auth/
  usecase/                  土台（2.3）
    tasks/ comments/ browse/ views/ schemas/ workspaces/
    stores/ replication/ notifications/ accounts/ maintenance/
  cli/                      旧 cmd/hedgehug の本体
    prompt/                 確認と選択の部品（4.4）
    scenario/               コマンドをまたぐ通しのテスト（テストだけのパッケージ。5 節）
  server/ (+openapigen/)
web/                        今のまま
api/openapi.yaml            今のまま
```

### 2.2 旧パッケージの行き先

| 旧 | 新 |
| --- | --- |
| atomicfile | lib/osfs に統合 |
| plural, gitrepo, gitrepotest, github, fakegh, testenv | lib/ の同名 |
| model, schema, merge | app/domain/ の同名 |
| store, index, history, historytest, inbox, query, config, view | app/storage/ の同名 |
| sync | app/remote/replicate（途中残りの検出は storage/store へ） |
| auth | app/remote/auth |
| mount | Open・Set・Mounted → storage/mount。Resolve・Moved・Hit → usecase 直下。Query・Entry・Group・Result → usecase/browse。CrossRefs・TaskRef・CrossRef → usecase/browse |
| context | Split・RepoID → lib/giturl。State・Load・Save → storage/current。Resolve と入出力の型（Input・Destination・Confirm・Choice ほか）→ usecase 直下 |
| doctor | app/usecase/maintenance |
| ops | app/usecase（2.3） |
| server, openapigen | app/server/ の同名 |
| cmd/hedgehug の main.go 以外 | app/cli |

### 2.3 usecase の分け方

直下（パッケージ `usecase`）は、どの領域からも使う土台にする。

- session（Store を開く、書き手、1 操作 1 コミット）、errors、TaskView などの返す型（旧 view.go）、readonly、conn（GitHub への接続とアカウントの選択）、ResolveRef（旧 mount_ops.go）
- mount から移す ID の解決、context から移す行き先の判断
- 領域をまたいで使うヘルパー（task.go の `NotDeleted`・`LiveParent`・`EndRank` など）。どれを下ろすかは plan で、領域をまたぐ呼び出しを洗い出して確定する

| パッケージ | 旧 ops のファイル |
| --- | --- |
| tasks | task, addhere, assign, edit, patch, move, delete, showat, duplicates, mention, progress, undo |
| comments | comment |
| browse | read, people, crossrefs ＋ mount の Query・CrossRefs |
| views | views |
| schemas | maintain の SchemaCheck・Migrate, schemaview, scoped |
| workspaces | workspace |
| stores | storeops, discover, movestore, use |
| replication | sync ＋ git の merge driver の操作（4.2） |
| notifications | inbox |
| accounts | auth |
| maintenance | maintain の Doctor, purge, rebuild ＋ 旧 doctor |

領域どうしの import は、土台を通すのを基本にする。領域から別の領域を呼ぶ必要が出たら、その関数を土台に下ろす。

## 3. OS の境界（提案 D）

今、OS に触る呼び出しは次のとおり（テストを除く）。

| 呼び出し | 件数 | 主な場所 |
| --- | --- | --- |
| `os.Rename` | 12 | sync, config, atomicfile, historytest |
| `os.Remove`・`os.RemoveAll` | 44 | gitrepo, cmd, store, sync, config, ops |
| `os.ReadFile` | 25 | cmd, sync, ops, view, doctor, context, config ほか |
| `os.OpenFile`・`os.WriteFile`・`os.MkdirAll` | 20 | store, sync, gitrepo, config |
| ロック（flock） | 2 | store, gitrepo |
| `exec.Command` | 3 | sync（バックグラウンド同期）, auth（ブラウザ）, cmd（エディタ） |

- `lib/osfs` にこれらの薄い関数を置き、OS ごとの差は osfs の `_unix.go` の中だけに置く。atomicfile は osfs に統合する。
- `store.WithFileLock` を osfs に移す。auth と config から store への依存がなくなる。
- forbidigo で、osfs 以外が `os.Rename`・`os.Remove`・`os.RemoveAll`・`os.ReadFile`・`os.WriteFile`・`os.OpenFile`・`os.MkdirAll`・`exec.Command`・`syscall`（`golang.org/x/sys/unix` を含む）を呼ぶことを禁止する。テストのファイルと、テスト用のパッケージ（gitrepotest・historytest・testenv・fakegh）は対象外にする。
- 呼び方は今と同じにする（関数の置き換えだけ）。Windows 版は書かない。

## 4. 入口の境界（提案 A・F・G・H）

### 4.1 cmd/hedgehug（F）

CLI の本体を `internal/app/cli` に移し、`cmd/hedgehug/main.go` は `cli.Main(version)` を呼ぶだけにする。

### 4.2 cli と server が import してよいもの（A）

| 許可 | 禁止 |
| --- | --- |
| usecase（直下と各領域）、domain、lib/plural、lib/osfs、web | storage、remote、lib/gitrepo、lib/github |

- osfs を許可するのは、cli がエディタの起動や一時ファイルに使うため。
- 今 storage・remote を直接使っている処理は usecase に移す。例: `hh show` の履歴が受け取る `gitrepo.LogEntry` を usecase の型にする。git の merge driver が merge を直接呼んでいるのを `usecase/replication` の操作にする。表示に使う `index.Entry` は usecase の返す型に置き換える。
- domain を許可するのは、`model.Task` を表示するたびに usecase で包み直すと、同じ形の型が二重にできるため。
- このルールはテストのファイル（`cli/scenario` を含む）にもかける（決定 11）。例外は決定 12 のテスト用の部品だけ。
- 回 1 の後の時点で、cli のテストは replicate・history・inbox・index・store・gitrepo を、server のテストは replicate・config・history・inbox・store・gitrepo を直接 import している。データの準備（タスクを作る、コミットを積む）と書き込み結果の確認（ファイルの中身、コミットの trailer）に使っている。これを usecase の操作で書き直す。usecase に今ない読み取りが要るときは、usecase の操作として足す（テストだけが使う操作でもよい）。書き直してもテストが確かめる内容は変えない。

### 4.3 JSON の形（G）

- CLI の `--json` を server と同じ usecase の型で出す。CLI にしかない項目はその型に足す。
- 形が変わるのは次の 2 つ。どちらも、新しい形を期待するテストを先に書いてから変える。
  - `hh undo --json`: `undone` が文字列から LogView になる
  - `hh mv --store --json`: server の形に揃える
- それ以外のコマンドの `--json` は、今の出力と同じになることをテスト（既存の golden を含む）で確かめる。

### 4.4 確認とプロンプト（H）

- 今は `askBranchParent`・`askPick`（add）、`askRetry`（editor）、`askScoped`・`confirmMove`（mv）、`confirmPurge`（purge）、`askChoice`（store init）の 7 つが、それぞれ入力の読み取りと再入力を持っている。
- `cli/prompt` に、はい・いいえの確認と一覧からの選択の 2 つを置き、7 つをそれで書き直す。非対話のときの扱い（`--yes`、端末でないときのエラー）も部品の中で一度だけ書く。
- 文言と挙動は変えない。server は確認の形（`Confirm`・`Choice`）を API で返すまま。

## 5. テストの整理（提案 I）

- `history/bench_test.go` の `ops.Session` を使う計測は `usecase/browse` のベンチに移す。history のベンチは history の API だけで測る。
- depguard の層のルールはテストのファイルにもかける。例外は決定 12 のテスト用の部品だけ。
- `final_review_test.go`・`final_fixes_test.go`（ops）と `final_review_test.go`（cmd）は、中のテストを、試している操作のテストファイルに配り直す。
- コマンドをまたぐ通しのテスト（`e2e_test`・`e2e_3a`・`e2e_3b`・`e2e_3c`・`scenario_test`・`golden_test`）を `cli/scenario` に移す。
- ops のテストは 2.3 の領域ごとに分ける。

## 6. 入れる順番

機械的な移動を最初に行い、以降は最終的な配置の上で作業する（同じファイルを 2 回移さないため）。

| PR | 中身 | 挙動 |
| --- | --- | --- |
| 1. 移動と改名 | 2.2 の行き先へ移す（mount と context は解体まで `usecase/mount`・`usecase/current` に仮置きする。doctor はこの時点で `usecase/maintenance` に置く）。名前の変更、F、CI の shard のパスの書き換え | 変わらない |
| 2. OS の境界 | 3 節。github を `github.Options`（API と web の URL）で受け取る形にし、平文 HTTP の判定（IsLoopback・PlainHTTP）を config から github に移す（2026-11-04 利用者が決定）。forbidigo を有効にする | 変わらない |
| 3. usecase の再編 | mount と context の解体（2.2）、途中残りの検出を storage/store へ、2.3 の領域分け | 変わらない |
| 4. 入口の境界 | 4.2〜4.4。cli・server のテストの書き直し（決定 11）。cli・server の depguard のルール（4.2、テストを含む）を有効にする | 4.3 の 2 か所だけ変わる |
| 5. テストと仕上げ | 5 節、depguard の残りのルール（2 節の層の向き）を有効にする、shard の見直し、親設計の書き直し（8 節） | 変わらない |
| 6. 最終確認 | PR 5 のマージ後の main で `go list` から層別のパッケージ構成と依存を作り直し、層別構成の Artifact（https://claude.ai/artifact/WFTVztg3Wvhgj4UnCwzsiH）を更新する。2.1 の配置・2 節の層の向き・4.2 の禁止と突き合わせ、ずれと層の向きに反する依存の一覧を添えて利用者に見せる。利用者が最終判断をする。直すものが出たら、この PR で直す | 変わらない |

- 途中残りの検出を storage へ移すのは PR 3 で行い、E には依存させない。
- CI の shard は、PR 1 でパスだけを書き換える（`ops` → `internal/app/usecase`、`cli` → `internal/app/cli`、`sync` → `internal/app/remote/replicate`）。ここで作るのは `internal/app/usecase` と `internal/app/cli` の単一パッケージで、サブパッケージ（仮置きの mount・current、maintenance、のちの 11 の usecase 領域）は PR 5 で shard を作り直すまで `others` に入る。PR 5 で、CI の要約に出るパッケージごとの時間を見て shard の分け方を決め直す。必須チェックの名前（`test`）は変えない。

## 7. 確かめ方

- 各 PR で `go build ./...`・`go vet ./...`・golangci-lint を通す。全テストは各 PR の仕上げに 1 回回す。
- PR 1〜3・5・6 は挙動を変えないので、既存のテストがそのまま通ることで確かめる。テストの中身の変更は import と型の名前の書き換えだけにする。
- PR 4 は 4.3 のとおり、変わる 2 か所は先にテストを書き、それ以外は今の出力と同じことをテストで確かめる。cli・server のテストの書き直し（決定 11）は、準備と確認の手段を usecase に替えるだけで、確かめる内容（期待する出力・ファイル・コミット）は変えない。PR 4 で cli・server の depguard を有効にした時点で、4.2 の禁止がテストを含めて lint で通ることを確かめる。
- PR 5 で depguard の残りのルールを有効にした時点で、2 節の層の向きが lint で通ることを確かめる。

## 8. 親設計への反映

親設計の「パッケージ構成」節のコードブロックを、2.1 の配置に書き直す。層の向き（決定 2）と、cli・server が usecase だけを通すこと（4.2）を、その下に 2〜3 行で書く。PR 5 で行う。
