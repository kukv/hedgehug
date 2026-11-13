import type { MovedFrom } from "../../model/transfer";
import { type Readable, readonly, writable } from "../../reactive/store";
import type { Detail, DetailView } from "../items/detail";
import { movedLine } from "./movedText";

// 旧 ID から開いたときに詳細パネルの上部に出す 1 行(A5a-16)。普通に開いたときは出さない。
// show したアイテムがパネルに出たら文を出し、そのあと別のアイテムを開くかパネルを閉じたら消す。
// 出る前の閉じた状態(null)では消さない(アドレスの解決は、画面を移る購読がパネルを閉じたあとで開くので)。
// 出る前に別のアイテムが出たときは、その開き方に負けたので消す。

export interface MovedNotice {
  // text は、開いているパネルが show したアイテムのときだけ文、それ以外は空。
  text: Readable<string>;
  show(storeId: string, itemId: string, m: MovedFrom): void;
  // clear は、show したものを捨てる(パネルがそのアイテムを開けなかったとき。次に普通に開いても出さない)。
  clear(): void;
}

export interface MovedNoticeDeps {
  detail: Pick<Detail, "view">;
  now(): Date;
}

interface Entry {
  storeId: string;
  itemId: string;
  moved: MovedFrom;
  seen: boolean;
}

export function createMovedNotice(deps: MovedNoticeDeps): MovedNotice {
  const text = writable("");
  let entry: Entry | null = null;

  deps.detail.view.subscribe((v: DetailView | null) => {
    if (!entry) return;
    if (v && v.item.storeId === entry.storeId && v.item.id === entry.itemId) {
      entry.seen = true;
      text.set(movedLine(entry.moved, deps.now()));
      return;
    }
    if (!v && !entry.seen) return;
    entry = null;
    text.set("");
  });

  return {
    text: readonly(text),
    show(storeId, itemId, moved) {
      entry = { storeId, itemId, moved, seen: false };
      text.set("");
    },
    clear() {
      entry = null;
      text.set("");
    },
  };
}
