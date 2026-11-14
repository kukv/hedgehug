import {
  type APIRequestContext,
  expect,
  type Locator,
  type Page,
  type TestInfo,
} from "@playwright/test";

// 通しのテストの共通の手順。

export const tokenUrl = (path = "/") => `${path}#token=e2e-token`;

// sidebar は、サイドバーを出して返す。860px 以下の幅では、引き出しを開く。
export async function sidebar(page: Page, info: TestInfo) {
  if (info.project.name !== "desktop") {
    await page.getByRole("button", { name: "メニュー", exact: true }).click();
  }
  return page.getByRole("complementary");
}

// unique は、作るものの名前に幅の名前とやり直しの回数を入れる(A2 の計画の決定 21)。
// e2e のサーバーは worker ごとに起動し(fixtures.ts)、worker は 1 つの幅しか流さない。テストが落ちると worker は
// 作り直され、やり直し(CI の retries)は新しいサーバーで流れる。なので重なりを避けるためには要らないが、
// どの幅・何回目で作ったものかを名前で見分けられるように残す。
export const unique = (info: TestInfo, title: string) =>
  `${title} ${info.project.name}-${info.retry}`;

// e2e のサーバーの Store「e2e」とサンプルのワークスペース「Web」の内部 ID(cmd/hedgehug/wire_e2e.go・apptest.Sample)。
const e2eStore = "01E2ESTORE0000000000000001";
const sampleWorkspace = "01SAMPLEWS0000000000000001";

// 接頭辞は英大文字で始まる 10 文字以内の英大文字と数字。
export const prefixOf = (info: TestInfo) =>
  `Z${info.project.name[0]?.toUpperCase()}${info.retry}${Math.floor(Math.random() * 1e5)}`;

// 作成ダイアログから、書き込める Store「e2e」にワークスペースを作る。
export async function createWorkspace(page: Page, info: TestInfo, name: string, prefix: string) {
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
  await expect(page.getByRole("heading", { name })).toBeVisible();
}

