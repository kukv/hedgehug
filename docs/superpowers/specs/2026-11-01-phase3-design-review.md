# フェーズ 3 設計の見直し

- 日付: 2026-11-01
- 対象: `docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md`（以下 spec）と `docs/superpowers/plans/2026-11-01-phase3a-auth-github-discover.md`（以下 plan）
- 目的: 3a の実装中に質問が続いたため、実装を止めて設計を見直す。この文書に、決めてほしいことを通し番号（**決1**〜）でまとめた。各項目に推奨を付けてあるので、推奨と違うものだけ返してもらえれば、残りは推奨で確定する。
- 結果: 2026-11-01、利用者が決1〜決20 をすべて推奨のとおり承認した。
- 確認の後にすること: spec と plan を直し、ブランチを main へ rebase し、Task 8 の修正から再開する（「6. 再開の手順」）。この文書を出すまで、コードには触っていない。

## 0. いまの状態

- ブランチ `phase3-stores-accounts`（未 push）は残っている。作業していた worktree（drifting-rolling-puddle）は消えていたので、この worktree（stateful-hugging-barto）に checkout した。
- gitignore していた SDD の台帳（Task ごとの brief・report・preflight）は worktree と一緒に失われた。Task 8 のレビューで直すことはメモに残っているので再開に支障はない。再開のときに台帳を作り直す。
- 3a は Task 1〜7 が完了・レビュー済み、Task 8（`hh auth`）は実装済みで要修正、Task 9〜14 は未着手。
- **main が大きく動いた**。フェーズ4a（PR #20）で、CLI の処理の大半が `cmd/hedgehug` から `internal/ops` に移った（`Session`、`ops.Error` の Kind/Code、`StoreInit`・`StoreAdd`・`Sync` など）。フェーズ4の spec は「4a を先に main に入れ、フェーズ3のブランチを rebase してもらう」と決めている。plan の Task 9〜14 は移る前の `cmd/hedgehug` を前提に書いてあるので、そのままでは使えない（「2.7」と「3.1」）。

## 1. 実装中の決定を spec 本文に確定する（手順1）

### 1.1 spec に書く利用者の決定（決定済み・未記入）

1. login の比較は大文字小文字を区別しない（`auth login` の照合と、config の同じ host・login の重複判定の両方）。keychain のキーは config の綴りのまま。
2. `hh auth login <name> --host <別の host>` は使い方の誤り（終了コード2）。
3. gh・keychain・device・env が空のトークン（前後の空白を除いて空）を返したらエラーにし、取り方ごとの案内を添える。env の値も前後の空白を除く。

### 1.2 plan の「spec にない細部の判断」1〜29 を spec に移す

承認済みの 1〜29 は plan にだけ書いてある。3b・3c の plan からも参照できるよう、spec の該当する節に移す（plan には「spec の○○を見る」とだけ残す）。移し先は次のとおり。

| 判断 | 移し先の節 |
| --- | --- |
| 1, 2, 22 | 設定 |
| 3, 4, 12, 13, 26, 28 | 認証 |
| 23 | エラー |
| 18, 24, 29 | テスト |
| 5, 6, 7, 9, 10, 11, 21, 27 | 読み取り専用・メンバー一覧・同期での GitHub の情報の更新 |
| 8 | 読み取り専用（入口は 4a の後の場所に直す。決11） |
| 14, 15, 25 | Store の作成と発見 |
| 16, 17 | 開発環境 |
| 19 | fakegh |
| 20 | パッケージ構成と repo ID の正規化 |

判断5（access.json・members.json の読み書きを `internal/sync` に置く）に合わせて、spec のパッケージ構成の `internal/mount/access.go`・`members.go` を消す。

### 1.3 確認が済んでいないもの

- **決1**: Task 5 で、Device Flow の応答の `expires_in` が 0（または無い）ときは「期限なし」とした（報告済み・未確認）。GitHub App で「Expire user authorization tokens」をオフにした場合の応答がこの形になる。
  - 推奨: このまま「期限なし」とし、spec の「認証」に書く。`auth status` の EXPIRES は `-`。リフレッシュはしない。

## 2. 共通の決まり（手順2）

実装中の質問の多くは、同じ種類の問いを Task ごとに決め直していたことから来ていた。次の7つを spec に「共通の決まり」の節として足し、各節はそこを参照する形にする。

