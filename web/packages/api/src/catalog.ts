import type { CatalogPort } from "@hedgehug/core";
import { call } from "./errors";
import { getMe, listMembers, listProjects, listStores, listWorkspaces } from "./generated";
import { toProject } from "./projects";

// 一覧の口の実装。API の型を core の型に直す(設計書 6 節)。一覧の null は空にする。
export function createCatalogPort(): CatalogPort {
  return {
    listStores: () =>
      call(async () =>
        ((await listStores()) ?? []).map((s) => ({
          id: s.id,
          name: s.name,
          color: s.color,
          readOnly: s.readOnly,
        })),
      ),
    listWorkspaces: (storeId) =>
      call(async () =>
        ((await listWorkspaces(storeId)) ?? []).map((w) => ({
          id: w.id,
          storeId,
          name: w.name,
          prefix: w.prefix,
          oldPrefixes: w.oldPrefixes ?? [],
        })),
      ),
    listProjects: (storeId, workspaceId) =>
      call(async () =>
        ((await listProjects(storeId, workspaceId)) ?? []).map((p) => toProject(storeId, p)),
      ),
    listMembers: (storeId) =>
      call(async () =>
        ((await listMembers(storeId)) ?? []).map((m) => ({ login: m.login, name: m.name })),
      ),
    me: (storeId) => call(async () => (await getMe(storeId)).login),
  };
}
