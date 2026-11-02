import { cleanup, fireEvent } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderWithCore } from "../test-utils";
import Sidebar from "./Sidebar.svelte";

const workspaces = [
  {
    store: "shared",
    tone: "purple",
    ulid: "01WSREL",
    name: "release",
    prefix: "REL",
    projects: [
      { ulid: "01PRJ1", title: "リリース準備 v2.4", progress: { done: 3, total: 14 } },
      { ulid: "01PRJ2", title: "リリース準備 v2.5", progress: { done: 0, total: 3 } },
    ],
  },
  {
    store: "org-b/ops",
    tone: "readonly",
    ulid: "01WSOPS",
    name: "ops",
    prefix: "OPS",
    projects: [],
  },
];

function mount() {
  return renderWithCore(
    Sidebar,
    {},
    { stores: { workspaces: () => workspaces, subscribe: () => () => {} } },
  );
}

beforeEach(() => {
  history.replaceState(null, "", "/");
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("Sidebar", () => {
  it("ロゴと host を出す", () => {
    const { getByText } = mount();
    expect(getByText("hh")).toBeTruthy();
    expect(getByText(location.host)).toBeTruthy();
  });

  it("4e の入口（マイタスク・通知・アクティビティ・保存済みビュー）は出さない", () => {
    const { queryByText } = mount();
    for (const t of ["マイタスク", "通知", "アクティビティ", "保存済みビュー"]) {
      expect(queryByText(t)).toBeNull();
    }
  });

  it("ワークスペース › プロジェクトの入れ子と進捗を出す", () => {
    const { getByRole, getByText } = mount();
    const ws = getByRole("link", { name: /release/ });
    expect(ws.getAttribute("href")).toBe("/workspaces/shared/01WSREL");
    expect(getByText("REL")).toBeTruthy();
    const p = getByRole("link", { name: /リリース準備 v2.4/ });
    expect(p.getAttribute("href")).toBe("/projects/01PRJ1");
    expect(p.textContent).toContain("3/14");
    expect(getByText("プロジェクトなし")).toBeTruthy();
  });

  it("Store 名に / があっても href を符号化する", () => {
    const { getByRole } = mount();
    expect(getByRole("link", { name: /ops/ }).getAttribute("href")).toBe(
      "/workspaces/org-b%2Fops/01WSOPS",
    );
  });

  it("今いるプロジェクトを aria-current で示す", () => {
    history.replaceState(null, "", "/projects/01PRJ2");
    const { getByRole } = mount();
    expect(getByRole("link", { name: /リリース準備 v2.5/ }).getAttribute("aria-current")).toBe(
      "page",
    );
    expect(
      getByRole("link", { name: /リリース準備 v2.4/ }).getAttribute("aria-current"),
    ).toBeNull();
  });

  it("今いるワークスペースを示す。/ は最初のワークスペース", () => {
    history.replaceState(null, "", "/");
    const { getByRole } = mount();
    expect(getByRole("link", { name: /release/ }).getAttribute("aria-current")).toBe("page");
  });

  it("リンクを押すと router で移動する", async () => {
    const { getByRole } = mount();
    await fireEvent.click(getByRole("link", { name: /リリース準備 v2.4/ }));
    expect(location.pathname).toBe("/projects/01PRJ1");
  });
});
