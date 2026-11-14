import { describe, expect, it, vi } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { fakeWorkspacesPort } from "./fixtures.test-util";
import { createWorkspaceSettings, workspaceSettingsTabs } from "./settings";

async function setup() {
  const catalogPort = fakeCatalogPort();
  let loads = 0;
  const countingPort = {
    ...catalogPort,
    listStores: async () => {
      loads += 1;
      return catalogPort.listStores();
    },
  };
  const catalog = createCatalog(countingPort);
  await catalog.load();
  const port = fakeWorkspacesPort();
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const screen = createWorkspaceSettings({
    port,
    catalog,
    toasts,
    listMembers: async () => [{ login: "taro", name: "田中" }],
  });
  return { screen, port, toasts, loads: () => loads };
}

describe("workspace settings", () => {
  it("has the eight sub tabs in the order of spec 03", () => {
    expect(workspaceSettingsTabs.map((t) => t.label)).toEqual([
      "一般",
      "プロパティ",
      "ステータス",
      "優先度",
      "関連システム",
      "ワークフロー",
      "テンプレート",
      "自動化",
    ]);
  });

  it("loads the store's members with the settings", async () => {
    const { screen } = await setup();
    await screen.open("S1", "S1-W1");
    expect(get(screen.state)?.members).toEqual([{ login: "taro", name: "田中" }]);
  });

  it("reloads without clearing the state or the sub tab when the same workspace is opened again", async () => {
    const { screen, port } = await setup();
    await screen.open("S1", "S1-W1");
    screen.selectTab("statuses");
    const reads = vi.spyOn(port, "get");
    const opening = screen.open("S1", "S1-W1");
    expect(get(screen.state)).not.toBeNull(); // 読み直しの間も消さない
    await opening;
    expect(reads).toHaveBeenCalledTimes(1);
    expect(get(screen.state)?.tab).toBe("statuses");
  });

  it("reads the automation only while the 自動化 sub tab is open", async () => {
    const { screen, port } = await setup();
    await screen.open("S1", "S1-W1");
    expect(get(screen.state)?.automation).toBeNull();
    expect(port.calls).not.toContain("automation");
    screen.selectTab("automation");
    await vi.waitFor(() =>
      expect(get(screen.state)?.automation).toEqual({ rules: [], recent: [] }),
    );
    expect(port.calls.filter((c) => c === "automation")).toHaveLength(1);
    screen.selectTab("statuses");
    await screen.reload();
    expect(get(screen.state)?.automation).toBeNull();
    expect(port.calls.filter((c) => c === "automation")).toHaveLength(1);
  });

  it("opens on 一般 and keeps the sub tab while the workspace stays", async () => {
    const { screen } = await setup();
    await screen.open("S1", "S1-W1");
    screen.selectTab("statuses");
    await screen.open("S1", "S1-W1");
    expect(get(screen.state)?.tab).toBe("statuses");
    await screen.open("S1", "S1-W2");
    expect(get(screen.state)?.tab).toBe("general");
  });

  it("opens on the sub tab handed as the section, reading the automation once (C4a-15)", async () => {
    const { screen, port } = await setup();
    await screen.open("S1", "S1-W1");
    await screen.open("S1", "S1-W2", "automation");
    expect(get(screen.state)?.tab).toBe("automation");
    expect(get(screen.state)?.automation).toEqual({ rules: [], recent: [] });
    expect(port.calls.filter((c) => c === "automation")).toHaveLength(1);
  });

  it("ignores a section that is not a sub tab", async () => {
    const { screen } = await setup();
    await screen.open("S1", "S1-W1", "rules-templates");
    expect(get(screen.state)?.tab).toBe("general");
  });

  it("shows a refusal as an error toast and reloads", async () => {
    const { screen, port, toasts } = await setup();
    await screen.open("S1", "S1-W1");
    port.refuseNext("名前は空にできません");
    const ok = await screen.run((p, s, w) => p.rename(s, w, ""));
    expect(ok).toBe(false);
    expect(get(toasts.list).map((t) => [t.text, t.error])).toEqual([
      ["名前は空にできません", true],
    ]);
    expect(port.calls.filter((c) => c === "get")).toHaveLength(2); // 開いたときと、断りのあと
  });

  it("hands a refusal to onRefused instead of the toast, for refusals shown in a dialog", async () => {
    const { screen, port, toasts } = await setup();
    await screen.open("S1", "S1-W1");
    port.refuseNext("同じ名前のプロパティがあります");
    const messages: string[] = [];
    const ok = await screen.run(
      (p, s, w) => p.rename(s, w, "x"),
      undefined,
      (m) => messages.push(m),
    );
    expect([ok, messages, get(toasts.list)]).toEqual([
      false,
      ["同じ名前のプロパティがあります"],
      [],
    ]);
  });

  it("shows the success toast only when given", async () => {
    const { screen, toasts } = await setup();
    await screen.open("S1", "S1-W1");
    await screen.run((p, s, w) => p.rename(s, w, "フロント"));
    await screen.run((p, s, w) => p.changePrefix(s, w, "WEB"), "接頭辞を変えました");
    expect(get(toasts.list).map((t) => t.text)).toEqual(["接頭辞を変えました"]);
  });

  it("reloads the catalog after a rename, prefix change or move, and only then", async () => {
    const { screen, loads } = await setup();
    await screen.open("S1", "S1-W1");
    expect(loads()).toBe(1);
    await screen.run((p, s, w) => p.addValue(s, w, "statuses", "レビュー"));
    expect(loads()).toBe(1);
    await screen.run((p, s, w) => p.rename(s, w, "フロント"));
    expect(loads()).toBe(2);
    await screen.run((p, s, w) => p.changePrefix(s, w, "WEB"));
    expect(loads()).toBe(3);
    await screen.run((p, s, w) => p.move(s, w, "down"));
    expect(loads()).toBe(4);
  });

  it("clears the state while another workspace loads", async () => {
    const { screen } = await setup();
    await screen.open("S1", "S1-W1");
    const opening = screen.open("S1", "S1-W2");
    expect(get(screen.state)).toBeNull();
    await opening;
    expect(get(screen.state)?.detail.id).toBe("S1-W2");
  });

  it("reloads the settings even when the catalog load throws", async () => {
    const catalogPort = fakeCatalogPort();
    const catalog = createCatalog(catalogPort);
    await catalog.load();
    const port = fakeWorkspacesPort();
    const toasts = createToasts({ set: () => 0, clear: () => {} });
    const screen = createWorkspaceSettings({ port, catalog, toasts, listMembers: async () => [] });
    await screen.open("S1", "S1-W1");
    catalogPort.listStores = async () => {
      throw new Error("down");
    };
    const before = port.calls.filter((c) => c === "get").length;
    await expect(screen.run((p, s, w) => p.rename(s, w, "フロント"))).rejects.toThrow("down");
    expect(port.calls.filter((c) => c === "get").length).toBe(before + 1);
  });

  it("is read only on a read-only store", async () => {
    const { screen } = await setup();
    await screen.open("S2", "S2-W1");
    expect(get(screen.state)?.readOnly).toBe(true);
    await screen.open("S1", "S1-W1");
    expect(get(screen.state)?.readOnly).toBe(false);
  });

  it("shows the refusal as an error toast before a dialog opens", async () => {
    const catalogPort = fakeCatalogPort();
    const two = {
      ...catalogPort,
      listWorkspaces: async (storeId: string) => [
        { id: `${storeId}-W1`, storeId, name: "Web", prefix: "HH" },
        { id: `${storeId}-W2`, storeId, name: "API", prefix: "API" },
      ],
    };
    const catalog = createCatalog(two);
    await catalog.load();
    const toasts = createToasts({ set: () => 0, clear: () => {} });
    const screen = createWorkspaceSettings({
      port: fakeWorkspacesPort(),
      catalog,
      toasts,
      listMembers: async () => [],
    });
    await screen.open("S1", "S1-W1");
    expect(screen.askPrefix("WEB")).toBe(true);
    expect(screen.askPrefix("API")).toBe(false);
    expect(screen.askPrefix("HH")).toBe(true); // 自分の接頭辞は重ならない
    expect(screen.askDelete("statuses", "st1")).toBe(false); // 初期かつ未着手の既定
    expect(screen.askDelete("statuses", "st3")).toBe(true);
    expect(get(toasts.list).map((t) => [t.text, t.error])).toEqual([
      ["この Store には同じ接頭辞のワークスペースがあります", true],
      [
        "「未着手」は初期ステータスで、未着手の既定のステータスでもあります。先に別のステータスを、初期ステータスと未着手の既定にしてください",
        true,
      ],
    ]);
  });
});
