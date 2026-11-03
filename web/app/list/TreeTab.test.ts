import { cleanup, fireEvent } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DEFAULT_KANBAN, DEFAULT_TABLE } from "../../core/views";
import { renderWithCore } from "../test-utils";
import TreeTab from "./TreeTab.svelte";

// The tree has its own tests: here it shows the filter it is given and where its actions are.
const { open } = vi.hoisted(() => ({ open: vi.fn() }));
vi.mock("../tree/TreeView.svelte", () => ({
  default: (
    anchor: Comment,
    props: { query: string; onquery: (q: string) => void; actions: (anchor: Comment) => void },
  ) => {
    const p = document.createElement("p");
    p.textContent = `filter: ${props.query}`;
    const b = document.createElement("button");
    b.textContent = "絞り込む";
    b.onclick = () => props.onquery("kind:bug");
    anchor.before(p, b);
    props.actions(anchor);
  },
}));
vi.mock("../views/dialogs-state.svelte", () => ({ openViewSave: open }));

const workspace = {
  store: "shared",
  tone: "purple",
  ulid: "W1",
  name: "release",
  prefix: "REL",
  projects: [],
};
const stores = (count: number) => ({
  workspace: () => workspace,
  workspaces: () => [workspace],
  stores: () => Array.from({ length: count }, (_, i) => ({ name: i === 0 ? "shared" : `s${i}` })),
  loaded: () => true,
  subscribe: () => () => {},
});
const cache = { task: () => undefined, subscribe: () => () => {} };

function mount(count = 1) {
  return renderWithCore(
    TreeTab,
    { view: { kind: "workspace" as const, store: "shared", ulid: "W1" } },
    { stores: stores(count), cache },
  );
}

beforeEach(() => open.mockClear());
afterEach(cleanup);

describe("TreeTab", () => {
  it("gives the tree the filter that leaves the canceled tasks out", () => {
    const { getByText } = mount();
    expect(getByText("filter: -status:canceled")).toBeTruthy();
  });

  it("opens the save dialog with the fixed part and the filter, as a tree", async () => {
    const { getByRole } = mount();
    await fireEvent.click(getByRole("button", { name: "絞り込む" }));
    await fireEvent.click(getByRole("button", { name: "ビューを保存" }));
    expect(open).toHaveBeenCalledWith({
      draft: {
        layout: "tree",
        query: "workspace:release kind:bug",
        kanban: DEFAULT_KANBAN,
        table: DEFAULT_TABLE,
      },
      store: "shared",
    });
  });

  it("cannot save before the workspace is known", () => {
    const { getByRole } = renderWithCore(
      TreeTab,
      { view: { kind: "workspace" as const, store: "shared", ulid: "W1" } },
      { stores: { ...stores(1), workspace: () => undefined, loaded: () => false }, cache },
    );
    expect(getByRole("button", { name: "ビューを保存" })).toBeDisabled();
  });

  it("keeps the view to the store of the screen when two stores are mounted", async () => {
    const { getByRole } = mount(2);
    await fireEvent.click(getByRole("button", { name: "ビューを保存" }));
    expect(open.mock.calls[0]?.[0].draft.query).toBe(
      "store:shared workspace:release -status:canceled",
    );
  });
});
