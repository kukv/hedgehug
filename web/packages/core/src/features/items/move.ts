import { NotFound, Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import type { MoveCandidate } from "../../model/itemAction";
import type { ItemActionsPort } from "../../port/itemActions";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import type { Toasts } from "../toasts/toasts";
import { filterMoveCandidates } from "./moveCandidates";
import type { ItemRefresh } from "./refresh";

// 「移動…」の候補のメニュー(02「同じワークスペースの中での移動」)。メニューを開いたら候補を読み、検索語で絞り、
// 選んだら移す。移したあとはトーストを出し、ツリーと詳細パネルを読み直す(応答のアイテムは使わない)。
// 移動に確認ダイアログは挟まず、選んだらすぐ移す。

// 断りでも「ない」でもない失敗のときの文言。
export const moveFailedMessage = "移動できませんでした";
// トーストは、プロジェクトが変わるときは移し先のプロジェクト名、変わらないときは新しい親の ID(A4b-13)。
export const movedToProjectMessage = (projectTitle: string) => `${projectTitle} に移動しました`;
export const movedUnderMessage = (display: string) => `${display} の下に移動しました`;

export interface MoveMenuState {
  item: Item;
  // x・y は、メニューを出す位置。
  x: number;
  y: number;
  // loading は、候補を読んでいる間。
  loading: boolean;
  query: string;
  // shown は、検索語で絞った候補(読み込み中は空)。
  shown: MoveCandidate[];
}

export interface ItemMoveDeps {
  actions: Pick<ItemActionsPort, "moveCandidates" | "move">;
  catalog: Catalog;
  toasts: Toasts;
  refresh: Pick<ItemRefresh, "afterChange">;
}

export interface ItemMove {
  state: Readable<MoveMenuState | null>;
  // open は、候補のメニューを (x, y) に出して候補を読む。閲覧のみの Store では何もしない。
  open(item: Item, x: number, y: number): Promise<void>;
  setQuery(query: string): void;
  // choose は、メニューを閉じて、候補の下(Epic ならプロジェクトの直下)へ移す。
  choose(candidate: MoveCandidate): Promise<void>;
  // moveTo は、メニューを介さずに item を候補の下へ移す(ドラッグの代わりの経路など)。移せたら true。
  moveTo(item: Item, candidate: MoveCandidate): Promise<boolean>;
  close(): void;
}

// toastOf は、移したあとのトースト(A4b-13)。プロジェクトが変わるかは、候補のプロジェクトの内部 ID と、
// 移すアイテムのいまのプロジェクトの内部 ID で比べる。プロジェクトの候補は、いつもプロジェクト名の文。
function toastOf(item: Item, c: MoveCandidate): string {
  if (c.kind === "project") return movedToProjectMessage(c.title);
  return c.projectId === item.projectId
    ? movedUnderMessage(c.display)
    : movedToProjectMessage(c.projectTitle);
}

export function createItemMove(deps: ItemMoveDeps): ItemMove {
  const state = writable<MoveMenuState | null>(null);
  // all は、読んだ候補(絞る前)。seq は、開く・閉じるの世代(読んでいる間に閉じたら、古い結果を捨てる)。
  let all: MoveCandidate[] = [];
  let seq = 0;
  // busy は、移している間。そのあいだの移動は受けない。
  let busy = false;

  const failed = (e: unknown) =>
    deps.toasts.show(e instanceof Refused ? e.message : moveFailedMessage, { error: true });

  async function moveTo(item: Item, c: MoveCandidate): Promise<boolean> {
    if (busy || deps.catalog.store(item.storeId)?.readOnly) return false;
    busy = true;
    try {
      const to = c.kind === "project" ? { project: c.id } : { parent: c.id };
      await deps.actions.move(item.storeId, item.id, to);
      deps.toasts.show(toastOf(item, c));
    } catch (e) {
      failed(e);
      // 移すアイテムか移し先が別の経路で消えていたら、画面を読み直して消す。
      if (e instanceof NotFound) await deps.refresh.afterChange();
      return false;
    } finally {
      busy = false;
    }
    await deps.refresh.afterChange();
    return true;
  }

  return {
    state: readonly(state),
    async open(item, x, y) {
      if (deps.catalog.store(item.storeId)?.readOnly) return;
      const gen = ++seq;
      all = [];
      state.set({ item, x, y, loading: true, query: "", shown: [] });
      let loaded: MoveCandidate[];
      try {
        loaded = await deps.actions.moveCandidates(item.storeId, item.id);
      } catch (e) {
        if (gen !== seq) return;
        state.set(null);
        // すでに消えているアイテムは、画面を読み直して消す。
        if (e instanceof NotFound) await deps.refresh.afterChange();
        else failed(e);
        return;
      }
      // 古い読み込みの結果は、新しい候補を上書きしないよう捨てる。
      if (gen !== seq) return;
      all = loaded;
      const s = get(state);
      if (!s) return;
      state.set({ ...s, loading: false, shown: filterMoveCandidates(all, s.query) });
    },
    setQuery(query) {
      const s = get(state);
      if (s) state.set({ ...s, query, shown: filterMoveCandidates(all, query) });
    },
    async choose(c) {
      const s = get(state);
      if (!s || s.loading || busy) return;
      seq += 1;
      state.set(null);
      await moveTo(s.item, c);
    },
    moveTo,
    close() {
      seq += 1;
      state.set(null);
    },
  };
}
