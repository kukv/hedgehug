import {
  type Gone,
  type Located,
  type LocatedProject,
  type NotCarriedKind,
  type TransfersPort,
  TransferUndoFailed,
} from "@hedgehug/core";
import { call } from "./errors";
import { ApiError } from "./fetcher";
import {
  getProjectTransferPreview,
  getTransferPreview,
  listProjectTransferTargets,
  listTransferTargets,
  locateItem,
  locateProject,
  transferItem,
  transferProject,
} from "./generated";

// 別のワークスペース・別の Store への移動(A5a)。生成物の型名は import しない(Go の出力型の名前に左右されないため)。
// null の一覧は空にする。

// Answer・ProjectAnswer は、locate-item・locate-project の応答(OpenAPI では kind 以外がどれも省略できる)。
type Answer = Awaited<ReturnType<typeof locateItem>>;
type ProjectAnswer = Awaited<ReturnType<typeof locateProject>>;

const reasons: readonly string[] = ["deleted", "purged", "hidden", "elsewhere"];

// badResponse は、kind に要る値が欠けた応答(サーバーの誤り。断りでも「ない」でもない)。
const badResponse = (op = "locate-item") => new Error(`${op}: 応答に要る値がありません`);

// located は、応答を kind で found・gone に分け、それぞれに要る値が揃っているかを確かめる。
function located(a: Answer): Located {
  if (a.kind === "found") {
    if (!a.storeId || !a.projectId || !a.itemId) throw badResponse();
    const f: Located = {
      kind: "found",
      storeId: a.storeId,
      projectId: a.projectId,
      itemId: a.itemId,
    };
    if (a.movedFrom) {
      const { old, at, display, current } = a.movedFrom;
      f.movedFrom = { old, at, display, current };
    }
    return f;
  }
  if (a.kind === "gone") return gone(a, badResponse);
  throw badResponse();
}

// gone は、gone の応答に要る値が揃っているかを確かめる(アイテムとプロジェクトで同じ形)。
function gone(a: Answer | ProjectAnswer, bad: () => Error): { kind: "gone" } & Gone {
  if (!a.reason || !reasons.includes(a.reason) || !a.old || !a.at || !a.newDisplay) throw bad();
  const base = { kind: "gone", old: a.old, at: a.at, newDisplay: a.newDisplay } as const;
  // hidden・elsewhere は文に Store 名を書くので、storeName のない応答は不正とする。
  if (a.reason === "hidden" || a.reason === "elsewhere") {
    if (!a.storeName) throw bad();
    return { ...base, reason: a.reason, storeName: a.storeName };
  }
  const reason = a.reason as "deleted" | "purged";
  return a.storeName ? { ...base, reason, storeName: a.storeName } : { ...base, reason };
}

// locatedProject は、locate-project の応答を found・gone に分ける(located のプロジェクト版。itemId はない)。
function locatedProject(a: ProjectAnswer): LocatedProject {
  const bad = () => badResponse("locate-project");
  if (a.kind === "found") {
    if (!a.storeId || !a.projectId) throw bad();
    const f: LocatedProject = { kind: "found", storeId: a.storeId, projectId: a.projectId };
    if (a.movedFrom) {
      const { old, at, display, current } = a.movedFrom;
      f.movedFrom = { old, at, display, current };
    }
    return f;
  }
  if (a.kind === "gone") return gone(a, bad);
  throw bad();
}

// undoFailed は、移動の 409(移動元から取り除けず取り消しもできなかった。detail は 02「保存」の文の全文)。
async function undoFailed<T>(f: () => Promise<T>): Promise<T> {
  try {
    return await f();
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) throw new TransferUndoFailed(e.message);
    throw e;
  }
}

export function createTransfersPort(): TransfersPort {
  return {
    targets: (storeId, itemId) =>
      call(async () =>
        ((await listTransferTargets(storeId, itemId)) ?? []).map((t) => ({
          storeId: t.storeId,
          storeName: t.storeName,
          workspaceId: t.workspaceId,
          workspaceName: t.workspaceName,
          prefix: t.prefix,
          projects: (t.projects ?? []).map((p) => ({
            id: p.id,
            title: p.title,
            epics: (p.epics ?? []).map((e) => ({ id: e.id, display: e.display, title: e.title })),
          })),
        })),
      ),
    preview: (storeId, itemId, to) =>
      call(async () => {
        const p = await getTransferPreview(storeId, itemId, {
          toStore: to.storeId,
          toWorkspace: to.workspaceId,
          toProject: to.projectId,
        });
        return {
          below: p.below,
          deleted: p.deleted,
          nextDisplay: p.nextDisplay,
          crossStore: p.crossStore,
          remoteNotice: p.remoteNotice,
          destinationHasRemote: p.destinationHasRemote,
          replacements: (p.replacements ?? []).map((r) => ({
            status: r.status,
            name: r.name,
            count: r.count,
            choices: (r.choices ?? []).map((c) => ({ id: c.id, name: c.name })),
            default: r.default,
          })),
        };
      }),
    transfer: (storeId, itemId, input) =>
      call(() =>
        undoFailed(async () => {
          const r = await transferItem(storeId, itemId, input);
          return {
            toStore: r.toStore,
            itemId: r.itemId,
            display: r.display,
            emptied: r.emptied ?? [],
          };
        }),
      ),
    locate: (storeName, id) =>
      call(async () => located(await locateItem({ store: storeName, id }))),
    projectTargets: (storeId, projectId) =>
      call(async () =>
        ((await listProjectTransferTargets(storeId, projectId)) ?? []).map((t) => ({
          storeId: t.storeId,
          storeName: t.storeName,
          workspaceId: t.workspaceId,
          workspaceName: t.workspaceName,
        })),
      ),
    projectPreview: (storeId, projectId, to) =>
      call(async () => {
        const p = await getProjectTransferPreview(storeId, projectId, {
          toStore: to.storeId,
          toWorkspace: to.workspaceId,
        });
        return {
          items: p.items,
          deleted: p.deleted,
          prefix: p.prefix,
          source: p.source,
          destination: p.destination,
          crossStore: p.crossStore,
          remoteNotice: p.remoteNotice,
          destinationHasRemote: p.destinationHasRemote,
          hasRules: p.hasRules,
          // kind はサーバーが A5b-8 の 6 種類だけを返す。
          notCarried: (p.notCarried ?? []).map((n) => ({
            kind: n.kind as NotCarriedKind,
            name: n.name,
            missing: n.missing ?? [],
            count: n.count,
          })),
          replacements: (p.replacements ?? []).map((r) => ({
            status: r.status,
            name: r.name,
            count: r.count,
            choices: (r.choices ?? []).map((c) => ({ id: c.id, name: c.name })),
            default: r.default,
          })),
        };
      }),
    transferProject: (storeId, projectId, input) =>
      call(() =>
        undoFailed(async () => {
          const r = await transferProject(storeId, projectId, input);
          return {
            toStore: r.toStore,
            projectId: r.projectId,
            display: r.display,
            destination: r.destination,
          };
        }),
      ),
    locateProject: (storeName, id) =>
      call(async () => locatedProject(await locateProject({ store: storeName, id }))),
  };
}
