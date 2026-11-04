// Screens this spec relies on (see screens.ts for the shared list):
//   - the "…" of a task ("その他の操作") and of a row of the tree ("メニュー: <タイトル>") have the item
//     "別の Store へ移動…" when two Stores are mounted and the Store of the task can be written; the
//     palette has the command of the same name for the selected task
//   - the dialog "別の Store へ移動" has the radio group "移動先の Store" (a radio for each other Store,
//     disabled with "閲覧のみのため移動できません（…）" when it is read-only), the combobox "<Store> のワーク
//     スペース" (when the Store has more than one), the combobox "親を選ぶ", the region "移動先で変わること"
//     (its heading names it), the region "外す参照" and the button "n 件の参照を外して移動" / "移動"
import type { Page } from "@playwright/test";
import { expect, type Hedgehug, test } from "./harness";
import { row, toast } from "./screens";

/** Two Stores: demo (DM) with a parent, its child and a task blocked by the parent; second (SC) with two workspaces. */
function fixture(hh: Hedgehug) {
  const parent = hh.add("Parent task");
  const child = hh.add("Child task", "--parent", parent.id);
  const blocked = hh.add("Blocked task");
  hh.hh("dep", "add", blocked.id, "--blocked-by", parent.id);
  hh.hh("store", "init", "second", "--prefix", "SC", "--workspace", "a");
  hh.hh("workspace", "add", "b", "--prefix", "SB", "--store", "second");
  return { parent, child, blocked };
}

const dialog = (page: Page) => page.getByRole("dialog", { name: "別の Store へ移動" });

async function openFromDetail(page: Page, hh: Hedgehug, ulid: string) {
  await hh.open(page, `/tasks/${ulid}`);
  await page.getByRole("button", { name: "その他の操作" }).click();
  await page.getByRole("menuitem", { name: "別の Store へ移動…" }).click();
  await expect(dialog(page)).toBeVisible();
}

test("moves a task and its descendants to another Store, cutting the references that cross", async ({
  page,
  hh,
}) => {
  const { parent, blocked } = fixture(hh);
  await openFromDetail(page, hh, parent.ulid);
  const d = dialog(page);

  // Nothing is asked until a Store is picked, and the one the task is in is not offered.
  await expect(d.getByRole("radio", { name: /^demo/ })).toHaveCount(0);
  await expect(d.getByRole("button", { name: "移動", exact: true })).toBeDisabled();
  await d.getByRole("radio", { name: /^second/ }).check();

  // second has two workspaces: the plan waits for one.
  await expect(d.getByRole("region", { name: "移動先で変わること" })).toHaveCount(0);
  await d.getByRole("combobox", { name: "second のワークスペース" }).selectOption("a");

  const changes = d.getByRole("region", { name: "移動先で変わること" });
  await expect(changes).toContainText("移動先の a で振り直します");
  await expect(changes).toContainText("a のルートに置きます");
  await expect(d).toContainText("子孫 1");
  const cut = d.getByRole("region", { name: /^外す参照/ });
  await expect(cut.getByRole("listitem")).toHaveCount(1);
  await expect(cut).toContainText("ブロックされている");
  await expect(cut).toContainText(blocked.id);
  await expect(d.getByRole("button", { name: /コマンドをコピー/ })).toContainText(
    `hh mv ${parent.id} --store second --workspace a --yes`,
  );
  // what the design leaves out
  await expect(d).not.toContainText("rewrite-history");
  await expect(d).not.toContainText("消すには");

  await d.getByRole("button", { name: "1 件の参照を外して移動" }).click();
  await expect(
    toast(page, new RegExp(`${parent.id} を second に移動しました（SC-\\d+）`)),
  ).toBeVisible();
  await expect(dialog(page)).toHaveCount(0);

  // It is in second, under the same ULID, and the old ID still finds it.
  const moved = hh.show("SC-1");
  expect(moved.ulid).toBe(parent.ulid);
  expect(hh.show(parent.id).ulid).toBe(parent.ulid);
  expect((await hh.get<{ store: string }>(`/api/tasks/${parent.ulid}`)).store).toBe("second");
  await expect(page).toHaveURL(new RegExp(`/tasks/${parent.ulid}$`));
  await expect(page.getByRole("textbox", { name: "タイトル" })).toHaveValue("Parent task");
});

