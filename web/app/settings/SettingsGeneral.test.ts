import { cleanup, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SyncStatus } from "../../core/connection";
import { renderWithCore } from "../test-utils";
import SettingsGeneral from "./SettingsGeneral.svelte";

// DoctorSection and PurgeSection have their own tests: here they only say they were drawn, and for what.
vi.mock("./DoctorSection.svelte", () => ({
  default: (anchor: Comment, props: { store: string; many: boolean }) => {
    const p = document.createElement("p");
    p.textContent = `doctor of ${props.store} many=${props.many}`;
    anchor.before(p);
  },
}));
vi.mock("./PurgeSection.svelte", () => ({
  default: (anchor: Comment, props: { stores: string[] }) => {
    const p = document.createElement("p");
    p.textContent = `purge of ${props.stores.join(",")}`;
    anchor.before(p);
  },
}));

const status = (over: Partial<SyncStatus> & { store: string }): SyncStatus => ({
  remotes: ["origin"],
  interval: 120,
  push_interval: 30,
  unpushed: 0,
  ...over,
});

function mount(statuses: SyncStatus[], names = statuses.map((s) => s.store)) {
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
  const stores = {
    stores: () => names.map((name) => ({ name })),
    readOnly: () => null,
    tone: () => "teal",
    subscribe: () => () => {},
  };
  return renderWithCore(SettingsGeneral, {}, { conn, stores, api: {} });
}

beforeEach(() => {
  vi.useFakeTimers({
    toFake: ["setInterval", "clearInterval", "setTimeout", "clearTimeout", "Date"],
  });
  vi.setSystemTime(new Date("2026-11-02T09:00:30Z"));
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe("SettingsGeneral", () => {
  it("見出しと、このマシンだけの設定であることの説明（パスは出さない）", () => {
    const { getByRole, getByText, container } = mount([status({ store: "a" })]);
    expect(getByRole("heading", { level: 1, name: "同期と保守" })).toBeTruthy();
    expect(
      getByText("このマシンだけの設定です（Hedgehug の設定ファイル config.yaml の sync）"),
    ).toBeTruthy();
    expect(container.textContent).not.toContain("~/.config");
  });

  it("同期のタイミング: push は push_interval 秒後、fetch は interval 秒（表示中）と 10 分（非表示中）", () => {
    const { getByRole } = mount([status({ store: "a", interval: 120, push_interval: 30 })]);
    const sec = getByRole("region", { name: "同期のタイミング" });
    expect(within(sec).getByText(/書き込みの 30 秒後にまとめて push/)).toBeTruthy();
    expect(within(sec).getByText(/表示中は 2 分ごと/)).toBeTruthy();
    expect(within(sec).getByText(/非表示の間は 10 分ごと/)).toBeTruthy();
    expect(sec.textContent).toContain("sync.interval");
    expect(sec.textContent).toContain("sync.push_interval");
    expect(sec.textContent).toContain("hh ui");
  });

  it("非表示の間の fetch は max(interval, 10 分)。interval が長ければそれ", () => {
    const { getByText, getAllByText } = mount([status({ store: "a", interval: 900 })]);
    expect(getByText(/表示中は 15 分ごと/)).toBeTruthy();
    expect(getAllByText(/非表示の間は 15 分ごと/)).toHaveLength(1);
  });

  it("間隔は最初の Store の値を1つだけ出す", () => {
    const { getAllByText } = mount([
      status({ store: "a", interval: 90, push_interval: 10 }),
      status({ store: "b", interval: 300, push_interval: 60 }),
    ]);
    expect(getAllByText(/表示中は 1 分 30 秒ごと/)).toHaveLength(1);
  });

  it("同期の状態は Store ごとに段を分け、それぞれの状態と「今すぐ同期」を持つ", () => {
    const { getByRole } = mount([
      status({ store: "a", unpushed: 0 }),
      status({ store: "b", unpushed: 3, pending_since: "2026-11-02T08:58:30Z" }),
    ]);
    const a = getByRole("region", { name: "Store: a" });
    const b = getByRole("region", { name: "Store: b" });
    expect(within(a).getByText("同期済み")).toBeTruthy();
    expect(within(b).getByText("送信待ち")).toBeTruthy();
    expect(within(b).getByText("3 コミット")).toBeTruthy();
    expect(within(a).getByRole("button", { name: /今すぐ同期/ })).toBeTruthy();
    expect(within(b).getByRole("button", { name: /今すぐ同期/ })).toBeTruthy();
  });

  it("足元に hh sync を出す。--store は Store が2つ以上のときだけ", () => {
    const one = mount([status({ store: "a" })]);
    expect(one.getByRole("button", { name: "コマンドをコピー: hh sync" })).toBeTruthy();
    cleanup();
    const two = mount([status({ store: "a" }), status({ store: "b" })]);
    const b = two.getByRole("region", { name: "Store: b" });
    expect(
      within(b).getByRole("button", { name: "コマンドをコピー: hh sync --store b" }),
    ).toBeTruthy();
  });

  it("Store が2つ以上のときだけ Store のバッジを付ける", () => {
    const one = mount([status({ store: "a" })]);
    expect(one.container.querySelector(".badge")).toBeNull();
    cleanup();
    const two = mount([status({ store: "a" }), status({ store: "b" })]);
    expect(two.container.querySelectorAll(".badge")).toHaveLength(2);
  });

  it("同期の状態をまだ受け取っていないときはそう言う", () => {
    const { getByText, queryByRole } = mount([], ["a"]);
    expect(getByText("同期の状態をまだ受け取っていません")).toBeTruthy();
    expect(queryByRole("region", { name: "Store: a" })).toBeNull();
  });

  it("診断と修復は Store ごとの段で、削除済みタスクの完全削除は Store の一覧を渡して1つ", () => {
    const { getByRole, getByText, queryByText } = mount([status({ store: "a" })], ["a", "b"]);
    const doctor = getByRole("region", { name: "診断と修復" });
    expect(within(doctor).getByText("doctor of a many=true")).toBeTruthy();
    expect(within(doctor).getByText("doctor of b many=true")).toBeTruthy();
    const purge = getByRole("region", { name: "削除済みタスクの完全削除" });
    expect(within(purge).getByText("purge of a,b")).toBeTruthy();
    expect(getByText("purge of a,b")).toBeTruthy();
    expect(queryByText("doctor of a many=false")).toBeNull();
  });

  it("Store が1つのときは many=false", () => {
    const { getByText } = mount([status({ store: "a" })]);
    expect(getByText("doctor of a many=false")).toBeTruthy();
    expect(getByText("purge of a")).toBeTruthy();
  });
});
