import { createDrafts } from "@hedgehug/core";

// 書きかけの登録簿。wire.ts(画面の移り方・詳細パネル)と tab.ts(反映を待つ条件)の両方が使うので、
// wire.ts の外に置く(tab.ts は wire.ts から読まれるので、tab.ts が wire.ts を読むと循環する)。
export const drafts = createDrafts();
