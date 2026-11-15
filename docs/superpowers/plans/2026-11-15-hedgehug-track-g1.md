# hedgehug トラック G1(Store の運用の土台)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 本物のビルドで、config.yaml に書いた Store を起動のときに開き、Store の一覧・同期・GitHub・「あなた」を本物の値で動かす。「あなた」がメンバーにいなければ書き込みと一緒に足し、設定の「一般」でログイン名と同期のタイミングを変えられるようにする。

**Architecture:** config.yaml(`infra/machineconfig`)に `stores`・`login` を足し、そこから `port.StoreList`・`port.SyncTargets`・`port.StoreGitHub` を実装する。`port.Identity` は `app/stores.Identity` が `StoreGitHub` と `Accounts` から決める(Store を読まない)。「あなた」の追加は `app/members.JoinOnWrite` が `port.Stores` を包んで、同じ保存に足す。追加したログイン名は応答のヘッダー `Hedgehug-Joined` で画面に知らせ、トーストを出す。`cmd/hedgehug/wire.go` は起動のときに config.yaml の Store を `gitstore.Stores.Attach` で開き、仮の `no*` を本物に差し替える。合わせて、同期のロックの範囲を狭め、`Auto` の取りこぼしを直す。

**Tech Stack:** Go 1.27.3(go-git v5、huma v2、go.yaml.in/yaml/v4)。web は Svelte 5・TypeScript・Vitest・Playwright。依存ライブラリは足さない。

**持ち場:** `internal/infra/machineconfig`、`internal/app/stores`、`internal/app/members`、`internal/app/syncing`、`internal/infra/gitstore`(`replicator.go` だけ)、`internal/adapter/httpapi/{stores,syncing}`、`internal/adapter/httpapi/server.go`、`cmd/hedgehug`、`web/packages/core/src/features/{settings,sync,router,palette}`、`web/packages/api/src/fetcher.ts`、`web/app/src/screens/settings`、`web/app/src/shell/Sidebar.svelte`。

**待つトラック:** なし(A・C は完了。develop c2459c83 から)。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/08-stores-maintenance.md`
  - `:7-15`(設定のタブ)
  - `:17-38`(一般: ログイン名・確認ダイアログ・同期のタイミング・すべて同期・キーボード)
  - `:58`(閲覧のみの印はマウントで読んだ権限が元)
  - `:156`(非表示の Store を自動同期と GitHub の自動の読み取りから外す)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/01-overview.md`
  - `:64-79`(マシンごとに持つもの)
  - `:81-92`(「あなた」の決まり方と自動の追加)
  - `:207-217`(キーボードの表)
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/04-search-layouts.md` `:553-574`(ツリーのキー操作の表)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` `:277-288`(config.yaml の中身と state.db)
- 分け方: `docs/superpowers/plans/2026-11-15-hedgehug-track-g.md`

## Global Constraints

土台・トラック C・トラック D の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限は `internal/archtest/size.go` のとおり。Go ファイルは 300 行、テストは 600 行まで。関数は 60 行、認知的複雑度は 15 まで。パッケージのテスト以外のファイルは 20 個まで。
  - `internal/app/port`・`internal/app/apptest` は 20 個で満杯。ファイルを足さない。口の追加は既存の `port/syncsettings.go`・`port/storegithub.go`・`port/identity.go` に、偽物は `apptest/sync.go`・`apptest/storegithub.go` に足す。
