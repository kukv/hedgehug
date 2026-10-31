# フェーズ 2: 同期とマージ 設計

- 日付: 2026-10-31
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「実装フェーズ」2. 同期とマージ、「同期とマージ」の章、「データ形式」の「人が読める ID」「ワークスペース」、「メンションと通知」、「開発環境」
- 前段: `docs/superpowers/specs/2026-10-31-phase1-5-workspace-keys-design.md`（ほか 1a〜1d-2 の spec）

## 位置づけ

フェーズ2は1つの spec と1つのブランチで作る。親設計がフェーズ2に挙げたものはすべて入れる。

- fetch / merge / push、フィールド単位の3-wayマージ、コメントの和集合、不変条件の修復
- 上書きの記録と通知（`hh inbox`）、複数 remote、git の merge driver の登録、`hh undo`、`hh restore --at`
- メンションとウォッチャーの通知
- 同期での ID の衝突の振り直しと通知、`workspaces/` と `.hedgehug/numbers.yaml` のマージ、接頭辞の重複の案内（採番そのものはフェーズ1.5で作った）
- 開発環境: bare リポジトリへの同期、インプロセスの file transport、git を外した CI ジョブ
- 完了条件: 複数のクローンでランダムな同時編集を繰り返すプロパティテストで、全クローンが同じ状態に収束し、値が失われない

親設計に明記はないが、同期の中でしか作れないので、次もフェーズ2で作る。

- 「自分」の判定に使う `accounts[]` の host・login と `stores[].account` の読み込み。`created_by`・コメントの `author` をここで埋める（フェーズ1で「フェーズ3まで空」としていたもの）
- 2つ目のクローンを作る `hh store add <name> --remote <url>`

### 用語

フェーズ1.5の呼び方に合わせる。人が読める識別子（`WEB-42`）は **ID**（ファイルの `id:`）、内部の識別子は **ULID**（`ulid:`）、振り直す前の ID は `former_ids`、ワークスペースの接頭辞は `id_prefix`。

### フェーズ2で扱わないもの

親設計の「作る時期」でフェーズ3以降と決まっているものだけ。

- `url_rewrites`、`hh store init` で bare リポジトリを作ること、`hh store discover`: フェーズ3。フェーズ2の bare remote は devenv が作る
- トークンを使う認証（`accounts[].token`）: フェーズ3。フェーズ2の remote はローカルのパスだけを想定し、認証を使わない
- Store 間の移動での振り直し、履歴の書き換え（`purge --rewrite-history`）の検出: フェーズ3
- 接頭辞が重複した Store の `org-a:API-12` の表示: Store のマウント（フェーズ3）で作る。フェーズ2は重複の案内（inbox と doctor）まで
- メンション先に Store を読む権限がないときの警告: メンバー一覧（フェーズ3）が要る
- 自動化の通知: 自動化（フェーズ5）が inbox に同じ仕組みで足す
- GUI の定期同期（表示中 2 分、非表示中 10 分）と同期状態の表示: GUI（フェーズ4）が `internal/sync` を呼ぶ

## パッケージ構成

```
internal/gitrepo/
  remote.go    remote の設定、Fetch、Push、clone
  transport.go file スキームをインプロセスの transport/server に差し替える
  merge.go     MergeBase、2つのツリーの差分、パスのバイト列の読み出し、親を指定したコミット
  maintain.go  未圧縮のオブジェクトの数え上げと repack
internal/merge/
  merge.go     Merge: 種類ごとの3-wayマージ（同期・merge driver・undo で共有）
  task.go      タスクのフィールド単位のマージ
  text.go      行単位の diff3
  yaml.go      schema.yaml / automations.yaml / store.yaml / numbers.yaml のマージ
  repair.go    マージ後の不変条件の修復（ID の衝突と振り直しを含む）
internal/model/
  numbers.go   ファイルの集合から接頭辞ごとの最大番号を数える（store の採番と repair で共有）
internal/store/
  merge.go     Store.CommitMerge: ロックの中でマージの結果を書き、親を指定してコミットする
internal/sync/
  sync.go      Run: 1回の同期
  history.go   各側の時刻と author（Side.When と Side.Author）を履歴から求める。merge driver と共有
  state.go     .hedgehug/local/sync.json
  background.go バックグラウンド同期の起動と待ち合わせ
  spawn_unix.go spawn_windows.go
internal/inbox/
  inbox.go     通知の作成、保存、既読
internal/config/
  config.go    remotes、account、accounts、sync を読む
internal/index/
  refs.go      本文とコメントのメンションと #ID、バックリンク
cmd/hedgehug/
  sync.go inbox.go watch.go undo.go merge_driver.go store_add.go
tools/devenv/
  main.go      bare remote と共有 Store を足す
```

依存の向きは `cmd/hedgehug → sync → merge → model, schema`、`sync → store → gitrepo`、`sync → inbox`、`sync → index`。`merge` は git を知らない（バイト列と、各側の時刻と author、各タスクが origin にあるかどうかだけを受け取る）。

## 識別子と trailer

### 自分

- config.yaml の `accounts[]` から `host` と `login` を読む。アカウントの ID は `<host>/<login>`（例 `github.com/alice`）。
- Store の書き手は `stores[].account` が指すアカウント。指定がなく、アカウントが1つだけならそれを使う。どちらもなければ書き手は空で、`created_by` と `author` は空のまま、`Hedgehug-Author` を付けない。`hh inbox` は「accounts が設定されていない」と警告する。
- 「自分」は `accounts[]` のすべてのアカウント（親設計の「自分」の定義どおり）。
- コミットの author（名前とメール）は今と同じく git の設定から取る。

### trailer

既存の `Hedgehug-Op`・`Hedgehug-Task`・`Hedgehug-Workspace` に、次を足す。

| trailer | 付けるコミット | 値 |
| --- | --- | --- |
| `Hedgehug-Author` | 書き手が分かるすべてのコミット | 書き手のアカウント ID |
| `Hedgehug-Overwritten` | マージコミット | `<ファイルのパス> <フィールド> <負けた値の JSON> <負けた側の author>`。1つの負けた値につき1行 |
| `Hedgehug-Repaired` | マージコミット、修復のコミット | `<タスクかワークスペースの ULID> <修復の種類> <元の値の JSON> <その変更の author>` |
| `Hedgehug-Undo` | undo のコミット | 取り消したコミットのハッシュ |

