import { expect, type Page, type TestInfo, test } from "@playwright/test";
import { sidebar, tokenUrl, unique } from "./helpers";

// 自動化の通しのテスト(B4)。ルールはテストごとのワークスペースに作る(ほかのテストの操作で動かないように)。

const e2eStore = "01E2ESTORE0000000000000001";
const auth = { Authorization: "Bearer e2e-token" };
const prefixOf = (info: TestInfo) =>
  `A${info.project.name[0]?.toUpperCase()}${info.retry}${Math.floor(Math.random() * 1e5)}`;

async function createWorkspace(page: Page, info: TestInfo, name: string, prefix: string) {
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await side
    .getByRole("region", { name: "e2e" })
    .getByRole("button", { name: "ワークスペースを追加" })
    .click();
  const dialog = page.getByRole("dialog", { name: "ワークスペースを追加" });
  await dialog.getByRole("textbox", { name: "名前" }).fill(name);
  await dialog.getByRole("textbox", { name: "接頭辞" }).fill(prefix);
  await dialog.getByRole("button", { name: "作成" }).click();
  await expect(dialog).toBeHidden();
}

// newTaskByApi は、ワークスペース prefix にプロジェクト・Epic・Task を API で作り、Task の ID とステータスの ID を返す。
async function newTaskByApi(page: Page, prefix: string) {
  const req = page.request;
  const list = await (
    await req.get(`/api/stores/${e2eStore}/workspaces`, { headers: auth })
  ).json();
  const ws = list.find((w: { prefix: string }) => w.prefix === prefix).id as string;
  const detail = await (
    await req.get(`/api/stores/${e2eStore}/workspaces/${ws}`, { headers: auth })
  ).json();
  const status = (name: string) =>
    detail.settings.statuses.find((v: { name: string }) => v.name === name).id as string;
  const project = await (
    await req.post(`/api/stores/${e2eStore}/workspaces/${ws}/projects`, {
      headers: auth,
      data: { title: "自動化" },
    })
  ).json();
  const epic = await (
    await req.post(`/api/stores/${e2eStore}/items`, {
      headers: auth,
      data: { project: project.id, kind: "Epic", title: "E" },
    })
  ).json();
  const task = await (
    await req.post(`/api/stores/${e2eStore}/items`, {
      headers: auth,
      data: {
        project: project.id,
        parent: epic.id,
        kind: "Task",
        title: "T",
        values: { assignee: "taro" },
      },
    })
  ).json();
  return { ws, task: task.id as string, number: task.number as number, status };
}

async function openAutomation(page: Page, prefix: string) {
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/settings`));
  await page.getByRole("tab", { name: "自動化", exact: true }).click();
}

test("a rule runs on a status change and shows in recent runs", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "自動化"), prefix);
  const { task, number, status } = await newTaskByApi(page, prefix);
  await openAutomation(page, prefix);
  await expect(page.getByText("ルールはありません")).toBeVisible();
  await expect(page.getByText("まだ動いていません")).toBeVisible();

  await page.getByRole("button", { name: "ルールを作成" }).click();
  const dialog = page.getByRole("dialog", { name: "ルールを作成" });
  await dialog.getByRole("textbox", { name: "名前" }).fill("完了で外す");
  await dialog
    .getByRole("combobox", { name: "きっかけ" })
    .selectOption({ label: "ステータスが次になったとき" });
  await dialog.getByRole("combobox", { name: "ステータス" }).selectOption({ label: "Done" });
  await dialog.getByRole("combobox", { name: "すること" }).selectOption({ label: "担当者を変更" });
  await expect(dialog.getByRole("combobox", { name: "値" })).toHaveValue("@none");
  await dialog.getByRole("button", { name: "保存" }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByText("ルールを作成しました")).toBeVisible();
  await expect(
    page.getByText("ステータスが Done になったとき → 担当者を 未割り当て に"),
  ).toBeVisible();

  const getTask = async () =>
    (await page.request.get(`/api/stores/${e2eStore}/items/${task}`, { headers: auth })).json();
  // 応答の assignee をそのまま見る(項目がなければ失敗する)。
  expect(await getTask()).toHaveProperty("assignee", "taro");
  const res = await page.request.put(`/api/stores/${e2eStore}/items/${task}/properties/status`, {
    headers: auth,
    data: { value: status("Done") },
  });
  expect(res.ok()).toBeTruthy();
  expect(await getTask()).toHaveProperty("assignee", "");

  await openAutomation(page, prefix);
  const recent = page.getByRole("region", { name: "最近の実行" });
  await expect(recent).toContainText(`${prefix}-${number}`);
  await expect(recent).toContainText("「完了で外す」で担当者を なし に変更");
});

test("refuses a duplicate name and switches a rule off", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "自動化の断り"), prefix);
  await openAutomation(page, prefix);
  const open = async () => {
    await page.getByRole("button", { name: "ルールを作成" }).click();
    const dialog = page.getByRole("dialog", { name: "ルールを作成" });
    await dialog.getByRole("textbox", { name: "名前" }).fill("A");
    await dialog.getByRole("button", { name: "保存" }).click();
    return dialog;
  };
  await expect(await open()).toBeHidden();
  const dup = await open();
  await expect(dup.getByRole("alert")).toHaveText("同じ名前のルールがあります");
  await dup.getByRole("button", { name: "キャンセル" }).click();
  await expect(dup).toBeHidden();
  const sw = page.getByRole("switch", { name: "A を有効にする" });
  await expect(sw).toBeChecked();
  await sw.click();
  await expect(sw).not.toBeChecked();
});

test("edits a rule's name from the dialog", async ({ page }, info) => {
  const prefix = prefixOf(info);
  await createWorkspace(page, info, unique(info, "自動化の編集"), prefix);
  await openAutomation(page, prefix);
  await page.getByRole("button", { name: "ルールを作成" }).click();
  const create = page.getByRole("dialog", { name: "ルールを作成" });
  await create.getByRole("textbox", { name: "名前" }).fill("前の名前");
  await create.getByRole("button", { name: "保存" }).click();
  await expect(create).toBeHidden();

  await page.getByRole("button", { name: "編集", exact: true }).click();
  const edit = page.getByRole("dialog", { name: "ルールの編集" });
  await edit.getByRole("textbox", { name: "名前" }).fill("後の名前");
  await edit.getByRole("button", { name: "保存" }).click();
  // 保存に失敗すると、ダイアログは閉じず赤字の断りが出る。
  await expect(edit.getByRole("alert")).toHaveCount(0);
  await expect(edit).toBeHidden();
  await expect(page.getByRole("switch", { name: "後の名前 を有効にする" })).toBeVisible();
  await expect(page.getByRole("switch", { name: "前の名前 を有効にする" })).toHaveCount(0);
});
