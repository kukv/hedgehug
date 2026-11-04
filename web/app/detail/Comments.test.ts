import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { resetToasts, toasts } from "../toast.svelte";
import Comments from "./Comments.svelte";
import {
  type FakeCoreOptions,
  fail,
  makeCore,
  mount,
  ok,
  showView,
  stubLayout,
  typeInto,
  viewOf,
} from "./testkit";

const c1 = {
  ulid: "C1",
  author: "github.com/tanaka",
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

const setup = (readonly = false, opts: FakeCoreOptions = {}) => {
  const k = makeCore({ show, accounts: ["github.com/tanaka", "github.com/koki"], ...opts });
  return { k, ...mount(Comments, { show, readonly }, k) };
};

describe("Comments", () => {
  beforeAll(stubLayout);
  beforeEach(() => resetToasts());

  it("lists the comments with their author", () => {
    setup();
    expect(screen.getByRole("heading", { name: /コメント/ })).toHaveTextContent("1");
    expect(screen.getByText("ガイドでは「カーソル」で統一します。")).toBeInTheDocument();
    expect(screen.getByText("github.com/tanaka")).toBeInTheDocument();
  });

  it("posts a comment, shows it at once and tells the cache the commit is its own", async () => {
    const { k, container } = setup();
    k.api.POST.mockImplementation(() =>
      ok({
        comment: {
          ulid: "C2",
          author: "github.com/koki",
          created: "2026-11-04T11:00:00Z",
          body: "了解です",
        },
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
    const dialog = await screen.findByRole("dialog", { name: "コメントを削除" });
    expect(
      within(dialog).getByText("このコメントを削除します。元に戻せません。"),
    ).toBeInTheDocument();
    expect(dialog).toHaveTextContent("hh comment rm C1");
    expect(k.api.DELETE).not.toHaveBeenCalled();
    await fireEvent.click(within(dialog).getByRole("button", { name: "削除" }));
    await waitFor(() => expect(k.api.DELETE).toHaveBeenCalled());
    expect(k.api.DELETE).toHaveBeenCalledWith("/api/tasks/{id}/comments/{cid}", {
      params: { path: { id: "U6", cid: "C1" }, query: { store: "shared" } },
    });
    await waitFor(() =>
      expect(screen.queryByText("ガイドでは「カーソル」で統一します。")).toBeNull(),
    );
    expect(toasts.list[0]?.message).toBe("コメントを削除しました");
  });

  it("does not delete when the confirmation is cancelled", async () => {
    const { k } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    const dialog = await screen.findByRole("dialog", { name: "コメントを削除" });
    await fireEvent.click(within(dialog).getByRole("button", { name: "キャンセル" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(k.api.DELETE).not.toHaveBeenCalled();
    expect(screen.getByText("ガイドでは「カーソル」で統一します。")).toBeInTheDocument();
  });

  it("shows the error in the confirmation and keeps it open", async () => {
    const { k } = setup();
    k.api.DELETE.mockImplementation(() => fail(409, "comment.not_author", { author: "tanaka" }));
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    const dialog = await screen.findByRole("dialog", { name: "コメントを削除" });
    await fireEvent.click(within(dialog).getByRole("button", { name: "削除" }));
    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
  });

  it("keeps what was written here when a read brings only the change of someone else", async () => {
    const { k, container, rerender } = setup();
    k.api.PATCH.mockImplementation(() =>
      ok({ comment: { ...c1, body: "直しました" }, commit: "c2" }),
    );
    await fireEvent.click(screen.getByRole("button", { name: "編集" }));
    typeInto(container, "直しました", 0);
    await fireEvent.click(await screen.findByRole("button", { name: "保存" }));
    expect(await screen.findByText("直しました")).toBeInTheDocument();
    const other = { ulid: "C2", author: "sato", created: "2026-11-04T11:00:00Z", body: "他の人" };
    await rerender({ show: { ...show, comments: [c1, other] }, readonly: false });
    expect(await screen.findByText("他の人")).toBeInTheDocument();
    expect(screen.getByText("直しました")).toBeInTheDocument();
    await rerender({
      show: { ...show, comments: [{ ...c1, body: "直しました" }, other] },
      readonly: false,
    });
    expect(screen.getByText("直しました")).toBeInTheDocument();
  });

  it("keeps a comment deleted here hidden until the read lacks it", async () => {
    const { k, rerender } = setup();
    k.api.DELETE.mockImplementation(() => ok({ comment: c1, commit: "c5" }));
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    const dialog = await screen.findByRole("dialog", { name: "コメントを削除" });
    await fireEvent.click(within(dialog).getByRole("button", { name: "削除" }));
    await waitFor(() => expect(screen.queryByText(c1.body)).toBeNull());
    const other = { ulid: "C2", author: "sato", created: "2026-11-04T11:00:00Z", body: "他の人" };
    await rerender({ show: { ...show, comments: [c1, other] }, readonly: false });
    expect(await screen.findByText("他の人")).toBeInTheDocument();
    expect(screen.queryByText(c1.body)).toBeNull();
    await rerender({ show: { ...show, comments: [other] }, readonly: false });
    expect(screen.getByText("他の人")).toBeInTheDocument();
  });

  it("keeps a comment posted here until the read has it", async () => {
    const { k, container, rerender } = setup();
    const mine = {
      ulid: "C9",
      author: "github.com/koki",
      created: "2026-11-04T12:00:00Z",
      body: "自分の",
    };
    k.api.POST.mockImplementation(() => ok({ comment: mine, commit: "c4" }));
    typeInto(container, "自分の", 0);
    await fireEvent.click(await screen.findByRole("button", { name: "コメント" }));
    expect(await screen.findByText("自分の")).toBeInTheDocument();
    const other = { ulid: "C2", author: "sato", created: "2026-11-04T11:00:00Z", body: "他の人" };
    await rerender({ show: { ...show, comments: [c1, other] }, readonly: false });
    expect(await screen.findByText("他の人")).toBeInTheDocument();
    expect(screen.getByText("自分の")).toBeInTheDocument();
  });

  it("shows an edit of a comment posted here, and keeps it when a read brings it as it was posted", async () => {
    const { k, container, rerender } = setup();
    const mine = {
      ulid: "C9",
      author: "github.com/koki",
      created: "2026-11-04T12:00:00Z",
      body: "自分の",
    };
    k.api.POST.mockImplementation(() => ok({ comment: mine, commit: "c4" }));
    k.api.PATCH.mockImplementation(() =>
      ok({ comment: { ...mine, body: "直した自分の" }, commit: "c5" }),
    );
    typeInto(container, "自分の", 0);
    await fireEvent.click(await screen.findByRole("button", { name: "コメント" }));
    expect(await screen.findByText("自分の")).toBeInTheDocument();
    const edits = screen.getAllByRole("button", { name: "編集" });
    await fireEvent.click(edits[edits.length - 1] as HTMLElement);
    typeInto(container, "直した自分の", 0);
    await fireEvent.click(await screen.findByRole("button", { name: "保存" }));
    expect(await screen.findByText("直した自分の")).toBeInTheDocument();
    await rerender({ show: { ...show, comments: [c1, mine] }, readonly: false });
    expect(screen.getByText("直した自分の")).toBeInTheDocument();
    expect(screen.queryByText("自分の")).toBeNull();
  });

  it("keeps the second edit of a comment posted here when a read brings it as it was posted", async () => {
    const { k, container, rerender } = setup();
    const mine = {
      ulid: "C9",
      author: "github.com/koki",
      created: "2026-11-04T12:00:00Z",
      body: "自分の",
    };
    k.api.POST.mockImplementation(() => ok({ comment: mine, commit: "c4" }));
    let n = 0;
    k.api.PATCH.mockImplementation(() =>
      ok({ comment: { ...mine, body: n++ === 0 ? "直した1" : "直した2" }, commit: "c5" }),
    );
    typeInto(container, "自分の", 0);
    await fireEvent.click(await screen.findByRole("button", { name: "コメント" }));
    expect(await screen.findByText("自分の")).toBeInTheDocument();
    for (const text of ["直した1", "直した2"]) {
      const edits = screen.getAllByRole("button", { name: "編集" });
      await fireEvent.click(edits[edits.length - 1] as HTMLElement);
      typeInto(container, text, 0);
      await fireEvent.click(await screen.findByRole("button", { name: "保存" }));
      expect(await screen.findByText(text)).toBeInTheDocument();
    }
    await rerender({ show: { ...show, comments: [c1, mine] }, readonly: false });
    expect(screen.getByText("直した2")).toBeInTheDocument();
    expect(screen.queryByText("自分の")).toBeNull();
  });

  for (const texts of [["直した1"], ["直した1", "直した2"]]) {
    it(`keeps the edit of a comment posted here through a read that lacks it (${texts.length} edits)`, async () => {
      const { k, container, rerender } = setup();
      const mine = {
        ulid: "C9",
        author: "github.com/koki",
        created: "2026-11-04T12:00:00Z",
        body: "自分の",
      };
      const other = { ulid: "C2", author: "sato", created: "2026-11-04T11:00:00Z", body: "他の人" };
      k.api.POST.mockImplementation(() => ok({ comment: mine, commit: "c4" }));
      let n = 0;
      k.api.PATCH.mockImplementation(() =>
        ok({ comment: { ...mine, body: texts[n++] }, commit: "c5" }),
      );
      typeInto(container, "自分の", 0);
      await fireEvent.click(await screen.findByRole("button", { name: "コメント" }));
      expect(await screen.findByText("自分の")).toBeInTheDocument();
      const edit = async (text: string) => {
        const edits = screen.getAllByRole("button", { name: "編集" });
        await fireEvent.click(edits[edits.length - 1] as HTMLElement);
        typeInto(container, text, 0);
        await fireEvent.click(await screen.findByRole("button", { name: "保存" }));
        expect(await screen.findByText(text)).toBeInTheDocument();
      };
      await edit(texts[0] as string);
      // a read sent before the post comes late, with the comment of someone else
      await rerender({ show: { ...show, comments: [c1, other] }, readonly: false });
      expect(await screen.findByText("他の人")).toBeInTheDocument();
      if (texts[1]) await edit(texts[1]);
      // then a read that has the comment as it was posted
      await rerender({ show: { ...show, comments: [c1, other, mine] }, readonly: false });
      expect(screen.getByText(texts[texts.length - 1] as string)).toBeInTheDocument();
      expect(screen.queryByText("自分の")).toBeNull();
    });
  }

  for (const edited of [false, true]) {
    it(`deletes a comment posted here before a read has it${edited ? ", after an edit" : ""}`, async () => {
      const { k, container, rerender } = setup();
      const mine = {
        ulid: "C9",
        author: "github.com/koki",
        created: "2026-11-04T12:00:00Z",
        body: "自分の",
      };
      const other = { ulid: "C2", author: "sato", created: "2026-11-04T11:00:00Z", body: "他の人" };
      k.api.POST.mockImplementation(() => ok({ comment: mine, commit: "c4" }));
      k.api.PATCH.mockImplementation(() =>
        ok({ comment: { ...mine, body: "直した自分の" }, commit: "c5" }),
      );
      k.api.DELETE.mockImplementation(() => ok({ comment: mine, commit: "c6" }));
      typeInto(container, "自分の", 0);
      await fireEvent.click(await screen.findByRole("button", { name: "コメント" }));
      let text = "自分の";
      expect(await screen.findByText(text)).toBeInTheDocument();
      if (edited) {
        const edits = screen.getAllByRole("button", { name: "編集" });
        await fireEvent.click(edits[edits.length - 1] as HTMLElement);
        text = "直した自分の";
        typeInto(container, text, 0);
        await fireEvent.click(await screen.findByRole("button", { name: "保存" }));
        expect(await screen.findByText(text)).toBeInTheDocument();
      }
      const removes = screen.getAllByRole("button", { name: "削除" });
      await fireEvent.click(removes[removes.length - 1] as HTMLElement);
      const dialog = await screen.findByRole("dialog", { name: "コメントを削除" });
      await fireEvent.click(within(dialog).getByRole("button", { name: "削除" }));
      await waitFor(() => expect(screen.queryByText(text)).toBeNull());
      expect(screen.getByRole("heading", { name: /コメント/ })).toHaveTextContent("1");
      // a read that lacks it (sent before the post, or after the deletion)
      await rerender({ show: { ...show, comments: [c1, other] }, readonly: false });
      expect(await screen.findByText("他の人")).toBeInTheDocument();
      expect(screen.queryByText(text)).toBeNull();
      expect(screen.queryByText("自分の")).toBeNull();
    });
  }

  it("keeps a comment posted and deleted here hidden from a read sent before the deletion", async () => {
    const { k, container, rerender } = setup();
    const mine = {
      ulid: "C9",
      author: "github.com/koki",
      created: "2026-11-04T12:00:00Z",
      body: "自分の",
    };
    const other = { ulid: "C2", author: "sato", created: "2026-11-04T11:00:00Z", body: "他の人" };
    k.api.POST.mockImplementation(() => ok({ comment: mine, commit: "c4" }));
    k.api.DELETE.mockImplementation(() => ok({ comment: mine, commit: "c6" }));
    typeInto(container, "自分の", 0);
    await fireEvent.click(await screen.findByRole("button", { name: "コメント" }));
    expect(await screen.findByText("自分の")).toBeInTheDocument();
    const removes = screen.getAllByRole("button", { name: "削除" });
    await fireEvent.click(removes[removes.length - 1] as HTMLElement);
    const dialog = await screen.findByRole("dialog", { name: "コメントを削除" });
    await fireEvent.click(within(dialog).getByRole("button", { name: "削除" }));
    await waitFor(() => expect(screen.queryByText("自分の")).toBeNull());
    // a read sent after the post and before the deletion has it as it was posted
    await rerender({ show: { ...show, comments: [c1, other, mine] }, readonly: false });
    expect(await screen.findByText("他の人")).toBeInTheDocument();
    expect(screen.queryByText("自分の")).toBeNull();
    // the read after the deletion lacks it
    await rerender({ show: { ...show, comments: [c1, other] }, readonly: false });
    expect(screen.queryByText("自分の")).toBeNull();
  });

  it("posts once when the key is pressed twice before the answer", async () => {
    const { k, container } = setup();
    k.api.POST.mockImplementation(() => new Promise(() => {}));
    typeInto(container, "x", 0);
    const editor = viewOf(container, 0).contentDOM;
    const press = () =>
      fireEvent.keyDown(editor, { key: "Enter", code: "Enter", keyCode: 13, ctrlKey: true });
    await press();
    await press();
    expect(k.api.POST).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "コメント" })).toBeDisabled();
  });

  it("cannot close the confirmation while the deletion runs, so its error is not lost", async () => {
    const { k } = setup();
    let release!: () => void;
    k.api.DELETE.mockImplementation(
      () =>
        new Promise((r) => {
          release = () => r(fail(409, "comment.not_author", { author: "tanaka" }));
        }),
    );
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    const dialog = await screen.findByRole("dialog", { name: "コメントを削除" });
    await fireEvent.click(within(dialog).getByRole("button", { name: "削除" }));
    expect(within(dialog).getByRole("button", { name: "キャンセル" })).toBeDisabled();
    await fireEvent.keyDown(dialog, { key: "Escape" });
    expect(screen.getByRole("dialog", { name: "コメントを削除" })).toBeInTheDocument();
    release();
    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
  });

  describe("who may edit or delete", () => {
    const buttons = () => [
      screen.queryAllByRole("button", { name: "編集" }).length,
      screen.queryAllByRole("button", { name: "削除" }).length,
    ];
    const by = (author: string) => ({
      ulid: "C2",
      author,
      created: "2026-11-04T11:00:00Z",
      body: "x",
    });
    const withComment = (author: string, opts: FakeCoreOptions = {}) => {
      const s = { ...show, comments: [by(author)] };
      const k = makeCore({ show: s, accounts: ["github.com/koki"], ...opts });
      return { k, ...mount(Comments, { show: s, readonly: false }, k) };
    };

    it("loads the accounts when the comments are shown", () => {
      const { k } = withComment("github.com/koki");
      expect(k.accounts.load).toHaveBeenCalled();
    });

    it("offers them on the comment of an account of this machine, whatever the case", () => {
      withComment("GitHub.com/Koki");
      expect(buttons()).toEqual([1, 1]);
    });

    it("does not offer them on the comment of someone else", () => {
      withComment("github.com/bob");
      expect(buttons()).toEqual([0, 0]);
    });

    it("does not offer them on a comment with no author", () => {
      withComment("");
      expect(buttons()).toEqual([0, 0]);
    });

    it("does not offer them before the accounts are read", () => {
      withComment("github.com/koki", { accountsLoaded: false });
      expect(buttons()).toEqual([0, 0]);
    });

    it("does not offer them when the accounts could not be read", () => {
      withComment("github.com/koki", { accountsError: "boom" });
      expect(buttons()).toEqual([0, 0]);
    });
  });

  it("has no form when read-only", () => {
    setup(true);
    expect(screen.queryByRole("button", { name: "コメント" })).toBeNull();
    expect(screen.queryByRole("button", { name: "編集" })).toBeNull();
  });
});
