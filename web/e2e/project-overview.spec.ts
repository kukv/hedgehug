import { expect, test } from "@playwright/test";
import {
  createWorkspace,
  newProject,
  newTaskUnder,
  prefixOf,
  taskUnderEpic,
  tokenUrl,
  unique,
} from "./helpers";

// プロジェクトの概要の編集・説明・コメント・リポジトリ・Epic の行の通しのテスト(A4a)。
// HH-P1 は他のテストが使うので触らず、ワークスペースとプロジェクトを毎回作る。

const overview = (prefix: string) => tokenUrl(`/s/e2e/p/${prefix}-P1/overview`);

test("edits the overview's properties, one save per item, and shows the history", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "概要"), prefix);
  await newProject(page, info, prefix);
  const other = await newProject(page, info, prefix, 2);
  await page.goto(overview(prefix));
  const props = page.getByRole("region", { name: "プロパティ", exact: true });
  const title = props.getByRole("textbox", { name: "タイトル" });

  // Epic が 0 件の文言と注記。
  await expect(
    page
      .getByRole("region", { name: "Epic", exact: true })
      .getByText("Epic はまだありません。Epic の下に Story・Task・Bug を作れます。"),
  ).toBeVisible();
  await expect(
    page.getByText("プロジェクトのプロパティは固定です。ステータスの値も変更できません。"),
  ).toBeVisible();

  // タイトル: 空は断って元に戻す。ほかのプロジェクトと同じ名前も断る。Enter で保存し、見出しも変わる。
  const before = await title.inputValue();
  await title.fill("   ");
  await title.press("Enter");
  await expect(page.getByText("タイトルは空にできません")).toBeVisible();
  await expect(title).toHaveValue(before);
  await title.fill(other);
  await title.press("Enter");
  await expect(
    page.getByText("このワークスペースには同じ名前のプロジェクトがあります"),
  ).toBeVisible();
  await expect(title).toHaveValue(before);
  const renamed = unique(info, "新しい名前");
  await title.fill(renamed);
  await title.press("Enter");
  await expect(page.getByRole("heading", { name: `${prefix}-P1 ${renamed}` })).toBeVisible();

  // ステータスとオーナー。
  await props.getByRole("combobox", { name: "ステータス" }).selectOption("InProgress");
  await expect(props.getByRole("combobox", { name: "ステータス" })).toHaveValue("InProgress");
  await props.getByRole("combobox", { name: "オーナー" }).selectOption("hanako");

  // 日付: 終了日が開始日より前なら断って元に戻す。同じ日は認める。空にすることは常にできる。
  const start = props.getByLabel("開始日", { exact: true });
  const end = props.getByLabel("終了日", { exact: true });
  await start.fill("2026-11-20");
  await start.blur();
  await end.fill("2026-11-19");
  await end.blur();
  await expect(page.getByText("終了日は開始日以降にしてください")).toBeVisible();
  await expect(end).toHaveValue("");
  await end.fill("2026-11-20");
  await end.blur();
  await start.fill("");
  await start.blur();

  // 読み直しても残り、履歴に「作成」と項目の変更が出る。
  await page.goto(overview(prefix));
  await expect(props.getByRole("combobox", { name: "ステータス" })).toHaveValue("InProgress");
  await expect(props.getByRole("combobox", { name: "オーナー" })).toHaveValue("hanako");
  await expect(start).toHaveValue("");
  await expect(end).toHaveValue("2026-11-20");
  const history = page.getByRole("region", { name: "履歴", exact: true });
  await expect(history).toContainText("作成");
  await expect(history).toContainText("ステータスを InProgress に変更");
});

test("edits the description and posts a comment on the overview", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "概要の説明"), prefix);
  await newProject(page, info, prefix);
  const description = page.getByRole("region", { name: "説明", exact: true });
  await description.getByRole("button", { name: "説明を追加" }).click();
  await description.getByRole("textbox", { name: "説明" }).fill("プロジェクトの説明");
  await description.getByRole("textbox", { name: "説明" }).press("Control+Enter");
  await expect(description).toContainText("プロジェクトの説明");
  const comments = page.getByRole("region", { name: "コメント", exact: true });
  const box = comments.getByPlaceholder(
    "コメントを書く。@ログイン名 でメンション。Ctrl+Enter で送信",
  );
  await box.fill("プロジェクトのコメント");
  await box.press("Control+Enter");
  await expect(comments.getByRole("article")).toHaveCount(1);
  // ウォッチはアイテムだけ。概要にはない。
  await expect(page.getByRole("button", { name: /^ウォッチ/ })).toHaveCount(0);
  await page.goto(overview(prefix));
  await expect(description).toContainText("プロジェクトの説明");
  await expect(comments).toContainText("プロジェクトのコメント");
  // 説明の保存は、履歴に「説明を変更」として出る。
  await expect(page.getByRole("region", { name: "履歴", exact: true })).toContainText("説明を変更");
});

