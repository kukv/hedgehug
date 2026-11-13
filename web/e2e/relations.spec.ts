import { expect, test } from "@playwright/test";
import { newTask } from "./helpers";

test("adds a blocked by relation, shows Blocked in the panel and the tree, and removes it", async ({
  page,
}, info) => {
  const blocker = await newTask(page, info, "ブロックする");
  const blocked = await newTask(page, info, "ブロックされる");
  const panel = blocked.panel;
  const rel = panel.getByRole("region", { name: "関連" });
  await expect(rel.getByText("関連はありません")).toBeVisible();

  await rel.getByRole("combobox", { name: "関連の表記" }).selectOption("blocked by");
  await rel.getByRole("textbox", { name: "相手" }).fill(blocker.name);
  await rel.getByRole("button", { name: blocker.name }).click();
  await rel.getByRole("button", { name: "追加", exact: true }).click();

  await expect(rel.getByRole("heading", { name: "blocked by" })).toBeVisible();
  await expect(rel.getByRole("link", { name: blocker.name })).toBeVisible();
  await expect(panel.getByText("ブロック中")).toBeVisible();
  const treeRow = page
    .getByRole("list", { name: "ツリー" })
    .getByRole("listitem")
    .filter({ hasText: blocked.name });
  await expect(treeRow).toContainText("ブロック中");

  // 同じ種類の関連がある相手は、候補に出ても選べず、理由が付く。
  await rel.getByRole("textbox", { name: "相手" }).fill(blocker.name);
  await expect(rel.getByRole("button", { name: blocker.name })).toBeDisabled();
  await expect(rel.getByText("すでに blocked by の関連があります")).toBeVisible();

  await rel.getByRole("button", { name: /を解除$/ }).click();
  await expect(rel.getByText("関連はありません")).toBeVisible();
  await expect(panel.getByText("ブロック中")).toBeHidden();
  await expect(treeRow).not.toContainText("ブロック中");
});
