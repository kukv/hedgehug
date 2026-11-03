import { fireEvent, screen } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import { closeViewDialog, viewDialog } from "../views/dialogs-state.svelte";
import { mountWithViews } from "../views/testkit";
import ListTab from "./ListTab.svelte";
import type { ListScope } from "./scope";

// The scope of the screen and the two lists are stood in for: this test is about what ListTab holds.
vi.mock("./scope.svelte", () => ({
  useListScope: () => ({ ready: true, store: "shared", saveBase: "workspace:release" }),
}));
vi.mock("../kanban/KanbanView.svelte", async () => ({
  default: (await import("../views/fakes/FakeKanban.svelte")).default,
}));
vi.mock("../table/TableView.svelte", async () => ({
  default: (await import("../views/fakes/FakeTable.svelte")).default,
}));

const workspace: ListScope = { kind: "workspace", store: "shared", ulid: "W1" };

afterEach(() => closeViewDialog());

describe("ListTab", () => {
  it("カンバンのタブ: フィルタの既定は -status:canceled、設定は既定", () => {
    mountWithViews(ListTab, { view: workspace, layout: "kanban" });
    expect(JSON.parse(screen.getByTestId("kanban").dataset.view ?? "")).toEqual(workspace);
    expect(screen.getByTestId("kanban-query")).toHaveTextContent("-status:canceled");
    expect(screen.getByTestId("kanban-settings")).toHaveTextContent("status/none");
  });

  it("テーブルのタブ: 設定は id,title,status,assignee,due・グループなし", () => {
    mountWithViews(ListTab, { view: workspace, layout: "table" });
    expect(screen.getByTestId("table-query")).toHaveTextContent("-status:canceled");
    expect(screen.getByTestId("table-settings")).toHaveTextContent(
      "id,title,status,assignee,due/none",
    );
  });

  it("フィルタと設定は画面の状態で、保存のダイアログに渡る（D7・D8）", async () => {
    mountWithViews(ListTab, { view: workspace, layout: "kanban" });
    await fireEvent.click(screen.getByRole("button", { name: "kanban-set-query" }));
    await fireEvent.click(screen.getByRole("button", { name: "kanban-set-swimlane" }));
    expect(screen.getByTestId("kanban-query")).toHaveTextContent("assignee:@me");
    expect(screen.getByTestId("kanban-settings")).toHaveTextContent("status/assignee");
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toMatchObject({
      kind: "save",
      draft: {
        layout: "kanban",
        query: "workspace:release assignee:@me",
        kanban: { columns: "status", swimlane: "assignee" },
      },
      store: "shared",
    });
  });

  it("カンバンとテーブルのタブを替えても、フィルタは残る", async () => {
    const { rerender } = mountWithViews(ListTab, { view: workspace, layout: "kanban" });
    await fireEvent.click(screen.getByRole("button", { name: "kanban-set-query" }));
    await rerender({ view: workspace, layout: "table" });
    expect(screen.queryByTestId("kanban")).toBeNull();
    expect(screen.getByTestId("table-query")).toHaveTextContent("assignee:@me");
    await fireEvent.click(screen.getByRole("button", { name: "table-set-group" }));
    await fireEvent.click(screen.getByRole("button", { name: "ビューを保存" }));
    expect(viewDialog.current).toMatchObject({
      draft: {
        layout: "table",
        query: "workspace:release assignee:@me",
        table: { groupBy: "parent" },
      },
    });
  });
});
