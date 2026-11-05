import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { beforeEach, describe, expect, it } from "vitest";
import { resetToasts, toasts } from "../toast.svelte";
import History from "./History.svelte";
import { fail, makeCore, mount, ok, showView, taskView } from "./testkit";

const log = (hash: string, op: string, date: string, subject: string) => ({
  hash,
  date,
  op,
  subject,
  task: "U6",
  id: "REL-6",
  store: "shared",
  author: { name: "koki", email: "k@example.com" },
  trailers: [],
});
const logs = [
  log("bbbbbbb1", "set", "2026-11-04T11:42:00Z", "set REL-6"),
  log("aaaaaaa1", "add", "2026-11-03T13:58:00Z", "add REL-6"),
];
const show = showView({
  ulid: "U6",
  id: "REL-6",
  store: "shared",
  title: "今の題",
  status: "in_progress",
  updated: "2026-11-04T11:42:00Z",
});
const then = showView({ ...show, title: "元の題", status: "todo" });

// GET history answers the list, GET history/{commit} the task at that time.
function setup(readonly = false, at = then) {
  const k = makeCore({ show });
  k.api.GET.mockImplementation((path: string) =>
    path === "/api/tasks/{id}/history/{commit}" ? ok(at) : ok(logs),
  );
  return { k, ...mount(History, { show, readonly }, k) };
}

describe("History", () => {
  beforeEach(() => resetToasts());

  it("says which rule made a commit, and which rule was refused for this task", async () => {
    const k = makeCore({ show });
    const trailers = [
      { key: "Hedgehug-Automation", value: "pr-opened pr.opened" },
      { key: "Hedgehug-Automation-Rejected", value: "U6 pr-merged points: required" },
      { key: "Hedgehug-Automation-Rejected", value: "U7 pr-merged about another task" },
    ];
    k.api.GET.mockImplementation(() => ok([{ ...logs[0], trailers }]));
    mount(History, { show, readonly: false }, k);
    expect(
      await screen.findByText("自動化 · PR が開かれた · ルール pr-opened"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("自動化を拒否 · ルール pr-merged · points: required"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/about another task/)).toBeNull();
  });

  it("loads the commits of the task, newest first, with the operation in Japanese", async () => {
    const { k } = setup();
    expect(await screen.findByText(/koki · 変更/)).toBeInTheDocument();
    expect(k.api.GET).toHaveBeenCalledWith("/api/tasks/{id}/history", {
      params: { path: { id: "U6" }, query: { store: "shared", limit: 50 } },
    });
    const rows = screen.getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("Hedgehug-Op: set");
    expect(rows[0]).toHaveTextContent("bbbbbbb");
    expect(rows[1]).toHaveTextContent("作成");
  });

  it("reads the log again when only a comment was saved (updated unchanged)", async () => {
    const { k, rerender } = setup();
    await screen.findByText(/koki · 変更/);
    const calls = () =>
      k.api.GET.mock.calls.filter((c: unknown[]) => c[0] === "/api/tasks/{id}/history").length;
    expect(calls()).toBe(1);
    await rerender({ show: { ...show, comments: [{ ulid: "C1" }] }, readonly: false });
    await waitFor(() => expect(calls()).toBe(2));
  });

  it("offers 'この時点に戻す' on every commit but the newest", async () => {
    setup();
    await screen.findByText(/koki · 変更/);
    expect(screen.getAllByRole("button", { name: "この時点に戻す" })).toHaveLength(1);
  });

  it("lists what changes, then restores at the commit and shows the note of the response", async () => {
    const { k } = setup();
    k.api.POST.mockImplementation(() =>
      ok({ task: taskView({ ulid: "U6" }), commit: "c9", note: "2 項目を戻しました" }),
    );
    await fireEvent.click(await screen.findByRole("button", { name: "この時点に戻す" }));
    const dialog = await screen.findByRole("dialog");
    expect(k.api.GET).toHaveBeenCalledWith("/api/tasks/{id}/history/{commit}", {
      params: { path: { id: "U6", commit: "aaaaaaa1" }, query: { store: "shared" } },
    });
    const table = await within(dialog).findByRole("table");
    expect(
      within(table).getByRole("row", { name: /タイトル.*今の題.*元の題/ }),
    ).toBeInTheDocument();
    expect(
      within(table).getByRole("row", { name: /ステータス.*in_progress.*todo/ }),
    ).toBeInTheDocument();
    await fireEvent.click(within(dialog).getByRole("button", { name: "2 項目を戻す" }));
    await waitFor(() =>
      expect(k.api.POST).toHaveBeenCalledWith("/api/tasks/{id}/history/{commit}/restore", {
        params: { path: { id: "U6", commit: "aaaaaaa1" }, query: { store: "shared" } },
      }),
    );
    expect(k.cache.applyWrite).toHaveBeenCalledWith({ task: expect.anything(), commit: "c9" });
    expect(toasts.list[0]?.message).toBe("2 項目を戻しました");
  });

  it("cannot close the restore confirmation while the restore runs, so its error shows", async () => {
    const { k } = setup();
    let release!: () => void;
    k.api.POST.mockImplementation(
      () =>
        new Promise((r) => {
          release = () => r(fail(409, "store.read_only", { store: "shared" }));
        }),
    );
    await fireEvent.click(await screen.findByRole("button", { name: "この時点に戻す" }));
    const dialog = await screen.findByRole("dialog");
    await fireEvent.click(await within(dialog).findByRole("button", { name: "2 項目を戻す" }));
    expect(within(dialog).getByRole("button", { name: "キャンセル" })).toBeDisabled();
    await fireEvent.keyDown(dialog, { key: "Escape" });
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    release();
    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
  });

  it("says so, and offers no restore, when the task is the same as at that time", async () => {
    setup(false, show);
    await fireEvent.click(await screen.findByRole("button", { name: "この時点に戻す" }));
    const dialog = await screen.findByRole("dialog");
    expect(await within(dialog).findByText(/この時点と同じです/)).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: /項目を戻す/ })).toBeNull();
  });

  it("shows the error when the task did not exist at that time", async () => {
    const { k } = setup();
    k.api.GET.mockImplementation((path: string) =>
      path === "/api/tasks/{id}/history/{commit}"
        ? fail(404, "restore_at.not_found", { at: "aaaaaaa1" })
        : ok(logs),
    );
    await fireEvent.click(await screen.findByRole("button", { name: "この時点に戻す" }));
    const dialog = await screen.findByRole("dialog");
    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
    expect(within(dialog).queryByRole("button", { name: /項目を戻す/ })).toBeNull();
  });

  it("does not offer the restore for a deleted task", async () => {
    setup(true);
    await screen.findByText(/koki · 変更/);
    expect(screen.queryByRole("button", { name: "この時点に戻す" })).toBeNull();
  });
});
