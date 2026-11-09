import { describe, expect, it } from "vitest";
import { ApiError } from "./api/client";
import { diffForRestore, showAt } from "./history";
import { fakeApi, problem, showOf, tv } from "./testing";

describe("showAt", () => {
  it("reads the task at a commit", async () => {
    const { api, calls } = fakeApi(() => showOf(tv("A", "old")));
    const s = await showAt(api, "T-A", "abc123", "shared");
    expect(s.title).toBe("old");
    expect(calls[0]?.path).toBe("/api/tasks/T-A/history/abc123");
    expect(calls[0]?.query.get("store")).toBe("shared");
  });

  it("throws restore_at.not_found when the commit has no such task", async () => {
    const { api } = fakeApi(() => problem(404, "restore_at.not_found", { at: "abc123" }));
    const err = await showAt(api, "T-A", "abc123").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).type).toBe("restore_at.not_found");
  });
});

describe("diffForRestore", () => {
  const base = showOf(
    tv("A", "title", {
      assignees: ["alice"],
      labels: ["ops", "web"],
      priority: "high",
      points: 3,
      props: { area: "web" },
      blocked_by: [{ ulid: "B", id: "T-B", title: "b" }],
    }),
    { body: "text" },
  );

  it("is empty for the same task", () => {
    expect(diffForRestore(base, { ...base })).toEqual([]);
  });

  it("is empty when only the order of a list differs", () => {
    const then = { ...base, labels: ["web", "ops"] };
    expect(diffForRestore(base, then)).toEqual([]);
  });

  it("lists only the fields that differ, as now to after", () => {
    const then = showOf(
      tv("A", "older title", {
        status: "done",
        parent: "P1",
        assignees: [],
        labels: ["ops", "web"],
        priority: "high",
        points: 5,
        due: "2026-11-07",
        props: { area: "web" },
        blocked_by: [],
      }),
      { body: "text" },
    );
    const got = diffForRestore(base, then, {
      idOf: (u) => (u === "P1" ? "REL-1" : u),
      propLabel: (n) => n,
    });
    expect(got).toEqual([
      { field: "title", label: "タイトル", from: "title", to: "older title" },
      { field: "status", label: "ステータス", from: "todo", to: "done" },
      { field: "parent", label: "親", from: "ルート", to: "REL-1" },
      { field: "assignees", label: "担当者", from: "alice", to: "（なし）" },
      { field: "due", label: "期限", from: "（なし）", to: "2026-11-07" },
      { field: "points", label: "ポイント", from: "3", to: "5" },
      { field: "blocked_by", label: "Blocked by", from: "T-B", to: "（なし）" },
    ]);
  });

  it("5 つの関連の差分を、props. なしの名前と表示名で出す", () => {
    const ref = (id: string) => [{ ulid: id, id, title: id }];
    const now = showOf(tv("A", "t", { causes: ref("T-1"), relates: ref("T-2") }));
    const then = showOf(tv("A", "t", { relates: ref("T-2"), clones: ref("T-3") }));
    expect(diffForRestore(now, then)).toEqual([
      { field: "causes", label: "Causes", from: "T-1", to: "（なし）" },
      { field: "clones", label: "Clones", from: "（なし）", to: "T-3" },
    ]);
  });

  it("shows a changed body only as changed, and a prop that goes away as none", () => {
    const then = showOf(tv("A", "title", { ...base, props: {} }), { body: "other" });
    expect(diffForRestore(base, then)).toEqual([
      { field: "props.area", label: "area", from: "web", to: "（なし）" },
      { field: "body", label: "本文", from: "現在の本文", to: "この時点の本文" },
    ]);
  });

  it("lists the built-in properties by their names at the top of the task, with the labels the screen shows", () => {
    const now = showOf(tv("A", "t", { priority: "low", start: "2026-11-01", labels: ["bug"] }));
    const then = showOf(
      tv("A", "t", {
        priority: null,
        start: "2026-11-02",
        estimate: "1h30m",
        labels: ["bug", "ux"],
      }),
    );
    expect(diffForRestore(now, then)).toEqual([
      { field: "priority", label: "優先度", from: "low", to: "（なし）" },
      { field: "start", label: "開始日", from: "2026-11-01", to: "2026-11-02" },
      { field: "estimate", label: "見積もり", from: "（なし）", to: "1h30m" },
      { field: "labels", label: "ラベル", from: "bug", to: "bug、ux" },
    ]);
  });

  it("tells 0 points from none", () => {
    const now = showOf(tv("A", "t", { points: null }));
    const then = showOf(tv("A", "t", { points: 0 }));
    expect(diffForRestore(now, then)).toEqual([
      { field: "points", label: "ポイント", from: "（なし）", to: "0" },
    ]);
  });

  it("joins the items of a list prop", () => {
    const a = showOf(tv("A", "t", { props: { tags: ["x", "y"] } }));
    const b = showOf(tv("A", "t", { props: { tags: ["x"] } }));
    expect(diffForRestore(a, b)).toEqual([
      { field: "props.tags", label: "tags", from: "x、y", to: "x" },
    ]);
  });
});
