// 受信箱(トラック C4a)の core の export。index.ts が全部を再公開する(index.ts は 300 行の上限に近い)。

export {
  createInbox,
  emptyOf,
  type Inbox,
  type InboxDeps,
  type InboxState,
  inboxAppliedMessage,
  inboxEmptyTitle,
  inboxTabs,
  pillOf,
} from "./features/inbox/inbox";
export type {
  InboxAction,
  InboxCounts,
  InboxPage,
  InboxRow,
  InboxRowKind,
  InboxStore,
  InboxTab,
  InboxTarget,
  InboxTargetKind,
} from "./model/inbox";
export type { InboxCursor, InboxPort } from "./port/inbox";
