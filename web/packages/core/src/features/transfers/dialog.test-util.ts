import type { Item } from "../../model/item";
import { get, writable } from "../../reactive/store";
import { createCatalog } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";
import type { DetailView } from "../items/detail";
import { item } from "../items/fixtures.test-util";
import { itemDisplayId } from "../router/route";
import { createToasts } from "../toasts/toasts";
import { createTransferDialog } from "./dialog";
import { fakeTransfers } from "./transfers.test-util";

// dialog.test.ts と dialogSubmit.test.ts の共通の準備。
// カタログは Store work(S1、書ける)と ro(S2、閲覧のみ)。候補は S1 の「Mobile」と S3(team)の「Team」。

// viewOf は、ダイアログが見る item と ancestors だけを持つ詳細パネルの表示。
export const viewOf = (it: Item, ancestors: Item[] = []) =>
  ({ item: it, ancestors }) as unknown as DetailView;

export async function setup() {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  const fake = fakeTransfers();
  fake.targetList = [
    {
      storeId: "S1",
      storeName: "work",
      workspaceId: "S1-W2",
      workspaceName: "Mobile",
      prefix: "MB",
      projects: [
        {
          id: "S1-W2-P1",
          title: "アプリ",
          epics: [{ id: "S1-W2-E1", display: "MB-1", title: "通知" }],
        },
        { id: "S1-W2-P2", title: "空", epics: [] },
      ],
    },
    {
      storeId: "S3",
      storeName: "team",
      workspaceId: "S3-W1",
      workspaceName: "Team",
      prefix: "TM",
      projects: [
        {
          id: "S3-W1-P1",
          title: "運営",
          epics: [{ id: "S3-W1-E1", display: "TM-4", title: "認可" }],
        },
      ],
    },
  ];
  const toastList = createToasts({ set: () => 0, clear: () => {} });
  const panel = writable<DetailView | null>(null);
  // confirms は confirmLeave に渡されたもの。env.allow が false なら「キャンセル」と答える。
  const confirms: unknown[] = [];
  const moved: string[] = [];
  const shown: boolean[] = [];
  const env = { online: true, refreshed: 0, allow: true };
  const dialog = createTransferDialog({
    transfers: fake,
    catalog,
    toasts: toastList,
    refresh: {
      afterChange: async () => {
        env.refreshed += 1;
      },
    },
    detail: { view: panel },
    drafts: {
      confirmLeave: async (only) => {
        confirms.push(only);
        return env.allow;
      },
    },
    online: () => env.online,
    afterMoved: async (from, to, panelShowed) => {
      moved.push(`${itemDisplayId("HH", from.number)} → ${to.toStore}/${to.itemId}`);
      shown.push(panelShowed);
    },
  });
  const task = item("S1", "S1-P1", 3, "Task", null, "決済");
  const epic = item("S1", "S1-P1", 1, "Epic", null, "認証");
  const toasts = () => get(toastList.list).map((t) => ({ text: t.text, error: t.error }));
  const toastTexts = () => toasts().map((t) => t.text);
  return { dialog, fake, panel, confirms, moved, shown, env, task, epic, toasts, toastTexts };
}
