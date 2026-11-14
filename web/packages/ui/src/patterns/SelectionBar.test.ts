import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import SelectionBar from "./SelectionBar.svelte";

describe("SelectionBar", () => {
  it("shows the count and answers with the buttons", async () => {
    const ondelete = vi.fn();
    const onclear = vi.fn();
    render(SelectionBar, { props: { text: "2 件を選択中", readOnly: false, ondelete, onclear } });
    expect(screen.getByText("2 件を選択中")).toBeInTheDocument();
    const remove = screen.getByRole("button", { name: "一括削除…" });
    expect(remove).toBeEnabled();
    expect(remove).not.toHaveAttribute("title");
    await fireEvent.click(remove);
    expect(ondelete).toHaveBeenCalledOnce();
    await fireEvent.click(screen.getByRole("button", { name: "選択を解除" }));
    expect(onclear).toHaveBeenCalledOnce();
  });

  it("disables only the bulk delete when every row is read-only", async () => {
    const ondelete = vi.fn();
    const onclear = vi.fn();
    render(SelectionBar, { props: { text: "1 件を選択中", readOnly: true, ondelete, onclear } });
    const remove = screen.getByRole("button", { name: "一括削除…" });
    expect(remove).toBeDisabled();
    expect(remove).toHaveAttribute("title", "閲覧のみの Store です");
    await fireEvent.click(remove);
    expect(ondelete).not.toHaveBeenCalled();
    const clear = screen.getByRole("button", { name: "選択を解除" });
    expect(clear).toBeEnabled();
    await fireEvent.click(clear);
    expect(onclear).toHaveBeenCalledOnce();
  });
});
