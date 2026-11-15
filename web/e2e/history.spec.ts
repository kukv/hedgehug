import { expect, test } from "./fixtures";
import { newTask, tokenUrl, unique } from "./helpers";

test("the history shows the creation and a title change, newest first", async ({ page }, info) => {
  const { name, panel } = await newTask(page, info, "履歴の確認");
  const history = panel.getByRole("region", { name: "履歴" });
  await expect(history.getByRole("listitem")).toHaveCount(1);
  await expect(history.getByRole("listitem").first()).toContainText("作成");

  const title = panel.getByRole("textbox", { name: "タイトル" });
  await title.fill(`${name} 改`);
  await title.press("Enter");
  await expect(history.getByRole("listitem")).toHaveCount(2);
  await expect(history.getByRole("listitem").first()).toContainText(
    `タイトルを「${name} 改」に変更`,
  );
  await expect(history.getByRole("listitem").last()).toContainText("作成");
  await expect(history.getByRole("button", { name: "すべて表示" })).toBeHidden();

  // 説明を保存すると、履歴に「説明を変更」が増える。
  const description = panel.getByRole("region", { name: "説明" });
  await description.getByRole("button", { name: "説明を追加" }).click();
  const edit = description.getByRole("textbox", { name: "説明" });
  await edit.fill("説明です");
  await edit.press("Control+Enter");
  await expect(history.getByRole("listitem").first()).toContainText("説明を変更");
});

// 同期で受け取った保存の行には「同期で受信」が添わる。マークは、そのアイテムの最後の行(いまの保存)に付く。
test("a row saved by sync carries 同期で受信, the creation row does not", async ({
  page,
}, info) => {
  const auth = { Authorization: "Bearer e2e-token" };
  const store = "01E2ESTORE0000000000000001";
  const base = `/api/stores/${store}`;
  const created = await page.request.post(`${base}/items`, {
    headers: auth,
    data: {
      project: "01SAMPLEPR0000000000000001",
      parent: "01SAMPLEIT0000000000000001",
      kind: "Task",
      title: unique(info, "受信の履歴"),
    },
  });
  expect(created.ok()).toBe(true);
  const task = await created.json();
  const renamed = await page.request.put(`${base}/items/${task.id}/title`, {
    headers: auth,
    data: { title: unique(info, "受信の履歴 改") },
  });
  expect(renamed.ok()).toBe(true);
  const marked = await page.request.post("/e2e/receive", {
    headers: auth,
    data: { store, item: task.id },
  });
  expect(marked.ok()).toBe(true);

  await page.goto(tokenUrl(`/s/e2e/i/HH-${task.number}`));
  const history = page
    .getByRole("region", { name: "詳細パネル" })
    .getByRole("region", { name: "履歴" });
  await expect(history.getByRole("listitem")).toHaveCount(2);
  await expect(history.getByRole("listitem").first()).toContainText("同期で受信");
  await expect(history.getByRole("listitem").last()).not.toContainText("同期で受信");
});
