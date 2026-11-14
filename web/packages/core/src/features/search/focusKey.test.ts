import { describe, expect, it } from "vitest";
import { focusesFilter } from "./focusKey";

const ok = {
  key: "/",
  ctrl: false,
  meta: false,
  alt: false,
  typing: false,
  dialogOpen: false,
  hasField: true,
};

describe("focusesFilter", () => {
  it("/ で欄に移る(true なら / は入れない)", () => {
    expect(focusesFilter(ok)).toBe(true);
  });
  it("/ 以外のキーは何もしない", () => {
    expect(focusesFilter({ ...ok, key: "c" })).toBe(false);
  });
  it("修飾キーつきは何もしない", () => {
    expect(focusesFilter({ ...ok, ctrl: true })).toBe(false);
    expect(focusesFilter({ ...ok, meta: true })).toBe(false);
    expect(focusesFilter({ ...ok, alt: true })).toBe(false);
  });
  it("入力中は何もしない", () => {
    expect(focusesFilter({ ...ok, typing: true })).toBe(false);
  });
  it("ダイアログを開いているときは何もしない", () => {
    expect(focusesFilter({ ...ok, dialogOpen: true })).toBe(false);
  });
  it("欄のない画面では何もしない", () => {
    expect(focusesFilter({ ...ok, hasField: false })).toBe(false);
  });
});