### 2.1 秘密の送り先

- **決2**: トークンを送ってよい先を、次の1つの規則にまとめる。
  - git の通信: 書き換え後の URL の host（ポートを除く）がアカウントの host と同じとき（決定済み）。
  - GitHub API・Device Flow: そのアカウントの host の `hosts[]` の `api`・`web`（既定値を含む）と、オリジン（スキーム・host・ポート）が同じ URL だけ。`Link` の `next` などで別のオリジンを指されたら、トークンを付けずにエラーにする（Task 4 で別 host は断っている。比べる単位を host からオリジンに揃える）。
  - トークンを、エラー文・警告・ログ・`--json`・コマンドラインの引数に出さない。標準入力（`--with-token`）、環境変数、keychain、gh の標準出力からだけ受け取る。gh の標準エラーをエラーに添えるときは、トークンの形（`gh[opsu]_`、`github_pat_`）を伏せる。
  - 推奨: この形で確定する。

### 2.2 名前・ID・login の大文字小文字

いまは比べ方が場所ごとにばらばらになっている。実例: `config.Host`・`IsGitHubHost`・`hosts[]` の重複検査は host を区別して比べるが、`context.Split` は host を小文字にし、トークンを付けるときの比較（`sameHost`）は区別しない。そのため `hosts: [{host: GHE.example.com}]` と書くと、その host の remote が GitHub のホストと判定されない。

- **決3**: 次の表に揃える。

| もの | 比べ方 | 保存・表示 |
| --- | --- | --- |
| host | 区別しない（DNS と同じ） | 書かれた綴り。読み込み時に小文字にはしない |
| login | 区別しない（決定済み） | config の綴り |
| アカウント ID（`<host>/<login>`） | 区別しない。「自分」の判定（`@me`、`created_by`、コメントの本人確認）、メンバー一覧、メンションの警告のすべて | 書かれた綴り |
| repo ID | 小文字にしてから比べる（spec のまま） | 小文字 |
| owner・repo（GitHub にリポジトリを作るとき） | — | URL の綴りのまま（判断20） |
| Store 名・アカウント名 | 区別する（config のキー） | — |
| ID・接頭辞 | 区別しない（今のまま） | — |

  - 推奨: この表で確定する。host の比較を `config` の1つの関数にまとめ、`Host`・`IsGitHubHost`・重複検査・`sameHost`・`auth login` の重複判定（いまは `x.Host == host`）が使う。

### 2.3 空の値

- **決4**: 次に揃える。
  - config の省略できる値（`token`、`default_store`、`hosts[].api` など）は、前後の空白を除いて空なら、キーがないのと同じに扱う。必須の値（`host`、`login`、`from`、`to`、`match`、`account`）が空なら読み込みのエラー。
  - 外から受け取る値（トークン、標準入力の1行、`GET /user` の login）は前後の空白を除き、空ならエラー（1.1 の 3 を一般化）。
  - 「分からない」と「無い」を分ける: `members.json` の `collaborators` は `null` が分からない、`[]` が誰もいない（spec のまま）。
  - 出力: 表では値のない欄を `-`、`--json` では値のないキーを省く（今の `omitempty` の慣習）。「調べていない」は、値がないこととは別の状態として出す（次の `auth status` の例）。
  - 推奨: この形で確定する。Task 8 の Minor「token が空のアカウントの `auth status` の JSON」は、いま `"ok": true` を出していて調べていないのに通ったように見えるので、`ok` をやめて表の STATE 列と同じ `"state": "ok" | "error" | "none"` にする（`none` は token のないアカウント）。

### 2.4 エラーと警告

- **決5**: 次に揃える。
  - 頼まれたことができないときはエラー、頼まれたことはできたが気にしてほしいことがあるときは警告にする。警告は `Session.Warn`（CLI は標準エラーに `warning: …`）か結果の `Warnings` に入れ、GUI にも届くようにする。標準エラーへ直接書かない。
  - 終了コードは今の決まり（1: 実行の失敗、2: 使い方の誤り）。確認を断ったら終了コード1で「中止しました」。調べた結果に問題があったとき（`auth status`、`doctor`）は終了コード1。
  - 人に見せる文言は英語（plan の Global Constraints 24 を spec に書く）。spec の日本語の文面は意味を表したもの。
  - フェーズ3で増えるエラーに、`ops.Error` の Kind と Code を割り当てる。GUI（4f）はこれで文面と HTTP の状態を選ぶ。

