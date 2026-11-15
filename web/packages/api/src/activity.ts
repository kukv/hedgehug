import type { ActivityPort } from "@hedgehug/core";
import { call } from "./errors";
import { listActivity } from "./generated";

// アクティビティの口の実装。ワークスペースの値「{storeId}/{workspaceId}」は、クエリの store と workspace に分ける。
// 続きの印は、サーバーが返した cursor をそのまま before に渡す。
export function createActivityPort(): ActivityPort {
  return {
    list: ({ workspace, kind }, before) =>
      call(async () => {
        const slash = workspace.indexOf("/");
        const body = await listActivity({
          ...(workspace && {
            store: workspace.slice(0, slash),
            workspace: workspace.slice(slash + 1),
          }),
          kind,
          ...(before && { before }),
        });
        return { rows: body.rows ?? [], more: body.more };
      }),
  };
}
