import type { BacklogFrame } from "../../model/planning";

// バックログの画面の並べ替えの保存の中身(05「並び順」、F2a-3・F2a-8・F2a-12)と、消えるときのトースト(F2a-10)。

// Drop は、placeItem に渡す保存の中身。sprint は移動先の枠(null はバックログ)、anchor は基準の行
// (null は枠の末尾)。after が true なら基準の直後、false なら直前。
export interface Drop {
  sprint: string | null;
  anchor: string | null;
  after: boolean;
}

const keyOf = (f: BacklogFrame): string | null => f.sprint?.id ?? null;

// locate は、アイテムの行がある枠と、枠の中の位置。画面にない行なら null。
function locate(frames: BacklogFrame[], itemId: string) {
  for (const frame of frames) {
    const index = frame.rows.findIndex((r) => r.id === itemId);
    if (index >= 0) return { frame, index };
  }
  return null;
}

// dropPlacement は、ドロップ先から保存の中身を決める。target.frame はスプリントの ID(バックログは null)、
// target.row は行の内部 ID(枠の空いた部分は null)。行の上ならその行の直前、空いた部分なら枠の末尾。
// 同じ位置(自分自身、同じ枠の自分の次の行、同じ枠の空いた部分で自分が末尾)と、画面にない枠・行は null。
export function dropPlacement(
  frames: BacklogFrame[],
  itemId: string,
  target: { frame: string | null; row: string | null },
): Drop | null {
  const dest = frames.find((f) => keyOf(f) === target.frame);
  const here = locate(frames, itemId);
  if (!dest || !here) return null;
  const same = here.frame === dest;
  if (target.row === null) {
    if (same && here.index === dest.rows.length - 1) return null;
    return { sprint: target.frame, anchor: null, after: false };
  }
  const at = dest.rows.findIndex((r) => r.id === target.row);
  if (at < 0 || (same && (at === here.index || at === here.index + 1))) return null;
  return { sprint: target.frame, anchor: target.row, after: false };
}

// moveUp は、同じ枠の前の行の直前。先頭なら null。
export function moveUp(frames: BacklogFrame[], itemId: string): Drop | null {
  const here = locate(frames, itemId);
  const prev = here?.frame.rows[here.index - 1];
  if (!here || !prev) return null;
  return { sprint: keyOf(here.frame), anchor: prev.id, after: false };
}

// moveDown は、同じ枠の次の行の直後。末尾なら null。
export function moveDown(frames: BacklogFrame[], itemId: string): Drop | null {
  const here = locate(frames, itemId);
  const next = here?.frame.rows[here.index + 1];
  if (!here || !next) return null;
  return { sprint: keyOf(here.frame), anchor: next.id, after: true };
}

// goneText は、利用者の操作でアイテムがバックログの枠から消えるときのトースト(05「並びと表示」)。
export const goneText = (display: string, category: "done" | "canceled"): string =>
  `${display} は${category === "done" ? "完了" : "中止"}のため、バックログには表示されません`;
