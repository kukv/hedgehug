# CI の短縮とローカルのゴミの削減 設計

- 日付: 2026-11-02
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の対応 OS と「開発環境」
- 前段: PR #18（テストの速度）、PR #26（CI の高速化）

## 目的

フェーズ4の続き（4d）に入る前に、次の2つを片付ける。

1. CI の待ち時間を縮める。いまは全体で約 7.5 分かかる
2. ローカルでテストを回したときに残るゴミを減らす（Windows 側の C: の容量に響いている）

## 現状（main の最新の CI、2026-11-02）

| ジョブ | 全体 | テストの工程 |
| --- | --- | --- |
| test (windows-latest, -race) | 7分28秒 | 6分10秒 |
| test (ubuntu-latest, -race) | 3分30秒 | 3分01秒 |
| test (macos-latest, race なし) | 3分26秒 | 2分25秒 |
| lint | 2分36秒 | golangci-lint 2分12秒 |
| test-without-git（ubuntu, race なし, 全テスト） | 1分17秒 | 54秒 |

- 全体の待ちは Windows で決まっている
- ubuntu の全テストは race なしで 54 秒、race ありで 181 秒。Linux では CPU 律速で、時間の約7割は -race
- 2110 件のテストのうち、t.Parallel のないテストファイルが 72 個、固定の sleep が 49 か所ある

### ポートとアダプター化を入れない理由

ファイル操作と git をインターフェースにしてテストをメモリ上で回す案は、「Windows のジョブがアダプターのテストだけになる」のが主な効果だった。Windows をやめると、この効果はほぼなくなる。Linux・macOS ではテストは I/O 律速ではないので、メモリ上の実装に替えても大きくは縮まない。一方で、`cmd/hedgehug` のテスト（1万行強）は本物のディレクトリを前提にしており、ファイルロック・atomicfile の rename・マージドライバのテスト（本物の git とバイナリ）はメモリ上に移せない。変更の量に見合わないため、今回は入れない（利用者の決定）。

## 利用者の決定

1. ポートとアダプター化は今回は入れない
2. Windows は配布もやめる。Windows では WSL を使ってもらう
3. -race は全パッケージに付けたまま、ジョブを分けて同時に回す
4. macOS も ubuntu と同じ分け方でジョブを分け、全テストを回す
5. ローカルのゴミは、C: の容量の減少と /tmp の残りとして現れている。~/.cache/go-build が 20GB 近くになったこともある

## A. Windows をやめる

### 消すもの

- `.goreleaser.yaml` の windows（`goos` の2か所と zip の `format_overrides`）
- Windows 専用のファイル: `internal/store/lock_windows.go`、`internal/sync/spawn_windows.go`、`internal/sync/childlog_windows.go`、`internal/atomicfile/atomicfile_windows.go`。対になる `*_unix.go` は名前とビルド制約をそのまま残す
- 本番コードの Windows の分岐: `internal/config/paths.go` の USERPROFILE・APPDATA・LOCALAPPDATA、`internal/gitrepo/transport.go` のドライブ文字（`driveRE`）、`cmd/hedgehug/editor.go` の `GOOS == "windows"`、`tools/devenv/main.go` の USERPROFILE
- テストの Windows の分岐（`runtime.GOOS == "windows"` の 15 か所ほど、`.exe` の付け足し、Windows でだけ飛ばす処理）と、Windows だけを確かめるテスト
- ci.yaml の Windows 向けの設定（`TMP`・`TEMP` の付け替え、pwsh を避けるための説明）

`go mod tidy` は全 OS の依存を見るので、go-keyring などが Windows 向けに使う間接依存（wincred・go-winio）は go.sum に残ってよい。

### 残すもの

- CRLF を LF にする処理（`model/frontmatter.go`、`merge/yaml.go`、`index/checklist.go`）。WSL のファイルを Windows のエディタで開くと CRLF になることがあるため
- `filepath.ToSlash` などのパスの扱い。Linux・macOS でも正しく、消すと差分が大きくなるだけなので触らない

### 文書

- README.md・README.ja.md・CONTRIBUTING.md の対応 OS を「Linux と macOS。Windows では WSL を使う」にする
- 親設計の対応 OS と配布の記述を同じ内容に直す
- 過去の spec・plan は当時の記録なので直さない

