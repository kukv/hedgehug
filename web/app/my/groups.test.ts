import { describe, expect, it } from "vitest";
import type { TaskView } from "../../core/api/client";
import { task } from "../tree/fixtures";
import {
  bucketOf,
  dueDay,
  groupMyTasks,
  loginsOf,
  MY_QUERY,
  NO_PROJECT,
  workspaceCount,
} from "./groups";

// Wednesday 2026-11-04, at an hour that is neither the start nor the end of the day.
const wed = new Date(2026, 10, 4, 15, 30);

function due(ulid: string, date: string | undefined, extra: Partial<TaskView> = {}): TaskView {
  return task(ulid, "task", null, { props: date ? { due: date } : {}, ...extra });
}

describe("MY_QUERY", () => {
  it("asks for the tasks of the person that are not finished", () => {
    expect(MY_QUERY).toBe("assignee:@me -status:done -status:canceled");
  });
});

describe("dueDay", () => {
  it("reads the day of a date, and null for none or for something that is not a date", () => {
    expect(dueDay(due("A", "2026-11-04"))).toBe(dueDay(due("B", "2026-11-04T23:59:00Z")));
    expect((dueDay(due("A", "2026-11-05")) ?? 0) - (dueDay(due("B", "2026-11-04")) ?? 0)).toBe(1);
    expect(dueDay(due("A", undefined))).toBeNull();
    expect(dueDay(due("A", "next week"))).toBeNull();
  });
});

describe("bucketOf: the week begins on Monday", () => {
  const at = (date: string, now = wed) => bucketOf(dueDay(due("A", date)), now);

  it("tells overdue, today, this week and later from a Wednesday", () => {
    expect(at("2026-11-03")).toBe("overdue");
    expect(at("2026-11-04")).toBe("today");
    expect(at("2026-11-05")).toBe("week");
    expect(at("2026-11-08")).toBe("week"); // Sunday is the last day of this week
    expect(at("2026-11-09")).toBe("later"); // Monday begins the next
    expect(bucketOf(null, wed)).toBe("none");
  });

  it("counts Monday as the first day and Sunday as the last", () => {
    const mon = new Date(2026, 10, 2, 0, 5);
    expect(at("2026-11-02", mon)).toBe("today");
    expect(at("2026-11-01", mon)).toBe("overdue"); // the Sunday before is last week
    expect(at("2026-11-08", mon)).toBe("week");
    expect(at("2026-11-09", mon)).toBe("later");
    const sun = new Date(2026, 10, 8, 23, 59);
    expect(at("2026-11-08", sun)).toBe("today");
    expect(at("2026-11-09", sun)).toBe("later"); // nothing is left of the week after today
  });
});

describe("groupMyTasks by due date", () => {
  it("groups in the order overdue, today, this week, later, none, and leaves empty groups out", () => {
    const tasks = [
      due("N", undefined),
      due("L", "2026-11-20"),
      due("W", "2026-11-06"),
      due("T", "2026-11-04"),
      due("O", "2026-10-30"),
    ];
    const groups = groupMyTasks(tasks, "due", wed);
    expect(groups.map((g) => [g.name, g.tasks.map((t) => t.ulid)])).toEqual([
      ["期限切れ", ["O"]],
      ["今日", ["T"]],
      ["今週", ["W"]],
      ["来週以降", ["L"]],
      ["期限なし", ["N"]],
    ]);
    expect(groups.map((g) => g.tone)).toEqual(["alert", "alert", "plain", "plain", "plain"]);
    expect(groupMyTasks([due("N", undefined)], "due", wed).map((g) => g.name)).toEqual([
      "期限なし",
    ]);
    expect(groupMyTasks([], "due", wed)).toEqual([]);
  });

  it("puts the soonest first within a group, and keeps the order of the ones that are equal", () => {
    const tasks = [due("B", "2026-11-07"), due("C", "2026-11-05"), due("A", "2026-11-05")];
    const [week] = groupMyTasks(tasks, "due", wed);
    expect(week?.tasks.map((t) => t.ulid)).toEqual(["C", "A", "B"]);
  });
});

describe("groupMyTasks by project", () => {
  const p = (ulid: string, title: string) => ({ ulid, id: `P-${ulid}`, title });

  it("groups by project, sorted by name, with the tasks outside any project last", () => {
    const tasks = [
      due("1", undefined),
      due("2", "2026-11-06", { project: p("PB", "ビルド") }),
      due("3", undefined, { project: p("PA", "あいうえお") }),
      due("4", "2026-11-05", { project: p("PB", "ビルド") }),
    ];
    const groups = groupMyTasks(tasks, "project", wed);
    expect(groups.map((g) => [g.name, g.tasks.map((t) => t.ulid)])).toEqual([
      ["あいうえお", ["3"]],
      ["ビルド", ["4", "2"]], // the sooner due date first
      [NO_PROJECT, ["1"]],
    ]);
  });

  it("keeps projects of the same name in two stores apart", () => {
    const tasks = [
      due("1", undefined, { project: p("P1", "release") }),
      due("2", undefined, { project: p("P2", "release") }),
    ];
    expect(groupMyTasks(tasks, "project", wed)).toHaveLength(2);
  });
});

describe("workspaceCount", () => {
  it("counts the workspaces the tasks are in, a workspace of another store apart", () => {
    const ws = (store: string, ulid: string) => ({ store, workspace: { ulid } }) as never;
    const tasks = [
      due("1", undefined, ws("shared", "W1")),
      due("2", undefined, ws("shared", "W1")),
      due("3", undefined, ws("shared", "W2")),
      due("4", undefined, ws("me", "W1")),
    ];
    expect(workspaceCount(tasks)).toBe(3);
    expect(workspaceCount([])).toBe(0);
  });
});

describe("loginsOf", () => {
  it("takes the login after the host, once each, and skips a store with no account", () => {
    expect(
      loginsOf([
        { account: "github.com/alice" },
        { account: "github.com/alice" },
        {},
        { account: "ghe.example.com/alice-corp" },
      ]),
    ).toEqual(["alice", "alice-corp"]);
    expect(loginsOf([])).toEqual([]);
  });
});
