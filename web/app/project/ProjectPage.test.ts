import { cleanup, screen } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import { renderWithCore } from "../test-utils";
import { task } from "../tree/fixtures";
import { entry } from "../workspace/testkit";
import ProjectPage from "./ProjectPage.svelte";

// The bodies of the tabs have their own tests: here each only says which one is drawn.
vi.mock("../list/TreeTab.svelte", () => ({
  default: (anchor: Comment) => {
    const p = document.createElement("p");
    p.textContent = "tree tab";
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

const project = task("P1", "project", null, {
  id: "REL-15",
  title: "リリース準備 v2.4",
  workspace: { ulid: "W1", name: "release", id_prefix: "REL" },
}) as TaskView;

function mount(tab: "tree" | "kanban" | "table", read = true) {
  const none = () => () => {};
  return renderWithCore(
    ProjectPage,
    { ulid: "P1", tab },
    {
      cache: {
        task: (u: string) => (read && u === "P1" ? project : undefined),
        watchDetail: () => ({ ulid: () => null, error: () => null, dispose() {} }),
        subscribe: none,
      },
      stores: {
        workspaces: () => [entry()],
        workspace: () => entry(),
        loaded: () => true,
        tone: () => "purple",
        subscribe: none,
      },
      schema: { subscribe: none },
    },
  );
}

afterEach(cleanup);

describe("ProjectPage", () => {
  it("draws the heading with the tabs above the tree", () => {
    mount("tree");
    expect(screen.getByRole("heading", { level: 1, name: "リリース準備 v2.4" })).toBeTruthy();
    expect(screen.getByRole("navigation", { name: "ビューの切り替え" })).toBeTruthy();
    expect(screen.getByText("tree tab")).toBeTruthy();
  });

  it.each(["kanban", "table"] as const)("draws the %s tab for the project", (tab) => {
    mount(tab);
    expect(screen.getByText(`${tab} tab of project P1`)).toBeTruthy();
    expect(screen.queryByText("tree tab")).toBeNull();
  });

  it("leaves the body to say the project is missing, and draws no heading until it is read", () => {
    mount("kanban", false);
    expect(screen.queryByRole("heading")).toBeNull();
    expect(screen.getByText("kanban tab of project P1")).toBeTruthy();
  });
});