Windows をやめたあとは、`GOOS=windows` のビルドは通らなくてよい。Makefile の `release-check` からも windows を外す。

## B. CI

### テストのジョブを分ける

- test のマトリクスを `os: [ubuntu-latest, macos-latest]` × `shard: 3 つ` にする。ubuntu は -race 付き、macOS は race なし（いまと同じ）
- 分け方は、パッケージの一覧を3つに分けて ci.yaml に書く。最初の PR の CI でパッケージごとの時間を出し（gotestsum の JSON から集計する）、3つの合計が均等になるように決める。候補は `internal/ops`／`cmd/hedgehug`／その他
- 新しいパッケージが漏れないよう、3つ目は「1つ目と2つ目に含まれないすべて」として `go list ./...` から引く
- キャッシュのキーは OS・race の有無・shard の番号で決める。shard ごとにビルドするパッケージが違うため。保存は今と同じく main への push のときだけ、各 shard が自分の分を保存する（1つ約 385MB × 6 と nogit の約 315MB で約 2.6GB。リポジトリの上限 10GB に収まる）

### 結果をまとめるジョブ

- ubuntu の各 shard は JUnit（`test-report-<n>.xml`）とカバレッジ（`coverage-<n>.out`）を artifact として上げる
- report ジョブ（ubuntu、全 shard のあと、`!cancelled()`）がそれらを落とし、カバレッジを1つにつなぐ（2つ目以降の `mode:` の行を除く）。そのうえで Test Report（dorny、`test-report-*.xml`）と octocov に渡す。投稿する条件は今と同じ（同じリポジトリの PR、Dependabot を除く）
### 必須チェック

ruleset `protect-default-branch`（id 26175024）は `test (ubuntu-latest)`・`test (macos-latest)`・`test (windows-latest)` を必須にしている。このままだと Windows のジョブがなくなった PR はマージできない。

- report ジョブの名前を `test` にし、全 shard の結果を見て、1つでも失敗・取り消しがあれば失敗する（`needs` と `if: always()`、`needs.*.result` を確かめる）。shard の数や分け方を変えても、必須チェックの名前は変わらない
- ruleset の必須チェックから3つの `test (…)` を外し、`test` を足す。ruleset の変更は利用者の承認を得てから `gh api` で行う。PR の CI が緑になったのを確かめてから変える

### lint

- golangci-lint-action の自前のキャッシュは PR でも保存している（`skip-save-cache: false`）。テストと同じく保存は main への push だけにする（PR では `skip-save-cache: true`）
- 2分12秒の内訳を最初の PR の CI で確かめる。Go のビルドキャッシュを復元して短くなるなら、テストと同じ方式（保存は main だけ、`go mod verify`）で入れる。短くならなければ入れない

### そのまま残すもの

- test-without-git（54秒で最長の工程ではない）
- release.yaml と security.yaml（Windows を含まない）

### 見込み

ubuntu の race 付きは shard あたり約 1 分、macOS は約 50 秒、それぞれ準備に 30〜60 秒かかる。lint が 2 分台のまま残れば lint が最長になる。全体の待ちは約 7.5 分から 2 分前後になる見込み。

## C. テスト自体を速くする

### t.Parallel

- t.Parallel のないテストファイル 72 個を1つずつ見て、付けられるテストには付ける
- 直列のまま残すのは、プロセスの状態を触るテスト（`t.Setenv`・`os.Chdir`・`InstallHTTPS`・パッケージ変数のフックの差し替え・go-keyring のモック）だけ。残す理由が名前やコメントから分からないものには、理由のコメントを1行付ける

### sleep

- 固定の sleep 49 か所を1つずつ分類する
  - 状態が変わるのを待っているもの（poll の `2 * pollInterval`、ロックの待ち、goroutine が始まるのを待つものなど）は、条件が成り立つまで短い間隔で確かめる形か、チャネルでの合図に替える。待ち時間の上限はテストの失敗として出す
  - 時刻の精度のためのもの（秒の精度の時刻を進める `1100ms` など）や、「待たないこと」を確かめるものは残す
- 時計を差し替えられるところ（`now` のフック）は、sleep をやめて時計を進める

## D. ローカルのゴミ

### ~/.cache/go-build

