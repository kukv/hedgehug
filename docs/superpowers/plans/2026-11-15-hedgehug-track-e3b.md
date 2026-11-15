# hedgehug トラック E3b(保存したビュー)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 検索の条件とレイアウトの設定に名前を付けて、このマシンに保存できるようにする。作るものは次のとおり。state.db の表と API、別のブラウザのタブへの反映、検索の画面の「ビューとして保存」のダイアログ、ビューの画面と「未保存の変更」の「変更を破棄」「保存」、ビューのメニュー(名前を変更・複製・削除)、サイドバーの「保存したビュー」、パレットのビューの群と「ビューとして保存…」、プロパティの削除の確認の添え書き(03 の 265 行)。

**Architecture:** 保存した内容は state.db の表 `views` に、ビューごとに `{query, layout, table}` で置く。画面に出している内容(未保存の変更を含む)は、画面の状態の鍵 `view:{id}` に置く(E3b-3)。「未保存の変更」は、この 2 つを中身で比べて決める。ビューの画面は、検索の画面の部品(絞り込み欄・ツールバー・テーブル・ツリー)をそのまま使う。そのため、まず core の `createSearch` と部品を「どの画面の状態の鍵を使うか」を引数にとる形に直す(Task 3)。名前の重複の検査と、なくなったプロパティのキーの列を外す処理はサーバーが行う。キーは非表示の Store も含めて探す必要があり、それを知っているのはサーバーだけだからである。ビューを変えたら、新しい出来事の種類 `views` を SSE で流し、ほかのブラウザのタブはサイドバーの一覧と比べの元だけを読み直す。

