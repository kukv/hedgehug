import type { Kind } from "./item";

// スプリントとリリース(05)。項目は API の DTO と同じ。日付は YYYY-MM-DD(空は値なし)。

export type SprintState = "planned" | "active" | "closed";
export type ReleaseState = "unreleased" | "released" | "archived";

export interface Totals {
  count: number;
  points: number;
}

export interface Sprint {
  id: string;
  name: string;
  goal: string;
  start: string;
  end: string;
  state: SprintState;
  // 完了で確定した値。完了していなければ null。
  confirmed: { committed: Totals; completed: Totals } | null;
}

export interface Release {
  id: string;
  name: string;
  date: string;
  description: string;
  state: ReleaseState;
}

// PlanRow は、カードのアイテムの行。status は値の ID、assignee はログイン名(空は未割り当て)。
export interface PlanRow {
  id: string;
  display: string;
  kind: Kind;
  title: string;
  status: string;
  assignee: string;
  points: number | null;
}

// BacklogRow は、バックログの画面の枠の行。priority は作業項目の順位、project はステータスのメニューを絞るのに使う。
export interface BacklogRow extends PlanRow {
  priority: string;
  project: string;
}

// BacklogFrame は、バックログの画面の枠。sprint が null ならバックログの枠。
export interface BacklogFrame {
  sprint: Sprint | null;
  total: Totals;
  rows: BacklogRow[];
}

export interface SprintCard {
  sprint: Sprint;
  total: Totals;
  done: Totals;
  // 未着手・進行中の件数。
  undone: number;
  canceled: number;
  items: PlanRow[];
}

export interface ReleaseCard {
  release: Release;
  target: number;
  done: number;
  undone: number;
  items: PlanRow[];
}

// SprintDefaults は、作成ダイアログの初めの値。
export interface SprintDefaults {
  name: string;
  start: string;
  end: string;
}

export interface PlanningOverview {
  active: Sprint | null;
  defaults: SprintDefaults;
  sprints: Sprint[];
  releases: Release[];
  // retentionDays は、その Store の削除済みの保持日数(スプリントの削除の確認の文)。
  retentionDays: number;
}

export interface SprintInput {
  name: string;
  goal: string;
  start: string;
  end: string;
}

export interface ReleaseInput {
  name: string;
  date: string;
  description: string;
}
