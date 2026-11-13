import { createHistory, type NotesPort, writable } from "@hedgehug/core";

// 履歴の区画の状態(wire.ts が 300 行に近いので、ここで作る)。詳細パネルとプロジェクトの概要は同時に
// 開くので、種類ごとに別々に持つ。descriptionSaves は、説明を保存した回数(アイテムの再読み込みは
// 起きないので、履歴の区画が説明の保存のあとに読み直すための印)。
export function createNoteHistory(notes: Pick<NotesPort, "history">) {
  const itemHistory = createHistory({ notes });
  const projectHistory = createHistory({ notes });
  return {
    itemHistory,
    projectHistory,
    // lists は、画面への反映(wire.ts の reflect)で読み直すもの。
    lists: [itemHistory, projectHistory],
    descriptionSaves: writable(0),
  };
}
