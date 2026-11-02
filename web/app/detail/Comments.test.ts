import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { resetToasts, toasts } from "../toast.svelte";
import Comments from "./Comments.svelte";
import { fail, makeCore, mount, ok, showView, stubLayout, typeInto, viewOf } from "./testkit";

const c1 = {
  ulid: "C1",
  author: "tanaka",
  created: "2026-11-04T10:12:00Z",
  body: "ガイドでは「カーソル」で統一します。",
};
const show = showView({
  ulid: "U6",
  id: "REL-6",
  store: "shared",
  comments: [c1],
  updated: "2026-11-04T10:12:00Z",
});

const setup = (readonly = false) => {
  const k = makeCore({ show });
  return { k, ...mount(Comments, { show, readonly }, k) };
};

describe("Comments", () => {
  beforeAll(stubLayout);
  beforeEach(() => resetToasts());

  it("lists the comments with their author", () => {
    setup();
    expect(screen.getByRole("heading", { name: /コメント/ })).toHaveTextContent("1");
    expect(screen.getByText("ガイドでは「カーソル」で統一します。")).toBeInTheDocument();
    expect(screen.getByText("tanaka")).toBeInTheDocument();
  });

  it("posts a comment, shows it at once and tells the cache the commit is its own", async () => {
    const { k, container } = setup();
    k.api.POST.mockImplementation(() =>
      ok({
        comment: { ulid: "C2", author: "koki", created: "2026-11-04T11:00:00Z", body: "了解です" },
        commit: "c1",
      }),
    );
    typeInto(container, "了解です");
    await fireEvent.click(await screen.findByRole("button", { name: "コメント" }));
    expect(k.api.POST).toHaveBeenCalledWith("/api/tasks/{id}/comments", {
      params: { path: { id: "U6" }, query: { store: "shared" } },
      body: { body: "了解です" },
    });
    expect(await screen.findByText("了解です")).toBeInTheDocument();
    expect(k.cache.noteOwnCommit).toHaveBeenCalledWith("c1");
    await waitFor(() => expect(viewOf(container).state.doc.toString()).toBe(""));
  });

  it("cannot post an empty comment", () => {
    setup();
    expect(screen.getByRole("button", { name: "コメント" })).toBeDisabled();
  });

  it("offers the people for @ in the comment, from GET /api/people", async () => {
    const { k, container } = setup();
    k.api.GET.mockImplementation(() => ok(["suzuki", "suzumura"]));
    const view = viewOf(container);
    view.dispatch({
      changes: { from: 0, insert: "@suz" },
      selection: { anchor: 4 },
      userEvent: "input.type",
    });
    await vi.waitFor(() =>
      expect(k.api.GET).toHaveBeenCalledWith("/api/people", {
        params: { query: { store: "shared" } },
      }),
    );
  });

  it("edits a comment", async () => {
    const { k, container } = setup();
    k.api.PATCH.mockImplementation(() =>
      ok({ comment: { ...c1, body: "直しました" }, commit: "c2" }),
    );
    await fireEvent.click(screen.getByRole("button", { name: "編集" }));
    typeInto(container, "直しました", 0);
    await fireEvent.click(await screen.findByRole("button", { name: "保存" }));
    expect(k.api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}/comments/{cid}", {
      params: { path: { id: "U6", cid: "C1" }, query: { store: "shared" } },
      body: { body: "直しました" },
    });
    expect(await screen.findByText("直しました")).toBeInTheDocument();
  });

  it("drops its overlay when the comments are read again, even if the task's updated did not change", async () => {
    const { k, container, rerender } = setup();
    k.api.PATCH.mockImplementation(() =>
      ok({ comment: { ...c1, body: "直しました" }, commit: "c2" }),
    );
    await fireEvent.click(screen.getByRole("button", { name: "編集" }));
    typeInto(container, "直しました", 0);
    await fireEvent.click(await screen.findByRole("button", { name: "保存" }));
    expect(await screen.findByText("直しました")).toBeInTheDocument();
    await rerender({
      show: { ...show, comments: [{ ...c1, body: "別の人が直した" }] },
      readonly: false,
    });
    expect(await screen.findByText("別の人が直した")).toBeInTheDocument();
    expect(screen.queryByText("直しました")).toBeNull();
  });

  it("shows comment.not_author under the comment when someone else's comment is edited", async () => {
    const { k } = setup();
    k.api.PATCH.mockImplementation(() => fail(409, "comment.not_author", { author: "tanaka" }));
    await fireEvent.click(screen.getByRole("button", { name: "編集" }));
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });

  it("deletes a comment and says so", async () => {
    const { k } = setup();
    k.api.DELETE.mockImplementation(() => ok({ comment: c1, commit: "c3" }));
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    expect(k.api.DELETE).toHaveBeenCalledWith("/api/tasks/{id}/comments/{cid}", {
      params: { path: { id: "U6", cid: "C1" }, query: { store: "shared" } },
    });
    await waitFor(() =>
      expect(screen.queryByText("ガイドでは「カーソル」で統一します。")).toBeNull(),
    );
    expect(toasts.list[0]?.message).toBe("コメントを削除しました");
  });

  it("has no form when read-only", () => {
    setup(true);
    expect(screen.queryByRole("button", { name: "コメント" })).toBeNull();
    expect(screen.queryByRole("button", { name: "編集" })).toBeNull();
  });
});
