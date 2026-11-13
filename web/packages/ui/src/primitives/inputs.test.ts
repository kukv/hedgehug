import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import Checkbox from "./Checkbox.svelte";
import Input from "./Input.svelte";
import Switch from "./Switch.svelte";

describe("Input", () => {
  it("reports the typed value as it changes", async () => {
    const oninput = vi.fn();
    render(Input, { props: { label: "相手", oninput } });
    await fireEvent.input(screen.getByRole("textbox", { name: "相手" }), {
      target: { value: "HH" },
    });
    expect(oninput).toHaveBeenCalledWith("HH");
  });

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

  it("is a number field with a step and a minimum, and hands back what was typed as a string", async () => {
    const seen: unknown[] = [];
    render(Input, {
      props: {
        label: "時間",
        type: "number",
        step: "0.25",
        min: "0",
        get value() {
          return "";
        },
        set value(v: string) {
          seen.push(v);
        },
      },
    });
    const input = screen.getByRole("spinbutton", { name: "時間" });
    expect(input).toHaveAttribute("step", "0.25");
    expect(input).toHaveAttribute("min", "0");
    await fireEvent.input(input, { target: { value: "1.50" } });
    expect(seen).toEqual(["1.50"]);
  });

  it("shows the value the parent keeps when the parent changes what was typed", async () => {
    render(Input, {
      props: {
        label: "接頭辞",
        get value() {
          return "A";
        },
        set value(_v: string) {},
      },
    });
    const input = screen.getByRole("textbox", { name: "接頭辞" });
    await fireEvent.input(input, { target: { value: "a" } });
    expect(input).toHaveValue("A");
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

describe("Switch", () => {
  it("with onchange stays as given and reports the next value", async () => {
    const seen: boolean[] = [];
    render(Switch, {
      props: { label: "有効", checked: false, onchange: (v: boolean) => seen.push(v) },
    });
    const sw = screen.getByRole("switch", { name: "有効" });
    await fireEvent.click(sw);
    expect(seen).toEqual([true]);
    expect(sw).not.toBeChecked();
  });

  it("without onchange toggles as before", async () => {
    render(Switch, { props: { label: "有効" } });
    const sw = screen.getByRole("switch", { name: "有効" });
    await fireEvent.click(sw);
    expect(sw).toBeChecked();
  });
});
