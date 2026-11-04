import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { InboxTab, Notification } from "../../core/notifications";
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
  opts: {
    stores?: string[];
    unreadStores?: boolean;
    getError?: Error;
    tab?: InboxTab;
    readOnly?: Record<string, string>;
  } = {},
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
    { tab: opts.tab ?? "all" },
    {
      api,
      conn,
      stores: {
        stores: () => names.map((name) => ({ name })),
        readOnly: (s: string) => opts.readOnly?.[s] ?? null,
        tone: () => "purple",
        displayId: (_s: string, id: string) => id,
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

  it("each tab is a link to its own address, and the tab of the address is the current one", async () => {
    setup([overwritten, mention], { tab: "mention" });
    await screen.findByText(/メンションしました/);
    const links = within(screen.getByRole("navigation", { name: "通知の種類" })).getAllByRole(
      "link",
    );
    expect(links.map((l) => l.getAttribute("href"))).toEqual([
      "/inbox",
      "/inbox?tab=overwritten",
      "/inbox?tab=mention",
      "/inbox?tab=repair",
    ]);
    expect(links.map((l) => l.getAttribute("aria-current"))).toEqual([null, null, "page", null]);
    // it opens in the same tab: the address changes, and App gives the page the tab
    await fireEvent.click(links[1] as HTMLElement);
    expect(location.pathname + location.search).toBe("/inbox?tab=overwritten");
  });

  it("a tab shows its own items, and says so when it has none", async () => {
    const { view } = setup([overwritten, mention, assigned]);
    await screen.findByText(/上書きされました/);
    await view.rerender({ tab: "mention" });
    expect(screen.getAllByRole("listitem")).toHaveLength(1);
    expect(screen.queryByText(/担当者に追加/)).toBeNull();
    await view.rerender({ tab: "repair" });
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

  it("cannot revert an overwrite in a read-only Store, and says why; the other items are not touched", async () => {
    const { api } = setup([overwritten, mention], {
      readOnly: { shared: "the remote cannot be written" },
    });
    const b = await screen.findByRole("button", { name: "自分の値に戻す" });
    expect(b).toBeDisabled();
    expect(b).toHaveAttribute("title", "この Store は閲覧のみです（remote に書き込めません）");
    b.click();
    expect(api.POST).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "このままにする" })).toBeEnabled();
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

  describe("focus", () => {
    // A browser may drop the focus of a button that is disabled or gone; jsdom keeps it, so the test does.
    const drop = () => (document.activeElement as HTMLElement | null)?.blur();

    it("goes to the row when the buttons of the row are gone", async () => {
      const { api, set } = setup([assigned]);
      const button = await screen.findByRole("button", { name: "既読にする" });
      button.focus();
      set([{ ...assigned, read: true }]);
      api.PATCH.mockImplementation(() => {
        drop();
        return Promise.resolve({ data: undefined, response: new Response(null, { status: 204 }) });
      });
      await fireEvent.click(button);
      await waitFor(() => expect(rowOf("REL-27")).toHaveFocus());
      expect(within(rowOf("REL-27")).queryAllByRole("button")).toHaveLength(0);
    });

    it("goes to the row after the revert too", async () => {
      const { set } = setup([overwritten]);
      const button = await screen.findByRole("button", { name: "自分の値に戻す" });
      button.focus();
      set([{ ...overwritten, read: true }]);
      await fireEvent.click(button);
      drop();
      await waitFor(() => expect(rowOf("REL-11")).toHaveFocus());
    });

    it("goes to the heading when 'mark all' is disabled by it", async () => {
      const { api, set } = setup([mention, assigned]);
      const button = await screen.findByRole("button", { name: "すべて既読にする" });
      button.focus();
      set([
        { ...mention, read: true },
        { ...assigned, read: true },
      ]);
      api.POST.mockImplementation(() => {
        drop();
        return ok({ read: [] });
      });
      await fireEvent.click(button);
      await waitFor(() =>
        expect(screen.getByRole("heading", { level: 1, name: "通知" })).toHaveFocus(),
      );
      expect(button).toBeDisabled();
    });

    it("stays where the user moved it meanwhile", async () => {
      const { api, set } = setup([assigned]);
      const button = await screen.findByRole("button", { name: "既読にする" });
      set([{ ...assigned, read: true }]);
      const tab = screen.getAllByRole("link", { name: /すべて/ })[0] as HTMLElement;
      api.PATCH.mockImplementation(() => {
        tab.focus();
        return Promise.resolve({ data: undefined, response: new Response(null, { status: 204 }) });
      });
      await fireEvent.click(button);
      await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(2));
      await new Promise((r) => setTimeout(r, 10));
      expect(tab).toHaveFocus();
    });
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

  it("shows the answer of the latest request when an older one arrives last", async () => {
    const { api, conn } = setup([]);
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(1));
    const pending: ((v: unknown) => void)[] = [];
    api.GET.mockImplementation(() => new Promise((r) => pending.push(r)) as never);
    conn.emit("resync");
    conn.emit("resync");
    await waitFor(() => expect(pending).toHaveLength(2));
    const answer = (x: Notification) => ok([x]);
    pending[1]?.(await answer(n({ ulid: "NB", task: task("REL-2") })));
    await screen.findByText("title of REL-2");
    pending[0]?.(await answer(n({ ulid: "NA", task: task("REL-1") })));
    await new Promise((r) => setTimeout(r, 10));
    expect(screen.getByText("title of REL-2")).toBeTruthy();
    expect(screen.queryByText("title of REL-1")).toBeNull();
  });

  it("stops listening to the connection when it is gone", async () => {
    const { api, conn, view } = setup([mention]);
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(1));
    view.unmount();
    conn.emit("resync");
    conn.emit("tasks.changed", { store: "shared", ulids: [], removed: [] });
    await new Promise((r) => setTimeout(r, 10));
    expect(api.GET).toHaveBeenCalledTimes(1);
  });

  it("keeps the items it has when a reload fails, with the error above them", async () => {
    const { api, conn } = setup([mention]);
    await screen.findByText("title of REL-4");
    api.GET.mockImplementation(() => Promise.reject(new Error("boom")) as never);
    conn.emit("resync");
    expect(await screen.findByRole("alert")).toHaveTextContent("boom");
    expect(screen.getByText("title of REL-4")).toBeTruthy();
  });

  it("does not read an item again while its request is on the way", async () => {
    const { api } = setup([mention]);
    let release: () => void = () => {};
    api.PATCH.mockImplementation(
      () =>
        new Promise((r) => {
          release = () => r({ data: undefined, response: new Response(null, { status: 204 }) });
        }) as never,
    );
    const link = await screen.findByRole("link", { name: "REL-4" });
    await fireEvent.click(link);
    await fireEvent.click(link);
    expect(api.PATCH).toHaveBeenCalledTimes(1);
    release();
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
