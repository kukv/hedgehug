import { cleanup, fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Account } from "../../core/accounts";
import type { Schemas } from "../../core/api/client";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import StoreDiscover from "./StoreDiscover.svelte";

type Candidate = Schemas.Candidate;

const acct = (name: string, over: Partial<Account> = {}): Account => ({
  name,
  id: `github.com/${name}`,
  state: "ok",
  login: name,
  token: "keychain",
  ...over,
});

const cand = (name: string, owner: string, over: Partial<Candidate> = {}): Candidate => ({
  name,
  url: `https://github.com/${owner}/${name}.git`,
  account: "me",
  permission: "write",
  ...over,
});

type Query = { account?: string; from?: string };
type Mount = {
  accounts?: Account[];
  loaded?: boolean;
  get?: (query: Query) => unknown;
  post?: () => unknown;
};

function mount(m: Mount = {}) {
  const discoverStores = vi.fn((query: Query) => (m.get ? m.get(query) : ok([])));
  const mountStores = vi.fn((_body: unknown) =>
    m.post ? m.post() : ok({ mounted: [], failed: [] }),
  );
  const accounts = {
    accounts: () => m.accounts ?? [acct("me")],
    loaded: () => m.loaded ?? true,
    error: () => null,
    load: vi.fn(async () => {}),
    reload: vi.fn(async () => {}),
    subscribe: () => () => {},
  };
  const stores = { reload: vi.fn(async () => {}), subscribe: () => () => {} };
  const onmounted = vi.fn();
  renderWithCore(
    StoreDiscover,
    { mountLabel: "マウント", onmounted },
    { api: { discoverStores, mountStores }, accounts, stores },
  );
  return { discoverStores, mountStores, accounts, stores, onmounted };
}

const type = (el: HTMLElement, value: string) => fireEvent.input(el, { target: { value } });
const cli = () => screen.getByRole("button", { name: /^コマンドをコピー/ }).textContent;

beforeEach(() => {
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
});
afterEach(() => cleanup());

describe("StoreDiscover: the search on GitHub", () => {
  it("looks on its own when an account has a token", async () => {
    const { discoverStores } = mount({ get: () => ok([cand("hh-tasks", "org")]) });
    await screen.findByText("org");
    expect(discoverStores).toHaveBeenCalledTimes(1);
    expect(discoverStores).toHaveBeenCalledWith({});
  });

  it("does not look when no account has a token, and offers the folder", async () => {
    const { discoverStores } = mount({
      accounts: [acct("a", { token: undefined }), acct("b", { state: "error" })],
    });
    expect(
      await screen.findByText("GitHub で探すにはトークンのあるアカウントが要ります"),
    ).toBeTruthy();
    expect(discoverStores).not.toHaveBeenCalled();
    expect(screen.getByLabelText("フォルダ")).toBeTruthy();
  });

  it("waits for the accounts to be read", async () => {
    const { discoverStores, accounts } = mount({ loaded: false });
    expect(accounts.load).toHaveBeenCalled();
    expect(screen.getByText("読み込み中…")).toBeTruthy();
    expect(discoverStores).not.toHaveBeenCalled();
  });

  it("puts the candidates under their owner, with permission and a name to call them", async () => {
    mount({
      get: () =>
        ok([
          cand("hh-a", "org-x"),
          cand("hh-b", "org-x", { permission: "read" }),
          cand("mine", "alice", { account: "me" }),
          cand("local", "x", { url: "/srv/remotes/local.git" }),
        ]),
    });
    await screen.findByText("org-x");
    expect(screen.getByText("alice")).toBeTruthy();
    expect(screen.getByText("その他")).toBeTruthy();
    expect(screen.getByText("org-x/hh-a")).toBeTruthy();
    expect(screen.getByText("/srv/remotes/local.git")).toBeTruthy();
    expect(screen.getAllByText("書き込み")).toHaveLength(3);
    expect(screen.getByText("閲覧のみ")).toBeTruthy();
    expect((screen.getByLabelText("org-x/hh-a の呼び名") as HTMLInputElement).value).toBe("hh-a");
    expect((screen.getByLabelText("org-x/hh-a を選ぶ") as HTMLInputElement).checked).toBe(false);
  });

  it("says so when it finds nothing, and tells who to ask", async () => {
    mount({ get: () => ok([]) });
    expect(await screen.findByText("見つかりませんでした")).toBeTruthy();
    expect(
      screen.getByText(
        "探しているチームの Store が見つからないときは、そのリポジトリの権限をチームの管理者に依頼してください。",
      ),
    ).toBeTruthy();
  });

  it("shows why it could not look, and looks again", async () => {
    let n = 0;
    const { discoverStores } = mount({
      get: () => (n++ === 0 ? fail(502, "github.unavailable") : ok([cand("hh-a", "org")])),
    });
    await screen.findByRole("alert");
    await fireEvent.click(screen.getByRole("button", { name: "もう一度探す" }));
    await screen.findByText("org");
    expect(discoverStores).toHaveBeenCalledTimes(2);
  });

  it("shows the command at the foot", async () => {
    mount();
    await screen.findByText("見つかりませんでした");
    expect(cli()).toContain("$ hh store discover");
  });
});

