# フェーズ 4f: フェーズ3の画面（概要）

> 4f は3回に分ける（2026-11-04 に利用者が決定）。この文書は3回に共通の決定事項・細部・回の分け方。Task の本文は各回の plan に書く: `2026-11-04-phase4f1-go.md`（Go の不足）、`2026-11-04-phase4f2-settings.md`（アカウント・Store の設定と Onboarding）、`2026-11-04-phase4f3-stores.md`（サイドバー・マイタスク・閲覧のみの表示・Store 間の移動と E2E）。各回は前の回のマージのあとに main から切る。

**Goal:** フェーズ3の機能（複数 Store、閲覧のみの Store、アカウントと認証、Store の発見・マウント・追加・解除・既定、Store 間の移動、接頭辞が重なる Store の Store 名付きの表示、コメントの本人確認）を GUI でもできるようにする（spec の 4f の完了条件）。あわせて、4d で 4f に回した Store の色を選ぶ画面と保存場所を作る。

**Architecture:** 4e までの `web/core`・`web/app` の上に画面を足す。足りない API・usecase・CLI は 4f-1 で先に Go に入れる（4e-1 と同じ形）。usecase（旧 ops。`internal/app/usecase`）の関数（`AuthLogin`・`AuthAdd`・`AuthStatus`・`AuthLogout`・`StoreDiscover`・`StoreMount`・`StoreAdd`・`StoreRemove`・`StoreDefault`・`PrepareMoveStore`・`MoveStore`）はフェーズ3で作り済みで、4f-1 は API に出すことと、GUI に要る形の足し分だけを行う。

**Tech Stack:** 4e と同じ。新しい依存は足さない。

**Spec:** `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`（「画面と範囲」の「後のフェーズ」の欄の（3）、「4f の細部」）と `docs/superpowers/specs/2026-11-01-phase3-stores-accounts-design.md`。画面は Hedgehug GUI デザインのキャンバス（https://claude.ai/artifact/K14rwULxyyS2nYNbMKSr5L）の SettingsAccounts・SettingsStores・StoreMove・Onboarding・SettingsNav・Sidebar・MyTasks を正とし、下の「作らないもの」と細部で直したところだけ変える。

## Global Constraints

4e の概要 plan の Global Constraints をすべて引き継ぐ（日本語の文言、トークンの CSS 変数、外部に通信しない、キーボード、`svelte-check` 警告ゼロと `biome ci` 指摘ゼロ、spec と違う挙動は止めて報告、Go のテストは `-race`、Go のジョブは `web/dist` なしで通る、まだない画面への入口は出さない、キャンバスの CLI の表記は実際のコマンドに合わせる）。加えて:

- トークンはどの API の応答にも、ログにも、SSE にも出さない（貼り付けたトークンは要求の本文で受け取り、keychain に書くだけ）
- GUI が出す `hh` の行は実在するコマンドとフラグだけにする
- 構成見直し（`docs/superpowers/specs/2026-11-04-codebase-restructure-design.md`）の回 1 の後の配置で書く: usecase は `internal/app/usecase`、CLI は `internal/app/cli`、server は `internal/app/server`、config は `internal/app/storage/config`、fakegh は `internal/lib/fakegh`。構成見直しの回 2 以降は 4f-1 の後に、4f-1 で足したコードごと移す（2026-11-04 に利用者が決定）。回 4 の手戻りを減らすため、4f-1 で cli・server に足すコードは storage・remote・`lib/gitrepo`・`lib/github` を新しく直接 import しない（構成見直しの 4.2）。必要な読み書きは usecase の操作として足す
- 構成見直しの回 4 は `hh mv --store --json` を server と同じ usecase の型に揃える。4f-1 の `move-store` の応答（細部 8）は usecase の `MoveStoreResult` をそのまま返し、回 4 はそれに CLI を揃える


## 利用者の決定（2026-11-04、spec に反映済み）

1. 4f を3回に分ける: 4f-1 Go、4f-2 アカウント・Store の設定と Onboarding、4f-3 サイドバー・マイタスク・閲覧のみの表示・Store 間の移動と E2E
2. Store の色と表示は config.yaml の `stores[].color`・`stores[].hidden` に書く（このマシンだけ）。CLI に `hh store set <名前> [--color <色>] [--hidden=true|false]` を足す。`hidden` は GUI の表示だけに効き、CLI の対象は変えない
3. GUI のアカウントの追加は Device Flow も作る（サーバーがポーリングし、結果を SSE で知らせる）。トークンの貼り付け（keychain）、gh、環境変数、トークンなしのアカウント（`hh auth add`）も作る
4. キャンバスにあって CLI の機能がないものは作らない（下の「作らないもの」）
5. 設計の確認（2026-11-04）: 発見の「フォルダから探す」（`--from`）と、移動の親の指定（`--parent`）は GUI にも出す

## 作らないもの（決定。持ち越しではない）

- StoreMove の `purge --rewrite-history` の導線と、その確認ダイアログ（2026-11-02 に機能ごと廃止）。履歴の警告の「消すには …」の文も出さない
- SettingsAccounts の「GitHub 連携で使うアカウント」（`repo_accounts` の編集）と GitHub App のインストールの警告（フェーズ5の機能）
- Onboarding の gh にログイン済みのアカウントの自動検出（CLI にない）
- SettingsStores の merge driver の状態と登録、remote の追加、フォルダを開く、形式の統計（フォーマット・タスク数・未圧縮オブジェクト）、メンバー数、「新しく見つかった Store」の自動の案内（どれも CLI にない）
- `hh use`（コンテキスト解決。カレントディレクトリが意味を持たない GUI は使わないと、フェーズ3の spec で決まっている）

## 回の分け方と中身

