import { NotFound } from "../../model/errors";
import type { Gone, MovedFrom } from "../../model/transfer";
import type { ItemsPort } from "../../port/items";
import type { TransfersPort } from "../../port/transfers";
import type { StoreNode, WorkspaceNode } from "../catalog/catalog";
import {
  type ProjectTab,
  parseDisplayId,
  projectDisplayId,
  type Route,
  type SettingsTab,
  type WorkspaceTab,
} from "./route";

// アドレス(Store 名と画面の ID)を内部 ID にする(01「画面のアドレス」、A2 の計画の決定 7)。
// いまの ID・接頭辞で見つからないときだけ、旧いものをたどる。ワークスペースとプロジェクトは、一覧の旧い接頭辞
// (oldPrefixes。A5a-21)で探す。ワークスペースも、一覧ではアドレスの Store だけを(いまの接頭辞・旧い接頭辞で)
// 探し、なければサーバーの locateWorkspace に聞く(B5c-13。プロジェクトと同じ)。アイテムは、アドレスの Store のいまの ID で見つからなければ、サーバーの locate に
// 聞く(アドレスの Store の旧 ID を、ほかの Store のいまの ID より先に見る。A5a-15)。プロジェクトも、一覧では
// アドレスの Store だけを(いまの接頭辞・旧い接頭辞で)探し、なければサーバーの locateProject に聞く(旧 ID・
// ほかの Store のいまの ID・一覧を読み直す前の移したばかりのいまの ID。A5b-24)。一覧でほかの Store を探すと、
// 別の Store へ移した直後に、ほかの Store の同じ ID のプロジェクトを開いてしまう。どれでも見つからなければホーム。

export type Resolved =
  | { name: "home"; gone?: Gone }
  | {
      name: "workspace";
      storeId: string;
      workspaceId: string;
      tab: WorkspaceTab;
      movedFrom?: MovedFrom;
    }
  | { name: "project"; storeId: string; projectId: string; tab: ProjectTab; movedFrom?: MovedFrom }
  | { name: "item"; storeId: string; projectId: string; itemId: string; movedFrom?: MovedFrom }
  | { name: "settings"; tab: SettingsTab };

const home = { name: "home" } as const;

// findWorkspace は、接頭辞 prefix のワークスペースを、stores のいまの接頭辞で探してから旧い接頭辞で探す。
function findWorkspace(
  stores: StoreNode[],
  prefix: string,
  pick: (s: StoreNode, w: WorkspaceNode) => Resolved | undefined,
): Resolved {
  for (const old of [false, true]) {
    for (const s of stores) {
      for (const w of s.workspaces) {
        if (!(old ? w.oldPrefixes.includes(prefix) : w.prefix === prefix)) continue;
        const r = pick(s, w);
        if (r) return r;
      }
    }
  }
  return home;
}

// findProject は、表示 ID のプロジェクト。旧い接頭辞のワークスペースでは、その接頭辞で書いた ID が同じものを選ぶ。
function findProject(stores: StoreNode[], display: string, tab: ProjectTab): Resolved {
  const d = parseDisplayId(display);
  if (d?.kind !== "project") return home;
  return findWorkspace(stores, d.prefix, (s, w) => {
    const p = w.projects.find((x) => projectDisplayId(d.prefix, x.number) === display);
    return p && { name: "project", storeId: s.id, projectId: p.id, tab };
  });
}

// askWorkspace は、一覧にないワークスペースのアドレスをサーバーに聞く(B5c-13)。
async function askWorkspace(
  store: string,
  prefix: string,
  tab: WorkspaceTab,
  transfers: Pick<TransfersPort, "locateWorkspace">,
): Promise<Resolved> {
  try {
    const l = await transfers.locateWorkspace(store, prefix);
    if (l.kind === "gone") {
      const { kind: _, ...gone } = l;
      return { name: "home", gone };
    }
    const { storeId, workspaceId, movedFrom } = l;
    return { name: "workspace", storeId, workspaceId, tab, ...(movedFrom ? { movedFrom } : {}) };
  } catch (e) {
    if (e instanceof NotFound) return home;
    throw e;
  }
}

