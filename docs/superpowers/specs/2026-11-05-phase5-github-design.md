# フェーズ 5: GitHub 連携の設計

- 日付: 2026-11-05
- 親設計: Hedgehug 設計ドキュメント（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）の「実装フェーズ」5. GitHub 連携、「GitHub 連携」、「開発環境」の「GitHub API のフェイク」
- 画面のデザイン: Hedgehug GUI デザイン（https://claude.ai/artifact/K14rwULxyyS2nYNbMKSr5L）
- 前段: `docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md`、`docs/superpowers/specs/2026-11-01-phase4-gui-design.md`、`docs/superpowers/specs/2026-11-04-codebase-restructure-design.md`

## 位置づけ

フェーズ5は1つの spec と、小フェーズごとの plan・ブランチ・PR で作る。実装の最初の回は、構成見直しの回 6 がマージされた後の main から切る（2026-11-05 に利用者が決定）。構成見直しが cli・server・usecase・lib/github を組み替えている間は、この spec だけを進める。

| 小フェーズ | 内容 | 完了条件 |
|---|---|---|
| 5a リンクと取り込み | `gh` の型とマージ、`hh link`・`hh unlink`、自動リンク、GraphQL、fakegh の PR・Issue・レビュー・checks、`repo_accounts`、`has:pr`、レート制限、実 API の定期テスト | `hh sync` で PR・Issue の状態が `gh` に入り、`hh show` と `--json` に出る。別のアカウントの repo も `repo_accounts` のとおりに読める |
| 5b 自動化と開発の開始 | `automations.yaml`、ルールの実行、`hh automation`、`hh start`・`hh pr`、`hh store init --template dev` のルール、`hh mv --store` での持ち運び、inbox | CLI だけで `hh start` から PR のマージまでを、手動のステータス変更なしで回せる（シナリオテスト） |
| 5c Issue 同期 | `mirror`・`push`・`link`、アダプタ、`hh issue`、`hh gh import` | 3つのモードと一括取り込みが fakegh で動き、手編集の上書きが inbox に出る |
| 5d-1 Go | API、SSE、OpenAPI、取得とレート制限の状態、ルールの回数の集計 | 全エンドポイントを Go のテストで確かめ、生成物に差分がない |
| 5d-2 表示 | PR のチップ、タスク詳細、プロジェクト、通知、アクティビティ、状態の表示 | 下の「5d の画面」の表示がブラウザで動く（E2E） |
| 5d-3 設定 | 自動化の設定、プロジェクトの設定、`repo_accounts`、GitHub App の警告、Issue の取り込み | 下の「5d の画面」の設定がブラウザで動き、フェーズ5の CLI の操作が GUI でもできる（`hh start`・`hh pr` はコマンドのコピー。E2E） |

フェーズ5全体の完了条件は親設計のとおり、「`hh start` から PR のマージまでを、手動のステータス変更なしで回せる」こと。

## パッケージ

構成見直しの 2.1 の配置に足す。cli と server は usecase だけを通す（4.2 の depguard）。

| 場所 | 置くもの |
|---|---|
| `domain/model` | `gh` の型、リンクの種類、`Link.Fields` |
| `domain/merge` | `gh.prs`・`gh.issues` の要素ごとのマージ |
| `domain/automation`（新規） | `automations.yaml` の形式、検査、イベントの判定、ルールの評価。入出力は値だけで、I/O を持たない |
| `lib/github` | GraphQL のクライアント、PR の作成、Issue の作成と更新、ラベル・担当者・マイルストーンの読み取り |
| `lib/gitrepo` | HEAD から作るブランチの作成、作業中のブランチの読み取り、コード repo の push |
| `lib/fakegh`・`cmd/fakegh` | PR・Issue・レビュー・checks、GraphQL、操作用のコマンド |
| `storage/ghstate`（新規） | repo ごとの最終取得時刻、走査の起点（`Since`）、失敗した試行の時刻と警告、レート制限の状態。`.hedgehug/local/github.json` に置く（Store の履歴には入らない） |
| `usecase`（土台） | 同期（`replication`）から呼ぶもの: 取り込み（`FetchGitHub`）、自動リンク、取り込みの順番の 5 のルールの実行と Issue 同期のモードの反映（5b・5c）。領域どうしは import しないため（構成見直しの spec 決定 20。2026-11-05 利用者が決定） |
| `usecase/integration`（新規） | リンクの付け外し、`hh start`・`hh pr`・`hh issue`・`hh gh import` |
| `usecase/automations`（新規） | ルールの一覧と編集 |

