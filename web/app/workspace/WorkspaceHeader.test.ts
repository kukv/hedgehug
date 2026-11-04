import { cleanup, within } from "@testing-library/svelte";
import { createRawSnippet } from "svelte";
import { afterEach, describe, expect, it } from "vitest";
import type { WorkspaceEntry } from "../../core/stores";
import { renderWithCore } from "../test-utils";
import WorkspaceHeader from "./WorkspaceHeader.svelte";

afterEach(cleanup);

const entry: WorkspaceEntry = {
  store: "shared",
  tone: "purple",
  ulid: "W1",
  name: "release",
  prefix: "REL",
  tasks: 24,
  deleted: 1,
  prefixSet: "2026-10-29T12:00:00Z",
  projects: [],
};

describe("WorkspaceHeader", () => {
  it("Store のバッジ・名前・接頭辞・見出しを出す", () => {
    const { getByRole, getByText } = renderWithCore(WorkspaceHeader, { entry, tab: "overview" });
    expect(getByRole("heading", { level: 1, name: "release" })).toBeTruthy();
    expect(getByText("REL")).toBeTruthy();
    expect(getByText("ワークスペース")).toBeTruthy();
    expect(getByText("shared")).toBeTruthy();
  });

  it("タブは概要・ツリー・カンバン・テーブルと右端の設定。今のタブだけ aria-current", () => {
    const { getByRole } = renderWithCore(WorkspaceHeader, { entry, tab: "tree" });
    const nav = getByRole("navigation", { name: "ビューの切り替え" });
    const links = within(nav).getAllByRole("link");
    expect(links.map((a) => a.textContent?.trim())).toEqual([
      "概要",
      "ツリー",
      "カンバン",
      "テーブル",
      "設定",
    ]);
    expect(links.map((a) => a.getAttribute("href"))).toEqual([
      "/workspaces/shared/W1",
      "/workspaces/shared/W1/tree",
      "/workspaces/shared/W1/kanban",
      "/workspaces/shared/W1/table",
      "/workspaces/shared/W1/settings",
    ]);
    expect(links.map((a) => a.getAttribute("aria-current"))).toEqual([
      null,
      "page",
      null,
      null,
      null,
    ]);
  });

  it.each([
    ["kanban", "カンバン"],
    ["table", "テーブル"],
  ] as const)("%s のタブのとき「%s」が今のタブ", (tab, label) => {
    const { getByRole } = renderWithCore(WorkspaceHeader, { entry, tab });
    expect(getByRole("link", { name: label }).getAttribute("aria-current")).toBe("page");
  });

  it("ガント・バックログなど後のフェーズのタブは出さない", () => {
    const { queryByRole } = renderWithCore(WorkspaceHeader, { entry, tab: "overview" });
    for (const name of ["ガント", "バックログ", "スプリント", "リリース", "レポート"]) {
      expect(queryByRole("link", { name })).toBeNull();
    }
  });

  it("設定のタブのとき設定が今のタブ", () => {
    const { getByRole } = renderWithCore(WorkspaceHeader, { entry, tab: "settings" });
    expect(getByRole("link", { name: "設定" }).getAttribute("aria-current")).toBe("page");
  });

  it("actions を見出しの右に出す", () => {
    const actions = createRawSnippet(() => ({ render: () => "<button>タスク</button>" }));
    const { getByRole } = renderWithCore(WorkspaceHeader, { entry, tab: "overview", actions });
    expect(getByRole("button", { name: "タスク" })).toBeTruthy();
  });
});
