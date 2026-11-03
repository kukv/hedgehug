import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { DoctorReport, Finding } from "../../core/doctor";
import { FakeConn } from "../../core/testing";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import DoctorSection from "./DoctorSection.svelte";

const finding = (over: Partial<Finding> & { check: string }): Finding => ({
  msg: "a problem",
  severity: "error",
  where: "somewhere",
  ...over,
});

const report = (over: Partial<DoctorReport> = {}): DoctorReport => ({
  store: "a",
  findings: [],
  repaired: [],
  ...over,
});

const taskFinding = finding({
  check: "task",
  where: "DM-2 (U2) child",
  msg: "parent: the parent is deleted",
  hint: "hedgehug doctor --fix fixes it",
  task: "U2",
});
const fileFinding = finding({
  check: "file",
  where: "tasks/x.md",
  msg: "does not decode",
  severity: "warning",
});

type Over = {
  reports?: Record<string, DoctorReport>;
  task?: (ulid: string) => { id: string } | undefined;
  taskError?: boolean;
};

function setup(over: Over = {}, props: { store?: string; many?: boolean } = {}) {
  const reports = over.reports ?? { a: report() };
  const conn = new FakeConn();
  const api = {
    GET: vi.fn((path: string, opts: { params: { query?: { store?: string }; path?: object } }) => {
      if (path === "/api/doctor") {
        const r = reports[opts.params.query?.store ?? ""];
        return ok(r ? [r] : []);
      }
      return over.taskError ? fail(404, "task.not_found") : ok({ id: "DM-2", ulid: "U2" });
    }),
    POST: vi.fn((_p: string, opts: { params: { query?: { store?: string } } }) =>
      ok([reports[opts.params.query?.store ?? ""] ?? report()]),
    ),
  };
  const view = renderWithCore(
    DoctorSection,
    { store: props.store ?? "a", many: props.many ?? false },
    {
      api,
      conn,
      cache: { task: over.task ?? (() => undefined), subscribe: () => () => {} },
      stores: { tone: () => "teal", subscribe: () => () => {} },
    },
  );
  return { api, conn, view };
}

const region = (name = "診断: a") => screen.getByRole("region", { name });

beforeEach(() => {
  resetToasts();
  history.replaceState(null, "", "/settings/general");
});
afterEach(() => cleanup());

