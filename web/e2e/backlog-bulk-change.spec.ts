import type { Page, TestInfo } from "@playwright/test";
import { expect, test } from "./fixtures";
import {
  createWorkspace,
  prefixOf,
  projectWithEpic,
  storyUnderEpic,
  tokenUrl,
  unique,
} from "./helpers";

// バックログの一括変更の通しのテスト(F2b2)。ワークスペースは毎回作り、Story を -2・-3・-4 の 3 件置く。

const bar = (page: Page, n: number) => page.getByText(`${n} 件を選択中`);
const dialogOf = (page: Page) => page.getByRole("dialog", { name: "選択したアイテムを一括変更" });
const confirmOf = (page: Page) => page.getByRole("dialog", { name: "一括変更", exact: true });

async function openBacklog(page: Page, info: TestInfo) {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "一括変更"), prefix);
  await projectWithEpic(page, info, prefix);
  for (const title of ["検索", "通知", "設定"]) {
    await storyUnderEpic(page, info, `${prefix}-1`, title);
  }
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/backlog`));
  await expect(page.getByLabel(`${prefix}-4 を選択`)).toBeVisible();
  return [`${prefix}-2`, `${prefix}-3`, `${prefix}-4`] as const;
}

async function openDialog(page: Page, field: string) {
  await page.getByRole("button", { name: "一括変更…" }).click();
  const dialog = dialogOf(page);
  await expect(dialog).toBeVisible();
  await dialog.getByLabel("項目").selectOption({ label: field });
  return dialog;
}

test("bulk changes the status", async ({ page }, info) => {
  const [s2, s3, s4] = await openBacklog(page, info);
  await page.getByLabel(`${s2} を選択`).check();
  await page.getByLabel(`${s3} を選択`).check();
  const dialog = await openDialog(page, "ステータス");
  await expect(dialog.getByText("2 件(Story)")).toBeVisible();
  await dialog.getByLabel("値").selectOption({ label: "Done" });
  await dialog.getByRole("button", { name: "ステータスを変更" }).click();

  const confirm = confirmOf(page);
  await expect(confirm.getByText("2 件のステータスを Done に変更します。")).toBeVisible();
  await confirm.getByRole("button", { name: "2 件を変更" }).click();

  await expect(
    page.getByText("2 件を変えました。2 件は完了のため、バックログには表示されません"),
  ).toBeVisible();
  await expect(page.getByLabel(`${s2} を選択`)).toHaveCount(0);
  await expect(page.getByLabel(`${s3} を選択`)).toHaveCount(0);
  await expect(bar(page, 2)).toHaveCount(0);
  await expect(page.getByLabel(`${s4} を選択`)).toBeVisible();
});

test("nothing to change", async ({ page }, info) => {
  const [, , s4] = await openBacklog(page, info);
  await page.getByLabel(`${s4} を選択`).check();
  const dialog = await openDialog(page, "ステータス");
  // 値の初期選択は先頭のステータスで、新しいアイテムのいまのステータスと同じ。値が入るまで待つ。
  await expect(dialog.getByLabel("値")).toHaveValue(/.+/);
  await dialog.getByRole("button", { name: "ステータスを変更" }).click();

  await expect(page.getByText("変わるものはありません")).toBeVisible();
  await expect(confirmOf(page)).toHaveCount(0);
  await expect(bar(page, 1)).toBeVisible();
});

test("points refusal", async ({ page }, info) => {
  const [, , s4] = await openBacklog(page, info);
  await page.getByLabel(`${s4} を選択`).check();
  const dialog = await openDialog(page, "ポイント");
  await dialog.getByLabel("値").fill("-1");
  await dialog.getByRole("button", { name: "ポイントを変更" }).click();
  await expect(dialog.getByRole("alert")).toHaveText("ポイントは 0 以上の数で入力してください");
  await expect(dialog).toBeVisible();

  await dialog.getByLabel("値").fill("3");
  await dialog.getByRole("button", { name: "ポイントを変更" }).click();
  await confirmOf(page).getByRole("button", { name: "1 件を変更" }).click();

  await expect(page.getByText("1 件を変えました")).toBeVisible();
  await expect(page.getByLabel(`${s4} を選択`)).toBeChecked();
  await expect(bar(page, 1)).toBeVisible();
  const row = page.getByRole("listitem").filter({ has: page.getByLabel(`${s4} を選択`) });
  await expect(row.getByLabel("ポイント")).toHaveValue("3");
});

test("escape closes only the systems menu", async ({ page }, info) => {
  const [, , s4] = await openBacklog(page, info);
  await page.getByLabel(`${s4} を選択`).check();
  const dialog = await openDialog(page, "関連システムを追加");
  await dialog.getByRole("button", { name: "選択" }).click();
  const menu = page.getByRole("menu", { name: "値" });
  await expect(menu).toBeVisible();

  await page.keyboard.press("Escape");
  await expect(menu).toBeHidden();
  await expect(dialog).toBeVisible();
});

test("read-only store cannot bulk change", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/w/HH/backlog"));
  await page.getByLabel("HH-2 を選択").check();
  const change = page.getByRole("button", { name: "一括変更…" });
  await expect(change).toBeDisabled();
  await expect(change).toHaveAttribute("title", "閲覧のみの Store です");
});
