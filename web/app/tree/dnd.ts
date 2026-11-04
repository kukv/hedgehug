import { combine } from "@atlaskit/pragmatic-drag-and-drop/combine";
import {
  draggable,
  dropTargetForElements,
  monitorForElements,
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
  // The Store that cannot be written (the one of the place or of the dragged task) and why (in
  // Japanese), or null/absent when both can.
  readOnly?: { store: string; why: string } | null;
  // The ID of a task as the screen writes it (with its Store where the prefix is shared).
  idOf?: (t: TaskView) => string;
};

// silent: nothing to do and nothing to say (a drop on itself). A move to where
// the task already is is not judged here: the screen may list only some of the
// siblings, and the server answers such a move without an error.
export type DropDecision =
  | { ok: true; to: MoveArgs }
  | { ok: false; silent: boolean; title: string; reason: string };

const silent: DropDecision = { ok: false, silent: true, title: "", reason: "" };

function refuse(title: string, reason: string): DropDecision {
  return { ok: false, silent: false, title, reason };
}

function name(t: TaskView, idOf: (t: TaskView) => string): string {
  return `「${idOf(t)} ${t.title}」`;
}

// dropDecision says whether dragged may go to the place next to target, and
// the move to make when it may. It reads no DOM; the checks come in the order
// the reasons are worth giving in.
export function dropDecision(i: DropInput): DropDecision {
  const { dragged, target, place } = i;
  const idOf = i.idOf ?? ((t: TaskView) => t.id);
  if (dragged.ulid === target.ulid) return silent;

  if (i.readOnly) {
    return refuse(
      "閲覧のみの Store には置けません",
      `${i.readOnly.store} は閲覧のみです（${i.readOnly.why}）。`,
    );
  }

  if (dragged.store !== target.store) {
    return refuse(
      place === "child"
        ? "別の Store のタスクの下には置けません"
        : "別の Store の並びには置けません",
      `${name(dragged, idOf)}は ${dragged.store}、${name(target, idOf)}は ${target.store} にあります。Store をまたぐ移動はドラッグではできません。`,
    );
  }
  if (dragged.workspace.ulid !== target.workspace.ulid) {
    return refuse(
      "別のワークスペースには置けません",
      `${name(dragged, idOf)}は ${dragged.workspace.name}、${name(target, idOf)}は ${target.workspace.name} にあります。ワークスペースをまたぐ移動はドラッグではできません（スプリントやリリースが外れるため）。`,
    );
  }

  const parentUlid = place === "child" ? target.ulid : target.parent;
  let up: string | null = parentUlid;
  for (let n = 0; up && n < 10000; n++) {
    if (up === dragged.ulid) {
      return refuse(
        "自分の子孫の下には置けません",
        `${name(dragged, idOf)}の下の階層に置くことになり、親子が循環します。`,
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
  const rejection = checkParent(i.schema, dragged, parent ?? null, idOf);
  if (rejection) return refuse(rejection.title, rejection.reason);

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
// modes used here.
const INDENT = 26;

export type RowDndOptions = {
  ulid: string;
  depth: number;
  // The row has children and shows them: its lower part makes a child, as its
  // middle does (a task put after it would land below its children).
  openChildren: boolean;
  onDragStart: () => void;
  // The row cannot be moved (its Store is read-only): it is not draggable, but it still takes drops
  // (which are turned down).
  locked?: boolean;
  // The place a drop on this row would mean and the task being dragged; both
  // null when the pointer leaves.
  onOver: (place: DropPlace | null, sourceUlid: string | null) => void;
  onDrop: (sourceUlid: string, place: DropPlace) => void;
};

// rowDnd makes a row draggable and a drop target. Use it as {@attach rowDnd(get)}.
// The options are read when needed, so one attachment serves the row for as
// long as it lives, whatever the options become.
export function rowDnd(get: () => RowDndOptions): Attachment<HTMLElement> {
  return (el) =>
    combine(
      ...(get().locked
        ? []
        : [
            draggable({
              element: el,
              getInitialData: () => ({ type: ROW, ulid: get().ulid }),
              onDragStart: () => get().onDragStart(),
            }),
          ]),
      dropTargetForElements({
        element: el,
        canDrop: ({ source }) => source.data.type === ROW,
        getData: ({ input, element }) =>
          attachInstruction(
            { type: ROW, ulid: get().ulid },
            {
              input,
              element,
              currentLevel: get().depth,
              indentPerLevel: INDENT,
              mode: get().openChildren ? "expanded" : "standard",
            },
          ),
        onDrag: ({ self, source }) =>
          get().onOver(placeOf(extractInstruction(self.data)), String(source.data.ulid)),
        onDragLeave: () => get().onOver(null, null),
        onDrop: ({ self, source }) => {
          const place = placeOf(extractInstruction(self.data));
          get().onOver(null, null);
          if (place) get().onDrop(String(source.data.ulid), place);
        },
      }),
    );
}

// treeDragEnd calls onEnd when the drag of a row ends, wherever it ends: dropped
// outside the tree, cancelled, or with the dragged row gone from a long list.
export function treeDragEnd(onEnd: () => void): Attachment<HTMLElement> {
  return () =>
    monitorForElements({
      canMonitor: ({ source }) => source.data.type === ROW,
      onDrop: onEnd,
    });
}

// treeAutoScroll scrolls the list when a drag nears its edge.
export function treeAutoScroll(): Attachment<HTMLElement> {
  return (el) => autoScrollForElements({ element: el });
}