| 回 | 中身 | 完了条件 |
|---|---|---|
| 4f-1 Go | 細部 1〜16 の API・usecase・CLI の追加。`api/openapi.yaml` と `web/core/api/schema.d.ts` の再生成、`web/core/messages.ts` の文言 | Go のテスト（usecase・server・CLI のゴールデン）と OpenAPI の照合が通る |
| 4f-2 設定 | SettingsNav（17）、SettingsAccounts（18〜22）、SettingsStores（23〜28）、Onboarding（29〜31）、CreateStore のアカウント（32）、`stores.changed` の受け取り（33） | アカウントの追加（Device Flow を含む）・ログアウト、Store の発見とマウント・URL で追加・解除・既定・色・表示が GUI で動く（E2E） |
| 4f-3 Store | 閲覧のみの表示と編集の無効（34〜37）、非表示の Store（38〜40）、マイタスク（41〜42）、Store 名付きの ID（43）、コメントの本人確認（44）、StoreMove（45〜50）、E2E（51） | 閲覧のみの Store で書き込みの操作が出ず、非表示の Store が外れ、Store 間の移動が GUI で動く（E2E）。spec の 4f の完了条件を満たす |

## この plan で埋める spec にない細部

### 4f-1 Go の不足

1. **config.yaml の色と表示**: `config.StoreEntry` に `Color string`（`color`。`teal`・`orange`・`purple`・`sky`・`pink`・`olive`・`brown` のどれか。空は「並び順で割り当て」）と `Hidden bool`（`hidden`。省略は false）を足す。ほかの値は読み込みのエラー（既存の読み込みのエラーと同じ形、ファイルのパスと行）。書くときは知らないキーとコメントを残す（`Config.Update`）。灰色は閲覧のみ用なので選べない
2. **`hh store set <名前> [--color <色>] [--hidden=true|false]`**: 渡したものだけ書き換える。`--color ""` は空に戻す。どちらも渡さなければ使い方の誤り（終了コード2）。ない名前はエラー（登録済みの名前を並べる。`hh store default` と同じ）。今と同じ値なら書かない。出力は `Set org-a: color=teal hidden=false`、`--json` は `StoreView`。親設計の「コマンド一覧」に足す
3. **`StoreView` に足すもの**: `color`（config の値。空なら省く）、`hidden`（bool）、`read_only`（閲覧のみの理由の文。`SyncStatus.read_only` と同じ `acc.Why()`。書けるなら省く）。`default_store`（bool。config.yaml の `default_store` が指す Store だけ true。GUI の既定の印はこれを使い、`GET /api/stores` の `default` は使わない）。`hh store list --json` にも同じ項目が出る（ゴールデンを更新）。`hh store list` の表には列を足さない
4. **Store の書き込みの API**（どれも `Config.Update` の1回で、成功したら `stores.changed` を出す）:
   - `POST /api/stores/add`（本文 `{name, remotes, account?}`。`hh store add`。`path` は既定。応答は `StoreView`）
   - `DELETE /api/stores/{name}?keep=true|false`（`hh store remove [--keep]`。既定は `keep=false`＝ローカルのクローンも消す。未 push などで消せなければ 409 `store.unpushed` をそのまま返す。応答は `{name, path, deleted}`）
   - `PUT /api/stores/default`（本文 `{name}`。`hh store default <名前>`。応答は `{name}`）
   - `PATCH /api/stores/{name}`（本文 `{color?, hidden?}`。細部 2 と同じ。応答は `StoreView`）
5. **発見とマウントの API**: `GET /api/stores/discover?account=&from=`（`hh store discover`。応答は候補の配列 `{name, url, account, permission}`。`from` があってアカウントが決まらなければ 422 `choice.required` と、選べるアカウントの名前）と `POST /api/stores/mount`（本文は候補の配列。名前は画面で変えたもの。応答は `{mounted: [名前], failed: [{name, code, message}]}`。`MountFailure.Err` は problem と同じ `code` と英文に直す。1つ以上マウントしたら `stores.changed`）
6. **アカウントの API**:
   - `GET /api/accounts`（`hh auth status --json` と同じ配列。`ErrFindings` は誤りにせず、各要素の `state` で返す。`GET /user` を呼ぶので、画面を開いたときと操作のあとだけ呼ぶ）
   - `POST /api/accounts`（本文 `{name, login, host?, token?}`。`hh auth add`。`token` は `gh`・`env:<VAR>`・空のどれか。`keychain`・`device` は 400 で、細部 7・8 を案内する）
   - `PUT /api/accounts/{name}/token`（本文 `{token}`。`hh auth login <name> --with-token` と同じで、`GET /user` の login が合うのを確かめてから keychain に書く。名前が config になければ `hh auth login --with-token` と同じく、login からアカウントを足す（本文の `host` は省略で github.com）。応答は `AuthLoginResult`）
   - `DELETE /api/accounts/{name}/token`（`hh auth logout <name>`）
