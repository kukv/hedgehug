import { cleanup, fireEvent } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithCore } from "../test-utils";
import SettingsLayout from "./SettingsLayout.svelte";
import SettingsNav from "./SettingsNav.svelte";
import SettingsRoute from "./SettingsRoute.svelte";

const health = (version: string) => ({
  data: { pid: 1, version },
  response: new Response(null, { status: 200 }),
});

function core(names: string[], get = vi.fn(async () => health("0.9.0"))) {
  return {
    api: { GET: get },
    stores: {
      stores: () => names.map((name) => ({ name })),
      subscribe: () => () => {},
    },
  };
}

const page = { page: "workspaces", store: "shared" } as const;

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

  it("まだない画面の項目（同期と保守・スキーマ・アカウントなど）は出さない", () => {
    const { queryByText } = renderWithCore(SettingsNav, { page }, core(["shared"]));
    for (const t of ["同期と保守", "スキーマ", "アカウント", "自動化", "マウント中の Store"]) {
      expect(queryByText(new RegExp(t))).toBeNull();
    }
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

  it("/settings は最初の Store のワークスペースに置き換える（履歴を積まない）", async () => {
    history.replaceState(null, "", "/settings");
    const before = history.length;
    const { queryByRole } = renderWithCore(
      SettingsRoute,
      { page: { page: "workspaces", store: "" } },
      core(["me", "shared"]),
    );
    await vi.waitFor(() => expect(location.pathname).toBe("/settings/workspaces/me"));
    expect(history.length).toBe(before);
    // while it has no Store the layout is not drawn
    expect(queryByRole("navigation", { name: "設定のナビゲーション" })).toBeNull();
  });

  it("Store がまだ読めていない間は /settings のまま待つ", async () => {
    history.replaceState(null, "", "/settings");
    renderWithCore(SettingsRoute, { page: { page: "workspaces", store: "" } }, core([]));
    await new Promise((r) => setTimeout(r, 20));
    expect(location.pathname).toBe("/settings");
  });
});
