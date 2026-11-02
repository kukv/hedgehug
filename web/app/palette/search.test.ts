import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import type { WorkspaceEntry } from "../../core/stores";
import { task } from "../tree/fixtures";
import {
  buildCommands,
  type CommandEnv,
  detectMode,
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
    expect(prime).toHaveBeenCalledWith(found);
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
  projects: [task("P1", "project", null, { id: "REL-15", title: "リリース準備 v2.4" })],
};

function env(over: Partial<CommandEnv> = {}): CommandEnv {
  return {
    syncNow: vi.fn(),
    createTask: vi.fn(),
    createProject: vi.fn(),
    workspaces: [ws],
    go: vi.fn(),
    hrefs: {
      workspace: (s, u) => `/workspaces/${s}/${u}`,
      project: (u) => `/projects/${u}`,
      task: (u) => `/tasks/${u}`,
    },
    selected: null,
    recent: [],
    pickStatus: vi.fn(),
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
      "release を開く",
      "リリース準備 v2.4 を開く",
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
    expect(labels(buildCommands(env(), ">同期"))).toEqual(["今すぐ同期する"]);
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
    const proj = gs[1]?.items[1];
    expect(proj).toMatchObject({ href: "/projects/P1", store: "shared" });
    if (proj?.type === "command") proj.run();
    expect(e.go).toHaveBeenCalledWith("/projects/P1");
  });

  it("adds the commands for the selected task", () => {
    const t = task("T1", "task", null, { id: "REL-6" });
    const gs = buildCommands(env({ selected: t }), ">");
    expect(gs.map((g) => g.name)).toEqual(["コマンド", "開く", "REL-6 に対して"]);
  });
});

describe("taskCommands", () => {
  it("changes the status in the palette, creates a child, and opens the detail", () => {
    const e = env();
    const t = task("T1", "task", null, { id: "REL-6" });
    const g = taskCommands(e, t);
    expect(g.name).toBe("REL-6 に対して");
    const [status, child, detail] = g.items;
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
