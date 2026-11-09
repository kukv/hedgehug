import { cleanup, fireEvent, screen } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fail, ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import { resetToasts, toasts } from "../toast.svelte";
import MigrateDialog from "./MigrateDialog.svelte";

type Body = { dry_run?: boolean; new: string; old: string; target: string };
type Out = { changed: unknown[]; skipped: string[]; commit: string | null };

const out = (changed: number, skipped = 0, commit: string | null = null): Out => ({
  changed: Array.from({ length: changed }, (_, i) => ({ ulid: `U${i}` })),
  skipped: Array.from({ length: skipped }, (_, i) => `S${i}`),
  commit,
});

const deferred = <T>() => {
  let resolve!: (v: T) => void;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
};

type Opts = {
  target?: "status";
  old?: string;
  names?: string[];
  many?: boolean;
  post?: (body: Body) => unknown;
};

function setup(opts: Opts = {}) {
  const post = vi.fn((body: Body, _query: { store: string }) =>
    (opts.post ?? ((b: Body) => ok(out(b.dry_run ? 3 : 3, 0, b.dry_run ? null : "c1"))))(body),
  );
  const reload = vi.fn(async () => {});
  const onclose = vi.fn();
  const ondone = vi.fn();
  const view = renderWithCore(
    MigrateDialog,
    {
      store: "shared",
      many: opts.many ?? false,
      target: opts.target ?? "status",
      old: opts.old ?? "triage",
      names: opts.names ?? ["todo", "triage", "done", "review"],
      onclose,
      ondone,
    },
    { api: { migrateSchema: post }, schema: { reload, subscribe: () => () => {} } },
  );
  return { post, reload, onclose, ondone, ...view };
}

const select = () => screen.getByRole("combobox", { name: "マージ先" }) as HTMLSelectElement;
const pick = (v: string) => fireEvent.change(select(), { target: { value: v } });
// past the pause after which the dry run is sent
const settle = () => vi.advanceTimersByTimeAsync(500);