describe("DoctorSection: the diagnosis", () => {
  it("asks for the store's diagnosis once, when it is drawn", async () => {
    const { api, conn } = setup();
    await waitFor(() =>
      expect(within(region()).getByText("問題は見つかりませんでした")).toBeTruthy(),
    );
    expect(api.GET).toHaveBeenCalledTimes(1);
    expect(api.GET).toHaveBeenCalledWith("/api/doctor", { params: { query: { store: "a" } } });
    // a heavy check: neither a change of the tasks nor a resync asks again
    conn.emit("tasks.changed", { store: "a" });
    conn.emit("resync");
    conn.emit("schema.changed", { store: "a" });
    await Promise.resolve();
    expect(api.GET).toHaveBeenCalledTimes(1);
  });

  it("groups the findings by check, with the severity as a word and a shape", async () => {
    setup({ reports: { a: report({ findings: [fileFinding, taskFinding] }) } });
    const r = region();
    await waitFor(() => expect(within(r).getByRole("heading", { name: /^ファイル/ })).toBeTruthy());
    expect(within(r).getByRole("heading", { name: /^タスク/ })).toBeTruthy();
    // the words of the server as they are
    expect(within(r).getByText("does not decode")).toBeTruthy();
    expect(within(r).getByText("parent: the parent is deleted")).toBeTruthy();
    expect(within(r).getByText("hedgehug doctor --fix fixes it")).toBeTruthy();
    expect(within(r).getByText("tasks/x.md")).toBeTruthy();
    const items = within(r).getAllByRole("listitem");
    expect(items.map((i) => i.getAttribute("data-severity"))).toEqual(["warning", "error"]);
    expect(within(items[0] as HTMLElement).getByText("警告")).toBeTruthy();
    expect(within(items[1] as HTMLElement).getByText("エラー")).toBeTruthy();
    // a shape that is not the colour alone
    expect(items[0]?.querySelector("svg")).toBeTruthy();
    expect(items[1]?.querySelector("svg")).toBeTruthy();
    expect(within(r).getByText("エラー 1 件 · 警告 1 件")).toBeTruthy();
  });

  it("a check it does not know is written as it is, and an empty severity is an error", async () => {
    setup({
      reports: { a: report({ findings: [finding({ check: "brand_new", severity: "" })] }) },
    });
    await waitFor(() => expect(within(region()).getByRole("heading", { name: /^brand_new/ })));
    expect(within(region()).getByText("エラー")).toBeTruthy();
  });

  it("a task's finding opens the task: the ID is the cache's, else the task is asked for", async () => {
    const { api } = setup({ reports: { a: report({ findings: [taskFinding] }) } }, { many: false });
    const link = await within(region()).findByRole("link", { name: "DM-2 を開く" });
    expect(link.getAttribute("href")).toBe("/tasks/U2");
    expect(api.GET).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "U2" }, query: { store: "a" } },
    });
  });

  it("an ID in the cache is not asked for", async () => {
    const { api } = setup({
      reports: { a: report({ findings: [taskFinding] }) },
      task: () => ({ id: "DM-9" }),
    });
    await within(region()).findByRole("link", { name: "DM-9 を開く" });
    expect(api.GET).toHaveBeenCalledTimes(1);
  });

  it("a task that cannot be read is still opened, without an ID or a short ULID", async () => {
    setup({ reports: { a: report({ findings: [taskFinding] }) }, taskError: true });
    const link = await within(region()).findByRole("link", { name: "タスクを開く" });
    expect(link.getAttribute("href")).toBe("/tasks/U2");
    expect(link.textContent).not.toContain("U2");
  });

  it("only a finding of a task has the link", async () => {
    setup({ reports: { a: report({ findings: [fileFinding] }) } });
    await within(region()).findByText("does not decode");
    expect(within(region()).queryByRole("link")).toBeNull();
  });

  it("asks again with 「もう一度実行」, which waits while it asks", async () => {
    const { api } = setup();
    await within(region()).findByText("問題は見つかりませんでした");
    const again = within(region()).getByRole("button", { name: "もう一度実行" });
    await fireEvent.click(again);
    expect((again as HTMLButtonElement).disabled).toBe(true);
    await waitFor(() => expect((again as HTMLButtonElement).disabled).toBe(false));
    expect(api.GET).toHaveBeenCalledTimes(2);
  });

  it("a failure to read is said, and a failure of the second read keeps the first result", async () => {
    const { api } = setup({ reports: { a: report({ findings: [fileFinding] }) } });
    await within(region()).findByText("does not decode");
    api.GET.mockImplementation((() => fail(500, "internal")) as never);
    await fireEvent.click(within(region()).getByRole("button", { name: "もう一度実行" }));
    await waitFor(() =>
      expect(within(region()).getByRole("alert").textContent).toContain(
        "サーバーで予期しないエラー",
      ),
    );
    expect(within(region()).getByText("does not decode")).toBeTruthy();
  });

  it("a failure of the first read is said, not swallowed", async () => {
    const { api } = setup();
    cleanup();
    api.GET.mockImplementation((() => fail(500, "internal")) as never);
    renderWithCore(
      DoctorSection,
      { store: "a", many: false },
      {
        api,
        conn: new FakeConn(),
        cache: { task: () => undefined, subscribe: () => () => {} },
        stores: { tone: () => "teal", subscribe: () => () => {} },
      },
    );
    await waitFor(() =>
      expect(within(region()).getByRole("alert").textContent).toContain(
        "サーバーで予期しないエラー",
      ),
    );
    expect(within(region()).queryByText("問題は見つかりませんでした")).toBeNull();
  });

  it("the store badge is for two stores or more", async () => {
    const one = setup({}, { many: false });
    await within(region()).findByText("問題は見つかりませんでした");
    expect(one.view.container.querySelector(".badge")).toBeNull();
    cleanup();
    const two = setup({}, { many: true });
    await within(region()).findByText("問題は見つかりませんでした");
    expect(two.view.container.querySelector(".badge")).toBeTruthy();
  });

  it("an answer that a newer store overtook is dropped", async () => {
    const late: { resolve: (v: unknown) => void }[] = [];
    const api = {
      GET: vi.fn((_p: string, o: { params: { query: { store: string } } }) =>
        o.params.query.store === "a"
          ? new Promise((resolve) => late.push({ resolve }))
          : ok([report({ store: "b", findings: [fileFinding] })]),
      ),
      POST: vi.fn(),
    };
    const view = renderWithCore(
      DoctorSection,
      { store: "a", many: false },
      {
        api,
        conn: new FakeConn(),
        cache: { task: () => undefined, subscribe: () => () => {} },
        stores: { tone: () => "teal", subscribe: () => () => {} },
      },
    );
    await view.rerender({ store: "b", many: false });
    await within(region("診断: b")).findByText("does not decode");
    late[0]?.resolve({
      data: [report({ store: "a", findings: [taskFinding] })],
      response: new Response(null, { status: 200 }),
    });
    await Promise.resolve();
    await Promise.resolve();
    expect(within(region("診断: b")).queryByText("parent: the parent is deleted")).toBeNull();
    expect(within(region("診断: b")).getByText("does not decode")).toBeTruthy();
  });

  it("an answer that comes after it is gone changes nothing", async () => {
    let resolve: (v: unknown) => void = () => {};
    const api = {
      GET: vi.fn(() => new Promise((r) => (resolve = r))),
      POST: vi.fn(),
    };
    const view = renderWithCore(
      DoctorSection,
      { store: "a", many: false },
      {
        api,
        conn: new FakeConn(),
        cache: { task: () => undefined, subscribe: () => () => {} },
        stores: { tone: () => "teal", subscribe: () => () => {} },
      },
    );
    view.unmount();
    resolve({
      data: [report({ findings: [taskFinding] })],
      response: new Response(null, { status: 200 }),
    });
    await Promise.resolve();
    await Promise.resolve();
    // the task is not asked for either: nothing but the diagnosis was sent
    expect(api.GET).toHaveBeenCalledTimes(1);
  });

  it("does not build what the screen leaves out", async () => {
    setup({ reports: { a: report({ findings: [fileFinding] }) } });
    await within(region()).findByText("does not decode");
    const text = region().textContent ?? "";
    for (const t of ["作り直", "repack", "登録する", "--moved"]) expect(text).not.toContain(t);
  });
});

