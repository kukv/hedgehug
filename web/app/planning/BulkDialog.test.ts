import { cleanup, fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import { fakeApi } from "../../core/testing";
import { renderWithCore } from "../test-utils";
import BulkDialog from "./BulkDialog.svelte";

afterEach(cleanup);

const task = { ulid: "A", id: "T-1", store: "s", title: "one" };
const schema = {
  subscribe: () => () => {},
  load: async () => undefined,
  get: () => ({ statuses: [{ name: "done", label: "完了", category: "done" }], props: {} }),
};
const cache = { subscribe: () => () => {}, noteOwnCommit: vi.fn() };

function setup(answer: (dry: boolean) => unknown) {
  const { api, calls } = fakeApi((r) => answer((r.body as { dry_run: boolean }).dry_run));
  const onclose = vi.fn();
  renderWithCore(
    BulkDialog,
    { target: { ulids: ["A"], store: "s" }, onclose },
    { api, schema, cache },
  );
  return { calls, onclose };
}

const button = () => screen.getByRole("button", { name: /件を更新/ }) as HTMLButtonElement;

describe("BulkDialog", () => {
  it("shows the count, and sends the update after the dry run", async () => {
    const { calls, onclose } = setup((dry) => ({
      tasks: [task],
      commits: dry ? [] : [{ store: "s", commit: "abc" }],
    }));
    await fireEvent.change(screen.getByLabelText("値"), { target: { value: "done" } });
    await screen.findByText("1 件が対象です");
    await fireEvent.click(button());
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(calls.map((c) => (c.body as { dry_run: boolean }).dry_run)).toEqual([true, false]);
    expect(calls[1]?.body).toMatchObject({ ulids: ["A"], status: "done" });
  });

  it("does not run while a task breaks the schema, and lists it", async () => {
    setup(() => ({
      tasks: [task],
      violations: [{ task, field: "status", message: "points が必要です" }],
    }));
    await fireEvent.change(screen.getByLabelText("値"), { target: { value: "done" } });
    await screen.findByText("1 件が対象です · 違反 1 件");
    expect(screen.getByText("T-1: points が必要です")).toBeInTheDocument();
    expect(button().disabled).toBe(true);
  });

  it("offers the statuses of every Store for a query", async () => {
    const { api } = fakeApi(() => ({ tasks: [] }));
    const of = (name: string) => ({ statuses: [{ name, category: "todo" }], props: {} });
    renderWithCore(
      BulkDialog,
      { target: { query: "label:x" }, onclose: () => {} },
      {
        api,
        cache,
        schema: { ...schema, get: (s: string) => of(s === "a" ? "todo" : "review") },
        stores: { subscribe: () => () => {}, stores: () => [{ name: "a" }, { name: "b" }] },
      },
    );
    expect(await screen.findByRole("option", { name: "todo" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "review" })).toBeInTheDocument();
  });

  it("does not ask before a status is chosen", async () => {
    const { calls } = setup(() => ({ tasks: [] }));
    await new Promise((r) => setTimeout(r, 250));
    expect(calls).toEqual([]);
    expect(button().disabled).toBe(true);
  });
});
