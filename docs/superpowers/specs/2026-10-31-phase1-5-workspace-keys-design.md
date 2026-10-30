# フェーズ 1.5: ワークスペースと人が読めるキー 設計

- 日付: 2026-10-31
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「人が読めるキー」「書き込みの流れ」「実装フェーズ」「詰めるべき詳細」。本書の決定に合わせて親設計も書き換える（「設計ドキュメントの書き換え」）
- 前段: `docs/superpowers/specs/2026-10-30-phase1d2-maintenance-design.md`（ほか 1a〜1d-1 の spec）

## 位置づけ

設計ドキュメント rev 232〜234 で、キーの採番が「同期の push で確定」から「作成時に採番」に変わった。フェーズ2（同期とマージ）の前に、キーの採番と、CLI のすべての表示・入力をキーにすることを済ませる。

検討の中で、接頭辞を container の種類のタスク（`key_prefix`）に持たせる設計が破綻していることがわかった。project に置き場所の制限がなく、ストーリーやタスクの下にも置けるため、「接頭辞はプロジェクトに紐づく」という想定と合わない。そこで、接頭辞を持つ単位として**ワークスペース**を新しく作る。

| 段階 | 内容 |
| --- | --- |
| 1.5（本書） | ワークスペースのモデルとコマンド、作成時の採番、振り直し、purge の記録、接頭辞の検査（Store の中と、config に登録された他の Store）、すべての表示と入力をキーと ULID に、短縮 ID の削除、doctor、設計ドキュメントの書き換え |
| 2 | 同期での検出と修復: キーの衝突の振り直しと通知、`workspaces/` と `.hedgehug/numbers.yaml` のマージ、接頭辞の重複の案内 |
| 3 | Store 名付きの表示（`org-a:API-12`）、Store 間の `mv` での振り直し |

### 利用者の決定

- 接頭辞を持つのはワークスペースだけ。1つの Store にワークスペースを N 個作れる。Store の既定の接頭辞はなくす
- ワークスペースはタスクとは別の定義（kind ではない）
- すべてのタスクはどれか1つのワークスペースに属する。属さないタスクは作れない
- project は子タスクにできない（ルートにしか置けない）
- 親を指定しない `hh add` は `--workspace` で指定する。ワークスペースが1つだけなら省略できる。Store の「既定のワークスペース」は持たない
- 既存の Store の移行は考えない（未リリース）
- `hh log` と `hh show` の履歴は、今のキーに置き換えて表示する
- `hh purge` は、消したタスクの番号を接頭辞ごとの最大値として記録する（add はその記録を書かない）
- 別のクローンで同じキーが振られたら、先に push した側を残し、未 push の側を振り直す（実装はフェーズ2）
- Store 間の接頭辞の重複は、マウント中の Store と照合して防ぐ。先勝ちとし、後からの側には変更を案内する
- 表示はすべてキーと ULID を並べる
- 人が打つ入力は、キーと完全な ULID の両方を受け付ける
- 短縮 ID は二度と使わない。表示・入力・コードのどこにも残さない

## データモデル

### ワークスペース

`workspaces/<ULID>.md` に置く。形式は sprints/ と同じで、frontmatter と本文（説明）からなる。

```markdown
---
id: 01JC0A1B2C3D4E5F6G7H8J9K0M
name: web
key_prefix: WEB
prefix_set: 2026-10-31T10:00:00+09:00
created: 2026-10-31T10:00:00+09:00
updated: 2026-10-31T10:00:00+09:00
---
Web フロントエンドのタスク。
```

- `id`・`name`・`key_prefix`・`prefix_set` は必須
- `prefix_set` は接頭辞を最後に設定した時刻。Store 間の重複で先勝ちを判定するのに使う。`hh workspace prefix` で更新する
- `name` は Store の中で一意（大文字小文字を区別しない）。使える文字は Store 名と同じ規則で、定義は `model.ValidName` の1つ（`config.ValidStoreName` と `model.ValidWorkspaceName` はこれを使う）
- `model.Workspace` を `model.Sprint` と同じ作りで足し、`model.WorkspacePath(id)` を `workspaces/<ULID>.md` とする

