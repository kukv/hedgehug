import { cleanup, fireEvent, render } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { resetToasts, toast, toasts } from "../toast.svelte";
import Avatar from "./Avatar.svelte";
import Button from "./Button.svelte";
import Chip from "./Chip.svelte";
import CliLine from "./CliLine.svelte";
import IdLabel from "./IdLabel.svelte";
import StoreBadge from "./StoreBadge.svelte";
import Toasts from "./Toasts.svelte";

const label = (text: string) => createRawSnippet(() => ({ render: () => `<span>${text}</span>` }));
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

describe("Chip / StoreBadge / Avatar", () => {
  it("Chip は tone を属性に出す", () => {
    const { container } = render(Chip, { props: { tone: "warning", children: label("制約違反") } });
    expect(container.querySelector("[data-tone='warning']")?.textContent).toContain("制約違反");
  });

  it("StoreBadge は Store 名と tone を出す", () => {
    const { container, getByTitle } = render(StoreBadge, {
      props: { store: "shared", tone: "purple" },
    });
    expect(getByTitle("Store: shared").textContent).toContain("shared");
    expect(container.querySelector("[data-tone='purple']")).toBeTruthy();
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
  it("ID を出し、フォーカスで ULID とコピーのボタンを出す", async () => {
    const { getByRole, queryByText, getByText } = render(IdLabel, {
      props: { id: "REL-6", ulid: "01HXYZULID" },
    });
    expect(getByText("REL-6")).toBeTruthy();
    expect(queryByText("01HXYZULID")).toBeNull();
    getByRole("button", { name: /REL-6/ }).focus();
    expect(await findAfter(() => queryByText("01HXYZULID"))).toBeTruthy();
    await fireEvent.click(getByRole("button", { name: "ULID をコピー" }));
    expect(writeText).toHaveBeenCalledWith("01HXYZULID");
  });

  it("alwaysUlid なら常に ULID を出す", () => {
    const { getByText } = render(IdLabel, {
      props: { id: "REL-6", ulid: "01HXYZULID", alwaysUlid: true },
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
