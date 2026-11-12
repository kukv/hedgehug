import { NotFound, Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import type { ItemsPort } from "../../port/items";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog, ProjectNode, StoreNode, WorkspaceNode } from "../catalog/catalog";
import { itemDisplayId } from "../router/route";
import type { Toasts } from "../toasts/toasts";
import { ancestorsOf, compareSiblings } from "./tree";

// 詳細パネルの状態(02「詳細パネル」のうち A2 の分。A2 の計画の決定 18)。

export interface DetailView {
  store: StoreNode;
  workspace: WorkspaceNode;
  project: ProjectNode;
  item: Item;
  displayId: string;
  // ancestors は、パンくずに出す祖先(根から順)。
  ancestors: Item[];
  // children は、子(Epic なら Story・Task・Bug、Story・Task・Bug なら Step)を並び順で。
  children: Item[];
  readOnly: boolean;
}

export interface Detail {
  view: Readable<DetailView | null>;
  full: Readable<boolean>;
  open(storeId: string, itemId: string): Promise<void>;
  close(): void;
  toggleFull(): void;
  // rename は、タイトルを保存する。false なら、入力欄を元のタイトルに戻す。
  rename(title: string): Promise<boolean>;
  // addStep は、Step を末尾に作る。true なら、入力欄を空にする。
  addStep(title: string): Promise<boolean>;
  // moveUp は、Step を直前の Step と入れ替える。先頭なら何もしない。
  moveUp(itemId: string): Promise<void>;
}

export interface DetailDeps {
  catalog: Catalog;
  items: ItemsPort;
  toasts: Toasts;
  // onChanged は、保存したあと(開いているプロジェクトの画面を読み直す)。
  onChanged(): void;
}

// normalizeTitle は、連続する空白と改行を 1 つの空白にまとめ、前後の空白を取り除く(02「タイトル」)。
export function normalizeTitle(s: string): string {
  return s.split(/\s+/).filter(Boolean).join(" ");
}

export const copiedMessage = (id: string) => `コピーしました: ${id}`;
export const copyFailedMessage = (id: string) =>
  `コピーできませんでした。${id} を選択してコピーしてください`;

export function createDetail(deps: DetailDeps): Detail {
  const view = writable<DetailView | null>(null);
  const full = writable(false);
  let latest = 0;

  async function load(storeId: string, itemId: string) {
    latest += 1;
    const mine = latest;
    try {
      const item = await deps.items.get(storeId, itemId);
      const all = await deps.items.list(storeId, item.projectId);
      const store = deps.catalog.store(storeId);
      const project = deps.catalog.project(storeId, item.projectId);
      const workspace = project && deps.catalog.workspace(storeId, project.workspaceId);
      if (mine !== latest) return;
      if (!store || !project || !workspace) {
        view.set(null);
        return;
      }
      view.set({
        store,
        workspace,
        project,
        item,
        displayId: itemDisplayId(workspace.prefix, item.number),
        ancestors: ancestorsOf(all, item.id),
        children: all.filter((it) => it.parentId === item.id).sort(compareSiblings),
        readOnly: store.readOnly,
      });
    } catch (e) {
      // 開いているアイテムがなくなったら、パネルを閉じる(02「詳細パネル」)。
      if (!(e instanceof NotFound)) throw e;
      if (mine === latest) view.set(null);
    }
  }

  async function saved() {
    const v = get(view);
    if (v) await load(v.store.id, v.item.id);
    deps.onChanged();
  }

  // refusedToast は、断りをエラーのトーストで出す。断り以外は投げる。
  function refusedToast(e: unknown) {
    if (!(e instanceof Refused)) throw e;
    deps.toasts.show(e.message, { error: true });
  }

  return {
    view: readonly(view),
    full: readonly(full),
    open: load,
    close() {
      latest += 1;
      view.set(null);
      full.set(false);
    },
    toggleFull: () => full.update((f) => !f),
    async rename(title) {
      const v = get(view);
      if (!v) return false;
      const next = normalizeTitle(title);
      if (next === "") {
        deps.toasts.show("タイトルは空にできません", { error: true });
        return false;
      }
      if (next === v.item.title) return true;
      try {
        await deps.items.rename(v.store.id, v.item.id, next);
      } catch (e) {
        refusedToast(e);
        return false;
      }
      await saved();
      return true;
    },
    async addStep(title) {
      const v = get(view);
      const next = normalizeTitle(title);
      // 空のまま Enter を押したときは何もしない(02「子と Step」)。
      if (!v || next === "") return false;
      try {
        await deps.items.addStep(v.store.id, v.item.id, next);
      } catch (e) {
        refusedToast(e);
        return false;
      }
      await saved();
      return true;
    },
    async moveUp(itemId) {
      const v = get(view);
      if (!v || v.children[0]?.id === itemId) return;
      try {
        await deps.items.moveUp(v.store.id, itemId);
      } catch (e) {
        refusedToast(e);
        return;
      }
      await saved();
    },
  };
}
