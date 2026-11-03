import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../../core/api/client";
import { cliViewSave, cliViewShow } from "../../core/cli";
import { draftOf, saveBodyOf } from "../../core/views";
import { resetToasts, toasts } from "../toast.svelte";
import { carryDraft, closeViewDialog, viewDialog } from "./dialogs-state.svelte";
import { makeViews, mountWithViews, viewEntry } from "./testkit";
import ViewPage from "./ViewPage.svelte";

vi.mock("../tree/TreeView.svelte", async () => ({
  default: (await import("./fakes/FakeTree.svelte")).default,
}));
vi.mock("../kanban/KanbanView.svelte", async () => ({
  default: (await import("./fakes/FakeKanban.svelte")).default,
}));
vi.mock("../table/TableView.svelte", async () => ({
  default: (await import("./fakes/FakeTable.svelte")).default,
}));

const weekly = viewEntry({ name: "weekly", layout: "table", query: "kind:task" });

function setup(entries = [weekly], store = "shared", name = "weekly") {
  const views = makeViews(entries);
  return mountWithViews(ViewPage, { store, name }, { views });
}

const tab = (name: string) =>
  within(screen.getByRole("navigation", { name: "レイアウトの切り替え" })).getByRole("link", {
    name,
  });

describe("ViewPage", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/views/shared/weekly");
  });
  afterEach(() => closeViewDialog());

  it("見出しに名前・保存先の Store・ファイルの場所を出し、保存した layout で開く", () => {
    setup();
    expect(screen.getByRole("heading", { level: 1, name: "weekly" })).toBeVisible();
    expect(screen.getByTitle("Store: shared")).toBeVisible();
    expect(screen.getByText("shared の views/weekly.yaml")).toBeVisible();
    expect(tab("テーブル")).toHaveAttribute("aria-current", "page");
    expect(tab("ツリー")).not.toHaveAttribute("aria-current");
    expect(screen.getByTestId("table")).toBeVisible();
    expect(screen.queryByTestId("kanban")).toBeNull();
  });

  it("全 Store で評価する画面を、保存したクエリと設定で出し、足元は hh view show", () => {
    setup();
    const t = screen.getByTestId("table");
    expect(JSON.parse(t.dataset.view ?? "")).toEqual({ kind: "all" });
    expect(screen.getByTestId("table-query")).toHaveTextContent("kind:task");
    expect(screen.getByTestId("table-settings")).toHaveTextContent(
      "id,title,status,assignee,due/none",
    );
    expect(t.dataset.cli).toBe(cliViewShow("weekly", "shared"));
  });

  it("layout ごとに本文が替わる", () => {
    setup([viewEntry({ name: "weekly", layout: "kanban" })]);
    expect(screen.getByTestId("kanban")).toBeVisible();
    expect(screen.getByTestId("kanban-settings")).toHaveTextContent("status/none");
  });

  it("ツリーのビューは TreeView にクエリを渡す", () => {
    setup([viewEntry({ name: "weekly", layout: "tree", query: "kind:task" })]);
    expect(screen.getByTestId("tree-query")).toHaveTextContent("kind:task");
  });

  it("タブを押すと layout だけを PUT して、その layout の本文になる（未保存の変更を送らない）", async () => {
    const { views } = setup();
    await fireEvent.click(tab("カンバン"));
    await waitFor(() => expect(screen.getByTestId("kanban")).toBeVisible());
    expect(views.save).toHaveBeenCalledWith("shared", "weekly", { layout: "kanban" });
    expect(tab("カンバン")).toHaveAttribute("aria-current", "page");
    expect(location.pathname).toBe("/views/shared/weekly");
    expect(screen.queryByText("未保存の変更")).toBeNull();
  });

  it("タブの足元に、layout を変える hh view save を出す", () => {
    setup();
    expect(
      screen.getByText("切り替えは保存したビューの layout を変えます（1 コミット）"),
    ).toBeVisible();
    expect(
      screen.getByRole("button", {
        name: `コマンドをコピー: ${cliViewSave({ name: "weekly", store: "shared", body: { layout: "table" } })}`,
      }),
    ).toBeVisible();
  });

  it("切り替えをサーバーが断ったら、その言葉をトーストに出して、layout は変わらない", async () => {
    const { views } = setup([viewEntry({ name: "weekly", layout: "kanban", query: "kind:task" })]);
    views.save.mockRejectedValueOnce(new ApiError({ status: 422, type: "view.tree_order" }));
    await fireEvent.click(tab("ツリー"));
    await waitFor(() => expect(toasts.list).toHaveLength(1));
    expect(toasts.list[0]?.message).toBe(
      "ツリーのビューは rank の順に並ぶため、group: と sort: は使えません",
    );
    expect(tab("カンバン")).toHaveAttribute("aria-current", "page");
    expect(screen.getByTestId("kanban")).toBeVisible();
  });

  it("クエリが保存した値と違えば「未保存の変更」と「ビューを更新」を出し、1回の PUT に全部送る", async () => {
    const { views } = setup();
    expect(screen.queryByText("未保存の変更")).toBeNull();
    expect(screen.queryByRole("button", { name: "ビューを更新" })).toBeNull();

    await fireEvent.click(screen.getByRole("button", { name: "table-set-group" }));
    await fireEvent.click(screen.getByRole("button", { name: "table-set-query" }));
    expect(screen.getByText("未保存の変更")).toBeVisible();

    const next = {
      ...draftOf(weekly.view),
      table: { columns: ["id", "title", "status", "assignee", "due"], groupBy: "parent" as const },
    };
    await fireEvent.click(screen.getByRole("button", { name: "ビューを更新" }));
    await waitFor(() => expect(screen.queryByText("未保存の変更")).toBeNull());
    expect(views.save).toHaveBeenCalledTimes(1);
    expect(views.save).toHaveBeenCalledWith(
      "shared",
      "weekly",
      saveBodyOf({ ...next, query: "assignee:@me" }),
    );
    expect(toasts.list[0]?.message).toBe("ビュー weekly を更新しました");
    expect(screen.getByTestId("table-settings")).toHaveTextContent("/parent");
  });

  it("設定だけを変えても未保存になる（カンバンのスイムレーン）", async () => {
    setup([viewEntry({ name: "weekly", layout: "kanban" })]);
    await fireEvent.click(screen.getByRole("button", { name: "kanban-set-swimlane" }));
    expect(screen.getByText("未保存の変更")).toBeVisible();
    expect(screen.getByTestId("kanban-settings")).toHaveTextContent("status/assignee");
  });

  it("更新の足元に、今の設定を保存する hh view save を出す", async () => {
    setup([viewEntry({ name: "weekly", layout: "kanban", query: "" })]);
    await fireEvent.click(screen.getByRole("button", { name: "kanban-set-query" }));
    const d = { ...draftOf(weekly.view), layout: "kanban" as const, query: "assignee:@me" };
    d.kanban = { columns: "status", swimlane: "none" };
    expect(
      screen.getByRole("button", {
        name: `コマンドをコピー: ${cliViewSave({ name: "weekly", store: "shared", body: saveBodyOf(d) })}`,
      }),
    ).toBeVisible();
  });

  it("未保存の変更は、layout を切り替えても残り、更新では保存した layout で送る", async () => {
    const { views } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "table-set-query" }));
    await fireEvent.click(tab("カンバン"));
    await waitFor(() => expect(screen.getByTestId("kanban")).toBeVisible());
    expect(views.save).toHaveBeenLastCalledWith("shared", "weekly", { layout: "kanban" });
    expect(screen.getByTestId("kanban-query")).toHaveTextContent("assignee:@me");
    expect(screen.getByText("未保存の変更")).toBeVisible();
    await fireEvent.click(screen.getByRole("button", { name: "ビューを更新" }));
    await waitFor(() => expect(screen.queryByText("未保存の変更")).toBeNull());
    expect(views.save).toHaveBeenLastCalledWith(
      "shared",
      "weekly",
      saveBodyOf({ ...draftOf(weekly.view), layout: "kanban", query: "assignee:@me" }),
    );
  });

  it("ツリーのビューでクエリに sort: を入れたら、更新を無効にして理由を出す（細部 54）", async () => {
    const { views } = setup([viewEntry({ name: "weekly", layout: "tree", query: "kind:task" })]);
    await fireEvent.click(screen.getByRole("button", { name: "tree-set-sort" }));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "ツリーのビューは rank の順に並ぶため、group: と sort: は使えません",
    );
    expect(screen.getByRole("button", { name: "ビューを更新" })).toBeDisabled();
    await fireEvent.click(screen.getByRole("button", { name: "ビューを更新" }));
    expect(views.save).not.toHaveBeenCalled();
  });

  it("更新をサーバーが断ったら、トーストに出して未保存のまま残す", async () => {
    const { views } = setup();
    views.save.mockRejectedValueOnce(
      new ApiError({ status: 422, type: "view.bad_column", args: { column: "x" } }),
    );
    await fireEvent.click(screen.getByRole("button", { name: "table-set-query" }));
    await fireEvent.click(screen.getByRole("button", { name: "ビューを更新" }));
    await waitFor(() => expect(toasts.list).toHaveLength(1));
    expect(toasts.list[0]?.message).toBe(
      "列 x は、フィールドでもスキーマのプロパティでもありません",
    );
    expect(screen.getByText("未保存の変更")).toBeVisible();
  });

  it("「…」から改名と削除のダイアログを開く", async () => {
    setup();
    await fireEvent.click(screen.getByRole("button", { name: "ビューの操作（改名・削除）" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "改名…" }));
    expect(viewDialog.current).toMatchObject({ kind: "rename", entry: weekly });

    await fireEvent.click(screen.getByRole("button", { name: "ビューの操作（改名・削除）" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "削除…" }));
    expect(viewDialog.current).toMatchObject({ kind: "delete", entry: weekly });
  });

  it("未保存の変更があるまま改名を開くと、その変更を持って行く。なければ持たない", async () => {
    setup();
    await fireEvent.click(screen.getByRole("button", { name: "ビューの操作（改名・削除）" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "改名…" }));
    expect(viewDialog.current).toMatchObject({ kind: "rename", carry: null });
    closeViewDialog();

    await fireEvent.click(screen.getByRole("button", { name: "table-set-query" }));
    await fireEvent.click(screen.getByRole("button", { name: "ビューの操作（改名・削除）" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "改名…" }));
    expect(viewDialog.current).toMatchObject({
      kind: "rename",
      carry: { layout: "table", query: "assignee:@me" },
    });
  });

  it("改名で持ち越された変更があれば、開いたときから「未保存の変更」になる", () => {
    carryDraft("shared", "weekly", { ...draftOf(weekly.view), query: "assignee:@me" });
    setup();
    expect(screen.getByText("未保存の変更")).toBeVisible();
    expect(screen.getByTestId("table-query")).toHaveTextContent("assignee:@me");
  });

  it("保存したクエリに sort: があれば、ツリーのタブを無効にして理由を出す（細部 54）", async () => {
    const { views } = setup([viewEntry({ name: "weekly", layout: "table", query: "sort:title" })]);
    const why = "ツリーのビューは rank の順に並ぶため、group: と sort: は使えません";
    expect(tab("ツリー")).toHaveAttribute("aria-disabled", "true");
    expect(tab("ツリー")).toHaveAttribute("title", why);
    expect(screen.getByText(`ツリーには切り替えられません。${why}`)).toBeVisible();
    await fireEvent.click(tab("ツリー"));
    expect(views.save).not.toHaveBeenCalled();
    expect(tab("カンバン")).not.toHaveAttribute("aria-disabled");
  });

  it("問題がなければツリーのタブは有効", () => {
    setup();
    expect(tab("ツリー")).not.toHaveAttribute("aria-disabled");
    expect(screen.queryByText(/ツリーには切り替えられません/)).toBeNull();
  });

  it("カンバン・テーブルには、ビューの名前と Store を渡す（設定を保存するコマンドに使う）", () => {
    setup();
    expect(JSON.parse(screen.getByTestId("table").dataset.saved ?? "")).toEqual({
      name: "weekly",
      store: "shared",
    });
  });

  it("同じ名前のビューが別の Store にあっても、URL の Store のものを出す（細部 53）", () => {
    setup(
      [
        viewEntry({ name: "weekly", layout: "kanban" }, "other", "teal"),
        viewEntry({ name: "weekly", layout: "table" }, "shared"),
      ],
      "other",
    );
    expect(screen.getByTestId("kanban")).toBeVisible();
    expect(screen.getByText("other の views/weekly.yaml")).toBeVisible();
  });

  it("ビューが見つからなければ（消された・改名された）、見つからないと言う（D9）", async () => {
    const { views } = setup();
    expect(screen.getByTestId("table")).toBeVisible();
    views.set([]);
    expect(await screen.findByText("ビュー「weekly」が見つかりません")).toBeVisible();
    await fireEvent.click(screen.getByRole("button", { name: "ホームへ戻る" }));
    expect(location.pathname).toBe("/");
  });

  it("読み込みが終わるまでは、見つからないと言わない", () => {
    const views = makeViews([]);
    views.loaded.mockReturnValue(false);
    mountWithViews(ViewPage, { store: "shared", name: "weekly" }, { views });
    expect(screen.getByText("読み込み中…")).toBeVisible();
    expect(screen.queryByText(/見つかりません/)).toBeNull();
  });
});
