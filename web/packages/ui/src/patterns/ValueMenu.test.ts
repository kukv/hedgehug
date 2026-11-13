import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import ValueMenu from "./ValueMenu.svelte";

const opts = (n: number) =>
  Array.from({ length: n }, (_, i) => ({ id: `v${i}`, label: `値${i}`, checked: i === 1 }));
const base = { open: true, x: 0, y: 0, label: "優先度" };

describe("ValueMenu", () => {
  it("marks the current value and reports the choice", async () => {
    const onselect = vi.fn();
    render(ValueMenu, { props: { ...base, options: opts(3), onselect, onclose: vi.fn() } });
    expect(screen.getByRole("menuitemcheckbox", { name: /値1/ })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    await fireEvent.click(screen.getByRole("menuitemcheckbox", { name: /値2/ }));
    expect(onselect).toHaveBeenCalledWith("v2");
  });

  it("has no search field below 7 options", () => {
    render(ValueMenu, {
      props: { ...base, options: opts(6), onselect: vi.fn(), onclose: vi.fn() },
    });
    expect(screen.queryByRole("searchbox")).toBeNull();
  });

  it("filters by label from 7 options", async () => {
    render(ValueMenu, {
      props: { ...base, options: opts(7), onselect: vi.fn(), onclose: vi.fn() },
    });
    await fireEvent.input(screen.getByRole("searchbox"), { target: { value: "値6" } });
    expect(screen.getAllByRole("menuitemcheckbox")).toHaveLength(1);
  });

  it("keeps the search field outside the named menu role, and the arrows reach the items from it", async () => {
    render(ValueMenu, {
      props: { ...base, options: opts(7), onselect: vi.fn(), onclose: vi.fn() },
    });
    const search = screen.getByRole("searchbox", { name: "検索" });
    // role="menu" の中に置けるのは項目だけなので、検索欄はその外(同じ浮いた枠の中)に置く。
    expect(screen.getByRole("menu", { name: "優先度" }).contains(search)).toBe(false);
    search.focus();
    await fireEvent.keyDown(search, { key: "ArrowDown" });
    expect(document.activeElement).toBe(screen.getByRole("menuitemcheckbox", { name: /値0/ }));
  });

  it("closes after a choice", async () => {
    const onclose = vi.fn();
    render(ValueMenu, { props: { ...base, options: opts(2), onselect: vi.fn(), onclose } });
    await fireEvent.click(screen.getByRole("menuitemcheckbox", { name: /値0/ }));
    expect(onclose).toHaveBeenCalledTimes(1);
  });

  it("stays open for multi", async () => {
    const onclose = vi.fn();
    render(ValueMenu, {
      props: { ...base, multi: true, options: opts(2), onselect: vi.fn(), onclose },
    });
    await fireEvent.click(screen.getByRole("menuitemcheckbox", { name: /値0/ }));
    expect(onclose).not.toHaveBeenCalled();
  });

  it("closes on Escape and on a press outside", async () => {
    const onclose = vi.fn();
    render(ValueMenu, { props: { ...base, options: opts(2), onselect: vi.fn(), onclose } });
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(onclose).toHaveBeenCalledTimes(1);
    await fireEvent.pointerDown(document.body);
    expect(onclose).toHaveBeenCalledTimes(2);
  });

  it("disables an option and shows its reason", async () => {
    const onselect = vi.fn();
    const reason = "ワークフローの制約: Todo から Done には変更できません(InProgress のみ)";
    render(ValueMenu, {
      props: {
        ...base,
        onselect,
        onclose: vi.fn(),
        options: [
          { id: "a", label: "Todo", checked: true },
          { id: "b", label: "Done", checked: false, disabled: true, reason },
        ],
      },
    });
    const done = screen.getByRole("menuitemcheckbox", { name: /Done/ });
    expect(done).toBeDisabled();
    expect(screen.getByText(reason)).toBeInTheDocument();
    await fireEvent.click(done);
    expect(onselect).not.toHaveBeenCalled();
  });
});
