// Screens this spec relies on (see screens.ts for the shared list):
//   - /settings/workspaces/<store> lists the workspaces of the Store in a table (one row each, with
//     the prefix, the projects, the tasks and the day the prefix was last changed) and has the button "ワークスペースを追加", which opens
//     the dialog named "<store> にワークスペースを追加" (the textboxes "名前" and "ID の接頭辞", the
//     buttons "追加" and "キャンセル")
//   - /workspaces/<store>/<ulid>/settings has the textbox "名前" with the button "保存", the prefix
//     with the button "接頭辞を変更…" (dialog "<name> の接頭辞を変更": the textbox "新しい接頭辞", the
//     button "<n> 件を振り直す"), and "削除" (dialog "ワークスペース <name> を削除", button "削除")
//   - the detail of a task has the button "その他の操作", whose menu has "別のワークスペースへ移動…"; a
//     row of the tree has the button "メニュー: <title>" with the same item. Both open the dialog
//     "<ID> <title> を移動": the radio group "移動先のワークスペース" (a radio is named
//     "<name>（<prefix>）", the current workspace adds " — 今のワークスペース" and is disabled) and the
//     button "移動"
//   - the toasts say: "ワークスペース <name>（<prefix>）を <store> に追加しました",
//     "ワークスペースの名前を <a> から <b> に変えました",
//     "<name> の接頭辞を <a> から <b> に変えました（<n> 件の ID を振り直しました）",
//     "ワークスペース <name> を削除しました", "<ID> を <name> に移しました（今の ID は <ID>）"
import { expect, type Hedgehug, test } from "./harness";
import { row, toast } from "./screens";

type Ws = { ulid: string; name: string; id_prefix: string; tasks: number };

function workspaceOf(hh: Hedgehug, name: string): Ws {
  const w = hh.json<Ws[]>("workspace", "ls").find((x) => x.name === name);
  if (!w) throw new Error(`no workspace ${name}`);
  return w;
}

const names = (hh: Hedgehug) => hh.json<Ws[]>("workspace", "ls").map((w) => w.name);

test("the settings list adds a workspace and refuses the names and prefixes that are taken", async ({
  page,
  hh,
}) => {
  await hh.open(page, "/settings/workspaces/demo");
  await expect(page.getByRole("row", { name: /demo/ })).toContainText("DM");

  await page.getByRole("button", { name: "ワークスペースを追加" }).click();
  const dialog = page.getByRole("dialog", { name: "demo にワークスペースを追加" });
  const name = dialog.getByRole("textbox", { name: "名前" });
  const prefix = dialog.getByRole("textbox", { name: "ID の接頭辞" });
  const add = dialog.getByRole("button", { name: "追加", exact: true });
  await expect(add).toBeDisabled();

  await name.fill("Ops");
  await expect(dialog).toContainText("名前は小文字の英数字と - _ で、英数字から始めてください");
  await name.fill("demo");
  await expect(dialog).toContainText("demo は同じ Store のワークスペースが使っています");
  await name.fill("ops");
  await prefix.fill("dm"); // typed in lower case: the field turns it into capitals
  await expect(prefix).toHaveValue("DM");
  await expect(dialog).toContainText("DM は同じ Store のワークスペース demo が使っています");
  await expect(add).toBeDisabled();

  await prefix.fill("OPS");
  await expect(dialog.getByRole("button", { name: /コマンドをコピー/ })).toContainText(
    "hh workspace add ops --prefix OPS --store demo",
  );
  await expect(add).toBeEnabled();
  await add.click();

  await expect(toast(page, "ワークスペース ops（OPS）を demo に追加しました")).toBeVisible();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("row", { name: /ops/ })).toContainText("OPS");
  expect(names(hh)).toEqual(["demo", "ops"]);
});

test("the settings tab renames the workspace, and refuses a name another workspace has", async ({
  page,
  hh,
}) => {
  hh.hh("workspace", "add", "other", "--prefix", "OT");
  const demo = workspaceOf(hh, "demo");
  await hh.open(page, `/workspaces/demo/${demo.ulid}/settings`);

  const name = page.getByRole("textbox", { name: "名前" });
  const save = page.getByRole("button", { name: "保存" });
  await expect(name).toHaveValue("demo");
  await expect(save).toBeDisabled();
  await expect(page.getByText("タスクの ID は変わりません")).toBeVisible();

  await name.fill("other");
  await expect(page.getByText("other は同じ Store のワークスペースが使っています")).toBeVisible();
  await expect(save).toBeDisabled();

  await name.fill("platform");
  await expect(
    page.getByText("名前だけが変わります。接頭辞 DM とタスクの ID はそのままです"),
  ).toBeVisible();
  await save.click();

  await expect(toast(page, "ワークスペースの名前を demo から platform に変えました")).toBeVisible();
  await expect(save).toBeDisabled();
  expect(names(hh)).toEqual(["other", "platform"]);
});

