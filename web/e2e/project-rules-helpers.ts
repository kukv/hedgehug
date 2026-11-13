import { expect, type Locator, type Page } from "@playwright/test";
import { openSettings, tokenUrl } from "./helpers";

// プロジェクト別のルールの通しのテスト(B5a)の共通の手順。

export const readOnly = "閲覧のみの Store です";

// settingsOf は、プロジェクト(接頭辞-Pn)の設定タブを開く。
export async function settingsOf(page: Page, prefix: string, n = 1) {
  await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P${n}/settings`));
  await expect(page.locator("#rules-statuses")).toBeVisible();
}

// section は、設定タブの区画(h2 の id から引く)。
export const section = (page: Page, id: string) =>
  page.locator("section").filter({ has: page.locator(`#${id}`) });

// goTree は、設定タブからツリーのタブへ、画面を読み直さずに移る。
export async function goTree(page: Page) {
  await page.getByRole("tab", { name: "ツリー", exact: true }).click();
  const tree = page.getByRole("list", { name: "ツリー" });
  await expect(tree).toBeVisible();
  return tree;
}

export const statusButton = (row: Locator) => row.getByRole("button", { name: /^ステータス: / });

// createTemplate は、ワークスペースの設定のテンプレートのサブタブから、種類 Task のテンプレートを作る。
export async function createTemplate(page: Page, prefix: string, name: string) {
  await openSettings(page, prefix, "テンプレート");
  await page.getByRole("button", { name: "テンプレートを作成" }).click();
  const dialog = page.getByRole("dialog", { name: "テンプレートを作成" });
  await dialog.getByRole("textbox", { name: "名前" }).fill(name);
  await dialog.getByRole("combobox", { name: "種類" }).selectOption({ label: "Task" });
  await dialog.getByRole("button", { name: "保存", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.locator(".row").filter({ hasText: name })).toBeVisible();
}

// createOwnRule は、プロジェクトの設定タブの自動化の区画から、専用ルールを作る。
export async function createOwnRule(
  page: Page,
  name: string,
  fill: (dialog: Locator) => Promise<void>,
) {
  await page.getByRole("button", { name: "このプロジェクトだけのルールを追加" }).click();
  const dialog = page.getByRole("dialog", { name: "ルールを作成" });
  await dialog.getByRole("textbox", { name: "名前" }).fill(name);
  await fill(dialog);
  await dialog.getByRole("button", { name: "保存" }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText("ルールを作成しました")).toBeVisible();
}
