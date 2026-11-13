import { expect, test } from "@playwright/test";
import { createWorkspace, openSettings, prefixOf, projectWithEpic, saved, unique } from "./helpers";

// 作成ダイアログの説明の欄と、テンプレートの説明の当てはめの通しのテスト(A4a)。
// 既定のテンプレートの説明が最初から入ることは create-template.spec.ts が通す。

test("the template picker fills and clears the description, and create another empties it", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "作成の説明"), prefix);
  // テンプレート「説明つき」: 種類 Task、説明「テンプレートの説明」。既定にはしない。
  await openSettings(page, prefix, "テンプレート");
  await page.getByRole("button", { name: "テンプレートを作成" }).click();
  const template = page.getByRole("dialog", { name: "テンプレートを作成" });
  await template.getByRole("textbox", { name: "名前" }).fill("説明つき");
  await template.getByRole("combobox", { name: "種類" }).selectOption({ label: "Task" });
  await template.getByRole("textbox", { name: "説明" }).fill("テンプレートの説明");
  await Promise.all([
    saved(page, "/templates"),
    template.getByRole("button", { name: "保存", exact: true }).click(),
  ]);
  await expect(template).toBeHidden();

  const panel = await projectWithEpic(page, info, prefix);
  await page.keyboard.press("Escape");
  await expect(panel).toBeHidden();

  await page.keyboard.press("c");
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  const description = dialog.getByRole("textbox", { name: "説明" });
  const picker = dialog.getByRole("combobox", { name: "テンプレート" });
  await dialog.getByRole("combobox", { name: "種類" }).selectOption({ label: "Task" });

  // 選ぶとテンプレートの説明が入り、「なし」に戻すと(入力していなければ)空に戻る。
  await picker.selectOption({ label: "説明つき" });
  await expect(description).toHaveValue("テンプレートの説明");
  await picker.selectOption({ label: "なし" });
  await expect(description).toHaveValue("");
  await picker.selectOption({ label: "説明つき" });
  await expect(description).toHaveValue("テンプレートの説明");

  // 利用者が入力した説明は、「なし」に戻しても残る。
  await description.fill("# 手順\n自分の説明");
  await picker.selectOption({ label: "なし" });
  await expect(description).toHaveValue("# 手順\n自分の説明");

  // 説明つきで作ると、詳細パネルに Markdown で出る。
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("説明つきの Task");
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(page.getByText(`${prefix}-2 を作成しました`)).toBeVisible();
  await expect(
    page
      .getByRole("region", { name: "詳細パネル" })
      .getByRole("region", { name: "説明", exact: true })
      .getByRole("heading", { name: "手順" }),
  ).toBeVisible();

  // 「続けて作成」: 作ったあと、タイトルと説明だけが空になり、テンプレートは残る。
  await page.keyboard.press("Escape");
  await page.keyboard.press("c");
  await dialog.getByRole("combobox", { name: "種類" }).selectOption({ label: "Task" });
  await dialog.getByRole("checkbox", { name: "続けて作成" }).check();
  await picker.selectOption({ label: "説明つき" });
  await expect(description).toHaveValue("テンプレートの説明");
  await dialog.getByRole("textbox", { name: "タイトル" }).fill("続けて作る Task");
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(page.getByText(`${prefix}-3 を作成しました`)).toBeVisible();
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("textbox", { name: "タイトル" })).toHaveValue("");
  await expect(description).toHaveValue("");
  await expect(picker.locator("option:checked")).toHaveText("説明つき");
});
