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

describe("App のゲート", () => {
  it("トークンがなければトークンの画面を出す", () => {
    const { getByText } = render(App);
    flushSync();
    expect(
      getByText(
        "この画面は hh ui が開いた URL からだけ使えます。端末で hh ui を実行してください。",
      ),
    ).toBeTruthy();
  });

  it("トークンがあればトークンの画面は出さない", () => {
    history.replaceState(null, "", "/#token=tkn");
    const { queryByText } = render(App);
    flushSync();
    expect(queryByText(/hh ui が開いた URL からだけ/)).toBeNull();
  });
});
