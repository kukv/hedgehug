import type { GoneReason, Located, TransfersPort } from "@hedgehug/core";
import { call } from "./errors";
import { getTransferPreview, listTransferTargets, locateItem, transferItem } from "./generated";

// 別のワークスペース・別の Store への移動(A5a)。生成物の型名は import しない(Go の出力型の名前に左右されないため)。
// null の一覧は空にする。

// Answer は、locate-item の応答(OpenAPI では kind 以外がどれも省略できる)。
type Answer = Awaited<ReturnType<typeof locateItem>>;

const reasons: readonly string[] = ["deleted", "purged", "hidden", "elsewhere"];

// badResponse は、kind に要る値が欠けた応答(サーバーの誤り。断りでも「ない」でもない)。
const badResponse = () => new Error("locate-item: 応答に要る値がありません");

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
  if (a.kind === "gone") {
    if (!a.reason || !reasons.includes(a.reason) || !a.old || !a.at || !a.newDisplay)
      throw badResponse();
    const g: Located = {
      kind: "gone",
      reason: a.reason as GoneReason,
      old: a.old,
      at: a.at,
      newDisplay: a.newDisplay,
    };
    if (a.storeName) g.storeName = a.storeName;
    return g;
  }
  throw badResponse();
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
      call(async () => {
        const r = await transferItem(storeId, itemId, input);
        return {
          toStore: r.toStore,
          itemId: r.itemId,
          display: r.display,
          emptied: r.emptied ?? [],
        };
      }),
    locate: (storeName, id) =>
      call(async () => located(await locateItem({ store: storeName, id }))),
  };
}
