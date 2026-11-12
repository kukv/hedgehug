export type Kind = "Epic" | "Story" | "Task" | "Bug" | "Step";

export interface Item {
  id: string;
  storeId: string;
  workspaceId: string;
  projectId: string;
  // parentId は、親の内部 ID。Epic は null。
  parentId: string | null;
  number: number;
  kind: Kind;
  rank: string;
  title: string;
  watchers: string[];
  createdAt: string;
  createdBy: string;
}

export interface NewItem {
  projectId: string;
  parentId: string | null;
  kind: Kind;
  title: string;
}
