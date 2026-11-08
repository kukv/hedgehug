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
  props: {},
} as unknown as SchemaView;

function renderRow(extra: Parameters<typeof task>[3] & object, kind = "task") {
  const t = task("T1", kind, null, extra);
  return render(TreeRow, {
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
}

describe("TreeRow", () => {
  it("ステータスの color と種類の固定の形を行のアイコンに出す", () => {
    const { container } = renderRow({ status: "review" }, "bug");
    const status = container.querySelector("svg[data-shape]") as SVGElement;
    const kind = container.querySelector("svg[data-kind]") as SVGElement;
    expect(status.style.getPropertyValue("--c")).toBe("var(--store-pink-dot)");
    expect(kind.dataset.kind).toBe("bug");
  });

  it("ステップがあれば「2/3」の内訳を出し、なければ出さない", () => {
    const { container } = renderRow({ steps: { done: 2, total: 3 } });
    expect(container.querySelector('[title="ステップ"]')?.textContent?.trim()).toBe("2/3");
    cleanup();
    expect(renderRow({}).container.querySelector('[title="ステップ"]')).toBeNull();
  });
});
