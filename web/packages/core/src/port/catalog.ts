import type { Member } from "../model/member";
import type { Project } from "../model/project";
import type { StoreInfo } from "../model/store";
import type { Workspace } from "../model/workspace";

// 一覧の口(Store・ワークスペース・プロジェクト・メンバー・あなた)。実装は api パッケージにある。
// 失敗は、断りなら Refused、ないものなら NotFound、それ以外はそのまま投げる。
export interface CatalogPort {
  listStores(): Promise<StoreInfo[]>;
  listWorkspaces(storeId: string): Promise<Workspace[]>;
  listProjects(storeId: string, workspaceId: string): Promise<Project[]>;
  listMembers(storeId: string): Promise<Member[]>;
  // me は、その Store での「あなた」のログイン名。
  me(storeId: string): Promise<string>;
}
