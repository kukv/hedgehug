import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Log } from "../../core/activity";
import type { WorkspaceEntry } from "../../core/stores";
import { FakeConn } from "../../core/testing";
import { fail, ok } from "../detail/testkit";
import type { ActivitySync } from "../router";
import { renderWithCore } from "../test-utils";
import ActivityPage from "./ActivityPage.svelte";

function log(over: Partial<Log> & { hash: string }): Log {
  return {
    author: { name: "koki", email: "koki@example.com" },
    date: new Date().toISOString(),
    id: "REL-1",
    op: "set",
    store: "shared",
    subject: "set REL-1",
    task: "U-REL-1",
    trailers: [{ key: "Hedgehug-Op", value: "set" }],
    ...over,
  };
}

const ws = (store: string, ulid: string, name: string, prefix: string): WorkspaceEntry => ({
  store,
  tone: store === "shared" ? "purple" : "orange",
  ulid,
  name,
  prefix,
  tasks: 0,
  deleted: 0,
  prefixSet: "",
  projects: [],
});

const spaces = [
  ws("shared", "W1", "release", "REL"),
  ws("backend", "W2", "payments", "PAY"),
  ws("backend", "W3", "release", "REL2"),
];

const added = log({ hash: "aaaaaaa1111", op: "add", subject: "add REL-1", id: "REL-1" });
const merged = log({
  hash: "bbbbbbb2222",
  op: "merge",
  subject: "merge tanaka",
  author: { name: "sato", email: "s@example.com" },
  id: null,
  task: null,
  trailers: [{ key: "Hedgehug-Op", value: "merge" }],
});

type Opts = {
  stores?: string[];
  workspaces?: string;
  sync?: ActivitySync;
  getError?: Error;
  hidden?: boolean; // every Store is hidden
};

function setup(items: Log[], opts: Opts = {}) {
  let current = items;
  const conn = new FakeConn();
  const api = {
    GET: vi.fn(() => (opts.getError ? Promise.reject(opts.getError) : ok(current))),
  };
  const names = opts.stores ?? ["shared"];
  const view = renderWithCore(
    ActivityPage,
    { workspaces: opts.workspaces ? opts.workspaces.split(",") : [], sync: opts.sync ?? "" },
    {
      api,
      conn,
      stores: {
        stores: () => names.map((name) => ({ name })),
        readOnly: () => null,
        hidden: () => opts.hidden ?? false,
        tone: () => "purple",
        displayId: (_s: string, id: string) => id,
        workspaces: () => spaces,
        workspace: () => undefined,
        loaded: () => true,
        subscribe: () => () => {},
      },
    },
  );
  return {
    api,
    conn,
    view,
    set(next: Log[]) {
      current = next;
    },
  };
}

const lastQuery = (api: { GET: ReturnType<typeof vi.fn> }) => {
  const arg = api.GET.mock.lastCall?.[1] as { params: { query: Record<string, unknown> } };
  return arg.params.query;
};

beforeEach(() => history.replaceState(null, "", "/activity"));
afterEach(() => cleanup());