原因: `-trimpath` なしでビルドすると、自分のパッケージのキャッシュのキーにパッケージのディレクトリの絶対パスが入る（Go 1.27 の `cmd/go/internal/work/exec.go` の `dir %s`）。worktree はそれぞれ場所が違うので、同じコードでも worktree ごとに全パッケージを別にキャッシュする。worktree の数（Claude とサブエージェントの worktree）× ビルドの種類（通常・-race・-race とカバレッジ・CGO_ENABLED=0）× 変更の回数が溜まり、Go は5日使わなかったものしか消さない。

- mise.toml に `[env] GOFLAGS = "-trimpath"` を入れ、worktree の間でキャッシュを共有する
- CI の各ジョブでも `GOFLAGS: -trimpath` を設定し、ローカルと同じビルドにする
- `make test` からカバレッジを外す（`-race` だけ）。カバレッジ付きは `make cover` に分ける。カバレッジ付きのビルドは別の種類としてキャッシュを増やすため
- Makefile に `clean-cache`（`go clean -cache -testcache`）を足し、CONTRIBUTING に「容量が気になったら実行する」と書く

### /tmp の残り

原因: テストを途中で止めると、`t.TempDir()` と、TestMain が作る `hedgehug-bin-*`・`devenv-bin-*`・`hedgehug-path-*`・`hh-server-test*` が残る。TestMain の後始末は正常に終わったときしか動かない。

- `internal/testenv` を作る。`testenv.Main(m)` は次を行ってから `m.Run()` を呼び、終わったら作ったディレクトリを消して終了する
  1. `$TMPDIR/hedgehug-test-<pid>` を作り、`TMPDIR` をそこに向ける。以後の `t.TempDir()` と `os.MkdirTemp("", …)` はすべてその下にできる
  2. 元の `$TMPDIR` にある `hedgehug-test-<pid>` のうち、その pid のプロセスがもう動いていないものを消す（`syscall.Kill(pid, 0)` が `ESRCH` を返すもの。`EPERM` は動いているとみなす）。前回途中で止めた実行の残りが、次の実行で消える。pid が別のプロセスに再利用されていれば消さずに残るが、次に空いたときに消える
- テストのあるすべてのパッケージの TestMain から `testenv.Main` を呼ぶ。TestMain のないパッケージには `main_test.go` を足す。すでに TestMain があるもの（`cmd/hedgehug`・`internal/server`）は、子プロセスとして動くときの分岐（エディタ役など）を先に済ませてから `testenv.Main` に入る
- テストの子プロセス（自分自身を子として起動するもの）は `TMPDIR` を受け継ぐ。子が `testenv.Main` に入るとその下に自分の分を作り、終わると消す。消すのは `hedgehug-test-<pid>` の形の名前で pid が動いていないものだけなので、親のディレクトリを消すことはない

### C: の容量（リポジトリの外）

WSL の仮想ディスク（ext4.vhdx）は書き込みで膨らみ、中のファイルを消しても縮まない。上の対策で書き込みは減るが、すでに膨らんだ分は戻らない。Windows 側で `wsl --manage <ディストロ名> --set-sparse true` を一度実行すると、消した分が C: に返るようになる。CONTRIBUTING の WSL の説明に書く。

## 検証

- ローカルでは `go build ./...` と `go vet ./...` だけを実行する。テストは CI で確かめる
- ローカルでテストを回すしかないときは、`TMPDIR` を作業領域に向けて1回だけ回す（`-count=1 -race ./...`、timeout は 600000）
- 完了の条件
  - CI の全ジョブが緑で、全体の待ちが縮んでいる（最初の PR の CI と main の CI の時間を PR の説明に載せる）
  - テストの件数が減っていない（Windows だけのテストを消した分を除く。消したものは PR の説明に挙げる）
  - `GOFLAGS=-trimpath` で、別の worktree のビルドがキャッシュに当たる（`go build -x` で compile が走らないこと）
  - `testenv` の後始末を `internal/testenv` のテストで確かめる: テストのバイナリを子として起動して `hedgehug-test-<子の pid>` を作らせ、SIGKILL で止める。そのあと掃除の関数を呼ぶと、そのディレクトリが消え、動いているプロセス（自分）のディレクトリは残る