// askProject は、一覧にないプロジェクトの ID をサーバーに聞く。読めない ID は聞かない。
async function askProject(
  store: string,
  id: string,
  tab: ProjectTab,
  transfers: Pick<TransfersPort, "locateProject">,
): Promise<Resolved> {
  if (parseDisplayId(id)?.kind !== "project") return home;
  try {
    const l = await transfers.locateProject(store, id);
    if (l.kind === "gone") {
      const { kind: _, ...gone } = l;
      return { name: "home", gone };
    }
    const { storeId, projectId, movedFrom } = l;
    return { name: "project", storeId, projectId, tab, ...(movedFrom ? { movedFrom } : {}) };
  } catch (e) {
    if (e instanceof NotFound) return home;
    throw e;
  }
}

async function findItem(
  stores: StoreNode[],
  store: string,
  id: string,
  items: ItemsPort,
  transfers: Pick<TransfersPort, "locate">,
): Promise<Resolved> {
  const d = parseDisplayId(id);
  if (d?.kind !== "item") return home;
  const s = stores.find((x) => x.name === store);
  const w = s?.workspaces.find((x) => x.prefix === d.prefix);
  if (s && w) {
    try {
      const it = await items.findByNumber(s.id, w.id, d.number);
      return { name: "item", storeId: s.id, projectId: it.projectId, itemId: it.id };
    } catch (e) {
      if (!(e instanceof NotFound)) throw e;
    }
  }
  return locate(store, id, transfers);
}

async function locate(
  store: string,
  id: string,
  transfers: Pick<TransfersPort, "locate">,
): Promise<Resolved> {
  try {
    const l = await transfers.locate(store, id);
    if (l.kind === "gone") {
      const { kind: _, ...gone } = l;
      return { name: "home", gone };
    }
    const { storeId, projectId, itemId, movedFrom } = l;
    return { name: "item", storeId, projectId, itemId, ...(movedFrom ? { movedFrom } : {}) };
  } catch (e) {
    if (e instanceof NotFound) return home;
    throw e;
  }
}

// askServer が false のときは、プロジェクトを一覧だけで探す(出していたプロジェクトのアドレスの読み直し。A5b-19)。
export async function resolveRoute(
  route: Route,
  stores: StoreNode[],
  items: ItemsPort,
  transfers: Pick<TransfersPort, "locate" | "locateProject" | "locateWorkspace">,
  askServer = true,
): Promise<Resolved> {
  switch (route.name) {
    case "home":
      return home;
    case "workspace": {
      const named = stores.filter((s) => s.name === route.store);
      const r = findWorkspace(named, route.prefix, (s, w) => ({
        name: "workspace",
        storeId: s.id,
        workspaceId: w.id,
        tab: route.tab,
      }));
      if (r.name !== "home" || !askServer) return r;
      return askWorkspace(route.store, route.prefix, route.tab, transfers);
    }
    case "project": {
      const named = stores.filter((s) => s.name === route.store);
      const r = findProject(named, route.project, route.tab);
      if (r.name !== "home" || !askServer) return r;
      return askProject(route.store, route.project, route.tab, transfers);
    }
    case "item":
      return findItem(stores, route.store, route.item, items, transfers);
    case "settings":
      return { name: "settings", tab: route.tab };
  }
}

// routeOfProject は、プロジェクトの画面のアドレス。一覧にないときはホーム。
export function routeOfProject(
  stores: StoreNode[],
  storeId: string,
  projectId: string,
  tab: ProjectTab,
): Route {
  const s = stores.find((x) => x.id === storeId);
  for (const w of s?.workspaces ?? []) {
    const p = w.projects.find((x) => x.id === projectId);
    if (s && p) return { name: "project", store: s.name, project: p.displayId, tab };
  }
  return home;
}

// routeOfWorkspace は、ワークスペースの画面のアドレス。一覧にないときはホーム。
export function routeOfWorkspace(
  stores: StoreNode[],
  storeId: string,
  workspaceId: string,
  tab: WorkspaceTab = "overview",
): Route {
  const s = stores.find((x) => x.id === storeId);
  const w = s?.workspaces.find((x) => x.id === workspaceId);
  return s && w ? { name: "workspace", store: s.name, prefix: w.prefix, tab } : home;
}
