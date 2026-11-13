import { describe, expect, it, vi } from "vitest";
import { NotFound } from "../../model/errors";
import type { Comment } from "../../model/notes";
import { get } from "../../reactive/store";
import { commentFailedMessage } from "./comments";
import { setup as baseSetup, comment, other, target } from "./commentsFixtures.test-util";

const setup = (rows?: Comment[]) => baseSetup(vi.fn, rows);

describe("comments: failures and races", () => {
  it("reloads quietly when posting to a target that no longer exists", async () => {
    const { comments, notes, toasts } = setup();
    await comments.open(target);
    notes.postComment.mockRejectedValueOnce(new NotFound("gone"));
    comments.setNew("x");
    expect(await comments.post()).toBe(false);
    expect(notes.listComments).toHaveBeenCalledTimes(2);
    expect(get(toasts.list)).toEqual([]);
  });

  it("reloads quietly when editing a comment that was deleted elsewhere", async () => {
    const { comments, notes, toasts, setRows } = setup();
    await comments.open(target);
    comments.startEdit("K1");
    comments.setEditing("後");
    setRows([]);
    notes.editComment.mockRejectedValueOnce(new NotFound("gone"));
    expect(await comments.saveEdit()).toBe(false);
    expect(get(comments.state).rows).toEqual([]);
    expect(get(comments.state).editing).toBeNull();
    expect(get(toasts.list)).toEqual([]);
  });

  it("shows an error toast for a failure that is not a refusal, and keeps the input", async () => {
    const { comments, notes, toasts } = setup();
    await comments.open(target);
    notes.postComment.mockRejectedValueOnce(new Error("network"));
    comments.setNew("x");
    expect(await comments.post()).toBe(false);
    expect(get(comments.state).newBody).toBe("x");
    expect(get(comments.state).busy).toBe(false);
    expect(get(toasts.list)[0]).toMatchObject({ text: commentFailedMessage, error: true });
  });

  it("does not let a finished request for the old target release the busy flag of the new one", async () => {
    const { comments, notes } = setup();
    await comments.open(target);
    comments.setNew("A");
    let releaseA: () => void = () => {};
    notes.postComment.mockImplementationOnce(
      () =>
        new Promise<Comment>((resolve) => (releaseA = () => resolve(comment("K2", "hanako", "A")))),
    );
    const postingA = comments.post();
    await comments.open(other);
    comments.setNew("B");
    let releaseB: () => void = () => {};
    notes.postComment.mockImplementationOnce(
      () =>
        new Promise<Comment>((resolve) => (releaseB = () => resolve(comment("K3", "hanako", "B")))),
    );
    const postingB = comments.post();
    releaseA();
    await postingA;
    expect(get(comments.state).busy).toBe(true);
    expect(await comments.post()).toBe(false);
    releaseB();
    await postingB;
    expect(get(comments.state).busy).toBe(false);
  });

  it("drops an edit or a delete question for a comment that disappeared on reload", async () => {
    const { comments, setRows } = setup();
    await comments.open(target);
    comments.startEdit("K1");
    comments.setEditing("書きかけ");
    comments.askDelete("K1");
    setRows([]);
    await comments.reload();
    expect(get(comments.state).editing).toBeNull();
    expect(get(comments.state).deleting).toBeNull();
  });

  it("keeps what was typed while a post was in flight", async () => {
    const { comments, notes } = setup();
    await comments.open(target);
    comments.setNew("送る");
    let release: () => void = () => {};
    notes.postComment.mockImplementationOnce(
      () =>
        new Promise<Comment>(
          (resolve) => (release = () => resolve(comment("K2", "hanako", "送る"))),
        ),
    );
    const posting = comments.post();
    comments.setNew("送る。続き");
    release();
    await posting;
    expect(get(comments.state).newBody).toBe("送る。続き");
  });

  it("keeps what was typed while an edit save was in flight", async () => {
    const { comments, notes } = setup();
    await comments.open(target);
    comments.startEdit("K1");
    comments.setEditing("後");
    let release: () => void = () => {};
    notes.editComment.mockImplementationOnce(
      () =>
        new Promise<Comment>(
          (resolve) => (release = () => resolve(comment("K1", "hanako", "後", true))),
        ),
    );
    const saving = comments.saveEdit();
    comments.setEditing("後。続き");
    release();
    await saving;
    expect(get(comments.state).editing).toEqual({ id: "K1", body: "後。続き" });
  });
});