- app の機能どうしは import しない(`app/members` は `app/stores` を import しない)。
- 本物の `Identity`・`StoreList`・`SyncTargets`・`StoreGitHub` は `Stores.Read`・`Write` を呼ばない。同期の Store のロックの中から呼ばれる(トラック C `:2058`)。
- 本物の組み立てだけで使う関数は `//go:build !e2e` のファイル(`wire.go`)に置く。e2e だけの入口は `//go:build e2e`。
- 依存ライブラリは足さない。足す必要が出たら `go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけを使う(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画とテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 文言(08 のとおり):
  - ログイン名の断り: 「英数字・ハイフン・アンダースコアのログイン名(39 文字以内)を入力してください」
  - 確認ダイアログの本文(Store ごとに 1 行): 「{Store 名} での あなた が @{旧ログイン名} から {新ログイン名} に変わります。」。ボタンは「変更」(警告色)と「キャンセル」
  - あなたを足したときのトースト: 「{ログイン名} をメンバーに追加しました」
  - 同期のタイミングの注意書き: 「このマシンだけの設定です。」
  - 項目名と選択肢: 「裏で自動的に同期する」/「間隔」1・5・15・30・60 分ごと/「書き込みのあと同期するまで」10・30・60 秒/「GitHub を読む間隔」5・15・30・60 分ごと(表記は「{N} 分ごと」「{N} 秒」)
- 手元の検証は 1 回で足りる(CI でも流れる)。`make check`・e2e のビルドの lint(`golangci-lint run --build-tags e2e ./...`)・`make e2e`。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| G1-1 | config.yaml に足す鍵: `login`(文字列)、`stores`(並びがサイドバーの並び)。既定の Store の鍵(`default_store`)は、読み書きする G3 が足す。`stores` の各行は `id`・`name`・`path`(Store のコピーの絶対パス)・`remote`(文字列。なければ省く)・`mirrors`(文字列の並び)・`account`(アカウントの `id`)・`color`・`hidden`・`read_only`。追加と変更は G2・G3 が書く。G1 は読むだけ(ログイン名と同期のタイミングは書く) | 保存形式 `:279`。`name`・`path`・`account`・`color`・`hidden` は前回の実装と同じ鍵名(C2-4・D2-14 と同じく、同じ意味のものは前回の名前)。前回の `remotes`(並び)は意味が違う(2 本目以降からも取り込んだ)ので使わず、`remote` と `mirrors` に分ける |
| G1-2 | `id` のない行・`path` を開けない行・`path` の `store.yaml` の Store ID が `id` と違う行・同じ `id` の行が 2 つ以上あるときのそれらすべては、起動のときに `slog.Warn` を出して開かず、どの一覧にも出さない。config.yaml を解析できないときも、警告を出して Store 0 個で起動する。config.yaml からは消さない。警告を出すため、`StoreEntries` は `id`・`path` が空の行も返し、判定は wire が行う(実装で Task 1 の約束から変えた) | 前回の版の config.yaml の `stores` は `id` を持たず、前回の形式の Store は開けない。読めない行で起動を止めない。同じ `id` の行は、一覧が `id` で絞るので片方だけ開くと両方が出てしまう。消すのは利用者の操作(G3 のこのマシンから削除)だけ |
| G1-3 | config.yaml を書き直すとき、手で書いたコメントと知らない鍵は残さない(いまの書き方のまま)。D2-14 の申し送りはこれで閉じる | config.yaml を書くのは画面の操作だけで、手で書くのは `hosts` の client ID ぐらい。その値は残る。コメントを残す書き方(yaml.Node の部分の書き換え)は、鍵ごとの書き換えの処理が要り、得るものに比べて大きい |
| G1-4 | 土台の決定 1 の `MachineSettings` は口として作らない。ログイン名の書き口は `port.StoreGitHub.SetMachineLogin`、同期のタイミングの書き口は `port.SyncSettings.SetTiming` に足す | `port`・`apptest` は 20 ファイルで満杯。読み口がすでにその 2 つにある |
| G1-5 | `port.Identity` の本物は `app/stores.Identity{GitHub port.StoreGitHub, Accounts port.Accounts}`。Store に割り当てたアカウントがあり、そのアカウントが登録されていればそのログイン名、なければマシンのログイン名。どちらもなければ `ErrNoLogin`(「このマシンのログイン名が未設定です」)。Store が一覧にないときは `port.ErrStoreNotFound` | 01 `:85-88`。割り当てたアカウントが消えているのは手で書き換えたときだけで、07 のアカウントの削除は割り当てを「なし」にする。トラック C `:2058` の制約を守る |
| G1-6 | 「あなた」の追加は `members.JoinOnWrite(stores port.Stores, list port.StoreGitHub) port.Stores` が `Write` を包んで行う。実行者が人(`change.ActorUser`)で、その Store が閲覧のみでなく、`tx.Members().List()` にそのログイン名(大文字と小文字を区別しない)がいないとき、利用者の `fn` を呼ぶ前に `member.Join(login)` を足す。`fn` が何も書かなかったら、足した追加も取り消す(保存しない)。保存できたときだけ、ctx の入れ物(`port.WithJoined`・`port.Joined`)に記録する | 01 `:88`「その書き込みと一緒に追加する」。何も書かない操作はそもそも書き込みでない。保存に失敗したらトーストを出さない(`restoredHeader` と同じく 2xx のときだけ)。担当者・オーナーの候補はメンバーの一覧から出す(`projects/setowner.go` の `memberOf` は `Write` の前に `Stores.Read` で確かめる)ので、メンバーにいない「あなた」は、次の書き込みで足されるまで候補に出ない。01 の「次に書き込むときに」のとおりで、変えない |
| G1-7 | 閲覧のみの Store への書き込みを、サーバーは今までどおり断らない(track-a2 `:837`)。「あなた」の追加だけは行わない | 01 `:90`「閲覧のみの Store では追加しない」。画面は書く操作を無効にしている(01 `:118`) |
| G1-8 | 応答のヘッダー `Hedgehug-Joined: <ログイン名>`(複数の Store に足したときは 1 つにまとめ、`, ` で区切る)。画面の `fetcher` が `onJoined` の受け手に渡し、`wire.ts` がログイン名ごとにトースト「{ログイン名} をメンバーに追加しました」を出す | 既存の `Hedgehug-Restored` と同じ形(`server.go`)。ログイン名は英数字・ハイフン・アンダースコアなのでエスケープは要らない |
| G1-9 | `SyncTargets.List` は、remote が SSH の形(`git@github.com:owner/name(.git)`)なら `https://github.com/owner/name.git` に直して返す。ミラーも同じ。HTTPS の形はそのまま返す | トラック C `:5286`。07 は SSH 鍵を使わない。通信はトークンの HTTPS |
| G1-10 | `gitstore.Replicator.Sync` の通信(受け取りと送り)を Store のロックの外で行う。手順は Task 5。同じ Store の `Sync`・`SyncSave` は、Replicator の中の Store ごとの mutex で 1 つずつにする(通信どうしは重ねない)。判定(`finish` の `judge`)と結果の数え方は、ロックの中で、送った先端(閲覧のみなら受け取った先端)を基準にする(いまの先端ではない)。`SyncSave`(移動の送り届け)は今のまま、通信のあいだもロックを持つ | トラック C `:5285`。通信は 1 回最大 60 秒・3 回まで。そのあいだ画面の保存が待たされる。オブジェクトは中身で決まる名前なので、受け取りと送りはロックなしで足してよい。送ったあとにできた保存は次の同期で送る。判定は作業ツリーを読むので、保存の途中を読まないようロックの中で行い、送っていない自分の保存を判定済みにしないよう送った先端までにする。`SyncSave` は送れなかったら保存を捨てる(先端を戻す)ので、そのあいだに別の保存が入ってはいけない。`refs/hedgehug/remote/…` は受け取りが書き換えるので、通信どうしは重ねない |
| G1-11 | `Auto` は Store ごとに最後に試みた時刻をメモリに持ち、`due` には state.db の記録とメモリの新しい方を渡す | トラック C `:5289`。記録に失敗すると毎秒「時間が来た」と判定され続ける |
| G1-12 | オフラインの知らせが期限で消えたタブは「期限切れのオフライン」として覚え、そのタブから `online: true` が来たらオンライン復帰の契機にする。画面は `online: true` の知らせの送信に失敗したら、次の `check` で送り直す(成功するまで)。見えないタブのタイマーの間引きは、今の形で足りる(待つ条件は見えているタブだけが送り、オフラインの期限は 2 分)ので変えない | C2 画面 `:2863`・トラック C `:5290` |
| G1-13 | 設定のタブは G1 では増やさない(「一般」「アカウントとメンバー」のまま)。サイドバーの歯車と、パレットの「設定を開く」と、`/settings`(タブなし)の開く先を「一般」に変える。パレットの設定の群には「設定: 一般」を足さない | 分け方の G-3。トラック C `:2065`・C5d-1。C5d-2 が「アカウントとメンバー」にしていた理由(一般が空)はなくなる。パレットの設定の群(02 `:634`)に「一般」はない |
| G1-14 | サイドバーのワークスペースの開閉は、画面の状態(state.db の `screen_states`)の画面 `sidebar` に `{"collapsed": [ワークスペースの内部 ID…]}` で残す | track-a2 `:65`。保存形式 `:286` は画面の状態を state.db に置く。01 `:73` の「折りたたみ」 |
| G1-15 | ログイン名の確定: 前後の空白を取り除いてから比べる。同じなら何もしない(Enter のあとのフォーカスの外れで 2 回確定しても 2 回目は何もしない。確定の処理中は次の確定を受けない)。大文字と小文字だけが違うときは確認を出さずに保存する | 08 `:19`。「あなた」はメンバーと大文字と小文字を区別せずに照合する(01 `:87`)ので変わらない。アカウントの編集の確認(`account.EditLines`)も同じく出さない |
| G1-16 | 確認ダイアログの対象は、アカウントが「なし」の Store(非表示・閲覧のみを含む)。旧ログイン名はいまのマシンのログイン名。旧ログイン名が空なら確認を出さない(言える旧い名前がない)。ダイアログのタイトルは「ログイン名を変更」 | 08 `:21`。タイトルは仕様にないので、操作の名前にした(アカウントの確認と同じ作り) |
| G1-17 | ログイン名を変えたら、「あなた」が変わった Store ごとに `port.Event{Kind: EventStore, Store: id}` を出す(画面はマイタスクや「あなた」の印を読み直す)。同期のタイミングを変えたら `EventSync` を出す | 06「画面への反映」。ほかのタブにも反映する |
| G1-18 | 同期のタイミングの値が選択肢にない(手で書き換えた)ときは、その項目を初期値として扱う(同期もそれで動き、画面もその値を出す) | 08 `:27-32` の選択肢だけを認める。画面に出す値と動く値を同じにする |
| G1-19 | Store の色が空か 7 色にないとき、`StoreList` は「前の行がまだ使っていない最初の色、なければ teal」を返す | 08 `:73` の既定の決め方を読むときにも当てる(手で書き換えたときだけ起きる) |
| G1-20 | 「一般」の「すべて同期」のボタンは、remote を持つ表示中の Store が 1 つ以上あるときだけ出し、`sync.syncAll` を呼ぶ(トーストは同期カードと同じ) | 08 `:36`。C2 画面の「作らなかった入口は sync.syncAll を呼ぶだけ」 |
| G1-21 | e2e のビルドの `Identity` は今の `apptest.Identity{Login:"taro"}` のまま。e2e では、ログイン名を変えても「あなた」は変わらない(画面の確認ダイアログと保存した値だけを確かめる) | e2e の偽の Store にはアカウントを割り当てた Store があり、本物の `Identity` に替えると既存の e2e の「あなた」が変わる |

