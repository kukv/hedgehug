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

// CandidateScope は、候補を出す範囲。all は表示中のすべての Store、workspace はそのワークスペースだけ。
// myTasks は、`project:` だけ projects(`{storeId}/{projectId}`)に入るプロジェクトに絞り、ほかのキーは all と同じ。
export type CandidateScope =
  | { kind: "all" }
  | { kind: "myTasks"; projects: ReadonlySet<string> }
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
  if (scope.kind !== "workspace") return src.stores;
  return src.stores
    .filter((s) => s.id === scope.storeId)
    .map((s) => ({ ...s, workspaces: s.workspaces.filter((w) => w.id === scope.workspaceId) }));
}

function hits(label: string, typed: string): boolean {
  return label.toLowerCase().includes(typed.toLowerCase());
}

function projectEntries(stores: StoreNode[], scope: CandidateScope, typed: string): Entry[] {
  return stores.flatMap((s) =>
    s.workspaces.flatMap((w) =>
      w.projects
        .filter((p) => scope.kind !== "myTasks" || scope.projects.has(`${s.id}/${p.id}`))
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

// workspaceEntries は、「名前 接頭辞」(Web HH)のラベルで探す。
function workspaceEntries(stores: StoreNode[], typed: string): FilterCandidate[] {
  // 接頭辞が同じなら `workspace:HH` はどちらにも一致するので、最初の 1 つだけを出す。
  const seen = new Set<string>();
  return stores
    .flatMap((s) => s.workspaces)
    .map((w) => ({ label: `${w.name} ${w.prefix}`, insert: w.prefix }))
    .filter((c) => hits(c.label, typed))
    .filter((c) => !seen.has(c.insert.toLowerCase()) && seen.add(c.insert.toLowerCase()));
}

function itemEntries(src: CandidateSources, scope: CandidateScope, typed: string): Entry[] {
  return src.items
    .filter(
      (i) =>
        scope.kind !== "workspace" ||
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
  if (term.key === "project") found = withDots(projectEntries(stores, scope, term.typed), stores);
  else if (term.key === "workspace") found = workspaceEntries(stores, term.typed);
  else if (term.key === "assignee") found = memberEntries(src, scope, term.typed);
  else if (itemKeys.has(term.key)) found = withDots(itemEntries(src, scope, term.typed), stores);
  return found.slice(0, limit);
}

// replaced は、置き換える部分 [start, end)。ふつうは打った値の部分。打った値が開き引用符から始まり、カーソルが
// 引用符の中にあるときは、カーソルの後ろの文字も値の続きなので置き換える(残すと値の後ろに残る):
// - 引用符の中で `,` が先に来れば、引用符を残し、`,` の前までを置き換える(後ろの値はそのまま)。
// - でなければ、閉じ引用符まで(なければ末尾まで。閉じていない引用符は末尾までを 1 つの語にする)を置き換える。
function replaced(text: string, term: CaretTerm): { start: number; end: number } {
  const plain = { start: term.start, end: term.end };
  if (text[term.start] !== '"') return plain;
  const quotes = text.slice(term.start, term.end).split('"').length - 1;
  if (quotes % 2 === 0) return plain;
  const close = text.indexOf('"', term.end);
  const stop = close < 0 ? text.length : close;
  const comma = text.indexOf(",", term.end);
  if (comma >= 0 && comma < stop) return { start: term.start + 1, end: comma };
  return { start: term.start, end: close < 0 ? text.length : close + 1 };
}

// applyCandidate は、打った値の部分を入れるものに置き換え、カーソルをその直後に置く。
export function applyCandidate(
  text: string,
  term: CaretTerm,
  c: FilterCandidate,
): { text: string; caret: number } {
  const { start, end } = replaced(text, term);
  return {
    text: text.slice(0, start) + c.insert + text.slice(end),
    caret: start + c.insert.length,
  };
}
