import { cleanup } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import type { WorkspaceEntry } from "../../core/stores";
import { renderWithCore } from "../test-utils";
import WorkspacePage from "./WorkspacePage.svelte";

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

function mount(
  tab: "overview" | "tree" | "settings",
  over: { loaded?: boolean; found?: boolean } = {},
) {
  const loaded = over.loaded ?? true;
  const found = over.found ?? true;
  return renderWithCore(
    WorkspacePage,
    { store: "shared", ulid: "W1", tab },
    {
      stores: {
        workspace: (s: string, u: string) =>
          found && s === "shared" && u === "W1" ? entry : undefined,
        loaded: () => loaded,
        tone: () => "purple",
        subscribe: () => () => {},
      },
    },
  );
}

beforeEach(() => history.replaceState(null, "", "/"));
afterEach(cleanup);

describe("WorkspacePage", () => {
  it.each(["overview", "settings"] as const)("%s のタブでも見出しとタブを出す", (tab) => {
    const { getByRole } = mount(tab);
    expect(getByRole("heading", { level: 1, name: "release" })).toBeTruthy();
    expect(
      getByRole("navigation", { name: "ビューの切り替え" }).querySelector("[aria-current]")
        ?.textContent,
    ).toContain(tab === "overview" ? "概要" : "設定");
  });

  it("ワークスペースがないときは、そう言ってホームへ戻れる", () => {
    const { getByText, getByRole } = mount("overview", { found: false });
    expect(getByText("ワークスペース「W1」が見つかりません")).toBeTruthy();
    getByRole("button", { name: "ホームへ戻る" }).click();
    expect(location.pathname).toBe("/");
  });

  it("Store を読み込むまでは読み込み中", () => {
    const { getByText, queryByRole } = mount("overview", { loaded: false, found: false });
    expect(getByText("読み込み中…")).toBeTruthy();
    expect(queryByRole("heading")).toBeNull();
  });
});
