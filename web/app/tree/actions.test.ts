import { describe, expect, it, vi } from "vitest";
import { createBelow, moveTask, patchTask } from "./actions";
import { task } from "./fixtures";

const ok = (data: unknown) => Promise.resolve(data);

function deps() {
  const api = {
    createTask: vi.fn(() => ok({ task: task("N1", "task", "P"), commit: "c1" })),
    moveTask: vi.fn(() => ok({ task: task("N1", "task", "P", { rank: "b" }), commit: "c2" })),
    updateTask: vi.fn(() =>
      ok({ task: task("T1", "task", "P", { status: "done" }), commit: "c3" }),
    ),
  };
  const cache = { applyWrite: vi.fn() };
  return { api, cache } as unknown as Parameters<typeof moveTask>[0] & {
    api: typeof api;
    cache: typeof cache;
  };
}

describe("moveTask", () => {
  it("posts the move with the store and records the write", async () => {
    const d = deps();
    await moveTask(d, task("T1", "task", "P"), { parent: "E1", after: "T2" });
    expect(d.api.moveTask).toHaveBeenCalledWith(
      "T1",
      { parent: "E1", after: "T2" },
      { store: "shared" },
    );
    expect(d.cache.applyWrite).toHaveBeenCalledWith({
      task: expect.objectContaining({ ulid: "N1" }),
      commit: "c2",
    });
  });
});

describe("patchTask", () => {
  it("patches the status", async () => {
    const d = deps();
    await patchTask(d, task("T1", "task", "P"), { status: "done" });
    expect(d.api.updateTask).toHaveBeenCalledWith("T1", { status: "done" }, { store: "shared" });
    expect(d.cache.applyWrite).toHaveBeenCalledWith({
      task: expect.objectContaining({ status: "done" }),
      commit: "c3",
    });
  });
});

describe("createBelow", () => {
  const base = { title: "new", kind: "task", store: "shared", parent: "P", workspace: "REL" };

  it("creates right after the row above it, in one request", async () => {
    const d = deps();
    const t = await createBelow(d, { ...base, after: "T1" });
    expect(d.api.createTask).toHaveBeenCalledTimes(1);
    expect(d.api.createTask).toHaveBeenCalledWith(
      { title: "new", kind: "task", parent: "P", after: "T1" },
      { store: "shared" },
    );
    expect(t.ulid).toBe("N1");
  });

  it("names no position when it goes to the end of the list", async () => {
    const d = deps();
    await createBelow(d, { ...base, after: null });
    expect(d.api.createTask).toHaveBeenCalledWith(
      { title: "new", kind: "task", parent: "P" },
      { store: "shared" },
    );
  });

  it("names the workspace when there is no parent", async () => {
    const d = deps();
    await createBelow(d, { ...base, parent: null, after: null });
    expect(d.api.createTask).toHaveBeenCalledWith(
      { title: "new", kind: "task", workspace: "REL" },
      { store: "shared" },
    );
  });
});
