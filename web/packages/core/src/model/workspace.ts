export interface Workspace {
  id: string;
  storeId: string;
  name: string;
  prefix: string;
}

export type Category = "todo" | "doing" | "done" | "canceled";

export type PropertyType =
  | "status"
  | "user"
  | "priority"
  | "date"
  | "number"
  | "duration"
  | "systems"
  | "text"
  | "single"
  | "multi"
  | "check"
  | "url";

export interface WorkspaceValue {
  id: string;
  name: string;
  color: string;
  category?: Category;
}

export interface WorkspaceProperty {
  id: string;
  key: string;
  name: string;
  type: PropertyType;
  builtin: boolean;
  kinds: string[];
  hidden: boolean;
  choices: WorkspaceValue[];
}

export interface WorkspaceSettings {
  properties: WorkspaceProperty[];
  statuses: WorkspaceValue[];
  priorities: WorkspaceValue[];
  systems: WorkspaceValue[];
  initialStatus: string;
  initialPriority: string;
  categoryDefaults: Partial<Record<Category, string>>;
}

export interface WorkspaceDetail extends Workspace {
  oldPrefixes: string[];
  settings: WorkspaceSettings;
}

export interface WorkspaceUsage {
  items: number;
  firstNumbers: number[];
  // 値の ID ごと・追加プロパティの ID ごとの件数
  values: Record<string, number>;
  properties: Record<string, number>;
}

// ValueList は、"statuses"・"priorities"・"systems" か、選択肢のプロパティの ID。
export type ValueList = string;

export interface NewProperty {
  name: string;
  key: string;
  type: PropertyType;
  kinds: string[];
}
