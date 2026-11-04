import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Schemas } from "../../core/api/client";
import type { StoreTone } from "../../core/stores";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import StoreCard from "./StoreCard.svelte";

type Item = Schemas["StoreItem"];

const item = (over: Partial<Item> = {}): Item => ({
  name: "demo",
  path: "/home/u/stores/demo",
  remotes: ["git@github.com:org/demo.git"],
  account: "work",
  hidden: false,
  default: false,
  default_store: false,
  ...over,
});

type Mount = {
  store?: Item;
  lastVisible?: boolean;
  readOnly?: string | null;
  tone?: StoreTone;
  patch?: () => unknown;
  put?: () => unknown;
  sync?: Map<string, { last_success?: string }>;
};

function mount(m: Mount = {}) {
  const store = m.store ?? item();
  const api = {
    PATCH: vi.fn(() => (m.patch ? m.patch() : ok({ ...store }))),
    PUT: vi.fn(() => (m.put ? m.put() : ok({ name: store.name }))),
  };
  const stores = {
    stores: () => [store],
    tone: () => m.tone ?? "orange",
    readOnly: () => m.readOnly ?? null,
    reload: vi.fn(async () => {}),
    subscribe: () => () => {},
  };
  const onremove = vi.fn();
  const conn = { sync: m.sync ?? new Map(), subscribe: () => () => {}, on: () => () => {} };
  renderWithCore(
    StoreCard,
    { store, lastVisible: m.lastVisible ?? false, onremove },
    { api, stores, conn },
  );
  return { api, stores, onremove };
}

const card = (name = "demo") => screen.getByRole("article", { name });
const dotOf = (name = "demo") =>
  screen
    .getByRole("button", { name: `${name} の色を変える（このマシンだけ）` })
    .querySelector("[data-tone]")
    ?.getAttribute("data-tone");

beforeEach(() => {
  resetToasts();
  history.replaceState(null, "", "/settings/stores");
});
afterEach(() => cleanup());

