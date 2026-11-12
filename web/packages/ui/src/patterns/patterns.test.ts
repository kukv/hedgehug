import { render, screen } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import KindIcon from "./KindIcon.svelte";
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
