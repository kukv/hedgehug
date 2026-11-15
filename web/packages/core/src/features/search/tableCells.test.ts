import { describe, expect, it, vi } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import type { PlanningOverview } from "../../model/planning";
import { get } from "../../reactive/store";
import { settings } from "../items/settings.test-util";
import { sprintSaveFailed } from "../planning/sprintDialogs";
import { createTableCells } from "./tableCells";
import { emptyValues, type SearchRow } from "./types";

const row = (over: Partial<SearchRow> = {}) =>
  ({ storeId: "s1", workspaceId: "w1", itemId: "i1", values: emptyValues, ...over }) as SearchRow;

const plan = {
  sprints: [{ id: "sp1", name: "S1", state: "active" }],
  releases: [{ id: "rl1", name: "R1", state: "open" }],
} as unknown as PlanningOverview;

function setup(read: () => Promise<unknown> = async () => ({ settings })) {
  const deps = {
    items: {
      setProperty: vi.fn(async (..._a: unknown[]) => ({}) as never),
      addValue: vi.fn(async (..._a: unknown[]) => ({}) as never),
      removeValue: vi.fn(async (..._a: unknown[]) => ({}) as never),
    },
    planning: {
      overview: vi.fn(async () => plan),
      setItemSprint: vi.fn(async (..._a: unknown[]) => {}),
      setItemRelease: vi.fn(async (..._a: unknown[]) => {}),
    },
    workspaces: { get: vi.fn(read) },
    catalogPort: { listMembers: vi.fn(async () => []), me: vi.fn(async () => "") },
    toasts: { show: vi.fn() },
    changed: vi.fn(async () => {}),
  };
  const cells = createTableCells(deps as never);
  return { deps, cells, reload: vi.fn(async () => {}) };
}
const flush = async () => {
  for (let i = 0; i < 20; i++) await Promise.resolve();
};

describe("createTableCells loading", () => {
  it("loads each workspace once", async () => {
    const { deps, cells } = setup();
    cells.load([row(), row({ itemId: "i2" }), row({ workspaceId: "w2" })]);
    await flush();
    expect(deps.workspaces.get).toHaveBeenCalledTimes(2);
    expect(cells.context("s1", "w1")?.settings).toBe(settings);
    expect(cells.context("s1", "w3")).toBeUndefined();
    expect(get(cells.loaded)).toBe(2);
  });

  it("keeps the old context until reloaded", async () => {
    const gates: (() => void)[] = [];
    const { cells } = setup(
      () => new Promise((res) => gates.push(() => res({ settings: { ...settings } }))),
    );
    cells.load([row()]);
    gates[0]?.();
    await flush();
    const old = cells.context("s1", "w1");
    expect(old).toBeDefined();
    cells.reset();
    cells.load([row()]);
    expect(cells.context("s1", "w1")).toBe(old);
    gates[1]?.();
    await flush();
    expect(cells.context("s1", "w1")).not.toBe(old);
    expect(cells.context("s1", "w1")).toBeDefined();
  });

  it("drops a result started before reset", async () => {
    const gates: (() => void)[] = [];
    const { cells } = setup(() => new Promise((res) => gates.push(() => res({ settings }))));
    cells.load([row()]);
    cells.reset();
    gates[0]?.();
    await flush();
    expect(cells.context("s1", "w1")).toBeUndefined();
  });

  it("does not read again after unload", async () => {
    const { deps, cells } = setup();
    const a = [row()];
    cells.load(a);
    await flush();
    cells.unload(a);
    cells.reset();
    await flush();
    expect(deps.workspaces.get).toHaveBeenCalledTimes(1);
  });

  it("ignores unload of rows replaced by a newer load", async () => {
    const { deps, cells } = setup();
    const a = [row()];
    const b = [row({ workspaceId: "w2" })];
    cells.load(a);
    cells.load(b);
    await flush();
    cells.unload(a);
    cells.reset();
    await flush();
    expect(deps.workspaces.get).toHaveBeenCalledTimes(3);
    expect(deps.workspaces.get).toHaveBeenLastCalledWith("s1", "w2");
  });

  it("reads the last loaded rows again on reset", async () => {
    const { deps, cells } = setup();
    cells.load([row(), row({ workspaceId: "w2" })]);
    await flush();
    cells.reset();
    await flush();
    expect(deps.workspaces.get).toHaveBeenCalledTimes(4);
    expect(cells.context("s1", "w2")).toBeDefined();
    expect(get(cells.loaded)).toBe(4);
  });

  it("retries a failed workspace", async () => {
    let fail = true;
    const { deps, cells } = setup(async () => {
      if (fail) throw new Error("x");
      return { settings };
    });
    cells.load([row()]);
    await flush();
    expect(cells.context("s1", "w1")).toBeUndefined();
    fail = false;
    cells.load([row()]);
    await flush();
    expect(deps.workspaces.get).toHaveBeenCalledTimes(2);
    expect(cells.context("s1", "w1")).toBeDefined();
  });
});

