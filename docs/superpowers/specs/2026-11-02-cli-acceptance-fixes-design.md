# CLI の受入テストの修正 設計

- 日付: 2026-11-02
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「CLI 仕様」「Store とアカウント」「認証」「GitHub App」「コンテキスト解決」「Store 間の移動（昇格）」
- 前段: PR #30（4d-2）マージ後の main（6f060f0）での CLI の受入テスト
- 受入テストの記録: CLI リファレンス（https://claude.ai/artifact/Arnn3ZSK2uuCD16fC1XBGN）、ローカル受入テスト手順（https://claude.ai/artifact/PwxHewn4T7H1i94o9r9U8b）

## 目的

受入テストで利用者が気になった点と、試走で見つかった問題を直す。4d-3（ツリー）に進む前に入れる。どれも 2026-11-02 に利用者が決めた。

## 決定の一覧

| # | 対象 | 決定 |
| --- | --- | --- |
| 1 | `hh workspace prefix` | `hh workspace set-prefix <ws> <P>` に改名する |
| 2 | `hh comment <id> [text]` | `hh comment add <id> [text]` にし、`comment edit` / `comment rm` と揃える。`hh comment <id>` は廃止 |
| 3 | `hh show` | 件名と進捗を項目の行として出す |
| 4 | `hh store remove` | 既定でローカルのディレクトリも消す。消すと戻らない場合は拒否する。`--keep` で残す。`--purge` は廃止 |
| 5 | アカウント | 必須にする。トークンなしのアカウントを足す `hh auth add` を作る |
| 6 | GitHub App の client ID | バイナリに組み込まない。Device Flow は `hosts[].client_id` を書いたときだけ使える |
| 7 | `hh add` の repos | 既定で付けない。`--repo` で付ける。`--no-repo` は廃止 |
| 8 | git worktree | worktree の中でもカレントのリポジトリを見つける |
| 9 | `hh purge` | `hh purge [--older-than <期間>]`（既定 90 日）に一本化する。`purge <id>`・`purge --moved` は廃止 |
| 10 | `hh purge --rewrite-history` | 廃止する。書き換えられた履歴への追従は、記録に頼らない `hh doctor --fix` の修復にする |
| 11 | GUI と API の完全削除 | 外す（`DELETE /api/tasks/{id}?purge=true`、タスク詳細の「完全削除」） |
| 12 | 文言 | 英語の複数形を揃える。`ls --help` に `-` で始まる項の書き方を足す |

変えないと決めたもの: 担当者は host 付き（`github.com/bob`）のまま。clone より前のコミットは inbox に出さない。WSL でブラウザが開かない件は直さない。プロジェクト・ワークスペース単位の watch は作らない。`ls --json`・`--format` に `repos`・`watchers` を足さない（phase1d1 の spec のとおり `show --json` だけ）。ID を渡すコマンドで `--store` を要らなくする件は、別に全体を調べる。長期運用での履歴の増加は、別にブレストから始める。

## 1. `workspace set-prefix`

- `hh workspace prefix <ws> <P>` を `hh workspace set-prefix <ws> <P>` にする。動き・出力・`--json` は今と同じ。
- 旧名は残さない（リリース前のため）。
- 出力の `Changed the id prefix of …; renumbered N tasks` は 12 の複数形に合わせる。
- API（`PATCH /api/workspaces/…` などの既存のエンドポイント）は CLI の名前と関係しないので変えない。GUI が CLI の行を見せる箇所（`web/core/cli.ts`）に接頭辞の変更がないことを確かめた。

## 2. `comment add`

- `hh comment add <id> [text...]`: 今の `hh comment <id> [text...]` と同じ（text 省略で `$EDITOR`、メンションの自動の watcher、出力 `Added <comment-ulid>`）。
- `hh comment <id>` は「サブコマンドが要る」の使い方の誤り（終了コード 2）にし、`add`・`edit`・`rm` を案内する。`hh dep` と同じ形。
- `web/core/cli.ts` の `cliComment(id, "add")` は `hh comment add <id>` を返す。

## 3. `show` の件名と進捗

1 行目の見出しをやめ、項目の行として出す。

```
  title       フォームを作る
  id          TEAM-2
  ulid        01M6DTM7D7T6WSG0365BATZEK8
  …
  status      todo
  checklist   ☑ 1/3          ← 本文にチェックリストがあるときだけ
  progress    1/2            ← 進捗を持つ種類（container の役割）のときだけ
  parent      …
```

