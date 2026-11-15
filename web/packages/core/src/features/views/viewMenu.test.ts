import { describe, expect, it } from "vitest";
import type { Toasts } from "../toasts/toasts";
import { deleteView, deleteViewBody, duplicateView, viewMenuItems } from "./viewMenu";
import type { SavedView } from "./views";

describe("viewMenuItems / deleteViewBody (E3b-13)", () => {
  it("lists the three entries", () => {
    expect(viewMenuItems.map((i) => i.label)).toEqual(["名前を変更…", "複製", "削除…"]);
  });
  it("words the delete confirmation", () => {
    expect(deleteViewBody("バグ")).toBe(
      "ビュー「バグ」を削除します。アイテムは消えません。元に戻せません。",
    );
  });
});

describe("deleteView", () => {
  function setup(ok: boolean) {
    const shown: string[] = [];
    const replaced: unknown[] = [];
    const deps = {
      views: { remove: async () => ok },
      toasts: { show: (m: string) => shown.push(m) } as unknown as Toasts,
      navigation: { replace: (r: unknown) => replaced.push(r) },
    };
    return { deps, shown, replaced };
  }
  it("toasts and goes home on success", async () => {
    const { deps, shown, replaced } = setup(true);
    await deleteView(deps, "v1");
    expect(shown).toEqual(["ビューを削除しました"]);
    expect(replaced).toEqual([{ name: "mytasks" }]);
  });
  it("stays when the delete failed", async () => {
    const { deps, shown, replaced } = setup(false);
    await deleteView(deps, "v1");
    expect(shown).toEqual([]);
    expect(replaced).toEqual([]);
  });
});

describe("duplicateView", () => {
  it("opens the copy", async () => {
    const opened: string[] = [];
    const copy = { id: "v2" } as SavedView;
    await duplicateView(
      { views: { duplicate: async () => copy }, open: async (id) => void opened.push(id) },
      "v1",
    );
    expect(opened).toEqual(["v2"]);
  });
  it("does nothing when refused", async () => {
    const opened: string[] = [];
    await duplicateView(
      { views: { duplicate: async () => null }, open: async (id) => void opened.push(id) },
      "v1",
    );
    expect(opened).toEqual([]);
  });
});
