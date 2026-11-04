import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fail, makeCore, mount, ok, taskView } from "../detail/testkit";
import { resetToasts, toasts } from "../toast.svelte";
import { entry } from "../workspace/testkit";
import StoreMove from "./StoreMove.svelte";

const task = taskView({
  ulid: "U1",
  id: "REL-1",
  title: "親のタスク",
  store: "shared",
  parent: "UP",
});
const parentTask = taskView({ ulid: "UP", id: "REL-0", title: "元の親", store: "shared" });

const spaces = [
  entry({ store: "shared", ulid: "W1", name: "release", prefix: "REL" }),
  entry({ store: "second", ulid: "WA", name: "a", prefix: "SC" }),
  entry({ store: "multi", ulid: "WM1", name: "api", prefix: "API" }),
  entry({ store: "multi", ulid: "WM2", name: "web", prefix: "WEB" }),
  entry({ store: "ro", ulid: "WR", name: "r", prefix: "RO" }),
  entry({ store: "hid", ulid: "WH", name: "h", prefix: "HID" }),
];

const stores = [
  { name: "shared", default: true },
  { name: "second", remotes: ["git@example.com:org/second.git"] },
  { name: "multi" },
  { name: "ro" },
  { name: "hid" },
];

function planOf(over: Record<string, unknown> = {}) {
  return {
    from: "shared",
    to: "second",
    workspace: { name: "a", prefix: "SC", ulid: "WA" },
    tasks: [
      { id: "REL-1", title: "親のタスク", ulid: "U1" },
      { id: "REL-2", title: "子", ulid: "U2" },
    ],
    unlinked: [],
    hidden: [],
    ...over,
  };
}

const result = {
  from: "shared",
  to: "second",
  tasks: [
    { new_id: "SC-1", old_id: "REL-1", ulid: "U1" },
    { new_id: "SC-2", old_id: "REL-2", ulid: "U2" },
  ],
};

/** The routes the dialog calls; each one is replaced by a test that needs another answer. */
function setup(opts: { plan?: unknown; move?: unknown; qualify?: boolean } = {}) {
  const k = makeCore({
    workspaces: spaces,
    storeItems: stores,
    tasks: [task, parentTask],
    readOnlyBy: { ro: "the remote cannot be written" },
    hidden: ["hid"],
    qualify: opts.qualify,
  });
  const routes = {
    plan: vi.fn<(init: unknown) => Promise<unknown>>(() => ok(opts.plan ?? planOf())),
    move: vi.fn<(init: unknown) => Promise<unknown>>(() => ok(opts.move ?? result)),
  };
  k.api.POST.mockImplementation((path: string, init: unknown) =>
    path.endsWith("/plan") ? routes.plan(init) : routes.move(init),
  );
  k.api.GET.mockImplementation(() => ok({ tasks: [] }));
  const onclose = vi.fn();
  return { k, onclose, routes, ...mount(StoreMove, { task, onclose }, k) };
}

const radio = (name: RegExp | string) => screen.getByRole("radio", { name });
const pick = (store: string) => fireEvent.click(radio(new RegExp(`^${store}`)));
const cli = () => screen.getByRole("button", { name: /コマンドをコピー/ });

