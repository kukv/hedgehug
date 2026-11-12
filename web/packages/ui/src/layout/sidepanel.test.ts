import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import ValueMenu from "../patterns/ValueMenu.svelte";
import Dialog from "../primitives/Dialog.svelte";
import { text } from "../test-snippet";
import SidePanel from "./SidePanel.svelte";

const base = { label: "詳細", head: text("見出し"), children: text("本文") };

describe("SidePanel", () => {
  it("renders nothing when closed", () => {
    render(SidePanel, { props: { ...base, open: false, onclose: vi.fn() } });
    expect(screen.queryByRole("region", { name: "詳細" })).toBeNull();
  });

  it("shows the head and the body, and marks full", () => {
    render(SidePanel, { props: { ...base, open: true, full: true, onclose: vi.fn() } });
    const panel = screen.getByRole("region", { name: "詳細" });
    expect(panel).toHaveTextContent("見出し");
    expect(panel).toHaveTextContent("本文");
    expect(panel).toHaveClass("full");
  });

  it("closes on Escape", async () => {
    const onclose = vi.fn();
    render(SidePanel, { props: { ...base, open: true, onclose } });
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(onclose).toHaveBeenCalledOnce();
  });

  it("leaves Escape to an open dialog", async () => {
    const onclose = vi.fn();
    render(SidePanel, { props: { ...base, open: true, onclose } });
    render(Dialog, { props: { open: true, title: "作成", onclose: vi.fn(), children: text("x") } });
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(onclose).not.toHaveBeenCalled();
  });

  it("leaves Escape to an open menu", async () => {
    const onclose = vi.fn();
    render(SidePanel, { props: { ...base, open: true, onclose } });
    render(ValueMenu, {
      props: {
        open: true,
        x: 0,
        y: 0,
        label: "優先度",
        options: [],
        onselect: vi.fn(),
        onclose: vi.fn(),
      },
    });
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(onclose).not.toHaveBeenCalled();
  });
});
