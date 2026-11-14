import type { InboxPort } from "@hedgehug/core";
import { call } from "./errors";
import {
  applyInboxNotice,
  inboxUnread,
  keepInboxNotice,
  listInbox,
  readAllInbox,
  setInboxRead,
} from "./generated";

// 受信箱の口の実装。続きの印は before_at と before_id を必ず両方付ける(サーバーは片方だけを断る)。
export function createInboxPort(): InboxPort {
  return {
    list: (tab, before) =>
      call(async () => {
        const body = await listInbox(
          before ? { tab, before_at: before.at, before_id: before.id } : { tab },
        );
        return { rows: body.rows ?? [], more: body.more, unread: body.unread };
      }),
    unread: () => call(async () => (await inboxUnread()).counts),
    setRead: (id, read) => call(() => setInboxRead(id, { read })),
    readAll: (tab) => call(() => readAllInbox({ tab })),
    apply: (id) => call(() => applyInboxNotice(id)),
    keep: (id) => call(() => keepInboxNotice(id)),
  };
}
