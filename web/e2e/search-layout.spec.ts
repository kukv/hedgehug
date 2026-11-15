import type { Page, TestInfo } from "@playwright/test";
import { expect, test } from "./fixtures";
import { resetSearchScreen, sidebar, tokenUrl } from "./helpers";
import { lastHeader } from "./search-table-helpers";
import { e2eStore, filterOf, rowOf, search, seed } from "./search-tree-helpers";

// 検索の画面のレイアウトの切り替え・ツリーの注記・削除済みの案内・テーブルで開く入口(E3a-1・7〜10、Review Focus 5)の
// 通しのテスト。画面の状態はサーバーに残るので、各テストの前に初めの値(テーブル)に戻す。

test.beforeEach(async ({ page }) => {
  await resetSearchScreen(page.request);
});

const layoutOf = (page: Page, label: string) =>
  page.getByRole("group", { name: "レイアウト" }).getByRole("button", { name: label, exact: true });
const treeOf = (page: Page) => page.getByRole("list", { name: "ツリー" });
const paletteOf = (page: Page) => page.getByRole("dialog", { name: "コマンドパレット" });
const saved = (page: Page) =>
  page.waitForResponse(
    (r) =>
      r.url().includes("/api/screen-states/search") && r.request().method() === "PUT" && r.ok(),
  );
const loaded = (page: Page) =>
  page.waitForResponse(
    (r) => r.url().includes("/api/screen-states/search") && r.request().method() === "GET",
  );

// openAt は、アドレスで検索の画面を開く(画面の状態のレイアウトで開く)。
async function openAt(page: Page) {
  const read = loaded(page);
  await page.goto(tokenUrl("/search"));
  await read;
}

// choose は、レイアウトを切り替えて、画面の状態に書けるのを待つ。
async function choose(page: Page, label: string) {
  const put = saved(page);
  await layoutOf(page, label).click();
  await put;
  await expect(layoutOf(page, label)).toHaveAttribute("aria-pressed", "true");
}

// addStartColumn は、テーブルに「開始日」の列を足して、画面の状態に書けるのを待つ。
async function addStartColumn(page: Page) {
  await page.getByRole("button", { name: "表示する列", exact: true }).click();
  const put = saved(page);
  await page
    .getByRole("menu", { name: "表示する列" })
    .getByRole("menuitemcheckbox", { name: "開始日" })
    .click();
  await put;
  await page.keyboard.press("Escape");
  await expect(lastHeader(page)).toHaveText("開始日");
}

// clickSearchLink は、サイドバーの「検索」を押す。狭い幅で同じ画面を押した(stay)ときは引き出しが閉じないので、暗くした
// 背景の引き出しに重ならない右上を押して閉じる(frame.spec.ts と同じ)。
async function clickSearchLink(page: Page, info: TestInfo, stay: boolean) {
  const side = await sidebar(page, info);
  await side.getByRole("link", { name: "検索", exact: true }).click();
  if (!stay || info.project.name === "desktop") return;
  const width = page.viewportSize()?.width ?? 0;
  await page
    .getByRole("button", { name: "メニューを閉じる" })
    .click({ position: { x: width - 10, y: 10 } });
  await expect(side).not.toBeInViewport();
}

// seedTask は、新しいワークスペースに Epic と、その下の Task(接頭辞-2)を作る。
async function seedTask(page: Page, info: TestInfo) {
  const s = await seed(page, info);
  const p = await s.project(1);
  const epic = await s.item(p.id, "Epic", `${s.mark} E`);
  const task = await s.item(p.id, "Task", `${s.mark} T`, epic);
  return { ...s, task, query: `kind:task ${s.mark}` };
}

