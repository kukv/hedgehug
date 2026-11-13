import { expect, type Locator, type Page, type TestInfo, test } from "@playwright/test";
import { createWorkspace, newTaskUnder, prefixOf, tokenUrl, unique } from "./helpers";

// gh は、e2e のサーバーの偽の GitHub に PR・Issue を置く(D3-23)。
async function gh(page: Page, q: Record<string, string>) {
  const res = await page.request.post(`/e2e/github?${new URLSearchParams(q)}`, {
    headers: { Authorization: "Bearer e2e-token" },
  });
  expect(res.status()).toBe(204);
}

// setup は、新しいワークスペース・プロジェクト(リポジトリなし)・Epic・Task を作り、Task の詳細パネルを開く。
async function setup(page: Page, info: TestInfo) {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, `リンク ${prefix}`), prefix);
  const panel = await newTaskUnder(page, info, prefix);
  const repo = `org-${prefix.toLowerCase()}/web`;
  return { prefix, panel, repo, github: panel.getByRole("region", { name: "GitHub" }) };
}

const panelGithub = (page: Page) =>
  page.getByRole("region", { name: "詳細パネル" }).getByRole("region", { name: "GitHub" });

async function link(
  page: Page,
  section: Locator,
  kind: "PR" | "Issue",
  repo: string | null,
  n: string,
) {
  await section.getByRole("button", { name: `${kind} をリンク` }).click();
  const dialog = page.getByRole("dialog", { name: `${kind} をリンク` });
  // repo が null なら、プロジェクトのリポジトリから選ぶ(セレクトが出るのを待つ)
  if (repo === null)
    await expect(dialog.getByRole("combobox", { name: "リポジトリ" })).toBeVisible();
  else await dialog.getByRole("textbox", { name: "リポジトリ" }).fill(repo);
  await dialog.getByRole("textbox", { name: "番号" }).fill(n);
  await dialog.getByRole("button", { name: "リンク" }).click();
  return dialog;
}

test("PR をリンクし、プロジェクトのリポジトリにも追加し、解除する", async ({ page }, info) => {
  const { prefix, github, repo } = await setup(page, info);
  await gh(page, {
    repo,
    number: "41",
    kind: "pr",
    title: "Add login form",
    state: "open",
    review: "pending",
    checks: "failure",
  });
  await expect(github.getByText("リンクはありません")).toBeVisible();
  const dialog = await link(page, github, "PR", repo, "41");
  await expect(dialog).toBeHidden();
  await expect(github.getByRole("link", { name: `${repo}#41 — Add login form` })).toHaveAttribute(
    "href",
    `https://github.com/${repo}/pull/41`,
  );
  await expect(github.getByText("PR Open")).toBeVisible();
  await expect(github.getByText("レビュー待ち")).toBeVisible();
  await expect(github.getByText("CI 失敗")).toBeVisible();
  // プロジェクトのリポジトリに加わり、概要の「PR と Issue」に出る
  await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/overview`));
  await expect(page.getByRole("list", { name: "リポジトリ" }).getByText(repo)).toBeVisible();
  await expect(
    page.getByRole("list", { name: "PR と Issue" }).getByRole("link", { name: `${prefix}-2` }),
  ).toBeVisible();
  // 解除
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  const again = page
    .getByRole("region", { name: "詳細パネル" })
    .getByRole("region", { name: "GitHub" });
  await again.getByRole("button", { name: `${repo}#41 のリンクを解除` }).click();
  await expect(page.getByText("リンクを解除しました")).toBeVisible();
  await expect(again.getByText("リンクはありません")).toBeVisible();
});

