import { expect, type Locator, type Page, type TestInfo } from "@playwright/test";
import { prefixOf, resetSearchScreen, tokenUrl } from "./helpers";

// 検索のツリーの通しのテスト(E3a)の材料。search-tree.spec.ts と search-tree-keys.spec.ts で使う。
// 書き込むテストは、API で作った新しいワークスペースだけを使う(サンプルは同じ worker のほかのテストと共有のため)。

// e2e のサーバーの Store の内部 ID(cmd/hedgehug/wire_e2e.go)。readonly は閲覧のみ。
export const e2eStore = "01E2ESTORE0000000000000001";
export const readonlyStore = "01E2ESTORE0000000000000002";
// サンプル(apptest.Sample)の内部 ID。Store「e2e」と Store「readonly」の両方に同じ ID で入っている。
export const sampleProject = "01SAMPLEPR0000000000000001";
export const sampleEpic = "01SAMPLEIT0000000000000001";
export const sampleTask = "01SAMPLEIT0000000000000002";

const auth = { Authorization: "Bearer e2e-token" };
const base = `/api/stores/${e2eStore}`;

async function post(page: Page, path: string, data: unknown) {
  const res = await page.request.post(path, { headers: auth, data });
  if (!res.ok()) throw new Error(`${path}: ${res.status()} ${await res.text()}`);
  return res.json();
}

// Seed は、Store「e2e」に作った新しいワークスペース。mark はアイテムのタイトルに入れる語(接頭辞を含むので、ほかの
// テストと重ならない)。プロジェクトのタイトルには mark を入れない(絞り込みに一致させないため)。
export interface Seed {
  prefix: string;
  mark: string;
  // project は、n 番目のプロジェクト(タイトルは `樹の計画{接頭辞} {n}`)を作り、内部 ID とタイトルを返す。
  project(n: number): Promise<{ id: string; title: string }>;
  // item は、アイテムを作り、内部 ID を返す。
  item(projectId: string, kind: string, title: string, parent?: string): Promise<string>;
}

export async function seed(page: Page, info: TestInfo): Promise<Seed> {
  const prefix = prefixOf(info);
  const ws = await post(page, `${base}/workspaces`, { name: `樹 ${prefix}`, prefix });
  return {
    prefix,
    mark: `樹試験${prefix}`,
    async project(n) {
      const title = `樹の計画${prefix} ${n}`;
      const p = await post(page, `${base}/workspaces/${ws.id}/projects`, { title });
      return { id: p.id, title };
    },
    async item(project, kind, title, parent) {
      return (await post(page, `${base}/items`, { project, kind, title, parent })).id;
    },
  };
}

export const filterOf = (page: Page) => page.getByRole("combobox", { name: "絞り込み" });

// search は、絞り込みを query にして、その検索の応答を待つ。
export async function search(page: Page, query: string) {
  const done = page.waitForResponse(
    (r) =>
      r.url().endsWith("/api/search") &&
      r.request().method() === "POST" &&
      r.request().postDataJSON()?.query === query,
  );
  await filterOf(page).fill(query);
  await done;
}

// openTree は、画面の状態をツリーにして検索の画面を開き、絞り込みを query にする。サイドバーの「検索」は
// テーブルで開く入口(E3a-9)なので、アドレスで開く(画面の状態のレイアウトで開く)。
export async function openTree(page: Page, query: string) {
  await resetSearchScreen(page.request, "tree");
  const loaded = page.waitForResponse(
    (r) => r.url().includes("/api/screen-states/search") && r.request().method() === "GET",
  );
  await page.goto(tokenUrl("/search"));
  await loaded;
  await search(page, query);
  const tree = page.getByRole("list", { name: "ツリー" });
  await expect(tree).toBeVisible();
  return tree;
}

// rowOf は、Store storeId の内部 ID id の行(行の鍵は `{storeId}/{内部 ID}`。E3a-5)。
export const rowOf = (tree: Locator, storeId: string, id: string) =>
  tree.locator(`li[data-row="${storeId}/${id}"]`);

export const currentOf = (tree: Locator) => tree.locator('li[aria-current="true"]');

// pick は、行の Store の点を押して、キーのフォーカスをその行に付ける(タイトルを押すと開いてしまう)。
export const pick = (row: Locator) => row.locator(":scope > i.sdot").click();

// rowKeys は、描いている行の鍵を上から順に返す。
export const rowKeys = (tree: Locator) =>
  tree
    .locator("li[data-row]")
    .evaluateAll((els) => els.map((el) => (el as HTMLElement).dataset.row ?? ""));

// titlesWith は、ツリーの行のタイトルのうち、text を含むものを上から順に返す。
export async function titlesWith(tree: Locator, text: string) {
  const all = await tree.getByRole("link").allTextContents();
  return all.filter((t) => t.includes(text));
}
