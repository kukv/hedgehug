import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { TaskView } from "../../core/api/client";
import { FakeConn } from "../../core/testing";
import { fail, ok } from "../detail/testkit";
import { emitter, renderWithCore } from "../test-utils";
import { resetToasts } from "../toast.svelte";
import SettingsSchema from "./SettingsSchema.svelte";

const sch = {
  statuses: [
    { name: "todo", label: "未着手", category: "todo", default: true },
    { name: "triage", label: "仕分け", category: "todo" },
    { name: "done", label: "完了", category: "done", default: true },
  ],
  kinds: [
    { name: "project", label: "プロジェクト", role: "container", parents: ["root"] },
    { name: "task", label: "タスク", role: "work", parents: null, default: true },
  ],
  props: {
    priority: { type: "enum", values: ["low", "high"] },
    points: { type: "number", label: "ポイント" },
    relates: { type: "task_ref", multi: true, symmetric: true },
    duplicates: { type: "task_ref", multi: true, label: "重複", inverse: "重複されている" },
  },
  prop_order: ["priority", "points", "relates", "duplicates"],
  workflow: { transitions: { done: ["todo"] }, required: { done: ["points"] } } as Record<
    "transitions" | "required",
    Record<string, string[]>
  >,
  scoped: [
    {
      where: "project:P1",
      statuses: ["todo", "done"],
      workflow: { transitions: { done: ["todo"] }, required: {} },
    },
  ],
};

const t = (ulid: string, over: Partial<TaskView> = {}) =>
  ({
    ulid,
    id: `REL-${ulid}`,
    title: `title ${ulid}`,
    store: "shared",
    status: "todo",
    kind: "task",
    props: {},
    ...over,
  }) as unknown as TaskView;

const tasks = [
  t("P1", { kind: "project", title: "リリース準備", id: "REL-15" }),
  t("A", { status: "todo", props: { priority: "high", points: 3 } }),
  t("B", {
    status: "triage",
    blocked_by: [{ ulid: "A", id: "REL-A", title: "title A" }],
    props: { relates: ["A"] },
  }),
  t("C", { status: "done", props: { priority: "high" } }),
];

type Finding = { check: string; msg: string; severity: string; where: string; task?: string };

type Opts = {
  schema?: typeof sch;
  tasks?: TaskView[];
  findings?: Finding[];
  stores?: string[];
  get?: (op: "listTasks" | "checkSchema", query: { store: string }) => unknown;
  post?: (body: Record<string, unknown>) => unknown;
  loadError?: Error;
  // the schema is not in yet and does not come
  pending?: boolean;
  store?: string;
  tab?: "statuses" | "kinds" | "props" | "relations" | "workflow";
  readOnly?: string;
};

function setup(opts: Opts = {}) {
  const conn = new FakeConn();
  const cache = emitter();
  const state = {
    findings: opts.findings ?? [],
    schema: opts.schema ?? sch,
    failLoad: opts.loadError !== undefined,
    // the cache holds the schema once it has read it
    held: opts.loadError === undefined && !opts.pending,
  };
  const listTasks = vi.fn((query: { store: string }) => {
    const r = opts.get?.("listTasks", query);
    return r !== undefined ? r : ok({ tasks: opts.tasks ?? tasks });
  });
  const checkSchema = vi.fn((query: { store: string }) => {
    const r = opts.get?.("checkSchema", query);
    return r !== undefined ? r : ok({ findings: state.findings });
  });
  const post = vi.fn((body: Record<string, unknown>, _query: { store: string }) =>
    opts.post ? opts.post(body) : ok({ changed: [], skipped: [], commit: null }),
  );
  const reload = vi.fn(async () => {});
  const load = vi.fn(() =>
    opts.pending
      ? new Promise(() => {})
      : state.failLoad
        ? Promise.reject(opts.loadError)
        : Promise.resolve(sch).then((v) => {
            state.held = true;
            cache.emit(); // the cache tells the page what it now holds
            return v;
          }),
  );
  const names = opts.stores ?? ["shared"];
  const view = renderWithCore(
    SettingsSchema,
    { store: opts.store ?? "shared", tab: opts.tab ?? "statuses" },
    {
      api: { listTasks, checkSchema, migrateSchema: post },
      conn,
      stores: {
        stores: () => names.map((name) => ({ name })),
        readOnly: () => opts.readOnly ?? null,
        tone: () => "purple",
        displayId: (_s: string, id: string) => id,
        subscribe: () => () => {},
      },
      schema: {
        get: () => (state.held ? state.schema : undefined),
        load,
        reload,
        subscribe: cache.subscribe,
      },
    },
  );
  return { listTasks, checkSchema, post, reload, load, conn, cache, state, ...view };
}