`<フィールド>` はタスクなら `title`、`props.priority` のようにドットでつなぐ。値の JSON は1行にする。負けた側の author は、その側で merge-base 以降にそのファイルを最後に変えたコミットの `Hedgehug-Author`（なければコミットの author のメール）。修復の種類は「マージ後の修復」の表の名前（`parent_cycle`、`id_collision` など）。

`Hedgehug-Op` に `merge`、`repair`、`setup`、`undo`、`revert-overwritten` を足す。

### 件名

フェーズ1.5の形（`hedgehug: <操作> <ID> <タイトル>`、タスクのない操作は `hedgehug: <操作> <Detail>`）に合わせる。

- マージ: `hedgehug: merge <remote>`（例 `hedgehug: merge origin`）
- 修復: `hedgehug: repair <件数> issues`
- setup: `hedgehug: setup merge driver`
- undo: `hedgehug: undo <取り消したコミットの件名から hedgehug: を除いたもの>`
- inbox revert: `hedgehug: revert-overwritten <ID> <タイトル>`

`hh log` の件名の ID の置き換えは、`Hedgehug-Task` を持つコミットだけが対象なので、マージなどは件名のまま出る。

## マージ（internal/merge）

### 入口

```go
type Side struct {
    Data   []byte    // nil はファイルがない
    When   func(path []string) time.Time // そのキーを merge-base 以降に最後に変えたコミットの時刻（yaml 用）
    Author string    // その側で最後にファイルを変えたコミットの author
}

type Result struct {
    Data        []byte // nil はファイルを消す
    Overwritten []Overwritten
    Notices     []Notice // 削除の取り消しなど、inbox に出すもの
}

type Policy int // LWW（同期と merge driver）か OursWins（undo）

func Merge(path string, sc *schema.Schema, base []byte, ours, theirs Side, p Policy) (Result, error)
```

`When` は、各側の履歴を merge-base までたどり、各版の yaml を読んでキーごとに最後に変わったコミットを求める。この関数は `internal/sync` の `history.go` に置き、同期と merge driver で共有する。

- 種類はパスで決める: `tasks/` はタスク、`comments/` はコメント、`worklogs/` は作業時間、`sprints/`・`releases/`・`templates/`・`workspaces/` はそれぞれの形式、`schema.yaml`・`automations.yaml`・`.hedgehug/store.yaml` は yaml のキー単位、`.hedgehug/numbers.yaml` は接頭辞ごとの最大、ほかは「その他のファイル」。
- `sc` は、先に両側の schema.yaml をマージした結果。multi の props を見分けるのに使う。
- 結果は対称にする。`Merge(p, sc, b, o, t)` と `Merge(p, sc, b, t, o)` は同じバイト列と同じ上書きの集合を返す。
- 結果のファイルは既存の `model` のエンコーダで書く。土台にする yaml.Node は、両側の `Author`、次にファイルのバイト列の辞書順で大きい側に決める（ours と決めると、両側でコメントやキーの順が違うときに対称性が崩れるため）。土台に対して変わったキーだけを置き換えるので、コメントと未知のキーは残る。
- どの種類でも、両側のバイト列（yaml のファイルは値）が同じなら、それをそのまま採り、上書きとしない。
- `Merge` は1つのファイルだけを見る。ファイルをまたぐ不変条件（ID の衝突、親とワークスペースの一致など）は修復（repair.go）で扱う。

### 勝ち負けの決め方（LWW）

両側が同じ値を別の値に変えたとき:

1. タスク・コメント・作業時間・スプリント・リリース・テンプレート・ワークスペースは、ファイルの `updated` が新しい側（ワークスペースの `id_prefix` だけは `prefix_set` が新しい側）。yaml のファイルは `When(path)` が新しい側。
2. 同じなら `Author` の辞書順で大きい側。
3. それも同じなら、値をエンコードしたバイト列の辞書順で大きい側。

負けた値は `Overwritten` に入れる。両側が同じ値に変えたときは上書きとしない。

### タスク

| フィールド | 片側だけ変更 | 両側が変更 |
| --- | --- | --- |
| title / status / kind / parent / rank / sprint / workspace / created / created_by | 変更側 | LWW |
| id | 変更側 | LWW。負けた側の ID は `former_ids` に足す（番号を再利用しないため） |
| props の各キー、`gh` の各キー、未知のキー | 変更側 | キーごとに LWW。multi の値（multi_enum、multi の task_ref）は集合マージ |
| assignees / watchers / labels / repos / blocked_by / releases / links / former_ids | 変更側の差分を適用 | 両側の追加と削除を両方適用（集合マージ） |
| 本文 | 変更側 | 行単位の diff3。重なった箇所は両方を残して区切り線を入れる |
| updated | 変更側 | 大きい方 |
| deleted_at | 変更側 | 下記 |

- `ulid` は変わらない（パスと同じ）。
- 集合マージの順序: base の順に残った要素、ours で足された要素、theirs で足された要素のうち ours にないもの、の順に並べてから、theirs と ours を入れ替えても同じになるように、足された要素どうしは値の辞書順にする。links は `ref` で要素を見分ける。
- `former_ids` から要素を消すことはない（`edit` も `set` も変えられない）ので、集合マージは和集合になる。
- 本文の重なり: `updated`（同じなら author、値）で古い側、区切り線の行 `-----`、新しい側の順に並べる。重なりは上書きとしない（値は失われない）。
- 削除と編集の競合: 片側が `deleted_at` を付け、もう片側が `deleted_at` 以外のフィールドを変えたら、`deleted_at` を外して編集を残す。削除した側の author に `Notice{revived}` を出す。両側が `deleted_at` を変えたときは LWW。
- ファイルの有無: 片側がファイルを消し（purge）、もう片側が変えていなければ消す。もう片側が変えていれば、変えた側のファイルを残し、消した側の author に `Notice{revived}` を出す。両側が新しく作ったファイル（ULID なので同じパスにはならないが、手で作った場合）は、base を空として扱う。
- 片側のファイルが読めない（壊れている）ときは「その他のファイル」の規則に落とす。

### コメント・作業時間の記録