test("changing the prefix renumbers the tasks, and the old IDs still find them", async ({
  page,
  hh,
}) => {
  const first = hh.add("First");
  const second = hh.add("Second");
  const demo = workspaceOf(hh, "demo");
  await hh.open(page, `/workspaces/demo/${demo.ulid}/settings`);

  await page.getByRole("button", { name: "接頭辞を変更…" }).click();
  const dialog = page.getByRole("dialog", { name: "demo の接頭辞を変更" });
  await expect(dialog).toContainText(
    "配下の 2 件（削除済みを含む）の ID を新しい接頭辞で振り直します",
  );
  const field = dialog.getByRole("textbox", { name: "新しい接頭辞" });
  const go = dialog.getByRole("button", { name: "2 件を振り直す" });
  await expect(go).toBeDisabled();

  await field.fill("dm");
  await expect(dialog).toContainText("今と同じ接頭辞です");
  await expect(go).toBeDisabled();
  await field.fill("rls");
  await expect(dialog.getByRole("button", { name: /コマンドをコピー/ })).toContainText(
    "hh workspace set-prefix demo RLS --store demo",
  );
  await go.click();

  await expect(
    toast(page, "demo の接頭辞を DM から RLS に変えました（2 件の ID を振り直しました）"),
  ).toBeVisible();
  await expect(page.getByText("RLS-1, RLS-2 … と振ります")).toBeVisible();
  expect(hh.show(first.ulid).id).toBe("RLS-1");
  expect(hh.show(second.ulid).id).toBe("RLS-2");
  expect(hh.show("DM-1").ulid).toBe(first.ulid); // the old ID still finds the task
});

test("a workspace with tasks, deleted ones included, cannot be removed; an empty one can", async ({
  page,
  hh,
}) => {
  hh.add("Kept");
  const gone = hh.add("Removed");
  hh.hh("rm", gone.id);
  hh.hh("workspace", "add", "sandbox", "--prefix", "SBX");
  const demo = workspaceOf(hh, "demo");
  const sandbox = workspaceOf(hh, "sandbox");

  await hh.open(page, `/workspaces/demo/${demo.ulid}/settings`);
  await expect(
    page.getByText("タスク 2 件（削除済み 1 件を含む）が残っているため削除できません"),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "削除", exact: true })).toBeDisabled();

  await hh.open(page, `/workspaces/demo/${sandbox.ulid}/settings`);
  await page.getByRole("button", { name: "削除", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "ワークスペース sandbox を削除" });
  await expect(dialog).toContainText("にはタスクがありません");
  await dialog.getByRole("button", { name: "削除", exact: true }).click();

  await expect(toast(page, "ワークスペース sandbox を削除しました")).toBeVisible();
  await expect(dialog).toBeHidden();
  await expect(page).toHaveURL(`${hh.origin}/`); // the screen of the workspace that is gone is left
  expect(names(hh)).toEqual(["demo"]);
});

test("a task moves to another workspace from the … menu of its detail", async ({ page, hh }) => {
  const t = hh.add("Move me");
  hh.hh("workspace", "add", "ops", "--prefix", "OPS");
  await hh.open(page, `/tasks/${t.ulid}`);

  await page.getByRole("button", { name: "その他の操作" }).click();
  await page.getByRole("menuitem", { name: /別のワークスペースへ移動/ }).click();
  const dialog = page.getByRole("dialog", { name: /Move me を移動$/ });
  await expect(dialog.getByRole("radio", { name: /^demo/ })).toBeDisabled();
  await expect(dialog.getByRole("radio", { name: "ops（OPS）" })).toBeChecked();
  await expect(dialog).toContainText(
    "ID を OPS の番号に振り直します（DM-1 は旧 ID として残ります）",
  );
  await expect(dialog.getByRole("button", { name: /コマンドをコピー/ })).toContainText(
    "hh mv DM-1 --workspace ops",
  );
  await dialog.getByRole("button", { name: "移動", exact: true }).click();

  await expect(toast(page, "DM-1 を ops に移しました（今の ID は OPS-1）")).toBeVisible();
  const moved = hh.json<{ id: string; former_ids: string[]; workspace: { name: string } }>(
    "show",
    t.ulid,
  );
  expect(moved.id).toBe("OPS-1");
  expect(moved.workspace.name).toBe("ops");
  expect(moved.former_ids).toContain("DM-1");
  await expect(page.getByRole("navigation", { name: "パンくず" })).toContainText("ops");
});

test("a task moves to another workspace from the … menu of a row of the tree", async ({
  page,
  hh,
}) => {
  const t = hh.add("Move from the tree");
  hh.hh("workspace", "add", "ops", "--prefix", "OPS");
  const demo = workspaceOf(hh, "demo");
  await hh.open(page, `/workspaces/demo/${demo.ulid}/tree`);

  await row(page, "Move from the tree").click();
  await page.getByRole("button", { name: "メニュー: Move from the tree" }).click();
  await page.getByRole("menuitem", { name: /別のワークスペースへ移動/ }).click();
  const dialog = page.getByRole("dialog", { name: /Move from the tree を移動$/ });
  await dialog.getByRole("button", { name: "移動", exact: true }).click();

  await expect(toast(page, "DM-1 を ops に移しました（今の ID は OPS-1）")).toBeVisible();
  await expect(row(page, "Move from the tree")).toBeHidden(); // it left this workspace
  expect(hh.json<{ workspace: { name: string } }>("show", t.ulid).workspace.name).toBe("ops");
});