### タスク

- `workspace: <ULID>` を必須のフィールドとして足す。書き出す順は `id`・`key`・`former_keys`・`workspace`・`kind`…
- `key_prefix` を削除する（`Task.KeyPrefix`、`taskKeys` から外す）
- `key` は必須。`former_keys` はこれまでどおり
- 子は親と同じワークスペースに属する。ワークスペースをまたぐ `mv` では配下の全ファイルを書き換える

### store.yaml

- `key_prefix` を削除する（`StoreMeta.KeyPrefix`、`storeMetaKeys` から外す）。未リリースなので互換の処理は作らず、フォーマットのバージョンも 1 のままにする

### 種類

既定のスキーマ（`schema.Default()` と、そこから作る `schema.Dev()`）で、project の `parents` を `[root]` にする。スキーマは利用者が定義するものなので、独自のスキーマでは変えられる。

### キーと採番

- 形式は `{接頭辞}-{番号}`。番号は 1 から始まる10進数で、先頭に 0 を付けない
- 番号は接頭辞ごとの連番で、Store 全体で数える。次の番号は、次のうち最大のものに 1 を足した値
  - 全タスク（削除済みを含む）の `key` と `former_keys` のうち、その接頭辞の番号
  - 読めない（デコードできない）タスクファイルの中の、キーの形（`\b[A-Z][A-Z0-9]{1,9}-[1-9][0-9]*\b`）をした語の番号。キーを読み分けられないので、形の合う語をすべて数える。数えすぎても番号が飛ぶだけで、重複は生まない
  - `.hedgehug/numbers.yaml` のその接頭辞の値
  - 同じ操作の中ですでに振った番号
- ワークスペースごとではなく接頭辞ごとに数える。違いが出るのは、あるワークスペースが手放した接頭辞を別のワークスペースが使ったときで、その場合も続きから振るので番号は再利用されない
- 振り直しの決まりは1つにする: **キーがない、またはキーの接頭辞が所属するワークスペースの接頭辞と違うタスクは、書き込みのときに番号を振る**。古いキーは `former_keys` の末尾に足す。ワークスペースをまたぐ `mv`、`hh workspace prefix`、手で編集したファイルのどれも、この決まりで扱う
- 番号を振る対象は、その操作で `PutTask` したタスクだけ。振り直しが要るタスクを読むだけでは書き換えない

### purge の記録

`.hedgehug/numbers.yaml`（同期するファイル。`local/` ではない）。

```yaml
# purge したタスクの番号のうち、接頭辞ごとの最大値。採番で番号を再利用しないための記録
WEB: 42
API: 7
```

- `Tx.RemoveTask` が、消すタスクの `key` と `former_keys` の番号を接頭辞ごとに比べ、記録より大きければ同じコミットで更新する
- add はこのファイルを書かない
- 同期でのマージは接頭辞ごとに大きい方を採る（フェーズ2）

### 接頭辞の検査

- 形式は `^[A-Z][A-Z0-9]{1,9}$`（英大文字で始まる2〜10文字の英大文字と数字）
- 同じ Store のワークスペース同士で重複を禁止する。`Store.Update` で検査する
- ワークスペースを作るとき（`store init` を含む）と接頭辞を変えるときは、config に登録された他の Store のワークスペースとも照合し、重複していれば拒否する。エラーには、どの Store のどのワークスペースが使っているかを出す。開けない Store は照合を飛ばし、警告を出す
- 同期の後に重複が見つかった場合（フェーズ2）は、`prefix_set` が後の側に変更を案内する。Store の中では inbox と doctor、Store 間では解消するまで Store 名付きで表示する（フェーズ3）。Store の中で接頭辞が重複しても、番号は接頭辞ごとに数えるのでキーは重複しない

