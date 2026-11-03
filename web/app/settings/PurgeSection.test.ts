import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { PurgePlan } from "../../core/doctor";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import PurgeSection from "./PurgeSection.svelte";

const plan = (over: Partial<PurgePlan> = {}): PurgePlan => ({
  store: "a",
  tasks: [
    { ulid: "U1", id: "DM-1", title: "old one" },
    { ulid: "U2", id: "DM-2", title: "old two" },
  ],
  forwards: [
    { ulid: "F1", moved_to: "T1", moved_at: "2026-01-01T00:00:00Z" },
    { ulid: "F2", moved_to: "T2", moved_at: "2026-01-02T00:00:00Z" },
    { ulid: "F3", moved_to: "T3", moved_at: "2026-01-03T00:00:00Z" },
  ],
  ...over,
});

const empty = plan({ tasks: [], forwards: [] });

function setup(stores: string[] = ["a"], plans: PurgePlan[] = [plan()]) {
  let n = 0;
  const api = {
    GET: vi.fn((_p: string, o: { params: { query: { store?: string } } }) =>
      ok({ ...(plans[Math.min(n++, plans.length - 1)] as PurgePlan), store: o.params.query.store }),
    ),
    POST: vi.fn((_p: string, o: { params: { query: { store?: string } } }) =>
      ok({ ...plan(), store: o.params.query.store, commit: "0123456789abcdef" }),
    ),
  };
  const view = renderWithCore(
    PurgeSection,
    { stores },
    { api, stores: { tone: () => "teal", subscribe: () => () => {} } },
  );
  return { api, view };
}

const days = () => screen.getByRole("textbox", { name: "N 日より古いもの" }) as HTMLInputElement;
const check = () => screen.getByRole("button", { name: "確かめる" }) as HTMLButtonElement;
const type = (v: string) => fireEvent.input(days(), { target: { value: v } });

beforeEach(() => {
  resetToasts();
  history.replaceState(null, "", "/settings/general");
});
afterEach(() => cleanup());

describe("PurgeSection: the input", () => {
  it("starts at 90 days, and says 0 is everything", () => {
    setup();
    expect(days().value).toBe("90");
    expect(check().disabled).toBe(false);
    expect(screen.getByText(/0 ですべて/)).toBeTruthy();
    // a text field, so that no spin buttons offer a negative number
    expect(screen.queryByRole("spinbutton")).toBeNull();
  });

  it.each(["", "  ", "-1", "1.5", "abc", "36501"])("%j cannot be checked", async (v) => {
    setup();
    await type(v);
    expect(check().disabled).toBe(true);
    expect(days().getAttribute("aria-invalid")).toBe("true");
    const hint = document.getElementById(days().getAttribute("aria-describedby") ?? "");
    expect(hint?.textContent).toContain("0 以上の整数");
  });

  it("0 can be checked", async () => {
    setup();
    await type("0");
    expect(check().disabled).toBe(false);
    expect(days().getAttribute("aria-invalid")).toBeNull();
  });
});

