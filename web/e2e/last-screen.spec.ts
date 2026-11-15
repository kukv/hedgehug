import type { Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { prefixOf, send, tokenUrl, unique } from "./helpers";
import { base } from "./search-table-helpers";

// 最後に開いていた画面の通しのテスト(01「画面のアドレス」、E4a-14・E4a-20)。同じ worker のサーバーをほかのテストと
// 共有するので、/ を開くテストは、先に最後の画面を置くか、書き終えるのを待ってから開く。

// written は、最後の画面に path を書き終えるのを待つ(書き終える前に / を開くと、前の画面が開く)。本文は
// {"body": "{\"path\":\"…\"}"} なので、path のあとのエスケープした引用符まで見て、前方の一致を除く。
const written = (page: Page, path: string) =>
  page.waitForResponse(
    (r) =>
      r.url().endsWith("/api/screen-states/last-screen") &&
      r.request().method() === "PUT" &&
      (r.request().postData() ?? "").includes(`"${path}\\"`),
  );

// e2e のサーバーの Store「team」の内部 ID(cmd/hedgehug/wire_e2e.go)。
const teamStore = "01E2ESTORE0000000000000003";

// putLastScreen は、最後の画面を API で置く(null は、覚えていない)。
const putLastScreen = (page: Page, path: string | null) =>
  send(page, "put", "/api/screen-states/last-screen", {
    body: JSON.stringify(path === null ? null : { path }),
  });

const myTasksHeading = (page: Page) => page.getByRole("heading", { name: "マイタスク", level: 1 });

test("opens the last screen", async ({ page }) => {
  await page.goto(tokenUrl("/s/e2e/w/HH"));
  const tabs = page.getByRole("tablist");
  const done = written(page, "/s/e2e/w/HH/sprints");
  await tabs.getByRole("tab", { name: "スプリント" }).click();
  await done;

  await page.goto(tokenUrl("/"));
  await expect(tabs.getByRole("tab", { name: "スプリント" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(page).toHaveURL(/\/s\/e2e\/w\/HH\/sprints$/);
});

test("opens my tasks the first time", async ({ page }) => {
  await putLastScreen(page, null);
  await page.goto(tokenUrl("/"));
  await expect(myTasksHeading(page)).toBeVisible();
  await expect(page).toHaveURL(/\/my-tasks$/);
});

// 最後の画面がもうないときは、止まらず・繰り返さずに、理由のトーストを 1 回出してマイタスクを開く(Review Focus 4)。
// 理由のトーストは転送の記録で見つけたときだけ出る(01「画面のアドレス」)ので、team へ移してから削除する。開いたままだと、
// 移した知らせで画面が解決し直して最後の画面をマイタスクに書き換えるので、アプリを離れてから移し、最後の画面を置き直す。
test("opens my tasks when the last screen is gone", async ({ page }, info) => {
  const prefix = prefixOf(info);
  const name = unique(info, `最後の画面 ${prefix}`);
  const ws = await send(page, "post", `${base}/workspaces`, { name, prefix });
  const path = `/s/e2e/w/${prefix}/overview`;
  const done = written(page, path);
  await page.goto(tokenUrl(`/s/e2e/w/${prefix}`));
  await expect(page.getByRole("heading", { name })).toBeVisible();
  await done;
  await page.goto("about:blank");
  const moved = await send(page, "post", `${base}/workspaces/${ws.id}/transfer`, {
    toStore: teamStore,
  });
  await send(page, "delete", `/api/stores/${teamStore}/workspaces/${moved.workspaceId}`, undefined);
  await putLastScreen(page, path);

  const toMyTasks = written(page, "/my-tasks");
  await page.goto(tokenUrl("/"));
  await expect(myTasksHeading(page)).toBeVisible();
  const toast = page
    .getByRole("status")
    .getByText(
      /^このワークスペースは .+ に team へ移動しましたが、削除されています。設定の「削除済み」から復元できます$/,
    );
  await expect(toast).toBeVisible();
  await toMyTasks;
  await expect(toast).toHaveCount(1);
});

test("unknown address opens my tasks", async ({ page }) => {
  await page.goto(tokenUrl("/nope"));
  await expect(myTasksHeading(page)).toBeVisible();
});
