// Screens this spec relies on (see screens.ts for the shared list):
//   - with no store, the page shows Onboarding: the text "Hedgehug へようこそ", the three steps
//     (アカウント, Store を選ぶ, はじめる), and in the first one the accounts of the config ("e2e",
//     with no token) and the button "次へ: Store を探す"
//   - every step has the button "新しく Store を作成", which opens the dialog "Store を作成"
//     (CreateStore) with the textboxes
//     "名前", "ID の接頭辞" (typed letters become upper case) and "最初のワークスペース（任意）"
//     (empty means the store's name), the line "hh store init <name> --prefix <PREFIX>"
//     at the foot, and the buttons "キャンセル" and "Store を作成"
//   - a prefix that is not an upper-case letter followed by upper-case letters or
//     digits (2 to 10 in all) is refused with a message and nothing is made
//   - a store with no workspace (hh workspace rm) shows "ワークスペースがありません" and the
//     line `hh workspace add ...` on the home
//   - after the create the toast says "Store「<name>」を作成しました", and the page shows
//     the tree of the new store's first workspace, which the sidebar lists as a link
import { expect, test } from "./harness";
import { row, toast } from "./screens";

test.use({ store: null });

test("a first store is made from Onboarding", async ({ page, hh }) => {
  await hh.open(page);
  await expect(page.getByText("Hedgehug へようこそ")).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "GitHub のアカウントを登録します" }),
  ).toBeVisible();
  await expect(page.getByText("e2e@github.com")).toBeVisible();

  // An account with no token still goes on; the folder is the way to look.
  await page.getByRole("button", { name: "次へ: Store を探す" }).click();
  await expect(page.getByRole("heading", { name: "使う Store を選んでください" })).toBeVisible();
  await expect(page.getByText("GitHub で探すにはトークンのあるアカウントが要ります")).toBeVisible();

  await page.getByRole("button", { name: "新しく Store を作成" }).click();
  const dialog = page.getByRole("dialog", { name: "Store を作成" });
  await dialog.getByLabel("名前", { exact: true }).fill("team");
  await dialog.getByLabel("ID の接頭辞").fill("tm");
  await expect(dialog.getByLabel("ID の接頭辞")).toHaveValue("TM");
  await expect(
    dialog.getByRole("button", { name: /hh store init team --prefix TM/ }),
  ).toBeVisible();
  await dialog.getByRole("button", { name: "Store を作成" }).click();

  await expect(toast(page, "Store「team」を作成しました")).toBeVisible();
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
  await page.getByRole("button", { name: "新しく Store を作成" }).click();
  const dialog = page.getByRole("dialog", { name: "Store を作成" });
  await dialog.getByLabel("名前", { exact: true }).fill("team");
  await dialog.getByLabel("ID の接頭辞").fill("1A");
  await dialog.getByRole("button", { name: "Store を作成" }).click();

  await expect(
    dialog.getByText("接頭辞は英大文字と数字の 2〜10 文字で、英字から始めてください"),
  ).toBeVisible();
  expect(hh.hh("store", "list")).not.toContain("team");
});

test.describe("a store with no workspace", () => {
  test.use({ store: { name: "demo", prefix: "DM" } });

  test("the home says so and shows the command to add one", async ({ page, hh }) => {
    hh.hh("workspace", "rm", "demo");
    await hh.open(page);

    await expect(page.getByText("ワークスペースがありません")).toBeVisible();
    await expect(page.getByText(/hh workspace add/)).toBeVisible();
  });
});
