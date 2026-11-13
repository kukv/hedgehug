import type { LinksPort } from "@hedgehug/core";
import { call } from "./errors";
import { addLink, listLinks, listProjectLinks, refreshLinks, removeLink } from "./generated";

// createLinksPort は、PR・Issue のリンクの口(07)。生成された型が core の狭い型と同じ形なのでそのまま渡す。
export function createLinksPort(): LinksPort {
  return {
    list: (storeId, itemId) => call(async () => (await listLinks(storeId, itemId)) ?? []),
    refresh: (storeId, itemId) => call(async () => (await refreshLinks(storeId, itemId)) ?? []),
    add: (storeId, itemId, input) =>
      call(async () => {
        await addLink(storeId, itemId, input);
      }),
    remove: (storeId, itemId, target) =>
      call(async () => {
        await removeLink(storeId, itemId, {
          kind: target.kind,
          repo: target.repo,
          number: target.number,
        });
      }),
    listProject: (storeId, projectId) =>
      call(async () => (await listProjectLinks(storeId, projectId)) ?? []),
  };
}
