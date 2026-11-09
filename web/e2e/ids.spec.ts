// Screens this spec relies on (see screens.ts for the shared list):
//   - /query/tree?q= is the tree of every Store: a row is a treeitem with its ID; where the prefix is
//     in two Stores, the ID is "<store>:<ID>"
//   - Ctrl+K (⌘K on a Mac) opens the palette: the dialog "ID・検索・コマンド" with the combobox; the
//     input "<store>:<ID>" is an ID, and Enter opens the task of that Store
//   - the detail (/tasks/<ulid>) has the comments as listitems with the author and the body, and the
//     buttons "編集" and "削除" on the ones of the person (the account is e2e = github.com/e2e)
//   - the second user (the peer fixture) is the login "bob"
import { DEFAULT_STORE, expect, Peer, test } from "./harness";

test("an ID whose prefix is in two Stores is written with the Store, and the palette opens it", async ({
  page,
  hh,
}) => {
  // hh store init refuses a prefix a Store has; a Store that comes from a remote can share it
  const other = Peer.publish("second", "DM");
  try {
    hh.hh("store", "add", "second", "--remote", other.remote ?? "");
    hh.hh("sync");
    const a = hh.add("In demo");
    const b = hh.add("In second", "--store", "second");
    expect(a.id).toBe(b.id);

    await hh.open(page, "/query/tree?q=");
    await expect(page.getByRole("treeitem").filter({ hasText: `demo:${a.id}` })).toBeVisible();
    await expect(page.getByRole("treeitem").filter({ hasText: `second:${b.id}` })).toBeVisible();

    await page.keyboard.press("ControlOrMeta+k");
    const palette = page.getByRole("dialog", { name: "ID・検索・コマンド" });
    await palette.getByRole("combobox").fill(`second:${b.id}`);
    await expect(palette.getByRole("option", { name: /In second/ })).toBeVisible();
    await expect(palette.getByRole("option", { name: /In demo/ })).toHaveCount(0);
    await page.keyboard.press("Enter");

    await expect(page).toHaveURL(new RegExp(`/tasks/${b.ulid}$`));
    await expect(page.getByRole("textbox", { name: "タイトル" })).toHaveValue("In second");
    await expect(page.getByText(`Project · second:${b.id}`)).toBeVisible();
  } finally {
    other.stop();
  }
});

test("a Store of its own prefix keeps the plain ID, and a hidden Store is still opened by <store>:<ID>", async ({
  page,
  hh,
}) => {
  hh.hh("store", "init", "second", "--prefix", "SC");
  hh.add("In demo");
  const b = hh.add("In second", "--store", "second");
  hh.hh("store", "set", "second", "--hidden=true");

  await hh.open(page, "/query/tree?q=");
  const row = page.getByRole("treeitem").filter({ hasText: "In demo" });
  await expect(row).toBeVisible();
  await expect(row).not.toContainText(":");

  await page.keyboard.press("ControlOrMeta+k");
  const palette = page.getByRole("dialog", { name: "ID・検索・コマンド" });
  await palette.getByRole("combobox").fill(`second:${b.id}`);
  await expect(palette.getByRole("option", { name: /In second/ })).toBeVisible();
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(new RegExp(`/tasks/${b.ulid}$`));
});

test.describe("comments", () => {
  test.use({ store: { ...DEFAULT_STORE, remote: true } });

  test("edit and delete are offered on the comments of the person only", async ({
    page,
    hh,
    peer,
  }) => {
    const t = hh.add("Talk");
    hh.hh("sync");
    peer.hh("sync");
    hh.hh("comment", "add", t.id, "mine");
    peer.hh("comment", "add", t.id, "theirs");
    peer.hh("sync");
    hh.hh("sync");

    await hh.open(page, `/tasks/${t.ulid}`);
    const mine = page.getByRole("listitem").filter({ hasText: "mine" });
    const theirs = page.getByRole("listitem").filter({ hasText: "theirs" });
    await expect(theirs).toBeVisible();
    await expect(mine.getByRole("button", { name: "編集" })).toBeVisible();
    await expect(mine.getByRole("button", { name: "削除" })).toBeVisible();
    await expect(theirs.getByRole("button", { name: "編集" })).toHaveCount(0);
    await expect(theirs.getByRole("button", { name: "削除" })).toHaveCount(0);
  });
});