describe("PurgeSection: the plan", () => {
  it("lists the tasks and the count of forward files that would go", async () => {
    const { api } = setup();
    await type("0");
    await fireEvent.click(check());
    expect(api.GET).toHaveBeenCalledWith("/api/purge", {
      params: { query: { older_than: "0d", store: "a" } },
    });
    const list = await screen.findByRole("list", { name: "完全に削除されるタスク" });
    const rows = within(list).getAllByRole("listitem");
    expect(rows.map((r) => r.textContent)).toEqual([
      expect.stringContaining("DM-1"),
      expect.stringContaining("DM-2"),
    ]);
    expect(rows[0]?.textContent).toContain("old one");
    expect(screen.getByText("2 件のタスクと 3 件の転送ファイル")).toBeTruthy();
    expect(screen.getByRole("button", { name: "完全に削除…" })).toBeTruthy();
  });

  it("nothing to remove: it says so and offers no deletion", async () => {
    setup(["a"], [empty]);
    await fireEvent.click(check());
    expect(await screen.findByText("消すものはありません")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "完全に削除…" })).toBeNull();
  });

  it("forward files alone are something to remove", async () => {
    setup(["a"], [plan({ tasks: [] })]);
    await fireEvent.click(check());
    await screen.findByText("0 件のタスクと 3 件の転送ファイル");
    expect(screen.getByRole("button", { name: "完全に削除…" })).toBeTruthy();
    expect(screen.queryByRole("list", { name: "完全に削除されるタスク" })).toBeNull();
  });

  it("the focus comes back to the button that checked, or to the field when Enter did", async () => {
    setup();
    check().focus();
    await fireEvent.click(check());
    await screen.findByRole("button", { name: "完全に削除…" });
    await waitFor(() => expect(document.activeElement).toBe(check()));
    days().focus();
    await fireEvent.keyDown(days(), { key: "Enter" });
    await waitFor(() => expect(days().disabled).toBe(false));
    await waitFor(() => expect(document.activeElement).toBe(days()));
  });

  it("the plan is in a live region", async () => {
    setup(["a"], [empty]);
    await fireEvent.click(check());
    const t = await screen.findByText("消すものはありません");
    expect(t.closest("[aria-live]")?.getAttribute("aria-live")).toBe("polite");
  });

  it("only the summary line is a live region, not the list", async () => {
    setup();
    await fireEvent.click(check());
    const list = await screen.findByRole("list", { name: "完全に削除されるタスク" });
    expect(list.closest("[aria-live]")).toBeNull();
    expect(screen.getByText("2 件のタスクと 3 件の転送ファイル").getAttribute("aria-live")).toBe(
      "polite",
    );
  });

  it("a change of the days takes the plan away: only what was checked is deleted", async () => {
    setup();
    await fireEvent.click(check());
    await screen.findByRole("button", { name: "完全に削除…" });
    await type("30");
    expect(screen.queryByRole("button", { name: "完全に削除…" })).toBeNull();
    expect(screen.queryByRole("list", { name: "完全に削除されるタスク" })).toBeNull();
  });

  it("a failure to read the plan is said", async () => {
    const { api } = setup();
    api.GET.mockImplementation((() => fail(400, "purge.bad_period", { value: "90d" })) as never);
    await fireEvent.click(check());
    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toContain("期間ではありません"),
    );
  });

  it("the check waits while it asks, and the inputs with it", async () => {
    const late: ((v: unknown) => void)[] = [];
    const api = {
      GET: vi.fn(() => new Promise((r) => late.push(r))),
      POST: vi.fn(),
    };
    renderWithCore(
      PurgeSection,
      { stores: ["a"] },
      { api, stores: { tone: () => "teal", subscribe: () => () => {} } },
    );
    await fireEvent.click(check());
    expect(check().disabled).toBe(true);
    expect(days().disabled).toBe(true);
    late[0]?.({ data: plan(), response: new Response(null, { status: 200 }) });
    await screen.findByRole("button", { name: "完全に削除…" });
    expect(check().disabled).toBe(false);
  });

  it("an answer that comes after it is gone changes nothing", async () => {
    let resolve: (v: unknown) => void = () => {};
    const api = { GET: vi.fn(() => new Promise((r) => (resolve = r))), POST: vi.fn() };
    const view = renderWithCore(
      PurgeSection,
      { stores: ["a"] },
      { api, stores: { tone: () => "teal", subscribe: () => () => {} } },
    );
    await fireEvent.click(check());
    view.unmount();
    resolve({ data: plan(), response: new Response(null, { status: 200 }) });
    await Promise.resolve();
    expect(api.POST).not.toHaveBeenCalled();
  });
});

describe("PurgeSection: the stores", () => {
  it("one store has no choice of it", () => {
    setup(["a"]);
    expect(screen.queryByRole("combobox", { name: "Store" })).toBeNull();
  });

  it("two stores or more: the user picks one, and the plan is that store's", async () => {
    const { api } = setup(["a", "b"]);
    const pick = screen.getByRole("combobox", { name: "Store" }) as HTMLSelectElement;
    expect([...pick.options].map((o) => o.value)).toEqual(["a", "b"]);
    await fireEvent.click(check());
    await screen.findByRole("button", { name: "完全に削除…" });
    await fireEvent.change(pick, { target: { value: "b" } });
    // a plan of another store is not this one's
    expect(screen.queryByRole("button", { name: "完全に削除…" })).toBeNull();
    await fireEvent.click(check());
    await screen.findByRole("button", { name: "完全に削除…" });
    expect(api.GET).toHaveBeenLastCalledWith("/api/purge", {
      params: { query: { older_than: "90d", store: "b" } },
    });
  });
});

