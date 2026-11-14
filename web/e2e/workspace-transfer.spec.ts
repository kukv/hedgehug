import { expect, test } from "./fixtures";
import {
  createWorkspace,
  prefixOf,
  projectWithEpic,
  sidebar,
  taskUnderEpic,
  tokenUrl,
  unique,
} from "./helpers";

// ワークスペースの別の Store への移動と、ワークスペースの古いアドレスの通しのテスト(B5c)。
// 移すワークスペースは毎回作る(サンプルの「Web」は、ほかのテストが使うので移さない)。移動先は team。

const nameLabel = "確認のため、ワークスペースの名前を入力してください";

test("moves a workspace to the Store team, and follows its old addresses", async ({
  page,
}, info) => {
  const prefix = prefixOf(info);
  const name = unique(info, "Store を移るワークスペース");
  await createWorkspace(page, info, name, prefix);
  await projectWithEpic(page, info, prefix);
  await taskUnderEpic(page, info, `${prefix}-1`, "A");

  await test.step("confirms with the workspace name and moves it", async () => {
    await page.goto(tokenUrl(`/s/e2e/w/${prefix}/settings`));
    await page.getByRole("combobox", { name: "移動先の Store" }).selectOption({ label: "team" });
    await page.getByRole("button", { name: "移動…", exact: true }).click();
    const dialog = page.getByRole("dialog", { name: "別の Store へ移動" });
    await expect(dialog.getByText(`「${name}」を e2e から team へ移動します。`)).toBeVisible();
    await expect(
      dialog.getByText("プロジェクト 1 件・アイテム 2 件を移動します。", { exact: true }),
    ).toBeVisible();

    // 違う名前では移らず、ダイアログは開いたまま。
    const input = dialog.getByRole("textbox", { name: nameLabel });
    await input.fill("違う名前");
    await dialog.getByRole("button", { name: "移動", exact: true }).click();
    await expect(dialog.getByRole("alert")).toHaveText("名前が一致しません");
    await expect(dialog).toBeVisible();

    await input.fill(name);
    await dialog.getByRole("button", { name: "移動", exact: true }).click();
    await expect(
      page.getByRole("status").getByText("team へ移動しました", { exact: true }),
    ).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/s/team/w/${prefix}/settings`));
    // 自分で移したあとの新しいアドレスでは、見出しの下の 1 行は出ない。
    await expect(page.getByText("このワークスペースは ")).toHaveCount(0);
  });

  await test.step("shows the workspace under team in the sidebar, and not under e2e", async () => {
    const side = await sidebar(page, info);
    await expect(
      side.getByRole("region", { name: "team" }).getByRole("link", { name, exact: true }),
    ).toBeVisible();
    await expect(
      side.getByRole("region", { name: "e2e" }).getByRole("link", { name, exact: true }),
    ).toHaveCount(0);
  });

  await test.step("opens the old address on the same tab, with the line only there", async () => {
    await page.goto(tokenUrl(`/s/e2e/w/${prefix}/overview`));
    const line = page.getByText(/^このワークスペースは .+ に team へ移動しました$/);
    await expect(line).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/s/team/w/${prefix}/overview`));
    await expect(page.getByRole("heading", { name })).toBeVisible();
    await page.getByRole("tab", { name: "スプリント" }).click();
    await expect(page).toHaveURL(new RegExp(`/s/team/w/${prefix}/sprints`));
    await expect(line).toHaveCount(0);
  });

  await test.step("opens an item from its old address", async () => {
    await page.goto(tokenUrl(`/s/e2e/i/${prefix}-1`));
    await expect(page).toHaveURL(new RegExp(`/s/team/p/${prefix}-P1/`));
    const panel = page.getByRole("region", { name: "詳細パネル" });
    await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(
      unique(info, "通知"),
    );
  });
});

test("disables moving a workspace in a read-only Store with the reason", async ({ page }) => {
  await page.goto(tokenUrl("/s/readonly/w/HH/settings"));
  const select = page.getByRole("combobox", { name: "移動先の Store" });
  await expect(select).toBeDisabled();
  await expect(select).toHaveAttribute("title", "閲覧のみの Store です");
  const button = page.getByRole("button", { name: "移動…", exact: true });
  await expect(button).toBeDisabled();
  await expect(button).toHaveAttribute("title", "閲覧のみの Store です");
});
