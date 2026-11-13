import type { Member } from "../model/member";
import type { Project } from "../model/project";
import type { StoreInfo } from "../model/store";
import type { Workspace } from "../model/workspace";

// 一覧の口(Store・ワークスペース・プロジェクト・メンバー・あなた)。実装は api パッケージにある。
// 失敗は、断りなら Refused、ないものなら NotFound、それ以外はそのまま投げる。
export interface CatalogPort {
  listStores(): Promise<StoreInfo[]>;
  // listWorkspaces の oldPrefixes は、旧い接頭辞(画面のアドレスの旧い接頭辞のため。A5a-21)。
  listWorkspaces(storeId: string): Promise<(Workspace & { oldPrefixes?: string[] })[]>;
  listProjects(storeId: string, workspaceId: string): Promise<Project[]>;
  listMembers(storeId: string): Promise<Member[]>;
  // me は、その Store での「あなた」のログイン名。
  me(storeId: string): Promise<string>;
}