**Tech Stack:** Go 1.27.3、huma v2、SQLite(`internal/infra/statedb`)。画面は Svelte 5(`web/app`・`web/packages/core`・`web/packages/ui`・`web/packages/api`、orval の生成)、e2e は Playwright。依存ライブラリは足さない。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/04-search-layouts.md`: 「サイドバー」の並び(44〜53 行)と「保存したビュー」(108〜110 行)、「検索」(354〜360 行)、「保存したビュー」の全節(362〜432 行)、「4 つのレイアウトに共通すること」の表と「画面の構成」「画面の状態」「閲覧のみの Store のアイテム」(436〜487 行)、「モックからの変更点」の 1001 行
- 01: 74・115・116・126・165(アドレスはビューの識別子)・178(画面の状態は別のタブへ反映しない)・149 と 173〜176(画面がなくなったらマイタスク)・186・200・242 行
- 02: コマンドパレットの「ビュー」の群(625 行)、「このクエリで」(631 行)、群の上限と並び(639 行)、Ctrl+Enter(645 行)、「ビュー」(654 行)、「ビューとして保存…」(663 行)、772 行
- 03: 169 行(ワークスペースの削除でビューは残る)、265 行(プロパティの削除の確認の添え書き)、608〜612・638・639 行
- 06: 155・161 行(保存したビューの一覧の反映)・180 行
- 08: 156・265 行(非表示・このマシンから削除した Store のアイテムは出ない。ビューは残る)
- 設計書: `2026-11-12-hedgehug-architecture-design.md` の「このマシンと外部の口」の `Views`・`Events` の行、`2026-11-12-hedgehug-storage-design.md` の 286 行
- 計画: `2026-11-15-hedgehug-track-e.md`、`2026-11-15-hedgehug-track-e1.md` の末尾「E3(保存したビュー)」、`2026-11-15-hedgehug-track-e3a.md` の末尾「E3b(保存したビュー)」、`2026-11-13-hedgehug-track-b2.md` の B2-20 と 79 行

## 始める前に

- ブランチは `feat/track-e3b`(develop 52f3fbdf から)。push は `git push -u origin feat/track-e3b`(ブランチの上流が `origin/develop` になっているため)。PR は develop 向けに 1 本。
- E3a は develop に入った(PR #230)。E4〜E7 はこの計画と並行して進む。E3b は E4〜E7 をブロックしない。

## Global Constraints

E1・E2a・E3a の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。いまの数: `internal/app/port` は口が 20 個で**満杯**(`Views` の口は `internal/app/views` に置く。`app/search` の `ScreenStates` と同じ)、`internal/infra/statedb` 13 個、`internal/app/search` 11 個、`internal/adapter/httpapi/routes.go` 206 行、`cmd/hedgehug/wire.go` 134 行、`wire_e2e.go` 222 行、`web/app/src/wire.ts` **299 行**(Task 4 で差し引き減らす。足さない)、`web/app/src/search.ts` 188 行、`web/app/src/screens/tree/SearchTree.svelte` **200 行**(Task 3 で import を引数に替えるだけ。行を増やさない)、`web/app/src/screens/search/SearchBody.svelte` 135 行、`web/app/src/shell/Sidebar.svelte` 168 行(ビューの一覧は別の部品にする)、`web/app/src/App.svelte` 146 行、`web/packages/core/src/features/palette/palette.ts` **282 行**、`features/palette/actions.ts` 253 行、`features/router/resolve.ts` 220 行、`features/screen/screen.ts` 232 行、`web/packages/core/src/index.ts` 206 行(ビューの公開物は新しい `viewsIndex.ts` から出し、`index.ts` は 1 行の再公開だけ足す)。
- 文言は仕様の文のとおりに書く(下の表の E3b-N に写した文がそれ)。閲覧のみの帯は core の `readOnlyBands`。
- API を変えたら `make generate` で `api/openapi.yaml` と `web/packages/api/src/generated.ts` を作り直してコミットする。
- 依存ライブラリは足さない。足す必要が出たら、`go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- トークンらしい値(`ghp_` などで始まる文字列)を、計画にもテストにも書かない。
- e2e のビルドも CI で lint する。本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。手元の検証は 1 回で足りる。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| E3b-1 | ビューが持つものは、名前と内容 `{query, layout, table}`。`layout` は `"tree"`・`"table"`(E5・E6 が値を足す)、`table` は検索の画面と同じ `{columns, group, sort}`。カンバン・ガントの設定は E5・E6 が足す。ツリーの開閉(`tree.collapsed`)は内容に入れない | 04 の 366〜379・418 行。E3a の申し送り |
| E3b-2 | ビューの識別子は、ULID を小文字にした 26 文字(`[0-9a-z]{26}`)。アドレスは `/views/{id}` | 01 の 165 行(名前を変えても変わらない識別子)。画面の状態の鍵の形 `^[a-z0-9:_/-]{1,200}$` に合わせる。大文字のままだと鍵の書き込みが 422 になり、`screenStates.write` は失敗を出さないので、未保存の変更が黙って残らなくなる |
| E3b-3 | 画面に出している内容は、画面の状態の鍵 `view:{id}` に、検索と同じ形 `{query, layout, table, tree}` で置く。鍵に何もないときは、保存した内容(開閉は空)で始める。「変更を破棄」は、この鍵を保存した内容で上書きする(開閉はそのまま)。ビューを削除したら、同じ処理の中でこの鍵の行も消す | 04 の 409〜420・430 行。名前の変更では識別子が変わらないので引き継がれ、複製では新しい識別子なので引き継がない |
| E3b-4 | 「未保存の変更」は、保存した内容と画面の内容の `{query, layout, table}` を、`readSearchState` で形をそろえてから比べる。列の並びの違いも変更とみなす。画面の状態を読み終える(最初の `open` が終わる)までは、印を出さない | 04 の 418 行。読む前は初めの値なので、比べると印がちらつく |
| E3b-5 | 名前の比べ方: 前後の空白を除き、大文字と小文字を区別しない(Go の `strings.ToLower(strings.TrimSpace(name))` を比べの鍵にする)。保存する名前は前後の空白を除いたもの。名前の変更では、自分自身との一致は重複としない。断りの文は「名前を入力してください」「同じ名前のビューがあります」(422) | 04 の 370・398・399・426 行 |
| E3b-6 | 2 つのブラウザのタブが同時に同じ名前で作っても重ならないように、ビューを書く操作(作成・保存・名前の変更・複製・削除)はサーバーの中で 1 つずつ行う(`views.Service` が `sync.Mutex` を持つ) | E3b-5。ビューを書くのはこのサーバーだけ(CLI は書かない) |
| E3b-7 | 並び: サーバーが比べの鍵の順(同じなら名前、さらに識別子の順)で返し、サイドバーとパレットはその順のまま使う | 04 の 110 行(名前順)、02 の 639 行(ビューはサイドバーの並び)。並べ方を 1 か所にする |
| E3b-8 | 保存(作成とビューの画面の「保存」)のとき、表示する列のうち `p:{キー}` の列で、そのキーがこのマシンのどの Store(非表示を含む)の削除されていないワークスペースにもないものを外す。判定は検索のエラーと同じキーの集まり(`app/search` の `catalog` のキー) | 04 の 401・416 行 |
| E3b-9 | ビューの画面の「保存」のあとは、サーバーが返した内容で画面の内容を置き換える(`restore`)。列を外したあとも印が残らないようにするため | E3b-4・E3b-8。外した列は、どのワークスペースにもないので画面にも出ていない |
| E3b-10 | ダイアログ「ビューとして保存」: 見出し「ビューとして保存」、入力欄の名前「名前」、確認の表示「レイアウト: {ツリー|テーブル} ・ 条件: {クエリ}」(クエリはコードの書体、前後の空白を除いて空なら「(なし)」)、ボタン「保存」「キャンセル」、Enter で保存。断りは入力欄の下に出し、ダイアログは開いたまま。保存できたら、トースト「ビュー「{名前}」を保存しました」を出し、そのビューの画面へ移る(履歴に積む) | 04 の 387〜403 行。E3a-14 と同じ組み立て |
| E3b-11 | 検索の画面の「ビューとして保存」のボタンは、ツールバーの右端(画面ごとの操作の位置)に置く。テーブルでもツリーでも出す。ビューの画面には置かない | 04 の 358・450 行、381・407 行 |
| E3b-12 | ビューの画面: 見出しの上(パンくずの位置)に「保存したビュー」、見出しはビューの名前。見出しの右に、未保存の変更があるときだけ印「未保存の変更」とボタン「変更を破棄」「保存」、その右にメニューのボタン(読み上げの名前「{名前} のメニュー」)。中身は検索の画面と同じ(閲覧のみの帯、絞り込み欄、注記、レイアウトの切り替え、設定、件数、テーブルかツリー)。「変更を破棄」は確認なしで戻す。「保存」のトーストは「ビューを保存しました」 | 04 の 405〜418 行 |
| E3b-13 | ビューのメニュー: 「名前を変更…」「複製」「削除…」の順。「名前を変更…」のダイアログは、見出し「ビューの名前を変更」、入力欄の名前「名前」(初めは今の名前)、ボタン「保存」「キャンセル」、Enter で保存、断りは入力欄の下。成功したら閉じるだけ(トーストなし)。「複製」は、保存した内容を「{元の名前} のコピー」(重なれば「{元の名前} のコピー 2」「… 3」と、空いている最小の番号)で作り、そのビューの画面へ移る(トーストなし)。「削除…」は確認ダイアログ(見出し「ビューを削除」、本文「ビュー「{名前}」を削除します。アイテムは消えません。元に戻せません。」、ボタン「削除」「キャンセル」)のあと削除し、トースト「ビューを削除しました」を出してホームへ移る(履歴に積まない。ワークスペースの削除と同じ `navigation.replace`)。ホームは E4 がマイタスクに置き換える | 04 の 422〜430 行。名前の変更と複製のトーストは 04 にない |
| E3b-14 | ビューの画面で、テーブルの選択・一括変更・一括削除・行末のメニュー・パレットの「選んでいるアイテム」・ツリーの操作は、検索の画面と同じに使える。Store が変わった知らせでは、検索の画面と同じく結果と候補の元を読み直す | 04 の 405・443 行(検索と同じ構成)。06 の 155 行 |
| E3b-15 | サイドバーの「保存したビュー」: Store ごとのブロックのあと、同期のカードの前。見出し「保存したビュー」、行はビューのアイコンと名前で、押すとビューの画面(Ctrl+クリック・中クリックは新しいタブ)。いま開いているビューの行を強調する(ナビと同じ `aria-current="page"`)。1 件もないときは「検索の画面から保存できます」 | 04 の 46〜53・61・108〜110 行 |
| E3b-16 | パレット: 「ビュー」の群は、名前に入力を含む(大文字と小文字を区別しない)ビューを、E3b-7 の順で最大 5 件。入力があるときだけ出す。Enter でそのビューを開き、Ctrl+Enter で新しいタブに開く。「このクエリで」の群は「検索で開く」「ビューとして保存…」「{N} 件をまとめて変更…」の順にする。「ビューとして保存…」は、パレットを閉じ、「検索で開く」と同じ道で検索の画面をテーブルで開き、そのクエリにしてから、検索が落ち着くのを待ってダイアログを開く(`createChangeAll` と同じ。移れなかったら何もしない)。無効にする条件はない | 02 の 625・631・639・645・654・663 行。E3a-22・E3a-23(新しいタブで保存した列を初めの値で上書きしない) |
| E3b-17 | 別のブラウザのタブへの反映: ビューを書いたら、出来事 `views`(Store は空)を流す。受けたタブはビューの一覧を読み直す。開いているビューの画面の絞り込みとレイアウトは書き換えず、印だけが新しい保存の内容との比べで変わる。開いているビューが消えたら(削除・識別子のないアドレス)、ホームへ移る(トーストなし) | 06 の 161・180 行、01 の 149・176 行 |
| E3b-18 | プロパティの削除の確認(03 の 265 行): 削除するプロパティのキーが、ほかのどのワークスペース(このマシンのすべての Store、非表示を含む、削除されていないもの)にもなく、そのキーを使う語(否定を含む)が検索の条件にある保存したビューが 1 つでもあれば、確認ダイアログの本文の末尾に「このキーを使う保存したビューの検索条件は、キーがなくなるためエラーになります。」を足す。調べられなかったときは足さない | 03 の 265・638 行。キーの範囲は検索のエラー(04 の 416 行)と同じ |
| E3b-19 | 06 の 161 行の「「変更あり」の表示」を「「未保存の変更」の印」に直す(04 の 409〜411 行の呼び方にそろえる) | ビューの画面の文言は 04 が定める(06 は 04 を参照する側) |
| E3b-20 | 設計書の `Views` の口は `internal/app/views` に置く(`port` は満杯)。`Events` の行の「知らせる」ものに「保存したビューが変わった」を足す | 設計書の「このマシンと外部の口」 |
| E3b-21 | 回すもの: カンバンの WIP 上限のステータスの改名・削除への付け替え(03 の 285〜289・369 行)はカンバンの設定を足す E5、ガントの設定は E6、Store の名前の変更でビューのクエリと `view:*` の画面の状態の `store:旧名` を書き換えること(08 の 140 行)は Store の名前の変更を作る G | それぞれの設定・操作がまだない |

## Review Focus

