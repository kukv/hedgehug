// The settings of phase 5d-3, with the fake of GitHub as github.com:
//   - /settings/automations/<store> has the rules, the issue sync and taking in issues
//   - the settings of a project stop a rule of the Store there, and add one of its own
//   - /settings/accounts has repo_accounts and the warning of the GitHub App
//   - the section "GitHub" of a task opens an issue
import { expect, type Hedgehug, LOGIN, type TaskJson, test } from "./harness";

test.use({ github: true });

const STORE = "demo";
const REPO = "org-a/web";
const SETTINGS = `/settings/automations/${STORE}`;

type Rule = { id: string; enabled: boolean; disabled_in: { ulid: string }[] };
const rules = (hh: Hedgehug) => hh.json<Rule[]>("automation", "list");
const fields = (hh: Hedgehug) =>
  hh.json<{ default_sync: string; push: string[]; mirror: string[] }>("automation", "fields");

function connect(hh: Hedgehug): void {
  hh.fakegh("token", "add", "t1", "--user", LOGIN);
  hh.hhIn("t1\n", "auth", "login", LOGIN, "--with-token");
  hh.fakegh("repo", "add", REPO);
  hh.fakegh("perm", "set", REPO, LOGIN, "write");
}

async function repoTask(hh: Hedgehug, title: string) {
  const body = { title, repos: [`github.com/${REPO}`] };
  return (await hh.post<{ task: TaskJson }>(`/api/tasks?store=${STORE}`, body)).task;
}

test("the rules are added, counted, turned off and removed", async ({ page, hh }) => {
  connect(hh);
  await hh.open(page, await hh.treePath());
  await page.goto(`${hh.origin}/settings/general`);
  await page.getByRole("link", { name: "自動化と GitHub" }).click();
  await expect(page).toHaveURL(new RegExp(`${SETTINGS}$`));

  await page.getByRole("button", { name: "ルールを追加" }).click();
  const dialog = page.getByRole("dialog", { name: "ルールを追加" });
  await dialog.getByRole("textbox", { name: "id" }).fill("opened");
  await dialog.getByLabel("遷移先").selectOption("status:in_progress");
  await dialog.getByRole("button", { name: "追加" }).click();
  await expect(dialog).toHaveCount(0);
  const rule = page.getByRole("article", { name: "opened" });
  await expect(rule).toContainText("ステータス → In progress");
  await expect(rule).toContainText("直近 7 日 0 回");
  expect(rules(hh).map((r) => r.id)).toEqual(["opened"]);

  // A rule that ran counts, without a reload.
  const task = await repoTask(hh, "Ship it");
  hh.fakegh("pr", "open", REPO, "--branch", `hh/${task.id}-ship`);
  hh.hh("sync");
  await expect(rule).toContainText("直近 7 日 1 回");
  await expect(page.getByText(/GitHub から取得: /)).toBeVisible();
  await expect(page.getByText(/レート制限 [\d,]+ \/ [\d,]+/)).toBeVisible();

  await rule.getByRole("checkbox", { name: "有効: opened" }).uncheck();
  await expect(rule).toContainText("無効");
  expect(rules(hh)[0]?.enabled).toBe(false);

  await rule.getByRole("button", { name: "削除: opened" }).click();
  await page
    .getByRole("dialog", { name: "ルール opened を削除" })
    .getByRole("button", { name: "削除" })
    .click();
  await expect(rule).toHaveCount(0);
  expect(rules(hh)).toEqual([]);
});

test("a rule says what can refuse it", async ({ page, hh }) => {
  const project = hh.add("Release", "--kind", "project");
  hh.hh("schema", "scope", "set", project.id, "--required", "done=points");
  hh.hh(
    "automation",
    "add",
    "--id",
    "merged",
    "--on",
    "pr.merged",
    "--category",
    "done",
    "--project",
    project.id,
  );
  await hh.open(page, SETTINGS);
  await expect(page.getByRole("article", { name: "merged" })).toContainText(
    "Doneには ポイント が必要です",
  );
});

test("the issue sync is set here", async ({ page, hh }) => {
  await hh.open(page, SETTINGS);
  await page.getByRole("radio", { name: /mirror/ }).check();
  await expect.poll(() => fields(hh).default_sync).toBe("mirror");
  await page.getByRole("checkbox", { name: "mirror: 本文" }).check();
  await expect.poll(() => fields(hh).mirror).toContain("body");

  const milestone = page.getByRole("checkbox", { name: "push: マイルストーン" });
  await expect(milestone).toBeDisabled();
  await page.getByLabel("マイルストーンのプロパティ").fill("target");
  await page.getByLabel("マイルストーンのプロパティ").blur();
  await expect(milestone).toBeEnabled();
  await milestone.check();
  await expect.poll(() => fields(hh).push).toContain("milestone");
});

