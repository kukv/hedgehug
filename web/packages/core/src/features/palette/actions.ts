import type { Item } from "../../model/item";
import type { PaletteItem, PaletteProject, RecentItem } from "../../model/palette";
import { get, type Readable } from "../../reactive/store";
import { type StoreNode, type WorkspaceNode, writableWorkspaces } from "../catalog/catalog";
import { readOnlyNote } from "../catalog/readonly";
import { childKindOf, type Here } from "../items/createDefaults";
import type { ItemDialog } from "../items/createDialogTypes";
import type { Detail } from "../items/detail";
import type { ProjectDialog } from "../projects/createDialog";
import type { Navigation } from "../router/navigation";
import { routeOfProject, routeOfWorkspace } from "../router/resolve";
import type { Route } from "../router/route";
import { syncAllHint } from "../sync/card";
import type { SyncState } from "../sync/sync";
import type { Toasts } from "../toasts/toasts";
import type { TransferDialog } from "../transfers/dialog";
import type { WorkspaceDialog } from "../workspaces/createDialog";
import type { Candidate, GroupId } from "./candidates";
import type { Matched } from "./match";
import type { PlanKind, PlanRequest } from "./planRequest";

// コマンドパレットの候補と、選んだときの動き(02「コマンドパレット」の実行の表、A5c-10〜A5c-12・A5c-14)。
// ダイアログを開く候補・画面を移す候補は、先にパレットを閉じてから呼ぶ(A5c-16)。

export interface ActionDeps {
  catalog: { stores: Readable<StoreNode[]> };
  navigation: Pick<Navigation, "go">;
  itemDialog: Pick<ItemDialog, "open">;
  projectDialog: Pick<ProjectDialog, "open">;
  workspaceDialog: Pick<WorkspaceDialog, "open">;
  transferDialog: Pick<TransferDialog, "open">;
  detail: Pick<Detail, "open" | "target">;
  sync: Pick<SyncState, "syncAll">;
  planRequest: Pick<PlanRequest, "ask">;
  toasts: Pick<Toasts, "show">;
  // here は、いま開いている画面のワークスペースとプロジェクト。
  here(): Here;
  mac: boolean;
  // close は、パレットを閉じる(パレットが渡す)。
  close(): void;
}

// closing は、パレットを閉じてから f を呼ぶ実行。
const closing =
  (d: ActionDeps, f: () => unknown): (() => Promise<void>) =>
  async () => {
    d.close();
    await f();
  };

// moveTo は、画面を移す候補(Ctrl+Enter では同じ行き先を新しいタブで開く。A5c-12)。
function moveTo(d: ActionDeps, key: string, group: GroupId, label: string, route: Route) {
  return { key, group, label, route, run: closing(d, () => d.navigation.go(route)) };
}

function findWorkspace(stores: StoreNode[], id?: string) {
  for (const store of stores) {
    const workspace = store.workspaces.find((w) => w.id === id);
    if (workspace) return { store, workspace };
  }
  return undefined;
}

// commandCandidates は、コマンドの群(行き先がいまあるもの。持ち場と境目)。
export function commandCandidates(d: ActionDeps): Candidate[] {
  const g = "command";
  return [
    {
      key: "command:new",
      group: g,
      label: "新規作成",
      hint: "C",
      run: closing(d, () => d.itemDialog.open()),
    },
    moveTo(d, "command:settings", g, "設定を開く", { name: "settings", tab: "accounts" }),
    {
      key: "command:sync",
      group: g,
      label: "すべての Store を同期",
      hint: syncAllHint(d.mac),
      run: closing(d, () => d.sync.syncAll()),
    },
    {
      key: "command:workspace",
      group: g,
      label: "ワークスペースを追加",
      run: closing(d, () => d.workspaceDialog.open()),
    },
  ];
}

// createProject は、プロジェクトの作成ダイアログを開く。書き込めるワークスペースがなければ startValues と同じ順の文で
// 断る(A5c-14)。
function createProject(d: ActionDeps, stores: StoreNode[], here: Here) {
  if (writableWorkspaces(stores).length > 0) return d.projectDialog.open(here.workspaceId);
  const onlyReadOnly = stores.length > 0 && stores.every((s) => s.readOnly);
  d.toasts.show(
    onlyReadOnly
      ? "プロジェクトを作成できる Store がありません(閲覧のみの Store だけです)"
      : "先にワークスペースを作ってください",
    { error: true },
  );
}

