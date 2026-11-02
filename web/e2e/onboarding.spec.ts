// Screens this spec relies on (see screens.ts for the shared list):
//   - with no store, the page shows Onboarding: the text "Hedgehug へようこそ" and the
//     button "Store を作る", with no GitHub account to register first
//   - the button opens the dialog "Store を作成" (CreateStore) with the textboxes
//     "名前", "ID の接頭辞" (typed letters become upper case) and "最初のワークスペース（任意）"
//     (empty means the store's name), the line "hh store init <name> --prefix <PREFIX>"
//     at the foot, and the buttons "キャンセル" and "Store を作る"
//   - a prefix that is not an upper-case letter followed by upper-case letters or
//     digits (2 to 10 in all) is refused with a message and nothing is made
//   - after the create the toast says "Store「<name>」を作りました", and the page shows
//     the tree of the new store's first workspace, which the sidebar lists as a link
import { expect, test } from "./harness";
import { row, toast } from "./screens";

test.use({ store: null });

test("a first store is made from Onboarding", async ({ page, hh }) => {
  await hh.open(page);
  await expect(page.getByText("Hedgehug へようこそ")).toBeVisible();

  await page.getByRole("button", { name: "Store を作る" }).click();
  const dialog = page.getByRole("dialog", { name: "Store を作成" });
  await dialog.getByLabel("名前", { exact: true }).fill("team");
  await dialog.getByLabel("ID の接頭辞").fill("tm");
  await expect(dialog.getByLabel("ID の接頭辞")).toHaveValue("TM");
  await expect(
    dialog.getByRole("button", { name: /hh store init team --prefix TM/ }),
  ).toBeVisible();
  await dialog.getByRole("button", { name: "Store を作る" }).click();

  await expect(toast(page, "Store「team」を作りました")).toBeVisible();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("link", { name: /team/ })).toBeVisible();
  expect(hh.hh("store", "list")).toContain("team");

  // The new store takes tasks: the first one gets the prefix.
  await page.getByRole("button", { name: /^タスクを追加/ }).click();
  await page.getByRole("textbox", { name: "新しいタスクのタイトル" }).fill("First task");
  await page.keyboard.press("Enter");
  await expect(row(page, "First task")).toContainText("TM-1");
});

test("a prefix of the wrong shape is refused and nothing is made", async ({ page, hh }) => {
  await hh.open(page);
  await page.getByRole("button", { name: "Store を作る" }).click();
  const dialog = page.getByRole("dialog", { name: "Store を作成" });
  await dialog.getByLabel("名前", { exact: true }).fill("team");
  await dialog.getByLabel("ID の接頭辞").fill("1A");
  await dialog.getByRole("button", { name: "Store を作る" }).click();

  await expect(
    dialog.getByText("接頭辞は英大文字と数字の 2〜10 文字で、英字から始めてください"),
  ).toBeVisible();
  expect(hh.hh("store", "list")).not.toContain("team");
});
