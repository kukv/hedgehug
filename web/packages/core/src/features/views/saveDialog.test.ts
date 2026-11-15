import { describe, expect, it } from "vitest";
import type { Toasts } from "../toasts/toasts";
import type { ViewContent } from "./content";
import { saveAsView, saveSummary } from "./saveDialog";
import type { SavedView } from "./views";

const content: ViewContent = {
  query: "kind:bug",
  layout: "tree",
  table: { columns: ["id", "title"], group: "none", sort: null },
};

describe("saveSummary (E3b-10)", () => {
  it("names the layout and blanks an empty query", () => {
    expect(saveSummary({ ...content, query: "  ", layout: "table" })).toEqual({
      layout: "テーブル",
      query: null,
    });
    expect(saveSummary({ ...content, query: " kind:bug " })).toEqual({
      layout: "ツリー",
      query: "kind:bug",
    });
  });
});

function setup(result: SavedView | string | null) {
  const shown: string[] = [];
  const opened: string[] = [];
  const deps = {
    views: { create: async () => result },
    toasts: { show: (t: string) => shown.push(t) } as unknown as Toasts,
    open: async (id: string) => {
      opened.push(id);
      return true;
    },
  };
  return { deps, shown, opened };
}

describe("saveAsView (E3b-10)", () => {
  it("shows the toast and opens the new view", async () => {
    const s = setup({ id: "v1", name: "バグ", content });
    expect(await saveAsView(s.deps, "バグ", content)).toBeNull();
    expect(s.shown).toEqual(["ビュー「バグ」を保存しました"]);
    expect(s.opened).toEqual(["v1"]);
  });

  it("returns the refusal without toast or opening", async () => {
    const s = setup("同じ名前のビューがあります");
    expect(await saveAsView(s.deps, "バグ", content)).toBe("同じ名前のビューがあります");
    expect(s.shown).toEqual([]);
    expect(s.opened).toEqual([]);
  });

  it("does nothing for a failure that already showed an error", async () => {
    const s = setup(null);
    expect(await saveAsView(s.deps, "バグ", content)).toBeNull();
    expect(s.shown).toEqual([]);
    expect(s.opened).toEqual([]);
  });
});
