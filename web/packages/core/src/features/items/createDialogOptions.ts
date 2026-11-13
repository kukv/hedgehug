import type { Item, Kind } from "../../model/item";
import type { StoreNode } from "../catalog/catalog";
import { chosen, sprintChoices } from "../planning/choices";
import type { Option } from "../projects/createDialog";
import { itemDisplayId } from "../router/route";
import { parentMissing } from "./createDefaults";
import { rowsFor, type WorkspaceContext } from "./createDialogLoad";
import { structureOf, type TemplateForm, templateOptions } from "./createTemplate";
import type { Values } from "./createValues";

// 作成ダイアログの選択肢と、入力から作り直す欄(02「アイテムの作成」)。

// wsOptions は、書ける Store のワークスペース。
export const wsOptions = (stores: StoreNode[]): Option[] =>
  stores
    .filter((s) => !s.readOnly)
    .flatMap((s) =>
      s.workspaces.map((w) => ({ value: w.id, label: `${s.name} / ${w.name} (${w.prefix})` })),
    );

export const parentOptions = (prefix: string, candidates: Item[]): Option[] =>
  candidates.map((it) => ({
    value: it.id,
    label: `${itemDisplayId(prefix, it.number)} ${it.title}`,
  }));

// missing は、親の候補がないときの赤字。Epic と、候補があるときは空。
export const missing = (kind: Kind, candidates: Item[]): string =>
  kind !== "Epic" && candidates.length === 0 ? parentMissing(kind) : "";

// viewOf は、入力から作り直す欄(プロパティの行・テンプレートの欄・一緒に作るもの・スプリントの選択肢)。
export const viewOf = (ctx: WorkspaceContext, form: TemplateForm) => ({
  rows: rowsFor(ctx, form),
  templates: templateOptions(ctx.settings, form.kind),
  structure: structureOf(ctx.settings, form, ctx.people.members),
  sprints: sprintChoices(ctx.plan, "").map((c) => ({ value: c.id, label: c.label })),
});

// keptSprint は、読み直したあとのスプリント。選んでいたものが選択肢になければ「バックログ」(完了・削除されたもの)。
export const keptSprint = (ctx: WorkspaceContext, sprint: string): string =>
  chosen(sprintChoices(ctx.plan, ""), sprint);

// carried は、読み直しを待つあいだに入力された分(タイトル・説明・種類・テンプレート・「続けて作成」)を、読み直した
// 入力へ引き継ぐ(決定 27)。値は、呼び出し側が決めたもの。
export const carried = <F extends TemplateForm & { keepOpen: boolean }>(
  loaded: F,
  now: F,
  values: Values,
): F => ({
  ...loaded,
  title: now.title,
  kind: now.kind,
  description: now.description,
  templateId: now.templateId,
  templateDescription: now.templateDescription,
  keepOpen: now.keepOpen,
  values,
});
