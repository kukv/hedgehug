// 行の持ち手のドラッグ(Svelte の action。01「ドラッグ」、05「並び順」)。マウスもタッチも Pointer Events で動く。
// 持ち手の pointerdown で始め、pointermove で指の下の行(data-row)と枠(data-frame)を読み、pointerup で
// ドロップする。pointercancel と Esc ではドロップせずにやめる。持ち手には CSS の touch-action: none を付ける。

// DropTarget は、ドロップ先。frame は枠の data-frame(空文字は null)、row は行の data-row(枠の空いた部分は null)。
export interface DropTarget {
  frame: string | null;
  row: string | null;
}

export interface DragRowsOptions {
  itemId: string;
  // disabled なら始めない。ドラッグ中に disabled になったら、ドロップせずにやめる。
  disabled: boolean;
  ondrop(target: DropTarget): void;
  // onhover は、指の下のドロップ先が変わるたびに呼ぶ。枠の外とやめたときは null。
  onhover(target: DropTarget | null): void;
}

// targetAt は、座標の下のドロップ先。枠の外なら null。
function targetAt(x: number, y: number): DropTarget | null {
  const el = document.elementFromPoint(x, y);
  const frame = el?.closest<HTMLElement>("[data-frame]");
  if (!el || !frame) return null;
  const row = el.closest<HTMLElement>("[data-row]");
  return {
    frame: frame.dataset.frame || null,
    row: row && frame.contains(row) ? (row.dataset.row ?? null) : null,
  };
}

export function dragRows(handle: HTMLElement, options: DragRowsOptions) {
  let opts = options;
  // pointer は、ドラッグ中のポインターの ID。ドラッグしていなければ null。
  let pointer: number | null = null;
  let target: DropTarget | null = null;

  function move(event: PointerEvent) {
    if (event.pointerId !== pointer) return;
    target = targetAt(event.clientX, event.clientY);
    opts.onhover(target);
  }
  function stop(drop: boolean) {
    if (pointer === null) return;
    handle.removeEventListener("pointermove", move);
    handle.removeEventListener("pointerup", up);
    handle.removeEventListener("pointercancel", cancel);
    window.removeEventListener("keydown", key);
    if (handle.hasPointerCapture?.(pointer)) handle.releasePointerCapture(pointer);
    const dropped = target;
    pointer = null;
    target = null;
    opts.onhover(null);
    if (drop && dropped) opts.ondrop(dropped);
  }
  function up(event: PointerEvent) {
    if (event.pointerId === pointer) stop(true);
  }
  function cancel(event: PointerEvent) {
    if (event.pointerId === pointer) stop(false);
  }
  function key(event: KeyboardEvent) {
    if (event.key === "Escape") stop(false);
  }
  function down(event: PointerEvent) {
    if (opts.disabled || pointer !== null || event.button !== 0) return;
    event.preventDefault();
    pointer = event.pointerId;
    handle.setPointerCapture?.(event.pointerId);
    handle.addEventListener("pointermove", move);
    handle.addEventListener("pointerup", up);
    handle.addEventListener("pointercancel", cancel);
    window.addEventListener("keydown", key);
  }

  handle.addEventListener("pointerdown", down);
  return {
    update(next: DragRowsOptions) {
      opts = next;
      if (opts.disabled) stop(false);
    },
    destroy() {
      stop(false);
      handle.removeEventListener("pointerdown", down);
    },
  };
}