1. **「保存」のあとも「未保存の変更」の印が消えない。** プロパティ「customer」を全ワークスペースから消したあと、列「customer」を出していたビューで絞り込みを変えて「保存」すると、サーバーは列を外して保存する。画面の内容に列が残っていると、比べで印が残る(E3b-9)。Task 1 の `TestSavePrunesDeadColumns` と Task 6 の `save replaces the screen with the saved content` に入れる。
2. **未保存の変更が再読み込みで消える。** ビューの画面で絞り込みを変えて再読み込みすると、印と絞り込みが残っている(識別子が大文字だと画面の状態を書けない。E3b-2)。Task 1 の `TestCreateUsesLowercaseID` と Task 6 の e2e に入れる。
3. **別のタブの変更で、開いている絞り込みが書き換わる・消えたビューに居残る。** タブ A でビュー「バグ」を開いて絞り込みを変えたまま、タブ B で同じビューを保存すると、タブ A の絞り込みはそのままで、印だけが新しい保存の内容との比べに替わる。タブ B で削除すると、タブ A はホームへ移る(E3b-17)。Task 4 の `views event reaches reflect` と Task 11 の e2e に入れる。
4. **名前の比べがずれる。** 「Bugs」があるとき「 bugs 」は「同じ名前のビューがあります」、「Bugs」を「BUGS」へ名前の変更はできる、「Bugs のコピー」があるときの複製は「Bugs のコピー 2」(E3b-5・E3b-13)。Task 1 の名前のテストに入れる。
5. **パレットの「ビューとして保存…」が、新しいタブで保存した列を初めの値で保存する・断ったのにダイアログが開く。** 新しいブラウザのタブで、検索の画面の列を「ID・タイトル・ポイント」にしてあるとき、パレットから「ビューとして保存…」で保存したビューの列は「ID・タイトル・ポイント」。「入力を破棄」で断ったら、ダイアログは開かない(E3b-16)。Task 9 の `saveQueryAsView waits for the search` と `saveQueryAsView does nothing when refused` に入れる。

---

### Task 1: 保存したビューの表と usecase

**Files:**
- Create: `internal/infra/statedb/views.go`、`internal/infra/statedb/views_test.go`、`internal/app/views/views.go`(口 `Store`・型・`Service` の作成・保存・一覧)、`internal/app/views/edit.go`(名前の変更・複製・削除)、`internal/app/views/views_test.go`、`internal/app/views/edit_test.go`
- Modify: `internal/app/search/keys.go`(`CustomKeys` を足す)、`internal/app/search/search_test.go` か既存のキーのテスト

**Interfaces:**
- Produces(`internal/app/views`):
  - `type Content struct { Query string; Layout string; Table Table }`、`type Table struct { Columns []string; Group string; Sort *Sort }`、`type Sort struct { Key string; Desc bool }`(JSON の綴りは `query`・`layout`・`table`・`columns`・`group`・`sort`・`key`・`desc`。画面の状態 `search` と同じ)
  - `type View struct { ID string; Name string; Content Content }`
  - `type Store interface { List(ctx) ([]View, error); Get(ctx, id string) (View, bool, error); Put(ctx, v View) error; Delete(ctx, id string) error }`(`Delete` は画面の状態 `view:{id}` の行も消す)
  - `type Service struct { Store Store; IDs port.IDs; Keys func(ctx context.Context) (map[string]bool, error); Events port.Events; mu sync.Mutex }`(使う側は `*Service`)
  - `func (s *Service) List(ctx) ([]View, error)` — E3b-7 の順
  - `func (s *Service) Create(ctx, name string, c Content) (View, error)`、`Save(ctx, id string, c Content) (View, error)`、`Rename(ctx, id, name string) (View, error)`、`Duplicate(ctx, id string) (View, error)`、`Delete(ctx, id string) error` — 書いたら `Events.Publish(port.Event{Kind: port.EventViews})`(`EventViews` は Task 2 で足す。このタスクではまだ定数がないので、Task 1 のうちに `port/events.go` に `EventViews EventKind = "views"` とコメント「保存したビューが変わったこと(E3b-17)。Store は空」を足してよい)
  - `func NameKey(name string) string` — E3b-5 の比べの鍵
  - 見つからない識別子は `port.ErrNotFound`、名前の断りは `refusal.New(…)`
- Produces(`internal/infra/statedb`): `func (d *DB) Views() Views`、移行 `views/001`(`CREATE TABLE views (id TEXT PRIMARY KEY, name TEXT NOT NULL, body TEXT NOT NULL)`。`body` は `Content` の JSON)
- Produces(`internal/app/search`): `func (s Service) CustomKeys(ctx context.Context) (map[string]bool, error)` — `catalog` のキー(小文字)

- [ ] **Step 1: 失敗するテストを書く**

`statedb/views_test.go`:
- `TestViewsRoundTrip`: `Put` → `Get` で同じ `View`(内容の列・並べ替えを含む)、`List` に 1 件、もう一度 `Put` で上書き、ない識別子の `Get` は false。
- `TestViewsDeleteDropsScreenState`: `ScreenStates().Put(ctx, "view:abc", "{}")` と `Put(View{ID: "abc"})` のあと `Delete("abc")` → ビューがなく、`ScreenStates().Get(ctx, "view:abc")` が false。`"search"` の画面の状態は残る。

`app/views/views_test.go`・`edit_test.go`(このファイルの中に、`map` で持つ `Store` の偽物と、記録する `Events` の偽物を書く。`IDs` は `apptest.NewIDs()`):
- `TestCreateUsesLowercaseID`(Review Focus 2): 作ったビューの `ID` が `^[0-9a-z]{26}$` に合う。
- `TestCreateTrimsName`: 「 バグ 」で作ると名前は「バグ」。
- `TestCreateRefusesEmptyName`: 「   」→ 断りの文が「名前を入力してください」。
- `TestCreateRefusesSameName`(Review Focus 4): 「Bugs」があるとき「 bugs 」→「同じ名前のビューがあります」。
- `TestRenameOwnCase`(Review Focus 4): 「Bugs」を「BUGS」に変えられる。別の「Tasks」へは「同じ名前のビューがあります」。空は「名前を入力してください」。
- `TestDuplicateNames`(Review Focus 4): 「Bugs」の複製は「Bugs のコピー」、もう一度で「Bugs のコピー 2」、「bugs のコピー 3」を作ってから、さらにもう一度で「Bugs のコピー 4」。複製の内容は元の保存した内容と同じで、識別子は別。
- `TestSavePrunesDeadColumns`(Review Focus 1): `Keys` が `{"customer": true}` を返すとき、列 `["id", "p:customer", "p:gone", "title"]` で `Save` → 保存した列は `["id", "p:customer", "title"]`。返す `View` も同じ。`Create` も同じく外す。
- `TestListOrder`: 「b」「A」「a2」を作ると `List` は「A」「a2」「b」。
- `TestWritesPublishViews`: 作成・保存・名前の変更・複製・削除のそれぞれで `EventViews` が 1 回流れる。断られた操作では流れない。
- `TestUnknownViewIsNotFound`: ない識別子の `Save`・`Rename`・`Duplicate`・`Delete` は `errors.Is(err, port.ErrNotFound)`。

