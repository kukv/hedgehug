import type { Page } from "@playwright/test";
import { expect, type Hedgehug, test } from "./harness";
import { dragCard } from "./screens";

const day = (n: number): string => {
  const d = new Date();
  d.setDate(d.getDate() + n);
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${dd}`;
};

const openTab = async (page: Page, hh: Hedgehug, tab: string) =>
  hh.open(page, (await hh.treePath()).replace(/\/tree$/, `/${tab}`));

// A commit keeps whole seconds: what joins after the start has to be in a later second to be seen so.
async function runSprint(page: Page, hh: Hedgehug) {
  hh.hh("add", "one", "-p", "points=3", "-p", "estimate=2h", "--sprint", "Sprint 1");
  hh.hh("add", "two", "-p", "points=2", "--sprint", "Sprint 1");
  hh.hh("sprint", "start", "Sprint 1");
  await page.waitForTimeout(1100);
  hh.hh("add", "late", "-p", "points=4", "--sprint", "Sprint 1");
  hh.hh("set", "DM-1", "status=in_progress");
  hh.hh("set", "DM-1", "status=done");
  hh.hh("worklog", "DM-1", "3h");
}

test("the board of a running sprint has the burndown and what joined", async ({ page, hh }) => {
  hh.hh("sprint", "add", "Sprint 1", "--start", day(-2), "--end", day(9));
  await runSprint(page, hh);
  await openTab(page, hh, "sprints");

  const now = page.getByRole("region", { name: "Sprint 1 の状況" });
  await expect(now).toContainText("残り points");
  await expect(now).toContainText("6 pt");
  await expect(now).toContainText("/ 9 pt");
  await expect(now).toContainText("+4 pt");
  await expect(
    now.getByRole("img", { name: "Sprint 1 のバーンダウン" }).locator("canvas"),
  ).toHaveCount(1);

  // The numbers follow a commit made elsewhere.
  hh.hh("set", "DM-2", "status=done");
  await expect(now).toContainText("4 pt");
});

test("the reports show what the CLI reports, under the same conditions", async ({ page, hh }) => {
  hh.hh("sprint", "add", "Sprint 1", "--start", day(-2), "--end", day(9));
  await runSprint(page, hh);
  hh.hh("sprint", "close", "Sprint 1", "--to", "backlog");
  await openTab(page, hh, "reports");

  const burndown = page.getByRole("region", { name: /^バーンダウン/ });
  await expect(burndown.getByRole("heading")).toContainText("Sprint 1");
  await expect(
    burndown.getByRole("img", { name: "Sprint 1 のバーンダウン" }).locator("canvas"),
  ).toHaveCount(1);
  await expect(
    burndown.getByRole("button", { name: /hh report burndown "Sprint 1" --by points/ }),
  ).toBeVisible();
  await burndown.getByRole("button", { name: "件数" }).click();
  await expect(burndown.getByRole("button", { name: /--by count/ })).toBeVisible();
  await expect(burndown.locator("details")).toContainText("3 件");

  const velocity = page.getByRole("region", { name: "ベロシティ" });
  await expect(velocity).toContainText("平均 3 pt");
  await expect(velocity.locator("details table")).toContainText("Sprint 1");
  await velocity.getByLabel("期間").selectOption("10");
  await expect(velocity.getByRole("button", { name: /--last 10/ })).toBeVisible();

  const flow = page.getByRole("region", { name: "累積フロー図" });
  await expect(flow.getByRole("img", { name: "累積フロー図" }).locator("canvas")).toHaveCount(1);
  await flow.getByRole("button", { name: "ステータス" }).click();
  await expect(flow.locator("details table")).toContainText("in_progress");
  await flow.getByLabel("開始").fill(day(-5));
  await expect(flow.getByRole("button", { name: new RegExp(`--since ${day(-5)}`) })).toBeVisible();

  const cycle = page.getByRole("region", { name: /^サイクルタイム/ });
  await expect(cycle).toContainText("に完了した 1 件");

  const estimate = page.getByRole("region", { name: "見積もりと実績" });
  await expect(estimate).toContainText("2h → 3h（超過）");
  await estimate.getByLabel("対象").selectOption("Sprint 1");
  await expect(estimate.getByRole("button", { name: /--sprint "Sprint 1"/ })).toBeVisible();
});

test("a sprint goes from planned to its velocity on the GUI alone", async ({ page, hh }) => {
  hh.hh("add", "one", "-p", "points=3");
  await openTab(page, hh, "settings");
  await page.getByRole("button", { name: "スプリントを作る…" }).click();
  const made = page.getByRole("dialog");
  await made.getByLabel("名前", { exact: true }).fill("Sprint 1");
  await made.getByRole("button", { name: "作成" }).click();

  await page.getByRole("link", { name: "バックログ" }).click();
  await page.getByRole("button", { name: "one の移し先" }).click();
  await page.getByRole("menuitem", { name: "Sprint 1 に入れる" }).click();
  const panel = page.getByRole("region", { name: "Sprint 1" });
  await panel.getByRole("button", { name: "開始する" }).click();
  const start = page.getByRole("dialog");
  await start.getByRole("textbox", { name: "start" }).fill(day(0));
  await start.getByRole("textbox", { name: "end" }).fill(day(13));
  await start.getByRole("button", { name: "開始する" }).click();
  await expect(panel.getByRole("link", { name: "ボードを開く" })).toBeVisible();
  await panel.getByRole("link", { name: "ボードを開く" }).click();

  const one = hh.show("DM-1");
  await dragCard(
    page,
    page.locator(`li[data-ulid="${one.ulid}"]`),
    page.locator('ul[data-column="done"]'),
  );
  await expect.poll(() => hh.show("DM-1").status).toBe("done");

  await page.getByRole("button", { name: "スプリントを閉じる…" }).click();
  await page.getByRole("contentinfo").getByRole("button", { name: "閉じる" }).click();
  await expect(page.getByRole("row", { name: /Sprint 1 closed/ })).toBeVisible();

  await page.getByRole("link", { name: "レポート" }).click();
  await expect(
    page.getByRole("region", { name: "ベロシティ" }).locator("details table"),
  ).toContainText("Sprint 1");
});
