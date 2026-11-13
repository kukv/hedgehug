import { expect, test } from "./fixtures";
import { createWorkspace, newTaskUnder, prefixOf, tokenUrl, unique } from "./helpers";

// コメント・ウォッチの通しのテスト(A4a)。ワークスペースは毎回作る(サーバーは 3 つの幅で共有)。

const placeholder = "コメントを書く。@ログイン名 でメンション。Ctrl+Enter で送信";

test("posts, edits and deletes a comment, adds a mention, and toggles watching", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "コメント"), prefix);
  const panel = await newTaskUnder(page, info, prefix);
  const section = panel.getByRole("region", { name: "コメント", exact: true });
  const box = section.getByPlaceholder(placeholder);
  const comments = section.getByRole("article");

  // 書きかけのまま Esc を押すと確認する。「キャンセル」で入力に戻る。
  await box.fill("書きかけ");
  await box.press("Escape");
  const discard = page.getByRole("dialog", { name: "入力を破棄" });
  await expect(discard).toContainText("保存していないコメントを破棄します。");
  await discard.getByRole("button", { name: "キャンセル" }).click();
  await expect(box).toHaveValue("書きかけ");

  // メンションのボタンは、あなた(taro)以外のメンバーだけ。押すと末尾に「@ログイン名 」が入る。
  await box.fill("確認をお願いします ");
  await expect(section.getByRole("button", { name: "@taro" })).toHaveCount(0);
  await section.getByRole("button", { name: "@hanako" }).click();
  await expect(box).toHaveValue("確認をお願いします @hanako ");
  await box.press("Control+Enter");
  await expect(box).toHaveValue("");
  await expect(comments).toHaveCount(1);
  await expect(comments.first()).toContainText("taro");
  await expect(comments.first()).toContainText("確認をお願いします");
  await expect(comments.first().locator('[data-mention="hanako"]')).toBeVisible();
  await expect(comments.first()).not.toContainText("(編集済み)");

  // 「送信」ボタンでも投稿できる。空の本文は投稿しない。
  const send = section.getByRole("button", { name: "送信" });
  await send.click();
  await expect(send).toBeEnabled();
  await expect(box).toHaveValue("");
  await expect(comments).toHaveCount(1);
  await box.fill("二つ目");
  await section.getByRole("button", { name: "送信" }).click();
  await expect(comments).toHaveCount(2);

  // 編集。前と同じなら、何も保存せずに閉じる(「(編集済み)」は付かない)。
  const first = comments.first();
  await first.getByRole("button", { name: "編集" }).click();
  await first.getByRole("button", { name: "保存" }).click();
  await expect(first.getByRole("textbox", { name: "コメントを編集" })).toBeHidden();
  await expect(first).not.toContainText("(編集済み)");
  await first.getByRole("button", { name: "編集" }).click();
  await first.getByRole("textbox", { name: "コメントを編集" }).fill("確認しました");
  await first.getByRole("button", { name: "保存" }).click();
  await expect(first).toContainText("確認しました");
  await expect(first).toContainText("(編集済み)");

  // 削除は確認してから。
  await first.getByRole("button", { name: "削除" }).click();
  const dialog = page.getByRole("dialog", { name: "コメントを削除" });
  await expect(dialog).toContainText("このコメントを削除します。");
  await dialog.getByRole("button", { name: "削除", exact: true }).click();
  await expect(page.getByText("コメントを削除しました")).toBeVisible();
  await expect(comments).toHaveCount(1);
  await expect(comments.first()).toContainText("二つ目");

  // 読み直しても残る。
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  await expect(
    page
      .getByRole("region", { name: "詳細パネル" })
      .getByRole("region", { name: "コメント", exact: true })
      .getByRole("article"),
  ).toHaveCount(1);

  // ウォッチ: 押すたびに「ウォッチ」と「ウォッチ中」が入れ替わり(aria-pressed も)、読み直しても残る。
  const watching = panel.getByRole("button", { name: "ウォッチ中", exact: true });
  const idle = panel.getByRole("button", { name: "ウォッチ", exact: true });
  // 作った人は、最初からウォッチ中。
  await expect(watching).toBeVisible();
  await expect(watching).toHaveAttribute("aria-pressed", "true");
  const [before, after] = [watching, idle];
  await before.click();
  await expect(after).toBeVisible();
  await expect(after).toHaveAttribute("aria-pressed", "false");
  await page.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  await expect(after).toBeVisible();
  await after.click();
  await expect(before).toBeVisible();
});
