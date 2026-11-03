import { beforeEach, describe, expect, it, vi } from "vitest";

type Target = {
  canDrop: (a: { source: { data: Record<string, unknown> } }) => boolean;
  onDragEnter: (a: { source: { data: Record<string, unknown> } }) => void;
  onDragLeave: () => void;
  onDrop: (a: { source: { data: Record<string, unknown> } }) => void;
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

import { type CardDndOptions, type CellDndOptions, cardDnd, cellDnd } from "./dnd";

const el = {} as HTMLElement;
const card = { data: { type: "kanban-card", ulid: "A", lane: "gh/koki" } };

beforeEach(() => {
  h.target = undefined;
  h.drag = undefined;
});

describe("cardDnd", () => {
  it("carries the task and the row it was taken from, read when the drag starts", () => {
    const onDragStart = vi.fn();
    let opts: CardDndOptions = { ulid: "A", lane: "x", onDragStart };
    cardDnd(() => opts)(el);
    opts = { ulid: "B", lane: "y", onDragStart };
    const d = h.drag as { getInitialData: () => unknown; onDragStart: () => void };
    expect(d.getInitialData()).toEqual({ type: "kanban-card", ulid: "B", lane: "y" });
    d.onDragStart();
    expect(onDragStart).toHaveBeenCalled();
  });
});

describe("cellDnd", () => {
  function setup() {
    const opts: CellDndOptions = { onOver: vi.fn(), onDrop: vi.fn() };
    cellDnd(() => opts)(el);
    return { opts, target: h.target as Target };
  }

  it("takes cards only", () => {
    const { target } = setup();
    expect(target.canDrop({ source: card })).toBe(true);
    expect(target.canDrop({ source: { data: { type: "tree-row" } } })).toBe(false);
  });

  it("tells which card is over, and when it leaves", () => {
    const { opts, target } = setup();
    target.onDragEnter({ source: card });
    expect(opts.onOver).toHaveBeenLastCalledWith({ ulid: "A", lane: "gh/koki" });
    target.onDragLeave();
    expect(opts.onOver).toHaveBeenLastCalledWith(null);
  });

  it("clears the cell and drops the card", () => {
    const { opts, target } = setup();
    target.onDrop({ source: card });
    expect(opts.onOver).toHaveBeenCalledWith(null);
    expect(opts.onDrop).toHaveBeenCalledWith({ ulid: "A", lane: "gh/koki" });
  });
});