test("issues are taken in under a project, and without one there is no place to take them", async ({
  page,
  hh,
}) => {
  connect(hh);
  hh.fakegh("repo", "label", REPO, "v2");
  hh.fakegh("issue", "open", REPO, "--title", "取り込む Issue");
  hh.fakegh("issue", "edit", `${REPO}#1`, "--label", "v2");
  await hh.open(page, SETTINGS);

  const region = page.getByRole("region", { name: "既存の Issue をまとめて取り込む" });
  await region.getByRole("combobox", { name: "リポジトリ" }).fill(REPO);
  await region.getByRole("textbox", { name: "ラベル" }).fill("v2");
  // the root holds only projects: with none, there is nowhere to take the issues in
  await expect(region.getByText("取り込み先のプロジェクトがありません")).toBeVisible();
  await expect(region.getByRole("button", { name: "件数を確認する" })).toBeDisabled();

  const project = hh.add("Release", "--kind", "project"); // the page follows the store
  await expect(region.getByText("取り込み先のプロジェクトがありません")).toHaveCount(0);
  await region.getByRole("button", { name: "件数を確認する" }).click();
  await region.getByRole("button", { name: "1 件を mirror で取り込む" }).click();
  await expect(page.getByRole("status").filter({ hasText: "1 件を取り込みました" })).toBeVisible();
  expect(hh.hh("ls")).toContain("取り込む Issue");
  const imported = hh.json<{ tasks: { title: string; parent: string | null }[] }>("ls").tasks;
  expect(imported.find((t) => t.title === "取り込む Issue")?.parent).toBe(project.ulid);
});

test("a project stops a rule of the Store and has its own", async ({ page, hh }) => {
  hh.hh("automation", "add", "--id", "opened", "--on", "pr.opened", "--status", "in_progress");
  const project = hh.add("Release", "--kind", "project");
  await hh.open(page, `/projects/${project.ulid}/settings`);

  await page.getByRole("checkbox", { name: "このプロジェクトで有効: opened" }).uncheck();
  await expect(
    page.getByRole("checkbox", { name: "このプロジェクトで無効: opened" }),
  ).toBeVisible();
  expect(rules(hh)[0]?.disabled_in.map((p) => p.ulid)).toEqual([project.ulid]);

  await page.getByRole("button", { name: "このプロジェクトだけのルールを追加" }).click();
  const dialog = page.getByRole("dialog", { name: "このプロジェクトだけのルールを追加" });
  await dialog.getByRole("textbox", { name: "id" }).fill("own");
  await dialog.getByLabel("遷移先").selectOption("status:in_progress");
  await dialog.getByRole("button", { name: "追加" }).click();
  await expect(page.getByText("このプロジェクトだけ", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "削除: own" }).click();
  await expect(page.getByText("このプロジェクトだけ", { exact: true })).toHaveCount(0);
});

test("repo_accounts is edited and ordered, by the drag too", async ({ page, hh }) => {
  connect(hh);
  await hh.open(page, "/settings/accounts");
  const region = page.getByRole("region", { name: "GitHub 連携で使うアカウント" });

  const add = async (match: string) => {
    await region.getByRole("button", { name: "ルールを追加" }).click();
    await region.getByLabel("リポジトリ").fill(match);
    await region.getByLabel("アカウント").selectOption(LOGIN);
    await region.getByRole("button", { name: "追加", exact: true }).click();
    await expect(region.getByText(match, { exact: true })).toBeVisible();
  };
  await add("github.com/org-a/*");
  await add("*");

  const order = async () =>
    (await hh.get<{ rules: { match: string }[] }>("/api/repo-accounts")).rules.map((r) => r.match);
  await region.getByRole("button", { name: "上へ: *" }).click();
  await expect.poll(order).toEqual(["*", "github.com/org-a/*"]);

  await region
    .getByRole("listitem")
    .nth(1)
    .dragTo(region.getByRole("listitem").nth(0), { targetPosition: { x: 40, y: 2 } });
  await expect.poll(order).toEqual(["github.com/org-a/*", "*"]);

  await region.getByRole("button", { name: "削除: *" }).click();
  await expect.poll(order).toEqual(["github.com/org-a/*"]);
});

test("the GitHub App that is not installed is told on the account", async ({ page, hh }) => {
  connect(hh);
  await repoTask(hh, "Read me");
  hh.fakegh("repo", "uninstall", "org-a");
  hh.hh("sync");

  await hh.open(page, "/settings/accounts");
  await expect(page.getByRole("link", { name: /アカウントと認証/ })).toContainText("1");
  const warn = page.getByRole("article", { name: LOGIN }).getByRole("note");
  await expect(warn).toContainText("org-a/web");
  await expect(warn.getByRole("link", { name: "管理者にインストールを依頼" })).toHaveAttribute(
    "href",
    `https://github.com/organizations/org-a/settings/installations`,
  );

  await warn.getByRole("button", { name: "代わりに gh のトークンを使う" }).click();
  await expect(
    page.getByRole("status").filter({ hasText: "次の取得から gh のトークンを使います" }),
  ).toBeVisible();
  await expect
    .poll(async () => (await hh.get<{ token?: string }[]>("/api/accounts"))[0]?.token)
    .toBe("gh");
});

test("a task opens an issue", async ({ page, hh }) => {
  connect(hh);
  const task = await repoTask(hh, "Report it");
  await hh.open(page, `/tasks/${task.ulid}`);

  const section = page.getByRole("region", { name: "GitHub" });
  await section.getByRole("button", { name: "Issue を作成" }).click();
  const dialog = page.getByRole("dialog", { name: "Issue を作成" });
  await expect(dialog.getByRole("combobox", { name: "リポジトリ" })).toHaveValue(REPO);
  await dialog.getByRole("button", { name: "作る" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(section.getByLabel(`同期: ${REPO}#1`)).toHaveValue("push");
});