2つの新しい領域を足すので、構成見直しの spec の 2.3 の領域の数を 12 から 14 に直す。

## データの形

### タスクファイル

```yaml
links:
  - {type: github_pr, ref: github.com/org-a/web/pull/123, sync: mirror}
  - {type: github_issue, ref: github.com/org-a/web/issues/45, sync: push, fields: [title, status]}
gh:
  prs:
    - ref: github.com/org-a/web/pull/123
      state: open
      review: pending
      checks: success
      merged_at: null
  issues:
    - ref: github.com/org-a/web/issues/45
      state: open
      state_reason: null
      synced: {title: Web の API クライアントを v2 に更新, status: open}
```

`links` は人と自動リンクが書く値で、`gh` は GitHub から取得して Hedgehug が書く値である。

| キー | 値 |
|---|---|
| `gh.prs[].ref`・`gh.issues[].ref` | リンクと同じ ref。配列の要素を見分ける（親設計に追加） |
| `gh.prs[].state` | `open`・`draft`・`merged`・`closed` |
| `gh.prs[].review` | `none`・`pending`・`approved`・`changes_requested`（`none` は親設計に追加） |
| `gh.prs[].checks` | `success`・`failure`・`pending`。CI がない PR では書かない |
| `gh.prs[].merged_at` | マージ日時 |
| `gh.issues[].state` | `open`・`closed` |
| `gh.issues[].state_reason` | `completed`・`not_planned`（親設計に追加）。open では書かない。`DUPLICATE` は `not_planned`、理由のない古い closed は `completed` |
| `gh.issues[].synced` | 最後に揃えたときの各フィールドの値（親設計に追加） |

`review` の `none` は、レビュー依頼がない状態を表す。`pending` と区別しないと `pr.review_requested` を判定できない。

`review` は次の順で決める。GitHub の `reviewDecision` が `APPROVED`・`CHANGES_REQUESTED` ならそれ。必須レビューのない repo では `reviewDecision` が null になるので、それ以外のときは各レビュアーの最新のレビューを見て、`CHANGES_REQUESTED` があれば `changes_requested`、なければ `APPROVED` があれば `approved`。どちらもなく、レビュー依頼が1件以上あれば `pending`。ほかは `none`（`REVIEW_REQUIRED` で依頼が0件のときも `none`）。

`sync` の値は `mirror`・`push`・`link`・`off` である。PR の `sync` は常に `mirror` で、`off` だけに変えられる。

`hh unlink` はリンクを消さず、`sync: off` にする。`off` のリンクは、表示と取得から外す。自動リンクは `off` の ref を飛ばす。消してしまうと、本文に `hedgehug:<ULID>` が残っている PR が次の走査でまた紐付く。

`model.Task.GH` は `*yaml.Node` のまま持ち、`GitHub()`・`SetGitHub()` で型付きの値を読み書きする。知らないキーは保持する。今の形（`number`・`state` など）を含め、`prs`・`issues` 以外のキーは読めて、そのまま書き戻せるようにする。`model` のテストは変えずに通す。`merge/task_test.go` は、`Link` が `Fields` を持って comparable でなくなるため、`Link` の比較の2箇所だけ直す（期待値は変えない）。`prs`・`issues` の要素は、変えない部分を元のノードのまま残す（知らないキー、`1.0`、日付の形、コメント）。

### マージ

`gh.prs`・`gh.issues` は `ref` で要素を見分け、要素ごとにキー単位でマージする。値が衝突したら新しい時刻の側を採る。`gh` の衝突は inbox に出さない。次の取得で GitHub の値に戻るためである。`gh` だけの変更は、削除との競合で編集に数えない。別のメンバーが消したタスクを、取り込みが復活させないためである（`links` は人の値なので、数える）。

`links` のマージは今のまま（`ref` を見分ける集合）。同じ `ref` で `sync`・`fields` が両側で違うときは、タスクの `updated` が新しい側を採り、負けた値は上書きの記録に入る。`unlink` と別のメンバーのモード変更が同時に起きても、どちらかの値に収束する。5a のテストでこの動きを固定する。

