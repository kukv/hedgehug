import { cleanup } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { WorkspaceEntry } from "../../core/stores";
import { renderWithCore } from "../test-utils";
import WorkspacePage from "./WorkspacePage.svelte";

// The bodies of the tabs have their own tests: here each only says which one is drawn.
const { marker } = vi.hoisted(() => ({
  marker: (what: string) => (anchor: Comment, props: { entry: { name: string } }) => {
    const p = document.createElement("p");
    p.textContent = `${what} of ${props.entry.name}`;
    anchor.before(p);
  },
}));
vi.mock("./WorkspaceOverview.svelte", () => ({ default: marker("overview") }));
vi.mock("./WorkspaceSettings.svelte", () => ({ default: marker("settings") }));
vi.mock("../list/TreeTab.svelte", () => ({
  default: (anchor: Comment, props: { view: { ulid: string } }) => {
    const p = document.createElement("p");
    p.textContent = `tree tab of ${props.view.ulid}`;
    anchor.before(p);
  },
}));
vi.mock("../list/ListTab.svelte", () => ({
  default: (anchor: Comment, props: { layout: string; view: { kind: string; ulid: string } }) => {
    const p = document.createElement("p");
    p.textContent = `${props.layout} tab of ${props.view.kind} ${props.view.ulid}`;
    anchor.before(p);
  },
}));

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
  tab: "overview" | "tree" | "kanban" | "table" | "settings",
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
  it("設定のタブは見出しとタブの下に設定を出す", () => {
    const { getByRole, getByText } = mount("settings");
    expect(getByRole("heading", { level: 1, name: "release" })).toBeTruthy();
    expect(
      getByRole("navigation", { name: "ビューの切り替え" }).querySelector("[aria-current]")
        ?.textContent,
    ).toContain("設定");
    expect(getByText("settings of release")).toBeTruthy();
  });

  it("ツリーのタブは見出しの下にツリーを出す", () => {
    const { getByRole, getByText } = mount("tree");
    expect(getByRole("heading", { level: 1, name: "release" })).toBeTruthy();
    expect(getByText("tree tab of W1")).toBeTruthy();
  });

  it.each(["kanban", "table"] as const)("%s のタブは見出しの下にそのレイアウトを出す", (tab) => {
    const { getByRole, getByText, queryByText } = mount(tab);
    expect(getByRole("heading", { level: 1, name: "release" })).toBeTruthy();
    expect(getByText(`${tab} tab of workspace W1`)).toBeTruthy();
    expect(queryByText("tree tab of W1")).toBeNull();
    expect(
      getByRole("navigation", { name: "ビューの切り替え" }).querySelector("[aria-current]")
        ?.textContent,
    ).toContain(tab === "kanban" ? "カンバン" : "テーブル");
  });

  it("概要のタブは見出しごと WorkspaceOverview に任せる", () => {
    const { getByText, queryByRole } = mount("overview");
    expect(getByText("overview of release")).toBeTruthy();
    expect(queryByRole("heading", { level: 1 })).toBeNull(); // the page draws none of its own
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