describe("createTableCells save", () => {
  it("saves a property", async () => {
    const { deps, cells, reload } = setup();
    const ok = await cells.save(
      row(),
      { kind: "property", propertyId: "pri", value: "v2" },
      reload,
    );
    expect(ok).toBe(true);
    expect(deps.items.setProperty).toHaveBeenCalledWith("s1", "i1", "pri", "v2");
    expect(deps.toasts.show).not.toHaveBeenCalled();
    expect(deps.changed).toHaveBeenCalledOnce();
    expect(reload).toHaveBeenCalledOnce();
  });

  it("adds or removes a toggled value", async () => {
    const { deps, cells, reload } = setup();
    await cells.save(row(), { kind: "toggle", propertyId: "sys", valueId: "a", on: true }, reload);
    expect(deps.items.addValue).toHaveBeenCalledWith("s1", "i1", "sys", "a");
    await cells.save(row(), { kind: "toggle", propertyId: "sys", valueId: "a", on: false }, reload);
    expect(deps.items.removeValue).toHaveBeenCalledWith("s1", "i1", "sys", "a");
    expect(deps.changed).toHaveBeenCalledTimes(2);
    expect(reload).toHaveBeenCalledTimes(2);
  });

  it("saves a plan choice, sending null for empty", async () => {
    const { deps, cells, reload } = setup();
    const r = row({ values: { ...emptyValues, sprint: "sp1" } });
    expect(await cells.save(r, { kind: "plan", key: "sprint", id: "" }, reload)).toBe(true);
    expect(deps.planning.setItemSprint).toHaveBeenCalledWith("s1", "i1", null);
    await cells.save(r, { kind: "plan", key: "release", id: "rl1" }, reload);
    expect(deps.planning.setItemRelease).toHaveBeenCalledWith("s1", "i1", "rl1");
  });

  it("skips a plan choice equal to the current one", async () => {
    const { deps, cells, reload } = setup();
    cells.load([row()]);
    await flush();
    const r = row({ values: { ...emptyValues, sprint: "sp1" } });
    expect(await cells.save(r, { kind: "plan", key: "sprint", id: "sp1" }, reload)).toBe(true);
    // 選択肢にない値は「なし」(空)として比べる。
    const gone = row({ values: { ...emptyValues, release: "zz" } });
    expect(await cells.save(gone, { kind: "plan", key: "release", id: "" }, reload)).toBe(true);
    expect(deps.planning.setItemSprint).not.toHaveBeenCalled();
    expect(deps.planning.setItemRelease).not.toHaveBeenCalled();
    expect(deps.changed).not.toHaveBeenCalled();
    expect(reload).not.toHaveBeenCalled();
  });

  it("shows the refusal and still reloads", async () => {
    const { deps, cells, reload } = setup();
    deps.items.setProperty.mockRejectedValueOnce(new Refused("終了日は開始日以降にしてください"));
    const ok = await cells.save(row(), { kind: "property", propertyId: "end", value: "x" }, reload);
    expect(ok).toBe(false);
    expect(deps.toasts.show).toHaveBeenCalledWith("終了日は開始日以降にしてください", {
      error: true,
    });
    expect(deps.changed).toHaveBeenCalledOnce();
    expect(reload).toHaveBeenCalledOnce();
  });

  it("shows the save failure for a not-found item", async () => {
    const { deps, cells, reload } = setup();
    deps.items.setProperty.mockRejectedValueOnce(new NotFound("x"));
    const ok = await cells.save(row(), { kind: "property", propertyId: "p", value: "x" }, reload);
    expect(ok).toBe(false);
    expect(deps.toasts.show).toHaveBeenCalledWith(sprintSaveFailed, { error: true });
  });
});
