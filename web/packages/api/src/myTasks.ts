import type { MyTasksPort } from "@hedgehug/core";
import { call } from "./errors";
import { myTasks } from "./generated";
import { rowOf } from "./search";

// マイタスク(E4a)。null の一覧は空にする。
export function createMyTasksPort(): MyTasksPort {
  return {
    list: (filter) =>
      call(async () => {
        const r = await myTasks({ filter });
        return { errors: r.errors ?? [], rows: (r.rows ?? []).map(rowOf) };
      }),
  };
}
