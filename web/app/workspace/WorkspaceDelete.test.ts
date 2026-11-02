import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fail, makeCore, mount, ok } from "../detail/testkit";
import { resetToasts, toasts } from "../toast.svelte";
import { runUndo } from "../undo";
import { entry, written } from "./testkit";
import WorkspaceDelete from "./WorkspaceDelete.svelte";

vi.mock("../undo", () => ({ runUndo: vi.fn(async () => {}) }));

const sandbox = entry({ ulid: "W3", name: "sandbox", prefix: "SBX", tasks: 0, deleted: 0 });

function setup(target = sandbox) {
  const k = makeCore({ workspaces: [entry(), target] });
  k.api.DELETE.mockImplementation(() =>
    ok(written({ ulid: "W3", name: "sandbox", id_prefix: "SBX" })),
  );
  const onclose = vi.fn();
  return { k, onclose, ...mount(WorkspaceDelete, { entry: target, onclose }, k) };
}

describe("WorkspaceDelete", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("removes an empty workspace", async () => {
    const { k, onclose } = setup();
    expect(screen.getByRole("dialog", { name: "ワークスペース sandbox を削除" })).toBeVisible();
    expect(screen.getByText(/にはタスクがありません/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh workspace rm sandbox --store shared",
    );
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(k.api.DELETE).toHaveBeenCalledWith("/api/workspaces/{id}", {
      params: { path: { id: "W3" }, query: { store: "shared" } },
    });
    expect(k.stores.reload).toHaveBeenCalled();
    expect(toasts.list[0]?.message).toBe("ワークスペース sandbox を削除しました");
  });

  it("puts an undo on the toast, which takes the removal back in the store of the workspace", async () => {
    const { onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    const undo = toasts.list[0]?.undo;
    expect(undo).toBeTypeOf("function");
    await undo?.();
    expect(runUndo).toHaveBeenCalledTimes(1);
    expect(runUndo).toHaveBeenCalledWith(expect.anything(), "shared");
  });

  it("leaves the screen of the workspace that is removed", async () => {
    history.replaceState(null, "", "/workspaces/shared/W3/settings");
    const { onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(location.pathname).toBe("/");
  });

  it("stays on the screen of another workspace", async () => {
    history.replaceState(null, "", "/workspaces/shared/W1/settings");
    const { onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(location.pathname).toBe("/workspaces/shared/W1/settings");
  });

  it("says why it cannot be removed while tasks remain, the deleted ones included", async () => {
    const { k } = setup(
      entry({ ulid: "W3", name: "sandbox", prefix: "SBX", tasks: 24, deleted: 1 }),
    );
    expect(
      screen.getByText(
        "タスク 25 件（削除済み 1 件を含む）が残っているため削除できません。別のワークスペースへ移すか完全に削除すると消せます",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "削除" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toBeDisabled();
    expect(k.api.DELETE).not.toHaveBeenCalled();
  });

  it("is blocked by a task that arrives while the dialog is open", async () => {
    const { k } = setup();
    k.stores.workspaces.mockReturnValue([entry(), { ...sandbox, tasks: 1 }]);
    k.notify();
    await waitFor(() => expect(screen.getByRole("button", { name: "削除" })).toBeDisabled());
    expect(screen.getByText(/タスク 1 件が残っているため削除できません/)).toBeInTheDocument();
  });

  it("keeps the dialog open and shows the words of the server when it refuses (409 as a safety net)", async () => {
    const { k, onclose } = setup();
    k.api.DELETE.mockImplementation(() => fail(409, "workspace.not_empty", { tasks: 2 }));
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "ワークスペースにタスクが 2 件（削除済みを含む）残っています",
    );
    expect(onclose).not.toHaveBeenCalled();
    expect(toasts.list).toHaveLength(0);
  });
});