7. **Device Flow の API**: `POST /api/auth/device`（本文 `{name?, host?}`。`hh auth login [<name>] [--host]` と同じ判定。`ShowCode` で受けた `user_code`・`verification_uri`・`expires_in` を返すまで待ち、ポーリングは裏で続ける。応答は `{id, user_code, verification_uri, expires_at}`）。同時に走る Device Flow はサーバーに1つだけで、走っている間の2回目は 409 `auth.device_running`（problem の `args` に走っているフローの `{id, user_code, verification_uri, expires_at}` を載せる。画面は引き継ぐか、`DELETE` でやめる。細部 69）。`DELETE /api/auth/device/{id}` でやめる（裏のポーリングを止める）。終わったら SSE の `auth.device` を出す（`{id, state: "done"|"error", account?, code?, message?}`。`done` は `AuthLoginResult` の `account`。`error` は usecase の `Code` と英文）。client ID が空のときの誤りは開始の応答で返す（usecase のまま）
8. **Store 間の移動の API**: `POST /api/tasks/{id}/move-store/plan`（本文 `{to, workspace?, parent?, scoped?}`。`PrepareMoveStore`。応答は細部 9 の `MovePlanView`。scoped の選択が要るのに `scoped` がなければ、誤りにせず `scoped.required: true` で返す）と `POST /api/tasks/{id}/move-store`（本文は同じものに `confirmed: MovePlanView` を足す。`confirmed` は必須（ないと 400 `confirm.required`）。ロックの中で作り直した予定の `MovePlanView` と一致しなければ 409 `move.changed`。応答は `MoveStoreResult` の JSON（`hh mv --store --json` と同じ））。移動元か移動先が閲覧のみなら usecase のまま 409 `store.read_only`
9. **`MovePlanView`**（GUI に見せる構造化した予定。`MoveView` は CLI の文の一覧のまま残す）: `{from, to, workspace: {ulid, name, prefix}, parent?: {ulid, id, title}, tasks: [{ulid, id, title}], unlinked: [{task: {ulid, id}, field, target: {ulid, label}}], hidden: [アカウント], scoped?: {required, choice?, dropped: [文]}}`。`field` は `blocked_by`・プロパティ名・`sprint`・`releases`。`label` は `MovePlan.Labels` の値（ID・スプリント名・リリース名、なければ ULID）。usecase に `MovePlan.GUIView()` と、`MoveStoreInput.ConfirmedView *MovePlanView`（`Confirmed` と同じ照合を `GUIView()` の比較で行う）を足す
10. **SSE `stores.changed`**: 細部 4・5・7（アカウントを足して Store にアカウントを書いたとき）の書き込みのあとと、poll が Store の出入り（config の `stores[]` に現れた・消えた・パスが変わった）を見つけたときに出す（本文は空）。消えた Store は poll が持っている repo を閉じ、その Store の `sync.state` を出さなくなる。CLI で `hh store set` したときも poll が config の変化（色・表示・既定・アカウント）で出す
11. **poll の Store の解除**: `DELETE /api/stores/{name}` は、usecase の `StoreRemove` の前に poll のその Store の repo を手放す（`Poller.ReleaseFor(path, fn)`。ディレクトリを消すため。`StoreRemove` が終わるまで poll を止める。消せずに断られたときは poller が黙って開き直す。4f-1 の決定 1）
12. **トークンの扱い**: 細部 6・7 の応答とログにトークンを出さないことをテストで確かめる（応答の本文にトークンの文字列が含まれないこと）
13. **`web/core/messages.ts`**: `move.changed`・`auth.device_running`・`choice.required`（アカウントの選択）・`store.unpushed`・`store.mount_failed`・`account.*` など、4f-1 で API に出る Code の日本語の文言を足す（すでにあるものは確かめるだけ）
14. 1〜11 の OpenAPI を再生成し、`web/core/api/schema.d.ts` を作り直す（`pnpm gen`）。SSE の型（`web/core/connection.ts` の `Handlers`。`sse.ts` にイベント名の一覧はない）に `auth.device` を足す。`stores.changed` の受け取りは 4f-2（細部 33）。4f-1 の決定 12
15. 親設計の「コマンド一覧」に `hh store set` を足す（決定 2）
16. Device Flow の API のテストは fakegh（`internal/lib/fakegh` を httptest で、`device_interval` 0・`auto_approve`）で通す

### 4f-2 アカウント・Store の設定と Onboarding

17. **SettingsNav**: 「このマシン」に「アカウントと認証」（`/settings/accounts`）と「マウント中の Store」（`/settings/stores`）を、同期と保守の前に足す（キャンバスの順）。「Store ごとの設定」の選択欄は、閲覧のみの Store に「（閲覧のみ）」を添え、非表示の Store も並べる。サイドバーの歯車の行き先は `/settings/general` のまま
18. **SettingsAccounts の一覧**: `GET /api/accounts` の各アカウントをカードで出す: 呼び名、`login@host`、状態（`ok` は「有効」、`error` は文、`none` は「トークンなし」）、トークンの取得元（`gh`・`env:VAR`・`keychain`・`device`・なし）と、`device` の期限、使っている Store（`GET /api/stores` の `account` が一致するもの）。アバターは名前の頭文字
19. **アカウントの追加**: 「アカウントを追加」で、取得元を Device Flow・トークンを貼る・gh・環境変数・トークンなしから選ぶ。Device Flow は呼び名（任意。既定は login）とホスト（テキスト欄。既定 github.com。GHES は `hosts[]` に書いた host）を入れて開始し、コード（コピーのボタン付き）と `verification_uri` のリンク（新しいタブ）と期限までの残り時間を出し、SSE の `auth.device` で完了を出す。閉じたら `DELETE /api/auth/device/{id}`。トークンを貼るは呼び名・ホスト・トークン（`type=password`）。gh・環境変数は呼び名・login・ホスト（環境変数は変数名も）。トークンなしは呼び名・login・ホスト。足元に、それぞれの `hh auth login` / `hh auth add` の行を出す（トークンは行に入れない）
20. **アカウントの操作**: カードの「…」に、`keychain`・`device` なら「ログインし直す」（同じ取得元で 19 を開く）と「ログアウト」（確認ダイアログ。`DELETE /api/accounts/{name}/token`）。`gh`・`env` には「Hedgehug が持っているものはありません」と出し、操作は出さない
21. アカウントの削除は CLI にないので作らない（config.yaml を編集する案内も出さない）
22. 操作のあとは `GET /api/accounts` と `GET /api/stores` を取り直す
23. **SettingsStores の一覧**: `GET /api/stores` の各 Store を config の順にカードで出す: 色のボタン、名前、権限（「書き込み」または「閲覧のみ」と理由）、`default_store` が true なら「既定の Store」（`default` ではなく `default_store`）、`remotes[0]` と `account`、最後の同期（キャンバスのタスク数は出さない。Store ごとの件数を返す API がなく、非表示の Store の件数をキャッシュから数えられないため）、表示のチェック
24. **色**: 色のボタンで7色のパレットを開き、選ぶと `PATCH /api/stores/{name}` `{color}`。「自動」で空に戻す。閲覧のみの Store は灰色で出すが、選んだ色は保存できる（閲覧のみでなくなったら使う）。ボタンの名前は「<名前> の色を変える（このマシンだけ）」
25. **表示**: チェックを外すと `PATCH {hidden: true}`。すべての Store を外すことはできない（最後の1つは無効にして理由を出す）
26. **詳細**: 開くと remotes（2つ目以降に「ミラー」）、アカウント、ローカルのパス、操作（「既定にする」（既定でないとき）、「マウントを解除…」）を出す
27. **マウントの解除**: 確認ダイアログで「ローカルのクローンも消す」（既定でオン。`keep=false`）を選べる。409 `store.unpushed` は文言を出してダイアログに留まり、「クローンを残して解除」を案内する。最後の Store を外すと Onboarding に戻る
28. **見出しの操作**: 「Store を探す」（細部 30 の発見を、ダイアログで開く）、「URL で追加」（名前・URL（複数可）・アカウント（2つ以上のときだけ選択欄）のダイアログ。`POST /api/stores/add`）、「Store を作る」（CreateStore）
29. **Onboarding**: Store が0件のとき。3段（アカウント・Store を選ぶ・はじめる）。アカウントの段は、アカウントがあれば並べて「次へ」、なければ 19 の追加を出す（トークンなしでも次へ進める）。Store を選ぶの段は細部 30、はじめるの段はマウントした Store を並べて「始める」でホームへ。どの段でも「新しく Store を作る」（CreateStore）を出す
30. **発見**: トークンを持つアカウントがあれば `GET /api/stores/discover` を自動で呼び、候補をアカウント（owner）ごとに見出しを付けて並べる（キャンバスのとおり）。各候補は選択のチェック・repo・権限（write は「書き込み」、read は「閲覧のみ」）・呼び名の欄（提案された名前）。「フォルダから探す」でパスを入れて `from` で探す（アカウントが決まらなければ選択欄を出す）。0件は「見つかりませんでした」と、権限を管理者に依頼する案内。「マウントして始める」（設定ではマウント）で `POST /api/stores/mount`、失敗したものはその行に文言を出し、ほかは進む。足元は `hh store discover`
31. 発見の待ち時間は、読み込み中の状態（4d の States）で出す
32. **CreateStore のアカウント**: アカウントが2つ以上あれば選択欄を出し、`POST /api/stores` に `account` を送る（1つなら送らず usecase に任せる）。足元の `hh store init` に `--account` を足す
33. **`stores.changed`**: `StoreDirectory` が受けて Store とワークスペースを取り直す。設定の画面も取り直す。消えた Store の `conn.sync` の項目を捨てる

