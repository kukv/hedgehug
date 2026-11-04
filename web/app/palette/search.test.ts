import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import type { WorkspaceEntry } from "../../core/stores";
import { task } from "../tree/fixtures";
import {
  buildCommands,
  type CommandEnv,
  detectMode,
  isStoreQualifiedId,
  queryCommands,
  type SearchDeps,
  searchTasks,
  taskCommands,
} from "./search";

describe("detectMode", () => {
  it.each([
    [">同期", "command"],
    ["  > sync", "command"],
    [">", "command"],
    ["status:doing assignee:@me", "query"],
    ["is:blocked", "query"],
    ["-status:canceled", "query"],
    ["due<2026-11-10", "query"],
    ["points>=3", "query"],
    ["sh-17", "id"],
    ["REL-6", "id"],
    ["01JC8ZK3Q4M5N6P7R8S9T0V1W2", "id"],
    ["API の破壊的変更", "id"],
    ["", "id"],
    ["   ", "id"],
  ])("%j is %s", (input, mode) => {
    expect(detectMode(input)).toBe(mode);
  });
});

describe("detectMode with store names", () => {
  const names = ["org-b", "personal"];
  it.each([
    ["org-b:API-12", "id"],
    ["  personal:web-3 ", "id"],
    ["status:todo", "query"],
    ["x:API-1", "query"],
    ["org-b:status", "query"],
    ["org-b:API-12 more", "query"],
  ])("%j is %s", (input, mode) => {
    expect(detectMode(input, names)).toBe(mode);
  });

  it("keeps a colon word as a query without the names", () => {
    expect(detectMode("org-b:API-12")).toBe("query");
  });

  it("tells a store-qualified id", () => {
    expect(isStoreQualifiedId("org-b:API-12", names)).toBe(true);
    expect(isStoreQualifiedId("org-b:API", names)).toBe(false);
    expect(isStoreQualifiedId("API-12", names)).toBe(false);
    expect(isStoreQualifiedId("zzz:API-12", names)).toBe(false);
  });
});

const ok = (data: unknown) => ({ data, response: new Response(null, { status: 200 }) });
const notFound = {
  error: { type: "task.not_found", title: "Not Found", status: 404, detail: "not found" },
  response: new Response(null, { status: 404 }),
};

const rel6 = {
  ulid: "U6",
  id: "REL-6",
  title: "Web の API クライアントを v2 に更新",
  kind: "task",
  store: "shared",
  former_ids: ["SH-17"],
};

function deps(over: { tasks?: TaskView[]; listError?: unknown } = {}) {
  const GET = vi.fn(
    async (
      path: string,
      opts: { params: { path?: { id: string }; query?: { store?: string; q?: string } } },
    ) => {
      if (path === "/api/tasks/{id}") {
        const id = opts.params.path?.id.toLowerCase();
        const hit = id === "sh-17" || id === "rel-6" || id === "u6";
        return hit ? ok(rel6) : notFound;
      }
      if (over.listError) return over.listError;
      return ok({ tasks: over.tasks ?? [] });
    },
  );
  const prime = vi.fn();
  const d: SearchDeps = {
    api: { GET } as unknown as SearchDeps["api"],
    hrefOf: (u) => `/tasks/${u}`,
    epoch: () => 42,
    prime,
  };
  return { d, GET, prime };
}

