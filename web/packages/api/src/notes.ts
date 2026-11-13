import type { NotesPort } from "@hedgehug/core";
import { call } from "./errors";
import {
  describeItem,
  describeProject,
  getItemDescription,
  getProjectDescription,
} from "./generated";

// 説明・コメント・作業時間・関連・履歴の口の実装。区画ごとのタスクが、自分のメソッドをここに足す。
export function createNotesPort(): NotesPort {
  return {
    description: (t) =>
      call(async () => {
        const read =
          t.kind === "item"
            ? await getItemDescription(t.storeId, t.id)
            : await getProjectDescription(t.storeId, t.id);
        return read.text;
      }),
    describe: (t, base, text) =>
      call(async () => {
        const body = { base, text };
        const saved =
          t.kind === "item"
            ? await describeItem(t.storeId, t.id, body)
            : await describeProject(t.storeId, t.id, body);
        return { text: saved.text, conflict: saved.conflict };
      }),
  };
}
