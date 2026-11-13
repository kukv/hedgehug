import type { Kind } from "../../model/item";
import type { PlanningOverview, Sprint } from "../../model/planning";

// アイテムのスプリント・リリースの選択肢(02「スプリント・リリース」、05、F1-5・F1-15・F1-21)。
// 詳細パネルと作成ダイアログが使う。id の空はバックログ・「なし」。

export interface PlanChoice {
  id: string;
  label: string;
}

// showSprint は、スプリントの欄を出す種類(Story・Task・Bug)。showRelease は、リリースの欄を出す種類(Step 以外)。
export const showSprint = (kind: Kind): boolean =>
  kind === "Story" || kind === "Task" || kind === "Bug";
export const showRelease = (kind: Kind): boolean => kind !== "Step";

const stateRank = { active: 0, planned: 1, closed: 2 } as const;

// compareSprints は、実行中、計画中を開始日の早い順(期間未定は最後)、同じは名前の昇順(F1-21)。
// 残した完了済みのものは最後。
function compareSprints(a: Sprint, b: Sprint): number {
  if (a.state !== b.state) return stateRank[a.state] - stateRank[b.state];
  if (a.start !== b.start) {
    if (a.start === "") return 1;
    if (b.start === "") return -1;
    return a.start < b.start ? -1 : 1;
  }
  if (a.name === b.name) return 0;
  return a.name < b.name ? -1 : 1;
}

// sprintChoices は、「バックログ」と完了していないスプリント。いま入っている current が完了済みなら、それも残す。
// 削除済み・ないスプリントは概要にないので、選択肢に出ない(chosen が「バックログ」にする。F1-5)。
export function sprintChoices(o: PlanningOverview, current: string): PlanChoice[] {
  const usable = o.sprints
    .filter((s) => s.state !== "closed" || s.id === current)
    .sort(compareSprints)
    .map((s) => ({ id: s.id, label: s.state === "active" ? `${s.name}(実行中)` : s.name }));
  return [{ id: "", label: "バックログ" }, ...usable];
}

// releaseChoices は、「なし」とアーカイブでないリリース(概要の並び)。いま入っている current がアーカイブなら、それも残す。
export function releaseChoices(o: PlanningOverview, current: string): PlanChoice[] {
  const usable = o.releases
    .filter((r) => r.state !== "archived" || r.id === current)
    .map((r) => ({ id: r.id, label: r.name }));
  return [{ id: "", label: "なし" }, ...usable];
}

// chosen は、選択肢に出す値。current が選択肢になければ、バックログ・「なし」(空)。
export const chosen = (choices: PlanChoice[], current: string): string =>
  choices.some((c) => c.id === current) ? current : "";