// ワークスペースの設定タブを、アドレスから開く。
export async function openSettings(page: Page, prefix: string, subtab: string) {
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}/settings`));
  await page.getByRole("tab", { name: subtab, exact: true }).click();
}

// newTask は、Epic「認証」の下に Task を作り、その詳細パネルを開いた状態にする。
export async function newTask(page: Page, info: TestInfo, title: string) {
  const name = unique(info, title);
  await page.goto(tokenUrl("/s/e2e/p/HH-P1/tree"));
  await page.getByRole("list", { name: "ツリー" }).getByRole("link", { name: "認証" }).click();
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await panel.getByRole("button", { name: "Task" }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(name);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(name);
  return { name, panel };
}

// addProperty は、サンプルのワークスペースに追加プロパティを作る(B2 の API)。
export async function addProperty(
  request: APIRequestContext,
  body: { name: string; key: string; type: string },
) {
  const res = await request.post(
    `/api/stores/${e2eStore}/workspaces/${sampleWorkspace}/properties`,
    { headers: { Authorization: "Bearer e2e-token" }, data: { ...body, kinds: [] } },
  );
  if (!res.ok()) throw new Error(`addProperty: ${res.status()} ${await res.text()}`);
}

// saved は、設定の保存の応答を待つ。待たずにアドレスで移ると、保存が終わる前に次の画面を読むことがある。
export const saved = (page: Page, path: string) =>
  page.waitForResponse((r) => r.url().includes(path) && r.request().method() !== "GET" && r.ok());

// newProject は、ワークスペース prefix に n 番目のプロジェクト(接頭辞-Pn)を作り、その概要を開いた状態にする。
// 作ったプロジェクトのタイトルを返す。
export async function newProject(page: Page, info: TestInfo, prefix: string, n = 1) {
  const title = unique(info, `計画 ${prefix} ${n}`);
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}`));
  await page.getByRole("button", { name: "プロジェクト", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "プロジェクトを作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(title);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("heading", { name: `${prefix}-P${n} ${title}` })).toBeVisible();
  return title;
}

// projectWithEpic は、プロジェクト(接頭辞-P1)と Epic(接頭辞-1)を作り、Epic の詳細パネルを開いた状態にする。
export async function projectWithEpic(page: Page, info: TestInfo, prefix: string) {
  await newProject(page, info, prefix);
  const epic = unique(info, "通知");
  await page.getByRole("button", { name: "Epic", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(epic);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(dialog).toBeHidden();
  await expect(page.getByText(`${prefix}-1 を作成しました`)).toBeVisible();
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(epic);
  return panel;
}

// createTaskFrom は、Epic の詳細パネルの「Task」(kind が Story なら「Story」)から作り、作ったものの詳細パネルが開くのを待つ。
async function createTaskFrom(page: Page, panel: Locator, name: string, kind = "Task") {
  await panel.getByRole("button", { name: kind }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(name);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(name);
}

// newTaskUnder は、projectWithEpic のあと、Epic の下に Task(接頭辞-2)を作り、その詳細パネルを開いた状態にする。
export async function newTaskUnder(page: Page, info: TestInfo, prefix: string) {
  const panel = await projectWithEpic(page, info, prefix);
  await createTaskFrom(page, panel, unique(info, "ログイン"));
  return panel;
}

// taskUnderEpic は、Epic(epicId は表示の ID)の詳細パネルを開き、その下に Task を作って、その詳細パネルを開いた状態にする。
export async function taskUnderEpic(page: Page, info: TestInfo, epicId: string, title: string) {
  await page.goto(tokenUrl(`/s/e2e/i/${epicId}`));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await createTaskFrom(page, panel, unique(info, title));
  return panel;
}

// storyUnderEpic は、Epic(epicId は表示の ID)の詳細パネルを開き、その下に Story を作って、その詳細パネルを開いた状態にする。
export async function storyUnderEpic(page: Page, info: TestInfo, epicId: string, title: string) {
  await page.goto(tokenUrl(`/s/e2e/i/${epicId}`));
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await createTaskFrom(page, panel, unique(info, title), "Story");
  return panel;
}

// addEpic は、プロジェクト(接頭辞-P1)のツリーの「Epic」ボタンから Epic を作る(projectWithEpic の 2 つ目以降)。作った Epic のタイトルを返す。
export async function addEpic(page: Page, info: TestInfo, prefix: string, title: string) {
  const name = unique(info, title);
  await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/tree`));
  await page.getByRole("button", { name: "Epic", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "新規作成" });
  await dialog.getByRole("textbox", { name: "タイトル" }).fill(name);
  await dialog.getByRole("textbox", { name: "タイトル" }).press("Enter");
  await expect(dialog).toBeHidden();
  // 保存して読み直すと、作った Epic の詳細パネルが開く(待たずにツリーを開き直すと、保存の前の一覧を読むことがある)。
  const panel = page.getByRole("region", { name: "詳細パネル" });
  await expect(panel.getByRole("textbox", { name: "タイトル" })).toHaveValue(name);
  return name;
}

// treeOf は、プロジェクト(接頭辞-P1)のツリーを開いて返す。
export async function treeOf(page: Page, prefix: string) {
  await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/tree`));
  const tree = page.getByRole("list", { name: "ツリー" });
  await expect(tree).toBeVisible();
  return tree;
}

// treeRow は、表示の ID のツリーの行。
export const treeRow = (tree: Locator, id: string) =>
  tree.getByRole("listitem").filter({ has: tree.page().getByText(id, { exact: true }) });

// treeTitles は、ツリーの行のタイトルを上から順に返す(並びの確認に使う)。
export const treeTitles = (tree: Locator) => tree.getByRole("link").allTextContents();

// githubRead は、e2e のサーバーに、GitHub を読む間隔によらない 1 回の読み取りをさせる(D4a-19)。
// そのサーバーのすべての Store を読むので、呼んだテストのリンクだけでなく、同じサーバーを使うほかのテスト
// (worker ごとに 1 つのサーバー。fixtures.ts。同じ worker で先に流れたもの)のリンクも取り直す。
export async function githubRead(request: APIRequestContext) {
  const res = await request.post("/e2e/github-read", {
    headers: { Authorization: "Bearer e2e-token" },
  });
  if (res.status() !== 204) throw new Error(`githubRead: ${res.status()} ${await res.text()}`);
}

// e2ePost は、e2e のサーバーの入口(偽の GitHub の答えを決めるもの)に POST する。
async function e2ePost(request: APIRequestContext, path: string, q: Record<string, string>) {
  const res = await request.post(`${path}?${new URLSearchParams(q)}`, {
    headers: { Authorization: "Bearer e2e-token" },
  });
  if (res.status() !== 204) throw new Error(`${path}: ${res.status()} ${await res.text()}`);
}

// githubPull は、偽の GitHub に PR を置く(D4b-16)。branch・title・body・state・comment・updated を渡せる。
export const githubPull = (
  request: APIRequestContext,
  q: { repo: string; number: string } & Record<string, string>,
) => e2ePost(request, "/e2e/github-pull", q);

// githubApp は、owner のリポジトリを GitHub App 未導入にする(missing が false なら戻す)。
export const githubApp = (request: APIRequestContext, owner: string, missing: boolean) =>
  e2ePost(request, "/e2e/github-app", { owner, missing: missing ? "1" : "0" });

// githubRepo は、リポジトリ全体を読めなくする(unreadable が false なら戻す)。
export const githubRepo = (request: APIRequestContext, repo: string, unreadable: boolean) =>
  e2ePost(request, "/e2e/github-repo", { repo, unreadable: unreadable ? "1" : "0" });

// addRepo は、プロジェクト(接頭辞-P1)の概要を開き、「リポジトリを追加」でリポジトリを足す。足したリポジトリは
// 裏ですぐに探す(D4b-9)。概要の「リポジトリと PR」の区画を返す。
export async function addRepo(page: Page, prefix: string, repo: string) {
  await page.goto(tokenUrl(`/s/e2e/p/${prefix}-P1/overview`));
  const section = page.getByRole("region", { name: "リポジトリと PR" });
  await section.getByRole("button", { name: "リポジトリを追加" }).click();
  await section.getByRole("textbox", { name: "リポジトリ" }).fill(repo);
  await section.getByRole("button", { name: "追加", exact: true }).click();
  await expect(
    section.getByRole("list", { name: "リポジトリ" }).getByText(repo, { exact: true }),
  ).toBeVisible();
  return section;
}
