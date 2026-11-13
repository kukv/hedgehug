# hedgehug トラック B5a(プロジェクト別のルール)実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仕様 03「プロジェクト別のルール」の 4 つ(使うステータス・遷移の制約・テンプレート・自動化)と「ワークスペースの設定に戻す」を、プロジェクトの設定タブで編集でき、ステータスのメニュー・作成ダイアログ・移動・自動化の判定に効くようにする。

**Architecture:** ルールはワークスペースの `log.jsonl` の項目 `project_rules/<プロジェクト>/…` に持ち、新しい domain パッケージ `domain/projectrule` が読み(`FromLog`)、判定(使うステータス・両方が許す遷移・使えるテンプレート)と書く行を持つ。`domain/automation` は `projectrule` を import して、専用ルール(`scope`)・動かさない指定・使わないステータスで止まることを判定する(逆向きの import はしない)。操作は新しい `app/projectrules` に置き、アイテムの操作(`app/items`)と設定の追従(`app/workspaces`)は `projectrule` の関数を呼ぶ。画面には、ワークスペースの詳細(`GET workspace`)に全プロジェクトのルールを載せ、core の関数がプロジェクトごとの選択肢を作る。

**Tech Stack:** Go 1.27.3、huma v2、Svelte 5(`web/app`)、vitest、Playwright。

**Spec:**
- `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/03-workspace-settings.md`「範囲と画面の構成」(サブタブの上の 1 行、表の 55〜61 行)、「プロジェクト別のルール」全節、「値の編集 / 削除」のプロジェクト別のルールの箇条、「テンプレート / 一覧と編集」の削除、「ワークフロー / 遷移の制約」の断りの文言、「自動化 / 実行の規則」「制約で止まったとき」、「変更が及ぶ範囲」の最後の行
- 02「ステータスの変更」(88〜100 行)、「ダイアログの中での連動」の「プロジェクトを変えたとき」(156 行)、「同じワークスペースの中での移動」(209〜218 行)、「プロジェクト」の設定タブ(527〜539 行)
- `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md` 42・58・154・316 行
- `docs/superpowers/plans/2026-11-12-hedgehug-track-b.md`(小計画の表と漏れの表)、`2026-11-13-hedgehug-track-b3.md`(B3-1・B3-2)、`2026-11-13-hedgehug-track-b4.md`(B4-1〜B4-23)、`2026-11-13-hedgehug-track-a3b.md`(A3b-12・A3b-14)、`2026-11-13-hedgehug-track-a4b.md`(「ステータスの置き換え」と Space の未決 8・9)

## Global Constraints

土台とトラック B の計画の Global Constraints をすべて引き継ぐ。加えて次を守る。

- 大きさの上限: Go ファイル 300 行(テスト 600 行)、関数 60 行、認知的複雑度 15、パッケージのテスト以外のファイル 20 個、Svelte 200 行、TS 300 行。`app/workspaces`・`app/apptest`・`app/port`・`infra/gitstore` は 20 個で満杯なので、ファイルを足さない。`app/items` は 19 個なので、足すのは 1 個まで(この計画では `movestatus.go`)。
- 依存の向き: `domain/projectrule` は domain だけを import する。`domain/automation` は `projectrule` を import してよいが、`projectrule` は `automation` を import しない。app の機能どうしは import しない。
- 依存ライブラリは足さない。足すなら `go get <module>@vX.Y.Z`・`pnpm add -E <pkg>@X.Y.Z` のように版を明記し、公開から 7 日以上たった版だけ(Go は `go list -m -json <module>@<ver>` の Time で確かめる)。
- 計画やテストに、トークンらしい値(`ghp_` などで始まる文字列)を書かない。
- 画面の文言・アクティビティの文・断りの文は、仕様 02・03 の文字どおりに書く。
- `api/openapi.yaml` と `web/packages/api/src/generated.ts` は手で直さない。`make generate` と `cd web && pnpm gen` で作り直し、同じコミットに入れる。
- e2e のビルドも CI で lint する。本物の組み立てだけで使う関数は `//go:build !e2e` のファイルに置く。e2e は Playwright の worker ごとに `bin/hedgehug-e2e` を空いた番号で起動する(`web/e2e/fixtures.ts`)。
- 手元の検証は 1 回で足りる(CI でも流れる)。
- PR はこの小計画で 1 本(向き先 develop、ブランチ `feat/track-b5a`)。force push は拒否されるので、載せ直すときは別名のブランチで出す。

