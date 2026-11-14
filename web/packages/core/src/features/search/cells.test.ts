import { describe, expect, it } from "vitest";
import { cellView, hoursText } from "./cells";
import type { SearchColumn, SearchRow } from "./types";

const now = new Date(2026, 10, 15, 12, 0, 0);

function row(over: Partial<SearchRow> = {}): SearchRow {
  return {
    storeId: "s",
    storeName: "s",
    storeColor: "",
    workspaceId: "w",
    projectId: "p",
    itemId: "i",
    display: "HH-1",
    title: "t",
    kind: "task",
    status: { name: "Todo", category: "todo" },
    assignee: null,
    priority: null,
    end: "",
    points: null,
    oldIdHit: false,
    readOnly: false,
    deleted: null,
    start: "",
    estimate: null,
    systems: [],
    sprint: "",
    release: "",
    project: { display: "HH", title: "P" },
    parent: null,
    created: now.toISOString(),
    updated: now.toISOString(),
    unused: [],
    removed: [],
    custom: {},
    ...over,
  };
}

const col = (key: string, type = ""): SearchColumn => ({ key, name: key, type, sortable: true });

describe("cellView", () => {
  it("unused wins, then removed", () => {
    const r = row({ unused: ["priority"], removed: ["priority"], priority: { name: "高" } });
    expect(cellView(r, col("priority"), now)).toEqual({ kind: "unused" });
    expect(cellView(row({ removed: ["priority"] }), col("priority"), now)).toEqual({
      kind: "removed",
    });
  });

  it("priority shows its name", () => {
    expect(cellView(row({ priority: { name: "高" } }), col("priority"), now)).toEqual({
      kind: "text",
      text: "高",
    });
    expect(cellView(row(), col("priority"), now)).toEqual({ kind: "empty" });
  });

  it("end uses the due label", () => {
    expect(cellView(row({ end: "2026-11-10" }), col("end"), now)).toEqual({
      kind: "due",
      text: "期限切れ 11/10",
      emphasis: "overdue",
    });
    const done = row({ end: "2026-11-10", status: { name: "Done", category: "done" } });
    expect(cellView(done, col("end"), now)).toEqual({ kind: "due", text: "11/10", emphasis: null });
    expect(cellView(row(), col("end"), now)).toEqual({ kind: "empty" });
  });

  it("start is a formatted day", () => {
    expect(cellView(row({ start: "2027-01-05" }), col("start"), now)).toEqual({
      kind: "text",
      text: "2027/01/05",
    });
    expect(cellView(row(), col("start"), now)).toEqual({ kind: "empty" });
  });

  it("points and estimate", () => {
    expect(cellView(row({ points: 3 }), col("points"), now)).toEqual({ kind: "text", text: "3" });
    expect(cellView(row({ estimate: 1.255 }), col("estimate"), now)).toEqual({
      kind: "text",
      text: "1.26 時間",
    });
    expect(cellView(row({ estimate: 2 }), col("estimate"), now)).toEqual({
      kind: "text",
      text: "2 時間",
    });
    expect(cellView(row(), col("estimate"), now)).toEqual({ kind: "empty" });
  });

  it("system, sprint, release, project", () => {
    expect(cellView(row({ systems: ["A", "B"] }), col("system"), now)).toEqual({
      kind: "text",
      text: "A, B",
    });
    expect(cellView(row(), col("system"), now)).toEqual({ kind: "empty" });
    expect(cellView(row(), col("sprint"), now)).toEqual({ kind: "empty" });
    expect(cellView(row({ release: "v1" }), col("release"), now)).toEqual({
      kind: "text",
      text: "v1",
    });
    expect(cellView(row(), col("project"), now)).toEqual({ kind: "text", text: "P" });
  });

  it("created and updated are relative", () => {
    const at = new Date(now.getTime() - 3 * 3600_000).toISOString();
    expect(cellView(row({ created: at }), col("created"), now)).toEqual({
      kind: "relative",
      text: "3 時間前",
    });
    expect(cellView(row({ updated: at }), col("updated"), now)).toEqual({
      kind: "relative",
      text: "3 時間前",
    });
  });

  it("custom cells use the cell's own type", () => {
    const due = col("p:due", "date");
    const text = row({ custom: { "p:due": { text: "来週", type: "text" } } });
    expect(cellView(text, due, now)).toEqual({ kind: "text", text: "来週" });
    const date = row({ custom: { "p:due": { text: "2026-12-01", type: "date" } } });
    expect(cellView(date, due, now)).toEqual({ kind: "text", text: "12/1" });
    const user = row({ custom: { "p:owner": { text: "田中", type: "user" } } });
    expect(cellView(user, col("p:owner", "user"), now)).toEqual({ kind: "person", name: "田中" });
    const chk = row({ custom: { "p:done": { text: "true", type: "check" } } });
    expect(cellView(chk, col("p:done", "check"), now)).toEqual({ kind: "check", checked: true });
    const url = row({ custom: { "p:site": { text: "https://example.com", type: "url" } } });
    expect(cellView(url, col("p:site", "url"), now)).toEqual({
      kind: "url",
      href: "https://example.com",
    });
    expect(cellView(row(), col("p:none", "text"), now)).toEqual({ kind: "empty" });
  });
});

describe("hoursText", () => {
  it("rounds to 2 decimals and drops trailing zeros", () => {
    expect(hoursText(0.5)).toBe("0.5");
    expect(hoursText(1.005)).toBe("1.01");
    expect(hoursText(3)).toBe("3");
  });
});
