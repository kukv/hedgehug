import {
  draggable,
  dropTargetForElements,
  monitorForElements,
} from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import { autoScrollForElements } from "@atlaskit/pragmatic-drag-and-drop-auto-scroll/element";
import type { Attachment } from "svelte/attachments";

const CARD = "kanban-card";

// A card being dragged: its task, and the row it was taken from ("" without swimlanes).
export type DragSource = { ulid: string; lane: string };

function sourceOf(data: Record<string | symbol, unknown>): DragSource {
  return { ulid: String(data.ulid), lane: String(data.lane) };
}

// locked: the card cannot be moved (its Store is read-only), so it is not made draggable at all.
export type CardDndOptions = DragSource & { onDragStart: () => void; locked?: boolean };

// cardDnd makes a card draggable. Use it as {@attach cardDnd(get)}. The options are read when
// needed, so one attachment serves the card for as long as it lives.
export function cardDnd(get: () => CardDndOptions): Attachment<HTMLElement> {
  return (el) => {
    if (get().locked) return;
    return draggable({
      element: el,
      getInitialData: () => ({ type: CARD, ulid: get().ulid, lane: get().lane }),
      onDragStart: () => get().onDragStart(),
    });
  };
}

export type CellDndOptions = {
  // The card that is over the cell, or null when it leaves.
  onOver: (source: DragSource | null) => void;
  onDrop: (source: DragSource) => void;
};

// cellDnd makes a cell of the board (a column in a row) a place cards are dropped on.
export function cellDnd(get: () => CellDndOptions): Attachment<HTMLElement> {
  return (el) =>
    dropTargetForElements({
      element: el,
      canDrop: ({ source }) => source.data.type === CARD,
      onDragEnter: ({ source }) => get().onOver(sourceOf(source.data)),
      onDragLeave: () => get().onOver(null),
      onDrop: ({ source }) => {
        get().onOver(null);
        get().onDrop(sourceOf(source.data));
      },
    });
}

// boardDragEnd calls onEnd when the drag of a card ends, wherever it ends: dropped outside the
// board, cancelled, or with the dragged card gone from the screen.
export function boardDragEnd(onEnd: () => void): Attachment<HTMLElement> {
  return () =>
    monitorForElements({
      canMonitor: ({ source }) => source.data.type === CARD,
      onDrop: onEnd,
    });
}

// boardAutoScroll scrolls the board when a drag nears its edge.
export function boardAutoScroll(): Attachment<HTMLElement> {
  return (el) => autoScrollForElements({ element: el });
}
