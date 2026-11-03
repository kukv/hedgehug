import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Notification } from "../../core/notifications";
import { FakeConn } from "../../core/testing";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import InboxPage from "./InboxPage.svelte";

const task = (id: string) => ({ ulid: `U-${id}`, id, title: `title of ${id}` });

function n(over: Partial<Notification> & { ulid: string }): Notification {
  return {
    author: "tanaka",
    commit: "c",
    kind: "mention",
    read: false,
    store: "shared",
    task: task("REL-4"),
    time: "2026-11-04T01:00:00Z",
    to: "alice",
    ...over,
  };
}

const overwritten = n({
  ulid: "N1",
  kind: "overwritten",
  author: "",
  task: task("REL-11"),
  detail: { field: "points", value: 2 },
  time: "2026-11-04T02:00:00Z",
});
const mention = n({ ulid: "N2", detail: { comment: "C1" }, time: "2026-11-04T01:30:00Z" });
const assigned = n({
  ulid: "N3",
  kind: "assigned",
  task: task("REL-27"),
  time: "2026-11-04T01:00:00Z",
});
const watched = n({
  ulid: "N4",
  kind: "watch",
  read: true,
  task: task("REL-13"),
  detail: { changed: ["status"] },
  time: "2026-11-03T09:00:00Z",
});
const repair = n({
  ulid: "N5",
  kind: "prefix_duplicate",
  author: "",
  task: null as never,
  detail: { kind: "prefix_duplicate", workspace: "W1", value: "REL" },
  time: "2026-11-03T08:00:00Z",
});

function setup(
  items: Notification[],
  opts: { stores?: string[]; unreadStores?: boolean; getError?: Error } = {},
) {
  let current = items;
  const conn = new FakeConn();
  const api = {
    GET: vi.fn(() => (opts.getError ? Promise.reject(opts.getError) : ok(current))),
    POST: vi.fn((path: string) =>
      path.endsWith("/revert") ? ok({ task: { id: "REL-11" } }) : ok({ read: [] }),
    ),
    PATCH: vi.fn(() =>
      Promise.resolve({ data: undefined, response: new Response(null, { status: 204 }) }),
    ),
  };
  const names = opts.stores ?? ["shared"];
  const view = renderWithCore(
    InboxPage,
    {},
    {
      api,
      conn,
      stores: {
        stores: () => names.map((name) => ({ name })),
        tone: () => "purple",
        workspaces: () => [],
        workspace: (_s: string, u: string) => (u === "W1" ? { name: "release" } : undefined),
        loaded: () => true,
        subscribe: () => () => {},
      },
    },
  );
  return {
    api,
    conn,
    view,
    set(next: Notification[]) {
      current = next;
    },
  };
}

const rowOf = (id: string) => {
  const row = screen.getAllByRole("listitem").find((r) => r.textContent?.includes(id));
  if (!row) throw new Error(`no row of ${id}`);
  return row;
};

beforeEach(() => {
  resetToasts();
  history.replaceState(null, "", "/inbox");
});
afterEach(() => cleanup());

