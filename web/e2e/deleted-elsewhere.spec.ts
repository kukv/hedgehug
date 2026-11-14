import type { Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import {
  createWorkspace,
  newProject,
  newTaskUnder,
  prefixOf,
  sidebar,
  tokenUrl,
  unique,
} from "./helpers";

// 別のタブで削除されたものに、このタブから保存したときの通しのテスト(C3a-screen・C3c)。
// このタブは説明・設定の値の名前・コメントを編集しているあいだやダイアログのあいだ(06「反映を待つとき」)なので、
// 別のタブの削除が反映されず、画面が開いたまま保存できる。

const commentPlaceholder = "コメントを書く。@ログイン名 でメンション。Ctrl+Enter で送信";

const restoredToast = (id: string) =>
  `${id} は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました`;

test("saving a description of an item deleted in another tab restores it", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "別で削除 アイテム"), prefix);
  const panel = await newTaskUnder(page, info, prefix);
  const section = panel.getByRole("region", { name: "説明", exact: true });
  await section.getByRole("button", { name: "説明を追加" }).click();
  const edit = section.getByRole("textbox", { name: "説明" });
  await edit.fill("別のタブで削除されたあとの説明");

  const other = await page.context().newPage();
  try {
    // 別のタブで、ツリーの行のメニューからアイテム(接頭辞-2)を削除する。
    await other.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/tree`));
    const tree = other.getByRole("list", { name: "ツリー" });
    await tree
      .getByRole("listitem")
      .filter({ has: other.getByText(`${prefix}-2`, { exact: true }) })
      .getByRole("button", { name: / のメニュー$/ })
      .click();
    await other.getByRole("menuitem", { name: /^削除…/ }).click();
    const dialog = other.getByRole("dialog", { name: "Task を削除" });
    await dialog.getByRole("button", { name: "削除", exact: true }).click();
    await expect(other.getByText(`${prefix}-2 を削除しました`, { exact: true })).toBeVisible();

    // このタブで保存すると、削除が取り消され、トーストが出る。
    await edit.press("Control+Enter");
    await expect(page.getByText(restoredToast(`${prefix}-2`), { exact: true })).toBeVisible();
    await expect(edit).toBeHidden();
    await expect(section).toContainText("別のタブで削除されたあとの説明");

    // 別のタブでアイテムのアドレスを開き直すと、詳細パネルが開く(削除済みではない)。
    await other.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
    await expect(
      other
        .getByRole("region", { name: "詳細パネル" })
        .getByRole("region", { name: "説明", exact: true }),
    ).toContainText("別のタブで削除されたあとの説明");
  } finally {
    await other.close();
  }
});

test("saving a description of a project deleted in another tab restores it", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "別で削除 プロジェクト"), prefix);
  await newProject(page, info, prefix);
  const section = page.getByRole("region", { name: "説明", exact: true });
  await section.getByRole("button", { name: "説明を追加" }).click();
  const edit = section.getByRole("textbox", { name: "説明" });
  await edit.fill("別のタブで削除されたあとの概要");

  const other = await page.context().newPage();
  try {
    // 別のタブで、設定のタブからプロジェクトを削除する。
    await other.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/settings`));
    await other.getByRole("button", { name: "プロジェクトを削除", exact: true }).click();
    await other
      .getByRole("dialog", { name: "プロジェクトを削除" })
      .getByRole("button", { name: "削除", exact: true })
      .click();
    await expect(other.getByText("プロジェクトを削除しました", { exact: true })).toBeVisible();

    await edit.press("Control+Enter");
    await expect(page.getByText(restoredToast(`${prefix}-P1`), { exact: true })).toBeVisible();
    await expect(edit).toBeHidden();
    await expect(section).toContainText("別のタブで削除されたあとの概要");

    // 取り消したあとは、別のタブでプロジェクトの概要が開ける。
    await other.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/overview`));
    await expect(other.getByRole("heading", { name: new RegExp(`^${prefix}-P1 `) })).toBeVisible();
  } finally {
    await other.close();
  }
});

test("renaming a status deleted in another tab restores it", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "別で削除 ステータス"), prefix);
  const rowOf = (p: Page, value: string) =>
    p.locator(".row").filter({ has: p.getByRole("button", { name: `${value}の色` }) });
  const openStatuses = async (p: Page) => {
    await p.goto(tokenUrl(`/s/e2e/w/${prefix}/settings`));
    await p.getByRole("tab", { name: "ステータス", exact: true }).click();
  };
  await openStatuses(page);
  await page.getByRole("textbox", { name: "新しい値の名前" }).fill("Review");
  await page.getByRole("button", { name: "追加", exact: true }).click();
  // 名前の入力欄は、Enter かカーソルが離れたときに保存する。入力だけして、カーソルを置いたままにする。
  const input = rowOf(page, "Review").getByRole("textbox", { name: "名前" });
  await input.fill("Reviewing");

  const other = await page.context().newPage();
  try {
    // 別のタブで、同じステータスを削除する。
    await openStatuses(other);
    await rowOf(other, "Review").getByRole("button", { name: "削除" }).click();
    await other.getByRole("dialog").getByRole("button", { name: "削除", exact: true }).click();
    await expect(other.getByText("「Review」を削除しました", { exact: true })).toBeVisible();
    await expect(rowOf(other, "Review")).toHaveCount(0);

    // このタブで Enter を押すと、削除が取り消され、トーストに削除の前の名前が出る。
    await input.press("Enter");
    await expect(
      page.getByText(
        "「Review」は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました",
        { exact: true },
      ),
    ).toBeVisible();

    // 別のタブで一覧を開き直すと、新しい名前で戻っている。同じアドレスへの goto はハッシュだけの移動になり
    // 読み直さないので、reload で開き直す。
    await other.reload();
    await other.getByRole("tab", { name: "ステータス", exact: true }).click();
    await expect(rowOf(other, "Reviewing")).toBeVisible();
  } finally {
    await other.close();
  }
});

test("editing a comment deleted in another tab restores it", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "別で削除 コメント"), prefix);
  const panel = await newTaskUnder(page, info, prefix);
  const section = panel.getByRole("region", { name: "コメント", exact: true });
  await section.getByPlaceholder(commentPlaceholder).fill("削除される前の本文");
  await section.getByRole("button", { name: "送信" }).click();
  const comment = section.getByRole("article");
  await expect(comment).toHaveCount(1);
  await comment.getByRole("button", { name: "編集" }).click();
  const edit = comment.getByRole("textbox", { name: "コメントを編集" });
  await edit.fill("別のタブで削除されたあとの本文");

  const other = await page.context().newPage();
  try {
    // 別のタブで、同じアイテムを開いてコメントを削除する。
    await other.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
    const others = other
      .getByRole("region", { name: "詳細パネル" })
      .getByRole("region", { name: "コメント", exact: true })
      .getByRole("article");
    await others.first().getByRole("button", { name: "削除" }).click();
    await other
      .getByRole("dialog", { name: "コメントを削除" })
      .getByRole("button", { name: "削除", exact: true })
      .click();
    await expect(other.getByText("コメントを削除しました", { exact: true })).toBeVisible();
    await expect(others).toHaveCount(0);

    // このタブで保存すると、削除が取り消され、トーストが出て、新しい本文が表示される。
    await comment.getByRole("button", { name: "保存" }).click();
    await expect(
      page.getByText(
        "コメントは別の画面で削除されていましたが、変更を保存したため、削除を取り消しました",
        { exact: true },
      ),
    ).toBeVisible();
    await expect(edit).toBeHidden();
    await expect(comment).toContainText("別のタブで削除されたあとの本文");

    // 別のタブでアイテムのアドレスを開き直すと、コメントがある。
    await other.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
    await expect(others).toHaveCount(1);
    await expect(others.first()).toContainText("別のタブで削除されたあとの本文");
  } finally {
    await other.close();
  }
});

test("changing a setting of a workspace deleted in another tab restores it", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  const name = unique(info, "別で削除 ワークスペース");
  const renamed = unique(info, "別で削除 戻した");
  await createWorkspace(page, info, name, prefix);
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/settings`));
  // 名前の入力欄は、Enter かカーソルが離れたときに保存する。入力だけして、カーソルを置いたままにする。
  const input = page.getByRole("textbox", { name: "名前", exact: true });
  await expect(input).toHaveValue(name);
  await input.fill(renamed);

  const other = await page.context().newPage();
  try {
    // 別のタブで、設定の「一般」から同じワークスペースを削除する(B5b)。
    await other.goto(tokenUrl(`/s/e2e/w/${prefix}/settings`));
    await other.getByRole("button", { name: "ワークスペースを削除…", exact: true }).click();
    const dialog = other.getByRole("dialog", { name: "ワークスペースを削除" });
    await dialog
      .getByRole("textbox", { name: "確認のため、ワークスペースの名前を入力してください" })
      .fill(name);
    await dialog.getByRole("button", { name: "削除", exact: true }).click();
    await expect(other.getByText(`「${name}」を削除しました`)).toBeVisible();

    // このタブで Enter を押すと、削除が取り消され、トーストに削除の前の名前が出る。
    await input.press("Enter");
    await expect(
      page.getByText(
        `「${name}」は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました`,
        { exact: true },
      ),
    ).toBeVisible();

    // 別のタブで開き直すと、サイドバーに新しい名前のワークスペースが出る。
    await other.goto(tokenUrl());
    const side = await sidebar(other, info);
    await expect(side.getByRole("link", { name: renamed, exact: true })).toBeVisible();
  } finally {
    await other.close();
  }
});

