import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ScreenStatesPort, SearchPort } from "../../port/search";
import { get } from "../../reactive/store";
import { createScreenStates } from "../screenstate/screenStates";
import { createToasts } from "../toasts/toasts";
import { result, timer } from "./fixtures.test-util";
import { createSearch } from "./search";
import { defaultTable, selectable } from "./table";

function setup(saved: string | null = null) {
  const prune = vi.fn();
  const search = vi.fn(async (q: string) => result(q));
  const port: SearchPort = { search };
  const sp: ScreenStatesPort = {
    get: vi.fn(async () => saved),
    put: vi.fn(async () => {}),
  };
  const toasts = createToasts(timer);
  const s = createSearch({
    port,
    screenStates: createScreenStates(sp),
    timer,
    toasts,
    selection: { prune },
  });
  return { s, search, sp, toasts, prune };
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("createSearch with the table settings (E2a)", () => {
  it("opens the E1 shaped saved state with the default table", async () => {
    const t = setup('{"query":"x"}');
    await t.s.open();
    expect(get(t.s.state).table).toEqual(defaultTable);
  });

  it("setTable writes columns only without searching, and searches at once when group changes", async () => {
    const t = setup('{"query":"x"}');
    await t.s.open();
    t.search.mockClear();
    t.s.setTable({ ...defaultTable, columns: ["id", "title"] });
    await vi.advanceTimersByTimeAsync(500);
    expect(t.search).not.toHaveBeenCalled();
    expect(t.sp.put).toHaveBeenLastCalledWith(
      "search",
      JSON.stringify({ query: "x", table: { ...defaultTable, columns: ["id", "title"] } }),
    );
    t.s.setTable({ columns: ["id", "title"], group: "kind", sort: null });
    await vi.advanceTimersByTimeAsync(0);
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("x", { sort: null, group: "kind" });
  });

  it("setQuery searches with the current table request", async () => {
    const t = setup();
    await t.s.open();
    t.s.setTable({ ...defaultTable, sort: { key: "p:Foo", desc: true } });
    t.s.setQuery("q");
    await vi.advanceTimersByTimeAsync(150);
    expect(t.search).toHaveBeenLastCalledWith("q", {
      sort: { key: "foo", desc: true },
      group: "none",
    });
  });

  it("replaceQuery and prepareQuery keep the table", async () => {
    const t = setup(JSON.stringify({ query: "a", table: { ...defaultTable, group: "kind" } }));
    await t.s.open();
    await t.s.replaceQuery("b");
    t.s.prepareQuery("c");
    expect(get(t.s.state).table.group).toBe("kind");
  });

  it("prunes the selection with each result", async () => {
    const t = setup();
    await t.s.replaceQuery("a");
    expect(t.prune).toHaveBeenLastCalledWith(selectable(result("a").rows));
    t.search.mockResolvedValueOnce({ ...result(), errors: ["bad"] });
    await t.s.replaceQuery("b");
    expect(t.prune).toHaveBeenLastCalledWith([]);
  });

  it("settled resolves at once when nothing is pending", async () => {
    const t = setup();
    await t.s.settled();
  });

  it("settled waits for the debounce and the answer after setQuery", async () => {
    const t = setup();
    t.s.setQuery("q");
    let done = false;
    void t.s.settled().then(() => (done = true));
    await vi.advanceTimersByTimeAsync(149);
    expect(done).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    expect(done).toBe(true);
    expect(get(t.s.state).result?.rows[0]?.itemId).toBe("q");
  });

  it("settled after prepareQuery waits for open's answer", async () => {
    const t = setup();
    t.s.prepareQuery("kind:bug");
    let done = false;
    void t.s.settled().then(() => (done = true));
    await vi.advanceTimersByTimeAsync(500);
    expect(done).toBe(false);
    await t.s.open();
    await vi.advanceTimersByTimeAsync(0);
    expect(done).toBe(true);
    expect(get(t.s.state).result?.rows[0]?.itemId).toBe("kind:bug");
  });

  it("a columns-only setTable while open is reading does not stop open's search", async () => {
    const t = setup();
    let release: (v: string | null) => void = () => {};
    t.sp.get = vi.fn(() => new Promise<string | null>((r) => (release = r)));
    const opening = t.s.open();
    t.s.setTable({ ...defaultTable, columns: ["id", "title"] });
    release('{"query":"x"}');
    await opening;
    expect(t.search).toHaveBeenCalledTimes(1);
  });

  it("settled still resolves when a search throws", async () => {
    const t = setup();
    t.search.mockRejectedValueOnce(new Error("down"));
    t.s.setQuery("q");
    let done = false;
    void t.s.settled().then(() => (done = true));
    await vi.advanceTimersByTimeAsync(150);
    expect(done).toBe(true);
  });

  it("setTable does not search when sort and group are unchanged, and searches once for a sort-only change", async () => {
    const t = setup();
    await t.s.open();
    t.search.mockClear();
    t.s.setTable({ ...defaultTable });
    await vi.advanceTimersByTimeAsync(0);
    expect(t.search).not.toHaveBeenCalled();
    t.s.setTable({ ...defaultTable, sort: { key: "title", desc: false } });
    await vi.advanceTimersByTimeAsync(0);
    expect(t.search).toHaveBeenCalledTimes(1);
    expect(t.search).toHaveBeenCalledWith("", {
      sort: { key: "title", desc: false },
      group: "none",
    });
  });
});
