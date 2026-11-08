import { cleanup, fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SchemaView } from "../../core/schema";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import SchemaDefDialog from "./SchemaDefDialog.svelte";

const sch = {
  statuses: [
    { name: "todo", label: "未着手", category: "todo", default: true },
    { name: "in_progress", label: "進行中", category: "doing", color: "#3451D1" },
    { name: "done", label: "完了", category: "done", default: true },
  ],
  kinds: [
    { name: "project", label: "プロジェクト", role: "container", parents: ["root"] },
    { name: "epic", label: "エピック", role: "container", parents: ["root", "project"] },
    { name: "task", label: "タスク", role: "work", parents: null, default: true },
    { name: "sub", label: "サブ", role: "work", parents: ["task"], template: "bug" },
  ],
  props: {
    priority: { type: "enum", values: ["low", "high"] },
    relates: { type: "task_ref", multi: true, symmetric: true, label: "関連" },
    duplicates: { type: "task_ref", label: "重複", inverse: "重複元" },
  },
  prop_order: ["priority", "relates", "duplicates"],
  workflow: {},
  scoped: [],
} as unknown as SchemaView;

type Target = "status" | "kind" | "prop" | "relation";

function setup(
  target: Target,
  name?: string,
  over: Record<string, unknown> = {},
  schema: SchemaView = sch,
) {
  const w = () => ok({ schema: sch, commit: "c1" });
  const api = {
    addStatus: vi.fn(w),
    changeStatus: vi.fn(w),
    addKind: vi.fn(w),
    changeKind: vi.fn(w),
    addProp: vi.fn(w),
    changeProp: vi.fn(w),
    listTemplates: vi.fn(() =>
      ok([
        { name: "bug", children: 0 },
        { name: "feature", children: 2 },
      ]),
    ),
    ...over,
  };
  const reload = vi.fn(async () => {});
  const onclose = vi.fn();
  const ondone = vi.fn();
  const view = renderWithCore(
    SchemaDefDialog,
    {
      store: "shared",
      target,
      name,
      schema,
      ulidOf: (id: string) => (id === "WEB-2" ? "U2" : undefined),
      onclose,
      ondone,
    },
    { api, schema: { reload, subscribe: () => () => {} } },
  );
  return { api, reload, onclose, ondone, ...view };
}

const type = (label: string, v: string) =>
  fireEvent.input(screen.getByRole("textbox", { name: label }), { target: { value: v } });

