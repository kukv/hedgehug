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
});
