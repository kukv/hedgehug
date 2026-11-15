import { Refused } from "../../model/errors";
import type { ViewDto, ViewsPort } from "../../port/views";
import { type Readable, readonly, writable } from "../../reactive/store";
import type { Toasts } from "../toasts/toasts";
import { type ViewContent, viewContent } from "./content";

// 保存したビューの一覧と書く操作(04「保存したビュー」)。並びはサーバーが返した順のまま使う(E3b-7)。
// 断り(422)は文を返す。断り以外の失敗はエラーのトーストを出して null・false を返す。書いたあとは、返った値で一覧を
// 直してから読み直す。

export interface SavedView {
  id: string;
  name: string;
  content: ViewContent;
}

export interface Views {
  list: Readable<SavedView[]>;
  // load は、一覧を読み直す。失敗しても投げず、前の一覧を残す。
  load(): Promise<void>;
  // create は、作ったビューか断りの文。断り以外の失敗は null。
  create(name: string, c: ViewContent): Promise<SavedView | string | null>;
  save(id: string, c: ViewContent): Promise<SavedView | null>;
  // rename は、断りの文。変えられたときと、断り以外の失敗(トーストを出す)は null。
  rename(id: string, name: string): Promise<string | null>;
  duplicate(id: string): Promise<SavedView | null>;
  remove(id: string): Promise<boolean>;
}

const savedView = (v: ViewDto): SavedView => ({
  id: v.id,
  name: v.name,
  content: viewContent(v.content),
});

export function createViews(deps: { port: ViewsPort; toasts: Toasts }): Views {
  const list = writable<SavedView[]>([]);
  // latest は、一覧を変えた回数。読み込みの途中で書いたら、その読み込みの結果は捨てる(書いた値を古い一覧で戻さない)。
  let latest = 0;

  async function load() {
    latest += 1;
    const mine = latest;
    try {
      const rows = await deps.port.list();
      if (mine === latest) list.set(rows.map(savedView));
    } catch {
      // 前の一覧を残す。
    }
  }

  // write は、書く操作を 1 回行い、返った値で一覧を直して読み直す。断りは文、断り以外の失敗は null を返す。
  async function write<T>(
    run: () => Promise<T>,
    patch: (rows: SavedView[], result: T) => SavedView[],
  ): Promise<{ result: T } | string | null> {
    let result: T;
    try {
      result = await run();
    } catch (e) {
      if (e instanceof Refused) return e.message;
      deps.toasts.show(e instanceof Error ? e.message : String(e), { error: true });
      return null;
    }
    latest += 1;
    list.update((rows) => patch(rows, result));
    void load();
    return { result };
  }

  const replaceIn = (rows: SavedView[], v: SavedView) => rows.map((r) => (r.id === v.id ? v : r));
  // append は、末尾に足す。出来事で読み直した一覧が書いた応答より先に着き、もう入っていることがあるので、
  // 同じ識別子を除いてから足す。
  const append = (rows: SavedView[], v: SavedView) => [...rows.filter((r) => r.id !== v.id), v];
  const failedOrRefused = (r: unknown): r is string | null => r === null || typeof r === "string";

  async function upsert(run: () => Promise<ViewDto>, add: boolean): Promise<SavedView | null> {
    const r = await write(async () => savedView(await run()), add ? append : replaceIn);
    if (typeof r === "string") deps.toasts.show(r, { error: true });
    return failedOrRefused(r) ? null : r.result;
  }

  return {
    list: readonly(list),
    load,
    async create(name, c) {
      const r = await write(async () => savedView(await deps.port.create(name, c)), append);
      return failedOrRefused(r) ? r : r.result;
    },
    save: (id, c) => upsert(() => deps.port.save(id, c), false),
    async rename(id, name) {
      const r = await write(async () => savedView(await deps.port.rename(id, name)), replaceIn);
      return typeof r === "string" ? r : null;
    },
    duplicate: (id) => upsert(() => deps.port.duplicate(id), true),
    async remove(id) {
      const r = await write(
        () => deps.port.remove(id),
        (rows) => rows.filter((v) => v.id !== id),
      );
      if (typeof r === "string") deps.toasts.show(r, { error: true });
      return !failedOrRefused(r);
    },
  };
}
