import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { formatDate } from "../../core/format";
import { fail, makeCore, mount, ok } from "../detail/testkit";
import { resetToasts, toasts } from "../toast.svelte";
import { closeWorkspaceDialog, workspaceDialog } from "./dialogs-state.svelte";
import { entry, written } from "./testkit";
import WorkspaceSettings from "./WorkspaceSettings.svelte";

const release = entry(); // 24 tasks and 1 deleted
const shared = entry({ ulid: "W2", name: "shared", prefix: "SH", tasks: 9, deleted: 0 });

function setup(target = release) {
  const k = makeCore({ workspaces: [target, shared] });
  k.api.PATCH.mockImplementation(() => ok(written({ name: "releases" }, { commit: "c1" })));
  return { k, ...mount(WorkspaceSettings, { entry: target }, k) };
}

const nameField = () => screen.getByRole("textbox", { name: "名前" });
const saveButton = () => screen.getByRole("button", { name: "保存" });

describe("WorkspaceSettings", () => {
  beforeEach(() => resetToasts());
  afterEach(() => closeWorkspaceDialog());

  it("shows the name as saved: nothing to save", () => {
    setup();
    expect(nameField()).toHaveValue("release");
    expect(saveButton()).toBeDisabled();
    expect(screen.getByText("タスクの ID は変わりません")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /コマンドをコピー/ })[0]).toHaveTextContent(
      "hh workspace rename release <新しい名前> --store shared",
    );
  });

  it("renames in one write", async () => {
    const { k } = setup();
    await fireEvent.input(nameField(), { target: { value: "releases" } });
    expect(
      screen.getByText("名前だけが変わります。接頭辞 REL とタスクの ID はそのままです"),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: /コマンドをコピー/ })[0]).toHaveTextContent(
      "hh workspace rename release releases --store shared",
    );
    await fireEvent.click(saveButton());
    await waitFor(() => expect(k.api.PATCH).toHaveBeenCalled());
    expect(k.api.PATCH).toHaveBeenCalledWith("/api/workspaces/{id}", {
      params: { path: { id: "W1" }, query: { store: "shared" } },
      body: { name: "releases" },
    });
    expect(k.stores.reload).toHaveBeenCalled();
    await waitFor(() =>
      expect(toasts.list[0]?.message).toBe(
        "ワークスペースの名前を release から releases に変えました",
      ),
    );
  });

  it("only reads the settings of a read-only Store: no rename, no prefix change, no delete", async () => {
    const sentence = "この Store は閲覧のみです（remote に書き込めません）";
    const k = makeCore({
      workspaces: [shared],
      readOnly: "the remote cannot be written",
    });
    mount(WorkspaceSettings, { entry: shared }, k);
    expect(nameField()).toHaveAttribute("readonly");
    for (const b of [
      saveButton(),
      screen.getByRole("button", { name: "接頭辞を変更…" }),
      screen.getByRole("button", { name: "削除" }),
    ]) {
      expect(b).toBeDisabled();
      expect(b).toHaveAttribute("title", sentence);
    }
    await fireEvent.keyDown(nameField(), { key: "Enter" });
    expect(k.api.PATCH).not.toHaveBeenCalled();
    expect(workspaceDialog.current).toBeNull();
  });

  it("saves with Enter, but not while an IME composes", async () => {
    const { k } = setup();
    await fireEvent.input(nameField(), { target: { value: "releases" } });
    await fireEvent.keyDown(nameField(), { key: "Enter", isComposing: true });
    expect(k.api.PATCH).not.toHaveBeenCalled();
    await fireEvent.keyDown(nameField(), { key: "Enter" });
    await waitFor(() => expect(k.api.PATCH).toHaveBeenCalledTimes(1));
  });

  it("refuses a name another workspace of the store has, in red, and cannot save", async () => {
    const { k } = setup();
    await fireEvent.input(nameField(), { target: { value: "shared" } });
    const msg = screen.getByText("shared は同じ Store のワークスペースが使っています");
    expect(msg).toHaveClass("bad");
    expect(nameField()).toHaveAttribute("aria-invalid", "true");
    expect(saveButton()).toBeDisabled();
    await fireEvent.keyDown(nameField(), { key: "Enter" });
    expect(k.api.PATCH).not.toHaveBeenCalled();
    expect(screen.getAllByRole("button", { name: /コマンドをコピー/ })[0]).toBeDisabled();
  });

  it("refuses a name that is not one", async () => {
    setup();
    await fireEvent.input(nameField(), { target: { value: "Release" } });
    expect(screen.getByText(/名前は小文字の英数字と/)).toBeInTheDocument();
    expect(saveButton()).toBeDisabled();
  });

  it("shows the words of the server when it refuses", async () => {
    const { k } = setup();
    k.api.PATCH.mockImplementation(() =>
      fail(409, "workspace.taken", {}, "a workspace named x exists"),
    );
    await fireEvent.input(nameField(), { target: { value: "x" } });
    await fireEvent.click(saveButton());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "そのワークスペースの名前はすでに使われています",
    );
  });

  it("keeps what is typed when only the counts change, and follows a rename made elsewhere", async () => {
    const { rerender } = setup();
    await fireEvent.input(nameField(), { target: { value: "typing" } });
    await rerender({ entry: { ...release, tasks: 30 } });
    expect(nameField()).toHaveValue("typing");
    await rerender({ entry: { ...release, name: "renamed" } });
    await waitFor(() => expect(nameField()).toHaveValue("renamed"));
  });

  it("shows the prefix, when it was last changed, how many tasks a change renumbers, and opens the dialog", async () => {
    setup();
    expect(screen.getByText("REL", { selector: ".prefix" })).toBeInTheDocument();
    expect(screen.getByText(/REL-1, REL-2 … と振ります/)).toBeInTheDocument();
    expect(
      screen.getByText(`最後に変えたのは ${formatDate(release.prefixSet, new Date())}`, {
        exact: false,
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/配下の 25 件（削除済みを含む）の ID を新しい接頭辞で振り直します/),
    ).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("button", { name: "接頭辞を変更…" }));
    expect(workspaceDialog.current).toMatchObject({ kind: "prefix", entry: { ulid: "W1" } });
  });

  it("shows the ULID and copies it", async () => {
    setup(entry({ ulid: "01JC0A1B2C3D4E5F6G7H8J9K0M" }));
    expect(screen.getByText("01JC0A1B2C3D4E5F6G7H8J9K0M")).toBeInTheDocument();
    expect(screen.getByText(/CLI では名前・接頭辞・ULID のどれでも指せます/)).toBeInTheDocument();
    const writeText = vi.fn(async () => {});
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    await fireEvent.click(screen.getByRole("button", { name: "ULID をコピー" }));
    await waitFor(() => expect(toasts.list[0]?.message).toBe("ULID をコピーしました"));
    expect(writeText).toHaveBeenCalledWith("01JC0A1B2C3D4E5F6G7H8J9K0M");
  });

  it("cannot delete while tasks remain, the deleted ones included, and says so", () => {
    setup();
    expect(
      screen.getByText(
        "タスク 25 件（削除済み 1 件を含む）が残っているため削除できません。別のワークスペースへ移すか完全に削除すると消せます",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "削除" })).toBeDisabled();
  });

  it("opens the delete dialog for an empty workspace", async () => {
    setup(entry({ tasks: 0, deleted: 0 }));
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    expect(workspaceDialog.current).toMatchObject({ kind: "delete", entry: { ulid: "W1" } });
  });
});