### 参照

ワークスペースをまたぐ参照（`parent` を除く `blocked_by` と `task_ref` 型のプロパティ）は許す。「参照は Store の中に閉じる」は今までどおり。

### キーの衝突（方針。実装はフェーズ2）

別のクローンで同じキーが振られた場合、採番の判定に使う remote（Store の最初の remote）にすでにある側を残し、未 push の側を同期のマージの中で振り直す。振り直した側は新しい番号を取り、古いキーを `former_keys` に残し、inbox に「WEB-43 を WEB-45 に振り直した」と通知する。`Resolve` は `key` を `former_keys` より先に引くので、WEB-43 で引くと残った側が見つかる。

## 採番の実装

### 置き場所

番号は `Store.Update` の中、つまり Store のロックの中で振る。add がロックの外で番号を決めると、並行する add と重複するため。

`index` が `store` を import しているので、`store` はインデックスを呼べない。そこで、`store` に最大番号を返すインターフェースを置き、`index` が実装する。

```go
// store
// KeyNumbers returns, for each prefix, the largest number used by the keys
// and former keys of the tasks as HEAD has them. Update calls it under the
// lock.
type KeyNumbers func() (map[string]int, error)

// SetKeyNumbers makes Update count numbers with f instead of reading every
// task file.
func (s *Store) SetKeyNumbers(f KeyNumbers)
```

- `index.Open` が `s.SetKeyNumbers(ix.keyNumbers)` を呼ぶ。`keyNumbers` は `Refresh` してから、エントリの `Key` と `FormerKeys` から接頭辞ごとの最大を返す。ロックの中では HEAD が動かないので、差分の更新で済む
- 読めないタスクファイルのエントリは、ファイルの中のキーの形の語の番号を `Entry.BrokenNumbers` に持ち（インデックスのキャッシュにも入る）、`keyNumbers` はそれも数える
- 設定されていなければ、`Store` は全タスクファイルを読んで数える（index を使わないテストや devenv のため）。読めないファイルは中身からキーの形の語を数える
- 数えた結果に `numbers.yaml` と、同じ操作の中で振った番号を合わせる

### Update の流れ

`fn` の後、検査の前に `tx.assignKeys(sc)` を呼ぶ。

1. `taskOrder` の各タスク（削除されたものを除く）について、ワークスペースを読む（同じ操作で `PutWorkspace` したものを優先）
2. キーがない、または接頭辞が違えば番号を振る。古いキーは `former_keys` へ
3. 最大番号は、最初に必要になったときに一度だけ `KeyNumbers` で取り、以後は手元で増やす

`assignKeys` は呼び出し側が渡した `*model.Task` の `Key` と `FormerKeys` をその場で書き換える。採番の後の検査や書き込み、コミットが失敗したときは、書き換える前の値に戻してからエラーを返す。失敗した操作が呼び出し側のタスクを変えたままにしないため。

コミットの件名は `hedgehug: <操作> <キー> <タイトル>`。タスクのない操作（ワークスペースの操作など）は `Op.Detail` に変更の内容を入れ、`hedgehug: <操作> <Detail>` にする。ワークスペースを変える操作は、`Op.Workspace` でそのワークスペースの ULID を trailer `Hedgehug-Workspace` に入れる（`Hedgehug-Op: workspace` はそのまま）。

### 書き込み時の検査（`tx.validate` に足す）

- `workspace` がない、または存在しないワークスペースを指している
- 子と親でワークスペースが違う
- ワークスペースの `key_prefix` の形式が違う、Store の中で重複している、`name` が重複している
- タスクが残っているワークスペースを消そうとしている（削除済みのタスクも数える）。読めないタスクファイルは所属を読めないが、中身に `workspace: <消すワークスペースの ULID>` があれば、そのワークスペースのタスクとみなして拒否し、エラーでファイルを示す。直したときに存在しないワークスペースを指すタスクにならないように
- キーの重複は書き込み時には検査しない。振る番号は使われた最大より大きいので、採番が重複を生むことはない。手で編集したファイルや同期による重複は doctor（`duplicate_key`）で知らせる

