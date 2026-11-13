import type { HistoryPort } from "../../port/history";

// fakeHistory は、積んだパスを持つ履歴の偽物。back は、ブラウザの戻るを押したときの動き。
export function fakeHistory(start: string) {
  const stack = [start];
  let index = 0;
  const listeners = new Set<() => void>();
  const history: HistoryPort = {
    path: () => stack[index] ?? "/",
    push(path) {
      stack.splice(index + 1, stack.length, path);
      index = stack.length - 1;
    },
    replace(path) {
      stack[index] = path;
    },
    onPop(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
  const back = () => {
    index -= 1;
    for (const l of listeners) l();
  };
  return { history, stack, back };
}
