import { expect, test } from "./fixtures";
import { apiTask, newTask, receiveMark, renameByApi, tokenUrl, unique } from "./helpers";

test("the history shows the creation and a title change, newest first", async ({ page }, info) => {
  const { name, panel } = await newTask(page, info, "履歴の確認");
  const history = panel.getByRole("region", { name: "履歴" });
  await expect(history.getByRole("listitem")).toHaveCount(1);
  await expect(history.getByRole("listitem").first()).toContainText("作成");

  const title = panel.getByRole("textbox", { name: "タイトル" });
  await title.fill(`${name} 改`);
  await title.press("Enter");
  await expect(history.getByRole("listitem")).toHaveCount(2);
  await expect(history.getByRole("listitem").first()).toContainText(
    `タイトルを「${name} 改」に変更`,
  );
  await expect(history.getByRole("listitem").last()).toContainText("作成");
  await expect(history.getByRole("button", { name: "すべて表示" })).toBeHidden();

  // 説明を保存すると、履歴に「説明を変更」が増える。
  const description = panel.getByRole("region", { name: "説明" });
  await description.getByRole("button", { name: "説明を追加" }).click();
  const edit = description.getByRole("textbox", { name: "説明" });
  await edit.fill("説明です");
  await edit.press("Control+Enter");
  await expect(history.getByRole("listitem").first()).toContainText("説明を変更");
});

// 同期で受け取った保存の行には「同期で受信」が添わる。マークは、そのアイテムの最後の行(いまの保存)に付く。
test("a row saved by sync carries 同期で受信, the creation row does not", async ({
  page,
}, info) => {
  const task = await apiTask(page.request, unique(info, "受信の履歴"));
  await renameByApi(page.request, task.id, unique(info, "受信の履歴 改"));
  await receiveMark(page.request, task.id);

  await page.goto(tokenUrl(`/s/e2e/i/HH-${task.number}`));
  const history = page
    .getByRole("region", { name: "詳細パネル" })
    .getByRole("region", { name: "履歴" });
  await expect(history.getByRole("listitem")).toHaveCount(2);
  await expect(history.getByRole("listitem").first()).toContainText("同期で受信");
  await expect(history.getByRole("listitem").last()).not.toContainText("同期で受信");
});
