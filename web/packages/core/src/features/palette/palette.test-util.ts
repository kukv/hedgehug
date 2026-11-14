import type { Item } from "../../model/item";
import type { PaletteIndex, RecentItem } from "../../model/palette";
import type { WorkspaceSettings } from "../../model/workspace";
import type { CatalogPort } from "../../port/catalog";
import { get, type Writable, writable } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import type { Here } from "../items/createDefaults";
import { item } from "../items/fixtures.test-util";
import { settings } from "../items/settings.test-util";
import type { Route } from "../router/route";
import type { SearchResult } from "../search/types";
import { createPalette, type Palette } from "./palette";
import { createPlanRequest } from "./planRequest";

// パレットのテストの組み立て。一覧は Store「work」(S1、書ける、teal)と「ro」(S2、閲覧のみ、orange)で、
// どちらもワークスペース Web(HH)とプロジェクト HH-P1・HH-P2 を持つ(catalog の fixtures)。

export const epic = item("S1", "S1-P1", 1, "Epic", null, "認証");
export const index: PaletteIndex = {
  items: [
    pi("S1", 1, "認証"),
    pi("S1", 2, "画面"),
    // S2 にも HH-1 がある(選んでいるアイテムの見出しに Store の点を付ける)。
    pi("S2", 1, "別の認証"),
  ],
  projects: [
    {
      storeId: "S1",
      workspaceId: "S1-W1",
      projectId: "S1-P1",
      display: "HH-P1",
      title: "プロジェクト 1",
      workspaceName: "Web",
      aliases: [],
    },
  ],
};
export const recent: RecentItem[] = [
  {
    storeId: "S1",
    itemId: "S1-I2",
    display: "HH-2",
    title: "画面",
    projectTitle: "プロジェクト 1",
  },
];

export function pi(storeId: string, n: number, title: string) {
  return {
    storeId,
    workspaceId: `${storeId}-W1`,
    projectId: `${storeId}-P1`,
    itemId: `${storeId}-I${n}`,
    number: n,
    display: `HH-${n}`,
    title,
    projectTitle: "プロジェクト 1",
    aliases: [],
  };
}

export interface Opts {
  here?: Here;
  // selected は、選んでいるアイテム(items.get が返すもの)。null なら選んでいない。
  selected?: Item | null;
  failItem?: boolean;
  index?: () => Promise<PaletteIndex>;
  recent?: () => Promise<RecentItem[]>;
  target?: { storeId: string; itemId: string } | null;
  settings?: WorkspaceSettings;
  // settingsRead は、ステータスの一覧の設定を読む前に待つもの(待たせる・失敗させるため)。
  settingsRead?: () => Promise<void>;
  catalogPort?: CatalogPort;
  mac?: boolean;
  // moved は、navigation.go の結果(「入力を破棄」の確認で断られたら false)。
  moved?: boolean;
  // search は、検索の解析の応答(「このクエリで」。省くとキーなし・エラーなしで 0 件)。
  search?: (query: string) => Promise<SearchResult>;
}

export const noKey: SearchResult = {
  errors: [],
  hasKey: false,
  group: "",
  deleted: false,
  rows: [],
  columns: [],
  groupBy: "",
  groups: [],
};
export function found(n: number): SearchResult {
  const row = {} as SearchResult["rows"][number];
  return { ...noKey, hasKey: true, rows: Array.from({ length: n }, () => row) };
}

// manualTimer は、fire を呼ぶまで動かさないタイマー(入力を待つ 150ms をテストから進める)。
export function manualTimer() {
  const pending = new Map<number, { fn: () => void; ms: number }>();
  let next = 0;
  return {
    set: (fn: () => void, ms: number) => {
      pending.set(++next, { fn, ms });
      return next;
    },
    clear: (h: unknown) => void pending.delete(h as number),
    // fire は、待っているものを呼ぶ(待ち時間を返す)。
    fire(): number[] {
      const all = [...pending.values()];
      pending.clear();
      for (const t of all) t.fn();
      return all.map((t) => t.ms);
    },
    count: () => pending.size,
  };
}

