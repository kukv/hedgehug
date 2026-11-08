import { cleanup, fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SchemaView } from "../../core/schema";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import SchemaDeleteDialog from "./SchemaDeleteDialog.svelte";

const sch = {
  statuses: [
    { name: "todo", category: "todo", default: true },
    { name: "on_hold", category: "doing" },
    { name: "done", category: "done", default: true },
  ],
  kinds: [
    { name: "epic", role: "container", parents: ["root"] },
    { name: "sub", role: "work", parents: ["epic"] },
    { name: "task", role: "work", parents: null, default: true },
  ],
  props: { priority: { type: "enum", values: ["low", "high"] } },
  workflow: {
    transitions: { todo: ["on_hold"], on_hold: ["done"] },
    required: { on_hold: ["priority"] },
  },
  scoped: [
    {
      where: "project:P1",
      statuses: ["todo", "on_hold"],
      workflow: { transitions: { todo: ["on_hold"] }, required: {} },
    },
  ],
} as unknown as SchemaView;

function setup(
  target: "status" | "kind" | "prop" | "value",
  name: string,
  prop?: string,
  over: Record<string, unknown> = {},
) {
  const api = {
    deleteStatus: vi.fn(() => ok({ schema: sch, commit: "c" })),
    deleteKind: vi.fn(() => ok({ schema: sch, commit: "c" })),
    deleteProp: vi.fn(() => ok({ schema: sch, commit: "c" })),
    deleteValue: vi.fn(() => ok({ schema: sch, commit: "c" })),
    ...over,
  };
  const reload = vi.fn(async () => {});
  const onclose = vi.fn();
  const ondone = vi.fn();
  renderWithCore(
    SchemaDeleteDialog,
    {
      store: "shared",
      target,
      name,
      prop,
      schema: sch,
      projectLabel: (ulid: string) => (ulid === "P1" ? "REL-15 リリース準備" : ulid),
      onclose,
      ondone,
    },
    { api, schema: { reload, subscribe: () => () => {} } },
  );
  return { api, reload, onclose, ondone };
}

describe("SchemaDeleteDialog", () => {
  beforeEach(() => resetToasts());
  afterEach(cleanup);

  it("lists the references and deletes", async () => {
    const { api, reload, onclose } = setup("status", "on_hold");
    expect(screen.getByRole("dialog", { name: "ステータス「on_hold」を削除" })).toBeVisible();
    expect(screen.getByText("次の参照も外れます")).toBeVisible();
    expect(screen.getAllByRole("listitem")).toHaveLength(5);
    expect(screen.getAllByText(/REL-15 リリース準備/).length).toBeGreaterThan(0);
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() =>
      expect(api.deleteStatus).toHaveBeenCalledWith("on_hold", { store: "shared" }),
    );
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(reload).toHaveBeenCalledWith("shared");
    expect(toasts.list.map((t) => t.message)).toContain("ステータス「on_hold」を削除しました");
  });

  it("deletes a value of a property", async () => {
    const { api } = setup("value", "low", "priority");
    expect(screen.getByRole("dialog", { name: "priority の値「low」を削除" })).toBeVisible();
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    await waitFor(() =>
      expect(api.deleteValue).toHaveBeenCalledWith("priority", "low", { store: "shared" }),
    );
  });

  it("refuses a kind that leaves another with no parent", () => {
    setup("kind", "epic");
    expect(screen.getByRole("button", { name: "削除" })).toBeDisabled();
    expect(
      screen.getByText(
        "種類 sub の置ける親がなくなるため削除できません。先に sub の置ける親を変えてください",
      ),
    ).toBeVisible();
  });

  it("shows the server's in_use", async () => {
    const { onclose } = setup("status", "on_hold", undefined, {
      deleteStatus: vi.fn(() =>
        fail(
          422,
          "schema.in_use",
          { count: 2, deleted: 1 },
          "2 tasks uses on_hold; merge it into another first",
        ),
      ),
    });
    await fireEvent.click(screen.getByRole("button", { name: "削除" }));
    expect(
      await screen.findByText("2 tasks uses on_hold; merge it into another first"),
    ).toBeVisible();
    expect(onclose).not.toHaveBeenCalled();
  });
});