### 4f-3 サイドバー・マイタスク・閲覧のみ・移動

34. **閲覧のみの判定**: `StoreView.read_only` か `SyncStatus.read_only` があれば閲覧のみ（`StoreDirectory.readOnly(store)`）。理由の文は `read_only` の値を日本語に直す（`github_permission`・`not_writable`・`push_denied` の3つ。`messages.ts` の `store.read_only` と同じ言い方）
35. **編集の無効**: 閲覧のみの Store のタスクでは、TaskDetail（本文・プロパティ・チェックリスト・関連・コメントの投稿と編集・ウォッチ以外の書き込み・履歴の復元・削除・移動）、カンバンのドラッグ、テーブルのセル、ツリーの追加・字下げ・並べ替え・完了・ドラッグ、作成のワークスペースの選択（閲覧のみのワークスペースは無効にして理由）、ワークスペースの追加・設定・削除、プロジェクトの設定、スキーマの migrate、doctor の修復（その Store だけ）、purge、ビューの改名・削除を無効にする。ウォッチは自分のローカルの状態ではなく Store への書き込みなので無効にする
36. **閲覧のみの印**: StoreBadge の灰色に加え、ツールチップと読み上げで「<名前>（閲覧のみ）」。TaskDetail とプロジェクト・ワークスペースの画面の上に「この Store は閲覧のみです（理由）」の帯を出す（States の帯と同じ部品）
37. 無効にした操作は、ボタンを消さずに `disabled` にして理由をツールチップで出す（キーボードの操作は理由をトーストで出す）
38. **非表示の Store**: `StoreView.hidden` の Store は、サイドバーのワークスペース、保存済みビューの一覧（その Store に保存されたもの）、クエリとビューの結果（ツリー・カンバン・テーブル）、マイタスク、パレットの検索と「最近開いたもの」、作成のワークスペースの選択から外す（クライアントで `task.store` を見て外す）。通知とアクティビティには出す。URL で直接開いたワークスペース・プロジェクト・タスクは見せ、上に「この Store は非表示です」と「表示する」（`PATCH {hidden: false}`）の帯を出す
39. 非表示の Store の色は残す（表示し直したとき同じ色）
40. 色の割り当て: `color` があればそれ、なければ今の並び順の割り当て（`color` を持つ Store も順番を1つ使う。色が重なることは許す）
41. **マイタスクのワークスペース別**: まとめ方に「ワークスペース」を足す（期限・プロジェクト・ワークスペースの3つ）。見出しはワークスペースの名前と Store のバッジ。並びはサイドバーと同じ
42. **非表示の Store の案内**: マイタスクの問い合わせの結果のうち非表示の Store のものの件数を「非表示の Store に n 件（org-b）」と出し、SettingsStores へのリンクを付ける。0件なら出さない
43. **Store 名付きの ID**: 表示中か否かを問わずマウント中の Store のあいだで、同じ接頭辞のワークスペースが2つ以上の Store にあるとき、その接頭辞の ID を `<store>:<ID>` で出す（`IdLabel` に Store を渡す。Go の `index.DisplayID` と同じ規則。パレットの検索は `<store>:<ID>` の入力も受ける）。ULID のホバーは今のまま
44. **コメントの本人確認**: `GET /api/accounts` の `id`（`host/login`）のどれかとコメントの `author` が一致するときだけ編集・削除を出す。`author` が空なら誰も出さない。アカウントは `web/core` に `AccountsCache` を作って持つ（4f-2 の設定と共有。`stores.changed` と操作のあとで取り直す）
45. **StoreMove の入口**: TaskDetail・ツリーの「…」とパレットに「別の Store へ移動…」。マウント中の Store が2つ以上で、タスクの Store が閲覧のみでないときだけ出す
46. **ダイアログ**: 見出しに ID・タイトル・子孫の数。移動先の Store をラジオで並べる（色の点、名前、`remotes[0]`。閲覧のみは無効にして理由。移動元は出さない）。移動先のワークスペースは選択欄（2つ以上のとき。1つなら名前だけ）。親は任意で、移動先の Store のタスクを検索して選ぶ（関連の追加の検索の部品を使う）。選ぶたびに `move-store/plan` を取り直す
47. **予定の表示**: 「移動先で変わること」に、ID（「移動先の <ワークスペース> で振り直します」。番号は予告しない。作成と同じ）、親（移動元の親から外れて、移動先の親かワークスペースのルートへ）、移動元に転送ファイルを残すこと。「外す参照」に `unlinked` を種類（`blocked_by` は「ブロックされている」、プロパティは名前、`sprint`・`releases` は「スプリント」「リリース」）・相手の ID・名前で並べる。`hidden` があれば注意の帯「見えなくなる人: …」と「移動前の内容は <移動元> の git 履歴に残り、<移動元> を読める人は過去の内容を見られます」。scoped が要るときは「プロジェクトの表示の設定を持っていく / 持っていかない」のラジオと、持っていくときに外れるもの（`dropped`）
48. **実行**: ボタンは「移動」（外す参照があれば「n 件の参照を外して移動」）。`confirmed` に見せた予定を添えて送り、409 `move.changed` なら予定を取り直して「内容が変わりました。確かめてから移動してください」と出す。成功したら移動先のタスクを開き、トースト「API-12 を org-b に移動しました（WEB-45）」。足元は `hh mv <ID> --store <名前> [--workspace] [--parent] [--scoped carry|drop] --yes`
49. スキーマの違反（usecase の `move.invalid`。タスクごとの違反を args に持つ）は、ダイアログの中にタスクごとの違反を並べる
50. undo の `undo.blocked`（`move-in`・`move-out`）は 4e のトーストのまま
51. **E2E**: harness に複数 Store（2つ、接頭辞が重なる組を含む）と閲覧のみの Store（ローカルの bare を書けなくして `hh sync`）を作る設定を足す。4f-2 で Device Flow（harness が `cmd/fakegh` を起動）・トークンなしのアカウントの追加・発見（`--from`）とマウント・URL で追加・解除・既定・色・表示、4f-3 で閲覧のみの編集の無効・非表示の Store・マイタスクのワークスペース別・Store 名付きの ID・Store 間の移動を通す

