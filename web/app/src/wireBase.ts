import { onRestored } from "@hedgehug/api";
import {
  createToasts,
  restoredCommentToast,
  restoredNameToast,
  restoredToast,
} from "@hedgehug/core";

// wire.ts が 300 行の上限に近いので、ほかの組み立てが先に使うものをここに分けた(wire.ts は toasts を再公開する)。

export const timer = {
  set: (fn: () => void, ms: number) => setTimeout(fn, ms),
  clear: (handle: unknown) => clearTimeout(handle as ReturnType<typeof setTimeout>),
};
export const toasts = createToasts(timer);

// 別の画面で削除されていたものへの保存で削除を取り消したとき、取り消したものごとに通常のトーストを出す。
onRestored((restored) => {
  for (const { kind, text } of restored) {
    if (kind === "name") toasts.show(restoredNameToast(text));
    else if (kind === "comment") toasts.show(restoredCommentToast());
    else toasts.show(restoredToast(text));
  }
});
