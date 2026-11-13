// Spy は、vitest の vi.fn が返す関数のうち、ここで使う部分(core の本体のコードは vitest を読めないので、型を自前で持つ)。
type Spy<A extends unknown[], R> = ((...args: A) => R) & {
  mockImplementationOnce(impl: (...args: A) => R): unknown;
  mockRejectedValueOnce(error: unknown): unknown;
  mockResolvedValueOnce(value: Awaited<R>): unknown;
};
export type MakeSpy = <A extends unknown[], R>(impl: (...args: A) => R) => Spy<A, R>;

import type { Comment } from "../../model/notes";
import type { NoteTarget } from "../../port/notes";
import { createToasts } from "../toasts/toasts";
import { createComments } from "./comments";

export const target: NoteTarget = { kind: "item", storeId: "S1", id: "I1" };
export const other: NoteTarget = { kind: "item", storeId: "S1", id: "I2" };
export const members = [
  { login: "taro", name: "太郎" },
  { login: "hanako", name: "花子" },
];

export function comment(id: string, by: string, body: string, edited = false): Comment {
  return {
    id,
    by,
    byName: by === "ghost" ? "不明なユーザー" : by,
    at: "2026-11-13T09:00:00.000Z",
    body,
    edited,
  };
}

// setup は、メモリ上のコメントの口と書きかけの登録簿で createComments を作る。
export function setup(fn: MakeSpy, initial: Comment[] = [comment("K1", "hanako", "前")]) {
  let rows = initial;
  const calls: string[] = [];
  const notes = {
    listComments: fn(async (_t: NoteTarget) => rows),
    postComment: fn(async (_t: NoteTarget, body: string) => {
      calls.push(`post:${body}`);
      const c = comment(`K${rows.length + 1}`, "hanako", body);
      rows = [...rows, c];
      return c;
    }),
    editComment: fn(async (_t: NoteTarget, id: string, body: string) => {
      calls.push(`edit:${id}:${body}`);
      rows = rows.map((r) => (r.id === id ? { ...r, body, edited: true } : r));
      return rows.find((r) => r.id === id) as Comment;
    }),
    deleteComment: fn(async (_t: NoteTarget, id: string) => {
      calls.push(`delete:${id}`);
      rows = rows.filter((r) => r.id !== id);
    }),
  };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const dirty: Record<string, () => boolean> = {};
  const held: Record<string, () => boolean> = {};
  const confirmLeave = fn(async (_key?: string) => true);
  const drafts = {
    register: (key: string, d: () => boolean, h: () => boolean = d) => {
      dirty[key] = d;
      held[key] = h;
      return () => {
        delete dirty[key];
        delete held[key];
      };
    },
    confirmLeave,
  };
  const comments = createComments({
    notes,
    catalogPort: { listMembers: async () => members, me: async () => "hanako" },
    toasts,
    drafts,
    scope: "item",
  });
  return {
    comments,
    notes,
    toasts,
    dirty,
    held,
    confirmLeave,
    calls,
    setRows: (r: Comment[]) => {
      rows = r;
    },
  };
}
