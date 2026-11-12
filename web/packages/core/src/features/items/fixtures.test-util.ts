import { NotFound, Refused } from "../../model/errors";
import type { Item, Kind } from "../../model/item";
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
  };
}

export interface FakeItems extends ItemsPort {
  all: Item[];
  calls: string[];
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
  const checkTitle = (title: string) => {
    if (title.trim() === "") throw new Refused("タイトルを入力してください");
  };
  return {
    all,
    calls,
    async list(_s, projectId) {
      return all.filter((x) => x.projectId === projectId).sort((a, b) => a.number - b.number);
    },
    async get(_s, id) {
      return find(id);
    },
    async create(storeId, input) {
      calls.push(`create ${input.kind} ${input.title}`);
      checkTitle(input.title);
      return add(storeId, {
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
