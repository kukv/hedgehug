# トラック A5a(アイテムの別のワークスペース・別の Store への移動) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** アイテムのメニュー「別のワークスペースへ移動…」で、Epic・Story・Task・Bug を下のもの(削除済みを含む)ごと、同じ Store の別のワークスペースと別の Store のワークスペースへ仕様どおりに移し、転送記録を残して、旧 ID(移動前の ID と旧い接頭辞の ID)から画面のアドレスで辿れるようにする。

**Architecture:** 移動の規則(値の引き直し・番号の順・人の集め方・転送記録と `moved_in` の形)は新しい `internal/domain/transfer` に純粋な関数で置き、操作は新しい `internal/app/transfers` に置く(`app/items` は 19 ファイルで、移動は 1 ファイルに収まらないため)。同じ Store の中は 1 回の `Stores.Write`。別の Store は、移動先を新しい口 `Replicator.SyncSave`(1 つのロックの中で受け取り → 保存 → 送信。送れなければ保存を捨てる)で保存し、そのあと移動元を `Stores.Write` で保存する。トークン・受け取った位置・同期の状態は `app/syncing` が持つので、`syncing.Service.Deliver` を足し、`app/transfers` は自分の口 `Delivery` で受け取る(配線は `cmd/hedgehug`)。旧 ID の解決は `app/transfers.Locate` が、表示中のすべての Store の転送記録・旧い接頭辞・`purged.jsonl` を読んで決める。

**Tech Stack:** Go(huma・domain/app/infra/adapter の層、go-git v5)、TypeScript(core・api の orval)、Svelte 5(ui・app)、Vitest、Playwright。

**Spec:** `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md`(「別のワークスペースと別の Store への移動」の全節、「詳細パネル」の閲覧のみの無効と「上から順の構成」のメニュー、「履歴」の「{旧 ID} から移動」)、`01-overview.md`(「画面のアドレス」)、`03-workspace-settings.md`(「接頭辞」の旧い接頭辞でのアドレス、「自動化」の動かない操作)、`06-sync-notifications.md`(「別の Store へ移動したものと重なったとき」、ほかの書き手が移したときにパネルを閉じる)。保存形式は `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md`(「変更記録」の `moved_in`、「移動・番号・旧 ID」、「削除・復元・完全に削除」の `purged.jsonl`、「アクティビティと履歴」)。

**実装担当にタスクを渡すときは、タスクの本文と一緒に「Global Constraints」「持ち場と境目」「決めたこと」「タスク間の名前」の節を必ず渡す。** 食い違ったときは、この 4 節と仕様が本文より優先する。

**前提として読むもの:** トラック A の計画 `2026-11-12-hedgehug-track-a.md` の A5 の行、A4b の計画 `2026-11-13-hedgehug-track-a4b.md` の「持ち場と境目」「決めたこと」(A4b-3・A4b-4・A4b-11・A4b-16・A4b-22)と末尾の「実装で計画から変えたこと」、A2 の計画の決定 7(アドレスの解決)。

## Global Constraints

- 依存は追加しない(足すときは `go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` で版を明記し、公開から 7 日以上の版だけ。Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 大きさの上限: Go ファイル 300 行(テスト 600 行)・関数 60 行・認知的複雑度 15・パッケージ 20 ファイル(テストを除いて数える)、Svelte 200 行、TS 300 行。超えたら分ける。上限の 20 にいる `internal/infra/gitstore`・`internal/app/apptest` には新しいファイルを足さず既存のファイルに足す(行が溢れるなら、既存のファイルのまとまりを同じパッケージの中で付け替えるのではなく、Task の本文の指示どおり下位のパッケージに出す)。`internal/app/items` は 19 で、足さない。
- 向き: adapter→app→domain、infra→app/port。app の機能どうしは import しない(テストも)。`app/transfers` と `app/syncing` は互いを import せず、`cmd/hedgehug` の配線でつなぐ。
- 画面の文言は仕様の文字どおり。モックの動きは写さない。
- 通知は作らない。自動化は動かさない(03「自動化」: 「別のワークスペースへ移動…」では動かない)。
- 閲覧のみの Store のアイテムでは、メニューの「別のワークスペースへ移動…」を無効にし「閲覧のみの Store です」を添える。閲覧のみの Store は移動先の候補に出さない。
- 各タスクは TDD(失敗するテスト → 失敗を確かめる → 実装 → 通す → コミット)。タスクごとにレビューする。
- コミットの末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`。
- PR はトラックで 1 本(develop 向け、ブランチ `feat/track-a5a`)。force push は拒否されるので、載せ直すときは別名のブランチで出す。
- 計画やテストにトークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e のビルドも CI で lint する。
- 確認のコマンド: Go は `go test ./internal/... ./cmd/...`、全体は `make check`、画面は `pnpm -C web test`・`pnpm -C web check`、API の型は `make generate` のあと `pnpm -C web gen`、e2e は `pnpm -C web e2e`(`web/e2e/fixtures.ts` が worker ごとに空いた番号で `bin/hedgehug-e2e` を起動する)。手元の検証は 1 回で足りる。
- `api/openapi.yaml`・`web/packages/api/src/generated.ts` は手でマージしない。API を変えたタスクで `make generate` → `pnpm -C web gen` で作り直してコミットする。

## Review Focus

1. **送信の失敗で移動先に何も残らない**: 移動先に remote があり、受け取りは通って送信が断られた(書き込みの拒否・重なりの繰り返し・途中で切れた)とき、移動先の Store に移したアイテムのフォルダもコミットも残らず、移動元もそのまま。テストは Task 3(`SyncSave` の送信失敗)と Task 6(移動の全体)。
2. **向きの逆の移動が同時に起きても止まらない**: Store A → B の移動と B → A の移動を同時に流しても、ロックを入れ子に取らないので、どちらも終わる(片方が失敗してもよいが、止まらない)。テストは Task 6。
3. **何度も移したものの旧 ID**: A → B → C、A → B → A(番号は再利用しない)、同じものを 2 人が別々に移して記録が 2 件、のどれでも旧 ID がいまの場所に辿り着く。テストは Task 7。
4. **削除済みの子と、移すものの外との関連**: 削除済みの子が、削除済みのまま元の削除日時で移り、番号は同じ親の削除されていない子のあと。別の Store へ移すときは、外のアイテム(削除済みを含む)との関連が両方の側から消え、中の関連は残る。テストは Task 5・6。
5. **移動先にない人**: 担当者・ユーザー型の値・ウォッチ・作成者・コメントと作業時間の書き手・履歴の操作者のうち、移動先のメンバーにいない人(大文字小文字だけが違う人は同じ人)が、移動先の保存の中で 1 人 1 行の `join` として足される(アクティビティの「メンバー {ログイン名} を追加」は、この行から C4 の画面が作る。A5a-11)。テストは Task 1・6。

---

## 持ち場と境目

| 入れるもの | 仕様 |
|---|---|
| メニュー「別のワークスペースへ移動…」(詳細パネルとツリーの行。Epic・Story・Task・Bug。閲覧のみは無効) | 02「上から順の構成」、A4b-16 の順 |
| 移す先の候補と、候補がないときの表示 | 02「移す先の選び方」 |
| 移動のダイアログ(移動先のワークスペース・プロジェクト・親の Epic、冒頭の文、「移動先で変わること」、「ステータスの置き換え」、「移動」のボタン) | 02「移す先の選び方」 |
| 同じ Store の別のワークスペースへの移動(1 回の保存、番号の振り直し、値の引き直し、転送記録) | 02「移すもの」「値の引き直し」「保存」、保存形式「移動・番号・旧 ID」 |
| 別の Store への移動(移動先の受け取り・保存・送信 → 移動元の保存、失敗で元のまま、`moved_in`、関連の解除、メンバーの追加) | 02「関連・転送記録・旧 ID」「保存」 |
| 転送記録 `transfers.jsonl` の読み書き、`purged.jsonl` の読み取り | 保存形式「移動・番号・旧 ID」「削除・復元・完全に削除」 |
| 旧 ID の解決: 画面のアドレス(移動前の ID・旧い接頭辞の ID)、見つからないときのトースト 4 種、旧 ID から開いたときの「{旧 ID} は {日時} に {新 ID} へ移動しました」 | 02「旧 ID から辿れる場所」「見つからないとき」「移した日時の表示」、01「画面のアドレス」、03「接頭辞」 |
| 履歴の「{旧 ID} から移動」(移動元の Store・ワークスペース・プロジェクト・日時)と、`moved_in` の `names` による値の名前 | 02「関連・転送記録・旧 ID」、保存形式「アクティビティと履歴」 |
| アクティビティ「{旧 ID} を {移動元} から {移動先} へ移動」「メンバー {ログイン名} を追加」、トースト「{旧 ID} は {新 ID} になりました」(空になった項目つき) | 02「保存」「値の引き直し」 |
| 移したあと、詳細パネルを新しい ID のアイテムに切り替える。ほかの書き手が移したときはパネルを閉じる | 02「保存」、06 |

| 入れないもの | 回す先 | 理由 |
|---|---|---|
| プロジェクトの移動(設定タブの「別のワークスペースへ移動」の区画) | A5b | ユーザーの決定(2026-11-14、A5 を 3 本に分ける)。A5b は A5a の `domain/transfer`・口・`Delivery`・`Locate` を使う |
| コマンドパレット(「別のワークスペースへ移動…」の候補、旧 ID の候補と「旧 ID・{日付} に移動」の添え書き) | A5c | 同上。A5c は `Locate` と `transfers` の API を使う |
| 移動先のプロジェクトが使わないステータス(「ステータスの置き換え」の対象のうち、名前は移動先にあるがプロジェクトが使わないもの) | B5 | プロジェクト別のルールは B5。A5a は「移動先のプロジェクトが使うステータス」を `transfer.UsableStatuses` 1 か所で決め、いまはワークスペースのステータスすべてを返す。B5 がこの関数だけを差し替える(A4b の「入れないもの」と同じ扱い) |
| スプリントとリリースを外す(ダイアログの「スプリントとリリースは外す」の行は出す) | F | アイテムにスプリント・リリースの項目がまだない(A4b の「入れないもの」)。F が項目を足すときに、`transfer` の引き直しで外す行を足す |
| 同期の修復(5)のワークスペースの追従(「ダイアログを出せない経路」) | C3 | 同期の修復は C3。C3 は `transfer.Remap`(置き換えを渡さないと同じ分類の既定にする)と `transfer.Number` を使う |
| 検索(自由テキスト・`parent:`・`project:`・関連のキー)での旧 ID | E | 検索は E。E は `Locate` と同じ規則で旧 ID を引く(`transfers.OldIDs` を使う) |
| アクティビティの画面と受信箱の旧 ID のリンク、移動先の保存のメンバーの `join` の行から「メンバー {ログイン名} を追加」を 1 人 1 行で出すこと(A5a-11) | アクティビティの画面と受信箱を作るトラック(C4) | 画面がまだない。リンクを開くときは、A5a の画面のアドレスの解決(`locate-item`)に旧 ID を渡せば辿れる |
| 別の Store へ移したものと、移動元で他の人の変更が重なったときの合わせ方と通知の文 | C2(合わせ方は入っている: `underTransferred` とフォルダの削除が勝つ)、C4(通知の文) | |
| 「完全に削除」の書き込み(`purged.jsonl` に足す) | G | A5a は読むだけ |
| 旧 ID の削除済みのトーストの「設定の「削除済み」から復元できます」の行き先 | G | 文だけを出す(リンクなし。仕様のとおり) |

## 決めたこと

| # | 決定 | 根拠 |
|---|---|---|
| A5a-1 | 転送記録の 1 行は `transfer.Record{ID, Op change.ID; At time.Time; By string; Entity string; Target change.ID; Old, New string; ToStore, ToStoreName, ToRemote string}` で、JSON は保存形式の例のキー(`id`・`op`・`at`・`by`・`target`・`old`・`new`・`to_store`・`to_store_name`・`to_remote`)に `entity`(`item`・`project`・`workspace`)を足したもの。同じ Store の別のワークスペースへの移動は `to_store` に自分の Store ID を書き、`to_store_name`・`to_remote` は空(C2 の `transferTargets` は `to_store` が自分なら数えない)。`id`・`op`・`at`・`by` は `Tx` が付ける | 保存形式「移動・番号・旧 ID」、`sync_tree_more.go` の `transferTargets` |
| A5a-2 | `moved_in` の行は、アイテムごとに 1 行、写した行のあとに同じ操作で書く。`f` は空、`to` に `{"old":"HH-12","from_store":"<Store ID>","from_store_name":"…","from_workspace":"<内部 ID>","from_workspace_name":"…","from_project":"<内部 ID>","from_project_title":"…","at":"…","names":{"<値・プロパティの内部 ID>":"名前"}}`。`names` は写した行の `from`・`to` に現れる、移動元のワークスペースの値とプロパティの内部 ID すべて | 保存形式「変更記録」の `moved_in`、「アクティビティと履歴」 |
| A5a-3 | 同じ Store の中の移動は、移すアイテムそれぞれに `set place`(ワークスペース・プロジェクト・親・新しい番号)を書き、移動先のワークスペースに `number` の行を番号ごとに書き、引き直した値を `set`(`add`・`remove`)で書き、`transfers.jsonl` にアイテムごとに 1 行足す。すべて 1 回の `Write` | 保存形式「移動・番号・旧 ID」の「別のワークスペース(同じ Store)」 |
| A5a-4 | 別の Store への移動は、移動先に、移すアイテムのフォルダ(`log.jsonl` の全行をそのまま・`description.md`)を `ItemWriter.Import` で写し、続けて同じ操作で `moved_in`・`set place`・引き直しの `set`・`number` を書く。移動元は `ItemWriter.Remove` でフォルダを消し、外の相手のアイテムに関連の `remove` を書き、`transfers.jsonl` に 1 行ずつ足す | 保存形式「移動・番号・旧 ID」の「別の Store」 |
| A5a-5 | 新しい口 `Replicator.SyncSave(ctx, storeID, remote, op, fn) (SyncResult, error)`: 1 つのロックの中で、受け取って合わせ、`fn` で保存し、送る。送れなければ(何度試しても重なる場合も)、保存のコミットだけを捨てて、受け取って合わせた結果は残す。受け取りで失敗したら保存しない。`app/syncing` に `Service.Deliver(ctx, storeID, op, fn) (syncstatus.Reason, error)` を足し(理由が空なら保存できた)、remote がなければ `Stores.Write` だけ、あれば `SyncSave` を呼び、トークン・受け取った位置・同期の状態の記録を `run` と同じに扱う。失敗の理由は `syncstatus.Reason` で返し、error は理由で表せない失敗(このマシンの状態を読み書きできないなど)だけ。`app/transfers` の口 `Delivery` は同じ形なので、`syncing.Service` をそのまま渡せる | 02「保存」の (1)〜(3) と「どれかが失敗したら、何も変えずに元のままにする」。`Write` と `Sync` を続けて呼ぶと、あいだでロックが外れ、送れなかった保存を取り消せない |
| A5a-6 | 別の Store への移動の順は、(0) 移動元を読んで移すものを決める(ロックは読むあいだだけ)、(1)〜(3) 移動先を `Delivery.Deliver`、(4) 移動元を `Stores.Write`。ロックを入れ子に取らない。(4) が失敗したら、移動先に、写したフォルダを `Remove` する保存を `Deliver` で書いて戻し(アクティビティの文は「{新 ID} の移動を取り消し」)、エラーを返す | 02「保存」。向きの逆の 2 つの移動が互いを待たないため(Review Focus 2) |
| A5a-7 | 値の引き直しは `transfer.Remap(it item.Item, from, to workspace.Settings, people PeopleMap, statusChoice map[change.ID]change.ID) RemapResult` の 1 か所。仕様の表どおり(ステータスは名前の完全一致で `UsableStatuses` にあるもの、なければ `statusChoice`、それもなければ同じ分類の移動先の既定。優先度・関連システム・単一選択・複数選択は名前の完全一致、優先度が空になれば移動先の初期優先度。テキスト・数値・日付・チェック・URL はキーと型が同じなら残す。担当者・ユーザー型・ウォッチは同じ Store ならそのまま、別の Store なら `people`)。`RemapResult` は書く行と、空になった項目の名前(トーストの順はプロパティの定義の順。優先度を初期優先度にしたら「優先度」) | 02「値の引き直し」 |
| A5a-8 | 「移動先のプロジェクトが使うステータス」は `transfer.UsableStatuses(to workspace.Settings, p project.ID) []change.ID`。いまはワークスペースのステータスすべて(B5 が差し替える) | 持ち場と境目 |
| A5a-9 | 番号は `transfer.Number(root item.Item, below []item.Item, next int) map[item.ID]int`: 移すアイテムが `next`、続いて子・孫を並び順に深さ優先、削除済みの子は同じ親の下の削除されていない子のあと(削除済みどうしは並び順) | 02「移すもの」 |
| A5a-10 | 移動先のメンバーに足す人は `transfer.People(items, logs, members) []member.Member`(担当者・ユーザー型の値・ウォッチ・作成者・コメントと作業時間の書き手・行の `by`。`hedgehug[bot]` を除く。ログイン名は大文字小文字を区別せずに比べ、移動先にいる人は移動先の書き方に置き換える。足す人の表示名とログイン名は移動元のメンバーと同じ)。足す行は既存の `member.Join` | 02「保存」のメンバー、「値の引き直し」の担当者 |
| A5a-11 | アクティビティの文(保存の Summary): 移動の操作は「{旧 ID} を {移動元} から {移動先} へ移動」。{移動元}・{移動先} はワークスペース名で、別の Store のときは「{Store 名} の {ワークスペース名}」。移動先・移動元の両方の保存に同じ文。メンバーの追加(`member.Join`)は移動先の同じ保存に入れる(別の保存にすると、移動が失敗したときにメンバーだけが残り「元のまま」にならない)。「メンバー {ログイン名} を追加」の行は、アクティビティの画面が、その保存のメンバーの `join` の行から 1 人 1 行で作る(アクティビティの画面を作るトラックへ。持ち場と境目) | 02「保存」。「どれかが失敗したら、何も変えずに元のままにする」を、アクティビティの 1 コミット 1 行より優先する |
| A5a-12 | トースト: 「{旧 ID} は {新 ID} になりました」、空になった項目があれば「。合う値がなく空になった項目: A・B」を続ける。失敗は「{移動先の Store 名} に送信できないため移動できません: {理由}」(理由は 06 の短い理由。`syncstatus` の文を使う)、オフラインは「オフラインのため {移動先の Store 名} へ移動できません」、それ以外の失敗(断りでない保存の失敗)は「移動できませんでした」(仮。A4b の失敗のトーストと同じ扱い)。オフラインかは画面が決める: 移動先に remote があり、ブラウザがオフライン(`navigator.onLine === false`)なら、送らずにオフラインの文を出す。サーバーが `unreachable` を返したときは「…送信できないため移動できません: remote に接続できません」 | 02「保存」。サーバーはこのマシンにあるので、オフラインを知るのはブラウザ |
| A5a-13 | 移す先の候補は、表示中の Store のうち閲覧のみでないもの(`StoreList`)の、サイドバーで非表示でなく、削除済みでないプロジェクトが 1 つ以上あるワークスペース(自分のワークスペースを除く)。並びは Store の並び → ワークスペースの並び。候補がなければダイアログを開かずトースト「移せるワークスペースがありません(プロジェクトのある、書き込めるワークスペースが要ります)」 | 02「移す先の選び方」。ワークスペースの非表示は B の持ち場で、`StoreList`・カタログの既存の値を使う(Task 4 が確かめる) |
| A5a-14 | ダイアログの予告はサーバーの `GET items/{item}/transfer-preview?toStore=&toWorkspace=&toProject=` が返す: 下の削除済みでない件数 N・削除済みの件数 M・移動先の次の番号の ID(「HH-13 からの番号に振り直されます」の HH-13)・別の Store か・remote の注意を出すか・ステータスの置き換えの行(`{status, name, count, choices[], default}`)。行の並びは移動元のワークスペースのステータスの並び | 02「移す先の選び方」。下の孫・削除済みは画面が読んでいない |
| A5a-15 | 旧 ID の解決は `transfers.Service.Locate(ctx, storeName, display string) (Located, error)`。順: (a) いまの ID で見つかる(Store 名の Store → サイドバーの順)ならそれ。(b) 旧い接頭辞で見つかる(いまの接頭辞のワークスペースが優先。03)。(c) 表示中のすべての Store の転送記録から `old` が一致する行を集め、`at` の新しい順に、`target` の内部 ID を表示中のすべての Store から探す。見つかれば開く(旧 ID から開いた印と、最初に一致した行の日時・新 ID を添える)。見つからなければ、削除済み → 完全に削除(`purged.jsonl` に `target`)→ 移動先の Store が表示中でない → いまの場所が記録の移動先と違う、の順で文を選ぶ。どれでもなければ見つからない(ホームへ) | 02「関連・転送記録・旧 ID」、01「画面のアドレス」、03「接頭辞」 |
| A5a-16 | 旧 ID から開いたときの「{旧 ID} は {日時} に {新 ID} へ移動しました」は、詳細パネルの上部(タイトルの上)に 1 行で出し、パネルを閉じるか別のアイテムを開けば消える。普通に開いたときは出さない | 02「移した日時の表示」。場所は仕様にないので、プロジェクトの概要の上部(02「名前の変更・移動・削除」)に合わせた |
| A5a-17 | 履歴の「{旧 ID} から移動」は、`moved_in` の行(別の Store)と、転送記録のうち `target` がこのアイテムの行(同じ Store)から作る。文は「{旧 ID} から移動({移動元の Store 名} / {ワークスペース名} / {プロジェクト名})」で、時刻は行の `at`(01 の書き方)。同じ Store の中の移動の `set place` の行は「親を変更」に出さない | 02「関連・転送記録・旧 ID」、A4b の「親を変更」の直し |
| A5a-18 | 自分で移したとき、詳細パネルを開いていれば新しいアイテム(新しい Store・内部 ID)に切り替える。ほかの書き手(同期・別のタブ・別のプログラム)が移したときは、パネルの読み直しで、内部 ID が同じでもワークスペースが変わっていれば閉じ、404 なら閉じる(A4b-22) | 02「保存」、06 |
| A5a-19 | API(`/api/…`): `GET stores/{store}/items/{item}/transfer-targets`(候補)、`GET stores/{store}/items/{item}/transfer-preview`、`POST stores/{store}/items/{item}/transfer`(本文 `{toStore, toWorkspace, toProject, toParent?, statusChoices: {from: to}}` → `{toStore, itemId, display, emptied: string[]}`)、`GET locate?store=&id=`(→ `{kind: "found", storeId, projectId, itemId, movedFrom?: {old, at, display}}` か `{kind: "gone", message}` か 404)。失敗の理由は既存の `apierr` の形で返す | |
| A5a-20 | e2e のビルドに、書き込める 3 つ目の Store `team`(remote なし、サンプルデータ、ワークスペース「Team」接頭辞 `TM`)と、`e2e` の Store に 2 つ目のワークスペース「Mobile」(接頭辞 `MB`、プロジェクト 1 つ、Epic 1 つ)を足す。remote ありの経路は Go のテスト(偽の `Replicator`)で確かめる | 通しのテストで、同じ Store と別の Store の両方を流すため |

---

## ファイルの地図

| 場所 | 新規・変更 | 役割 |
|---|---|---|
| `internal/domain/transfer/` | 新規 | `Record`・`MovedIn`(行の形と読み書き)、`Remap`・`UsableStatuses`、`Number`、`People`、`Collect`(移すものと外との関連) |
| `internal/domain/change/` | 変更 | `MovedIn` の種類(`t: moved_in`)の定数 |
| `internal/domain/history/` | 変更 | 「{旧 ID} から移動」、`moved_in` の `names` で値の名前を引く |
| `internal/app/port/` | 変更 | `Reader.Transfers()`・`Reader.Purged()`、`Tx.WriteTransfers()`、`ItemWriter.Import`・`Remove`、`Replicator.SyncSave` |
| `internal/infra/gitstore/` | 変更 | 上の口の実装(新しいファイルを足さない) |
| `internal/app/apptest/` | 変更 | 偽物と照合テスト、`Replicator.SyncSave` の偽物 |
| `internal/app/syncing/` | 変更 | `Deliver` |
| `internal/app/transfers/` | 新規 | `Service`・`Delivery`、`Targets`・`Preview`・`Move`・`Locate` |
| `internal/adapter/httpapi/transfers/` | 新規 | API |
| `cmd/hedgehug/` | 変更 | 配線、e2e のビルドの Store とサンプル |
| `web/packages/core/src/port/transfers.ts`・`web/packages/api/src/transfers.ts` | 新規 | 口と api の実装 |
| `web/packages/core/src/features/transfers/` | 新規 | ダイアログのモデル(候補・選び直し・置き換えの行・文)、旧 ID の解決 |
| `web/packages/core/src/features/items/menu.ts` | 変更 | 「別のワークスペースへ移動…」 |
| `web/packages/core/src/features/router/resolve.ts` | 変更 | `locate` に渡す |
| `web/app/src/screens/item/TransferDialog.svelte` ほか | 新規・変更 | ダイアログ、パネルの上部の 1 行 |

## タスクの一覧

| # | タスク | 待つもの |
|---|---|---|
| 1 | `domain/transfer`: `Record`・`MovedIn`・`Number`・`People`・`Collect` | — |
| 2 | `domain/transfer`: `Remap`・`UsableStatuses` | 1 |
| 3 | 口と保存: `Transfers`・`Purged`・`WriteTransfers`・`Import`・`Remove`(gitstore・偽物・照合テスト) | 1 |
| 4 | 口と同期: `Replicator.SyncSave`(gitstore・偽物)と `syncing.Service.Deliver` | 3 |
| 5 | `app/transfers`: 候補・予告・同じ Store の中の移動 | 2・3 |
| 6 | `app/transfers`: 別の Store への移動(`Delivery`、メンバー、関連の解除、戻し) | 4・5 |
| 7 | `app/transfers`: `Locate` と履歴の「{旧 ID} から移動」 | 3・5 |
| 8 | API と配線(`httpapi/transfers`、`cmd/hedgehug`、e2e のビルドの Store とサンプル) | 5〜7 |
| 9 | 画面の口とダイアログのモデル(core) | 8 |
| 10 | ダイアログ・メニュー・トースト・パネルの切り替え(app) | 9 |
| 11 | 画面のアドレスの旧 ID の解決と、パネルの上部の 1 行 | 9 |
| 12 | 通しのテスト(e2e)と PR | すべて |

## タスク間の名前

### Go

| 名前 | 置き場所 | 形 |
|---|---|---|
| `change.MovedIn` | `internal/domain/change/record.go` | `const MovedIn Type = "moved_in"` |
| `transfer.Record` | `internal/domain/transfer/record.go` | A5a-1。`func ParseRecords(b []byte) []Record`(読めない行は飛ばす)、`func (r Record) JSON() []byte` |
| `transfer.MovedInInfo`・`transfer.NewMovedIn`・`transfer.MovedInOf` | `internal/domain/transfer/movedin.go` | `type MovedInInfo struct { Old, FromStore, FromStoreName string; FromWorkspace change.ID; FromWorkspaceName string; FromProject change.ID; FromProjectTitle string; At time.Time; Names map[change.ID]string }`、`func NewMovedIn(m MovedInInfo) change.Draft`、`func MovedInOf(log []change.Record) []MovedInInfo`(書いた順) |
| `transfer.Number` | `internal/domain/transfer/number.go` | A5a-9 |
| `transfer.Set`・`transfer.Collect` | `internal/domain/transfer/collect.go` | `type Set struct { Root item.Item; Below []item.Item; Outside []Cut }`、`type Cut struct { From, To item.ID; Type string }`(外の相手との関連。両方の側)。`func Collect(root item.ID, all []item.Item) (Set, error)` |
| `transfer.People` | `internal/domain/transfer/people.go` | A5a-10。`type PeopleMap map[string]string`(移動元のログイン名の小文字 → 移動先のログイン名) |
| `transfer.Remap`・`transfer.RemapResult`・`transfer.UsableStatuses` | `internal/domain/transfer/remap.go` | A5a-7・A5a-8。`type RemapResult struct { Drafts []change.Draft; Emptied []string }` |
| `port.TransferReader`・`port.TransferWriter`・`port.PurgedReader`・`port.Purged` | `internal/app/port/stores.go` | `Reader.Transfers() TransferReader { List() ([]transfer.Record, error) }`、`Reader.Purged() PurgedReader { List() ([]Purged, error) }`(`type Purged struct { Entity string; Target change.ID; Display string; At time.Time }`)、`Tx.WriteTransfers() TransferWriter { Append(r transfer.Record) error }`(`ID`・`Op`・`At`・`By` は Tx が埋める) |
| `ItemWriter.Import`・`ItemWriter.Remove` | 同上 | `Import(id item.ID, log []change.Record, description string) error`(フォルダがあれば `ErrInvalidOperation`。行はそのまま書き、`*.yaml` は作り直す)、`Remove(id item.ID) error`(フォルダを消す。なければ `ErrNotFound`) |
| `port.Replicator.SyncSave` | `internal/app/port/replicator.go` | `SyncSave(ctx, storeID StoreID, remote Remote, op Operation, fn func(tx Tx) error) (SyncResult, error)` |
| `(syncing.Service) Deliver` | `internal/app/syncing/deliver.go` | `Deliver(ctx, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) (syncstatus.Reason, error)`(A5a-5) |
| `transfers.Service`・`transfers.Delivery` | `internal/app/transfers/service.go` | `type Delivery interface { Deliver(ctx, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) (syncstatus.Reason, error) }`。`Service{ Stores port.Stores; StoreList port.StoreList; Delivery Delivery; Clock port.Clock; IDs port.IDs; Me func(ctx) (string, error) }`(既存の機能の Service に合わせて Task 5 が決める) |
| `transfers.Target`・`(Service) Targets` | `internal/app/transfers/targets.go` | `type Target struct { Store port.StoreInfo; Workspace workspace.Workspace; Projects []project.Project; Epics map[project.ID][]item.Item }`、`Targets(ctx, store, id) ([]Target, error)` |
| `transfers.Preview`・`(Service) Preview` | `internal/app/transfers/preview.go` | A5a-14 |
| `transfers.MoveInput`・`transfers.Moved`・`(Service) Move` | `internal/app/transfers/move.go`(同じ Store)・`move_store.go`(別の Store) | `type MoveInput struct { ToStore port.StoreID; ToWorkspace workspace.ID; ToProject project.ID; ToParent *item.ID; StatusChoices map[change.ID]change.ID }`、`type Moved struct { ToStore port.StoreID; Item item.ID; Display string; Emptied []string }`、送れなかったときは `*transfers.SendError{Store string; Reason syncstatus.Reason}`(`errors.As` で取る) |
| `transfers.Located`・`(Service) Locate`・`transfers.OldIDs` | `internal/app/transfers/locate.go` | A5a-15。`type Located struct { Found bool; Store port.StoreID; Project project.ID; Item item.ID; MovedFrom *MovedFrom; Message string }`、`type MovedFrom struct { Old string; At time.Time; Display string }` |

### HTTP API

| operationId | メソッドとパス | 本文 → 応答 | タスク |
|---|---|---|---|
| `list-transfer-targets` | GET `/api/stores/{store}/items/{item}/transfer-targets` | → `{storeId, storeName, workspaceId, workspaceName, prefix, projects: {id, title, epics: {id, display, title}[]}[]}[]` | 8 |
| `get-transfer-preview` | GET `/api/stores/{store}/items/{item}/transfer-preview` | query `toStore, toWorkspace, toProject` → A5a-14 | 8 |
| `transfer-item` | POST `/api/stores/{store}/items/{item}/transfer` | A5a-19 | 8 |
| `locate-item` | GET `/api/locate` | query `store, id` → A5a-19 | 8 |

### 画面

| 名前 | 置き場所 | 形 |
|---|---|---|
| `TransfersPort` | `web/packages/core/src/port/transfers.ts` | `targets(storeId, itemId)`・`preview(storeId, itemId, to)`・`transfer(storeId, itemId, input)`・`locate(storeName, id)`。api の実装 `web/packages/api/src/transfers.ts`、偽物 `web/packages/core/src/features/transfers/transfers.test-util.ts` |
| `createTransferDialog` | `web/packages/core/src/features/transfers/dialog.ts` | Task 9 が決める(候補・選んだ値・予告・置き換えの選択・文・送信) |
| `itemTransfer` | `web/app/src/wireActions.ts` | `itemTransfer.open(item)` |
| `ItemMenuId` の `"transfer"` | `web/packages/core/src/features/items/menu.ts` | 「移動…」の次、「種類を変更…」の前 |

---

## 本文より優先する直し(3 人の書き手の指摘をまとめたもの)

この節は、上の「決めたこと」「タスク間の名前」と、各タスクの本文より優先する。指摘の原文は末尾の「付録」にある。

**決めたことの直し**

- A5a-6 の戻し: 移動元の保存が失敗したときの移動先の戻しは、写したフォルダの `Remove` に加えて、その移動で足したメンバーを `change.NewRemove("members", login)` で外す(`member.Join` は `members` への `add`)。`number` の行は残す(番号は再利用しないので害がない)。**Task 6**。
- A5a-6 の補足: 移動先の保存と移動元の保存のあいだでプロセスが落ちたときは、両方の Store に同じ内部 ID のアイテムが残る。保存を 2 つの Store にまたいで 1 つにする仕組みは持たない(仕様は Store ごとに 1 回の保存と決めている)。この状態では `Import` が同じ内部 ID を断るので移し直せない。利用者はどちらかを削除して直す。プロセスが落ちる場合だけの端の場面なので、これ以上は扱わない。
- A5a-5 の補足: `SyncSave` は送信が重なると受け取り直して `fn` を呼び直す(最大 3 回)。**Task 6 の `fn` は、番号・メンバー・値の引き直しを、毎回 `fn` に渡された `Tx` から読んで決める**(前の試みで読んだ値を使わない)。
- A5a-5 の `syncing.Service` には保存の口 `Stores port.Stores` を足す。配線では、ほかの機能と同じく `NotifyWrites` で包んだ `Stores` を渡す(remote のない移動先の保存も画面に知らせるため)。`SyncSave` の経路は `Deliver` が `EventSync` を知らせる。`Deliver` は閲覧のみの Store を `port.ErrInvalidOperation` で断り、成功したらミラーへ送る。`AfterSuccess` は呼ばない。**Task 4・8**。
- A5a-5 の補足: `fn` の失敗と送信の失敗を分けるため、`port.SaveError`(`fn` の失敗を包む型)を足す。`Deliver` は `SaveError` なら中のエラーをそのまま返し、理由にしない。**Task 4**。
- A5a-7: `Remap` に移動先のプロジェクトを渡す。形は `Remap(it item.Item, from, to workspace.Settings, toProject project.ID, people PeopleMap, statusChoice map[change.ID]change.ID) RemapResult`。`people == nil` は同じ Store(人はそのまま)。予告の「ステータスの置き換え」の行と `Remap` が同じ規則を使うよう、`transfer.Unmatched(statuses []change.ID, from, to workspace.Settings, toProject project.ID) []change.ID`(移動先で使えないステータス)を Task 2 が作り、Task 5 の `preview.go` も使う。**Task 2・5**。
- A5a-10: `People` の形は Task 1 の本文のもの `People(from workspace.Settings, items []item.Item, logs map[item.ID][]change.Record, fromMembers, toMembers []member.Member) ([]member.Member, PeopleMap)` にする(足す人と、移動元のログイン名から移動先の書き方への対応の両方を返す)。Task 6 は自分で `PeopleMap` を作らず、これを使う。**Task 1・6**。
- A5a-13: ワークスペースの非表示はなく、非表示は Store 単位(`StoreList` が除く)。ワークスペースの並びは `workspace.RankOf(ログ)` の昇順(同じなら内部 ID の順)。
- A5a-15・A5a-19 の見つからないとき: 文はサーバーで作らず、画面が作る(日時の書き方を Go と TS に 2 つ持たないため。A5c のパレットの「旧 ID・{日付} に移動」も TS で作る)。`Located` は `Gone *Gone`(`type Gone struct { Reason string; Old string; At time.Time; NewDisplay string; StoreName string; NowDisplay string }`、`Reason` は `deleted`・`purged`・`hidden`・`elsewhere`)を持ち、`Message` は持たない。API は `{kind: "gone", reason, old, at, newDisplay, storeName?, nowDisplay?}`。画面の `formatDateTime`(Task 11)で 4 つの文を作る。`whenText`(Task 7 の本文)は作らない。4 つ目の文(`elsewhere`)の当てはめは Task 7 の本文のとおり(より新しい記録の移動先の Store が表示中でない)。**Task 7・8・9・11**。
- A5a-15 の `MovedFrom`: `Display` は開くアイテムのいまの ID、`At` は旧 ID が一致した記録の日時。A → B → C で A の旧 ID から開くと「HH-2 は {A→B の日時} に {C のいまの ID} へ移動しました」。**Task 7・11**。
- A5a-16 に足す: 画面のアドレスの、旧い接頭辞でのワークスペース(`/s/{store}/w/{旧い接頭辞}`)とプロジェクト(`{旧い接頭辞}-P{番号}`)も、そのワークスペース・プロジェクトを開く(03「接頭辞」。いまの接頭辞が優先)。A2 の計画が「旧い接頭辞は A5 で一緒に足す」としていたもの。旧い接頭辞はカタログのワークスペースに無ければ足す(`get-workspace` の `oldPrefixes` と同じ値)。転送記録でのプロジェクトの旧 ID は A5b(プロジェクトの移動)。**Task 11**。
- A5a-17: 同じ Store の移動の履歴は、`set place` の行(`from` に移動元の内部 ID)と、同じ `op` の転送記録の `old` から作る。`history.Context` に `Origin` を、`apphistory.Service` に `StoreList` を足す(Task 7 の本文のとおり)。
- 移動元が閲覧のみなら `refusal.New("閲覧のみの Store です")`、Step と選べない移動先は `port.ErrNotFound`(Task 5 の本文のとおり)。Step のメニューには「別のワークスペースへ移動…」を出さない(仕様に Step の無効の理由の文言がなく、Step は単独で移せない)。
- 移動の送信が失敗したときは、ダイアログを開いたままにしてトーストを出す(選び直して再び押せるように)。`NotFound` のときだけ閉じる。**Task 10**。
- 仕様にない文言(ダイアログのタイトル・項目名・「移動先で変わること」の各行・ワークスペースの選択肢の書き方・「移動できませんでした」・「{新 ID} の移動を取り消し」)は、Task 12 で仕様 02 に書き足し、PR の本文に一覧で出す。

**名前の直し**

| もの | 作るタスク | あとのタスク |
|---|---|---|
| `transfers.Service` の欄 | 5 | `Stores port.Stores`・`StoreList port.StoreList`・`Delivery Delivery`・`Identity port.Identity`・`Targets port.SyncTargets`(移動先の remote の有無と URL)・`Clock port.Clock`。`IDs` は持たない |
| 予告の応答のキー | 8 | 画面(Task 9)の名前に揃える: `{below, deleted, nextDisplay, crossStore, remoteNotice, destinationHasRemote, replacements: [{status, name, count, choices: [{id, name}], default}]}`。`destinationHasRemote` はオフラインの判断(A5a-12)に使う |
| 移動の応答 | 8 | `{toStore, itemId, display, emptied}`(200)。送れなかったときは 422 で `detail` に「{移動先の Store 名} に送信できないため移動できません: {理由}」の全文 |
| 別の Store へ移したあとの内部 ID | 6 | `Import` は内部 ID を変えない。`Moved.Item` は元の内部 ID。e2e のサンプルの `SampleMobile`・`SampleTeam` の内部 ID は `Sample` と分ける |
| 偽物の `(*apptest.Stores).AddPurged(id port.StoreID, p port.Purged)` | 3 | 7 |
| `apptest.Replicator` の `Stores`・`SendErrs`、`ReplicatorCall.Save` | 4 | 6 |
| `internal/app/apptest/reentry`(`stores.go` から出す下位のパッケージ) | 3 | |
| `transfer.Set.Contains`・`transfer.ErrStep`・`transfer.Entity*` | 1 | 5・6 |
| e2e のサンプル: Store 名 `team`、Mobile の Epic は `MB-1` | 8 | 12(ずれたら Task 12 の Step 1 で本文の値を直す) |

**待つもの**: Task 4 は Task 3 のあと(どちらも gitstore と apptest を触る)。Task 2 は Task 3 と並べてよい。

**ファイルの地図に足すもの**: `internal/app/apptest/reentry/`、`internal/app/history`(`StoreList`)、`web/packages/core/src/model/transfer.ts`、`features/transfers/` の `dialogText.ts`・`afterMoved.ts`・`movedNotice.ts`(名前は本文のとおり)、`features/dates` の `formatDateTime`、`features/items/detail.ts`・`features/screen/screen.ts`、`web/app/src/wire.ts`。

**計画のコードの確かめ具合**: 本文のテストとコードは下書きで、どれもコンパイル・実行していない。各タスクの「失敗を確かめる」の期待は実行せずに書いた。この節と仕様が正。

---

> 骨組み `2026-11-14-hedgehug-track-a5a.md` の「タスクの一覧」の Task 1〜4 の本文。骨組みに差し込む。骨組みと違う形にした所は、末尾の「骨組みへの指摘」に理由を書いた(本文は、その直した形で書いてある所と、骨組みのままの所がある。直した所には本文の中に「(指摘 n)」と付けた)。

---

### Task 1: `domain/transfer`: `Record`・`MovedIn`・`Number`・`People`・`Collect`

> **この Task への上書き**(「本文より優先する直し」): `People` は本文の形のまま(Task 6 が使う)。`Set.Contains`・`ErrStep`・`Entity*` を作る。


移動の規則のうち、入出力のない部分を `internal/domain/transfer` に置く: 転送記録の 1 行、`moved_in` の行、番号の順、移動先のメンバーに足す人、移すものと外との関連。値の引き直しは Task 2。

**この Task が決めること:**

- `transfer.Record` の JSON は、キーの並びを `id`・`op`・`at`・`by`・`entity`・`target`・`old`・`new`・`to_store`・`to_store_name`・`to_remote` にする。`at` は `change.TimeLayout`。`to_store_name`・`to_remote` は空なら書かない(同じ Store の別のワークスペースへの移動は `to_store` に自分の Store ID だけを書く。A5a-1)。`entity` は `item`・`project`・`workspace`(定数 `EntityItem`・`EntityProject`・`EntityWorkspace`)。
- `ParseRecords` は、JSON として読めない行・`id` が空の行・`at` が RFC 3339 でない行を飛ばす(手で書き換えた行。仕様どおり直さない)。`entity` がない行は `item` として読む。
- `Collect` の「外」は、`all`(Store のすべてのアイテム。削除済みを含む)にいるアイテムだけ。`all` にいない相手への関連(壊れた関連)は数えない。
- `People` は、`hedgehug[bot]`・`member.ValidLogin` に合わないログイン名を数えない。足す人の綴りは、移動元のメンバーにいればその綴り、いなければ見つけた綴りのうち辞書順で先頭のもの。表示名は移動元のメンバーの表示名(いなければログイン名)。

**Files:**
- Modify: `internal/domain/change/record.go`(`MovedIn` を 1 つ足す)
- Create: `internal/domain/transfer/record.go`・`movedin.go`・`number.go`・`collect.go`・`people.go`
- Test: `internal/domain/transfer/record_test.go`・`movedin_test.go`・`number_test.go`・`collect_test.go`・`people_test.go`(どれも新規)

**Interfaces:**
- Consumes: `change.Record`・`change.Draft`・`change.Value`・`change.Stamp`・`change.TimeLayout`・`change.BotLogin`、`item.Item`・`item.Descendants`・`item.SortSiblings`・`item.Relation`、`member.Member`・`member.ValidLogin`、`workspace.Settings`・`workspace.Property`・`workspace.TypeUser`
- Produces:
  - `const change.MovedIn Type = "moved_in"`(`Resolve` は知らない種類を読み飛ばすので、`item.FromLog` の結果は変わらない)
  - `type transfer.Record struct { ID, Op change.ID; At time.Time; By string; Entity string; Target change.ID; Old, New string; ToStore, ToStoreName, ToRemote string }`、`const EntityItem = "item"`・`EntityProject = "project"`・`EntityWorkspace = "workspace"`、`func ParseRecords(b []byte) []Record`、`func (r Record) JSON() []byte`(末尾の改行なし)
  - `type transfer.MovedInInfo struct { Old, FromStore, FromStoreName string; FromWorkspace change.ID; FromWorkspaceName string; FromProject change.ID; FromProjectTitle string; At time.Time; Names map[change.ID]string }`、`func NewMovedIn(m MovedInInfo) change.Draft`(`T: change.MovedIn`、`F` なし、`to` に A5a-2 の形)、`func MovedInOf(log []change.Record) []MovedInInfo`(書いた順。読めない行は飛ばす)
  - `func transfer.Number(root item.Item, below []item.Item, next int) map[item.ID]int`
  - `type transfer.Cut struct { From, To item.ID; Type string }`、`type transfer.Set struct { Root item.Item; Below []item.Item; Outside []Cut }`、`func (s Set) Contains(id item.ID) bool`(指摘 3)、`var transfer.ErrNotFound`・`transfer.ErrStep`、`func Collect(root item.ID, all []item.Item) (Set, error)`
  - `type transfer.PeopleMap map[string]string`(移動元のログイン名の小文字 → 移動先のログイン名)、`func People(from workspace.Settings, items []item.Item, logs [][]change.Record, fromMembers, toMembers []member.Member) ([]member.Member, PeopleMap)`(指摘 2。1 つ目は移動先に足す人を小文字のログイン名の順で。2 つ目は、数えたすべての人の対応で、足す人も入る)

- [ ] **Step 1: 失敗するテストを書く(`Record`)**

`internal/domain/transfer/record_test.go`(新規)

```go
package transfer_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/transfer"
)

var (
	jst = time.FixedZone("JST", 9*3600)
	at  = time.Date(2026, 11, 14, 9, 0, 0, 0, jst)
)

func TestRecordJSON(t *testing.T) {
	cross := transfer.Record{ID: "T1", Op: "O1", At: at, By: "taro", Entity: transfer.EntityItem, Target: "I1", Old: "HH-12", New: "WEB-31",
		ToStore: "S2", ToStoreName: "team-b", ToRemote: "https://github.com/org/team-b"}
	want := `{"id":"T1","op":"O1","at":"2026-11-14T09:00:00.000+09:00","by":"taro","entity":"item","target":"I1","old":"HH-12","new":"WEB-31","to_store":"S2","to_store_name":"team-b","to_remote":"https://github.com/org/team-b"}`
	if got := string(cross.JSON()); got != want {
		t.Errorf("cross-store JSON =\n%s\nwant\n%s", got, want)
	}
	// 同じ Store の別のワークスペースへの移動は、to_store に自分の Store ID だけを書く。
	same := transfer.Record{ID: "T2", Op: "O1", At: at, By: "taro", Entity: transfer.EntityItem, Target: "I1", Old: "HH-12", New: "MB-3", ToStore: "S1"}
	wantSame := `{"id":"T2","op":"O1","at":"2026-11-14T09:00:00.000+09:00","by":"taro","entity":"item","target":"I1","old":"HH-12","new":"MB-3","to_store":"S1"}`
	if got := string(same.JSON()); got != wantSame {
		t.Errorf("same-store JSON =\n%s\nwant\n%s", got, wantSame)
	}
}

func TestParseRecordsSkipsUnreadableLines(t *testing.T) {
	cross := transfer.Record{ID: "T1", Op: "O1", At: at, By: "taro", Entity: transfer.EntityProject, Target: "P1", Old: "HH-P1", New: "WEB-P4", ToStore: "S2", ToStoreName: "team-b"}
	in := string(cross.JSON()) + "\n" +
		"not json\n" +
		`{"id":"T3","at":"bad"}` + "\n" +
		`{"at":"2026-11-14T09:00:00.000+09:00","target":"I9"}` + "\n\n" +
		// entity のない行は item として読む。
		`{"id":"T4","op":"O2","at":"2026-11-14T09:00:00.000+09:00","by":"taro","target":"I2","old":"HH-1","new":"HH-2","to_store":"S2"}` + "\n"
	got := transfer.ParseRecords([]byte(in))
	if len(got) != 2 {
		t.Fatalf("records = %+v, want 2", got)
	}
	if !got[0].At.Equal(at) {
		t.Errorf("at = %v", got[0].At)
	}
	got[0].At = cross.At
	if !reflect.DeepEqual(got[0], cross) {
		t.Errorf("first = %+v, want %+v", got[0], cross)
	}
	if got[1].ID != "T4" || got[1].Entity != transfer.EntityItem || got[1].Old != "HH-1" || got[1].ToStore != "S2" {
		t.Errorf("second = %+v", got[1])
	}
}
```

- [ ] **Step 2: 失敗するテストを書く(`MovedIn`)**

`internal/domain/transfer/movedin_test.go`(新規)

```go
package transfer_test

import (
	"encoding/json"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

func movedIn(old string) transfer.MovedInInfo {
	return transfer.MovedInInfo{
		Old: old, FromStore: "S1", FromStoreName: "team-a", FromWorkspace: "W1", FromWorkspaceName: "Web",
		FromProject: "P1", FromProjectTitle: "ログイン", At: at, Names: map[change.ID]string{"V2": "High", "V1": "Todo"},
	}
}

func stamp(ds ...change.Draft) []change.Record {
	n := 0
	return change.Stamp(nil, ds, change.Meta{Op: "O", At: at, Actor: change.User("taro")}, func() change.ID {
		n++
		return change.ID("r" + string(rune('0'+n)))
	})
}

func TestNewMovedInWritesTheStoredShape(t *testing.T) {
	d := transfer.NewMovedIn(movedIn("HH-12"))
	if d.T != change.MovedIn || d.F != "" {
		t.Fatalf("draft = %+v", d)
	}
	b, err := json.Marshal(stamp(d)[0])
	want := `{"id":"r1","op":"O","at":"2026-11-14T09:00:00.000+09:00","by":"taro","actor":"user","t":"moved_in",` +
		`"to":{"old":"HH-12","from_store":"S1","from_store_name":"team-a","from_workspace":"W1","from_workspace_name":"Web",` +
		`"from_project":"P1","from_project_title":"ログイン","at":"2026-11-14T09:00:00.000+09:00","names":{"V1":"Todo","V2":"High"}},"prev":[]}`
	if err != nil || string(b) != want {
		t.Errorf("row =\n%s, %v\nwant\n%s", b, err, want)
	}
}

func TestMovedInOfReadsInWrittenOrderAndSkipsBrokenRows(t *testing.T) {
	log := stamp(transfer.NewMovedIn(movedIn("HH-12")), transfer.NewMovedIn(movedIn("WEB-31")),
		change.Draft{T: change.MovedIn, To: json.RawMessage(`"壊れた行"`)})
	got := transfer.MovedInOf(log)
	if len(got) != 2 || got[0].Old != "HH-12" || got[1].Old != "WEB-31" {
		t.Fatalf("MovedInOf = %+v", got)
	}
	g := got[0]
	if g.FromStore != "S1" || g.FromStoreName != "team-a" || g.FromWorkspace != "W1" || g.FromWorkspaceName != "Web" ||
		g.FromProject != "P1" || g.FromProjectTitle != "ログイン" || !g.At.Equal(at) || g.Names["V2"] != "High" {
		t.Errorf("first = %+v", g)
	}
}

// moved_in の行は、アイテムの値を変えない。
func TestMovedInRowDoesNotChangeTheItem(t *testing.T) {
	it := item.Item{ID: "I1", Place: item.Place{Workspace: "W2", Project: "P2", Number: 3}, Kind: item.Task, Rank: "a", Title: "移ってきた", Watchers: []string{}}
	log := stamp(item.Create(it), transfer.NewMovedIn(movedIn("HH-12")))
	got, ok := item.FromLog(log)
	if !ok || got.Title != "移ってきた" || got.Place != it.Place {
		t.Errorf("FromLog = %+v, %v", got, ok)
	}
}
```

- [ ] **Step 3: 失敗するテストを書く(`Number`)**

`internal/domain/transfer/number_test.go`(新規)

```go
package transfer_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// node は、テスト用のアイテム。parent が空なら親なし。
func node(id, parent, rank string, deleted bool) item.Item {
	it := item.Item{ID: change.ID(id), Rank: rank, Created: item.Stamp{At: at}}
	if parent != "" {
		p := change.ID(parent)
		it.Place.Parent = &p
	}
	if deleted {
		it.Deleted = &change.Deletion{At: at}
	}
	return it
}

// 02「移すもの」: 移すアイテムが先、続いて子・孫を並び順に深さ優先。削除済みの子は、同じ親の下の削除されていない子のあと。
func TestNumberOrder(t *testing.T) {
	root := node("E", "", "a", false)
	below := []item.Item{ // わざと並びを崩す
		node("B", "E", "b", true),
		node("A2", "A", "0", true), // 順位は A1 より前だが、削除済みなので A1 のあと
		node("C", "E", "c", false),
		node("A1", "A", "a", false),
		node("A", "E", "a", false),
	}
	got := transfer.Number(root, below, 13)
	want := map[item.ID]int{"E": 13, "A": 14, "A1": 15, "A2": 16, "C": 17, "B": 18}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Number = %v, want %v", got, want)
	}
}

func TestNumberOfAnItemWithoutChildren(t *testing.T) {
	got := transfer.Number(node("T", "E", "a", false), nil, 7)
	if !reflect.DeepEqual(got, map[item.ID]int{"T": 7}) {
		t.Errorf("Number = %v", got)
	}
}
```

- [ ] **Step 4: 失敗するテストを書く(`Collect`)**

`internal/domain/transfer/collect_test.go`(新規)

```go
package transfer_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

func related(it item.Item, typ string, to change.ID) item.Item {
	it.Relations = append(it.Relations, item.Relation{Type: typ, To: to})
	return it
}

func TestCollectBelowAndOutside(t *testing.T) {
	e := node("E", "", "a", false)
	a := related(node("A", "E", "a", false), "relates", "B") // 中どうし: 残る
	b := related(node("B", "E", "b", true), "relates", "Y")  // 外の削除済みへ
	step := node("T", "A", "a", false)
	step.Kind = item.Step
	x := related(node("X", "", "z", false), "blocks", "A") // 外から中へ
	y := node("Y", "", "y", true)
	z := related(node("Z", "", "y", false), "blocks", "NOWHERE") // 壊れた関連は数えない
	all := []item.Item{y, x, b, step, a, e, z}

	set, err := transfer.Collect("E", all)
	if err != nil {
		t.Fatal(err)
	}
	var below []item.ID
	for _, it := range set.Below {
		below = append(below, it.ID)
	}
	if set.Root.ID != "E" || !slices.Equal(below, []item.ID{"A", "T", "B"}) {
		t.Errorf("root %s, below %v (削除済みも含み、深さ優先・兄弟の順)", set.Root.ID, below)
	}
	want := []transfer.Cut{{From: "B", To: "Y", Type: "relates"}, {From: "X", To: "A", Type: "blocks"}}
	if !slices.Equal(set.Outside, want) {
		t.Errorf("Outside = %+v, want %+v (両方の側。From・To・Type の順)", set.Outside, want)
	}
	if !set.Contains("T") || !set.Contains("E") || set.Contains("X") {
		t.Error("Contains が中と外を分けていない")
	}
}

func TestCollectRefuses(t *testing.T) {
	step := node("T", "A", "a", false)
	step.Kind = item.Step
	all := []item.Item{node("A", "", "a", false), step}
	if _, err := transfer.Collect("T", all); !errors.Is(err, transfer.ErrStep) {
		t.Errorf("step root = %v, want ErrStep", err)
	}
	if _, err := transfer.Collect("NONE", all); !errors.Is(err, transfer.ErrNotFound) {
		t.Errorf("unknown root = %v, want ErrNotFound", err)
	}
}
```

- [ ] **Step 5: 失敗するテストを書く(`People`。Review Focus 5)**

`internal/domain/transfer/people_test.go`(新規)

```go
package transfer_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 02「保存」のメンバー: 担当者・ユーザー型の値・ウォッチ・作成者・コメントと作業時間の書き手・履歴の操作者のうち、
// 移動先のメンバーにいない人(大文字小文字だけが違う人は同じ人)が、1 人 1 回だけ足される。
func TestPeopleAddsEachMissingPersonOnce(t *testing.T) {
	from := workspace.Settings{Properties: []workspace.Property{
		{ID: "PR", Key: "reviewer", Name: "レビュアー", Type: workspace.TypeUser},
		{ID: "PM", Key: "memo", Name: "メモ", Type: workspace.TypeText},
	}}
	it := item.Item{
		ID: "I1", Assignee: "Taro", Watchers: []string{"hanako", "HANAKO"}, Created: item.Stamp{By: "jiro"},
		Fields: map[change.ID]json.RawMessage{"PR": json.RawMessage(`"saburo"`), "PM": json.RawMessage(`"mika"`)}, // テキストは人ではない
	}
	logs := [][]change.Record{{
		{By: "shiro", T: change.Comment},                 // コメントの書き手
		{By: "shiro", T: change.Worklog},                 // 作業時間の書き手(同じ人は 1 回だけ)
		{By: change.BotLogin, T: change.Set, F: "title"}, // 人でない実行者は数えない
		{By: "bad name", T: change.Set, F: "title"},      // ログイン名に使えない文字
		{By: "goro", T: change.Set, F: "title"},          // 履歴の操作者
	}}
	fromMembers := []member.Member{{Login: "hanako", Name: "花子"}, {Login: "jiro", Name: "次郎"}, {Login: "Taro", Name: "太郎"}, {Login: "goro", Name: "五郎"}}
	toMembers := []member.Member{{Login: "taro", Name: "taro"}, {Login: "jiro", Name: "jiro"}}

	add, people := transfer.People(from, []item.Item{it}, logs, fromMembers, toMembers)

	wantAdd := []member.Member{{Login: "goro", Name: "五郎"}, {Login: "hanako", Name: "花子"}, {Login: "saburo", Name: "saburo"}, {Login: "shiro", Name: "shiro"}}
	if !reflect.DeepEqual(add, wantAdd) {
		t.Errorf("add = %+v, want %+v", add, wantAdd)
	}
	wantMap := transfer.PeopleMap{"taro": "taro", "jiro": "jiro", "hanako": "hanako", "goro": "goro", "saburo": "saburo", "shiro": "shiro"}
	if !reflect.DeepEqual(people, wantMap) {
		t.Errorf("map = %v, want %v (移動先にいる人は移動先の綴りに置き換える)", people, wantMap)
	}
}

func TestPeopleWithNobodyMissing(t *testing.T) {
	it := item.Item{ID: "I1", Assignee: "taro", Created: item.Stamp{By: "taro"}}
	add, people := transfer.People(workspace.Settings{}, []item.Item{it}, nil,
		[]member.Member{{Login: "taro", Name: "太郎"}}, []member.Member{{Login: "Taro", Name: "Taro"}})
	if len(add) != 0 || people["taro"] != "Taro" {
		t.Errorf("add = %+v, map = %v", add, people)
	}
}
```

- [ ] **Step 6: 失敗を確かめる**

Run: `go test ./internal/domain/transfer/...`
Expected: コンパイルエラー(`undefined: transfer.Record`・`change.MovedIn` など)。

- [ ] **Step 7: 実装する**

`internal/domain/change/record.go` の `Type` の定数に `MovedIn Type = "moved_in"`(コメント: 別の Store から移ってきた。保存形式「変更記録」)を足す。

`internal/domain/transfer/` を作る(パッケージのコメント: 移動の規則。入出力をしない)。

- `record.go`: `Record`・定数・`ParseRecords`・`JSON`。Step 1 の JSON の並びで、行の型(`line`)を別に持つ(`change.Record` と同じ作り)。`ParseRecords` は改行で分け、空の行は飛ばす。
- `movedin.go`: `MovedInInfo`・`NewMovedIn`・`MovedInOf`。`to` の型はキーを `old`・`from_store`・`from_store_name`・`from_workspace`・`from_workspace_name`・`from_project`・`from_project_title`・`at`(`change.TimeLayout`)・`names` の順に持つ。
- `number.go`: `Number(root, below, next)`。アルゴリズム(シグネチャとテストで決まらない所): `below` の並びは信用せず、`Parent` で子の一覧を作る。親ごとに `item.SortSiblings` で並べ、削除されていない子を先、削除済みの子を後に(それぞれ並び順を保つ)、1 人ずつ番号を振って、その子の下へ進む(深さ優先)。`root` が `next`。
- `collect.go`: `Collect`。`root` を `all` から探す(なければ `ErrNotFound`、`Kind` が `Step` なら `ErrStep`)。`Below` は `item.Descendants(root, all)`(削除済みを含む)。`Outside`: `all` の各アイテムの各関連について、持つ側と相手の片方だけが中(`Contains`)で、相手が `all` にいるものを `Cut{From: 持つ側, To: 相手, Type}` にし、`From`・`To`・`Type` の順に並べて重ねない。
- `people.go`: `PeopleMap`・`People`。数える人は、`items` の `Assignee`・`Watchers`・`Created.By`、`from.Properties` のうち `Type == workspace.TypeUser` で `Builtin` でないプロパティの `Fields` の値(JSON の文字列)、`logs` のすべての行の `By`。空・`change.BotLogin`・`!member.ValidLogin` は数えない。小文字で束ね、`toMembers` に小文字で一致する人がいれば `PeopleMap[小文字] = 移動先の綴り`、いなければ綴り(上の決めどおり)と表示名を決めて足す人にし、`PeopleMap[小文字]` はその綴り。

- [ ] **Step 8: 通す**

Run: `go test ./internal/domain/transfer/... ./internal/domain/change/... ./internal/domain/item/... && golangci-lint run ./internal/domain/...`
Expected: PASS、lint なし。

- [ ] **Step 9: コミットする**

```bash
git add internal/domain/change/record.go internal/domain/transfer
git commit -m "$(cat <<'EOF'
feat: 移動の規則のうち、転送記録・moved_in・番号の順・メンバーに足す人・外との関連を domain/transfer に足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: `transfer.Remap`・`UsableStatuses`

> **この Task への上書き**(「本文より優先する直し」): `Remap` に `toProject project.ID` を足す。`people == nil` は同じ Store のテストを入れる。`transfer.Unmatched` を作る。


アイテム 1 件の値を、移動先のワークスペースの定義に引き直す(02「値の引き直し」。A5a-7・A5a-8)。`app/transfers` が、移すアイテムごとに呼んで、返る行を移動先の保存に書く。

**この Task が決めること:**

- 行の並び: `from.Properties` の並びで、プロパティごとにまとめて出す(組み込みが先: ステータス・担当者・優先度・開始日・終了日・ポイント・見積もり時間・関連システム、続いて追加プロパティ)。ウォッチは最後。プロパティの中の並び: 複数の値は、取り除く行を先、足す行を後(取り除く値はアイテムの持っている並びのまま)。追加プロパティは、移動先の欄への `set` を先、移動元の欄を空にする `set` を後。
- 値の内部 ID は、ワークスペースごとに違う。そのため、組み込みのステータス・優先度・関連システムと、追加プロパティの値は、一致しても必ず移動先の ID に書き直す。追加プロパティの欄の名前(`fields/<プロパティの内部 ID>`)も、移動先のプロパティの内部 ID に変わるので、残す値は移動先の欄へ書き、移動元の欄は `null`(複数は `remove`)にする。
- 移動先のプロパティは、組み込みでなく、`Key` と `Type` が移動元と同じもの。見つからない値は、移動元の欄を空にして、空になった項目に並べる。
- ステータス: 名前が完全に一致して `UsableStatuses` にあるものはそれ。なければ `statusChoice[元の ID]`(`UsableStatuses` にあるときだけ使う)。それもなければ、元のステータスの分類(`from.Statuses` から引く)の `to.CategoryDefaults`。元の ID が `from` にいなければ `to.InitialStatus`。ステータスは「空になった項目」に並べない(02)。
- 優先度: 名前が一致する移動先の優先度。一致しなければ `to.InitialPriority` にして、「優先度」(移動元の優先度プロパティの名前)を空になった項目に並べる。優先度のないアイテムは何も書かない。
- 関連システム・複数選択: 一致したものだけを足す。元の値が 1 つ以上あって 1 つも一致しないときだけ、空になった項目に並べる(1 つでも残れば並べない)。単一選択・テキスト・数値・日付・チェック・URL・ユーザーは、残せなければ並べる。
- `people` が nil なら同じ Store(担当者・ユーザー型・ウォッチはそのまま)。nil でなければ別の Store で、ログインを小文字にして `people` で引いた移動先の綴りに書き換える(綴りが変わらなければ行を書かない。`people` にいないログインはそのまま)。ウォッチで、置き換えたログインがすでにウォッチにいるときは、足さない。
- 空になった項目の名前は、移動元のプロパティの名前(`Property.Name`)で、`from.Properties` の並び。1 件のアイテムの結果に同じ名前は 1 回。

**Files:**
- Create: `internal/domain/transfer/remap.go`(`Remap`・`RemapResult`・`UsableStatuses`・ステータスと優先度・関連システム)、`internal/domain/transfer/remap_fields.go`(追加プロパティと人。指摘 1)
- Test: `internal/domain/transfer/remap_test.go`(新規)

**Interfaces:**
- Consumes: Task 1 の `transfer.PeopleMap`、`item.Item`・`item.FieldKey`・`item.Field`・`workspace.Settings`・`workspace.Property`・`workspace.Value`・`workspace.Category*`・`workspace.Type*`・`change.NewSet`・`change.NewAdd`・`change.NewRemove`・`project.ID`
- Produces:
  - `type transfer.RemapResult struct { Drafts []change.Draft; Emptied []string }`
  - `func transfer.Remap(it item.Item, from, to workspace.Settings, toProject project.ID, people PeopleMap, statusChoice map[change.ID]change.ID) RemapResult`(指摘 1)
  - `func transfer.UsableStatuses(to workspace.Settings, p project.ID) []change.ID`(いまは `to.Statuses` の ID すべてを並びのまま。B5 がこの関数だけを差し替える)

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/transfer/remap_test.go`(新規)

```go
package transfer_test

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// settings は、既定の設定で、値の内部 ID を <p>V01〜<p>V16 にしたもの。ステータスは V01〜V05
// (Todo・InProgress・Done・Pending・Cancel)、優先度は V06〜V10(Highest・High・Medium・Low・Lowest)、
// 関連システムは V11〜V16(Web・API・iOS・Android・DB・CI)。
func settings(p string) workspace.Settings {
	n := 0
	return workspace.DefaultSettings(func() change.ID { n++; return change.ID(fmt.Sprintf("%sV%02d", p, n)) })
}

func choices(p string, names ...string) []workspace.Value {
	var vs []workspace.Value
	for i, n := range names {
		vs = append(vs, workspace.Value{ID: change.ID(fmt.Sprintf("%s%d", p, i+1)), Name: n})
	}
	return vs
}

// 移動元 F と移動先 T。T は、Pending を Hold に変え、優先度 Lowest と関連システム iOS をなくし、追加プロパティを一部だけ持つ。
func pair() (from, to workspace.Settings) {
	from, to = settings("F"), settings("T")
	from.Properties = append(from.Properties,
		workspace.Property{ID: "FP1", Key: "memo", Name: "メモ", Type: workspace.TypeText},
		workspace.Property{ID: "FP2", Key: "effort", Name: "工数", Type: workspace.TypeNumber},
		workspace.Property{ID: "FP3", Key: "env", Name: "環境", Type: workspace.TypeSingle, Choices: choices("FE", "本番", "検証")},
		workspace.Property{ID: "FP4", Key: "targets", Name: "対象", Type: workspace.TypeMulti, Choices: choices("FT", "A", "B", "C")},
		workspace.Property{ID: "FP5", Key: "reviewer", Name: "レビュアー", Type: workspace.TypeUser},
		workspace.Property{ID: "FP6", Key: "needs", Name: "要確認", Type: workspace.TypeCheck},
	)
	to.Statuses[3].Name = "Hold"
	to.Priorities = to.Priorities[:4]
	to.Systems = slices.Delete(slices.Clone(to.Systems), 2, 3)
	to.Properties = append(to.Properties,
		workspace.Property{ID: "TP1", Key: "memo", Type: workspace.TypeText},
		workspace.Property{ID: "TP2", Key: "effort", Type: workspace.TypeText}, // 型が違う
		workspace.Property{ID: "TP3", Key: "env", Type: workspace.TypeSingle, Choices: choices("TE", "本番")},
		workspace.Property{ID: "TP4", Key: "targets", Type: workspace.TypeMulti, Choices: choices("TT", "A", "B")},
		workspace.Property{ID: "TP5", Key: "reviewer", Type: workspace.TypeUser},
	)
	return from, to
}

func show(r transfer.RemapResult) []string {
	var out []string
	for _, d := range r.Drafts {
		out = append(out, fmt.Sprintf("%s %s %s>%s", d.T, d.F, d.From, d.To))
	}
	return out
}

func fullItem() item.Item {
	pts := 3.0
	return item.Item{
		ID: "I1", Status: "FV04", Priority: "FV10", Systems: []change.ID{"FV11", "FV13"}, Points: &pts,
		Assignee: "Taro", Watchers: []string{"Hanako"},
		Choices: map[change.ID][]change.ID{"FP3": {"FE2"}, "FP4": {"FT1", "FT3"}},
		Fields: map[change.ID]json.RawMessage{
			"FP1": json.RawMessage(`"メモ本文"`), "FP2": json.RawMessage(`3`), "FP3": json.RawMessage(`"FE2"`),
			"FP5": json.RawMessage(`"Jiro"`), "FP6": json.RawMessage(`true`),
		},
	}
}

// 同じ Store(people が nil): 担当者・ユーザー型・ウォッチはそのまま。
func TestRemapSameStore(t *testing.T) {
	from, to := pair()
	got := transfer.Remap(fullItem(), from, to, "P1", nil, nil)
	want := []string{
		`set status "FV04">"TV01"`, // Pending は移動先になく、同じ分類(未着手)の既定 Todo
		`set priority "FV10">"TV08"`, // Lowest は移動先になく、初期優先度 Medium
		`remove systems "FV11">`, `remove systems "FV13">`, `add systems >"TV11"`, // Web は一致、iOS はなし
		`set fields/TP1 null>"メモ本文"`, `set fields/FP1 "メモ本文">null`,
		`set fields/FP2 3>null`, // 型が違うので捨てる
		`set fields/FP3 "FE2">null`, // 検証は移動先にない
		`remove fields/FP4 "FT1">`, `remove fields/FP4 "FT3">`, `add fields/TP4 >"TT1"`, // A は一致、C はなし
		`set fields/TP5 null>"Jiro"`, `set fields/FP5 "Jiro">null`,
		`set fields/FP6 true>null`, // 移動先にない
	}
	if g := show(got); !slices.Equal(g, want) {
		t.Errorf("rows =\n%q\nwant\n%q", g, want)
	}
	// 値が入っていたのに空になった項目。ステータスと、一部だけ残った複数選択は並べない。優先度は初期優先度にしたので並べる。
	if wantEmptied := []string{"優先度", "工数", "環境", "要確認"}; !slices.Equal(got.Emptied, wantEmptied) {
		t.Errorf("Emptied = %v, want %v", got.Emptied, wantEmptied)
	}
}

// 別の Store(people が非 nil): 担当者・ユーザー型・ウォッチを移動先のログイン名に置き換える。
func TestRemapOtherStoreReplacesPeople(t *testing.T) {
	from, to := pair()
	people := transfer.PeopleMap{"taro": "taro", "hanako": "hanako2", "jiro": "jiro"}
	got := show(transfer.Remap(fullItem(), from, to, "P1", people, nil))
	for _, w := range []string{
		`set assignee "Taro">"taro"`, `set fields/TP5 null>"jiro"`, `remove watchers "Hanako">`, `add watchers >"hanako2"`,
	} {
		if !slices.Contains(got, w) {
			t.Errorf("rows lack %s: %q", w, got)
		}
	}
	// 担当者はステータスと優先度のあいだ、ウォッチは最後。
	if i, j, k := slices.Index(got, `set status "FV04">"TV01"`), slices.Index(got, `set assignee "Taro">"taro"`), slices.Index(got, `set priority "FV10">"TV08"`); !(i < j && j < k) {
		t.Errorf("order: status %d, assignee %d, priority %d", i, j, k)
	}
	if got[len(got)-1] != `add watchers >"hanako2"` {
		t.Errorf("last row = %s, want the watchers", got[len(got)-1])
	}
}

func TestRemapStatus(t *testing.T) {
	from, to := pair()
	remap := func(status change.ID, choice map[change.ID]change.ID) []string {
		return show(transfer.Remap(item.Item{ID: "I1", Status: status}, from, to, "P1", nil, choice))
	}
	// 名前が一致して使えるものはそのまま(選択は使わない)。
	if got := remap("FV03", map[change.ID]change.ID{"FV03": "TV02"}); !slices.Equal(got, []string{`set status "FV03">"TV03"`}) {
		t.Errorf("Done = %q", got)
	}
	// 使えないものは、選んだステータス。
	if got := remap("FV04", map[change.ID]change.ID{"FV04": "TV02"}); !slices.Equal(got, []string{`set status "FV04">"TV02"`}) {
		t.Errorf("chosen = %q", got)
	}
	// 選んだものが使えるステータスにないときは、同じ分類の既定。
	if got := remap("FV04", map[change.ID]change.ID{"FV04": "NOWHERE"}); !slices.Equal(got, []string{`set status "FV04">"TV01"`}) {
		t.Errorf("invalid choice = %q", got)
	}
	// 移動元にいない ID は、移動先の初期ステータス。ステータスのないアイテムは何も書かない。
	if got := remap("FGONE", nil); !slices.Equal(got, []string{`set status "FGONE">"TV01"`}) {
		t.Errorf("unknown = %q", got)
	}
	if got := remap("", nil); len(got) != 0 {
		t.Errorf("no status = %q", got)
	}
}

func TestUsableStatusesIsEveryStatusOfTheWorkspace(t *testing.T) {
	_, to := pair()
	want := []change.ID{"TV01", "TV02", "TV03", "TV04", "TV05"}
	if got := transfer.UsableStatuses(to, "P1"); !slices.Equal(got, want) {
		t.Errorf("UsableStatuses = %v, want %v", got, want)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

Run: `go test ./internal/domain/transfer/ -run 'Remap|Usable'`
Expected: コンパイルエラー(`undefined: transfer.Remap`)。

- [ ] **Step 3: 実装する**

`remap.go`: 上の「決めること」の順で、プロパティごとの小さな関数に分ける(1 つの関数に全部を入れない。認知的複雑度 15・関数 60 行の上限)。`Remap` は、`from.Properties` を順に回し、組み込みなら `status`・`assignee`・`priority`・`systems` の関数、追加なら `remap_fields.go` の関数を呼んで `RemapResult` に足す。最後にウォッチを足す。名前での引き当ては、`workspace.Value.Name` の完全一致(大文字小文字も区別する)。`remap_fields.go`: 追加プロパティ(`Builtin` でないもの)の型ごとの引き直しと、人の置き換え(`people`)。

- [ ] **Step 4: 通す**

Run: `go test ./internal/domain/transfer/... && golangci-lint run ./internal/domain/transfer/...`
Expected: PASS、lint なし。

- [ ] **Step 5: コミットする**

```bash
git add internal/domain/transfer/remap.go internal/domain/transfer/remap_fields.go internal/domain/transfer/remap_test.go
git commit -m "$(cat <<'EOF'
feat: アイテムの値を移動先のワークスペースの定義に引き直す transfer.Remap を足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: 口と保存: `Reader.Transfers`・`Reader.Purged`・`Tx.WriteTransfers`・`ItemWriter.Import`・`Remove`

> **この Task への上書き**(「本文より優先する直し」): `AddPurged`・`reentry` は本文のとおり。Task 4 はこの Task のあと。


転送記録 `transfers.jsonl` の読み書き、`purged.jsonl` の読み取り、アイテムのフォルダごとの写しと削除を、保存の口に足す。本物(`internal/infra/gitstore`)・偽物(`internal/app/apptest`)・照合テスト(`RunStoresContract`)が同じ振る舞いになるようにする。

**この Task が決めること:**

- `ItemWriter.Import(id, log, description)`: `log` の行を、`id` のアイテムのフォルダの `log.jsonl` に、**そのまま**(`id`・`op`・`at`・`by`・`actor`・`prev` を変えずに)書く。`description` が空でなければ `description.md` にする。`*.yaml` は保存のたびに作り直す(本物)。同じ Tx で続けて `Append` すると、`prev` は Import した行の先端につながる。`id` が `change.ValidID` でない・`log` が空・そのフォルダに行がすでにある(同じ Tx で書いたものを含む)・同じ Tx で `Remove` したものは、何も書かず `port.ErrInvalidOperation` を包んだエラー。`log` の中身(作成の行があるか)は調べない。
- `ItemWriter.Remove(id)`: アイテムのフォルダ(`log.jsonl`・`item.yaml`・`description.md`)を消す。ないものは `port.ErrNotFound`、`id` が使えなければ `ErrInvalidOperation`。同じ Tx で `Append`・`Describe` していた分は捨てる。同じ Tx で読むと、`Items().Get` と `Changes().Of` は `ErrNotFound`、`Items().List` に出ない。消すだけの Write も、行を書く Write と同じに `op.Check` する(Summary が必要)。
- `Tx.WriteTransfers().Append(r)`: `ID`・`Op`・`At`・`By` は Tx が埋める(`ID` は `IDs.New()`、`Op`・`At` はその操作のもの、`By` は `meta.Actor.By()`)。`Entity` が 3 種のどれでもない・`Target` が `change.ValidID` でない・`ToStore` が空なら `ErrInvalidOperation`。転送記録だけを書く Write も `op.Check` する。同じ Tx の `Transfers().List()` は、書いた分を、ファイルにある分のあとに含む。
- `Reader.Purged().List()`: `purged.jsonl` の行(`id`・`at`・`entity`・`target`・`display`)を、ファイルの並びで返す。読めない行は飛ばす。ファイルがなければ空。書く口は持たない(G)。偽物には、テスト用に `(*Stores).AddPurged(id, p)` を足す(指摘 6)。
- 失敗した Write(`fn` のエラー・コミットの失敗)は、`Import`・`Remove`・`WriteTransfers` の分も含めて何も残さない。

**置き場所と行数**(`internal/infra/gitstore` と `internal/app/apptest` は 20 ファイルで、足せない):

| ファイル | いま | 足すもの | 足したあと |
|---|---|---|---|
| `gitstore/write.go` | 144 | `Write` を `writeLocked` に分ける(+6)、`tx` の `Import`・`Remove`・`WriteTransfers`・`itemWriter`(約 +50) | 約 200(Task 4 でさらに約 +55) |
| `gitstore/commit.go` | 232 | `writeExtras`(消す・転送記録を書く。約 +30) | 約 262 |
| `gitstore/files.go` | 131 | `readTransfers`・`readPurged`・`appendTransfers`(約 +45) | 約 176 |
| `gitstore/read.go` | 281 | `view` に `removed`・`transfers` の 2 フィールド、`log` の早期の戻り(3 行)、`Transfers`・`Purged` の 2 メソッド。合わせて +10 以内(Reader の型は `files.go` に置く) | 291 以下 |
| `gitstore/yaml_regen.go` | 114 | `staleItems` が `removed` を飛ばす(+1) | 115 |
| `apptest/stores.go` | 245 | `enter`・`goroutineID`・`holders` を `apptest/reentry` に出す(約 −37)、`write`(送信の前置き)・`Import`・`Remove`・`WriteTransfers`・`AddPurged`(約 +60) | 約 268 |
| `apptest/stores_read.go` | 214 | `view` のフィールドと `Transfers`・`Purged`・型(約 +28) | 約 242 |
| `apptest/stores_contract_isolation.go` | 128 | 転送・取り込み・削除の照合(約 +125) | 約 253 |
| `apptest/stores_contract.go` | 108 | 照合の一覧に 4 行 | 約 116 |
| `apptest/reentry/guard.go` | 新規(新しいパッケージ) | `Guard`(`stores.go` から出したもの) | 約 40 |

`stores.go` は、足すだけだと 300 を超える(約 310)ので、同じ Store の中で同じ goroutine が入れ子に入ることを見つける部分(`holders`・`enter`・`goroutineID`)を、下位のパッケージ `internal/app/apptest/reentry` に出す。出す前後で、既存の `stores_test.go` の入れ子の panic のテストが通ることで確かめる。

**Files:**
- Modify: `internal/app/port/stores.go`
- Modify: `internal/infra/gitstore/write.go`・`commit.go`・`files.go`・`read.go`・`yaml_regen.go`
- Modify: `internal/app/apptest/stores.go`・`stores_read.go`・`stores_contract.go`・`stores_contract_isolation.go`
- Create: `internal/app/apptest/reentry/guard.go`
- Test: `internal/app/apptest/reentry/guard_test.go`(新規)、`internal/app/apptest/stores_test.go`(`AddPurged` の 1 件)、`internal/infra/gitstore/transfers_test.go`(新規)

**Interfaces:**
- Consumes: Task 1 の `transfer.Record`・`transfer.Entity*`、`change.Record`・`change.ValidID`
- Produces(`internal/app/port/stores.go`):

```go
// Reader に足す
Transfers() TransferReader
Purged() PurgedReader
// Tx に足す
WriteTransfers() TransferWriter

type TransferReader interface{ List() ([]transfer.Record, error) } // ファイルの並び。同じ Tx で書いた分はそのあと
type PurgedReader interface{ List() ([]Purged, error) }
type Purged struct {
	Entity  string // item・project・workspace・sprint・release
	Target  change.ID
	Display string
	At      time.Time
}
type TransferWriter interface{ Append(r transfer.Record) error } // ID・Op・At・By は Tx が埋める

// ItemWriter に足す
Import(id item.ID, log []change.Record, description string) error
Remove(id item.ID) error
```

- [ ] **Step 1: 失敗するテストを書く(照合)**

`internal/app/apptest/stores_contract_isolation.go` に次を足す。`stores_contract.go` の `cases` に `{"transfer records are appended and listed", contractTransfers}`・`{"purged is empty without a purge", contractPurgedEmpty}`・`{"remove deletes an item folder", contractRemove}` を、別の Store を使う `t.Run` に `"import copies an item folder between stores"`(`contractImport(t, s, a, b)`)を足す。

```go
// 転送記録は、同じ操作の行と同じ op・at を持ち、同じ Tx の中から読める。失敗した Write は残さない。
func contractTransfers(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	rec := transfer.Record{Entity: transfer.EntityItem, Target: cItem.ID, Old: "HH-1", New: "MB-3",
		ToStore: "01OTHERSTORE", ToStoreName: "team", ToRemote: "https://github.com/org/team"}
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if err := tx.WriteTransfers().Append(rec); err != nil {
			return err
		}
		if got, err := tx.Transfers().List(); err != nil || len(got) != 1 {
			t.Errorf("in the tx: %+v, %v", got, err)
		}
		return tx.WriteItems().Append(cItem.ID, change.NewSet("title", "認証", "移動した"))
	})
	boom := errors.New("boom")
	if err := s.Write(t.Context(), id, taro, func(tx port.Tx) error {
		return errors.Join(tx.WriteTransfers().Append(rec), boom)
	}); !errors.Is(err, boom) {
		t.Fatalf("failing write = %v", err)
	}
	mustRead(t, s, id, func(r port.Reader) error {
		got, err := r.Transfers().List()
		log, _ := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID})
		if err != nil || len(got) != 1 || len(log) < 2 {
			t.Fatalf("transfers = %+v (%v), log = %d rows", got, err, len(log))
		}
		row := log[len(log)-1]
		g := got[0]
		if g.ID == "" || g.Op != row.Op || !g.At.Equal(row.At) || g.By != "taro" {
			t.Errorf("stamped = %+v, want op %s at %v by taro", g, row.Op, row.At)
		}
		g.ID, g.Op, g.At, g.By = "", "", rec.At, ""
		if g != rec {
			t.Errorf("record = %+v, want %+v", g, rec)
		}
		return nil
	})
	// 転送記録だけを書く Write も、Summary が要る。使えない行は書けない。
	noSummary := port.Operation{Actor: change.User("taro")}
	if err := s.Write(t.Context(), id, noSummary, func(tx port.Tx) error { return tx.WriteTransfers().Append(rec) }); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("no summary = %v, want ErrInvalidOperation", err)
	}
	for _, bad := range []transfer.Record{{Entity: "sprint", Target: cItem.ID, ToStore: "S"}, {Entity: "item", Target: "bad id", ToStore: "S"}, {Entity: "item", Target: cItem.ID}} {
		if err := s.Write(t.Context(), id, taro, func(tx port.Tx) error { return tx.WriteTransfers().Append(bad) }); !errors.Is(err, port.ErrInvalidOperation) {
			t.Errorf("Append(%+v) = %v, want ErrInvalidOperation", bad, err)
		}
	}
}

func contractPurgedEmpty(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	mustRead(t, s, id, func(r port.Reader) error {
		if got, err := r.Purged().List(); err != nil || len(got) != 0 {
			t.Errorf("Purged = %+v, %v", got, err)
		}
		return nil
	})
}

// Remove は、アイテムのフォルダを消す。同じ Tx から読むと、もうない。説明も一緒に消える。
func contractRemove(t *testing.T, s port.Stores, id port.StoreID) {
	ref := change.Ref{Kind: change.KindItem, ID: cItem.ID}
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error { return tx.WriteItems().Describe(cItem.ID, "説明") })
	boom := errors.New("boom")
	if err := s.Write(t.Context(), id, taro, func(tx port.Tx) error {
		return errors.Join(tx.WriteItems().Remove(cItem.ID), boom)
	}); !errors.Is(err, boom) {
		t.Fatalf("failing write = %v", err)
	}
	mustRead(t, s, id, func(r port.Reader) error {
		if d, err := r.Items().Description(cItem.ID); err != nil || d != "説明" {
			t.Errorf("after the failed write: %q, %v", d, err)
		}
		return nil
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if err := tx.WriteItems().Remove(cItem.ID); err != nil {
			return err
		}
		if _, err := tx.Items().Get(cItem.ID); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Get in the tx = %v", err)
		}
		if err := tx.WriteItems().Import(cItem.ID, []change.Record{{ID: "x"}}, ""); !errors.Is(err, port.ErrInvalidOperation) {
			t.Errorf("Import after Remove = %v, want ErrInvalidOperation", err)
		}
		return nil
	})
	mustRead(t, s, id, func(r port.Reader) error {
		if _, err := r.Items().Get(cItem.ID); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Get = %v", err)
		}
		if _, err := r.Changes().Of(ref); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Changes().Of = %v", err)
		}
		if _, err := r.Items().Description(cItem.ID); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("Description = %v", err)
		}
		if its, err := r.Items().List(cProject.ID); err != nil || len(its) != 0 {
			t.Errorf("List = %+v, %v", its, err)
		}
		if _, err := r.Projects().Get(cProject.ID); err != nil {
			t.Errorf("the project went too: %v", err)
		}
		return nil
	})
	if err := s.Write(t.Context(), id, taro, func(tx port.Tx) error { return tx.WriteItems().Remove(cItem.ID) }); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Remove of a missing item = %v, want ErrNotFound", err)
	}
	if err := s.Write(t.Context(), id, taro, func(tx port.Tx) error { return tx.WriteItems().Remove("bad id") }); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("Remove of a bad id = %v, want ErrInvalidOperation", err)
	}
}

// Import は、別の Store のアイテムの行と説明を、行を変えずに写す。続けて書く行は、写した行の先端につながる。
func contractImport(t *testing.T, s port.Stores, a, b port.StoreID) {
	ref := change.Ref{Kind: change.KindItem, ID: cItem.ID}
	mustWrite(t, s, a, taro, writeFixture)
	mustWrite(t, s, a, taro, func(tx port.Tx) error { return tx.WriteItems().Describe(cItem.ID, "説明") })
	var log []change.Record
	var text string
	mustRead(t, s, a, func(r port.Reader) error {
		var err error
		if log, err = r.Changes().Of(ref); err != nil {
			return err
		}
		text, err = r.Items().Description(cItem.ID)
		return err
	})
	mustWrite(t, s, b, taro, func(tx port.Tx) error {
		if err := tx.WriteItems().Import(cItem.ID, log, text); err != nil {
			return err
		}
		return tx.WriteItems().Append(cItem.ID, change.NewSet("title", "認証", "移動先で変えた"))
	})
	mustRead(t, s, b, func(r port.Reader) error {
		got, err := r.Changes().Of(ref)
		if err != nil || len(got) != len(log)+1 {
			t.Fatalf("rows = %d, %v, want %d", len(got), err, len(log)+1)
		}
		for i, w := range log {
			g := got[i]
			if g.ID != w.ID || g.Op != w.Op || !g.At.Equal(w.At) || g.By != w.By || g.Actor != w.Actor || g.T != w.T || g.F != w.F ||
				string(g.From) != string(w.From) || string(g.To) != string(w.To) || !slices.Equal(g.Prev, w.Prev) {
				t.Errorf("row %d = %+v, want %+v (行は変えずに写す)", i, g, w)
			}
		}
		if last := got[len(got)-1]; !slices.Equal(last.Prev, []change.ID{log[0].ID}) {
			t.Errorf("the row after the import has prev %v, want [%s]", last.Prev, log[0].ID)
		}
		it, err := r.Items().Get(cItem.ID)
		if d, _ := r.Items().Description(cItem.ID); err != nil || it.Title != "移動先で変えた" || d != "説明" {
			t.Errorf("item = %+v (%v), description %q", it, err, d)
		}
		return nil
	})
	for name, call := range map[string]func(tx port.Tx) error{
		"existing folder": func(tx port.Tx) error { return tx.WriteItems().Import(cItem.ID, log, "") },
		"empty log":       func(tx port.Tx) error { return tx.WriteItems().Import("01CONTRACTIT00000000000009", nil, "") },
		"bad id":          func(tx port.Tx) error { return tx.WriteItems().Import("bad id", log, "") },
	} {
		if err := s.Write(t.Context(), b, taro, call); !errors.Is(err, port.ErrInvalidOperation) {
			t.Errorf("Import (%s) = %v, want ErrInvalidOperation", name, err)
		}
	}
}
```

あわせて、ファイルの先頭の import に `slices` と `github.com/kukv/hedgehug/internal/domain/transfer` を足す。

`internal/app/apptest/reentry/guard_test.go`(新規)

```go
package reentry_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest/reentry"
)

func TestGuardPanicsOnNestedEnterInOneGoroutine(t *testing.T) {
	var g reentry.Guard
	leave := g.Enter(`store "S1"`)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("a nested Enter did not panic")
			}
		}()
		g.Enter(`store "S1"`)
	}()
	leave()
	g.Enter(`store "S1"`)() // 出たあとは入れる
}

func TestGuardLetsOtherGoroutinesIn(t *testing.T) {
	var g reentry.Guard
	defer g.Enter("x")()
	done := make(chan struct{})
	go func() { defer close(done); g.Enter("x")() }()
	<-done
}
```

`internal/app/apptest/stores_test.go` に足す:

```go
func TestFakeAddPurged(t *testing.T) {
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	p := port.Purged{Entity: "item", Target: "01PURGEDIT0000000000000001", Display: "HH-12", At: time.Date(2026, 12, 12, 0, 0, 5, 0, time.UTC)}
	s.AddPurged("S1", p)
	err := s.Read(t.Context(), "S1", func(r port.Reader) error {
		got, err := r.Purged().List()
		if err != nil || len(got) != 1 || got[0] != p {
			t.Errorf("Purged = %+v, %v", got, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: 失敗するテストを書く(gitstore の保存のしかた)**

`internal/infra/gitstore/transfers_test.go`(新規)。`newStore`・`files`・`headCount`・`headCommit`・`worktreeStatus`・`taskRef`(`recover_test.go`)を使う。

```go
package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

var moveOp = port.Operation{Actor: change.User("taro"), Summary: "移動"}

// sampleLog は、サンプルのストアのタスクの行。
func sampleTaskLog(t *testing.T, s *Stores, id port.StoreID) []change.Record {
	t.Helper()
	var log []change.Record
	if err := s.Read(t.Context(), id, func(r port.Reader) error {
		var err error
		log, err = r.Changes().Of(taskRef)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return log
}

func TestImportWritesTheFolderInOneCommit(t *testing.T) {
	src, srcID, srcDir := newStore(t)
	if err := apptest.Sample(t.Context(), src, srcID); err != nil {
		t.Fatal(err)
	}
	log := sampleTaskLog(t, src, srcID)
	dst, dstID, dstDir := newStore(t)
	before := headCount(t, dstDir)
	if err := dst.Write(t.Context(), dstID, moveOp, func(tx port.Tx) error {
		return tx.WriteItems().Import(apptest.SampleTask, log, "説明")
	}); err != nil {
		t.Fatal(err)
	}
	if headCount(t, dstDir) != before+1 {
		t.Errorf("commits = %d, want %d", headCount(t, dstDir), before+1)
	}
	p := "items/" + string(apptest.SampleTask) + "/"
	got, want := files(t, dstDir), files(t, srcDir)
	if got[p+"log.jsonl"] != want[p+"log.jsonl"] {
		t.Errorf("log.jsonl =\n%s\nwant (写した行は変わらない)\n%s", got[p+"log.jsonl"], want[p+"log.jsonl"])
	}
	if got[p+"description.md"] != "説明" || !strings.Contains(got[p+"item.yaml"], "ログイン画面を作る") {
		t.Errorf("description.md = %q, item.yaml =\n%s", got[p+"description.md"], got[p+"item.yaml"])
	}
	if st := worktreeStatus(t, dstDir); !st.IsClean() {
		t.Errorf("worktree is not clean:\n%s", st)
	}
}

func TestRemoveDeletesTheFolderInOneCommit(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	before := headCount(t, dir)
	if err := s.Write(t.Context(), id, moveOp, func(tx port.Tx) error { return tx.WriteItems().Remove(apptest.SampleTask) }); err != nil {
		t.Fatal(err)
	}
	if headCount(t, dir) != before+1 {
		t.Errorf("commits = %d, want %d", headCount(t, dir), before+1)
	}
	if _, err := os.Stat(filepath.Join(dir, "items", string(apptest.SampleTask))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the folder is still there: %v", err)
	}
	if _, ok := files(t, dir)["items/"+string(apptest.SampleEpic)+"/log.jsonl"]; !ok {
		t.Error("another item went too")
	}
	if st := worktreeStatus(t, dir); !st.IsClean() {
		t.Errorf("worktree is not clean:\n%s", st)
	}
}

func TestTransfersAreAppendedToOneFile(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	rec := func(old string) transfer.Record {
		return transfer.Record{Entity: transfer.EntityItem, Target: apptest.SampleTask, Old: old, New: "MB-3", ToStore: "01OTHERSTORE", ToStoreName: "team"}
	}
	for _, old := range []string{"HH-2", "HH-9"} {
		if err := s.Write(t.Context(), id, moveOp, func(tx port.Tx) error { return tx.WriteTransfers().Append(rec(old)) }); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, "transfers.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	got := transfer.ParseRecords(b)
	if len(got) != 2 || got[0].Old != "HH-2" || got[1].Old != "HH-9" || strings.Count(string(b), "\n") != 2 {
		t.Fatalf("transfers.jsonl =\n%s", b)
	}
	if msg := headCommit(t, dir).Message; !strings.Contains(msg, "Hedgehug-Op: "+string(got[1].Op)) {
		t.Errorf("the last commit %q does not carry op %s", msg, got[1].Op)
	}
	if st := worktreeStatus(t, dir); !st.IsClean() {
		t.Errorf("worktree is not clean (transfers.jsonl は同じコミットに入る):\n%s", st)
	}
	err = s.Read(t.Context(), id, func(r port.Reader) error {
		list, err := r.Transfers().List()
		if err != nil || len(list) != 2 || list[0].ID != got[0].ID || list[1].ID != got[1].ID {
			t.Errorf("List = %+v, %v", list, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPurgedIsReadFromTheFileAndSkipsBrokenLines(t *testing.T) {
	s, id, dir := newStore(t)
	file := `{"id":"01P1","at":"2026-12-12T00:00:05.000+09:00","by":"hedgehug[bot]","actor":"none","entity":"item","target":"01JF3KQ8W2A7XG0M5C9D4E6HTR","display":"HH-12"}` + "\n" +
		"not json\n" + `{"id":"01P2","at":"bad","entity":"item","target":"01X"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "purged.jsonl"), []byte(file), 0o644); err != nil {
		t.Fatal(err)
	}
	err := s.Read(t.Context(), id, func(r port.Reader) error {
		got, err := r.Purged().List()
		if err != nil || len(got) != 1 || got[0].Entity != "item" || got[0].Target != "01JF3KQ8W2A7XG0M5C9D4E6HTR" ||
			got[0].Display != "HH-12" || got[0].At.UTC().Format("2006-01-02T15:04:05") != "2026-12-11T15:00:05" {
			t.Errorf("Purged = %+v, %v", got, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// コミットが失敗したら、写したフォルダ・消したフォルダ・転送記録は、何も残らず元に戻る。
func TestFailedWriteRestoresImportedAndRemovedFolders(t *testing.T) {
	s, id, dir := newStore(t)
	if err := apptest.Sample(t.Context(), s, id); err != nil {
		t.Fatal(err)
	}
	log := sampleTaskLog(t, s, id)
	beforeFiles, beforeHead := files(t, dir), headCommit(t, dir).Hash
	boom := errors.New("boom")
	s.beforeCommit = func() error { return boom }
	err := s.Write(t.Context(), id, moveOp, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Remove(apptest.SampleTask),
			tx.WriteItems().Import("01MOVEDIT0000000000000001", log, "写し"),
			tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityItem, Target: apptest.SampleTask, Old: "HH-2", New: "MB-3", ToStore: "S"}),
		)
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Write = %v", err)
	}
	if !reflect.DeepEqual(files(t, dir), beforeFiles) || headCommit(t, dir).Hash != beforeHead {
		t.Error("the failed write left files or a commit behind")
	}
	if st := worktreeStatus(t, dir); !st.IsClean() {
		t.Errorf("worktree is not clean:\n%s", st)
	}
}
```

- [ ] **Step 3: 失敗を確かめる**

Run: `go test ./internal/app/apptest/... ./internal/infra/gitstore/ -run 'Contract|Fake|Import|Remove|Transfers|Purged|FailedWrite|Guard'`
Expected: コンパイルエラー(`tx.WriteTransfers undefined`・`r.Purged undefined`・`undefined: reentry` など)。

- [ ] **Step 4: 実装する(口)**

`internal/app/port/stores.go`: 上の「Produces」のとおり、`Reader`・`Tx`・`ItemWriter` に足し、型を足す。`transfer` と `time` を import する。

- [ ] **Step 5: 実装する(偽物)**

1. `apptest/reentry/guard.go`(新規): `stores.go` の `holders`・`holdersMu`・`enter`・`goroutineID` をそのまま出し、`type Guard struct{ mu sync.Mutex; holders map[uint64]bool }`(ゼロ値で使える。`holders` は初めて使うときに作る)と `func (g *Guard) Enter(what string) (leave func())` にする。panic の文は `apptest: nested Read or Write of <what> in one goroutine (gitstore would deadlock)`。`stores.go` の `store` は `guard reentry.Guard` を持ち、`Read`・`Write` は `defer st.guard.Enter(fmt.Sprintf("store %q", id))()`。`AddStore` の `holders` の初期化は消す。`bytes`・`runtime`・`strconv` の import も消す。
2. `stores.go`: `store` に `transfers []transfer.Record`・`purged []port.Purged`。`Write` の本体を `write(ctx, id, op, fn, send func() error) error`(`send` は nil でよい。`op.Check` のあと、反映の前に呼び、エラーなら何も反映せずそのエラーを返す。Task 4 の `SyncSave` の偽物が使う)に移し、`Write` は `write(..., nil)` を呼ぶ。反映は、`pending` の行・`removed` のフォルダ(`logs` と `texts` から消す)・`transfers`・`texts` の順。「何も書かない」判定に `removed`・`transfers` を足す。`tx` の `WriteItems` は `Import`・`Remove` を持つ `itemWriter`(`idWriter` を埋め込む)を返し、`WriteTransfers` は `transferWriter` を返す。`Import`・`Remove`・`transferWriter.Append` は「この Task が決めること」のとおり。`Remove` は `pending[ref]`・`texts[ref]` を捨て、`removed[ref] = true`。`Import` は行を `cloneRecord` で写して `pending[ref]` にし、`places` の覚えを消し、説明があれば `texts[ref]` にする。`AddPurged(id, p)` はテスト用(`SetRetentionDays` と同じ書き方)。
3. `stores_read.go`: `view` に `removed map[change.Ref]bool`・`transfers []transfer.Record`(同じ Tx で書いた分)・`baseTransfers []transfer.Record`・`purged []port.Purged`。`log` は `removed[ref]` なら nil を返す(`ids` は `log` が空のものを `Get` が `ErrNotFound` にするので、そのまま)。`Read`・`Write` が `view` を作る所で `baseTransfers`・`purged` を渡す。`Transfers()`・`Purged()` と、その型を足す。

- [ ] **Step 6: 実装する(gitstore)**

1. `files.go`: `readTransfers(root) ([]transfer.Record, error)`(`transfers.jsonl`。なければ空。`transfer.ParseRecords`)、`readPurged(root) ([]port.Purged, error)`(`purged.jsonl` の行を `id`・`at`・`entity`・`target`・`display` で読み、`id` が空・`at` が読めない行は飛ばす)、`appendTransfers(root, recs)`(`recs[i].JSON()` と `\n` を、なければ作って末尾に足す)。
2. `read.go`: `view` に `removed map[change.Ref]bool`・`transfers []transfer.Record`、`log` の早期の戻り、`Transfers()`・`Purged()`(型は `files.go` の関数を呼ぶ小さな型で、`files.go` に置く)。行数は上の表のとおり +10 以内。
3. `write.go`: `Write` の、ロックを取ったあとの部分を `func (s *Stores) writeLocked(root string, op port.Operation, fn func(tx port.Tx) error) error` に出し(`Write` は `dir`・`open`・`defer unlock()` のあと `writeLocked` を呼ぶだけ)。「何も書かない」の判定は `len(t.pending) == 0 && len(t.texts) == 0 && len(t.removed) == 0 && len(t.transfers) == 0`。`Write` の初めの `view` に `removed: map[change.Ref]bool{}` を足す。`itemWriter`(`idWriter` を埋め込む)の `Import`・`Remove`、`tx.WriteTransfers`・`transferWriter.Append` を足す。`Import` は `t.log(ref)` が空であることを確かめ、`t.pending[ref] = 写した行`、`delete(t.places, id)`、説明があれば `t.texts[ref]`。`Remove` は `t.log(ref)` が空なら `ErrNotFound`、`delete(t.pending, ref)`・`delete(t.texts, ref)`・`t.removed[ref] = true`・`delete(t.places, id)`。
4. `commit.go`: `writeExtras(wt *git.Worktree, t *tx) error` を足し、`writeAndCommit` の `rewriteStale` のあと、`beforeCommit` の前に 1 回呼ぶ。`writeExtras` は、`t.removed` を並べて(`writeTexts` と同じ並べ方)、フォルダの中のファイルを `os.ReadDir` で取り、`wt.Remove` で 1 つずつ消してから `os.RemoveAll` でフォルダを消す(フォルダがなければ飛ばす)。続けて、`t.transfers` があれば `appendTransfers` して `addPaths(wt, "transfers.jsonl")`。消すのを書き換え(`rewriteStale`)のあとにするので、消すアイテムの `item.yaml` を書き直して消す無駄は、`yaml_regen.go` の `staleItems` が `c.t.removed[ref]` を飛ばして避ける(`writing` の判定に `|| c.t.removed[ref]`)。`writeAndCommit` の認知的複雑度は 13 → 14 になるので、`golangci-lint` で 15 を超えないことを確かめる。

- [ ] **Step 7: 通す**

Run: `go test ./internal/app/... ./internal/infra/gitstore/... && wc -l internal/infra/gitstore/read.go internal/infra/gitstore/write.go internal/infra/gitstore/commit.go internal/infra/gitstore/files.go internal/app/apptest/stores.go internal/app/apptest/stores_read.go internal/app/apptest/stores_contract_isolation.go && golangci-lint run ./internal/...`
Expected: PASS。`wc -l` は表の「足したあと」以下(どれも 300 以下)。lint なし。

- [ ] **Step 8: コミットする**

```bash
git add internal/app/port/stores.go internal/app/apptest internal/infra/gitstore
git commit -m "$(cat <<'EOF'
feat: 転送記録と完全に削除の記録を読み書きする口と、アイテムのフォルダの写し・削除を足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: 口と同期: `Replicator.SyncSave`(gitstore・偽物)と `syncing.Service.Deliver`

> **この Task への上書き**(「本文より優先する直し」): `syncing.Service.Stores` は `NotifyWrites` で包んだものを配線で渡す。`port.SaveError` を作る。


移動先に remote があるとき、「受け取る → 保存する → 送る」を 1 つのロックの中で行い、送れなければ保存のコミットだけを捨てる口を足す(A5a-5。Review Focus 1)。`app/syncing` には、トークン・受け取った位置・同期の状態の記録を `run` と同じに扱って、この口を呼ぶ `Deliver` を足す。

**この Task が決めること:**

- `SyncSave` の 1 回の試みは、(1) `receive`(取り込み。`Sync` の前半と同じ) → (2) 保存(`fn`) → (3) `push`。(1) の失敗は、保存しない(`fn` を呼ばない)。(2) の失敗(`fn` のエラー・`op.Check`・コミットの失敗)は、`*port.SaveError` で包んで返す(`errors.Is` で元のエラーも分かる)。送信が通信の失敗で断られたら、(2) のコミットだけを捨てて、そのエラーをそのまま返す。送信が他の人の更新と重なって拒否されたら(`errRejected`)、同じく捨てて、(1) からやり直す(最大 `maxSendTries` 回。超えたら `port.ErrSendConflict`)。**`fn` は試みごとに呼ばれる。** 呼ぶたびに新しい `Tx` で、前の呼び出しの書き込みは残っていない。受け取ったもの(次の試みで取り込み直すもの)が `fn` の読む状態に入る。
- 保存のコミットを捨てる方法: (1) のあと(保存の前)の先端のコミット `base` を覚える。捨てるときは、`guarded`(保存の途中の印を置く)の中で、既存の `rollback(wt, base)`(`wt.Reset` のハードリセットで先端のブランチを `base` に戻し、作業ツリーとインデックスを `base` に合わせ、`wt.Clean(Dir: true)` で `base` にないファイルとフォルダを消す)を呼ぶ。`base` は (1) で取り込んだ結果なので、受け取って合わせた結果は残り、保存のコミットだけが消える。捨てている途中で止まったら、印が残り、次の操作の `recoverInterrupted` が、そのときの先端に作業ツリーを合わせる。
- 受け取りで `port.ErrRemoteRewritten` になったときは、`Sync` と同じく `restoreSeen` を足す。`Remote.ReceiveOnly` の `SyncSave` は `port.ErrInvalidOperation` を包んで断る(閲覧のみの Store には保存しない)。
- 偽物: `apptest.Replicator` に `Stores *Stores`(`fn` を実行する Store)と `SendErrs map[port.StoreID]error`(送信の失敗)を足す。`SyncSave` は、`Errs[id]` があれば `fn` を呼ばずにそれを返し(受け取りの失敗)、なければ `Stores.write(..., send)` で保存する。`send` は `SendErrs[id]` を返し、あればその Store に何も反映しない。反映できたら `Results[id]` を返す。`fn` などの保存の失敗は `*port.SaveError`。呼び出しは `ReplicatorCall{Save: true}` で覚える。
- `syncing.Service.Deliver(ctx, id, op, fn)`: Store が `Targets` にいなければ `port.ErrStoreNotFound`。閲覧のみなら `port.ErrInvalidOperation` を包んで断る。`Remote` がなければ `Service.Stores.Write` だけ(`Replicator` は呼ばない)。あれば、アカウントがなければ `syncstatus.NoAccount`(保存しない。状態は記録しない)、トークンを得られなければ `run` と同じ理由、あれば `State.Seen` を渡して `Replicator.SyncSave` を呼ぶ。通信の失敗は `remoteReason` で理由にして `State.Failed` に記録し、`syncstatus.Reason` を返す(error は nil)。`*port.SaveError` は、理由にせず、そのまま error で返す(状態には記録しない)。成功したら `State.Succeeded(…, res.Seen)`、ミラーへ送り(`run` と同じ `mirror`)、`EventSync` を知らせる。`AfterSuccess` は呼ばない。
- `Service` に `Stores port.Stores` を足す(指摘 5)。配線では、`NotifyWrites` で包む前の `Stores` を渡す(包むと、書いたあとに自動の同期が起き、`SyncSave` がいま送ったものをもう一度送りに行く)。
- `run` の「アカウントがない・トークンを得られない」の部分を、`Deliver` と共有する `func (s *Service) token(ctx, t port.SyncTarget) (tok string, r syncstatus.Reason, err error)` に出す(`run` のふるまいは変えない。既存のテストで確かめる)。

**置き場所と行数:**

| ファイル | いま | 足すもの | 足したあと |
|---|---|---|---|
| `gitstore/replicator.go` | 264 | `begin`(`Sync` の前置き。`Sync` は約 −20、`begin` は約 +26)、`once` の前半を `receive` に出す(+6) | 約 276 |
| `gitstore/write.go` | 約 200(Task 3 のあと) | `SyncSave`・`saveOnce`・`discard`(約 +55)。保存と同じ場所に置く(`replicator.go` に足すと 300 を超える) | 約 255 |
| `apptest/sync.go` | 172 | `SyncSave` の偽物とフィールド(約 +40) | 約 212 |
| `port/replicator.go` | 60 | `SyncSave`・`SaveError`(約 +14) | 約 74 |
| `syncing/deliver.go` | 新規 | `Deliver`(約 70) | 約 70 |
| `syncing/run.go` | 92 | `token` を出す(ほぼ同じ行数) | 約 95 |

**Files:**
- Modify: `internal/app/port/replicator.go`
- Modify: `internal/infra/gitstore/replicator.go`・`write.go`
- Modify: `internal/app/apptest/sync.go`
- Create: `internal/app/syncing/deliver.go`
- Modify: `internal/app/syncing/service.go`(`Stores` のフィールド)・`run.go`(`token`)
- Test: `internal/infra/gitstore/syncsave_test.go`(新規)、`internal/app/apptest/replicator_test.go`(足す)、`internal/app/syncing/deliver_test.go`(新規)、`internal/app/port/replicator_test.go`(新規)

**Interfaces:**
- Consumes: Task 3 の `Stores.writeLocked`(gitstore)・`Stores.write`(偽物)、既存の `open`・`rollback`・`guarded`・`syncRun`・`push`・`result`・`restoreSeen`・`errRejected`・`maxSendTries`、`syncing` の `remoteReason`・`tokenReason`・`mirror`・`failed`・`publish`
- Produces:
  - `port.Replicator.SyncSave(ctx context.Context, storeID StoreID, remote Remote, op Operation, fn func(tx Tx) error) (SyncResult, error)`
  - `type port.SaveError struct{ Err error }`(`Error() string`・`Unwrap() error`。通信の失敗は包まない)
  - `(*gitstore.Replicator).SyncSave`、`(*apptest.Replicator).SyncSave`、`apptest.Replicator.Stores`・`SendErrs`、`apptest.ReplicatorCall.Save`
  - `syncing.Service.Stores port.Stores`、`func (s *Service) Deliver(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) (syncstatus.Reason, error)`

- [ ] **Step 1: 失敗するテストを書く(`SaveError`)**

`internal/app/port/replicator_test.go`(新規)

```go
package port_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
)

func TestSaveErrorUnwraps(t *testing.T) {
	var err error = &port.SaveError{Err: port.ErrInvalidOperation}
	var se *port.SaveError
	if !errors.Is(err, port.ErrInvalidOperation) || !errors.As(err, &se) || errors.Is(err, port.ErrWriteDenied) {
		t.Errorf("SaveError = %v", err)
	}
}
```

- [ ] **Step 2: 失敗するテストを書く(gitstore の `SyncSave`。Review Focus 1)**

`internal/infra/gitstore/syncsave_test.go`(新規)。`newRemote`・`firstMachine`・`cloneMachine`・`sameFiles`・`snap`・`unchanged`・`set`・`t0` を使う。

```go
package gitstore

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

const movedItem item.ID = "01MOVEDIT0000000000000001"

var saveOp = port.Operation{Actor: change.User("taro"), Summary: "移動"}

// createMoved は、移動先に足すアイテムのフォルダを作る(移動先の保存の代わり)。
func createMoved(tx port.Tx) error {
	return tx.WriteItems().Append(movedItem, item.Create(item.Item{
		ID: movedItem, Place: item.Place{Workspace: "01SAMPLEWS0000000000000001", Number: 9}, Kind: item.Epic, Rank: "a", Title: "移ってきた", Watchers: []string{},
	}))
}

// 受け取って、保存して、送る。受け取ったものは保存の前に合わさっている。
func TestSyncSaveReceivesSavesAndSends(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	b.write(t, "b", set("rank", "a", "z"))
	b.sync(t, remote)

	res, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, createMoved)
	if err != nil || res.Sent != 1 || res.Received != 1 || res.Seen == "" {
		t.Fatalf("SyncSave = %+v, %v, want Sent 1, Received 1", res, err)
	}
	b.sync(t, remote)
	sameFiles(t, a, b)
	if _, ok := files(t, b.dir)["items/"+string(movedItem)+"/log.jsonl"]; !ok {
		t.Error("the other machine did not get the saved item")
	}
}

// (1) の受け取りが断られたら、保存しない。
func TestSyncSaveDoesNotSaveWhenReceiveFails(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	before := snap(t, a)
	called := 0
	_, err := a.rep.SyncSave(t.Context(), a.id, port.Remote{URL: "https://remote.invalid/missing.git"}, saveOp, func(tx port.Tx) error {
		called++
		return createMoved(tx)
	})
	var se *port.SaveError
	if err == nil || errors.As(err, &se) || called != 0 {
		t.Errorf("err = %v, fn called %d times, want a receive failure and no call", err, called)
	}
	unchanged(t, a, before)
}

// Review Focus 1: 送信が断られたら、移動先に、移したアイテムのフォルダもコミットも残らない。受け取ったものは残る。
func TestSyncSaveRefusedSendLeavesOnlyWhatWasReceived(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	b.write(t, "b", set("rank", "a", "z"))
	b.sync(t, remote)
	a.rep.beforeSend = func() error { return port.ErrWriteDenied }

	_, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, createMoved)
	var se *port.SaveError
	if !errors.Is(err, port.ErrWriteDenied) || errors.As(err, &se) {
		t.Fatalf("SyncSave = %v, want ErrWriteDenied (not a SaveError)", err)
	}
	if _, ok := files(t, a.dir)["items/"+string(movedItem)+"/log.jsonl"]; ok {
		t.Error("the saved item's folder is still there")
	}
	// 先端は、受け取って合わせた結果(b の保存)で、保存のコミットはない。ファイルも作業ツリーもそれと同じ。
	sameFiles(t, a, b)
}

// 送信が重なったら、取り込み直してから fn を呼び直す。fn は新しい状態を読む。
func TestSyncSaveRunsFnAgainAfterAnOverlap(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	once := false
	a.rep.beforeSend = func() error {
		if !once {
			once = true
			b.write(t, "b", set("rank", "a", "z"))
			b.sync(t, remote)
		}
		return nil
	}
	var seen []string
	_, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, func(tx port.Tx) error {
		it, err := tx.Items().Get("01SAMPLEIT0000000000000002")
		seen = append(seen, it.Rank)
		return errors.Join(err, createMoved(tx))
	})
	if err != nil || !reflect.DeepEqual(seen, []string{"V", "z"}) { // サンプルの順位は rank.First
		t.Fatalf("err = %v, fn saw ranks %v, want [V z] (2 回目は受け取った変更を読む)", err, seen)
	}
	b.sync(t, remote)
	sameFiles(t, a, b)
}

func TestSyncSaveGivesUpAfterThreeOverlaps(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	n := 0
	a.rep.beforeSend = func() error {
		n++
		b.write(t, "b", set("rank", "", string(rune('a'+n))))
		b.sync(t, remote)
		return nil
	}
	calls := 0
	_, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, func(tx port.Tx) error { calls++; return createMoved(tx) })
	if !errors.Is(err, port.ErrSendConflict) || n != 3 || calls != 3 {
		t.Errorf("err = %v after %d sends, fn called %d times, want ErrSendConflict, 3, 3", err, n, calls)
	}
	a.rep.beforeSend = nil
	a.sync(t, remote)
	b.sync(t, remote)
	sameFiles(t, a, b)
	if _, ok := files(t, a.dir)["items/"+string(movedItem)+"/log.jsonl"]; ok {
		t.Error("an abandoned save came back")
	}
}

func TestSyncSaveWrapsFnErrorsAndSendsNothing(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	boom := errors.New("boom")
	_, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, func(tx port.Tx) error { return boom })
	var se *port.SaveError
	if !errors.As(err, &se) || !errors.Is(err, boom) {
		t.Errorf("err = %v, want a SaveError around boom", err)
	}
	if res := b.sync(t, remote); res != (port.SyncResult{}) {
		t.Errorf("b received %+v, want nothing", res)
	}
}

func TestSyncSaveRefusesReceiveOnly(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	remote.ReceiveOnly = true
	before := snap(t, a)
	if _, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, createMoved); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("err = %v, want ErrInvalidOperation", err)
	}
	unchanged(t, a, before)
}
```

(サンプルの Task の順位は `rank.First`(`V`)。)

- [ ] **Step 3: 失敗するテストを書く(偽物)**

`internal/app/apptest/replicator_test.go` に足す。

```go
func TestReplicatorSyncSave(t *testing.T) {
	clock := apptest.NewClock(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC))
	stores := apptest.NewStores(clock, apptest.NewIDs())
	for _, id := range []port.StoreID{"S1", "S2", "S3"} {
		stores.AddStore(id)
	}
	boom := errors.New("boom")
	r := &apptest.Replicator{
		Stores:   stores,
		Results:  map[port.StoreID]port.SyncResult{"S1": {Sent: 1, Seen: "v9"}},
		Errs:     map[port.StoreID]error{"S3": port.ErrAuthFailed},
		SendErrs: map[port.StoreID]error{"S2": port.ErrWriteDenied},
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "追加"}
	join := func(tx port.Tx) error { return tx.WriteMembers().Append(member.Init(), member.Join("taro")) }
	logins := func(id port.StoreID) int {
		n := 0
		_ = stores.Read(t.Context(), id, func(r port.Reader) error { ms, _ := r.Members().List(); n = len(ms); return nil })
		return n
	}
	if res, err := r.SyncSave(t.Context(), "S1", port.Remote{URL: "u", Token: "tok"}, op, join); err != nil || res.Seen != "v9" || logins("S1") != 1 {
		t.Errorf("S1 = %+v, %v, members %d", res, err, logins("S1"))
	}
	if _, err := r.SyncSave(t.Context(), "S2", port.Remote{}, op, join); !errors.Is(err, port.ErrWriteDenied) || logins("S2") != 0 {
		t.Errorf("S2 = %v, members %d, want ErrWriteDenied and nothing saved", err, logins("S2"))
	}
	called := false
	if _, err := r.SyncSave(t.Context(), "S3", port.Remote{}, op, func(port.Tx) error { called = true; return nil }); !errors.Is(err, port.ErrAuthFailed) || called {
		t.Errorf("S3 = %v, fn called %v, want ErrAuthFailed and no call", err, called)
	}
	_, err := r.SyncSave(t.Context(), "S1", port.Remote{}, op, func(port.Tx) error { return boom })
	var se *port.SaveError
	if !errors.As(err, &se) || !errors.Is(err, boom) {
		t.Errorf("fn error = %v, want a SaveError around boom", err)
	}
	if calls := r.Calls(); len(calls) != 4 || !calls[0].Save || calls[0].Remote.Token != "tok" || calls[0].Mirror {
		t.Errorf("Calls = %+v", calls)
	}
}
```

imports に `change`・`member`・`time` を足す。

- [ ] **Step 4: 失敗するテストを書く(`Deliver`)**

`internal/app/syncing/deliver_test.go`(新規)。`newFixture`(S1 team: remote・ミラー・アカウントあり、S2 solo: remote なし、S3 docs: アカウントなし、S4 ro: 閲覧のみ)・`at` を使う。

```go
package syncing_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

var deliverOp = port.Operation{Actor: change.User("taro"), Summary: "移動"}

func addTaro(tx port.Tx) error { return tx.WriteMembers().Append(member.Init(), member.Join("taro")) }

func newDeliverFixture(t *testing.T) (*fixture, *apptest.Stores) {
	t.Helper()
	f := newFixture(t)
	stores := apptest.NewStores(apptest.NewClock(at), apptest.NewIDs())
	for _, id := range []port.StoreID{"S1", "S2", "S3", "S4"} {
		stores.AddStore(id)
	}
	f.svc.Stores = stores
	f.rep.Stores = stores
	return f, stores
}

func memberCount(t *testing.T, s port.Stores, id port.StoreID) int {
	t.Helper()
	n := 0
	if err := s.Read(t.Context(), id, func(r port.Reader) error { ms, err := r.Members().List(); n = len(ms); return err }); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDeliverSavesAndSendsWithTheStoresToken(t *testing.T) {
	f, stores := newDeliverFixture(t)
	reason, err := f.svc.Deliver(t.Context(), "S1", deliverOp, addTaro)
	if reason != "" || err != nil || memberCount(t, stores, "S1") != 1 {
		t.Fatalf("Deliver = %q, %v, members %d", reason, err, memberCount(t, stores, "S1"))
	}
	calls := f.rep.Calls()
	if len(calls) != 2 || !calls[0].Save || calls[0].Remote != (port.Remote{URL: "https://github.com/o/team.git", Token: "tok-taro"}) || !calls[1].Mirror {
		t.Errorf("calls = %+v, want a SyncSave then the mirror", calls)
	}
	rs, _ := f.state.Records(t.Context())
	if len(rs) != 1 || !rs[0].LastSuccess.Equal(at) || rs[0].Reason != "" {
		t.Errorf("records = %+v", rs)
	}
	if evs := f.events.List(); len(evs) != 1 || evs[0] != (port.Event{Kind: port.EventSync, Store: "S1"}) {
		t.Errorf("events = %+v", evs)
	}
	if got := f.afterCalls(); len(got) != 0 {
		t.Errorf("AfterSuccess = %v, want none", got)
	}
	// 次の Deliver には、受け取った位置を渡す。
	if _, err := f.svc.Deliver(t.Context(), "S1", deliverOp, func(port.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	var seen []string
	for _, c := range f.rep.Calls() {
		if c.Save {
			seen = append(seen, c.Remote.Seen)
		}
	}
	if !slices.Equal(seen, []string{"", "v2"}) {
		t.Errorf("Seen passed = %q", seen)
	}
}

// 送信が断られたら、理由を返し、記録し、何も保存しない。
func TestDeliverRefusedSendSavesNothingAndRecordsTheReason(t *testing.T) {
	f, stores := newDeliverFixture(t)
	f.rep.SendErrs = map[port.StoreID]error{"S1": port.ErrWriteDenied}
	reason, err := f.svc.Deliver(t.Context(), "S1", deliverOp, addTaro)
	if reason != syncstatus.WriteDenied || err != nil || memberCount(t, stores, "S1") != 0 {
		t.Fatalf("Deliver = %q, %v, members %d", reason, err, memberCount(t, stores, "S1"))
	}
	rs, _ := f.state.Records(t.Context())
	if len(rs) != 1 || rs[0].Reason != syncstatus.WriteDenied {
		t.Errorf("records = %+v", rs)
	}
	if got := f.tokens.Rejected(); len(got) != 0 {
		t.Errorf("Rejected = %v", got)
	}
	f.rep.SendErrs = map[port.StoreID]error{"S1": port.ErrAuthFailed}
	if reason, _ := f.svc.Deliver(t.Context(), "S1", deliverOp, addTaro); reason != syncstatus.AuthFailed {
		t.Errorf("reason = %q", reason)
	}
	if got := f.tokens.Rejected(); !slices.Equal(got, []port.StoreID{"S1"}) {
		t.Errorf("Rejected = %v, want [S1]", got)
	}
}

func TestDeliverWithoutRemoteJustWrites(t *testing.T) {
	f, stores := newDeliverFixture(t)
	reason, err := f.svc.Deliver(t.Context(), "S2", deliverOp, addTaro)
	if reason != "" || err != nil || memberCount(t, stores, "S2") != 1 || len(f.rep.Calls()) != 0 {
		t.Errorf("Deliver = %q, %v, members %d, calls %+v", reason, err, memberCount(t, stores, "S2"), f.rep.Calls())
	}
}

func TestDeliverWithoutAccountSavesNothing(t *testing.T) {
	f, stores := newDeliverFixture(t)
	reason, err := f.svc.Deliver(t.Context(), "S3", deliverOp, addTaro)
	rs, _ := f.state.Records(t.Context())
	if reason != syncstatus.NoAccount || err != nil || memberCount(t, stores, "S3") != 0 || len(f.rep.Calls()) != 0 || len(rs) != 0 {
		t.Errorf("Deliver = %q, %v, members %d, calls %+v, records %+v", reason, err, memberCount(t, stores, "S3"), f.rep.Calls(), rs)
	}
}

func TestDeliverRefusesAReadOnlyStore(t *testing.T) {
	f, stores := newDeliverFixture(t)
	_, err := f.svc.Deliver(t.Context(), "S4", deliverOp, addTaro)
	if !errors.Is(err, port.ErrInvalidOperation) || memberCount(t, stores, "S4") != 0 || len(f.rep.Calls()) != 0 {
		t.Errorf("Deliver = %v, members %d, calls %+v", err, memberCount(t, stores, "S4"), f.rep.Calls())
	}
}

// 保存の失敗は、理由にせず error で返し、同期の失敗として記録しない。
func TestDeliverReturnsSaveErrors(t *testing.T) {
	f, _ := newDeliverFixture(t)
	boom := errors.New("boom")
	reason, err := f.svc.Deliver(t.Context(), "S1", deliverOp, func(port.Tx) error { return boom })
	rs, _ := f.state.Records(t.Context())
	if reason != "" || !errors.Is(err, boom) || len(rs) != 0 {
		t.Errorf("Deliver = %q, %v, records %+v", reason, err, rs)
	}
	if _, err := f.svc.Deliver(t.Context(), "S9", deliverOp, addTaro); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("unknown store = %v", err)
	}
}
```

- [ ] **Step 5: 失敗を確かめる**

Run: `go test ./internal/app/port/... ./internal/app/apptest/... ./internal/app/syncing/... ./internal/infra/gitstore/ -run 'SaveError|SyncSave|Deliver'`
Expected: コンパイルエラー(`undefined: port.SaveError`・`r.SyncSave undefined`・`unknown field Stores` など)。

- [ ] **Step 6: 実装する(口と偽物)**

1. `port/replicator.go`: `Replicator` に `SyncSave`(コメント: 1 つのロックの中で、受け取り・保存・送信を行う。受け取りが失敗したら保存しない。送れなければ保存だけを捨てて、受け取って合わせた結果は残す。`fn` は、送信が重なって取り込み直すたびに呼ばれる。保存の失敗は `*SaveError`、通信の失敗は `Err*` のどれか)と `SaveError` を足す。
2. `apptest/sync.go`: 上の決めたとおり。`ReplicatorCall` に `Save bool`。
3. 偽物の `Stores.write` は Task 3 で作ってある。

- [ ] **Step 7: 実装する(gitstore)**

1. `replicator.go`: `Sync` の前置き(`dir`・`open`・`PlainOpen`・`Head`・`syncRun` の組み立て)を `func (r *Replicator) begin(id port.StoreID, remote port.Remote) (s syncRun, unlock func(), err error)` に出し、`Sync` はそれを使う(`Sync` のふるまいは変えない。既存のテストで確かめる)。`once` の前半(`fetch` から `integrate` まで)を `func (s syncRun) receive(ctx context.Context) (plumbing.Hash, error)`(返すのは remote の先端 `theirs`)に出し、`once` は `receive` のあと `receiveOnly`・`push` を続ける。
2. `write.go`: 次の 3 つを足す。

```go
// SyncSave は、1 つのロックの中で、受け取り・保存・送信を行う(port.Replicator)。
func (r *Replicator) SyncSave(ctx context.Context, id port.StoreID, remote port.Remote, op port.Operation, fn func(tx port.Tx) error) (port.SyncResult, error)

// saveOnce は、受け取り → 保存 → 送信を 1 回行う。送信が断られたら、保存のコミットを捨てる。
func (s syncRun) saveOnce(ctx context.Context, op port.Operation, fn func(tx port.Tx) error) (port.SyncResult, error)

// discard は、先端を base に戻し、作業ツリーを合わせる。
func (s syncRun) discard(base plumbing.Hash) error
```

`SyncSave`: `remote.ReceiveOnly` なら `port.ErrInvalidOperation` を包んで返す。`begin` のあと、`Sync` と同じ形で `maxSendTries` 回、`saveOnce` を呼ぶ(`errRejected` なら続ける。`port.ErrRemoteRewritten` なら `restoreSeen` を足す。超えたら `port.ErrSendConflict`)。`saveOnce`: `receive` の失敗はそのまま返す。`s.repo.Head()` を `base` として覚える。`s.r.stores.writeLocked(s.root, op, fn)` の失敗は `&port.SaveError{Err: err}`。`s.push(ctx, theirs)`(`beforeSend` のフックを含む)の失敗は、`errors.Join(err, s.discard(base))` で返す(`errors.Is(…, errRejected)` は Join を通る)。成功なら `s.result(theirs, head, true)`。`discard`: `s.repo.Worktree()` を得て、`s.guarded(func() error { return rollback(wt, base) })`。

- [ ] **Step 8: 実装する(`syncing`)**

1. `service.go`: `Service` に `Stores port.Stores` を足す(コメント: `Deliver` が使う。`NotifyWrites` で包む前のもの)。
2. `run.go`: `token` を出して `run` が使う。ふるまいは変えない。
3. `deliver.go`(新規): 上の「決めること」のとおりに `Deliver` を書く。`once` の「動いている同期を待つ」仕組みは使わない(保存を共有できない)。

- [ ] **Step 9: 通す**

Run: `go test ./internal/app/... ./internal/infra/gitstore/... && wc -l internal/infra/gitstore/replicator.go internal/infra/gitstore/write.go internal/app/apptest/sync.go internal/app/syncing/deliver.go && golangci-lint run ./internal/...`
Expected: PASS(既存の `Sync`・`run` のテストも)。`wc -l` はどれも 300 以下。lint なし。

- [ ] **Step 10: コミットする**

```bash
git add internal/app/port internal/app/apptest internal/app/syncing internal/infra/gitstore
git commit -m "$(cat <<'EOF'
feat: 受け取り・保存・送信を 1 つのロックで行い、送れなければ保存だけを捨てる SyncSave と、それを呼ぶ Deliver を足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

---

### Task 5: `app/transfers` — 候補・予告・同じ Store の中の移動

> **この Task への上書き**(「本文より優先する直し」): `Service` の欄は「名前の直し」のとおり。予告のステータスの置き換えは `transfer.Unmatched` を使う。


決定 A5a-3・A5a-7(の呼び出し側)・A5a-9・A5a-11・A5a-13・A5a-14 を実装する。Task 2(`transfer.Remap`・`UsableStatuses`)と Task 3(口)に依存する。このタスクで `internal/app/transfers` を作る(1 ファイル 1 操作)。

**Files:**
- Create: `internal/app/transfers/service.go`(`Service`・`Delivery`・読む補助)・`targets.go`・`preview.go`・`move.go`
- Create(テスト): `internal/app/transfers/helpers_test.go`・`targets_test.go`・`preview_test.go`・`move_test.go`

`internal/app/transfers` は 4 ファイル(Task 6 が `move_store.go`・`names.go`、Task 7 が `locate.go` を足して 7)。app の他の機能(`app/items`・`app/syncing` など)は、テストを含めて import しない。

**Interfaces:**
- Consumes:
  - `transfer.Collect(root item.ID, all []item.Item) (transfer.Set, error)`、`transfer.Number(root item.Item, below []item.Item, next int) map[item.ID]int`、`transfer.Remap(it item.Item, from, to workspace.Settings, people transfer.PeopleMap, statusChoice map[change.ID]change.ID) transfer.RemapResult`(`people` が `nil` のときは同じ Store として、人の値をそのまま残す。指摘 3)、`transfer.UsableStatuses(to workspace.Settings, p project.ID) []change.ID`、`transfer.Record`(A5a-1)
  - `port.Tx.WriteTransfers() port.TransferWriter`(`Append(r transfer.Record) error`)、`port.Reader.Transfers()`(Task 3)
  - 既存: `port.LiveItem`・`port.LiveProject`・`port.LiveProjects`・`port.LiveItems`、`numbering.Next`・`numbering.Draft`、`workspace.RankOf`、`rank.After`、`item.Descendants`・`item.SortSiblings`・`item.Live`、`refusal.New`
- Produces:
  - `transfers.Delivery`(`Deliver(ctx, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) (syncstatus.Reason, error)`)と
    `transfers.Service{Stores port.Stores; StoreList port.StoreList; Targets port.SyncTargets; Delivery Delivery; Identity port.Identity; Clock port.Clock}`(`Me`・`IDs` は持たない。指摘 1)
  - `transfers.Target{Store port.StoreInfo; Workspace workspace.Workspace; Projects []project.Project; Epics map[project.ID][]item.Item}`、`(Service) Targets(ctx, store port.StoreID, id item.ID) ([]Target, error)`
  - `transfers.StatusRow{Status change.ID; Name string; Count int; Choices []workspace.Value; Default change.ID}`、
    `transfers.Preview{Below, Deleted int; NextID string; AcrossStores, RemoteNote bool; Statuses []StatusRow}`、
    `(Service) Preview(ctx, store port.StoreID, id item.ID, in MoveInput) (Preview, error)`(`in` の `ToParent`・`StatusChoices` は使わない)
  - `transfers.MoveInput{ToStore port.StoreID; ToWorkspace workspace.ID; ToProject project.ID; ToParent *item.ID; StatusChoices map[change.ID]change.ID}`、`transfers.Moved{ToStore port.StoreID; Item item.ID; Display string; Emptied []string}`、
    `(Service) Move(ctx, store port.StoreID, id item.ID, in MoveInput) (Moved, error)`。このタスクは `in.ToStore == store` だけを扱い、ほかは `port.ErrStoreNotFound`(Task 6 が差し替える)
  - ログに書く行(同じ Store): 移すアイテムごとに `set place`(ワークスペース・プロジェクト・親・新しい番号)、移すアイテムだけに `set rank`、`Remap` の行。移動先のワークスペースのログに `number` の行を番号ごとに。`transfers.jsonl` にアイテムごとに 1 行(`Entity: "item"`・`Target`・`Old`・`New`・`ToStore` は自分の Store ID、`ToStoreName`・`ToRemote` は空)。すべて 1 回の `Write`
  - アクティビティ(A5a-11): 「{旧 ID} を {移動元のワークスペース名} から {移動先のワークスペース名} へ移動」(例: 「HH-2 を Web から Mobile へ移動」)

**決めごと(骨組みの決定を実装に落としたもの):**
- 移せるもの: 削除されていない Epic・Story・Task・Bug。Step・ないもの・削除済みは `port.ErrNotFound`(Step は親と一緒に移る。02)。移す先のワークスペース・プロジェクト・親が、ないか削除済み、プロジェクトが移動先のワークスペースのものでない、移動元と同じワークスペース、Epic に `ToParent` がある、Epic 以外で `ToParent` が移動先のプロジェクトの削除されていない Epic でない、のどれかなら `port.ErrNotFound`。
- 移動先の Store が `StoreList` にない(非表示を含む)なら `port.ErrStoreNotFound`、閲覧のみなら `port.ErrNotFound`(候補に出さないものは選べない)。移動元の Store が閲覧のみなら `refusal.New("閲覧のみの Store です")`。
- 読む処理は 1 つの関数(`plan`)にし、`Read` でアクティビティの 1 行を決めるときと `Write` の中で書くときの両方で呼ぶ(A4b の `items.Move` と同じ)。`plan` が返すのは、移すもの(`transfer.Set`)・番号(`transfer.Number`)・移動元と移動先の設定・`Remap` の結果。移動先の次の番号は `numbering.Next(移動先のワークスペースのログ, numbering.Items)` を、`Write` の中の読み取りで取る。
- 移すアイテムの順位は、移動先の同じ親(Epic ならプロジェクト)の、削除されていない子の最後のあと(`rank.After(最後の順位)`。子がなければ `rank.After("")`)。下のものの順位・親は変えない。
- `Moved.Display` は移したアイテムの新しい ID(移動先の接頭辞と番号)、`Moved.Item` は内部 ID のまま(同じ Store では内部 ID は変わらない)。`Emptied` は、移したすべてのアイテムの `RemapResult.Emptied` の和集合を、移動元のワークスペースのプロパティの定義の順に並べたもの。
- 移動先の「ステータスの置き換え」の行(`StatusRow`)の対象は、移すアイテム(下のもの・削除済みを含む)のステータスのうち、名前が `UsableStatuses(移動先, プロジェクト)` のどの値の名前とも一致しないもの。行の並びは移動元のステータスの定義の順、`Count` はそのステータスのアイテム数、`Choices` は `UsableStatuses` の値、`Default` は移動先の `CategoryDefaults[元のステータスの分類]`。`Remap` と同じ判定なので、変えるときは両方を直す(指摘 3)。
- `Targets` の対象は、`StoreList` の閲覧のみでない Store の、削除されていないプロジェクトが 1 つ以上ある、自分のワークスペース(`store` と同じ Store の移動元のワークスペース)以外のワークスペース。Store の並び → ワークスペースの並び(`workspace.RankOf(ログ)` の昇順、同じなら内部 ID の順)。`Epics` は削除されていない Epic を `item.SortSiblings` の順で。Store のワークスペースの非表示は Store 単位で `StoreList` が既に除く(指摘 6)。
- `Preview.NextID` は移動先の接頭辞と次の番号(例「MB-2」)。`AcrossStores` は `in.ToStore != store`。`RemoteNote` は、別の Store で、`Targets` の移動先に remote がなく移動元に remote があるとき。

#### 5-1 テストの補助

- [ ] **Step 1: `internal/app/transfers/helpers_test.go` を書く**

テストの補助は、Task 6・7 も使う。偽の `Stores` を `recorder` で包んでアクティビティの文を覚え、`Delivery` の偽物(`stubDelivery`)は「保存して、送れなかったら保存を捨てる」を `Stores.Write` の中のエラーで真似る。

```go
package transfers_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// S1「main」はサンプル(Web・HH)と Mobile(MB)、S2「team」は Team(TM)、S3「readonly」は閲覧のみで Archive(AR)。
// S1 は remote あり、S2・S3 はなし。S1 の「あなた」は hanako。
const (
	store1 port.StoreID = "S1"
	store2 port.StoreID = "S2"
	store3 port.StoreID = "S3"

	mobileWS      workspace.ID = "01TESTWS000000000000000002"
	mobileProject project.ID   = "01TESTPR000000000000000002"
	mobileEpic    item.ID      = "01TESTIT000000000000000001"
	teamWS        workspace.ID = "01TESTWS000000000000000003"
	teamProject   project.ID   = "01TESTPR000000000000000003"
	teamEpic      item.ID      = "01TESTIT000000000000000002"
	archiveWS     workspace.ID = "01TESTWS000000000000000004"
	archiveProj   project.ID   = "01TESTPR000000000000000004"
	archiveEpic   item.ID      = "01TESTIT000000000000000003"
)

type activity struct {
	Store   port.StoreID
	Summary string
}

// recorder は、Write に渡したアクティビティの 1 行を覚える(複数の goroutine から書いてよい)。
type recorder struct {
	*apptest.Stores
	mu  sync.Mutex
	got []activity
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	r.mu.Lock()
	r.got = append(r.got, activity{id, op.Summary})
	r.mu.Unlock()
	return r.Stores.Write(ctx, id, op, fn)
}

// summaries は、Store に書いたアクティビティの文を、書いた順に返す。
func (r *recorder) summaries(id port.StoreID) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, a := range r.got {
		if a.Store == id {
			out = append(out, a.Summary)
		}
	}
	return out
}

// stubDelivery は Delivery の偽物。reasons に理由を置いた Store は、保存を捨てて理由を返す(送れなかった)。
// gate は Deliver の最初に呼ぶ(向きの逆の 2 つの移動を重ねるため)。
type stubDelivery struct {
	stores  port.Stores
	reasons map[port.StoreID]syncstatus.Reason
	gate    func(port.StoreID)
}

var errNotSent = errors.New("not sent")

func (d *stubDelivery) Deliver(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) (syncstatus.Reason, error) {
	if d.gate != nil {
		d.gate(id)
	}
	r := d.reasons[id]
	err := d.stores.Write(ctx, id, op, func(tx port.Tx) error {
		if err := fn(tx); err != nil {
			return err
		}
		if r != "" {
			return errNotSent // fn の保存を残さない
		}
		return nil
	})
	if errors.Is(err, errNotSent) {
		return r, nil
	}
	return "", err
}

// newBarrier は、n 回目の呼び出しが来るまで、呼び出しを待たせる。
func newBarrier(n int) func(port.StoreID) {
	var mu sync.Mutex
	arrived, open := 0, make(chan struct{})
	return func(port.StoreID) {
		mu.Lock()
		arrived++
		if arrived == n {
			close(open)
		}
		mu.Unlock()
		<-open
	}
}

type fixture struct {
	svc     transfers.Service
	rec     *recorder
	list    *apptest.StoreList
	deliver *stubDelivery
}

func newFixture(t *testing.T) fixture { return newFixtureWith(t, nil) }

// newFixtureWith は、mobile で Mobile の設定を変えた(例: ステータスを減らす、優先度の名前を変える)フィクスチャ。
func newFixtureWith(t *testing.T, mobile func(*workspace.Settings)) fixture {
	t.Helper()
	ids := apptest.NewIDs()
	stores := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	rec := &recorder{Stores: stores}
	for _, id := range []port.StoreID{store1, store2, store3} {
		stores.AddStore(id)
	}
	if err := apptest.Sample(t.Context(), stores, store1); err != nil {
		t.Fatal(err)
	}
	addWorkspace(t, rec, store1, workspace.Workspace{ID: mobileWS, Name: "Mobile", Prefix: "MB"}, mobileProject, mobileEpic, mobile)
	addMembers(t, rec, store2, "Taro") // 大文字小文字だけが違う綴り
	addWorkspace(t, rec, store2, workspace.Workspace{ID: teamWS, Name: "Team", Prefix: "TM"}, teamProject, teamEpic, nil)
	addMembers(t, rec, store3, "taro")
	addWorkspace(t, rec, store3, workspace.Workspace{ID: archiveWS, Name: "Archive", Prefix: "AR"}, archiveProj, archiveEpic, nil)
	list := &apptest.StoreList{Stores: []port.StoreInfo{
		{ID: store1, Name: "main"}, {ID: store2, Name: "team"}, {ID: store3, Name: "readonly", ReadOnly: true},
	}}
	deliver := &stubDelivery{stores: rec}
	svc := transfers.Service{
		Stores: rec, StoreList: list, Delivery: deliver, Identity: apptest.Identity{Login: "hanako"},
		Clock: apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)),
		Targets: &apptest.SyncTargets{Targets: []port.SyncTarget{
			{ID: store1, Name: "main", Remote: "https://example.invalid/main.git"}, {ID: store2, Name: "team"}, {ID: store3, Name: "readonly", ReadOnly: true},
		}},
	}
	return fixture{svc: svc, rec: rec, list: list, deliver: deliver}
}

func write(t *testing.T, s port.Stores, store port.StoreID, login string, fn func(tx port.Tx) error) {
	t.Helper()
	op := port.Operation{Actor: change.User(login), Summary: "テスト"}
	if err := s.Write(t.Context(), store, op, fn); err != nil {
		t.Fatal(err)
	}
}

func addMembers(t *testing.T, s port.Stores, store port.StoreID, logins ...string) {
	t.Helper()
	write(t, s, store, "taro", func(tx port.Tx) error {
		ds := []change.Draft{member.Init()}
		for _, l := range logins {
			ds = append(ds, member.Join(l))
		}
		return tx.WriteMembers().Append(ds...)
	})
}

// addWorkspace は、ワークスペースと、プロジェクト 1 つ(「アプリ」・番号 1)と Epic 1 つ(「iOS」・番号 1)を足す。
// pr が空ならワークスペースだけ。値の内部 ID は ws の接頭辞ごとに別(例 VALMB001)。mutate があれば設定を変えてから作る。
func addWorkspace(t *testing.T, s port.Stores, store port.StoreID, ws workspace.Workspace, pr project.ID, epic item.ID, mutate func(*workspace.Settings)) {
	t.Helper()
	n := 0
	st := workspace.DefaultSettings(func() change.ID { n++; return change.ID(fmt.Sprintf("VAL%s%03d", ws.Prefix, n)) })
	if mutate != nil {
		mutate(&st)
	}
	d, err := workspace.CreateWithSettings(ws, st)
	if err != nil {
		t.Fatal(err)
	}
	write(t, s, store, "taro", func(tx port.Tx) error {
		if err := tx.WriteWorkspaces().Append(ws.ID, d); err != nil || pr == "" {
			return err
		}
		p := project.Project{ID: pr, Place: project.Place{Workspace: ws.ID, Number: 1}, Rank: "V", Title: "アプリ", Status: project.Todo}
		e := item.Item{ID: epic, Place: item.Place{Workspace: ws.ID, Project: pr, Number: 1}, Kind: item.Epic, Rank: "V", Title: "iOS"}
		return errors.Join(
			tx.WriteWorkspaces().Append(ws.ID, numbering.Draft(numbering.Projects, 1), numbering.Draft(numbering.Items, 1)),
			tx.WriteProjects().Append(pr, project.Create(p)),
			tx.WriteItems().Append(epic, item.Create(e)),
		)
	})
}

// addItem は、アイテムを作る(作成の行と、番号の行)。
func addItem(t *testing.T, s port.Stores, store port.StoreID, it item.Item) item.Item {
	t.Helper()
	write(t, s, store, "taro", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(it.ID, item.Create(it)),
			tx.WriteWorkspaces().Append(it.Place.Workspace, numbering.Draft(numbering.Items, it.Place.Number)),
		)
	})
	return it
}

func addRelation(t *testing.T, s port.Stores, store port.StoreID, from, to item.ID, typ string) {
	t.Helper()
	write(t, s, store, "taro", func(tx port.Tx) error {
		return tx.WriteItems().Append(from, change.NewAdd("relations", item.Relation{Type: typ, To: to}))
	})
}

func itemOf(t *testing.T, s port.Stores, store port.StoreID, id item.ID) item.Item {
	t.Helper()
	var it item.Item
	if err := s.Read(t.Context(), store, func(r port.Reader) (err error) { it, err = r.Items().Get(id); return }); err != nil {
		t.Fatal(err)
	}
	return it
}

func logOf(t *testing.T, s port.Stores, store port.StoreID, ref change.Ref) []change.Record {
	t.Helper()
	var log []change.Record
	if err := s.Read(t.Context(), store, func(r port.Reader) (err error) { log, err = r.Changes().Of(ref); return }); err != nil {
		t.Fatal(err)
	}
	return log
}

func recordsOf(t *testing.T, s port.Stores, store port.StoreID) []transfer.Record {
	t.Helper()
	var rs []transfer.Record
	if err := s.Read(t.Context(), store, func(r port.Reader) (err error) { rs, err = r.Transfers().List(); return }); err != nil {
		t.Fatal(err)
	}
	return rs
}

// nextNumber は、ワークスペースの次の番号。
func nextNumber(t *testing.T, s port.Stores, store port.StoreID, ws workspace.ID) int {
	t.Helper()
	return numbering.Next(logOf(t, s, store, change.Ref{Kind: change.KindWorkspace, ID: ws}), numbering.Items)
}

// valueID は、ワークスペースのステータス(kind が "status")か優先度("priority")の、名前の値の内部 ID。
func valueID(t *testing.T, s port.Stores, store port.StoreID, ws workspace.ID, kind, name string) change.ID {
	t.Helper()
	var st workspace.Settings
	if err := s.Read(t.Context(), store, func(r port.Reader) (err error) { st, err = r.Workspaces().Settings(ws); return }); err != nil {
		t.Fatal(err)
	}
	vs := st.Statuses
	if kind == "priority" {
		vs = st.Priorities
	}
	for _, v := range vs {
		if v.Name == name {
			return v.ID
		}
	}
	t.Fatalf("%s %q がない", kind, name)
	return ""
}

func ptr(id item.ID) *item.ID { return &id }
```

- [ ] **Step 2: コンパイルできることを確かめる**

`internal/app/transfers` にまだ本体がないので、この時点では `transfers.Service` が見つからず落ちる。次の Step のテストを書いてから、5-4 で通す。

#### 5-2 候補と予告

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/transfers/targets_test.go`:

```go
package transfers_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 候補は、閲覧のみの Store(readonly の Archive)と自分のワークスペース(Web)を除き、Store の並び → ワークスペースの並び。
func TestTargetsExcludeOwnWorkspaceReadOnlyStoresAndEmptyWorkspaces(t *testing.T) {
	f := newFixture(t)
	// プロジェクトのない、プロジェクトが削除済みのワークスペース(team の Empty)も出さない。
	addWorkspace(t, f.rec, store2, workspace.Workspace{ID: "01TESTWS000000000000000009", Name: "Empty", Prefix: "EM"}, "01TESTPR000000000000000009", "01TESTIT000000000000000009", nil)
	if err := apptest.DeleteProject(t.Context(), f.rec, store2, "01TESTPR000000000000000009"); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Targets(t.Context(), store1, apptest.SampleTask)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tg := range got {
		names = append(names, tg.Store.Name+"/"+tg.Workspace.Name)
	}
	if want := []string{"main/Mobile", "team/Team"}; !slices.Equal(names, want) {
		t.Fatalf("targets = %v, want %v", names, want)
	}
	mobile := got[0]
	if len(mobile.Projects) != 1 || mobile.Projects[0].ID != mobileProject || len(mobile.Epics[mobileProject]) != 1 || mobile.Epics[mobileProject][0].ID != mobileEpic {
		t.Errorf("Mobile の項目 = %+v", mobile)
	}
}

// Step は単独では移せない。ないもの・削除済みも同じ。
func TestTargetsOfAStepOrADeletedItemIsNotFound(t *testing.T) {
	f := newFixture(t)
	parent := apptest.SampleTask
	step := addItem(t, f.rec, store1, item.Item{ID: "01TESTIT000000000000000010", Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &parent, Number: 3}, Kind: item.Step, Rank: "V", Title: "下書き"})
	if _, err := f.svc.Targets(t.Context(), store1, step.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Step: %v", err)
	}
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleTask, step.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Targets(t.Context(), store1, apptest.SampleTask); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除済み: %v", err)
	}
}
```

`internal/app/transfers/preview_test.go`。Mobile からステータス Pending を除き、Task の下に、Pending の Step(生きている)と削除済みの Step を置く:

```go
package transfers_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func withoutPending(st *workspace.Settings) {
	st.Statuses = slices.DeleteFunc(st.Statuses, func(v workspace.Value) bool { return v.Name == "Pending" })
}

// 予告: 下の削除されていない件数 1・削除済み 1、移動先の次の番号の ID、置き換えの行(Pending だけ。生きている Step 1 件と削除済みの Step 1 件で Count は 2)。
func TestPreviewCountsAndStatusRows(t *testing.T) {
	f := newFixtureWith(t, withoutPending)
	parent := apptest.SampleTask
	mk := func(id item.ID, n int, status change.ID) item.Item {
		return addItem(t, f.rec, store1, item.Item{ID: id, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &parent, Number: n}, Kind: item.Step, Rank: "V", Title: "s", Status: status})
	}
	mk("01TESTIT000000000000000010", 3, apptest.SampleStatusPending)
	gone := mk("01TESTIT000000000000000011", 4, apptest.SampleStatusPending)
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, gone.ID); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Preview(t.Context(), store1, apptest.SampleTask, transfers.MoveInput{ToStore: store1, ToWorkspace: mobileWS, ToProject: mobileProject})
	if err != nil {
		t.Fatal(err)
	}
	if got.Below != 1 || got.Deleted != 1 || got.NextID != "MB-2" || got.AcrossStores || got.RemoteNote {
		t.Errorf("preview = %+v", got)
	}
	todo := valueID(t, f.rec, store1, mobileWS, "status", "Todo")
	if len(got.Statuses) != 1 {
		t.Fatalf("rows = %+v", got.Statuses)
	}
	row := got.Statuses[0]
	if row.Status != apptest.SampleStatusPending || row.Name != "Pending" || row.Count != 2 || row.Default != todo || len(row.Choices) != 4 {
		t.Errorf("row = %+v", row)
	}
}

// 別の Store へ: 移動元に remote があり、移動先(team)にない → remote の注意を出す。
func TestPreviewAcrossStoresWarnsWhenOnlyTheSourceHasARemote(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.Preview(t.Context(), store1, apptest.SampleTask, transfers.MoveInput{ToStore: store2, ToWorkspace: teamWS, ToProject: teamProject})
	if err != nil || !got.AcrossStores || !got.RemoteNote || got.NextID != "TM-2" {
		t.Errorf("preview = %+v, %v", got, err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

`go test ./internal/app/transfers/` → `undefined: transfers.Service` などのコンパイルエラーで落ちる。

#### 5-3 同じ Store の中の移動

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/transfers/move_test.go`。まず Review Focus 4(削除済みの子)を含む基本。生きている Step を後に、削除済みの Step を先に作って、削除済みの子が生きている子のあとの番号になることを確かめる。

```go
package transfers_test

import (
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/refusal"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func toMobile() transfers.MoveInput {
	return transfers.MoveInput{ToStore: store1, ToWorkspace: mobileWS, ToProject: mobileProject, ToParent: ptr(mobileEpic)}
}

const (
	stepGoneID item.ID = "01TESTIT000000000000000010"
	stepLiveID item.ID = "01TESTIT000000000000000011"
)

// Task HH-2 の下に、削除済みの Step(HH-3)→生きている Step(HH-4)の順に作る。
func withSteps(t *testing.T, f fixture) {
	t.Helper()
	parent := apptest.SampleTask
	for i, id := range []item.ID{stepGoneID, stepLiveID} {
		addItem(t, f.rec, store1, item.Item{ID: id, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &parent, Number: 3 + i}, Kind: item.Step, Rank: "V", Title: "s"})
	}
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, stepGoneID); err != nil {
		t.Fatal(err)
	}
}

func TestMoveToAnotherWorkspaceRenumbersDeletedChildrenAfterLiveOnes(t *testing.T) {
	f := newFixture(t)
	withSteps(t, f)
	deletedAt := itemOf(t, f.rec, store1, stepGoneID).Deleted.At
	got, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile())
	if err != nil {
		t.Fatal(err)
	}
	if got.Display != "MB-2" || got.Item != apptest.SampleTask || got.ToStore != store1 || len(got.Emptied) != 0 {
		t.Errorf("moved = %+v", got)
	}
	// 移すアイテムが MB-2、生きている子が MB-3、削除済みの子が MB-4(作った順では逆)。
	for id, want := range map[item.ID]int{apptest.SampleTask: 2, stepLiveID: 3, stepGoneID: 4} {
		it := itemOf(t, f.rec, store1, id)
		if it.Place.Workspace != mobileWS || it.Place.Project != mobileProject || it.Place.Number != want {
			t.Errorf("%s の場所 = %+v, 番号は %d", id, it.Place, want)
		}
	}
	task := itemOf(t, f.rec, store1, apptest.SampleTask)
	if task.Place.Parent == nil || *task.Place.Parent != mobileEpic || task.Rank <= "V" {
		t.Errorf("移すアイテムの親と順位 = %v, %q(Mobile の Epic の子の末尾)", task.Place.Parent, task.Rank)
	}
	if step := itemOf(t, f.rec, store1, stepLiveID); *step.Place.Parent != apptest.SampleTask || step.Rank != "V" {
		t.Errorf("下のものの親と順位は変えない: %+v", step.Place)
	}
	// 削除済みのまま、元の削除日時。
	if gone := itemOf(t, f.rec, store1, stepGoneID); gone.Deleted == nil || !gone.Deleted.At.Equal(deletedAt) {
		t.Errorf("削除済みの印 = %+v", gone.Deleted)
	}
	// 番号は移動先のワークスペースの次の番号から使い切る。移動元の番号は戻らない。
	if n := nextNumber(t, f.rec, store1, mobileWS); n != 5 {
		t.Errorf("Mobile の次の番号 = %d, want 5", n)
	}
	if n := nextNumber(t, f.rec, store1, apptest.SampleWorkspace); n != 5 {
		t.Errorf("Web の次の番号 = %d(再利用しない), want 5", n)
	}
}

// 1 回の保存、転送記録 3 件(アイテムごと)、アクティビティ。
func TestMoveWithinStoreIsOneSaveWithTransferRecords(t *testing.T) {
	f := newFixture(t)
	withSteps(t, f)
	before := len(f.rec.summaries(store1))
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
		t.Fatal(err)
	}
	if got := f.rec.summaries(store1)[before:]; !slices.Equal(got, []string{"HH-2 を Web から Mobile へ移動"}) {
		t.Errorf("書いたアクティビティ = %q", got)
	}
	recs := recordsOf(t, f.rec, store1)
	byTarget := map[item.ID]string{}
	for _, r := range recs {
		byTarget[r.Target] = r.Old + ">" + r.New
		if r.Entity != "item" || r.ToStore != string(store1) || r.ToStoreName != "" || r.ToRemote != "" || r.By != "hanako" {
			t.Errorf("記録 = %+v", r)
		}
	}
	want := map[item.ID]string{apptest.SampleTask: "HH-2>MB-2", stepLiveID: "HH-4>MB-3", stepGoneID: "HH-3>MB-4"}
	if len(recs) != 3 || !maps.Equal(byTarget, want) {
		t.Errorf("転送記録 = %v", byTarget)
	}
	// 移すアイテムの place の行と記録は同じ操作。
	log := logOf(t, f.rec, store1, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
	var placeOp change.ID
	for _, r := range log {
		if r.T == change.Set && r.F == "place" {
			placeOp = r.Op
		}
	}
	for _, r := range recs {
		if r.Op != placeOp {
			t.Errorf("記録の操作 %s と place の操作 %s が違う", r.Op, placeOp)
		}
	}
}

// Epic は、プロジェクトの直下(親なし)に移り、下の Task もついて行く。
func TestMoveEpicIntoAnotherWorkspaceTakesItsTasks(t *testing.T) {
	f := newFixture(t)
	in := transfers.MoveInput{ToStore: store1, ToWorkspace: mobileWS, ToProject: mobileProject}
	got, err := f.svc.Move(t.Context(), store1, apptest.SampleEpic, in)
	if err != nil || got.Display != "MB-2" {
		t.Fatalf("moved = %+v, %v", got, err)
	}
	epic, task := itemOf(t, f.rec, store1, apptest.SampleEpic), itemOf(t, f.rec, store1, apptest.SampleTask)
	if epic.Place.Parent != nil || epic.Place.Number != 2 || epic.Rank <= "V" || task.Place.Number != 3 || *task.Place.Parent != apptest.SampleEpic || task.Place.Workspace != mobileWS {
		t.Errorf("epic = %+v, task = %+v", epic, task)
	}
}
```

値の引き直し: Mobile のステータスから Pending を除き、優先度「Medium」を「Normal」に改名する(合う名前がなくなる)。Task の下の Step に Pending を持たせる。

```go
func TestMoveRemapsValuesAndReportsEmptiedOnes(t *testing.T) {
	f := newFixtureWith(t, func(st *workspace.Settings) {
		withoutPending(st)
		st.Priorities[2].Name = "Normal" // Medium が合わなくなる。初期優先度はこの値
	})
	write(t, f.rec, store1, "taro", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask,
			change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusPending),
			change.NewSet("assignee", nil, "hanako"))
	})
	inProgress := valueID(t, f.rec, store1, mobileWS, "status", "InProgress")
	in := toMobile()
	in.StatusChoices = map[change.ID]change.ID{apptest.SampleStatusPending: inProgress}
	got, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, in)
	if err != nil {
		t.Fatal(err)
	}
	it := itemOf(t, f.rec, store1, apptest.SampleTask)
	if it.Status != inProgress {
		t.Errorf("ステータスは選んだ置き換え: %q", it.Status)
	}
	if it.Priority != valueID(t, f.rec, store1, mobileWS, "priority", "Normal") || !slices.Equal(got.Emptied, []string{"優先度"}) {
		t.Errorf("優先度 = %q, emptied = %v(初期優先度にして「優先度」を並べる)", it.Priority, got.Emptied)
	}
	if it.Assignee != "hanako" || !slices.Equal(it.Watchers, []string{"taro"}) {
		t.Errorf("同じ Store の人はそのまま: %q, %v", it.Assignee, it.Watchers)
	}
}

// 置き換えを渡さないと、同じ分類(Pending は未着手)の移動先の既定(Todo)。
func TestMoveWithoutAStatusChoiceUsesTheDefaultOfTheSameCategory(t *testing.T) {
	f := newFixtureWith(t, withoutPending)
	write(t, f.rec, store1, "taro", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusPending))
	})
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
		t.Fatal(err)
	}
	if it := itemOf(t, f.rec, store1, apptest.SampleTask); it.Status != valueID(t, f.rec, store1, mobileWS, "status", "Todo") {
		t.Errorf("status = %q", it.Status)
	}
}

func TestMoveRefusesWhatCannotBeChosen(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	same := transfers.MoveInput{ToStore: store1, ToWorkspace: apptest.SampleWorkspace, ToProject: apptest.SampleProject, ToParent: ptr(apptest.SampleEpic)}
	cases := []struct {
		name string
		id   item.ID
		in   transfers.MoveInput
		want error
	}{
		{"同じワークスペース", apptest.SampleTask, same, port.ErrNotFound},
		{"Task に親がない", apptest.SampleTask, transfers.MoveInput{ToStore: store1, ToWorkspace: mobileWS, ToProject: mobileProject}, port.ErrNotFound},
		{"Epic に親がある", apptest.SampleEpic, toMobile(), port.ErrNotFound},
		{"プロジェクトが別のワークスペースのもの", apptest.SampleTask, transfers.MoveInput{ToStore: store1, ToWorkspace: mobileWS, ToProject: apptest.SampleProject, ToParent: ptr(mobileEpic)}, port.ErrNotFound},
		{"親が移動先のプロジェクトの Epic でない", apptest.SampleTask, transfers.MoveInput{ToStore: store1, ToWorkspace: mobileWS, ToProject: mobileProject, ToParent: ptr(apptest.SampleEpic)}, port.ErrNotFound},
		{"閲覧のみの Store", apptest.SampleTask, transfers.MoveInput{ToStore: store3, ToWorkspace: archiveWS, ToProject: archiveProj, ToParent: ptr(archiveEpic)}, port.ErrNotFound},
		{"表示中でない Store", apptest.SampleTask, transfers.MoveInput{ToStore: "S9", ToWorkspace: mobileWS, ToProject: mobileProject, ToParent: ptr(mobileEpic)}, port.ErrStoreNotFound},
	}
	for _, c := range cases {
		before := len(f.rec.summaries(store1))
		if _, err := f.svc.Move(ctx, store1, c.id, c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
		if len(f.rec.summaries(store1)) != before {
			t.Errorf("%s: 何も保存しない", c.name)
		}
	}
	f.list.Stores[0].ReadOnly = true // 移動元が閲覧のみ
	var r *refusal.Refusal
	if _, err := f.svc.Move(ctx, store1, apptest.SampleTask, toMobile()); !errors.As(err, &r) || r.Message != "閲覧のみの Store です" {
		t.Errorf("移動元が閲覧のみ: %v", err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

`go test ./internal/app/transfers/` → `undefined: transfers.Service`(まだ本体がない)で落ちる。

- [ ] **Step 3: 実装する**

- `service.go`: `Delivery`・`Service`(上の Produces のとおり)。`allWorkspaceItems`ではなく、この Task が読むのは移すアイテムのプロジェクトの `Items().List`(削除済みを含む)だけ。`Collect(id, そのプロジェクトのアイテム)` で移すものを決める。`lastRank(items []item.Item, parent *item.ID) string` はここに置く(`app/items` の同名の関数は import できないので写す)。
- `targets.go`: `Target`・`Targets`。`StoreList.List` を順に、閲覧のみを飛ばし、`Stores.Read` で `Workspaces().List()` → `port.LiveProjects` → 1 つ以上あるものだけ。読み取りを入れ子にしない(Store ごとに 1 回ずつ順に読む)。
- `preview.go`: `Preview`・`StatusRow`・`(Service) Preview`。ステータスの判定は上の「決めごと」。
- `move.go`: `MoveInput`・`Moved`・`(Service) Move`。`in.ToStore != store` は `port.ErrStoreNotFound`。`plan(r port.Reader, ...)` が検査と材料の読み取り(上の「決めごと」の順で `ErrNotFound` を返す)、`Read` で `plan` を呼んでアクティビティを決め、`Identity.Me(ctx, store)` で実行者を作り、`Write` の中でもう一度 `plan` を呼んで行を書く。書く順は、アイテムごとの `Append(id, place[, rank], remap.Drafts...)`、ワークスペースの `number` の行、`WriteTransfers().Append`。移動先のワークスペースの次の番号は `Write` の中で `tx` から読む。関数は 60 行以内に収めるため、書く部分は `writeWithin(tx, p plan) error` に分ける。

- [ ] **Step 4: 通す**

`go test ./internal/app/transfers/` → PASS。`go vet ./internal/app/transfers/` と `golangci-lint run ./internal/app/transfers/...`(`make lint` でもよい)が通る。

- [ ] **Step 5: コミット**

```bash
git add internal/app/transfers
git commit -m "feat: アイテムの別のワークスペースへの移動(同じ Store)と、移す先の候補・予告を追加" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: `app/transfers` — 別の Store への移動

> **この Task への上書き**(「本文より優先する直し」): `fn` の中で毎回 `Tx` から読む。戻しで足したメンバーも外す。`PeopleMap` は `transfer.People` の戻り値を使う。


決定 A5a-4・A5a-5(呼ぶ側)・A5a-6・A5a-10(呼ぶ側)・A5a-11・A5a-12(サーバーの文)を実装する。Task 4(`Deliver`)と Task 5 に依存する。Review Focus 1・2・4・5 のテストを入れる。

**Files:**
- Create: `internal/app/transfers/move_store.go`・`names.go`
- Modify: `internal/app/transfers/move.go`(別の Store への分岐)
- Create(テスト): `internal/app/transfers/move_store_test.go`・`move_store_race_test.go`

**Interfaces:**
- Consumes:
  - Task 5 のすべて、`transfer.Set`・`transfer.Cut`・`transfer.NewMovedIn`・`transfer.MovedInInfo`・`transfer.MovedInOf`・`transfer.PeopleMap`
  - `transfer.People(items []item.Item, logs map[item.ID][]change.Record, from workspace.Settings, source, dest []member.Member) []member.Member`(移動先にいない人を、移動元の表示名・ログイン名で返す。指摘 2)
  - `port.ItemWriter.Import(id item.ID, log []change.Record, description string) error`・`Remove(id item.ID) error`(Task 3)
  - `Delivery.Deliver`(Task 5。`syncing.Service` がそのまま満たす。Task 4)
- Produces:
  - `*transfers.SendError{Store string; Reason syncstatus.Reason}`。`Error()` は「{Store} に送信できないため移動できません: {Reason の短い理由}」(A5a-12)。例: 「team に送信できないため移動できません: remote への書き込みが許可されていません」
  - `(Service) Move` が `in.ToStore != store` のときに別の Store へ移す(戻り値は Task 5 と同じ形。`Moved.Item` は内部 ID のまま)
  - 移動先に書く行(1 回の `Deliver`): 移すアイテムごとに、`Import`(`log.jsonl` の全行をそのまま・`description.md`)→ `moved_in`(`transfer.NewMovedIn`)→ `set place`(移すアイテムは `set rank` も)→ `Remap` の行 → 中のアイテムが持つ、外の相手との関連の `remove`。ワークスペースの `number` の行、足す人の `member.Join`。アクティビティは移動元・移動先とも「{旧 ID} を {Store 名} の {移動元のワークスペース名} から {Store 名} の {移動先のワークスペース名} へ移動」(例: 「HH-2 を main の Web から team の Team へ移動」。指摘 9)
  - 移動元に書く行(`Stores.Write`): 移すアイテムごとに `Remove`、外のアイテムが持つ関連の `remove`、`transfers.jsonl` にアイテムごとに 1 行(`Entity: "item"`・`Target`・`Old`・`New`・`ToStore`・`ToStoreName`(このマシンでの移動先の Store 名)・`ToRemote`(移動先の remote。なければ空))
  - 戻し(A5a-6): 移動元の保存が失敗したら、移動先に「{新 ID} の移動を取り消し」の保存(写したフォルダの `Remove`)を `Deliver` で書く

**決めごと(骨組みの決定を実装に落としたもの):**
- 順(A5a-6): (0) 移動元を `Read` して移すもの・ログ・説明・メンバーを集める(ロックは読むあいだだけ)、(1)〜(3) 移動先を `Delivery.Deliver`(中で移動先を検査し、番号を取る)、(4) 移動元を `Stores.Write`。どの段階でも、片方の Store のロックを持ったまま、もう片方に触らない(向きの逆の 2 つの移動が止まらない)。`Deliver` の実行者は `Identity.Me(ctx, 移動先)`、移動元の保存は `Identity.Me(ctx, 移動元)`。
- `Deliver` が理由(`Reason`)を返したら `&SendError{Store: 移動先の Store 名, Reason: r}`。`fn` が返したエラー(`ErrNotFound`・断り)は、そのまま返る。`Deliver` のエラーは理由で表せない失敗なので、そのまま返す。
- 外との関連(Review Focus 4): `transfer.Collect` の `Outside` の各 `Cut` で、持つ側(`From`)が移すものの中なら移動先で、そのアイテムの `relations` を `remove` する。持つ側が外なら移動元で、その外のアイテム(削除済みを含む)の `relations` に `remove` を書く。中のアイテム同士の関連は残す。
- 人(Review Focus 5): `transfer.People(...)` が返した人(1 人 1 回)を、移動先の同じ保存で `member.Join` する。`PeopleMap` は「移動先のメンバー(`Deliver` の中で `tx.Members().List()`)と足す人の、ログイン名の小文字 → 移動先での綴り」で作る(移動元のログイン名がここに引けなければ、`Remap` が値を捨てる)。
- `moved_in` の `Names`(`names.go` の `namesOf(log []change.Record, from workspace.Settings) map[change.ID]string`): 写す行の `f` が指すプロパティの内部 ID(組み込みは `f` がキー、追加は `fields/<ID>`、`systems`・`status`・`priority` を含む)と名前、行の `from`・`to` の JSON の文字列(配列なら要素。`create` の行は見ない)のうち、`from` の値の内部 ID(ステータス・優先度・関連システム・各選択肢)と一致するものと、その名前。
- 移動元の保存の中で、移すアイテムの `Remove` が `ErrNotFound`(別の経路で先に消えた・移った)なら、その保存は失敗として扱い、戻す。
- 戻しも失敗したら `errors.Join(元のエラー, 戻しのエラー)` を返す。戻しは、写したフォルダだけを消す。足したメンバーと `number` の行は戻さない(指摘 10)。

#### 6-1 移動先の保存・移動元の保存・外との関連・人

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/transfers/move_store_test.go`。移すもの: Task HH-2(担当者 hanako、ウォッチ taro、hanako のコメント、状態 Todo)、その下の生きている Step(HH-4)と削除済みの Step(HH-3)。外: Epic HH-1 が Task を `relates` で持つ、Task が Epic を `blocks` で持つ、削除済みの外の Task HH-5 が Task を `relates` で持つ。中: Task が Step を `relates` で持つ。

```go
package transfers_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

const outsideGoneID item.ID = "01TESTIT000000000000000012"

func toTeam() transfers.MoveInput {
	return transfers.MoveInput{ToStore: store2, ToWorkspace: teamWS, ToProject: teamProject, ToParent: ptr(teamEpic)}
}

// crossFixture は、Task HH-2 を、上の構成にしたフィクスチャ。
func crossFixture(t *testing.T) fixture {
	t.Helper()
	f := newFixture(t)
	withSteps(t, f) // 削除済みの Step HH-3、生きている Step HH-4
	outside := addItem(t, f.rec, store1, item.Item{ID: outsideGoneID, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Number: 5}, Kind: item.Task, Rank: "X", Title: "外"})
	addRelation(t, f.rec, store1, apptest.SampleEpic, apptest.SampleTask, "relates")
	addRelation(t, f.rec, store1, apptest.SampleTask, apptest.SampleEpic, "blocks")
	addRelation(t, f.rec, store1, apptest.SampleTask, stepLiveID, "relates")
	addRelation(t, f.rec, store1, outside.ID, apptest.SampleTask, "relates")
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, outside.ID); err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.User("hanako"), Summary: "テスト"}
	err := f.rec.Write(t.Context(), store1, op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("assignee", nil, "hanako"),
				change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusInProgress),
				change.NewComment("01TESTCM000000000000000001", nil, comment.Value("確認しました"))))
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// 移動先にフォルダが写り、移動元からは消え、転送記録が 1 件ずつ残る。移動先の番号は TM-2 から(Epic が TM-1)。
func TestMoveAcrossStoresCopiesTheFolderAndRemovesTheSource(t *testing.T) {
	f := crossFixture(t)
	srcLog := logOf(t, f.rec, store1, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
	got, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam())
	if err != nil {
		t.Fatal(err)
	}
	if got.ToStore != store2 || got.Item != apptest.SampleTask || got.Display != "TM-2" {
		t.Errorf("moved = %+v", got)
	}
	for id, want := range map[item.ID]int{apptest.SampleTask: 2, stepLiveID: 3, stepGoneID: 4} {
		it := itemOf(t, f.rec, store2, id)
		if it.Place.Workspace != teamWS || it.Place.Project != teamProject || it.Place.Number != want {
			t.Errorf("%s = %+v", id, it.Place)
		}
	}
	if gone := itemOf(t, f.rec, store2, stepGoneID); gone.Deleted == nil {
		t.Error("削除済みのまま移る")
	}
	// 写した行はそのまま先頭に残り、そのあとに moved_in と place が続く。
	dst := logOf(t, f.rec, store2, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
	if len(dst) <= len(srcLog) || dst[0].ID != srcLog[0].ID || dst[len(srcLog)-1].ID != srcLog[len(srcLog)-1].ID {
		t.Fatalf("log が写っていない: %d 行 → %d 行", len(srcLog), len(dst))
	}
	mi := transfer.MovedInOf(dst)
	if len(mi) != 1 || mi[0].Old != "HH-2" || mi[0].FromStore != string(store1) || mi[0].FromStoreName != "main" ||
		mi[0].FromWorkspaceName != "Web" || mi[0].FromProjectTitle != "ログイン" || mi[0].FromWorkspace != apptest.SampleWorkspace {
		t.Fatalf("moved_in = %+v", mi)
	}
	if mi[0].Names[apptest.SampleStatusTodo] != "Todo" || mi[0].Names[apptest.SampleStatusInProgress] != "InProgress" || mi[0].Names["status"] != "ステータス" {
		t.Errorf("names = %v(値の名前とプロパティの名前を持つ)", mi[0].Names)
	}
	// 移動元からフォルダが消え、転送記録が 3 件。
	if err := f.rec.Read(t.Context(), store1, func(r port.Reader) error {
		_, err := r.Items().Get(apptest.SampleTask)
		return err
	}); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("移動元に残っている: %v", err)
	}
	recs := recordsOf(t, f.rec, store1)
	if len(recs) != 3 {
		t.Fatalf("転送記録 = %+v", recs)
	}
	for _, r := range recs {
		if r.Entity != "item" || r.ToStore != string(store2) || r.ToStoreName != "team" || r.ToRemote != "" {
			t.Errorf("記録 = %+v", r)
		}
	}
	wantSummary := "HH-2 を main の Web から team の Team へ移動"
	if s := f.rec.summaries(store2); s[len(s)-1] != wantSummary {
		t.Errorf("移動先のアクティビティ = %q", s)
	}
	if s := f.rec.summaries(store1); s[len(s)-1] != wantSummary {
		t.Errorf("移動元のアクティビティ = %q", s)
	}
}

// Review Focus 4: 移すものの外(削除済みを含む)との関連は両側から消え、中の関連は残る。
func TestMoveAcrossStoresCutsRelationsToOutsideItemsOnBothSides(t *testing.T) {
	f := crossFixture(t)
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); err != nil {
		t.Fatal(err)
	}
	moved := itemOf(t, f.rec, store2, apptest.SampleTask)
	if len(moved.Relations) != 1 || moved.Relations[0] != (item.Relation{Type: "relates", To: stepLiveID}) {
		t.Errorf("移したものの関連 = %+v(中の Step との関連だけ残る)", moved.Relations)
	}
	for _, id := range []item.ID{apptest.SampleEpic, outsideGoneID} {
		if rel := itemOf(t, f.rec, store1, id).Relations; len(rel) != 0 {
			t.Errorf("外の %s の関連 = %+v(相手の側からも消える)", id, rel)
		}
	}
}

// Review Focus 5: 移動先のメンバーにいない人(hanako は担当者・コメント・保存の実行者に現れる)は 1 回だけ足され、
// 大文字小文字だけが違う人(taro と Taro)は足さず、移動先の綴りに置き換わる。足す行は移動の操作と同じ操作。
func TestMoveAcrossStoresAddsMissingPeopleOnceAndMapsCaseInsensitively(t *testing.T) {
	f := crossFixture(t)
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); err != nil {
		t.Fatal(err)
	}
	var logins []string
	_ = f.rec.Read(t.Context(), store2, func(r port.Reader) error {
		ms, err := r.Members().List()
		for _, m := range ms {
			logins = append(logins, m.Login)
		}
		return err
	})
	slices.Sort(logins)
	if !slices.Equal(logins, []string{"Taro", "hanako"}) {
		t.Errorf("移動先のメンバー = %v", logins)
	}
	moved := itemOf(t, f.rec, store2, apptest.SampleTask)
	if moved.Assignee != "hanako" || !slices.Equal(moved.Watchers, []string{"Taro"}) {
		t.Errorf("担当者 = %q, ウォッチ = %v", moved.Assignee, moved.Watchers)
	}
	members := logOf(t, f.rec, store2, change.Ref{Kind: change.KindMembers})
	itemLog := logOf(t, f.rec, store2, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
	var joinOp, movedOp change.ID
	for _, r := range members {
		if r.T == change.Add && strings.Contains(string(r.To), "hanako") {
			joinOp = r.Op
		}
	}
	for _, r := range itemLog {
		if r.T == change.MovedIn {
			movedOp = r.Op
		}
	}
	if joinOp == "" || joinOp != movedOp {
		t.Errorf("メンバーの追加の操作 %q と moved_in の操作 %q は同じ", joinOp, movedOp)
	}
}

// Review Focus 1: 送れなかった(移動先に remote があり、書き込みが断られた)とき、移動先に何も残らず、移動元もそのまま。
func TestMoveAcrossStoresThatCannotBeSentLeavesBothStoresUntouched(t *testing.T) {
	f := crossFixture(t)
	f.deliver.reasons = map[port.StoreID]syncstatus.Reason{store2: syncstatus.WriteDenied}
	srcBefore := logOf(t, f.rec, store1, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
	nextBefore := nextNumber(t, f.rec, store2, teamWS)
	_, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam())
	var se *transfers.SendError
	if !errors.As(err, &se) || se.Store != "team" || se.Reason != syncstatus.WriteDenied {
		t.Fatalf("err = %v", err)
	}
	if want := "team に送信できないため移動できません: remote への書き込みが許可されていません"; err.Error() != want {
		t.Errorf("文 = %q, want %q", err.Error(), want)
	}
	if after := logOf(t, f.rec, store1, change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}); len(after) != len(srcBefore) {
		t.Errorf("移動元の行が変わった: %d → %d", len(srcBefore), len(after))
	}
	if len(recordsOf(t, f.rec, store1)) != 0 {
		t.Error("転送記録を書いた")
	}
	if err := f.rec.Read(t.Context(), store2, func(r port.Reader) error {
		_, err := r.Items().Get(apptest.SampleTask)
		return err
	}); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("移動先にアイテムが残っている: %v", err)
	}
	if n := nextNumber(t, f.rec, store2, teamWS); n != nextBefore {
		t.Errorf("移動先の次の番号 = %d, want %d", n, nextBefore)
	}
}

type failWrite struct {
	port.Stores
	store port.StoreID
}

var errBoom = errors.New("boom")

func (w failWrite) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	if id == w.store {
		return errBoom
	}
	return w.Stores.Write(ctx, id, op, fn)
}

// 移動元の保存が失敗したら、移動先に写したフォルダを消して戻し、エラーを返す。
func TestMoveAcrossStoresRollsBackTheDestinationWhenTheSourceSaveFails(t *testing.T) {
	f := crossFixture(t)
	f.svc.Stores = failWrite{Stores: f.rec, store: store1}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	for _, id := range []item.ID{apptest.SampleTask, stepLiveID, stepGoneID} {
		if err := f.rec.Read(t.Context(), store2, func(r port.Reader) error { _, err := r.Items().Get(id); return err }); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("移動先に %s が残っている: %v", id, err)
		}
		if it := itemOf(t, f.rec, store1, id); it.Place.Workspace != apptest.SampleWorkspace {
			t.Errorf("移動元の %s が変わった: %+v", id, it.Place)
		}
	}
	if s := f.rec.summaries(store2); s[len(s)-1] != "TM-2 の移動を取り消し" {
		t.Errorf("移動先のアクティビティ = %q", s)
	}
}
```

`internal/app/transfers/move_store_race_test.go`(Review Focus 2)。Store S1 の Task を S2 へ、S2 の Epic(TM-1)を S1 へ、同時に移す。`Deliver` の入口で 2 つの移動を揃えるので、片方の Store のロックを持ったまま `Deliver` を呼ぶ実装は、互いを待って止まる。

```go
package transfers_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/transfers"
)

func TestOppositeMovesBetweenTwoStoresDoNotStall(t *testing.T) {
	f := newFixture(t)
	f.deliver.gate = newBarrier(2) // 2 つの移動が両方 Deliver に入るまで待つ
	toWeb := transfers.MoveInput{ToStore: store1, ToWorkspace: apptest.SampleWorkspace, ToProject: apptest.SampleProject}
	done := make(chan error, 2)
	go func() { _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toTeam()); done <- err }()
	go func() { _, err := f.svc.Move(t.Context(), store2, teamEpic, toWeb); done <- err }()
	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("移動が失敗した: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("2 つの移動が止まった(ロックを入れ子に取っている)")
		}
	}
	if it := itemOf(t, f.rec, store2, apptest.SampleTask); it.Place.Workspace != teamWS {
		t.Errorf("Task = %+v", it.Place)
	}
	if it := itemOf(t, f.rec, store1, teamEpic); it.Place.Workspace != apptest.SampleWorkspace {
		t.Errorf("Epic = %+v", it.Place)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

`go test ./internal/app/transfers/ -run 'Across|Opposite'` → `undefined: transfers.SendError` などのコンパイルエラー。続いて `SendError` だけ作ると、`Move` が `port.ErrStoreNotFound` を返す失敗(`err = store not found`)になる。

- [ ] **Step 3: 実装する**

- `move_store.go`: `SendError`・`(Service) moveAcross`。`move.go` の `Move` の `in.ToStore != store` の行を `s.moveAcross(ctx, store, id, in)` に差し替える。段階ごとに関数を分ける(60 行以内): `gather`(0: 移動元の読み取り。`Collect` は移動元の Store 全体のアイテム(全ワークスペース・全プロジェクト・削除済みを含む。`Workspaces().List()` → `Projects().List` → `Items().List`)で呼ぶ)、`deliverTo`(1〜3 の `fn`: 移動先の検査、`numbering.Next`、`transfer.Number`、メンバー、`Import`、行)、`finishSource`(4)、`rollback`。移動先の検査は Task 5 の `ErrNotFound` の規則と同じ(`plan` の検査部分を、`port.Reader` を取る関数にして共有する)。
- `names.go`: `namesOf`(上の「決めごと」)。

- [ ] **Step 4: 通す**

`go test ./internal/app/transfers/` → PASS。`go test -race ./internal/app/transfers/ -run Opposite -count=20` も通る(止まらない)。

- [ ] **Step 5: コミット**

```bash
git add internal/app/transfers
git commit -m "feat: アイテムの別の Store への移動(受け取り・保存・関連の解除・メンバーの追加・戻し)を追加" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: `Locate`(旧 ID の解決)と、履歴の「{旧 ID} から移動」

> **この Task への上書き**(「本文より優先する直し」): 見つからないときは文を作らず `Gone` を返す。`MovedFrom.Display` はいまの ID。`whenText` は作らない。


決定 A5a-15・A5a-17 を実装する。7-1 で `app/transfers` に `Locate`、7-2 で `domain/history` に「{旧 ID} から移動」と `moved_in` の `names` での値の名前、7-3 で `app/history` に材料を渡す。Task 3(口)と Task 5 に依存する(Task 6 には依存しない。別の Store の記録は、テストで `WriteTransfers` から直接書く)。Review Focus 3 のテストを入れる。

**Files:**
- Create: `internal/app/transfers/locate.go`・`locate_test.go`
- Create: `internal/domain/history/moved.go`・`moved_test.go`
- Modify: `internal/domain/history/history.go`(`Context.Origin`)・`item.go`(行の文と値の名前の引き方)
- Modify: `internal/app/history/service.go`(`StoreList`・`Origin` を渡す)と、`setup` を定義しているテストファイル、`moved_test.go`(新規)

`internal/app/transfers` は 7 ファイル、`internal/domain/history` は 5 ファイル。

**Interfaces:**
- Consumes:
  - `port.Reader.Transfers().List()`・`port.Reader.Purged().List()`(Task 3)、`transfer.Record`・`transfer.MovedInOf`・`transfer.MovedInInfo`・`transfer.NewMovedIn`(Task 1)、`workspace.OldPrefixes`・`workspace.RankOf`、`apptest.StoreList`
  - `(*apptest.Stores).AddPurged(id port.StoreID, p port.Purged)`(Task 3 が偽物に足す。`purged.jsonl` の行を足すテスト用。指摘 4)
- Produces:
  - `transfers.Located{Found bool; Store port.StoreID; Project project.ID; Item item.ID; MovedFrom *MovedFrom; Message string}`、`transfers.MovedFrom{Old string; At time.Time; Display string}`、`(Service) Locate(ctx, storeName, display string) (Located, error)`。いまの ID・旧い接頭辞で見つかったら `Found`(`MovedFrom` なし)、転送記録で見つかったら `Found` と `MovedFrom`、見つからず理由があれば `Found: false` と `Message`(エラーなし)、理由もなければ `port.ErrNotFound`
  - `transfers.OldIDs`(E が検索で使う): `func (s Service) OldIDs(ctx context.Context, display string) ([]OldID, error)`、`type OldID struct{ Store port.StoreID; Item item.ID }`。`Locate` の (b)・(c) と同じ規則で、旧 ID に当たる削除されていないアイテムをすべて返す(`Locate` はその先頭を開く)
  - `history.Origin`(下)と `history.Context.Origin`。`apphistory.Service.StoreList port.StoreList`
  - 履歴の文: 「{旧 ID} から移動({移動元の Store 名} / {ワークスペース名} / {プロジェクト名})」。時刻は行の `at`

**決めごと(骨組みの決定を実装に落としたもの):**
- `display` は「接頭辞-番号」(最後の `-` で分け、番号は数字だけ)。読めなければ `port.ErrNotFound`。
- 順(A5a-15): (a) いまの ID: 表示中の Store を、名前が `storeName` の Store を先頭、残りは `StoreList` の順に見て、削除されていないアイテムのうち、ワークスペースのいまの接頭辞と番号が一致する先頭。(b) 旧い接頭辞: (a) で見つからなければ、`workspace.OldPrefixes` に接頭辞があるワークスペースの、同じ番号の削除されていないアイテム。(c) 転送記録: すべての表示中の Store の `transfers.jsonl` のうち `Entity == "item"` かつ `Old == display` の行を集め、`At` の新しい順に、`Target` の内部 ID を表示中のすべての Store から探して、削除されていないものがあれば `Found`(`MovedFrom` は最初に一致した行の `Old`・`At`・`New`)。
- (c) で削除されていないものがなければ、最初に一致した行を `rec` として、次の順で `Message` を決める。(1) `rec.Target` が表示中のどこかの Store で削除済み: 「{旧 ID} は {日時} に {rec.New} へ移動しましたが、削除されています。設定の「削除済み」から復元できます」。(2) 表示中のどこかの Store の `purged.jsonl` に `Target == rec.Target`: 「{旧 ID} は {日時} に {rec.New} へ移動しましたが、完全に削除されました」。(3) `rec.ToStore` が表示中の Store でない: 「{旧 ID} は {日時} に {Store 名} へ移動しました。この Store はいま表示できません」。(4) `rec.Target` を指す、より新しい記録 `later`(表示中のすべての Store の `transfers.jsonl` のうち、`Target` が同じで `At` が `rec` より後の最新のもの)があり、`later.ToStore` が `rec.ToStore` と違い、表示中の Store でない: 「{旧 ID} は {日時} に {rec.New} へ移動し、いまは {later.New} にあります」。(5) どれでもなければ `port.ErrNotFound`。(4) の当てはめは骨組みの文だけでは決まらないので、この計画で決めた(指摘 8)。
- {Store 名}: `rec.ToStore` の Store がこのマシンの `StoreList` にあればその名前、なければ `rec.ToStoreName`。
- {日時}: 01「日付と時刻」の日付と時刻。今年(`Clock.Now()` の年)なら「M/D HH:MM」(例: 「11/12 09:00」)、それ以外の年なら「YYYY/MM/DD HH:MM」(例: 「2026/11/12 09:00」)。時刻帯は `Clock.Now()` のもの。`locate.go` の `whenText(at, now time.Time) string`。
- 履歴の移動の行(7-2): 次のどちらかが、1 行の「{旧 ID} から移動(…)」になる。(i) `moved_in` の行(別の Store から。旧 ID・Store 名・ワークスペース名・プロジェクト名は行の値)。(ii) 同じ Store の中の移動: 場所の `set` の行で、`from` と `to` のワークスペースが違い、同じ操作の `moved_in` の行がないもの。旧 ID は `Origin.Transfers` の、その行と同じ `Op` の行の `Old`(なければ、その行を出さない)、Store 名は `Origin.StoreName`、ワークスペース名とプロジェクト名は `from` の内部 ID を `Origin.WorkspaceName`・`Origin.ProjectTitle` で引く。ワークスペースが変わる場所の `set` の行は、「親を変更」に出さない(同じ操作の `moved_in` の行が文を持つ)。
- 履歴の値の名前(7-2): 行を書いたときにアイテムがいたワークスペースの定義で引く。ワークスペースは、ログの作成と場所の `set` の `to` から追う。`from` の値は、その操作の最初の場所の `set` の `from` のワークスペース(場所の `set` がなければ、その行の直前のワークスペース)、`to` の値は、その操作の最後の場所の `set` の `to` のワークスペースの定義で引く。いまのワークスペースは `Context.Settings`、このほかのワークスペースで同じ Store にあるものは `Origin.Settings(id)`、どちらにもない(別の Store から来た)ものは、その行より後で最初の `moved_in` の `Names` で引く。`Names` にもない値は「(削除された値)」。`Names` で引くときの値の型は、いまのワークスペースの同じ内部 ID のプロパティから決め(組み込みは内部 ID がキー)、そのプロパティがなければ、値の文字列が `Names` にあればその名前、なければそのまま出す。

#### 7-1 `Locate`

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/transfers/locate_test.go`。既存の補助(Task 5)に加え、記録を直接書く補助を置く。記録の `At` は `Write` が付ける(偽の `Stores` の時計。2026-11-12 09:00 ごろ)。

```go
package transfers_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const roamer item.ID = "01TESTIT000000000000000020" // 移動を重ねたアイテム

// putRecord は、転送記録を直接足す(Entity は item)。
func putRecord(t *testing.T, f fixture, store port.StoreID, target item.ID, old, newID string, to port.StoreID, toName string) {
	t.Helper()
	write(t, f.rec, store, "taro", func(tx port.Tx) error {
		return tx.WriteTransfers().Append(transfer.Record{Entity: "item", Target: target, Old: old, New: newID, ToStore: string(to), ToStoreName: toName})
	})
}

func locate(t *testing.T, f fixture, store, display string) transfers.Located {
	t.Helper()
	got, err := f.svc.Locate(t.Context(), store, display)
	if err != nil {
		t.Fatalf("Locate(%q, %q) = %v", store, display, err)
	}
	return got
}

// いまの ID。同じ ID が 2 つの Store にあるときは、Store 名の Store を先に、なければ並びの先頭の Store。
func TestLocateCurrentIDPrefersTheNamedStore(t *testing.T) {
	f := newFixture(t)
	addWorkspace(t, f.rec, store2, workspace.Workspace{ID: "01TESTWS000000000000000020", Name: "Web2", Prefix: "HH"}, "01TESTPR000000000000000020", "01TESTIT000000000000000021", nil)
	other := addItem(t, f.rec, store2, item.Item{ID: "01TESTIT000000000000000022", Place: item.Place{Workspace: "01TESTWS000000000000000020", Project: "01TESTPR000000000000000020", Parent: ptr("01TESTIT000000000000000021"), Number: 2}, Kind: item.Task, Rank: "V", Title: "別の HH-2"})
	if got := locate(t, f, "team", "HH-2"); !got.Found || got.Store != store2 || got.Item != other.ID || got.MovedFrom != nil {
		t.Errorf("team: %+v", got)
	}
	for _, name := range []string{"main", "zzz"} { // 名前の Store になければ並びの先頭
		if got := locate(t, f, name, "HH-2"); got.Store != store1 || got.Item != apptest.SampleTask || got.Project != apptest.SampleProject {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

// 旧い接頭辞(HH → WEB)で見つかる。あとから別のワークスペース(Mobile)が HH を使うと、いまの接頭辞の方を開く。
func TestLocateOldPrefix(t *testing.T) {
	f := newFixture(t)
	write(t, f.rec, store1, "taro", func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet("prefix", "HH", "WEB"))
	})
	if got := locate(t, f, "main", "HH-2"); got.Item != apptest.SampleTask || got.MovedFrom != nil {
		t.Errorf("旧い接頭辞: %+v", got)
	}
	if got := locate(t, f, "main", "WEB-2"); got.Item != apptest.SampleTask {
		t.Errorf("いまの接頭辞: %+v", got)
	}
	write(t, f.rec, store1, "taro", func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(mobileWS, change.NewSet("prefix", "MB", "HH"))
	})
	mobileTask := addItem(t, f.rec, store1, item.Item{ID: "01TESTIT000000000000000023", Place: item.Place{Workspace: mobileWS, Project: mobileProject, Parent: ptr(mobileEpic), Number: 2}, Kind: item.Task, Rank: "V", Title: "Mobile の 2"})
	if got := locate(t, f, "main", "HH-2"); got.Item != mobileTask.ID {
		t.Errorf("いまの接頭辞のワークスペースが優先: %+v", got)
	}
}

// Review Focus 3: A → B → A(番号は再利用しないので、戻ったあとの ID は HH-3)。旧 ID はどちらも、いまの場所に辿り着く。
func TestLocateOfAnItemMovedAwayAndBack(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
		t.Fatal(err)
	}
	back := transfers.MoveInput{ToStore: store1, ToWorkspace: apptest.SampleWorkspace, ToProject: apptest.SampleProject, ToParent: ptr(apptest.SampleEpic)}
	if got, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, back); err != nil || got.Display != "HH-3" {
		t.Fatalf("戻す: %+v, %v", got, err)
	}
	recs := recordsOf(t, f.rec, store1) // HH-2 → MB-2、MB-2 → HH-3
	first := locate(t, f, "main", "HH-2")
	if !first.Found || first.Item != apptest.SampleTask || first.MovedFrom == nil ||
		first.MovedFrom.Old != "HH-2" || first.MovedFrom.Display != "MB-2" || !first.MovedFrom.At.Equal(recs[0].At) {
		t.Errorf("HH-2: %+v", first)
	}
	if got := locate(t, f, "main", "MB-2"); got.Item != apptest.SampleTask || got.MovedFrom.Display != "HH-3" {
		t.Errorf("MB-2: %+v", got)
	}
	if got := locate(t, f, "main", "HH-3"); got.Item != apptest.SampleTask || got.MovedFrom != nil {
		t.Errorf("HH-3(いまの ID): %+v", got)
	}
}

// Review Focus 3: A → B → C。A の記録(HH-2 → TM-2)はそのまま、いまの場所(閲覧のみの Store の Archive)へ辿り着く。
// 閲覧のみの Store の中でも、表示だけは解決できる。
func TestLocateFollowsAChainAcrossStores(t *testing.T) {
	f := newFixture(t)
	addItem(t, f.rec, store3, item.Item{ID: roamer, Place: item.Place{Workspace: archiveWS, Project: archiveProj, Parent: ptr(archiveEpic), Number: 7}, Kind: item.Task, Rank: "V", Title: "旅"})
	putRecord(t, f, store1, roamer, "HH-2", "TM-2", store2, "team")
	putRecord(t, f, store2, roamer, "TM-2", "AR-7", store3, "readonly")
	got := locate(t, f, "main", "HH-2")
	if !got.Found || got.Store != store3 || got.Item != roamer || got.Project != archiveProj || got.MovedFrom.Display != "TM-2" {
		t.Errorf("HH-2: %+v", got)
	}
	if got := locate(t, f, "main", "TM-2"); got.Store != store3 || got.MovedFrom.Display != "AR-7" {
		t.Errorf("TM-2: %+v", got)
	}
}

// Review Focus 3: 同じものを 2 人が別々に移して、記録が 2 件(TM-2 と AR-7)。内部 ID の現在地(team)に辿り着く。
// 添える日時と新 ID は、最初に一致した行(新しい方の記録)のもの。
func TestLocateWithTwoRecordsOfTheSameItem(t *testing.T) {
	f := newFixture(t)
	addItem(t, f.rec, store2, item.Item{ID: roamer, Place: item.Place{Workspace: teamWS, Project: teamProject, Parent: ptr(teamEpic), Number: 2}, Kind: item.Task, Rank: "V", Title: "旅"})
	putRecord(t, f, store1, roamer, "HH-2", "TM-2", store2, "team")
	putRecord(t, f, store1, roamer, "HH-2", "AR-7", store3, "readonly")
	if got := locate(t, f, "main", "HH-2"); got.Store != store2 || got.Item != roamer || got.MovedFrom.Display != "AR-7" {
		t.Errorf("HH-2: %+v", got)
	}
}

// 見つからないとき(画面は開かず、4 つの文を言い分ける)。{日時} は今年なら M/D HH:MM。
func TestLocateMessagesWhenTheItemIsNotFound(t *testing.T) {
	const gone item.ID = "01TESTIT000000000000000030"
	cases := []struct {
		name  string
		setup func(t *testing.T, f fixture)
		want  string
	}{
		{"削除済み", func(t *testing.T, f fixture) {
			if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, toMobile()); err != nil {
				t.Fatal(err)
			}
			if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleTask); err != nil {
				t.Fatal(err)
			}
		}, "HH-2 は 11/12 09:00 に MB-2 へ移動しましたが、削除されています。設定の「削除済み」から復元できます"},
		{"完全に削除", func(t *testing.T, f fixture) {
			putRecord(t, f, store1, gone, "HH-2", "TM-2", store2, "team")
			f.rec.AddPurged(store2, port.Purged{Entity: "item", Target: gone, Display: "TM-2", At: time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)})
		}, "HH-2 は 11/12 09:00 に TM-2 へ移動しましたが、完全に削除されました"},
		{"移動先の Store が表示中でない", func(t *testing.T, f fixture) {
			putRecord(t, f, store1, gone, "HH-2", "WEB-31", "S9", "elsewhere")
		}, "HH-2 は 11/12 09:00 に elsewhere へ移動しました。この Store はいま表示できません"},
		{"いまの場所が記録の移動先と違う", func(t *testing.T, f fixture) {
			putRecord(t, f, store1, gone, "HH-2", "WEB-31", store2, "team")
			putRecord(t, f, store2, gone, "WEB-31", "TM-5", "S9", "elsewhere")
		}, "HH-2 は 11/12 09:00 に WEB-31 へ移動し、いまは TM-5 にあります"},
	}
	for _, c := range cases {
		f := newFixture(t)
		if c.name != "削除済み" { // サンプルの Task が HH-2 を使っているので、削除して旧 ID を空ける
			if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleTask); err != nil {
				t.Fatal(err)
			}
		}
		c.setup(t, f)
		got, err := f.svc.Locate(t.Context(), "main", "HH-2")
		if err != nil || got.Found || got.Message != c.want {
			t.Errorf("%s: %+v, %v\nwant message %q", c.name, got, err, c.want)
		}
	}
}

// 今年でない日時は YYYY/MM/DD HH:MM。
func TestLocateMessageDateOfAnotherYear(t *testing.T) {
	f := newFixture(t)
	f.svc.Clock = apptest.NewClock(time.Date(2027, 1, 5, 9, 0, 0, 0, time.UTC))
	putRecord(t, f, store1, "01TESTIT000000000000000030", "ZZ-9", "WEB-31", "S9", "elsewhere")
	got, err := f.svc.Locate(t.Context(), "main", "ZZ-9")
	if err != nil || got.Message != "ZZ-9 は 2026/11/12 09:00 に elsewhere へ移動しました。この Store はいま表示できません" {
		t.Errorf("%+v, %v", got, err)
	}
}

// 理由がなければ見つからない(ホームへ)。削除しただけのいまの ID も同じ。
func TestLocateNotFound(t *testing.T) {
	f := newFixture(t)
	for _, id := range []string{"ZZ-9", "nonsense", "HH-", "-3"} {
		if _, err := f.svc.Locate(t.Context(), "main", id); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%q: %v", id, err)
		}
	}
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Locate(t.Context(), "main", "HH-2"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除しただけのいまの ID: %v", err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

`go test ./internal/app/transfers/ -run Locate` → `f.svc.Locate undefined` のコンパイルエラー。

- [ ] **Step 3: 実装する**

`locate.go`: `Located`・`MovedFrom`・`OldID`・`Locate`・`OldIDs`・`whenText`。Store ごとに `Stores.Read` を 1 回ずつ順に使い(入れ子にしない)、各 Store の `Workspaces`・`Projects`・`Items`・`Transfers`・`Purged` を集めた中間の値(`snapshot`)にしてから、(a)〜(c)・(1)〜(5) を純粋な関数で決める(60 行・複雑度 15 に収まるように、`currentID`・`oldPrefix`・`byRecords`・`whyGone` に分ける)。

- [ ] **Step 4: 通す**

`go test ./internal/app/transfers/` → PASS。

- [ ] **Step 5: コミット**

```bash
git add internal/app/transfers
git commit -m "feat: 旧 ID(移動前の ID・旧い接頭辞)から、いまの場所を解決する Locate を追加" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

#### 7-2 履歴(`domain/history`)

- [ ] **Step 1: 失敗するテストを書く**

`internal/domain/history/moved_test.go`(既存の `history_test.go` の `step`・`logOf`・`at`・`ctx`・`texts`・`taro`・`names` を使う)。

```go
package history_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// 移動元のワークスペース(WA)の設定。値の内部 ID は A01〜(ステータス A01 Todo・A02 InProgress)。
func settingsA() workspace.Settings {
	n := 0
	return workspace.DefaultSettings(func() change.ID { n++; return change.ID(fmt.Sprintf("A%02d", n)) })
}

func taskIn(ws workspace.ID, pr project.ID) item.Item {
	return item.Item{ID: "T", Place: item.Place{Workspace: ws, Project: pr, Number: 2}, Kind: item.Task, Rank: "V", Title: "ログイン", Status: "A01"}
}

func ptrID(id item.ID) *item.ID { return &id }

// 別の Store から移ってきた。moved_in の行が「{旧 ID} から移動(…)」になり、同じ操作の場所の変更は「親を変更」に出ない。
// 移る前の行の値は moved_in の names で、移動と同じ操作のステータスは from を names、to をいまの設定で引く。
func TestItemRowsMovedInFromAnotherStore(t *testing.T) {
	from, to := taskIn("WA", "PA"), taskIn("WB", "PB")
	info := transfer.MovedInInfo{
		Old: "HH-2", FromStore: "S1", FromStoreName: "main", FromWorkspace: "WA", FromWorkspaceName: "Web",
		FromProject: "PA", FromProjectTitle: "ログイン", At: at(3),
		Names: map[change.ID]string{"A01": "Todo", "A02": "InProgress", "status": "ステータス"},
	}
	log := logOf(
		step{"O1", at(0), taro, item.Create(from)},
		step{"O2", at(1), taro, change.NewSet("status", "A01", "A02")},
		step{"O3", at(2), taro, change.NewSet("status", "A02", "A99")}, // names にない値
		step{"O4", at(3), taro, transfer.NewMovedIn(info)},
		step{"O4", at(3), taro, change.NewSet("place", from.Place, to.Place)},
		step{"O4", at(3), taro, change.NewSet("status", "A99", "V01")},
	)
	rows := history.ItemRows(log, ctx())
	want := []string{
		"太郎: ステータスを (削除された値) → Todo",
		"太郎: HH-2 から移動(main / Web / ログイン)",
		"太郎: ステータスを InProgress → (削除された値)",
		"太郎: ステータスを Todo → InProgress",
		"太郎: 作成",
	}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
	if !rows[1].At.Equal(at(3)) {
		t.Errorf("移した日時 = %v", rows[1].At)
	}
}

// 同じ Store の別のワークスペースへ。場所の変更の行と、転送記録(同じ op)から作り、移動元のワークスペースの値の名前で引く。
func TestItemRowsMovedWithinTheStore(t *testing.T) {
	from, to := taskIn("WA", "PA"), taskIn("WB", "PB")
	reparented := to.Place
	reparented.Parent = ptrID("E9")
	log := logOf(
		step{"O1", at(0), taro, item.Create(from)},
		step{"O2", at(1), taro, change.NewSet("status", "A01", "A02")},
		step{"O3", at(3), taro, change.NewSet("place", from.Place, to.Place)},
		step{"O3", at(3), taro, change.NewSet("status", "A02", "V01")},
		step{"O4", at(4), taro, change.NewSet("place", to.Place, reparented)},
	)
	c := ctx()
	c.Origin = history.Origin{
		Settings:      func(id workspace.ID) (workspace.Settings, bool) { return settingsA(), id == "WA" },
		WorkspaceName: func(id workspace.ID) (string, bool) { return "Web", id == "WA" },
		ProjectTitle:  func(id project.ID) (string, bool) { return "ログイン", id == "PA" },
		StoreName:     "main",
		Transfers:     []transfer.Record{{Op: "O3", Target: "T", Old: "HH-2", New: "MB-2"}},
	}
	got := texts(history.ItemRows(log, c))
	want := []string{
		"太郎: 親を変更", // 同じワークスペースの中の親の変更は、いままでどおり
		"太郎: ステータスを InProgress → Todo",
		"太郎: HH-2 から移動(main / Web / ログイン)",
		"太郎: ステータスを Todo → InProgress",
		"太郎: 作成",
	}
	if !slices.Equal(got, want) {
		t.Errorf("rows =\n%q\nwant\n%q", got, want)
	}
}

// 転送記録がない(手で書き換えた)ワークスペースの変更は、文を作らない。
func TestItemRowsWorkspaceChangeWithoutARecordIsNotShown(t *testing.T) {
	from, to := taskIn("WA", "PA"), taskIn("WB", "PB")
	log := logOf(step{"O1", at(0), taro, item.Create(from)}, step{"O2", at(1), taro, change.NewSet("place", from.Place, to.Place)})
	if got := texts(history.ItemRows(log, ctx())); !slices.Equal(got, []string{"太郎: 作成"}) {
		t.Errorf("rows = %q", got)
	}
}
```

(`ctx()` のいまの設定は `settings()`(V01 Todo)で、アイテムのいまのワークスペースは WB とみなす。いまのワークスペースの ID を `Context` に渡すかは実装で決めてよいが、渡すなら `ctx()` は変えず、テストの側から `Context` に足す。)

- [ ] **Step 2: 失敗を確かめる**

`go test ./internal/domain/history/` → `undefined: history.Origin`(コンパイルエラー)。

- [ ] **Step 3: 実装する**

`moved.go`(新規): `type Origin struct { Settings func(workspace.ID) (workspace.Settings, bool); WorkspaceName func(workspace.ID) (string, bool); ProjectTitle func(project.ID) (string, bool); StoreName string; Transfers []transfer.Record }`(`Transfers` は、このアイテムを `Target` に持つ記録)。`Context` に `Origin Origin` を足す(`history.go`)。`item.go` の `itemText` に、上の「決めごと」の (i)・(ii) と「親を変更」の除外を足す。値の名前は、行ごとに「`from` のワークスペース」「`to` のワークスペース」を決める補助(ログを 1 度なめて、操作ごとの前後のワークスペースを持つ)と、設定を引く関数にまとめる。`Names` で引くときは、`names` の全部の値を、ステータス・優先度・関連システム・各選択肢のどの一覧にも同じ内部 ID で入れた設定を組んで `item.Display` に渡す(一覧のどれで引いても名前が返るので、値の型を知らなくてよい)。`moved_in` の行は `transfer.MovedInOf([]change.Record{r})` で 1 行ずつ読む。

- [ ] **Step 4: 通す**

`go test ./internal/domain/history/` → PASS(既存の文のテストが壊れない)。

- [ ] **Step 5: コミット**

```bash
git add internal/domain/history
git commit -m "feat: 履歴に「{旧 ID} から移動」を足し、移る前の値を移動元の定義と moved_in の names で引く" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

#### 7-3 履歴の材料(`app/history`)

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/history/moved_test.go`。サンプルに「Mobile」を足し(`transfers` のテストの `addWorkspace` と同じ作り方: ワークスペースの作成・プロジェクト)、Task に場所の変更と転送記録を、`hanako` の 1 回の保存で直接書く。

```go
func TestItemHistoryOfAnItemMovedWithinTheStore(t *testing.T) {
	svc, s := setup(t) // setup は StoreList に {S1, "main"} を渡す
	addMobile(t, s)    // このファイルの補助。ワークスペース「Mobile」(mobileWorkspace)とプロジェクト(mobileProject)を足す
	write(t, s, "hanako", func(tx port.Tx) error {
		task, err := tx.Items().Get(apptest.SampleTask)
		if err != nil {
			return err
		}
		to := task.Place
		to.Workspace, to.Project, to.Parent, to.Number = mobileWorkspace, mobileProject, nil, 2
		return errors.Join(
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("place", task.Place, to)),
			tx.WriteTransfers().Append(transfer.Record{Entity: "item", Target: apptest.SampleTask, Old: "HH-2", New: "MB-2", ToStore: string(store)}),
		)
	})
	rows, err := svc.Item(t.Context(), store, apptest.SampleTask)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"hanako: HH-2 から移動(main / Web / ログイン)", "taro: 作成"}
	if got := texts(rows); !slices.Equal(got, want) {
		t.Errorf("rows = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

`go test ./internal/app/history/` → `Service` に `StoreList` がない(`setup` の変更で)コンパイルエラー。

- [ ] **Step 3: 実装する**

`service.go`: `Service` に `StoreList port.StoreList` を足す。`Item` の `Read` の中で、すべてのワークスペースの設定・名前、すべてのプロジェクトの名前、`Transfers().List()` のうち `Target` がこのアイテムの行、`StoreList.List` の `store` の名前を集めて、`history.Origin` を組み、`Context` に渡す(クロージャは、集めた map を引く)。`setup` に `StoreList: apptest.StoreList{Stores: []port.StoreInfo{{ID: store, Name: "main"}}}` を足す。

- [ ] **Step 4: 通す**

`go test ./internal/app/history/ ./internal/domain/history/ ./internal/app/transfers/` → PASS。

- [ ] **Step 5: コミット**

```bash
git add internal/app/history
git commit -m "feat: アイテムの履歴に、同じ Store の移動の元のワークスペース・プロジェクトの名前を渡す" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: API と配線(`httpapi/transfers`、`cmd/hedgehug`、e2e のビルドの Store とサンプル)

> **この Task への上書き**(「本文より優先する直し」): 予告・移動・`locate-item` の応答のキーは「名前の直し」のとおり(`gone` は `reason` ほか)。


決定 A5a-19・A5a-20 を実装する。8-1 でサンプル(Mobile・Team)、8-2 で API、8-3 で `routes.go`・`wire.go`・`wire_e2e.go` をつなぐ。Task 5〜7 に依存する。

**Files:**
- Modify: `internal/app/apptest/sample.go`、同じパッケージの既存のテストファイル(`stores_test.go`)
- Create: `internal/adapter/httpapi/transfers/transfers.go`(`Register`・候補・予告)・`move.go`・`locate.go`、テスト `transfers_test.go`・`move_test.go`・`locate_test.go`
- Modify: `internal/adapter/httpapi/routes.go`(`Deps` と登録)
- Modify: `cmd/hedgehug/wire.go`・`wire_e2e.go`・`wire_e2e_test.go`
- Generated: `api/openapi.yaml`・`web/packages/api/src/generated.ts`(`make generate` → `pnpm -C web gen`)

`internal/app/apptest` は 20 ファイルの上限なので、新しいファイルは足さない。`internal/adapter/httpapi/transfers` は 3 ファイル。

**Interfaces:**
- Consumes: Task 5〜7 の `transfers.Service`・`Targets`・`Preview`・`Move`・`Locate`・`SendError`・`Delivery`、`apierr.From`、`syncstatus`
- Produces:
  - `apptest.SampleMobile(ctx, s port.Stores, id port.StoreID) error`: サンプルの Store(`Sample` のあと)に、ワークスペース「Mobile」(接頭辞 `MB`)、プロジェクト「アプリ」(MB-P1)、Epic「iOS」(MB-1)を 1 回の保存で足す。`apptest.SampleTeam(ctx, s, id) error`: 空の Store に、メンバー taro・hanako、ワークスペース「Team」(接頭辞 `TM`)、プロジェクト「運営」(TM-P1)、Epic「チーム運営」(TM-1)を書く。内部 ID は `Sample` と重ならない(別の Store へ移すとき、同じ内部 ID があると写せないため)。値の内部 ID も別(`01SAMPLEMBV`・`01SAMPLETMV` に 15 桁の連番)。定数 `SampleMobileWorkspace`・`SampleMobileProject`・`SampleMobileEpic`(`01SAMPLEWS0000000000000002`・`01SAMPLEPR0000000000000002`・`01SAMPLEIT0000000000000003`)、`SampleTeamWorkspace`・`SampleTeamProject`・`SampleTeamEpic`(`01SAMPLEWS0000000000000003`・`01SAMPLEPR0000000000000003`・`01SAMPLEIT0000000000000004`)
  - `transfers.Register(api huma.API, svc apptransfers.Service)`(`internal/adapter/httpapi/transfers`)
  - HTTP(骨組みの表のとおり):
    - `list-transfer-targets` → `[{storeId, storeName, workspaceId, workspaceName, prefix, projects: [{id, title, epics: [{id, display, title}]}]}]`(空は `[]`)
    - `get-transfer-preview`(query `toStore`・`toWorkspace`・`toProject`)→ `{below, deleted, nextId, acrossStores, remoteNote, statuses: [{status, name, count, choices: [{id, name}], default}]}`(空は `[]`)
    - `transfer-item`(本文 `{toStore, toWorkspace, toProject, toParent?, statusChoices}`)→ 200 `{toStore, itemId, display, emptied}`(`emptied` の空は `[]`)。断りは 422、`port.ErrNotFound`・`port.ErrStoreNotFound` は 404、`*transfers.SendError` は 422 で `detail` に `Error()` の文
    - `locate-item`(GET `/api/locate`、query `store`・`id`)→ `{kind: "found", storeId, projectId, itemId, movedFrom?: {old, at, display}}` か `{kind: "gone", message}`。見つからなければ 404。`at` は `change.TimeLayout`
  - `httpapi.Deps` に `Delivery apptransfers.Delivery`・`SyncTargets port.SyncTargets`(`cmd/hedgehug` が `syncing.Service` を `Delivery` に渡す。app の機能どうしを import しないため)。履歴の API に `StoreList` を渡す

#### 8-1 サンプル(Mobile・Team)

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/apptest/stores_test.go` の末尾に足す(パッケージ名はそのファイルに合わせる):

```go
func TestSampleMobileAndTeamDoNotShareInternalIDsWithTheSample(t *testing.T) {
	ctx := t.Context()
	s := apptest.NewStores(apptest.NewClock(time.Now()), apptest.NewIDs())
	s.AddStore("A")
	s.AddStore("B")
	if err := errors.Join(apptest.Sample(ctx, s, "A"), apptest.SampleMobile(ctx, s, "A"), apptest.SampleTeam(ctx, s, "B")); err != nil {
		t.Fatal(err)
	}
	if err := s.Read(ctx, "A", func(r port.Reader) error {
		ws, err := r.Workspaces().List()
		if err != nil || len(ws) != 2 {
			t.Errorf("A のワークスペース = %+v, %v", ws, err)
		}
		mobile, err := r.Workspaces().Get(apptest.SampleMobileWorkspace)
		if err != nil || mobile.Name != "Mobile" || mobile.Prefix != "MB" {
			t.Errorf("Mobile = %+v, %v", mobile, err)
		}
		epic, err := r.Items().Get(apptest.SampleMobileEpic)
		if err != nil || epic.Place.Number != 1 || epic.Place.Project != apptest.SampleMobileProject {
			t.Errorf("MB-1 = %+v, %v", epic, err)
		}
		st, err := r.Workspaces().Settings(apptest.SampleMobileWorkspace)
		if err != nil || st.Statuses[0].ID == apptest.SampleStatusTodo {
			t.Errorf("値の内部 ID が Web と同じ: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Read(ctx, "B", func(r port.Reader) error {
		ms, _ := r.Members().List()
		team, err := r.Workspaces().Get(apptest.SampleTeamWorkspace)
		epic, _ := r.Items().Get(apptest.SampleTeamEpic)
		if len(ms) != 2 || err != nil || team.Prefix != "TM" || epic.Place.Number != 1 || epic.Title != "チーム運営" {
			t.Errorf("Team: %+v %+v %+v, %v", ms, team, epic, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

`go test ./internal/app/apptest/` → `undefined: apptest.SampleMobile`。

- [ ] **Step 3: 実装する**

`sample.go`: 定数と `SampleMobile`・`SampleTeam` を足す。`sampleValueIDs()` は接頭辞を引数に取る `valueIDs(prefix string)` に直し、`Sample` は `valueIDs("01SAMPLEVAL")`、Mobile は `"01SAMPLEMBV"`、Team は `"01SAMPLETMV"` で呼ぶ(15 桁の連番はそのまま)。`Sample` の値の ID は変えない(既存のテストが参照する)。番号の行(`numbering.Draft`)は、プロジェクト 1 とアイテム 1 を書く。

- [ ] **Step 4: 通す**

`go test ./internal/app/apptest/` → PASS。

#### 8-2 API

- [ ] **Step 1: 失敗するテストを書く**

`internal/adapter/httpapi/transfers/transfers_test.go`(`package transfers_test`)。S1「main」= `Sample` + `SampleMobile`、S2「team」= `SampleTeam`。`Delivery` は、保存して、理由があれば保存を捨てる偽物(Task 5 の `stubDelivery` の `Deliver` と同じ内容を、`deliver` という名前でここに写す)。

```go
type env struct {
	api     humatest.TestAPI
	stores  *apptest.Stores
	reasons map[port.StoreID]syncstatus.Reason
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ids := apptest.NewIDs()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), ids)
	s.AddStore("S1")
	s.AddStore("S2")
	if err := errors.Join(apptest.Sample(t.Context(), s, "S1"), apptest.SampleMobile(t.Context(), s, "S1"), apptest.SampleTeam(t.Context(), s, "S2")); err != nil {
		t.Fatal(err)
	}
	e := &env{stores: s, reasons: map[port.StoreID]syncstatus.Reason{}}
	_, e.api = humatest.New(t)
	transfers.Register(e.api, apptransfers.Service{
		Stores: s, Delivery: &deliver{stores: s, reasons: e.reasons}, Identity: apptest.Identity{Login: "taro"},
		Clock:     apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)),
		StoreList: apptest.StoreList{Stores: []port.StoreInfo{{ID: "S1", Name: "main"}, {ID: "S2", Name: "team"}}},
		Targets: &apptest.SyncTargets{Targets: []port.SyncTarget{
			{ID: "S1", Name: "main", Remote: "https://example.invalid/main.git"}, {ID: "S2", Name: "team"},
		}},
	})
	return e
}
```

テスト(`humatest` の `api.Get`・`api.Post` と `json.Unmarshal`。`items` の API のテストの書き方に合わせる):

```go
func TestTargets(t *testing.T) {
	e := newEnv(t)
	res := e.api.Get("/api/stores/S1/items/" + string(apptest.SampleTask) + "/transfer-targets")
	var got []struct {
		StoreID, StoreName, WorkspaceID, WorkspaceName, Prefix string
		Projects                                               []struct {
			ID, Title string
			Epics     []struct{ ID, Display, Title string }
		}
	}
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil || len(got) != 2 {
		t.Fatalf("%d %s", res.Code, res.Body)
	}
	m, tm := got[0], got[1]
	if m.StoreName != "main" || m.WorkspaceName != "Mobile" || m.Prefix != "MB" || m.Projects[0].Title != "アプリ" ||
		m.Projects[0].Epics[0].Display != "MB-1" || m.Projects[0].Epics[0].Title != "iOS" ||
		tm.StoreName != "team" || tm.Prefix != "TM" {
		t.Errorf("targets = %+v", got)
	}
}

func TestPreview(t *testing.T) {
	e := newEnv(t)
	q := "?toStore=S1&toWorkspace=" + string(apptest.SampleMobileWorkspace) + "&toProject=" + string(apptest.SampleMobileProject)
	res := e.api.Get("/api/stores/S1/items/" + string(apptest.SampleTask) + "/transfer-preview" + q)
	want := `{"below":0,"deleted":0,"nextId":"MB-2","acrossStores":false,"remoteNote":false,"statuses":[]}`
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != want {
		t.Errorf("%d %s", res.Code, res.Body)
	}
	q = "?toStore=S2&toWorkspace=" + string(apptest.SampleTeamWorkspace) + "&toProject=" + string(apptest.SampleTeamProject)
	res = e.api.Get("/api/stores/S1/items/" + string(apptest.SampleTask) + "/transfer-preview" + q)
	if !strings.Contains(res.Body.String(), `"nextId":"TM-2","acrossStores":true,"remoteNote":true`) {
		t.Errorf("別の Store: %s", res.Body)
	}
}
```

移動(`move_test.go`): 同じ Store の移動と、別の Store の移動、送れなかったとき、404。

```go
func body(store, ws, pr, parent string) map[string]any {
	b := map[string]any{"toStore": store, "toWorkspace": ws, "toProject": pr, "statusChoices": map[string]string{}}
	if parent != "" {
		b["toParent"] = parent
	}
	return b
}

func TestTransferWithinTheStoreAndAcrossStores(t *testing.T) {
	e := newEnv(t)
	task := "/api/stores/S1/items/" + string(apptest.SampleTask)
	res := e.api.Post(task+"/transfer", body("S1", string(apptest.SampleMobileWorkspace), string(apptest.SampleMobileProject), string(apptest.SampleMobileEpic)))
	want := `{"toStore":"S1","itemId":"` + string(apptest.SampleTask) + `","display":"MB-2","emptied":[]}`
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != want {
		t.Fatalf("同じ Store: %d %s", res.Code, res.Body)
	}
	res = e.api.Post(task+"/transfer", body("S2", string(apptest.SampleTeamWorkspace), string(apptest.SampleTeamProject), string(apptest.SampleTeamEpic)))
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"toStore":"S2"`) || !strings.Contains(res.Body.String(), `"display":"TM-2"`) {
		t.Fatalf("別の Store: %d %s", res.Code, res.Body)
	}
	if res := e.api.Get("/api/stores/S2/items/" + string(apptest.SampleTask)); res.Code != 200 {
		t.Errorf("移動先で読めない: %d", res.Code)
	}
	if res := e.api.Get(task); res.Code != 404 {
		t.Errorf("移動元に残っている: %d", res.Code)
	}
}

// 送れなかったとき: 422。文は「{Store 名} に送信できないため移動できません: {理由}」。移動元はそのまま。
func TestTransferThatCannotBeSent(t *testing.T) {
	e := newEnv(t)
	e.reasons["S2"] = syncstatus.WriteDenied
	task := "/api/stores/S1/items/" + string(apptest.SampleTask)
	res := e.api.Post(task+"/transfer", body("S2", string(apptest.SampleTeamWorkspace), string(apptest.SampleTeamProject), string(apptest.SampleTeamEpic)))
	if res.Code != 422 || !strings.Contains(res.Body.String(), "team に送信できないため移動できません: remote への書き込みが許可されていません") {
		t.Errorf("%d %s", res.Code, res.Body)
	}
	if res := e.api.Get(task); res.Code != 200 || !strings.Contains(res.Body.String(), `"workspace":"`+string(apptest.SampleWorkspace)+`"`) {
		t.Errorf("移動元が変わった: %d %s", res.Code, res.Body)
	}
}

func TestTransferNotFound(t *testing.T) {
	e := newEnv(t)
	epic := "/api/stores/S1/items/" + string(apptest.SampleEpic)
	if res := e.api.Post(epic+"/transfer", body("S1", string(apptest.SampleMobileWorkspace), string(apptest.SampleMobileProject), string(apptest.SampleMobileEpic))); res.Code != 404 {
		t.Errorf("Epic に親: %d", res.Code)
	}
	if res := e.api.Post(epic+"/transfer", body("S9", string(apptest.SampleMobileWorkspace), string(apptest.SampleMobileProject), "")); res.Code != 404 {
		t.Errorf("ない Store: %d", res.Code)
	}
}
```

旧 ID(`locate_test.go`):

```go
func TestLocate(t *testing.T) {
	e := newEnv(t)
	task := string(apptest.SampleTask)
	res := e.api.Get("/api/locate?store=main&id=HH-2") // いまの ID
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"kind":"found"`) || !strings.Contains(res.Body.String(), `"itemId":"`+task+`"`) || strings.Contains(res.Body.String(), "movedFrom") {
		t.Fatalf("いまの ID: %d %s", res.Code, res.Body)
	}
	e.api.Post("/api/stores/S1/items/"+task+"/transfer", body("S2", string(apptest.SampleTeamWorkspace), string(apptest.SampleTeamProject), string(apptest.SampleTeamEpic)))
	res = e.api.Get("/api/locate?store=main&id=HH-2") // 旧 ID は team へ辿り着く
	var got struct {
		Kind, StoreID, ProjectID, ItemID string
		MovedFrom                        struct{ Old, At, Display string }
	}
	if res.Code != 200 || json.Unmarshal(res.Body.Bytes(), &got) != nil ||
		got.Kind != "found" || got.StoreID != "S2" || got.ItemID != task || got.ProjectID != string(apptest.SampleTeamProject) ||
		got.MovedFrom.Old != "HH-2" || got.MovedFrom.Display != "TM-2" || !strings.HasPrefix(got.MovedFrom.At, "2026-11-12T09:00:") {
		t.Errorf("旧 ID: %d %s", res.Code, res.Body)
	}
	if err := apptest.DeleteItems(t.Context(), e.stores, "S2", apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	res = e.api.Get("/api/locate?store=main&id=HH-2") // 削除済みは、画面を開かず文を返す
	want := `{"kind":"gone","message":"HH-2 は 11/12 09:00 に TM-2 へ移動しましたが、削除されています。設定の「削除済み」から復元できます"}`
	if res.Code != 200 || strings.TrimSpace(res.Body.String()) != want {
		t.Errorf("削除済み: %d %s", res.Code, res.Body)
	}
	if res := e.api.Get("/api/locate?store=main&id=ZZ-9"); res.Code != 404 {
		t.Errorf("見つからない: %d", res.Code)
	}
}
```

- [ ] **Step 2: 失敗を確かめる**

`go test ./internal/adapter/httpapi/transfers/` → `undefined: transfers.Register`(コンパイルエラー)。

- [ ] **Step 3: 実装する**

`items/move.go` の書き方に合わせる(本文の型には機能の名前を付ける: `TransferTarget`・`TransferPreview`・`TransferItem`・`TransferResult`・`Located`。空の配列は `[]`、`omitempty` は `movedFrom` と `Located` の任意の項目だけ)。`transfer-item` の応答は 200 で本文あり。エラーは `apierr.From` に任せるが、`*apptransfers.SendError` は `errors.As` で拾って `huma.Error422UnprocessableEntity(se.Error())`(`apierr` は変えない。この API だけの扱い)。`locate-item` は `Located.Found` なら `kind: "found"`(`message` は出さない)、そうでなければ `kind: "gone"` と `message`。`at` は `MovedFrom.At.Format(change.TimeLayout)`。

- [ ] **Step 4: 通す**

`go test ./internal/adapter/httpapi/transfers/` → PASS。

#### 8-3 つなぐ(`routes.go`・`wire.go`・`wire_e2e.go`)

- [ ] **Step 1: 失敗するテストを書く**

`cmd/hedgehug/wire_e2e_test.go` に足す(ビルドタグ `e2e`)。`wire` が組んだ `Deps` で `httpapi.NewAPI` を作り、同じ Store の移動・別の Store(remote なし)への移動・remote あり(`e2e` の Store)への移動が、本物の `Delivery`(`syncing.Service`)と偽の `Replicator` で通ることを、`http.ServeMux` に直接要求して確かめる。

```go
func TestE2EWireMovesItemsWithinAndAcrossStores(t *testing.T) {
	t.Setenv("HEDGEHUG_TOKEN", "secret")
	t.Setenv("HEDGEHUG_ADDR", "127.0.0.1:0")
	deps, err := wire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	httpapi.NewAPI(mux, deps)
	post := func(path string, body map[string]any) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b)))
		return rec
	}
	transfer := func(store, id string) string { return "/api/stores/" + store + "/items/" + id + "/transfer" }
	none := map[string]string{}
	// 同じ Store の Mobile へ → team(remote なし)へ → e2e(remote あり。偽の Replicator)へ戻す。
	steps := []struct {
		path string
		body map[string]any
		want string
	}{
		{transfer(e2eStore, string(apptest.SampleTask)), map[string]any{"toStore": e2eStore, "toWorkspace": apptest.SampleMobileWorkspace, "toProject": apptest.SampleMobileProject, "toParent": apptest.SampleMobileEpic, "statusChoices": none}, `"display":"MB-2"`},
		{transfer(e2eStore, string(apptest.SampleTask)), map[string]any{"toStore": teamStore, "toWorkspace": apptest.SampleTeamWorkspace, "toProject": apptest.SampleTeamProject, "toParent": apptest.SampleTeamEpic, "statusChoices": none}, `"display":"TM-2"`},
		{transfer(teamStore, string(apptest.SampleTask)), map[string]any{"toStore": e2eStore, "toWorkspace": apptest.SampleWorkspace, "toProject": apptest.SampleProject, "toParent": apptest.SampleEpic, "statusChoices": none}, `"display":"HH-3"`},
	}
	for i, s := range steps {
		if rec := post(s.path, s.body); rec.Code != 200 || !strings.Contains(rec.Body.String(), s.want) {
			t.Fatalf("手順 %d: %d %s", i+1, rec.Code, rec.Body)
		}
	}
}
```

(`SyncSave` の偽物は、`Errs` に何も置かなければ成功する(Task 4)。import は `bytes`・`encoding/json`・`net/http`・`net/http/httptest`・`strings`・`httpapi`・`apptest`。)

あわせて、同じファイルに、`wire` の Store 一覧が 3 つ(`e2e`・`readonly`・`team`)で、`e2e` の Store に 2 つのワークスペース(Web・Mobile)、`team` にワークスペース「Team」があることを確かめるテストを足す。

- [ ] **Step 2: 失敗を確かめる**

`go test -tags e2e ./cmd/hedgehug/` → `undefined: teamStore`(コンパイルエラー)。

- [ ] **Step 3: 実装する**

- `routes.go`: `Deps` に `Delivery apptransfers.Delivery` と `SyncTargets port.SyncTargets` を足す(コメント: `cmd` が `syncing.Service` を渡す)。`NewAPI` に `transfers.Register(api, apptransfers.Service{Stores: deps.Stores, StoreList: deps.StoreList, Targets: deps.SyncTargets, Delivery: deps.Delivery, Identity: deps.Identity, Clock: deps.Clock})` を足し、`apphistory.Service{... StoreList: deps.StoreList}` にする。
- `wire.go`: `svc` に、Task 4 が足した保存の口(`Stores`)を `syncing.NotifyWrites(stores, auto.Wrote)` で渡す(`auto` は `svc` から作るので、`auto` を作った直後に代入する。指摘 5)。`deps` に `Delivery: svc`・`SyncTargets: noSyncTargets{}`。
- `wire_e2e.go`: 定数 `teamStore = "01E2ESTORE0000000000000003"`。Store の組み立てのループを、`e2eStore` と `readonlyStore` に `Sample`、`e2eStore` にだけ `SampleMobile`、`teamStore` に `SampleTeam` に直す。`StoreList` に `{ID: teamStore, Name: "team", Color: "green"}`。`e2eSync(hub, stores)` に Store を渡し、`Targets` に remote なしの `team` を足し、`Stores: stores` を入れる。`Deps` に `Delivery: syncSvc`・`SyncTargets: syncSvc.Targets`。
- `make generate` → `pnpm -C web gen` で `api/openapi.yaml`・`generated.ts` を作り直す。

- [ ] **Step 4: 通す**

`go test ./internal/... ./cmd/...` と `go test -tags e2e ./cmd/hedgehug/` が PASS。`make check` が通る(`golangci-lint` は `-tags e2e` のビルドも見る)。

- [ ] **Step 5: 既存の通しのテストが壊れていないか、1 回だけ確かめる**

`pnpm -C web e2e` を 1 回流す。サンプルに Mobile と `team` が増えたので、ワークスペースや Store の数・並びを数えている既存の spec(`sidebar.spec.ts`・`sync.spec.ts`・`workspace-settings.spec.ts` など)が落ちたら、期待を新しいサンプルに合わせて直す(画面の挙動は変えない)。

- [ ] **Step 6: コミット**

```bash
git add internal/app/apptest internal/adapter/httpapi cmd/hedgehug api/openapi.yaml web/packages/api web/e2e
git commit -m "feat: アイテムの移動と旧 ID の解決の API をつなぎ、e2e のビルドに Store team とワークスペース Mobile を足す" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

---

### Task 9: 画面の口とダイアログのモデル(core)

> **この Task への上書き**(「本文より優先する直し」): `Located` の `gone` は `{reason, old, at, newDisplay, storeName?, nowDisplay?}`。


Task 8 がサーバーに足した 4 本の API を画面の口 `TransfersPort` に載せ、api の実装・偽物・ダイアログのモデル `createTransferDialog` を作る。ダイアログの部品(Svelte)・メニュー・パネルの切り替えは Task 10、画面のアドレスの解決は Task 11。

**この Task が決めること:**

- ダイアログの選択肢の値は `{Store の ID}/{ワークスペースの ID}`(内部 ID に `/` は含まれない)。選択肢の見え方は「{Store 名} / {ワークスペース名}」(履歴の「{旧 ID} から移動」の書き方に合わせる。A5a-17)。
- 予告(`TransferPreview`)は、移動先のワークスペースかプロジェクトが変わるたびに読み直す。親の Epic を選び直しても読み直さない(予告は親に依存しない)。古い読み込みの結果は捨てる。
- 書き込める Store・非表示でない・プロジェクトがあるワークスペースの絞り込みはサーバー(`transfer-targets`、A5a-13)が済ませて返す。画面は並びをそのまま使う。
- ボタン「移動」を押したときの失敗は、トーストを出して**ダイアログを開いたままにする**(移動先を選び直して送り直せるため)。削除・移動(A4b)が閉じるのと違うので、骨組みへの指摘 6 に書いた。ただし `NotFound`(移すアイテムが別の経路で消えた)は閉じて画面を読み直す。
- 移したあとの詳細パネルの切り替えと読み直しは、口 `afterMoved(from, to)` に任せる(Task 10 が作り、wire でつなぐ)。モデルは「トーストを出す → `afterMoved` を呼ぶ」までを持つ。
- API の応答のキー(`generated.ts`)は、骨組みの A5a-14・A5a-19 にキー名がないので、この Task が決め、Task 8 が合わせる(骨組みへの指摘 1)。

**Files:**
- Create: `web/packages/core/src/model/transfer.ts`・`web/packages/core/src/port/transfers.ts`
- Create: `web/packages/core/src/features/transfers/transfers.test-util.ts`(偽物)
- Create: `web/packages/core/src/features/transfers/dialogText.ts`・`dialogText.test.ts`
- Create: `web/packages/core/src/features/transfers/dialog.ts`・`dialog.test.ts`・`dialogSubmit.test.ts`
- Create: `web/packages/api/src/transfers.ts`・`transfers.test.ts`
- Modify: `web/packages/api/src/index.ts`(`createTransfersPort` を足す)・`web/packages/api/src/generated.ts`・`api/openapi.yaml`(生成)
- Modify: `web/packages/core/src/actionsIndex.ts`(24 行。型と `createTransferDialog` を足す)

**Interfaces:**
- Consumes: Task 8 の 4 本の API(骨組みの「HTTP API」の表)。`call`(api の `errors.ts`)。core の `Catalog`・`Toasts`・`Detail`・`Drafts`・`ItemRefresh`・`Option`(`features/projects/createDialog`)・`itemDisplayId`・`isItemDraft`・`readOnlyNote`・`Refused`・`NotFound`。
- Produces:
  - `model/transfer.ts`(すべて `@hedgehug/core` から export):

    ```ts
    export interface TransferEpic { id: string; display: string; title: string }
    export interface TransferProject { id: string; title: string; epics: TransferEpic[] }
    export interface TransferTarget {
      storeId: string; storeName: string;
      workspaceId: string; workspaceName: string; prefix: string;
      projects: TransferProject[];
    }
    export interface TransferDestination { storeId: string; workspaceId: string; projectId: string }
    export interface StatusChoice { id: string; name: string }
    // ステータスの置き換えの 1 行。status は使えない元のステータスの内部 ID、count は移すもののうちその件数。
    export interface StatusReplacement {
      status: string; name: string; count: number; choices: StatusChoice[]; default: string;
    }
    export interface TransferPreview {
      below: number;                  // 下の削除されていないものの件数 N
      deleted: number;                // 下の削除済みの件数 M
      nextDisplay: string;            // 移動先の次の番号の ID(HH-13)
      crossStore: boolean;            // 別の Store か
      remoteNotice: boolean;          // 「移動先に remote がないため…」を出すか
      destinationHasRemote: boolean;  // 移動先の Store に remote があるか(オフラインの判断に使う。指摘 1)
      replacements: StatusReplacement[]; // 移動元のワークスペースのステータスの並び
    }
    export interface TransferInput {
      toStore: string; toWorkspace: string; toProject: string;
      toParent?: string;              // Epic を移すときは付けない
      statusChoices: Record<string, string>; // 元のステータスの内部 ID → 移動先のステータスの内部 ID
    }
    export interface TransferResult { toStore: string; itemId: string; display: string; emptied: string[] }
    export interface MovedFrom { old: string; at: string; display: string } // at は ISO 8601
    export type Located =
      | { kind: "found"; storeId: string; projectId: string; itemId: string; movedFrom?: MovedFrom }
      | { kind: "gone"; message: string };
    ```
  - `port/transfers.ts`:

    ```ts
    export interface TransfersPort {
      targets(storeId: string, itemId: string): Promise<TransferTarget[]>;
      preview(storeId: string, itemId: string, to: TransferDestination): Promise<TransferPreview>;
      transfer(storeId: string, itemId: string, input: TransferInput): Promise<TransferResult>;
      // locate は、アドレスの Store 名と ID(旧 ID・旧い接頭辞でもよい)をいまの場所にする。ないものは NotFound。
      locate(storeName: string, id: string): Promise<Located>;
    }
    ```
    失敗は、断りなら `Refused`(message は画面にそのまま出す文)、ないものなら `NotFound`、それ以外はそのまま投げる。
  - api(`@hedgehug/api`): `createTransfersPort(): TransfersPort`
  - 偽物 `fakeTransfers(): FakeTransfers`(`transfers.test-util.ts`): `calls: string[]`(`targets {id}`・`preview {id} {store}/{workspace}/{project}`・`transfer {id} {JSON}`・`locate {store} {id}`)、答え `targetList`・`previewOf: Record<string, TransferPreview>`(キーは `{store}/{workspace}/{project}`)・`defaultPreview`・`result`・`located`、失敗 `refuseNext(message)`・`failNext()`・`missing: Set<string>`(入れた ID の操作は `NotFound`。`locate` は引数の ID で見る)・`holdNext(): { release }`(次の `preview` か `transfer` を待たせる)
  - core のモデル(`dialog.ts`):

    ```ts
    export interface ReplacementRow { status: string; name: string; label: string; options: Option[]; value: string }
    export interface TransferDialogState {
      item: Item; displayId: string;
      workspaces: Option[]; projects: Option[];
      parents: Option[] | null;       // Epic を移すときは null(親を選ばない)
      form: { workspace: string; project: string; parent: string };
      loading: boolean;               // 予告を読んでいる間。intro・changes・replacements は空
      intro: string;                  // 冒頭の文(1〜2 文)
      noEpic: string;                 // "" か「このプロジェクトには Epic がありません」
      changes: string[];              // 見出し「移動先で変わること」の下の行
      replacements: ReplacementRow[]; // 見出し「ステータスの置き換え」の下の行(なければ空で、見出しも出さない)
      canSubmit: boolean; busy: boolean;
    }
    export interface TransferDialogDeps {
      transfers: Pick<TransfersPort, "targets" | "preview" | "transfer">;
      catalog: Catalog; toasts: Toasts;
      refresh: Pick<ItemRefresh, "afterChange">;
      detail: Pick<Detail, "view">; drafts: Pick<Drafts, "confirmLeave">;
      online(): boolean;
      afterMoved(from: Item, to: TransferResult): Promise<void>;
    }
    export interface TransferDialog {
      state: Readable<TransferDialogState | null>;
      open(item: Item): Promise<void>;
      setWorkspace(key: string): Promise<void>;
      setProject(projectId: string): Promise<void>;
      setParent(epicId: string): void;
      setReplacement(status: string, to: string): void;
      submit(): Promise<void>;
      cancel(): void;
    }
    export function createTransferDialog(deps: TransferDialogDeps): TransferDialog;
    ```
  - 文言の定数と関数(`dialogText.ts`。Task 10 以降も使う): `noTargetsMessage`・`noEpicMessage`・`changesHeading`・`replacementsHeading`・`transferFailedMessage`・`offlineMessage(storeName)`・`transferredMessage(oldId, display, emptied)`・`transferIntro(displayId, title, below, deleted)`・`changeRows(p: TransferPreview)`

- [ ] **Step 1: 生成物を作り、関数名を確かめる**

Run:

```bash
make generate
pnpm -C web gen
grep -c "^export const \(listTransferTargets\|getTransferPreview\|transferItem\|locateItem\) " web/packages/api/src/generated.ts
git status --short api web/packages/api/src/generated.ts
```

Expected: 件数は `4`。差分は出ない(Task 8 が作り直している)。出たときは作り直し忘れなので、その差分もこの Task のコミットに入れる。関数の引数の形(クエリは最後の引数のオブジェクト)は `grep -n "export const getTransferPreview\|export const locateItem" -A3 web/packages/api/src/generated.ts` で確かめる。関数の名前や応答のキー(下の api のテストのキー)が違うときは、Task 8 の operationId とキーを直す(この Task では直さない)。

- [ ] **Step 2: モデルの型と口を書く**

`web/packages/core/src/model/transfer.ts` と `web/packages/core/src/port/transfers.ts` を、上の Produces の型のとおり作る(内容はそのまま。コメントは各型の 1 行)。`actionsIndex.ts` に `export type { … } from "./model/transfer"`(上の型すべて)と `export type { TransfersPort } from "./port/transfers"` を足す。

Run: `pnpm -C web check`
Expected: エラーなし

- [ ] **Step 3: api の実装の失敗するテストを書く**

`web/packages/api/src/transfers.test.ts`(`itemActions.test.ts` と同じ `respond`・`call` の書き方)。

```ts
describe("transfers port", () => {
  it("reads the targets, turning null projects and epics into empty lists", async () => {
    const fetch = respond(200, [
      { storeId: "S1", storeName: "work", workspaceId: "W2", workspaceName: "Mobile", prefix: "MB",
        projects: [{ id: "P1", title: "アプリ", epics: null }] },
    ]);
    expect(await createTransfersPort().targets("S1", "I1")).toEqual([
      { storeId: "S1", storeName: "work", workspaceId: "W2", workspaceName: "Mobile", prefix: "MB",
        projects: [{ id: "P1", title: "アプリ", epics: [] }] },
    ]);
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/transfer-targets");
  });

  it("reads the preview for a destination, with the status replacements", async () => {
    const fetch = respond(200, {
      below: 2, deleted: 1, nextDisplay: "MB-13", crossStore: true, remoteNotice: true,
      destinationHasRemote: false,
      replacements: [{ status: "st9", name: "Review", count: 3, choices: [{ id: "st1", name: "Todo" }], default: "st1" }],
    });
    const p = await createTransfersPort().preview("S1", "I1", { storeId: "S3", workspaceId: "W9", projectId: "P9" });
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/transfer-preview?toStore=S3&toWorkspace=W9&toProject=P9");
    expect(p.replacements[0]).toEqual({ status: "st9", name: "Review", count: 3, choices: [{ id: "st1", name: "Todo" }], default: "st1" });
    expect(p).toMatchObject({ below: 2, deleted: 1, nextDisplay: "MB-13", crossStore: true, remoteNotice: true });
  });

  it("posts the transfer and returns the new item, a null emptied list being empty", async () => {
    const fetch = respond(200, { toStore: "S3", itemId: "N1", display: "TM-4", emptied: null });
    const input = { toStore: "S3", toWorkspace: "W9", toProject: "P9", toParent: "E1", statusChoices: { st9: "st1" } };
    expect(await createTransfersPort().transfer("S1", "I1", input)).toEqual({ toStore: "S3", itemId: "N1", display: "TM-4", emptied: [] });
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/transfer");
    expect(call(fetch)[1].method).toBe("POST");
    expect(JSON.parse(String(call(fetch)[1].body))).toEqual(input);
  });

  it("turns a 422 into Refused with the message, and a 404 into NotFound", async () => {
    respond(422, { title: "Unprocessable Entity", status: 422, detail: "team に送信できないため移動できません: 認証に失敗しました" });
    await expect(createTransfersPort().transfer("S1", "I1", { toStore: "S3", toWorkspace: "W", toProject: "P", statusChoices: {} }))
      .rejects.toEqual(new Refused("team に送信できないため移動できません: 認証に失敗しました"));
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createTransfersPort().targets("S1", "I1")).rejects.toBeInstanceOf(NotFound);
  });

  it("locates an address: found with movedFrom, gone with the message, 404 as NotFound", async () => {
    const found = { kind: "found", storeId: "S3", projectId: "P9", itemId: "N1", movedFrom: { old: "HH-5", at: "2026-11-13T00:05:00Z", display: "TM-4" } };
    const fetch = respond(200, found);
    expect(await createTransfersPort().locate("work", "HH-5")).toEqual(found);
    expect(call(fetch)[0]).toBe("/api/locate?store=work&id=HH-5");
    respond(200, { kind: "gone", message: "HH-5 は 11/13 09:05 に TM-4 へ移動しましたが、完全に削除されました" });
    expect(await createTransfersPort().locate("work", "HH-5")).toEqual({ kind: "gone", message: "HH-5 は 11/13 09:05 に TM-4 へ移動しましたが、完全に削除されました" });
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createTransfersPort().locate("work", "HH-5")).rejects.toBeInstanceOf(NotFound);
  });
});
```

(`Refused`・`NotFound` は `@hedgehug/core` から import。Store 名はクエリで `encodeURIComponent` される。)

Run: `pnpm -C web --filter @hedgehug/api test -- transfers`
Expected: FAIL(`./transfers` がない)

- [ ] **Step 4: api の実装を書く**

`web/packages/api/src/transfers.ts`: `createTransfersPort(): TransfersPort`。`itemActions.ts` と同じく生成物の型名は import せず、`call` で包み、`projects`・`epics`・`emptied`・`choices`・`replacements` の null を空の配列にする。`locate` の応答は `kind` で `found`・`gone` に分ける。`index.ts` に `export { createTransfersPort } from "./transfers";` を足す。

Run: `pnpm -C web --filter @hedgehug/api test -- transfers && pnpm -C web check`
Expected: PASS 5 件

- [ ] **Step 5: 偽物を書く**

`transfers.test-util.ts`(`actions.test-util.ts` と同じ作り。`calls` の文字列と `previewOf`・`defaultPreview`・`result`・`located`・`refuseNext`・`failNext`・`missing`・`holdNext` は上の Produces のとおり)。`defaultPreview` の初期値は `{ below: 0, deleted: 0, nextDisplay: "MB-2", crossStore: false, remoteNotice: false, destinationHasRemote: false, replacements: [] }`。`result` の初期値は `{ toStore: "S1", itemId: "S1-I9", display: "MB-2", emptied: [] }`。`holdNext` で待たせるのは `preview` と `transfer` の次の 1 回。

- [ ] **Step 6: 文言の失敗するテストを書く**

`dialogText.test.ts`:

```ts
describe("transferIntro (02「移す先の選び方」)", () => {
  it.each([
    [0, 0, "HH-3 決済 を移動します。"],
    [2, 0, "HH-1 認証 と、その下の 2 件を移動します。"],
    [2, 1, "HH-1 認証 と、その下の 2 件を移動します。削除済みの 1 件も一緒に移ります。"],
    [0, 4, "HH-3 決済 を移動します。削除済みの 4 件も一緒に移ります。"],
  ])("below %d, deleted %d", (below, deleted, want) => {
    const [id, title] = below === 2 ? ["HH-1", "認証"] : ["HH-3", "決済"];
    expect(transferIntro(id, title, below, deleted)).toBe(want);
  });
});

describe("changeRows", () => {
  const p = { below: 0, deleted: 0, nextDisplay: "HH-13", crossStore: false, remoteNotice: false,
    destinationHasRemote: false, replacements: [] };
  it("lists the rows for a move inside one Store", () => {
    expect(changeRows(p)).toEqual([
      "ID は HH-13 からの番号に振り直されます",
      "スプリントとリリースは外されます",
      "優先度・関連システムなどは、同じ名前の値に合わせます。合う値がなければ空になります。ステータスは、同じ名前で移動先のプロジェクトが使うものに合わせ、合わないものは「ステータスの置き換え」で選んだステータスにします",
      "移動元に転送の記録(移した日時を含む)を残し、古い ID からも辿れます",
    ]);
  });
  it("adds the cross-Store rows after the value row, and the remote notice only when asked", () => {
    const rows = changeRows({ ...p, crossStore: true });
    expect(rows.slice(3, 5)).toEqual([
      "移さないアイテムとの関連は解除されます。移すもの同士の関連は残ります",
      "移動先のメンバーにいない人は、移動先のメンバーに加わります",
    ]);
    expect(rows).not.toContain("移動先に remote がないため、ほかの人の Store からは見えなくなります。");
    expect(changeRows({ ...p, crossStore: true, remoteNotice: true })[5]).toBe(
      "移動先に remote がないため、ほかの人の Store からは見えなくなります。");
    expect(changeRows({ ...p, crossStore: true, remoteNotice: true }).at(-1)).toBe(
      "移動元に転送の記録(移した日時を含む)を残し、古い ID からも辿れます");
  });
});

describe("messages", () => {
  it("builds the done toast (02「保存」)", () => {
    expect(transferredMessage("HH-5", "MB-2", [])).toBe("HH-5 は MB-2 になりました");
    expect(transferredMessage("HH-5", "MB-2", ["優先度", "関連システム"])).toBe(
      "HH-5 は MB-2 になりました。合う値がなく空になった項目: 優先度・関連システム");
  });
  it("has the fixed texts", () => {
    expect(offlineMessage("team")).toBe("オフラインのため team へ移動できません");
    expect(noTargetsMessage).toBe("移せるワークスペースがありません(プロジェクトのある、書き込めるワークスペースが要ります)");
    expect(noEpicMessage).toBe("このプロジェクトには Epic がありません");
    expect([changesHeading, replacementsHeading, transferFailedMessage]).toEqual(["移動先で変わること", "ステータスの置き換え", "移動できませんでした"]);
  });
});
```

Run: `pnpm -C web --filter @hedgehug/core test -- dialogText`
Expected: FAIL(`./dialogText` がない)

- [ ] **Step 7: 文言を実装する**

`dialogText.ts` に、Step 6 の定数と関数を書く。行の並びは、ID の行・スプリントとリリースの行・値の引き直しの行・(別の Store なら)関連の行・メンバーの行・(`remoteNotice` なら)remote の行・転送記録の行。

Run: `pnpm -C web --filter @hedgehug/core test -- dialogText`
Expected: PASS

- [ ] **Step 8: ダイアログのモデルの失敗するテストを書く**

`dialog.test.ts`(開く・選び直し)。準備: `fakeCatalogPort`(Store `work`=S1 が書ける、`ro`=S2 が閲覧のみ)・`fakeTransfers()`・`fakeItemActions` ではなく自前の `afterMoved` の記録。`targetList` は 2 件: S1 の「Mobile」(`S1-W2`、プロジェクト `S1-W2-P1`「アプリ」に Epic `S1-W2-E1` MB-1「通知」、プロジェクト `S1-W2-P2`「空」に Epic なし)と、S3 の「Team」(`S3-W1`、プロジェクト `S3-W1-P1` に Epic `S3-W1-E1` TM-4「認可」)。移すアイテムは `item("S1", "S1-P1", 3, "Task", null, "決済")`(表示 ID は HH-3)と `item("S1", "S1-P1", 1, "Epic", null, "認証")`(HH-1)。

```ts
it("shows a toast and opens nothing when there is no candidate", async () => {
  fake.targetList = [];
  await dialog.open(task);
  expect(get(dialog.state)).toBeNull();
  expect(toastTexts()).toEqual([noTargetsMessage]);
});

it("does nothing for an item in a read-only Store", async () => {
  await dialog.open(item("S2", "S2-P1", 3, "Task", null));
  expect(fake.calls).toEqual([]);
  expect(get(dialog.state)).toBeNull();
});

it("selects the first workspace, project and Epic, and reads the preview for them", async () => {
  fake.defaultPreview = { ...fake.defaultPreview, below: 0, deleted: 0, nextDisplay: "MB-2" };
  await dialog.open(task);
  expect(fake.calls).toEqual(["targets S1-I3", "preview S1-I3 S1/S1-W2/S1-W2-P1"]);
  const s = get(dialog.state);
  expect(s?.workspaces).toEqual([
    { value: "S1/S1-W2", label: "work / Mobile" },
    { value: "S3/S3-W1", label: "team / Team" },
  ]);
  expect(s?.projects.map((o) => o.label)).toEqual(["アプリ", "空"]);
  expect(s?.parents).toEqual([{ value: "S1-W2-E1", label: "MB-1 通知" }]);
  expect(s?.form).toEqual({ workspace: "S1/S1-W2", project: "S1-W2-P1", parent: "S1-W2-E1" });
  expect(s?.intro).toBe("HH-3 決済 を移動します。");
  expect(s?.changes[0]).toBe("ID は MB-2 からの番号に振り直されます");
  expect(s?.canSubmit).toBe(true);
});

it("does not select a parent for an Epic and can submit in a project without Epics", async () => {
  await dialog.open(epic);
  expect(get(dialog.state)?.parents).toBeNull();
  await dialog.setProject("S1-W2-P2");
  expect(get(dialog.state)?.noEpic).toBe("");
  expect(get(dialog.state)?.canSubmit).toBe(true);
});

it("says the project has no Epic and cannot submit, for a Task", async () => {
  await dialog.open(task);
  await dialog.setProject("S1-W2-P2");
  const s = get(dialog.state);
  expect(s?.noEpic).toBe("このプロジェクトには Epic がありません");
  expect(s?.parents).toEqual([]);
  expect(s?.canSubmit).toBe(false);
});

it("reads the preview again and rebuilds the status rows when the workspace or project changes", async () => {
  fake.previewOf["S1/S1-W2/S1-W2-P1"] = { ...fake.defaultPreview, replacements: [
    { status: "st9", name: "Review", count: 3, choices: [{ id: "st1", name: "Todo" }, { id: "st2", name: "Doing" }], default: "st1" }] };
  await dialog.open(task);
  expect(get(dialog.state)?.replacements).toEqual([
    { status: "st9", name: "Review", label: "Review(3 件)", options: [{ value: "st1", label: "Todo" }, { value: "st2", label: "Doing" }], value: "st1" }]);
  dialog.setReplacement("st9", "st2");
  expect(get(dialog.state)?.replacements[0]?.value).toBe("st2");
  await dialog.setWorkspace("S3/S3-W1");
  expect(fake.calls.at(-1)).toBe("preview S1-I3 S3/S3-W1/S3-W1-P1");
  expect(get(dialog.state)?.replacements).toEqual([]);
  expect(get(dialog.state)?.form).toEqual({ workspace: "S3/S3-W1", project: "S3-W1-P1", parent: "S3-W1-E1" });
  await dialog.setWorkspace("S1/S1-W2");
  expect(get(dialog.state)?.replacements[0]?.value).toBe("st1");
});

it("shows the loading state, with the button off, until the preview arrives, and drops a stale preview", async () => {
  const hold = fake.holdNext();
  const opening = dialog.open(task);
  await flush();
  expect(get(dialog.state)).toMatchObject({ loading: true, intro: "", changes: [], canSubmit: false });
  hold.release();
  await opening;
  expect(get(dialog.state)?.loading).toBe(false);
});

it("closes when cancelled, and ignores the preview that comes after", async () => {
  const hold = fake.holdNext();
  const opening = dialog.open(task);
  await flush();
  dialog.cancel();
  hold.release();
  await opening;
  expect(get(dialog.state)).toBeNull();
});

it("asks to discard a draft first when the panel shows the item, and stops on cancel", async () => {
  panel.set({ item: task } as DetailView);
  confirm.mockResolvedValueOnce(false);
  await dialog.open(task);
  expect(fake.calls).toEqual([]);
  expect(get(dialog.state)).toBeNull();
});
```

`dialogSubmit.test.ts`(送信)。準備は同じ。

```ts
it("sends the selected destination and the status choices, then toasts and calls afterMoved", async () => {
  fake.previewOf["S1/S1-W2/S1-W2-P1"] = { ...fake.defaultPreview, replacements: [
    { status: "st9", name: "Review", count: 1, choices: [{ id: "st2", name: "Doing" }], default: "st2" }] };
  fake.result = { toStore: "S1", itemId: "S1-I3", display: "MB-2", emptied: [] };
  await dialog.open(task);
  await dialog.submit();
  expect(fake.calls.at(-1)).toBe(`transfer S1-I3 ${JSON.stringify({ toStore: "S1", toWorkspace: "S1-W2", toProject: "S1-W2-P1", toParent: "S1-W2-E1", statusChoices: { st9: "st2" } })}`);
  expect(toastTexts()).toEqual(["HH-3 は MB-2 になりました"]);
  expect(moved).toEqual(["HH-3 → S1/S1-I3"]);
  expect(get(dialog.state)).toBeNull();
});

it("omits toParent for an Epic and lists the emptied items in the toast", async () => {
  fake.result = { toStore: "S3", itemId: "N1", display: "TM-5", emptied: ["優先度", "関連システム"] };
  await dialog.open(epic);
  await dialog.setWorkspace("S3/S3-W1");
  await dialog.submit();
  expect(fake.calls.at(-1)).toBe(`transfer S1-I1 ${JSON.stringify({ toStore: "S3", toWorkspace: "S3-W1", toProject: "S3-W1-P1", statusChoices: {} })}`);
  expect(toastTexts()).toEqual(["HH-1 は TM-5 になりました。合う値がなく空になった項目: 優先度・関連システム"]);
});

it("shows a refusal as it is, keeps the dialog open, and does not call afterMoved", async () => {
  await dialog.open(task);
  fake.refuseNext("team に送信できないため移動できません: remote に接続できません");
  await dialog.submit();
  expect(toasts()).toEqual([{ text: "team に送信できないため移動できません: remote に接続できません", error: true }]);
  expect(get(dialog.state)?.busy).toBe(false);
  expect(moved).toEqual([]);
});

it("says 移動できませんでした for another failure", async () => {
  await dialog.open(task);
  fake.failNext();
  await dialog.submit();
  expect(toasts()).toEqual([{ text: "移動できませんでした", error: true }]);
  expect(get(dialog.state)).not.toBeNull();
});

it("refuses before sending when the destination has a remote and the browser is offline", async () => {
  online = false;
  fake.previewOf["S3/S3-W1/S3-W1-P1"] = { ...fake.defaultPreview, crossStore: true, destinationHasRemote: true };
  await dialog.open(task);
  await dialog.setWorkspace("S3/S3-W1");
  await dialog.submit();
  expect(fake.calls.some((c) => c.startsWith("transfer"))).toBe(false);
  expect(toasts()).toEqual([{ text: "オフラインのため team へ移動できません", error: true }]);
});

it("sends offline when the destination has no remote or the move stays inside the Store", async () => {
  online = false;
  await dialog.open(task);
  await dialog.submit();
  expect(fake.calls.at(-1)?.startsWith("transfer")).toBe(true);
});

it("closes and reloads the screen when the item is gone", async () => {
  await dialog.open(task);
  fake.missing.add("S1-I3");
  await dialog.submit();
  expect(get(dialog.state)).toBeNull();
  expect(toasts()).toEqual([{ text: "移動できませんでした", error: true }]);
  expect(refreshed).toBe(1);
});

it("sends only once while the first send is pending", async () => {
  await dialog.open(task);
  const hold = fake.holdNext();
  const first = dialog.submit();
  void dialog.submit();
  await flush();
  hold.release();
  await first;
  expect(fake.calls.filter((c) => c.startsWith("transfer")).length).toBe(1);
});
```

Run: `pnpm -C web --filter @hedgehug/core test -- features/transfers`
Expected: FAIL(`./dialog` がない)

- [ ] **Step 9: ダイアログのモデルを実装する**

`dialog.ts` に `createTransferDialog(deps)` を書く(上の Produces の形)。テストが決める振る舞いに加えて、次を守る。

- `open` は、閲覧のみの Store か、アイテムのワークスペースの接頭辞がカタログにないときは何もしない。開いているパネルが `item` か `item` の祖先のものなら、先に `deps.drafts.confirmLeave(isItemDraft)` を聞く(`delete.ts` と同じ。パネルの `view` の `ancestors` を使う)。
- 読み込み・選び直しは世代(`seq`)を持ち、古い結果を捨てる。`cancel` も世代を進める。
- 予告・候補の読み込みの失敗は、ダイアログを閉じ、`Refused` の文か `transferFailedMessage` をエラーのトーストで出す。`NotFound` は `deps.refresh.afterChange()` も呼ぶ(`move.ts` の `open` と同じ)。
- 移動先で変わる行・冒頭の文・ステータスの行は `dialogText.ts` の関数と `TransferPreview` から作る(状態に文字列を重ねて持たず、`state` を `set` するときに作る)。
- ステータスの行の `name` は元のステータス名、`label` は `{name}({count} 件)`、`options` は `choices` を `{ value: id, label: name }` にしたもの、初めの値は `default`。
- 送信の順: (1) `busy` なら何もしない、(2) 別の Store で移動先に remote があり `!deps.online()` ならオフラインのトーストだけ、(3) `transfers.transfer`、(4) 成功したらダイアログを閉じ、`transferredMessage` のトーストを出し、`await deps.afterMoved(item, result)`。

Run: `pnpm -C web --filter @hedgehug/core test -- features/transfers && pnpm -C web fix && pnpm -C web check && pnpm -C web size`
Expected: PASS(dialogText 5 件・dialog 8 件・dialogSubmit 8 件ほか)、エラーなし、300 行を超えるファイルなし

- [ ] **Step 10: export して、コミットする**

`actionsIndex.ts` に `export { createTransferDialog, type TransferDialog, type TransferDialogState } from "./features/transfers/dialog";` と、Task 10 以降が使う `dialogText.ts` の定数と関数の export を足す。

Run: `pnpm -C web check && pnpm -C web test`
Expected: エラーなし、すべて通る

```bash
git add api web
git commit -m "$(cat <<'EOF'
feat: 別のワークスペースへ移動の画面の口とダイアログのモデルを足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: ダイアログの部品・メニュー・パネルの切り替え(app)

> **この Task への上書き**(「本文より優先する直し」): 送信の失敗ではダイアログを開いたまま。


**この Task が決めること:**

- メニューの項目 `"transfer"`、ラベル「別のワークスペースへ移動…」、順は「移動…」の次・「種類を変更…」の前(骨組みの「画面」の表)。閲覧のみの Store では無効で「閲覧のみの Store です」を添える(`writes` に足す)。**Step の行・パネルには出さない**(仕様 02 に「Step は単独では移せない」とあるが、Step の理由の文言がないため。骨組みへの指摘 4)。候補がないことでは無効にしない(選んだら、ダイアログを開かずトースト。Task 9 のモデルが出す)。
- ダイアログの部品の読み上げの名前(Task 12 の e2e が使う): ダイアログのタイトル「別のワークスペースへ移動」、セレクトは「移動先のワークスペース」「プロジェクト」「親の Epic」、ステータスの行のセレクトは「{ステータス名} の置き換え先」、ボタンは「移動」「キャンセル」。見出しは `changesHeading`・`replacementsHeading`。
- 移したあと(`afterMoved`)は、開いているパネルが移したアイテム自身なら、閉じて(確認なし)から新しい Store・内部 ID で開き直し、そのあとツリー・パネル・区画を読み直す。パネルが別のアイテムなら読み直すだけ(移したものの子孫なら、読み直しで閉じる)。
- ほかの書き手が移したとき(A5a-18): `Detail.reload` が、開いているアイテムと同じ内部 ID で**ワークスペースが違う**アイテムを読んだら、確認を出さずに閉じる。別の Store へ移されたときは、いまも 404 で閉じる。

**Files:**
- Modify: `web/packages/core/src/features/items/menu.ts`・`menu.test.ts`・`menuHost.ts`・`menuHost.test-util.ts`・`menuHost.test.ts`
- Create: `web/packages/core/src/features/transfers/afterMoved.ts`・`afterMoved.test.ts`
- Modify: `web/packages/core/src/features/items/detail.ts`(271 行)
- Create: `web/packages/core/src/features/items/detailMoved.test.ts`
- Modify: `web/packages/core/src/actionsIndex.ts`(`createAfterMoved`)
- Create: `web/app/src/screens/item/TransferDialog.svelte`
- Modify: `web/app/src/App.svelte`(96 行)・`web/app/src/wireActions.ts`(107 行)・`web/app/src/wire.ts`(`transfersPort` を足す。275 行)

**Interfaces:**
- Consumes: Task 9 の `createTransferDialog`・`TransferDialogState`・文言の定数、`TransfersPort`・`createTransfersPort`。`itemRefresh`・`detail`・`drafts`・`catalog`・`toasts`・`online`(wire.ts の `online: Readable<boolean>`。`isOnline` 関数は `./tab` の `online`)
- Produces:
  - core: `ItemMenuId` に `"transfer"`、`transferLabel = "別のワークスペースへ移動…"`、`createItemMenuHost` の deps に `transfer: Pick<TransferDialog, "open">`、`createAfterMoved(deps: { detail: Pick<Detail, "view" | "close" | "open">; refresh: Pick<ItemRefresh, "afterChange"> }): (from: Item, to: TransferResult) => Promise<void>`
  - `wire.ts`: `export const transfersPort = createTransfersPort();`(画面のアドレスの解決も使うので wire.ts に置く。Task 11)
  - `wireActions.ts`: `itemTransfer: TransferDialog`(開くのは `itemTransfer.open(item)`)
  - app: `web/app/src/screens/item/TransferDialog.svelte`

- [ ] **Step 1: メニューの失敗するテストを書く**

`menu.test.ts` を次のとおり直し、足す。

- 「lists the detail-panel items…」の期待に `["transfer", "別のワークスペースへ移動…"]` を `["move", "移動…"]` の次に足す。
- 「starts the tree-row menu…」の最後の `.length` を `10` から `11` にする。
- 「has no 'add a child' for a Step row…」の `step` の期待は、`["open", ...itemMenu(ctx()).map((e) => e.id).filter((id) => id !== "transfer")]` にする。
- 足す:

```ts
it("disables 'transfer' only in a read-only Store, with the read-only note", () => {
  expect(byId(ctx({ readOnly: true })).transfer).toEqual({
    id: "transfer", label: "別のワークスペースへ移動…", disabled: true, hint: "閲覧のみの Store です" });
  for (const kind of ["Epic", "Story", "Task", "Bug"] as const) {
    expect(byId(ctx({ kind, hasOtherProjects: false })).transfer?.disabled).toBe(false);
  }
});

it("has no 'transfer' for a Step", () => {
  expect(itemMenu(ctx({ kind: "Step" })).map((e) => e.id)).not.toContain("transfer");
});
```

`menuHost.test-util.ts` の `createItemMenuHost` の deps に `transfer: { open: async (it: Item) => void log.push(`transfer ${it.id}`) }` を足す。`menuHost.test.ts` に足す:

```ts
it("opens the transfer dialog for the item when 'transfer' is selected", async () => {
  const { host, log } = await setup();
  host.open(t3, ctx, 10, 20);
  await host.select("transfer");
  expect(log).toEqual(["transfer S1-I3"]);
});

it("does not run 'transfer' in a read-only Store", async () => {
  const { host, log } = await setup();
  host.open(ro, ctx, 10, 20);
  await host.select("transfer");
  expect(log).toEqual([]);
});
```

Run: `pnpm -C web --filter @hedgehug/core test -- items/menu`
Expected: FAIL(`"transfer"` がない)

- [ ] **Step 2: メニューを実装する**

`menu.ts`: `ItemMenuId` に `"transfer"` を足し、`transferLabel` を export し、`writes` に足し、`itemMenu` の「移動…」の次に `entry("transfer", transferLabel)` を足す。ただし `ctx.kind === "Step"` では足さない。先頭のコメントの「「別のワークスペースへ移動…」は、その操作がないので出さない。」の行を、Step には出さないことの説明に直す。`menuHost.ts`: deps に `transfer: Pick<TransferDialog, "open">` を足し(`TransferDialog` は `../transfers/dialog` から type import)、`run` の `switch` に `case "transfer": return deps.transfer.open(item);` を足す。

Run: `pnpm -C web --filter @hedgehug/core test -- items/menu && pnpm -C web check`
Expected: menu・menuHost のテストが通る。`check` は `wireActions.ts` の `createItemMenuHost` に `transfer` がなくて失敗する(Step 6 で直す)

- [ ] **Step 3: 移したあとの切り替えの失敗するテストを書く**

`afterMoved.test.ts`。`detail` は `view`(writable)・`close`・`open` を持つ偽物で、`calls` に残す。`refresh.afterChange` も `calls` に `change` を残す。

```ts
const from = item("S1", "S1-P1", 3, "Task", null, "決済");
const result = { toStore: "S3", itemId: "S3-I9", display: "TM-4", emptied: [] };

it("closes the panel without asking, opens the new item, then reloads, when the panel shows the moved item", async () => {
  view.set({ item: from } as DetailView);
  await createAfterMoved({ detail, refresh })(from, result);
  expect(calls).toEqual(["close", "open S3/S3-I9", "change"]);
});

it("opens the same item again inside one Store (the internal ID stays)", async () => {
  view.set({ item: from } as DetailView);
  await createAfterMoved({ detail, refresh })(from, { ...result, toStore: "S1", itemId: from.id });
  expect(calls).toEqual(["close", "open S1/S1-I3", "change"]);
});

it("only reloads when the panel shows another item or is closed", async () => {
  view.set({ item: item("S1", "S1-P1", 4, "Task", null) } as DetailView);
  await createAfterMoved({ detail, refresh })(from, result);
  view.set(null);
  await createAfterMoved({ detail, refresh })(from, result);
  expect(calls).toEqual(["change", "change"]);
});

it("does not take an item of another Store with the same internal ID for the moved one", async () => {
  view.set({ item: { ...item("S2", "S2-P1", 3, "Task", null), id: from.id } } as DetailView);
  await createAfterMoved({ detail, refresh })(from, result);
  expect(calls).toEqual(["change"]);
});
```

`detailMoved.test.ts`(`detailGone.test.ts` と同じ `setup`)。

```ts
it("closes without asking when another writer moved the open item to another workspace (same internal ID)", async () => {
  const { detail, items, task } = await setup();
  await detail.open("S1", task.id);
  detail.toggleFull();
  items.all[items.all.indexOf(task)] = { ...task, workspaceId: "S1-W2" };
  await detail.reload();
  expect(get(detail.view)).toBeNull();
  expect(get(detail.full)).toBe(false);
});

it("keeps the panel when the reloaded item stays in the same workspace", async () => {
  const { detail, items, task } = await setup();
  await detail.open("S1", task.id);
  items.all[items.all.indexOf(task)] = { ...task, title: "新しい名前" };
  await detail.reload();
  expect(get(detail.view)?.item.title).toBe("新しい名前");
});

it("shows the item in its new workspace when it is opened (not reloaded) after we moved it ourselves", async () => {
  const { detail, items, task } = await setup();
  await detail.open("S1", task.id);
  items.all[items.all.indexOf(task)] = { ...task, workspaceId: "S1-W2" };
  detail.close();
  await detail.open("S1", task.id);
  expect(get(detail.view)?.item.workspaceId).toBe("S1-W2");
});
```

Run: `pnpm -C web --filter @hedgehug/core test -- afterMoved detailMoved`
Expected: FAIL(`afterMoved` がない・1 件目が閉じない)

- [ ] **Step 4: 切り替えと読み直しの閉じ方を実装する**

`afterMoved.ts`: `createAfterMoved`(Step 3 のとおり。パネルが移したアイテム自身かは、Store の ID と内部 ID の両方で比べる)。`detail.ts` の `load`: `deps.items.get` のあと、`refresh` が真で、開いているアイテム(`get(view)?.item`)と内部 ID が同じでワークスペースが違うときは、`mine === latest` なら `shut()` して返す(`shut` は確認しない)。`actionsIndex.ts` に `createAfterMoved` を足す。

Run: `pnpm -C web --filter @hedgehug/core test && pnpm -C web size`
Expected: PASS、`detail.ts` は 300 行以下

- [ ] **Step 5: ダイアログの部品を書く**

`web/app/src/screens/item/TransferDialog.svelte`(200 行以内)。値と送信はすべて core の `itemTransfer` が持つ。`ChangeKindDialog.svelte` と同じ作りで、`const dialog = itemTransfer.state;`。

- `Dialog`(`wide`、タイトル「別のワークスペースへ移動」、`onclose={itemTransfer.cancel}`)の中に、冒頭の `<p>{s.intro}</p>`、`Select`「移動先のワークスペース」(`options={s.workspaces}`、`value={s.form.workspace}`、`onchange={(v) => void itemTransfer.setWorkspace(v)}`)、`Select`「プロジェクト」、`s.parents !== null` のときだけ `Select`「親の Epic」。`s.noEpic` があれば `<p class="form-error">`(`role="alert"` は付けない。トーストの `status` と取り違えない)。
- `s.loading` のあいだは、冒頭の文・見出し・行を出さない(`intro` が空)。
- 見出し `<h3>` に `changesHeading`、その下に `<ul>` で `s.changes` の行。`s.replacements.length > 0` のとき、`<h3>` に `replacementsHeading`、行ごとに `<span>{row.label}</span>` と `Select`(ラベル `${row.name} の置き換え先`、`options={row.options}`、`value={row.value}`、`onchange={(v) => itemTransfer.setReplacement(row.status, v)}`)。
- フッターは「キャンセル」(`itemTransfer.cancel`)と「移動」(`variant="primary"`、`disabled={!s.canSubmit}`、`onclick={() => void itemTransfer.submit()}`)。
- 書き方は `CreateItemDialog.svelte` に合わせる(`.form` の縦並び・`.grow`)。

`App.svelte` の末尾の `<ChangeKindDialog />` の次に `<TransferDialog />` を 1 行足し、import を足す。

- [ ] **Step 6: 組み立てる**

`wire.ts`: `createItemsPort` と同じ import から `createTransfersPort` を import し、`export const itemsPort = …` の次に `export const transfersPort = createTransfersPort();` を足す。

`wireActions.ts`: `createTransferDialog`・`createAfterMoved` を import し、`itemMove` の次に足す。

```ts
// 「別のワークスペースへ移動…」のダイアログ(02「別のワークスペースと別の Store への移動」)。開くのは itemTransfer.open(item)。
export const itemTransfer = createTransferDialog({
  transfers: transfersPort,
  catalog,
  toasts,
  refresh: itemRefresh,
  detail,
  drafts,
  online: isOnline,
  afterMoved: createAfterMoved({ detail, refresh: itemRefresh }),
});
```

`itemMenuHost` の deps に `transfer: itemTransfer,` を足し、末尾の `navigation.route.subscribe` に `itemTransfer.cancel();` を足す。`isOnline` は `import { online as isOnline } from "./tab";`。

Run:

```bash
pnpm -C web fix && pnpm -C web check && pnpm -C web test && pnpm -C web size && pnpm -C web deps
wc -l web/app/src/wire.ts web/app/src/wireActions.ts web/app/src/screens/item/TransferDialog.svelte
```

Expected: エラーなし(`svelte-check` の警告も 0)。3 つとも上限(300・300・200)以内。`wire.ts` が 300 を超えるときは、`transfersPort` を `wireBase.ts` に置き、`wire.ts` はそこから import する。

- [ ] **Step 7: コミットする**

```bash
git add web
git commit -m "$(cat <<'EOF'
feat: メニュー「別のワークスペースへ移動…」と移動のダイアログを足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: 画面のアドレスの旧 ID の解決と、パネルの上部の 1 行

> **この Task への上書き**(「本文より優先する直し」): 見つからないときの 4 つの文を `formatDateTime` で作る。旧い接頭辞でのワークスペース・プロジェクトのアドレスも足す。


**この Task が決めること:**

- アイテムの入口(`/s/{Store 名}/i/{ID}`)は、まず今までどおり、いまの ID(Store 名の Store → サイドバーの並び)で探す。見つからなかったときだけ `locate`(旧 ID・旧い接頭辞・転送記録。A5a-15)を呼ぶ。普通の経路で `locate` を呼ばない。
- `locate` の答え: `found` ならそのアイテムを開く(`movedFrom` があれば、パネルの上部の 1 行用に渡す)。`gone` なら、`message` をトーストで出し(エラーの色にしない)、アドレスをホームに置き換える(`navigation.replace`。再び解決しても同じトーストを出さない)。`NotFound`(どの旧 ID でもない)は今までどおりホーム(トーストなし)。
- 見つからないときの 4 つの文は、サーバーが `message` に作る(Task 7・8)。画面は文を作らず、そのまま出す。文は仕様 02「見つからないとき」のとおり: 「{旧 ID} は {日時} に {新 ID} へ移動しましたが、削除されています。設定の「削除済み」から復元できます」/「…移動しましたが、完全に削除されました」/「{旧 ID} は {日時} に {Store 名} へ移動しました。この Store はいま表示できません」/「{旧 ID} は {日時} に {B の新 ID} へ移動し、いまは {C の新 ID} にあります」。
- 旧 ID から開いたときの 1 行(A5a-16): 「{旧 ID} は {日時} に {新 ID} へ移動しました」。詳細パネルの上部(種類の行の上)に 1 行。パネルを閉じるか別のアイテムを開けば消える。普通に開いたときは出さない。日時は「今年なら M/D、それ以外は YYYY/MM/DD」と「HH:MM」を空白でつなぐ(01「日付と時刻」)。`features/dates/date.ts` に `formatDateTime` を足す。
- 行には `role="status"` を付けない(トーストが `status` で、e2e が数える)。

**Files:**
- Modify: `web/packages/core/src/features/dates/date.ts`・`date.test.ts`
- Create: `web/packages/core/src/features/transfers/movedNotice.ts`・`movedNotice.test.ts`
- Modify: `web/packages/core/src/features/router/resolve.ts`・`resolve.test.ts`
- Modify: `web/packages/core/src/features/screen/screen.ts`・既存の screen のテスト
- Modify: `web/packages/core/src/actionsIndex.ts`(`createMovedNotice`)・`web/packages/core/src/index.ts`(`formatDateTime`。166 行)
- Modify: `web/app/src/wire.ts`・`web/app/src/screens/item/DetailPanel.svelte`(126 行)

**Interfaces:**
- Consumes: Task 9 の `TransfersPort.locate`・`Located`・`MovedFrom`。`ItemsPort.findByNumber`・`parseDisplayId`・`Toasts`・`Navigation`・`Detail.view`
- Produces:
  - `formatDateTime(at: Date, now: Date): string`(例: `2026-11-13 09:05` で今年なら `11/13 09:05`、去年なら `2025/12/31 23:59`)
  - `resolveRoute(route, stores, items, transfers: Pick<TransfersPort, "locate">): Promise<Resolved>`。`Resolved` の `item` に `movedFrom?: MovedFrom`、`home` に `gone?: string` を足す
  - `ScreenDeps` に `transfers: Pick<TransfersPort, "locate">`・`toasts: Toasts` を足し、`openItem(storeId, itemId, movedFrom?: MovedFrom)` にする
  - `createMovedNotice(deps: { detail: Pick<Detail, "view">; now(): Date }): MovedNotice`。`MovedNotice` は `{ text: Readable<string>; show(storeId: string, itemId: string, m: MovedFrom): void }`。`text` は、開いているパネルが `show` したアイテムのときだけ文、それ以外は空

- [ ] **Step 1: 日時の失敗するテストを書く**

`date.test.ts` に足す(時刻はローカル時刻で作る)。

```ts
describe("formatDateTime (01「日付と時刻」)", () => {
  const now = new Date(2026, 10, 14, 12, 0);
  it("shows M/D HH:MM for this year and YYYY/MM/DD HH:MM for other years", () => {
    expect(formatDateTime(new Date(2026, 10, 13, 9, 5), now)).toBe("11/13 09:05");
    expect(formatDateTime(new Date(2025, 11, 31, 23, 59), now)).toBe("2025/12/31 23:59");
  });
});
```

Run: `pnpm -C web --filter @hedgehug/core test -- dates/date`
Expected: FAIL(`formatDateTime` がない)

- [ ] **Step 2: 日時を実装する**

`date.ts` に `formatDateTime(at, now)` を足す(日付は `formatDay`、時刻は 2 桁ずつ)。`index.ts` の `./features/dates/date` の export に足す。

Run: `pnpm -C web --filter @hedgehug/core test -- dates/date`
Expected: PASS

- [ ] **Step 3: パネルの上部の 1 行の失敗するテストを書く**

`movedNotice.test.ts`。`detail.view` は `writable<DetailView | null>(null)`、`now` は `() => new Date(2026, 10, 14, 12, 0)`。移動の記録は `{ old: "HH-5", at: new Date(2026, 10, 13, 9, 5).toISOString(), display: "WEB-1" }`。

```ts
const line = "HH-5 は 11/13 09:05 に WEB-1 へ移動しました";

it("shows the line once the panel shows the item, not before", () => {
  notice.show("S1", "I5", moved);
  expect(get(notice.text)).toBe("");
  view.set({ item: { storeId: "S1", id: "I5" } } as DetailView);
  expect(get(notice.text)).toBe(line);
});

it("clears when another item is opened, and stays cleared when the first one is opened again", () => {
  notice.show("S1", "I5", moved);
  view.set({ item: { storeId: "S1", id: "I5" } } as DetailView);
  view.set({ item: { storeId: "S1", id: "I6" } } as DetailView);
  expect(get(notice.text)).toBe("");
  view.set({ item: { storeId: "S1", id: "I5" } } as DetailView);
  expect(get(notice.text)).toBe("");
});

it("clears when the panel is closed", () => {
  notice.show("S1", "I5", moved);
  view.set({ item: { storeId: "S1", id: "I5" } } as DetailView);
  view.set(null);
  expect(get(notice.text)).toBe("");
});

it("does not mix up the same internal ID of another Store, and a later show replaces the line", () => {
  notice.show("S1", "I5", moved);
  view.set({ item: { storeId: "S2", id: "I5" } } as DetailView);
  expect(get(notice.text)).toBe("");
  notice.show("S2", "I5", { ...moved, old: "HH-7", display: "WEB-2" });
  view.set({ item: { storeId: "S2", id: "I5" } } as DetailView);
  expect(get(notice.text)).toBe("HH-7 は 11/13 09:05 に WEB-2 へ移動しました");
});
```

Run: `pnpm -C web --filter @hedgehug/core test -- movedNotice`
Expected: FAIL(`./movedNotice` がない)

- [ ] **Step 4: 1 行を実装する**

`movedNotice.ts`: `createMovedNotice`。`show` したアイテムが一度パネルに出たあとで、別のアイテムか閉じた状態になったら消す(出る前の `null` では消さない。画面のアドレスの解決は、画面を移る購読がパネルを閉じたあとでパネルを開くので)。`actionsIndex.ts` に `createMovedNotice` と `type MovedNotice` を足す。

Run: `pnpm -C web --filter @hedgehug/core test -- movedNotice`
Expected: PASS 4 件

- [ ] **Step 5: アドレスの解決の失敗するテストを書く**

`resolve.test.ts` の `resolveRoute` の既存の呼び出しすべてに、第 4 引数 `fakeTransfers()` を足す。足すテスト:

```ts
it("does not call locate when the current ID is found", async () => {
  const r = await resolveRoute({ name: "item", store: "work", item: "HH-2" }, stores, items, transfers);
  expect(r).toMatchObject({ name: "item", itemId: "S1-I2" });
  expect(transfers.calls).toEqual([]);
});

it("asks locate with the Store name and the ID when the ID is not found, and opens what it finds with the moved-from note", async () => {
  transfers.located = { kind: "found", storeId: "S3", projectId: "S3-P1", itemId: "S3-I9",
    movedFrom: { old: "HH-5", at: "2026-11-13T00:05:00.000Z", display: "TM-4" } };
  const r = await resolveRoute({ name: "item", store: "work", item: "HH-5" }, stores, items, transfers);
  expect(transfers.calls).toEqual(["locate work HH-5"]);
  expect(r).toEqual({ name: "item", storeId: "S3", projectId: "S3-P1", itemId: "S3-I9",
    movedFrom: { old: "HH-5", at: "2026-11-13T00:05:00.000Z", display: "TM-4" } });
});

it("opens a found item without movedFrom when locate gives none (an old prefix)", async () => {
  transfers.located = { kind: "found", storeId: "S1", projectId: "S1-P1", itemId: "S1-I2" };
  const r = await resolveRoute({ name: "item", store: "work", item: "OLD-2" }, stores, items, transfers);
  expect(r).toEqual({ name: "item", storeId: "S1", projectId: "S1-P1", itemId: "S1-I2" });
});

it("gives the message of a gone item with home", async () => {
  transfers.located = { kind: "gone", message: "HH-5 は 11/13 09:05 に TM-4 へ移動しましたが、完全に削除されました" };
  expect(await resolveRoute({ name: "item", store: "work", item: "HH-5" }, stores, items, transfers))
    .toEqual({ name: "home", gone: "HH-5 は 11/13 09:05 に TM-4 へ移動しましたが、完全に削除されました" });
});

it("is plain home when locate has nothing (NotFound), and does not ask for an unreadable ID", async () => {
  transfers.missing.add("HH-99");
  expect(await resolveRoute({ name: "item", store: "work", item: "HH-99" }, stores, items, transfers)).toEqual({ name: "home" });
  transfers.calls.length = 0;
  expect(await resolveRoute({ name: "item", store: "work", item: "abc" }, stores, items, transfers)).toEqual({ name: "home" });
  expect(transfers.calls).toEqual([]);
});
```

(`fakeTransfers` の `locate` は、`missing` に引数の ID が入っていれば `NotFound` にする。Task 9 の偽物はこの動きを持つ。)

screen のテストに足す(`createScreen` の依存に `transfers`・`toasts` を足し、`openItem` の呼び出しを記録する)。

```ts
it("opens the found item with its moved-from note after replacing the address with the project tree", async () => {
  transfers.located = { kind: "found", storeId: "S1", projectId: "S1-P1", itemId: "S1-I9", movedFrom };
  navigation.replace({ name: "item", store: "work", item: "HH-5" });
  await flush();
  expect(opened).toEqual([["S1", "S1-I9", movedFrom]]);
  expect(formatRoute(get(navigation.route))).toBe("/s/work/p/HH-P1/tree");
});

it("toasts a gone message once, as a plain toast, and replaces the address with home", async () => {
  transfers.located = { kind: "gone", message: "HH-5 は 11/13 09:05 に TM-4 へ移動しましたが、削除されています。設定の「削除済み」から復元できます" };
  navigation.replace({ name: "item", store: "work", item: "HH-5" });
  await flush();
  await catalog.load();
  await flush();
  expect(toastTexts()).toEqual(["HH-5 は 11/13 09:05 に TM-4 へ移動しましたが、削除されています。設定の「削除済み」から復元できます"]);
  expect(formatRoute(get(navigation.route))).toBe("/");
});
```

Run: `pnpm -C web --filter @hedgehug/core test -- router/resolve screen`
Expected: FAIL

- [ ] **Step 6: アドレスの解決を実装する**

`resolve.ts`: `Resolved` に `movedFrom?` と `gone?` を足し、`resolveRoute` の第 4 引数に `transfers`、`findItem` はいまの ID で見つからなかった(最後の `return home` の手前で、`d?.kind === "item"` のとき)だけ `transfers.locate(store, id)` を呼ぶ。冒頭のコメントの「旧い接頭辞(B1)と転送の記録(A5)はたどらない。」を、いまの ID で見つからないときだけ `locate` に渡すことの説明に直す。`screen.ts`: `resolveRoute` に `deps.transfers` を渡し、`r.name === "home" && r.gone` のときは `deps.toasts.show(r.gone)` のあと `deps.navigation.replace({ name: "home" })` して返す。アイテムの入口は `deps.openItem(r.storeId, r.itemId, r.movedFrom)` にする(`navigation.replace` のあとに呼ぶ今までの順を変えない)。

Run: `pnpm -C web --filter @hedgehug/core test`
Expected: PASS

- [ ] **Step 7: 組み立てて、パネルに出す**

`wire.ts`: `createMovedNotice` を import し、`detail` の次に `export const movedNotice = createMovedNotice({ detail, now: () => new Date() });` を足す。`createScreen` の呼び出しに `transfers: transfersPort, toasts,` を足し、`openItem` を次にする。

```ts
openItem: (storeId, itemId, movedFrom) => {
  if (movedFrom) movedNotice.show(storeId, itemId, movedFrom);
  void detail.open(storeId, itemId);
},
```

`DetailPanel.svelte`: `import { detail, movedNotice } from "../../wire";` にし、`<script>` に `const moved = movedNotice.text;` を足し、`{#if v.readOnly}` の帯の次(種類の行の前)に `{#if $moved}<p class="moved">{$moved}</p>{/if}` を足す。`.moved { margin: 0; font-size: 12.5px; color: var(--muted); }` を `<style>` に足す。

Run:

```bash
pnpm -C web fix && pnpm -C web check && pnpm -C web test && pnpm -C web size && pnpm -C web deps
wc -l web/app/src/wire.ts web/app/src/screens/item/DetailPanel.svelte web/packages/core/src/index.ts
```

Expected: エラーなし、すべて通る。`wire.ts` が 300 を超えるときは、`movedNotice` を `wireBase.ts` に移さず、`wireActions.ts` の `createScreen` の組み立てごとは動かせないので、`wire.ts` の `itemsPort` 周りの `export` を `wireBase.ts` の再公開に寄せて行を減らす(A4b の Task 9 と同じ分け方)。

- [ ] **Step 8: コミットする**

```bash
git add web
git commit -m "$(cat <<'EOF'
feat: 画面のアドレスの旧 ID を転送記録でたどり、パネルの上部に移した日時を出す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 12: 通しのテスト(e2e)と PR

> **この Task への上書き**(「本文より優先する直し」): 仕様にない文言を仕様 02 に足し、PR の本文に一覧で出す。


A5a の全部を Playwright で 3 つの幅(スマホ・タブレット・デスクトップ)の通しで確かめ、`make check` を通し、PR を 1 本出す。e2e のサーバーは worker ごと(`web/e2e/fixtures.ts`)で、同じ worker のほかのテストと Store を共有する。サンプルを書き換えると壊れるので、`createWorkspace` で自分のワークスペースを作って使う。移動先に使う `e2e` の Store のワークスペース「Mobile」(`MB`、プロジェクト `MB-P1`、Epic `MB-1`)と `team` の Store(ワークスペース「Team」、`TM`)は、A5a-20 のとおりサンプルにある(`cmd/hedgehug/wire_e2e.go`)。サンプルへの書き込みは、移動先に増える分だけで、ほかのテストは Mobile・Team を使わない。

**増やす数: spec 1 ファイル・テスト 2 件(3 幅で 6 回)。** 1 件目は移動の通し(`test.step` で分ける。ワークスペースの作成は 1 回)、2 件目は閲覧のみの確認(サンプルを読むだけ)。同じテストを何度も流さない。

| テスト | 確かめること |
|---|---|
| `transfer.spec.ts` の 1 件目 | 同じ Store の別のワークスペースへ(Task)・別の Store へ(Epic と下の Task 1 件)・旧 ID のアドレスから新しいアイテムが開き上部の 1 行が出ること・普通に開くと 1 行が出ないこと・移したものを削除したあとの旧 ID のトースト |
| 同 2 件目 | 閲覧のみの Store のパネルのメニューで「別のワークスペースへ移動…」が無効で「閲覧のみの Store です」が添うこと |

**Files:**
- Create: `web/e2e/transfer.spec.ts`
- Modify: `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md`(確定した文言を足す。Step 4)
- Modify: `docs/superpowers/plans/2026-11-14-hedgehug-track-a5a.md`(末尾に「実装で計画から変えたこと」。Step 6)

**Interfaces:**
- Consumes: ここまでの全部。`web/e2e/helpers.ts` の `createWorkspace`・`prefixOf`・`projectWithEpic`・`taskUnderEpic`・`tokenUrl`・`unique`、`fixtures.ts` の `test`・`expect`
- Produces: `transfer.spec.ts`

**前提にするロケーター(アクセシブルな名前)**

| 画面 | 名前 | 部品 |
|---|---|---|
| 詳細パネル | region「詳細パネル」、ヘッダーの button「メニュー」(`exact`)、ID は `code`、上部の 1 行は `<p>`(role なし) | 既存と Task 11 |
| メニュー | menuitem「別のワークスペースへ移動…」「削除…」 | Task 10 |
| ダイアログ | dialog「別のワークスペースへ移動」、select「移動先のワークスペース」「プロジェクト」「親の Epic」、見出し「移動先で変わること」、button「移動」(`exact`) | Task 10 |
| 削除 | dialog「Task を削除」、button「削除」(`exact`) | A4b |
| トースト | role `status` | 既存 |

- [ ] **Step 1: 部品と突き合わせる**

Run: `grep -rn "aria-label\|label=" web/app/src/screens/item/TransferDialog.svelte web/app/src/screens/item/DetailHeader.svelte web/app/src/screens/item/ItemMenu.svelte && grep -n "Mobile\|team\|Team\|\"TM\"\|\"MB\"" cmd/hedgehug/wire_e2e.go cmd/hedgehug/*_e2e.go`
Expected: 上の表の名前が見つかる。サンプルの Mobile の Epic が `MB-1`、Team の Store の名前が `team` であること。違うときは、次のテストの `MB-1`・`MB-2`・`team` を実際の値に直す(部品とサンプルは直さない)。

- [ ] **Step 2: 通しのテストを書く**

`web/e2e/transfer.spec.ts`(新規)。

```ts
import type { Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { createWorkspace, prefixOf, projectWithEpic, taskUnderEpic, tokenUrl, unique } from "./helpers";

// 別のワークスペース・別の Store への移動と、旧 ID のアドレスの通しのテスト(A5a)。自分のワークスペースに
// Epic(-1)と Task A(-2)・B(-3)を作り、A を同じ Store の「Mobile」へ、Epic を別の Store「team」へ移す。

const panelOf = (page: Page) => page.getByRole("region", { name: "詳細パネル" });

async function openTransfer(page: Page, address: string) {
  await page.goto(tokenUrl(address));
  const panel = panelOf(page);
  await panel.getByRole("button", { name: "メニュー", exact: true }).click();
  await page.getByRole("menuitem", { name: /^別のワークスペースへ移動…/ }).click();
  const dialog = page.getByRole("dialog", { name: "別のワークスペースへ移動" });
  await expect(dialog).toBeVisible();
  return dialog;
}

test("moves items to another workspace and another Store, and follows the old IDs", async ({ page }, info) => {
  const t = (s: string) => unique(info, s);
  const prefix = prefixOf(info);
  await createWorkspace(page, info, t("移動元"), prefix);
  await projectWithEpic(page, info, prefix);
  await taskUnderEpic(page, info, `${prefix}-1`, "A");
  await taskUnderEpic(page, info, `${prefix}-1`, "B");

  await test.step("moves Task A to the workspace Mobile in the same Store", async () => {
    const dialog = await openTransfer(page, `/s/e2e/i/${prefix}-2`);
    await dialog.getByRole("combobox", { name: "移動先のワークスペース" }).selectOption({ label: "e2e / Mobile" });
    await expect(dialog.getByText(`${prefix}-2 ${t("A")} を移動します。`)).toBeVisible();
    await expect(dialog.getByRole("heading", { name: "移動先で変わること" })).toBeVisible();
    await expect(dialog.getByText("ID は MB-2 からの番号に振り直されます")).toBeVisible();
    await expect(dialog.getByText("スプリントとリリースは外されます")).toBeVisible();
    await expect(dialog.getByRole("combobox", { name: "親の Epic" })).toBeVisible();
    await dialog.getByRole("button", { name: "移動", exact: true }).click();
    await expect(page.getByRole("status").getByText(`${prefix}-2 は MB-2 になりました`)).toBeVisible();
    // 開いていたパネルが、新しい ID のアイテムに切り替わる。
    await expect(panelOf(page).locator("code", { hasText: "MB-2" })).toBeVisible();
  });

  await test.step("moves the Epic and the Task under it to the Store team", async () => {
    const dialog = await openTransfer(page, `/s/e2e/i/${prefix}-1`);
    await dialog.getByRole("combobox", { name: "移動先のワークスペース" }).selectOption({ label: "team / Team" });
    await expect(dialog.getByText(`${prefix}-1 ${t("通知")} と、その下の 1 件を移動します。`)).toBeVisible();
    // Epic は親を選ばない。別の Store なので、関連の解除とメンバーの行が出る。
    await expect(dialog.getByRole("combobox", { name: "親の Epic" })).toHaveCount(0);
    await expect(dialog.getByText("移さないアイテムとの関連は解除されます。移すもの同士の関連は残ります")).toBeVisible();
    await expect(dialog.getByText("移動先のメンバーにいない人は、移動先のメンバーに加わります")).toBeVisible();
    await dialog.getByRole("button", { name: "移動", exact: true }).click();
    await expect(page.getByRole("status").getByText(new RegExp(`^${prefix}-1 は TM-\\d+ になりました$`))).toBeVisible();
  });

  await test.step("opens the new item from the old ID, with the moved-from line only then", async () => {
    await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
    await expect(panelOf(page).getByText(new RegExp(`^${prefix}-2 は .+ に MB-2 へ移動しました$`))).toBeVisible();
    await expect(panelOf(page).locator("code", { hasText: "MB-2" })).toBeVisible();
    await page.goto(tokenUrl(`/s/e2e/i/${prefix}-1`));
    await expect(panelOf(page).getByText(new RegExp(`^${prefix}-1 は .+ に TM-\\d+ へ移動しました$`))).toBeVisible();
    // いまの ID で開いたときは出ない。
    await page.goto(tokenUrl("/s/e2e/i/MB-2"));
    await expect(panelOf(page).locator("code", { hasText: "MB-2" })).toBeVisible();
    await expect(panelOf(page).getByText(/へ移動しました$/)).toHaveCount(0);
  });

  await test.step("says the moved Task is deleted when its old ID is opened after the deletion", async () => {
    await panelOf(page).getByRole("button", { name: "メニュー", exact: true }).click();
    await page.getByRole("menuitem", { name: /^削除…/ }).click();
    await page.getByRole("dialog", { name: "Task を削除" }).getByRole("button", { name: "削除", exact: true }).click();
    await expect(panelOf(page)).toBeHidden();
    await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
    await expect(page.getByRole("status").getByText(
      new RegExp(`^${prefix}-2 は .+ に MB-2 へ移動しましたが、削除されています。設定の「削除済み」から復元できます$`))).toBeVisible();
    await expect(panelOf(page)).toBeHidden();
  });
});

test("disables the transfer item in a read-only Store with the reason", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/p/HH-P1/tree"));
  await page.getByRole("list", { name: "ツリー" }).getByRole("link", { name: "認証" }).click();
  const panel = panelOf(page);
  await panel.getByRole("button", { name: "メニュー", exact: true }).click();
  const entry = page.getByRole("menuitem", { name: /^別のワークスペースへ移動…/ });
  await expect(entry).toBeDisabled();
  await expect(entry).toContainText("閲覧のみの Store です");
});
```

(2 件目の Epic「認証」は `tree-readonly.spec.ts` と同じサンプルの行。)

- [ ] **Step 3: e2e を 1 回流す**

Run:

```bash
make e2e-bin
pnpm -C web e2e transfer.spec.ts
```

Expected: 6 件(2 テスト × 3 幅)が通る。落ちたときは、メッセージと `trace` から原因を直す(部品の名前が表と違えばテストの側を直す)。同じテストを何度も流さない。

- [ ] **Step 4: 確定した文言を仕様に足す**

`02-items-projects.md` の「移す先の選び方」に、この計画で決めた文言を足す。差分で見せて承認を取る(振る舞いではなく文言)。足すもの: ダイアログのタイトル「別のワークスペースへ移動」、項目の名前「移動先のワークスペース」「プロジェクト」「親の Epic」(ワークスペースの選択肢は「{Store 名} / {ワークスペース名}」)、「移動先で変わること」の各行の文(Task 9 の `changeRows` のとおり)、ステータスの置き換えのセレクトの名前「{ステータス名} の置き換え先」、移動のダイアログが開いたままになる失敗の扱い(送信の失敗のトーストのあとも、移動先を選び直して送り直せる)、Step のメニューに「別のワークスペースへ移動…」を出さないこと。

- [ ] **Step 5: 全体の検査を流す**

Run: `make check`
Expected: 緑(Go・lint・web の test・svelte-check・依存の検査・サイズ・ビルド)。e2e は Step 3 で流したので、ここで全件を流し直さない。ただし `make check` が e2e を含むなら、それが 1 回目の全件になる。

- [ ] **Step 6: 計画に実装で変えたことを足す**

`docs/superpowers/plans/2026-11-14-hedgehug-track-a5a.md` の末尾に「## 実装で計画から変えたこと」を足し、Task 9〜12 の実装で本文から変えたものを書く(変えたものがなければ「なし」)。進み具合のメモは書かない。

- [ ] **Step 7: コミットして、PR を出す**

```bash
git add web docs
git commit -m "$(cat <<'EOF'
test: 別のワークスペース・別の Store への移動と旧 ID のアドレスの通しのテストを足す

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
git push -u origin feat/track-a5a
```

PR の本文の書き方は `gh pr view 177 --json body -q .body` に合わせる(見出しは「概要」「範囲」「既存の振る舞いを変えるところ」「仕様に足した文言」「実装で見つけて直したもの」「回したもの」「develop の取り込み」「確かめたこと」。中身はこのトラックの実際に合わせ、ない見出しは書かない)。末尾の行は `🤖 Generated with [Claude Code](https://claude.com/claude-code)`。

```bash
gh pr create --base develop --head feat/track-a5a \
  --title "feat: トラック A5a(アイテムの別のワークスペース・別の Store への移動)" \
  --body-file <本文を書いたファイル>
```

Expected: PR の URL が返る。PR はトラックで 1 本(Task 1〜12 すべてを含む)。force push は拒否されるので、載せ直すときは別名のブランチで出す。「確かめたこと」には、`make check` の件数と、e2e を流した回数(1 回)・件数を書く。

---

---

## 付録: 書き手の骨組みへの指摘(原文)

### 骨組みへの指摘(Task 1〜4)

1. **`transfer.Remap` に移動先のプロジェクトを渡す引数が要る。** 骨組みの形 `Remap(it, from, to, people, statusChoice)` では、`UsableStatuses(to, p project.ID)` を呼べない(A5a-8 は `p` を取り、B5 が差し替える)。本文は `Remap(it, from, to, toProject project.ID, people, statusChoice)`。「タスク間の名前」と A5a-7 を直す。あわせて、`people == nil` は同じ Store、非 nil は別の Store(別の引数を足さない)とした。追加プロパティと人の置き換えは `remap_fields.go` に分けた(`remap.go` が 300 行を超えないため)。「ファイルの地図」の `domain/transfer` に `remap_fields.go` を足す。
2. **`transfer.People` の形。** 骨組みは `People(items, logs, members) []member.Member` と `PeopleMap` の型だけ。これでは、(a) 追加プロパティのうちユーザー型のものが分からない(`workspace.Settings` が要る)、(b) 移動先にいる人の綴りへの置き換え(`PeopleMap`)を作る人がいない、(c) 移動元と移動先のメンバーの両方が要る。本文は `People(from workspace.Settings, items, logs [][]change.Record, fromMembers, toMembers []member.Member) ([]member.Member, PeopleMap)`。Task 6 は、1 つ目を「`member.Join(ログイン名)` と、表示名がログイン名と違うときの `member.Rename(ログイン名, ログイン名, 表示名)`」で書き、2 つ目を `Remap` に渡す。
3. **`domain/transfer` の足したもの。** `Set.Contains(id)`(Task 6 が、関連の持つ側が移すものの中か外かで、移動先の `remove` と移動元の外の相手への `remove` を分けるため)、`ErrNotFound`・`ErrStep`(`Collect` の断り。Task 5 が `port.ErrNotFound` と 02 の文言に直す)、`EntityItem` などの定数。「タスク間の名前」に足す。
4. **`port.SaveError`(新しい型)が要る。** `SyncSave` は、保存の失敗(`fn` のエラー・`op.Check`・コミット)と通信の失敗を、呼び出し側が分けられなければ、`fn` の失敗が「remote に接続できません」になってしまう(`remoteReason` の既定)。本文は `port/replicator.go` に `SaveError` を足し、`Deliver` は `SaveError` を理由にせず error で返す。A5a-5 の「error は理由で表せない失敗だけ」と合う。「タスク間の名前」に足す。
5. **`syncing.Service` に `Stores port.Stores` が要る。** 骨組みの A5a-5 は「remote がなければ `Stores.Write` だけ」と書くが、`Service` には `Stores` の項目がない。本文は項目を足し、配線(Task 8)では `NotifyWrites` で包む前の `Stores` を渡す(包むと、`SyncSave` が送ったあとに自動の同期がもう一度走る)。`Deliver` は閲覧のみの Store を `ErrInvalidOperation` で断り(`Targets` の `ReadOnly`)、`run` と同じくミラーへ送り、`EventSync` を知らせ、`AfterSuccess` は呼ばない。`app/transfers` が閲覧のみを先に断ること(A5a-13)とは別に、口の側でも断る。
6. **A5a-5 の「送れなければ保存を捨てる」の細部と、`fn` を何度も呼ぶこと。** 送信が重なったとき、骨組みの文だけでは、保存を残したまま取り込み直すのか分からない。本文は、毎回保存を捨てて取り込み直し、`fn` を呼び直す。理由は、移動先の「次の番号」が取り込み直しで変わるため。Task 6 の `fn` は、番号・メンバー・`transfers.Number` の結果を、`fn` の中の `Tx` から読んで決める(外で決めた値を使い回さない)。この前提を A5a-5 と Task 6 の冒頭に書く。保存の途中でプロセスが落ちた場合の、移動先だけに残る状態は扱わない(A5a-6 の「(4) の失敗で戻す」は、落ちたときには働かない)。
7. **置き場所。** `SyncSave` は `gitstore/write.go` に置く(`replicator.go` は 264 行で、足すと 300 を超える)。`apptest/stores.go` は足すと約 310 になるので、`holders`・`enter`・`goroutineID`(約 37 行)を新しい下位のパッケージ `internal/app/apptest/reentry` に出す。転送・取り込み・削除の照合は `stores_contract_isolation.go`(128 行)に足す(名前は「分離」だが、行に余裕があるのはここだけ)。「ファイルの地図」の `apptest/` の行に `reentry/` を足す。
8. **偽物の足したもの。** `apptest.Replicator` に `Stores`・`SendErrs`、`ReplicatorCall.Save`、`apptest.Stores.AddPurged`(`purged.jsonl` に書く口がなく、テストで置けないため。書くのは G)。Task 7 の `Locate` のテストが使う。
9. **A5a-1 の JSON の細部。** `entity` は常に書く(ない行は `item` として読む)、`to_store_name`・`to_remote` は空なら書かない。同期の `transferTargets` は `target`・`to_store` だけを読むので、そのまま動く。
10. **`Import` の決め。** `Remove` したあとの同じ Tx での `Import` は `ErrInvalidOperation`(A5a-6 の「戻し」は別の `Deliver` で書くので、同じ Tx では起きない)。`Import` は行の中身(作成の行があるか)を調べない。A5a-4 の「`ErrInvalidOperation`(フォルダがあれば)」に、「`id` が使えない・`log` が空」を足す。
11. **Task の待つもの。** Task 4 は Task 3 の `writeLocked`(gitstore)・`write`(偽物)を使い、同じファイル(`write.go`・`stores.go`・`sync.go` の隣)を触るので、骨組みどおり 3 のあとに直列で行う。Task 2 は Task 1 の `PeopleMap` だけを使うので、Task 3 と並べてよい。

### 骨組みへの指摘(Task 5〜8)

本文は骨組みの形に従って書いた。次の点は、骨組みのままでは実装担当が決められない、または違う形の方がよいと分かったもの。

1. **`Service` の欄**(骨組みの「タスク間の名前」): `Me func(ctx) (string, error)` は、既存の機能の `Identity port.Identity`(Store ごとの `Me(ctx, store)`)に合わせた。`IDs` は使い道がない(ID・操作・時刻は `Tx` が付ける)ので外し、移動先の remote の有無・URL を知るための `Targets port.SyncTargets` を足した(転送記録の `to_remote` と、予告の remote の注意に要る)。`cmd/hedgehug` は `noSyncTargets{}`(本物)・`syncSvc.Targets`(e2e)を渡す。
2. **`transfer.People` の引数**: `People(items, logs, members)` では、(a) ユーザー型のプロパティの値の人を見つけるための移動元のワークスペースの設定、(b) 足す人の表示名を移動元のメンバーから取ること、(c) 移動先にいる人を除くための移動先のメンバー、が足りない。Task 6 は `People(items []item.Item, logs map[item.ID][]change.Record, from workspace.Settings, source, dest []member.Member) []member.Member` として呼ぶ(Task 1 の書き手に伝える)。`PeopleMap` は、`People` の結果と移動先のメンバーから Task 6 が作る(`People` が `PeopleMap` を返す必要はない)。
3. **`transfer.Remap` と予告の重なり**: (a) `UsableStatuses(to, p)` を使うには、`Remap` に移動先のプロジェクト `p` が要る。いまは「ワークスペースのステータスすべて」で `p` を使わないので骨組みの形で書いたが、B5 が `UsableStatuses` を差し替えるときに引数が足りなくなる。`Remap` に `toProject project.ID` を足す方がよい。(b) 予告の「ステータスの置き換え」の行は `Remap` と同じ判定(名前が `UsableStatuses` のどれとも違う)で作るので、Task 5 の `preview.go` と `Remap` に同じ規則が 2 か所できる。`transfer` に未対応のステータスを返す関数を 1 つ置いて両方が使う形にする案もある(Task 2 の書き手の判断)。(c) `people` が `nil` のときは「同じ Store としてそのまま残す」と決めて Task 5 のテストが期待している(Task 2 のテストにも入れてほしい)。
4. **偽物の `purged.jsonl`**: `Reader.Purged()` を読むだけなので、Task 7 の「完全に削除されました」のテストが記録を置く入口がない。Task 3 が `(*apptest.Stores).AddPurged(id port.StoreID, p port.Purged)` を偽物に足す(G が本物の書き込みを足すまでのテスト用)。
5. **`syncing.Service` の保存の口**: `Deliver` は remote がなければ `Stores.Write` を呼ぶので、`syncing.Service` に保存の口を持たせる必要がある。Task 8 は、そのフィールド名を `Stores port.Stores` と仮定している(Task 4 の書き手と揃える)。本物の組み立てでは `NotifyWrites` で包んだ Stores を渡す。
6. **A5a-13 のワークスペースの非表示**: 「サイドバーで非表示」は Store 単位の設定で、`StoreList` が非表示の Store をすでに除く。ワークスペース単位の非表示はないので、「Task 4 が確かめる」は不要。ワークスペースの並びは `workspace.RankOf(ログ)` の昇順(同じなら内部 ID の順)に決めた。
7. **A5a-17 の履歴の元**: 同じ Store の移動の履歴は、「転送記録の `target`」だけでは移動元のワークスペース名・プロジェクト名が分からない(`Record` に持たない)。場所の `set` の行(`from` に移動元の内部 ID がある)から作り、旧 ID だけを転送記録(同じ `Op`)から取る形にした。そのため `history.Context` に `Origin`(移動元の設定・名前・Store 名・記録)を足し、`apphistory.Service` に `StoreList` を足している(`routes.go` で渡す)。A5a-17 の「`moved_in` の行(別の Store)と転送記録(同じ Store)から作る」の言い換え。
8. **A5a-15 の 4 つ目の文**: 「いまの場所が記録の移動先と違う」は、見つかれば開くので、見つからないときの文にはなりえない。「同じアイテムを指す、より新しい記録の移動先の Store が表示中でない(B から C へ移した、その C が見えない)」ときに、この文を出す形に当てはめた(`rec.ToStore` が見えないときは 3 つ目の文)。
9. **A5a-11 のワークスペース名**: 別の Store のとき「{Store 名} の {ワークスペース名}」を移動元・移動先の両方に付けた(例「HH-2 を main の Web から team の Team へ移動」)。片方だけの読み方もできるので、違えば直す。
10. **戻し(A5a-6)**: 移動先に足したメンバーとワークスペースの `number` の行は、フォルダの `Remove` だけの戻しでは残る(`number` は再利用しないので害はない。メンバーは、移動元の保存が失敗した場合だけ、使われないまま残る)。メンバーを外す操作がないため。
11. **A5a-18 の「新しい内部 ID」**: 別の Store へ移しても、`Import` は内部 ID を変えない(同じフォルダ名で写す)ので、`Moved.Item` は内部 ID のまま。画面が切り替えるのは Store とワークスペースだけ。移動先に同じ内部 ID があれば `Import` が `ErrInvalidOperation` で断る(e2e のサンプルは、そのために `SampleMobile`・`SampleTeam` の内部 ID を `Sample` と分けた)。
12. **`MovedFrom.Display`**: 骨組みどおり「最初に一致した行の新 ID」にした。A → B → C の旧 ID(HH-2)で C のアイテムを開くと、パネルの 1 行は「HH-2 は {日時} に TM-2 へ移動しました」(B の ID)と読める。いまの ID(C)にする方が自然かもしれないので、Task 11 で画面を作るときに決める。
13. **Step・閲覧のみの移動元**: Step の移動と、選べない移動先は `port.ErrNotFound`、移動元が閲覧のみのときは `refusal.New("閲覧のみの Store です")` にした(骨組みには書かれていない)。

### 骨組みへの指摘(Task 9〜12)

1. **A5a-14・A5a-19 にキー名がなく、「移動先に remote があるか」が足りない。** 本文は予告を `{below, deleted, nextDisplay, crossStore, remoteNotice, destinationHasRemote, replacements: [{status, name, count, choices: [{id, name}], default}]}` とし、候補を `list-transfer-targets` の表のとおり、`transfer-item` の応答を `{toStore, itemId, display, emptied}`、`locate-item` を `{kind: "found"|"gone", storeId, projectId, itemId, movedFrom?: {old, at(ISO 8601), display}, message}` で書いた。Task 8 がこのキーに合わせる(Task 9 の Step 1 が `generated.ts` で確かめる)。`destinationHasRemote` は、A5a-12 のオフラインの判断(移動先に remote があり、ブラウザがオフラインなら送らずに断る)に画面が要る。A5a-14 の「remote の注意を出すか」(移動先に remote がなく、移動元にある)とは別の値。
2. **`gone` の `message` をサーバーが作ると、日時の書き方(01)が Go と TS の二重になる。** 本文は A5a-19 に従い、画面は `message` をそのままトーストに出す。直すなら、`gone` を `{kind: "gone", reason, old, at, newDisplay, storeName?, currentDisplay?}` で返し、画面が `formatDateTime` で 4 つの文を組む形が 1 か所で済む。旧 ID から開いたときの 1 行(A5a-16)は、すでに画面が `formatDateTime` で作る。
3. **送信の失敗の文(A5a-12)は、サーバーが 422 の `detail` に全文(「{Store 名} に送信できないため移動できません: {理由}」)を入れる前提。** 画面は `Refused` の文をそのまま出す。Task 8 が満たすこと。
4. **Step の行・パネルのメニューに「別のワークスペースへ移動…」を出さない。** 仕様 02 は「Step は単独では移せない」とあるが、Step で無効にする理由の文言がなく、メニューの無効の理由は 4 種類しか定めていない。新しい文言を作らず、項目を出さないことにした。無効にして理由を足す形がよければ仕様に文言を足してから直す。
5. **仕様に文の指定がなく、本文で決めた文言**(Task 12 の Step 4 で仕様 02 に足し、差分で承認を取る): ダイアログのタイトル「別のワークスペースへ移動」、セレクトの名前「移動先のワークスペース」「プロジェクト」「親の Epic」「{ステータス名} の置き換え先」、ワークスペースの選択肢「{Store 名} / {ワークスペース名}」、「移動先で変わること」の 7 行(「ID は {ID} からの番号に振り直されます」「スプリントとリリースは外されます」「優先度・関連システムなどは、…」「移さないアイテムとの関連は解除されます。…」「移動先のメンバーにいない人は、…」「移動先に remote がないため、…」(これだけ仕様の文)「移動元に転送の記録(移した日時を含む)を残し、…」)。
6. **移動の送信が失敗したら、ダイアログを開いたままにする**(削除・移動(A4b)は閉じる)。移動先を選び直して送り直せるため。`NotFound` だけ閉じる。
7. **画面のアドレスの旧い接頭辞でのワークスペース・プロジェクトのアドレス**(03 の 113・115 行、「ワークスペースの旧い画面のアドレスを開くと「このワークスペースは {日時} に {Store 名} へ移動しました」」)は、骨組みのどの Task にもない(`locate` はアイテムの入口だけ)。ワークスペース・プロジェクトの移動は A5b なので、A5b の持ち場に「旧い接頭辞と転送記録でのワークスペース・プロジェクトのアドレス」を足す必要がある。
8. **ファイルの地図に足りないもの:** `web/packages/core/src/features/items/detail.ts`(A5a-18 の「ワークスペースが変わっていたら閉じる」)、`web/packages/core/src/features/screen/screen.ts`(`transfers`・`toasts` の依存と `openItem` の第 3 引数)、`features/dates/date.ts`(`formatDateTime`)、`features/transfers/`(`dialogText.ts`・`afterMoved.ts`・`movedNotice.ts`)、`web/app/src/wire.ts`(`transfersPort`・`movedNotice`)。
9. **e2e のサンプルの前提:** Mobile の Epic が `MB-1`(次の番号が `MB-2`)、Team の Store の名前が `team`。Task 8 のサンプルとずれたら、Task 12 の Step 1 で本文の値を直す。remote ありの経路と「移動先に remote がないため…」の行、ステータスの置き換えの行は、e2e ではなく Go のテストと Task 9 のモデルのテストで確かめる。
