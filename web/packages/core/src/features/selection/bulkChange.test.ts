import { describe, expect, it } from "vitest";
import type { BulkChangeResult } from "../../model/bulkChange";
import { Refused } from "../../model/errors";
import { get } from "../../reactive/store";
import { fakeItemActions } from "../items/actions.test-util";
import { flush, gate } from "../items/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { type BulkChangeForm, canSubmit, createBulkChange } from "./bulkChange";
import { offBacklogText } from "./bulkChangeResult";
import { createRowSelection, type SelectedRow } from "./rowSelection";

const row = (storeId: string, itemId: string, display: string): SelectedRow => ({
  storeId,
  itemId,
  display,
  readOnly: false,
});

const result = (over: Partial<BulkChangeResult> = {}): BulkChangeResult => ({
  changed: 0,
  kept: [],
  failed: [],
  offBacklog: { done: 0, canceled: 0 },
  valueText: "",
  ...over,
});

function setup() {
  const actions = fakeItemActions();
  actions.choices = {
    kinds: ["Task"],
    statuses: ["Todo", "Done"],
    priorities: [],
    sprints: [],
    releases: [],
    systems: ["Web", "API"],
    members: [],
  };
  const toasts = createToasts({ set: () => 0, clear: () => {} });
  const calls: string[] = [];
  const bc = createBulkChange({
    actions,
    toasts,
    detail: {
      reload: async () => {
        calls.push("reload");
      },
    },
  });
  const selection = createRowSelection();
  selection.toggle(row("S1", "a", "HH-1"), true);
  selection.toggle(row("S1", "b", "HH-2"), true);
  const after = async () => {
    calls.push("after");
  };
  const messages = () => get(toasts.list).map((t) => `${t.error ? "!" : ""}${t.text}`);
  const form = () => get(bc.state) as BulkChangeForm;
  return { bc, actions, selection, after, calls, messages, form };
}

