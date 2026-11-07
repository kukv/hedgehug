import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Account } from "../../core/accounts";
import { ok } from "../detail/testkit";
import { router } from "../router";
import { renderWithCore } from "../test-utils";
import Onboarding from "./Onboarding.svelte";

const acct = (name: string, over: Partial<Account> = {}): Account => ({
  name,
  id: `github.com/${name}`,
  state: "ok",
  login: name,
  token: "keychain",
  ...over,
});

const cand = (name: string, permission = "write") => ({
  name,
  url: `https://github.com/org/${name}.git`,
  account: "me",
  permission,
});

type Mount = {
  accounts?: Account[];
  loaded?: boolean;
  found?: ReturnType<typeof cand>[];
  mount?: () => unknown;
};

function mount(m: Mount = {}) {
  const discoverStores = vi.fn((_query: unknown) => ok(m.found ?? []));
  const mountStores = vi.fn((_body: unknown) =>
    m.mount ? m.mount() : ok({ mounted: [], failed: [] }),
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
  const ondone = vi.fn();
  renderWithCore(
    Onboarding,
    { ondone },
    { api: { discoverStores, mountStores }, accounts, stores },
  );
  return { discoverStores, mountStores, accounts, ondone };
}

const next = () => screen.getByRole("button", { name: "次へ: Store を探す" }) as HTMLButtonElement;
const current = () => screen.getByRole("listitem", { current: "step" }).textContent;
const tick = (el: HTMLElement) => fireEvent.click(el);

beforeEach(() => {
  history.replaceState(null, "", "/");
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
});
afterEach(() => cleanup());

describe("Onboarding: the accounts step", () => {
  it("welcomes, names the three steps and shows the one that is on", () => {
    mount();
    expect(screen.getByText("Hedgehug へようこそ")).toBeTruthy();
    const steps = screen.getByRole("list", { name: "手順" });
    expect(
      within(steps)
        .getAllByRole("listitem")
        .map((x) => x.textContent),
    ).toEqual([
      expect.stringContaining("アカウント"),
      expect.stringContaining("Store を選ぶ"),
      expect.stringContaining("始める"),
    ]);
    expect(current()).toContain("アカウント");
    expect(screen.getByRole("heading", { name: "GitHub のアカウントを登録します" })).toBeTruthy();
    expect(screen.getByText(/Hedgehug はタスクを GitHub のリポジトリに保存し/)).toBeTruthy();
    expect(screen.getByText("あとから 設定 › アカウントと認証 で変えられます")).toBeTruthy();
  });

  it("lists the accounts, and an account without a token still goes on", () => {
    mount({ accounts: [acct("e2e", { token: undefined, state: "none" })] });
    expect(screen.getByText("e2e")).toBeTruthy();
    expect(screen.getByText("e2e@github.com")).toBeTruthy();
    expect(next().disabled).toBe(false);
    expect(screen.queryByRole("region", { name: "アカウントを追加" })).toBeNull();
  });

  it("opens the form to add one when asked", async () => {
    mount();
    await tick(screen.getByRole("button", { name: "アカウントを追加" }));
    expect(screen.getByRole("region", { name: "アカウントを追加" })).toBeTruthy();
  });

  it("with no account shows the form, and cannot go on, saying why", () => {
    mount({ accounts: [] });
    expect(screen.getByRole("region", { name: "アカウントを追加" })).toBeTruthy();
    expect(next().disabled).toBe(true);
    expect(screen.getByText("アカウントを 1 つ追加してください")).toBeTruthy();
  });
});

describe("Onboarding: choosing and starting", () => {
  async function toDiscover(m: Mount = {}) {
    const r = mount(m);
    await tick(next());
    await screen.findByRole("heading", { name: "使う Store を選んでください" });
    return r;
  }

  it("goes to the Stores step, looks, and goes back", async () => {
    const { discoverStores } = await toDiscover({ found: [cand("hh-a")] });
    expect(current()).toContain("Store を選ぶ");
    await screen.findByText("org/hh-a");
    expect(discoverStores).toHaveBeenCalledWith({});
    await tick(screen.getByRole("button", { name: "戻る" }));
    expect(screen.getByRole("heading", { name: "GitHub のアカウントを登録します" })).toBeTruthy();
  });

  it("mounts, then shows what was mounted, and starts", async () => {
    const { mountStores, ondone } = await toDiscover({
      found: [cand("hh-a"), cand("hh-b", "read")],
      mount: () => ok({ mounted: ["hh-a", "hh-b"], failed: [] }),
    });
    await tick(await screen.findByLabelText("org/hh-a を選ぶ"));
    await tick(screen.getByLabelText("org/hh-b を選ぶ"));
    await tick(screen.getByRole("button", { name: "マウントして次へ" }));
    await screen.findByRole("heading", { name: "Store を使い始めます" });
    expect(mountStores).toHaveBeenCalledTimes(1);
    expect(current()).toContain("始める");
    const list = screen.getByRole("list", { name: "マウントした Store" });
    expect(within(list).getByText("hh-a")).toBeTruthy();
    expect(within(list).getByText("閲覧のみ")).toBeTruthy();
    const go = vi.spyOn(router, "go").mockImplementation(() => {});
    await tick(screen.getByRole("button", { name: "始める" }));
    expect(ondone).toHaveBeenCalledTimes(1);
    expect(go).toHaveBeenCalledWith("/");
    go.mockRestore();
  });

  it("stays on a row that failed, and goes on by the button", async () => {
    await toDiscover({
      found: [cand("hh-a"), cand("hh-b")],
      mount: () =>
        ok({
          mounted: ["hh-a"],
          failed: [{ name: "hh-b", code: "store.exists", message: "used" }],
        }),
    });
    await tick(await screen.findByLabelText("org/hh-a を選ぶ"));
    await tick(screen.getByLabelText("org/hh-b を選ぶ"));
    await tick(screen.getByRole("button", { name: "マウントして次へ" }));
    await screen.findByText("hh-b という Store はすでにあります");
    expect(screen.queryByRole("heading", { name: "Store を使い始めます" })).toBeNull();
    await tick(screen.getByRole("button", { name: "次へ" }));
    expect(screen.getByRole("heading", { name: "Store を使い始めます" })).toBeTruthy();
    expect(
      within(screen.getByRole("list", { name: "マウントした Store" })).getByText("hh-a"),
    ).toBeTruthy();
  });

  it("has no way on before something is mounted", async () => {
    await toDiscover();
    expect(screen.queryByRole("button", { name: "次へ" })).toBeNull();
  });
});

describe("Onboarding: making a new Store", () => {
  it("is offered in every step and opens CreateStore", async () => {
    mount({ found: [cand("hh-a")], mount: () => ok({ mounted: ["hh-a"], failed: [] }) });
    const open = async () => {
      await tick(screen.getByRole("button", { name: "新しく Store を作成" }));
      expect(screen.getByRole("dialog", { name: "Store を作成" })).toBeTruthy();
      await tick(screen.getByRole("button", { name: "キャンセル" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    };
    await open();
    await tick(next());
    await screen.findByLabelText("org/hh-a を選ぶ");
    await open();
    await tick(screen.getByLabelText("org/hh-a を選ぶ"));
    await tick(screen.getByRole("button", { name: "マウントして次へ" }));
    await screen.findByRole("heading", { name: "Store を使い始めます" });
    await open();
  });
});
