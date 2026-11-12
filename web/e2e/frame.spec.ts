import { expect, test } from "@playwright/test";

const url = "/#token=e2e-token";

test("shows the frame and reads the server with the token", async ({ page }) => {
  await page.goto(url);
  await expect(page.getByRole("heading", { name: "hedgehug" })).toBeVisible();
  // e2e 用のビルドは -ldflags で版を入れないので "dev"。トークンが合わなければ 401 で出ない。
  await expect(page.getByText("dev", { exact: true })).toBeVisible();
});

test("keeps the token out of the address and still reads the server after a reload", async ({
  page,
}) => {
  await page.goto(url);
  await expect(page.getByText("dev", { exact: true })).toBeVisible();
  expect(new URL(page.url()).hash).toBe("");
  await page.reload();
  await expect(page.getByText("dev", { exact: true })).toBeVisible();
});

test("the sidebar is a drawer only when the frame is 860px or narrower", async ({ page }, info) => {
  await page.goto(url);
  const sidebar = page.getByRole("complementary");
  const menu = page.getByRole("button", { name: "メニュー", exact: true });
  if (info.project.name === "desktop") {
    await expect(sidebar).toBeInViewport();
    await expect(menu).toBeHidden();
    return;
  }
  await expect(sidebar).not.toBeInViewport();
  await menu.click();
  await expect(sidebar).toBeInViewport();
  // 暗くした背景の、引き出しに重ならない右上を押す。
  const width = page.viewportSize()?.width ?? 0;
  await page
    .getByRole("button", { name: "メニューを閉じる" })
    .click({ position: { x: width - 10, y: 10 } });
  await expect(sidebar).not.toBeInViewport();
});
