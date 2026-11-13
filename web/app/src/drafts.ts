import { createDrafts, isItemDraft } from "@hedgehug/core";

// 書きかけの登録簿。wire.ts(画面の移り方・詳細パネル)と tab.ts(反映を待つ条件)の両方が使うので、
// wire.ts の外に置く(tab.ts は wire.ts から読まれるので、tab.ts が wire.ts を読むと循環する)。
export const drafts = createDrafts();

// leaveItem は、詳細パネルを閉じる・別のアイテムを開く前の確認。アイテムの書きかけだけを見る
// (同時に開いているプロジェクトの概要の書きかけは、画面を離れるときにだけ聞く)。
export const leaveItem = () => drafts.confirmLeave(isItemDraft);
