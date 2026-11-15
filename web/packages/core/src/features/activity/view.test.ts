import { describe, expect, it } from "vitest";
import type { StoreNode } from "../catalog/catalog";
import { row } from "./activity.test-util";
import {
  activityDays,
  activityEmpty,
  activityKindOptions,
  clockOf,
  workspaceOptions,
} from "./view";

// local は、このマシンの時刻帯での時刻の RFC 3339。
const local = (y: number, m: number, d: number, h = 12, min = 0) =>
  new Date(y, m - 1, d, h, min).toISOString();

describe("activityDays", () => {
  const rows = [
    row(5, { at: local(2026, 11, 15, 10) }),
    row(4, { at: local(2026, 11, 15, 8) }),
    row(3, { at: local(2026, 11, 14, 23, 59) }),
    row(2, { at: local(2025, 11, 14, 9) }),
  ];

  it("groups rows into 今日, 昨日 and YYYY/MM/DD, keeping the order inside a day", () => {
    const days = activityDays(rows, "2026-11-15");
    expect(days.map((d) => d.head)).toEqual(["今日", "昨日", "2025/11/14"]);
    expect(days.map((d) => d.rows.map((r) => r.key))).toEqual([["k5", "k4"], ["k3"], ["k2"]]);
  });

  it("changes the heads when today moves on", () => {
    const days = activityDays(rows, "2026-11-16");
    expect(days.map((d) => d.head)).toEqual(["昨日", "2026/11/14", "2025/11/14"]);
  });

  it("crosses a month boundary for 昨日", () => {
    const days = activityDays([row(1, { at: local(2026, 10, 31) })], "2026-11-01");
    expect(days.map((d) => d.head)).toEqual(["昨日"]);
  });

  it("is empty for no rows", () => {
    expect(activityDays([], "2026-11-15")).toEqual([]);
  });
});

describe("clockOf", () => {
  it("is HH:MM of the machine's time zone", () => {
    expect(clockOf(local(2026, 11, 15, 9, 5))).toBe("09:05");
  });
});

describe("options", () => {
  it("has the four kinds", () => {
    expect(activityKindOptions).toEqual([
      { value: "all", label: "すべて" },
      { value: "user", label: "人の変更だけ" },
      { value: "sync", label: "同期だけ" },
      { value: "automation", label: "自動化だけ" },
    ]);
  });

  it("builds workspace options from the catalog", () => {
    const workspace = (id: string, storeId: string, name: string) => ({
      id,
      storeId,
      name,
      prefix: id,
      oldPrefixes: [],
      projects: [],
    });
    const store = (id: string, name: string, workspaces: StoreNode["workspaces"]): StoreNode => ({
      id,
      name,
      color: "blue",
      readOnly: false,
      workspaces,
    });
    const stores = [
      store("S1", "team", [workspace("W1", "S1", "メイン"), workspace("W2", "S1", "サブ")]),
      store("S2", "me", [workspace("W3", "S2", "個人")]),
    ];
    expect(workspaceOptions(stores)).toEqual([
      { value: "", label: "すべて" },
      { value: "S1/W1", label: "team / メイン" },
      { value: "S1/W2", label: "team / サブ" },
      { value: "S2/W3", label: "me / 個人" },
    ]);
  });

  it("has the empty text", () => {
    expect(activityEmpty).toBe("この条件のコミットはありません");
  });
});