describe("ActivityPage", () => {
  it("asks once for every Store, 100 commits, with no filter", async () => {
    const { api } = setup([added]);
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(1));
    expect(api.GET).toHaveBeenCalledWith("/api/activity", { params: { query: { limit: 100 } } });
    expect(screen.getByRole("heading", { level: 1, name: "アクティビティ" })).toBeTruthy();
    expect(screen.getByText("各 Store の git log · 誰がいつ何を変えたか")).toBeTruthy();
  });

  it("passes the filters of the address to the server", async () => {
    const { api } = setup([], { workspaces: "W1,W2", sync: "only" });
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(1));
    expect(lastQuery(api)).toEqual({ limit: 100, workspace: "W1,W2", sync: "only" });
  });

  it("shows a commit: the time, the author, the op, the subject, the ID and the trailers", async () => {
    setup([added]);
    const row = await screen.findByRole("listitem");
    expect(within(row).getByText("koki")).toBeTruthy();
    expect(within(row).getByText("作成")).toBeTruthy();
    expect(within(row).getByText("add REL-1")).toBeTruthy();
    expect(within(row).getByRole("link", { name: "REL-1" }).getAttribute("href")).toBe(
      "/tasks/U-REL-1",
    );
    expect(within(row).getByText("aaaaaaa · Hedgehug-Op: set")).toBeTruthy();
    expect(row.querySelector("time")?.textContent).toMatch(/^\d\d:\d\d$/);
    // under the head of the day
    expect(screen.getByRole("heading", { level: 2 }).textContent).toMatch(
      /^今日 · \d\d\/\d\d（.）$/,
    );
  });

  it("shows a merge as the sync of its author, with no link", async () => {
    setup([merged]);
    const row = await screen.findByRole("listitem");
    expect(within(row).getByText("同期（sato）")).toBeTruthy();
    expect(within(row).getByText("同期のマージ")).toBeTruthy();
    expect(within(row).queryAllByRole("link")).toHaveLength(0);
  });

  it("a task that the index no longer has is a link by its ULID", async () => {
    setup([log({ hash: "ccccccc3333", id: null, task: "U-GONE" })]);
    const link = await screen.findByRole("link", { name: "U-GONE" });
    expect(link.getAttribute("href")).toBe("/tasks/U-GONE");
  });

  it("shows the Store badge only with two Stores or more", async () => {
    const one = setup([added]);
    await screen.findByRole("listitem");
    expect(within(screen.getByRole("listitem")).queryByTitle("Store: shared")).toBeNull();
    one.view.unmount();
    setup([added], { stores: ["shared", "backend"] });
    await screen.findByRole("listitem");
    expect(within(screen.getByRole("listitem")).getByTitle("Store: shared")).toBeTruthy();
  });

  it("does not offer automations; it has all, people and syncs", async () => {
    setup([added]);
    await screen.findByRole("listitem");
    const group = screen.getByRole("group", { name: "変更の種類" });
    expect(
      within(group)
        .getAllByRole("button")
        .map((b) => b.textContent?.trim()),
    ).toEqual(["すべて", "人", "同期"]);
    expect(within(group).getByRole("button", { name: "すべて" }).getAttribute("aria-pressed")).toBe(
      "true",
    );
    expect(screen.queryByText("自動化")).toBeNull();
  });

  it("people and syncs go to the address, and ask again", async () => {
    const { api } = setup([added]);
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(1));
    await fireEvent.click(screen.getByRole("button", { name: "同期" }));
    expect(location.pathname + location.search).toBe("/activity?sync=only");
    await fireEvent.click(screen.getByRole("button", { name: "人" }));
    expect(location.search).toBe("?sync=exclude");
    await fireEvent.click(screen.getByRole("button", { name: "すべて" }));
    expect(location.pathname + location.search).toBe("/activity");
  });

  it("the workspaces go to the address as ULIDs", async () => {
    setup([added]);
    await screen.findByRole("listitem");
    await fireEvent.click(screen.getByRole("button", { name: /ワークスペース$/ }));
    await fireEvent.click(screen.getByRole("checkbox", { name: /payments/ }));
    expect(location.pathname + location.search).toBe("/activity?workspace=W2");
  });

  it("does not leave out a hidden Store: its commits and its workspaces are there (細部 93)", async () => {
    setup([added], { hidden: true });
    await screen.findByRole("listitem");
    await fireEvent.click(screen.getByRole("button", { name: /ワークスペース$/ }));
    expect(screen.getByRole("checkbox", { name: /payments/ })).toBeTruthy();
  });

  it("says there is nothing when there is nothing", async () => {
    setup([]);
    expect(await screen.findByText("アクティビティはありません")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "さらに読み込む" })).toBeNull();
  });

  it("offers more only while a full page came, and asks for 100 more", async () => {
    const full = Array.from({ length: 100 }, (_, i) =>
      log({ hash: `h${String(i).padStart(6, "0")}` }),
    );
    const { api, set } = setup(full);
    const more = await screen.findByRole("button", { name: "さらに読み込む" });
    set([...full, log({ hash: "hx00000" })]);
    await fireEvent.click(more);
    await waitFor(() => expect(lastQuery(api).limit).toBe(200));
    // 101 came for 200: that is all there is
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "さらに読み込む" })).toBeNull(),
    );
    expect(screen.getAllByRole("listitem")).toHaveLength(101);
  });

  it("stops offering more at 1000, the most the server gives", async () => {
    const many = (n: number) =>
      Array.from({ length: n }, (_, i) => log({ hash: `h${String(i).padStart(6, "0")}` }));
    const { api, set } = setup(many(100));
    for (let limit = 200; limit <= 1000; limit += 100) {
      set(many(limit));
      await fireEvent.click(await screen.findByRole("button", { name: "さらに読み込む" }));
      await waitFor(() => expect(lastQuery(api).limit).toBe(limit));
      await waitFor(() => expect(screen.getAllByRole("listitem")).toHaveLength(limit));
    }
    expect(screen.queryByRole("button", { name: "さらに読み込む" })).toBeNull();
  });

  it("disables the button while a request is on its way", async () => {
    const full = Array.from({ length: 100 }, (_, i) =>
      log({ hash: `h${String(i).padStart(6, "0")}` }),
    );
    const { api } = setup(full);
    const more = await screen.findByRole("button", { name: "さらに読み込む" });
    expect(more).toBeEnabled();
    let release: (v: unknown) => void = () => {};
    api.GET.mockImplementation(() => new Promise((r) => (release = r)) as never);
    await fireEvent.click(more);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "さらに読み込む" })).toBeDisabled(),
    );
    release(
      await ok(
        Array.from({ length: 200 }, (_, i) => log({ hash: `g${String(i).padStart(6, "0")}` })),
      ),
    );
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "さらに読み込む" })).toBeEnabled(),
    );
  });

  describe("focus of the button to load more", () => {
    const many = (n: number) =>
      Array.from({ length: n }, (_, i) => log({ hash: `h${String(i).padStart(6, "0")}` }));
    // A browser may drop the focus of a button that is disabled; jsdom keeps it, so the test does.
    const drop = () => (document.activeElement as HTMLElement | null)?.blur();

    it("comes back to the button when it is still there", async () => {
      const { api, set } = setup(many(100));
      const more = await screen.findByRole("button", { name: "さらに読み込む" });
      more.focus();
      set(many(200));
      api.GET.mockImplementation(() => {
        drop();
        return ok(many(200)) as never;
      });
      await fireEvent.click(more);
      await waitFor(() => expect(screen.getAllByRole("listitem")).toHaveLength(200));
      expect(screen.getByRole("button", { name: "さらに読み込む" })).toHaveFocus();
    });

    it("goes to the first row that came when the button is gone", async () => {
      const { api } = setup(many(100));
      const more = await screen.findByRole("button", { name: "さらに読み込む" });
      more.focus();
      api.GET.mockImplementation(() => {
        drop();
        return ok(many(150)) as never;
      });
      await fireEvent.click(more);
      await waitFor(() =>
        expect(screen.queryByRole("button", { name: "さらに読み込む" })).toBeNull(),
      );
      expect(screen.getAllByRole("listitem")[100]).toHaveFocus();
    });

    it("does not move the focus for a load that a change of the filter took over", async () => {
      const { api } = setup(many(100));
      const more = await screen.findByRole("button", { name: "さらに読み込む" });
      const pending: ((v: unknown) => void)[] = [];
      api.GET.mockImplementation(() => new Promise((r) => pending.push(r)) as never);
      await fireEvent.click(more);
      await waitFor(() => expect(pending).toHaveLength(1));
      // the filter changes: the limit goes back to 100 and the page asks again
      await fireEvent.click(screen.getByRole("button", { name: "同期" }));
      await waitFor(() => expect(pending).toHaveLength(2));
      drop();
      pending[0]?.(await ok(many(200)));
      pending[1]?.(await ok(many(100)));
      await waitFor(() =>
        expect(screen.getByRole("button", { name: "さらに読み込む" })).toBeEnabled(),
      );
      await new Promise((r) => setTimeout(r, 10));
      // the load was not the one of the button: the focus is left where it is
      expect(document.body).toHaveFocus();
    });

    it("goes to the heading when nothing new came and the button is gone", async () => {
      const { api } = setup(many(100));
      const more = await screen.findByRole("button", { name: "さらに読み込む" });
      more.focus();
      api.GET.mockImplementation(() => {
        drop();
        return ok(many(100).slice(0, 99)) as never;
      });
      await fireEvent.click(more);
      await waitFor(() =>
        expect(screen.queryByRole("button", { name: "さらに読み込む" })).toBeNull(),
      );
      expect(screen.getByRole("heading", { level: 1, name: "アクティビティ" })).toHaveFocus();
    });
  });

  it("shows two commits of one day that arrive apart, and one with a date that is none", async () => {
    setup([
      log({ hash: "a000000", date: "2026-11-04T11:00:00" }),
      log({ hash: "b000000", date: "2026-11-03T11:00:00" }),
      log({ hash: "c000000", date: "2026-11-04T09:00:00" }),
      log({ hash: "d000000", date: "nope" }),
    ]);
    await screen.findAllByRole("listitem");
    expect(screen.getAllByRole("listitem")).toHaveLength(4);
    expect(screen.getAllByRole("heading", { level: 2 })).toHaveLength(3);
    expect(screen.getByRole("heading", { level: 2, name: "日時不明" })).toBeTruthy();
    expect(screen.getByText("--:--")).toBeTruthy();
    // the id of a head has no space
    expect(document.querySelector('[id^="day-"][id*=" "]')).toBeNull();
  });

  it("goes back to 100 when a filter changes", async () => {
    const full = Array.from({ length: 100 }, (_, i) =>
      log({ hash: `h${String(i).padStart(6, "0")}` }),
    );
    const { api } = setup(full);
    await fireEvent.click(await screen.findByRole("button", { name: "さらに読み込む" }));
    await waitFor(() => expect(lastQuery(api).limit).toBe(200));
    await fireEvent.click(screen.getByRole("button", { name: "同期" }));
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(3));
    // the page is given its filters by the address, not by itself: the limit is the one thing it resets
    expect(lastQuery(api).limit).toBe(100);
  });

  it("the command under it is hh log, with the names of the workspaces chosen", async () => {
    setup([added]);
    expect(await screen.findByRole("button", { name: "コマンドをコピー: hh log" })).toBeTruthy();
    cleanup();
    setup([added], { workspaces: "W2" });
    expect(
      await screen.findByRole("button", { name: "コマンドをコピー: hh log --workspace payments" }),
    ).toBeTruthy();
  });

  it("writes a workspace whose name another Store has also by its ULID", async () => {
    setup([added], { workspaces: "W3,W2" });
    expect(
      await screen.findByRole("button", {
        name: "コマンドをコピー: hh log --workspace W3,payments",
      }),
    ).toBeTruthy();
  });

  it("asks again at tasks.changed and at resync", async () => {
    const { api, conn } = setup([added]);
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(1));
    conn.emit("tasks.changed", { store: "shared", ulids: [], removed: [] });
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(2));
    conn.emit("resync");
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(3));
  });

  it("stops asking when the screen is gone", async () => {
    const { api, conn, view } = setup([added]);
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(1));
    view.unmount();
    conn.emit("tasks.changed", { store: "shared", ulids: [], removed: [] });
    conn.emit("resync");
    await new Promise((r) => setTimeout(r, 10));
    expect(api.GET).toHaveBeenCalledTimes(1);
  });

  it("shows the answer of the latest request when an older one arrives last", async () => {
    const { api, conn } = setup([]);
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(1));
    const pending: ((v: unknown) => void)[] = [];
    api.GET.mockImplementation(() => new Promise((r) => pending.push(r)) as never);
    conn.emit("resync");
    conn.emit("resync");
    await waitFor(() => expect(pending).toHaveLength(2));
    pending[1]?.(await ok([log({ hash: "new0000", subject: "the new one" })]));
    await screen.findByText("the new one");
    pending[0]?.(await ok([log({ hash: "old0000", subject: "the old one" })]));
    await new Promise((r) => setTimeout(r, 10));
    expect(screen.getByText("the new one")).toBeTruthy();
    expect(screen.queryByText("the old one")).toBeNull();
  });

  it("keeps the list and shows the error above it when a new request fails", async () => {
    const { api, conn } = setup([added]);
    await screen.findByText("add REL-1");
    api.GET.mockImplementation((() => fail(500, "internal")) as never);
    conn.emit("resync");
    await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
    expect(screen.getByText("add REL-1")).toBeTruthy();
  });

  it("shows the error when the first request fails", async () => {
    setup([], { getError: new Error("boom") });
    await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
    expect(screen.queryByText("アクティビティはありません")).toBeNull();
  });
});
