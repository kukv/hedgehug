import type { Kind } from "../model/item";
import type { NewProject, Project, ProjectStatus } from "../model/project";

// プロジェクトの口。実装は api パッケージにある。
export interface ProjectsPort {
  get(storeId: string, projectId: string): Promise<Project>;
  create(storeId: string, workspaceId: string, input: NewProject): Promise<Project>;
  // 概要の編集(02「概要の編集」)。1 項目を変えるごとに 1 回の保存。検査と断りはサーバーが行う。
  rename(storeId: string, projectId: string, title: string): Promise<Project>;
  setStatus(storeId: string, projectId: string, status: ProjectStatus): Promise<Project>;
  // setOwner の owner は、ログイン名。空は「なし」。
  setOwner(storeId: string, projectId: string, owner: string): Promise<Project>;
  // setDate の value は YYYY-MM-DD。空は「なし」。
  setDate(
    storeId: string,
    projectId: string,
    field: "start" | "end",
    value: string,
  ): Promise<Project>;
  addRepo(storeId: string, projectId: string, repo: string): Promise<Project>;
  removeRepo(storeId: string, projectId: string, repo: string): Promise<Project>;
  // プロジェクト別のルール(03「プロジェクト別のルール」)。1 回の操作が 1 回の保存。検査と断りはサーバーが行う。
  setStatusUsed(storeId: string, projectId: string, statusId: string, used: boolean): Promise<void>;
  setTransition(
    storeId: string,
    projectId: string,
    from: string,
    to: string,
    allowed: boolean,
  ): Promise<void>;
  setTemplateUsed(
    storeId: string,
    projectId: string,
    templateId: string,
    used: boolean,
  ): Promise<void>;
  // setTemplateDefault の value は、"" (ワークスペースの既定に戻す)・"@none"・テンプレートの ID。
  setTemplateDefault(storeId: string, projectId: string, kind: Kind, value: string): Promise<void>;
  setRuleRuns(storeId: string, projectId: string, ruleId: string, runs: boolean): Promise<void>;
  // resetRules は、区画をワークスペースの設定に戻す(自動化では専用ルールも消す)。
  resetRules(storeId: string, projectId: string, section: RulesSection): Promise<void>;
}

export type RulesSection = "statuses" | "workflow" | "templates" | "automation";
