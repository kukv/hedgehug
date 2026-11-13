import { expect, test } from "./fixtures";
import {
  createWorkspace,
  newProject,
  openSettings,
  prefixOf,
  projectWithEpic,
  tokenUrl,
  treeRow,
  unique,
} from "./helpers";
import {
  createOwnRule,
  createTemplate,
  readOnly,
  section,
  settingsOf,
  statusButton,
} from "./project-rules-helpers";

// プロジェクト別のルールの通しのテスト(B5a)のつづき。ワークスペースは毎回作る。

test("unchecking a status a project-only rule uses is refused and reverts", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "外せない"), prefix);
  await newProject(page, info, prefix);
  await settingsOf(page, prefix);
  await createOwnRule(page, "保留で外す", async (dialog) => {
    await dialog
      .getByRole("combobox", { name: "きっかけ" })
      .selectOption({ label: "ステータスが次になったとき" });
    await dialog.getByRole("combobox", { name: "ステータス" }).selectOption({ label: "Pending" });
  });

  const statuses = section(page, "rules-statuses");
  const pending = statuses.getByRole("checkbox", { name: "Pending", exact: true });
  await pending.click();
  await expect(
    page.getByText(
      "「Pending」はこのプロジェクトだけのルール「保留で外す」が使っているため、使わないことにできません。先にルールを変えてください",
    ),
  ).toBeVisible();
  await expect(pending).toBeChecked();
  await expect(statuses.locator("#rules-statuses")).toContainText("5 / 5");
});

test("resets a section to the workspace settings after a confirmation", async ({ page }, info) => {
  const prefix = prefixOf(info);
  const name = unique(info, "戻す");
  await createWorkspace(page, info, name, prefix);
  await newProject(page, info, prefix);
  await settingsOf(page, prefix);
  const statuses = section(page, "rules-statuses");
  const reset = statuses.getByRole("button", { name: "ワークスペースの設定に戻す…" });
  await expect(reset).toBeDisabled();
  await statuses.getByRole("checkbox", { name: "Pending", exact: true }).click();
  await expect(statuses.locator("#rules-statuses")).toContainText("4 / 5");

  await reset.click();
  const dialog = page.getByRole("dialog", { name: "ワークスペースの設定に戻す" });
  await expect(
    dialog.getByText(
      `このプロジェクトの使うステータスのルールをすべて削除し、${name} の設定に戻します。`,
    ),
  ).toBeVisible();
  await dialog.getByRole("button", { name: "戻す", exact: true }).click();
  await expect(page.getByText("ワークスペースの設定に戻しました")).toBeVisible();
  await expect(statuses.locator("#rules-statuses")).toContainText("5 / 5");
  await expect(statuses.getByRole("checkbox", { checked: false })).toHaveCount(0);
  await expect(reset).toBeDisabled();
});

