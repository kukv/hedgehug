import { expect, type Page, type TestInfo, test } from "@playwright/test";
import { sidebar, tokenUrl, unique } from "./helpers";

// ワークスペースの設定の通しのテスト(B2)。サーバーは 3 つの幅で共有なので、
// 名前と接頭辞は幅とやり直しの回数、乱数で毎回変える。

// 接頭辞は英大文字で始まる 10 文字以内の英大文字と数字。
const prefixOf = (info: TestInfo) =>
  `Z${info.project.name[0]?.toUpperCase()}${info.retry}${Math.floor(Math.random() * 1e5)}`;

// 作成ダイアログから、書き込める Store「e2e」にワークスペースを作る。
async function createWorkspace(page: Page, info: TestInfo, name: string, prefix: string) {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await side
    .getByRole("region", { name: "e2e" })
    .getByRole("button", { name: "ワークスペースを追加" })
    .click();
  const dialog = page.getByRole("dialog", { name: "ワークスペースを追加" });
  await dialog.getByRole("textbox", { name: "名前" }).fill(name);
  await dialog.getByRole("textbox", { name: "接頭辞" }).fill(prefix);
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("heading", { name })).toBeVisible();
}

// afterSave は、path に合う保存の応答のあとに来る、設定の読み直し(ワークスペースと件数の GET)の
// 応答を両方待つ。保存の前に呼んでおく。
function afterSave(page: Page, path: RegExp) {
  let saved = false;
  const reread = (end: RegExp) =>
    page.waitForResponse((r) => {
      const url = new URL(r.url()).pathname;
      if (r.request().method() !== "GET" && path.test(url)) saved = true;
      return saved && r.request().method() === "GET" && end.test(url);
    });
  return Promise.all([reread(/\/workspaces\/[^/]+$/), reread(/\/workspaces\/[^/]+\/usage$/)]);
}