describe("searchTasks: ID and text", () => {
  it("finds a task by a former ID and says so", async () => {
    const { d } = deps();
    const r = await searchTasks(d, "sh-17", "id");
    expect(r.error).toBeNull();
    expect(r.groups).toHaveLength(1);
    expect(r.groups[0]?.name).toBe("タスク");
    expect(r.groups[0]?.items[0]).toMatchObject({
      type: "task",
      ulid: "U6",
      id: "REL-6",
      store: "shared",
      note: "旧 ID SH-17 で一致",
      href: "/tasks/U6",
    });
  });

  it("finds a task by its ID without a note, in any case", async () => {
    const { d } = deps();
    const r = await searchTasks(d, "rel-6", "id");
    expect(r.groups[0]?.items[0]).toMatchObject({ id: "REL-6" });
    expect(r.groups[0]?.items[0]).not.toHaveProperty("note");
  });

  it("looks the ID up once, in every store, and ignores an ID nobody has", async () => {
    const { d, GET } = deps();
    const r = await searchTasks(d, "REL-99", "id");
    expect(r.error).toBeNull();
    expect(r.groups).toEqual([]);
    expect(GET).toHaveBeenCalledWith("/api/tasks/{id}", { params: { path: { id: "REL-99" } } });
  });

  it("searches the words of the ID and title and adds what it finds to the cache", async () => {
    const found = [task("U9", "task", null, { id: "REL-9", title: "API v2 の移行ガイド" })];
    const { d, GET, prime } = deps({ tasks: found });
    const r = await searchTasks(d, "api v2", "id");
    expect(GET).toHaveBeenCalledWith("/api/tasks", {
      params: { query: { q: '"api" "v2"', store: undefined } },
    });
    expect(prime).toHaveBeenCalledWith(found, 42);
    expect(r.groups[0]?.items.map((i) => (i.type === "task" ? i.id : ""))).toEqual(["REL-9"]);
  });

  it("does not list a task twice and keeps the match by ID first", async () => {
    const found = [
      task("U6", "task", null, { id: "REL-6", title: "Web の API クライアントを v2 に更新" }),
      task("U9", "task", null, { id: "REL-9", title: "x" }),
    ];
    const { d } = deps({ tasks: found });
    const r = await searchTasks(d, "REL-6", "id");
    expect(r.groups[0]?.items.map((i) => (i.type === "task" ? i.ulid : ""))).toEqual(["U6", "U9"]);
  });

  it("returns nothing for an empty input and no group when nothing matches", async () => {
    const { d, GET } = deps();
    expect(await searchTasks(d, "  ", "id")).toEqual({ groups: [], error: null });
    expect(GET).not.toHaveBeenCalled();
    expect((await searchTasks(d, "zzz", "id")).groups).toEqual([]);
  });

  it("stops at 20 results", async () => {
    const many = Array.from({ length: 30 }, (_, i) =>
      task(`M${i}`, "task", null, { id: `REL-${i}` }),
    );
    const { d } = deps({ tasks: many });
    const r = await searchTasks(d, "x", "id");
    expect(r.groups[0]?.items).toHaveLength(20);
  });
});

describe("searchTasks: query", () => {
  it("passes the query as it is and names the group with the count", async () => {
    const found = [task("U1", "task", null), task("U2", "task", null)];
    const { d, GET } = deps({ tasks: found });
    const r = await searchTasks(d, "status:doing assignee:@me", "query");
    expect(GET).toHaveBeenCalledWith("/api/tasks", {
      params: { query: { q: "status:doing assignee:@me", tree: false } },
    });
    expect(r.groups).toHaveLength(1);
    expect(r.groups[0]?.name).toBe("クエリに一致 · 2 件");
  });

  it("returns the error of a query that does not run", async () => {
    const { d } = deps({
      listError: {
        error: {
          type: "query.invalid",
          title: "Bad",
          status: 400,
          detail: "term 1 (foo:bar): unknown key",
        },
        response: new Response(null, { status: 400 }),
      },
    });
    const r = await searchTasks(d, "foo:bar", "query");
    expect(r.groups).toEqual([]);
    expect(r.error).toMatchObject({ type: "query.invalid" });
  });
});

const ws: WorkspaceEntry = {
  store: "shared",
  tone: "purple",
  ulid: "W1",
  name: "release",
  prefix: "REL",
  tasks: 24,
  deleted: 1,
  prefixSet: "2026-10-29T12:00:00Z",
  projects: [task("P1", "project", null, { id: "REL-15", title: "リリース準備 v2.4" })],
};

function env(over: Partial<CommandEnv> = {}): CommandEnv {
  return {
    syncNow: vi.fn(),
    createTask: vi.fn(),
    createProject: vi.fn(),
    addWorkspace: vi.fn(),
    undo: vi.fn(),
    saveQuery: vi.fn(),
    workspaces: [ws],
    views: [],
    storeNames: ["shared"],
    go: vi.fn(),
    hrefs: {
      workspace: (s, u) => `/workspaces/${s}/${u}`,
      workspaceSettings: (s, u) => `/workspaces/${s}/${u}/settings`,
      settingsWorkspaces: (s) => `/settings/workspaces/${s}`,
      settingsSchema: (s) => `/settings/schema/${s}`,
      settingsGeneral: () => "/settings/general",
      settingsAccounts: () => "/settings/accounts",
      settingsStores: () => "/settings/stores",
      project: (u) => `/projects/${u}`,
      projectSettings: (u) => `/projects/${u}/settings`,
      task: (u) => `/tasks/${u}`,
      view: (s, n) => `/views/${s}/${encodeURIComponent(n)}`,
      query: (l, q) => `/query/${l}?${new URLSearchParams({ q })}`,
      my: () => "/my",
      inbox: () => "/inbox",
      activity: () => "/activity",
    },
    selected: null,
    project: null,
    recent: [],
    pickStatus: vi.fn(),
    moveTask: vi.fn(),
    readOnly: () => null,
    ...over,
  };
}

