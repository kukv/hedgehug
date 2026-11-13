import type { Item } from "../../model/item";
import type { TransferPreview, TransferTarget } from "../../model/transfer";
import type { Option } from "../projects/createDialog";
import { changeRows, noEpicMessage, transferIntro } from "./dialogText";

// 「別のワークスペースへ移動…」のダイアログの状態の形と、元の値から状態を作る関数(dialog.ts が使う)。

// ReplacementRow は、見出し「ステータスの置き換え」の下の 1 行。label は「{name}({count} 件)」。
export interface ReplacementRow {
  status: string;
  name: string;
  label: string;
  options: Option[];
  value: string;
}

export interface TransferDialogState {
  item: Item;
  displayId: string;
  // workspaces の値は `{Store の ID}/{ワークスペースの ID}`、見え方は「{Store 名} / {ワークスペース名}」。
  workspaces: Option[];
  projects: Option[];
  parents: Option[] | null; // Epic を移すときは null(親を選ばない)
  form: { workspace: string; project: string; parent: string };
  loading: boolean; // 予告を読んでいる間。intro・changes・replacements は空
  intro: string; // 冒頭の文(1〜2 文)
  noEpic: string; // "" か「このプロジェクトには Epic がありません」
  changes: string[]; // 見出し「移動先で変わること」の下の行
  replacements: ReplacementRow[]; // 見出し「ステータスの置き換え」の下の行(なければ空で、見出しも出さない)
  canSubmit: boolean;
  busy: boolean;
}

// Current は、開いているダイアログの元の値(表示の文字列は render で作る)。
export interface Current {
  item: Item;
  displayId: string;
  targets: TransferTarget[];
  target: TransferTarget;
  form: { workspace: string; project: string; parent: string };
  preview: TransferPreview | null; // 読んでいる間は null
  choices: Record<string, string>; // 元のステータスの内部 ID → 選んだステータスの内部 ID
  busy: boolean;
}

export const keyOf = (t: TransferTarget) => `${t.storeId}/${t.workspaceId}`;
export const isEpic = (it: Item) => it.kind === "Epic";
const epicsOf = (c: Current) => c.target.projects.find((p) => p.id === c.form.project)?.epics ?? [];

// formFor は、ワークスペース t の projectId(省略すると先頭)と、その先頭の Epic を選んだ値。Epic を移すなら親は ""。
export function formFor(it: Item, t: TransferTarget, projectId = t.projects[0]?.id ?? "") {
  const epics = t.projects.find((p) => p.id === projectId)?.epics ?? [];
  return {
    workspace: keyOf(t),
    project: projectId,
    parent: isEpic(it) ? "" : (epics[0]?.id ?? ""),
  };
}

export function view(c: Current): TransferDialogState {
  const p = c.preview;
  const epics = epicsOf(c);
  const epic = isEpic(c.item);
  return {
    item: c.item,
    displayId: c.displayId,
    workspaces: c.targets.map((t) => ({
      value: keyOf(t),
      label: `${t.storeName} / ${t.workspaceName}`,
    })),
    projects: c.target.projects.map((x) => ({ value: x.id, label: x.title })),
    parents: epic ? null : epics.map((e) => ({ value: e.id, label: `${e.display} ${e.title}` })),
    form: { ...c.form },
    loading: p === null,
    intro: p ? transferIntro(c.displayId, c.item.title, p.below, p.deleted) : "",
    noEpic: !epic && epics.length === 0 ? noEpicMessage : "",
    changes: p ? changeRows(p) : [],
    replacements: (p?.replacements ?? []).map((r) => ({
      status: r.status,
      name: r.name,
      label: `${r.name}(${r.count} 件)`,
      options: r.choices.map((x) => ({ value: x.id, label: x.name })),
      value: c.choices[r.status] ?? r.default,
    })),
    canSubmit: p !== null && !c.busy && (epic || c.form.parent !== ""),
    busy: c.busy,
  };
}
