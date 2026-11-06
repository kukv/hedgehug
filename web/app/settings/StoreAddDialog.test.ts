import { cleanup, fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Account } from "../../core/accounts";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import StoreAddDialog from "./StoreAddDialog.svelte";

const acct = (name: string): Account => ({
  name,
  id: `github.com/${name}`,
  state: "ok",
  login: name,
  token: "keychain",
});

const added = { name: "team", path: "/p", remotes: ["u"], hidden: false, default_store: false };

function mount(m: { accounts?: Account[]; post?: () => unknown } = {}) {
  const api = { addStore: vi.fn((_body: unknown) => (m.post ? m.post() : ok(added))) };
  const reload = vi.fn(async () => {});
  const accounts = {
    accounts: () => m.accounts ?? [],
    loaded: () => true,
    error: () => null,
    load: vi.fn(async () => {}),
    reload: vi.fn(async () => {}),
    subscribe: () => () => {},
  };
  const onclose = vi.fn();
  renderWithCore(
    StoreAddDialog,
    { open: true, onclose },
    { api, accounts, stores: { reload, subscribe: () => () => {} } },
  );
  return { api, reload, accounts, onclose };
}

const type = (el: HTMLElement, value: string) => fireEvent.input(el, { target: { value } });
const nameField = () => screen.getByLabelText("名前") as HTMLInputElement;
const urlField = () => screen.getByLabelText("URL（1行に1つ）") as HTMLTextAreaElement;
const submit = () => screen.getByRole("button", { name: "追加する" });
const cli = () => screen.getByRole("button", { name: /^コマンドをコピー/ }).textContent;

beforeEach(() => {
  resetToasts();
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
});
afterEach(() => cleanup());

describe("StoreAddDialog: the name", () => {
  it("proposes the last part of the URL without .git while the name is empty", async () => {
    mount();
    await type(urlField(), "git@github.com:org/hh-tasks.git");
    expect(nameField().value).toBe("hh-tasks");
  });

  it("reads the first line and a trailing slash", async () => {
    mount();
    await type(urlField(), "https://example.com/org/team.git/\nhttps://example.com/org/other.git");
    expect(nameField().value).toBe("team");
  });

  it("follows the URL while the name is the proposal", async () => {
    mount();
    await type(urlField(), "https://example.com/o/a.git");
    expect(nameField().value).toBe("a");
    await type(urlField(), "https://example.com/o/ab.git");
    expect(nameField().value).toBe("ab");
  });

  it("does not touch a name that was typed", async () => {
    mount();
    await type(nameField(), "mine");
    await type(urlField(), "https://example.com/o/a.git");
    expect(nameField().value).toBe("mine");
  });

  it("does not overwrite a proposal the user edited", async () => {
    mount();
    await type(urlField(), "https://example.com/o/a.git");
    await type(nameField(), "edited");
    await type(urlField(), "https://example.com/o/b.git");
    expect(nameField().value).toBe("edited");
  });
});

describe("StoreAddDialog: adding", () => {
  it("sends nothing without a name or a URL, and says what is missing", async () => {
    const { api } = mount();
    await fireEvent.click(submit());
    expect(api.addStore).not.toHaveBeenCalled();
    expect(screen.getByText("名前を入れてください")).toBeTruthy();
    expect(screen.getByText("URL を入れてください")).toBeTruthy();
  });

  it("sends the name and the URLs one per line, dropping blank lines", async () => {
    const { api, reload, onclose } = mount();
    await type(nameField(), " team ");
    await type(urlField(), "git@a:o/r.git\n\n  \n git@b:o/r.git \n");
    await fireEvent.click(submit());
    await waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
    expect(api.addStore).toHaveBeenCalledWith({
      name: "team",
      remotes: ["git@a:o/r.git", "git@b:o/r.git"],
      account: undefined,
    });
    expect(reload).toHaveBeenCalledTimes(1);
    expect(toasts.list.map((t) => t.message)).toEqual(["Store「team」を追加しました"]);
  });

  it("stays and shows the words when the server refuses", async () => {
    const { onclose } = mount({ post: () => fail(409, "store.exists", { name: "team" }) });
    await type(nameField(), "team");
    await type(urlField(), "git@a:o/r.git");
    await fireEvent.click(submit());
    await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
    expect(screen.getByRole("alert").textContent).toContain("team という Store はすでにあります");
    expect(onclose).not.toHaveBeenCalled();
  });

  it("reads the accounts when it opens", () => {
    const { accounts } = mount();
    expect(accounts.load).toHaveBeenCalled();
  });
});

