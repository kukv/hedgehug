import { fireEvent, screen, waitFor } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import type { Link } from "../../core/github";
import { renderWithCore } from "../test-utils";
import GitHubSection from "./GitHubSection.svelte";
import { showView } from "./testkit";

const pr: Link = {
  type: "github_pr",
  ref: "github.com/org-a/web/pull/88",
  label: "org-a/web#88",
  sync: "mirror",
  title: "API クライアントを v2 へ",
  branch: "hh/REL-6-api",
  via: "branch",
  state: "draft",
  review: "pending",
  checks: "pending",
};
const issue: Link = {
  type: "github_issue",
  ref: "github.com/org-a/web/issues/45",
  label: "org-a/web#45",
  sync: "push",
  title: "Web を更新",
  state: "open",
};
const ok = (data: unknown) => Promise.resolve({ data, response: new Response() });
const show = showView({ ulid: "U1", id: "REL-6", store: "shared", links: [issue, pr] });

function setup(readonly = false) {
  const applyWrite = vi.fn();
  const api = {
    GET: vi.fn((path: string) =>
      ok(
        path === "/api/github"
          ? [
              {
                repo: "github.com/org-a/web",
                last_fetch: new Date(Date.now() - 180_000).toISOString(),
              },
            ]
          : { default_sync: "link", push: ["title", "body"], mirror: ["title"], props: {} },
      ),
    ),
    POST: vi.fn(() => ok({ task: show, commit: "c1" })),
    DELETE: vi.fn(() => ok({ task: show, commit: "c2" })),
  };
  renderWithCore(GitHubSection, { show, readonly }, { api, cache: { applyWrite } });
  return { api, applyWrite };
}

describe("GitHubSection", () => {
  it("shows the pull request first, with its state, review, CI, branch and where it came from", async () => {
    setup();
    const rows = document.querySelectorAll(".row");
    expect(rows[0]?.textContent).toContain("org-a/web#88");
    expect(rows[1]?.textContent).toContain("org-a/web#45");
    const row = rows[0]?.textContent ?? "";
    for (const t of ["API クライアントを v2 へ", "draft", "—", "CI 実行中"]) {
      expect(row).toContain(t);
    }
    expect(row).toContain("hh/REL-6-api · ブランチ名から自動リンク");
    expect(screen.getByRole("link", { name: "org-a/web#88" }).getAttribute("href")).toBe(
      "https://github.com/org-a/web/pull/88",
    );
    await screen.findByText("3 分前に取得");
    await screen.findByText("タイトル・本文");
  });

  it("changes the mode of an issue", async () => {
    const { api, applyWrite } = setup();
    await fireEvent.change(screen.getByLabelText("同期: org-a/web#45"), {
      target: { value: "mirror" },
    });
    await waitFor(() => expect(applyWrite).toHaveBeenCalled());
    expect(api.POST).toHaveBeenCalledWith("/api/tasks/{id}/links", {
      params: { path: { id: "U1" }, query: { store: "shared" } },
      body: { issue: "org-a/web#45", sync: "mirror" },
    });
  });

  it("takes a link off", async () => {
    const { api, applyWrite } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "リンクを外す: org-a/web#88" }));
    await waitFor(() => expect(applyWrite).toHaveBeenCalled());
    expect(api.DELETE).toHaveBeenCalledWith("/api/tasks/{id}/links", {
      params: { path: { id: "U1" }, query: { store: "shared", pr: "org-a/web#88" } },
    });
  });

  it("links a pull request from the dialog", async () => {
    const { api, applyWrite } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "リンク" }));
    await fireEvent.input(screen.getByPlaceholderText("org/repo#123"), {
      target: { value: "org-a/web#9" },
    });
    screen.getByText(/hh link REL-6 --pr "org-a\/web#9"/);
    await fireEvent.click(screen.getByRole("button", { name: "リンクする" }));
    await waitFor(() => expect(applyWrite).toHaveBeenCalled());
    expect(api.POST).toHaveBeenCalledWith("/api/tasks/{id}/links", {
      params: { path: { id: "U1" }, query: { store: "shared" } },
      body: { pr: "org-a/web#9" },
    });
  });

  it("has no writes on a Store that only reads", () => {
    setup(true);
    expect(screen.queryByRole("button", { name: "リンク" })).toBeNull();
    expect(screen.queryByRole("button", { name: "リンクを外す: org-a/web#88" })).toBeNull();
    expect((screen.getByLabelText("同期: org-a/web#45") as HTMLSelectElement).disabled).toBe(true);
    screen.getByRole("button", { name: "PR を作る" });
  });
});
