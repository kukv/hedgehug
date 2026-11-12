import { NotFound } from "../../model/errors";
import type { ItemsPort } from "../../port/items";
import type { StoreNode } from "../catalog/catalog";
import { type ProjectTab, parseDisplayId, type Route, type SettingsTab } from "./route";

// アドレス(Store 名と画面の ID)を内部 ID にする(01「画面のアドレス」、A2 の計画の決定 7)。
// アドレスの Store 名の Store を先に探し、なければサイドバーの並びの順にほかの Store を探す。
// 見つからなければホーム。旧い接頭辞(B1)と転送の記録(A5)はたどらない。

export type Resolved =
  | { name: "home" }
  | { name: "workspace"; storeId: string; workspaceId: string }
  | { name: "project"; storeId: string; projectId: string; tab: ProjectTab }
  | { name: "item"; storeId: string; projectId: string; itemId: string }
  | { name: "settings"; tab: SettingsTab };

const home = { name: "home" } as const;

function inOrder(stores: StoreNode[], name: string): StoreNode[] {
  return [...stores.filter((s) => s.name === name), ...stores.filter((s) => s.name !== name)];
}

async function findItem(
  stores: StoreNode[],
  store: string,
  id: string,
  items: ItemsPort,
): Promise<Resolved> {
  const d = parseDisplayId(id);
  if (d?.kind !== "item") return home;
  for (const s of inOrder(stores, store)) {
    const w = s.workspaces.find((x) => x.prefix === d.prefix);
    if (!w) continue;
    try {
      const it = await items.findByNumber(s.id, w.id, d.number);
      return { name: "item", storeId: s.id, projectId: it.projectId, itemId: it.id };
    } catch (e) {
      if (!(e instanceof NotFound)) throw e;
    }
  }
  return home;
}

export async function resolveRoute(
  route: Route,
  stores: StoreNode[],
  items: ItemsPort,
): Promise<Resolved> {
  switch (route.name) {
    case "home":
      return home;
    case "workspace":
      for (const s of inOrder(stores, route.store)) {
        const w = s.workspaces.find((x) => x.prefix === route.prefix);
        if (w) return { name: "workspace", storeId: s.id, workspaceId: w.id };
      }
      return home;
    case "project":
      for (const s of inOrder(stores, route.store)) {
        for (const w of s.workspaces) {
          const p = w.projects.find((x) => x.displayId === route.project);
          if (p) return { name: "project", storeId: s.id, projectId: p.id, tab: route.tab };
        }
      }
      return home;
    case "item":
      return findItem(stores, route.store, route.item, items);
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
export function routeOfWorkspace(stores: StoreNode[], storeId: string, workspaceId: string): Route {
  const s = stores.find((x) => x.id === storeId);
  const w = s?.workspaces.find((x) => x.id === workspaceId);
  return s && w ? { name: "workspace", store: s.name, prefix: w.prefix } : home;
}
