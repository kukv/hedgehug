import { extractInstruction } from "@atlaskit/pragmatic-drag-and-drop-hitbox/tree-item";
import { beforeEach, describe, expect, it, vi } from "vitest";

type Target = {
  getData: (a: { input: unknown; element: Element }) => Record<string | symbol, unknown>;
  onDrop: (a: { self: { data: Record<string | symbol, unknown> }; source: unknown }) => void;
};
const h = vi.hoisted(() => ({ target: undefined as unknown, drag: undefined as unknown }));

vi.mock("@atlaskit/pragmatic-drag-and-drop/element/adapter", () => ({
  draggable: (o: unknown) => {
    h.drag = o;
    return () => {};
  },
  dropTargetForElements: (o: unknown) => {
    h.target = o;
    return () => {};
  },
  monitorForElements: () => () => {},
}));

import { type RowDndOptions, rowDnd } from "./dnd";

// A row 40px tall at the top of the page.
const el = {
  getBoundingClientRect: () => ({
    top: 0,
    bottom: 40,
    left: 0,
    right: 100,
    height: 40,
    width: 100,
  }),
} as unknown as HTMLElement;

function placeAt(y: number, openChildren: boolean) {
  const opts: RowDndOptions = {
    ulid: "R",
    depth: 0,
    openChildren,
    onDragStart() {},
    onOver() {},
    onDrop() {},
  };
  rowDnd(() => opts)(el);
  const t = h.target as Target;
  const data = t.getData({ input: { clientX: 10, clientY: y }, element: el });
  return extractInstruction(data)?.type;
}

beforeEach(() => {
  h.target = undefined;
});

describe("rowDnd: where on the row", () => {
  it("splits a closed row in quarters: before, child, after", () => {
    expect(placeAt(5, false)).toBe("reorder-above");
    expect(placeAt(20, false)).toBe("make-child");
    expect(placeAt(35, false)).toBe("reorder-below");
  });
  it("makes the lower part of an open row with children a child too", () => {
    expect(placeAt(5, true)).toBe("reorder-above");
    expect(placeAt(20, true)).toBe("make-child");
    expect(placeAt(35, true)).toBe("make-child");
  });
});

describe("rowDnd: the options are read when needed", () => {
  it("follows the row it is given later, with one attachment", () => {
    let opts: RowDndOptions = {
      ulid: "A",
      depth: 0,
      openChildren: false,
      onDragStart() {},
      onOver() {},
      onDrop: vi.fn(),
    };
    rowDnd(() => opts)(el);
    const t = h.target as Target;
    const seen = (h.drag as { getInitialData: () => { ulid: string } }).getInitialData().ulid;
    expect(seen).toBe("A");
    const drop = vi.fn();
    opts = { ...opts, ulid: "B", onDrop: drop };
    expect((h.drag as { getInitialData: () => { ulid: string } }).getInitialData().ulid).toBe("B");
    const self = {
      data: t.getData({ input: { clientX: 10, clientY: 20 }, element: el }),
    };
    t.onDrop({ self, source: { data: { ulid: "S" } } });
    expect(drop).toHaveBeenCalledWith("S", "child");
  });
});
