# フェーズ 1b: Store の読み書きと git コミット 設計

- 日付: 2026-10-30
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「実装フェーズ」1. Core とファイル形式、「同期とマージ」の「書き込みの流れ」「OS ごとの差」
- 前段: `docs/superpowers/specs/2026-10-30-phase1a-model-format-design.md`

## 位置づけ

フェーズ1を 1a〜1d に分けたうちの 1b。1a の codec の上に、ディスクへの読み書き、ロック、原子的な書き込み、1操作1コミットを載せる。

| 段階 | 内容 |
| --- | --- |
| 1a（完了） | 型・ファイル形式の codec・ULID・rank・パス・スキーマ検証 |
| 1b（本書） | Store のディスク読み書き、原子的な書き込み、ロック、go-git での1操作1コミット |
| 1c | インデックス、短縮 ID、クエリの基本、循環と Store 外参照の検出 |
| 1d | CLI コマンド群、devenv、`HEDGEHUG_HOME` |

### 1b で扱うもの

- `internal/gitrepo`: go-git による repo の作成・オープン・コミット
- `internal/store`: Store の作成・オープン、各ファイル型の読み出しと走査、`Update` による書き込み、ロック、原子的な書き込み、Windows 固有の対処
- `schema.ValidateWrite`: 書き込み時の検査をまとめた関数

### 1b で扱わないもの

- push キュー、fetch、merge、repack、merge driver の登録と `.gitattributes`: フェーズ2
- インデックス、短縮 ID、循環の検出: 1c
- CLI（`hh store init` など）: 1d
- アカウントの解決: フェーズ3。コミットの author は呼び出し側が渡す

## パッケージ構成

```
internal/gitrepo/
  repo.go          Repo、Init、Open
  commit.go        Commit（パス単位の add / 削除、trailer、author）
internal/store/
  store.go         Store、Create、Open、Meta、Schema
  read.go          1件の読み出しと走査
  tx.go            Tx、Update の流れ、日時の付与、変更の検出
  validate.go      書き込み時の検査の入力集め（親・祖先・変更前）
  atomic.go        一時ファイル＋rename、失敗時の巻き戻し
  lock.go          ロックの待ち合わせ（共通）
  lock_unix.go     flock
  lock_windows.go  LockFileEx
  fs_unix.go       読み込みと rename（素の os）
  fs_windows.go    FILE_SHARE_DELETE での読み込み、rename の再試行
internal/schema/
  validate.go      ValidateWrite を追加
```

依存の向きは `store → gitrepo`、`store → schema → model`。`gitrepo` は `model` にも `schema` にも依存しない。フェーズ2の `sync` は `gitrepo` と `store` の両方に依存する。

追加する依存ライブラリは `github.com/go-git/go-git/v5`（v5.19.3）と `golang.org/x/sys`。v6 は 2026-10-30 時点で beta なので使わない。

## gitrepo

```go
type Signature struct{ Name, Email string }

type Trailer struct{ Key, Value string }

type Commit struct {
	Paths    []string  // Store ルートからの相対パス（スラッシュ区切り）
	Subject  string
	Trailers []Trailer
	Author   Signature // committer も同じにする
	When     time.Time
}

func Init(dir string) (*Repo, error)
func Open(dir string) (*Repo, error)
func (r *Repo) Commit(c Commit) (plumbing.Hash, error)
```

- `Commit` は `Paths` だけを index に反映してからコミットする。
  - ディスクにあるパスは `AddWithOptions{Path, SkipStatus: true}` で足す。素の `Add` はコミットのたびに作業ツリー全体の status を取り、タスク数に比例して遅くなるため。
  - ディスクにないパスは index から直接外す。go-git は、削除したパスに対しては `SkipStatus` を付けても全体の status を取るため。
- メッセージは「件名、空行、trailer」の順に組み立てる。trailer は `Key: Value` を1行ずつ書く。
- `Paths` が空なら `ErrNothingToCommit` を返す。
- コミットは index 全体から作る。手で `git add` しておいた別の変更があると、それも同じコミットに入る。Store のファイルは Hedgehug を通して扱う前提（親設計）なので、1b ではこれを制約として受け入れる。
- trailer の解析、log、revert、fetch、push はフェーズ2で必要になったときに足す。

## Store

### 作成とオープン

```go
func Create(dir string, meta *model.StoreMeta, author gitrepo.Signature) (*Store, error)
func Open(dir string) (*Store, error)
func (s *Store) Meta() *model.StoreMeta
func (s *Store) Schema() *schema.Schema
```

