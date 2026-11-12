import { get, type Readable, readonly, writable } from "../../reactive/store";

// トースト(01「画面の移り方と、共通の表示」): 同時に 4 件まで。5 件目が出ると古いものから消える。
// 通常は 4.5 秒、エラーは 7 秒で消え、「×」(dismiss)でも閉じられる。

export interface ToastItem {
  id: string;
  text: string;
  error: boolean;
}

// Timer は、時間が来たら呼ぶ仕組み。core は DOM の setTimeout を知らないので、app が渡す。
export interface Timer {
  set(fn: () => void, ms: number): unknown;
  clear(handle: unknown): void;
}

export interface Toasts {
  list: Readable<ToastItem[]>;
  show(text: string, opts?: { error?: boolean }): void;
  dismiss(id: string): void;
}

export const maxToasts = 4;
export const toastMs = 4500;
export const errorToastMs = 7000;

export function createToasts(timer: Timer): Toasts {
  const list = writable<ToastItem[]>([]);
  const handles = new Map<string, unknown>();
  let next = 0;

  function dismiss(id: string) {
    const handle = handles.get(id);
    if (handle === undefined) return;
    timer.clear(handle);
    handles.delete(id);
    list.update((items) => items.filter((t) => t.id !== id));
  }

  return {
    list: readonly(list),
    show(text, opts = {}) {
      next += 1;
      const toast: ToastItem = { id: `t${next}`, text, error: opts.error ?? false };
      handles.set(
        toast.id,
        timer.set(() => dismiss(toast.id), toast.error ? errorToastMs : toastMs),
      );
      list.update((items) => [...items, toast]);
      const items = get(list);
      for (const old of items.slice(0, Math.max(0, items.length - maxToasts))) dismiss(old.id);
    },
    dismiss,
  };
}
