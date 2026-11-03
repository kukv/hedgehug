import { cleanup, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import { FakeConn } from "../../core/testing";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import { task } from "../tree/fixtures";
import ProjectViolations from "./ProjectViolations.svelte";

const bad = (id: string, message: string) =>
  ({
    ...task(`U-${id}`, "task", "P1", { id, title: `title of ${id}` }),
    problems: [{ code: "required", field: "points", message }],
  }) as TaskView;

function setup(violations: TaskView[] = [], store = "shared") {
  const conn = new FakeConn();
  const api = { GET: vi.fn((..._: unknown[]) => ok({ tasks: violations })) };
  const r = renderWithCore(ProjectViolations, { ulid: "P1", store }, { api, conn });
  return { ...r, api, conn };
}

const tasksChanged = (store: string) => ({ store, ulids: [], removed: [] });

beforeEach(() => history.replaceState(null, "", "/projects/P1/settings"));
afterEach(cleanup);

describe("ProjectViolations", () => {
  it("asks for the invalid tasks of the project, in its Store", async () => {
    const { api } = setup();
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(1));
    expect(api.GET).toHaveBeenCalledWith("/api/tasks", {
      params: { query: { q: "is:invalid project:P1", store: "shared" } },
    });
  });

  it("lists the tasks that break the rules, with links, or says there are none", async () => {
    setup([bad("REL-11", "points is required to enter done")]);
    const list = await screen.findByRole("list", { name: "違反しているタスク" });
    const row = within(list).getByRole("listitem");
    expect(within(row).getByRole("link", { name: "REL-11" })).toHaveAttribute(
      "href",
      "/tasks/U-REL-11",
    );
    expect(row.textContent).toMatch(/points is required to enter done/);
    expect(screen.getByText("いまの違反 1 件")).toBeTruthy();
    cleanup();
    setup();
    expect(await screen.findByText("違反しているタスクはありません")).toBeTruthy();
  });

  it("reads again at schema.changed, tasks.changed and resync of its Store", async () => {
    const { api, conn } = setup();
    await screen.findByText("違反しているタスクはありません");
    conn.emit("schema.changed", { store: "shared" });
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(2));
    conn.emit("tasks.changed", tasksChanged("shared"));
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(3));
    conn.emit("resync");
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(4));
  });

  it("leaves a change of another Store alone", async () => {
    const { api, conn } = setup();
    await screen.findByText("違反しているタスクはありません");
    conn.emit("schema.changed", { store: "backend" });
    conn.emit("tasks.changed", tasksChanged("backend"));
    await new Promise((r) => setTimeout(r, 10));
    expect(api.GET).toHaveBeenCalledTimes(1);
  });

  it("stops reading when the screen is gone", async () => {
    const { api, conn, unmount } = setup();
    await waitFor(() => expect(api.GET).toHaveBeenCalledTimes(1));
    unmount();
    conn.emit("schema.changed", { store: "shared" });
    conn.emit("tasks.changed", tasksChanged("shared"));
    conn.emit("resync");
    await new Promise((r) => setTimeout(r, 10));
    expect(api.GET).toHaveBeenCalledTimes(1);
  });

  it("shows the answer of the latest read when an older one arrives last", async () => {
    const { api, conn } = setup();
    await screen.findByText("違反しているタスクはありません");
    const pending: ((v: unknown) => void)[] = [];
    api.GET.mockImplementation((() => new Promise((r) => pending.push(r))) as never);
    conn.emit("resync");
    conn.emit("resync");
    await waitFor(() => expect(pending).toHaveLength(2));
    pending[1]?.(await ok({ tasks: [bad("REL-12", "the new one")] }));
    await screen.findByText("the new one");
    pending[0]?.(await ok({ tasks: [bad("REL-11", "the old one")] }));
    await new Promise((r) => setTimeout(r, 10));
    expect(screen.getByText("the new one")).toBeTruthy();
    expect(screen.queryByText("the old one")).toBeNull();
  });

  it("keeps the list and shows the error when a new read fails", async () => {
    const { api, conn } = setup([bad("REL-11", "the one that stays")]);
    await screen.findByText("the one that stays");
    api.GET.mockImplementation((() => fail(500, "internal")) as never);
    conn.emit("resync");
    await waitFor(() => expect(screen.getAllByRole("alert").length).toBeGreaterThan(0));
    expect(screen.getByText("the one that stays")).toBeTruthy();
  });

  it("drops the list of a Store when the project is in another one", async () => {
    const { api, rerender } = setup([bad("REL-11", "from the first Store")]);
    await screen.findByText("from the first Store");
    api.GET.mockImplementation((() => new Promise(() => {})) as never);
    await rerender({ ulid: "P1", store: "backend" });
    expect(screen.queryByText("from the first Store")).toBeNull();
    expect(screen.getByText("読み込み中…")).toBeTruthy();
  });
});
