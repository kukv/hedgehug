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
  k.api.POST.mockImplementation(() => ok({ task: created, commit: "c1" }));
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
    await fireEvent.change(screen.getByLabelText("due"), { target: { value: "2026-12-01" } });

    expect(screen.getByText(/--kind project/)).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("button", { name: "作成" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(k.api.POST).toHaveBeenCalledWith("/api/tasks", {
      params: { query: { store: "shared" } },
      body: {
        title: "Web フロント刷新",
        kind: "project",
        workspace: "release",
        assignees: ["sato"],
        repos: ["github.com/org-a/web"],
        props: { due: "2026-12-01" },
      },
    });
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
    await waitFor(() => expect(k.api.POST).toHaveBeenCalled());
    const [, init] = k.api.POST.mock.calls[0] as [string, { body: object }];
    expect(init.body).not.toHaveProperty("repos");
  });

  it("asks for the workspace when there are several", async () => {
    const { k } = setup();
    await fireEvent.click(screen.getByRole("radio", { name: /shared（SH）/ }));
    await name("x");
    await fireEvent.click(screen.getByRole("button", { name: "作成" }));
    await waitFor(() => expect(k.api.POST).toHaveBeenCalled());
    const [, init] = k.api.POST.mock.calls[0] as [string, { body: { workspace: string } }];
    expect(init.body.workspace).toBe("shared");
  });

  it("fixes the workspace when there is one", () => {
    setup(true);
    expect(screen.queryByRole("radiogroup", { name: "ワークスペース" })).toBeNull();
    expect(screen.getByText(/release（REL）/)).toBeInTheDocument();
  });

  it("focuses the name as soon as the dialog opens", async () => {
    setup();
    await waitFor(() => expect(screen.getByRole("textbox", { name: "名前" })).toHaveFocus());
  });

  it("shows the command with <名前> and cannot copy it until a name is typed", async () => {
    setup();
    const cmd = screen.getByRole("button", { name: /コマンドをコピー/ });
    expect(cmd).toHaveTextContent("<名前>");
    expect(cmd).toBeDisabled();
  });

  it("cannot be created without a name", () => {
    setup();
    expect(screen.getByRole("button", { name: "作成" })).toBeDisabled();
  });
});
