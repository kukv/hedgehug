import { describe, expect, it } from "vitest";
import { initialValues, shownValues, switchWorkspace } from "./createValues";
import { settings, settings2 } from "./settings.test-util";

describe("create values", () => {
  it("starts with the initial status and priority", () => {
    expect(initialValues(settings)).toEqual({ status: "st1", priority: "pr2" });
  });

  it("keeps assignee, dates, points and estimate when the workspace changes", () => {
    const values = {
      status: "st2",
      priority: "pr1",
      assignee: "taro",
      start: "2026-11-11",
      estimate: "2",
      systems: ["sy1"],
      F1: "C1",
    };
    expect(switchWorkspace(values, settings2, () => true)).toEqual({
      status: "ns1",
      priority: "np1",
      assignee: "taro",
      start: "2026-11-11",
      estimate: "2",
    });
  });

  it("drops the assignee who is not a member of the new store", () => {
    expect(switchWorkspace({ assignee: "taro" }, settings2, () => false).assignee).toBeUndefined();
  });

  it("sends only the values shown for the kind", () => {
    const values = { status: "st1", priority: "pr2", points: "3", end: "2026-11-20", F2: "" };
    expect(shownValues(values, settings, "Task")).toEqual({ status: "st1", priority: "pr2" });
    expect(shownValues(values, settings, "Story")).toEqual({
      status: "st1",
      priority: "pr2",
      points: "3",
    });
  });
});