| エラー | Kind | Code |
| --- | --- | --- |
| 読み取り専用の Store への書き込み | KindConflict | `store.read_only`（Args: `store`、`reason`） |
| Store をまたぐ参照 | KindInvalid | `ref.cross_store` |
| 2つ以上の Store で当たった ID | KindAmbiguous | 今の `task.ambiguous` に、候補の Store 名付き ID を Args で足す |
| 移動済みのタスク | KindNotFound | `task.moved`（Args: 移動先の Store と ID） |
| トークンが取れない・401 | KindInvalid | `auth.token`・`auth.unauthorized`（Args: アカウント名） |
| GitHub API の 403・404・レート制限 | KindInvalid | `github.forbidden`・`github.not_found`・`github.rate_limit` |
| 端末がなく選択・確認ができない | KindUsage | `choice.required`（Args: 候補）・`confirm.required` |

  - 推奨: この表で確定する。

### 2.5 端末と非端末

spec は、端末でないときの動きを質問ごとに別々に決めている（ブランチ名の段は飛ばす、repo の段は直近を使う、既定の Store はエラー、discover は一覧だけ、mv は `--yes` を求める、など）。GUI（4f）では端末の入力がそもそもない。

- **決6**: 質問を4種類に分け、種類ごとに決める。

| 種類 | 例 | 端末 | 端末でないとき |
| --- | --- | --- | --- |
| 確認 | `mv --store`、`purge --moved` | `[y/N]` | `--yes` が必須。なければ終了コード2 |
| 選択 | ワークスペース、既定の Store、repo の段の候補、`--scoped` | 番号で選ぶ | 選ぶフラグが必須。なければ候補を並べて終了コード2。ただし「直近」など既定があれば既定を使う（repo の段の `recent`） |
| 推測の確認 | ブランチ名の段、ブラウザを開くか | `[Y/n]` | 飛ばす（推測は使わない） |
| 取り返しのつかない操作 | `purge --rewrite-history` | Store 名を入力 | 常にエラー（`--yes` でも通さない） |

  - 「端末」は標準入力が端末のこと。`--json` でも同じ扱い。
  - `internal/ops` は聞かない。必要な答えは入力（`Yes bool`、選んだ値、入力された Store 名）で受け取り、足りなければ `choice.required`・`confirm.required` のエラーを候補付きで返す。CLI がそれを見て端末で聞き直し、GUI はダイアログを出す。
  - 推奨: この形で確定する。spec の各節の端末の記述はこの表を参照する形に直す。discover の「端末でないときは一覧だけ」は「選択」の例外として残す（一覧を出すことが目的のコマンドなので）。

### 2.6 途中で失敗したときの巻き戻し

- **決7**: 次に揃える。
  - ローカルに作ったもの（ディレクトリ、bare リポジトリ、config.yaml の変更、keychain の値）は、失敗したら元に戻す。元に戻すのは、そのコマンドが作った・変えたものだけ。
  - 外に作ったもの（GitHub のリポジトリ、push したコミット）は消さない。残したものと次にすることをエラーに添える（spec の `store init` のまま）。
  - 2か所に書く操作は、書く順と、再実行で仕上がることを決める: `auth login` は keychain → config.yaml、`mv --store` は移動先 → 移動元（spec のまま）、履歴の書き換えは「履歴の書き換え」の手順。
  - 複数の対象をまとめて扱うコマンド（`discover` で複数を選んだとき、`sync` の全 Store）は、1つずつ独立に行い、失敗したものだけを巻き戻し、残りは続けて、最後にまとめて報告する（1つでも失敗したら終了コード1）。
  - 推奨: この形で確定する。Task 8 の Minor「keychain に書けたが config の Save が失敗した」は、この規則で、keychain を前の値に戻す（前の値がなければ消す）。

### 2.7 ロック

