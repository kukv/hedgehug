import type { Locator, Page, TestInfo } from "@playwright/test";
import { expect, test } from "./fixtures";
import {
  addRepo,
  createWorkspace,
  githubPull,
  githubRead,
  githubRepo,
  newTaskUnder,
  openSettings,
  prefixOf,
  tokenUrl,
  unique,
} from "./helpers";

// PR の自動リンク(07「PR の自動リンク」)。偽の GitHub に PR を置き(/e2e/github-pull)、プロジェクトにリポジトリを
// 足すと、裏ですぐに探してリンクする(D4b-9)。2 回目からは /e2e/github-read の読み取りで探す。

// setup は、新しいワークスペース・プロジェクト(リポジトリなし)・Epic(接頭辞-1)・Task(接頭辞-2)を作る。
async function setup(page: Page, info: TestInfo) {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, `自動リンク ${prefix}`), prefix);
  await newTaskUnder(page, info, prefix);
  return { prefix, id: `${prefix}-2`, repo: `org-${prefix.toLowerCase()}/web` };
}

// openItem は、アイテムの詳細パネルを開き、パネルと GitHub の欄を返す。
async function openItem(page: Page, id: string) {
  await page.goto(tokenUrl(`/s/e2e/i/${id}`));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  return { panel, github: panel.getByRole("region", { name: "GitHub" }) };
}

// autoLinkedBy は、詳細パネルの履歴の「{repo}#{n} を自動でリンク」の行の、実行者の欄。
function autoLinkedBy(panel: Locator, ref: string) {
  return panel
    .getByRole("region", { name: "履歴" })
    .getByRole("listitem")
    .filter({ hasText: `${ref} を自動でリンク` })
    .locator(".who");
}

test("リポジトリを足すと PR が自動でリンクされる", async ({ page }, info) => {
  const { prefix, id, repo } = await setup(page, info);
  await githubPull(page.request, {
    repo,
    number: "12",
    branch: `hh/${id}-login`,
    title: "ログイン画面",
    state: "open",
  });
  await addRepo(page, prefix, repo);
  const { panel, github } = await openItem(page, id);
  await expect(github.getByRole("link", { name: `${repo}#12 — ログイン画面` })).toBeVisible();
  await expect(github.getByText("PR Open")).toBeVisible();
  // アクティビティの文は「{ID} に {repo}#{n} を自動でリンク」。アイテムの履歴では、そのアイテムの「{ID} に」を出さない
  await expect(autoLinkedBy(panel, `${repo}#12`)).toHaveText("GitHub");
});

test("2 回目からはマージのルールが動く", async ({ page }, info) => {
  const { prefix, id, repo } = await setup(page, info);
  await openSettings(page, prefix, "自動化");
  await page.getByRole("button", { name: "ルールを作成" }).click();
  const dialog = page.getByRole("dialog", { name: "ルールを作成" });
  await dialog.getByRole("textbox", { name: "名前" }).fill("マージで完了");
  await dialog
    .getByRole("combobox", { name: "きっかけ" })
    .selectOption({ label: "リンクした PR がマージされたとき" });
  await dialog
    .getByRole("combobox", { name: "すること" })
    .selectOption({ label: "ステータスを変更" });
  await dialog.getByRole("combobox", { name: "値" }).selectOption({ label: "Done" });
  await dialog.getByRole("button", { name: "保存" }).click();
  await expect(dialog).toBeHidden();
  // 初めて探すときは、マージ済みの PR を付けても、きっかけを起こさない(D4b-7)
  await githubPull(page.request, {
    repo,
    number: "1",
    branch: `hh/${id}`,
    title: "一本目",
    state: "merged",
  });
  await addRepo(page, prefix, repo);
  await expect(
    (await openItem(page, id)).github.getByRole("link", { name: `${repo}#1 — 一本目` }),
  ).toBeVisible();
  // リンクを書いた保存のあとで開き直し、その保存の履歴の行が出てからステータスを見る(読み直す前の Todo を見ない)
  const { panel, github } = await openItem(page, id);
  const props = panel.getByRole("region", { name: "プロパティ" });
  await expect(autoLinkedBy(panel, `${repo}#1`)).toHaveText("GitHub");
  await expect(props.getByRole("button", { name: "ステータス: Todo" })).toBeVisible();
  // 2 回目からは、付けたマージ済みの PR でルールが動く(タイトルの ID で付く)
  await githubPull(page.request, {
    repo,
    number: "2",
    title: `${id} 二本目`,
    state: "merged",
  });
  await githubRead(page.request);
  await expect(github.getByRole("link", { name: `${repo}#2 — ${id} 二本目` })).toBeVisible();
  await expect(props.getByRole("button", { name: "ステータス: Done" })).toBeVisible();
});

test("探せなかった理由が概要に出る", async ({ page }, info) => {
  const { prefix, repo } = await setup(page, info);
  const section = await addRepo(page, prefix, repo);
  const failed = `PR を探せませんでした: ${repo} を読めません。リポジトリの権限と、使うアカウントを確かめてください`;
  await githubRepo(page.request, repo, true);
  await githubRead(page.request);
  await expect(section.getByText(failed)).toBeVisible();
  await githubRepo(page.request, repo, false);
  await githubRead(page.request);
  await expect(section.getByText(failed)).toBeHidden();
});
