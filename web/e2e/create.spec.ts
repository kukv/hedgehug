import { expect, test } from "@playwright/test";
import { tokenUrl, unique } from "./helpers";

test("creates an Epic, a Task under it and a Step from the project screen", async ({
  page,
}, info) => {
  const epic = unique(info, "通知");
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/overview"));
  await page.getByRole("button", { name: "Epic", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await expect(dialog.getByRole("combobox", { name: "種類" })).toHaveValue("Epic");
  await expect(dialog.getByText(/^ID は HH-\d+ になります。$/)).toBeVisible();
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(dialog.getByText("タイトルを入力してください")).toBeVisible();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(epic);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(dialog).toBeHidden();
  await expect(page.getByText(/^HH-\d+ を作成しました$/)).toBeVisible();
  await expect(page.getByRole("region", { name: "Epic" }).getByText(epic)).toBeVisible();
  // 「続けて作成」がオフなので、作った Epic の詳細パネルが開く。Esc で閉じる。
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(epic);
  await page.keyboard.press("Escape");

  await page.getByRole("tab", { name: "ツリー" }).click();
  const tree = page.getByRole("list", { name: "ツリー" });
  await tree
    .getByRole("listitem")
    .filter({ hasText: epic })
    .getByRole("button", { name: "Story を追加" })
    .click();
  await expect(dialog.getByRole("combobox", { name: "種類" })).toHaveValue("Story");
  await dialog.getByRole("combobox", { name: "種類" }).selectOption("Task");
  await expect(dialog.getByRole("combobox", { name: "親" })).toHaveText(new RegExp(epic));
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(`${epic} の Task`);
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(tree.getByText(`${epic} の Task`)).toBeVisible();
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(`${epic} の Task`);
  await page.keyboard.press("Escape");

  await tree
    .getByRole("listitem")
    .filter({ hasText: `${epic} の Task` })
    .getByRole("button", { name: "Step を追加" })
    .click();
  await dialog.getByRole("checkbox", { name: "続けて作成" }).check();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(`${epic} の手順`);
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toHaveValue("");
  await expect(dialog.getByRole("combobox", { name: "種類" })).toHaveValue("Step");
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(tree.getByText(`${epic} の手順`)).toBeVisible();
});

test("the C key opens the dialog with a Task in the open project", async ({ page }) => {
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/tree"));
  await expect(page.getByRole("heading", { name: "HH-P1 ログイン" })).toBeVisible();
  await page.keyboard.press("c");
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await expect(dialog.getByRole("combobox", { name: "種類" })).toHaveValue("Task");
  await expect(dialog.getByRole("combobox", { name: "プロジェクト" })).toHaveText(/ログイン/);
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toBeFocused();
  // 入力中の C は文字になり、ダイアログを開き直さない。
  await page.keyboard.press("c");
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toHaveValue("c");
});

test("a read-only store has no enabled create buttons", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/p/HH-P1/tree"));
  await expect(page.getByRole("button", { name: "Epic", exact: true })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Story を追加" })).toBeDisabled();
});
