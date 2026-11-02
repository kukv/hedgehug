import { cleanup, fireEvent } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SyncStatus } from "../../core/connection";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import SyncCard from "./SyncCard.svelte";

const status = (over: Partial<SyncStatus> & { store: string }): SyncStatus => ({
  remotes: ["origin"],
  ...over,
});

function mount(statuses: SyncStatus[], post = vi.fn()) {
  const conn = {
    state: "open",
    lastOpenAt: null,
    sync: new Map(statuses.map((s) => [s.store, s])),
    unread: 0,
    subscribe: () => () => {},
    on: () => () => {},
    start() {},
    stop() {},
  };
  return { post, ...renderWithCore(SyncCard, {}, { conn, api: { POST: post } }) };
}

beforeEach(() => {
  vi.useFakeTimers({
    toFake: ["setInterval", "clearInterval", "setTimeout", "clearTimeout", "Date"],
  });
  vi.setSystemTime(new Date("2026-11-02T09:00:30Z"));
  resetToasts();
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("SyncCard", () => {
  it("同期済み: 相対の最終同期・未 push・次の fetch のカウントダウン", () => {
    const { getByText } = mount([
      status({
        store: "a",
        unpushed: 0,
        last_success: "2026-11-02T09:00:00Z",
        next_fetch: "2026-11-02T09:01:42Z",
      }),
    ]);
    expect(getByText("同期済み")).toBeTruthy();
    expect(getByText("0 件")).toBeTruthy();
    expect(getByText("1:12")).toBeTruthy();
  });

  it("次の fetch は 1 秒ごとに進む", async () => {
    const { findByText } = mount([
      status({ store: "a", unpushed: 0, next_fetch: "2026-11-02T09:01:42Z" }),
    ]);
    vi.advanceTimersByTime(2000);
    expect(await findByText("1:10")).toBeTruthy();
  });

  it("カウントダウンを出さないときは interval を回さない", () => {
    const spy = vi.spyOn(globalThis, "setInterval");
    mount([status({ store: "a", unpushed: 0 })]);
    expect(spy).not.toHaveBeenCalled();
    cleanup();
    mount([status({ store: "a", unpushed: 0, next_fetch: "2026-11-02T09:01:42Z" })]);
    expect(spy).toHaveBeenCalledTimes(1);
  });

  it("送信待ち: 未 push のコミット数と、push までの秒数", () => {
    const { getByText } = mount([
      status({
        store: "a",
        unpushed: 3,
        pending_since: "2026-11-02T08:58:30Z",
        push_at: "2026-11-02T09:00:52Z",
      }),
    ]);
    expect(getByText("送信待ち")).toBeTruthy();
    expect(getByText("3 コミット")).toBeTruthy();
    expect(getByText("22 秒後に push")).toBeTruthy();
  });

  it("送信待ち: push の時刻が過ぎていれば「まもなく push」、1 分以上先は分で出す", () => {
    const a = mount([status({ store: "a", unpushed: 1, push_at: "2026-11-02T09:00:00Z" })]);
    expect(a.getByText("まもなく push")).toBeTruthy();
    cleanup();
    const b = mount([status({ store: "a", unpushed: 1, push_at: "2026-11-02T09:02:00Z" })]);
    expect(b.getByText("2 分後に push")).toBeTruthy();
  });

  it("push できません: Store 名と文言", () => {
    const { getByText } = mount([
      status({ store: "org-a/hh-shared", unpushed: 1, last_error: "push rejected" }),
    ]);
    expect(getByText("push できません")).toBeTruthy();
    expect(getByText("org-a/hh-shared: push rejected")).toBeTruthy();
    expect(getByText("1 コミット")).toBeTruthy();
  });

  it("オフライン: navigator.onLine が偽", () => {
    vi.spyOn(navigator, "onLine", "get").mockReturnValue(false);
    const { getByText } = mount([
      status({ store: "a", unpushed: 5, last_success: "2026-11-02T00:12:00Z" }),
    ]);
    expect(getByText("オフライン")).toBeTruthy();
    expect(getByText("5 コミット")).toBeTruthy();
  });

  it("今すぐ同期で POST /api/sync を呼び、結果をトーストに出す", async () => {
    const post = vi.fn().mockResolvedValue({
      data: [
        {
          store: "a",
          merges: 1,
          pushed: [],
          overwritten: [],
          repaired: [],
          invalid: [],
          notices: [],
          warnings: [],
        },
      ],
      response: new Response(),
    });
    const { getByRole } = mount([status({ store: "a", unpushed: 0 })], post);
    await fireEvent.click(getByRole("button", { name: /今すぐ同期/ }));
    await vi.waitFor(() => expect(toasts.list).toHaveLength(1));
    expect(post).toHaveBeenCalledWith("/api/sync");
    expect(toasts.list[0]?.message).toBe("同期しました · マージ 1 件");
  });

  it("⇧⌘S（Linux は Ctrl+Shift+S）でも同期する", async () => {
    vi.spyOn(navigator, "platform", "get").mockReturnValue("Linux x86_64");
    const post = vi.fn().mockResolvedValue({ data: [], response: new Response() });
    mount([status({ store: "a", unpushed: 0 })], post);
    await fireEvent.keyDown(window, { key: "S", ctrlKey: true, shiftKey: true });
    await vi.waitFor(() => expect(post).toHaveBeenCalledTimes(1));
  });

  it("同期の API が失敗したらエラーの文言をトーストに出す", async () => {
    const post = vi.fn().mockRejectedValue(new Error("boom"));
    const { getByRole } = mount([status({ store: "a", unpushed: 0 })], post);
    await fireEvent.click(getByRole("button", { name: /今すぐ同期/ }));
    await vi.waitFor(() => expect(toasts.list).toHaveLength(1));
    expect(toasts.list[0]?.message).toBe("操作に失敗しました: boom");
  });

  it("同期中はボタンを無効にする", async () => {
    let resolve!: (v: unknown) => void;
    const post = vi.fn().mockReturnValue(new Promise((r) => (resolve = r)));
    const { getByRole } = mount([status({ store: "a", unpushed: 0 })], post);
    await fireEvent.click(getByRole("button", { name: /今すぐ同期/ }));
    expect((getByRole("button", { name: /同期中/ }) as HTMLButtonElement).disabled).toBe(true);
    resolve({ data: [], response: new Response() });
  });
});
