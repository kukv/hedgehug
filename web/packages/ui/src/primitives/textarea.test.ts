import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import Textarea from "./Textarea.svelte";

describe("Textarea", () => {
  it("is named by its label, takes the rows, and puts the caret at the end when it takes focus", () => {
    render(Textarea, { props: { label: "説明", rows: 8, value: "本文", focusOnMount: true } });
    const box = screen.getByRole("textbox", { name: "説明" }) as HTMLTextAreaElement;
    expect(box).toHaveAttribute("rows", "8");
    expect(box).toHaveValue("本文");
    expect(box).toHaveFocus();
    expect(box.selectionStart).toBe(2);
  });

  it("does not take focus unless asked", () => {
    render(Textarea, { props: { label: "説明" } });
    expect(screen.getByRole("textbox", { name: "説明" })).not.toHaveFocus();
  });

  it("reports keydown", async () => {
    const onkeydown = vi.fn();
    render(Textarea, { props: { label: "説明", onkeydown } });
    await fireEvent.keyDown(screen.getByRole("textbox", { name: "説明" }), { key: "Escape" });
    expect(onkeydown).toHaveBeenCalledWith(expect.objectContaining({ key: "Escape" }));
  });

  it("is disabled with the note", () => {
    render(Textarea, { props: { label: "説明", disabled: true, title: "閲覧のみの Store です" } });
    const box = screen.getByRole("textbox", { name: "説明" });
    expect(box).toBeDisabled();
    expect(box).toHaveAttribute("title", "閲覧のみの Store です");
  });
});
