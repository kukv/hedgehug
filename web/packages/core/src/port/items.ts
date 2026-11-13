import type { Item, NewItem, PropertyInput } from "../model/item";

// アイテムの口。実装は api パッケージにある。
export interface ItemsPort {
  // list は、プロジェクトのすべてのアイテムを番号の順で返す。
  list(storeId: string, projectId: string): Promise<Item[]>;
  get(storeId: string, itemId: string): Promise<Item>;
  create(storeId: string, input: NewItem): Promise<Item>;
  rename(storeId: string, itemId: string, title: string): Promise<Item>;
  addStep(storeId: string, parentId: string, title: string): Promise<Item>;
  // setProperty は、プロパティの値を 1 つ変える(空は ""・null、複数でないプロパティ用)。
  setProperty(
    storeId: string,
    itemId: string,
    propertyId: string,
    value: PropertyInput,
  ): Promise<Item>;
  // addValue・removeValue は、複数の値(関連システムなど)を 1 つずつ入り切りする。
  addValue(storeId: string, itemId: string, propertyId: string, valueId: string): Promise<Item>;
  removeValue(storeId: string, itemId: string, propertyId: string, valueId: string): Promise<Item>;
  // checkStep は、Step のチェックを入れ切りする(ステータスが変わる)。
  checkStep(storeId: string, itemId: string, done: boolean): Promise<Item>;
  // reorder は、アイテムを、同じ親の兄弟 anchorId の直前(after なら直後)に置く。動くのは itemId の 1 件だけ。
  reorder(storeId: string, itemId: string, anchorId: string, after: boolean): Promise<void>;
  // nextNumber は、ID の予告に出す番号(ワークスペースの連番の次)。
  nextNumber(storeId: string, workspaceId: string): Promise<number>;
  findByNumber(storeId: string, workspaceId: string, number: number): Promise<Item>;
}