test("takes the plan again when a task is added under the ones to move", async ({ page, hh }) => {
  const { parent, child } = fixture(hh);
  await openFromDetail(page, hh, parent.ulid);
  const d = dialog(page);
  await d.getByRole("radio", { name: /^second/ }).check();
  await d.getByRole("combobox", { name: "second のワークスペース" }).selectOption("a");
  await expect(d).toContainText("子孫 1");

  hh.hh("add", "Late task", "--parent", child.id);
  await d.getByRole("button", { name: /移動/ }).last().click();
  await expect(d.getByRole("alert")).toContainText(
    "内容が変わりました。確かめてから移動してください",
  );
  await expect(d).toContainText("子孫 2");
  await expect(dialog(page)).toBeVisible();

  // The plan as it is now goes through.
  await d.getByRole("button", { name: /移動/ }).last().click();
  await expect(toast(page, "second に移動しました")).toBeVisible();
  expect(hh.show("SC-1").ulid).toBe(parent.ulid);
});

test("puts the task under a parent of the destination", async ({ page, hh }) => {
  const { child } = fixture(hh);
  const target = hh.json<{ id: string; ulid: string }>(
    "add",
    "Target parent",
    "--store",
    "second",
    "--workspace",
    "a",
  );
  await openFromDetail(page, hh, child.ulid);
  const d = dialog(page);
  await d.getByRole("radio", { name: /^second/ }).check();
  await d.getByRole("combobox", { name: "親を選ぶ" }).fill("Target");
  await d.getByRole("option", { name: new RegExp(target.id) }).click();
  await expect(d.getByRole("region", { name: "移動先で変わること" })).toContainText(
    `${target.id} Target parent の下に置きます`,
  );
  // the parent chose the workspace
  await expect(d.getByRole("combobox", { name: "second のワークスペース" })).toHaveValue("a");
  await d.getByRole("button", { name: "移動", exact: true }).click();
  await expect(toast(page, "second に移動しました")).toBeVisible();
  expect((await hh.get<{ parent: string | null }>(`/api/tasks/${child.ulid}`)).parent).toBe(
    target.ulid,
  );
});

test("a Store that cannot be written is not a place to move to", async ({ page, hh }) => {
  const { parent } = fixture(hh);
  hh.readOnlyStore("ro", "RO");
  await openFromDetail(page, hh, parent.ulid);
  const ro = dialog(page).getByRole("radio", { name: /^ro/ });
  await expect(ro).toBeDisabled();
  await expect(ro).toHaveAccessibleDescription(
    "閲覧のみのため移動できません（remote に書き込めません）",
  );
  await expect(dialog(page).getByRole("radio", { name: /^second/ })).toBeEnabled();
});

test("opens from the menu of a row of the tree and from the palette", async ({ page, hh }) => {
  fixture(hh);
  await hh.open(page, "/query/tree?q=");
  await page.getByRole("button", { name: "メニュー: Parent task" }).click();
  await page.getByRole("menuitem", { name: "別の Store へ移動…" }).click();
  await expect(dialog(page)).toBeVisible();
  await dialog(page).getByRole("button", { name: "キャンセル" }).click();
  await expect(dialog(page)).toHaveCount(0);

  await row(page, "Parent task").click();
  await page.keyboard.press("ControlOrMeta+k");
  const palette = page.getByRole("dialog", { name: "ID・検索・コマンド" });
  await palette.getByRole("combobox").fill(">別の Store");
  await palette.getByRole("option", { name: /別の Store へ移動/ }).click();
  await expect(dialog(page)).toBeVisible();
});

test("has no entry with one Store", async ({ page, hh }) => {
  const t = hh.add("Only task");
  await hh.open(page, `/tasks/${t.ulid}`);
  await page.getByRole("button", { name: "その他の操作" }).click();
  await expect(page.getByRole("menuitem", { name: /別のワークスペースへ移動/ })).toBeVisible();
  await expect(page.getByRole("menuitem", { name: /別の Store へ移動/ })).toHaveCount(0);
});
