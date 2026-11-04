import { cleanup, fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import StoreRemoveDialog from "./StoreRemoveDialog.svelte";

function mount(
  del: () => unknown = () => ok({ name: "team", path: "/p", deleted: true }),
  remotes: string[] | null = ["ssh://example/r.repo"],
) {
  const api = { DELETE: vi.fn(del) };
  const reload = vi.fn(async () => {});
  const onclose = vi.fn();
  renderWithCore(
    StoreRemoveDialog,
    { name: "team", onclose },
    {
      api,
      stores: { reload, stores: () => [{ name: "team", remotes }], subscribe: () => () => {} },
    },
  );
  return { api, reload, onclose };
}

const submit = () => screen.getByRole("button", { name: "解除する" });

beforeEach(() => {
  resetToasts();
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
});
afterEach(() => cleanup());

describe("StoreRemoveDialog", () => {
  it("asks about the mount of the Store, with the clone deleted by default", () => {
    mount();
    expect(screen.getByRole("dialog", { name: "team のマウントを解除" })).toBeTruthy();
    const box = screen.getByRole("checkbox", {
      name: "ローカルのクローンも消す",
    }) as HTMLInputElement;
    expect(box.checked).toBe(true);
  });

  it("shows hh store remove, with --keep once the clone is kept", async () => {
    mount();
    expect(screen.getByRole("button", { name: /^コマンドをコピー/ }).textContent).toContain(
      "$ hh store remove team",
    );
    expect(screen.getByRole("button", { name: /^コマンドをコピー/ }).textContent).not.toContain(
      "--keep",
    );
    await fireEvent.click(screen.getByRole("checkbox", { name: "ローカルのクローンも消す" }));
    expect(screen.getByRole("button", { name: /^コマンドをコピー/ }).textContent).toContain(
      "$ hh store remove team --keep",
    );
  });

  it("deletes the clone: DELETE with keep=false, a toast, a reload and the dialog closes", async () => {
    const { api, reload, onclose } = mount();
    await fireEvent.click(submit());
    await waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
    expect(api.DELETE).toHaveBeenCalledWith("/api/stores/{name}", {
      params: { path: { name: "team" }, query: { keep: false } },
    });
    expect(reload).toHaveBeenCalledTimes(1);
    expect(toasts.list.map((t) => t.message)).toEqual(["team のマウントを解除しました"]);
  });

  it("keeps the clone with keep=true when the check is off", async () => {
    const { api, onclose } = mount();
    await fireEvent.click(screen.getByRole("checkbox", { name: "ローカルのクローンも消す" }));
    await fireEvent.click(submit());
    await waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
    expect(api.DELETE).toHaveBeenCalledWith("/api/stores/{name}", {
      params: { path: { name: "team" }, query: { keep: true } },
    });
  });

  it("stays on store.unpushed with the words, and offers to keep the clone", async () => {
    const del = vi
      .fn()
      .mockImplementationOnce(() => fail(409, "store.unpushed", { name: "team" }))
      .mockImplementationOnce(() => ok({ name: "team", path: "/p", deleted: false }));
    const { api, onclose, reload } = mount(del);
    expect(screen.queryByRole("button", { name: "クローンを残して解除" })).toBeNull();
    await fireEvent.click(submit());
    await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
    expect(screen.getByRole("alert").textContent).toContain("まだ push していない変更があります");
    expect(onclose).not.toHaveBeenCalled();
    expect(reload).not.toHaveBeenCalled();
    expect(toasts.list).toHaveLength(0);

    await fireEvent.click(screen.getByRole("button", { name: "クローンを残して解除" }));
    await waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
    expect(api.DELETE).toHaveBeenLastCalledWith("/api/stores/{name}", {
      params: { path: { name: "team" }, query: { keep: true } },
    });
    expect(toasts.list.map((t) => t.message)).toEqual(["team のマウントを解除しました"]);
  });

  it("shows any other refusal and stays, without the keep button", async () => {
    const { onclose } = mount(() => fail(409, "store.locked"));
    await fireEvent.click(submit());
    await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
    expect(screen.getByRole("alert").textContent).toContain("別の hh が書き込み中です");
    expect(screen.queryByRole("button", { name: "クローンを残して解除" })).toBeNull();
    expect(onclose).not.toHaveBeenCalled();
  });

  it("starts with the clone kept for a Store with no remote, and says why when it is turned on", async () => {
    const { api, onclose } = mount(undefined, null);
    const box = screen.getByRole("checkbox", {
      name: "ローカルのクローンも消す",
    }) as HTMLInputElement;
    expect(box.checked).toBe(false);
    expect(screen.queryByText(/クローンごとは解除できません/)).toBeNull();
    expect(screen.getByRole("button", { name: /^コマンドをコピー/ }).textContent).toContain(
      "--keep",
    );
    await fireEvent.click(box);
    expect(screen.getByText(/タスクが失われるため/)).toBeTruthy();
    await fireEvent.click(box);
    await fireEvent.click(submit());
    await waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
    expect(api.DELETE).toHaveBeenCalledWith("/api/stores/{name}", {
      params: { path: { name: "team" }, query: { keep: true } },
    });
  });

  it("says a Store with no remote cannot delete its clone on 409, not to sync", async () => {
    mount(() => fail(409, "store.unpushed", { name: "team" }), []);
    await fireEvent.click(screen.getByRole("checkbox", { name: "ローカルのクローンも消す" }));
    await fireEvent.click(submit());
    await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
    expect(screen.getByRole("alert").textContent).toContain("タスクが失われるため");
    expect(screen.getByRole("alert").textContent).not.toContain("先に同期してください");
    expect(screen.getByRole("button", { name: "クローンを残して解除" })).toBeTruthy();
  });

  it("is closed while no Store is asked about", () => {
    renderWithCore(
      StoreRemoveDialog,
      { name: null, onclose: vi.fn() },
      { api: {}, stores: { reload: async () => {}, stores: () => [], subscribe: () => () => {} } },
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
