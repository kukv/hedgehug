// The screens of phase 5d-2, with the fake of GitHub as github.com:
//   - a task with a pull request shows a chip in the tree, the kanban, the table and マイタスク
//   - the section "GitHub" of a task lists its links; 「リンク」 opens the dialog 「<ID> にリンク」
//   - the tab 「概要」 of a project has the region 「リポジトリと PR」
//   - /inbox?tab=automation has what the rules did; a refused rule offers to fill what is missing
//   - /activity?sync=automation has what GitHub and the rules wrote
import type { Locator, Page } from "@playwright/test";
import { expect, type Hedgehug, LOGIN, type TaskJson, test } from "./harness";

test.use({ github: true });

const STORE = "demo";
const REPO = "org-a/web";
const ME = `github.com/${LOGIN}`;

function connect(hh: Hedgehug): void {
  hh.fakegh("token", "add", "t1", "--user", LOGIN);
  hh.hhIn("t1\n", "auth", "login", LOGIN, "--with-token");
  hh.fakegh("repo", "add", REPO);
  hh.fakegh("perm", "set", REPO, LOGIN, "write");
}

/** Reads GitHub now, past the interval of 5 minutes. */
function fetchGitHub(hh: Hedgehug): void {
  hh.expireFetch(STORE);
  hh.hh("sync");
}

/** A task that names the repository (hh add takes it from the current directory only). */
async function repoTask(hh: Hedgehug, title: string, over: object = {}) {
  const body = { title, repos: [`github.com/${REPO}`], ...over };
  return (await hh.post<{ task: TaskJson }>(`/api/tasks?store=${STORE}`, body)).task;
}

/** A task with a pull request opened on its branch, read from GitHub. */
async function taskWithPr(hh: Hedgehug, title = "Fix the login", over: object = {}) {
  const task = await repoTask(hh, title, over);
  hh.fakegh("pr", "open", REPO, "--branch", `hh/${task.id}-fix`, "--title", "Login の修正");
  hh.hh("sync");
  return task;
}

const chip = (at: Page | Locator, text: RegExp): Locator => at.getByRole("link", { name: text });

