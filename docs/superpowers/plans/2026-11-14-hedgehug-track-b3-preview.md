# トラック B3 の残り(「一緒に作るもの」のプレビューの担当者) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** アイテムの作成ダイアログの「一緒に作るもの」で、子の担当者を、保存のとき(`workspace.TemplateValuesFor`)と同じ規則で出す。担当者のプロパティが非表示か、子の種類で使わないときは、子とその子の「親と同じ」の Step を「未割り当て」と出す。

**Architecture:** core の `templateStructure`(`web/packages/core/src/features/workspaces/templateApply.ts`)に `settings` を渡し、子の値 `assignee` を、プロパティ `assignee` があり・子の種類で使い・表示中のときだけ使う。呼び出し元は `structureOf`(`web/packages/core/src/features/items/createTemplate.ts`)の 1 か所。サーバーは変えない。

**Tech Stack:** TypeScript(core)、Vitest。

**Spec:** `docs/superpowers/specs/2026-11-11-hedgehug-functional-spec/02-items-projects.md`(「ダイアログの項目」の「一緒に作るもの」)、`03-workspace-settings.md`(「適用するときの規則」の「表示中でなくなったプロパティや、その種類で使わないプロパティは適用しない」)。

## Global Constraints

- 依存は追加しない。
- TS ファイル 300 行まで。
- TDD(失敗するテスト → 失敗を確かめる → 実装 → 通す → コミット)。
- コミットの末尾に `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`。
- PR は 1 本(develop 向け、ブランチ `feat/track-b3-preview`)。push は HTTPS(`https://github.com/kukv/hedgehug.git`)。
- 確認のコマンド: `pnpm -C web test`・`pnpm -C web check`。

## 決めたこと

| # | 決めたこと | 根拠 |
|---|---|---|
| B3P-1 | 子の担当者は、テンプレートの子の値 `assignee` のうち、`settings.properties` に ID `assignee` のプロパティがあり、その `kinds` に子の種類があり、`hidden` でないときだけ使う。ほかは「未割り当て」。空文字も「未割り当て」 | サーバーの `TemplateValuesFor`(`keepValues` は `slices.Contains(p.Kinds, kind)`・`HoldsTemplateValue`・空でない値、続けて `!p.Hidden`。担当者は `validValue` で常に正しい) |
| B3P-2 | 子の Step の「親と同じ」は、B3P-1 で決めた子の担当者を使う | サーバーの `expander.steps` は親のアイテムの担当者(`applyValues` のあと)を使う |
| B3P-3 | もとになるアイテムの Step の「親と同じ」と、Step の担当者の直接の指定は、いまのまま | サーバーはもとになるアイテムに入力の値を使い、Step の直接の指定を絞り込まない |

## Task 1: プレビューの子の担当者を保存と同じ規則で出す

**Files:**
- Modify: `web/packages/core/src/features/workspaces/templateApply.ts`(`templateStructure` に `settings: WorkspaceSettings` を足す)
- Modify: `web/packages/core/src/features/items/createTemplate.ts`(`structureOf` から `settings` を渡す)
- Test: `web/packages/core/src/features/workspaces/templateApply.test.ts`、`web/packages/core/src/features/items/createTemplate.test.ts`

- [ ] **Step 1: 失敗するテストを書く** — `templateApply.test.ts` の `templateStructure` に、Epic のテンプレート(子 Story「QA」の値 `assignee: "hanako"`、その下の Step「確認」が `@parent`)で、(a) プロパティ `assignee` が `hidden: true`、(b) プロパティ `assignee` の `kinds` に `Story` がない、のどちらでも、子「QA」と Step「確認」が「未割り当て」になるテストを足す。既存のテストは、担当者が全種類で表示中の設定を渡す形に直す。
- [ ] **Step 2: 失敗を確かめる** — `pnpm -C web test -- templateApply` で新しいテストが落ちる。
- [ ] **Step 3: 実装する** — B3P-1〜3 のとおり。
- [ ] **Step 4: 通す** — `pnpm -C web test`・`pnpm -C web check`。
- [ ] **Step 5: コミット** — `fix(web): 「一緒に作るもの」の子の担当者を、保存と同じく非表示・その種類で使わない値を捨てて出す`
