import { combine } from "@atlaskit/pragmatic-drag-and-drop/combine";
import {
  draggable,
  dropTargetForElements,
} from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import {
  attachClosestEdge,
  extractClosestEdge,
} from "@atlaskit/pragmatic-drag-and-drop-hitbox/closest-edge";
import { getReorderDestinationIndex } from "@atlaskit/pragmatic-drag-and-drop-hitbox/util/get-reorder-destination-index";

/** names with the item at from moved to to (indexes in names). Out of the list, nothing moves. */
export function moved(names: readonly string[], from: number, to: number): string[] {
  const out = [...names];
  const inside = (i: number) => i >= 0 && i < out.length;
  if (!inside(from) || !inside(to) || from === to) return out;
  out.splice(to, 0, ...out.splice(from, 1));
  return out;
}

type Opts = {
  list: string;
  name: string;
  index: number;
  onmove: (from: number, to: number) => void;
  // A list that cannot be written: nothing is dragged and nothing is dropped.
  disabled?: boolean;
};

// The drag handle of a row is the element with data-handle inside it.
export const HANDLE = "data-handle";

/** A Svelte action for a row: drag by its handle, drop on another row of the same list. */
export function sortableRow(node: HTMLElement, opts: Opts) {
  let cur = opts;
  const edge = (e: "top" | "bottom" | null) => {
    if (e) node.dataset.edge = e;
    else delete node.dataset.edge;
  };
  const stop = combine(
    draggable({
      element: node,
      dragHandle: node.querySelector<HTMLElement>(`[${HANDLE}]`) ?? undefined,
      canDrag: () => !cur.disabled,
      getInitialData: () => ({ list: cur.list, name: cur.name, index: cur.index }),
    }),
    dropTargetForElements({
      element: node,
      canDrop: ({ source }) => !cur.disabled && source.data.list === cur.list,
      getData: ({ input }) =>
        attachClosestEdge(
          { list: cur.list, name: cur.name, index: cur.index },
          { element: node, input, allowedEdges: ["top", "bottom"] },
        ),
      onDrag: ({ self, source }) => {
        const e = extractClosestEdge(self.data);
        edge(source.data.name === cur.name ? null : e === "top" || e === "bottom" ? e : null);
      },
      onDragLeave: () => edge(null),
      onDrop: ({ self, source }) => {
        edge(null);
        const from = source.data.index as number;
        const to = getReorderDestinationIndex({
          startIndex: from,
          indexOfTarget: cur.index,
          closestEdgeOfTarget: extractClosestEdge(self.data),
          axis: "vertical",
        });
        if (to !== from) cur.onmove(from, to);
      },
    }),
  );
  return {
    update(o: Opts) {
      cur = o;
    },
    destroy() {
      stop();
    },
  };
}
