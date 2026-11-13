import type { Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { sidebar, tokenUrl, unique } from "./helpers";

// status は、GET /api/sync の応答を差し替える(決定 S21)。
async function status(page: Page, body: unknown) {
  await page.route("**/api/sync", (route) =>
    route.request().method() === "GET" ? route.fulfill({ json: body }) : route.fallback(),
  );
}

const hoursAgo = (h: number) => new Date(Date.now() - h * 3_600_000).toISOString();

// goOffline は、navigator.onLine を差し替えて offline の出来事を送る(setOffline は localhost も止める)。
async function goOffline(page: Page) {
  await page.evaluate(() => {
    Object.defineProperty(navigator, "onLine", { configurable: true, get: () => false });
    window.dispatchEvent(new Event("offline"));
  });
}

test("the sync card shows the stores and syncs them all", async ({ page }, info) => {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await expect(side.getByText(/最終同期 .+ · Store 2 件/)).toBeVisible();
  const all = side.getByRole("button", { name: "すべて同期" });
  await expect(all).toHaveAttribute("title", /Shift\+(Ctrl|Cmd)\+S/);
  await all.click();
  await expect(
    page.getByRole("status").filter({ hasText: "2 件の Store を同期しました" }),
  ).toBeVisible();
  await expect(side.getByText("すべて最新")).toBeVisible();
});

test("Shift+Ctrl+S syncs while typing in the title", async ({ page }) => {
  await page.goto(tokenUrl("/s/e2e/i/HH-2"));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  const title = panel.getByRole("textbox", { name: "タイトル" });
  await title.fill("入力中のタイトル");
  await page.keyboard.press("Shift+Control+S");
  await expect(
    page.getByRole("status").filter({ hasText: "2 件の Store を同期しました" }),
  ).toBeVisible();
  await expect(title).toHaveValue("入力中のタイトル");
  await expect(title).toBeFocused();
  // サーバーは同じ worker のほかのテストと共有なので、元のタイトルに戻してから離れる(保存されない)。
  await title.fill("ログイン画面を作る");
  await title.blur();
});

// フォーカスを置いただけで打ち換えていないタイトルは、離れたときに古い値で書き戻さず、同期で受け取った値に合わせる。
test("an untouched focused title takes the synced title on blur", async ({
  page,
  context,
}, info) => {
  const task = unique(info, "反映のタイトル");
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/tree"));
  await page.getByRole("list", { name: "ツリー" }).getByRole("link", { name: "認証" }).click();
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await panel.getByRole("button", { name: "Task" }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(task);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  const title = panel.getByRole("textbox", { name: "タイトル" });
  await expect(title).toHaveValue(task);
  const id = await panel.locator("code.id").textContent();

  await title.focus();
  const other = await context.newPage();
  await other.goto(tokenUrl(`/s/e2e/i/${id}`));
  const otherTitle = other
    .getByRole("region", { name: "詳細パネル" })
    .getByRole("textbox", { name: "タイトル" });
  await expect(otherTitle).toHaveValue(task);
  await otherTitle.fill(`${task} 別タブ`);
  await otherTitle.press("Enter");
  await expect(otherTitle).toHaveValue(`${task} 別タブ`);

  await page.bringToFront();
  await expect(title).toBeFocused();
  await page.keyboard.press("Shift+Control+S");
  await expect(
    page.getByRole("status").filter({ hasText: "2 件の Store を同期しました" }),
  ).toBeVisible();
  await expect(title).toHaveValue(task);
  await title.blur();
  await expect(title).toHaveValue(`${task} 別タブ`);
  // サーバーに残った値を、別のページで開き直して確かめる(/i/ のアドレスはツリーへ移るので、reload では開かない)。
  await other.goto(tokenUrl(`/s/e2e/i/${id}`));
  await expect(otherTitle).toHaveValue(`${task} 別タブ`);
});

test("「Store ごとの同期状態」 syncs one store", async ({ page }, info) => {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await side.getByRole("button", { name: /すべて最新|未送信/ }).click();
  const menu = page.getByRole("menu", { name: "Store ごとの同期状態" });
  await menu.getByRole("menuitem", { name: /^readonly/ }).click();
  await expect(
    page.getByRole("status").filter({ hasText: "readonly を同期しました(送信 0 件・受信 0 件)" }),
  ).toBeVisible();
});

test("a sync error shows on the card, the menu and the drawer button", async ({ page }, info) => {
  await status(page, {
    auto: true,
    stores: [
      {
        store: "S1",
        name: "team-a",
        hidden: false,
        readOnly: false,
        error: "認証に失敗しました",
        unsent: 0,
        lastSuccess: hoursAgo(48),
      },
      {
        store: "S2",
        name: "solo",
        hidden: false,
        readOnly: false,
        error: "",
        unsent: 3,
        lastSuccess: hoursAgo(1),
      },
    ],
  });
  await page.goto(tokenUrl());
  if (info.project.name !== "desktop") {
    await expect(page.getByRole("img", { name: "同期エラー" })).toBeVisible();
  }
  const side = await sidebar(page, info);
  await expect(side.getByRole("button", { name: "1 件の Store で同期エラー" })).toBeVisible();
  await expect(side.getByText("1 件の Store が 24 時間以上同期されていません")).toBeVisible();
  await side.getByRole("button", { name: "1 件の Store で同期エラー" }).click();
  await expect(page.getByRole("menu", { name: "Store ごとの同期状態" })).toContainText(
    "同期エラー: 認証に失敗しました · 2 日前",
  );
});

test("a failed sync of all shows the lines in one error toast", async ({ page }, info) => {
  await page.route("**/api/sync", (route) =>
    route.request().method() === "POST"
      ? route.fulfill({
          json: { result: "failed", lines: ["成功 1 件・失敗 1 件", "e2e: 認証に失敗しました"] },
        })
      : route.fallback(),
  );
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await side.getByRole("button", { name: "すべて同期" }).click();
  const toast = page.getByRole("status").filter({ hasText: "成功 1 件・失敗 1 件" });
  await expect(toast).toContainText("e2e: 認証に失敗しました");
  await expect(toast).toHaveClass(/error/);
});

test("offline shows the band, tells the server and refuses a manual sync", async ({
  page,
}, info) => {
  const sent: unknown[] = [];
  await page.route("**/api/sync/network", async (route) => {
    sent.push(route.request().postDataJSON());
    await route.fulfill({ status: 204 });
  });
  await page.goto(tokenUrl());
  await goOffline(page);
  await expect(
    page.getByText(
      "オフラインです。変更はこのマシンにコミットします。つながったら「すべて同期」で push してください。",
    ),
  ).toBeVisible();
  await expect.poll(() => sent.length).toBeGreaterThan(0);
  expect(sent[0]).toMatchObject({ online: false });
  const side = await sidebar(page, info);
  await side.getByRole("button", { name: "すべて同期" }).click();
  await expect(
    page
      .getByRole("status")
      .filter({ hasText: "オフラインのため同期できません。つながったら同期します" }),
  ).toBeVisible();
  await page.evaluate(() => {
    Object.defineProperty(navigator, "onLine", { configurable: true, get: () => true });
    window.dispatchEvent(new Event("online"));
  });
  await expect(page.getByText(/^オフラインです。/)).toBeHidden();
  await expect.poll(() => sent.at(-1)).toMatchObject({ online: true });
});

test("the band follows the automatic sync setting", async ({ page }) => {
  await status(page, {
    auto: true,
    stores: [
      {
        store: "S1",
        name: "team-a",
        hidden: false,
        readOnly: false,
        error: "",
        unsent: 0,
        lastSuccess: "",
      },
    ],
  });
  await page.goto(tokenUrl());
  await goOffline(page);
  await expect(
    page.getByText("オフラインです。変更はこのマシンにコミットし、つながったら push します。"),
  ).toBeVisible();
});

test("a change in another tab shows within a few seconds", async ({ page, context }, info) => {
  await page.goto(tokenUrl("/s/e2e/w/HH"));
  const side = await sidebar(page, info);
  const other = await context.newPage();
  await other.goto(tokenUrl("/s/e2e/w/HH"));
  const name = unique(info, "反映のプロジェクト");
  await other.getByRole("button", { name: "プロジェクト", exact: true }).click();
  const dialog = other.getByRole("dialog", { name: "プロジェクトを作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(name);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(dialog).toBeHidden();
  await expect(side.getByRole("link", { name })).toBeVisible({ timeout: 3000 });
});

test("a held tab tells the server and waits to apply", async ({ page }) => {
  const holds: unknown[] = [];
  await page.route("**/api/sync/hold", async (route) => {
    holds.push(route.request().postDataJSON());
    await route.fulfill({ status: 204 });
  });
  await page.goto(tokenUrl("/s/e2e/i/HH-2"));
  const title = page
    .getByRole("region", { name: "詳細パネル" })
    .getByRole("textbox", { name: "タイトル" });
  await title.focus();
  await expect.poll(() => holds.at(-1)).toMatchObject({ hold: true });
  await title.blur();
  await expect.poll(() => holds.at(-1)).toMatchObject({ hold: false });
});
