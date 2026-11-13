import { describe, expect, it } from "vitest";
import { NotFound } from "../../model/errors";
import type { Item } from "../../model/item";
import { get, writable } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { flush, gate, item } from "../items/fixtures.test-util";
import type { Route } from "../router/route";
import { createToasts } from "../toasts/toasts";
import {
  createWorkspaceDelete,
  type DeleteWorkspaceTarget,
  leaveDeletedWorkspace,
  nameMismatch,
  workspaceDeleteDialog,
  workspaceDeletedMessage,
} from "./delete";
import { fakeWorkspacesPort } from "./fixtures.test-util";

const web: DeleteWorkspaceTarget = { storeId: "S1", id: "S1-W1", name: "Web", prefix: "HH" };
const ro: DeleteWorkspaceTarget = { storeId: "S2", id: "S2-W1", name: "Web", prefix: "HH" };
const preview = { projects: 2, items: 5, retentionDays: 30 };

async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const actions = fakeWorkspacesPort();
  actions.workspaceDeletePreview = async () => {
    actions.calls.push("workspaceDeletePreview");
    return preview;
  };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const after: string[] = [];
  const del = createWorkspaceDelete({
    actions,
    catalog,
    toasts,
    afterDelete: async (w) => void after.push(w.id),
  });
  const messages = () => get(toasts.list).map((t) => `${t.error ? "!" : ""}${t.text}`);
  return { del, actions, after, messages };
}

describe("workspaceDeleteDialog", () => {
  it("writes the title, the body, the label and the button", () => {
    expect(workspaceDeleteDialog(web, preview)).toEqual({
      workspace: web,
      title: "ワークスペースを削除",
      lines: [
        "「Web」と、その中の 2 件のプロジェクト・5 件のアイテム、スプリント・リリース・設定を削除します。",
        "設定の「削除済み」から戻せます。30 日を過ぎると完全に削除されます。",
      ],
      label: "確認のため、ワークスペースの名前を入力してください",
      ok: "削除",
    });
    expect(workspaceDeletedMessage("Web")).toBe("「Web」を削除しました");
  });
});

describe("createWorkspaceDelete", () => {
  it("opens the dialog after reading the preview", async () => {
    const { del } = await setup();
    await del.open(web);
    expect(get(del.state)).toMatchObject({ title: "ワークスペースを削除", error: null });
  });

  it("does nothing in a read-only Store", async () => {
    const { del, actions } = await setup();
    await del.open(ro);
    expect(get(del.state)).toBeNull();
    expect(actions.calls).toEqual([]);
  });

  it("does not delete on a mismatched name and shows the error", async () => {
    const { del, actions, after } = await setup();
    await del.open(web);
    await del.confirm("Wb");
    expect(actions.calls).toEqual(["workspaceDeletePreview"]);
    expect(get(del.state)?.error).toBe("名前が一致しません");
    expect(nameMismatch).toBe("名前が一致しません");
    expect(after).toEqual([]);
  });

  it("does not delete when only the case differs", async () => {
    const { del, actions } = await setup();
    await del.open(web);
    await del.confirm("web");
    expect(actions.calls).toEqual(["workspaceDeletePreview"]);
    expect(get(del.state)?.error).toBe("名前が一致しません");
  });

  it("clears the error when the input changes", async () => {
    const { del } = await setup();
    await del.open(web);
    await del.confirm("Wb");
    del.clearError();
    expect(get(del.state)?.error).toBeNull();
  });

  it("deletes on a matching name ignoring surrounding spaces", async () => {
    const { del, actions, after, messages } = await setup();
    await del.open(web);
    await del.confirm(" Web ");
    expect(actions.calls).toEqual(["workspaceDeletePreview", "deleteWorkspace"]);
    expect(get(del.state)).toBeNull();
    expect(messages()).toEqual(["「Web」を削除しました"]);
    expect(after).toEqual([web.id]);
  });

  it("does nothing on confirm or cancel without a dialog", async () => {
    const { del, actions } = await setup();
    await del.confirm("Web");
    del.cancel();
    expect(actions.calls).toEqual([]);
  });

  it("drops a preview that arrives after cancel", async () => {
    const { del, actions } = await setup();
    const g = gate();
    actions.workspaceDeletePreview = async () => {
      await g.wait;
      return preview;
    };
    const opening = del.open(web);
    await flush();
    del.cancel();
    g.release();
    await opening;
    expect(get(del.state)).toBeNull();
  });

  it("sends one delete for a double confirm", async () => {
    const { del, actions } = await setup();
    await del.open(web);
    const g = gate();
    actions.deleteWorkspace = async () => {
      actions.calls.push("deleteWorkspace");
      await g.wait;
    };
    const first = del.confirm("Web");
    await flush();
    await del.confirm("Web");
    g.release();
    await first;
    expect(actions.calls.filter((c) => c === "deleteWorkspace")).toHaveLength(1);
  });

  it("leaves without a dialog when the preview is NotFound", async () => {
    const { del, actions, after, messages } = await setup();
    actions.workspaceDeletePreview = async () => {
      throw new NotFound("gone");
    };
    await del.open(web);
    expect(get(del.state)).toBeNull();
    expect(messages()).toEqual([]);
    expect(after).toEqual([web.id]);
  });

  it("leaves without a toast when the delete is NotFound", async () => {
    const { del, actions, after, messages } = await setup();
    await del.open(web);
    actions.deleteWorkspace = async () => {
      throw new NotFound("gone");
    };
    await del.confirm("Web");
    expect(get(del.state)).toBeNull();
    expect(messages()).toEqual([]);
    expect(after).toEqual([web.id]);
  });

  it("shows other errors as an error toast and does not leave", async () => {
    const { del, actions, after, messages } = await setup();
    actions.workspaceDeletePreview = async () => {
      throw new Error("network");
    };
    await del.open(web);
    expect(messages()).toEqual(["!削除できませんでした"]);
    actions.workspaceDeletePreview = async () => preview;
    await del.open(web);
    actions.refuseNext("削除できません");
    await del.confirm("Web");
    expect(messages()).toEqual(["!削除できませんでした", "!削除できません"]);
    expect(get(del.state)).toBeNull();
    expect(after).toEqual([]);
  });
});

