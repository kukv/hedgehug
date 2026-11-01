# フェーズ 3: 複数 Store とアカウント 設計

- 日付: 2026-11-01
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「実装フェーズ」3. 複数 Store とアカウント、「Store とアカウント」の章、「コンテキスト解決」、「メンションと通知」、「GUI 仕様」の「複数 Store の見せ方」（CLI に当たる部分）、「開発環境」
- 前段: `docs/superpowers/specs/2026-10-31-phase2-sync-merge-design.md`（ほか 1a〜1.5 の spec）

## 位置づけ

フェーズ3は1つの spec で設計し、plan・実装・PR を 3a → 3b → 3c の3つに分ける（利用者の決定）。依存の順に並べた。

| 段 | 節 | 内容 |
| --- | --- | --- |
| 3a | 設定と認証、GitHub API と fakegh、Store の作成と発見 | 認証、`internal/github`、fakegh、`store init/discover`、読み取り専用、メンバー一覧 |
| 3b | マウントと統合ビュー、コンテキスト解決 | `internal/mount`、Store をまたぐクエリと ID の解決、参照の検査、コメントの本人確認、メンションの警告、`internal/context`、`hh use` |
| 3c | Store 間の移動、履歴の書き換え | `mv --store`、転送ファイル、`purge --moved`、`purge --rewrite-history` と他のクローンでの取り込み |

devenv とテストは、各段で動く範囲を足す（「開発環境」「テスト」）。完了条件の E2E は 3c で通す。

親設計がフェーズ3に挙げたものはすべて入れる。

- ローカル設定、4種類の認証（gh / env / keychain / Device Flow）、Store のマウントと発見
- Store をまたぐクエリ、Store 内に閉じた参照の検査、読み取り専用マウント、Store 間の移動（移動先での振り直しを含む）と履歴からの除去、接頭辞が重複した Store の Store 名付きの表示、コンテキスト解決、`hh use`
- 開発環境: `url_rewrites`、`hh store init` のローカル分岐、`hh store discover --from`
- 完了条件: 個人 Store と2つの Org の Store を、別々のアカウントで同時に扱える

親設計で別の章に書かれ、フェーズ3の機能が要るので、次もフェーズ3で作る。

- メンバー一覧（「Store とアカウント」の「メンバー」）と、Store を読む権限がない人をメンションしたときの警告（フェーズ2でメンバー一覧を待っていたもの）
- コメントの編集・削除の本人確認（1d-2 でアカウントを待っていたもの）
- GHES 向けの、ホストごとの API の場所と client ID の上書き（「GitHub App」）
- `internal/github` と、フェーズ3の機能が使う範囲の fakegh（利用者の決定。親設計の「作る時期」ではフェーズ5だった）

### フェーズ2で作り済みのもの

`accounts[]` の `name`・`host`・`login` の読み込み、`stores[].account`、「自分」と `@me`、`created_by` とコメントの `author`、コミットの author、`hh store add/list/remove`、`sync` の設定。フェーズ3はこれらを前提にする。

### フェーズ3で扱わないもの

- `repo_accounts` を使う処理（GitHub 連携でのアカウントの選択）: フェーズ5。読み込みと検査はフェーズ3で作る
- GraphQL のクライアントと、fakegh の PR・Issue・レビュー・checks: フェーズ5
- `automations.yaml` の `where` 付きルールを Store 間の移動で持っていくこと: ルールの形式を決めるフェーズ5で、自動化と一緒に作る
- 担当者の入力補完（メンバー一覧を使う）: GUI はフェーズ4、シェル補完はフェーズ7
- GUI の Store の色・バッジ・表示切り替え: フェーズ4（`internal/mount` を使う）

### 用語

フェーズ1.5・2の呼び方に合わせる。人が読める識別子（`WEB-42`）は **ID**、内部の識別子は **ULID**。ほかに次を使う。

- **マウント**: config.yaml の `stores[]` に登録した Store。マウント中の Store の集合を `internal/mount` が扱う
- **Store 名付きの ID**: `org-a:API-12` の形。入力ではどこでも受け付け、表示では接頭辞が Store 間で重複している間だけ使う
- **転送ファイル**: Store 間の移動で移動元に残す、`ulid` と `moved_to` だけを持つタスクファイル
- **repo ID**: remote URL を正規化した `github.com/org-a/api` の形

## 利用者の決定（2026-11-01）

| 項目 | 決定 |
| --- | --- |
| fakegh | フェーズ3の機能が使う範囲を前倒しで作る |
| GitHub App | まだ登録していない。登録手順を付録に書き、client ID は登録後に入れる |
| 骨組み | Store ごとの index はそのままにし、上に `internal/mount` を作る（案A） |
| `hh ls` の表示 | マウント中の Store が2つ以上なら、先頭に STORE の列を足す |
| `--store` のない読み取り | マウント中の全 Store を対象にする。ID は全 Store から引き、絞れないときだけ `org-a:API-12` を求める |
| コメントの本人確認 | author が自分のいずれかのアカウントのときだけ編集・削除できる。author が空なら誰もできない |
| 既定の Store | config.yaml の `default_store`。最初に init / add した Store を自動で書き、`hh store default` で変える |
| 履歴の書き換えの取り込み | 他のクローンでは、作り直したクローンに未 push の変更を3-wayマージで自動で戻す |
| 移動先のスキーマに合わないタスク | 拒否し、タスクごとの違反を並べる |
| 移動と同時の変更 | 転送ファイルを勝たせ、負けた変更を記録して通知する |
| 分割 | spec は1つ、plan・実装・PR を 3a・3b・3c に分ける |

3a の実装中に質問が続いたため、設計を見直した（`docs/superpowers/specs/2026-11-01-phase3-design-review.md`。決1〜決20 を利用者が推奨のとおり承認した）。結論はこの spec の各節に書いてあり、横断的な決まりは次の「共通の決まり」にまとめた。

## 共通の決まり

各節はこの決まりに従う。各節に別の書き方があれば、その節に理由を書く。

### 処理の置き場所

フェーズ3の処理は `internal/ops` に置く（フェーズ4a の後の形。GUI の 4f がそのまま呼ぶため）。`cmd/hedgehug` に残すのは、端末の入出力（質問、標準入力のトークン、Device Flow のコードの表示、ブラウザを開くこと）と表示だけ。トークンの取得（`authOptions`・`tokenFor`）、git の通信（`connFor`）、GitHub のクライアント（`githubFor`）、アカウントの選び方は `ops.Session` のメソッドにする。

### 秘密の送り先

- git の通信: 書き換え後の URL の host（ポートを除く）が、その Store のアカウントの host と同じときだけトークンを付ける（「git の通信」）。
- GitHub API・Device Flow: アカウントの host の `api`・`web`（既定値を含む）と、オリジン（スキーム・host・ポート）が同じ URL にだけトークンを送る。`Link` の `next` などが別のオリジンを指したら、トークンを付けずにエラーにする。リダイレクトも同じで、別のオリジンへのリダイレクトは追わずにエラーにする。remote の host がアカウントの host と違うときは、そのアカウントで GitHub API を呼ばない（`store init` はエラー `account.host_mismatch`、同期の権限の取得は警告）。
- 平文の通信にはトークンを送らない（利用者の決定。2026-11-01）。送ってよいのは `https://` と、ループバック（`127.0.0.0/8`、`::1`、`localhost`）への `http://` だけ（devenv・fakegh・テストのため）。それ以外の `http://` の remote は、トークンを付けずにエラー（`auth.insecure`。「<URL> uses plain http, so the token is not sent; use https」の意味）にする。アカウントにトークンがなければ、今までどおり認証なしで試す。`hosts[].api`・`web` にループバック以外の `http://` を書いたら読み込みのエラー。
- トークンを、エラー文・警告・ログ・`--json`・コマンドラインの引数に出さない。受け取るのは標準入力（`--with-token`）、環境変数、keychain、gh の標準出力だけ。gh の標準エラーをエラーに添えるときは、トークンの形（`gho_`・`ghp_`・`ghs_`・`ghu_`・`github_pat_` で始まる語）を `***` に伏せる。

### 大文字小文字

| もの | 比べ方 | 保存・表示 |
| --- | --- | --- |
| host | 区別しない | 書かれた綴り（読み込みで小文字にしない） |
| login | 区別しない | config の綴り。keychain のキー（`<host>/<login>`）も config の綴り |
| アカウント ID（`<host>/<login>`） | 区別しない。「自分」の判定（`@me`、`created_by`、コメントの本人確認）、メンバー一覧、メンションの警告のすべて | 書かれた綴り |
| repo ID | 小文字にしてから比べる | 小文字 |
| owner・repo（GitHub にリポジトリを作るとき） | — | URL の綴りのまま |
| Store 名・アカウント名 | 区別する | — |
| ID・接頭辞 | 区別しない（今のまま） | — |

host とアカウント ID の比較は `internal/config` の1つずつの関数（`SameHost`、`SameAccount`）にまとめ、`Host`・`IsGitHubHost`・`hosts[]` の重複検査・トークンを付けるときの比較・`auth login` の重複判定・`Me` を使う比較がそれを使う。

### 空の値

- config の省略できる値（`token`、`default_store`、`hosts[].api`・`web`・`client_id` など）は、前後の空白を除いて空なら、キーがないのと同じに扱う。必須の値（`hosts[].host`、`accounts[].host`・`login`、`url_rewrites[].from`・`to`、`repo_accounts[].match`・`account`）が空なら読み込みのエラー。
- 外から受け取る値（トークン、標準入力の1行、`GET /user` の login）は前後の空白を除き、空ならエラーにする。トークンが空のときは取り方ごとの案内を添える（「トークンの取得」）。
- 「分からない」と「無い」を分ける（`members.json` の `collaborators` は `null` が分からない、`[]` が誰もいない）。
- 出力: 表では値のない欄を `-`、`--json` では値のないキーを省く。「調べていない」は値がないこととは別の状態として出す（例: `auth status` の `state` の `none`）。

### エラーと警告

- 頼まれたことができないときはエラー、できたが気にしてほしいことがあるときは警告。警告は `Session.Warn`（CLI は標準エラーに `warning: …`）か、結果の `Warnings` に入れる。標準エラーへ直接書かない。
- 終了コードは 1: 実行の失敗、2: 使い方の誤り。確認を断ったら終了コード1で「中止しました」。調べた結果に問題があったとき（`auth status`、`doctor`）は終了コード1。
- 人に見せる文言は既存の CLI に合わせて英語にする。この spec の日本語の文面は意味を表したもの。
- フェーズ3で増えるエラーの `ops.Error` の Kind と Code:

