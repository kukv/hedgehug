import { createPalette, createPlanRequest, createSelection } from "@hedgehug/core";
import { hereOf } from "./here";
import { isMac } from "./tab";
import {
  catalog,
  detail,
  itemDialog,
  itemsPort,
  links,
  navigation,
  projectDialog,
  screen,
  sync,
  toasts,
  workspaceDialog,
  workspacesPort,
} from "./wire";
import { itemTransfer } from "./wireActions";
import { palettePort } from "./wireBase";

// コマンドパレット(02「コマンドパレット」、A5c)。wire.ts が 300 行なので、ここで組み立てる。

// selection は、パレットの「選んでいるアイテム」の出どころ(A5c-9)。詳細パネルは 0、ツリーは 1
// (ProjectTree.svelte が出ているあいだ登録する)。
export const selection = createSelection();
selection.provide(0, () => {
  const t = detail.target();
  if (!t) return null;
  return {
    ...t,
    source: "detail",
    // メニューで変えるのと同じ経路(A5c-10)。
    setStatus: async (statusId) => {
      await detail.setProperty(t.itemId, "status", statusId);
    },
  };
});

// planRequest は、スプリント・リリースの作成の要求(A5c-11)。計画のタブ(SprintsTab・ReleasesTab)が受け取る。
export const planRequest = createPlanRequest();

export const palette = createPalette({
  catalog,
  navigation,
  itemDialog,
  projectDialog,
  workspaceDialog,
  transferDialog: itemTransfer,
  detail,
  sync,
  planRequest,
  toasts,
  here: () => hereOf(screen, catalog),
  mac: isMac(),
  port: palettePort,
  items: itemsPort,
  workspaces: workspacesPort,
  selection,
  route: navigation.route,
  openTab: (route) => links.openTab(route),
  now: () => new Date(),
});