- 片側だけにあるファイル（新規）はそのまま取り込む。
- 両側で同じファイルを変えたら、ファイル全体を1つの値として `updated` で LWW にする（本文も含む）。負けた側のファイル全体を上書きとして記録する（フィールドは `*`）。

### スプリント・リリース・テンプレート・ワークスペース

フィールドごとに LWW。本文はタスクと同じ diff3。ファイルの有無はタスクと同じ規則（片側が消し、もう片側が変えていれば残して `Notice{revived}`）。

ワークスペースの `id_prefix` と `prefix_set` は1組として扱い、`prefix_set` が新しい側の組を採る（`prefix_set` が同じなら LWW の 2・3 段目）。`name` などほかのフィールドは `updated` で決める。

### schema.yaml / automations.yaml / store.yaml

- mapping はキーごとに再帰的にマージする。
- 要素を見分けられる列は、要素ごとにマージする。見分けるキーは、automations の `rules` が `id`、`scoped` が `where`、schema の `statuses`・`kinds`・`categories` が `name`。props と `workflow` は mapping なのでキーで見分ける。片側だけで足された要素は残し、片側だけで消された要素は消す。並びは集合マージと同じ規則で決める。
- それ以外の値（スカラーと、見分けられない列）は LWW。時刻は `When(path)` で、その側で merge-base 以降にそのキーの値を最後に変えたコミットの時刻（親設計で未決だった点の決定。ファイルの形式は変えない）。
- 片側が要素を消し、もう片側がその要素の中を変えたら、変えた側を残して `Notice{revived}` を出す。

### numbers.yaml

接頭辞ごとに、base・ours・theirs のうち大きい値を採る（LWW ではない）。番号を再利用しないための記録なので、値が小さくなることはない。片側にしかない接頭辞はそのまま残す。上書きとしない。コメントは yaml のキー単位のマージと同じく土台の側を残す。

### その他のファイル

両側で変わっていれば、各側で最後にそのファイルを変えたコミットの時刻が新しい側を採り、負けた側を上書き（フィールドは `*`、値はファイルの SHA-1）として記録する。`.gitattributes` もこの規則で扱う。

### 本文の diff3

- 行単位。base と各側の差分は Myers の diff で取る（go-git が依存している `github.com/sergi/go-diff` の `DiffLinesToChars` を使う）。
- 片側だけが変えた範囲はその側を採る。両側が同じに変えた範囲は1つにする。両側が別々に変えて重なった範囲は、前記の順で両方を並べ、間に `-----` の行を入れる。
- 末尾の改行の有無も行として扱う。

### マージ後の修復（repair.go）

```go
type Snapshot interface { ... } // 修復の対象: マージした後のファイルの集合（HEAD のファイルにマージの結果を重ねたもの）

type RepairInput struct {
    Files    Snapshot
    Base     Snapshot                 // merge-base のファイル。修復のコミットでは nil
    OnOrigin func(ulid ULID, id string) bool // origin の main のツリーで、そのタスクがその ID を持つか
    Author   func(path string) string // その値を最後に変えた側の author
    Now      time.Time
}

func Repair(in RepairInput) ([]Change, []Repaired, error)
```

ファイルの集合全体を見て、次の順に直す。直した内容はマージコミット（または修復のコミット）に含め、`Hedgehug-Repaired` に残す。

| 種類 | 不変条件 | 修復 |
| --- | --- | --- |
| `workspace_revived` | タスクが指すワークスペースが存在する | 片側がワークスペースを消し（`workspace rm`）、もう片側がそこにタスクを足した場合など。base にそのワークスペースのファイルがあれば戻し、消した側の author に通知する。base にもなければ（手で作ったタスクなど）直さず、doctor の `no_workspace` に任せる |
| `id_prefix` | ワークスペースの `id_prefix` の形式が正しい | 手で編集したファイルから入る。base の値が正しければ base の組（`id_prefix` と `prefix_set`）に戻し、変えた側の author に通知する。base の値も正しくなければ直さない（採番が「`hh workspace prefix` で直す」と案内して拒否する） |
| `parent_cycle` | 親の循環がない | 循環に入ったタスクのうち、今回のマージで parent が変わり、`updated` が最も新しい（同じなら author、ULID）タスクの parent を base の値に戻す。base がない（修復のコミット）か、戻しても循環するなら、ルートへ移す（rank はルートの末尾）。循環がなくなるまで繰り返す。戻されたタスクの author に通知する |
| `parent_deleted` | 削除されていないタスクの親は削除されていない（`deleted_at` なし、かつ存在する） | 子をルートに移し（parent を null、rank はルートの末尾）、子の author に通知する |
| `workspace_mismatch` | 子は親と同じワークスペースに属する | 片側が親を `mv --workspace` し、もう片側が元のワークスペースで子を足した場合など。子（とその配下）を親のワークスペースへ移す。ID は次の行の決まりで振り直る。子の author に通知する |
| `id_collision` | 2つのタスクが同じ `id` を持たない | 同じ `id` のタスクの組ごとに、残す側を1つ決め、ほかを振り直す。残すのは、origin の main のツリーでその ID を持つタスク（先に push した側）。どれもそうでない、または2つ以上がそう（ミラーとのマージや、手でマージした場合）なら、ULID の小さい側。振り直した側の古い ID は `former_ids` に残る。振り直したタスクの author に通知する（「WEB-43 (ULID) を WEB-45 に振り直した」）。本文やコメントの `#WEB-43` は書き換えない（通知でそう伝える） |
| `id_renumbered` | ID の接頭辞が所属するワークスペースの接頭辞と一致する | フェーズ1.5の「書き込みのときに振る」決まりを、マージコミットを書き込みとみなして当てる。ID がない、または接頭辞が違うタスクに番号を振り、古い ID を `former_ids` の末尾に足す。所属するワークスペースの `id_prefix` の形式が違うタスクは振らない。通知はしない（`id_collision` と `workspace_mismatch` から来たものは、その行で通知する） |
| `prefix_duplicate` | 同じ Store のワークスペース同士で `id_prefix` が重複しない | データは変えない（接頭辞は必須なので外せない。番号は接頭辞ごとに数えるので ID は重複しない）。`prefix_set` が後の側（同じなら ULID の大きい側）のワークスペースについて、それを最後に変えた author に「`hh workspace prefix <ws> <新しい接頭辞>` で変える」と案内する |
| `name_duplicate` | 同じ Store のワークスペース同士で `name` が重複しない（大文字小文字を区別しない） | データは変えない。`created` が後の側（同じなら ULID の大きい側）について、`hh workspace rename` で変えるよう案内する |
| `rank` の重複 | — | データは変えない。表示は既存どおり rank、ULID の順。`mv` で前後の rank が同じなら、その兄弟の rank を `NBetween` で振り直してから置く |
| 依存の循環、配置違反、ワークフロー違反 | — | 直さない。既存の検査（`is:invalid`、`doctor`、`schema check`）で警告する |

