import type { Locator, Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import {
  createWorkspace,
  prefixOf,
  projectWithEpic,
  storyUnderEpic,
  tokenUrl,
  unique,
} from "./helpers";

// バックログのタブの通しのテスト(F2a)。ワークスペースは毎回作る。

// rowIn は、枠の中の、表示の ID の行。
const rowIn = (frame: Locator, id: string) =>
  frame.locator("[data-row]").filter({ has: frame.page().getByText(id, { exact: true }) });

// ids は、枠の行の表示の ID を上から順に返す。
const ids = (frame: Locator) => frame.locator("[data-row] code.id").allTextContents();

// dragHandle は、行の持ち手を、別の行の上までマウスで運んで離す。
async function dragHandle(page: Page, handle: Locator, target: Locator) {
  // 狭い幅では、行が枠からはみ出して横にスクロールしている。持ち手を画面の中に出してから位置を読む。
  await handle.scrollIntoViewIfNeeded();
  const from = await handle.boundingBox();
  const to = await target.boundingBox();
  if (!from || !to) throw new Error("dragHandle: 要素の位置を読めません");
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(to.x + to.width / 2, to.y + to.height / 2, { steps: 12 });
  await page.mouse.up();
}

test("creates, reorders and plans items on the backlog", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "計画"), prefix);
  // プロジェクト(接頭辞-P1)、Epic(接頭辞-1)、Story(接頭辞-2・-3・-4)。
  await projectWithEpic(page, info, prefix);
  for (const title of ["検索", "通知", "設定"]) {
    await storyUnderEpic(page, info, `${prefix}-1`, title);
  }
  const [s2, s3, s4] = [`${prefix}-2`, `${prefix}-3`, `${prefix}-4`];

  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/backlog`));
  const backlog = page.getByRole("region", { name: "バックログ", exact: true });
  await expect(
    backlog.getByText("3 件 · 0 pt · スプリントに入っていない未完了の Story・Task・Bug"),
  ).toBeVisible();
  await expect.poll(() => ids(backlog)).toEqual([s2, s3, s4]);

  // スプリントを作る。空の枠に案内が出る。
  await backlog.getByRole("button", { name: "スプリントを作成" }).click();
  const create = page.getByRole("dialog", { name: "スプリントを作成" });
  await expect(create.getByRole("textbox", { name: "名前" })).toHaveValue("Sprint 1");
  // 既定では期間が入っている。空にして、期間未定で作る。
  await create.getByLabel("開始日").fill("");
  await create.getByLabel("終了日").fill("");
  await create.getByRole("button", { name: "作成", exact: true }).click();
  await expect(create).toBeHidden();
  const sprint = page.getByRole("region", { name: "Sprint 1", exact: true });
  await expect(sprint.getByText("計画中")).toBeVisible();
  await expect(sprint.getByText("期間未定")).toBeVisible();
  await expect(sprint.getByText("0 件 · 0 pt")).toBeVisible();
  await expect(
    sprint.getByText("バックログからここにドラッグ、または行の「スプリント」で選択"),
  ).toBeVisible();

  // -4 を「上へ」: 並びが -2・-4・-3。
  await rowIn(backlog, s4).getByRole("button", { name: "上へ" }).click();
  await expect.poll(() => ids(backlog)).toEqual([s2, s4, s3]);

  // -2 のポイントに 3 を入れて Enter。
  await rowIn(backlog, s2).getByRole("spinbutton", { name: "ポイント" }).fill("3");
  await rowIn(backlog, s2).getByRole("spinbutton", { name: "ポイント" }).press("Enter");
  await expect(
    backlog.getByText("3 件 · 3 pt · スプリントに入っていない未完了の Story・Task・Bug"),
  ).toBeVisible();

  // -3 のスプリントで Sprint 1 を選ぶ。
  await rowIn(backlog, s3).getByRole("combobox", { name: "スプリント" }).selectOption("Sprint 1");
  await expect.poll(() => ids(sprint)).toEqual([s3]);
  await expect.poll(() => ids(backlog)).toEqual([s2, s4]);

  // -2 の持ち手を、Sprint 1 の -3 の行へドラッグ: Sprint 1 の枠が -2・-3。
  await dragHandle(
    page,
    rowIn(backlog, s2).getByRole("button", { name: "ドラッグの持ち手" }),
    rowIn(sprint, s3),
  );
  await expect.poll(() => ids(sprint)).toEqual([s2, s3]);
  await expect.poll(() => ids(backlog)).toEqual([s4]);

  // Sprint 1 の -2 を Done にする。スプリントの枠ではトーストを出さない。
  const gone = page.getByText(`${s2} は完了のため、バックログには表示されません`);
  await rowIn(sprint, s2)
    .getByRole("button", { name: /^ステータス: / })
    .click();
  await page
    .getByRole("menu", { name: "ステータス" })
    .getByRole("menuitemcheckbox", { name: /^Done/ })
    .click();
  await expect(rowIn(sprint, s2).getByRole("button", { name: "ステータス: Done" })).toBeVisible();
  await expect(gone).toHaveCount(0);

  // 続けて -2 のスプリントでバックログを選ぶ。完了のためバックログには出ず、トーストが出る。
  await rowIn(sprint, s2).getByRole("combobox", { name: "スプリント" }).selectOption("バックログ");
  await expect(gone).toBeVisible();
  await expect.poll(() => ids(sprint)).toEqual([s3]);
  await expect(page.locator("[data-row]").filter({ hasText: s2 })).toHaveCount(0);

  // 開始する。
  await sprint.getByRole("button", { name: "開始", exact: true }).click();
  const start = page.getByRole("dialog", { name: "Sprint 1" });
  await start.getByRole("button", { name: "開始", exact: true }).click();
  await expect(start).toBeHidden();
  await expect(sprint.getByText("実行中", { exact: true })).toBeVisible();

  // 「作成」は種類が Story で開き、作ったものはバックログの枠の末尾に入る。
  await backlog.getByRole("button", { name: "作成", exact: true }).click();
  const item = page.getByRole("dialog", { name: "新規作成" });
  await expect(item.getByRole("combobox", { name: "種類" })).toHaveValue("Story");
  const title = unique(info, "追加");
  await item.getByRole("textbox", { name: "タイトル" }).fill(title);
  await item.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(item).toBeHidden();
  await expect.poll(() => ids(backlog)).toEqual([s4, `${prefix}-5`]);
});
