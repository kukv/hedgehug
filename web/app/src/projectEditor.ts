import { createProjectsPort } from "@hedgehug/api";
import { createProjectEditor, createRulesEditor } from "@hedgehug/core";
import { catalog, projectScreen, toasts } from "./wire";

// プロジェクトの概要の編集(wire.ts が 300 行なので、ここで作る)。
export const projectsPort = createProjectsPort();

// 保存したら、一覧(サイドバーと見出しはここから出る)と開いているプロジェクトを読み直す。
const onChanged = async () => {
  await catalog.load();
  await projectScreen.reload();
};

export const projectEditor = createProjectEditor({ projects: projectsPort, toasts, onChanged });

// プロジェクト別のルール(使うステータス・遷移の制約・テンプレート・自動化)の編集。
export const rulesEditor = createRulesEditor({ toasts, onChanged });
