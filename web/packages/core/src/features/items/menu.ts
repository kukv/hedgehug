import type { Item, Kind } from "../../model/item";
import { readOnlyNote } from "../catalog/readonly";
import { saveAsTemplateLabel } from "../workspaces/saveAsTemplate";
import { childKindOf } from "./createDefaults";
import { visibleNeighbors } from "./siblings";

// アイテムのメニュー(02「上から順の構成」)の項目と、無効にするときの理由(A4b-16)。詳細パネルのメニューと
// ツリーの行のメニューが同じ項目を使う。ツリーの行のメニューだけ、先頭に「詳細を開く」「{種類} を追加」が付く。
// 「別のワークスペースへ移動…」は、その操作がないので出さない。

export type ItemMenuId =
  | "open"
  | "add-child"
  | "move-up"
  | "move-down"
  | "move"
  | "change-kind"
  | "duplicate"
  | "save-as-template"
  | "copy-id"
  | "delete";

// MenuEntry は、メニューの 1 項目。disabled なら hint が無効の理由(押せない)。
export interface MenuEntry {
  id: ItemMenuId;
  label: string;
  disabled: boolean;
  hint?: string;
}

export interface ItemMenuContext {
  kind: Kind;
  readOnly: boolean;
  // row は、ツリーの行のメニューか(先頭に「詳細を開く」「{種類} を追加」を足す)。詳細パネルは false。
  row: boolean;
  // hasPrev・hasNext は、見えている兄弟の中に、直前・直後があるか(visibleNeighbors)。
  hasPrev: boolean;
  hasNext: boolean;
  // hasOtherProjects は、同じワークスペースにほかのプロジェクトがあるか。Epic の「移動…」だけが見る。
  hasOtherProjects: boolean;
}

// 無効の理由の文言(02「詳細パネル」のメニューの項目。A4b-16)。
export const menuReasons = {
  atTop: "先頭です",
  atBottom: "末尾です",
  epicKind: "Epic の種類は変更できません",
  noProject: "移せるプロジェクトがありません",
};

// 閲覧のみの Store で無効にする項目(「詳細を開く」「ID をコピー」は使える)。
const writes: ReadonlySet<ItemMenuId> = new Set([
  "add-child",
  "move-up",
  "move-down",
  "move",
  "change-kind",
  "duplicate",
  "save-as-template",
  "delete",
]);

// itemMenu は、項目を決まった順で返す。無効の理由が重なるときは、閲覧のみを先にする。
export function itemMenu(ctx: ItemMenuContext): MenuEntry[] {
  const entry = (id: ItemMenuId, label: string, reason = ""): MenuEntry => {
    const hint = ctx.readOnly && writes.has(id) ? readOnlyNote : reason;
    return hint ? { id, label, disabled: true, hint } : { id, label, disabled: false };
  };
  const epic = ctx.kind === "Epic";
  const childKind = childKindOf(ctx.kind);
  const head: MenuEntry[] = [];
  if (ctx.row) {
    head.push(entry("open", "詳細を開く"));
    if (childKind) head.push(entry("add-child", `${childKind} を追加`));
  }
  return [
    ...head,
    entry("move-up", "上へ移動", ctx.hasPrev ? "" : menuReasons.atTop),
    entry("move-down", "下へ移動", ctx.hasNext ? "" : menuReasons.atBottom),
    entry("move", "移動…", epic && !ctx.hasOtherProjects ? menuReasons.noProject : ""),
    entry("change-kind", "種類を変更…", epic ? menuReasons.epicKind : ""),
    entry("duplicate", "複製"),
    entry("save-as-template", saveAsTemplateLabel),
    entry("copy-id", "ID をコピー"),
    entry("delete", "削除…"),
  ];
}

// itemMenuContext は、アイテムと、見えている行の並び(詳細パネルは削除されていない全兄弟)から、メニューの文脈を作る。
// projectCount は、そのワークスペースのプロジェクトの数(Epic の「移動…」の可否)。row は、ツリーの行のメニューなら true。
export function itemMenuContext(
  it: Item,
  rows: readonly Item[],
  opts: { readOnly: boolean; projectCount: number; row?: boolean },
): ItemMenuContext {
  const { prev, next } = visibleNeighbors(rows, it.id);
  return {
    kind: it.kind,
    readOnly: opts.readOnly,
    row: opts.row ?? false,
    hasPrev: prev !== undefined,
    hasNext: next !== undefined,
    hasOtherProjects: opts.projectCount > 1,
  };
}
