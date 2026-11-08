import { describe, expect, it } from "vitest";
import {
  canPlace,
  defaultKindUnder,
  isGroup,
  KINDS,
  kindChoices,
  kindLabel,
  kindsUnder,
  parentRule,
} from "./kinds";

describe("KINDS", () => {
  it("固定の 6 種類を決まった順で並べる", () => {
    expect(KINDS.map((k) => k.name)).toEqual(["project", "epic", "story", "task", "bug", "step"]);
    expect(KINDS.map((k) => k.label)).toEqual(["Project", "Epic", "Story", "Task", "Bug", "Step"]);
  });
});

describe("canPlace", () => {
  it("Go の表と同じ置き場所", () => {
    expect(canPlace("project", null)).toBe(true);
    expect(canPlace("project", "project")).toBe(false);
    expect(canPlace("epic", "project")).toBe(true);
    expect(canPlace("epic", "epic")).toBe(false);
    expect(canPlace("epic", null)).toBe(false);
    for (const k of ["story", "task", "bug"]) {
      expect(canPlace(k, "project")).toBe(true);
      expect(canPlace(k, "epic")).toBe(true);
      expect(canPlace(k, "task")).toBe(false);
      expect(canPlace(k, null)).toBe(false);
    }
    for (const p of ["story", "task", "bug"]) expect(canPlace("step", p)).toBe(true);
    expect(canPlace("step", "project")).toBe(false);
    expect(canPlace("step", "step")).toBe(false);
    expect(canPlace("nowhere", null)).toBe(false);
  });
});

describe("isGroup / kindLabel", () => {
  it("グループは project と epic", () => {
    expect(KINDS.filter((k) => isGroup(k.name)).map((k) => k.name)).toEqual(["project", "epic"]);
    expect(isGroup("nowhere")).toBe(false);
  });

  it("表示名。知らない名前はそのまま", () => {
    expect(kindLabel("epic")).toBe("Epic");
    expect(kindLabel("nowhere")).toBe("nowhere");
  });
});

describe("defaultKindUnder / kindsUnder", () => {
  it("親から既定の種類を決める", () => {
    expect(defaultKindUnder(null)).toBe("project");
    expect(defaultKindUnder("project")).toBe("task");
    expect(defaultKindUnder("epic")).toBe("task");
    for (const p of ["story", "task", "bug"]) expect(defaultKindUnder(p)).toBe("step");
  });

  it("親の下に置ける種類だけを順に返す", () => {
    expect(kindsUnder(null).map((k) => k.name)).toEqual(["project"]);
    expect(kindsUnder("epic").map((k) => k.name)).toEqual(["story", "task", "bug"]);
    expect(kindsUnder("bug").map((k) => k.name)).toEqual(["step"]);
    expect(kindsUnder("step")).toEqual([]);
  });
});

describe("parentRule", () => {
  it("どの下に置けるかを日本語で言う", () => {
    expect(parentRule("project")).toBe("Project は最上位にだけ置けます");
    expect(parentRule("epic")).toBe("Epic を置けるのは Project の下だけです");
    expect(parentRule("task")).toBe("Task を置けるのは Project・Epic の下だけです");
    expect(parentRule("step")).toBe("Step を置けるのは Story・Task・Bug の下だけです");
    expect(parentRule("nowhere")).toBe("nowhere は種類にありません");
  });
});

describe("kindChoices", () => {
  it("置けない種類は disabled で理由を持つ。今の種類は選べたまま", () => {
    const c = kindChoices("task", "story");
    expect(c.map((x) => x.value)).toEqual(["project", "epic", "story", "task", "bug", "step"]);
    expect(c.filter((x) => !x.disabled).map((x) => x.value)).toEqual(["task", "step"]);
    expect(c.find((x) => x.value === "epic")?.title).toBe(parentRule("epic"));
    expect(c.find((x) => x.value === "task")?.title).toBeUndefined();
  });

  it("親の種類が分からないときは何も除かない", () => {
    expect(kindChoices("task", undefined).every((x) => !x.disabled)).toBe(true);
  });
});