## この計画で決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| B5a-1 | B5 を機能で 3 本に分ける。B5a = プロジェクト別のルール(この計画)、B5b = ワークスペースの削除、B5c = ワークスペースの別の Store への移動(A5 のあと。転送の記録を A5 と共有)。プロジェクトを別のワークスペースへ移すときのルールの引き継ぎは A5(プロジェクトの移動と同じ保存。トラック B の計画の漏れの表)。トラック B の計画の表を直す(Task 1) | ユーザーの決定(2026-11-14) |
| B5a-2 | 保存の項目は、`project_rules/<P>/hidden_statuses`(add・remove、値はステータス ID)、`project_rules/<P>/workflow/<From>`(add・remove、値は To のステータス ID)、`project_rules/<P>/templates`(add・remove、テンプレート ID)、`project_rules/<P>/disabled_rules`(add・remove、ルール ID)、`project_rules/<P>/template_default/<種類>`(set。値はテンプレート ID か「なし」の `@none`。null は「ワークスペースと同じ」)。遷移はワークスペースと同じ形にし、保存形式の設計書 316 行の `workflow/<From>><To>` を `workflow/<From>`(値は To)に直す(Task 1) | B3-1(「B5 のプロジェクトの制約も同じ形」)。保存形式 154 行(「From と To の組ごと」の add・remove は、この形でも 1 つの行が 1 つの組)。`@none` は `workspace.StepNone`・`automation.Unassigned` と同じ書き方 |
| B5a-3 | `add`・`remove` の 4 つの項目を、ワークスペースのログの `change.Options.Multi`(`domain/workspace/workspace.go:40`)に足す。`template_default/<種類>` は足さない | 足さないと `Resolve` が 1 つの値として扱い、同期で両方の追加が残らない(06「値を複数持つ項目」) |
| B5a-4 | ルールは `workspace.Settings` に載せず、`projectrule.FromLog(log) Set`(`map[project.ID]Rules`)で読む。`Rules` が空のプロジェクトは `Set` に入れない | B4-2 と同じ理由(`automation` が `workspace` を使い、`automation` は `projectrule` を使うので、`Settings` に載せると輪になる) |
| B5a-5 | ステータスを「使う」の判定は `projectrule.Uses(s, r, id)`: `hidden_statuses` に入っていないか、初期ステータスか、分類ごとの既定のどれかなら使う。どの判定・選択肢・件数もこの関数で決める | 03「使うステータス」2 段落目(外した印は残すが、使うものとして扱う)。これで Step のチェック・ツリーの Space(行き先は分類の既定)・複製・Step の追加・テンプレートでの作成(初期ステータス)は、使わないステータスにならない。A4b の未決 8(Space の行き先)は、断りも行き先の変更も要らない |
| B5a-6 | 遷移の判定は `projectrule.CheckTransition(s, r, from, to)`。from が空・同じ・いまの一覧にないときは通す。許す To = ワークスペースの許す To(行に制限がなければすべて)∩ プロジェクトの許す To(行に使うステータスのチェックがなければすべて)から、使わないステータスと from を除いたもの。プロジェクトの行のチェックは、From と To の両方が使うステータスで、ワークスペースの制約も許す To のものだけ数える(ワークスペースの制約を後から変えて残ったチェックは数えず、画面ではチェックなしの無効のセルにする)。両方の行に制限がなければ通す。断りの文は「ワークフローの制約: A から B には変更できません(X・Y のみ)」、X・Y が空なら「(変更できる先がありません)」。`workspace.CheckTransition` を呼んでいた所(`app/items/setproperty.go`・`domain/automation/run.go`)はこの関数に替える | 03「遷移の制約」の断りの段落、「プロジェクトの遷移の制約」2 段落目。使わないステータスを X・Y から除くのは、ワークスペースだけの制約のときも同じにする(メニューに出ない先を「移れる先」として見せないため。02「ステータスの変更」の {移れる先}) |
| B5a-7 | 使わないステータスへの変更を断るのは `items.SetProperty`(詳細パネル・ツリー・概要のメニュー、Step のチェック。のちのカンバン・セル・一括変更・パレットもここを通る)と `items.Create`(作成ダイアログのステータス)。文言は「このプロジェクトでは「{ステータス}」を使いません」。値の削除の置き換え(`replaceRows`)・「制約を無視して…」(`applyStatus`)・移動は、この判定を通らないことをテストで押さえる | 02 の 98 行、03 の 160 行(断らない経路) |
| B5a-8 | 作成で、選んだテンプレートがそのプロジェクトで使えないときは `port.ErrNotFound`(種類が合わないときと同じ扱い。`createtemplate.go:18`) | 画面は使えるテンプレートしか出さないので、ふつうは起きない(B4-6 と同じ考え方) |
| B5a-9 | 使えるテンプレート = 種類が合い、かつ(プロジェクトの `templates` のうち、いまあるテンプレートが 0 件なら全部、そうでなければその中)。種類ごとの既定 = プロジェクトの `template_default` が `@none` なら「なし」、テンプレート ID ならそれが使えるときはそれ・使えないときは「なし」、未設定ならワークスペースの既定が使えるときはそれ・使えないときは「なし」 | 03「プロジェクトのテンプレート」2・3 段落目。使えない既定は警告なしで「なし」 |
| B5a-10 | 専用ルールは `automation.Rule.Scope project.ID` を持ち、`rules/<ID>/scope` に作成のときだけ書く。編集で `Scope` は変えない。専用ルールのきっかけとすることのステータスは、そのプロジェクトで使うもの(`Uses`)でなければ `ErrNotFound`。編集では、変えたきっかけ・することのステータスだけを確かめる(ステータスの削除の書き換えなどで使わないステータスを指したままのルールも改名できる) | 保存形式 316 行(ワークスペースのルールに `scope`)。03「専用ルール」(選択肢は使うステータス)。B4-6 |
| B5a-11 | `automation.RunAll(s, set, rules, it, evs)` に `projectrule.Set` を足す。専用ルールは `Scope == it.Place.Project` のときだけ、アイテムのプロジェクトの `disabled_rules` に入るルールは対象にしない。することのステータスが使わないものなら止め、`Stopped` を「このプロジェクトでは「{ステータス}」を使いません」にする(遷移の検査より先に見る)。`app/items/automate.go` と `app/githublink/fire.go` の両方が `Set` を渡す。「Step がすべて完了したとき」は親の項目で判定する(いまのまま。親の `Place.Project` を使う) | 03「実行の規則」4 つ目の箇条、「制約で止まったとき」。使わないステータスはメニューに出ないので、遷移より先に断る |
| B5a-12 | 「使うステータス」の操作で、外すのを断るのは: 初期ステータス(「初期ステータスのため外せません」)、分類の既定(「{分類}の既定のため外せません」)、両方(「初期ステータスで、{分類}の既定でもあるため外せません」)、専用ルールが使う(「「{ステータス}」はこのプロジェクトだけのルール「{ルール名}」が使っているため、使わないことにできません。先にルールを変えてください」)。前の 3 つは画面ではチェックを無効にするので、サーバーの断りは同じ文で持つだけ。{分類} は「未着手」「進行中」「完了」「中止」 | 03「使うステータス」2・3 段落目 |
| B5a-13 | プロジェクトの遷移の表で、ワークスペースの制約が許さないセルにチェックを付ける操作は「ワークスペースの制約で変更できません」で断る(画面では無効) | 03「プロジェクトの遷移の制約」1 段落目 |
| B5a-14 | 「ワークスペースの設定に戻す」は区画ごとに 1 回の保存。使うステータス = `hidden_statuses` をすべて remove、ワークフローの制約 = `workflow/*` をすべて remove、テンプレート = `templates` をすべて remove し `template_default/*` を null に set、自動化 = `disabled_rules` をすべて remove し専用ルールをすべて delete。その区画に行がないときは `ErrNotFound`(画面ではボタンが無効)。確認の本文の「このプロジェクトだけのルール {N} 件も削除します。」は N が 1 以上のときだけ続ける | 03「ワークスペースの設定に戻す」。0 件のときに「0 件も削除します」と出さないのは、B3 の「変わるプロジェクトがあるときだけ添える」と同じ扱い |
| B5a-15 | 削除済みのプロジェクトへのルールの操作は `ErrNotFound`。ワークスペースの自動化の一覧では、専用ルールの名前のあとに持ち主のプロジェクトの ID を「HH-P1」と添え、削除済みなら「HH-P1(削除済み)」とする(印はサーバーが一覧の `scopeLabel`・`scopeDeleted` で渡す)。サブタブの上の 1 行は、削除済みのプロジェクトを出さず数えない | 03「移動・削除・復元」最後の箇条、「範囲と画面の構成」4 段落目、「ルールの一覧」(464 行) |
| B5a-16 | 追従: ステータスの削除は、すべてのプロジェクトの `hidden_statuses` から remove し、プロジェクトの遷移は `WorkflowDeleteDrafts` と同じ規則で remove する(専用ルールは B4 の `StatusDeleteDrafts` が、全ルールを対象にすでに書き換える)。テンプレートの削除は、すべてのプロジェクトの `templates` から remove し、そのテンプレートを指す `template_default/*` を null にする。テンプレートの種類の変更も同じ行を書く | 03「値の編集 / 削除」のプロジェクト別のルールの箇条、「プロジェクトのテンプレート」3 段落目 |
| B5a-17 | テンプレートの削除の確認の「{N} 件のプロジェクト別のルールからも削除されます。」「{ID} は、すべてのテンプレートを使えるようになります。」は、画面(core)がワークスペースの詳細の `projectRules` から数える。{ID} は削除済みでないプロジェクトの表示 ID(複数なら「・」でつなぐ) | 03「一覧と編集」の削除。サーバーに予告の口を足さずに済む(ルールは詳細に載っている) |
| B5a-18 | 画面への載せ方: `GET workspace` の `settings` に `projectRules`(プロジェクト ID → `{hiddenStatuses, workflow, templates, templateDefaults, disabledRules}`)を足す。自動化のルールの DTO に `scope`(空はワークスペース全体)を足す。core はこの 2 つから、プロジェクトごとの選択肢を作る | 詳細パネル・作成ダイアログ・プロジェクトの画面はすでに `workspaces.get` を読んでいるので、ルールの変更は、保存のあとの画面への反映(`reflect` → `detail.reload`、作成ダイアログは開くたびに読み直す `createDialog.ts:172`)で効く |
| B5a-19 | 移動でプロジェクトが変わるとき、`items.MoveInput` に `Statuses map[change.ID]change.ID`(使わないステータス → 置き換え先)を足す。移すアイテム(下のものと削除済みを含む)のうち、ステータスが移動先で使わないものを置き換える。指定のない使わないステータスは、同じ分類の移動先の既定にする。置き換え先が移動先で使わないステータスなら `ErrNotFound`。置き換えは遷移の制約を見ず、自動化を動かさない。予告 `MovePreview(ctx, store, id, project) (MovePreview, error)` は、下の件数(削除済みを除く)と、使わないステータスごとの件数(削除済みを含む)を返す | 02「同じワークスペースの中での移動」。03 の 160 行(移動は断らないが、選び直させるので使わないステータスにならない)。A4b の申し送り(`MoveInput` に足す) |
| B5a-20 | 画面の移動: メニューの「移動…」と Alt+Shift+↑↓(`ItemMove.moveTo`)は、プロジェクトが変わるときに予告を読み、使わないステータスがあれば確認ダイアログ「移動」を出す。「移動」で移し `true`、「キャンセル」で何もせず `false` を返す(Alt+Shift の順番待ちは捨てる)。なければダイアログを出さずに移す | 02 の 216 行、A4b の未決 9 |
| B5a-21 | 作成ダイアログでプロジェクトを変えたとき: ステータスが新しいプロジェクトで使わないものなら初期ステータスに戻す。テンプレートが使えないものなら「なし」に戻し(説明がテンプレート由来なら空に戻す)、新しいプロジェクトのいまの種類の既定があれば当てはめる。使えるならそのまま。A3b-14 の「プロジェクトを変えたときは当てはめない」と、そのテスト(`keeps the template when the project changes (B5 narrows it later)`)を、この規則に直す | 02 の 156 行 |
| B5a-22 | サブタブの上の 1 行の名前を押すと、プロジェクトの設定タブを開き、該当の区画(`id="rules-statuses"` などの見出し)までスクロールしてフォーカスする。区画の指定は、画面のアドレスに足さず、core の `navigation` に一度だけ読む「開く区画」を持たせる | 03「範囲と画面の構成」4 段落目。画面のアドレスの綴り(01)を増やさない |
| B5a-23 | カンバン・コマンドパレット・一括変更・検索とビューのカンバンは develop にまだないので、この計画では作らない。それらを作るトラックは、ステータスの選択肢と断りに `projectrule`(サーバー)・core の `projectStatusOptions`(画面)を使い、変更は `items.SetProperty` を通す。トラック B の計画の漏れの表に持ち場(E・A5)を書く(Task 1) | 画面がないものに判定を足せない。判定の関数は B5a が用意する |
| B5a-24 | `workspace.yaml` に `project_rules`(プロジェクト ID ごとに `hidden_statuses`・`workflow`・`templates`・`template_defaults`・`disabled_rules`。値は ID と行末の名前の注釈)を書き、ルールの行に `scope` を書く | 保存形式 42 行 |
| B5a-25 | 同期で重なったときの「上書き」の通知(06 の 228 行)と、削除された値・なくなったプロジェクトを指すルールの修復(保存形式「同期の修復」の (7)・(8))はトラック C(C3・C4)の持ち場。B5a は手の操作と追従だけを作る | トラック B の計画の決定 10 |
| B5a-26 | 別のワークスペース・別の Store への移動(A5a)で「移動先のプロジェクトが使うステータス」を決める `transfer.UsableStatuses`(`internal/domain/transfer/remap.go`)は、B5a の `projectrule.UsedStatuses` で決める。A5a が develop にあとから入ったので、A5a が合わせた: `UsableStatuses(to workspace.Settings, toRules projectrule.Rules)` が `UsedStatuses` を呼び、呼ぶ側の `Unmatched`・`Remap` も `projectrule.Rules` を受け取る(移動先のワークスペースのログから `projectrule.FromLog(log).Of(プロジェクト)` で読む)。テストは A5a の `domain/transfer/remap_test.go`・`app/transfers/projectrules_test.go` | A5a のセッションからの連絡(2026-11-14、ユーザー了承済み)。仕様 02「移す先の選び方」「値の引き直し」の「移動先のプロジェクトが使わないもの」 |