- **決8**: 次に揃える。
  - Store のロック: コミットと `.hedgehug/local/` の JSON（`access.json`、`members.json`、`rewrites-applied.json`）の書き込みは、Store のロックの中で行う。plan の Task 11 は `store add` の後の読み取り専用の判定をロックの外で書いているので、ロックの中に入れる。
  - 2つ以上の Store のロックは、Store の ULID の小さい順に取る。spec の `mv --store` は「移動先 → 移動元」の順で取ると書いてあり、A→B と B→A の移動が同時に走ると互いに待ち合う。順番を ULID で決めればこれが起きない。
  - `auth.lock`: Device Flow のトークンを keychain に読み書きするときに取る。リフレッシュ（今のまま）に加え、`auth login` の保存と `auth logout` の削除でも取る。Device Flow のポーリング（数分かかる）の間は取らず、保存の直前に取る。Task 8 の Minor「login の Device Flow 中に auth.lock を取らない」はこの形で直す。
  - config.yaml: CLI と GUI サーバーが同時に書きうる（`auth login`、`store add`、`store default` など）。設定ディレクトリの `config.lock` を取り、取った後に読み直して変更を当ててから保存する。
  - `auth.lock` と `config.lock` は、持っている間にほかのロックを取らない。
  - `context.json` はロックを取らず、一時ファイルからの rename だけにする（最後に書いたものが残る。中身は「直近」と `hh use` だけなので、食い違っても害がない）。
  - 推奨: この形で確定する。config.lock は今のフェーズ2の書き込み（`store add/remove`）にも効くが、フェーズ3で書く場所が増え、GUI も書くので、ここで入れる。

## 3. 3a の残り・3b・3c の読み直し（手順3）

2 の決まりに照らして読み直し、決まらず挙動が変わるものだけを挙げる。2 の決まりで答えが出たものは載せていない。

### 3.1 フェーズ4a に合わせて変わること（rebase が強いる書き換え）

ブランチと main の衝突は小さい（ブランチが触った既存のファイルは `app.go` 6行、`main.go` 26行、`store_add.go` 4行、`store_init.go` 2行、`internal/sync/sync.go` 2行）。大きいのは plan の書き先の変更。

| plan の前提 | 4a の後 |
| --- | --- |
| 書き込みの拒否は `app.update` と `doctor --fix` と `purge` の入口（判断8） | 4a の後は、doctor の修正（`ops/maintain.go` の `fixStore` → `doctor.Fix`）を除くすべての書き込み（purge・undo・restore --at を含む）が `ops.Session.UpdateCommit` を通る。拒否は `UpdateCommit` の最初と `fixStore` の最初の2か所に置く。順番は決11 |
| `a.storeAdd(name, remotes, path, account)` | `ops.Session.StoreAdd` に account を足す。init は `StoreInitInput` に Account と Path を足す。discover も ops に置く |
| `a.syncOptions` | `Session.SyncOptions` に Conn と GitHub を入れる |
| `sync.Unpushed`（判断28） | main に `ops.Unpushed` がすでにある。数える処理は `sync.Unpushed` の1つにし、`ops.Unpushed` はそれを呼ぶ |
| `a.config()`・`a.tokenFor`・`a.authOptions`・`a.githubFor`（Task 7・8・9） | `ops.Session` のメソッドにする（GUI の 4f も使うため）。`hh auth` の入出力（標準入力のトークン、Device Flow のコードの表示）だけを CLI に残す |

- **決9**: フェーズ3の処理は、CLI だけで使うもの（端末の入出力、表示）を除いて `internal/ops` に置く。
  - 推奨: この形で確定する。4f で GUI がそのまま呼べるようにするため。

- **決10**: rebase の時期。
  - 推奨: 確認が済んだらすぐ、Task 8 を直す前に rebase する。並行しているフェーズ4b（`phase4b-views`）も `internal/ops` を触るので、4b が先にマージされたら、そのたびに追いかけて rebase する。

### 3.2 3a の残り（Task 9〜14）

- **決11**: `UpdateCommit` は、ID の解決や書き込み前の検査（purge の「削除済みでない」など）の後に呼ばれる。そこで拒否すると、読み取り専用の Store でも先に別のエラーが出ることがある。spec は「Store を開いた時点で拒否する」と書いている。
  - 推奨: spec のとおり、書き込みの操作が Store を開いた直後に拒否する。そのために、書き込みの操作が Store を開く関数を1つにし（例: `Session.OpenForWrite`）、そこで拒否する。`UpdateCommit` にも同じ検査を残し、入口を通らない書き込みを取りこぼさないようにする。