- `title` は最初の行。ファイルが壊れていてタイトルがなければ今と同じく `-`。
- `checklist` と `progress` は `status` の直後。値は今の `ops.ProgressText` の 2 つの部分を分けたもの。どちらも、出す条件は今の `ProgressText` が空でない条件と同じ。
- `--json` は変えない（もともと項目として持つ）。

## 4. `store remove`

| 状況 | 動き |
| --- | --- |
| `--keep` あり | config.yaml から外すだけ。ディレクトリは残す（今の既定と同じ） |
| remote があり、push していないコミットがない | config.yaml から外し、ディレクトリを消す |
| push していないコミットがある | 拒否（終了コード 1）。今の `--purge` と同じ文言で `hedgehug sync --store <name>` を案内し、`--keep` も示す |
| remote がない | 拒否（終了コード 1）。「remote のない Store を消すとタスクは戻せません。残すなら --keep で外し、要らなければディレクトリを自分で消してください」の旨 |
| ディレクトリがすでにない | config.yaml から外すだけ（今と同じ） |

- `--purge` は廃止する。
- 出力: 消したとき `Removed store <name> and deleted <path>`、`--keep` のとき今の `Removed store <name> from the config; <path> is left`。
- `default_store` が指していたら消すのは今と同じ。
- 消す処理は、ディレクトリのロック（store のロック）を、確かめる前から消し終わるまで持つ（今の `--purge` は取っていないので足す）。

## 5. アカウントを必須にする

### 5.1 アカウントなしで動くコマンド

`accounts` が空のとき、次のコマンドだけを動かす。

- `hh auth login`、`hh auth add`、`hh auth status`
- `--help`、`help`、`--version`
- `hh merge-driver`（git が同期の途中で呼ぶ。止めると git のマージが壊れる）

ほかのコマンドは、何もせずに止める（終了コード 1）。

```
hedgehug: no account is set up; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)
```

- 判定は CLI の `beforeCommand` の前、全コマンド共通の場所で 1 回だけ行う。`hh ui` もここで止まるので、GUI サーバーはアカウントがある前提で動く。
- `auth status` はアカウントがないとき、今の `No account is set up; run hedgehug auth login` を `auth add` も案内する文に変える。

### 5.2 Store のアカウント

- `stores[].account` がない Store は、`accounts` が 1 つならそのアカウントを使う。
- `accounts` が 2 つ以上で `stores[].account` がない Store があれば、config.yaml の読み込みのエラーにする（ファイルのパスと Store 名を入れる。既存の読み込みのエラーと同じ形）。
- `store init`・`store add`・`store discover` の `--account` の既定（アカウントが 1 つならそれ）は今と同じ。

### 5.3 コミットの作者

- コミットの作者は常に Store のアカウントから作る（`<login> <login@users.noreply.<host>>`。フェーズ2の spec の形）。
- `config.Author`（`.gitconfig` の user.name / user.email を読む処理）と、それを使う分岐を消す。`GIT_CONFIG_GLOBAL` を読む理由がなくなるので、`tools/devenv` の `env` が出す `export GIT_CONFIG_GLOBAL=…` と、devenv が作る `.gitconfig` も消す。
- `created_by` とコメントの `author` が空になる経路（アカウントのない Store）はなくなる。表示の `-` の扱いは、既存のファイル（空の値を持つもの）のために残す。

### 5.4 `hh auth add`

```
hh auth add <name> --login <login> [--host github.com] [--token gh|env:VAR|keychain]
```

- config.yaml の `accounts[]` に 1 件足す。`--token` を省くと `token:` を書かない（トークンを使わないアカウント）。
- 検査は config.yaml の読み込みと同じ: `name` の重複、同じ host と login（大文字小文字を区別しない）の重複、`--token` の値。
- トークンは取りに行かない（`gh` も keychain も呼ばない）。確かめるのは `hh auth status`。
- `keychain` を指定したときは、トークンを入れる手順（`hh auth login <name> --with-token`）を出力で案内する。
- アカウントが 1 つだけのとき、`stores[].account` のない Store はそのアカウントを使っている（5.2）。2 つ目を足すとそれらの Store が読み込みのエラーになるので、同じ `Config.Update` の中で、足す前に `account: <今のアカウント>` をそれらの Store に書く。`hh auth login` が新しいアカウントを足すときも同じ。出力にも 1 行出す（`Set account personal on stores me, org-a`）。読み込みのエラーは、手で書いた config.yaml でだけ起きる。
- 出力: `Added account <name> (<host>/<login>)`。`--json` は `{name, id, token}`（token は書いたときだけ）。
- config.yaml は既存の `Config.Update`（ロックと、知らないキー・コメントを残す保存）で書く。

