import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import ChipsField from "./ChipsField.svelte";

describe("ChipsField", () => {
  it("adds a value with Enter and on blur, and ignores duplicates", async () => {
    const onchange = vi.fn();
    render(ChipsField, { values: ["koki"], label: "担当者", onchange });
    const input = screen.getByRole("textbox", { name: "担当者" });
    await fireEvent.input(input, { target: { value: "sato" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(onchange).toHaveBeenLastCalledWith(["koki", "sato"]);
    await fireEvent.input(input, { target: { value: "koki" } });
    await fireEvent.blur(input);
    expect(onchange).toHaveBeenCalledTimes(1);
  });

  it("adds the typed chip on Ctrl+Enter and leaves the event to the dialog", async () => {
    const onchange = vi.fn();
    render(ChipsField, { values: [], label: "担当者", onchange });
    const input = screen.getByRole("textbox", { name: "担当者" });
    await fireEvent.input(input, { target: { value: "sato" } });
    const e = new KeyboardEvent("keydown", {
      key: "Enter",
      ctrlKey: true,
      bubbles: true,
      cancelable: true,
    });
    input.dispatchEvent(e);
    expect(onchange).toHaveBeenCalledWith(["sato"]);
    expect(e.defaultPrevented).toBe(false);
  });

  it("keeps the event from the dialog when the typed text is invalid, so nothing is created", async () => {
    const onchange = vi.fn();
    render(ChipsField, {
      values: [],
      label: "リポジトリ",
      onchange,
      transform: (t: string) => (t.includes("/") ? t : null),
      invalidMessage: "読めません",
    });
    const input = screen.getByRole("textbox", { name: "リポジトリ" });
    await fireEvent.input(input, { target: { value: "nope" } });
    const e = new KeyboardEvent("keydown", {
      key: "Enter",
      ctrlKey: true,
      bubbles: true,
      cancelable: true,
    });
    input.dispatchEvent(e);
    expect(e.defaultPrevented).toBe(true);
    expect(await screen.findByRole("alert")).toHaveTextContent("読めません");
    expect(onchange).not.toHaveBeenCalled();
  });

  it("does not add a chip while an IME is converting", async () => {
    const onchange = vi.fn();
    render(ChipsField, { values: [], label: "担当者", onchange });
    const input = screen.getByRole("textbox", { name: "担当者" });
    await fireEvent.input(input, { target: { value: "さとう" } });
    await fireEvent.keyDown(input, { key: "Enter", isComposing: true });
    await fireEvent.keyDown(input, { key: "Enter", keyCode: 229 });
    await fireEvent.keyDown(input, { key: ",", isComposing: true });
    expect(onchange).not.toHaveBeenCalled();
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(onchange).toHaveBeenCalledWith(["さとう"]);
  });

  it("removes a chip with its button and with Backspace on an empty input", async () => {
    const onchange = vi.fn();
    render(ChipsField, { values: ["a", "b"], label: "ラベル", onchange });
    await fireEvent.click(screen.getByRole("button", { name: "b を外す" }));
    expect(onchange).toHaveBeenLastCalledWith(["a"]);
    await fireEvent.keyDown(screen.getByRole("textbox", { name: "ラベル" }), { key: "Backspace" });
    expect(onchange).toHaveBeenLastCalledWith(["a"]);
  });

  it("rejects text that transform refuses and shows the message and the hint", async () => {
    const onchange = vi.fn();
    render(ChipsField, {
      values: [],
      label: "リポジトリ",
      onchange,
      transform: (t: string) => (t.includes("/") ? t.toLowerCase() : null),
      invalidMessage: "読めません",
      hintFor: (t: string) => (t ? `Enter で ${t} として追加します` : null),
    });
    const input = screen.getByRole("textbox", { name: "リポジトリ" });
    await fireEvent.input(input, { target: { value: "nope" } });
    expect(screen.getByText("Enter で nope として追加します")).toBeInTheDocument();
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(screen.getByRole("alert")).toHaveTextContent("読めません");
    expect(onchange).not.toHaveBeenCalled();
    await fireEvent.input(input, { target: { value: "Org/Repo" } });
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(onchange).toHaveBeenCalledWith(["org/repo"]);
  });
});