### `automations.yaml`

```yaml
rules:
  - id: pr-opened
    on: pr.opened
    set: {status: in_progress}
  - id: pr-review
    on: pr.review_requested
    set: {status: review}
    enabled: false
  - id: pr-merged
    on: pr.merged
    when: all_linked_prs_merged
    set: {category: done}
  - id: issue-closed
    on: issue.closed
    set: {category: done}
  - id: release-review
    where: "project:01JBZ7XXH2M4Q8R1T5V9W3Y6ZA"
    on: pr.review_requested
    set: {status: waiting}
scoped:
  - where: "project:01JBZ7XXH2M4Q8R1T5V9W3Y6ZA"
    disable: [pr-review]
issue_fields:
  default_sync: link
  push: [title, body, status, assignees, labels]
  mirror: [title, status, assignees]
  props: {milestone: target}
```

`rules` は `id`、`scoped` は `where` で見分ける。今の `kindYAML` のマージをそのまま使い、`issue_fields` はキー単位でマージする。ファイルがないときは、ルールは空で、`issue_fields` は上の `default_sync`・`push`・`mirror` を既定に使う。`props` の既定は空である（マイルストーンは既定で無効）。

`set` は `status` か `category` のどちらか1つ。`when` は `all_linked_prs_merged` だけを持つ。`hh store init --template dev` は、`pr-opened`・`pr-review`・`pr-merged`・`issue-closed` の4つを有効にする。`pr-review` の行き先は `schema.Dev()` の `review` である。

`hh doctor` と `hh schema check` は、`id` の重複、存在しないステータス・カテゴリ、存在しないプロジェクトを指す `where`、存在しない `id` を指す `disable`、知らない `on`・`when`・`issue_fields` の値を報告する。

## 5a リンクと取り込み

### コマンド

| コマンド | 動き |
|---|---|
| `hh link <id> --pr org/repo#123` | `links` に足す |
| `hh link <id> --issue org/repo#45 [--sync mirror\|push\|link]` | `links` に足す。既定は 5a では `link`、5b から `issue_fields.default_sync`。`automations.yaml` が読めないか `default_sync` が不正なら、`--sync` なしの指定は `automation.invalid` で失敗する。リンク済みならモードを変える |
| `hh unlink <id> --pr ...`・`--issue ...` | `sync: off` にし、`gh` の要素を外す |
| `hh show`・`--json` | リンクと `gh` を出す |

`has:pr` は、`off` でない PR のリンクがあるタスクに一致する。`hh edit` は `gh` の変更を拒否する（`tasks.CheckEdit` に足す）。`gh` の値を書くのは取り込みだけにする。`hh link`・`hh unlink` は要素を外すだけ。

### 取り込みの順番

`hh sync` と GUI の定期同期は、Store ごとに次の順で動く。

1. 今の同期を実行する
2. 対象の repo ごとに、前回の取得より後に更新された PR・Issue と、リンク済みの PR・Issue を GraphQL で取る
3. 自動リンクを判定する
4. 変わった `links`・`gh` を1コミットにする
5. ルールとモードの反映を実行する（5b・5c）
6. コミットができたときだけ、もう一度 push する

先に今の同期を実行するのは、別のメンバーが取り込み済みの変化を自分のものとして扱わないためである。

