import { cleanup, fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SchemaView } from "../../core/schema";
import type { SchemaUsage } from "../../core/schema-usage";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import SchemaList from "./SchemaList.svelte";

const sch = {
  statuses: [],
  priorities: ["low", "mid", "high"],
  labels: ["bug", "ux"],
  props: {},
  workflow: {},
  scoped: [],
} as unknown as SchemaView;

const usage = {
  statuses: {},
  props: {},
  values: {},
  priorities: { low: 0, mid: 0, high: 2 },
  labels: { bug: 3, ux: 0 },
} as SchemaUsage;

function setup(
  kind: "priorities" | "labels",
  over: {
    schema?: SchemaView;
    usage?: SchemaUsage | null;
    off?: string | null;
    api?: Record<string, unknown>;
  } = {},
) {
  const w = () => ok({ schema: sch, commit: "c1" });
  const api = { changePriorities: vi.fn(w), changeLabels: vi.fn(w), ...over.api };
  const reload = vi.fn(async () => {});
  const ondone = vi.fn();
  const schema = over.schema ?? sch;
  const view = renderWithCore(
    SchemaList,
    {
      store: "shared",
      kind,
      schema,
      usage: over.usage === undefined ? usage : over.usage,
      off: over.off ?? null,
      ondone,
    },
    { api, schema: { reload, get: () => schema, subscribe: () => () => {} } },
  );
  return { api, reload, ondone, ...view };
}

const rows = () => screen.queryAllByRole("textbox", { name: "値" }) as HTMLInputElement[];
const names = () => rows().map((r) => r.value);
const save = () => screen.getByRole("button", { name: "保存" });

describe("SchemaList: the priorities", () => {
  beforeEach(() => resetToasts());
  afterEach(cleanup);

  it("shows the priorities in order, and keeps the save off until something changes", () => {
    setup("priorities");
    expect(screen.getByRole("heading", { name: "優先度" })).toBeVisible();
    expect(names()).toEqual(["low", "mid", "high"]);
    expect(save()).toBeDisabled();
  });

  it("adds, renames and saves the whole list in order, with the values it comes from", async () => {
    const { api, reload, ondone } = setup("priorities");
    await fireEvent.input(rows()[1] as HTMLElement, { target: { value: "medium" } });
    await fireEvent.click(screen.getByRole("button", { name: "値を追加" }));
    await fireEvent.input(rows()[3] as HTMLElement, { target: { value: "urgent" } });
    await fireEvent.click(save());
    await waitFor(() => expect(api.changePriorities).toHaveBeenCalledTimes(1));
    expect(api.changePriorities).toHaveBeenCalledWith(
      {
        values: [
          { name: "low", from: ["low"] },
          { name: "medium", from: ["mid"] },
          { name: "high", from: ["high"] },
          { name: "urgent" },
        ],
      },
      { store: "shared" },
    );
    await waitFor(() => expect(ondone).toHaveBeenCalled());
    expect(reload).toHaveBeenCalledWith("shared");
    expect(toasts.list.map((t) => t.message)).toContain("優先度を保存しました");
    expect(api.changeLabels).not.toHaveBeenCalled();
  });

  it("does not delete a priority that a task uses, and merges it into another", async () => {
    const { api } = setup("priorities");
    const del = screen.getByRole("button", { name: "削除（値 high）" });
    expect(del).toBeDisabled();
    expect(del.getAttribute("title")).toContain("タスクが使っているため削除できません");
    await fireEvent.click(screen.getByRole("button", { name: "マージ（値 high）" }));
    const to = screen.getByRole("combobox", { name: "マージ先（値 high）" }) as HTMLSelectElement;
    const mid = [...to.options].find((o) => o.textContent === "mid");
    await fireEvent.change(to, { target: { value: mid?.value } });
    await fireEvent.click(save());
    await waitFor(() => expect(api.changePriorities).toHaveBeenCalled());
    expect(api.changePriorities).toHaveBeenCalledWith(
      {
        values: [
          { name: "low", from: ["low"] },
          { name: "mid", from: ["mid", "high"] },
        ],
      },
      { store: "shared" },
    );
  });

  it("deletes a priority that nobody uses, but never the last one", async () => {
    setup("priorities", { schema: { ...sch, priorities: ["low", "mid"] } as SchemaView });
    await fireEvent.click(screen.getByRole("button", { name: "削除（値 low）" }));
    expect(names()).toEqual(["mid"]);
    const last = screen.getByRole("button", { name: "削除（値 mid）" });
    expect(last).toBeDisabled();
    expect(last.getAttribute("title")).toBe("優先度は 1 つ以上必要です");
  });

  it("does not save a value that is empty or twice, case ignored", async () => {
    setup("priorities");
    await fireEvent.input(rows()[0] as HTMLElement, { target: { value: "HIGH" } });
    expect(
      screen.getByText("値が空か、重複しています（大文字小文字は区別しません）"),
    ).toBeVisible();
    expect(save()).toBeDisabled();
    await fireEvent.input(rows()[0] as HTMLElement, { target: { value: "" } });
    expect(save()).toBeDisabled();
    await fireEvent.input(rows()[0] as HTMLElement, { target: { value: "minor" } });
    expect(save()).toBeEnabled();
  });

  it("says why the server refused, and keeps what was typed", async () => {
    const { ondone } = setup("priorities", {
      api: {
        changePriorities: vi.fn(() =>
          fail(422, "schema.in_use", { count: 2, deleted: 0, value: "low" }),
        ),
      },
    });
    await fireEvent.input(rows()[0] as HTMLElement, { target: { value: "minor" } });
    await fireEvent.click(save());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "値 low は、タスク 2 件が使っているため",
    );
    expect(names()[0]).toBe("minor");
    expect(ondone).not.toHaveBeenCalled();
  });

  it("only reads in a Store that cannot be written, and says why", () => {
    setup("priorities", { off: "このStoreは閲覧のみです" });
    expect(screen.getByText("このStoreは閲覧のみです")).toBeVisible();
    expect(save()).toBeDisabled();
    for (const r of rows()) expect(r).toBeDisabled();
    expect(screen.getByRole("button", { name: "値を追加" })).toBeDisabled();
  });
});