- 番号を振るとき（`id_collision`、`id_renumbered`）の最大は、修復の対象のファイルの集合で数える。フェーズ1.5と同じく、全タスク（削除済みを含む）の `id` と `former_ids`、読めないタスクファイルの中の ID の形の語、`numbers.yaml`（マージした後のもの）、同じ修復の中で振った番号から取る。HEAD のインデックス（`IDNumbers`）はマージの結果を映していないので使わない。数える処理は `store/ids.go` から `model.CountIDNumbers` に移し、store の採番と修復で共有する。1つの組の中、組どうしは ULID の順に振る。
- 修復で変えたタスクとワークスペースの `updated` は修復の時刻にする。`id_renumbered` と `id_collision` で ID だけを変えたタスクの `updated` も同じ（振り直しは他のクローンにも届ける必要があり、`updated` を進めないと LWW で古い ID に戻されうるため）。
- 修復は決定的にする。同じファイルの集合と同じ `OnOrigin` からは同じ結果を出す。
- 修復のコミットでは `Base` がない。`parent_cycle` は base に戻さずルートへ移す。`workspace_revived` と `id_prefix` は戻す先がないので直さない（doctor に任せる）。

### Store 間の接頭辞の重複

同期で新しいワークスペースや接頭辞の変更が届いたら、config に登録された他の Store のワークスペースと照合する（フェーズ1.5の書き込み時の照合と同じ関数）。重複していて、この Store の側の `prefix_set` が後なら、そのワークスペースを最後に変えた author に「`hh workspace prefix` で変える」と案内する（inbox の `prefix_duplicate`、Store 名を添える）。この Store の側が先なら何もしない（後の側の Store が同期したときに案内される）。開けない Store は照合を飛ばす。`org-a:API-12` の表示はフェーズ3。

## gitrepo の追加

- `SetRemotes(urls []string)`: `.git/config` の remote を、1つ目を `origin`、2つ目以降を `mirror1`、`mirror2`… として書く。config.yaml にない remote は消す。
- `Fetch(remote string) (plumbing.Hash, error)`: `refs/heads/main` を `refs/remotes/<remote>/main` に取る。remote が空ならゼロハッシュ。
- `Push(remote string) error`: `main` を push する。fast-forward でなければ `ErrRejected`。
- `Clone(url, dir string) (*Repo, error)`。
- `MergeBase(a, b plumbing.Hash)`、`IsAncestor(a, b)`、`DiffTrees(a, b) ([]Change, error)`（パスと変更の種類）、`ReadAt(commit, path) ([]byte, error)`、`LastChange(from, base plumbing.Hash, path string) (LogEntry, bool)`（その側で base 以降にパスを最後に変えたコミット）。
- `Commit` に `Parents []plumbing.Hash` を足す（空なら今と同じ HEAD）。
- `FastForward(h)`: HEAD を h に進め、作業ツリーと index を合わせる。
- `LooseObjects() (int, error)`、`Repack() error`。
- `transport.go`: パッケージの初期化で `client.InstallProtocol("file", server.NewClient(loader))` を呼ぶ。loader は endpoint のパスを、そのまま bare リポジトリ（`filesystem.NewStorage`）として開く。Windows のドライブ文字（`C:\...`、`/C:/...`）も受け付ける。存在しないパスは `transport.ErrRepositoryNotFound`。

## store の追加

- `Store.CommitMerge(parents []plumbing.Hash, files map[string][]byte, op Op) error`: Store のロックの中で、マージと修復の結果のファイル（nil は削除）を既存の原子的な書き込み（一時ファイルと rename）で書き、`gitrepo.Commit` の手順で index に載せ、親を指定してコミットする。書き込み時の検査（`tx.validate`）はしない（依存の循環などはマージ後も警告に留めるため）。採番（`assignIDs`）もしない（振り直しは修復が済ませている）。ロックを取った時点の HEAD が、マージの計算に使った HEAD と違えば `ErrHeadMoved` を返し、同期はロックを外して計算し直す。
- `Tx.validateWorkspaces` の重複の検査を、操作で変えたフィールドに限る。同期で生じた重複（`prefix_duplicate`、`name_duplicate`）があるワークスペースでも、重複していないほうのフィールドの変更（重複した接頭辞のワークスペースの `rename` など）は通す。案内どおりに `workspace prefix` や `workspace rename` で直せるようにするため。
- undo と `restore --at` が使う `Tx.PutTask` は既存のまま。ID の扱いは「undo と restore --at」に書く。

## 同期（internal/sync）

### remote

config.yaml の `stores[].remotes`（URL の列）を正とする。同期の初めに `SetRemotes` で `.git/config` に反映する。採番の判定（ID の衝突でどちらを残すか）は `origin` で行い、2つ目以降はミラーとして扱う。

### 1回の同期（Run）

1. 全 remote から fetch する（Store のロックの外で行う）。
2. 作業ツリーの未コミットの変更（`gitrepo.Status`）が、これからマージで書くパスと重なるなら、何も変えずに `ErrDirty` で止め、`hh doctor` を案内する。
3. `.gitattributes` に `tasks/**/*.md merge=hedgehug`、`comments/**/*.md merge=hedgehug`、`worklogs/**/*.md merge=hedgehug`、`workspaces/*.md merge=hedgehug`、`*.yaml merge=hedgehug` がなければ足すコミット（`Hedgehug-Op: setup`、`Store.Update` で書く）を作る。merge driver を登録する（後述）。
4. remote ごとに（`origin`、ミラーの順）:
   - remote の main が HEAD の祖先なら何もしない。
   - HEAD が remote の main の祖先なら、Store のロックの中で `FastForward` する。
   - それ以外は merge-base を求め、`DiffTrees(base, HEAD)` と `DiffTrees(base, remote)` の両方に出たパスを `merge.Merge` にかける（片側だけのパスはその側を採る）。schema.yaml を最初にマージし、その結果を `sc` として残りに渡す。HEAD のファイルにマージの結果を重ねた集合に `merge.Repair` をかける（`OnOrigin` は fetch した `origin` の main のツリーを読む）。`Store.CommitMerge` で、親を `[HEAD, remote]` にしたマージコミット（`Hedgehug-Op: merge`、`Hedgehug-Overwritten`、`Hedgehug-Repaired`）を作る。`ErrHeadMoved` なら、この remote のマージを計算し直す。
