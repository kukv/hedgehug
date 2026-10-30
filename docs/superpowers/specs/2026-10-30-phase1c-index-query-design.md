# フェーズ 1c: インデックス・短縮 ID・クエリ 設計

- 日付: 2026-10-30
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「実装フェーズ」1. Core とファイル形式、「インデックス」「人が読めるキー」「タスク間の関連」「参照は Store の中に閉じる」「クエリ言語」
- 前段: `docs/superpowers/specs/2026-10-30-phase1a-model-format-design.md`、`docs/superpowers/specs/2026-10-30-phase1b-store-git-design.md`

## 位置づけ

| 段階 | 内容 |
| --- | --- |
| 1a（完了） | 型・ファイル形式の codec・ULID・rank・パス・スキーマ検証 |
| 1b（完了） | Store のディスク読み書き、原子的な書き込み、ロック、go-git での1操作1コミット |
| 1c（本書） | インデックス、短縮 ID、クエリの基本、循環と Store 外参照の検出 |
| 1d | CLI コマンド群、devenv、`HEDGEHUG_HOME` |

### 1c で扱うもの

- `internal/index`: タスクのインデックス（キャッシュの読み込み・差分更新・保存）、短縮 ID、ID の解決、子と逆方向の関連、タスクごとの問題（`is:invalid`）
- `internal/query`: クエリ言語のパーサと評価、並び替え、グループ化
- `internal/store`: 書き込み時の参照先の存在と循環の検査、`.hedgehug/local/` の読み書き口
- `internal/gitrepo`: HEAD のツリー、ツリーの差分、blob の読み出し
- `internal/schema`: Rank の `bad_value`（1a からの持ち越し）、違反コード `missing_ref` と `cycle`
- `internal/model`: `ID` の `MarshalBinary` / `UnmarshalBinary`（gob でキャッシュに書くため）

### 1c で扱わないもの

- コメント・作業時間のインデックス、メンション（`mentions:@me`）、ウォッチャー（`watching:@me`）: フェーズ2
- `is:overwritten`: フェーズ2
- `store:`、Store をまたぐクエリ、アカウントによる `@me` の解決: フェーズ3
- `has:pr` などの GitHub 連携の条件: フェーズ5
- `sprint:` / `release:`: フェーズ6
- `--tree` の表示、出力形式: 1d（CLI 側で index の子の一覧を使って組む）
- 性能目標（1万タスクで `hh ls` 200ms）の計測と調整: フェーズ7

## パッケージ構成

```
internal/gitrepo/
  tree.go          HeadTree、DiffTrees、ReadFile、Files
internal/store/
  local.go         ReadLocal、WriteLocal（.hedgehug/local/ の読み書き）
  refs.go          書き込み時の参照先の存在と循環の検査
  validate.go      祖先をたどる処理を循環で止める（ErrCycle をやめる）
internal/schema/
  validate.go      Rank の検査、CodeMissingRef・CodeCycle
internal/model/
  id.go            MarshalBinary・UnmarshalBinary
internal/index/
  index.go         Index、Open、Refresh、Task、Tasks、Children、Reverse
  entry.go         Entry（インデックスに持つ1タスク分の値）
  build.go         全件の構築と差分更新、キャッシュの読み書き
  shortid.go       短縮 ID と Resolve
  problems.go      タスクごとの問題の計算
internal/query/
  parse.go         構文解析
  eval.go          意味の検査と評価
  sort.go          sort と group
```

依存の向きは `query → index → store → schema → model`、`index → gitrepo`。`store` は `index` を知らない。

親設計のパッケージ表ではインデックスは `internal/store` に入っているが、1c で `internal/index` に分けた。1b の `gitrepo` と同じく、親設計の表の更新を申し送る。

## gitrepo の追加

```go
type Change struct {
	Path    string // リポジトリのルートからの相対パス（スラッシュ区切り）
	Deleted bool
}

func (r *Repo) HeadTree() (plumbing.Hash, error)
func (r *Repo) DiffTrees(from, to plumbing.Hash, prefix string) ([]Change, error)
func (r *Repo) ReadFile(tree plumbing.Hash, path string) ([]byte, error)
func (r *Repo) Files(tree plumbing.Hash, prefix string) iter.Seq2[string, error]
```

- `HeadTree` は HEAD のコミットのツリーのハッシュを返す。
- `DiffTrees` は `prefix` 以下で追加・変更・削除されたファイルを返す。`from` のツリーがリポジトリにないときは `ErrTreeNotFound` を返す（呼び出し側は全件を作り直す）。
- `Files` は `prefix` 以下のファイルのパスを返す。順序は決めない。
- `gitrepo` はファイル形式を知らないまま。どのパスがタスクかは呼び出し側が `model.ParseTaskPath` で判断する。

