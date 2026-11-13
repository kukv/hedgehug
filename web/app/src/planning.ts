import { createWorkspacesPort } from "@hedgehug/api";
import { createSprints } from "@hedgehug/core";
import { reloads } from "./reloads";
import { listMembers, planningPort } from "./wire";

// スプリントの画面(05「スプリントの画面」)。wire.ts が 300 行なので、ここで作る。
// カードはタブを開いているあいだだけ読み、Store の出来事で読み直す(wire.ts の reflect が reloads を呼ぶ)。
export const sprints = createSprints({
  port: planningPort,
  workspaces: createWorkspacesPort(),
  listMembers,
});
reloads.add(() => sprints.reload());