5. 最後に HEAD のファイルの集合全体に `merge.Repair`（`Base` なし）をかける。merge driver による手でのマージや、手で編集したファイルが持ち込んだ違反を直すため。直すものがあれば修復のコミット（`Hedgehug-Op: repair`）を作る。4 でマージした場合はそこで修復が済んでいるので、ここで直すものは出ない。インデックスを使って候補を絞る（ID の重複、ワークスペースの不一致、親の循環と削除済みの親は index の問題として持っている）。
6. `origin` に push する。push は Store のロックの外で行う（ローカルの書き込みを、通信の間止めないため。捨てるコミットはないので、push の間に積まれた書き込みは次の push に乗るだけ）。`.hedgehug/local/sync.lock` が同期どうしを直列にする。拒否されたら 1 に戻る。待ち時間は 0.5 秒から倍にしていき、6 回まで。`origin` が通ったらミラーへ push する。ミラーで拒否されたら、ミラーも次の周でマージの対象になる（周の上限は同じ）。
7. inbox の通知を作る（後述）。
8. 未圧縮のオブジェクトが 6,700 個を超えていたら repack する。
9. `sync.json` に結果を書く。index は次に開くときに HEAD の差分で更新される。

remote のない Store は何もしない。Run の結果は、マージの数、上書き、修復（振り直した ID を含む）、警告（`is:invalid` に入ったタスク）、push した remote、エラーを返す。

### ID の衝突が起きる流れ

1. クローン A と B が、どちらも未 push の状態でそれぞれ `WEB-43` を振る（採番は各クローンの Store の中で最大 + 1）。
2. B が先に同期し、origin に `WEB-43`（B のタスク）が入る。
3. A が同期すると、マージした集合で `WEB-43` が2つになる。origin の main のツリーで `WEB-43` を持つのは B のタスクなので、A のタスクを振り直す（`WEB-45` など）。古い `WEB-43` は A のタスクの `former_ids` に入る。
4. A の inbox に「WEB-43 (A のタスクの ULID) を WEB-45 に振り直した」が届く。`hh show WEB-43` は `id` を `former_ids` より先に引くので B のタスクを出す。
5. A が push すると、B は fast-forward で振り直しを受け取る。B の inbox には、A のタスクの author が B でなければ何も届かない。

push の拒否と再試行の間に別の衝突が入っても、次の周のマージの修復で同じく振り直す。

### merge driver

- 登録: `.git/config` に次を書く。パスは `os.Executable()` の絶対パス。

  ```
  [merge "hedgehug"]
      name = Hedgehug field merge
      driver = "<hedgehug の絶対パス>" merge-driver %O %A %B %P
  ```

- 登録する時: `hh store init`、`hh store add`、同期の 3。
- `hedgehug merge-driver <base> <ours> <theirs> <path>`: リポジトリの作業ツリーから HEAD（ours）と `MERGE_HEAD`（theirs）を読み、`LastChange` で各側の時刻と author を引いて `merge.Merge` を呼ぶ。両側の schema.yaml もこの場でマージして `sc` にする。結果を `<ours>` に書いて 0 で終わる。`MERGE_HEAD` がない場合（rebase、cherry-pick など）は各側の履歴を引けないので、時刻はファイルの `updated`、yaml のファイルは時刻なしとして、author は空で比べる（LWW の 3 段目の値のバイト列で決まる）。git が作るマージコミットには trailer を書けないので、上書きと通知は `.hedgehug/local/overwritten/<ULID>.json` に書き、次の同期か `hh inbox` で inbox に取り込む。
- merge driver は1ファイルずつ呼ばれるので、ファイルをまたぐ修復（ID の衝突など）はしない。次の `hh sync` の 5 で直す。
- `hh doctor` は、`.gitattributes` に `merge=hedgehug` があるのに driver が登録されていない、または登録されたパスに実行ファイルがない場合に警告する（直し方: `hh sync` を一度実行する）。

### 状態（sync.json）

`.hedgehug/local/sync.json`:

```json
{"pending_since": "...", "last_attempt": "...", "last_success": "...", "last_error": "...", "error_reported": false}
```

- 書き込みのコマンドはコミットの後に `pending_since` を（空なら）今にする。同期が成功し、その時点までの書き込みを push したら空にする。
- 書き込みはファイルのロック付きで行う（Store のロックとは別の `.hedgehug/local/state.lock`）。

### バックグラウンド同期

- **起動**: 次のどちらかで `hedgehug sync --background --store <name>` をデタッチして起動する。`.hedgehug/local/bg.lock` を取れなかったら（待っているプロセスがいれば）起動しない。
  - 書き込みのコマンドがコミットした後。
  - どのコマンドでも、開始時に `last_success` から `sync.interval`（既定 2 分）以上たっていて、remote がある Store。`merge-driver` と `sync` 自身は除く。
- **子プロセス**: `bg.lock` を持ったまま、`pending_since + sync.push_interval`（既定 30 秒）まで待つ（`pending_since` が空なら待たない）。`.hedgehug/local/sync.lock` を取って `Run` する。終わった時点で `pending_since` があれば、また待つ。なければ終わる。
- **前面の `hh sync`**: `sync.lock` を取って（バックグラウンドが同期中なら終わるまで待って）すぐに `Run` する。
- **デタッチ**: Unix は `SysProcAttr{Setsid: true}` と標準入出力を `/dev/null` にして起動。Windows は `DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP`。子プロセスの stderr は `.hedgehug/local/sync.log` に書く（直近 1 MiB まで）。
- **失敗の知らせ**: コマンドの開始時、`last_error` があり `error_reported` が false なら、stderr に `hedgehug: background sync of <store> failed: <error> (run hedgehug sync)` を1行出し、`error_reported` を true にする。
- **止める**: 環境変数 `HEDGEHUG_AUTO_SYNC=0` でバックグラウンドの起動をしない。テスト、merge driver の中で使う。
- 設定: config.yaml の `sync.interval` と `sync.push_interval`（Go の `time.ParseDuration` の形）。

