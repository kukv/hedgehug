import { cleanup, fireEvent, render } from "@testing-library/svelte";
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

  it("ホーム / はマイタスクを開く（URL は / のまま）", async () => {
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
    const { findByRole } = render(App);
    expect(await findByRole("heading", { level: 1, name: "マイタスク" })).toBeTruthy();
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

  it("Onboarding は Store が増えても、始めるまで出し続ける", async () => {
    history.replaceState(null, "", "/#token=tkn");
    let mounted = false;
    vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : (input as Request).url;
      if (url.includes("/api/events")) return new Promise(() => {});
      if (url.includes("/api/accounts")) {
        return json([
          { name: "me", id: "github.com/me", state: "ok", login: "me", token: "keychain" },
        ]);
      }
      if (url.includes("/api/stores/discover")) {
        return json([
          {
            name: "hh-a",
            url: "https://github.com/org/hh-a.git",
            account: "me",
            permission: "write",
          },
        ]);
      }
      if (url.includes("/api/stores/mount")) {
        mounted = true;
        return json({ mounted: ["hh-a"], failed: [] });
      }
      if (url.includes("/api/workspaces")) return json([]);
      if (url.includes("/api/stores")) {
        return json(mounted ? [{ name: "hh-a", path: "/p", remotes: ["u"] }] : []);
      }
      return new Promise(() => {});
    });
    const { findByText, findByLabelText, getByRole, queryByText } = render(App);
    await findByText("Hedgehug へようこそ");
    await fireEvent.click(await findByText("次へ: Store を探す"));
    await fireEvent.click(await findByLabelText("org/hh-a を選ぶ"));
    await fireEvent.click(getByRole("button", { name: "マウントして次へ" }));
    expect(await findByText("Store を使い始めます")).toBeTruthy();
    // the Store is mounted and listed, and the Onboarding is still here
    await new Promise((r) => setTimeout(r, 50));
    expect(queryByText("Store を使い始めます")).toBeTruthy();
    await fireEvent.click(getByRole("button", { name: "始める" }));
    expect(queryByText("Hedgehug へようこそ")).toBeNull();
  });

  // A fetch for the Onboarding: the Stores there are now, and an event stream the test writes to.
  function onboardingApi() {
    const state = { stores: [] as unknown[], created: false };
    let events: ReadableStreamDefaultController<Uint8Array> | null = null;
    const enc = new TextEncoder();
    vi.stubGlobal("fetch", async (input: RequestInfo | URL, init?: RequestInit) => {
      const req = input as Request;
      const url = typeof input === "string" ? input : req.url;
      const method = (init?.method ?? req.method ?? "GET").toUpperCase();
      if (url.includes("/api/events")) {
        return new Response(
          new ReadableStream<Uint8Array>({
            start(c) {
              events = c;
            },
          }),
          { headers: { "Content-Type": "text/event-stream" } },
        );
      }
      if (url.includes("/api/accounts")) {
        return json([
          { name: "me", id: "github.com/me", state: "ok", login: "me", token: "keychain" },
        ]);
      }
      if (url.includes("/api/stores/discover")) {
        return json([
          {
            name: "hh-a",
            url: "https://github.com/org/hh-a.git",
            account: "me",
            permission: "write",
          },
        ]);
      }
      if (url.includes("/api/stores/mount")) {
        state.stores = [{ name: "hh-a", path: "/p", remotes: ["u"] }];
        return json({ mounted: ["hh-a"], failed: [] });
      }
      if (url.includes("/api/workspaces")) return json([]);
      if (url.includes("/api/stores") && method === "POST") {
        state.stores = [{ name: "me", path: "/p", remotes: [] }];
        return json(
          {
            name: "me",
            path: "/p",
            workspace: { ulid: "W1", name: "me", id_prefix: "ME", prefix_set: "", tasks: 0 },
            remotes: [],
            pushed: [],
          },
          201,
        );
      }
      if (url.includes("/api/stores")) return json(state.stores);
      return new Promise(() => {});
    });
    return {
      state,
      changed: () => events?.enqueue(enc.encode("event: stores.changed\ndata: {}\n\n")),
    };
  }

  it("Store が 0 件に戻ったら、始めたあとでも Onboarding がまた出る", async () => {
    history.replaceState(null, "", "/#token=tkn");
    const api = onboardingApi();
    const { findByText, findByLabelText, getByRole, queryByText } = render(App);
    await findByText("Hedgehug へようこそ");
    await fireEvent.click(await findByText("次へ: Store を探す"));
    await fireEvent.click(await findByLabelText("org/hh-a を選ぶ"));
    await fireEvent.click(getByRole("button", { name: "マウントして次へ" }));
    await findByText("Store を使い始めます");
    await fireEvent.click(getByRole("button", { name: "始める" }));
    await vi.waitFor(() => expect(queryByText("Hedgehug へようこそ")).toBeNull());
    // the last Store is unmounted somewhere else
    api.state.stores = [];
    api.changed();
    expect(await findByText("Hedgehug へようこそ")).toBeTruthy();
    // and it begins from the first step
    expect(getByRole("heading", { name: "GitHub のアカウントを登録します" })).toBeTruthy();
  });

  it("CreateStore で作り終えたら Onboarding を閉じる", async () => {
    history.replaceState(null, "", "/#token=tkn");
    onboardingApi();
    const { findByText, findByLabelText, getByRole, queryByText } = render(App);
    await findByText("Hedgehug へようこそ");
    await fireEvent.click(getByRole("button", { name: "新しく Store を作る" }));
    await fireEvent.input(await findByLabelText("名前"), { target: { value: "me" } });
    await fireEvent.input(getByRole("textbox", { name: "ID の接頭辞" }), {
      target: { value: "me" },
    });
    await fireEvent.click(getByRole("button", { name: "Store を作る" }));
    await vi.waitFor(() => expect(queryByText("Hedgehug へようこそ")).toBeNull());
  });
});
