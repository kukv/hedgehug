import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { text } from "../test-snippet";
import Dialog from "./Dialog.svelte";
import Menu from "./Menu.svelte";
import Toasts from "./Toasts.svelte";

describe("Dialog", () => {
  it("renders nothing when closed", () => {
    render(Dialog, {
      props: { open: false, title: "削除", onclose: vi.fn(), children: text("本文") },
    });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("closes on Escape and on a press outside, not inside", async () => {
    const onclose = vi.fn();
    render(Dialog, { props: { open: true, title: "削除", onclose, children: text("本文") } });
    const dialog = screen.getByRole("dialog", { name: "削除" });
    await fireEvent.mouseDown(dialog);
    expect(onclose).not.toHaveBeenCalled();
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(onclose).toHaveBeenCalledTimes(1);
    await fireEvent.mouseDown(dialog.parentElement as HTMLElement);
    expect(onclose).toHaveBeenCalledTimes(2);
  });

  it("ignores Escape while composing text", async () => {
    const onclose = vi.fn();
    render(Dialog, { props: { open: true, title: "削除", onclose, children: text("本文") } });
    await fireEvent.keyDown(window, { key: "Escape", isComposing: true });
    expect(onclose).not.toHaveBeenCalled();
  });
});

describe("Menu", () => {
  const items = [
    { id: "copy", label: "ID をコピー", hint: "C" },
    { separator: true as const },
    { id: "delete", label: "削除", danger: true, disabled: true },
  ];

  it("reports the selected item and skips a disabled one", async () => {
    const onselect = vi.fn();
    render(Menu, { props: { open: true, x: 10, y: 10, items, onselect, onclose: vi.fn() } });
    await fireEvent.click(screen.getByRole("menuitem", { name: /ID をコピー/ }));
    await fireEvent.click(screen.getByRole("menuitem", { name: /削除/ }));
    expect(onselect).toHaveBeenCalledExactlyOnceWith("copy");
  });

  it("closes on Escape", async () => {
    const onclose = vi.fn();
    render(Menu, { props: { open: true, x: 0, y: 0, items, onselect: vi.fn(), onclose } });
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(onclose).toHaveBeenCalledOnce();
  });
});

describe("Toasts", () => {
  it("reports dismiss and action", async () => {
    const ondismiss = vi.fn();
    const onaction = vi.fn();
    const toasts = [{ id: "t1", text: "保存しました", action: "開く" }];
    render(Toasts, { props: { toasts, ondismiss, onaction } });
    expect(screen.getByRole("status")).toHaveTextContent("保存しました");
    await fireEvent.click(screen.getByRole("button", { name: "開く" }));
    await fireEvent.click(screen.getByRole("button", { name: "閉じる" }));
    expect(onaction).toHaveBeenCalledWith("t1");
    expect(ondismiss).toHaveBeenCalledWith("t1");
  });
});