beforeEach(() => {
  vi.spyOn(navigator, "platform", "get").mockReturnValue("MacIntel");
});
afterEach(() => {
  vi.restoreAllMocks();
});

const labels = (gs: ReturnType<typeof buildCommands>) =>
  gs.flatMap((g) => g.items.map((i) => (i.type === "command" ? i.label : i.title)));

describe("buildCommands", () => {
  it("lists the commands, then where to go", () => {
    const gs = buildCommands(env(), ">");
    expect(gs.map((g) => g.name)).toEqual(["コマンド", "開く"]);
    expect(labels(gs)).toEqual([
      "今すぐ同期する",
      "タスクを作成",
      "プロジェクトを作成",
      "ワークスペースを追加",
      "直前の操作を取り消す",
      "doctor を実行",
      "release を開く",
      "ワークスペースの設定を開く",
      "リリース準備 v2.4 を開く",
      "設定のワークスペース一覧を開く",
      "設定のスキーマを開く",
      "設定のアカウントと認証を開く",
      "設定のマウント中の Store を開く",
      "設定の同期と保守を開く",
      "マイタスクを開く",
      "通知を開く",
      "アクティビティを開く",
    ]);
  });

  it("lists the tasks opened last, filtered by the words", () => {
    const e = env({
      recent: [task("R1", "task", null, { id: "REL-3", title: "移行ガイド" })],
    });
    const gs = buildCommands(e, ">");
    expect(gs.map((g) => g.name)).toEqual(["コマンド", "最近開いたもの", "開く"]);
    expect(gs[1]?.items[0]).toMatchObject({ type: "task", id: "REL-3", href: "/tasks/R1" });
    expect(buildCommands(e, ">ガイド").map((g) => g.name)).toEqual(["最近開いたもの"]);
  });

  it("filters by the words after >", () => {
    expect(labels(buildCommands(env(), ">同期"))).toEqual([
      "今すぐ同期する",
      "設定の同期と保守を開く",
    ]);
    expect(labels(buildCommands(env(), "> リリース"))).toEqual(["リリース準備 v2.4 を開く"]);
    expect(buildCommands(env(), ">zzz")).toEqual([]);
  });

  it("writes the shortcut for the platform", () => {
    const mac = buildCommands(env(), ">同期")[0]?.items[0];
    expect(mac).toMatchObject({ kbd: "⇧⌘S" });
    vi.spyOn(navigator, "platform", "get").mockReturnValue("Linux x86_64");
    const other = buildCommands(env(), ">同期")[0]?.items[0];
    expect(other).toMatchObject({ kbd: "Ctrl+Shift+S" });
    const task = taskCommands(env(), { ulid: "T1", id: "REL-6" } as TaskView);
    expect(task.items[2]).toMatchObject({ kbd: "Ctrl+↵" });
  });

  it("runs a command and goes to a workspace or a project with a URL for a new tab", () => {
    const e = env();
    const gs = buildCommands(e, ">");
    const sync = gs[0]?.items[0];
    if (sync?.type === "command") sync.run();
    expect(e.syncNow).toHaveBeenCalled();
    const proj = gs[1]?.items[2];
    expect(proj).toMatchObject({ href: "/projects/P1", store: "shared" });
    if (proj?.type === "command") proj.run();
    expect(e.go).toHaveBeenCalledWith("/projects/P1");
  });

  it("adds a workspace and takes back the last operation", () => {
    const e = env();
    const items = buildCommands(e, ">")[0]?.items ?? [];
    const add = items.find((i) => i.key === "cmd:workspace-add");
    const undo = items.find((i) => i.key === "cmd:undo");
    expect(add).toMatchObject({ label: "ワークスペースを追加", icon: "plus" });
    expect(undo).toMatchObject({ label: "直前の操作を取り消す", icon: "undo" });
    if (add?.type === "command") add.run();
    expect(e.addWorkspace).toHaveBeenCalledTimes(1);
    if (undo?.type === "command") undo.run();
    expect(e.undo).toHaveBeenCalledTimes(1);
    expect(labels(buildCommands(e, ">取り消"))).toEqual(["直前の操作を取り消す"]);
    expect(labels(buildCommands(e, ">追加"))).toEqual(["ワークスペースを追加"]);
  });

  it("opens the settings of a workspace, named by the workspace, and the list of a store", () => {
    const e = env();
    const gs = buildCommands(e, ">設定");
    expect(gs.map((g) => g.name)).toEqual(["開く"]);
    const [settings, list, schema] = gs[0]?.items ?? [];
    expect(settings).toMatchObject({
      label: "ワークスペースの設定を開く",
      note: "release",
      store: "shared",
      href: "/workspaces/shared/W1/settings",
    });
    expect(list).toMatchObject({
      label: "設定のワークスペース一覧を開く",
      store: "shared",
      href: "/settings/workspaces/shared",
    });
    if (settings?.type === "command") settings.run();
    expect(e.go).toHaveBeenCalledWith("/workspaces/shared/W1/settings");
    if (list?.type === "command") list.run();
    expect(e.go).toHaveBeenCalledWith("/settings/workspaces/shared");
    expect(schema).toMatchObject({
      label: "設定のスキーマを開く",
      icon: "settings",
      store: "shared",
      href: "/settings/schema/shared",
    });
    if (schema?.type === "command") schema.run();
    expect(e.go).toHaveBeenCalledWith("/settings/schema/shared");
  });

  it("opens the sync and maintenance settings, once whatever the number of stores", () => {
    const e = env({ storeNames: ["shared", "ops"] });
    const gs = buildCommands(e, ">同期と保守");
    const items = gs.flatMap((g) => g.items);
    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({
      label: "設定の同期と保守を開く",
      icon: "settings",
      href: "/settings/general",
    });
    if (items[0]?.type === "command") items[0].run();
    expect(e.go).toHaveBeenCalledWith("/settings/general");
  });

  it("opens the accounts and the mounted Stores settings, once each whatever the number of stores", () => {
    const e = env({ storeNames: ["shared", "ops"] });
    for (const [word, label, href] of [
      ["アカウント", "設定のアカウントと認証を開く", "/settings/accounts"],
      ["マウント中", "設定のマウント中の Store を開く", "/settings/stores"],
    ] as const) {
      const items = buildCommands(e, `>${word}`).flatMap((g) => g.items);
      expect(items).toHaveLength(1);
      expect(items[0]).toMatchObject({ label, icon: "settings", href });
      if (items[0]?.type === "command") items[0].run();
      expect(e.go).toHaveBeenLastCalledWith(href);
    }
  });

  it("runs the diagnosis by opening the sync and maintenance settings, which asks for it", () => {
    const e = env({ storeNames: ["shared", "ops"] });
    const items = buildCommands(e, ">doctor").flatMap((g) => g.items);
    expect(items).toHaveLength(1);
    expect(items[0]).toMatchObject({
      label: "doctor を実行",
      icon: "checklist",
      href: "/settings/general",
    });
    if (items[0]?.type === "command") items[0].run();
    expect(e.go).toHaveBeenCalledWith("/settings/general");
  });

  it("lists the settings of a workspace for each workspace, the list once for a store", () => {
    const second = { ...ws, ulid: "W2", name: "ops", prefix: "OPS", projects: [] };
    const gs = buildCommands(env({ workspaces: [ws, second] }), ">設定");
    expect(
      gs[0]?.items.map((i) => (i.type === "command" ? `${i.label}|${i.note ?? ""}` : "")),
    ).toEqual([
      "ワークスペースの設定を開く|release",
      "ワークスペースの設定を開く|ops",
      "設定のワークスペース一覧を開く|",
      "設定のスキーマを開く|",
      "設定のアカウントと認証を開く|",
      "設定のマウント中の Store を開く|",
      "設定の同期と保守を開く|",
    ]);
  });

  it("opens the settings of the project that is open, and offers nothing when none is", () => {
    expect(buildCommands(env(), ">プロジェクトの設定")).toEqual([]);
    const e = env({ project: { ulid: "P1", title: "リリース準備 v2.4" } });
    const gs = buildCommands(e, ">プロジェクトの設定");
    expect(gs.map((g) => g.name)).toEqual(["開く"]);
    const [item] = gs[0]?.items ?? [];
    expect(item).toMatchObject({
      label: "プロジェクトの設定を開く",
      note: "リリース準備 v2.4",
      href: "/projects/P1/settings",
    });
    if (item?.type === "command") item.run();
    expect(e.go).toHaveBeenCalledWith("/projects/P1/settings");
  });

  it("lists the workspaces of a store that has none, once per store", () => {
    const gs = buildCommands(env({ workspaces: [], storeNames: ["shared", "empty"] }), ">設定");
    expect(
      gs[0]?.items.map((i) => (i.type === "command" ? `${i.label}|${i.store ?? ""}` : "")),
    ).toEqual([
      "設定のワークスペース一覧を開く|shared",
      "設定のワークスペース一覧を開く|empty",
      "設定のスキーマを開く|shared",
      "設定のスキーマを開く|empty",
      "設定のアカウントと認証を開く|",
      "設定のマウント中の Store を開く|",
      "設定の同期と保守を開く|",
    ]);
  });

  it("opens the tasks of the person, and each saved view with the store it is in", () => {
    const views = [
      { store: "shared", tone: "purple" as const, view: { name: "今週のリリース作業" } },
      { store: "backend", tone: "orange" as const, view: { name: "ブロック中" } },
    ] as CommandEnv["views"];
    const e = env({ views });
    const gs = buildCommands(e, ">");
    const opens = gs.find((g) => g.name === "開く")?.items ?? [];
    expect(
      opens.slice(-5).map((i) => (i.type === "command" ? [i.label, i.store, i.href] : [])),
    ).toEqual([
      ["マイタスクを開く", undefined, "/my"],
      ["通知を開く", undefined, "/inbox"],
      ["アクティビティを開く", undefined, "/activity"],
      [
        "今週のリリース作業 を開く",
        "shared",
        "/views/shared/%E4%BB%8A%E9%80%B1%E3%81%AE%E3%83%AA%E3%83%AA%E3%83%BC%E3%82%B9%E4%BD%9C%E6%A5%AD",
      ],
      [
        "ブロック中 を開く",
        "backend",
        "/views/backend/%E3%83%96%E3%83%AD%E3%83%83%E3%82%AF%E4%B8%AD",
      ],
    ]);
    const view = opens.at(-2);
    if (view?.type === "command") view.run();
    expect(e.go).toHaveBeenCalledWith(
      "/views/shared/%E4%BB%8A%E9%80%B1%E3%81%AE%E3%83%AA%E3%83%AA%E3%83%BC%E3%82%B9%E4%BD%9C%E6%A5%AD",
    );
  });

  it("narrows the places to open by the word, the saved views too", () => {
    const views = [
      { store: "backend", tone: "orange" as const, view: { name: "ブロック中" } },
    ] as CommandEnv["views"];
    const gs = buildCommands(env({ views }), ">ブロック");
    expect(gs.map((g) => g.name)).toEqual(["開く"]);
    expect(gs[0]?.items.map((i) => (i.type === "command" ? i.label : ""))).toEqual([
      "ブロック中 を開く",
    ]);
    expect(
      buildCommands(env(), ">マイ")[0]?.items.map((i) => (i.type === "command" ? i.label : "")),
    ).toEqual(["マイタスクを開く"]);
  });

  it("leaves out the creates and the workspace add when no Store can be written", () => {
    const keys = (e: CommandEnv) =>
      buildCommands(e, ">")[0]?.items.map((i) => (i.type === "command" ? i.key : "")) ?? [];
    const all = keys(env());
    expect(all).toEqual(
      expect.arrayContaining(["cmd:create-task", "cmd:create-project", "cmd:workspace-add"]),
    );
    const ro = keys(env({ readOnly: () => "remote に書き込めません" }));
    for (const k of ["cmd:create-task", "cmd:create-project", "cmd:workspace-add"]) {
      expect(ro).not.toContain(k);
    }
    expect(ro).toContain("cmd:sync");
    // one Store that can be written is enough
    const some = keys(
      env({
        workspaces: [ws, { ...ws, store: "backend", ulid: "W2", name: "ops", prefix: "OPS" }],
        storeNames: ["shared", "backend"],
        readOnly: (s) => (s === "shared" ? "x" : null),
      }),
    );
    expect(some).toEqual(
      expect.arrayContaining(["cmd:create-task", "cmd:create-project", "cmd:workspace-add"]),
    );
  });

  it("adds the commands for the selected task", () => {
    const t = task("T1", "task", null, { id: "REL-6" });
    const gs = buildCommands(env({ selected: t }), ">");
    expect(gs.map((g) => g.name)).toEqual(["コマンド", "開く", "REL-6 に対して"]);
  });
});

