import type { Item, Kind } from "../../model/item";
import type { WorkspaceSettings } from "../../model/workspace";
import type { CatalogPort } from "../../port/catalog";
import type { ItemsPort } from "../../port/items";
import type { PlanningPort } from "../../port/planning";
import type { WorkspacesPort } from "../../port/workspaces";
import type { Readable } from "../../reactive/store";
import type { Catalog } from "../catalog/catalog";
import type { Option } from "../projects/createDialog";
import type { Toasts } from "../toasts/toasts";
import type { StructureRow } from "../workspaces/templateApply";
import type { CreateContext, Here } from "./createDefaults";
import type { Values } from "./createValues";
import type { PropertyRow } from "./properties";

// アイテムの作成ダイアログの型(createDialog.ts が 300 行の上限に近いので分けた。createDialog.ts が再公開する)。

export interface ItemForm {
  workspaceId: string;
  projectId: string;
  kind: Kind;
  parentId: string | null;
  title: string;
  keepOpen: boolean;
  // values は、プロパティの内部 ID → 入力。隠した欄の値も覚えておく。
  values: Values;
  // templateId は、当てはめているテンプレート。空は「なし」。
  templateId: string;
  // description は説明の欄。templateDescription は、いま当てはめているテンプレートの説明(なければ空)。
  description: string;
  templateDescription: string;
  // sprint は、入れるスプリント。空は「バックログ」。Story・Task・Bug のときだけ送る(F1-21)。
  sprint: string;
}

export interface ItemDialogState {
  form: ItemForm;
  workspaces: Option[];
  projects: Option[];
  parents: Option[];
  // parentMissing は、親の候補がないときの赤字。Epic と、候補があるときは空。
  parentMissing: string;
  // preview は「ID は HH-13 になります。」。
  preview: string;
  // rows は、種類で使うプロパティの行。settings は、そのワークスペースの設定。
  rows: PropertyRow[];
  // templates は、テンプレートのセレクトの選択肢(使えるものがなければ空で、欄を出さない)。
  templates: Option[];
  // structure は「一緒に作るもの」。テンプレートが「なし」、または子も Step もないときは null。
  structure: { heading: string; rows: StructureRow[] } | null;
  // sprints は、スプリントのセレクトの選択肢(「バックログ」と完了していないスプリント)。
  sprints: Option[];
  settings: WorkspaceSettings;
  error: string;
}

export interface ItemDialog {
  state: Readable<ItemDialogState | null>;
  open(ctx?: CreateContext): Promise<void>;
  update(patch: Partial<ItemForm>): Promise<void>;
  submit(): Promise<void>;
  close(): void;
}

export interface ItemDialogDeps {
  catalog: Catalog;
  items: ItemsPort;
  workspaces: WorkspacesPort;
  catalogPort: CatalogPort;
  planning: PlanningPort;
  toasts: Toasts;
  here(): Here;
  openProjectDialog(workspaceId: string): void;
  // onCreated は、作ったあと。keptOpen は「続けて作成」がオンだったか。
  onCreated(item: Item, keptOpen: boolean): void;
}
