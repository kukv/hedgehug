import { fireEvent, render } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import { describe, expect, it, vi } from "vitest";
import ListBar from "./ListBar.svelte";

const text = (s: string) => createRawSnippet(() => ({ render: () => `<button>${s}</button>` }));

const props = () => ({
  base: "workspace:release",
  applied: "-status:canceled",
  count: "16 件",
  cli: "hh ls workspace:release -- -status:canceled",
  error: null,
  onchange: vi.fn(),
});

describe("ListBar", () => {
  it("shows the filter: the fixed part, the query applied, the count and the hh line", () => {
    const { getByLabelText, getByText, getByTitle } = render(ListBar, props());
    expect(getByTitle("この画面で固定のクエリ").textContent).toBe("workspace:release");
    expect((getByLabelText("フィルタ（hh ls と同じクエリ）") as HTMLInputElement).value).toBe(
      "-status:canceled",
    );
    expect(getByText("16 件")).toBeTruthy();
    expect(getByText("$ hh ls workspace:release -- -status:canceled")).toBeTruthy();
  });

  it("tells the query typed with Enter", async () => {
    const p = props();
    const { getByLabelText } = render(ListBar, p);
    const input = getByLabelText("フィルタ（hh ls と同じクエリ）");
    await fireEvent.input(input, { target: { value: " assignee:@me " } });
    await fireEvent.keyDown(input, { key: "Enter" });
    expect(p.onchange).toHaveBeenCalledWith("assignee:@me");
  });

  it("shows the error of the query", () => {
    const { getByRole } = render(ListBar, {
      ...props(),
      error: new Error("bad query"),
    });
    expect(getByRole("alert")).toBeTruthy();
  });

  it("puts the controls and the actions at the right, when given", () => {
    const none = render(ListBar, props());
    expect(none.queryByText("列: ステータス")).toBeNull();
    none.unmount();
    const { getByText } = render(ListBar, {
      ...props(),
      controls: text("列: ステータス"),
      actions: text("ビューを保存"),
    });
    expect(getByText("列: ステータス")).toBeTruthy();
    expect(getByText("ビューを保存")).toBeTruthy();
  });

  it("has no fixed part for every store", () => {
    const { queryByTitle } = render(ListBar, { ...props(), base: "" });
    expect(queryByTitle("この画面で固定のクエリ")).toBeNull();
  });
});
