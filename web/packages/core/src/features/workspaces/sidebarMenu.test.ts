import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { fakeWorkspacesPort } from "./fixtures.test-util";
import { canMoveWorkspace, createWorkspaceMover, deleteMenuItem } from "./sidebarMenu";

async function setup() {
  const catalog = createCatalog({
    ...fakeCatalogPort(),
    listWorkspaces: async (storeId) => [
      { id: `${storeId}-W1`, storeId, name: "Web", prefix: "HH" },
      { id: `${storeId}-W2`, storeId, name: "Docs", prefix: "DOC" },
    ],
  });
  await catalog.load();
  const port = fakeWorkspacesPort();
  const toasts = createToasts({ set: () => 0, clear: () => undefined });
  return { catalog, port, toasts, mover: createWorkspaceMover({ catalog, port, toasts }) };
}

describe("workspace menu", () => {
  it("disables 上へ at the top, 下へ at the bottom, and both in a read-only store", async () => {
    const { catalog } = await setup();
    const work = catalog.store("S1");
    const ro = catalog.store("S2");
    if (!work || !ro) throw new Error("fixture");
    expect(canMoveWorkspace(work, "S1-W1", "up")).toBe(false);
    expect(canMoveWorkspace(work, "S1-W1", "down")).toBe(true);
    expect(canMoveWorkspace(work, "S1-W2", "up")).toBe(true);
    expect(canMoveWorkspace(work, "S1-W2", "down")).toBe(false);
    expect(canMoveWorkspace(ro, "S2-W1", "down")).toBe(false);
  });

  it("moves without a toast", async () => {
    const { port, toasts, mover } = await setup();
    await mover.move("S1", "S1-W1", "down");
    expect(port.calls).toEqual(["move"]);
    expect(get(toasts.list)).toEqual([]);
  });

  it("shows a refusal as an error toast", async () => {
    const { port, toasts, mover } = await setup();
    port.refuseNext("並びを変えられませんでした");
    await mover.move("S1", "S1-W1", "down");
    expect(get(toasts.list).map((t) => [t.text, t.error])).toEqual([
      ["並びを変えられませんでした", true],
    ]);
  });
});

describe("workspace menu delete item", () => {
  it("is 削除… and disabled with a hint in a read-only store", async () => {
    const { catalog } = await setup();
    const work = catalog.store("S1");
    const ro = catalog.store("S2");
    if (!work || !ro) throw new Error("fixture");
    expect(deleteMenuItem(work)).toEqual({ id: "delete", label: "削除…", disabled: false });
    expect(deleteMenuItem(ro)).toEqual({
      id: "delete",
      label: "削除…",
      disabled: true,
      hint: "閲覧のみの Store です",
    });
  });
});