| 項目 | 決まり |
|---|---|
| 対象の repo | タスクの `repos` と、`off` でないリンク先の repo。対象の repo を持たない Store は、全体を飛ばす |
| 間隔 | repo ごとに5分。明示の `hh sync` でも守る。初回の走査は更新の新しい 100 件まで |
| 走査の起点 | repo ごとの `Since`。走査で見た PR の `updatedAt` の最大値（GitHub の時計）で、更新がなければ据え置く。手元の時計は間隔の判定だけに使う |
| 失敗の繰り返し | 失敗した試行の時刻 `LastTry` と警告文 `Warning` を `ghstate` に残す。間隔は `LastFetch` と `LastTry` の新しいほうから数え、間隔内は問い合わせずに `Warning` を再掲する。読めたら消す。自動リンク先が GitHub にない警告も同じように再掲する |
| 取得時刻 | 取得に成功し、見つけたものを書けたときだけ進める。タスクに書けなかった repo は進めない |
| remote のない Store | `hh sync`（引数なし）、`hh sync --store`、GUI の定期同期のどれでも取り込む（push はしない）。`hh sync`（引数なし）の対象は、remote のない Store のうち取得対象の repo を持つもの。GUI の定期同期は書き込みのたびに対象を決めるので、索引を開かずにすべての Store を回し、対象の repo がなければ同期の中で何もしない。remote のない Store への書き込みは push の待ちにせず、次の取り込みは interval 後とする |
| レート制限 | 取得の応答の `rateLimit` を `ghstate` に残す。残量が 10% を切ったら間隔を3倍にし、回復の時刻まで延ばす |
| 閲覧のみの Store | 取り込まない |
| アカウント | `repo_accounts` を上から当てはめる。一致しなければ Store の `account`。読めなければ repo 名を付けて警告する |
| 失敗 | 警告だけを出し、同期は成功のままにする |
| author | `hedgehug-automation（実行したメンバー）` |
| コミット | 1回の取得につき1つ。op は `gh-fetch`。`hh undo` の対象にしない。Issue の反映はその後のタスクごとのコミット（op は `gh-mirror`・`gh-push`）で、同じく `hh undo` の対象にしない |
| アクティビティ | 5a〜5c の間は「同期」に入れる。5d-1 で「自動化」の絞り込みを作るときに移す |

GitHub Enterprise Server の GraphQL の場所は、`Host` の API の URL から求める（`/api/v3` を `/api/graphql` に置き換える。github.com は `/graphql`）。

### 自動リンク

親設計のとおり、次のどれかでタスクを指している PR を紐付ける。

- ブランチ名: `hh/<ID>-...`・`hedgehug/<ID>-...`
- タイトル・本文・コメント: `hedgehug:<ULID>`・`hh:<ULID>`
- タイトル・本文・コメントの ID（`API-123` の形）: その repo を参照している Store の接頭辞に一致するものだけ。`former_ids` も含む

本文の最終行（末尾の空白を除く）が `hedgehug:<ULID>` の PR は、タイトルと本文の ID と印を読まず、その印のタスクだけを紐付ける。`hh pr` はタスクのタイトルと本文を PR に写すので、そこで触れただけの別のタスクを紐付けないためである。ブランチ名とコメントは読む。

コメントから付いた紐付けは、コメントが編集・削除されても外さない。

### fakegh

親設計のとおり、Hedgehug が使う GraphQL のクエリと REST のエンドポイントだけに答える。状態は操作用のコマンドで動かす（`fakegh pr open`・`pr merge`・`pr review`・`pr checks`、`fakegh issue open`・`close`・`edit`）。

## 5b 自動化と開発の開始

### 完了条件の流れ

| 操作 | 起きること | ステータス |
|---|---|---|
| `hh start WEB-42` | `hh/WEB-42-<slug>` を作り、自分を担当者にする | `doing` の既定 |
| `hh pr WEB-42` | push して PR を作り、リンクして、その repo をすぐ取り込む | 変わらない |
| レビュー依頼 → `hh sync` | `pr-review` が発火 | `review` |
| マージ → `hh sync` | `pr-merged` が発火（リンク中の PR がすべてマージ済みのとき） | `done` の既定 |

### イベント

取り込みの前後の `gh` の差分から作る。比べるのは Store に書かれた値で、手元の記憶は使わない。別のメンバーが書いた遷移が同期で届いても発火しない。あとから人がステータスを戻しても、再び発火しない。

| イベント | 発火する条件 |
|---|---|
| `pr.opened` | `gh.prs` に `open` か `draft` の PR が新しく入った |
| `pr.review_requested` | `review` が `none` から `pending` になった |
| `pr.merged` | `state` が `merged` になった |
| `issue.closed` | Issue の `state` が `closed` になった |

`hh unlink` は `gh` から要素を外すので、そのあと同じ PR を `hh link` し直すと、`gh` に新しく入る。そのため `pr.opened` が発火する。

`gh` に前の値がない要素は、`state` は未定、`review` は `none` として判定する。取得の間に開いてマージされた PR を初めて見たときも `pr.merged` が発火する。

### ルールの実行

