// Screens this spec relies on:
//   - /settings/stores lists each mounted Store as the article "<名前>": the button "<名前> の色を変える
//     （このマシンだけ）" (its dot is [data-tone]) opens a menu of menuitemcheckbox ("ティール" ...
//     "ブラウン", "自動"); the checkbox "表示"; the text "既定の Store" on the default one; the button
//     "詳細を開く: <名前>" (then "閉じる: <名前>") opens the details with "既定にする" and "マウントを解除…"
//   - the buttons "URL で追加" (dialog "URL で追加": "URL（1行に1つ）", "名前", "追加する") and
//     "Store を作る"
//   - "マウントを解除…" opens the dialog "<名前> のマウントを解除": the checkbox "ローカルのクローンも消す",
//     the buttons "解除する" and (after store.unpushed) "クローンを残して解除"
//   - the toasts are in the status regions: "既定の Store を <名前> にしました", "Store「<名前>」を追加しました",
//     "<名前> のマウントを解除しました"
//   - with no Store left, the page shows Onboarding ("Hedgehug へようこそ")
import { existsSync } from "node:fs";
import type { Locator, Page } from "@playwright/test";
import { DEFAULT_STORE, expect, type Hedgehug, Peer, test } from "./harness";

type StoreJson = {
  name: string;
  path: string;
  color?: string;
  hidden: boolean;
  default_store: boolean;
};

/** The bare repository a published Store was pushed to; a Store without one is a mistake of the test. */
function remoteOf(peer: Peer): string {
  if (!peer.remote) throw new Error("the published Store has no remote");
  return peer.remote;
}

const card = (page: Page, name: string): Locator => page.getByRole("article", { name });
const toast = (page: Page, text: string): Locator =>
  page.getByRole("status").filter({ hasText: text });
const colorButton = (page: Page, name: string): Locator =>
  card(page, name).getByRole("button", { name: `${name} の色を変える（このマシンだけ）` });
const stores = (hh: Hedgehug): StoreJson[] => hh.json<StoreJson[]>("store", "list");
const store = (hh: Hedgehug, name: string): StoreJson | undefined =>
  stores(hh).find((s) => s.name === name);

async function unmount(page: Page, name: string): Promise<Locator> {
  await card(page, name)
    .getByRole("button", { name: `詳細を開く: ${name}` })
    .click();
  await card(page, name).getByRole("button", { name: "マウントを解除…" }).click();
  return page.getByRole("dialog", { name: `${name} のマウントを解除` });
}

test("the colour is saved for this machine, and automatic takes it away", async ({ page, hh }) => {
  hh.hh("store", "init", "second", "--prefix", "SC");
  await hh.open(page, "/settings/stores");
  // the colour of its place first
  await expect(colorButton(page, "second").locator("[data-tone]")).toHaveAttribute(
    "data-tone",
    "orange",
  );

  await colorButton(page, "second").click();
  await page.getByRole("menuitemcheckbox", { name: /スカイ/ }).click();
  await expect(colorButton(page, "second").locator("[data-tone]")).toHaveAttribute(
    "data-tone",
    "sky",
  );
  expect(store(hh, "second")?.color).toBe("sky");

  await colorButton(page, "second").click();
  await expect(page.getByRole("menuitemcheckbox", { name: /スカイ/ })).toHaveAttribute(
    "aria-checked",
    "true",
  );
  await page.getByRole("menuitemcheckbox", { name: /自動/ }).click();
  await expect(colorButton(page, "second").locator("[data-tone]")).toHaveAttribute(
    "data-tone",
    "orange",
  );
  expect(store(hh, "second")?.color ?? "").toBe("");
});

test("hiding a Store keeps the last shown one on", async ({ page, hh }) => {
  hh.hh("store", "init", "second", "--prefix", "SC");
  await hh.open(page, "/settings/stores");

  // the box shows what the server has, so it turns off when the list is read again
  await card(page, "demo").getByRole("checkbox", { name: "表示" }).click();
  await expect(card(page, "demo").getByRole("checkbox", { name: "表示" })).not.toBeChecked();
  expect(store(hh, "demo")?.hidden).toBe(true);
  const last = card(page, "second").getByRole("checkbox", { name: "表示" });
  await expect(last).toBeDisabled();
  await expect(card(page, "second")).toContainText("すべての Store を非表示にはできません");

  await card(page, "demo").getByRole("checkbox", { name: "表示" }).click();
  await expect(card(page, "demo").getByRole("checkbox", { name: "表示" })).toBeChecked();
  expect(store(hh, "demo")?.hidden).toBe(false);
  await expect(last).toBeEnabled();
});

