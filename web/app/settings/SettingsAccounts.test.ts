import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Account } from "../../core/accounts";
import { ApiError } from "../../core/api/client";
import { fail, ok } from "../detail/testkit";
import { emitter, renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import SettingsAccounts from "./SettingsAccounts.svelte";

const acct = (name: string, over: Partial<Account> = {}): Account => ({
  name,
  id: `github.com/${name}`,
  state: "ok",
  login: name,
  token: "keychain",
  ...over,
});

type Mount = {
  list?: Account[];
  loaded?: boolean;
  error?: ApiError | null;
  stores?: { name: string; account?: string }[];
  del?: () => ReturnType<typeof ok> | ReturnType<typeof fail>;
};

function mount(m: Mount = {}) {
  const e = emitter();
  const state = { list: m.list ?? [], loaded: m.loaded ?? true, error: m.error ?? null };
  const accounts = {
    accounts: () => state.list,
    loaded: () => state.loaded,
    error: () => state.error,
    load: vi.fn(async () => {}),
    reload: vi.fn(async () => {}),
    subscribe: e.subscribe,
  };
  const stores = {
    stores: () => m.stores ?? [],
    reload: vi.fn(async () => {}),
    subscribe: () => () => {},
  };
  const api = {
    DELETE: vi.fn(() => (m.del ? m.del() : ok(undefined))),
    POST: vi.fn(),
    PUT: vi.fn(),
  };
  const conn = { on: () => () => {} };
  renderWithCore(SettingsAccounts, {}, { accounts, stores, api, conn });
  return { accounts, stores, api, state, emit: e.emit };
}

const card = (name: string) => screen.getByRole("article", { name });

beforeEach(() => {
  resetToasts();
  history.replaceState(null, "", "/settings/accounts");
});
afterEach(() => cleanup());

describe("SettingsAccounts: the list", () => {
  it("keeps the heading and the lead, and reads the accounts when it opens", () => {
    const { accounts } = mount({ list: [acct("a")] });
    expect(screen.getByRole("heading", { level: 1, name: "アカウントと認証" })).toBeTruthy();
    expect(screen.getByText(/ここに登録した全アカウントを「自分」として扱います/)).toBeTruthy();
    expect(accounts.load).toHaveBeenCalledTimes(1);
  });

  it("shows loading while the accounts are read", () => {
    mount({ loaded: false });
    expect(screen.getByText("読み込み中…")).toBeTruthy();
    expect(screen.queryByRole("article")).toBeNull();
  });

  it("shows why they cannot be read, with a button to read again", async () => {
    const { accounts } = mount({
      loaded: false,
      error: new ApiError({ status: 502, type: "github.rate_limit", args: { account: "a" } }),
    });
    expect(screen.getByRole("alert").textContent).toContain("アカウントを読めません");
    expect(screen.queryByText("読み込み中…")).toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: "もう一度読む" }));
    expect(accounts.reload).toHaveBeenCalledTimes(1);
  });

  it("keeps the list it had when a later reading fails", () => {
    mount({
      list: [acct("a")],
      error: new ApiError({ status: 0, type: "network", detail: "offline" }),
    });
    expect(screen.getByRole("alert")).toBeTruthy();
    expect(card("a")).toBeTruthy();
  });

  it("says there are no accounts, with the button to add one", async () => {
    mount({ list: [] });
    expect(screen.getByText("アカウントがありません")).toBeTruthy();
    // one button, not two
    await fireEvent.click(screen.getByRole("button", { name: "アカウントを追加" }));
    expect(screen.getByRole("region", { name: "アカウントを追加" })).toBeTruthy();
  });

  it("shows the name, login@host and the state of an account whose token works", () => {
    mount({ list: [acct("work", { id: "ghe.example.com/alice", login: "alice" })] });
    const c = within(card("work"));
    expect(c.getByRole("img", { name: "work" })).toBeTruthy();
    expect(c.getByRole("heading", { name: "work" })).toBeTruthy();
    expect(c.getByText("alice@ghe.example.com")).toBeTruthy();
    expect(c.getByText("有効")).toBeTruthy();
  });

  it("shows the sentence of an account in error as it is, in English", () => {
    mount({ list: [acct("a", { state: "error", error: "GET /user: 401 Bad credentials" })] });
    const c = within(card("a"));
    expect(c.getByText("エラー")).toBeTruthy();
    expect(c.getByText("GET /user: 401 Bad credentials")).toBeTruthy();
  });

  it("shows an account with no token, its login from the id", () => {
    mount({ list: [acct("bot", { state: "none", token: undefined, login: undefined })] });
    const c = within(card("bot"));
    expect(c.getByText("トークンなし")).toBeTruthy();
    expect(c.getByText("bot@github.com")).toBeTruthy();
    // no source of a token, and no Store
    expect(c.getAllByText("なし", { selector: "dd" })).toHaveLength(2);
  });

  it("shows where the token of each way comes from", () => {
    mount({
      list: [
        acct("g", { token: "gh", login: "alice" }),
        acct("e", { token: "env:HH_TOKEN" }),
        acct("k", { token: "keychain" }),
        acct("d", { token: "device", expires: "2026-11-04T09:05:00Z" }),
      ],
    });
    const g = within(card("g"));
    expect(g.getByText("gh auth token --hostname github.com --user g")).toBeTruthy();
    expect(within(card("e")).getByText("HH_TOKEN")).toBeTruthy();
    expect(within(card("e")).getByText(/環境変数/)).toBeTruthy();
    expect(within(card("k")).getByText(/keychain · OS の資格情報ストア/)).toBeTruthy();
    const d = within(card("d"));
    expect(d.getByText(/Device Flow · OS の資格情報ストア/)).toBeTruthy();
    const expires = new Date("2026-11-04T09:05:00Z");
    const p = (n: number) => String(n).padStart(2, "0");
    expect(
      d.getByText(
        `アクセストークンの期限 ${expires.getFullYear()}-${p(expires.getMonth() + 1)}-${p(expires.getDate())} ${p(expires.getHours())}:${p(expires.getMinutes())}`,
      ),
    ).toBeTruthy();
  });

  it("shows the Stores that use the account, or none", () => {
    mount({
      list: [acct("a"), acct("b"), acct("c")],
      stores: [
        { name: "s1", account: "a" },
        { name: "s2", account: "a" },
        { name: "s3", account: "b" },
        { name: "s4" },
      ],
    });
    expect(within(card("a")).getByText("s1 · s2")).toBeTruthy();
    expect(within(card("b")).getByText("s3")).toBeTruthy();
    expect(within(card("c")).getByText("なし", { selector: "dd" })).toBeTruthy();
  });

  it("has no delete and no account of the GitHub App", () => {
    mount({ list: [acct("a")] });
    expect(screen.queryByText(/削除/)).toBeNull();
    expect(screen.queryByText(/GitHub 連携/)).toBeNull();
    expect(screen.queryByText(/GitHub App/)).toBeNull();
  });
});

