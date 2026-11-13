import { NotFound, Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import type { MoveCandidate, MovePreview } from "../../model/itemAction";
import type { ItemActionsPort } from "../../port/itemActions";
import type { WorkspacesPort } from "../../port/workspaces";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import { itemDisplayId } from "../router/route";
import type { Toasts } from "../toasts/toasts";
import { usesStatus } from "../workspaces/projectRules";
import { filterMoveCandidates } from "./moveCandidates";
import type { ItemRefresh } from "./refresh";

// 「移動…」の候補のメニュー(02「同じワークスペースの中での移動」)。メニューを開いたら候補を読み、検索語で絞り、
// 選んだら移す。移したあとはトーストを出し、ツリーと詳細パネルを読み直す(応答のアイテムは使わない)。
// 確認ダイアログは、プロジェクトが変わり、移し先が使わないステータスのアイテムがあるときだけ出す(B5a-20)。
// それ以外は、選んだらすぐ移す。

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

// MoveConfirmRow は、移し先が使わないステータス 1 つの置き換えの行。value は選んでいる置き換え先。
export interface MoveConfirmRow {
  status: string;
  label: string;
  options: { id: string; label: string }[];
  value: string;
}

// MoveConfirm は、「ステータスの置き換え」を含む移動の確認ダイアログ(02「同じワークスペースの中での移動」)。
export interface MoveConfirm {
  title: "移動";
  body: string;
  rows: MoveConfirmRow[];
}

export interface ItemMoveDeps {
  actions: Pick<ItemActionsPort, "moveCandidates" | "movePreview" | "move">;
  workspaces: Pick<WorkspacesPort, "get">;
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
  // moveTo は、メニューを介さずに item を候補の下へ移す(ツリーの Alt+Shift+↑↓)。移せたら true。
  // 閲覧のみの Store・移している最中は何もしない(false)。
  // プロジェクトが変わり、使わないステータスがあれば、確認ダイアログの答えを待つ(キャンセルなら false)。
  moveTo(item: Item, candidate: MoveCandidate): Promise<boolean>;
  close(): void;
  // moveConfirm は、出している確認ダイアログ(出していなければ null)。
  moveConfirm: Readable<MoveConfirm | null>;
  // confirmMove は、statuses(使わないステータス → 置き換え先)で移す。cancelMove は何もせず閉じる。
  confirmMove(statuses: Record<string, string>): void;
  cancelMove(): void;
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
  const confirm = writable<MoveConfirm | null>(null);
  // answer は、確認ダイアログの答えを待っている moveTo への返し口(置き換え。null ならキャンセル)。
  let answer: ((statuses: Record<string, string> | null) => void) | null = null;
  // all は、読んだ候補(絞る前)。seq は、開く・閉じるの世代(読んでいる間に閉じたら、古い結果を捨てる)。
  let all: MoveCandidate[] = [];
  let seq = 0;
  // busy は、移している間。そのあいだの移動は受けない。
  let busy = false;

  const failed = (e: unknown) =>
    deps.toasts.show(e instanceof Refused ? e.message : moveFailedMessage, { error: true });

  // askReplacements は、確認ダイアログを出し、置き換えの答え(キャンセルなら null)を待つ。
  async function askReplacements(
    item: Item,
    c: MoveCandidate,
    p: MovePreview,
  ): Promise<Record<string, string> | null> {
    const ws = await deps.workspaces.get(item.storeId, item.workspaceId);
    const prefix = deps.catalog.workspace(item.storeId, item.workspaceId)?.prefix ?? "";
    const name = (id: string) => ws.settings.statuses.find((v) => v.id === id)?.name ?? id;
    const options = ws.settings.statuses
      .filter((v) => usesStatus(ws.settings, c.projectId, v.id))
      .map((v) => ({ id: v.id, label: v.name }));
    const target = `${itemDisplayId(prefix, item.number)} ${item.title}`;
    const projectTitle = c.kind === "project" ? c.title : c.projectTitle;
    confirm.set({
      title: "移動",
      body:
        p.below > 0
          ? `${target} と、その下の ${p.below} 件を ${projectTitle} へ移動します。`
          : `${target} を移動します。`,
      rows: p.statuses.map((s) => ({
        status: s.status,
        label: `${name(s.status)}(${s.count} 件)`,
        options,
        value: s.default,
      })),
    });
    return new Promise((resolve) => {
      answer = resolve;
    });
  }

  async function moveTo(item: Item, c: MoveCandidate): Promise<boolean> {
    if (busy || deps.catalog.store(item.storeId)?.readOnly) return false;
    busy = true;
    try {
      const to = c.kind === "project" ? { project: c.id } : { parent: c.id };
      let statuses: Record<string, string> | undefined;
      // プロジェクトが変わるときだけ、移し先が使わないステータスを予告で読む。
      if (c.projectId !== item.projectId) {
        const preview = await deps.actions.movePreview(item.storeId, item.id, to);
        if (preview.statuses.length > 0) {
          const chosen = await askReplacements(item, c, preview);
          if (chosen === null) return false;
          statuses = chosen;
        }
      }
      await deps.actions.move(item.storeId, item.id, to, statuses);
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
    moveConfirm: readonly(confirm),
    confirmMove(statuses) {
      confirm.set(null);
      answer?.(statuses);
      answer = null;
    },
    cancelMove() {
      confirm.set(null);
      answer?.(null);
      answer = null;
    },
    close() {
      seq += 1;
      state.set(null);
    },
  };
}
