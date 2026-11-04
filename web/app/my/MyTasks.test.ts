import { cleanup, fireEvent, screen, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, type TaskView } from "../../core/api/client";
import { renderWithCore } from "../test-utils";
import { schema, task } from "../tree/fixtures";
import { MY_QUERY } from "./groups";
import MyTasks from "./MyTasks.svelte";

const project = { ulid: "P1", id: "REL-1", title: "リリース準備 v2.4" };
const t = (ulid: string, over: Partial<TaskView>): TaskView =>
  task(ulid, "task", null, { id: `REL-${ulid}`, title: `title ${ulid}`, ...over });

const tasks = [
  t("1", { props: { due: "2026-11-04" }, project, status: "doing" }),
  t("2", { props: { due: "2026-11-06" }, project }),
  t("3", {
    props: { due: "2026-11-07" },
    store: "me",
    workspace: { ulid: "W9", name: "me", id_prefix: "ME" },
  }),
  t("4", { props: {}, project }),
] as TaskView[];

function mount(
  over: {
    list?: () => string[];
    error?: ApiError | null;
    loading?: boolean;
    hidden?: string[];
    stores?: { name: string; account?: string }[];
  } = {},
) {
  const watchList = vi.fn((_q: { q: string; tree: boolean }) => ({
    ulids: () => (over.list ? over.list() : tasks.map((x) => x.ulid)),
    error: () => over.error ?? null,
    loading: () => over.loading ?? false,
    dispose: vi.fn(),
  }));
  const none = () => () => {};
  const view = renderWithCore(
    MyTasks,
    {},
    {
      cache: {
        subscribe: none,
        watchList,
        task: (u: string) => tasks.find((x) => x.ulid === u),
      },
      stores: {
        stores: () =>
          over.stores ?? [
            { name: "shared", account: "github.com/alice" },
            { name: "me", account: "ghe.example.com/alice-corp" },
          ],
        hidden: (s: string) => over.hidden?.includes(s) ?? false,
        // the sidebar order: "me" before "shared"
        visibleWorkspaces: () => [
          { store: "me", ulid: "W9", name: "me" },
          { store: "shared", ulid: "W1", name: "release" },
        ],
        readOnly: () => null,
        tone: (s: string) => (s === "shared" ? "purple" : "teal"),
        loaded: () => true,
        subscribe: none,
      },
      schema: { get: () => schema, load: async () => schema, subscribe: none },
    },
  );
  return { watchList, ...view };
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 10, 4, 12, 0)); // Wednesday
  history.replaceState(null, "", "/my");
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("MyTasks", () => {
  it("asks for the tasks of the person in every store, the default query", () => {
    const { watchList } = mount();
    expect(watchList).toHaveBeenCalledWith({ q: MY_QUERY, tree: false });
    expect(screen.getByLabelText("フィルタ（hh ls と同じクエリ）")).toHaveValue(MY_QUERY);
  });

  it("names the accounts, and counts the workspaces and the tasks", () => {
    mount();
    expect(screen.getByRole("heading", { level: 1, name: "マイタスク" })).toBeTruthy();
    expect(screen.getByText("自分 = alice ＋ alice-corp")).toBeTruthy();
    expect(screen.getByText("2 ワークスペース · 4 件")).toBeTruthy();
  });

  it("groups by due date: today, this week, none", () => {
    mount();
    const today = screen.getByRole("region", { name: "今日" });
    expect(within(today).getByText("title 1")).toBeTruthy();
    const week = screen.getByRole("region", { name: "今週" });
    expect(within(week).getByText("title 2")).toBeTruthy();
    expect(within(week).getByText("title 3")).toBeTruthy();
    const none = screen.getByRole("region", { name: "期限なし" });
    expect(within(none).getByText("title 4")).toBeTruthy();
    expect(screen.queryByRole("region", { name: "期限切れ" })).toBeNull();
  });

  it("shows the ID without a store, the project, the due date and the badge of the store", () => {
    mount();
    const row = screen.getByRole("row", { name: /title 2/ });
    expect(within(row).getByRole("link", { name: "REL-2" })).toHaveAttribute("href", "/tasks/2");
    expect(within(row).getByRole("link", { name: "title 2" })).toHaveAttribute("href", "/tasks/2");
    expect(within(row).getByText("リリース準備 v2.4")).toBeTruthy();
    expect(within(row).getByText("11/06")).toBeTruthy();
    expect(within(row).getByText("shared")).toBeTruthy();
    expect(within(screen.getByRole("row", { name: /title 3/ })).getByText("me")).toBeTruthy();
  });

  it("groups by project from the menu, the tasks outside any project last", async () => {
    mount();
    await fireEvent.click(screen.getByRole("button", { name: "まとめ方: 期限" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "プロジェクト" }));
    const p = screen.getByRole("region", { name: "リリース準備 v2.4" });
    expect(within(p).getAllByRole("row")).toHaveLength(3);
    const none = screen.getByRole("region", { name: "プロジェクトなし" });
    expect(within(none).getByText("title 3")).toBeTruthy();
    expect(screen.getByRole("button", { name: "まとめ方: プロジェクト" })).toBeTruthy();
  });

  it("asks again with the query typed in the field", async () => {
    const { watchList } = mount();
    const field = screen.getByLabelText("フィルタ（hh ls と同じクエリ）");
    await fireEvent.input(field, { target: { value: "assignee:@me kind:bug" } });
    await fireEvent.keyDown(field, { key: "Enter" });
    expect(watchList).toHaveBeenLastCalledWith({ q: "assignee:@me kind:bug", tree: false });
  });

  it("says there is nothing to do when the default query finds nothing", () => {
    mount({ list: () => [] });
    expect(screen.getByText("一致するタスクはありません")).toBeTruthy();
    expect(screen.getByText("自分が担当で、完了と中止でないタスクはありません")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "フィルタを解除" })).toBeNull();
  });

  it("goes back to the default query from a query that finds nothing", async () => {
    const { watchList } = mount({ list: () => [] });
    const field = screen.getByLabelText("フィルタ（hh ls と同じクエリ）");
    await fireEvent.input(field, { target: { value: "kind:bug" } });
    await fireEvent.keyDown(field, { key: "Enter" });
    await fireEvent.click(screen.getByRole("button", { name: "フィルタを解除" }));
    expect(watchList).toHaveBeenLastCalledWith({ q: MY_QUERY, tree: false });
  });

  it("says the list is being read", () => {
    mount({ loading: true });
    expect(screen.getByText("読み込み中…")).toBeTruthy();
    expect(screen.queryByText("一致するタスクはありません")).toBeNull();
  });

  it("tells why the query does not run, and draws no list", () => {
    mount({ error: new ApiError({ status: 400, type: "account.none" }) });
    expect(screen.getByRole("alert")).toHaveTextContent("アカウントが設定されていません");
    expect(screen.queryByRole("region")).toBeNull();
    expect(screen.queryByText("一致するタスクはありません")).toBeNull();
  });

  it("shows the command that gives the same list", () => {
    mount();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      `hh ls "${MY_QUERY}"`,
    );
  });

  it("offers the three groupings in the order of the canvas", async () => {
    mount();
    await fireEvent.click(screen.getByRole("button", { name: "まとめ方: 期限" }));
    expect(screen.getAllByRole("menuitem").map((e) => e.textContent?.trim())).toEqual([
      "期限",
      "ワークスペース",
      "プロジェクト",
    ]);
  });

  it("groups by workspace in the order of the sidebar, with the badge of the Store", async () => {
    mount();
    await fireEvent.click(screen.getByRole("button", { name: "まとめ方: 期限" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "ワークスペース" }));
    const regions = screen.getAllByRole("region");
    expect(regions).toHaveLength(2);
    // "me" is before "shared" in the sidebar
    expect(within(regions[0] as HTMLElement).getByRole("heading", { name: "me" })).toBeTruthy();
    expect(within(regions[0] as HTMLElement).getAllByRole("row")).toHaveLength(1);
    const release = screen.getByRole("region", { name: "release" });
    expect(within(release).getAllByRole("row")).toHaveLength(3);
    // the badge is by the name, outside the heading (so the region keeps the name alone)
    expect(release.querySelector(".g-head")).toHaveTextContent("shared");
    expect(screen.getByRole("button", { name: "まとめ方: ワークスペース" })).toBeTruthy();
  });

  describe("a hidden Store", () => {
    it("leaves its tasks out of the groups and the count, and tells how many with a link", () => {
      mount({ hidden: ["me"] });
      expect(screen.queryByText("title 3")).toBeNull();
      expect(screen.getByText("1 ワークスペース · 3 件")).toBeTruthy();
      expect(screen.getByText("非表示の Store に 1 件（me）")).toBeTruthy();
      expect(screen.getByRole("link", { name: "マウント中の Store を開く" })).toHaveAttribute(
        "href",
        "/settings/stores",
      );
    });

    it("lists the Stores in the order of the directory when there are two", () => {
      mount({ hidden: ["shared", "me"] });
      expect(screen.getByText("非表示の Store に 4 件（shared、me）")).toBeTruthy();
    });

    it("says nothing when no task is of a hidden Store", () => {
      mount({ hidden: ["other"] });
      expect(screen.queryByText(/非表示の Store に/)).toBeNull();
      expect(screen.queryByRole("link", { name: "マウント中の Store を開く" })).toBeNull();
    });

    it("keeps the notice when every task is of a hidden Store", () => {
      mount({ hidden: ["shared", "me"] });
      expect(screen.getByText("一致するタスクはありません")).toBeTruthy();
      expect(screen.getByText("非表示の Store に 4 件（shared、me）")).toBeTruthy();
    });
  });
});
