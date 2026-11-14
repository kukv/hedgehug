import { createScreenStatesPort, createSearchPort } from "@hedgehug/api";
import {
  type CaretTerm,
  createScreenStates,
  createSearch,
  type FilterCandidate,
  filterCandidates,
  get,
  type Member,
  type PaletteItem,
} from "@hedgehug/core";
import { reloads } from "./reloads";
import { catalog, listMembers, navigation } from "./wire";
import { palettePort, timer, toasts } from "./wireBase";

// 検索の画面(04「検索」、E1)。wire.ts が 300 行なので、ここで作る。
export const screenStates = createScreenStates(createScreenStatesPort());
// searchPort は、パレットの「このクエリで」も使う。
export const searchPort = createSearchPort();
export const search = createSearch({ port: searchPort, screenStates, timer, toasts });

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