describe("taskCommands", () => {
  it("offers a task of a read-only Store only its detail: no status, no child, no move", () => {
    const t = task("T1", "task", null, { id: "REL-6" });
    const e = env({
      workspaces: [ws, { ...ws, ulid: "W2", name: "ops", prefix: "OPS" }],
      readOnly: (s) => (s === "shared" ? "remote に書き込めません" : null),
    });
    expect(taskCommands(e, t).items.map((i) => i.key)).toEqual(["task-open:T1"]);
    // a task of another Store is still offered everything
    const other = taskCommands(e, { ...t, store: "backend" });
    expect(other.items.map((i) => i.key)).toContain("task-status:T1");
  });

  it("offers no move when the store has no other workspace, and none in another store", () => {
    const t = task("T1", "task", null, { id: "REL-6" });
    const none = taskCommands(env(), t);
    expect(none.items).toHaveLength(3);
    const elsewhere = taskCommands(
      env({ workspaces: [ws, { ...ws, store: "backend", ulid: "W2", name: "ops" }] }),
      t,
    );
    expect(elsewhere.items.map((i) => i.key)).not.toContain("task-move:T1");
  });

  it("changes the status in the palette, creates a child, and opens the detail", () => {
    const e = env({ workspaces: [ws, { ...ws, ulid: "W2", name: "ops", prefix: "OPS" }] });
    const t = task("T1", "task", null, { id: "REL-6" });
    const g = taskCommands(e, t);
    expect(g.name).toBe("REL-6 に対して");
    const [status, child, detail, move] = g.items;
    expect(g.items).toHaveLength(4);
    expect(move).toMatchObject({
      key: "task-move:T1",
      label: "別のワークスペースへ移動…",
      icon: "workspace",
    });
    if (move?.type === "command") move.run();
    expect(e.moveTask).toHaveBeenCalledWith(t);
    expect(status).toMatchObject({ label: "ステータスを変更…", keepOpen: true });
    expect(child).toMatchObject({ label: "子タスクを作る", kbd: "C" });
    expect(detail).toMatchObject({ label: "詳細を開く", kbd: "⌘↵", href: "/tasks/T1" });
    if (status?.type === "command") status.run();
    expect(e.pickStatus).toHaveBeenCalledWith(t);
    if (child?.type === "command") child.run();
    expect(e.createTask).toHaveBeenCalledWith("T1");
    if (detail?.type === "command") detail.run();
    expect(e.go).toHaveBeenCalledWith("/tasks/T1");
  });
});

describe("queryCommands", () => {
  it("opens the query as a table without saving, and saves it as a view", () => {
    const e = env();
    const g = queryCommands(e, "status:doing assignee:@me");
    expect(g.name).toBe("このクエリで");
    expect(g.items.map((i) => (i.type === "command" ? i.label : ""))).toEqual([
      "テーブルで開く",
      "ビューとして保存…",
    ]);
    const [open, save] = g.items;
    expect(open).toMatchObject({
      href: "/query/table?q=status%3Adoing+assignee%3A%40me",
    });
    if (open?.type === "command") open.run();
    expect(e.go).toHaveBeenCalledWith("/query/table?q=status%3Adoing+assignee%3A%40me");
    if (save?.type === "command") save.run();
    expect(e.saveQuery).toHaveBeenCalledWith("status:doing assignee:@me");
    // a command of the query is not one of the bulk update, which is for phase 6
    expect(g.items.some((i) => i.type === "command" && /まとめて更新/.test(i.label))).toBe(false);
  });
});
