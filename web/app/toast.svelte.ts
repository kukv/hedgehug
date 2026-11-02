import { errorMessage } from "../core/messages";

export type ToastInput = {
  message: string;
  undo?: () => void | Promise<void>;
  link?: { label: string; href: string };
};
export type ToastItem = ToastInput & { id: number };

const SHORT_MS = 6000;
const UNDO_MS = 10000;

export const toasts = $state<{ list: ToastItem[] }>({ list: [] });

let seq = 0;
const timers = new Map<number, ReturnType<typeof setTimeout>>();

export function toast(t: ToastInput): void {
  const id = ++seq;
  toasts.list = [...toasts.list, { ...t, id }];
  timers.set(
    id,
    setTimeout(() => dismissToast(id), t.undo ? UNDO_MS : SHORT_MS),
  );
}

export function dismissToast(id: number): void {
  clearTimeout(timers.get(id));
  timers.delete(id);
  toasts.list = toasts.list.filter((t) => t.id !== id);
}

/** Runs the undo of a toast and closes it; a failure becomes a toast of its own. */
export async function undoToast(id: number): Promise<void> {
  const t = toasts.list.find((x) => x.id === id);
  dismissToast(id);
  if (!t?.undo) return;
  try {
    await t.undo();
  } catch (e) {
    toast({ message: errorMessage(e) });
  }
}

/** For tests: drops every toast and timer. */
export function resetToasts(): void {
  for (const t of timers.values()) clearTimeout(t);
  timers.clear();
  toasts.list = [];
}
