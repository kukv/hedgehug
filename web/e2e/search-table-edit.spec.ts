import type { Locator, Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { auth, base, e2eStore, openWith, rowOf, seed, send } from "./search-table-helpers";

// 検索のテーブルのセルの編集の通しのテスト(E2b。04「セル」)。作ったアイテムのタイトルにテストごとの印を入れて絞り込む。

const item = async (page: Page, id: string) =>
  (await page.request.get(`/api/stores/${e2eStore}/items/${id}`, { headers: auth })).json();

// showColumns は、「表示する列」のメニューで列を足す(画面の状態の保存を待つ)。
async function showColumns(page: Page, names: string[]) {
  await page.getByRole("button", { name: "表示する列", exact: true }).click();
  const menu = page.getByRole("menu", { name: "表示する列" });
  for (const name of names) {
    const saved = page.waitForResponse(
      (r) =>
        r.url().includes("/api/screen-states/search") && r.request().method() === "PUT" && r.ok(),
    );
    await menu.getByRole("menuitemcheckbox", { name }).click();
    await saved;
  }
  await page.keyboard.press("Escape");
}

// tabUntil は、to にフォーカスが移るまで Tab を押す(日付の欄は年・月・日の順に Tab が進む)。
async function tabUntil(page: Page, to: Locator) {
  for (let i = 0; i < 8 && !(await to.evaluate((el) => el === document.activeElement)); i++) {
    await page.keyboard.press("Tab");
  }
  await expect(to).toBeFocused();
}

// saved は、操作のあとの読み直し(検索の応答)まで待つ。
async function saved(page: Page, act: () => Promise<void>) {
  const done = page.waitForResponse(
    (r) => r.url().endsWith("/api/search") && r.request().method() === "POST",
  );
  await act();
  await done;
}

const toast = (page: Page) => page.getByText("保存できませんでした");

test("changes the priority from the cell menu", async ({ page }, info) => {
  const { mark, storyId } = await seed(page, info);
  await openWith(page, info, mark);
  const row = rowOf(page, `${mark} B`);
  await row.getByRole("button", { name: /^優先度: Medium/ }).click();
  await page
    .getByRole("menu", { name: "優先度" })
    .getByRole("menuitemcheckbox", { name: "High", exact: true })
    .click();
  await expect(row.getByRole("button", { name: /^優先度: High/ })).toBeVisible();
  await expect(toast(page)).toBeHidden();
  const saved = await item(page, storyId);
  const settings = await (
    await page.request.get(`${base}/workspaces/${saved.workspace}`, { headers: auth })
  ).json();
  const high = settings.settings.priorities.find((p: { name: string }) => p.name === "High");
  expect(saved.priority).toBe(high.id);
});

test("clears points by emptying the input", async ({ page }, info) => {
  const { mark, storyId } = await seed(page, info);
  await openWith(page, info, mark);
  const points = rowOf(page, `${mark} B`).getByRole("spinbutton", { name: "ポイント" });
  await saved(page, async () => {
    await points.fill("3");
    await points.press("Enter");
  });
  await expect(points).toHaveValue("3");
  await expect.poll(async () => (await item(page, storyId)).points).toBe(3);
  await saved(page, async () => {
    await points.fill("");
    await points.press("Enter");
  });
  await expect(points).toHaveValue("");
  await expect.poll(async () => (await item(page, storyId)).points).toBeNull();
});

test("tab from start to end saves both", async ({ page }, info) => {
  const { mark, storyId } = await seed(page, info);
  await openWith(page, info, mark);
  // 終了日を外して入れ直し、開始日の右隣(列の末尾)に並べる(Tab で開始日から終了日へ移れるように)。
  await showColumns(page, ["終了日", "開始日", "終了日"]);
  const row = rowOf(page, `${mark} B`);
  await row.getByLabel("開始日", { exact: true }).fill("2026-11-20");
  const end = row.getByLabel("終了日", { exact: true });
  await tabUntil(page, end);
  await end.fill("2026-11-25");
  await tabUntil(page, row.getByRole("button", { name: /のメニュー$/ }));
  await expect
    .poll(async () => {
      const it = await item(page, storyId);
      return `${it.start} ${it.end}`;
    })
    .toBe("2026-11-20 2026-11-25");
  await expect(toast(page)).toBeHidden();
});

test("the systems menu stays open while choosing two", async ({ page }, info) => {
  const { mark, storyId } = await seed(page, info);
  await openWith(page, info, mark);
  await showColumns(page, ["関連システム"]);
  await rowOf(page, `${mark} B`)
    .getByRole("button", { name: /^関連システム/ })
    .click();
  const menu = page.getByRole("menu", { name: "関連システム" });
  await menu.getByRole("menuitemcheckbox", { name: "Web", exact: true }).click();
  await expect(menu).toBeVisible();
  await menu.getByRole("menuitemcheckbox", { name: "API", exact: true }).click();
  await expect(menu).toBeVisible();
  await expect(menu.getByRole("menuitemcheckbox", { name: "Web", exact: true })).toHaveAttribute(
    "aria-checked",
    "true",
  );
  await expect.poll(async () => (await item(page, storyId)).systems.length).toBe(2);
});

test("picks an assignee and a sprint from selects", async ({ page }, info) => {
  const { mark, storyId, workspaceId, prefix } = await seed(page, info);
  const sprint = await send(page, "post", `${base}/workspaces/${workspaceId}/sprints`, {
    name: `S${prefix}`,
    goal: "",
    start: "",
    end: "",
  });
  await openWith(page, info, mark);
  await showColumns(page, ["スプリント"]);
  const row = rowOf(page, `${mark} B`);
  const who = row.getByRole("combobox", { name: "担当者" });
  const labels = await who.locator("option").allTextContents();
  expect(labels).toContain("未割り当て");
  const me = labels.find((l) => l.includes("(あなた)"));
  expect(me).toBeDefined();
  const login = await who.locator("option", { hasText: "(あなた)" }).getAttribute("value");
  expect(login).toBeTruthy();
  expect((await item(page, storyId)).assignee).toBe("");
  await who.selectOption({ label: me ?? "" });
  await expect.poll(async () => (await item(page, storyId)).assignee).toBe(login);

  await row.getByRole("combobox", { name: "スプリント" }).selectOption({ label: `S${prefix}` });
  await expect.poll(async () => (await item(page, storyId)).sprint).toBe(sprint.id);
  await expect(toast(page)).toBeHidden();
  const names = await page.getByRole("main").getByRole("columnheader").allTextContents();
  const col = names.findIndex((n) => n.trim() === "スプリント");
  await expect(rowOf(page, `${mark} A`).getByRole("cell").nth(col)).toHaveText("—");
});

test("the status button follows the workflow", async ({ page }, info) => {
  const { mark, storyId } = await seed(page, info);
  await openWith(page, info, mark);
  const row = rowOf(page, `${mark} B`);
  await row.getByRole("button", { name: /^ステータス: / }).click();
  const menu = page.getByRole("menu", { name: "ステータス" });
  const before = (await item(page, storyId)).status;
  await menu.getByRole("menuitemcheckbox", { name: /^Done/ }).click();
  await expect(row.getByRole("button", { name: "ステータス: Done" })).toBeVisible();
  await expect.poll(async () => (await item(page, storyId)).status).not.toBe(before);
});

test("read-only rows disable the cells", async ({ page }, info) => {
  await openWith(page, info, "store:readonly ログイン画面");
  const row = page.getByRole("row", { name: /HH-2/ });
  for (const control of [
    row.getByRole("button", { name: /^優先度: / }),
    row.getByRole("button", { name: /^ステータス: / }),
    row.getByRole("spinbutton", { name: "ポイント" }),
  ]) {
    await expect(control).toBeDisabled();
    await expect(control).toHaveAttribute("title", "閲覧のみの Store です");
  }
  await expect(row.getByRole("checkbox")).toBeEnabled();
  // 列見出しの並べ替えも使える。
  const title = page.getByRole("main").getByRole("columnheader", { name: "タイトル", exact: true });
  await title.getByRole("button").click();
  await expect(title).toHaveAttribute("aria-sort", "ascending");
});

test("deleted rows show values without editors", async ({ page }, info) => {
  const { mark, storyId } = await seed(page, info);
  await send(page, "delete", `${base}/items/${storyId}`, undefined);
  await openWith(page, info, `is:deleted ${mark}`);
  const row = rowOf(page, `${mark} B`);
  await expect(row).toBeVisible();
  await expect(row.getByRole("button", { name: /^優先度/ })).toHaveCount(0);
  await expect(row.getByRole("spinbutton")).toHaveCount(0);
  await expect(row.getByRole("cell", { name: "Medium", exact: true })).toBeVisible();
});

test("a refused end date goes back to the saved value", async ({ page }, info) => {
  const { mark } = await seed(page, info);
  await openWith(page, info, mark);
  await showColumns(page, ["終了日", "開始日", "終了日"]);
  const row = rowOf(page, `${mark} B`);
  const end = row.getByLabel("終了日", { exact: true });
  // 開始日の保存と読み直しが終わってから終了日を入れる(保存が前後すると、断られる理由が変わる)。
  await saved(page, async () => {
    await row.getByLabel("開始日", { exact: true }).fill("2026-11-20");
    await tabUntil(page, end);
  });
  await saved(page, async () => {
    await end.fill("2026-11-10");
    await tabUntil(page, row.getByRole("button", { name: /のメニュー$/ }));
  });
  await expect(page.getByText("終了日は開始日以降にしてください")).toBeVisible();
  await expect(end).toHaveValue("");
});

test("editing the grouped column moves the row without carrying the menu", async ({
  page,
}, info) => {
  const { mark } = await seed(page, info);
  await openWith(page, info, mark);
  await page.getByRole("combobox", { name: "グループ" }).selectOption({ label: "優先度" });
  await expect(page.getByRole("row", { name: "Medium 2", exact: true })).toBeVisible();
  const row = rowOf(page, `${mark} B`);
  await row.getByRole("button", { name: /^優先度: Medium/ }).click();
  await page
    .getByRole("menu", { name: "優先度" })
    .getByRole("menuitemcheckbox", { name: "High", exact: true })
    .click();
  await expect(page.getByRole("row", { name: "High 1", exact: true })).toBeVisible();
  await expect(page.getByRole("menu", { name: "優先度" })).toBeHidden();
  const a = rowOf(page, `${mark} A`);
  await expect(a.getByRole("button", { name: /^優先度: Medium/ })).toBeVisible();
  const b = rowOf(page, `${mark} B`);
  await expect(b.getByRole("button", { name: /^優先度: High/ })).toBeVisible();
});

test("a row whose workspace was deleted elsewhere shows the save failure", async ({
  page,
}, info) => {
  const { mark, workspaceId } = await seed(page, info);
  await openWith(page, info, mark);
  const row = rowOf(page, `${mark} B`);
  const points = row.getByRole("spinbutton", { name: "ポイント" });
  await expect(points).toBeVisible();
  await send(page, "delete", `${base}/workspaces/${workspaceId}`, undefined);
  await points.fill("5");
  await points.press("Enter");
  await expect(toast(page)).toBeVisible();
  await expect(row).toHaveCount(0);
});

test("saving a cell of a row deleted elsewhere restores it and shows the toast", async ({
  page,
}, info) => {
  const { mark, storyId, prefix } = await seed(page, info);
  await openWith(page, info, mark);
  const row = rowOf(page, `${mark} B`);
  const points = row.getByRole("spinbutton", { name: "ポイント" });
  await expect(points).toBeVisible();
  await send(page, "delete", `${base}/items/${storyId}`, undefined);
  await saved(page, async () => {
    await points.fill("7");
    await points.press("Enter");
  });
  await expect(
    page.getByText(
      `${prefix}-2 は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました`,
    ),
  ).toBeVisible();
  await expect(toast(page)).toBeHidden();
  await expect(row).toBeVisible();
  await expect(row.getByText("削除済み")).toHaveCount(0);
  await expect.poll(async () => (await item(page, storyId)).points).toBe(7);
});
