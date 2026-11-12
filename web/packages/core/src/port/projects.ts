import type { NewProject, Project } from "../model/project";

// プロジェクトの口。実装は api パッケージにある。
export interface ProjectsPort {
  get(storeId: string, projectId: string): Promise<Project>;
  create(storeId: string, workspaceId: string, input: NewProject): Promise<Project>;
}
