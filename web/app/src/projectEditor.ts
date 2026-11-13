import { createProjectsPort } from "@hedgehug/api";
import { createProjectEditor } from "@hedgehug/core";
import { catalog, projectScreen, toasts } from "./wire";

// プロジェクトの概要の編集(wire.ts が 300 行なので、ここで作る)。
export const projectEditor = createProjectEditor({
  projects: createProjectsPort(),
  toasts,
  // 保存したら、一覧(サイドバーと見出しはここから出る)と開いているプロジェクトを読み直す。
  onChanged: async () => {
    await catalog.load();
    await projectScreen.reload();
  },
});
