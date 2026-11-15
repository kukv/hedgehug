import { expect, test } from "./fixtures";
import { tokenUrl } from "./helpers";

const url = tokenUrl();

test("shows the frame and reads the server with the token", async ({ page }) => {
  await page.goto(url);
  // トークンが合わなければ、マイタスクの読み込みが 401 で出ない。
  await expect(page.getByRole("heading", { name: "マイタスク", level: 1 })).toBeVisible();
});

test("keeps the token out of the address and still reads the server after a reload", async ({
  page,
}) => {
  await page.goto(url);
  const heading = page.getByRole("heading", { name: "マイタスク", level: 1 });
  await expect(heading).toBeVisible();
  expect(new URL(page.url()).hash).toBe("");
  expect(new URL(page.url()).pathname).toBe("/my-tasks");
  await page.reload();
  await expect(heading).toBeVisible();
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
