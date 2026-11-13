import { expect, test } from "./fixtures";
import { sidebar, tokenUrl, unique } from "./helpers";

test("opens a project's overview and tree from the sidebar", async ({ page }, info) => {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await side.getByRole("region", { name: "e2e" }).getByRole("link", { name: "ログイン" }).click();
  await expect(page.getByRole("heading", { name: "HH-P1 ログイン" })).toBeVisible();
  await expect(page).toHaveURL(/\/s\/e2e\/p\/HH-P1\/overview$/);
  await expect(page.getByRole("region", { name: "Epic" }).getByText("認証")).toBeVisible();
  await page.getByRole("tab", { name: "ツリー" }).click();
  await expect(page).toHaveURL(/\/s\/e2e\/p\/HH-P1\/tree$/);
  const tree = page.getByRole("list", { name: "ツリー" });
  await expect(tree.getByText("HH-2", { exact: true })).toBeVisible();
  await expect(tree.getByText("ログイン画面を作る")).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/overview$/);
});

test("creates a project from the workspace screen and opens it", async ({ page }, info) => {
  const title = unique(info, "決済");
  await page.goto(tokenUrl("/s/e2e/w/HH"));
  await page.getByRole("button", { name: "プロジェクト", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "プロジェクトを作成" });
  await expect(dialog.getByRole("combobox", { name: "オーナー" })).toHaveValue("taro");
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("ログイン");
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(
    dialog.getByText("このワークスペースには同じ名前のプロジェクトがあります"),
  ).toBeVisible();
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(title);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(dialog).toBeHidden();
  await expect(
    page.getByRole("heading", { name: new RegExp(`^HH-P\\d+ ${title}$`) }),
  ).toBeVisible();
  const side = await sidebar(page, info);
  await expect(side.getByRole("link", { name: title })).toBeVisible();
});

test("a read-only store cannot create a project", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/w/HH"));
  await expect(page.getByRole("button", { name: "プロジェクト", exact: true })).toBeDisabled();
  await page.goto(tokenUrl("/s/readonly/p/HH-P1/overview"));
  await expect(page.getByRole("heading", { name: "HH-P1 ログイン" })).toBeVisible();
  await expect(
    page.getByText(
      "readonly は閲覧のみの Store です。閲覧はできますが、作成・変更・削除はできません。",
    ),
  ).toBeVisible();
});
