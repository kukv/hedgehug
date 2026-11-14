import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import RowCheck from "./RowCheck.svelte";

describe("RowCheck", () => {
  it("is named by its label and passes the next value", async () => {
    const onchange = vi.fn();
    render(RowCheck, { props: { label: "HH-2 を選択", checked: false, onchange } });
    const box = screen.getByRole("checkbox", { name: "HH-2 を選択" });
    expect(box).not.toBeChecked();
    expect(box).toBeEnabled();
    await fireEvent.click(box);
    expect(onchange).toHaveBeenCalledWith(true);
  });

  it("shows checked and passes false when unchecked", async () => {
    const onchange = vi.fn();
    render(RowCheck, { props: { label: "HH-3 を選択", checked: true, onchange } });
    const box = screen.getByRole("checkbox", { name: "HH-3 を選択" });
    expect(box).toBeChecked();
    await fireEvent.click(box);
    expect(onchange).toHaveBeenCalledWith(false);
  });
});
