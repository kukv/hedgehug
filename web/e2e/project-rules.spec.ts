import { expect, test } from "./fixtures";
import {
  createWorkspace,
  newProject,
  newTaskUnder,
  openSettings,
  prefixOf,
  projectWithEpic,
  treeRow,
  unique,
} from "./helpers";
import {
  createOwnRule,
  createTemplate,
  goTree,
  section,
  settingsOf,
  statusButton,
} from "./project-rules-helpers";

// プロジェクト別のルールの通しのテスト(B5a)。ワークスペースは毎回作る(サンプルは同じ worker のほかのテストが使う)。

test("unchecking a status hides it from the tree's status menu without a reload", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "使うステータス"), prefix);
  await newTaskUnder(page, info, prefix);
  await settingsOf(page, prefix);
  const statuses = section(page, "rules-statuses");
  await expect(statuses.locator("#rules-statuses")).toContainText("5 / 5");

  // 初期ステータスで未着手の既定でもある Todo は、外せない。
  const todo = statuses.getByRole("checkbox", { name: "Todo", exact: true });
  await expect(todo).toBeChecked();
  await expect(todo).toBeDisabled();
  await expect(statuses).toContainText("初期ステータスで、未着手の既定でもあるため外せません");

  await statuses.getByRole("checkbox", { name: "Pending", exact: true }).click();
  await expect(statuses.locator("#rules-statuses")).toContainText("4 / 5");
  await expect(statuses.getByRole("checkbox", { name: "Pending", exact: true })).not.toBeChecked();

  // 画面を読み直さずにツリーへ移り、Task のステータスのメニューを開く。
  const tree = await goTree(page);
  await statusButton(treeRow(tree, `${prefix}-2`)).click();
  const menu = page.getByRole("menu", { name: "ステータス" });
  await expect(menu.getByRole("menuitemcheckbox", { name: /^Done/ })).toBeEnabled();
  await expect(menu.getByRole("menuitemcheckbox", { name: /^Pending/ })).toHaveCount(0);
});

test("a project workflow restriction disables a status in the menu with its reason", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "プロジェクトの制約"), prefix);
  await newTaskUnder(page, info, prefix);
  await settingsOf(page, prefix);
  const workflow = section(page, "rules-workflow");
  const cell = workflow.getByRole("checkbox", { name: "Todo から InProgress", exact: true });
  await cell.click();
  await expect(cell).toBeChecked();
  await expect(workflow).toContainText("制限あり");

  const tree = await goTree(page);
  await statusButton(treeRow(tree, `${prefix}-2`)).click();
  const menu = page.getByRole("menu", { name: "ステータス" });
  await expect(menu.getByRole("menuitemcheckbox", { name: /^Done/ })).toBeDisabled();
  await expect(menu).toContainText(
    "ワークフローの制約: Todo から Done には変更できません(InProgress のみ)",
  );
});

