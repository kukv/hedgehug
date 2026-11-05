import { beforeEach, describe, expect, it, vi } from "vitest";
import { hrefOf, onLinkClick, parseRoute, type Route, router } from "./router";

beforeEach(() => {
  history.replaceState(null, "", "/");
});

describe("parseRoute", () => {
  it.each([
    ["/", { name: "home" }],
    [
      "/workspaces/shared/01HWS",
      { name: "workspace", store: "shared", ulid: "01HWS", tab: "overview" },
    ],
    [
      "/workspaces/shared/01HWS/",
      { name: "workspace", store: "shared", ulid: "01HWS", tab: "overview" },
    ],
    [
      "/workspaces/shared/01HWS/tree",
      { name: "workspace", store: "shared", ulid: "01HWS", tab: "tree" },
    ],
    [
      "/workspaces/shared/01HWS/settings",
      { name: "workspace", store: "shared", ulid: "01HWS", tab: "settings" },
    ],
    [
      "/workspaces/org-b%2Fops/01HWS/tree",
      { name: "workspace", store: "org-b/ops", ulid: "01HWS", tab: "tree" },
    ],
    [
      "/workspaces/shared/01HWS/kanban",
      { name: "workspace", store: "shared", ulid: "01HWS", tab: "kanban" },
    ],
    [
      "/workspaces/shared/01HWS/table",
      { name: "workspace", store: "shared", ulid: "01HWS", tab: "table" },
    ],
    [
      "/workspaces/shared/01HWS/gantt",
      { name: "workspace", store: "shared", ulid: "01HWS", tab: "gantt" },
    ],
    [
      "/workspaces/shared/01HWS/reports",
      { name: "workspace", store: "shared", ulid: "01HWS", tab: "reports" },
    ],
    ["/projects/01HPJ/gantt", { name: "project", ulid: "01HPJ", tab: "gantt" }],
    ["/projects/01HPJ/reports", { name: "home" }],
    ["/workspaces/shared/01HWS/tree/extra", { name: "home" }],
    ["/projects/01HPR", { name: "project", ulid: "01HPR", tab: "tree" }],
    ["/projects/01HPR/tree", { name: "project", ulid: "01HPR", tab: "tree" }],
    ["/projects/01HPR/kanban", { name: "project", ulid: "01HPR", tab: "kanban" }],
    ["/projects/01HPR/table", { name: "project", ulid: "01HPR", tab: "table" }],
    ["/projects/01HPR/settings", { name: "project", ulid: "01HPR", tab: "settings" }],
    ["/projects/01HPR/nope", { name: "home" }],
    ["/views/shared/%E4%BB%8A%E9%80%B1", { name: "view", store: "shared", view: "今週" }],
    ["/views/org-b%2Fops/weekly", { name: "view", store: "org-b/ops", view: "weekly" }],
    ["/views/shared", { name: "home" }],
    ["/views/shared/weekly/extra", { name: "home" }],
    ["/query/table", { name: "query", layout: "table", q: "" }],
    ["/query/kanban/", { name: "query", layout: "kanban", q: "" }],
    ["/query/gantt", { name: "query", layout: "gantt", q: "" }],
    ["/query/timeline", { name: "home" }],
    ["/query", { name: "home" }],
    ["/my", { name: "my" }],
    ["/my/extra", { name: "home" }],
    ["/inbox", { name: "inbox", tab: "all" }],
    ["/inbox/", { name: "inbox", tab: "all" }],
    ["/inbox/extra", { name: "home" }],
    ["/projects/P1/overview", { name: "project", ulid: "P1", tab: "overview" }],
    ["/activity", { name: "activity", workspaces: [], sync: "" }],
    ["/activity/", { name: "activity", workspaces: [], sync: "" }],
    ["/activity/extra", { name: "home" }],
    ["/tasks/01HTK/", { name: "task", ulid: "01HTK" }],
    ["/settings", { name: "settings", page: "general" }],
    ["/settings/general", { name: "settings", page: "general" }],
    ["/settings/general/extra", { name: "home" }],
    ["/settings/workspaces/shared", { name: "settings", page: "workspaces", store: "shared" }],
    ["/settings/automations/shared", { name: "settings", page: "automations", store: "shared" }],
    [
      "/settings/workspaces/org-b%2Fops",
      { name: "settings", page: "workspaces", store: "org-b/ops" },
    ],
    [
      "/settings/schema/shared",
      { name: "settings", page: "schema", store: "shared", tab: "statuses" },
    ],
    [
      "/settings/schema/org-b%2Fops",
      { name: "settings", page: "schema", store: "org-b/ops", tab: "statuses" },
    ],
    ["/settings/accounts", { name: "settings", page: "accounts" }],
    ["/settings/stores", { name: "settings", page: "stores" }],
    ["/settings/accounts/extra", { name: "home" }],
    ["/settings/stores/extra", { name: "home" }],
    ["/settings/schema", { name: "home" }],
    ["/settings/schema/shared/extra", { name: "home" }],
    ["/nothing/here", { name: "home" }],
    ["/workspaces/shared", { name: "home" }],
  ])("%s", (path, route) => {
    expect(parseRoute(path)).toEqual(route);
  });

  it("/query/<layout> は search の q を読む。ほかのパスは search を捨てる", () => {
    expect(parseRoute("/query/table", "?q=status%3Atodo%20assignee%3A%40me")).toEqual({
      name: "query",
      layout: "table",
      q: "status:todo assignee:@me",
    });
    expect(parseRoute("/query/tree", "?x=1")).toEqual({ name: "query", layout: "tree", q: "" });
    expect(parseRoute("/my", "?q=a")).toEqual({ name: "my" });
    expect(parseRoute("/inbox", "?q=a")).toEqual({ name: "inbox", tab: "all" });
    expect(parseRoute("/projects/01HPR/table", "?q=a")).toEqual({
      name: "project",
      ulid: "01HPR",
      tab: "table",
    });
  });
});

