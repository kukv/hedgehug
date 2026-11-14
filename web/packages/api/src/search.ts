import type { ScreenStatesPort, SearchPort } from "@hedgehug/core";
import { call } from "./errors";
import { screenStateGet, screenStatePut, search } from "./generated";

// 検索と画面の状態(E1)。生成物の型名は import しない。null の一覧は空にする。

export function createSearchPort(): SearchPort {
  return {
    search: (query) =>
      call(async () => {
        const r = await search({ query });
        return {
          errors: r.errors ?? [],
          hasKey: r.hasKey,
          group: r.group,
          deleted: r.deleted,
          rows: (r.rows ?? []).map((x) => ({
            storeId: x.storeId,
            storeName: x.storeName,
            storeColor: x.storeColor,
            workspaceId: x.workspaceId,
            projectId: x.projectId,
            itemId: x.itemId,
            display: x.display,
            title: x.title,
            kind: x.kind,
            status: x.status ?? null,
            assignee: x.assignee ?? null,
            priority: x.priority ?? null,
            end: x.end,
            points: x.points ?? null,
            oldIdHit: x.oldIdHit,
            readOnly: x.readOnly,
            deleted: x.deleted ?? null,
          })),
        };
      }),
  };
}

export function createScreenStatesPort(): ScreenStatesPort {
  return {
    get: (screen) => call(async () => (await screenStateGet(screen)).body),
    put: (screen, body) => call(() => screenStatePut(screen, { body })),
  };
}
