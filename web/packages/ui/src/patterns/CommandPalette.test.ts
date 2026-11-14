import { fireEvent, render, screen } from "@testing-library/svelte";
import { tick } from "svelte";
import { describe, expect, it, vi } from "vitest";
import Dialog from "../primitives/Dialog.svelte";
import { text } from "../test-snippet";
import type { PaletteView } from "../types";
import CommandPalette from "./CommandPalette.svelte";

const footer = "↑↓ で選択 ・ Enter で開く ・ Ctrl+Enter で新しいタブ ・ Esc で閉じる";
const palette = (over: Partial<PaletteView> = {}): PaletteView => ({
  input: "",
  rows: [
    { kind: "head", text: "HH-1 に対して" },
    { kind: "candidate", c: { key: "a", label: "ステータスを変更…" } },
    { kind: "candidate", c: { key: "b", label: "詳細を開く", hint: "Enter" } },
    { kind: "candidate", c: { key: "c", label: "Done", disabled: true, hint: "移れません" } },
    { kind: "more", text: "ほか 3 件あります。入力を続けて絞り込んでください" },
  ],
  selected: 1,
  footer,
  selectedHead: null,
  ...over,
});

function setup(state: PaletteView | null = palette(), mac = false) {
  const props = {
    view: state,
    mac,
    onopen: vi.fn(),
    oninput: vi.fn(),
    onmove: vi.fn(),
    onchoose: vi.fn(),
    onclose: vi.fn(),
  };
  const r = render(CommandPalette, { props });
  return { ...props, rerender: r.rerender };
}

// opener は、パレットを開く前にフォーカスのある入力欄。
function opener() {
  const input = document.createElement("input");
  input.setAttribute("aria-label", "開く前");
  document.body.append(input);
  input.focus();
  return input;
}