| エラー | Kind | Code |
| --- | --- | --- |
| 読み取り専用の Store への書き込み | KindConflict | `store.read_only`（Args: `store`、`reason`） |
| Store をまたぐ参照 | KindInvalid | `ref.cross_store` |
| 2つ以上の Store で当たった ID | KindAmbiguous | `task.ambiguous`（Args に候補の Store 名付き ID）。コメントの ULID なら `comment.ambiguous` |
| `--store` と、別の Store 名を付けた ID を一緒に渡した | KindUsage | `store.conflict` |
| 移動済みのタスク | KindNotFound | `task.moved`（Args: 移動先の Store と ID） |
| トークンが取れない・401 | KindInvalid | `auth.token`・`auth.unauthorized`（Args: アカウント名） |
| GitHub API の 403・404・レート制限 | KindInvalid | `github.forbidden`・`github.not_found`・`github.rate_limit` |
| 平文の http の remote にトークンを送ろうとした | KindInvalid | `auth.insecure`（Args: `url`） |
| 端末がなく選択・確認ができない | KindUsage | `choice.required`（Args: 候補）・`confirm.required` |

### 端末と非端末

「端末」は標準入力が端末のこと。`--json` でも同じ扱い。質問は4種類に分ける。

| 種類 | 例 | 端末 | 端末でないとき |
| --- | --- | --- | --- |
| 確認 | `mv --store`、`purge --moved` | `[y/N]` | `--yes` が必須。なければ `confirm.required`（終了コード2） |
| 選択 | ワークスペース、既定の Store、repo の段の候補、`--scoped`、`discover --from` のアカウント | 番号で選ぶ | 選ぶフラグが必須。なければ候補を並べた `choice.required`（終了コード2）。既定（repo の段の `recent`）があれば既定を使う |
| 推測の確認 | ブランチ名の段、ブラウザを開くか | `[Y/n]` | 飛ばす |
| 取り返しのつかない操作 | `purge --rewrite-history` | Store 名を入力 | 常にエラー（`--yes` でも通さない） |

`internal/ops` は聞かない。答えは入力（`Yes`、選んだ値、入力された Store 名）で受け取り、足りなければ `choice.required`・`confirm.required` を候補付きで返す。CLI は端末ならそれを見て聞き直し、GUI はダイアログを出す。`store discover` は一覧を出すのが目的のコマンドなので、端末でないときは一覧だけを出す（「選択」の例外）。

### 途中で失敗したときの巻き戻し

- ローカルに作ったもの・変えたもの（ディレクトリ、bare リポジトリ、config.yaml、keychain の値）は、失敗したら元に戻す。戻すのはそのコマンドが作った・変えたものだけ。keychain は前の値に戻し、前の値がなければ消す。
- 外に作ったもの（GitHub のリポジトリ、push したコミット）は消さない。残したものと次にすることをエラーに添える。
- 2か所に書く操作は書く順を決め、再実行で仕上がるようにする: `auth login` は keychain → config.yaml、`mv --store` は移動先 → 移動元、履歴の書き換えは「履歴の書き換え」の手順。
- 複数の対象をまとめて扱うコマンド（`discover` で複数を選んだとき、`sync` の全 Store）は、1つずつ独立に行い、失敗したものだけを巻き戻し、残りを続け、最後にまとめて報告する（1つでも失敗したら終了コード1）。

### ロック

- Store のロック: コミットは今の Store のロック（`store.lock`）の中で行う。同期が書く `.hedgehug/local/` の JSON（`access.json`、`members.json`、`rewrites-applied.json`）は、同期と同じ `sync.lock`（`WithLocalLock`）の中で書く。同期の外から書くとき（`store add`・`discover` の clone 直後の判定）も `sync.lock` を取る。
- 2つ以上の Store のロックは、Store の ULID の小さい順に取る。
- `auth.lock`（設定ディレクトリ）: Device Flow のトークンを keychain に読み書きするとき（リフレッシュ、`auth login` の保存、`auth logout` の削除）に取る。Device Flow のポーリングの間は取らず、保存の直前に取る。
- `config.lock`（設定ディレクトリ）: config.yaml を書くときに取り、取った後に読み直して変更を当ててから保存する（`Config.Update(func(*Config) error)`）。CLI と GUI サーバーが同時に書いても変更を失わない。
- `auth.lock` と `config.lock` は、持っている間にほかのロックを取らない。
- `context.json` はロックを取らず、一時ファイルからの rename だけにする（最後に書いたものが残る）。

## パッケージ構成

```
internal/config/
  config.go      hosts、default_store、url_rewrites、repo_accounts、accounts[].token を足す。Update と config.lock
  hosts.go       Host の既定値、IsGitHubHost、SameHost、SameAccount、Rewrite、Unrewrite
internal/auth/   新規
  source.go      gh / env / keychain / device からトークンを取る
  keychain.go    go-keyring の薄い包み（テストでモックに差し替える）
  device.go      Device Flow とリフレッシュ、マシン全体のロック（auth.lock の WithLock）
internal/github/ 新規
  client.go      REST のクライアント、エラーの分類、ページの取得
  repos.go       リポジトリの作成、topic、/user/repos、権限、コラボレーター
  device.go      Device Flow の HTTP の部分
internal/fakegh/ 新規
  server.go      HTTP ハンドラ（GitHub の部分と管理用の /_fakegh/）
  state.go       メモリ上の状態と seed の読み込み
cmd/fakegh/      新規。serve と操作用の CLI
internal/gitrepo/
  conn.go        url_rewrites の適用、HTTPS のトークン認証、SSH、push の拒否の判別
  rewrite.go     新規。履歴からパスを除いたコミット列を作る
internal/mount/  新規
  mount.go       マウント中の Store の集合、開けない Store の扱い
  resolve.go     Store をまたぐ ID の解決、Store 名付きの ID
  query.go       Store ごとの評価と、結果の結合・並べ替え
internal/context/ 新規
  resolve.go     コンテキスト解決の5段階
  repoid.go      remote URL の正規化
  state.go       context.json（hh use と、repo ごとの直近の project）
internal/store/
  move.go        新規。Store 間の移動の両側の書き込み
internal/model/
  forward.go     新規。転送ファイル
internal/merge/  転送ファイルの規則と、parent_moved・ref_moved の修復
internal/sync/
  access.go      新規。読み取り専用の状態（access.json）
  members.go     新規。メンバー一覧（members.json）
  rewrite.go     新規。履歴の書き換えの検出、作り直し、取り込み
  github.go      新規。同期のたびの権限とメンバーの更新（5分おき）
internal/ops/    フェーズ4a でできた操作の層。フェーズ3の操作を足す（auth、store init/add/discover/default、use、mv --store、purge --moved/--rewrite-history）
internal/doctor/ cross_store_ref、duplicate_across_stores、forward の検査
cmd/hedgehug/
  auth.go store_discover.go store_default.go use.go purge_moved.go purge_rewrite.go（端末の入出力と表示だけ）
tools/devenv/    hosts、fakegh.json、url_rewrites、org-b、web、alice-corp
```

依存の向きは `cmd/hedgehug → ops → context → mount → index, store`、`mount → sync`（状態の読み出し）、`sync → auth → github, config, store`（auth は Device Flow の HTTP に github を、ロックに store を使う）、`gitrepo → auth`（トークンは呼び出し側が渡す。`gitrepo` は `auth` を import しない）。`internal/fakegh` はテストと `cmd/fakegh` だけが使う。読み取り専用の状態とメンバー一覧は同期が書くので `internal/sync` に置き、`mount` はそれを読む。

## 設定（internal/config）

config.yaml で新しく読むもの。Save は今と同じく、知らないキーとコメントを残す。

```yaml
accounts:
  - name: personal
    host: github.com
    login: alice
    token: gh              # gh / env:VAR / keychain / device。省略可
hosts:                     # 省略可
  - host: github.com
    api: http://127.0.0.1:7777     # 省略時の既定は下記
    web: http://127.0.0.1:7777
    client_id: Iv23li...           # Device Flow の client ID の上書き
default_store: me
url_rewrites:
  - {from: "https://github.com/", to: "/home/me/hh-dev/remotes/"}
repo_accounts:
  - {match: github.com/org-a/*, account: work}
  - {match: "*", account: personal}
```

- `accounts[].token`: `gh`、`env:<変数名>`、`keychain`、`device` のどれか。`config.TokenSource{Kind, Env}` で持ち、`Kind` は `""`・`gh`・`env`・`keychain`・`device`。省略したアカウントはトークンを使わない（フェーズ2の設定はすべてこれ。ローカルのパスの remote は今までどおり動く）。それ以外の値は読み込みのエラー（ファイルのパスと行を入れる。既存の読み込みのエラーと同じ形）。
- `hosts[]`: `host` は必須で重複（大文字小文字を区別しない）を禁止する。`host` はポートなしのホスト名で書く（remote の URL の host はポートを除いて比べるため、ポート付きで書いても一致しない）。`api` の既定は、`github.com` なら `https://api.github.com`、それ以外は `https://<host>/api/v3`。`web` の既定は `https://<host>`。`client_id` の既定は組み込みの値（github.com のみ。付録の App の client ID。`internal/auth` の変数 `builtinClientID` で、App の登録まで空）。
- `default_store`: `stores[]` の名前。ない名前を指していたら読み込みのエラー。
- `url_rewrites[]`: `from` と `to` は必須。適用は「git の通信」の節。
- `repo_accounts[]`: `match` は repo ID の glob（`path.Match`。`*` は全体にも一致する）、`account` は `accounts[]` の名前。ない名前は読み込みのエラー。フェーズ3は読み込みと検査だけ。
- 同じ host と login（大文字小文字を区別しない）のアカウントを2つ書いたら読み込みのエラー。
- 書くもの（「ロック」の `Config.Update` で書く）: `default_store`（`store init`・`store add`・`store discover` で、まだなければその Store を書く。`hh store default <name>` で変える。`store remove` で指している Store を外したら消す）、`accounts[]` の追加（`hh auth login` で新しいアカウントを足すとき）。

### `hh store default [<name>]`

引数なしは今の既定を出す（なければ `-`）。名前を渡すと書き換える。今と同じ名前なら書かない。ない名前はエラー（登録済みの名前を並べる）。`--json` 対応。

## 認証（internal/auth）

### トークンの取得

`auth.Token(acct config.Account, opt Options) (string, error)`。トークンは通信するときにだけ取る。ローカルで済むコマンドでは gh も keychain も呼ばない。

