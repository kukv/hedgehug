import { cleanup, fireEvent, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../core/api/client";
import { closeCreate, createState } from "../create-state.svelte";
import { MY_QUERY } from "../my/groups";
import { closePalette, palette } from "../palette-state.svelte";
import { renderWithCore } from "../test-utils";
import { openWorkspaceDelete } from "../workspace/dialogs-state.svelte";
import Sidebar from "./Sidebar.svelte";

vi.mock("../workspace/dialogs-state.svelte", () => ({
  openWorkspaceDelete: vi.fn(),
}));

const workspaces = [
  {
    store: "shared",
    tone: "purple",
    ulid: "01WSREL",
    name: "release",
    prefix: "REL",
    projects: [
      { ulid: "01PRJ1", title: "リリース準備 v2.4", progress: { done: 3, total: 14 } },
      { ulid: "01PRJ2", title: "リリース準備 v2.5", progress: { done: 0, total: 3 } },
    ],
  },
  {
    store: "org-b/ops",
    tone: "readonly",
    ulid: "01WSOPS",
    name: "ops",
    prefix: "OPS",
    projects: [],
  },
];

function mount(over: Record<string, unknown> = {}) {
  return renderWithCore(
    Sidebar,
    {},
    {
      stores: {
        workspaces: () => workspaces,
        visibleWorkspaces: () => workspaces,
        hidden: () => false,
        readOnly: () => null,
        subscribe: () => () => {},
      },
      ...over,
    },
  );
}

// A cache whose query for the tasks of the person finds these tasks, or does not run.
function myCache(found: {
  ulids?: string[];
  loading?: boolean;
  error?: unknown;
  stores?: Record<string, string>;
}) {
  const watchList = vi.fn((_q: { q: string; tree: boolean }) => ({
    ulids: () => found.ulids ?? [],
    loading: () => found.loading ?? false,
    error: () => found.error ?? null,
    dispose: vi.fn(),
  }));
  const task = (u: string) => (found.stores?.[u] ? { store: found.stores[u] } : undefined);
  return { watchList, cache: { subscribe: () => () => {}, watchList, task } };
}

const savedViews = (...entries: [store: string, tone: string, name: string][]) => ({
  list: () => entries.map(([store, tone, name]) => ({ store, tone, view: { name } })),
  get: () => undefined,
  loaded: () => true,
  subscribe: () => () => {},
});

beforeEach(() => {
  vi.clearAllMocks();
  history.replaceState(null, "", "/");
  closePalette();
  closeCreate();
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("Sidebar", () => {
  it("ロゴと host を出す", () => {
    const { getByText } = mount();
    expect(getByText("hh")).toBeTruthy();
    expect(getByText(location.host)).toBeTruthy();
  });

  it("通知とアクティビティの行が出る", () => {
    const { getByRole } = mount();
    expect(getByRole("link", { name: /通知/ })).toBeTruthy();
    expect(getByRole("link", { name: /アクティビティ/ })).toBeTruthy();
  });

  it("アクティビティは /activity を開く行。通知の下に並び、今の画面なら aria-current", async () => {
    const { getByRole, getAllByRole } = mount();
    const link = getByRole("link", { name: /アクティビティ/ });
    expect(link.getAttribute("href")).toBe("/activity");
    expect(link.getAttribute("aria-current")).toBeNull();
    const names = getAllByRole("link").map((a) => a.textContent?.trim());
    expect(names.indexOf("アクティビティ")).toBe(names.indexOf("通知") + 1);
    await fireEvent.click(link);
    expect(location.pathname).toBe("/activity");
    cleanup();
    const again = mount();
    expect(again.getByRole("link", { name: /アクティビティ/ }).getAttribute("aria-current")).toBe(
      "page",
    );
  });

  it("通知は /inbox を開く行。マイタスクの下に並び、今の画面なら aria-current", async () => {
    const { getByRole, getAllByRole } = mount();
    const link = getByRole("link", { name: /通知/ });
    expect(link.getAttribute("href")).toBe("/inbox");
    expect(link.getAttribute("aria-current")).toBeNull();
    const names = getAllByRole("link").map((a) => a.textContent?.trim());
    expect(names.indexOf("通知")).toBe(names.findIndex((t) => t?.startsWith("マイタスク")) + 1);
    await fireEvent.click(link);
    expect(location.pathname).toBe("/inbox");
    cleanup();
    const again = mount();
    expect(again.getByRole("link", { name: /通知/ }).getAttribute("aria-current")).toBe("page");
  });

  it("通知の行は未読の数を出す。0 なら出さない", () => {
    const none = mount();
    expect(none.getByRole("link", { name: /通知/ }).textContent?.trim()).toBe("通知");
    cleanup();
    const some = mount({
      conn: {
        state: "open",
        sync: new Map(),
        unread: 3,
        subscribe: () => () => {},
        on: () => () => {},
      },
    });
    expect(some.getByRole("link", { name: /通知/ }).textContent).toMatch(/3/);
  });

  it("マイタスクは /my を開く行。今の画面なら aria-current", async () => {
    const { getByRole } = mount();
    const link = getByRole("link", { name: /マイタスク/ });
    expect(link.getAttribute("href")).toBe("/my");
    expect(link.getAttribute("aria-current")).toBeNull();
    await fireEvent.click(link);
    expect(location.pathname).toBe("/my");
    cleanup();
    const again = mount();
    expect(again.getByRole("link", { name: /マイタスク/ }).getAttribute("aria-current")).toBe(
      "page",
    );
  });

  it("マイタスクの件数は、マイタスクの画面の既定のクエリを全 Store に問い合わせた数", () => {
    const { watchList, cache } = myCache({ ulids: ["A", "B", "C"] });
    const { getByRole } = mount({ cache });
    expect(watchList).toHaveBeenCalledWith({ q: MY_QUERY, tree: false });
    expect(getByRole("link", { name: /マイタスク/ })).toHaveTextContent("マイタスク 3");
  });

  it("読み込み中と、クエリが走らないとき（アカウントがない）は件数を出さない", () => {
    const loading = mount({ cache: myCache({ loading: true, ulids: ["A"] }).cache });
    expect(loading.getByRole("link", { name: /マイタスク/ })).toHaveTextContent(/^マイタスク$/);
    cleanup();
    const failed = mount({
      cache: myCache({ error: new ApiError({ status: 400, type: "account.none" }) }).cache,
    });
    expect(failed.getByRole("link", { name: /マイタスク/ })).toHaveTextContent(/^マイタスク$/);
  });

  it("保存済みビューがなければ、その見出しも出さない", () => {
    const { queryByRole, queryByText } = mount();
    expect(queryByRole("region", { name: "保存済みビュー" })).toBeNull();
    expect(queryByText("保存済みビュー")).toBeNull();
  });

  it("保存済みビューを並べ、/views/<Store>/<名前> を開く。ツールチップは保存先の Store", async () => {
    const views = savedViews(
      ["shared", "purple", "今週のリリース作業"],
      ["backend", "orange", "ブロック中"],
      ["org-b/ops", "teal", "a/b を含む名前"],
    );
    const { getByRole } = mount({ views });
    const region = getByRole("region", { name: "保存済みビュー" });
    const links = within(region).getAllByRole("link");
    expect(links.map((a) => a.textContent?.trim())).toEqual([
      "今週のリリース作業",
      "ブロック中",
      "a/b を含む名前",
    ]);
    expect(links.map((a) => a.getAttribute("href"))).toEqual([
      "/views/shared/%E4%BB%8A%E9%80%B1%E3%81%AE%E3%83%AA%E3%83%AA%E3%83%BC%E3%82%B9%E4%BD%9C%E6%A5%AD",
      "/views/backend/%E3%83%96%E3%83%AD%E3%83%83%E3%82%AF%E4%B8%AD",
      "/views/org-b%2Fops/a%2Fb%20%E3%82%92%E5%90%AB%E3%82%80%E5%90%8D%E5%89%8D",
    ]);
    expect(links.map((a) => a.getAttribute("title"))).toEqual([
      "shared の views/ に保存",
      "backend の views/ に保存",
      "org-b/ops の views/ に保存",
    ]);
    // the colour of the store is the dot (the store's token), not a hex
    expect(links[0]?.querySelector(".dot")?.getAttribute("style")).toBe(
      "background: var(--store-purple-dot);",
    );
    await fireEvent.click(links[1] as HTMLElement);
    expect(location.pathname).toBe("/views/backend/%E3%83%96%E3%83%AD%E3%83%83%E3%82%AF%E4%B8%AD");
  });

  it("同じ名前のビューは、Store の色の点とツールチップで見分ける", () => {
    const views = savedViews(
      ["shared", "purple", "ブロック中"],
      ["backend", "orange", "ブロック中"],
    );
    const { getByRole } = mount({ views });
    const links = within(getByRole("region", { name: "保存済みビュー" })).getAllByRole("link");
    expect(links.map((a) => a.getAttribute("title"))).toEqual([
      "shared の views/ に保存",
      "backend の views/ に保存",
    ]);
    expect(links.map((a) => a.querySelector(".dot")?.getAttribute("style"))).toEqual([
      "background: var(--store-purple-dot);",
      "background: var(--store-orange-dot);",
    ]);
  });

  it("開いているビューだけ aria-current にする", () => {
    history.replaceState(null, "", "/views/backend/%E3%83%96%E3%83%AD%E3%83%83%E3%82%AF%E4%B8%AD");
    const views = savedViews(
      ["shared", "purple", "ブロック中"],
      ["backend", "orange", "ブロック中"],
    );
    const { getByRole } = mount({ views });
    const links = within(getByRole("region", { name: "保存済みビュー" })).getAllByRole("link");
    expect(links.map((a) => a.getAttribute("aria-current"))).toEqual([null, "page"]);
  });

  it("右上の歯車は設定の同期と保守（/settings/general）を開く", async () => {
    const { getByRole } = mount();
    const gear = getByRole("link", { name: "設定" });
    expect(gear.getAttribute("href")).toBe("/settings/general");
    expect(gear.getAttribute("aria-current")).toBeNull();
    await fireEvent.click(gear);
    expect(location.pathname).toBe("/settings/general");
  });

  it("設定を開いている間は歯車を aria-current にする", () => {
    history.replaceState(null, "", "/settings/workspaces/shared");
    const { getByRole } = mount();
    expect(getByRole("link", { name: "設定" }).getAttribute("aria-current")).toBe("page");
  });

  describe("非表示の Store", () => {
    const hiddenOps = {
      workspaces: () => workspaces,
      visibleWorkspaces: () => workspaces.filter((w) => w.store !== "org-b/ops"),
      hidden: (s: string) => s === "org-b/ops",
      readOnly: () => null,
      subscribe: () => () => {},
    };
    const hiddenFirst = {
      ...hiddenOps,
      visibleWorkspaces: () => workspaces.filter((w) => w.store !== "shared"),
      hidden: (s: string) => s === "shared",
    };

    it("ワークスペースを外す", () => {
      const { queryByRole, getByRole } = mount({ stores: hiddenOps });
      expect(getByRole("link", { name: /release/ })).toBeTruthy();
      expect(queryByRole("link", { name: /ops/ })).toBeNull();
    });

    it("ホームで強調するのは、表示中の最初のワークスペース", () => {
      const { getByRole } = mount({ stores: hiddenFirst });
      expect(getByRole("link", { name: /ops/ }).getAttribute("aria-current")).toBe("page");
    });

    it("保存済みビューを外す。表示中のビューがなければ見出しも出さない", () => {
      const views = savedViews(["shared", "purple", "見える"], ["org-b/ops", "teal", "隠れる"]);
      const shown = mount({ stores: hiddenOps, views });
      const region = shown.getByRole("region", { name: "保存済みビュー" });
      expect(
        within(region)
          .getAllByRole("link")
          .map((a) => a.textContent?.trim()),
      ).toEqual(["見える"]);
      cleanup();
      const none = mount({ stores: hiddenOps, views: savedViews(["org-b/ops", "teal", "隠れる"]) });
      expect(none.queryByRole("region", { name: "保存済みビュー" })).toBeNull();
    });

    it("マイタスクの件数は、非表示の Store のタスクを数えない", () => {
      const { cache } = myCache({
        ulids: ["A", "B", "C"],
        stores: { A: "shared", B: "org-b/ops", C: "shared" },
      });
      const { getByRole } = mount({ stores: hiddenOps, cache });
      expect(getByRole("link", { name: /マイタスク/ })).toHaveTextContent("マイタスク 2");
    });
  });

  it("ワークスペースはどのタブにいても今いる行にする", () => {
    history.replaceState(null, "", "/workspaces/shared/01WSREL/tree");
    const { getByRole } = mount();
    expect(getByRole("link", { name: /release/ }).getAttribute("aria-current")).toBe("page");
  });

  it("ワークスペース › プロジェクトの入れ子と進捗を出す", () => {
    const { getByRole, getByText } = mount();
    const ws = getByRole("link", { name: /release/ });
    expect(ws.getAttribute("href")).toBe("/workspaces/shared/01WSREL");
    expect(getByText("REL")).toBeTruthy();
    const p = getByRole("link", { name: /リリース準備 v2.4/ });
    expect(p.getAttribute("href")).toBe("/projects/01PRJ1");
    expect(p.textContent).toContain("3/14");
    expect(getByText("プロジェクトなし")).toBeTruthy();
  });

  it("Store 名に / があっても href を符号化する", () => {
    const { getByRole } = mount();
    expect(getByRole("link", { name: /ops/ }).getAttribute("href")).toBe(
      "/workspaces/org-b%2Fops/01WSOPS",
    );
  });

  it("今いるプロジェクトを aria-current で示す", () => {
    history.replaceState(null, "", "/projects/01PRJ2");
    const { getByRole } = mount();
    expect(getByRole("link", { name: /リリース準備 v2.5/ }).getAttribute("aria-current")).toBe(
      "page",
    );
    expect(
      getByRole("link", { name: /リリース準備 v2.4/ }).getAttribute("aria-current"),
    ).toBeNull();
  });

  it("今いるワークスペースを示す。/ は最初のワークスペース", () => {
    history.replaceState(null, "", "/");
    const { getByRole } = mount();
    expect(getByRole("link", { name: /release/ }).getAttribute("aria-current")).toBe("page");
  });

  it("リンクを押すと router で移動する", async () => {
    const { getByRole } = mount();
    await fireEvent.click(getByRole("link", { name: /リリース準備 v2.4/ }));
    expect(location.pathname).toBe("/projects/01PRJ1");
  });

  it("ほかのダイアログが開いている間は ⌘K を無視する", async () => {
    vi.spyOn(navigator, "platform", "get").mockReturnValue("Linux x86_64");
    mount();
    const dlg = document.createElement("div");
    dlg.setAttribute("aria-modal", "true");
    document.body.append(dlg);
    await fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    expect(palette.open).toBe(false);
    dlg.remove();
    await fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    expect(palette.open).toBe(true);
  });

  it("検索欄と ⌘K でパレットを開く", async () => {
    vi.spyOn(navigator, "platform", "get").mockReturnValue("Linux x86_64");
    const { getByRole } = mount();
    await fireEvent.click(getByRole("button", { name: /ID・検索・コマンド/ }));
    expect(palette.open).toBe(true);
    closePalette();
    await fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    expect(palette.open).toBe(true);
  });

  it("プロジェクトを追加で作成ダイアログの要求を出す", async () => {
    const { getByRole } = mount();
    await fireEvent.click(getByRole("button", { name: "プロジェクトを追加" }));
    expect(createState.request).toEqual({ kind: "project" });
  });

  it("ワークスペースの「…」は設定を開く・削除…の 2 つを出す", async () => {
    const { getByRole, getAllByRole } = mount();
    await fireEvent.click(getByRole("button", { name: "release の操作（設定を開く・削除）" }));
    expect(getByRole("menu", { name: "ワークスペースの操作" })).toBeTruthy();
    expect(getAllByRole("menuitem").map((i) => i.textContent?.trim())).toEqual([
      "設定を開く",
      "削除…",
    ]);
  });

  it("閲覧のみの Store のワークスペースは、設定は開けるが削除…は理由つきで無効にする", async () => {
    const { getByRole } = mount({
      stores: {
        workspaces: () => workspaces,
        visibleWorkspaces: () => workspaces,
        hidden: () => false,
        readOnly: (s: string) => (s === "org-b/ops" ? "the remote cannot be written" : null),
        subscribe: () => () => {},
      },
    });
    await fireEvent.click(getByRole("button", { name: "ops の操作（設定を開く・削除）" }));
    expect(getByRole("menuitem", { name: "設定を開く" })).toBeEnabled();
    const del = getByRole("menuitem", { name: /削除…/ });
    expect(del).toBeDisabled();
    expect(del).toHaveTextContent("この Store は閲覧のみです（remote に書き込めません）");
    await fireEvent.click(del);
    expect(openWorkspaceDelete).not.toHaveBeenCalled();
  });

  it("「設定を開く」はそのワークスペースの設定のタブへ移る", async () => {
    const { getByRole, queryByRole } = mount();
    await fireEvent.click(getByRole("button", { name: "release の操作（設定を開く・削除）" }));
    await fireEvent.click(getByRole("menuitem", { name: "設定を開く" }));
    expect(location.pathname).toBe("/workspaces/shared/01WSREL/settings");
    expect(queryByRole("menu")).toBeNull();
  });

  it("「削除…」は削除の確認を開く（ワークスペースを渡す）", async () => {
    const { getByRole, queryByRole } = mount();
    await fireEvent.click(getByRole("button", { name: "ops の操作（設定を開く・削除）" }));
    await fireEvent.click(getByRole("menuitem", { name: "削除…" }));
    expect(openWorkspaceDelete).toHaveBeenCalledWith(workspaces[1]);
    expect(location.pathname).toBe("/");
    expect(queryByRole("menu")).toBeNull();
  });
});