### 5.5 テストと開発環境

- テストの config を作るヘルパー（`internal/testenv` など）は、既定でアカウントを 1 つ書く。アカウントなしの動きを確かめるテストだけが外す。
- 影響の大きさ: `cmd/hedgehug` のテスト（1万行強）の多くはアカウントなしで動いている。期待する出力の `by -`・コミットの作者（`.gitconfig` の名前）が変わるので、ほとんどの CLI のテストで fixture と期待値を直すことになる。plan ではこれを独立したタスクにする。
- `tools/devenv` はもともとアカウントを書いているので変えない。
- ローカル受入テスト手順の Artifact は、config.yaml の手書きを `hh auth add me --login alice` に置き換える。`.gitconfig` を作る手順は消す。

## 6. client ID を組み込まない

- `internal/auth` の `builtinClientID` を消す。`ClientID(h)` は `hosts[].client_id` だけを返す。
- Device Flow（`hh auth login` で `--with-token` なし）は、そのホストの `client_id` が空なら何もせずに止める（終了コード 1）。

```
hedgehug: the device flow needs hosts[].client_id for github.com in config.yaml; or use token: gh, token: env:<VAR>, or hedgehug auth login <name> --with-token
```

- `token: device` のアカウントのリフレッシュも同じ `client_id` を使う。空なら、リフレッシュの失敗として今と同じく `hh auth login <name>` を案内する。
- 親設計の「client ID は配布物に含める」を「config.yaml の `hosts[].client_id` で指定する。配布物には含めない」に直す。

## 7. `add --repo`

- 既定では、カレントのリポジトリをタスクの `repos` に書かない。
- `--repo` を付けたときだけ、カレントのリポジトリの repo ID を `repos` の先頭に足す（今の既定の動きと同じ）。カレントがリポジトリでない、または remote から repo ID を作れないときは、使い方の誤り（終了コード 2）で知らせる。
- `--no-repo` は廃止する。
- 入れ先の決め方（コンテキスト解決の 4 段目: カレントのリポジトリにひも付いたプロジェクトを探す）は変えない。`--repo` の有無に関係なく働く。
- GUI からの作成（API）は `ops.Add` を直接呼び、カレントのリポジトリを使わない（`internal/server/tasks_write.go`）ので変わらない。

## 8. git worktree の中のリポジトリ

- 原因: `internal/context/resolve.go` の `openRepo` は `git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})` で開いている。worktree の `.git` は本体の `.git/worktrees/<名前>` を指すファイルで、remote を持つ `config` は本体の `.git` にある。go-git は `EnableDotGitCommonDir` を付けないと本体の `config` を読まないので、remote のないリポジトリに見える。
- 直し方: `EnableDotGitCommonDir: true` を足す。ブランチ名（コンテキスト解決の 3 段目）は worktree 側の `HEAD` から読むので、今と同じく worktree ごとのブランチになる。
- テスト: 本物の git で worktree を作り、その中で repo ID とブランチ名が取れることを確かめる（`git` コマンドがない CI のジョブでは skip する。既存の本物の git を使うテストと同じ扱い）。

## 9. `purge --older-than`

```
hh purge [--older-than <N>d] [--store <name>] [--yes]
```

- 消すもの:
  - 削除済みタスク: `deleted_at` が「今 − 期間」より前のタスクのファイルと、そのコメント・作業時間の記録（今の `purge <id>` が消すものと同じ）。
  - 転送ファイル: `moved_at` が「今 − 期間」より前のもの。
