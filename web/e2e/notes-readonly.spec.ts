import { expect, test } from "@playwright/test";
import { tokenUrl } from "./helpers";

// 閲覧のみの Store では、書く操作を無効にして「閲覧のみの Store です」を添える(02「詳細パネル」の冒頭)。
// 閲覧のみの Store のサンプル(HH-2・HH-P1)は読むだけ。サンプルにはコメント・作業時間の記録がないので、
// 既存の行の「編集」「削除」の無効は確かめられない(入力欄と「記録」「送信」の無効だけ確かめる)。

const note = "閲覧のみの Store です";

test("a read-only store's panel disables every write control of the notes", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/i/HH-2"));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("button", { name: "説明を追加" })).toBeDisabled();

  const comments = panel.getByRole("region", { name: "コメント", exact: true });
  await expect(
    comments.getByPlaceholder("コメントを書く。@ログイン名 でメンション。Ctrl+Enter で送信"),
  ).toBeDisabled();
  await expect(comments.getByRole("button", { name: "送信" })).toBeDisabled();
  await expect(comments.getByRole("button", { name: "送信" })).toHaveAttribute("title", note);

  const worklog = panel.getByRole("region", { name: "作業時間", exact: true });
  await expect(worklog.getByLabel("時間", { exact: true })).toBeDisabled();
  await expect(worklog.getByLabel("メモ", { exact: true })).toBeDisabled();
  await expect(worklog.getByRole("button", { name: "記録" })).toBeDisabled();
  await expect(worklog.getByRole("button", { name: "記録" })).toHaveAttribute("title", note);

  const relations = panel.getByRole("region", { name: "関連", exact: true });
  await expect(relations.getByRole("combobox", { name: "関連の表記" })).toBeDisabled();
  await expect(relations.getByRole("button", { name: "追加", exact: true })).toBeDisabled();

  await expect(panel.getByRole("button", { name: /^ウォッチ/ })).toBeDisabled();
});

test("a read-only store's overview disables the property fields and the repositories", async ({
  page,
}) => {
  await page.goto(tokenUrl("/s/readonly/p/HH-P1/overview"));
  const props = page.getByRole("region", { name: "プロパティ", exact: true });
  await expect(props.getByRole("textbox", { name: "タイトル" })).toBeDisabled();
  await expect(props.getByRole("combobox", { name: "ステータス" })).toBeDisabled();
  await expect(props.getByRole("combobox", { name: "オーナー" })).toBeDisabled();
  await expect(props.getByLabel("開始日", { exact: true })).toBeDisabled();
  await expect(props.getByLabel("終了日", { exact: true })).toBeDisabled();
  const add = page
    .getByRole("region", { name: "リポジトリと PR", exact: true })
    .getByRole("button", { name: "リポジトリを追加" });
  await expect(add).toBeDisabled();
  await expect(add).toHaveAttribute("title", note);
  await expect(
    page
      .getByRole("region", { name: "説明", exact: true })
      .getByRole("button", { name: "説明を追加" }),
  ).toBeDisabled();
  await expect(
    page
      .getByRole("region", { name: "コメント", exact: true })
      .getByPlaceholder("コメントを書く。@ログイン名 でメンション。Ctrl+Enter で送信"),
  ).toBeDisabled();
  await expect(
    page
      .getByRole("region", { name: "Epic", exact: true })
      .getByRole("button", { name: /^ステータス:/ })
      .first(),
  ).toBeDisabled();
});

test("a read-only store's panel disables adding and removing links, and copying the mark works", async ({
  page,
}) => {
  await page.goto(tokenUrl("/s/readonly/i/HH-2"));
  const github = page
    .getByRole("region", { name: "詳細パネル" })
    .getByRole("region", { name: "GitHub" });
  for (const name of ["PR をリンク", "Issue をリンク"]) {
    await expect(github.getByRole("button", { name })).toBeDisabled();
    await expect(github.getByRole("button", { name })).toHaveAttribute("title", note);
  }
  await expect(github.getByRole("button", { name: "PR 用の印をコピー" })).toBeEnabled();
});