## store の追加

### `.hedgehug/local/` の読み書き

```go
func (s *Store) ReadLocal(name string) ([]byte, error)  // なければ ErrNotFound
func (s *Store) WriteLocal(name string, data []byte) error // 一時ファイル＋rename
func (s *Store) Repo() *gitrepo.Repo
```

- `.hedgehug/local/` は Store が持つ場所なので、index はここを通して読み書きする。`name` はパス区切りを含まない名前に限る。
- `WriteLocal` はロックを取らない。

### 書き込み時の参照先の存在と循環

`Update` の検査（1b の「書き込み時の検査」）のあとに、`PutTask` したタスクごとに次を検査する。新規なら `old` は nil として扱う。

**参照先がない（`missing_ref`）**

- その書き込みで新しく足した参照について、参照先が Tx を反映した状態に存在するかを見る。新しく足した参照 = `cur` にあって `old` にない ID。
- 対象は `parent`、`blocked_by`、task_ref 型の props（`Multi` なら各要素）。task_ref 型かどうかは、そのタスクの実効スキーマで判断する。
- 存在 = `tx.Task(id)` が成功するか、ファイルはあるがデコードできない。削除済み（`deleted_at` あり）も存在として扱う。同じ Tx で `RemoveTask` したタスクは存在しない。
- 違反は `Violation{Code: "missing_ref", Field: <parent | blocked_by | prop 名>}`。
- 親が見つからないときの 1b の `ErrNotFound` による拒否は、この違反に置き換える。

**循環（`cycle`）**

- 新しく足した辺 `t → x` について、Tx を反映した状態で `x` から同じ種類の辺をたどり、`t` に戻るなら違反にする。辺の種類は `parent` と `blocked_by` の2つ。`x == t`（自己参照）も違反。
- もとからあった辺だけでできている循環は拒否しない。循環を抱えたタスクも、それと関係のない編集はできる。
- たどる途中で見つからない・デコードできないタスクに当たったら、そこでたどるのをやめる。
- 違反は `Violation{Code: "cycle", Field: <parent | blocked_by>}`。

**祖先をたどる処理（1b の `lineage`）**

- 同じ ID に戻ってきたら `ErrCycle` を返すのをやめ、そこでたどるのをやめる。それまでに集めた祖先を使う。新しく生じた parent の循環は上の検査で拒否する。
- 引数 `newParent` をなくす。直接の親が見つからないときは、親を変える書き込みかどうかにかかわらず、そこでたどるのをやめて `parentKind` を空文字にする。親が見つからない書き込みを拒否するのは `missing_ref` だけになる。
- これで、1b からの持ち越し「親が purge されたタスクが壊れていると、修復の書き込みが `ErrNotFound` になる」も直る（壊れたタスクは新規として扱われ、parent が「新しく足した参照」になるが、ファイルの読み出しエラーで止まらなくなる。ただし `missing_ref` で拒否されるので、修復するときは parent を外すか付け替える）。
- 親が見つからず `missing_ref` になるとき、`ValidatePlacement` も空の親（root）として `placement` を返すことがある。書き込みでは両方を載せる。
- `ErrCycle` は使われなくなるので消す。

違反があれば、1b と同じ `*ValidationError` で返す。スキーマの違反と参照の違反が両方あれば、1つの `ValidationError` にまとめる。

## schema の追加

- `ValidateTask` で `Rank` が空でなく `Rank.Validate()` に失敗したら、`Violation{Code: "bad_value", Field: "rank"}` を返す。
- 違反コードの定数 `CodeMissingRef = "missing_ref"` と `CodeCycle = "cycle"` を足す。検査そのものは Store 全体を見る必要があるので、store と index が行う。

## 短縮 ID と ID の解決

- 短縮 ID は、ULID の末尾から数えて、Store の全タスク（削除済みも含む）の中で一意になる最短の文字数（最低4文字）。ULID の先頭10文字はタイムスタンプで、同じ時期に作ったタスクどうしで長く重なるため、ランダム部分の末尾を使う。
- タスクが増えると伸びることがある（git の短縮ハッシュと同じ性質）。そのためコミットの件名など履歴に残す場所では ULID を使い、短縮 ID は表示と入力だけに使う。1b の申し送り（件名の `<タスク>` を短縮 ID にするか）は、ULID のままとする。