test("using one template leaves only it and none in the create dialog", async ({ page }, info) => {
  const prefix = prefixOf(info);
  const a = unique(info, "雛形A");
  const b = unique(info, "雛形B");
  await createWorkspace(page, info, unique(info, "使うテンプレート"), prefix);
  await createTemplate(page, prefix, a);
  await createTemplate(page, prefix, b);
  const panel = await projectWithEpic(page, info, prefix);

  // ルールなしでは、作成ダイアログの選択肢は 2 つのテンプレートと「なし」。
  const options = () =>
    page
      .getByRole("dialog", { name: "新規作成" })
      .getByRole("combobox", { name: "テンプレート" })
      .locator("option");
  await panel.getByRole("button", { name: "Task" }).click();
  await expect
    .poll(async () => (await options().allTextContents()).sort())
    .toEqual([a, b, "なし"].sort());
  await page.keyboard.press("Escape");

  await settingsOf(page, prefix);
  const templates = section(page, "rules-templates");
  await templates.getByRole("checkbox", { name: a, exact: true }).click();
  await expect(templates.getByRole("checkbox", { name: a, exact: true })).toBeChecked();

  await page.getByRole("tab", { name: "ツリー", exact: true }).click();
  await page.getByRole("button", { name: "Epic", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await dialog.getByRole("combobox", { name: "種類" }).selectOption({ label: "Task" });
  await expect
    .poll(async () => (await options().allTextContents()).sort())
    .toEqual([a, "なし"].sort());
});

test("a project-only rule shows in the workspace automation tab and its line opens the section", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "専用ルール"), prefix);
  await newProject(page, info, prefix);
  await settingsOf(page, prefix);
  await section(page, "rules-statuses")
    .getByRole("checkbox", { name: "Pending", exact: true })
    .click();
  await expect(page.locator("#rules-statuses")).toContainText("4 / 5");
  await createOwnRule(page, "着手で外す", async (dialog) => {
    await dialog
      .getByRole("combobox", { name: "きっかけ" })
      .selectOption({ label: "ステータスが次になったとき" });
    await dialog
      .getByRole("combobox", { name: "ステータス" })
      .selectOption({ label: "InProgress" });
    await dialog
      .getByRole("combobox", { name: "すること" })
      .selectOption({ label: "担当者を変更" });
    await expect(dialog.getByRole("combobox", { name: "値" })).toHaveValue("@none");
  });
  const rule = section(page, "rules-automation");
  await expect(rule).toContainText("ステータスが InProgress になったとき → 担当者を 未割り当て に");
  await expect(rule).toContainText("このプロジェクトだけ");

  // ワークスペースの設定の自動化のサブタブにも出て、上の 1 行から、プロジェクトの設定タブのその区画へ移れる。
  await openSettings(page, prefix, "自動化");
  await expect(
    page.getByText("ステータスが InProgress になったとき → 担当者を 未割り当て に"),
  ).toBeVisible();
  // 専用ルールの行には、名前のあとに持ち主のプロジェクトの ID を添える。
  const row = page
    .locator(".row")
    .filter({ has: page.getByRole("switch", { name: "着手で外す を有効にする" }) });
  await expect(row).toContainText(`${prefix}-P1`);
  // ここで編集しても、ステータスの選択肢はそのプロジェクトが使うものだけ。
  await row.getByRole("button", { name: "編集" }).click();
  const edit = page.getByRole("dialog", { name: "ルールの編集" });
  await expect(edit.getByRole("combobox", { name: "ステータス" }).locator("option")).toHaveText([
    "Todo",
    "InProgress",
    "Done",
    "Cancel",
  ]);
  await edit.getByRole("button", { name: "キャンセル" }).click();
  const title = unique(info, `計画 ${prefix} 1`);
  const line = page.getByTestId("rules-projects-line");
  await expect(line).toHaveText(
    `プロジェクト別のルールがあるプロジェクト: ${prefix}-P1 ${title}(1 件)`,
  );
  await line.getByRole("button", { name: `${prefix}-P1 ${title}` }).click();
  await expect(page).toHaveURL(new RegExp(`/p/${prefix}-P1/settings`));
  await expect(page.locator("#rules-automation")).toBeFocused();
});

test("the statuses subtab lists the projects with rules and opens the statuses section", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "ステータスの 1 行"), prefix);
  const t1 = await newProject(page, info, prefix, 1);
  const t2 = await newProject(page, info, prefix, 2);
  const label = "プロジェクト別のルールがあるプロジェクト: ";

  await openSettings(page, prefix, "ステータス");
  await expect(page.getByTestId("rules-projects-line")).toHaveCount(0);

  await settingsOf(page, prefix, 1);
  await section(page, "rules-statuses")
    .getByRole("checkbox", { name: "Pending", exact: true })
    .click();
  await expect(page.locator("#rules-statuses")).toContainText("4 / 5");
  await openSettings(page, prefix, "ステータス");
  const line = page.getByTestId("rules-projects-line");
  await expect(line).toHaveText(`${label}${prefix}-P1 ${t1}(1 件)`);

  await settingsOf(page, prefix, 2);
  await section(page, "rules-statuses")
    .getByRole("checkbox", { name: "Pending", exact: true })
    .click();
  await expect(page.locator("#rules-statuses")).toContainText("4 / 5");
  await openSettings(page, prefix, "ステータス");
  await expect(line).toHaveText(`${label}${prefix}-P1 ${t1}・${prefix}-P2 ${t2}(2 件)`);

  await line.getByRole("button", { name: `${prefix}-P2 ${t2}` }).click();
  await expect(page).toHaveURL(new RegExp(`/p/${prefix}-P2/settings`));
  await expect(page.locator("#rules-statuses")).toBeFocused();
});
