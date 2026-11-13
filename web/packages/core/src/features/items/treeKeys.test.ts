import { describe, expect, it } from "vitest";
import { type KeyInput, treeKeyAction } from "./treeKeys";

const key = (k: string, mods: Partial<KeyInput> = {}): KeyInput => ({
  key: k,
  ctrlKey: false,
  metaKey: false,
  altKey: false,
  shiftKey: false,
  ...mods,
});
const task = { kind: "Task" } as const;

describe("treeKeyAction (read-only keys)", () => {
  it("maps the arrows, Enter and the menu keys", () => {
    expect(treeKeyAction(key("ArrowUp"), task)).toBe("up");
    expect(treeKeyAction(key("ArrowDown"), task)).toBe("down");
    expect(treeKeyAction(key("ArrowRight"), task)).toBe("right");
    expect(treeKeyAction(key("ArrowLeft"), task)).toBe("left");
    expect(treeKeyAction(key("Enter"), task)).toBe("open");
    expect(treeKeyAction(key("ContextMenu"), task)).toBe("menu");
    expect(treeKeyAction(key("F10", { shiftKey: true }), task)).toBe("menu");
  });

  it("opens in a new tab with Ctrl+Enter and Cmd+Enter", () => {
    expect(treeKeyAction(key("Enter", { ctrlKey: true }), task)).toBe("open-tab");
    expect(treeKeyAction(key("Enter", { metaKey: true }), task)).toBe("open-tab");
  });

  it("leaves other combinations alone", () => {
    expect(treeKeyAction(key("F10"), task)).toBeNull();
    expect(treeKeyAction(key("Enter", { shiftKey: true }), task)).toBeNull();
    expect(treeKeyAction(key("Enter", { altKey: true }), task)).toBeNull();
    expect(treeKeyAction(key("Enter", { ctrlKey: true, shiftKey: true }), task)).toBeNull();
    expect(treeKeyAction(key("ArrowLeft", { altKey: true }), task)).toBeNull();
    expect(treeKeyAction(key("ArrowDown", { shiftKey: true }), task)).toBeNull();
    expect(treeKeyAction(key("Tab"), task)).toBeNull();
    expect(treeKeyAction(key("Escape"), task)).toBeNull();
  });

  it("does nothing without a focused row or while composing text", () => {
    expect(treeKeyAction(key("ArrowDown"), null)).toBeNull();
    expect(treeKeyAction(key("Enter", { isComposing: true }), task)).toBeNull();
  });
});