describe("InboxPage", () => {
  it("asks once for the items of every store, the read ones too", async () => {
    const { api } = setup([overwritten]);
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(1));
    expect(api.GET).toHaveBeenCalledWith("/api/notifications", {
      params: { query: { all: true } },
    });
    expect(screen.getByRole("heading", { level: 1, name: "通知" })).toBeTruthy();
    expect(
      screen.getByText(
        "同期で届いた変更から · 自分の書き込みは通知しません · 既読はこのマシンに保存",
      ),
    ).toBeTruthy();
  });

  it("puts the unread first, newest first, with the sentence of each", async () => {
    setup([watched, assigned, mention, overwritten]);
    await screen.findByText("あなたが設定した points が、同時に編集した値で上書きされました");
    const rows = screen.getAllByRole("listitem");
    expect(rows.map((r) => /(REL-\d+)/.exec(r.textContent ?? "")?.[1])).toEqual([
      "REL-11",
      "REL-4",
      "REL-27",
      "REL-13",
    ]);
    expect(
      within(rowOf("REL-4")).getByText("tanaka がコメントであなたをメンションしました"),
    ).toBeTruthy();
    expect(within(rowOf("REL-11")).getByText("points: 2（あなた）")).toBeTruthy();
  });

  it("has the four tabs with the number of unread; zero shows no number", async () => {
    setup([overwritten, mention, assigned, watched]);
    await screen.findByText(/上書きされました/);
    const tabs = screen.getByRole("navigation", { name: "通知の種類" });
    const names = within(tabs)
      .getAllByRole("link")
      .map((t) => t.textContent?.replace(/\s+/g, ""));
    // no tab for assigned, watch or automations; the repair tab has nothing unread
    expect(names).toEqual(["すべて3", "上書き1", "メンション1", "同期の修復"]);
  });

  it("a tab shows its own items, and says so when it has none", async () => {
    setup([overwritten, mention, assigned]);
    await screen.findByText(/上書きされました/);
    await fireEvent.click(screen.getByRole("link", { name: /メンション/ }));
    expect(screen.getAllByRole("listitem")).toHaveLength(1);
    expect(screen.queryByText(/担当者に追加/)).toBeNull();
    await fireEvent.click(screen.getByRole("link", { name: /同期の修復/ }));
    expect(screen.getByText("同期の修復の通知はありません")).toBeTruthy();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
  });

  it("says there is nothing when there is nothing", async () => {
    setup([]);
    expect(await screen.findByText("通知はありません")).toBeTruthy();
    expect(screen.getByRole("button", { name: "すべて既読にする" })).toBeDisabled();
  });

  it("shows the Store badge only with two Stores or more", async () => {
    const one = setup([overwritten]);
    await screen.findByText(/上書きされました/);
    expect(screen.queryByTitle("Store: shared")).toBeNull();
    one.view.unmount();
    setup([overwritten], { stores: ["shared", "backend"] });
    await screen.findByText(/上書きされました/);
    expect(screen.getByTitle("Store: shared")).toBeTruthy();
  });

  it("opening a task marks the item read first", async () => {
    const { api } = setup([mention]);
    const link = await screen.findByRole("link", { name: "REL-4" });
    expect(link.getAttribute("href")).toBe("/tasks/U-REL-4");
    await fireEvent.click(link);
    expect(api.PATCH).toHaveBeenCalledWith("/api/notifications/{id}", {
      params: { path: { id: "N2" } },
      body: { read: true },
    });
    expect(location.pathname).toBe("/tasks/U-REL-4");
  });

  it("the middle button on the ID or the title marks the item read; the browser opens the tab", async () => {
    const { api } = setup([mention, { ...assigned, ulid: "N6", task: task("REL-30") }]);
    await fireEvent(
      await screen.findByRole("link", { name: "REL-4" }),
      new MouseEvent("auxclick", { button: 1, bubbles: true, cancelable: true }),
    );
    await fireEvent(
      screen.getByRole("link", { name: "title of REL-30" }),
      new MouseEvent("auxclick", { button: 1, bubbles: true, cancelable: true }),
    );
    expect(api.PATCH).toHaveBeenCalledTimes(2);
    expect(api.PATCH).toHaveBeenCalledWith("/api/notifications/{id}", {
      params: { path: { id: "N2" } },
      body: { read: true },
    });
    expect(location.pathname).toBe("/inbox");
  });

  it("the title opens the task too, and a read item is not marked again", async () => {
    const { api } = setup([watched]);
    await fireEvent.click(await screen.findByRole("link", { name: "title of REL-13" }));
    expect(location.pathname).toBe("/tasks/U-REL-13");
    expect(api.PATCH).not.toHaveBeenCalled();
  });

  it("an item of a workspace has no link to a task", async () => {
    setup([repair]);
    expect(
      await screen.findByText(
        "ワークスペース release の接頭辞がほかのワークスペースと重なっています",
      ),
    ).toBeTruthy();
    expect(within(screen.getByRole("listitem")).queryAllByRole("link")).toHaveLength(0);
    expect(screen.getByRole("button", { name: "既読にする" })).toBeTruthy();
  });

  it("an unread item is marked read by its button", async () => {
    const { api } = setup([assigned]);
    await fireEvent.click(await screen.findByRole("button", { name: "既読にする" }));
    expect(api.PATCH).toHaveBeenCalledWith("/api/notifications/{id}", {
      params: { path: { id: "N3" } },
      body: { read: true },
    });
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(2));
  });

  it("an overwritten item has the buttons of the canvas, and no plain 既読にする", async () => {
    const { api } = setup([overwritten]);
    const row = await screen.findByRole("listitem");
    expect(within(row).queryByRole("button", { name: "既読にする" })).toBeNull();
    await fireEvent.click(within(row).getByRole("button", { name: "このままにする" }));
    expect(api.PATCH).toHaveBeenCalledWith("/api/notifications/{id}", {
      params: { path: { id: "N1" } },
      body: { read: true },
    });
  });

  it("a read item has no buttons", async () => {
    setup([{ ...overwritten, read: true }]);
    await screen.findByText(/上書きされました/);
    expect(within(rowOf("REL-11")).queryAllByRole("button")).toHaveLength(0);
  });

  it("reverts to my value, and says so", async () => {
    const { api } = setup([overwritten]);
    await fireEvent.click(await screen.findByRole("button", { name: "自分の値に戻す" }));
    await waitFor(() =>
      expect(api.POST).toHaveBeenCalledWith("/api/notifications/{id}/revert", {
        params: { path: { id: "N1" } },
      }),
    );
    await waitFor(() =>
      expect(toasts.list[0]?.message).toBe("REL-11 の points を自分の値に戻しました"),
    );
    expect(toasts.list[0]?.undo).toBeUndefined();
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(2));
  });

  it("shows the words of the server when the revert is refused", async () => {
    const { api } = setup([overwritten]);
    api.POST.mockImplementation(
      () => fail(404, "task.moved", { input: "REL-11", store: "backend", id: "PAY-3" }) as never,
    );
    await fireEvent.click(await screen.findByRole("button", { name: "自分の値に戻す" }));
    await waitFor(() => expect(toasts.list).toHaveLength(1));
    expect(toasts.list[0]?.message).not.toMatch(/戻しました/);
  });

  it("marks everything read in one call without asking which Store", async () => {
    const { api } = setup([overwritten, mention]);
    await fireEvent.click(await screen.findByRole("button", { name: "すべて既読にする" }));
    expect(api.POST).toHaveBeenCalledWith("/api/notifications/read", { body: { all: true } });
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(2));
  });

  it("disables 'mark all' when nothing is unread", async () => {
    setup([watched]);
    await screen.findByText(/変更しました/);
    expect(screen.getByRole("button", { name: "すべて既読にする" })).toBeDisabled();
  });

  it("asks again when the unread count changes, at resync and at tasks.changed", async () => {
    const { api, conn } = setup([mention]);
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(1));
    conn.unread = 1;
    conn.changeSync({ store: "shared" } as never); // notifies the subscribers
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(2));
    conn.emit("resync");
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(3));
    conn.emit("tasks.changed", { store: "shared", ulids: [], removed: [] });
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(4));
  });

  it("shows the words of the server when the list cannot be read", async () => {
    setup([], { getError: new Error("boom") });
    expect(await screen.findByRole("alert")).toHaveTextContent("boom");
  });

  it("shows the command under the list, and no tab of the later phases", async () => {
    setup([mention]);
    await screen.findByText(/メンション/);
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh inbox --all",
    );
    for (const name of ["割り当て", "ウォッチ", "自動化"]) {
      expect(screen.queryByRole("link", { name })).toBeNull();
    }
  });
});
