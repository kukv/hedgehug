// The inbox as the screen shows it: which tab an item is in, the order, and the sentence for an
// item, made from its kind and detail (the server sends no text; the words are the screen's).
import type { Schemas } from "./api/client";
import { formatDateTime } from "./format";

export type Notification = Schemas["Notification"];
export type InboxTab = "all" | "overwritten" | "mention" | "repair";

export const INBOX_TABS: readonly { tab: InboxTab; label: string }[] = [
  { tab: "all", label: "すべて" },
  { tab: "overwritten", label: "上書き" },
  { tab: "mention", label: "メンション" },
  { tab: "repair", label: "同期の修復" },
];

const REPAIR_KINDS: readonly string[] = [
  "repaired",
  "revived",
  "renumbered",
  "prefix_duplicate",
  "rewrite_applied",
  "purged_change",
];

/** The tabs an item of the kind is in. Every item is in "all"; assigned and watch only there. */
export function tabOf(kind: string): InboxTab[] {
  if (kind === "overwritten") return ["all", "overwritten"];
  if (kind === "mention") return ["all", "mention"];
  if (REPAIR_KINDS.includes(kind)) return ["all", "repair"];
  return ["all"];
}

export function filterByTab(items: Notification[], tab: InboxTab): Notification[] {
  return items.filter((n) => tabOf(n.kind).includes(tab));
}

/** The number of unread items in each tab. */
export function tabCounts(items: Notification[]): Record<InboxTab, number> {
  const counts: Record<InboxTab, number> = { all: 0, overwritten: 0, mention: 0, repair: 0 };
  for (const n of items) {
    if (n.read) continue;
    for (const t of tabOf(n.kind)) counts[t]++;
  }
  return counts;
}

/** The unread first; in each half the newest first, and the larger ULID first for the same time. */
export function sortNotifications(items: Notification[]): Notification[] {
  return [...items].sort((a, b) => {
    if (a.read !== b.read) return a.read ? 1 : -1;
    const t = Date.parse(b.time) - Date.parse(a.time);
    if (t !== 0) return t;
    return a.ulid < b.ulid ? 1 : a.ulid > b.ulid ? -1 : 0;
  });
}

/** The task of an item; a workspace's item (prefix_duplicate...) has none. */
export function taskOf(n: Notification): Notification["task"] | null {
  const t = n.task as Notification["task"] | null | undefined;
  return t?.ulid ? t : null;
}

/** The field an overwritten item is about (empty when its detail does not tell). */
export function overwrittenField(n: Notification): string {
  return str(obj(n.detail).field);
}

type Obj = Record<string, unknown>;
const obj = (v: unknown): Obj => (typeof v === "object" && v !== null ? (v as Obj) : {});
const str = (v: unknown): string => (typeof v === "string" ? v : "");

/** A JSON value in a short form: a string without quotes, a list with commas, null as 空. */
function valueText(v: unknown): string {
  if (v === null || v === undefined) return "空";
  if (typeof v === "string") return v;
  if (Array.isArray(v)) return v.map(valueText).join(", ");
  if (typeof v === "object") return JSON.stringify(v);
  return String(v);
}

export type NotificationText = { label: string; text: string; mine?: string };

const MOVED_AWAY = "移った先のワークスペースにはありません";

function repairedText(detail: Obj): string {
  const kind = str(detail.kind);
  const value = detail.value;
  switch (kind) {
    case "parent_moved":
      return "親が別の Store へ移ったため、ルートに移しました";
    case "ref_moved": {
      const field = str(obj(value).field);
      return field
        ? `${field} が指していたタスクが別の Store へ移ったため、参照を外しました`
        : "別の Store へ移ったタスクへの参照を外しました";
    }
    case "sprint_detached":
    case "release_detached": {
      const name = str(value);
      if (name) {
        const what = kind === "sprint_detached" ? "スプリント" : "リリース";
        return `${what} ${name} から外しました（${MOVED_AWAY}）`;
      }
      break;
    }
    case "parent_cycle": {
      const m = obj(value);
      if ("to" in m || "To" in m) {
        const to = m.to ?? m.To;
        return `親の循環を検出したため、${to ? "元の親" : "ルート"}に戻しました`;
      }
      break;
    }
    case "parent_deleted":
      return "親が削除されたため、ルートに移しました";
  }
  if (kind === "") return "同期の修復";
  return value === undefined || value === null
    ? `同期の修復: ${kind}`
    : `同期の修復: ${kind}（元の値: ${valueText(value)}）`;
}

