import { describe, expect, it } from "vitest";
import { isQueuedAction, type KeyInput, treeKeyAction } from "./treeKeys";

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

describe("treeKeyAction (write keys)", () => {
  it("maps Space, S, K and C, whatever the letter case", () => {
    expect(treeKeyAction(key(" "), task)).toBe("toggle-done");
    expect(treeKeyAction(key("s"), task)).toBe("status-menu");
    expect(treeKeyAction(key("S", { shiftKey: true }), task)).toBe("status-menu");
    expect(treeKeyAction(key("k"), task)).toBe("change-kind");
    expect(treeKeyAction(key("K"), task)).toBe("change-kind");
    expect(treeKeyAction(key("c"), task)).toBe("create-sibling");
    expect(treeKeyAction(key("C"), task)).toBe("create-sibling");
  });

  it("ignores K on an Epic, but not the other keys", () => {
    const epic = { kind: "Epic" } as const;
    expect(treeKeyAction(key("k"), epic)).toBeNull();
    expect(treeKeyAction(key("s"), epic)).toBe("status-menu");
    expect(treeKeyAction(key("c"), epic)).toBe("create-sibling");
  });

  it("maps Alt+arrows to the sibling move and Alt+Shift+arrows to the parent change", () => {
    expect(treeKeyAction(key("ArrowUp", { altKey: true }), task)).toBe("move-up");
    expect(treeKeyAction(key("ArrowDown", { altKey: true }), task)).toBe("move-down");
    expect(treeKeyAction(key("ArrowUp", { altKey: true, shiftKey: true }), task)).toBe(
      "reparent-up",
    );
    expect(treeKeyAction(key("ArrowDown", { altKey: true, shiftKey: true }), task)).toBe(
      "reparent-down",
    );
  });

  it("leaves the shortcuts of the browser and the app alone", () => {
    expect(treeKeyAction(key("c", { ctrlKey: true }), task)).toBeNull();
    expect(treeKeyAction(key("c", { metaKey: true }), task)).toBeNull();
    expect(treeKeyAction(key("s", { altKey: true }), task)).toBeNull();
    expect(treeKeyAction(key(" ", { shiftKey: true }), task)).toBeNull();
    expect(treeKeyAction(key("ArrowUp", { altKey: true, ctrlKey: true }), task)).toBeNull();
    expect(treeKeyAction(key("ArrowUp", { ctrlKey: true }), task)).toBeNull();
  });

  it("knows which actions are queued", () => {
    expect(
      ["move-up", "move-down", "reparent-up", "reparent-down"].every((a) =>
        isQueuedAction(a as never),
      ),
    ).toBe(true);
    expect(isQueuedAction("toggle-done")).toBe(false);
    expect(isQueuedAction("up")).toBe(false);
    expect(isQueuedAction(null)).toBe(false);
  });
});