## Review Focus

- Enter で確定した直後にフォーカスが外れる → 確認ダイアログが 2 つ出たり、2 回保存したりしない(Task 9 のテスト)。
- 本物の HTTP の経路で、メンバーにいない人がアイテムを作る → 同じ保存でメンバーが足され、応答に `Hedgehug-Joined` が付く(Task 6 のテスト)。
- 何も変えない保存(同じ値を入れる)を、メンバーにいない人がする → 保存もトーストも出ない(Task 3 のテスト)。
- 同期が受け取りの通信で待っているあいだに画面から保存する → 保存は通信を待たない(Task 5 のテスト)。
- config.yaml に前回の版の `stores`(`id` なし)が残っている → 起動は止まらず、その行は出ない。アカウントを足すなどで書き直しても起動できる(Task 1 の `TestOldShapeSurvivesRewrite`・Task 6 のテスト)。

---

### Task 1: config.yaml の Store の一覧と読み口

**Files:**
- Create: `internal/infra/machineconfig/stores.go`、`internal/infra/machineconfig/stores_test.go`
- Modify: `internal/infra/machineconfig/file.go`(`config` に鍵を足す)

**Interfaces:**
- Produces:
  - `type storeYAML struct{ ID, Name, Path, Remote string; Mirrors []string; Account, Color string; Hidden, ReadOnly bool }`(鍵は G1-1)
  - `func (f *File) Stores(open func(id port.StoreID) bool) StoresView` — `open` は起動のときに開けた Store か。`StoresView` は次の 3 つを実装する:
    - `port.StoreList`: 開けた・非表示でない行を、config の並びで `port.StoreInfo{ID, Name, Color, ReadOnly}`(色は G1-19)
    - `port.SyncTargets`: 開けた行すべて(非表示・閲覧のみ・remote なしを含む)を `port.SyncTarget`(remote とミラーは G1-9 で直す)
    - `port.StoreGitHub`: `List`(開けた行すべて、`Remote` は remote があるか)、`Unassign`(その `account` の行を空にして書く)、`MachineLogin`(`login`)
  - `func (f *File) StoreEntries() ([]StoreEntry, error)` — `type StoreEntry struct{ ID port.StoreID; Path string }`。wire が起動のときに開くために使う(`id` か `path` が空の行は返さない)

