# hedgehug 設計: パッケージ構成

機能仕様(`2026-11-11-hedgehug-functional-spec/01〜08`)と保存形式の設計(`2026-11-12-hedgehug-storage-design.md`)を実装するときの、サーバー(Go)と画面(TypeScript・Svelte)のパッケージの分け方と、守るルールを決める。技術構成は保存形式の設計の「技術構成」の節のとおりである。

## この設計で防ぐこと

前回の実装で困ったことと、この設計での対策。

| 前回困ったこと | 対策 |
|---|---|
| 層やパッケージの境目があいまいで、1 つ直すと関係ないところが壊れた | 依存の向きを表で決め、lint とテストの 2 つで強制する(1・2 節、5 節) |
| パッケージが大きくなりすぎて読めなくなった | 機能ごとにパッケージを分け、ファイル・関数・パッケージの大きさに上限を置いて CI で落とす(4 節) |
| テストが書きにくく遅かった(git や GitHub に直接つないでいた) | usecase は「口」(インターフェース)越しにしか外に触らない。テストはメモリ上の偽物で動かす。本物と偽物は同じ照合テストで確かめる(3 節) |
| 画面のコンポーネントが巨大で、表示とロジックが混ざっていた | 画面を core(ロジック)・api・ui(見た目)・app(結び付け)の 4 つに分ける(6 節) |

あわせて、次の 3 つに備える。

- CLI は後回しにするが、あとから足せる(`adapter/cli` を足し、cmd に起動のしかたを 1 つ足すだけ)。
- Git で管理しない版を出すかもしれない。保存の口を git の言葉を使わずに決め、infra の実装を差し替えられるようにする。Git でない版の中身は、整理ができてから決める。
- 別のプラットフォームでの展開(まだ決まっていない)。画面のロジックを Svelte に依存しない素の TypeScript に置き、ui と app を作り直せば移れるようにする。

## 1. サーバーのパッケージ

```
cmd/hedgehug/            起動と組み立て。infra の実装を app の口に差し込むだけ(ここだけが全部を知る)

internal/domain/         規則だけ。標準ライブラリとほかの domain 以外を import しない。入出力をしない
  item/                  アイテム、種類と親子、Step、コメント、作業時間、関連、PR・Issue のリンク
  project/               プロジェクトと ID(WEB-P1)
  workspace/             接頭辞、プロパティ・値、ステータスと分類、ワークフロー、テンプレート
  automation/            自動化のルールと、きっかけ・条件の判定
  projectrule/           プロジェクト別のルール(使わないステータスなど)
  planning/              スプリント、リリース
  member/                メンバーと「あなた」
  change/                変更の記録(何を何から何に変えたか)。履歴とアクティビティの元
  query/                 クエリの解析と照合
  repair/                同期・診断の修復(親の循環、番号の重なりなど)。状態を受け取り、直す内容を返す

internal/app/            usecase。1 つの操作を 1 ファイル 1 関数にする
  port/                  口の定義(3 節)
  items/  projects/  workspaces/  planning/  reports/  search/
  sync/  inbox/  activity/  github/  stores/  maintenance/
  apptest/               口のメモリ上の偽物と、口ごとの照合テスト

internal/infra/          口の実装。外部のライブラリを使ってよいのは infra と adapter と cmd だけ
  gitstore/              Store を git で持つ実装(log.jsonl・item.yaml、ファイルロック、同期の合わせ方)
  statedb/               このマシンのデータ(SQLite。索引、受信箱、保存したビュー、設定)
  github/                GitHub の API
  secret/                トークン(キーチェーン)
  sysclock/              時計と内部 ID の採番

internal/adapter/        外とのつなぎ
  httpapi/               HTTP の API(huma)、Server-Sent Events、画面のファイルの配信
  cli/                   (あとで足す)httpapi と同じく app だけを呼ぶ

internal/archtest/       依存の向きと大きさの上限を確かめるテスト(5 節)

web/                     画面(6 節)
api/openapi.yaml         huma が出す OpenAPI。画面の api パッケージはここから作る
```

機能のパッケージ(app/<機能>)は、仕様の文書と次のように対応する。1 つのパッケージが 4 節の上限を超えそうなら、操作のまとまりで分ける(例: `app/items` から `app/itemmove` を分ける)。

| app のパッケージ | 主な仕様 |
|---|---|
| items、projects | 02 |
| workspaces | 03 |
| search | 04 |
| planning、reports | 05 |
| sync、inbox、activity | 06 |
| github | 07 |
| stores、maintenance | 08 |