- `Create` は、`dir` が存在しないか空のときだけ作る。それ以外ならエラー。
  1. `gitrepo.Init`（既定のブランチは `main`）
  2. `.hedgehug/store.yaml`（`meta`。`format` は `model.StoreFormat` にそろえる）、`schema.yaml`（`schema.Default()`）、`.gitignore`（`.hedgehug/local/` の1行）を書く
  3. 初回コミットを作る（件名 `hedgehug: init`、trailer `Hedgehug-Op: init`）
- `Open` は `store.yaml` と `schema.yaml` を読む。`store.yaml` の `format` が対応より新しいときのエラーは、1a の `DecodeStoreMeta` が返す。
- `Meta()` は `Open` 時点の値を返す。`Schema()` は最後に読んだスキーマを返す。`Update` はロックを取ったあとに `schema.yaml` を読み直して差し替え、検査にはそれを使う。ほかの `Store` や同期が書き換えたスキーマも、次の `Update` で反映される。読み直したスキーマをデコードできなければ、`Update` はエラーを返す。`Update` で `schema.yaml` を書いたら、成功したあとに差し替える。

### 読み出し

```go
func (s *Store) Task(id model.ID) (*model.Task, error)
func (s *Store) Tasks() iter.Seq2[*model.Task, error]
func (s *Store) Comments(task model.ID) iter.Seq2[*model.Comment, error]
func (s *Store) Worklogs(task model.ID) iter.Seq2[*model.Worklog, error]
func (s *Store) Sprint(id model.ID) (*model.Sprint, error)
func (s *Store) Sprints() iter.Seq2[*model.Sprint, error]
func (s *Store) Release(id model.ID) (*model.Release, error)
func (s *Store) Releases() iter.Seq2[*model.Release, error]
func (s *Store) Template(name string) (*model.Template, error)
func (s *Store) Templates() iter.Seq2[*model.Template, error]
```

- 読み出しはロックを取らない。書き込みは rename で置き換えるので、書きかけのファイルを読むことはない。
- 見つからないときは `ErrNotFound` を wrap して返す。
- 走査は `ParseTaskPath` などで解釈できるパスだけを返し、それ以外のファイルは読み飛ばす。走査の順序は決めない。
- デコードのエラーは、ファイルパスを付けて wrap する（`*model.DecodeError` は `errors.As` で取り出せる）。
- 削除済み（`deleted_at` あり）のものも返す。絞り込みは呼び出し側が行う。

### 書き込み `Update`

```go
type Op struct {
	Name   string            // add / update / rm / mv など。Hedgehug-Op に入る
	Task   model.ID          // 操作の主なタスク。なければゼロ値
	Author gitrepo.Signature
}

func (s *Store) Update(op Op, fn func(*Tx) error) error
```

`Tx` の API:

```go
func (tx *Tx) Task(id model.ID) (*model.Task, error) // 積んだ書き込みも反映して読む
func (tx *Tx) PutTask(t *model.Task) error
func (tx *Tx) RemoveTask(id model.ID) error          // タスクとそのコメント・作業時間のファイルを消す（purge 用）
func (tx *Tx) PutComment(task model.ID, c *model.Comment) error
func (tx *Tx) PutWorklog(task model.ID, w *model.Worklog) error
func (tx *Tx) PutSprint(sp *model.Sprint) error
func (tx *Tx) PutRelease(r *model.Release) error
func (tx *Tx) PutTemplate(t *model.Template) error
func (tx *Tx) PutSchema(sc *schema.Schema) error
```

- `Put*` はメモリ上に書き込み予定を積むだけで、ディスクには触れない。同じファイルに2回 `Put` したら後のものが勝つ。
- `Tx` は `fn` の中でだけ使える。`fn` を抜けたあとに呼ぶとエラー。

流れ:

1. `.hedgehug/local/lock` のロックを取る（「ロック」の節）。同じ `Store` の中では、ロックの前に `sync.Mutex` でも直列にする。ロックを取ったら `schema.yaml` を読み直す。
2. `fn(tx)` を実行する。エラーなら何も書かずに返す。
3. 書き込み時の検査をかける（「書き込み時の検査」の節）。違反があれば何も書かずに `*ValidationError` を返す。`PutSchema` のスキーマは `Check()` にかけ、問題があれば拒否する。
4. 日時を付けて、変更を検出する（「日時の付与と変更の検出」の節）。変更が1つもなければ、コミットせずに `nil` を返す。
5. 変更のあるファイルを原子的に書く（「原子的な書き込み」の節）。`RemoveTask` の分は消す。
6. `gitrepo.Commit` に変更したパスを渡してコミットする。
7. ロックを解放する。

5 か 6 で失敗したときは、書き終えたファイルを元のバイト列に戻し、新しく作ったファイルは消す。戻すのにも失敗したら、元のエラーと巻き戻しのエラーを `errors.Join` で返す。作業ツリーに残ったコミットされていない変更は、1d の `hh doctor` で知らせる。