- [ ] **Step 1: 失敗するテストを書く**(`stores_test.go`。config.yaml を一時フォルダに書いて読む)
  - `TestStoreListOrderAndHidden`: 3 行(2 行目 `hidden: true`)→ `List` は 1・3 行目だけ、config の並び
  - `TestStoreListSkipsUnopened`: `open` が false を返す ID は 3 つの一覧のどれにも出ない
  - `TestStoreListColorFallback`: `color` が空・`"grey"` の行は、前の行が使っていない最初の色(teal・orange・purple・sky・pink・olive・brown の順)
  - `TestSyncTargetsSSHRemote`: `remote: git@github.com:org/tasks.git` → `https://github.com/org/tasks.git`。`git@github.com:org/tasks`(.git なし)も同じ。`https://github.com/org/tasks` はそのまま。ミラーも同じ
  - `TestStoreGitHubUnassign`: 2 行が `account: a1` → `Unassign(a1)` 後に読み直すと両方空、ほかの鍵(`accounts`・`hosts`・`sync`)は残る
  - `TestStoreEntriesSkipsOldShape`: 前回の版の形 `stores: [{name: a, path: /x, remotes: [u]}]` を読んでもエラーにならず、`StoreEntries` は空
  - `TestOldShapeSurvivesRewrite`: 前回の版の形の行がある config.yaml で `Unassign` を呼んで書き直したあとも、読み直しがエラーにならない
- [ ] **Step 2: `go test ./internal/infra/machineconfig/ -run 'Store|Sync' ` で落ちることを確かめる**
- [ ] **Step 3: 実装する**(SSH の直しは `^git@github\.com:([^/]+)/(.+?)(\.git)?/?$` → `https://github.com/$1/$2.git`)
- [ ] **Step 4: テストが通ることを確かめる**
- [ ] **Step 5: Commit** `feat(machineconfig): config.yaml の Store の一覧を読む(G1)`

### Task 2: ログイン名と同期のタイミングの書き口、本物の Identity

**Files:**
- Modify: `internal/app/port/storegithub.go`(`SetMachineLogin` を足す)、`internal/app/port/syncsettings.go`(`SetTiming` を足す)、`internal/app/port/identity.go`(`ErrNoLogin`)
- Modify: `internal/infra/machineconfig/stores.go`・`sync.go` とそれぞれのテスト
- Modify: `internal/app/apptest/storegithub.go`・`internal/app/apptest/sync.go`(偽物)
- Create: `internal/app/stores/identity.go`、`internal/app/stores/identity_test.go`

**Interfaces:**
- Produces:
  - `StoreGitHub.SetMachineLogin(ctx, login string) error` — 検査はしない(呼ぶ側が `member.ValidLogin` で確かめる)
  - `SyncSettings.SetTiming(ctx, t port.SyncTiming) error` — `sync` の 4 つの鍵をすべて書く(`auto` は true も false も書く。ほかの 3 つは秒)
  - `var ErrNoLogin = errors.New("このマシンのログイン名が未設定です")`(port/identity.go)
  - `type stores.Identity struct{ GitHub port.StoreGitHub; Accounts port.Accounts }` と `func (i Identity) Me(ctx, store port.StoreID) (string, error)`(G1-5)