describe("StoreAddDialog: the account", () => {
  it("has no select with fewer than two accounts, and sends no account", async () => {
    const { api } = mount({ accounts: [acct("work")] });
    expect(screen.queryByLabelText("アカウント")).toBeNull();
    await type(nameField(), "team");
    await type(urlField(), "git@a:o/r.git");
    await fireEvent.click(submit());
    await waitFor(() => expect(api.addStore).toHaveBeenCalledTimes(1));
    expect(api.addStore.mock.calls[0]?.[0]).toEqual({
      name: "team",
      remotes: ["git@a:o/r.git"],
      account: undefined,
    });
  });

  it("has a select from two accounts, on 選ばない, and sends no account until one is chosen", async () => {
    const { api } = mount({ accounts: [acct("work"), acct("home")] });
    const select = screen.getByLabelText("アカウント") as HTMLSelectElement;
    expect(select.value).toBe("");
    expect(Array.from(select.options).map((o) => o.textContent)).toEqual([
      "選ばない",
      "work",
      "home",
    ]);
    await type(nameField(), "team");
    await type(urlField(), "git@a:o/r.git");
    await fireEvent.click(submit());
    await waitFor(() => expect(api.addStore).toHaveBeenCalledTimes(1));
    expect(api.addStore.mock.calls[0]?.[0]).toEqual({
      name: "team",
      remotes: ["git@a:o/r.git"],
      account: undefined,
    });
  });

  it("sends the chosen account", async () => {
    const { api } = mount({ accounts: [acct("work"), acct("home")] });
    await fireEvent.change(screen.getByLabelText("アカウント"), { target: { value: "home" } });
    await type(nameField(), "team");
    await type(urlField(), "git@a:o/r.git");
    await fireEvent.click(submit());
    await waitFor(() => expect(api.addStore).toHaveBeenCalledTimes(1));
    expect(api.addStore.mock.calls[0]?.[0]).toEqual({
      name: "team",
      remotes: ["git@a:o/r.git"],
      account: "home",
    });
  });

  it("shows the words of choice.required and the select with its candidates", async () => {
    const { api } = mount({
      accounts: [acct("work")],
      post: () => fail(400, "choice.required", { what: "--account", candidates: ["work", "home"] }),
    });
    await type(nameField(), "team");
    await type(urlField(), "git@a:o/r.git");
    await fireEvent.click(submit());
    await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
    expect(screen.getByRole("alert").textContent).toContain("アカウント");
    const select = screen.getByLabelText("アカウント") as HTMLSelectElement;
    expect(Array.from(select.options).map((o) => o.textContent)).toEqual([
      "選ばない",
      "work",
      "home",
    ]);
    api.addStore.mockImplementation(() => ok(added));
    await fireEvent.change(select, { target: { value: "home" } });
    await fireEvent.click(submit());
    await waitFor(() => expect(api.addStore).toHaveBeenCalledTimes(2));
    expect(api.addStore.mock.calls[1]?.[0]).toEqual({
      name: "team",
      remotes: ["git@a:o/r.git"],
      account: "home",
    });
  });
});

describe("StoreAddDialog: the command line", () => {
  it("is hh store add with placeholders until the form is filled, and cannot be copied", () => {
    mount();
    expect(cli()).toContain("$ hh store add <名前> --remote <URL>");
    expect(
      (screen.getByRole("button", { name: /^コマンドをコピー/ }) as HTMLButtonElement).disabled,
    ).toBe(true);
  });

  it("has every URL as --remote and the account only when chosen", async () => {
    mount({ accounts: [acct("work"), acct("home")] });
    await type(nameField(), "team");
    await type(urlField(), "git@a:o/r.git\ngit@b:o/r.git");
    expect(cli()).toContain("$ hh store add team --remote git@a:o/r.git --remote git@b:o/r.git");
    expect(cli()).not.toContain("--account");
    await fireEvent.change(screen.getByLabelText("アカウント"), { target: { value: "work" } });
    expect(cli()).toContain("--account work");
  });
});