describe("hrefOf", () => {
  it("parseRoute と往復できる。Store 名の / は符号化する", () => {
    const r = { name: "workspace", store: "org-b/ops", ulid: "01HWS", tab: "overview" } as const;
    expect(hrefOf(r)).toBe("/workspaces/org-b%2Fops/01HWS");
    expect(parseRoute(hrefOf(r))).toEqual(r);
    expect(hrefOf({ name: "home" })).toBe("/");
    expect(hrefOf({ name: "task", ulid: "01HTK" })).toBe("/tasks/01HTK");
    expect(hrefOf({ name: "inbox", tab: "all" })).toBe("/inbox");
    expect(parseRoute(hrefOf({ name: "inbox", tab: "all" }))).toEqual({
      name: "inbox",
      tab: "all",
    });
  });

  it("アクティビティは ?workspace=（ULID のカンマ区切り）と ?sync= を持つ", () => {
    expect(parseRoute("/activity", "?workspace=A,B&sync=only")).toEqual({
      name: "activity",
      workspaces: ["A", "B"],
      sync: "only",
    });
    expect(parseRoute("/activity", "?workspace=A%2CB&sync=exclude")).toEqual({
      name: "activity",
      workspaces: ["A", "B"],
      sync: "exclude",
    });
    // a sync that is none of the two is "all", and empty items of the list are dropped
    expect(parseRoute("/activity", "?sync=automation")).toEqual({
      name: "activity",
      workspaces: [],
      sync: "automation",
    });
    expect(parseRoute("/activity", "?workspace=A,,&sync=x")).toEqual({
      name: "activity",
      workspaces: ["A"],
      sync: "",
    });
    expect(hrefOf({ name: "activity", workspaces: [], sync: "" })).toBe("/activity");
    expect(hrefOf({ name: "activity", workspaces: ["A", "B"], sync: "" })).toBe(
      "/activity?workspace=A,B",
    );
    expect(hrefOf({ name: "activity", workspaces: [], sync: "only" })).toBe("/activity?sync=only");
    const r: Route = { name: "activity", workspaces: ["A", "B"], sync: "exclude" };
    expect(hrefOf(r)).toBe("/activity?workspace=A,B&sync=exclude");
    expect(parseRoute("/activity", "?workspace=A,B&sync=exclude")).toEqual(r);
  });

  it("タブはパスの末尾に付ける。概要は付けない", () => {
    const ws = { name: "workspace", store: "shared", ulid: "01HWS" } as const;
    expect(hrefOf({ ...ws, tab: "tree" })).toBe("/workspaces/shared/01HWS/tree");
    expect(hrefOf({ ...ws, tab: "settings" })).toBe("/workspaces/shared/01HWS/settings");
    expect(hrefOf({ name: "project", ulid: "01HPR", tab: "tree" })).toBe("/projects/01HPR");
    for (const r of [
      { ...ws, tab: "tree" },
      { ...ws, tab: "settings" },
      { name: "project", ulid: "01HPR", tab: "tree" },
      { name: "settings", page: "workspaces", store: "org-b/ops" },
      { name: "settings", page: "schema", store: "org-b/ops", tab: "statuses" },
      { name: "inbox", tab: "all" },
      { name: "settings", page: "general" },
    ] as const) {
      expect(parseRoute(hrefOf(r))).toEqual(r);
    }
  });

  it("プロジェクトとワークスペースのカンバン・テーブルのパス。ツリーだけ末尾を付けない", () => {
    expect(hrefOf({ name: "project", ulid: "01HPR", tab: "kanban" })).toBe(
      "/projects/01HPR/kanban",
    );
    expect(hrefOf({ name: "project", ulid: "01HPR", tab: "table" })).toBe("/projects/01HPR/table");
    expect(hrefOf({ name: "project", ulid: "01HPR", tab: "settings" })).toBe(
      "/projects/01HPR/settings",
    );
    const ws = { name: "workspace", store: "shared", ulid: "01HWS" } as const;
    expect(hrefOf({ ...ws, tab: "kanban" })).toBe("/workspaces/shared/01HWS/kanban");
    expect(hrefOf({ ...ws, tab: "table" })).toBe("/workspaces/shared/01HWS/table");
  });

  it("ビュー・クエリ・マイタスクのパス。名前と q は符号化し、q が空なら ?q= を付けない", () => {
    expect(hrefOf({ name: "view", store: "org-b/ops", view: "今週 の/作業" })).toBe(
      "/views/org-b%2Fops/%E4%BB%8A%E9%80%B1%20%E3%81%AE%2F%E4%BD%9C%E6%A5%AD",
    );
    expect(hrefOf({ name: "query", layout: "table", q: "" })).toBe("/query/table");
    expect(hrefOf({ name: "query", layout: "kanban", q: "assignee:@me -status:done" })).toBe(
      "/query/kanban?q=assignee%3A%40me+-status%3Adone",
    );
    expect(hrefOf({ name: "my" })).toBe("/my");
  });

  it("ビュー・クエリ・マイタスクは parseRoute と往復できる", () => {
    for (const r of [
      { name: "view", store: "org-b/ops", view: "今週 の/作業" },
      { name: "query", layout: "table", q: "" },
      { name: "query", layout: "tree", q: 'title:"a b" +x -y&z=1' },
      { name: "my" },
      { name: "project", ulid: "01HPR", tab: "kanban" },
      { name: "workspace", store: "shared", ulid: "01HWS", tab: "table" },
    ] as const) {
      const href = hrefOf(r);
      const url = new URL(href, "http://x");
      expect(parseRoute(url.pathname, url.search)).toEqual(r);
    }
  });

  it("設定のパスは Store 名を符号化する。同期と保守は /settings/general", () => {
    expect(hrefOf({ name: "settings", page: "workspaces", store: "org-b/ops" })).toBe(
      "/settings/workspaces/org-b%2Fops",
    );
    expect(hrefOf({ name: "settings", page: "general" })).toBe("/settings/general");
    expect(hrefOf({ name: "settings", page: "accounts" })).toBe("/settings/accounts");
    expect(hrefOf({ name: "settings", page: "stores" })).toBe("/settings/stores");
    expect(hrefOf({ name: "settings", page: "schema", store: "org-b/ops", tab: "statuses" })).toBe(
      "/settings/schema/org-b%2Fops",
    );
  });
});

