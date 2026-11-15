import { expect, test } from "./fixtures";
import { saved, sidebar, tokenUrl } from "./helpers";

test("the sidebar lists stores and opens a workspace that survives a reload", async ({
  page,
}, info) => {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  const e2e = side.getByRole("region", { name: "e2e" });
  await expect(e2e.getByRole("link", { name: "ログイン" })).toBeVisible();
  await e2e.getByRole("link", { name: "Web" }).click();
  await expect(page.getByRole("heading", { name: "Web" })).toBeVisible();
  await expect(page).toHaveURL(/\/s\/e2e\/w\/HH(\/overview)?$/);
  await page.reload();
  await expect(page.getByRole("heading", { name: "Web" })).toBeVisible();
});

test("a read-only store's workspace shows the band", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/w/HH"));
  await expect(
    page.getByText(
      "readonly は閲覧のみの Store です。閲覧はできますが、作成・変更・削除はできません。",
    ),
  ).toBeVisible();
});

test("the arrow hides and shows the projects of a workspace", async ({ page }, info) => {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  const e2e = side.getByRole("region", { name: "e2e" });
  // 通しのテストが作ったワークスペースが並ぶので、サンプルの「Web」の行に絞る。
  const twisty = e2e
    .locator(".ws-row")
    .filter({ has: page.getByRole("link", { name: "Web", exact: true }) })
    .getByRole("button", { name: "開閉" });
  await twisty.click();
  await expect(e2e.getByRole("link", { name: "ログイン" })).toBeHidden();
  await twisty.click();
  await expect(e2e.getByRole("link", { name: "ログイン" })).toBeVisible();
});

test("a closed workspace stays closed after a reload", async ({ page }, info) => {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  const e2e = side.getByRole("region", { name: "e2e" });
  const twisty = e2e
    .locator(".ws-row")
    .filter({ has: page.getByRole("link", { name: "Web", exact: true }) })
    .getByRole("button", { name: "開閉" });
  try {
    // 開閉を書き終えてから読み直す(書き終える前に読み直すと、前の値が出る)。
    const stored = saved(page, "/api/screen-states/sidebar");
    await twisty.click();
    await expect(e2e.getByRole("link", { name: "ログイン" })).toBeHidden();
    await stored;
    await page.reload();
    await sidebar(page, info);
    await expect(e2e.getByRole("link", { name: "Web", exact: true })).toBeVisible();
    await expect(e2e.getByRole("link", { name: "ログイン" })).toBeHidden();
  } finally {
    // 同じサーバーを使う後のテストのために、途中で失敗しても開き直す。
    if (await e2e.getByRole("link", { name: "ログイン" }).isHidden()) await twisty.click();
  }
  await expect(e2e.getByRole("link", { name: "ログイン" })).toBeVisible();
});
