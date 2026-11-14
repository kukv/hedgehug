// パレットの「選んでいるアイテム」(02「コマンドパレット」、A5c-9)。画面の部品が rank を付けて登録し、
// current は rank の小さい順に、最初に null でないものを返す(詳細パネル 0、ツリー 1、バックログ 2)。

export interface SelectedItem {
  storeId: string;
  itemId: string;
  // source は、どの部品が選んでいるか("detail"・"tree"・"backlog")。
  source: "detail" | "tree" | "backlog" | string;
  setStatus(statusId: string): Promise<void>;
}

export interface Selection {
  // provide は、選んでいるアイテムの取り出し方を登録する。返す関数で登録を外す。
  provide(rank: number, get: () => SelectedItem | null): () => void;
  current(): SelectedItem | null;
}

export function createSelection(): Selection {
  const providers = new Set<{ rank: number; get: () => SelectedItem | null }>();
  return {
    provide(rank, get) {
      const p = { rank, get };
      providers.add(p);
      return () => {
        providers.delete(p);
      };
    },
    current() {
      const sorted = [...providers].sort((a, b) => a.rank - b.rank);
      for (const p of sorted) {
        const it = p.get();
        if (it) return it;
      }
      return null;
    },
  };
}
