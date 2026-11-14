import type { Page, TestInfo } from "@playwright/test";
import { expect } from "./fixtures";
import {
  createWorkspace,
  prefixOf,
  projectWithEpic,
  saved,
  storyUnderEpic,
  tokenUrl,
  unique,
} from "./helpers";

// レポートの通しのテスト(F3)の準備。spec どうしで import するとテストが重複して登録されるので、spec の外に置く。

// setPoints は、アイテムの詳細パネルでポイントを入れる。
async function setPoints(page: Page, id: string, points: string) {
  await page.goto(tokenUrl(`/s/e2e/i/${id}`));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  const field = panel.getByRole("spinbutton", { name: "ポイント" });
  const done = saved(page, "/properties");
  await field.fill(points);
  await field.press("Enter");
  await done;
}

// moveStatus は、アイテムの詳細パネルでステータスを変える。
async function moveStatus(page: Page, id: string, from: string, to: string) {
  await page.goto(tokenUrl(`/s/e2e/i/${id}`));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await panel.getByRole("button", { name: `ステータス: ${from}` }).click();
  await page
    .getByRole("menu", { name: "ステータス" })
    .getByRole("menuitemcheckbox", { name: new RegExp(`^${to}`) })
    .click();
  await expect(panel.getByRole("button", { name: `ステータス: ${to}` })).toBeVisible();
}

// startedSprint は、ワークスペース・プロジェクト・Epic(接頭辞-1)と Story A(接頭辞-2、3 pt)・Story B(接頭辞-3、2 pt)を
// 作り、Sprint 1 に入れて開始し、A を InProgress → Done にする。接頭辞を返す。Task 14・15 の e2e も使う。
export async function startedSprint(page: Page, info: TestInfo) {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "レポート"), prefix);
  await projectWithEpic(page, info, prefix);
  await storyUnderEpic(page, info, `${prefix}-1`, "A");
  await storyUnderEpic(page, info, `${prefix}-1`, "B");
  await setPoints(page, `${prefix}-2`, "3");
  await setPoints(page, `${prefix}-3`, "2");

  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/sprints`));
  await page.getByRole("button", { name: "スプリントを作成" }).first().click();
  const create = page.getByRole("dialog", { name: "スプリントを作成" });
  await create.getByRole("button", { name: "作成", exact: true }).click();
  await expect(create).toBeHidden();
  for (const id of [`${prefix}-2`, `${prefix}-3`]) {
    await page.goto(tokenUrl(`/s/e2e/i/${id}`));
    const assigned = saved(page, "/sprint");
    await page
      .getByRole("region", { name: "詳細パネル" })
      .getByRole("combobox", { name: "スプリント" })
      .selectOption({ label: "Sprint 1" });
    await assigned;
  }

  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/sprints`));
  const s1 = page.getByRole("article", { name: "Sprint 1" });
  await s1.getByRole("button", { name: "開始", exact: true }).click();
  const start = page.getByRole("dialog", { name: "Sprint 1" });
  await start.getByRole("button", { name: "開始", exact: true }).click();
  await expect(start).toBeHidden();

  await moveStatus(page, `${prefix}-2`, "Todo", "InProgress");
  await moveStatus(page, `${prefix}-2`, "InProgress", "Done");
  return prefix;
}
