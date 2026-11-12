import type { Page, TestInfo } from "@playwright/test";

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
