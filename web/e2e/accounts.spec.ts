// Screens this spec relies on:
//   - /settings/accounts lists each account as the article "<名前>" (heading, "<login>@<host>", the
//     state "有効" / "エラー" / "トークンなし", the row "トークンの取得元" with keychain, Device Flow, gh,
//     環境変数 or なし); its button "<名前> の操作" opens a menu
//   - the button "アカウントを追加" opens the region "アカウントを追加": the radios "Device Flow",
//     "トークンを貼る", "gh", "環境変数", "トークンなし"; the fields "名前", "ホスト", "ログイン名", "トークン";
//     the buttons "GitHub でログイン" (the code appears as .user-code), "トークンを保存", "追加する", "閉じる"
//   - the menu item "ログアウト…" opens the dialog "<名前> からログアウト" with the button "ログアウト"
//   - the toasts are in the status regions: "アカウント <名前> を追加しました" and so on
//
// github: true starts the fake of GitHub as github.com; the Device Flow is approved from the test
// with fakegh, as a person would on github.com. The account "e2e" has no token to begin with.
import type { Locator, Page } from "@playwright/test";
import { expect, type Hedgehug, test } from "./harness";

test.use({ github: true });

type Status = { name: string; token?: string; state: string; error?: string };

const card = (page: Page, name: string): Locator => page.getByRole("article", { name });
const toast = (page: Page, text: string): Locator =>
  page.getByRole("status").filter({ hasText: text });
const addForm = (page: Page): Locator => page.getByRole("region", { name: "アカウントを追加" });

/** hh auth status --json; it exits 1 while an account is in error, and still prints the accounts. */
function authStatus(hh: Hedgehug): Status[] {
  try {
    return hh.json<Status[]>("auth", "status");
  } catch (e) {
    return JSON.parse(String((e as { stdout?: string }).stdout)) as Status[];
  }
}

async function openAdd(page: Page, way: string): Promise<Locator> {
  await page.getByRole("button", { name: "アカウントを追加" }).click();
  const form = addForm(page);
  await form.getByRole("radio", { name: way, exact: true }).check();
  return form;
}

test("an account with no token is added and shows in hh auth status", async ({ page, hh }) => {
  await hh.open(page, "/settings/accounts");
  await expect(card(page, "e2e")).toContainText("トークンなし");

  const form = await openAdd(page, "トークンなし");
  await form.getByLabel(/^名前/).fill("bot");
  await form.getByRole("textbox", { name: "ログイン名", exact: true }).fill("bot");
  await form.getByRole("button", { name: "追加" }).click();

  await expect(toast(page, "アカウント bot を追加しました")).toBeVisible();
  await expect(addForm(page)).toHaveCount(0);
  await expect(card(page, "bot")).toContainText("トークンなし");
  await expect(card(page, "bot")).toContainText("bot@github.com");
  expect(hh.json<Status[]>("auth", "status").map((a) => a.name)).toContain("bot");
});

test("a Device Flow login waits for the approval and then shows the account", async ({
  page,
  hh,
}) => {
  await hh.open(page, "/settings/accounts");
  const form = await openAdd(page, "Device Flow");
  await form.getByLabel(/^名前/).fill("work");
  await form.getByRole("button", { name: "GitHub でログイン" }).click();

  const code = form.locator(".user-code");
  await expect(code).toBeVisible();
  await expect(form.getByText(/承認を待っています · コードの期限まで \d\d:\d\d/)).toBeVisible();
  // approved on GitHub by a login that no account of hh has yet
  hh.fakegh("device", "approve", ((await code.textContent()) ?? "").trim(), "--user", "worker");

  await expect(toast(page, "アカウント work を追加しました")).toBeVisible();
  await expect(addForm(page)).toHaveCount(0);
  await expect(card(page, "work")).toContainText("有効");
  await expect(card(page, "work")).toContainText("Device Flow");
  expect(hh.json<Status[]>("auth", "status")).toContainEqual(
    expect.objectContaining({ name: "work", token: "device", state: "ok" }),
  );
});

test("closing the form stops the Device Flow that was running", async ({ page, hh }) => {
  await hh.open(page, "/settings/accounts");
  const form = await openAdd(page, "Device Flow");
  await form.getByRole("button", { name: "GitHub でログイン" }).click();
  await expect(form.locator(".user-code")).toBeVisible();

  const stopped = page.waitForResponse(
    (r) =>
      r.request().method() === "DELETE" &&
      new URL(r.url()).pathname.startsWith("/api/auth/device/"),
  );
  await form.getByRole("button", { name: "閉じる" }).click();
  await stopped;

  // a flow that was still running would answer 409 and be taken over
  const again = await openAdd(page, "Device Flow");
  await again.getByRole("button", { name: "GitHub でログイン" }).click();
  await expect(again.locator(".user-code")).toBeVisible();
  await expect(again.getByText("別の画面で始めたログインを続けます")).toHaveCount(0);
});

test("a pasted token is kept in the keychain, and log out takes it away", async ({ page, hh }) => {
  hh.fakegh("token", "add", "t9", "--user", "e2e");
  await hh.open(page, "/settings/accounts");

  const form = await openAdd(page, "トークンを貼る");
  await form.getByLabel(/^名前/).fill("e2e");
  await form.getByRole("textbox", { name: "トークン", exact: true }).fill("t9");
  await form.getByRole("button", { name: "トークンを保存" }).click();

  await expect(toast(page, "e2e のトークンを保存しました")).toBeVisible();
  await expect(card(page, "e2e")).toContainText("有効");
  await expect(card(page, "e2e")).toContainText("keychain");
  expect(hh.json<Status[]>("auth", "status")).toContainEqual(
    expect.objectContaining({ name: "e2e", token: "keychain", state: "ok" }),
  );

  await card(page, "e2e").getByRole("button", { name: "e2e の操作" }).click();
  await page.getByRole("menuitem", { name: "ログアウト…" }).click();
  const dialog = page.getByRole("dialog", { name: "e2e からログアウト" });
  await expect(
    dialog.getByRole("button", { name: "コマンドをコピー: hh auth logout e2e" }),
  ).toBeVisible();
  await dialog.getByRole("button", { name: "ログアウト" }).click();

  await expect(toast(page, "e2e からログアウトしました")).toBeVisible();
  // config.yaml keeps token: keychain, so the account has nothing to read: it is in error until it logs in again
  await expect(card(page, "e2e")).toContainText("エラー");
  await expect(card(page, "e2e")).toContainText("the keychain has no token of github.com/e2e");
  const status = authStatus(hh).find((a) => a.name === "e2e");
  expect(status).toMatchObject({ token: "keychain", state: "error" });
  expect(status?.error).toContain("the keychain has no token");
});
