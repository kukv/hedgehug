import { cleanup, render } from "@testing-library/svelte";
import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App.svelte";

// The screens of the new routes have their own tests: here each only says which one is drawn,
// and with what.
vi.mock("./views/ViewPage.svelte", () => ({
  default: (anchor: Comment, props: { store: string; name: string }) => {
    const p = document.createElement("p");
    p.textContent = `view page ${props.store}/${props.name}`;
    anchor.before(p);
  },
}));
vi.mock("./views/QueryPage.svelte", () => ({
  default: (anchor: Comment, props: { layout: string; q: string }) => {
    const p = document.createElement("p");
    p.textContent = `query page ${props.layout} ${props.q}`;
    anchor.before(p);
  },
}));
vi.mock("./list/ListTab.svelte", () => ({
  default: (anchor: Comment, props: { layout: string; view: { ulid: string } }) => {
    const p = document.createElement("p");
    p.textContent = `${props.layout} tab of ${props.view.ulid}`;
    anchor.before(p);
  },
}));

beforeEach(() => {
  vi.stubGlobal("fetch", () => new Promise(() => {}));
  sessionStorage.clear();
  history.replaceState(null, "", "/not/a/page#token=tkn");
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("App", () => {
  it("本文の領域を出し、知らないパスを / に直し、ハッシュのトークンを消す", () => {
    const { container } = render(App);
    flushSync();
    expect(container.querySelector("main")).toBeTruthy();
    expect(location.pathname).toBe("/");
    expect(location.hash).toBe("");
    expect(sessionStorage.getItem("hedgehug.token")).toBe("tkn");
  });

  it("設定の画面ではサイドバーを隠してナビに替える（サイドバーは残す）", () => {
    history.replaceState(null, "", "/settings/workspaces/shared#token=tkn");
    const { container } = render(App);
    flushSync();
    expect(container.querySelector("aside.side")).toHaveAttribute("hidden");
    expect(container.querySelector("main")).toHaveClass("flush");
    expect(location.pathname).toBe("/settings/workspaces/shared");
  });

  it("/my はマイタスクの画面", () => {
    history.replaceState(null, "", "/my#token=tkn");
    const { getByRole } = render(App);
    flushSync();
    expect(getByRole("heading", { level: 1, name: "マイタスク" })).toBeTruthy();
    expect(location.pathname).toBe("/my");
  });

  it("/inbox は通知の画面", () => {
    history.replaceState(null, "", "/inbox#token=tkn");
    const { getByRole } = render(App);
    flushSync();
    expect(getByRole("heading", { level: 1, name: "通知" })).toBeTruthy();
    expect(location.pathname).toBe("/inbox");
  });

  it("/activity はアクティビティの画面。絞り込みを残す", () => {
    history.replaceState(null, "", "/activity?workspace=W1&sync=only#token=tkn");
    const { getByRole } = render(App);
    flushSync();
    expect(getByRole("heading", { level: 1, name: "アクティビティ" })).toBeTruthy();
    expect(getByRole("button", { name: "同期" }).getAttribute("aria-pressed")).toBe("true");
    expect(location.pathname + location.search).toBe("/activity?workspace=W1&sync=only");
  });

  it("/views/<Store>/<名前> はビューの画面。名前は URL から戻して渡す", () => {
    history.replaceState(null, "", "/views/org-b%2Fops/%E4%BB%8A%E9%80%B1#token=tkn");
    const { getByText } = render(App);
    flushSync();
    expect(getByText("view page org-b/ops/今週")).toBeTruthy();
  });

  it("/query/<レイアウト>?q= はクエリの画面。q を残す", () => {
    history.replaceState(null, "", "/query/table?q=status%3Adoing+assignee%3A%40me#token=tkn");
    const { getByText } = render(App);
    flushSync();
    expect(getByText("query page table status:doing assignee:@me")).toBeTruthy();
    expect(location.pathname + location.search).toBe(
      "/query/table?q=status%3Adoing+assignee%3A%40me",
    );
  });

  it("プロジェクトのカンバン・テーブルのタブは、そのレイアウトの画面", () => {
    history.replaceState(null, "", "/projects/01HPR/kanban#token=tkn");
    const first = render(App);
    flushSync();
    expect(first.getByText("kanban tab of 01HPR")).toBeTruthy();
    first.unmount();
    history.replaceState(null, "", "/projects/01HPR/table#token=tkn");
    const second = render(App);
    flushSync();
    expect(second.getByText("table tab of 01HPR")).toBeTruthy();
  });

  it("設定以外の画面ではサイドバーを出す", () => {
    history.replaceState(null, "", "/tasks/01HTK#token=tkn");
    const { container } = render(App);
    flushSync();
    expect(container.querySelector("aside.side")).not.toHaveAttribute("hidden");
  });
});
