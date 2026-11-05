import { cleanup, fireEvent, waitFor } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { WorkspaceEntry } from "../../core/stores";
import { renderWithCore } from "../test-utils";
import ReportsPage from "./ReportsPage.svelte";

// uPlot needs a canvas; the numbers it is given have their tests in core/reports.
vi.mock("./Plot.svelte", () => ({
  default: (anchor: Comment, props: { label: string }) => {
    const p = document.createElement("p");
    p.textContent = `plot: ${props.label}`;
    anchor.before(p);
  },
}));

afterEach(cleanup);

const entry = { store: "shared", name: "main", ulid: "W1" } as WorkspaceEntry;

const closed = {
  ulid: "S1",
  name: "Sprint 1",
  state: "closed",
  start: "2026-11-02",
  end: "2026-11-13",
  tasks: 1,
  done: 1,
  points: 3,
};

function setup(sprints: unknown[] = [closed]) {
  const get = vi.fn(async (path: string, init: { params: { query: Record<string, unknown> } }) => {
    const answers: Record<string, unknown> = {
      "/api/sprints": sprints,
      "/api/releases": [],
      "/api/reports/burndown": {
        sprint: closed,
        by: init?.params.query.by === "count" ? "count" : "points",
        days: [{ date: "2026-11-02", scope: 5, done: 2, remaining: 3, added: 1 }],
      },
      "/api/reports/velocity": [
        { sprint: closed, points: 12, tasks: 3, unpointed: 0 },
        { sprint: { ...closed, name: "Sprint 2" }, points: 8, tasks: 2, unpointed: 0 },
      ],
      "/api/reports/cfd": {
        by: "category",
        columns: ["todo", "done"],
        days: [{ date: "2026-11-02", counts: { todo: 1, done: 2 } }],
      },
      "/api/reports/cycle": {
        since: "2026-10-01",
        until: "2026-11-02",
        cycle: { count: 3, median: "36h", p85: "2h" },
        lead: { count: 3, median: "48h", p85: "72h" },
      },
      "/api/reports/estimate": {
        estimate: "2h",
        actual: "3h",
        tasks: [{ ulid: "A", id: "T-1", title: "alpha", estimate: "2h", actual: "3h" }],
      },
    };
    return { data: answers[path], response: { ok: true, status: 200 } };
  });
  const view = renderWithCore(
    ReportsPage,
    { entry },
    { api: { GET: get }, conn: { on: () => () => {} } },
  );
  const calls = (path: string) =>
    get.mock.calls.filter((c) => c[0] === path).map((c) => c[1].params.query);
  return { ...view, calls };
}

describe("ReportsPage", () => {
  it("shows all five reports of the workspace", async () => {
    const { findByText, getByText, calls } = setup();
    expect(await findByText("plot: ベロシティ")).toBeTruthy();
    expect(await findByText("plot: 累積フロー図")).toBeTruthy();
    expect(await findByText("1.5 日")).toBeTruthy();
    expect(await findByText(/2h → 3h（超過）/)).toBeTruthy();
    expect(getByText("直近 3 スプリントの平均").textContent).toContain("10 pt");
    expect(calls("/api/reports/velocity")[0]).toMatchObject({ workspace: "main", last: 5 });
  });

  it("asks again when a condition changes, as the flags of hh report", async () => {
    const { findByText, getByRole, getByLabelText, calls } = setup();
    await findByText("plot: ベロシティ");

    await fireEvent.click(getByRole("button", { name: "件数" }));
    await waitFor(() =>
      expect(calls("/api/reports/burndown").at(-1)).toMatchObject({
        sprint: "Sprint 1",
        by: "count",
      }),
    );
    await fireEvent.click(getByRole("button", { name: "ステータス" }));
    await waitFor(() => expect(calls("/api/reports/cfd").at(-1)).toMatchObject({ by: "status" }));
    await fireEvent.change(getByLabelText("期間"), { target: { value: "10" } });
    await waitFor(() => expect(calls("/api/reports/velocity").at(-1)).toMatchObject({ last: 10 }));
    await fireEvent.change(getByLabelText("対象"), { target: { value: "Sprint 1" } });
    await waitFor(() =>
      expect(calls("/api/reports/estimate").at(-1)).toMatchObject({ sprint: "Sprint 1" }),
    );
    await fireEvent.input(getByLabelText("開始"), { target: { value: "2026-10-05" } });
    await waitFor(() =>
      expect(calls("/api/reports/cycle").at(-1)).toMatchObject({ since: "2026-10-05" }),
    );
  });

  it("does not ask for a burndown without a started sprint", async () => {
    const { findByText, calls } = setup([{ ...closed, state: "planned" }]);
    expect(await findByText("開始したスプリントはありません")).toBeTruthy();
    expect(calls("/api/reports/burndown")).toEqual([]);
  });
});
