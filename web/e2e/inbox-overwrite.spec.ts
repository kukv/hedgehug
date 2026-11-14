import type { APIRequestContext, Page, TestInfo } from "@playwright/test";
import { expect, test } from "./fixtures";
import { prefixOf, tokenUrl, unique } from "./helpers";

// 受信箱の上書きの行の「あなたの値 → 残った値」と「自分の値に戻す」の通しのテスト(C4c2)。同期は e2e のビルドで
// 再現できないので、上書きの行は入口 POST /e2e/notice で入れる(C4c2-12)。行は、そのテストで作った本文で探す。

const e2eStore = "01E2ESTORE0000000000000001";
// 閲覧のみの Store「readonly」と、そのサンプルのワークスペース「Web」・Task HH-2・ステータスの ID
// (cmd/hedgehug/wire_e2e.go・apptest.Sample)。
const readonlyStore = "01E2ESTORE0000000000000002";
const sampleWorkspace = "01SAMPLEWS0000000000000001";
const sampleTask = "01SAMPLEIT0000000000000002";
const sampleTodo = "01SAMPLEVAL000000000000001";
const sampleDone = "01SAMPLEVAL000000000000003";
const auth = { Authorization: "Bearer e2e-token" };

async function post(req: APIRequestContext, path: string, data: unknown) {
  const res = await req.post(path, { headers: auth, data });
  if (!res.ok()) throw new Error(`${path}: ${res.status()} ${await res.text()}`);
  return res.status() === 204 ? undefined : res.json();
}

async function put(req: APIRequestContext, path: string, data: unknown) {
  const res = await req.put(path, { headers: auth, data });
  if (!res.ok()) throw new Error(`${path}: ${res.status()} ${await res.text()}`);
}

// newTask は、テストごとのワークスペースにプロジェクト・Epic・Task を作り、Task のステータスを変える手順を返す。
async function newTask(page: Page, info: TestInfo) {
  const req = page.request;
  const prefix = prefixOf(info);
  const base = `/api/stores/${e2eStore}`;
  const ws = await post(req, `${base}/workspaces`, {
    name: unique(info, `上書き ${prefix}`),
    prefix,
  });
  const detail = await (await req.get(`${base}/workspaces/${ws.id}`, { headers: auth })).json();
  const status = (name: string) =>
    detail.settings.statuses.find((v: { name: string }) => v.name === name).id as string;
  const project = await post(req, `${base}/workspaces/${ws.id}/projects`, { title: "上書き" });
  const epic = await post(req, `${base}/items`, { project: project.id, kind: "Epic", title: "E" });
  const task = await post(req, `${base}/items`, {
    project: project.id,
    parent: epic.id,
    kind: "Task",
    title: "T",
  });
  const setStatus = (name: string) =>
    put(req, `${base}/items/${task.id}/properties/status`, { value: status(name) });
  return {
    workspace: ws.id as string,
    item: task.id as string,
    display: `${prefix}-${task.number}`,
    status,
    setStatus,
  };
}

// addOverwrite は、上書きの通知を受信箱に足し、本文 text(テストごとの名前)を返す。
async function addOverwrite(
  page: Page,
  info: TestInfo,
  n: { store: string; workspace: string; item: string; display: string; field: string },
  mine?: string,
  kept?: string,
) {
  const text = unique(info, `上書き ${prefixOf(info)}`);
  await post(page.request, "/e2e/notice", {
    Kind: "overwrite",
    Store: n.store,
    Workspace: n.workspace,
    Item: n.item,
    Display: n.display,
    Text: text,
    Sender: "hanako",
    Field: n.field,
    Mine: mine,
    Kept: kept,
  });
  return text;
}

// openRow は、受信箱の画面をアドレスから開き、本文 text の行を返す。
async function openRow(page: Page, text: string) {
  await page.goto(tokenUrl("/inbox"));
  await expect(page.getByRole("heading", { name: "受信箱" })).toBeVisible();
  return page.getByRole("listitem").filter({ hasText: text });
}

test("reverts an overwritten status to my value", async ({ page }, info) => {
  const t = await newTask(page, info);
  await t.setStatus("Done");
  const text = await addOverwrite(
    page,
    info,
    { store: e2eStore, ...t, field: "status" },
    t.status("Todo"),
    t.status("Done"),
  );
  const row = await openRow(page, text);
  await expect(row.getByText("あなたの値 Todo → 残った値 Done")).toBeVisible();
  await row.getByRole("button", { name: "自分の値に戻す" }).click();
  await expect(
    page.getByRole("status").filter({ hasText: "上書き前の値に戻しました" }),
  ).toBeVisible();
  await expect(page.locator(".toast.error")).toHaveCount(0);
  await expect(row.getByText("自分の値に戻しました", { exact: true })).toBeVisible();
  await expect(row.getByRole("button", { name: "自分の値に戻す" })).toHaveCount(0);

  await row.getByRole("link").click();
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(
    panel
      .getByRole("region", { name: "プロパティ" })
      .getByRole("button", { name: "ステータス: Todo" }),
  ).toBeVisible();
});

test("the revert button is disabled once the value changed again", async ({ page }, info) => {
  const t = await newTask(page, info);
  await t.setStatus("Done");
  const text = await addOverwrite(
    page,
    info,
    { store: e2eStore, ...t, field: "status" },
    t.status("Todo"),
    t.status("Done"),
  );
  await t.setStatus("InProgress");
  const row = await openRow(page, text);
  await expect(row.getByRole("button", { name: "自分の値に戻す" })).toBeDisabled();
  await expect(row.getByText("そのあとさらに変更されています(いまは InProgress)")).toBeVisible();
});

test("the revert button is disabled in a read-only store", async ({ page }, info) => {
  const text = await addOverwrite(
    page,
    info,
    {
      store: readonlyStore,
      workspace: sampleWorkspace,
      item: sampleTask,
      display: "HH-2",
      field: "status",
    },
    sampleDone,
    sampleTodo,
  );
  const row = await openRow(page, text);
  await expect(row.getByText("あなたの値 Done → 残った値 Todo")).toBeVisible();
  await expect(row.getByRole("button", { name: "自分の値に戻す" })).toBeDisabled();
  await expect(row.getByText("閲覧のみの Store です")).toBeVisible();
});

test("an overwrite of a body-only field shows neither values nor the button", async ({
  page,
}, info) => {
  const t = await newTask(page, info);
  const text = await addOverwrite(page, info, { store: e2eStore, ...t, field: "place" });
  const row = await openRow(page, text);
  await expect(row.getByText(text)).toBeVisible();
  await expect(row.getByText(/^あなたの値/)).toHaveCount(0);
  await expect(row.getByRole("button", { name: "自分の値に戻す" })).toHaveCount(0);
});