- [ ] **Step 1: 失敗するテストを書く**
  - machineconfig: `TestSetMachineLoginKeepsStores`(`login` を書いても `stores`・`accounts` が残る)、`TestTimingSnapsToChoices`(`interval: 120` → 5 分、`push_interval: 45` → 30 秒、`github_interval: 60` → 5 分、選択肢の値はそのまま。G1-18)、`TestSetTimingRoundTrip`(`{Auto:false, 15m, 10s, 30m}` を書いて読むと同じ)
  - `identity_test.go`(偽物の StoreGitHub と Accounts で): アカウントあり → そのログイン名 / アカウントなし → マシンのログイン名 / 割り当てたアカウントが登録にない → マシンのログイン名 / どちらも空 → `errors.Is(err, port.ErrNoLogin)` / 一覧にない Store → `port.ErrStoreNotFound`
- [ ] **Step 2: 落ちることを確かめる** `go test ./internal/infra/machineconfig/ ./internal/app/stores/`
- [ ] **Step 3: 実装する**(Timing は選択肢 `Interval ∈ {1,5,15,30,60}分`・`AfterWrite ∈ {10,30,60}秒`・`GitHubEvery ∈ {5,15,30,60}分` にない値を初期値にする)。偽物に `SetMachineLogin`・`SetTiming` を足す
- [ ] **Step 4: 通ることを確かめる**(`go test ./internal/...`)
- [ ] **Step 5: Commit** `feat(stores): ログイン名と同期のタイミングを書き、Store ごとの「あなた」を決める(G1)`

### Task 3: 「あなた」を書き込みと一緒にメンバーへ足す

**Files:**
- Modify: `internal/app/port/identity.go`(`WithJoined`・`Joined` を足す。`port/revive.go` の `WithRevived`・`Revived` と同じ作り)
- Create: `internal/app/members/join.go`、`internal/app/members/join_test.go`
- Modify: `internal/adapter/httpapi/server.go`(`restoredHeader` の応答で `Hedgehug-Joined` も付ける)、`internal/adapter/httpapi/server_test.go`

**Interfaces:**
- Consumes: Task 2 の `StoreGitHub.List`(閲覧のみの判定)
- Produces:
  - `func port.WithJoined(ctx) context.Context`、`func port.Joined(ctx) []string`(足したログイン名。同じものは 1 回)
  - `func members.JoinOnWrite(s port.Stores, list port.StoreGitHub) port.Stores`(G1-6。`Write` 以外はそのまま渡す)

- [ ] **Step 1: 失敗するテストを書く**(`join_test.go`。`apptest.Stores` とメンバー hanako だけの Store、実行者 taro)
  - `TestJoinWithWrite`: アイテムを作る保存 → コミットは 1 つ、メンバーに taro、`port.Joined(ctx) == ["taro"]`
  - `TestJoinBeforeFn`: `fn` の中の `tx.Members().List()` に taro がいる
  - `TestJoinSkipsEmptyWrite`: 何も書かない `fn` → コミットなし、メンバーは hanako だけ、`Joined` は空、`Write` は nil を返す
  - `TestJoinSkipsMemberCaseInsensitive`: メンバーに `Taro` がいれば足さない
  - `TestJoinSkipsReadOnlyAndNonUser`: 閲覧のみの Store、実行者が同期・GitHub・自動化 → 足さない
  - `TestJoinRolledBackOnError`: `fn` がエラー → メンバーは変わらず、`Joined` は空
  - `server_test.go`: `Joined` に 2 件ある 200 の応答に `Hedgehug-Joined: taro, tarou`、409 の応答には付かない
- [ ] **Step 2: 落ちることを確かめる**
- [ ] **Step 3: 実装する** — 何も書かなかったかは、`fn` に渡す `Tx` を包んで、書き口(`WriteWorkspaces/Projects/Items/Members/Transfers` の `Append`・`Import`・`Describe`・`Remove`)の呼び出しを数えて判定する(自分が足した `Join` は数えない)。数が 0 なら内部の番兵エラーで巻き戻し、外へは nil を返す
- [ ] **Step 4: 通ることを確かめる**
- [ ] **Step 5: Commit** `feat(members): 「あなた」がメンバーにいなければ書き込みと一緒に足す(G1)`

### Task 4: 画面に「あなた」の追加のトーストを出す

**Files:**
- Modify: `web/packages/api/src/fetcher.ts`、`web/packages/api/src/fetcher.test.ts`
- Modify: `web/packages/core/src/features/sync/restored.ts`(`joinedToast` を足す)とそのテスト
- Modify: `web/app/src/wire.ts`(`onJoined` でトースト)

**Interfaces:**
- Produces: `export function onJoined(listener: ((logins: string[]) => void) | null): void`(api)、`export function joinedToast(login: string): string`(core。「{login} をメンバーに追加しました」)

- [ ] **Step 1: 失敗するテストを書く** — `fetcher.test.ts`: `Hedgehug-Joined: taro, tarou` の 200 → 受け手が `["taro","tarou"]`、ヘッダーなし → 呼ばれない、失敗の応答 → 呼ばれない。`restored.test.ts`: `joinedToast("taro") === "taro をメンバーに追加しました"`
- [ ] **Step 2: `pnpm -C web test` で落ちることを確かめる**
- [ ] **Step 3: 実装する**(`notifyRestored` の隣に `notifyJoined`。wire はログイン名ごとに 1 つのトースト)
- [ ] **Step 4: 通ることを確かめる**
- [ ] **Step 5: Commit** `feat(web): メンバーに追加したときのトーストを出す(G1)`