## 2. サーバーの依存の向き

```
cmd ──→ adapter ──→ app ──→ domain
  └───→ infra ──→ app/port ─┘
```

| パッケージ | import してよいもの |
|---|---|
| domain/* | 標準ライブラリと、ほかの domain |
| app/port | domain |
| app/<機能> | domain、app/port。ほかの app/<機能> は import しない。機能をまたぐ規則は domain に置く |
| app/apptest | domain、app/port |
| infra/<実装> | domain、app/port、外部ライブラリ。ほかの infra は import しない |
| adapter/* | app、domain、外部ライブラリ。infra は import しない |
| cmd | すべて |

- テストのファイルも同じ表に従う。例外は、どのパッケージのテストからも `app/apptest` を import してよいことだけである。
- domain の中の循環は Go が禁止するので、domain どうしの向きは決めない。

## 3. 口(app/port)

口は「何をしたいか」の言葉で決め、git の言葉(コミット、ブランチ、clone)を出さない。git の言葉が出るのは `infra/gitstore` の中だけである。

### Store のデータの口

```go
// 1 回の操作 = 1 回の Write。git 版では 1 コミットになる。
type Stores interface {
    Read(ctx context.Context, storeID StoreID, fn func(r Reader) error) error
    Write(ctx context.Context, storeID StoreID, op Operation, fn func(tx Tx) error) error // fn がエラーなら何も残らない
}

type Operation struct {
    Actor   change.Actor // あなた・自動化・同期・GitHub
    Summary string       // アクティビティの 1 行
}

type Reader interface {
    Workspaces() WorkspaceReader
    Projects() ProjectReader
    Items() ItemReader
    Planning() PlanningReader
    Members() MemberReader
    Changes() ChangeReader // 履歴とアクティビティ
}

type Tx interface {
    Reader
    WriteWorkspaces() WorkspaceWriter
    WriteProjects() ProjectWriter
    WriteItems() ItemWriter
    WritePlanning() PlanningWriter
    WriteMembers() MemberWriter
}
```

- usecase は domain の型を読み書きするだけである。変更の記録は `domain/change` の型で、git 版では log.jsonl の行になる。
- Store をまたぐ移動は、2 つの Store への Write を組み合わせる。途中で失敗したときの扱いは仕様 02・03 のとおりで、usecase が持つ。
- 種類ごとの Reader・Writer の関数は、各機能の計画で、その機能が使うものだけを足していく。先回りして足さない。

### Git の版だけが持つ口

| 口 | すること |
|---|---|
| `Replicator` | 同期(受け取る・合わせる・送る)、ミラーへの送信、「remote の履歴にそろえる」 |
| `StoreProvisioner` | Store を作る・clone する・フォルダから探す・このマシンから外す |

- 画面は、サーバーから「この Store で使える機能」を受け取り、使えない機能を出さない。git 版ではすべて使える。
- 合わせたあとの修復(親の循環など)は、`domain/repair` の規則を `app/sync` が呼び、Stores.Write(実行者「同期」)で保存する。項目ごとの後勝ちで合わせる処理は `infra/gitstore` の中に置く。
- Git でない版の口の中身は、整理ができてから決める。

### このマシンと外部の口

| 口 | すること | 実装 |
|---|---|---|
| `Index` | Store をまたぐ検索と並べ替え | infra/statedb |
| `Inbox` | 通知と既読 | infra/statedb |
| `Views` | 保存したビュー | infra/statedb |
| `MachineSettings` | 一般の設定、アカウントの一覧、Store ごとの色・非表示・アカウントの割り当て | infra/statedb |
| `GitHub` | PR・Issue・リポジトリ・コラボレーターの読み取り、デバイスフロー | infra/github |
| `Secrets` | トークン | infra/secret |
| `Clock`・`IDs` | 現在時刻、内部 ID | infra/sysclock |
| `Events` | 「Store が変わった」「受信箱が変わった」を画面に知らせる | adapter/httpapi(SSE) |

- `Events` だけは adapter が実装する。adapter は app を import してよいので、依存の向きは崩れない。CLI や他のタブが同じ Store に書いたときは、gitstore が変化を見つけて Events に流す。

### テストのための仕組み

- `app/apptest` に、すべての口のメモリ上の偽物を置く。usecase のテストは偽物だけで動き、git も GitHub もつながない。
- 口ごとに照合テスト(この口はこう振る舞うべき、というテストの集まり)を `app/apptest` に関数として置く。同じテストを偽物と本物(gitstore・statedb)の両方に流し、偽物が本物とずれないことを確かめる。Git でない版を作るときも、このテストを通せば差し替えられる。
- GitHub の実装は、httptest の偽のサーバーで確かめる。本物の GitHub に対する確かめは、前回と同じく週 1 回の CI(7 節)で行う。

## 4. 大きさの上限

数字は仮の値で、超えたら CI で落とす。超えたときは、上限を上げずに分ける。上げるのは、ユーザーと相談して決めたときだけである。

| 対象 | 上限 | 確かめ方 |
|---|---|---|
| Go のファイル | 300 行(テストは 600 行。生成したファイルは除く) | archtest |
| Go の関数 | 60 行、認知的複雑度 15 | golangci-lint の funlen・gocognit |
| Go のパッケージ | テスト以外のファイル 20 個 | archtest |
| usecase | 1 ファイルに 1 操作 | レビュー |
| Svelte のコンポーネント | 200 行 | web の確認用スクリプト |
| TypeScript のファイル | 300 行(生成したファイルは除く) | web の確認用スクリプト |

## 5. ルールの強制

- **Go:** golangci-lint の depguard で 2 節の表を書く。加えて `internal/archtest` が `go list -deps -json` の結果から import を調べ、2 節の表と 4 節の上限に違反したら落ちる。lint の設定漏れも、このテストで拾う。
- **画面:** パッケージごとの package.json に書いた依存しか解決できないよう、pnpm の厳格な設定にする。あわせて dependency-cruiser で 6 節の表を確かめる。

## 6. 画面のパッケージ

```
web/                         pnpm workspace
  packages/core/             素の TypeScript。Svelte も DOM も知らない
    port/                    サーバーとの口(読み書き、変更の知らせ)、時計
    model/                   画面で使う型(API の型をそのまま使わない。api が変換する)
    reactive/                小さな購読の仕組み(Svelte のストアの約束どおり subscribe を持つ)
    features/<機能>/          機能ごとの状態・操作・入力の検査・表示用の値(例: ツリーの並び、カンバンの列)
    query/                   検索窓のクエリの解析と候補
  packages/api/              orval で作る API クライアントと、core の口の実装、SSE の受信
  packages/ui/               見た目だけの Svelte 部品。props を受けて events を返す
    tokens/                  色・余白・文字・ブレークポイント(CSS 変数)
    primitives/              ボタン、入力、セレクト、ダイアログ、メニュー、トースト、タブ
    patterns/                ツリーの行、カンバンのカード、表、ガントのバー、ピル、検索窓
    layout/                  全体の枠、サイドバー、分割ペイン、狭い幅での積み重ね
  app/                       Svelte の画面。core の状態と ui の部品を結び付けるだけ
    screens/<画面>/           ワークスペース、プロジェクト、検索、受信箱、設定 など
    main.ts                  組み立て(api の実装を core に渡す)
  e2e/                       Playwright の通しのテスト
```

| パッケージ | import してよいもの |
|---|---|
| core | なし(標準の TypeScript だけ) |
| api | core |
| ui | Svelte だけ |
| app | core、api、ui |

- **UI とロジックの分け方:** 画面に出す値の計算、入力の検査、操作の流れ(ダイアログの開閉、確認、送信、失敗の表示)は core に置く。ui の部品は、渡された値を出し、利用者の操作を events で返すだけにする。app の画面は、core の状態を ui の部品に渡し、events を core の操作につなぐだけにする。
- **別の UI 技術に移るとき:** core と api をそのまま使い、ui と app を作り直す。
- **レスポンシブ:** ui の部品は、画面の幅ではなく、置かれた枠の幅で形を変える(コンテナクエリ)。狭い枠での形を、部品ごとに決める。
- **ツール:** 前回と同じ pnpm、Biome、svelte-check、Vitest、Playwright を使う。

### 画面のテスト

- core は Vitest だけで確かめる(DOM なし)。
- ui は部品ごとに Vitest と Testing Library で確かめる。
- app は Playwright で、スマホ・タブレット・デスクトップの 3 つの幅で通しで確かめる。サーバーは、口に `app/apptest` の偽物を差し込んだもので起動する(e2e 用のビルドタグで組み立てを切り替える)。

## 7. CI とツール

前回の CI とツールを、パスを新しい構成に合わせて引き継ぐ。

| ワークフロー | すること |
|---|---|
| ci-backend | go mod verify、go.mod・go.sum の整い、golangci-lint の fmt と lint、テスト(race、OS とシャードの行列、カバレッジ) |
| ci-frontend | pnpm install、API の型が最新か(orval の生成物の差分)、Biome、svelte-check、Vitest、ビルド、Playwright |
| github-live | 週 1 回、本物の GitHub に対して infra/github を確かめる(GitHub の計画で足す) |
| release | goreleaser(mise で入れる) |
| security | 今あるものをそのまま使う |

- ツールの版は mise で決める。手元の実行は Makefile にまとめる(前回と同じ名前の generate・build・web・web-check・e2e・fmt・lint・test・cover)。
- ci-backend に、`internal/archtest` を含むテストがそのまま入る。

## 8. 実装の分け方

最初に短い「土台」を作り、複数の作業が同時に触るもの(共有部分)を先に固める。土台は、共通の骨組みを 1 本作ったあと、触るフォルダが重ならない Go と画面の 2 本に分けて並べて作る(2026-11-12 決定)。そのあとは、機能ごとの縦割りの組(トラック)を、別々のブランチ・worktree で並べて進める。

### 段階 0: 土台(共通の骨組みのあと、Go と画面の 2 本)

| 作るもの | 理由 |
|---|---|
| パッケージの骨組み、archtest・depguard・dependency-cruiser、CI(7 節) | 全トラックの前提 |
| domain の中心の型(アイテム、ワークスペース、プロジェクト、メンバー、変更の記録) | ほぼ全トラックが使う |
| 口(app/port)の定義、apptest のメモリ上の偽物とサンプルデータ | 各トラックが、ほかのトラックを待たずに偽物の上で作れる |
| gitstore の読み書き(1 回の操作 = 1 コミット、log.jsonl・item.yaml) | 保存の実体 |
| httpapi の骨組み。ルートの登録は機能ごとのファイルに分ける | ルートの追加で衝突しないため |
| web の 4 パッケージの骨組み、ui のトークンと基本部品(ボタン、ダイアログ、メニュー、トースト等)、画面の全体の枠 | 見た目の土台をそろえるため |

### 段階 1: トラック

| トラック | 中身 | 主な仕様 | 待つもの |
|---|---|---|---|
| A アイテムとプロジェクト | 作成・編集・移動・削除、詳細パネル、ツリー | 02 | 土台 |
| B ワークスペースの設定 | プロパティ、ワークフロー、テンプレート、自動化、プロジェクト別のルール | 03 | 土台 |
| C 同期と通知 | 合わせ方(後勝ち)、同期の修復、受信箱、他のタブへの反映 | 06 | 土台 |
| D GitHub | アカウント、デバイスフロー、PR・Issue のリンク、自動リンク | 07 | 土台 |
| E 検索とレイアウト | クエリ、カンバン・テーブル・ガント、保存したビュー、パレット、一括変更 | 04 | A |
| F 計画とレポート | スプリント、リリース、バックログ、レポート | 05 | A |
| G Store の運用と保守 | 複数の Store、別の Store への移動、削除済み、診断、ミラー、フォルダから探す | 08 | A、C |

- 同時に動かすトラックは 4 つまで。第 1 波は A・B・C・D。終わったものから E・F・G を入れる。
- 各トラックは、計画を書く → エージェントがタスクごとに実装 → タスクごとにレビュー、で進める。

### 計画を書く時期

- 土台の計画を最初に書く。
- 土台を実装しているあいだに、A・B・C・D の計画を並べて書く。
- E・F・G の計画は、待つトラックが develop に入ってから書く(先に書くと、入った実装と食い違うため)。
- 各計画の冒頭に、持ち場のフォルダと、待つトラックを書く。

### 衝突を防ぐルール

- **持ち場:** 各トラックは、自分の機能のフォルダだけを触る(`domain/<機能>`、`app/<機能>`、`adapter/httpapi/<機能>`、`web/packages/core/features/<機能>`、`web/app/screens/<機能>`)。
- **共有部分の変更:** 口、domain の中心の型、ui の基本部品、cmd の組み立てを変えるときは、その変更だけの小さな PR を先に develop へ入れ、ほかのトラックは取り込む(rebase)。
- **生成物:** `api/openapi.yaml` と web の API クライアントは、手で衝突を直さない。rebase のあとに `make generate` で作り直す。最新かどうかは CI で確かめる。
- **小さく入れる:** トラックの中でもタスク単位で PR を作り、こまめに develop へ入れる。
- **画面の見本:** 画面の見た目はモック版 19(https://claude.ai/artifact/Qw2jqgdbYtbcA4VTMHGCVA)に合わせる。

CLI はトラックに入れない。仕様を別の段階で決めてから、`adapter/cli` として足す。