test("the default Store moves to the one chosen", async ({ page, hh }) => {
  hh.hh("store", "init", "second", "--prefix", "SC");
  await hh.open(page, "/settings/stores");
  await expect(card(page, "demo")).toContainText("既定の Store");
  await expect(card(page, "second")).not.toContainText("既定の Store");

  await card(page, "second").getByRole("button", { name: "詳細を開く: second" }).click();
  await card(page, "second").getByRole("button", { name: "既定にする" }).click();

  await expect(toast(page, "既定の Store を second にしました")).toBeVisible();
  await expect(card(page, "second")).toContainText("既定の Store");
  await expect(card(page, "demo")).not.toContainText("既定の Store");
  expect(store(hh, "second")?.default_store).toBe(true);
});

test("a Store is added by the URL of a remote that another home pushed", async ({ page, hh }) => {
  const other = Peer.publish("team", "TM");
  try {
    await hh.open(page, "/settings/stores");
    await page.getByRole("button", { name: "URL で追加" }).click();
    const dialog = page.getByRole("dialog", { name: "URL で追加" });
    await dialog.getByLabel("URL（1行に1つ）").fill(remoteOf(other));
    // the name is proposed from the URL (remote.git), and replaced by the one that is typed
    await expect(dialog.getByLabel("名前")).toHaveValue("remote");
    await dialog.getByLabel("名前").fill("team");
    await expect(dialog.getByRole("button", { name: /hh store add team --remote / })).toBeVisible();
    await dialog.getByRole("button", { name: "追加する" }).click();

    await expect(toast(page, "Store「team」を追加しました")).toBeVisible();
    await expect(dialog).toBeHidden();
    await expect(card(page, "team")).toBeVisible();
    expect(stores(hh).map((s) => s.name)).toContain("team");
  } finally {
    other.stop();
  }
});

test("unmounting deletes the clone, and a Store with unpushed changes asks first", async ({
  page,
  hh,
}) => {
  const other = Peer.publish("team", "TM");
  const another = Peer.publish("kept", "KP");
  try {
    hh.hh("store", "add", "team", "--remote", remoteOf(other));
    hh.hh("store", "add", "kept", "--remote", remoteOf(another));

    await hh.open(page, "/settings/stores");
    const dir = store(hh, "team")?.path ?? "";
    expect(existsSync(dir)).toBe(true);

    const dialog = await unmount(page, "team");
    await expect(dialog.getByRole("checkbox", { name: "ローカルのクローンも消す" })).toBeChecked();
    await expect(dialog.getByRole("button", { name: /hh store remove team$/ })).toBeVisible();
    await dialog.getByRole("button", { name: "解除する" }).click();
    await expect(toast(page, "team のマウントを解除しました")).toBeVisible();
    await expect(card(page, "team")).toHaveCount(0);
    expect(existsSync(dir)).toBe(false);
    expect(stores(hh).map((s) => s.name)).not.toContain("team");

    // a change that was not pushed
    hh.hh("add", "not pushed", "--store", "kept");
    const keptDir = store(hh, "kept")?.path ?? "";
    const again = await unmount(page, "kept");
    await again.getByRole("button", { name: "解除する" }).click();
    await expect(again.getByRole("alert")).toContainText("まだ push していない変更があります");
    await expect(card(page, "kept")).toBeVisible();
    expect(existsSync(keptDir)).toBe(true);

    await again.getByRole("button", { name: "クローンを残して解除" }).click();
    await expect(toast(page, "kept のマウントを解除しました")).toBeVisible();
    await expect(card(page, "kept")).toHaveCount(0);
    expect(existsSync(keptDir)).toBe(true);
    expect(stores(hh).map((s) => s.name)).not.toContain("kept");
  } finally {
    other.stop();
    another.stop();
  }
});

// A Store that was pushed (it has a remote of its own): without one, the first commit is not pushed
// and hh refuses to delete the clone (store.unpushed).
test.describe("the last Store", () => {
  test.use({ store: { ...DEFAULT_STORE, remote: true } });

  test("unmounting it brings Onboarding back", async ({ page, hh }) => {
    await hh.open(page, "/settings/stores");
    const dialog = await unmount(page, "demo");
    await dialog.getByRole("button", { name: "解除する" }).click();
    await expect(page.getByText("Hedgehug へようこそ")).toBeVisible();
    expect(stores(hh)).toEqual([]);
  });
});

test("a Store with no remote starts with its clone kept, since the server never deletes it", async ({
  page,
  hh,
}) => {
  await hh.open(page, "/settings/stores");
  const dir = store(hh, "demo")?.path ?? "";
  const dialog = await unmount(page, "demo");
  const box = dialog.getByRole("checkbox", { name: "ローカルのクローンも消す" });
  await expect(box).not.toBeChecked();
  await expect(dialog.getByRole("button", { name: /hh store remove demo --keep$/ })).toBeVisible();
  await box.check();
  await expect(dialog).toContainText("タスクが失われるため");
  await box.uncheck();
  await dialog.getByRole("button", { name: "解除する" }).click();
  await expect(page.getByText("Hedgehug へようこそ")).toBeVisible();
  expect(existsSync(dir)).toBe(true);
});