### コミットメッセージ

- 件名は `hedgehug: <Op.Name> <タスク> <title>`。`<タスク>` はタスクの `key` があればそれ、なければ ULID をそのまま書く。`key` と `title` は、`Tx` を反映したあとのタスクから取る。`Op.Task` がゼロ値なら件名は `hedgehug: <Op.Name>` だけにする。
- trailer は `Hedgehug-Op: <Op.Name>` と、`Op.Task` があれば `Hedgehug-Task: <ULID>`。
- 親設計の例（`hedgehug: update 01JB リリース準備`）は短縮 ID を使っているが、短縮 ID の規則は 1c で決める。1b では ULID をそのまま書く。

### 日時の付与と変更の検出

- `Update` ごとに時刻を1つ取り、その操作で書くすべてのファイルに使う。時刻は秒に切り詰め、ローカルのオフセットで書く。時計は非公開のフィールドで差し替えられるようにする（テスト用）。
- `id` を持つファイル型（タスク・コメント・作業時間・スプリント・リリース）とテンプレートについて:
  1. `created` がゼロ値なら操作の時刻を入れる。
  2. `updated` を変更前の値のまま Encode して、ディスク上のバイト列と比べる。同じならその書き込みは捨てる。
  3. 違えば `updated` を操作の時刻にして Encode し直す。
- `schema.yaml` は日時を持たないので、Encode したバイト列をそのまま比べる。
- `created_by` は Store では埋めない（呼び出し側が渡す）。

### 書き込み時の検査

制約の規則は `schema` パッケージに置き、Store は検査に必要な入力を集めて呼ぶだけにする。

```go
// schema/validate.go
func ValidateWrite(old *model.Task, oldParentKind string, oldE *Effective, cur *model.Task, parentKind string, e *Effective) []Violation
```

- `ValidateTask`、`ValidatePlacement`、`ValidateTransition` をまとめてかける。`ValidateTransition` は、新規なら `from` を空にして（入る先のステータスの必須項目を見る）、既存ならステータスが変わったときだけかける。2つの検査が同じ違反（`unknown_kind` など）を返したら1つにまとめる。
- `old` に同じ検査をかけて出た違反は、結果から差し引く。違反を比べるときは `Code` と `Field` の組で見る。こうして、その書き込みで新しく生じた違反だけを返す。フェーズ2の同期のあとで違反を抱えたタスクが生じても、それと関係のない編集はできる。
- `old` は変更前の親と祖先の文脈（`oldParentKind` と `oldE`）で検査する。移動で新しく生じた配置の違反は差し引かない。
- `old` が nil（新規）なら、差し引くものはない。`oldParentKind` と `oldE` は使わない。
- スキーマにない props は、1a の方針どおり違反にしない。

Store の側（`store/validate.go`）では、`PutTask` で積んだタスクのそれぞれについて、`fn` を抜けたあとに次を行う。

- `old`: `Tx` を反映する前のディスク上のタスク。デコードできなければ nil（新規）として扱い、新しい内容をすべて検査する。壊れたファイルを書き込みで直せるようにするため。
- `parentKind`: 親タスクの `kind`。親は `Tx` を反映したあとの状態から引く（テンプレートを展開するとき、同じ操作で親と子を作るため）。親が見つからないときは、親を変える書き込み（新規も含む）なら `ErrNotFound` を wrap したエラーで拒否する。親を変えない書き込みなら拒否せず、空文字にする。親が purge されたタスクも編集できるようにするため。親がなければ空文字（`root` として扱われる）。
- `e`: 祖先の ID を近い順に集めて、`Schema().For(ancestors)` を呼ぶ。同じ操作で `PutSchema` したときは、新しいスキーマを使う。祖先は parent をたどってファイルを読む。見つからない祖先があれば、そこでたどるのをやめ、それまでに集めた祖先を使う。同じ ID に戻ってきたら、`ErrCycle` を返す（本格的な循環の検出は 1c）。
- `oldParentKind` と `oldE`: `old` について、`parentKind` と `e` と同じものを求める。ただし親と祖先は `Tx` を反映する前のディスクから引く。親や祖先が見つからなければ、そこでたどるのをやめる（親が見つからなければ `oldParentKind` は空文字）。
- 違反があれば `*ValidationError{Task model.ID; Violations []schema.Violation}` を返す。

スプリント・リリース・テンプレート・コメント・作業時間には、スキーマの検査をかけない（Encode できればよい）。

### 原子的な書き込み