```go
func (ix *Index) ShortID(id model.ID) string
func (ix *Index) Resolve(s string) (model.ID, error)

type AmbiguousError struct {
	Input      string
	Candidates []model.ID
}
```

`Resolve` は次の順で探し、最初に当たった段階の結果を返す。大文字小文字は区別しない。

1. キー（`key`）の完全一致
2. 古いキー（`former_keys`）の完全一致
3. 26文字なら ULID の完全一致
4. 4文字以上なら ULID の末尾一致

同じ段階で複数当たれば `*AmbiguousError`、どこにも当たらなければ `ErrNotFound` を返す。削除済みのタスクも対象にする。

## インデックス

### 持つもの

対象はタスクだけ。1件ごとに次を持つ。

```go
type Entry struct {
	ID         model.ID
	Key        string
	FormerKeys []string
	Kind       string
	Title      string
	Parent     *model.ID
	Rank       model.Rank
	Status     string
	Assignees  []string
	Labels     []string
	BlockedBy  []model.ID
	Props      []Prop // ファイルの順。型の解釈はクエリの時点でスキーマに従う
	Created    time.Time
	Updated    time.Time
	DeletedAt  *time.Time
	Text       string // title と body を小文字にしたもの（自由語検索用）
	Violations []schema.Violation // スキーマの違反（キャッシュする）
	Broken     string // デコードできなかったときのエラー。空ならデコードできた
}

type Prop struct {
	Key    string
	Values []string // スカラーなら1つ、スカラーの列なら複数。それ以外の形は空
}
```

- デコードできないファイルは、パスから ID を取り、`Broken` にエラーを入れた Entry にする。ほかのフィールドは空。
- `Violations` は `ValidateTask` と `ValidatePlacement` の結果（そのタスクの実効スキーマと親の種類で求める）。ワークフローの遷移はタスク単体では判断できないので含めない。親が index にないときは、親の種類がわからないので `ValidatePlacement` をかけない（`missing_ref` だけを出す）。
- gob で書くため、`model.ID` に `MarshalBinary` / `UnmarshalBinary` を足す（`ulid.ULID` の同名のメソッドに委ねる）。ゼロ値の ID も往復できること。

### キャッシュ

- `.hedgehug/local/index` に、形式バージョン・HEAD のツリーハッシュ・全 Entry を gob で保存する。
- 内容は HEAD のツリーの blob から読む。キャッシュのキーと内容を一致させるため。コミットしていない手編集は反映しない（1d の `hh doctor` で知らせる）。

### `Open` と `Refresh`

```go
func Open(s *store.Store) (*Index, error)
func (ix *Index) Refresh() error // HEAD が変わっていれば差分を取り込む
```

1. キャッシュを読む。ない・デコードできない・形式バージョンが違うときは、全件を作り直す。
2. HEAD のツリーがキャッシュと同じなら、そのまま使う。
3. 違えば `DiffTrees(キャッシュのツリー, HEAD, "")` を取る。`ErrTreeNotFound` なら全件を作り直す。
   - `tasks/` 以下の変更は、そのファイルを blob から読み直して Entry を差し替える。削除なら Entry を消す。
   - `schema.yaml` が変わっていれば、全タスクの `Violations` を計算し直す。
   - そうでなければ、変わったタスクとその子孫（新しい状態の親子で求める）の `Violations` を計算し直す。親の種類と祖先の `scoped` が検査に効くため。変わっていない子孫の内容は blob から読み直してデコードする。
4. 参照の問題（下の「問題」）を全件について計算し直す。メモリ上のグラフの計算なので、キャッシュしない。
5. キャッシュを `WriteLocal` で保存する。保存に失敗しても `Open` は失敗させない（次回作り直すだけなので）。ロックは取らない。どの結果も、そのツリーに対しては正しいので後勝ちでよい。

全件の作り直しは `Files(HEAD, "tasks/")` で全ファイルを読む。

### 読み出し

```go
func (ix *Index) Task(id model.ID) (*Entry, bool)
func (ix *Index) Tasks() []*Entry              // 順序は決めない
func (ix *Index) Children(id model.ID) []*Entry // rank → ID の順
func (ix *Index) Roots() []*Entry               // 親のないタスクと、親が Store にないタスク。rank → ID の順
func (ix *Index) Reverse(id model.ID, rel string) []*Entry
func (ix *Index) Problems(id model.ID) []Problem
func (ix *Index) Schema() *schema.Schema
```