### 4f-1 の plan で足した細部（2026-11-04）

4f-1 の plan（`2026-11-04-phase4f1-go.md`）を組むときに決めたもの（52〜64）と、API の決まり・実装で決まったこと（65〜70）。どれも概要の細部 1〜16 と、フェーズ3の usecase の形から導いた。実装で plan から変わった点は、4f-1 の plan 末尾の「実装で plan から変えた点」と、細部ごとの実装とテストの表が詳しい。

52. **poll の手放し**（細部 11 を直す）: `Poller.ReleaseFor(path, fn)` が、repo を手放してから `fn`（`StoreRemove`）が終わるまで `pollMu` を持つ。消せずに断られたときは poller が Store を黙って開き直す（`restored`。`tasks.changed` は出さない）
53. **`PATCH /api/stores/{name}` で `color`・`hidden` のどちらもない**: 400 `store.set_args`
54. **`stores.changed` は1回の書き込みで2回届くことがある**（API の書き込みのあとと、poll が config の変化を見たとき）。受け手の取り直しは冪等。最初の poll では出さない
55. **発見の `choice.required`**: 細部 5 の 422 は `GET /api/stores/discover` の入口だけ。ほかの `choice.required`（`POST /api/stores/add` の `account`、`move-store` の `workspace`・`scoped`）は 400
56. **`choice.required` の文言**: `args.what` の種類（アカウント・移動先のワークスペース・scoped）ごとの言い方。`--account` は「アカウント」、`--workspace` は「移動先のワークスペース」
57. **`auth.token` の Code**: 空のトークンは `auth.token_empty` に分ける。型のない誤りに `auth.client_id`・`auth.device_failed`・`auth.canceled`・`auth.mismatch` を付ける。`auth.device_failed`（ネットワークの失敗・コードの期限切れ・拒否）は `KindInvalid`（422）。英文と終了コードは変えない
58. **Device Flow のキャンセル**: `auth.Options.Context`・`auth.ErrCanceled`、usecase の `DeviceCode`・`AuthLoginInput.OnCode`・`Context`。やめる操作は進行中の HTTP の呼び出しを止め、保存に入っていれば `done` で終わる。サーバーの `Shutdown` は裏の goroutine が返るまで待つ
59. **`POST /api/auth/device` の成功は 201**。`gh`・`env` のアカウントで開始すると 422 `auth.no_device_flow`。`keychain`・`device` を `POST /api/accounts` に渡すと 400 `auth.token_source`。`PUT /api/accounts/{name}/token` の応答は `uses` を含む
60. **`move-store` の応答**: `MoveStoreResult.View()` の `MoveStoreResultView`（phase3 spec の手順 9 と同じ形）。CLI を揃えるのは構成見直しの回 4
61. **`MovePlanView`**: `unlinked[].field` は `props.` を付けたまま。`scoped.required` は選択がまだないときだけ true。ワークスペースが決まらない予定は 400 `choice.required`（`--workspace`）。`scoped` の不正な値は schema の enum で 400 `request.invalid`
62. **細部 15**: 親設計は外部の文書なので、phase4 spec の「親設計の修正」に1行足した（e98cec3）
63. **SSE の型**: `web/core/connection.ts` の `Handlers` に `auth.device` の型だけ足し、`stores.changed` の受け取りは 4f-2
64. **`StoreView.default_store`**: `GET /api/stores` の `default` は CLI が `--store` なしで選ぶ Store のまま（2つ以上なら全部 false）。config.yaml の `default_store` が指す Store は `default_store`。SettingsStores の「既定の Store」「既定にする」は `default_store` を使う
65. **（API の決まり a）`choice.required` の status**: discover は 422、move-store（`move-store/plan` を含む）は 400。GUI は `type` で分岐し、status では分岐しない
66. **（API の決まり b）`move.invalid` の `args.violations`**: `"  <ID> <title>: <violation>"` の文字列の配列。`schema.violation` はオブジェクトの配列。4f-3 の細部 49 は両方を扱う
67. **（API の決まり c）`MovePlanView.unlinked[].field`**: `props.<name>`。画面は `props.` を外して出す（`blocked_by`・`sprint`・`releases` はそのまま）
68. **（API の決まり d）書き込みの応答の `default`**: Store の追加と `PATCH` の応答は `default` を持たない `StoreView`。GUI は `default_store` を使い、書き込みのあとは `GET /api/stores` を取り直す
69. **Device Flow の 409**: `auth.device_running` の `args` に走っているフローの `{id, user_code, verification_uri, expires_at}`（コードがまだ出ていなければ `id` だけ）。GUI は引き継ぐか、`DELETE /api/auth/device/{id}` でやめる
70. **トークンの書き込み**: `PUT /api/accounts/{name}/token` と Device Flow の完了は、sync のループと poller をすぐ動かす（トークンがなくて閲覧のみだった Store が書けるようになるため）。GUI は応答のあと `GET /api/stores` を取り直す

