import { NotFound, Refused } from "../../model/errors";
import type { Item, PropertyInput } from "../../model/item";
import type { WorkspaceSettings } from "../../model/workspace";
import type { CatalogPort } from "../../port/catalog";
import type { ItemsPort } from "../../port/items";
import type { WorkspacesPort } from "../../port/workspaces";
import { get, type Readable, readonly, writable } from "../../reactive/store";
import type { Catalog, ProjectNode, StoreNode, WorkspaceNode } from "../catalog/catalog";
import { itemDisplayId } from "../router/route";
import type { Toasts } from "../toasts/toasts";
import { type People, type PropertyRow, propertyRows, valueOf as valueOfItem } from "./properties";
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
  // siblings は、同じ親の下の自分を含む兄弟を並び順で(メニューの「上へ移動」「下へ移動」。詳細パネルに絞り込みはない)。
  siblings: Item[];
  readOnly: boolean;
  // settings・people・rows は、プロパティの区画(02「プロパティの入力と保存」)。
  settings: WorkspaceSettings;
  people: People;
  rows: PropertyRow[];
}

export interface Detail {
  view: Readable<DetailView | null>;
  full: Readable<boolean>;
  open(storeId: string, itemId: string): Promise<void>;
  close(): void;
  // requestClose は、入力中のものがあれば「入力を破棄」の確認をしてから閉じる(「×」・Esc)。断ったら開いたまま。
  // close は確認しない(画面の移り変わりと、反映でアイテムがなくなったときの閉じ方)。
  requestClose(): Promise<void>;
  toggleFull(): void;
  // rename は、タイトルを保存する。false なら、入力欄を元のタイトルに戻す。
  rename(title: string): Promise<boolean>;
  // addStep は、Step を末尾に作る。true なら、入力欄を空にする。
  addStep(title: string): Promise<boolean>;
  // moveUp は、Step を直前の Step の直前に置く(並びは入れ替えと同じ)。先頭なら何もしない。
  moveUp(itemId: string): Promise<void>;
  // setProperty は、プロパティの値を 1 つ保存する。false なら、入力欄を保存済みの値に戻す。
  // itemId が開いているアイテムでもその子でもなければ、保存せずに false。
  setProperty(itemId: string, propertyId: string, value: PropertyInput): Promise<boolean>;
  // toggleValue は、関連システム・複数選択の値を 1 つ入れる・外す。
  toggleValue(propertyId: string, valueId: string, on: boolean): Promise<void>;
  // checkStep は、Step のチェックを入れ切りする(ステータスが変わる)。
  checkStep(stepId: string, done: boolean): Promise<void>;
  // reload は、開いているアイテムを読み直す(画面への反映)。開いていなければ何もしない。
  // なくなっていたら、パネルを閉じる(06「反映で変わる画面」)。
  reload(): Promise<void>;
  // target は、開いている(開こうとしている)アイテム。開けなかったときと閉じたときは null。読み直しが開く途中を
  // 追い越しても、同じアイテムのまま(旧 ID から開いたときの上部の 1 行を、開けなかったときだけ捨てるのに使う)。
  target(): { storeId: string; itemId: string } | null;
}

export interface DetailDeps {
  catalog: Catalog;
  items: ItemsPort;
  workspaces: WorkspacesPort;
  catalogPort: CatalogPort;
  toasts: Toasts;
  // onChanged は、保存したあと(開いているプロジェクトの画面を読み直す)。
  onChanged(): void;
  // leave は、入力中のものがあるとき、捨ててよいかを聞く(別のアイテムを開く・パネルを閉じる)。ないときは常に true。
  leave?: () => Promise<boolean>;
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
  // target は、開いている(開こうとしている)アイテム。reload が、開いている途中の別のアイテムを古い表示で上書きしないために持つ。
  let target: { storeId: string; itemId: string } | null = null;

  // context は、ワークスペースごとの設定とメンバー。一度読んだら使い回し、画面への反映(reload)と
  // 詳細パネルを閉じたときに読み直す(A3 前半の決定 29)。
  type Context = { settings: WorkspaceSettings; people: People };
  let contexts = new Map<string, Promise<Context>>();
  function contextOf(storeId: string, workspaceId: string, refresh: boolean): Promise<Context> {
    const key = `${storeId}/${workspaceId}`;
    const cached = contexts.get(key);
    if (cached && !refresh) return cached;
    const read = (async () => {
      const [ws, members, me] = await Promise.all([
        deps.workspaces.get(storeId, workspaceId),
        deps.catalogPort.listMembers(storeId).catch(() => []),
        // me が決まらないときは空(A2 の決定 3)。
        deps.catalogPort.me(storeId).catch(() => ""),
      ]);
      return { settings: ws.settings, people: { members, me } };
    })();
    contexts.set(key, read);
    // 読めなかったら、次のときに読み直せるよう残さない。
    read.catch(() => contexts.get(key) === read && contexts.delete(key));
    return read;
  }

