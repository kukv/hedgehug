import { createScreenStatesPort, createSearchPort } from "@hedgehug/api";
import {
  type CaretTerm,
  createChangeAll,
  createRowSelection,
  createScreenStates,
  createSearch,
  type FilterCandidate,
  filterCandidates,
  get,
  type Member,
  menuAt,
  type PaletteItem,
  rowMenuContext,
  type SearchRow,
  type SelectedItem,
  setRowStatus,
} from "@hedgehug/core";
import { reloads } from "./reloads";
import { catalog, itemsPort, listMembers, navigation } from "./wire";
import { bulkChange, itemMenuHost } from "./wireActions";
import { palettePort, timer, toasts } from "./wireBase";

// 検索の画面(04「検索」、E1)。wire.ts が 300 行なので、ここで作る。
export const screenStates = createScreenStates(createScreenStatesPort());
// searchPort は、パレットの「このクエリで」も使う。
export const searchPort = createSearchPort();
// searchSelection は、検索の画面の選んでいる行(E2a-16)。結果が変わるたびに、search が選べる行で整理する。
export const searchSelection = createRowSelection();
export const search = createSearch({
  port: searchPort,
  screenStates,
  timer,
  toasts,
  selection: searchSelection,
});

// openRowMenu は、テーブルの行末の「…」で、行のアイテムのメニュー(02)を開く(E2a-17)。rect は「…」の
// getBoundingClientRect()。プロジェクトのアイテムを読めない・アイテムがないときは何もしない。
export async function openRowMenu(
  row: SearchRow,
  rect: { left: number; bottom: number },
): Promise<void> {
  const items = await itemsPort.list(row.storeId, row.projectId).catch(() => null);
  const found = items && rowMenuContext(items, row.itemId);
  if (!found) return;
  const { x, y } = menuAt(rect);
  itemMenuHost.open(found.item, found.ctx, x, y);
}

// selectedOne は、パレットの「選んでいるアイテム」(E2a-18)。検索のテーブルでちょうど 1 行を選んでいるときだけ返す。
export function selectedOne(): SelectedItem | null {
  const rows = get(searchSelection.rows);
  const only = rows.length === 1 ? rows[0] : undefined;
  if (!only) return null;
  const { storeId, itemId } = only;
  return {
    storeId,
    itemId,
    source: "table",
    setStatus: (statusId) =>
      setRowStatus(
        { items: itemsPort, toasts, reload: () => search.reload() },
        storeId,
        itemId,
        statusId,
      ),
  };
}

// refreshCandidates は、開いている検索の画面の候補の元を読み直す。画面を閉じると null。
let refreshCandidates: (() => Promise<void>) | null = null;

// Store が変わった知らせで、検索の画面を開いていれば、結果と候補の元を読み直す(E1-19)。
reloads.add(async () => {
  if (get(navigation.route).name !== "search") return;
  await Promise.all([search.reload(), refreshCandidates?.()]);
});

export interface SearchCandidates {
  // list は、カーソルの位置の語の候補。assignee: のときは、まだ読んでいない Store のメンバーを読み始める。
  list(term: CaretTerm | null): FilterCandidate[];
  // stop は、画面を閉じるとき。Store が変わっても元を読み直さなくなる。
  stop(): void;
}

// createCandidates は、絞り込み欄の候補の元を持つ(E1-20)。画面を開くたびに作る。アイテムはパレットの索引を
// 作ったときに 1 回読み、メンバーは最初に assignee: の候補を出すときに Store ごとに読む。Store が変わった知らせでは、
// 索引を読み直し、読んだメンバーを捨てる(次の assignee: で読み直す)。
// 読み終わったら、読んだ元を添えて changed を呼ぶ(画面は、いまの語がその元を使うなら候補を出し直す)。
export function createCandidates(changed: (source: "items" | "members") => void): SearchCandidates {
  let items: PaletteItem[] = [];
  const members = new Map<string, Member[]>();
  // asked は、読み込み中か読めた Store。失敗した Store は外し、次の assignee: で読み直す。
  const asked = new Set<string>();
  // round は、元を読み直した回数。読み直す前に始めた読み込みの応答は捨てる。
  let round = 0;

  function loadItems(): Promise<void> {
    const mine = round;
    return palettePort
      .index()
      .then((index) => {
        if (mine !== round) return;
        items = index.items;
        changed("items");
      })
      .catch(() => {});
  }

  function loadMembers() {
    const mine = round;
    for (const s of get(catalog.stores)) {
      if (asked.has(s.id)) continue;
      asked.add(s.id);
      listMembers(s.id)
        .then((m) => {
          if (mine !== round) return;
          members.set(s.id, m);
          changed("members");
        })
        .catch(() => {
          if (mine === round) asked.delete(s.id);
        });
    }
  }

  function refresh(): Promise<void> {
    round += 1;
    members.clear();
    asked.clear();
    changed("members");
    return loadItems();
  }

  void loadItems();
  refreshCandidates = refresh;

  return {
    list(term) {
      if (!term) return [];
      if (term.key === "assignee") loadMembers();
      const sources = {
        stores: get(catalog.stores),
        items,
        members: [...members].flatMap(([storeId, ms]) =>
          ms.map((m) => ({ storeId, login: m.login, name: m.name })),
        ),
      };
      return filterCandidates(term, sources, { kind: "all" });
    },
    stop() {
      if (refreshCandidates === refresh) refreshCandidates = null;
    },
  };
}

// openSearch は、パレットの「検索で開く」(E1-21)。検索は 1 回だけ: 検索の画面にいればその場で検索し、ほかの画面からは
// 絞り込みを書いておき、開いた画面の open が検索する(同じ画面へは移らず open も呼ばれない)。
// 移れたかを返す。「入力を破棄」の確認で断られたら、検索の準備を取り消して false。
export async function openSearch(query: string): Promise<boolean> {
  if (get(navigation.route).name === "search") {
    await search.replaceQuery(query);
    return true;
  }
  search.prepareQuery(query);
  const moved = await navigation.go({ name: "search" });
  if (!moved) search.cancelPrepare();
  return moved;
}

// bulkChangeAll は、パレットの「{N} 件をまとめて変更…」(E2a-19)。
export const bulkChangeAll = createChangeAll({
  openSearch,
  settled: () => search.settled(),
  state: () => get(search.state),
  selection: searchSelection,
  openBulkChange: () => bulkChange.open(searchSelection, { after: () => search.reload() }),
});
