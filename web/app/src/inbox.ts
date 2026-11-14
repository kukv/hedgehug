import { createInboxPort } from "@hedgehug/api";
import { createInbox, get, type InboxRow, type Route, routeOfWorkspace } from "@hedgehug/core";
import { inboxReloads } from "./reloads";
import { catalog, detail, navigation, toasts } from "./wire";

// 受信箱(06「受信箱」)。wire.ts が 300 行なので、ここで作る。
export const inbox = createInbox({ port: createInboxPort(), toasts });

// inbox の知らせで、サイドバーの未読数と、受信箱を開いていれば一覧を読み直す(C4a-13)。
inboxReloads.add(() => inbox.reloadUnread());
inboxReloads.add(async () => {
  if (get(navigation.route).name === "inbox") await inbox.reload();
});
// サイドバーの「受信箱」の未読数は、どの画面から始めても出す。
void inbox.reloadUnread();

export interface InboxLink {
  route: Route;
  // action は、ふつうのクリックで行うこと(通知を既読にしてから開く)。新しいタブで開くときは呼ばない。
  action: () => void;
}

// linkOf は、行の対象のリンク(C4a-8)。削除されたアイテム・ワークスペースはリンクにしない(null)。
export function linkOf(row: InboxRow): InboxLink | null {
  const t = row.target;
  const read = () => void inbox.markRead(row);
  switch (t.kind) {
    case "item":
    case "moved": {
      const route: Route = { name: "item", store: row.store.name, item: t.display };
      // 別の Store へ移ったアイテムは、旧 ID のアドレスを開き、転送記録から探す。
      const open =
        t.kind === "item"
          ? () => void detail.open(row.store.id, t.item)
          : () => navigation.go(route);
      return {
        route,
        action: () => {
          read();
          open();
        },
      };
    }
    case "workspace": {
      // ワークスペースの設定の区画(「自動化」)を開く(C4a-15)。
      const route = routeOfWorkspace(get(catalog.stores), row.store.id, t.workspace, "settings");
      return {
        route,
        action: () => {
          read();
          navigation.go(route, t.section);
        },
      };
    }
    default:
      return null;
  }
}
