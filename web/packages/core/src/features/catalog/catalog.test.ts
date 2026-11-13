import { describe, expect, it } from "vitest";
import { get } from "../../reactive/store";
import { createCatalog, writableWorkspaces } from "./catalog";
import { fakeCatalogPort } from "./fixtures.test-util";
import { readOnlyBand } from "./readonly";

describe("catalog", () => {
  it("loads stores, workspaces and projects with their display IDs", async () => {
    const catalog = createCatalog(fakeCatalogPort());
    expect(get(catalog.stores)).toEqual([]);
    await catalog.load();
    const stores = get(catalog.stores);
    expect(stores.map((s) => s.name)).toEqual(["work", "ro"]);
    expect(stores[0]?.workspaces[0]?.projects.map((p) => p.displayId)).toEqual(["HH-P1", "HH-P2"]);
    expect(catalog.store("S2")?.readOnly).toBe(true);
    expect(catalog.workspace("S1", "S1-W1")?.prefix).toBe("HH");
    expect(catalog.project("S1", "S1-P2")?.title).toBe("プロジェクト 2");
    expect(catalog.project("S1", "S2-P2")).toBeUndefined();
  });

  it("keeps the list when loading fails", async () => {
    const port = fakeCatalogPort();
    const catalog = createCatalog(port);
    await catalog.load();
    port.listStores = async () => {
      throw new Error("offline");
    };
    await expect(catalog.load()).rejects.toThrow("offline");
    expect(get(catalog.stores)).toHaveLength(2);
  });

  it("uses only the last of overlapping loads", async () => {
    const port = fakeCatalogPort();
    let release: () => void = () => {};
    const slow = new Promise<void>((r) => {
      release = r;
    });
    let calls = 0;
    const catalog = createCatalog({
      ...port,
      listStores: async () => {
        calls += 1;
        // 1 回目は遅く、古い一覧を返す。2 回目はすぐ、新しい一覧を返す。
        if (calls === 1) {
          await slow;
          return [{ id: "OLD", name: "old", color: "teal", readOnly: false }];
        }
        return port.listStores();
      },
    });
    const first = catalog.load();
    await catalog.load();
    release();
    await first;
    expect(get(catalog.stores).map((s) => s.name)).toEqual(["work", "ro"]);
  });

  it("waits for the last of overlapping loads before a superseded load returns", async () => {
    const port = fakeCatalogPort();
    let release: () => void = () => {};
    const slow = new Promise<void>((r) => {
      release = r;
    });
    let calls = 0;
    const catalog = createCatalog({
      ...port,
      listStores: async () => {
        calls += 1;
        // 2 回目(最後に始めた読み直し)が遅い。1 回目は先に終わるが、結果は捨てられる。
        if (calls === 2) await slow;
        return port.listStores();
      },
    });
    const first = catalog.load();
    const second = catalog.load();
    let firstDone = false;
    void first.then(() => {
      firstDone = true;
    });
    for (let i = 0; i < 20; i++) await Promise.resolve();
    expect(firstDone).toBe(false);
    release();
    await first;
    expect(get(catalog.stores).map((s) => s.name)).toEqual(["work", "ro"]);
    await second;
  });

  it("puts the old prefixes of a workspace on its node, empty when the port gives none", async () => {
    const port = fakeCatalogPort();
    const list = port.listWorkspaces.bind(port);
    port.listWorkspaces = async (s) =>
      (await list(s)).map((w) => (s === "S1" ? { ...w, prefix: "WEB", oldPrefixes: ["HH"] } : w));
    const catalog = createCatalog(port);
    await catalog.load();
    expect(get(catalog.stores).map((s) => s.workspaces[0]?.oldPrefixes)).toEqual([["HH"], []]);
  });

  it("lists the workspaces of writable stores only", async () => {
    const catalog = createCatalog(fakeCatalogPort());
    await catalog.load();
    expect(writableWorkspaces(get(catalog.stores)).map((w) => w.id)).toEqual(["S1-W1"]);
  });
});

describe("readOnlyBand", () => {
  it("is the sentence of spec 01", () => {
    expect(readOnlyBand("ro")).toBe(
      "ro は閲覧のみの Store です。閲覧はできますが、作成・変更・削除はできません。",
    );
  });
});
