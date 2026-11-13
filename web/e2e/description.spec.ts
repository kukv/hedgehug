import { expect, test } from "./fixtures";
import { newTask } from "./helpers";

// 詳細パネルの説明(A4a)。サーバーは同じ worker のほかのテストと共有なので、Task は毎回作る(helpers の newTask)。
const body = "## 手順\n- 一つ目\n**太字** と https://example.com/guide を見る";

test("writes a description, shows it as Markdown and asks before discarding a change", async ({
  page,
}, info) => {
  const { panel } = await newTask(page, info, "説明の確認");
  const section = panel.getByRole("region", { name: "説明" });
  const edit = section.getByRole("textbox", { name: "説明" });

  // 空のときは「説明を追加」。押すと入力欄になり、下に案内と 2 つのボタンが出る。
  await section.getByRole("button", { name: "説明を追加" }).click();
  await expect(
    section.getByText("Markdown を使用できます。@ログイン名 でメンション"),
  ).toBeVisible();
  await edit.fill(body);
  await edit.press("Control+Enter");
  await expect(edit).toBeHidden();
  await expect(section.getByRole("heading", { name: "手順" })).toBeVisible();
  await expect(section.getByRole("listitem")).toHaveText(["一つ目"]);
  const link = section.getByRole("link", { name: "https://example.com/guide" });
  await expect(link).toHaveAttribute("target", "_blank");
  await expect(link).toHaveAttribute("rel", "noopener noreferrer");

  // 変えずに保存したときは、何も保存せずに閉じる。
  await section.getByRole("button", { name: "説明を編集" }).click();
  await expect(edit).toHaveValue(body);
  await edit.press("Control+Enter");
  await expect(edit).toBeHidden();

  // 変えて Esc: 確認が出る。「キャンセル」は入力欄に戻る。
  await section.getByRole("button", { name: "説明を編集" }).click();
  await edit.fill(`${body}\n追記`);
  await edit.press("Escape");
  const confirm = page.getByRole("dialog", { name: "入力を破棄" });
  await expect(confirm).toContainText("保存していない説明の変更を破棄します。");
  await confirm.getByRole("button", { name: "キャンセル" }).click();
  await expect(confirm).toBeHidden();
  await expect(edit).toBeFocused();
  await expect(edit).toHaveValue(`${body}\n追記`);

  // パネルを閉じる操作(「×」)でも確認が出る。「キャンセル」ならパネルは開いたまま。
  await panel.getByRole("button", { name: "閉じる" }).click();
  await expect(confirm).toBeVisible();
  await confirm.getByRole("button", { name: "キャンセル" }).click();
  await expect(panel).toBeVisible();

  // 「破棄」は入力欄を閉じ、保存してある説明に戻る(パネルは閉じない)。
  await edit.press("Escape");
  await confirm.getByRole("button", { name: "破棄" }).click();
  await expect(edit).toBeHidden();
  await expect(panel).toBeVisible();
  await expect(section.getByRole("heading", { name: "手順" })).toBeVisible();

  // 「キャンセル」ボタンは確認なしで入力を捨てる。
  await section.getByRole("button", { name: "説明を編集" }).click();
  await edit.fill("捨てる");
  await section.getByRole("button", { name: "キャンセル" }).click();
  await expect(confirm).toBeHidden();
  await expect(edit).toBeHidden();
});
