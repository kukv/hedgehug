import { expect, test } from "./fixtures";
import { sidebar, tokenUrl } from "./helpers";
import { send } from "./search-table-helpers";

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

test("the clock warning shows in the general tab and on the sync card, and goes away when the clock is back", async ({
  page,
}, info) => {
  const skew = (seconds: number) => send(page, "post", "/e2e/clock-skew", { seconds });
  const warning = (dir: string, n: number) =>
    `このマシンの時計が GitHub の時刻より ${n} 分${dir}います。同期で後から届いた変更の勝ち負けと、削除済みを完全に削除する日が、他の人とずれます。OS の日時の設定を確かめてください。`;
  const card = (n: number) => `このマシンの時計が ${n} 分ずれています`;
  try {
    await page.goto(tokenUrl("/settings/accounts"));
    const tabs = page.getByRole("tab");
    await expect(tabs.nth(0)).toHaveText("一般");
    await expect(tabs.nth(1)).toHaveText("アカウントとメンバー");

    await skew(-420);
    await tabs.nth(0).click();
    await expect(page).toHaveURL(/\/settings\/general$/);
    await expect(page.getByText(warning("進んで", 7))).toBeVisible();
    const side = await sidebar(page, info);
    await expect(side.getByText(card(7))).toBeVisible();

    await skew(360);
    await expect(side.getByText(card(6))).toBeVisible();
    await expect(page.getByText(warning("遅れて", 6))).toBeVisible();

    await skew(60);
    await expect(page.getByText(/GitHub の時刻より/)).toHaveCount(0);
    await expect(side.getByText(/このマシンの時計が/)).toHaveCount(0);
  } finally {
    await skew(0);
  }
});
