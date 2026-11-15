import { describe, expect, it } from "vitest";
import { get, writable } from "../../reactive/store";
import { initialSearchState, type SearchState } from "../search/layout";
import type { Toasts } from "../toasts/toasts";
import type { ViewContent } from "./content";
import { createViewScreen } from "./viewScreen";
import type { SavedView } from "./views";

const saved: ViewContent = {
  query: "kind:bug",
  layout: "table",
  table: { columns: ["id", "title", "p:customer"], group: "none", sort: null },
};

const id = "01jabcdefghjkmnpqrstvwx001";

// setup は、一覧・画面の検索の状態・読み終えた印を偽物で持つ。restore は呼ばれた内容を覚え、画面の内容を置き換える。
function setup(result: SavedView | null = { id, name: "バグ", content: saved }) {
  const list = writable<SavedView[]>([{ id, name: "バグ", content: saved }]);
  const state = writable<SearchState>({ ...initialSearchState(), ...saved });
  const ready = writable(false);
  const restored: ViewContent[] = [];
  const savedWith: ViewContent[] = [];
  const shown: string[] = [];
  const screen = createViewScreen({
    id,
    views: {
      list,
      async save(_id, c) {
        savedWith.push(c);
        if (result) list.set([result]);
        return result;
      },
    },
    search: {
      state,
      restore(c) {
        restored.push(c);
        state.update((s) => ({ ...s, ...c }));
      },
    },
    ready,
    toasts: { show: (t: string) => shown.push(t) } as unknown as Toasts,
  });
  const setQuery = (query: string) => state.update((s) => ({ ...s, query }));
  return { screen, list, state, ready, restored, savedWith, shown, setQuery };
}

describe("createViewScreen (E3b-4・E3b-9・E3b-12)", () => {
  it("dirty after a change", () => {
    const s = setup();
    s.ready.set(true);
    expect(get(s.screen.dirty)).toBe(false);
    s.setQuery("kind:task");
    expect(get(s.screen.dirty)).toBe(true);
    s.setQuery("kind:bug");
    expect(get(s.screen.dirty)).toBe(false);
  });

  it("not dirty before ready", () => {
    const s = setup();
    s.setQuery("kind:task");
    expect(get(s.screen.dirty)).toBe(false);
    s.ready.set(true);
    expect(get(s.screen.dirty)).toBe(true);
  });

  it("discard restores the saved content", () => {
    const s = setup();
    s.ready.set(true);
    s.setQuery("kind:task");
    s.screen.discard();
    expect(s.restored).toEqual([saved]);
    expect(get(s.screen.dirty)).toBe(false);
  });

  it("save replaces the screen with the saved content", async () => {
    const pruned: ViewContent = {
      ...saved,
      query: "kind:task",
      table: { ...saved.table, columns: ["id", "title"] },
    };
    const s = setup({ id, name: "バグ", content: pruned });
    s.ready.set(true);
    s.setQuery("kind:task");
    await s.screen.save();
    expect(s.savedWith).toEqual([{ ...saved, query: "kind:task" }]);
    expect(s.restored).toEqual([pruned]);
    expect(get(s.screen.dirty)).toBe(false);
    expect(s.shown).toEqual(["ビューを保存しました"]);
  });

  it("save failure leaves the screen as it is", async () => {
    const s = setup(null);
    s.ready.set(true);
    s.setQuery("kind:task");
    await s.screen.save();
    expect(s.restored).toEqual([]);
    expect(s.shown).toEqual([]);
    expect(get(s.screen.dirty)).toBe(true);
  });

  it("saved content changed elsewhere", () => {
    const s = setup();
    s.ready.set(true);
    s.setQuery("kind:task");
    const values: boolean[] = [];
    const stop = s.screen.dirty.subscribe((d) => values.push(d));
    s.list.set([{ id, name: "バグ", content: { ...saved, query: "kind:task" } }]);
    expect(s.restored).toEqual([]);
    expect(get(s.state).query).toBe("kind:task");
    expect(values).toEqual([true, false]);
    stop();
  });

  it("not dirty when the view is not in the list", () => {
    const s = setup();
    s.ready.set(true);
    s.setQuery("kind:task");
    s.list.set([]);
    expect(get(s.screen.dirty)).toBe(false);
    s.screen.discard();
    expect(s.restored).toEqual([]);
  });
  it("ignores save while a save is waiting for its answer", async () => {
    const s = setup();
    s.ready.set(true);
    s.setQuery("kind:task");
    const first = s.screen.save();
    const second = s.screen.save();
    await Promise.all([first, second]);
    expect(s.savedWith).toHaveLength(1);
    expect(s.shown).toEqual(["ビューを保存しました"]);
    await s.screen.save();
    expect(s.savedWith).toHaveLength(2);
  });
});