- 書き込み先と同じディレクトリに一時ファイル（`.<名前>.tmp-<乱数>`）を作り、書いて fsync してから rename で置き換える。親ディレクトリがなければ作る。パーミッションはディレクトリが 0750、ファイルが 0600（`os.CreateTemp` の既定）。git が記録するモードは実行ビットだけなので、clone した側には影響しない。
- rename に失敗したら、一時ファイルは消す。
- Windows の rename は、`ERROR_ACCESS_DENIED` か `ERROR_SHARING_VIOLATION` のときだけ、50ms 間隔で最大10回まで再試行する。ほかのエラーはすぐ返す。
- Windows で Hedgehug がファイルを読むときは、`FILE_SHARE_DELETE` 付きで開く。自分の読み込みが rename を妨げないようにするため。
- `RemoveTask` で消したあとに空になったシャードのディレクトリは消さない（git はディレクトリを追跡しないため、残しても害がない）。

### ロック

- ロックファイルは `.hedgehug/local/lock`。なければ作る。CLI と GUI サーバーで共有する。
- Unix は `flock(LOCK_EX|LOCK_NB)`、Windows は `LockFileEx(LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY)` を使う。どちらもファイルハンドルごとのロックなので、同じプロセスの中で別々の `Store` を開いても互いに排他になる。
- 取れなければ 50ms 間隔で取り直し、10秒たっても取れなければ `ErrLocked` を返す。
- プロセスが落ちれば OS がロックを解放するので、古いロックは残らない。

### エラー

| エラー | 意味 |
| --- | --- |
| `ErrNotFound` | 指定したファイルがない（`errors.Is` で判定） |
| `ErrLocked` | 10秒以内にロックを取れなかった |
| `*ValidationError` | 書き込み時の検査で違反があった |
| `*SchemaError` | `PutSchema` したスキーマが `Check()` で問題を返した |
| `ErrCycle` | 書き込み時の検査で、parent をたどったら同じタスクに戻った |
| `gitrepo.ErrNothingToCommit` | 空のパスでコミットしようとした（Store からは呼ばない） |

## テスト

- **gitrepo**（一時ディレクトリの repo を使う）
  - 渡したパスだけがコミットに入る
  - 件名と trailer、author が正しく入る
  - 消したパスがツリーから消える
  - 空のパスでは `ErrNothingToCommit` を返す
- **store**
  - `Create` してから `Open` すると、`Meta()` と `Schema()` を読める。初回コミットが1つある
  - 1回の `Update` でコミットが1つでき、件名と trailer が正しい
  - 変更がなければコミットしない
  - `fn` がエラーを返したら、ファイルにもコミットにも何も残らない
  - コミットが失敗したら、ファイルが元に戻る（非公開の差し替え口で失敗させる）
  - `created` / `updated` の付き方: 新規、変更あり、変更なし
  - `Tx.Task` が積んだ書き込みを反映して読む
  - 同じ操作で親と子を作れる
  - `RemoveTask` でタスクとコメント・作業時間が消え、コミットされる
  - 走査が、解釈できないパスを読み飛ばす
- **書き込み時の検査**
  - `ValidateWrite` のテーブル駆動テスト: 新規、新しく生じた違反、もともとあった違反、移動、遷移、必須
  - Store 経由: 違反で拒否されて何も残らない。親がない。parent の循環
- **ロック**
  - 別々の `Store` から同時に `Update` すると、直列に処理される（コミット数と内容で確かめる）
  - ロックを持ったままにすると、待ちの上限で `ErrLocked` を返す（テストでは上限を短くする）
  - ロックを持ったまま kill した子プロセスのあとでも、ロックを取れる（helper process パターン）
- **Windows**
  - rename の再試行を、rename 関数を差し替えた単体テストで確かめる（再試行するエラー、しないエラー、上限）
  - CI はすでに ubuntu / macos / windows の matrix で動いているので、全テストが3つの OS で走る

## 後続への申し送り

- フェーズ2: push キュー（「書き込みの流れ」の 4）、repack（未圧縮オブジェクト 6,700 個で実行）、merge driver の登録と `.gitattributes` の `merge=hedgehug` の行は `sync` で扱う。
- フェーズ2: `gitrepo` に trailer の解析、log、revert を足す（`hh undo`、`hh restore --at`、履歴ビュー）。
- 1c: コミットの件名の `<タスク>` を、key がないときは短縮 ID にするか決める。
- 1d: `hh doctor` で、作業ツリーに残ったコミットされていない変更を知らせる。
- 親設計ドキュメント: パッケージ構成の表では `internal/sync` が commit を持つが、1b で `internal/gitrepo` に分けた。親設計の表を更新する。

## 完了条件

- `Create` した Store に、全ファイル型を `Update` で書いて読み戻せる。1回の `Update` がちょうど1つのコミットになり、件名と trailer が規則どおりになる。
- 書き込みの失敗と検査の違反で、ファイルにもコミットにも中途半端な状態が残らない。
- `make check` が ubuntu / macos / windows で通る。
