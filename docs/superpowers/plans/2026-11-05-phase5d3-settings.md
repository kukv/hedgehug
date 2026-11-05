# フェーズ 5d-3: 設定 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 自動化の設定、プロジェクトの設定、`repo_accounts`、GitHub App の警告、Issue の取り込みと作成をブラウザで動かし、フェーズ5の CLI の操作を GUI でもできるようにする（spec の 5d-3 の完了条件。E2E で確かめる）。

**Architecture:** API は 5d-1 で揃っているので、変えるのは web だけ。文言と判定は `web/core/automations.ts`・`web/core/github.ts` に置き、画面はそれを呼ぶ。読み取りは `web/app/github.svelte.ts` の `useRules`・`useIssueFields`・`useGitHubRepos` に `useRuleStats`・全 Store の読み取りを足す。

**Tech Stack:** Svelte 5、pragmatic-drag-and-drop、vitest、Playwright、fakegh。

**Spec:** `docs/superpowers/specs/2026-11-05-phase5-github-design.md` の「5d の画面」。**キャンバス:** Hedgehug GUI デザインの SettingsAutomations・SettingsAccounts・ProjectSettings・SettingsNav。

**PR:** この plan と引き継ぎは実装の PR に入れる。5d-2 の引き継ぎは直すところがなかった。

## Global Constraints

- コミットの署名は 1Password。失敗したら index に積んで利用者に知らせる
- `make` は `MAKEFLAGS=-j10`。E2E は `make e2e`
- 既存のテストの期待値は、この plan が明示したものの他は変えない（`SettingsNav` の項目が 1 つ増える）
- Go は変えない。変える必要が出たら `go generate ./internal/app/server` と `pnpm gen` を走らせる
- 閲覧のみの Store は `useReadOnlySentence` で書き込みを無効にし、`why` で理由を出す

## この回で決めたこと

| # | 項目 | 決定 | 理由 |
|---|---|---|---|
| 1 | `管理者にインストールを依頼` | `https://<host>/organizations/<owner>/settings/installations` へのリンク。host と owner は repo から取る | App の slug をどこにも持たない。Org の管理者だけが開け、個人 owner では開けない。2026-11-05 利用者が決定 |
| 2 | 5a の手の確認 | この回で一緒に行う。実装の後で手順を示し、結果を引き継ぎに書く | 2026-11-05 利用者が決定 |
| 3 | ルールの編集 | 追加・削除・有効の切り替え。書き換えと並び替えは作らない | API と `hh automation` にない。削除はキャンバスにないが CLI にあるので出す |
| 4 | ルールの追加 | `RuleDialog`。id、イベント、`status` か `category`、`when` は `pr.merged` のときだけ。プロジェクトの設定からは `project` が付く | CLI と同じ項目 |
| 5 | 拒否の注記 | 行き先のステータスの必須項目を `requiredFields` から求めて「完了には points が必要です。…」、拒否があれば「直近 7 日では REL-11 で 1 回拒否しました」 | キャンバス。`stats` の `rejected_tasks` を使う |
| 6 | Issue の同期の表 | 1 フィールド 1 行（6 行）。チェックは即時に `PATCH`。キャンバスの「タイトル・本文」1 行にしない | `push`・`mirror` の値と 1 対 1 にする |
| 7 | マイルストーン | 「タスク側」の列に props のキーの入力。空の間はチェックを無効にする | `props.milestone` が空だと同期されない（`domain/automation/issue.go`） |
| 8 | 取り込みの置き場所 | プロジェクトと、ワークスペースごとの「<名前> のルート」。前者は `parent`、後者は `workspace` | API の 2 つをそのまま選ばせる |
| 9 | 取り込みの部品 | ロジックを `issue-import.svelte.ts`、入力欄を `IssueImportFields.svelte` に分け、ダイアログと設定の両方が使う | ダイアログは `Dialog` の footer にボタンを置き、設定は行の中に置く |
| 10 | プロジェクトの自動化 | 切り替えと追加は即時に書く（1 操作 1 コミット）。draft と `保存` に混ぜない | API が別。`schema.yaml` と 1 コミットにするには Go の変更が要るが、利点がない。キャンバスの「1 コミット」とは違う |
| 11 | プロジェクトの行の状態 | 自分の `disabled_in` にあれば「このプロジェクトで停止」。Store のルールが無効なら「<Store> で無効」（固定）。親のプロジェクトで止まっていれば「親のプロジェクトで停止」（固定）。自分のルールは `有効`（ルール自身の切り替え）と削除 | `?project=` の `enabled` は合成なので、`?project=` なしの一覧と突き合わせる |
| 12 | Issue の作成 | `GitHubSection` に `Issue を作る` → `IssueCreateDialog`。repo はタスクの `repos` が 1 つなら入れておく | キャンバスにないが、CLI の操作を GUI でできるようにする完了条件。CliLine は `hh issue` |
| 13 | `repo_accounts` | ドラッグと `上へ`・`下へ` のボタン、削除、`ルールを追加`。操作のたびに全体を `PUT`。CliLine はなし | CLI がない。ボタンはキーボードの代わり |
| 14 | App の警告 | アカウントの `app_missing` の repo を owner でまとめて出す。`gh のトークンで代わりに使う` は gh のアカウントには出さない | すでに gh を使っている |
| 15 | ナビのバッジ | `アカウントと認証` に App が入っていない repo の数。`sync.state` の `github.app_missing` の合計 | キャンバス。新しい読み取りが要らない |
| 16 | 取得時刻の見出し | `fetchLine` と「レート制限 4,812 / 5,000」。残りが最も少ない repo の値。読めていなければ出さない | キャンバス |

