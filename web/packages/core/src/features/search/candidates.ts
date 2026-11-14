// 絞り込み欄の候補(04「候補」、E1-20)。カーソルの位置の語のキーと打った値から、元の一覧を探す。
import type { PaletteItem } from "../../model/palette";
import type { StoreNode } from "../catalog/catalog";
import type { CaretTerm } from "./caret";

export interface FilterCandidate {
  label: string;
  // insert は、選ぶと打った値の代わりに入れるもの。
  insert: string;
  // dot は、同じ ID が別の Store にもあるときの、その Store の色(トークン名)。
  dot?: string;
}

export interface CandidateSources {
  stores: StoreNode[];
  items: PaletteItem[];
  members: { storeId: string; login: string; name: string }[];
}

// E1 は all だけを使う。E4・E7 が workspace(そのワークスペースの範囲)を使う。
export type CandidateScope =
  | { kind: "all" }
  | { kind: "workspace"; storeId: string; workspaceId: string };

const limit = 10;
const itemKeys = new Set([
  "parent",
  "epic",
  "blocks",
  "blocked_by",
  "causes",
  "caused_by",
  "duplicates",
  "duplicated_by",
  "relates_to",
  "clones",
  "cloned_by",
]);

interface Entry {
  storeId: string;
  display: string;
  label: string;
  insert: string;
}

// withDots は、同じ表示の ID が別の Store にもあるものに、その Store の色を付ける。
function withDots(entries: Entry[], stores: StoreNode[]): FilterCandidate[] {
  const storeIds = new Map<string, Set<string>>();
  for (const e of entries) {
    const key = e.display.toLowerCase();
    const set = storeIds.get(key) ?? new Set<string>();
    set.add(e.storeId);
    storeIds.set(key, set);
  }
  return entries.map((e) => {
    const color = stores.find((s) => s.id === e.storeId)?.color;
    const shared = (storeIds.get(e.display.toLowerCase())?.size ?? 0) > 1;
    return shared && color
      ? { label: e.label, insert: e.insert, dot: color }
      : { label: e.label, insert: e.insert };
  });
}

function scopedStores(src: CandidateSources, scope: CandidateScope): StoreNode[] {
  if (scope.kind === "all") return src.stores;
  return src.stores
    .filter((s) => s.id === scope.storeId)
    .map((s) => ({ ...s, workspaces: s.workspaces.filter((w) => w.id === scope.workspaceId) }));
}

function hits(label: string, typed: string): boolean {
  return label.toLowerCase().includes(typed.toLowerCase());
}

function projectEntries(stores: StoreNode[], typed: string): Entry[] {
  return stores.flatMap((s) =>
    s.workspaces.flatMap((w) =>
      w.projects
        .map((p) => ({
          storeId: s.id,
          display: p.displayId,
          label: `${p.displayId} ${p.title}`,
          insert: p.displayId,
        }))
        .filter((e) => hits(e.label, typed)),
    ),
  );
}

function workspaceEntries(stores: StoreNode[], typed: string): FilterCandidate[] {
  // 接頭辞が同じなら `workspace:HH` はどちらにも一致するので、最初の 1 つだけを出す。
  const seen = new Set<string>();
  return stores
    .flatMap((s) => s.workspaces)
    .filter((w) => hits(w.name, typed) || hits(w.prefix, typed))
    .filter((w) => !seen.has(w.prefix.toLowerCase()) && seen.add(w.prefix.toLowerCase()))
    .map((w) => ({ label: w.name, insert: w.prefix }));
}

function itemEntries(src: CandidateSources, scope: CandidateScope, typed: string): Entry[] {
  return src.items
    .filter(
      (i) =>
        scope.kind === "all" ||
        (i.storeId === scope.storeId && i.workspaceId === scope.workspaceId),
    )
    .map((i) => ({
      storeId: i.storeId,
      display: i.display,
      label: `${i.display} ${i.title}`,
      insert: i.display,
    }))
    .filter((e) => hits(e.label, typed));
}

function memberEntries(
  src: CandidateSources,
  scope: CandidateScope,
  typed: string,
): FilterCandidate[] {
  const seen = new Set<string>();
  const out: FilterCandidate[] = [];
  for (const m of src.members) {
    if (scope.kind === "workspace" && m.storeId !== scope.storeId) continue;
    const login = m.login.toLowerCase();
    const label = `${m.name} @${m.login}`;
    if (seen.has(login) || !hits(label, typed)) continue;
    seen.add(login);
    out.push({ label, insert: m.login });
  }
  return out;
}

// candidateSource は、キーの候補が使う元。items はパレットの索引、members は Store のメンバー、catalog は
// プロジェクト・ワークスペースの一覧。候補のないキーは null。元を読み終えたとき、候補を作り直すかを決めるのに使う。
export function candidateSource(key: string): "items" | "members" | "catalog" | null {
  if (key === "assignee") return "members";
  if (key === "project" || key === "workspace") return "catalog";
  return itemKeys.has(key) ? "items" : null;
}

// filterCandidates は、打った値を含むラベルを元の並びのまま 10 件まで返す。打った値が空のときは出さない。
export function filterCandidates(
  term: CaretTerm,
  src: CandidateSources,
  scope: CandidateScope,
): FilterCandidate[] {
  if (term.typed === "") return [];
  const stores = scopedStores(src, scope);
  let found: FilterCandidate[] = [];
  if (term.key === "project") found = withDots(projectEntries(stores, term.typed), stores);
  else if (term.key === "workspace") found = workspaceEntries(stores, term.typed);
  else if (term.key === "assignee") found = memberEntries(src, scope, term.typed);
  else if (itemKeys.has(term.key)) found = withDots(itemEntries(src, scope, term.typed), stores);
  return found.slice(0, limit);
}

// applyCandidate は、打った値の部分([start, end))だけを入れるものに置き換え、カーソルをその直後に置く。
export function applyCandidate(
  text: string,
  term: CaretTerm,
  c: FilterCandidate,
): { text: string; caret: number } {
  // 開き引用符から入れ始めたときは、カーソルのすぐ後ろの閉じ引用符も置き換える(残すと引用符が閉じない)。
  const end = text[term.start] === '"' && text[term.end] === '"' ? term.end + 1 : term.end;
  return {
    text: text.slice(0, term.start) + c.insert + text.slice(end),
    caret: term.start + c.insert.length,
  };
}