| token | 取り方 | 失敗したとき |
| --- | --- | --- |
| `gh` | `gh auth token --hostname <host> --user <login>` を実行し、標準出力の前後の空白を除く | gh がない・終了コードが0でない・空: 「gh auth login --hostname <host> を実行するか、token を変えてください」と gh の標準エラー（トークンの形を伏せる）を添える |
| `env:VAR` | 環境変数 VAR の前後の空白を除いた値 | 空: 「VAR が設定されていません」 |
| `keychain` | keychain の `hedgehug` / `<host>/<login>` の値 | ない・空: `hh auth login <name> --with-token` を案内 |
| `device` | 同じ場所の JSON（`access_token`、`refresh_token`、`expires_at`、`refresh_expires_at`）。`expires_at` まで5分を切っていたらリフレッシュする。`expires_at` がない（Device Flow の応答の `expires_in` が 0 かない）トークンは期限なしとしてリフレッシュしない | ない・空・リフレッシュトークンの期限切れ・リフレッシュの失敗（401、`bad_refresh_token`）: `hh auth login <name>` を案内 |

- 外部コマンドの実行（gh）と keychain は、テストから差し替えられるようにする（`Options` に `Run` と `Keyring` を持たせる）。keychain は `Options.Keychain()` から得る（nil なら OS の keychain。この分岐は1か所）。`hh auth` もここから得る。
- keychain は `github.com/zalando/go-keyring` を使う（cgo なし。macOS Keychain、Windows 資格情報マネージャー、Secret Service）。Secret Service がない環境（WSL など）では、go-keyring のエラーに「gh か env:VAR を使ってください」を添える。ファイルへの保存などの代わりの手段は作らない。

### リフレッシュとロック

- `device` のリフレッシュは、`<設定ディレクトリ>/auth.lock` のロック（store のロックと同じ仕組み。`store.WithFileLock`。待ち時間は30秒）を取ってから行う。取った後に keychain を読み直し、まだ期限が近いときだけ `POST {web}/login/oauth/access_token`（`grant_type=refresh_token`）を呼ぶ。CLI と GUI サーバーが同時に更新して、片方のリフレッシュトークンが無効になるのを防ぐ。
- 新しいトークンを keychain に書いてからロックを外す。

### `hh auth`

| コマンド | 内容 |
| --- | --- |
| `hh auth login [<name>] [--host <host>] [--with-token]` | `<name>` が config にあり、`--host` がそのアカウントの host と違えば使い方の誤り（終了コード2）。`<name>` が config にあれば、その token で動く。`device` なら Device Flow、`keychain` なら `--with-token` で標準入力から1行読んで保存する（`--with-token` がなければ、keychain には Device Flow で入れられないので、その旨のエラー）。`gh` と `env` は「Hedgehug でログインするものはありません」と出して終了コード0。token が空なら、Device Flow（`--with-token` なら標準入力のトークンを keychain へ）でトークンを取り、`GET /user` の login が config と合う（大文字小文字を区別しない）のを確かめてから、そのアカウントに `token: device`（または `keychain`）を書く。合わなければ何も書かずにエラー（利用者の決定）。`<name>` がないか config にないときは、Device Flow（`--with-token` なら標準入力のトークン）でトークンを取り、`GET /user` で login を得て、`accounts[]` に `{name, host, login, token: device|keychain}` を足す。名前の既定は login。`--host` の既定は `github.com`。同じ host と login（どちらも大文字小文字を区別しない）のアカウントがすでにあれば足さずに、そのアカウントのトークンを置き換える。書く順は keychain → config.yaml で、config.yaml の保存に失敗したら keychain を前の値に戻す（前の値がなければ消す）。keychain の保存は `auth.lock` の中で行う（Device Flow のポーリングの間は取らない） |
| `hh auth status` | アカウントごとに、名前、`<host>/<login>`、token の種類、`device` の期限（期限なしは `-`）、状態を出す。状態は `ok`、エラーの文（トークンが取れない、`GET /user` の失敗、login が config と合わない）、token のないアカウントは `-`（調べない）。エラーが1つでもあれば終了コード1。`--json` は `{name, id, token, expires, state: "ok"|"error"|"none", login, error}`（値のないキーは省く） |
| `hh auth logout <name>` | keychain の値を消す（`keychain` と `device`。`device` は `auth.lock` の中で）。config のアカウントは残す。`gh` と `env` は「Hedgehug が持っているものはありません」と出して終了コード0 |

Device Flow の流れ: `POST {web}/login/device/code`（`client_id`）→ `user_code` と `verification_uri` を出す（端末ならブラウザを開くかを `[Y/n]` で聞く。開くのは OS の既定の方法で、macOS は `open`、Windows は `rundll32 url.dll,FileProtocolHandler`、それ以外は `xdg-open`。開けなくても URL とコードは出ているので警告だけにする）→ `interval` ごとに `POST {web}/login/oauth/access_token` を呼び、`authorization_pending` は待ち、`slow_down` は間隔を5秒延ばし、`expired_token` と `access_denied` はエラー。client ID が空（App の登録前）なら、「hosts の client_id を設定してください」のエラー。

## GitHub API（internal/github）

REST だけを作る。フェーズ3で使うのは次の表のもの。

| 用途 | API |
| --- | --- |
| login の確認 | `GET /user` |
| Device Flow | `POST {web}/login/device/code`、`POST {web}/login/oauth/access_token` |
| リポジトリの作成 | owner が自分の login なら `POST /user/repos`、それ以外は `POST /orgs/{org}/repos`（どちらも `private: true`、`auto_init: false`）。続けて `PUT /repos/{o}/{r}/topics`（`names: ["hedgehug-store"]`） |
| 発見 | `GET /user/repos?affiliation=owner,collaborator,organization_member&per_page=100`。`Link` の `next` をすべてたどる |
| 権限 | `GET /repos/{o}/{r}` の `permissions.push` |
| コラボレーター | `GET /repos/{o}/{r}/collaborators?per_page=100`（全ページ） |

- クライアントは `github.New(host config.Host, token string, hc *http.Client)`。`Accept: application/vnd.github+json`、`X-GitHub-Api-Version: 2022-11-28`、`Authorization: Bearer <token>` を付ける。
- エラーは型で分ける: `ErrUnauthorized`（401。`hh auth login` を案内）、`ErrForbidden`（403。レート制限でないもの）、`ErrNotFound`（404）、`RateLimitError`（403/429 で `X-RateLimit-Remaining: 0` か `Retry-After`。再開できる時刻を持つ）、`ErrExists`（リポジトリ作成の 422 で `name already exists`）。
- 発見の結果は、`full_name`、`clone_url`、`topics`、`permissions.push`、`private` を使う。
- タイムアウトは1リクエスト30秒。
- 認証と API の失敗は `github.Describe(err, <アカウント ID>, <アカウント名>, <していたこと>)` で `<host>/<login> cannot <していたこと>: <原因>` の形にし、401 なら `(run hedgehug auth login <アカウント名>)` を足す（「エラー」）。

## fakegh

- `internal/fakegh` が HTTP ハンドラ、`cmd/fakegh` が実行ファイル。テストは `internal/fakegh` を httptest で動かす。応答の JSON は、GitHub の実際の応答から Hedgehug が読むキーと、よく使われるキー（`id`、`node_id`、`html_url` など）を持つ形にする。
- 状態（メモリ）: ユーザー（login）、Org（メンバー）、トークン（トークン → login）、repo（owner、name、private、topics、コラボレーターと権限 `read`/`write`/`admin`）、device code（`device_code`、`user_code`、承認したユーザー、期限）、発行したアクセストークン・リフレッシュトークンと期限。
- `fakegh serve [--addr 127.0.0.1:7777] [--seed <json>]`: seed の形は状態をそのまま JSON にしたもの。devenv が書く。Device Flow の `interval` は seed の `device_interval`（秒、既定5）。テストは0にし、`auto_approve` で発行と同時に承認させる。
- 操作用の CLI（起動中のサーバーの `/_fakegh/` を呼ぶ。`--server` の既定は `http://127.0.0.1:7777`）:
  - `fakegh perm set <owner/repo> <login> <read|write|admin|none>`
  - `fakegh repo add <owner/repo> [--topic t]... [--public]`
  - `fakegh device approve <user_code> --user <login>`
  - `fakegh token add <token> --user <login>`
  - `fakegh token expire <login>`（アクセストークンを期限切れにする。リフレッシュを試すため）
- GitHub の部分は前節の表のエンドポイントだけに答える。それ以外は 404。認証の必要なエンドポイントで知らないトークンは 401。
- リポジトリの作成は状態に足すだけで、bare リポジトリは作らない（devenv では `store init` がローカルの分岐に入り、fakegh を呼ばないため）。

## git の通信（internal/gitrepo）

- `url_rewrites` は fetch・push・clone の直前にだけ適用する。`from` が URL の先頭に一致する規則のうち、`from` が最も長いものを使い、その部分を `to` に置き換える。Store の `.git/config`、config.yaml、表示、repo ID はすべて書き換え前の値を使う。書き換えは呼び出し側（`sync`・`store`）が `gitrepo` に関数として渡す。
- 認証（平文の http の扱いは「共通の決まり」の「秘密の送り先」）: 書き換え後の URL が `http://` か `https://` なら、その Store の `account` のトークンを `BasicAuth{Username: "x-access-token", Password: token}` で付ける。ただし付けるのは、書き換え後の URL の host（大文字小文字を区別せず、ポートを除く）がアカウントの host と同じときだけ。別の host のミラーや、書き換えで別のサーバーへ向けた通信には付けない（利用者の決定。トークンを意図しないサーバーに渡さないため）。`account` がない、またはアカウントに `token` がないときは認証なしで試す。SSH（`ssh://` と `user@host:path`）は go-git の SSH で ssh-agent を使い、`known_hosts` を確かめる。ローカルのパスは今と同じ（インプロセスの file transport）。
- push の拒否の判別: HTTP の 403、receive-pack の権限エラー、ローカルのパスで書けない（`EACCES`・`EPERM`）を `ErrPushDenied` にまとめる。読み取り専用の判定に使う。

## Store の作成と発見

### `hh store init <name> --prefix P --remote <url>... [--account a] [--path dir]`

フェーズ2の動き（Store とワークスペースを作り、remotes を記録し、merge driver を登録し、初回の push）に、remote の用意を足す。remote ごとに、書き換え後の URL で分岐する。

- **ローカルのパス**: なければ bare リポジトリを作る（親ディレクトリも）。あれば、空の bare であることを求める（フェーズ2と同じ）。
- **GitHub のホスト**（`github.com` か `hosts[]` の host）: URL から owner と repo を取り（末尾の `.git` を除く）、「GitHub API」の表のとおり private のリポジトリを作って topic を付ける。SSH の形（`git@github.com:o/r.git`、`ssh://`）も host で判定する。`ErrExists`（422）のときは、remote に ref が1つもない（空）かを確かめ、空なら作成を飛ばして topic を付け、初回の push へ進む。中身があればエラーにして `hh store add` を案内する（利用者の決定）。
- **それ以外**: 作らない。既存の空のリポジトリを前提にする（フェーズ2と同じ）。