## Review Focus

1. **初期ステータスや分類の既定を外したあとで、ワークスペース側でそれが初期・既定でなくなった。** 外した印が再び働き、メニューから消える。逆に、外したステータスが既定になったら使うものに戻る(B5a-5)。Task 3 のテストに入れる。
2. **ワークスペースの行は制限なし、プロジェクトの行のチェックが使わないステータスだけ。** その行は「ワークスペースと同じ」として扱い、どこへでも移れる(B5a-6)。Task 3 のテストに入れる。
3. **作成ダイアログでプロジェクトを行き来する。** 使わないステータスは初期ステータスに、使えないテンプレートは「なし」と新しい既定に戻り、利用者が入れた説明は消えない(B5a-21)。Task 14 のテストに入れる。
4. **移動で、削除済みの子だけが使わないステータスを持つ。** 予告の件数に入り、ダイアログが出て、置き換えられる(B5a-19)。Task 8 のテストに入れる。
5. **専用ルールが使うステータスを、ワークスペースの設定で削除した。** 専用ルールは B4 の規則で書き換わり、`hidden_statuses` とプロジェクトの遷移から外れ、書き換え先がそのプロジェクトで使わないものでも残る(B5a-16)。Task 10 のテストに入れる。

---

## ファイルの地図

```
internal/domain/projectrule/rules.go          Rules・Set・FromLog・項目名
internal/domain/projectrule/judge.go          Uses・CheckUse・Allowed・CheckTransition・UsableTemplates・DefaultTemplate
internal/domain/projectrule/drafts.go         区画ごとの操作の行と断り・戻す
internal/domain/projectrule/follow.go         ステータス・テンプレートの削除と種類の変更の追従、テンプレートの削除の影響
internal/domain/projectrule/*_test.go
internal/domain/workspace/workspace.go        (変更)Multi に project_rules の 4 項目
internal/domain/automation/rule.go・log.go・check.go・run.go (変更)Scope・RunAll の Set
internal/app/projectrules/service.go・ops.go・reset.go  操作
internal/app/items/setproperty.go・create.go・createtemplate.go・automate.go・move.go (変更)
internal/app/items/movestatus.go              MovePreview・置き換え
internal/app/githublink/fire.go               (変更)RunAll に Set
internal/app/workspaces/value_delete.go・template.go・get.go (変更)追従と詳細
internal/app/automations/rules.go・view.go    (変更)Scope と活動の文
internal/app/apptest/stores_contract_values.go(変更)project_rules の契約
internal/infra/gitstore/yaml_settings.go・yaml_templates.go (変更)workspace.yaml
internal/adapter/httpapi/projectrules/        ルートと DTO
internal/adapter/httpapi/workspaces/dto.go・automations/dto.go・items/*.go (変更)
web/packages/core/src/model/workspace.ts      (変更)ProjectRules・AutomationRule.scope
web/packages/core/src/features/workspaces/projectRules.ts   判定と選択肢(Go の judge と同じ)
web/packages/core/src/features/projects/rulesEditor.ts      設定タブの操作
web/packages/core/src/features/items/(変更)properties.ts・createTemplate.ts・createDialog.ts・move 関係
web/packages/api/src/(変更)workspaces.ts・projects.ts・items.ts
web/app/src/screens/project/ProjectSettings.svelte と Rules*.svelte(区画ごと)
web/app/src/screens/workspace-settings/(変更)WorkflowTable.svelte・RuleDialog.svelte・AutomationPanel.svelte・TemplatesPanel.svelte・RulesProjectsLine.svelte
web/e2e/project-rules.spec.ts
docs/superpowers/plans/2026-11-12-hedgehug-track-b.md、docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md (変更)
```

---

### Task 1: トラック B の計画と保存形式の設計書を直す

**Files:**
- Modify: `docs/superpowers/plans/2026-11-12-hedgehug-track-b.md`(61 行の B5 の行、73・74・80・81 行の漏れの表)
- Modify: `docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md:316`

- [ ] **Step 1: 小計画の一覧の B5 を B5a・B5b・B5c の 3 行にする**

B5a の範囲・完成の条件はこの計画の Goal と B5a-1。B5b = 「一般」の「削除」とサイドバーのワークスペースのメニューの「削除…」、名前・接頭辞の重複から削除済みを除く規則(決定 12)。待つものなし。B5c = 「一般」の「別の Store へ移動」と「変更が及ぶ範囲」の移動の行。待つものは A5(転送の記録)とトラック C の `Replicator`。

- [ ] **Step 2: 漏れの表を直す**

「一般 / 別の Store へ移動」= B5c、「一般 / 削除」= B5b(復元は G)、「プロジェクト別のルール / …」= B5a(プロジェクトの移動での引き継ぎは A5)、「変更が及ぶ範囲」の B5 = B5a(プロジェクト別のルール)・B5b(削除)・B5c(移動)。表の下に 1 行足す: 「カンバン・コマンドパレット・一括変更・検索とビューのカンバンでの使わないステータスの扱いは、それらを作るトラック(E・A5)が B5a の `projectrule` と core の `projectStatusOptions` で行う(B5a-23)」。

