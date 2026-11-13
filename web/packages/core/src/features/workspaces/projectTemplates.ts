import type { Kind } from "../../model/item";
import type { WorkspaceSettings } from "../../model/workspace";
import { defaultIn, noTemplate, rulesOf, usableIn } from "./projectRules";
import { propertyKinds } from "./properties";

// プロジェクトの「テンプレート」の区画(03「プロジェクトのテンプレート」)の材料。

export interface TemplateUse {
  id: string;
  name: string;
  kind: string;
  checked: boolean;
}

export interface ProjectKindDefault {
  kind: Kind;
  // ""はワークスペースと同じ、noTemplate は「なし」、それ以外はテンプレート ID
  value: string;
  options: { value: string; label: string }[];
}

export interface ProjectTemplateView {
  uses: TemplateUse[];
  // 使うテンプレートが 1 件も選ばれていない(すべて使える)
  allUsable: boolean;
  defaults: ProjectKindDefault[];
}

export function projectTemplateView(
  settings: WorkspaceSettings,
  projectId: string,
): ProjectTemplateView {
  const r = rulesOf(settings, projectId);
  const uses = settings.templates.map((t) => ({
    id: t.id,
    name: t.name,
    kind: t.kind,
    checked: r.templates.includes(t.id),
  }));
  const defaults = propertyKinds.map((kind): ProjectKindDefault => {
    const usable = usableIn(settings, r, kind);
    const inherited = defaultIn(settings, { ...r, templateDefaults: {} }, kind);
    const own = (r.templateDefaults as Partial<Record<string, string>>)[kind] ?? "";
    // 使えないテンプレートの既定は「なし」と同じ扱い(B5a-9)
    const value = own === "" || usable.some((t) => t.id === own) ? own : noTemplate;
    return {
      kind,
      value,
      options: [
        { value: "", label: `ワークスペースと同じ(${inherited?.name ?? "なし"})` },
        { value: noTemplate, label: "なし" },
        ...usable.map((t) => ({ value: t.id, label: t.name })),
      ],
    };
  });
  return { uses, allUsable: !uses.some((u) => u.checked), defaults };
}
