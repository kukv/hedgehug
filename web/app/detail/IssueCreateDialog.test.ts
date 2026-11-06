import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { renderWithCore } from "../test-utils";
import IssueCreateDialog from "./IssueCreateDialog.svelte";
import { ok } from "./testkit";

function setup(repos: string[], post = vi.fn(() => ok({ task: {}, commit: "c1", warnings: [] }))) {
  const onclose = vi.fn();
  const applyWrite = vi.fn();
  renderWithCore(
    IssueCreateDialog,
    { task: { ulid: "T1", id: "REL-2", store: "shared", repos }, onclose },
    { api: { createIssue: post }, cache: { applyWrite, subscribe: () => () => {} } },
  );
  return { post, onclose, applyWrite };
}

describe("IssueCreateDialog", () => {
  it("starts with the one repository the task names", async () => {
    const { post, onclose, applyWrite } = setup(["github.com/org-a/web"]);
    expect(screen.getByLabelText("リポジトリ")).toHaveValue("org-a/web");
    expect(screen.getByRole("dialog").textContent).toContain("hh issue REL-2 --repo org-a/web");
    await fireEvent.click(screen.getByRole("button", { name: "作る" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(post).toHaveBeenCalledWith("T1", { repo: "org-a/web" }, { store: "shared" });
    expect(applyWrite).toHaveBeenCalledWith({ task: {}, commit: "c1" });
  });

  it("asks for the repository of a task with none or several", () => {
    setup(["github.com/org-a/web", "github.com/org-a/api"]);
    expect(screen.getByLabelText("リポジトリ")).toHaveValue("");
    expect(screen.getByRole("button", { name: "作る" })).toBeDisabled();
  });

  it("shows why it could not be made", async () => {
    const post = vi.fn(() => Promise.reject(new Error("no")));
    const { onclose } = setup(["github.com/org-a/web"], post as never);
    await fireEvent.click(screen.getByRole("button", { name: "作る" }));
    await screen.findByRole("alert");
    expect(onclose).not.toHaveBeenCalled();
  });
});
