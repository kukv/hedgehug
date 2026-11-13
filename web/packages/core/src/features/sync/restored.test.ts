import { describe, expect, it } from "vitest";
import { restoredToast } from "./restored";

describe("restoredToast", () => {
  it("別の画面で削除されていたものを取り消したことを知らせる", () => {
    expect(restoredToast("HH-12")).toBe(
      "HH-12 は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました",
    );
  });
});