test("switches layouts and keeps the filter", async ({ page }, info) => {
  const s = await seedTask(page, info);
  await openAt(page);
  await expect(layoutOf(page, "テーブル")).toHaveAttribute("aria-pressed", "true");
  await expect(layoutOf(page, "ツリー")).toHaveAttribute("aria-pressed", "false");
  await search(page, s.query);
  await addStartColumn(page);
  const check = page.getByRole("checkbox", { name: `${s.prefix}-2 を選択` });
  await check.check();
  await expect(page.getByText("1 件を選択中", { exact: true })).toBeVisible();

  await choose(page, "ツリー");
  await expect(rowOf(treeOf(page), e2eStore, s.task)).toBeVisible();
  await expect(filterOf(page)).toHaveValue(s.query);
  // ツリーには選択も設定もない(E3a-7・10)。件数だけを出す。
  await expect(page.getByText("1 件を選択中", { exact: true })).toBeHidden();
  await expect(page.getByRole("button", { name: "表示する列", exact: true })).toBeHidden();
  await expect(page.getByText("1 件", { exact: true })).toBeVisible();

  await choose(page, "テーブル");
  await expect(filterOf(page)).toHaveValue(s.query);
  await expect(lastHeader(page)).toHaveText("開始日");
  await expect(check).not.toBeChecked();
  await expect(page.getByText("1 件を選択中", { exact: true })).toBeHidden();
});

// Review Focus 5: ツリーにしたあと、再読み込みではツリーのまま開き、サイドバーの「検索」ではテーブルに戻る。
test("reload keeps tree, sidebar opens table", async ({ page, context }, info) => {
  const s = await seedTask(page, info);
  await openAt(page);
  await search(page, s.query);
  await addStartColumn(page);
  await choose(page, "ツリー");

  await page.reload();
  await expect(rowOf(treeOf(page), e2eStore, s.task)).toBeVisible();
  await expect(layoutOf(page, "ツリー")).toHaveAttribute("aria-pressed", "true");
  await expect(filterOf(page)).toHaveValue(s.query);

  // すでに検索の画面にいても、テーブルに切り替える。
  const put = saved(page);
  await clickSearchLink(page, info, true);
  await put;
  await expect(layoutOf(page, "テーブル")).toHaveAttribute("aria-pressed", "true");
  await expect(lastHeader(page)).toHaveText("開始日");
  await expect(filterOf(page)).toHaveValue(s.query);

  // 新しいブラウザのタブで、ほかの画面からサイドバーの「検索」を押しても、保存した絞り込みと列は残る。
  await choose(page, "ツリー");
  const other = await context.newPage();
  await other.goto(tokenUrl());
  await expect(other.getByRole("main")).toBeVisible();
  const read = loaded(other);
  await clickSearchLink(other, info, false);
  await read;
  await expect(other).toHaveURL(/\/search$/);
  await expect(layoutOf(other, "テーブル")).toHaveAttribute("aria-pressed", "true");
  await expect(filterOf(other)).toHaveValue(s.query);
  await expect(lastHeader(other)).toHaveText("開始日");
  await expect(other.getByText("1 件", { exact: true })).toBeVisible();
});

test("notes for sort and group", async ({ page }) => {
  await resetSearchScreen(page.request, "tree");
  await openAt(page);
  await search(page, "sort:id group:kind");
  const note = page.getByText("ツリー・ガントでは sort: と group: は使われず、並び順のままです", {
    exact: true,
  });
  await expect(note).toBeVisible();
  await search(page, "sort:id");
  await expect(
    page.getByText("ツリー・ガントでは sort: は使われず、並び順のままです", { exact: true }),
  ).toBeVisible();
  await search(page, "sort:id group:kind");
  await expect(note).toBeVisible();
  await choose(page, "テーブル");
  await expect(note).toBeHidden();
});

