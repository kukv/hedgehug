import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeAll, beforeEach, describe, expect, it } from "vitest";
import { resetToasts, toasts } from "../toast.svelte";
import TaskDetail from "./TaskDetail.svelte";
import { fail, makeCore, mount, ok, showView, stubLayout, taskView } from "./testkit";

const U = "01JBZ8K3QW7M2X9V4T6R5N8P0A";
const base = showView({
  ulid: U,
  id: "REL-6",
  store: "shared",
  kind: "task",
  title: "Web の API クライアントを v2 に更新",
  former_ids: ["SH-17"],
  status: "in_progress",
  parent: "UP",
  body: "- [x] a\n- [ ] b\n",
  workspace: { ulid: "W1", name: "release", id_prefix: "REL" },
  watchers: ["sato"],
});
const parent = taskView({
  ulid: "UP",
  id: "REL-1",
  title: "API v2 への移行",
  kind: "project",
  parent: null,
});

function setup(over: Partial<typeof base> = {}, tasks = [parent]) {
  const show = { ...base, ...over };
  const k = makeCore({ show, account: "github.com/koki", tasks });
  return { k, ...mount(TaskDetail, { ulid: U }, k) };
}

describe("TaskDetail", () => {
  beforeAll(stubLayout);
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("shows the kind and ID, the former ID, the ULID and the title", () => {
    setup();
    expect(screen.getByText(/^タスク ·/)).toHaveTextContent("タスク · REL-6");
    expect(screen.getByText(/SH-17/)).toBeInTheDocument();
    expect(screen.getByText(U)).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "タイトル" })).toHaveValue(
      "Web の API クライアントを v2 に更新",
    );
  });

  it("shows the breadcrumb with the store, the workspace, the ancestors and the ID", async () => {
    setup();
    const nav = screen.getByRole("navigation", { name: "パンくず" });
    expect(nav).toHaveTextContent("shared");
    expect(nav).toHaveTextContent("release");
    await waitFor(() => expect(nav).toHaveTextContent("REL-1 API v2 への移行"));
    expect(nav).toHaveTextContent("REL-6");
  });

  it("edits the title in place through the SaveController", async () => {
    const { k } = setup();
    const title = screen.getByRole("textbox", { name: "タイトル" });
    await fireEvent.input(title, { target: { value: "新しい題" } });
    expect(k.save.edit).toHaveBeenCalledWith(U, "shared", "title", "新しい題");
    await fireEvent.blur(title);
    expect(k.save.flush).toHaveBeenCalledWith(U);
  });

  it("shows the save state, and a retry for a failure", async () => {
    const k = makeCore({ show: base, tasks: [parent] });
    k.save.state.mockReturnValue({ kind: "saved", pendingPush: true });
    const { unmount } = mount(TaskDetail, { ulid: U }, k);
    expect(screen.getByRole("status", { name: "保存の状態" })).toHaveTextContent(
      "保存済み · push 待ち",
    );
    unmount();
    k.save.state.mockReturnValue({ kind: "error", message: "保存できませんでした" });
    mount(TaskDetail, { ulid: U }, k);
    await fireEvent.click(screen.getByRole("button", { name: "やり直す" }));
    expect(k.save.flush).toHaveBeenCalledWith(U);
  });

  it("toggles the watch: PUT when not watching, DELETE when watching, and the button follows", async () => {
    const { k } = setup();
    k.api.PUT.mockImplementation(() => ok({ task: taskView({ ulid: U }), commit: "c1" }));
    k.api.DELETE.mockImplementation(() => ok({ task: taskView({ ulid: U }), commit: "c2" }));
    await fireEvent.click(screen.getByRole("button", { name: "ウォッチ" }));
    expect(k.api.PUT).toHaveBeenCalledWith("/api/tasks/{id}/watchers/me", {
      params: { path: { id: U }, query: { store: "shared" } },
    });
    await waitFor(() =>
      expect(k.cache.applyWrite).toHaveBeenCalledWith({ task: expect.anything(), commit: "c1" }),
    );
    await fireEvent.click(await screen.findByRole("button", { name: "ウォッチ中" }));
    expect(k.api.DELETE).toHaveBeenCalledWith("/api/tasks/{id}/watchers/me", {
      params: { path: { id: U }, query: { store: "shared" } },
    });
  });

  it("knows it is watching from the watchers, written as host/login", () => {
    setup({ watchers: ["sato", "github.com/koki"] });
    expect(screen.getByRole("button", { name: "ウォッチ中" })).toBeInTheDocument();
  });

  it("shows an error toast when the watch request fails", async () => {
    const { k } = setup();
    k.api.PUT.mockImplementation(() => fail(409, "store.read_only", { store: "shared" }));
    await fireEvent.click(screen.getByRole("button", { name: "ウォッチ" }));
    await waitFor(() => expect(toasts.list).toHaveLength(1));
  });

  it("warns that a blocker is not done yet, and does not when it is done", async () => {
    const blocked = {
      blocked_by: [{ ulid: "UB", id: "REL-2", title: "API の破壊的変更をまとめる" }],
    };
    const open = taskView({
      ulid: "UB",
      id: "REL-2",
      title: "API の破壊的変更をまとめる",
      status: "in_progress",
    });
    const { unmount } = setup(blocked, [parent, open]);
    expect(await screen.findByText(/にブロックされています/)).toHaveTextContent("REL-2");
    unmount();
    setup(blocked, [parent, { ...open, status: "done" }]);
    await waitFor(() => expect(screen.queryByText(/にブロックされています/)).toBeNull());
  });

  it("follows the blocker and the parent when the cache takes in a change of them", async () => {
    const blocked = {
      blocked_by: [{ ulid: "UB", id: "REL-2", title: "API の破壊的変更をまとめる" }],
    };
    const open = taskView({ ulid: "UB", id: "REL-2", title: "blocker", status: "in_progress" });
    const tasks = [parent, open];
    const k = makeCore({ show: { ...base, ...blocked }, account: "github.com/koki", tasks });
    k.schema.subscribe = () => () => false; // only the cache tells this screen about the other tasks
    mount(TaskDetail, { ulid: U }, k);
    expect(await screen.findByText(/にブロックされています/)).toHaveTextContent("REL-2");
    tasks[1] = { ...open, status: "done" };
    tasks[0] = { ...parent, title: "API v3 への移行" };
    k.notify();
    await waitFor(() => expect(screen.queryByText(/にブロックされています/)).toBeNull());
    await waitFor(() =>
      expect(screen.getByRole("navigation", { name: "パンくず" })).toHaveTextContent(
        "REL-1 API v3 への移行",
      ),
    );
  });

  it("toggles a checklist item by rewriting the body and flushing at once", async () => {
    const { k } = setup();
    await fireEvent.click(screen.getByRole("checkbox", { name: "b" }));
    expect(k.save.edit).toHaveBeenCalledWith(U, "shared", "body", "- [x] a\n- [x] b\n");
    expect(k.save.flush).toHaveBeenCalledWith(U);
  });

  it("shows a deleted task read-only with its banner", () => {
    setup({ deleted_at: "2026-11-03T10:00:00Z" });
    expect(screen.getByRole("status", { name: "削除済み" })).toHaveTextContent("削除済み");
    expect(screen.getByRole("textbox", { name: "タイトル" })).toHaveAttribute("readonly");
  });

  it("shows a broken file with the hh doctor command and no editors", () => {
    setup({ broken: "yaml: line 3: bad", problems: [] });
    expect(screen.getByText(/hh doctor で確かめてください/)).toBeInTheDocument();
    expect(screen.getByText("$ hh doctor")).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "タイトル" })).toBeNull();
  });

  it("shows the load error", async () => {
    const k = makeCore({ show: undefined, loadError: new Error("x") });
    mount(TaskDetail, { ulid: U }, k);
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });
});
