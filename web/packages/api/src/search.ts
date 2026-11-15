import type { ScreenStatesPort, SearchPort, SearchRow, SearchTableRequest } from "@hedgehug/core";
import { call } from "./errors";
import { screenStateGet, screenStatePut, search } from "./generated";

// 検索と画面の状態(E1)。生成物の型名は import しない。null の一覧は空にする。

// body は、table があるときだけ sort・group を本文に入れる(パレットは送らない)。
function body(query: string, table?: SearchTableRequest) {
  return table ? { query, sort: table.sort, group: table.group } : { query };
}

export type SearchRowDto = Awaited<ReturnType<typeof search>>["rows"][number];

// rowOf は、行 1 つを core の型にする(検索とマイタスクで使う)。
export function rowOf(x: SearchRowDto): SearchRow {
  return {
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
    statusId: x.statusId,
    assignee: x.assignee ?? null,
    priority: x.priority ?? null,
    end: x.end,
    points: x.points ?? null,
    oldIdHit: x.oldIdHit,
    readOnly: x.readOnly,
    deleted: x.deleted ?? null,
    start: x.start,
    estimate: x.estimate ?? null,
    systems: x.systems ?? [],
    sprint: x.sprint,
    release: x.release,
    project: { display: x.project.display, title: x.project.title },
    parent: x.parent ?? null,
    created: x.created,
    updated: x.updated,
    unused: x.unused ?? [],
    removed: x.removed ?? [],
    custom: x.custom ?? {},
    values: {
      priority: x.values.priority,
      systems: x.values.systems ?? [],
      choices: (x.values.choices ?? {}) as SearchRow["values"]["choices"],
      fields: (x.values.fields ?? {}) as SearchRow["values"]["fields"],
      sprint: x.values.sprint,
      release: x.values.release,
    },
  };
}

export function createSearchPort(): SearchPort {
  return {
    search: (query, table) =>
      call(async () => {
        const r = await search(body(query, table));
        return {
          errors: r.errors ?? [],
          hasKey: r.hasKey,
          hasSort: r.hasSort,
          group: r.group,
          deleted: r.deleted,
          columns: (r.columns ?? []).map((c) => ({
            key: c.key,
            name: c.name,
            type: c.type,
            sortable: c.sortable,
          })),
          groupBy: r.groupBy ?? "",
          groups: (r.groups ?? []).map((g) => ({ label: g.label, rows: g.rows ?? [] })),
          rows: (r.rows ?? []).map(rowOf),
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
