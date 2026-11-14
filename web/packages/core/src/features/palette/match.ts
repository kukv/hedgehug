import type { PaletteAlias, PaletteIndex, PaletteItem, PaletteProject } from "../../model/palette";
import type { StoreNode, WorkspaceNode } from "../catalog/catalog";
import { dayOf, formatDay } from "../dates/date";

// コマンドパレットの候補の一致と並び(02「コマンドパレット」、A5c-5・A5c-6・A5c-8)。

export interface Matched<T> {
  c: T;
  hint: string;
}

// norm は、入力を前後の空白を除いて小文字にする(A5c-5)。
const norm = (input: string) => input.trim().toLowerCase();
const has = (text: string, q: string) => text.toLowerCase().includes(q);

// byName は、名前に入力を含むもの(大文字小文字を区別しない)。入力が空なら全部。
export function byName<T extends { label: string }>(list: T[], input: string): T[] {
  const q = norm(input);
  return q === "" ? list : list.filter((x) => has(x.label, q));
}

// oldIdHint は、旧 ID の添え書き。いまの ID でも一致したとき・旧 ID が一致しないときは null。
// 複数の旧 ID が一致したら、日時の新しいもの(日時のあるものを、旧い接頭辞のものより先にする)。
function oldIdHint(display: string, aliases: PaletteAlias[], q: string, now: Date): string | null {
  if (has(display, q)) return null;
  const hit = aliases.filter((a) => has(a.id, q));
  if (hit.length === 0) return null;
  const dated = hit.flatMap((a) => (a.at === null ? [] : [new Date(a.at)]));
  if (dated.length === 0) return "旧 ID";
  const newest = dated.reduce((a, b) => (b.getTime() > a.getTime() ? b : a));
  return `旧 ID・${formatDay(dayOf(newest), now.getFullYear())} に移動`;
}

// order は、サイドバーの並びの位置(Store・ワークスペース・プロジェクトの順の番号)。一覧にないものは後ろ。
function order(stores: StoreNode[]) {
  const ws = new Map<string, number>();
  const ps = new Map<string, number>();
  let w = 0;
  let p = 0;
  for (const s of stores) {
    for (const x of s.workspaces) {
      ws.set(`${s.id}/${x.id}`, w++);
      for (const y of x.projects) ps.set(`${s.id}/${y.id}`, p++);
    }
  }
  return {
    workspace: (storeId: string, id: string) => ws.get(`${storeId}/${id}`) ?? w,
    project: (storeId: string, id: string) => ps.get(`${storeId}/${id}`) ?? p,
  };
}

// split は、ID の一致(いまの ID・旧 ID)を先、タイトルだけの一致を後にして、それぞれを並べる。
function split<T extends { display: string; title: string; aliases: PaletteAlias[] }>(
  list: T[],
  q: string,
  hint: (c: T, old: string | null) => string,
  now: Date,
  compare: (a: T, b: T) => number,
): Matched<T>[] {
  const byId: Matched<T>[] = [];
  const byTitle: Matched<T>[] = [];
  for (const c of list) {
    if (has(c.display, q) || c.aliases.some((a) => has(a.id, q))) {
      byId.push({ c, hint: hint(c, oldIdHint(c.display, c.aliases, q, now)) });
    } else if (has(c.title, q)) {
      byTitle.push({ c, hint: hint(c, null) });
    }
  }
  const sort = (xs: Matched<T>[]) => xs.sort((a, b) => compare(a.c, b.c));
  return [...sort(byId), ...sort(byTitle)];
}

// matchItems は、アイテムの候補。並びは ID の一致 → タイトルの一致、それぞれ Store・ワークスペースの並び → 番号の昇順。
// ヒントはプロジェクト名、旧 ID で一致したら「{プロジェクト名}・旧 ID・{日付} に移動」か「{プロジェクト名}・旧 ID」。
export function matchItems(
  index: PaletteIndex,
  stores: StoreNode[],
  input: string,
  now: Date,
): Matched<PaletteItem>[] {
  const q = norm(input);
  if (q === "") return [];
  const o = order(stores);
  return split(
    index.items,
    q,
    (c, old) => (old === null ? c.projectTitle : `${c.projectTitle}・${old}`),
    now,
    (a, b) =>
      o.workspace(a.storeId, a.workspaceId) - o.workspace(b.storeId, b.workspaceId) ||
      a.number - b.number,
  );
}

// matchProjects は、プロジェクトの候補。並びは ID の一致 → タイトルの一致、それぞれサイドバーの並び。
// ヒントはワークスペース名、旧 ID で一致したら「旧 ID・{日付} に移動」か「旧 ID」。
export function matchProjects(
  index: PaletteIndex,
  stores: StoreNode[],
  input: string,
  now: Date,
): Matched<PaletteProject>[] {
  const q = norm(input);
  if (q === "") return [];
  const o = order(stores);
  return split(
    index.projects,
    q,
    (c, old) => old ?? c.workspaceName,
    now,
    (a, b) => o.project(a.storeId, a.projectId) - o.project(b.storeId, b.projectId),
  );
}

// matchWorkspaces は、名前か接頭辞に入力を含むワークスペース。サイドバーの並びで、ヒントは Store 名。
export function matchWorkspaces(stores: StoreNode[], input: string): Matched<WorkspaceNode>[] {
  const q = norm(input);
  if (q === "") return [];
  return stores.flatMap((s) =>
    s.workspaces
      .filter((w) => has(w.name, q) || has(w.prefix, q))
      .map((c) => ({ c, hint: s.name })),
  );
}
