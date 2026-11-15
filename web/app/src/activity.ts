import { createActivityPort } from "@hedgehug/api";
import {
  type ActivityRow,
  type ActivityTarget,
  createActivity,
  get,
  type Route,
  resolveRoute,
} from "@hedgehug/core";
import { reloads, syncReloads } from "./reloads";
import { catalog, detail, itemsPort, navigation, toasts, transfersPort } from "./wire";

// アクティビティ(06「アクティビティ」)。wire.ts が 300 行なので、ここで作る。
export const activity = createActivity({ port: createActivityPort(), toasts });

// 出来事 store・sync で、アクティビティの画面を開いているときだけ読み直す(C4d-15)。
// store と sync が同時に来ると 2 回呼ぶが、古い読み直しの答えは createActivity が捨てる。
const reloadActivity = async () => {
  if (get(navigation.route).name === "activity") await activity.reload();
};
reloads.add(reloadActivity);
syncReloads.add(reloadActivity);

export interface ActivityLink {
  route: Route;
  // action は、ふつうのクリックで行うこと。新しいタブで開くときは呼ばない。
  action?: () => void;
}

// openCurrent は、いまの ID のアイテムを、画面を移らずに詳細パネルで開く(受信箱と同じ)。行は内部 ID を
// 持たないので、アドレスと同じ解決で探す。見つからない・読めないとき(読み直しのあいだに移ったなど)はアドレスを開き、
// アドレスの解決に任せる(消えていればトーストが出る)。
async function openCurrent(route: Route) {
  const r = await resolveRoute(route, get(catalog.stores), itemsPort, transfersPort).catch(
    () => null,
  );
  if (r?.name === "item" && !r.movedFrom) void detail.open(r.storeId, r.itemId);
  else navigation.go(route);
}

// activityLinkOf は、行のアイテム・プロジェクトの ID のリンク(C4d-15)。リンクにしないとき(削除済みなど)は null。
// アイテムの旧 ID はアドレスを開き、転送記録から探す。プロジェクトは概要を開く(旧 ID はプロジェクトの画面がいまの場所を探す)。
export function activityLinkOf(
  row: ActivityRow,
  t: ActivityTarget,
  kind: "item" | "project",
): ActivityLink | null {
  if (t.link === "") return null;
  const store = row.store.name;
  if (kind === "project")
    return { route: { name: "project", store, project: t.display, tab: "overview" } };
  const route: Route = { name: "item", store, item: t.display };
  return t.link === "current" ? { route, action: () => void openCurrent(route) } : { route };
}
