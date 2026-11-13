import { describe, expect, it, vi } from "vitest";
import type { Comment } from "../../model/notes";
import { get } from "../../reactive/store";
import { setup as baseSetup, target } from "./commentsFixtures.test-util";

const setup = (rows?: Comment[]) => baseSetup(vi.fn, rows);

describe("comments: drafts", () => {
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
});