describe("PurgeSection: the deletion", () => {
  async function toDialog(stores: string[] = ["a"]) {
    const r = setup(stores);
    await type("0");
    await fireEvent.click(check());
    await fireEvent.click(await screen.findByRole("button", { name: "完全に削除…" }));
    return { ...r, dialog: screen.getByRole("dialog", { name: "完全に削除する" }) };
  }

  it("asks first: it cannot be undone, how many, and the command", async () => {
    const { dialog } = await toDialog();
    expect(dialog.textContent).toContain("取り消せません");
    expect(dialog.textContent).toContain("2 件の削除済みタスク");
    expect(dialog.textContent).toContain("3 件の転送ファイル");
    expect(
      within(dialog).getByRole("button", { name: "コマンドをコピー: hh purge --older-than 0d" }),
    ).toBeTruthy();
  });

  it("the command names the store for two stores or more", async () => {
    const { dialog } = await toDialog(["a", "b"]);
    expect(
      within(dialog).getByRole("button", {
        name: "コマンドをコピー: hh purge --older-than 0d --store a",
      }),
    ).toBeTruthy();
  });

  it("deletes what was checked, tells the commit, and puts the focus back on the check", async () => {
    const { api, dialog } = await toDialog();
    await fireEvent.click(within(dialog).getByRole("button", { name: "完全に削除する" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(api.POST).toHaveBeenCalledWith("/api/purge", {
      params: { query: { store: "a" } },
      body: { older_than: "0d", confirmed: expect.objectContaining({ store: "a" }) },
    });
    expect(toasts.list.map((t) => t.message)).toEqual([
      "2 件のタスクを完全に削除しました（0123456）",
    ]);
    expect(screen.queryByRole("button", { name: "完全に削除…" })).toBeNull();
    await waitFor(() => expect(document.activeElement).toBe(check()));
  });

  it("a change after the check (409 purge.changed) says so, and checks again", async () => {
    const { api, dialog } = await toDialog();
    api.POST.mockImplementation((() => fail(409, "purge.changed")) as never);
    api.GET.mockImplementation((() =>
      ok(plan({ tasks: [{ ulid: "U9", id: "DM-9", title: "new one" }] }))) as never);
    await fireEvent.click(within(dialog).getByRole("button", { name: "完全に削除する" }));
    await waitFor(() => expect(toasts.list).toHaveLength(1));
    expect(toasts.list[0]?.message).toContain("確認のあとにタスクが変わりました");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    const list = await screen.findByRole("list", { name: "完全に削除されるタスク" });
    expect(list.textContent).toContain("DM-9");
    expect(list.textContent).not.toContain("DM-1");
    // the dialog is closed and the check is over: the focus is on the button that began the deletion
    await waitFor(() =>
      expect(document.activeElement).toBe(screen.getByRole("button", { name: "完全に削除…" })),
    );
  });

  it("another failure is a toast, and the dialog stays", async () => {
    const { api, dialog } = await toDialog();
    api.POST.mockImplementation((() => fail(500, "internal")) as never);
    await fireEvent.click(within(dialog).getByRole("button", { name: "完全に削除する" }));
    await waitFor(() => expect(toasts.list).toHaveLength(1));
    expect(screen.getByRole("dialog")).toBeTruthy();
    expect(
      (
        within(screen.getByRole("dialog")).getByRole("button", {
          name: "完全に削除する",
        }) as HTMLButtonElement
      ).disabled,
    ).toBe(false);
  });

  it("while it deletes, nothing can be changed and Esc does not close the dialog", async () => {
    const { api, dialog } = await toDialog();
    let resolve: (v: unknown) => void = () => {};
    api.POST.mockImplementation((() => new Promise((r) => (resolve = r))) as never);
    await fireEvent.click(within(dialog).getByRole("button", { name: "完全に削除する" }));
    expect(
      (within(dialog).getByRole("button", { name: "完全に削除する" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
    expect(
      (within(dialog).getByRole("button", { name: "キャンセル" }) as HTMLButtonElement).disabled,
    ).toBe(true);
    expect(days().disabled).toBe(true);
    expect(check().disabled).toBe(true);
    await fireEvent.keyDown(window, { key: "Escape" });
    await fireEvent.click(within(dialog).getByRole("button", { name: "閉じる" }));
    expect(screen.queryByRole("dialog")).toBeTruthy();
    resolve({
      data: { ...plan(), commit: "abcdef0123" },
      response: new Response(null, { status: 200 }),
    });
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });

  it("cancel closes the dialog and keeps the plan", async () => {
    const { dialog } = await toDialog();
    await fireEvent.click(within(dialog).getByRole("button", { name: "キャンセル" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("button", { name: "完全に削除…" })).toBeTruthy();
  });

  it("does not build what the screen leaves out", () => {
    const { view } = setup();
    for (const t of ["作り直", "repack", "登録する", "--moved"]) {
      expect(view.container.textContent).not.toContain(t);
    }
  });
});
