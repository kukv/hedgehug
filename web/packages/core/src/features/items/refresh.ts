import type { Item } from "../../model/item";
import { get } from "../../reactive/store";
import type { ProjectScreen } from "../projects/screen";
import type { Detail } from "./detail";

// アイテムを削除・移動・種類の変更・複製・並べ替えしたあとの読み直し。変更の応答の Item は画面に使わず
// (`blocked` がいつも false のため)、ツリーと詳細パネルと、変更で中身が変わる区画(関連・ブロック中)を読み直す。

export interface ItemRefreshDeps {
  detail: Pick<Detail, "view" | "close" | "reload">;
  projectScreen: Pick<ProjectScreen, "reload">;
  // sections は、詳細パネルの区画のうち、読み直すもの(app は関連の区画を渡す)。
  sections: { reload(): Promise<unknown> }[];
}

export interface ItemRefresh {
  // afterChange は、ツリー・詳細パネル・区画を読み直す。1 つが失敗しても残りは読み、失敗は呼んだ側に伝えない。
  afterChange(): Promise<void>;
  // afterDelete は、開いている詳細パネルが item のものなら閉じてから(確認は出さない)、afterChange と同じ読み直しをする。
  // item の下にあるものが開いているときは、読み直しの 404 で閉じる(02「詳細パネル」)。
  afterDelete(item: Item): Promise<void>;
}

export function createItemRefresh(deps: ItemRefreshDeps): ItemRefresh {
  async function afterChange() {
    await Promise.allSettled([
      deps.detail.reload(),
      deps.projectScreen.reload(),
      ...deps.sections.map((s) => s.reload()),
    ]);
  }
  return {
    afterChange,
    async afterDelete(item) {
      const open = get(deps.detail.view)?.item;
      if (open?.storeId === item.storeId && open.id === item.id) deps.detail.close();
      await afterChange();
    },
  };
}
