import type { Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { sidebar } from "./helpers";
import {
  currentOf,
  e2eStore,
  openTree,
  pick,
  readonlyStore,
  rowKeys,
  rowOf,
  sampleEpic,
  sampleProject,
  seed,
  titlesWith,
} from "./search-tree-helpers";

// 検索の画面のツリーのキー操作とプロジェクトの行(E3a-16・17、04「ツリーのキー操作」)の通しのテスト。

// writesTo は、アドレスが pattern に合う、GET 以外の要求を集める。
function writesTo(page: Page, pattern: RegExp) {
  const seen: string[] = [];
  page.on("request", (r) => {
    if (r.method() !== "GET" && pattern.test(r.url())) seen.push(`${r.method()} ${r.url()}`);
  });
  return seen;
}
// moveRequests は、プロジェクトの並べ替えの API(E3a-15)を呼んだものを集める。
const moveRequests = (page: Page) => writesTo(page, /\/projects\/[^/]+\/move$/);

test("space and alt-up on item rows", async ({ page }, info) => {
  const s = await seed(page, info);
  const p1 = await s.project(1);
  const epic = await s.item(p1.id, "Epic", `${s.mark} E`);
  await s.item(p1.id, "Task", `${s.mark} A`, epic);
  const b = await s.item(p1.id, "Task", `${s.mark} B`, epic);

  const tree = await openTree(page, s.mark);
  const rowB = rowOf(tree, e2eStore, b);
  await pick(rowB);
  await expect(currentOf(tree)).toHaveAttribute("data-row", `${e2eStore}/${b}`);
  // Space は、完了の既定のステータスにする。
  await page.keyboard.press("Space");
  await expect(rowB.getByRole("button", { name: /^ステータス: / })).toHaveAccessibleName(
    "ステータス: Done",
  );
  // Alt+↑ は、見えている兄弟の前へ動かし、フォーカスは動かした行に残る。
  await page.keyboard.press("Alt+ArrowUp");
  await expect
    .poll(() => titlesWith(tree, s.mark))
    .toEqual([`${s.mark} E`, `${s.mark} B`, `${s.mark} A`]);
  await expect(currentOf(tree)).toHaveAttribute("data-row", `${e2eStore}/${b}`);
});

// Review Focus 3: 閲覧のみの Store の行と書ける Store の行が並んでも、書き込みの判定はフォーカスのある行の Store で決める。
// サンプルを読むだけで、書き換えない(作成ダイアログは開いたら閉じる)。
test("read-only row next to writable row", async ({ page }) => {
  const moves = moveRequests(page);
  const readonlyWrites = writesTo(page, new RegExp(`/api/stores/${readonlyStore}/`));
  const tree = await openTree(page, "認証");
  const roEpic = rowOf(tree, readonlyStore, sampleEpic);
  const status = roEpic.getByRole("button", { name: /^ステータス: / });
  const before = await status.getAttribute("aria-label");

  // 閲覧のみの Store の Epic: C・Space は何もしない。
  await pick(roEpic);
  await page.keyboard.press("c");
  await page.keyboard.press("Space");
  // 閲覧のみの Store のプロジェクトの行: C・Alt+↑↓ は何もしない。
  await pick(rowOf(tree, readonlyStore, sampleProject));
  for (const key of ["c", "Alt+ArrowUp", "Alt+ArrowDown"]) await page.keyboard.press(key);
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(status).toHaveAttribute("aria-label", before ?? "");

  // 書ける Store の Epic の C は、作成ダイアログを開く。
  await pick(rowOf(tree, e2eStore, sampleEpic));
  await page.keyboard.press("c");
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await expect(dialog).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  // ダイアログを開き閉じするあいだに、閲覧のみの Store へ書く要求が飛んでいれば、ここで分かる。
  expect(moves).toEqual([]);
  expect(readonlyWrites).toEqual([]);
});

test("project row keys and menu", async ({ page }, info) => {
  const moves = moveRequests(page);
  const s = await seed(page, info);
  const [p1, p2, p3] = [await s.project(1), await s.project(2), await s.project(3)];
  const epic = await s.item(p1.id, "Epic", `${s.mark} E`);
  await s.item(p2.id, "Epic", `${s.mark} F`);
  await s.item(p3.id, "Epic", `${s.mark} G`);

  const tree = await openTree(page, s.mark);
  const projectsOf = async () => (await titlesWith(tree, `樹の計画${s.prefix}`)).join(",");
  const current = currentOf(tree);
  // Alt+↓ は、プロジェクトのメニューの「下へ移動」と同じ(サイドバーの並びを動かす)。
  await pick(rowOf(tree, e2eStore, p1.id));
  await page.keyboard.press("Alt+ArrowDown");
  await expect.poll(projectsOf).toBe([p2.title, p1.title, p3.title].join(","));
  await expect(current).toHaveAttribute("data-row", `${e2eStore}/${p1.id}`);
  // 末尾のプロジェクトの Alt+↓ は何もしない(API を呼ばない)。
  await pick(rowOf(tree, e2eStore, p3.id));
  await page.keyboard.press("Alt+ArrowDown");
  expect(moves).toHaveLength(1);

  // メニューの「名前を変更…」。
  const renamed = `${p1.title} 改`;
  await tree.getByRole("button", { name: `${p1.title} のメニュー` }).click();
  await page.getByRole("menuitem", { name: /^名前を変更…/ }).click();
  const dialog = page.getByRole("dialog", { name: "プロジェクトの名前を変更" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(renamed);
  await dialog.getByRole("button", { name: "保存" }).click();
  await expect(dialog).toBeHidden();
  await expect(rowOf(tree, e2eStore, p1.id).getByRole("link")).toHaveText(renamed);

  // Epic の Alt+Shift+↓ は、見えている次のプロジェクト(P3)の下へ移す。
  await pick(rowOf(tree, e2eStore, epic));
  await page.keyboard.press("Alt+Shift+ArrowDown");
  await expect(rowOf(tree, e2eStore, p1.id)).toHaveCount(0);
  const keys = await rowKeys(tree);
  expect(keys.indexOf(`${e2eStore}/${epic}`)).toBeGreaterThan(keys.indexOf(`${e2eStore}/${p3.id}`));

  // Enter は、プロジェクトの概要を開く。
  await pick(rowOf(tree, e2eStore, p2.id));
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(new RegExp(`/s/e2e/p/${s.prefix}-P2/overview$`));

  // サイドバーの並びも変わっている。
  const side = await sidebar(page, info);
  const links = side
    .getByRole("region", { name: "e2e" })
    .getByRole("link", { name: new RegExp(`樹の計画${s.prefix}`) });
  await expect(links).toHaveText([p2.title, renamed, p3.title]);
  expect(moves).toHaveLength(1);
});