describe("MigrateDialog", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    resetToasts();
  });
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("names the definition in the title, and sends nothing while the name is empty or the same", async () => {
    const { post } = setup();
    expect(screen.getByRole("dialog", { name: "ステータス「triage」をマージ" })).toBeVisible();
    await settle();
    await pick("todo");
    await pick("");
    await settle();
    expect(post).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "マージ" })).toBeDisabled();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh schema migrate status triage <マージ先>",
    );
  });

  it("sends the dry run once the typing stops, and says how many tasks it would rewrite", async () => {
    const { post } = setup({ post: () => ok(out(3, 2)) });
    await pick("todo");
    await vi.advanceTimersByTimeAsync(100);
    await pick("review");
    await vi.advanceTimersByTimeAsync(100);
    expect(post).not.toHaveBeenCalled();
    await settle();
    expect(post).toHaveBeenCalledTimes(1);
    expect(post).toHaveBeenCalledWith(
      { target: "status", old: "triage", new: "review", dry_run: true },
      { store: "shared" },
    );
    expect(screen.getByText("3 件のタスクが書き換わります")).toBeVisible();
    expect(screen.getByText("読み込めないため書き換えないタスク 2 件")).toBeVisible();
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh schema migrate status triage review",
    );
  });

  it("says so when no task changes", async () => {
    setup({ post: () => ok(out(0)) });
    await pick("review");
    await settle();
    expect(screen.getByText("書き換わるタスクはありません")).toBeVisible();
    expect(screen.queryByText(/読み込めない/)).toBeNull();
  });

  it("does not let the run go ahead of the dry run of the name in the field", async () => {
    const first = deferred<unknown>();
    setup({ post: () => first.promise });
    await pick("review");
    await settle();
    // the dry run has not come back
    expect(screen.getByRole("button", { name: "マージ" })).toBeDisabled();
    first.resolve(await ok(out(1)));
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.getByRole("button", { name: "マージ" })).toBeEnabled();
    // a new name takes the permission back until its dry run is in
    await pick("done");
    expect(screen.getByRole("button", { name: "マージ" })).toBeDisabled();
    expect(screen.queryByText("1 件のタスクが書き換わります")).toBeNull();
  });

  it("drops the answer of an older dry run that comes after a newer one", async () => {
    const slow = deferred<unknown>();
    const bodies: Body[] = [];
    setup({
      post: (b) => {
        bodies.push(b);
        return bodies.length === 1 ? slow.promise : ok(out(7));
      },
    });
    await pick("todo");
    await settle();
    await pick("done");
    await settle();
    expect(screen.getByText("7 件のタスクが書き換わります")).toBeVisible();
    slow.resolve(await ok(out(1)));
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.getByText("7 件のタスクが書き換わります")).toBeVisible();
    expect(screen.queryByText("1 件のタスクが書き換わります")).toBeNull();
  });

  it("shows the refusal of a dry run and keeps the run off", async () => {
    setup({ post: () => fail(422, "schema.invalid", {}, "名前が不正です") });
    await pick("todo");
    await settle();
    expect(screen.getByRole("alert")).toBeVisible();
    expect(screen.getByRole("button", { name: "マージ" })).toBeDisabled();
  });

  it("merges into one of the other definitions, chosen from a list, and says what goes", async () => {
    const { post } = setup();
    expect(screen.getByRole("dialog", { name: "ステータス「triage」をマージ" })).toBeVisible();
    expect([...select().options].map((o) => o.value)).toEqual(["", "todo", "done", "review"]);
    expect(screen.queryByRole("textbox")).toBeNull();
    await pick("todo");
    await settle();
    expect(post.mock.calls[0]?.[0]).toEqual({
      target: "status",
      old: "triage",
      new: "todo",
      dry_run: true,
    });
    expect(screen.getByText(/「triage」は定義から消え、「todo」にマージされます/)).toBeVisible();
    expect(screen.getByRole("button", { name: "マージ" })).toBeEnabled();
  });

  it("sends --store only when there are many stores", async () => {
    const { post } = setup({ many: true });
    await pick("review");
    await settle();
    expect(post.mock.calls[0]?.[0]).toEqual({
      target: "status",
      old: "triage",
      new: "review",
      dry_run: true,
    });
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toHaveTextContent(
      "hh schema migrate status triage review --store shared",
    );
  });

  it("runs it, reads the schema again, says how many it rewrote and closes", async () => {
    const { post, reload, ondone, onclose } = setup({
      post: (b) => ok(b.dry_run ? out(3, 1) : out(3, 1, "c1")),
    });
    await pick("review");
    await settle();
    await fireEvent.click(screen.getByRole("button", { name: "マージ" }));
    await vi.advanceTimersByTimeAsync(0);
    expect(post).toHaveBeenLastCalledWith(
      { target: "status", old: "triage", new: "review" },
      { store: "shared" },
    );
    expect(reload).toHaveBeenCalledWith("shared");
    expect(toasts.list[0]?.message).toBe(
      "3 件を書き換えました（読み込めないため書き換えなかったタスク 1 件）",
    );
    expect(ondone).toHaveBeenCalledTimes(1);
    expect(onclose).toHaveBeenCalledTimes(1);
  });

  it("keeps the dialog and shows the refusal when the run fails", async () => {
    const { onclose, ondone } = setup({
      post: (b) => (b.dry_run ? ok(out(1)) : fail(409, "schema.conflict", {}, "競合しました")),
    });
    await pick("review");
    await settle();
    const run = screen.getByRole("button", { name: "マージ" });
    run.focus(); // a click focuses the button in a browser; it is disabled while the run is on
    await fireEvent.click(run);
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.getByRole("alert")).toBeVisible();
    // the focus a disabled button dropped goes to the field, to try another target
    expect(select()).toHaveFocus();
    expect(onclose).not.toHaveBeenCalled();
    expect(ondone).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "マージ" })).toBeEnabled();
  });

  it("holds the dialog while the run is on its way: inputs and buttons off, Esc and Cancel do nothing", async () => {
    const run = deferred<unknown>();
    const { onclose } = setup({ post: (b) => (b.dry_run ? ok(out(1)) : run.promise) });
    await pick("review");
    await settle();
    await fireEvent.click(screen.getByRole("button", { name: "マージ" }));
    expect(select()).toBeDisabled();
    expect(screen.getByRole("button", { name: "キャンセル" })).toBeDisabled();
    await fireEvent.keyDown(document, { key: "Escape" });
    expect(onclose).not.toHaveBeenCalled();
    run.resolve(await ok(out(1, 0, "c1")));
    await vi.advanceTimersByTimeAsync(0);
    expect(onclose).toHaveBeenCalledTimes(1);
  });

  it("closes with Cancel and with Esc when nothing is on its way", async () => {
    const { onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "キャンセル" }));
    await fireEvent.keyDown(document, { key: "Escape" });
    expect(onclose).toHaveBeenCalledTimes(2);
  });

  it("sends nothing and shows nothing from an answer that comes after it is gone", async () => {
    const late = deferred<unknown>();
    const { post, unmount } = setup({ post: () => late.promise });
    await pick("review");
    await settle();
    expect(post).toHaveBeenCalledTimes(1);
    unmount();
    late.resolve(await ok(out(1)));
    await vi.advanceTimersByTimeAsync(0);
    expect(screen.queryByText(/件のタスク/)).toBeNull();
    // and a pause that was still running when it went away sends nothing
    cleanup();
    const again = setup();
    await pick("todo");
    again.unmount();
    await settle();
    expect(again.post).not.toHaveBeenCalled();
  });
});
