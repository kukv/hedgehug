import { expect, test } from "./fixtures";
import {
  currentOf,
  e2eStore,
  openTree,
  pick,
  readonlyStore,
  rowOf,
  sampleEpic,
  sampleProject,
  sampleTask,
  search,
  seed,
  titlesWith,
} from "./search-tree-helpers";

// 検索の画面のツリー(E3a。04「ツリー」)の通しのテスト。描き方・開閉・Store をまたいだ同じ ID・見えている兄弟での並べ替え。
// キー操作とプロジェクトの行は search-tree-keys.spec.ts。

test("draws matches with dim ancestors", async ({ page }, info) => {
  const s = await seed(page, info);
  const p1 = await s.project(1);
  const p2 = await s.project(2);
  const epic = await s.item(p1.id, "Epic", `${s.mark} E`);
  const task = await s.item(p1.id, "Task", `${s.mark} T`, epic);
  const other = await s.item(p1.id, "Story", `${s.mark} S`, epic);
  const elsewhere = await s.item(p2.id, "Epic", `${s.mark} F`);

  const tree = await openTree(page, `kind:task ${s.mark}`);
  await expect(rowOf(tree, e2eStore, task)).toBeVisible();
  // ワークスペースの見出し(Store の札とワークスペースの名前)。
  await expect(tree.getByText(`樹 ${s.prefix}`, { exact: true })).toBeVisible();
  await expect(rowOf(tree, e2eStore, p1.id)).toBeVisible();
  // 一致しない祖先は薄く、一致した行は薄くない。
  await expect(rowOf(tree, e2eStore, epic)).toHaveClass(/\bdim\b/);
  await expect(rowOf(tree, e2eStore, task)).not.toHaveClass(/\bdim\b/);
  // 一致を含まない行とプロジェクトは出ない。
  await expect(rowOf(tree, e2eStore, other)).toHaveCount(0);
  await expect(rowOf(tree, e2eStore, p2.id)).toHaveCount(0);
  await expect(rowOf(tree, e2eStore, elsewhere)).toHaveCount(0);
  // 件数は一致した行の数(薄い祖先とプロジェクトの行は数えない)。
  await expect(page.getByText("1 件", { exact: true })).toBeVisible();
});

test("collapse survives reload", async ({ page }, info) => {
  const s = await seed(page, info);
  const p1 = await s.project(1);
  const epic = await s.item(p1.id, "Epic", `${s.mark} E`);
  const task = await s.item(p1.id, "Task", `${s.mark} T`, epic);

  const tree = await openTree(page, s.mark);
  await expect(rowOf(tree, e2eStore, task)).toBeVisible();
  const saved = page.waitForResponse(
    (r) => r.url().includes("/api/screen-states/search") && r.request().method() === "PUT",
  );
  await rowOf(tree, e2eStore, epic).getByRole("button", { name: "開閉" }).click();
  await saved;
  await expect(rowOf(tree, e2eStore, task)).toHaveCount(0);

  await page.reload();
  await expect(rowOf(tree, e2eStore, epic)).toBeVisible();
  await expect(rowOf(tree, e2eStore, epic).getByRole("button", { name: "開閉" })).toHaveAttribute(
    "aria-expanded",
    "false",
  );
  await expect(rowOf(tree, e2eStore, task)).toHaveCount(0);
});

// Review Focus 1: Store「e2e」と Store「readonly」のサンプルは同じ内部 ID なので、行の鍵に Store を入れないと開閉と
// フォーカスがもう片方に効く。読むだけで、書き換えない。
test("same id in two stores", async ({ page }) => {
  const tree = await openTree(page, "ログイン画面を作る");
  const [epicA, taskA] = [rowOf(tree, e2eStore, sampleEpic), rowOf(tree, e2eStore, sampleTask)];
  const [epicB, taskB] = [
    rowOf(tree, readonlyStore, sampleEpic),
    rowOf(tree, readonlyStore, sampleTask),
  ];
  await expect(taskA).toBeVisible();
  await expect(taskB).toBeVisible();

  await epicA.getByRole("button", { name: "開閉" }).click();
  await expect(taskA).toHaveCount(0);
  await expect(taskB).toBeVisible();
  await expect(epicB.getByRole("button", { name: "開閉" })).toHaveAttribute(
    "aria-expanded",
    "true",
  );
  await epicA.getByRole("button", { name: "開閉" }).click();
  await expect(taskA).toBeVisible();

  // ↓ で、2 つの Store の行を別の行として順に移る。
  const current = currentOf(tree);
  await pick(epicA);
  await expect(current).toHaveAttribute("data-row", `${e2eStore}/${sampleEpic}`);
  for (const key of [
    `${e2eStore}/${sampleTask}`,
    `${readonlyStore}/${sampleProject}`,
    `${readonlyStore}/${sampleEpic}`,
    `${readonlyStore}/${sampleTask}`,
  ]) {
    await page.keyboard.press("ArrowDown");
    await expect(current).toHaveAttribute("data-row", key);
  }
});

// Review Focus 2: 兄弟が A・B・C・D・F で B・D が一致しないとき、C の「上へ移動」は見えている A の前へ動かす
// (並びは C・A・B・D・F)。
test("move up among visible siblings", async ({ page }, info) => {
  const s = await seed(page, info);
  const p1 = await s.project(1);
  const epic = await s.item(p1.id, "Epic", `${s.mark}親`);
  for (const name of ["合 A", "別 B", "合 C", "別 D", "合 F"]) {
    await s.item(p1.id, "Task", `${s.mark}${name}`, epic);
  }

  const tree = await openTree(page, `${s.mark}合`);
  expect(await titlesWith(tree, s.mark)).toEqual([
    `${s.mark}親`,
    `${s.mark}合 A`,
    `${s.mark}合 C`,
    `${s.mark}合 F`,
  ]);
  await tree.getByRole("button", { name: `${s.prefix}-4 のメニュー` }).click();
  await page.getByRole("menuitem", { name: /^上へ移動/ }).click();
  await expect(tree.getByRole("link").filter({ hasText: s.mark }).nth(1)).toHaveText(
    `${s.mark}合 C`,
  );

  await search(page, s.mark);
  await expect(tree.getByRole("link", { name: `${s.mark}別 B` })).toBeVisible();
  expect(await titlesWith(tree, s.mark)).toEqual([
    `${s.mark}親`,
    `${s.mark}合 C`,
    `${s.mark}合 A`,
    `${s.mark}別 B`,
    `${s.mark}別 D`,
    `${s.mark}合 F`,
  ]);
});
