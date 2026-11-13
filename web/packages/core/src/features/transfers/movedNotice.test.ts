import { beforeEach, describe, expect, it } from "vitest";
import { get, writable } from "../../reactive/store";
import type { DetailView } from "../items/detail";
import { createMovedNotice, type MovedNotice } from "./movedNotice";

// 旧 ID から開いたときの詳細パネルの上部の 1 行(A5a-16)。
const moved = {
  old: "HH-5",
  at: new Date(2026, 10, 13, 9, 5).toISOString(),
  display: "WEB-1",
  current: "WEB-1",
};
const line = "HH-5 は 11/13 09:05 に WEB-1 へ移動しました";
const shown = (storeId: string, id: string) => ({ item: { storeId, id } }) as DetailView;

describe("createMovedNotice", () => {
  let view = writable<DetailView | null>(null);
  let notice: MovedNotice;
  beforeEach(() => {
    view = writable<DetailView | null>(null);
    notice = createMovedNotice({ detail: { view }, now: () => new Date(2026, 10, 14, 12, 0) });
  });

  it("shows the line once the panel shows the item, not before", () => {
    notice.show("S1", "I5", moved);
    expect(get(notice.text)).toBe("");
    view.set(shown("S1", "I5"));
    expect(get(notice.text)).toBe(line);
  });

  it("also says the later move when the item moved again after the recorded one", () => {
    notice.show("S1", "I5", { ...moved, display: "TM-4", current: "AR-7" });
    view.set(shown("S1", "I5"));
    expect(get(notice.text)).toBe(
      "HH-5 は 11/13 09:05 に TM-4 へ移動し、その後 AR-7 へ移動しました",
    );
  });

  it("clears when another item is opened, and stays cleared when the first one is opened again", () => {
    notice.show("S1", "I5", moved);
    view.set(shown("S1", "I5"));
    view.set(shown("S1", "I6"));
    expect(get(notice.text)).toBe("");
    view.set(shown("S1", "I5"));
    expect(get(notice.text)).toBe("");
  });

  it("clears when the panel is closed", () => {
    notice.show("S1", "I5", moved);
    view.set(shown("S1", "I5"));
    view.set(null);
    expect(get(notice.text)).toBe("");
  });

  it("does not mix up the same internal ID of another Store, and a later show replaces the line", () => {
    notice.show("S1", "I5", moved);
    view.set(shown("S2", "I5"));
    expect(get(notice.text)).toBe("");
    notice.show("S2", "I5", { ...moved, old: "HH-7", display: "WEB-2", current: "WEB-2" });
    view.set(shown("S2", "I5"));
    expect(get(notice.text)).toBe("HH-7 は 11/13 09:05 に WEB-2 へ移動しました");
  });

  it("keeps the line through a reload of the same item", () => {
    notice.show("S1", "I5", moved);
    view.set(shown("S1", "I5"));
    view.set(shown("S1", "I5"));
    expect(get(notice.text)).toBe(line);
  });

  it("drops a line whose item another open took the place of before it showed", () => {
    notice.show("S1", "I5", moved);
    view.set(shown("S1", "I6"));
    view.set(shown("S1", "I5"));
    expect(get(notice.text)).toBe("");
  });
});
