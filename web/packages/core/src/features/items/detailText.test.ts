import { describe, expect, it } from "vitest";
import { copiedMessage, copyFailedMessage } from "./copyId";
import { normalizeTitle } from "./detail";

// detail.test.ts が 300 行の上限を超えたので、文言の検査をここに分けた。

describe("titles and copy messages", () => {
  it("normalizes spaces and line breaks", () => {
    expect(normalizeTitle("  a \n\t b  ")).toBe("a b");
  });

  it("says what was copied", () => {
    expect(copiedMessage("HH-2")).toBe("コピーしました: HH-2");
    expect(copyFailedMessage("HH-2")).toBe(
      "コピーできませんでした。HH-2 を選択してコピーしてください",
    );
  });
});