describe("SchemaList: the labels", () => {
  beforeEach(() => resetToasts());
  afterEach(cleanup);

  it("saves the labels through the labels call", async () => {
    const { api } = setup("labels");
    expect(screen.getByRole("heading", { name: "ラベル" })).toBeVisible();
    expect(names()).toEqual(["bug", "ux"]);
    await fireEvent.click(screen.getByRole("button", { name: "削除（値 ux）" }));
    await fireEvent.click(save());
    await waitFor(() => expect(api.changeLabels).toHaveBeenCalledTimes(1));
    expect(api.changeLabels).toHaveBeenCalledWith(
      { values: [{ name: "bug", from: ["bug"] }] },
      { store: "shared" },
    );
    expect(api.changePriorities).not.toHaveBeenCalled();
    expect(toasts.list.map((t) => t.message)).toContain("ラベルを保存しました");
  });

  it("counts the tasks of a label: one that a task holds is not deleted", () => {
    setup("labels");
    expect(screen.getByRole("button", { name: "削除（値 bug）" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "削除（値 ux）" })).toBeEnabled();
  });

  it("may be empty, and may be emptied: the last label can go", async () => {
    const { api } = setup("labels", { schema: { ...sch, labels: ["ux"] } as SchemaView });
    await fireEvent.click(screen.getByRole("button", { name: "削除（値 ux）" }));
    expect(names()).toEqual([]);
    expect(screen.getByText("ラベルはまだありません。「値を追加」で足します。")).toBeVisible();
    await fireEvent.click(save());
    await waitFor(() =>
      expect(api.changeLabels).toHaveBeenCalledWith({ values: [] }, { store: "shared" }),
    );
  });

  it("starts from an empty list for a schema with no labels", () => {
    setup("labels", { schema: { ...sch, labels: null } as SchemaView });
    expect(screen.getByText("ラベルはまだありません。「値を追加」で足します。")).toBeVisible();
  });
});
