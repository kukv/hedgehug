import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import Relations from "./Relations.svelte";
import { fail, makeCore, mount, ok, showView, taskView } from "./testkit";

const blocker = taskView({ ulid: "UB", id: "REL-2", title: "破壊的変更", status: "waiting" });
const show = showView({
  ulid: "U6",
  id: "REL-6",
  store: "shared",
  blocked_by: [{ ulid: "UB", id: "REL-2", title: "破壊的変更" }],
  props: {},
});

const setup = (readonly = false) => {
  const k = makeCore({ show, tasks: [blocker] });
  return { k, ...mount(Relations, { show, readonly }, k) };
};

describe("Relations", () => {
  it("says that only tasks of the same store can be referred to, and groups the relations", async () => {
    setup();
    expect(screen.getByText("同じ Store（shared）のタスクだけを参照できます")).toBeInTheDocument();
    const group = await screen.findByRole("group", { name: "ブロックされている" });
    expect(within(group).getByText("REL-2")).toBeInTheDocument();
    expect(within(group).getByText("破壊的変更")).toBeInTheDocument();
  });

  it("follows the title and the status of a related task when the cache takes in a change", async () => {
    const tasks = [blocker];
    const k = makeCore({ show, tasks });
    k.schema.subscribe = () => () => false; // only the cache tells this screen about the other tasks
    mount(Relations, { show, readonly: false }, k);
    const group = await screen.findByRole("group", { name: "ブロックされている" });
    expect(within(group).getByText("破壊的変更")).toBeInTheDocument();
    tasks[0] = { ...blocker, title: "変更後の題" };
    k.notify();
    await waitFor(() => expect(within(group).getByText("変更後の題")).toBeInTheDocument());
  });

  it("removes a blocker with a PATCH of the rest", async () => {
    const { k } = setup();
    k.api.PATCH.mockImplementation(() => ok({ task: taskView({ ulid: "U6" }), commit: "c1" }));
    await fireEvent.click(await screen.findByRole("button", { name: "REL-2 を外す" }));
    expect(k.api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "U6" }, query: { store: "shared" } },
      body: { blocked_by: [] },
    });
    await waitFor(() =>
      expect(k.cache.applyWrite).toHaveBeenCalledWith({ task: expect.anything(), commit: "c1" }),
    );
  });

  it("disables the controls while a change is being written, so the next one cannot lose it", async () => {
    const { k } = setup();
    let done: (v: Awaited<ReturnType<typeof ok>>) => void = () => {};
    k.api.PATCH.mockImplementation(
      () => new Promise<Awaited<ReturnType<typeof ok>>>((r) => (done = r)),
    );
    const remove = await screen.findByRole("button", { name: "REL-2 を外す" });
    await fireEvent.click(remove);
    expect(remove).toBeDisabled();
    expect(screen.getByRole("button", { name: "関連を追加" })).toBeDisabled();
    done({ data: { task: taskView({ ulid: "U6" }), commit: "c1" }, response: new Response() });
    await waitFor(() => expect(remove).toBeEnabled());
  });

  it("disables a single-valued kind that already has a value, with the reason", async () => {
    const withDup = showView({ ...show, props: { duplicates: "UD" } });
    const k = makeCore({ show: withDup, tasks: [blocker] });
    mount(Relations, { show: withDup, readonly: false }, k);
    await fireEvent.click(screen.getByRole("button", { name: "関連を追加" }));
    await fireEvent.click(screen.getByRole("button", { name: "関連の種類" }));
    expect(screen.getByRole("menuitem", { name: /重複している/ })).toBeDisabled();
    expect(screen.getByText("すでに設定されています（外してから追加）")).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /関連/ })).toBeEnabled();
  });

  it("adds a relation: pick the kind, search, pick a task", async () => {
    const { k } = setup();
    k.api.GET.mockImplementation(() =>
      ok({ tasks: [taskView({ ulid: "UN", id: "REL-9", title: "新しい関連" })] }),
    );
    k.api.PATCH.mockImplementation(() => ok({ task: taskView({ ulid: "U6" }), commit: "c2" }));
    await fireEvent.click(screen.getByRole("button", { name: "関連を追加" }));
    await fireEvent.click(screen.getByRole("button", { name: "関連の種類" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "関連" }));
    await fireEvent.input(screen.getByRole("combobox", { name: "タスクを検索" }), {
      target: { value: "REL-9" },
    });
    await fireEvent.click(await screen.findByRole("option", { name: /REL-9/ }));
    await waitFor(() =>
      expect(k.api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}", {
        params: { path: { id: "U6" }, query: { store: "shared" } },
        body: { props: { relates: ["UN"] } },
      }),
    );
  });

  it("shows the server's error, such as ref.cross_store, in the section", async () => {
    const { k } = setup();
    k.api.GET.mockImplementation(() =>
      ok({ tasks: [taskView({ ulid: "UN", id: "REL-9", title: "x" })] }),
    );
    k.api.PATCH.mockImplementation(() =>
      fail(422, "ref.cross_store", { store: "other", id: "X-1" }),
    );
    await fireEvent.click(screen.getByRole("button", { name: "関連を追加" }));
    await fireEvent.input(screen.getByRole("combobox", { name: "タスクを検索" }), {
      target: { value: "X" },
    });
    await fireEvent.click(await screen.findByRole("option", { name: /REL-9/ }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });

  it("hides the add and remove controls when read-only", async () => {
    setup(true);
    await screen.findByRole("group", { name: "ブロックされている" });
    expect(screen.queryByRole("button", { name: "関連を追加" })).toBeNull();
    expect(screen.queryByRole("button", { name: "REL-2 を外す" })).toBeNull();
  });
});
