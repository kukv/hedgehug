import { expect, test } from "./fixtures";
import {
  createWorkspace,
  prefixOf,
  projectWithEpic,
  taskUnderEpic,
  tokenUrl,
  unique,
} from "./helpers";

// プロジェクトの別のワークスペース・別の Store への移動と、プロジェクトの旧 ID のアドレスの通しのテスト(A5b)。
// 移動元と移動先のワークスペースはどちらも作る(サンプルの Mobile は transfer.spec.ts が次の番号を MB-2 と決めて使う)。
// 別の Store の移動先は team の Team(transfer.spec.ts は TM-\d+ で受けるので、番号が重なってもかまわない)。

test("moves a project to another workspace and another Store, and follows the old ID", async ({
  page,
}, info) => {
  const t = (s: string) => unique(info, s);
  const prefix = prefixOf(info);
  // 移動先の接頭辞は、移動元の接頭辞の先頭の Z を Y に替えたもの(prefixOf の値は Z で始まるので重ならない)。
  const dest = `Y${prefix.slice(1)}`;
  // ワークスペースの名前は、同じ worker のサーバーで transfer.spec.ts が Store「e2e」に作る「移動元」と重ねない
  // (Store の中で同じ名前のワークスペースは作れない)。
  const from = t("プロジェクトの移動元");
  const to = t("プロジェクトの移動先");
  await createWorkspace(page, info, to, dest);
  await createWorkspace(page, info, from, prefix);
  await projectWithEpic(page, info, prefix);
  await taskUnderEpic(page, info, `${prefix}-1`, "A");
  const title = t(`計画 ${prefix} 1`);

  await test.step("moves the project to the other workspace in the same Store", async () => {
    await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/settings`));
    await page
      .getByRole("combobox", { name: "移動先のワークスペース" })
      .selectOption({ label: `e2e / ${to}` });
    await page.getByRole("button", { name: "移動", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "プロジェクトを移動" });
    await expect(
      dialog.getByText(
        `「${title}」を ${from} から ${to} へ移動します。2 件のアイテムは ${dest} の番号になります。`,
        { exact: true },
      ),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "移動", exact: true }).click();
    await expect(
      page.getByRole("status").getByText(`${to} へ移動しました`, { exact: true }),
    ).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/s/e2e/p/${dest}-P1/settings`));
    await expect(page.getByRole("heading", { name: `${dest}-P1 ${title}` })).toBeVisible();
    // 自分で移したあとの新しいアドレスでは、見出しの下の 1 行は出ない。
    await expect(page.getByText(new RegExp(`^${prefix}-P1 は `))).toHaveCount(0);
  });

  await test.step("moves the project on to the Store team", async () => {
    await page
      .getByRole("combobox", { name: "移動先のワークスペース" })
      .selectOption({ label: "team / Team" });
    await page.getByRole("button", { name: "移動", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "プロジェクトを移動" });
    await expect(
      dialog.getByText(`「${title}」を e2e の ${to} から team の Team へ移動します。`),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "移動", exact: true }).click();
    await expect(
      page.getByRole("status").getByText("team の Team へ移動しました", { exact: true }),
    ).toBeVisible();
    await expect(page).toHaveURL(/\/s\/team\/p\/TM-P\d+\/settings/);
  });

  await test.step("opens the moved project from the old ID on the same tab, with the line only there", async () => {
    await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/tree`));
    const line = page.getByText(
      new RegExp(`^${prefix}-P1 は .+ に ${dest}-P1 へ移動し、その後 TM-P\\d+ へ移動しました$`),
    );
    await expect(line).toBeVisible();
    await expect(page).toHaveURL(/\/s\/team\/p\/TM-P\d+\/tree/);
    await expect(page.getByRole("tab", { name: "ツリー" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expect(
      page.getByRole("heading", { name: new RegExp(`^TM-P\\d+ ${title}$`) }),
    ).toBeVisible();
    await page.getByRole("tab", { name: "概要" }).click();
    await expect(page).toHaveURL(/\/s\/team\/p\/TM-P\d+\/overview/);
    await expect(line).toHaveCount(0);
  });

  await test.step("opens a moved item from its old ID", async () => {
    await page.goto(tokenUrl(`/s/e2e/i/${prefix}-1`));
    const panel = page.getByRole("region", { name: "詳細パネル" });
    await expect(
      panel.getByText(
        new RegExp(`^${prefix}-1 は .+ に ${dest}-1 へ移動し、その後 TM-\\d+ へ移動しました$`),
      ),
    ).toBeVisible();
    await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(t("通知"));
  });
});

test("disables moving a project in a read-only Store with the reason", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/p/HH-P1/settings"));
  const select = page.getByRole("combobox", { name: "移動先のワークスペース" });
  await expect(select).toBeDisabled();
  await expect(select).toHaveAttribute("title", "閲覧のみの Store です");
  const button = page.getByRole("button", { name: "移動", exact: true });
  await expect(button).toBeDisabled();
  await expect(button).toHaveAttribute("title", "閲覧のみの Store です");
});
