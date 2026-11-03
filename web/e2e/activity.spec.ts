// Screens this spec relies on (see screens.ts for the shared list):
//   - /activity is the commits of every store: the heading "アクティビティ", the button
//     "<n> / <m> ワークスペース" that opens the dialog "ワークスペースで絞り込む" (a checkbox for each
//     workspace, named by its name and prefix), the group "変更の種類" with the buttons すべて / 人 / 同期,
//     and a row (a listitem) for each commit with the author, the op ("作成"), the subject and the ID as
//     a link; "アクティビティはありません" when there is none
//   - the filters are in the address: ?workspace=<ULID>[,<ULID>] and ?sync=exclude|only
import { expect, test } from "./harness";

test.use({ store: { name: "demo", prefix: "AL", workspace: "alpha" } });

type Ws = { ulid: string; name: string };

test("the feed lists the commits of the workspaces, and a workspace picked narrows it and stays after a reload", async ({
  page,
  hh,
}) => {
  hh.hh("workspace", "add", "beta", "--prefix", "BT");
  const a = hh.add("In alpha", "--workspace", "alpha");
  const b = hh.add("In beta", "--workspace", "beta");
  const beta = hh.json<Ws[]>("workspace", "ls").find((w) => w.name === "beta");
  if (!beta) throw new Error("no workspace beta");

  await hh.open(page, "/activity");
  await expect(page.getByRole("heading", { level: 1, name: "アクティビティ" })).toBeVisible();
  const rowA = page.getByRole("listitem").filter({ has: page.getByRole("link", { name: a.id }) });
  const rowB = page.getByRole("listitem").filter({ has: page.getByRole("link", { name: b.id }) });
  await expect(rowA).toContainText("作成");
  await expect(rowA).toContainText("e2e");
  await expect(rowB).toBeVisible();

  await page.getByRole("button", { name: /ワークスペース$/ }).click();
  await page
    .getByRole("dialog", { name: "ワークスペースで絞り込む" })
    .getByRole("checkbox", { name: /beta/ })
    .check();
  await expect(page).toHaveURL(new RegExp(`/activity\\?workspace=${beta.ulid}$`));
  await expect(rowB).toBeVisible();
  await expect(rowA).toHaveCount(0);
  await expect(page.getByRole("button", { name: "1 / 2 ワークスペース" })).toBeVisible();
  await expect(page.getByRole("button", { name: /^コマンドをコピー/ })).toHaveAccessibleName(
    "コマンドをコピー: hh log --workspace beta",
  );

  await page.reload();
  await expect(rowB).toBeVisible();
  await expect(rowA).toHaveCount(0);
});

test("syncs only has no commit when nothing has been synced", async ({ page, hh }) => {
  hh.add("Alone");
  await hh.open(page, "/activity");
  await expect(page.getByRole("listitem").first()).toBeVisible();

  await page
    .getByRole("group", { name: "変更の種類" })
    .getByRole("button", { name: "同期" })
    .click();
  await expect(page).toHaveURL(/\/activity\?sync=only$/);
  await expect(page.getByText("アクティビティはありません")).toBeVisible();
  await expect(page.getByRole("listitem")).toHaveCount(0);

  await page.getByRole("group", { name: "変更の種類" }).getByRole("button", { name: "人" }).click();
  await expect(page).toHaveURL(/\/activity\?sync=exclude$/);
  await expect(page.getByRole("listitem").first()).toBeVisible();
});