## inbox（internal/inbox）

### 作るもの

同期の 7 で、同期前の HEAD から新しい HEAD までに届いたコミット（自分の `Hedgehug-Author` のものを除く）と、そこで作ったマージコミット・修復のコミットを見て作る。

| 種類 | 条件 |
| --- | --- |
| mention | タスクの本文、またはコメントで、新しく足された `@login` / `@host/login` が自分のアカウントに当たる。`@login` は host を問わず login で、`@host/login` は完全一致で比べる。コードブロックとインラインコードの中は除く |
| assigned | assignees に自分のアカウントが足された |
| watch | watchers に自分がいるタスクで、status の変更、コメントの追加、assignees の変更があった |
| overwritten | `Hedgehug-Overwritten` の負けた側の author が自分 |
| revived | `Notice{revived}` の対象（消した側）が自分 |
| repaired | `Hedgehug-Repaired` の対象が自分。修復の種類（`parent_cycle`、`parent_deleted`、`workspace_mismatch`、`workspace_revived`、`id_prefix`）を内容に持つ |
| renumbered | `Hedgehug-Repaired` の `id_collision` の対象が自分。古い ID と新しい ID を内容に持つ |
| prefix_duplicate | `Hedgehug-Repaired` の `prefix_duplicate`・`name_duplicate` の対象が自分。Store 間の重複（「Store 間の接頭辞の重複」）も同じ種類で、相手の Store 名を持つ |

修復と通知の trailer は、同期で自分が作ったマージコミットや修復のコミットにも付く。これらは自分の `Hedgehug-Author` を持つが、対象の author が自分なら通知する（自分の値が負けた・振り直されたことを知らせるため）。`.hedgehug/local/overwritten/` の記録（merge driver）も取り込んで消す。

### 保存

- `.hedgehug/local/inbox.jsonl` に1行1件で追記する。各件は ULID、種類、タスクの ULID と今の ID（表示のときに今の ID に引き直す）、コミット、author、時刻、内容（上書きならフィールドと値）を持つ。
- 既読は `.hedgehug/local/inbox-read.json`（ULID の集合）。どちらもマシンごと。
- 同じ件を二度作らない。上書きは（ファイル、フィールド、負けた値、負けた側）で、ほかはコミットと種類とタスクで重複を除く。2つのクローンが同じ組をそれぞれマージして、同じ trailer を持つマージコミットが2つ届く場合があるため。`prefix_duplicate` は（ワークスペース、相手、種類）で、解消するまで一度だけ出す。

### コマンド

- `hh inbox [--all] [--json]`: 全 Store の未読（`--all` で既読も）を新しい順に出す。各件は完全な ULID で示し、タスクは「ID (ULID) タイトル」で出す（フェーズ1.5の表示の原則どおり。短縮 ULID は使わない）。
- `hh inbox read <ULID>... | --all`: 既読にする。完全な ULID だけを受け付ける。
- `hh inbox revert <ULID>`: overwritten の件で、自分の値をそのフィールドに書き戻す（ふつうの `set` と同じ書き込み、`Hedgehug-Op: revert-overwritten`）。書き戻したら既読にする。

## watchers とバックリンク

- 書き込み時に watchers へ自動で足す: add の書き手、set や assign で足された assignees、comment の書き手、本文とコメントで新しくメンションされたアカウント（`@login` は書き手の Store のアカウントの host を補う）。
- `hh watch <id>` / `hh unwatch <id>`: 自分（Store の書き手）を足す・外す。
- index: タスクの本文とコメントから `@...` と `#<ID>` を拾い、`Entry` にメンションと参照を持つ。`#<ID>` は今の ID と古い ID の両方で引く（`index.Resolve` と同じ順）。`Backlinks(ulid)` で参照元を返す。`hh show` に `Referenced by` を「ID (ULID) タイトル」の形で出す（`--json` は `referenced_by`、要素は `{"ulid", "id", "title"}`）。

## undo と restore --at

### `hh undo [--store <name>]`

- HEAD から first-parent をたどり、`Hedgehug-Author` が自分で、`Hedgehug-Op` が merge / repair / setup / undo 以外のコミットのうち、まだ `Hedgehug-Undo` で取り消されていない最新のものを探す。
- そのコミットが変えた各パスに `merge.Merge(path, sc, base = そのコミットの版, ours = HEAD の版, theirs = その親の版, merge.OursWins)` を当てる。`OursWins` は、両側が変えたフィールドで LWW の代わりに ours を採る方針（集合マージと diff3 の重なりも ours を優先する）。これで、そのコミットの後に他の人が変えたフィールドは HEAD の値が残り、自分だけが変えたフィールドは親の値に戻る。残したフィールドは結果に出す。
- `id` と `former_ids` は戻さず、HEAD の値を残す。戻すと、振った番号が `id` からも `former_ids` からも消え、番号が再利用されうるため。`workspace` が戻って接頭辞が合わなくなったタスクは、`Store.Update` の採番の決まりで新しい番号を振られる（`mv --workspace` を undo すると、元のワークスペースの新しい番号になる）。
- `workspace` が戻るタスクの、今の配下（ロックの中でインデックスを `Refresh` して集める。取り消すコミットの後に足された子を含む）も、同じワークスペースへ移す。`mv --workspace` と同じく、親と子でワークスペースが違う状態を作らないため。
- 書き込みは `Store.Update` で行い、書き込み時の検査を通す。検査で拒否された場合（取り消すと親の循環になるなど）は何も変えずにエラーを返す（終了コード 1）。変わったタスクの `updated` は今にする。
- コミットは `Hedgehug-Op: undo`、`Hedgehug-Undo: <hash>`。取り消すものがなければ `Nothing to undo` で 0。

### `hh restore <id> --at <時点>`