### Task 5: 同期のロックの範囲を狭める

**Files:**
- Modify: `internal/infra/gitstore/replicator.go`、`internal/infra/gitstore/replicator_test.go`(または既存の同期のテストのファイルのうち 600 行に余裕のあるもの)

**Interfaces:**
- 変えない: `Replicator.Sync(ctx, id, remote) (port.SyncResult, error)` の形と結果(G1-10)

- [ ] **Step 1: 失敗するテストを書く** — `TestSyncDoesNotHoldLockWhileFetching`: テスト用の差し込み口(`beforeSend` と同じ形で `duringFetch func()` を足す)の中で `Stores.Write` を呼び、受け取りが終わる前に保存が終わる(チャネルで確かめる。`time.Sleep` を使わない)。同じく `duringSend` の中の `Write` も待たされない。送ったあとの保存は次の同期で送られる(未送信 1)
- [ ] **Step 2: `go test ./internal/infra/gitstore/ -run TestSyncDoesNotHoldLock` で落ちる(デッドロックで時間切れ)ことを確かめる**(`-timeout 30s`)
- [ ] **Step 3: 実装する** — `Sync` を次の順にする(`SyncSave` は変えない。G1-10)。
  1. Store ごとの mutex を取る(`Sync` が終わるまで。`SyncSave` も同じ mutex を取る)。
  2. ロックを取り、`open(root)` で途中の保存の跡を戻し、先端(`start`)とブランチを読み、`baseline` を呼ぶ。ロックを放す。
  3. 試みごとに: `link.fetch`(ロックの外。`refs/hedgehug/remote/*` を書く)→ ロックを取り、`open(root)`・`checkSeen`・`checkRemoteStore`・`integrate`・`repairUnsent` のあと先端を読んで `mine` とする → 閲覧のみなら、そのままロックの中で `finish(theirs, theirs, false)` → ロックを放す。書ける Store は `link.send(mine)`(ロックの外)→ ロックを取り、remote の位置の ref を `mine` にし、`finish(theirs, mine, true)` → ロックを放す。
  4. `finish` の `judge` と `result` は、`repo.Head()` ではなく渡した `mine` を基準にする(`SyncSave` から呼ぶときは送った先端を渡すので結果は変わらない)。
  5. `errRejected` のやり直しは 3 から。`ErrRemoteRewritten` のときの `restoreSeen` はロックの中で行う。
- [ ] **Step 4: gitstore のテストを全部流して通ることを確かめる** `go test ./internal/infra/gitstore/...`(同期の場面のテスト・`cmd/hedgehug` の場面のテストも `go test ./cmd/...`)
- [ ] **Step 5: Commit** `fix(sync): 受け取りと送りのあいだ Store のロックを持たない(G1)`

### Task 6: 本物の部品をつなぐ

**Files:**
- Modify: `cmd/hedgehug/wire.go`(6 か所の `no*` を本物に。起動のときに Store を開く。`JoinOnWrite` を通す)
- Delete: `cmd/hedgehug/nostores.go`
- Create: `cmd/hedgehug/stores_scenario_test.go`

**Interfaces:**
- Consumes: Task 1 の `File.Stores`・`StoreEntries`、Task 2 の `stores.Identity`、Task 3 の `members.JoinOnWrite`
- 組み立て: `stores → JoinOnWrite → NotifyWrites` の順に包む(`deps.Stores` と `svc.Stores`)。`newReplicator` の `noticing.Service` にも本物の `Identity` と `StoreList` を渡す。`deps.SyncTargets` と `deps.StoreList` は同じ `StoresView` から(a5c `:379`)

- [ ] **Step 1: 失敗する場面のテストを書く**(`t.Setenv("XDG_CONFIG_HOME", …)`・`XDG_DATA_HOME` を一時フォルダに。`gitstore.Create` で Store を 1 つ作り、メンバーは hanako だけにする。config.yaml に `login: taro`・その Store の行と、`id` のない前回の形の行を書く)
  - `TestWireOpensConfiguredStores`: `wire(ctx)` の `deps.StoreList.List` がその Store 1 つ、`deps.Identity.Me` が taro
  - `TestWireJoinsYouOnWrite`: `httpapi.New(token, fstest.MapFS{}, deps)` に、トークン付きでアイテムを作る要求を送る(`httptest`)→ 応答が 2xx で `Hedgehug-Joined: taro`、Store のメンバーに taro。同じ要求をもう一度送ると、ヘッダーは付かない
  - `TestWireSkipsUnopenable`: `path` を消した行は出ず、`wire` はエラーを返さない
- [ ] **Step 2: `go test ./cmd/hedgehug/ -run TestWire` で落ちることを確かめる**
- [ ] **Step 3: 実装する**(開けなかった行は G1-2 のとおり `slog.Warn`。`wire.go:103` 付近の「G までないので通知は出ない」のコメントも直す)
- [ ] **Step 4: `go test ./cmd/... ./internal/...` と `golangci-lint run --build-tags e2e ./...` が通ることを確かめる**
- [ ] **Step 5: Commit** `feat(wire): config.yaml の Store を開き、本物の一覧と「あなた」をつなぐ(G1)`