// plan は、スプリント・リリースのタブへ移り、作成ダイアログを開く要求を置く。閲覧のみの Store では置かずに断る(A5c-11)。
function plan(
  d: ActionDeps,
  stores: StoreNode[],
  store: StoreNode,
  ws: WorkspaceNode,
  kind: PlanKind,
) {
  d.navigation.go(
    routeOfWorkspace(stores, store.id, ws.id, kind === "sprint" ? "sprints" : "releases"),
  );
  if (store.readOnly) d.toasts.show(readOnlyNote);
  else d.planRequest.ask({ workspaceId: ws.id, kind });
}

// createCandidates は、作成の群。スプリント・リリースは、ワークスペース・プロジェクトの画面のときだけ。
export function createCandidates(d: ActionDeps): Candidate[] {
  const stores = get(d.catalog.stores);
  const here = d.here();
  const g = "create";
  const list: Candidate[] = [
    {
      key: "create:project",
      group: g,
      label: "プロジェクトを作成",
      run: closing(d, () => createProject(d, stores, here)),
    },
  ];
  const at = findWorkspace(stores, here.workspaceId);
  if (!at) return list;
  for (const [kind, label] of [
    ["sprint", "スプリントを作成"],
    ["release", "リリースを作成"],
  ] as const) {
    list.push({
      key: `create:${kind}`,
      group: g,
      label,
      run: closing(d, () => plan(d, stores, at.store, at.workspace, kind)),
    });
  }
  return list;
}

// settingsCandidates は、設定の群。ワークスペース・プロジェクトは、その画面のときだけ。
export function settingsCandidates(d: ActionDeps): Candidate[] {
  const stores = get(d.catalog.stores);
  const here = d.here();
  const g = "settings";
  const list = [
    moveTo(d, "settings:accounts", g, "設定: アカウントとメンバー", {
      name: "settings",
      tab: "accounts",
    }),
  ];
  const at = findWorkspace(stores, here.workspaceId);
  if (!at) return list;
  const { store, workspace } = at;
  list.push(
    moveTo(
      d,
      "settings:workspace",
      g,
      `設定: ワークスペース(${workspace.name})`,
      routeOfWorkspace(stores, store.id, workspace.id, "settings"),
    ),
  );
  const project = workspace.projects.find((p) => p.id === here.projectId);
  if (project) {
    list.push(
      moveTo(
        d,
        "settings:project",
        g,
        `設定: プロジェクト(${project.title})`,
        routeOfProject(stores, store.id, project.id, "settings"),
      ),
    );
  }
  return list;
}

// selectedCandidates は、選んでいるアイテムの群(A5c-10)。toStatus は「ステータスを変更…」(パレットの中の一覧に切り替える)。
export function selectedCandidates(
  d: ActionDeps,
  it: Item,
  readOnly: boolean,
  toStatus: () => Promise<void>,
): Candidate[] {
  const g = "selected";
  const t = d.detail.target();
  const shown = t?.storeId === it.storeId && t.itemId === it.id;
  const detail: Candidate[] = shown
    ? []
    : [
        {
          key: "selected:detail",
          group: g,
          label: "詳細を開く",
          run: closing(d, () => d.detail.open(it.storeId, it.id)),
        },
      ];
  if (readOnly) return detail;
  const list: Candidate[] = [
    { key: "selected:status", group: g, label: "ステータスを変更…", run: toStatus },
  ];
  const child = childKindOf(it.kind);
  if (child) {
    const ctx = {
      workspaceId: it.workspaceId,
      projectId: it.projectId,
      kind: child,
      parentId: it.id,
    };
    list.push({
      key: "selected:child",
      group: g,
      label: `${child} を追加`,
      run: closing(d, () => d.itemDialog.open(ctx)),
    });
  }
  list.push(...detail);
  if (it.kind !== "Step") {
    list.push({
      key: "selected:transfer",
      group: g,
      label: "別のワークスペースへ移動…",
      run: closing(d, () => d.transferDialog.open(it)),
    });
  }
  return list;
}

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
