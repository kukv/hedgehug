import { describe, expect, it, vi } from "vitest";
import type { ActivityPage } from "../../model/activity";
import { get } from "../../reactive/store";
import { row, rows, setup } from "./activity.test-util";

const all = { workspace: "", kind: "all" } as const;

describe("createActivity (06「アクティビティ」)", () => {
  it("opens the 300 newest rows without before, and more passes the last row's cursor", async () => {
    const t = setup(vi.fn, rows(301));
    await t.activity.open();
    let s = get(t.activity.state);
    expect(s).toMatchObject({ filter: all, more: true, loading: false, loaded: true });
    expect(s.rows).toHaveLength(300);
    expect(t.port.list).toHaveBeenLastCalledWith(all);

    await t.activity.more();
    s = get(t.activity.state);
    expect(t.port.list).toHaveBeenLastCalledWith(all, row(2).cursor);
    expect(s.rows).toHaveLength(301);
    expect(s.rows.at(-1)?.key).toBe("k1");
    expect(s.more).toBe(false);
  });

  it("does nothing on more when there is no more", async () => {
    const t = setup(vi.fn, rows(2));
    await t.activity.open();
    await t.activity.more();
    expect(t.port.list).toHaveBeenCalledTimes(1);
  });

  it("reads again from the top when the filter changes, merging partial filters", async () => {
    const t = setup(vi.fn, rows(3));
    await t.activity.open();
    await t.activity.setFilter({ workspace: "S1/W1" });
    expect(t.port.list).toHaveBeenLastCalledWith({ workspace: "S1/W1", kind: "all" });
    await t.activity.setFilter({ kind: "sync" });
    expect(t.port.list).toHaveBeenLastCalledWith({ workspace: "S1/W1", kind: "sync" });
    expect(get(t.activity.state).filter).toEqual({ workspace: "S1/W1", kind: "sync" });
    expect(get(t.activity.state).rows).toHaveLength(3);
  });

  it("drops an answer for a filter that is no longer selected", async () => {
    const t = setup(vi.fn, [row(2), row(1)]);
    let release: (p: ActivityPage) => void = () => {};
    t.port.list.mockImplementationOnce(() => new Promise<ActivityPage>((r) => (release = r)));
    const opening = t.activity.open();
    await t.activity.setFilter({ kind: "user" });
    release({ rows: [row(2)], more: false });
    await opening;
    expect(get(t.activity.state).rows.map((r) => r.key)).toEqual(["k2", "k1"]);
    expect(get(t.activity.state).filter.kind).toBe("user");
  });

  it("reloads as many rows as were shown, 300 at a time (11 reads for 3300 rows)", async () => {
    const t = setup(vi.fn, rows(3300));
    await t.activity.open();
    for (let i = 0; i < 10; i++) await t.activity.more();
    expect(get(t.activity.state).rows).toHaveLength(3300);
    t.port.list.mockClear();
    await t.activity.reload();
    expect(t.port.list).toHaveBeenCalledTimes(11);
    expect(t.port.list).toHaveBeenNthCalledWith(1, all, undefined);
    expect(t.port.list).toHaveBeenNthCalledWith(2, all, row(3001).cursor);
    expect(get(t.activity.state).rows).toHaveLength(3300);
    expect(get(t.activity.state).more).toBe(false);
  });

  it("shows new rows after a reload", async () => {
    const t = setup(vi.fn, [row(1)]);
    await t.activity.open();
    t.change([row(2), row(1)]);
    await t.activity.reload();
    expect(get(t.activity.state).rows.map((r) => r.key)).toEqual(["k2", "k1"]);
  });

  it("shows a failed read as an error toast without throwing, and keeps loaded false", async () => {
    const t = setup(vi.fn, rows(1));
    t.port.list.mockRejectedValueOnce(new Error("読めません"));
    await t.activity.open();
    expect(get(t.activity.state)).toMatchObject({ loading: false, loaded: false });
    expect(get(t.toasts.list).map((x) => [x.text, x.error])).toEqual([["読めません", true]]);
  });

  it("swallows a failed reload and stops loading", async () => {
    const t = setup(vi.fn, rows(2));
    await t.activity.open();
    t.port.list.mockRejectedValueOnce(new Error("読めません"));
    await t.activity.reload();
    expect(get(t.activity.state).rows).toHaveLength(2);
    expect(get(t.toasts.list)).toHaveLength(0);
  });

  it("drops the answer of a reload when the filter changes meanwhile", async () => {
    const t = setup(vi.fn, rows(3));
    await t.activity.open();
    let release: (p: ActivityPage) => void = () => {};
    t.port.list.mockImplementationOnce(() => new Promise<ActivityPage>((r) => (release = r)));
    const reloading = t.activity.reload();
    await t.activity.setFilter({ kind: "user" });
    release({ rows: [row(9)], more: false });
    await reloading;
    expect(get(t.activity.state).filter.kind).toBe("user");
    expect(get(t.activity.state).rows.map((r) => r.key)).toEqual(["k3", "k2", "k1"]);
  });

  it("drops the answer of a reload when more runs meanwhile", async () => {
    const t = setup(vi.fn, rows(301));
    await t.activity.open();
    let release: (p: ActivityPage) => void = () => {};
    t.port.list.mockImplementationOnce(() => new Promise<ActivityPage>((r) => (release = r)));
    const reloading = t.activity.reload();
    await t.activity.more();
    release({ rows: [row(9)], more: false });
    await reloading;
    expect(get(t.activity.state).rows).toHaveLength(301);
  });

  it("stops a reload when the server returns no rows but more", async () => {
    const t = setup(vi.fn, rows(301));
    await t.activity.open();
    t.port.list.mockImplementationOnce(async () => ({ rows: [], more: true }));
    await t.activity.reload();
    expect(t.port.list).toHaveBeenCalledTimes(2);
    expect(get(t.activity.state)).toMatchObject({ rows: [], loading: false, loaded: true });
  });
});