describe("StoreDiscover: the search in a folder", () => {
  it("looks with from, and the command has it", async () => {
    const { discoverStores } = mount({
      accounts: [acct("me", { token: undefined })],
      get: () => ok([cand("a", "x", { url: "/srv/r/a.git", account: undefined })]),
    });
    await type(await screen.findByLabelText("フォルダ"), "/srv/r");
    expect(cli()?.trim()).toBe("$ hh store discover --from /srv/r");
    await fireEvent.click(screen.getByRole("button", { name: "探す" }));
    await screen.findByText("/srv/r/a.git");
    expect(discoverStores).toHaveBeenCalledWith({ from: "/srv/r" });
  });

  it("asks for the account when the server cannot tell, and looks again with it", async () => {
    const { discoverStores } = mount({
      get: (q) =>
        q.from && !q.account
          ? fail(422, "choice.required", { what: "--account", candidates: ["a", "b"] })
          : q.from
            ? ok([cand("a", "x", { url: "/srv/r/a.git" })])
            : ok([]),
    });
    await screen.findByText("見つかりませんでした");
    await type(screen.getByLabelText("フォルダ"), "/srv/r");
    await fireEvent.click(screen.getByRole("button", { name: "探す" }));
    const pick = (await screen.findByLabelText("アカウント")) as HTMLSelectElement;
    expect(Array.from(pick.options).map((o) => o.value)).toEqual(["", "a", "b"]);
    await fireEvent.change(pick, { target: { value: "b" } });
    expect(cli()?.trim()).toBe("$ hh store discover --account b --from /srv/r");
    await fireEvent.click(screen.getByRole("button", { name: "探す" }));
    await screen.findByText("/srv/r/a.git");
    expect(discoverStores).toHaveBeenLastCalledWith({ account: "b", from: "/srv/r" });
  });
});

