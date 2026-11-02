import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { beforeEach, describe, expect, it } from "vitest";
import { resetToasts, toasts, undoToast } from "../toast.svelte";
import DeleteDialogs from "./DeleteDialogs.svelte";
import { fail, makeCore, mount, ok, showView, taskView } from "./testkit";

const show = showView({ ulid: "U6", id: "REL-6", store: "shared", title: "リリース作業" });

describe("DeleteDialogs (variant delete)", () => {
  beforeEach(() => resetToasts());

  it("lists the descendants that go with it and deletes with an undo toast", async () => {
    const k = makeCore({ show });
    k.api.GET.mockImplementation((_p: string, o: { params: { query: { q: string } } }) =>
      o.params.query.q === "parent:U6"
        ? ok({
            tasks: [
              taskView({ ulid: "A", id: "REL-9", title: "負荷試験" }),
              taskView({ ulid: "B", id: "REL-10", title: "502" }),
            ],
          })
        : ok({ tasks: [] }),
    );
    k.api.DELETE.mockImplementation(() => ok({ commit: "c1", tasks: [taskView({ ulid: "U6" })] }));
    k.api.POST.mockImplementation(() => ok({ commit: "c2", tasks: [taskView({ ulid: "U6" })] }));
    mount(DeleteDialogs, { show, variant: "delete" }, k);
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    const dialog = await screen.findByRole("dialog");
    expect(await within(dialog).findByText(/配下の 2 件も一緒に削除します/)).toBeInTheDocument();
    expect(within(dialog).getByText("REL-9")).toBeInTheDocument();
    await fireEvent.click(within(dialog).getByRole("button", { name: "3 件を削除" }));
    await waitFor(() => expect(toasts.list).toHaveLength(1));
    const t = toasts.list[0];
    expect(t?.message).toContain("リリース作業");
    await undoToast(t?.id ?? 0);
    expect(k.api.POST).toHaveBeenCalledWith("/api/tasks/{id}/restore", {
      params: { path: { id: "U6" }, query: { store: "shared" } },
    });
  });

  it("deletes a task without children with a plain button", async () => {
    const k = makeCore({ show });
    k.api.GET.mockImplementation(() => ok({ tasks: [] }));
    mount(DeleteDialogs, { show, variant: "delete" }, k);
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    const dialog = await screen.findByRole("dialog");
    const del = await within(dialog).findByRole("button", { name: "削除" });
    await waitFor(() => expect(del).toBeEnabled());
  });
});

describe("DeleteDialogs (variant deleted)", () => {
  const gone = showView({ ...show, deleted_at: "2026-11-03T10:00:00Z" });
  const targets = [
    { ulid: "U6", id: "REL-6", title: "リリース作業" },
    { ulid: "U7", id: "REL-7", title: "子" },
  ];

  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/tasks/U6");
  });

  it("restores", async () => {
    const k = makeCore({ show: gone });
    k.api.POST.mockImplementation(() => ok({ commit: "c1", tasks: [taskView({ ulid: "U6" })] }));
    mount(DeleteDialogs, { show: gone, variant: "deleted" }, k);
    await fireEvent.click(screen.getByRole("button", { name: "復元" }));
    await waitFor(() =>
      expect(k.api.POST).toHaveBeenCalledWith("/api/tasks/{id}/restore", expect.anything()),
    );
    await waitFor(() => expect(toasts.list[0]?.message).toBe("REL-6 を復元しました"));
  });

  it("purges only after the targets are listed and the store name is typed", async () => {
    const k = makeCore({ show: gone });
    k.api.DELETE.mockImplementationOnce(() =>
      fail(409, "purge.unconfirmed", { targets }),
    ).mockImplementationOnce(() => ok({ commit: "c3", ulids: ["U6", "U7"] }));
    mount(DeleteDialogs, { show: gone, variant: "deleted" }, k);
    await fireEvent.click(screen.getByRole("button", { name: "完全に削除" }));
    const dialog = await screen.findByRole("dialog");
    expect(await within(dialog).findByText("REL-7")).toBeInTheDocument();
    expect(
      within(dialog).getByText(
        "タスクのファイルとコメントを消します。Hedgehug からは復元できません。",
      ),
    ).toBeInTheDocument();
    const go = within(dialog).getByRole("button", { name: "完全に削除" });
    expect(go).toBeDisabled();
    await fireEvent.input(within(dialog).getByRole("textbox", { name: "Store 名" }), {
      target: { value: "shared" },
    });
    expect(go).toBeEnabled();
    await fireEvent.click(go);
    await waitFor(() =>
      expect(k.api.DELETE).toHaveBeenLastCalledWith("/api/tasks/{id}", {
        params: { path: { id: "U6" }, query: { store: "shared", purge: true, confirm: "U6,U7" } },
      }),
    );
    await waitFor(() => expect(location.pathname).toBe("/"));
    expect(k.cache.applyWrite).toHaveBeenCalledWith({ ulids: ["U6", "U7"], commit: "c3" });
  });

  it("replaces the targets and clears the input when the confirmation is refused again", async () => {
    const k = makeCore({ show: gone });
    const changed = [...targets, { ulid: "U8", id: "REL-8", title: "増えた子" }];
    k.api.DELETE.mockImplementationOnce(() =>
      fail(409, "purge.unconfirmed", { targets }),
    ).mockImplementationOnce(() => fail(409, "purge.unconfirmed", { targets: changed }));
    mount(DeleteDialogs, { show: gone, variant: "deleted" }, k);
    await fireEvent.click(screen.getByRole("button", { name: "完全に削除" }));
    const dialog = await screen.findByRole("dialog");
    await within(dialog).findByText("REL-7");
    const box = within(dialog).getByRole("textbox", { name: "Store 名" });
    await fireEvent.input(box, { target: { value: "shared" } });
    await fireEvent.click(within(dialog).getByRole("button", { name: "完全に削除" }));
    expect(await within(dialog).findByText("REL-8")).toBeInTheDocument();
    expect(box).toHaveValue("");
    expect(within(dialog).getByRole("alert")).toHaveTextContent("対象が変わりました");
  });
});
