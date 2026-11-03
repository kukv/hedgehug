import { fireEvent, screen } from "@testing-library/svelte";
import { afterEach, beforeAll, describe, expect, it } from "vitest";
import { closeWorkspaceDialog, workspaceDialog } from "../workspace/dialogs-state.svelte";
import TaskDetail from "./TaskDetail.svelte";
import { makeCore, mount, showView, stubLayout, workspaces } from "./testkit";

const U = "01JBZ8K3QW7M2X9V4T6R5N8P0A";
const show = showView({
  ulid: U,
  id: "REL-6",
  title: "Web の API クライアントを v2 に更新",
  workspace: { ulid: "W1", name: "release", id_prefix: "REL" },
});

describe("TaskDetail: the … menu", () => {
  beforeAll(stubLayout);
  afterEach(() => closeWorkspaceDialog());

  it("opens the move dialog for this task", async () => {
    const k = makeCore({ show });
    mount(TaskDetail, { ulid: U }, k);
    await fireEvent.click(screen.getByRole("button", { name: "その他の操作" }));
    await fireEvent.click(
      await screen.findByRole("menuitem", { name: /別のワークスペースへ移動/ }),
    );
    expect(screen.queryByRole("menu")).toBeNull();
    expect(workspaceDialog.current).toMatchObject({ kind: "move", task: { ulid: U, id: "REL-6" } });
  });

  it("turns the item off, and says why, when the store has no other workspace", async () => {
    const k = makeCore({ show, workspaces: [workspaces[0] as (typeof workspaces)[number]] });
    mount(TaskDetail, { ulid: U }, k);
    await fireEvent.click(screen.getByRole("button", { name: "その他の操作" }));
    const item = await screen.findByRole("menuitem", { name: /別のワークスペースへ移動/ });
    expect(item).toBeDisabled();
    expect(item).toHaveTextContent("この Store にはほかのワークスペースがありません");
  });

  it("has no menu on a deleted task", () => {
    const k = makeCore({ show: { ...show, deleted_at: "2026-11-03T10:00:00Z" } });
    mount(TaskDetail, { ulid: U }, k);
    expect(screen.queryByRole("button", { name: "その他の操作" })).toBeNull();
  });
});
