import { cleanup, render } from "@testing-library/svelte";
import { flushSync } from "svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App.svelte";

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

  it("設定以外の画面ではサイドバーを出す", () => {
    history.replaceState(null, "", "/tasks/01HTK#token=tkn");
    const { container } = render(App);
    flushSync();
    expect(container.querySelector("aside.side")).not.toHaveAttribute("hidden");
  });
});