- `Reverse(id, rel)` は `rel` の関連で `id` を指しているタスクを返す。`rel` は `blocked_by` か task_ref 型の props の名前。symmetric の関連は、どちら側から張ったものも返し、重複はまとめる。
- 削除済みのタスクも返す。絞り込みは呼び出し側（クエリ）が行う。

### グラフをたどる処理

parent と blocked_by は循環を含みうる（`is:invalid` で見せるため、index は循環を受け入れる）。`project:` の評価、子孫の `Violations` の再計算、`Roots`、循環の検出など、parent と blocked_by をたどる処理はすべて訪問済みの集合を持ち、同じタスクに戻ったら止める。

### 問題（`is:invalid`）

```go
type Problem struct {
	Code  string // schema の違反コード、missing_ref、cycle、broken
	Field string
	Msg   string
}
```

タスクごとに次を集める。

- `Broken` があれば `broken`
- `Violations`（スキーマの違反）
- `missing_ref`: `parent`・`blocked_by`・task_ref 型の props が、index にないタスクを指している
- `cycle`: `parent` か `blocked_by` の循環に含まれている（循環に含まれる全タスクに付ける）

親設計では、同期のあとに生じたこれらは警告として表示する。1c はその材料を `Problems` と `is:invalid` で出す。

## クエリ言語

### 構文

- 空白で区切った項をすべて AND で結ぶ。
- 項は次のどれか。
  - `key:value`
  - `key<value`、`key>value`、`key<=value`、`key>=value`
  - `"語句"`（自由語。空白を含められる）
  - コロンも比較演算子もない単語（自由語）
- 値の `,` は OR（`priority:high,urgent`）。
- 項の先頭の `-` は否定（`-label:wip`、`-"語句"`）。
- 値 `none` は「空」に一致する（`assignee:none`、`due:none`）。
- 値も `"..."` で囲める（`label:"needs review"`）。

### キー

| キー | 意味 |
| --- | --- |
| `status` | ステータス名かカテゴリ名（`todo` / `doing` / `done` / `canceled`） |
| `kind` | 種類の名前 |
| `label` | ラベルのどれかが一致 |
| `assignee` | 担当者のどれかが一致。`@me` は `Options.Me` のどれかに展開する |
| `parent` | 直接の親。値は `Resolve` で解決する |
| `project` | 祖先のどれか。値は `Resolve` で解決する |
| props の名前 | 型に応じて比べる（下の「値の比べ方」） |
| `blocked_by` / `blocks` | `blocked_by:X` は X にブロックされているタスク、`blocks:X` は X をブロックしているタスク |
| task_ref 型の props の名前、`inverse` の表示名 | 関連で引く。`duplicates:X` は X を重複しているタスク、`inverse` の名前では逆方向。symmetric の関連は両方向とも一致する |
| `is:blocked` | `blocked_by` のタスクのうち、done / canceled カテゴリでないものが1つでもある（見つからないタスクは数えない） |
| `is:invalid` | `Problems` が1つでもある |
| `is:deleted` | 削除済み。`is:deleted` を含まないクエリは、削除済みのタスクを結果から除く |
| `sort:<field>`、`sort:-<field>` | 並び替え。`-` で降順。1つのクエリに1つまで |
| `group:<field>` | グループ化。1つのクエリに1つまで |

`sort` と `group` に使える field は `status`、`kind`、`label`（group のみ）、`assignee`（group のみ）、`title`、`created`、`updated`、props の名前。

### 値の比べ方

- ステータスのカテゴリは Store のスキーマのステータス定義で引く。
- props は Store のスキーマの型で解釈する。
  - `text` / `enum` / `multi_enum` / `user` / `url`: 大文字小文字を区別しない完全一致。複数値はどれかが一致すれば真。
  - `number` / `date` / `datetime` / `duration`: 値を型として解釈して比べる。比較演算子を使える。`date` は `2026-11-10`、`duration` は `model.Duration` の書式。
  - `bool`: `true` / `false`。
  - `task_ref`: 値を `Resolve` で解決して ID で比べる。
- ファイルの値が型として解釈できないタスクは、その項に一致しない（`is:invalid` で見つけられる）。
- 自由語は `Entry.Text` の部分一致（大文字小文字を区別しない）。コメントの本文は含めない。

### エラー

次は、項の位置（何番目の項か）と項の文字列を付けたエラーにする。打ち間違いを空の結果で隠さないため。

