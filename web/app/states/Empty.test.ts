import { cleanup, fireEvent } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithCore } from "../test-utils";
import Empty from "./Empty.svelte";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("Empty", () => {
  it("noworkspace: 案内と、Store ごとの hh workspace add をコピーできる形で出す", () => {
    const { getByText, getAllByRole } = renderWithCore(Empty, {
      variant: "noworkspace",
      commands: [
        "hh workspace add <名前> --prefix <接頭辞> --store a",
        "hh workspace add <名前> --prefix <接頭辞> --store b",
      ],
    });
    expect(getByText("ワークスペースがありません")).toBeTruthy();
    expect(getAllByRole("button", { name: /hh workspace add/ })).toHaveLength(2);
  });

  it("notfound: ワークスペースの名前と、ホームへ戻るボタン", async () => {
    const onHome = vi.fn();
    const { getByText, getByRole } = renderWithCore(Empty, {
      variant: "notfound",
      scope: "workspace",
      name: "01HW",
      onHome,
    });
    expect(getByText("ワークスペース「01HW」が見つかりません")).toBeTruthy();
    await fireEvent.click(getByRole("button", { name: "ホームへ戻る" }));
    expect(onHome).toHaveBeenCalledTimes(1);
  });

  it("notfound: Store の名前を出す", () => {
    const { getByText } = renderWithCore(Empty, {
      variant: "notfound",
      scope: "store",
      name: "nope",
    });
    expect(getByText("Store「nope」が見つかりません")).toBeTruthy();
  });

  it("notfound: ビューの名前を出す", () => {
    const { getByText } = renderWithCore(Empty, {
      variant: "notfound",
      scope: "view",
      name: "今週のリリース作業",
    });
    expect(getByText("ビュー「今週のリリース作業」が見つかりません")).toBeTruthy();
  });

  it("notfound: プロジェクトは名前を出さない", () => {
    const { getByText } = renderWithCore(Empty, { variant: "notfound", scope: "project" });
    expect(getByText("プロジェクトが見つかりません")).toBeTruthy();
  });

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