test("the chip follows the pull request in the tree, the kanban, the table and マイタスク", async ({
  page,
  hh,
}) => {
  connect(hh);
  const task = await taskWithPr(hh, "Fix the login", { assignees: [ME] });
  hh.hh(
    "view",
    "save",
    "prs",
    "--query",
    "status:todo",
    "--layout",
    "table",
    "--columns",
    "id,title,pr",
  );

  await hh.open(page, await hh.treePath());
  await expect(chip(page, /web#1\s+open/)).toBeVisible();

  // It changes as the page looks at it: the review, then CI, then the draft of another PR.
  hh.fakegh("pr", "review", `${REPO}#1`, "--state", "changes_requested");
  fetchGitHub(hh);
  await expect(chip(page, /web#1\s+修正依頼/)).toBeVisible();
  hh.fakegh("pr", "checks", `${REPO}#1`, "failure");
  fetchGitHub(hh);
  await expect(chip(page, /web#1\s+CI 失敗/)).toBeVisible();

  hh.fakegh("pr", "open", REPO, "--branch", `hh/${task.id}-more`, "--draft");
  fetchGitHub(hh);
  await expect(chip(page, /web#1\s+CI 失敗\s+\+1/)).toBeVisible();

  await page.goto(`${hh.origin}${(await hh.treePath()).replace(/\/tree$/, "/kanban")}`);
  await expect(
    page.locator(`li[data-ulid="${task.ulid}"]`).getByRole("link", { name: /web#1/ }),
  ).toBeVisible();
  await page.goto(`${hh.origin}/views/${STORE}/prs`);
  await expect(chip(page, /web#1/)).toBeVisible();
  await page.goto(`${hh.origin}/my`);
  await expect(chip(page, /web#1/)).toBeVisible();
});

test("the kanban says which columns the rules bring tasks to", async ({ page, hh }) => {
  hh.add("On the board");
  hh.hh("automation", "add", "--id", "opened", "--on", "pr.opened", "--status", "in_progress");
  hh.hh(
    "automation",
    "add",
    "--id",
    "merged",
    "--on",
    "pr.merged",
    "--category",
    "done",
    "--when",
    "all_linked_prs_merged",
  );
  await hh.open(page, (await hh.treePath()).replace(/\/tree$/, "/kanban"));
  const head = (key: string) => page.locator(`.head[data-column="${key}"]`);
  await expect(head("in_progress")).toContainText("PR を開くと自動でここへ");
  await expect(head("done")).toContainText("全 PR のマージで自動でここへ");
  await expect(head("todo")).not.toContainText("自動でここへ");

  // The rule goes: the words go with it, without a reload.
  hh.hh("automation", "rm", "opened");
  await expect(head("in_progress")).not.toContainText("自動でここへ");
});

test("the detail of a task lists its links, and links, changes the mode and takes them off", async ({
  page,
  hh,
}) => {
  connect(hh);
  const task = await taskWithPr(hh);
  hh.fakegh("issue", "open", REPO, "--title", "ログインできない");
  await hh.open(page, `/tasks/${task.ulid}`);

  const section = page.getByRole("region", { name: "GitHub" });
  const row = section.locator(".row").first();
  await expect(row).toContainText("org-a/web#1");
  await expect(row).toContainText("Login の修正");
  await expect(row).toContainText(`hh/${task.id}-fix · ブランチ名から自動リンク`);

  await section.getByRole("button", { name: "リンク", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: `${task.id} にリンク` });
  await dialog.getByRole("button", { name: "Issue" }).click();
  await dialog.getByPlaceholder("org/repo#123").fill(`${REPO}#2`);
  await dialog.getByRole("button", { name: "リンクする" }).click();
  await expect(dialog).toHaveCount(0);

  const mode = section.getByLabel(`同期: ${REPO}#2`);
  await mode.selectOption("mirror");
  await expect
    .poll(() =>
      hh
        .get<{ links: { sync: string }[] }>(`/api/tasks/${task.ulid}`)
        .then((t) => t.links.map((l) => l.sync)),
    )
    .toContain("mirror");

  await section.getByRole("button", { name: `リンクを外す: ${REPO}#2` }).click();
  await expect(section.getByLabel(`同期: ${REPO}#2`)).toHaveCount(0);
});

test("the overview of a project has its pull requests, and says the reads are held back", async ({
  page,
  hh,
}) => {
  connect(hh);
  const project = hh.add("Release", "--kind", "project");
  const task = await repoTask(hh, "Ship it", { parent: project.ulid });
  hh.fakegh("pr", "open", REPO, "--branch", `hh/${task.id}-ship`, "--title", "出荷");
  hh.fakegh("rate", "set", "10", "5000", "--reset", "3600");
  hh.hh("sync");

  await hh.open(page, `/projects/${project.ulid}`);
  await page.getByRole("link", { name: "概要" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${project.ulid}/overview$`));
  const region = page.getByRole("region", { name: "リポジトリと PR" });
  await expect(region.getByRole("row").filter({ hasText: "出荷" })).toContainText(task.id);
  await expect(region).toContainText("org-a/web · PR 1");
  await expect(region).toContainText("GitHub の取得");
  await expect(region.getByRole("status")).toContainText("取得を 15 分おきに延ばしています");

  await region.getByRole("button", { name: "repo を追加" }).click();
  await region.getByLabel("リポジトリ").fill("org-a/ops");
  await region.getByRole("button", { name: "追加する" }).click();
  await expect(region).toContainText("org-a/ops · PR 0");
});

test("a refused rule is told, and the field it needs moves the task", async ({ page, hh }) => {
  connect(hh);
  hh.hh("automation", "add", "--id", "opened", "--on", "pr.opened", "--status", "in_progress");
  hh.hh("automation", "add", "--id", "merged", "--on", "pr.merged", "--category", "done");
  const project = hh.add("Release", "--kind", "project");
  hh.hh("schema", "scope", "set", project.id, "--required", "done=points");
  const task = await taskWithPr(hh, "Ship it", { parent: project.ulid, assignees: [ME] });
  await expect.poll(() => hh.show(task.id).status).toBe("in_progress");

  hh.fakegh("pr", "merge", `${REPO}#1`);
  fetchGitHub(hh);
  expect(hh.show(task.id).status).toBe("in_progress");

  await hh.open(page, "/inbox?tab=automation");
  await expect(page.getByText(/ルール opened が 進行中 にしました（PR が開かれた）/)).toBeVisible();
  await expect(page.getByText(/ルール merged で動かせませんでした/)).toBeVisible();

  await page.getByRole("button", { name: "points を入力して完了にする" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("spinbutton").fill("3");
  await dialog.getByRole("button", { name: "完了にする" }).click();
  await expect.poll(() => hh.show(task.id).status).toBe("done");

  await page.goto(`${hh.origin}/activity?sync=automation`);
  await expect(page.getByText(/自動化を拒否 · ルール merged/)).toBeVisible();
  await expect(page.getByText("PR が開かれた · ルール opened")).toBeVisible();
  await page.goto(`${hh.origin}/tasks/${task.ulid}`);
  await expect(page.getByText("自動化 · PR が開かれた · ルール opened")).toBeVisible();
});

test("an empty project takes in the issues of a label", async ({ page, hh }) => {
  connect(hh);
  hh.fakegh("repo", "label", REPO, "v2");
  hh.fakegh("issue", "open", REPO, "--title", "取り込む Issue");
  hh.fakegh("issue", "edit", `${REPO}#1`, "--label", "v2");
  const project = hh.add("Empty", "--kind", "project");

  await hh.open(page, `/projects/${project.ulid}`);
  await page.getByRole("button", { name: "Issue から取り込む" }).click();
  const dialog = page.getByRole("dialog", { name: "Issue から取り込む" });
  await dialog.getByRole("combobox", { name: "リポジトリ" }).fill(REPO);
  await dialog.getByRole("textbox", { name: "ラベル" }).fill("v2");
  await dialog.getByRole("button", { name: "件数を確かめる" }).click();
  await dialog.getByRole("button", { name: "1 件を mirror で取り込む" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole("treeitem").filter({ hasText: "取り込む Issue" })).toBeVisible();
});

test("the palette copies hh start", async ({ page, hh, context }) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  const task = hh.add("Start me");
  await hh.open(page, await hh.treePath());
  await page.keyboard.press("ControlOrMeta+k");
  await page.getByRole("combobox").fill(task.id);
  await page
    .getByRole("option", { name: new RegExp(`ブランチを作って始める（hh start ${task.id}）`) })
    .click();
  await expect(page.getByRole("status").filter({ hasText: "コピーしました" })).toBeVisible();
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(`hh start ${task.id}`);
});
