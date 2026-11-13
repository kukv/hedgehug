import { NotFound } from "../../model/errors";
import type { Item } from "../../model/item";
import type { ItemsPort } from "../../port/items";
import { get } from "../../reactive/store";
import { createCatalog, type StoreNode } from "../catalog/catalog";
import { fakeCatalogPort } from "../catalog/fixtures.test-util";

// アドレスの解決のテストの共通の準備。stores は、テスト用の一覧(Store「work」と「ro」)。
export async function stores(): Promise<StoreNode[]> {
  const catalog = createCatalog(fakeCatalogPort());
  await catalog.load();
  return get(catalog.stores);
}

// itemsIn は、storeId の Store にだけ HH-12 があるアイテムの口。ほかの呼び出しは使わない。
export function itemsIn(storeId: string): ItemsPort {
  const unused = async (): Promise<never> => {
    throw new Error("unused");
  };
  return {
    list: unused,
    get: unused,
    create: unused,
    rename: unused,
    addStep: unused,
    setProperty: unused,
    addValue: unused,
    removeValue: unused,
    checkStep: unused,
    reorder: unused,
    nextNumber: unused,
    async findByNumber(s, w, n) {
      if (s !== storeId || n !== 12) throw new NotFound("not found");
      return { id: `${s}-I12`, storeId: s, workspaceId: w, projectId: `${s}-P1` } as Item;
    },
  };
}
