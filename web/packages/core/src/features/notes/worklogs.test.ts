import { describe, expect, it, vi } from "vitest";
import { NotFound, Refused } from "../../model/errors";
import type { Worklog } from "../../model/notes";
import { get } from "../../reactive/store";
import { createToasts } from "../toasts/toasts";
import { createWorklogs, worklogDeleteDialog, worklogDeletedMessage } from "./worklogs";

function entry(id: string, hours: string, date = "2026-11-13", memo = ""): Worklog {
  return { id, by: "hanako", byName: "花子", date, hours, memo };
}

function setup(initial: Worklog[] = [entry("W1", "1.25", "2026-11-13", "下書き")]) {
  let rows = initial;
  const calls: string[] = [];
  const notes = {
    listWorklogs: vi.fn(async () => rows),
    addWorklog: vi.fn(async (_s: string, _i: string, hours: string, memo: string) => {
      calls.push(`add:${hours}:${memo}`);
      const w = entry(`W${rows.length + 1}`, hours, "2026-11-13", memo);
      rows = [...rows, w];
      return w;
    }),
    deleteWorklog: vi.fn(async (_s: string, _i: string, id: string) => {
      calls.push(`delete:${id}`);
      rows = rows.filter((r) => r.id !== id);
    }),
  };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const worklogs = createWorklogs({ notes, toasts, now: () => new Date(2026, 10, 13, 12, 0, 0) });
  return { worklogs, notes, toasts, calls, setRows: (r: Worklog[]) => (rows = r) };
}

describe("createWorklogs", () => {
  it("opens an item with its worklogs, and keeps the input for the same item", async () => {
    const { worklogs } = setup();
    await worklogs.open("S1", "I1");
    expect(get(worklogs.state).rows.map((r) => r.id)).toEqual(["W1"]);
    worklogs.setHours("2");
    await worklogs.open("S1", "I1");
    expect(get(worklogs.state).hours).toBe("2");
    await worklogs.open("S1", "I2");
    expect(get(worklogs.state)).toMatchObject({ hours: "", memo: "", error: "" });
  });

  it("records the hours as typed with the memo, clears the inputs and reloads", async () => {
    const { worklogs, calls } = setup();
    await worklogs.open("S1", "I1");
    worklogs.setHours(" 0.5 ");
    worklogs.setMemo(" レビュー ");
    expect(await worklogs.add()).toBe(true);
    expect(calls).toEqual(["add:0.5:レビュー"]);
    const s = get(worklogs.state);
    expect(s).toMatchObject({ hours: "", memo: "", error: "" });
    expect(s.rows.map((r) => r.hours)).toEqual(["1.25", "0.5"]);
  });

  it("shows the refusal under the field without sending when the hours are bad", async () => {
    const { worklogs, notes } = setup();
    await worklogs.open("S1", "I1");
    for (const [input, message] of [
      ["", "時間を入力してください"],
      ["0", "時間を入力してください"],
      ["abc", "時間を入力してください"],
      ["0.123", "時間は小数第 2 位までで入力してください"],
    ]) {
      worklogs.setHours(input as string);
      expect(await worklogs.add()).toBe(false);
      expect(get(worklogs.state).error).toBe(message);
    }
    expect(notes.addWorklog).not.toHaveBeenCalled();
    worklogs.setHours("1");
    expect(get(worklogs.state).error).toBe("");
  });

  it("shows a refusal from the server under the field and keeps the input", async () => {
    const { worklogs, notes } = setup();
    notes.addWorklog.mockRejectedValueOnce(new Refused("時間を入力してください"));
    await worklogs.open("S1", "I1");
    worklogs.setHours("1");
    expect(await worklogs.add()).toBe(false);
    expect(get(worklogs.state)).toMatchObject({ hours: "1", error: "時間を入力してください" });
  });

  it("asks with the date and hours, then deletes, shows the toast and reloads", async () => {
    const { worklogs, calls, toasts } = setup();
    await worklogs.open("S1", "I1");
    worklogs.askDelete("W1");
    expect(get(worklogs.state).deleting).toEqual({
      id: "W1",
      lines: ["11/13 の 1.25 時間の記録を削除します。"],
    });
    worklogs.cancelDelete();
    expect(get(worklogs.state).deleting).toBeNull();
    worklogs.askDelete("W1");
    await worklogs.confirmDelete();
    expect(calls).toEqual(["delete:W1"]);
    expect(get(worklogs.state)).toMatchObject({ rows: [], deleting: null });
    expect(get(toasts.list)[0]).toMatchObject({ text: worklogDeletedMessage, error: false });
  });

  it("writes the year of a record from another year", async () => {
    const { worklogs } = setup([entry("W1", "2", "2025-12-31")]);
    await worklogs.open("S1", "I1");
    worklogs.askDelete("W1");
    expect(get(worklogs.state).deleting?.lines).toEqual([
      "2025/12/31 の 2 時間の記録を削除します。",
    ]);
  });

  it("reloads quietly when the record was already deleted elsewhere", async () => {
    const { worklogs, notes, toasts, setRows } = setup();
    await worklogs.open("S1", "I1");
    notes.deleteWorklog.mockImplementationOnce(async () => {
      setRows([]);
      throw new NotFound("not found");
    });
    worklogs.askDelete("W1");
    await worklogs.confirmDelete();
    expect(get(worklogs.state).rows).toEqual([]);
    expect(get(toasts.list)).toEqual([]);
  });

  it("drops everything when closed", async () => {
    const { worklogs } = setup();
    await worklogs.open("S1", "I1");
    worklogs.setHours("2");
    worklogs.close();
    expect(get(worklogs.state)).toMatchObject({ target: null, rows: [], hours: "" });
  });

  it("uses the words of the spec", () => {
    expect(worklogDeleteDialog).toEqual({ title: "作業時間を削除", ok: "削除" });
    expect(worklogDeletedMessage).toBe("作業時間を削除しました");
  });
});