- **決12**: GUI の「今すぐ同期」を、`push_denied` を外す「明示的な同期」（判断11）に含めるか。
  - 推奨: 含める。利用者が自分で押した同期なので。
- **決13**: GitHub の情報を取るのに失敗したとき、`checked` を進めるか。進めないと、バックグラウンドの同期のたびに（5分を待たずに）取り直し、毎回警告が出る。
  - 推奨: 失敗しても `checked` を進め、次は5分後に試す。判定は変えない（spec のまま）。
- **決14**: `discover --from` で、アカウントが2つ以上あり `--account` がないときは、アカウントなしでマウントする（spec）。アカウントのない Store に書くと、作者は git の設定から取る（今の `config.CommitAuthor`）。GitHub の Store では「自分」と認識されない作者になる。
  - 推奨: 2.5 の「選択」として扱う。端末なら選ばせ、端末でなければ `choice.required`（終了コード2）。GitHub の discover はアカウントが候補ごとに決まるので変わらない。

### 3.3 3b

- **決15**: `internal/mount` と `ops.Session` の関係。今の `Session` は1つの Store を前提にしている（`StoreName`、`Entry`、`Open`）。
  - 推奨: `Session` にマウント（`mount.Set`）を持たせる。一覧の操作は `Set` を回し、ID を受け取る操作は全 Store から引いて、当たった Store について `ForStore` のセッションで書く。`--store` は `Session.StoreName` のまま絞り込みに使う。spec の依存の向きに `ops → context → mount` を足す。
- **決16**: GUI からの作成（4f）は、カレントディレクトリが意味を持たないので、コンテキスト解決の 3（ブランチ名）と 4（repo）を使わない。
  - 推奨: コンテキスト解決は CLI の `hh add` だけが使い、ops の作成は入れ先（Store・親・ワークスペース）を必ず受け取る。spec の「コンテキスト解決」にそう書く。
- コメントの本人確認・`@me`・メンションの警告の比べ方は決3で決まる。開けない Store の警告は決5で `Session.Warn` に出る。

### 3.4 3c

- **決17**: `purge --rewrite-history` の確認を GUI（4f）でどうするか。決6では CLI の端末でしか通らない。
  - 推奨: ops の入力に「入力された Store 名」を持たせ、CLI は端末で、GUI はダイアログで Store 名を入力させる。どちらも `--yes` に当たるものは作らない。
- **決18**: 履歴の書き換えを取り込むとき、作り直したクローンとディレクトリを入れ替える。Windows では、別のプロセス（GUI サーバー）がその Store のファイルを開いていると rename が失敗する。
  - 推奨: 入れ替えが失敗したら 4 の「途中で失敗したときの直し方」と同じ扱いにし、次に開いたとき（または `doctor --fix`）にやり直す。GUI サーバーは同期を自分で走らせるので、サーバーの中で取り込むときは、入れ替えの前にその Store の index を閉じる。
- `mv --store` のロックの順は決8で、確認と `--scoped` は決6で決まる。

## 4. 複雑すぎる箇所を簡単にできないか（手順4）

機能は削らず、先送りもしない。各案に、spec の「利用者の決定」のどれを満たすかを書く。

### 4.1 移動と同時の変更の競合

spec は、転送ファイルとタスクの競合（フィールドごとの差分を記録）、転送ファイルどうし（専用の LWW）、転送ファイルと削除、転送ファイルの下のコメント・作業時間（ファイル全体を記録）、修復 `parent_moved`・`ref_moved` を、それぞれ別の規則として書いている。

