# hedgehug トラック A2(プロジェクトの画面と詳細パネルの骨組み)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 画面で、プロジェクトを作り、Epic・Task・Step を作り、詳細パネルでタイトルを変えられるようにする(トラック A の計画の A2 の行)。

**Architecture:** サーバーには、画面が要る読み出し(Store・メンバー・あなた)と、アイテムの操作(タイトルの変更・Step の追加・「上へ」・次の番号・番号で探す)を足す。画面は、アドレスの解釈・一覧の状態・ダイアログと詳細パネルの流れを `core` に置き、`ui` の部品と `app` の画面でつなぐ(設計書 6 節)。ほかのトラックの持ち場に入るもの(Store の一覧の口、メンバー、アドレス、ui の部品)は、共有部分の小さな PR として先に develop へ入れる。

**Tech Stack:** 土台と同じ(Go 1.27.3、huma v2、Svelte 5、orval、Vitest、Testing Library、Playwright)。新しい依存は足さない。

**Spec:**
- `docs/superpowers/plans/2026-11-12-hedgehug-track-a.md`(A2 の行・「仕様 02 の節と小計画」・未決の点。型と関数名は A1 に合わせる)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md`: 「アイテムの作成」(開き方と初期値・ダイアログの項目のうち A2 の分・ダイアログの中での連動のうち種類・ワークスペース・プロジェクト・作成したあと)、「詳細パネル」(冒頭・上から順の構成・タイトル・子と Step のうち一覧・追加・「上へ」・GitHub の区画の枠)、「プロジェクト」(冒頭・作成・画面の見出しと構成)
- 01「閲覧のみの Store」「画面の移り方と、共通の表示」「画面のアドレス」「新しいタブで開く」
- 04「サイドバー」(ナビの枠・Store ごとのブロック)、「ツリー」の「行」
- 08「トークンのない URL」(トークンの扱いだけ。画面は G)
- `docs/superpowers/specs/2026-11-12-hedgehug-architecture-design.md` 6 節・8 節
- 見た目: モック版 19(https://claude.ai/artifact/Qw2jqgdbYtbcA4VTMHGCVA)の `screens.js`(`Sidebar`・`PageProject`・`ProjectOverview`)、`detail.js`(`Peek`・作成ダイアログ・`createProjectDialog`)、`index.html` の CSS。振る舞いは仕様に従い、モックの動きを写さない。

**持ち場:** トラック A の持ち場に加え、A2 では次を作る。
- `web/app/src/shell/`(サイドバーと画面の切り替え。04「サイドバー」の持ち主が決まっていなかったので A2 が枠を作り、ワークスペースのメニューの残りの項目は B2 が足す。トラック B の計画の未決 1 の推奨)
- `web/app/src/screens/item/`(作成ダイアログと詳細パネル。どの画面からも開くので `screens/project` の外に置く)
- `web/app/src/screens/workspace/`(ワークスペースの画面の見出しだけの枠。タブの中身は B・E・F)

**待つもの:** A1(#134〜#137・#140・#142)と土台 Task 12(#138)。どちらも develop に入っている。

## Global Constraints

トラック A の計画の Global Constraints をすべて引き継ぐ(土台の Global Constraints を含む)。加えて:

- 画面の文言・トースト・断りの文言は、仕様の文字どおりに書く。仕様にない文言は足さない。足すときは「この計画で決めたこと」に載せる。
- 新しい依存(npm・Go)は足さない。ルーターも自前で書く。
- `api/openapi.yaml` と `web/packages/api/src/generated.ts` は手で直さない。API を変えたタスクで `make generate` と `cd web && pnpm gen` を実行し、同じコミットに入れる。
- 大きさの上限(Svelte 200 行・TS 300 行・Go ファイル 300 行・関数 60 行)を超えたら、上限を上げずに分ける。
- ui の部品は渡された値を出して events を返すだけ。値の計算・検査・操作の流れは core に置く(設計書 6 節)。
- 計画やテストに `ghp_` などトークンらしい値を書かない。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| 1 | ワークスペースの一覧は、B1(#149)が入れた `list-workspaces`(`GET /api/stores/{store}/workspaces` → `[{id, name, prefix}]`。配列は null もありうる)を使う | ユーザーの回答(2026-11-13)では A2 が先に作る予定だったが、計画を書いているあいだに B1 が develop に入った |
| 2 | Store の一覧は新しい口 `port.StoreList`(`List(ctx) ([]port.StoreInfo, error)`。`StoreInfo{ID, Name, Color, ReadOnly}`、表示中の Store だけをサイドバーの並びで返す)で読む。本物はトラック G が足す。それまで本物のサーバーは空の一覧を返す(`cmd` の `noStores`)。偽物は `apptest.StoreList` | `port.Stores` に一覧がない。Store の名前・色・閲覧のみ・非表示はマシンごとの設定(01)で、G の持ち場 |
| 3 | メンバーの一覧(`GET /api/stores/{store}/members` → `[{login, name}]`)と、あなたのログイン名(`GET /api/stores/{store}/me` → `{login}`)を、共有部分の PR で足す(`app/members`・`adapter/httpapi/members`)。あなたが決まらないとき(本物の `noIdentity`)、`/me` は 500 になり、画面はオーナーの初めの値を「なし」にする | プロジェクトの作成ダイアログのオーナー(初めの値はあなた。02)。メンバーは G の持ち場なので共有部分にする |
| 4 | 画面のアドレスは、パス(`/s/{Store名}/p/{HH-P1}/{タブ}`、`/s/{Store名}/w/{接頭辞}`、`/s/{Store名}/i/{HH-12}`、`/`)にする。解釈と組み立ては `core/features/router` に置き、History API との結び付けは `app` に置く。ライブラリは使わない | 01 はアドレスの綴りを定めない。サーバーはパスに index.html を返す(`httpapi/server.go` の `spa`) |
| 5 | トークンは、起動時にハッシュ(`#token=`)から読んで `sessionStorage` に移し、アドレスからハッシュを消す。再読み込みは `sessionStorage` から読む。アプリの中の操作で新しいタブを開くときは、行き先のアドレスに `#token=` を付けて開く。リンクの `href` にはトークンを入れない | 08: 右クリックの「新しいタブで開く」とアドレスの貼り付けはトークンのない URL になる。アプリの中の操作で開いたタブと再読み込みは使える |
| 6 | `/` と、解決できないアドレスは、本文の空いた「ホーム」を出す。マイタスク(トラック E)が入ったら、ホームをマイタスクに替える。見つからないときのトースト(転送の記録)は A5 で足す | 01 はマイタスクを開くと定めるが、マイタスクはまだない |
| 7 | 画面の ID からの解決: プロジェクト(`HH-P1`)は、一覧の状態(Store → ワークスペース → プロジェクト)から接頭辞と番号で探す。アイテム(`HH-12`)は、接頭辞でワークスペースを決め、新しい API `GET /api/stores/{store}/workspaces/{workspace}/items/by-number/{number}`(operationId `find-item-by-number`)で探す。アドレスの Store 名の Store になければ、サイドバーの並びの順にほかの Store を探す。旧い接頭辞(B1 の `get-workspace` の `oldPrefixes`)と転送の記録はたどらず、A5 で一緒に足す | 01「画面のアドレス」。旧い ID からたどる規則は A5 の転送の記録と一組 |
| 8 | ID の予告は、新しい API `GET /api/stores/{store}/workspaces/{workspace}/next-item-number` → `{number}`(operationId `next-item-number`)で読む。ワークスペースを選び直すたびに読み直す | 番号はワークスペースのログの `number` の行から決まる(A1 の決定 3)。画面はログを持たない |
| 9 | タイトルの変更は `PUT /api/stores/{store}/items/{item}/title` 本文 `{title}` → 更新後のアイテム(`rename-item`)。整えたあとが空なら断る(「タイトルは空にできません」)。前と同じなら保存せず、いまのアイテムを返す。アクティビティは「{ID} のタイトルを変更」、変更は `change.NewSet("title", 旧, 新)` | 02「タイトル」 |
| 10 | Step の追加は `POST /api/stores/{store}/items/{item}/steps` 本文 `{title}` → 作った Step(`add-step`)。作り方は作成と同じ(番号・並び・ウォッチ中の人)で、アクティビティだけ「{親の ID} に Step「{タイトル}」を追加」。担当者は親と同じ・ステータスは初期ステータス・優先度は初期優先度、は A3 で足す | 02「子と Step」。A3 がプロパティを足す |
| 11 | 「上へ」は `POST /api/stores/{store}/items/{item}/move-up` → `204`(`move-item-up`)。同じ親の兄弟を並び順(並び順の値、同じなら作成の時刻、同じなら内部 ID)に並べ、直前の兄弟と並び順の値を入れ替える(`change.NewSet("rank", …)` を 2 つのアイテムに 1 行ずつ、1 回の保存)。先頭なら何も保存しない。アクティビティは「{ID} の順位を変更」。A4 のメニューの「上へ移動」も同じ操作を使う | 02「子と Step」・04「並び順と並べ替え」 |
| 12 | 兄弟の並び順は、画面でも「並び順の値 → 作成の時刻 → 内部 ID」で決める(`core/features/items/tree.ts` の `compareSiblings`) | 04「ツリー」の「行」 |
| 13 | 断りは、api パッケージが 422 の応答の `detail` を読んで core の `Refused`(`message` が仕様の文言)にし、404 を core の `NotFound` にする。画面は `Refused` の `message` をダイアログの赤字かトーストに出す | トラック A の決定 1(API は 422 で `detail` に文言) |
| 14 | 画面の検査は、サーバーの検査に任せる(作成ダイアログで「作成」を押したら送り、`Refused` を赤字で出す)。例外は 2 つ: 詳細パネルのタイトルが空(送らずに「タイトルは空にできません」をトーストで出し、元に戻す)と、Step の追加の入力が空(何もしない) | 文言を 1 か所(Go の domain)に置く。02 の 2 つの例外は「送らない」動きなので画面で決める |
| 15 | プロジェクトの画面のタブは、A2 では「概要」「ツリー」だけを出す。カンバン・テーブル・ガントはトラック E、設定はトラック B・A4 が、自分のタブを足す | 02「画面の見出しと構成」。まだない画面を出さない |
| 16 | 概要のタブは、A2 では「Epic」の区画(一覧と「Epic を追加」)と、プロパティ(ステータス・オーナー・開始日・終了日)の表示だけを出す。タイトル・説明・プロパティの編集、リポジトリ、コメント、履歴は A4 | 02「概要の編集」は A4 |
| 17 | ツリーのタブは、A2 では行(開閉の矢印・種類のアイコン・ID・タイトル・子を追加する「+」)だけを出す。ステータス・担当者・進捗・期限・ポイント・ブロック中は A3・A4、メニュー・キー操作・絞り込み・ドラッグは A4 とトラック E | 04「ツリー」。A2 の完成の条件に要るのは作成と詳細パネルを開くこと |
| 18 | 詳細パネルは、A2 では、ヘッダー(パンくず・全画面・閉じる)、種類の行(種類・ID・ID のコピー)、タイトル、子(Epic は「Story・Task・Bug」、Story・Task・Bug は「Step」)、GitHub の区画の枠(見出しと「リンクはありません」)だけを出す。ウォッチ・メニュー・「ブロック中」・プロパティ・説明・関連・作業時間・コメント・履歴・幅のドラッグとその保存・子の行のステータスと担当者・Step のチェックと「×」・完了の数は A3・A4・D | 02「詳細パネル」。A2 の行の範囲 |
| 19 | 一覧の状態(Store → ワークスペース → プロジェクト)は、起動時と、プロジェクトを作ったあとに読み直す。アイテムを作った・変えたあとは、開いているプロジェクトの画面と詳細パネルを読み直す。ほかのタブ・同期での反映はトラック C | 06「他のタブへの反映」は C |
| 20 | 作成ダイアログの「続けて作成」は、ui の新しい部品 `Checkbox` で出す。詳細パネルは ui の新しい部品 `SidePanel`(右からせり出す・全画面)で出す | 02「ダイアログの項目」・「詳細パネル」 |
| 21 | e2e のサーバーには Store を 2 つ置く。`e2e`(書ける、色は空色 `sky`)と `readonly`(閲覧のみ、色は橙 `orange`)。どちらにも `apptest.Sample` のデータを入れる。e2e の 3 つの幅は 1 つのサーバーを共有するので、テストで作るタイトルには幅の名前を入れる(同じ名前のプロジェクトの断りを避ける) | 01「閲覧のみの Store」を e2e で確かめる。`playwright.config.ts` の `webServer` は 1 つ |
| 22 | C キーは、入力欄の中とダイアログを開いているときを除き、作成ダイアログを開く。初期値は、いま開いている画面(プロジェクト・ワークスペース)から決める。ツリーの行のフォーカスは A2 にないので、行に合わせた初期値は A4・E | 02「開き方と初期値」 |
| 23 | 作成ダイアログの見出しは「新規作成」(アイテム)と「プロジェクトを作成」(プロジェクト)、足元のボタンは「キャンセル」と「作成」 | 仕様に見出しの文言がない。モック版 19 の文言 |
| 24 | サイドバーは、A2 では、アプリの名前、「新規作成」のボタン、Store ごとのブロック(見出しのバッジ・ワークスペースの行・プロジェクトの行)、ワークスペースのメニューの「プロジェクトを作成」だけを出す。ナビ・「ID・検索・コマンド」・Store のメニュー・「ワークスペースを追加」・保存したビュー・同期カードは、行き先の画面を作るトラックが足す。ワークスペースの開閉は、そのブラウザのタブのあいだだけ覚える(このマシンに残すのはトラック G のマシンの設定が入ってから) | 04「サイドバー」。行き先のない項目を出さない |
| 25 | プロジェクトの見出しは「{HH-P1} {タイトル}」の 1 行の文字列で出す。タブの切り替えは「新しいタブで開く」の対象にしない(ui の `Tabs` がボタンだけのため。対象にするのは、タブを使うトラックが `Tabs` をリンクにするとき) | ui の `PageHead`・`Tabs` の形 |
| 26 | 概要の Epic が 0 件のときは、区画に何も出さない(文言は A4 の「概要の編集」で決める) | 仕様に 0 件の文言がない |
| 27 | プロジェクトの作成ダイアログでワークスペースを別の Store のものに替えたら、オーナーをその Store での「あなた」に戻す。オーナーの選択肢は「なし」とメンバーの表示名 | 02「作成」のオーナーの初めの値はあなた。メンバーは Store ごと |
| 28 | アイテムの作成ダイアログの親の候補は番号の順で、「{HH-1} {タイトル}」と出す。プロジェクトの候補はタイトルだけ。ワークスペースを替えた先にプロジェクトがないときは、プロジェクトを空のままにし、「作成」でサーバーの断り「プロジェクトを選んでください」を出す | 02「ダイアログの項目」に並びと表示の形がない。開くときにプロジェクトがない場合だけが 02 にある |
| 29 | 断り(`Refused`)以外の失敗(通信の失敗・500)は、A2 では画面に出さない。「サーバーに接続できない」の画面はトラック G | 08 |

---

## ファイルの地図

```
Go(共有部分 Task 1)
internal/app/port/storelist.go                    StoreList・StoreInfo
internal/app/apptest/storelist.go                 StoreList の偽物
internal/app/stores/list.go                       Store の一覧(Service)
internal/app/members/list.go                      メンバーの一覧と、あなた
internal/adapter/httpapi/stores/stores.go         list-stores
internal/adapter/httpapi/members/members.go       list-members・get-me
internal/adapter/httpapi/routes.go                (変更)Deps.StoreList と Register の 3 行
cmd/hedgehug/wire.go・wire_e2e.go                 (変更)StoreList を差し込む。e2e は Store を 2 つ

Go(アイテムの操作 Task 2・3)
internal/app/items/create.go                      (変更)作成の本体を Step の追加と共用する
internal/app/items/rename.go                      タイトルの変更
internal/app/items/addstep.go                     Step の追加
internal/app/items/moveup.go                      「上へ」
internal/app/items/numbers.go                     次の番号・番号で探す(読むだけ)
internal/domain/item/order.go                     兄弟の並び(SortSiblings)
internal/adapter/httpapi/items/items.go           (変更)Register を分ける
internal/adapter/httpapi/items/edit.go            rename-item・add-step・move-item-up
internal/adapter/httpapi/items/numbers.go         next-item-number・find-item-by-number

web(共有部分 Task 4・5)
web/packages/core/src/features/router/route.ts    Route・parseRoute・formatRoute・displayId の解釈
web/packages/core/src/features/router/navigation.ts  createNavigation(HistoryPort)
web/packages/core/src/port/history.ts             HistoryPort
web/packages/core/src/features/toasts/toasts.ts   createToasts(4 件まで・4.5 秒/7 秒)
web/packages/api/src/token.ts                     (変更)rememberToken・tokenForNewTab
web/app/src/history.ts                            HistoryPort の実装(History API)
web/app/src/links.ts                              リンクのクリック(新しいタブで開く)
web/packages/ui/src/primitives/Input.svelte       (変更)onblur・onkeydown・autofocus・id
web/packages/ui/src/primitives/Checkbox.svelte    チェックボックス
web/packages/ui/src/patterns/KindIcon.svelte      種類のアイコン
web/packages/ui/src/patterns/StoreBadge.svelte    Store のバッジ
web/packages/ui/src/patterns/StatusDot.svelte     分類の色の点
web/packages/ui/src/patterns/ReadOnlyBand.svelte  閲覧のみの帯
web/packages/ui/src/layout/SidePanel.svelte       右からせり出すパネル(全画面)

web(Task 6〜11)
web/packages/core/src/model/{store,workspace,project,item,member,errors}.ts
web/packages/core/src/port/{catalog,projects,items}.ts
web/packages/core/src/features/catalog/catalog.ts      一覧の状態
web/packages/core/src/features/router/resolve.ts       画面の ID → 内部 ID
web/packages/core/src/features/items/tree.ts           兄弟の並び・木
web/packages/core/src/features/projects/screen.ts      プロジェクトの画面の状態
web/packages/core/src/features/projects/createDialog.ts
web/packages/core/src/features/items/createDialog.ts
web/packages/core/src/features/items/createDefaults.ts 初期値の決め方
web/packages/core/src/features/items/detail.ts         詳細パネルの状態
web/packages/api/src/{catalog,projects,items,errors}.ts
web/app/src/wire.ts・App.svelte                         (変更)
web/app/src/shell/{Sidebar,SidebarStore,Screen}.svelte
web/app/src/screens/workspace/WorkspaceScreen.svelte
web/app/src/screens/project/{ProjectScreen,ProjectOverview,ProjectTree,TreeRow,CreateProjectDialog}.svelte
web/app/src/screens/item/{CreateItemDialog,CreateItemFields,DetailPanel,DetailHeader,DetailChildren}.svelte
web/e2e/project.spec.ts・readonly.spec.ts
```

## 型と口(タスクをまたいで使う名前)

各タスクの実装者は、自分のタスクしか読まない。ほかのタスクとの境目の名前と型は、ここに書いたものを正とする。

### Go

```go
// internal/app/port/storelist.go
type StoreInfo struct {
	ID       StoreID
	Name     string // Store 名(英数字・ハイフン・アンダースコア。08)
	Color    string // ui のトークン --t-<色> の名前(例 "sky")
	ReadOnly bool
}
type StoreList interface {
	// List は、表示中の Store を、サイドバーの並びで返す。
	List(ctx context.Context) ([]StoreInfo, error)
}

// internal/app/apptest/storelist.go
type StoreList struct{ Stores []port.StoreInfo }
func (l StoreList) List(context.Context) ([]port.StoreInfo, error)

// internal/app/stores/list.go
type Service struct{ Stores port.StoreList }
func (s Service) All(ctx context.Context) ([]port.StoreInfo, error)

// internal/app/members/list.go
type Service struct{ Stores port.Stores; Identity port.Identity }
func (s Service) List(ctx context.Context, store port.StoreID) ([]member.Member, error)
func (s Service) Me(ctx context.Context, store port.StoreID) (string, error)

// internal/adapter/httpapi/routes.go
type Deps struct {
	Version   string
	Stores    port.Stores
	IDs       port.IDs
	Identity  port.Identity
	StoreList port.StoreList
}

// internal/domain/item/order.go
// SortSiblings は、兄弟を並び順の値 → 作成の時刻 → 内部 ID の順に並べ替える(04「ツリー」)。
func SortSiblings(its []item.Item)

// internal/app/items
func (s Service) Rename(ctx context.Context, store port.StoreID, id item.ID, title string) (item.Item, error)
func (s Service) AddStep(ctx context.Context, store port.StoreID, parent item.ID, title string) (item.Item, error)
func (s Service) MoveUp(ctx context.Context, store port.StoreID, id item.ID) error
func (s Service) NextNumber(ctx context.Context, store port.StoreID, ws workspace.ID) (int, error)
func (s Service) FindByNumber(ctx context.Context, store port.StoreID, ws workspace.ID, number int) (item.Item, error)
```

API(すべて `Authorization: Bearer` が要る。誤りは `apierr.From`: 断り → 422 と `detail`、見つからない → 404):

| operationId | メソッドとパス | 本文 | 応答 |
|---|---|---|---|
| `list-stores` | `GET /api/stores` | — | `200 [{id, name, color, readOnly}]` |
| `list-workspaces`(B1 が作成済み) | `GET /api/stores/{store}/workspaces` | — | `200 [{id, name, prefix}]` |
| `list-members` | `GET /api/stores/{store}/members` | — | `200 [{login, name}]` |
| `get-me` | `GET /api/stores/{store}/me` | — | `200 {login}` |
| `rename-item` | `PUT /api/stores/{store}/items/{item}/title` | `{title}`(型名 `RenameItem`) | `200 Item` |
| `add-step` | `POST /api/stores/{store}/items/{item}/steps` | `{title}`(型名 `AddStep`) | `200 Item` |
| `move-item-up` | `POST /api/stores/{store}/items/{item}/move-up` | — | `204` |
| `next-item-number` | `GET /api/stores/{store}/workspaces/{workspace}/next-item-number` | — | `200 {number}`(型名 `NextNumber`) |
| `find-item-by-number` | `GET /api/stores/{store}/workspaces/{workspace}/items/by-number/{number}` | — | `200 Item` |

### TypeScript(core)

```ts
// model/errors.ts
export class Refused extends Error {}   // message が仕様の文言
export class NotFound extends Error {}

// model/store.ts
export interface StoreInfo { id: string; name: string; color: string; readOnly: boolean }
// model/workspace.ts
export interface Workspace { id: string; storeId: string; name: string; prefix: string }
// model/project.ts
export type ProjectStatus = "Todo" | "InProgress" | "Done" | "Pending" | "Cancel";
export interface Project {
  id: string; storeId: string; workspaceId: string; number: number; rank: string;
  title: string; owner: string; status: ProjectStatus; start: string; end: string;
}
export interface NewProject { title: string; owner: string; status: ProjectStatus; start: string; end: string }
// model/item.ts
export type Kind = "Epic" | "Story" | "Task" | "Bug" | "Step";
export interface Item {
  id: string; storeId: string; workspaceId: string; projectId: string; parentId: string | null;
  number: number; kind: Kind; rank: string; title: string; watchers: string[];
  createdAt: string; createdBy: string;
}
export interface NewItem { projectId: string; parentId: string | null; kind: Kind; title: string }
// model/member.ts
export interface Member { login: string; name: string }

// port/catalog.ts
export interface CatalogPort {
  listStores(): Promise<StoreInfo[]>;
  listWorkspaces(storeId: string): Promise<Workspace[]>;
  listProjects(storeId: string, workspaceId: string): Promise<Project[]>;
  listMembers(storeId: string): Promise<Member[]>;
  me(storeId: string): Promise<string>;
}
// port/projects.ts
export interface ProjectsPort {
  get(storeId: string, projectId: string): Promise<Project>;
  create(storeId: string, workspaceId: string, input: NewProject): Promise<Project>;
}
// port/items.ts
export interface ItemsPort {
  list(storeId: string, projectId: string): Promise<Item[]>;
  get(storeId: string, itemId: string): Promise<Item>;
  create(storeId: string, input: NewItem): Promise<Item>;
  rename(storeId: string, itemId: string, title: string): Promise<Item>;
  addStep(storeId: string, parentId: string, title: string): Promise<Item>;
  moveUp(storeId: string, itemId: string): Promise<void>;
  nextNumber(storeId: string, workspaceId: string): Promise<number>;
  findByNumber(storeId: string, workspaceId: string, number: number): Promise<Item>;
}
// port/history.ts
export interface HistoryPort {
  path(): string;
  push(path: string): void;
  replace(path: string): void;
  onPop(listener: () => void): () => void;
}

// features/router/route.ts
export type ProjectTab = "overview" | "tree";
export type Route =
  | { name: "home" }
  | { name: "workspace"; store: string; prefix: string }
  | { name: "project"; store: string; project: string; tab: ProjectTab } // project は "HH-P1"
  | { name: "item"; store: string; item: string };                       // item は "HH-12"
export function parseRoute(path: string): Route;
export function formatRoute(route: Route): string;
export type DisplayId = { prefix: string; kind: "project" | "item"; number: number };
export function parseDisplayId(s: string): DisplayId | null;
export function itemDisplayId(prefix: string, number: number): string;    // "HH-12"
export function projectDisplayId(prefix: string, number: number): string; // "HH-P1"

// features/router/navigation.ts
export interface Navigation {
  route: Readable<Route>;
  go(route: Route): void;       // 履歴に積む
  replace(route: Route): void;  // 履歴に積まない
  start(): () => void;          // 戻る・進むを受け始め、止める関数を返す
}
export function createNavigation(history: HistoryPort): Navigation;

// features/toasts/toasts.ts
export interface ToastItem { id: string; text: string; error: boolean }
export interface Toasts { list: Readable<ToastItem[]>; show(text: string, opts?: { error?: boolean }): void; dismiss(id: string): void }
export function createToasts(timer?: { set(fn: () => void, ms: number): unknown; clear(handle: unknown): void }): Toasts;

// features/catalog/catalog.ts
export interface ProjectNode extends Project { displayId: string }
export interface WorkspaceNode extends Workspace { projects: ProjectNode[] }
export interface StoreNode extends StoreInfo { workspaces: WorkspaceNode[] }
export interface Catalog {
  stores: Readable<StoreNode[]>;
  load(): Promise<void>;
  workspace(storeId: string, workspaceId: string): WorkspaceNode | undefined;
  project(storeId: string, projectId: string): ProjectNode | undefined;
  store(storeId: string): StoreNode | undefined;
}
export function createCatalog(port: CatalogPort): Catalog;
export function writableWorkspaces(stores: StoreNode[]): WorkspaceNode[];

// features/router/resolve.ts
export type Resolved =
  | { name: "home" }
  | { name: "workspace"; storeId: string; workspaceId: string }
  | { name: "project"; storeId: string; projectId: string; tab: ProjectTab }
  | { name: "item"; storeId: string; projectId: string; itemId: string };
export function resolveRoute(route: Route, stores: StoreNode[], items: ItemsPort): Promise<Resolved>;
export function routeOfProject(stores: StoreNode[], storeId: string, projectId: string, tab: ProjectTab): Route;
export function routeOfWorkspace(stores: StoreNode[], storeId: string, workspaceId: string): Route;

// features/items/tree.ts
export interface TreeNode { item: Item; children: TreeNode[] }
export function compareSiblings(a: Item, b: Item): number;
export function buildTree(items: Item[]): TreeNode[];   // 根は parentId が null(Epic)
export function ancestorsOf(items: Item[], id: string): Item[];  // 根から順
```

ダイアログと画面の状態(Task 8〜10)は、それぞれのタスクの Interfaces に書く。

---

## タスクの一覧

| # | タスク | 種類 | PR |
|---|---|---|---|
| 1 | 共有: 読み出しの口と API(Store・メンバー・あなた) | Go | `feat/track-a2-1` |
| 2 | アイテムの操作(タイトルの変更・Step の追加・「上へ」・次の番号・番号で探す) | Go | `feat/track-a2-2` |
| 3 | アイテムの操作の API | Go・生成物 | `feat/track-a2-3` |
| 4 | 共有: 画面のアドレスとトークン | web | `feat/track-a2-4` |
| 5 | 共有: ui の部品とトースト | web | `feat/track-a2-5` |
| 6 | 口の実装と一覧の状態 | web | `feat/track-a2-6` |
| 7 | サイドバーと画面の切り替え | web | `feat/track-a2-7` |
| 8 | プロジェクトの画面と作成ダイアログ | web | `feat/track-a2-8` |
| 9 | アイテムの作成ダイアログ | web | `feat/track-a2-9` |
| 10 | 詳細パネル | web | `feat/track-a2-10` |
| 11 | 3 つの幅の通しのテスト | e2e | `feat/track-a2-11` |

順: 1 → 2 → 3 は Go のライン、4 → 5 は画面の共有部分のライン。この 2 本は並べて進められる。6 は 1・3・4・5 のあと。7 → 8 → 9 → 10 → 11 は順に。

ブランチ: タスクごとに `origin/develop` から切り、PR の向き先は `develop`。前のタスクがまだ develop に入っていないときは、前のタスクのブランチから切り、PR の説明に「#N の上」と書く。force push は拒否されるので、載せ直すときは別名のブランチで出す。

**A2 の完成の条件:** `make check` と `cd web && pnpm check && pnpm test && pnpm e2e` が通る。3 つの幅の Playwright で、プロジェクトを作り、Epic・Task・Step を作り、詳細パネルでタイトルを変えられる。閲覧のみの Store では、作成のボタン・タイトルの入力欄・Step の入力欄が無効で、帯が出る。

---

### Task 1: 共有: 読み出しの口と API(Store・メンバー・あなた)

共有部分の PR。G(Store・メンバー)の持ち場に、画面が読むものだけを先に入れる。ワークスペースの一覧(`list-workspaces`、本文 `WorkspaceSummary{id, name, prefix}`)は B1(#149)で develop に入っているので、このタスクでは作らない(骨組みの決定 1 の「A2 が先に作る」は不要になった)。

**Files:**
- Create: `internal/app/port/storelist.go`、`internal/app/apptest/storelist.go`
- Create: `internal/app/stores/list.go`、`internal/app/members/list.go`
- Create: `internal/adapter/httpapi/stores/stores.go`、`internal/adapter/httpapi/members/members.go`
- Modify: `internal/adapter/httpapi/routes.go`(`Deps.StoreList` と Register の 2 行)
- Modify: `cmd/hedgehug/wire.go`(`noStores`)、`cmd/hedgehug/wire_e2e.go`(Store を 2 つ)
- Test: `internal/app/stores/list_test.go`、`internal/app/members/list_test.go`、`internal/adapter/httpapi/stores/stores_test.go`、`internal/adapter/httpapi/members/members_test.go`
- Modify(生成物): `api/openapi.yaml`、`web/packages/api/src/generated.ts`

**Interfaces:**
- Consumes: 土台の `port.Stores`・`port.Reader`(`Members().List`)、A1 Task 1 の `port.Identity`・`apptest.Identity`・`apierr.From`
- Produces:
  - `port.StoreInfo{ID StoreID; Name, Color string; ReadOnly bool}`、`port.StoreList{List(ctx) ([]StoreInfo, error)}`
  - `apptest.StoreList{Stores []port.StoreInfo}`(並べた順のまま返す)
  - `stores.Service{Stores port.StoreList}`、`(Service) All(ctx) ([]port.StoreInfo, error)`(骨組みの `List` フィールドは、メソッド名とぶつからないよう `Stores` にした)
  - `members.Service{Stores port.Stores; Identity port.Identity}`、`(Service) List(ctx, store) ([]member.Member, error)`(ログイン名の順)、`(Service) Me(ctx, store) (string, error)`
  - `httpapi.Deps.StoreList port.StoreList`(`Identity` の後ろに、コメント `// トラック A2(Store の一覧。本物はトラック G)` を付けて置く)
  - API: `list-stores`・`list-members`・`get-me`(本文の型名 `StoreInfo`・`Member`・`Me`)
  - e2e のサーバー: Store `01E2ESTORE0000000000000001`(名前 `e2e`、色 `sky`、書ける)と `01E2ESTORE0000000000000002`(名前 `readonly`、色 `orange`、閲覧のみ)。どちらにも `apptest.Sample`

- [ ] **Step 1: 失敗するテストを書く(app)**

`internal/app/stores/list_test.go`:

```go
package stores_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/stores"
)

func TestAllKeepsTheSidebarOrder(t *testing.T) {
	want := []port.StoreInfo{
		{ID: "S2", Name: "work", Color: "sky"},
		{ID: "S1", Name: "oss", Color: "orange", ReadOnly: true},
	}
	got, err := stores.Service{Stores: apptest.StoreList{Stores: want}}.All(t.Context())
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("All = %+v, %v", got, err)
	}
}
```

`internal/app/members/list_test.go`:

```go
package members_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/members"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/member"
)

func TestListAndMe(t *testing.T) {
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	svc := members.Service{Stores: s, Identity: apptest.Identity{Login: "hanako"}}
	got, err := svc.List(t.Context(), "S1")
	// member.FromLog はログイン名の順に返す。表示名の初期値はログイン名。
	want := []member.Member{{Login: "hanako", Name: "hanako"}, {Login: "taro", Name: "taro"}}
	if err != nil || len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("List = %+v, %v; want %+v", got, err, want)
	}
	if me, err := svc.Me(t.Context(), "S1"); err != nil || me != "hanako" {
		t.Errorf("Me = %q, %v", me, err)
	}
	if _, err := svc.List(t.Context(), "nope"); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("unknown store: %v", err)
	}
}
```

- [ ] **Step 2: 失敗することを確かめる**

Run: `go test ./internal/app/stores/ ./internal/app/members/`
Expected: FAIL(`undefined: stores.Service` などのビルドエラー)

- [ ] **Step 3: 口と app を書く**

`internal/app/port/storelist.go`:

```go
package port

import "context"

// StoreInfo は、このマシンに取り付けた Store の 1 つ。名前・色・閲覧のみの印は、マシンごとの設定(01)。
type StoreInfo struct {
	ID       StoreID
	Name     string // Store 名(英数字・ハイフン・アンダースコア。08)
	Color    string // 画面のトークン --t-<色> の名前(例 "sky")
	ReadOnly bool
}

// StoreList は、表示中の Store の一覧の口。本物の実装はトラック G が足す。
type StoreList interface {
	// List は、表示中の Store を、サイドバーの並びで返す。非表示の Store は返さない。
	List(ctx context.Context) ([]StoreInfo, error)
}
```

`internal/app/apptest/storelist.go`:

```go
package apptest

import (
	"context"
	"slices"

	"github.com/kukv/hedgehug/internal/app/port"
)

// StoreList は、Stores に並べた Store を、その順でそのまま返す偽物。
type StoreList struct {
	Stores []port.StoreInfo
}

func (l StoreList) List(context.Context) ([]port.StoreInfo, error) {
	return slices.Clone(l.Stores), nil
}
```

`internal/app/stores/list.go`:

```go
// Package stores は、このマシンの Store の一覧の操作を持つ。Store の追加・削除はトラック G が足す。
package stores

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
)

// Service は、Store の一覧の操作が使う口。
type Service struct {
	Stores port.StoreList
}

// All は、表示中の Store を、サイドバーの並びで返す。
func (s Service) All(ctx context.Context) ([]port.StoreInfo, error) {
	return s.Stores.List(ctx)
}
```

`internal/app/members/list.go`:

```go
// Package members は、Store のメンバーの操作を持つ。メンバーへの追加と表示名の変更はトラック G が足す。
package members

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/member"
)

// Service は、メンバーの操作が使う口。
type Service struct {
	Stores   port.Stores
	Identity port.Identity
}

// List は、store のメンバーを返す。
func (s Service) List(ctx context.Context, store port.StoreID) ([]member.Member, error) {
	var out []member.Member
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		out, err = r.Members().List()
		return err
	})
	return out, err
}

// Me は、store での「あなた」のログイン名を返す(01「「あなた」の決まり方」)。
func (s Service) Me(ctx context.Context, store port.StoreID) (string, error) {
	return s.Identity.Me(ctx, store)
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `go test -race ./internal/app/stores/ ./internal/app/members/`
Expected: PASS

- [ ] **Step 5: 失敗するテストを書く(API)**

`internal/adapter/httpapi/stores/stores_test.go`:

```go
package stores_test

import (
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/stores"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	appstores "github.com/kukv/hedgehug/internal/app/stores"
)

func TestListStores(t *testing.T) {
	_, api := humatest.New(t)
	stores.Register(api, appstores.Service{Stores: apptest.StoreList{Stores: []port.StoreInfo{
		{ID: "S1", Name: "e2e", Color: "sky"},
		{ID: "S2", Name: "readonly", Color: "orange", ReadOnly: true},
	}}})
	res := api.Get("/api/stores")
	want := `[{"id":"S1","name":"e2e","color":"sky","readOnly":false},{"id":"S2","name":"readonly","color":"orange","readOnly":true}]`
	if res.Code != http.StatusOK || res.Body.String() != want+"\n" {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
}

func TestListStoresEmptyIsAnArray(t *testing.T) {
	_, api := humatest.New(t)
	stores.Register(api, appstores.Service{Stores: apptest.StoreList{}})
	if res := api.Get("/api/stores"); res.Code != http.StatusOK || res.Body.String() != "[]\n" {
		t.Errorf("empty: %d %q", res.Code, res.Body)
	}
}
```

`internal/adapter/httpapi/members/members_test.go`:

```go
package members_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/members"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appmembers "github.com/kukv/hedgehug/internal/app/members"
	"github.com/kukv/hedgehug/internal/app/port"
)

func newStores(t *testing.T) *apptest.Stores {
	t.Helper()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMembersAndMe(t *testing.T) {
	_, api := humatest.New(t)
	members.Register(api, appmembers.Service{Stores: newStores(t), Identity: apptest.Identity{Login: "taro"}})
	res := api.Get("/api/stores/S1/members")
	if res.Code != http.StatusOK || res.Body.String() != `[{"login":"hanako","name":"hanako"},{"login":"taro","name":"taro"}]`+"\n" {
		t.Errorf("members: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/me"); res.Code != http.StatusOK || res.Body.String() != `{"login":"taro"}`+"\n" {
		t.Errorf("me: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/nope/members"); res.Code != http.StatusNotFound {
		t.Errorf("unknown store: %d", res.Code)
	}
}

type noIdentity struct{}

func (noIdentity) Me(context.Context, port.StoreID) (string, error) {
	return "", errors.New("このマシンのログイン名が未設定です")
}

// 「あなた」が決まらないときは 500。画面はオーナーの初めの値を「なし」にする(A2 の決定 3)。
func TestMeWithoutIdentityIs500(t *testing.T) {
	_, api := humatest.New(t)
	members.Register(api, appmembers.Service{Stores: newStores(t), Identity: noIdentity{}})
	if res := api.Get("/api/stores/S1/me"); res.Code != http.StatusInternalServerError {
		t.Errorf("me: %d %s", res.Code, res.Body)
	}
}
```

- [ ] **Step 6: 失敗することを確かめる**

Run: `go test ./internal/adapter/httpapi/stores/ ./internal/adapter/httpapi/members/`
Expected: FAIL(`undefined: stores.Register` などのビルドエラー)

- [ ] **Step 7: API を書き、登録する**

`internal/adapter/httpapi/stores/stores.go`:

```go
// Package stores は、このマシンの Store の一覧の API を持つ。
package stores

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appstores "github.com/kukv/hedgehug/internal/app/stores"
)

// StoreInfo は、API が返す Store。
type StoreInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color" doc:"画面のトークン --t-<色> の名前"`
	ReadOnly bool   `json:"readOnly"`
}

type listOutput struct{ Body []StoreInfo }

// Register は、Store の一覧の API を登録する。
func Register(api huma.API, svc appstores.Service) {
	huma.Register(api, huma.Operation{OperationID: "list-stores", Method: http.MethodGet, Path: "/api/stores", Summary: "表示中の Store をサイドバーの並びで返す"},
		func(ctx context.Context, _ *struct{}) (*listOutput, error) {
			list, err := svc.All(ctx)
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &listOutput{Body: []StoreInfo{}}
			for _, s := range list {
				out.Body = append(out.Body, StoreInfo{ID: string(s.ID), Name: s.Name, Color: s.Color, ReadOnly: s.ReadOnly})
			}
			return out, nil
		})
}
```

`internal/adapter/httpapi/members/members.go`:

```go
// Package members は、Store のメンバーと「あなた」の API を持つ。
package members

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appmembers "github.com/kukv/hedgehug/internal/app/members"
	"github.com/kukv/hedgehug/internal/app/port"
)

// Member は、API が返すメンバー。
type Member struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

// Me は、「あなた」のログイン名。
type Me struct {
	Login string `json:"login"`
}

type storePath struct {
	Store string `path:"store"`
}

type listOutput struct{ Body []Member }

type meOutput struct{ Body Me }

// Register は、メンバーの API を登録する。
func Register(api huma.API, svc appmembers.Service) {
	huma.Register(api, huma.Operation{OperationID: "list-members", Method: http.MethodGet, Path: "/api/stores/{store}/members", Summary: "Store のメンバーを返す"},
		func(ctx context.Context, in *storePath) (*listOutput, error) {
			list, err := svc.List(ctx, port.StoreID(in.Store))
			if err != nil {
				return nil, apierr.From(err)
			}
			out := &listOutput{Body: []Member{}}
			for _, m := range list {
				out.Body = append(out.Body, Member{Login: m.Login, Name: m.Name})
			}
			return out, nil
		})
	// 「あなた」が決まらない(本物の Identity がまだない)ときは、エラーをそのまま返し 500 になる。
	huma.Register(api, huma.Operation{OperationID: "get-me", Method: http.MethodGet, Path: "/api/stores/{store}/me", Summary: "Store での「あなた」のログイン名を返す"},
		func(ctx context.Context, in *storePath) (*meOutput, error) {
			login, err := svc.Me(ctx, port.StoreID(in.Store))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &meOutput{Body: Me{Login: login}}, nil
		})
}
```

`internal/adapter/httpapi/routes.go` の import に次の 4 つを足す:

```go
	"github.com/kukv/hedgehug/internal/adapter/httpapi/members"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/stores"
	appmembers "github.com/kukv/hedgehug/internal/app/members"
	appstores "github.com/kukv/hedgehug/internal/app/stores"
```

`Deps` の `Identity` の後ろ(トラック D の口の前)に `StoreList` を足す:

```go
	Identity port.Identity

	// トラック A2(Store の一覧。本物はトラック G)
	StoreList port.StoreList

	// トラック D(GitHub)
```

`NewAPI` の `system.Register(api, deps.Version)` の下に 2 行を足す:

```go
	stores.Register(api, appstores.Service{Stores: deps.StoreList})
	members.Register(api, appmembers.Service{Stores: deps.Stores, Identity: deps.Identity})
```

`cmd/hedgehug/wire.go` の `httpapi.Deps{…}` の `Identity: noIdentity{},` の次の行に `StoreList: noStores{},` を足し(`make fmt` で桁がそろう)、`noIdentity` の上に `noStores` を足す:

```go
// noStores は、Store の一覧(トラック G)が入るまでの仮。表示する Store はない。
type noStores struct{}

func (noStores) List(context.Context) ([]port.StoreInfo, error) { return []port.StoreInfo{}, nil }
```

`cmd/hedgehug/wire_e2e.go` の import に `"github.com/kukv/hedgehug/internal/app/port"` を足し、Store の ID の定数を次にする:

```go
// e2e 用のビルドの Store の ID。どちらにもサンプルデータを入れる。readonlyStore は閲覧のみの Store。
const (
	e2eStore      = "01E2ESTORE0000000000000001"
	readonlyStore = "01E2ESTORE0000000000000002"
)
```

`wire` の `stores.AddStore(e2eStore)` と `apptest.Sample` の呼び出しを、2 つの Store の繰り返しにする:

```go
	for _, id := range []port.StoreID{e2eStore, readonlyStore} {
		stores.AddStore(id)
		if err := apptest.Sample(ctx, stores, id); err != nil {
			return httpapi.Deps{}, err
		}
	}
```

返す `httpapi.Deps{…}` の `Identity` の次の行に `StoreList` を足す(Accounts・Secrets・GitHub・Clock はそのまま):

```go
		Identity: apptest.Identity{Login: "taro"},
		StoreList: apptest.StoreList{Stores: []port.StoreInfo{
			{ID: e2eStore, Name: "e2e", Color: "sky"},
			{ID: readonlyStore, Name: "readonly", Color: "orange", ReadOnly: true},
		}},
```

`wire_e2e.go` は e2e のビルドタグ付きなので `make fmt` が触らない。`gofmt -w cmd/hedgehug/wire_e2e.go` で整える。

閲覧のみの印は画面だけが使う。サーバーは、閲覧のみの Store への書き込みを断らない(送信の拒否はトラック C・G)。e2e は画面の無効化だけを確かめる。

- [ ] **Step 8: 通ることを確かめ、生成物を作り直す**

Run: `go test -race ./internal/adapter/... ./internal/app/...`
Expected: `internal/adapter/httpapi` の `TestOpenAPIYAMLIsUpToDate` だけが FAIL(`api/openapi.yaml is stale`)。ほかは PASS。

Run: `make generate && go test -race ./... && go build -tags e2e ./cmd/... && go vet -tags e2e ./cmd/... && cd web && pnpm gen && cd ..`
Expected: すべて PASS。`api/openapi.yaml` に `list-stores`・`list-members`・`get-me` が足され、`web/packages/api/src/generated.ts` に `listStores`・`listMembers`・`getMe` が足される。

- [ ] **Step 9: 全体を確かめる**

Run: `make check`
Expected: すべて成功(lint 0 件、archtest を含むテストが PASS)

- [ ] **Step 10: Commit**

```bash
git add internal/app/port/storelist.go internal/app/apptest/storelist.go internal/app/stores internal/app/members \
  internal/adapter/httpapi/stores internal/adapter/httpapi/members internal/adapter/httpapi/routes.go \
  cmd/hedgehug/wire.go cmd/hedgehug/wire_e2e.go api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat: 画面が読む Store・メンバー・あなたの口と API を足す(A2 Task 1)"
```

PR の説明に、`port.StoreList` の本物と `members` への追加はトラック G が足すことを書く。

---

### Task 2: アイテムの操作(タイトルの変更・Step の追加・「上へ」・次の番号・番号で探す)

**Files:**
- Create: `internal/domain/item/order.go`
- Modify: `internal/app/items/create.go`(作成の本体を `create` に分ける)、`internal/app/items/service.go`(`withID`)
- Create: `internal/app/items/rename.go`、`addstep.go`、`moveup.go`、`numbers.go`
- Test: `internal/domain/item/order_test.go`、`internal/app/items/edit_test.go`

**Interfaces:**
- Consumes: A1 の `items.Service`・`CreateInput`・`item.NormalizeTitle`・`item.CheckCreate`(Step の親の検査)・`numbering.Next`・`change.NewSet`
- Produces:
  - `item.SortSiblings(its []item.Item)`(並び順の値 → 作成の時刻 → 内部 ID)
  - `(Service) Rename(ctx, store, id item.ID, title string) (item.Item, error)`: 整えたあとが空なら `refusal`「タイトルは空にできません」。同じなら保存せず、いまのアイテムを返す。件名「{ID} のタイトルを変更」
  - `(Service) AddStep(ctx, store, parent item.ID, title string) (item.Item, error)`: 件名「{親の ID} に Step「{タイトル}」を追加」。親が Epic なら「Step は Epic の下に置けません」、空なら「タイトルを入力してください」
  - `(Service) MoveUp(ctx, store, id item.ID) error`: 件名「{ID} の順位を変更」。先頭なら保存しない
  - `(Service) NextNumber(ctx, store, ws workspace.ID) (int, error)`: ワークスペースがなければ `port.ErrNotFound`
  - `(Service) FindByNumber(ctx, store, ws workspace.ID, number int) (item.Item, error)`: なければ `port.ErrNotFound`
  - 見つからないアイテムは、どの操作も `port.ErrNotFound`

**決め方:** アクティビティの 1 行(`Operation.Summary`)は保存の前に決めるので、ID を書く操作は、まず `withID` でアイテムとワークスペースの接頭辞を読む。保存しない場合(同じタイトル、先頭の「上へ」)は、行のない `Write` にする(`port.Stores.Write` の約束で何も残らない)。「上へ」で直前の兄弟と並び順の値が同じとき(同期で重なったとき)は、入れ替えても並びが変わらないので保存しない(重なりの修復はトラック C)。F2a Task 12 で変更: `MoveUp` はトラック A4b で `items.Reorder` に置き換わり、順位が同じでも `rank.Around` で動かし、同じ順位の群の中には入らず群の外に置く(例: A(V)・B(W)・C(W) で A の「下へ移動」は B・C・A になる)。

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/item/order_test.go`:

```go
package item_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/item"
)

func TestSortSiblings(t *testing.T) {
	at := func(m int) item.Stamp { return item.Stamp{At: time.Date(2026, 11, 13, 9, m, 0, 0, time.UTC)} }
	its := []item.Item{
		{ID: "01C", Rank: "W", Created: at(0)},
		{ID: "01B", Rank: "V", Created: at(1)},
		{ID: "01D", Rank: "V", Created: at(0)},
		{ID: "01A", Rank: "V", Created: at(1)},
	}
	item.SortSiblings(its)
	// 並び順の値 V が先、V どうしは作成の早い 01D、同じ時刻の 01A と 01B は内部 ID の順、最後に W。
	want := []item.ID{"01D", "01A", "01B", "01C"}
	for i, id := range want {
		if its[i].ID != id {
			t.Fatalf("order = %v, want %v", its, want)
		}
	}
}
```

`internal/app/items/edit_test.go`(`items_test.go` の `store`・`message` を使う):

```go
package items_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// recorder は、Write に渡したアクティビティの 1 行を覚える。
type recorder struct {
	*apptest.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func setupRec(t *testing.T) (items.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return items.Service{Stores: rec, IDs: ids, Identity: apptest.Identity{Login: "hanako"}}, rec
}

// logLen は、アイテムのログの行数。何も保存しなかったことを確かめるのに使う。
func logLen(t *testing.T, s port.Stores, id item.ID) int {
	t.Helper()
	n := 0
	err := s.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: id})
		n = len(log)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func last(r *recorder) string {
	if len(r.summaries) == 0 {
		return ""
	}
	return r.summaries[len(r.summaries)-1]
}

func TestRename(t *testing.T) {
	svc, rec := setupRec(t)
	it, err := svc.Rename(t.Context(), store, apptest.SampleTask, "  ログイン画面を\n 直す ")
	if err != nil || it.Title != "ログイン画面を 直す" {
		t.Fatalf("Rename = %+v, %v", it, err)
	}
	if got := last(rec); got != "HH-2 のタイトルを変更" {
		t.Errorf("summary = %q", got)
	}
}

func TestRenameToTheSameTitleSavesNothing(t *testing.T) {
	svc, rec := setupRec(t)
	before := logLen(t, rec, apptest.SampleTask)
	it, err := svc.Rename(t.Context(), store, apptest.SampleTask, " ログイン画面を作る ")
	if err != nil || it.Title != "ログイン画面を作る" || logLen(t, rec, apptest.SampleTask) != before {
		t.Errorf("Rename = %+v, %v; log %d → %d", it, err, before, logLen(t, rec, apptest.SampleTask))
	}
}

func TestRenameRefusesAnEmptyTitle(t *testing.T) {
	svc, rec := setupRec(t)
	if _, err := svc.Rename(t.Context(), store, apptest.SampleTask, " \n "); message(err) != "タイトルは空にできません" {
		t.Errorf("err = %v", err)
	}
	if len(rec.summaries) != 0 {
		t.Errorf("a refused rename wrote: %v", rec.summaries)
	}
	if _, err := svc.Rename(t.Context(), store, "01NOPE00000000000000000000", "x"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

func TestAddStep(t *testing.T) {
	svc, rec := setupRec(t)
	step, err := svc.AddStep(t.Context(), store, apptest.SampleTask, " 下書き\n を書く ")
	if err != nil {
		t.Fatal(err)
	}
	if step.Kind != item.Step || step.Place.Parent == nil || *step.Place.Parent != apptest.SampleTask ||
		step.Place.Number != 3 || step.Title != "下書き を書く" || step.Place.Project != apptest.SampleProject {
		t.Errorf("step %+v", step)
	}
	if got := last(rec); got != "HH-2 に Step「下書き を書く」を追加" {
		t.Errorf("summary = %q", got)
	}
	second, err := svc.AddStep(t.Context(), store, apptest.SampleTask, "見直す")
	if err != nil || second.Rank <= step.Rank {
		t.Errorf("second step %+v, %v; must come after %q", second, err, step.Rank)
	}
}

func TestAddStepRefuses(t *testing.T) {
	svc, _ := setupRec(t)
	if _, err := svc.AddStep(t.Context(), store, apptest.SampleEpic, "下書き"); message(err) != "Step は Epic の下に置けません" {
		t.Errorf("under epic: %v", err)
	}
	if _, err := svc.AddStep(t.Context(), store, apptest.SampleTask, " "); message(err) != "タイトルを入力してください" {
		t.Errorf("empty: %v", err)
	}
	if _, err := svc.AddStep(t.Context(), store, "01NOPE00000000000000000000", "x"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("unknown parent: %v", err)
	}
}

func TestMoveUpSwapsWithThePreviousSibling(t *testing.T) {
	svc, rec := setupRec(t)
	a, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "1")
	b, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "2")
	if err := svc.MoveUp(t.Context(), store, b.ID); err != nil {
		t.Fatal(err)
	}
	if got := last(rec); got != "HH-4 の順位を変更" {
		t.Errorf("summary = %q", got)
	}
	a2, _ := svc.Get(t.Context(), store, a.ID)
	b2, _ := svc.Get(t.Context(), store, b.ID)
	if b2.Rank != a.Rank || a2.Rank != b.Rank {
		t.Errorf("ranks: a %q→%q, b %q→%q", a.Rank, a2.Rank, b.Rank, b2.Rank)
	}
}

func TestMoveUpAtTheTopSavesNothing(t *testing.T) {
	svc, rec := setupRec(t)
	a, _ := svc.AddStep(t.Context(), store, apptest.SampleTask, "1")
	before := logLen(t, rec, a.ID)
	if err := svc.MoveUp(t.Context(), store, a.ID); err != nil || logLen(t, rec, a.ID) != before {
		t.Errorf("MoveUp = %v; log %d → %d", err, before, logLen(t, rec, a.ID))
	}
	// Epic はプロジェクトの直下の兄弟の中で動く。サンプルの Epic は 1 つだけなので先頭。
	if err := svc.MoveUp(t.Context(), store, apptest.SampleEpic); err != nil {
		t.Errorf("epic: %v", err)
	}
}

func TestNextNumberAndFindByNumber(t *testing.T) {
	svc, _ := setupRec(t)
	if n, err := svc.NextNumber(t.Context(), store, apptest.SampleWorkspace); err != nil || n != 3 {
		t.Errorf("NextNumber = %d, %v", n, err)
	}
	it, err := svc.FindByNumber(t.Context(), store, apptest.SampleWorkspace, 2)
	if err != nil || it.ID != apptest.SampleTask {
		t.Errorf("FindByNumber(2) = %+v, %v", it, err)
	}
	if _, err := svc.FindByNumber(t.Context(), store, apptest.SampleWorkspace, 9); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("FindByNumber(9): %v", err)
	}
	if _, err := svc.NextNumber(t.Context(), store, "01NOPE00000000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("NextNumber(unknown): %v", err)
	}
}
```

- [ ] **Step 2: 失敗することを確かめる**

Run: `go test ./internal/domain/item/ ./internal/app/items/`
Expected: FAIL(`undefined: item.SortSiblings`、`svc.Rename undefined` などのビルドエラー)

- [ ] **Step 3: 実装する**

`internal/domain/item/order.go`:

```go
package item

import (
	"cmp"
	"slices"
	"strings"
)

// SortSiblings は、兄弟を並び順の値 → 作成の時刻 → 内部 ID の順に並べ替える(04「ツリー」の「行」)。
// 並び順の値が同じものは、作成の保存が早い方を先にし、同じなら内部 ID の辞書順にする。
func SortSiblings(its []Item) {
	slices.SortFunc(its, func(a, b Item) int {
		return cmp.Or(
			strings.Compare(a.Rank, b.Rank),
			a.Created.At.Compare(b.Created.At),
			strings.Compare(string(a.ID), string(b.ID)),
		)
	})
}
```

`internal/app/items/create.go` の `Create` の冒頭を、作成と Step の追加が共用する `create` に分ける(`Create` の残りの本文はそのまま `create` の本文になる):

```go
// Create は、アイテムを作る(02「作成したあと」)。
func (s Service) Create(ctx context.Context, in CreateInput) (item.Item, error) {
	return s.create(ctx, in, fmt.Sprintf("%s「%s」を作成", in.Kind, item.NormalizeTitle(in.Title)))
}

// create は、作成と Step の追加が共用する本体。summary はアクティビティの 1 行。
func (s Service) create(ctx context.Context, in CreateInput, summary string) (item.Item, error) {
	me, err := s.Identity.Me(ctx, in.Store)
	if err != nil {
		return item.Item{}, err
	}
	title := item.NormalizeTitle(in.Title)
	op := port.Operation{Actor: change.User(me), Summary: summary}
	var id item.ID
	// ここから下は、もとの Create の本文のまま(s.Stores.Write … return s.Get(ctx, in.Store, id))
```

`internal/app/items/service.go` を次にする:

```go
// Package items は、アイテムの操作(02)を持つ。1 ファイル 1 操作。
package items

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// Service は、アイテムの操作が使う口。
type Service struct {
	Stores   port.Stores
	IDs      port.IDs
	Identity port.Identity
}

// withID は、アイテムと、その画面の ID(HH-12)を返す。アクティビティの 1 行に ID を書く操作が使う。
// アクティビティの 1 行は保存の前に決めるので、保存とは別に読む。
func (s Service) withID(ctx context.Context, store port.StoreID, id item.ID) (item.Item, string, error) {
	var it item.Item
	var display string
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		var err error
		if it, err = r.Items().Get(id); err != nil {
			return err
		}
		w, err := r.Workspaces().Get(it.Place.Workspace)
		if err != nil {
			return err
		}
		display = it.DisplayID(w.Prefix)
		return nil
	})
	return it, display, err
}
```

`internal/app/items/rename.go`:

```go
package items

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// Rename は、タイトルを変える(02「タイトル」)。整えたあとが空なら断り、前と同じなら何も保存しない。
// どちらでもなければ 1 回の保存で、アクティビティは「{ID} のタイトルを変更」。変えたあとのアイテムを返す。
func (s Service) Rename(ctx context.Context, store port.StoreID, id item.ID, title string) (item.Item, error) {
	title = item.NormalizeTitle(title)
	if title == "" {
		return item.Item{}, refusal.New("タイトルは空にできません")
	}
	_, display, err := s.withID(ctx, store, id)
	if err != nil {
		return item.Item{}, err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return item.Item{}, err
	}
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("%s のタイトルを変更", display)}
	err = s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		it, err := tx.Items().Get(id)
		if err != nil {
			return err
		}
		if it.Title == title {
			return nil // 行のない Write は何も残さない
		}
		return tx.WriteItems().Append(id, change.NewSet("title", it.Title, title))
	})
	if err != nil {
		return item.Item{}, err
	}
	return s.Get(ctx, store, id)
}
```

`internal/app/items/addstep.go`:

```go
package items

import (
	"context"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// AddStep は、parent の子の末尾に Step を作る(02「子と Step」の「Step の追加」)。作り方は作成と同じで、
// アクティビティだけ「{親の ID} に Step「{タイトル}」を追加」にする。担当者・ステータス・優先度は A3 で足す。
// 親が Story・Task・Bug でなければ、作成と同じ文言で断る。
func (s Service) AddStep(ctx context.Context, store port.StoreID, parent item.ID, title string) (item.Item, error) {
	p, display, err := s.withID(ctx, store, parent)
	if err != nil {
		return item.Item{}, err
	}
	in := CreateInput{Store: store, Project: p.Place.Project, Parent: &parent, Kind: item.Step, Title: title}
	return s.create(ctx, in, fmt.Sprintf("%s に Step「%s」を追加", display, item.NormalizeTitle(title)))
}
```

`internal/app/items/moveup.go`:

```go
package items

import (
	"context"
	"errors"
	"fmt"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// MoveUp は、同じ親の兄弟の中で、直前の兄弟と並び順の値を入れ替える(02「子と Step」の「上へ」、04 の
// 「上へ移動」)。先頭なら何も保存しない。アクティビティは「{ID} の順位を変更」。
// 直前の兄弟と並び順の値が同じとき(同期で重なったとき)は、入れ替えても並びが変わらないので何も保存しない。
// 重なりの修復はトラック C。
func (s Service) MoveUp(ctx context.Context, store port.StoreID, id item.ID) error {
	_, display, err := s.withID(ctx, store, id)
	if err != nil {
		return err
	}
	me, err := s.Identity.Me(ctx, store)
	if err != nil {
		return err
	}
	op := port.Operation{Actor: change.User(me), Summary: fmt.Sprintf("%s の順位を変更", display)}
	return s.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		it, err := tx.Items().Get(id)
		if err != nil {
			return err
		}
		all, err := tx.Items().List(it.Place.Project)
		if err != nil {
			return err
		}
		sib := siblings(all, it.Place.Parent)
		item.SortSiblings(sib)
		i := indexOf(sib, id)
		if i <= 0 || sib[i-1].Rank == it.Rank {
			return nil // 行のない Write は何も残さない
		}
		prev := sib[i-1]
		return errors.Join(
			tx.WriteItems().Append(it.ID, change.NewSet("rank", it.Rank, prev.Rank)),
			tx.WriteItems().Append(prev.ID, change.NewSet("rank", prev.Rank, it.Rank)),
		)
	})
}

// siblings は、parent の子(parent が nil ならプロジェクトの直下)を返す。
func siblings(all []item.Item, parent *item.ID) []item.Item {
	var out []item.Item
	for _, it := range all {
		if (parent == nil && it.Place.Parent == nil) ||
			(parent != nil && it.Place.Parent != nil && *it.Place.Parent == *parent) {
			out = append(out, it)
		}
	}
	return out
}

func indexOf(its []item.Item, id item.ID) int {
	for i, it := range its {
		if it.ID == id {
			return i
		}
	}
	return -1
}
```

`internal/app/items/numbers.go`(読むだけの 2 つの操作を 1 ファイルに置く。設計書 4 節の「1 ファイル 1 操作」は保存する操作に当てる):

```go
package items

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// NextNumber は、ws で次に作るアイテムの番号を返す(作成ダイアログの ID の予告。02)。目安で、実際の番号は
// 保存のときに決まる。ワークスペースがなければ port.ErrNotFound。
func (s Service) NextNumber(ctx context.Context, store port.StoreID, ws workspace.ID) (int, error) {
	n := 0
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		if _, err := r.Workspaces().Get(ws); err != nil {
			return err
		}
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
		if err != nil {
			return err
		}
		n = numbering.Next(log, numbering.Items)
		return nil
	})
	return n, err
}

// FindByNumber は、ws の番号 number のアイテムを返す(画面のアドレスの HH-12 の解決。01)。
// なければ port.ErrNotFound。旧い接頭辞と転送の記録はたどらない(B1・A5)。
func (s Service) FindByNumber(ctx context.Context, store port.StoreID, ws workspace.ID, number int) (item.Item, error) {
	var found item.Item
	err := s.Stores.Read(ctx, store, func(r port.Reader) error {
		ps, err := r.Projects().List(ws)
		if err != nil {
			return err
		}
		for _, p := range ps {
			its, err := r.Items().List(p.ID)
			if err != nil {
				return err
			}
			for _, it := range its {
				if it.Place.Workspace == ws && it.Place.Number == number {
					found = it
					return nil
				}
			}
		}
		return port.ErrNotFound
	})
	return found, err
}
```

`FindByNumber` はワークスペースのすべてのアイテムを読む。件数が増えて遅くなったら、番号の索引を口に足す(A1 の見送り「一覧の性能」と同じ扱い)。

- [ ] **Step 4: 通ることを確かめる**

Run: `go test -race ./internal/domain/item/ ./internal/app/items/ && make lint`
Expected: PASS、lint 0 件

- [ ] **Step 5: Commit**

```bash
git add internal/domain/item/order.go internal/domain/item/order_test.go internal/app/items
git commit -m "feat(app): アイテムのタイトルの変更・Step の追加・上へ・次の番号・番号で探すを足す(A2 Task 2)"
```

---

### Task 3: アイテムの操作の API

**Files:**
- Modify: `internal/adapter/httpapi/items/items.go`(`Register` を 3 つに分ける)
- Create: `internal/adapter/httpapi/items/edit.go`、`internal/adapter/httpapi/items/numbers.go`
- Test: `internal/adapter/httpapi/items/edit_test.go`
- Modify(生成物): `api/openapi.yaml`、`web/packages/api/src/generated.ts`

**Interfaces:**
- Consumes: Task 2 の `(appitems.Service) Rename`・`AddStep`・`MoveUp`・`NextNumber`・`FindByNumber`、A1 の `items.Item`・`toAPI`・`oneOutput`・`getInput`、`apierr.From`
- Produces(すべて `Authorization: Bearer` が要る。断り → 422 と `detail`、見つからない → 404):
  - `rename-item` `PUT /api/stores/{store}/items/{item}/title` 本文 `RenameItem{title}` → `200 Item`
  - `add-step` `POST /api/stores/{store}/items/{item}/steps` 本文 `AddStep{title}` → `200 Item`
  - `move-item-up` `POST /api/stores/{store}/items/{item}/move-up` → `204`
  - `next-item-number` `GET /api/stores/{store}/workspaces/{workspace}/next-item-number` → `200 NextNumber{number}`
  - `find-item-by-number` `GET /api/stores/{store}/workspaces/{workspace}/items/by-number/{number}` → `200 Item`(`number` は 1 以上。0 以下は 422)
  - 生成されるクライアントの関数: `renameItem`・`addStep`・`moveItemUp`・`nextItemNumber`・`findItemByNumber`(Task 6 が使う)

- [ ] **Step 1: 失敗するテストを書く**

`internal/adapter/httpapi/items/edit_test.go`(`items_test.go` の `newAPI` を使う):

```go
package items_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
)

func TestRenameItem(t *testing.T) {
	api := newAPI(t)
	res := api.Put("/api/stores/S1/items/"+string(apptest.SampleTask)+"/title", map[string]any{"title": " 画面を\n直す "})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"title":"画面を 直す"`) {
		t.Errorf("rename: %d %s", res.Code, res.Body)
	}
	res = api.Put("/api/stores/S1/items/"+string(apptest.SampleTask)+"/title", map[string]any{"title": " "})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "タイトルは空にできません") {
		t.Errorf("empty: %d %s", res.Code, res.Body)
	}
	if res := api.Put("/api/stores/S1/items/01NOPE00000000000000000000/title", map[string]any{"title": "x"}); res.Code != http.StatusNotFound {
		t.Errorf("unknown: %d", res.Code)
	}
}

func TestAddStepAndMoveUp(t *testing.T) {
	api := newAPI(t)
	res := api.Post("/api/stores/S1/items/"+string(apptest.SampleTask)+"/steps", map[string]any{"title": "下書き"})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"kind":"Step"`) || !strings.Contains(res.Body.String(), `"number":3`) {
		t.Fatalf("add step: %d %s", res.Code, res.Body)
	}
	res = api.Post("/api/stores/S1/items/"+string(apptest.SampleEpic)+"/steps", map[string]any{"title": "下書き"})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "Step は Epic の下に置けません") {
		t.Errorf("under epic: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/" + string(apptest.SampleTask) + "/move-up"); res.Code != http.StatusNoContent {
		t.Errorf("move up: %d %s", res.Code, res.Body)
	}
	if res := api.Post("/api/stores/S1/items/01NOPE00000000000000000000/move-up"); res.Code != http.StatusNotFound {
		t.Errorf("move up unknown: %d", res.Code)
	}
}

func TestNumbers(t *testing.T) {
	api := newAPI(t)
	ws := "/api/stores/S1/workspaces/" + string(apptest.SampleWorkspace)
	if res := api.Get(ws + "/next-item-number"); res.Code != http.StatusOK || res.Body.String() != `{"number":3}`+"\n" {
		t.Errorf("next: %d %s", res.Code, res.Body)
	}
	if res := api.Get(ws + "/items/by-number/2"); res.Code != http.StatusOK || !strings.Contains(res.Body.String(), string(apptest.SampleTask)) {
		t.Errorf("by number: %d %s", res.Code, res.Body)
	}
	if res := api.Get(ws + "/items/by-number/9"); res.Code != http.StatusNotFound {
		t.Errorf("by number 9: %d", res.Code)
	}
	if res := api.Get("/api/stores/S1/workspaces/01NOPE00000000000000000000/next-item-number"); res.Code != http.StatusNotFound {
		t.Errorf("next unknown ws: %d", res.Code)
	}
}
```

本文のない POST は `api.Post(path)` と書く(`nil` を渡すと humatest が panic する)。

- [ ] **Step 2: 失敗することを確かめる**

Run: `go test ./internal/adapter/httpapi/items/`
Expected: FAIL(`rename: 404`、`add step: 404` などのルートがない誤り)

- [ ] **Step 3: 実装する**

`internal/adapter/httpapi/items/items.go` の `Register` を、3 つの登録を呼ぶだけにし、もとの本文を `registerCreateRead` に移す:

```go
// Register は、アイテムの API を登録する。
func Register(api huma.API, svc appitems.Service) {
	registerCreateRead(api, svc)
	registerEdit(api, svc)
	registerNumbers(api, svc)
}

func registerCreateRead(api huma.API, svc appitems.Service) {
	// ここから下は、もとの Register の本文のまま(create-item・get-item・list-project-items)
```

`internal/adapter/httpapi/items/edit.go`:

```go
package items

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// RenameItem は、タイトルの変更の本文。
type RenameItem struct {
	Title string `json:"title"`
}

// AddStep は、Step の追加の本文。
type AddStep struct {
	Title string `json:"title"`
}

type renameInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  RenameItem
}

type addStepInput struct {
	Store string `path:"store"`
	Item  string `path:"item"`
	Body  AddStep
}

func registerEdit(api huma.API, svc appitems.Service) {
	huma.Register(api, huma.Operation{OperationID: "rename-item", Method: http.MethodPut, Path: "/api/stores/{store}/items/{item}/title", Summary: "アイテムのタイトルを変える"},
		func(ctx context.Context, in *renameInput) (*oneOutput, error) {
			it, err := svc.Rename(ctx, port.StoreID(in.Store), item.ID(in.Item), in.Body.Title)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &oneOutput{Body: toAPI(it)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "add-step", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/steps", Summary: "アイテムの子の末尾に Step を作る"},
		func(ctx context.Context, in *addStepInput) (*oneOutput, error) {
			it, err := svc.AddStep(ctx, port.StoreID(in.Store), item.ID(in.Item), in.Body.Title)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &oneOutput{Body: toAPI(it)}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "move-item-up", Method: http.MethodPost, Path: "/api/stores/{store}/items/{item}/move-up", Summary: "アイテムを直前の兄弟と入れ替える", DefaultStatus: http.StatusNoContent},
		func(ctx context.Context, in *getInput) (*struct{}, error) {
			if err := svc.MoveUp(ctx, port.StoreID(in.Store), item.ID(in.Item)); err != nil {
				return nil, apierr.From(err)
			}
			return nil, nil //nolint:nilnil // 204 は本文なし
		})
}
```

`internal/adapter/httpapi/items/numbers.go`:

```go
package items

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/apierr"
	appitems "github.com/kukv/hedgehug/internal/app/items"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// NextNumber は、次に作るアイテムの番号(ID の予告の目安)。
type NextNumber struct {
	Number int `json:"number"`
}

type wsInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
}

type byNumberInput struct {
	Store     string `path:"store"`
	Workspace string `path:"workspace"`
	Number    int    `path:"number" minimum:"1"`
}

type nextOutput struct{ Body NextNumber }

func registerNumbers(api huma.API, svc appitems.Service) {
	huma.Register(api, huma.Operation{OperationID: "next-item-number", Method: http.MethodGet, Path: "/api/stores/{store}/workspaces/{workspace}/next-item-number", Summary: "ワークスペースで次に作るアイテムの番号を返す"},
		func(ctx context.Context, in *wsInput) (*nextOutput, error) {
			n, err := svc.NextNumber(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace))
			if err != nil {
				return nil, apierr.From(err)
			}
			return &nextOutput{Body: NextNumber{Number: n}}, nil
		})
	huma.Register(api, huma.Operation{OperationID: "find-item-by-number", Method: http.MethodGet, Path: "/api/stores/{store}/workspaces/{workspace}/items/by-number/{number}", Summary: "ワークスペースの番号でアイテムを返す"},
		func(ctx context.Context, in *byNumberInput) (*oneOutput, error) {
			it, err := svc.FindByNumber(ctx, port.StoreID(in.Store), workspace.ID(in.Workspace), in.Number)
			if err != nil {
				return nil, apierr.From(err)
			}
			return &oneOutput{Body: toAPI(it)}, nil
		})
}
```

- [ ] **Step 4: 通ることを確かめ、生成物を作り直す**

Run: `go test -race ./internal/adapter/httpapi/items/`
Expected: PASS

Run: `make generate && go test -race ./internal/... && cd web && pnpm gen && cd ..`
Expected: PASS。`api/openapi.yaml` に 5 つの operationId(`rename-item`・`add-step`・`move-item-up`・`next-item-number`・`find-item-by-number`)が足され、`move-item-up` の応答は `"204"`。`web/packages/api/src/generated.ts` に `renameItem`・`addStep`・`moveItemUp`・`nextItemNumber`・`findItemByNumber` が足される。

- [ ] **Step 5: 全体を確かめる**

Run: `make check`
Expected: すべて成功(lint 0 件。書式が崩れていたら `make fmt` で直す)

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/httpapi/items api/openapi.yaml web/packages/api/src/generated.ts
git commit -m "feat(adapter): アイテムのタイトルの変更・Step の追加・上へ・番号の API を足す(A2 Task 3)"
```

---

### Task 4: 共有: 画面のアドレスとトークン

**Files:**
- Create: `web/packages/core/src/port/history.ts`
- Create: `web/packages/core/src/features/router/route.ts`・`route.test.ts`
- Create: `web/packages/core/src/features/router/navigation.ts`・`navigation.test.ts`
- Modify: `web/packages/core/src/index.ts`
- Modify: `web/packages/api/src/token.ts`・`token.test.ts`・`index.ts`
- Create: `web/app/src/history.ts`・`web/app/src/links.ts`
- Modify: `web/app/src/wire.ts`・`web/app/src/main.ts`
- Modify: `web/e2e/frame.spec.ts`

**Interfaces:**
- Consumes: 土台の `core/reactive/store`(`writable`・`readonly`・`get`)、`api/fetcher` の `setToken`、`api/token` の `tokenFromHash`。
- Produces(Task 6〜11 が使う):
  - core: 「型と口」の `Route`・`ProjectTab`・`parseRoute`・`formatRoute`・`DisplayId`・`parseDisplayId`・`itemDisplayId`・`projectDisplayId`・`Navigation`・`createNavigation(history)`・`HistoryPort`。`navigation.go` は、いまのアドレスと同じなら何もしない。
  - api: `rememberToken(location, storage, replace): string`、`newTabUrl(path, token): string`、`tokenKey`、`TokenStorage`。
  - app: `wire.ts` の `navigation`(`start()` 済み)と `links`。`links.href(route): string`、`links.click(event: MouseEvent, route): void`(ふつうのクリックは `navigation.go`、Ctrl・Cmd・Shift+クリックと中クリックは `#token=` を付けて新しいタブ、右クリックはブラウザに任せる)。リンクは `<a href={links.href(r)} onclick={(e) => links.click(e, r)} onauxclick={(e) => links.click(e, r)}>` と書く。
- 共有部分の PR(設計書 8 節)。ほかのトラックの画面も、このアドレスとリンクを使う。

- [ ] **Step 1: 失敗するテストを書く**

`web/packages/core/src/features/router/route.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import {
  formatRoute,
  itemDisplayId,
  parseDisplayId,
  parseRoute,
  projectDisplayId,
  type Route,
} from "./route";

describe("parseRoute and formatRoute", () => {
  const cases: [string, Route][] = [
    ["/", { name: "home" }],
    ["/s/work/w/HH", { name: "workspace", store: "work", prefix: "HH" }],
    [
      "/s/work/p/HH-P1/overview",
      { name: "project", store: "work", project: "HH-P1", tab: "overview" },
    ],
    ["/s/work/p/HH-P1/tree", { name: "project", store: "work", project: "HH-P1", tab: "tree" }],
    ["/s/work/i/HH-12", { name: "item", store: "work", item: "HH-12" }],
  ];

  it.each(cases)("reads %s and writes it back", (path, route) => {
    expect(parseRoute(path)).toEqual(route);
    expect(formatRoute(route)).toBe(path);
  });

  it("opens the overview when the project address has no tab", () => {
    expect(parseRoute("/s/work/p/HH-P1")).toEqual({
      name: "project",
      store: "work",
      project: "HH-P1",
      tab: "overview",
    });
  });

  it("encodes and decodes each part", () => {
    const route: Route = { name: "item", store: "a b", item: "x/y" };
    expect(formatRoute(route)).toBe("/s/a%20b/i/x%2Fy");
    expect(parseRoute("/s/a%20b/i/x%2Fy")).toEqual(route);
  });

  it.each([
    "/nope",
    "/s/work",
    "/s/work/x/HH-1",
    "/s/work/p/HH-P1/kanban",
    "/s/work/i/HH-1/more",
    "/s//i/HH-1",
    "/s/work/i/%E0%A4%A",
  ])("reads %s as home", (path) => {
    expect(parseRoute(path)).toEqual({ name: "home" });
  });
});

describe("display IDs", () => {
  it("reads item and project IDs", () => {
    expect(parseDisplayId("HH-12")).toEqual({ prefix: "HH", kind: "item", number: 12 });
    expect(parseDisplayId("WEB2-P3")).toEqual({ prefix: "WEB2", kind: "project", number: 3 });
  });

  it.each(["", "HH", "hh-1", "HH-0", "HH-P", "1H-1", "ABCDEFGHIJK-1", "HH-01", "HH-1x"])(
    "rejects %s",
    (s) => {
      expect(parseDisplayId(s)).toBeNull();
    },
  );

  it("writes item and project IDs", () => {
    expect(itemDisplayId("HH", 12)).toBe("HH-12");
    expect(projectDisplayId("HH", 1)).toBe("HH-P1");
  });
});
```

`web/packages/core/src/features/router/navigation.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { HistoryPort } from "../../port/history";
import { get } from "../../reactive/store";
import { createNavigation } from "./navigation";

// fakeHistory は、積んだパスを持つ履歴の偽物。back は、ブラウザの戻るを押したときの動き。
function fakeHistory(start: string) {
  const stack = [start];
  let index = 0;
  const listeners = new Set<() => void>();
  const history: HistoryPort = {
    path: () => stack[index] ?? "/",
    push(path) {
      stack.splice(index + 1, stack.length, path);
      index = stack.length - 1;
    },
    replace(path) {
      stack[index] = path;
    },
    onPop(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
  const back = () => {
    index -= 1;
    for (const l of listeners) l();
  };
  return { history, stack, back };
}

describe("navigation", () => {
  it("starts at the route of the current address", () => {
    const nav = createNavigation(fakeHistory("/s/work/i/HH-3").history);
    expect(get(nav.route)).toEqual({ name: "item", store: "work", item: "HH-3" });
  });

  it("pushes on go, replaces on replace, and follows the back button", () => {
    const { history, stack, back } = fakeHistory("/");
    const nav = createNavigation(history);
    const stop = nav.start();
    nav.go({ name: "project", store: "work", project: "HH-P1", tab: "overview" });
    nav.replace({ name: "project", store: "work", project: "HH-P1", tab: "tree" });
    expect(stack).toEqual(["/", "/s/work/p/HH-P1/tree"]);
    expect(get(nav.route)).toMatchObject({ name: "project", tab: "tree" });
    back();
    expect(get(nav.route)).toEqual({ name: "home" });
    stop();
  });

  it("does not push the address that is already open", () => {
    const { history, stack } = fakeHistory("/");
    const nav = createNavigation(history);
    nav.go({ name: "workspace", store: "work", prefix: "HH" });
    nav.go({ name: "workspace", store: "work", prefix: "HH" });
    expect(stack).toEqual(["/", "/s/work/w/HH"]);
  });

  it("stops following the back button", () => {
    const { history, back } = fakeHistory("/");
    const nav = createNavigation(history);
    nav.go({ name: "workspace", store: "work", prefix: "HH" });
    nav.start()();
    back();
    expect(get(nav.route)).toEqual({ name: "workspace", store: "work", prefix: "HH" });
  });
});
```

`web/packages/api/src/token.test.ts` を次の内容に置き換える(既存の `tokenFromHash` のテストは残す):

`web/packages/api/src/token.test.ts`:

```ts
import { describe, expect, it, vi } from "vitest";
import { newTabUrl, rememberToken, tokenFromHash, tokenKey } from "./token";

describe("tokenFromHash", () => {
  it("reads the token from the hash that the server printed", () => {
    expect(tokenFromHash("#token=abc123")).toBe("abc123");
  });

  it("returns null when the URL has no token", () => {
    expect(tokenFromHash("")).toBeNull();
    expect(tokenFromHash("#foo=1")).toBeNull();
    expect(tokenFromHash("#token=")).toBeNull();
  });
});

function memoryStorage() {
  const map = new Map<string, string>();
  return {
    map,
    getItem: (key: string) => map.get(key) ?? null,
    setItem: (key: string, value: string) => {
      map.set(key, value);
    },
  };
}

describe("rememberToken", () => {
  it("moves the token from the hash to the storage and removes the hash", () => {
    const storage = memoryStorage();
    const replace = vi.fn();
    const token = rememberToken(
      { hash: "#token=abc", pathname: "/s/work/i/HH-1", search: "" },
      storage,
      replace,
    );
    expect(token).toBe("abc");
    expect(storage.map.get(tokenKey)).toBe("abc");
    expect(replace).toHaveBeenCalledExactlyOnceWith("/s/work/i/HH-1");
  });

  it("reads the stored token after a reload, without touching the address", () => {
    const storage = memoryStorage();
    storage.setItem(tokenKey, "abc");
    const replace = vi.fn();
    expect(rememberToken({ hash: "", pathname: "/", search: "" }, storage, replace)).toBe("abc");
    expect(replace).not.toHaveBeenCalled();
  });

  it("returns empty when there is no token anywhere", () => {
    expect(rememberToken({ hash: "", pathname: "/", search: "" }, memoryStorage(), vi.fn())).toBe(
      "",
    );
  });
});

describe("newTabUrl", () => {
  it("puts the token in the hash, where tokenFromHash reads it", () => {
    const url = newTabUrl("/s/work/i/HH-1", "abc");
    expect(url).toBe("/s/work/i/HH-1#token=abc");
    expect(tokenFromHash(url.slice(url.indexOf("#")))).toBe("abc");
  });
});
```

`web/e2e/frame.spec.ts` の最初のテストのあとに、再読み込みのテストを足す:

```ts
test("keeps the token out of the address and still reads the server after a reload", async ({
  page,
}) => {
  await page.goto(url);
  await expect(page.getByText("dev", { exact: true })).toBeVisible();
  expect(new URL(page.url()).hash).toBe("");
  await page.reload();
  await expect(page.getByText("dev", { exact: true })).toBeVisible();
});
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `cd web && pnpm test`
Expected: FAIL(`./route`・`./navigation` が見つからない、`rememberToken`・`newTabUrl` が export されていない)

- [ ] **Step 3: 実装する**

`web/packages/core/src/port/history.ts`:

```ts
// ブラウザの履歴の口。実装は app にある(History API)。core は DOM を知らない。
export interface HistoryPort {
  // path は、いまのアドレスのパス(例 "/s/work/p/HH-P1/tree")。
  path(): string;
  push(path: string): void;
  replace(path: string): void;
  // onPop は、戻る・進むで path が変わったときに listener を呼ぶ。止める関数を返す。
  onPop(listener: () => void): () => void;
}
```

`web/packages/core/src/features/router/route.ts`:

```ts
// 画面のアドレス(01「画面のアドレス」)。綴りはこの計画の決定 4:
//   /                               ホーム
//   /s/{Store名}/w/{接頭辞}           ワークスペース
//   /s/{Store名}/p/{HH-P1}/{タブ}     プロジェクト
//   /s/{Store名}/i/{HH-12}           アイテムの入口
// 解釈できないアドレスはホームにする(決定 6)。

export type ProjectTab = "overview" | "tree";

export type Route =
  | { name: "home" }
  | { name: "workspace"; store: string; prefix: string }
  | { name: "project"; store: string; project: string; tab: ProjectTab }
  | { name: "item"; store: string; item: string };

const projectTabs: readonly string[] = ["overview", "tree"] satisfies ProjectTab[];

function decode(segment: string): string | null {
  try {
    return decodeURIComponent(segment);
  } catch {
    return null;
  }
}

export function parseRoute(path: string): Route {
  const segs = path
    .split("/")
    .filter((s) => s !== "")
    .map(decode);
  if (segs.some((s) => s === null || s === "")) return { name: "home" };
  const [s, store, kind, id, tab, ...rest] = segs as string[];
  if (s !== "s" || store === undefined || id === undefined || rest.length > 0) {
    return { name: "home" };
  }
  if (kind === "w" && tab === undefined) return { name: "workspace", store, prefix: id };
  if (kind === "i" && tab === undefined) return { name: "item", store, item: id };
  if (kind === "p") {
    const t = tab ?? "overview";
    if (projectTabs.includes(t)) {
      return { name: "project", store, project: id, tab: t as ProjectTab };
    }
  }
  return { name: "home" };
}

export function formatRoute(route: Route): string {
  const e = encodeURIComponent;
  switch (route.name) {
    case "home":
      return "/";
    case "workspace":
      return `/s/${e(route.store)}/w/${e(route.prefix)}`;
    case "project":
      return `/s/${e(route.store)}/p/${e(route.project)}/${route.tab}`;
    case "item":
      return `/s/${e(route.store)}/i/${e(route.item)}`;
  }
}

// 画面の ID。接頭辞は英大文字で始まる 10 文字以内の英大文字と数字(03)。プロジェクトは番号の前に P。
export type DisplayId = { prefix: string; kind: "project" | "item"; number: number };

const displayIdPattern = /^([A-Z][A-Z0-9]{0,9})-(P?)([1-9][0-9]*)$/;

export function parseDisplayId(s: string): DisplayId | null {
  const m = displayIdPattern.exec(s);
  if (!m) return null;
  const [, prefix = "", p, digits = ""] = m;
  const number = Number(digits);
  if (!Number.isSafeInteger(number)) return null;
  return { prefix, kind: p === "P" ? "project" : "item", number };
}

export function itemDisplayId(prefix: string, number: number): string {
  return `${prefix}-${number}`;
}

export function projectDisplayId(prefix: string, number: number): string {
  return `${prefix}-P${number}`;
}
```

`web/packages/core/src/features/router/navigation.ts`:

```ts
import type { HistoryPort } from "../../port/history";
import { type Readable, readonly, writable } from "../../reactive/store";
import { formatRoute, parseRoute, type Route } from "./route";

// 画面の移り方。go は履歴に積み、replace は積まない(01「画面のアドレス」の積む・積まない)。
export interface Navigation {
  route: Readable<Route>;
  go(route: Route): void;
  replace(route: Route): void;
  // start は、戻る・進むを受け始め、止める関数を返す。
  start(): () => void;
}

export function createNavigation(history: HistoryPort): Navigation {
  const route = writable<Route>(parseRoute(history.path()));
  return {
    route: readonly(route),
    go(next) {
      const path = formatRoute(next);
      // いまと同じ画面へは移らない(履歴を積まず、画面を移ったときの動きもしない)。
      if (path === history.path()) return;
      history.push(path);
      route.set(parseRoute(path));
    },
    replace(next) {
      const path = formatRoute(next);
      history.replace(path);
      route.set(parseRoute(path));
    },
    start() {
      return history.onPop(() => route.set(parseRoute(history.path())));
    },
  };
}
```

`web/packages/core/src/index.ts`:

```ts
export { createNavigation, type Navigation } from "./features/router/navigation";
export {
  type DisplayId,
  formatRoute,
  itemDisplayId,
  type ProjectTab,
  parseDisplayId,
  parseRoute,
  projectDisplayId,
  type Route,
} from "./features/router/route";
export { createShell, type Shell } from "./features/shell/shell";
export { createSystemInfo, type SystemInfoState } from "./features/system/system";
export type { SystemInfo } from "./model/system";
export type { HistoryPort } from "./port/history";
export type { SystemPort } from "./port/system";
export {
  get,
  type Readable,
  readonly,
  type Subscriber,
  type Unsubscriber,
  type Writable,
  writable,
} from "./reactive/store";
```

`web/packages/api/src/token.ts`:

```ts
// サーバーが出す URL は http://127.0.0.1:<番号>/#token=<トークン>。トークンをハッシュに置くのは、
// サーバーのログと Referer に残さないため(土台の計画の決定 7)。
export function tokenFromHash(hash: string): string | null {
  const token = new URLSearchParams(hash.replace(/^#/, "")).get("token");
  return token ? token : null;
}

// tokenKey は、sessionStorage に置くトークンの鍵。
export const tokenKey = "hedgehug.token";

// TokenStorage は、sessionStorage のうち使う部分(テストで差し替える)。
export interface TokenStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

// rememberToken は、起動時に一度呼び、使うトークンを返す(A2 の計画の決定 5)。
// アドレスのハッシュにトークンがあれば storage に移し、replace でアドレスからハッシュを消す。
// なければ(同じタブの再読み込み)storage から読む。どちらにもなければ空。
export function rememberToken(
  location: { hash: string; pathname: string; search: string },
  storage: TokenStorage,
  replace: (url: string) => void,
): string {
  const fromHash = tokenFromHash(location.hash);
  if (fromHash) {
    storage.setItem(tokenKey, fromHash);
    replace(location.pathname + location.search);
    return fromHash;
  }
  return storage.getItem(tokenKey) ?? "";
}

// newTabUrl は、アプリの中の操作で新しいタブに開くアドレス。トークンをハッシュに付ける
// (リンクの href には付けない。右クリックの「新しいタブで開く」はトークンのない URL になる。08)。
export function newTabUrl(path: string, token: string): string {
  return `${path}#token=${encodeURIComponent(token)}`;
}
```

`web/packages/api/src/index.ts`:

```ts
export { ApiError, setToken } from "./fetcher";
export { createSystemPort } from "./system";
export { newTabUrl, rememberToken, tokenFromHash } from "./token";
```

`web/app/src/history.ts`:

```ts
import type { HistoryPort } from "@hedgehug/core";

// browserHistory は、core の履歴の口を History API で実装する。
export function browserHistory(): HistoryPort {
  return {
    path: () => location.pathname,
    push: (path) => history.pushState(null, "", path),
    replace: (path) => history.replaceState(null, "", path),
    onPop(listener) {
      window.addEventListener("popstate", listener);
      return () => window.removeEventListener("popstate", listener);
    },
  };
}
```

`web/app/src/links.ts`:

```ts
import { newTabUrl } from "@hedgehug/api";
import { formatRoute, type Navigation, type Route } from "@hedgehug/core";

// 画面を移る部品(サイドバーの項目・パンくず・タブ・行のタイトル)のリンク。
// <a href={links.href(r)} onclick={(e) => links.click(e, r)} onauxclick={(e) => links.click(e, r)}> で使う。
export interface Links {
  // href は、リンクのアドレス。トークンを入れない(右クリックの「新しいタブで開く」はトークンのない URL。08)。
  href(route: Route): string;
  // click は、ふつうのクリックで画面を移り、Ctrl・Cmd・Shift+クリックと中クリックで新しいタブに開く(01)。
  click(event: MouseEvent, route: Route): void;
}

export function createLinks(
  navigation: Navigation,
  token: string,
  open: (url: string) => void = (url) => window.open(url, "_blank", "noopener"),
): Links {
  return {
    href: formatRoute,
    click(event, route) {
      // 右クリック(button 2)などは、ブラウザのメニューに任せる。
      if (event.button > 1) return;
      event.preventDefault();
      if (event.button === 1 || event.ctrlKey || event.metaKey || event.shiftKey) {
        open(newTabUrl(formatRoute(route), token));
        return;
      }
      navigation.go(route);
    },
  };
}
```

`web/app/src/wire.ts`:

```ts
import { createSystemPort, rememberToken, setToken } from "@hedgehug/api";
import { createNavigation, createShell, createSystemInfo } from "@hedgehug/core";
import { browserHistory } from "./history";
import { createLinks } from "./links";

// 組み立て: api の実装を core に渡す(設計書 6 節)。画面はここで作った状態を使う。

// トークンは、ほかの何よりも先に決める(アドレスからハッシュを消してから、履歴を読む)。A2 の計画の決定 5。
const token = rememberToken(location, sessionStorage, (url) => history.replaceState(null, "", url));
setToken(token);

export const shell = createShell();
export const system = createSystemInfo(createSystemPort());
export const navigation = createNavigation(browserHistory());
navigation.start();
export const links = createLinks(navigation, token);
```

`web/app/src/main.ts`:

```ts
import "@fontsource/ibm-plex-sans-jp/400.css";
import "@fontsource/ibm-plex-sans-jp/500.css";
import "@fontsource/ibm-plex-sans-jp/600.css";
import "@fontsource/jetbrains-mono/400.css";
import "@fontsource/jetbrains-mono/500.css";
import "@hedgehug/ui/tokens.css";
import "@hedgehug/ui/base.css";
import { mount } from "svelte";
import App from "./App.svelte";

// トークンは wire.ts が読む(App.svelte が wire.ts を import するので、ここより先に動く)。
const target = document.getElementById("app");
if (!target) throw new Error("#app がありません");
mount(App, { target });
```

`wire.ts` は `App.svelte` から import されるので、`main.ts` の本文より先に動く。トークンを読む処理を `wire.ts` の先頭に移すのは、履歴(`browserHistory().path()`)を読む前にハッシュを消し、`links` にトークンを渡すため。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `cd web && pnpm fix && pnpm lint && pnpm check && pnpm test && pnpm deps && pnpm size`
Expected: すべて PASS(core 35 件・api 8 件・ui 13 件)、dependency-cruiser の違反 0、大きさの上限内

Run: `make e2e`
Expected: 9 件 PASS(3 つの幅 × frame.spec.ts の 3 件)

- [ ] **Step 5: Commit**

```bash
git add web/packages/core/src web/packages/api/src/token.ts web/packages/api/src/token.test.ts web/packages/api/src/index.ts web/app/src web/e2e/frame.spec.ts
git commit -m "feat(web): 画面のアドレスと、トークンをタブに残す仕組みを足す(A2 Task 4)"
```

---

### Task 5: 共有: ui の部品とトースト

**Files:**
- Modify: `web/packages/ui/src/primitives/Input.svelte`・`web/packages/ui/src/types.ts`・`web/packages/ui/src/index.ts`
- Create: `web/packages/ui/src/primitives/Checkbox.svelte`・`inputs.test.ts`
- Create: `web/packages/ui/src/patterns/KindIcon.svelte`・`StoreBadge.svelte`・`StatusDot.svelte`・`ReadOnlyBand.svelte`・`patterns.test.ts`
- Create: `web/packages/ui/src/layout/SidePanel.svelte`・`sidepanel.test.ts`
- Create: `web/packages/core/src/features/toasts/toasts.ts`・`toasts.test.ts`
- Modify: `web/packages/core/src/index.ts`・`web/app/src/wire.ts`・`web/app/src/App.svelte`

**Interfaces:**
- Consumes: 土台の ui のトークン(`--k-*`・`--cat-*`・`--t-<色>-bg/fg`・`--c-amber-*`・`--sidebar-w`・`--shadow-dialog`)、`AppFrame` の枠のコンテナ `frame`、`Toasts` 部品。
- Produces(Task 7〜10 が使う):
  - `Input` に props `heading?: boolean`(詳細パネルのタイトルの、枠のない大きな 1 行)、`id?`、`autofocus?: boolean`、`onblur?(e: FocusEvent)`、`onkeydown?(e: KeyboardEvent)` を足す。既存の props は変えない。
  - `Checkbox`(`checked` は bindable、`label`・`disabled?`・`title?`)。
  - `KindIcon`(`kind: KindName`。`KindName = "Epic" | "Story" | "Task" | "Bug" | "Step" | "Project"`。`role="img"`、名前は種類)。
  - `StoreBadge`(`name`・`color`。`color` は `--t-<色>` の色の名前: teal・orange・purple・sky・pink・olive・brown)。
  - `StatusDot`(`category: Category`。`Category = "todo" | "doing" | "done" | "canceled"`、`label`)。
  - `ReadOnlyBand`(`text`・`inset?`。`role="note"`)。
  - `SidePanel`(`open`・`full?`・`label`・`onclose`・snippet `head`・`children`。`<section aria-label={label}>` なので `getByRole("region", { name })` で指せる。Esc で `onclose`。ただし `aria-modal="true"` の要素(ダイアログ)が開いているあいだは呼ばない。`AppFrame` の中に置く)。
  - core: `createToasts(timer: Timer): Toasts`、`Toasts { list; show(text, { error? }); dismiss(id) }`、`ToastItem { id; text; error }`、`Timer { set(fn, ms): unknown; clear(handle): void }`。
  - app: `wire.ts` の `toasts`。`App.svelte` は `<Toasts toasts={$toastList} ondismiss={toasts.dismiss} />` を出す。
- 共有部分の PR(設計書 8 節「ui の基本部品」)。

- [ ] **Step 1: 失敗するテストを書く**

`web/packages/ui/src/primitives/inputs.test.ts`:

```ts
import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import Checkbox from "./Checkbox.svelte";
import Input from "./Input.svelte";

describe("Input", () => {
  it("reports blur and keydown, and takes focus when asked", async () => {
    const onblur = vi.fn();
    const onkeydown = vi.fn();
    render(Input, {
      props: { label: "タイトル", value: "認証", autofocus: true, onblur, onkeydown },
    });
    const input = screen.getByRole("textbox", { name: "タイトル" });
    expect(input).toHaveValue("認証");
    expect(input).toHaveFocus();
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(onkeydown).toHaveBeenCalledWith(expect.objectContaining({ key: "Enter" }));
    await fireEvent.blur(input);
    expect(onblur).toHaveBeenCalledOnce();
  });

  it("does not take focus unless asked", () => {
    render(Input, { props: { label: "タイトル" } });
    expect(screen.getByRole("textbox", { name: "タイトル" })).not.toHaveFocus();
  });

  it("is disabled with the note", () => {
    render(Input, {
      props: { label: "タイトル", heading: true, disabled: true, title: "閲覧のみの Store です" },
    });
    const input = screen.getByRole("textbox", { name: "タイトル" });
    expect(input).toBeDisabled();
    expect(input).toHaveAttribute("title", "閲覧のみの Store です");
  });
});

describe("Checkbox", () => {
  it("is named by its label and toggles", async () => {
    render(Checkbox, { props: { label: "続けて作成" } });
    const box = screen.getByRole("checkbox", { name: "続けて作成" });
    expect(box).not.toBeChecked();
    await fireEvent.click(box);
    expect(box).toBeChecked();
  });

  it("is disabled", () => {
    render(Checkbox, { props: { label: "続けて作成", disabled: true } });
    expect(screen.getByRole("checkbox", { name: "続けて作成" })).toBeDisabled();
  });
});
```

`web/packages/ui/src/patterns/patterns.test.ts`:

```ts
import { render, screen } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import KindIcon from "./KindIcon.svelte";
import ReadOnlyBand from "./ReadOnlyBand.svelte";
import StatusDot from "./StatusDot.svelte";
import StoreBadge from "./StoreBadge.svelte";

describe("KindIcon", () => {
  it.each([
    ["Epic", "E"],
    ["Step", "✓"],
    ["Project", "P"],
  ] as const)("shows %s as %s", (kind, glyph) => {
    render(KindIcon, { props: { kind } });
    expect(screen.getByRole("img", { name: kind })).toHaveTextContent(glyph);
  });
});

describe("StoreBadge", () => {
  it("shows the Store name in its color", () => {
    render(StoreBadge, { props: { name: "work", color: "teal" } });
    const badge = screen.getByText("work");
    expect(badge.getAttribute("style")).toContain("--t-teal-bg");
  });
});

describe("StatusDot", () => {
  it("is named by its label", () => {
    render(StatusDot, { props: { category: "doing", label: "InProgress" } });
    expect(screen.getByRole("img", { name: "InProgress" })).toHaveClass("doing");
  });
});

describe("ReadOnlyBand", () => {
  it("shows the text as a note", () => {
    render(ReadOnlyBand, { props: { text: "ro は閲覧のみの Store です。" } });
    expect(screen.getByRole("note")).toHaveTextContent("ro は閲覧のみの Store です。");
  });
});
```

`web/packages/ui/src/layout/sidepanel.test.ts`:

```ts
import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import Dialog from "../primitives/Dialog.svelte";
import { text } from "../test-snippet";
import SidePanel from "./SidePanel.svelte";

const base = { label: "詳細", head: text("見出し"), children: text("本文") };

describe("SidePanel", () => {
  it("renders nothing when closed", () => {
    render(SidePanel, { props: { ...base, open: false, onclose: vi.fn() } });
    expect(screen.queryByRole("region", { name: "詳細" })).toBeNull();
  });

  it("shows the head and the body, and marks full", () => {
    render(SidePanel, { props: { ...base, open: true, full: true, onclose: vi.fn() } });
    const panel = screen.getByRole("region", { name: "詳細" });
    expect(panel).toHaveTextContent("見出し");
    expect(panel).toHaveTextContent("本文");
    expect(panel).toHaveClass("full");
  });

  it("closes on Escape", async () => {
    const onclose = vi.fn();
    render(SidePanel, { props: { ...base, open: true, onclose } });
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(onclose).toHaveBeenCalledOnce();
  });

  it("leaves Escape to an open dialog", async () => {
    const onclose = vi.fn();
    render(SidePanel, { props: { ...base, open: true, onclose } });
    render(Dialog, { props: { open: true, title: "作成", onclose: vi.fn(), children: text("x") } });
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(onclose).not.toHaveBeenCalled();
  });
});
```

`web/packages/core/src/features/toasts/toasts.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createToasts, type Timer } from "./toasts";

// fakeTimer は、時間を手で進める時計。
function fakeTimer() {
  let now = 0;
  let seq = 0;
  const pending = new Map<number, { at: number; fn: () => void }>();
  const timer: Timer = {
    set(fn, ms) {
      seq += 1;
      pending.set(seq, { at: now + ms, fn });
      return seq;
    },
    clear(handle) {
      pending.delete(handle as number);
    },
  };
  const advance = (ms: number) => {
    now += ms;
    for (const [h, p] of [...pending]) {
      if (p.at <= now) {
        pending.delete(h);
        p.fn();
      }
    }
  };
  return { timer, advance, pending };
}

const texts = (t: ReturnType<typeof createToasts>) => get(t.list).map((x) => x.text);

describe("toasts", () => {
  it("hides a toast after 4.5 seconds and an error after 7 seconds", () => {
    const { timer, advance } = fakeTimer();
    const toasts = createToasts(timer);
    toasts.show("HH-13 を作成しました");
    toasts.show("タイトルは空にできません", { error: true });
    expect(get(toasts.list)).toEqual([
      { id: "t1", text: "HH-13 を作成しました", error: false },
      { id: "t2", text: "タイトルは空にできません", error: true },
    ]);
    advance(4499);
    expect(texts(toasts)).toHaveLength(2);
    advance(1);
    expect(texts(toasts)).toEqual(["タイトルは空にできません"]);
    advance(2500);
    expect(texts(toasts)).toEqual([]);
  });

  it("keeps 4 at most and drops the oldest", () => {
    const { timer, pending } = fakeTimer();
    const toasts = createToasts(timer);
    for (const n of [1, 2, 3, 4, 5]) toasts.show(`#${n}`);
    expect(texts(toasts)).toEqual(["#2", "#3", "#4", "#5"]);
    expect(pending.size).toBe(4);
  });

  it("closes on dismiss and stops its timer", () => {
    const { timer, pending } = fakeTimer();
    const toasts = createToasts(timer);
    toasts.show("コピーしました: HH-1");
    toasts.dismiss("t1");
    expect(texts(toasts)).toEqual([]);
    expect(pending.size).toBe(0);
    toasts.dismiss("t1");
    expect(texts(toasts)).toEqual([]);
  });
});
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `cd web && pnpm test`
Expected: FAIL(`Checkbox.svelte`・`KindIcon.svelte` など・`./toasts` が見つからない、`Input` が `onblur` を呼ばない)

- [ ] **Step 3: 実装する**

見た目はモック版 19 の `index.html` の `.kind`・`.k-*`・`.store-badge`・`.t-*`・`.sdot`・`.cat-*`・`.band`・`.peek*`・`.title-input` に合わせた。Store の色は、クラスでなく CSS 変数で渡す(Svelte は動的なクラス名に当たる CSS を消すため)。

`web/packages/ui/src/primitives/Input.svelte`:

```svelte
<script lang="ts">
  interface Props {
    value?: string;
    label: string;
    placeholder?: string;
    size?: "md" | "sm";
    mono?: boolean;
    // heading は、詳細パネルのタイトルのような、枠のない大きな 1 行の入力欄にする。
    heading?: boolean;
    disabled?: boolean;
    title?: string;
    id?: string;
    // autofocus は、出たときにフォーカスを置く(ダイアログのタイトルの欄など)。
    autofocus?: boolean;
    onblur?: (event: FocusEvent) => void;
    onkeydown?: (event: KeyboardEvent) => void;
  }

  let {
    value = $bindable(""),
    label,
    placeholder,
    size = "md",
    mono = false,
    heading = false,
    disabled = false,
    title,
    id,
    autofocus = false,
    onblur,
    onkeydown,
  }: Props = $props();

  let element: HTMLInputElement | undefined = $state();

  // autofocus 属性は a11y の警告になるので、出たときに focus を呼ぶ。
  $effect(() => {
    if (autofocus) element?.focus();
  });
</script>

<input
  bind:this={element}
  class="input {size}"
  class:mono
  class:heading
  aria-label={label}
  {id}
  {placeholder}
  {disabled}
  {title}
  {onblur}
  {onkeydown}
  bind:value
>

<style>
  .input {
    height: var(--control-h);
    border: 1px solid var(--line-input);
    border-radius: 7px;
    background: var(--surface);
    padding: 0 10px;
    min-width: 0;
  }
  .input:focus {
    outline: 2px solid var(--accent);
    outline-offset: -1px;
    border-color: transparent;
  }
  .input:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .sm {
    height: 28px;
    font-size: 12.5px;
  }
  .mono {
    font-family: var(--font-mono);
  }
  .heading {
    height: auto;
    border-color: transparent;
    border-radius: 6px;
    padding: 4px 6px;
    margin: 0 -6px;
    background: transparent;
    width: calc(100% + 12px);
    font-size: 18px;
    line-height: 1.4;
  }
  .heading:hover:not(:disabled) {
    border-color: var(--line-input);
  }
  .heading:disabled {
    opacity: 1;
    cursor: default;
  }
</style>
```

`web/packages/ui/src/primitives/Checkbox.svelte`:

```svelte
<script lang="ts">
  interface Props {
    checked?: boolean;
    // label は、チェックボックスの右に出す文字(例「続けて作成」)。
    label: string;
    disabled?: boolean;
    title?: string;
  }

  let { checked = $bindable(false), label, disabled = false, title }: Props = $props();
</script>

<label class="check" class:disabled {title}>
  <input type="checkbox" {disabled} bind:checked>
  <span>{label}</span>
</label>

<style>
  .check {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--ink-2);
    font-size: 12.5px;
    cursor: pointer;
  }
  input {
    margin: 0;
    width: 14px;
    height: 14px;
    accent-color: var(--accent);
    cursor: inherit;
  }
  .disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
</style>
```

`web/packages/ui/src/types.ts`:

```ts
export interface SelectOption {
  value: string;
  label: string;
}

export interface Tab {
  id: string;
  label: string;
  count?: number;
}

export type MenuItem =
  | { id: string; label: string; hint?: string; danger?: boolean; disabled?: boolean }
  | { separator: true };

export interface Toast {
  id: string;
  text: string;
  error?: boolean;
  action?: string;
}

// KindName は、種類のアイコンに出す種類。Project はプロジェクトのアイコン。
export type KindName = "Epic" | "Story" | "Task" | "Bug" | "Step" | "Project";

// Category は、ステータスの分類(03)。点の色を決める。
export type Category = "todo" | "doing" | "done" | "canceled";
```

`web/packages/ui/src/patterns/KindIcon.svelte`:

```svelte
<script lang="ts">
  import type { KindName } from "../types";

  interface Props {
    kind: KindName;
  }

  let { kind }: Props = $props();

  const glyphs: Record<KindName, string> = {
    Epic: "E",
    Story: "S",
    Task: "T",
    Bug: "B",
    Step: "✓",
    Project: "P",
  };
</script>

<span
  class="kind"
  role="img"
  aria-label={kind}
  title={kind}
  style="--k: var(--k-{kind.toLowerCase()})"
>
  {glyphs[kind]}
</span>

<style>
  .kind {
    width: 18px;
    height: 18px;
    border-radius: 5px;
    display: inline-grid;
    place-items: center;
    font-size: 10.5px;
    font-weight: 600;
    color: var(--surface);
    background: var(--k);
    flex: none;
    font-family: var(--font-mono);
  }
</style>
```

`web/packages/ui/src/patterns/StoreBadge.svelte`:

```svelte
<script lang="ts">
  interface Props {
    name: string;
    // color は、トークン --t-<色>-bg・--t-<色>-fg の色の名前(teal・orange・purple・sky・pink・olive・brown)。
    color: string;
  }

  let { name, color }: Props = $props();
</script>

<span class="store-badge" style="--tbg: var(--t-{color}-bg); --tfg: var(--t-{color}-fg)">
  {name}
</span>

<style>
  .store-badge {
    display: inline-flex;
    align-items: center;
    height: 20px;
    padding: 0 7px;
    border-radius: 5px;
    font-size: 11.5px;
    font-weight: 500;
    font-family: var(--font-mono);
    background: var(--tbg);
    color: var(--tfg);
    white-space: nowrap;
  }
</style>
```

`web/packages/ui/src/patterns/StatusDot.svelte`:

```svelte
<script lang="ts">
  import type { Category } from "../types";

  interface Props {
    category: Category;
    // label は、読み上げに使う名前(例 "InProgress")。
    label: string;
  }

  let { category, label }: Props = $props();
</script>

<i
  class="sdot"
  class:todo={category === "todo"}
  class:doing={category === "doing"}
  class:done={category === "done"}
  class:canceled={category === "canceled"}
  role="img"
  aria-label={label}
  title={label}
></i>

<style>
  .sdot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    display: inline-block;
    flex: none;
  }
  .todo {
    border: 1.5px solid var(--cat-todo);
  }
  .doing {
    background: var(--cat-doing);
    box-shadow: inset 0 0 0 2px var(--surface);
    border: 1.5px solid var(--cat-doing);
  }
  .done {
    background: var(--cat-done);
  }
  .canceled {
    background: var(--cat-canceled);
  }
</style>
```

`web/packages/ui/src/patterns/ReadOnlyBand.svelte`:

```svelte
<script lang="ts">
  interface Props {
    // text は、帯に出す文(01「閲覧のみの Store」の文。core が作る)。
    text: string;
    // inset は、詳細パネルの中のように、角を丸めて余白の中に置く。
    inset?: boolean;
  }

  let { text, inset = false }: Props = $props();
</script>

<p class="band" class:inset role="note">{text}</p>

<style>
  .band {
    margin: 0;
    padding: 8px 24px;
    background: var(--c-amber-bg);
    color: var(--c-amber-fg);
    font-size: 12.5px;
    border-bottom: 1px solid var(--line);
  }
  .inset {
    border-radius: 8px;
    border: 0;
    padding: 8px 12px;
  }
</style>
```

`web/packages/ui/src/layout/SidePanel.svelte`:

```svelte
<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    open: boolean;
    // full は、全画面(サイドバーの右いっぱい。860px 以下の枠では画面いっぱい)にする。
    full?: boolean;
    // label は、パネルの名前(読み上げと、テストでの指し方)。
    label: string;
    // onclose は、Esc を押したときに呼ぶ。ダイアログ(aria-modal)が開いているあいだは、ダイアログが Esc を受ける。
    onclose: () => void;
    head: Snippet;
    children: Snippet;
  }

  let { open, full = false, label, onclose, head, children }: Props = $props();

  function onkeydown(event: KeyboardEvent) {
    if (!open || event.key !== "Escape") return;
    if (document.querySelector('[aria-modal="true"]')) return;
    onclose();
  }
</script>

<svelte:window {onkeydown} />

<!-- AppFrame の中に置く(860px 以下の判定に、AppFrame の枠のコンテナクエリを使う)。 -->
{#if open}
  <section class="panel" class:full aria-label={label}>
    <header class="panel-head">{@render head()}</header>
    <div class="panel-body">{@render children()}</div>
  </section>
{/if}

<style>
  .panel {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    width: min(880px, 100vw);
    background: var(--surface);
    border-left: 1px solid var(--line);
    box-shadow: var(--shadow-dialog);
    z-index: 31;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    padding-top: env(safe-area-inset-top, 0px);
    padding-bottom: env(safe-area-inset-bottom, 0px);
    padding-right: env(safe-area-inset-right, 0px);
  }
  .panel-head {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 10px 14px;
    border-bottom: 1px solid var(--line);
    position: sticky;
    top: 0;
    background: var(--surface);
    z-index: 2;
  }
  .panel-body {
    padding: 14px 20px 40px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .full {
    left: var(--sidebar-w);
    width: auto;
    box-shadow: none;
  }
  .full .panel-body {
    width: 100%;
    max-width: 1240px;
    margin: 0 auto;
    padding: 20px 32px 48px;
  }
  @container frame (max-width: 860px) {
    .full {
      left: 0;
    }
    .full .panel-body {
      padding: 14px 16px 40px;
    }
  }
</style>
```

`web/packages/ui/src/index.ts`:

```ts
export { default as AppFrame } from "./layout/AppFrame.svelte";
export { default as Brand } from "./layout/Brand.svelte";
export { default as PageHead } from "./layout/PageHead.svelte";
export { default as SidePanel } from "./layout/SidePanel.svelte";
export { default as KindIcon } from "./patterns/KindIcon.svelte";
export { default as ReadOnlyBand } from "./patterns/ReadOnlyBand.svelte";
export { default as StatusDot } from "./patterns/StatusDot.svelte";
export { default as StoreBadge } from "./patterns/StoreBadge.svelte";
export { default as Button } from "./primitives/Button.svelte";
export { default as Checkbox } from "./primitives/Checkbox.svelte";
export { default as Dialog } from "./primitives/Dialog.svelte";
export { default as IconButton } from "./primitives/IconButton.svelte";
export { default as Input } from "./primitives/Input.svelte";
export { default as Menu } from "./primitives/Menu.svelte";
export { default as Select } from "./primitives/Select.svelte";
export { default as Switch } from "./primitives/Switch.svelte";
export { default as Tabs } from "./primitives/Tabs.svelte";
export { default as Toasts } from "./primitives/Toasts.svelte";
export type { Category, KindName, MenuItem, SelectOption, Tab, Toast } from "./types";
```

`web/packages/core/src/features/toasts/toasts.ts`:

```ts
import { get, type Readable, readonly, writable } from "../../reactive/store";

// トースト(01「画面の移り方と、共通の表示」): 同時に 4 件まで。5 件目が出ると古いものから消える。
// 通常は 4.5 秒、エラーは 7 秒で消え、「×」(dismiss)でも閉じられる。

export interface ToastItem {
  id: string;
  text: string;
  error: boolean;
}

// Timer は、時間が来たら呼ぶ仕組み。core は DOM の setTimeout を知らないので、app が渡す。
export interface Timer {
  set(fn: () => void, ms: number): unknown;
  clear(handle: unknown): void;
}

export interface Toasts {
  list: Readable<ToastItem[]>;
  show(text: string, opts?: { error?: boolean }): void;
  dismiss(id: string): void;
}

export const maxToasts = 4;
export const toastMs = 4500;
export const errorToastMs = 7000;

export function createToasts(timer: Timer): Toasts {
  const list = writable<ToastItem[]>([]);
  const handles = new Map<string, unknown>();
  let next = 0;

  function dismiss(id: string) {
    const handle = handles.get(id);
    if (handle === undefined) return;
    timer.clear(handle);
    handles.delete(id);
    list.update((items) => items.filter((t) => t.id !== id));
  }

  return {
    list: readonly(list),
    show(text, opts = {}) {
      next += 1;
      const toast: ToastItem = { id: `t${next}`, text, error: opts.error ?? false };
      handles.set(
        toast.id,
        timer.set(() => dismiss(toast.id), toast.error ? errorToastMs : toastMs),
      );
      list.update((items) => [...items, toast]);
      const items = get(list);
      for (const old of items.slice(0, Math.max(0, items.length - maxToasts))) dismiss(old.id);
    },
    dismiss,
  };
}
```

`web/packages/core/src/index.ts`:

```ts
export { createNavigation, type Navigation } from "./features/router/navigation";
export {
  type DisplayId,
  formatRoute,
  itemDisplayId,
  type ProjectTab,
  parseDisplayId,
  parseRoute,
  projectDisplayId,
  type Route,
} from "./features/router/route";
export { createShell, type Shell } from "./features/shell/shell";
export { createSystemInfo, type SystemInfoState } from "./features/system/system";
export {
  createToasts,
  type Timer,
  type ToastItem,
  type Toasts,
} from "./features/toasts/toasts";
export type { SystemInfo } from "./model/system";
export type { HistoryPort } from "./port/history";
export type { SystemPort } from "./port/system";
export {
  get,
  type Readable,
  readonly,
  type Subscriber,
  type Unsubscriber,
  type Writable,
  writable,
} from "./reactive/store";
```

`web/app/src/wire.ts`:

```ts
import { createSystemPort, rememberToken, setToken } from "@hedgehug/api";
import { createNavigation, createShell, createSystemInfo, createToasts } from "@hedgehug/core";
import { browserHistory } from "./history";
import { createLinks } from "./links";

// 組み立て: api の実装を core に渡す(設計書 6 節)。画面はここで作った状態を使う。

// トークンは、ほかの何よりも先に決める(アドレスからハッシュを消してから、履歴を読む)。A2 の計画の決定 5。
const token = rememberToken(location, sessionStorage, (url) => history.replaceState(null, "", url));
setToken(token);

export const shell = createShell();
export const system = createSystemInfo(createSystemPort());
export const navigation = createNavigation(browserHistory());
navigation.start();
export const links = createLinks(navigation, token);
export const toasts = createToasts({
  set: (fn, ms) => setTimeout(fn, ms),
  clear: (handle) => clearTimeout(handle as ReturnType<typeof setTimeout>),
});
```

`web/app/src/App.svelte`:

```svelte
<script lang="ts">
  import { AppFrame, Brand, PageHead, Toasts } from "@hedgehug/ui";
  import { shell, system, toasts } from "./wire";

  const drawerOpen = shell.drawerOpen;
  const version = system.version;
  const toastList = toasts.list;
  system.load();
</script>

<AppFrame drawerOpen={$drawerOpen} onclosedrawer={shell.closeDrawer}>
  {#snippet sidebar()}
    <Brand />
  {/snippet}
  <PageHead title="hedgehug" ondrawer={shell.openDrawer} />
  <div class="content">
    {#if $version}
      <p class="version">{$version}</p>
    {/if}
  </div>
</AppFrame>
<Toasts toasts={$toastList} ondismiss={toasts.dismiss} />

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px 24px 40px;
    min-height: 0;
  }
  .version {
    color: var(--muted);
    font-family: var(--font-mono);
    margin: 0;
  }
</style>
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `cd web && pnpm fix && pnpm lint && pnpm check && pnpm test && pnpm deps && pnpm size`
Expected: すべて PASS(core 38 件・api 8 件・ui 28 件)、svelte-check の警告 0

Run: `make e2e`
Expected: 9 件 PASS

- [ ] **Step 5: Commit**

```bash
git add web/packages/ui/src web/packages/core/src web/app/src
git commit -m "feat(web): 種類のアイコン・Store のバッジ・閲覧のみの帯・せり出すパネル・チェックボックスとトーストの状態を足す(A2 Task 5)"
```

---

### Task 6: 口の実装と一覧の状態

**前提:** Task 1・3 が develop に入っていて、`web/packages/api/src/generated.ts` に `listStores`・`listMembers`・`getMe`・`listProjects`・`getProject`・`createProject`・`listProjectItems`・`getItem`・`createItem`・`renameItem`・`addStep`・`moveItemUp`・`nextItemNumber`・`findItemByNumber` があること(`make generate && cd web && pnpm gen` で作ったもの)。`listWorkspaces` は、B1(#149)で develop に入っている(`GET /api/stores/{store}/workspaces`、operationId `list-workspaces`、応答は `WorkspaceSummary[] | null`。`WorkspaceSummary` は `{id, name, prefix}`)。A2 の Task 1 はこれを作らず、そのまま使う。引数は orval の形(パスの値を順に、本文があればそのあと)。`findItemByNumber` の番号は `number`。一覧の応答は `T[] | null`(huma の配列は null になりうる)なので、`?? []` で空にする。応答の型名には頼らず、項目名だけを使う。

**Files:**
- Create: `web/packages/core/src/model/errors.ts`・`store.ts`・`workspace.ts`・`project.ts`・`item.ts`・`member.ts`
- Create: `web/packages/core/src/port/catalog.ts`・`projects.ts`・`items.ts`
- Create: `web/packages/core/src/features/catalog/catalog.ts`・`readonly.ts`・`catalog.test.ts`・`fixtures.test-util.ts`
- Create: `web/packages/core/src/features/router/resolve.ts`・`resolve.test.ts`
- Create: `web/packages/core/src/features/items/tree.ts`・`tree.test.ts`
- Modify: `web/packages/core/src/index.ts`
- Modify: `web/packages/api/src/fetcher.ts`(失敗の応答の `detail` を `ApiError` の `message` にする)
- Create: `web/packages/api/src/errors.ts`・`catalog.ts`・`projects.ts`・`items.ts`・`ports.test.ts`
- Modify: `web/packages/api/src/index.ts`

**Interfaces:**
- Consumes: Task 4 の `projectDisplayId`・`parseDisplayId`・`Route`・`ProjectTab`、Task 1・3 の生成物。
- Produces(Task 7〜10 が使う): 「型と口」の model・port・`createCatalog`・`writableWorkspaces`・`resolveRoute`・`routeOfProject`・`routeOfWorkspace`・`compareSiblings`・`buildTree`・`ancestorsOf`。加えて:
  - `readOnlyNote`(`"閲覧のみの Store です"`。無効にした入力欄・ボタンの `title`)と `readOnlyBand(storeName): string`(01 の帯の文)。`features/catalog/readonly.ts`。
  - `Catalog.load()` は失敗を投げ、いまの一覧を変えない。重なった読み直しは、最後に始めたものだけを使う。
  - `buildTree` は、親が一覧にないアイテムを出さない。`ancestorsOf` は自分を入れず、根から順。
  - api: `createCatalogPort()`・`createProjectsPort()`・`createItemsPort()`。失敗は `Refused`(422、`message` は `detail`)・`NotFound`(404)・それ以外はそのまま(`ApiError`)。`createItem` の本文の `parent` は、親なしなら `""`。
  - `fixtures.test-util.ts` の `fakeCatalogPort()`(Store「work」S1 書ける・「ro」S2 閲覧のみ。どちらもワークスペース Web(HH)`{S}-W1`、プロジェクト HH-P1・HH-P2 `{S}-P1`・`{S}-P2`)は、Task 7〜10 の core のテストでも使う。

- [ ] **Step 1: 失敗するテストを書く**

`web/packages/core/src/features/catalog/fixtures.test-util.ts`:

```ts
import type { Project } from "../../model/project";
import type { CatalogPort } from "../../port/catalog";

// テスト用の一覧: Store「work」(書ける)と「ro」(閲覧のみ)。どちらもワークスペース Web(HH)と
// プロジェクト HH-P1・HH-P2 を持つ。内部 ID は Store ごとに違う。
export function project(storeId: string, workspaceId: string, n: number): Project {
  return {
    id: `${storeId}-P${n}`,
    storeId,
    workspaceId,
    number: n,
    rank: `V${n}`,
    title: `プロジェクト ${n}`,
    owner: "taro",
    status: "Todo",
    start: "",
    end: "",
  };
}

export function fakeCatalogPort(): CatalogPort {
  return {
    listStores: async () => [
      { id: "S1", name: "work", color: "teal", readOnly: false },
      { id: "S2", name: "ro", color: "orange", readOnly: true },
    ],
    listWorkspaces: async (storeId) => [
      { id: `${storeId}-W1`, storeId, name: "Web", prefix: "HH" },
    ],
    listProjects: async (storeId, workspaceId) => [
      project(storeId, workspaceId, 1),
      project(storeId, workspaceId, 2),
    ],
    listMembers: async () => [{ login: "taro", name: "太郎" }],
    me: async () => "taro",
  };
}
```

`web/packages/core/src/features/catalog/catalog.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog, writableWorkspaces } from "./catalog";
import { fakeCatalogPort } from "./fixtures.test-util";
import { readOnlyBand } from "./readonly";

describe("catalog", () => {
  it("loads stores, workspaces and projects with their display IDs", async () => {
    const catalog = createCatalog(fakeCatalogPort());
    expect(get(catalog.stores)).toEqual([]);
    await catalog.load();
    const stores = get(catalog.stores);
    expect(stores.map((s) => s.name)).toEqual(["work", "ro"]);
    expect(stores[0]?.workspaces[0]?.projects.map((p) => p.displayId)).toEqual(["HH-P1", "HH-P2"]);
    expect(catalog.store("S2")?.readOnly).toBe(true);
    expect(catalog.workspace("S1", "S1-W1")?.prefix).toBe("HH");
    expect(catalog.project("S1", "S1-P2")?.title).toBe("プロジェクト 2");
    expect(catalog.project("S1", "S2-P2")).toBeUndefined();
  });

  it("keeps the list when loading fails", async () => {
    const port = fakeCatalogPort();
    const catalog = createCatalog(port);
    await catalog.load();
    port.listStores = async () => {
      throw new Error("offline");
    };
    await expect(catalog.load()).rejects.toThrow("offline");
    expect(get(catalog.stores)).toHaveLength(2);
  });

  it("uses only the last of overlapping loads", async () => {
    const port = fakeCatalogPort();
    let release: () => void = () => {};
    const slow = new Promise<void>((r) => {
      release = r;
    });
    let calls = 0;
    const catalog = createCatalog({
      ...port,
      listStores: async () => {
        calls += 1;
        // 1 回目は遅く、古い一覧を返す。2 回目はすぐ、新しい一覧を返す。
        if (calls === 1) {
          await slow;
          return [{ id: "OLD", name: "old", color: "teal", readOnly: false }];
        }
        return port.listStores();
      },
    });
    const first = catalog.load();
    await catalog.load();
    release();
    await first;
    expect(get(catalog.stores).map((s) => s.name)).toEqual(["work", "ro"]);
  });

  it("lists the workspaces of writable stores only", async () => {
    const catalog = createCatalog(fakeCatalogPort());
    await catalog.load();
    expect(writableWorkspaces(get(catalog.stores)).map((w) => w.id)).toEqual(["S1-W1"]);
  });
});

describe("readOnlyBand", () => {
  it("is the sentence of spec 01", () => {
    expect(readOnlyBand("ro")).toBe(
      "ro は閲覧のみの Store です。閲覧はできますが、作成・変更・削除はできません。",
    );
  });
});
```

`web/packages/core/src/features/router/resolve.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { NotFound } from "../../model/errors";
import type { Item } from "../../model/item";
import type { ItemsPort } from "../../port/items";
import { get } from "../../reactive/store";
import { createCatalog, type StoreNode } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { resolveRoute, routeOfProject, routeOfWorkspace } from "./resolve";

async function stores(): Promise<StoreNode[]> {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  return get(catalog.stores);
}

// itemsIn は、storeId の Store にだけ HH-12 があるアイテムの口。ほかの呼び出しは使わない。
function itemsIn(storeId: string): ItemsPort {
  const unused = async (): Promise<never> => {
    throw new Error("unused");
  };
  return {
    list: unused,
    get: unused,
    create: unused,
    rename: unused,
    addStep: unused,
    moveUp: unused,
    nextNumber: unused,
    async findByNumber(s, w, n) {
      if (s !== storeId || n !== 12) throw new NotFound("not found");
      return { id: `${s}-I12`, storeId: s, workspaceId: w, projectId: `${s}-P1` } as Item;
    },
  };
}

describe("resolveRoute", () => {
  it("resolves a project by its display ID in the named store", async () => {
    const r = await resolveRoute(
      { name: "project", store: "ro", project: "HH-P2", tab: "tree" },
      await stores(),
      itemsIn("S1"),
    );
    expect(r).toEqual({ name: "project", storeId: "S2", projectId: "S2-P2", tab: "tree" });
  });

  it("looks in the other stores when the named store has no such ID", async () => {
    const r = await resolveRoute(
      { name: "item", store: "work", item: "HH-12" },
      await stores(),
      itemsIn("S2"),
    );
    expect(r).toEqual({ name: "item", storeId: "S2", projectId: "S2-P1", itemId: "S2-I12" });
  });

  it("resolves a workspace by its prefix", async () => {
    const r = await resolveRoute(
      { name: "workspace", store: "nope", prefix: "HH" },
      await stores(),
      itemsIn("S1"),
    );
    expect(r).toEqual({ name: "workspace", storeId: "S1", workspaceId: "S1-W1" });
  });

  it.each([
    { name: "project", store: "work", project: "HH-P9", tab: "overview" },
    { name: "project", store: "work", project: "HH-9", tab: "overview" },
    { name: "item", store: "work", item: "HH-13" },
    { name: "item", store: "work", item: "XX-12" },
    { name: "workspace", store: "work", prefix: "XX" },
  ] as const)("goes home when %o is not found", async (route) => {
    expect(await resolveRoute(route, await stores(), itemsIn("S1"))).toEqual({ name: "home" });
  });

  it("passes on errors other than not found", async () => {
    const items = itemsIn("S1");
    items.findByNumber = async () => {
      throw new Error("offline");
    };
    await expect(
      resolveRoute({ name: "item", store: "work", item: "HH-12" }, await stores(), items),
    ).rejects.toThrow("offline");
  });
});

describe("routes of internal IDs", () => {
  it("writes the addresses of a project and a workspace", async () => {
    const s = await stores();
    expect(routeOfProject(s, "S2", "S2-P1", "overview")).toEqual({
      name: "project",
      store: "ro",
      project: "HH-P1",
      tab: "overview",
    });
    expect(routeOfWorkspace(s, "S1", "S1-W1")).toEqual({
      name: "workspace",
      store: "work",
      prefix: "HH",
    });
    expect(routeOfProject(s, "S1", "nope", "tree")).toEqual({ name: "home" });
    expect(routeOfWorkspace(s, "S9", "S1-W1")).toEqual({ name: "home" });
  });
});
```

`web/packages/core/src/features/items/tree.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { Item, Kind } from "../../model/item";
import { ancestorsOf, buildTree, compareSiblings, type TreeNode } from "./tree";

function item(
  id: string,
  kind: Kind,
  parentId: string | null,
  rank: string,
  createdAt = "2026-11-13T09:00:00.000Z",
): Item {
  return {
    id,
    storeId: "S1",
    workspaceId: "W1",
    projectId: "P1",
    parentId,
    number: 1,
    kind,
    rank,
    title: id,
    watchers: ["taro"],
    createdAt,
    createdBy: "taro",
  };
}

const ids = (nodes: TreeNode[]): unknown[] =>
  nodes.map((n) => (n.children.length ? [n.item.id, ids(n.children)] : n.item.id));

describe("compareSiblings", () => {
  it("orders by rank, then by the time of creation, then by ID", () => {
    const list = [
      item("C", "Task", "E", "W"),
      item("B", "Task", "E", "V", "2026-11-13T09:00:01.000Z"),
      item("A2", "Task", "E", "V"),
      item("A1", "Task", "E", "V"),
    ];
    expect([...list].sort(compareSiblings).map((x) => x.id)).toEqual(["A1", "A2", "B", "C"]);
  });

  it("compares the times as times, not as text", () => {
    const a = item("A", "Task", "E", "V", "2026-11-13T18:00:00.000+09:00");
    const b = item("B", "Task", "E", "V", "2026-11-13T10:00:00.000Z");
    expect(compareSiblings(a, b)).toBeLessThan(0);
  });
});

describe("buildTree", () => {
  it("nests epics, their children and steps in sibling order", () => {
    const tree = buildTree([
      item("E2", "Epic", null, "W"),
      item("E1", "Epic", null, "V"),
      item("T2", "Task", "E1", "W"),
      item("T1", "Task", "E1", "V"),
      item("S1", "Step", "T1", "V"),
      item("X", "Task", "gone", "V"),
    ]);
    expect(ids(tree)).toEqual([["E1", [["T1", ["S1"]], "T2"]], "E2"]);
  });
});

describe("ancestorsOf", () => {
  it("returns the ancestors from the root", () => {
    const items = [
      item("E", "Epic", null, "V"),
      item("T", "Task", "E", "V"),
      item("S", "Step", "T", "V"),
    ];
    expect(ancestorsOf(items, "S").map((x) => x.id)).toEqual(["E", "T"]);
    expect(ancestorsOf(items, "E")).toEqual([]);
    expect(ancestorsOf(items, "nope")).toEqual([]);
  });

  it("stops at a loop", () => {
    const items = [item("A", "Task", "B", "V"), item("B", "Epic", "A", "V")];
    expect(ancestorsOf(items, "A").map((x) => x.id)).toEqual(["B"]);
  });
});
```

`web/packages/api/src/ports.test.ts`:

```ts
import { NotFound, Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createCatalogPort } from "./catalog";
import { setToken } from "./fetcher";
import { createItemsPort } from "./items";

afterEach(() => {
  vi.unstubAllGlobals();
});

// respond は、fetch を 1 つの応答に差し替え、呼び出しを返す。
function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async () =>
      // 204 の応答は本文を持てない(Response は null の本文だけを受ける)。
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

function call(fetch: ReturnType<typeof respond>, n = 0) {
  return fetch.mock.calls[n] as unknown as [string, RequestInit];
}

const apiItem = {
  id: "I1",
  workspace: "W1",
  project: "P1",
  parent: null,
  number: 13,
  kind: "Epic",
  rank: "V",
  title: "認証",
  watchers: null,
  createdAt: "2026-11-13T09:00:00.000Z",
  createdBy: "taro",
};

describe("items port", () => {
  it("creates an item with an empty parent for none, and returns the model", async () => {
    const fetch = respond(200, apiItem);
    const it = await createItemsPort().create("S1", {
      projectId: "P1",
      parentId: null,
      kind: "Epic",
      title: "認証",
    });
    const [url, init] = call(fetch);
    expect(url).toBe("/api/stores/S1/items");
    expect(JSON.parse(String(init.body))).toEqual({
      project: "P1",
      parent: "",
      kind: "Epic",
      title: "認証",
    });
    expect(it).toMatchObject({ id: "I1", storeId: "S1", parentId: null, watchers: [] });
  });

  it("turns a 422 into Refused with the words of the spec", async () => {
    respond(422, {
      title: "Unprocessable Entity",
      status: 422,
      detail: "タイトルは空にできません",
    });
    const err = await createItemsPort()
      .rename("S1", "I1", " ")
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(Refused);
    expect((err as Refused).message).toBe("タイトルは空にできません");
  });

  it("turns a 404 into NotFound", async () => {
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createItemsPort().findByNumber("S1", "W1", 99)).rejects.toBeInstanceOf(NotFound);
  });

  it("reads the next number and moves up", async () => {
    const fetch = respond(200, { number: 14 });
    await expect(createItemsPort().nextNumber("S1", "W1")).resolves.toBe(14);
    expect(call(fetch)[0]).toBe("/api/stores/S1/workspaces/W1/next-item-number");
    const moved = respond(204, undefined);
    await expect(createItemsPort().moveUp("S1", "I1")).resolves.toBeUndefined();
    expect(call(moved)[0]).toBe("/api/stores/S1/items/I1/move-up");
  });
});

describe("catalog port", () => {
  it("adds the store ID to workspaces and reads null lists as empty", async () => {
    respond(200, [{ id: "W1", name: "Web", prefix: "HH" }]);
    await expect(createCatalogPort().listWorkspaces("S1")).resolves.toEqual([
      { id: "W1", storeId: "S1", name: "Web", prefix: "HH" },
    ]);
    respond(200, null);
    await expect(createCatalogPort().listStores()).resolves.toEqual([]);
  });

  it("reads the login of me", async () => {
    const fetch = respond(200, { login: "taro" });
    await expect(createCatalogPort().me("S1")).resolves.toBe("taro");
    expect(call(fetch)[0]).toBe("/api/stores/S1/me");
  });
});
```

`fixtures.test-util.ts` は、名前が `.test.ts` で終わらないので Vitest はテストとして読まない。core の外へは export しない。

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `cd web && pnpm test`
Expected: FAIL(`./catalog`・`./resolve`・`./tree`・`./items`・`./catalog`(api)が見つからない)

- [ ] **Step 3: 実装する**

`web/packages/core/src/model/errors.ts`:

```ts
// サーバーが操作を断ったとき(API の 422)。message は仕様の文言で、画面にそのまま出す(A2 の計画の決定 13)。
export class Refused extends Error {
  override name = "Refused";
}

// 指したものがないとき(API の 404)。
export class NotFound extends Error {
  override name = "NotFound";
}
```

`web/packages/core/src/model/store.ts`:

```ts
// 表示中の Store。名前・色・閲覧のみはこのマシンの設定(01)。
export interface StoreInfo {
  id: string;
  name: string;
  // color は、ui のトークン --t-<色> の名前。
  color: string;
  readOnly: boolean;
}
```

`web/packages/core/src/model/workspace.ts`:

```ts
export interface Workspace {
  id: string;
  storeId: string;
  name: string;
  prefix: string;
}
```

`web/packages/core/src/model/project.ts`:

```ts
export type ProjectStatus = "Todo" | "InProgress" | "Done" | "Pending" | "Cancel";

export interface Project {
  id: string;
  storeId: string;
  workspaceId: string;
  number: number;
  rank: string;
  title: string;
  // owner は、ログイン名。空は「なし」。
  owner: string;
  status: ProjectStatus;
  // start・end は YYYY-MM-DD。空は「なし」。
  start: string;
  end: string;
}

export interface NewProject {
  title: string;
  owner: string;
  status: ProjectStatus;
  start: string;
  end: string;
}
```

`web/packages/core/src/model/item.ts`:

```ts
export type Kind = "Epic" | "Story" | "Task" | "Bug" | "Step";

export interface Item {
  id: string;
  storeId: string;
  workspaceId: string;
  projectId: string;
  // parentId は、親の内部 ID。Epic は null。
  parentId: string | null;
  number: number;
  kind: Kind;
  rank: string;
  title: string;
  watchers: string[];
  createdAt: string;
  createdBy: string;
}

export interface NewItem {
  projectId: string;
  parentId: string | null;
  kind: Kind;
  title: string;
}
```

`web/packages/core/src/model/member.ts`:

```ts
export interface Member {
  login: string;
  // name は、表示名。初期値はログイン名。
  name: string;
}
```

`web/packages/core/src/port/catalog.ts`:

```ts
import type { Member } from "../model/member";
import type { Project } from "../model/project";
import type { StoreInfo } from "../model/store";
import type { Workspace } from "../model/workspace";

// 一覧の口(Store・ワークスペース・プロジェクト・メンバー・あなた)。実装は api パッケージにある。
// 失敗は、断りなら Refused、ないものなら NotFound、それ以外はそのまま投げる。
export interface CatalogPort {
  listStores(): Promise<StoreInfo[]>;
  listWorkspaces(storeId: string): Promise<Workspace[]>;
  listProjects(storeId: string, workspaceId: string): Promise<Project[]>;
  listMembers(storeId: string): Promise<Member[]>;
  // me は、その Store での「あなた」のログイン名。
  me(storeId: string): Promise<string>;
}
```

`web/packages/core/src/port/projects.ts`:

```ts
import type { NewProject, Project } from "../model/project";

// プロジェクトの口。実装は api パッケージにある。
export interface ProjectsPort {
  get(storeId: string, projectId: string): Promise<Project>;
  create(storeId: string, workspaceId: string, input: NewProject): Promise<Project>;
}
```

`web/packages/core/src/port/items.ts`:

```ts
import type { Item, NewItem } from "../model/item";

// アイテムの口。実装は api パッケージにある。
export interface ItemsPort {
  // list は、プロジェクトのすべてのアイテムを番号の順で返す。
  list(storeId: string, projectId: string): Promise<Item[]>;
  get(storeId: string, itemId: string): Promise<Item>;
  create(storeId: string, input: NewItem): Promise<Item>;
  rename(storeId: string, itemId: string, title: string): Promise<Item>;
  addStep(storeId: string, parentId: string, title: string): Promise<Item>;
  moveUp(storeId: string, itemId: string): Promise<void>;
  // nextNumber は、ID の予告に出す番号(ワークスペースの連番の次)。
  nextNumber(storeId: string, workspaceId: string): Promise<number>;
  findByNumber(storeId: string, workspaceId: string, number: number): Promise<Item>;
}
```

`web/packages/core/src/features/catalog/catalog.ts`:

```ts
import type { Project } from "../../model/project";
import type { StoreInfo } from "../../model/store";
import type { Workspace } from "../../model/workspace";
import type { CatalogPort } from "../../port/catalog";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import { projectDisplayId } from "../router/route";

// 一覧の状態: 表示中の Store → ワークスペース → プロジェクト(サイドバー・作成ダイアログ・アドレスの解決が使う)。
// 起動時と、プロジェクトを作ったあとに読み直す(A2 の計画の決定 19)。

export interface ProjectNode extends Project {
  displayId: string;
}

export interface WorkspaceNode extends Workspace {
  projects: ProjectNode[];
}

export interface StoreNode extends StoreInfo {
  workspaces: WorkspaceNode[];
}

export interface Catalog {
  stores: Readable<StoreNode[]>;
  // load は、すべてを読み直す。失敗はそのまま投げ、いまの一覧は変えない。
  load(): Promise<void>;
  store(storeId: string): StoreNode | undefined;
  workspace(storeId: string, workspaceId: string): WorkspaceNode | undefined;
  project(storeId: string, projectId: string): ProjectNode | undefined;
}

export function createCatalog(port: CatalogPort): Catalog {
  const stores = writable<StoreNode[]>([]);
  // 読み直しが重なったときは、最後に始めたものの結果だけを使う。
  let latest = 0;

  async function loadWorkspace(storeId: string, w: Workspace): Promise<WorkspaceNode> {
    const projects = await port.listProjects(storeId, w.id);
    return {
      ...w,
      projects: projects.map((p) => ({ ...p, displayId: projectDisplayId(w.prefix, p.number) })),
    };
  }

  async function loadStore(info: StoreInfo): Promise<StoreNode> {
    const workspaces = await port.listWorkspaces(info.id);
    return {
      ...info,
      workspaces: await Promise.all(workspaces.map((w) => loadWorkspace(info.id, w))),
    };
  }

  const store = (storeId: string) => get(stores).find((s) => s.id === storeId);
  const workspace = (storeId: string, workspaceId: string) =>
    store(storeId)?.workspaces.find((w) => w.id === workspaceId);

  return {
    stores: readonly(stores),
    async load() {
      latest += 1;
      const mine = latest;
      const infos = await port.listStores();
      const loaded = await Promise.all(infos.map(loadStore));
      if (mine === latest) stores.set(loaded);
    },
    store,
    workspace,
    project(storeId, projectId) {
      for (const w of store(storeId)?.workspaces ?? []) {
        const p = w.projects.find((x) => x.id === projectId);
        if (p) return p;
      }
      return undefined;
    },
  };
}

// writableWorkspaces は、作成ダイアログに出すワークスペース(閲覧のみの Store のものを除く。01・02)。
// 非表示の Store は、一覧にもともと入らない。
export function writableWorkspaces(stores: StoreNode[]): WorkspaceNode[] {
  return stores.filter((s) => !s.readOnly).flatMap((s) => s.workspaces);
}
```

`web/packages/core/src/features/catalog/readonly.ts`:

```ts
// 閲覧のみの Store の文言(01「閲覧のみの Store」)。

// readOnlyNote は、無効にした入力欄・ボタンに添える文言。
export const readOnlyNote = "閲覧のみの Store です";

// readOnlyBand は、ワークスペース・プロジェクトの画面と詳細パネルの上部に出す帯の文。
export function readOnlyBand(storeName: string): string {
  return `${storeName} は閲覧のみの Store です。閲覧はできますが、作成・変更・削除はできません。`;
}
```

`web/packages/core/src/features/router/resolve.ts`:

```ts
import { NotFound } from "../../model/errors";
import type { ItemsPort } from "../../port/items";
import type { StoreNode } from "../catalog/catalog";
import { type ProjectTab, parseDisplayId, type Route } from "./route";

// アドレス(Store 名と画面の ID)を内部 ID にする(01「画面のアドレス」、A2 の計画の決定 7)。
// アドレスの Store 名の Store を先に探し、なければサイドバーの並びの順にほかの Store を探す。
// 見つからなければホーム。旧い接頭辞(B1)と転送の記録(A5)はたどらない。

export type Resolved =
  | { name: "home" }
  | { name: "workspace"; storeId: string; workspaceId: string }
  | { name: "project"; storeId: string; projectId: string; tab: ProjectTab }
  | { name: "item"; storeId: string; projectId: string; itemId: string };

const home = { name: "home" } as const;

function inOrder(stores: StoreNode[], name: string): StoreNode[] {
  return [...stores.filter((s) => s.name === name), ...stores.filter((s) => s.name !== name)];
}

async function findItem(
  stores: StoreNode[],
  store: string,
  id: string,
  items: ItemsPort,
): Promise<Resolved> {
  const d = parseDisplayId(id);
  if (d?.kind !== "item") return home;
  for (const s of inOrder(stores, store)) {
    const w = s.workspaces.find((x) => x.prefix === d.prefix);
    if (!w) continue;
    try {
      const it = await items.findByNumber(s.id, w.id, d.number);
      return { name: "item", storeId: s.id, projectId: it.projectId, itemId: it.id };
    } catch (e) {
      if (!(e instanceof NotFound)) throw e;
    }
  }
  return home;
}

export async function resolveRoute(
  route: Route,
  stores: StoreNode[],
  items: ItemsPort,
): Promise<Resolved> {
  switch (route.name) {
    case "home":
      return home;
    case "workspace":
      for (const s of inOrder(stores, route.store)) {
        const w = s.workspaces.find((x) => x.prefix === route.prefix);
        if (w) return { name: "workspace", storeId: s.id, workspaceId: w.id };
      }
      return home;
    case "project":
      for (const s of inOrder(stores, route.store)) {
        for (const w of s.workspaces) {
          const p = w.projects.find((x) => x.displayId === route.project);
          if (p) return { name: "project", storeId: s.id, projectId: p.id, tab: route.tab };
        }
      }
      return home;
    case "item":
      return findItem(stores, route.store, route.item, items);
  }
}

// routeOfProject は、プロジェクトの画面のアドレス。一覧にないときはホーム。
export function routeOfProject(
  stores: StoreNode[],
  storeId: string,
  projectId: string,
  tab: ProjectTab,
): Route {
  const s = stores.find((x) => x.id === storeId);
  for (const w of s?.workspaces ?? []) {
    const p = w.projects.find((x) => x.id === projectId);
    if (s && p) return { name: "project", store: s.name, project: p.displayId, tab };
  }
  return home;
}

// routeOfWorkspace は、ワークスペースの画面のアドレス。一覧にないときはホーム。
export function routeOfWorkspace(stores: StoreNode[], storeId: string, workspaceId: string): Route {
  const s = stores.find((x) => x.id === storeId);
  const w = s?.workspaces.find((x) => x.id === workspaceId);
  return s && w ? { name: "workspace", store: s.name, prefix: w.prefix } : home;
}
```

`web/packages/core/src/features/items/tree.ts`:

```ts
import type { Item } from "../../model/item";

// アイテムの親子(04「ツリー」の「行」)。兄弟は、並び順の値 → 作成の時刻 → 内部 ID の順(A2 の計画の決定 12)。

export interface TreeNode {
  item: Item;
  children: TreeNode[];
}

export function compareSiblings(a: Item, b: Item): number {
  if (a.rank !== b.rank) return a.rank < b.rank ? -1 : 1;
  const at = Date.parse(a.createdAt) - Date.parse(b.createdAt);
  if (at !== 0) return at;
  return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
}

// buildTree は、プロジェクトのアイテムを木にする。根は親のないもの(Epic)。親が一覧にないアイテムは出さない。
export function buildTree(items: Item[]): TreeNode[] {
  const byParent = new Map<string | null, Item[]>();
  for (const it of items) {
    const list = byParent.get(it.parentId) ?? [];
    list.push(it);
    byParent.set(it.parentId, list);
  }
  const grow = (parentId: string | null, seen: Set<string>): TreeNode[] =>
    [...(byParent.get(parentId) ?? [])]
      .filter((it) => !seen.has(it.id))
      .sort(compareSiblings)
      .map((it) => ({ item: it, children: grow(it.id, new Set(seen).add(it.id)) }));
  return grow(null, new Set());
}

// ancestorsOf は、id のアイテムの祖先を、根から順に返す(自分は入れない)。
export function ancestorsOf(items: Item[], id: string): Item[] {
  const byId = new Map(items.map((it) => [it.id, it]));
  const out: Item[] = [];
  const seen = new Set([id]);
  let parentId = byId.get(id)?.parentId ?? null;
  while (parentId !== null && !seen.has(parentId)) {
    const parent = byId.get(parentId);
    if (!parent) break;
    out.unshift(parent);
    seen.add(parentId);
    parentId = parent.parentId;
  }
  return out;
}
```

`web/packages/core/src/index.ts`:

```ts
export {
  type Catalog,
  createCatalog,
  type ProjectNode,
  type StoreNode,
  type WorkspaceNode,
  writableWorkspaces,
} from "./features/catalog/catalog";
export { readOnlyBand, readOnlyNote } from "./features/catalog/readonly";
export { ancestorsOf, buildTree, compareSiblings, type TreeNode } from "./features/items/tree";
export { createNavigation, type Navigation } from "./features/router/navigation";
export {
  type Resolved,
  resolveRoute,
  routeOfProject,
  routeOfWorkspace,
} from "./features/router/resolve";
export {
  type DisplayId,
  formatRoute,
  itemDisplayId,
  type ProjectTab,
  parseDisplayId,
  parseRoute,
  projectDisplayId,
  type Route,
} from "./features/router/route";
export { createShell, type Shell } from "./features/shell/shell";
export { createSystemInfo, type SystemInfoState } from "./features/system/system";
export {
  createToasts,
  type Timer,
  type ToastItem,
  type Toasts,
} from "./features/toasts/toasts";
export { NotFound, Refused } from "./model/errors";
export type { Item, Kind, NewItem } from "./model/item";
export type { Member } from "./model/member";
export type { NewProject, Project, ProjectStatus } from "./model/project";
export type { StoreInfo } from "./model/store";
export type { SystemInfo } from "./model/system";
export type { Workspace } from "./model/workspace";
export type { CatalogPort } from "./port/catalog";
export type { HistoryPort } from "./port/history";
export type { ItemsPort } from "./port/items";
export type { ProjectsPort } from "./port/projects";
export type { SystemPort } from "./port/system";
export {
  get,
  type Readable,
  readonly,
  type Subscriber,
  type Unsubscriber,
  type Writable,
  writable,
} from "./reactive/store";
```

`web/packages/api/src/fetcher.ts`:

```ts
// orval が作る呼び出しが通る fetch。トークンを付け、失敗した応答は ApiError にする。

let token = "";

export function setToken(value: string): void {
  token = value;
}

// ApiError の message は、応答の detail(断りの文言。huma のエラーの本文)があればそれ、なければ状態の行。
export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

function detailOf(text: string): string | null {
  try {
    const body: unknown = JSON.parse(text);
    if (body && typeof body === "object" && "detail" in body && typeof body.detail === "string") {
      return body.detail;
    }
  } catch {
    // JSON でない本文(401 の "unauthorized" など)は、状態の行にする。
  }
  return null;
}

export async function fetcher<T>(url: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(url, { ...init, headers });
  const text = await res.text();
  if (!res.ok) {
    throw new ApiError(res.status, detailOf(text) ?? `${res.status} ${res.statusText}`);
  }
  return (text === "" ? undefined : JSON.parse(text)) as T;
}
```

`web/packages/api/src/errors.ts`:

```ts
import { NotFound, Refused } from "@hedgehug/core";
import { ApiError } from "./fetcher";

// call は、API の呼び出しの失敗を core の言葉にする(A2 の計画の決定 13)。
// 422 は Refused(message は detail の仕様の文言)、404 は NotFound、ほかはそのまま投げる。
export async function call<T>(fn: () => Promise<T>): Promise<T> {
  try {
    return await fn();
  } catch (e) {
    if (e instanceof ApiError && e.status === 422) throw new Refused(e.message);
    if (e instanceof ApiError && e.status === 404) throw new NotFound(e.message);
    throw e;
  }
}
```

`web/packages/api/src/projects.ts`:

```ts
import type { Project, ProjectsPort } from "@hedgehug/core";
import { call } from "./errors";
import { type Project as ApiProject, createProject, getProject } from "./generated";

export function toProject(storeId: string, p: ApiProject): Project {
  return {
    id: p.id,
    storeId,
    workspaceId: p.workspace,
    number: p.number,
    rank: p.rank,
    title: p.title,
    owner: p.owner,
    status: p.status,
    start: p.start,
    end: p.end,
  };
}

export function createProjectsPort(): ProjectsPort {
  return {
    get: (storeId, projectId) =>
      call(async () => toProject(storeId, await getProject(storeId, projectId))),
    create: (storeId, workspaceId, input) =>
      call(async () => toProject(storeId, await createProject(storeId, workspaceId, input))),
  };
}
```

`web/packages/api/src/catalog.ts`:

```ts
import type { CatalogPort } from "@hedgehug/core";
import { call } from "./errors";
import { getMe, listMembers, listProjects, listStores, listWorkspaces } from "./generated";
import { toProject } from "./projects";

// 一覧の口の実装。API の型を core の型に直す(設計書 6 節)。一覧の null は空にする。
export function createCatalogPort(): CatalogPort {
  return {
    listStores: () =>
      call(async () =>
        ((await listStores()) ?? []).map((s) => ({
          id: s.id,
          name: s.name,
          color: s.color,
          readOnly: s.readOnly,
        })),
      ),
    listWorkspaces: (storeId) =>
      call(async () =>
        ((await listWorkspaces(storeId)) ?? []).map((w) => ({
          id: w.id,
          storeId,
          name: w.name,
          prefix: w.prefix,
        })),
      ),
    listProjects: (storeId, workspaceId) =>
      call(async () =>
        ((await listProjects(storeId, workspaceId)) ?? []).map((p) => toProject(storeId, p)),
      ),
    listMembers: (storeId) =>
      call(async () =>
        ((await listMembers(storeId)) ?? []).map((m) => ({ login: m.login, name: m.name })),
      ),
    me: (storeId) => call(async () => (await getMe(storeId)).login),
  };
}
```

`web/packages/api/src/items.ts`:

```ts
import type { Item, ItemsPort } from "@hedgehug/core";
import { call } from "./errors";
import {
  type Item as ApiItem,
  addStep,
  createItem,
  findItemByNumber,
  getItem,
  listProjectItems,
  moveItemUp,
  nextItemNumber,
  renameItem,
} from "./generated";

export function toItem(storeId: string, it: ApiItem): Item {
  return {
    id: it.id,
    storeId,
    workspaceId: it.workspace,
    projectId: it.project,
    parentId: it.parent,
    number: it.number,
    kind: it.kind,
    rank: it.rank,
    title: it.title,
    watchers: it.watchers ?? [],
    createdAt: it.createdAt,
    createdBy: it.createdBy,
  };
}

export function createItemsPort(): ItemsPort {
  return {
    list: (storeId, projectId) =>
      call(async () =>
        ((await listProjectItems(storeId, projectId)) ?? []).map((it) => toItem(storeId, it)),
      ),
    get: (storeId, itemId) => call(async () => toItem(storeId, await getItem(storeId, itemId))),
    create: (storeId, input) =>
      call(async () =>
        toItem(
          storeId,
          await createItem(storeId, {
            project: input.projectId,
            parent: input.parentId ?? "",
            kind: input.kind,
            title: input.title,
          }),
        ),
      ),
    rename: (storeId, itemId, title) =>
      call(async () => toItem(storeId, await renameItem(storeId, itemId, { title }))),
    addStep: (storeId, parentId, title) =>
      call(async () => toItem(storeId, await addStep(storeId, parentId, { title }))),
    moveUp: (storeId, itemId) =>
      call(async () => {
        await moveItemUp(storeId, itemId);
      }),
    nextNumber: (storeId, workspaceId) =>
      call(async () => (await nextItemNumber(storeId, workspaceId)).number),
    findByNumber: (storeId, workspaceId, number) =>
      call(async () => toItem(storeId, await findItemByNumber(storeId, workspaceId, number))),
  };
}
```

`web/packages/api/src/index.ts`:

```ts
export { createCatalogPort } from "./catalog";
export { ApiError, setToken } from "./fetcher";
export { createItemsPort } from "./items";
export { createProjectsPort } from "./projects";
export { createSystemPort } from "./system";
export { newTabUrl, rememberToken, tokenFromHash } from "./token";
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `cd web && pnpm fix && pnpm lint && pnpm check && pnpm test && pnpm deps && pnpm size`
Expected: すべて PASS(core 58 件・api 14 件・ui 28 件)

- [ ] **Step 5: Commit**

```bash
git add web/packages/core/src web/packages/api/src
git commit -m "feat(web): 一覧・プロジェクト・アイテムの口と、一覧の状態・アドレスの解決・木を足す(A2 Task 6)"
```

---

### Task 7: サイドバーと画面の切り替え

アドレスを一覧の状態で解決していまの画面を決め、サイドバー(名前と Store ごとのブロック)、ワークスペースの画面の見出しだけの枠、ホームを出す。アイテムの入口(`/s/{Store名}/i/{HH-12}`)は、そのアイテムのプロジェクトのツリーのアドレスに替える(詳細パネルを開くのは Task 10)。

**Files:**
- Create: `web/packages/core/src/features/screen/screen.ts`、`web/packages/core/src/features/screen/screen.test.ts`
- Create: `web/packages/core/src/features/projects/status.ts`、`web/packages/core/src/features/projects/status.test.ts`
- Create: `web/packages/core/src/features/items/fixtures.test-util.ts`(Task 8〜10 のテストも使うアイテムの口の偽物)
- Modify: `web/packages/core/src/index.ts`
- Create: `web/app/src/shell/Sidebar.svelte`、`web/app/src/shell/SidebarStore.svelte`、`web/app/src/shell/Screen.svelte`
- Create: `web/app/src/screens/workspace/WorkspaceScreen.svelte`
- Modify: `web/app/src/wire.ts`、`web/app/src/App.svelte`
- Create: `web/e2e/helpers.ts`、`web/e2e/sidebar.spec.ts`

**Interfaces:**
- Consumes: Task 4 の `createNavigation`・`Navigation`・`Route`・`formatRoute`・`createLinks(navigation, token)`(`links.href(route)`・`links.click(event, route)`)・`rememberToken`。Task 5 の `StoreBadge`・`StatusDot`・`ReadOnlyBand`・`createToasts(timer)`。Task 6 の `createCatalog`・`Catalog`・`StoreNode`・`WorkspaceNode`・`resolveRoute`・`routeOfProject`・`Resolved`・`readOnlyBand`・`createCatalogPort`・`createItemsPort`・`ItemsPort`・`NotFound`。
- Produces:
  - core `createScreen(deps: { navigation; catalog; items: ItemsPort; openItem(storeId, itemId): void }): Screen`。`Screen = { current: Readable<Resolved | null>; start(): () => void }`。アイテムの入口は `navigation.replace(プロジェクトのツリー)` のあと `openItem` を呼ぶ。
  - core `projectCategory(status: ProjectStatus): "todo" | "doing" | "done" | "canceled"`、`projectStatuses: ProjectStatus[]`(5 つの並び)、型 `ProjectCategory`。
  - core テスト用 `fakeItemsPort(initial?: Item[]): FakeItems`(`all`・`calls` を持つ)、`item(storeId, projectId, n, kind, parentId, title?)`、`flush()`(マイクロタスクを回す。core は `setTimeout` の型を持たない)。
  - app `wire.ts` の `catalog`・`screen`(ほかは Task 4〜5 のまま)。`catalog.load()` のあとに `screen.start()`。画面を移ったら `shell.closeDrawer()`。
  - e2e `helpers.ts` の `tokenUrl(path)`・`sidebar(page, info)`(狭い幅では引き出しを開く)・`unique(info, title)`(幅とやり直しの回数を入れた名前)。

- [ ] **Step 1: core の失敗するテストを書く**

`web/packages/core/src/features/items/fixtures.test-util.ts`:

```ts
import { NotFound, Refused } from "../../model/errors";
import type { Item, Kind } from "../../model/item";
import type { ItemsPort } from "../../port/items";

// flush は、待っている Promise の続きを先に進める(core は setTimeout の型を持たないので、マイクロタスクを回す)。
export async function flush(): Promise<void> {
  for (let i = 0; i < 50; i++) await Promise.resolve();
}

// テスト用のアイテムの口。メモリ上にアイテムを持ち、作成・変更をそのまま反映する。
// 番号はワークスペースごとの連番。断りは、タイトルが空のときだけ(サーバーの検査の代わり)。

export function item(
  storeId: string,
  projectId: string,
  n: number,
  kind: Kind,
  parentId: string | null,
  title = `アイテム ${n}`,
): Item {
  return {
    id: `${storeId}-I${n}`,
    storeId,
    workspaceId: `${storeId}-W1`,
    projectId,
    parentId,
    number: n,
    kind,
    rank: "V",
    title,
    watchers: ["taro"],
    createdAt: `2026-11-13T09:00:${String(n).padStart(2, "0")}.000Z`,
    createdBy: "taro",
  };
}

export interface FakeItems extends ItemsPort {
  all: Item[];
  calls: string[];
}

export function fakeItemsPort(initial: Item[] = []): FakeItems {
  const all = [...initial];
  const calls: string[] = [];
  const find = (id: string) => {
    const it = all.find((x) => x.id === id);
    if (!it) throw new NotFound("not found");
    return it;
  };
  const next = () => Math.max(0, ...all.map((x) => x.number)) + 1;
  const add = (storeId: string, base: Omit<Item, "id" | "number" | "storeId">): Item => {
    const n = next();
    const it = { ...base, id: `${storeId}-I${n}`, storeId, number: n };
    all.push(it);
    return it;
  };
  const checkTitle = (title: string) => {
    if (title.trim() === "") throw new Refused("タイトルを入力してください");
  };
  return {
    all,
    calls,
    async list(_s, projectId) {
      return all.filter((x) => x.projectId === projectId).sort((a, b) => a.number - b.number);
    },
    async get(_s, id) {
      return find(id);
    },
    async create(storeId, input) {
      calls.push(`create ${input.kind} ${input.title}`);
      checkTitle(input.title);
      return add(storeId, {
        workspaceId: `${storeId}-W1`,
        projectId: input.projectId,
        parentId: input.parentId,
        kind: input.kind,
        rank: "V",
        title: input.title,
        watchers: ["taro"],
        createdAt: new Date(Date.UTC(2026, 10, 13, 10, 0, all.length)).toISOString(),
        createdBy: "taro",
      });
    },
    async rename(_s, id, title) {
      calls.push(`rename ${id} ${title}`);
      const it = find(id);
      it.title = title;
      return it;
    },
    async addStep(storeId, parentId, title) {
      calls.push(`addStep ${parentId} ${title}`);
      checkTitle(title);
      const parent = find(parentId);
      return add(storeId, {
        workspaceId: parent.workspaceId,
        projectId: parent.projectId,
        parentId,
        kind: "Step",
        rank: "W",
        title,
        watchers: ["taro"],
        createdAt: new Date(Date.UTC(2026, 10, 13, 10, 0, all.length)).toISOString(),
        createdBy: "taro",
      });
    },
    async moveUp(_s, id) {
      calls.push(`moveUp ${id}`);
    },
    async nextNumber() {
      return next();
    },
    async findByNumber(_s, _w, n) {
      const it = all.find((x) => x.number === n);
      if (!it) throw new NotFound("not found");
      return it;
    },
  };
}
```

`web/packages/core/src/features/screen/screen.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { HistoryPort } from "../../port/history";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakeItemsPort, flush, item } from "../items/fixtures.test-util";
import { createNavigation } from "../router/navigation";
import { createScreen } from "./screen";

function memoryHistory(start: string): HistoryPort & { paths: string[] } {
  const paths = [start];
  return {
    paths,
    path: () => paths[paths.length - 1] ?? "/",
    push: (p) => void paths.push(p),
    replace: (p) => {
      paths[paths.length - 1] = p;
    },
    onPop: () => () => {},
  };
}

async function setup(start: string) {
  const history = memoryHistory(start);
  const navigation = createNavigation(history);
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const items = fakeItemsPort([item("S1", "S1-P1", 12, "Task", null)]);
  const opened: string[] = [];
  const screen = createScreen({
    navigation,
    catalog,
    items,
    openItem: (s, i) => void opened.push(`${s}/${i}`),
  });
  screen.start();
  await flush();
  return { screen, navigation, history, opened };
}

describe("createScreen", () => {
  it("is null before the first resolution and then follows the address", async () => {
    const { screen, navigation } = await setup("/s/work/p/HH-P2/overview");
    expect(get(screen.current)).toEqual({
      name: "project",
      storeId: "S1",
      projectId: "S1-P2",
      tab: "overview",
    });
    navigation.go({ name: "workspace", store: "work", prefix: "HH" });
    await flush();
    expect(get(screen.current)).toEqual({ name: "workspace", storeId: "S1", workspaceId: "S1-W1" });
  });

  it("opens an item entry as its project's tree with the panel, without pushing history", async () => {
    const { screen, history, opened } = await setup("/s/work/i/HH-12");
    await flush();
    expect(history.paths).toEqual(["/s/work/p/HH-P1/tree"]);
    expect(get(screen.current)).toMatchObject({ name: "project", projectId: "S1-P1", tab: "tree" });
    expect(opened).toEqual(["S1/S1-I12"]);
  });

  it("shows home for an address it cannot resolve", async () => {
    const { screen } = await setup("/s/work/p/HH-P9/overview");
    expect(get(screen.current)).toEqual({ name: "home" });
  });
});
```

`web/packages/core/src/features/projects/status.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { projectCategory } from "./status";

describe("projectCategory", () => {
  it("maps the five statuses to the four categories", () => {
    expect(projectCategory("Todo")).toBe("todo");
    expect(projectCategory("Pending")).toBe("todo");
    expect(projectCategory("InProgress")).toBe("doing");
    expect(projectCategory("Done")).toBe("done");
    expect(projectCategory("Cancel")).toBe("canceled");
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web/packages/core && npx vitest run src/features/screen src/features/projects`
Expected: FAIL(`./screen`・`./status` がない)

- [ ] **Step 3: core を実装する**

`web/packages/core/src/features/screen/screen.ts`:

```ts
import type { ItemsPort } from "../../port/items";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import type { Navigation } from "../router/navigation";
import { type Resolved, resolveRoute, routeOfProject } from "../router/resolve";

// いま出す画面。アドレスを一覧の状態で内部 ID にする(A2 の計画の決定 7)。
// アイテムの入口は、そのアイテムのプロジェクトの「ツリー」に替え(履歴に積まない)、詳細パネルを開く(01)。

export interface Screen {
  // current は、出す画面。アドレスを解決しているあいだは、前の画面のまま。最初の解決までは null。
  current: Readable<Resolved | null>;
  // start は、アドレスと一覧の変化を受け始め、止める関数を返す。一覧を読んだあとで呼ぶ。
  start(): () => void;
}

export interface ScreenDeps {
  navigation: Navigation;
  catalog: Catalog;
  items: ItemsPort;
  // openItem は、アイテムの入口で開いたアイテムの詳細パネルを開く。
  openItem(storeId: string, itemId: string): void;
}

export function createScreen(deps: ScreenDeps): Screen {
  const current = writable<Resolved | null>(null);
  // 解決が重なったときは、最後に始めたものの結果だけを使う。
  let latest = 0;

  async function resolve() {
    latest += 1;
    const mine = latest;
    const stores = get(deps.catalog.stores);
    const r = await resolveRoute(get(deps.navigation.route), stores, deps.items);
    if (mine !== latest) return;
    if (r.name === "item") {
      deps.navigation.replace(routeOfProject(stores, r.storeId, r.projectId, "tree"));
      deps.openItem(r.storeId, r.itemId);
      return;
    }
    current.set(r);
  }

  return {
    current: readonly(current),
    start() {
      // どちらの subscribe も、いまの値ですぐに 1 回呼ぶ。2 回目の解決が 1 回目に勝つので、結果は同じ。
      const stopRoute = deps.navigation.route.subscribe(() => void resolve());
      const stopCatalog = deps.catalog.stores.subscribe(() => void resolve());
      return () => {
        stopRoute();
        stopCatalog();
      };
    },
  };
}
```

`web/packages/core/src/features/projects/status.ts`:

```ts
import type { ProjectStatus } from "../../model/project";

// ProjectCategory は、プロジェクトのステータスの点の色の分類(04「Store ごとのブロック」)。
export type ProjectCategory = "todo" | "doing" | "done" | "canceled";

// projectCategory は、Todo と Pending を未着手、InProgress を進行中、Done を完了、Cancel を中止にする。
export function projectCategory(status: ProjectStatus): ProjectCategory {
  switch (status) {
    case "InProgress":
      return "doing";
    case "Done":
      return "done";
    case "Cancel":
      return "canceled";
    default:
      return "todo";
  }
}

// projectStatuses は、作成ダイアログのステータスの選択肢の並び(02「プロジェクト」)。
export const projectStatuses: ProjectStatus[] = ["Todo", "InProgress", "Done", "Pending", "Cancel"];
```

`web/packages/core/src/index.ts` に足す(`./features/items/tree` の export の次と、`./features/shell/shell` の export の前):

```ts
export {
  type ProjectCategory,
  projectCategory,
  projectStatuses,
} from "./features/projects/status";
```

```ts
export { createScreen, type Screen } from "./features/screen/screen";
```

- [ ] **Step 4: core のテストが通ることを確かめる**

Run: `cd web/packages/core && npx tsc -p . && npx vitest run`
Expected: PASS

- [ ] **Step 5: 画面の失敗するテストを書く**

`web/e2e/helpers.ts`:

```ts
import type { Page, TestInfo } from "@playwright/test";

// 通しのテストの共通の手順。

export const tokenUrl = (path = "/") => `${path}#token=e2e-token`;

// sidebar は、サイドバーを出して返す。860px 以下の幅では、引き出しを開く。
export async function sidebar(page: Page, info: TestInfo) {
  if (info.project.name !== "desktop") {
    await page.getByRole("button", { name: "メニュー", exact: true }).click();
  }
  return page.getByRole("complementary");
}

// unique は、3 つの幅が同じサーバーを使うので、作るものの名前に幅の名前を入れる(A2 の計画の決定 21)。
// やり直し(CI の retries)でも重ならないように、やり直しの回数も入れる。
export const unique = (info: TestInfo, title: string) =>
  `${title} ${info.project.name}-${info.retry}`;
```

`web/e2e/sidebar.spec.ts`:

```ts
import { expect, test } from "@playwright/test";
import { sidebar, tokenUrl } from "./helpers";

test("the sidebar lists stores and opens a workspace that survives a reload", async ({
  page,
}, info) => {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  const e2e = side.getByRole("region", { name: "e2e" });
  await expect(e2e.getByRole("link", { name: "ログイン" })).toBeVisible();
  await e2e.getByRole("link", { name: "Web" }).click();
  await expect(page.getByRole("heading", { name: "Web" })).toBeVisible();
  await expect(page).toHaveURL(/\/s\/e2e\/w\/HH$/);
  await page.reload();
  await expect(page.getByRole("heading", { name: "Web" })).toBeVisible();
});

test("a read-only store's workspace shows the band", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/w/HH"));
  await expect(
    page.getByText(
      "readonly は閲覧のみの Store です。閲覧はできますが、作成・変更・削除はできません。",
    ),
  ).toBeVisible();
});

test("the arrow hides and shows the projects of a workspace", async ({ page }, info) => {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  const e2e = side.getByRole("region", { name: "e2e" });
  await e2e.getByRole("button", { name: "開閉" }).click();
  await expect(e2e.getByRole("link", { name: "ログイン" })).toBeHidden();
  await e2e.getByRole("button", { name: "開閉" }).click();
  await expect(e2e.getByRole("link", { name: "ログイン" })).toBeVisible();
});
```

Run: `make e2e-bin && cd web && pnpm e2e sidebar.spec`
Expected: FAIL(サイドバーに Store の区画がない)

- [ ] **Step 6: 画面を実装する**

`web/app/src/shell/SidebarStore.svelte`:

```svelte
<script lang="ts">
  import { projectCategory, type Resolved, type Route, type StoreNode } from "@hedgehug/core";
  import { StatusDot, StoreBadge } from "@hedgehug/ui";
  import type { Links } from "../links";

  // サイドバーの Store ごとのブロック(04「Store ごとのブロック」)。
  interface Props {
    store: StoreNode;
    current: Resolved | null;
    links: Links;
    // collapsed は、プロジェクトの行を隠しているワークスペースの内部 ID。
    collapsed: Set<string>;
    ontoggle: (workspaceId: string) => void;
  }

  let { store, current, links, collapsed, ontoggle }: Props = $props();

  const wsRoute = (prefix: string): Route => ({ name: "workspace", store: store.name, prefix });
  const projectRoute = (displayId: string): Route => ({
    name: "project",
    store: store.name,
    project: displayId,
    tab: "overview",
  });
  const isWs = (id: string) =>
    current?.name === "workspace" && current.storeId === store.id && current.workspaceId === id;
  const isProject = (id: string) =>
    current?.name === "project" && current.storeId === store.id && current.projectId === id;
</script>

<section class="side-sec" aria-label={store.name}>
  <div class="side-head"><StoreBadge name={store.name} color={store.color} /></div>
  {#each store.workspaces as w (w.id)}
    <div class="ws-row">
      <button
        type="button"
        class="twisty"
        class:open={!collapsed.has(w.id)}
        aria-label="開閉"
        aria-expanded={!collapsed.has(w.id)}
        onclick={() => ontoggle(w.id)}
      >
        ›
      </button>
      <a
        class="ws-name"
        class:on={isWs(w.id)}
        aria-current={isWs(w.id) ? "page" : undefined}
        href={links.href(wsRoute(w.prefix))}
        onclick={(e) => links.click(e, wsRoute(w.prefix))}
        onauxclick={(e) => links.click(e, wsRoute(w.prefix))}
        >{w.name}</a
      >
      <code class="prefix">{w.prefix}</code>
    </div>
    {#if !collapsed.has(w.id)}
      {#each w.projects as p (p.id)}
        <a
          class="proj-row"
          class:on={isProject(p.id)}
          aria-current={isProject(p.id) ? "page" : undefined}
          href={links.href(projectRoute(p.displayId))}
          onclick={(e) => links.click(e, projectRoute(p.displayId))}
          onauxclick={(e) => links.click(e, projectRoute(p.displayId))}
        >
          <StatusDot category={projectCategory(p.status)} label={p.status} />
          <span>{p.title}</span>
        </a>
      {/each}
    {/if}
  {:else}
    <p class="side-empty">ワークスペースはまだありません</p>
  {/each}
</section>

<style>
  .side-sec {
    display: flex;
    flex-direction: column;
    gap: 1px;
    padding-top: 8px;
  }
  .side-head {
    display: flex;
    align-items: center;
    padding: 0 4px 4px;
  }
  .ws-row {
    display: flex;
    align-items: center;
    gap: 4px;
    min-height: 28px;
  }
  .twisty {
    width: 18px;
    height: 18px;
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
    transition: transform 0.12s;
  }
  .twisty.open {
    transform: rotate(90deg);
  }
  .ws-name,
  .proj-row {
    color: var(--ink);
    text-decoration: none;
    border-radius: 6px;
  }
  .ws-name {
    flex: 1;
    min-width: 0;
    font-weight: 500;
    padding: 3px 4px;
  }
  .proj-row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 6px 4px 26px;
    min-height: 28px;
  }
  .ws-name:hover,
  .proj-row:hover {
    background: var(--hover);
  }
  .on {
    background: var(--accent-faint);
    color: var(--accent-ink);
  }
  .prefix {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--muted);
  }
  .side-empty {
    color: var(--muted);
    font-size: 12px;
    margin: 4px 6px;
  }
</style>
```

`web/app/src/shell/Sidebar.svelte`:

```svelte
<script lang="ts">
  import type { Resolved, StoreNode } from "@hedgehug/core";
  import { Brand } from "@hedgehug/ui";
  import { SvelteSet } from "svelte/reactivity";
  import type { Links } from "../links";
  import SidebarStore from "./SidebarStore.svelte";

  // サイドバー(04「サイドバー」)。A2 は、名前と Store ごとのブロックだけを出す(「新規作成」は Task 9)。
  interface Props {
    stores: StoreNode[];
    current: Resolved | null;
    links: Links;
  }

  let { stores, current, links }: Props = $props();

  // 開閉は、このタブのあいだだけ覚える(このマシンに残すのはトラック G のマシンの設定が入ってから)。
  const collapsed = new SvelteSet<string>();
  function toggle(id: string) {
    if (collapsed.has(id)) collapsed.delete(id);
    else collapsed.add(id);
  }
</script>

<Brand />
{#each stores as store (store.id)}
  <SidebarStore {store} {current} {links} {collapsed} ontoggle={toggle} />
{/each}
```

`web/app/src/screens/workspace/WorkspaceScreen.svelte`:

```svelte
<script lang="ts">
  import { readOnlyBand, type StoreNode, type WorkspaceNode } from "@hedgehug/core";
  import { PageHead, ReadOnlyBand, StoreBadge } from "@hedgehug/ui";

  // ワークスペースの画面の見出しだけの枠(A2 の計画の持ち場)。タブの中身はトラック B・E・F が足す。
  interface Props {
    store: StoreNode;
    workspace: WorkspaceNode;
    ondrawer: () => void;
  }

  let { store, workspace, ondrawer }: Props = $props();
</script>

<PageHead title={workspace.name} {ondrawer}>
  {#snippet crumbs()}
    <StoreBadge name={store.name} color={store.color} />
    <code class="prefix">{workspace.prefix}</code>
  {/snippet}
</PageHead>
<div class="content">
  {#if store.readOnly}
    <ReadOnlyBand text={readOnlyBand(store.name)} />
  {/if}
</div>

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px 24px 40px;
    min-height: 0;
  }
  .prefix {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
</style>
```

`web/app/src/shell/Screen.svelte`:

```svelte
<script lang="ts">
  import type { Resolved, StoreNode } from "@hedgehug/core";
  import { PageHead } from "@hedgehug/ui";
  import WorkspaceScreen from "../screens/workspace/WorkspaceScreen.svelte";

  // いまの画面を出す。画面が替わるたびに作り直すので、本文は先頭から出る(01「画面の移り方」)。
  interface Props {
    current: Resolved | null;
    stores: StoreNode[];
    version: string | null;
    ondrawer: () => void;
  }

  let { current, stores, version, ondrawer }: Props = $props();

  const store = $derived(
    current && current.name !== "home" ? stores.find((s) => s.id === current.storeId) : undefined,
  );
  const workspace = $derived(
    current?.name === "workspace"
      ? store?.workspaces.find((w) => w.id === current.workspaceId)
      : undefined,
  );
</script>

{#key current}
  {#if store && workspace}
    <WorkspaceScreen {store} {workspace} {ondrawer} />
  {:else}
    <!-- ホーム。マイタスク(トラック E)が入ったら替える(A2 の計画の決定 6)。 -->
    <PageHead title="hedgehug" {ondrawer} />
    <div class="content">
      {#if version}
        <p class="version">{version}</p>
      {/if}
    </div>
  {/if}
{/key}

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px 24px 40px;
    min-height: 0;
  }
  .version {
    color: var(--muted);
    font-family: var(--font-mono);
    margin: 0;
  }
</style>
```

`web/app/src/wire.ts`(全体):

```ts
import {
  createCatalogPort,
  createItemsPort,
  createSystemPort,
  rememberToken,
  setToken,
} from "@hedgehug/api";
import {
  createCatalog,
  createNavigation,
  createScreen,
  createShell,
  createSystemInfo,
  createToasts,
} from "@hedgehug/core";
import { browserHistory } from "./history";
import { createLinks } from "./links";

// 組み立て: api の実装を core に渡す(設計書 6 節)。画面はここで作った状態を使う。

// トークンは、ほかの何よりも先に決める(アドレスからハッシュを消してから、履歴を読む)。A2 の計画の決定 5。
const token = rememberToken(location, sessionStorage, (url) => history.replaceState(null, "", url));
setToken(token);

export const shell = createShell();
export const system = createSystemInfo(createSystemPort());
export const navigation = createNavigation(browserHistory());
navigation.start();
export const links = createLinks(navigation, token);
export const toasts = createToasts({
  set: (fn, ms) => setTimeout(fn, ms),
  clear: (handle) => clearTimeout(handle as ReturnType<typeof setTimeout>),
});

const itemsPort = createItemsPort();
export const catalog = createCatalog(createCatalogPort());
export const screen = createScreen({
  navigation,
  catalog,
  items: itemsPort,
  // 詳細パネルは Task 10 で足す。それまでアイテムの入口は、プロジェクトのツリーを開くだけ。
  openItem: () => {},
});

// 画面を移ったら、サイドバーのドロワーを閉じる(01「画面の移り方」)。
navigation.route.subscribe(() => shell.closeDrawer());

// 一覧を読んでから、アドレスの解決を始める(一覧がないと Store 名と ID を解決できない)。
void catalog.load().then(() => screen.start());
```

`web/app/src/App.svelte`(全体):

```svelte
<script lang="ts">
  import { AppFrame, Toasts } from "@hedgehug/ui";
  import Screen from "./shell/Screen.svelte";
  import Sidebar from "./shell/Sidebar.svelte";
  import { catalog, links, screen, shell, system, toasts } from "./wire";

  const drawerOpen = shell.drawerOpen;
  const version = system.version;
  const toastList = toasts.list;
  const stores = catalog.stores;
  const current = screen.current;
  system.load();
</script>

<AppFrame drawerOpen={$drawerOpen} onclosedrawer={shell.closeDrawer}>
  {#snippet sidebar()}
    <Sidebar stores={$stores} current={$current} {links} />
  {/snippet}
  <Screen current={$current} stores={$stores} version={$version} ondrawer={shell.openDrawer} />
</AppFrame>
<Toasts toasts={$toastList} ondismiss={toasts.dismiss} />
```

- [ ] **Step 7: すべて通ることを確かめる**

Run: `cd web && pnpm lint && pnpm check && pnpm test && pnpm deps && pnpm size && cd .. && make e2e-bin && cd web && pnpm e2e`
Expected: PASS(e2e は 3 幅 × 6 本 = 18 本)

- [ ] **Step 8: Commit**

```bash
git add web/packages/core/src web/app/src web/e2e
git commit -m "feat(web): サイドバーと、アドレスで画面を切り替える枠を足す(A2 Task 7)"
```

---

### Task 8: プロジェクトの画面と作成ダイアログ

プロジェクトの画面(見出し・「概要」「ツリー」のタブ)と、プロジェクトの作成ダイアログを足す。入口は、ワークスペースの画面の「プロジェクト」のボタンと、サイドバーのワークスペースのメニューの「プロジェクトを作成」。ui の `Input` と `Dialog` を少し変える(共有部分。PR の説明に「ui の Input に type と focusOnMount、Dialog の重なりの順を変えた」と書く)。

**Files:**
- Modify(ui・共有部分): `web/packages/ui/src/primitives/Input.svelte`、`web/packages/ui/src/primitives/inputs.test.ts`、`web/packages/ui/src/primitives/Dialog.svelte`
- Create: `web/packages/core/src/features/dates/date.ts`(+`date.test.ts`)
- Create: `web/packages/core/src/features/projects/screen.ts`(+`screen.test.ts`)
- Create: `web/packages/core/src/features/projects/createDialog.ts`(+`createDialog.test.ts`)
- Modify: `web/packages/core/src/index.ts`
- Create: `web/app/src/screens/project/ProjectScreen.svelte`、`ProjectOverview.svelte`、`ProjectTree.svelte`、`TreeRow.svelte`、`CreateProjectDialog.svelte`
- Create: `web/app/src/shell/WorkspaceMenu.svelte`
- Modify: `web/app/src/shell/SidebarStore.svelte`、`web/app/src/shell/Screen.svelte`、`web/app/src/screens/workspace/WorkspaceScreen.svelte`、`web/app/src/wire.ts`、`web/app/src/App.svelte`
- Create: `web/e2e/project.spec.ts`

**Interfaces:**
- Consumes: Task 6 の `ProjectsPort`・`CatalogPort`(`listMembers`・`me`)・`createProjectsPort`・`Refused`・`buildTree`・`compareSiblings`・`itemDisplayId`・`writableWorkspaces`。Task 7 の `screen`・`projectCategory`・`projectStatuses`・`fakeItemsPort`。
- Produces:
  - ui `Input` の props に `type?: "text" | "date"` と `focusOnMount?: boolean`(Task 5 の `autofocus` を改名。`autofocus` は使う側のマークアップで Biome の `a11y/noAutofocus` に当たる)。
  - core `formatDay(ymd: string, thisYear: number): string`(今年は「M/D」、それ以外は「YYYY/MM/DD」、空は「なし」)。
  - core `createProjectScreen(catalog, catalogPort, items): ProjectScreen`。`ProjectScreen = { view: Readable<ProjectView | null>; load(storeId, projectId); reload() }`。`ProjectView = { store; workspace; project: ProjectNode; items: Item[]; tree: TreeNode[]; epics: Item[]; members: Member[] }`。`ownerName(login, members)`(「なし」・表示名・「不明なユーザー」)。
  - core `createProjectDialog({ catalog, catalogPort, projects, onCreated(storeId, project) }): ProjectDialog`。`ProjectDialog = { state: Readable<ProjectDialogState | null>; open(workspaceId?); update(patch: Partial<ProjectForm>); submit(); close() }`。`ProjectForm = { workspaceId; title; owner; status; start; end }`。`ProjectDialogState = { form; workspaces: Option[]; owners: Option[]; error }`。`Option = { value; label }`。
  - app `wire.ts` の `projectScreen`・`projectDialog`。作ったら `navigation.go(routeOfProject(..., "overview"))`。

- [ ] **Step 1: ui の失敗するテストを書く**

`web/packages/ui/src/primitives/inputs.test.ts` の、フォーカスのテストの props を `autofocus: true` から `focusOnMount: true` に替え、`it("is disabled with the note", ...)` の前に足す:

```ts
  it("is a date field when asked", () => {
    render(Input, { props: { label: "開始日", type: "date", value: "2026-11-13" } });
    const input = screen.getByLabelText("開始日");
    expect(input).toHaveAttribute("type", "date");
    expect(input).toHaveValue("2026-11-13");
  });
```

Run: `cd web/packages/ui && npx vitest run src/primitives/inputs.test.ts`
Expected: FAIL(type が text、focusOnMount でフォーカスが付かない)

- [ ] **Step 2: ui を直す**

`web/packages/ui/src/primitives/Input.svelte` の `<script>` と `<input>` を次にする(`<style>` はそのまま):

```svelte
<script lang="ts">
  interface Props {
    value?: string;
    label: string;
    // type は、入力の種類。date は日付の入力欄(値は YYYY-MM-DD)。
    type?: "text" | "date";
    placeholder?: string;
    size?: "md" | "sm";
    mono?: boolean;
    // heading は、詳細パネルのタイトルのような、枠のない大きな 1 行の入力欄にする。
    heading?: boolean;
    disabled?: boolean;
    title?: string;
    id?: string;
    // focusOnMount は、出たときにフォーカスを置く(ダイアログのタイトルの欄など)。autofocus という名前は、
    // 使う側のマークアップで Biome の a11y/noAutofocus に当たるので使わない。
    focusOnMount?: boolean;
    onblur?: (event: FocusEvent) => void;
    onkeydown?: (event: KeyboardEvent) => void;
  }

  let {
    value = $bindable(""),
    label,
    type = "text",
    placeholder,
    size = "md",
    mono = false,
    heading = false,
    disabled = false,
    title,
    id,
    focusOnMount = false,
    onblur,
    onkeydown,
  }: Props = $props();

  let element: HTMLInputElement | undefined = $state();

  // autofocus 属性は a11y の警告になるので、出たときに focus を呼ぶ。
  $effect(() => {
    if (focusOnMount) element?.focus();
  });
</script>

<input
  bind:this={element}
  class="input {size}"
  class:mono
  class:heading
  aria-label={label}
  {type}
  {id}
  {placeholder}
  {disabled}
  {title}
  {onblur}
  {onkeydown}
  bind:value
>
```

`web/packages/ui/src/primitives/Dialog.svelte` の `.scrim` の `z-index: 40;` を次に替える(狭い幅では、サイドバーの引き出し(50)から開くダイアログが引き出しの下に隠れるため):

```css
    /* サイドバーの引き出し(50)から開くダイアログもあるので、その上に重ねる。メニュー(60)より下。 */
    z-index: 55;
```

Run: `cd web/packages/ui && npx vitest run && npx svelte-check --tsconfig ./tsconfig.json --fail-on-warnings`
Expected: PASS

- [ ] **Step 3: core の失敗するテストを書く**

`web/packages/core/src/features/dates/date.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { formatDay } from "./date";

describe("formatDay", () => {
  it("shows M/D this year, YYYY/MM/DD otherwise, and なし when empty", () => {
    expect(formatDay("2026-03-05", 2026)).toBe("3/5");
    expect(formatDay("2025-12-31", 2026)).toBe("2025/12/31");
    expect(formatDay("", 2026)).toBe("なし");
  });
});
```

`web/packages/core/src/features/projects/screen.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { fakeItemsPort, item } from "../items/fixtures.test-util";
import { createProjectScreen, ownerName } from "./screen";

async function setup() {
  const port = fakeCatalogPort();
  const catalog = createCatalog(port);
  await catalog.load();
  const epic = { ...item("S1", "S1-P1", 1, "Epic", null), rank: "W" };
  const first = { ...item("S1", "S1-P1", 3, "Epic", null), rank: "V" };
  const task = item("S1", "S1-P1", 2, "Task", epic.id);
  const items = fakeItemsPort([epic, task, first]);
  return { screen: createProjectScreen(catalog, port, items), items };
}

describe("createProjectScreen", () => {
  it("loads the project with its tree and epics in sibling order", async () => {
    const { screen } = await setup();
    await screen.load("S1", "S1-P1");
    const v = get(screen.view);
    expect(v?.project.displayId).toBe("HH-P1");
    expect(v?.workspace.prefix).toBe("HH");
    expect(v?.epics.map((e) => e.number)).toEqual([3, 1]);
    expect(v?.tree.map((n) => n.item.number)).toEqual([3, 1]);
    expect(v?.tree[1]?.children.map((n) => n.item.number)).toEqual([2]);
    expect(v?.members).toEqual([{ login: "taro", name: "太郎" }]);
  });

  it("reloads the shown project and is null for a project not in the catalog", async () => {
    const { screen, items } = await setup();
    await screen.load("S1", "S1-P1");
    await items.create("S1", { projectId: "S1-P1", parentId: null, kind: "Epic", title: "新しい" });
    await screen.reload();
    expect(get(screen.view)?.items).toHaveLength(4);
    await screen.load("S1", "nope");
    expect(get(screen.view)).toBeNull();
  });
});

describe("ownerName", () => {
  it("shows なし, the member's name, or 不明なユーザー", () => {
    const members = [{ login: "taro", name: "太郎" }];
    expect(ownerName("", members)).toBe("なし");
    expect(ownerName("Taro", members)).toBe("太郎");
    expect(ownerName("jiro", members)).toBe("不明なユーザー");
  });
});
```

`web/packages/core/src/features/projects/createDialog.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { Refused } from "../../model/errors";
import type { NewProject, Project } from "../../model/project";
import type { ProjectsPort } from "../../port/projects";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort, project } from "../catalog/fixtures.test-util";
import { createProjectDialog } from "./createDialog";

async function setup() {
  const port = fakeCatalogPort();
  const catalog = createCatalog(port);
  await catalog.load();
  const sent: { storeId: string; workspaceId: string; input: NewProject }[] = [];
  const projects: ProjectsPort = {
    get: async () => {
      throw new Error("unused");
    },
    async create(storeId, workspaceId, input) {
      if (input.title.trim() === "") throw new Refused("タイトルを入力してください");
      sent.push({ storeId, workspaceId, input });
      return { ...project(storeId, workspaceId, 3), title: input.title };
    },
  };
  const created: string[] = [];
  const dialog = createProjectDialog({
    catalog,
    catalogPort: port,
    projects,
    onCreated: (s: string, p: Project) => void created.push(`${s}/${p.id}`),
  });
  return { dialog, sent, created };
}

describe("createProjectDialog", () => {
  it("opens on a writable workspace with あなた as the owner and Todo", async () => {
    const { dialog } = await setup();
    await dialog.open("S2-W1"); // 閲覧のみの Store のワークスペースは採らない
    const s = get(dialog.state);
    expect(s?.form).toEqual({
      workspaceId: "S1-W1",
      title: "",
      owner: "taro",
      status: "Todo",
      start: "",
      end: "",
    });
    expect(s?.workspaces).toEqual([{ value: "S1-W1", label: "work / Web" }]);
    expect(s?.owners).toEqual([
      { value: "", label: "なし" },
      { value: "taro", label: "太郎" },
    ]);
  });

  it("shows the refusal in the dialog and keeps the input", async () => {
    const { dialog, sent } = await setup();
    await dialog.open();
    await dialog.update({ title: "  " });
    await dialog.submit();
    expect(get(dialog.state)?.error).toBe("タイトルを入力してください");
    expect(get(dialog.state)?.form.title).toBe("  ");
    expect(sent).toEqual([]);
  });

  it("creates the project, closes, and hands it over", async () => {
    const { dialog, sent, created } = await setup();
    await dialog.open();
    await dialog.update({ title: "決済", status: "InProgress", start: "2026-11-01" });
    await dialog.submit();
    expect(sent).toEqual([
      {
        storeId: "S1",
        workspaceId: "S1-W1",
        input: { title: "決済", owner: "taro", status: "InProgress", start: "2026-11-01", end: "" },
      },
    ]);
    expect(get(dialog.state)).toBeNull();
    expect(created).toEqual(["S1/S1-P3"]);
  });
});
```

Run: `cd web/packages/core && npx vitest run src/features/dates src/features/projects`
Expected: FAIL(`./date`・`./screen`・`./createDialog` がない)

- [ ] **Step 4: core を実装する**

`web/packages/core/src/features/dates/date.ts`:

```ts
// 日付の表示(01「日付と時刻」): 今年なら「M/D」、それ以外の年なら「YYYY/MM/DD」。空は「なし」(01「画面の移り方」)。

// formatDay は、YYYY-MM-DD を表示の形にする。thisYear は、このマシンの時刻帯での今年。
export function formatDay(ymd: string, thisYear: number): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(ymd);
  if (!m) return "なし";
  const [, y, mo, d] = m;
  if (Number(y) === thisYear) return `${Number(mo)}/${Number(d)}`;
  return `${y}/${mo}/${d}`;
}
```

`web/packages/core/src/features/projects/screen.ts`:

```ts
import type { Item } from "../../model/item";
import type { Member } from "../../model/member";
import type { CatalogPort } from "../../port/catalog";
import type { ItemsPort } from "../../port/items";
import { type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog, ProjectNode, StoreNode, WorkspaceNode } from "../catalog/catalog";
import { buildTree, compareSiblings, type TreeNode } from "../items/tree";

// プロジェクトの画面の状態(02「画面の見出しと構成」)。アイテムを作った・変えたあとは reload で読み直す(A2 の計画の決定 19)。

export interface ProjectView {
  store: StoreNode;
  workspace: WorkspaceNode;
  project: ProjectNode;
  items: Item[];
  tree: TreeNode[];
  // epics は、概要の「Epic」の区画の行(並び順)。
  epics: Item[];
  members: Member[];
}

export interface ProjectScreen {
  view: Readable<ProjectView | null>;
  // load は、プロジェクトを読む。一覧にないプロジェクトなら null にする。
  load(storeId: string, projectId: string): Promise<void>;
  // reload は、いま出しているプロジェクトを読み直す。出していなければ何もしない。
  reload(): Promise<void>;
}

// ownerName は、オーナーの表示(01: 空は「なし」、メンバーにいない人は「不明なユーザー」)。
export function ownerName(login: string, members: Member[]): string {
  if (login === "") return "なし";
  return (
    members.find((m) => m.login.toLowerCase() === login.toLowerCase())?.name ?? "不明なユーザー"
  );
}

export function createProjectScreen(
  catalog: Catalog,
  catalogPort: CatalogPort,
  items: ItemsPort,
): ProjectScreen {
  const view = writable<ProjectView | null>(null);
  let shown: { storeId: string; projectId: string } | null = null;
  let latest = 0;

  async function load(storeId: string, projectId: string) {
    shown = { storeId, projectId };
    latest += 1;
    const mine = latest;
    const store = catalog.store(storeId);
    const project = catalog.project(storeId, projectId);
    const workspace = project && catalog.workspace(storeId, project.workspaceId);
    if (!store || !project || !workspace) {
      view.set(null);
      return;
    }
    const [list, members] = await Promise.all([
      items.list(storeId, projectId),
      catalogPort.listMembers(storeId),
    ]);
    if (mine !== latest) return;
    const epics = list.filter((it) => it.parentId === null).sort(compareSiblings);
    view.set({ store, workspace, project, items: list, tree: buildTree(list), epics, members });
  }

  return {
    view: readonly(view),
    load,
    async reload() {
      if (shown) await load(shown.storeId, shown.projectId);
    },
  };
}
```

`web/packages/core/src/features/projects/createDialog.ts`:

```ts
import { Refused } from "../../model/errors";
import type { Project, ProjectStatus } from "../../model/project";
import type { CatalogPort } from "../../port/catalog";
import type { ProjectsPort } from "../../port/projects";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import { type Catalog, writableWorkspaces } from "../catalog/catalog";

// プロジェクトの作成ダイアログ(02「プロジェクト」の「作成」)。検査はサーバーに任せ、断りの文言を赤字で出す
// (A2 の計画の決定 14)。作ったら一覧を読み直し、onCreated でそのプロジェクトの概要を開く。

export interface Option {
  value: string;
  label: string;
}

export interface ProjectForm {
  workspaceId: string;
  title: string;
  owner: string;
  status: ProjectStatus;
  start: string;
  end: string;
}

export interface ProjectDialogState {
  form: ProjectForm;
  // workspaces は「Store名 / ワークスペース名」の選択肢(閲覧のみの Store を除く)。
  workspaces: Option[];
  // owners は「なし」とメンバー。
  owners: Option[];
  error: string;
}

export interface ProjectDialog {
  state: Readable<ProjectDialogState | null>;
  // open は、workspaceId のワークスペースで開く。書けないか省いたときは、書ける先頭のワークスペース。
  open(workspaceId?: string): Promise<void>;
  // update は、入力を変える。ワークスペースを変えたら、オーナーの選択肢とあなたを読み直す。
  update(patch: Partial<ProjectForm>): Promise<void>;
  submit(): Promise<void>;
  close(): void;
}

export interface ProjectDialogDeps {
  catalog: Catalog;
  catalogPort: CatalogPort;
  projects: ProjectsPort;
  onCreated(storeId: string, project: Project): void;
}

export function createProjectDialog(deps: ProjectDialogDeps): ProjectDialog {
  const state = writable<ProjectDialogState | null>(null);

  const storeOf = (workspaceId: string) =>
    get(deps.catalog.stores).find((s) => s.workspaces.some((w) => w.id === workspaceId));

  // owners は、ワークスペースの Store のメンバーと、あなた(決まらなければ空 = なし。A2 の計画の決定 3)。
  async function owners(workspaceId: string): Promise<{ options: Option[]; me: string }> {
    const store = storeOf(workspaceId);
    if (!store) return { options: [{ value: "", label: "なし" }], me: "" };
    const [members, me] = await Promise.all([
      deps.catalogPort.listMembers(store.id),
      deps.catalogPort.me(store.id).catch(() => ""),
    ]);
    const options = [
      { value: "", label: "なし" },
      ...members.map((m) => ({ value: m.login, label: m.name })),
    ];
    return { options, me };
  }

  function set(patch: Partial<ProjectDialogState>) {
    state.update((s) => (s ? { ...s, ...patch } : s));
  }

  return {
    state: readonly(state),
    async open(workspaceId) {
      const choices = writableWorkspaces(get(deps.catalog.stores));
      const ws = choices.find((w) => w.id === workspaceId) ?? choices[0];
      if (!ws) return;
      const labels = get(deps.catalog.stores)
        .filter((s) => !s.readOnly)
        .flatMap((s) => s.workspaces.map((w) => ({ value: w.id, label: `${s.name} / ${w.name}` })));
      const o = await owners(ws.id);
      state.set({
        form: { workspaceId: ws.id, title: "", owner: o.me, status: "Todo", start: "", end: "" },
        workspaces: labels,
        owners: o.options,
        error: "",
      });
    },
    async update(patch) {
      const s = get(state);
      if (!s) return;
      const form = { ...s.form, ...patch };
      if (patch.workspaceId === undefined || patch.workspaceId === s.form.workspaceId) {
        set({ form });
        return;
      }
      set({ form });
      // 別の Store のワークスペースに替えたら、その Store のメンバーと、その Store でのあなたにする。
      if (storeOf(patch.workspaceId)?.id === storeOf(s.form.workspaceId)?.id) return;
      const o = await owners(patch.workspaceId);
      state.update((cur) =>
        cur ? { ...cur, owners: o.options, form: { ...cur.form, owner: o.me } } : cur,
      );
    },
    async submit() {
      const s = get(state);
      const store = s && storeOf(s.form.workspaceId);
      if (!s || !store) return;
      const { workspaceId, ...input } = s.form;
      try {
        const created = await deps.projects.create(store.id, workspaceId, input);
        state.set(null);
        await deps.catalog.load();
        deps.onCreated(store.id, created);
      } catch (e) {
        if (!(e instanceof Refused)) throw e;
        set({ error: e.message });
      }
    },
    close() {
      state.set(null);
    },
  };
}
```

`web/packages/core/src/index.ts` に足す(`./features/catalog/readonly` の export の次):

```ts
export { formatDay } from "./features/dates/date";
```

(`./features/items/tree` の export の次、`./features/projects/status` の前):

```ts
export {
  createProjectDialog,
  type Option,
  type ProjectDialog,
  type ProjectDialogState,
  type ProjectForm,
} from "./features/projects/createDialog";
export {
  createProjectScreen,
  ownerName,
  type ProjectScreen,
  type ProjectView,
} from "./features/projects/screen";
```

Run: `cd web/packages/core && npx tsc -p . && npx vitest run`
Expected: PASS

- [ ] **Step 5: 画面の失敗するテストを書く**

`web/e2e/project.spec.ts`:

```ts
import { expect, test } from "@playwright/test";
import { sidebar, tokenUrl, unique } from "./helpers";

test("opens a project's overview and tree from the sidebar", async ({ page }, info) => {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await side.getByRole("region", { name: "e2e" }).getByRole("link", { name: "ログイン" }).click();
  await expect(page.getByRole("heading", { name: "HH-P1 ログイン" })).toBeVisible();
  await expect(page).toHaveURL(/\/s\/e2e\/p\/HH-P1\/overview$/);
  await expect(page.getByRole("region", { name: "Epic" }).getByText("認証")).toBeVisible();
  await page.getByRole("tab", { name: "ツリー" }).click();
  await expect(page).toHaveURL(/\/s\/e2e\/p\/HH-P1\/tree$/);
  const tree = page.getByRole("list", { name: "ツリー" });
  await expect(tree.getByText("HH-2", { exact: true })).toBeVisible();
  await expect(tree.getByText("ログイン画面を作る")).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/overview$/);
});

test("creates a project from the workspace screen and opens it", async ({ page }, info) => {
  const title = unique(info, "決済");
  await page.goto(tokenUrl("/s/e2e/w/HH"));
  await page.getByRole("button", { name: "プロジェクト", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "プロジェクトを作成" });
  await expect(dialog.getByRole("combobox", { name: "オーナー" })).toHaveValue("taro");
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("ログイン");
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(
    dialog.getByText("このワークスペースには同じ名前のプロジェクトがあります"),
  ).toBeVisible();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(title);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(dialog).toBeHidden();
  await expect(
    page.getByRole("heading", { name: new RegExp(`^HH-P\\d+ ${title}$`) }),
  ).toBeVisible();
  const side = await sidebar(page, info);
  await expect(side.getByRole("link", { name: title })).toBeVisible();
});

test("a read-only store cannot create a project", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/w/HH"));
  await expect(page.getByRole("button", { name: "プロジェクト", exact: true })).toBeDisabled();
  await page.goto(tokenUrl("/s/readonly/p/HH-P1/overview"));
  await expect(page.getByRole("heading", { name: "HH-P1 ログイン" })).toBeVisible();
  await expect(
    page.getByText(
      "readonly は閲覧のみの Store です。閲覧はできますが、作成・変更・削除はできません。",
    ),
  ).toBeVisible();
});
```

Run: `make e2e-bin && cd web && pnpm e2e project.spec`
Expected: FAIL(プロジェクトの画面がない)

- [ ] **Step 6: 画面を実装する**

`web/app/src/screens/project/TreeRow.svelte`:

```svelte
<script lang="ts">
  import { itemDisplayId, type TreeNode } from "@hedgehug/core";
  import { KindIcon } from "@hedgehug/ui";
  import Self from "./TreeRow.svelte";

  // ツリーの 1 行と、その下の行(04「ツリー」の「行」のうち、A2 の計画の決定 17 の分)。
  interface Props {
    node: TreeNode;
    prefix: string;
    depth: number;
    collapsed: Set<string>;
    ontoggle: (id: string) => void;
  }

  let { node, prefix, depth, collapsed, ontoggle }: Props = $props();

  const it = $derived(node.item);
  const open = $derived(!collapsed.has(it.id));
</script>

<li class="tree-row" style:--d={depth}>
  {#if node.children.length > 0}
    <button
      type="button"
      class="twisty"
      class:open
      aria-label="開閉"
      aria-expanded={open}
      onclick={() => ontoggle(it.id)}
    >
      ›
    </button>
  {:else}
    <span class="twisty-space"></span>
  {/if}
  <KindIcon kind={it.kind} />
  <code class="id">{itemDisplayId(prefix, it.number)}</code>
  <span class="row-title">{it.title}</span>
</li>
{#if open}
  {#each node.children as c (c.item.id)}
    <Self node={c} {prefix} depth={depth + 1} {collapsed} {ontoggle} />
  {/each}
{/if}

<style>
  .tree-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    padding: 2px 8px 2px calc(8px + var(--d) * 22px);
    border-bottom: 1px solid var(--line);
  }
  .twisty,
  .twisty-space {
    width: 18px;
    height: 18px;
    flex: none;
  }
  .twisty {
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
    transition: transform 0.12s;
  }
  .twisty.open {
    transform: rotate(90deg);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
```

`web/app/src/screens/project/ProjectTree.svelte`:

```svelte
<script lang="ts">
  import type { ProjectView } from "@hedgehug/core";
  import { SvelteSet } from "svelte/reactivity";
  import TreeRow from "./TreeRow.svelte";

  // プロジェクトの「ツリー」のタブ。Epic を一番上の階層に並べる(04「ツリー」の「行」)。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  // 開閉は、画面を出しているあいだだけ覚える。初めはすべて開いている。
  const collapsed = new SvelteSet<string>();
  function toggle(id: string) {
    if (collapsed.has(id)) collapsed.delete(id);
    else collapsed.add(id);
  }
</script>

<ul class="tree" aria-label="ツリー">
  {#each view.tree as node (node.item.id)}
    <TreeRow {node} prefix={view.workspace.prefix} depth={0} {collapsed} ontoggle={toggle} />
  {/each}
</ul>

<style>
  .tree {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
  }
</style>
```

`web/app/src/screens/project/ProjectOverview.svelte`(概要のコンテナクエリは、枠の要素 `.overview-frame` に置く。グリッド自身に置くと、自分の幅では形が変わらない):

```svelte
<script lang="ts">
  import { formatDay, itemDisplayId, ownerName, type ProjectView } from "@hedgehug/core";
  import { KindIcon } from "@hedgehug/ui";

  // プロジェクトの「概要」のタブ。A2 では「Epic」の区画と、プロパティの表示だけ(A2 の計画の決定 16)。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  const year = new Date().getFullYear();
  const p = $derived(view.project);
</script>

<div class="overview-frame">
  <div class="overview">
    <section class="card-sec" aria-label="Epic">
      <div class="sec-head">
        <h2>Epic</h2>
      </div>
      {#each view.epics as e (e.id)}
        <div class="list-row">
          <KindIcon kind="Epic" />
          <code class="id">{itemDisplayId(view.workspace.prefix, e.number)}</code>
          <span class="row-title">{e.title}</span>
        </div>
      {/each}
    </section>
    <section class="props" aria-label="プロパティ">
      <div class="prop"><span class="prop-k">ステータス</span><span>{p.status}</span></div>
      <div class="prop">
        <span class="prop-k">オーナー</span><span>{ownerName(p.owner, view.members)}</span>
      </div>
      <div class="prop">
        <span class="prop-k">開始日</span><span>{formatDay(p.start, year)}</span>
      </div>
      <div class="prop">
        <span class="prop-k">終了日</span><span>{formatDay(p.end, year)}</span>
      </div>
    </section>
  </div>
</div>

<style>
  .overview-frame {
    container: overview / inline-size;
  }
  .overview {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 260px;
    gap: 24px;
  }
  @container overview (max-width: 640px) {
    .overview {
      grid-template-columns: minmax(0, 1fr);
    }
  }
  .sec-head {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  h2 {
    font-size: 14px;
    margin: 0 0 8px;
  }
  .list-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    border-bottom: 1px solid var(--line);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    flex: 1;
    min-width: 0;
  }
  .props {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .prop {
    display: grid;
    grid-template-columns: 90px minmax(0, 1fr);
    align-items: center;
  }
  .prop-k {
    color: var(--muted);
    font-size: 12.5px;
  }
</style>
```

`web/app/src/screens/project/ProjectScreen.svelte`(`$view.store.id === storeId` も比べる。サンプルの 2 つの Store は同じ内部 ID を持つ):

```svelte
<script lang="ts">
  import { type ProjectTab, projectCategory, type Route, readOnlyBand } from "@hedgehug/core";
  import { PageHead, ReadOnlyBand, StatusDot, StoreBadge, Tabs } from "@hedgehug/ui";
  import { links, navigation, projectScreen } from "../../wire";
  import ProjectOverview from "./ProjectOverview.svelte";
  import ProjectTree from "./ProjectTree.svelte";

  // プロジェクトの画面(02「画面の見出しと構成」)。タブは「概要」「ツリー」だけ(A2 の計画の決定 15)。
  interface Props {
    storeId: string;
    projectId: string;
    tab: ProjectTab;
    ondrawer: () => void;
  }

  let { storeId, projectId, tab, ondrawer }: Props = $props();

  const view = projectScreen.view;
  $effect(() => {
    void projectScreen.load(storeId, projectId);
  });

  const tabs = [
    { id: "overview", label: "概要" },
    { id: "tree", label: "ツリー" },
  ];
  const wsRoute = $derived<Route | null>(
    $view ? { name: "workspace", store: $view.store.name, prefix: $view.workspace.prefix } : null,
  );
  function select(id: string) {
    if (!$view) return;
    const next = id === "tree" ? "tree" : "overview";
    navigation.go({
      name: "project",
      store: $view.store.name,
      project: $view.project.displayId,
      tab: next,
    });
  }
</script>

{#if $view && $view.store.id === storeId && $view.project.id === projectId && wsRoute}
  {@const p = $view.project}
  <PageHead title="{p.displayId} {p.title}" {ondrawer}>
    {#snippet crumbs()}
      <StoreBadge name={$view.store.name} color={$view.store.color} />
      <a
        class="crumb"
        href={links.href(wsRoute)}
        onclick={(e) => links.click(e, wsRoute)}
        onauxclick={(e) => links.click(e, wsRoute)}
        >{$view.workspace.name}</a
      >
    {/snippet}
    {#snippet actions()}
      <span class="status-pill">
        <StatusDot category={projectCategory(p.status)} label={p.status} />{p.status}
      </span>
    {/snippet}
  </PageHead>
  <Tabs {tabs} current={tab} onselect={select} />
  <div class="content">
    {#if $view.store.readOnly}
      <ReadOnlyBand text={readOnlyBand($view.store.name)} />
    {/if}
    {#if tab === "tree"}
      <ProjectTree view={$view} />
    {:else}
      <ProjectOverview view={$view} />
    {/if}
  </div>
{/if}

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px 24px 40px;
    min-height: 0;
  }
  .crumb {
    color: var(--muted);
    text-decoration: none;
  }
  .crumb:hover {
    color: var(--ink);
    text-decoration: underline;
  }
  .status-pill {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 2px 10px;
    border: 1px solid var(--line);
    border-radius: 999px;
    font-size: 12.5px;
  }
</style>
```

`web/app/src/screens/project/CreateProjectDialog.svelte`:

```svelte
<script lang="ts">
  import { type ProjectStatus, projectStatuses } from "@hedgehug/core";
  import { Button, Dialog, Input, Select } from "@hedgehug/ui";
  import { projectDialog } from "../../wire";

  // プロジェクトの作成ダイアログ(02「プロジェクト」の「作成」)。値と送信は core の projectDialog が持つ。
  const dialog = projectDialog.state;
  const statuses = projectStatuses.map((s) => ({ value: s, label: s }));

  function onkeydown(event: KeyboardEvent) {
    // Enter で作成する(02)。日本語の変換を確定する Enter は除く。
    if (event.key === "Enter" && !event.isComposing) void projectDialog.submit();
  }
</script>

{#if $dialog}
  {@const s = $dialog}
  <Dialog open title="プロジェクトを作成" onclose={projectDialog.close}>
    <div class="form">
      <div class="field">
        <span>ワークスペース</span>
        <Select
          label="ワークスペース"
          options={s.workspaces}
          bind:value={
            () => s.form.workspaceId,
            (v) => void projectDialog.update({ workspaceId: v })
          }
        />
      </div>
      <div class="field">
        <span>タイトル</span>
        <Input
          label="タイトル"
          focusOnMount
          {onkeydown}
          bind:value={() => s.form.title, (v) => void projectDialog.update({ title: v })}
        />
      </div>
      <div class="field">
        <span>オーナー</span>
        <Select
          label="オーナー"
          options={s.owners}
          bind:value={() => s.form.owner, (v) => void projectDialog.update({ owner: v })}
        />
      </div>
      <div class="field">
        <span>ステータス</span>
        <Select
          label="ステータス"
          options={statuses}
          bind:value={
            () => s.form.status,
            (v) => void projectDialog.update({ status: v as ProjectStatus })
          }
        />
      </div>
      <div class="dates">
        <div class="field">
          <span>開始日</span>
          <Input
            label="開始日"
            type="date"
            bind:value={() => s.form.start, (v) => void projectDialog.update({ start: v })}
          />
        </div>
        <div class="field">
          <span>終了日</span>
          <Input
            label="終了日"
            type="date"
            bind:value={() => s.form.end, (v) => void projectDialog.update({ end: v })}
          />
        </div>
      </div>
      {#if s.error}
        <p class="form-error" role="alert">{s.error}</p>
      {/if}
    </div>
    {#snippet footer()}
      <span class="grow"></span>
      <Button onclick={projectDialog.close}>キャンセル</Button>
      <Button variant="primary" onclick={() => void projectDialog.submit()}>作成</Button>
    {/snippet}
  </Dialog>
{/if}

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .field > span {
    font-size: 12.5px;
    color: var(--muted);
  }
  .dates {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }
  .form-error {
    color: var(--danger);
    margin: 0;
    font-size: 13px;
  }
  .grow {
    flex: 1;
  }
</style>
```

`web/app/src/shell/WorkspaceMenu.svelte`:

```svelte
<script lang="ts">
  import { readOnlyNote, type StoreNode, type WorkspaceNode } from "@hedgehug/core";
  import { IconButton, Menu } from "@hedgehug/ui";
  import { projectDialog } from "../wire";

  // サイドバーのワークスペースのメニュー(04「ワークスペースのメニュー」)。A2 は「プロジェクトを作成」だけを置き、
  // ほかの項目はトラック B2 が足す(トラック B の計画の未決 1)。
  interface Props {
    store: StoreNode;
    workspace: WorkspaceNode;
  }

  let { store, workspace }: Props = $props();

  let open = $state(false);
  let x = $state(0);
  let y = $state(0);

  function show(event: MouseEvent) {
    const r = (event.currentTarget as HTMLElement).getBoundingClientRect();
    x = r.left;
    y = r.bottom + 4;
    open = true;
  }

  function select(id: string) {
    open = false;
    if (id === "create-project") void projectDialog.open(workspace.id);
  }
</script>

<IconButton label="ワークスペースのメニュー" size="xs" onclick={show}>⋯</IconButton>
<Menu
  {open}
  {x}
  {y}
  items={[
    {
      id: "create-project",
      label: "プロジェクトを作成",
      disabled: store.readOnly,
      hint: store.readOnly ? readOnlyNote : undefined,
    },
  ]}
  onselect={select}
  onclose={() => (open = false)}
/>
```

`web/app/src/shell/SidebarStore.svelte`(全体。ワークスペースの行の接頭辞の次にメニューを置く):

```svelte
<script lang="ts">
  import { projectCategory, type Resolved, type Route, type StoreNode } from "@hedgehug/core";
  import { StatusDot, StoreBadge } from "@hedgehug/ui";
  import type { Links } from "../links";
  import WorkspaceMenu from "./WorkspaceMenu.svelte";

  // サイドバーの Store ごとのブロック(04「Store ごとのブロック」)。
  interface Props {
    store: StoreNode;
    current: Resolved | null;
    links: Links;
    // collapsed は、プロジェクトの行を隠しているワークスペースの内部 ID。
    collapsed: Set<string>;
    ontoggle: (workspaceId: string) => void;
  }

  let { store, current, links, collapsed, ontoggle }: Props = $props();

  const wsRoute = (prefix: string): Route => ({ name: "workspace", store: store.name, prefix });
  const projectRoute = (displayId: string): Route => ({
    name: "project",
    store: store.name,
    project: displayId,
    tab: "overview",
  });
  const isWs = (id: string) =>
    current?.name === "workspace" && current.storeId === store.id && current.workspaceId === id;
  const isProject = (id: string) =>
    current?.name === "project" && current.storeId === store.id && current.projectId === id;
</script>

<section class="side-sec" aria-label={store.name}>
  <div class="side-head"><StoreBadge name={store.name} color={store.color} /></div>
  {#each store.workspaces as w (w.id)}
    <div class="ws-row">
      <button
        type="button"
        class="twisty"
        class:open={!collapsed.has(w.id)}
        aria-label="開閉"
        aria-expanded={!collapsed.has(w.id)}
        onclick={() => ontoggle(w.id)}
      >
        ›
      </button>
      <a
        class="ws-name"
        class:on={isWs(w.id)}
        aria-current={isWs(w.id) ? "page" : undefined}
        href={links.href(wsRoute(w.prefix))}
        onclick={(e) => links.click(e, wsRoute(w.prefix))}
        onauxclick={(e) => links.click(e, wsRoute(w.prefix))}
        >{w.name}</a
      >
      <code class="prefix">{w.prefix}</code>
      <WorkspaceMenu {store} workspace={w} />
    </div>
    {#if !collapsed.has(w.id)}
      {#each w.projects as p (p.id)}
        <a
          class="proj-row"
          class:on={isProject(p.id)}
          aria-current={isProject(p.id) ? "page" : undefined}
          href={links.href(projectRoute(p.displayId))}
          onclick={(e) => links.click(e, projectRoute(p.displayId))}
          onauxclick={(e) => links.click(e, projectRoute(p.displayId))}
        >
          <StatusDot category={projectCategory(p.status)} label={p.status} />
          <span>{p.title}</span>
        </a>
      {/each}
    {/if}
  {:else}
    <p class="side-empty">ワークスペースはまだありません</p>
  {/each}
</section>

<style>
  .side-sec {
    display: flex;
    flex-direction: column;
    gap: 1px;
    padding-top: 8px;
  }
  .side-head {
    display: flex;
    align-items: center;
    padding: 0 4px 4px;
  }
  .ws-row {
    display: flex;
    align-items: center;
    gap: 4px;
    min-height: 28px;
  }
  .twisty {
    width: 18px;
    height: 18px;
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
    transition: transform 0.12s;
  }
  .twisty.open {
    transform: rotate(90deg);
  }
  .ws-name,
  .proj-row {
    color: var(--ink);
    text-decoration: none;
    border-radius: 6px;
  }
  .ws-name {
    flex: 1;
    min-width: 0;
    font-weight: 500;
    padding: 3px 4px;
  }
  .proj-row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 6px 4px 26px;
    min-height: 28px;
  }
  .ws-name:hover,
  .proj-row:hover {
    background: var(--hover);
  }
  .on {
    background: var(--accent-faint);
    color: var(--accent-ink);
  }
  .prefix {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--muted);
  }
  .side-empty {
    color: var(--muted);
    font-size: 12px;
    margin: 4px 6px;
  }
</style>
```

`web/app/src/screens/workspace/WorkspaceScreen.svelte`(全体。見出しの右に「プロジェクト」のボタン):

```svelte
<script lang="ts">
  import { readOnlyBand, readOnlyNote, type StoreNode, type WorkspaceNode } from "@hedgehug/core";
  import { Button, PageHead, ReadOnlyBand, StoreBadge } from "@hedgehug/ui";
  import { projectDialog } from "../../wire";

  // ワークスペースの画面の見出しだけの枠(A2 の計画の持ち場)。タブの中身はトラック B・E・F が足す。
  interface Props {
    store: StoreNode;
    workspace: WorkspaceNode;
    ondrawer: () => void;
  }

  let { store, workspace, ondrawer }: Props = $props();
</script>

<PageHead title={workspace.name} {ondrawer}>
  {#snippet crumbs()}
    <StoreBadge name={store.name} color={store.color} />
    <code class="prefix">{workspace.prefix}</code>
  {/snippet}
  {#snippet actions()}
    <Button
      disabled={store.readOnly}
      title={store.readOnly ? readOnlyNote : undefined}
      onclick={() => void projectDialog.open(workspace.id)}
      >プロジェクト</Button
    >
  {/snippet}
</PageHead>
<div class="content">
  {#if store.readOnly}
    <ReadOnlyBand text={readOnlyBand(store.name)} />
  {/if}
</div>

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px 24px 40px;
    min-height: 0;
  }
  .prefix {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
</style>
```

`web/app/src/shell/Screen.svelte`(全体。プロジェクトの画面を足す):

```svelte
<script lang="ts">
  import type { Resolved, StoreNode } from "@hedgehug/core";
  import { PageHead } from "@hedgehug/ui";
  import ProjectScreen from "../screens/project/ProjectScreen.svelte";
  import WorkspaceScreen from "../screens/workspace/WorkspaceScreen.svelte";

  // いまの画面を出す。画面が替わるたびに作り直すので、本文は先頭から出る(01「画面の移り方」)。
  interface Props {
    current: Resolved | null;
    stores: StoreNode[];
    version: string | null;
    ondrawer: () => void;
  }

  let { current, stores, version, ondrawer }: Props = $props();

  const store = $derived(
    current && current.name !== "home" ? stores.find((s) => s.id === current.storeId) : undefined,
  );
  const workspace = $derived(
    current?.name === "workspace"
      ? store?.workspaces.find((w) => w.id === current.workspaceId)
      : undefined,
  );
</script>

{#key current}
  {#if current?.name === "project"}
    <ProjectScreen
      storeId={current.storeId}
      projectId={current.projectId}
      tab={current.tab}
      {ondrawer}
    />
  {:else if store && workspace}
    <WorkspaceScreen {store} {workspace} {ondrawer} />
  {:else}
    <!-- ホーム。マイタスク(トラック E)が入ったら替える(A2 の計画の決定 6)。 -->
    <PageHead title="hedgehug" {ondrawer} />
    <div class="content">
      {#if version}
        <p class="version">{version}</p>
      {/if}
    </div>
  {/if}
{/key}

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px 24px 40px;
    min-height: 0;
  }
  .version {
    color: var(--muted);
    font-family: var(--font-mono);
    margin: 0;
  }
</style>
```

`web/app/src/wire.ts`(全体):

```ts
import {
  createCatalogPort,
  createItemsPort,
  createProjectsPort,
  createSystemPort,
  rememberToken,
  setToken,
} from "@hedgehug/api";
import {
  createCatalog,
  createNavigation,
  createProjectDialog,
  createProjectScreen,
  createScreen,
  createShell,
  createSystemInfo,
  createToasts,
  get,
  routeOfProject,
} from "@hedgehug/core";
import { browserHistory } from "./history";
import { createLinks } from "./links";

// 組み立て: api の実装を core に渡す(設計書 6 節)。画面はここで作った状態を使う。

// トークンは、ほかの何よりも先に決める(アドレスからハッシュを消してから、履歴を読む)。A2 の計画の決定 5。
const token = rememberToken(location, sessionStorage, (url) => history.replaceState(null, "", url));
setToken(token);

export const shell = createShell();
export const system = createSystemInfo(createSystemPort());
export const navigation = createNavigation(browserHistory());
navigation.start();
export const links = createLinks(navigation, token);
export const toasts = createToasts({
  set: (fn, ms) => setTimeout(fn, ms),
  clear: (handle) => clearTimeout(handle as ReturnType<typeof setTimeout>),
});

const itemsPort = createItemsPort();
const catalogPort = createCatalogPort();
export const catalog = createCatalog(catalogPort);
export const projectScreen = createProjectScreen(catalog, catalogPort, itemsPort);
export const projectDialog = createProjectDialog({
  catalog,
  catalogPort,
  projects: createProjectsPort(),
  // 作ったら、そのプロジェクトの概要を開く。トーストは出さない(02「作成」)。
  onCreated: (storeId, project) =>
    navigation.go(routeOfProject(get(catalog.stores), storeId, project.id, "overview")),
});
export const screen = createScreen({
  navigation,
  catalog,
  items: itemsPort,
  // 詳細パネルは Task 10 で足す。それまでアイテムの入口は、プロジェクトのツリーを開くだけ。
  openItem: () => {},
});

// 画面を移ったら、サイドバーのドロワーを閉じる(01「画面の移り方」)。
navigation.route.subscribe(() => shell.closeDrawer());

// 一覧を読んでから、アドレスの解決を始める(一覧がないと Store 名と ID を解決できない)。
void catalog.load().then(() => screen.start());
```

`web/app/src/App.svelte`(全体):

```svelte
<script lang="ts">
  import { AppFrame, Toasts } from "@hedgehug/ui";
  import CreateProjectDialog from "./screens/project/CreateProjectDialog.svelte";
  import Screen from "./shell/Screen.svelte";
  import Sidebar from "./shell/Sidebar.svelte";
  import { catalog, links, screen, shell, system, toasts } from "./wire";

  const drawerOpen = shell.drawerOpen;
  const version = system.version;
  const toastList = toasts.list;
  const stores = catalog.stores;
  const current = screen.current;
  system.load();
</script>

<AppFrame drawerOpen={$drawerOpen} onclosedrawer={shell.closeDrawer}>
  {#snippet sidebar()}
    <Sidebar stores={$stores} current={$current} {links} />
  {/snippet}
  <Screen current={$current} stores={$stores} version={$version} ondrawer={shell.openDrawer} />
</AppFrame>
<CreateProjectDialog />
<Toasts toasts={$toastList} ondismiss={toasts.dismiss} />
```

- [ ] **Step 7: すべて通ることを確かめる**

Run: `cd web && pnpm lint && pnpm check && pnpm test && pnpm deps && pnpm size && cd .. && make e2e-bin && cd web && pnpm e2e`
Expected: PASS(e2e は 27 本)

- [ ] **Step 8: Commit**

```bash
git add web/packages/ui/src web/packages/core/src web/app/src web/e2e
git commit -m "feat(web): プロジェクトの画面(概要・ツリー)と作成ダイアログを足す(A2 Task 8)"
```

---

### Task 9: アイテムの作成ダイアログ

作成ダイアログ(種類・ワークスペース・プロジェクト・親・タイトル・ID の予告・続けて作成)と、その入口(サイドバーの「新規作成」、C キー、プロジェクトの見出しの「Epic」、概要の「Epic を追加」、ツリーの行の「+」)を足す。作ったあとに詳細パネルを開くのは Task 10。

**Files:**
- Create: `web/packages/core/src/features/items/createDefaults.ts`(+`createDefaults.test.ts`)
- Create: `web/packages/core/src/features/items/createDialog.ts`(+`createDialog.test.ts`)
- Modify: `web/packages/core/src/index.ts`
- Create: `web/app/src/screens/item/CreateItemDialog.svelte`、`web/app/src/screens/item/CreateItemFields.svelte`
- Modify: `web/app/src/shell/Sidebar.svelte`、`web/app/src/App.svelte`、`web/app/src/wire.ts`、`web/app/src/screens/project/ProjectScreen.svelte`、`ProjectOverview.svelte`、`ProjectTree.svelte`、`TreeRow.svelte`
- Create: `web/e2e/create.spec.ts`

**Interfaces:**
- Consumes: Task 5 の `Checkbox`・`createToasts`・`Toasts`。Task 6 の `ItemsPort`(`list`・`create`・`nextNumber`)。Task 8 の `Option`・`projectDialog`・`projectScreen`。
- Produces:
  - core `startValues(stores, ctx: CreateContext, here: Here): Start`、`parentCandidates(items, kind)`、`pickParent(candidates, wanted)`、`parentMissing(kind)`、`childKindOf(kind): Kind | null`(行の「+」の種類)。`CreateContext = { workspaceId?; projectId?; kind?; parentId? }`、`Here = { workspaceId?; projectId? }`。
  - core `createItemDialog({ catalog, items, toasts, here(): Here, openProjectDialog(workspaceId), onCreated(item, keptOpen) }): ItemDialog`。`ItemDialog = { state: Readable<ItemDialogState | null>; open(ctx?: CreateContext); update(patch: Partial<ItemForm>); submit(); close() }`。`ItemForm = { workspaceId; projectId; kind; parentId: string | null; title; keepOpen }`。`ItemDialogState = { form; workspaces; projects; parents: Option[]; parentMissing: string; preview: string; error: string }`。`kinds: Kind[]`。
  - app `wire.ts` の `itemDialog`。

- [ ] **Step 1: core の失敗するテストを書く**

`web/packages/core/src/features/items/createDefaults.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog, type StoreNode } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import {
  childKindOf,
  parentCandidates,
  parentMissing,
  pickParent,
  startValues,
} from "./createDefaults";
import { item } from "./fixtures.test-util";

async function stores(): Promise<StoreNode[]> {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  return get(catalog.stores);
}

describe("startValues", () => {
  it("takes the given place, skipping a read-only store's workspace", async () => {
    const s = await stores();
    expect(startValues(s, { workspaceId: "S1-W1", projectId: "S1-P2", kind: "Epic" }, {})).toEqual({
      ok: true,
      workspace: s[0]?.workspaces[0],
      projectId: "S1-P2",
      kind: "Epic",
      parentId: null,
    });
    const r = startValues(s, {}, { workspaceId: "S2-W1", projectId: "S2-P2" });
    expect(r).toMatchObject({ ok: true, projectId: "S1-P1", kind: "Task" });
  });

  it("uses the open project when it is in the workspace", async () => {
    const s = await stores();
    expect(startValues(s, {}, { workspaceId: "S1-W1", projectId: "S1-P2" })).toMatchObject({
      projectId: "S1-P2",
    });
  });

  it("refuses with the spec's words when nothing can be written", async () => {
    expect(startValues([], {}, {})).toEqual({
      ok: false,
      message: "先にワークスペースを作ってください",
    });
    const onlyRo = (await stores()).filter((x) => x.readOnly);
    expect(startValues(onlyRo, {}, {})).toEqual({
      ok: false,
      message: "アイテムを作成できる Store がありません(閲覧のみの Store だけです)",
    });
    const s = await stores();
    const empty = s.map((x) => ({
      ...x,
      workspaces: x.workspaces.map((w) => ({ ...w, projects: [] })),
    }));
    expect(startValues(empty, {}, {})).toMatchObject({
      ok: false,
      message: "Web にはプロジェクトがありません。先にプロジェクトを作ってください",
    });
  });
});

describe("parent candidates", () => {
  const epic = item("S1", "P", 1, "Epic", null);
  const task = item("S1", "P", 2, "Task", epic.id);
  const step = item("S1", "P", 3, "Step", task.id);

  it("are Epics for work items, work items for Steps, none for Epics", () => {
    const all = [epic, task, step];
    expect(parentCandidates(all, "Bug")).toEqual([epic]);
    expect(parentCandidates(all, "Step")).toEqual([task]);
    expect(parentCandidates(all, "Epic")).toEqual([]);
  });

  it("keeps the wanted parent if it can be one, else takes the first", () => {
    expect(pickParent([epic], epic.id)).toBe(epic.id);
    expect(pickParent([epic], task.id)).toBe(epic.id);
    expect(pickParent([], null)).toBeNull();
  });

  it("says what is missing", () => {
    expect(parentMissing("Step")).toBe(
      "このプロジェクトには Story・Task・Bug がありません。先に作ってください",
    );
    expect(parentMissing("Task")).toBe(
      "このプロジェクトには Epic がありません。先に作ってください",
    );
  });
});

describe("childKindOf", () => {
  it("is Story under an Epic, Step under work items, and none under a Step", () => {
    expect(childKindOf("Epic")).toBe("Story");
    expect(childKindOf("Bug")).toBe("Step");
    expect(childKindOf("Step")).toBeNull();
  });
});
```

`web/packages/core/src/features/items/createDialog.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import type { Item } from "../../model/item";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createItemDialog } from "./createDialog";
import { fakeItemsPort, item } from "./fixtures.test-util";

async function setup(here = {}) {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const epic = item("S1", "S1-P1", 1, "Epic", null, "認証");
  const task = item("S1", "S1-P1", 2, "Task", epic.id, "画面");
  const items = fakeItemsPort([epic, task]);
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const projectDialogs: string[] = [];
  const created: [Item, boolean][] = [];
  const dialog = createItemDialog({
    catalog,
    items,
    toasts,
    here: () => here,
    openProjectDialog: (ws) => void projectDialogs.push(ws),
    onCreated: (it, kept) => void created.push([it, kept]),
  });
  return { dialog, items, toasts, projectDialogs, created, epic, task };
}

describe("createItemDialog", () => {
  it("opens with a Task under the first Epic and the ID preview", async () => {
    const { dialog, epic } = await setup({ workspaceId: "S1-W1", projectId: "S1-P1" });
    await dialog.open();
    const s = get(dialog.state);
    expect(s?.form).toMatchObject({ projectId: "S1-P1", kind: "Task", parentId: epic.id });
    expect(s?.workspaces).toEqual([{ value: "S1-W1", label: "work / Web (HH)" }]);
    expect(s?.parents).toEqual([{ value: epic.id, label: "HH-1 認証" }]);
    expect(s?.preview).toBe("ID は HH-3 になります。");
  });

  it("re-picks the parent when the kind changes and keeps the title", async () => {
    const { dialog, task } = await setup({ workspaceId: "S1-W1", projectId: "S1-P1" });
    await dialog.open();
    await dialog.update({ title: "手順" });
    await dialog.update({ kind: "Step" });
    expect(get(dialog.state)?.form).toMatchObject({
      kind: "Step",
      parentId: task.id,
      title: "手順",
    });
    await dialog.update({ kind: "Epic" });
    expect(get(dialog.state)?.form.parentId).toBeNull();
    expect(get(dialog.state)?.parentMissing).toBe("");
  });

  it("shows the missing-parent words when there is no candidate", async () => {
    const { dialog } = await setup({ workspaceId: "S1-W1", projectId: "S1-P2" });
    await dialog.open();
    expect(get(dialog.state)?.parentMissing).toBe(
      "このプロジェクトには Epic がありません。先に作ってください",
    );
  });

  it("shows the server's refusal in the dialog", async () => {
    const { dialog } = await setup();
    await dialog.open({ kind: "Epic" });
    await dialog.submit();
    expect(get(dialog.state)?.error).toBe("タイトルを入力してください");
  });

  it("creates, toasts, and closes or keeps going", async () => {
    const { dialog, toasts, created } = await setup();
    await dialog.open({ kind: "Epic" });
    await dialog.update({ title: "決済", keepOpen: true });
    await dialog.submit();
    expect(get(toasts.list).map((t) => t.text)).toEqual(["HH-3 を作成しました"]);
    expect(get(dialog.state)?.form).toMatchObject({ title: "", keepOpen: true, kind: "Epic" });
    expect(get(dialog.state)?.preview).toBe("ID は HH-4 になります。");
    await dialog.update({ title: "通知", keepOpen: false });
    await dialog.submit();
    expect(get(dialog.state)).toBeNull();
    expect(created.map(([it, kept]) => [it.title, kept])).toEqual([
      ["決済", true],
      ["通知", false],
    ]);
  });

  it("does not open without a project and switches to the project dialog", async () => {
    const catalog = createCatalog({
      ...fakeCatalogPort(),
      listProjects: async () => [],
    });
    await catalog.load();
    const toasts = createToasts({ set: () => 0, clear: () => {} });
    const opened: string[] = [];
    const dialog = createItemDialog({
      catalog,
      items: fakeItemsPort(),
      toasts,
      here: () => ({}),
      openProjectDialog: (ws) => void opened.push(ws),
      onCreated: () => {},
    });
    await dialog.open();
    expect(get(dialog.state)).toBeNull();
    expect(get(toasts.list)[0]).toMatchObject({
      text: "Web にはプロジェクトがありません。先にプロジェクトを作ってください",
      error: true,
    });
    expect(opened).toEqual(["S1-W1"]);
  });
});
```

Run: `cd web/packages/core && npx vitest run src/features/items`
Expected: FAIL(`./createDefaults`・`./createDialog` がない)

- [ ] **Step 2: core を実装する**

`web/packages/core/src/features/items/createDefaults.ts`:

```ts
import type { Item, Kind } from "../../model/item";
import type { StoreNode, WorkspaceNode } from "../catalog/catalog";
import { writableWorkspaces } from "../catalog/catalog";

// アイテムの作成ダイアログの初期値と親の候補(02「開き方と初期値」「ダイアログの項目」)。

// CreateContext は、開いた場所が指定したもの(ボタン・行の「+」など)。
export interface CreateContext {
  workspaceId?: string;
  projectId?: string;
  kind?: Kind;
  parentId?: string | null;
}

// Here は、いま開いている画面のワークスペースとプロジェクト。
export interface Here {
  workspaceId?: string;
  projectId?: string;
}

export type Start =
  | { ok: true; workspace: WorkspaceNode; projectId: string; kind: Kind; parentId: string | null }
  | { ok: false; message: string; workspaceWithoutProject?: WorkspaceNode };

// startValues は、ワークスペース・プロジェクト・種類・親の初期値を決める。親は候補で選び直す(pickParent)。
// ワークスペースは、指定・いまのワークスペース・いまのプロジェクトのワークスペース・書ける先頭、の順で、
// 閲覧のみの Store のものは採らない。
export function startValues(stores: StoreNode[], ctx: CreateContext, here: Here): Start {
  const writable = writableWorkspaces(stores);
  const workspace =
    [ctx.workspaceId, here.workspaceId]
      .map((id) => writable.find((w) => w.id === id))
      .find((w) => w !== undefined) ?? writable[0];
  if (!workspace) {
    const onlyReadOnly = stores.length > 0 && stores.every((s) => s.readOnly);
    return {
      ok: false,
      message: onlyReadOnly
        ? "アイテムを作成できる Store がありません(閲覧のみの Store だけです)"
        : "先にワークスペースを作ってください",
    };
  }
  const inWs = (id?: string) => workspace.projects.find((p) => p.id === id)?.id;
  const projectId = inWs(ctx.projectId) ?? inWs(here.projectId) ?? workspace.projects[0]?.id;
  if (!projectId) {
    return {
      ok: false,
      message: `${workspace.name} にはプロジェクトがありません。先にプロジェクトを作ってください`,
      workspaceWithoutProject: workspace,
    };
  }
  return {
    ok: true,
    workspace,
    projectId,
    kind: ctx.kind ?? "Task",
    parentId: ctx.parentId ?? null,
  };
}

// parentCandidates は、親にできるもの。Step は Story・Task・Bug、Epic はなし、それ以外は Epic(番号の順)。
export function parentCandidates(items: Item[], kind: Kind): Item[] {
  if (kind === "Epic") return [];
  if (kind === "Step") return items.filter((it) => ["Story", "Task", "Bug"].includes(it.kind));
  return items.filter((it) => it.kind === "Epic");
}

// pickParent は、wanted が候補にあればそれ、なければ候補の先頭。Epic と候補がないときは null。
export function pickParent(candidates: Item[], wanted: string | null): string | null {
  if (wanted && candidates.some((it) => it.id === wanted)) return wanted;
  return candidates[0]?.id ?? null;
}

// parentMissing は、親の候補がないときに、セレクトの代わりに出す赤字(02「ダイアログの項目」)。
export function parentMissing(kind: Kind): string {
  return kind === "Step"
    ? "このプロジェクトには Story・Task・Bug がありません。先に作ってください"
    : "このプロジェクトには Epic がありません。先に作ってください";
}

// childKindOf は、行の「+」で作る種類(その行の下に置ける最初の種類。04「ツリー」の「行」)。Step は null。
export function childKindOf(kind: Kind): Kind | null {
  if (kind === "Epic") return "Story";
  if (kind === "Step") return null;
  return "Step";
}
```

`web/packages/core/src/features/items/createDialog.ts`:

```ts
import { Refused } from "../../model/errors";
import type { Item, Kind } from "../../model/item";
import type { ItemsPort } from "../../port/items";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import type { Option } from "../projects/createDialog";
import { itemDisplayId } from "../router/route";
import type { Toasts } from "../toasts/toasts";
import {
  type CreateContext,
  type Here,
  parentCandidates,
  parentMissing,
  pickParent,
  startValues,
} from "./createDefaults";

// アイテムの作成ダイアログ(02「アイテムの作成」のうち A2 の分)。検査はサーバーに任せ、断りを赤字で出す
// (A2 の計画の決定 14)。テンプレート・プロパティ・スプリント・説明は A3。

export interface ItemForm {
  workspaceId: string;
  projectId: string;
  kind: Kind;
  parentId: string | null;
  title: string;
  keepOpen: boolean;
}

export interface ItemDialogState {
  form: ItemForm;
  workspaces: Option[];
  projects: Option[];
  parents: Option[];
  // parentMissing は、親の候補がないときの赤字。Epic と、候補があるときは空。
  parentMissing: string;
  // preview は「ID は HH-13 になります。」。
  preview: string;
  error: string;
}

export interface ItemDialog {
  state: Readable<ItemDialogState | null>;
  open(ctx?: CreateContext): Promise<void>;
  update(patch: Partial<ItemForm>): Promise<void>;
  submit(): Promise<void>;
  close(): void;
}

export interface ItemDialogDeps {
  catalog: Catalog;
  items: ItemsPort;
  toasts: Toasts;
  here(): Here;
  openProjectDialog(workspaceId: string): void;
  // onCreated は、作ったあと。keptOpen は「続けて作成」がオンだったか。
  onCreated(item: Item, keptOpen: boolean): void;
}

export const kinds: Kind[] = ["Epic", "Story", "Task", "Bug", "Step"];

export function createItemDialog(deps: ItemDialogDeps): ItemDialog {
  const state = writable<ItemDialogState | null>(null);
  // projectItems は、選んでいるプロジェクトのアイテム(親の候補を作る)。
  let projectItems: Item[] = [];

  const stores = () => get(deps.catalog.stores);
  const storeOf = (wsId: string) => stores().find((s) => s.workspaces.some((w) => w.id === wsId));
  const wsOf = (wsId: string) => storeOf(wsId)?.workspaces.find((w) => w.id === wsId);
  const wsOptions = () =>
    stores()
      .filter((s) => !s.readOnly)
      .flatMap((s) =>
        s.workspaces.map((w) => ({ value: w.id, label: `${s.name} / ${w.name} (${w.prefix})` })),
      );

  const parentOptions = (prefix: string, candidates: Item[]) =>
    candidates.map((it) => ({
      value: it.id,
      label: `${itemDisplayId(prefix, it.number)} ${it.title}`,
    }));
  const missing = (kind: Kind, candidates: Item[]) =>
    kind !== "Epic" && candidates.length === 0 ? parentMissing(kind) : "";

  // refresh は、ワークスペース・プロジェクト・種類から、選択肢・親・ID の予告を作り直す。
  async function refresh(form: ItemForm, error = ""): Promise<ItemDialogState | null> {
    const store = storeOf(form.workspaceId);
    const ws = wsOf(form.workspaceId);
    if (!store || !ws) return null;
    const [list, next] = await Promise.all([
      form.projectId ? deps.items.list(store.id, form.projectId) : Promise.resolve([]),
      deps.items.nextNumber(store.id, ws.id),
    ]);
    projectItems = list;
    const candidates = parentCandidates(list, form.kind);
    const parentId = form.kind === "Epic" ? null : pickParent(candidates, form.parentId);
    return {
      form: { ...form, parentId },
      workspaces: wsOptions(),
      projects: ws.projects.map((p) => ({ value: p.id, label: p.title })),
      parents: parentOptions(ws.prefix, candidates),
      parentMissing: missing(form.kind, candidates),
      preview: `ID は ${itemDisplayId(ws.prefix, next)} になります。`,
      error,
    };
  }

  return {
    state: readonly(state),
    async open(ctx = {}) {
      const start = startValues(stores(), ctx, deps.here());
      if (!start.ok) {
        deps.toasts.show(start.message, { error: true });
        if (start.workspaceWithoutProject) deps.openProjectDialog(start.workspaceWithoutProject.id);
        return;
      }
      const form: ItemForm = {
        workspaceId: start.workspace.id,
        projectId: start.projectId,
        kind: start.kind,
        parentId: start.parentId,
        title: "",
        keepOpen: false,
      };
      state.set(await refresh(form));
    },
    async update(patch) {
      const s = get(state);
      if (!s) return;
      const form = { ...s.form, ...patch };
      if (patch.workspaceId !== undefined && patch.workspaceId !== s.form.workspaceId) {
        // ワークスペースを変えたら、プロジェクトを先頭にし、親を選び直す(02「ダイアログの中での連動」)。
        form.projectId = wsOf(patch.workspaceId)?.projects[0]?.id ?? "";
        state.set(await refresh(form, s.error));
        return;
      }
      if (patch.projectId !== undefined && patch.projectId !== s.form.projectId) {
        state.set(await refresh(form, s.error));
        return;
      }
      if (patch.kind !== undefined && patch.kind !== s.form.kind) {
        // 種類を変えたら、親を新しい種類の候補から選び直す。タイトルは残す。
        const candidates = parentCandidates(projectItems, form.kind);
        const ws = wsOf(form.workspaceId);
        form.parentId = form.kind === "Epic" ? null : pickParent(candidates, form.parentId);
        state.set({
          ...s,
          form,
          parents: parentOptions(ws?.prefix ?? "", candidates),
          parentMissing: missing(form.kind, candidates),
        });
        return;
      }
      state.set({ ...s, form });
    },
    async submit() {
      const s = get(state);
      const store = s && storeOf(s.form.workspaceId);
      const ws = s && wsOf(s.form.workspaceId);
      if (!s || !store || !ws) return;
      const { projectId, kind, title } = s.form;
      const parentId = kind === "Epic" ? null : s.form.parentId;
      let created: Item;
      try {
        created = await deps.items.create(store.id, { projectId, parentId, kind, title });
      } catch (e) {
        if (!(e instanceof Refused)) throw e;
        state.set({ ...s, error: e.message });
        return;
      }
      deps.toasts.show(`${itemDisplayId(ws.prefix, created.number)} を作成しました`);
      if (s.form.keepOpen) {
        // 続けて作成: タイトルとエラーだけを空にし、ほかは残す(02「作成したあと」)。
        state.set(await refresh({ ...s.form, title: "" }));
      } else {
        state.set(null);
      }
      deps.onCreated(created, s.form.keepOpen);
    },
    close() {
      state.set(null);
    },
  };
}
```

`web/packages/core/src/index.ts` に足す(`./features/dates/date` の export の次):

```ts
export { type CreateContext, childKindOf, type Here } from "./features/items/createDefaults";
export {
  createItemDialog,
  type ItemDialog,
  type ItemDialogState,
  type ItemForm,
  kinds,
} from "./features/items/createDialog";
```

Run: `cd web/packages/core && npx tsc -p . && npx vitest run`
Expected: PASS

- [ ] **Step 3: 画面の失敗するテストを書く**

`web/e2e/create.spec.ts`:

```ts
import { expect, test } from "@playwright/test";
import { tokenUrl, unique } from "./helpers";

test("creates an Epic, a Task under it and a Step from the project screen", async ({
  page,
}, info) => {
  const epic = unique(info, "通知");
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/overview"));
  await page.getByRole("button", { name: "Epic", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await expect(dialog.getByRole("combobox", { name: "種類" })).toHaveValue("Epic");
  await expect(dialog.getByText(/^ID は HH-\d+ になります。$/)).toBeVisible();
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(dialog.getByText("タイトルを入力してください")).toBeVisible();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(epic);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(dialog).toBeHidden();
  await expect(page.getByText(/^HH-\d+ を作成しました$/)).toBeVisible();
  await expect(page.getByRole("region", { name: "Epic" }).getByText(epic)).toBeVisible();

  await page.getByRole("tab", { name: "ツリー" }).click();
  const tree = page.getByRole("list", { name: "ツリー" });
  await tree
    .getByRole("listitem")
    .filter({ hasText: epic })
    .getByRole("button", { name: "Story を追加" })
    .click();
  await expect(dialog.getByRole("combobox", { name: "種類" })).toHaveValue("Story");
  await dialog.getByRole("combobox", { name: "種類" }).selectOption("Task");
  await expect(dialog.getByRole("combobox", { name: "親" })).toHaveText(new RegExp(epic));
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(`${epic} の Task`);
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(tree.getByText(`${epic} の Task`)).toBeVisible();

  await tree
    .getByRole("listitem")
    .filter({ hasText: `${epic} の Task` })
    .getByRole("button", { name: "Step を追加" })
    .click();
  await dialog.getByRole("checkbox", { name: "続けて作成" }).check();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(`${epic} の手順`);
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toHaveValue("");
  await expect(dialog.getByRole("combobox", { name: "種類" })).toHaveValue("Step");
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(tree.getByText(`${epic} の手順`)).toBeVisible();
});

test("the C key opens the dialog with a Task in the open project", async ({ page }) => {
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/tree"));
  await expect(page.getByRole("heading", { name: "HH-P1 ログイン" })).toBeVisible();
  await page.keyboard.press("c");
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await expect(dialog.getByRole("combobox", { name: "種類" })).toHaveValue("Task");
  await expect(dialog.getByRole("combobox", { name: "プロジェクト" })).toHaveText(/ログイン/);
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toBeFocused();
  // 入力中の C は文字になり、ダイアログを開き直さない。
  await page.keyboard.press("c");
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toHaveValue("c");
});

test("a read-only store has no enabled create buttons", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/p/HH-P1/tree"));
  await expect(page.getByRole("button", { name: "Epic", exact: true })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Story を追加" })).toBeDisabled();
});
```

Run: `make e2e-bin && cd web && pnpm e2e create.spec`
Expected: FAIL(「Epic」のボタンがない)

- [ ] **Step 4: 画面を実装する**

`web/app/src/screens/item/CreateItemFields.svelte`:

```svelte
<script lang="ts">
  import type { ItemDialogState, Kind } from "@hedgehug/core";
  import { kinds } from "@hedgehug/core";
  import { Input, Select } from "@hedgehug/ui";
  import { itemDialog } from "../../wire";

  // 作成ダイアログの項目(02「ダイアログの項目」のうち A2 の分: 種類・ワークスペース・プロジェクト・親・タイトル)。
  interface Props {
    s: ItemDialogState;
  }

  let { s }: Props = $props();

  const kindOptions = kinds.map((k) => ({ value: k, label: k }));

  function onkeydown(event: KeyboardEvent) {
    // Enter で作成する(02)。日本語の変換を確定する Enter は除く。
    if (event.key === "Enter" && !event.isComposing) void itemDialog.submit();
  }
</script>

<div class="grid">
  <div class="field">
    <span>種類</span>
    <Select
      label="種類"
      options={kindOptions}
      bind:value={() => s.form.kind, (v) => void itemDialog.update({ kind: v as Kind })}
    />
  </div>
  <div class="field">
    <span>ワークスペース</span>
    <Select
      label="ワークスペース"
      options={s.workspaces}
      bind:value={() => s.form.workspaceId, (v) => void itemDialog.update({ workspaceId: v })}
    />
  </div>
  <div class="field">
    <span>プロジェクト</span>
    <Select
      label="プロジェクト"
      options={s.projects}
      bind:value={() => s.form.projectId, (v) => void itemDialog.update({ projectId: v })}
    />
  </div>
  {#if s.form.kind !== "Epic"}
    <div class="field">
      <span>親</span>
      {#if s.parentMissing}
        <p class="form-error">{s.parentMissing}</p>
      {:else}
        <Select
          label="親"
          options={s.parents}
          bind:value={() => s.form.parentId ?? "", (v) => void itemDialog.update({ parentId: v })}
        />
      {/if}
    </div>
  {/if}
</div>
<div class="field">
  <span>タイトル</span>
  <Input
    label="タイトル"
    placeholder="{s.form.kind} のタイトル"
    focusOnMount
    {onkeydown}
    bind:value={() => s.form.title, (v) => void itemDialog.update({ title: v })}
  />
</div>

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: 12px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .field > span {
    font-size: 12.5px;
    color: var(--muted);
  }
  .form-error {
    color: var(--danger);
    margin: 0;
    font-size: 13px;
  }
</style>
```

`web/app/src/screens/item/CreateItemDialog.svelte`:

```svelte
<script lang="ts">
  import { Button, Checkbox, Dialog } from "@hedgehug/ui";
  import { itemDialog } from "../../wire";
  import CreateItemFields from "./CreateItemFields.svelte";

  // アイテムの作成ダイアログ(02「アイテムの作成」)。値と送信は core の itemDialog が持つ。
  const dialog = itemDialog.state;
</script>

{#if $dialog}
  {@const s = $dialog}
  <Dialog open title="新規作成" wide onclose={itemDialog.close}>
    <div class="form">
      <CreateItemFields {s} />
      <p class="preview">{s.preview}</p>
      {#if s.error}
        <p class="form-error" role="alert">{s.error}</p>
      {/if}
    </div>
    {#snippet footer()}
      <Checkbox
        label="続けて作成"
        bind:checked={() => s.form.keepOpen, (v) => void itemDialog.update({ keepOpen: v })}
      />
      <span class="grow"></span>
      <Button onclick={itemDialog.close}>キャンセル</Button>
      <Button variant="primary" onclick={() => void itemDialog.submit()}>作成</Button>
    {/snippet}
  </Dialog>
{/if}

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .preview {
    margin: 0;
    color: var(--muted);
    font-size: 12.5px;
  }
  .form-error {
    color: var(--danger);
    margin: 0;
    font-size: 13px;
  }
  .grow {
    flex: 1;
  }
</style>
```

`web/app/src/shell/Sidebar.svelte`(全体。名前の下に「新規作成」):

```svelte
<script lang="ts">
  import type { Resolved, StoreNode } from "@hedgehug/core";
  import { Brand, Button } from "@hedgehug/ui";
  import { SvelteSet } from "svelte/reactivity";
  import type { Links } from "../links";
  import { itemDialog } from "../wire";
  import SidebarStore from "./SidebarStore.svelte";

  // サイドバー(04「サイドバー」)。A2 は、名前・「新規作成」・Store ごとのブロックだけを出す。
  interface Props {
    stores: StoreNode[];
    current: Resolved | null;
    links: Links;
  }

  let { stores, current, links }: Props = $props();

  // 開閉は、このタブのあいだだけ覚える(このマシンに残すのはトラック G のマシンの設定が入ってから)。
  const collapsed = new SvelteSet<string>();
  function toggle(id: string) {
    if (collapsed.has(id)) collapsed.delete(id);
    else collapsed.add(id);
  }
</script>

<Brand />
<Button variant="primary" onclick={() => void itemDialog.open()}>
  新規作成 <kbd class="kbd">C</kbd>
</Button>
{#each stores as store (store.id)}
  <SidebarStore {store} {current} {links} {collapsed} ontoggle={toggle} />
{/each}

<style>
  .kbd {
    font-family: var(--font-mono);
    font-size: 11px;
    opacity: 0.75;
    margin-left: 4px;
  }
</style>
```

`web/app/src/screens/project/ProjectScreen.svelte`(全体。見出しの右に「Epic」):

```svelte
<script lang="ts">
  import {
    type ProjectTab,
    projectCategory,
    type Route,
    readOnlyBand,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button, PageHead, ReadOnlyBand, StatusDot, StoreBadge, Tabs } from "@hedgehug/ui";
  import { itemDialog, links, navigation, projectScreen } from "../../wire";
  import ProjectOverview from "./ProjectOverview.svelte";
  import ProjectTree from "./ProjectTree.svelte";

  // プロジェクトの画面(02「画面の見出しと構成」)。タブは「概要」「ツリー」だけ(A2 の計画の決定 15)。
  interface Props {
    storeId: string;
    projectId: string;
    tab: ProjectTab;
    ondrawer: () => void;
  }

  let { storeId, projectId, tab, ondrawer }: Props = $props();

  const view = projectScreen.view;
  $effect(() => {
    void projectScreen.load(storeId, projectId);
  });

  const tabs = [
    { id: "overview", label: "概要" },
    { id: "tree", label: "ツリー" },
  ];
  const wsRoute = $derived<Route | null>(
    $view ? { name: "workspace", store: $view.store.name, prefix: $view.workspace.prefix } : null,
  );
  function select(id: string) {
    if (!$view) return;
    const next = id === "tree" ? "tree" : "overview";
    navigation.go({
      name: "project",
      store: $view.store.name,
      project: $view.project.displayId,
      tab: next,
    });
  }
  // addEpic は、このプロジェクトを親にした Epic の作成ダイアログを開く(02「画面の見出しと構成」)。
  function addEpic() {
    if (!$view) return;
    void itemDialog.open({
      workspaceId: $view.workspace.id,
      projectId: $view.project.id,
      kind: "Epic",
    });
  }
</script>

{#if $view && $view.store.id === storeId && $view.project.id === projectId && wsRoute}
  {@const p = $view.project}
  <PageHead title="{p.displayId} {p.title}" {ondrawer}>
    {#snippet crumbs()}
      <StoreBadge name={$view.store.name} color={$view.store.color} />
      <a
        class="crumb"
        href={links.href(wsRoute)}
        onclick={(e) => links.click(e, wsRoute)}
        onauxclick={(e) => links.click(e, wsRoute)}
        >{$view.workspace.name}</a
      >
    {/snippet}
    {#snippet actions()}
      <span class="status-pill">
        <StatusDot category={projectCategory(p.status)} label={p.status} />{p.status}
      </span>
      <Button
        variant="primary"
        disabled={$view.store.readOnly}
        title={$view.store.readOnly ? readOnlyNote : undefined}
        onclick={addEpic}
        ><span aria-hidden="true">+</span>
        Epic</Button
      >
    {/snippet}
  </PageHead>
  <Tabs {tabs} current={tab} onselect={select} />
  <div class="content">
    {#if $view.store.readOnly}
      <ReadOnlyBand text={readOnlyBand($view.store.name)} />
    {/if}
    {#if tab === "tree"}
      <ProjectTree view={$view} />
    {:else}
      <ProjectOverview view={$view} onaddepic={addEpic} />
    {/if}
  </div>
{/if}

<style>
  .content {
    flex: 1;
    overflow: auto;
    padding: 16px 24px 40px;
    min-height: 0;
  }
  .crumb {
    color: var(--muted);
    text-decoration: none;
  }
  .crumb:hover {
    color: var(--ink);
    text-decoration: underline;
  }
  .status-pill {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 2px 10px;
    border: 1px solid var(--line);
    border-radius: 999px;
    font-size: 12.5px;
  }
</style>
```

`web/app/src/screens/project/ProjectOverview.svelte`(全体。「Epic」の区画の見出しに「Epic を追加」):

```svelte
<script lang="ts">
  import {
    formatDay,
    itemDisplayId,
    ownerName,
    type ProjectView,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button, KindIcon } from "@hedgehug/ui";

  // プロジェクトの「概要」のタブ。A2 では「Epic」の区画と、プロパティの表示だけ(A2 の計画の決定 16)。
  interface Props {
    view: ProjectView;
    onaddepic: () => void;
  }

  let { view, onaddepic }: Props = $props();

  const year = new Date().getFullYear();
  const p = $derived(view.project);
</script>

<div class="overview-frame">
  <div class="overview">
    <section class="card-sec" aria-label="Epic">
      <div class="sec-head">
        <h2>Epic</h2>
        <span class="grow"></span>
        <Button
          size="sm"
          disabled={view.store.readOnly}
          title={view.store.readOnly ? readOnlyNote : undefined}
          onclick={onaddepic}
          ><span aria-hidden="true">+</span>
          Epic を追加</Button
        >
      </div>
      {#each view.epics as e (e.id)}
        <div class="list-row">
          <KindIcon kind="Epic" />
          <code class="id">{itemDisplayId(view.workspace.prefix, e.number)}</code>
          <span class="row-title">{e.title}</span>
        </div>
      {/each}
    </section>
    <section class="props" aria-label="プロパティ">
      <div class="prop"><span class="prop-k">ステータス</span><span>{p.status}</span></div>
      <div class="prop">
        <span class="prop-k">オーナー</span><span>{ownerName(p.owner, view.members)}</span>
      </div>
      <div class="prop">
        <span class="prop-k">開始日</span><span>{formatDay(p.start, year)}</span>
      </div>
      <div class="prop">
        <span class="prop-k">終了日</span><span>{formatDay(p.end, year)}</span>
      </div>
    </section>
  </div>
</div>

<style>
  .overview-frame {
    container: overview / inline-size;
  }
  .overview {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 260px;
    gap: 24px;
  }
  @container overview (max-width: 640px) {
    .overview {
      grid-template-columns: minmax(0, 1fr);
    }
  }
  .sec-head {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .grow {
    flex: 1;
  }
  h2 {
    font-size: 14px;
    margin: 0 0 8px;
  }
  .list-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    border-bottom: 1px solid var(--line);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    flex: 1;
    min-width: 0;
  }
  .props {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .prop {
    display: grid;
    grid-template-columns: 90px minmax(0, 1fr);
    align-items: center;
  }
  .prop-k {
    color: var(--muted);
    font-size: 12.5px;
  }
</style>
```

`web/app/src/screens/project/TreeRow.svelte`(全体。行の「+」):

```svelte
<script lang="ts">
  import {
    childKindOf,
    type Item,
    itemDisplayId,
    readOnlyNote,
    type TreeNode,
  } from "@hedgehug/core";
  import { IconButton, KindIcon } from "@hedgehug/ui";
  import Self from "./TreeRow.svelte";

  // ツリーの 1 行と、その下の行(04「ツリー」の「行」のうち、A2 の計画の決定 17 の分)。
  interface Props {
    node: TreeNode;
    prefix: string;
    depth: number;
    collapsed: Set<string>;
    readOnly: boolean;
    ontoggle: (id: string) => void;
    // onadd は、行の「+」(その行を親にした作成ダイアログ)。
    onadd: (parent: Item) => void;
  }

  let { node, prefix, depth, collapsed, readOnly, ontoggle, onadd }: Props = $props();

  const it = $derived(node.item);
  const open = $derived(!collapsed.has(it.id));
  const child = $derived(childKindOf(it.kind));
</script>

<li class="tree-row" style:--d={depth}>
  {#if node.children.length > 0}
    <button
      type="button"
      class="twisty"
      class:open
      aria-label="開閉"
      aria-expanded={open}
      onclick={() => ontoggle(it.id)}
    >
      ›
    </button>
  {:else}
    <span class="twisty-space"></span>
  {/if}
  <KindIcon kind={it.kind} />
  <code class="id">{itemDisplayId(prefix, it.number)}</code>
  <span class="row-title">{it.title}</span>
  {#if child}
    <IconButton
      label="{child} を追加"
      size="xs"
      disabled={readOnly}
      title={readOnly ? readOnlyNote : undefined}
      onclick={() => onadd(it)}
      >+</IconButton
    >
  {/if}
</li>
{#if open}
  {#each node.children as c (c.item.id)}
    <Self node={c} {prefix} depth={depth + 1} {collapsed} {readOnly} {ontoggle} {onadd} />
  {/each}
{/if}

<style>
  .tree-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    padding: 2px 8px 2px calc(8px + var(--d) * 22px);
    border-bottom: 1px solid var(--line);
  }
  .twisty,
  .twisty-space {
    width: 18px;
    height: 18px;
    flex: none;
  }
  .twisty {
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
    transition: transform 0.12s;
  }
  .twisty.open {
    transform: rotate(90deg);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
```

`web/app/src/screens/project/ProjectTree.svelte`(全体):

```svelte
<script lang="ts">
  import { childKindOf, type Item, type ProjectView } from "@hedgehug/core";
  import { SvelteSet } from "svelte/reactivity";
  import { itemDialog } from "../../wire";
  import TreeRow from "./TreeRow.svelte";

  // プロジェクトの「ツリー」のタブ。Epic を一番上の階層に並べる(04「ツリー」の「行」)。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  // 開閉は、画面を出しているあいだだけ覚える。初めはすべて開いている。
  const collapsed = new SvelteSet<string>();
  function toggle(id: string) {
    if (collapsed.has(id)) collapsed.delete(id);
    else collapsed.add(id);
  }
  function add(parent: Item) {
    const kind = childKindOf(parent.kind);
    if (!kind) return;
    void itemDialog.open({
      workspaceId: parent.workspaceId,
      projectId: parent.projectId,
      kind,
      parentId: parent.id,
    });
  }
</script>

<ul class="tree" aria-label="ツリー">
  {#each view.tree as node (node.item.id)}
    <TreeRow
      {node}
      prefix={view.workspace.prefix}
      depth={0}
      {collapsed}
      readOnly={view.store.readOnly}
      ontoggle={toggle}
      onadd={add}
    />
  {/each}
</ul>

<style>
  .tree {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
  }
</style>
```

`web/app/src/wire.ts`(全体):

```ts
import {
  createCatalogPort,
  createItemsPort,
  createProjectsPort,
  createSystemPort,
  rememberToken,
  setToken,
} from "@hedgehug/api";
import {
  createCatalog,
  createItemDialog,
  createNavigation,
  createProjectDialog,
  createProjectScreen,
  createScreen,
  createShell,
  createSystemInfo,
  createToasts,
  get,
  type Here,
  routeOfProject,
} from "@hedgehug/core";
import { browserHistory } from "./history";
import { createLinks } from "./links";

// 組み立て: api の実装を core に渡す(設計書 6 節)。画面はここで作った状態を使う。

// トークンは、ほかの何よりも先に決める(アドレスからハッシュを消してから、履歴を読む)。A2 の計画の決定 5。
const token = rememberToken(location, sessionStorage, (url) => history.replaceState(null, "", url));
setToken(token);

export const shell = createShell();
export const system = createSystemInfo(createSystemPort());
export const navigation = createNavigation(browserHistory());
navigation.start();
export const links = createLinks(navigation, token);
export const toasts = createToasts({
  set: (fn, ms) => setTimeout(fn, ms),
  clear: (handle) => clearTimeout(handle as ReturnType<typeof setTimeout>),
});

const itemsPort = createItemsPort();
const catalogPort = createCatalogPort();
export const catalog = createCatalog(catalogPort);
export const projectScreen = createProjectScreen(catalog, catalogPort, itemsPort);
export const projectDialog = createProjectDialog({
  catalog,
  catalogPort,
  projects: createProjectsPort(),
  // 作ったら、そのプロジェクトの概要を開く。トーストは出さない(02「作成」)。
  onCreated: (storeId, project) =>
    navigation.go(routeOfProject(get(catalog.stores), storeId, project.id, "overview")),
});
export const screen = createScreen({
  navigation,
  catalog,
  items: itemsPort,
  // 詳細パネルは Task 10 で足す。それまでアイテムの入口は、プロジェクトのツリーを開くだけ。
  openItem: () => {},
});

// here は、作成ダイアログの初期値に使う、いま開いている画面のワークスペースとプロジェクト(02)。
function here(): Here {
  const c = get(screen.current);
  if (c?.name === "workspace") return { workspaceId: c.workspaceId };
  if (c?.name === "project") {
    return {
      workspaceId: catalog.project(c.storeId, c.projectId)?.workspaceId,
      projectId: c.projectId,
    };
  }
  return {};
}

export const itemDialog = createItemDialog({
  catalog,
  items: itemsPort,
  toasts,
  here,
  openProjectDialog: (workspaceId) => void projectDialog.open(workspaceId),
  onCreated: () => void projectScreen.reload(),
});

// 画面を移ったら、サイドバーのドロワーを閉じる(01「画面の移り方」)。
navigation.route.subscribe(() => shell.closeDrawer());

// 一覧を読んでから、アドレスの解決を始める(一覧がないと Store 名と ID を解決できない)。
void catalog.load().then(() => screen.start());
```

`web/app/src/App.svelte`(全体。C キー):

```svelte
<script lang="ts">
  import { AppFrame, Toasts } from "@hedgehug/ui";
  import CreateItemDialog from "./screens/item/CreateItemDialog.svelte";
  import CreateProjectDialog from "./screens/project/CreateProjectDialog.svelte";
  import Screen from "./shell/Screen.svelte";
  import Sidebar from "./shell/Sidebar.svelte";
  import { catalog, itemDialog, links, screen, shell, system, toasts } from "./wire";

  const drawerOpen = shell.drawerOpen;
  const version = system.version;
  const toastList = toasts.list;
  const stores = catalog.stores;
  const current = screen.current;
  system.load();

  // C キーで作成ダイアログを開く。入力中とダイアログを開いているときは使えない(02「開き方と初期値」)。
  function onkeydown(event: KeyboardEvent) {
    if (event.key.toLowerCase() !== "c" || event.ctrlKey || event.metaKey || event.altKey) return;
    const t = event.target as HTMLElement | null;
    const typing = t?.closest("input, textarea, select, [contenteditable='true']");
    if (typing || document.querySelector('[aria-modal="true"]')) return;
    event.preventDefault();
    void itemDialog.open();
  }
</script>

<svelte:window {onkeydown} />

<AppFrame drawerOpen={$drawerOpen} onclosedrawer={shell.closeDrawer}>
  {#snippet sidebar()}
    <Sidebar stores={$stores} current={$current} {links} />
  {/snippet}
  <Screen current={$current} stores={$stores} version={$version} ondrawer={shell.openDrawer} />
</AppFrame>
<CreateProjectDialog />
<CreateItemDialog />
<Toasts toasts={$toastList} ondismiss={toasts.dismiss} />
```

- [ ] **Step 5: すべて通ることを確かめる**

Run: `cd web && pnpm lint && pnpm check && pnpm test && pnpm deps && pnpm size && cd .. && make e2e-bin && cd web && pnpm e2e`
Expected: PASS(e2e は 36 本)

- [ ] **Step 6: Commit**

```bash
git add web/packages/core/src web/app/src web/e2e
git commit -m "feat(web): アイテムの作成ダイアログと、その入口を足す(A2 Task 9)"
```

---

### Task 10: 詳細パネル

詳細パネル(ヘッダー・種類の行・タイトル・子と Step・GitHub の区画の枠)を足す。行のタイトル(ツリー・概要の Epic・子)とアイテムの入口で開き、作成ダイアログで作ったあと(「続けて作成」がオフ)にも開く。画面を移ると閉じる。

**Files:**
- Create: `web/packages/core/src/features/items/detail.ts`(+`detail.test.ts`)
- Modify: `web/packages/core/src/index.ts`
- Modify: `web/app/src/links.ts`(`click` に 3 つめの引数 `action`)
- Create: `web/app/src/screens/item/DetailPanel.svelte`、`DetailHeader.svelte`、`DetailChildren.svelte`
- Modify: `web/app/src/screens/project/ProjectOverview.svelte`、`ProjectTree.svelte`、`TreeRow.svelte`、`web/app/src/wire.ts`、`web/app/src/App.svelte`
- Create: `web/e2e/detail.spec.ts`
- Modify: `web/e2e/create.spec.ts`

**Interfaces:**
- Consumes: Task 5 の `SidePanel`(`AppFrame` の中に置く。`open`・`full`・`label`・`onclose`・`head`)・`IconButton`・`KindIcon`・`ReadOnlyBand`(`inset`)。Task 6 の `ItemsPort`(`get`・`list`・`rename`・`addStep`・`moveUp`)・`ancestorsOf`。Task 9 の `itemDialog`。
- Produces:
  - core `createDetail({ catalog, items, toasts, onChanged() }): Detail`。`Detail = { view: Readable<DetailView | null>; full: Readable<boolean>; open(storeId, itemId); close(); toggleFull(); rename(title): Promise<boolean>; addStep(title): Promise<boolean>; moveUp(itemId) }`。`DetailView = { store; workspace; project; item; displayId; ancestors: Item[]; children: Item[]; readOnly }`。
  - core `normalizeTitle(s)`、`copiedMessage(id)`、`copyFailedMessage(id)`。
  - app `links.click(event, route, action?)`: ふつうのクリックで `action` を呼ぶ(なければ画面を移る)。新しいタブで開く操作はこれまでどおり。
  - app `wire.ts` の `detail`。

- [ ] **Step 1: core の失敗するテストを書く**

`web/packages/core/src/features/items/detail.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { copiedMessage, copyFailedMessage, createDetail, normalizeTitle } from "./detail";
import { fakeItemsPort, item } from "./fixtures.test-util";

async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const epic = item("S1", "S1-P1", 1, "Epic", null, "認証");
  const task = item("S1", "S1-P1", 2, "Task", epic.id, "画面");
  const s1 = { ...item("S1", "S1-P1", 3, "Step", task.id, "設計"), rank: "W" };
  const s2 = { ...item("S1", "S1-P1", 4, "Step", task.id, "実装"), rank: "V" };
  const items = fakeItemsPort([epic, task, s1, s2]);
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  let changed = 0;
  const detail = createDetail({ catalog, items, toasts, onChanged: () => void changed++ });
  return { detail, items, toasts, task, s1, s2, changed: () => changed };
}

describe("createDetail", () => {
  it("opens an item with its ancestors and children in sibling order", async () => {
    const { detail, task } = await setup();
    await detail.open("S1", task.id);
    const v = get(detail.view);
    expect(v?.displayId).toBe("HH-2");
    expect(v?.ancestors.map((a) => a.title)).toEqual(["認証"]);
    expect(v?.children.map((c) => c.title)).toEqual(["実装", "設計"]);
    expect(v?.project.displayId).toBe("HH-P1");
    expect(v?.readOnly).toBe(false);
  });

  it("refuses an empty title without sending, and skips an unchanged one", async () => {
    const { detail, items, toasts, task } = await setup();
    await detail.open("S1", task.id);
    expect(await detail.rename("  \n ")).toBe(false);
    expect(get(toasts.list)[0]).toMatchObject({ text: "タイトルは空にできません", error: true });
    expect(await detail.rename(" 画面 ")).toBe(true);
    expect(items.calls).toEqual([]);
  });

  it("saves a normalized title and tells the screen", async () => {
    const { detail, items, task, changed } = await setup();
    await detail.open("S1", task.id);
    expect(await detail.rename("ログイン\n 画面")).toBe(true);
    expect(items.calls).toEqual([`rename ${task.id} ログイン 画面`]);
    expect(get(detail.view)?.item.title).toBe("ログイン 画面");
    expect(changed()).toBe(1);
  });

  it("adds a Step, ignoring an empty one", async () => {
    const { detail, items, task } = await setup();
    await detail.open("S1", task.id);
    expect(await detail.addStep("   ")).toBe(false);
    expect(await detail.addStep("確認")).toBe(true);
    expect(items.calls).toEqual([`addStep ${task.id} 確認`]);
    expect(get(detail.view)?.children.map((c) => c.title)).toContain("確認");
  });

  it("moves a Step up, doing nothing for the first", async () => {
    const { detail, items, task, s1, s2 } = await setup();
    await detail.open("S1", task.id);
    await detail.moveUp(s2.id);
    await detail.moveUp(s1.id);
    expect(items.calls).toEqual([`moveUp ${s1.id}`]);
  });

  it("closes when the item is gone, and toggles full screen", async () => {
    const { detail, task } = await setup();
    await detail.open("S1", task.id);
    detail.toggleFull();
    expect(get(detail.full)).toBe(true);
    await detail.open("S1", "nope");
    expect(get(detail.view)).toBeNull();
    detail.close();
    expect(get(detail.full)).toBe(false);
  });
});

describe("titles and copy messages", () => {
  it("normalizes spaces and line breaks", () => {
    expect(normalizeTitle("  a \n\t b  ")).toBe("a b");
  });

  it("says what was copied", () => {
    expect(copiedMessage("HH-2")).toBe("コピーしました: HH-2");
    expect(copyFailedMessage("HH-2")).toBe(
      "コピーできませんでした。HH-2 を選択してコピーしてください",
    );
  });
});
```

Run: `cd web/packages/core && npx vitest run src/features/items/detail.test.ts`
Expected: FAIL(`./detail` がない)

- [ ] **Step 2: core を実装する**

`web/packages/core/src/features/items/detail.ts`:

```ts
import { NotFound, Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import type { ItemsPort } from "../../port/items";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog, ProjectNode, StoreNode, WorkspaceNode } from "../catalog/catalog";
import { itemDisplayId } from "../router/route";
import type { Toasts } from "../toasts/toasts";
import { ancestorsOf, compareSiblings } from "./tree";

// 詳細パネルの状態(02「詳細パネル」のうち A2 の分。A2 の計画の決定 18)。

export interface DetailView {
  store: StoreNode;
  workspace: WorkspaceNode;
  project: ProjectNode;
  item: Item;
  displayId: string;
  // ancestors は、パンくずに出す祖先(根から順)。
  ancestors: Item[];
  // children は、子(Epic なら Story・Task・Bug、Story・Task・Bug なら Step)を並び順で。
  children: Item[];
  readOnly: boolean;
}

export interface Detail {
  view: Readable<DetailView | null>;
  full: Readable<boolean>;
  open(storeId: string, itemId: string): Promise<void>;
  close(): void;
  toggleFull(): void;
  // rename は、タイトルを保存する。false なら、入力欄を元のタイトルに戻す。
  rename(title: string): Promise<boolean>;
  // addStep は、Step を末尾に作る。true なら、入力欄を空にする。
  addStep(title: string): Promise<boolean>;
  // moveUp は、Step を直前の Step と入れ替える。先頭なら何もしない。
  moveUp(itemId: string): Promise<void>;
}

export interface DetailDeps {
  catalog: Catalog;
  items: ItemsPort;
  toasts: Toasts;
  // onChanged は、保存したあと(開いているプロジェクトの画面を読み直す)。
  onChanged(): void;
}

// normalizeTitle は、連続する空白と改行を 1 つの空白にまとめ、前後の空白を取り除く(02「タイトル」)。
export function normalizeTitle(s: string): string {
  return s.split(/\s+/).filter(Boolean).join(" ");
}

export const copiedMessage = (id: string) => `コピーしました: ${id}`;
export const copyFailedMessage = (id: string) =>
  `コピーできませんでした。${id} を選択してコピーしてください`;

export function createDetail(deps: DetailDeps): Detail {
  const view = writable<DetailView | null>(null);
  const full = writable(false);
  let latest = 0;

  async function load(storeId: string, itemId: string) {
    latest += 1;
    const mine = latest;
    try {
      const item = await deps.items.get(storeId, itemId);
      const all = await deps.items.list(storeId, item.projectId);
      const store = deps.catalog.store(storeId);
      const project = deps.catalog.project(storeId, item.projectId);
      const workspace = project && deps.catalog.workspace(storeId, project.workspaceId);
      if (mine !== latest) return;
      if (!store || !project || !workspace) {
        view.set(null);
        return;
      }
      view.set({
        store,
        workspace,
        project,
        item,
        displayId: itemDisplayId(workspace.prefix, item.number),
        ancestors: ancestorsOf(all, item.id),
        children: all.filter((it) => it.parentId === item.id).sort(compareSiblings),
        readOnly: store.readOnly,
      });
    } catch (e) {
      // 開いているアイテムがなくなったら、パネルを閉じる(02「詳細パネル」)。
      if (!(e instanceof NotFound)) throw e;
      if (mine === latest) view.set(null);
    }
  }

  async function saved() {
    const v = get(view);
    if (v) await load(v.store.id, v.item.id);
    deps.onChanged();
  }

  // refusedToast は、断りをエラーのトーストで出す。断り以外は投げる。
  function refusedToast(e: unknown) {
    if (!(e instanceof Refused)) throw e;
    deps.toasts.show(e.message, { error: true });
  }

  return {
    view: readonly(view),
    full: readonly(full),
    open: load,
    close() {
      latest += 1;
      view.set(null);
      full.set(false);
    },
    toggleFull: () => full.update((f) => !f),
    async rename(title) {
      const v = get(view);
      if (!v) return false;
      const next = normalizeTitle(title);
      if (next === "") {
        deps.toasts.show("タイトルは空にできません", { error: true });
        return false;
      }
      if (next === v.item.title) return true;
      try {
        await deps.items.rename(v.store.id, v.item.id, next);
      } catch (e) {
        refusedToast(e);
        return false;
      }
      await saved();
      return true;
    },
    async addStep(title) {
      const v = get(view);
      const next = normalizeTitle(title);
      // 空のまま Enter を押したときは何もしない(02「子と Step」)。
      if (!v || next === "") return false;
      try {
        await deps.items.addStep(v.store.id, v.item.id, next);
      } catch (e) {
        refusedToast(e);
        return false;
      }
      await saved();
      return true;
    },
    async moveUp(itemId) {
      const v = get(view);
      if (!v || v.children[0]?.id === itemId) return;
      try {
        await deps.items.moveUp(v.store.id, itemId);
      } catch (e) {
        refusedToast(e);
        return;
      }
      await saved();
    },
  };
}
```

`web/packages/core/src/index.ts` に足す(`./features/items/createDialog` の export の次):

```ts
export {
  copiedMessage,
  copyFailedMessage,
  createDetail,
  type Detail,
  type DetailView,
  normalizeTitle,
} from "./features/items/detail";
```

Run: `cd web/packages/core && npx tsc -p . && npx vitest run`
Expected: PASS

- [ ] **Step 3: 画面の失敗するテストを書く**

`web/e2e/detail.spec.ts`:

```ts
import { expect, test } from "@playwright/test";
import { tokenUrl, unique } from "./helpers";

test("the item entry opens the project's tree with the panel", async ({ page }) => {
  await page.goto(tokenUrl("/s/e2e/i/HH-2"));
  await expect(page).toHaveURL(/\/s\/e2e\/p\/HH-P1\/tree$/);
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue("ログイン画面を作る");
  await expect(panel.getByText("HH-2", { exact: true })).toBeVisible();
  await expect(panel.getByRole("navigation", { name: "パンくず" }).getByText("HH-1")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(panel).toBeHidden();
});

test("renames a Task, adds Steps and moves one up", async ({ page }, info) => {
  const task = unique(info, "設定画面");
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/tree"));
  await page.getByRole("list", { name: "ツリー" }).getByRole("link", { name: "認証" }).click();
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await panel.getByRole("button", { name: "Task" }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(task);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  // 「続けて作成」がオフなら、作ったアイテムの詳細パネルが開く。
  const title = panel.getByRole("textbox", { name: "タイトル" });
  await expect(title).toHaveValue(task);

  await title.fill("   ");
  await title.press("Enter");
  await expect(page.getByText("タイトルは空にできません")).toBeVisible();
  await expect(title).toHaveValue(task);
  await title.fill(`${task}  を直す`);
  await title.press("Enter");
  await expect(title).toHaveValue(`${task} を直す`);
  await expect(
    page.getByRole("list", { name: "ツリー" }).getByRole("link", { name: `${task} を直す` }),
  ).toBeVisible();

  const steps = panel.getByRole("region", { name: "Step" });
  const input = panel.getByRole("textbox", { name: "Step を追加" });
  await input.fill("設計");
  await input.press("Enter");
  await expect(input).toHaveValue("");
  await expect(input).toBeFocused();
  await input.fill("実装");
  await input.press("Enter");
  await expect(steps.getByRole("link")).toHaveText(["設計", "実装"]);
  await steps.getByRole("button", { name: "上へ" }).nth(1).click();
  await expect(steps.getByRole("link")).toHaveText(["実装", "設計"]);
});

test("copies the ID", async ({ page, context }) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto(tokenUrl("/s/e2e/i/HH-1"));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await panel.getByRole("button", { name: "ID をコピー" }).click();
  await expect(page.getByText("コピーしました: HH-1")).toBeVisible();
});

test("a read-only store's panel shows the band and cannot be edited", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/i/HH-2"));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(
    panel.getByText(
      "readonly は閲覧のみの Store です。閲覧はできますが、作成・変更・削除はできません。",
    ),
  ).toBeVisible();
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toBeDisabled();
  await expect(panel.getByRole("textbox", { name: "Step を追加" })).toBeDisabled();
});
```

`web/e2e/create.spec.ts` の 1 本めは、作ったあとに詳細パネルが開くので、パネルを閉じてから次へ進むように直す(全体):

```ts
import { expect, test } from "@playwright/test";
import { tokenUrl, unique } from "./helpers";

test("creates an Epic, a Task under it and a Step from the project screen", async ({
  page,
}, info) => {
  const epic = unique(info, "通知");
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/overview"));
  await page.getByRole("button", { name: "Epic", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await expect(dialog.getByRole("combobox", { name: "種類" })).toHaveValue("Epic");
  await expect(dialog.getByText(/^ID は HH-\d+ になります。$/)).toBeVisible();
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(dialog.getByText("タイトルを入力してください")).toBeVisible();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(epic);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(dialog).toBeHidden();
  await expect(page.getByText(/^HH-\d+ を作成しました$/)).toBeVisible();
  await expect(page.getByRole("region", { name: "Epic" }).getByText(epic)).toBeVisible();
  // 「続けて作成」がオフなので、作った Epic の詳細パネルが開く。Esc で閉じる。
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(epic);
  await page.keyboard.press("Escape");

  await page.getByRole("tab", { name: "ツリー" }).click();
  const tree = page.getByRole("list", { name: "ツリー" });
  await tree
    .getByRole("listitem")
    .filter({ hasText: epic })
    .getByRole("button", { name: "Story を追加" })
    .click();
  await expect(dialog.getByRole("combobox", { name: "種類" })).toHaveValue("Story");
  await dialog.getByRole("combobox", { name: "種類" }).selectOption("Task");
  await expect(dialog.getByRole("combobox", { name: "親" })).toHaveText(new RegExp(epic));
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(`${epic} の Task`);
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(tree.getByText(`${epic} の Task`)).toBeVisible();
  await page.keyboard.press("Escape");

  await tree
    .getByRole("listitem")
    .filter({ hasText: `${epic} の Task` })
    .getByRole("button", { name: "Step を追加" })
    .click();
  await dialog.getByRole("checkbox", { name: "続けて作成" }).check();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(`${epic} の手順`);
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toHaveValue("");
  await expect(dialog.getByRole("combobox", { name: "種類" })).toHaveValue("Step");
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(tree.getByText(`${epic} の手順`)).toBeVisible();
});

test("the C key opens the dialog with a Task in the open project", async ({ page }) => {
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/tree"));
  await expect(page.getByRole("heading", { name: "HH-P1 ログイン" })).toBeVisible();
  await page.keyboard.press("c");
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await expect(dialog.getByRole("combobox", { name: "種類" })).toHaveValue("Task");
  await expect(dialog.getByRole("combobox", { name: "プロジェクト" })).toHaveText(/ログイン/);
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toBeFocused();
  // 入力中の C は文字になり、ダイアログを開き直さない。
  await page.keyboard.press("c");
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toHaveValue("c");
});

test("a read-only store has no enabled create buttons", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/p/HH-P1/tree"));
  await expect(page.getByRole("button", { name: "Epic", exact: true })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Story を追加" })).toBeDisabled();
});
```

Run: `make e2e-bin && cd web && pnpm e2e detail.spec create.spec`
Expected: FAIL(詳細パネルがない)

- [ ] **Step 4: 画面を実装する**

`web/app/src/links.ts`(全体):

```ts
import { newTabUrl } from "@hedgehug/api";
import { formatRoute, type Navigation, type Route } from "@hedgehug/core";

// 画面を移る部品(サイドバーの項目・パンくず・タブ・行のタイトル)のリンク。
// <a href={links.href(r)} onclick={(e) => links.click(e, r)} onauxclick={(e) => links.click(e, r)}> で使う。
export interface Links {
  // href は、リンクのアドレス。トークンを入れない(右クリックの「新しいタブで開く」はトークンのない URL。08)。
  href(route: Route): string;
  // click は、ふつうのクリックで画面を移り、Ctrl・Cmd・Shift+クリックと中クリックで新しいタブに開く(01)。
  // action を渡すと、ふつうのクリックで画面を移る代わりに action を呼ぶ(行のタイトルで詳細パネルを開くとき)。
  click(event: MouseEvent, route: Route, action?: () => void): void;
}

export function createLinks(
  navigation: Navigation,
  token: string,
  open: (url: string) => void = (url) => window.open(url, "_blank", "noopener"),
): Links {
  return {
    href: formatRoute,
    click(event, route, action) {
      // 右クリック(button 2)などは、ブラウザのメニューに任せる。
      if (event.button > 1) return;
      event.preventDefault();
      if (event.button === 1 || event.ctrlKey || event.metaKey || event.shiftKey) {
        open(newTabUrl(formatRoute(route), token));
        return;
      }
      if (action) action();
      else navigation.go(route);
    },
  };
}
```

`web/app/src/screens/item/DetailHeader.svelte`:

```svelte
<script lang="ts">
  import type { DetailView, Route } from "@hedgehug/core";
  import { IconButton, StoreBadge } from "@hedgehug/ui";
  import { detail, links } from "../../wire";

  // 詳細パネルのヘッダー: パンくず(Store・ワークスペース・プロジェクト・祖先)、全画面、閉じる(02「上から順の構成」)。
  // ウォッチとメニューは A4(A2 の計画の決定 18)。
  interface Props {
    v: DetailView;
    full: boolean;
  }

  let { v, full }: Props = $props();

  const wsRoute = $derived<Route>({
    name: "workspace",
    store: v.store.name,
    prefix: v.workspace.prefix,
  });
  const projectRoute = $derived<Route>({
    name: "project",
    store: v.store.name,
    project: v.project.displayId,
    tab: "overview",
  });
  const itemRoute = (displayId: string): Route => ({
    name: "item",
    store: v.store.name,
    item: displayId,
  });
  const ancestorId = (n: number) => `${v.workspace.prefix}-${n}`;
</script>

<nav class="crumbs" aria-label="パンくず">
  <StoreBadge name={v.store.name} color={v.store.color} />
  <a
    href={links.href(wsRoute)}
    onclick={(e) => links.click(e, wsRoute)}
    onauxclick={(e) => links.click(e, wsRoute)}
    >{v.workspace.name}</a
  >
  <span class="sep">/</span>
  <a
    href={links.href(projectRoute)}
    onclick={(e) => links.click(e, projectRoute)}
    onauxclick={(e) => links.click(e, projectRoute)}
    >{v.project.title}</a
  >
  {#each v.ancestors as a (a.id)}
    {@const r = itemRoute(ancestorId(a.number))}
    <span class="sep">/</span>
    <a
      href={links.href(r)}
      onclick={(e) => links.click(e, r, () => void detail.open(v.store.id, a.id))}
      onauxclick={(e) => links.click(e, r)}
      >{ancestorId(a.number)}</a
    >
  {/each}
</nav>
<span class="grow"></span>
<IconButton label={full ? "パネルで表示" : "全画面で表示"} size="sm" onclick={detail.toggleFull}
  >{full ? "⤡" : "⤢"}</IconButton
>
<IconButton label="閉じる" size="sm" onclick={detail.close}>×</IconButton>

<style>
  .crumbs {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    flex-wrap: wrap;
    font-size: 12.5px;
  }
  .crumbs a {
    color: var(--muted);
    text-decoration: none;
  }
  .crumbs a:hover {
    color: var(--ink);
    text-decoration: underline;
  }
  .sep {
    color: var(--muted-2);
  }
  .grow {
    flex: 1;
  }
</style>
```

`web/app/src/screens/item/DetailChildren.svelte`:

```svelte
<script lang="ts">
  import {
    type DetailView,
    type Item,
    itemDisplayId,
    type Kind,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button, IconButton, Input, KindIcon } from "@hedgehug/ui";
  import { detail, itemDialog, links } from "../../wire";

  // 詳細パネルの「子」(02「子と Step」のうち、一覧・Step の追加・「上へ」)。完了の数・ステータス・担当者・
  // Step のチェックと「×」は A3・A4(A2 の計画の決定 18)。
  interface Props {
    v: DetailView;
  }

  let { v }: Props = $props();

  const isEpic = $derived(v.item.kind === "Epic");
  const note = $derived(v.readOnly ? readOnlyNote : undefined);
  let newStep = $state("");

  const id = (it: Item) => itemDisplayId(v.workspace.prefix, it.number);
  const route = (it: Item) => ({ name: "item" as const, store: v.store.name, item: id(it) });

  function add(kind: Kind) {
    void itemDialog.open({
      workspaceId: v.workspace.id,
      projectId: v.project.id,
      kind,
      parentId: v.item.id,
    });
  }

  async function onkeydown(event: KeyboardEvent) {
    if (event.key !== "Enter" || event.isComposing) return;
    // 追加したら入力欄を空にし、フォーカスはそのまま残す(02「子と Step」)。
    if (await detail.addStep(newStep)) newStep = "";
  }
</script>

<section class="peek-sec" aria-label={isEpic ? "Story・Task・Bug" : "Step"}>
  <h3>{isEpic ? "Story・Task・Bug" : "Step"}</h3>
  {#each v.children as c (c.id)}
    <div class="child-row">
      {#if isEpic}
        <KindIcon kind={c.kind} />
        <code class="id">{id(c)}</code>
      {/if}
      <a
        class="row-title"
        href={links.href(route(c))}
        onclick={(e) => links.click(e, route(c), () => void detail.open(v.store.id, c.id))}
        onauxclick={(e) => links.click(e, route(c))}
        >{c.title}</a
      >
      {#if !isEpic}
        <code class="id">{id(c)}</code>
        <IconButton
          label="上へ"
          size="xs"
          disabled={v.readOnly}
          title={note}
          onclick={() => void detail.moveUp(c.id)}
          >↑</IconButton
        >
      {/if}
    </div>
  {/each}
  {#if isEpic}
    <div class="row-gap">
      {#each ["Story", "Task", "Bug"] as const as k (k)}
        <Button size="sm" disabled={v.readOnly} title={note} onclick={() => add(k)}>
          <span aria-hidden="true">+</span> {k}
        </Button>
      {/each}
    </div>
  {:else}
    <Input
      label="Step を追加"
      placeholder="Step を追加して Enter"
      disabled={v.readOnly}
      title={note}
      {onkeydown}
      bind:value={newStep}
    />
  {/if}
</section>

<style>
  .peek-sec {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  h3 {
    font-size: 13px;
    margin: 0;
  }
  .child-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 30px;
    border-bottom: 1px solid var(--line);
  }
  .row-title {
    flex: 1;
    min-width: 0;
    color: var(--ink);
    text-decoration: none;
  }
  .row-title:hover {
    text-decoration: underline;
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-gap {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }
</style>
```

`web/app/src/screens/item/DetailPanel.svelte`:

```svelte
<script lang="ts">
  import { copiedMessage, copyFailedMessage, readOnlyBand, readOnlyNote } from "@hedgehug/core";
  import { IconButton, Input, KindIcon, ReadOnlyBand, SidePanel } from "@hedgehug/ui";
  import { detail, toasts } from "../../wire";
  import DetailChildren from "./DetailChildren.svelte";
  import DetailHeader from "./DetailHeader.svelte";

  // アイテムの詳細パネル(02「詳細パネル」のうち A2 の分。A2 の計画の決定 18)。
  const view = detail.view;
  const full = detail.full;

  // タイトルの入力欄。開いたアイテムが変わったら、そのタイトルに戻す。
  let title = $state("");
  $effect(() => {
    title = $view?.item.title ?? "";
  });

  async function save() {
    // 保存できなかった(空・断り)ときは、元のタイトルに戻す(02「タイトル」)。
    if (!(await detail.rename(title))) title = $view?.item.title ?? "";
  }

  function onkeydown(event: KeyboardEvent) {
    // Enter でもフォーカスを外して保存する。
    if (event.key === "Enter" && !event.isComposing) (event.currentTarget as HTMLElement).blur();
  }

  async function copy(id: string) {
    try {
      await navigator.clipboard.writeText(id);
      toasts.show(copiedMessage(id));
    } catch {
      toasts.show(copyFailedMessage(id), { error: true });
    }
  }
</script>

<SidePanel open={$view !== null} full={$full} label="詳細パネル" onclose={detail.close}>
  {#snippet head()}
    {#if $view}
      <DetailHeader v={$view} full={$full} />
    {/if}
  {/snippet}
  {#if $view}
    {@const v = $view}
    {#if v.readOnly}
      <ReadOnlyBand text={readOnlyBand(v.store.name)} inset />
    {/if}
    <div class="kind-row">
      <KindIcon kind={v.item.kind} />
      <span>{v.item.kind}</span>
      <code class="id">{v.displayId}</code>
      <IconButton label="ID をコピー" size="xs" onclick={() => void copy(v.displayId)}
        >⧉</IconButton
      >
    </div>
    <Input
      label="タイトル"
      heading
      disabled={v.readOnly}
      title={v.readOnly ? readOnlyNote : undefined}
      onblur={() => void save()}
      {onkeydown}
      bind:value={title}
    />
    {#if v.item.kind !== "Step"}
      <DetailChildren {v} />
    {/if}
    <section class="peek-sec" aria-label="GitHub">
      <h3>GitHub</h3>
      <p class="muted">リンクはありません</p>
    </section>
  {/if}
</SidePanel>

<style>
  .kind-row {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12.5px;
  }
  .id {
    font-family: var(--font-mono);
    color: var(--muted);
  }
  .peek-sec h3 {
    font-size: 13px;
    margin: 0 0 6px;
  }
  .muted {
    color: var(--muted);
    font-size: 12.5px;
    margin: 0;
  }
</style>
```

`web/app/src/screens/project/TreeRow.svelte`(全体。タイトルをリンクにする):

```svelte
<script lang="ts">
  import {
    childKindOf,
    type Item,
    itemDisplayId,
    readOnlyNote,
    type TreeNode,
  } from "@hedgehug/core";
  import { IconButton, KindIcon } from "@hedgehug/ui";
  import Self from "./TreeRow.svelte";

  // ツリーの 1 行と、その下の行(04「ツリー」の「行」のうち、A2 の計画の決定 17 の分)。
  interface Props {
    node: TreeNode;
    prefix: string;
    depth: number;
    collapsed: Set<string>;
    readOnly: boolean;
    ontoggle: (id: string) => void;
    // onadd は、行の「+」(その行を親にした作成ダイアログ)。
    onadd: (parent: Item) => void;
    // onopen は、タイトルを押したとき(詳細パネルを開く)。Ctrl+クリックなどは新しいタブでアイテムの入口を開く。
    onopen: (event: MouseEvent, item: Item) => void;
    href: (item: Item) => string;
  }

  let { node, prefix, depth, collapsed, readOnly, ontoggle, onadd, onopen, href }: Props = $props();

  const it = $derived(node.item);
  const open = $derived(!collapsed.has(it.id));
  const child = $derived(childKindOf(it.kind));
</script>

<li class="tree-row" style:--d={depth}>
  {#if node.children.length > 0}
    <button
      type="button"
      class="twisty"
      class:open
      aria-label="開閉"
      aria-expanded={open}
      onclick={() => ontoggle(it.id)}
    >
      ›
    </button>
  {:else}
    <span class="twisty-space"></span>
  {/if}
  <KindIcon kind={it.kind} />
  <code class="id">{itemDisplayId(prefix, it.number)}</code>
  <a
    class="row-title"
    href={href(it)}
    onclick={(e) => onopen(e, it)}
    onauxclick={(e) => onopen(e, it)}
    >{it.title}</a
  >
  {#if child}
    <IconButton
      label="{child} を追加"
      size="xs"
      disabled={readOnly}
      title={readOnly ? readOnlyNote : undefined}
      onclick={() => onadd(it)}
      >+</IconButton
    >
  {/if}
</li>
{#if open}
  {#each node.children as c (c.item.id)}
    <Self
      node={c}
      {prefix}
      depth={depth + 1}
      {collapsed}
      {readOnly}
      {ontoggle}
      {onadd}
      {onopen}
      {href}
    />
  {/each}
{/if}

<style>
  .tree-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    padding: 2px 8px 2px calc(8px + var(--d) * 22px);
    border-bottom: 1px solid var(--line);
  }
  .twisty,
  .twisty-space {
    width: 18px;
    height: 18px;
    flex: none;
  }
  .twisty {
    border: 0;
    background: none;
    color: var(--muted);
    cursor: pointer;
    transition: transform 0.12s;
  }
  .twisty.open {
    transform: rotate(90deg);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    color: var(--ink);
    text-decoration: none;
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
```

`web/app/src/screens/project/ProjectTree.svelte`(全体):

```svelte
<script lang="ts">
  import {
    childKindOf,
    type Item,
    itemDisplayId,
    type ProjectView,
    type Route,
  } from "@hedgehug/core";
  import { SvelteSet } from "svelte/reactivity";
  import { detail, itemDialog, links } from "../../wire";
  import TreeRow from "./TreeRow.svelte";

  // プロジェクトの「ツリー」のタブ。Epic を一番上の階層に並べる(04「ツリー」の「行」)。
  interface Props {
    view: ProjectView;
  }

  let { view }: Props = $props();

  // 開閉は、画面を出しているあいだだけ覚える。初めはすべて開いている。
  const collapsed = new SvelteSet<string>();
  function toggle(id: string) {
    if (collapsed.has(id)) collapsed.delete(id);
    else collapsed.add(id);
  }
  const route = (it: Item): Route => ({
    name: "item",
    store: view.store.name,
    item: itemDisplayId(view.workspace.prefix, it.number),
  });
  const href = (it: Item) => links.href(route(it));
  const open = (e: MouseEvent, it: Item) =>
    links.click(e, route(it), () => void detail.open(view.store.id, it.id));
  function add(parent: Item) {
    const kind = childKindOf(parent.kind);
    if (!kind) return;
    void itemDialog.open({
      workspaceId: parent.workspaceId,
      projectId: parent.projectId,
      kind,
      parentId: parent.id,
    });
  }
</script>

<ul class="tree" aria-label="ツリー">
  {#each view.tree as node (node.item.id)}
    <TreeRow
      {node}
      prefix={view.workspace.prefix}
      depth={0}
      {collapsed}
      readOnly={view.store.readOnly}
      ontoggle={toggle}
      onadd={add}
      onopen={open}
      {href}
    />
  {/each}
</ul>

<style>
  .tree {
    list-style: none;
    margin: 0;
    padding: 0;
    border-top: 1px solid var(--line);
  }
</style>
```

`web/app/src/screens/project/ProjectOverview.svelte`(全体):

```svelte
<script lang="ts">
  import {
    formatDay,
    type Item,
    itemDisplayId,
    ownerName,
    type ProjectView,
    type Route,
    readOnlyNote,
  } from "@hedgehug/core";
  import { Button, KindIcon } from "@hedgehug/ui";
  import { detail, links } from "../../wire";

  // プロジェクトの「概要」のタブ。A2 では「Epic」の区画と、プロパティの表示だけ(A2 の計画の決定 16)。
  interface Props {
    view: ProjectView;
    onaddepic: () => void;
  }

  let { view, onaddepic }: Props = $props();

  const year = new Date().getFullYear();
  const p = $derived(view.project);
  const id = (it: Item) => itemDisplayId(view.workspace.prefix, it.number);
  const route = (it: Item): Route => ({ name: "item", store: view.store.name, item: id(it) });
</script>

<div class="overview-frame">
  <div class="overview">
    <section class="card-sec" aria-label="Epic">
      <div class="sec-head">
        <h2>Epic</h2>
        <span class="grow"></span>
        <Button
          size="sm"
          disabled={view.store.readOnly}
          title={view.store.readOnly ? readOnlyNote : undefined}
          onclick={onaddepic}
          ><span aria-hidden="true">+</span>
          Epic を追加</Button
        >
      </div>
      {#each view.epics as e (e.id)}
        <div class="list-row">
          <KindIcon kind="Epic" />
          <code class="id">{id(e)}</code>
          <a
            class="row-title"
            href={links.href(route(e))}
            onclick={(ev) => links.click(ev, route(e), () => void detail.open(view.store.id, e.id))}
            onauxclick={(ev) => links.click(ev, route(e))}
            >{e.title}</a
          >
        </div>
      {/each}
    </section>
    <section class="props" aria-label="プロパティ">
      <div class="prop"><span class="prop-k">ステータス</span><span>{p.status}</span></div>
      <div class="prop">
        <span class="prop-k">オーナー</span><span>{ownerName(p.owner, view.members)}</span>
      </div>
      <div class="prop">
        <span class="prop-k">開始日</span><span>{formatDay(p.start, year)}</span>
      </div>
      <div class="prop">
        <span class="prop-k">終了日</span><span>{formatDay(p.end, year)}</span>
      </div>
    </section>
  </div>
</div>

<style>
  .overview-frame {
    container: overview / inline-size;
  }
  .overview {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 260px;
    gap: 24px;
  }
  @container overview (max-width: 640px) {
    .overview {
      grid-template-columns: minmax(0, 1fr);
    }
  }
  .sec-head {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .grow {
    flex: 1;
  }
  h2 {
    font-size: 14px;
    margin: 0 0 8px;
  }
  .list-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    border-bottom: 1px solid var(--line);
  }
  .id {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--muted);
  }
  .row-title {
    color: var(--ink);
    text-decoration: none;
    flex: 1;
    min-width: 0;
  }
  .props {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .prop {
    display: grid;
    grid-template-columns: 90px minmax(0, 1fr);
    align-items: center;
  }
  .prop-k {
    color: var(--muted);
    font-size: 12.5px;
  }
</style>
```

`web/app/src/wire.ts`(全体):

```ts
import {
  createCatalogPort,
  createItemsPort,
  createProjectsPort,
  createSystemPort,
  rememberToken,
  setToken,
} from "@hedgehug/api";
import {
  createCatalog,
  createDetail,
  createItemDialog,
  createNavigation,
  createProjectDialog,
  createProjectScreen,
  createScreen,
  createShell,
  createSystemInfo,
  createToasts,
  get,
  type Here,
  routeOfProject,
} from "@hedgehug/core";
import { browserHistory } from "./history";
import { createLinks } from "./links";

// 組み立て: api の実装を core に渡す(設計書 6 節)。画面はここで作った状態を使う。

// トークンは、ほかの何よりも先に決める(アドレスからハッシュを消してから、履歴を読む)。A2 の計画の決定 5。
const token = rememberToken(location, sessionStorage, (url) => history.replaceState(null, "", url));
setToken(token);

export const shell = createShell();
export const system = createSystemInfo(createSystemPort());
export const navigation = createNavigation(browserHistory());
navigation.start();
export const links = createLinks(navigation, token);
export const toasts = createToasts({
  set: (fn, ms) => setTimeout(fn, ms),
  clear: (handle) => clearTimeout(handle as ReturnType<typeof setTimeout>),
});

const itemsPort = createItemsPort();
const catalogPort = createCatalogPort();
export const catalog = createCatalog(catalogPort);
export const projectScreen = createProjectScreen(catalog, catalogPort, itemsPort);
export const projectDialog = createProjectDialog({
  catalog,
  catalogPort,
  projects: createProjectsPort(),
  // 作ったら、そのプロジェクトの概要を開く。トーストは出さない(02「作成」)。
  onCreated: (storeId, project) =>
    navigation.go(routeOfProject(get(catalog.stores), storeId, project.id, "overview")),
});
export const detail = createDetail({
  catalog,
  items: itemsPort,
  toasts,
  onChanged: () => void projectScreen.reload(),
});
export const screen = createScreen({
  navigation,
  catalog,
  items: itemsPort,
  openItem: (storeId, itemId) => void detail.open(storeId, itemId),
});

// here は、作成ダイアログの初期値に使う、いま開いている画面のワークスペースとプロジェクト(02)。
function here(): Here {
  const c = get(screen.current);
  if (c?.name === "workspace") return { workspaceId: c.workspaceId };
  if (c?.name === "project") {
    return {
      workspaceId: catalog.project(c.storeId, c.projectId)?.workspaceId,
      projectId: c.projectId,
    };
  }
  return {};
}

export const itemDialog = createItemDialog({
  catalog,
  items: itemsPort,
  toasts,
  here,
  openProjectDialog: (workspaceId) => void projectDialog.open(workspaceId),
  // 「続けて作成」がオフなら、作ったアイテムの詳細パネルを開く(02「作成したあと」)。
  onCreated: (item, keptOpen) => {
    void projectScreen.reload();
    if (!keptOpen) void detail.open(item.storeId, item.id);
  },
});

// 画面を移ったら、サイドバーのドロワーと詳細パネルを閉じる(01「画面の移り方」)。
navigation.route.subscribe(() => {
  shell.closeDrawer();
  detail.close();
});

// 一覧を読んでから、アドレスの解決を始める(一覧がないと Store 名と ID を解決できない)。
void catalog.load().then(() => screen.start());
```

`web/app/src/App.svelte`(全体。`DetailPanel` は `AppFrame` の中):

```svelte
<script lang="ts">
  import { AppFrame, Toasts } from "@hedgehug/ui";
  import CreateItemDialog from "./screens/item/CreateItemDialog.svelte";
  import DetailPanel from "./screens/item/DetailPanel.svelte";
  import CreateProjectDialog from "./screens/project/CreateProjectDialog.svelte";
  import Screen from "./shell/Screen.svelte";
  import Sidebar from "./shell/Sidebar.svelte";
  import { catalog, itemDialog, links, screen, shell, system, toasts } from "./wire";

  const drawerOpen = shell.drawerOpen;
  const version = system.version;
  const toastList = toasts.list;
  const stores = catalog.stores;
  const current = screen.current;
  system.load();

  // C キーで作成ダイアログを開く。入力中とダイアログを開いているときは使えない(02「開き方と初期値」)。
  function onkeydown(event: KeyboardEvent) {
    if (event.key.toLowerCase() !== "c" || event.ctrlKey || event.metaKey || event.altKey) return;
    const t = event.target as HTMLElement | null;
    const typing = t?.closest("input, textarea, select, [contenteditable='true']");
    if (typing || document.querySelector('[aria-modal="true"]')) return;
    event.preventDefault();
    void itemDialog.open();
  }
</script>

<svelte:window {onkeydown} />

<AppFrame drawerOpen={$drawerOpen} onclosedrawer={shell.closeDrawer}>
  {#snippet sidebar()}
    <Sidebar stores={$stores} current={$current} {links} />
  {/snippet}
  <Screen current={$current} stores={$stores} version={$version} ondrawer={shell.openDrawer} />
  <DetailPanel />
</AppFrame>
<CreateProjectDialog />
<CreateItemDialog />
<Toasts toasts={$toastList} ondismiss={toasts.dismiss} />
```

- [ ] **Step 5: すべて通ることを確かめる**

Run: `cd web && pnpm lint && pnpm check && pnpm test && pnpm deps && pnpm size && cd .. && make e2e-bin && cd web && pnpm e2e`
Expected: PASS(e2e は 48 本)

- [ ] **Step 6: Commit**

```bash
git add web/packages/core/src web/app/src web/e2e
git commit -m "feat(web): 詳細パネル(タイトル・子と Step)を足す(A2 Task 10)"
```

---

### Task 11: 3 つの幅の通しのテスト

A2 の完成の条件(プロジェクトを作り、Epic・Task・Step を作り、詳細パネルでタイトルを変えられる)を 1 本の流れで確かめ、すべての検査を通す。閲覧のみの Store の無効化は、Task 7〜10 の e2e がそれぞれ確かめている(ワークスペースの「プロジェクト」、プロジェクトの「Epic」とツリーの「+」、詳細パネルのタイトルと Step の入力欄、帯)。

**Files:**
- Create: `web/e2e/flow.spec.ts`

**Interfaces:**
- Consumes: Task 7〜10 の画面と `web/e2e/helpers.ts`。
- Produces: なし。

- [ ] **Step 1: 通しのテストを書く**

`web/e2e/flow.spec.ts`:

```ts
import { expect, test } from "@playwright/test";
import { sidebar, tokenUrl, unique } from "./helpers";

// A2 の完成の条件: プロジェクトを作り、Epic・Task・Step を作り、詳細パネルでタイトルを変えられる。
test("creates a project, an Epic, a Task and a Step, and renames the Task", async ({
  page,
}, info) => {
  const project = unique(info, "請求");
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await side
    .getByRole("region", { name: "e2e" })
    .getByRole("button", { name: "ワークスペースのメニュー" })
    .click();
  await page.getByRole("menuitem", { name: "プロジェクトを作成" }).click();
  const pd = page.getByRole("dialog", { name: "プロジェクトを作成" });
  await pd.getByRole("textbox", { name: "タイトル" }).fill(project);
  await pd.getByRole("button", { name: "作成" }).click();
  await expect(page.getByRole("heading", { name: new RegExp(`${project}$`) })).toBeVisible();

  await page.getByRole("button", { name: "Epic", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("請求書");
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue("請求書");

  await panel.getByRole("button", { name: "Task" }).click();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("PDF を出す");
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue("PDF を出す");
  await panel.getByRole("textbox", { name: "Step を追加" }).fill("見た目を決める");
  await panel.getByRole("textbox", { name: "Step を追加" }).press("Enter");
  await expect(panel.getByRole("region", { name: "Step" }).getByRole("link")).toHaveText([
    "見た目を決める",
  ]);

  await panel.getByRole("textbox", { name: "タイトル" }).fill("PDF と CSV を出す");
  await panel.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await page.keyboard.press("Escape");
  await page.getByRole("tab", { name: "ツリー" }).click();
  const tree = page.getByRole("list", { name: "ツリー" });
  await expect(tree.getByRole("link")).toHaveText([
    "請求書",
    "PDF と CSV を出す",
    "見た目を決める",
  ]);
  await page.reload();
  await expect(tree.getByRole("link", { name: "PDF と CSV を出す" })).toBeVisible();
});
```

- [ ] **Step 2: 通ることを確かめる**

Run: `make e2e-bin && cd web && pnpm e2e flow.spec`
Expected: PASS(3 幅)。落ちたら、それは Task 7〜10 の不具合なので、該当するタスクのファイルを直す(テストを弱めない)。

- [ ] **Step 3: すべての検査を通す**

Run: `make check && make e2e`
Expected: PASS(e2e は 51 本。`make check` の `web-check` は、生成物 `generated.ts` がコミット済みであることも確かめる)

- [ ] **Step 4: Commit**

```bash
git add web/e2e/flow.spec.ts
git commit -m "test(e2e): A2 の完成の条件を 3 つの幅で通しで確かめる(A2 Task 11)"
```