アカウントは `--account`、なければアカウントが1つならそれ。GitHub のホストの remote があるのにアカウントが決まらなければ、名前を並べたエラー。アカウントが決まれば `stores[].account` に書き、決まらなければ書かない。

422 のときの「remote に ref が1つもないか」は、git の通信（go-git の `Remote.List`。`url_rewrites` と認証を通す）で確かめる（`gitrepo.RemoteEmpty`）。既存の空のリポジトリを使ったときは、自分が作ったものではないので、失敗しても「GitHub に作った」とは言わない。作成が 403 のときは、付録の末尾の案内を添える。

途中で失敗したら、作ったローカルの bare リポジトリと Store のディレクトリを消す。GitHub に作ったリポジトリは消さずに残し、エラーにその URL と `hh store add <name> --remote <url>` を添える。

### `hh store add`

フェーズ2の動きに、`--account a` と、clone の後の読み取り専用の判定（下記）を足す。clone は書き換えと認証を通す。

### `hh store discover [--account a] [--from <dir>] [--all]`

- 候補の集め方:
  - `--from` なし: `--account` があればそのアカウント、なければ `token` を持つ全アカウントで、`/user/repos` を取り、`topics` に `hedgehug-store` を含むものに絞る。候補の URL は `clone_url`、アカウントはそのアカウント。同じ repo が複数のアカウントで見つかったら、`push` 権限のあるアカウントを、どちらも同じならアカウントの順で先のものを使う。
  - `--from <dir>`: dir の下を歩き、bare リポジトリ（`HEAD`・`objects/`・`refs/` を持つディレクトリ）のうち、HEAD のツリーに `.hedgehug/store.yaml` を持つものを候補にする。候補の URL は、`url_rewrites` の `to` が先頭に一致する規則のうち最も長いものを逆にたどって `from` の形に戻す（一致する規則がなければパスのまま）。アカウントは `--account`、なければアカウントが1つならそれ、なければなし。権限は「書けるか」で判定する。
- マウント済みの Store は外す。比べるのは、各 Store の remotes と候補の URL を repo ID の正規化（「コンテキスト解決」）にかけたもの。ローカルのパスは `filepath.Clean` した絶対パスで比べる。
- 提案する名前: repo 名（末尾の `.git` を除く）。使われていれば `<owner>-<repo>`、それも使われていれば `<owner>-<repo>-2`、`-3` を付ける。Store の名前の規則に合わないときは、英小文字にし、使えない文字を `-` にして作る。それでも有効な名前にならない（日本語だけなど）ときは `store-1`、`store-2`…のうち使われていない最初のもの。
- `--from` の bare リポジトリの判定は `HEAD` ファイル・`objects/`・`refs/` の3つがあること。見つけたら、その下へは入らない。
- `--from` でアカウントが決まらない（`--account` がなく、アカウントが2つ以上）ときは、「選択」の質問にする（「端末と非端末」）。
- 端末なら、候補を番号・名前・URL・権限（`read`/`write`）で並べ、「マウントする番号（例: 1,3 / all / 空でやめる）」を聞く。選んだものを `hh store add` と同じ処理で clone する（path は既定）。複数を選んだら1つずつ行い、失敗したものだけを巻き戻して残りを続ける（「途中で失敗したときの巻き戻し」）。名前を変えたいときは `hh store add` を使うよう、一覧の下に書く。
- 端末でないときは一覧を出すだけでマウントしない。`--all` を付けると全候補をマウントする。`--json` は候補の配列（`name`、`url`、`account`、`permission`）と、マウントしたものの名前。
- 候補が0件なら「見つかりませんでした」と出して終了コード0。

### 読み取り専用

- 判定する時点:
  - `store add`・`store discover` で clone した直後
  - 同期で GitHub の情報を取るとき（前回から5分以上たっていたら。親設計の「GitHub の取得は 5 分」）。ローカルのパスの「書けるか」もこの時機に確かめる
  - push が `ErrPushDenied` で拒否されたとき
- 判定のしかた（origin が GitHub のホストかどうかは、`url_rewrites` で書き換える前の URL で見る。メンバー一覧も同じ。利用者の決定。`store init` の分岐だけは書き換えた後の URL で見る）:
  - origin が GitHub のホスト: `permissions.push`。API のエラーなら判定を変えない。
  - origin がローカルのパス: bare リポジトリの `objects/` に一時ファイルを作って消せるか。
  - それ以外: push の拒否だけ。次の明示的な同期（`hh sync` と GUI の「今すぐ同期」）の前に、この理由の判定をいったん外す。
  - 「GitHub のホスト」は `context.Split` が返す host（ポートなし）が `github.com` か `hosts[]` の host（大文字小文字を区別しない）であること。URL の形（https・ssh://・`user@host:path`）では決めない。remote URL を読むのは `context.Split` の1つだけ。
  - origin への push が `ErrPushDenied` のときは、`access.json` に `push_denied` を書いてから `org-a is read-only (the remote denied the push)` のエラーにする。ミラーへの push の拒否は今のミラーの警告にする。
  - 外す規則は「判定の元が書けると示したら外す」: GitHub は `push` が true、ローカルは一時ファイルを作れた、push の拒否は明示的な同期で push が通った。
- 状態は `.hedgehug/local/access.json` に `{"read_only": bool, "reason": "github_permission" | "not_writable" | "push_denied", "checked": 時刻}` で持つ。ないときは書ける扱い。
- 読み取り専用の Store に書き込むコマンドは、Store を開いた時点で拒否する（終了コード1。`store.read_only`）。書き込みの操作は Store を `Session.OpenForWrite` で開き、そこで拒否する。`--store` か Store 名付きの ID（`org-a:API-12`）で Store を指したときは、ID の解決や書き込み前の検査より先に拒否が出る。Store を指さないときは、全 Store から ID を引いて Store が決まった直後（ほかの検査より前）に拒否する（ID を引くまでどの Store か分からないため。利用者の決定。2026-11-01）。入口を通らない書き込みを取りこぼさないよう、`Session.UpdateCommit` と doctor の修正（`fixStore`）の最初にも同じ検査を置く。文面は `org-a は読み取り専用です（GitHub の権限が read です）` のように理由ごとに変える。対象は、タスク・コメント・ワークスペース・スキーマ・undo・restore・purge・mv など、コミットを作るすべてのコマンド。
- バックグラウンドの同期と `hh sync` は、読み取り専用の Store では fetch とマージだけをする。ローカルにコミットがないのでマージは早送りになる。早送りにならないとき（読み取り専用になる前の未 push のコミットがある）は、マージと修復をしてコミットを残し、push はせず、`hh sync` の出力と `sync --status` に「未 push のコミットが N 件ありますが、読み取り専用のため push できません」と出す（`sync.Result.Unpushable`）。HEAD にあって origin にないコミットの数え方は `sync.Unpushed` の1つにし、`ops.Unpushed` はそれを呼ぶ。
- 権限が戻ったと分かったら（API の `push`、または書けるようになった）、自動で外す。

### メンバー一覧

- `.hedgehug/local/members.json` に `{"collaborators": [{"id": "github.com/bob", "permission": "read"}] | null, "authors": ["github.com/alice", ...], "head": "<author を集めた最後の HEAD>", "checked": 時刻}` で持つ。
- `collaborators` は、origin が GitHub のホストのときに、同期で GitHub の情報を取るついでに取る。取れなかったとき（403 に限らず失敗はすべて）は `null` にする（以前の一覧は残さない）。403 は読み取りだけの人に起きるのがふつうなので警告にせず、ほかの失敗は警告にする。`authors` は、HEAD の履歴の `Hedgehug-Author` の集合（同期の後に、前回の HEAD からの差分だけ足す）。
- フェーズ3で使うのは「メンションの権限の警告」だけ。一覧を見るコマンドは作らない（親設計にない）。

### 同期での GitHub の情報の更新

`internal/sync/github.go`。`sync.Run` の最後に、前回の取得（`access.json` の `checked`）から5分以上たっていて、origin が GitHub のホストなら、権限とコラボレーターを取り、`access.json` と `members.json` を書く。失敗は同期の失敗にせず、警告として結果に入れる（`Result.Warnings`。フェーズ2のミラーの警告と同じ）。失敗しても `checked` を進め、次は5分後に試す（判定は変えない）。`access.json`・`members.json` の書き込みは `sync.lock` の中で行う（`store add`・`discover` の clone 直後の判定も）。`sync` は `config` を import しないので、`sync.GitHubRepo` にアカウントの ID と名前を文字列で持たせる。

## マウントと統合ビュー（internal/mount）

### マウント

- `mount.Open(cfg, opt) *Set`: config の `stores[]` の順に Store を開く。index は必要になったときに開く（`(*Mounted).Index()`）。
- 開けない Store（パスがない、`store.yaml` が壊れている）は `Set.Skipped` に理由と一緒に入れ、コマンドは標準エラーに `warning: org-b を開けません: <理由>` と出して続ける。`--store` で名指しした Store が開けないときはエラー。
- 各 `Mounted` は、名前、config の要素、`*store.Store`、Store の ULID、読み取り専用の状態を持つ。
- 開けない Store の警告は `Session.Warn` に出す（GUI にも届く）。
- `ops.Session` がマウント（`mount.Set`）を持つ。一覧の操作は `Set` を回し、ID を受け取る操作は全 Store から引いて、当たった Store について `ForStore` のセッションで書く。`--store` は `Session.StoreName` のまま絞り込みに使う。

### コマンドごとの対象

| 種類 | コマンド | 対象 |
| --- | --- | --- |
| 一覧 | `ls`、`ls --tree`、`inbox`、`log`（ID なし）、`sync`、`sync --status`、`doctor` | 全 Store。`--store` で1つに絞る（`ls` はクエリの `store:` でも絞れる） |
| ID を受け取る | `show`、`set`、`edit`、`mv`、`status`、`done`、`rm`、`restore`、`dep`、`comment`（追加・編集・削除）、`watch`、`unwatch`、`log <id>`、`inbox revert` | 全 Store から ID を引く |
| Store 単位 | `workspace`、`schema`、`undo`、`purge`（`--moved` を含む） | `--store`。なければ Store が1つならそれ、2つ以上ならエラー（名前を並べる） |
| 作成 | `add` | コンテキスト解決 |

`--store` は今と同じく全体フラグとして残す。ID を受け取るコマンド（`hh use` を含む。利用者の決定。2026-11-01）で `--store` を付けたときは、その Store の中だけで引く。別の Store 名を付けた ID（`org-b:API-1`）と一緒なら使い方の誤り（`store.conflict`、終了コード2）。保存済みのビュー（`hh view`）は Store の中に保存されるので「Store 単位」に入る。`inbox` で開けない Store は、ほかの一覧と同じく警告して飛ばす（`--store` で名指ししたときはエラー。利用者の決定。2026-11-01）。

