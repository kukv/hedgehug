import type { APIRequestContext, Locator, Page, TestInfo } from "@playwright/test";
import { expect, test } from "./fixtures";
import { prefixOf, sidebar, tokenUrl, unique } from "./helpers";

// 受信箱の通しのテスト(C4a)。テストごとのワークスペースに「InProgress になったら Done にする」ルールと、
// InProgress から Cancel にしか変えられない遷移の制約を作り、アイテムを InProgress にしてルールを止める。
// 同じ worker のテストは同じサーバー(同じ受信箱)を使うので、未読数は前のテストの通知と足し算になる。
// 未読数は、そのテストの前の数との差で確かめる。行は、そのテストで作ったルールの名前で探す。

const e2eStore = "01E2ESTORE0000000000000001";
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

// unread は、受信箱のタブごとの未読数(サーバーの値)。
async function unread(req: APIRequestContext) {
  const res = await req.get("/api/inbox/unread", { headers: auth });
  return (await res.json()).counts as { all: number; automation: number };
}

// tabName は、未読数 n のタブの名前。0 なら数字を出さない。
const tabName = (label: string, n: number) => (n === 0 ? label : `${label} ${n}`);

// stoppedNotice は、新しいワークスペースでルールを制約で止め、受信箱に止まった通知を 1 件作る。
// 作る前の「すべて」と「自動化」の未読数(before・automationBefore)も返す。
async function stoppedNotice(page: Page, info: TestInfo) {
  const req = page.request;
  const { all: before, automation: automationBefore } = await unread(req);
  const prefix = prefixOf(info);
  const base = `/api/stores/${e2eStore}`;
  const ws = await post(req, `${base}/workspaces`, {
    name: unique(info, `受信箱 ${prefix}`),
    prefix,
  });
  const detail = await (await req.get(`${base}/workspaces/${ws.id}`, { headers: auth })).json();
  const status = (name: string) =>
    detail.settings.statuses.find((v: { name: string }) => v.name === name).id as string;
  const rule = unique(info, `完了にする ${prefix}`);
  await post(req, `${base}/workspaces/${ws.id}/automation/rules`, {
    name: rule,
    trigger: "status",
    triggerStatus: status("InProgress"),
    allMerged: false,
    action: "status",
    value: status("Done"),
  });
  // InProgress から変えられる先を Cancel だけにする(Done へは変えられない)。
  await put(
    req,
    `${base}/workspaces/${ws.id}/workflow/${status("InProgress")}/${status("Cancel")}`,
    {
      allowed: true,
    },
  );
  const project = await post(req, `${base}/workspaces/${ws.id}/projects`, { title: "受信箱" });
  const epic = await post(req, `${base}/items`, { project: project.id, kind: "Epic", title: "E" });
  const task = await post(req, `${base}/items`, {
    project: project.id,
    parent: epic.id,
    kind: "Task",
    title: "T",
  });
  await put(req, `${base}/items/${task.id}/properties/status`, { value: status("InProgress") });
  const statusOf = async () =>
    (await (await req.get(`${base}/items/${task.id}`, { headers: auth })).json()).status as string;
  const setStatus = (name: string) =>
    put(req, `${base}/items/${task.id}/properties/status`, { value: status(name) });
  return { before, automationBefore, rule, status, statusOf, setStatus };
}

// badge は、サイドバーの「受信箱」の未読数のバッジ。860px 以下の幅では引き出しを開く。
async function badge(page: Page, info: TestInfo) {
  const side = await sidebar(page, info);
  const link = side
    .getByRole("navigation", { name: "ナビ" })
    .getByRole("link", { name: /^受信箱/ });
  return { link, count: link.getByRole("img", { name: /^未読 \d+ 件$/ }) };
}

async function expectCount(count: Locator, n: number) {
  if (n === 0) await expect(count).toHaveCount(0);
  else await expect(count).toHaveAccessibleName(`未読 ${n} 件`);
}

// openInbox は、受信箱の画面をアドレスから開き、ルール rule の通知の行を返す。
async function openInbox(page: Page, rule: string) {
  await page.goto(tokenUrl("/inbox"));
  await expect(page.getByRole("heading", { name: "受信箱" })).toBeVisible();
  return rowOf(page, rule);
}

const rowOf = (page: Page, rule: string) =>
  page.getByRole("listitem").filter({ hasText: `「${rule}」が制約で止まりました` });

