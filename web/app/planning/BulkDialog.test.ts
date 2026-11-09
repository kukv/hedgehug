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
  get: () => ({
    statuses: [{ name: "done", label: "完了", category: "done" }],
    priorities: ["low", "high"],
    labels: ["bug", "ux"],
    props: { area: { type: "text" }, effort: { type: "number" } },
  }),
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

  it("lists the built-in properties in their fixed order, then the properties of the schema", () => {
    setup(() => ({ tasks: [] }));
    const names = [...(screen.getByLabelText("項目") as HTMLSelectElement).options].map(
      (o) => o.textContent,
    );
    expect(names).toEqual([
      "ステータス",
      "スプリント",
      "リリース",
      "担当者",
      "優先度",
      "開始日",
      "期限",
      "見積もり",
      "ポイント",
      "ラベル",
      "area",
      "effort",
    ]);
  });

  it("sets a built-in property at the top of the body: the priority from the list, a date, the points as a number, null to clear", async () => {
    const { calls } = setup(() => ({ tasks: [task] }));
    const pick = async (field: string) =>
      fireEvent.change(screen.getByLabelText("項目"), { target: { value: field } });
    const last = () => calls.at(-1)?.body;

    await pick("priority");
    expect(
      [...(screen.getByLabelText("値") as HTMLSelectElement).options].map((o) => o.value),
    ).toEqual(["", "low", "high"]);
    await fireEvent.change(screen.getByLabelText("値"), { target: { value: "high" } });
    await waitFor(() => expect(last()).toMatchObject({ priority: "high", dry_run: true }));

    await pick("due");
    await fireEvent.input(screen.getByLabelText("値"), { target: { value: "2026-11-20" } });
    await waitFor(() => expect(last()).toMatchObject({ due: "2026-11-20" }));
    await fireEvent.input(screen.getByLabelText("値"), { target: { value: "" } });
    await waitFor(() => expect(last()).toMatchObject({ due: null }));

    await pick("points");
    await fireEvent.input(screen.getByLabelText("値"), { target: { value: "0" } });
    await waitFor(() => expect(last()).toMatchObject({ points: 0 }));

    await pick("estimate");
    await fireEvent.input(screen.getByLabelText("値"), { target: { value: "1h30m" } });
    await waitFor(() => expect(last()).toMatchObject({ estimate: "1h30m" }));
    expect(calls.every((c) => (c.body as { props?: unknown }).props === undefined)).toBe(true);
  });

  it("does not send a value that is not a number as the points or a number property: it would clear them", async () => {
    const { calls } = setup(() => ({ tasks: [task] }));
    for (const [field, text] of [
      ["points", "abc"],
      ["points", "1,5"],
      ["props.effort", "x"],
    ] as const) {
      await fireEvent.change(screen.getByLabelText("項目"), { target: { value: field } });
      await fireEvent.input(screen.getByLabelText("値"), { target: { value: text } });
      expect(screen.getByText("数を入れてください")).toBeVisible();
    }
    await new Promise((r) => setTimeout(r, 300));
    expect(calls).toEqual([]);
    expect(button().disabled).toBe(true);
    // a number clears the warning and is sent
    await fireEvent.input(screen.getByLabelText("値"), { target: { value: "2.5" } });
    expect(screen.queryByText("数を入れてください")).toBeNull();
    await waitFor(() => expect(calls.at(-1)?.body).toMatchObject({ props: { effort: 2.5 } }));
  });

  it("chooses the labels from the list of the schema, and sends the whole list", async () => {
    const { calls } = setup(() => ({ tasks: [task] }));
    await fireEvent.change(screen.getByLabelText("項目"), { target: { value: "labels" } });
    expect(screen.queryByRole("textbox")).toBeNull();
    await fireEvent.change(screen.getByRole("combobox", { name: "ラベル" }), {
      target: { value: "bug" },
    });
    await fireEvent.change(screen.getByRole("combobox", { name: "ラベル" }), {
      target: { value: "ux" },
    });
    await waitFor(() => expect(calls.at(-1)?.body).toMatchObject({ labels: ["bug", "ux"] }));
  });

  it("still puts a property of the schema in props", async () => {
    const { calls } = setup(() => ({ tasks: [task] }));
    await fireEvent.change(screen.getByLabelText("項目"), { target: { value: "props.effort" } });
    await fireEvent.input(screen.getByLabelText("値"), { target: { value: "3" } });
    await waitFor(() => expect(calls.at(-1)?.body).toMatchObject({ props: { effort: 3 } }));
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
