import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { fakeWorkspacesPort } from "./fixtures.test-util";
import { createWorkspaceSettings, settingsTabs } from "./settings";

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
  const screen = createWorkspaceSettings({ port, catalog, toasts });
  return { screen, port, toasts, loads: () => loads };
}

describe("workspace settings", () => {
  it("has the eight sub tabs in the order of spec 03", () => {
    expect(settingsTabs.map((t) => t.label)).toEqual([
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

  it("opens on 一般 and keeps the sub tab while the workspace stays", async () => {
    const { screen } = await setup();
    await screen.open("S1", "S1-W1");
    screen.selectTab("statuses");
    await screen.open("S1", "S1-W1");
    expect(get(screen.state)?.tab).toBe("statuses");
    await screen.open("S1", "S1-W2");
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
    const screen = createWorkspaceSettings({ port, catalog, toasts });
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
});
