import { cleanup, fireEvent, render } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fakeCore, renderWithCore } from "../test-utils";
import { resetToasts, toast, toasts } from "../toast.svelte";
import Avatar from "./Avatar.svelte";
import Button from "./Button.svelte";
import CliLine from "./CliLine.svelte";
import IdLabel from "./IdLabel.svelte";
import StoreBadge from "./StoreBadge.svelte";
import Toasts from "./Toasts.svelte";

const label = (text: string) => createRawSnippet(() => ({ render: () => `<span>${text}</span>` }));
const fakeStores = () => fakeCore().stores;
const writeText = vi.fn().mockResolvedValue(undefined);

beforeEach(() => {
  writeText.mockClear();
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
  resetToasts();
});
afterEach(cleanup);

describe("Button", () => {
  it("variant と size を属性に出し、押すと onclick が呼ばれる", async () => {
    const onclick = vi.fn();
    const { getByRole } = render(Button, {
      props: { variant: "primary", size: "sm", onclick, children: label("作成") },
    });
    const b = getByRole("button", { name: "作成" });
    expect(b.getAttribute("data-variant")).toBe("primary");
    expect(b.getAttribute("data-size")).toBe("sm");
    expect(b.getAttribute("type")).toBe("button");
    await fireEvent.click(b);
    expect(onclick).toHaveBeenCalledTimes(1);
  });

  it("disabled は無効の属性を持つ", () => {
    const { getByRole } = render(Button, { props: { disabled: true, children: label("作成") } });
    expect((getByRole("button") as HTMLButtonElement).disabled).toBe(true);
  });
});

describe("StoreBadge / Avatar", () => {
  it("StoreBadge は Store 名と tone を出す", () => {
    const { container, getByTitle } = renderWithCore(StoreBadge, {
      store: "shared",
      tone: "purple",
    });
    expect(getByTitle("Store: shared").textContent).toContain("shared");
    expect(container.querySelector("[data-tone='purple']")).toBeTruthy();
  });

  it("StoreBadge は閲覧のみの Store なら title と読み上げに「閲覧のみ」を足す", () => {
    const { getByTitle } = renderWithCore(
      StoreBadge,
      { store: "ro", tone: "readonly" },
      { stores: { readOnly: () => "the remote cannot be written", subscribe: () => () => {} } },
    );
    expect(getByTitle("ro（閲覧のみ）").textContent).toContain("（閲覧のみ）");
  });

  it("StoreBadge は tone が灰色でも Store が書けるなら「閲覧のみ」を足さない", () => {
    const { getByTitle } = renderWithCore(StoreBadge, { store: "x", tone: "readonly" });
    expect(getByTitle("Store: x").textContent).not.toContain("閲覧のみ");
  });

  it("Avatar は名前の先頭2文字を大文字で出す", () => {
    const { getByRole } = render(Avatar, { props: { name: "koki" } });
    expect(getByRole("img", { name: "koki" }).textContent).toBe("KO");
  });
});

describe("CliLine", () => {
  it("$ 付きで出し、クリックで $ なしをコピーする", async () => {
    const { getByRole, findByText } = render(CliLine, { props: { command: 'hh add "Fix bug"' } });
    const b = getByRole("button");
    expect(b.textContent).toContain('$ hh add "Fix bug"');
    await fireEvent.click(b);
    expect(writeText).toHaveBeenCalledWith('hh add "Fix bug"');
    expect(await findByText("コピーしました")).toBeTruthy();
  });
});

