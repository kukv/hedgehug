import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import PropField from "./PropField.svelte";
import type { PropRow } from "./prop-rows";

const row = (over: Partial<PropRow>): PropRow => ({
  name: "p",
  label: "p",
  editor: "text",
  multi: false,
  options: [],
  value: null,
  ...over,
});

describe("PropField", () => {
  it("shows a bool as a choice of はい / いいえ and sends a boolean", async () => {
    const onedit = vi.fn();
    const oncommit = vi.fn();
    render(PropField, {
      row: row({ name: "ok", label: "ok", editor: "bool" }),
      value: true,
      onedit,
      oncommit,
    });
    const button = screen.getByRole("button", { name: "ok" });
    expect(button).toHaveTextContent("はい");
    await fireEvent.click(button);
    await fireEvent.click(screen.getByRole("menuitem", { name: "いいえ" }));
    expect(onedit).toHaveBeenCalledWith(false);
    expect(oncommit).toHaveBeenCalledTimes(1);
  });

  it("uses a menu for a single enum and chips for a multi one", () => {
    const options = ["low", "mid"];
    const { unmount } = render(PropField, {
      row: row({ label: "priority", editor: "enum", options }),
      value: "low",
      onedit: vi.fn(),
      oncommit: vi.fn(),
    });
    expect(screen.getByRole("button", { name: "priority" })).toBeInTheDocument();
    unmount();
    render(PropField, {
      row: row({ label: "tags", editor: "multi_enum", multi: true, options }),
      value: ["low"],
      onedit: vi.fn(),
      oncommit: vi.fn(),
    });
    expect(screen.getByRole("combobox", { name: "tags" })).toBeInTheDocument();
  });
});