/**
 * The label and the sentence of an item (the Japanese of what `hh inbox` says). workspaceName
 * gives a workspace its name from its ULID, for the items that are about a workspace.
 */
export function notificationText(
  n: Notification,
  opts: { workspaceName?: (ulid: string) => string | undefined } = {},
): NotificationText {
  const detail = obj(n.detail);
  const who = n.author ? `${n.author} が` : "";
  switch (n.kind) {
    case "overwritten": {
      const field = str(detail.field);
      if (!field) return { label: "上書き", text: "同時に編集した値で上書きされました" };
      return {
        label: "上書き",
        text: `あなたが設定した ${field} が、同時に編集した値で上書きされました`,
        mine: `${field}: ${valueText(detail.value)}（あなた）`,
      };
    }
    case "mention": {
      const where = str(detail.comment) ? "コメント" : "本文";
      return {
        label: "メンション",
        text: who
          ? `${who}${where}であなたをメンションしました`
          : `${where}であなたがメンションされました`,
      };
    }
    case "assigned":
      return {
        label: "割り当て",
        text: who ? `${who}あなたを担当者に追加しました` : "あなたが担当者に追加されました",
      };
    case "watch": {
      const changed = Array.isArray(detail.changed) ? detail.changed.map(String) : [];
      if (changed.length > 0) {
        const what = changed.join(", ");
        return {
          label: "ウォッチ",
          text: who ? `${who} ${what} を変更しました` : `${what} が変更されました`,
        };
      }
      return { label: "ウォッチ", text: who ? `${who}コメントしました` : "コメントがありました" };
    }
    case "repaired":
      return { label: "同期の修復", text: repairedText(detail) };
    case "revived":
      return {
        label: "同期の修復",
        text: "削除と同時にあなたが編集していたため、編集を優先して復活させました",
      };
    case "renumbered": {
      const old = str(detail.old);
      return {
        label: "同期の修復",
        text: `ID を ${old} から ${str(detail.new)} に振り直しました。${old} でも引けます`,
      };
    }
    case "prefix_duplicate": {
      const what = str(detail.kind) === "name_duplicate" ? "名前" : "接頭辞";
      const name = opts.workspaceName?.(str(detail.workspace));
      return {
        label: "同期の修復",
        text: `${name ? `ワークスペース ${name} の` : ""}${what}がほかのワークスペースと重なっています`,
      };
    }
    case "rewrite_applied":
      return { label: "同期の修復", text: "履歴の書き換え（purge）を取り込みました" };
    case "purged_change":
      return {
        label: "同期の修復",
        text: "完全に削除されたタスクへの、まだ送っていない変更を捨てました",
      };
  }
  return { label: n.kind, text: `種類: ${n.kind}` };
}

const pad = (n: number): string => String(n).padStart(2, "0");

/** When an item came, in the local time: 11:41 today, 昨日 18:20, then 11/02 10:12. */
export function formatWhen(iso: string, now: Date): string {
  const d = new Date(iso);
  const hm = `${pad(d.getHours())}:${pad(d.getMinutes())}`;
  const day = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diff = Math.round((day(now) - day(d)) / 86_400_000);
  if (diff === 0) return hm;
  if (diff === 1) return `昨日 ${hm}`;
  return formatDateTime(iso, now);
}
