import { cleanup, fireEvent, render, screen, within } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { WorkspaceEntry } from "../../core/stores";
import WorkspacePicker from "./WorkspacePicker.svelte";

const ws = (store: string, ulid: string, name: string, prefix: string): WorkspaceEntry => ({
  store,
  tone: store === "shared" ? "purple" : "orange",
  ulid,
  name,
  prefix,
  tasks: 0,
  deleted: 0,
  prefixSet: "",
  projects: [],
});

const all = [
  ws("shared", "W1", "release", "REL"),
  ws("shared", "W2", "shared", "SH"),
  ws("backend", "W3", "payments", "PAY"),
];

function setup(selected: string[] = []) {
  const onchange = vi.fn();
  const view = render(WorkspacePicker, { workspaces: all, selected, onchange });
  return { onchange, view };
}

const open = () => fireEvent.click(screen.getByRole("button", { name: /ワークスペース$/ }));

afterEach(() => cleanup());

describe("WorkspacePicker", () => {
  it("shows how many are selected of all; none selected is all", () => {
    setup();
    expect(screen.getByRole("button", { name: "3 / 3 ワークスペース" })).toBeTruthy();
    cleanup();
    setup(["W1", "W3"]);
    expect(screen.getByRole("button", { name: "2 / 3 ワークスペース" })).toBeTruthy();
  });

  it("opens a dialog with the workspaces under a heading for each Store", async () => {
    setup(["W3"]);
    expect(screen.queryByRole("dialog")).toBeNull();
    await open();
    const dialog = screen.getByRole("dialog", { name: "ワークスペースで絞り込む" });
    expect(within(dialog).getAllByRole("checkbox")).toHaveLength(3);
    expect(within(dialog).getByRole("checkbox", { name: /payments/ })).toBeChecked();
    expect(within(dialog).getByRole("checkbox", { name: /release/ })).not.toBeChecked();
    expect(within(dialog).getByTitle("Store: shared")).toBeTruthy();
    expect(within(dialog).getByTitle("Store: backend")).toBeTruthy();
    expect(within(dialog).getByText("1 件を選択中 · 未選択のときはすべて")).toBeTruthy();
  });

  it("tells the ULIDs of the selection as it changes", async () => {
    const { onchange } = setup(["W1"]);
    await open();
    await fireEvent.click(screen.getByRole("checkbox", { name: /payments/ }));
    expect(onchange).toHaveBeenLastCalledWith(["W1", "W3"]);
    await fireEvent.click(screen.getByRole("checkbox", { name: /release/ }));
    expect(onchange).toHaveBeenLastCalledWith([]);
  });

  it("finds by a part of the name or the prefix, not minding the case", async () => {
    setup();
    await open();
    const find = screen.getByRole("textbox", { name: "ワークスペースを探す" });
    await fireEvent.input(find, { target: { value: "PAY" } });
    expect(screen.getAllByRole("checkbox")).toHaveLength(1);
    await fireEvent.input(find, { target: { value: "ELE" } });
    expect(screen.getByRole("checkbox", { name: /release/ })).toBeTruthy();
    await fireEvent.input(find, { target: { value: "nothing" } });
    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
    // a Store with no match has no heading
    expect(screen.queryByTitle("Store: shared")).toBeNull();
  });

  it("counts only the selected ULIDs that are workspaces", async () => {
    setup(["W1", "GONE"]);
    expect(screen.getByRole("button", { name: "1 / 3 ワークスペース" })).toBeTruthy();
    await open();
    expect(screen.getByText("1 件を選択中 · 未選択のときはすべて")).toBeTruthy();
  });

  it("closes when the focus leaves for something outside, not for something inside", async () => {
    setup();
    await open();
    const find = screen.getByRole("textbox", { name: "ワークスペースを探す" });
    await fireEvent.focusOut(find, {
      relatedTarget: screen.getByRole("checkbox", { name: /release/ }),
    });
    expect(screen.getByRole("dialog")).toBeTruthy();
    await fireEvent.focusOut(find, { relatedTarget: document.body });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("clears the selection", async () => {
    const { onchange } = setup(["W1", "W2"]);
    await open();
    await fireEvent.click(screen.getByRole("button", { name: "選択を解除" }));
    expect(onchange).toHaveBeenLastCalledWith([]);
  });

  it("closes with Esc and with a press outside, not with a press inside", async () => {
    setup();
    await open();
    await fireEvent.pointerDown(screen.getByRole("textbox", { name: "ワークスペースを探す" }));
    expect(screen.getByRole("dialog")).toBeTruthy();
    await fireEvent.keyDown(screen.getByRole("textbox", { name: "ワークスペースを探す" }), {
      key: "Escape",
    });
    expect(screen.queryByRole("dialog")).toBeNull();
    await open();
    await fireEvent.pointerDown(document.body);
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