- **決19**: 次のように規則を減らす。
  1. 移動したタスクに属するファイル（タスクファイル、その下のコメントと作業時間の記録）へのもう片側の変更は、ファイルごとに負けとし、`Hedgehug-Overwritten: <パス> * <ファイル全体の JSON> <author>` で全文を記録する。タスクファイルもコメントと同じ扱いにし、フィールドごとの差分は取らない。
  2. 転送ファイルどうし（`moved_to` が違う）は、専用の規則を作らず、ふつうのフィールドのマージ（`moved_to` の LWW と `Hedgehug-Overwritten`）に任せる。結果は spec と同じになる。
  3. 転送ファイルと削除は 1 に含まれる（削除もそのファイルへの変更）。
  4. `parent_moved` は、今の `parent_deleted`（子をルートへ移す）の条件に「親が転送ファイル」を足して同じ処理で直す。通知の種類は `parent_moved` として分ける。
  5. `ref_moved` は今のところ同じ形の修復がないので、そのまま作る。
  - 満たす決定: 「移動と同時の変更: 転送ファイルを勝たせ、負けた変更を記録して通知する」。記録の粒度がフィールドからファイルに変わるが、負けた内容はすべて残り、inbox で全文を見られる（`inbox revert` は移動済みでは使えないので、フィールドごとの記録を使う場所がない）。
  - 推奨: この形にする。

### 4.2 履歴の書き換えの自動取り込み

作り直したクローンに未 push の変更を3-wayマージで戻すこと（利用者の決定）は、そのまま守る。複雑さの多くは、ディレクトリの入れ替えの途中で落ちたときの復旧を、Store を開くすべての経路が知っていることにある。

- **決20**: 入れ替えの途中で落ちた状態からの復旧を、Store を開く処理から外し、`doctor --fix`（spec の `rewrite_leftover`）と、取り込みを始める処理の最初だけで行う。Store を開いたときに残りを見つけたら、`warning: org-a の履歴の書き換えの取り込みが途中です（hh doctor --fix で仕上げます）` を出し、`<path>` がなければその Store を「開けない Store」として扱う。
  - 満たす決定: 「履歴の書き換えの取り込み: 作り直したクローンに未 push の変更を3-wayマージで自動で戻す」。取り込みと復旧の機能はすべて残る。自動で直る時機が「次に開いたとき」から「次の同期・doctor --fix」に変わる。
  - 推奨: この形にする。`mount.Open`・`store.Open` が書き換えの状態を知らなくて済む。

### 4.3 読み取り専用の3種の判定

GitHub の権限、ローカルのパスに書けるか、push の拒否の3種を見直した。どれかを外すと、clone の直後に読み取り専用と分かる場面が減る（ローカルのパスなら最初の push まで分からず、未 push のコミットが残る）ので、要求を満たしたまま減らすことはできないと判断した。

- 代わりに、外す規則を「判定の元が書けると示したら外す」の1文にまとめる（GitHub は `push` が true、ローカルは一時ファイルを作れた、push の拒否は次の明示的な同期で試して通った）。中身は spec と同じで、書き方だけを揃える。
- 判定は `sync.CheckAccess` の1つの関数で行う（plan のまま）。

### 4.4 見直したが変えないもの

- コンテキスト解決の5段階、`sort:status` の Store をまたぐ並び、Store 名付きの ID の表示（接頭辞が重なっている間だけ）: どれも利用者の決定か親設計の要求で、決6・決16で端末と GUI の扱いが決まれば、これ以上簡単にする余地はない。
- `mv --store` の2コミットと再実行: 2つの Store に1つのコミットでは書けないので、この形が最小。

## 5. Task 8 と最後の直しで直すもの（確認の後）

決まりに合わせて、次を直す。

- Task 8: login の比較（決3。config の重複判定にも `EqualFold` を入れ、テストを足す）、`--host` の食い違いは終了コード2のまま、keychain と config の巻き戻し（決7）、`auth.lock`（決8）、`auth status` の JSON（決4）、`x.Host == host` の比較（決3）。
- Task 1・2 の Minor（最後の直しで片付ける予定のもの）: `default_store`・`url_rewrites` などが非スカラーのときのエラー文、`store default` が同じ値でも Save する、Task 2 のテスト不足（Delete の ErrNotFound、壊れた DeviceToken JSON、未知の Kind、device 未ログインの案内、wrapKeyring のヒント、Keychain() の nil）、remote URL の解析が `context.Split` だけかの確認。

## 6. 再開の手順

1. この文書の決1〜決20に答えてもらう。
2. spec を直す（1 の確定、2 の「共通の決まり」の節、3・4 の結論）。
3. plan の Task 8〜14 を 4a の後のコードに合わせて書き直す（3.1）。plan の冒頭の判断1〜29は spec へ移し、plan からは参照だけにする。
4. ブランチを main へ rebase し、テストを通す。
5. SDD の台帳を作り直し、Task 8 の修正から再開する。
