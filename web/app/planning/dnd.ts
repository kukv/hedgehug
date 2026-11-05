import {
  draggable,
  dropTargetForElements,
} from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import {
  attachClosestEdge,
  extractClosestEdge,
} from "@atlaskit/pragmatic-drag-and-drop-hitbox/closest-edge";
import type { Attachment } from "svelte/attachments";

const ROW = "plan-row";

export type Edge = "top" | "bottom";

// rowDrag makes a row draggable. locked: its Store is read-only, so it is not made draggable.
export function rowDrag(get: () => { ulid: string; locked: boolean }): Attachment<HTMLElement> {
  return (el) => {
    if (get().locked) return;
    return draggable({ element: el, getInitialData: () => ({ type: ROW, ulid: get().ulid }) });
  };
}

// rowDrop makes a row a place to drop on: before or after it, by the half the pointer is over.
export function rowDrop(
  get: () => {
    ulid: string;
    onOver: (edge: Edge | null) => void;
    onDrop: (dragged: string, edge: Edge) => void;
  },
): Attachment<HTMLElement> {
  return (el) =>
    dropTargetForElements({
      element: el,
      canDrop: ({ source }) => source.data.type === ROW && source.data.ulid !== get().ulid,
      getData: ({ input, element }) =>
        attachClosestEdge({ row: get().ulid }, { input, element, allowedEdges: ["top", "bottom"] }),
      onDrag: ({ self }) => get().onOver(extractClosestEdge(self.data) as Edge | null),
      onDragLeave: () => get().onOver(null),
      onDrop: ({ source, self }) => {
        get().onOver(null);
        get().onDrop(String(source.data.ulid), (extractClosestEdge(self.data) ?? "bottom") as Edge);
      },
    });
}

// listDrop makes a list, or a panel, a place to drop on. A row inside takes the drop first, so
// onDrop here is for what falls on the rest of it.
export function listDrop(
  get: () => { onOver: (over: boolean) => void; onDrop: (dragged: string, inner: boolean) => void },
): Attachment<HTMLElement> {
  return (el) =>
    dropTargetForElements({
      element: el,
      canDrop: ({ source }) => source.data.type === ROW,
      onDragEnter: () => get().onOver(true),
      onDragLeave: () => get().onOver(false),
      onDrop: ({ source, location }) => {
        get().onOver(false);
        get().onDrop(String(source.data.ulid), location.current.dropTargets[0]?.element !== el);
      },
    });
}