## 4f-2 の plan で足した細部（71〜）

4f-2 の plan（`2026-11-04-phase4f2-settings.md`）で足したもの。71〜87 は plan の番号のまま、88〜 は実装で決まったもの。実装とテストの場所は plan の末尾の表にある。

71. **E2E の keyring**（2026-11-04 に利用者が決定）: Device Flow の完了・トークンの貼り付け・ログアウトは OS の keychain に書くが、CI の E2E（ubuntu）には Secret Service がなく、手元の macOS では本物の Keychain に書いてしまう。Go に `auth.FileKeyring(path)`（JSON のファイルに持つ `auth.Keyring`）を足し、ビルドタグ `e2e` のときだけ CLI が `$HEDGEHUG_HOME/e2e-keyring.json` のそれを使う。`make e2e` はタグ付きで `bin/hedgehug-e2e` を作り、harness はそれを使う。リリースのバイナリ（タグなし）は今のまま OS の keychain を使う
72. **E2E の GitHub**: harness は Device Flow とトークンの貼り付けを使う spec だけ `cmd/fakegh` を起動し（`fakegh serve --addr 127.0.0.1:0 --seed <json>`。`device_interval: 0`、利用者 `e2e`）、config.yaml に `hosts: [{host: github.com, api: <fakegh>, web: <fakegh>, client_id: e2e}]` を書く。承認は `fakegh device approve <コード> --user e2e --server <fakegh>`、貼るトークンは `fakegh token add <トークン> --user e2e`
73. **閲覧のみの判定を前倒し**: 細部 34 の `StoreDirectory.readOnly(store)` と理由の日本語を、設定の画面（細部 17・23）が使うのでこの回で作る。`read_only` の値は usecase の `Access.Why()` の英文（`the GitHub permission is read`・`the remote cannot be written`・`the remote denied the push`）なので、その3つを「GitHub の権限が読み取りです」「remote に書き込めません」「remote が push を拒みました」に直し、ほかの文はそのまま出す（`web/core/stores.ts` の `readOnlyReason`）。4f-3 はこれを使う
74. **`AccountsCache` の取り直し**: `GET /api/accounts` は `GET /user` を呼ぶので、自分からは読み込まない。アカウントと認証・Onboarding・CreateStore・URL で追加の画面が開いたときに `load()`（読み込み済みなら何もしない）、操作のあとに `reload()`。`stores.changed` では、読み込み済みのときだけ取り直す（細部 44 の「`stores.changed` と操作のあと」を、読み込んでいないときは呼ばない形にした）
75. **`stores.changed` で捨てる同期の状態**: `Connection` に `retainSync(names)` を足し、`StoreDirectory` が Store の一覧を取り直したあとに今の Store の名前で呼ぶ（細部 33 の「消えた Store の `conn.sync` の項目を捨てる」）。変わったときだけ購読者に知らせる
76. **設定のルート**: `/settings/accounts`（`{ page: "accounts" }`）と `/settings/stores`（`{ page: "stores" }`）。Store の名前を持たない（このマシンの設定）。キャンバスの順に「アカウントと認証」「マウント中の Store」「同期と保守」。ナビのバッジ（キャンバスの数）は出さない（何の数かを決める元がない）
77. **パレット**: 「設定のアカウントと認証を開く」「設定のマウント中の Store を開く」を足す（細部 105 と同じく、画面を作った Task で足す）
78. **Onboarding の閉じ方**: Onboarding は Store が0件になったときに出て、「始める」を押すか、CreateStore で作り終えるまで出し続ける（マウントで Store が1件以上になっても、はじめるの段を見せるため閉じない）。`App.svelte` が「Onboarding の途中」を持つ。最後の Store を解除して0件になれば、また出る（細部 27）
79. **Onboarding のアカウントの段**: アカウントが0件なら「次へ」を無効にし、理由（「アカウントを1つ追加してください」）を出す（Store を書くにも作るにもアカウントが要る）。トークンなしのアカウントでも次へ進める（細部 29）。アカウントの追加は細部 19 の部品をそのまま使う
80. **発見の段のアカウントなし**: トークンを持つアカウントがないときは自動では探さず、「GitHub で探すにはトークンのあるアカウントが要ります」と「フォルダから探す」だけを出す（usecase は `account.none` で断るので、先に呼ばない）
81. **発見の候補の名前**: 候補の `name`（usecase が提案した名前）を呼び名の欄の初期値にする。マウントの本文は選んだ候補の `{name, url, account}` を、呼び名を欄の値に変えて送る（`MountCandidate` に `permission` はなく、余分なフィールドは受け付けない。権限は画面に残して「はじめる」の段に出す）。空の呼び名の候補は送らず、その行に「名前を入れてください」を出す
82. **Device Flow の引き継ぎ**: 開始が 409 `auth.device_running` なら、`args` にコードがあればそのフローを引き継いで同じ画面を出し（「別の画面で始めたログインを続けます」）、`id` だけなら「やめてやり直す」（`DELETE` してから開始し直す）を出す（細部 69）。期限（`expires_at`）を過ぎたら「コードの期限が切れました」と「やり直す」を出す。`auth.device` の `error` は `new ApiError({status: 0, type: code, detail: message})` を作って `errorMessage` に渡して出す（`errorMessage` は `ApiError` だけを文言にする）
83. **`gh`・環境変数のカード**: 取得元が `gh`・`env:VAR` のアカウントの「…」には「Hedgehug が持っているものはありません」だけを出す（細部 20）。カードの状態が `error` のとき、その文（`AuthStatus.error`）は英文のまま出す（GitHub やコマンドの文で、Code がない）
84. **URL で追加の名前**: 名前は必須で、URL の最後の部分（`.git` を除く）を、名前が空のあいだだけ提案する。URL は1行に1つ（空行は捨てる）
85. **CreateStore のアカウント**: アカウントの一覧は `AccountsCache` から取る（細部 32）。読み込み中・読めないときは選択欄を出さず、`account` を送らない
86. **remote のない Store の解除**（細部 27 の「既定でオン」を、remote のない Store について直す）: サーバー（`lockToDelete`）は remote のない Store のクローンを、タスクが失われるため常に消さない（409 `store.unpushed`）。「先に同期してください」は remote がないので実行できない。解除のダイアログは、`remotes` が空の Store では「ローカルのクローンも消す」を既定でオフにし、オンにすると「remote のない Store は、クローンを消すとタスクが失われるため、クローンごとは解除できません。「クローンを残して解除」を使ってください」と出す。この Store の 409 `store.unpushed` は、同期の案内ではなくこの文を出す
87. **discover --from は HEAD ではなく refs/heads/main を見る**: 素の `git init --bare` の HEAD は master のままで、push は main にしか書かないので、HEAD を見ていた `discover --from` は Store を黙って見落としていた（`gitrepo.MainHasFile`）。2026-11-04 に利用者がこの回で直すと決定。E2E の harness は `--initial-branch=main` を付けずに bare を作る
88. **E2E の補足**（実装で決まった）: `fakegh serve --seed` は JSON ではなくファイルのパスを取る（harness が `<home>/fakegh-seed.json` を書く）。E2E のビルドは `HEDGEHUG_HOME` がなければ panic せず、終了コード 1 のエラーで止まる（手元の macOS で本物の Keychain に書かないため）。`make e2e-bin` は `web` に依存する（`MAKEFLAGS=-j` で `make web e2e-bin` が並行に走り、バイナリが一つ前のバンドルを埋め込んだため）
89. **Device Flow の E2E と、ログアウトのあとの表示**: E2E の承認は `worker` として行う（`e2e` で承認すると既にある `e2e` のアカウントに合流する）。ログアウトしたあとの keychain のアカウントは、usecase が取得元を `keychain` のまま持つので「トークンなし」ではなく `error` で出る。カードはこのとき「ログインし直す」を出す
90. **トークンの足し分**: `--fs-15`（15px）、`--control-h-lg`（40px。入力欄の高さ。生の 40px があった7ファイルを置き換えた）、Onboarding の `--logo-size`・`--step-size`・`--onboarding-w`・`--onboarding-pad-x`
91. **色の名前**: パレットの7色の日本語（ティール・オレンジ・パープル・スカイ・ピンク・オリーブ・ブラウン）はキャンバスにないので実装者が付けた（`StoreColorPicker.svelte` の `STORE_COLORS`）

