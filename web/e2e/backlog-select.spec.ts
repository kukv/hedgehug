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

// バックログの行の選択と一括削除の通しのテスト(F2b1)。ワークスペースは毎回作り、Story を -2・-3・-4 の 3 件置く。

const readOnlyNote = "閲覧のみの Store です";
const paletteOf = (page: Page) => page.getByRole("dialog", { name: "コマンドパレット" });
const bar = (page: Page, n: number) => page.getByText(`${n} 件を選択中`);

async function openBacklog(page: Page, info: TestInfo) {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "選択"), prefix);
  await projectWithEpic(page, info, prefix);
  for (const title of ["検索", "通知", "設定"]) {
    await storyUnderEpic(page, info, `${prefix}-1`, title);
  }
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/backlog`));
  await expect(page.getByLabel(`${prefix}-4 を選択`)).toBeVisible();
  return [`${prefix}-2`, `${prefix}-3`, `${prefix}-4`] as const;
}

test("selects rows and clears", async ({ page }, info) => {
  const [s2, s3] = await openBacklog(page, info);
  await page.getByLabel(`${s2} を選択`).check();
  await page.getByLabel(`${s3} を選択`).check();
  await expect(bar(page, 2)).toBeVisible();

  await page.getByRole("button", { name: "選択を解除" }).click();
  await expect(bar(page, 2)).toHaveCount(0);
  await expect(page.getByLabel(`${s2} を選択`)).not.toBeChecked();
  await expect(page.getByLabel(`${s3} を選択`)).not.toBeChecked();
});

test("bulk deletes the selected rows", async ({ page }, info) => {
  const [s2, s3, s4] = await openBacklog(page, info);
  await page.getByLabel(`${s2} を選択`).check();
  await page.getByLabel(`${s3} を選択`).check();
  await page.getByRole("button", { name: "一括削除…" }).click();

  const dialog = page.getByRole("dialog", { name: "一括削除" });
  await expect(
    dialog.getByText(
      `${s2}、${s3} と、その下にあるものを削除します。設定の「削除済み」から戻せます。30 日を過ぎると完全に削除されます。`,
    ),
  ).toBeVisible();
  await dialog.getByRole("button", { name: "2 件を削除" }).click();

  await expect(page.getByText("2 件を削除しました")).toBeVisible();
  await expect(dialog).toBeHidden();
  await expect(page.getByLabel(`${s2} を選択`)).toHaveCount(0);
  await expect(page.getByLabel(`${s3} を選択`)).toHaveCount(0);
  await expect(page.getByLabel(`${s4} を選択`)).toBeVisible();
  await expect(bar(page, 2)).toHaveCount(0);
});

test("palette sees one selected row", async ({ page }, info) => {
  const [s2, , s4] = await openBacklog(page, info);
  await page.getByLabel(`${s4} を選択`).check();
  await page.keyboard.press("ControlOrMeta+k");
  await expect(paletteOf(page)).toBeVisible();
  await expect(paletteOf(page).getByText(`${s4} に対して`)).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(paletteOf(page)).toBeHidden();

  await page.getByLabel(`${s2} を選択`).check();
  await page.keyboard.press("ControlOrMeta+k");
  await expect(paletteOf(page)).toBeVisible();
  await expect(paletteOf(page).getByText(`${s4} に対して`)).toHaveCount(0);
});

test("leaving the tab clears the selection", async ({ page }, info) => {
  const [, , s4] = await openBacklog(page, info);
  await page.getByLabel(`${s4} を選択`).check();
  await expect(bar(page, 1)).toBeVisible();

  const tabs = page.getByRole("tablist");
  await tabs.getByRole("tab", { name: "スプリント" }).click();
  await expect(page).toHaveURL(/\/sprints/);
  await tabs.getByRole("tab", { name: "バックログ" }).click();
  await expect(page.getByLabel(`${s4} を選択`)).toBeVisible();
  await expect(bar(page, 1)).toHaveCount(0);
});

test("read-only store can select but not delete", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/w/HH/backlog"));
  await page.getByLabel("HH-2 を選択").check();
  await expect(bar(page, 1)).toBeVisible();
  const del = page.getByRole("button", { name: "一括削除…" });
  await expect(del).toBeDisabled();
  await expect(del).toHaveAttribute("title", readOnlyNote);
});