describe("DoctorSection: the repair", () => {
  it("has no 「修復…」 when nothing was found", async () => {
    setup();
    await within(region()).findByText("問題は見つかりませんでした");
    expect(within(region()).queryByRole("button", { name: /修復/ })).toBeNull();
  });

  it("asks first: what is repaired, and the command with --store only for two stores", async () => {
    setup({ reports: { a: report({ findings: [taskFinding] }) } }, { many: true });
    await within(region()).findByText("parent: the parent is deleted");
    await fireEvent.click(within(region()).getByRole("button", { name: "修復…" }));
    const d = screen.getByRole("dialog", { name: "doctor で修復する" });
    for (const t of [
      "親の循環",
      "削除済みの親",
      "親と違うワークスペース",
      "重なった ID",
      "1 コミット",
    ]) {
      expect(d.textContent).toContain(t);
    }
    expect(d.textContent).toContain("force push");
    expect(
      within(d).getByRole("button", { name: "コマンドをコピー: hh doctor --fix --store a" }),
    ).toBeTruthy();
  });

  it("the command has no --store for one store", async () => {
    setup({ reports: { a: report({ findings: [taskFinding] }) } });
    await within(region()).findByText("parent: the parent is deleted");
    await fireEvent.click(within(region()).getByRole("button", { name: "修復…" }));
    expect(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: "コマンドをコピー: hh doctor --fix",
      }),
    ).toBeTruthy();
  });

  it("repairs, tells what was repaired and what is left, and puts the focus on the result", async () => {
    const { api } = setup({ reports: { a: report({ findings: [taskFinding, fileFinding] }) } });
    await within(region()).findByText("does not decode");
    api.POST.mockImplementation(() =>
      ok([
        report({
          repaired: [
            {
              ulid: "U2",
              id: "DM-2",
              title: "child",
              kind: "parent_deleted",
              value: { from: "U1", to: null },
            },
            {
              ulid: "U3",
              id: "DM-3",
              title: "other",
              kind: "id_collision",
              value: { old: "DM-3", new: "DM-4" },
            },
          ],
          findings: [fileFinding],
        }),
      ]),
    );
    await fireEvent.click(within(region()).getByRole("button", { name: "修復…" }));
    await fireEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "修復する" }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(api.POST).toHaveBeenCalledWith("/api/doctor/fix", { params: { query: { store: "a" } } });
    expect(toasts.list.map((t) => t.message)).toEqual(["2 件を修復しました"]);
    const result = within(region()).getByRole("region", { name: "修復の結果" });
    expect(result.textContent).toContain("DM-2");
    expect(result.textContent).toContain("親が削除されたため、ルートに移しました");
    expect(result.textContent).toContain("ID が重なっていたため、DM-3 から DM-4 に振り直しました");
    await waitFor(() => expect(document.activeElement).toBe(result));
    // what is left
    expect(within(region()).getByText("does not decode")).toBeTruthy();
    expect(within(region()).queryByText("parent: the parent is deleted")).toBeNull();
  });

  it("tells a store cloned again from origin", async () => {
    const { api } = setup({ reports: { a: report({ findings: [fileFinding] }) } });
    await within(region()).findByText("does not decode");
    api.POST.mockImplementation(() =>
      ok([
        report({
          rebuilt: { replayed: 3, dropped: ["DM-7"], kept: "/old/store" },
          repaired: [],
          findings: [],
        }),
      ]),
    );
    await fireEvent.click(within(region()).getByRole("button", { name: "修復…" }));
    await fireEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "修復する" }),
    );
    const result = await within(region()).findByRole("region", { name: "修復の結果" });
    expect(result.textContent).toContain("origin から clone し直しました");
    expect(result.textContent).toContain("送っていなかった 3 コミットを戻しました");
    expect(result.textContent).toContain("戻せなかったタスク: DM-7");
    expect(result.textContent).toContain("/old/store");
    expect(toasts.list[0]?.message).toBe("Store を origin から clone し直しました");
    expect(within(region()).getByText("問題は見つかりませんでした")).toBeTruthy();
  });

  it("says when there was nothing to repair", async () => {
    const { api } = setup({ reports: { a: report({ findings: [fileFinding] }) } });
    await within(region()).findByText("does not decode");
    api.POST.mockImplementation(() => ok([report({ findings: [fileFinding] })]));
    await fireEvent.click(within(region()).getByRole("button", { name: "修復…" }));
    await fireEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "修復する" }),
    );
    const result = await within(region()).findByRole("region", { name: "修復の結果" });
    expect(result.textContent).toContain("修復するものはありませんでした");
    expect(toasts.list[0]?.message).toBe("修復するものはありませんでした");
  });

  it("a failure is a toast, and the dialog stays for another try", async () => {
    const { api } = setup({ reports: { a: report({ findings: [fileFinding] }) } });
    await within(region()).findByText("does not decode");
    api.POST.mockImplementation((() => fail(500, "internal", {}, "boom")) as never);
    await fireEvent.click(within(region()).getByRole("button", { name: "修復…" }));
    await fireEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "修復する" }),
    );
    await waitFor(() => expect(toasts.list).toHaveLength(1));
    expect(toasts.list[0]?.message).toContain("boom");
    const d = screen.getByRole("dialog");
    expect(
      (within(d).getByRole("button", { name: "修復する" }) as HTMLButtonElement).disabled,
    ).toBe(false);
    expect(within(region()).queryByRole("region", { name: "修復の結果" })).toBeNull();
  });

  it("while it repairs, the buttons wait and Esc does not close the dialog", async () => {
    const { api } = setup({ reports: { a: report({ findings: [fileFinding] }) } });
    await within(region()).findByText("does not decode");
    let resolve: (v: unknown) => void = () => {};
    api.POST.mockImplementation((() => new Promise((r) => (resolve = r))) as never);
    await fireEvent.click(within(region()).getByRole("button", { name: "修復…" }));
    const d = screen.getByRole("dialog");
    await fireEvent.click(within(d).getByRole("button", { name: "修復する" }));
    expect(
      (within(d).getByRole("button", { name: "修復する" }) as HTMLButtonElement).disabled,
    ).toBe(true);
    expect(
      (within(d).getByRole("button", { name: "キャンセル" }) as HTMLButtonElement).disabled,
    ).toBe(true);
    expect(
      (within(region()).getByRole("button", { name: "もう一度実行" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
    await fireEvent.keyDown(window, { key: "Escape" });
    await fireEvent.click(within(d).getByRole("button", { name: "閉じる" }));
    expect(screen.queryByRole("dialog")).toBeTruthy();
    resolve({ data: [report()], response: new Response(null, { status: 200 }) });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("Esc closes the dialog when nothing is on its way, and the focus goes back", async () => {
    setup({ reports: { a: report({ findings: [fileFinding] }) } });
    await within(region()).findByText("does not decode");
    const open = within(region()).getByRole("button", { name: "修復…" });
    open.focus();
    await fireEvent.click(open);
    await fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(document.activeElement).toBe(open));
  });
});
