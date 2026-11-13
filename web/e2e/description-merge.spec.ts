import { expect, test } from "./fixtures";
import { createWorkspace, newTaskUnder, prefixOf, tokenUrl, unique } from "./helpers";

// 説明の Markdown・メンションの表示と、別のタブと同じ行を変えたときの保存の通しのテスト(A4a)。
// 書き方・破棄の確認は description.spec.ts が通す。ワークスペースは毎回作る(サーバーは同じ worker のほかのテストと共有)。

test("renders Markdown with mentions of members only, and keeps both when two tabs change a line", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "説明の Markdown"), prefix);
  const panel = await newTaskUnder(page, info, prefix);
  const section = panel.getByRole("region", { name: "説明", exact: true });
  await section.getByRole("button", { name: "説明を追加" }).click();
  await section
    .getByRole("textbox", { name: "説明" })
    .fill(
      "# 見出し\n- 項目\n**太字** と `code` を @hanako に見てもらう。@nobody はメンバーではない\n```\nコード行\n```",
    );
  await section.getByRole("textbox", { name: "説明" }).press("Control+Enter");
  await expect(section.getByRole("heading", { name: "見出し" })).toBeVisible();
  await expect(section.getByRole("listitem")).toHaveText(["項目"]);
  await expect(section.locator("strong")).toHaveText("太字");
  await expect(section.locator("p code")).toHaveText("code");
  await expect(section.locator("pre")).toContainText("コード行");
  // メンションの印が付くのは、Store のメンバーだけ。
  await expect(section.locator("[data-mention]")).toHaveCount(1);
  await expect(section.locator('[data-mention="hanako"]')).toHaveText("@hanako");

  // 同じ行の保存: 説明を 2 行にしてから、別のタブと同じ行を変えて保存する。
  const edit = section.getByRole("textbox", { name: "説明" });
  await section.getByRole("button", { name: "説明を編集" }).click();
  await edit.fill("1行目\n2行目");
  await edit.press("Control+Enter");
  await expect(edit).toBeHidden();

  // 別のタブで同じアイテムを開き、同じ行を先に変えて保存する。
  const other = await page.context().newPage();
  await other.goto(tokenUrl(`/s/e2e/i/${prefix}-2`));
  const otherSection = other
    .getByRole("region", { name: "詳細パネル" })
    .getByRole("region", { name: "説明", exact: true });
  const otherEdit = otherSection.getByRole("textbox", { name: "説明" });

  // このタブは編集を始める。別のタブが保存したあとで、このタブが保存する。
  await section.getByRole("button", { name: "説明を編集" }).click();
  await edit.fill("1行目\n2行目 (A)");
  await otherSection.getByRole("button", { name: "説明を編集" }).click();
  await otherEdit.fill("1行目\n2行目 (B)");
  await otherEdit.press("Control+Enter");
  await expect(otherEdit).toBeHidden();
  await expect(otherSection).toContainText("2行目 (B)");

  await edit.press("Control+Enter");
  await expect(
    page.getByText("説明の同じ箇所が同時に編集されたため、両方を残しました"),
  ).toBeVisible();
  await expect(section).toContainText("2行目 (A)");
  await expect(section).toContainText("2行目 (B)");
  await other.close();
});
