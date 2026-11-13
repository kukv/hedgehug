import type { Kind } from "../../model/item";
import type { PlanningOverview } from "../../model/planning";

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

// sprintChoices は、「バックログ」と完了していないスプリント(概要の並び。API が、実行中、計画中を開始日の
// 早い順・期間未定は最後・同じは名前の昇順、完了の順に並べている。F1-21)。いま入っている current が
// 完了済みなら、それも残す(完了は最後)。並べ直さない(名前の比べ方がサーバーとずれないように)。
// 削除済み・ないスプリントは概要にないので、選択肢に出ない(chosen が「バックログ」にする。F1-5)。
export function sprintChoices(o: PlanningOverview, current: string): PlanChoice[] {
  const usable = o.sprints
    .filter((s) => s.state !== "closed" || s.id === current)
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

// asOptions は、選択肢をセレクトの選択肢(value・label)にする。
export const asOptions = (choices: PlanChoice[]): { value: string; label: string }[] =>
  choices.map((c) => ({ value: c.id, label: c.label }));
