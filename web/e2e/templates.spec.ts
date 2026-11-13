import { expect, test } from "./fixtures";
import { createWorkspace, openSettings, prefixOf, tokenUrl, unique } from "./helpers";

// ワークフローとテンプレートの通しのテスト(B3)。サーバーは同じ worker のほかのテストと共有なので、
// ワークスペースの名前・接頭辞・テンプレートの名前は、幅とやり直しの回数、乱数で毎回変える。

test("the workflow tab restricts and frees a status's transitions", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "ワークフロー"), prefix);
  await openSettings(page, prefix, "ワークフロー");

  const cell = page.getByRole("checkbox", { name: "Todo から InProgress", exact: true });
  const todo = page.locator("tbody tr").filter({ has: cell });
  await expect(todo).toContainText("どこへでも");
  await expect(todo.getByRole("cell", { name: "—", exact: true })).toHaveCount(1);

  await cell.click();
  await expect(todo).toContainText("制限あり");
  await expect(cell).toBeChecked();
  // 読み直しても残る。
  await openSettings(page, prefix, "ワークフロー");
  await expect(todo).toContainText("制限あり");
  await expect(cell).toBeChecked();

  await cell.click();
  await expect(todo).toContainText("どこへでも");
  await expect(cell).not.toBeChecked();
});

test("creates, defaults, re-kinds and deletes a template", async ({ page }, info) => {
  const prefix = prefixOf(info);
  const name = unique(info, `調査 ${Math.floor(Math.random() * 1e5)}`);
  await createWorkspace(page, info, unique(info, "テンプレート"), prefix);
  await openSettings(page, prefix, "テンプレート");
  await expect(page.getByText("テンプレートはありません")).toBeVisible();

  // 作成。
  await page.getByRole("button", { name: "テンプレートを作成" }).click();
  const create = page.getByRole("dialog", { name: "テンプレートを作成" });
  const fillTemplate = async () => {
    await create.getByRole("textbox", { name: "名前" }).fill(name);
    await create.getByRole("combobox", { name: "種類" }).selectOption({ label: "Task" });
  };
  await fillTemplate();
  await create.getByRole("textbox", { name: "タイトルの書き出し" }).fill("調査: ");
  await create.getByRole("combobox", { name: "優先度" }).selectOption({ label: "High" });
  await create.getByRole("button", { name: "Step を追加" }).click();
  await create.getByRole("textbox", { name: "Step のタイトル" }).fill("再現");
  // 「担当者」は、値の欄と Step の行の 2 つ。Step の行の方は後ろ。
  await create
    .getByRole("combobox", { name: "担当者" })
    .last()
    .selectOption({ label: "未割り当て" });
  await create.getByRole("button", { name: "保存", exact: true }).click();
  await expect(create).toBeHidden();
  await expect(page.getByText("テンプレートを作成しました")).toBeVisible();
  const row = page.locator(".row").filter({ hasText: name });
  await expect(row).toContainText("Step 1 件");

  // 同じ名前は、ダイアログの中で断られる。
  await page.getByRole("button", { name: "テンプレートを作成" }).click();
  await fillTemplate();
  await create.getByRole("button", { name: "保存", exact: true }).click();
  await expect(create.getByText("同じ名前のテンプレートがあります")).toBeVisible();
  await create.getByRole("button", { name: "キャンセル" }).click();
  await expect(create).toBeHidden();
  await expect(row).toHaveCount(1);

  // 種類ごとの既定。読み直しても選ばれている。
  const taskDefault = page.getByRole("combobox", { name: "Task", exact: true });
  await taskDefault.selectOption({ label: name });
  await expect(taskDefault.locator("option:checked")).toHaveText(name);
  await openSettings(page, prefix, "テンプレート");
  await expect(taskDefault.locator("option:checked")).toHaveText(name);

  // 種類を Epic に変えると、Step を捨てる確認が出る。保存すると件数が消え、Task の既定は「なし」に戻る。
  await row.getByRole("button", { name: "編集" }).click();
  const edit = page.getByRole("dialog", { name: "テンプレートの編集" });
  await edit.getByRole("combobox", { name: "種類" }).selectOption({ label: "Epic" });
  await edit.getByRole("button", { name: "保存", exact: true }).click();
  const confirm = page.getByRole("dialog", { name: "テンプレートを保存" });
  await expect(confirm).toContainText("Step 1 件は、この種類では保存されません");
  await confirm.getByRole("button", { name: "保存", exact: true }).click();
  await expect(edit).toBeHidden();
  await expect(row).not.toContainText("Step 1 件");
  await expect(taskDefault.locator("option:checked")).toHaveText("なし");

  // 削除。
  await row.getByRole("button", { name: "削除" }).click();
  const del = page.getByRole("dialog", { name: "テンプレートを削除" });
  await expect(del).toContainText(`「${name}」を削除します。作ったアイテムには影響しません。`);
  await del.getByRole("button", { name: "削除", exact: true }).click();
  await expect(page.getByText("テンプレートを削除しました")).toBeVisible();
  await expect(page.getByText("テンプレートはありません")).toBeVisible();
});

test("saves an item as a template from the panel menu", async ({ page }, info) => {
  const task = unique(info, `雛形 ${Math.floor(Math.random() * 1e5)}`);
  const name = `${task} のテンプレート`;
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/tree"));
  await page.getByRole("list", { name: "ツリー" }).getByRole("link", { name: "認証" }).click();
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await panel.getByRole("button", { name: "Task" }).click();
  const create = page.getByRole("dialog", { name: "新規作成" });
  await create.getByRole("textbox", { name: "タイトル" }).fill(task);
  await create.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(task);
  const input = panel.getByRole("textbox", { name: "Step を追加" });
  await input.fill("設計");
  await input.press("Enter");
  await expect(panel.getByRole("region", { name: "Step" }).getByRole("link")).toHaveText(["設計"]);

  await panel.getByRole("button", { name: "メニュー", exact: true }).click();
  await page.getByRole("menuitem", { name: "テンプレートとして保存…" }).click();
  const dialog = page.getByRole("dialog", { name: "テンプレートとして保存" });
  await expect(dialog.getByRole("textbox", { name: "名前" })).toHaveValue(task);
  await expect(dialog.getByRole("textbox", { name: "タイトルの書き出し" })).toHaveValue(task);
  await expect(dialog.getByRole("checkbox", { name: "子と Step を含める" })).toBeChecked();
  await dialog.getByRole("textbox", { name: "名前" }).fill(name);
  await dialog.getByRole("button", { name: "保存", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText("テンプレートを作成しました")).toBeVisible();

  await openSettings(page, "HH", "テンプレート");
  await expect(page.locator(".row").filter({ hasText: name })).toContainText("Step 1 件");
});

test("a read-only store's item cannot be saved as a template", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/i/HH-2"));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await panel.getByRole("button", { name: "メニュー", exact: true }).click();
  await expect(page.getByRole("menuitem", { name: /テンプレートとして保存…/ })).toBeDisabled();
});
