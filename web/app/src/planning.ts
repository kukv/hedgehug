import { createWorkspacesPort } from "@hedgehug/api";
import { createSprints, runSprintAction } from "@hedgehug/core";
import { reloads } from "./reloads";
import { listMembers, planning, planningPort, toasts } from "./wire";

// スプリントの画面(05「スプリントの画面」)。wire.ts が 300 行なので、ここで作る。
// カードはタブを開いているあいだだけ読み、Store の出来事で読み直す(wire.ts の reflect が reloads を呼ぶ)。
export const sprints = createSprints({
  port: planningPort,
  workspaces: createWorkspacesPort(),
  listMembers,
});
reloads.add(() => sprints.reload());

// sprintAction は、スプリントのダイアログの保存(core の runSprintAction)。保存のあとは、カードと概要
// (見出しの「実行中」、「開始」の理由、作成の既定)をすぐ読み直す。
export const sprintAction = (op: () => Promise<string | null>, onRefused: (m: string) => void) =>
  runSprintAction(
    {
      toasts,
      reload: async () => {
        await Promise.all([sprints.reload(), planning.reload()]);
      },
    },
    op,
    onRefused,
  );
