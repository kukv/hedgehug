import { Refused } from "../../model/errors";
import type { Item } from "../../model/item";
import type { PlanningOverview } from "../../model/planning";
import type { PlanningPort } from "../../port/planning";
import { chosen, releaseChoices, sprintChoices } from "../planning/choices";
import { sprintSaveFailed } from "../planning/sprintDialogs";
import type { Toasts } from "../toasts/toasts";

// 詳細パネルのスプリント・リリースの保存(02「スプリント・リリース」、F1-15)。detail.ts が 300 行の上限に近いので分けた。

// assignPlan は、アイテムのスプリント(key が "sprint")かリリースを id にする(空はバックログ・「なし」)。
// 表示している値と同じなら送らずに null(使えない参照を書き換えない。F1-5)。断られたらその文の、
// ほかの失敗(404・通信の失敗など)は「保存できませんでした」のエラーのトーストを出して false(F1-18)。
export async function assignPlan(
  deps: { planning: PlanningPort; toasts: Toasts },
  storeId: string,
  item: Item,
  plan: PlanningOverview,
  key: "sprint" | "release",
  id: string,
): Promise<boolean | null> {
  const choices = key === "sprint" ? sprintChoices : releaseChoices;
  if (id === chosen(choices(plan, item[key]), item[key])) return null;
  const write = key === "sprint" ? "setItemSprint" : "setItemRelease";
  try {
    await deps.planning[write](storeId, item.id, id || null);
  } catch (e) {
    deps.toasts.show(e instanceof Refused ? e.message : sprintSaveFailed, { error: true });
    return false;
  }
  return true;
}
