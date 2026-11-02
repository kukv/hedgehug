import { cleanup, fireEvent } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithCore } from "../test-utils";
import Empty from "./Empty.svelte";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("Empty", () => {
  it("empty: プロジェクトの名前とタスクを作成のボタン（C）", async () => {
    const onCreate = vi.fn();
    const { getByText, getByRole } = renderWithCore(Empty, {
      variant: "empty",
      scope: "project",
      name: "Web フロント刷新",
      onCreate,
    });
    expect(getByText("Web フロント刷新 にはまだタスクがありません")).toBeTruthy();
    await fireEvent.click(getByRole("button", { name: /タスクを作成/ }));
    expect(onCreate).toHaveBeenCalledTimes(1);
  });

  it("empty: onCreate がなければボタンを出さない", () => {
    const { queryByRole } = renderWithCore(Empty, {
      variant: "empty",
      scope: "workspace",
      name: "release",
    });
    expect(queryByRole("button")).toBeNull();
  });

  it("nomatch: 一致しないことと、note、フィルタを解除", async () => {
    const onClear = vi.fn();
    const { getByText, getByRole } = renderWithCore(Empty, {
      variant: "nomatch",
      note: "完了と中止を除外しています",
      onClear,
    });
    expect(getByText("一致するタスクはありません")).toBeTruthy();
    expect(getByText("完了と中止を除外しています")).toBeTruthy();
    await fireEvent.click(getByRole("button", { name: "フィルタを解除" }));
    expect(onClear).toHaveBeenCalledTimes(1);
  });
});