describe("leaveDeletedWorkspace", () => {
  async function leaveSetup(route: Route, opened: Item | null = null) {
    const catalog = createCatalog(fakeCatalogPort());
    await catalog.load();
    const replaced: Route[] = [];
    const events: string[] = [];
    let closed = 0;
    const load = catalog.load;
    const deps = {
      navigation: {
        route: writable<Route>(route),
        replace: (r: Route) => {
          events.push("replace");
          replaced.push(r);
        },
      },
      catalog: {
        stores: catalog.stores,
        load: async () => {
          events.push("load");
          await load();
        },
      },
      detail: {
        view: writable(opened ? { item: opened } : null),
        close: () => {
          closed += 1;
        },
      },
    };
    return { deps, replaced, events, closed: () => closed };
  }

  it.each<Route>([
    { name: "workspace", store: "work", prefix: "HH", tab: "settings" },
    { name: "workspace", store: "work", prefix: "HH", tab: "overview" },
    { name: "project", store: "work", project: "HH-P1", tab: "overview" },
    { name: "item", store: "work", item: "HH-12" },
  ])("replaces to home first, then reloads, from %j", async (route) => {
    const { deps, replaced, events } = await leaveSetup(route);
    await leaveDeletedWorkspace(deps, web);
    expect(replaced).toEqual([{ name: "home" }]);
    expect(events).toEqual(["replace", "load"]);
  });

  it.each<Route>([
    { name: "workspace", store: "work", prefix: "WEB", tab: "overview" },
    { name: "project", store: "work", project: "HHX-P1", tab: "overview" },
    { name: "workspace", store: "ro", prefix: "HH", tab: "overview" },
    { name: "home" },
  ])("does not move from %j but reloads", async (route) => {
    const { deps, replaced, events } = await leaveSetup(route);
    await leaveDeletedWorkspace(deps, web);
    expect(replaced).toEqual([]);
    expect(events).toEqual(["load"]);
  });

  it("closes the detail panel only for an item of the workspace", async () => {
    const mine = item("S1", "S1-P1", 1, "Task", null);
    const a = await leaveSetup({ name: "home" }, mine);
    await leaveDeletedWorkspace(a.deps, web);
    expect(a.closed()).toBe(1);
    const b = await leaveSetup({ name: "home" }, { ...mine, workspaceId: "S1-W2" });
    await leaveDeletedWorkspace(b.deps, web);
    expect(b.closed()).toBe(0);
  });
});
