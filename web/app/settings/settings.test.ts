import { cleanup, fireEvent, within } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithCore } from "../test-utils";
import SettingsLayout from "./SettingsLayout.svelte";
import SettingsNav from "./SettingsNav.svelte";
import SettingsRoute from "./SettingsRoute.svelte";

// SettingsWorkspaces has its own test: here it only says it was drawn, and for which Store.
vi.mock("./SettingsWorkspaces.svelte", () => ({
  default: (anchor: Comment, props: { store: string }) => {
    const p = document.createElement("p");
    p.textContent = `workspaces of ${props.store}`;
    anchor.before(p);
  },
}));

// SettingsSchema too.
vi.mock("./SettingsSchema.svelte", () => ({
  default: (anchor: Comment, props: { store: string; tab: string }) => {
    const p = document.createElement("p");
    p.textContent = `schema of ${props.store} (${props.tab})`;
    anchor.before(p);
  },
}));

// SettingsGeneral has its own test too.
vi.mock("./SettingsGeneral.svelte", () => ({
  default: (anchor: Comment) => {
    const p = document.createElement("p");
    p.textContent = "general";
    anchor.before(p);
  },
}));

const health = (version: string) => ({
  data: { pid: 1, version },
  response: new Response(null, { status: 200 }),
});

function core(
  names: string[],
  get = vi.fn(async () => health("0.9.0")),
  loaded = true,
  readOnly: Record<string, string> = {},
) {
  return {
    api: { GET: get },
    stores: {
      stores: () => names.map((name) => ({ name })),
      readOnly: (name: string) => readOnly[name] ?? null,
      loaded: () => loaded,
      subscribe: () => () => {},
    },
  };
}

const page = { page: "workspaces", store: "shared" } as const;
const general = { page: "general" } as const;
const schemaPage = { page: "schema", store: "shared", tab: "statuses" } as const;