describe("StoreDiscover: mounting", () => {
  const two = () => ok([cand("hh-a", "org"), cand("hh-b", "org")]);

  it("sends the chosen ones under the names in the fields", async () => {
    const { mountStores, stores, onmounted } = mount({
      get: two,
      post: () => ok({ mounted: ["mine"], failed: [] }),
    });
    await screen.findByText("org");
    await type(screen.getByLabelText("org/hh-a の呼び名"), " mine ");
    await fireEvent.click(screen.getByLabelText("org/hh-a を選ぶ"));
    await fireEvent.click(screen.getByRole("button", { name: "マウント" }));
    await waitFor(() => expect(mountStores).toHaveBeenCalledTimes(1));
    expect(mountStores).toHaveBeenCalledWith([
      { name: "mine", url: "https://github.com/org/hh-a.git", account: "me" },
    ]);
    await screen.findByText("マウントしました");
    expect(stores.reload).toHaveBeenCalledTimes(1);
    expect(onmounted).toHaveBeenCalledWith({
      mounted: [{ name: "mine", permission: "write" }],
      failed: 0,
    });
    expect((screen.getByLabelText("org/hh-a を選ぶ") as HTMLInputElement).checked).toBe(false);
  });

  it("cannot mount with nothing chosen", async () => {
    mount({ get: two });
    await screen.findByText("org");
    expect((screen.getByRole("button", { name: "マウント" }) as HTMLButtonElement).disabled).toBe(
      true,
    );
  });

  it("does not send a candidate with an empty name, and says so on its row", async () => {
    const { mountStores } = mount({ get: two, post: () => ok({ mounted: ["hh-b"], failed: [] }) });
    await screen.findByText("org");
    await type(screen.getByLabelText("org/hh-a の呼び名"), "  ");
    await fireEvent.click(screen.getByLabelText("org/hh-a を選ぶ"));
    await fireEvent.click(screen.getByLabelText("org/hh-b を選ぶ"));
    await fireEvent.click(screen.getByRole("button", { name: "マウント" }));
    await waitFor(() => expect(mountStores).toHaveBeenCalledTimes(1));
    expect(mountStores.mock.calls[0]?.[0]).toEqual([
      { name: "hh-b", url: "https://github.com/org/hh-b.git", account: "me" },
    ]);
    expect(screen.getByText("名前を入れてください")).toBeTruthy();
  });

  it("does not call the server when the only chosen one has no name", async () => {
    const { mountStores } = mount({ get: two });
    await screen.findByText("org");
    await type(screen.getByLabelText("org/hh-a の呼び名"), "");
    await fireEvent.click(screen.getByLabelText("org/hh-a を選ぶ"));
    await fireEvent.click(screen.getByRole("button", { name: "マウント" }));
    expect(screen.getByText("名前を入れてください")).toBeTruthy();
    expect(mountStores).not.toHaveBeenCalled();
  });

  it("puts a failure on its row and goes on with the others", async () => {
    const { onmounted, stores } = mount({
      get: two,
      post: () =>
        ok({
          mounted: ["hh-b"],
          failed: [{ name: "hh-a", code: "store.exists", message: "the name hh-a is used" }],
        }),
    });
    await screen.findByText("org");
    await fireEvent.click(screen.getByLabelText("org/hh-a を選ぶ"));
    await fireEvent.click(screen.getByLabelText("org/hh-b を選ぶ"));
    await fireEvent.click(screen.getByRole("button", { name: "マウント" }));
    await screen.findByText("マウントしました");
    expect(screen.getByText("hh-a という Store はすでにあります")).toBeTruthy();
    expect(onmounted).toHaveBeenCalledWith({
      mounted: [{ name: "hh-b", permission: "write" }],
      failed: 1,
    });
    expect(stores.reload).toHaveBeenCalledTimes(1);
    // the one that failed stays chosen, to be tried again
    expect((screen.getByLabelText("org/hh-a を選ぶ") as HTMLInputElement).checked).toBe(true);
  });

  it("does not reload the Stores when nothing was mounted", async () => {
    const { stores, onmounted } = mount({
      get: two,
      post: () =>
        ok({ mounted: [], failed: [{ name: "hh-a", code: "store.exists", message: "used" }] }),
    });
    await screen.findByText("org");
    await fireEvent.click(screen.getByLabelText("org/hh-a を選ぶ"));
    await fireEvent.click(screen.getByRole("button", { name: "マウント" }));
    await waitFor(() => expect(onmounted).toHaveBeenCalledWith({ mounted: [], failed: 1 }));
    expect(stores.reload).not.toHaveBeenCalled();
  });
});

describe("StoreDiscover: rows that are not on the screen", () => {
  it("cannot mount a row chosen before a search that asks for the account", async () => {
    const { mountStores } = mount({
      get: (q) =>
        q.from
          ? fail(422, "choice.required", { what: "--account", candidates: ["a", "b"] })
          : ok([cand("hh-a", "org")]),
    });
    await fireEvent.click(await screen.findByLabelText("org/hh-a を選ぶ"));
    await type(screen.getByLabelText("フォルダ"), "/srv/r");
    await fireEvent.click(screen.getByRole("button", { name: "探す" }));
    await screen.findByLabelText("アカウント");
    expect(screen.queryByLabelText("org/hh-a を選ぶ")).toBeNull();
    expect((screen.getByRole("button", { name: "マウント" }) as HTMLButtonElement).disabled).toBe(
      true,
    );
    expect(mountStores).not.toHaveBeenCalled();
  });

  it("drops the answer of an older search that comes after a newer one", async () => {
    const github: { resolve: (r: unknown) => void }[] = [];
    mount({
      get: (q) =>
        q.from
          ? ok([cand("newer", "org", { url: "/srv/r/newer.git" })])
          : new Promise((resolve) => github.push({ resolve })),
    });
    // the search on GitHub is still waiting when the folder is searched
    await type(await screen.findByLabelText("フォルダ"), "/srv/r");
    await waitFor(() => expect(github).toHaveLength(1));
    await fireEvent.click(screen.getByRole("button", { name: "探す" }));
    await screen.findByText("/srv/r/newer.git");
    github[0]?.resolve(await ok([cand("older", "org")]));
    await new Promise((r) => setTimeout(r, 20));
    expect(screen.getByText("/srv/r/newer.git")).toBeTruthy();
    expect(screen.queryByText("org/older")).toBeNull();
  });
});