test("moving an epic to a project that does not use its status asks for a replacement", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "移動"), prefix);
  const panel = await projectWithEpic(page, info, prefix);
  const t2 = await newProject(page, info, prefix, 2);
  await settingsOf(page, prefix, 2);
  await section(page, "rules-statuses")
    .getByRole("checkbox", { name: "Pending", exact: true })
    .click();
  await expect(page.locator("#rules-statuses")).toContainText("4 / 5");

  // P1 の Epic を Pending にしてから、P2 へ移す。
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-1`));
  await panel
    .getByRole("region", { name: "プロパティ" })
    .getByRole("button", { name: "ステータス: Todo" })
    .click();
  await page.getByRole("menuitemcheckbox", { name: /^Pending/ }).click();
  await expect(
    panel
      .getByRole("region", { name: "プロパティ" })
      .getByRole("button", { name: "ステータス: Pending" }),
  ).toBeVisible();

  await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/tree`));
  const tree = page.getByRole("list", { name: "ツリー" });
  await treeRow(tree, `${prefix}-1`)
    .getByRole("button", { name: / のメニュー$/ })
    .click();
  await page.getByRole("menuitem", { name: /^移動…/ }).click();
  await page.getByRole("textbox", { name: "移動先を検索" }).fill(t2);
  await page.getByRole("menuitem", { name: `${prefix}-P2 ${t2}` }).click();
  const dialog = page.getByRole("dialog", { name: "移動" });
  await expect(dialog.getByText("ステータスの置き換え")).toBeVisible();
  await expect(dialog.getByText("Pending(1 件)")).toBeVisible();
  const select = dialog.getByRole("combobox", { name: "Pending(1 件)" });
  await expect(select.locator("option", { hasText: "Pending" })).toHaveCount(0);
  await dialog.getByRole("button", { name: "移動", exact: true }).click();
  await expect(page.getByText(`${t2} に移動しました`)).toBeVisible();

  await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P2/tree`));
  const tree2 = page.getByRole("list", { name: "ツリー" });
  await expect(statusButton(treeRow(tree2, `${prefix}-1`))).toHaveAccessibleName(
    "ステータス: Todo",
  );
});

test("deleting a template shows its effect on a project's rules", async ({ page }, info) => {
  const prefix = prefixOf(info);
  const name = unique(info, "消す雛形");
  await createWorkspace(page, info, unique(info, "テンプレートの削除"), prefix);
  await createTemplate(page, prefix, name);
  await newProject(page, info, prefix);
  await settingsOf(page, prefix);
  const templates = section(page, "rules-templates");
  await templates.getByRole("checkbox", { name, exact: true }).click();
  await expect(templates.getByRole("checkbox", { name, exact: true })).toBeChecked();
  await expect(templates).not.toContainText("すべて使えます");

  await openSettings(page, prefix, "テンプレート");
  await page
    .locator(".row")
    .filter({ hasText: name })
    .getByRole("button", { name: "削除" })
    .click();
  const dialog = page.getByRole("dialog", { name: "テンプレートを削除" });
  await expect(dialog).toContainText("1 件のプロジェクト別のルールからも削除されます。");
  await expect(dialog).toContainText(
    `${prefix}-P1 は、すべてのテンプレートを使えるようになります。`,
  );
  await dialog.getByRole("button", { name: "削除", exact: true }).click();
  await expect(page.getByText("テンプレートを削除しました")).toBeVisible();

  await settingsOf(page, prefix);
  await expect(section(page, "rules-templates")).toContainText("すべて使えます");
});

test("a read-only store's project settings tab disables every section", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/p/HH-P1/settings"));
  await expect(page.locator("#rules-statuses")).toBeVisible();
  await expect(page.getByText(readOnly).first()).toBeVisible();

  await expect(page.getByRole("textbox", { name: "名前" })).toBeDisabled();

  const statuses = section(page, "rules-statuses");
  const pending = statuses.getByRole("checkbox", { name: "Pending", exact: true });
  await expect(pending).toBeDisabled();
  await expect(pending.locator("..")).toHaveAttribute("title", readOnly);

  const workflow = section(page, "rules-workflow");
  const cell = workflow.getByRole("checkbox", { name: "Todo から InProgress", exact: true });
  await expect(cell).toBeDisabled();
  await expect(cell.locator("..")).toHaveAttribute("title", readOnly);

  const templates = section(page, "rules-templates");
  const taskDefault = templates.getByRole("combobox", { name: "Task", exact: true });
  await expect(taskDefault).toBeDisabled();
  await expect(taskDefault).toHaveAttribute("title", readOnly);

  const automation = section(page, "rules-automation");
  const add = automation.getByRole("button", { name: "このプロジェクトだけのルールを追加" });
  await expect(add).toBeDisabled();
  await expect(add).toHaveAttribute("title", readOnly);
  for (const s of [statuses, workflow, templates, automation]) {
    await expect(s.getByRole("button", { name: "ワークスペースの設定に戻す…" })).toBeDisabled();
  }
});