beforeEach(() => history.replaceState(null, "", "/settings/workspaces/shared"));
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("SettingsNav", () => {
  it("ワークスペースの項目を今のページとして出し、戻るリンクがある", () => {
    const { getByRole } = renderWithCore(SettingsNav, { page }, core(["shared"]));
    const item = getByRole("link", { name: /ワークスペース/ });
    expect(item.getAttribute("href")).toBe("/settings/workspaces/shared");
    expect(item.getAttribute("aria-current")).toBe("page");
    expect(getByRole("link", { name: /Hedgehug に戻る/ }).getAttribute("href")).toBe("/");
    expect(getByRole("heading", { name: "設定" })).toBeTruthy();
  });

  it("まだない画面の項目（自動化・テンプレート）は出さず、アカウントとマウント中の Store は出す", () => {
    const { queryByText, getByRole } = renderWithCore(SettingsNav, { page }, core(["shared"]));
    for (const t of ["自動化", "テンプレート"]) {
      expect(queryByText(new RegExp(t))).toBeNull();
    }
    expect(getByRole("link", { name: /アカウントと認証/ })).toBeTruthy();
    expect(getByRole("link", { name: /マウント中の Store/ })).toBeTruthy();
  });

  it("「このマシン」の段はアカウントと認証・マウント中の Store・同期と保守の順で、バッジを付けない", () => {
    const { getByRole } = renderWithCore(SettingsNav, { page }, core(["shared"]));
    const links = within(getByRole("region", { name: "このマシン" })).getAllByRole("link");
    expect(links.map((l) => l.textContent?.trim())).toEqual([
      "アカウントと認証",
      "マウント中の Store",
      "同期と保守",
    ]);
    expect(links.map((l) => l.getAttribute("href"))).toEqual([
      "/settings/accounts",
      "/settings/stores",
      "/settings/general",
    ]);
  });

  it.each([
    ["accounts", "アカウントと認証"],
    ["stores", "マウント中の Store"],
  ] as const)("%s のページでは、その項目だけが今のページ", (name, label) => {
    const { getByRole } = renderWithCore(SettingsNav, { page: { page: name } }, core(["shared"]));
    const machine = within(getByRole("region", { name: "このマシン" }));
    for (const l of machine.getAllByRole("link")) {
      const on = l.textContent?.trim() === label;
      expect(l.getAttribute("aria-current")).toBe(on ? "page" : null);
    }
  });

  it("アカウントとマウント中の Store でも、選んだ Store を覚えている", async () => {
    const view = renderWithCore(SettingsNav, { page }, core(["me", "shared"]));
    await view.rerender({ page: { page: "accounts" } });
    expect((view.getByLabelText("設定する Store") as HTMLSelectElement).value).toBe("shared");
    expect(view.getByRole("link", { name: /スキーマ/ }).getAttribute("href")).toBe(
      "/settings/schema/shared",
    );
    await view.rerender({ page: { page: "stores" } });
    expect((view.getByLabelText("設定する Store") as HTMLSelectElement).value).toBe("shared");
  });

  it("閲覧のみの Store は「（閲覧のみ）」を添える（一覧の read_only も同期の read_only も）。非表示の Store も並ぶ", () => {
    const { getByLabelText } = renderWithCore(
      SettingsNav,
      { page },
      core(["shared", "ro", "hidden"], undefined, true, {
        ro: "the remote cannot be written",
      }),
    );
    const select = getByLabelText("設定する Store") as HTMLSelectElement;
    expect([...select.options].map((o) => o.value)).toEqual(["shared", "ro", "hidden"]);
    expect([...select.options].map((o) => o.textContent?.trim())).toEqual([
      "shared",
      "ro（閲覧のみ）",
      "hidden",
    ]);
  });

  it("スキーマの項目は Store ごとの設定の段にあり、今のページならそれを示す", () => {
    const { getByRole } = renderWithCore(SettingsNav, { page: schemaPage }, core(["shared"]));
    const perStore = getByRole("region", { name: "Store ごとの設定" });
    const item = within(perStore).getByRole("link", { name: /スキーマ/ });
    expect(item.getAttribute("href")).toBe("/settings/schema/shared");
    expect(item.getAttribute("aria-current")).toBe("page");
    const ws = within(perStore).getByRole("link", { name: /ワークスペース/ });
    expect(ws.getAttribute("aria-current")).toBeNull();
    expect(ws.getAttribute("href")).toBe("/settings/workspaces/shared");
  });

  it("ワークスペースを開いているときは、スキーマの項目は同じ Store を向く", () => {
    const { getByRole } = renderWithCore(SettingsNav, { page }, core(["me", "shared"]));
    const item = getByRole("link", { name: /スキーマ/ });
    expect(item.getAttribute("href")).toBe("/settings/schema/shared");
    expect(item.getAttribute("aria-current")).toBeNull();
  });

  it("同期と保守では、スキーマの項目は選んだ Store（まだなら最初）を向く", async () => {
    history.replaceState(null, "", "/settings/general");
    const { getByRole, getByLabelText } = renderWithCore(
      SettingsNav,
      { page: general },
      core(["me", "shared"]),
    );
    expect(getByRole("link", { name: /スキーマ/ }).getAttribute("href")).toBe(
      "/settings/schema/me",
    );
    await fireEvent.change(getByLabelText("設定する Store"), { target: { value: "shared" } });
    expect(getByRole("link", { name: /スキーマ/ }).getAttribute("href")).toBe(
      "/settings/schema/shared",
    );
  });

  it("スキーマの Store の選択欄は、移ってもスキーマのページのまま", async () => {
    history.replaceState(null, "", "/settings/schema/shared");
    const { getByLabelText } = renderWithCore(
      SettingsNav,
      { page: schemaPage },
      core(["me", "shared"]),
    );
    await fireEvent.change(getByLabelText("設定する Store"), { target: { value: "me" } });
    expect(location.pathname).toBe("/settings/schema/me");
  });

  it("スキーマから同期と保守へ移っても、選んだ Store を覚えている", async () => {
    const view = renderWithCore(SettingsNav, { page: schemaPage }, core(["me", "shared"]));
    await view.rerender({ page: general });
    expect((view.getByLabelText("設定する Store") as HTMLSelectElement).value).toBe("shared");
  });

  it("Store がなければスキーマの項目も出さない", () => {
    const { queryByRole } = renderWithCore(SettingsNav, { page: general }, core([]));
    expect(queryByRole("link", { name: /スキーマ/ })).toBeNull();
  });

  it("2段: 「このマシン」に同期と保守、「Store ごとの設定」にワークスペース", () => {
    const { getByRole } = renderWithCore(SettingsNav, { page }, core(["shared"]));
    const machine = getByRole("region", { name: "このマシン" });
    const link = within(machine).getByRole("link", { name: /同期と保守/ });
    expect(link.getAttribute("href")).toBe("/settings/general");
    expect(link.getAttribute("aria-current")).toBeNull();
    const perStore = getByRole("region", { name: "Store ごとの設定" });
    expect(within(perStore).getByRole("link", { name: /ワークスペース/ })).toBeTruthy();
    expect(within(perStore).queryByRole("link", { name: /同期と保守/ })).toBeNull();
  });

  it("同期と保守を開いているときは、それを今のページにし、ワークスペースは最初の Store に向ける", () => {
    const { getByRole } = renderWithCore(SettingsNav, { page: general }, core(["me", "shared"]));
    expect(getByRole("link", { name: /同期と保守/ }).getAttribute("aria-current")).toBe("page");
    const ws = getByRole("link", { name: /ワークスペース/ });
    expect(ws.getAttribute("aria-current")).toBeNull();
    expect(ws.getAttribute("href")).toBe("/settings/workspaces/me");
  });

  it("ワークスペースの Store から同期と保守へ移っても、選んだ Store を覚えている", async () => {
    const view = renderWithCore(SettingsNav, { page }, core(["me", "shared"]));
    expect((view.getByLabelText("設定する Store") as HTMLSelectElement).value).toBe("shared");
    await view.rerender({ page: general });
    expect((view.getByLabelText("設定する Store") as HTMLSelectElement).value).toBe("shared");
    expect(view.getByRole("link", { name: /ワークスペース/ }).getAttribute("href")).toBe(
      "/settings/workspaces/shared",
    );
  });

  it("同期と保守で選んだ Store は、ワークスペースの Store が変われば追従する", async () => {
    const view = renderWithCore(SettingsNav, { page: general }, core(["me", "shared"]));
    await fireEvent.change(view.getByLabelText("設定する Store"), { target: { value: "shared" } });
    await view.rerender({ page: { page: "workspaces", store: "me" } });
    expect((view.getByLabelText("設定する Store") as HTMLSelectElement).value).toBe("me");
    await view.rerender({ page: general });
    expect((view.getByLabelText("設定する Store") as HTMLSelectElement).value).toBe("me");
  });

  it("同期と保守で Store を選ぶと、移らずに項目のリンク先の Store が変わる", async () => {
    history.replaceState(null, "", "/settings/general");
    const { getByLabelText, getByRole } = renderWithCore(
      SettingsNav,
      { page: general },
      core(["me", "shared"]),
    );
    const select = getByLabelText("設定する Store") as HTMLSelectElement;
    expect(select.value).toBe("me");
    await fireEvent.change(select, { target: { value: "shared" } });
    expect(location.pathname).toBe("/settings/general");
    expect(getByRole("link", { name: /ワークスペース/ }).getAttribute("href")).toBe(
      "/settings/workspaces/shared",
    );
  });

  it("Store がなければ Store ごとの項目は出さない", () => {
    const { queryByRole } = renderWithCore(SettingsNav, { page: general }, core([]));
    expect(queryByRole("link", { name: /ワークスペース/ })).toBeNull();
    expect(queryByRole("link", { name: /同期と保守/ })).toBeTruthy();
  });

  it("Store が 1 つなら選択欄を出さない", () => {
    const { queryByLabelText } = renderWithCore(SettingsNav, { page }, core(["shared"]));
    expect(queryByLabelText("設定する Store")).toBeNull();
  });

  it("Store が 2 つ以上なら選択欄を出し、選ぶと Store を URL に持って移る", async () => {
    const { getByLabelText } = renderWithCore(SettingsNav, { page }, core(["shared", "org-b/ops"]));
    const select = getByLabelText("設定する Store") as HTMLSelectElement;
    expect(select.value).toBe("shared");
    expect([...select.options].map((o) => o.value)).toEqual(["shared", "org-b/ops"]);
    await fireEvent.change(select, { target: { value: "org-b/ops" } });
    expect(location.pathname).toBe("/settings/workspaces/org-b%2Fops");
  });

  it("脚注に GET /api/health の版を出す。読めなければ出さない", async () => {
    const ok = renderWithCore(SettingsNav, { page }, core(["shared"]));
    expect(await ok.findByText("Hedgehug 0.9.0")).toBeTruthy();
    cleanup();
    const bad = renderWithCore(
      SettingsNav,
      { page },
      core(
        ["shared"],
        vi.fn(async () => Promise.reject(new Error("down"))),
      ),
    );
    await vi.waitFor(() => expect(bad.core.api.GET).toHaveBeenCalled());
    expect(bad.queryByText(/^Hedgehug \d/)).toBeNull();
  });
});

