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
export const search = createSearch({ port: createSearchPort(), screenStates, timer, toasts });

// Store が変わった知らせで、検索の画面を開いていれば読み直す(E1-19)。
reloads.add(async () => {
  if (get(navigation.route).name === "search") await search.reload();
});

export interface SearchCandidates {
  // list は、カーソルの位置の語の候補。assignee: のときは、まだ読んでいない Store のメンバーを読み始める。
  list(term: CaretTerm | null): FilterCandidate[];
}

// createCandidates は、絞り込み欄の候補の元を持つ(E1-20)。画面を開くたびに作る。アイテムはパレットの索引を
// 作ったときに 1 回読み、メンバーは最初に assignee: の候補を出すときに Store ごとに読む。
// 読み終わったら changed を呼ぶ(画面はいまの語で候補を出し直す)。
export function createCandidates(changed: () => void): SearchCandidates {
  let items: PaletteItem[] = [];
  const members = new Map<string, Member[]>();
  // asked は、読み込み中か読めた Store。失敗した Store は外し、次の assignee: で読み直す。
  const asked = new Set<string>();
  palettePort
    .index()
    .then((index) => {
      items = index.items;
      changed();
    })
    .catch(() => {});

  function loadMembers() {
    for (const s of get(catalog.stores)) {
      if (asked.has(s.id)) continue;
      asked.add(s.id);
      listMembers(s.id)
        .then((m) => {
          members.set(s.id, m);
          changed();
        })
        .catch(() => asked.delete(s.id));
    }
  }

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
  };
}