- 期間は日数だけで書く: `90d`。`0d` はすべて。既定は `90d`。負の値や書き方の誤りは使い方の誤り（終了コード 2）。
- 対象の Store: `--store` で指定する。マウント中の Store が 1 つなら省略できる。2 つ以上で省略したら、ほかの Store を指すコマンドと同じく「Store を選んでください」のエラー（複数の Store をまとめて消す操作は、明示して 1 つずつ行う）。読み取り専用の Store は push できないので拒否する。1 コミットで書く。
- 子が削除されていない削除済みタスク（doctor が直す前の状態）は消さずに残し、警告で ID を並べる。今の `purge.child_not_deleted` の判定を使う。
- 確認: 端末なら対象（Store ごとに、削除済みタスクの ID とタイトル、転送ファイルの数）を出して `[y/N]` で聞く。`--yes` で聞かない。端末でなく `--yes` もなければ、確認が要るエラー（終了コード 2）。今の `purge --moved` の確認と同じ形。
- 出力: `Purged N tasks and M forward files from <store>`（12 の複数形）。対象がなければ `Nothing to purge`。`--json` は `{store, tasks:[{ulid,id,title}], forwards:[{ulid,moved_to,moved_at}], commit}`。
- `workspace rm` は削除済みタスクも数えるので、`rm` した直後は消せない。拒否の文言（今の `… or purge them`）を `… move them with hedgehug mv --workspace, or delete them with hedgehug rm and remove them with hedgehug purge --older-than 0d --store <store>` にする。`undo` の同じ文言も同じにする。タスクが 1 件のときは 3 つの `them` を `it` にする（2026-11-03 に利用者が決定）。
- コミットの op は今の `purge` と `purge-moved` を `purge` 1 つにする。
- `undo` は今の `purge` と同じく取り消せない操作として扱う（理由の文言も今のまま）。
- `purge <id>`・`purge --moved` は廃止する。位置引数を渡したら使い方の誤りにし、`--older-than` を案内する。

### 9.1 転送ファイルの `moved_at`

- 転送ファイルの項目を `ulid`・`moved_to`・`moved_at`（RFC 3339）の 3 つにする。3 つとも必須。リリース前なので、`moved_at` のない古い形式は読まない（壊れたファイルとして扱う）。
- `mv --store` が転送ファイルを書くときに、その操作の時刻を入れる。
- マージ（同期とマージドライバ）は今の転送ファイルの扱いのまま。`moved_at` は両側で同じ値になる（同じファイルを片側だけが書く）ので、競合したら大きい方。
- 親設計の転送ファイルの記述（「`moved_to` だけを持つ転送ファイル」）に `moved_at` を足す。

## 10. `--rewrite-history` の廃止

- 消すもの: `hh purge --rewrite-history`（`cmd/hedgehug/purge_rewrite.go`）、`internal/ops/rewrite.go` の書き換えの操作、`internal/gitrepo/rewrite.go` の履歴の書き換え、それらのテスト。`undo` の `rewrite-history` の理由の文言。
- 「履歴を書き換える機能」と「書き換えられた履歴に追従する機能」を分ける（2026-11-02 に利用者が決定。1 セットにしていたのが設計の誤り）。前者は消し、後者は書き換えの記録に頼らない doctor の修復にする。
- 消すもの（追従の側）: `.hedgehug/rewrites.yaml` の記録（`internal/model/rewrites.go`）と、記録を見て同期の中で自動で clone し直す処理（`internal/sync/rewrite.go` の `Detect` の記録を読む部分、`rewrites-applied.json`、ミラーの書き換えの扱い）、doctor の `rewrite_leftover` の検査、それらのテスト。
- 同期: fetch のあと、origin の先頭が手元で追っていた履歴を含まなければ、今と同じく同期を止めて `force-push.json` に記録する（今の `ErrForcePush`）。文言は「origin の履歴が手元と食い違っている。確かめて hedgehug doctor --fix で clone し直す」の旨にする。同期の中では自動で直さない（誤った force push でも全員の手元がそれに揃ってしまうため）。
- doctor: 今の `unexpected_force_push` の検査はそのまま。`--fix` で直す対象に足す。直し方は、push していない変更を退避し、origin から clone し直し、退避した変更を戻す（今の `internal/sync/rebuild.go` の作り直しを、記録なしで使う）。直したら `force-push.json` を消す。`--fix` の 1 コミットの修復（親の循環など）とは別の手順なので、出力では作り直しを別の行で知らせる。戻せなかった変更があれば、退避先のパスを出す。入れ替えの直前に古い方の HEAD が変わっていたら（今の作り直しは次の同期でやり直す）、入れ替えずにやめて `hedgehug doctor --fix` のやり直しを案内する。
- マージ後の修復（親の循環、ID の重複など）は別の仕組みで、変えない。
- `mv --store` の確認の文言から `use hedgehug purge --rewrite-history to remove it` を消し、「移動前の内容は <store> の git 履歴に残り、<store> を読める人は見られる」までにする。
- 親設計の「Store 間の移動（昇格）」の `hh purge --rewrite-history` の段落を、「履歴から消す機能は持たない。消す必要があれば git filter-repo で手作業で書き換え、ほかのクローンは次の同期で食い違いを見つけて止まり、hedgehug doctor --fix で clone し直す」に直す。ドキュメント（フェーズ7）にこの手順を書く。

