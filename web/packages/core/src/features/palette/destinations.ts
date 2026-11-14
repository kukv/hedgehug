import type { PaletteItem, PaletteProject, RecentItem } from "../../model/palette";
import { get } from "../../reactive/store";
import type { StoreNode, WorkspaceNode } from "../catalog/catalog";
import { routeOfProject, routeOfWorkspace } from "../router/resolve";
import type { Route } from "../router/route";
import { type ActionDeps, closing, moveTo } from "./actions";
import type { Candidate, GroupId } from "./candidates";
import type { Matched } from "./match";

// 行き先を開く候補: アイテム・最近開いたもの(詳細パネル)、プロジェクト・ワークスペース(概要)。
// どれも Ctrl+Enter で新しいタブに開ける(A5c-12)。

// openItem は、詳細パネルを開く候補(アイテム・最近開いたもの)。Ctrl+Enter では /s/{Store 名}/i/{いまの ID}。
function openItem(
  d: ActionDeps,
  stores: StoreNode[],
  group: GroupId,
  x: { storeId: string; itemId: string; display: string; title: string },
  hint: string,
): Candidate {
  const store = stores.find((s) => s.id === x.storeId);
  return {
    key: `${group}:${x.storeId}/${x.itemId}`,
    group,
    label: `${x.display} ${x.title}`,
    hint,
    ...(store ? { route: { name: "item", store: store.name, item: x.display } as Route } : {}),
    run: closing(d, () => d.detail.open(x.storeId, x.itemId)),
  };
}

export function itemCandidates(d: ActionDeps, matched: Matched<PaletteItem>[]): Candidate[] {
  const stores = get(d.catalog.stores);
  return matched.map((m) => openItem(d, stores, "item", m.c, m.hint));
}

export function recentCandidates(d: ActionDeps, recent: RecentItem[]): Candidate[] {
  const stores = get(d.catalog.stores);
  return recent.map((r) => openItem(d, stores, "recent", r, r.projectTitle));
}

export function projectCandidates(d: ActionDeps, matched: Matched<PaletteProject>[]): Candidate[] {
  const stores = get(d.catalog.stores);
  return matched.map(({ c, hint }) => ({
    ...moveTo(
      d,
      `project:${c.storeId}/${c.projectId}`,
      "project",
      `${c.display} ${c.title}`,
      routeOfProject(stores, c.storeId, c.projectId, "overview"),
    ),
    hint,
  }));
}

export function workspaceCandidates(d: ActionDeps, matched: Matched<WorkspaceNode>[]): Candidate[] {
  const stores = get(d.catalog.stores);
  return matched.map(({ c, hint }) => ({
    ...moveTo(
      d,
      `workspace:${c.storeId}/${c.id}`,
      "workspace",
      c.name,
      routeOfWorkspace(stores, c.storeId, c.id),
    ),
    hint,
  }));
}
