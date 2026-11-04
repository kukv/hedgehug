import { cleanup, fireEvent, screen, within } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import { renderWithCore } from "../test-utils";
import { task } from "../tree/fixtures";
import { entry } from "../workspace/testkit";
import ProjectPage from "./ProjectPage.svelte";

// The bodies of the tabs have their own tests: here each only says which one is drawn.
type TabProps = { query: string; onquery: (q: string) => void };
// What a tab is given for its filter, and a button that changes it as the field would.
const filterOf = (props: TabProps) => {
  const f = document.createElement("span");
  f.textContent = `filter: ${props.query}`;
  const b = document.createElement("button");
  b.textContent = "set-filter";
  b.onclick = () => props.onquery("kind:bug");
  return [f, b];
};
vi.mock("../list/TreeTab.svelte", () => ({
  default: (anchor: Comment, props: TabProps) => {
    const p = document.createElement("p");
    p.textContent = "tree tab";
    anchor.before(p, ...filterOf(props));
  },
}));
vi.mock("../list/ListTab.svelte", () => ({
  default: (
    anchor: Comment,
    props: TabProps & { layout: string; view: { kind: string; ulid: string } },
  ) => {
    const p = document.createElement("p");
    p.textContent = `${props.layout} tab of ${props.view.kind} ${props.view.ulid}`;
    anchor.before(p, ...filterOf(props));
  },
}));

vi.mock("./ProjectSettings.svelte", () => ({
  default: (anchor: Comment, props: { ulid: string }) => {
    const p = document.createElement("p");
    p.textContent = `settings of ${props.ulid}`;
    anchor.before(p);
  },
}));

const project = task("P1", "project", null, {
  id: "REL-15",
  title: "リリース準備 v2.4",
  workspace: { ulid: "W1", name: "release", id_prefix: "REL" },
}) as TaskView;

function mount(tab: "tree" | "kanban" | "table" | "settings", read = true, readOnly?: string) {
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
        readOnly: () => readOnly ?? null,
        subscribe: none,
      },
      schema: { subscribe: none },
    },
  );
}

afterEach(cleanup);

describe("ProjectPage", () => {
  it("shows the band of a read-only Store above the heading, and none for a Store that can be written", () => {
    const { unmount } = mount("tree", true, "the remote cannot be written");
    expect(screen.getByRole("status", { name: "閲覧のみ" })).toHaveTextContent(
      "この Store は閲覧のみです（remote に書き込めません）",
    );
    unmount();
    mount("tree");
    expect(screen.queryByRole("status", { name: "閲覧のみ" })).toBeNull();
  });

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

  it("draws the settings of the project, and none of the other tabs, on the settings tab", () => {
    mount("settings");
    expect(screen.getByText("settings of P1")).toBeTruthy();
    expect(screen.queryByText("tree tab")).toBeNull();
    expect(screen.queryByText(/ tab of project/)).toBeNull();
    expect(screen.getByRole("link", { name: "設定" })).toHaveAttribute("aria-current", "page");
  });

  it("keeps the filter across the tabs, and starts with the one that leaves the canceled out", async () => {
    const { rerender } = mount("tree");
    expect(screen.getByText("filter: -status:canceled")).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: "set-filter" }));
    for (const tab of ["kanban", "table", "tree"] as const) {
      await rerender({ ulid: "P1", tab });
      // The stand-ins leave what they drew, so the last filter shown is the tab drawn last.
      expect(screen.getAllByText(/^filter:/).at(-1)?.textContent).toBe("filter: kind:bug");
    }
  });

  it("draws the tabs at once, before the project is read, and leaves the body to say it is missing", () => {
    mount("kanban", false);
    // as the heading of the tree did: no title yet, but the tabs do not pop in later
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("");
    const tabs = screen.getByRole("navigation", { name: "ビューの切り替え" });
    expect(within(tabs).getAllByRole("link")).toHaveLength(4);
    expect(within(tabs).getByRole("link", { name: "カンバン" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByText("kanban tab of project P1")).toBeTruthy();
  });
});
