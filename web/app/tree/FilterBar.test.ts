import { fireEvent, render } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import FilterBar from "./FilterBar.svelte";

describe("FilterBar", () => {
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
