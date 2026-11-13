import { isQueuedAction, type KeyInput, type TreeKeyAction } from "./treeKeys";

// キーの順番待ち(04「ツリーのキー操作」の最後の段落、A4b-21)。Alt+↑↓・Alt+Shift+↑↓ を続けて押したとき、押した回数だけ
// 1 回ずつ順に実行する。前の 1 回(保存と読み直し)が終わるまで次を始めない。途中で断られたとき、ほかのキーを押したとき、
// 画面を移ったときは、待っている分を捨てる(clear)。

export interface KeyQueue {
  // push は、1 回分の実行を待ちに足す。run は、成功なら true、断られたなら false を返す(投げても断られたことにする)。
  push(run: () => Promise<boolean>): void;
  // clear は、待っている分を捨てる。いま実行している 1 回は止めない。
  clear(): void;
  // pending は、待っている件数(いま実行している分は入れない)。
  pending(): number;
}

export function createKeyQueue(): KeyQueue {
  let waiting: Array<() => Promise<boolean>> = [];
  let running = false;

  async function drain() {
    running = true;
    try {
      for (let run = waiting.shift(); run; run = waiting.shift()) {
        const ok = await run().catch(() => false);
        if (!ok) waiting = [];
      }
    } finally {
      running = false;
    }
  }

  return {
    push(run) {
      waiting.push(run);
      if (!running) void drain();
    },
    clear() {
      waiting = [];
    },
    pending: () => waiting.length,
  };
}

const modifierKeys: ReadonlySet<string> = new Set(["Alt", "Shift", "Control", "Meta"]);

// keepsQueue は、このキーを押しても、待っている分を捨てなくてよいか。同じ種類のキー(Alt+↑↓・Alt+Shift+↑↓)と、
// Alt・Shift などの修飾キーだけの押下(Alt+Shift+↓ を押す途中で必ず起きる)は捨てない。それ以外はすべて「ほかのキー」。
export function keepsQueue(event: KeyInput, action: TreeKeyAction | null): boolean {
  return modifierKeys.has(event.key) || isQueuedAction(action);
}