### ID の解決

- 入力は、ID・古い ID・ULID のどれでも、`<Store 名>:` を前に付けた形でも受け付ける（大文字小文字は区別しない。Store 名は区別する）。`:` の前がマウント中の Store の名前でなければ、`:` を含む入力として今までどおり扱う。
- Store 名がなければ、全 Store の index で `Resolve` する。
  - 1つの Store で当たればそれ。
  - 0なら今と同じ「見つかりません」のエラー。転送ファイルにだけ当たったときは「移動の案内」（Store 間の移動の節）。
  - 2つ以上の Store で当たったら、`AmbiguousError` と同じ形で `org-a:API-12`、`org-b:API-12` を並べたエラー（終了コード2）。`KindAmbiguous` は終了コード2（使い方の誤り）にし、1つの Store の中で2つ以上に当たったとき（`task.ambiguous`、`workspace.ambiguous`）も2にそろえる（利用者の決定。2026-11-01）。
- 2つ目の ID（`dep --blocked-by`、`mv --parent`/`--before`/`--after`、`task_ref` のプロパティ、`add --parent`）は、1つ目のタスクの Store（`add` では入れ先の Store）の中だけで引く。その Store で当たらず、別の Store で当たったら、「org-b の WEB-4 は参照できません（参照は Store の中に閉じます）」で拒否する（終了コード1）。`set blocked_by=`、`edit` で書いた参照も、書き込み時の検査で同じ文面にする。

### 表示

- `ls`: マウント中の Store が2つ以上なら、先頭に `STORE` の列を置く。1つなら今のまま。
- 並び:
  - `sort:` がなければ、config の Store の順、その中は今の順。
  - `sort:` があれば、Store をまたいで並べる。値の比べ方は今と同じ。ただし `sort:status` は、カテゴリの順（todo、doing、done、canceled）で比べ、同じカテゴリの中はそれぞれのスキーマでのステータスの位置、それも同じなら Store の順で比べる。`sort:kind` は、親設計の「Store をまたぐ統合ビューは役割で揃える」に従い、役割の順（container、work）で比べ、同じ役割の中はそれぞれのスキーマでの種類の位置、それも同じなら Store の順で比べる。enum・multi_enum のプロパティは、それぞれのスキーマでの値の位置で比べる。同じ値どうしは Store の順、その中は今の順。
  - `group:` は、グループの値（表示名）が同じものを Store をまたいで1つのグループにする。グループの順は、最初に現れた Store での順。
- `ls --tree`: Store が2つ以上なら、Store ごとに見出し（Store 名）を置き、その下に今のツリーを出す。
- `log`（ID なし）: 全 Store のコミットを時刻の新しい順に混ぜる。Store が2つ以上なら、各行の先頭に Store 名を出す。`--limit` は全体の件数（利用者の決定。2026-11-01）。`--json` の各コミットに `store` を足す。
- `show`: `id` と `ulid` の行の後に `store` の行を足す（Store が1つでも出す）。
- `--json`: タスクの出力に `store`（Store 名）を常に足す。`--ulids-only` は変えない。
- Store 名付きの表示: マウント中の2つ以上の Store のワークスペースが同じ接頭辞（大文字小文字を区別しない）を持つ間は、その接頭辞の ID を、人が見る出力（一覧、`show`、確認メッセージ、エラー、inbox、`log` の件名の置き換え）で `org-a:API-12` と出す。`--json` の `id` は変えない（`store` があるので）。

### クエリの `store:`

`store:<名前>`。`,` で OR、先頭の `-` で否定。値は Store 名と完全に一致させる。マウント中でない名前は query のエラー（`unknown store "x"`）。`store:` は、Store ごとの評価に入る前に対象の Store を絞るのに使い、`query.Run` には渡さない。

### Store をまたぐ評価

`mount.Query(set, q, opt)`: 対象の Store ごとに `query.Run` を呼び、結果の Entry に Store を添えて結合し、上の規則で並べ替え・グループ化する。`query.Options.Me` は全 Store で同じ（全アカウント）。ある Store でクエリのエラー（そのスキーマにないプロパティなど）が出たら、全体をエラーにする。ただし「プロパティがスキーマにない」エラーは、その Store を0件として扱い、ほかの Store で1つでも知っていれば続ける（Store ごとにスキーマが違うため）。全 Store で知らなければ今のエラー。`project:`・`parent:`・`blocked_by:` など値に ID を書く項も同じで、その ID が見つからない Store は0件として扱い、どの Store でも見つからなければ今のエラーにする。値を `org-a:API-1` の形で書いたら、その Store の中だけで引く（利用者の決定。2026-11-01）。`status:`・`kind:` と enum・multi_enum のプロパティの値も同じで、その Store のスキーマにない値はその Store では何にも当たらず、どの Store のスキーマにもなければ今のエラーにする（利用者の決定。2026-11-01）。これらは値ごとに当てるので、否定（`-status:review`）はその値を持たない Store の全件に当たる。

### 参照が Store の中に閉じているかの検査

- 書き込み時は「ID の解決」の規則で弾く。
- 同期の後と `doctor` で、参照（`parent`、`blocked_by`、`task_ref`）の先が自分の Store になく、別のマウント中の Store にあるものを見つけたら、データを変えずに警告にする。doctor の検査に `cross_store_ref`（警告）を足す。同期では `Result.Warnings` に入れ、`hh sync` の出力に出す。index の「参照先がない」の問題はそのまま残る（Store の中では壊れた参照なので）。

### コメントの本人確認

`comment edit` と `comment rm` は、コメントの `author` が「自分」（`accounts[]` のいずれか。アカウント ID は大文字小文字を区別せずに比べる）のときだけ許す。author が空、または自分でなければ拒否する（終了コード1。`github.com/bob のコメントは編集できません`。空なら `書いた人が分からないコメントは編集できません`）。

### メンションの権限の警告

本文（`edit`。`add`・`set` には本文の入口がない）とコメント（追加・編集）の書き込みで、書き込み前に無かった `@login` について、その Store の `members.json` の `collaborators` が `null` でなく、その中にいない人なら、書き込みの後に標準エラーへ `warning: bob は org-a を読む権限がありません` と出す。`@github.com/login` の形はそのアカウント ID で、`@login` はコラボレーターの login と比べる（どちらも大文字小文字を区別しない）。`collaborators` が `null` なら警告しない（判断できない）。書き込みは常に行う。

## コンテキスト解決（internal/context）

CLI の `hh add` で入れ先を決める。上から順に試し、最初に当たったものを使う。カレントディレクトリが意味を持たない GUI（4f）はコンテキスト解決を使わず、ops の作成は入れ先（Store・親・ワークスペース）を必ず受け取る。段ごとの質問は「端末と非端末」に従う（3 は推測の確認、4 と 5 は選択）。

1. **明示**: `--store`、`--project`、`--parent`、`--workspace`
   - `--parent <id>` は、`--store` があればその中で、なければ全 Store から引く。`--project <id>` も同じで、当たったタスクの kind が `project` でなければエラー。`--parent` と `--project` の併用はエラー（使い方の誤り、終了コード2）。
   - `--store` だけなら、その Store のワークスペースのルート。ワークスペースは `--workspace`、なければ 5 と同じ選び方。
   - `--workspace` だけなら、全 Store のワークスペースから名前・接頭辞・ULID で引き（`ResolveWorkspace`）、そのルート。2つ以上の Store で当たったら Store 名付きで並べたエラー。
2. **`hh use`**: `context.json` の `use`（Store の ULID と project の ULID）があれば、その project。Store がマウントされていない、project がない・削除済み・転送ファイルになっていれば、`warning: hh use の project が見つかりません（hh use --clear で外せます）` を出してこの段を飛ばす。
3. **ブランチ名**: カレントディレクトリから上にたどって git リポジトリを探し（go-git の `DetectDotGit`）、HEAD がブランチを指していて、名前が `hh/<ID>-…` か `hedgehug/<ID>-…`（`<ID>` は `-` の後の数字まで）なら、その ID を全 Store から引く。当たれば、端末なら `WEB-42 ログイン画面の改修 の子として作りますか? [Y/n]` を聞き、Y ならそれを親にする。n、端末でない、当たらないときはこの段を飛ばす。
4. **repo**: 同じリポジトリの remote を repo ID にする。`origin` があればそれだけ、なければ全 remote を名前順に。その repo ID のどれかを `repos` に持つ、削除されていない project を全 Store から探す。
   - 1件ならそれ。
   - 2件以上なら、`context.json` の `recent[<repo ID>]` の project を既定にして、端末で選ばせる（番号、既定は Enter）。端末でないときは `recent` のものを使い、なければ候補を Store 名付きの ID で並べたエラー。
5. **既定の Store**: `default_store` のワークスペースのルート。`default_store` がなければ、Store が1つならそれ、2つ以上なら端末で選ばせ、端末でなければエラー（`--store` か `hh store default` を案内）。ワークスペースは1つならそれ、2つ以上なら端末で選ばせ、端末でなければエラー（`--workspace` を案内）。

- 決まった入れ先が読み取り専用の Store なら、その時点で拒否する。
- **repo リンクの自動付与**: 4 で repo ID が作れたとき（カレントが git リポジトリの中で、正規化できる remote があるとき）は、どの段で入れ先が決まったかによらず、新しいタスクの `repos` にその repo ID（`origin` のもの、なければ名前順で最初のもの）を足す。`--no-repo` で付けない。
- **直近の記録**: 4 で project を選んだとき（1件でも）、`recent[<repo ID>]` をその project にする。
- **表示**: 作成時に、今の出力の後に1行 `→ org-a / リリース準備 に追加しました` を足す（`<Store 名> / <親のタイトル>`。ルートならワークスペース名）。`--json` には `store` と `parent`（ULID か null）、`workspace` を足す。

### repo ID の正規化

- `https://host/owner/repo(.git)`、`http://…`、`ssh://[user@]host[:port]/owner/repo(.git)`、`user@host:owner/repo(.git)`、`git://…` を受け付ける。
- 結果は `<host>/<owner>/<repo>`。host・owner・repo を小文字にし、末尾の `.git` と `/` を除く。ポートは捨てる。
- それ以外の形（ローカルのパス、`owner/repo` を取れない URL）は repo ID にならない（4 の段は当たらない）。
- remote URL は `url_rewrites` で書き換える前の値を使う（devenv でも `.gitconfig` の `insteadOf` は remote の値を変えない）。

### context.json

`<データディレクトリ>/context.json`（`HEDGEHUG_HOME` があればその下、なければ `XDG_DATA_HOME/hedgehug`）。`{"use": {"store": "<Store の ULID>", "project": "<ULID>"} | null, "recent": {"<repo ID>": {"store": ..., "project": ...}}}`。書き込みは一時ファイルからの rename。壊れていたら警告を出して空として扱う。

