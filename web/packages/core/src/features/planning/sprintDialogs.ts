import { Refused } from "../../model/errors";
import type { PlanningOverview, Sprint, SprintCard, SprintInput } from "../../model/planning";
import type { Toasts } from "../toasts/toasts";
import { pt } from "./sprints";

// スプリントの作成・編集・開始・完了・削除のダイアログ(05「スプリント」)。入力の検査はサーバーが断る(F1-8)。
// 断りの文はダイアログの中に出し、ダイアログは閉じない。成功のトーストは F1-18。

export const sprintCreated = (name: string) => `${name} を作成しました`;
export const sprintStarted = (name: string) => `${name} を開始しました`;
export const sprintClosed = (name: string) => `${name} を完了しました`;
export const sprintDeleted = "スプリントを削除しました";
// 断りでない失敗(通信の失敗など)のエラーのトースト(F1-18)。
export const sprintSaveFailed = "保存できませんでした";

// newSprintForm は、作成ダイアログの初めの値(PlanningOverview.defaults)。ゴールは空。
export function newSprintForm(o: PlanningOverview): SprintInput {
  return { name: o.defaults.name, goal: "", start: o.defaults.start, end: o.defaults.end };
}

// sprintForm は、編集ダイアログの初めの値。
export function sprintForm(s: Sprint): SprintInput {
  return { name: s.name, goal: s.goal, start: s.start, end: s.end };
}

// startBody は、開始のダイアログの本文。件数とポイントは中止を除く(API が除いて数えている)。
export function startBody(c: SprintCard): string {
  return `${c.total.count} 件・${pt(c.total.points)} pt で始めます。`;
}

// addDays は、YYYY-MM-DD の n 日後。時刻帯に左右されないよう UTC の日で数える。
function addDays(ymd: string, n: number): string {
  const [y, m, d] = ymd.split("-").map(Number);
  return new Date(Date.UTC(y ?? 0, (m ?? 1) - 1, (d ?? 1) + n)).toISOString().slice(0, 10);
}

// startDefaults は、開始のダイアログの日付の既定。開始日は今日、終了日はスプリントの終了日が今日以降なら
// その日、なければ今日の 13 日後。today は、このマシンの時刻帯での今日(YYYY-MM-DD)。
export function startDefaults(s: Sprint, today: string): { start: string; end: string } {
  return { start: today, end: s.end !== "" && s.end >= today ? s.end : addDays(today, 13) };
}

// closeBody は、完了のダイアログの本文。中止があれば、集計に含めないことを続ける。
export function closeBody(c: SprintCard): string {
  const base = `完了 ${c.done.count} 件・未完了 ${c.undone} 件です。`;
  if (c.canceled === 0) return base;
  return `${base}中止 ${c.canceled} 件はスプリントに残り、集計には含めません。`;
}

export interface CloseDestination {
  // id は、計画中のスプリントの ID。null はバックログ。
  id: string | null;
  label: string;
}

// closeDestinations は、未完了のアイテムの移動先。計画中のスプリントを開始日の早い順(期間未定は最後。
// 同じ開始日は概要の並びのまま)に並べ、最後に「バックログ」。closing は完了するスプリントの ID。
export function closeDestinations(o: PlanningOverview, closing: string): CloseDestination[] {
  const planned = o.sprints
    .filter((s) => s.state === "planned" && s.id !== closing)
    .sort((a, b) => {
      if (a.start === b.start) return 0;
      if (a.start === "") return 1;
      if (b.start === "") return -1;
      return a.start < b.start ? -1 : 1;
    });
  return [...planned.map((s) => ({ id: s.id, label: s.name })), { id: null, label: "バックログ" }];
}

// closeDefault は、移動先の既定。計画中のスプリントがあれば開始日が最も早いもの、なければバックログ(null)。
export function closeDefault(o: PlanningOverview, closing: string): string | null {
  return closeDestinations(o, closing)[0]?.id ?? null;
}

// keptDestination は、概要を読み直したあとの移動先。選んでいたもの(null はバックログ)がまだ選択肢にあれば
// そのまま、なければ既定に戻す(削除・開始された計画中のスプリントを選んだままにしない)。
export function keptDestination(
  o: PlanningOverview,
  closing: string,
  current: string | null,
): string | null {
  const list = closeDestinations(o, closing);
  return list.some((d) => d.id === current) ? current : (list[0]?.id ?? null);
}

// deleteBody は、削除の確認ダイアログの本文(05「削除」)。items は入っているアイテムの数(完了・中止を含む)、
// retentionDays は、その Store の削除済みの保持日数。
export function deleteBody(name: string, items: number, retentionDays: number): string {
  const inside =
    items > 0
      ? `入っている ${items} 件はスプリントから外れます。未完了のものはバックログに戻ります。`
      : "";
  return `「${name}」を削除します。${inside}設定の「削除済み」から戻せます。${retentionDays} 日を過ぎると完全に削除されます。`;
}

export interface SprintActionDeps {
  toasts: Pick<Toasts, "show">;
  // reload は、スプリントのカードと概要を読み直す。
  reload(): Promise<void>;
}

// runSprintAction は、ダイアログの保存を 1 つ行う。op は成功のトーストの文(なければ null)を返す。
// 断りは onRefused に渡し(ダイアログの中に出す)、ほかの失敗はエラーのトーストにする。成功なら true。
export async function runSprintAction(
  deps: SprintActionDeps,
  op: () => Promise<string | null>,
  onRefused: (message: string) => void,
): Promise<boolean> {
  try {
    const toast = await op();
    if (toast) deps.toasts.show(toast);
  } catch (e) {
    if (e instanceof Refused) {
      onRefused(e.message);
      return false;
    }
    deps.toasts.show(sprintSaveFailed, { error: true });
    await deps.reload();
    return false;
  }
  await deps.reload();
  return true;
}
