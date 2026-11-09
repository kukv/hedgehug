import type { Locator, Page } from "@playwright/test";
import { expect, type Hedgehug, test } from "./harness";

// A date a number of days from today, so the bars are near where the chart opens.
const day = (n: number): string => {
  const d = new Date();
  d.setDate(d.getDate() + n);
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${dd}`;
};
const short = (date: string): string => date.slice(5).replace("-", "/");

const openGantt = async (page: Page, hh: Hedgehug) =>
  hh.open(page, (await hh.treePath()).replace(/\/tree$/, "/gantt"));

const bar = (page: Page, name: string): Locator =>
  page.getByRole("button", { name: new RegExp(`^${name} `) });

const commits = (hh: Hedgehug, id: string): number =>
  hh.json<{ history: unknown[] }>("show", id).history.length;

async function drag(page: Page, target: Locator, dx: number, edge?: "start" | "end") {
  await target.scrollIntoViewIfNeeded();
  const box = await target.locator(".body").boundingBox();
  if (!box) throw new Error("the bar is not on the screen");
  const x = edge === "end" ? box.x + box.width - 1 : box.x + box.width / 2;
  const y = box.y + box.height / 2;
  await page.mouse.move(x, y);
  await page.mouse.down();
  await page.mouse.move(x + dx / 2, y, { steps: 3 });
  await page.mouse.move(x + dx, y, { steps: 3 });
  await page.mouse.up();
}

test("a bar moves and stretches by dragging, each drop one commit", async ({ page, hh }) => {
  hh.hh("sprint", "add", "Sprint 1");
  hh.hh("add", "one", "-p", `start=${day(1)}`, "-p", `due=${day(3)}`);
  await openGantt(page, hh);

  const one = bar(page, "DM-1 one");
  await expect(one).toBeVisible();
  const before = commits(hh, "DM-1");
  await drag(page, one, 38 * 2);
  await expect.poll(() => hh.show("DM-1")).toMatchObject({ start: day(3), due: day(5) });
  expect(commits(hh, "DM-1")).toBe(before + 1);

  await drag(page, bar(page, "DM-1 one"), 38, "end");
  await expect.poll(() => hh.show("DM-1")).toMatchObject({ start: day(3), due: day(6) });
  expect(commits(hh, "DM-1")).toBe(before + 2);
});

test("a bar moves from the keyboard", async ({ page, hh }) => {
  hh.hh("sprint", "add", "Sprint 1");
  hh.hh("add", "one", "-p", `start=${day(1)}`, "-p", `due=${day(2)}`);
  await openGantt(page, hh);

  await bar(page, "DM-1 one").focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("status").filter({ hasText: "Enter で保存" })).toBeVisible();
  await page.keyboard.press("Escape");
  await page.keyboard.press("ArrowLeft");
  await page.keyboard.press("Enter");
  await expect.poll(() => hh.show("DM-1")).toMatchObject({ start: day(0), due: day(1) });
});

test("the chart draws sprints, releases, arrows, what adds up and what is late", async ({
  page,
  hh,
}) => {
  hh.hh("sprint", "add", "Sprint 1", "--start", day(0), "--end", day(13));
  hh.hh("sprint", "start", "Sprint 1");
  hh.hh("release", "add", "v1", "--date", day(10));
  hh.hh("add", "container", "--kind", "project", "-p", `due=${day(4)}`);
  hh.hh("add", "first", "--parent", "DM-1", "-p", `start=${day(1)}`, "-p", `due=${day(5)}`);
  hh.hh("add", "second", "--parent", "DM-1", "-p", `start=${day(2)}`, "-p", `due=${day(6)}`);
  hh.hh("add", "undated", "--parent", "DM-1", "--sprint", "Sprint 1");
  hh.hh("set", "DM-3", "blocked_by+=DM-2");
  await openGantt(page, hh);

  await expect(page.locator("svg title", { hasText: "Sprint 1" })).toHaveCount(1);
  await expect(
    page.locator("svg title", { hasText: `v1 リリース予定 ${short(day(10))}` }),
  ).toHaveCount(1);
  await expect(page.locator("path.link[data-late]")).toHaveCount(1);
  await expect(page.locator('[data-kind="rollup"][data-fill="warn"]')).toHaveCount(1);
  await expect(page.locator('[data-kind="sprint"]')).toHaveCount(1);

  const warnings = page.getByRole("list", { name: "ガントの警告" });
  await expect(warnings).toContainText("ブロックしている DM-2 の終了");
  await expect(warnings).toContainText("DM-1：子の期間");

  // The roadmap keeps the containers, whose bar still counts what is under them.
  await page.getByRole("button", { name: "ロードマップ" }).click();
  await expect(
    page.getByRole("list", { name: "ガントのタスク" }).getByRole("listitem"),
  ).toHaveCount(1);
  // The bar ends where the last of its children ends: the undated task has the end of its sprint.
  await expect(page.locator('[data-kind="rollup"] title')).toContainText(short(day(13)));
});

test("a gantt is saved as a view with its settings, and opens from a query", async ({
  page,
  hh,
}) => {
  hh.hh("sprint", "add", "Sprint 1");
  hh.hh("add", "container", "--kind", "project");
  hh.hh("add", "work", "--parent", "DM-1", "-p", `due=${day(2)}`, "-p", `start=${day(1)}`);
  await openGantt(page, hh);

  await page.getByRole("button", { name: "ロードマップ" }).click();
  await page.getByLabel("目盛り").selectOption("week");
  await page.getByRole("button", { name: "ビューを保存" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("名前", { exact: true }).fill("roadmap");
  await dialog.getByRole("button", { name: "保存" }).click();

  await expect(page.getByRole("heading", { name: "roadmap" })).toBeVisible();
  await expect(
    page.getByRole("list", { name: "ガントのタスク" }).getByRole("listitem"),
  ).toHaveCount(1);
  const saved = hh.json<{ layout: string; gantt: { roadmap: boolean } }[]>("view", "list");
  expect(saved[0]).toMatchObject({ layout: "gantt", gantt: { roadmap: true } });

  await hh.open(page, "/query/gantt?q=kind:task");
  await expect(page.getByRole("button", { name: /^DM-2 work / })).toBeVisible();
  await expect(page.getByRole("heading", { name: "クエリ" })).toBeVisible();
});

test("a project has the chart of its tasks", async ({ page, hh }) => {
  hh.hh("sprint", "add", "Sprint 1");
  hh.hh("add", "p", "--kind", "project");
  hh.hh("add", "inside", "--parent", "DM-1", "-p", `due=${day(2)}`);
  hh.hh("add", "outside", "-p", `due=${day(2)}`);
  const p = hh.json<{ ulid: string }>("show", "DM-1");
  await hh.open(page, `/projects/${p.ulid}`);
  await page.getByRole("link", { name: "ガント" }).click();
  await expect(page.getByRole("button", { name: /^DM-2 inside / })).toBeVisible();
  await expect(page.getByRole("button", { name: /^DM-3 outside / })).toHaveCount(0);
});

test("a tab of a workspace without plans stays hidden", async ({ page, hh }) => {
  hh.hh("add", "one");
  await hh.open(page, await hh.treePath());
  await expect(page.getByRole("link", { name: "ガント" })).toHaveCount(0);
  await expect(page.getByRole("link", { name: "レポート" })).toHaveCount(0);
});