test("リンクの断り方(07「追加」の表)", async ({ page }, info) => {
  const { prefix, github, repo } = await setup(page, info);
  await gh(page, { repo, number: "1", kind: "issue", title: "落ちる", state: "open" });
  await gh(page, { repo, number: "2", kind: "pr", title: "直す", state: "draft" });
  let dialog = await link(page, github, "PR", "org/*", "1");
  await expect(dialog.getByRole("alert")).toHaveText(
    "リポジトリは owner/name の形式で入力してください",
  );
  await dialog.getByRole("textbox", { name: "リポジトリ" }).fill(repo);
  await dialog.getByRole("textbox", { name: "番号" }).fill("x");
  await dialog.getByRole("button", { name: "リンク" }).click();
  await expect(dialog.getByRole("alert")).toHaveText("番号を入力してください");
  await dialog.getByRole("textbox", { name: "番号" }).fill("99");
  await dialog.getByRole("button", { name: "リンク" }).click();
  await expect(dialog.getByRole("alert")).toHaveText(`${repo}#99 が見つかりません`);
  await dialog.getByRole("textbox", { name: "番号" }).fill("1");
  await dialog.getByRole("button", { name: "リンク" }).click();
  await expect(dialog.getByRole("alert")).toHaveText(`${repo}#1 は PR ではなく Issue です`);
  await dialog.getByRole("button", { name: "キャンセル" }).click();
  // 1 本目の Issue は付き、プロジェクトにリポジトリができたので 2 回目からは選ぶ
  dialog = await link(page, github, "Issue", repo, "1");
  await expect(dialog).toBeHidden();
  // リンクの直後は、プロジェクトのリポジトリの読み直しが終わっていないことがあるので、開き直してから開く
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  dialog = await link(page, panelGithub(page), "Issue", null, "2");
  await expect(dialog.getByRole("alert")).toHaveText(`${repo}#2 は Issue ではなく PR です`);
  await dialog.getByRole("textbox", { name: "番号" }).fill("1");
  await dialog.getByRole("button", { name: "リンク" }).click();
  await expect(dialog.getByRole("alert")).toHaveText("もうリンクしています");
});

test("開いたときに取り直し、改名を追い、失敗を行に出す", async ({ page }, info) => {
  const { prefix, github, repo } = await setup(page, info);
  await gh(page, { repo, number: "5", kind: "pr", title: "A", state: "open" });
  await expect(await link(page, github, "PR", repo, "5")).toBeHidden();
  // GitHub でマージされ、リポジトリが改名された(旧い名前で引くと新しい名前が返り、新しい名前でも引ける)
  const moved = `${repo}2`;
  await gh(page, { repo, number: "5", kind: "pr", title: "A", state: "merged", moved });
  await gh(page, { repo: moved, number: "5", kind: "pr", title: "A", state: "merged" });
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  const section = panel.getByRole("region", { name: "GitHub" });
  await expect(section.getByRole("link", { name: `${moved}#5 — A` })).toBeVisible();
  await expect(section.getByText("PR Merged")).toBeVisible();
  const history = panel.getByRole("region", { name: "履歴" });
  await expect(history.getByText(`リンクを ${repo}#5 から ${moved}#5 に更新`)).toBeVisible();
  await expect(history.getByText(`${moved}#5 の状態を Merged に変更`)).toBeVisible();
  // 取り直しに失敗した行には理由を添え、保存してある値のまま出す
  await gh(page, { repo: moved, number: "5", gone: "1" });
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  await expect(section.getByRole("alert")).toHaveText(
    `状態を更新できませんでした: ${moved}#5 が見つかりません`,
  );
  await expect(section.getByText("PR Merged")).toBeVisible();
});

test("アカウントが 1 つもなければ、リンクの追加を無効にする", async ({ page }, info) => {
  await page.route("**/api/accounts", (route) => route.fulfill({ json: [] }));
  const { github } = await setup(page, info);
  for (const name of ["PR をリンク", "Issue をリンク"]) {
    const button = github.getByRole("button", { name });
    await expect(button).toBeDisabled();
    await expect(button).toHaveAttribute("title", "GitHub のアカウントを登録すると使えます");
  }
  await expect(github.getByRole("button", { name: "PR 用の印をコピー" })).toBeEnabled();
});