describe("SettingsLayout", () => {
  it("左にナビ、右に本文を出す", () => {
    const children = createRawSnippet(() => ({ render: () => "<p>本文</p>" }));
    const { getByRole, getByText } = renderWithCore(
      SettingsLayout,
      { page, children },
      core(["shared"]),
    );
    expect(getByRole("navigation", { name: "設定のナビゲーション" })).toBeTruthy();
    expect(getByRole("region", { name: "設定の本文" })).toContainElement(getByText("本文"));
  });
});

describe("SettingsRoute", () => {
  it("Store がある /settings/workspaces/<store> はレイアウトを出し、移動しない", () => {
    const { getByRole } = renderWithCore(SettingsRoute, { page }, core(["shared", "me"]));
    expect(getByRole("navigation", { name: "設定のナビゲーション" })).toBeTruthy();
    expect(location.pathname).toBe("/settings/workspaces/shared");
  });

  it("存在しない Store の URL は見つからない旨とホームへの戻りを出し、レイアウトは出さない", async () => {
    history.replaceState(null, "", "/settings/workspaces/nope");
    const { getByText, getByRole, queryByRole } = renderWithCore(
      SettingsRoute,
      { page: { page: "workspaces", store: "nope" } },
      core(["shared"]),
    );
    expect(getByText("Store「nope」が見つかりません")).toBeTruthy();
    expect(queryByRole("navigation", { name: "設定のナビゲーション" })).toBeNull();
    await fireEvent.click(getByRole("button", { name: "ホームへ戻る" }));
    expect(location.pathname).toBe("/");
  });

  it("Store の一覧を読み終えるまでは、見つからないとは言わない", () => {
    history.replaceState(null, "", "/settings/workspaces/nope");
    const { queryByText } = renderWithCore(
      SettingsRoute,
      { page: { page: "workspaces", store: "nope" } },
      core([], undefined, false),
    );
    expect(queryByText(/見つかりません/)).toBeNull();
  });

  it("本文にその Store のワークスペースの一覧を出す", () => {
    const { getByRole, getByText } = renderWithCore(
      SettingsRoute,
      { page },
      core(["shared", "me"]),
    );
    expect(getByRole("region", { name: "設定の本文" })).toContainElement(
      getByText("workspaces of shared"),
    );
  });

  it("/settings/schema/<store> は、その Store のスキーマを本文に出す", () => {
    history.replaceState(null, "", "/settings/schema/shared");
    const { getByRole, getByText } = renderWithCore(
      SettingsRoute,
      { page: schemaPage },
      core(["shared", "me"]),
    );
    expect(getByRole("region", { name: "設定の本文" })).toContainElement(
      getByText("schema of shared (statuses)"),
    );
    expect(location.pathname).toBe("/settings/schema/shared");
  });

  it("スキーマのタブはページに渡す", () => {
    history.replaceState(null, "", "/settings/schema/shared?tab=kinds");
    const { getByText } = renderWithCore(
      SettingsRoute,
      { page: { page: "schema", store: "shared", tab: "kinds" } },
      core(["shared"]),
    );
    expect(getByText("schema of shared (kinds)")).toBeTruthy();
  });

  it("存在しない Store のスキーマの URL は見つからない旨を出す", () => {
    history.replaceState(null, "", "/settings/schema/nope");
    const { getByText, queryByRole } = renderWithCore(
      SettingsRoute,
      { page: { page: "schema", store: "nope", tab: "statuses" } },
      core(["shared"]),
    );
    expect(getByText("Store「nope」が見つかりません")).toBeTruthy();
    expect(queryByRole("navigation", { name: "設定のナビゲーション" })).toBeNull();
  });

  it("ワークスペースからスキーマへ移ってもナビは作り直さない", async () => {
    const view = renderWithCore(SettingsRoute, { page }, core(["me", "shared"]));
    const nav = view.getByRole("navigation", { name: "設定のナビゲーション" });
    await view.rerender({ page: schemaPage });
    expect(view.getByRole("navigation", { name: "設定のナビゲーション" })).toBe(nav);
    expect(view.getByText("schema of shared (statuses)")).toBeTruthy();
  });

  it("同期と保守は Store を持たずに、レイアウトと本文を出す", () => {
    history.replaceState(null, "", "/settings/general");
    const { getByRole, getByText } = renderWithCore(
      SettingsRoute,
      { page: general },
      core(["me", "shared"]),
    );
    expect(getByRole("region", { name: "設定の本文" })).toContainElement(getByText("general"));
    expect(location.pathname).toBe("/settings/general");
  });

  it.each([
    ["accounts", "アカウントと認証"],
    ["stores", "マウント中の Store"],
  ] as const)("/settings/%s は Store を持たずにレイアウトと見出しを出す", (name, heading) => {
    history.replaceState(null, "", `/settings/${name}`);
    const { getByRole } = renderWithCore(
      SettingsRoute,
      { page: { page: name } },
      core(["me", "shared"]),
    );
    expect(getByRole("region", { name: "設定の本文" })).toContainElement(
      getByRole("heading", { level: 1, name: heading }),
    );
    expect(getByRole("navigation", { name: "設定のナビゲーション" })).toBeTruthy();
    expect(location.pathname).toBe(`/settings/${name}`);
  });

  it("アカウントの見出しの説明は、トークンの置き場所を言う", () => {
    history.replaceState(null, "", "/settings/accounts");
    const { getByText } = renderWithCore(
      SettingsRoute,
      { page: { page: "accounts" } },
      core(["me"]),
    );
    expect(getByText(/トークンはこのマシンにだけ保存し、Store には書きません/)).toBeTruthy();
  });

  it("マウント中の Store の見出しの説明は、表示を外した Store の扱いを言う", () => {
    history.replaceState(null, "", "/settings/stores");
    const { getByText } = renderWithCore(SettingsRoute, { page: { page: "stores" } }, core(["me"]));
    expect(
      getByText(
        "このマシンで扱う Store です。見える範囲は各リポジトリのアクセス権で決まります。「表示」を外した Store のワークスペースは、サイドバーにもビューにも出しません。",
      ),
    ).toBeTruthy();
  });

  it("同期と保守は Store がまだ読めていなくても出す", () => {
    history.replaceState(null, "", "/settings/general");
    const { getByText } = renderWithCore(
      SettingsRoute,
      { page: general },
      core([], undefined, false),
    );
    expect(getByText("general")).toBeTruthy();
  });

  it("ワークスペースから同期と保守へ移ってもナビは作り直さない（Store の選択が残る）", async () => {
    const view = renderWithCore(SettingsRoute, { page }, core(["me", "shared"]));
    const nav = view.getByRole("navigation", { name: "設定のナビゲーション" });
    await view.rerender({ page: general });
    expect(view.getByRole("navigation", { name: "設定のナビゲーション" })).toBe(nav);
    expect((view.getByLabelText("設定する Store") as HTMLSelectElement).value).toBe("shared");
    expect(view.getByText("general")).toBeTruthy();
  });

  it("/settings/general にいるときは置き換えない", async () => {
    history.replaceState(null, "", "/settings/general");
    const replace = vi.spyOn(history, "replaceState");
    renderWithCore(SettingsRoute, { page: general }, core(["me"]));
    await Promise.resolve();
    expect(replace).not.toHaveBeenCalled();
  });

  it("/settings は /settings/general に置き換える（履歴を積まない）", async () => {
    history.replaceState(null, "", "/settings");
    const before = history.length;
    renderWithCore(SettingsRoute, { page: general }, core(["me", "shared"]));
    await vi.waitFor(() => expect(location.pathname).toBe("/settings/general"));
    expect(history.length).toBe(before);
  });
});