## 4f-3 の plan で足した細部（92〜）

4f-3 の plan（`2026-11-04-phase4f3-stores.md`）で足したもの。92〜106 は plan の番号のまま、107〜 は実装で決まったもの。実装とテストの場所は plan の末尾の表にある。

92. **「Store が2つ以上」の判定は、表示中か否かを問わない**: `useManyStores`（`web/app/svelte-store.ts`）・`TreeView` の `multi`・`KanbanView` の `manyStores`・`useListScope` の `saveBase` は今のまま、マウント中の全 Store で数える（非表示を外すと、Store のバッジが出たり消えたりして、同じタスクの見た目が設定で変わるため。細部 43・45 も全 Store で数える）
93. **非表示の Store を外す場所**（細部 38 を、今のコードの場所に当てた）: サイドバーのワークスペース（`Sidebar.svelte` の `workspaces`。ホームで強調する「最初のワークスペース」も表示中の最初）、サイドバーの保存済みビュー（`ViewEntry.store`）、サイドバーのマイタスクの件数（`myCount`）、ツリー・カンバン・テーブル（`TreeView` の `roots`、`KanbanView`・`TableView` の `tasks`、ブロック中・中止の件数と一覧）、マイタスク、パレットのワークスペース・ビュー・Store の一覧・タスクの検索・「最近開いたもの」、作成のワークスペースの選択（`CreateTask`・`CreateProject`。`resolveCreateContext` の既定も表示中の最初）、ワークスペースの追加の Store の選択（`WorkspaceAdd`）。外すのはクライアント（`task.store`・`entry.store` を見る）。通知・アクティビティ・設定（Store ごとの設定の選択欄・同期と保守・purge）は外さない。ワークスペースやプロジェクトのページの中（URL で開いたもの）は、その Store のものなので外さない
94. **表示中が0件にはならない**: 4f-2 の細部 25 で最後の1つは非表示にできないので、「表示中の Store がない」状態は作らない（config.yaml を手で書いた場合は、今の「ワークスペースがない」表示のまま）
95. **Store 名付きの ID の計算**: API は Store 名付きの ID を返さない（`TaskView` は `id` と `store` だけ）。`StoreDirectory` に `sharedPrefixes(): Set<string>`（マウント中の全 Store の `workspaces()` の接頭辞を大文字にして、2つ以上の Store にあるもの。Go の `mount.SharedPrefixes`（`internal/app/storage/mount/mount.go`）と同じ規則。Go が削除済みのワークスペースの接頭辞も数えるかを読んで合わせ、`workspaces()` に出ないものを数える必要があれば止めて報告する）と `displayId(store, id): string`（`id` が `<接頭辞>-<番号>` の形で、接頭辞が `sharedPrefixes` にあれば `<store>:<id>`、ほかは `id`）を足す。`IdLabel` に `store?: string` を足し、渡されたら `displayId` で出す。`IdLabel` を使わず ID を出している場所（下の Task 5 の表）も `displayId` を通す。トースト・ダイアログの見出しの ID も同じ
96. **パレットの `<store>:<ID>`**: `detectMode`（`web/app/palette/search.ts`）は `:` を含む語をクエリとして扱うので、入力全体が `<マウント中の Store 名>:<ID>` の形なら ID として扱い、`GET /api/tasks/{id}` に `<store>:<ID>` をそのまま渡す（サーバーの `ForTask` が `store:ID` を解決する。`internal/app/usecase/mount_ops.go`）。Store 名でない語の `:`（`status:todo` など）は今のままクエリ
97. **`store.read_only` の文言の理由**: `messages.ts` の `store.read_only` は `args.reason`（英文）をそのまま出していたので、`readOnlyReason` を通す（理由の言い方を画面のどこでも揃える）
98. **閲覧のみの帯**: `web/app/states/StoreBands.svelte`（props `store`）を作り、閲覧のみなら「この Store は閲覧のみです（理由）」、非表示なら「この Store は非表示です」と「表示する」（細部 38）を出す。TaskDetail（今の削除済み・壊れた帯と同じ並び）、ワークスペースのページ、プロジェクトのページ、ビューのページ（その Store に保存されたビュー）の上に置く。見た目は `Banners.svelte` の `band` と同じトークン
99. **閲覧のみで無効にするものの足し分**（細部 35 を、今のコードの場所に当てた）: 同じ Store の中の移動（「別のワークスペースへ移動…」と、ツリーの字下げ・並べ替え）、カンバンの列の「+」とツリーの追加（作成）、保存済みビューのレイアウトの切り替えと列・まとめ方の保存（`views.update`。コミットを作るため）、通知の「元に戻す」（`revert`）。閲覧のみの Store のワークスペースへの作成は、作成のダイアログでそのワークスペースを無効にして理由を出す（細部 35）。プロジェクトの作成も同じ。ワークスペースの追加は、閲覧のみの Store を選択欄で無効にする（`ViewSave` と同じ形）
100. **E2E の閲覧のみの Store**: harness に `Hedgehug.readOnlyStore(name, prefix)` を足す。`Peer.publish(name, prefix)` で別の home に Store を作って push し、その bare の `objects` の下のディレクトリをすべて 0o500 にしてから（Go の `gitrepotest.LockObjects` と同じ）`hh store add <name> --remote <bare>` する。マウントの同期の `CheckAccess` が `gitrepo.Writable` で書けないことを見つけ、`not_writable`（「remote に書き込めません」）の閲覧のみになる。片付けの前に 0o700 に戻す。root で走っているときは `test.skip`（root は権限を無視するため）。E2E は `HEDGEHUG_AUTO_SYNC=0` なので、`store add` のあとに `hh sync` を流し、画面を見る前に `hh store list --json` の `read_only` が入ったことを確かめる（入らなければ harness の問題として止めて報告する）
101. **Store 間の移動のあとのキャッシュ**: `MoveStoreResultView` に `commit` がないので、`applyWrite`・`noteOwnCommit` は使わない。成功したら `cache.refreshTrees()`（画面のツリーだけを取り直す）を呼び、移動したタスクを ULID で開く（ULID は移動で変わらない）。ツリーでない一覧（マイタスク・カンバン・テーブル）は poll の `tasks.changed` で取り直る。`commit` がないので、この `tasks.changed` は外からの変更（ライブの印）として出る。これは受け入れる（2026-11-04。API の形を変えないため）
102. **移動のダイアログの見出しの子孫の数**: 予定（`MovePlanView.tasks`）の件数から1を引いた数を「子孫 n」で出す（最初の予定を取るまでは出さない）。キャンバスのコメント・作業時間の数は、数える API がないので出さない
103. **パレットの「別の Store へ移動…」**: `CommandItem.icon` に `store` を足す（`icons` に Store の形がなければ、キャンバスの SettingsNav の円柱を足す）
104. **非表示の Store も移動先に出す**: 移動先のラジオは非表示の Store も並べる（非表示は表示の設定で、書き込みの先として選べなくする理由がないため。細部 46 の「移動元は出さない」以外は全 Store）
105. **正確な参照は非表示の Store で外さない**（2026-11-04）: パレットに打った ID・旧 ID・ULID・`<store>:<ID>` は、URL と同じようにそのタスクを開く（非表示の Store のタスクでも。タスクのページに「非表示」の帯が出る）。非表示で外すのは、語の検索・クエリ・「最近開いたもの」だけ（細部 93 の「パレットのタスクの検索」はこの意味）。`detectMode` と `<store>:<ID>` の判定に渡す Store 名は、非表示を含むマウント中の全 Store
106. **移動先の親がワークスペースを決める**（2026-11-04）: Go の `place` は親があればその親のワークスペースを使うので、親を選んだら移動先のワークスペースの選択をその親のものにする（ワークスペースが未選択でも予定は取る）。別のワークスペースを選んだら親を外し、移動先の Store を変えたらワークスペースと親の両方を外す
107. **カンバンの列の「+」の無効**（実装で決まった）: その「+」が作る先の Store がすべて閲覧のみのときだけ無効にする。ワークスペース・プロジェクトのページではボードの Store だけ、それ以外ではマウント中の全 Store（細部 99 の「全 Store が閲覧のみのとき」を、作る先で決めた）
108. **パレットの閲覧のみ**: 閲覧のみの Store のタスクには「詳細を開く」だけを出す（状態の変更・移動・子タスクを作るは出さない）。作成・ワークスペースの追加のコマンドは、書ける Store がなければ出さない
109. **作成の親が閲覧のみ**: 親のタスクのワークスペースが閲覧のみなら、作成のダイアログは送信を無効にして閲覧のみの文を出す
110. **無効の理由の出し方**: 理由は `ui/Button` の `why`（`title` と `aria-describedby`）で出す。ラジオの注記は `aria-describedby`、閲覧のみのテーブルの行は先頭のセルの隠した `span` で読ませる
111. **`StoreBadge` の「閲覧のみ」**: `useReadOnly` で決める。`tone` は色だけで、知らない Store にも `"readonly"` を返す
112. **パレットの語の検索・クエリ**: 非表示の Store は、問い合わせに `-store:<名前>` を足してサーバーで外す（正確な参照は細部 105 のとおり外さない）
113. **パレットの「別の Store へ移動…」のアイコン**: 既存の `database`（円柱）を使う（細部 103 の `store` は足さない）
114. **本文の参照とメンション**: 本文の `#<ID>` とメンションの挿入は Store 名なしの ID のまま（Go の本文の参照の正規表現に Store 名がない）。`task.moved` のメッセージはサーバーが持つ ID のまま
115. **移動のダイアログの移動元の親**: 「<移動元の親> から外れて」の文は、`MovePlanView` に移動元の親がないので、task のキャッシュから作る
116. **E2E の接頭辞が重なる Store**: `hh store init` は使われている接頭辞を断るので、`Peer.publish` で別の home に作って push し、`hh store add` でマウントする
117. **パレットの undo は閲覧のみで止めない**: undo は利用者が自分でした操作を戻すので、Store が閲覧のみになっていても出す。書けなければサーバーが断り、トーストで知らせる（細部 50 の `undo.blocked` と同じ扱い）
