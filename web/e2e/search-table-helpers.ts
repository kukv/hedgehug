import type { Page, TestInfo } from "@playwright/test";
import { prefixOf, resetSearchScreen, sidebar, tokenUrl } from "./helpers";

// 検索のテーブルの通しのテスト(E2a)の材料。search-table.spec.ts と search-table-select.spec.ts で使う。

export const e2eStore = "01E2ESTORE0000000000000001";
export const auth = { Authorization: "Bearer e2e-token" };
export const base = `/api/stores/${e2eStore}`;

export async function send(
  page: Page,
  method: "post" | "put" | "delete",
  path: string,
  data: unknown,
) {
  const res = await page.request[method](path, { headers: auth, data });
  if (!res.ok()) throw new Error(`${path}: ${res.status()} ${await res.text()}`);
  return res.status() === 204 ? undefined : res.json();
}

// seed は、新しいワークスペースに、Epic「{mark} A」(接頭辞-1)とその下の Story「{mark} B」(接頭辞-2)を作る。
export async function seed(page: Page, info: TestInfo) {
  const prefix = prefixOf(info);
  const mark = `表試験${prefix}`;
  const ws = await send(page, "post", `${base}/workspaces`, { name: `表 ${prefix}`, prefix });
  const project = await send(page, "post", `${base}/workspaces/${ws.id}/projects`, {
    title: `表の計画${prefix}`,
  });
  const epic = await send(page, "post", `${base}/items`, {
    project: project.id,
    kind: "Epic",
    title: `${mark} A`,
  });
  const story = await send(page, "post", `${base}/items`, {
    project: project.id,
    kind: "Story",
    parent: epic.id,
    title: `${mark} B`,
  });
  return {
    mark,
    prefix,
    workspaceId: ws.id as string,
    projectId: project.id as string,
    epicId: epic.id as string,
    storyId: story.id as string,
  };
}

// openWith は、テーブルの設定を初めの値に戻して検索の画面を開き、絞り込みを mark にして、その検索の応答を待つ。
export async function openWith(page: Page, info: TestInfo, mark: string) {
  await resetSearchScreen(page.request);
  await page.goto(tokenUrl());
  const side = await sidebar(page, info);
  const loaded = page.waitForResponse(
    (r) => r.url().includes("/api/screen-states/search") && r.request().method() === "GET",
  );
  await side.getByRole("link", { name: "検索", exact: true }).click();
  await loaded;
  await type(page, mark);
}

export async function type(page: Page, query: string) {
  const done = page.waitForResponse(
    (r) => r.url().endsWith("/api/search") && r.request().postDataJSON()?.query === query,
  );
  await page.getByRole("combobox", { name: "絞り込み" }).fill(query);
  await done;
}

export const headers = (page: Page) => page.getByRole("main").getByRole("columnheader");
// lastHeader は、いちばん右の列の見出し。末尾の「⋯」の列の見出しは空なので数えない。
export const lastHeader = (page: Page) => headers(page).filter({ hasText: /\S/ }).last();
export const rowOf = (page: Page, title: string) =>
  page.getByRole("row", { name: new RegExp(title) });
