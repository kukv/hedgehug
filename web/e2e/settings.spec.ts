import { expect, test } from "./fixtures";
import { sidebar, tokenUrl } from "./helpers";

test("the gear in the sidebar opens the settings, and a reload keeps them", async ({
  page,
}, info) => {
  await page.goto(tokenUrl("/"));
  const side = await sidebar(page, info);
  await side.getByRole("link", { name: "設定" }).click();
  await expect(page).toHaveURL(/\/settings\/accounts$/);
  await expect(page.getByRole("heading", { name: "設定" })).toBeVisible();
  await expect(page.getByText("このマシンの設定と、Store の管理")).toBeVisible();
  await expect(page.getByRole("tab", { name: "アカウントとメンバー" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await page.reload();
  await expect(page.getByRole("heading", { name: "設定" })).toBeVisible();
});