- 適用の順は、Store のルール → `where` 付きのルール。それぞれファイルの上から順に当て、後が勝つ。GUI の一覧もこの順に出す
- 同じ取得で複数のイベントが起きたら、上の表の順に処理する
- `enabled: false` のルールと、`scoped` の `disable` に入っているルールは飛ばす
- 削除済みのタスクは飛ばす
- すでに目的のステータスなら何もしない。`set: {category: c}` は、今のステータスがそのカテゴリなら何もしない（既定でない完了のステータスを既定に戻さない）。違えば `c` の既定のステータスにする
- `ValidateTransition` で、プロジェクトの上書きを含めて判定する。拒否されたらステータスは変えない
- 同じ取得でルールが複数当たるときは、順に当てて途中の値から `ValidateTransition` する。拒否されたルールは記録して飛ばし、次のルールへ進む。コミットは最後に通った値へ1つで、trailer は最後に通ったルールを書く
- `when: all_linked_prs_merged` は、`off` でない PR のリンクが1件以上あり、そのすべてが取り込み後の `gh.prs` で `merged` のとき成り立つ。`gh` にまだ入っていない PR と `closed` の PR は満たさない
- ルールの評価は取り込みのコミットの中で行う。拒否をそのコミットの trailer に書くためである。ステータスのコミットまでストアのロックを持ち続ける
- ステータスの変更はタスクごとに1コミットにする。op は `set`、author は `hedgehug-automation（実行したメンバー）`、trailer は `Hedgehug-Automation: <rule id> <event>`。`hh undo` は1件ずつ効く。`hh undo` は元の値へ戻す操作なので、手動の変更も含めて、ワークフローの遷移と必須フィールドを問わない
- 拒否は、取り込みのコミットの trailer に `Hedgehug-Automation-Rejected: <ULID> <rule id> <理由>` と書く

### 通知

inbox に種類 `automation`（結果）と `automation_rejected`（拒否）を足す。inbox は手元のものなので、trailer を同期で受け取った各自の手元で、そのタスクの担当者とウォッチャーに通知する。実行した本人が担当者かウォッチャーなら、本人にも出す。親設計の「自分の書き込みは通知しない」の例外である。

### `hh start`

親設計の方針どおり、`git` コマンドを使わず go-git で行う。

- カレントのリポジトリの HEAD から `hh/<ID>-<slug>` を作り、HEAD をそこへ向ける。コミットは同じなので作業ツリーのファイルは変わらない
- `slug` はタイトルを小文字にし、ASCII の英数字の並びを `-` でつないだもので、40 文字まで。空なら `hh/<ID>` とする
- すでに `doing` カテゴリのタスクは、ステータスを動かさない
- すでにそのブランチにいるなら、ブランチの操作はしない
- ブランチがあり別のブランチにいるなら、切り替えない。`git switch <ブランチ>` を案内し、担当者とステータスの更新だけ行う
- `hh/<ID>` か `hh/<ID>-` で始まるブランチは、そのタスクのブランチとして扱う。タイトルを変えても2本目を作らない。`<ID>` には `former_ids` も含み、大文字と小文字は区別しない。`hh pr` も同じ見方をする
- 担当者とステータスの更新は、1回の書き込みにする。`doing` への遷移が制約で拒否されたら、ブランチは残し、理由を出して終わる

### `hh pr`

- ブランチは、作業中のブランチが `hh/<ID>` か `hh/<ID>-` で始まるものならそれを使う。違えばローカルのブランチから同じ形のものを探す。なければ `hh start` を案内して止める
- push は fast-forward だけ行う。拒否されたら止める
- そのブランチに開いた PR があれば、作らずにリンクする。push の後で失敗しても、打ち直せば続きから進む
- push はコードの repo の `origin` へ、`repo_accounts` のアカウントで行う
- PR の向き先は repo の既定のブランチ。タイトルと本文はタスクから作り、本文の末尾に `hedgehug:<ULID>` を入れる
- `--draft` で draft にする
- 作った PR をリンクし、remote のある Store は同期してから、5分の間隔によらずその repo を取り込む。ここで `pr.opened` が発火する。同期が失敗したら取り込みを飛ばして警告する。先に同期するのは `hh sync` と同じ理由で、別のメンバーが取り込み済みの変化を自分のものとして扱わないためである

### `hh automation`

親設計の CLI 一覧に足す。GUI で編集するので、CLI と GUI を対称にする（4e の ProjectSettings と同じ考え方）。