### Tx に足すもの

- `PutWorkspace(*model.Workspace)`、`RemoveWorkspace(id)`、`Workspace(id)`（同じ操作で書いたものを優先して読む）
- `Store.Workspaces()`、`Store.Workspace(id)`、`Store.Numbers()`（`numbers.yaml`）

## コマンド

### store init

`hh store init <name> --prefix <接頭辞> [--workspace <名前>] [--template dev]`

- `--prefix` は必須。Store と最初のワークスペースを1つのコミットで作る
- `--workspace` を省くと Store 名をワークスペース名にする
- 最初のワークスペースは、後から作るものと扱いが同じ（「既定」という印は持たない）

### workspace

| コマンド | 内容 |
| --- | --- |
| `hh workspace add <名前> --prefix <接頭辞>` | ワークスペースを作る |
| `hh workspace ls` | 名前・接頭辞・ULID・タスク数（削除済みを除く）を出す |
| `hh workspace rename <ws> <新しい名前>` | 名前を変える |
| `hh workspace prefix <ws> <新しい接頭辞>` | 接頭辞を変え、配下の全タスク（削除済みを含む）を1コミットで振り直す。`prefix_set` を更新する |
| `hh workspace rm <ws>` | タスクが1件もないとき（削除済みを含む）だけ消す。残っていれば件数を示し、`mv --workspace` か `purge` を案内する |

- `<ws>` は名前・接頭辞・ULID のどれでも指定でき、大文字小文字を区別しない
- `--json` に対応する
- コミットの件名は何を変えたかを示す: `hedgehug: workspace add web (WEB)`、`hedgehug: workspace rename web → front (WEB)`、`hedgehug: workspace prefix api API → BE`、`hedgehug: workspace rm web (WEB)`。trailer は `Hedgehug-Op: workspace` と `Hedgehug-Workspace: <ULID>`。件名の古い名前や接頭辞は、コマンドがロックの前に読んだもの
- `prefix` が振り直すタスクは、`Update` の中（Store のロックの中）でインデックスを `Refresh` してから集める。インデックスを読んだ後に並行して足されたタスクも振り直すため。古いキーもロックの中で読み、新しいキーはトランザクションに入れたタスクから出す

### add

- `--parent` を指定したら、親のワークスペースに入れる。`--workspace` も指定して親と違えばエラー
- 親がなければ `--workspace` で指定する。ワークスペースが1つだけなら省略できる。2つ以上あって指定がなければ、ワークスペースの一覧を添えてエラーにする
- `-p key_prefix=...` は廃止（タスクに `key_prefix` がなくなる）

### mv

- `hh mv <id> --workspace <ws>`: 配下ごと別のワークスペースのルートへ移し、振り直す。`--parent` と一緒に使うと、その親（指定したワークスペースのタスク）の下へ移す
- `--parent` で別のワークスペースのタスクを親にした場合も、同じく配下ごと移して振り直す
- `mv <id> --workspace <今いるワークスペース>` を `--parent` なしで指定したときは、親を変えない
- ワークスペースが変わるときは、移したタスクごとにキーの変化を1行ずつ（`WEB-3 → API-12` など）表示する。新しいキーは、コミットの後にインデックスを引かず、トランザクションに入れたタスク（`assignKeys` がその場でキーを入れる）から出す
- 配下のタスクは `Update` の中でインデックスを `Refresh` してから集める（削除済みも含む）。インデックスを読んだ後に並行して足された子も一緒に移すため。`rm`・`restore`・`purge` も配下を同じくロックの中で集める。`purge` は確認の前に一度集めて一覧を示し、ロックの中で集め直す。生きているタスクが加わっていれば拒否し、確認した一覧と変わっていれば「もう一度 purge する」よう案内して止める（`--yes` のときは集め直したものを消す）

