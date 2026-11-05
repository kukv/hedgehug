import { cleanup, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import { TaskCache } from "../../core/cache";
import { formatDate } from "../../core/format";
import type { WorkspaceEntry } from "../../core/stores";
import { changed, FakeConn, fakeApi, problem, tv } from "../../core/testing";
import { closeCreate, createState } from "../create-state.svelte";
import { router } from "../router";
import { emitter, renderWithCore } from "../test-utils";
import { schema } from "../tree/fixtures";
import WorkspaceOverview from "./WorkspaceOverview.svelte";

afterEach(() => {
  cleanup();
  closeCreate();
});

// The entry the directory holds: its count is older than the list, to show that the page counts the list.
const entry: WorkspaceEntry = {
  store: "shared",
  tone: "purple",
  ulid: "W1",
  name: "release",
  prefix: "REL",
  projects: [],
  tasks: 3,
  deleted: 0,
  sprints: 0,
  releases: 0,
  prefixSet: "2026-10-29T12:00:00Z",
};

const year = new Date().getFullYear();
const project = tv("P1", "リリース準備 v2.4", {
  id: "REL-15",
  kind: "project",
  status: "doing",
  rank: "a",
  assignees: ["sato"],
  props: { due: `${year}-11-12` },
  progress: { done: 3, total: 14 },
});
const child = tv("C1", "子のタスク", { id: "REL-16", parent: "P1", status: "doing" });
const rootLate = tv("R2", "手順書を見直す", { id: "REL-18", rank: "c", status: "todo" });
const rootEarly = tv("R1", "CI を整理", {
  id: "REL-22",
  rank: "b",
  status: "doing",
  assignees: ["tanaka"],
  props: { due: `${year}-11-08` },
});
const done = tv("D1", "終わったもの", { id: "REL-3", rank: "d", status: "done" });
const canceled = tv("X1", "やめたもの", { id: "REL-4", rank: "e", status: "canceled" });
const all = [project, child, rootLate, rootEarly, done, canceled];

function setup(
  initial: TaskView[],
  route?: (path: string) => unknown,
  schemaFails = false,
  readOnly: string | null = null,
) {
  const rows = { current: initial };
  const { api, calls } = fakeApi((r) => {
    const own = route?.(r.path);
    if (own !== undefined) return own;
    return { tasks: rows.current };
  });
  const conn = new FakeConn();
  const cache = new TaskCache({ api, conn: conn.asConnection() });
  const sch = emitter();
  let schemaReady = !schemaFails;
  const r = renderWithCore(
    WorkspaceOverview,
    { entry },
    {
      ...(readOnly === null
        ? {}
        : {
            stores: {
              stores: () => [],
              tone: () => "readonly",
              displayId: (_s: string, id: string) => id,
              readOnly: () => readOnly,
              hidden: () => false,
              workspaces: () => [entry],
              workspace: () => entry,
              loaded: () => true,
              subscribe: sch.subscribe,
            },
          }),
      cache,
      schema: {
        get: () => (schemaReady ? schema : undefined),
        load: async () => {
          if (schemaFails) throw new Error("schema down");
          return schema;
        },
        subscribe: sch.subscribe,
      },
    },
  );
  // Puts the schema in the store by another path after the load has failed.
  const schemaArrives = () => {
    schemaReady = true;
    sch.emit();
  };
  return { ...r, rows, conn, cache, calls, schemaArrives };
}

describe("WorkspaceOverview", () => {
  it("asks for the tasks of the workspace by its ULID, in its store", async () => {
    const { calls } = setup(all);
    await screen.findByText("6 件 · プロジェクト 1");
    const q = calls.find((c) => c.path === "/api/tasks");
    expect(q?.query.get("q")).toBe("workspace:W1");
    expect(q?.query.get("store")).toBe("shared");
  });

  it("counts every task and the projects, and the categories", async () => {
    setup(all);
    expect(await screen.findByText("6 件 · プロジェクト 1")).toBeInTheDocument();
    expect(screen.getByText("削除済みは含めません")).toBeInTheDocument();
    expect(screen.getByText("未着手 1")).toBeInTheDocument();
    expect(screen.getByText("進行中 3")).toBeInTheDocument();
    expect(screen.getByText("完了 1")).toBeInTheDocument();
    expect(screen.getByText("中止 1")).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "未着手 1 · 進行中 3 · 完了 1 · 中止 1" }),
    ).toBeInTheDocument();
  });

  it("says why the band is missing when the schema cannot be read, and still counts", async () => {
    setup(all, undefined, true);
    expect(await screen.findByText("6 件 · プロジェクト 1")).toBeInTheDocument();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "スキーマを読めないため、カテゴリ別の件数を出せません",
    );
    expect(screen.queryByRole("img", { name: /未着手/ })).toBeNull();
  });

  it("shows the band once the schema arrives, even after its load failed", async () => {
    const { schemaArrives } = setup(all, undefined, true);
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "スキーマを読めないため、カテゴリ別の件数を出せません",
    );
    schemaArrives();
    expect(
      await screen.findByRole("img", { name: "未着手 1 · 進行中 3 · 完了 1 · 中止 1" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("lists the projects with progress, due date and assignee", async () => {
    setup(all);
    const section = await screen.findByRole("region", { name: "プロジェクト" });
    const row = within(section).getByRole("row", { name: /REL-15/ });
    expect(within(row).getByText("3/14")).toBeInTheDocument();
    expect(within(row).getByText(formatDate(`${year}-11-12`, new Date()))).toBeInTheDocument();
    expect(within(row).getByRole("img", { name: "sato" })).toBeInTheDocument();
    const href = router.href({ name: "project", ulid: "P1", tab: "tree" });
    expect(within(row).getByRole("link", { name: "リリース準備 v2.4" })).toHaveAttribute(
      "href",
      href,
    );
  });

  it("lists the tasks outside the projects in rank order, without the children of a project", async () => {
    setup(all);
    const section = await screen.findByRole("region", { name: "プロジェクトの外のタスク" });
    const ids = within(section)
      .getAllByRole("row")
      .slice(1)
      .map((r) => r.textContent ?? "");
    expect(ids).toHaveLength(4);
    expect(ids[0]).toContain("REL-22");
    expect(ids[1]).toContain("REL-18");
    expect(ids[2]).toContain("REL-3");
    expect(ids[3]).toContain("REL-4");
    expect(within(section).queryByText("子のタスク")).toBeNull();
    const tree = router.href({ name: "workspace", store: "shared", ulid: "W1", tab: "tree" });
    expect(within(section).getByRole("link", { name: "ツリーで開く" })).toHaveAttribute(
      "href",
      tree,
    );
  });

  it("follows tasks.changed: a task added from outside changes the counts and the list", async () => {
    const { rows, conn } = setup(all);
    await screen.findByText("6 件 · プロジェクト 1");

    rows.current = [
      ...all,
      tv("N1", "あとから足した", { id: "REL-30", rank: "f", status: "doing" }),
    ];
    conn.emit("tasks.changed", changed("shared", ["N1"], "c1"));

    expect(await screen.findByText("7 件 · プロジェクト 1")).toBeInTheDocument();
    expect(screen.getByText("進行中 4")).toBeInTheDocument();
    expect(screen.getByText("あとから足した")).toBeInTheDocument();
  });

  it("follows tasks.changed: a project that is gone leaves the list", async () => {
    const { rows, conn } = setup(all);
    await screen.findByRole("row", { name: /REL-15/ });

    rows.current = all.filter((t) => t.ulid !== "P1" && t.ulid !== "C1");
    conn.emit("tasks.changed", changed("shared", [], "c2", "rm", ["P1", "C1"]));

    expect(await screen.findByText("4 件 · プロジェクト 0")).toBeInTheDocument();
    expect(screen.getByText("プロジェクトはまだありません")).toBeInTheDocument();
  });

  it("keeps the query when only the entry changes (a rename, a new count)", async () => {
    const { cache, rerender, calls } = setup(all);
    const watch = vi.spyOn(cache, "watchList");
    await screen.findByText("6 件 · プロジェクト 1");
    const fetched = calls.filter((c) => c.path === "/api/tasks").length;

    await rerender({ entry: { ...entry, name: "release-2", tasks: 9 } });

    expect(screen.getByRole("heading", { name: /release-2/ })).toBeInTheDocument();
    expect(watch).not.toHaveBeenCalled();
    expect(calls.filter((c) => c.path === "/api/tasks")).toHaveLength(fetched);
  });

  it("opens the create dialogs from the buttons", async () => {
    setup(all);
    await screen.findByText("6 件 · プロジェクト 1");
    screen.getByRole("button", { name: "タスクを作成" }).click();
    expect(createState.request).toEqual({ kind: "task" });
    screen.getByRole("button", { name: "プロジェクトを作成" }).click();
    expect(createState.request).toEqual({ kind: "project" });
  });

  it("disables the create buttons of a read-only Store, with the reason", async () => {
    const sentence = "この Store は閲覧のみです（remote に書き込めません）";
    setup(all, undefined, false, "the remote cannot be written");
    await screen.findByText("6 件 · プロジェクト 1");
    for (const name of ["タスクを作成", "プロジェクトを作成"]) {
      const b = screen.getByRole("button", { name });
      expect(b).toBeDisabled();
      expect(b).toHaveAttribute("title", sentence);
    }
    screen.getByRole("button", { name: "タスクを作成" }).click();
    expect(createState.request).toBeNull();
  });

  it("disables the create button of the empty workspace of a read-only Store", async () => {
    setup([], undefined, false, "the remote cannot be written");
    await screen.findByText("release にはまだタスクがありません");
    const b = screen.getByRole("button", { name: /^タスクを作成 / });
    expect(b).toBeDisabled();
    expect(b).toHaveAttribute("title", "この Store は閲覧のみです（remote に書き込めません）");
  });

  it("shows the hh ls line for the workspace", async () => {
    setup(all);
    await screen.findByText("6 件 · プロジェクト 1");
    const line = screen.getByRole("button", { name: /^コマンドをコピー: hh ls/ });
    expect(line.getAttribute("aria-label")).toContain("workspace:release");
    expect(line.getAttribute("aria-label")).toContain("--tree");
    expect(line.getAttribute("aria-label")).toContain("--store shared");
  });

  it("says so when the workspace has no task", async () => {
    setup([]);
    expect(await screen.findByText("release にはまだタスクがありません")).toBeInTheDocument();
  });

  it("shows why the tasks could not be read", async () => {
    setup([], () => problem(500, "internal"));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByText("読み込み中…")).toBeNull());
  });
});
