import { fireEvent, render } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import FilterBar from "./FilterBar.svelte";

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
});