- 時点は、日時（RFC 3339）、日付（その日の終わり、ローカル時刻）、コミットハッシュ（短縮可）を受け付ける。
- HEAD から first-parent をたどり、その時点以前の最新のコミットでのタスクファイルを読む。そこにタスクがなければエラー（終了コード 1）。
- その内容で書き戻し、`updated` を今にする。その時点で削除されていなければ `deleted_at` も外れる。コメントと作業時間は戻さない。`Hedgehug-Op: restore`。
- `id`・`former_ids`・`workspace` は戻さず、今の値を残す。`id` と `former_ids` は undo と同じ理由。`workspace` は配下と一緒に動かす必要があり、それは `mv --workspace` の役目なので、戻す時点と今でワークスペースが違えば、そう表示して `mv --workspace` を案内する（ほかのフィールドは戻す）。
- 書き込み時の検査を通す。親が今は存在しない・削除済み・循環になるなどで拒否されたら、何も変えずにエラーを返す。
- 既存の `hh restore <id>`（削除の取り消し）はそのまま。

## コマンドと設定の変更

| コマンド | 内容 |
| --- | --- |
| `hh sync [--store <name>]` | すぐに同期して結果を出す。振り直した ID は「WEB-43 (ULID) → WEB-45」の形で1行ずつ出す。`--json` 対応 |
| `hh sync --status` | 各 Store の `sync.json` と、origin より先にある未 push のコミット数を出す |
| `hh store init <name> --prefix <接頭辞> --remote <url>...` | フェーズ1.5の初期化（`--prefix` 必須）に加え、remotes を config.yaml に記録し、merge driver を登録し、初回の push をする。remote は空の bare リポジトリを前提にする。空でなく push が拒否されたら、`hh store add` を案内してエラーにする |
| `hh store add <name> --remote <url>... [--path <dir>]` | origin を clone し、config.yaml に登録し、merge driver を登録する。path の既定は `store init` と同じ。clone した Store のワークスペースを登録済みの他の Store と照合し、重複があれば警告する（どちらの `prefix_set` が後か、後の側で `hh workspace prefix` を使うよう案内する）。登録はする（重複は ID の重複を生まず、Store 名付きの表示はフェーズ3で作る） |
| `hh inbox` / `inbox read` / `inbox revert` | 前記 |
| `hh watch` / `hh unwatch` | 前記 |
| `hh undo` | 前記 |
| `hh restore <id> --at <時点>` | 前記 |
| `hedgehug merge-driver` / `hedgehug sync --background` | 隠しコマンド |

config.yaml で新しく読むもの（Save は今と同じく、知らないキーとコメントを残す）:

```yaml
accounts:
  - name: personal
    host: github.com
    login: alice
stores:
  - name: org-a
    path: ~/.local/share/hedgehug/stores/org-a
    remotes: [/home/me/hh-dev/remotes/org-a/hh-shared.git]
    account: personal
sync:
  interval: 2m
  push_interval: 30s
```

## doctor の追加

- merge driver の登録（前記）。
- `.hedgehug/local/overwritten/` に取り込まれていない記録が残っている（`hh inbox` で取り込める）。
- フェーズ1.5の `duplicate_id` の直し方に「`hh sync` で、push 済みの側を残して振り直す」を足す。ワークスペースの接頭辞と名前の重複の直し方は今のまま（`hh workspace prefix` / `rename`）。

## 開発環境

### devenv

- `init` で `remotes/kukv/hh-tasks.git`（alice の個人 Store の remote）と `remotes/org-a/hh-shared.git` を bare で作る（go-git の `PlainInit(bare)`）。
- alice と bob の config.yaml に `accounts`（`github.com/alice`、`github.com/bob`）を書く。
- alice の個人 Store は今と同じ（接頭辞 `DEV`）で、`remotes/kukv/hh-tasks.git` を remote にする。bob の個人 Store は今と同じくローカルだけ。
- 共有 Store `org-a` を、alice が `store init org-a --prefix ORG --remote <hh-shared.git>` で作り、bob が `store add org-a --remote <hh-shared.git>` で clone する。`ORG` は各自の個人 Store の `DEV` と重ならない。
- `env` は `HEDGEHUG_AUTO_SYNC` を出さない（devenv では本物のバックグラウンド同期を試せるようにする）。devenv 自身が Store を作るときは `HEDGEHUG_AUTO_SYNC=0` 相当で起動しない。
- `remotes/org-a/web.git` は、使うフェーズ3のコンテキスト解決で足す。

### git を外した CI ジョブ

`.github/workflows/ci.yaml` に、ubuntu で PATH から git を外して `go test ./...` を流すジョブを足す（`git` を探せない PATH を作り、`go` と `gotestsum` だけを置く）。git を使うテスト（merge driver の `git merge`）は、`exec.LookPath("git")` が失敗したら skip する。それ以外のテストが git なしで通ることを確かめる。

## エラー

- `ErrDirty`: 作業ツリーの未コミット変更がマージと重なる。「`hh doctor` で確認し、変更をコミットするか戻してから `hh sync`」を案内する。終了コード 1。
- push の再試行を使い切った: 最後の拒否を返す。ローカルのコミット（マージコミットを含む）は残り、次の同期でその上からマージする。
- remote が見つからない・読めない: そのまま返す（`sync.json` に記録）。
- merge driver の失敗: 0 以外で終わり、git の既定の衝突に任せる。

## テスト

