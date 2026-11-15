import { describe, expect, it } from "vitest";
import { Refused } from "../../model/errors";
import type { ViewDto, ViewsPort } from "../../port/views";
import { get } from "../../reactive/store";
import { createToasts } from "../toasts/toasts";
import type { ViewContent } from "./content";
import { createViews } from "./views";

const content: ViewContent = {
  query: "kind:bug",
  layout: "table",
  table: { columns: ["id", "title"], group: "none", sort: null },
};

const idOf = (n: number) => `01jabcdefghjkmnpqrstvwx${String(n).padStart(3, "0")}`;

// fakePort は、メモリ上のビューの口。fail に名前を入れたメソッドは、断り以外のエラーで落ちる。
function fakePort(start: ViewDto[] = []) {
  let rows = [...start];
  let seq = start.length;
  const fail = new Set<keyof ViewsPort>();
  const check = (m: keyof ViewsPort) => {
    if (fail.has(m)) throw new Error("通信できませんでした");
  };
  const find = (id: string) => {
    const v = rows.find((r) => r.id === id);
    if (!v) throw new Error("not found");
    return v;
  };
  const port: ViewsPort = {
    async list() {
      check("list");
      return rows.map((r) => ({ ...r }));
    },
    async create(name, c) {
      check("create");
      if (name.trim() === "") throw new Refused("名前を入力してください");
      seq += 1;
      const v = { id: idOf(seq), name: name.trim(), content: c };
      rows.push(v);
      return v;
    },
    async save(id, c) {
      check("save");
      const v = { ...find(id), content: c };
      rows = rows.map((r) => (r.id === id ? v : r));
      return v;
    },
    async rename(id, name) {
      check("rename");
      if (rows.some((r) => r.id !== id && r.name === name)) {
        throw new Refused("同じ名前のビューがあります");
      }
      const v = { ...find(id), name };
      rows = rows.map((r) => (r.id === id ? v : r));
      return v;
    },
    async duplicate(id) {
      check("duplicate");
      const from = find(id);
      seq += 1;
      const v = { id: idOf(seq), name: `${from.name} のコピー`, content: from.content };
      rows.push(v);
      return v;
    },
    async remove(id) {
      check("remove");
      rows = rows.filter((r) => r.id !== id);
    },
    async keyImpact() {
      return false;
    },
  };
  return { port, fail };
}

function setup(start: ViewDto[] = []) {
  const { port, fail } = fakePort(start);
  const toasts = createToasts({ set: () => 0, clear: () => undefined });
  const views = createViews({ port, toasts });
  return { views, port, fail, toasts };
}

// deferred は、外から解ける Promise。
function deferred<T>() {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => (resolve = r));
  return { promise, resolve };
}

const bug: ViewDto = { id: idOf(1), name: "バグ", content };

describe("createViews", () => {
  it("returns the refusal of create and keeps the list", async () => {
    const { views } = setup([bug]);
    await views.load();
    expect(await views.create("   ", content)).toBe("名前を入力してください");
    expect(get(views.list).map((v) => v.name)).toEqual(["バグ"]);
  });

  it("adds the created view to the list", async () => {
    const { views } = setup();
    await views.load();
    const v = await views.create("タスク", content);
    expect(v).toMatchObject({ name: "タスク", content });
    expect(get(views.list).map((x) => x.name)).toEqual(["タスク"]);
  });

  it("shows an error toast and returns null when create fails otherwise", async () => {
    const { views, fail, toasts } = setup();
    fail.add("create");
    expect(await views.create("タスク", content)).toBeNull();
    expect(get(toasts.list).map((t) => t.text)).toEqual(["通信できませんでした"]);
  });

  it("renames the view in the list", async () => {
    const { views } = setup([bug]);
    await views.load();
    expect(await views.rename(bug.id, "BUG")).toBeNull();
    expect(get(views.list).map((v) => v.name)).toEqual(["BUG"]);
  });

  it("returns the refusal of rename", async () => {
    const other = { id: idOf(2), name: "タスク", content };
    const { views } = setup([bug, other]);
    await views.load();
    expect(await views.rename(bug.id, "タスク")).toBe("同じ名前のビューがあります");
    expect(get(views.list).map((v) => v.name)).toEqual(["バグ", "タスク"]);
  });

  it("removes the view from the list", async () => {
    const { views } = setup([bug]);
    await views.load();
    expect(await views.remove(bug.id)).toBe(true);
    expect(get(views.list)).toEqual([]);
  });

  it("shows an error toast and returns false when remove fails", async () => {
    const { views, fail, toasts } = setup([bug]);
    await views.load();
    fail.add("remove");
    expect(await views.remove(bug.id)).toBe(false);
    expect(get(views.list)).toHaveLength(1);
    expect(get(toasts.list).map((t) => t.text)).toEqual(["通信できませんでした"]);
  });

  it("saves and duplicates, updating the list", async () => {
    const { views } = setup([bug]);
    await views.load();
    const next = { ...content, query: "kind:task" };
    expect(await views.save(bug.id, next)).toMatchObject({ id: bug.id, content: next });
    const copy = await views.duplicate(bug.id);
    expect(copy).toMatchObject({ name: "バグ のコピー", content: next });
    expect(get(views.list).map((v) => v.content.query)).toEqual(["kind:task", "kind:task"]);
  });

  it("keeps the previous list when load fails", async () => {
    const { views, fail } = setup([bug]);
    await views.load();
    fail.add("list");
    await views.load();
    expect(get(views.list).map((v) => v.name)).toEqual(["バグ"]);
  });
  it("keeps one row per id when a load lands before the write's answer", async () => {
    // サーバーは応答を返す前に出来事を出すので、出来事で読み直した一覧(新しいビューを含む)が先に着くことがある。
    for (const op of ["create", "duplicate"] as const) {
      const { views, port } = setup([bug]);
      await views.load();
      const copy: ViewDto = { id: idOf(2), name: "バグ のコピー", content };
      const answer = deferred<ViewDto>();
      port[op] = () => answer.promise;
      port.list = async () => [bug, copy];
      const pending =
        op === "create" ? views.create("バグ のコピー", content) : views.duplicate(bug.id);
      await views.load();
      // 書いたあとの読み直しは解けないままにして、patch だけの結果を見る。
      port.list = () => new Promise(() => {});
      answer.resolve(copy);
      await pending;
      expect(get(views.list).map((v) => v.id)).toEqual([bug.id, copy.id]);
    }
  });

  it("drops a load that started before a write and lands after it", async () => {
    const { views, port } = setup([bug]);
    await views.load();
    const old = deferred<ViewDto[]>();
    const list = port.list;
    port.list = () => old.promise;
    const loading = views.load();
    port.list = list;
    await views.rename(bug.id, "BUG");
    old.resolve([bug]);
    await loading;
    expect(get(views.list).map((v) => v.name)).toEqual(["BUG"]);
  });

  it("shows the refusal of save, duplicate and remove as an error toast", async () => {
    const { views, port, toasts } = setup([bug]);
    await views.load();
    const refuse = async () => {
      throw new Refused("ビューが見つかりません");
    };
    port.save = refuse;
    port.duplicate = refuse;
    port.remove = refuse;
    expect(await views.save(bug.id, content)).toBeNull();
    expect(await views.duplicate(bug.id)).toBeNull();
    expect(await views.remove(bug.id)).toBe(false);
    expect(get(toasts.list).map((t) => ({ text: t.text, error: t.error }))).toEqual(
      Array(3).fill({ text: "ビューが見つかりません", error: true }),
    );
    expect(get(views.list).map((v) => v.name)).toEqual(["バグ"]);
  });
});
