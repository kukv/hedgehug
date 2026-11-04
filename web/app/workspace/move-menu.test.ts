import { describe, expect, it } from "vitest";
import { MOVE_STORE_VALUE, MOVE_VALUE, moveStoreItem, moveWorkspaceItem } from "./move-menu";
import { entry } from "./testkit";

const task = { store: "shared", workspace: { ulid: "W1", name: "release", id_prefix: "REL" } };

describe("moveWorkspaceItem", () => {
  it("is on when the store has another workspace", () => {
    const item = moveWorkspaceItem(task, [entry(), entry({ ulid: "W2", name: "shared" })]);
    expect(item).toMatchObject({
      value: MOVE_VALUE,
      label: "別のワークスペースへ移動…",
      disabled: false,
    });
    expect(item.hint).toBeUndefined();
  });

  it("is off, and says why, when the only other workspaces are in other stores", () => {
    const item = moveWorkspaceItem(task, [entry(), entry({ store: "backend", ulid: "B1" })]);
    expect(item.disabled).toBe(true);
    expect(item.hint).toBe("この Store にはほかのワークスペースがありません");
  });

  it("is off, and says why, when the store is read-only", () => {
    const item = moveWorkspaceItem(
      task,
      [entry(), entry({ ulid: "W2", name: "shared" })],
      "remote に書き込めません",
    );
    expect(item.disabled).toBe(true);
    expect(item.hint).toBe("この Store は閲覧のみです（remote に書き込めません）");
  });
});

describe("moveStoreItem", () => {
  const two = [{ name: "shared" }, { name: "second" }];

  it("is there when two Stores are mounted and the Store of the task can be written", () => {
    expect(moveStoreItem(two, null)).toEqual({
      value: MOVE_STORE_VALUE,
      label: "別の Store へ移動…",
    });
  });

  it("is not there with one Store, or when the Store of the task is read-only", () => {
    expect(moveStoreItem([{ name: "shared" }], null)).toBeNull();
    expect(moveStoreItem(two, "remote に書き込めません")).toBeNull();
  });
});
