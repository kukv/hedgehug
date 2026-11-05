import { cleanup, fireEvent, waitFor } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import { tv } from "../../core/testing";
import { DEFAULT_GANTT, type GanttSettings } from "../../core/views";
import { emitter, renderWithCore } from "../test-utils";
import GanttView from "./GanttView.svelte";

afterEach(cleanup);

vi.stubGlobal(
  "ResizeObserver",
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
);

const workspace = {
  store: "shared",
  tone: "purple",
  ulid: "W1",
  name: "main",
  prefix: "T",
  projects: [],
};

type Roots = { ulid: string; children: Roots[] }[];

function setup(
  over: {
    tasks?: TaskView[];
    roots?: Roots;
    readOnly?: string | null;
    settings?: GanttSettings;
    sprints?: unknown[];
    releases?: unknown[];
  } = {},
) {
  const tasks = over.tasks ?? [
    tv("E", "epic", { kind: "epic" }),
    tv("A", "alpha", { id: "T-1", props: { start: "2026-11-02", due: "2026-11-04" } }),
    tv("B", "beta", { id: "T-2", props: { start: "2026-11-03", due: "2026-11-05" } }),
  ];
  const roots = over.roots ?? [
    {
      ulid: "E",
      children: [
        { ulid: "A", children: [] },
        { ulid: "B", children: [] },
      ],
    },
  ];
  const patch = vi.fn(async (_: string, _init: { body: unknown; params: unknown }) => ({
    data: { task: tasks[1], commit: "c1" },
    response: { ok: true, status: 200 },
  }));
  const get = vi.fn(async (path: string) => ({
    data: path === "/api/sprints" ? (over.sprints ?? []) : (over.releases ?? []),
    response: { ok: true, status: 200 },
  }));
  const applyWrite = vi.fn();
  const e = emitter();
  const core = {
    api: { PATCH: patch, GET: get },
    cache: {
      ...e,
      task: (u: string) => tasks.find((t) => t.ulid === u),
      applyWrite,
      watchDetail: () => ({ error: () => null, dispose() {} }),
      watchTree: () => ({
        roots: () => roots,
        cycle: () => [],
        error: () => null,
        loading: () => false,
        dispose() {},
      }),
    },
    stores: {
      ...emitter(),
      workspace: () => workspace,
      workspaces: () => [workspace],
      stores: () => [{ name: "shared" }],
      loaded: () => true,
      readOnly: () => over.readOnly ?? null,
      displayId: (_: string, id: string) => id,
    },
    schema: {
      ...emitter(),
      get: () => ({
        kinds: [
          { name: "epic", role: "container" },
          { name: "task", role: "work" },
        ],
        statuses: [{ name: "todo", category: "todo" }],
        props: { start: { type: "date" }, due: { type: "date" }, points: { type: "number" } },
      }),
      load: async () => {},
    },
    conn: { on: () => () => {} },
  };
  const onsettings = vi.fn();
  const view = renderWithCore(
    GanttView,
    {
      view: { kind: "workspace", store: "shared", ulid: "W1" },
      query: "-status:canceled",
      onquery: () => {},
      settings: over.settings ?? DEFAULT_GANTT,
      onsettings,
    },
    core,
  );
  return { ...view, patch, applyWrite, onsettings };
}

const barName = /T-1 alpha 11\/02 – 11\/04/;

