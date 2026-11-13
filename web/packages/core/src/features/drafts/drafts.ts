import { type Readable, readonly, writable } from "../../reactive/store";

// 書きかけ(説明・コメントの入力欄の、保存していない入力)の登録簿(02「説明」、06「反映を待つとき」)。
// 入力欄が register で自分を登録する。app は anyHeld を反映を待つ条件に、confirmLeave を画面を離れる前の
// 確認に使う。

export const discardTitle = "入力を破棄";
export const discardOk = "破棄";
const commentLine = "保存していないコメントを破棄します。";
const descriptionLine = "保存していない説明の変更を破棄します。";

export interface DiscardPrompt {
  title: string;
  lines: string[];
  ok: string;
}

// LeaveGuard は、画面を離れてよいかを聞く側(ルーター)が使う部分。
export interface LeaveGuard {
  anyDirty(): boolean;
  confirmLeave(): Promise<boolean>;
}

export interface Drafts extends LeaveGuard {
  // register は、入力欄を登録し、外す関数を返す。key は "description:…" か "comment:…"(: より前の語で確認の
  // 本文を決める)。dirty は保存してあるものと違うか、held は違い、かつ空でないか(省略すると dirty と同じ)。
  register(key: string, dirty: () => boolean, held?: () => boolean): () => void;
  // anyHeld は、空でない書きかけがあるか(自動の同期と画面の反映を待つ条件)。
  anyHeld(): boolean;
  // confirmLeave は、違う入力があれば確認を出し、「破棄」なら true を返す。key を渡すと、その入力だけを見る。
  // 関数を渡すと、それが true を返す key の入力だけを見る(詳細パネルは isItemDraft)。
  confirmLeave(only?: string | ((key: string) => boolean)): Promise<boolean>;
  // prompt は、出している確認(なければ null)。answer は、そのボタン(true が「破棄」)。
  prompt: Readable<DiscardPrompt | null>;
  answer(discard: boolean): void;
}

interface Entry {
  dirty(): boolean;
  held(): boolean;
}

// isItemDraft は、アイテムの書きかけか(説明は "description:item/…"、コメントは "comment:new:item" と
// "comment:edit:item")。詳細パネルを閉じる・別のアイテムを開くときは、プロジェクトの概要の書きかけを見ない。
export const isItemDraft = (key: string) =>
  key.startsWith("description:item/") || /^comment:[a-z]+:item$/.test(key);

const lineOf = (key: string) => (key.split(":")[0] === "comment" ? commentLine : descriptionLine);

const picks = (only: string | ((key: string) => boolean) | undefined, key: string) =>
  only === undefined || (typeof only === "string" ? key === only : only(key));

export function createDrafts(): Drafts {
  const entries = new Map<string, Entry>();
  const prompt = writable<DiscardPrompt | null>(null);
  // waiting は、いま出している確認の答えを待つ関数。asking は、そのあいだに重ねて聞かれたときに返す同じ Promise。
  let waiting = null as ((discard: boolean) => void) | null;
  let asking: Promise<boolean> | null = null;
  const dirtyKeys = (only?: string | ((key: string) => boolean)) =>
    [...entries]
      .filter(([key, e]) => picks(only, key) && e.dirty())
      .map(([key]) => key);

  return {
    register(key, dirty, held = dirty) {
      const entry = { dirty, held };
      entries.set(key, entry);
      // 同じ key で登録し直したあとに、前の解除が走っても、新しい登録は消さない。
      return () => {
        if (entries.get(key) === entry) entries.delete(key);
      };
    },
    anyHeld: () => [...entries.values()].some((e) => e.held()),
    anyDirty: () => dirtyKeys().length > 0,
    confirmLeave(only) {
      const keys = dirtyKeys(only);
      if (keys.length === 0) return Promise.resolve(true);
      asking ??= new Promise<boolean>((resolve) => {
        waiting = resolve;
        const lines = [...new Set(keys.map(lineOf))];
        prompt.set({ title: discardTitle, lines, ok: discardOk });
      });
      return asking;
    },
    answer(discard) {
      const done = waiting;
      waiting = null;
      asking = null;
      prompt.set(null);
      done?.(discard);
    },
    prompt: readonly(prompt),
  };
}
