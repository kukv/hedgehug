import type { LinkInput, LinkRow, LinkTarget, ProjectLinkRow, RepoFailure } from "../model/link";

// リンクの口。実装は api パッケージにある。断りは Refused、ないものは NotFound で投げる。
export interface LinksPort {
  list(storeId: string, itemId: string): Promise<LinkRow[]>;
  refresh(storeId: string, itemId: string): Promise<LinkRow[]>;
  add(storeId: string, itemId: string, input: LinkInput): Promise<void>;
  remove(storeId: string, itemId: string, target: LinkTarget): Promise<void>;
  listProject(storeId: string, projectId: string): Promise<ProjectLinkRow[]>;
  listRepoFailures(storeId: string, projectId: string): Promise<RepoFailure[]>;
}
