import { NotFound, Refused } from "../../model/errors";
import type {
  PropertyType,
  Workspace,
  WorkspaceDetail,
  WorkspaceProperty,
  WorkspaceTemplate,
  WorkspaceValue,
} from "../../model/workspace";
import type { Direction, WorkspacesPort } from "../../port/workspaces";

export interface FakeWorkspacesPort extends WorkspacesPort {
  // 呼ばれた操作の名前(読む操作も含む)
  calls: string[];
  // 次の書き込みを、この文言で断らせる
  refuseNext(message: string): void;
}

const v = (id: string, name: string, color: string, category?: WorkspaceValue["category"]) =>
  category ? { id, name, color, category } : { id, name, color };

function builtin(id: string, name: string, type: PropertyType): WorkspaceProperty {
  return { id, key: id, name, type, builtin: true, kinds: [], hidden: false, choices: [] };
}

// 既定: 8 プロパティ・5 ステータス・5 優先度・6 関連システム
export function defaultDetail(storeId: string, id: string): WorkspaceDetail {
  return {
    id,
    storeId,
    name: "Web",
    prefix: "HH",
    oldPrefixes: [],
    settings: {
      properties: [
        builtin("status", "ステータス", "status"),
        builtin("priority", "優先度", "priority"),
        builtin("owner", "担当者", "user"),
        builtin("start", "開始日", "date"),
        builtin("end", "終了日", "date"),
        builtin("estimate", "見積もり時間", "duration"),
        builtin("systems", "関連システム", "systems"),
        builtin("memo", "メモ", "text"),
      ],
      statuses: [
        v("st1", "未着手", "gray", "todo"),
        v("st2", "進行中", "blue", "doing"),
        v("st3", "レビュー中", "purple", "doing"),
        v("st4", "完了", "green", "done"),
        v("st5", "中止", "red", "canceled"),
      ],
      priorities: [
        v("pr1", "緊急", "red"),
        v("pr2", "高", "orange"),
        v("pr3", "中", "yellow"),
        v("pr4", "低", "sky"),
        v("pr5", "なし", "gray"),
      ],
      systems: [
        v("sy1", "認証基盤", "teal"),
        v("sy2", "決済", "indigo"),
        v("sy3", "通知", "pink"),
        v("sy4", "検索", "amber"),
        v("sy5", "管理画面", "slate"),
        v("sy6", "バッチ", "brown"),
      ],
      initialStatus: "st1",
      initialPriority: "pr3",
      categoryDefaults: { todo: "st1", doing: "st2", done: "st4", canceled: "st5" },
      workflow: {},
      templates: [],
      templateDefaults: {},
    },
  };
}

const clone = <T>(x: T): T => JSON.parse(JSON.stringify(x)) as T;

function swap<T>(list: T[], index: number, direction: Direction) {
  const to = direction === "up" ? index - 1 : index + 1;
  const a = list[index];
  const b = list[to];
  if (a === undefined || b === undefined) return;
  list[index] = b;
  list[to] = a;
}

