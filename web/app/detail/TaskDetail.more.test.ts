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

  it("opens the move-to-another-Store dialog for this task when two Stores are mounted", async () => {
    const k = makeCore({ show, storeItems: [{ name: "shared" }, { name: "second" }] });
    mount(TaskDetail, { ulid: U }, k);
    await fireEvent.click(screen.getByRole("button", { name: "その他の操作" }));
    await fireEvent.click(await screen.findByRole("menuitem", { name: "別の Store へ移動…" }));
    expect(workspaceDialog.current).toMatchObject({
      kind: "move-store",
      task: { ulid: U, id: "REL-6" },
    });
  });

  it("does not offer the move to another Store with one Store, or in a read-only Store", async () => {
    const one = makeCore({ show });
    const { unmount } = mount(TaskDetail, { ulid: U }, one);
    await fireEvent.click(screen.getByRole("button", { name: "その他の操作" }));
    await screen.findByRole("menuitem", { name: /別のワークスペースへ移動/ });
    expect(screen.queryByRole("menuitem", { name: /別の Store へ移動/ })).toBeNull();
    unmount();

    const ro = makeCore({
      show,
      storeItems: [{ name: "shared" }, { name: "second" }],
      readOnly: "the remote cannot be written",
    });
    mount(TaskDetail, { ulid: U }, ro);
    await fireEvent.click(screen.getByRole("button", { name: "その他の操作" }));
    await screen.findByRole("menuitem", { name: /別のワークスペースへ移動/ });
    expect(screen.queryByRole("menuitem", { name: /別の Store へ移動/ })).toBeNull();
  });

  it("has no menu on a deleted task", () => {
    const k = makeCore({ show: { ...show, deleted_at: "2026-11-03T10:00:00Z" } });
    mount(TaskDetail, { ulid: U }, k);
    expect(screen.queryByRole("button", { name: "その他の操作" })).toBeNull();
  });
});
