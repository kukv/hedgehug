import { describe, expect, it } from "vitest";
import { restoredCommentToast, restoredNameToast, restoredToast } from "./restored";

describe("restoredToast", () => {
  it("別の画面で削除されていたものを取り消したことを知らせる", () => {
    expect(restoredToast("HH-12")).toBe(
      "HH-12 は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました",
    );
  });
});

describe("restoredNameToast / restoredCommentToast", () => {
  it("設定の値の名前を付けて知らせる", () => {
    expect(restoredNameToast("Review")).toBe(
      "「Review」は別の画面で削除されていましたが、変更を保存したため、削除を取り消しました",
    );
  });

  it("コメントを知らせる", () => {
    expect(restoredCommentToast()).toBe(
      "コメントは別の画面で削除されていましたが、変更を保存したため、削除を取り消しました",
    );
  });
});