// Call は、呼ばれた口の記録。open は、呼ばれたときにパレットが開いていたか。
export interface Call {
  name: string;
  args: unknown[];
  open: boolean;
}

export async function setup(o: Opts = {}) {
  const catalog = createCatalog(o.catalogPort ?? fakeCatalogPort());
  await catalog.load();
  const calls: Call[] = [];
  const toasts: string[] = [];
  const route: Writable<Route> = writable<Route>({ name: "home" });
  const planRequest = createPlanRequest();
  let palette: Palette | null = null;
  const rec =
    (name: string) =>
    (...args: unknown[]) => {
      calls.push({ name, args, open: palette !== null && get(palette.state) !== null });
    };
  const timer = manualTimer();
  const searched: string[] = [];
  const sel = o.selected === undefined ? epic : o.selected;
  palette = createPalette({
    catalog,
    navigation: {
      go: async (...a) => {
        rec("go")(...a);
        return o.moved ?? true;
      },
    },
    itemDialog: { open: async (...a) => rec("itemDialog.open")(...a) },
    projectDialog: { open: async (...a) => rec("projectDialog.open")(...a) },
    workspaceDialog: { open: rec("workspaceDialog.open") },
    transferDialog: { open: async (...a) => rec("transferDialog.open")(...a) },
    detail: {
      open: async (...a) => rec("detail.open")(...a),
      target: () => o.target ?? null,
    },
    sync: { syncAll: async () => rec("syncAll")() },
    planRequest,
    toasts: { show: (text) => void toasts.push(text) },
    here: () => o.here ?? {},
    mac: o.mac ?? false,
    port: { index: o.index ?? (async () => index), recent: o.recent ?? (async () => recent) },
    items: {
      get: async (storeId, itemId) => {
        if (o.failItem || !sel) throw new Error("not found");
        return { ...sel, storeId, id: itemId };
      },
    },
    workspaces: {
      get: async (storeId, id) => {
        await o.settingsRead?.();
        return {
          id,
          storeId,
          name: "Web",
          prefix: "HH",
          oldPrefixes: [],
          settings: o.settings ?? settings,
        };
      },
    },
    selection: {
      current: () =>
        sel
          ? {
              storeId: sel.storeId,
              itemId: sel.id,
              source: "detail",
              setStatus: async (id) => rec("setStatus")(id),
            }
          : null,
    },
    route,
    openTab: rec("openTab"),
    now: () => new Date("2026-11-14T00:00:00Z"),
    timer,
    search: {
      search: async (q) => {
        searched.push(q);
        return (o.search ?? (async () => noKey))(q);
      },
    },
    openSearch: async (...a) => rec("openSearch")(...a),
  });
  return { palette, calls, toasts, route, planRequest, catalog, timer, searched };
}

// labels は、行を文字列にする(見出しは「# 」、押せない行は「~ 」、無効の候補は「x 」を頭に付ける)。
export function labels(p: Palette): string[] {
  return (get(p.state)?.rows ?? []).map((r) => {
    if (r.kind === "head") return `# ${r.text}`;
    if (r.kind === "more") return `~ ${r.text}`;
    return `${r.c.disabled ? "x " : ""}${r.c.label}`;
  });
}

// pick は、label の候補を選ぶ(Enter。newTab なら Ctrl+Enter)。
export async function pick(p: Palette, label: string, newTab = false): Promise<void> {
  const i = (get(p.state)?.rows ?? []).findIndex(
    (r) => r.kind === "candidate" && r.c.label === label,
  );
  if (i < 0) throw new Error(`no candidate ${label}: ${labels(p).join(" | ")}`);
  await p.choose(i, newTab);
}

export const named = (calls: Call[], name: string) => calls.filter((c) => c.name === name);
