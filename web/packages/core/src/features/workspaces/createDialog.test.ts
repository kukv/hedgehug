import { describe, expect, it } from "vitest";
import type { CatalogPort } from "../../port/catalog";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createWorkspaceDialog } from "./createDialog";
import { fakeWorkspacesPort } from "./fixtures.test-util";

async function setup(override: Partial<CatalogPort> = {}) {
  const catalog = createCatalog({ ...fakeCatalogPort(), ...override });
  await catalog.load();
  const port = fakeWorkspacesPort();
  const created: [string, string][] = [];
  const dialog = createWorkspaceDialog({
    catalog,
    port,
    onCreated: (storeId, w) => void created.push([storeId, w.name]),
  });
  return { dialog, port, created };
}

describe("workspace dialog", () => {
  it("opens on the store of the menu and upper-cases the prefix", async () => {
    const { dialog } = await setup();
    dialog.open("S1");
    dialog.update({ prefix: "web" });
    expect(get(dialog.state)?.form).toEqual({ storeId: "S1", name: "", prefix: "WEB" });
    expect(get(dialog.state)?.stores.map((s) => s.value)).not.toContain("S2"); // 閲覧のみ
  });

  it("falls back to the first writable store", async () => {
    const { dialog } = await setup();
    dialog.open("S2");
    expect(get(dialog.state)?.form.storeId).toBe("S1");
  });

  it("shows the refusal inside the dialog", async () => {
    const { dialog, port } = await setup();
    dialog.open("S1");
    port.refuseNext("この Store には同じ接頭辞のワークスペースがあります");
    await dialog.submit();
    expect(get(dialog.state)?.error).toBe("この Store には同じ接頭辞のワークスペースがあります");
  });

  it("reloads the sidebar and opens the overview after creating", async () => {
    const { dialog, created } = await setup();
    dialog.open("S1");
    dialog.update({ name: "Docs", prefix: "DOC" });
    await dialog.submit();
    expect(get(dialog.state)).toBeNull();
    expect(created).toEqual([["S1", "Docs"]]);
  });

  it("tells why it cannot add a workspace", async () => {
    const none = await setup({ listStores: async () => [] });
    none.dialog.open();
    expect(get(none.dialog.state)?.blocked).toBe("先に Store を追加してください");
    const readOnly = await setup({
      listStores: async () => [{ id: "S2", name: "ro", color: "orange", readOnly: true }],
    });
    readOnly.dialog.open();
    expect(get(readOnly.dialog.state)?.blocked).toBe(
      "ワークスペースを追加できる Store がありません",
    );
  });
});