  // shut は、パネルを閉じる(確認なし)。全画面も解除する。close と、読み直しでアイテムやプロジェクトがなくなっていたときの閉じ方が同じ。
  function shut() {
    latest += 1;
    target = null;
    contexts = new Map();
    view.set(null);
    full.set(false);
  }

  async function load(storeId: string, itemId: string, refresh = false) {
    latest += 1;
    const mine = latest;
    target = { storeId, itemId };
    try {
      const item = await deps.items.get(storeId, itemId);
      // ほかの書き手が別のワークスペースへ移していたら、確認なしで閉じる(A5a-18。自分で移したときは開き直す)。
      const shown = get(view)?.item;
      const moved = shown?.storeId === storeId && shown.id === item.id;
      if (refresh && moved && shown.workspaceId !== item.workspaceId) {
        if (mine === latest) shut();
        return;
      }
      const [all, { settings, people }] = await Promise.all([
        deps.items.list(storeId, item.projectId),
        contextOf(storeId, item.workspaceId, refresh),
      ]);
      const store = deps.catalog.store(storeId);
      const project = deps.catalog.project(storeId, item.projectId);
      const workspace = project && deps.catalog.workspace(storeId, project.workspaceId);
      if (mine !== latest) return;
      if (!store || !project || !workspace) return shut();
      view.set({
        store,
        workspace,
        project,
        item,
        displayId: itemDisplayId(workspace.prefix, item.number),
        ancestors: ancestorsOf(all, item.id),
        children: all.filter((it) => it.parentId === item.id).sort(compareSiblings),
        siblings: all.filter((it) => it.parentId === item.parentId).sort(compareSiblings),
        readOnly: store.readOnly,
        settings: settings,
        people,
        rows: propertyRows(settings, item.kind, (p) => valueOfItem(item, p), people),
      });
    } catch (e) {
      // 開いているアイテムがなくなったら、パネルを閉じる(02「詳細パネル」)。
      if (!(e instanceof NotFound)) throw e;
      if (mine === latest) shut();
    }
  }

  // saved は、保存したあとに画面を読み直す。保存のあいだに別のアイテムへ移っていたら、読み直さない。
  // 反映の reload で latest が進んでいても、同じアイテムを開いていれば読み直す。
  async function saved(storeId: string, itemId: string) {
    if (target?.storeId === storeId && target.itemId === itemId) await load(storeId, itemId);
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
    async open(storeId, itemId) {
      // 別のアイテムが開いているときは、入力中のものを捨ててよいかを先に聞く(02「説明」)。
      const other = target !== null && (target.storeId !== storeId || target.itemId !== itemId);
      if (other && deps.leave && !(await deps.leave())) return;
      await load(storeId, itemId);
    },
    close: shut,
    target: () => (target ? { ...target } : null),
    async requestClose() {
      if (!deps.leave || (await deps.leave())) this.close();
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
      await saved(v.store.id, v.item.id);
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
      await saved(v.store.id, v.item.id);
      return true;
    },
    async moveUp(itemId) {
      const v = get(view);
      if (!v) return;
      // 直前の子の直前に置く。先頭と、子でないものは何もしない。
      const prev = v.children[v.children.findIndex((c) => c.id === itemId) - 1];
      if (!prev) return;
      try {
        await deps.items.reorder(v.store.id, itemId, prev.id, false);
      } catch (e) {
        refusedToast(e);
        return;
      }
      await saved(v.store.id, v.item.id);
    },
    async setProperty(itemId, propertyId, value) {
      const v = get(view);
      // 開いているアイテムとその子のほかは保存しない(入力中に開くアイテムが変わったときの、前の入力)。
      if (!v || (itemId !== v.item.id && !v.children.some((c) => c.id === itemId))) return false;
      try {
        await deps.items.setProperty(v.store.id, itemId, propertyId, value);
      } catch (e) {
        refusedToast(e);
        return false;
      }
      await saved(v.store.id, v.item.id);
      return true;
    },
    async toggleValue(propertyId, valueId, on) {
      const v = get(view);
      if (!v) return;
      try {
        if (on) await deps.items.addValue(v.store.id, v.item.id, propertyId, valueId);
        else await deps.items.removeValue(v.store.id, v.item.id, propertyId, valueId);
      } catch (e) {
        refusedToast(e);
        return;
      }
      await saved(v.store.id, v.item.id);
    },
    async checkStep(stepId, done) {
      const v = get(view);
      if (!v) return;
      try {
        await deps.items.checkStep(v.store.id, stepId, done);
      } catch (e) {
        refusedToast(e);
        return;
      }
      await saved(v.store.id, v.item.id);
    },
    async reload() {
      if (target) await load(target.storeId, target.itemId, true);
    },
  };
}
