import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import BlockedBadge from "./BlockedBadge.svelte";
import KindIcon from "./KindIcon.svelte";
import LayoutSwitch from "./LayoutSwitch.svelte";
import ReadOnlyBand from "./ReadOnlyBand.svelte";
import StatusDot from "./StatusDot.svelte";
import StoreBadge from "./StoreBadge.svelte";

describe("KindIcon", () => {
  it.each([
    ["Epic", "E"],
    ["Step", "✓"],
    ["Project", "P"],
  ] as const)("shows %s as %s", (kind, glyph) => {
    render(KindIcon, { props: { kind } });
    expect(screen.getByRole("img", { name: kind })).toHaveTextContent(glyph);
  });
});

describe("StoreBadge", () => {
  it("shows the Store name in its color", () => {
    render(StoreBadge, { props: { name: "work", color: "teal" } });
    const badge = screen.getByText("work");
    expect(badge.getAttribute("style")).toContain("--t-teal-bg");
  });
});

describe("StatusDot", () => {
  it("is named by its label", () => {
    render(StatusDot, { props: { category: "doing", label: "InProgress" } });
    expect(screen.getByRole("img", { name: "InProgress" })).toHaveClass("doing");
  });
});

describe("ReadOnlyBand", () => {
  it("shows the text as a note", () => {
    render(ReadOnlyBand, { props: { text: "ro は閲覧のみの Store です。" } });
    expect(screen.getByRole("note")).toHaveTextContent("ro は閲覧のみの Store です。");
  });
});

describe("BlockedBadge", () => {
  it("says the item is blocked", () => {
    render(BlockedBadge);
    expect(screen.getByText("ブロック中")).toBeInTheDocument();
  });
});

describe("LayoutSwitch", () => {
  it("presses the chosen layout and answers with the id", async () => {
    const onchange = vi.fn();
    const choices = [
      { id: "tree", label: "ツリー" },
      { id: "table", label: "テーブル" },
    ];
    render(LayoutSwitch, { props: { choices, value: "tree", onchange } });
    const group = screen.getByRole("group", { name: "レイアウト" });
    const buttons = Array.from(group.querySelectorAll("button"));
    expect(buttons.map((b) => b.textContent?.trim())).toEqual(["ツリー", "テーブル"]);
    expect(screen.getByRole("button", { name: "ツリー" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "テーブル" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    await fireEvent.click(screen.getByRole("button", { name: "テーブル" }));
    expect(onchange).toHaveBeenCalledWith("table");
  });
});
