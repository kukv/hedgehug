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
      props: { priority: "high", points: 3 },
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
        props: { priority: "high", points: 5, due: "2026-11-07" },
        blocked_by: [],
      }),
      { body: "text" },
    );
    const got = diffForRestore(base, then, {
      idOf: (u) => (u === "P1" ? "REL-1" : u),
      propLabel: (n) => (n === "points" ? "ポイント" : n),
    });
    expect(got).toEqual([
      { field: "title", label: "タイトル", from: "title", to: "older title" },
      { field: "status", label: "ステータス", from: "todo", to: "done" },
      { field: "parent", label: "親", from: "ルート", to: "REL-1" },
      { field: "assignees", label: "担当者", from: "alice", to: "（なし）" },
      { field: "blocked_by", label: "ブロック", from: "T-B", to: "（なし）" },
      { field: "props.due", label: "due", from: "（なし）", to: "2026-11-07" },
      { field: "props.points", label: "ポイント", from: "3", to: "5" },
    ]);
  });

  it("shows a changed body only as changed, and a prop that goes away as none", () => {
    const then = showOf(tv("A", "title", { ...base, props: { points: 3 } }), { body: "other" });
    expect(diffForRestore(base, then)).toEqual([
      { field: "props.priority", label: "priority", from: "high", to: "（なし）" },
      { field: "body", label: "本文", from: "現在の本文", to: "この時点の本文" },
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
