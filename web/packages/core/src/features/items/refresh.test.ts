import { describe, expect, it } from "vitest";
import type { Item } from "../../model/item";
import { writable } from "../../reactive/store";
import type { DetailView } from "./detail";
import { item } from "./fixtures.test-util";
import { createItemRefresh } from "./refresh";

function setup(open: Item | null) {
  const calls: string[] = [];
  const view = writable<DetailView | null>(open ? ({ item: open } as DetailView) : null);
  const refresh = createItemRefresh({
    detail: {
      view,
      close: () => {
        calls.push("close");
        view.set(null);
      },
      reload: async () => {
        calls.push("detail");
      },
    },
    projectScreen: {
      reload: async () => {
        calls.push("tree");
      },
    },
    sections: [
      {
        reload: async () => {
          calls.push("relations");
        },
      },
    ],
  });
  return { refresh, calls };
}

const epic = item("S1", "S1-P1", 1, "Epic", null);
const task = item("S1", "S1-P1", 2, "Task", epic.id);

describe("item refresh", () => {
  it("reloads the detail panel, the tree and the sections after a change", async () => {
    const { refresh, calls } = setup(task);
    await refresh.afterChange();
    expect(calls).toEqual(["detail", "tree", "relations"]);
  });

  it("does not let one failed reload stop the others", async () => {
    const calls: string[] = [];
    const refresh = createItemRefresh({
      detail: {
        view: writable(null),
        close: () => {},
        reload: async () => {
          throw new Error("network");
        },
      },
      projectScreen: {
        reload: async () => {
          calls.push("tree");
        },
      },
      sections: [
        {
          reload: async () => {
            calls.push("relations");
          },
        },
      ],
    });
    await refresh.afterChange();
    expect(calls).toEqual(["tree", "relations"]);
  });

  it("closes the panel before reloading when the deleted item is the open one", async () => {
    const { refresh, calls } = setup(task);
    await refresh.afterDelete(task);
    expect(calls).toEqual(["close", "detail", "tree", "relations"]);
  });

  it("keeps the panel when another item is deleted (the reload closes it on a 404)", async () => {
    const { refresh, calls } = setup(task);
    await refresh.afterDelete(epic);
    expect(calls).toEqual(["detail", "tree", "relations"]);
  });

  it("reloads without closing when no panel is open", async () => {
    const { refresh, calls } = setup(null);
    await refresh.afterDelete(task);
    expect(calls).toEqual(["detail", "tree", "relations"]);
  });
});