### `hh use [<project>] [--clear]`

`<project>` を全 Store から引き（Store 名付きも可）、kind が `project` でなければエラー。`use` に書いて `→ org-a / リリース準備 を既定にしました` と出す。引数なしは今の設定（なければ `-`）、`--clear` は外す。`--json` 対応。

## Store 間の移動

### 転送ファイル（internal/model）

```markdown
---
ulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A
moved_to: 01JC0STORE...      # 移動先の Store の ULID（store.yaml の ulid）
---
```

- タスクと同じパス（`tasks/<a>/<b>/<ULID>.md`）に置く。`model.DecodeTask` は `moved_to` を持つファイルを `*model.Forward` として返す（`ulid` と `moved_to` だけを読み、ほかのキーがあれば壊れたファイル扱い）。
- index は転送ファイルをタスクとして扱わず、`ix.Forwards()` として別に持つ。`Resolve` は転送ファイルの ULID には当たらない。
- 古い ID からの案内のため、index は転送ファイルごとに、そのパスがタスクだった最後の版を git の履歴から1回読み、その `id` と `former_ids` を覚える（キャッシュに残す。転送ファイルの形は変えない。利用者の決定。2026-11-01）。

### `hh mv <id> --store <dest> [--workspace ws] [--parent p] [--scoped carry|drop] [--yes]`

1. **集める**: 指したタスク（移動元 Store）と、その子孫すべて（削除済みを含む）、それらのコメントと作業時間の記録。移動先は `--store` の Store。移動元と同じならエラー（`mv --workspace` を案内）。移動元・移動先のどちらかが読み取り専用ならエラー。
2. **置き場所**: `--parent` があれば移動先の Store の中で引き、その子にする（ワークスペースは親と同じ）。なければ移動先のワークスペースのルート。ワークスペースは `--workspace`、なければ1つならそれ、2つ以上なら端末で選ばせ、端末でなければエラー。rank は移動先の兄弟の末尾。
3. **検査**: 移動先のスキーマ（`scoped` を含む実効の定義）で、各タスクをふだんの書き込みと同じ検査（kind、ステータス、プロパティ、必須項目、親子の組）にかける。違反があれば、何も変えずにタスクごとの違反を並べたエラー（終了コード1）。遷移の検査はしない（Store の移動はステータスの遷移ではない）。
4. **切る参照**: 動かすタスクの集合 M と、移動元に残るタスクのあいだの参照（`blocked_by` と `task_ref` 型のプロパティ。両方向）を集める。`parent` は M の中で閉じている（子孫をすべて動かすので）。M のタスクの `sprint` と `releases` は移動元の Store のものを指すので、移動先では外し、外す参照の一覧と `Hedgehug-Unlinked`（フィールドは `sprint`・`releases`）に入れる。PR・Issue のリンク（`links`）はそのまま（利用者の決定。2026-11-01）。
5. **確認**: 外す参照の一覧と、履歴の警告（下記）を出す。端末なら `移動しますか? [y/N]`。端末でないときは `--yes` がなければエラー（終了コード2）。外す参照がなくても確認は取る。
6. **scoped**: 動かすタスクに project があり、移動元の `schema.yaml` の `scoped` にその `where: project:<ULID>` があれば、持っていくかを `--scoped` で決める。なければ端末で聞き（`[c]arry / [d]rop`）、端末でなければエラー。持っていくときは、移動先の定義で検査し、移動先にないステータス・プロパティ・テンプレートを指す部分を外し、外したものを一覧で出す。移動先の `schema.yaml` に `scoped` の要素として足す。どちらの場合も、移動元の `schema.yaml` からは消す。
7. **移動先に書く**（1コミット、`Hedgehug-Op: move-in`）: M の各タスクを移動先に書く。ID は移動先のワークスペースで振り直し（古い ID は `former_ids` の末尾へ。`former_ids` にすでにある移動元での古い ID も残す）、`workspace` を移動先のものに、M の外を指す参照を外し、`updated` を今にする。コメントと作業時間の記録はそのまま写す。持っていく `scoped` も同じコミット。trailer: `Hedgehug-Task`（移動したタスク）、`Hedgehug-Moved-From: <移動元の Store の ULID>`、外した参照ごとに `Hedgehug-Unlinked: <ULID> <フィールド> <ULID>`。
8. **移動元に書く**（1コミット、`Hedgehug-Op: move-out`）: M の各タスクファイルを転送ファイルに置き換え、M のコメントと作業時間の記録を消し、残るタスクの M を指す参照を外し、`.hedgehug/numbers.yaml` に M の ID の番号（`id` と `former_ids` のうち移動元の接頭辞のもの）の最大を記録し、`scoped` を消す。trailer: `Hedgehug-Task`、`Hedgehug-Moved-To: <移動先の Store の ULID>`、`Hedgehug-Unlinked`。
9. **出力**: `Moved API-12 (ULID) → org-b WEB-45` を M の各タスクについて1行。`--json` は `{from, to, tasks: [{ulid, old_id, new_id}], unlinked: [...], scoped: {carried, dropped}}`。

- 7 の後に 8 が失敗したら、エラーに「移動先には書きました。同じコマンドをもう一度実行すると移動元を仕上げます」と出す。再実行で、移動先に M の ULID がすでに同じ内容で（`move-in` のコミットの後に）あれば 7 を飛ばして 8 だけを行う。
- 7 と 8 は、それぞれの Store のロックの中で行う。2つのロックは Store の ULID の小さい順に取る（「ロック」。逆向きの移動が同時に走っても待ち合わない）。
- `hh undo`: 最後の操作が `move-in`・`move-out` なら undo せずに `hh mv --store` で戻すよう案内し、`rewrite-history` なら取り消せないと出す（`undo.blocked`）。undo の対象の操作が変えたタスクがその後に移動していたら、何も変えずに移動の案内を出す（利用者の決定。2026-11-01）。
- 転送ファイルにだけ当たる ID・ULID の案内: `API-12 は org-b へ移動しました（org-b:WEB-45）`。移動先の Store がマウントされていなければ、`API-12 は別の Store（ULID 01JC0…）へ移動しました。その Store をマウントすると辿れます`。終了コード1。

### 履歴の警告

5 で毎回、`移動前の内容は org-a の git 履歴に残り、org-a を読める人は過去の内容を見られます。消すには hh purge --rewrite-history を使います` を出す。両方の Store の `members.json` の `collaborators` が取れていれば、移動元にいて移動先にいない人を `見えなくなる人: github.com/bob, github.com/carol` として足す。

### `hh purge --moved [--store s] [--yes]`

その Store の転送ファイルを一覧（ULID と、移動先の Store 名か ULID、移動先での今の ID が分かればそれ）で出し、端末なら確認、端末でなければ `--yes` を求め、1コミットで消す（`Hedgehug-Op: purge-moved`）。0件なら何もしない。

### 同期での扱い（internal/merge）

- **移動したタスクに属するファイル**: 片側がタスクを転送ファイルに変え、もう片側がそのタスクに属するファイル（タスクファイル、その下のコメントと作業時間の記録）を変えていたら（追加・変更・削除）、転送ファイルの側を採る。もう片側の変更はファイルごとに負けとし、`Hedgehug-Overwritten: <パス> * <ファイル全体の JSON> <author>` で全文を記録する（タスクファイルもコメントと同じ扱いで、フィールドごとの差分は取らない。削除なら値は `null`）。inbox の上書きの通知に、移動先（`org-b の WEB-45 に反映してください`。移動先がマウントされていなければ Store の ULID）を添え、inbox で全文を見られる。`inbox revert` は、移動済みのタスクには「移動先で直してください」と出して何もしない。
- **転送ファイルと転送ファイル**: 専用の規則は作らない。両側が別の Store へ移していたら（`moved_to` が違う）、ふつうのフィールドのマージ（`moved_to` の LWW と `Hedgehug-Overwritten`）で決まる。負けた移動先には生きたタスクが残るので、「Store をまたぐ ULID の重複」になる（下記）。
- **修復**（`Repair`）:
  - `parent_moved`: 今の `parent_deleted` の条件に「親が転送ファイル」を足し、同じ処理（子をルートへ移す、rank は末尾）で直す。通知の種類は `parent_moved` として分ける（`Hedgehug-Repaired: <ULID> parent_moved {"from": 元の親, "to": null} <author>`）。
  - `ref_moved`: `blocked_by` と `task_ref` が転送ファイルを指していたら外す。`Hedgehug-Repaired: <ULID> ref_moved {"field":..., "target":...} <author>`。
  - どちらも、その変更をした人に inbox で通知する（今の修復の通知と同じ）。

### Store をまたぐ ULID の重複

同じ ULID の生きたタスクが2つ以上のマウント中の Store にあるとき（同時に別の Store へ移した場合など）。`doctor` の検査 `duplicate_across_stores`（警告）と、同期の後の警告で、`01JBZ… が org-b（WEB-45）と org-c（OPS-3）の両方にあります。不要な方を hh rm と hh purge で消してください` と出す。ID の解決で ULID を入力したときは、2つ以上の Store で当たったときの規則（Store 名付きで並べたエラー）になる。

## 履歴の書き換え

### `hh purge --rewrite-history <id> [--store s]`

1. **対象の Store**: `--store`。なければ、ID が当たった Store。移動済みのタスク（移動先で当たる）なら、転送ファイルを持つマウント中の Store。そういう Store が2つ以上なら `--store` を求める。
2. **対象の ULID**: 指したタスクとその子孫（削除済みを含む）。移動済みなら、移動先での今の子孫を使い、その中で対象の Store に転送ファイルかタスクファイルがあるもの。
3. **前提**: 対象の Store を同期し、未 push のコミットと未コミットの変更がないことを確かめる。あれば拒否する。読み取り専用なら拒否する。
4. **確認**: 消すタスクの一覧と、「すべての remote の履歴を書き換えて force push します。他のメンバーのクローンは次の同期で作り直されます」を出し、Store 名を入力させる（端末でなければエラー。`--yes` では通さない）。ops は入力された Store 名を受け取って照らし合わせ、GUI（4f）はダイアログで Store 名を入力させる。
5. **まだ生きているタスク**（転送ファイルでない）: 先に `hh purge` と同じ処理（番号を `numbers.yaml` に記録してファイルを消す）を1コミットで行う。
6. **書き換え**（`gitrepo/rewrite.go`）: root から HEAD まで、各コミットを親の順に作り直す。
   - ツリーから、対象の ULID の `tasks/<a>/<b>/<ULID>.md`、`comments/<a>/<b>/<ULID>/`、`worklogs/<a>/<b>/<ULID>/` を除く。
   - メッセージ: `Hedgehug-Task` が対象の ULID のコミットは、件名のタイトルの部分を `(purged)` にする（`hedgehug: <操作> <ID> (purged)`）。`Hedgehug-Overwritten` で対象のパスを指す行と、`Hedgehug-Repaired` で対象の ULID の行を消す。ほかの行と、author・committer・時刻は保つ。
   - ツリーが親と同じになったコミットも残す（履歴の形を変えない）。