// 作ったワークスペースの設定タブを、アドレスから開く。
async function openSettings(page: Page, prefix: string, subtab = "一般") {
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/settings`));
  await page.getByRole("tab", { name: subtab, exact: true }).click();
}

const subtabs = [
  "一般",
  "プロパティ",
  "ステータス",
  "優先度",
  "関連システム",
  "ワークフロー",
  "テンプレート",
  "自動化",
];

test("creates a workspace from the store menu and the block-end button", async ({ page }, info) => {
  const name = unique(info, "決済基盤");
  const prefix = prefixOf(info);
  await page.goto(tokenUrl());
  let side = await sidebar(page, info);
  const block = side.getByRole("region", { name: "e2e" });
  await block.getByRole("button", { name: "Store のメニュー" }).click();
  await page.getByRole("menuitem", { name: "ワークスペースを追加" }).click();
  const dialog = page.getByRole("dialog", { name: "ワークスペースを追加" });
  await expect(dialog.getByRole("combobox", { name: "Store" })).toHaveValue(/.+/);
  await dialog.getByRole("textbox", { name: "名前" }).fill(name);
  await dialog.getByRole("textbox", { name: "接頭辞" }).fill(prefix.toLowerCase());
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("heading", { name })).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`/s/e2e/w/${prefix}(/overview)?$`));
  side = await sidebar(page, info);
  await expect(side.getByRole("link", { name, exact: true })).toBeVisible();

  // 同じ接頭辞でもう一度作ると、ダイアログの中で断られる。
  await side
    .getByRole("region", { name: "e2e" })
    .getByRole("button", { name: "ワークスペースを追加" })
    .click();
  await dialog.getByRole("textbox", { name: "名前" }).fill(`${name} 2`);
  await dialog.getByRole("textbox", { name: "接頭辞" }).fill(prefix);
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(
    dialog.getByText("この Store には同じ接頭辞のワークスペースがあります"),
  ).toBeVisible();
});

test("the workspace menu opens the settings and moves the workspace down", async ({
  page,
}, info) => {
  const first = unique(info, "移動 A");
  const second = unique(info, "移動 B");
  await createWorkspace(page, info, first, prefixOf(info));
  await createWorkspace(page, info, second, prefixOf(info));
  // 並びの確認は、このテストが作った 2 つの相対の順だけを見る(ほかの幅が同じ Store で動かすため)。
  // 狭い幅の引き出しの開閉は動きがあって当てにならないので、毎回開き直したページで開く。
  const block = async () => {
    await page.goto(tokenUrl());
    return (await sidebar(page, info)).getByRole("region", { name: "e2e" });
  };
  const order = async () =>
    (await block()).locator("a.ws-name").evaluateAll((els) => els.map((e) => e.textContent ?? ""));
  const idx = async (name: string) => (await order()).indexOf(name);
  const menuOf = async (name: string) =>
    (await block())
      .locator(".ws-row")
      .filter({ has: page.getByRole("link", { name, exact: true }) })
      .getByRole("button", { name: "ワークスペースのメニュー" });

  await (await menuOf(first)).click();
  await page.getByRole("menuitem", { name: "設定を開く" }).click();
  await expect(page).toHaveURL(/\/settings$/);
  await expect(page.getByRole("tab", { name: "設定", exact: true })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  for (const label of subtabs) {
    await expect(page.getByRole("tab", { name: label, exact: true })).toBeVisible();
  }

  // first が second より下になるまで押す(隣がほかのワークスペースのこともある)。1 回以上、10 回まで。
  for (let i = 0; i < 10; i++) {
    const prev = (await order()).join("\n");
    await (await menuOf(first)).click();
    const down = page.getByRole("menuitem", { name: "下へ移動" });
    // 末尾なら押せない(ほかの幅が second を先に動かして、すでに first の方が下のとき)。
    if (await down.isDisabled()) {
      await page.keyboard.press("Escape");
      break;
    }
    await down.click();
    await expect.poll(async () => (await order()).join("\n")).not.toBe(prev);
    if ((await idx(first)) > (await idx(second))) break;
  }
  await expect
    .poll(
      async () =>
        (await idx(second)) >= 0 &&
        (await idx(first)) >= 0 &&
        (await idx(first)) > (await idx(second)),
    )
    .toBe(true);
});

test("the general tab restores an empty name and confirms a prefix change", async ({
  page,
}, info) => {
  const name = unique(info, "一般");
  const prefix = prefixOf(info);
  const next = prefixOf(info);
  await createWorkspace(page, info, name, prefix);
  await openSettings(page, prefix);

  const input = page.getByRole("textbox", { name: "名前", exact: true });
  await input.fill("");
  await input.blur();
  await expect(page.getByText("名前は空にできません")).toBeVisible();
  await expect(input).toHaveValue(name);

  const prefixInput = page.getByRole("textbox", { name: "接頭辞", exact: true });
  await expect(page.getByText("アイテムはまだありません")).toBeVisible();
  // 空の接頭辞は、確認ダイアログを開かずに、エラーのトーストで断られる。
  await prefixInput.fill("");
  await page.getByRole("button", { name: "変更", exact: true }).click();
  await expect(page.getByText("接頭辞を入力してください")).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await prefixInput.fill(next.toLowerCase());
  await expect(prefixInput).toHaveValue(next);
  await page.getByRole("button", { name: "変更", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "接頭辞を変更" });
  await expect(dialog).toContainText(
    `接頭辞を ${prefix} から ${next} に変更します。ID が変わるアイテムはありません。`,
  );
  await dialog.getByRole("button", { name: "変更", exact: true }).click();
  await expect(page.getByText("接頭辞を変えました")).toBeVisible();
  await expect(page.getByRole("main").locator(".prefix")).toHaveText(next);
  await expect(page).toHaveURL(new RegExp(`/s/e2e/w/${next}/settings$`));
});

test("the statuses tab adds, recolors, reorders and deletes values", async ({ page }, info) => {
  const name = unique(info, "ステータス");
  const prefix = prefixOf(info);
  await createWorkspace(page, info, name, prefix);
  await openSettings(page, prefix, "ステータス");

  const rowOf = (value: string) =>
    page.locator(".row").filter({ has: page.getByRole("button", { name: `${value}の色` }) });
  await page.getByRole("textbox", { name: "新しい値の名前" }).fill("Review");
  await page.getByRole("button", { name: "追加", exact: true }).click();
  const review = rowOf("Review");
  await expect(review).toBeVisible();

  await review.getByRole("button", { name: "Reviewの色" }).click();
  await page.getByRole("menuitemradio", { name: "紫", exact: true }).click();
  await expect(review.getByRole("button", { name: "Reviewの色" })).toHaveAttribute("title", "紫");

  await review.getByRole("combobox", { name: "Reviewの分類" }).selectOption({ label: "進行中" });
  await expect(review.getByRole("combobox", { name: "Reviewの分類" })).toHaveValue("doing");

  const names = () =>
    page
      .locator('.row input[aria-label="名前"]')
      .evaluateAll((els) => els.map((e) => (e as HTMLInputElement).value));
  expect((await names()).at(-1)).toBe("Review");
  await review.getByRole("button", { name: "上へ" }).click();
  await expect.poll(async () => (await names()).at(-2)).toBe("Review");

  // 作ったばかりのワークスペースの「Todo」は初期ステータスで未着手の既定でもある。ごみ箱を押すと、
  // 確認ダイアログを開かずに、1 つの文のエラーのトーストが出る。
  const dialog = page.getByRole("dialog");
  await rowOf("Todo").getByRole("button", { name: "削除" }).click();
  await expect(
    page.getByText(
      "「Todo」は初期ステータスで、未着手の既定のステータスでもあります。先に別のステータスを、初期ステータスと未着手の既定にしてください",
    ),
  ).toBeVisible();
  await expect(dialog).toHaveCount(0);

  await review.getByRole("button", { name: "初期ステータスにする" }).click();
  await expect(review.getByText("初期", { exact: true })).toBeVisible();

  // 初期でなくなると、未着手の既定だけの文になる。これも、ダイアログを開かない。
  await rowOf("Todo").getByRole("button", { name: "削除" }).click();
  await expect(page.getByText(/「Todo」は未着手の既定のステータスです/)).toBeVisible();
  await expect(dialog).toHaveCount(0);

  // 既定の付け替えが保存されて読み直されるまでは、まだ断られる。保存のあとの読み直し
  // (ワークスペースと件数の 2 つ)の応答を待ってから押す。
  const reloaded = afterSave(page, /\/category-defaults\/todo$/);
  await page.getByRole("combobox", { name: "未着手の既定" }).selectOption({ label: "Pending" });
  await reloaded;
  await rowOf("Todo").getByRole("button", { name: "削除" }).click();
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "削除", exact: true }).click();
  await expect(page.getByText("「Todo」を削除しました")).toBeVisible();
  await expect(rowOf("Todo")).toHaveCount(0);
});

test("the properties tab adds, edits and deletes a property", async ({ page }, info) => {
  const name = unique(info, "プロパティ");
  const prefix = prefixOf(info);
  await createWorkspace(page, info, name, prefix);
  await openSettings(page, prefix, "プロパティ");

  await page.getByRole("button", { name: "プロパティを追加" }).click();
  const add = page.getByRole("dialog", { name: "プロパティを追加" });
  await add.getByRole("textbox", { name: "名前" }).fill("顧客");
  await add.getByRole("textbox", { name: "キー" }).fill("Customer");
  await add.getByRole("combobox", { name: "型" }).selectOption({ label: "単一選択" });
  await add.getByRole("button", { name: "追加", exact: true }).click();
  await expect(
    add.getByText("キーは英小文字で始まる英小文字・数字・_ にしてください"),
  ).toBeVisible();
  await add.getByRole("textbox", { name: "キー" }).fill("customer");
  await add.getByRole("button", { name: "追加", exact: true }).click();
  await expect(page.getByText("プロパティを追加しました")).toBeVisible();

  const choices = page.getByRole("dialog", { name: "「顧客」の選択肢" });
  await choices.getByRole("textbox", { name: "新しい値の名前" }).fill("A社");
  await choices.getByRole("button", { name: "追加", exact: true }).click();
  await expect(choices.locator('input[aria-label="名前"]').first()).toHaveValue("A社");
  await choices.getByRole("button", { name: "閉じる", exact: true }).last().click();
  await expect(choices).toBeHidden();

  const rowOf = (label: string) =>
    page.locator(".row").filter({ has: page.getByRole("button", { name: `${label}の使う種類` }) });
  const customer = rowOf("顧客");
  await expect(customer.getByRole("button", { name: "選択肢 1" })).toBeVisible();

  await customer.getByRole("button", { name: "顧客の使う種類" }).click();
  await page
    .getByRole("group", { name: "使う種類" })
    .getByRole("checkbox", { name: "Epic" })
    .uncheck();
  await page.keyboard.press("Escape");

  const assignee = rowOf("担当者");
  await assignee.getByRole("button", { name: "非表示" }).click();
  await expect(assignee.getByRole("button", { name: "表示", exact: true })).toBeVisible();
  await expect(assignee).toHaveClass(/dim/);

  await customer.getByRole("button", { name: "削除" }).click();
  const del = page.getByRole("dialog", { name: "プロパティを削除" });
  await expect(del).toContainText("「顧客」を削除します。");
  await del.getByRole("button", { name: "削除", exact: true }).click();
  await expect(page.getByText("プロパティを削除しました")).toBeVisible();
  await expect(customer).toHaveCount(0);
});

test("a read-only store's settings are viewable but not editable", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/w/HH/settings"));
  for (const label of subtabs) {
    await expect(page.getByRole("tab", { name: label, exact: true })).toBeEnabled();
  }
  await expect(page.getByRole("textbox", { name: "名前", exact: true })).toBeDisabled();
  await page.getByRole("tab", { name: "優先度", exact: true }).click();
  await expect(page.getByRole("button", { name: "追加", exact: true })).toBeDisabled();
  await expect(page.getByRole("textbox", { name: "新しい値の名前" })).toBeDisabled();
});
