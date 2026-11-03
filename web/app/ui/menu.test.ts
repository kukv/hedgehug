import { cleanup, fireEvent, render } from "@testing-library/svelte";
import { createRawSnippet, flushSync } from "svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import Menu from "./Menu.svelte";

afterEach(cleanup);

const items = [
  { value: "todo", label: "未着手" },
  { value: "doing", label: "進行中", hint: "S" },
  { value: "off", label: "無効", disabled: true },
  { value: "done", label: "完了" },
];

function open(over: Record<string, unknown> = {}) {
  const onselect = vi.fn();
  const onclose = vi.fn();
  const r = render(Menu, {
    props: { anchor: null, items, label: "ステータス", onselect, onclose, ...over },
  });
  flushSync();
  return { onselect, onclose, ...r };
}

const focused = () => document.activeElement?.textContent ?? "";

describe("Menu", () => {
  it("role=menu と項目を出し、最初の有効な項目にフォーカスする", async () => {
    const { getByRole, getAllByRole } = open();
    expect(getByRole("menu", { name: "ステータス" })).toBeTruthy();
    expect(getAllByRole("menuitem")).toHaveLength(4);
    await vi.waitFor(() => expect(focused()).toContain("未着手"));
  });

  it("checked を持つ項目は menuitemcheckbox で、aria-checked に状態を出し、矢印で回れる", async () => {
    const { getAllByRole, queryAllByRole } = open({
      items: [
        { value: "a", label: "甲", checked: true },
        { value: "b", label: "乙", checked: false },
      ],
    });
    const boxes = getAllByRole("menuitemcheckbox");
    expect(boxes.map((b) => b.getAttribute("aria-checked"))).toEqual(["true", "false"]);
    expect(queryAllByRole("menuitem")).toHaveLength(0);
    await vi.waitFor(() => expect(focused()).toContain("甲"));
    await fireEvent.keyDown(boxes[0] as HTMLElement, { key: "ArrowDown" });
    await vi.waitFor(() => expect(focused()).toContain("乙"));
  });

  it("selected の項目から始める", async () => {
    open({ selected: "done" });
    await vi.waitFor(() => expect(focused()).toContain("完了"));
  });

  it("↓↑ で無効な項目を飛ばして回る", async () => {
    const { getByRole } = open();
    const menu = getByRole("menu");
    await fireEvent.keyDown(menu, { key: "ArrowDown" });
    await vi.waitFor(() => expect(focused()).toContain("進行中"));
    await fireEvent.keyDown(menu, { key: "ArrowDown" });
    await vi.waitFor(() => expect(focused()).toContain("完了"));
    await fireEvent.keyDown(menu, { key: "ArrowDown" });
    await vi.waitFor(() => expect(focused()).toContain("未着手"));
    await fireEvent.keyDown(menu, { key: "ArrowUp" });
    await vi.waitFor(() => expect(focused()).toContain("完了"));
  });

  it("Home / End", async () => {
    const { getByRole } = open();
    const menu = getByRole("menu");
    await fireEvent.keyDown(menu, { key: "End" });
    await vi.waitFor(() => expect(focused()).toContain("完了"));
    await fireEvent.keyDown(menu, { key: "Home" });
    await vi.waitFor(() => expect(focused()).toContain("未着手"));
  });

  it("Enter で選ぶ", async () => {
    const { getByRole, onselect } = open();
    const menu = getByRole("menu");
    await fireEvent.keyDown(menu, { key: "ArrowDown" });
    await fireEvent.keyDown(menu, { key: "Enter" });
    expect(onselect).toHaveBeenCalledWith("doing");
  });

  it("クリックで選ぶ。無効な項目は選べない", async () => {
    const { getByRole, onselect } = open();
    await fireEvent.click(getByRole("menuitem", { name: /無効/ }));
    expect(onselect).not.toHaveBeenCalled();
    await fireEvent.click(getByRole("menuitem", { name: /完了/ }));
    expect(onselect).toHaveBeenCalledWith("done");
  });

  it("Esc と Tab で閉じる", async () => {
    const { getByRole, onclose } = open();
    await fireEvent.keyDown(getByRole("menu"), { key: "Escape" });
    await fireEvent.keyDown(getByRole("menu"), { key: "Tab" });
    expect(onclose).toHaveBeenCalledTimes(2);
  });

  it("外側を押すと閉じる。anchor の中は除く", async () => {
    const anchor = document.createElement("button");
    const outside = document.createElement("div");
    document.body.append(anchor, outside);
    const { onclose } = open({ anchor });
    await fireEvent.pointerDown(anchor);
    expect(onclose).not.toHaveBeenCalled();
    await fireEvent.pointerDown(outside);
    expect(onclose).toHaveBeenCalledTimes(1);
    anchor.remove();
    outside.remove();
  });

  it("フォーカスが外の要素へ移ったら閉じる（ダイアログやパレットが開いたとき）。anchor へ移ったときは除く", async () => {
    const anchor = document.createElement("button");
    const other = document.createElement("input");
    document.body.append(anchor, other);
    const { onclose, getAllByRole } = open({ anchor });
    await vi.waitFor(() => expect(focused()).toContain("未着手"));
    anchor.focus();
    expect(onclose).not.toHaveBeenCalled();
    getAllByRole("menuitem")[0]?.focus();
    other.focus();
    expect(onclose).toHaveBeenCalledTimes(1);
    anchor.remove();
    other.remove();
  });

  it("anchor の下に置く", () => {
    const anchor = document.createElement("div");
    anchor.getBoundingClientRect = () =>
      ({ top: 100, bottom: 140, left: 30, right: 200, width: 170, height: 40 }) as DOMRect;
    const { getByRole } = open({ anchor });
    const menu = getByRole("menu") as HTMLElement;
    expect(menu.style.top).toBe("144px");
    expect(menu.style.left).toBe("30px");
  });

  it("閉じたら開いた元の要素へフォーカスを戻す", async () => {
    const opener = document.createElement("button");
    document.body.append(opener);
    opener.focus();
    const { getByRole, unmount } = open();
    await vi.waitFor(() => expect(focused()).toContain("未着手"));
    await fireEvent.keyDown(getByRole("menu"), { key: "Escape" });
    unmount();
    expect(document.activeElement).toBe(opener);
    opener.remove();
  });

  it("選んだ結果でフォーカスが別へ移っていたら奪わない", async () => {
    const opener = document.createElement("button");
    const other = document.createElement("button");
    document.body.append(opener, other);
    opener.focus();
    const { unmount } = open();
    other.focus();
    unmount();
    expect(document.activeElement).toBe(other);
    opener.remove();
    other.remove();
  });

  it("item の snippet で項目の中身を差し替えられる", () => {
    const item = createRawSnippet(() => ({ render: () => '<span data-custom="1">★</span>' }));
    const { container } = open({ item });
    expect(container.querySelectorAll("[data-custom]")).toHaveLength(4);
  });
});
