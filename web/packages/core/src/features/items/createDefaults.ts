import type { Item, Kind } from "../../model/item";
import type { StoreNode, WorkspaceNode } from "../catalog/catalog";
import { writableWorkspaces } from "../catalog/catalog";

// アイテムの作成ダイアログの初期値と親の候補(02「開き方と初期値」「ダイアログの項目」)。

// CreateContext は、開いた場所が指定したもの(ボタン・行の「+」など)。
export interface CreateContext {
  workspaceId?: string;
  projectId?: string;
  kind?: Kind;
  parentId?: string | null;
}

// Here は、いま開いている画面のワークスペースとプロジェクト。
export interface Here {
  workspaceId?: string;
  projectId?: string;
}

export type Start =
  | { ok: true; workspace: WorkspaceNode; projectId: string; kind: Kind; parentId: string | null }
  | { ok: false; message: string; workspaceWithoutProject?: WorkspaceNode };

// startValues は、ワークスペース・プロジェクト・種類・親の初期値を決める。親は候補で選び直す(pickParent)。
// ワークスペースは、指定・いまのワークスペース・いまのプロジェクトのワークスペース・書ける先頭、の順で、
// 閲覧のみの Store のものは採らない。
export function startValues(stores: StoreNode[], ctx: CreateContext, here: Here): Start {
  const writable = writableWorkspaces(stores);
  const workspace =
    [ctx.workspaceId, here.workspaceId]
      .map((id) => writable.find((w) => w.id === id))
      .find((w) => w !== undefined) ?? writable[0];
  if (!workspace) {
    const onlyReadOnly = stores.length > 0 && stores.every((s) => s.readOnly);
    return {
      ok: false,
      message: onlyReadOnly
        ? "アイテムを作成できる Store がありません(閲覧のみの Store だけです)"
        : "先にワークスペースを作ってください",
    };
  }
  const inWs = (id?: string) => workspace.projects.find((p) => p.id === id)?.id;
  const projectId = inWs(ctx.projectId) ?? inWs(here.projectId) ?? workspace.projects[0]?.id;
  if (!projectId) {
    return {
      ok: false,
      message: `${workspace.name} にはプロジェクトがありません。先にプロジェクトを作ってください`,
      workspaceWithoutProject: workspace,
    };
  }
  return {
    ok: true,
    workspace,
    projectId,
    kind: ctx.kind ?? "Task",
    parentId: ctx.parentId ?? null,
  };
}

// parentCandidates は、親にできるもの。Step は Story・Task・Bug、Epic はなし、それ以外は Epic(番号の順)。
export function parentCandidates(items: Item[], kind: Kind): Item[] {
  if (kind === "Epic") return [];
  if (kind === "Step") return items.filter((it) => ["Story", "Task", "Bug"].includes(it.kind));
  return items.filter((it) => it.kind === "Epic");
}

// pickParent は、wanted が候補にあればそれ、なければ候補の先頭。Epic と候補がないときは null。
export function pickParent(candidates: Item[], wanted: string | null): string | null {
  if (wanted && candidates.some((it) => it.id === wanted)) return wanted;
  return candidates[0]?.id ?? null;
}

// parentMissing は、親の候補がないときに、セレクトの代わりに出す赤字(02「ダイアログの項目」)。
export function parentMissing(kind: Kind): string {
  return kind === "Step"
    ? "このプロジェクトには Story・Task・Bug がありません。先に作ってください"
    : "このプロジェクトには Epic がありません。先に作ってください";
}

// childKindOf は、行の「+」で作る種類(その行の下に置ける最初の種類。04「ツリー」の「行」)。Step は null。
export function childKindOf(kind: Kind): Kind | null {
  if (kind === "Epic") return "Story";
  if (kind === "Step") return null;
  return "Step";
}