describe("SettingsAccounts: the menu", () => {
  const open = async (name: string) =>
    fireEvent.click(screen.getByRole("button", { name: `${name} の操作` }));
  const items = () => screen.getAllByRole("menuitem").map((i) => i.textContent?.trim());

  it("has log in again and log out for keychain", async () => {
    mount({ list: [acct("k", { token: "keychain" })] });
    await open("k");
    expect(items()).toEqual(["ログインし直す", "ログアウト…"]);
  });

  it("has log in again and log out for Device Flow", async () => {
    mount({ list: [acct("d", { token: "device" })] });
    await open("d");
    expect(items()).toEqual(["ログインし直す", "ログアウト…"]);
  });

  it("says Hedgehug has nothing for gh and for an environment variable", async () => {
    mount({ list: [acct("g", { token: "gh" }), acct("e", { token: "env:X" })] });
    for (const n of ["g", "e"]) {
      await open(n);
      expect(items()).toEqual(["Hedgehug が持っているものはありません"]);
      expect((screen.getByRole("menuitem") as HTMLButtonElement).disabled).toBe(true);
      await fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
    }
  });

  it("has login with Device Flow and paste a token for an account with no token", async () => {
    mount({ list: [acct("bot", { state: "none", token: undefined })] });
    await open("bot");
    expect(items()).toEqual(["GitHub でログイン", "トークンを貼る"]);
  });

  it("opens Device Flow with the name and the host filled in for log in again of a device account", async () => {
    mount({ list: [acct("d", { id: "ghe.example.com/d", token: "device" })] });
    await open("d");
    await fireEvent.click(screen.getByRole("menuitem", { name: "ログインし直す" }));
    expect((screen.getByRole("radio", { name: "Device Flow" }) as HTMLInputElement).checked).toBe(
      true,
    );
    expect((screen.getByLabelText(/呼び名/) as HTMLInputElement).value).toBe("d");
    expect((screen.getByLabelText("ホスト") as HTMLInputElement).value).toBe("ghe.example.com");
  });

  it("opens the pasted token form for log in again of a keychain account", async () => {
    mount({ list: [acct("k", { token: "keychain" })] });
    await open("k");
    await fireEvent.click(screen.getByRole("menuitem", { name: "ログインし直す" }));
    expect(
      (screen.getByRole("radio", { name: "トークンを貼る" }) as HTMLInputElement).checked,
    ).toBe(true);
    expect((screen.getByLabelText(/呼び名/) as HTMLInputElement).value).toBe("k");
  });

  it("opens each way for an account with no token", async () => {
    mount({ list: [acct("bot", { state: "none", token: undefined })] });
    await open("bot");
    await fireEvent.click(screen.getByRole("menuitem", { name: "トークンを貼る" }));
    expect(
      (screen.getByRole("radio", { name: "トークンを貼る" }) as HTMLInputElement).checked,
    ).toBe(true);
    await open("bot");
    await fireEvent.click(screen.getByRole("menuitem", { name: "GitHub でログイン" }));
    expect((screen.getByRole("radio", { name: "Device Flow" }) as HTMLInputElement).checked).toBe(
      true,
    );
  });
});

