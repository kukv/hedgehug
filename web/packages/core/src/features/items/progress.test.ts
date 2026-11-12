import { describe, expect, it } from "vitest";
import { item } from "./fixtures.test-util";
import { categoryOf, childCount, dueLabel, stepProgress } from "./progress";
import { settings } from "./settings.test-util";

const step = (n: number, status: string) => ({
  ...item("S1", "S1-P1", n, "Step", "S1-I2"),
  status,
});

describe("progress", () => {
  const today = new Date(2026, 10, 13);

  it("finds the category of a status, undefined for an unknown one", () => {
    expect(categoryOf(settings, "st3")).toBe("done");
    expect(categoryOf(settings, "gone")).toBeUndefined();
  });

  it("counts done children and leaves canceled ones out", () => {
    const steps = [step(3, "st3"), step(4, "st1"), step(5, "st4")];
    expect(childCount(steps, settings)).toEqual({ done: 1, total: 2 });
    expect(stepProgress(steps, "S1-I2", settings)).toEqual({ done: 1, total: 2 });
    expect(stepProgress([], "S1-I2", settings)).toBeNull();
  });

  it("labels the due date", () => {
    expect(dueLabel("", "todo", today)).toBeNull();
    expect(dueLabel("2026-11-12", "todo", today)).toEqual({
      text: "期限切れ 11/12",
      emphasis: "overdue",
    });
    expect(dueLabel("2026-11-13", "doing", today)).toEqual({
      text: "今日 11/13",
      emphasis: "today",
    });
    expect(dueLabel("2026-11-12", "done", today)).toEqual({ text: "11/12", emphasis: null });
    expect(dueLabel("2027-01-05", undefined, today)).toEqual({
      text: "2027/01/05",
      emphasis: null,
    });
  });
});
