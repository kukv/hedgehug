import { cleanup, fireEvent, render } from "@testing-library/svelte";
import { createRawSnippet, flushSync, mount, unmount } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import Dialog from "./Dialog.svelte";
import Menu from "./Menu.svelte";

const body = createRawSnippet(() => ({
  render: () => `<div><input aria-label="名前" /><input aria-label="本文" /></div>`,
}));
const foot = createRawSnippet(() => ({ render: () => `<button type="button">作成</button>` }));

beforeEach(() => {
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
});
afterEach(cleanup);

function open(over: Record<string, unknown> = {}) {
  const onclose = vi.fn();
  const r = render(Dialog, {
    props: {
      open: true,
      title: "タスクを作成",
      width: 620,
      onclose,
      children: body,
      footer: foot,
      ...over,
    },
  });
  flushSync();
  return { onclose, ...r };
}

describe("Dialog", () => {
  it("title を見出しにし、role=dialog・aria-modal・aria-labelledby を付ける", () => {
    const { getByRole } = open();
    const d = getByRole("dialog", { name: "タスクを作成" });
    expect(d.getAttribute("aria-modal")).toBe("true");
    expect(getByRole("heading", { name: "タスクを作成" })).toBeTruthy();
  });

  it("open=false なら何も出さない", () => {
    const { queryByRole } = open({ open: false });
    expect(queryByRole("dialog")).toBeNull();
  });

  it("幅を max-width にする", () => {
    const { getByRole } = open({ width: 760 });
    expect((getByRole("dialog") as HTMLElement).style.maxWidth).toBe("760px");
  });

  it("Esc で閉じる", async () => {
    const { onclose } = open();
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(onclose).toHaveBeenCalledTimes(1);
  });

  it("変換中の Esc では閉じない", async () => {
    const { onclose } = open();
    await fireEvent.keyDown(window, { key: "Escape", isComposing: true });
    await fireEvent.keyDown(window, { key: "Escape", keyCode: 229 });
    expect(onclose).not.toHaveBeenCalled();
  });

  it("閉じるボタンで閉じる", async () => {
    const { onclose, getByRole } = open();
    await fireEvent.click(getByRole("button", { name: "閉じる" }));
    expect(onclose).toHaveBeenCalledTimes(1);
  });

  it("開いたら本文の最初の入力欄にフォーカスする", async () => {
    const { getByLabelText } = open();
    await vi.waitFor(() => expect(document.activeElement).toBe(getByLabelText("名前")));
  });

  it("Tab は末尾から先頭へ、⇧Tab は先頭から末尾へ回り、外へ出ない", async () => {
    const { getByRole, getByLabelText } = open();
    await vi.waitFor(() => expect(document.activeElement).toBe(getByLabelText("名前")));
    const close = getByRole("button", { name: "閉じる" });
    const last = getByRole("button", { name: "作成" });
    last.focus();
    const tab = await fireEvent.keyDown(last, { key: "Tab" });
    expect(tab).toBe(false); // preventDefault された
    expect(document.activeElement).toBe(close);
    const back = await fireEvent.keyDown(close, { key: "Tab", shiftKey: true });
    expect(back).toBe(false);
    expect(document.activeElement).toBe(last);
  });

  it("cli があれば足元に CliLine を出す", () => {
    const { getByRole } = open({ cli: "hh add Foo" });
    expect(getByRole("button", { name: /hh add Foo/ })).toBeTruthy();
  });

  it("Dialog の中の Menu で Esc を押すと Menu だけが閉じ、Dialog は閉じない", async () => {
    const onclose = vi.fn();
    const onMenuClose = vi.fn();
    const inner = createRawSnippet(() => ({ render: () => "<div></div>" }));
    const d = render(Dialog, {
      props: { open: true, title: "親", width: 560, onclose, children: inner },
    });
    flushSync();
    // The menu is rendered inside the dialog's own DOM.
    const host = d.getByRole("dialog");
    const m = mount(Menu, {
      target: host,
      props: {
        anchor: null,
        items: [{ value: "a", label: "A" }],
        onselect: vi.fn(),
        onclose: onMenuClose,
      },
    });
    flushSync();
    const menu = d.getByRole("menu");
    expect(host.contains(menu)).toBe(true);
    await fireEvent.keyDown(menu, { key: "Escape" });
    expect(onMenuClose).toHaveBeenCalledTimes(1);
    expect(onclose).not.toHaveBeenCalled();
    unmount(m);
  });

  it("重ねて開いたときは一番上のダイアログだけが Esc で閉じる", async () => {
    const a = vi.fn();
    const b = vi.fn();
    render(Dialog, { props: { open: true, title: "下", width: 560, onclose: a, children: body } });
    render(Dialog, { props: { open: true, title: "上", width: 560, onclose: b, children: body } });
    flushSync();
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(b).toHaveBeenCalledTimes(1);
    expect(a).not.toHaveBeenCalled();
  });
});
