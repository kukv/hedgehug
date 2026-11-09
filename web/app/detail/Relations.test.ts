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
    const group = await screen.findByRole("group", { name: "Blocked by" });
    expect(within(group).getByText("REL-2")).toBeInTheDocument();
    expect(within(group).getByText("破壊的変更")).toBeInTheDocument();
  });

  it("follows the title and the status of a related task when the cache takes in a change", async () => {
    const tasks = [blocker];
    const k = makeCore({ show, tasks });
    k.schema.subscribe = () => () => false; // only the cache tells this screen about the other tasks
    mount(Relations, { show, readonly: false }, k);
    const group = await screen.findByRole("group", { name: "Blocked by" });
    expect(within(group).getByText("破壊的変更")).toBeInTheDocument();
    tasks[0] = { ...blocker, title: "変更後の題" };
    k.notify();
    await waitFor(() => expect(within(group).getByText("変更後の題")).toBeInTheDocument());
  });

  it("keeps the ID a link and shows the ULID to copy when the pointer is on it", async () => {
    setup();
    const group = await screen.findByRole("group", { name: "Blocked by" });
    const link = within(group).getByRole("link", { name: "REL-2" });
    expect(link).toHaveAttribute("href", "/tasks/UB");
    expect(within(group).queryByText("UB")).toBeNull();
    await fireEvent.mouseEnter(link.parentElement as HTMLElement);
    expect(within(group).getByText("UB")).toBeInTheDocument();
    expect(within(group).getByRole("button", { name: "ULID をコピー" })).toBeInTheDocument();
  });

  it("removes a blocker with a PATCH of the rest", async () => {
    const { k } = setup();
    k.api.updateTask.mockImplementation(() => ok({ task: taskView({ ulid: "U6" }), commit: "c1" }));
    await fireEvent.click(await screen.findByRole("button", { name: "REL-2 を外す" }));
    expect(k.api.updateTask).toHaveBeenCalledWith("U6", { blocked_by: [] }, { store: "shared" });
    await waitFor(() =>
      expect(k.cache.applyWrite).toHaveBeenCalledWith({ task: expect.anything(), commit: "c1" }),
    );
  });

  it("disables the controls while a change is being written, so the next one cannot lose it", async () => {
    const { k } = setup();
    let done: (v: unknown) => void = () => {};
    k.api.updateTask.mockImplementation(() => new Promise<unknown>((r) => (done = r)));
    const remove = await screen.findByRole("button", { name: "REL-2 を外す" });
    await fireEvent.click(remove);
    expect(remove).toBeDisabled();
    expect(screen.getByRole("button", { name: "関連を追加" })).toBeDisabled();
    done({ task: taskView({ ulid: "U6" }), commit: "c1" });
    await waitFor(() => expect(remove).toBeEnabled());
  });

  it("選べる関連は順方向の 5 つの表示名だけ", async () => {
    setup();
    await fireEvent.click(screen.getByRole("button", { name: "関連を追加" }));
    await fireEvent.click(screen.getByRole("button", { name: "関連の種類" }));
    expect(screen.getAllByRole("menuitem").map((e) => e.textContent?.trim())).toEqual([
      "Blocked by",
      "Causes",
      "Relates to",
      "Duplicates",
      "Clones",
    ]);
  });

  it("逆方向のグループは表示名で出し、外すボタンを持たない", async () => {
    const withBack = showView({
      ...show,
      reverse: { caused_by: [{ ulid: "UB", id: "REL-2", title: "破壊的変更" }] },
    });
    const k = makeCore({ show: withBack, tasks: [blocker] });
    mount(Relations, { show: withBack, readonly: false }, k);
    const group = await screen.findByRole("group", { name: "Caused by" });
    expect(within(group).getByText("REL-2")).toBeInTheDocument();
    expect(within(group).queryByRole("button")).toBeNull();
  });

  it("adds a relation: pick the kind, search, pick a task", async () => {
    const { k } = setup();
    k.api.listTasks.mockImplementation(() =>
      ok({ tasks: [taskView({ ulid: "UN", id: "REL-9", title: "新しい関連" })] }),
    );
    k.api.updateTask.mockImplementation(() => ok({ task: taskView({ ulid: "U6" }), commit: "c2" }));
    await fireEvent.click(screen.getByRole("button", { name: "関連を追加" }));
    await fireEvent.click(screen.getByRole("button", { name: "関連の種類" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "Relates to" }));
    await fireEvent.input(screen.getByRole("combobox", { name: "タスクを検索" }), {
      target: { value: "REL-9" },
    });
    await fireEvent.click(await screen.findByRole("option", { name: /REL-9/ }));
    await waitFor(() =>
      expect(k.api.updateTask).toHaveBeenCalledWith("U6", { relates: ["UN"] }, { store: "shared" }),
    );
  });

  it("shows the server's error, such as ref.cross_store, in the section", async () => {
    const { k } = setup();
    k.api.listTasks.mockImplementation(() =>
      ok({ tasks: [taskView({ ulid: "UN", id: "REL-9", title: "x" })] }),
    );
    k.api.updateTask.mockImplementation(() =>
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
    await screen.findByRole("group", { name: "Blocked by" });
    expect(screen.queryByRole("button", { name: "関連を追加" })).toBeNull();
    expect(screen.queryByRole("button", { name: "REL-2 を外す" })).toBeNull();
  });
});
