import { afterEach, describe, expect, it, vi } from "vitest";
import { isMac, kbdLabel, modKey, registerShortcut } from "./keys";

function press(init: KeyboardEventInit, target: EventTarget = window) {
  const e = new KeyboardEvent("keydown", { bubbles: true, cancelable: true, ...init });
  target.dispatchEvent(e);
  return e;
}

afterEach(() => {
  vi.restoreAllMocks();
  document.body.innerHTML = "";
});

describe("isMac / modKey", () => {
  it("Mac は ⌘、ほかは Ctrl", () => {
    vi.spyOn(navigator, "platform", "get").mockReturnValue("MacIntel");
    expect(isMac()).toBe(true);
    expect(modKey(new KeyboardEvent("keydown", { metaKey: true }))).toBe(true);
    expect(modKey(new KeyboardEvent("keydown", { ctrlKey: true }))).toBe(false);
    vi.spyOn(navigator, "platform", "get").mockReturnValue("Win32");
    expect(isMac()).toBe(false);
    expect(modKey(new KeyboardEvent("keydown", { ctrlKey: true }))).toBe(true);
  });
});

describe("kbdLabel", () => {
  it("Mac は記号、ほかは Ctrl+", () => {
    vi.spyOn(navigator, "platform", "get").mockReturnValue("MacIntel");
    expect(kbdLabel("mod+k")).toBe("⌘K");
    expect(kbdLabel("shift+mod+s")).toBe("⇧⌘S");
    expect(kbdLabel("mod+enter")).toBe("⌘↵");
    vi.spyOn(navigator, "platform", "get").mockReturnValue("Linux x86_64");
    expect(kbdLabel("mod+k")).toBe("Ctrl+K");
    expect(kbdLabel("shift+mod+s")).toBe("Ctrl+Shift+S");
  });
});

describe("registerShortcut", () => {
  it("mod+k は Ctrl+K で呼ばれ、解除すると呼ばれない", () => {
    vi.spyOn(navigator, "platform", "get").mockReturnValue("Linux x86_64");
    const fn = vi.fn();
    const off = registerShortcut("mod+k", fn);
    const e = press({ key: "k", ctrlKey: true });
    expect(fn).toHaveBeenCalledTimes(1);
    expect(e.defaultPrevented).toBe(true);
    press({ key: "k" });
    expect(fn).toHaveBeenCalledTimes(1);
    off();
    press({ key: "k", ctrlKey: true });
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("shift+mod+s は Shift が要る", () => {
    vi.spyOn(navigator, "platform", "get").mockReturnValue("Linux x86_64");
    const fn = vi.fn();
    const off = registerShortcut("shift+mod+s", fn);
    press({ key: "s", ctrlKey: true });
    expect(fn).not.toHaveBeenCalled();
    press({ key: "S", ctrlKey: true, shiftKey: true });
    expect(fn).toHaveBeenCalledTimes(1);
    off();
  });

  it("修飾キーなしのショートカットは入力欄の中では効かない", () => {
    const fn = vi.fn();
    const off = registerShortcut("c", fn);
    const input = document.createElement("input");
    document.body.append(input);
    press({ key: "c" }, input);
    expect(fn).not.toHaveBeenCalled();
    press({ key: "c" });
    expect(fn).toHaveBeenCalledTimes(1);
    off();
  });

  it("修飾キーなしのショートカットはダイアログが開いている間は効かない。⌘ 付きは効く", () => {
    vi.spyOn(navigator, "platform", "get").mockReturnValue("Linux x86_64");
    const plain = vi.fn();
    const withMod = vi.fn();
    const offA = registerShortcut("c", plain);
    const offB = registerShortcut("mod+k", withMod);
    const dlg = document.createElement("div");
    dlg.setAttribute("aria-modal", "true");
    document.body.append(dlg);
    press({ key: "c" });
    press({ key: "k", ctrlKey: true });
    expect(plain).not.toHaveBeenCalled();
    expect(withMod).toHaveBeenCalledTimes(1);
    offA();
    offB();
  });
});
