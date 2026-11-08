import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import FieldInput from "./FieldInput.svelte";

describe("FieldInput", () => {
  it("text: edits on every input and commits on blur", async () => {
    const onedit = vi.fn();
    const oncommit = vi.fn();
    render(FieldInput, { editor: "text", value: "old", label: "area", onedit, oncommit });
    const input = screen.getByRole("textbox", { name: "area" });
    expect(input).toHaveValue("old");
    await fireEvent.input(input, { target: { value: "new" } });
    expect(onedit).toHaveBeenLastCalledWith("new");
    expect(oncommit).not.toHaveBeenCalled();
    await fireEvent.blur(input);
    expect(oncommit).toHaveBeenCalledTimes(1);
  });

  it("text: an empty value is null for a property and kept for a title", async () => {
    const onedit = vi.fn();
    const { unmount } = render(FieldInput, {
      editor: "text",
      value: "x",
      label: "area",
      onedit,
      oncommit: vi.fn(),
    });
    await fireEvent.input(screen.getByRole("textbox", { name: "area" }), { target: { value: "" } });
    expect(onedit).toHaveBeenLastCalledWith(null);
    unmount();
    render(FieldInput, {
      editor: "text",
      value: "x",
      label: "タイトル",
      nullable: false,
      onedit,
      oncommit: vi.fn(),
    });
    await fireEvent.input(screen.getByRole("textbox", { name: "タイトル" }), {
      target: { value: "" },
    });
    expect(onedit).toHaveBeenLastCalledWith("");
  });

  it("text: takes the value back on blur when a refused edit is no longer pending", async () => {
    const { rerender } = render(FieldInput, {
      editor: "text",
      value: "A",
      label: "area",
      pending: false,
      onedit: vi.fn(),
      oncommit: vi.fn(),
    });
    const input = screen.getByRole("textbox", { name: "area" });
    await fireEvent.focus(input);
    await fireEvent.input(input, { target: { value: "B" } });
    await rerender({ value: "A", pending: true });
    // The server refused it and the edit was taken back; the user is still in the input.
    await rerender({ value: "A", pending: false });
    expect(input).toHaveValue("B");
    await fireEvent.blur(input);
    expect(input).toHaveValue("A");
  });

  it("does not overwrite what is typed while a save is pending, and takes the new value afterwards", async () => {
    const { rerender } = render(FieldInput, {
      editor: "text",
      value: "a",
      label: "area",
      pending: false,
      onedit: vi.fn(),
      oncommit: vi.fn(),
    });
    const input = screen.getByRole("textbox", { name: "area" });
    await fireEvent.input(input, { target: { value: "typed" } });
    await rerender({ value: "from outside", pending: true });
    expect(input).toHaveValue("typed");
    await rerender({ value: "from outside", pending: false });
    expect(input).toHaveValue("from outside");
  });

  it("number: sends a number, or null for an empty input", async () => {
    const onedit = vi.fn();
    render(FieldInput, {
      editor: "number",
      value: 3,
      label: "points",
      onedit,
      oncommit: vi.fn(),
    });
    const input = screen.getByRole("spinbutton", { name: "points" });
    await fireEvent.input(input, { target: { value: "8" } });
    expect(onedit).toHaveBeenLastCalledWith(8);
    await fireEvent.input(input, { target: { value: "" } });
    expect(onedit).toHaveBeenLastCalledWith(null);
  });

  it("date commits right after the edit", async () => {
    const calls: string[] = [];
    const onedit = vi.fn((v: unknown) => calls.push(`edit:${String(v)}`));
    const oncommit = vi.fn(() => calls.push("commit"));
    render(FieldInput, { editor: "date", value: "2026-11-07", label: "due", onedit, oncommit });
    await fireEvent.change(screen.getByLabelText("due"), { target: { value: "2026-11-09" } });
    expect(calls).toEqual(["edit:2026-11-09", "commit"]);
  });

  it("select: shows the current choice, opens a menu and commits the chosen one", async () => {
    const calls: string[] = [];
    const onedit = vi.fn((v: unknown) => calls.push(`edit:${String(v)}`));
    const oncommit = vi.fn(() => calls.push("commit"));
    render(FieldInput, {
      editor: "select",
      value: "todo",
      label: "ステータス",
      options: [
        { value: "todo", label: "未着手" },
        { value: "done", label: "完了" },
      ],
      nullable: false,
      onedit,
      oncommit,
    });
    const button = screen.getByRole("button", { name: "ステータス" });
    expect(button).toHaveTextContent("未着手");
    await fireEvent.click(button);
    expect(screen.getByRole("menu", { name: "ステータス" })).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("menuitem", { name: "完了" }));
    expect(calls).toEqual(["edit:done", "commit"]);
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("select: a nullable select has a choice that clears the value", async () => {
    const onedit = vi.fn();
    render(FieldInput, {
      editor: "select",
      value: "mid",
      label: "priority",
      options: [{ value: "mid", label: "mid" }],
      onedit,
      oncommit: vi.fn(),
    });
    await fireEvent.click(screen.getByRole("button", { name: "priority" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "—" }));
    expect(onedit).toHaveBeenLastCalledWith(null);
  });

  it("select: a disabled choice cannot be taken", async () => {
    const onedit = vi.fn();
    render(FieldInput, {
      editor: "select",
      value: "task",
      label: "種類",
      nullable: false,
      options: [
        { value: "task", label: "タスク" },
        { value: "project", label: "プロジェクト", disabled: true, title: "最上位にだけ置けます" },
      ],
      onedit,
      oncommit: vi.fn(),
    });
    await fireEvent.click(screen.getByRole("button", { name: "種類" }));
    expect(screen.getByRole("menuitem", { name: /^プロジェクト/ })).toBeDisabled();
    expect(screen.getByText("最上位にだけ置けます")).toBeInTheDocument();
  });

  it("readonly shows the value without editing", () => {
    const { unmount } = render(FieldInput, {
      editor: "text",
      value: "fixed",
      label: "area",
      readonly: true,
      onedit: vi.fn(),
      oncommit: vi.fn(),
    });
    expect(screen.getByRole("textbox", { name: "area" })).toHaveAttribute("readonly");
    unmount();
    render(FieldInput, {
      editor: "select",
      value: "a",
      label: "s",
      options: [{ value: "a", label: "A" }],
      readonly: true,
      onedit: vi.fn(),
      oncommit: vi.fn(),
    });
    expect(screen.getByRole("button", { name: "s" })).toBeDisabled();
  });
});
