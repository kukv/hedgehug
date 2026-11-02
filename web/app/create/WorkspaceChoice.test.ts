import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { workspaces } from "../detail/testkit";
import WorkspaceChoice from "./WorkspaceChoice.svelte";

describe("WorkspaceChoice", () => {
  it("lists every workspace with its prefix and store, and reports the chosen one", async () => {
    const onchange = vi.fn();
    render(WorkspaceChoice, { entries: workspaces, value: "shared/W1", name: "ws", onchange });
    expect(screen.getByRole("radiogroup", { name: "ワークスペース" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /release（REL）/ })).toBeChecked();
    await fireEvent.click(screen.getByRole("radio", { name: /shared（SH）/ }));
    expect(onchange).toHaveBeenCalledWith("shared/W2");
  });
});