| コマンド | 動き |
|---|---|
| `hh automation list [--project <id>] [--json]` | ルールを、`scoped` の状態と一緒に一覧する |
| `hh automation add --id <id> --on <event> (--status <s>\|--category <c>) [--when <w>] [--project <id>]` | ルールを足す |
| `hh automation rm <id>` | ルールを消す |
| `hh automation disable <id> [--project <id>]`・`enable` | `--project` がなければ `enabled` を、あれば `scoped` の `disable` を変える |
| `hh automation fields [--default-sync <mode>] [--push <a,b>] [--mirror <a,b>] [--prop <key=value>]` | `issue_fields` を出す。フラグがあれば変える |

### `hh mv --store`

今の `scoped` の「持っていく／捨てる」の選択に、`automations.yaml` の `where` 付きルールと `scoped` の `disable` も含める。`automations.yaml` にだけ対象があるときも選択を求める。元の Store からは、どちらを選んでも消す。移動先に合わないもの（ステータスやカテゴリがないルール、`id` が重なるルール、移動先にない `id` を指す `disable`）は外して一覧に出す。

## 5c Issue 同期

### モード

| モード | 向き | 反映するフィールド（既定） |
|---|---|---|
| `mirror` | GitHub → タスク | タイトル・状態・担当者 |
| `push` | タスク → GitHub | タイトル・本文・状態・担当者・ラベル |
| `link` | なし | `gh.issues[].state` の取り込みだけ |

反映するフィールドは、リンクの `fields`、`issue_fields` の順に見る。知らない名前は飛ばす。同じタスクに `mirror` のリンクが複数あるときは、フィールドごとに `links` の先頭のリンクが持つ。後のリンクはそのフィールドを見ない。

`hh link --sync` でモードを変えたら `synced` を消す。古い基準が手編集に見えるためである。

### 実行の順

取り込みの順番の 5 は次の順で動く。

1. 取り込みのトランザクションの中で `mirror` を比べ、ルールを評価する。ルールは `mirror` の後のステータスと値から評価する
2. タスクごとの `gh-mirror` のコミットに、フィールド、ステータス、`synced`、trailer をまとめる。author は `hedgehug-automation（実行したメンバー）`。ここまでストアのロックを持ち続ける
3. ルールの `set` のコミットを書く。`gh-mirror` に失敗したタスクは飛ばす
4. ロックを放し、`push` のリンクを比べて `PATCH` する。ルールが動かした後のステータスが同じ同期で届く
5. 書けたものをタスクごとの `gh-push` のコミットにする。`synced` と、書いた `state`・`state_reason` を `gh` に写す

`gh-mirror` が schema の検証に落ちたら警告にする。`synced` が進まないので、次の取得で比べ直す。

### アダプタ

タイトル・本文・状態・担当者・ラベル・マイルストーンのフィールドごとに、読み取り、差分、書き込み、名前の解決を持つ。

- マイルストーンは既定で無効。`issue_fields.push` に足し、`props` で対応先のキーを決めると有効になる
- 読み取りは 5a の GraphQL に乗せ、書き込みは REST で行う
- 名前の解決は、1回の同期の中だけキャッシュする
- `push` の本文は末尾に `hedgehug:<ULID>` を入れ、比べるときは除く

### 基準の値

タスクと Issue の値が違うだけでは、どちらが変わったのか分からない。`gh.issues[].synced` に、最後に揃えたときの値を持ち、3つを比べる。

| モード | GitHub が基準と違う | タスクが基準と違う |
|---|---|---|
| `mirror` | タスクへ反映する | 次の反映で GitHub の値に戻し、inbox に出す |
| `push` | タスクの値で上書きし、inbox に出す | GitHub へ反映する |

`synced` のキーは `title`・`body`・`status`・`assignees`・`labels`・`milestone`。`status` は `open`・`completed`・`not_planned`。比べる値は GitHub 側の表現に揃える。

- login とラベルは小文字にして整列して比べる。送るときは GitHub の表記にする
- 本文は印を除き、CRLF を LF にし、末尾の空白を落として比べる。`push` の本文に印がなければ、本文が同じでも書き直す
- 別の host の担当者は比べない。`mirror` は残し、`push` は除いて警告する
- `synced` にないフィールドは基準がない。モードの向きへ黙って揃え、inbox に出さない

