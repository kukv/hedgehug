import { describe, expect, it } from "vitest";
import { MOVE_VALUE, moveWorkspaceItem } from "./move-menu";
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
