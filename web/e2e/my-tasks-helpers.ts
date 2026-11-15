import { expect, type Page, type TestInfo } from "@playwright/test";
import { prefixOf, send, tokenUrl } from "./helpers";
import { auth, base } from "./search-table-helpers";

// マイタスクの通しのテスト(E4a)の材料。アイテムはサンプルのプロジェクト HH-P1・Epic HH-1 の下に作る
// (apptest.Sample。helpers.ts の apiTask と同じ本文)。

const sampleWorkspace = "01SAMPLEWS0000000000000001";

const pad = (n: number) => String(n).padStart(2, "0");

// ymd は、このマシンの時刻帯での日付(YYYY-MM-DD)。サーバーは同じマシンで動く。
export const ymd = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;

// shownDay は、YYYY-MM-DD を画面の日付の形(今年は M/D、ほかの年は YYYY/MM/DD)にする。core の formatDay と同じ。
export function shownDay(day: string, now = new Date()): string {
  const [y, m, d] = day.split("-");
  if (Number(y) === now.getFullYear()) return `${Number(m)}/${Number(d)}`;
  return `${y}/${m}/${d}`;
}

// dayFrom は、今日から n 日あとの日付(YYYY-MM-DD)。正午で作るので、夏時間でもずれない。
export function dayFrom(n: number): string {
  const t = new Date();
  return ymd(new Date(t.getFullYear(), t.getMonth(), t.getDate() + n, 12));
}

// markOf は、テストごとの絞り込みの印。ほかのテストが作ったアイテムを絞り込みで除く。
export const markOf = (info: TestInfo) => `マイ試験${prefixOf(info)}`;

// resetMyTasks は、マイタスクの画面の状態を初めの値に戻す(同じ worker のサーバーをほかのテストと共有するため)。
export async function resetMyTasks(page: Page) {
  await send(page, "put", "/api/screen-states/my-tasks", {
    body: JSON.stringify({ filter: "", groupBy: "due" }),
  });
}

// valueId は、サンプルのワークスペースの設定から、list(statuses・priorities)の値の ID を名前で引く。
async function valueId(page: Page, list: "statuses" | "priorities", name: string) {
  const res = await page.request.get(`${base}/workspaces/${sampleWorkspace}`, { headers: auth });
  const values = (await res.json()).settings[list] as { id: string; name: string }[];
  const found = values.find((v) => v.name === name);
  if (!found) throw new Error(`${list} に ${name} がない`);
  return found.id;
}

const setProperty = (page: Page, item: string, property: string, value: string) =>
  send(page, "put", `${base}/items/${item}/properties/${property}`, { value });

// myTask は、taro が担当する Task を作り、内部 ID を返す。end は終了日(YYYY-MM-DD)、priority は優先度の名前。
export async function myTask(
  page: Page,
  title: string,
  opts: { end?: string; priority?: string } = {},
): Promise<string> {
  const task = await send(page, "post", `${base}/items`, {
    project: "01SAMPLEPR0000000000000001",
    parent: "01SAMPLEIT0000000000000001",
    kind: "Task",
    title,
  });
  await setProperty(page, task.id, "assignee", "taro");
  if (opts.end) await setProperty(page, task.id, "end", opts.end);
  if (opts.priority) {
    await setProperty(page, task.id, "priority", await valueId(page, "priorities", opts.priority));
  }
  return task.id as string;
}

// openMyTasks は、マイタスクの画面をアドレスから開き、画面の状態を読み終えるのを待つ。reload なら、開いている画面を
// 読み込み直す(同じアドレスへの goto は、ハッシュだけの違いなのでページを読み直さない)。
export async function openMyTasks(page: Page, reload = false) {
  const loaded = page.waitForResponse(
    (r) => r.url().includes("/api/screen-states/my-tasks") && r.request().method() === "GET",
  );
  if (reload) await page.reload();
  else await page.goto(tokenUrl("/my-tasks"));
  await loaded;
  await expect(page.getByRole("heading", { name: "マイタスク", level: 1 })).toBeVisible();
}

// filterBy は、絞り込みに query を打ち、その問い合わせの応答を待つ。
export async function filterBy(page: Page, query: string) {
  const done = page.waitForResponse(
    (r) => r.url().endsWith("/api/my-tasks") && r.request().postDataJSON()?.filter === query,
  );
  await page.getByRole("combobox", { name: "絞り込み" }).fill(query);
  await done;
}

// groupHeadings は、本体の見出し(まとめ方のグループ)。
export const groupHeadings = (page: Page) =>
  page.getByRole("main").getByRole("heading", { level: 2 });

// rowOf は、タイトルを含む行。
export const rowOf = (page: Page, title: string) =>
  page.getByRole("main").getByRole("listitem").filter({ hasText: title });
