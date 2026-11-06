import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import type { Link } from "../../core/github";
import { ok, showView, taskView } from "../detail/testkit";
import { renderWithCore } from "../test-utils";
import ProjectOverview from "./ProjectOverview.svelte";

const pr = (n: number, over: Partial<Link> = {}): Link => ({
  type: "github_pr",
  ref: `github.com/org-a/api/pull/${n}`,
  label: `org-a/api#${n}`,
  sync: "mirror",
  title: `PR ${n}`,
  state: "open",
  review: "none",
  ...over,
});

const project = showView({
  ulid: "P1",
  id: "REL-1",
  kind: "project",
  store: "shared",
  repos: ["github.com/org-a/web"],
});
const child = taskView({
  ulid: "T1",
  id: "REL-2",
  store: "shared",
  links: [pr(127, { checks: "failure" })],
});

function setup(github: unknown[] = []) {
  const applyWrite = vi.fn();
  const api = {
    getGithub: vi.fn((_query?: unknown) => ok(github)),
    updateTask: vi.fn((_id?: string, _body?: unknown, _query?: unknown) =>
      ok({ task: project, commit: "c1" }),
    ),
  };
  const none = () => () => {};
  renderWithCore(
    ProjectOverview,
    { ulid: "P1" },
    {
      api,
      cache: {
        subscribe: none,
        applyWrite,
        task: (u: string) => (u === "P1" ? project : u === "T1" ? child : undefined),
        detail: () => project,
        watchDetail: () => ({ error: () => null, dispose() {} }),
        watchList: () => ({
          ulids: () => ["T1"],
          error: () => null,
          loading: () => false,
          dispose() {},
        }),
      },
      stores: {
        subscribe: none,
        stores: () => [{ name: "shared" }],
        readOnly: () => null,
        displayId: (_s: string, id: string) => id,
        workspaces: () => [],
        loaded: () => true,
      },
    },
  );
  return { api };
}

describe("ProjectOverview", () => {
  it("lists the pull requests of the tasks and the repositories with their counts", async () => {
    setup();
    const row = (await screen.findByRole("link", { name: "api#127" })).closest("tr") as HTMLElement;
    for (const t of ["PR 127", "CI 失敗", "—"]) expect(within(row).getAllByText(t)).toBeTruthy();
    screen.getByText("org-a/web · PR 0");
    screen.getByText("org-a/api · PR 1");
  });

  it("says when GitHub was read, and that the reads are held back", async () => {
    const at = (min: number) => new Date(Date.now() + min * 60_000).toISOString();
    setup([
      {
        repo: "github.com/org-a/web",
        last_fetch: at(-3),
        next_fetch: at(2),
        slowed_until: at(30),
      },
    ]);
    await screen.findByText(/GitHub の取得 3 分前 · 次は 2 分後/);
    screen.getByText(/取得を 15 分おきに延ばしています/);
  });

  it("adds a repository to the project", async () => {
    const { api } = setup();
    await fireEvent.click(screen.getByRole("button", { name: "repo を追加" }));
    await fireEvent.input(screen.getByLabelText("リポジトリ"), { target: { value: "org-a/ops" } });
    await fireEvent.click(screen.getByRole("button", { name: "追加する" }));
    await waitFor(() =>
      expect(api.updateTask).toHaveBeenCalledWith(
        "P1",
        { repos: ["github.com/org-a/web", "github.com/org-a/ops"] },
        { store: "shared" },
      ),
    );
  });
});