describe("createBulkChange", () => {
  it("opens with status", async () => {
    const t = setup();
    await t.bc.open(t.selection, { after: t.after });
    expect(t.actions.calls).toEqual(["bulkChangeChoices S1/a,S1/b"]);
    expect(t.form().stage).toBe("form");
    expect(t.form().field).toBe("status");
    expect(t.form().option).toBe("Todo");
    expect(t.form().summary).toBe("2 件(Task)");
  });

  it("fixed at open", async () => {
    const t = setup();
    await t.bc.open(t.selection, { after: t.after });
    t.selection.toggle(row("S1", "c", "HH-3"), true);
    await t.bc.submit();
    expect(t.actions.calls[1]).toContain("bulkChangePreview S1/a,S1/b ");
  });

  it("field change resets the value", async () => {
    const t = setup();
    await t.bc.open(t.selection, { after: t.after });
    t.bc.setOption("Done");
    t.bc.setField("points");
    expect(t.form().option).toBe("");
    expect(t.form().text).toBe("");
    expect(t.form().error).toBeNull();
  });

  it("refusal stays in the dialog", async () => {
    const t = setup();
    await t.bc.open(t.selection, { after: t.after });
    t.bc.setField("points");
    t.bc.setText("-1");
    t.actions.bulkChangePreview = async () => {
      throw new Refused("ポイントは 0 以上の数で入力してください");
    };
    await t.bc.submit();
    expect(t.form().stage).toBe("form");
    expect(t.form().error).toBe("ポイントは 0 以上の数で入力してください");
    expect(t.form().busy).toBe(false);
  });

  it("nothing changes", async () => {
    const t = setup();
    t.actions.changePreview = result({
      kept: [{ store: "S1", item: "a", reason: "同じ名前の値がありません" }],
    });
    await t.bc.open(t.selection, { after: t.after });
    await t.bc.submit();
    expect(t.messages()).toEqual(["!変更できたものはありません: HH-1(同じ名前の値がありません)"]);
    expect(get(t.bc.state)).toBeNull();
    expect(t.actions.calls.some((c) => c.startsWith("bulkChange "))).toBe(false);
    expect(t.calls).not.toContain("after");
  });

  it("confirm runs and keeps the selection", async () => {
    const t = setup();
    t.actions.changePreview = result({ changed: 2, valueText: "Done" });
    t.actions.changeResult = result({ changed: 2 });
    await t.bc.open(t.selection, { after: t.after });
    t.bc.setOption("Done");
    await t.bc.submit();
    expect(get(t.bc.state)).toEqual({
      stage: "confirm",
      title: "一括変更",
      lines: ["2 件のステータスを Done に変更します。"],
      ok: "2 件を変更",
    });
    await t.bc.confirm();
    expect(t.actions.calls[2]).toBe(
      `bulkChange S1/a,S1/b ${JSON.stringify({ field: "status", value: "Done", values: [], category: "" })}`,
    );
    expect(t.messages()).toEqual(["2 件を変えました"]);
    expect(get(t.bc.state)).toBeNull();
    expect(get(t.selection.rows)).toHaveLength(2);
    expect(t.calls.filter((c) => c === "after")).toHaveLength(1);
    expect(t.calls.filter((c) => c === "reload")).toHaveLength(1);
  });

  it("suffix", async () => {
    const t = setup();
    t.actions.changePreview = result({ changed: 1 });
    t.actions.changeResult = result({ changed: 1, offBacklog: { done: 1, canceled: 0 } });
    await t.bc.open(t.selection, { after: t.after, suffix: offBacklogText });
    await t.bc.submit();
    await t.bc.confirm();
    expect(t.messages()).toEqual([
      "1 件を変えました。1 件は完了のため、バックログには表示されません",
    ]);
  });

  it("run failure", async () => {
    const t = setup();
    t.actions.changePreview = result({ changed: 1 });
    await t.bc.open(t.selection, { after: t.after });
    await t.bc.submit();
    t.actions.failNext();
    await t.bc.confirm();
    expect(t.messages()).toEqual(["!保存できませんでした"]);
    expect(get(t.bc.state)).toBeNull();
    expect(t.calls).toContain("after");
  });

  it("cancel while loading", async () => {
    const t = setup();
    const g = gate();
    t.actions.bulkChangeChoices = async () => {
      await g.wait;
      return t.actions.choices;
    };
    const p = t.bc.open(t.selection, { after: t.after });
    await flush();
    t.bc.cancel();
    g.release();
    await p;
    expect(get(t.bc.state)).toBeNull();
  });

  it("systems", async () => {
    const t = setup();
    await t.bc.open(t.selection, { after: t.after });
    t.bc.setField("systemsAdd");
    expect(canSubmit(t.form())).toBe(false);
    t.bc.toggleValue("Web");
    expect(canSubmit(t.form())).toBe(true);
    t.bc.toggleValue("Web");
    expect(canSubmit(t.form())).toBe(false);
  });

  it("cancel while previewing", async () => {
    const t = setup();
    await t.bc.open(t.selection, { after: t.after });
    const g = gate();
    t.actions.bulkChangePreview = async () => {
      await g.wait;
      return result({ changed: 1 });
    };
    const p = t.bc.submit();
    await flush();
    t.bc.cancel();
    g.release();
    await p;
    expect(get(t.bc.state)).toBeNull();
  });

  it("submit twice previews once", async () => {
    const t = setup();
    await t.bc.open(t.selection, { after: t.after });
    const g = gate();
    let n = 0;
    t.actions.bulkChangePreview = async () => {
      n += 1;
      await g.wait;
      return result({ changed: 1 });
    };
    const p1 = t.bc.submit();
    const p2 = t.bc.submit();
    g.release();
    await Promise.all([p1, p2]);
    expect(n).toBe(1);
  });

  it("preview failure closes and reloads", async () => {
    const t = setup();
    await t.bc.open(t.selection, { after: t.after });
    t.actions.bulkChangePreview = async () => {
      throw new Error("boom");
    };
    await t.bc.submit();
    expect(t.messages()).toEqual(["!保存できませんでした"]);
    expect(get(t.bc.state)).toBeNull();
    expect(t.calls.filter((c) => c === "reload")).toHaveLength(1);
    expect(t.calls.filter((c) => c === "after")).toHaveLength(1);
  });

  it("confirm twice runs once", async () => {
    const t = setup();
    t.actions.changePreview = result({ changed: 1 });
    await t.bc.open(t.selection, { after: t.after });
    await t.bc.submit();
    const h = t.actions.holdNext();
    const p1 = t.bc.confirm();
    const p2 = t.bc.confirm();
    await flush();
    h.release();
    await Promise.all([p1, p2]);
    expect(t.actions.calls.filter((c) => c.startsWith("bulkChange "))).toHaveLength(1);
  });

  it("no options cannot submit", async () => {
    const t = setup();
    await t.bc.open(t.selection, { after: t.after });
    t.bc.setField("priority");
    expect(t.form().options).toEqual([]);
    expect(canSubmit(t.form())).toBe(false);
  });

  it("cancel during run then reopen keeps the new form", async () => {
    const t = setup();
    t.actions.changePreview = result({ changed: 1 });
    await t.bc.open(t.selection, { after: t.after });
    await t.bc.submit();
    const h = t.actions.holdNext();
    const p = t.bc.confirm();
    await flush();
    t.bc.cancel();
    await t.bc.open(t.selection, { after: t.after });
    h.release();
    await p;
    expect(get(t.bc.state)?.stage).toBe("form");
    expect(t.calls).toContain("after");
  });
});
