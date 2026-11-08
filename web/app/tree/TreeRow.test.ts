import { cleanup, render } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { SchemaView } from "../../core/schema";
import { task } from "./fixtures";

vi.mock("../context", () => ({
  core: () => ({
    stores: { subscribe: () => () => {}, displayId: (_store: string, id: string) => id },
  }),
}));

import TreeRow from "./TreeRow.svelte";

afterEach(cleanup);

const schema = {
  statuses: [{ name: "review", category: "doing", color: "pink" }],
  kinds: [{ name: "incident", label: "障害", role: "work", icon: "bug" }],
  props: {},
} as unknown as SchemaView;

describe("TreeRow", () => {
  it("スキーマの color と icon を行のアイコンに出す", () => {
    const t = task("T1", "incident", null, { status: "review" });
    const { container } = render(TreeRow, {
      props: {
        row: {
          ulid: "T1",
          task: t,
          depth: 0,
          parent: null,
          hasChildren: false,
          childCount: 0,
          expanded: false,
        },
        schema,
        selected: false,
        focusable: true,
        live: undefined,
        blockedBy: undefined,
        onselect() {},
        ontoggle() {},
        onopen() {},
      },
    });
    const status = container.querySelector("svg[data-shape]") as SVGElement;
    const kind = container.querySelector("svg[data-kind]") as SVGElement;
    expect(status.style.getPropertyValue("--c")).toBe("var(--store-pink-dot)");
    expect(kind.dataset.kind).toBe("bug");
  });
});
