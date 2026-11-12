import { expect, test } from "@playwright/test";
import { tokenUrl, unique } from "./helpers";

test("the item entry opens the project's tree with the panel", async ({ page }) => {
  await page.goto(tokenUrl("/s/e2e/i/HH-2"));
  await expect(page).toHaveURL(/\/s\/e2e\/p\/HH-P1\/tree$/);
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue("ログイン画面を作る");
  await expect(panel.getByText("HH-2", { exact: true })).toBeVisible();
  await expect(panel.getByRole("navigation", { name: "パンくず" }).getByText("HH-1")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(panel).toBeHidden();
});

test("renames a Task, adds Steps and moves one up", async ({ page }, info) => {
  const task = unique(info, "設定画面");
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/tree"));
  await page.getByRole("list", { name: "ツリー" }).getByRole("link", { name: "認証" }).click();
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await panel.getByRole("button", { name: "Task" }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(task);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  // 「続けて作成」がオフなら、作ったアイテムの詳細パネルが開く。
  const title = panel.getByRole("textbox", { name: "タイトル" });
  await expect(title).toHaveValue(task);

  await title.fill("   ");
  await title.press("Enter");
  await expect(page.getByText("タイトルは空にできません")).toBeVisible();
  await expect(title).toHaveValue(task);
  await title.fill(`${task}  を直す`);
  await title.press("Enter");
  await expect(title).toHaveValue(`${task} を直す`);
  await expect(
    page.getByRole("list", { name: "ツリー" }).getByRole("link", { name: `${task} を直す` }),
  ).toBeVisible();

  const steps = panel.getByRole("region", { name: "Step" });
  const input = panel.getByRole("textbox", { name: "Step を追加" });
  await input.fill("設計");
  await input.press("Enter");
  await expect(input).toHaveValue("");
  await expect(input).toBeFocused();
  await input.fill("実装");
  await input.press("Enter");
  await expect(steps.getByRole("link")).toHaveText(["設計", "実装"]);
  await steps.getByRole("button", { name: "上へ" }).nth(1).click();
  await expect(steps.getByRole("link")).toHaveText(["実装", "設計"]);
});

test("a Step's checkbox follows the saved status, also when the save is refused", async ({
  page,
}, info) => {
  const step = unique(info, "確認の Step");
  await page.goto(tokenUrl("/s/e2e/i/HH-2"));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  const input = panel.getByRole("textbox", { name: "Step を追加" });
  await input.fill(step);
  await input.press("Enter");
  const box = panel.getByRole("checkbox", { name: step });
  await expect(box).not.toBeChecked();
  // 保存の応答を待ってから次へ進む(保存が重なったり、あとの断りの設定に先の保存が掛かったりしないように)。
  const saved = () => page.waitForResponse((r) => r.url().endsWith("/check") && r.ok());
  await Promise.all([saved(), box.check()]);
  await expect(box).toBeChecked();
  await Promise.all([saved(), box.uncheck()]);
  await expect(box).not.toBeChecked();

  // 断られたら、チェックは保存済みの状態(オフ)に戻る。
  await page.route("**/api/stores/*/items/*/check", (route) =>
    route.fulfill({
      status: 422,
      contentType: "application/problem+json",
      body: JSON.stringify({ title: "Unprocessable Entity", status: 422, detail: "テストの断り" }),
    }),
  );
  await box.click();
  await expect(page.getByText("テストの断り")).toBeVisible();
  await expect(box).not.toBeChecked();

  // 通信が失敗したときも、チェックは保存済みの状態に戻る。
  await page.unroute("**/api/stores/*/items/*/check");
  await page.route("**/api/stores/*/items/*/check", (route) => route.abort());
  await box.click();
  await expect(box).not.toBeChecked();
});

test("copies the ID", async ({ page, context }) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto(tokenUrl("/s/e2e/i/HH-1"));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await panel.getByRole("button", { name: "ID をコピー" }).click();
  await expect(page.getByText("コピーしました: HH-1")).toBeVisible();
});

test("a read-only store's panel shows the band and cannot be edited", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/i/HH-2"));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(
    panel.getByText(
      "readonly は閲覧のみの Store です。閲覧はできますが、作成・変更・削除はできません。",
    ),
  ).toBeVisible();
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toBeDisabled();
  await expect(panel.getByRole("textbox", { name: "Step を追加" })).toBeDisabled();
});
