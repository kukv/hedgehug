import { NotFound, Refused } from "../../model/errors";
import type { Item, Kind, PropertyInput } from "../../model/item";
import type { ItemsPort } from "../../port/items";

// flush は、待っている Promise の続きを先に進める(core は setTimeout の型を持たないので、マイクロタスクを回す)。
export async function flush(): Promise<void> {
  for (let i = 0; i < 50; i++) await Promise.resolve();
}

// gate は、release を呼ぶまで待たせる Promise(待っているあいだの操作を試すために使う)。
export function gate(): { wait: Promise<void>; release: () => void } {
  let release = () => {};
  const wait = new Promise<void>((resolve) => {
    release = resolve;
  });
  return { wait, release };
}

// テスト用のアイテムの口。メモリ上にアイテムを持ち、作成・変更をそのまま反映する。
// 番号はワークスペースごとの連番。断りは、タイトルが空のときだけ(サーバーの検査の代わり)。

const initialValues: Pick<
  Item,
  | "status"
  | "priority"
  | "assignee"
  | "start"
  | "end"
  | "points"
  | "estimate"
  | "systems"
  | "choices"
  | "fields"
> = {
  status: "st1",
  priority: "pr2",
  assignee: "",
  start: "",
  end: "",
  points: null,
  estimate: null,
  systems: [],
  choices: {},
  fields: {},
};

export function item(
  storeId: string,
  projectId: string,
  n: number,
  kind: Kind,
  parentId: string | null,
  title = `アイテム ${n}`,
): Item {
  return {
    id: `${storeId}-I${n}`,
    storeId,
    workspaceId: `${storeId}-W1`,
    projectId,
    parentId,
    number: n,
    kind,
    rank: "V",
    title,
    watchers: ["taro"],
    createdAt: `2026-11-13T09:00:${String(n).padStart(2, "0")}.000Z`,
    createdBy: "taro",
    ...initialValues,
  };
}

export interface FakeItems extends ItemsPort {
  all: Item[];
  calls: string[];
  // refuseNext は、次の setProperty か create を、この文言の断りにする(1 回だけ)。
  refuseNext(message: string): void;
  // holdCreate は、release を呼ぶまで create を待たせる。
  holdCreate(): { release: () => void };
  // createdValues は、create に渡された values の記録。
  createdValues: Record<string, PropertyInput>[];
}

export function fakeItemsPort(initial: Item[] = []): FakeItems {
  const all = [...initial];
  const calls: string[] = [];
  const find = (id: string) => {
    const it = all.find((x) => x.id === id);
    if (!it) throw new NotFound("not found");
    return it;
  };
  const next = () => Math.max(0, ...all.map((x) => x.number)) + 1;
  const add = (storeId: string, base: Omit<Item, "id" | "number" | "storeId">): Item => {
    const n = next();
    const it = { ...base, id: `${storeId}-I${n}`, storeId, number: n };
    all.push(it);
    return it;
  };
  let refusal = "";
  let hold: Promise<void> | null = null;
  const createdValues: Record<string, PropertyInput>[] = [];
  // takeRefusal は、refuseNext で決めた断りを 1 回だけ投げる。
  const takeRefusal = () => {
    if (refusal === "") return;
    const m = refusal;
    refusal = "";
    throw new Refused(m);
  };
  // set は、プロパティの値をアイテムに入れる(テスト用の簡単な形。組み込みはキー、追加は fields)。
  const set = (it: Item, propertyId: string, value: PropertyInput) => {
    const builtin = ["status", "priority", "assignee", "start", "end"];
    if (builtin.includes(propertyId))
      (it as unknown as Record<string, PropertyInput>)[propertyId] = value ?? "";
    else if (propertyId === "points" || propertyId === "estimate")
      it[propertyId] = value === "" || value === null ? null : Number(value);
    else it.fields = { ...it.fields, [propertyId]: value as string | number | boolean | null };
  };
  const checkTitle = (title: string) => {
    if (title.trim() === "") throw new Refused("タイトルを入力してください");
  };
  return {
    all,
    calls,
    refuseNext(message) {
      refusal = message;
    },
    holdCreate() {
      const g = gate();
      hold = g.wait;
      return { release: g.release };
    },
    createdValues,
    async setProperty(_s, id, propertyId, value) {
      calls.push(`setProperty ${id} ${propertyId} ${JSON.stringify(value)}`);
      takeRefusal();
      const it = find(id);
      set(it, propertyId, value);
      return it;
    },
    async addValue(_s, id, propertyId, valueId) {
      calls.push(`addValue ${id} ${propertyId} ${valueId}`);
      const it = find(id);
      if (propertyId === "systems") it.systems = [...it.systems, valueId];
      return it;
    },
    async removeValue(_s, id, propertyId, valueId) {
      calls.push(`removeValue ${id} ${propertyId} ${valueId}`);
      const it = find(id);
      if (propertyId === "systems") it.systems = it.systems.filter((x) => x !== valueId);
      return it;
    },
    async checkStep(_s, id, done) {
      calls.push(`checkStep ${id} ${done}`);
      const it = find(id);
      it.status = done ? "st3" : "st1";
      return it;
    },
    async list(_s, projectId) {
      return all.filter((x) => x.projectId === projectId).sort((a, b) => a.number - b.number);
    },
    async get(_s, id) {
      return find(id);
    },
    async create(storeId, input) {
      if (hold) {
        const h = hold;
        hold = null;
        await h;
      }
      takeRefusal();
      calls.push(`create ${input.kind} ${input.title}`);
      checkTitle(input.title);
      createdValues.push(input.values);
      const given = input.values;
      return add(storeId, {
        ...initialValues,
        ...(typeof given.status === "string" ? { status: given.status } : {}),
        ...(typeof given.priority === "string" ? { priority: given.priority } : {}),
        workspaceId: `${storeId}-W1`,
        projectId: input.projectId,
        parentId: input.parentId,
        kind: input.kind,
        rank: "V",
        title: input.title,
        watchers: ["taro"],
        createdAt: new Date(Date.UTC(2026, 10, 13, 10, 0, all.length)).toISOString(),
        createdBy: "taro",
      });
    },
    async rename(_s, id, title) {
      calls.push(`rename ${id} ${title}`);
      const it = find(id);
      it.title = title;
      return it;
    },
    async addStep(storeId, parentId, title) {
      calls.push(`addStep ${parentId} ${title}`);
      checkTitle(title);
      const parent = find(parentId);
      return add(storeId, {
        workspaceId: parent.workspaceId,
        projectId: parent.projectId,
        parentId,
        kind: "Step",
        rank: "W",
        title,
        ...initialValues,
        watchers: ["taro"],
        createdAt: new Date(Date.UTC(2026, 10, 13, 10, 0, all.length)).toISOString(),
        createdBy: "taro",
      });
    },
    async moveUp(_s, id) {
      calls.push(`moveUp ${id}`);
    },
    async nextNumber() {
      return next();
    },
    async findByNumber(_s, _w, n) {
      const it = all.find((x) => x.number === n);
      if (!it) throw new NotFound("not found");
      return it;
    },
  };
}