### Task 7: Auto の取りこぼしを直す

**Files:**
- Modify: `internal/app/syncing/auto.go`、`internal/app/syncing/auto_test.go`(か、600 行に余裕のある既存のテスト)
- Modify: `web/packages/core/src/features/sync/signals.ts`、`signals.test.ts`

- [ ] **Step 1: 失敗するテストを書く**
  - `TestAutoRemembersAttemptWhenRecordFails`: `SyncState` の記録が失敗する偽物で、間隔 5 分 → 1 回同期したあと、次の `Step`(1 秒後)では同期しない(G1-11)
  - `TestAutoOnlineAfterOfflineLapsed`: `Network(tab,false)` → 2 分 1 秒進めて `Waiting()`(期限切れで消える)→ `Network(tab,true)` で次の `Step` が全 Store を同期する(G1-12)
  - `signals.test.ts`: オフライン → オンラインで `network(tab,true)` が reject → 次の `check` でもう一度 `network(tab,true)` を送る。成功したらそれ以上送らない
- [ ] **Step 2: 落ちることを確かめる**
- [ ] **Step 3: 実装する**(`Auto` に `tried map[port.StoreID]time.Time` と `lapsed map[string]bool`。`waiting` で消すときに `lapsed` へ移す)
- [ ] **Step 4: 通ることを確かめる**
- [ ] **Step 5: Commit** `fix(sync): 記録に失敗しても毎秒同期せず、期限切れのあとのオンライン復帰を取りこぼさない(G1)`

### Task 8: ログイン名と同期のタイミングの API

**Files:**
- Create: `internal/app/stores/login.go`、`internal/app/stores/login_test.go`
- Create: `internal/adapter/httpapi/stores/login.go`、`login_test.go`
- Create: `internal/adapter/httpapi/syncing/timing.go`、`timing_test.go`
- Modify: `internal/domain/account/impact.go`(`LoginLines` を足す)、`internal/adapter/httpapi/routes.go`、`httpapi.Deps`(`SyncSettings port.SyncSettings` を足す。`StoreGitHub`・`Events` は既存)、`cmd/hedgehug/wire.go`・`wire_e2e.go`(本物は `file.SyncSettings()`、e2e は `apptest` の偽物。e2e の `apptest.StoreGitHub`(`wire_e2e.go:93`)に、アカウントなしの非表示の行 `{ID: "e2e-local", Name: "local", Hidden: true}` を足す。非表示なので GitHub の自動の読み取りの対象にならず、アカウントに割り当てていないので `accounts.spec.ts` の削除・編集の確認の行は変わらない)、`api/openapi.yaml` と web の API クライアント(`make generate`)

**Interfaces:**
- Produces:
  - `func account.LoginLines(from, to string, uses []StoreUse) []string` — いまの `identityLines` を出したもの(G1-15・G1-16)
  - `type stores.Login struct{ GitHub port.StoreGitHub; Events port.Events }` と
    - `Get(ctx) (string, error)`
    - `Preview(ctx, login string) ([]string, error)` — 確かめて、確認の本文の行(空なら確認なし)。使えなければ `refusal` の断り(文言は Global Constraints)
    - `Set(ctx, login string) error` — 前後の空白を除き、`Preview` と同じ検査で断る(画面を通らない呼び出しでも、使えない名前を保存しない。保存すると以後のすべての保存が `port.Operation` の検査で失敗する)。同じなら何もしない。保存したら G1-17 の出来事
  - API:
    - `GET /api/settings/login` → `{login}`
    - `POST /api/settings/login/preview` `{login}` → `{lines: string[]}`(400 で断りの文言)
    - `PUT /api/settings/login` `{login}` → 204
    - `GET /api/sync/timing` → `{auto: bool, intervalMinutes, afterWriteSeconds, githubMinutes}`
    - `PUT /api/sync/timing` 同じ本文 → 204(選択肢にない値は 400「選べない値です」)。保存したら `EventSync`

- [ ] **Step 1: 失敗するテストを書く**
  - `login_test.go`(app): 2 つの Store(team-a はアカウントなし、team-b はあり)、マシンのログイン名 taro → `Preview("tarou")` は `["team-a での あなた が @taro から tarou に変わります。"]`、`Preview("Taro")` は空、`Preview(" tarou ")` は前後を除いて同じ、`Preview("ta ro")`・`Preview("")`・40 文字は断り、マシンのログイン名が空なら空、`Set("tarou")` で team-a の `EventStore` が 1 つ、`Set("taro")`(同じ)は何もしない、`Set("ta ro")` は断ってマシンのログイン名は taro のまま
  - httpapi の 2 つのテスト: 上の各経路の状態コードと本文
- [ ] **Step 2: 落ちることを確かめる**
- [ ] **Step 3: 実装する**(`identityLines` を `LoginLines` として出し、`EditLines`・`DeleteLines` はそれを呼ぶ)。`make generate`
- [ ] **Step 4: `go test ./...` と `make check` が通ることを確かめる**
- [ ] **Step 5: Commit** `feat(settings): ログイン名と同期のタイミングを読み書きする API(G1)`

### Task 9: 設定の「一般」