### edit

- `key` と `former_keys` の変更は拒否する
- `workspace` の変更は `mv --workspace` を案内して拒否する。ただし、元の `workspace` が空か、このストアのワークスペースでないときは、直すための変更として受け付ける
- `workspace` の変更は「`hedgehug mv --workspace` を使う」と拒否する

### set

`workspace`・`key`・`former_keys` は設定できない（`TaskKeys` に入っているので今までどおり拒否する）。

### クエリ

`workspace:<ws>` を足す。`<ws>` は名前・接頭辞・ULID のどれでもよく、`,` で OR、`-` で否定できる。

## 表示

### 原則

タスクを表示するところはすべて、キーと完全な ULID を並べる。ULID は省略しない。

```
$ hh ls
KEY     ID                          STATUS  KIND  ASSIGNEES  LABELS  TITLE
WEB-42  01JBZ8K3QW7M2X9V4T6R5N8P0D  todo    task                     リリース準備

$ hh ls --tree
WEB-1 01JBZ7Q2M4N6P8R0S2T4V6W8X0 [doing] ログイン改修
└─ WEB-42 01JBZ8K3QW7M2X9V4T6R5N8P0D [todo] リリース準備

$ hh log
a1b2c3d  2026-10-31 10:00  kukv  hedgehug: status API-7 (01JBZ8K3QW7M2X9V4T6R5N8P0D) リリース準備
```

- 確認メッセージ: `Added WEB-42 (01JBZ8K3QW7M2X9V4T6R5N8P0D) リリース準備`
- `hh show`: 見出しにキーと ULID を出し、`workspace` の行を足す。parent・blocked_by・関連・子・逆参照も `WEB-42 (01JB…全桁) タイトル` の形にする。`key` と `id` の行は見出しに出るので出さない（`--json` の `id` はそのまま）。`former_keys` があれば出す
- 「候補が複数ある」エラー、`refName`、doctor と migrate の一覧も、キーと ULID にする
- エディタの一時ファイル名は `WEB-42-*.md`
- ファイルが壊れていてキーを読めないタスクは、キーの列を `?` にし、ULID だけを出す

### 履歴

- `hh log` と `hh show` の履歴は、件名 `hedgehug: <操作> <キー> <タイトル>` のキーの部分を、trailer `Hedgehug-Task` の ULID からインデックスで引いた今のキーと ULID に置き換える
- 件名がこの形でないとき、trailer がないとき（store init など）、ULID がインデックスにないとき（purge 済み）は件名のまま出す
- `--json` の `subject` は件名のまま（加工しない）。`task` に ULID、`key` に今のキー（引けなければ `null`）を足す
- コミットの件名を作る処理（`commitFor`）は、採番の後のキーを使う。ULID で代用するのは、キーのないタスク（ファイルが壊れているなど）のときだけ。purge で消すタスクのキーは、ディスクに残っているファイルから読む

### --json と --ids-only

- `--json`: `short` を消す。`id` と `key` を常に出し、`workspace`（`{"id", "name", "key_prefix"}`）を足す。参照（`blocked_by`、`show` の関連）は `{"id", "key", "title"}` にする
- `--ids-only`: 完全な ULID を1行に1つ出す。スクリプトで扱う変わらない値なので

### コメント

表示も入力も完全な ULID だけを使う。`comment edit/rm` は末尾一致をやめ、26文字の ULID だけを受け付ける。

## 入力

- 利用者が打つコマンドの案内（「先に X を restore する」など）はタスクをキーだけで示す。説明として名指しするときは「キー (ULID)」と併記する
- タスクの指定は、キー、古いキー、完全な ULID のどれでも受け付ける。大文字小文字は区別しない。`index.Resolve` は「キー → 古いキー → ULID」の順で引き、末尾一致の段を消す
- 短縮 ID を削除する: `index.ShortID`、`index.ShortLengths`、`Index.short`、`minShort`、`commentIndex.Short`、`minCommentInput`、`--json` の `short`