## 11. GUI と API の完全削除を外す

- `DELETE /api/tasks/{id}` の `purge`・`confirm` のクエリと、`purge.unconfirmed`・`purge.changed`・`purge.child_not_deleted` の応答を外す。`api/openapi.yaml` を生成し直し、`web/core/api/schema.d.ts` も更新する。
- `web/core/cli.ts` の `cliPurge`、`web/core/messages.ts` の `purge.*` のメッセージ、関係するテストを消す。
- GUI の spec（phase4）のタスク詳細と Dialogs の「完全削除」を消す。期間を指定した purge は GUI の「設定 → 同期と保守」に置く（2026-11-02 に利用者が決定）。画面と API はその画面を作る 4e で作り、phase4 の spec の 4e の範囲に足す。

## 12. 文言

### 12.1 英語の複数形

- 数を入れる文言は、1 のとき単数、それ以外は複数にする。`cmd/hedgehug` と `internal/ops` に `plural(n int, one string) string`（`1 task`、`2 tasks`）のような小さな関数を 1 つ置き、数を入れる文言はすべてそれを通す。
- 対象の例: `Synced <store>: N merges`、`move N tasks to <store>`、`Move N tasks to <store>? [y/N]`、`N commits not pushed`、`renumbered N tasks`、`and N tasks under it`、`N tasks now have problems`、`still has N tasks`。実装の最初に `%d` を含む英語の文言を全部洗い出し、plan に一覧を書く。

### 12.2 `ls --help`

`ls` の Long に次を足す。

```
A term that starts with - (a negation, such as -status:done) is read as a
flag; put it after --: hedgehug ls -- -status:done
```

## 後片付け

- 親設計: 「CLI 仕様」のコマンド一覧（`workspace set-prefix`、`comment add`、`purge [--older-than]`、`auth add`）、「Store とアカウント」の `store remove`、「認証」「GitHub App」の client ID、「コンテキスト解決」の後の `add --repo`、「Store 間の移動（昇格）」の `--rewrite-history`、転送ファイルの `moved_at`、アカウントが必須であること。
- 既存の spec: phase1d1（`comment`、`purge`、`show`、`add --no-repo`）、phase1-5（`workspace prefix`）、phase2（コミットの作者、`.gitconfig`）、phase3（`purge --moved`・`--rewrite-history`・`store remove --purge`・`builtinClientID`・アカウントなしの動き）、phase4（完全削除）は、今回の変更を書いた節を足すか、該当の記述を直す。
- Artifact: CLI リファレンスとローカル受入テスト手順を、変更後のコマンドと出力に合わせて更新する。
- README は機能がまだない段階の記述なので変えない。

## テスト

- 各決定に、変更後の動きを確かめるテストを足し、廃止したコマンド・オプションが使い方の誤りになることを確かめる。
- ローカル受入テスト手順を、変更後のバイナリで最初から最後まで通す。
- 全テストは仕上げに 1 回だけ回す。

## plan で決めた細部（2026-11-02、利用者の承認）

plan `docs/superpowers/plans/2026-11-02-cli-acceptance-fixes.md` の各 Task 末尾の「spec にない細部」は、すべて承認済み。主なもの:

- `undo` は `purge` のコミットを `undo.blocked`（`a purge cannot be undone`）で断る（今のコードは断っていなかった）。
- 英語の複数形は葉のパッケージ `internal/plural`（`Count`・`Pick`）に置き、`cmd`・`ops` に加えて `doctor`・`store`・`sync` の文言も揃える。
- `doctor --fix` の作り直し: 同期の中で入れ替える仕組みとミラーの force push を消す。戻せなかった変更があれば古い Store を `<path>.kept-<時刻>` に残す。作り直しは push せず次の同期に任せる。読み取り専用の Store も作り直す。
- `store remove --json` の `purged` は `deleted` に改名する。アカウントが 2 つ以上で `--account` がないときの `store init`・`store add` は `choice.required` で拒否する。`DELETE /api/tasks/{id}?purge=true` の `purge` は無視して通常の削除にする。`purge --older-than` の境界は「以前」を含み、上限は `36500d`。
