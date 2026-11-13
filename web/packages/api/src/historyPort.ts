import type { NotesPort } from "@hedgehug/core";
import { call } from "./errors";
import { getItemHistory, getProjectHistory } from "./generated";

export type HistoryCalls = Pick<NotesPort, "history">;

// historyCalls は、NotesPort の履歴のメソッド(createNotesPort の戻り値に広げる)。
export function historyCalls(): HistoryCalls {
  return {
    history: (target) =>
      call(
        async () =>
          (target.kind === "item"
            ? await getItemHistory(target.storeId, target.id)
            : await getProjectHistory(target.storeId, target.id)) ?? [],
      ),
  };
}
