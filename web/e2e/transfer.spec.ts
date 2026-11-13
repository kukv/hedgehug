import type { Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import {
  createWorkspace,
  prefixOf,
  projectWithEpic,
  taskUnderEpic,
  tokenUrl,
  unique,
} from "./helpers";

// 別のワークスペース・別の Store への移動と、旧 ID のアドレスの通しのテスト(A5a)。自分のワークスペースに
// Epic(-1)と Task A(-2)・B(-3)を作り、A を同じ Store の「Mobile」へ、Epic を別の Store「team」へ移す。
// サンプルの Mobile(MB、Epic MB-1)と team の Team(TM)は、ほかのテストが使わない(cmd/hedgehug/wire_e2e.go)。

const panelOf = (page: Page) => page.getByRole("region", { name: "詳細パネル" });

async function openTransfer(page: Page, address: string) {
  await page.goto(tokenUrl(address));
  const panel = panelOf(page);
  await panel.getByRole("button", { name: "メニュー", exact: true }).click();
  await page.getByRole("menuitem", { name: /^別のワークスペースへ移動…/ }).click();
  const dialog = page.getByRole("dialog", { name: "別のワークスペースへ移動" });
  await expect(dialog).toBeVisible();
  return dialog;
}

test("moves items to another workspace and another Store, and follows the old IDs", async ({
  page,
}, info) => {
  const t = (s: string) => unique(info, s);
  const prefix = prefixOf(info);
  await createWorkspace(page, info, t("移動元"), prefix);
  await projectWithEpic(page, info, prefix);
  await taskUnderEpic(page, info, `${prefix}-1`, "A");
  await taskUnderEpic(page, info, `${prefix}-1`, "B");

  await test.step("moves Task A to the workspace Mobile in the same Store", async () => {
    const dialog = await openTransfer(page, `/s/e2e/i/${prefix}-2`);
    await dialog
      .getByRole("combobox", { name: "移動先のワークスペース" })
      .selectOption({ label: "e2e / Mobile" });
    await expect(dialog.getByText(`${prefix}-2 ${t("A")} を移動します。`)).toBeVisible();
    await expect(dialog.getByRole("heading", { name: "移動先で変わること" })).toBeVisible();
    await expect(dialog.getByText("ID は MB-2 からの番号に振り直されます")).toBeVisible();
    await expect(dialog.getByText("スプリントとリリースは外されます")).toBeVisible();
    await expect(dialog.getByRole("combobox", { name: "親の Epic" })).toBeVisible();
    await dialog.getByRole("button", { name: "移動", exact: true }).click();
    await expect(
      page.getByRole("status").getByText(`${prefix}-2 は MB-2 になりました`),
    ).toBeVisible();
    // 開いていたパネルが、新しい ID のアイテムに切り替わる。
    await expect(panelOf(page).locator("code", { hasText: "MB-2" })).toBeVisible();
  });

  await test.step("moves the Epic and the Task under it to the Store team", async () => {
    const dialog = await openTransfer(page, `/s/e2e/i/${prefix}-1`);
    await dialog
      .getByRole("combobox", { name: "移動先のワークスペース" })
      .selectOption({ label: "team / Team" });
    await expect(
      dialog.getByText(`${prefix}-1 ${t("通知")} と、その下の 1 件を移動します。`),
    ).toBeVisible();
    // Epic は親を選ばない。別の Store なので、関連の解除とメンバーの行が出る。
    await expect(dialog.getByRole("combobox", { name: "親の Epic" })).toHaveCount(0);
    await expect(
      dialog.getByText("移さないアイテムとの関連は解除されます。移すもの同士の関連は残ります"),
    ).toBeVisible();
    await expect(
      dialog.getByText("移動先のメンバーにいない人は、移動先のメンバーに加わります"),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "移動", exact: true }).click();
    await expect(
      page.getByRole("status").getByText(new RegExp(`^${prefix}-1 は TM-\\d+ になりました$`)),
    ).toBeVisible();
  });

  await test.step("opens the new item from the old ID, with the moved-from line only then", async () => {
    await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
    await expect(
      panelOf(page).getByText(new RegExp(`^${prefix}-2 は .+ に MB-2 へ移動しました$`)),
    ).toBeVisible();
    await expect(panelOf(page).locator("code", { hasText: "MB-2" })).toBeVisible();
    await page.goto(tokenUrl(`/s/e2e/i/${prefix}-1`));
    await expect(
      panelOf(page).getByText(new RegExp(`^${prefix}-1 は .+ に TM-\\d+ へ移動しました$`)),
    ).toBeVisible();
    // 移したあとの ID で開いたときは出ない。
    await page.goto(tokenUrl("/s/e2e/i/MB-2"));
    await expect(panelOf(page).locator("code", { hasText: "MB-2" })).toBeVisible();
    await expect(panelOf(page).getByText(/へ移動しました$/)).toHaveCount(0);
  });

  await test.step("says the moved Task is deleted when its old ID is opened after the deletion", async () => {
    await panelOf(page).getByRole("button", { name: "メニュー", exact: true }).click();
    await page.getByRole("menuitem", { name: /^削除…/ }).click();
    await page
      .getByRole("dialog", { name: "Task を削除" })
      .getByRole("button", { name: "削除", exact: true })
      .click();
    await expect(panelOf(page)).toBeHidden();
    await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
    await expect(
      page
        .getByRole("status")
        .getByText(
          new RegExp(
            `^${prefix}-2 は .+ に MB-2 へ移動しましたが、削除されています。設定の「削除済み」から復元できます$`,
          ),
        ),
    ).toBeVisible();
    await expect(panelOf(page)).toBeHidden();
  });
});

test("disables the transfer item in a read-only Store with the reason", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/p/HH-P1/tree"));
  await page.getByRole("list", { name: "ツリー" }).getByRole("link", { name: "認証" }).click();
  const panel = panelOf(page);
  await panel.getByRole("button", { name: "メニュー", exact: true }).click();
  const entry = page.getByRole("menuitem", { name: /^別のワークスペースへ移動…/ });
  await expect(entry).toBeDisabled();
  await expect(entry).toContainText("閲覧のみの Store です");
});
