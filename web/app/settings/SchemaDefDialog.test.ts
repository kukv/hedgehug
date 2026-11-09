import { cleanup, fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SchemaView } from "../../core/schema";
import type { SchemaUsage } from "../../core/schema-usage";
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
  props: {
    tier: { type: "enum", values: ["low", "high"] },
    memo: { type: "text", label: "メモ" },
  },
  prop_order: ["tier", "memo"],
  workflow: {},
  scoped: [],
} as unknown as SchemaView;

type Target = "status" | "prop";

function setup(
  target: Target,
  name?: string,
  over: Record<string, unknown> = {},
  schema: SchemaView = sch,
  usage: SchemaUsage | null = null,
) {
  const w = () => ok({ schema: sch, commit: "c1" });
  const api = {
    addStatus: vi.fn(w),
    changeStatus: vi.fn(w),
    addProp: vi.fn(w),
    changeProp: vi.fn(w),
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
      usage,
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
    expect(screen.getByRole("radio", { name: "自由な色" })).toBeChecked();
    expect((screen.getByLabelText("色を選ぶ") as HTMLInputElement).value).toBe("#3451d1");
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

  it("lists the tasks a change would break, each with a link to the task", async () => {
    const { onclose } = setup("status", "todo", {
      changeStatus: vi.fn(() =>
        fail(422, "schema.tasks_invalid", { violations: ["WEB-2 バグ: parent: 親が合いません"] }),
      ),
    });
    await type("表示名", "新");
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    const link = await screen.findByRole("link", { name: "WEB-2" });
    expect(link.getAttribute("href")).toContain("U2");
    expect(screen.getByText(/バグ: parent: 親が合いません/)).toBeVisible();
    expect(onclose).not.toHaveBeenCalled();
  });

  it("adds an enum with a row per value, with no task_ref among the types", async () => {
    const { api } = setup("prop");
    const types = screen.getByRole("combobox", { name: "型" }) as HTMLSelectElement;
    expect([...types.options].map((o) => o.value)).not.toContain("task_ref");
    await type("名前", "severity");
    await fireEvent.change(types, { target: { value: "enum" } });
    await fireEvent.click(screen.getByRole("button", { name: "値を追加" }));
    await fireEvent.click(screen.getByRole("button", { name: "値を追加" }));
    const rows = screen.getAllByRole("textbox", { name: "値" });
    await fireEvent.input(rows[0] as HTMLElement, { target: { value: "low" } });
    await fireEvent.input(rows[1] as HTMLElement, { target: { value: " high " } });
    await fireEvent.click(screen.getByRole("button", { name: "追加" }));
    await waitFor(() => expect(api.addProp).toHaveBeenCalled());
    expect(api.addProp).toHaveBeenCalledWith(
      { name: "severity", type: "enum", values: ["low", "high"] },
      { store: "shared" },
    );
  });

  it("offers no task_ref type, and no flags of a relation", async () => {
    setup("prop");
    const types = Array.from(
      (screen.getByRole("combobox", { name: "型" }) as HTMLSelectElement).options,
    ).map((o) => o.value);
    expect(types).not.toContain("task_ref");
    expect(screen.queryByRole("checkbox", { name: "双方向" })).toBeNull();
    expect(screen.queryByRole("textbox", { name: "逆方向の表示名" })).toBeNull();
  });

  it("changes the name in the edit dialog and sends it as name", async () => {
    const { api } = setup("status", "in_progress");
    expect((screen.getByRole("textbox", { name: "名前" }) as HTMLInputElement).value).toBe(
      "in_progress",
    );
    expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
    await type("名前", "doing");
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(api.changeStatus).toHaveBeenCalledTimes(1));
    expect(api.changeStatus).toHaveBeenCalledWith(
      "in_progress",
      { name: "doing" },
      { store: "shared" },
    );
    expect(toasts.list.map((t) => t.message)).toContain("ステータス「doing」を保存しました");
  });

  it("sends the name with the other changes in one write", async () => {
    const { api } = setup("prop", "memo");
    await type("名前", "note");
    await type("表示名", "ノート");
    await fireEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(api.changeProp).toHaveBeenCalledTimes(1));
    expect(api.changeProp).toHaveBeenCalledWith(
      "memo",
      { name: "note", label: "ノート" },
      { store: "shared" },
    );
  });

  it("does not save an empty name", async () => {
    setup("status", "todo");
    await type("名前", " ");
    expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
  });

  describe("the values of an enum", () => {
    const usage = {
      statuses: {},
      props: {},
      values: { tier: { low: 0, high: 2 } },
    } as unknown as SchemaUsage;
    const rows = () => screen.getAllByRole("textbox", { name: "値" }) as HTMLInputElement[];
    const save = async (api: { changeProp: ReturnType<typeof vi.fn> }) => {
      await fireEvent.click(screen.getByRole("button", { name: "保存" }));
      await waitFor(() => expect(api.changeProp).toHaveBeenCalledTimes(1));
      return (api.changeProp.mock.calls as unknown[][])[0]?.[1];
    };

    it("shows a row per value, and nothing to save until one changes", () => {
      setup("prop", "tier", {}, sch, usage);
      expect(rows().map((r) => r.value)).toEqual(["low", "high"]);
      expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
    });

    it("adds a value (no from)", async () => {
      const { api } = setup("prop", "tier", {}, sch, usage);
      await fireEvent.click(screen.getByRole("button", { name: "値を追加" }));
      await fireEvent.input(rows()[2] as HTMLElement, { target: { value: "mid" } });
      expect(await save(api)).toEqual({
        values: [{ name: "low", from: ["low"] }, { name: "high", from: ["high"] }, { name: "mid" }],
      });
    });

    it("renames a value by its text (one from, another name)", async () => {
      const { api } = setup("prop", "tier", {}, sch, usage);
      await fireEvent.input(rows()[0] as HTMLElement, { target: { value: "minor" } });
      expect(await save(api)).toEqual({
        values: [
          { name: "minor", from: ["low"] },
          { name: "high", from: ["high"] },
        ],
      });
    });

    it("deletes an unused value, and disables the delete of a used one with the reason", async () => {
      const { api } = setup("prop", "tier", {}, sch, usage);
      const used = screen.getByRole("button", { name: "削除（値 high）" });
      expect(used).toBeDisabled();
      expect(used.getAttribute("title")).toBe(
        "タスクが使っているため削除できません。マージで別の値へ移してください",
      );
      await fireEvent.click(screen.getByRole("button", { name: "削除（値 low）" }));
      expect(rows().map((r) => r.value)).toEqual(["high"]);
      expect(await save(api)).toEqual({ values: [{ name: "high", from: ["high"] }] });
    });

    it("merges a value into another in the dialog: the row goes, its from joins the target", async () => {
      const { api } = setup("prop", "tier", {}, sch, usage);
      await fireEvent.click(screen.getByRole("button", { name: "マージ（値 high）" }));
      const to = screen.getByRole("combobox", { name: "マージ先（値 high）" }) as HTMLSelectElement;
      const low = [...to.options].find((o) => o.textContent === "low");
      await fireEvent.change(to, { target: { value: low?.value } });
      expect(rows().map((r) => r.value)).toEqual(["low"]);
      expect(await save(api)).toEqual({ values: [{ name: "low", from: ["low", "high"] }] });
    });

    it("moves a value with the keys of the handle", async () => {
      const { api } = setup("prop", "tier", {}, sch, usage);
      await fireEvent.keyDown(screen.getByRole("button", { name: "並べ替え（値 low）" }), {
        key: "ArrowDown",
        altKey: true,
      });
      expect(rows().map((r) => r.value)).toEqual(["high", "low"]);
      expect(await save(api)).toEqual({
        values: [
          { name: "high", from: ["high"] },
          { name: "low", from: ["low"] },
        ],
      });
    });

    it("merges into a value that was renamed: the target keeps its from and takes the other", async () => {
      const { api } = setup("prop", "tier", {}, sch, usage);
      await fireEvent.input(rows()[0] as HTMLElement, { target: { value: "minor" } });
      await fireEvent.click(screen.getByRole("button", { name: "マージ（値 high）" }));
      const to = screen.getByRole("combobox", { name: "マージ先（値 high）" }) as HTMLSelectElement;
      const minor = [...to.options].find((o) => o.textContent === "minor");
      await fireEvent.change(to, { target: { value: minor?.value } });
      expect(await save(api)).toEqual({ values: [{ name: "minor", from: ["low", "high"] }] });
    });

    it("deletes a value that was renamed: its from is left out, so it goes", async () => {
      const { api } = setup("prop", "tier", {}, sch, usage);
      await fireEvent.input(rows()[0] as HTMLElement, { target: { value: "minor" } });
      await fireEvent.click(screen.getByRole("button", { name: "削除（値 minor）" }));
      expect(await save(api)).toEqual({ values: [{ name: "high", from: ["high"] }] });
    });

    it("leaves an empty new row out of the merge targets", async () => {
      setup("prop", "tier", {}, sch, usage);
      await fireEvent.click(screen.getByRole("button", { name: "値を追加" }));
      await fireEvent.click(screen.getByRole("button", { name: "マージ（値 high）" }));
      const to = screen.getByRole("combobox", { name: "マージ先（値 high）" }) as HTMLSelectElement;
      expect([...to.options].map((o) => o.textContent)).toEqual(["選んでください", "low"]);
    });

    it("focuses the select after マージ, and closes it with Esc without closing the dialog", async () => {
      const { onclose } = setup("prop", "tier", {}, sch, usage);
      await fireEvent.click(screen.getByRole("button", { name: "マージ（値 high）" }));
      const to = await screen.findByRole("combobox", { name: "マージ先（値 high）" });
      await waitFor(() => expect(to).toHaveFocus());
      await fireEvent.keyDown(to, { key: "Escape" });
      expect(screen.queryByRole("combobox", { name: "マージ先（値 high）" })).toBeNull();
      expect(onclose).not.toHaveBeenCalled();
      await waitFor(() =>
        expect(screen.getByRole("button", { name: "マージ（値 high）" })).toHaveFocus(),
      );
    });

    it("closes the select when it loses the focus", async () => {
      setup("prop", "tier", {}, sch, usage);
      await fireEvent.click(screen.getByRole("button", { name: "マージ（値 high）" }));
      const to = await screen.findByRole("combobox", { name: "マージ先（値 high）" });
      await fireEvent.blur(to);
      expect(screen.queryByRole("combobox", { name: "マージ先（値 high）" })).toBeNull();
    });

    it("moves the focus to the next row after a delete, the previous one at the end, then to 値を追加", async () => {
      const free = {
        statuses: {},
        props: {},
        values: { tier: { low: 0, high: 0 } },
      } as unknown as SchemaUsage;
      setup("prop", "tier", {}, sch, free);
      await fireEvent.click(screen.getByRole("button", { name: "削除（値 low）" }));
      await waitFor(() => expect(rows()[0]).toHaveFocus());
      expect(rows()[0]?.value).toBe("high");
      await fireEvent.click(screen.getByRole("button", { name: "削除（値 high）" }));
      await waitFor(() => expect(screen.getByRole("button", { name: "値を追加" })).toHaveFocus());
    });

    it("moves the focus to the row before when the last row is deleted", async () => {
      const free = {
        statuses: {},
        props: {},
        values: { tier: { low: 0, high: 0 } },
      } as unknown as SchemaUsage;
      setup("prop", "tier", {}, sch, free);
      await fireEvent.click(screen.getByRole("button", { name: "削除（値 high）" }));
      await waitFor(() => expect(rows()[0]).toHaveFocus());
    });

    it("focuses the next row after a merge, and the new row after 値を追加", async () => {
      setup("prop", "tier", {}, sch, usage);
      await fireEvent.click(screen.getByRole("button", { name: "マージ（値 low）" }));
      const to = screen.getByRole("combobox", { name: "マージ先（値 low）" }) as HTMLSelectElement;
      const high = [...to.options].find((o) => o.textContent === "high");
      await fireEvent.change(to, { target: { value: high?.value } });
      await waitFor(() => expect(rows()[0]).toHaveFocus());
      await fireEvent.click(screen.getByRole("button", { name: "値を追加" }));
      await waitFor(() => expect(rows()[1]).toHaveFocus());
    });

    it("does not save the dialog on Enter in a value field", async () => {
      const { api } = setup("prop", "tier", {}, sch, usage);
      await fireEvent.input(rows()[0] as HTMLElement, { target: { value: "minor" } });
      await fireEvent.keyDown(rows()[0] as HTMLElement, { key: "Enter" });
      expect(api.changeProp).not.toHaveBeenCalled();
    });

    it("refuses an empty or repeated value", async () => {
      setup("prop", "tier", {}, sch, usage);
      await fireEvent.click(screen.getByRole("button", { name: "値を追加" }));
      expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
      await fireEvent.input(rows()[2] as HTMLElement, { target: { value: "low" } });
      expect(screen.getByRole("button", { name: "保存" })).toBeDisabled();
      expect(screen.getByText("値が空か、重複しています")).toBeVisible();
    });
  });
});
