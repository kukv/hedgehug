import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import Select from "./Select.svelte";

describe("Select", () => {
  it("reports the chosen value to onchange", async () => {
    const onchange = vi.fn();
    const options = [
      { value: "a", label: "A" },
      { value: "b", label: "B" },
    ];
    render(Select, { props: { label: "種類", value: "a", options, onchange } });
    await fireEvent.change(screen.getByRole("combobox", { name: "種類" }), {
      target: { value: "b" },
    });
    expect(onchange).toHaveBeenCalledWith("b");
  });
});
