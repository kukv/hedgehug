import { describe, expect, it } from "vitest";
import { isMemberOf, tokenize } from "./markdown";

const nobody = () => false;
const text = (s: string) => ({ kind: "text" as const, text: s });

describe("tokenize blocks", () => {
  it("makes headings of three levels; four # or no space is a paragraph", () => {
    expect(tokenize("# 見出し1\n## 見出し2\n### 見出し3\n#### 4つ\n#空白なし", nobody)).toEqual([
      { kind: "heading", level: 1, inlines: [text("見出し1")] },
      { kind: "heading", level: 2, inlines: [text("見出し2")] },
      { kind: "heading", level: 3, inlines: [text("見出し3")] },
      { kind: "paragraph", inlines: [text("#### 4つ")] },
      { kind: "paragraph", inlines: [text("#空白なし")] },
    ]);
  });

  it("groups list lines by kind and reads the checkbox marks", () => {
    expect(tokenize("- 一つ目\n* 二つ目\n- [ ] 未\n- [x] 済\n1. 番号\n2. 番号2", nobody)).toEqual([
      {
        kind: "list",
        ordered: false,
        items: [
          { check: "none", inlines: [text("一つ目")] },
          { check: "none", inlines: [text("二つ目")] },
          { check: "unchecked", inlines: [text("未")] },
          { check: "checked", inlines: [text("済")] },
        ],
      },
      {
        kind: "list",
        ordered: true,
        items: [
          { check: "none", inlines: [text("番号")] },
          { check: "none", inlines: [text("番号2")] },
        ],
      },
    ]);
  });

  it("keeps a fenced code block as it is; an unclosed fence runs to the end", () => {
    expect(tokenize("前\n```ts\nconst a = 1;\n\n  b\n```\n後", nobody)).toEqual([
      { kind: "paragraph", inlines: [text("前")] },
      { kind: "code", text: "const a = 1;\n\n  b" },
      { kind: "paragraph", inlines: [text("後")] },
    ]);
    expect(tokenize("```\nabc\ndef", nobody)).toEqual([{ kind: "code", text: "abc\ndef" }]);
    expect(tokenize("```\n```", nobody)).toEqual([{ kind: "code", text: "" }]);
  });

  it("does not open a fence on a line that also closes it (```foo```)", () => {
    expect(tokenize("```foo```\n後", nobody)).toEqual([
      { kind: "paragraph", inlines: [text("``"), { kind: "code", text: "foo" }, text("``")] },
      { kind: "paragraph", inlines: [text("後")] },
    ]);
  });

  it("makes one paragraph per other line and skips blank lines and CRLF", () => {
    expect(tokenize("一行目\r\n\r\n二行目\n   \n三行目", nobody)).toEqual([
      { kind: "paragraph", inlines: [text("一行目")] },
      { kind: "paragraph", inlines: [text("二行目")] },
      { kind: "paragraph", inlines: [text("三行目")] },
    ]);
    expect(tokenize("", nobody)).toEqual([]);
  });
});

describe("tokenize inlines", () => {
  const isTaro = isMemberOf([{ login: "Taro", name: "太郎" }]);

  it("reads bold, code, a link without its trailing dot, and members' mentions", () => {
    expect(
      tokenize("**太字** と `code` と https://example.com/a?b=1. の @TARO さん @guest", isTaro),
    ).toEqual([
      {
        kind: "paragraph",
        inlines: [
          { kind: "bold", text: "太字" },
          text(" と "),
          { kind: "code", text: "code" },
          text(" と "),
          { kind: "link", url: "https://example.com/a?b=1" },
          text(". の "),
          // 大文字小文字を区別せずに一致し、表示は打ったとおり。メンバーでない @guest はただの文字。
          { kind: "mention", login: "TARO" },
          text(" さん @guest"),
        ],
      },
    ]);
  });

  it("treats markup, other schemes and a bare http:// as plain text (XSS)", () => {
    const line = "<script>alert(1)</script> [x](javascript:alert(1)) http:// https://.";
    expect(tokenize(line, nobody)).toEqual([{ kind: "paragraph", inlines: [text(line)] }]);
  });

  it("takes @ after any character as a mention, but not inside a URL", () => {
    expect(tokenize("https://a.example/@taro と foo@Taro", isTaro)).toEqual([
      {
        kind: "paragraph",
        inlines: [
          { kind: "link", url: "https://a.example/@taro" },
          text(" と foo"),
          { kind: "mention", login: "Taro" },
        ],
      },
    ]);
  });

  it("reads inlines inside a list item", () => {
    expect(tokenize("- 担当 @taro", isTaro)).toEqual([
      {
        kind: "list",
        ordered: false,
        items: [{ check: "none", inlines: [text("担当 "), { kind: "mention", login: "taro" }] }],
      },
    ]);
  });
});

describe("isMemberOf", () => {
  it("compares logins without regard to case", () => {
    const is = isMemberOf([{ login: "Taro", name: "太郎" }]);
    expect(is("taro")).toBe(true);
    expect(is("jiro")).toBe(false);
  });
});
