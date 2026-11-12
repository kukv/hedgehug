import type { WorkspaceProperty, WorkspaceSettings, WorkspaceValue } from "../../model/workspace";
import type { WorkspacesPort } from "../../port/workspaces";
import { fakeWorkspacesPort } from "../workspaces/fixtures.test-util";

const all = ["Epic", "Story", "Task", "Bug", "Step"];
const v = (id: string, name: string, category?: WorkspaceValue["category"]): WorkspaceValue =>
  category ? { id, name, color: "gray", category } : { id, name, color: "gray" };
const prop = (
  id: string,
  name: string,
  type: WorkspaceProperty["type"],
  extra: Partial<WorkspaceProperty> = {},
): WorkspaceProperty => ({
  id,
  key: id,
  name,
  type,
  builtin: true,
  kinds: all,
  hidden: false,
  choices: [],
  ...extra,
});

// テスト用の設定。終了日は非表示、ポイントは Story だけ、F1 は単一選択の追加プロパティ、F2 はテキストの追加プロパティ。
export const settings: WorkspaceSettings = {
  properties: [
    prop("status", "ステータス", "status"),
    prop("assignee", "担当者", "user"),
    prop("priority", "優先度", "priority"),
    prop("start", "開始日", "date"),
    prop("end", "終了日", "date", { hidden: true }),
    prop("points", "ポイント", "number", { kinds: ["Story"] }),
    prop("estimate", "見積もり時間", "duration"),
    prop("systems", "関連システム", "systems"),
    prop("F1", "環境", "single", {
      builtin: false,
      choices: [v("C1", "本番"), v("C2", "検証")],
    }),
    prop("F2", "メモ", "text", { builtin: false }),
  ],
  statuses: [
    v("st1", "Todo", "todo"),
    v("st2", "InProgress", "doing"),
    v("st3", "Done", "done"),
    v("st4", "Cancel", "canceled"),
  ],
  priorities: [v("pr1", "High"), v("pr2", "Medium")],
  systems: [v("sy1", "Web"), v("sy2", "API")],
  initialStatus: "st1",
  initialPriority: "pr2",
  categoryDefaults: { todo: "st1", doing: "st2", done: "st3", canceled: "st4" },
  workflow: {},
  templates: [],
  templateDefaults: {},
};

// settings2 は、別のワークスペースの設定(初期値が違う)。
export const settings2: WorkspaceSettings = {
  ...settings,
  statuses: [v("ns1", "新規", "todo"), v("ns2", "完了", "done")],
  priorities: [v("np1", "普通")],
  initialStatus: "ns1",
  initialPriority: "np1",
  categoryDefaults: { todo: "ns1", done: "ns2" },
};

// fakeWorkspaces は、get だけを byId(ワークスペースの内部 ID → 設定)で返す口。ないものは settings。
export function fakeWorkspaces(byId: Record<string, WorkspaceSettings> = {}): WorkspacesPort {
  return {
    ...fakeWorkspacesPort(),
    get: async (storeId, id) => ({
      id,
      storeId,
      name: "Web",
      prefix: "HH",
      oldPrefixes: [],
      settings: byId[id] ?? settings,
    }),
  };
}