- **merge の表テスト**: 前記の表の各行（片側・両側・同じ値・LWW の各段の決着・集合マージ・削除と編集・purge と編集・diff3 の重なり・yaml の要素の追加と削除・numbers.yaml の最大・ワークスペースの `id_prefix` と `prefix_set` の組・両側が `id` を変えたときの `former_ids`・その他のファイル）。
- **対称性の fuzz**: ランダムな base と2つの変更から、ours と theirs を入れ替えても結果のバイト列と上書きの集合が同じこと。
- **diff3**: 重ならない変更、同じ変更、重なる変更、末尾の改行、空のファイル。
- **repair**: 表の各行。親の2つ組・3つ組の循環（base あり・なし）、削除済みの親、消されたワークスペースの復活、不正な接頭辞の base への戻し、ワークスペースの違う子、ID の衝突（origin にある側を残す、どちらもない・両方あるときは ULID の小さい側、3つ以上の組、振った番号がマージした集合の最大 + 1 になる、読めないファイルと numbers.yaml の番号を数える）、接頭辞の不一致の振り直し、接頭辞と名前の重複の案内。決定性（同じ入力から同じ結果）。
- **gitrepo**: インプロセスの file transport で fetch / push / clone（git の実行ファイルを呼ばないことを、PATH を空にして確かめる）、拒否の検出、`LastChange`、`DiffTrees`、`FastForward`、`Repack`。
- **store**: `CommitMerge`（親の指定、検査をしない、`ErrHeadMoved`）、重複のあるワークスペースの変更で、変えていないフィールドの重複では拒否しないこと。
- **sync**: fast-forward、マージ、拒否からの再試行（テスト用のフックで別のクローンが間に push する）、ID の衝突の振り直し（2つのクローンで同じ ID を振り、後から同期した側が振り直す。両方が収束する）、push の間のローカルの書き込みが失われない、手でマージした後の修復のコミット、ミラーへの push、`ErrDirty`、`.gitattributes` の追加、Store 間の接頭辞の重複の案内。
- **プロパティテスト**（完了条件）: 1つの bare remote と3つのクローンを作り、ランダムな操作（add / set / status / mv / `mv --workspace` / rm / restore / purge / comment / 本文の編集 / schema.yaml の変更 / `workspace add` / `workspace prefix` / `workspace rename` / `workspace rm`）とランダムな順序の同期を繰り返す。同期をはさまずに複数のクローンで add することで、ID の衝突を必ず起こす。最後に全クローンが変わらなくなるまで同期し、次を確かめる。
  1. 全クローンの HEAD のツリーが一致する。
  2. 値が失われていない。
     - frontmatter のフィールドの書き込みは、最終の値である、因果的に後の書き込み（その書き込みのコミットを祖先に持つコミットでの同じフィールドの書き込み）で置き換えられた、`Hedgehug-Overwritten` に記録された、`Hedgehug-Repaired` で戻された、のどれか。
     - `id` の書き込み（採番）は、最終の `id` か `former_ids` に残っている。
     - `deleted_at` の書き込みは、上に加えて、復活の通知（`Notice{revived}`）がある場合も可。
     - 本文の書き込みは、書いた各行が、因果的に後の書き込みで消されたものを除いて、最終の本文（重なりで並べた部分を含む）に残っている。
     - 集合のフィールドは、足した要素が、因果的に後の書き込みで消されたものを除いて残っている。
  3. 不変条件が守られている: 親の循環がない、削除されていないタスクの親は削除されていない、`id` が重複しない、どの ID の番号も再利用されていない（あるタスクに一度振られた ID を別のタスクが持たない）、全タスクのワークスペースが存在する、子は親と同じワークスペース、ID の接頭辞はワークスペースの接頭辞と一致する（接頭辞の形式が正しいワークスペースで）。
  seed を固定し、`-short` では回数を減らす。失敗したら seed と操作の列を出す。
- **CLI**: `sync`（振り直しの表示を含む）、`sync --status`、`store add`（接頭辞の重複の警告を含む）、`store init --prefix --remote`、`inbox`（read / revert、renumbered と prefix_duplicate の表示）、`watch`、`undo`（他人の後の変更を残す場合、`mv --workspace` の取り消しと配下）、`restore --at`（ワークスペースが違う場合の案内）。出力はゴールデンテストで固定する。
- **E2E**: CLI のバイナリで alice と bob のシナリオ（同じタスクを同時に編集 → 両方 sync → 収束、bob の inbox に上書き、`inbox revert`、`undo`、同期前にそれぞれ add して ID が衝突 → 後の側に renumbered）。バックグラウンド同期は、`push_interval` を短くして実際に子プロセスを起動し、remote に届くことを確かめる。
- **merge driver**: 本物の git で2つのクローンを作り、`git pull` でフィールド単位のマージになることを確かめ、続く `hh sync` で ID の衝突が修復のコミットで直ることを確かめる（git がなければ skip）。

## 前のフェーズからの持ち越しの扱い

| 持ち越し | 出どころ | 扱い |
| --- | --- | --- |
| schema.yaml / automations.yaml の LWW で比べる時刻 | 1a | 「schema.yaml / automations.yaml / store.yaml」の節で決めた |
| merge driver の登録の確認 | 1d-2 | doctor の警告 |
| `update` が途中で失敗すると `ix.tasks` が半端に残る | 1c | 同期の後の index の更新で通る経路なので、失敗を注入するテストで、次の `Open` で正しい状態になることを確かめる |
| コメントの author、`created_by` | 1d-1 | 「自分」の節で埋める |
| ID の衝突の振り直しと通知、`workspaces/` と `numbers.yaml` のマージ、接頭辞の重複の案内 | 1.5 | 「マージ後の修復」「numbers.yaml」「スプリント・リリース・テンプレート・ワークスペース」「Store 間の接頭辞の重複」で作る |

フェーズ1・1.5の spec で意図して決めたもの（restore の秒精度など）は変えない。

## 親設計ドキュメントへの反映

spec の承認後に、次を親設計に書き足す。

- 「フィールドごとのマージ規則」: schema.yaml などの LWW の時刻を「その側でそのキーを最後に変えたコミットの時刻」とすること。表に `workspace`、ワークスペースのファイル（`id_prefix` は `prefix_set` で決める）、`numbers.yaml`（接頭辞ごとの最大）を足し、両側が `id` を変えたら負けた ID を `former_ids` に足すこと。
- 「上書きされた変更の扱い」: `Hedgehug-Overwritten` の最初の欄を、タスクだけでなくコメントや yaml も指せるように、タスクからファイルのパスに変えること。
- 「マージ後の不変条件」: ワークスペースの復活、親とワークスペースの不一致、接頭辞の不一致の振り直し、名前の重複の案内を足し、ID の衝突で origin にどちらもない・両方あるときは ULID の小さい側を残すこと。手でマージした後は次の同期の修復のコミットで直すこと。
- 「書き込みの流れ」: trailer の一覧（`Hedgehug-Author`、`Hedgehug-Repaired`、`Hedgehug-Undo`）と、マージ・修復・setup の件名。
- 「消失への耐性」: undo と `restore --at` は `id`・`former_ids` を戻さないこと（`restore --at` は `workspace` も）。
- 「アーキテクチャと技術選定」のパッケージ構成に `internal/merge`・`internal/sync`・`internal/inbox` を足す。
