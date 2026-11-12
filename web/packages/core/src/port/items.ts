import type { Item, NewItem } from "../model/item";

// アイテムの口。実装は api パッケージにある。
export interface ItemsPort {
  // list は、プロジェクトのすべてのアイテムを番号の順で返す。
  list(storeId: string, projectId: string): Promise<Item[]>;
  get(storeId: string, itemId: string): Promise<Item>;
  create(storeId: string, input: NewItem): Promise<Item>;
  rename(storeId: string, itemId: string, title: string): Promise<Item>;
  addStep(storeId: string, parentId: string, title: string): Promise<Item>;
  moveUp(storeId: string, itemId: string): Promise<void>;
  // nextNumber は、ID の予告に出す番号(ワークスペースの連番の次)。
  nextNumber(storeId: string, workspaceId: string): Promise<number>;
  findByNumber(storeId: string, workspaceId: string, number: number): Promise<Item>;
}
