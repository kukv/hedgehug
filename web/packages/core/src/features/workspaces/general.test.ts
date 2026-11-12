import { describe, expect, it } from "vitest";
import { prefixConfirm, prefixExamples, prefixHelp } from "./general";

const usage = (items: number, firstNumbers: number[]) => ({
  items,
  firstNumbers,
  values: {},
  properties: {},
});

describe("general", () => {
  it("shows up to three ids as examples", () => {
    expect(prefixExamples("HH", [1, 2, 5])).toBe("HH-1, HH-2, HH-5");
    expect(prefixExamples("HH", [1, 2, 3, 4])).toBe("HH-1, HH-2, HH-3");
    expect(prefixExamples("HH", [])).toBe("アイテムはまだありません");
  });

  it("writes the confirm text of spec 03", () => {
    expect(prefixConfirm("HH", "WEB", usage(12, [1, 2, 3]))).toBe(
      "12 件の ID が HH-1 → WEB-1 のように変わります。古い ID で書かれたコメントやリンクは、そのままの文字で残ります。旧い接頭辞でも検索できます。",
    );
    expect(prefixConfirm("HH", "WEB", usage(3, [4, 5, 6]))).toContain("HH-4 → WEB-4");
    expect(prefixConfirm("HH", "WEB", usage(0, []))).toBe(
      "接頭辞を HH から WEB に変更します。ID が変わるアイテムはありません。旧い接頭辞でも検索できます。",
    );
  });

  it("has the help text of spec 03", () => {
    expect(prefixHelp).toBe(
      "Epic・Story・Task・Bug・Step の ID の頭に付きます。変更すると、すべての ID が新しい接頭辞で表示されます(番号はそのままです)。",
    );
  });
});
