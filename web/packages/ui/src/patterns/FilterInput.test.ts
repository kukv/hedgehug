import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import type { FilterOption } from "../types";
import FilterInput from "./FilterInput.svelte";

const options: FilterOption[] = [
  { label: "status:" },
  { label: "assignee:", dot: "blue" },
  { label: "sprint:" },
];

function setup(over: Record<string, unknown> = {}) {
  const props = {
    value: "st",
    placeholder: "絞り込み",
    options,
    errors: [] as string[],
    oninput: vi.fn(),
    onchoose: vi.fn(),
    onhelp: vi.fn(),
    ...over,
  };
  render(FilterInput, { props });
  return props;
}
const box = () => screen.getByRole("combobox");

describe("FilterInput", () => {
  it("入力欄に data-filter-input とプレースホルダを持ち、入力を文字とカーソル位置で渡す", async () => {
    const p = setup();
    expect(box()).toHaveAttribute("data-filter-input");
    expect(box()).toHaveAttribute("placeholder", "絞り込み");
    await fireEvent.input(box(), { target: { value: "sta" } });
    expect(p.oninput).toHaveBeenCalledWith("sta", expect.any(Number));
  });

  it("候補はフォーカスがあるあいだだけ出る", async () => {
    setup();
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(box()).toHaveAttribute("aria-expanded", "false");
    await fireEvent.focus(box());
    expect(screen.getAllByRole("option")).toHaveLength(3);
    expect(box()).toHaveAttribute("aria-expanded", "true");
    await fireEvent.blur(box());
    expect(screen.queryByRole("listbox")).toBeNull();
  });

  it("↑↓ で選び、Enter で選んだ位置を渡す", async () => {
    const p = setup();
    await fireEvent.focus(box());
    await fireEvent.keyDown(box(), { key: "ArrowDown" });
    expect(screen.getAllByRole("option")[1] as HTMLElement).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await fireEvent.keyDown(box(), { key: "ArrowUp" });
    await fireEvent.keyDown(box(), { key: "ArrowUp" });
    expect(screen.getAllByRole("option")[2]).toHaveAttribute("aria-selected", "true");
    await fireEvent.keyDown(box(), { key: "Enter" });
    expect(p.onchoose).toHaveBeenCalledWith(2);
  });

  it("クリックで選べる", async () => {
    const p = setup();
    await fireEvent.focus(box());
    await fireEvent.click(screen.getAllByRole("option")[1] as HTMLElement);
    expect(p.onchoose).toHaveBeenCalledWith(1);
  });

  it("変換中の Enter は何もしない", async () => {
    const p = setup();
    await fireEvent.focus(box());
    await fireEvent.keyDown(box(), { key: "Enter", isComposing: true });
    await fireEvent.keyDown(box(), { key: "Enter", keyCode: 229 });
    expect(p.onchoose).not.toHaveBeenCalled();
  });

  it("候補が出ていない Enter は何もしない", async () => {
    const p = setup({ options: [] });
    await fireEvent.focus(box());
    await fireEvent.keyDown(box(), { key: "Enter" });
    expect(p.onchoose).not.toHaveBeenCalled();
  });

  it("候補が出ているときの Esc は候補だけを閉じ、外へ伝えない。2 回目は伝わる", async () => {
    setup();
    const outer = vi.fn();
    window.addEventListener("keydown", outer);
    await fireEvent.focus(box());
    await fireEvent.keyDown(box(), { key: "Escape" });
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(outer).not.toHaveBeenCalled();
    await fireEvent.keyDown(box(), { key: "Escape" });
    expect(outer).toHaveBeenCalledTimes(1);
    window.removeEventListener("keydown", outer);
  });

  it("閉じたあと、入力するとまた候補が出る", async () => {
    setup();
    await fireEvent.focus(box());
    await fireEvent.keyDown(box(), { key: "Escape" });
    await fireEvent.input(box(), { target: { value: "sta" } });
    expect(screen.getAllByRole("option")).toHaveLength(3);
  });

  it("Store の点を出す", async () => {
    setup();
    await fireEvent.focus(box());
    expect(
      (screen.getAllByRole("option")[1] as HTMLElement).querySelector(".store-dot"),
    ).not.toBeNull();
    expect(
      (screen.getAllByRole("option")[0] as HTMLElement).querySelector(".store-dot"),
    ).toBeNull();
  });

  it("「検索ヘルプ」のボタンで onhelp", async () => {
    const p = setup();
    await fireEvent.click(screen.getByRole("button", { name: "検索ヘルプ" }));
    expect(p.onhelp).toHaveBeenCalled();
  });

  it("エラーを 1 行ずつ role=alert で出す", () => {
    setup({ errors: ["「foo」というキーはありません", "「status:」に値がありません"] });
    const alerts = screen.getAllByRole("alert");
    expect(alerts).toHaveLength(2);
    expect(alerts[0]).toHaveTextContent("「foo」というキーはありません");
  });
});