`app/search`: `TestCustomKeysIncludesHiddenStores`: 非表示の Store のワークスペースだけにあるキーも、`CustomKeys` に入る(既存の `catalog` のテストの組み立てを使う)。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/infra/statedb/ ./internal/app/views/ ./internal/app/search/`
Expected: FAIL(`Views`・`views.Service`・`CustomKeys` がない)

- [ ] **Step 3: 表・口・usecase を書く**

`Delete` は 1 つのトランザクションで `views` と `screen_states`(`screen = 'view:' || id`)の行を消す。`Service` の書く操作は `s.mu` を持ったまま、読み・検査・書きを行う。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./internal/infra/statedb/ ./internal/app/views/ ./internal/app/search/ && make lint`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal
git commit -m "feat(views): 保存したビューの表と usecase を足す(E3b Task 1)"
```

---

### Task 2: ビューの API・出来事 `views`・プロパティの削除の調べ

**Files:**
- Create: `internal/adapter/httpapi/views/views.go`、`internal/adapter/httpapi/views/views_test.go`、`internal/app/views/impact.go`、`internal/app/views/impact_test.go`、`internal/domain/query/uses.go`、`internal/domain/query/uses_test.go`、`web/packages/api/src/views.ts`、`web/packages/api/src/views.test.ts`、`web/packages/core/src/port/views.ts`
- Modify: `internal/app/search/keys.go`(`KeyHeldElsewhere`)、`internal/adapter/httpapi/events/events.go`(`writeFrame` に `views`)、`events_test.go`、`internal/adapter/httpapi/routes.go`(`Deps.Views appviews.Store`、`views.Register`)、`cmd/hedgehug/wire.go`・`wire_e2e.go`(`Views: db.Views()`)、`web/packages/api/src/sync.ts`(出来事の名前に `views`)、`sync.test.ts`、`api/openapi.yaml`・`web/packages/api/src/generated.ts`(生成)

**Interfaces:**
- Consumes: Task 1 の `views.Service`・`Content`・`View`
- Produces:
  - Go: `func UsesKey(text, key string) bool`(`domain/query`。組み込みでないキー `key` を使う語が、否定を含めて 1 つでもあるか。大文字と小文字を区別しない)
  - Go: `func (s Service) KeyHeldElsewhere(ctx, store port.StoreID, ws workspace.ID, key string) (bool, error)`(`app/search`。このマシンのすべての Store、非表示を含む、削除されていないワークスペースのうち、`(store, ws)` 以外に、追加プロパティのキー `key` を持つものがあるか)
  - Go: `Service.HeldElsewhere func(ctx, port.StoreID, workspace.ID, string) (bool, error)` を `views.Service` に足し、`func (s *Service) KeyImpact(ctx, store port.StoreID, ws workspace.ID, key string) (bool, error)` — E3b-18
  - API(`/api/views` の下。Store をまたぐので `/api/stores/{store}` の下に置かない):
    - `GET /api/views`(`list-views`)→ `{views: View[]}`
    - `POST /api/views`(`create-view`)本文 `{name, content}` → `View`(200)
    - `PUT /api/views/{id}`(`save-view`)本文 `{content}` → `View`
    - `POST /api/views/{id}/rename`(`rename-view`)本文 `{name}` → `View`
    - `POST /api/views/{id}/duplicate`(`duplicate-view`)→ `View`
    - `DELETE /api/views/{id}`(`delete-view`)→ 204
    - `GET /api/views/key-impact?storeId=&workspaceId=&key=`(`view-key-impact`)→ `{affected: boolean}`
    - 型 `View { id, name, content: ViewContent }`、`ViewContent { query: string, layout: "tree"|"table", table: { columns: string[], group: string, sort: {key, desc} | null } }`。`id` の形は `^[0-9a-z]{26}$`。内容の JSON が 64 KiB を超えたら 422「ビューが大きすぎます(64 KiB まで)」
  - SSE: `event: views`、`data: {"store":""}`
  - api: `createViewsPort(): ViewsPort`
  - core(`port/views.ts`): `interface ViewContentDto`(API の形)、`interface ViewDto { id; name; content: ViewContentDto }`、`interface ViewsPort { list(): Promise<ViewDto[]>; create(name, content): Promise<ViewDto>; save(id, content): Promise<ViewDto>; rename(id, name): Promise<ViewDto>; duplicate(id): Promise<ViewDto>; remove(id): Promise<void>; keyImpact(storeId, workspaceId, key): Promise<boolean> }`。422 の断りは、既存の port と同じく、断りの文を `message` に持つエラーで投げる(プロジェクトの名前の変更の `renameOrMessage` が受けている形)

- [ ] **Step 1: 失敗するテストを書く**

- `uses_test.go` `TestUsesKey`: `"customer:A"` と `"-Customer:A status:Todo"` は `customer` を使う、`"status:Todo customer"`(キーのない語)と `"status:Todo"` は使わない。
- `impact_test.go`: `TestKeyImpactWhenLastHolder`: `HeldElsewhere` が false、ビューの条件が `customer:A` → true。`TestKeyImpactHeldElsewhere`: `HeldElsewhere` が true → false。`TestKeyImpactUnused`: どのビューの条件にも `customer` がない → false。
- `app/search` `TestKeyHeldElsewhere`: 同じキーが別のワークスペース(非表示の Store のものを含む)にあれば true、自分のワークスペースだけなら false、削除済みのワークスペースだけなら false。
- `views_test.go`(API): 作成 → 一覧に出る、同じ名前の作成は 422 で本文に「同じ名前のビューがあります」、保存・名前の変更・複製・削除が通る、ない識別子は 404、`id` が `ABC` は 422、`layout` が `kanban` は 422、内容が 64 KiB を超えると 422、`key-impact` が `{affected: …}` を返す。
- `events_test.go`: `port.Event{Kind: port.EventViews}` が `event: views` と `data: {"store":""}` になる。
- `sync.test.ts`: `views` の出来事を受けて、種類 `views` を渡す。
- `views.test.ts`: `createViewsPort` の各メソッドが正しい道と本文で呼ぶ(既存の `*.ports.test.ts` の書き方)。

- [ ] **Step 2: 落ちることを確かめる**

Run: `go test ./internal/domain/query/ ./internal/app/views/ ./internal/app/search/ ./internal/adapter/httpapi/... && pnpm -C web test -- packages/api`
Expected: FAIL

- [ ] **Step 3: API・出来事・組み立てを書き、`make generate` を流す**

`routes.go` で `appsearch.Service` を 1 つ作り、`views.Service{Store: deps.Views, IDs: deps.IDs, Keys: svc.CustomKeys, HeldElsewhere: svc.KeyHeldElsewhere, Events: hub}` に渡す。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test ./... && make lint && pnpm -C web test -- packages/api`
Expected: PASS(`api/openapi.yaml` と `generated.ts` が変わっている)

- [ ] **Step 5: コミット**

```bash
git add internal cmd api web/packages/api web/packages/core/src/port/views.ts
git commit -m "feat(views): 保存したビューの API と出来事 views を足す(E3b Task 2)"
```

---

### Task 3: 検索の流れと部品を、画面の状態の鍵ごとに使えるようにする

**Files:**
- Create: `web/app/src/screens/search/SearchFilter.svelte`(閲覧のみの帯・絞り込み欄と候補・注記・検索ヘルプを `SearchScreen.svelte` から移す)
- Modify: `web/packages/core/src/features/search/search.ts`・`layout.ts`・`search.test.ts`、`web/app/src/search.ts`、`web/app/src/screens/search/SearchScreen.svelte`・`SearchBody.svelte`・`SearchToolbar.svelte`、`web/app/src/screens/tree/SearchTree.svelte`

**Interfaces:**
- Produces(core):
  - `SearchDeps.screen?: string`(初めは `searchScreen`)、`SearchDeps.fallback?: () => unknown`(画面の状態に何もないときに `readSearchState` に渡す値)
  - `Search.restore(content: { query: string; layout: SearchLayout; table: TableSettings }): void` — 3 つを置き換えて書き(開閉はそのまま)、待っている検索を取り消して、待たずに検索する。テーブル以外にするときは選択を解除する
