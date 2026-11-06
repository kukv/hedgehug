import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fail, makeCore, mount, ok } from "../detail/testkit";
import { resetToasts, toasts } from "../toast.svelte";
import { entry, storeItem, written } from "./testkit";
import WorkspaceAdd from "./WorkspaceAdd.svelte";

const spaces = [
  entry(),
  entry({ ulid: "W2", name: "shared", prefix: "SH" }),
  entry({ store: "backend", ulid: "B1", name: "api", prefix: "API" }),
];

function setup(opts: { two?: boolean; store?: string } = {}) {
  const k = makeCore({ workspaces: spaces });
  k.stores.stores.mockReturnValue(
    (opts.two
      ? [storeItem("shared", true), storeItem("backend")]
      : [storeItem("shared", true)]) as never,
  );
  k.api.addWorkspace.mockImplementation(() =>
    ok(written({ ulid: "W9", name: "ops", id_prefix: "OPS" }, { commit: "c1" })),
  );
  const onclose = vi.fn();
  return { k, onclose, ...mount(WorkspaceAdd, { store: opts.store, onclose }, k) };
}

async function fillIn(name: string, prefix: string) {
  await fireEvent.input(screen.getByRole("textbox", { name: "名前" }), { target: { value: name } });
  await fireEvent.input(screen.getByRole("textbox", { name: "ID の接頭辞" }), {
    target: { value: prefix },
  });
}

