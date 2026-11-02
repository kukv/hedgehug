import { describe, expect, it } from "vitest";
import { opLabel } from "./history-labels";

describe("opLabel", () => {
  it("names the operations", () => {
    expect(opLabel("add")).toBe("作成");
    expect(opLabel("set")).toBe("変更");
    expect(opLabel("mv")).toBe("移動");
    expect(opLabel("rm")).toBe("削除");
    expect(opLabel("restore")).toBe("復元");
    expect(opLabel("merge")).toBe("同期でマージ");
  });
  it("names every operation internal/ops writes into a commit", () => {
    const ops = [
      "init",
      "add",
      "set",
      "edit",
      "status",
      "dep",
      "mv",
      "rm",
      "restore",
      "purge",
      "purge-moved",
      "rewrite-history",
      "revert-overwritten",
      "move-in",
      "move-out",
      "migrate",
      "undo",
      "workspace",
      "comment",
      "watch",
      "unwatch",
      "view",
      "setup",
      "merge",
      "repair",
      "replay",
    ];
    for (const op of ops) expect(opLabel(op), op).not.toBe(op);
  });
  it("passes an unknown op through and names a missing one", () => {
    expect(opLabel("frobnicate")).toBe("frobnicate");
    expect(opLabel(null)).toBe("変更");
  });
});
