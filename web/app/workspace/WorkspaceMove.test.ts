import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fail, makeCore, mount, ok, taskView } from "../detail/testkit";
import { resetToasts, toasts } from "../toast.svelte";
import { entry } from "./testkit";
import WorkspaceMove from "./WorkspaceMove.svelte";

const task = taskView({
  ulid: "U6",
  id: "REL-6",
  title: "Web の API クライアントを v2 に更新",
  store: "shared",
});
const moved = taskView({
  ulid: "U6",
  id: "SH-12",
  title: task.title,
  workspace: { ulid: "W2", name: "shared", id_prefix: "SH" },
});

const spaces = [
  entry(),
  entry({ ulid: "W2", name: "shared", prefix: "SH" }),
  entry({ ulid: "W3", name: "ops", prefix: "OPS" }),
  entry({ store: "backend", ulid: "B1", name: "api", prefix: "API" }),
];

function setup(list = spaces) {
  const k = makeCore({ workspaces: list });
  k.api.POST.mockImplementation(() => ok({ task: moved, commit: "c3", detached: null }));
  const onclose = vi.fn();
  return { k, onclose, ...mount(WorkspaceMove, { task, onclose }, k) };
}

describe("WorkspaceMove", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("lists the workspaces of the store of the task, and the one it is in is not a choice", () => {
    setup();
    expect(
      screen.getByRole("dialog", { name: "REL-6 Web の API クライアントを v2 に更新 を移動" }),
    ).toBeVisible();
    const group = screen.getByRole("radiogroup", { name: "移り先のワークスペース" });
    expect(group).toHaveTextContent("release（REL） — 今のワークスペース");
    expect(screen.getByRole("radio", { name: /^release/ })).toBeDisabled();
    expect(screen.getByRole("radio", { name: "shared（SH）" })).toBeChecked();
    expect(screen.getByRole("radio", { name: "ops（OPS）" })).not.toBeChecked();
    expect(screen.queryByRole("radio", { name: /api/ })).toBeNull(); // another store
  });

  it("explains the move, with the prefix of the workspace picked", async () => {
    setup();
    expect(
      screen.getByText("移り先のルートに置きます。配下のタスクも一緒に移ります"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("ID を SH の番号に振り直します（REL-6 は former_ids に残ります）"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("スプリント・リリースが移り先のものでなければ外れます"),
    ).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("radio", { name: "ops（OPS）" }));
    expect(
      screen.getByText("ID を OPS の番号に振り直します（REL-6 は former_ids に残ります）"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh mv REL-6 --workspace ops",
    );
  });

  it("moves the task and opens the way to it in the toast", async () => {
    const { k, onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "移動" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(k.api.POST).toHaveBeenCalledWith("/api/tasks/{id}/move", {
      params: { path: { id: "U6" }, query: { store: "shared" } },
      body: { workspace: "W2" },
    });
    expect(k.cache.applyWrite).toHaveBeenCalledWith({ task: moved, commit: "c3" });
    expect(toasts.list[0]).toMatchObject({
      message: "REL-6 を shared に移しました（今の ID は SH-12）",
      link: { label: "開く", href: "/tasks/U6" },
    });
  });

  it("names what the move took off the task", async () => {
    const { k, onclose } = setup();
    k.api.POST.mockImplementation(() =>
      ok({
        task: moved,
        commit: "c3",
        detached: [{ task: "U6", id: "SH-12", sprint: "Sprint 44", releases: ["v2.4"] }],
      }),
    );
    await fireEvent.click(screen.getByRole("button", { name: "移動" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(toasts.list[0]?.message).toBe(
      "REL-6 を shared に移しました（今の ID は SH-12）。SH-12 はスプリント Sprint 44 とリリース v2.4 から外れました",
    );
  });

  it("has nowhere to go when the store has no other workspace", () => {
    setup([entry(), entry({ store: "backend", ulid: "B1", name: "api", prefix: "API" })]);
    expect(
      screen.getByText("この Store（shared）にはほかのワークスペースがありません。"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("radiogroup")).toBeNull();
    expect(screen.getByRole("button", { name: "移動" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toBeDisabled();
  });

  it("keeps the dialog open and shows the words of the server when it refuses", async () => {
    const { k, onclose } = setup();
    k.api.POST.mockImplementation(() => fail(400, "move.invalid", {}, "task is deleted"));
    await fireEvent.click(screen.getByRole("button", { name: "移動" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "ここには移せません: task is deleted",
    );
    expect(onclose).not.toHaveBeenCalled();
    expect(k.cache.applyWrite).not.toHaveBeenCalled();
  });
});
