import { describe, expect, it, vi } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import type { Comment } from "../../model/notes";
import type { NoteTarget } from "../../port/notes";
import { get } from "../../reactive/store";
import { createToasts } from "../toasts/toasts";
import {
  commentAvatar,
  commentDeleteDialog,
  commentDeletedMessage,
  commentPlaceholder,
  createComments,
  isMine,
} from "./comments";

const target: NoteTarget = { kind: "item", storeId: "S1", id: "I1" };
const other: NoteTarget = { kind: "item", storeId: "S1", id: "I2" };
const members = [
  { login: "taro", name: "太郎" },
  { login: "hanako", name: "花子" },
];

function comment(id: string, by: string, body: string, edited = false): Comment {
  return {
    id,
    by,
    byName: by === "ghost" ? "不明なユーザー" : by,
    at: "2026-11-13T09:00:00.000Z",
    body,
    edited,
  };
}

function setup(initial: Comment[] = [comment("K1", "hanako", "前")]) {
  let rows = initial;
  const calls: string[] = [];
  const notes = {
    listComments: vi.fn(async (_t: NoteTarget) => rows),
    postComment: vi.fn(async (_t: NoteTarget, body: string) => {
      calls.push(`post:${body}`);
      const c = comment(`K${rows.length + 1}`, "hanako", body);
      rows = [...rows, c];
      return c;
    }),
    editComment: vi.fn(async (_t: NoteTarget, id: string, body: string) => {
      calls.push(`edit:${id}:${body}`);
      rows = rows.map((r) => (r.id === id ? { ...r, body, edited: true } : r));
      return rows.find((r) => r.id === id) as Comment;
    }),
    deleteComment: vi.fn(async (_t: NoteTarget, id: string) => {
      calls.push(`delete:${id}`);
      rows = rows.filter((r) => r.id !== id);
    }),
  };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const dirty: Record<string, () => boolean> = {};
  const held: Record<string, () => boolean> = {};
  const confirmLeave = vi.fn(async (_key?: string) => true);
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

describe("createComments", () => {
  it("opens a target with its comments, members and me", async () => {
    const { comments } = setup();
    await comments.open(target);
    const s = get(comments.state);
    expect(s.rows.map((r) => r.id)).toEqual(["K1"]);
    expect(s.members).toEqual(members);
    expect(s.me).toBe("hanako");
  });

  it("keeps the input when the same target is opened again, and clears it for another target", async () => {
    const { comments } = setup();
    await comments.open(target);
    comments.setNew("書きかけ");
    await comments.open(target);
    expect(get(comments.state).newBody).toBe("書きかけ");
    await comments.open(other);
    expect(get(comments.state).newBody).toBe("");
  });

  it("ignores the result of a request for a target that is no longer open", async () => {
    const { comments, notes } = setup();
    let release: (rows: Comment[]) => void = () => {};
    notes.listComments.mockImplementationOnce(
      () => new Promise<Comment[]>((resolve) => (release = resolve)),
    );
    const slow = comments.open(target);
    notes.listComments.mockImplementationOnce(async () => [comment("K9", "taro", "新")]);
    await comments.open(other);
    release([comment("K1", "hanako", "古い対象")]);
    await slow;
    expect(get(comments.state).target).toEqual(other);
    expect(get(comments.state).rows.map((r) => r.id)).toEqual(["K9"]);
  });

  it("does not clear the input of a new target when a post for the old target finishes", async () => {
    const { comments, notes } = setup();
    await comments.open(target);
    comments.setNew("古い対象");
    let release: () => void = () => {};
    notes.postComment.mockImplementationOnce(
      () =>
        new Promise<Comment>((resolve) => {
          release = () => resolve(comment("K2", "hanako", "古い対象"));
        }),
    );
    const posting = comments.post();
    await comments.open(other);
    comments.setNew("新しい対象");
    release();
    await posting;
    expect(get(comments.state).newBody).toBe("新しい対象");
    expect(get(comments.state).target).toEqual(other);
  });

  it("posts the trimmed body, clears the input and reloads", async () => {
    const { comments, calls } = setup();
    await comments.open(target);
    comments.setNew("  @taro 見てください \n");
    expect(await comments.post()).toBe(true);
    expect(calls).toEqual(["post:@taro 見てください"]);
    const s = get(comments.state);
    expect(s.newBody).toBe("");
    expect(s.rows.map((r) => r.body)).toEqual(["前", "@taro 見てください"]);
  });

  it("does nothing when the body is empty", async () => {
    const { comments, notes } = setup();
    await comments.open(target);
    comments.setNew(" \n ");
    expect(await comments.post()).toBe(false);
    expect(notes.postComment).not.toHaveBeenCalled();
  });

  it("keeps the input and shows the refusal in a toast", async () => {
    const { comments, notes, toasts } = setup();
    notes.postComment.mockRejectedValueOnce(new Refused("コメントを入力してください"));
    await comments.open(target);
    comments.setNew("x");
    expect(await comments.post()).toBe(false);
    expect(get(comments.state).newBody).toBe("x");
    expect(get(toasts.list)[0]).toMatchObject({ text: "コメントを入力してください", error: true });
  });

  it("edits: saves a changed body, and closes without saving an empty or unchanged one", async () => {
    const { comments, calls } = setup();
    await comments.open(target);
    comments.startEdit("K1");
    expect(get(comments.state).editing).toEqual({ id: "K1", body: "前" });
    comments.setEditing("前");
    expect(await comments.saveEdit()).toBe(true);
    expect(get(comments.state).editing).toBeNull();
    comments.startEdit("K1");
    comments.setEditing("  ");
    expect(await comments.saveEdit()).toBe(true);
    expect(get(comments.state).editing).toBeNull();
    expect(calls).toEqual([]);
    comments.startEdit("K1");
    comments.setEditing("後");
    expect(await comments.saveEdit()).toBe(true);
    expect(calls).toEqual(["edit:K1:後"]);
    expect(get(comments.state).rows[0]).toMatchObject({ body: "後", edited: true });
    expect(get(comments.state).editing).toBeNull();
  });

  it("asks before deleting, then deletes, shows the toast and reloads", async () => {
    const { comments, calls, toasts } = setup();
    await comments.open(target);
    comments.askDelete("K1");
    expect(get(comments.state).deleting).toBe("K1");
    comments.cancelDelete();
    expect(get(comments.state).deleting).toBeNull();
    comments.askDelete("K1");
    await comments.confirmDelete();
    expect(calls).toEqual(["delete:K1"]);
    expect(get(comments.state).rows).toEqual([]);
    expect(get(comments.state).deleting).toBeNull();
    expect(get(toasts.list)[0]).toMatchObject({ text: commentDeletedMessage, error: false });
  });

  it("reloads quietly when the comment was already deleted elsewhere", async () => {
    const { comments, notes, toasts, setRows } = setup();
    await comments.open(target);
    notes.deleteComment.mockImplementationOnce(async () => {
      setRows([]);
      throw new NotFound("not found");
    });
    comments.askDelete("K1");
    await comments.confirmDelete();
    expect(get(comments.state).rows).toEqual([]);
    expect(get(toasts.list)).toEqual([]);
  });

  it("registers a draft only for a non-blank new comment and a changed edit", async () => {
    const { comments, dirty, held } = setup();
    await comments.open(target);
    expect(dirty["comment:new:item"]?.()).toBe(false);
    comments.setNew("   ");
    expect(dirty["comment:new:item"]?.()).toBe(false);
    comments.setNew("書きかけ");
    expect(dirty["comment:new:item"]?.()).toBe(true);
    expect(dirty["comment:edit:item"]?.()).toBe(false);
    comments.startEdit("K1");
    expect(dirty["comment:edit:item"]?.()).toBe(false);
    comments.setEditing("");
    expect(dirty["comment:edit:item"]?.()).toBe(true);
    // 空にした編集は、保存と違うが、失うものがないので反映は待たない。
    expect(held["comment:edit:item"]?.()).toBe(false);
    comments.setEditing("変えた");
    expect(held["comment:edit:item"]?.()).toBe(true);
  });

  it("unregisters the drafts when closed", async () => {
    const { comments, dirty } = setup();
    await comments.open(target);
    expect(Object.keys(dirty).sort()).toEqual(["comment:edit:item", "comment:new:item"]);
    comments.close();
    expect(Object.keys(dirty)).toEqual([]);
    await comments.open(target);
    expect(Object.keys(dirty).sort()).toEqual(["comment:edit:item", "comment:new:item"]);
  });

  it("asks before discarding a new comment on Escape, only when there is something to lose", async () => {
    const { comments, confirmLeave } = setup();
    await comments.open(target);
    await comments.escapeNew();
    expect(confirmLeave).not.toHaveBeenCalled();
    comments.setNew("書きかけ");
    confirmLeave.mockResolvedValueOnce(false);
    await comments.escapeNew();
    expect(get(comments.state).newBody).toBe("書きかけ");
    await comments.escapeNew();
    expect(confirmLeave).toHaveBeenCalledTimes(2);
    expect(confirmLeave).toHaveBeenCalledWith("comment:new:item");
    expect(get(comments.state).newBody).toBe("");
  });

  it("closes an unchanged edit on Escape without asking, and asks for a changed one", async () => {
    const { comments, confirmLeave } = setup();
    await comments.open(target);
    comments.startEdit("K1");
    await comments.escapeEdit();
    expect(confirmLeave).not.toHaveBeenCalled();
    expect(get(comments.state).editing).toBeNull();
    comments.startEdit("K1");
    comments.setEditing("変えた");
    confirmLeave.mockResolvedValueOnce(false);
    await comments.escapeEdit();
    expect(get(comments.state).editing).not.toBeNull();
    await comments.escapeEdit();
    expect(confirmLeave).toHaveBeenCalledWith("comment:edit:item");
    expect(get(comments.state).editing).toBeNull();
  });

  it("drops the input and the rows when closed", async () => {
    const { comments } = setup();
    await comments.open(target);
    comments.setNew("x");
    comments.close();
    expect(get(comments.state)).toMatchObject({ target: null, rows: [], newBody: "" });
  });

  it("knows my comments and the avatar of an unknown author", () => {
    expect(isMine(comment("K1", "Hanako", "x"), "hanako")).toBe(true);
    expect(isMine(comment("K1", "taro", "x"), "hanako")).toBe(false);
    expect(isMine(comment("K1", "taro", "x"), "")).toBe(false);
    expect(commentAvatar(comment("K1", "taro", "x"))).toBe("taro");
    expect(commentAvatar(comment("K1", "ghost", "x"))).toBe("");
  });

  it("uses the words of the spec", () => {
    expect(commentPlaceholder).toBe("コメントを書く。@ログイン名 でメンション。Ctrl+Enter で送信");
    expect(commentDeleteDialog).toEqual({
      title: "コメントを削除",
      lines: ["このコメントを削除します。"],
      ok: "削除",
    });
  });
});
