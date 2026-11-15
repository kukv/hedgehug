import type { ViewsPort } from "@hedgehug/core";
import { call } from "./errors";
import {
  createView,
  deleteView,
  duplicateView,
  listViews,
  renameView,
  saveView,
  viewKeyImpact,
} from "./generated";

// 保存したビュー(E3b)。生成物の型名は import しない。

export function createViewsPort(): ViewsPort {
  return {
    list: () => call(async () => (await listViews()).views ?? []),
    create: (name, content) => call(() => createView({ name, content })),
    save: (id, content) => call(() => saveView(id, { content })),
    rename: (id, name) => call(() => renameView(id, { name })),
    duplicate: (id) => call(() => duplicateView(id)),
    remove: (id) => call(() => deleteView(id)),
    keyImpact: (storeId, workspaceId, key) =>
      call(async () => (await viewKeyImpact({ storeId, workspaceId, key })).affected),
  };
}
