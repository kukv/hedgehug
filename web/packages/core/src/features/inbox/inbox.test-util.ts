// Spy は、vitest の vi.fn が返す関数のうち、ここで使う部分(core の本体のコードは vitest を読めないので、型を自前で持つ)。
type Spy<A extends unknown[], R> = ((...args: A) => R) & {
  mockImplementationOnce(impl: (...args: A) => R): unknown;
  mockRejectedValueOnce(error: unknown): unknown;
  mockResolvedValueOnce(value: Awaited<R>): unknown;
};
export type MakeSpy = <A extends unknown[], R>(impl: (...args: A) => R) => Spy<A, R>;

import type { InboxCounts, InboxRow, InboxRowKind, InboxTab } from "../../model/inbox";
import type { InboxCursor } from "../../port/inbox";
import { createToasts } from "../toasts/toasts";
import { createInbox } from "./inbox";

const tabs: InboxTab[] = ["all", "mention", "assign", "watch", "overwrite", "automation", "repair"];

// inTab は、行がタブに出るか(止まった通知は「自動化」のタブに出る)。
const inTab = (r: InboxRow, tab: InboxTab) =>
  tab === "all" || r.kind === tab || (tab === "automation" && r.kind === "automation_stopped");

// before は、行が印より前(古い)か。並びは at の新しい順、同じなら id の大きい順。
const before = (r: InboxRow, c: InboxCursor) => r.at < c.at || (r.at === c.at && r.id < c.id);

export function row(id: number, kind: InboxRowKind = "automation", read = false): InboxRow {
  // at は、id が大きいほど新しい。ナノ秒までの固定幅にする(印の文字列をそのまま比べる)。
  const at = `2026-11-14T00:00:00.${String(id).padStart(9, "0")}Z`;
  return {
    id,
    at,
    kind,
    read,
    sender: "",
    text: `通知 ${id}`,
    reason: "",
    store: { id: "S1", name: "team", color: "blue", readOnly: false },
    target: {
      kind: "item",
      item: `I${id}`,
      display: `HH-${id}`,
      itemKind: "Task",
      title: "タイトル",
      workspace: "W1",
      workspaceName: "メイン",
      section: "",
    },
  };
}

// rows は、1 から n までの行(新しい順)。
export const rows = (n: number, kind: InboxRowKind = "automation") =>
  Array.from({ length: n }, (_, i) => row(n - i, kind));

// setup は、メモリ上の受信箱の口で createInbox を作る。
export function setup(fn: MakeSpy, initial: InboxRow[]) {
  let all = initial;
  const counts = (): InboxCounts =>
    Object.fromEntries(
      tabs.map((t) => [t, all.filter((r) => !r.read && inTab(r, t)).length]),
    ) as InboxCounts;
  const port = {
    list: fn(async (tab: InboxTab, c?: InboxCursor) => {
      const hit = all.filter((r) => inTab(r, tab) && (!c || before(r, c)));
      return { rows: hit.slice(0, 300), more: hit.length > 300, unread: counts() };
    }),
    unread: fn(async () => counts()),
    setRead: fn(async (id: number, read: boolean) => {
      all = all.map((r) => (r.id === id ? { ...r, read } : r));
    }),
    readAll: fn(async (tab: InboxTab) => {
      all = all.map((r) => (inTab(r, tab) ? { ...r, read: true } : r));
    }),
    apply: fn(async (id: number) => {
      all = all.map((r) => (r.id === id ? { ...r, read: true } : r));
    }),
    keep: fn(async (id: number) => {
      all = all.map((r) => (r.id === id ? { ...r, read: true } : r));
    }),
  };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const inbox = createInbox({ port, toasts });
  return {
    inbox,
    port,
    toasts,
    // change は、口の裏の行を書き換える(別のタブ・同期での変化)。
    change: (next: InboxRow[]) => {
      all = next;
    },
  };
}