describe("GanttView", () => {
  it("lists the tree beside bars that move, and a parent that adds up does not", () => {
    const { getByRole, getByText, container } = setup();
    expect(getByRole("list", { name: "ガントのタスク" })).toBeTruthy();
    expect(getByText("alpha")).toBeTruthy();
    expect(getByRole("button", { name: barName })).toBeTruthy();
    const parent = container.querySelector('[data-ulid="E"]');
    expect(parent?.getAttribute("data-kind")).toBe("rollup");
    expect(parent?.getAttribute("role")).toBeNull();
    expect(parent?.querySelector("title")?.textContent).toContain("子から集計");
  });

  it("folds the children of a row", async () => {
    const { getByRole, queryByText } = setup();
    await fireEvent.click(getByRole("button", { name: "epic の子を畳む" }));
    expect(queryByText("alpha")).toBeNull();
    expect(getByRole("button", { name: "epic の子を開く" })).toBeTruthy();
  });

  it("changes the settings from its controls", async () => {
    const { getByRole, getByLabelText, onsettings } = setup();
    await fireEvent.click(getByRole("button", { name: "ロードマップ" }));
    expect(onsettings).toHaveBeenLastCalledWith({ ...DEFAULT_GANTT, roadmap: true });
    await fireEvent.change(getByLabelText("開始"), { target: { value: "due" } });
    expect(onsettings).toHaveBeenLastCalledWith({ ...DEFAULT_GANTT, start: "due" });
  });

  it("keeps a move from the keyboard in one PATCH, on Enter", async () => {
    const { getByRole, patch, applyWrite } = setup();
    const bar = getByRole("button", { name: barName });
    await fireEvent.keyDown(bar, { key: "ArrowRight" });
    await fireEvent.keyDown(bar, { key: "ArrowRight" });
    expect(patch).not.toHaveBeenCalled();
    await fireEvent.keyDown(bar, { key: "Enter" });
    await waitFor(() => expect(patch).toHaveBeenCalledTimes(1));
    expect(patch.mock.calls[0]?.[1].body).toEqual({
      props: { start: "2026-11-04", due: "2026-11-06" },
    });
    await waitFor(() => expect(applyWrite).toHaveBeenCalledTimes(1));
  });

  it("moves only the end with Shift, and Esc takes a move back", async () => {
    const { getByRole, patch } = setup();
    const bar = getByRole("button", { name: barName });
    await fireEvent.keyDown(bar, { key: "ArrowRight" });
    await fireEvent.keyDown(bar, { key: "Escape" });
    await fireEvent.keyDown(bar, { key: "Enter" });
    expect(patch).not.toHaveBeenCalled();
    await fireEvent.keyDown(bar, { key: "ArrowRight", shiftKey: true });
    await fireEvent.keyDown(bar, { key: "Enter" });
    await waitFor(() => expect(patch).toHaveBeenCalledTimes(1));
    expect(patch.mock.calls[0]?.[1].body).toEqual({ props: { due: "2026-11-05" } });
  });

  it("offers no move on a Store that cannot be written", () => {
    const { queryByRole, container } = setup({ readOnly: "read_only" });
    expect(queryByRole("button", { name: barName })).toBeNull();
    expect(container.querySelector('[data-ulid="A"]')).toBeTruthy();
  });

  it("says in words what is late, and draws the arrow", () => {
    const tasks = [
      tv("A", "alpha", { id: "T-1", props: { start: "2026-11-02", due: "2026-11-04" } }),
      tv("B", "beta", {
        id: "T-2",
        blocked_by: [{ ulid: "A", id: "T-1", title: "alpha" }],
        props: { start: "2026-11-03", due: "2026-11-05" },
      }),
    ];
    const { getByRole, container } = setup({
      tasks,
      roots: [
        { ulid: "A", children: [] },
        { ulid: "B", children: [] },
      ],
    });
    expect(getByRole("list", { name: "ガントの警告" }).textContent).toContain(
      "ブロックしている T-1 の終了（11/04）が、T-2 の開始（11/03）より後です。",
    );
    expect(container.querySelector("path.link[data-late]")).toBeTruthy();
  });

  it("draws the sprint as a band, the release as a line, and a dateless task over its sprint", async () => {
    const dated = tv("A", "alpha", {
      sprint: { ulid: "S1", name: "Sprint 1", state: "active" },
    });
    const { container } = setup({
      tasks: [dated],
      roots: [{ ulid: "A", children: [] }],
      sprints: [
        { ulid: "S1", name: "Sprint 1", state: "active", start: "2026-11-02", end: "2026-11-13" },
      ],
      releases: [{ ulid: "R1", name: "v1", state: "unreleased", date: "2026-11-12" }],
    });
    await waitFor(() =>
      expect(container.querySelector('[data-ulid="A"]')?.getAttribute("data-kind")).toBe("sprint"),
    );
    const titles = [...container.querySelectorAll("title")].map((t) => t.textContent);
    expect(titles).toContain("Sprint 1 11/02 – 11/13");
    expect(titles).toContain("v1 リリース予定 11/12");
  });
});
