// Screens this spec relies on (see screens.ts for the shared list):
//   - a Store that cannot be written (Hedgehug.readOnlyStore) has the badge titled "<名前>（閲覧のみ）"
//     (in the sidebar and in the breadcrumb of a task)
//   - the detail of a task of such a Store has the status "閲覧のみ" (the band "この Store は閲覧のみ
//     です（remote に書き込めません）"); the title is a textbox that is readonly, the body a textbox
//     "本文" that is not editable, "ウォッチ" and "削除" are disabled with the sentence as their
//     title, and there is no button "コメント"
//   - the tree of such a Store (/workspaces/<store>/<ulid>/tree) answers Space, Enter, Tab and the
//     like with the toast of the same sentence and writes nothing, and has no add row; its table
//     (/table) has the cell "タイトル: <ID>" as a readonly textbox
import { expect, type Hedgehug, test } from "./harness";
import { row, toast } from "./screens";

/** The path of a tab ("tree", "table") of the first workspace of a Store. */
async function tab(hh: Hedgehug, store: string, name: string): Promise<string> {
  const spaces = await hh.get<{ ulid: string }[]>(`/api/workspaces?store=${store}`);
  const space = spaces[0];
  if (!space) throw new Error(`the store ${store} has no workspace`);
  return `/workspaces/${store}/${space.ulid}/${name}`;
}

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

test("the tree of a read-only Store says why a key does nothing, and writes nothing", async ({
  page,
  hh,
}) => {
  hh.add("Writable");
  hh.readOnlyStore("ro", "RO", ["Read only"]);
  await hh.open(page, await tab(hh, "ro", "tree"));

  // Space waits for the schema of the Store by itself (the add row that treeReady waits for is not
  // shown in a read-only Store).
  await row(page, "Read only").click();
  await page.keyboard.press("Space");
  // The band of the page says the same sentence, so the toast is the one in the live region.
  await expect(toast(page, SENTENCE).and(page.locator("[aria-live]"))).toBeVisible();
  expect(hh.show("RO-1").status).toBe("todo");
  // No add row, and Enter opens no input.
  await expect(page.getByRole("button", { name: /タスクを追加/ })).toHaveCount(0);
  await page.keyboard.press("Enter");
  await expect(page.getByRole("textbox", { name: "新しいタスクのタイトル" })).toHaveCount(0);
});

test("the table of a read-only Store cannot be edited", async ({ page, hh }) => {
  hh.add("Writable");
  hh.readOnlyStore("ro", "RO", ["Read only"]);
  await hh.open(page, await tab(hh, "ro", "table"));

  await expect(page.getByRole("textbox", { name: "タイトル: RO-1" })).toHaveAttribute(
    "readonly",
    "",
  );
});

test("the tree of a Store that can be written still takes Space", async ({ page, hh }) => {
  const t = hh.add("Writable");
  hh.readOnlyStore("ro", "RO");
  await hh.openTree(page);
  await row(page, "Writable").click();
  await page.keyboard.press("Space");
  await expect.poll(() => hh.show(t.id).status).toBe("done");
});