7. **記録**: 書き換えた HEAD の上に、`.hedgehug/rewrites.yaml` に `{at: 時刻, before: <書き換え前の origin の先頭のハッシュ>, ulids: [...]}` を足すコミットを置く（`Hedgehug-Op: rewrite-history`）。ファイルは配列で、前の記録を残す。
8. **push**: すべての remote に force push する。origin が失敗したらエラー（手元は書き換えたまま。もう一度実行すると 7 から再開する。`rewrites.yaml` の最後の `before` が今の origin の先頭なら 7 を飛ばす）。ミラーの失敗は警告。
   - 書き換え前の履歴を持つミラーはマージしない: 同期で、ミラーの先頭が `rewrites.yaml` のいずれかの `before` を含み、手元の HEAD がそれを含まないなら、そのミラーはマージせず、手元の HEAD を force push する。作り直した他のクローンでも同じ（利用者の決定。2026-11-01。消したタスクが古いミラーから戻らないため）。
9. 出力: 消したタスクと、書き換えたコミット数。

### 他のクローンでの検出

同期の fetch で、origin の新しい先頭が、手元で追っている origin（`refs/remotes/origin/main` の fetch 前の値）の子孫でないとき:

- 新しい先頭の `rewrites.yaml` に、まだ取り込んでいない記録（`.hedgehug/local/rewrites-applied.json` にない `at`）があり、その `before` が、手元で追っている origin と同じか、その子孫なら、作り直しに進む。
- そうでなければ、予期しない force push として同期を止める。手元は何も変えない。エラーは `origin の履歴が書き換えられましたが、Hedgehug の記録がありません。hh doctor で確かめてください`。doctor の検査 `unexpected_force_push`（エラー）が同じ状態を出す。

### 作り直しと未 push の変更の取り込み

1. Store のロックを取る。
2. `<path>.rewrite-new` に origin を clone し、remotes と merge driver を設定する。
3. 未 push のコミット（手元の HEAD が、手元で追っている origin より先にあるもの）があれば取り込む。base は手元で追っている origin、ours は手元の HEAD、theirs は新しい origin の先頭。ファイルの集合は3つのツリーの和で、ふだんの同期のマージ（`merge`）にかける。
   - 対象の ULID のパスは戻さない（ours が変えていても theirs の削除を採る）。そのタスクへの未 push の変更があれば、inbox に `消されたタスク WEB-45 への未 push の変更は取り込みませんでした` を出す（内容は書かない）。
   - 結果は、新しいクローンの HEAD の上に通常のコミット1つ（`Hedgehug-Op: replay`、`Hedgehug-Author`、負けた値の `Hedgehug-Overwritten`）として作る。マージ後の修復もふだんどおり行う。
4. `.hedgehug/local/` を、index のキャッシュを除いて新しいクローンへ写す。`rewrites-applied.json` に `at` を足す。
5. ディレクトリを入れ替える: 古い方を `<path>.before-rewrite-<時刻>` に rename し、新しい方を `<path>` に rename する。その後、古い方を消す。
6. push する（ふだんの同期の push）。
7. inbox に `org-a の履歴の書き換えを取り込みました（未 push の変更 N 件を戻しました）` を出す。

- 5 の入れ替えは、1〜4 で持ったロック（Store のロックと `sync.lock`。どちらも `<path>` の中にある）を外し、その Store の git のファイルを閉じてから行う（Windows では開いているファイルを含むディレクトリを rename できないため。利用者の決定。2026-11-01）。入れ替えの直前に、古い方の HEAD が 4 の時点から変わっていないかを確かめ、変わっていたら `.rewrite-new` を消して入れ替えをやめ、次の同期でやり直す（警告）。6 の push は、入れ替えた後の `<path>` でふだんの同期として行う。
- 5 より前に失敗したら `.rewrite-new` を消してエラー。手元は変わらない。
- 5 の途中で失敗した状態（`<path>` がなく、`.before-rewrite-*` と `.rewrite-new` がある、など）は、`doctor --fix`（`rewrite_leftover`）と、次の取り込みを始めるとき（同期の最初）に見つけて入れ替えをやり直す。`<path>` があり `.before-rewrite-*` も残っていれば、古い方を消すところから続ける。Store を開く処理（`store.Open`・`mount.Open`）は書き換えの状態を知らない。開いたときに残りがあれば `warning: org-a の履歴の書き換えの取り込みが途中です（hh doctor --fix で仕上げます）` を出し、`<path>` がなければその Store を「開けない Store」として扱う。
- Windows では、別のプロセス（GUI サーバー）が Store のファイルを開いていると 5 の rename が失敗する。そのときも上の「途中で失敗した状態」として扱う。GUI サーバーの中で取り込むときは、入れ替えの前にその Store の index を閉じる。
- バックグラウンドの同期で見つけたときも同じ処理をする。

## コマンドの一覧（足すもの・変えるもの）

| コマンド | 段 | 内容 |
| --- | --- | --- |
| `hh auth login/status/logout` | 3a | 「認証」 |
| `hh store init --account` | 3a | remote の用意 |
| `hh store add --account` | 3a | 読み取り専用の判定 |
| `hh store discover [--account] [--from] [--all]` | 3a | 「Store の作成と発見」 |
| `hh store default [<name>]` | 3a | 既定の Store |
| ID を受け取るコマンド全部 | 3b | 全 Store からの解決、Store 名付きの ID |
| `hh ls`、`ls --tree`、`show`、`inbox`、`log` | 3b | 全 Store、STORE の列、`store:` |
| `hh add [--project] [--no-repo]` | 3b | コンテキスト解決 |
| `hh use [<project>] [--clear]` | 3b | 「コンテキスト解決」 |
| `hh comment edit/rm` | 3b | 本人確認 |
| `hh mv --store [--scoped] [--yes]` | 3c | 「Store 間の移動」 |
| `hh purge --moved`、`purge --rewrite-history` | 3c | 「Store 間の移動」「履歴の書き換え」 |

足すコマンドはすべて `--json` に対応する（`purge --rewrite-history` の確認は `--json` でも端末の入力を求める）。

## doctor の追加

| 検査 | 重さ | 段 |
| --- | --- | --- |
| `cross_store_ref`: 参照の先が別のマウント中の Store にある | 警告 | 3b |
| `duplicate_across_stores`: 同じ ULID の生きたタスクが2つ以上の Store にある | 警告 | 3c |
| `forward_target`: 転送ファイルの `moved_to` がマウント中のどの Store でもない | 情報（マウントしていないだけのことがあるため） | 3c |
| `unexpected_force_push`: origin が手元で追っている origin の子孫でなく、記録もない | エラー | 3c |
| `rewrite_leftover`: `.rewrite-new`・`.before-rewrite-*` が残っている | 警告（`--fix` で入れ替えをやり直す） | 3c |

検査の結果は重さ（`error`・`warning`・`info`）を持つ。フェーズ2までの検査はすべて `error`。表示は `warning` と `info` の行にだけ重さを添え（`error` は今の表示のまま）、`--json` の各 finding に `severity` を足す。終了コード1は `error` か `warning` があるとき。`info` だけなら0（利用者の決定。2026-11-01）。

`doctor` は全 Store を見る（「コマンドごとの対象」）。Store をまたぐ検査は、マウント中の Store の index を使う。マウント中の Store が2つ以上なら（`--store` で1つに絞っても。`ls` と同じ。利用者の決定。2026-11-01）、Store ごとに見出し（Store 名）を置き、その下にその Store の修正と検査結果を出す。`--json` は Store ごとの配列 `[{store, repaired, findings}]`（Store が1つでも配列）。`--fix` も全 Store を直し、読み取り専用の Store は直さずに警告を出す（利用者の決定。2026-11-01）。

## エラー

- 終了コードは今の決まり（1: 実行の失敗、2: 使い方の誤り）に合わせる。確認の拒否（`N`）は終了コード1で「中止しました」。
- Kind と Code は「共通の決まり」の「エラーと警告」の表のとおり。
- 認証と GitHub API のエラーは、どのアカウントで何をしようとしたかを添える（例: `github.com/alice-corp で org-a/hh-shared の権限を取れません: 401（hh auth login work を実行してください）`）。
- 同期で GitHub の情報を取れないことは、同期の失敗にしない（警告）。

## 開発環境

### devenv

`go run ./tools/devenv init` が作るものを次のようにする（3a で hosts・fakegh・`url_rewrites`・org-b、3b で web、3c で変更なし）。

```
~/hh-dev/
  fakegh.json                      # fakegh の seed
  remotes/kukv/hh-tasks.git        # alice の個人 Store
  remotes/org-a/hh-shared.git      # org-a の Store
  remotes/org-b/hh-ops.git         # org-b の Store（3a で足す）
  remotes/org-a/web.git            # コードのリポジトリ（3b で足す）
  users/alice/                     # personal（alice）と work（alice-corp）の2アカウント
  users/bob/                       # bob。org-b は read だけ
```

- 各ユーザーの config.yaml: `accounts`（`token: env:HH_DEV_TOKEN`。alice の2つ目のアカウント alice-corp は `env:HH_DEV_TOKEN_WORK`。1つの変数では2つのアカウントのトークンを分けられないため。`devenv env alice` が両方を export する）、`hosts`（github.com の `api` と `web` を `http://127.0.0.1:7777`）、`url_rewrites`（`https://github.com/` → `~/hh-dev/remotes/`）、`stores`（remote は `https://github.com/...` の形で書き、書き換えで remotes に向ける）、`default_store`。
- 各ユーザーの `.gitconfig` に、同じ書き換えの `insteadOf` を書く。
- alice は Store ごとにアカウントを分ける（個人 Store は alice、org-a と org-b は alice-corp）。
- bob の org-b の bare リポジトリは書き込めないようにはしない（alice と共有するため）。bob の読み取り専用は fakegh の権限（`read`）で再現し、`fakegh perm set` で切り替えて試せる。
- `fakegh.json`: ユーザー alice・alice-corp・bob、Org org-a（alice-corp・bob）・org-b（alice-corp・bob）、repo（`kukv/hh-tasks`、`org-a/hh-shared`、`org-b/hh-ops` に topic `hedgehug-store`、`org-a/web` は topic なし）、権限、トークン（`HH_DEV_TOKEN` に入れる値をユーザーごとに）。
- 3b: org-a の Store に、`repos: [github.com/org-a/web]` を持つ project を1つ作り、alice の `users/alice/src/web` に web を clone しておく。
- fakegh の URL は `devenv init --fakegh <url>`（既定 `http://127.0.0.1:7777`）で変えられる（E2E が httptest の URL を渡すため）。
- `init` の最後に、fakegh の起動（`go run ./cmd/fakegh serve --seed ~/hh-dev/fakegh.json`）と、ユーザーの切り替え方を出す。

