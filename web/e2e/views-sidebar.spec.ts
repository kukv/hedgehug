import type { Page } from "@playwright/test";
import { expect, test } from "./fixtures";
import { sidebar, tokenUrl } from "./helpers";

// サイドバーの「保存したビュー」(E3b-15)。1 件もない状態を確かめるので、ほかのテストのビューが残らない別の
// サーバーで動かす(ownServer は describe の中では使えないので、ファイルを分けている)。
test.use({ ownServer: true });

async function newView(page: Page, name: string) {
  const table = {
    columns: ["id", "title", "status", "assignee", "priority", "end", "points"],
    group: "none",
    sort: null,
  };
  const res = await page.request.post("/api/views", {
    headers: { Authorization: "Bearer e2e-token" },
    data: { name, content: { query: "kind:bug", layout: "table", table } },
  });
  if (!res.ok()) throw new Error(`newView: ${res.status()} ${await res.text()}`);
  return ((await res.json()) as { id: string }).id;
}

test("sidebar lists views by name", async ({ page }, info) => {
  await page.goto(tokenUrl("/search"));
  const side = await sidebar(page, info);
  const section = side.getByRole("region", { name: "保存したビュー" });
  await expect(section.getByRole("heading", { name: "保存したビュー" })).toBeVisible();
  await expect(section.getByText("検索の画面から保存できます")).toBeVisible();

  await newView(page, "b");
  const a = await newView(page, "A");
  await page.goto(tokenUrl("/search"));
  await sidebar(page, info);
  await expect(section.getByText("検索の画面から保存できます")).toBeHidden();
  await expect(section.getByRole("link")).toHaveText(["A", "b"]);

  await section.getByRole("link", { name: "A", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/views/${a}`));
  await expect(page.getByRole("heading", { name: "A", exact: true })).toBeVisible();
  await sidebar(page, info);
  await expect(section.getByRole("link", { name: "A", exact: true })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(section.getByRole("link", { name: "b", exact: true })).not.toHaveAttribute(
    "aria-current",
    "page",
  );
});
