// Screens this spec relies on (see screens.ts for the shared list):
//   - a Store that cannot be written (Hedgehug.readOnlyStore) has the badge titled "<名前>（閲覧のみ）"
//     (in the sidebar and in the breadcrumb of a task)
//   - the detail of a task of such a Store has the status "閲覧のみ" (the band "この Store は閲覧のみ
//     です（remote に書き込めません）"); the title is a textbox that is readonly, the body a textbox
//     "本文" that is not editable, "ウォッチ" and "削除" are disabled with the sentence as their
//     title, and there is no button "コメント"
import { expect, test } from "./harness";

const SENTENCE = "この Store は閲覧のみです（remote に書き込めません）";

test("a read-only Store is marked, and its task is only read", async ({ page, hh }) => {
  hh.add("Writable");
  hh.readOnlyStore("ro", "RO", ["Read only"]);
  const ro = hh.show("RO-1");

  // The sidebar marks the Store.
  await hh.open(page, "/");
  await expect(
    page.getByRole("navigation", { name: "メインナビゲーション" }).getByTitle("ro（閲覧のみ）"),
  ).toBeVisible();

  // The task is read, not edited.
  await hh.open(page, `/tasks/${ro.ulid}`);
  await expect(page.getByRole("status", { name: "閲覧のみ" })).toHaveText(SENTENCE);
  await expect(page.getByRole("textbox", { name: "タイトル" })).toHaveAttribute("readonly", "");
  await expect(page.getByRole("textbox", { name: "本文" })).toHaveAttribute(
    "contenteditable",
    "false",
  );
  for (const name of ["ウォッチ", "削除"]) {
    const b = page.getByRole("button", { name, exact: true });
    await expect(b).toBeDisabled();
    await expect(b).toHaveAttribute("title", SENTENCE);
  }
  await expect(page.getByRole("button", { name: "コメント", exact: true })).toHaveCount(0);
  await page.getByRole("button", { name: "その他の操作" }).click();
  const move = page.getByRole("menuitem", { name: /別のワークスペースへ移動/ });
  await expect(move).toBeDisabled();
  await expect(move).toContainText(SENTENCE);
});

test("a Store that can be written is edited as before", async ({ page, hh }) => {
  const t = hh.add("Writable");
  hh.readOnlyStore("ro", "RO");
  await hh.open(page, `/tasks/${t.ulid}`);
  await expect(page.getByRole("status", { name: "閲覧のみ" })).toHaveCount(0);
  await expect(page.getByRole("textbox", { name: "タイトル" })).not.toHaveAttribute("readonly", "");
  await expect(page.getByRole("button", { name: "ウォッチ", exact: true })).toBeEnabled();
  await expect(page.getByRole("button", { name: "削除", exact: true })).toBeEnabled();
});
