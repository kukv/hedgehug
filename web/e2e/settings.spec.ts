import { expect, test } from "./fixtures";
import { saved, send, sidebar, tokenUrl } from "./helpers";

test("the gear in the sidebar opens the general tab of the settings, and a reload keeps them", async ({
  page,
}, info) => {
  await page.goto(tokenUrl("/"));
  const side = await sidebar(page, info);
  await side.getByRole("link", { name: "設定" }).click();
  await expect(page).toHaveURL(/\/settings\/general$/);
  await expect(page.getByRole("heading", { name: "設定" })).toBeVisible();
  await expect(page.getByText("このマシンの設定と、Store の管理")).toBeVisible();
  await expect(page.getByRole("tab", { name: "一般" })).toHaveAttribute("aria-selected", "true");
  await page.reload();
  await expect(page.getByRole("heading", { name: "設定" })).toBeVisible();
});

test("the clock warning shows in the general tab and on the sync card, and goes away when the clock is back", async ({
  page,
}, info) => {
  const skew = (seconds: number) => send(page, "post", "/e2e/clock-skew", { seconds });
  const warning = (dir: string, n: number) =>
    `このマシンの時計が GitHub の時刻より ${n} 分${dir}います。同期で後から届いた変更の勝ち負けと、削除済みを完全に削除する日が、他の人とずれます。OS の日時の設定を確かめてください。`;
  const card = (n: number) => `このマシンの時計が ${n} 分ずれています`;
  try {
    await page.goto(tokenUrl("/settings/accounts"));
    const tabs = page.getByRole("tab");
    await expect(tabs.nth(0)).toHaveText("一般");
    await expect(tabs.nth(1)).toHaveText("アカウントとメンバー");

    await skew(-420);
    await page.getByRole("tab", { name: "一般" }).click();
    await expect(page).toHaveURL(/\/settings\/general$/);
    await expect(page.getByText(warning("進んで", 7))).toBeVisible();
    const side = await sidebar(page, info);
    await expect(side.getByText(card(7))).toBeVisible();

    await skew(360);
    await expect(side.getByText(card(6))).toBeVisible();
    await expect(page.getByText(warning("遅れて", 6))).toBeVisible();

    await skew(60);
    await expect(page.getByText(/GitHub の時刻より/)).toHaveCount(0);
    await expect(side.getByText(/このマシンの時計が/)).toHaveCount(0);
  } finally {
    await skew(0);
  }
});

test("the login asks before あなた changes, and a refusal goes back to the saved login", async ({
  page,
}) => {
  const field = page.getByRole("textbox", { name: "ログイン名" });
  try {
    await page.goto(tokenUrl("/settings/general"));
    await expect(field).toHaveValue("taro");
    await field.fill("tarou");
    await field.press("Enter");
    const dialog = page.getByRole("dialog", { name: "ログイン名を変更" });
    await expect(
      dialog.getByText("local での あなた が @taro から tarou に変わります。"),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "変更" }).click();
    await expect(dialog).toHaveCount(0);
    await expect(field).toHaveValue("tarou");
    await page.reload();
    await expect(field).toHaveValue("tarou");

    await field.fill("ta ro");
    await field.press("Enter");
    await expect(
      page.getByText("英数字・ハイフン・アンダースコアのログイン名(39 文字以内)を入力してください"),
    ).toBeVisible();
    await expect(field).toHaveValue("tarou");
    await expect(page.getByRole("dialog")).toHaveCount(0);
  } finally {
    await send(page, "put", "/api/settings/login", { login: "taro" });
  }
});

test("the sync timing is saved at once, and turning off auto disables the choices", async ({
  page,
}) => {
  const section = page.getByRole("region", { name: "同期のタイミング" });
  const auto = section.getByRole("checkbox", { name: "裏で自動的に同期する" });
  const interval = section.getByRole("combobox", { name: "間隔", exact: true });
  const choices = [
    interval,
    section.getByRole("combobox", { name: "書き込みのあと同期するまで" }),
    section.getByRole("combobox", { name: "GitHub を読む間隔" }),
  ];
  try {
    await page.goto(tokenUrl("/settings/general"));
    await expect(section.getByText("このマシンだけの設定です。")).toBeVisible();
    await expect(section.getByRole("button", { name: "すべて同期" })).toBeVisible();
    // e2e のサーバーは自動同期がオフで始まる。
    await expect(auto).not.toBeChecked();
    for (const c of choices) await expect(c).toBeDisabled();

    const on = saved(page, "/api/sync/timing");
    // チェックボックスは保存のあとの読み直しで印が変わるので、check() ではなく押して待つ。
    await auto.click();
    await on;
    await expect(auto).toBeChecked();
    for (const c of choices) await expect(c).toBeEnabled();
    const fifteen = saved(page, "/api/sync/timing");
    await interval.selectOption({ label: "15 分ごと" });
    await fifteen;
    await page.reload();
    await expect(interval.locator("option:checked")).toHaveText("15 分ごと");

    await auto.click();
    await expect(auto).not.toBeChecked();
    for (const c of choices) await expect(c).toBeDisabled();
  } finally {
    await send(page, "put", "/api/sync/timing", {
      auto: false,
      intervalMinutes: 5,
      afterWriteSeconds: 30,
      githubMinutes: 5,
    });
  }
});

test("the general tab lists the keyboard and the tree keys", async ({ page }) => {
  await page.goto(tokenUrl("/settings/general"));
  const keyboard = page.getByRole("table", { name: "キーボード" });
  await expect(keyboard.getByRole("row")).toHaveCount(6);
  await expect(keyboard.getByRole("row").nth(1)).toContainText("Ctrl+K(Cmd+K)");
  const tree = page.getByRole("table", { name: "ツリー" });
  await expect(tree.getByRole("row")).toHaveCount(13);
});
