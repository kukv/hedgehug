import { describe, expect, it } from "vitest";
import { caretTerm } from "./caret";

// at は、`|` をカーソルとして、その位置の語を読む。
const at = (marked: string) => {
  const caret = marked.indexOf("|");
  return caretTerm(marked.replace("|", ""), caret);
};

describe("caretTerm", () => {
  it("reads the key, the typed value and where it starts", () => {
    expect(at("assignee:tanaka,すず|")).toEqual({
      key: "assignee",
      not: false,
      start: 16,
      end: 18,
      typed: "すず",
    });
  });

  it("reads a negated key and a quote not yet closed", () => {
    expect(at('-parent:"HH|')).toMatchObject({ key: "parent", not: true, typed: "HH" });
  });

  it("lowercases the key and reads comparison operators", () => {
    expect(at("Status:in|")).toMatchObject({ key: "status", typed: "in", start: 7 });
    expect(at("points>=3|")).toMatchObject({ key: "points", typed: "3", start: 8 });
    expect(at("end<to|day")).toMatchObject({ key: "end", typed: "to", start: 4, end: 6 });
  });

  it("has an empty typed value right after the colon or a comma", () => {
    expect(at("status:|")).toMatchObject({ key: "status", typed: "", start: 7, end: 7 });
    expect(at("kind:story,|")).toMatchObject({ key: "kind", typed: "", start: 11 });
  });

  it("splits values at a comma even inside quotes, and trims spaces", () => {
    expect(at('release:"v2, v3|')).toMatchObject({ typed: "v3" });
  });

  it("finds the term the caret is on among many", () => {
    expect(at("kind:bug stat|us:done")).toBeNull();
    expect(at("kind:bug status:d|one")).toMatchObject({ key: "status", typed: "d", start: 16 });
    expect(at("kind:bug| status:done")).toMatchObject({ key: "kind", typed: "bug" });
  });

  // internal/domain/query/parse_test.go の TestParse と同じ入力。
  it.each([
    ["http://x", "http"],
    ["v2<v3", "v2"],
    ["status:,", "status"],
    ["-status:", "status"],
    ["-updated:none", "updated"],
    ["Status:done", "status"],
    ['release:"v2.0 beta', "release"],
    ["points>=3", "points"],
  ])("reads the key of %s as %s", (text, key) => {
    expect(caretTerm(text, text.length)?.key).toBe(key);
  });

  it.each([["C++>11"], ["- foo"], ['"status:x"'], ['"ガント 表"'], ['"" " " foo'], ["foo"], [""]])(
    "has no key for %s",
    (text) => {
      expect(caretTerm(text, text.length)).toBeNull();
    },
  );

  it("is null on a space between terms and in the key part", () => {
    expect(at("status:done | kind:bug")).toBeNull();
    expect(at("   |")).toBeNull();
    expect(at("sta|tus:done")).toBeNull();
  });

  it("treats the space inside an open quote as part of the same term", () => {
    expect(at('release:"v2.0 be|ta')).toMatchObject({ key: "release", typed: "v2.0 be" });
  });
});
