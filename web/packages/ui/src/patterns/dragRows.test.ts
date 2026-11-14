import { fireEvent } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { dragRows } from "./dragRows";

// 画面: スプリントの枠 s1 に行 r1・r2、バックログの枠(data-frame="")に行 r3。持ち手は r1 の中。
function page() {
  document.body.innerHTML = `
    <section data-frame="s1">
      <ul>
        <li data-row="r1"><button class="handle">持ち手</button><span class="r1-title">一</span></li>
        <li data-row="r2"><span class="r2-title">二</span></li>
      </ul>
      <p class="s1-empty">空いた部分</p>
    </section>
    <section data-frame="">
      <ul><li data-row="r3"><span class="r3-title">三</span></li></ul>
      <p class="backlog-empty">空いた部分</p>
    </section>
    <p class="outside">枠の外</p>`;
  const q = (s: string) => document.querySelector(s) as HTMLElement;
  return { handle: q(".handle"), q };
}

// pointAt は、elementFromPoint が返す要素を差し替える(jsdom は座標から要素を探せない)。
function pointAt(el: Element | null) {
  document.elementFromPoint = () => el;
}

function setup(disabled = false) {
  const { handle, q } = page();
  const ondrop = vi.fn();
  const onhover = vi.fn();
  const action = dragRows(handle, { itemId: "r1", disabled, ondrop, onhover });
  return { handle, q, ondrop, onhover, action };
}

beforeEach(() => {
  HTMLElement.prototype.setPointerCapture = vi.fn();
  HTMLElement.prototype.releasePointerCapture = vi.fn();
});

afterEach(() => {
  document.body.innerHTML = "";
});

describe("dragRows", () => {
  it("drops on the row under the pointer, with its frame", async () => {
    const { handle, q, ondrop, onhover } = setup();
    await fireEvent.pointerDown(handle, { pointerId: 1, button: 0 });
    expect(handle.setPointerCapture).toHaveBeenCalledWith(1);
    pointAt(q(".r2-title"));
    await fireEvent.pointerMove(handle, { pointerId: 1, clientX: 10, clientY: 20 });
    expect(onhover).toHaveBeenLastCalledWith({ frame: "s1", row: "r2" });
    await fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(ondrop).toHaveBeenCalledExactlyOnceWith({ frame: "s1", row: "r2" });
    expect(onhover).toHaveBeenLastCalledWith(null);
  });

  it("drops at the end of the backlog frame on its empty part", async () => {
    const { handle, q, ondrop } = setup();
    await fireEvent.pointerDown(handle, { pointerId: 1, button: 0 });
    pointAt(q(".backlog-empty"));
    await fireEvent.pointerMove(handle, { pointerId: 1 });
    await fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(ondrop).toHaveBeenCalledExactlyOnceWith({ frame: null, row: null });
  });

  it("drops at the end of a sprint frame on its empty part", async () => {
    const { handle, q, ondrop } = setup();
    await fireEvent.pointerDown(handle, { pointerId: 1, button: 0 });
    pointAt(q(".s1-empty"));
    await fireEvent.pointerMove(handle, { pointerId: 1 });
    await fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(ondrop).toHaveBeenCalledExactlyOnceWith({ frame: "s1", row: null });
  });

  it("tells the hover only when the target changes", async () => {
    const { handle, q, onhover } = setup();
    await fireEvent.pointerDown(handle, { pointerId: 1, button: 0 });
    pointAt(q(".r2-title"));
    await fireEvent.pointerMove(handle, { pointerId: 1 });
    await fireEvent.pointerMove(handle, { pointerId: 1 });
    pointAt(q(".s1-empty"));
    await fireEvent.pointerMove(handle, { pointerId: 1 });
    expect(onhover.mock.calls).toEqual([
      [{ frame: "s1", row: "r2" }],
      [{ frame: "s1", row: null }],
    ]);
  });

  it("does not drop outside any frame", async () => {
    const { handle, q, ondrop, onhover } = setup();
    await fireEvent.pointerDown(handle, { pointerId: 1, button: 0 });
    pointAt(q(".r2-title"));
    await fireEvent.pointerMove(handle, { pointerId: 1 });
    pointAt(q(".outside"));
    await fireEvent.pointerMove(handle, { pointerId: 1 });
    expect(onhover).toHaveBeenLastCalledWith(null);
    await fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(ondrop).not.toHaveBeenCalled();
  });

  it("stops without dropping on pointercancel", async () => {
    const { handle, q, ondrop, onhover } = setup();
    await fireEvent.pointerDown(handle, { pointerId: 1, button: 0 });
    pointAt(q(".r3-title"));
    await fireEvent.pointerMove(handle, { pointerId: 1 });
    await fireEvent.pointerCancel(handle, { pointerId: 1 });
    expect(onhover).toHaveBeenLastCalledWith(null);
    await fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(ondrop).not.toHaveBeenCalled();
  });

  it("stops without dropping on Escape", async () => {
    const { handle, q, ondrop, onhover } = setup();
    await fireEvent.pointerDown(handle, { pointerId: 1, button: 0 });
    pointAt(q(".r3-title"));
    await fireEvent.pointerMove(handle, { pointerId: 1 });
    await fireEvent.keyDown(window, { key: "Escape" });
    expect(onhover).toHaveBeenLastCalledWith(null);
    await fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(ondrop).not.toHaveBeenCalled();
  });

  it("does not start while disabled", async () => {
    const { handle, q, ondrop, onhover } = setup(true);
    await fireEvent.pointerDown(handle, { pointerId: 1, button: 0 });
    expect(handle.setPointerCapture).not.toHaveBeenCalled();
    pointAt(q(".r2-title"));
    await fireEvent.pointerMove(handle, { pointerId: 1 });
    await fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(onhover).not.toHaveBeenCalled();
    expect(ondrop).not.toHaveBeenCalled();
  });

  it("uses the options given by update, and stops a drag when it becomes disabled", async () => {
    const { handle, q, ondrop, onhover, action } = setup();
    const ondrop2 = vi.fn();
    action.update({ itemId: "r1", disabled: false, ondrop: ondrop2, onhover });
    await fireEvent.pointerDown(handle, { pointerId: 1, button: 0 });
    pointAt(q(".r2-title"));
    await fireEvent.pointerMove(handle, { pointerId: 1 });
    await fireEvent.pointerUp(handle, { pointerId: 1 });
    expect(ondrop).not.toHaveBeenCalled();
    expect(ondrop2).toHaveBeenCalledOnce();

    await fireEvent.pointerDown(handle, { pointerId: 2, button: 0 });
    await fireEvent.pointerMove(handle, { pointerId: 2 });
    action.update({ itemId: "r1", disabled: true, ondrop: ondrop2, onhover });
    expect(onhover).toHaveBeenLastCalledWith(null);
    await fireEvent.pointerUp(handle, { pointerId: 2 });
    expect(ondrop2).toHaveBeenCalledOnce();
  });

  it("stops a drag without dropping when destroyed", async () => {
    const { handle, q, ondrop, onhover, action } = setup();
    await fireEvent.pointerDown(handle, { pointerId: 1, button: 0 });
    pointAt(q(".r2-title"));
    await fireEvent.pointerMove(handle, { pointerId: 1 });
    action.destroy();
    expect(onhover).toHaveBeenLastCalledWith(null);
    await fireEvent.pointerUp(handle, { pointerId: 1 });
    await fireEvent.pointerDown(handle, { pointerId: 2, button: 0 });
    await fireEvent.pointerUp(handle, { pointerId: 2 });
    expect(ondrop).not.toHaveBeenCalled();
  });
});
