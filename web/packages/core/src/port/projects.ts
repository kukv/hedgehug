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
}