test("a stopped notice shows in the inbox and applies ignoring the workflow", async ({
  page,
}, info) => {
  const n = await stoppedNotice(page, info);
  await page.goto(tokenUrl());
  const side = await badge(page, info);
  await expectCount(side.count, n.before + 1);
  // サイドバーの「受信箱」から開く(画面を移ると引き出しは閉じる)。
  await side.link.click();
  await expect(page.getByRole("heading", { name: "受信箱" })).toBeVisible();

  const row = rowOf(page, n.rule);
  await expect(row.getByText("自動化", { exact: true })).toBeVisible();
  await expect(row.getByText(`「${n.rule}」が制約で止まりました`)).toBeVisible();
  await expect(
    row.getByText("ワークフローの制約: InProgress から Done には変更できません(Cancel のみ)"),
  ).toBeVisible();
  await expect(row.getByRole("button", { name: "既読にする" })).toBeVisible();

  await row.getByRole("button", { name: "制約を無視して Done にする" }).click();
  const toast = page.getByRole("status").filter({ hasText: "適用しました" });
  await expect(toast).toBeVisible();
  await expect(toast).not.toHaveClass(/error/);
  await expect(row.getByText("適用しました", { exact: true })).toBeVisible();
  await expect(row.getByRole("button", { name: /^制約を無視して/ })).toHaveCount(0);
  await expect(row.getByRole("button", { name: "未読にする" })).toBeVisible();
  expect(await n.statusOf()).toBe(n.status("Done"));
  await expectCount((await badge(page, info)).count, n.before);
});

test("keeping a stopped notice leaves the status as it is", async ({ page }, info) => {
  const n = await stoppedNotice(page, info);
  const row = await openInbox(page, n.rule);
  await row.getByRole("button", { name: "変更しない" }).click();
  await expect(row.getByText("このままにしました", { exact: true })).toBeVisible();
  await expect(row.getByRole("button", { name: "変更しない" })).toHaveCount(0);
  await expect(row.getByRole("button", { name: "未読にする" })).toBeVisible();
  expect(await n.statusOf()).toBe(n.status("InProgress"));
  await expectCount((await badge(page, info)).count, n.before);
});

test("the apply button is disabled once the status changed first", async ({ page }, info) => {
  const n = await stoppedNotice(page, info);
  await n.setStatus("Cancel");
  const row = await openInbox(page, n.rule);
  await expect(row.getByRole("button", { name: "制約を無視して Done にする" })).toBeDisabled();
  await expect(row.getByRole("button", { name: "変更しない" })).toBeEnabled();
  await expect(row.getByText("すでに変更されています(いまは Cancel)")).toBeVisible();
});

test("toggles read with the dot and reads all in the automation tab", async ({ page }, info) => {
  const n = await stoppedNotice(page, info);
  const row = await openInbox(page, n.rule);
  const all = page.getByRole("tab", { name: /^すべて/ });
  await expect(all).toHaveAccessibleName(tabName("すべて", n.before + 1));

  await row.getByRole("button", { name: "既読にする" }).click();
  await expect(row.getByRole("button", { name: "未読にする" })).toBeVisible();
  await expect(all).toHaveAccessibleName(tabName("すべて", n.before));
  await row.getByRole("button", { name: "未読にする" }).click();
  await expect(row.getByRole("button", { name: "既読にする" })).toBeVisible();
  await expect(all).toHaveAccessibleName(tabName("すべて", n.before + 1));

  // 「自動化」のタブの「すべて既読にする」は、前のテストの自動化の通知も既読にする。ほかのタブの未読は残る。
  const automation = page.getByRole("tab", { name: /^自動化/ });
  await expect(automation).toHaveAccessibleName(tabName("自動化", n.automationBefore + 1));
  await automation.click();
  await expect(automation).toHaveAttribute("aria-selected", "true");
  const readAll = page.getByRole("button", { name: "すべて既読にする" });
  await readAll.click();
  await expect(row.getByRole("button", { name: "未読にする" })).toBeVisible();
  await expect(automation).toHaveAccessibleName("自動化");
  await expect(readAll).toBeDisabled();
  const rest = n.before - n.automationBefore;
  await expect(all).toHaveAccessibleName(tabName("すべて", rest));
  await expectCount((await badge(page, info)).count, rest);
});

test("another tab receives the unread count", async ({ page }, info) => {
  const n = await stoppedNotice(page, info);
  const other = await page.context().newPage();
  const otherRow = await openInbox(other, n.rule);
  await expect(otherRow.getByRole("button", { name: "既読にする" })).toBeVisible();
  const otherAll = other.getByRole("tab", { name: /^すべて/ });
  await expect(otherAll).toHaveAccessibleName(tabName("すべて", n.before + 1));

  const row = await openInbox(page, n.rule);
  await row.getByRole("button", { name: "既読にする" }).click();
  await expect(row.getByRole("button", { name: "未読にする" })).toBeVisible();

  await expect(otherRow.getByRole("button", { name: "未読にする" })).toBeVisible();
  await expect(otherAll).toHaveAccessibleName(tabName("すべて", n.before));
  await expectCount((await badge(other, info)).count, n.before);
  await other.close();
});

test("the overwrite tab shows the empty message and its note", async ({ page }) => {
  await page.goto(tokenUrl("/inbox"));
  await page.getByRole("tab", { name: /^上書き/ }).click();
  await expect(page.getByText("お知らせはありません")).toBeVisible();
  await expect(
    page.getByText(
      "このマシンで変えた値が、同期であとから届いた変更に負けると、ここに出ます。試すには、値を変えてからサイドバーの同期ボタンを押してください。",
    ),
  ).toBeVisible();
});
