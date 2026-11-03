import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Lists } from "../../core/scoped";
import ConstraintEditor from "./ConstraintEditor.svelte";

const statuses = [
  { name: "todo", label: "未着手", category: "todo", default: true },
  { name: "in_progress", label: "進行中", category: "doing" },
  { name: "done", label: "完了", category: "done", default: true },
  { name: "canceled", label: "中止", category: "canceled", default: true },
];
const fields = ["area", "points", "assignees", "blocked_by"];

function mount(over: Partial<Parameters<typeof render<typeof ConstraintEditor>>[1]> = {}) {
  const onchange = vi.fn();
  const props = {
    mode: "transition" as const,
    statuses,
    fields,
    mine: {} as Lists,
    fromStore: {} as Lists,
    busy: false,
    onchange,
    ...over,
  };
  const r = render(ConstraintEditor, { props });
  return { ...r, onchange };
}

afterEach(cleanup);

describe("ConstraintEditor: transitions", () => {
  it("tells the Store's rows from the project's, and gives a way to take off only the project's", () => {
    mount({ fromStore: { canceled: ["todo"] }, mine: { todo: ["done", "in_progress"] } });
    const rows = within(screen.getByRole("list", { name: "遷移の制約" })).getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    const store = rows.find((r) => within(r).queryByText("Store"));
    expect(store?.textContent).toMatch(/中止/);
    expect(within(store as HTMLElement).queryByRole("button")).toBeNull();
    const mine = rows.find((r) => within(r).queryByText("このプロジェクトで足した"));
    expect(within(mine as HTMLElement).getByRole("button", { name: "完了を外す" })).toBeTruthy();
  });

  it("adds the targets in the order of the schema, for a status, and clears the form", async () => {
    const { onchange } = mount({ mine: { todo: ["done"] } });
    const form = screen.getByRole("group", { name: "遷移の制約を足す" });
    const add = within(form).getByRole("button", { name: "遷移の制約を足す" });
    expect(add).toBeDisabled();
    await fireEvent.change(within(form).getByRole("combobox", { name: "遷移元のステータス" }), {
      target: { value: "todo" },
    });
    // the status it starts from is not one of its targets
    expect(within(form).queryByRole("checkbox", { name: "未着手" })).toBeNull();
    await fireEvent.click(within(form).getByRole("checkbox", { name: "中止" }));
    await fireEvent.click(within(form).getByRole("checkbox", { name: "進行中" }));
    await fireEvent.click(add);
    expect(onchange).toHaveBeenCalledWith({ todo: ["done", "in_progress", "canceled"] });
    expect(within(form).getByRole("combobox", { name: "遷移元のステータス" })).toHaveFocus();
    expect(add).toBeDisabled();
  });

  it("tells what is taken off, and moves the focus to the list that is left", async () => {
    const { onchange } = mount({ mine: { todo: ["done", "in_progress"] } });
    await fireEvent.click(screen.getByRole("button", { name: "完了を外す" }));
    expect(onchange).toHaveBeenCalledWith({ todo: ["in_progress"] });
    await waitFor(() => expect(screen.getByRole("list", { name: "遷移の制約" })).toHaveFocus());
  });

  it("moves the focus to the form when the last row is taken off", async () => {
    const { rerender, onchange } = mount({ mine: { todo: ["done"] } });
    onchange.mockImplementation(() => rerender({ mine: {} }));
    await fireEvent.click(screen.getByRole("button", { name: "完了を外す" }));
    await waitFor(() =>
      expect(screen.getByRole("combobox", { name: "遷移元のステータス" })).toHaveFocus(),
    );
  });

  it("leaves every input alone while busy", () => {
    mount({ mine: { todo: ["done"] }, busy: true });
    expect(screen.getByRole("button", { name: "完了を外す" })).toBeDisabled();
    expect(screen.getByRole("combobox", { name: "遷移元のステータス" })).toBeDisabled();
    for (const box of screen.getAllByRole("checkbox")) expect(box).toBeDisabled();
  });
});

describe("ConstraintEditor: required fields", () => {
  it("offers the properties and the built-in fields it is given, named by the field", () => {
    mount({ mode: "required" });
    const form = screen.getByRole("group", { name: "必須の項目を足す" });
    expect(
      within(form)
        .getAllByRole("checkbox")
        .map((b) => b.closest("label")?.textContent?.trim()),
    ).toEqual(fields);
  });

  it("adds fields to a status, on top of the ones it has", async () => {
    const { onchange } = mount({ mode: "required", mine: { done: ["area"] } });
    const form = screen.getByRole("group", { name: "必須の項目を足す" });
    await fireEvent.change(within(form).getByRole("combobox", { name: "必須にするステータス" }), {
      target: { value: "done" },
    });
    await fireEvent.click(within(form).getByRole("checkbox", { name: "points" }));
    await fireEvent.click(within(form).getByRole("button", { name: "必須の項目を足す" }));
    expect(onchange).toHaveBeenCalledWith({ done: ["area", "points"] });
  });

  it("keeps a field ticked when the status chosen has the same name", async () => {
    const { onchange } = mount({ mode: "required", fields: ["todo", "points"] });
    const form = screen.getByRole("group", { name: "必須の項目を足す" });
    await fireEvent.click(within(form).getByRole("checkbox", { name: "todo" }));
    await fireEvent.change(within(form).getByRole("combobox", { name: "必須にするステータス" }), {
      target: { value: "todo" },
    });
    expect(within(form).getByRole("checkbox", { name: "todo" })).toBeChecked();
    await fireEvent.click(within(form).getByRole("button", { name: "必須の項目を足す" }));
    expect(onchange).toHaveBeenCalledWith({ todo: ["todo"] });
  });

  it("shows the Store's fields without a way to take them off", () => {
    mount({ mode: "required", fromStore: { done: ["points"] } });
    const row = within(screen.getByRole("list", { name: "必須の項目" })).getByRole("listitem");
    expect(row.textContent).toMatch(/points/);
    expect(within(row).queryByRole("button")).toBeNull();
  });

  it("says there are none when there are none", () => {
    mount({ mode: "required" });
    expect(screen.getByText("まだありません")).toBeTruthy();
    expect(screen.queryByRole("list", { name: "必須の項目" })).toBeNull();
  });
});
