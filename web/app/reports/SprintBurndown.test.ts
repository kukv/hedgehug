import { cleanup, waitFor } from "@testing-library/svelte";
import { flushSync } from "svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import { toApiError } from "../../core/api/client";
import type { Sprint } from "../../core/planning";
import type { WorkspaceEntry } from "../../core/stores";
import { plansWritten } from "../planning/plans.svelte";
import { renderWithCore } from "../test-utils";
import SprintBurndown from "./SprintBurndown.svelte";

vi.mock("./Plot.svelte", () => ({
  default: (anchor: Comment, props: { label: string }) => {
    const p = document.createElement("p");
    p.textContent = `plot: ${props.label}`;
    anchor.before(p);
  },
}));

afterEach(cleanup);

const entry = { store: "shared", name: "main", ulid: "W1" } as WorkspaceEntry;
const sprint = { ulid: "S1", name: "Sprint 1", state: "active" } as Sprint;

function setup(answer: { by?: string; failed?: boolean } = {}) {
  const listeners = new Map<string, (e: { store: string }) => void>();
  const get = vi.fn((_query: unknown) =>
    answer.failed
      ? Promise.reject(toApiError(409, { type: "report.not_started", title: "", status: 409 }))
      : Promise.resolve({
          sprint,
          by: answer.by ?? "points",
          days: [
            { date: "2026-11-02", scope: 8, done: 0, remaining: 8, added: 0 },
            { date: "2026-11-03", scope: 11, done: 2, remaining: 9, added: 3 },
          ],
        }),
  );
  const view = renderWithCore(
    SprintBurndown,
    { entry, sprint },
    {
      api: { reportBurndown: get },
      conn: {
        on: (name: string, fn: (e: { store: string }) => void) => {
          listeners.set(name, fn);
          return () => listeners.delete(name);
        },
      },
    },
  );
  return { ...view, get, listeners };
}

describe("SprintBurndown", () => {
  it("shows what is left, what joined and the chart", async () => {
    const { findByText, getByText, get } = setup();
    expect(await findByText("plot: Sprint 1 のバーンダウン")).toBeTruthy();
    expect(getByText("残り points")).toBeTruthy();
    expect(getByText("9 pt")).toBeTruthy();
    expect(getByText("/ 11 pt")).toBeTruthy();
    expect(getByText("+3 pt")).toBeTruthy();
    expect(get.mock.calls[0]?.[0]).toEqual({
      store: "shared",
      workspace: "main",
      sprint: "Sprint 1",
    });
  });

  it("says when it counts tasks, as a sprint without points does", async () => {
    const { findByText } = setup({ by: "count" });
    expect(await findByText("残り 件数")).toBeTruthy();
    expect(await findByText("+3 件")).toBeTruthy();
  });

  it("reads again after a commit of the Store, and not of another", async () => {
    const { findByText, get, listeners } = setup();
    await findByText("plot: Sprint 1 のバーンダウン");
    const n = get.mock.calls.length;
    listeners.get("tasks.changed")?.({ store: "other" });
    flushSync();
    expect(get).toHaveBeenCalledTimes(n);
    listeners.get("tasks.changed")?.({ store: "shared" });
    await waitFor(() => expect(get).toHaveBeenCalledTimes(n + 1));
  });

  it("reads again after a write of this tab", async () => {
    const { findByText, get } = setup();
    await findByText("plot: Sprint 1 のバーンダウン");
    const n = get.mock.calls.length;
    plansWritten();
    await waitFor(() => expect(get).toHaveBeenCalledTimes(n + 1));
  });

  it("says why it has nothing to show", async () => {
    const { findByRole } = setup({ failed: true });
    expect((await findByRole("alert")).textContent).toContain("まだ開始していません");
  });
});
