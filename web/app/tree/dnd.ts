import { combine } from "@atlaskit/pragmatic-drag-and-drop/combine";
import {
  draggable,
  dropTargetForElements,
} from "@atlaskit/pragmatic-drag-and-drop/element/adapter";
import { autoScrollForElements } from "@atlaskit/pragmatic-drag-and-drop-auto-scroll/element";
import {
  attachInstruction,
  extractInstruction,
  type Instruction,
} from "@atlaskit/pragmatic-drag-and-drop-hitbox/tree-item";
import type { Attachment } from "svelte/attachments";
import type { TaskView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";
import { checkParent, type MoveArgs } from "./placement";

export type DropPlace = "before" | "after" | "child";

export type DropInput = {
  dragged: TaskView;
  target: TaskView;
  place: DropPlace;
  schema: SchemaView | undefined;
  taskOf: (ulid: string) => TaskView | undefined;
  // The ULIDs of the target's siblings as listed, the target included (and the
  // dragged task, when it is one of them).
  siblings: string[];
};

// silent: nothing to do and nothing to say (a drop on itself, or where the
// task already is).
export type DropDecision =
  | { ok: true; to: MoveArgs }
  | { ok: false; silent: boolean; title: string; reason: string };

const silent: DropDecision = { ok: false, silent: true, title: "", reason: "" };

function refuse(title: string, reason: string): DropDecision {
  return { ok: false, silent: false, title, reason };
}

function name(t: TaskView): string {
  return `「${t.id} ${t.title}」`;
}

// dropDecision says whether dragged may go to the place next to target, and
// the move to make when it may. It reads no DOM; the checks come in the order
// the reasons are worth giving in.
export function dropDecision(i: DropInput): DropDecision {
  const { dragged, target, place } = i;
  if (dragged.ulid === target.ulid) return silent;

  if (dragged.store !== target.store) {
    return refuse(
      place === "child"
        ? "別の Store のタスクの下には置けません"
        : "別の Store の並びには置けません",
      `${name(dragged)}は ${dragged.store}、${name(target)}は ${target.store} にあります。Store をまたぐ移動はドラッグではできません。`,
    );
  }
  if (dragged.workspace.ulid !== target.workspace.ulid) {
    return refuse(
      "別のワークスペースには置けません",
      `${name(dragged)}は ${dragged.workspace.name}、${name(target)}は ${target.workspace.name} にあります。ワークスペースをまたぐ移動はドラッグではできません（スプリントやリリースが外れるため）。`,
    );
  }

  const parentUlid = place === "child" ? target.ulid : target.parent;
  let up: string | null = parentUlid;
  for (let n = 0; up && n < 10000; n++) {
    if (up === dragged.ulid) {
      return refuse(
        "自分の子孫の下には置けません",
        `${name(dragged)}の下の階層に置くことになり、親子が循環します。`,
      );
    }
    up = i.taskOf(up)?.parent ?? null;
  }

  const parent = parentUlid ? i.taskOf(parentUlid) : null;
  if (parentUlid && !parent) {
    return refuse("置き先の親を読み込めていません", "少し待ってからやり直してください。");
  }
  if (!i.schema) {
    return refuse(
      "少し待ってからやり直してください",
      `${dragged.store} の schema.yaml を読み込んでいます。`,
    );
  }
  const rejection = checkParent(i.schema, dragged, parent ?? null);
  if (rejection) return refuse(rejection.title, rejection.reason);

  if (place !== "child" && dragged.parent === target.parent) {
    const from = i.siblings.indexOf(dragged.ulid);
    const to = i.siblings.indexOf(target.ulid);
    if (from >= 0 && to >= 0 && (place === "before" ? from + 1 === to : from - 1 === to)) {
      return silent;
    }
  }

  if (place === "child") return { ok: true, to: { parent: target.ulid } };
  const next = place === "before" ? { before: target.ulid } : { after: target.ulid };
  return {
    ok: true,
    to: target.parent ? { parent: target.parent, ...next } : { root: true, ...next },
  };
}

// placeOf is the place the hitbox instruction means; the other instructions
// (blocked, reparent) mean no drop.
export function placeOf(ins: Instruction | null): DropPlace | null {
  switch (ins?.type) {
    case "reorder-above":
      return "before";
    case "reorder-below":
      return "after";
    case "make-child":
      return "child";
    default:
      return null;
  }
}

const ROW = "tree-row";
// One level of indentation, passed to the hitbox, which does not use it in the
// standard mode.
const INDENT = 26;

export type RowDndOptions = {
  ulid: string;
  depth: number;
  onDragStart: () => void;
  onDragEnd: () => void;
  // The place a drop on this row would mean and the task being dragged; both
  // null when the pointer leaves.
  onOver: (place: DropPlace | null, sourceUlid: string | null) => void;
  onDrop: (sourceUlid: string, place: DropPlace) => void;
};

// rowDnd makes a row draggable and a drop target. Use it as {@attach rowDnd(o)}.
export function rowDnd(o: RowDndOptions): Attachment<HTMLElement> {
  return (el) =>
    combine(
      draggable({
        element: el,
        getInitialData: () => ({ type: ROW, ulid: o.ulid }),
        onDragStart: o.onDragStart,
        onDrop: o.onDragEnd,
      }),
      dropTargetForElements({
        element: el,
        canDrop: ({ source }) => source.data.type === ROW,
        getData: ({ input, element }) =>
          attachInstruction(
            { type: ROW, ulid: o.ulid },
            { input, element, currentLevel: o.depth, indentPerLevel: INDENT, mode: "standard" },
          ),
        onDrag: ({ self, source }) =>
          o.onOver(placeOf(extractInstruction(self.data)), String(source.data.ulid)),
        onDragLeave: () => o.onOver(null, null),
        onDrop: ({ self, source }) => {
          const place = placeOf(extractInstruction(self.data));
          o.onOver(null, null);
          if (place) o.onDrop(String(source.data.ulid), place);
        },
      }),
    );
}

// treeAutoScroll scrolls the list when a drag nears its edge.
export function treeAutoScroll(): Attachment<HTMLElement> {
  return (el) => autoScrollForElements({ element: el });
}
