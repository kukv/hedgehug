import { cleanup, fireEvent, screen, waitFor } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { SchemaView } from "../../core/schema";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import SettingsTemplates from "./SettingsTemplates.svelte";

afterEach(cleanup);

const sch = (def?: Record<string, string>) =>
  ({
    statuses: [],
    props: {},
    templates: def ? { default: def } : undefined,
  }) as unknown as SchemaView;

function setup(
  over: { def?: Record<string, string>; off?: string | null; set?: () => unknown } = {},
) {
  const api = {
    listTemplates: vi.fn(() =>
      ok([
        { name: "bug", children: 0 },
        { name: "feature", children: 2 },
      ]),
    ),
    setTemplateDefaults: vi.fn(over.set ?? (() => ok({ schema: sch(), commit: "c" }))),
  };
  const reload = vi.fn(async () => {});
  renderWithCore(
    SettingsTemplates,
    { store: "shared", off: over.off ?? null },
    {
      api,
      schema: {
        get: () => sch(over.def),
        load: async () => sch(over.def),
        reload,
        subscribe: () => () => {},
      },
    },
  );
  return { api, reload };
}

describe("SettingsTemplates: 種類ごとの既定", () => {
  it("種類ごとに「<種類> の既定」の欄があり、今の値が選ばれている", async () => {
    setup({ def: { bug: "bug" } });
    const bug = (await screen.findByRole("combobox", { name: "Bug の既定" })) as HTMLSelectElement;
    await waitFor(() =>
      expect([...bug.options].map((o) => o.value)).toEqual(["", "bug", "feature"]),
    );
    expect(bug.value).toBe("bug");
    expect(screen.getByRole("combobox", { name: "Task の既定" })).toHaveValue("");
  });

  it("選ぶと、ほかの種類の既定を残したまま保存して読み直す", async () => {
    const { api, reload } = setup({ def: { bug: "bug" } });
    const task = await screen.findByRole("combobox", { name: "Task の既定" });
    await waitFor(() => expect(task.querySelectorAll("option")).toHaveLength(3));
    await fireEvent.change(task, { target: { value: "feature" } });
    await waitFor(() =>
      expect(api.setTemplateDefaults).toHaveBeenCalledWith(
        { default: { bug: "bug", task: "feature" } },
        { store: "shared" },
      ),
    );
    await waitFor(() => expect(reload).toHaveBeenCalledWith("shared"));
  });

  it("「なし」を選ぶとその種類だけ外す", async () => {
    const { api } = setup({ def: { bug: "bug", task: "feature" } });
    const bug = await screen.findByRole("combobox", { name: "Bug の既定" });
    await waitFor(() => expect(bug.querySelectorAll("option")).toHaveLength(3));
    await fireEvent.change(bug, { target: { value: "" } });
    await waitFor(() =>
      expect(api.setTemplateDefaults).toHaveBeenCalledWith(
        { default: { task: "feature" } },
        { store: "shared" },
      ),
    );
  });

  it("書き込めない Store では選べない", async () => {
    setup({ off: "r/o" });
    expect(await screen.findByRole("combobox", { name: "Bug の既定" })).toBeDisabled();
  });

  it("保存に失敗したら文で出す", async () => {
    setup({
      set: () => fail(422, "schema.invalid", { problems: ["templates.default: bad"] }),
    });
    const task = await screen.findByRole("combobox", { name: "Task の既定" });
    await waitFor(() => expect(task.querySelectorAll("option")).toHaveLength(3));
    await fireEvent.change(task, { target: { value: "bug" } });
    expect(await screen.findByText(/スキーマの規則に合わなくなります/)).toBeVisible();
  });
});
