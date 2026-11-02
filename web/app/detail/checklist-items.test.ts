import { describe, expect, it } from "vitest";
import { checklistItems, toggleChecklistLine } from "./checklist-items";

describe("checklistItems", () => {
  it("lists the checkbox lines with their line numbers", () => {
    const body = "intro\n- [ ] a\n* [x] b\n+ [X] c\n1. [ ] d\n2) [x] e\n- plain\n";
    expect(checklistItems(body)).toEqual([
      { line: 1, checked: false, text: "a" },
      { line: 2, checked: true, text: "b" },
      { line: 3, checked: true, text: "c" },
      { line: 4, checked: false, text: "d" },
      { line: 5, checked: true, text: "e" },
    ]);
  });

  it("counts nested items and ignores things that are not checkboxes", () => {
    expect(
      checklistItems("- [ ] a\n  - [x] b\n- [y] c\n- [] d\n- [ x] e\n- [x]f\n").map((i) => i.line),
    ).toEqual([0, 1]);
  });

  it("skips fenced code, and an unclosed fence hides the rest", () => {
    const body = "- [ ] a\n```\n- [x] in code\n```\n~~~md\n- [ ] in code\n~~~\n- [x] b\n";
    expect(checklistItems(body).map((i) => i.line)).toEqual([0, 7]);
    expect(checklistItems("- [x] a\n```\n- [ ] b\n").map((i) => i.line)).toEqual([0]);
    expect(checklistItems("````\n```\n- [ ] in code\n````\n- [ ] a\n").map((i) => i.line)).toEqual([
      4,
    ]);
  });

  it("reads CRLF bodies", () => {
    expect(checklistItems("- [ ] a\r\n- [x] b\r\n")).toEqual([
      { line: 0, checked: false, text: "a" },
      { line: 1, checked: true, text: "b" },
    ]);
  });
});

describe("toggleChecklistLine", () => {
  it("flips only the given line, even when two lines read the same", () => {
    const body = "- [ ] same\n- [ ] same\n";
    expect(toggleChecklistLine(body, 1)).toBe("- [ ] same\n- [x] same\n");
    expect(toggleChecklistLine("- [x] a\n", 0)).toBe("- [ ] a\n");
    expect(toggleChecklistLine("- [X] a\n", 0)).toBe("- [ ] a\n");
  });

  it("keeps CRLF and leaves the body alone when the line is not a checkbox", () => {
    expect(toggleChecklistLine("- [ ] a\r\n- [ ] b\r\n", 0)).toBe("- [x] a\r\n- [ ] b\r\n");
    expect(toggleChecklistLine("text [ ] here\n- [ ] a\n", 0)).toBe("text [ ] here\n- [ ] a\n");
    expect(toggleChecklistLine("```\n- [ ] a\n```\n", 1)).toBe("```\n- [ ] a\n```\n");
    expect(toggleChecklistLine("- [ ] a\n", 9)).toBe("- [ ] a\n");
  });
});
