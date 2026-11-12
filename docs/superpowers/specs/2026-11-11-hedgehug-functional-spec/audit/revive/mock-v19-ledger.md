# モック修正の台帳(仕様 01〜08 の復活の反映・2026-11-12)

出どころ: 454e1b3..HEAD の仕様 01〜08 の差分(各ファイル末尾「モックからの変更点」の追加行と本文)、audit/revive/answers.md、audit/mock-v17-decisions.md。今回はこの台帳だけで、モックのコードは変えない。

## 件数(JS ファイルごと)

| ファイル | 件数 |
|---|---|
| data.js | 29 |
| query.js | 11 |
| ui.js | 4 |
| layouts.js | 17 |
| screens.js | 11 |
| detail.js | 18 |
| planning.js | 9 |
| settings.js | 23 |
| extras.js | 14 |
| index.html | 2 |
| 直す合計 | 138 |
| 見せない(N-) | 10 |

番号は欠番あり(確認だけで直す箇所がないと分かった行を外した)。

## 全体の注意

- **保存データの版**: 上げる。data.js の seed() の `v: 6` と load() の `v.v === 6` を 7 にする(DA-05)。接頭辞を必須にして個人ワークスペースの接頭辞を入れる、組み込みプロパティ `estimate` を足す、プロジェクトに `rules` を持たせる、リンクに `review`・`checks`・`auto` を持たせる、アイテムに `unlinked` を持たせる、Store に `path`・`mirrors`・`isDefault` を持たせる、の 6 つが保存データの形を変えるため。版が合わない古い保存データは、いまの load() の動きどおり捨てて seed() に戻る。保存キー `hedgehug-mock-v4` はそのまま(キーの末尾は保存データの版ではない)。ビューと画面の状態を入れる `-local` には「最近開いたもの」を足す(DA-27)。
- 方針(mock-v17-decisions.md): 架空の GitHub と remote をモックに持たせる。「画面の状態」の切り替え(時計のずれ・他人のトークン・組織の一部など)は足さない。M2(待たせる 1 秒)、M4(保存の失敗は見せない)に従う。仕様に文言・動きがないものは既存の似た文言に合わせて仮に置き、置いたものを mock-v17-decisions.md の「仮に置いたもの」の形で残す。
- 列「主ファイル:関数」の先頭が担当ファイル。かっこの中が関わるファイル。行頭の番号の英字は主ファイル(DA=data.js、QU=query.js、UI=ui.js、LA=layouts.js、SC=screens.js、DE=detail.js、PL=planning.js、ST=settings.js、EX=extras.js、IX=index.html)。
- 仕様の箇所は「ファイル番号:見出し」。「モック」=モックからの変更点の表。

## data.js