describe("IdLabel", () => {
  const shared = (store: string, id: string) => (id.startsWith("DM-") ? `${store}:${id}` : id);

  it("store を渡すと、接頭辞が重なる ID に Store 名を付ける", () => {
    const { getByText } = renderWithCore(
      IdLabel,
      { id: "DM-1", ulid: "01HXYZULID", store: "second" },
      { stores: { ...fakeStores(), displayId: shared } },
    );
    expect(getByText("second:DM-1")).toBeTruthy();
  });

  it("接頭辞が重ならなければ Store 名を付けない", () => {
    const { getByText } = renderWithCore(
      IdLabel,
      { id: "REL-6", ulid: "01HXYZULID", store: "second" },
      { stores: { ...fakeStores(), displayId: shared } },
    );
    expect(getByText("REL-6")).toBeTruthy();
  });

  it("ID を出し、フォーカスで ULID とコピーのボタンを出す", async () => {
    const { getByRole, queryByText, getByText } = renderWithCore(IdLabel, {
      id: "REL-6",
      ulid: "01HXYZULID",
      store: "shared",
    });
    expect(getByText("REL-6")).toBeTruthy();
    expect(queryByText("01HXYZULID")).toBeNull();
    getByRole("button", { name: /REL-6/ }).focus();
    expect(await findAfter(() => queryByText("01HXYZULID"))).toBeTruthy();
    await fireEvent.click(getByRole("button", { name: "ULID をコピー" }));
    expect(writeText).toHaveBeenCalledWith("01HXYZULID");
  });

  it("ホバーで ULID とコピーのボタンを出し、外れると消す", async () => {
    const { getByRole, queryByText, getByText } = renderWithCore(IdLabel, {
      id: "REL-6",
      ulid: "01HXYZULID",
      store: "shared",
    });
    const wrap = getByText("REL-6").parentElement as HTMLElement;
    await fireEvent.mouseEnter(wrap);
    expect(await findAfter(() => queryByText("01HXYZULID"))).toBeTruthy();
    await fireEvent.click(getByRole("button", { name: "ULID をコピー" }));
    expect(writeText).toHaveBeenCalledWith("01HXYZULID");
    await fireEvent.mouseLeave(wrap);
    expect(await findAfter(() => (queryByText("01HXYZULID") ? null : true))).toBe(true);
  });

  it("ID のボタンは Tab の止まり先にならず、reveal なら ULID とコピーのボタンを出す", () => {
    const { getByRole, getByText } = renderWithCore(IdLabel, {
      id: "REL-6",
      ulid: "01HXYZULID",
      store: "shared",
      reveal: true,
    });
    expect(getByRole("button", { name: /REL-6/ })).toHaveAttribute("tabindex", "-1");
    expect(getByText("01HXYZULID")).toBeTruthy();
    expect(getByRole("button", { name: "ULID をコピー" })).not.toHaveAttribute("tabindex");
  });

  it("href の ID にフォーカスがあっても keydown は window に届く（C・⌘K のため）", async () => {
    const seen = vi.fn();
    window.addEventListener("keydown", seen);
    const { getByRole } = renderWithCore(IdLabel, {
      id: "REL-6",
      ulid: "01HXYZULID",
      store: "shared",
      href: "/tasks/01HXYZULID",
    });
    await fireEvent.keyDown(getByRole("link", { name: "REL-6" }), { key: "c" });
    window.removeEventListener("keydown", seen);
    expect(seen).toHaveBeenCalledTimes(1);
  });

  it("alwaysUlid なら常に ULID を出す", () => {
    const { getByText } = renderWithCore(IdLabel, {
      id: "REL-6",
      ulid: "01HXYZULID",
      store: "shared",
      alwaysUlid: true,
    });
    expect(getByText("01HXYZULID")).toBeTruthy();
  });
});

describe("Toasts", () => {
  it("メッセージとリンクと取り消しを出し、取り消しで undo を呼ぶ", async () => {
    const undo = vi.fn();
    const { getByText, getByRole, queryByText } = render(Toasts);
    toast({ message: "REL-9 を作成しました", link: { label: "開く", href: "/tasks/01H" }, undo });
    expect(await findAfter(() => queryByText("REL-9 を作成しました"))).toBeTruthy();
    expect(getByText("開く").getAttribute("href")).toBe("/tasks/01H");
    await fireEvent.click(getByRole("button", { name: "取り消し" }));
    expect(undo).toHaveBeenCalledTimes(1);
    expect(toasts.list).toHaveLength(0);
  });
});

async function findAfter<T>(read: () => T | null): Promise<T | null> {
  for (let i = 0; i < 20; i++) {
    const v = read();
    if (v) return v;
    await new Promise((r) => setTimeout(r, 0));
  }
  return null;
}