describe("StoreCard: the summary", () => {
  it("shows the name, the write badge, remotes[0] and the account", () => {
    mount();
    const c = within(card());
    expect(c.getByRole("heading", { name: "demo" })).toBeTruthy();
    expect(c.getByText("書き込み")).toBeTruthy();
    expect(c.getByText("git@github.com:org/demo.git · work")).toBeTruthy();
    expect(c.queryByText("閲覧のみ")).toBeNull();
    expect(c.queryByText("既定の Store")).toBeNull();
  });

  it("says no remote, and leaves the account out when there is none", () => {
    mount({ store: item({ remotes: null, account: undefined }) });
    expect(within(card()).getByText("remote なし")).toBeTruthy();
  });

  it("leaves the account out of the line when there is none", () => {
    mount({ store: item({ account: undefined }) });
    expect(within(card()).getByText("git@github.com:org/demo.git")).toBeTruthy();
  });

  it("shows read-only with the reason in Japanese", () => {
    mount({ readOnly: "the GitHub permission is read", tone: "readonly" });
    const c = within(card());
    expect(c.getByText("閲覧のみ")).toBeTruthy();
    expect(c.getByText("GitHub の権限が読み取りです")).toBeTruthy();
    expect(c.queryByText("書き込み")).toBeNull();
  });

  it("marks the default Store by default_store alone, never by default", () => {
    mount({ store: item({ default: true, default_store: false }) });
    expect(within(card()).queryByText("既定の Store")).toBeNull();
    cleanup();
    mount({ store: item({ default: false, default_store: true }) });
    expect(within(card()).getByText("既定の Store")).toBeTruthy();
  });

  it("says when it was synced last, or that it was not", () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-11-04T10:00:00Z"));
    try {
      mount({ store: item({ last_success: "2026-11-04T09:57:00Z" }) });
      expect(within(card()).getByText("3 分前に同期")).toBeTruthy();
      cleanup();
      mount({ store: item({ last_success: undefined }) });
      expect(within(card()).getByText("未同期")).toBeTruthy();
    } finally {
      vi.useRealTimers();
    }
  });

  it("prefers the last sync the connection knows over the one of the list", () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-11-04T10:00:00Z"));
    try {
      mount({
        store: item({ last_success: "2026-11-04T09:00:00Z" }),
        sync: new Map([["demo", { last_success: "2026-11-04T09:55:00Z" }]]),
      });
      expect(within(card()).getByText("5 分前に同期")).toBeTruthy();
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("StoreCard: the colour", () => {
  it("takes the saved colour first", () => {
    mount({ store: item({ color: "sky" }), tone: "orange" });
    expect(dotOf()).toBe("sky");
  });

  it("takes the colour of its place when none is saved", () => {
    mount({ tone: "purple" });
    expect(dotOf()).toBe("purple");
  });

  it("is grey when read-only, whatever is saved", () => {
    mount({ store: item({ color: "sky" }), readOnly: "the remote cannot be written" });
    expect(dotOf()).toBe("readonly");
  });

  it("opens the palette of seven colours and automatic, with a mark on the saved one", async () => {
    mount({ store: item({ color: "sky" }) });
    await fireEvent.click(
      screen.getByRole("button", { name: "demo の色を変える（このマシンだけ）" }),
    );
    const items = screen.getAllByRole("menuitemcheckbox");
    expect(items.map((i) => i.textContent?.replace(/選択中/, "").trim())).toEqual([
      "ティール",
      "オレンジ",
      "パープル",
      "スカイ",
      "ピンク",
      "オリーブ",
      "ブラウン",
      "自動",
    ]);
    expect(items.filter((i) => i.getAttribute("aria-checked") === "true")).toHaveLength(1);
    expect(
      screen.getByRole("menuitemcheckbox", { name: /スカイ/ }).getAttribute("aria-checked"),
    ).toBe("true");
    // the palette is not a dialog: no command line
    expect(screen.queryByText(/hh store set/)).toBeNull();
  });

  it("marks automatic when no colour is saved", async () => {
    mount();
    await fireEvent.click(
      screen.getByRole("button", { name: "demo の色を変える（このマシンだけ）" }),
    );
    expect(
      screen.getByRole("menuitemcheckbox", { name: /自動/ }).getAttribute("aria-checked"),
    ).toBe("true");
  });

  it("saves the chosen colour with PATCH and reads the Stores again", async () => {
    const { api, stores } = mount();
    await fireEvent.click(
      screen.getByRole("button", { name: "demo の色を変える（このマシンだけ）" }),
    );
    await fireEvent.click(screen.getByRole("menuitemcheckbox", { name: /スカイ/ }));
    await waitFor(() => expect(stores.reload).toHaveBeenCalledTimes(1));
    expect(api.PATCH).toHaveBeenCalledWith("/api/stores/{name}", {
      params: { path: { name: "demo" } },
      body: { color: "sky" },
    });
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("puts automatic back with an empty colour", async () => {
    const { api } = mount({ store: item({ color: "sky" }) });
    await fireEvent.click(
      screen.getByRole("button", { name: "demo の色を変える（このマシンだけ）" }),
    );
    await fireEvent.click(screen.getByRole("menuitemcheckbox", { name: /自動/ }));
    await waitFor(() => expect(api.PATCH).toHaveBeenCalledTimes(1));
    expect(api.PATCH).toHaveBeenCalledWith("/api/stores/{name}", {
      params: { path: { name: "demo" } },
      body: { color: "" },
    });
  });

  it("can be chosen with the keyboard", async () => {
    const { api } = mount();
    await fireEvent.click(
      screen.getByRole("button", { name: "demo の色を変える（このマシンだけ）" }),
    );
    const menu = screen.getByRole("menu");
    await fireEvent.keyDown(menu, { key: "ArrowDown" });
    await fireEvent.keyDown(menu, { key: "Enter" });
    await waitFor(() => expect(api.PATCH).toHaveBeenCalledTimes(1));
  });

  it("keeps the mark on the saved colour of a read-only Store", async () => {
    mount({ store: item({ color: "pink" }), readOnly: "the remote denied the push" });
    await fireEvent.click(
      screen.getByRole("button", { name: "demo の色を変える（このマシンだけ）" }),
    );
    expect(
      screen.getByRole("menuitemcheckbox", { name: /ピンク/ }).getAttribute("aria-checked"),
    ).toBe("true");
  });

  it("says why when the colour cannot be saved", async () => {
    mount({ patch: () => fail(400, "store.bad_color", { value: "x", candidates: ["teal"] }) });
    await fireEvent.click(
      screen.getByRole("button", { name: "demo の色を変える（このマシンだけ）" }),
    );
    await fireEvent.click(screen.getByRole("menuitemcheckbox", { name: /スカイ/ }));
    await waitFor(() => expect(toasts.list).toHaveLength(1));
    expect(toasts.list[0]?.message).toContain("Store の色に使えません");
  });
});

describe("StoreCard: shown", () => {
  it("saves hidden: true when the check is taken off", async () => {
    const { api, stores } = mount();
    const box = within(card()).getByRole("checkbox", { name: "表示" }) as HTMLInputElement;
    expect(box.checked).toBe(true);
    await fireEvent.click(box);
    await waitFor(() => expect(stores.reload).toHaveBeenCalledTimes(1));
    expect(api.PATCH).toHaveBeenCalledWith("/api/stores/{name}", {
      params: { path: { name: "demo" } },
      body: { hidden: true },
    });
  });

  it("saves hidden: false when the check is put on", async () => {
    const { api } = mount({ store: item({ hidden: true }) });
    const box = within(card()).getByRole("checkbox", { name: "表示" }) as HTMLInputElement;
    expect(box.checked).toBe(false);
    await fireEvent.click(box);
    await waitFor(() => expect(api.PATCH).toHaveBeenCalledTimes(1));
    expect(api.PATCH).toHaveBeenCalledWith("/api/stores/{name}", {
      params: { path: { name: "demo" } },
      body: { hidden: false },
    });
  });

  it("shows the server's value before the reload, and after a PATCH that is refused", async () => {
    let release: (v: unknown) => void = () => {};
    const pending = new Promise((r) => {
      release = r;
    });
    const { api, stores } = mount({ patch: () => pending });
    const box = within(card()).getByRole("checkbox", { name: "表示" }) as HTMLInputElement;
    await fireEvent.click(box);
    // not read again yet: the box still shows what the server has
    expect(box.checked).toBe(true);
    expect(stores.reload).not.toHaveBeenCalled();
    release(fail(400, "store.set_args"));
    await waitFor(() => expect(toasts.list).toHaveLength(1));
    expect(api.PATCH).toHaveBeenCalledTimes(1);
    expect(box.checked).toBe(true);
    expect(stores.reload).not.toHaveBeenCalled();
  });

  it("sends a single PATCH for a second click while the first is running", async () => {
    let release: (v: unknown) => void = () => {};
    const pending = new Promise((r) => {
      release = r;
    });
    const { api, stores } = mount({ patch: () => pending });
    const box = within(card()).getByRole("checkbox", { name: "表示" });
    await fireEvent.click(box);
    await fireEvent.click(box);
    expect(api.PATCH).toHaveBeenCalledTimes(1);
    release(ok({}));
    await waitFor(() => expect(stores.reload).toHaveBeenCalledTimes(1));
    expect(api.PATCH).toHaveBeenCalledTimes(1);
  });

  it("disables the check of the last shown Store, with the reason", async () => {
    const { api } = mount({ lastVisible: true });
    const box = within(card()).getByRole("checkbox", { name: "表示" }) as HTMLInputElement;
    expect(box.disabled).toBe(true);
    expect(within(card()).getByText("すべての Store を非表示にはできません")).toBeTruthy();
    await fireEvent.click(box);
    expect(api.PATCH).not.toHaveBeenCalled();
  });

  it("leaves a hidden Store's check on even when only one is shown", () => {
    mount({ store: item({ hidden: true }), lastVisible: false });
    const box = within(card()).getByRole("checkbox", { name: "表示" }) as HTMLInputElement;
    expect(box.disabled).toBe(false);
  });
});

describe("StoreCard: the details", () => {
  const toggle = (name = "demo") => screen.getByRole("button", { name: `詳細を開く: ${name}` });

  it("keeps the details closed until the toggle opens them", async () => {
    mount();
    expect(within(card()).queryByText("/home/u/stores/demo")).toBeNull();
    await fireEvent.click(toggle());
    expect(within(card()).getByText("/home/u/stores/demo")).toBeTruthy();
    const close = screen.getByRole("button", { name: "閉じる: demo" });
    expect(close.getAttribute("aria-expanded")).toBe("true");
    await fireEvent.click(close);
    expect(within(card()).queryByText("/home/u/stores/demo")).toBeNull();
  });

  it("lists the remotes with the mirrors marked, the account and the path", async () => {
    mount({ store: item({ remotes: ["git@a:o/r.git", "git@b:o/r.git", "git@c:o/r.git"] }) });
    await fireEvent.click(toggle());
    const c = within(card());
    expect(c.getAllByText("ミラー")).toHaveLength(2);
    const rows = c
      .getAllByRole("listitem")
      .map((li) => li.textContent?.replace(/\s+/g, " ").trim());
    expect(rows).toEqual(["git@a:o/r.git", "git@b:o/r.git ミラー", "git@c:o/r.git ミラー"]);
    expect(c.getByText("work", { selector: "dd" })).toBeTruthy();
  });

  it("says none for the account and the remotes that are not there", async () => {
    mount({ store: item({ remotes: null, account: undefined }) });
    await fireEvent.click(toggle());
    expect(within(card()).getAllByText("なし", { selector: "dd" })).toHaveLength(2);
  });

  it("makes the Store the default with PUT, and says so", async () => {
    const { api, stores } = mount();
    await fireEvent.click(toggle());
    await fireEvent.click(within(card()).getByRole("button", { name: "既定にする" }));
    await waitFor(() => expect(stores.reload).toHaveBeenCalledTimes(1));
    expect(api.PUT).toHaveBeenCalledWith("/api/stores/default", { body: { name: "demo" } });
    expect(toasts.list.map((t) => t.message)).toEqual(["既定の Store を demo にしました"]);
  });

  it("does not offer the default to the default Store", async () => {
    mount({ store: item({ default_store: true }) });
    await fireEvent.click(toggle());
    expect(within(card()).queryByRole("button", { name: "既定にする" })).toBeNull();
  });

  it("asks to unmount from the details", async () => {
    const { onremove } = mount();
    await fireEvent.click(toggle());
    await fireEvent.click(within(card()).getByRole("button", { name: "マウントを解除…" }));
    expect(onremove).toHaveBeenCalledTimes(1);
  });

  it("has no merge driver, no add remote, no open folder, no format and no members", async () => {
    mount();
    await fireEvent.click(toggle());
    for (const t of [/merge driver/i, /remote を追加/, /フォルダを開く/, /形式/, /メンバー/]) {
      expect(screen.queryByText(t)).toBeNull();
    }
    expect(screen.queryByText(/新しく見つかった/)).toBeNull();
    expect(screen.queryByText(/タスク/)).toBeNull();
  });
});
