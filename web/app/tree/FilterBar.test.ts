import { fireEvent, render } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "../../core/api/client";
import { renderWithCore } from "../test-utils";
import FilterBar from "./FilterBar.svelte";

const refused = new ApiError({
  status: 400,
  type: "query.invalid",
  detail: 'term 1 (status:in_progres): unknown status or category "in_progres"',
});
const schema = { statuses: [{ name: "in_progress", category: "doing" }], kinds: [], props: {} };
// The core of a screen that has read the schema of its one store.
const core = {
  stores: { stores: () => [{ name: "shared" }], subscribe: () => () => {} },
  schema: { get: () => schema, load: async () => schema, subscribe: () => () => {} },
};
// Lets the schemas be read, so that a test that expects no suggestion has waited for one.
const settle = () => new Promise((resolve) => setTimeout(resolve, 0));
const props = (over: Record<string, unknown> = {}) => ({
  base: "",
  applied: "status:in_progres",
  count: "0 件",
  cli: "hh ls",
  error: refused,
  onchange: vi.fn(),
  ...over,
});

describe("FilterBar", () => {
  it("shows the fixed part of the query, and nothing for none", () => {
    const props = { applied: "", count: "0 件", cli: "hh ls", error: null, onchange: vi.fn() };
    const a = render(FilterBar, { ...props, base: "project:REL-15" });
    expect(a.getByTitle("この画面で固定のクエリ").textContent).toBe("project:REL-15");
    a.unmount();
    const b = render(FilterBar, { ...props, base: "" });
    expect(b.queryByTitle("この画面で固定のクエリ")).toBeNull();
  });

  it("does not apply the typed query after it is removed", async () => {
    vi.useFakeTimers();
    try {
      const onchange = vi.fn();
      const { getByLabelText, unmount } = render(FilterBar, {
        base: "project:REL-15",
        applied: "",
        count: "0 件",
        cli: "hh ls",
        error: null,
        onchange,
      });
      await fireEvent.input(getByLabelText("フィルタ（hh ls と同じクエリ）"), {
        target: { value: "assignee:@me" },
      });
      unmount();
      vi.advanceTimersByTime(1000);
      expect(onchange).not.toHaveBeenCalled();
    } finally {
      vi.useRealTimers();
    }
  });

  it("offers the name that is a letter off, and applies it when pressed", async () => {
    const p = props();
    const { getByRole, findByRole, getByText } = renderWithCore(FilterBar, p, core);
    expect(getByRole("alert")).toHaveTextContent("クエリが正しくありません");
    // the schemas are read first
    const fix = await findByRole("button", { name: "status:in_progress" });
    expect(getByText(/のことですか？/)).toBeTruthy();
    await fireEvent.click(fix);
    expect(p.onchange).toHaveBeenCalledWith("status:in_progress");
  });

  it("keeps the other terms of the query it fixes", async () => {
    const p = props({ applied: "assignee:@me status:in_progres" });
    const { findByRole } = renderWithCore(FilterBar, p, core);
    await fireEvent.click(await findByRole("button", { name: "status:in_progress" }));
    expect(p.onchange).toHaveBeenCalledWith("assignee:@me status:in_progress");
  });

  it("offers nothing when the wrong term is in the part the screen fixes", async () => {
    const { queryByRole } = renderWithCore(
      FilterBar,
      props({ base: "workspace:demo", applied: "" }),
      core,
    );
    await settle();
    expect(queryByRole("button", { name: "status:in_progress" })).toBeNull();
  });

  it("offers nothing without a core, and still tells the error", async () => {
    const { getByRole, queryByRole } = render(FilterBar, props());
    await settle();
    expect(getByRole("alert")).toBeTruthy();
    expect(queryByRole("button", { name: "status:in_progress" })).toBeNull();
  });

  it("offers nothing when there is no error", async () => {
    const { queryByRole } = renderWithCore(FilterBar, props({ error: null }), core);
    await settle();
    expect(queryByRole("button", { name: "status:in_progress" })).toBeNull();
  });
});