describe("router", () => {
  it("go は履歴に積んで route を変える", () => {
    router.go("/projects/01HPR");
    expect(location.pathname).toBe("/projects/01HPR");
    expect(router.route).toEqual({ name: "project", ulid: "01HPR", tab: "tree" });
  });

  it("別名のパスは正準の形に直して履歴に積む", () => {
    router.go("/projects/01HPR/tree");
    expect(location.pathname).toBe("/projects/01HPR");
    router.go("/workspaces/shared/01HWS/tree");
    expect(router.route).toEqual({
      name: "workspace",
      store: "shared",
      ulid: "01HWS",
      tab: "tree",
    });
  });

  it("知らないパスは / に直す。replace なら履歴を積まない", () => {
    const before = history.length;
    router.go("/unknown", { replace: true });
    expect(location.pathname).toBe("/");
    expect(history.length).toBe(before);
  });

  it("クエリとハッシュは捨てる。ただし /query/<layout> の ?q= は残す", () => {
    router.go("/projects/01HPR?x=1#y");
    expect(location.pathname).toBe("/projects/01HPR");
    expect(location.search).toBe("");
    router.go("/query/table?q=status%3Atodo&x=1#y");
    expect(location.pathname).toBe("/query/table");
    expect(location.search).toBe("?q=status%3Atodo");
    expect(location.hash).toBe("");
    expect(router.route).toEqual({ name: "query", layout: "table", q: "status:todo" });
    router.go("/my?q=status%3Atodo");
    expect(location.pathname).toBe("/my");
    expect(location.search).toBe("");
  });

  it("/activity への go は ?workspace= と ?sync= を残し、ほかの query は捨てる", () => {
    router.go("/activity?workspace=A,B&sync=only&x=1#y");
    expect(location.pathname + location.search).toBe("/activity?workspace=A,B&sync=only");
    expect(router.route).toEqual({ name: "activity", workspaces: ["A", "B"], sync: "only" });
    router.go("/activity?sync=exclude");
    expect(router.route).toEqual({ name: "activity", workspaces: [], sync: "exclude" });
  });

  it("q だけ替えた /query への go は route を替える", () => {
    router.go("/query/tree?q=a");
    expect(router.route).toEqual({ name: "query", layout: "tree", q: "a" });
    router.go("/query/tree?q=b");
    expect(router.route).toEqual({ name: "query", layout: "tree", q: "b" });
  });

  it("' を含む q でも、いま表示中のルートへの go は履歴を積まない", () => {
    router.go("/query/table?q=title%3A%27a%20b%27");
    const before = history.length;
    router.go(hrefOf(router.route));
    expect(history.length).toBe(before);
  });

  it("q の + は %2B で書き、parseRoute が + のまま読み戻す", () => {
    const r = { name: "query", layout: "table", q: "a+b" } as const;
    expect(hrefOf(r)).toBe("/query/table?q=a%2Bb");
    const url = new URL(hrefOf(r), "http://x");
    expect(parseRoute(url.pathname, url.search)).toEqual(r);
  });

  it("replace は履歴を積まずに URL を替える", () => {
    router.go("/query/kanban?q=a");
    const before = history.length;
    router.replace("/query/kanban?q=a%20b");
    expect(history.length).toBe(before);
    expect(location.search).toBe("?q=a+b");
    expect(router.route).toEqual({ name: "query", layout: "kanban", q: "a b" });
    router.replace("/unknown");
    expect(location.pathname).toBe("/");
    expect(history.length).toBe(before);
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

describe("the tab of a screen is in its address", () => {
  it("the inbox has ?tab= for every tab but the first; an unknown tab is the first", () => {
    expect(parseRoute("/inbox", "?tab=overwritten")).toEqual({ name: "inbox", tab: "overwritten" });
    expect(parseRoute("/inbox", "?tab=mention")).toEqual({ name: "inbox", tab: "mention" });
    expect(parseRoute("/inbox", "?tab=repair")).toEqual({ name: "inbox", tab: "repair" });
    expect(parseRoute("/inbox", "?tab=nope")).toEqual({ name: "inbox", tab: "all" });
    expect(hrefOf({ name: "inbox", tab: "mention" })).toBe("/inbox?tab=mention");
  });

  it("the schema has ?tab= for every tab but the first; an unknown tab is the first", () => {
    for (const tab of ["kinds", "props", "workflow"] as const) {
      expect(parseRoute("/settings/schema/shared", `?tab=${tab}`)).toEqual({
        name: "settings",
        page: "schema",
        store: "shared",
        tab,
      });
    }
    expect(parseRoute("/settings/schema/shared", "?tab=nope")).toEqual({
      name: "settings",
      page: "schema",
      store: "shared",
      tab: "statuses",
    });
    expect(hrefOf({ name: "settings", page: "schema", store: "org-b/ops", tab: "props" })).toBe(
      "/settings/schema/org-b%2Fops?tab=props",
    );
  });

  it("router.go keeps the tab, and a tab of a path that is not the screen's is dropped", () => {
    router.go("/inbox?tab=mention");
    expect(location.pathname + location.search).toBe("/inbox?tab=mention");
    router.go("/settings/schema/shared?tab=kinds");
    expect(location.pathname + location.search).toBe("/settings/schema/shared?tab=kinds");
    router.go("/my?tab=kinds");
    expect(location.pathname + location.search).toBe("/my");
    router.go("/settings/workspaces/shared?tab=kinds");
    expect(location.pathname + location.search).toBe("/settings/workspaces/shared");
  });
});
