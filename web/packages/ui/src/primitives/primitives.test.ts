import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { text } from "../test-snippet";
import Button from "./Button.svelte";
import IconButton from "./IconButton.svelte";
import Tabs from "./Tabs.svelte";

describe("Button", () => {
  it("calls onclick", async () => {
    const onclick = vi.fn();
    render(Button, { props: { variant: "primary", onclick, children: text("保存") } });
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(onclick).toHaveBeenCalledOnce();
  });

  it("does not call onclick when disabled, and shows the note", async () => {
    const onclick = vi.fn();
    render(Button, {
      props: { disabled: true, title: "閲覧のみの Store です", onclick, children: text("作成") },
    });
    const button = screen.getByRole("button", { name: "作成" });
    await fireEvent.click(button);
    expect(onclick).not.toHaveBeenCalled();
    expect(button).toHaveAttribute("title", "閲覧のみの Store です");
  });

  it("reports a pressed state only when given one", () => {
    render(Button, { props: { pressed: true, children: text("ウォッチ中") } });
    render(Button, { props: { children: text("ほか") } });
    expect(screen.getByRole("button", { name: "ウォッチ中" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: "ほか" })).not.toHaveAttribute("aria-pressed");
  });
});

describe("IconButton", () => {
  it("is named by its label", () => {
    render(IconButton, { props: { label: "メニュー", children: text("≡") } });
    expect(screen.getByRole("button", { name: "メニュー" })).toBeInTheDocument();
  });

  it("does not call onclick when disabled", async () => {
    const onclick = vi.fn();
    render(IconButton, {
      props: { label: "メニュー", disabled: true, onclick, children: text("≡") },
    });
    await fireEvent.click(screen.getByRole("button", { name: "メニュー" }));
    expect(onclick).not.toHaveBeenCalled();
  });
});

describe("Tabs", () => {
  it("marks the current tab and reports the selected one", async () => {
    const onselect = vi.fn();
    const tabs = [
      { id: "tree", label: "ツリー" },
      { id: "board", label: "カンバン", count: 3 },
    ];
    render(Tabs, { props: { tabs, current: "tree", onselect } });
    expect(screen.getByRole("tab", { name: "ツリー" })).toHaveAttribute("aria-selected", "true");
    await fireEvent.click(screen.getByRole("tab", { name: /カンバン/ }));
    expect(onselect).toHaveBeenCalledWith("board");
  });
});
