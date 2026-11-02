import { beforeEach, describe, expect, it, vi } from "vitest";
import { hrefOf, onLinkClick, parseRoute, router } from "./router";

beforeEach(() => {
  history.replaceState(null, "", "/");
});

describe("parseRoute", () => {
  it.each([
    ["/", { name: "home" }],
    ["/workspaces/shared/01HWS", { name: "workspace", store: "shared", ulid: "01HWS" }],
    ["/workspaces/org-b%2Fops/01HWS", { name: "workspace", store: "org-b/ops", ulid: "01HWS" }],
    ["/projects/01HPR", { name: "project", ulid: "01HPR" }],
    ["/tasks/01HTK/", { name: "task", ulid: "01HTK" }],
    ["/nothing/here", { name: "home" }],
    ["/workspaces/shared", { name: "home" }],
  ])("%s", (path, route) => {
    expect(parseRoute(path)).toEqual(route);
  });
});

describe("hrefOf", () => {
  it("parseRoute と往復できる。Store 名の / は符号化する", () => {
    const r = { name: "workspace", store: "org-b/ops", ulid: "01HWS" } as const;
    expect(hrefOf(r)).toBe("/workspaces/org-b%2Fops/01HWS");
    expect(parseRoute(hrefOf(r))).toEqual(r);
    expect(hrefOf({ name: "home" })).toBe("/");
    expect(hrefOf({ name: "task", ulid: "01HTK" })).toBe("/tasks/01HTK");
  });
});

describe("router", () => {
  it("go は履歴に積んで route を変える", () => {
    router.go("/projects/01HPR");
    expect(location.pathname).toBe("/projects/01HPR");
    expect(router.route).toEqual({ name: "project", ulid: "01HPR" });
  });

  it("知らないパスは / に直す。replace なら履歴を積まない", () => {
    const before = history.length;
    router.go("/unknown", { replace: true });
    expect(location.pathname).toBe("/");
    expect(history.length).toBe(before);
  });

  it("クエリとハッシュは捨てる", () => {
    router.go("/projects/01HPR?x=1#y");
    expect(location.pathname).toBe("/projects/01HPR");
    expect(location.search).toBe("");
  });
});

describe("onLinkClick", () => {
  it("修飾キーなしの左クリックは router.go にする", () => {
    const e = new MouseEvent("click", { button: 0, cancelable: true });
    onLinkClick(e, "/projects/01HPR");
    expect(e.defaultPrevented).toBe(true);
    expect(location.pathname).toBe("/projects/01HPR");
  });

  it("⌘/Ctrl/⇧ クリックは window.open で開き、sessionStorage（トークン）を引き継ぐ", () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    for (const mod of [{ ctrlKey: true }, { metaKey: true }, { shiftKey: true }]) {
      const e = new MouseEvent("click", { button: 0, cancelable: true, ...mod });
      onLinkClick(e, "/projects/01HPR");
      expect(e.defaultPrevented).toBe(true);
    }
    expect(open).toHaveBeenCalledTimes(3);
    // No "noopener": a tab opened with it starts with an empty sessionStorage, and no token.
    expect(open).toHaveBeenCalledWith(`${location.origin}/projects/01HPR`, "_blank");
    expect(location.pathname).toBe("/");
    open.mockRestore();
  });

  it("Alt クリックと左以外のボタンはブラウザに任せる", () => {
    for (const init of [{ altKey: true, button: 0 }, { button: 1 }]) {
      const e = new MouseEvent("click", { cancelable: true, ...init });
      onLinkClick(e, "/projects/01HPR");
      expect(e.defaultPrevented).toBe(false);
    }
    expect(location.pathname).toBe("/");
  });
});