- Produces(app):
  - `SearchFilter.svelte` の引数 `{ search: Search }`、`SearchBody.svelte`・`SearchToolbar.svelte`・`SearchTree.svelte` の引数に `search: Search` を足す(`SearchBody` は `actions?: Snippet` も受け、`SearchToolbar` を通してツールバーの右端に置く)
  - `search.ts`: `function activate(s: Search): () => void`(Store が変わった知らせと `selectedOne` が使う、いま出している検索の流れを置き換え、戻す関数を返す。初めは `search`)。知らせの読み直しは、ルートが `search` か `view` のとき `active.reload()` と候補の元の読み直しを行う(`view` のルートは Task 4 で足す。このタスクでは `search` だけを見る)

- [ ] **Step 1: 失敗するテストを書く**

`search.test.ts`:
- `uses the given screen key`: `screen: "view:abc"` で作ると、`open` は `"view:abc"` を読み、`setQuery` は `"view:abc"` に書く。`"search"` には書かない。
- `fallback when nothing saved`: 画面の状態に何もなく、`fallback` が `{query: "kind:bug", layout: "tree", table: {columns: ["id", "title"], group: "none", sort: null}}` を返すとき、`open` のあとの状態がその値で、検索は `kind:bug` で 1 回。
- `saved state wins over fallback`: 画面の状態に `{query: "a"}` があれば、`fallback` を使わない。
- `restore replaces content and searches`: `open` のあと、ツリーで `collapsed: ["s/1"]` の状態から `restore({query: "x", layout: "table", table: …})` → 書いた値の `query`・`layout`・`table` が置き換わり、`tree.collapsed` は `["s/1"]` のまま。待たずに `port.search("x", …)` が呼ばれる。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web test -- features/search`
Expected: FAIL

- [ ] **Step 3: core と部品を直す**

`SearchScreen.svelte` は `PageHead`・`SearchFilter`・`SearchBody` を並べるだけにし、`search` を渡す。`SearchTree.svelte` は `import { search }` を引数に替えるだけで、行数を 200 行以内に保つ。動きは変えない。

- [ ] **Step 4: 通ることを確かめる(検索の画面の通しのテストが回帰の門)**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web size && make e2e-bin && pnpm -C web exec playwright test e2e/search.spec.ts e2e/search-layout.spec.ts e2e/search-table.spec.ts e2e/search-table-select.spec.ts e2e/search-tree.spec.ts e2e/search-tree-keys.spec.ts`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "refactor(web): 検索の流れと部品を画面の状態の鍵ごとに使えるようにする(E3b Task 3)"
```

---

### Task 4: ビューの一覧・アドレス・別のタブへの反映(core)

**Files:**
- Create: `web/packages/core/src/features/views/views.ts`(一覧と書く操作)、`views.test.ts`、`web/packages/core/src/features/views/content.ts`(内容の読み方と比べ)、`content.test.ts`、`web/packages/core/src/viewsIndex.ts`、`web/app/src/views.ts`(app の組み立て)
- Modify: `web/packages/core/src/index.ts`(`export * from "./viewsIndex"`)、`model/sync.ts`(`SyncEventKind` に `"views"`)、`features/sync/reflection.ts`(`order` の末尾に `"views"`)、`reflection.test.ts`、`features/router/route.ts`・`route.test.ts`、`features/router/resolve.ts`・`resolve.test.ts`、`features/screen/screen.ts`・新しいテスト `screenView.test.ts`、`web/app/src/reloads.ts`、`web/app/src/wire.ts`、`web/app/src/App.svelte`(最初に一覧を読む)、`web/app/src/search.ts`(知らせの読み直しに `view` のルート)

**Interfaces:**
- Consumes: Task 2 の `ViewsPort`・`ViewDto`、Task 3 の `readSearchState`
- Produces(core):
  - `type ViewContent = { query: string; layout: SearchLayout; table: TableSettings }`、`interface SavedView { id: string; name: string; content: ViewContent }`
  - `function viewContent(s: { query; layout; table }): ViewContent`(形をそろえて取り出す。`readSearchState` を通す)、`function viewDirty(saved: ViewContent, screen: SavedSearch): boolean`(E3b-4)、`const viewScreenKey = (id: string) => "view:" + id`
  - `interface Views { list: Readable<SavedView[]>; load(): Promise<void>; create(name: string, c: ViewContent): Promise<SavedView | string>; save(id: string, c: ViewContent): Promise<SavedView | null>; rename(id: string, name: string): Promise<string | null>; duplicate(id: string): Promise<SavedView | null>; remove(id: string): Promise<boolean> }`、`createViews(deps: { port: ViewsPort; toasts: Toasts }): Views` — `string` は断りの文。断り以外の失敗はエラーのトーストを出して `null`/`false`。書いたあとは返った値で `list` を直してから `load` で読み直す。`load` の失敗は前の一覧を残す
  - `Route` に `{ name: "view"; id: string }`(`/views/{id}`。`id` が `[0-9a-z]{26}` でなければホーム)、`Resolved` に `{ name: "view"; viewId: string }`
  - `ScreenDeps.views: Readable<SavedView[]>` — `screen.ts` は、`view` に解決したとき一覧にその識別子がなければ `navigation.replace({ name: "home" })`(トーストなし)。一覧が変わったら解決し直す(catalog と同じ)
- Produces(app):
  - `views.ts`: `viewsPort`、`views = createViews(…)`、`openView(id: string): Promise<boolean>`
  - `reloads.ts`: `viewsReloads`、`eventReloads: Partial<Record<SyncEventKind, Set<() => Promise<unknown>>>>`(`github`・`inbox`・`sync`・`views` の集まり)。`wire.ts` の `reflect` の `github`・`inbox`・`sync` の 3 つの `if` を、`eventReloads` を回す 1 つの繰り返しに替える(`store` と `github` で `reloads` を呼ぶ行はそのまま。行数を減らす)。`views.load` を `viewsReloads` に登録する

- [ ] **Step 1: 失敗するテストを書く**

- `content.test.ts` `viewDirty`: 同じ内容 → false。`query` だけ違う → true。`layout` が違う → true。列の並びが違う → true。`tree.collapsed` だけ違う → false。設定を変えて戻した値 → false。
- `views.test.ts`: `create` が 422 の断りなら文を返し、`list` は変わらない。成功なら `list` に入る。`rename` の成功で `list` の名前が替わる。`remove` で `list` から消える。`load` の失敗で前の一覧が残る。
- `reflection.test.ts` `views event reaches reflect`(Review Focus 3): `views` の出来事で `apply` が `["views"]` で呼ばれる。
- `route.test.ts`: `/views/01jabc…`(26 文字)↔ `{name: "view", id}`。`/views/ABC` と `/views/a/b` はホーム。
- `screenView.test.ts`: 一覧にあるビューのアドレスは `{name: "view", viewId}`。一覧にない識別子はホームへ `replace`。開いているあいだに一覧から消えたら、ホームへ `replace`。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web test -- features/views features/sync features/router features/screen`
Expected: FAIL

- [ ] **Step 3: 書く**

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web size`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(core): 保存したビューの一覧・アドレス・別のタブへの反映を足す(E3b Task 4)"
```

---

### Task 5: ダイアログ「ビューとして保存」と検索の画面のボタン

**Files:**
- Create: `web/packages/core/src/features/views/nameDialog.ts`、`nameDialog.test.ts`、`web/packages/core/src/features/views/saveDialog.ts`(確認の表示の文と、保存できたあとの流れ)、`saveDialog.test.ts`、`web/app/src/screens/views/SaveViewDialog.svelte`、`web/app/src/screens/views/ViewScreen.svelte`(仮の中身)、`web/e2e/views.spec.ts`
- Modify: `web/packages/core/src/viewsIndex.ts`、`web/app/src/views.ts`、`web/app/src/screens/search/SearchScreen.svelte`(`SearchBody` の `actions` にボタン「ビューとして保存」)、`web/app/src/shell/Screen.svelte`(`view` の分岐)、`web/app/src/App.svelte`(ダイアログを置く)

