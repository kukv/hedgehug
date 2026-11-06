import { cleanup, fireEvent, screen, waitFor } from "@testing-library/svelte";
import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AuthDeviceEvent } from "../../core/connection";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import AccountAdd, { type AddSource } from "./AccountAdd.svelte";

const NOW = new Date("2026-11-04T09:00:00Z");
const started = {
  id: "F1",
  user_code: "ABCD-1234",
  verification_uri: "https://github.com/login/device",
  expires_at: "2026-11-04T09:14:59Z",
};

type Reply = ReturnType<typeof ok> | ReturnType<typeof fail>;

function mount(
  props: {
    initial?: { source?: AddSource; name?: string; host?: string };
    onclose?: () => void;
  } = {},
  replies: { start?: () => Reply; add?: () => Reply; put?: () => Reply; del?: () => Reply } = {},
  existing: string[] = [],
) {
  const api = {
    startDeviceLogin: vi.fn((_body: unknown) => (replies.start ? replies.start() : ok(started))),
    addAccount: vi.fn((_body: unknown) => (replies.add ? replies.add() : ok({}))),
    setAccountToken: vi.fn(() =>
      replies.put ? replies.put() : ok({ account: "e2e", added: true, id: "x" }),
    ),
    cancelDeviceLogin: vi.fn(() => (replies.del ? replies.del() : ok(undefined))),
  };
  const handlers = new Set<(e: AuthDeviceEvent) => void>();
  const conn = {
    on: (_: string, fn: (e: AuthDeviceEvent) => void) => {
      handlers.add(fn);
      return () => handlers.delete(fn);
    },
  };
  const accounts = {
    accounts: () => existing.map((name) => ({ name })),
    reload: vi.fn(async () => {}),
    load: async () => {},
    loaded: () => true,
    error: () => null,
    subscribe: () => () => {},
  };
  const stores = { reload: vi.fn(async () => {}), stores: () => [], subscribe: () => () => {} };
  const ondone = vi.fn();
  const view = renderWithCore(AccountAdd, { ...props, ondone }, { api, conn, accounts, stores });
  const emit = (e: AuthDeviceEvent) => {
    for (const fn of [...handlers]) fn(e);
    flushSync();
  };
  return { api, accounts, stores, ondone, emit, view };
}

const radio = (name: string) => screen.getByRole("radio", { name }) as HTMLInputElement;
const field = (name: string | RegExp) => screen.getByLabelText(name) as HTMLInputElement;
const button = (name: string) => screen.getByRole("button", { name }) as HTMLButtonElement;
const cliText = () =>
  (screen.getByRole("button", { name: /^コマンドをコピー/ }).textContent ?? "").trim();
const type = (name: string | RegExp, v: string) =>
  fireEvent.input(field(name), { target: { value: v } });
const toastMessages = () => toasts.list.map((t) => t.message);

