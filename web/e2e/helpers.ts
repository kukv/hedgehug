import { type APIRequestContext, expect, type Page, type TestInfo } from "@playwright/test";

// 通しのテストの共通の手順。

export const tokenUrl = (path = "/") => `${path}#token=e2e-token`;

// sidebar は、サイドバーを出して返す。860px 以下の幅では、引き出しを開く。
export async function sidebar(page: Page, info: TestInfo) {
  if (info.project.name !== "desktop") {
    await page.getByRole("button", { name: "メニュー", exact: true }).click();
  }
  return page.getByRole("complementary");
}

// unique は、3 つの幅が同じサーバーを使うので、作るものの名前に幅の名前を入れる(A2 の計画の決定 21)。
// やり直し(CI の retries)でも重ならないように、やり直しの回数も入れる。
export const unique = (info: TestInfo, title: string) =>
  `${title} ${info.project.name}-${info.retry}`;

// e2e のサーバーの Store「e2e」とサンプルのワークスペース「Web」の内部 ID(cmd/hedgehug/wire_e2e.go・apptest.Sample)。
const e2eStore = "01E2ESTORE0000000000000001";
const sampleWorkspace = "01SAMPLEWS0000000000000001";

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