## doctor

次を足す。直し方を各問題に添える（1d-2 と同じく検査だけ）。

- キーやワークスペースがないタスク、存在しないワークスペースを指すタスク
- キーの形式が違うタスク
- キーの重複
- キーの接頭辞と所属するワークスペースの接頭辞の不一致（そのタスクを次に書き込むと振り直される）
- 親と違うワークスペースに属する子
- Store の中での接頭辞やワークスペース名の重複。重複したワークスペースのファイルそれぞれに、相手を名指して出す
- 壊れたワークスペースファイルや `numbers.yaml`（読めないものは `does not decode: …`）
- `numbers.yaml` の接頭辞の形式が違うもの、番号が 1 未満のもの。ファイルは読めているので `has an invalid entry: <接頭辞>: …` として、読めないものと分ける

問題コード `no_key`・`duplicate_key`・`key_prefix`・`no_workspace`・`workspace_mismatch` は index の問題で、`is:invalid` と `ls --json` の `problems` にも出る。

## テスト

- 採番: 最大値を `key`・`former_keys`・`numbers.yaml` から取る、削除済みも数える、同じ操作で複数件を振る、`KeyNumbers` の有無で結果が同じ
- 並行性: 2つの Store ハンドルから並行して add してもキーが重複しない
- 振り直し: ワークスペースをまたぐ `mv`、`workspace prefix`、手で接頭辞を変えた後の書き込み。番号を再利用しない、古いキーで引ける
- purge: 最大番号のタスクを purge した後の add で次の番号になる。`former_keys` の番号も記録される
- 接頭辞: 形式、Store の中での重複、config に登録された他の Store との重複、開けない Store を飛ばす
- 書き込み時の検査: ワークスペースのないタスク、親とワークスペースが違う子、タスクの残るワークスペースの削除
- `Resolve`: キー・古いキー・ULID で引ける、末尾一致では引けない
- CLI の出力はゴールデンテストで固定する: ls、tree、show、log、show の履歴、確認メッセージ、`--json`、`--ids-only`、workspace の各コマンド
- 短縮 ID の痕跡（`ShortID`・`ShortLengths`・`"short"`）がコードに残っていないことを grep で確かめる
- 既存のテストは、Store を作るところを `--prefix` 付き、タスクを `workspace` 付きに直す

## 設計ドキュメントの書き換え

本書の決定に合わせて、親設計の次の節を書き換える。

- 人が読めるキー: 全面的に書き換える（ワークスペース、Store 全体での接頭辞ごとの連番、purge の記録、振り直しの決まり、衝突、Store 間の先勝ち、表示の併記）。「番号は同期の push で確定します」「次の採番で振り直す」「採番機能を入れるときに既存のタスクにも振る」を消す
- データ形式: Store のファイル（`workspaces/`、`.hedgehug/numbers.yaml`）、タスクファイル（`workspace`、`key_prefix` の削除）、種類と階層（project はルートだけ）
- 新しい節「ワークスペース」
- コンテキスト解決: 最後の段を「ワークスペースが1つならそのルート、2つ以上なら選ばせる（端末でなければエラー）」にする
- マージ後の不変条件: 接頭辞の重複は `prefix_set` が後の側に変更を案内する。キーの衝突は先に push した側を残す
- CLI 仕様: コマンド一覧（`store init --prefix`、`workspace`、`add --workspace`、`mv --workspace`）、クエリ言語（`workspace:`）、出力形式（キーと ULID の併記、`--ids-only` は ULID）
- 実装フェーズ: フェーズ1.5を足し、フェーズ2の採番の項目を移す。フェーズ2には衝突と重複の検出・修復と通知、フェーズ3には Store 名付きの表示と Store 間の `mv` での振り直しを残す
- 詰めるべき詳細: 衝突の項目を決定済みにする
