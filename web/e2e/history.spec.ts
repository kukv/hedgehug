import { expect, test } from "@playwright/test";
import { newTask } from "./helpers";

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
