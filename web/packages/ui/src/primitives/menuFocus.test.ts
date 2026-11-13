import { fireEvent, render, screen } from "@testing-library/svelte";
import { tick } from "svelte";
import { describe, expect, it, vi } from "vitest";
import ValueMenu from "../patterns/ValueMenu.svelte";
import Menu from "./Menu.svelte";

const items = [
  { id: "up", label: "上へ移動", disabled: true },
  { id: "copy", label: "ID をコピー" },
  { id: "dup", label: "複製" },
];

describe("menu focus", () => {
  it("moves the focus to the first enabled item and gives it back when the menu closes", async () => {
    const opener = document.createElement("button");
    document.body.append(opener);
    opener.focus();
    const props = { open: true, x: 0, y: 0, items, onselect: vi.fn(), onclose: vi.fn() };
    const { rerender } = render(Menu, { props });
    await tick();
    await Promise.resolve();
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: /ID をコピー/ }));
    await rerender({ ...props, open: false });
    expect(document.activeElement).toBe(opener);
    opener.remove();
  });

  it("does not take the focus from a search field inside the menu", async () => {
    const options = Array.from({ length: 7 }, (_, i) => ({
      id: `o${i}`,
      label: `値 ${i}`,
      checked: false,
    }));
    render(ValueMenu, {
      props: { open: true, x: 0, y: 0, label: "値", options, onselect: vi.fn(), onclose: vi.fn() },
    });
    const search = screen.getByRole("searchbox", { name: "検索" });
    search.focus();
    await tick();
    await Promise.resolve();
    expect(document.activeElement).toBe(search);
  });

  it("does not take the focus back from a part that took it after the menu closed", async () => {
    const opener = document.createElement("button");
    const other = document.createElement("input");
    document.body.append(opener, other);
    opener.focus();
    const props = { open: true, x: 0, y: 0, items, onselect: vi.fn(), onclose: vi.fn() };
    const { rerender } = render(Menu, { props });
    await tick();
    await Promise.resolve();
    other.focus();
    await rerender({ ...props, open: false });
    expect(document.activeElement).toBe(other);
    opener.remove();
    other.remove();
  });

  it("moves between the enabled items with the arrow keys, Home and End, wrapping around", async () => {
    render(Menu, { props: { open: true, x: 0, y: 0, items, onselect: vi.fn(), onclose: vi.fn() } });
    await tick();
    await Promise.resolve();
    const copy = screen.getByRole("menuitem", { name: /ID をコピー/ });
    const dup = screen.getByRole("menuitem", { name: /複製/ });
    expect(document.activeElement).toBe(copy);
    await fireEvent.keyDown(copy, { key: "ArrowDown" });
    expect(document.activeElement).toBe(dup);
    await fireEvent.keyDown(dup, { key: "ArrowDown" });
    expect(document.activeElement).toBe(copy);
    await fireEvent.keyDown(copy, { key: "ArrowUp" });
    expect(document.activeElement).toBe(dup);
    await fireEvent.keyDown(dup, { key: "Home" });
    expect(document.activeElement).toBe(copy);
    await fireEvent.keyDown(copy, { key: "End" });
    expect(document.activeElement).toBe(dup);
  });

  it("leaves Home and End to a search field in the menu", async () => {
    const options = Array.from({ length: 7 }, (_, i) => ({
      id: `o${i}`,
      label: `値 ${i}`,
      checked: false,
    }));
    render(ValueMenu, {
      props: { open: true, x: 0, y: 0, label: "値", options, onselect: vi.fn(), onclose: vi.fn() },
    });
    const search = screen.getByRole("searchbox", { name: "検索" });
    search.focus();
    const event = new KeyboardEvent("keydown", { key: "Home", bubbles: true, cancelable: true });
    search.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(false);
    expect(document.activeElement).toBe(search);
  });
});