**Files:**
- Create: `web/packages/core/src/features/settings/general.ts`、`general.test.ts`(選択肢・表記・ログイン名の確定の判定)
- Create: `web/packages/core/src/port/settings.ts`(ログイン名とタイミングの口)、api パッケージの実装
- Modify: `web/app/src/screens/settings/GeneralSection.svelte`、`web/app/src/wire.ts`
- Modify: `web/packages/core/src/features/router/route.ts`(`/settings` の既定のタブ)、`web/app/src/shell/Sidebar.svelte`(歯車)、`web/packages/core/src/features/palette/actions.ts`(「設定を開く」)とそれぞれのテスト
- Modify: `web/e2e/settings.spec.ts`

**Interfaces:**
- Produces(core):
  - `intervalChoices = [1,5,15,30,60]`、`afterWriteChoices = [10,30,60]`、`githubChoices = [5,15,30,60]` と `minutesLabel(n) => "{n} 分ごと"`、`secondsLabel(n) => "{n} 秒"`
  - `createLoginField(port)`: `commit(text)` は前後を除いて同じなら何もしない・処理中の 2 回目は捨てる・本文の行があれば確認を求める・断りなら文言を返して元の値に戻す
  - `keyboardRows`(01 `:211-215` の 5 行を、キーと動作の文のまま)と `treeKeyRows`(04 `:559-572` の表のまま。アイテムの行・プロジェクトの行の 2 列)

- [ ] **Step 1: 失敗するテストを書く**(`general.test.ts`): 同じ値の `commit` は口を呼ばない、Enter とフォーカスの外れの続けての `commit` は 1 回だけ、本文の行が 1 つ以上なら確認を求め、キャンセルで元の値、断りの文言を返す。`route.test.ts`: `/settings` は `general`。`actions.test.ts`: 「設定を開く」の行き先は `general`
- [ ] **Step 2: 落ちることを確かめる**
- [ ] **Step 3: 画面を作る** — 上から、ログイン名の入力欄 → 時計のずれの警告(C5d のまま)→ 見出し「同期のタイミング」と注意書き・チェックボックスと 3 つの選択(チェックがオフなら 3 つを無効)・「すべて同期」(G1-20)→ 見出し「キーボード」の表 → 見出し「ツリー」の表。確認ダイアログは ui の確認ダイアログ(タイトル「ログイン名を変更」、行を並べ、「変更」は警告色)。日本語変換中の Enter では確定しない(`isComposing`)
- [ ] **Step 4: e2e を足す**(`settings.spec.ts`): 歯車で「一般」が開く / ログイン名を `tarou` にして Enter → 確認ダイアログに「local での あなた が @taro から tarou に変わります。」(Task 8 で足した偽物の行)→「変更」で欄が `tarou`、もう一度開いても `tarou` / `ta ro` で断りの文言と元の値 / 間隔を「15 分ごと」にして再読み込みしても「15 分ごと」/ チェックを外すと 3 つが無効。最後に値を元に戻す(worker のサーバーを共有するため)
- [ ] **Step 5: `make web-check` と `make e2e` が通ることを確かめる**
- [ ] **Step 6: Commit** `feat(settings): 「一般」にログイン名・同期のタイミング・キーボードの一覧を置く(G1)`

### Task 10: サイドバーの開閉を残す

**Files:**
- Modify: `web/app/src/shell/Sidebar.svelte`
- Create: `web/packages/core/src/features/shell/collapsed.ts`、`collapsed.test.ts`

**Interfaces:**
- Produces: `createCollapsed(states: ScreenStates): { has(id): boolean; toggle(id): void; ready: Promise<void> }` — 画面 `sidebar` の `{"collapsed": string[]}` を読み書きする(G1-14)

- [ ] **Step 1: 失敗するテストを書く** — 読めた `{"collapsed":["w1"]}` で `has("w1")`、`toggle("w2")` で `write("sidebar", {collapsed:["w1","w2"]})`、壊れた値・読めないときは空
- [ ] **Step 2: 落ちることを確かめる**
- [ ] **Step 3: 実装して `Sidebar.svelte` の `SvelteSet` を置き換える**(`A2` のコメント「このタブのあいだだけ」も直す)
- [ ] **Step 4: e2e(`sidebar.spec.ts`)に 1 つ足す** — ワークスペースを閉じて再読み込みしても閉じている。最後に開き直す
- [ ] **Step 5: `make web-check` と `make e2e` が通ることを確かめる**
- [ ] **Step 6: Commit** `feat(shell): サイドバーのワークスペースの開閉を画面の状態に残す(G1)`

### Task 11: 申し送りの元の計画と記録を合わせる

**Files:**
- Modify: `docs/superpowers/plans/2026-11-15-hedgehug-track-c5d.md`(C5d-2 の開く先に「G1-13 で一般に変えた」と 1 文)、`docs/superpowers/plans/2026-11-13-hedgehug-track-d2.md`(D2-14 に「G1-3 で閉じた」)、`docs/superpowers/plans/2026-11-13-hedgehug-track-c2.md`(`:5285`・`:5286`・`:5289` に「G1 で直した」)

- [ ] **Step 1: 各行に、G1 の決定の番号を 1 文で書き足す**
- [ ] **Step 2: Commit** `docs: G1 で閉じた申し送りを元の計画に書く`
