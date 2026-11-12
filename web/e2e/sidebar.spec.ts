import { expect, test } from "@playwright/test";
import { sidebar, tokenUrl } from "./helpers";

test("the sidebar lists stores and opens a workspace that survives a reload", async ({
  page,
}, info) => {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  const e2e = side.getByRole("region", { name: "e2e" });
  await expect(e2e.getByRole("link", { name: "ログイン" })).toBeVisible();
  await e2e.getByRole("link", { name: "Web" }).click();
  await expect(page.getByRole("heading", { name: "Web" })).toBeVisible();
  await expect(page).toHaveURL(/\/s\/e2e\/w\/HH$/);
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
  await e2e.getByRole("button", { name: "開閉" }).click();
  await expect(e2e.getByRole("link", { name: "ログイン" })).toBeHidden();
  await e2e.getByRole("button", { name: "開閉" }).click();
  await expect(e2e.getByRole("link", { name: "ログイン" })).toBeVisible();
});
