import type { Schemas } from "./api/client";
import { repairedKnownText } from "./notifications";

type Finding = Schemas["Finding"];
export type DoctorReport = Schemas["DoctorReport"];
export type PurgePlan = Schemas["PurgePlan"];

// The Japanese of the values of Finding.check (internal/doctor: checkOrder, and the workspace check).
const CHECKS: Record<string, string> = {
  schema: "スキーマ",
  template: "テンプレート",
  prop_name: "プロパティ名",
  override: "プロジェクトの設定",
  task: "タスク",
  file: "ファイル",
  view: "ビュー",
  worktree: "作業ツリー",
  merge_driver: "merge driver",
  inbox: "通知",
  git: "git",
  cross_store_ref: "Store をまたぐ参照",
  duplicate_across_stores: "Store をまたぐ重複",
  forward_target: "移動先",
  unexpected_force_push: "想定外の force push",
  workspace: "ワークスペース",
};

/** The name of a check; a check that is not known is written as it is. */
function checkName(check: string): string {
  return CHECKS[check] ?? check;
}

type FindingGroup = { check: string; name: string; items: Finding[] };

/** The findings by check, in the order their checks first come (the server sorts them by check). */
export function groupFindings(findings: readonly Finding[]): FindingGroup[] {
  const groups = new Map<string, FindingGroup>();
  for (const f of findings) {
    let g = groups.get(f.check);
    if (!g) {
      g = { check: f.check, name: checkName(f.check), items: [] };
      groups.set(f.check, g);
    }
    g.items.push(f);
  }
  return [...groups.values()];
}

export type Severity = "error" | "warning" | "info";

/** The severity of a finding; the server writes an error as "error", and so does an empty one here. */
export function severityOf(f: Finding): Severity {
  return f.severity === "warning" || f.severity === "info" ? f.severity : "error";
}

const rec = (v: unknown): Record<string, unknown> =>
  typeof v === "object" && v !== null ? (v as Record<string, unknown>) : {};
const str = (v: unknown): string => (typeof v === "string" ? v : "");

/** What a repair of doctor --fix did (Fixed.kind and Fixed.value), in words. */
export function fixedText(kind: string, value: unknown): string {
  switch (kind) {
    case "id_collision": {
      const v = rec(value);
      const [old, now] = [str(v.old), str(v.new)];
      return old && now
        ? `ID が重なっていたため、${old} から ${now} に振り直しました`
        : "ID が重なっていたため、振り直しました";
    }
    case "id_renumbered":
      return typeof value === "string" && value
        ? `ID を振り直しました（元の ID: ${value}）`
        : "ID を振り直しました";
    case "id_prefix":
      return "ID の接頭辞がワークスペースと違っていたため、振り直しました";
    case "workspace_mismatch":
      return "親と違うワークスペースにあったため、親のワークスペースに合わせました";
    case "workspace_revived":
      return "削除されていたワークスペースを復活させました";
  }
  // The kinds a sync repairs too are told as the inbox tells them.
  return repairedKnownText({ kind, value }) ?? kind;
}

// hh purge takes at most this many days (internal/ops/purge.go).
const MAX_PURGE_DAYS = 36500;

/** The days of "N 日より古いもの": a whole number from 0, or null when the input is not one. */
export function parsePurgeDays(input: string): number | null {
  const s = input.trim().normalize("NFKC");
  if (!/^[0-9]+$/.test(s)) return null;
  const n = Number(s);
  return n <= MAX_PURGE_DAYS ? n : null;
}