test("adds and removes repositories", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "リポジトリ"), prefix);
  await newProject(page, info, prefix);
  const repos = page.getByRole("region", { name: "リポジトリと PR", exact: true });
  const input = repos.getByRole("textbox", { name: "リポジトリ" });
  const list = repos.getByRole("list", { name: "リポジトリ" });
  await expect(repos.getByText("リポジトリはまだありません")).toBeVisible();

  // 形が違うと、入力欄の下に断りが出て、欄は開いたまま。前後の空白は除いて追加する。
  await repos.getByRole("button", { name: "リポジトリを追加" }).click();
  await input.fill("web");
  await input.press("Enter");
  await expect(repos.getByRole("alert")).toContainText("owner/name");
  await expect(input).toBeVisible();
  await input.fill(" example-org/web ");
  await input.press("Enter");
  await expect(list.getByText("example-org/web")).toBeVisible();
  await expect(input).toBeHidden();

  // 大文字と小文字を区別せずに、同じものは断る。「キャンセル」で欄を閉じる。
  await repos.getByRole("button", { name: "リポジトリを追加" }).click();
  await input.fill("Example-Org/WEB");
  await repos.getByRole("button", { name: "追加", exact: true }).click();
  await expect(repos.getByRole("alert")).toBeVisible();
  await expect(list.getByRole("listitem")).toHaveCount(1);
  await repos.getByRole("button", { name: "キャンセル" }).click();
  await expect(input).toBeHidden();

  // 読み直しても残る。「×」は確認なしで外し、トーストを出す。
  await page.goto(overview(prefix));
  await expect(list.getByText("example-org/web")).toBeVisible();
  await repos.getByRole("button", { name: "example-org/web を削除" }).click();
  await expect(page.getByText("example-org/web を削除しました")).toBeVisible();
  await expect(repos.getByText("リポジトリはまだありません")).toBeVisible();
});

test("an Epic row shows its progress and changes status from the row", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "Epic の行"), prefix);
  // Epic(接頭辞-1)の下に Task(接頭辞-2)を 1 件作り、その Task を Done にする。
  const panel = await newTaskUnder(page, info, prefix);
  await panel
    .getByRole("region", { name: "プロパティ" })
    .getByRole("button", { name: "ステータス: Todo" })
    .click();
  await page
    .getByRole("menu", { name: "ステータス" })
    .getByRole("menuitemcheckbox", { name: /^Done/ })
    .click();
  await expect(page.getByRole("button", { name: "ステータス: Done" }).first()).toBeVisible();

  await page.goto(overview(prefix));
  const row = page.getByRole("region", { name: "Epic", exact: true }).locator(".list-row");
  await expect(row).toHaveCount(1);
  await expect(row).toContainText(`${prefix}-1`);
  // 進捗は、Epic の下の Task の完了/対象。Task 1 件が Done なので 1 / 1、100%。
  const bar = row.getByRole("progressbar", { name: "進捗" });
  await expect(bar).toHaveAttribute("title", "1 / 1");
  await expect(row).toContainText("100%");

  // Epic の行のステータスから変える。
  await row.getByRole("button", { name: "ステータス: Todo" }).click();
  await page
    .getByRole("menu", { name: "ステータス" })
    .getByRole("menuitemcheckbox", { name: /^InProgress/ })
    .click();
  await expect(row.getByRole("button", { name: "ステータス: InProgress" })).toBeVisible();
  await page.goto(overview(prefix));
  await expect(row.getByRole("button", { name: "ステータス: InProgress" })).toBeVisible();

  // 2 件目の Task(Todo)を足すと、1 / 2、50% に変わる。
  await taskUnderEpic(page, info, `${prefix}-1`, "もう一つ");
  await page.goto(overview(prefix));
  await expect(bar).toHaveAttribute("title", "1 / 2");
  await expect(row).toContainText("50%");
});
