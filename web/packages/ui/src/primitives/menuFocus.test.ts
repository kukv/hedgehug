import { fireEvent, render, screen } from "@testing-library/svelte";
import { createRawSnippet, tick } from "svelte";
import { describe, expect, it, vi } from "vitest";
import ValueMenu from "../patterns/ValueMenu.svelte";
import Menu from "./Menu.svelte";
import { menuFocus } from "./menuFocus";

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

  it("keeps a search field in the head outside the menu role, named, focused and linked to the items by the arrows", async () => {
    const head = createRawSnippet(() => ({ render: () => `<input aria-label="移動先を検索">` }));
    render(Menu, {
      props: { open: true, x: 0, y: 0, items, onselect: vi.fn(), onclose: vi.fn(), head },
    });
    // 「移動…」の検索欄は、出たときにフォーカスを取る(Input の focusOnMount)。
    const search = screen.getByRole("textbox", { name: "移動先を検索" });
    search.focus();
    await tick();
    await Promise.resolve();
    // role="menu" の中に置けるのは項目だけなので、検索欄はその外(同じ浮いた枠の中)に置く。
    expect(screen.getByRole("menu").contains(search)).toBe(false);
    expect(document.activeElement).toBe(search);
    const copy = screen.getByRole("menuitem", { name: /ID をコピー/ });
    const dup = screen.getByRole("menuitem", { name: /複製/ });
    await fireEvent.keyDown(search, { key: "ArrowDown" });
    expect(document.activeElement).toBe(copy);
    await fireEvent.keyDown(copy, { key: "ArrowDown" });
    expect(document.activeElement).toBe(dup);
    await fireEvent.keyDown(dup, { key: "ArrowDown" });
    expect(document.activeElement).toBe(copy);
    search.focus();
    await fireEvent.keyDown(search, { key: "ArrowUp" });
    expect(document.activeElement).toBe(dup);
  });

  it("gives the focus back to where the first menu was opened from, when a menu opens another", async () => {
    // アイテムのメニューの「移動…」: 項目にフォーカスがあるまま次のメニューが出て、そのあと最初のメニューが消える。
    const opener = document.createElement("button");
    const first = document.createElement("div");
    const entry = document.createElement("button");
    first.append(entry);
    const second = document.createElement("div");
    const search = document.createElement("input");
    second.append(search);
    document.body.append(opener, first, second);
    opener.focus();
    const a = menuFocus(first);
    entry.focus();
    // 次のメニュー(Menu)は、出す前のフォーカス(最初のメニューの項目)を読んでおき、head の検索欄が出たときに
    // フォーカスを取ったあとで、それを menuFocus に渡す。
    const shownFrom = document.activeElement;
    a.destroy();
    first.remove();
    search.focus();
    const b = menuFocus(second, shownFrom);
    second.remove();
    b.destroy();
    expect(document.activeElement).toBe(opener);
    opener.remove();
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
