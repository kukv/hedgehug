import { Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import type { NotesPort } from "../../port/notes";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import { readOnlyNote } from "../catalog/readonly";
import type { Toasts } from "../toasts/toasts";

// ヘッダーのウォッチのボタン(02「ウォッチ」)。あなたがそのアイテムをウォッチ中かどうかを入り切りする。

export interface WatchView {
  watching: boolean;
  // label は、いまどちらかが分かる表示。
  label: string;
  disabled: boolean;
  // note は、無効にしたときの案内(閲覧のみの Store。ポインターを置いたときに出す)。
  note: string | undefined;
}

// watchView は、ボタンの表示。ログイン名は大文字と小文字を区別せずに比べる(01)。
// あなたが決まらないとき(me が空)は、ウォッチ中にならず、押せない。
export function watchView(item: Pick<Item, "watchers">, me: string, readOnly: boolean): WatchView {
  const watching = me !== "" && item.watchers.some((w) => w.toLowerCase() === me.toLowerCase());
  return {
    watching,
    label: watching ? "ウォッチ中" : "ウォッチ",
    disabled: readOnly || me === "",
    note: readOnly ? readOnlyNote : undefined,
  };
}

export interface Watch {
  // busy は、送信中(二重に押さない)。
  busy: Readable<boolean>;
  // toggle は、ウォッチの状態を逆にする。トーストは出さない。断られたときだけ、エラーのトーストを出す。
  toggle(item: Item, me: string): Promise<void>;
}

export interface WatchDeps {
  notes: Pick<NotesPort, "setWatch">;
  toasts: Toasts;
  // onChanged は、保存したあと(開いている詳細パネルを読み直す)。
  onChanged(): void;
}

export function createWatch(deps: WatchDeps): Watch {
  const busy = writable(false);
  return {
    busy: readonly(busy),
    async toggle(item, me) {
      if (get(busy) || me === "") return;
      const on = !watchView(item, me, false).watching;
      busy.set(true);
      try {
        await deps.notes.setWatch(item.storeId, item.id, on);
      } catch (e) {
        if (!(e instanceof Refused)) throw e;
        deps.toasts.show(e.message, { error: true });
        return;
      } finally {
        busy.set(false);
      }
      deps.onChanged();
    },
  };
}
