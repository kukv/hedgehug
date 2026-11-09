import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeCore, mount, ok, taskView, workspaces } from "../detail/testkit";
import { resetToasts, toasts } from "../toast.svelte";
import CreateProject from "./CreateProject.svelte";

const created = taskView({ ulid: "UPJ", id: "REL-20", title: "Web フロント刷新", kind: "project" });

function setup(one = false) {
  const k = makeCore({
    workspaces: one ? [workspaces[0] as (typeof workspaces)[number]] : undefined,
  });
  k.api.createTask.mockImplementation(() => ok({ task: created, commit: "c1" }));
  const onclose = vi.fn();
  return { k, onclose, ...mount(CreateProject, { request: { kind: "project" }, onclose }, k) };
}

async function name(text: string) {
  await fireEvent.input(screen.getByRole("textbox", { name: "名前" }), { target: { value: text } });
  await waitFor(() => expect(screen.getByRole("button", { name: "作成" })).toBeEnabled());
}

describe("CreateProject", () => {
  beforeEach(() => {
    resetToasts();
    history.replaceState(null, "", "/");
  });

  it("creates a project with the repositories, assignee and dates", async () => {
    const { k, onclose } = setup();
    await name("Web フロント刷新");

    const repo = screen.getByRole("textbox", { name: "リポジトリ" });
    await fireEvent.input(repo, { target: { value: "git@github.com:Org-A/Web.git" } });
    expect(
      screen.getByText(/Enter で github.com\/org-a\/web として追加します/),
    ).toBeInTheDocument();
    await fireEvent.keyDown(repo, { key: "Enter" });

    const who = screen.getByRole("textbox", { name: "担当者" });
    await fireEvent.input(who, { target: { value: "sato" } });
    await fireEvent.keyDown(who, { key: "Enter" });
    await fireEvent.click(screen.getByRole("button", { name: "優先度" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: "high" }));
    await fireEvent.change(screen.getByRole("combobox", { name: "ラベル" }), {
      target: { value: "bug" },
    });
    await fireEvent.change(screen.getByLabelText("開始日"), { target: { value: "2026-11-15" } });
    await fireEvent.change(screen.getByLabelText("期限"), { target: { value: "2026-12-01" } });

    expect(
      screen.getByText(/--no-repo.*--kind project.*-p repos=github.com\/org-a\/web/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/-p priority=high -p start=2026-11-15 -p due=2026-12-01 -p labels=bug/),
    ).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("button", { name: "作成" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(k.api.createTask).toHaveBeenCalledWith(
      {
        title: "Web フロント刷新",
        kind: "project",
        workspace: "release",
        assignees: ["sato"],
        repos: ["github.com/org-a/web"],
        priority: "high",
        start: "2026-11-15",
        due: "2026-12-01",
        labels: ["bug"],
      },
      { store: "shared" },
    );
    expect(toasts.list[0]).toMatchObject({
      message: "REL-20 を作成しました · release",
      link: { label: "開く", href: "/projects/UPJ" },
    });
  });

  it("refuses text that is not a repository and says why", async () => {
    setup();
    const repo = screen.getByRole("textbox", { name: "リポジトリ" });
    await fireEvent.input(repo, { target: { value: "/home/me/repo" } });
    await fireEvent.keyDown(repo, { key: "Enter" });
    expect(screen.getByRole("alert")).toHaveTextContent("リポジトリの URL として読めません");
  });

  it("creates a project without repositories (an operations-only project)", async () => {
    const { k } = setup();
    await name("運用");
    await fireEvent.click(screen.getByRole("button", { name: "作成" }));
    await waitFor(() => expect(k.api.createTask).toHaveBeenCalled());
    const [sent] = k.api.createTask.mock.calls[0] as [object];
    expect(sent).not.toHaveProperty("repos");
  });

  it("asks for the workspace when there are several", async () => {
    const { k } = setup();
    await fireEvent.click(screen.getByRole("radio", { name: /shared（SH）/ }));
    await name("x");
    await fireEvent.click(screen.getByRole("button", { name: "作成" }));
    await waitFor(() => expect(k.api.createTask).toHaveBeenCalled());
    const [sent] = k.api.createTask.mock.calls[0] as [{ workspace: string }];
    expect(sent.workspace).toBe("shared");
  });

  it("fixes the workspace when there is one", () => {
    setup(true);
    expect(screen.queryByRole("radiogroup", { name: "ワークスペース" })).toBeNull();
    expect(screen.getByText(/release（REL）/)).toBeInTheDocument();
  });

  it("focuses the name as soon as the dialog opens", async () => {
    setup();
    await new Promise((r) => setTimeout(r, 0));
    expect(screen.getByRole("textbox", { name: "名前" })).toHaveFocus();
  });

  it("shows the command with <名前> and cannot copy it until a name is typed", async () => {
    setup();
    const cmd = screen.getByRole("button", { name: /コマンドをコピー/ });
    expect(cmd).toHaveTextContent("<名前>");
    expect(cmd).toBeDisabled();
  });

  it("decides the workspace when the stores finish loading after the dialog opened", async () => {
    const k = makeCore();
    k.stores.workspaces.mockReturnValue([]);
    mount(CreateProject, { request: { kind: "project" }, onclose: vi.fn() }, k);
    await fireEvent.input(screen.getByRole("textbox", { name: "名前" }), {
      target: { value: "早押し" },
    });
    expect(screen.getByRole("button", { name: "作成" })).toBeDisabled();
    k.stores.workspaces.mockReturnValue(workspaces);
    k.notify();
    await waitFor(() => expect(screen.getByRole("button", { name: "作成" })).toBeEnabled());
  });

  it("creates with Ctrl+Enter, not while an IME composes, and not when a field took the key", async () => {
    const { k } = setup();
    await name("p");
    await fireEvent.keyDown(window, { key: "Enter", ctrlKey: true, isComposing: true });
    expect(k.api.createTask).not.toHaveBeenCalled();
    const e = new KeyboardEvent("keydown", {
      key: "Enter",
      ctrlKey: true,
      bubbles: true,
      cancelable: true,
    });
    e.preventDefault();
    window.dispatchEvent(e);
    expect(k.api.createTask).not.toHaveBeenCalled();
    await fireEvent.keyDown(window, { key: "Enter", ctrlKey: true });
    await waitFor(() => expect(k.api.createTask).toHaveBeenCalledTimes(1));
  });

  it("does not create with Ctrl+Enter while the repository typed is invalid", async () => {
    const { k } = setup();
    await name("p");
    const repo = screen.getByRole("textbox", { name: "リポジトリ" });
    await fireEvent.input(repo, { target: { value: "not a repo" } });
    await fireEvent.keyDown(repo, { key: "Enter", ctrlKey: true });
    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(k.api.createTask).not.toHaveBeenCalled();
  });

  it("cannot be created without a name", () => {
    setup();
    expect(screen.getByRole("button", { name: "作成" })).toBeDisabled();
  });
});

describe("CreateProject in a Store that cannot be written", () => {
  beforeEach(() => {
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

  function open(list: typeof workspaces, readOnlyBy: Record<string, string>) {
    const k = makeCore({ workspaces: list, readOnlyBy });
    k.api.createTask.mockImplementation(() => ok({ task: created, commit: "c1" }));
    return { k, ...mount(CreateProject, { request: { kind: "project" }, onclose: vi.fn() }, k) };
  }

  it("disables the workspace of a read-only Store with the reason, and starts from a writable one", async () => {
    const { k } = open([vault, ...workspaces], why);
    const ro = screen.getByRole("radio", { name: /vault（VT）/ });
    expect(ro).toBeDisabled();
    expect(ro.closest("label")).toHaveTextContent("vault（閲覧のみ）: remote に書き込めません");
    expect(screen.getByRole("radio", { name: /release（REL）/ })).toBeChecked();
    await name("p");
    await fireEvent.click(screen.getByRole("button", { name: "作成" }));
    await waitFor(() => expect(k.api.createTask).toHaveBeenCalled());
    expect(k.api.createTask.mock.calls[0]?.[1]).toMatchObject({ store: "shared" });
  });

  it("does not offer the workspace of a hidden Store, and starts from one that is shown", async () => {
    const k = makeCore({ workspaces: [vault, ...workspaces], hidden: ["vault"] });
    k.api.createTask.mockImplementation(() => ok({ task: created, commit: "c1" }));
    mount(CreateProject, { request: { kind: "project" }, onclose: vi.fn() }, k);
    expect(screen.queryByRole("radio", { name: /vault（VT）/ })).toBeNull();
    expect(screen.getByRole("radio", { name: /release（REL）/ })).toBeChecked();
  });

  it("cannot be sent when every Store is read-only, and says so", async () => {
    const { k } = open([vault], why);
    await fireEvent.input(screen.getByRole("textbox", { name: "名前" }), {
      target: { value: "p" },
    });
    expect(screen.getByRole("button", { name: "作成" })).toBeDisabled();
    expect(screen.getByText("書き込める Store がありません")).toBeInTheDocument();
    await fireEvent.keyDown(window, { key: "Enter", ctrlKey: true });
    expect(k.api.createTask).not.toHaveBeenCalled();
  });
});