describe("SchemaDefDialog", () => {
  beforeEach(() => resetToasts());
  afterEach(cleanup);

  it("adds a status with a palette color", async () => {
    const { api, reload, onclose, ondone } = setup("status");
    expect(screen.getByRole("dialog", { name: "ステータスを追加" })).toBeVisible();
    await type("名前", "review");
    await type("表示名", "レビュー");
    await fireEvent.change(screen.getByRole("combobox", { name: "カテゴリ" }), {
      target: { value: "doing" },
    });
    await fireEvent.click(screen.getByRole("radio", { name: "ティール" }));
    await fireEvent.click(screen.getByRole("button", { name: "追加" }));
    await waitFor(() => expect(api.addStatus).toHaveBeenCalledTimes(1));
    expect(api.addStatus).toHaveBeenCalledWith(
      { name: "review", label: "レビュー", category: "doing", color: "teal" },
      { store: "shared" },
    );
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(reload).toHaveBeenCalledWith("shared");
    expect(ondone).toHaveBeenCalled();
    expect(toasts.list.map((t) => t.message)).toContain("ステータス「review」を追加しました");
  });

  it("sends only what changed", async () => {
    const { api } = setup("status", "in_progress");
    expect(screen.getByRole("dialog", { name: "ステータス「in_progress」を編集" })).toBeVisible();
    expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
    await type("表示名", "新");
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(api.changeStatus).toHaveBeenCalledTimes(1));
    expect(api.changeStatus).toHaveBeenCalledWith(
      "in_progress",
      { label: "新" },
      { store: "shared" },
    );
    expect(toasts.list.map((t) => t.message)).toContain("ステータス「in_progress」を保存しました");
  });

  it("keeps a custom color unless another is picked", async () => {
    const { api } = setup("status", "in_progress");
    expect(screen.getByRole("radio", { name: "カスタム（#3451D1）" })).toBeChecked();
    await type("表示名", "新");
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(api.changeStatus).toHaveBeenCalled());
    const sent = (api.changeStatus.mock.calls as unknown[][])[0]?.[1];
    expect(sent).not.toHaveProperty("color");
  });

  it("does not let the category of a default status change", () => {
    setup("status", "todo");
    expect(screen.getByRole("combobox", { name: "カテゴリ" })).toBeDisabled();
    expect(
      screen.getByText(
        "既定のステータスはカテゴリを変えられません。先に別のステータスを既定にしてください",
      ),
    ).toBeVisible();
    expect(screen.getByRole("checkbox", { name: "既定" })).toBeDisabled();
    expect(screen.getByText("既定を外すには、別のステータスを既定にしてください")).toBeVisible();
  });

  it("refuses a kind with no parent chosen", async () => {
    setup("kind");
    await type("名前", "spike");
    await fireEvent.click(screen.getByRole("radio", { name: "指定する" }));
    expect(screen.getByRole("button", { name: "追加" })).toBeDisabled();
    expect(screen.getByText("置ける親を 1 つ以上選んでください")).toBeVisible();
    await fireEvent.click(screen.getByRole("checkbox", { name: "ルート" }));
    expect(screen.getByRole("button", { name: "追加" })).toBeEnabled();
  });

  it("adds a kind that can sit anywhere with anywhere, never an empty parents", async () => {
    const { api } = setup("kind");
    await type("名前", "spike");
    await fireEvent.click(screen.getByRole("button", { name: "追加" }));
    await waitFor(() => expect(api.addKind).toHaveBeenCalled());
    expect(api.addKind).toHaveBeenCalledWith(
      { name: "spike", role: "work", anywhere: true },
      { store: "shared" },
    );
  });

  it("lists the tasks a change would break, each with a link to the task", async () => {
    const { onclose } = setup("kind", "sub", {
      changeKind: vi.fn(() =>
        fail(422, "schema.tasks_invalid", { violations: ["WEB-2 バグ: parent: 親が合いません"] }),
      ),
    });
    await fireEvent.change(screen.getByRole("combobox", { name: "役割" }), {
      target: { value: "container" },
    });
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    const link = await screen.findByRole("link", { name: "WEB-2" });
    expect(link.getAttribute("href")).toContain("U2");
    expect(screen.getByText(/バグ: parent: 親が合いません/)).toBeVisible();
    expect(onclose).not.toHaveBeenCalled();
  });

  it("offers the templates of the Store and keeps the one the kind has", async () => {
    const { api } = setup("kind", "sub");
    const select = (await screen.findByRole("combobox", {
      name: "既定のテンプレート",
    })) as HTMLSelectElement;
    await waitFor(() =>
      expect([...select.options].map((o) => o.value)).toEqual(["", "bug", "feature"]),
    );
    expect(select.value).toBe("bug");
    await fireEvent.change(select, { target: { value: "" } });
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(api.changeKind).toHaveBeenCalled());
    expect(api.changeKind).toHaveBeenCalledWith("sub", { template: "" }, { store: "shared" });
  });

  it("does not let the role of the default kind change", () => {
    setup("kind", "task");
    expect(screen.getByRole("combobox", { name: "役割" })).toBeDisabled();
  });

  it("adds an enum with a value per line, with no task_ref among the types", async () => {
    const { api } = setup("prop");
    const types = screen.getByRole("combobox", { name: "型" }) as HTMLSelectElement;
    expect([...types.options].map((o) => o.value)).not.toContain("task_ref");
    await type("名前", "severity");
    await fireEvent.change(types, { target: { value: "enum" } });
    await fireEvent.input(screen.getByRole("textbox", { name: "値（1 行に 1 つ）" }), {
      target: { value: "low\n\n high \nlow" },
    });
    await fireEvent.click(screen.getByRole("button", { name: "追加" }));
    await waitFor(() => expect(api.addProp).toHaveBeenCalled());
    expect(api.addProp).toHaveBeenCalledWith(
      { name: "severity", type: "enum", values: ["low", "high"] },
      { store: "shared" },
    );
  });

  it("adds a relation as task_ref without a type field, and turns the inverse off for two-way", async () => {
    const { api } = setup("relation");
    expect(screen.queryByRole("combobox", { name: "型" })).toBeNull();
    await type("名前", "causes");
    await type("逆方向の表示名", "caused_by");
    await fireEvent.click(screen.getByRole("checkbox", { name: "双方向" }));
    expect(screen.getByRole("textbox", { name: "逆方向の表示名" })).toBeDisabled();
    await fireEvent.click(screen.getByRole("checkbox", { name: "複数" }));
    await fireEvent.click(screen.getByRole("button", { name: "追加" }));
    await waitFor(() => expect(api.addProp).toHaveBeenCalled());
    expect(api.addProp).toHaveBeenCalledWith(
      { name: "causes", type: "task_ref", multi: true, symmetric: true },
      { store: "shared" },
    );
  });

  it("edits a relation by the fields that changed", async () => {
    const { api } = setup("relation", "duplicates");
    expect(screen.getByRole("dialog", { name: "関連「duplicates」を編集" })).toBeVisible();
    await fireEvent.click(screen.getByRole("checkbox", { name: "複数" }));
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(api.changeProp).toHaveBeenCalled());
    expect(api.changeProp).toHaveBeenCalledWith("duplicates", { multi: true }, { store: "shared" });
  });

  describe("a kind the dialog does not touch", () => {
    const odd = {
      ...sch,
      kinds: [
        ...(sch.kinds ?? []),
        // a custom icon, and a parent that is no kind of the schema
        {
          name: "odd",
          label: "珍",
          role: "work",
          parents: ["task", "ghost"],
          icon: "star",
        },
      ],
    } as unknown as SchemaView;

    it("keeps a custom icon unless another is picked", async () => {
      const { api } = setup("kind", "odd", {}, odd);
      expect(screen.getByRole("radio", { name: "カスタム（star）" })).toBeChecked();
      await type("表示名", "新");
      await fireEvent.click(screen.getByRole("button", { name: "保存" }));
      await waitFor(() => expect(api.changeKind).toHaveBeenCalledTimes(1));
      const sent = (api.changeKind.mock.calls as unknown[][])[0]?.[1];
      expect(sent).toEqual({ label: "新" });
      expect(sent).not.toHaveProperty("icon");
    });

    it("sends no parents while the parents are as they were, a parent that is gone included", async () => {
      const { api } = setup("kind", "odd", {}, odd);
      expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
      await type("表示名", "新");
      await fireEvent.click(screen.getByRole("button", { name: "保存" }));
      await waitFor(() => expect(api.changeKind).toHaveBeenCalledTimes(1));
      const sent = (api.changeKind.mock.calls as unknown[][])[0]?.[1];
      expect(sent).not.toHaveProperty("parents");
      expect(sent).not.toHaveProperty("anywhere");
    });
  });
});