test("deleted shows guidance", async ({ page }) => {
  await resetSearchScreen(page.request, "tree");
  await openAt(page);
  await search(page, "is:deleted");
  await expect(
    page.getByText("削除済みのアイテムはテーブルで表示します。", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("0 件", { exact: true })).toBeVisible();
  await expect(treeOf(page)).toBeHidden();
  const put = saved(page);
  await page.getByRole("button", { name: "テーブルで開く", exact: true }).click();
  await put;
  await expect(layoutOf(page, "テーブル")).toHaveAttribute("aria-pressed", "true");
  await expect(filterOf(page)).toHaveValue("is:deleted");
  await expect(page.getByText("削除済みのアイテムはテーブルで表示します。")).toBeHidden();
});

// パレットの入口(E3a-9)も、ツリーからテーブルにする。
test("palette opens in table", async ({ page }, info) => {
  const s = await seedTask(page, info);
  await resetSearchScreen(page.request, "tree");
  await openAt(page);
  await expect(layoutOf(page, "ツリー")).toHaveAttribute("aria-pressed", "true");

  // 「このクエリで」の「検索で開く」(検索の画面にいるとき)。
  await page.keyboard.press("ControlOrMeta+k");
  await paletteOf(page).getByRole("combobox").fill(s.query);
  await paletteOf(page)
    .getByRole("option", { name: /^検索で開く/ })
    .click();
  await expect(paletteOf(page)).toBeHidden();
  await expect(layoutOf(page, "テーブル")).toHaveAttribute("aria-pressed", "true");
  await expect(filterOf(page)).toHaveValue(s.query);

  // コマンド「検索を開く」(検索の画面にいるとき)。
  await choose(page, "ツリー");
  await page.keyboard.press("ControlOrMeta+k");
  await paletteOf(page).getByRole("combobox").fill("検索を開く");
  await paletteOf(page)
    .getByRole("option", { name: /^検索を開く/ })
    .click();
  await expect(paletteOf(page)).toBeHidden();
  await expect(layoutOf(page, "テーブル")).toHaveAttribute("aria-pressed", "true");

  // 「{N} 件をまとめて変更…」(ほかの画面から)。
  await choose(page, "ツリー");
  await page.goto(tokenUrl());
  await expect(page.getByRole("main")).toBeVisible();
  await page.keyboard.press("ControlOrMeta+k");
  await paletteOf(page).getByRole("combobox").fill(s.query);
  await paletteOf(page)
    .getByRole("option", { name: /^1 件をまとめて変更…/ })
    .click();
  await expect(page.getByRole("dialog", { name: "選択したアイテムを一括変更" })).toBeVisible();
  await expect(layoutOf(page, "テーブル")).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByText("1 件を選択中", { exact: true })).toBeVisible();
});

// ツリーに戻したとき、組み直すまで前の件数を出さない。テーブルにしているあいだに届いた前の組み直しは捨てる。
test("does not show a stale tree count after switching back", async ({ page }, info) => {
  const s = await seedTask(page, info);
  const p2 = await s.project(2);
  await s.item(p2.id, "Epic", `${s.mark} F`);
  // 空の絞り込みで開くとすべてのプロジェクトを読んでしまうので、絞り込みを入れた画面の状態で開く(2 つめのプロジェクトは
  // まだ読まない)。
  await resetSearchScreen(page.request, "tree", s.query);
  await openAt(page);
  await expect(page.getByText("1 件", { exact: true })).toBeVisible();

  // プロジェクトのアイテムの読み込みを止めておく。
  let open = () => {};
  let gate = new Promise<void>((r) => (open = r));
  await page.route("**/api/stores/*/projects/*/items", async (route) => {
    await gate;
    await route.continue();
  });
  const late = page.waitForResponse((r) => r.url().endsWith(`/projects/${p2.id}/items`));
  await search(page, `kind:epic ${s.mark}`);
  await choose(page, "テーブル");
  open();
  await late;

  gate = new Promise<void>((r) => (open = r));
  await choose(page, "ツリー");
  await expect(page.getByText(/^\d+ 件$/)).toHaveCount(0);
  open();
  await expect(page.getByText("2 件", { exact: true })).toBeVisible();
});
