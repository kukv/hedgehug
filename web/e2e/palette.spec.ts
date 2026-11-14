import type { Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import {
  createWorkspace,
  newProject,
  prefixOf,
  projectWithEpic,
  taskUnderEpic,
  tokenUrl,
  treeOf,
  treeRow,
  unique,
} from "./helpers";

// コマンドパレットの通しのテスト(A5c。02「コマンドパレット」)。mac は Cmd+K、ほかは Ctrl+K なので ControlOrMeta を使う。
// ワークスペースを作り、Epic(-1)と Task A(-2)・B(-3)を足して、順に確かめる。

const paletteOf = (page: Page) => page.getByRole("dialog", { name: "コマンドパレット" });
const panelOf = (page: Page) => page.getByRole("region", { name: "詳細パネル" });

async function openPalette(page: Page) {
  await page.keyboard.press("ControlOrMeta+k");
  const palette = paletteOf(page);
  await expect(palette).toBeVisible();
  await expect(palette.getByRole("combobox", { name: "コマンドパレット" })).toBeFocused();
  return palette;
}

test("finds items, recent items, old IDs and commands from the palette", async ({ page }, info) => {
  const t = (s: string) => unique(info, s);
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "パレット用"), prefix);
  await projectWithEpic(page, info, prefix);
  await taskUnderEpic(page, info, `${prefix}-1`, "A");
  await taskUnderEpic(page, info, `${prefix}-1`, "B");

  await test.step("opens an item by a part of its ID, and lists it under the recent group afterwards", async () => {
    await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/tree`));
    const palette = await openPalette(page);
    await palette.getByRole("combobox").fill(`${prefix}-3`.toLowerCase());
    const option = palette.getByRole("option", { name: new RegExp(`${prefix}-3 ${t("B")}`) });
    await expect(option).toBeVisible();
    await page.keyboard.press("Enter");
    await expect(palette).toBeHidden();
    await expect(panelOf(page).getByRole("textbox", { name: "タイトル" })).toHaveValue(t("B"));
    await panelOf(page).getByRole("button", { name: "閉じる" }).click();
    await expect(panelOf(page)).toBeHidden();

    const again = await openPalette(page);
    await expect(again.getByText("最近開いたもの", { exact: true })).toBeVisible();
    await expect(
      again.getByRole("option", { name: new RegExp(`${prefix}-3 ${t("B")}`) }),
    ).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(again).toBeHidden();
  });

  await test.step("finds a moved item by its old ID and opens it where it moved to", async () => {
    await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
    await panelOf(page).getByRole("button", { name: "メニュー", exact: true }).click();
    await page.getByRole("menuitem", { name: /^別のワークスペースへ移動…/ }).click();
    const dialog = page.getByRole("dialog", { name: "別のワークスペースへ移動" });
    await dialog
      .getByRole("combobox", { name: "移動先のワークスペース" })
      .selectOption({ label: "e2e / Mobile" });
    await expect(dialog.getByRole("heading", { name: "移動先で変わること" })).toBeVisible();
    await dialog.getByRole("button", { name: "移動", exact: true }).click();
    await expect(
      page.getByRole("status").getByText(new RegExp(`^${prefix}-2 は MB-\\d+ になりました$`)),
    ).toBeVisible();
    await expect(dialog).toBeHidden();

    await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/tree`));
    const palette = await openPalette(page);
    await palette.getByRole("combobox").fill(`${prefix}-2`);
    const moved = palette.getByRole("option", {
      name: new RegExp(`${t("A")}.*・旧 ID・.+ に移動$`),
    });
    await expect(moved).toBeVisible();
    await page.keyboard.press("Enter");
    await expect(palette).toBeHidden();
    await expect(panelOf(page).locator("code", { hasText: /^MB-\d+$/ })).toBeVisible();
    await expect(panelOf(page).getByRole("textbox", { name: "タイトル" })).toHaveValue(t("A"));
  });

  await test.step("changes the status of the focused tree row, and the tree keys keep working", async () => {
    const tree = await treeOf(page, prefix);
    const current = tree.locator('li[aria-current="true"]');
    await treeRow(tree, `${prefix}-1`).locator("code").click();
    await expect(current).toContainText(`${prefix}-1`);
    await page.keyboard.press("ArrowDown");
    await expect(current).toContainText(`${prefix}-3`);
    const palette = await openPalette(page);
    await expect(palette.getByText(`${prefix}-3 に対して`, { exact: true })).toBeVisible();
    await palette.getByRole("combobox").fill("ステータスを変更");
    await page.keyboard.press("Enter");
    await expect(palette.getByText(`${prefix}-3 のステータス`, { exact: true })).toBeVisible();
    await palette.getByRole("option", { name: /Done/ }).click();
    await expect(palette).toBeHidden();
    await expect(
      treeRow(tree, `${prefix}-3`).getByRole("button", { name: /^ステータス: / }),
    ).toHaveAccessibleName("ステータス: Done");
    // フォーカスはツリーに戻り、いまの行は先頭へ戻らず、キーが効く。
    await expect(tree).toBeFocused();
    await expect(current).toContainText(`${prefix}-3`);
    await page.keyboard.press("ArrowUp");
    await expect(current).toContainText(`${prefix}-1`);
    await page.keyboard.press("ArrowDown");
    await expect(current).toContainText(`${prefix}-3`);
  });

  await test.step("does not open over a dialog", async () => {
    await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/tree`));
    await page.getByRole("button", { name: "Epic", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "新規作成" });
    await expect(dialog).toBeVisible();
    await page.keyboard.press("ControlOrMeta+k");
    await expect(paletteOf(page)).toHaveCount(0);
    await expect(dialog).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
  });

  await test.step("closes with the browser back, and Esc closes only the palette over the detail panel", async () => {
    const tree = await treeOf(page, prefix);
    await tree.getByRole("link", { name: t("通知") }).click();
    await expect(panelOf(page)).toBeVisible();
    const palette = await openPalette(page);
    await page.keyboard.press("Escape");
    await expect(palette).toBeHidden();
    await expect(panelOf(page)).toBeVisible();

    const second = await openPalette(page);
    await page.goBack();
    await expect(second).toBeHidden();
  });
});

test("creates a sprint from the palette, from another screen and from the sprints tab", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "パレット計画"), prefix);
  await newProject(page, info, prefix);

  const dialog = page.getByRole("dialog", { name: "スプリントを作成" });
  const create = async () => {
    const palette = await openPalette(page);
    await palette.getByRole("combobox").fill("スプリントを作成");
    await palette.getByRole("option", { name: /^スプリントを作成/ }).click();
    await expect(palette).toBeHidden();
    await expect(page).toHaveURL(new RegExp(`/s/e2e/w/${prefix}/sprints$`));
    await expect(dialog).toBeVisible();
  };

  // プロジェクトの画面(別の画面)から。
  await expect(page).toHaveURL(new RegExp(`/p/${prefix}-P1`));
  await create();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  // すでにスプリントのタブを出しているとき。
  await create();
  await dialog.getByRole("button", { name: "作成", exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText("Sprint 1 を作成しました")).toBeVisible();
});
