import { cleanup, fireEvent } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SyncStatus } from "../../core/connection";
import { renderWithCore } from "../test-utils";
import Banners from "./Banners.svelte";
import Disconnected from "./Disconnected.svelte";
import NoToken from "./NoToken.svelte";
import Onboarding from "./Onboarding.svelte";

beforeEach(() => {
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("NoToken", () => {
  it("hh ui の URL からだけ使えると伝え、hh ui をコピーできる", () => {
    const { getByText, getByRole } = renderWithCore(NoToken, {});
    expect(
      getByText(
        "この画面は hh ui が開いた URL からだけ使えます。端末で hh ui を実行してください。",
      ),
    ).toBeTruthy();
    expect(getByRole("button", { name: /hh ui/ })).toBeTruthy();
  });
});

describe("Disconnected", () => {
  it("最後に接続していた時刻と、5 秒ごとに再接続していることを出す", () => {
    const at = new Date(2026, 10, 2, 11, 42).getTime();
    const { getByRole, getByText } = renderWithCore(Disconnected, { lastOpenAt: at });
    expect(getByRole("alert")).toBeTruthy();
    expect(getByText("Hedgehug のサーバーに接続できません")).toBeTruthy();
    expect(getByText(/表示中の内容は 11:42 時点のもので、編集はできません/)).toBeTruthy();
    expect(getByText("5 秒ごとに再接続しています")).toBeTruthy();
  });

  it("画面全体をおおう面で、ショートカットも止める（data-blocking）", () => {
    const { getByRole } = renderWithCore(Disconnected, { lastOpenAt: null });
    expect(getByRole("alert").hasAttribute("data-blocking")).toBe(true);
  });

  it("一度も接続できていなければ時刻を出さない", () => {
    const { queryByText, getByText } = renderWithCore(Disconnected, { lastOpenAt: null });
    expect(queryByText(/時点のもの/)).toBeNull();
    expect(getByText(/端末で hh ui を実行してください/)).toBeTruthy();
  });
});

describe("Banners", () => {
  const status = (over: Partial<SyncStatus> & { store: string }): SyncStatus => ({
    remotes: ["origin"],
    interval: 120,
    push_interval: 30,
    ...over,
  });
  const mount = (statuses: SyncStatus[]) =>
    renderWithCore(
      Banners,
      {},
      {
        conn: {
          state: "open",
          lastOpenAt: null,
          sync: new Map(statuses.map((s) => [s.store, s])),
          unread: 0,
          subscribe: () => () => {},
          on: () => () => {},
          start() {},
          stop() {},
        },
      },
    );

  it("同期済みなら何も出さない", () => {
    const { container } = mount([status({ store: "a", unpushed: 0 })]);
    expect(container.querySelector("[role='status'], [role='alert']")).toBeNull();
  });

  it("オフラインの帯", () => {
    vi.spyOn(navigator, "onLine", "get").mockReturnValue(false);
    const { getByText } = mount([status({ store: "a", unpushed: 5 })]);
    expect(
      getByText("オフラインです。変更はこのマシンにコミットし、つながったら push します。"),
    ).toBeTruthy();
  });

  it("push できない Store ごとに帯を出し、サーバーの文言を添える", () => {
    const { getAllByRole } = mount([
      status({ store: "org-a/hh-shared", unpushed: 1, last_error: "push rejected" }),
      status({ store: "me", unpushed: 0 }),
    ]);
    const bands = getAllByRole("alert");
    expect(bands).toHaveLength(1);
    expect(bands[0]?.textContent).toContain("org-a/hh-shared を push できません");
    expect(bands[0]?.textContent).toContain("push rejected");
    expect(bands[0]?.textContent).toContain("変更はこのマシンに残っています");
  });
});

describe("Onboarding", () => {
  it("ようこそと、Store を作るボタンを出し、押すと CreateStore が開く", async () => {
    const { getByText, getByRole, queryByRole } = renderWithCore(Onboarding, {});
    expect(getByText("Hedgehug へようこそ")).toBeTruthy();
    expect(getByRole("button", { name: /hh store init/ })).toBeTruthy();
    expect(queryByRole("dialog")).toBeNull();
    await fireEvent.click(getByRole("button", { name: "Store を作る" }));
    expect(getByRole("dialog", { name: "Store を作成" })).toBeTruthy();
  });
});
