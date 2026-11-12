import { expect, test } from "@playwright/test";
import { addProperty, newTask, tokenUrl, unique } from "./helpers";

// 値を変えるテストは、3 つの幅が 1 つのサーバーを共有するので、自分の Task を作ってから使う。
// 保存の応答は、エンドポイントまで合わせて待つ。別の保存の応答に当たらないようにする。
type Res = { url: () => string; ok: () => boolean; request: () => { method: () => string } };
const itemProps = /\/api\/stores\/[^/]+\/items\/[^/]+\/properties\/[^/]+/;
const isSet = (r: Res) =>
  r.request().method() === "PUT" && new RegExp(`${itemProps.source}$`).test(r.url()) && r.ok();
const isToggle = (r: Res) =>
  ["POST", "DELETE"].includes(r.request().method()) &&
  new RegExp(`${itemProps.source}/values/[^/]+$`).test(r.url()) &&
  r.ok();

test("sets properties in the detail panel", async ({ page }, info) => {
  const { name, panel } = await newTask(page, info, "値");
  const props = panel.getByRole("region", { name: "プロパティ" });
  await props.getByRole("button", { name: "ステータス: Todo" }).click();
  await Promise.all([
    page.waitForResponse(isSet),
    page.getByRole("menuitemcheckbox", { name: /InProgress/ }).click(),
  ]);
  await expect(props.getByRole("button", { name: "ステータス: InProgress" })).toBeVisible();
  await Promise.all([
    page.waitForResponse(isSet),
    props.getByRole("combobox", { name: "担当者" }).selectOption("taro"),
  ]);
  await expect(props.getByRole("combobox", { name: "担当者" })).toHaveValue("taro");
  await props.getByLabel("開始日").fill("2026-11-11");
  await Promise.all([page.waitForResponse(isSet), props.getByLabel("開始日").blur()]);
  await props.getByLabel("終了日").fill("2026-11-10");
  await props.getByLabel("終了日").blur();
  await expect(page.getByText("終了日は開始日以降にしてください")).toBeVisible();
  await expect(props.getByLabel("終了日")).toHaveValue("");
  await props.getByLabel("見積もり時間").fill("0.123");
  await props.getByLabel("見積もり時間").press("Enter");
  await expect(page.getByText("見積もり時間は小数第 2 位までで入力してください")).toBeVisible();
  await props.getByRole("button", { name: /^関連システム:/ }).click();
  await Promise.all([
    page.waitForResponse(isToggle),
    page.getByRole("menuitemcheckbox", { name: "Web" }).click(),
  ]);
  await Promise.all([
    page.waitForResponse(isToggle),
    page.getByRole("menuitemcheckbox", { name: "API" }).click(),
  ]);
  await page.keyboard.press("Escape");
  await expect(props.getByRole("button", { name: "関連システム: Web, API" })).toBeVisible();
  // 保存された値を、読み直して確かめる(画面の状態だけで通らないように)。
  await page.reload();
  await page.getByRole("list", { name: "ツリー" }).getByRole("link", { name, exact: true }).click();
  const saved = page.getByRole("region", { name: "プロパティ" });
  await expect(saved.getByRole("button", { name: "ステータス: InProgress" })).toBeVisible();
  await expect(saved.getByRole("combobox", { name: "担当者" })).toHaveValue("taro");
  await expect(saved.getByLabel("開始日")).toHaveValue("2026-11-11");
  await expect(saved.getByRole("button", { name: "関連システム: Web, API" })).toBeVisible();
});

test("creates a task with properties", async ({ page }, info) => {
  const title = unique(info, "値つき");
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/tree"));
  await expect(page.getByRole("heading", { name: "HH-P1 ログイン" })).toBeVisible();
  await page.keyboard.press("c");
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await expect(dialog.getByRole("combobox", { name: "ステータス" })).toHaveValue(/.+/);
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(title);
  await dialog.getByLabel("見積もり時間").fill("-1");
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(dialog.getByText("見積もり時間は 0 以上の数で入力してください")).toBeVisible();
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toHaveValue(title);
  await dialog.getByLabel("見積もり時間").fill("2");
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(dialog).toBeHidden();
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(title);
  await expect(panel.getByLabel("見積もり時間")).toHaveValue("2");
  await expect(panel.getByRole("button", { name: "ステータス: Todo" })).toBeVisible();
});

test("checks a Step and shows the count", async ({ page }, info) => {
  const { panel } = await newTask(page, info, "Step つき");
  const input = panel.getByRole("textbox", { name: "Step を追加" });
  await input.fill("確認");
  await input.press("Enter");
  const steps = panel.getByRole("region", { name: "Step" });
  // Step の行に group の役割はない。Step は 1 つだけなので、region からチェックとボタンを直接探す。
  const box = steps.getByRole("checkbox", { name: "確認" });
  // チェックはクリックでは切り替わらず、保存して読み直してから変わる。check() は押した直後の切り替わりを求めるので、click() で押す。
  await Promise.all([
    page.waitForResponse((r) => r.url().endsWith("/check") && r.ok()),
    box.click(),
  ]);
  await expect(steps.getByRole("button", { name: "ステータス: Done" })).toBeVisible();
  await expect(steps.getByRole("heading")).toHaveText("Step 1/1");
  await Promise.all([
    page.waitForResponse((r) => r.url().endsWith("/check") && r.ok()),
    box.click(),
  ]);
  await expect(steps.getByRole("button", { name: "ステータス: Todo" })).toBeVisible();
});

test("changes a status from the tree", async ({ page }, info) => {
  const { name } = await newTask(page, info, "ツリー");
  await page.keyboard.press("Escape");
  const tree = page.getByRole("list", { name: "ツリー" });
  const row = tree
    .getByRole("listitem")
    .filter({ has: page.getByRole("link", { name, exact: true }) })
    .last();
  await row.getByRole("button", { name: "ステータス: Todo" }).click();
  await Promise.all([
    page.waitForResponse(isSet),
    page.getByRole("menuitemcheckbox", { name: /Done/ }).click(),
  ]);
  await expect(row.getByRole("button", { name: "ステータス: Done" })).toBeVisible();
});

test("sets an added text property", async ({ page, request }, info) => {
  const stamp = Date.now();
  const name = `${unique(info, "メモ")} ${stamp}`;
  const key = `memo_${info.project.name}_${info.retry}_${stamp}`;
  await addProperty(request, { name, key, type: "text" });
  const { name: task, panel } = await newTask(page, info, "追加の値");
  const props = panel.getByRole("region", { name: "プロパティ" });
  await expect(props.getByText("追加", { exact: true }).first()).toBeVisible();
  const field = props.getByRole("textbox", { name });
  await field.fill("  メモの値  ");
  await Promise.all([page.waitForResponse(isSet), field.blur()]);
  await page.reload();
  // 再読み込みでパネルは閉じる。ツリーから開き直す。
  await page.getByRole("list", { name: "ツリー" }).getByRole("link", { name: task, exact: true }).click();
  await expect(
    page.getByRole("region", { name: "プロパティ" }).getByRole("textbox", { name }),
  ).toHaveValue("メモの値");
});

test("a read-only store disables properties", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/i/HH-2"));
  const props = page.getByRole("region", { name: "プロパティ" });
  await expect(props.getByRole("button", { name: /^ステータス:/ })).toBeDisabled();
  await expect(props.getByRole("combobox", { name: "担当者" })).toBeDisabled();
  const link = props.getByRole("link", { name: "プロパティを追加・変更" });
  await expect(link).toBeVisible();
  await link.click();
  await expect(page).toHaveURL(/\/s\/readonly\/.*settings/);
});
