import { cleanup, fireEvent, render, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import Tabs from "./Tabs.svelte";

beforeEach(() => history.replaceState(null, "", "/"));
afterEach(cleanup);

const items = [
  { label: "概要", href: "/workspaces/shared/W1", current: false },
  { label: "ツリー", href: "/workspaces/shared/W1/tree", current: true },
];

describe("Tabs", () => {
  it("タブはリンクで、今のタブだけ aria-current=page", () => {
    const { getByRole } = render(Tabs, { items });
    const nav = getByRole("navigation", { name: "ビューの切り替え" });
    const tree = within(nav).getByRole("link", { name: "ツリー" });
    expect(tree.getAttribute("href")).toBe("/workspaces/shared/W1/tree");
    expect(tree.getAttribute("aria-current")).toBe("page");
    expect(within(nav).getByRole("link", { name: "概要" }).getAttribute("aria-current")).toBeNull();
  });

  it("count は 1 以上のときだけ、ラベルの後ろに数を出す", () => {
    const { getByRole } = render(Tabs, {
      items: [
        { label: "二", href: "/a", current: true, count: 2 },
        { label: "零", href: "/b", current: false, count: 0 },
        { label: "なし", href: "/c", current: false },
      ],
    });
    const count = (name: string) => getByRole("link", { name }).querySelector(".count");
    expect(count("二 2")?.textContent).toBe("2");
    expect(count("零")).toBeNull();
    expect(count("なし")).toBeNull();
  });

  it("右端の項目を最後に出し、アイコンをつけられる", () => {
    const { getAllByRole, container } = render(Tabs, {
      items,
      end: {
        label: "設定",
        href: "/workspaces/shared/W1/settings",
        current: false,
        icon: "settings",
      },
    });
    const links = getAllByRole("link");
    expect(links.map((a) => a.textContent?.trim())).toEqual(["概要", "ツリー", "設定"]);
    expect(links[2]?.querySelector("svg")).toBeTruthy();
    expect(container.querySelectorAll("svg")).toHaveLength(1);
  });

  it("押すと router で移動する（ページは読み込み直さない）", async () => {
    const { getByRole } = render(Tabs, { items });
    const ev = new MouseEvent("click", { button: 0, bubbles: true, cancelable: true });
    getByRole("link", { name: "概要" }).dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true);
    expect(location.pathname).toBe("/workspaces/shared/W1");
    await fireEvent.click(getByRole("link", { name: "ツリー" }));
    expect(location.pathname).toBe("/workspaces/shared/W1/tree");
  });

  it("onselect のあるタブは、押すとその場で onselect を呼び、移動しない", async () => {
    const onselect = vi.fn();
    const same = "/views/shared/weekly";
    const { getByRole } = render(Tabs, {
      items: [
        { label: "ツリー", href: same, current: true },
        { label: "カンバン", href: same, current: false, onselect },
      ],
    });
    const ev = new MouseEvent("click", { button: 0, bubbles: true, cancelable: true });
    getByRole("link", { name: "カンバン" }).dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true);
    expect(onselect).toHaveBeenCalledTimes(1);
    expect(location.pathname).toBe("/");
  });

  it("onselect のあるタブも、⌘ を押して押すと新しいタブで href を開く", async () => {
    const onselect = vi.fn();
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    const { getByRole } = render(Tabs, {
      items: [{ label: "カンバン", href: "/views/shared/weekly", current: false, onselect }],
    });
    await fireEvent.click(getByRole("link", { name: "カンバン" }), { metaKey: true });
    expect(onselect).not.toHaveBeenCalled();
    expect(open).toHaveBeenCalledWith(`${location.origin}/views/shared/weekly`, "_blank");
    open.mockRestore();
  });

  it("disabled のタブは理由を title に持ち、押しても動かず onselect も呼ばない", async () => {
    const onselect = vi.fn();
    const { getByRole } = render(Tabs, {
      items: [{ label: "ツリー", href: "/x", current: false, onselect, disabled: "理由です" }],
    });
    const a = getByRole("link", { name: "ツリー" });
    expect(a).toHaveAttribute("aria-disabled", "true");
    expect(a).toHaveAttribute("title", "理由です");
    expect(a).toHaveAccessibleDescription("理由です");
    const ev = new MouseEvent("click", { button: 0, bubbles: true, cancelable: true });
    a.dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true);
    expect(onselect).not.toHaveBeenCalled();
    expect(location.pathname).toBe("/");
  });

  it("label でナビゲーションの名前を変えられる", () => {
    const { getByRole } = render(Tabs, { items, label: "プロジェクトのタブ" });
    expect(getByRole("navigation", { name: "プロジェクトのタブ" })).toBeTruthy();
  });
});
