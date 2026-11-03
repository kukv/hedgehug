import { cleanup, fireEvent, screen, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import type { TaskView } from "../../core/api/client";
import { renderWithCore } from "../test-utils";
import { task } from "../tree/fixtures";
import { entry } from "../workspace/testkit";
import ProjectHeader from "./ProjectHeader.svelte";

const project = task("P1", "project", null, {
  id: "REL-15",
  title: "リリース準備 v2.4",
}) as TaskView;

function mount(props: { tab: "tree" | "kanban" | "table"; withWorkspace?: boolean }) {
  return renderWithCore(
    ProjectHeader,
    { project, workspace: props.withWorkspace === false ? undefined : entry(), tab: props.tab },
    { stores: { tone: () => "purple", subscribe: () => () => {} } },
  );
}

beforeEach(() => history.replaceState(null, "", "/projects/P1"));
afterEach(cleanup);

describe("ProjectHeader", () => {
  it("shows where the project is: the store, the workspace, its name and its ID", () => {
    mount({ tab: "tree" });
    const crumb = screen.getByRole("navigation", { name: "パンくず" });
    expect(within(crumb).getByText("shared")).toBeTruthy();
    expect(within(crumb).getByRole("link", { name: /release/ })).toHaveAttribute(
      "href",
      "/workspaces/shared/W1",
    );
    expect(screen.getByRole("heading", { level: 1, name: "リリース準備 v2.4" })).toBeTruthy();
    expect(within(crumb).getByText("REL-15")).toBeTruthy();
  });

  it("goes to the overview of the workspace through the router", async () => {
    mount({ tab: "tree" });
    await fireEvent.click(screen.getByRole("link", { name: /release/ }));
    expect(location.pathname).toBe("/workspaces/shared/W1");
  });

  it("draws no workspace until it is known", () => {
    mount({ tab: "tree", withWorkspace: false });
    expect(screen.queryByRole("link", { name: /release/ })).toBeNull();
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy();
  });

  it("has the tabs tree, kanban and table as links, the current one marked", () => {
    mount({ tab: "kanban" });
    const links = within(screen.getByRole("navigation", { name: "ビューの切り替え" })).getAllByRole(
      "link",
    );
    expect(links.map((a) => a.textContent?.trim())).toEqual(["ツリー", "カンバン", "テーブル"]);
    expect(links.map((a) => a.getAttribute("href"))).toEqual([
      "/projects/P1",
      "/projects/P1/kanban",
      "/projects/P1/table",
    ]);
    expect(links.map((a) => a.getAttribute("aria-current"))).toEqual([null, "page", null]);
  });

  it("does not offer the settings of a project yet", () => {
    mount({ tab: "tree" });
    expect(screen.queryByRole("link", { name: "設定" })).toBeNull();
  });
});