**Interfaces:**
- Consumes: Task 4 の `Views.create`・`openView`・`viewContent`
- Produces:
  - `interface NameDialog { state: Readable<{ error: string | null } | null>; open(submit: (name: string) => Promise<string | null>): void; save(name: string): Promise<void>; close(): void }`、`createNameDialog(): NameDialog` — `submit` が `null` を返したら閉じ、文なら開いたまま `error` に入れる。返事を待つあいだの 2 回目の `save` と、閉じた・開き直したあとの返事は無視する(`createRenameProjectDialog` と同じ守り)
  - `function saveSummary(c: ViewContent): { layout: string; query: string | null }`(`layout` は `layoutChoices` の名前、`query` は前後の空白を除いて空なら null。画面は null を「(なし)」と出す)
  - `function saveAsView(deps: { views: Pick<Views, "create">; toasts: Toasts; open(id: string): Promise<unknown> }, name: string, c: ViewContent): Promise<string | null>` — 成功でトースト「ビュー「{名前}」を保存しました」を出し、`open` で移る
  - app: `openSaveView(content: ViewContent): void`(ダイアログを開く。検索の画面のボタンとパレットが使う)

- [ ] **Step 1: 失敗するテストを書く**

- `nameDialog.test.ts`: 成功で閉じる、断りで `error` に文が入り開いたまま、返事を待つあいだの 2 回目の `save` は `submit` を呼ばない、閉じたあとに着いた断りで開き直さない。
- `saveDialog.test.ts`: `saveSummary({query: "  ", layout: "table", …})` → `{layout: "テーブル", query: null}`。`saveAsView` の成功でトーストの文が「ビュー「バグ」を保存しました」で `open` が新しい識別子で呼ばれる。断りでは文を返し、トーストも `open` もない。
- `views.spec.ts` `saves the search as a view`: 検索の画面で `kind:bug` を入れ、ツリーに切り替え、「ビューとして保存」→ ダイアログに「レイアウト: ツリー」と `kind:bug`、空のまま Enter →「名前を入力してください」、「バグ」で Enter → トースト「ビュー「バグ」を保存しました」、アドレスが `/views/…`、見出しが「バグ」。もう一度検索の画面から「 バグ 」で保存 →「同じ名前のビューがあります」。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web test -- features/views`
Expected: FAIL

- [ ] **Step 3: 書く**

ダイアログは `RenameProjectDialog.svelte` と同じ部品(`Dialog`・入力欄・ボタン)で組み、E3b-10 の文言にする。e2e はビューの画面の見出しが出ればよい(画面の中身は Task 6)。このタスクでは、ビューの画面の仮の中身として `PageHead` だけを出す `ViewScreen.svelte` を作り、`Screen.svelte` の `view` の分岐につなぐ。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web size && make e2e-bin && pnpm -C web exec playwright test e2e/views.spec.ts`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): 検索の画面からビューとして保存する(E3b Task 5)"
```

---

### Task 6: ビューの画面と「未保存の変更」

**Files:**
- Create: `web/packages/core/src/features/views/viewScreen.ts`(画面の印・破棄・保存の流れ)、`viewScreen.test.ts`
- Modify: `web/app/src/screens/views/ViewScreen.svelte`(Task 5 の仮の中身を置き換える)、`web/app/src/views.ts`(ビューの検索の流れを作る関数)、`web/app/src/shell/Screen.svelte`、`web/packages/core/src/viewsIndex.ts`、`web/e2e/views.spec.ts`

**Interfaces:**
- Consumes: Task 3 の `createSearch`(`screen`・`fallback`・`restore`)・`SearchFilter`・`SearchBody`・`activate`、Task 4 の `Views`・`viewDirty`・`viewScreenKey`
- Produces:
  - app: `createViewSearch(id: string): Search` — `screen: viewScreenKey(id)`、`fallback` は一覧のいまの保存した内容(開閉は空)、`selection` は `searchSelection`
  - core: `interface ViewScreen { dirty: Readable<boolean>; discard(): void; save(): Promise<void> }`、`createViewScreen(deps: { id: string; views: Pick<Views, "list" | "save">; search: Pick<Search, "state" | "restore">; ready: Readable<boolean>; toasts: Toasts }): ViewScreen` — `dirty` は `ready` が真で、一覧のそのビューがあり、`viewDirty` が真のとき。`discard` は保存した内容で `restore`。`save` は画面の内容を `views.save` し、返った内容で `restore` してからトースト「ビューを保存しました」(E3b-9)

- [ ] **Step 1: 失敗するテストを書く**

`viewScreen.test.ts`:
- `dirty after a change`: 読み終えたあと、絞り込みを変えると真、元に戻すと偽。
- `not dirty before ready`: `ready` が偽のあいだは、内容が違っても偽。
- `discard restores the saved content`: `restore` が保存した内容で呼ばれる。
- `save replaces the screen with the saved content`(Review Focus 1): `views.save` が列を外した内容を返すとき、`restore` がその内容で呼ばれ、`dirty` が偽、トーストが「ビューを保存しました」。保存に失敗したら `restore` もトーストも呼ばない。
- `saved content changed elsewhere`(Review Focus 3): 一覧のそのビューの内容が替わると、`restore` は呼ばれず、`dirty` だけが新しい内容との比べになる。

`views.spec.ts` に足す:
- `unsaved changes survive reload`(Review Focus 2): ビュー「バグ」で絞り込みを `kind:task` に変える → 見出しの横に「未保存の変更」と「変更を破棄」「保存」。再読み込みしても、印と `kind:task` が残る。「変更を破棄」で `kind:bug` に戻り印が消える。レイアウトを切り替えると印が付き、戻すと消える。
- `save keeps the view`: 変えて「保存」→ トースト「ビューを保存しました」、印が消え、検索の画面に移ってからビューを開き直しても `kind:task`。
- `view screen has no save-as button`: ビューの画面に「ビューとして保存」のボタンがない。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web test -- features/views`
Expected: FAIL

- [ ] **Step 3: 書く**

`ViewScreen.svelte` は `createViewSearch` で流れを作り、`onMount` で `activate` してから `open` し、`open` が終わったら `ready` を真にする。離れるときに `activate` の戻す関数を呼び、選択を解除する。見出しは E3b-12 のとおり `PageHead` の `crumbs`(「保存したビュー」)と `actions`(印・「変更を破棄」・「保存」・メニューのボタン。メニューの中身は Task 7)に置く。200 行を超えるなら、見出しの右の部品を `ViewHeadActions.svelte` に分ける。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web size && make e2e-bin && pnpm -C web exec playwright test e2e/views.spec.ts`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): ビューの画面と未保存の変更を足す(E3b Task 6)"
```

---

### Task 7: ビューのメニュー(名前を変更・複製・削除)

**Files:**
- Create: `web/app/src/screens/views/ViewMenu.svelte`、`web/app/src/screens/views/RenameViewDialog.svelte`、`web/packages/core/src/features/views/viewMenu.ts`、`viewMenu.test.ts`、`web/e2e/views-menu.spec.ts`
- Modify: `web/app/src/screens/views/ViewScreen.svelte`(または `ViewHeadActions.svelte`)、`web/app/src/views.ts`、`web/packages/core/src/viewsIndex.ts`

