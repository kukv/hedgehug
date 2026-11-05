import { fireEvent, screen, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DEFAULT_GANTT, DEFAULT_KANBAN, DEFAULT_TABLE } from "../../core/views";
import type { Layout } from "../router";
import { closeViewDialog, viewDialog } from "./dialogs-state.svelte";
import QueryPage from "./QueryPage.svelte";
import { mountWithViews } from "./testkit";

vi.mock("../tree/TreeView.svelte", async () => ({
  default: (await import("./fakes/FakeTree.svelte")).default,
}));
vi.mock("../kanban/KanbanView.svelte", async () => ({
  default: (await import("./fakes/FakeKanban.svelte")).default,
}));
vi.mock("../table/TableView.svelte", async () => ({
  default: (await import("./fakes/FakeTable.svelte")).default,
}));

const here = () => location.pathname + location.search;

function setup(layout: Layout = "table", q = "assignee:@me") {
  history.replaceState(null, "", `/query/${layout}?q=${encodeURIComponent(q)}`);
  return mountWithViews(QueryPage, { layout, q });
}

const tab = (name: string) =>
  within(screen.getByRole("navigation", { name: "レイアウトの切り替え" })).getByRole("link", {
    name,
  });

describe("QueryPage", () => {
  beforeEach(() => history.replaceState(null, "", "/"));
  afterEach(() => closeViewDialog());

  it("全 Store で評価する画面に、渡されたクエリをそのまま出す（既定の絞り込みは足さない）", () => {
    setup("table", "assignee:@me");
    expect(screen.getByRole("heading", { level: 1, name: "クエリ" })).toBeVisible();
    expect(screen.getByText("保存していないクエリ")).toBeVisible();
    expect(JSON.parse(screen.getByTestId("table").dataset.view ?? "")).toEqual({ kind: "all" });
    expect(screen.getByTestId("table-query")).toHaveTextContent("assignee:@me");
    expect(screen.getByTestId("table-settings")).toHaveTextContent(
      "id,title,status,assignee,due/none",
    );
  });

  it("layout のタブは、クエリを持ったまま /query/<layout> へ移る（保存しない）", async () => {
    setup("table", "assignee:@me");
    expect(tab("テーブル")).toHaveAttribute("aria-current", "page");
    expect(tab("カンバン")).toHaveAttribute("href", "/query/kanban?q=assignee%3A%40me");
    await fireEvent.click(tab("ツリー"));
    expect(here()).toBe("/query/tree?q=assignee%3A%40me");
  });

  it("layout ごとに本文が替わる", () => {
    setup("kanban");
    expect(screen.getByTestId("kanban")).toBeVisible();
    expect(screen.queryByTestId("table")).toBeNull();
  });

  it("ツリーも出る", () => {
    setup("tree", "kind:task");
    expect(screen.getByTestId("tree-query")).toHaveTextContent("kind:task");
  });

  it("フィルタ欄の変更は、履歴を積まずに URL の q に反映する", async () => {
    setup("table", "kind:task");
    const before = history.length;
    await fireEvent.click(screen.getByRole("button", { name: "table-set-query" }));
    expect(here()).toBe("/query/table?q=assignee%3A%40me");
    expect(history.length).toBe(before);
  });

  it("フィルタ欄の変更は、URL が戻ってくる前でも画面の値になり、外から q が変われば従う", async () => {
    const { rerender } = setup("table", "kind:task");
    await fireEvent.click(screen.getByRole("button", { name: "table-set-query" }));
    expect(screen.getByTestId("table-query")).toHaveTextContent("assignee:@me");
    await rerender({ layout: "table", q: "assignee:@me" });
    expect(screen.getByTestId("table-query")).toHaveTextContent("assignee:@me");
    await rerender({ layout: "table", q: "kind:bug" });
    expect(screen.getByTestId("table-query")).toHaveTextContent("kind:bug");
  });

  it("「ビューを保存」は今のレイアウト・クエリ・設定で保存のダイアログを開く（Store は決めない）", async () => {
    setup("table", "assignee:@me");
    await fireEvent.click(screen.getByRole("button", { name: "table-set-group" }));
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toEqual({
      kind: "save",
      draft: {
        layout: "table",
        query: "assignee:@me",
        kanban: DEFAULT_KANBAN,
        table: { ...DEFAULT_TABLE, groupBy: "parent" },
        gantt: DEFAULT_GANTT,
      },
      store: undefined,
    });
  });

  it("カンバンの設定も保存する", async () => {
    setup("kanban", "kind:task");
    await fireEvent.click(screen.getByRole("button", { name: "kanban-set-swimlane" }));
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toMatchObject({
      kind: "save",
      draft: { layout: "kanban", kanban: { columns: "status", swimlane: "assignee" } },
    });
  });

  it("ツリーにも「ビューを保存」がある", async () => {
    setup("tree", "kind:task");
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toMatchObject({
      kind: "save",
      draft: { layout: "tree", query: "kind:task" },
    });
  });
});
