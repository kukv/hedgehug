import { expect, test } from "./fixtures";
import { tokenUrl } from "./helpers";

// 受信箱が空のときの表示の通しのテスト(C4a)。同じ worker のほかのテスト(説明の重なりの e2e など)が作った
// 上書きの通知が残らないよう、このファイルのテストは別のサーバーで動かす。
test.use({ ownServer: true });

test("the overwrite tab shows the empty message and its note", async ({ page }) => {
  await page.goto(tokenUrl("/inbox"));
  await page.getByRole("tab", { name: /^上書き/ }).click();
  await expect(page.getByText("お知らせはありません")).toBeVisible();
  await expect(
    page.getByText(
      "このマシンで変えた値が、同期であとから届いた変更に負けると、ここに出ます。試すには、値を変えてからサイドバーの同期ボタンを押してください。",
    ),
  ).toBeVisible();
});
