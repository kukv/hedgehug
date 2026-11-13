import { describe, expect, it } from "vitest";
import { NotFound } from "../../model/errors";
import type { HistoryRow } from "../../model/history";
import { get } from "../../reactive/store";
import { flush } from "../items/fixtures.test-util";
import { createHistory, type HistoryDeps, historyLimit } from "./history";

const rowOf = (i: number): HistoryRow => ({
  who: "太郎",
  text: `変更 ${i}`,
  at: new Date(Date.UTC(2026, 10, 13, 9, 0, 0) - i * 60_000).toISOString(),
});
const rowsOf = (n: number) => Array.from({ length: n }, (_, i) => rowOf(i));
const item = { kind: "item", storeId: "S1", id: "I1" } as const;

function setup(initial: HistoryRow[]) {
  let rows = initial;
  let missing = false;
  const calls: string[] = [];
  const waiting: ((rows: HistoryRow[]) => void)[] = [];
  let hold = false;
  const notes: HistoryDeps["notes"] = {
    history(target) {
      calls.push(`${target.kind} ${target.storeId} ${target.id}`);
      if (missing) return Promise.reject(new NotFound("not found"));
      if (hold) return new Promise((resolve) => waiting.push(resolve));
      return Promise.resolve(rows);
    },
  };
  return {
    history: createHistory({ notes }),
    calls,
    waiting,
    set: (next: HistoryRow[]) => {
      rows = next;
    },
    vanish: () => {
      missing = true;
    },
    holdAnswers: () => {
      hold = true;
    },
  };
}

describe("createHistory", () => {
  it("読む前は loaded が false。40 件までを出し、それより古いものがあるときだけ more が true", async () => {
    const { history } = setup(rowsOf(45));
    expect(get(history.state)).toEqual({ rows: [], more: false, loaded: false });
    await history.load(item);
    const s = get(history.state);
    expect(s.loaded).toBe(true);
    expect(s.rows).toHaveLength(historyLimit);
    expect(s.rows[0]).toEqual(rowOf(0));
    expect(s.more).toBe(true);
  });

  it("ちょうど 40 件なら more は false", async () => {
    const { history } = setup(rowsOf(historyLimit));
    await history.load(item);
    expect(get(history.state)).toMatchObject({ more: false });
    expect(get(history.state).rows).toHaveLength(historyLimit);
  });

  it("履歴がなければ、空の一覧を読み終えた状態になる", async () => {
    const { history } = setup([]);
    await history.load(item);
    expect(get(history.state)).toEqual({ rows: [], more: false, loaded: true });
  });

  it("すべて表示を押すと全件を出し、同じ対象の読み直しでも保つ。別の対象では 40 件に戻す", async () => {
    const { history, set } = setup(rowsOf(45));
    await history.load(item);
    history.showAll();
    expect(get(history.state)).toMatchObject({ more: false });
    expect(get(history.state).rows).toHaveLength(45);
    set(rowsOf(46));
    await history.load(item);
    expect(get(history.state).rows).toHaveLength(46);
    await history.load({ kind: "project", storeId: "S1", id: "P1" });
    expect(get(history.state).rows).toHaveLength(historyLimit);
    expect(get(history.state).more).toBe(true);
  });

  it("読む前のすべて表示は、何もしない", () => {
    const { history } = setup(rowsOf(45));
    history.showAll();
    expect(get(history.state)).toEqual({ rows: [], more: false, loaded: false });
  });

  it("対象の種類をそのまま口に渡す", async () => {
    const { history, calls } = setup([]);
    await history.load(item);
    await history.load({ kind: "project", storeId: "S1", id: "P1" });
    expect(calls).toEqual(["item S1 I1", "project S1 P1"]);
  });

  it("古い読み取りの答えは捨てる", async () => {
    const { history, waiting, holdAnswers } = setup([]);
    holdAnswers();
    const slow = history.load(item);
    const fast = history.load(item);
    await flush();
    waiting[1]?.(rowsOf(2));
    await fast;
    waiting[0]?.(rowsOf(5));
    await slow;
    expect(get(history.state).rows).toHaveLength(2);
  });

  it("なくなっていたら、空にする", async () => {
    const { history, vanish } = setup(rowsOf(3));
    await history.load(item);
    vanish();
    await history.load(item);
    expect(get(history.state)).toEqual({ rows: [], more: false, loaded: true });
  });

  it("reset は、最初に戻す", async () => {
    const { history } = setup(rowsOf(3));
    await history.load(item);
    history.reset();
    expect(get(history.state)).toEqual({ rows: [], more: false, loaded: false });
  });

  it("reload は、開いている対象を読み直す。開いていなければ何もしない", async () => {
    const { history, calls, set } = setup(rowsOf(2));
    await history.reload();
    expect(calls).toEqual([]);
    await history.load(item);
    set(rowsOf(3));
    await history.reload();
    expect(calls).toEqual(["item S1 I1", "item S1 I1"]);
    expect(get(history.state).rows).toHaveLength(3);
    history.reset();
    await history.reload();
    expect(calls).toHaveLength(2);
  });

  it("読めなかったときは、何も出さず、いまの表示を変えない", async () => {
    let fail = false;
    const history = createHistory({
      notes: {
        history: () => (fail ? Promise.reject(new Error("boom")) : Promise.resolve(rowsOf(3))),
      },
    });
    await history.load(item);
    fail = true;
    await history.reload();
    expect(get(history.state).rows).toHaveLength(3);
    history.reset();
    await history.load(item);
    expect(get(history.state)).toEqual({ rows: [], more: false, loaded: false });
  });
});