### git を外した CI ジョブ

今のまま通す。fakegh は Go の HTTP サーバーなので git を要らない。gh の取得元のテストは、実行部分を差し替えるので gh を要らない。

## テスト

- **単体**
  - `internal/config`: 新しいキーの読み込み・検査・書き込み（知らないキーとコメントを残す）、`default_store` の自動の書き込みと remove での削除。
  - `internal/auth`: 4種類の取得（gh は実行部分を差し替え、keychain は go-keyring のモック）、Device Flow（`authorization_pending`、`slow_down`、`expired_token`、`access_denied`）とリフレッシュを fakegh で、ロックの競合（2つの goroutine が同時にリフレッシュしても `access_token` の呼び出しが1回）。
  - `internal/github`: fakegh を相手に各 API、ページの取得、エラーの分類（401、403、404、422、レート制限）。
  - `internal/fakegh`: 管理用エンドポイントと CLI。
  - `internal/gitrepo`: `url_rewrites`（最長一致、HTTP とローカル）、トークン認証の付与（go-git に smart HTTP のサーバーがなく git 本体なしでは立てられないので、テスト用に `https` の transport を差し替え、渡された `AuthMethod` を記録して確かめる。差し替えは `internal/gitrepo/gitrepotest`）、`ErrPushDenied` の判別、履歴の書き換え（ツリー、メッセージ、trailer、author と時刻の保持）。
  - `internal/mount`: 開けない Store、ID の解決（1つ・0・2つ以上・Store 名付き・転送ファイル）、`store:`、並びとグループ化、Store 名付きの表示。
  - `internal/context`: repo ID の正規化（表の各形）、5段階のそれぞれと飛ばす条件、`recent`、`context.json` の破損。
  - `internal/merge`: 転送ファイルの規則（タスク、転送ファイル、削除、コメント）と `parent_moved`・`ref_moved`。
  - `internal/sync`: GitHub の情報の更新（5分、失敗は警告）、読み取り専用での同期、履歴の書き換えの検出・作り直し・取り込み・途中で失敗した状態からの再開。
- **プロパティテスト**: フェーズ2のマージのプロパティテストに、`mv --store`（2つの Store を持つ仮想クローン）と、同時の編集・子の追加・コメント・参照を足す。全クローンが収束すること、負けた値が必ず記録されること（転送ファイルに負けた変更が `Hedgehug-Overwritten` にあること）、不変条件（転送ファイルを親に持つ生きたタスクがない、転送ファイルを指す参照がない）が守られることを確かめる。
- **CLI**: 足す・変えるコマンドの出力をゴールデンテストで固定する（STORE の列、Store 名付きの ID、`show` の `store` の行、コンテキスト解決の1行、`mv --store` の確認と出力、discover の一覧）。端末の入力は今のテストの方法で与える。
- **E2E**（CLI のバイナリ、fakegh を httptest で起動）
  - 3a: `auth login`（Device Flow を fakegh で承認）→ `auth status`、`store init` のローカル分岐と GitHub の分岐（書き換えなしの構成で fakegh にリポジトリができる）、`discover` と `discover --from`、bob の読み取り専用（書き込みの拒否、同期は fetch だけ、権限を戻すと外れる）。
    - `auth login` は、バイナリでなくこのプロセスの中で `run` を呼び、keychain は go-keyring のモックにする（バイナリは本物の OS の keychain に書き、keychain のない CI では通らないため）。
    - `store init` の GitHub 分岐は、`hosts` に `127.0.0.1` を GitHub のホストとして登録し、remote を何も待ち受けていない `http://127.0.0.1:1/...` にして、作成・topic・残したリポジトリの案内までを確かめる（本物の `github.com` へ push させない）。422 の「空」「中身あり」は、fakegh の `State.Repos` と `gitrepotest` の transport で同じ owner/name の bare に向けて作る。
  - 3b: 3つの Store での `ls`（STORE の列、`store:`、`@me`）、ID の解決と Store 名付きの ID、Store をまたぐ参照の拒否、web の中での `add`（repo の段、repo リンクの付与、表示の1行）、`hh use`、ブランチ名の段、コメントの本人確認、メンションの警告。
  - 3c（完了条件）: alice が個人 Store（alice）と org-a・org-b（alice-corp）を同時に扱う: 各 Store への add と sync、全 Store の `ls`、`mv --store`（参照の切断、転送ファイル、bob の側で同時の編集 → 同期で記録と通知）、`purge --moved`、`purge --rewrite-history` → bob の未 push の変更が作り直したクローンに戻り、消したタスクは戻らない。

## 前のフェーズからの持ち越しの扱い

| 持ち越し | 出どころ | 扱い |
| --- | --- | --- |
| Store の選び方のつなぎ（`--store` か1つだけ） | 1d-1 | 「コマンドごとの対象」と「コンテキスト解決」で置き換える |
| `store:`、Store をまたぐクエリ | 1c | 「クエリの `store:`」「Store をまたぐ評価」 |
| コメントの編集・削除の本人確認 | 1d-2 | 「コメントの本人確認」（利用者の決定） |
| `moved_to` の転送ファイル | 1a | 「転送ファイル」 |
| `url_rewrites`、`store init` の bare の作成、`store discover` | 2 | 「git の通信」「Store の作成と発見」 |
| トークンを使う認証 | 2 | 「認証」 |
| Store 間の移動での振り直し、履歴の書き換えの検出 | 2 | 「Store 間の移動」「履歴の書き換え」 |
| 接頭辞が重複した Store の `org-a:API-12` の表示 | 1.5・2 | 「表示」 |
| メンション先に Store を読む権限がないときの警告 | 2 | 「メンションの権限の警告」 |
| devenv の `remotes/org-a/web.git` | 2 | 「devenv」 |

## 親設計ドキュメントへの反映

spec の承認後に、次を親設計に書き足す。

- 「ローカル設定」: `accounts[].token` の省略、`hosts`（`api`・`web`・`client_id`）、`default_store` と `hh store default`。
- 「認証」: keychain の保存場所（`hedgehug` / `<host>/<login>`）、`hh auth login` の `--with-token` とアカウントの追加、`device` のリフレッシュの時機（5分前）。
- 「Store の作成と発見」: GitHub に作ったリポジトリは失敗しても消さないこと、discover の候補の集め方（`/user/repos`）と名前の決め方、端末でないときの動き。
- 「読み取り専用のマウント」: 判定の時点と方法、`access.json`、読み取り専用での同期。
- 「メンバー」: `members.json`、コラボレーターが取れないときの扱い。
- 「Store 間の移動（昇格）」: 2つのコミットと再実行、スキーマに合わないタスクの拒否（利用者の決定）、`Hedgehug-Moved-From`・`Hedgehug-Moved-To`・`Hedgehug-Unlinked`、番号を `numbers.yaml` に記録すること、同時の変更との競合（利用者の決定）、`parent_moved`・`ref_moved`、Store をまたぐ ULID の重複。
- 「Store 間の移動（昇格）」の履歴の書き換え: `rewrites.yaml`、予期しない force push、作り直しと未 push の変更の自動の取り込み（利用者の決定）。
- 「コンテキスト解決」: `--project` の意味、ブランチ名の段で端末でないときは飛ばすこと、`recent`、`default_store`、`context.json`。
- 「メンションと通知」: コメントの本人確認（author が空なら誰もできない。利用者の決定）。inbox に届くものに「移動済みのタスクへの変更」と「履歴の書き換えの取り込み」を足す。
- 「名前と識別子」の trailer の表に `Hedgehug-Moved-From`、`Hedgehug-Moved-To`、`Hedgehug-Unlinked` を足す。
- 「開発環境」の「作る時期」: fakegh のうちフェーズ3で使う範囲（ユーザー、Org、repo、topic、権限、コラボレーター、Device Flow）をフェーズ3に、PR・Issue などをフェーズ5に。
- 「実装フェーズ」3: 3a・3b・3c に分けて作ること。
- 「共通の決まり」の節の内容（秘密の送り先、大文字小文字、空の値、エラーと警告、端末と非端末、途中失敗の巻き戻し、ロック）を、親設計の該当する章に書く。
- 「アーキテクチャと技術選定」のパッケージ構成に `internal/mount`、`internal/fakegh`、`cmd/fakegh` を足す。技術選定の資格情報の行に `zalando/go-keyring` を書く。

## 付録: GitHub App の登録手順

Hedgehug 専用の GitHub App を、kukv のアカウント（または配布用の Org）で登録する。

1. GitHub の Settings → Developer settings → GitHub Apps → New GitHub App。
2. 入力:
   - GitHub App name: `Hedgehug`（使われていれば `Hedgehug CLI` など）
   - Homepage URL: `https://github.com/kukv/hedgehug`
   - Callback URL: 空（Device Flow だけを使う）
   - 「Expire user authorization tokens」: オン（アクセストークン 8 時間、リフレッシュトークン 6 か月）
   - 「Request user authorization (OAuth) during installation」: オフ
   - 「Enable Device Flow」: オン
   - Webhook: 「Active」をオフ
3. Permissions（Repository permissions）:
   - Contents: Read and write
   - Issues: Read and write
   - Pull requests: Read and write
   - Metadata: Read-only（自動で付く）
   - Administration: Read and write（`hh store init` でリポジトリを作るため。Org の設定で App にリポジトリ作成を許す場合に使われる）
   - Account permissions と Organization permissions: なし
4. 「Where can this GitHub App be installed?」: Any account。
5. 作成後、App の設定ページの「Client ID」（`Iv23li` で始まる文字列）を控える。クライアントシークレットと秘密鍵は作らない（Device Flow には要らない）。
6. Client ID を `internal/auth` の組み込みの値に入れる（client ID は公開してよい値）。
7. Org で使うときは、その Org に App をインストールし、Store のリポジトリへのアクセスを許す。インストールできない場合は `token: gh` を使う。

Administration の権限は、`store init` で GitHub にリポジトリを作るときに必要になる。App のユーザートークンは「App とユーザーの両方が持つ権限」の範囲でしか使えないため、ユーザーが Org でリポジトリを作れない場合は作成が 403 になる。そのときはエラーに「GitHub で空のリポジトリを作り、hh store init --remote でその URL を指定してください（作成は飛ばします）」を添える。既存の空のリポジトリが GitHub にあるとき（作成が 422）は、`store init` は作成を飛ばしてそのリポジトリを使う（上記）。
