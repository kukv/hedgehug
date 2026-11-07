import { cleanup, fireEvent, screen, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { formatDate } from "../../core/format";
import { makeCore, mount, taskView } from "../detail/testkit";
import { closeWorkspaceDialog, workspaceDialog } from "../workspace/dialogs-state.svelte";
import { entry } from "../workspace/testkit";
import SettingsWorkspaces from "./SettingsWorkspaces.svelte";

const ws = (ulid: string) => ({ ulid, name: ulid, id_prefix: ulid });
const project = (ulid: string, w: string) => taskView({ ulid, kind: "project", workspace: ws(w) });
const spaces = [
  entry({
    ulid: "W1",
    name: "release",
    prefix: "REL",
    tasks: 24,
    deleted: 1,
    projects: [project("P1", "W1"), project("P2", "W1")],
  }),
  entry({ ulid: "W2", name: "shared", prefix: "SH", tasks: 9, deleted: 0 }),
  entry({ store: "backend", ulid: "B1", name: "api", prefix: "API", tasks: 3, deleted: 0 }),
];

function setup() {
  const k = makeCore({ workspaces: spaces });
  return { k, ...mount(SettingsWorkspaces, { store: "shared" }, k) };
}

describe("SettingsWorkspaces", () => {
  beforeEach(() => history.replaceState(null, "", "/settings/workspaces/shared"));
  afterEach(() => closeWorkspaceDialog());

  it("lists the workspaces of the store with their prefix, projects, tasks (deleted ones included) and the day the prefix was last changed", () => {
    setup();
    const rows = screen.getAllByRole("row");
    expect(rows).toHaveLength(3); // the header and the two workspaces of shared
    expect(
      within(rows[0] as HTMLElement)
        .getAllByRole("columnheader")
        .map((h) => h.textContent?.trim()),
    ).toEqual([
      "名前",
      "ID の接頭辞",
      "プロジェクト",
      "タスク（削除済みを含む）",
      "最終更新日",
      "操作",
    ]);
    const release = within(rows[1] as HTMLElement);
    expect(release.getByRole("rowheader")).toHaveTextContent("release");
    expect(release.getAllByRole("cell").map((c) => c.textContent?.trim())).toEqual([
      "REL",
      "2",
      "25",
      formatDate("2026-10-29T12:00:00Z", new Date()),
      "設定",
    ]);
    const shared = within(rows[2] as HTMLElement);
    expect(shared.getAllByRole("cell").map((c) => c.textContent?.trim())).toEqual([
      "SH",
      "0",
      "9",
      formatDate("2026-10-29T12:00:00Z", new Date()),
      "設定",
    ]);
    expect(screen.queryByText("api")).toBeNull(); // a workspace of another store
  });

  it("says so when the store has no workspace, and a dash for a prefix never set", () => {
    const k = makeCore({
      workspaces: [
        entry({ store: "empty-store", ulid: "E1", name: "e", prefixSet: "0001-01-01T00:00:00Z" }),
      ],
    });
    mount(SettingsWorkspaces, { store: "none" }, k);
    expect(screen.getByText("この Store にはワークスペースがありません")).toBeInTheDocument();
    cleanup();
    mount(
      SettingsWorkspaces,
      { store: "empty-store" },
      makeCore({
        workspaces: [
          entry({ store: "empty-store", ulid: "E1", name: "e", prefixSet: "0001-01-01T00:00:00Z" }),
        ],
      }),
    );
    expect(screen.getByRole("rowheader", { name: "e" }).parentElement).toHaveTextContent("—");
  });

  it("links each row to the settings tab of the workspace", async () => {
    setup();
    const link = screen.getByRole("link", { name: "release の設定" });
    expect(link).toHaveAttribute("href", "/workspaces/shared/W1/settings");
    await fireEvent.click(link);
    expect(location.pathname).toBe("/workspaces/shared/W1/settings");
  });

  it("opens the add dialog for this store", async () => {
    setup();
    await fireEvent.click(screen.getByRole("button", { name: "ワークスペースを追加" }));
    expect(workspaceDialog.current).toEqual({ kind: "add", store: "shared" });
  });

  it("shows the prefixes of the other stores and the command, and leaves the prefix rule to the dialog", () => {
    setup();
    expect(screen.queryByText(/接頭辞は英大文字と数字/)).toBeNull();
    expect(screen.getByText(/マウント中のほかの Store が使っている接頭辞:/)).toHaveTextContent(
      "API（backend）",
    );
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh workspace ls --store shared",
    );
  });
});
