import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { parentRule } from "../../core/schema";
import {
  fail,
  makeCore,
  mount,
  ok,
  schemaView,
  stubLayout,
  taskView,
  typeInto,
  viewOf,
  workspaces,
} from "../detail/testkit";
import { selection } from "../selection.svelte";
import { resetToasts, toasts } from "../toast.svelte";
import CreateTask from "./CreateTask.svelte";

const parent = taskView({ ulid: "UP", id: "REL-8", title: "リリース作業", kind: "task" });
const created = taskView({ ulid: "U9", id: "REL-9", title: "新しい作業" });

function setup(opts: { onclose?: () => void; one?: boolean } = {}) {
  const k = makeCore({
    tasks: [parent],
    workspaces: opts.one ? [workspaces[0] as (typeof workspaces)[number]] : undefined,
  });
  k.api.createTask.mockImplementation(() => ok({ task: created, commit: "c1" }));
  const onclose = opts.onclose ?? vi.fn();
  return { k, onclose, ...mount(CreateTask, { request: { kind: "task" }, onclose }, k) };
}

async function typeTitle(text: string) {
  await fireEvent.input(screen.getByRole("textbox", { name: "タイトル" }), {
    target: { value: text },
  });
}

describe("CreateTask", () => {
  beforeAll(stubLayout);
  beforeEach(() => {
    selection.set(null);
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("takes the selected row as the parent, fixes the workspace to the parent's and creates", async () => {
    selection.set("UP");
    const { k, onclose } = setup();
    expect(screen.getByText("REL-8")).toBeInTheDocument();
    expect(screen.queryByRole("radiogroup", { name: "ワークスペース" })).toBeNull();
    expect(screen.getByText(/親と同じワークスペース/)).toBeInTheDocument();
    await typeTitle("新しい作業");
    expect(screen.getByText(/--no-repo.*--parent REL-8/)).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    await fireEvent.click(screen.getByRole("button", { name: /^作成/ }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(k.api.createTask).toHaveBeenCalledWith(
      { title: "新しい作業", parent: "UP", kind: "task", status: "todo" },
      { store: "shared" },
    );
    expect(k.cache.applyWrite).toHaveBeenCalledWith({ task: created, commit: "c1" });
    expect(toasts.list[0]).toMatchObject({
      message: "REL-9 を作成しました · release",
      link: { label: "開く", href: "/tasks/U9" },
    });
  });

  it("does not offer the workspaces of a hidden Store", () => {
    const vault = {
      ...(workspaces[0] as (typeof workspaces)[number]),
      store: "vault",
      ulid: "WV",
      name: "vault",
      prefix: "VT",
    };
    const k = makeCore({ tasks: [parent], workspaces: [vault, ...workspaces], hidden: ["vault"] });
    mount(CreateTask, { request: { kind: "task" }, onclose: vi.fn() }, k);
    expect(screen.queryByRole("radio", { name: /vault/ })).toBeNull();
    expect(screen.getByRole("radio", { name: /release/ })).toBeChecked();
  });

  it("creates under a task of a hidden Store, in the workspace of the parent", async () => {
    selection.set("UP");
    const k = makeCore({ tasks: [parent], hidden: ["shared"] });
    k.api.createTask.mockImplementation(() => ok({ task: created, commit: "c1" }));
    mount(CreateTask, { request: { kind: "task" }, onclose: vi.fn() }, k);
    expect(screen.getByText(/親と同じワークスペース/)).toBeInTheDocument();
    await typeTitle("新しい作業");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
  });

  it("decides the workspace when the stores finish loading after the dialog opened", async () => {
    const k = makeCore({ tasks: [parent] });
    k.stores.workspaces.mockReturnValue([]);
    mount(CreateTask, { request: { kind: "task" }, onclose: vi.fn() }, k);
    await typeTitle("早押し");
    expect(screen.getByRole("button", { name: /^作成/ })).toBeDisabled();
    k.stores.workspaces.mockReturnValue(workspaces);
    k.notify();
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
  });

  it("strikes out the kinds the parent cannot hold, with the reason as a tooltip", async () => {
    selection.set("UP");
    setup();
    const project = screen.getByRole("radio", { name: /プロジェクト/ });
    expect(project).toBeDisabled();
    expect(project.closest("label")).toHaveAttribute("title", parentRule(schemaView, "project"));
    await waitFor(() => expect(screen.getByRole("radio", { name: /タスク/ })).toBeChecked());
    expect(screen.getByRole("radio", { name: /バグ/ })).toBeEnabled();
  });

  it("asks for the workspace when there is no parent and several workspaces", async () => {
    const { k } = setup();
    expect(screen.getByRole("radio", { name: /release（REL）/ })).toBeChecked();
    await fireEvent.click(screen.getByRole("radio", { name: /shared（SH）/ }));
    await typeTitle("x");
    expect(screen.getByText(/ID は作成時に振ります/)).toHaveTextContent("shared（SH）");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    await fireEvent.click(screen.getByRole("button", { name: /^作成/ }));
    await waitFor(() => expect(k.api.createTask).toHaveBeenCalled());
    const [sent] = k.api.createTask.mock.calls[0] as [Record<string, unknown>];
    expect(sent).toMatchObject({ title: "x", workspace: "shared" });
    expect(sent).not.toHaveProperty("parent");
  });

  it("does not ask when there is only one workspace", () => {
    setup({ one: true });
    expect(screen.queryByRole("radiogroup", { name: "ワークスペース" })).toBeNull();
    expect(screen.getByText("ワークスペースは release だけです")).toBeInTheDocument();
  });

  it("'続けて作成' empties the title and the body and keeps the dialog", async () => {
    const { k, onclose, container } = setup();
    await typeTitle("一つ目");
    typeInto(container, "メモ");
    await waitFor(() => expect(screen.getByRole("button", { name: "続けて作成" })).toBeEnabled());
    await fireEvent.click(screen.getByRole("button", { name: "続けて作成" }));
    await waitFor(() => expect(k.api.createTask).toHaveBeenCalledTimes(1));
    const [sent] = k.api.createTask.mock.calls[0] as [{ body: string }];
    expect(sent.body).toBe("メモ");
    await waitFor(() => expect(screen.getByRole("textbox", { name: "タイトル" })).toHaveValue(""));
    expect(viewOf(container).state.doc.toString()).toBe("");
    expect(onclose).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("creates with Enter in the title, but not while an IME composes", async () => {
    const { k } = setup();
    await typeTitle("日本語");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    const title = screen.getByRole("textbox", { name: "タイトル" });
    await fireEvent.keyDown(title, { key: "Enter", isComposing: true });
    expect(k.api.createTask).not.toHaveBeenCalled();
    await fireEvent.keyDown(title, { key: "Enter" });
    await waitFor(() => expect(k.api.createTask).toHaveBeenCalledTimes(1));
  });

  it("creates with Ctrl+Enter anywhere in the dialog", async () => {
    const { k } = setup();
    await typeTitle("x");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    await fireEvent.keyDown(window, { key: "Enter", ctrlKey: true });
    await waitFor(() => expect(k.api.createTask).toHaveBeenCalledTimes(1));
  });

  it("creates with Ctrl+Enter typed in the assignee field, with the assignee, once", async () => {
    const { k } = setup();
    await typeTitle("x");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    const assignee = screen.getByRole("textbox", { name: "担当者" });
    await fireEvent.input(assignee, { target: { value: "sato" } });
    await fireEvent.keyDown(assignee, { key: "Enter", ctrlKey: true });
    await waitFor(() => expect(k.api.createTask).toHaveBeenCalledTimes(1));
    const [sent] = k.api.createTask.mock.calls[0] as [object];
    expect(sent).toMatchObject({ assignees: ["sato"] });
  });

  it("does not create with Ctrl+Enter while an IME composes", async () => {
    const { k } = setup();
    await typeTitle("x");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    await fireEvent.keyDown(window, { key: "Enter", ctrlKey: true, isComposing: true });
    expect(k.api.createTask).not.toHaveBeenCalled();
  });

  it("creates with Ctrl+Enter in the body editor, once", async () => {
    const { k, container } = setup();
    await typeTitle("x");
    typeInto(container, "メモ");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    await fireEvent.keyDown(viewOf(container).contentDOM, {
      key: "Enter",
      code: "Enter",
      keyCode: 13,
      ctrlKey: true,
    });
    await waitFor(() => expect(k.api.createTask).toHaveBeenCalledTimes(1));
    const [sent] = k.api.createTask.mock.calls[0] as [{ body: string }];
    expect(sent.body).toBe("メモ");
  });

  it("focuses the title as soon as the dialog opens with a parent", async () => {
    const k = makeCore({ tasks: [parent] });
    mount(CreateTask, { request: { kind: "task", parent: "UP" }, onclose: vi.fn() }, k);
    expect(screen.getByRole("button", { name: "親を外す" })).toBeInTheDocument();
    await new Promise((r) => setTimeout(r, 0));
    expect(screen.getByRole("textbox", { name: "タイトル" })).toHaveFocus();
  });

  it("focuses the title as soon as the dialog opens", async () => {
    setup();
    // Dialog moves the focus after a tick: look once everything has settled.
    await new Promise((r) => setTimeout(r, 0));
    expect(screen.getByRole("textbox", { name: "タイトル" })).toHaveFocus();
  });

  it("shows the command with <タイトル> and cannot copy it until a title is typed", async () => {
    setup();
    const cmd = screen.getByRole("button", { name: /コマンドをコピー/ });
    expect(cmd).toHaveTextContent("<タイトル>");
    expect(cmd).toBeDisabled();
    await typeTitle("x");
    expect(screen.getByRole("button", { name: /コマンドをコピー/ })).toBeEnabled();
  });

  it("cannot create without a title", () => {
    setup();
    expect(screen.getByRole("button", { name: /^作成/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: "続けて作成" })).toBeDisabled();
  });

  it("sends the properties and the assignee that were set", async () => {
    const { k } = setup();
    await typeTitle("x");
    await fireEvent.click(screen.getByRole("button", { name: "priority" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "high" }));
    const assignee = screen.getByRole("textbox", { name: "担当者" });
    await fireEvent.input(assignee, { target: { value: "sato" } });
    await fireEvent.keyDown(assignee, { key: "Enter" });
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    await fireEvent.click(screen.getByRole("button", { name: /^作成/ }));
    await waitFor(() => expect(k.api.createTask).toHaveBeenCalled());
    const [sent] = k.api.createTask.mock.calls[0] as [object];
    expect(sent).toMatchObject({ assignees: ["sato"], props: { priority: "high" } });
  });

  it("shows the server's error in the dialog and stays open", async () => {
    const { k, onclose } = setup();
    k.api.createTask.mockImplementation(() => fail(422, "workspace.choose", { names: ["a", "b"] }));
    await typeTitle("x");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    await fireEvent.click(screen.getByRole("button", { name: /^作成/ }));
    expect(await within(screen.getByRole("dialog")).findByRole("alert")).toBeInTheDocument();
    expect(onclose).not.toHaveBeenCalled();
  });

  it("lets the parent be removed, which brings the workspace choice back", async () => {
    selection.set("UP");
    setup();
    await fireEvent.click(screen.getByRole("button", { name: "親を外す" }));
    expect(screen.getByRole("radiogroup", { name: "ワークスペース" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /release（REL）/ })).toBeChecked();
  });

  it("puts the task under the parent that the request names", () => {
    const k = makeCore({ tasks: [parent] });
    mount(CreateTask, { request: { kind: "task", parent: "UP" }, onclose: vi.fn() }, k);
    expect(screen.getByText("REL-8")).toBeInTheDocument();
  });

  describe("the status of a column of the kanban", () => {
    async function createWith(request: Record<string, unknown>) {
      const k = makeCore({ tasks: [parent] });
      k.api.createTask.mockImplementation(() => ok({ task: created, commit: "c1" }));
      mount(CreateTask, { request: { kind: "task", ...request }, onclose: vi.fn() }, k);
      await typeTitle("列から");
      await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
      return {
        k,
        send: async () => {
          await fireEvent.click(screen.getByRole("button", { name: /^作成/ }));
          await waitFor(() => expect(k.api.createTask).toHaveBeenCalled());
          const [sent] = k.api.createTask.mock.calls[0] as [Record<string, unknown>];
          return sent;
        },
      };
    }

    it("starts the task in the status the request names, and the command says so", async () => {
      const { send } = await createWith({ status: "waiting" });
      expect(screen.getByText(/-p status=waiting/)).toBeInTheDocument();
      expect((await send()).status).toBe("waiting");
    });

    it("starts it in the default status of the category the request names", async () => {
      const { send } = await createWith({ category: "doing" });
      expect((await send()).status).toBe("in_progress");
    });

    it("falls back to the category, then to todo, for a status the workspace lacks", async () => {
      const a = await createWith({ status: "review", category: "done" });
      expect((await a.send()).status).toBe("done");
      cleanup();
      const b = await createWith({ status: "review" });
      expect((await b.send()).status).toBe("todo");
    });
  });
});

describe("CreateTask in a Store that cannot be written", () => {
  beforeAll(stubLayout);
  beforeEach(() => {
    selection.set(null);
    resetToasts();
    history.replaceState(null, "", "/");
  });

  const vault = {
    ...(workspaces[0] as (typeof workspaces)[number]),
    store: "vault",
    ulid: "WV",
    name: "vault",
    prefix: "VT",
  };
  const why = { vault: "the remote cannot be written" };

  function open(list: typeof workspaces, readOnlyBy: Record<string, string>, request = {}) {
    const k = makeCore({ tasks: [parent], workspaces: list, readOnlyBy });
    k.api.createTask.mockImplementation(() => ok({ task: created, commit: "c1" }));
    return {
      k,
      ...mount(CreateTask, { request: { kind: "task", ...request }, onclose: vi.fn() }, k),
    };
  }

  it("disables the workspace of a read-only Store with the reason, and starts from a writable one", async () => {
    const { k } = open([vault, ...workspaces], why);
    const ro = screen.getByRole("radio", { name: /vault（VT）/ });
    expect(ro).toBeDisabled();
    expect(ro.closest("label")).toHaveTextContent("vault（閲覧のみ）: remote に書き込めません");
    expect(ro.closest("label")).toHaveAttribute(
      "title",
      "vault（閲覧のみ）: remote に書き込めません",
    );
    expect(ro).toHaveAccessibleDescription("— vault（閲覧のみ）: remote に書き込めません");
    expect(screen.getByRole("radio", { name: /release（REL）/ })).toBeChecked();
    await typeTitle("x");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    await fireEvent.click(screen.getByRole("button", { name: /^作成/ }));
    await waitFor(() => expect(k.api.createTask).toHaveBeenCalled());
    expect(k.api.createTask.mock.calls[0]?.[1]).toMatchObject({ store: "shared" });
  });

  it("cannot be sent when every Store is read-only, and says so", async () => {
    const { k } = open([vault], why);
    await typeTitle("x");
    const send = screen.getByRole("button", { name: /^作成/ });
    await waitFor(() => expect(send).toBeDisabled());
    expect(screen.getByText("書き込める Store がありません")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "続けて作成" })).toBeDisabled();
    await fireEvent.keyDown(window, { key: "Enter", ctrlKey: true });
    expect(k.api.createTask).not.toHaveBeenCalled();
  });

  it("cannot put a task under a parent of a read-only Store", async () => {
    const { k } = open(
      [vault, ...workspaces],
      { shared: "the remote cannot be written" },
      { parent: "UP" },
    );
    await typeTitle("x");
    expect(screen.getByRole("button", { name: /^作成/ })).toBeDisabled();
    expect(
      screen.getByText("この Store は閲覧のみです（remote に書き込めません）"),
    ).toBeInTheDocument();
    await fireEvent.keyDown(window, { key: "Enter", ctrlKey: true });
    expect(k.api.createTask).not.toHaveBeenCalled();
  });

  it("follows a Store that turns read-only after the dialog opened", async () => {
    const { k } = open(workspaces, {});
    await typeTitle("x");
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeEnabled());
    k.stores.readOnly.mockReturnValue("the remote cannot be written");
    k.notify();
    await waitFor(() => expect(screen.getByRole("button", { name: /^作成/ })).toBeDisabled());
  });
});
