import { cleanup, screen, waitFor } from "@testing-library/svelte";
import { afterEach, describe, expect, it, vi } from "vitest";
import { FakeConn } from "../../core/testing";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import SchemaCheck from "./SchemaCheck.svelte";

type Finding = { check: string; msg: string; severity: string; where: string; task?: string };

type Opts = {
  findings?: Finding[];
  many?: boolean;
  get?: (store: string) => unknown;
  store?: string;
};

function setup(opts: Opts = {}) {
  const conn = new FakeConn();
  const get = vi.fn((_path: string, init: { params: { query: { store: string } } }) => {
    const r = opts.get?.(init.params.query.store);
    return r !== undefined ? r : ok({ findings: opts.findings ?? [] });
  });
  const view = renderWithCore(
    SchemaCheck,
    {
      store: opts.store ?? "shared",
      many: opts.many ?? false,
      idOf: (ulid: string) => (ulid === "A" ? "REL-A" : undefined),
    },
    { api: { GET: get }, conn },
  );
  return { get, conn, ...view };
}

const tasksChanged = (store: string) => ({ store, ulids: [], removed: [], commit: "c" });

describe("SchemaCheck", () => {
  afterEach(() => cleanup());

  it("asks for the check of the Store, and says the schema and the tasks agree when it finds nothing", async () => {
    const { get } = setup();
    expect(await screen.findByText("スキーマとタスクは合っています")).toBeVisible();
    expect(get).toHaveBeenCalledWith("/api/schema/check", {
      params: { query: { store: "shared" } },
    });
    expect(screen.getByRole("region", { name: "スキーマの検査" })).toBeVisible();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "$ hh schema check",
    );
  });

  it("lists the findings by check, with the ID of the task", async () => {
    setup({
      findings: [
        { check: "schema", msg: "未定義のステータス", severity: "error", where: "statuses[1]" },
        { check: "task", msg: "親が見つかりません", severity: "warning", where: "x", task: "A" },
        { check: "task", msg: "別のタスク", severity: "info", where: "y", task: "B" },
      ],
    });
    expect(await screen.findByText("3 件の指摘があります")).toBeVisible();
    expect(screen.getByRole("heading", { level: 3, name: /^スキーマ/ })).toBeVisible();
    expect(screen.getByText("未定義のステータス")).toBeVisible();
    expect(screen.getByRole("link", { name: "REL-A を開く" }).getAttribute("href")).toBe(
      "/tasks/A",
    );
    // a task whose ID is not known is still opened
    expect(screen.getByRole("link", { name: "タスクを開く" }).getAttribute("href")).toBe(
      "/tasks/B",
    );
  });

  it("writes --store in the command only when there are many Stores", async () => {
    setup({ many: true });
    expect(await screen.findByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "$ hh schema check --store shared",
    );
  });

  it("reads it again at schema.changed and tasks.changed of its Store, and at resync", async () => {
    const { get, conn } = setup();
    await waitFor(() => expect(get).toHaveBeenCalledTimes(1));
    conn.emit("schema.changed", { store: "other" });
    conn.emit("tasks.changed", tasksChanged("other"));
    expect(get).toHaveBeenCalledTimes(1);
    conn.emit("schema.changed", { store: "shared" });
    await waitFor(() => expect(get).toHaveBeenCalledTimes(2));
    conn.emit("tasks.changed", tasksChanged("shared"));
    await waitFor(() => expect(get).toHaveBeenCalledTimes(3));
    conn.emit("resync");
    await waitFor(() => expect(get).toHaveBeenCalledTimes(4));
  });

  it("keeps the findings it had and shows the failure above them when a reload fails", async () => {
    let n = 0;
    const { conn } = setup({
      findings: [{ check: "schema", msg: "古い指摘", severity: "error", where: "w" }],
      get: () => (++n === 2 ? fail(500, "internal", {}, "検査できません") : undefined),
    });
    await screen.findByText("古い指摘");
    conn.emit("schema.changed", { store: "shared" });
    expect(await screen.findByText(/検査の結果を読めません/)).toBeVisible();
    expect(screen.getByText("古い指摘")).toBeVisible();
    // and takes it away once a reload succeeds
    conn.emit("resync");
    await waitFor(() => expect(screen.queryByText(/検査の結果を読めません/)).toBeNull());
  });

  it("does not say it is checking when the first check failed", async () => {
    setup({ get: () => fail(500, "internal", {}, "検査できません") });
    expect(await screen.findByText(/検査の結果を読めません/)).toBeVisible();
    expect(screen.queryByText("検査しています…")).toBeNull();
    expect(screen.queryByText("スキーマとタスクは合っています")).toBeNull();
  });

  it("says it is checking until the first answer", () => {
    setup({ get: () => new Promise(() => {}) });
    expect(screen.getByText("検査しています…")).toBeVisible();
  });

  it("drops the answer of an older request that comes after a newer one", async () => {
    const slow: { resolve?: (v: unknown) => void } = {};
    let n = 0;
    const { conn } = setup({
      get: () => {
        n++;
        if (n === 2)
          return new Promise((r) => {
            slow.resolve = r;
          });
        if (n === 3)
          return ok({
            findings: [{ check: "schema", msg: "新しい", severity: "error", where: "w" }],
          });
        return undefined;
      },
    });
    await screen.findByText("スキーマとタスクは合っています");
    conn.emit("resync");
    conn.emit("resync");
    await screen.findByText("新しい");
    slow.resolve?.(await ok({ findings: [] }));
    await new Promise((r) => setTimeout(r, 0));
    expect(screen.getByText("新しい")).toBeVisible();
    expect(screen.queryByText("スキーマとタスクは合っています")).toBeNull();
  });

  it("reads the other Store when the Store changes, and drops the findings of the old one", async () => {
    const k = setup({
      findings: [{ check: "schema", msg: "shared の指摘", severity: "error", where: "w" }],
      get: (store) => (store === "ops" ? new Promise(() => {}) : undefined),
    });
    await screen.findByText("shared の指摘");
    await k.rerender({ store: "ops" });
    expect(k.get).toHaveBeenLastCalledWith("/api/schema/check", {
      params: { query: { store: "ops" } },
    });
    expect(screen.queryByText("shared の指摘")).toBeNull();
  });

  it("leaves every listener when it goes, and drops an answer that comes after", async () => {
    const offs = [vi.fn(), vi.fn(), vi.fn()];
    let i = 0;
    const conn = new FakeConn();
    vi.spyOn(conn, "on").mockImplementation(() => offs[i++] ?? (() => {}));
    const late: { resolve?: (v: unknown) => void } = {};
    const get = vi.fn(
      () =>
        new Promise((r) => {
          late.resolve = r;
        }),
    );
    const view = renderWithCore(
      SchemaCheck,
      { store: "shared", many: false, idOf: () => undefined },
      { api: { GET: get }, conn },
    );
    view.unmount();
    for (const off of offs) expect(off).toHaveBeenCalledTimes(1);
    late.resolve?.(await ok({ findings: [] }));
    await new Promise((r) => setTimeout(r, 0));
    expect(screen.queryByText("スキーマとタスクは合っています")).toBeNull();
  });
});