手編集の上書きは、コミットの trailer `Hedgehug-Issue-Overwritten: <ULID> <field> <ref> <side> <JSON の値>` に書く。`side` は上書きされた側（`task`・`github`）である。inbox の種類は `issue_overwritten`、宛先は `automation` と同じ担当者とウォッチャーで、詳細は `{ref, field, side, lost}`。`Hedgehug-Overwritten` は負けた author 宛てで、ここには author がいないので使わない。

### 状態の対応

| モード | 対応 |
|---|---|
| `push` | `done` カテゴリ → closed（completed）、`canceled` カテゴリ → closed（not_planned）、それ以外 → open |
| `mirror` | closed（completed）→ `done` の既定、closed（not_planned）→ `canceled` の既定、open → `todo` の既定 |

`mirror` の open は、タスクが `todo` か `doing` のカテゴリにあるなら動かさない。同じカテゴリの既定でないステータスは既定に戻さない。`mirror` の変更は `ValidateTransition` を通し、拒否されたら自動化と同じ trailer と通知にする。rule は `mirror`、通った変更の trailer は `Hedgehug-Automation: mirror issue.mirror` とする。`issue-closed` のルールは `mirror` の反映の後に当てる。

拒否の通知は GitHub が基準から変わった回だけ出し、`synced` は GitHub の値へ進める。タスクが基準と違うまま拒否され続ける間は何も書かない。ワークフローが許すようになれば、次の反映で揃う。

ステータスを `mirror` するリンクで not_planned に閉じた Issue は、`issue.closed` を発火しない。`issue-closed` の `set: {category: done}` が `canceled` を `done` に変え、次の反映で手編集として戻るためである。ステータスを `mirror` しないリンクでは発火する。

### 失敗したとき

| 場面 | 動き |
|---|---|
| 存在しないラベル、割り当てできない人、ないマイルストーン | 除いて反映し、警告する。作らない。除いた値を `synced` に書くので、上書きの通知にならない。警告は同期のたびに出る |
| GitHub への書き込みの失敗 | 警告する。差分が残るので次の同期で再試行する |
| Issue が消えた | 警告する。リンクは残す |
| タスクを削除した | Issue は触らず、反映を止める |

### コマンド

- `hh issue <id> [--repo org/repo]`: `push` モードで Issue を作る。repo はタスクの `repos` から決める。複数あれば `--repo` を求める。なければカレントの repo を使う。`issue_fields.push` のフィールドを渡し、閉じる側のタスクなら作成の後に閉じる。閉じられなくてもリンクは書き、基準を `open` にして警告する（次の同期で閉じる。打ち直して Issue が二重にならない）。`synced` と `gh` を同じコミットに書くので、次の同期で差分が出ない。op は `issue`。その repo の使用中の Issue リンクがあれば `link.exists`
- `hh gh import <org/repo> --label <label> [--parent <id>] [--workspace <name>] [--dry-run]`: 開いている Issue を `mirror` のタスクにする。本文は最初の1回だけ写す。リンク済みの Issue は飛ばす（`off` を含む）。`--dry-run` は件数だけを返す。タスクごとに op `add` の1コミットにするので、途中で落ちても打ち直せる。終わったらその repo を取り込み、担当者と状態を揃える

## 5d の画面

| 画面 | 作るもの |
|---|---|
| ツリー・カンバン・テーブル・マイタスク | PR のチップ、ツリーの PR 列、テーブルの PR 列、カンバンの列の「PR を開くと自動でここへ」「全 PR のマージで自動でここへ」 |
| タスク詳細 | GitHub セクション（PR・Issue・リンク元・モード）、`リンク`、`リンクを外す`、モードの切り替え、履歴の自動化 |
| プロジェクト | 「リポジトリと PR」の表、`repo を追加`、取得時刻 |
| 通知 | 自動化タブ、「割り当て」「ウォッチ」タブ、拒否の通知（`points を入力して完了にする`・`このままにする`）、手編集の上書きの通知 |
| アクティビティ | 「自動化」の絞り込み |
| 自動化の設定 | ルールの一覧と編集、回数の集計（直近7日）、Issue の同期、Issue の取り込み、取得時刻とレート制限 |
| プロジェクトの設定 | ルールの停止、このプロジェクトだけのルールの追加 |
| アカウントの設定 | `repo_accounts`（ドラッグで順番を変える）、GitHub App の未インストールの警告、`gh のトークンで代わりに使う`、`管理者にインストールを依頼` |
| 状態の表示 | 「取得を 15 分おきに延ばしています」、空のプロジェクトの `Issue から取り込む` |
| コマンドパレット | `ブランチを作って始める（hh start <ID>）`（コピー） |

