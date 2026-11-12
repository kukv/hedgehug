import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import Checkbox from "./Checkbox.svelte";
import Input from "./Input.svelte";

describe("Input", () => {
  it("reports blur and keydown, and takes focus when asked", async () => {
    const onblur = vi.fn();
    const onkeydown = vi.fn();
    render(Input, {
      props: { label: "タイトル", value: "認証", focusOnMount: true, onblur, onkeydown },
    });
    const input = screen.getByRole("textbox", { name: "タイトル" });
    expect(input).toHaveValue("認証");
    expect(input).toHaveFocus();
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(onkeydown).toHaveBeenCalledWith(expect.objectContaining({ key: "Enter" }));
    await fireEvent.blur(input);
    expect(onblur).toHaveBeenCalledOnce();
  });

  it("does not take focus unless asked", () => {
    render(Input, { props: { label: "タイトル" } });
    expect(screen.getByRole("textbox", { name: "タイトル" })).not.toHaveFocus();
  });

  it("is a date field when asked", () => {
    render(Input, { props: { label: "開始日", type: "date", value: "2026-11-13" } });
    const input = screen.getByLabelText("開始日");
    expect(input).toHaveAttribute("type", "date");
    expect(input).toHaveValue("2026-11-13");
  });

  it("is disabled with the note", () => {
    render(Input, {
      props: { label: "タイトル", heading: true, disabled: true, title: "閲覧のみの Store です" },
    });
    const input = screen.getByRole("textbox", { name: "タイトル" });
    expect(input).toBeDisabled();
    expect(input).toHaveAttribute("title", "閲覧のみの Store です");
  });
});

describe("Checkbox", () => {
  it("is named by its label and toggles", async () => {
    render(Checkbox, { props: { label: "続けて作成" } });
    const box = screen.getByRole("checkbox", { name: "続けて作成" });
    expect(box).not.toBeChecked();
    await fireEvent.click(box);
    expect(box).toBeChecked();
  });

  it("is disabled", () => {
    render(Checkbox, { props: { label: "続けて作成", disabled: true } });
    expect(screen.getByRole("checkbox", { name: "続けて作成" })).toBeDisabled();
  });
});

describe("Input onfocus", () => {
  it("reports focus", async () => {
    const onfocus = vi.fn();
    render(Input, { props: { label: "タイトル", onfocus } });
    await fireEvent.focus(screen.getByLabelText("タイトル"));
    expect(onfocus).toHaveBeenCalledOnce();
  });
});
