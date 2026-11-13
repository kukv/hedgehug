import { expect, type Page, test } from "@playwright/test";
import {
  createWorkspace,
  newTaskUnder,
  openSettings,
  prefixOf,
  projectWithEpic,
  saved,
  unique,
} from "./helpers";

// テンプレートでまとめて作る・遷移の制約の通しのテスト(A3b)。サーバーは 3 つの幅で共有なので、
// 名前と接頭辞は毎回変える。

// restrictTodo は、ワークフローのタブで Todo から移れる先を InProgress だけにする。
async function restrictTodo(page: Page, prefix: string) {
  await openSettings(page, prefix, "ワークフロー");
  await Promise.all([
    saved(page, "/workflow/"),
    page.getByRole("checkbox", { name: "Todo から InProgress", exact: true }).click(),
  ]);
}

test("creates a task with its template's steps in one go", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "テンプレートで作る"), prefix);
  // テンプレート「手順つき」: 種類 Task、書き出し「手順: 」、Step「準備」(親と同じ)・「記録」(未割り当て)。Task の既定にする。
  await openSettings(page, prefix, "テンプレート");
  await page.getByRole("button", { name: "テンプレートを作成" }).click();
  const template = page.getByRole("dialog", { name: "テンプレートを作成" });
  await template.getByRole("textbox", { name: "名前" }).fill("手順つき");
  await template.getByRole("combobox", { name: "種類" }).selectOption({ label: "Task" });
  await template.getByRole("textbox", { name: "タイトルの書き出し" }).fill("手順: ");
  await template.getByRole("textbox", { name: "説明" }).fill("## 準備\n手順書を読む");
  await template.getByRole("button", { name: "Step を追加" }).click();
  await template.getByRole("textbox", { name: "Step のタイトル" }).nth(0).fill("準備");
  await template.getByRole("button", { name: "Step を追加" }).click();
  await template.getByRole("textbox", { name: "Step のタイトル" }).nth(1).fill("記録");
  // 「担当者」は、値の欄と Step の行。最後が「記録」の行。
  await template
    .getByRole("combobox", { name: "担当者" })
    .last()
    .selectOption({ label: "未割り当て" });
  await template.getByRole("button", { name: "保存", exact: true }).click();
  await expect(template).toBeHidden();
  await expect(page.locator(".row").filter({ hasText: "手順つき" })).toContainText("Step 2 件");
  await Promise.all([
    saved(page, "/template-defaults/"),
    page.getByRole("combobox", { name: "Task", exact: true }).selectOption({ label: "手順つき" }),
  ]);
  await expect(
    page.getByRole("combobox", { name: "Task", exact: true }).locator("option:checked"),
  ).toHaveText("手順つき");

  const panel = await projectWithEpic(page, info, prefix);
  await page.keyboard.press("Escape");

  // 作成ダイアログを開くと、既定が当たっている。
  await page.keyboard.press("c");
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await expect(dialog.getByRole("combobox", { name: "テンプレート" })).toHaveValue(/.+/);
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toHaveValue("手順: ");
  await expect(dialog.getByRole("textbox", { name: "説明" })).toHaveValue("## 準備\n手順書を読む");
  await expect(dialog.getByRole("region", { name: "一緒に作るもの: Step 2 件" })).toBeVisible();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("手順: ログイン");
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(page.getByText(`${prefix}-2 ほか 2 件を作成しました`)).toBeVisible();
  // 詳細パネルの Step に 2 件が並ぶ(番号は深さ優先)。
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue("手順: ログイン");
  const steps = panel.getByRole("region", { name: "Step" });
  await expect(steps.getByRole("link")).toHaveText(["準備", "記録"]);
  await expect(steps.getByText(`${prefix}-3`, { exact: true })).toBeVisible();
  await expect(steps.getByText(`${prefix}-4`, { exact: true })).toBeVisible();
  await expect(panel.getByRole("heading", { name: "準備" })).toBeVisible();
});

test("the status menu disables a status the workflow forbids and tells why", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "制約"), prefix);
  await restrictTodo(page, prefix);
  // プロジェクト・Epic・Task(Todo)を作り、詳細パネルのステータスのボタンを押す。
  const panel = await newTaskUnder(page, info, prefix);
  const props = panel.getByRole("region", { name: "プロパティ" });
  await props.getByRole("button", { name: "ステータス: Todo" }).click();
  const menu = page.getByRole("menu", { name: "ステータス" });
  await expect(menu.getByRole("menuitemcheckbox", { name: /^Done/ })).toBeDisabled();
  await expect(menu).toContainText(
    "ワークフローの制約: Todo から Done には変更できません(InProgress のみ)",
  );
  await menu.getByRole("menuitemcheckbox", { name: /^InProgress/ }).click();
  await expect(page.getByRole("button", { name: "ステータス: InProgress" }).first()).toBeVisible();
});

test("checking a step the workflow forbids shows the refusal and changes nothing", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "Step の制約"), prefix);
  await restrictTodo(page, prefix);
  // プロジェクト・Epic・Task を作り、詳細パネルで Step「確認」を足す。
  const panel = await newTaskUnder(page, info, prefix);
  const input = panel.getByRole("textbox", { name: "Step を追加" });
  await input.fill("確認");
  await input.press("Enter");
  const steps = panel.getByRole("region", { name: "Step" });
  const step = steps.getByRole("checkbox", { name: "確認" });
  await expect(step).toBeVisible();
  await step.click();
  await expect(
    page.getByText("ワークフローの制約: Todo から Done には変更できません(InProgress のみ)"),
  ).toBeVisible();
  await expect(step).not.toBeChecked();
  await expect(steps.getByRole("button", { name: "ステータス: Todo" })).toBeVisible();
});
