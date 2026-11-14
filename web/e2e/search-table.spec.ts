import { expect, test } from "./fixtures";
import { resetSearchScreen, tokenUrl } from "./helpers";
import {
  base,
  headers,
  lastHeader,
  openWith,
  rowOf,
  seed,
  send,
  type,
} from "./search-table-helpers";

// 検索のテーブルの通しのテスト(E2a。04「テーブル」)。作ったアイテムのタイトルにテストごとの印を入れて絞り込み、
// サンプルや同じサーバーを使うほかのテストの件数に頼らない。

test("shows a column chosen in the menu at the right end, and keeps it after a reload", async ({
  page,
}, info) => {
  const { mark } = await seed(page, info);
  await openWith(page, info, mark);
  await page.getByRole("button", { name: "表示する列", exact: true }).click();
  const menu = page.getByRole("menu", { name: "表示する列" });
  await expect(menu.getByRole("menuitemcheckbox", { name: "タイトル" })).toBeDisabled();
  const saved = page.waitForResponse(
    (r) =>
      r.url().includes("/api/screen-states/search") && r.request().method() === "PUT" && r.ok(),
  );
  await menu.getByRole("menuitemcheckbox", { name: "開始日" }).click();
  await saved;
  await expect(lastHeader(page)).toHaveText("開始日");
  await expect(menu).toBeVisible();
  await expect(menu.getByRole("menuitemcheckbox", { name: "開始日" })).toHaveAttribute(
    "aria-checked",
    "true",
  );

  // 開いているときにボタンを押すと、閉じたままになる(開き直さない)。
  await page.getByRole("button", { name: "表示する列", exact: true }).click();
  await expect(menu).toBeHidden();

  await page.reload();
  await expect(lastHeader(page)).toHaveText("開始日");
});

test("sorts by the title header: ascending, descending, then none", async ({ page }, info) => {
  const { mark } = await seed(page, info);
  await openWith(page, info, mark);
  // 矢印は読み上げに含めないので、並べ替えても見出しの名前は「タイトル」のまま(向きは aria-sort)。
  const title = page.getByRole("main").getByRole("columnheader", { name: "タイトル", exact: true });
  const rows = rowOf(page, mark);

  await title.getByRole("button").click();
  await expect(title).toHaveAttribute("aria-sort", "ascending");
  await expect(title).toContainText("↑");
  await expect(rows.first()).toContainText(`${mark} A`);

  await title.getByRole("button").click();
  await expect(title).toHaveAttribute("aria-sort", "descending");
  await expect(title).toContainText("↓");
  await expect(rows.first()).toContainText(`${mark} B`);

  await title.getByRole("button").click();
  await expect(title).toHaveAttribute("aria-sort", "none");
  await expect(title).not.toContainText(/[↑↓]/);
});

test("groups by the group setting, or by group: when the setting is none", async ({
  page,
}, info) => {
  const { mark } = await seed(page, info);
  await openWith(page, info, mark);
  const group = page.getByRole("combobox", { name: "グループ" });
  const epicHead = page.getByRole("row", { name: "Epic 1", exact: true });
  const storyHead = page.getByRole("row", { name: "Story 1", exact: true });

  await group.selectOption({ label: "種類" });
  await expect(epicHead).toBeVisible();
  await expect(storyHead).toBeVisible();

  await group.selectOption({ label: "なし" });
  await expect(epicHead).toBeHidden();
  await type(page, `group:kind ${mark}`);
  await expect(epicHead).toBeVisible();
  await expect(storyHead).toBeVisible();

  // 設定が「なし」でなければ、`group:` より設定を使う。
  await group.selectOption({ label: "担当者" });
  await expect(page.getByRole("row", { name: "未割り当て 2", exact: true })).toBeVisible();
  await expect(epicHead).toBeHidden();
  // ツールバーの件数は、グループ分けをしても行の数のまま。
  await expect(page.getByText("2 件", { exact: true })).toBeVisible();
});

test("leaves the systems cell empty, and shows — for a property the kind does not use", async ({
  page,
}, info) => {
  const { mark, workspaceId } = await seed(page, info);
  // Epic では優先度を使わない設定にする(初めの設定ではすべての種類が使う)。
  await send(page, "put", `${base}/workspaces/${workspaceId}/properties/priority/kinds`, {
    kinds: ["Story", "Task", "Bug", "Step"],
  });
  await openWith(page, info, mark);
  await page.getByRole("button", { name: "表示する列", exact: true }).click();
  await page
    .getByRole("menu", { name: "表示する列" })
    .getByRole("menuitemcheckbox", { name: "関連システム" })
    .click();
  await page.keyboard.press("Escape");
  await expect(lastHeader(page)).toHaveText("関連システム");

  const names = await headers(page).allTextContents();
  const systems = names.findIndex((n) => n.trim() === "関連システム");
  const priority = names.findIndex((n) => n.trim() === "優先度");
  const epic = rowOf(page, `${mark} A`);
  const story = rowOf(page, `${mark} B`);
  await expect(epic.getByRole("cell").nth(systems)).toHaveText("");
  await expect(epic.getByRole("cell").nth(priority)).toHaveText("—");
  await expect(story.getByRole("cell").nth(priority)).not.toHaveText("—");
});

test("the palette's bulk change opens the table with every row selected and the dialog open", async ({
  page,
}, info) => {
  const { mark } = await seed(page, info);
  await resetSearchScreen(page.request);
  // 検索でない画面(ホーム)から開く。
  await page.goto(tokenUrl());
  await expect(page.getByRole("main")).toBeVisible();
  await page.keyboard.press("ControlOrMeta+k");
  const palette = page.getByRole("dialog", { name: "コマンドパレット" });
  const query = `${mark} kind:Story`;
  await palette.getByRole("combobox").fill(query);
  await palette.getByRole("option", { name: /^1 件をまとめて変更…/ }).click();
  await expect(palette).toBeHidden();
  await expect(page).toHaveURL(/\/search$/);
  await expect(page.getByRole("combobox", { name: "絞り込み" })).toHaveValue(query);
  await expect(page.getByText("1 件を選択中", { exact: true })).toBeVisible();
  await expect(page.getByRole("dialog", { name: "選択したアイテムを一括変更" })).toBeVisible();
});