| 番号 | 仕様の箇所 | 画面と部品 | 直す内容 | 主ファイル:関数(関わるファイル) |
|---|---|---|---|---|
| DA-01 | 01:用語と階層、03:ワークスペースの作成、モック「接頭辞」 | すべての ID 表示(ツリー・テーブル・詳細・検索・通知・アクティビティ) | ID をいつも `HH-12`、プロジェクトを `HH-P1` にする。接頭辞がないときの `#12`・`#P1` の分岐をやめる | data.js:itemCode, projectCode(全画面から呼ばれる) | 済
| DA-02 | 03:接頭辞、02:関連・転送記録・旧 ID | 旧い接頭辞でも見つかる(パレット・検索) | 旧い接頭辞が空のときの `#12` 形をやめる。changePrefix は空の接頭辞を formerPrefixes に入れない。別の接頭辞への変更は今のまま残す | data.js:oldCodes, changePrefix | 済
| DA-03 | 03:ワークスペースの作成 | サンプルの「個人」ワークスペース | 接頭辞なしのサンプルをなくす。st_me の「個人」に接頭辞 `ME` を付ける(アイテムの ID が `ME-1` になる) | data.js:seed(wsMe) | 済
| DA-04 | 03:ワークスペースの作成 | ワークスペースの作成 | 接頭辞を必須にする。引数の既定 `prefix \|\| ''` を外す | data.js:newWorkspace(呼び出し元は detail.js:openWsAdd、settings.js:addStore) | 済(settings.js:addStore の「メイン」は接頭辞 MAIN の暫定。ST の担当が入力欄方式に置き換える)
| DA-05 | 01・03・02(下記の 6 点) | 保存データ全体 | 版を 6 から 7 に上げる(seed の `v:` と load の判定)。理由は「全体の注意」の 6 点 | data.js:seed, load | 済
| DA-06 | 02:プロパティの種類、03:組み込みの 8 種、モック「見積もり時間」 | 組み込みプロパティ | 8 つ目 `estimate`(名前「見積もり時間」、型「時間」、組み込み)を、ポイントと関連システムの間に足す。テンプレートには持たせない | data.js:defaultSchema | 済(型は `hours`。layouts.js PropInputRaw・detail.js CreatePropInput・settings.js の型の表示に最小の分岐だけ足した)
| DA-07 | 02:作業時間、05:見積もりと実績 | 見積もり時間の読み出し | `estimateOf(it)`(時間・0.25 刻み・空は null)を足す。ポイントの pointsOf と並べる | data.js:pointsOf の隣に estimateOf(使う側は detail.js・layouts.js・planning.js・query.js・extras.js) | 済
| DA-08 | 05:見積もりと実績 | サンプルのアイテム | Sprint 24 と完了済みスプリントの作業項目のいくつかに見積もり時間を入れる(ポイントのみ・時間のみ・両方・作業時間なしが揃うように)。ポイントと時間の差が超過になる行も 1 つ作る | data.js:seed(mk の props) | 済
| DA-09 | 03:プロジェクト別のルール(使うステータス・遷移の制約・テンプレート・自動化) | プロジェクトのデータ | プロジェクトに `rules`(`statuses`: 使う id の配列か null、`transitions`: From→To のチェック、`templates`: 使う id の配列か null、`kindDefaults`、`offAutomations`: 動かさないルールの id、`automations`: 専用ルール)を持たせる。`usableStatuses(projectId)`・`statusUsable(it, id)`・`projectTransitions(p)` を足す。初期ステータスと分類の既定は常に使う | data.js:statusOf の近くに新しい関数、mkProject(seed 内)(screens.js・layouts.js・detail.js・extras.js・planning.js が使う) | 済(形は「土台で足したもの」のとおり。台帳の `statuses`(使う id)は、仕様の「外したものを保存・あとから足したものは初めから使う」に合わせて `offStatuses`(外した id)にした)
| DA-10 | 03:プロジェクト別のルール | サンプルのプロジェクト | 「v2 画面リニューアル」に、使わないステータス(Pending)、プロジェクトだけの遷移の制約、使うテンプレートの限定、動かさないルールの指定、専用ルール 1 件を入れる。4 区画のどれも中身がある状態にする | data.js:seed(mkProject の pV2) | 済
| DA-11 | 02:ステータスの変更、03:使わないステータス、モック「使わないステータスの選べない範囲」 | ステータスのメニュー・カンバンのドラッグ・一括変更・自動化 | checkStatusChange で、プロジェクトが使わないステータスを断る(文言「このプロジェクトでは「{ステータス}」を使いません」)。遷移の制約は、ワークスペースとプロジェクトの両方が許す先だけ許す。移動・同期・値の置き換え・診断の修復は checkStatusChange を通さない | data.js:checkStatusChange(layouts.js:A.statusMenu・D.kanban、extras.js:bulkPlan) | 済
| DA-12 | 03:実行の規則、モック「自動化の動く範囲」 | 自動化が動く経路 | 利用者が値を変える操作(一括変更はアイテムごと、ツリーの Space、パレットの「ステータスを変更…」)だけで runAutomations を呼ぶ。同期・診断の修復・値の削除の置き換え・「制約を無視して…」「自分の値に戻す」・種類の変更・移動・作成・複製・同期の修復では呼ばない(applyProp に `noAuto` を足して呼び分ける)。「Step がすべて完了したとき」は 1 回の操作で親ごとに 1 回 | data.js:applyProp, runAutomations(extras.js:bulkRun、layouts.js:setItemProp、detail.js:A.moveItem) | 済(applyProp の `noAuto`・`auto`・`remote` はすでにあった。runAutomations を呼ぶのは applyProp(利用者の操作)・commit の Step 完了・PR のリンクだけで、同期・修復・置き換え・移動・作成は呼ばないことを点検した)
| DA-13 | 03:きっかけとすること、03:実行の規則、03:自動化と削除済みのアイテム | 自動化の動き | きっかけ `review_requested`(PR が Open か Draft でレビュー待ちになったとき)・`issue_closed`(リンクした Issue が Closed になったとき)を足す。`pr_merged` に条件「リンク中の PR がすべてマージ済みのときだけ」(Closed の PR は数えない)を持たせる。削除済みのアイテムでは動かさず、復元してもさかのぼらない。プロジェクトの「動かさない」指定を見る。プロジェクトの専用ルールも動かす。使わないステータスのために止まったときも「「{ルール名}」が制約で止まりました」の通知を出す | data.js:runAutomations(settings.js:AutomationsEditor が設定側) | 済
| DA-14 | 02:ステータスの変更、02:アイテムの作成、03:使わないステータス | アイテムの作成 | createItem の作成時ステータスは、遷移の制約を掛けず、プロジェクトが使うステータスに限る。テンプレートはプロジェクトの限定に従う | data.js:createItem, templateProps(detail.js:CreatePropInput) | 済(createItem は使わないステータスを初期ステータスにする。作成ダイアログのテンプレートは usableTemplates・defaultTemplateFor を detail.js が使う。templateProps は変更なし)
| DA-15 | 02:同じワークスペースの中での移動、モック「移動のステータスの置き換え」 | プロジェクトを変える移動(ツリーのドラッグ・Alt+Shift+↑↓・メニューの「移動…」) | moveItem に、置き換えるステータスの対応(`statusMap`)を渡せるようにする。使わないステータスは、対応があればそれ、なければ移動先の同じ分類の既定にする。同期の修復はいつも既定にする。ステータス以外(優先度など)は今のとおり空にしてトーストで知らせる | data.js:moveItem(extras.js:mapValuesTo・relocate、layouts.js:treePlace・moveStep、detail.js:A.moveItem) | 済
| DA-16 | 04:集計の定義、04:キー(`created`・`updated`)、モック「検索の『更新日』」 | 検索・テーブルの作成日と更新日 | `createdAt(it)`(作成の保存の時刻。移動しても変わらない。複製は複製した日)、`updatedAt(it)`(作成の行と、実行者が人か自動化の行のうち最も新しい時刻。実行者が GitHub・同期の行は使わない)を足す | data.js:新しい関数(query.js・layouts.js が使う) | 済
| DA-17 | 06:アクティビティ(GitHub 由来の行)、07:表示と更新、モック「GitHub から取り直した変更」 | アクティビティと履歴 | githubChange を広げる。自動リンク(履歴に実行者「GitHub」・アクティビティ「{ID} に {リポジトリ}#{番号} を自動でリンク」)、レビューの状態の変更(「…のレビューを {状態} に変更」)、タイトルの変更(「…のタイトルを「{新しいタイトル}」に変更」)の行を足す。すべて自動の変更として残す | data.js:githubChange(extras.js:refreshLinks、detail.js:HistoryList) | 済
| DA-18 | 07:リンクの変更の通知、モック「GitHub から取り直した変更の通知」(Q27) | 受信箱 | GitHub から取り直した状態・タイトル・レビュー・自動リンクの変更では、ウォッチ・割り当ての通知を作らない(記録済みの決定を覆す)。自動化が動いたときの「自動化」の通知だけ出す | data.js:githubChange, watchersNotify, notify(サンプルの通知データに GitHub 由来のウォッチ通知があれば seed から外す) | 済(githubChange・自動リンクに通知を作る経路はなく、サンプルの通知にも GitHub 由来のものはない。何も足していない)
| DA-19 | 06:親が循環したとき、06:修復の決まり(Q12) | 同期の場面(アクティビティ・受信箱) | 同期の場面を足す。他の人が作った親の輪を切り、輪に入っていた種類に合わない親子も直す。アクティビティ「{ID} を {行き先の ID} の下へ移動(親の循環)」、実行者「同期」、通知「同期の修復」 | data.js:simulateRemote(新しい step) | 済
| DA-20 | 06:親と別のワークスペース・プロジェクトにある子 | 同期の場面 | 場面を足す。親と別のワークスペースにある子を親のワークスペースへ移す。アクティビティ「{旧 ID} を {新 ID} に移動(親に合わせる)」、プロジェクトだけ違うときは「{ID} をプロジェクト「{名前}」に移動(親に合わせる)」。移動先で使えないステータスは同じ分類の移動先の既定にする。転送記録(旧 ID)を残す | data.js:simulateRemote(新しい step)(extras.js:relocate) | 済
| DA-21 | 06:存在しないスプリント・リリースを指すとき、08:スプリント・リリースの完全な削除 | 同期の場面 | 場面を足す。完全に削除されたスプリント・リリースを指すアイテムを、バックログ・リリースなしに戻す。アクティビティ「{ID} をバックログに戻す(スプリントが完全に削除)」「{ID} のリリースを外す(リリースが完全に削除)」、履歴に「スプリントを変更」「リリースを変更」、通知は本文「スプリント「{名前}」は完全に削除されたため、{ID} をバックログに戻しました」 | data.js:simulateRemote(新しい step) | 済
| DA-22 | 06:削除された値を指すプロジェクト別のルール、06:プロジェクトがなくなったときのルール | 同期の場面 | 場面を足す。削除された値を指すプロジェクト別のルールを外す(アクティビティ「プロジェクト「{タイトル}」のルールから「{値の名前}」を削除(削除された値)」、ルールは「自動化「{名前}」を無効にする(削除された値)」)。なくなったプロジェクトのルールを外す(「…のルールを削除(プロジェクトがない)」)。種類ごとの既定は「{種類} の既定のテンプレートを変更(削除された値)」。通知はそのチェックをした人のマシンに作る | data.js:simulateRemote(新しい step) | 済(仕様 06 の行のうち、「プロジェクトがない」のルールを外す文(…のルールを削除(プロジェクトがない))は、ルールをプロジェクト自身が持つ形ではルールだけが残る状況を作れないため場面にしない。この状況はモックのデータでは起きない。ほかの行(使わないステータス・遷移・テンプレート・専用ルールの無効・種類ごとの既定)は済。場面はアクティビティと通知の行だけを出し、データは変えない
| DA-23 | 06:別の Store へ移動したものと重なったとき(Q11) | 同期の場面・受信箱 | 場面を足す。他の人が親を別の Store へ移したあとに、あなたが作った子は反映しない。「同期の修復」の通知に「…移動先は {新 ID} です({子の ID}「{タイトル}」)」を出す(通知の種類は増やさない) | data.js:simulateRemote(新しい step)、notify | 済
| DA-24 | 06:同期のあとの場面の数 | 手動同期の場面の巡回 | 場面の数が 12 から増えるので、`% 12` と `Math.floor(simStep / 12)` を場面の数の定数にする | data.js:simulateRemote(先頭) | 済
| DA-26 | 08:一覧、08:既定の Store、08:ミラー、08:保存先の表示、モック「ミラー」「既定の Store」 | Store のデータ | Store に `path`(保存先の絶対パス。同名があれば `-2`)、`mirrors`(`{ id, url, last }`)、`isDefault` を持たせる。サンプルは team を既定にし、team にミラー 1 件、upstream(閲覧のみ)にミラー 1 件を入れる。`defaultStore()` を足す。この 3 つは Store の保存にもアクティビティにも残さない(commit を通さない) | data.js:seed(stores)、新しい defaultStore(settings.js・detail.js が使う) | 済
| DA-27 | 01:Store で共有するものと、マシンごとに持つもの、02:コマンドパレット(最近開いたもの) | パレットの「最近開いたもの」 | UI に `recent`(アイテムの内部 id の新しい順、最大 8)を持たせ、このマシンだけの保存(`-local`)に入れる。save と load に足す。画面の状態のキー(isScreenKey)に、スプリントのボードの状態 `sb:` も足す | data.js:save, load, isScreenKey | 済
| DA-28 | 07:表示と更新、07:解除、02:GitHub、モック「PR の自動リンク」「自動リンクの解除」 | PR のリンクのデータ | リンクに `review`(なし・レビュー待ち・承認・変更依頼)、`checks`(成功・失敗・実行中・なし)、`auto`(GitHub が自動でリンクした)を持たせる。アイテムに `unlinked`(解除の記録。`{ repo, number }`)を持たせる。サンプルの PR のいくつかに入れる。検出元は持たない | data.js:seed(links の例)、mk(unlinked の初期値) | 済
| DA-29 | 06:手動で同期する、08:ミラー | 同期のあと | 同期のあと、書き込める Store のミラーの `last` を現在時刻にする(閲覧のみの Store は送らないので動かさない)。失敗は見せない | data.js:syncStore | 済
| DA-30 | 03:自動化、03:変更が及ぶ範囲 | 自動化のきっかけ | プロジェクトの専用ルールとワークスペース全体のルールの両方を、同じ実行の規則で動かす。リンクの `review` が「なし」から「レビュー待ち」に変わったこと、`state` が Open から Closed に変わったことを、きっかけの検知に使えるよう ev に前の値を渡す | data.js:runAutomations(extras.js:refreshLinks が ev を作る) | 済

## query.js

| 番号 | 仕様の箇所 | 画面と部品 | 直す内容 | 主ファイル:関数(関わるファイル) |
|---|---|---|---|---|
| QU-01 | 04:キー(`mentions`)、モック「検索のキーとの対応」 | 検索の絞り込み | `mentions:` を足す。値はログイン名(`@` 付きも可)・`@me`・`none`。説明と、削除されていないコメントの本文から、いまの `@ログイン名` を探す(コードの中も含む、`@` の前の文字は問わない)。`@me` はアイテムの Store での「あなた」。`none` はメンションがない | query.js:termMatches | 済
| QU-02 | 04:キー(`project`)、04:旧 ID の検索 | `project:` | 移動前と接頭辞を変える前の旧 ID(プロジェクトの formerIds と oldCodes)でも、いまのプロジェクトが一致する | query.js:termMatches(case 'project')(data.js:oldCodes) | 済(project: は formerIds と oldCodes をすでに見ていた。oldCodes から空の接頭辞の `#12` 形を除いたので、いまの形だけになる)
| QU-03 | 04:キー(関連の 9 つ)、モック「関連のキー」 | 検索の絞り込み | `blocks`・`blocked_by`・`causes`・`caused_by`・`duplicates`・`duplicated_by`・`relates_to`・`clones`・`cloned_by` を足す(02 の 9 つの表記の空白を `_` にした綴り)。値は ID か `none`。`relates_to` は向きがなく、どちら側からでも一致。`none` はその表記の見出しが一覧に出ないもの。削除済みの相手は数えない。旧 ID でも一致する | query.js:termMatches(data.js:relationEntries) | 済
| QU-04 | 04:キー(`is`)、04:削除済みを探す、モック「`is:invalid`」「`is:deleted`」「`is:overwritten`」 | `is:` の値 | `invalid`(08 の診断の検出項目のうちアイテムを指す行に当たる。診断を実行せず保存された状態で判定)、`deleted`(対象が削除済みのアイテムだけになる)、`overwritten`(このマシンの受信箱に未読の「上書き」の通知がある)を足す。未知の値は何にも一致しない | query.js:termMatches(case 'is')、runQuery(settings.js:doctorFindings、screens.js:myNotices) | 済(`is:invalid` は settings.js の doctorFindings のうちアイテムの行。`is:deleted` は runQuery の第 3 引数 range で範囲を渡す)
| QU-05 | 04:キー(`points`・`estimate`) | `points:`・`estimate:` | `points` の `none` を表に合わせる。`estimate` を足す(数、時間、`none`。比較の演算子が使える) | query.js:termMatches(case 'points' の隣に 'estimate') | 済(points の none はすでに表のとおり。estimate を足した)
| QU-06 | 04:キー(`created`・`updated`)、04:比較と日付 | `created:`・`updated:` | 日付と比較演算子で絞る。日付はこのマシンの時刻帯の日付。`none` はエラー | query.js:termMatches, dateValue(data.js:createdAt・updatedAt) | 済
| QU-07 | 04:キー(`group`)、04:まとめ方、モック「`group:`」 | `group:` | 一致の判定には使わず、まとめ方の指定として読む。2 つ書いたら最初だけ使う。parseQuery の結果から取り出す関数を足す | query.js:parseQuery, termMatches(case 'group')(layouts.js:groupKeyOf・laneOf) | 済
| QU-08 | 04:並べ替え | `sort:` | `sort:system`(関連システムの値の順)、`estimate`、`created`、`updated` を足す。複数のワークスペースの同名の関連システムは 1 つにして最初に現れた位置で並べる | query.js:sortValue, sortItems | 済
| QU-09 | 04:エラー、モック「検索のエラーの種類」 | 絞り込み欄の下の赤字 | `none` が使えないキー(`priority`・`created`・`updated`)に `none` を書くと「{キー} に none は使えません」(たとえば「priority に none は使えません」) | query.js:queryErrors | 済
| QU-10 | 04:候補 | 絞り込み欄の候補 | キーの候補に `mentions`・`estimate`・`created`・`updated`・`group`・関連の 9 個を足す。`parent:`・`epic:`・関連のキーの候補は「HH-12 ログイン画面の改修」の形で、書いたキーのままで入る。`workspace:` の候補は「Web HH」。`is:` の候補に `invalid`・`deleted`・`overwritten`。`mentions:`・`group:` の値の候補 | query.js:suggestToken, suggestPool, suggestCandidates(layouts.js:SuggestList) | 済
| QU-11 | 03:プロパティの予約語、04:組み込みと同じ綴りの追加プロパティ | 検索のキー | 追加プロパティが組み込みと同じ綴り(34 個)のときは、検索のキーとしては組み込みの意味を使う。そのプロパティはテーブルの列とセルでは使え、検索と並べ替えには使えない | query.js:customProp(layouts.js:tableColumnDefs) | 済

## ui.js

| 番号 | 仕様の箇所 | 画面と部品 | 直す内容 | 主ファイル:関数(関わるファイル) |
|---|---|---|---|---|
| UI-01 | 01:キーボード、06:手動で同期する、モック「すべて同期のキー」 | キーボード | Shift+Ctrl+S(Mac は Shift+Cmd+S)で「すべて同期」を動かす。サイドバーがある画面ならどこでも、入力中・ダイアログを開いているときも使える。remote のある表示中の Store が 1 つもないときはパレットの「すべての Store を同期」と同じトースト。Ctrl+K の「ダイアログを開いているときは使えない」はそのまま | ui.js:keydown リスナー(`typing` の判定より前に置く)(screens.js:syncAll) | 済
| UI-02 | 01:キーボード(Enter・Space)、04:ツリーのキー操作、モック「ツリーのキー操作」 | ツリーの行 | 行にフォーカスがあるとき、キーを layouts.js のツリーのキー処理に渡す(↑↓←→・Enter・Space・S・K・C・Alt+↑↓・Alt+Shift+↑↓・メニューキー・Shift+F10)。Tab・Shift+Tab は通常のフォーカス移動のまま。Enter・Space を `role="button"` の要素に効かせる既存の処理は、ツリーの行そのものには効かせない。閲覧のみの Store では何も起こさない | ui.js:keydown リスナー(layouts.js:TreeRow に追加するキー処理) | 済(行は class `tree-row` で、フォーカスされた行に keydown が来たら layouts.js の `treeKey(ev, rowEl)` を呼ぶ。true を返すと既定の動きを止める)
| UI-03 | 01:閲覧のみの Store で変えられるレポートの選択、モック | 閲覧のみの Store のレポート | 累積フロー図の層の分け、見積もりと実績の対象の選択、累積フロー図の期間の入力を、閲覧のみでも無効にしない | ui.js:lockReadOnly(planning.js:ReportsPage の入力の印) | 済(印は属性 `data-ro-ok`。その中の入力は閲覧のみでも無効にならない)
| UI-05 | 02:コマンドパレット(最近開いたもの)、モック「パレットの『最近開いたもの』」 | 詳細パネルを開く入口 | 詳細パネルを開くたびに、そのアイテムを UI.recent の先頭に入れる(重複は除く、最大 8)。A.peek のほか、パレット・リンクが UI.peek を直接入れる箇所からも同じ関数を呼ぶ | ui.js:A.peek(detail.js:paletteResults、extras.js:A.peekOld・movedNotice、data.js:save) | 済(記録は render() の notePeek で一括。UI.peek を直接入れる箇所を直さなくても、パネルが開くたびに UI.recent の先頭へ入る)

## layouts.js

| 番号 | 仕様の箇所 | 画面と部品 | 直す内容 | 主ファイル:関数(関わるファイル) |
|---|---|---|---|---|
| LA-01 | 04:ツリーのキー操作(↑↓←→・Enter) | ツリーの行 | 行をキーボードでフォーカスできるようにする(`tabindex`、動かすフォーカスは 1 行)。↑↓ で見えている前後の行(端で止まる)、→ で開く・最初の子へ、← で閉じる・親の行へ(一番上の階層の行では何もしない。プロジェクトの行は閉じるだけ)、Enter で詳細パネル(プロジェクトの行は概要) | layouts.js:TreeRow, ProjectTreeRow, TreeLayout(ui.js:keydown) | 済(`treeKey` を定義。行は `id`・`tabindex`(Tab の止まりは 1 行、中のボタンは -1)・`data-rowkind`・`data-depth`・`data-fold`。消えた行のフォーカスは次の行へ(render を包んで処理))
| LA-02 | 04:ツリーのキー操作(Space・S・K) | ツリーの行 | Space で完了と未着手を切り替える(完了でなければ完了の既定、完了なら未着手の既定。遷移の制約に反するときは何も変えず 02「ステータスの変更」の文言をエラーのトーストで出す。利用者の操作として自動化のきっかけになる)。S でステータスのメニュー、K で「種類を変更…」(Epic は何もしない)。プロジェクトの行は何もしない | layouts.js:setItemProp, A.statusMenu, TreeRow(detail.js の種類の変更のメニュー) | 済(Space は treeToggleDone、S は A.statusMenu、K は A.changeKind をそのまま呼ぶ。閲覧のみは何も起こさない)
| LA-03 | 04:ツリーのキー操作(Alt+↑↓・Alt+Shift+↑↓)、モック「ツリーのドラッグ」 | ツリーの行 | Alt+↑↓ でメニューの「上へ移動」「下へ移動」(絞り込み中は見えている兄弟の間)、プロジェクトの行はプロジェクトのメニューの同じ項目。Alt+Shift+↑↓ で前の親・次の親の下へ移す。プロジェクトが変わる移動で、移動先のプロジェクトが使わないステータスがあれば、メニューの「移動…」と同じ確認ダイアログ(「ステータスの置き換え」)を出す | layouts.js:moveSibling, moveStep, moveProjectStep, treePlace(extras.js:fixMoveParent、detail.js:A.moveItem) | 済(確認ダイアログは `askStatusMap`/`M.moveStatus`。メニューの「移動…」(detail.js:A.moveTo)が使うよう requests-LA.md の 1)
| LA-04 | 04:ツリーのキー操作(C・メニューキー)、02:アイテムの作成(C) | ツリーの行 | C で、その行の兄弟として同じワークスペース・プロジェクト・親・種類で作成ダイアログを開く(プロジェクトの行は下に Epic を作る)。メニューキー・Shift+F10 で行のメニュー。ツリー以外の C は今のとおり | layouts.js:TreeRow(detail.js:openCreate に兄弟の指定を足す) | 済(openCreate に `{ wsId, projectId, kind, parentId }` を渡す。いまの openCreate がそのまま受ける形)
| LA-05 | 04:ツリーのドラッグ、モック「ツリーのドラッグ」 | ツリー | 行の上端・下端にドロップして兄弟の順を変えられる(子にするのは中央)。ドラッグ以外の経路はメニューの「上へ移動」「下へ移動」 | layouts.js:treePlace, dropZoneOf の呼び出し(ui.js:dropZoneOf) | 済(上・中・下の 3 分割は D.treeOnItem に既にあり、確認のみ。プロジェクトが変わる移動は treePlace から確認ダイアログ)
| LA-06 | 04:カンバンの列、モック「プロジェクトのカンバンの列」 | プロジェクトのカンバン | プロジェクトで使わないステータスの列は、そのアイテムがあるときだけ出し、「使わない」の印を付ける。ワークスペースの画面・検索・ビューは変えない | layouts.js:kanbanColumns, KanbanLayout | 済(プロジェクトの画面だけ、使わないステータスの列をアイテムがあるときに出し「使わない」の印)
| LA-07 | 04:カンバンのドラッグ、モック「使わないステータスの列へのドラッグ」 | カンバンのドロップ | カードのプロジェクトが使わないステータスの列へ落としたときは、何も変えずに「このプロジェクトでは「{ステータス}」を使いません」を出す | layouts.js:D.kanban(data.js:checkStatusChange) | 済(checkStatusChange の理由を、何も変える前に出す。遷移の制約も同じ経路)
| LA-08 | 02:ステータスの変更、モック「ステータスのメニューの項目」 | ステータスのメニュー | プロジェクトが使うステータスだけ並べる。いまの値が使わないものなら、チェックつきで 1 項目だけ残し「このプロジェクトでは使わない」と添える。遷移の制約はワークスペースとプロジェクトの両方が許す先 | layouts.js:A.statusMenu(data.js:usableStatuses) | 済
| LA-09 | 02:プロパティの入力と保存(見積もり時間)、04:セル | 数値・見積もり時間の入力 | 見積もり時間の入力は 0.25 時間刻み。「見積もり時間は 0 以上の数で入力してください」「見積もり時間は小数第 2 位までで入力してください」を、ポイントの検査と同じ場所で出す。空にすると値を消す | layouts.js:setItemProp, PropInputRaw | 済(`hours` は step 0.25。数でない入力は C.propNumber が NaN にして断る)
| LA-10 | 04:テーブルの列、モック「`estimate` のキー・列・一括変更」 | テーブルの列 | 固定の列に「見積もり時間」を足す。列「作成日」「更新日」を足す(表示だけ。相対時間: たった今・N 分前・N 時間前・N 日前、7 日以上は日付)。「更新日」の見出しにポインターを置くと「GitHub と同期による変更は含みません」。「表示する列」メニューの並びは、標準の列を固定順に並べ、そのあとにプロパティ | layouts.js:tableColumnDefs, Cell(data.js:createdAt・updatedAt、ui.js:relTime) | 済
| LA-11 | 04:テーブルのグループ、モック「テーブルのグループの日付」「関連システムのテーブルのグループ」 | テーブルのグループ | 「グループ」に「関連システム」「作成日」「更新日」を足す。関連システムは複数の値を持つアイテムが全部のグループに出る(見出し「関連システムなし」。件数は見出しごと、ツールバーはアイテムごと)。作成日・更新日は日付の見出し | layouts.js:groupKeyOf, TableLayout, LayoutControls | 済(groupKeyOf は複数のグループを返す groupKeysOf に置き換え)
| LA-12 | 04:まとめ方、モック「`group:`」 | 検索・ビュー・マイタスクのクエリの `group:` | クエリの `group:` を、テーブルのグループとカンバンのスイムレーンの入口にする。「グループ」の設定が「なし」以外のときは設定が優先。ツリー・ガントでは使わず、灰色の注記を出す。`sort:` があるときのツリー・ガントの断りも同じ注記にする | layouts.js:groupKeyOf, laneOf, LayoutScreen, TreeLayout, GanttLayout(query.js:parseQuery) | 済(テーブルは tableGroupBy、カンバンは laneBy。注記は queryNote。カンバンの「担当者・プロジェクト・優先度だけ」の注記も仕様 04 の画面の構成のとおり足した)
| LA-13 | 04:検索ヘルプ | 検索ヘルプのダイアログ | ヘルプの表に行を足す: `mentions:@me`、関連のキー(`blocked_by:HH-1` / `relates_to:HH-1`)、`is:invalid`・`is:deleted`・`is:overwritten`、`estimate<=2`、`updated>-7d` / `created:today`、`sort:-updated`、`group:kind`。`workspace:HH / store:メイン` を `workspace:HH / store:team-b` にする | layouts.js:M.queryHelp(行 179 の例) | 済(仕様 04 の表の並びと文言)
| LA-14 | 04:削除済みを探す、モック「`is:deleted`」 | 検索のテーブル | `is:deleted` の検索はテーブルだけに出し、行は読むだけ。行を押すと設定「削除済み」のその行へ移る(08)。ツリー・カンバン・ガントでは案内を出す | layouts.js:LayoutScreen, TableLayout, Cell(settings.js:SettingsTrash が受ける側) | 済(行を押すと `UI.trashFocus` に id を入れて設定「削除済み」へ。受ける側は requests-LA.md の 2)
| LA-15 | 04:旧 ID の検索 | 検索の結果 | 自由な語・`project:`・`parent:`・関連のキーで旧 ID・旧い接頭辞でも見つかる。ID の隣の小さな薄い字の「旧 ID」を、いまの接頭辞での表示に合わせる | layouts.js:viaOldId, oldIdTag(query.js:termMatches) | 済(`parent:`・`epic:`・関連のキーでも旧 ID だけで見つかった行に印)
| LA-16 | 04:画面の状態(スプリントのボード)、05:実行中のスプリントのボード | スプリントのボードの状態 | 実行中のカードのボードは、絞り込み・列の分け方・スイムレーン・列ごとの WIP 制限を、ワークスペースごとの画面の状態(`sb:{ワークスペース}`)として持つ。カンバンのタブとは独立で、保存したビューには入らない。初期は列「ステータス」、スイムレーン「なし」、中止も隠さない | layouts.js:defaultSettings, screenState, KanbanLayout(planning.js:SprintCard、data.js:isScreenKey) | 済(`SprintBoard(wsId, items)` を足した。screenState は任意のキーで同じ形の状態を持つので `sb:{ws}` はそのまま独立。planning.js が使うよう requests-LA.md の 3)
| LA-17 | 04:ID と接頭辞 | ワークスペースの見出し・ツリー・テーブルの ID | 「接頭辞なし」のピルをなくす。ID の `#12` の書き方をなくす | layouts.js:layoutId, TreeRow, ProjectTreeRow(screens.js:PageWorkspace) | 済(layouts.js に `#12`・「接頭辞なし」は残っていなかった。ID は idLabel→itemCode のまま。PageWorkspace の見出しは screens.js 側)

## screens.js

| 番号 | 仕様の箇所 | 画面と部品 | 直す内容 | 主ファイル:関数(関わるファイル) |
|---|---|---|---|---|
| SC-01 | 04:ID と接頭辞、モック「ID と接頭辞」 | サイドバーとワークスペースの見出し | 「接頭辞なし」のピル(行 491 の分岐)と、サイドバーの接頭辞の条件分岐(行 35)をなくす | screens.js:PageWorkspace, Sidebar | 済
| SC-02 | 02:プロジェクトの設定タブ(7 区画)、03:プロジェクト別のルール、モック「プロジェクトの設定タブ」 | プロジェクトの設定タブ | 区画を 3 つから 7 つにする(使うステータス・ワークフローの制約・テンプレート・自動化を足す)。使うステータスはチェック(初期ステータスと分類の既定は常に使うので外せない)。遷移の制約は From と To の表で、空の行は「ワークスペースと同じ」。テンプレートは使うものの限定と種類ごとの既定。自動化は動かさないルールの指定と、このプロジェクトだけのルールの作成・編集・削除。各区画に「ワークスペースの設定に戻す」。閲覧のみの Store では無効 | screens.js:ProjectSettings(settings.js:WorkflowEditor・TemplatesEditor・AutomationsEditor を、プロジェクトの範囲でも使えるようにする) | 済
| SC-03 | 03:プロジェクト別のルール(記録とトースト) | プロジェクトの設定タブのアクティビティとトースト | 記録は 03 の表のとおり: 「プロジェクト「X」の使うステータスを変更」「…のワークフローの制約を変更」「…の使うテンプレートを変更」「…の{種類}の既定のテンプレートを変更」「…で自動化「Y」を動かす(動かさない)ようにする」「…の自動化「Y」を作成(編集、削除)」「…の{区画名}をワークスペースの設定に戻す」。トーストは専用ルールの作成・削除と「ワークスペースの設定に戻しました」だけ | screens.js:ProjectSettings の各操作(data.js:commit) | 済
| SC-04 | 02:プロジェクトの名前の変更・移動・削除、02:別のワークスペースと別の Store への移動、モック「プロジェクトのルールの移動」「移動のステータスの置き換え」 | プロジェクトを別のワークスペースへ移す確認ダイアログ | ダイアログに「ステータスの置き換え」の行(移すアイテムが使っていて、移動先にないステータスを 1 行ずつ。移動先のステータスのセレクト。初期値は移動先で同じ分類の既定)、ルールの移り方(移動先の同じ名前のステータス・テンプレートに合わせて一緒に移る。移らないものは先に一覧で出す)を足す。専用の自動化ルールは置き換えたステータスで書き換えてすべて移る。移動先に同じ名前のルールがあれば断る。「値は名前で合わせる…空になります」の文を「ステータスは選んだものに、優先度などは空に」に直す | screens.js:A.moveProject, moveProjectTo(extras.js:mapValuesTo・relocate・moveBlocked、data.js:moveItem) | 済
| SC-05 | 02:GitHub(リンクの行の部品)、モック「リンクの行の部品」 | プロジェクトの概要の PR の一覧 | 状態が Open・Draft の PR の行に、レビューとチェックの表示を足す | screens.js:ProjectOverview, prChip(detail.js:Peek) | 済
| SC-06 | 06:手動で同期する、01:キーボード | サイドバーの同期ボタンと同期カード | 同期のボタン・パレットのヒントに「Shift+Ctrl+S」を添える。オフラインを先に判定する(同期する Store が 0 件でもオフラインのトーストが先) | screens.js:SyncCard, syncAll(ui.js:keydown、detail.js:paletteResults) | 済(パレットのヒントは detail.js 側。requests-SC.md)
| SC-07 | 06:アクティビティ(対象・表示)、モック「履歴の実行者『同期』」 | アクティビティの行 | 実行者の表示「GitHub」を足す(自動リンク・レビュー・タイトル・状態の変更)。「同期」の行に、同期の修復の新しい文(DA-19〜DA-22)を出す。件数の絞り込みの「同期」に含める | screens.js:PageActivity, actKind(data.js:githubChange) | 済
| SC-08 | 06:受信箱(空のとき)、06:上書きの通知、06:同期の修復 | 受信箱 | タブ「同期の修復」の空の補足を「同期のときに直したもの(番号の重なり、削除された親、親の循環、移動した親に合わせて移した子など)がここに出ます。」にする。プロジェクト別のルールの「上書き」は項目名を「{プロジェクトの ID} のルール」、本文だけで戻すボタンを出さない。関連・リンクの「上書き」も本文だけ | screens.js:PageInbox, InboxRow, restoreMineBlock | 済
| SC-09 | 06:通知(自動化)、モック「自動化が制約で止まった」 | 受信箱の「自動化」の行 | 使わないステータスで止まったときも「「{ルール名}」が制約で止まりました」で出す | screens.js:InboxRow(data.js:runAutomations) | 済
| SC-10 | 04:ワークスペースの概要とプロジェクトの一覧、03:接頭辞 | ワークスペースの概要・プロジェクトの一覧 | 概要の中の ID 表示を接頭辞つきに統一する。プロジェクトの移動の確認の「{接頭辞 または #} の番号」(行 603)を「{接頭辞} の番号」にする | screens.js:WorkspaceOverview, A.moveProject(行 603) | 済
| SC-12 | 02:リポジトリの追加と削除 | プロジェクトの概要のリポジトリ欄 | 追加と削除の部品の文言・アクティビティを仕様に合わせる(EX-11 と同じ関数を呼ぶ) | screens.js:ProjectOverview(extras.js:repoField) | 済

## detail.js

| 番号 | 仕様の箇所 | 画面と部品 | 直す内容 | 主ファイル:関数(関わるファイル) |
|---|---|---|---|---|
| DE-01 | 02:作業時間、モック「作業時間の見出しの『見積もり』」(Q3) | 詳細パネルの「作業時間」の見出し | 「 · 見積もり N pt」を「 · 見積もり {N} 時間」「 · {N} pt」にする。見積もりはポイントと見積もり時間の総称 | detail.js:Peek(行 58)(data.js:estimateOf) | | 済
| DE-02 | 02:プロパティの入力と保存、02:入力の検査 | 詳細パネルのプロパティ | 「見積もり時間」の入力欄を足す。0.25 時間刻み。「見積もり時間は 0 以上の数で入力してください」「見積もり時間は小数第 2 位までで入力してください」 | detail.js:Peek(layouts.js:PropInputRaw) | | 済(詳細パネルは layouts.js の PropInput を使う。0.25 刻みと検査は LA-09、作成ダイアログには detail.js に入れた)
| DE-03 | 02:GitHub、07:表示と更新、モック「PR 用の印のコピー」「リンクの行の部品」 | 詳細パネルの GitHub の欄 | PR の行に、状態が Open・Draft のときレビューとチェックを出す。「PR 用の印をコピー」を置く(押すと内部の ID を含む 1 行をクリップボードへ、閲覧のみの Store でも押せる。内部の ID は画面に出さない) | detail.js:Peek(行 52 のリンクの行)(screens.js:prChip) | | 済(印は `hedgehug:{内部 ID}`。レビュー・チェックは `l.review`・`l.checks` を読む。依頼 R3・R7)
| DE-04 | 07:解除、07:追加、モック「自動リンクの解除」(Q26) | リンクの解除と追加 | 解除すると `unlinked` に記録し、その PR は二度と自動でリンクしない(画面には出さない)。「PR をリンク」で足し直すと記録を消す。「もうリンクしています」の検査は無効の記録で曖昧にしない | detail.js:A.addLink, A.removeLink(extras.js:refreshLinks) | | 済
| DE-05 | 02:作成の入口、02:ダイアログの項目 | 作成ダイアログの項目 | 「テンプレート」は選んだプロジェクトが使えるテンプレートがあるときだけ出す(「なし」を含む)。「プロパティ」は、その種類で使う表示中のすべてのプロパティ(組み込み・追加)を入力できる。ステータスは初期ステータスを初期値にし、プロジェクトが使うすべてのステータスを選べる。優先度は初期優先度。見積もり時間も入力できる | detail.js:CreatePropInput, openCreate, applyTemplate | | 済
| DE-06 | 02:ダイアログの項目(ID の予告)、モック「ID の形」 | 作成ダイアログの「ID の予告」 | 「ID は HH-13 になります。」の 1 つの形にする。「このワークスペースには接頭辞がありません」の案内と `#` の形をなくす | detail.js:CreatePropInput(行 389) | | 済
| DE-07 | 02:アイテムの作成(C)、モック「作成ダイアログの C キー」 | 作成ダイアログの初期値 | ツリーの行にフォーカスがあるときの C は、その行の兄弟として開く(LA-04 から呼ぶ)。それ以外は今のまま | detail.js:openCreate(layouts.js:TreeRow) | | 済(openCreate は pre の wsId・projectId・kind・parentId を受けるので、layouts.js は兄弟の値をそのまま渡せばよい。変更なし)
| DE-08 | 03:ワークスペースの作成、モック「接頭辞」 | 「ワークスペースを追加」のダイアログ | 接頭辞を必須にする(欄のラベルから「任意」を外し、検査に「接頭辞を入力してください」を足す)。説明文は「ID が `WEB-1` のようになります」だけにして、「付けないと #1」「接頭辞のないワークスペースは…」を消す。「この Store には接頭辞のないワークスペースがあります」の断りをなくす。名前と接頭辞の検査は Store の追加ダイアログ(ST-08)と同じ関数を呼ぶ | detail.js:openWsAdd, prefixError(行 565〜574)(settings.js:addStore の入力) | | 済(検査は `newWsError(name, prefix, storeId)` と `prefixError`。依頼 R5)
| DE-09 | 02:コマンドパレット(コマンド) | パレットのコマンド | 「すべての Store を同期」にヒント Shift+Ctrl+S(Mac は Shift+Cmd+S)。ほかのコマンド・入力の絞り込みは今のまま。案内に「Ctrl+Enter で新しいタブ」は出さない(見せない)。ワークスペースの作成の Store の初期値は既定の Store(あれば)を先頭にする | detail.js:paletteResults(cmds)(data.js:defaultStore) | | 済
| DE-10 | 02:コマンドパレット(「このクエリで」) | パレットの新しい群 | 入力が 04 のクエリとして読めるときだけ「このクエリで」を出す(キーを 1 つ以上含み、04 のエラーにならない)。「検索で開く」(ヒントに「{N} 件」。検索をテーブルで開き絞り込みだけを置き換える)、「ビューとして保存…」(同じく開いて「ビューとして保存」を出す)、「{N} 件をまとめて変更…」(開いてすべての行を選び一括変更を出す。一致が 0 件・すべて閲覧のみ・`is:deleted` のときは無効にして理由「一致するアイテムがありません」「閲覧のみの Store です」「削除済みのアイテムは変更できません」)。アイテムの候補は変えない | detail.js:paletteResults, Palette(query.js:parseQuery・queryErrors・runQuery、extras.js:BulkBar) | | 済
| DE-11 | 02:コマンドパレット(「{ID} に対して」) | パレットの新しい群 | 選んでいるアイテム(開いた時点の詳細パネル、なければツリーでフォーカスのある行、なければテーブル・バックログで選択がちょうど 1 件)があるときだけ「{ID} に対して」を出す。「ステータスを変更…」「{種類} を追加」(下に置ける最初の種類、Step には出さない)「詳細を開く」(詳細パネルが開いているアイテムでは出さない)「別のワークスペースへ移動…」(Epic・Story・Task・Bug だけ)。閲覧のみの Store のアイテムでは「詳細を開く」だけ。見出しの頭に Store の点(同じ ID のアイテムが別の Store にもあるとき) | detail.js:paletteResults, Palette(layouts.js・extras.js:選択の取り方) | | 済
| DE-12 | 02:コマンドパレット(「作成」「設定」) | パレットの新しい群 | 「作成」(プロジェクトを作成・スプリントを作成・リリースを作成)と「設定」(設定: アカウントとメンバー・Store・削除済み・診断と修復、設定: ワークスペース({名前})、設定: プロジェクト({名前}))の群を足す。スプリント・リリースの作成とワークスペース・プロジェクトの設定は、その画面を開いているときだけ出す。書き込めるワークスペースがないときの扱いは「新規作成」と同じ。診断は実行しない | detail.js:paletteResults(planning.js・settings.js を開く go の呼び出し) | | 済
| DE-13 | 02:コマンドパレット(「最近開いたもの」)、01:Store で共有するものと、マシンごとに持つもの | パレットの新しい群 | 入力が空のとき、最近開いたアイテムを新しい順に最大 8 件出す(ヒントにプロジェクト名)。記録するのは詳細パネルを開いたとき(行・パレット・リンクのどれからでも)。削除済みのアイテムと非表示の Store のアイテムは出さない(復元・再表示で戻る)。完全に削除したもの、Store をこのマシンから削除したものは覚えている一覧から外す。旧 ID でなくいまの ID で出す。開くたびに最新を読む | detail.js:paletteResults(記録の入口は ui.js:A.peek と、パレット・リンクから UI.peek を直接入れる箇所。data.js:UI.recent・save・load、settings.js:removeForGood) | | 済
| DE-14 | 02:コマンドパレット(「ステータスを変更…」)、03:ステータスを変えるキー操作とパレット | パレットのサブ一覧 | 「ステータスを変更…」を押すと、パレットの中でそのアイテムのプロジェクトが使うステータスの一覧に切り替える。各項目に分類を添え、いまのステータスにチェック(使わないものならチェックつきで 1 項目だけ残し「このプロジェクトでは使わない」)。遷移の制約で移れない項目は無効にして理由をヒントで出す。選ぶと変更してパレットを閉じ、自動化のきっかけになる。Esc はパレットごと閉じる | detail.js:paletteResults, Palette, paletteKey(layouts.js:setItemProp、data.js:checkStatusChange・runAutomations) | | 済
| DE-16 | 02:別のワークスペースと別の Store への移動(確認ダイアログ)、モック「移動のステータスの置き換え」 | アイテムの「別のワークスペースへ移動…」のダイアログ | 確認の中に「ステータスの置き換え」を 1 行ずつ出す(移動先にない、移すアイテムが使っているステータス。セレクトで移動先のステータスを選ぶ。置き換えるものがなければ欄を出さない)。プロジェクト専用の自動化ルールが使っていれば選んだステータスに書き換える。「ステータスが空になる」旨の文を消す | detail.js:A.moveItem(extras.js:wsMoveTargets・mapValuesTo・relocate・moveBlocked) | | 済: 「別のワークスペースへ移動…」のダイアログは extras.js:M.moveAcross で、同じ部品(statusRows・statusReplaceHtml・statusMapOf)を使う。次のとおり。「別のワークスペースへ移動…」のダイアログは extras.js:M.moveAcross にあり、detail.js の担当外(依頼 R1)。detail.js の分(同じワークスペース内の移動の確認 `askMoveStatuses`・`M.moveStatus`)は済(layouts.js の呼び出しは依頼 R2)
| DE-17 | 02:履歴、06:アクティビティ | 詳細パネルの履歴 | 実行者「GitHub」「同期」の行に、自動リンク・レビューの状態・タイトル・状態の変更と、「スプリントを変更」「リリースを変更」(同期の修復・完全な削除)の文を出す | detail.js:HistoryList(data.js:githubChange) | | 済
| DE-18 | 02:複製 | 複製 | 複製しないものに、解除の記録を含む GitHub リンクを足す | detail.js:A.duplicateItem | | 済
| DE-19 | 02:コマンドパレット(旧 ID・接頭辞) | パレットの ID 検索 | 旧い接頭辞で見つかったときのヒント「旧 ID」を残し、`#` の形(接頭辞なし)の旧 ID の分岐をなくす | detail.js:paletteResults(data.js:oldCodes) | | 済(`#` の形の分岐は残っていなかった。旧い接頭辞のヒント「旧 ID」はそのまま)

## planning.js

| 番号 | 仕様の箇所 | 画面と部品 | 直す内容 | 主ファイル:関数(関わるファイル) |
|---|---|---|---|---|
| PL-01 | 05:スプリントの画面、05:実行中のスプリントのボード(Q5)、モック「実行中のスプリントのボード」 | スプリントのカードのアイテムの欄 | 実行中のカードのアイテムの欄に「リスト」「ボード」の切り替えを置く。初めはボード、選んだ方はこのマシンにワークスペースごとに残す。ボードはカンバンと同じ規則で動く(初期の列は「ステータス」、スイムレーン「なし」、中止も隠さない)。使わないステータスの列へのドロップは断る。完了と計画中のカードはリストだけ | planning.js:SprintCard(layouts.js:KanbanLayout・D.kanban、data.js:UI の保存) | | 済(SprintBoard。layouts.js の KanbanLayout・LayoutControls・FilterBox を `sb:{ws}` の状態で使う。使わないステータスへのドロップの断りは D.kanban 側)
| PL-02 | 05:スプリントの画面(途中で追加・バーンダウン) | 実行中のカード | 「途中で追加 +{N} {単位}」(0 のとき・表示する日が 0 日のあいだは出さない)と、小さいバーンダウン(「理想」の点線と「残り」。単位は選んだ単位。表示する日が 0 日のあいだは「開始日({日付})から表示します」)を足す。「表で見る」を付ける | planning.js:SprintCard(同じファイルの burndown の計算 sprintScopeAt・statusAt・lineChart) | | 済
| PL-03 | 05:見積もりと実績、モック「見積もりと実績の見積もり」(Q4) | レポートの「見積もりと実績」 | 表を 1 つにして、列をアイテム・ポイント・見積もり時間・作業時間・差・時間/pt にする。差の列と合計の行を持つ。超過は符号と警告の色。行は、ポイントか見積もり時間があり作業時間が 1 件以上ある作業項目だけ | planning.js:ReportsPage(行 392 付近)(data.js:estimateOf・sumHours) | | 済
| PL-04 | 05:見積もりと実績(対象)、モック「見積もりと実績の対象の絞り込み」 | レポートの「見積もりと実績」の入力 | 「対象」のセレクトで、実行中と完了のスプリントに絞れる(いま入っているアイテムで判定。選択肢はバーンダウンとそろえる)。選択は保存する | planning.js:ReportsPage(data.js:isScreenKey の `rep:` に含まれる) | | 済(保存するキーは rep:{ws} の estSprint)
| PL-05 | 05:累積フロー図、モック「累積フロー図の期間」「累積フロー図のステータス別」 | レポートの累積フロー図 | 開始と終了を指定できる(既定は直近 21 日、終了は今日以前、保存しない。サイクルタイム・リードタイムには効かない)。見出しの「直近 21 日・件数」を指定した期間に変える。「分類」と「ステータス」の切り替えを足す(ステータス別は分類ごとにまとめて積み、ステータスの色。一覧にないステータスは「(削除された値)」)。期間を指定しない図は日付が変わると更新する | planning.js:ReportsPage, areaChart(行 388) | | 済(期間は保存せず、画面を離れると戻る。層の分けは rep:{ws} の cfdBy)
| PL-06 | 05:サイクルタイムとリードタイム、モック「P85」 | レポートのサイクルタイム・リードタイム | 中央値の下に「85% は {値} 日以内」を出す。線形補間で求める(中央値の定義と同じ流儀) | planning.js:ReportsPage, median の隣に新しい関数 | | 済
| PL-07 | 05:表で見る、モック「『表で見る』」 | バーンダウン・ベロシティ・累積フロー図・スプリントのカードの小さい図 | 図ごとに折りたたみの「表で見る」を付ける。列は図に描くもの: バーンダウン(日付・総量・残り・理想。見出しに単位。値のない日の残りは「—」)、ベロシティ(スプリント・計画・完了)、累積フロー図(日付と層ごとの列) | planning.js:ReportsPage, SprintCard, chartFrame(lineChart・barChart・areaChart) | | 済(viewTable)
| PL-08 | 05:削除(スプリント・リリース)、05:存在しないスプリントとリリースの参照(Q13) | スプリント・リリースの削除 | 削除したとき、アイテムのスプリント・リリースの参照を消さない(outIds にだけ記録していた参照外しをやめる)。画面ではバックログ・なしとして扱い、復元すれば戻る | planning.js:A.deleteSprint(行 164)、A.deleteRelease(行 269)(settings.js:restoreBlock・fixAfterRestore) | | 済(復元側の合わせは requests-PL.md の 1)
| PL-09 | 05:バックログ | バックログの対象 | 削除済みのスプリントを指すアイテムも、バックログの枠に入れる(スプリントの参照が見つからないものは「バックログ」扱い) | planning.js:boxItems, plannedSprints(data.js:sprintOf) | | 済(boxItems は sprintOf が null のアイテムを入れるので、削除済みスプリントを指すものも入る。確認のみ)

## settings.js

| 番号 | 仕様の箇所 | 画面と部品 | 直す内容 | 主ファイル:関数(関わるファイル) |
|---|---|---|---|---|
| ST-01 | 08:一般、01:キーボード | 設定「一般」のキーボード一覧 | 4 件に Shift+Ctrl+S(すべて同期)を足して 5 件にし、見出し「ツリー」のもとに 04 のツリーのキー操作(↑↓←→・Enter・Space・S・K・C・Alt+↑↓・Alt+Shift+↑↓・メニューキー・Shift+F10。Tab・Shift+Tab はフォーカス移動と書く)を並べる | settings.js:SettingsGeneral(行 37) | 済(5 件+「ツリー」。表は 04 のとおり Ctrl+Enter も入れて 15 行)
| ST-02 | 08:一覧、08:既定の Store | 設定「Store」の枠の見出しとボタン | 見出しに「既定」の印(既定の Store だけ)。ボタンに「既定にする」(既定でない Store だけ。閲覧のみの Store では無効にして「閲覧のみの Store は作成先にならないため」)。書き込める Store を追加して既定がなければ自動で既定にする | settings.js:SettingsStores, 新しい A.setDefaultStore(data.js:defaultStore) | 済(A.setDefault。自動で既定になるのは addStore)
| ST-03 | 08:一覧(保存先)、08:保存先の表示 | 設定「Store」の枠 | 「保存先」の行を足す(絶対パス・等幅・編集不可。右に「コピー」。押すとクリップボードへ、トースト「保存先のパスをコピーしました」。閲覧のみ・非表示でも使える) | settings.js:SettingsStores(data.js:Store の path) | 済(A.copyPath)
| ST-04 | 08:ミラー、モック「ミラー」(Q22) | 設定「Store」の枠 | 「ミラー」の見出しと「追加」。ミラーごとに 1 行(URL、状態「最終送信 {相対時間}」「未送信」、閲覧のみの Store は「閲覧のみのため送りません」、「削除」)。remote がない Store は「追加」を無効にして「remote を入れると使えます」。追加は github.com のリポジトリだけ。断り: 「この remote はすでに Store「{名前}」として追加されています」「このミラーはすでに追加されています」。変えても保存にもアクティビティにも残さない | settings.js:SettingsStores, 新しい A.addMirror・A.delMirror(extras.js:remoteKey・remoteRepo・storeByRemote) | 済(A.mirrorAddOpen・C.mirrorAdd・A.mirrorDel。remote を空にするとミラーも外れ、トーストに「ミラー N 件も外しました」。remote の変更が自分のミラーと同じなら断る)
| ST-05 | 08:一覧(ワークスペース)、03:接頭辞 | 設定「Store」の枠のワークスペースの行 | 行は名前(押すとそのワークスペースの設定)、接頭辞、「N プロジェクト · M アイテム」。接頭辞がないときの分岐(行 241)をなくす | settings.js:SettingsStores | 済
| ST-06 | 08:一覧(同期の状況)、06:同期の状態の表示 | 設定「Store」の同期の状況 | 「未送信 N 件 · 最終同期 …」(最終同期がなければ「なし」)。remote があってアカウントが「なし」なら、同期を試していなくても「同期エラー: アカウントが設定されていません」。直前の同期が失敗していれば「同期エラー: {理由}」。remote がなければ「remote がないため同期しません」。remote があれば「今すぐ同期」 | settings.js:syncState, SettingsStores(screens.js:syncReason・syncErrorOf) | 済(土台の syncErrorOf・syncReason で仕様どおりだったため、syncState は変えていない)
| ST-07 | 08:このマシンから削除、モック「コピーを残してこのマシンから削除」 | 「このマシンから削除」の確認 | 「このマシンのコピーを残す」(初期オフ)を足す。オンのときはコピーを消さず一覧から外すだけで、失われる旨の警告は出さない。同期中は断る | settings.js:A.removeStore(settings.js:purgeStore) | 済(専用の確認 M.storeRemove。「同期中は断る」はモックの同期が一瞬で終わるため起こせず、足していない。コピーを残した Store は S.keptCopies に残し、次の Store の保存先も避ける。extras.js への依頼は requests-ST.md の 3)
| ST-08 | 08:追加、08:追加(検査)、モック「GitHub 上のリポジトリの作成」「Store 追加のワークスペース」 | 「Store を追加」のダイアログ | 項目を足す。「GitHub にリポジトリを作る」(チェックボックス、初期オフ。remote が入っていてアカウントが「なし」でないときだけ選べる。remote が空なら「remote を入れると選べます」、アカウントが「なし」なら「アカウントを選ぶと使えます」。オンなら private で作る)、「ワークスペースの名前」「ワークスペースの接頭辞」(どちらも必須。remote が空のとき、または「GitHub にリポジトリを作る」がオンのときに出す。接頭辞は入力と同時に大文字、例は「WEB」。検査は DE-08 と同じ)。断り: 「{remote} は空ではありません。「GitHub にリポジトリを作る」をオフにして、clone して追加してください」、作れないときは 07「GitHub との通信に失敗したとき」の表示。作成は 1 秒ほど待たせて見せる(M2) | settings.js:A.addStore(行 282 付近)(detail.js:prefixError、extras.js:ghRepo・ghCommError) | 済(作成は架空の GitHub の GH_REPOS に足し、1 秒待たせ、最初の送信は syncStore で行う。topic の付与は失敗を起こせないため表示なし)
| ST-09 | 08:追加(作成するワークスペース)、03:Store を追加したときのワークスペース、モック「最初のワークスペースの作成」 | Store の追加のあとのワークスペース | 名前「メイン」・接頭辞なしの自動作成をやめる。新しく作る Store と、空のリポジトリを clone する Store(clone のあと空だと分かったときに欄を出す)では、入力された名前と接頭辞で 1 つ作る。hedgehug のデータがあるリポジトリと閲覧のみの Store では作らず、欄も出さない。トーストは「{名前} を追加しました」だけにし、「ワークスペース「メイン」を作りました」を消す。「メンバー {ログイン名} を追加」の別の行は今のとおり | settings.js:addStore(行 318〜335)(extras.js:mountName) | 済(addStore は `ws: { name, prefix }` を受け、`MAIN`・「メイン」は 0 件。トーストは台帳の「だけ」ではなく仕様 08:105・07:357 のとおり「{名前} を追加しました。ワークスペース「{入力した名前}」を作りました」。作らなかったときは「{名前} を追加しました」)
| ST-10 | 08:診断(検出項目)、06:1 つしかないものが 2 つになったとき | 設定「診断と修復」 | 「接頭辞なしの重複」の検出と、接頭辞「WS」を付ける修復をなくす(行 626 の `bare` の処理)。接頭辞の重複は残す(「WS」を付けず、数字を末尾に付ける今の直し方のまま) | settings.js:doctorFindings(行 622〜629)、doctorApply | 済
| ST-11 | 08:診断(検出項目の説明)、06:親が循環したとき、06:親と別のワークスペース・プロジェクトにある子 | 設定「診断と修復」の行の説明 | 「親の循環」「親が別のワークスペース・プロジェクトにある」「存在しないスプリント・リリース」は、「同期で直しきれず残ったもの」だけを検出する旨に説明を変える。存在しないスプリント・リリースに削除済みを含めない。親の別のワークスペースへの修復は、移動先で使えないステータスを同じ分類の既定にする。プロパティのキーの重複の直し方は同期と同じ(末尾に数字) | settings.js:doctorFindings, doctorApply | 済(診断の注意書きに同期で直しきれず残ったものだけと書いた。親が別のプロジェクトの場合は同じ Store 内なら moveItem、別のワークスペースなら relocate のあと使えないステータスを同じ分類の既定にする。プロパティのキーの重複は元から末尾に数字)
| ST-12 | 08:診断(結果の表示)、モック「診断の『検索で開く』」 | 診断の結果 | 結果の下に「検索で開く」を置き、検索を `is:invalid` で開く。診断で直せる件数が 0 の「すべて修復」は無効にする。行に非表示・閲覧のみの印を添える | settings.js:SettingsDoctor(ui.js:go で query 画面、query.js:`is:invalid`) | 済(A.doctorSearch。アイテムの検出が表示中の Store に 1 件以上あるときだけ出す。「すべて修復」の無効化と印は土台のままで仕様どおり)
| ST-13 | 03:接頭辞、モック「接頭辞のないワークスペース」 | ワークスペースの設定「一般」の接頭辞 | 空にできないようにする(説明文の「空にすると #…」を消す。プレースホルダー「なし」を外す。空で変更すると「接頭辞を入力してください」)。確認ダイアログは `HH → WEB` の形で、旧が空の `#1 → WEB-1` の形をなくす。記録は「接頭辞を A から B に変更」、トースト「接頭辞を変えました」 | settings.js:WsGeneral(行 775〜776)、changePrefix の呼び出し(行 790)、wsPrefixInput(detail.js:prefixError) | 済
| ST-14 | 03:別の Store へ移動、03:削除 | ワークスペースの別 Store への移動 | 「接頭辞のないワークスペースがある」ときの断りをなくす(行 796 の `err` の組み立て)。同じ接頭辞の断りは残す | settings.js:A.wsMove(行 796)(detail.js:prefixError) | 済
| ST-15 | 08:復元の単位 | ワークスペースの復元 | 「接頭辞のないワークスペースがあります。先にそのワークスペースの接頭辞を変更してください」の断りをなくす。同じ接頭辞の断りは残す | settings.js:restoreBlock(行 463)、fixAfterRestore | 済
| ST-16 | 03:プロパティ(組み込みの 8 種・予約語) | ワークスペースの設定「プロパティ」 | 組み込みの表を 8 行にする(見積もり時間・キー `estimate`・型「時間」)。プロパティのキーの予約語を 34 個にする(`category`・`estimate`・`group`・`mentions`・`created`・`updated`・関連の 9 個を足す) | settings.js:SchemaProps, propKeyOf | 済(確認のみ。土台で 8 行・型「時間」・予約語 34 個(QUERY_KEYS を paOk が使う)になっていたため触っていない)
| ST-17 | 03:値の削除(ステータス)、03:変更が及ぶ範囲 | ステータスの削除 | 代わりのステータスへの置き換えで、ワークフロー・自動化・テンプレートに加えて、プロジェクト別のルール(使うステータス・遷移の制約・専用ルール)も追従する。件数の表示にも含める | settings.js:ValuesEditor, usageOf, followWip(data.js:プロジェクトの rules) | 済(vdOk で、使わないステータス・遷移の制約・専用ルール(通知はプロジェクト名なし)も追従。ダイアログに「(プロジェクト別のルールを含む)」。アイテムの件数の表示はアイテム数のまま)
| ST-18 | 03:プロジェクト別のルール(編集の部品) | ワークフロー・テンプレート・自動化の編集 | ワークスペースの設定とプロジェクトの設定タブ(SC-02)で同じ部品を使えるようにする(範囲を引数にして、対象を w と p.rules で切り替える) | settings.js:WorkflowEditor, TemplatesEditor, AutomationsEditor(screens.js:ProjectSettings) | 済(`(w, p)` で区画ごと返す。呼び方は requests-ST.md の 1。テンプレートの削除・種類の変更、ルールの削除は、各プロジェクトの使うテンプレート・種類ごとの既定・動かさない指定からも外す)
| ST-19 | 03:きっかけとすること、03:自動化の条件、03:制約で止まったとき | 自動化のルールの編集 | きっかけを 4 つから 6 つにする(「レビューが依頼されたとき」「リンクした Issue が閉じたとき」を足す。説明はモックからの変更点の表の文言)。「リンクした PR がマージされたとき」のときだけ、チェックボックス「リンク中の PR がすべてマージ済みのときだけ」(初期オフ)。きっかけとすることが同じステータスのときは「きっかけと同じステータスには変更できません」。ルールの文(ruleText)も合わせる | settings.js:AutomationsEditor, ruleText(data.js:runAutomations) | 済(6 つのきっかけ、条件のチェックは `trigger.allMerged`。一覧の文は条件オンで「リンクした PR がすべてマージされたとき」。ワークスペースの一覧に専用ルールも持ち主のプロジェクトの ID つきで並ぶ)
| ST-20 | 08:完全に削除(確認)、08:削除済み、モック「プロジェクト別のルールと削除済み」 | 設定「削除済み」の完全に削除の確認 | プロジェクトの行に専用ルールがあるとき、「元に戻せません。」の前に「専用のルール {M} 件も削除します。」を入れる。完全に削除するとプロジェクト別のルールを同じ保存で取り除く。論理削除では残し、復元で戻る | settings.js:removeForGood, A.purgeRow, purgeStore, trashRows, restoreBlock | 済(プロジェクトを消すとプロジェクトの持つ rules ごと消える。論理削除では p.rules が残る)
| ST-21 | 08:スプリント・リリースの完全な削除、モック「スプリント・リリースの完全な削除」 | 完全に削除 | 完全に削除するスプリント・リリースを指すアイテムの参照を、同じ保存で外す。アイテムの履歴に「スプリントを変更」「リリースを変更」を残す。確認の本文の「この下にある削除済みの {N} 件({種類名の一覧})も一緒に…」を 08 の例の形にする(アイテムの行は「(Story 2・Step 1)」のように種類で並べる) | settings.js:removeForGood, purgeStore, underText(planning.js:A.deleteSprint) | 済(参照の取り外しと履歴は removeForGood。確認の本文は土台で 08 の例の形になっていた)
| ST-22 | 08:一覧(削除済み)、04:削除済みを探す | 設定「削除済み」 | 検索の `is:deleted` から行を押して来たとき、その Store の枠でその行を選んだ状態にして、見える位置までスクロールする | settings.js:SettingsTrash(layouts.js:LA-14 から状態を渡す) | 済(受け側。経路は `go({ name: 'settings', page: 'trash', focus: アイテムの内部 id })`。layouts.js への依頼は requests-ST.md の 2)
| ST-23 | 08:メンバー(あなたの入口)、01:「あなた」の決まり方 | 「あなた」をメンバーに載せる入口 | 「フォルダから探す」の取り込みも入口に加える(Store が 1 つもないときは行にログイン名の入力欄)。載せ方は今のとおり(別の行「メンバー {ログイン名} を追加」) | settings.js:addStore(extras.js:EX-02) | 済(「フォルダから探す」のボタンを足し、addStore が `verb: '取り込み'`・`ws: null` を受ける。メンバーの載せ方は今のまま)

## extras.js

| 番号 | 仕様の箇所 | 画面と部品 | 直す内容 | 主ファイル:関数(関わるファイル) |
|---|---|---|---|---|
| EX-01 | 08:初回起動 | 初回起動の画面 | 手順 2 の文を「新しく作成するか、GitHub 上のチームの Store を検索するか、このマシンにある Store のコピーを探します。remote なしで作成すると、このマシンの中だけで使えます。」にする。ボタンは「新しい Store を作成」(ダイアログでログイン名の確認・変更と最初のワークスペースの名前・接頭辞を入力)、「GitHub から検索」(アカウントがなければ無効にして「GitHub のアカウントを登録すると使えます」)、「フォルダから探す」(アカウントがなくても無効にしない)の 3 つ | extras.js:Onboarding(settings.js:A.addStore) | 済(アカウントがないとき「GitHub から検索」は無効のうえ、「GitHub のアカウントを登録すると使えます」を横に添えた)
| EX-02 | 08:フォルダから探す、モック「フォルダから探す」(Q21) | 「フォルダから探す」のダイアログ(新規) | このマシンの Store のコピーを探して取り込むダイアログを作る。架空のコピーを数件持たせる(パス、origin が github.com のもの、origin がないもの「なし(このマシンだけ)」、origin が github.com でないもの=一覧に出さない)。行の部品は、パス(等幅)・remote・名前の入力欄(初期値はフォルダ名から末尾の `.git` と `-tasks` を除いて整えたもの)と「取り込む」・「追加済み」(灰。remote が追加済みの Store と同じ)・「取り込みました」(灰。ダイアログは開いたまま)。Store が 1 つもないときはログイン名の入力欄も出す。取り込みは 1 秒ほど待たせる(M2)。取り込んだ Store に「あなた」をメンバーとして載せる | extras.js:新しい discoverFolders、A.discoverFolders、A.importFolder(settings.js:addStore) | 済(仕様 08 の形に合わせ、フォルダの入力欄と「探す」から探す形にした。架空のコピーは FOLDER_COPIES の 5 件と読めないフォルダ 1 件で、`S.keptCopies` のコピーも出す。仮置き: 検索できるフォルダは架空のパスだけで、それ以外は「フォルダを開けません」。取り込みの `addStore` には `verb: '取り込み'`・`ws: null` を渡す)
| EX-03 | 07:GitHub から Store を検索(マウント)、モック「Store の検索でマウントしたときのワークスペース」 | GitHub 検索の「マウント」 | clone のあと空で書き込める Store になるときだけ、名前と接頭辞を入力させ、その値でワークスペースを作る(「メイン」の自動作成はやめる)。それ以外は作らない | extras.js:mountName、行 613 の addStore 呼び出し(settings.js:addStore) | 済(clone のあと空で書き込める Store のときだけ、行の下に名前・接頭辞の欄と「追加」「キャンセル」を出し、`addStore(... ws)` に渡す。その間は他の行と「閉じる」を止める)
| EX-04 | 04:一括変更(項目・ボタン・説明) | 一括変更のダイアログ | 項目を「ステータス・担当者・優先度・スプリント・リリース・関連システムを追加・関連システムを削除・ポイント・見積もり時間・開始日・終了日」にする(置き換えの「関連システム」はなくす)。ボタンは「{項目}を変更」(関連システムは名前のまま)。説明に「プロジェクトで使わないステータスにはできないアイテムは…」を足し、ポイント・見積もり時間・開始日・終了日には「空にすると値を削除します。」。見積もり時間は 0 以上の数で小数第 2 位まで | extras.js:BulkBar, bulkOptions, bulkApply | 済(項目 11 個。関連システムは追加・削除の 2 項目で、置き換えはなくした)
| EX-05 | 04:一括変更(ステータスの値・分類) | 一括変更のステータスと関連システム | ステータスの一覧から、選んだアイテムのどのプロジェクトでも使わない名前を除く。末尾に「分類で変更」の項目として「未着手」「進行中」「完了」「中止」を足す(分類を選ぶと各アイテムをそのワークスペースのその分類の既定にする。すでにその分類のアイテムは変えない)。関連システムを追加・削除は、ワークスペースの関連システムの名前を合わせた一覧から複数を選ぶボタン(何も選ばないと押せない) | extras.js:bulkOptions, bulkApply(layouts.js の複数選択の部品) | 済(複数選択は、詳細パネルの `openMenu` の keep と同じ形の部品を extras.js に持った。layouts.js の部品は呼んでいない)
| EX-06 | 04:一括変更(確認の本文・残すもの) | 一括変更の確認と結果 | 本文を「{N} 件の{項目}を {値} に変更します。」にする(N は、変えずに残すものと値がすでに同じものを除いた件数。分類は「分類「完了」(各ワークスペースの既定のステータス)に変更します。」、見積もり時間は「N 時間」、関連システムは「{N} 件に関連システム「{値}」を追加します。」「{N} 件から関連システム「{値}」を削除します。」)。変えずに残す理由に「{ID}(値がない)」「{ID}(このプロジェクトでは使わないステータス)」を足す。関連システムはアイテムごとに判定し、選んだ値が 1 つでもないワークスペースのアイテムは残す | extras.js:bulkPlan, bulkRun, leftText | 済
| EX-07 | 02:値の引き直し、02:別のワークスペースと別の Store への移動 | 移動のときの値 | ステータスは、値の名前が完全に一致し移動先のプロジェクトが使うものはそのまま、そうでないものはダイアログで選んだものにする(選ばせられない経路は移動先の同じ分類の既定)。優先度・関連システム・単一選択・複数選択は名前が一致するものに(複数は一致したものだけ残す)、なければ空にしてトーストで知らせる。ポイント・見積もり時間・開始日・終了日などの組み込みは同じキーなのでそのまま残る。別の Store へはログイン名で引き直し、移動先のメンバーにいない人を加える | extras.js:mapValuesTo, relocate, adoptPeople(detail.js:A.moveItem、screens.js:moveProjectTo) | 済(DE-16 の本体、アイテムの「別のワークスペースへ移動」のダイアログの「ステータスの置き換え」も extras.js の M.moveAcross に入れた。`relocate` の末尾に `statusMap` を足した)
| EX-08 | 02:別のワークスペースと別の Store への移動(ルール)、03:移動・削除・復元 | プロジェクトの移動 | プロジェクトのルールを、移動先の同じ名前のステータス・テンプレートに合わせて一緒に移す。移らないものは確認ダイアログに先に一覧で出す。専用の自動化ルールは、ステータスを置き換えてすべて移す。移動先に同じ名前のルールがあれば断る。移動元に残ったルールと専用ルールは外す(通知は作らない) | extras.js:relocate, moveBlocked(screens.js:A.moveProject・moveProjectTo) | 済(`translateRules`・`projectMovePlan`・`moveBlocked(from, to, pj)` を足し、relocate は pj を渡すとルールを移動先に合わせる。screens.js は自前の計画で同じことをしているので、そのまま。requests-EX.md R4)
| EX-09 | 02:別のワークスペースと別の Store への移動(転送記録) | 移動の転送記録 | 同じ Store の中を含め、ID が変わるすべての移動で、移した日時を含む転送記録(formerIds の at)を残す。別の Store への移動は Store ごとに 1 回ずつ保存する。いまの記録の分岐が同じ Store を除いていないか確かめ、除いていれば直す | extras.js:relocate(data.js:wsTransfers・commit の also) | 済(直しなし。relocate は同じ Store の移動を含め、ID が変わるたびに `formerIds` へ at 付きで残し、`also` は別の Store のときだけ移動元を足す。同じ Store を除く分岐はなかった)
| EX-10 | 07:PR の自動リンク、07:表示と更新、モック「PR の自動リンク」「レビューとチェックの表示」 | PR のリンクの取り直し | refreshLinks を広げる。架空の GitHub の PR に、ブランチ名(`hh/` か `hedgehug/` のあとに ID)・タイトルや本文の ID・コメントの ID を持たせ、プロジェクトのリポジトリで見つかった PR を自動でリンクする(旧 ID は読まない、Issue は自動リンクしない、マージされずに閉じた PR は新しくリンクしない、`unlinked` の PR はリンクしない)。レビューの状態とチェックの結果を取り直し、Open・Draft の PR の行に出す。レビューが「なし」から「レビュー待ち」になったら `review_requested`、Issue が Closed になったら `issue_closed`、PR が Merged になったら `pr_merged` を起こす。変更は githubChange で記録する | extras.js:refreshLinks, ghFind, ghState(data.js:githubChange、detail.js:Peek) | 済(架空の GitHub の PR にブランチ・本文・コメント・レビュー・チェックを持たせた。`readGithubStore` が取り直しと自動リンクを行い、サンプルの 2 リポジトリは読み済み=初回扱いにしない。仮置き: サンプルの未リンクの PR 6 件を足した)
| EX-11 | 02:リポジトリの追加と削除、07:自動リンクの読み取りの時期 | プロジェクトのリポジトリ欄 | 追加は `owner/name` の形で入れる(前後の空白は除く)。形式違いは「リポジトリは owner/name の形式で入力してください」、すでにあるとき(大文字小文字を区別しない)は「もう追加されています」。アクティビティは「{タイトル} にリポジトリ {owner/name} を追加」、トーストは出さない。追加したら、すぐ自動リンクの探し方を動かす | extras.js:repoField, ghRepo(screens.js:ProjectOverview) | 済(extras.js 側は `repoAddError`・`searchRepoNow`・`repoSearchError` を足した。呼ぶのは screens.js の A.addRepo と概要、detail.js の linkOk で、requests-EX.md R1〜R3)
| EX-12 | 07:PR の自動リンク(読む場所の規則)、08:非表示と再表示 | 自動リンクの対象 | 架空の GitHub の PR の持ち物を、表示中で書き込める Store だけが探す(非表示と閲覧のみの Store は探さない)。探す関数を 1 つにして、index.html の読み取りの時計(IX-01)とリポジトリ追加から呼ぶ | extras.js:refreshLinks(index.html の読み取りの setInterval) | 済(探す関数は `readGithubStore(storeId)` の 1 つ。非表示と閲覧のみの Store は何もしない)
| EX-13 | 08:サーバーに接続できない画面とトークンのない画面、モック「サーバーに接続できない画面」 | 「サーバーに接続できない」画面(画面の状態) | 画面全体の置き換えをやめ、開いていた画面の上に重ねて出し、入力を消さない。書きかけがあれば、投稿・保存・キャンセルのあとで再読み込みする(「画面の状態」の切り替えとして今あるものを直すだけで、状態は増やさない) | extras.js:SimOverlay, MockStates(screens.js:Page、ui.js:render) | 済(直しなし。重ねて出す形と、再接続で元の画面と入力が残る形はすでにそのとおりで、ui.js:render が書きかけの欄を戻す)
| EX-14 | 08:追加、07:アカウント | 追加の検査 | remote の重複・ミラーの重複の比べ方(「追加」と同じ)を、ミラーの追加(ST-04)からも呼べるようにする | extras.js:remoteKey, remoteRepo, storeByRemote | 済(直しなし。3 つはすでに全体から呼べる関数で、settings.js の mirrorError が使っている)

## index.html(インラインのスクリプトと CSS)

| 番号 | 仕様の箇所 | 画面と部品 | 直す内容 | 主ファイル:関数(関わるファイル) |
|---|---|---|---|---|
| IX-01 | 07:自動リンクの読み取りの時期、07:GitHub を読む間隔 | 読み取りの時計 | 「GitHub を読む間隔」の時計に、自動リンクの探し方(EX-12)を足す。アプリを開いた直後とオンラインに戻ったときも、前回から間隔が過ぎていれば読む。自動で読むときは、06 の自動の同期を待たせる条件(uiBusy)のあいだは始めない。書き込める表示中の Store だけ | index.html:setInterval の読み取りブロック、lastRead(extras.js:refreshLinks) | 済(読む処理は readGithub()。1 つの Store の読み取りは extras.js の `readGithubStore(storeId)` が定義されていればそれを呼び、なければ従来どおり refreshLinks を各アイテムに呼ぶ)
| IX-02 | 01:画面の移り方と、共通の表示、06:同期の状態の表示 | 新しい部品の見た目 | CSS を足す: ツリーの行のフォーカス表示、レビュー・チェックの印、ボードの切り替え、「使わない」の列の印、パレットの新しい群の見出し、ミラーの行、保存先の行、「フォルダから探す」の行、灰色の注記(`group:`・`sort:` が効かないとき) | index.html:style(CSS) | 済

## 見せないもの

モックで見せられない、または見せない方針のもの。モックのコードは直さない。

| 番号 | 仕様の箇所 | 画面と部品 | 見せない理由 |
|---|---|---|---|
| N-01 | 01:画面のアドレス、01:新しいタブで開く、02:コマンドパレット(Ctrl+Enter)、04:ツリーのキー操作(Ctrl+Enter) | アドレス、ブラウザの戻る・進む・再読み込み、修飾キーつきのクリック、中クリック、パレットと行の Ctrl+Enter、案内「Ctrl+Enter で新しいタブ」 | 埋め込みで開くモックにはアドレスバーも新しいタブもなく、トークンの付いたアドレスの実体もない。Ctrl+Enter は Enter と同じに働かせ、案内の文は出さない(今ある「旧 ID のアドレスを開いたときのトースト」は残す) |
| N-02 | 01:範囲、06:画面への反映(反映するもの・反映を待つとき・反映で変わる画面・他の書き手との重なり) | 別のブラウザのタブ・CLI など別のプログラムが保存した変更の自動の反映、書きかけのあいだの反映待ち、開いている画面がなくなったときのマイタスクへの移動、レポートの「1 秒ほどで全タブに」 | 保存の実体(Store・別プログラム・複数タブ)がモックにない。画面に出る結果は、既存の同期の場面(simulateRemote)が代わりに見せる |
| N-03 | 06:同期のエラー、08:remote の履歴が書き換えられたとき、08:付け替えと書き換えの区別 | 「remote の履歴が書き換えられています」のエラー、設定「Store」の案内とボタン「remote の履歴にそろえる…」、8 行の確認ダイアログ(「元に戻せません。」)、「別の Store になっています」の説明 | 強制 push の検出は remote の実体がないと起きない。起こす切り替え(画面の状態)は足さない方針 |
| N-04 | 06:ミラーへの送信、08:ミラー(失敗) | ミラーへの送信の失敗(ミラーの行の「送れませんでした: {短い理由}」、手動同期のトースト) | 保存・送信の失敗は M4 と同じく見せない。成功の表示(最終送信)は ST-04・DA-29 で見せる |
| N-05 | 07:PR の自動リンク(読む場所・順序)、07:初回の読み取りと自動化 | ブランチ名の形(`hh/WEB-1-login`)、100 件までのコメント、古い方から読む順、初めて読むリポジトリで自動化を起こさない規則 | 読み方の規則は画面に出ない。結果(リンクが付く・外した PR は付かない・Issue は付かない)だけを EX-10 で見せる。初回の読み取りで自動化を起こさないことは、サンプルの設定上すでに読み済みなので現れない |
| N-06 | 06:他の人の変更の重なり(自動リンク)、01:書き込みの単位(GitHub) | 複数のマシンが同じ変更・同じ自動リンクを保存したときに 1 行・1 本にまとめる | 複数のマシンがモックにない |
| N-07 | 08:GitHub 上のリポジトリの作成 | 作るリポジトリの topic「hedgehug-store」、最初の送信を通常の同期で行うこと、送信が失敗しても Store を残して同期エラーにすること | 画面に出ない、または失敗の見せ方(M4) |
| N-08 | 08:非表示と再表示(PR の自動リンク・反映) | 非表示のあいだ PR の自動リンクを探さない、画面の反映が非表示の Store を更新しない | 画面に出る結果が「出ない」ことだけで、見せる画面がない。EX-12 で探す対象から外す形では入る |
| N-09 | 03:自動化の動く範囲(同期・診断のとき動かさない)、03:同期の修復での制約と自動化 | 同期・診断・同期の修復で自動化が動かない | 画面に出る結果が「何も起きない」ことだけ。DA-12 で呼び出しの経路を直すので、見え方は変わらない |
| N-10 | 08:診断(同期で直る項目)、03:必須の項目と「今の違反」 | 診断に項目を足さない、「今の違反」を戻さない | 作らない決定で、見せる画面がない |

## 土台で足したもの

data.js・query.js・ui.js・index.html を直したときに足した、データの形とヘルパー。後の担当(layouts.js・screens.js・detail.js・planning.js・settings.js・extras.js)はこれを使う。

### データの形(保存データの版は 7)

| 場所 | 形 | 使う想定のファイル |
|---|---|---|
| ワークスペース `prefix` | いつも空でない(サンプルの個人は `ME`)。`newWorkspace(storeId, name, prefix)` は prefix を必須で受ける(空を補わない) | detail.js(openWsAdd・prefixError)、settings.js(addStore) |
| `S.settings.lastRead` | 前回 GitHub を読んだ時刻(ミリ秒)。このマシンの値 | index.html |
| 組み込みプロパティ `estimate` | スキーマの `{ key: 'estimate', name: '見積もり時間', type: 'hours', builtin: true }`(ポイントと関連システムの間)。アイテムの `props.estimate` は数(時間)か null。`BUILTIN_TYPES.estimate = 'hours'` | detail.js・layouts.js・planning.js・extras.js・settings.js |
| `project.rules` | `{ offStatuses: [使わないステータスの id], transitions: { fromId: [toId] }, templates: [使うテンプレートの id(空はすべて)], kindDefaults: { 種類: テンプレート id か 'none'(キーがなければワークスペースと同じ) }, offAutomations: [動かさないワークスペースのルールの id], automations: [専用ルール { id, name, on, trigger, action }] }`。`newRules()` で空の形が作れる。プロジェクトの作成(detail.js:npOk、data.js の同期の場面)では `rules` を持たせなくても、読む側(rulesOf)は空として読む。書くときは `ensureRules(p)` | screens.js・layouts.js・detail.js・extras.js・planning.js・settings.js |
| 自動化のきっかけ | `trigger.type` は `pr_opened`・`review_requested`・`pr_merged`・`issue_closed`・`status`(`trigger.status`)・`steps_done`。`pr_merged` の条件は `trigger.allMerged`(真なら、リンク中の PR(マージされずに閉じたものを除く)が 1 件以上あり、すべてマージ済みのときだけ動く) | settings.js(AutomationsEditor・TRIGGERS)、extras.js(refreshLinks) |
| リンク `review`・`checks`・`auto` | `review`: `none`・`requested`・`approved`・`changes`(表示名は `REVIEW_LABEL`)。`checks`: `success`・`failure`・`pending`・`none`。`auto`: 真なら GitHub が自動でリンクした。状態の表示名は `LINK_STATE_LABEL`(open→Open など)。サンプルの PR 412 は承認・成功、418 はレビュー待ち・実行中・自動、420 はなし・成功・自動(Draft)、425 は修正依頼・失敗 | detail.js(リンクの行)、extras.js(refreshLinks・GH_REPOS) |
| アイテム `unlinked` | `[{ repo, number }]`。解除した PR の記録。作成時は `[]`。サンプルは HH の「WIP 制限を列ごとに設定する」が example-org/hedgehug-web#421 を持つ | extras.js(自動リンク)、detail.js(解除・足し直し) |
| Store `path`・`mirrors`・`isDefault` | `path`: 保存先の絶対パス。`mirrors`: `[{ id, url, last }]`(`last` は ISO か null=未送信)。`isDefault`: 既定の Store(1 つだけ)。サンプルは team が既定でミラー 1 件、upstream(閲覧のみ)がミラー 1 件(未送信) | settings.js・detail.js |
| `UI.recent` | 詳細パネルを開いたアイテムの内部 id の新しい順(最大 8)。このマシンの保存(`-local`)に入る | detail.js(paletteResults) |
| 履歴の行 | `sync: true` を持つ行は「同期」の実行者(by は null)。`github: true` は GitHub。`auto: true` は自動化。`moved: true` は移動でのステータスの置き換え | detail.js(HistoryList)、screens.js(PageActivity) |

### data.js の関数

| 関数 | 引数 → 返り値 | 使う想定 |
|---|---|---|
| `itemCode(it)`・`projectCode(p)` | いつも `HH-12`・`HH-P1` | 全画面 |
| `estimateOf(it)` | → 時間(数)か null | detail.js・layouts.js・planning.js・query.js・extras.js |
| `createdAt(it)`・`updatedAt(it)` | → ISO の時刻。updatedAt は人か自動化の履歴の行のうち最も新しいもの(GitHub・同期の行は使わない) | layouts.js(テーブルの列・グループ)、query.js |
| `localDateOf(iso)` | → このマシンの日付 `YYYY-MM-DD`(空は '') | layouts.js(グループの見出し) |
| `rulesOf(projectId)`・`ensureRules(p)`・`newRules()` | プロジェクトのルール(読む・書く・空の形) | settings 系のプロジェクト設定(screens.js) |
| `usableStatuses(projectId)` | → プロジェクトが使うステータスの配列(ワークスペースの順。初期ステータスと分類の既定は外せない) | layouts.js(カンバンの列・メニュー)、detail.js(作成ダイアログ・メニュー)、extras.js(一括変更) |
| `statusUsable(it, statusId)` | → 真偽 | layouts.js・detail.js・extras.js |
| `projectTransitions(p)` | → `{ fromId: [toId] }`(使わないステータスを除く。残りのない行は入らない) | screens.js(プロジェクトの設定) |
| `allowedTargets(it, from?)` | → 両方の制約が許す To の id 配列(使わないものを除く)か、制限がなければ null | layouts.js(メニューの無効化と理由) |
| `checkStatusChange(it, to)` | → 断る理由か null。文言: 「このプロジェクトでは「{名}」を使いません」「ワークフローの制約: A から B には変更できません(X・Y のみ)」「…(変更できる先がありません)」 | layouts.js・extras.js |
| `usableTemplates(projectId, kind?)`・`defaultTemplateFor(projectId, kind)` | → テンプレートの配列(プロジェクトの限定に従う)・種類の既定のテンプレートか null | detail.js(作成ダイアログ) |
| `automationsFor(wsId, projectId)` | → `[{ a, own }]`(そのプロジェクトで判定されるルールを順に) | planning.js・settings.js(プロジェクトの自動化の区画) |
| `allAutomations(wsId)` | → `[{ a, pj }]`(ワークスペースのルール、続けて各プロジェクトの専用ルール。pj は持ち主のプロジェクト。削除済みのプロジェクトのものも入る) | settings.js(ルールの一覧) |
| `runAutomations(it, ev)` | ev = `{ type, to?, link?, prev? }`。`link` は対象のリンク、`prev` は変わる前の値(状態・レビュー)。削除済みのアイテムでは何もしない | extras.js(refreshLinks)、detail.js(リンクの追加) |
| `moveItem(it, { parentId, projectId, statusMap })` | `statusMap`: `{ 旧ステータス id: 新ステータス id }`。プロジェクトが変わるとき、移動先で使わないステータスを map か同じ分類の既定に置き換える。→ `[{ item, from, to }]` | extras.js・layouts.js・detail.js |
| `unusedStatusesFor(items, projectId)` | → `[{ id, name, cat, count }]`(アイテムが使っていて移動先が使わないステータス。「ステータスの置き換え」の行) | extras.js・layouts.js・detail.js(確認ダイアログ) |
| `githubChange(it, link, value, kind = 'state')` | kind: `state`・`review`・`title`・`autolink`。履歴(GitHub)とアクティビティを足す。通知は作らない | extras.js(refreshLinks) |
| `defaultStore()`・`setDefaultStore(id)`・`newStorePath(name)` | 作成先の既定の Store(表示中で書き込めるときだけ。なければ null)・既定を 1 つにする・保存先のパス(同名は -2) | detail.js・settings.js |
| `touchRecent(id)`・`recentItems()`・`pruneRecent()` | 記録・表示できる最近のアイテム(削除済みと非表示の Store を除く)・存在しない id を外す(save で毎回呼ぶ) | detail.js(paletteResults) |
| `SIM_STEPS` | 手動同期の場面の数(17)。13〜16 の場面は 12 までと同じ形で、前提がなければ自分で作る | — |

### query.js

| 名前 | 内容 | 使う想定 |
|---|---|---|
| `QUERY_KEYS` | 34 個の予約語(03) | settings.js |
| `REL_KEYS` | `{ blocked_by: 'blocked by', … }` 関連のキーと一覧の表記の対応 | — |
| `QUERY_GROUPS`・`QUERY_IS` | `group:`・`is:` の値の候補 | layouts.js |
| `runQuery(items, q, range = {})` | `is:deleted` があると、items でなく表示中の Store の削除済みアイテム(`range` の `wsId`・`projectId` に合うもの)を対象にする | layouts.js・screens.js(第 3 引数に画面の範囲を渡す) |
| `queryWantsDeleted(q)` | → 否定でない `is:deleted` があるか | layouts.js(テーブルだけに出し、他のレイアウトは案内) |
| `deletedItems()` | → 設定「削除済み」に行として出るアイテム(表示中の Store) | layouts.js |
| `queryGroupKey(q)` | → 最初の `group:` の最初の値(テーブルが分けられる項目のときだけ。なければ '') | layouts.js(groupKeyOf・laneOf) |
| `termMatches(it, t, ctx)`・`mentionedLogins(it)`・`codeIs(x, v)` | ctx は `is:invalid` などの計算の使い回し(省略可) | — |
| `sortItems(items, key, desc)` | `system`・`estimate`・`created`・`updated` を追加。複数の値は順位の配列で比べる | layouts.js |
| `suggestToken` | `mentions`・`is`・`group`・関連の 9 個のキーも読む。候補は `{ label, value }`(`is`・`group` は値の名前) | layouts.js(SuggestList) |

### ui.js・index.html

- ui.js: `tree-row` の要素にフォーカスがあるとき `treeKey(ev, rowEl)`(layouts.js が定義。キーを使ったら true)を呼ぶ。Tab は渡さない。`role="button"` の Enter・Space の処理は `tree-row` には効かない。Shift+Ctrl+S(Cmd)は `syncAll()`(screens.js)を呼ぶ。`data-ro-ok` を持つ要素の中は閲覧のみでも無効にならない。
- index.html: `readGithub()` が時計から呼ばれる。extras.js が `readGithubStore(storeId)`(1 つの書き込める表示中の Store のリンクの取り直しと PR の自動リンク)を定義すると、それを呼ぶ。
- index.html の CSS(新しい部品の class): `.tree-row[tabindex]:focus-visible`(ツリーの行のフォーカス)、`.pr-chip.draft`、`.rv-chip.requested|approved|changes`、`.ck-chip.success|failure|pending`、`.board-switch`(中のボタンに `.on`)、`.kb-col.unused` と `.unused-mark`(「使わない」)、`.pal-item .pal-hint`、`.mirror-row` と `.mirror-url`・`.mirror-state`(`.warn`)、`.path-row` と `.path-text`、`.find-row` と `.find-path`、`.grey-note`。

### 確認

- `node --check` は 9 本の JS と index.html のインラインスクリプトで通った。
- data.js・query.js(と planning.js・extras.js・settings.js、DOM は空のスタブ)を node で読み、seed() が通ること、検索の新しいキー、使わないステータスの断り、ルールの自動化、移動の置き換え、同期の場面 0〜16 を 2 周が例外なく動くことを確かめた(手元のスクリプト。画面の描画は確かめていない)。
