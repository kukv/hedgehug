import { createWorkspacesPort } from "@hedgehug/api";
import { createBacklog, createReleases, createSprints, runSprintAction } from "@hedgehug/core";
import { propertySaved, reloads } from "./reloads";
import { detail, itemsPort, listMembers, planning, planningPort, toasts } from "./wire";

// スプリントの画面(05「スプリントの画面」)。wire.ts が 300 行なので、ここで作る。
// カードはタブを開いているあいだだけ読み、Store の出来事で読み直す(wire.ts の reflect が reloads を呼ぶ)。
export const sprints = createSprints({
  port: planningPort,
  workspaces: createWorkspacesPort(),
  listMembers,
});
reloads.add(() => sprints.reload());

// バックログの画面(05「バックログ」)。読み方はスプリントの画面と同じ。詳細パネルの保存の応答で、
// この画面から開いたアイテムが消えるときのトーストを出す(F2a-10・F2a-11)。
export const backlog = createBacklog({
  port: planningPort,
  items: itemsPort,
  workspaces: createWorkspacesPort(),
  listMembers,
  toasts,
  openDetail: (storeId, itemId) => void detail.open(storeId, itemId),
});
reloads.add(() => backlog.reload());
propertySaved.add((storeId, propertyId, item) => backlog.noticeDetail(storeId, propertyId, item));

// sprintAction は、スプリントのダイアログの保存(core の runSprintAction)。保存のあとは、カード・バックログの枠と
// 概要(見出しの「実行中」、「開始」の理由、作成の既定)をすぐ読み直す。
export const sprintAction = (op: () => Promise<string | null>, onRefused: (m: string) => void) =>
  runSprintAction(
    {
      toasts,
      reload: async () => {
        await Promise.all([sprints.reload(), backlog.reload(), planning.reload()]);
      },
    },
    op,
    onRefused,
  );

// リリースの画面(05「リリース」)。読み方はスプリントの画面と同じ。
export const releases = createReleases({
  port: planningPort,
  workspaces: createWorkspacesPort(),
  listMembers,
});
reloads.add(() => releases.reload());

// releaseAction は、リリースの保存(core の runSprintAction)。保存のあとは、カードと概要(アイテムの
// 「リリース」の選択肢)をすぐ読み直す。
export const releaseAction = (op: () => Promise<string | null>, onRefused: (m: string) => void) =>
  runSprintAction(
    {
      toasts,
      reload: async () => {
        await Promise.all([releases.reload(), planning.reload()]);
      },
    },
    op,
    onRefused,
  );