- [ ] **Step 3: 保存形式の 316 行の遷移の項目名を `project_rules/<プロジェクトの内部 ID>/workflow/<From の値>`(値は To の値)に直す**(B5a-2)

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/plans/2026-11-12-hedgehug-track-b.md docs/superpowers/specs/2026-11-12-hedgehug-storage-design.md
git commit -m "docs: B5 を B5a・B5b・B5c に分け、プロジェクトの遷移の項目名をワークスペースとそろえる"
```

---

### Task 2: `projectrule` の型と読み取り

**Files:**
- Create: `internal/domain/projectrule/rules.go`、`rules_test.go`
- Modify: `internal/domain/workspace/workspace.go:39-41`

**Interfaces:**
- Produces:
  - `type Rules struct { Hidden []change.ID; Workflow map[change.ID][]change.ID; Templates []change.ID; TemplateDefaults map[string]string; Disabled []change.ID }`(`TemplateDefaults` の値はテンプレート ID か `None`)
  - `const None = "@none"`
  - `type Set map[project.ID]Rules`、`func (s Set) Of(p project.ID) Rules`(なければゼロ値)
  - `func FromLog(log []change.Record) Set`
  - `func (r Rules) Empty() bool`
  - 項目名: `HiddenField(p)`, `WorkflowField(p, from)`, `TemplatesField(p)`, `DisabledField(p)`, `TemplateDefaultField(p, kind)` → 文字列(B5a-2 の綴り)
  - `func IsMulti(f string) bool`(4 つの add・remove の項目なら true)

- [ ] **Step 1: 失敗するテストを書く** — `TestFromLogReadsEachSection`: 作成の行のあとに `NewAdd(HiddenField(P), "S2")`、`NewAdd(WorkflowField(P,"S1"), "S3")`、`NewAdd(TemplatesField(P), "T1")`、`NewSet(TemplateDefaultField(P,"Task"), nil, None)`、`NewAdd(DisabledField(P), "R1")` を `change.Record` にして渡し、`Set{P: {Hidden: [S2], Workflow: {S1: [S3]}, Templates: [T1], TemplateDefaults: {"Task": "@none"}, Disabled: [R1]}}` になる。`TestFromLogDropsRemovedAndEmpty`: add のあと remove した値は消え、全部消えたプロジェクトは `Set` に入らない。`template_default` を null に set したら、その種類はマップにない。`TestWorkspaceMultiCoversProjectRules`(`workspace` パッケージのテスト): 同じ `hidden_statuses` への 2 つの add が `workspace` の `opts` の `Resolve` で両方残る。

- [ ] **Step 2: `go test ./internal/domain/projectrule/ ./internal/domain/workspace/` を流し、`FromLog` がないことで落ちるのを見る**

- [ ] **Step 3: 実装する** — `FromLog` は `change.Resolve(log, change.Options{Multi: IsMulti})` の `Values`・`Lists` から `project_rules/<P>/…` を拾う(`automation.collect` と同じ形)。`workspace.go` の `Multi` に `|| projectRuleList(f)` を足す。`workspace` は `projectrule` を import できない(`projectrule` が `workspace` を使う)ので、`workspace` 側に同じ判定(`strings.HasPrefix(f, "project_rules/")` かつ末尾が `/hidden_statuses`・`/templates`・`/disabled_rules`、または 3 つ目の要素が `workflow`)を小さな関数で持ち、`projectrule.IsMulti` と同じ結果になることを `projectrule` のテストで表にして確かめる。

- [ ] **Step 4: テストを流して通るのを見る**

- [ ] **Step 5: Commit** — `feat: プロジェクト別のルールの型と、ワークスペースのログからの読み取り`

---

### Task 3: 判定の関数

**Files:**
- Create: `internal/domain/projectrule/judge.go`、`judge_test.go`

**Interfaces:**
- Consumes: Task 2 の `Rules`
- Produces:
  - `func Uses(s workspace.Settings, r Rules, status change.ID) bool`(B5a-5)
  - `func CheckUse(s workspace.Settings, r Rules, status change.ID) error` — 使わないなら `refusal.New("このプロジェクトでは「" + 名前 + "」を使いません")`
  - `func UsedStatuses(s workspace.Settings, r Rules) []workspace.Value`(定義順)
  - `func Allowed(s workspace.Settings, r Rules, from change.ID) (to []change.ID, restricted bool)`(B5a-6。ステータスの並び)
  - `func CheckTransition(s workspace.Settings, r Rules, from, to change.ID) error`
  - `func UsableTemplates(s workspace.Settings, r Rules, kind string) []workspace.Template`
  - `func DefaultTemplate(s workspace.Settings, r Rules, kind string) (change.ID, bool)`(B5a-9)

- [ ] **Step 1: 失敗するテストを書く**(サンプルは `workspace.DefaultSettings` の 5 つ。Todo が初期、Todo・InProgress・Done・Cancel が分類の既定、Pending は既定でない)
  - `TestUsesKeepsInitialAndDefaults`: `Hidden: [Todo, Pending]` で `Uses(Todo)` = true、`Uses(Pending)` = false。設定の初期を InProgress に、todo の既定を InProgress に変えた `Settings` では `Uses(Todo)` = false(Review Focus 1)。
  - `TestCheckUseText`: `CheckUse(Pending)` の文が `このプロジェクトでは「Pending」を使いません`。
  - `TestCheckTransitionBothMustAllow`: ワークスペース `Todo → [InProgress, Done]`、プロジェクト `Todo → [InProgress]` で `Todo → Done` が `ワークフローの制約: Todo から Done には変更できません(InProgress のみ)`。
  - `TestCheckTransitionNoCommonTarget`: ワークスペース `Todo → [Done]`、プロジェクト `Todo → [InProgress]` で `…(変更できる先がありません)`。
  - `TestProjectRowOfUnusedOnlyIsUnrestricted`: プロジェクト `Todo → [Pending]`、`Hidden: [Pending]`、ワークスペースは制限なしで `CheckTransition(Todo, Done)` が nil(Review Focus 2)。
  - `TestAllowedDropsUnused`: ワークスペース `Todo → [InProgress, Pending]`、`Hidden: [Pending]` で `Allowed` = `[InProgress], true`。
  - `TestCheckTransitionPassesMissingFrom`: from が一覧にない ID なら nil。
  - `TestUsableTemplatesAndDefault`: Task のテンプレート T1・T2、ワークスペースの既定 T2。`Templates: [T1]` で使える = `[T1]`、既定 = なし(T2 が使えない)。`TemplateDefaults{"Task": T1}` で既定 = T1。`{"Task": "@none"}` でなし。`Templates` を空にして `TemplateDefaults{"Task": 種類の違う T3}` なら、ワークスペースの既定 T2 があっても「なし」。`Templates: [消したテンプレートの ID]` だけなら全部使える。

- [ ] **Step 2: 流して落ちるのを見る**

- [ ] **Step 3: 実装する**

- [ ] **Step 4: 流して通るのを見る**

- [ ] **Step 5: Commit** — `feat: 使うステータス・両方が許す遷移・使えるテンプレートの判定`

---

### Task 4: 区画ごとの操作の行と断り

**Files:**
- Create: `internal/domain/projectrule/drafts.go`、`drafts_test.go`

**Interfaces:**
- Consumes: Task 2・3。`automation` を import しないので、専用ルールは呼ぶ側が `[]ScopedRule{ID, Name, Statuses []change.ID}` にして渡す。
- Produces:
  - `type ScopedRule struct { Name string; Statuses []change.ID }`
  - `func StatusUseDrafts(s workspace.Settings, r Rules, p project.ID, status change.ID, used bool, own []ScopedRule) ([]change.Draft, error)` — 断りは B5a-12。同じ状態なら空。
  - `func WorkflowDrafts(s workspace.Settings, r Rules, p project.ID, from, to change.ID, allowed bool) ([]change.Draft, error)` — B5a-13。from か to が一覧にない・同じなら `ErrUnknown`(呼ぶ側で 404)。
  - `func TemplateUseDrafts(s workspace.Settings, r Rules, p project.ID, t change.ID, used bool) ([]change.Draft, error)`
  - `func TemplateDefaultDraft(s workspace.Settings, r Rules, p project.ID, kind, value string) (change.Draft, bool, error)` — value は `""`(ワークスペースと同じ)・`None`・使えるテンプレート ID。変わらなければ false。
  - `func RuleRunsDraft(r Rules, p project.ID, rule change.ID, runs bool) (change.Draft, bool)`
  - `type Section string` = `"statuses"`・`"workflow"`・`"templates"`・`"automation"`、`func (Section) Label() string` = 「使うステータス」「ワークフローの制約」「テンプレート」「自動化」
  - `func ResetDrafts(r Rules, p project.ID, sec Section) []change.Draft`(専用ルールの削除は呼ぶ側が足す。B5a-14)
  - `var ErrUnknown = errors.New("projectrule: unknown value")`

- [ ] **Step 1: 失敗するテストを書く** — `TestStatusUseRefusesInitialAndDefault`(3 つの文を表で。Todo は「初期ステータスで、未着手の既定でもあるため外せません」、InProgress は「進行中の既定のため外せません」、初期を Pending にした設定の Pending は「初期ステータスのため外せません」)、`TestStatusUseRefusesOwnRule`(既定でない Pending を使う `ScopedRule{Name: "保留で未割り当て", Statuses: [Pending]}` があるとき、Pending を外すと「「Pending」はこのプロジェクトだけのルール「保留で未割り当て」が使っているため、使わないことにできません。先にルールを変えてください」)、`TestStatusUseWritesAddRemove`(外すと `add hidden_statuses`、戻すと `remove`)、`TestWorkflowRefusesCellTheWorkspaceForbids`(「ワークスペースの制約で変更できません」)、`TestTemplateDefaultValues`(`""` は null の set、`None`、使えない ID は `ErrUnknown`)、`TestResetDraftsPerSection`(区画ごとに、その区画の行だけ。テンプレートは `templates` の remove と `template_default/*` の null)。

- [ ] **Step 2〜4: 落ちるのを見る → 実装 → 通るのを見る**

- [ ] **Step 5: Commit** — `feat: プロジェクト別のルールの操作の行と断り`

---

### Task 5: 削除と種類の変更の追従

**Files:**
- Create: `internal/domain/projectrule/follow.go`、`follow_test.go`

**Interfaces:**
- Produces:
  - `func StatusDeleteDrafts(set Set, id change.ID) []change.Draft`(B5a-16。プロジェクト ID の順に、`hidden_statuses` の remove、`workflow/<id>` の全 To の remove、ほかの行の `id` の remove)
  - `func TemplateDropDrafts(set Set, id change.ID) []change.Draft`(`templates` の remove、`template_default/*` が id なら null)
  - `func RuleDeleteDrafts(set Set, rule change.ID) []change.Draft`(ワークスペースのルールの削除で、プロジェクト ID の順に `disabled_rules` から rule を remove。`automations.Delete` が同じ保存に足す)

- [ ] **Step 1: 失敗するテストを書く** — 2 つのプロジェクトのルールで、`StatusDeleteDrafts` の行の並びと中身を `reflect.DeepEqual` で。`TemplateDropDrafts` は、既定だけで指しているプロジェクトにも行が出る。`RuleDeleteDrafts` は、外しているプロジェクトだけに行が出る。
- [ ] **Step 2〜4**
- [ ] **Step 5: Commit** — `feat: ステータス・テンプレートの削除にプロジェクト別のルールを追従させる行`

---

### Task 6: 自動化の専用ルール・動かさない指定・使わないステータス

**Files:**
- Modify: `internal/domain/automation/rule.go`(`Scope project.ID`)、`log.go`(`set` の `scope`、`Drafts` の `scope` は作成のときだけ)、`check.go`(`ValidFor`)、`run.go`(`RunAll` に `Set`)
- Modify: `internal/app/items/automate.go`、`internal/app/githublink/fire.go`(`projectrule.FromLog` を同じログから読んで渡す)
- Test: `internal/domain/automation/*_test.go`、`internal/app/items/automate_test.go`、`internal/app/githublink/fire_test.go`

**Interfaces:**
- Produces:
  - `func Run(s workspace.Settings, set projectrule.Set, rules []Rule, it item.Item, ev Event) []Outcome`、`RunAll` も同じ引数の並び
  - `func Valid(s workspace.Settings, set projectrule.Set, logins []string, old, r Rule) bool` — `Scope` があれば、old(編集前のルール。作成ならゼロ値)から変えたステータスは `projectrule.Uses(s, set.Of(r.Scope), …)` も満たす(B5a-10)
  - `func Statuses(r Rule) []change.ID`(きっかけ・することのステータス。Task 9 の `ScopedRule` を作るのに使う)

- [ ] **Step 1: 失敗するテストを書く** — `TestRunSkipsOtherProjectsScopedRule`、`TestRunSkipsDisabledInProject`、`TestRunStopsOnUnusedStatus`(`Stopped` = 「このプロジェクトでは「Pending」を使いません」、`Draft` なし)、`TestDraftsWriteScopeOnlyOnCreate`、`TestFromLogReadsScope`、`TestValidRejectsUnusedStatusForScopedRule`。app 側: `TestStatusChangeRunsScopedRuleOnlyInItsProject`(サンプルに 2 つ目のプロジェクトを作り、そのプロジェクトの専用ルールがサンプルのタスクでは動かない)、githublink の `fire` のテストに `disabled_rules` で止まる場面を 1 つ。
- [ ] **Step 2〜4**
- [ ] **Step 5: Commit** — `feat: 自動化で専用ルール・動かさない指定・使わないステータスを判定する`

---

### Task 7: アイテムの操作で使わないステータスと両方の遷移を断る

**Files:**
- Modify: `internal/app/items/setproperty.go:76-88`(`checkStatus`)、`create.go:134`(`checkValues`)、`createtemplate.go:18`(`templateOf`)
- Test: `internal/app/items/workflow_test.go`、`create_test.go`

**Interfaces:**
- Consumes: Task 3 の `CheckUse`・`CheckTransition`・`UsableTemplates`

- [ ] **Step 1: 失敗するテストを書く**
  - `TestSetStatusRefusesUnusedStatus`: サンプルのプロジェクトに `add hidden_statuses Pending` を書いた上で、タスクを Pending にすると `refusal` で「このプロジェクトでは「Pending」を使いません」、アイテムは変わらない。
  - `TestSetStatusChecksProjectWorkflow`: ワークスペースは制限なし、プロジェクト `Todo → [InProgress]` で `Todo → Done` が「ワークフローの制約: Todo から Done には変更できません(InProgress のみ)」。
  - `TestCreateRefusesUnusedStatus`、`TestCreateRefusesTemplateTheProjectDoesNotUse`(`port.ErrNotFound`)。
  - `TestPathsThatDoNotRefuseUnusedStatus`: Pending を外したあと、値の削除の置き換え先を Pending にした `DeleteValue` と、「制約を無視して…」で Pending にする `ApplyIgnoringWorkflow` が成功する(これは `app/workspaces`・`app/automations` のテストに置く。B5a-7)。
- [ ] **Step 2〜4** — `checkStatus` は、`tx.Changes().Of(ワークスペース)` から `projectrule.FromLog` を読み、`CheckUse` → `CheckTransition` の順に呼ぶ。
- [ ] **Step 5: Commit** — `feat: ステータスの変更と作成で、プロジェクトが使わないステータスと両方の遷移を断る`

---

### Task 8: 移動のステータスの置き換え(サーバー)

**Files:**
- Create: `internal/app/items/movestatus.go`、`movestatus_test.go`
- Modify: `internal/app/items/move.go`(`MoveInput.Statuses`、`writeMove` で置き換えの行)

**Interfaces:**
- Produces:
  - `MoveInput{ Parent *item.ID; Project *project.ID; Statuses map[change.ID]change.ID }`
  - `type MovePreview struct { Below int; Statuses []StatusCount }`、`type StatusCount struct { Status change.ID; Count int; Default change.ID }`(Default は同じ分類の移動先の既定)
  - `func (s Service) MovePreview(ctx context.Context, store port.StoreID, id item.ID, in MoveInput) (MovePreview, error)` — 移動先のプロジェクトがいまと同じなら `Statuses` は空

- [ ] **Step 1: 失敗するテストを書く** — `TestMovePreviewCountsDeletedChildren`(Epic の下に Pending の削除済みの Task が 1 件。移動先のプロジェクトが Pending を外している。`Below` は削除済みを数えず、`Statuses` = `[{Pending, 1, Todo}]`。Review Focus 4)、`TestMoveReplacesUnusedStatuses`(指定の置き換え先になる。指定がなければ `Default`。履歴に残り、自動化は動かない)、`TestMoveRefusesUnusedReplacement`(`ErrNotFound`)、`TestMoveWithinProjectKeepsStatus`。
- [ ] **Step 2〜4**
- [ ] **Step 5: Commit** — `feat: プロジェクトが変わる移動で、使わないステータスを置き換える`

---

### Task 9: `app/projectrules` の操作

**Files:**
- Create: `internal/app/projectrules/service.go`(`Service{Stores port.Stores; Identity port.Identity}`、共通の `edit`)、`ops.go`、`reset.go`、`*_test.go`

**Interfaces:**
- Produces(どれも `ctx, store port.StoreID, p project.ID` を先に取り、`error` を返す):
  - `SetStatusUsed(…, status change.ID, used bool)`
  - `SetTransition(…, from, to change.ID, allowed bool)`
  - `SetTemplateUsed(…, t change.ID, used bool)`
  - `SetTemplateDefault(…, kind, value string)`
  - `SetRuleRuns(…, rule change.ID, runs bool)` — ルールがワークスペース全体のものでなければ `ErrNotFound`
  - `Reset(…, sec projectrule.Section)`
- 活動の文(03 の表): 「プロジェクト「{タイトル}」の使うステータスを変更」「プロジェクト「{タイトル}」のワークフローの制約を変更」「プロジェクト「{タイトル}」の使うテンプレートを変更」「プロジェクト「{タイトル}」の{種類}の既定のテンプレートを変更」「プロジェクト「{タイトル}」で自動化「{ルール名}」を動かす(動かさない)ようにする」「プロジェクト「{タイトル}」の{区画名}をワークスペースの設定に戻す」。行がない操作(同じ状態)は保存しない。

- [ ] **Step 1: 失敗するテストを書く** — 操作ごとに、行と活動の文を記録係(`items/edit_test.go` の `setupRec` と同じ形)で確かめる。`TestDeletedProjectIsNotFound`(B5a-15)、`TestResetAutomationDeletesScopedRules`(専用ルール 2 件が `delete rules/<ID>` で消え、`disabled_rules` が空になる)、`TestResetWithoutRulesIsNotFound`、`TestStatusUsedRefusesOwnRule`(専用ルールは `automation.FromLog` を `ScopedRule` にして渡す)。
- [ ] **Step 2〜4**
- [ ] **Step 5: Commit** — `feat: プロジェクト別のルールの操作`

---

### Task 10: ワークスペースの設定の追従・詳細・専用ルールの操作

**Files:**
- Modify: `internal/app/workspaces/value_delete.go:34`(ステータスのとき `projectrule.StatusDeleteDrafts` を足す)、`template.go:63-65・84`(削除と種類の変更で `TemplateDropDrafts`)、`get.go`(`Detail.ProjectRules projectrule.Set`)
- Modify: `internal/app/automations/rules.go`(作成は `Scope` を受け、`Valid` に `Set` を渡す。活動の文は `Scope` があれば「プロジェクト「{タイトル}」の自動化「{名前}」を作成(編集、削除)」)、`view.go`(ルールに `Scope`)
- Test: 各パッケージの既存のテストファイル

- [ ] **Step 1: 失敗するテストを書く** — `TestDeleteStatusFollowsProjectRules`(Review Focus 5: 専用ルールのすることが Pending、`hidden_statuses` に Review。Review を削除し代わりを Pending にすると、専用ルールは Pending に書き換わって残り、`hidden_statuses` と遷移から Review が外れる)、`TestDeleteTemplateFollowsProjectRules`、`TestTemplateKindChangeFollowsProjectRules`、`TestGetIncludesProjectRules`、`TestCreateScopedRuleSummary`、`TestUpdateKeepsScope`。
- [ ] **Step 2〜4**
- [ ] **Step 5: Commit** — `feat: 値・テンプレートの削除をプロジェクト別のルールに追従させ、専用ルールを作れるようにする`

---

### Task 11: HTTP のルートと API の型

**Files:**
- Create: `internal/adapter/httpapi/projectrules/projectrules.go`(と DTO)
- Modify: `internal/adapter/httpapi/routes.go`(`projectrules.Register`)、`workspaces/dto.go`(`WorkspaceSettings.ProjectRules map[string]ProjectRules`)、`automations/dto.go`(`AutomationRuleInput.Scope string`)、`items/*.go`(移動の本文に `statuses map[string]string`、予告のルート)
- Regenerate: `api/openapi.yaml`、`web/packages/api/src/generated.ts`
- Modify: `web/packages/api/src/workspaces.ts`・`projects.ts`・`items.ts` と core の `port/*.ts`、`model/workspace.ts`(`ProjectRules`、`AutomationRule.scope`)

**Interfaces:**
- ルート(`OperationID` は括弧の中):
  - PUT `/api/stores/{store}/projects/{project}/rules/statuses/{status}` 本文 `{used}`(`set-project-status-used`)
  - PUT `…/rules/workflow/{from}/{to}` 本文 `{allowed}`(`set-project-transition`)
  - PUT `…/rules/templates/{template}` 本文 `{used}`(`set-project-template-used`)
  - PUT `…/rules/template-defaults/{kind}` 本文 `{template}`(`set-project-template-default`。`""`・`"@none"`・ID)
  - PUT `…/rules/automation/{rule}` 本文 `{runs}`(`set-project-rule-runs`)
  - POST `…/rules/{section}/reset`(`reset-project-rules`)
  - GET `/api/stores/{store}/items/{item}/move-preview?project=`(`preview-item-move`)
- core の型: `ProjectRules { hiddenStatuses: string[]; workflow: Record<string, string[]>; templates: string[]; templateDefaults: Partial<Record<ItemKind, string>>; disabledRules: string[] }`、`WorkspaceSettings.projectRules: Record<string, ProjectRules>`、`ProjectsPort` に 6 つ(`setStatusUsed`・`setTransition`・`setTemplateUsed`・`setTemplateDefault`・`setRuleRuns`・`resetRules`)、`ItemsPort.movePreview(storeId, itemId, projectId): Promise<MovePreview>` と `move` の `statuses`。

- [ ] **Step 1: 失敗するテストを書く** — httpapi の既存の形(`httptest` で 422 の文言、404)で、断りと削除済みのプロジェクトの 404 を 1 つずつ。api の `*.ports.test.ts` に、本文とパスの組み立てを 1 つずつ。
- [ ] **Step 2〜4** — `make generate && cd web && pnpm gen` で作り直す。
- [ ] **Step 5: Commit** — `feat: プロジェクト別のルールと移動の予告の API`

---

### Task 12: `workspace.yaml` と Store の契約

**Files:**
- Modify: `internal/infra/gitstore/yaml_settings.go`・`yaml_templates.go`(`rulesNode` に `scope`)、`internal/app/apptest/stores_contract_values.go`
- Test: `internal/infra/gitstore/yaml_*_test.go`

- [ ] **Step 1: 失敗するテストを書く** — gitstore: ルールのあるワークスペースの `workspace.yaml` の `project_rules:` の節が、B5a-24 の形で、値に行末の名前の注釈が付く(期待の YAML を文字列で)。専用ルールの行に `scope: <ID>  # HH-P1` が出る。契約: `contractProjectRules` を `stores_contract_values.go` に足し、偽物と gitstore で、`hidden_statuses` の 2 つの add と 1 つの remove が `projectrule.FromLog(Changes().Of(ws))` で同じに読める(`stores_contract.go` の場面の一覧にも足す)。
- [ ] **Step 2〜4**
- [ ] **Step 5: Commit** — `feat: workspace.yaml にプロジェクト別のルールを書き出す`

---

### Task 13: core の判定と選択肢(Go と同じ規則)

**Files:**
- Create: `web/packages/core/src/features/workspaces/projectRules.ts`、`projectRules.test.ts`
- Modify: `web/packages/core/src/features/workspaces/workflow.ts`(`transitionRefusal(settings, from, to, project?)`)、`web/packages/core/src/features/items/properties.ts:93`(`statusOptions(settings, current, workflow, project?)`)、`web/packages/core/src/features/workspaces/templateApply.ts:9-13`(`usableTemplates`・`defaultTemplate` に `project?`)、`workspacesIndex.ts`

**Interfaces:**
- Produces: `rulesOf(settings, projectId): ProjectRules`、`usesStatus(settings, projectId, statusId): boolean`、`projectStatusOptions` = `statusOptions` に project を渡したもの(いまの値が使わないものなら、その 1 項目をチェックつきで残し `hint` に「このプロジェクトでは使わない」)、`unusedRefusal(name)`、`projectWorkflowRows(settings, projectId)`(使うステータスだけの行。セルに `disabled` と `reason`「ワークスペースの制約で変更できません」、行の `note` は「ワークスペースと同じ」「制限あり」、両方が許す To がなければ `blocked: "変更できる先がありません"`)、`usedStatusRows(settings, projectId, items)`(B5a-12 の注記と「{N} 件が使用中」と件数「{使う数} / {全数}」)、`templateDeleteLines(settings, templateId, projects)`(B5a-17)、`rulesProjects(settings, section, projects)`(サブタブの 1 行の材料。ルールが空・削除済みは除く)、`rulesHeading(workspaceName)` = 「このプロジェクトだけのルールです。ステータスや種類の定義は {ワークスペース名} のままです。」

- [ ] **Step 1: 失敗するテストを書く** — Task 3 の Go のテストと同じ場面を同じ期待値で(Review Focus 1・2 を含む)。`projectStatusOptions` のいまの値が使わないときの 1 項目、`templateDeleteLines` の 2 つの文、`rulesProjects` の「プロジェクト別のルールがあるプロジェクト: HH-P1 ログイン・HH-P2 決済(2 件)」の文。
- [ ] **Step 2〜4**
- [ ] **Step 5: Commit** — `feat: core にプロジェクト別のルールの判定と選択肢を足す`

---

### Task 14: ステータスのメニューと作成ダイアログ

**Files:**
- Modify: `web/app/src/screens/item/StatusButton.svelte`(`project` を受ける)と呼ぶ 5 か所(`TreeRow.svelte:96`・`EpicRow.svelte:63`・`DetailChildren.svelte:63`・`StepRow.svelte:54`・`PropertyField.svelte:102`)、`web/packages/core/src/features/items/properties.ts:130-146`(`choiceRow`・`propertyRows` に project)、`createTemplate.ts:37・77`、`createDialog.ts:194`、`createDialogOptions.ts:31-35`
- Test: `web/packages/core/src/features/items/*.test.ts`

- [ ] **Step 1: 失敗するテストを書く** — `createDialog` のテスト: プロジェクトを Pending を外した P2 に変えると、選んでいた Pending が初期ステータスに戻る。P2 で使えないテンプレート T1 を選んでいたら「なし」に戻り、説明がテンプレート由来なら空、P2 の Task の既定 T2 が当たる。利用者が書いた説明は残る(Review Focus 3)。A3b の `keeps the template when the project changes (B5 narrows it later)` を `keeps the template when the new project can use it` に直し、使えないときの場面を足す(B5a-21)。詳細パネルの行の選択肢が、アイテムのプロジェクトの使うステータスだけになる。
- [ ] **Step 2〜4**
- [ ] **Step 5: Commit** — `feat: ステータスのメニューと作成ダイアログで、プロジェクトの使うステータスとテンプレートだけを出す`

---

### Task 15: 移動の確認ダイアログ「ステータスの置き換え」

**Files:**
- Modify: `web/packages/core/src/features/items/`(`ItemMove` の `choose`・`moveTo`。A4b の `wireActions.ts` の `itemMove`)
- Create: `web/app/src/screens/item/MoveStatusDialog.svelte`
- Test: `web/packages/core/src/features/items/move*.test.ts`

**Interfaces:**
- `ItemMove.moveTo(item, candidate): Promise<boolean>` は B5a-20 のとおり。ダイアログの状態 `moveConfirm: Readable<MoveConfirm | null>`、`MoveConfirm { title: "移動"; body: string; rows: { status; label: "{名前}({M} 件)"; options; value }[] }`、`confirmMove(values)`、`cancelMove()`。

- [ ] **Step 1: 失敗するテストを書く** — 予告に使わないステータスがあるとき、ダイアログの本文が「HH-1 リリース作業 と、その下の 2 件を 決済 へ移動します。」、行が「Pending(1 件)」で初期値が予告の `default`。「キャンセル」で `moveTo` が `false` を返し、`move` を呼ばない。予告が空なら、ダイアログを出さずに `move` を呼ぶ。同じプロジェクトの中の移動では予告を読まない。
- [ ] **Step 2〜4**
- [ ] **Step 5: Commit** — `feat: プロジェクトが変わる移動で、ステータスの置き換えを選ばせる`

---

### Task 16: ワークフローの表とルールのダイアログを部品にする

**Files:**
- Create: `web/app/src/screens/workspace-settings/WorkflowTable.svelte`(props `{ rows, readOnly, onchange(from, to, checked) }`)
- Modify: `WorkflowPanel.svelte`(表を部品に替える)、`RuleDialog.svelte`(props を `{ settings, members, readOnly, rules, rule, statuses, onsave(input): Promise<string | null>, onclose }` にし、`workspaceSettings.run` への直接の依存を外す。`statuses` はステータスの選択肢)、`AutomationPanel.svelte`(新しい props で呼ぶ)

- [ ] **Step 1: e2e の `workspace-settings.spec.ts`・`automation.spec.ts` を流して、いまの振る舞いが緑であることを見る**(部品にするだけで振る舞いを変えない)
- [ ] **Step 2: 部品にする**
- [ ] **Step 3: `cd web && pnpm check && pnpm test` と、同じ 2 つの e2e を流して緑**
- [ ] **Step 4: Commit** — `refactor: ワークフローの表とルールのダイアログを、プロジェクトでも使える部品にする`

---

### Task 17: プロジェクトの設定タブ(使うステータス・ワークフローの制約)

**Files:**
- Create: `web/packages/core/src/features/projects/rulesEditor.ts`(`createRulesEditor({ projects, toasts, onChanged })`。`run(op, ok?)`: 断りはエラーのトースト、成功で `ok` のトースト、いつも `onChanged`)、`rulesEditor.test.ts`
- Create: `web/app/src/screens/project/RulesStatuses.svelte`、`RulesWorkflow.svelte`、`ResetRulesButton.svelte`(確認ダイアログ「ワークスペースの設定に戻す」、ボタン「戻す」、本文 B5a-14、成功のトースト「ワークスペースの設定に戻しました」、行がなければ無効)
- Modify: `ProjectSettings.svelte`(見出しの下の 1 行 `rulesHeading`、区画の並び 02 の 527 行。「別のワークスペースへ移動」は A5 が足す位置としてコメントだけ)、`web/app/src/wire.ts`・`projectEditor.ts`(`rulesEditor` の組み立て。`onChanged` は `projectEditor` と同じ)

- [ ] **Step 1: 失敗するテストを書く** — `rulesEditor`: 断りの文がエラーのトーストに出て `onChanged` が呼ばれる。`reset` の成功で「ワークスペースの設定に戻しました」。
- [ ] **Step 2〜4** — 閲覧のみの Store では、入力とボタンを無効にして `readOnlyNote` を添える。区画の見出しに `id="rules-statuses"`・`id="rules-workflow"`。
- [ ] **Step 5: Commit** — `feat: プロジェクトの設定タブに使うステータスとワークフローの制約を足す`

---

### Task 18: プロジェクトの設定タブ(テンプレート・自動化)

**Files:**
- Create: `web/app/src/screens/project/RulesTemplates.svelte`(上に「使うテンプレート」のチェック。0 件なら「すべて使えます」。下に種類ごとのセレクト「ワークスペースと同じ({名前}または「なし」)」「なし」と使えるテンプレート)、`RulesAutomation.svelte`(判定されるルールを ID 順に。行 = スイッチ・要約・名前・出どころ「ワークスペース」「このプロジェクトだけ」。ワークスペースで無効なら「ワークスペースで無効」と添えてスイッチ無効。専用ルールは「編集」「削除」。末尾に「このプロジェクトだけのルールを追加」で Task 16 の `RuleDialog` を `statuses` = 使うステータスで開く)
- Modify: `web/packages/core/src/features/workspaces/automation.ts`(`projectRuleRows(automation, settings, members, projectId)`)

- [ ] **Step 1: 失敗するテストを書く** — `projectRuleRows`: ほかのプロジェクトの専用ルールは出ない、出どころの文、無効のワークスペースのルールの注記、`disabled_rules` のルールのスイッチが OFF。
- [ ] **Step 2〜4** — 区画の見出しに `id="rules-templates"`・`id="rules-automation"`。自動化の区画はプロジェクトの設定タブを開いているときだけ `workspaces.automation` を読む(B4-22 と同じ)。
- [ ] **Step 5: Commit** — `feat: プロジェクトの設定タブにテンプレートと自動化を足す`

---

### Task 19: ワークスペースの設定の画面の 3 か所

**Files:**
- Create: `web/app/src/screens/workspace-settings/RulesProjectsLine.svelte`(「ステータス」「ワークフロー」「テンプレート」「自動化」のサブタブの上。名前を押すと B5a-22 の「開く区画」を置いてプロジェクトの設定タブへ)
- Modify: `SettingsTab.svelte`(1 行を置く)、`AutomationPanel.svelte`(専用ルールの行に、名前のあとに「HH-P1」、削除済みなら「HH-P1(削除済み)」)、`TemplatesPanel.svelte`(削除の確認に `templateDeleteLines`)、`ProjectSettings.svelte`(「開く区画」を読んで `scrollIntoView` とフォーカス)、`web/packages/core/src/features/navigation`(`openSection: string | null` を一度だけ読む関数 `takeSection()`)

- [ ] **Step 1: 失敗するテストを書く** — core: `takeSection` は 2 回目に `null`。`ruleRows` が専用ルールに「HH-P1」、削除済みのプロジェクトなら「HH-P1(削除済み)」を付ける。
- [ ] **Step 2〜4**
- [ ] **Step 5: Commit** — `feat: 設定のサブタブにプロジェクト別のルールがあるプロジェクトを出し、削除の確認に影響を添える`

---

### Task 20: e2e

**Files:**
- Create: `web/e2e/project-rules.spec.ts`

- [ ] **Step 1: 場面を書く**(どれも自分のワークスペースを `createWorkspace` で作る)
  1. プロジェクトの設定タブで「Pending」のチェックを外すと、件数が「4 / 5」になり、ツリーのタスクのステータスのメニューに Pending が出ない(保存のあと、画面を読み直さずに)。Todo の行は無効で「初期ステータスで、未着手の既定でもあるため外せません」。
  2. プロジェクトのワークフローで Todo → InProgress だけにすると、メニューの Done が無効で、理由が「ワークフローの制約: Todo から Done には変更できません(InProgress のみ)」。
  3. 使うテンプレートを 1 つにすると、作成ダイアログのテンプレートの選択肢がそれと「なし」だけになる。
  4. 専用ルールを作り(「ステータスが次になったとき InProgress → 担当者を 未割り当て に」)、ワークスペースの自動化のサブタブにも、名前のあとに「HH-P1」を添えて出る。そこで編集しても、ステータスの選択肢はそのプロジェクトが使うものだけ。ステータスのサブタブの上に「プロジェクト別のルールがあるプロジェクト: …(1 件)」が出て、押すとプロジェクトの設定タブのその区画が開く。
  5. 「ワークスペースの設定に戻す…」で、確認の本文が出て、「戻す」のあとトースト「ワークスペースの設定に戻しました」、チェックがすべて付く。
  6. Epic を Pending を外したプロジェクトへ「移動…」すると、確認ダイアログに「Pending(1 件)」が出て、「移動」のあと Pending が置き換わる。
  7. 閲覧のみの Store のプロジェクトの設定タブで、4 つの区画の入力が無効で「閲覧のみの Store です」。
- [ ] **Step 2: `make e2e-bin && cd web && pnpm exec playwright test project-rules.spec.ts` を流して、3 幅で通る**
- [ ] **Step 3: Commit** — `test: プロジェクト別のルールの通しのテスト`

---

### Task 22: 同期の修復 (5) でのステータスの置き換え

develop の C3a(PR #199)が、06「親と別のワークスペース・プロジェクトにある子」のうちステータスの置き換えを B5 の持ち場として残した(C3a-16)。B5a で使わないステータスを作ったので、この PR に入れる(ユーザーの決定、2026-11-14)。

**Files:**
- Modify: `internal/app/repairing/place.go`(`moveToProject`)
- Test: `internal/app/repairing/place_test.go`

- [ ] **Step 1: 失敗するテストを書く** — `TestMatchProjectsReplacesUnusedStatus`: 親 Epic がプロジェクト P2(`hidden_statuses` に Pending)にあり、子 Task が P1 で Pending。修復のあと、子は P2 に移り、ステータスは同じ分類(未着手)の既定の Todo になる。置き換えは移動と同じ保存(同じ `op`)で、実行者は同期。`TestMatchProjectsKeepsUsedStatus`: 子のステータスを P2 が使うなら変えない。
- [ ] **Step 2: 流して落ちるのを見る**
- [ ] **Step 3: 実装する** — `moveToProject` は、ワークスペースのログから `projectrule.FromLog` を読み、`projectrule.Uses(st, set.Of(m.Project), it.Status)` が false なら、`st.CategoryDefaults[その分類]` への `set status` を place の行と同じ `Append` に足す(06 の 315 行)。
- [ ] **Step 4: 流して通るのを見る**
- [ ] **Step 5: Commit** — `fix: 親に合わせてプロジェクトを移す同期の修復で、移動先が使わないステータスを置き換える`

### Task 21: 全体の確認と PR

- [ ] **Step 0: develop を取り込む。A5a が先に入っていれば、B5a-26 のとおり `transfer.UsableStatuses` を差し替える(テストを書いてから)**
- [ ] **Step 1: `make check` と `make web-check` と e2e 全体を 1 回流し、緑を確かめる**
- [ ] **Step 2: 最終レビュー(superpowers:requesting-code-review)で出たものを直す。直したことは、この計画の末尾の「実装で計画から変えたこと」に書く**
- [ ] **Step 3: push して PR を出す** — 向き先 develop、タイトル「feat: トラック B5a(プロジェクト別のルール)」。本文に、できること・決めたこと(B5a-1〜B5a-26)・A3b-14 と A4b の未決 8・9 をどう片付けたか・確認の結果を書く。

---

## 実装で計画から変えたこと

- B5a-9: プロジェクトの既定のテンプレートが使えないときは、仕様(03「プロジェクトのテンプレート」)のとおり「なし」にする。ワークスペースの既定には戻らない。
- `TemplateDeleteImpact` は `workspace.Settings` も受ける形にした(削除済みのテンプレートの残りを数えないため)。そのあと、画面(core の `templateDeleteLines`)だけが数えるので(B5a-17)、`TemplateDeleteImpact`・`TemplateImpact` と、使われていなかった `ProjectRowRestricted` は消した。`UsedStatuses`(B5a-26)と `projectStatusOptions`(B5a-23)は残す。
- 移動の予告 `movePreview` は、`ItemActionsPort` に `MoveTarget`(プロジェクトか親)を受ける形で置いた。
- ui の `Checkbox` に、制御された `onchange` を足した(B4-23 の `Switch` と同じ。渡すと押しても自分では変わらず `checked` に従うので、断られたチェックが保存済みの値に戻る。`{#key}` の描き直しはフォーカスを失うのでやめた)。
- 移動の確認ダイアログの「ステータスの置き換え」のセレクトは、最初の描画から値を持つ束縛にした(e2e で見つけた)。
- 最終レビューで直したこと:
  1. ワークスペースの自動化の一覧で、専用ルールの名前のあとに持ち主のプロジェクトの ID「HH-P1」、削除済みなら「HH-P1(削除済み)」を添える(03 の 464 行)。計画にあった「ルール名にプロジェクトは添えない」は、通知の文の規則(03 の 572 行)の読み違いだった。印は、一覧の API の `scopeLabel`・`scopeDeleted` でサーバーが渡す。
  2. 専用ルールのダイアログ: ワークスペースの自動化のサブタブで編集するときも、ステータスの選択肢をそのプロジェクトが使うものにする。きっかけ・することを変えたときの初期のステータスは、選択肢の先頭にする。保存済みのルールが使わないステータスを指しているとき(ステータスの削除の書き換えなど)は、いまの値として選択肢に残す。サーバーの編集は、変えたきっかけ・することのステータスだけを使うものか確かめる(`automation.Valid` が編集前のルールを受ける)。保存で 404 になったときは、ダイアログの中に「ルールを保存できませんでした」と出す。
  3. ワークスペースのルールを消すと、同じ保存で、プロジェクトの `disabled_rules` からもその ID を除く(`projectrule.RuleDeleteDrafts`)。
  4. ワークスペースの制約が許さない To へのプロジェクトのチェック(ワークスペースの制約を後から変えて残ったもの)は数えず、表ではチェックなしの無効のセルにする(03 の 553 行)。そのチェックだけの行は「ワークスペースと同じ」。消すのは「ワークスペースの設定に戻す」でできる。Go の `projectTargets` と core の `projectTargets` を同じにした。
  5. 使われていない `projectrule.TemplateDeleteImpact`・`ProjectRowRestricted` を消した(上のとおり)。