describe("SettingsAccounts: log out", () => {
  async function ask(name: string) {
    await fireEvent.click(screen.getByRole("button", { name: `${name} の操作` }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "ログアウト…" }));
  }

  it("asks first, with the command, and does nothing when it is cancelled", async () => {
    const { api } = mount({ list: [acct("k")] });
    await ask("k");
    const d = within(screen.getByRole("dialog"));
    expect(
      d.getByText(
        /k からログアウトします。keychain のトークンを消します。アカウントは config.yaml に残ります/,
      ),
    ).toBeTruthy();
    expect(d.getByRole("button", { name: /^コマンドをコピー/ }).textContent).toContain(
      "hh auth logout k",
    );
    await fireEvent.click(d.getByRole("button", { name: "キャンセル" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(api.DELETE).not.toHaveBeenCalled();
  });

  it("deletes the token, says so and reads the accounts and the Stores again", async () => {
    const { api, accounts, stores } = mount({ list: [acct("k")] });
    await ask("k");
    await fireEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "ログアウト" }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(api.DELETE).toHaveBeenCalledWith("/api/accounts/{name}/token", {
      params: { path: { name: "k" } },
    });
    expect(toasts.list.map((t) => t.message)).toEqual(["k からログアウトしました"]);
    expect(accounts.reload).toHaveBeenCalledTimes(1);
    expect(stores.reload).toHaveBeenCalledTimes(1);
  });

  it("keeps the dialog and shows why when the server refuses", async () => {
    mount({ list: [acct("k")], del: () => fail(404, "account.not_found", { name: "k" }) });
    await ask("k");
    await fireEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "ログアウト" }),
    );
    expect((await within(screen.getByRole("dialog")).findByRole("alert")).textContent).toContain(
      "k",
    );
    expect(toasts.list).toHaveLength(0);
  });
});

describe("SettingsAccounts: the add form", () => {
  it("opens under the list in a frame and closes with its button", async () => {
    mount({ list: [acct("a")] });
    expect(screen.queryByRole("region", { name: "アカウントを追加" })).toBeNull();
    await fireEvent.click(screen.getByRole("button", { name: "アカウントを追加" }));
    const form = screen.getByRole("region", { name: "アカウントを追加" });
    expect(screen.queryByRole("dialog")).toBeNull();
    await fireEvent.click(within(form).getByRole("button", { name: "閉じる" }));
    expect(screen.queryByRole("region", { name: "アカウントを追加" })).toBeNull();
  });

  it("closes when an account was added", async () => {
    const { api } = mount({ list: [acct("a")] });
    api.POST.mockImplementation(() => ok({}));
    await fireEvent.click(screen.getByRole("button", { name: "アカウントを追加" }));
    await fireEvent.click(screen.getByRole("radio", { name: "トークンなし" }));
    await fireEvent.input(screen.getByLabelText(/呼び名/), { target: { value: "bot" } });
    await fireEvent.input(screen.getByLabelText("login"), { target: { value: "bot" } });
    await fireEvent.click(screen.getByRole("button", { name: "追加する" }));
    await waitFor(() =>
      expect(screen.queryByRole("region", { name: "アカウントを追加" })).toBeNull(),
    );
  });
});
