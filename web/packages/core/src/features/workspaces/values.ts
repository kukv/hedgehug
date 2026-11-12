import type {
  Category,
  ValueList,
  WorkspaceSettings,
  WorkspaceUsage,
  WorkspaceValue,
} from "../../model/workspace";

// 値の編集(03「値の編集」)。ステータス・優先度・関連システム・選択肢で同じ形を使う。

// 03「色」の 18 色。id は `--c-<id>-*` トークンの名前。
export const colors: { id: string; label: string }[] = [
  { id: "gray", label: "グレー" },
  { id: "slate", label: "青灰" },
  { id: "brown", label: "茶" },
  { id: "red", label: "赤" },
  { id: "rose", label: "ローズ" },
  { id: "pink", label: "桃" },
  { id: "magenta", label: "赤紫" },
  { id: "purple", label: "紫" },
  { id: "indigo", label: "藍" },
  { id: "blue", label: "青" },
  { id: "sky", label: "空色" },
  { id: "cyan", label: "水色" },
  { id: "teal", label: "青緑" },
  { id: "green", label: "緑" },
  { id: "olive", label: "オリーブ" },
  { id: "amber", label: "山吹" },
  { id: "yellow", label: "黄" },
  { id: "orange", label: "橙" },
];

export const categoryLabels: Record<Category, string> = {
  todo: "未着手",
  doing: "進行中",
  done: "完了",
  canceled: "中止",
};

export const valuesHelp = "名前・色・順番を変更できます。このワークスペースだけの値です。";
export const statusHelp =
  "分類は、ボードの列・バックログ・バーンダウン・マイタスクで「未着手・進行中・完了・中止」を見分けるのに使います。このワークスペースだけの値です。";

const initialDisabledText = "完了・中止のステータスは初期ステータスにできません";
const statusNoteText =
  "ワークフローの制約と自動化(プロジェクト別のルールを含む)からも削除されます。";

export interface ValueRow {
  id: string;
  name: string;
  color: string;
  category?: Category;
  // 使っているアイテムの件数
  count: number;
  // 「初期」の印(ステータス・優先度)
  initial: boolean;
  // 「初期ステータスにする」を無効にする理由
  initialDisabled?: string;
  // 上へ・下への無効
  first: boolean;
  last: boolean;
}

function valuesOf(settings: WorkspaceSettings, list: ValueList): WorkspaceValue[] {
  if (list === "statuses") return settings.statuses;
  if (list === "priorities") return settings.priorities;
  if (list === "systems") return settings.systems;
  return settings.properties.find((p) => p.id === list)?.choices ?? [];
}

export function valueRows(
  settings: WorkspaceSettings,
  list: ValueList,
  usage: WorkspaceUsage,
): ValueRow[] {
  const values = valuesOf(settings, list);
  return values.map((x, i) => {
    const row: ValueRow = {
      id: x.id,
      name: x.name,
      color: x.color,
      count: usage.values[x.id] ?? 0,
      initial:
        (list === "statuses" && x.id === settings.initialStatus) ||
        (list === "priorities" && x.id === settings.initialPriority),
      first: i === 0,
      last: i === values.length - 1,
    };
    if (x.category) row.category = x.category;
    if (list === "statuses" && (x.category === "done" || x.category === "canceled")) {
      row.initialDisabled = initialDisabledText;
    }
    return row;
  });
}

export function valueListLabel(settings: WorkspaceSettings, list: ValueList): string {
  if (list === "statuses") return "ステータス";
  if (list === "priorities") return "優先度";
  if (list === "systems") return "関連システム";
  const name = settings.properties.find((p) => p.id === list)?.name ?? "";
  return `「${name}」の選択肢`;
}

// 確認ダイアログ「「X」を削除」の中身(03「削除」の表)。
export interface DeleteDialogModel {
  title: string;
  body: string;
  // 単一の値を使っているときだけ
  replacements?: { value: string; label: string }[];
  initialReplacement?: string;
  // ステータスで使っているとき「N 件を「{代わり}」に置き換えます。」
  replaceNote?: (replacementName: string) => string;
  statusNote?: string;
}

export function deleteDialog(
  settings: WorkspaceSettings,
  list: ValueList,
  valueId: string,
  usage: WorkspaceUsage,
): DeleteDialogModel {
  const target = valuesOf(settings, list).find((x) => x.id === valueId);
  const count = usage.values[valueId] ?? 0;
  const model: DeleteDialogModel = { title: `「${target?.name ?? ""}」を削除`, body: "" };
  if (list === "statuses") model.statusNote = statusNoteText;
  if (count === 0) {
    model.body = "使っているアイテムはありません。";
    return model;
  }
  const property = settings.properties.find((p) => p.id === list);
  const single = list === "statuses" || list === "priorities" || property?.type === "single";
  if (!single) {
    model.body = `${count} 件がこの値を使っています。それらからこの値が削除されます。`;
    return model;
  }
  model.body = `${count} 件がこの値を使っています。代わりの値を選んでください。`;
  const others = valuesOf(settings, list).filter((x) => x.id !== valueId);
  const replacements = others.map((x) => ({ value: x.id, label: x.name }));
  if (list === "statuses" || list === "priorities") {
    const initial = list === "statuses" ? settings.initialStatus : settings.initialPriority;
    model.replacements = replacements;
    model.initialReplacement = others.some((x) => x.id === initial)
      ? initial
      : (others[0]?.id ?? "");
  } else {
    model.replacements = [{ value: "", label: "なし" }, ...replacements];
    model.initialReplacement = "";
  }
  if (list === "statuses") {
    model.replaceNote = (name) => `${count} 件を「${name}」に置き換えます。`;
  }
  return model;
}
