import type { APIRequestContext, Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { prefixOf, sidebar, tokenUrl, unique } from "./helpers";

// アクティビティの通しのテスト(C4d。06「アクティビティ」)。保存は API で行い、Store の変更の知らせで画面が読み直されることを確かめる。
// 同じ worker のほかのテストも同じサーバーに保存と同期の行を残すので、行はこのテストで作ったアイテムの ID と同期の件数で探す。

const e2eStore = "01E2ESTORE0000000000000001";
const sampleProject = "01SAMPLEPR0000000000000001";
const sampleEpic = "01SAMPLEIT0000000000000001";
const auth = { Authorization: "Bearer e2e-token" };

async function send(req: APIRequestContext, method: "post" | "put", path: string, data: unknown) {
  const res = await req[method](path, { headers: auth, data });
  if (!res.ok()) throw new Error(`${path}: ${res.status()} ${await res.text()}`);
  return res.status() === 204 ? undefined : res.json();
}

const rowOf = (page: Page, text: string) => page.getByRole("listitem").filter({ hasText: text });

test("lists saves and sync rows by day, with links and filters", async ({ page }, info) => {
  const base = `/api/stores/${e2eStore}`;
  const task = await send(page.request, "post", `${base}/items`, {
    project: sampleProject,
    parent: sampleEpic,
    kind: "Task",
    title: unique(info, "アクティビティ"),
  });
  // サンプルのワークスペース Web の接頭辞は HH。
  const id = `HH-${task.number}`;

  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  await side
    .getByRole("navigation", { name: "ナビ" })
    .getByRole("link", { name: "アクティビティ" })
    .click();
  await expect(page).toHaveURL(/\/activity$/);
  await expect(page.getByRole("heading", { name: "アクティビティ" })).toBeVisible();
  await expect(page.getByText("Store の git 履歴。1 つの変更が 1 つのコミットです")).toBeVisible();

  // 最初の読み込みで作成の行が出てから、画面を開いたまま保存すると、読み直して「今日」の先頭に出す。
  await expect(rowOf(page, `Task「${unique(info, "アクティビティ")}」を作成`)).toBeVisible();
  await send(page.request, "put", `${base}/items/${task.id}/title`, {
    title: unique(info, "アクティビティ 改"),
  });
  const today = page.getByRole("region", { name: "今日" });
  const renamed = rowOf(page, `${id} のタイトルを変更`);
  await expect(today.getByRole("listitem").first()).toContainText(`${id} のタイトルを変更`);
  await expect(renamed.locator("time")).toHaveText(/^\d{2}:\d{2}$/);
  await expect(renamed.getByText("e2e", { exact: true })).toBeVisible();
  await expect(renamed.getByText("Web", { exact: true })).toBeVisible();
  await expect(renamed.getByRole("link", { name: id, exact: true })).toBeVisible();
  await expect(renamed.locator(".sha")).toHaveText(/^[0-9a-f]{7}$/);

  // 種類「同期だけ」は、同期の行だけを出す。同期の行は SHA の欄が空。
  await send(page.request, "post", "/e2e/sync-log", { store: e2eStore, sent: 2, received: 1 });
  const kind = page.getByRole("combobox", { name: "種類" });
  await kind.selectOption({ label: "同期だけ" });
  const synced = rowOf(page, "origin と同期 (送信 2 件・受信 1 件)");
  await expect(synced).toBeVisible();
  await expect(synced.getByText("同期", { exact: true })).toBeVisible();
  await expect(synced.locator(".sha")).toHaveText("");
  await expect(renamed).toHaveCount(0);

  // ワークスペースの絞り込み: Web では出て、Mobile では Web の行が出ない。
  await kind.selectOption({ label: "すべて" });
  const workspace = page.getByRole("combobox", { name: "ワークスペース" });
  await workspace.selectOption({ label: "e2e / Web" });
  await expect(renamed).toBeVisible();
  await workspace.selectOption({ label: "e2e / Mobile" });
  await expect(synced).toBeVisible();
  await expect(renamed).toHaveCount(0);

  // ID のリンクで詳細パネルを開く(画面はアクティビティのまま)。
  await workspace.selectOption({ label: "すべて" });
  await renamed.getByRole("link", { name: id, exact: true }).click();
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(
    unique(info, "アクティビティ 改"),
  );
  await expect(page).toHaveURL(/\/activity$/);
});

test("returns the workspace filter to すべて when the selected workspace is deleted", async ({
  page,
}, info) => {
  const base = `/api/stores/${e2eStore}`;
  const prefix = prefixOf(info);
  const name = unique(info, `アクティビティ ${prefix}`);
  const ws = await send(page.request, "post", `${base}/workspaces`, { name, prefix });

  await page.goto(tokenUrl("/activity"));
  const workspace = page.getByRole("combobox", { name: "ワークスペース" });
  await workspace.selectOption({ label: `e2e / ${name}` });
  await expect(workspace).toHaveValue(`${e2eStore}/${ws.id}`);

  // 画面を開いたまま API で削除すると、選択肢から消え、絞り込みを「すべて」に戻して読み直す。
  // 選んでいた option が消えるとブラウザは先頭の「すべて」を出すので、表示だけでなく、ワークスペースを付けない読み直しを待つ。
  const all = page.waitForRequest(
    (r) => new URL(r.url()).pathname === "/api/activity" && !r.url().includes("workspace="),
  );
  const res = await page.request.delete(`${base}/workspaces/${ws.id}`, { headers: auth });
  if (!res.ok()) throw new Error(`delete-workspace: ${res.status()} ${await res.text()}`);
  await expect(workspace.locator("option", { hasText: `e2e / ${name}` })).toHaveCount(0);
  await all;
  await expect(workspace).toHaveValue("");
});
