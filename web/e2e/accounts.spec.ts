import { expect, type Page, type TestInfo, test } from "@playwright/test";
import { tokenUrl } from "./helpers";

// アカウントはマシンに 1 つの一覧で、3 つの幅が 1 つのサーバーを共有する。表示名とログイン名には幅とやり直しの
// 回数を入れる。gh CLI のトークンで確かめられるのは gh-{幅}-{回数}-a・-b だけ(D2 画面の計画の決定 S22)。
const tag = (info: TestInfo) => `${info.project.name}-${info.retry}`;

// answer は、e2e のサーバーに、デバイスフローのコードの答えを決めさせる(決定 S21)。
async function answer(page: Page, code: string, result: "grant" | "deny" | "expire", login = "") {
  const q = new URLSearchParams({ code, result, login });
  const res = await page.request.post(`/e2e/device?${q}`, {
    headers: { Authorization: "Bearer e2e-token" },
  });
  expect(res.status()).toBe(204);
}

async function openAdd(page: Page) {
  await page.goto(tokenUrl("/settings/accounts"));
  await page.getByRole("button", { name: "追加", exact: true }).click();
  return page.getByRole("dialog", { name: "アカウントを追加" });
}

const codePattern = /^\d{4}-\d{4}$/;

test("adds, edits and deletes an account that uses the gh CLI token", async ({ page }, info) => {
  const t = tag(info);
  const dialog = await openAdd(page);
  await dialog.getByRole("button", { name: "追加" }).click();
  await expect(dialog.getByText("表示名とログイン名を入力してください")).toBeVisible();
  await dialog.getByRole("textbox", { name: "表示名" }).fill(`gh ${t}`);
  await dialog.getByRole("textbox", { name: "GitHub のログイン名" }).fill(`gh-${t}-a`);
  await dialog.getByRole("combobox", { name: "認証方法" }).selectOption("gh");
  await dialog.getByRole("button", { name: "追加" }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText("アカウントを追加しました")).toBeVisible();
  const row = page.getByRole("listitem", { name: `gh ${t}` });
  await expect(row.getByText(`@gh-${t}-a`)).toBeVisible();
  await expect(row.getByText("github.com · gh CLI のトークン")).toBeVisible();
  await expect(row.getByText("利用可能")).toBeVisible();

  await row.getByRole("button", { name: "編集" }).click();
  const edit = page.getByRole("dialog", { name: "アカウントを編集" });
  await expect(edit.getByRole("textbox", { name: "表示名" })).toHaveValue(`gh ${t}`);
  await edit.getByRole("textbox", { name: "GitHub のログイン名" }).fill(`gh-${t}-b`);
  await edit.getByRole("button", { name: "保存" }).click();
  await expect(edit).toBeHidden();
  await expect(page.getByText("アカウントを変更しました")).toBeVisible();
  await expect(row.getByText(`@gh-${t}-b`)).toBeVisible();

  await row.getByRole("button", { name: "削除" }).click();
  const confirm = page.getByRole("dialog", { name: "アカウントを削除" });
  await expect(confirm.getByText(`「gh ${t}」を削除します。`)).toBeVisible();
  await confirm.getByRole("button", { name: "削除" }).click();
  await expect(page.getByText("アカウントを削除しました")).toBeVisible();
  await expect(row).toHaveCount(0);
});

test("signs in with the device flow, and goes back when another person allows it", async ({
  page,
}, info) => {
  const t = tag(info);
  const dialog = await openAdd(page);
  await dialog.getByRole("textbox", { name: "表示名" }).fill(`dev ${t}`);
  await dialog.getByRole("textbox", { name: "GitHub のログイン名" }).fill(`dev-${t}`);
  await expect(dialog.getByRole("combobox", { name: "認証方法" })).toHaveValue("device");
  await dialog.getByRole("button", { name: "追加" }).click();
  const signIn = page.getByRole("dialog", { name: "GitHub にサインイン" });
  await expect(signIn.getByText("このコードを GitHub で入力してください。")).toBeVisible();
  await expect(signIn.getByRole("link", { name: "https://github.com/login/device" })).toBeVisible();
  await expect(signIn.getByText("許可を待っています")).toBeVisible();

  await answer(
    page,
    (await signIn.getByText(codePattern).textContent()) ?? "",
    "grant",
    `other-${t}`,
  );
  await expect(signIn).toBeHidden();
  await expect(
    dialog.getByText(`このトークンは @other-${t} のものです。ログイン名を直してください`),
  ).toBeVisible();

  await dialog.getByRole("button", { name: "追加" }).click();
  await answer(
    page,
    (await signIn.getByText(codePattern).textContent()) ?? "",
    "grant",
    `dev-${t}`,
  );
  await expect(signIn).toBeHidden();
  await expect(page.getByText(`@dev-${t} としてサインインしました`)).toBeVisible();
  const row = page.getByRole("listitem", { name: `dev ${t}` });
  await expect(row.getByText("github.com · デバイスフローのトークン")).toBeVisible();
});

test("shows an expired or denied code with a retry, and cancelling adds nothing", async ({
  page,
}, info) => {
  const t = tag(info);
  const dialog = await openAdd(page);
  await dialog.getByRole("textbox", { name: "表示名" }).fill(`exp ${t}`);
  await dialog.getByRole("textbox", { name: "GitHub のログイン名" }).fill(`exp-${t}`);
  await dialog.getByRole("button", { name: "追加" }).click();
  const signIn = page.getByRole("dialog", { name: "GitHub にサインイン" });
  const code = signIn.getByText(codePattern);
  const first = (await code.textContent()) ?? "";
  await answer(page, first, "expire");
  await expect(signIn.getByText("コードの期限が切れました")).toBeVisible();
  await signIn.getByRole("button", { name: "再試行" }).click();
  await expect(code).not.toHaveText(first);
  await expect(signIn.getByText("許可を待っています")).toBeVisible();
  await answer(page, (await code.textContent()) ?? "", "deny");
  await expect(signIn.getByText("GitHub で許可が拒否されました")).toBeVisible();
  await signIn.getByRole("button", { name: "キャンセル" }).click();
  await expect(signIn).toBeHidden();
  await expect(page.getByRole("listitem", { name: `exp ${t}` })).toHaveCount(0);
});

test("shows an authentication error in the row, and a failed reauth under it", async ({ page }) => {
  // e2e のサーバーには認証エラーにする経路がないので、一覧と再認証の応答を差し替える(決定 S23)。
  await page.route("**/api/accounts", (route) =>
    route.request().method() === "GET"
      ? route.fulfill({
          json: {
            accounts: [
              {
                id: "AERR",
                name: "broken",
                login: "taro",
                method: "gh",
                status: "auth_error",
                reason: "このトークンは @jiro のものです。ログイン名を直してください",
              },
            ],
          },
        })
      : route.fallback(),
  );
  await page.route("**/api/accounts/AERR/reauth", (route) =>
    route.fulfill({
      status: 422,
      json: {
        detail: "gh CLI のトークンが見つかりません。端末で `gh auth login` を実行してください",
      },
    }),
  );
  await page.goto(tokenUrl("/settings/accounts"));
  const row = page.getByRole("listitem", { name: "broken" });
  await expect(row.getByText("認証エラー")).toBeVisible();
  await expect(
    row.getByText("このトークンは @jiro のものです。ログイン名を直してください"),
  ).toBeVisible();
  await row.getByRole("button", { name: "再認証" }).click();
  await expect(
    row.getByText("gh CLI のトークンが見つかりません。端末で `gh auth login` を実行してください"),
  ).toBeVisible();
});
