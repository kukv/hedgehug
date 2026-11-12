import type { NewProperty, PropertyType, WorkspaceSettings } from "../../model/workspace";

// プロパティの表(03「プロパティ」)。行の作りと文言をここに置く。

export const typeLabels: Record<PropertyType, string> = {
  status: "ステータス",
  user: "ユーザー",
  priority: "優先度",
  date: "日付",
  number: "数値",
  duration: "時間",
  systems: "関連システム",
  text: "テキスト",
  single: "単一選択",
  multi: "複数選択",
  check: "チェック",
  url: "URL",
};

// 追加できる 8 型。03 の表の順。時間は組み込みの見積もり時間だけが持つ。
export const addableTypes: PropertyType[] = [
  "text",
  "number",
  "date",
  "single",
  "multi",
  "user",
  "check",
  "url",
];

// 使う種類の 5 つ。
export const propertyKinds = ["Epic", "Story", "Task", "Bug", "Step"] as const;

export const propertiesHelp =
  "組み込みのプロパティは名前の変更や、種類ごとの表示・非表示を設定できます。追加したプロパティは、型・選択肢・キーを決められます。キーは絞り込みに使います(例: customer:サンプル)。";
export const keyHint = "英小文字・数字・_。絞り込みで使います";

export interface PropertyRow {
  id: string;
  name: string;
  key: string;
  builtin: boolean;
  // typeLabels の名前
  type: string;
  // 単一・複数選択だけ。「選択肢 N」
  choices?: number;
  // すべての種類なら "all"
  kinds: string[] | "all";
  // ステータス(使う種類を変えられない)
  kindsFixed: boolean;
  // 組み込み(ステータス以外)だけ
  hide?: "非表示" | "表示";
  // 追加プロパティだけ
  deletable: boolean;
  // 行を薄く出す
  hidden: boolean;
  first: boolean;
  last: boolean;
}

export function propertyRows(settings: WorkspaceSettings): PropertyRow[] {
  const all = settings.properties;
  return all.map((p, i) => {
    const row: PropertyRow = {
      id: p.id,
      name: p.name,
      key: p.key,
      builtin: p.builtin,
      type: typeLabels[p.type],
      kinds:
        p.type === "status" ||
        p.kinds.length === 0 ||
        propertyKinds.every((k) => p.kinds.includes(k))
          ? "all"
          : p.kinds,
      kindsFixed: p.type === "status",
      deletable: !p.builtin,
      hidden: p.hidden,
      first: i === 0,
      last: i === all.length - 1,
    };
    if (p.type === "single" || p.type === "multi") row.choices = p.choices.length;
    if (p.builtin && p.type !== "status") row.hide = p.hidden ? "表示" : "非表示";
    return row;
  });
}

// 確認ダイアログ「プロパティを削除」の本文(03「削除」)。
export function deletePropertyBody(name: string, count: number): string {
  const head = `「${name}」を削除します。`;
  return count > 0 ? `${head}値の入っている ${count} 件から値が消えます。` : head;
}

// 使う種類のチェックを 1 つ入れ替えた、保存する種類の一覧(03「使う種類」)。
// 1 つも残らないときは空にして、サーバーの断りに任せる。
export function toggleKind(kinds: string[] | "all", kind: string): string[] {
  const current: string[] = kinds === "all" ? [...propertyKinds] : kinds;
  const next = current.includes(kind) ? current.filter((k) => k !== kind) : [...current, kind];
  return propertyKinds.filter((k) => next.includes(k));
}

// 追加ダイアログの入力から送る値。種類を 1 つも選ばないとすべて(03「追加プロパティ」)。
export function newProperty(
  name: string,
  key: string,
  type: PropertyType,
  kinds: string[],
): NewProperty {
  return { name, key, type, kinds: kinds.length === 0 ? [...propertyKinds] : kinds };
}
