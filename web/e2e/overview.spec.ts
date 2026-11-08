// Screens this spec relies on (see screens.ts for the shared list):
//   - /workspaces/<store>/<ulid> is the overview of a workspace: the text "<n> 件 · プロジェクト <m>"
//     (every task that is not deleted, projects included), the legend "未着手 <n>", "進行中 <n>",
//     "完了 <n>", "中止 <n>", and the region "プロジェクト" (a row for each project: its ID, title
//     and progress "<done>/<total>"); the root holds only projects, so nothing else is listed
//   - the button "プロジェクトを作成" opens the create dialog of a project for this workspace (there is no
//     button for a task: the root holds only projects)
//   - the overview follows the store: a change the CLI makes changes the counts and the lists
import { expect, test } from "./harness";
import { toast } from "./screens";

type Workspace = { name: string; ulid: string };

test("the overview counts the tasks by category and lists the projects", async ({ page, hh }) => {
  const project = hh.add("Release prep", "--kind", "project");
  const notes = hh.add("Write notes", "--parent", project.id);
  hh.add("Cut the tag", "--parent", project.id);
  hh.hh("set", notes.id, "status=done");
  const other = hh.add("Other project", "--kind", "project");
  const busy = hh.add("Busy alone", "--kind", "project");
  hh.hh("set", busy.id, "status=in_progress");
  const [ws] = hh.json<Workspace[]>("workspace", "ls");
  if (!ws) throw new Error("the store has no workspace");

  await hh.open(page, `/workspaces/demo/${ws.ulid}`);

  // three projects and the two children of the first
  await expect(page.getByText("5 件 · プロジェクト 3")).toBeVisible();
  await expect(page.getByText("未着手 3", { exact: true })).toBeVisible();
  await expect(page.getByText("進行中 1", { exact: true })).toBeVisible();
  await expect(page.getByText("完了 1", { exact: true })).toBeVisible();
  await expect(page.getByText("中止 0", { exact: true })).toBeVisible();

  const projects = page.getByRole("region", { name: "プロジェクト", exact: true });
  const projectRow = projects.getByRole("row", { name: new RegExp(project.id) });
  await expect(projectRow).toContainText("Release prep");
  await expect(projectRow).toContainText("1/2");

  await expect(projects.getByRole("row", { name: new RegExp(other.id) })).toContainText(
    "Other project",
  );
  await expect(projects.getByRole("row", { name: new RegExp(busy.id) })).toBeVisible();

  // the children of the project are not rows of the list of projects
  await expect(projects.getByText("Write notes")).toHaveCount(0);
});

test("the overview follows what the CLI changes", async ({ page, hh }) => {
  const project = hh.add("Release prep", "--kind", "project");
  const first = hh.add("First", "--parent", project.id);
  hh.add("Second", "--parent", project.id);
  const [ws] = hh.json<Workspace[]>("workspace", "ls");
  if (!ws) throw new Error("the store has no workspace");

  await hh.open(page, `/workspaces/demo/${ws.ulid}`);
  await expect(page.getByText("3 件 · プロジェクト 1")).toBeVisible();
  const projects = page.getByRole("region", { name: "プロジェクト", exact: true });
  await expect(projects.getByRole("row", { name: new RegExp(project.id) })).toContainText("0/2");

  hh.hh("set", first.id, "status=done");
  await expect(projects.getByRole("row", { name: new RegExp(project.id) })).toContainText("1/2");
  await expect(page.getByText("完了 1", { exact: true })).toBeVisible();

  const added = hh.add("Added from the terminal"); // at the root: a project
  await expect(page.getByText("4 件 · プロジェクト 2")).toBeVisible();
  await expect(projects.getByRole("row", { name: new RegExp(added.id) })).toBeVisible();

  hh.hh("rm", added.id);
  await expect(page.getByText("3 件 · プロジェクト 1")).toBeVisible();
  await expect(projects.getByRole("row", { name: new RegExp(added.id) })).toHaveCount(0);
});

test("the create button makes a project in this workspace, and it shows in the list", async ({
  page,
  hh,
}) => {
  hh.add("Already here");
  const [ws] = hh.json<Workspace[]>("workspace", "ls");
  if (!ws) throw new Error("the store has no workspace");

  await hh.open(page, `/workspaces/demo/${ws.ulid}`);
  await expect(page.getByText("1 件 · プロジェクト 1")).toBeVisible();
  // the root holds only projects: there is one button, for a project
  await expect(page.getByRole("button", { name: "タスクを作成", exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "プロジェクトを作成", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "プロジェクトを作成" });
  await expect(dialog).toBeVisible();
  await dialog.getByRole("textbox", { name: "名前" }).fill("From the overview");
  await dialog.getByRole("button", { name: "作成", exact: true }).click();

  await expect(toast(page, "DM-2 を作成しました")).toBeVisible();
  await expect(page.getByText("2 件 · プロジェクト 2")).toBeVisible();
  const projects = page.getByRole("region", { name: "プロジェクト", exact: true });
  await expect(projects.getByRole("row", { name: /DM-2/ })).toContainText("From the overview");
  expect(hh.show("DM-2").kind).toBe("project");
});

test("a workspace with no task says so", async ({ page, hh }) => {
  const [ws] = hh.json<Workspace[]>("workspace", "ls");
  if (!ws) throw new Error("the store has no workspace");

  await hh.open(page, `/workspaces/demo/${ws.ulid}`);
  await expect(page.getByText("demo にはまだタスクがありません")).toBeVisible();

  hh.add("The first one");
  await expect(page.getByText("1 件 · プロジェクト 1")).toBeVisible();
});
