// Screens this spec relies on (see screens.ts for the shared list):
//   - /inbox is the inbox of every store: the heading "通知", the tabs (links in the navigation "通知の種類": すべて, メンション,
//     割り当て, ウォッチ, 上書き, 自動化, 同期の修復; the number is the unread of the tab), and a row for each item (a
//     listitem) with the ID as a link, the sentence, and the buttons: an overwritten value has
//     "自分の値に戻す" and "このままにする", any other unread item "既読にする"
//   - the sidebar ("メインナビゲーション") has the link "通知" with the number of unread items
//   - the second user (the peer fixture) is the login "bob"; the account of the page is "e2e"
import type { Page } from "@playwright/test";
import { DEFAULT_STORE, expect, type Hedgehug, type Peer, test } from "./harness";

test.use({ store: { ...DEFAULT_STORE, remote: true } });

const sidebar = (page: Page) => page.getByRole("navigation", { name: "メインナビゲーション" });
const inboxLink = (page: Page) => sidebar(page).getByRole("link", { name: /通知/ });

/**
 * Makes the value of the title that bob wrote win over the one of e2e, and leaves both synced:
 * e2e writes first, bob writes later (the last writer wins, by the second), then bob pushes, e2e
 * merges and loses, and bob takes in the merge. The task is made by e2e.
 */
async function conflict(hh: Hedgehug, peer: Peer, mine: string, theirs: string) {
  const task = hh.add("base");
  hh.hh("sync");
  peer.hh("sync");
  hh.hh("set", task.id, `title=${mine}`);
  // Commit times are in seconds, and the later write wins (LWW) by that time: bob's write has to
  // fall in a later second than ours, or the two tie. There is no way to move the clock, so wait.
  await new Promise((r) => setTimeout(r, 1100));
  peer.hh("set", task.id, `title=${theirs}`);
  peer.hh("sync");
  hh.hh("sync");
  peer.hh("sync");
  return task;
}

test("an overwritten value is put back from the inbox, and the item is read", async ({
  page,
  hh,
  peer,
}) => {
  const task = await conflict(hh, peer, "mine", "theirs");
  expect(hh.show(task.id).title).toBe("theirs");

  await hh.open(page, "/inbox");
  await expect(inboxLink(page)).toContainText("1", { timeout: 15000 });
  const row = page.getByRole("listitem").filter({ hasText: task.id });
  await expect(row).toContainText(
    "自分が設定した title が、同時に編集された別の値で上書きされました",
  );
  await expect(row).toContainText("title: mine（自分）");

  await row.getByRole("button", { name: "自分の値に戻す" }).click();
  await expect(
    page.getByRole("status").filter({ hasText: `${task.id} の title を自分の値に戻しました` }),
  ).toBeVisible();
  await expect.poll(() => hh.show(task.id).title).toBe("mine");
  // the row is read: it has no buttons any more, and the sidebar has no number
  await expect(row.getByRole("button", { name: "自分の値に戻す" })).toHaveCount(0);
  await expect(row.getByRole("button", { name: "このままにする" })).toHaveCount(0);
  await expect(inboxLink(page)).not.toContainText("1");
});

test("a mention is read by its button, and the number of unread goes down", async ({
  page,
  hh,
  peer,
}) => {
  const task = await conflict(hh, peer, "mine", "theirs");
  peer.hh("comment", "add", task.id, "look at this @e2e");
  peer.hh("sync");
  hh.hh("sync");

  await hh.open(page, "/inbox");
  await expect(inboxLink(page)).toContainText("2", { timeout: 15000 });
  await expect(
    page.getByRole("navigation", { name: "通知の種類" }).getByRole("link", { name: /すべて/ }),
  ).toContainText("2");

  await page.getByRole("link", { name: /メンション/ }).click();
  await expect(page).toHaveURL(/\/inbox\?tab=mention$/);
  const row = page
    .getByRole("listitem")
    .filter({ hasText: "bob がコメントであなたをメンションしました" });
  await expect(row).toBeVisible();
  await row.getByRole("button", { name: "既読にする" }).click();

  await expect(inboxLink(page)).toContainText("1");
  await expect(row.getByRole("button", { name: "既読にする" })).toHaveCount(0);
  // the other item is still unread
  await page.getByRole("link", { name: /上書き/ }).click();
  await expect(page.getByRole("button", { name: "自分の値に戻す" })).toBeVisible();
});

test("the tab is in the address, and the address of a tab opens that tab", async ({ page, hh }) => {
  await hh.open(page, "/inbox?tab=repair");
  const tabs = page.getByRole("navigation", { name: "通知の種類" });
  await expect(tabs.getByRole("link", { name: /同期の修復/ })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(page.getByText("同期の修復の通知はありません")).toBeVisible();
  await tabs.getByRole("link", { name: /すべて/ }).click();
  await expect(page).toHaveURL(/\/inbox$/);
});
