import { Refused } from "../../model/errors";
import type { SaveAsTemplateInput } from "../../model/workspace";
import type { WorkspacesPort } from "../../port/workspaces";
import type { DetailView } from "../items/detail";
import type { Toasts } from "../toasts/toasts";
import { templateCreated } from "./templates";

// アイテムのメニューの「テンプレートとして保存…」(03「アイテムからテンプレートを作る」、B3-17)。

export const saveAsTemplateLabel = "テンプレートとして保存…";

// TemplateSubject は、テンプレートにするアイテムと、その Store・子。詳細パネルの DetailView も、ツリーの行も、これを渡せる。
export type TemplateSubject = Pick<DetailView, "store" | "item" | "children">;

export interface SaveAsTemplateModel {
  // name・title は、ダイアログの初期値(アイテムのタイトル)。
  name: string;
  title: string;
  // showStructure は、子も Step もないアイテムでは false(「子と Step を含める」を出さない)。
  showStructure: boolean;
}

export function saveAsTemplateDialog(view: TemplateSubject): SaveAsTemplateModel {
  return {
    name: view.item.title,
    title: view.item.title,
    showStructure: view.children.length > 0,
  };
}

export interface SaveAsTemplate {
  // save は、保存してトースト「テンプレートを作成しました」を出す。断りは onRefused に渡し、false を返す。
  save(
    view: TemplateSubject,
    input: SaveAsTemplateInput,
    onRefused: (message: string) => void,
  ): Promise<boolean>;
}

export function createSaveAsTemplate(deps: {
  port: WorkspacesPort;
  toasts: Toasts;
}): SaveAsTemplate {
  return {
    async save(view, input, onRefused) {
      try {
        await deps.port.saveItemAsTemplate(view.store.id, view.item.id, input);
      } catch (e) {
        if (e instanceof Refused) {
          onRefused(e.message);
          return false;
        }
        throw e;
      }
      deps.toasts.show(templateCreated);
      return true;
    },
  };
}
