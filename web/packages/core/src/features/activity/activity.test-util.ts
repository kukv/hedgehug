import type { ActivityFilter, ActivityRow } from "../../model/activity";
import { createToasts } from "../toasts/toasts";
import { createActivity } from "./activity";

// Spy は、vitest の vi.fn が返す関数のうち、ここで使う部分(core の本体のコードは vitest を読めないので、型を自前で持つ)。
type Spy<A extends unknown[], R> = ((...args: A) => R) & {
  mockImplementationOnce(impl: (...args: A) => R): unknown;
  mockRejectedValueOnce(error: unknown): unknown;
  mockClear(): unknown;
};
export type MakeSpy = <A extends unknown[], R>(impl: (...args: A) => R) => Spy<A, R>;

// row は、鍵 k{n} の行。n が大きいほど新しい(n ミリ秒ずつ進む時刻)。
export function row(n: number, over: Partial<ActivityRow> = {}): ActivityRow {
  const at = new Date(Date.UTC(2026, 10, 14) + n).toISOString();
  return {
    key: `k${n}`,
    cursor: `${at}|k${n}`,
    at,
    actor: "user",
    who: "Alice",
    avatar: "Alice",
    text: `変更 ${n}`,
    store: { id: "S1", name: "team", color: "blue" },
    workspace: "メイン",
    sha: "abc1234",
    ...over,
  };
}

// rows は、1 から n までの行(新しい順)。
export const rows = (n: number) => Array.from({ length: n }, (_, i) => row(n - i));

// setup は、メモリ上のアクティビティの口で createActivity を作る。口は 300 件ずつ、before の cursor より後ろを返す。
export function setup(fn: MakeSpy, initial: ActivityRow[]) {
  let all = initial;
  const port = {
    list: fn(async (_filter: ActivityFilter, before?: string) => {
      const start = before ? all.findIndex((r) => r.cursor === before) + 1 : 0;
      const hit = all.slice(start);
      return { rows: hit.slice(0, 300), more: hit.length > 300 };
    }),
  };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const activity = createActivity({ port, toasts });
  return {
    activity,
    port,
    toasts,
    // change は、口の裏の行を書き換える(別のタブ・同期での変化)。
    change: (next: ActivityRow[]) => {
      all = next;
    },
  };
}
