import type { Kind, PropertyInput } from "../../model/item";
import type { WorkspaceSettings } from "../../model/workspace";
import { visibleProperties } from "./properties";

// 作成ダイアログの値(02「ダイアログの中での連動」のテンプレート以外、決定 25)。values はプロパティの
// 内部 ID → 入力。隠した欄の値も覚えておく(使える種類に戻すと値が戻る)。

export type Values = Record<string, PropertyInput>;

const kept = ["assignee", "start", "end", "points", "estimate"];

export function initialValues(settings: WorkspaceSettings): Values {
  return { status: settings.initialStatus, priority: settings.initialPriority };
}

// switchWorkspace は、ワークスペースを変えたときの値。担当者・開始日・終了日・ポイント・見積もり時間を残し、
// ほかは初期値に戻す。担当者は member(login) が true のときだけ残す。
export function switchWorkspace(
  values: Values,
  next: WorkspaceSettings,
  member: (login: string) => boolean,
): Values {
  const out = initialValues(next);
  for (const k of kept) {
    const v = values[k];
    if (v === undefined || v === "") continue;
    if (k === "assignee" && !member(String(v))) continue;
    out[k] = v;
  }
  return out;
}

// shownValues は、その種類で表示している欄の値だけ(「作成」で送るもの)。空の値は送らない。
export function shownValues(values: Values, settings: WorkspaceSettings, kind: Kind): Values {
  const out: Values = {};
  for (const p of visibleProperties(settings, kind)) {
    const v = values[p.id];
    if (v === undefined || v === "" || (Array.isArray(v) && v.length === 0)) continue;
    out[p.id] = v;
  }
  return out;
}