## 5d の表との対応

| spec の行 | Task |
|---|---|
| 自動化の設定（一覧と編集、回数、Issue の同期、取り込み、取得時刻とレート制限） | 2・3・4 |
| プロジェクトの設定（停止、このプロジェクトだけのルール） | 5 |
| アカウントの設定（`repo_accounts`、App の警告、gh のトークン、依頼） | 6 |
| フェーズ5の CLI を GUI で（`hh issue`。`hh gh import` は Task 4） | 7 |
| 完了条件の E2E | 8 |

ほかの行は 5d-2 で済んだ。`hh mv --store` は `MoveScopedChoice` が表示と自動化をまとめて選ぶ。

## File Structure

| ファイル | 役割 |
|---|---|
| `web/core/automations.ts`（新規） | ルールの文言（`setLabel`・`whenLabel`・`ruleYaml`・`statLabel`）、必須の注記、プロジェクトの行の状態、フィールドの表の定義 |
| `web/core/github.ts` | `installUrl`、`missingByOwner`、`rateLine` |
| `web/core/cli.ts` | `cliAutomationAdd`・`Rm`・`Toggle`・`Fields`、`cliIssue`、`cliGhImport` の `workspace` |
| `web/core/messages.ts` | `automation.exists`・`automation.not_found`・`automation.not_project`・`repo_accounts.invalid`・`issue.repo_required` |
| `web/app/router.ts` | `automations` のページ |
| `web/app/github.svelte.ts` | `useRuleStats`、`useAllGitHubRepos` |
| `web/app/settings/SettingsAutomations.svelte`（新規）・`RuleList.svelte`（新規）・`RuleDialog.svelte`（新規）・`IssueSyncTable.svelte`（新規）・`IssueImportSection.svelte`（新規） | 自動化の設定 |
| `web/app/states/issue-import.svelte.ts`（新規）・`IssueImportFields.svelte`（新規）・`IssueImportDialog.svelte` | 取り込み |
| `web/app/project/ProjectAutomations.svelte`（新規）・`ProjectSettings.svelte` | プロジェクトの自動化 |
| `web/app/settings/RepoAccounts.svelte`（新規）・`AppWarning.svelte`（新規）・`SettingsAccounts.svelte`・`AccountCard.svelte`・`SettingsNav.svelte`・`SettingsRoute.svelte` | アカウントとナビ |
| `web/app/detail/IssueCreateDialog.svelte`（新規）・`GitHubSection.svelte` | Issue の作成 |
| `web/e2e/settings-github.spec.ts`（新規） | E2E |

## Tasks

各 Task は失敗するテストを書く → 失敗を確かめる → 実装 → 通す → コミットの順で進める。web は `pnpm exec vitest run <ファイル>`、終わりに `make web-check`。

### Task 1: 土台

- [ ] `router.test.ts`: `/settings/automations/shared` の往復
- [ ] `router.ts`、`SettingsNav`（「自動化と GitHub」）、`SettingsRoute`
- [ ] `messages.ts` のコード（`messages.test.ts`）
- [ ] `cli.ts`（`cli.test.ts`）
- [ ] `github.svelte.ts` に `useRuleStats`・`useAllGitHubRepos`

### Task 2: ルールの一覧

- [ ] `core/automations.test.ts`: 表形式で `setLabel`・`whenLabel`・`ruleYaml`・`statLabel`・`requiredNote`
- [ ] `RuleList.svelte`: チップ、`disabled_in`・`project` のリンク、回数、`有効`、削除、注記
- [ ] `RuleDialog.svelte`: 追加（決定 4）。`automation.exists` をフォームに出す

### Task 3: ページの枠と Issue の同期

- [ ] `core/github.test.ts`: `rateLine`
- [ ] `SettingsAutomations.svelte`: 見出し、取得時刻、レート制限、延長の表示
- [ ] `IssueSyncTable.svelte`: 既定のモード、6 行の表、マイルストーンのキー（決定 6・7）

### Task 4: 取り込み

- [ ] `issue-import.svelte.ts`・`IssueImportFields.svelte` に分ける。`IssueImportDialog.test.ts` は変えずに通す
- [ ] `IssueImportSection.svelte`: 置き場所の選択（決定 8）、`cliGhImport` の `workspace`

### Task 5: プロジェクトの設定

- [ ] `core/automations.test.ts`: `projectRuleRow`（決定 11）
- [ ] `ProjectAutomations.svelte` と `ProjectSettings.svelte` への配置

### Task 6: アカウント

- [ ] `core/github.test.ts`: `installUrl`・`missingByOwner`
- [ ] `RepoAccounts.svelte`: 一覧、ドラッグ、ボタン、追加、削除。並べ替えは pragmatic-drag-and-drop の `reorder`
- [ ] `AppWarning.svelte` と `AccountCard` への配置、`SettingsNav` のバッジ

### Task 7: Issue の作成

- [ ] `IssueCreateDialog.svelte` と `GitHubSection` のボタン

### Task 8: E2E

- [ ] `settings-github.spec.ts`: ルールの追加・停止・削除が `hh automation list` に出る、回数、フィールドの変更が `hh automation fields` に出る、設定からの取り込み、プロジェクトの停止とルールの追加、`repo_accounts` の追加と並べ替えが config に書かれる、App の警告と `gh のトークンで代わりに使う`、タスク詳細からの Issue の作成

### Task 9: 仕上げ

- [ ] `make web-check`、`make check`、`make e2e`
- [ ] 引き継ぎ `docs/superpowers/plans/2026-11-05-phase5d3-handoff.md`
- [ ] 5a の手の確認（決定 2）
- [ ] PR