**Interfaces:**
- Consumes: Task 4 の `Views.rename`・`duplicate`・`remove`、Task 5 の `createNameDialog`
- Produces:
  - core: `const viewMenuItems: { id: "rename" | "duplicate" | "delete"; label: string }[]`(「名前を変更…」「複製」「削除…」)、`function deleteViewBody(name: string): string`(「ビュー「{名前}」を削除します。アイテムは消えません。元に戻せません。」)
  - core: `duplicateView(deps: { views: Pick<Views, "duplicate">; open(id: string): Promise<unknown> }, id: string): Promise<void>`、`deleteView(deps: { views: Pick<Views, "remove">; toasts: Toasts; navigation: Pick<Navigation, "replace"> }, id: string): Promise<void>` — 成功でトースト「ビューを削除しました」と `replace({ name: "home" })`

- [ ] **Step 1: 失敗するテストを書く**

- `viewMenu.test.ts`: `deleteViewBody("バグ")` の文。`deleteView` の成功でトーストとホームへの `replace`、失敗では `replace` しない。`duplicateView` の成功で新しい識別子を `open`。
- `views-menu.spec.ts`:
  - `rename`: メニュー「バグ のメニュー」→「名前を変更…」→ 見出し「ビューの名前を変更」、入力欄が「バグ」。空で「名前を入力してください」、別のビューの名前で「同じ名前のビューがあります」、「BUG」に変えると見出しとサイドバーが「BUG」。未保存の変更があったら、名前を変えたあとも印が残る。
  - `duplicate`: 未保存の変更があるビューで「複製」→「バグ のコピー」の画面に移り、内容は保存した内容で、印はない。もう一度元で「複製」→「バグ のコピー 2」。
  - `delete`: 「削除…」→ 確認の本文が E3b-13 の文、「削除」→ トースト「ビューを削除しました」、ホームに移り、サイドバーから消える。戻るで削除したビューのアドレスに戻るとホームになる。同じ名前「バグ」で保存し直したビューには印がない。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web test -- features/views`
Expected: FAIL

- [ ] **Step 3: 書く**

メニューは `ProjectMenu.svelte` と同じ部品と開き方にする。確認は ui の `ConfirmDialog`。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web size && make e2e-bin && pnpm -C web exec playwright test e2e/views-menu.spec.ts`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): ビューのメニューに名前の変更・複製・削除を足す(E3b Task 7)"
```

---

### Task 8: サイドバーの「保存したビュー」

**Files:**
- Create: `web/app/src/shell/SidebarViews.svelte`
- Modify: `web/app/src/shell/Sidebar.svelte`(Store ごとのブロックのあと、同期のカードの前に置く)、`web/e2e/views.spec.ts`

**Interfaces:**
- Consumes: Task 4 の `views.list`・`Route` の `view`、既存の `Links`
- Produces: `SidebarViews.svelte` の引数 `{ current: Resolved | null; links: Links }`

- [ ] **Step 1: 失敗する e2e を書く**

`views.spec.ts` `sidebar lists views by name`: ビューがないときサイドバーに「検索の画面から保存できます」。「b」「A」を保存すると、見出し「保存したビュー」の下に「A」「b」の順。「A」を押すとその画面が開き、その行が `aria-current="page"`。

- [ ] **Step 2: 落ちることを確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test e2e/views.spec.ts -g "sidebar"`
Expected: FAIL

- [ ] **Step 3: 書く**

行は E3b-15 のとおり。アイコンは ui にある線の SVG の書き方に合わせる(ほかの行のアイコンと同じ大きさ)。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web check && pnpm -C web size && pnpm -C web exec playwright test e2e/views.spec.ts`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): サイドバーに保存したビューの一覧を足す(E3b Task 8)"
```

---

### Task 9: パレットのビューの群と「ビューとして保存…」

**Files:**
- Create: `web/packages/core/src/features/palette/views.ts`(ビューの候補と、「ビューとして保存…」の流れ)、`web/packages/core/src/features/palette/views.test.ts`
- Modify: `features/palette/palette.ts`(`PaletteDeps` に `views`・`saveQueryAsView`、`mainRows` に `view` の群)、`features/palette/queryGroup.ts`・`queryGroup.test.ts`、`web/app/src/palette.ts`、`web/app/src/views.ts`、`web/e2e/palette.spec.ts` か `web/e2e/views.spec.ts`

**Interfaces:**
- Consumes: Task 4 の `views.list`・`Route` の `view`、Task 5 の `openSaveView`、既存の `openSearch`・`search.settled`・`search.state`
- Produces:
  - `function viewCandidates(a: ActionDeps, list: SavedView[], input: string): Candidate[]`(E3b-16。`group: "view"`、`route: {name: "view", id}`、実行でそのビューを開く)
  - `QueryGroupDeps.saveAsView(query: string): Promise<void>`、候補 `key: "query:saveView"`・`label: "ビューとして保存…"`(「検索で開く」と「{N} 件をまとめて変更…」のあいだ)
  - `function createSaveQueryAsView(deps: { openSearch(q: string): Promise<boolean>; settled(): Promise<void>; state(): SearchState; openSave(c: ViewContent): void }): (query: string) => Promise<void>`
  - `PaletteDeps.views: Readable<SavedView[]>`、`PaletteDeps.saveQueryAsView(query: string): Promise<void>`

- [ ] **Step 1: 失敗するテストを書く**

- `views.test.ts`: `viewCandidates` は名前に入力を含むもの(「BUG」に「bu」が一致)を一覧の順で返し、`route` を持つ。入力が空なら `buildRows` が出さない。6 件あれば 5 件と「ほか 1 件あります…」(既存の `buildRows` の働き)。
- `views.test.ts` `saveQueryAsView waits for the search`(Review Focus 5): `openSearch` が真を返し、`settled` が解けるまで `openSave` を呼ばない。解けたあとの `state()` の `{query, layout, table}`(保存した列を含む)で `openSave` を呼ぶ。
- `views.test.ts` `saveQueryAsView does nothing when refused`(Review Focus 5): `openSearch` が偽なら `settled` も `openSave` も呼ばない。
- `queryGroup.test.ts`: 読めるクエリでは「検索で開く」「ビューとして保存…」「{N} 件をまとめて変更…」の順。「ビューとして保存…」の実行で、パレットを閉じてから `saveAsView(query)`。
- e2e: パレットで「バ」と打つと「ビュー」の群に「バグ」、Enter でその画面。`kind:bug` と打って「ビューとして保存…」→ 検索の画面がテーブルで開き、ダイアログに「レイアウト: テーブル」と `kind:bug`。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web test -- features/palette`
Expected: FAIL

- [ ] **Step 3: 書く**

`palette.ts` は 300 行を超えないように、ビューの候補は `views.ts` の関数を呼ぶ 1 行だけを足す。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web size && make e2e-bin && pnpm -C web exec playwright test e2e/palette.spec.ts e2e/views.spec.ts`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): パレットにビューの群と「ビューとして保存…」を足す(E3b Task 9)"
```

---

### Task 10: プロパティの削除の確認の添え書き

**Files:**
- Modify: `web/packages/core/src/features/workspaces/properties.ts`・`properties.test.ts`(`deletePropertyBody`)、`web/app/src/screens/workspace-settings/PropertiesPanel.svelte`、`web/e2e/properties.spec.ts`

**Interfaces:**
- Consumes: Task 2 の `ViewsPort.keyImpact`(app の `viewsPort`)
- Produces: `deletePropertyBody(name: string, count: number, views = false): string` — `views` が真なら末尾に「このキーを使う保存したビューの検索条件は、キーがなくなるためエラーになります。」

- [ ] **Step 1: 失敗するテストを書く**

- `properties.test.ts`: `deletePropertyBody("顧客", 2, true)` →「「顧客」を削除します。値の入っている 2 件から値が消えます。このキーを使う保存したビューの検索条件は、キーがなくなるためエラーになります。」。`views` を省くと今の文のまま。
- `properties.spec.ts` `delete warns about saved views`: 追加プロパティ「customer」を足し、検索の画面で `customer:A` をビュー「顧客」として保存する。設定でそのプロパティの「削除」→ 確認の本文の末尾に添え書きが出る。削除したあとビュー「顧客」を開くと、絞り込み欄の下に「「customer」というキーはありません」。どのビューも使わないプロパティの削除では添え書きが出ない。

- [ ] **Step 2: 落ちることを確かめる**

Run: `pnpm -C web test -- features/workspaces`
Expected: FAIL

- [ ] **Step 3: 書く**

`PropertiesPanel.svelte` は、削除の確認を開いたときに `viewsPort.keyImpact(storeId, workspaceId, key)` を聞き、着いたら本文に反映する。失敗したら添え書きなし(E3b-18)。

- [ ] **Step 4: 通ることを確かめる**

Run: `pnpm -C web test && pnpm -C web check && pnpm -C web size && make e2e-bin && pnpm -C web exec playwright test e2e/properties.spec.ts`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web
git commit -m "feat(web): プロパティの削除の確認に保存したビューの添え書きを足す(E3b Task 10)"
```

