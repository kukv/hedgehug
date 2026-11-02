import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { flushSync } from "svelte";
import { afterEach, describe, expect, it } from "vitest";
import { makeCore, mount, taskView } from "../detail/testkit";
import {
  closeWorkspaceDialog,
  openPrefixChange,
  openWorkspaceAdd,
  openWorkspaceDelete,
  openWorkspaceMove,
  workspaceDialog,
} from "./dialogs-state.svelte";
import { entry } from "./testkit";
import WorkspaceDialogs from "./WorkspaceDialogs.svelte";

function setup() {
  const k = makeCore({
    workspaces: [entry(), entry({ ulid: "W2", name: "shared", prefix: "SH" })],
  });
  return mount(WorkspaceDialogs, {}, k);
}

describe("WorkspaceDialogs", () => {
  afterEach(() => closeWorkspaceDialog());

  it("draws nothing until a dialog is asked for", () => {
    setup();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(workspaceDialog.current).toBeNull();
  });

  it("draws the dialog that is asked for", () => {
    setup();
    openWorkspaceAdd("shared");
    flushSync();
    expect(screen.getByRole("dialog", { name: "shared にワークスペースを追加" })).toBeVisible();
    openPrefixChange(entry());
    flushSync();
    expect(screen.getByRole("dialog", { name: "release の接頭辞を変更" })).toBeVisible();
    openWorkspaceDelete(entry({ tasks: 0, deleted: 0 }));
    flushSync();
    expect(screen.getByRole("dialog", { name: "ワークスペース release を削除" })).toBeVisible();
    openWorkspaceMove(taskView({ id: "REL-6", title: "t" }));
    flushSync();
    expect(screen.getByRole("dialog", { name: "REL-6 t を移動" })).toBeVisible();
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
  });

  it("opens no delete confirmation for a workspace with tasks: its settings tab says why", () => {
    history.replaceState(null, "", "/");
    setup();
    openWorkspaceDelete(entry()); // 24 tasks and 1 deleted
    flushSync();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(workspaceDialog.current).toBeNull();
    expect(location.pathname).toBe("/workspaces/shared/W1/settings");
  });

  it("closes with Esc and with the close button", async () => {
    setup();
    openWorkspaceAdd();
    flushSync();
    await fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    openWorkspaceAdd();
    flushSync();
    await fireEvent.click(screen.getByRole("button", { name: "閉じる" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("does not close when the dim background is clicked", async () => {
    setup();
    openWorkspaceAdd();
    flushSync();
    const overlay = document.querySelector(".overlay") as HTMLElement;
    await fireEvent.click(overlay);
    expect(screen.getByRole("dialog")).toBeVisible();
  });
});
