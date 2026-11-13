import { describe, expect, it } from "vitest";
import type { MovedFrom } from "../../model/transfer";
import { openMoved } from "./openMoved";

// 旧 ID から開くときの、詳細パネルの上部の 1 行の扱い(A5a-16)。開けなかったときと失敗したときだけ捨てる。
const moved: MovedFrom = {
  old: "HH-5",
  at: "2026-11-13T09:05:00Z",
  display: "WEB-1",
  current: "WEB-1",
};

function setup(open: () => Promise<void>, target: { storeId: string; itemId: string } | null) {
  const calls: string[] = [];
  const deps = {
    detail: { open, target: () => target },
    notice: {
      show: (s: string, i: string) => calls.push(`show ${s}/${i}`),
      clear: () => calls.push("clear"),
    },
  };
  return { deps, calls };
}

describe("openMoved", () => {
  it("keeps the line when a reload overtook the open (the panel still targets the item)", async () => {
    const { deps, calls } = setup(async () => {}, { storeId: "S1", itemId: "I5" });
    await openMoved(deps, "S1", "I5", moved);
    expect(calls).toEqual(["show S1/I5"]);
  });

  it("drops the line when the panel could not open the item", async () => {
    const { deps, calls } = setup(async () => {}, null);
    await openMoved(deps, "S1", "I5", moved);
    expect(calls).toEqual(["show S1/I5", "clear"]);
  });

  it("drops the line when the panel targets another item", async () => {
    const { deps, calls } = setup(async () => {}, { storeId: "S2", itemId: "I5" });
    await openMoved(deps, "S1", "I5", moved);
    expect(calls).toEqual(["show S1/I5", "clear"]);
  });

  it("drops the line and does not reject when the open fails", async () => {
    const { deps, calls } = setup(() => Promise.reject(new Error("boom")), {
      storeId: "S1",
      itemId: "I5",
    });
    await expect(openMoved(deps, "S1", "I5", moved)).resolves.toBeUndefined();
    expect(calls).toEqual(["show S1/I5", "clear"]);
  });
});