const row = (name: string | RegExp) => screen.getByRole("row", { name });

describe("SettingsSchema", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/settings/schema/shared");
  });
  afterEach(() => cleanup());

  it("says the schema is shared by the Store and edited in the YAML", async () => {
    setup();
    expect(await screen.findByRole("heading", { level: 1, name: "スキーマ" })).toBeVisible();
    expect(screen.getByText(/この Store を使う共通の定義です/)).toBeVisible();
    expect(
      screen.getByText(
        /YAML（\.hedgehug\/schema\.yaml）を編集してください。名前とマージはこの画面からできます。/,
      ),
    ).toBeVisible();
  });

  it("shows the Store badge in the heading only with two Stores or more", async () => {
    const one = setup();
    await screen.findByRole("heading", { level: 1, name: "スキーマ" });
    expect(screen.queryByTitle("Store: shared")).toBeNull();
    one.unmount();
    setup({ stores: ["shared", "backend"] });
    await screen.findByRole("heading", { level: 1, name: "スキーマ" });
    expect(screen.getByTitle("Store: shared")).toBeVisible();
  });

  it("asks for the tasks and the check of the Store", async () => {
    const { listTasks, checkSchema } = setup({ store: "shared" });
    await waitFor(() => expect(listTasks.mock.calls).toHaveLength(1));
    expect(listTasks.mock.calls[0]?.[0]).toEqual({ store: "shared" });
    expect(checkSchema.mock.calls[0]?.[0]).toEqual({ store: "shared" });
  });

  it("lists the statuses with the category, the default and the number of tasks", async () => {
    setup();
    await waitFor(() => expect(within(row(/^todo/)).getByText("2")).toBeVisible());
    expect(within(row(/^todo/)).getAllByText("未着手")).toHaveLength(2); // the label and the category
    expect(within(row(/^todo/)).getByText("既定")).toBeVisible();
    expect(within(row(/^triage/)).getByText("1")).toBeVisible();
    expect(within(row(/^triage/)).queryByText("既定")).toBeNull();
    expect(row(/^done/)).toHaveTextContent("完了");
  });

  it("shows a dash for the numbers until the tasks are read", async () => {
    setup({ get: (op) => (op === "listTasks" ? new Promise(() => {}) : undefined) });
    await screen.findByRole("heading", { level: 2, name: "ステータス" });
    expect(within(row(/^todo/)).getByText("—")).toBeVisible();
  });

  it("lists the kinds with the parents they can sit under", async () => {
    setup({ tab: "kinds" });
    expect(await screen.findByRole("heading", { level: 2, name: "種類" })).toBeVisible();
    expect(within(row(/^project/)).getByText("ルート")).toBeVisible();
    expect(within(row(/^task/)).getByText("どこにでも")).toBeVisible();
    expect(within(row(/^task/)).getByText("3")).toBeVisible();
  });

  it("lists the properties and the values of an enum with their own numbers, without the relations", async () => {
    setup({ tab: "props" });
    await screen.findByRole("row", { name: /^points/ });
    expect(within(row(/^points/)).getByText("ポイント")).toBeVisible();
    expect(within(row(/^points/)).getByText("number")).toBeVisible();
    expect(screen.queryByRole("row", { name: /^relates/ })).toBeNull();
    await waitFor(() => expect(within(row(/^priority enum/)).getByText("2")).toBeVisible());
    expect(within(row(/priority › low/)).getByText("0")).toBeVisible();
    expect(within(row(/priority › high/)).getByText("2")).toBeVisible();
  });

  it("lists the properties and the relations in the order of the schema", async () => {
    const ordered = {
      ...sch,
      props: {
        a: { type: "number" },
        b: { type: "number" },
        r1: { type: "task_ref" },
        r2: { type: "task_ref" },
      },
      prop_order: ["b", "a", "r2", "r1"],
    };
    const { unmount } = setup({ tab: "props", schema: ordered as unknown as typeof sch });
    await screen.findByRole("row", { name: /^b / });
    const names = (rows: HTMLElement[]) => rows.map((r) => r.textContent?.[0]);
    expect(names(screen.getAllByRole("row").slice(1))).toEqual(["b", "a"]);
    unmount();
    setup({ tab: "relations", schema: ordered as unknown as typeof sch });
    await screen.findByRole("row", { name: /^r2/ });
    expect(
      screen
        .getAllByRole("row")
        .slice(2)
        .map((r) => r.textContent?.slice(0, 2)),
    ).toEqual(["r2", "r1"]);
  });

  it("lists the relations: blocked_by built in and without actions, then the task_ref properties", async () => {
    setup({ tab: "relations" });
    expect(await screen.findByRole("heading", { level: 2, name: "関連" })).toBeVisible();
    const names = screen
      .getAllByRole("row")
      .slice(1)
      .map((r) => r.querySelector("th")?.textContent);
    expect(names).toEqual(["blocked_by", "relates", "duplicates"]);
    await waitFor(() => expect(within(row(/^blocked_by/)).getByText("1")).toBeVisible());
    expect(within(row(/^blocked_by/)).getByText("ブロックされている")).toBeVisible();
    expect(within(row(/^blocked_by/)).getByText("複数 · 組み込み")).toBeVisible();
    expect(within(row(/^blocked_by/)).queryByRole("button")).toBeNull();
    expect(within(row(/^relates/)).getByText("複数 · 双方向")).toBeVisible();
    expect(within(row(/^relates/)).getByText("1")).toBeVisible();
    expect(within(row(/^duplicates/)).getByText("複数 · 逆方向: 重複されている")).toBeVisible();
    expect(within(row(/^duplicates/)).getByText("0")).toBeVisible();
  });

  it("offers only the other relations as where a relation is merged to", async () => {
    setup({ tab: "relations" });
    await fireEvent.click(await screen.findByRole("button", { name: "マージ…（relates）" }));
    const select = screen.getByRole("combobox", { name: "マージ先" }) as HTMLSelectElement;
    expect([...select.options].map((o) => o.value)).toEqual(["", "duplicates"]);
  });

  it("shows the transitions and the required fields of the workflow", async () => {
    setup({ tab: "workflow" });
    expect(await screen.findByText(/から移せるのは todo だけ/)).toBeVisible();
    expect(screen.getByText(/に入るには points が必要/)).toBeVisible();
  });

  it("says so when the workflow has no constraint", async () => {
    setup({ tab: "workflow", schema: { ...sch, workflow: { transitions: {}, required: {} } } });
    expect(await screen.findByText("制約はありません")).toBeVisible();
    expect(screen.getByText("必須の項目はありません")).toBeVisible();
  });

  it("lists the projects with an override, with a link to their settings", async () => {
    setup();
    const link = await screen.findByRole("link", { name: /リリース準備/ });
    expect(link.getAttribute("href")).toBe("/projects/P1/settings");
    expect(link).toHaveTextContent("REL-15");
    expect(screen.getByText("ステータス 2 件を表示 · 遷移の制約 1 件")).toBeVisible();
  });

  it("says so when no project has an override", async () => {
    setup({ schema: { ...sch, scoped: [] } });
    expect(await screen.findByText("上書きのあるプロジェクトはありません")).toBeVisible();
  });

  it("says the schema and the tasks agree when the check finds nothing", async () => {
    setup();
    expect(await screen.findByText("スキーマとタスクは合っています")).toBeVisible();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "$ hh schema check",
    );
  });

  it("names each table by its heading", async () => {
    const { rerender } = setup();
    expect(await screen.findByRole("table", { name: "ステータス" })).toBeVisible();
    await rerender({ tab: "kinds" });
    expect(screen.getByRole("table", { name: "種類" })).toBeVisible();
    await rerender({ tab: "props" });
    expect(screen.getByRole("table", { name: "プロパティ" })).toBeVisible();
    await rerender({ tab: "relations" });
    expect(screen.getByRole("table", { name: "関連" })).toBeVisible();
  });

  it("joins the empty cells of a value row into one", async () => {
    setup({ tab: "props" });
    const value = await screen.findByRole("row", { name: /priority › low/ });
    expect(value.querySelectorAll("td[colspan='3']")).toHaveLength(1);
    expect(value.querySelectorAll("td:empty")).toHaveLength(1);
  });

  it("each tab is a link to its own address, and the tab of the address is the current one", async () => {
    setup({ tab: "props", store: "org-b/ops" });
    await screen.findByRole("table", { name: "プロパティ" });
    const links = within(screen.getByRole("navigation", { name: "スキーマの種類" })).getAllByRole(
      "link",
    );
    expect(links.map((l) => l.getAttribute("href"))).toEqual([
      "/settings/schema/org-b%2Fops",
      "/settings/schema/org-b%2Fops?tab=kinds",
      "/settings/schema/org-b%2Fops?tab=props",
      "/settings/schema/org-b%2Fops?tab=relations",
      "/settings/schema/org-b%2Fops?tab=workflow",
      "/settings/schema/org-b%2Fops?tab=templates",
    ]);
    expect(links.map((l) => l.getAttribute("aria-current"))).toEqual([
      null,
      null,
      "page",
      null,
      null,
      null,
    ]);
    await fireEvent.click(links[4] as HTMLElement);
    expect(location.pathname + location.search).toBe("/settings/schema/org-b%2Fops?tab=workflow");
  });

  it("disables the rename and the merge of a read-only Store, with the reason", async () => {
    setup({ readOnly: "the remote cannot be written" });
    const rename = await screen.findByRole("button", { name: "名前…（triage）" });
    expect(rename).toBeDisabled();
    expect(rename).toHaveAttribute("title", "この Store は閲覧のみです（remote に書き込めません）");
    for (const b of screen.getAllByRole("button", { name: /^(名前|マージ)…/ })) {
      expect(b).toBeDisabled();
    }
    rename.click(); // a disabled button takes no click
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("closes the dialog when the Store changes, so that it cannot write to the other Store", async () => {
    const k = setup();
    await fireEvent.click(await screen.findByRole("button", { name: "名前…（triage）" }));
    expect(screen.getByRole("dialog")).toBeVisible();
    await k.rerender({ store: "ops" });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  describe("reading again", () => {
    it("asks for the check at schema.changed of this Store, not of another", async () => {
      const { listTasks, checkSchema, conn } = setup();
      await waitFor(() => expect(checkSchema.mock.calls).toHaveLength(1));
      conn.emit("schema.changed", { store: "other" });
      conn.emit("schema.changed", { store: "shared" });
      await waitFor(() => expect(checkSchema.mock.calls).toHaveLength(2));
      expect(listTasks.mock.calls).toHaveLength(1);
    });

    it("asks for both at tasks.changed of this Store, and at resync", async () => {
      const { listTasks, checkSchema, conn } = setup();
      await waitFor(() => expect(listTasks.mock.calls).toHaveLength(1));
      conn.emit("tasks.changed", { store: "other", ulids: [], removed: [], commit: "c" });
      expect(listTasks.mock.calls).toHaveLength(1);
      conn.emit("tasks.changed", { store: "shared", ulids: [], removed: [], commit: "c" });
      await waitFor(() => expect(listTasks.mock.calls).toHaveLength(2));
      expect(checkSchema.mock.calls).toHaveLength(2);
      conn.emit("resync");
      await waitFor(() => expect(listTasks.mock.calls).toHaveLength(3));
      expect(checkSchema.mock.calls).toHaveLength(3);
    });

    it("keeps the numbers it read and shows the failure above them when a reload fails", async () => {
      let n = 0;
      const { conn } = setup({
        get: (op) => {
          if (op === "listTasks" && ++n > 1) return fail(500, "internal", {}, "壊れました");
          return undefined;
        },
      });
      await waitFor(() => expect(within(row(/^todo/)).getByText("2")).toBeVisible());
      conn.emit("tasks.changed", { store: "shared", ulids: [], removed: [], commit: "c" });
      expect(await screen.findByText(/タスクの件数を読み込めません/)).toBeVisible();
      expect(within(row(/^todo/)).getByText("2")).toBeVisible();
    });

    it("takes the failure away once a reload succeeds", async () => {
      let n = 0;
      const { conn } = setup({
        get: (op) =>
          op === "listTasks" && ++n === 2 ? fail(500, "internal", {}, "壊れました") : undefined,
      });
      await screen.findByText("スキーマとタスクは合っています");
      conn.emit("resync");
      await screen.findByText(/タスクの件数を読み込めません/);
      conn.emit("resync");
      await waitFor(() => expect(screen.queryByText(/タスクの件数を読み込めません/)).toBeNull());
    });

    it("drops the answer of an older request that comes after a newer one", async () => {
      const slow: { resolve?: (v: unknown) => void } = {};
      let n = 0;
      const { conn } = setup({
        get: (op) => {
          if (op !== "listTasks") return undefined;
          n++;
          if (n === 2)
            return new Promise((r) => {
              slow.resolve = r;
            });
          if (n === 3) return ok({ tasks: [t("N1"), t("N2"), t("N3"), t("N4"), t("N5")] });
          return undefined;
        },
      });
      await waitFor(() => expect(within(row(/^todo/)).getByText("2")).toBeVisible());
      conn.emit("resync");
      conn.emit("resync");
      await waitFor(() => expect(within(row(/^todo/)).getByText("5")).toBeVisible());
      slow.resolve?.(await ok({ tasks: [t("Z")] }));
      await new Promise((r) => setTimeout(r, 0));
      expect(within(row(/^todo/)).getByText("5")).toBeVisible();
    });

    it("leaves every listener when it goes", () => {
      const offs = Array.from({ length: 5 }, () => vi.fn()); // two here, three in the check
      let i = 0;
      const conn = new FakeConn();
      vi.spyOn(conn, "on").mockImplementation(() => offs[i++] ?? (() => {}));
      const view = renderWithCore(
        SettingsSchema,
        { store: "shared", tab: "statuses" },
        {
          api: {
            listTasks: vi.fn(() => ok({ tasks: [], findings: [] })),
            checkSchema: vi.fn(() => ok({ tasks: [], findings: [] })),
          },
          conn,
          stores: {
            stores: () => [],
            readOnly: () => null,
            tone: () => "purple",
            displayId: (_s: string, id: string) => id,
            subscribe: () => () => {},
          },
          schema: { get: () => sch, load: async () => sch, subscribe: () => () => {} },
        },
      );
      view.unmount();
      for (const off of offs) expect(off).toHaveBeenCalledTimes(1);
    });

    it("reads the other Store when the Store changes, and does not show the old numbers meanwhile", async () => {
      const k = setup({
        get: (op, query) =>
          op === "listTasks" && query.store === "ops" ? new Promise(() => {}) : undefined,
      });
      await waitFor(() => expect(within(row(/^todo/)).getByText("2")).toBeVisible());
      await k.rerender({ store: "ops" });
      expect(k.listTasks.mock.calls.at(-1)?.[0]).toEqual({ store: "ops" });
      expect(k.checkSchema.mock.calls.at(-1)?.[0]).toEqual({ store: "ops" });
      // the numbers of the other Store are gone, and the ones of this Store are not in
      expect(within(row(/^todo/)).queryByText("2")).toBeNull();
      expect(within(row(/^todo/)).getByText("—")).toBeVisible();
    });
  });

  describe("when the schema cannot be read", () => {
    it("says so, with the words of the refusal", async () => {
      setup({ loadError: new Error("読めません") });
      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent("スキーマを読み込めません");
    });

    it("reads the schema again with the button, and shows it once it comes", async () => {
      const k = setup({ loadError: new Error("読めません") });
      await screen.findByRole("alert");
      k.state.failLoad = false;
      const retry = screen.getByRole("button", { name: "もう一度読み込む" });
      retry.focus();
      await fireEvent.click(retry);
      // the button goes with the error: the focus is not lost with it
      expect(screen.getByRole("heading", { level: 1, name: "スキーマ" })).toHaveFocus();
      expect(await screen.findByRole("table", { name: "ステータス" })).toBeVisible();
      expect(screen.queryByRole("alert")).toBeNull();
    });

    it("reads the schema again at resync while it is not in, and not when it is", async () => {
      const k = setup({ loadError: new Error("読めません") });
      await screen.findByRole("alert");
      expect(k.load).toHaveBeenCalledTimes(1);
      k.conn.emit("resync");
      await waitFor(() => expect(k.load).toHaveBeenCalledTimes(2));
      k.state.failLoad = false;
      k.conn.emit("resync");
      await screen.findByRole("table", { name: "ステータス" });
      const n = k.load.mock.calls.length;
      k.conn.emit("resync");
      expect(k.load).toHaveBeenCalledTimes(n);
    });

    it("says it is reading until the schema is in", () => {
      setup({ pending: true });
      expect(screen.getByText("読み込み中…")).toBeVisible();
      expect(screen.queryByRole("table")).toBeNull();
    });
  });

  describe("migrate", () => {
    it("opens the dialog of the status, rename and merge both", async () => {
      setup();
      await fireEvent.click(await screen.findByRole("button", { name: "名前…（triage）" }));
      expect(
        screen.getByRole("dialog", { name: "ステータス「triage」の名前を変更" }),
      ).toBeVisible();
      await fireEvent.click(screen.getByRole("button", { name: "キャンセル" }));
      expect(screen.queryByRole("dialog")).toBeNull();
      await fireEvent.click(screen.getByRole("button", { name: "マージ…（triage）" }));
      expect(screen.getByRole("dialog", { name: "ステータス「triage」をマージ" })).toBeVisible();
    });

    it("opens it for a kind, a property and a value", async () => {
      const { rerender } = setup({ tab: "kinds" });
      await fireEvent.click(await screen.findByRole("button", { name: "名前…（task）" }));
      expect(screen.getByRole("dialog", { name: "種類「task」の名前を変更" })).toBeVisible();
      await fireEvent.click(screen.getByRole("button", { name: "キャンセル" }));
      await rerender({ tab: "props" });
      await fireEvent.click(screen.getByRole("button", { name: "名前…（points）" }));
      expect(
        screen.getByRole("dialog", { name: "プロパティ「points」の名前を変更" }),
      ).toBeVisible();
      await fireEvent.click(screen.getByRole("button", { name: "キャンセル" }));
      await fireEvent.click(screen.getByRole("button", { name: "マージ…（priority の low）" }));
      expect(screen.getByRole("dialog", { name: "値「low」をマージ" })).toBeVisible();
    });

    it("offers no merge when nothing else is defined", async () => {
      setup({ tab: "kinds", schema: { ...sch, kinds: [sch.kinds[1] as never] } });
      expect(await screen.findByRole("button", { name: "名前…（task）" })).toBeVisible();
      expect(screen.queryByRole("button", { name: "マージ…（task）" })).toBeNull();
    });

    async function renameTriage(k: ReturnType<typeof setup>) {
      const open = await screen.findByRole("button", { name: "名前…（triage）" });
      open.focus(); // a click focuses the button in a browser
      await fireEvent.click(open);
      await fireEvent.input(screen.getByRole("textbox", { name: "新しい名前" }), {
        target: { value: "review" },
      });
      await vi.advanceTimersByTimeAsync(500);
      const run = screen.getByRole("button", { name: "名前を変更" });
      await waitFor(() => expect(run).toBeEnabled());
      await fireEvent.click(run);
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
      return k;
    }

    it("reads the tasks and the check again after it ran", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      try {
        const k = setup({
          post: (b) =>
            ok({ changed: [{ ulid: "B" }], skipped: [], commit: b.dry_run ? null : "c1" }),
        });
        await waitFor(() => expect(k.listTasks.mock.calls).toHaveLength(1));
        await renameTriage(k);
        expect(k.post).toHaveBeenLastCalledWith(
          { target: "status", old: "triage", new: "review" },
          { store: "shared" },
        );
        await waitFor(() => expect(k.listTasks.mock.calls).toHaveLength(2));
        expect(k.checkSchema.mock.calls).toHaveLength(2);
      } finally {
        vi.useRealTimers();
      }
    });

    it("gives the focus back to the row, or to the heading when the row is gone", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      try {
        const k = setup({
          post: (b) => ok({ changed: [], skipped: [], commit: b.dry_run ? null : "c1" }),
        });
        await renameTriage(k);
        expect(screen.getByRole("button", { name: "名前…（triage）" })).toHaveFocus();
        const again = screen.getByRole("button", { name: "名前…（triage）" });
        again.focus();
        await fireEvent.click(again);
        // the rename takes the row away
        k.state.schema = {
          ...sch,
          statuses: sch.statuses.map((s) => (s.name === "triage" ? { ...s, name: "review" } : s)),
        };
        k.cache.emit();
        await fireEvent.input(screen.getByRole("textbox", { name: "新しい名前" }), {
          target: { value: "x" },
        });
        await vi.advanceTimersByTimeAsync(500);
        const run = screen.getByRole("button", { name: "名前を変更" });
        await waitFor(() => expect(run).toBeEnabled());
        await fireEvent.click(run);
        await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
        await waitFor(() =>
          expect(screen.getByRole("heading", { level: 1, name: "スキーマ" })).toHaveFocus(),
        );
      } finally {
        vi.useRealTimers();
      }
    });
  });
});