決まりは次のとおり。

- `$ hh start <ID>` と `PR を作る` は、実行せずコマンドをコピーする。GUI はカレントのリポジトリを持たず、ブランチを作れず push もできない
- チップは1つの状態を、merged → draft → CI 失敗 → 修正依頼 → approved → open の順で選ぶ。PR が複数あるタスクは、マージされていない PR を先に見せ、残りを `+n` で示す
- `review: none` と draft のレビューは `—` と出す
- チップの位置は Foundations の順（状態の警告 → PR → 優先度 → 期限 → 担当者）
- GitHub App の未インストールは、取得の 403 の応答（`Resource not accessible by integration`）で判定し、`ghstate` に残す
- ルールの回数は、history の索引から `Hedgehug-Automation` の trailer を数える
- アクティビティの拒否は、拒否を書いた取り込みのコミットを指して表示する

### キャンバスの修正

5d の前に、Hedgehug GUI デザインのキャンバスを次のとおり直す。

- Activity: 拒否の記録の「コミットなし」と sha の `—` を、取り込みのコミットを指す表示にする
- TaskDetail: `リンクを外す` とモードの切り替えを足す
- SettingsAutomations: Issue の同期の表に「mirror モードで同期」の列を足す

## テスト

- **マージ**: 今のプロパティテストに、`links`（`off` を含む）と `gh` のランダムな同時更新を足す。全クローンが同じ値に収束し、`gh` の衝突が inbox に出ないことを確かめる
- **ファイル形式**: `gh`・`Link.Fields`・`automations.yaml` のゴールデンテストで、読み書きでキー順と知らないキーが変わらないことを確かめる
- **`domain/automation`**: 表形式のテストで、イベントの判定、適用の順、`disable`、`when` を確かめる
- **`lib/github`**: 記録した API レスポンスで GraphQL の解釈を確かめる。fakegh の応答には同じ記録を流用する
- **usecase**: fakegh を相手に、取り込み、自動リンク、`repo_accounts`、レート制限、失敗の警告、`hh unlink` の後に自動リンクが戻らないこと、別のメンバーが書いた遷移で発火しないことを確かめる
- **CLI**: 新しいコマンドのゴールデンテスト。`cli/scenario` に、`hh start` から PR のマージまでの通しのテストを足す。PATH から git を外した CI ジョブでも通す
- **Issue 同期**: モードとフィールドの組み合わせを表形式で確かめ、手編集の上書きと inbox を確かめる
- **実 API**: 親設計の検証用 Org に対する定期実行のワークフローを 5a で足し、GraphQL のクエリが実際の GitHub の応答と合うことと、レート制限の値を確かめる。固定のリポジトリ `hh-live` に PR を1件以上置き、secret は `HH_LIVE_ORG`・`HH_LIVE_TOKEN` とする。Device Flow と GitHub App の権限は人の操作が要るので、5a の仕上げで検証用 Org を使って手で確かめ、結果を引き継ぎに書く
- **web**: vitest と Playwright の E2E。fakegh で状態を動かし、チップ・タスク詳細・通知が変わることを確かめる

## 親設計の修正

本 spec の承認と同時に、Hedgehug 設計ドキュメントを直す。

- 「GitHub から取り込む値」に `ref`・`review: none`・`state_reason`・`synced` を足す
- 「リンク」に `sync: off` と `hh unlink` の動きを足す
- 「取得のタイミングとレート制限」に取り込みの順番を足す。author の記述を取り込みのコミットにも広げる
- 「同期するフィールド」に `issue_fields` の形式を足す
- 「自動化ルール」に `enabled`、適用の順、タスクごとのコミット、拒否の trailer を足す
- 「inbox に届くもの」の自動化に、宛先（担当者とウォッチャー）と「自分の書き込みは通知しない」の例外を足す
- 「コマンド一覧」に `hh automation` と、`hh gh import` の `--parent`・`--dry-run`、`hh link` の `--sync`、`hh issue` の `--repo` を足す
- 「タスクから開発を始める」に、ブランチがすでにあるときの動きを足す
