import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fail, makeCore, mount, ok } from "../detail/testkit";
import { resetToasts, toasts } from "../toast.svelte";
import PrefixChange from "./PrefixChange.svelte";
import { entry, written } from "./testkit";

const release = entry(); // 24 tasks and 1 deleted: 25 are renumbered
const others = [
  release,
  entry({ ulid: "W2", name: "shared", prefix: "SH" }),
  entry({ store: "backend", ulid: "B1", name: "api", prefix: "API" }),
];

function setup(target = release) {
  const k = makeCore({ workspaces: [target, ...others.slice(1)] });
  k.api.PATCH.mockImplementation(() =>
    ok(
      written(
        { id_prefix: "RLS" },
        {
          commit: "c2",
          ids: [
            { old: "REL-1", new: "RLS-1" },
            { old: "REL-2", new: "RLS-2" },
          ],
        },
      ),
    ),
  );
  const onclose = vi.fn();
  return { k, onclose, ...mount(PrefixChange, { entry: target, onclose }, k) };
}

const prefixField = () => screen.getByRole("textbox", { name: "新しい接頭辞" });

describe("PrefixChange", () => {
  beforeEach(() => resetToasts());

  it("says how many tasks are renumbered, the deleted ones included", () => {
    setup();
    expect(screen.getByRole("dialog", { name: "release の接頭辞を変更" })).toBeVisible();
    expect(
      screen.getByText(/配下の 25 件（削除済みを含む）の ID を新しい接頭辞で振り直します/),
    ).toBeInTheDocument();
    expect(screen.getByText(/古い ID は former_ids に残る/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "25 件を振り直す" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh workspace set-prefix release <接頭辞> --store shared",
    );
  });

  it("changes the prefix in one write and says how many ids changed", async () => {
    const { k, onclose } = setup();
    await fireEvent.input(prefixField(), { target: { value: "rls" } });
    expect(prefixField()).toHaveValue("RLS");
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh workspace set-prefix release RLS --store shared",
    );
    await fireEvent.click(screen.getByRole("button", { name: "25 件を振り直す" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(k.api.PATCH).toHaveBeenCalledWith("/api/workspaces/{id}", {
      params: { path: { id: "W1" }, query: { store: "shared" } },
      body: { prefix: "RLS" },
    });
    // the tasks.changed of this commit is ours: the rows must not read "CLI で更新"
    expect(k.cache.noteOwnCommit).toHaveBeenCalledWith("c2");
    expect(k.stores.reload).toHaveBeenCalled();
    expect(toasts.list[0]?.message).toBe(
      "release の接頭辞を REL から RLS に変えました（2 件の ID を振り直しました）",
    );
  });

  it("refuses the current prefix and the prefixes that are taken", async () => {
    const { k } = setup();
    const go = screen.getByRole("button", { name: "25 件を振り直す" });
    await fireEvent.input(prefixField(), { target: { value: "REL" } });
    expect(screen.getByText("今と同じ接頭辞です")).toBeInTheDocument();
    await fireEvent.input(prefixField(), { target: { value: "SH" } });
    expect(
      screen.getByText("SH は同じ Store のワークスペース shared が使っています"),
    ).toBeInTheDocument();
    await fireEvent.input(prefixField(), { target: { value: "API" } });
    expect(screen.getByText(/API はマウント中の backend のワークスペース api/)).toBeInTheDocument();
    await fireEvent.input(prefixField(), { target: { value: "r" } });
    expect(screen.getByText(/2〜10 文字/, { selector: ".bad" })).toBeInTheDocument();
    expect(go).toBeDisabled();
    await fireEvent.keyDown(prefixField(), { key: "Enter" });
    expect(k.api.PATCH).not.toHaveBeenCalled();
  });

  it("has a plain button when the workspace has no task", () => {
    setup(entry({ tasks: 0, deleted: 0 }));
    expect(
      screen.getByText("配下にタスクはありません。接頭辞だけが変わります。"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "変更" })).toBeDisabled();
  });

  it("keeps the dialog open and shows the words of the server when it refuses", async () => {
    const { k, onclose } = setup();
    k.api.PATCH.mockImplementation(() => fail(409, "workspace.prefix_taken", { prefix: "OPS" }));
    await fireEvent.input(prefixField(), { target: { value: "OPS" } });
    await fireEvent.keyDown(prefixField(), { key: "Enter" });
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "ID の接頭辞 OPS はすでに使われています",
    );
    expect(onclose).not.toHaveBeenCalled();
    expect(k.cache.noteOwnCommit).not.toHaveBeenCalled();
  });

  it("counts the tasks as they are now, not as they were when the dialog opened", async () => {
    const { k } = setup();
    const now = [{ ...release, tasks: 30, deleted: 2 }, ...others.slice(1)];
    k.stores.workspaces.mockReturnValue(now);
    k.notify();
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "32 件を振り直す" })).toBeInTheDocument(),
    );
  });
});