describe("WorkspaceAdd", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("adds the workspace to the store and says where to open it", async () => {
    const { k, onclose } = setup();
    expect(screen.getByRole("dialog", { name: "shared にワークスペースを追加" })).toBeVisible();
    expect(screen.queryByRole("radiogroup", { name: "Store" })).toBeNull();
    await fillIn("ops", "ops"); // the prefix is typed in capitals
    expect(screen.getByRole("textbox", { name: "ID の接頭辞" })).toHaveValue("OPS");
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh workspace add ops --prefix OPS --store shared",
    );
    await fireEvent.click(screen.getByRole("button", { name: "追加" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(k.api.addWorkspace).toHaveBeenCalledWith(
      { name: "ops", prefix: "OPS" },
      { store: "shared" },
    );
    expect(k.stores.reload).toHaveBeenCalled();
    expect(toasts.list[0]).toMatchObject({
      message: "ワークスペース ops（OPS）を shared に追加しました",
      link: { label: "開く", href: "/workspaces/shared/W9" },
    });
  });

  it("cannot add until both fields are good, and says why next to the field", async () => {
    const { k } = setup();
    const add = screen.getByRole("button", { name: "追加" });
    expect(add).toBeDisabled();
    // nothing is scolded before the first try
    expect(screen.queryByText("名前を入れてください")).toBeNull();

    await fillIn("Ops", "SH");
    expect(screen.getByText(/名前は小文字の英数字と/)).toBeInTheDocument();
    expect(
      screen.getByText("SH は同じ Store のワークスペース shared が使っています"),
    ).toBeInTheDocument();
    await fillIn("ops", "API");
    expect(
      screen.getByText(
        "API はマウント中の backend のワークスペース api が使っています。別の接頭辞にしてください",
      ),
    ).toBeInTheDocument();
    expect(add).toBeDisabled();
    expect(k.api.addWorkspace).not.toHaveBeenCalled();
  });

  it("shows the command with <名前> and cannot copy it until the fields are good", () => {
    setup();
    const cmd = screen.getByRole("button", { name: /コマンドをコピー/ });
    expect(cmd).toHaveTextContent("hh workspace add <名前> --prefix <接頭辞> --store shared");
    expect(cmd).toBeDisabled();
  });

  it("asks for the Store only when there are two, and starts with the one asked for", async () => {
    const { k } = setup({ two: true, store: "backend" });
    expect(screen.getByRole("dialog", { name: "backend にワークスペースを追加" })).toBeVisible();
    expect(screen.getByRole("radio", { name: "backend" })).toBeChecked();
    // the prefix of api is taken in backend, but not in shared
    await fillIn("ops", "API");
    expect(screen.getByRole("button", { name: "追加" })).toBeDisabled();
    await fireEvent.click(screen.getByRole("radio", { name: "shared" }));
    expect(screen.getByRole("dialog", { name: "shared にワークスペースを追加" })).toBeVisible();
    expect(
      screen.getByText(
        "API はマウント中の backend のワークスペース api が使っています。別の接頭辞にしてください",
      ),
    ).toBeInTheDocument();
    await fillIn("ops", "OPS");
    await fireEvent.click(screen.getByRole("button", { name: "追加" }));
    await waitFor(() => expect(k.api.addWorkspace).toHaveBeenCalled());
    expect(k.api.addWorkspace.mock.calls[0]?.[1]).toMatchObject({ store: "shared" });
  });

  it("does not offer a hidden Store", () => {
    const k = makeCore({ workspaces: spaces, hidden: ["backend"] });
    k.stores.stores.mockReturnValue([
      storeItem("shared", true),
      { ...storeItem("backend"), hidden: true },
    ] as never);
    mount(WorkspaceAdd, { store: "backend", onclose: vi.fn() }, k);
    // only one Store is left to choose from: the dialog names it and asks for no choice
    expect(screen.queryByRole("radio", { name: /backend/ })).toBeNull();
    expect(screen.getByRole("dialog", { name: "shared にワークスペースを追加" })).toBeVisible();
  });

  it("shows a read-only Store but does not let it be chosen, and starts from one that can be written", async () => {
    const k = makeCore({
      workspaces: spaces,
      readOnlyBy: { backend: "the remote cannot be written" },
    });
    k.stores.stores.mockReturnValue([storeItem("shared", true), storeItem("backend")] as never);
    k.api.addWorkspace.mockImplementation(() =>
      ok(written({ ulid: "W9", name: "ops", id_prefix: "OPS" })),
    );
    mount(WorkspaceAdd, { store: "backend", onclose: vi.fn() }, k);
    const ro = screen.getByRole("radio", { name: /backend/ });
    expect(ro).toBeDisabled();
    expect(ro.closest("label")).toHaveTextContent("閲覧のみ");
    expect(ro.closest("label")).toHaveAttribute("title", "remote に書き込めません");
    expect(ro).toHaveAccessibleDescription("閲覧のみ（remote に書き込めません）");
    expect(screen.getByRole("radio", { name: "shared" })).toBeChecked();
    expect(screen.getByRole("dialog", { name: "shared にワークスペースを追加" })).toBeVisible();
  });

  it("cannot add when every Store is read-only, and says so", async () => {
    const k = makeCore({ workspaces: spaces, readOnly: "the remote cannot be written" });
    k.stores.stores.mockReturnValue([storeItem("shared", true)] as never);
    mount(WorkspaceAdd, { onclose: vi.fn() }, k);
    await fillIn("ops", "OPS");
    expect(screen.getByText("書き込める Store がありません（すべて閲覧のみです）")).toBeVisible();
    expect(screen.getByRole("button", { name: "追加" })).toBeDisabled();
    await fireEvent.keyDown(screen.getByRole("textbox", { name: "名前" }), { key: "Enter" });
    expect(k.api.addWorkspace).not.toHaveBeenCalled();
  });

  it("starts with the default Store when none is asked for", () => {
    setup({ two: true });
    expect(screen.getByRole("radio", { name: "shared" })).toBeChecked();
  });

  it("keeps the dialog open and shows the words of the server when it refuses", async () => {
    const { k, onclose } = setup();
    k.api.addWorkspace.mockImplementation(() =>
      fail(409, "workspace.prefix_taken", { prefix: "OPS" }),
    );
    await fillIn("ops", "OPS");
    await fireEvent.click(screen.getByRole("button", { name: "追加" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "ID の接頭辞 OPS はすでに使われています",
    );
    expect(onclose).not.toHaveBeenCalled();
    expect(toasts.list).toHaveLength(0);
  });

  it("adds with Enter in a field, but not while an IME composes", async () => {
    const { k } = setup();
    await fillIn("ops", "OPS");
    const field = screen.getByRole("textbox", { name: "名前" });
    await fireEvent.keyDown(field, { key: "Enter", isComposing: true });
    await fireEvent.keyDown(field, { key: "Enter", keyCode: 229 });
    expect(k.api.addWorkspace).not.toHaveBeenCalled();
    await fireEvent.keyDown(field, { key: "Enter" });
    await waitFor(() => expect(k.api.addWorkspace).toHaveBeenCalledTimes(1));
  });

  it("focuses the name as soon as the dialog opens", async () => {
    setup();
    await new Promise((r) => setTimeout(r, 0));
    expect(screen.getByRole("textbox", { name: "名前" })).toHaveFocus();
  });
});
