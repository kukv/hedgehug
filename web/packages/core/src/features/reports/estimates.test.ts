import { describe, expect, it } from "vitest";
import {
  diffText,
  emptyEstimates,
  hoursText,
  perPointText,
  pointsText,
  sprintScopeNote,
} from "./estimates";

describe("estimates", () => {
  it("hoursText は 100 分の 1 時間の整数を時間にする", () => {
    expect(hoursText(125)).toBe("1.25 時間");
    expect(hoursText(200)).toBe("2 時間");
    expect(hoursText(null)).toBe("—");
  });
  it("diffText は正に符号を付け、正だけ over", () => {
    expect(diffText(100)).toEqual({ text: "+1 時間", over: true });
    expect(diffText(-25)).toEqual({ text: "-0.25 時間", over: false });
    expect(diffText(0)).toEqual({ text: "0 時間", over: false });
    expect(diffText(null)).toEqual({ text: "—", over: false });
  });
  it("perPointText は小数第 1 位まで", () => {
    expect(perPointText(3)).toBe("3.0");
    expect(perPointText(3.6)).toBe("3.6");
    expect(perPointText(null)).toBe("—");
  });
  it("pointsText", () => {
    expect(pointsText(5)).toBe("5 pt");
    expect(pointsText(null)).toBe("—");
  });
  it("文", () => {
    expect(sprintScopeNote).toBe(
      "作業時間は、アイテムに付いた記録のすべてです(別のスプリントにいたあいだの記録も含みます)。",
    );
    expect(emptyEstimates).toBe(
      "見積もり(ポイントか見積もり時間)と作業時間の両方があるアイテムがありません。アイテムの「作業時間」で記録できます。",
    );
  });
});