describe("StoreMove", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("starts with no Store picked, asks for no plan, and cannot move", () => {
    const { routes } = setup();
    expect(screen.getByRole("dialog", { name: "別の Store へ移動" })).toBeVisible();
    expect(screen.getByRole("dialog")).toHaveTextContent("REL-1 親のタスク");
    for (const r of screen.getAllByRole("radio")) expect(r).not.toBeChecked();
    expect(routes.plan).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "移動" })).toBeDisabled();
    expect(screen.queryByText("移動先で変わること")).toBeNull();
    expect(cli()).toHaveTextContent("hh mv REL-1 --store <Store> --yes");
  });

  it("lists the other Stores with their remote, a Store that is read-only as off, and a hidden one too", () => {
    setup();
    expect(screen.queryByRole("radio", { name: /^shared/ })).toBeNull(); // the source
    expect(radio(/^second/)).toBeEnabled();
    expect(radio(/^second/)).toHaveAccessibleName(/git@example\.com:org\/second\.git/);
    const ro = radio(/^ro/);
    expect(ro).toBeDisabled();
    expect(ro).toHaveAccessibleDescription(
      "閲覧のみのため移動できません（remote に書き込めません）",
    );
    expect(radio(/^hid/)).toBeEnabled(); // hidden is a setting of the screen, not of where it can go
  });

  it("asks the plan for the Store picked, and shows what changes", async () => {
    const { routes } = setup();
    await pick("second");
    await waitFor(() => expect(routes.plan).toHaveBeenCalledTimes(1));
    expect(routes.plan).toHaveBeenCalledWith({
      params: { path: { id: "U1" } },
      body: { to: "second" },
    });
    expect(await screen.findByText("移動先で変わること")).toBeVisible();
    expect(screen.getByText("移動先の a で振り直します")).toBeVisible();
    expect(screen.getByText("REL-0 元の親 から外れて、a のルートに置きます")).toBeVisible();
    expect(screen.getByText("転送ファイルを残し、古い ID からも辿れます")).toBeVisible();
    // the descendants: the tasks of the plan but the first
    expect(screen.getByText(/子孫 1/)).toBeVisible();
    expect(screen.queryByRole("region", { name: /^外す参照/ })).toBeNull();
    expect(screen.queryByRole("note")).toBeNull();
    expect(screen.getByRole("button", { name: "移動" })).toBeEnabled();
    expect(cli()).toHaveTextContent("hh mv REL-1 --store second --yes");
  });

  it("does not say the number the new ID will have, nor what is not in the API", async () => {
    setup();
    await pick("second");
    await screen.findByText("移動先で変わること");
    const text = document.body.textContent ?? "";
    expect(text).not.toMatch(/SC-\d/);
    expect(text).not.toMatch(/読める人 \d/);
    expect(text).not.toContain("種類・ステータス");
    expect(text).not.toContain("計画");
  });

  it("does not offer to rewrite the history", async () => {
    setup({ plan: planOf({ hidden: ["alice@example.com", "bob"] }) });
    await pick("second");
    const note = await screen.findByRole("note");
    expect(note).toHaveTextContent("見えなくなる人: alice@example.com、bob");
    expect(note).toHaveTextContent(
      "移動前の内容は shared の git 履歴に残り、shared を読める人は過去の内容を見られます",
    );
    const text = document.body.textContent ?? "";
    expect(text).not.toContain("purge");
    expect(text).not.toContain("rewrite-history");
    expect(text).not.toContain("消すには");
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("lists the references it cuts, and counts them in the button", async () => {
    setup({
      plan: planOf({
        unlinked: [
          {
            task: { id: "REL-3", ulid: "U3" },
            field: "blocked_by",
            target: { label: "REL-1", ulid: "U1" },
          },
          {
            task: { id: "REL-2", ulid: "U2" },
            field: "props.relates",
            target: { label: "REL-9", ulid: "U9" },
          },
          {
            task: { id: "REL-1", ulid: "U1" },
            field: "sprint",
            target: { label: "S1", ulid: "S" },
          },
        ],
      }),
    });
    await pick("second");
    const region = await screen.findByRole("region", { name: /^外す参照/ });
    const items = within(region).getAllByRole("listitem");
    expect(items).toHaveLength(3);
    expect(items[0]).toHaveTextContent("ブロックされている REL-1 REL-3");
    expect(items[1]).toHaveTextContent("relates REL-9 REL-2");
    expect(items[1]).not.toHaveTextContent("props.");
    expect(items[2]).toHaveTextContent("スプリント S1 REL-1");
    expect(screen.getByRole("button", { name: "3 件の参照を外して移動" })).toBeEnabled();
  });

  describe("the workspace", () => {
    it("is only named when the Store has one", async () => {
      const { routes } = setup();
      await pick("second");
      await screen.findByText("移動先で変わること");
      expect(screen.getByText("a（SC）")).toBeVisible();
      expect(screen.queryByRole("combobox", { name: "second のワークスペース" })).toBeNull();
      expect(routes.plan.mock.calls[0]?.[0]).toMatchObject({ body: { to: "second" } });
    });

    it("is picked from a list when the Store has more, and no plan is asked for before", async () => {
      const { routes } = setup({
        plan: planOf({ to: "multi", workspace: { name: "web", prefix: "WEB", ulid: "WM2" } }),
      });
      await pick("multi");
      const select = screen.getByRole("combobox", { name: "multi のワークスペース" });
      expect(select).toHaveValue("");
      expect(routes.plan).not.toHaveBeenCalled();
      expect(screen.getByRole("button", { name: "移動" })).toBeDisabled();
      expect(cli()).toHaveTextContent(
        "hh mv REL-1 --store multi --workspace <ワークスペース> --yes",
      );

      await fireEvent.change(select, { target: { value: "web" } });
      await waitFor(() => expect(routes.plan).toHaveBeenCalledTimes(1));
      expect(routes.plan).toHaveBeenCalledWith({
        params: { path: { id: "U1" } },
        body: { to: "multi", workspace: "web" },
      });
      expect(cli()).toHaveTextContent("hh mv REL-1 --store multi --workspace web --yes");
    });
  });

  describe("the parent", () => {
    it("is searched in the destination Store, and picked is named with a button to take it off", async () => {
      const { k, routes } = setup();
      await pick("second");
      await screen.findByText("移動先で変わること");
      const picker = screen.getByRole("combobox", { name: "親を選ぶ" });
      k.api.GET.mockImplementation(() =>
        ok({ tasks: [taskView({ ulid: "UQ", id: "SC-4", title: "新しい親", store: "second" })] }),
      );
      await fireEvent.input(picker, { target: { value: "新しい" } });
      await fireEvent.click(await screen.findByRole("option", { name: /SC-4/ }, { timeout: 2000 }));
      expect(k.api.GET).toHaveBeenCalledWith(
        "/api/tasks",
        expect.objectContaining({
          params: { query: expect.objectContaining({ store: "second" }) },
        }),
      );
      await waitFor(() => expect(routes.plan).toHaveBeenCalledTimes(2));
      expect(routes.plan).toHaveBeenLastCalledWith({
        params: { path: { id: "U1" } },
        body: { to: "second", parent: "UQ" },
      });
      expect(cli()).toHaveTextContent("hh mv REL-1 --store second --parent SC-4 --yes");
      expect(screen.getByRole("dialog")).toHaveTextContent("SC-4 新しい親");

      await fireEvent.click(screen.getByRole("button", { name: "外す" }));
      await waitFor(() => expect(routes.plan).toHaveBeenCalledTimes(3));
      expect(routes.plan).toHaveBeenLastCalledWith({
        params: { path: { id: "U1" } },
        body: { to: "second" },
      });
    });

    it("sets the workspace to its own, and another workspace or another Store drops it", async () => {
      const { k, routes } = setup({
        plan: planOf({ to: "multi", workspace: { name: "web", prefix: "WEB", ulid: "WM2" } }),
      });
      await pick("multi");
      const select = screen.getByRole("combobox", { name: "multi のワークスペース" });
      k.api.GET.mockImplementation(() =>
        ok({
          tasks: [
            taskView({
              ulid: "UQ",
              id: "WEB-4",
              title: "親",
              store: "multi",
              workspace: { ulid: "WM2", name: "web", id_prefix: "WEB" },
            }),
          ],
        }),
      );
      // no workspace is picked yet: the parent is enough for a plan
      await fireEvent.input(screen.getByRole("combobox", { name: "親を選ぶ" }), {
        target: { value: "親" },
      });
      await fireEvent.click(
        await screen.findByRole("option", { name: /WEB-4/ }, { timeout: 2000 }),
      );
      await waitFor(() => expect(routes.plan).toHaveBeenCalledTimes(1));
      expect(routes.plan).toHaveBeenCalledWith({
        params: { path: { id: "U1" } },
        body: { to: "multi", workspace: "web", parent: "UQ" },
      });
      expect(select).toHaveValue("web");

      // the same workspace keeps the parent; another one drops it
      await fireEvent.change(select, { target: { value: "web" } });
      expect(screen.getByRole("dialog")).toHaveTextContent("WEB-4 親");
      await fireEvent.change(select, { target: { value: "api" } });
      expect(screen.queryByRole("button", { name: "外す" })).toBeNull();
      expect(routes.plan).toHaveBeenLastCalledWith({
        params: { path: { id: "U1" } },
        body: { to: "multi", workspace: "api" },
      });
    });

    it("is dropped, with the workspace, when another Store is picked", async () => {
      const { k, routes } = setup();
      await pick("second");
      await screen.findByText("移動先で変わること");
      k.api.GET.mockImplementation(() =>
        ok({ tasks: [taskView({ ulid: "UQ", id: "SC-4", title: "親", store: "second" })] }),
      );
      await fireEvent.input(screen.getByRole("combobox", { name: "親を選ぶ" }), {
        target: { value: "親" },
      });
      await fireEvent.click(await screen.findByRole("option", { name: /SC-4/ }, { timeout: 2000 }));
      await pick("hid");
      await waitFor(() =>
        expect(routes.plan).toHaveBeenLastCalledWith({
          params: { path: { id: "U1" } },
          body: { to: "hid" },
        }),
      );
      expect(screen.queryByRole("button", { name: "外す" })).toBeNull();
    });

    it("is said in the plan, from the one it leaves to the one it goes under", async () => {
      setup({ plan: planOf({ parent: { id: "SC-4", title: "新しい親", ulid: "UQ" } }) });
      await pick("second");
      expect(
        await screen.findByText("REL-0 元の親 から外れて、SC-4 新しい親 の下に置きます"),
      ).toBeVisible();
    });
  });

  it("leaves out the first half of the parent when the task has none", async () => {
    const k = makeCore({ workspaces: spaces, storeItems: stores, tasks: [] });
    k.api.POST.mockImplementation(() => ok(planOf()));
    mount(StoreMove, { task: { ...task, parent: null }, onclose: vi.fn() }, k);
    await pick("second");
    expect(await screen.findByText("a のルートに置きます")).toBeVisible();
  });

  it("takes the plan of the Store picked last when the answers come in the other order", async () => {
    const { routes } = setup();
    let first: (v: unknown) => void = () => {};
    routes.plan.mockImplementationOnce(() => new Promise((res) => (first = res)));
    await pick("second");
    routes.plan.mockImplementationOnce(() =>
      ok(planOf({ to: "hid", workspace: { name: "h", prefix: "HID", ulid: "WH" } })),
    );
    await pick("hid");
    expect(await screen.findByText("移動先の h で振り直します")).toBeVisible();
    first({ data: planOf(), response: new Response(null, { status: 200 }) });
    await new Promise((r) => setTimeout(r, 10));
    expect(screen.getByText("移動先の h で振り直します")).toBeVisible();
    expect(screen.queryByText("移動先の a で振り直します")).toBeNull();
  });

  describe("scoped", () => {
    const scoped = (over: Record<string, unknown> = {}) => ({
      required: true,
      dropped: ["status review"],
      ...over,
    });

    it("must be chosen before the move, and the choice is sent with the plan", async () => {
      const { routes } = setup({ plan: planOf({ scoped: scoped() }) });
      await pick("second");
      const group = await screen.findByRole("radiogroup", { name: "プロジェクトの表示の設定" });
      expect(screen.getByRole("button", { name: "移動" })).toBeDisabled();
      expect(within(group).getByRole("radio", { name: "持っていく" })).not.toBeChecked();
      expect(within(group).getByRole("radio", { name: "持っていかない" })).not.toBeChecked();

      routes.plan.mockImplementation(() =>
        ok(planOf({ scoped: scoped({ required: false, choice: "carry" }) })),
      );
      await fireEvent.click(within(group).getByRole("radio", { name: "持っていく" }));
      await waitFor(() => expect(routes.plan).toHaveBeenCalledTimes(2));
      expect(routes.plan).toHaveBeenLastCalledWith({
        params: { path: { id: "U1" } },
        body: { to: "second", scoped: "carry" },
      });
      expect(await screen.findByText("status review")).toBeVisible();
      expect(screen.getByRole("button", { name: "移動" })).toBeEnabled();
      expect(cli()).toHaveTextContent("hh mv REL-1 --store second --scoped carry --yes");
    });
  });

  describe("the move", () => {
    it("sends the plan that was shown, opens the task and says it in a toast", async () => {
      const { k, onclose, routes } = setup();
      await pick("second");
      await screen.findByText("移動先で変わること");
      await fireEvent.click(screen.getByRole("button", { name: "移動" }));
      await waitFor(() => expect(onclose).toHaveBeenCalled());
      expect(routes.move).toHaveBeenCalledWith({
        params: { path: { id: "U1" } },
        body: { to: "second", confirmed: planOf() },
      });
      expect(k.cache.refreshTrees).toHaveBeenCalled();
      expect(k.cache.applyWrite).not.toHaveBeenCalled();
      expect(k.cache.noteOwnCommit).not.toHaveBeenCalled();
      expect(location.pathname).toBe("/tasks/U1");
      expect(toasts.list[0]).toMatchObject({
        message: "REL-1 を second に移動しました（SC-1）",
        link: { label: "開く", href: "/tasks/U1" },
      });
    });

    it("sends the workspace, the parent and the choice picked", async () => {
      const { routes } = setup({
        plan: planOf({
          to: "multi",
          workspace: { name: "web", prefix: "WEB", ulid: "WM2" },
          scoped: { required: false, choice: "drop", dropped: [] },
        }),
      });
      await pick("multi");
      await fireEvent.change(screen.getByRole("combobox", { name: "multi のワークスペース" }), {
        target: { value: "web" },
      });
      const group = await screen.findByRole("radiogroup", { name: "プロジェクトの表示の設定" });
      await fireEvent.click(within(group).getByRole("radio", { name: "持っていかない" }));
      await waitFor(() => expect(routes.plan).toHaveBeenCalledTimes(2));
      await fireEvent.click(screen.getByRole("button", { name: "移動" }));
      await waitFor(() => expect(routes.move).toHaveBeenCalled());
      expect(routes.move.mock.calls[0]?.[0]).toMatchObject({
        body: { to: "multi", workspace: "web", scoped: "drop" },
      });
    });

    it("writes the IDs with the Store where the prefix is shared", async () => {
      const { onclose } = setup({ qualify: true });
      await pick("second");
      await screen.findByText("移動先で変わること");
      await fireEvent.click(screen.getByRole("button", { name: "移動" }));
      await waitFor(() => expect(onclose).toHaveBeenCalled());
      expect(toasts.list[0]?.message).toBe("shared:REL-1 を second に移動しました（second:SC-1）");
    });

    it("takes the plan again when the tasks changed, and stays", async () => {
      const { routes, onclose } = setup();
      await pick("second");
      await screen.findByText("移動先で変わること");
      routes.move.mockImplementationOnce(() => fail(409, "move.changed"));
      routes.plan.mockImplementation(() =>
        ok(planOf({ tasks: [...planOf().tasks, { id: "REL-5", title: "新", ulid: "U5" }] })),
      );
      await fireEvent.click(screen.getByRole("button", { name: "移動" }));
      expect(await screen.findByRole("alert")).toHaveTextContent(
        "内容が変わりました。確かめてから移動してください",
      );
      await waitFor(() => expect(routes.plan).toHaveBeenCalledTimes(2));
      expect(await screen.findByText(/子孫 2/)).toBeVisible();
      expect(onclose).not.toHaveBeenCalled();
      expect(screen.getByRole("button", { name: "移動" })).toBeEnabled();
    });

    it("lists the violations of the destination for each task", async () => {
      const { routes, onclose } = setup();
      await pick("second");
      await screen.findByText("移動先で変わること");
      routes.move.mockImplementationOnce(() =>
        fail(422, "move.invalid", {
          store: "second",
          violations: [
            "  REL-1 親のタスク: kind epic is not defined",
            "  REL-2 子: status review is not defined",
          ],
        }),
      );
      await fireEvent.click(screen.getByRole("button", { name: "移動" }));
      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent("ここには移せません");
      const items = within(alert).getAllByRole("listitem");
      expect(items.map((i) => i.textContent)).toEqual([
        "REL-1 親のタスク: kind epic is not defined",
        "REL-2 子: status review is not defined",
      ]);
      expect(onclose).not.toHaveBeenCalled();
    });

    it("lists the violations of schema.violation too", async () => {
      const { routes } = setup();
      await pick("second");
      await screen.findByText("移動先で変わること");
      routes.move.mockImplementationOnce(() =>
        fail(422, "schema.violation", {
          violations: [{ code: "unknown_kind", field: "kind", message: "kind epic" }],
        }),
      );
      await fireEvent.click(screen.getByRole("button", { name: "移動" }));
      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent("スキーマに合わない変更です");
      expect(within(alert).getByRole("listitem")).toHaveTextContent(
        "未定義の種類です（kind）: kind epic",
      );
    });

    it("shows any other refusal in the words of errorMessage", async () => {
      const { routes, onclose } = setup();
      await pick("second");
      await screen.findByText("移動先で変わること");
      routes.move.mockImplementationOnce(() =>
        fail(409, "store.read_only", { store: "second", reason: "the remote cannot be written" }),
      );
      await fireEvent.click(screen.getByRole("button", { name: "移動" }));
      expect(await screen.findByRole("alert")).toHaveTextContent(
        "second は読み取り専用です（remote に書き込めません）",
      );
      expect(onclose).not.toHaveBeenCalled();
    });

    it("shows the refusal of a plan, and cannot move until there is one", async () => {
      const { routes } = setup();
      routes.plan.mockImplementation(() =>
        fail(400, "choice.required", { what: "--workspace", candidates: ["api", "web"] }),
      );
      await pick("second");
      expect(await screen.findByRole("alert")).toHaveTextContent(
        "移動先のワークスペースを選んでください",
      );
      expect(screen.getByRole("button", { name: "移動" })).toBeDisabled();
    });
  });

  it("closes with the cancel button", async () => {
    const { onclose } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "キャンセル" }));
    expect(onclose).toHaveBeenCalled();
  });
});
