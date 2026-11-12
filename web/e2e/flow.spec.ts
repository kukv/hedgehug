import { expect, test } from "@playwright/test";
import { sidebar, tokenUrl, unique } from "./helpers";

// A2 の完成の条件: プロジェクトを作り、Epic・Task・Step を作り、詳細パネルでタイトルを変えられる。
test("creates a project, an Epic, a Task and a Step, and renames the Task", async ({
  page,
}, info) => {
  const project = unique(info, "請求");
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  // 通しのテストが作ったワークスペースが並ぶので、サンプルの「Web」の行に絞る。
  await side
    .getByRole("region", { name: "e2e" })
    .locator(".ws-row")
    .filter({ has: page.getByRole("link", { name: "Web", exact: true }) })
    .getByRole("button", { name: "ワークスペースのメニュー" })
    .click();
  await page.getByRole("menuitem", { name: "プロジェクトを作成" }).click();
  const pd = page.getByRole("dialog", { name: "プロジェクトを作成" });
  await pd.getByRole("textbox", { name: "タイトル" }).fill(project);
  await pd.getByRole("button", { name: "作成" }).click();
  await expect(page.getByRole("heading", { name: new RegExp(`${project}$`) })).toBeVisible();

  await page.getByRole("button", { name: "Epic", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("請求書");
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue("請求書");

  await panel.getByRole("button", { name: "Task" }).click();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("PDF を出す");
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue("PDF を出す");
  await panel.getByRole("textbox", { name: "Step を追加" }).fill("見た目を決める");
  await panel.getByRole("textbox", { name: "Step を追加" }).press("Enter");
  await expect(panel.getByRole("region", { name: "Step" }).getByRole("link")).toHaveText([
    "見た目を決める",
  ]);

  await panel.getByRole("textbox", { name: "タイトル" }).fill("PDF と CSV を出す");
  await panel.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await page.keyboard.press("Escape");
  await page.getByRole("tab", { name: "ツリー" }).click();
  const tree = page.getByRole("list", { name: "ツリー" });
  await expect(tree.getByRole("link")).toHaveText([
    "請求書",
    "PDF と CSV を出す",
    "見た目を決める",
  ]);
  await page.reload();
  await expect(tree.getByRole("link", { name: "PDF と CSV を出す" })).toBeVisible();
});