test("editing a sprint deleted in another tab restores it", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "別で削除 スプリント"), prefix);
  const sprintsTab = tokenUrl(`/s/e2e/w/${prefix}/sprints`);
  const card = (p: Page) => p.getByRole("article", { name: "Sprint 1" });
  await page.goto(sprintsTab);
  // 既定の名前(Sprint 1)のまま作る。
  await page.getByRole("button", { name: "スプリントを作成" }).first().click();
  const create = page.getByRole("dialog", { name: "スプリントを作成" });
  await expect(create.getByRole("textbox", { name: "名前" })).toHaveValue("Sprint 1");
  await create.getByRole("button", { name: "作成", exact: true }).click();
  await expect(page.getByText("Sprint 1 を作成しました")).toBeVisible();
  // 編集のダイアログでゴールを入力し、保存はまだしない(ダイアログのあいだ反映を待つ)。
  await card(page).getByRole("button", { name: "Sprint 1 のメニュー" }).click();
  await page.getByRole("menuitem", { name: "編集…" }).click();
  const edit = page.getByRole("dialog", { name: "スプリントを編集" });
  await edit.getByRole("textbox", { name: "ゴール" }).fill("別のタブで削除されたあとのゴール");

  const other = await page.context().newPage();
  try {
    // 別のタブで、カードのメニューから同じスプリントを削除する。
    await other.goto(sprintsTab);
    await card(other).getByRole("button", { name: "Sprint 1 のメニュー" }).click();
    await other.getByRole("menuitem", { name: "削除…" }).click();
    await other
      .getByRole("dialog", { name: "スプリントを削除" })
      .getByRole("button", { name: "削除", exact: true })
      .click();
    await expect(other.getByText("スプリントを削除しました", { exact: true })).toBeVisible();
    await expect(card(other)).toHaveCount(0);

    // このタブで保存すると、削除が取り消され、トーストにスプリントの名前が出る。
    await edit.getByRole("button", { name: "保存", exact: true }).click();
    await expect(
      page.getByText(
        "「Sprint 1」は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました",
        { exact: true },
      ),
    ).toBeVisible();
    await expect(edit).toBeHidden();

    // 別のタブで開き直すと、スプリントのカードが新しいゴールで戻っている。
    await other.reload();
    await expect(card(other)).toContainText("ゴール: 別のタブで削除されたあとのゴール");
  } finally {
    await other.close();
  }
});
