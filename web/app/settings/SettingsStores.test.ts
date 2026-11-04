import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, type Schemas } from "../../core/api/client";
import { ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import { resetToasts } from "../toast.svelte";
import SettingsStores from "./SettingsStores.svelte";

type Item = Schemas["StoreItem"];

const item = (name: string, over: Partial<Item> = {}): Item => ({
  name,
  path: `/home/u/stores/${name}`,
  remotes: [`git@github.com:org/${name}.git`],
  hidden: false,
  default: false,
  default_store: false,
  ...over,
});

type Mount = { list?: Item[]; loaded?: boolean; error?: ApiError | null };

function mount(m: Mount = {}) {
  const state = { list: m.list ?? [item("a")], loaded: m.loaded ?? true, error: m.error ?? null };
  const stores = {
    stores: () => state.list,
    tone: () => "teal",
    readOnly: () => null,
    loaded: () => state.loaded,
    error: () => state.error,
    reload: vi.fn(async () => {}),
    subscribe: () => () => {},
  };
  const accounts = {
    accounts: () => [],
    loaded: () => true,
    error: () => null,
    load: vi.fn(async () => {}),
    reload: vi.fn(async () => {}),
    subscribe: () => () => {},
  };
  const api = {
    PATCH: vi.fn(() => ok({})),
    PUT: vi.fn(() => ok({})),
    POST: vi.fn(() => ok({})),
    DELETE: vi.fn(() => ok({ name: "b", path: "/p", deleted: true })),
  };
  const conn = { sync: new Map(), subscribe: () => () => {}, on: () => () => {} };
  renderWithCore(SettingsStores, {}, { stores, accounts, api, conn });
  return { stores, api, accounts };
}

const card = (name: string) => screen.getByRole("article", { name });

beforeEach(() => {
  resetToasts();
  history.replaceState(null, "", "/settings/stores");
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
});
afterEach(() => cleanup());

describe("SettingsStores: the list", () => {
  it("keeps the heading and the lead", () => {
    mount();
    expect(screen.getByRole("heading", { level: 1, name: "マウント中の Store" })).toBeTruthy();
    expect(screen.getByText(/このマシンで扱う Store です/)).toBeTruthy();
  });

  it("shows a card for each Store, in the order of the list", () => {
    mount({ list: [item("zeta"), item("alpha"), item("mid")] });
    expect(screen.getAllByRole("article").map((a) => a.getAttribute("aria-label"))).toEqual([
      "zeta",
      "alpha",
      "mid",
    ]);
  });

  it("shows loading while the Stores are read", () => {
    mount({ list: [], loaded: false });
    expect(screen.getByText("読み込み中…")).toBeTruthy();
    expect(screen.queryByRole("article")).toBeNull();
  });

  it("shows why they cannot be read, with a button to read again", async () => {
    const { stores } = mount({
      list: [],
      loaded: false,
      error: new ApiError({ status: 0, type: "network", detail: "offline" }),
    });
    expect(screen.getByRole("alert")).toBeTruthy();
    expect(screen.queryByText("読み込み中…")).toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: "もう一度読む" }));
    expect(stores.reload).toHaveBeenCalledTimes(1);
  });

  it("says there is no Store when the list is empty", () => {
    mount({ list: [] });
    expect(screen.getByText("マウント中の Store はありません")).toBeTruthy();
  });

  it("marks the default Store by default_store, not by default", () => {
    mount({
      list: [item("a", { default: true }), item("b", { default_store: true })],
    });
    expect(within(card("a")).queryByText("既定の Store")).toBeNull();
    expect(within(card("b")).getByText("既定の Store")).toBeTruthy();
  });

  it("disables the check of the only shown Store, and not the others", () => {
    mount({ list: [item("a"), item("b", { hidden: true })] });
    const a = within(card("a")).getByRole("checkbox", { name: "表示" }) as HTMLInputElement;
    const b = within(card("b")).getByRole("checkbox", { name: "表示" }) as HTMLInputElement;
    expect(a.disabled).toBe(true);
    expect(b.disabled).toBe(false);
  });

  it("leaves every check on when two Stores are shown", () => {
    mount({ list: [item("a"), item("b")] });
    for (const n of ["a", "b"]) {
      expect(
        (within(card(n)).getByRole("checkbox", { name: "表示" }) as HTMLInputElement).disabled,
      ).toBe(false);
    }
  });

  it("does not show what is not made: found Stores, task counts, the GitHub search", () => {
    mount({ list: [item("a"), item("b")] });
    expect(screen.queryByText(/新しく見つかった/)).toBeNull();
    expect(screen.queryByText(/タスク/)).toBeNull();
  });
});

describe("SettingsStores: the heading buttons", () => {
  it("opens the dialog that adds by URL", async () => {
    mount();
    await fireEvent.click(screen.getByRole("button", { name: "URL で追加" }));
    expect(screen.getByRole("dialog", { name: "URL で追加" })).toBeTruthy();
  });

  it("opens the dialog that looks for Stores", async () => {
    mount();
    await fireEvent.click(screen.getByRole("button", { name: "Store を探す" }));
    const dialog = screen.getByRole("dialog", { name: "Store を探す" });
    expect(within(dialog).getByLabelText("フォルダ")).toBeTruthy();
  });

  it("opens CreateStore", async () => {
    mount();
    await fireEvent.click(screen.getByRole("button", { name: "Store を作る" }));
    expect(screen.getByRole("dialog", { name: "Store を作成" })).toBeTruthy();
  });
});

describe("SettingsStores: unmounting", () => {
  it("opens the confirmation for the Store whose details were opened, and unmounts it", async () => {
    const { api, stores } = mount({ list: [item("a"), item("b")] });
    await fireEvent.click(within(card("b")).getByRole("button", { name: "詳細を開く: b" }));
    await fireEvent.click(within(card("b")).getByRole("button", { name: "マウントを解除…" }));
    const dialog = screen.getByRole("dialog", { name: "b のマウントを解除" });
    await fireEvent.click(within(dialog).getByRole("button", { name: "解除する" }));
    await waitFor(() => expect(api.DELETE).toHaveBeenCalledTimes(1));
    expect(api.DELETE).toHaveBeenCalledWith("/api/stores/{name}", {
      params: { path: { name: "b" }, query: { keep: false } },
    });
    await waitFor(() => expect(stores.reload).toHaveBeenCalled());
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });
});
