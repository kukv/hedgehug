import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { ok } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import IssueImportDialog from "./IssueImportDialog.svelte";

const target = { store: "shared", parent: "P1", parentId: "REL-1" };

function setup() {
  const onclose = vi.fn();
  const api = {
    importIssues: vi.fn((body: { dry_run: boolean }) =>
      ok({
        repo: "org-a/web",
        count: 3,
        skipped: 1,
        dry_run: body.dry_run,
        created: body.dry_run ? [] : ["REL-2", "REL-3", "REL-4"],
        warnings: [],
      }),
    ),
  };
  renderWithCore(IssueImportDialog, { target, onclose }, { api });
  return { api, onclose };
}

describe("IssueImportDialog", () => {
  it("counts first, then takes in under the project", async () => {
    const { api, onclose } = setup();
    await fireEvent.input(screen.getByLabelText("リポジトリ"), { target: { value: "org-a/web" } });
    await fireEvent.input(screen.getByLabelText("ラベル"), { target: { value: "v2" } });
    await fireEvent.click(screen.getByRole("button", { name: "件数を確認する" }));
    await screen.findByText("open の Issue 3 件 · うち 1 件はリンク済みのため飛ばします");
    expect(api.importIssues).toHaveBeenLastCalledWith(
      { repo: "org-a/web", label: "v2", parent: "P1", dry_run: true },
      { store: "shared" },
    );
    await fireEvent.click(screen.getByRole("button", { name: "3 件を mirror で取り込む" }));
    await waitFor(() => expect(onclose).toHaveBeenCalled());
    expect(api.importIssues).toHaveBeenLastCalledWith(
      { repo: "org-a/web", label: "v2", parent: "P1", dry_run: false },
      { store: "shared" },
    );
  });

  it("counts again after the label changes", async () => {
    setup();
    await fireEvent.input(screen.getByLabelText("リポジトリ"), { target: { value: "org-a/web" } });
    await fireEvent.input(screen.getByLabelText("ラベル"), { target: { value: "v2" } });
    await fireEvent.click(screen.getByRole("button", { name: "件数を確認する" }));
    await screen.findByRole("button", { name: "3 件を mirror で取り込む" });
    await fireEvent.input(screen.getByLabelText("ラベル"), { target: { value: "v3" } });
    screen.getByRole("button", { name: "件数を確認する" });
  });
});