describe("CommandPalette", () => {
  it("プレースホルダ「ID・タイトル・コマンドを入力」と下端の案内を出し、role=dialog・aria-modal=true", () => {
    setup();
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAttribute("aria-modal", "true");
    const input = screen.getByRole("combobox", { name: "コマンドパレット" });
    expect(input).toHaveAttribute("placeholder", "ID・タイトル・コマンドを入力");
    expect(input).toHaveFocus();
    expect(dialog).toHaveTextContent(footer);
  });

  it("閉じているときは何も出さない", () => {
    setup(null);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("見出し・押せない行・選んでいる行を描き、押せない行のクリックでは実行しない", async () => {
    const p = setup();
    expect(screen.getByText("HH-1 に対して")).toBeInTheDocument();
    const options = screen.getAllByRole("option");
    expect(options).toHaveLength(3);
    expect(options[0]).toHaveAttribute("aria-selected", "true");
    expect(options[1]).toHaveAttribute("aria-selected", "false");
    expect(options[1]).toHaveTextContent("Enter");
    expect(options[2]).toHaveAttribute("aria-disabled", "true");
    await fireEvent.click(options[2] as HTMLElement);
    await fireEvent.click(screen.getByText("HH-1 に対して"));
    await fireEvent.click(screen.getByText(/ほか 3 件あります/));
    expect(p.onchoose).not.toHaveBeenCalled();
    await fireEvent.click(options[1] as HTMLElement);
    expect(p.onchoose).toHaveBeenCalledWith(2, false);
  });

  it("ステータスの一覧のいまのステータスにチェックを付ける", () => {
    setup(
      palette({ rows: [{ kind: "candidate", c: { key: "s", label: "Todo", checked: true } }] }),
    );
    expect(screen.getByRole("option")).toHaveTextContent("✓");
  });

  it("選んでいるアイテムの見出しの頭に Store の点を付ける", () => {
    setup(palette({ selectedHead: { text: "HH-1 に対して", storeColor: "teal" } }));
    const dot = screen.getByText("HH-1 に対して").querySelector(".store-dot");
    expect(dot?.getAttribute("style")).toContain("--t-teal-fg");
  });

  it("入力を渡し、↑↓ で選択を動かす", async () => {
    const p = setup();
    await fireEvent.input(screen.getByRole("combobox"), { target: { value: "hh" } });
    expect(p.oninput).toHaveBeenCalledWith("hh");
    await fireEvent.keyDown(window, { key: "ArrowDown" });
    await fireEvent.keyDown(window, { key: "ArrowUp" });
    expect(p.onmove.mock.calls).toEqual([["down"], ["up"]]);
  });

  it("Enter で選んでいる行を実行し、Ctrl+Enter で choose(…, true)", async () => {
    const p = setup();
    await fireEvent.keyDown(window, { key: "Enter" });
    await fireEvent.keyDown(window, { key: "Enter", ctrlKey: true });
    expect(p.onchoose.mock.calls).toEqual([
      [undefined, false],
      [undefined, true],
    ]);
  });

  it("Mac では Cmd+Enter で choose(…, true)", async () => {
    const p = setup(palette(), true);
    await fireEvent.keyDown(window, { key: "Enter", metaKey: true });
    expect(p.onchoose).toHaveBeenCalledWith(undefined, true);
  });

  it("変換中の Enter では実行せず、変換中の Esc では閉じない", async () => {
    const p = setup();
    await fireEvent.keyDown(window, { key: "Enter", isComposing: true });
    await fireEvent.keyDown(window, { key: "Enter", keyCode: 229 });
    await fireEvent.keyDown(window, { key: "Escape", isComposing: true });
    await fireEvent.keyDown(window, { key: "Escape", keyCode: 229 });
    expect(p.onchoose).not.toHaveBeenCalled();
    expect(p.onclose).not.toHaveBeenCalled();
  });

  it("変換中の ↑↓ では選択を動かさない", async () => {
    const p = setup();
    await fireEvent.keyDown(window, { key: "ArrowDown", isComposing: true });
    await fireEvent.keyDown(window, { key: "ArrowUp", keyCode: 229 });
    expect(p.onmove).not.toHaveBeenCalled();
  });

  it("Esc・背景のクリックで閉じる", async () => {
    const p = setup();
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(p.onclose).toHaveBeenCalledTimes(1);
    await fireEvent.mouseDown(screen.getByRole("dialog"));
    expect(p.onclose).toHaveBeenCalledTimes(1);
    await fireEvent.mouseDown(screen.getByRole("dialog").parentElement as HTMLElement);
    expect(p.onclose).toHaveBeenCalledTimes(2);
  });
});

describe("CommandPalette のフォーカスと Esc", () => {
  it("Esc で閉じたら開く前の入力欄にフォーカスが戻る", async () => {
    const before = opener();
    const p = setup(null);
    await p.rerender({ view: palette() });
    expect(screen.getByRole("combobox")).toHaveFocus();
    await fireEvent.keyDown(screen.getByRole("combobox"), { key: "Escape" });
    expect(p.onclose).toHaveBeenCalledOnce();
    await p.rerender({ view: null });
    await tick();
    expect(before).toHaveFocus();
    before.remove();
  });

  it("閉じたときに別のダイアログ([aria-modal=true])が開いていればフォーカスを戻さない", async () => {
    const before = opener();
    const p = setup(null);
    await p.rerender({ view: palette() });
    render(Dialog, { props: { open: true, title: "作成", onclose: vi.fn(), children: text("x") } });
    await p.rerender({ view: null });
    await tick();
    expect(before).not.toHaveFocus();
    before.remove();
  });

  it("Esc はパレットだけが受け、ほかの Esc の受け手(詳細パネルなど)へ届けない", async () => {
    const other = vi.fn();
    window.addEventListener("keydown", other);
    const p = setup();
    await fireEvent.keyDown(screen.getByRole("combobox"), { key: "Escape" });
    window.removeEventListener("keydown", other);
    expect(p.onclose).toHaveBeenCalledOnce();
    expect(other).not.toHaveBeenCalled();
  });
});

describe("CommandPalette の Ctrl+K", () => {
  it("Ctrl+Shift+K・Ctrl+Alt+K では開かない", async () => {
    const p = setup(null);
    await fireEvent.keyDown(window, { key: "K", ctrlKey: true, shiftKey: true });
    await fireEvent.keyDown(window, { key: "k", ctrlKey: true, altKey: true });
    expect(p.onopen).not.toHaveBeenCalled();
  });

  it("入力欄にフォーカスがあっても Ctrl+K で開く", async () => {
    const p = setup(null);
    const input = document.createElement("input");
    document.body.append(input);
    input.focus();
    await fireEvent.keyDown(input, { key: "k", ctrlKey: true });
    expect(p.onopen).toHaveBeenCalledTimes(1);
    input.remove();
  });

  it("Mac は Cmd+K で開き、Ctrl+K では開かない", async () => {
    const p = setup(null, true);
    await fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    expect(p.onopen).not.toHaveBeenCalled();
    await fireEvent.keyDown(window, { key: "K", metaKey: true });
    expect(p.onopen).toHaveBeenCalledTimes(1);
  });

  it("ダイアログ([aria-modal=true])を開いているときは開かない", async () => {
    const p = setup(null);
    render(Dialog, { props: { open: true, title: "作成", onclose: vi.fn(), children: text("x") } });
    await fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    expect(p.onopen).not.toHaveBeenCalled();
  });

  it("パレットを開いているときは開き直さない", async () => {
    const p = setup();
    await fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    expect(p.onopen).not.toHaveBeenCalled();
  });
});
