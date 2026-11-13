import { describe, expect, it, vi } from "vitest";
import { NotFound } from "../../model/errors";
import type { Worklog } from "../../model/notes";
import { get } from "../../reactive/store";
import { createToasts } from "../toasts/toasts";
import { createWorklogs, worklogFailedMessage } from "./worklogs";

const entry = (id: string, hours = "1"): Worklog => ({
  id,
  by: "hanako",
  byName: "花子",
  date: "2026-11-13",
  hours,
  memo: "",
});

function setup() {
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const notes = {
    listWorklogs: vi.fn(async (_s: string, _i: string): Promise<Worklog[]> => [entry("W1")]),
    addWorklog: vi.fn(async (_s: string, _i: string, hours: string) => entry("W2", hours)),
    deleteWorklog: vi.fn(async (_s: string, _i: string, _id: string) => {}),
  };
  const worklogs = createWorklogs({ notes, toasts, now: () => new Date(2026, 10, 13) });
  return { worklogs, notes, toasts };
}

describe("createWorklogs failures", () => {
  it("ignores a list that arrives after another item was opened", async () => {
    const { worklogs, notes } = setup();
    let release: (rows: Worklog[]) => void = () => {};
    notes.listWorklogs.mockImplementationOnce(() => new Promise((r) => (release = r)));
    const first = worklogs.open("S1", "I1");
    await worklogs.open("S1", "I2");
    release([entry("OLD")]);
    await first;
    expect(get(worklogs.state).rows.map((r) => r.id)).toEqual(["W1"]);
    expect(get(worklogs.state).target).toEqual({ storeId: "S1", itemId: "I2" });
  });

  it("does not send twice while one is in flight", async () => {
    const { worklogs, notes } = setup();
    await worklogs.open("S1", "I1");
    let release: () => void = () => {};
    notes.addWorklog.mockImplementationOnce(
      () => new Promise((r) => (release = () => r(entry("W2")))),
    );
    worklogs.setHours("1");
    const first = worklogs.add();
    expect(await worklogs.add()).toBe(false);
    release();
    expect(await first).toBe(true);
    expect(notes.addWorklog).toHaveBeenCalledTimes(1);
    expect(get(worklogs.state).busy).toBe(false);
  });

  it("keeps the busy mark of the item that is open now when an old send finishes", async () => {
    const { worklogs, notes } = setup();
    await worklogs.open("S1", "I1");
    let release: () => void = () => {};
    notes.addWorklog.mockImplementationOnce(
      () => new Promise((r) => (release = () => r(entry("W2")))),
    );
    worklogs.setHours("1");
    const sending = worklogs.add();
    await worklogs.open("S1", "I2");
    worklogs.setHours("3");
    release();
    await sending;
    expect(get(worklogs.state)).toMatchObject({ hours: "3", memo: "", error: "" });
  });

  it("keeps what was typed while sending", async () => {
    const { worklogs, notes } = setup();
    await worklogs.open("S1", "I1");
    let release: () => void = () => {};
    notes.addWorklog.mockImplementationOnce(
      () => new Promise((r) => (release = () => r(entry("W2")))),
    );
    worklogs.setHours("1");
    const sending = worklogs.add();
    worklogs.setMemo("あとから");
    release();
    await sending;
    expect(get(worklogs.state)).toMatchObject({ hours: "1", memo: "あとから" });
  });

  it("reloads quietly when the item is gone on add, and shows an error toast for other failures", async () => {
    const { worklogs, notes, toasts } = setup();
    await worklogs.open("S1", "I1");
    worklogs.setHours("1");
    notes.addWorklog.mockRejectedValueOnce(new NotFound("gone"));
    expect(await worklogs.add()).toBe(false);
    expect(get(toasts.list)).toEqual([]);
    expect(get(worklogs.state)).toMatchObject({ busy: false, hours: "1" });
    notes.addWorklog.mockRejectedValueOnce(new Error("network"));
    expect(await worklogs.add()).toBe(false);
    expect(get(toasts.list)[0]).toMatchObject({ text: worklogFailedMessage, error: true });
    expect(get(worklogs.state)).toMatchObject({ busy: false, hours: "1", error: "" });
  });

  it("shows an error toast when the delete fails for another reason", async () => {
    const { worklogs, notes, toasts } = setup();
    await worklogs.open("S1", "I1");
    notes.deleteWorklog.mockRejectedValueOnce(new Error("network"));
    worklogs.askDelete("W1");
    await worklogs.confirmDelete();
    expect(get(toasts.list)[0]).toMatchObject({ text: worklogFailedMessage, error: true });
    expect(get(worklogs.state).deleting).toBeNull();
  });

  it("drops the delete question when a reload shows the record is gone", async () => {
    const { worklogs, notes } = setup();
    await worklogs.open("S1", "I1");
    worklogs.askDelete("W1");
    notes.listWorklogs.mockResolvedValueOnce([]);
    await worklogs.reload();
    expect(get(worklogs.state).deleting).toBeNull();
  });
});
