import type { PaletteAlias, PalettePort } from "@hedgehug/core";
import { call } from "./errors";
import { listRecentItems, openRecentItem, paletteIndex } from "./generated";

// コマンドパレット(A5c)。生成物の型名は import しない(Go の出力型の名前に左右されないため)。null の一覧は空にする。

const aliases = (as: readonly PaletteAlias[] | null | undefined): PaletteAlias[] =>
  (as ?? []).map((a) => ({ id: a.id, at: a.at ?? null }));

export function createPalettePort(): PalettePort {
  return {
    index: () =>
      call(async () => {
        const r = await paletteIndex();
        return {
          items: (r.items ?? []).map((it) => ({
            storeId: it.storeId,
            workspaceId: it.workspaceId,
            projectId: it.projectId,
            itemId: it.itemId,
            number: it.number,
            display: it.display,
            title: it.title,
            projectTitle: it.projectTitle,
            aliases: aliases(it.aliases),
          })),
          projects: (r.projects ?? []).map((p) => ({
            storeId: p.storeId,
            workspaceId: p.workspaceId,
            projectId: p.projectId,
            display: p.display,
            title: p.title,
            workspaceName: p.workspaceName,
            aliases: aliases(p.aliases),
          })),
        };
      }),
    recent: () =>
      call(async () =>
        ((await listRecentItems()) ?? []).map((r) => ({
          storeId: r.storeId,
          itemId: r.itemId,
          display: r.display,
          title: r.title,
          projectTitle: r.projectTitle,
        })),
      ),
    opened: (storeId, itemId) => call(() => openRecentItem({ storeId, itemId })),
  };
}