- 構文の誤り（閉じていない `"`、空の値）
- 未知のキー（組み込みのキーでも props の名前でも `inverse` の表示名でもない）
- 未知のステータス・カテゴリ・種類、enum の `values` にない値
- 比較演算子を使えない型への比較、型として解釈できない値
- `Resolve` できない ID、曖昧な ID
- `@me` を使ったのに `Options.Me` が空（「@me にはアカウントの設定が要る」。アカウントはフェーズ3）
- `sort:` か `group:` が2つ以上、使えない field

### API

```go
func Parse(s string) (*Query, error)
func Run(ix *index.Index, q *Query, opt Options) (*Result, error)

type Options struct {
	Me []string // assignee:@me に展開するアカウント
}

type Result struct{ Groups []Group }

type Group struct {
	Key   string // group: がなければ空
	Tasks []*index.Entry
}
```

- `Parse` は構文だけを見る。キーや値の意味の検査は `Run` で、index のスキーマを使って行う。
- 並び順: `sort:` があればその field、なければ rank。同点は rank → ID の順。空の値は昇順でも降順でも最後。
- グループの順: `status` はスキーマのステータスの順、`kind` はスキーマの種類の順、enum はスキーマの `values` の順、それ以外は値の昇順。空の値のグループ（`Key` は空）は最後。複数値の field（`label`、`assignee`、`multi_enum`）では、タスクは値ごとのグループに1回ずつ出る。
- `group:` がなければ、`Groups` はちょうど1つ。

## テスト

- **gitrepo**: `HeadTree`、`DiffTrees`（追加・変更・削除、`prefix` の絞り込み、`from` がないとき `ErrTreeNotFound`）、`ReadFile`、`Files`
- **store**
  - `missing_ref`: 新しく足した parent / blocked_by / task_ref が存在しないと拒否。もとからあった参照先なしは通す。削除済みへの参照は通す。同じ Tx で `RemoveTask` したタスクへの参照は拒否。同じ Tx で作ったタスクへの参照は通す
  - `cycle`: parent と blocked_by の新しい循環（2つ、3つ、自己参照）を拒否。もとからある循環を抱えたタスクの、関係のない編集は通る
  - `ReadLocal` / `WriteLocal`
- **schema**: Rank の `bad_value`
- **index**
  - 全件の構築と、キャッシュからの読み込みで同じ Entry になる
  - 差分更新: タスクの追加・変更・削除、`schema.yaml` の変更で `Violations` が変わる、親の種類の変更で子の `Violations` が変わる
  - キャッシュが壊れている・形式が違う・古いツリーがないときに作り直す
  - 短縮 ID: 最低4文字、衝突すると伸びる、削除済みも数える
  - `Resolve`: キー、古いキー、ULID、末尾一致、曖昧、なし、大文字小文字
  - `Children` / `Roots` / `Reverse`（inverse、symmetric）
  - `Problems`: broken、スキーマ違反、missing_ref、cycle
- **query**
  - `Parse` のテーブル駆動テスト（項の種類、OR、否定、引用、エラー）
  - `Run` のテーブル駆動テスト: キーごと、比較演算、`none`、自由語、`is:*`、削除済みの扱い、`@me`、エラーの各種
  - sort と group の順序
  - fuzz: `Parse` がパニックしない

## 後続への申し送り

- 1d: `hh ls --tree` は `Roots` と `Children` で組む。CLI は毎回 `index.Open` する。`@me` に渡すアカウントはフェーズ3まで空にする。
- 1d: `hh doctor` で、`Problems` を持つタスクと、作業ツリーに残ったコミットされていない変更（index に反映されない）を知らせる。
- フェーズ2: 同期のあとは `Refresh` で取り込む。コメントのインデックスとメンションを足す。
- フェーズ7: 1万タスクで性能を測る。全件の作り直し、`schema.yaml` の変更による全件の再検査、祖先の変更による子孫の読み直しが重いときは、ここで対処する。1c では最適化しない。
- 親設計ドキュメント: パッケージ構成の表で、インデックスを `internal/store` から `internal/index` に分けたことを反映する。短縮 ID の規則（末尾の最短一意、最低4文字）を「人が読めるキー」に書き足す。

## 完了条件

- Store に書いたタスクを index で引け、`Open` を繰り返すとキャッシュから同じ内容が読める。コミットのたびに差分だけが読み直される。
- キー・古いキー・短縮 ID でタスクを解決できる。
- 親設計のクエリの例（`status:doing priority:high,urgent due<2026-11-10 -label:wip kind:task is:blocked "検索語"` のうち、1c の範囲のもの）が期待どおりに評価される。
- 新しく生じる参照先なしと循環が書き込みで拒否され、もとからあるものは `is:invalid` で見つけられる。
- `make check` が ubuntu / macos / windows で通る。