---

### Task 11: 別のタブへの反映の通しのテスト・文書・すべての確かめ

**Files:**
- Create: `web/e2e/views-tabs.spec.ts`
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/06-sync-notifications.md`(161 行。E3b-19)、`docs/superpowers/specs/2026-11-12-hedgehug-architecture-design.md`(`Views`・`Events` の行。E3b-20)、`docs/superpowers/plans/2026-11-15-hedgehug-track-e.md`、`2026-11-15-hedgehug-track-e1.md`、`2026-11-15-hedgehug-track-e3a.md`、`2026-11-13-hedgehug-track-b2.md`、この計画の末尾

- [ ] **Step 1: 通しのテストを書く**

`views-tabs.spec.ts`(同じブラウザのコンテキストで 2 つのページ。Review Focus 3):
- タブ A でビュー「バグ」を開き、絞り込みを `kind:task` に変える(印が付く)。タブ B で同じビューを開き、`kind:task` にして「保存」→ タブ A の絞り込みは `kind:task` のまま、印が消える。
- タブ B で名前を「BUG」に変える → タブ A の見出しとサイドバーが「BUG」。
- タブ B で削除する → タブ A はホームに移り、サイドバーから消える。

- [ ] **Step 2: 通ることを確かめる**

Run: `make e2e-bin && pnpm -C web exec playwright test e2e/views-tabs.spec.ts`
Expected: PASS(落ちたら、該当のタスクの部品を直す)

- [ ] **Step 3: 文書を直す**

- 06 の 161 行: E3b-19。
- 設計書: E3b-20。
- トラック E の分け方: 「ほかのトラックから回された項目の行き先」の A5c の「ビューとして保存」「ビューの群」、B5b の「保存したビュー」の行に「E3b で足した」を添える。
- E1 の計画の末尾「E3(保存したビュー)」: 「ビューとして保存…」に「E3b で足した」を添える。「`search.setQuery` で絞り込みを入れ替え、`/search` を開く」の行に「E3b ではビューに自分のアドレス(`/views/{id}`)と画面の状態(`view:{id}`)を持たせた(01 の 165 行、04 の 409〜420 行)」を添える。「G(診断と設定)」の Store の名前の変更の行に「保存したビューのクエリと `view:*` の画面の状態も同じく書き換える(E3b-21)」を足す。
- E3a の計画の末尾「E3b(保存したビュー)」の各行に「E3b で足した」を添える。
- B2 の計画の B2-20 と 79 行: 保存したビューの添え書きと検索欄の文に「E3b で足した」、WIP 上限の付け替えに「E5 に回した(E3b-21)」を添える。
- この計画の末尾「E5・E6・G に渡すもの」を、実装で変えたことに合わせて直す。

- [ ] **Step 4: すべてを確かめる**

Run: `make check && make e2e-bin && pnpm -C web exec playwright test e2e/views.spec.ts e2e/views-menu.spec.ts e2e/views-tabs.spec.ts e2e/palette.spec.ts e2e/properties.spec.ts e2e/search.spec.ts e2e/search-layout.spec.ts e2e/search-table.spec.ts e2e/search-table-select.spec.ts e2e/search-tree.spec.ts e2e/search-tree-keys.spec.ts`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add web/e2e docs
git commit -m "docs: 保存したビューに合わせて仕様と計画を直す(E3b Task 11)"
```

## 実装で計画から変えたこと

- E4a が先に develop に入り、ホーム(`/`)は「最後に開いていた画面へ移る」アドレスに、マイタスクは別のルート `mytasks` になった。E3b-13・E3b-17 の「ホームへ移る」は、仕様(04 の 430 行、01 の 149 行)どおりマイタスク(`navigation.replace({ name: "mytasks" })`)へ移る形にした。
- 値の型(`Sort`・`Table`・`Content`・`View`)は `internal/domain/savedview` に置き、`internal/app/views` は別名で公開した(infra は port と domain だけを読む規則のため)。
- `routes.go` の `NewAPI` が関数の行数の上限を超えたので、検索とビューの登録を `registerSearch` に分けた。API の本文の型名は既存の型とぶつからないように `CreateViewBody`・`SaveViewBody`・`RenameViewBody` にした。
- core の `Views.create` は `SavedView | string | null`(`null` は断り以外の失敗で、エラーのトーストを出し済み)。`rename` の `null` は、成功と、トーストを出し済みの失敗の両方。
- `views`・`viewsPort` は `web/app/src/wireBase.ts` で作り、`web/app/src/views.ts` から再公開した(`wire.ts` との循環を避けるため)。一覧は `wire.ts` の起動で `Promise.all([catalog.load(), views.load()])` のあとに `screen.start()`。
- 名前の変更・削除のサイドバーでの確かめは Task 8 で足した。サイドバーの e2e は、自分のサーバーを使う `web/e2e/views-sidebar.spec.ts` に置いた。
- 削除は `replace` なので、「戻る」の確かめは、削除したアドレスを開き直してホームになることで見た。
- 別のタブへの反映の e2e(`web/e2e/views-tabs.spec.ts`)では、絞り込み欄にフォーカスがあるタブは画面への反映を待つ(06)ので、変更したタブの絞り込み欄からフォーカスを外してから、もう一方のタブで保存した。

## E5・E6・G に渡すもの

### E5(カンバン)

- ビューの内容(`internal/domain/savedview.Content`(`internal/app/views.Content` はその別名)・core の `ViewContent`・API の `ViewContent`)に、カンバンの設定(列の分け方・スイムレーン・列ごとの WIP 制限)を足す。`viewDirty` の比べにも入れる(04 の 373・409 行)。`layout` の値 `kanban` を API の enum と `layoutChoices` に足す。
- WIP 上限は列の名前で持つので、ステータスの改名にこのマシンの保存したビューの上限を付け替える(03 の 285〜289 行)。削除されたステータスの列の上限は、次の保存で取り除く(03 の 369 行。E3b-8 の列の取り除きと同じ所で行う)。

### E6(ガント)

- ビューの内容に、ガントの設定(目盛り・ロードマップの有無)を足し、`viewDirty` の比べに入れる。`layout` の値 `gantt` を足す。

### G(Store の名前の変更)

- Store の名前を変えるとき、保存したビューのクエリ(state.db の `views`)と、画面の状態 `view:*` の `query` にある `store:旧名`(大文字と小文字を区別しない)を `store:新名` に書き換える(08 の 140 行)。`search` の鍵の書き換え(E1 の申し送り)と同じ所で行う。
