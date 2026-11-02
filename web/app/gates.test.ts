import { cleanup, render } from "@testing-library/svelte";
import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App.svelte";

beforeEach(() => {
  vi.stubGlobal("fetch", () => new Promise(() => {}));
  sessionStorage.clear();
  history.replaceState(null, "", "/");
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

// jsdom does not know inert, so Svelte sets it as a plain property there; a browser reflects it as an attribute.
const isInert = (el: Element | null) =>
  el !== null && (el.hasAttribute("inert") || (el as Element & { inert?: boolean }).inert === true);

describe("App のゲート", () => {
  it("トークンがなければトークンの画面を出す", async () => {
    const { getByText } = render(App);
    flushSync();
    expect(
      getByText(
        "この画面は hh ui が開いた URL からだけ使えます。端末で hh ui を実行してください。",
      ),
    ).toBeTruthy();
  });

  it("トークンがあればトークンの画面は出さない", async () => {
    history.replaceState(null, "", "/#token=tkn");
    const { queryByText } = render(App);
    flushSync();
    expect(queryByText(/hh ui が開いた URL からだけ/)).toBeNull();
  });

  const json = (body: unknown, status = 200) =>
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": status >= 400 ? "application/problem+json" : "application/json" },
    });
  const NO_TOKEN = /この画面は hh ui が開いた URL からだけ/;

  it("API が 401 auth.token を返したら、トークンの画面に切り替え、背後を触れなくする", async () => {
    history.replaceState(null, "", "/#token=tkn");
    vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      if (url.includes("/api/events")) return new Promise(() => {});
      return json({ type: "auth.token", status: 401, title: "unauthorized" }, 401);
    });
    const { findByText, container } = render(App);
    expect(await findByText(NO_TOKEN)).toBeTruthy();
    expect(isInert(container.querySelector(".app"))).toBe(true);
  });

  it("/api/events が 401 auth.token を返しても、トークンの画面に切り替え、再接続しない", async () => {
    history.replaceState(null, "", "/#token=tkn");
    let events = 0;
    vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      if (url.includes("/api/events")) {
        events++;
        return json({ type: "auth.token", status: 401, title: "unauthorized" }, 401);
      }
      return new Promise(() => {});
    });
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      const { findByText } = render(App);
      expect(await findByText(NO_TOKEN)).toBeTruthy();
      await vi.advanceTimersByTimeAsync(30_000);
      expect(events).toBe(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("サーバーに接続できなければ Disconnected を出し、背後を触れなくする", async () => {
    history.replaceState(null, "", "/#token=tkn");
    vi.stubGlobal("fetch", async () => {
      throw new TypeError("Failed to fetch");
    });
    const { findByText, container } = render(App);
    expect(await findByText("Hedgehug のサーバーに接続できません")).toBeTruthy();
    expect(isInert(container.querySelector(".app"))).toBe(true);
  });

  it("Store の一覧が読めるまでは Onboarding を出さず、Store が 0 件だと出す", async () => {
    history.replaceState(null, "", "/#token=tkn");
    let release: (r: Response) => void = () => {};
    vi.stubGlobal("fetch", (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      if (url.includes("/api/stores")) return new Promise<Response>((r) => (release = r));
      return new Promise(() => {});
    });
    const { queryByText, findByText } = render(App);
    flushSync();
    await Promise.resolve();
    expect(queryByText("Hedgehug へようこそ")).toBeNull();
    release(json([]));
    expect(await findByText("Hedgehug へようこそ")).toBeTruthy();
  });

  it("ホーム / は最初のワークスペースの概要を開く（URL は / のまま）", async () => {
    history.replaceState(null, "", "/#token=tkn");
    vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      if (url.includes("/api/events")) return new Promise(() => {});
      if (url.includes("/api/workspaces")) {
        return json([
          {
            ulid: "01HWSME",
            name: "me",
            id_prefix: "ME",
            prefix_set: "2026-11-02T00:00:00Z",
            tasks: 0,
            deleted: 0,
          },
        ]);
      }
      if (url.includes("/api/stores")) {
        return json([{ name: "me", path: "/p", remotes: [] }]);
      }
      return new Promise(() => {});
    });
    const { findByRole, getByRole } = render(App);
    expect(await findByRole("heading", { level: 1, name: "me" })).toBeTruthy();
    expect(
      getByRole("navigation", { name: "ビューの切り替え" }).querySelector("[aria-current]")
        ?.textContent,
    ).toContain("概要");
    expect(location.pathname).toBe("/");
  });

  it("Store はあるのにワークスペースが 0 件なら、空の状態と Store ごとの hh コマンドを出す", async () => {
    history.replaceState(null, "", "/#token=tkn");
    vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      if (url.includes("/api/events")) return new Promise(() => {});
      if (url.includes("/api/workspaces")) return json([]);
      if (url.includes("/api/stores")) {
        return json([
          { name: "me", path: "/p", remotes: [] },
          { name: "team", path: "/q", remotes: [] },
        ]);
      }
      return new Promise(() => {});
    });
    const { findByText, getByRole, queryByText } = render(App);
    expect(await findByText("ワークスペースがありません")).toBeTruthy();
    expect(getByRole("button", { name: /--store me/ })).toBeTruthy();
    expect(getByRole("button", { name: /--store team/ })).toBeTruthy();
    expect(queryByText("Hedgehug へようこそ")).toBeNull();
  });
});