beforeEach(() => {
  vi.useFakeTimers({
    toFake: ["setInterval", "clearInterval", "setTimeout", "clearTimeout", "Date"],
    shouldAdvanceTime: true,
  });
  vi.setSystemTime(NOW);
  resetToasts();
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("AccountAdd: the ways to get the token", () => {
  it("lists the five with the words of the canvas, Device Flow chosen", () => {
    mount();
    for (const n of ["Device Flow", "トークンを貼る", "gh", "環境変数", "トークンなし"])
      expect(radio(n)).toBeTruthy();
    expect(radio("Device Flow").checked).toBe(true);
    expect(
      screen.getByText("Hedgehug 専用の GitHub App でログイン。gh がなくても使える"),
    ).toBeTruthy();
    expect(screen.getByText("自分で発行したトークンを OS の資格情報ストアに保存")).toBeTruthy();
    expect(
      screen.getByText("gh の複数アカウントをそのまま使う。gh があるときの第一候補"),
    ).toBeTruthy();
    expect(screen.getByText("指定した変数から読む。CI 向け")).toBeTruthy();
    expect(screen.getByText(/トークンを使わないアカウント/)).toBeTruthy();
  });

  it("shows the fields of the chosen way", async () => {
    mount();
    expect(field(/呼び名/)).toBeTruthy();
    expect(field("ホスト").value).toBe("github.com");
    expect(screen.queryByLabelText("login")).toBeNull();
    expect(screen.queryByLabelText("トークン")).toBeNull();

    await fireEvent.click(radio("トークンを貼る"));
    const t = field("トークン");
    expect(t.type).toBe("password");
    expect(t.getAttribute("autocomplete")).toBe("off");
    expect(screen.queryByLabelText("login")).toBeNull();

    await fireEvent.click(radio("gh"));
    expect(field("login")).toBeTruthy();
    expect(screen.queryByLabelText("変数名")).toBeNull();

    await fireEvent.click(radio("環境変数"));
    expect(field("変数名")).toBeTruthy();

    await fireEvent.click(radio("トークンなし"));
    expect(field("login")).toBeTruthy();
    expect(screen.queryByLabelText("変数名")).toBeNull();
  });

  it("opens with the way, the name and the host it is given (login again)", () => {
    mount({ initial: { source: "token", name: "work", host: "ghe.example.com" } });
    expect(radio("トークンを貼る").checked).toBe(true);
    expect(field(/呼び名/).value).toBe("work");
    expect(field("ホスト").value).toBe("ghe.example.com");
  });

  it("has a close button only when it is given a way to close", async () => {
    const view = mount();
    expect(screen.queryByRole("button", { name: "閉じる" })).toBeNull();
    view.view.unmount();
    const onclose = vi.fn();
    mount({ onclose });
    await fireEvent.click(button("閉じる"));
    expect(onclose).toHaveBeenCalledTimes(1);
  });
});

describe("AccountAdd: the line of hh", () => {
  it("is hh auth login for a Device Flow, with the name and a host that is not github.com", async () => {
    mount();
    expect(cliText()).toBe("$ hh auth login");
    await type(/呼び名/, "work");
    await type("ホスト", "ghe.example.com");
    expect(cliText()).toBe("$ hh auth login work --host ghe.example.com");
  });

  it("is hh auth login --with-token for a pasted token, and never has the token", async () => {
    mount({ initial: { source: "token" } });
    await type(/呼び名/, "work");
    await type("トークン", "ghp_secret");
    expect(cliText()).toBe("$ hh auth login work --with-token");
    expect(document.body.innerHTML).not.toContain("ghp_secret");
  });

  it("is hh auth add for gh, an environment variable and no token, and cannot be copied until complete", async () => {
    mount({ initial: { source: "gh" } });
    expect(screen.getByRole("button", { name: /^コマンドをコピー/ }).hasAttribute("disabled")).toBe(
      true,
    );
    await type(/呼び名/, "a");
    await type("login", "alice");
    expect(cliText()).toBe("$ hh auth add a --login alice --token gh");
    expect(screen.getByRole("button", { name: /^コマンドをコピー/ }).hasAttribute("disabled")).toBe(
      false,
    );

    await fireEvent.click(radio("環境変数"));
    await type("変数名", "HH_TOKEN");
    expect(cliText()).toBe("$ hh auth add a --login alice --token env:HH_TOKEN");

    await fireEvent.click(radio("トークンなし"));
    expect(cliText()).toBe("$ hh auth add a --login alice");
  });
});

describe("AccountAdd: Device Flow", () => {
  it("starts with the name and the host, and shows the code", async () => {
    const { api } = mount();
    await type(/呼び名/, "work");
    await fireEvent.click(button("GitHub でログイン"));
    expect(api.startDeviceLogin).toHaveBeenCalledWith({ name: "work", host: "github.com" });
    expect(await screen.findByText("ABCD-1234")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "GitHub でログイン" })).toBeNull();
  });

  it("sends no name when it is empty", async () => {
    const { api } = mount();
    await fireEvent.click(button("GitHub でログイン"));
    expect(api.startDeviceLogin).toHaveBeenCalledWith({ name: undefined, host: "github.com" });
  });

  it("on done: toast, close, and read the accounts and the Stores again", async () => {
    const { emit, ondone, accounts, stores, api } = mount();
    await fireEvent.click(button("GitHub でログイン"));
    await screen.findByText("ABCD-1234");
    emit({ id: "F1", state: "done", account: "work" });
    expect(toastMessages()).toEqual(["アカウント work を追加しました"]);
    expect(ondone).toHaveBeenCalledWith("work");
    expect(accounts.reload).toHaveBeenCalledTimes(1);
    expect(stores.reload).toHaveBeenCalledTimes(1);
    expect(screen.queryByText("ABCD-1234")).toBeNull();
    // the flow is over: nothing to cancel
    cleanup();
    expect(api.cancelDeviceLogin).not.toHaveBeenCalled();
  });

  it("says logged in, not added, for an account that was there", async () => {
    const { emit } = mount({}, {}, ["work"]);
    await fireEvent.click(button("GitHub でログイン"));
    await screen.findByText("ABCD-1234");
    emit({ id: "F1", state: "done", account: "work" });
    expect(toastMessages()).toEqual(["work にログインしました"]);
  });

  it("shows the words of an error and starts again from the button", async () => {
    const { emit, api } = mount();
    await fireEvent.click(button("GitHub でログイン"));
    await screen.findByText("ABCD-1234");
    emit({ id: "F1", state: "error", code: "auth.device_failed", message: "expired_token" });
    expect(screen.getByRole("alert").textContent).toContain(
      "デバイスフローでログインできませんでした",
    );
    await fireEvent.click(button("やり直す"));
    await waitFor(() => expect(api.startDeviceLogin).toHaveBeenCalledTimes(2));
    expect(await screen.findByText("ABCD-1234")).toBeTruthy();
  });

  it("cancels the flow when the form is closed", async () => {
    const { api, view } = mount();
    await fireEvent.click(button("GitHub でログイン"));
    await screen.findByText("ABCD-1234");
    view.unmount();
    expect(api.cancelDeviceLogin).toHaveBeenCalledWith("F1");
  });

  it("cancels the flow when another way is chosen, and ignores a failure of the cancel", async () => {
    const { api } = mount({}, { del: () => fail(404, "auth.device_not_found") });
    await fireEvent.click(button("GitHub でログイン"));
    await screen.findByText("ABCD-1234");
    await fireEvent.click(radio("gh"));
    expect(api.cancelDeviceLogin).toHaveBeenCalledTimes(1);
    expect(screen.queryByText("ABCD-1234")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("takes over a flow another screen started when the 409 has the code", async () => {
    const { api } = mount({}, { start: () => fail(409, "auth.device_running", started) });
    await fireEvent.click(button("GitHub でログイン"));
    expect(await screen.findByText("ABCD-1234")).toBeTruthy();
    expect(screen.getByText("別の画面で始めたログインを続けます")).toBeTruthy();
    expect(api.cancelDeviceLogin).not.toHaveBeenCalled();
  });

  it("offers to stop and start again when the 409 has only the id", async () => {
    let n = 0;
    const { api } = mount(
      {},
      { start: () => (n++ === 0 ? fail(409, "auth.device_running", { id: "OLD" }) : ok(started)) },
    );
    await fireEvent.click(button("GitHub でログイン"));
    await fireEvent.click(await screen.findByRole("button", { name: "やめてやり直す" }));
    expect(api.cancelDeviceLogin).toHaveBeenCalledWith("OLD");
    expect(await screen.findByText("ABCD-1234")).toBeTruthy();
    expect(api.startDeviceLogin).toHaveBeenCalledTimes(2);
  });

  it("does not start again when the form goes while the old flow is being stopped", async () => {
    let release: () => void = () => {};
    const pending = new Promise<void>((r) => {
      release = r;
    });
    let n = 0;
    const { api, view } = mount(
      {},
      {
        start: () => (n++ === 0 ? fail(409, "auth.device_running", { id: "OLD" }) : ok(started)),
        del: () => pending.then(() => ok(undefined)) as ReturnType<typeof ok>,
      },
    );
    await fireEvent.click(button("GitHub でログイン"));
    await fireEvent.click(await screen.findByRole("button", { name: "やめてやり直す" }));
    view.unmount();
    release();
    await vi.advanceTimersByTimeAsync(10);
    expect(api.startDeviceLogin).toHaveBeenCalledTimes(1);
  });

  it("shows the words of a refusal to start", async () => {
    mount({}, { start: () => fail(422, "auth.client_id", { host: "github.com" }) });
    await fireEvent.click(button("GitHub でログイン"));
    expect((await screen.findByRole("alert")).textContent).toContain("client_id");
  });
});

describe("AccountAdd: a pasted token", () => {
  async function fill() {
    await type(/呼び名/, "work");
    await type("トークン", "ghp_secret");
  }

  it("needs a name and a token", async () => {
    mount({ initial: { source: "token" } });
    expect(button("トークンを保存").disabled).toBe(true);
    await type(/呼び名/, "work");
    expect(button("トークンを保存").disabled).toBe(true);
    await type("トークン", "x");
    expect(button("トークンを保存").disabled).toBe(false);
  });

  it("puts the token to the account, and says it added the account", async () => {
    const { api, ondone, accounts, stores } = mount(
      { initial: { source: "token" } },
      { put: () => ok({ account: "work", added: true, id: "i" }) },
    );
    await fill();
    await fireEvent.click(button("トークンを保存"));
    await waitFor(() => expect(ondone).toHaveBeenCalledWith("work"));
    expect(api.setAccountToken).toHaveBeenCalledWith("work", {
      token: "ghp_secret",
      host: "github.com",
    });
    expect(toastMessages()).toEqual(["アカウント work を追加しました"]);
    expect(accounts.reload).toHaveBeenCalled();
    expect(stores.reload).toHaveBeenCalled();
    expect(field("トークン").value).toBe("");
  });

  it("says it saved the token of an account that was there", async () => {
    const { ondone } = mount(
      { initial: { source: "token" } },
      { put: () => ok({ account: "work", added: false, id: "i" }) },
    );
    await fill();
    await fireEvent.click(button("トークンを保存"));
    await waitFor(() => expect(ondone).toHaveBeenCalled());
    expect(toastMessages()).toEqual(["work のトークンを保存しました"]);
  });

  it("says nothing was kept for an account whose token comes from gh", async () => {
    const { ondone } = mount(
      { initial: { source: "token" } },
      { put: () => ok({ account: "work", added: false, id: "i", uses: "gh" }) },
    );
    await fill();
    await fireEvent.click(button("トークンを保存"));
    await waitFor(() => expect(ondone).toHaveBeenCalled());
    expect(toastMessages()[0]).toContain("保存したものはありません");
  });

  it("empties the token field when the server refuses, and shows why", async () => {
    const { ondone } = mount(
      { initial: { source: "token" } },
      { put: () => fail(401, "auth.unauthorized", { account: "work" }) },
    );
    await fill();
    await fireEvent.click(button("トークンを保存"));
    expect((await screen.findByRole("alert")).textContent).toContain("work");
    expect(field("トークン").value).toBe("");
    expect(ondone).not.toHaveBeenCalled();
    expect(document.body.innerHTML).not.toContain("ghp_secret");
  });
});

describe("AccountAdd: gh, an environment variable and no token", () => {
  it("needs a name and a login", async () => {
    mount({ initial: { source: "none" } });
    expect(button("追加する").disabled).toBe(true);
    await type(/呼び名/, "bot");
    expect(button("追加する").disabled).toBe(true);
    await type("login", "bot");
    expect(button("追加する").disabled).toBe(false);
  });

  it("adds an account without a token", async () => {
    const { api, ondone, accounts } = mount({ initial: { source: "none" } });
    await type(/呼び名/, "bot");
    await type("login", "bot");
    await fireEvent.click(button("追加する"));
    await waitFor(() => expect(ondone).toHaveBeenCalledWith("bot"));
    expect(api.addAccount).toHaveBeenCalledWith({
      name: "bot",
      login: "bot",
      host: "github.com",
      token: undefined,
    });
    expect(toastMessages()).toEqual(["アカウント bot を追加しました"]);
    expect(accounts.reload).toHaveBeenCalled();
  });

  it("adds an account whose token is gh", async () => {
    const { api, ondone } = mount({ initial: { source: "gh" } });
    await type(/呼び名/, "a");
    await type("login", "alice");
    await fireEvent.click(button("追加する"));
    await waitFor(() => expect(ondone).toHaveBeenCalled());
    expect(api.addAccount).toHaveBeenCalledWith({
      name: "a",
      login: "alice",
      host: "github.com",
      token: "gh",
    });
  });

  it("needs the name of the variable, in capitals, digits and _", async () => {
    const { api, ondone } = mount({ initial: { source: "env" } });
    await type(/呼び名/, "ci");
    await type("login", "ci-bot");
    expect(button("追加する").disabled).toBe(true);
    await type("変数名", "hh token");
    expect(button("追加する").disabled).toBe(true);
    expect(field("変数名").getAttribute("aria-invalid")).toBe("true");
    await type("変数名", "lower");
    expect(button("追加する").disabled).toBe(true);
    await type("変数名", "HH_TOKEN_2");
    expect(field("変数名").getAttribute("aria-invalid")).toBeNull();
    await fireEvent.click(button("追加する"));
    await waitFor(() => expect(ondone).toHaveBeenCalled());
    expect(api.addAccount).toHaveBeenCalledWith({
      name: "ci",
      login: "ci-bot",
      host: "github.com",
      token: "env:HH_TOKEN_2",
    });
  });

  it("shows why the server refused", async () => {
    mount(
      { initial: { source: "none" } },
      { add: () => fail(409, "account.exists", { account: "bot" }) },
    );
    await type(/呼び名/, "bot");
    await type("login", "bot");
    await fireEvent.click(button("追加する"));
    expect((await screen.findByRole("alert")).textContent).toContain("bot");
  });
});