// fakeWorkspacesPort は、ワークスペースの設定をメモリに持つ口。ID は最初に開いたときに既定の設定で作る。
export function fakeWorkspacesPort(): FakeWorkspacesPort {
  const details = new Map<string, WorkspaceDetail>();
  const calls: string[] = [];
  let refusal: string | null = null;

  const detail = (s: string, w: string) => {
    const key = `${s}/${w}`;
    let d = details.get(key);
    if (!d) {
      d = defaultDetail(s, w);
      details.set(key, d);
    }
    return d;
  };
  const list = (d: WorkspaceDetail, name: string): WorkspaceValue[] => {
    const l =
      name === "statuses"
        ? d.settings.statuses
        : name === "priorities"
          ? d.settings.priorities
          : name === "systems"
            ? d.settings.systems
            : d.settings.properties.find((p) => p.id === name)?.choices;
    if (!l) throw new NotFound(name);
    return l;
  };
  const prop = (d: WorkspaceDetail, id: string) => {
    const p = d.settings.properties.find((x) => x.id === id);
    if (!p) throw new NotFound(id);
    return p;
  };
  const value = (l: WorkspaceValue[], id: string) => {
    const x = l.find((y) => y.id === id);
    if (!x) throw new NotFound(id);
    return x;
  };
  // write は、書き込みを 1 つ行う。refuseNext があれば、何も変えずに断る。
  async function write<T>(name: string, fn: () => T): Promise<T> {
    calls.push(name);
    if (refusal !== null) {
      const message = refusal;
      refusal = null;
      throw new Refused(message);
    }
    return fn();
  }
  let seq = 0;
  const newId = (prefix: string) => {
    seq += 1;
    return `${prefix}${seq}`;
  };

  return {
    calls,
    refuseNext: (message) => {
      refusal = message;
    },
    async get(s, w) {
      calls.push("get");
      return clone(detail(s, w));
    },
    async usage() {
      calls.push("usage");
      return { items: 0, firstNumbers: [], values: {}, properties: {} };
    },
    create: (s, name, prefix) =>
      write("create", (): Workspace => ({ id: newId("W"), storeId: s, name, prefix })),
    rename: (s, w, name) =>
      write("rename", () => {
        if (name.trim() === "") throw new Refused("名前は空にできません");
        detail(s, w).name = name;
      }),
    changePrefix: (s, w, prefix) =>
      write("changePrefix", () => {
        const d = detail(s, w);
        d.oldPrefixes = [...d.oldPrefixes, d.prefix];
        d.prefix = prefix;
      }),
    move: (_s, _w, _direction) => write("move", () => undefined),
    addProperty: (s, w, input) =>
      write("addProperty", () => {
        const p: WorkspaceProperty = {
          ...input,
          id: newId("pp"),
          builtin: false,
          hidden: false,
          choices: [],
        };
        detail(s, w).settings.properties.push(p);
        return clone(p);
      }),
    renameProperty: (s, w, id, name) =>
      write("renameProperty", () => {
        prop(detail(s, w), id).name = name;
      }),
    setPropertyKinds: (s, w, id, kinds) =>
      write("setPropertyKinds", () => {
        prop(detail(s, w), id).kinds = kinds;
      }),
    setPropertyHidden: (s, w, id, hidden) =>
      write("setPropertyHidden", () => {
        prop(detail(s, w), id).hidden = hidden;
      }),
    moveProperty: (s, w, id, direction) =>
      write("moveProperty", () => {
        const ps = detail(s, w).settings.properties;
        swap(
          ps,
          ps.findIndex((p) => p.id === id),
          direction,
        );
      }),
    deleteProperty: (s, w, id) =>
      write("deleteProperty", () => {
        const d = detail(s, w);
        prop(d, id);
        d.settings.properties = d.settings.properties.filter((p) => p.id !== id);
      }),
    addValue: (s, w, name, valueName) =>
      write("addValue", () => {
        const x: WorkspaceValue = { id: newId("va"), name: valueName, color: "gray" };
        list(detail(s, w), name).push(x);
        return clone(x);
      }),
    renameValue: (s, w, name, id, to) =>
      write("renameValue", () => {
        value(list(detail(s, w), name), id).name = to;
      }),
    setValueColor: (s, w, name, id, color) =>
      write("setValueColor", () => {
        value(list(detail(s, w), name), id).color = color;
      }),
    setStatusCategory: (s, w, id, category) =>
      write("setStatusCategory", () => {
        value(detail(s, w).settings.statuses, id).category = category;
      }),
    moveValue: (s, w, name, id, direction) =>
      write("moveValue", () => {
        const l = list(detail(s, w), name);
        swap(l, l.indexOf(value(l, id)), direction);
      }),
    deleteValue: (s, w, name, id) =>
      write("deleteValue", () => {
        const d = detail(s, w);
        const l = list(d, name);
        l.splice(l.indexOf(value(l, id)), 1);
      }),
    setInitialStatus: (s, w, id) =>
      write("setInitialStatus", () => {
        detail(s, w).settings.initialStatus = id;
      }),
    setInitialPriority: (s, w, id) =>
      write("setInitialPriority", () => {
        detail(s, w).settings.initialPriority = id;
      }),
    setCategoryDefault: (s, w, category, id) =>
      write("setCategoryDefault", () => {
        detail(s, w).settings.categoryDefaults[category] = id;
      }),
    setTransition: (s, w, from, to, allowed) =>
      write("setTransition", () => {
        const wf = detail(s, w).settings.workflow;
        const tos = wf[from] ?? [];
        const without = tos.filter((t) => t !== to);
        wf[from] = allowed ? [...without, to] : without;
      }),
    createTemplate: (s, w, input) =>
      write("createTemplate", (): WorkspaceTemplate => ({ id: "t-new", ...clone(input) })),
    updateTemplate: (_s, _w, _id, _input) => write("updateTemplate", () => undefined),
    deleteTemplate: (_s, _w, _id) => write("deleteTemplate", () => undefined),
    setTemplateDefault: (_s, _w, _kind, _id) => write("setTemplateDefault", () => undefined),
    saveItemAsTemplate: (_s, _item, input) =>
      write(
        "saveItemAsTemplate",
        (): WorkspaceTemplate => ({
          id: "t-new",
          name: input.name,
          kind: "Task",
          title: input.title,
          description: "",
          values: {},
          children: [],
          steps: [],
        }),
      ),
  };
}
