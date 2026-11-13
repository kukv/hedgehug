import { NotFound, Refused } from "../../model/errors";
import type { Item, Kind } from "../../model/item";
import type { ItemActionsPort } from "../../port/itemActions";
import type { ItemsPort } from "../../port/items";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import { itemDisplayId } from "../router/route";
import type { Toasts } from "../toasts/toasts";
import type { ItemRefresh } from "./refresh";
import { ancestorsOf } from "./tree";

// 「種類を変更…」(02「種類の変更」)。メニューの項目を選ぶと、選べる種類のメニュー(下位のメニュー)を出し、
// 種類を選ぶと確認ダイアログを出す(A4b-18)。実行したら、トーストは出さず、ツリーと詳細パネルを読み直す。

// 断りでも「ない」でもない失敗(通信の失敗など)のときの文言。
export const kindFailedMessage = "種類を変更できませんでした";

const swappable: Kind[] = ["Story", "Task", "Bug"];

// kindOptions は、from から選べる種類(02「種類の変更」の表)。Epic は変えられない。Story・Task・Bug は
// 自分以外の Story・Task・Bug、Step は Story・Task・Bug。Step へは変えられない。
export function kindOptions(from: Kind): Kind[] {
  return from === "Epic" ? [] : swappable.filter((k) => k !== from);
}

// kindDialog は、確認ダイアログの中身。Step から変えるときは、本文に「{親の親の ID} の下に移ります。」を続ける。
export function kindDialog(item: Item, displayId: string, to: Kind, movesUnder: string | null) {
  const move = movesUnder === null ? "" : `${movesUnder} の下に移ります。`;
  return {
    title: "種類を変更",
    lines: [`${displayId} の種類を ${item.kind} から ${to} に変更します。${move}`],
    ok: "変更",
  };
}

export type KindState =
  // menu は、選べる種類のメニュー。x・y は、出す位置。
  | { step: "menu"; item: Item; x: number; y: number; options: Kind[] }
  | { step: "confirm"; item: Item; to: Kind; title: string; lines: string[]; ok: string };

export interface ItemKindDeps {
  actions: Pick<ItemActionsPort, "changeKind">;
  // items は、Step の「親の親」を調べるために、プロジェクトのアイテムを読む。
  items: Pick<ItemsPort, "list">;
  catalog: Catalog;
  toasts: Toasts;
  refresh: Pick<ItemRefresh, "afterChange">;
}

export interface ItemKind {
  state: Readable<KindState | null>;
  // open は、種類のメニューを (x, y) に出す。閲覧のみの Store と Epic では何もしない(二重の守り)。
  open(item: Item, x: number, y: number): void;
  // choose は、メニューで種類を選ぶ。メニューを閉じて、確認ダイアログを出す(Step は親の親を読んでから)。
  choose(kind: Kind): Promise<void>;
  // confirm は、種類を変える。トーストは出さない(断りと失敗のときだけ出す)。
  confirm(): Promise<void>;
  cancel(): void;
}

export function createItemKind(deps: ItemKindDeps): ItemKind {
  const state = writable<KindState | null>(null);
  // seq は、開く・閉じるの世代(親の親を読んでいる間に閉じたら、古い結果を捨てる)。
  let seq = 0;
  // busy は、変更の実行中。そのあいだの confirm は受けない。
  let busy = false;

  const prefixOf = (item: Item) => deps.catalog.workspace(item.storeId, item.workspaceId)?.prefix;

  return {
    state: readonly(state),
    open(item, x, y) {
      const options = kindOptions(item.kind);
      if (options.length === 0 || deps.catalog.store(item.storeId)?.readOnly) return;
      seq += 1;
      state.set({ step: "menu", item, x, y, options });
    },
    async choose(kind) {
      const s = get(state);
      const prefix = s?.step === "menu" ? prefixOf(s.item) : undefined;
      if (s?.step !== "menu" || prefix === undefined || !s.options.includes(kind)) return;
      const gen = ++seq;
      state.set(null);
      let under: string | null = null;
      if (s.item.kind === "Step") {
        // 親の親(Epic)の ID。読めない・見つからないときは、画面が古いので、読み直して出さない。
        const all = await deps.items.list(s.item.storeId, s.item.projectId).catch(() => []);
        const grand = ancestorsOf(all, s.item.id).at(-2);
        if (gen !== seq) return;
        if (!grand) return void (await deps.refresh.afterChange());
        under = itemDisplayId(prefix, grand.number);
      }
      state.set({
        step: "confirm",
        item: s.item,
        to: kind,
        ...kindDialog(s.item, itemDisplayId(prefix, s.item.number), kind, under),
      });
    },
    async confirm() {
      const s = get(state);
      if (s?.step !== "confirm" || busy) return;
      busy = true;
      try {
        await deps.actions.changeKind(s.item.storeId, s.item.id, s.to);
        state.set(null);
      } catch (e) {
        state.set(null);
        // すでに消えているのは、画面を読み直して消す(トーストなし)。
        if (e instanceof NotFound) await deps.refresh.afterChange();
        else
          deps.toasts.show(e instanceof Refused ? e.message : kindFailedMessage, { error: true });
        return;
      } finally {
        busy = false;
      }
      await deps.refresh.afterChange();
    },
    cancel() {
      seq += 1;
      state.set(null);
    },
  };
}
