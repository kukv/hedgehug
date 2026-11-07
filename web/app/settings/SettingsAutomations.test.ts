import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { ok, schemaView } from "../detail/testkit";
import { emitter, renderWithCore } from "../test-utils";
import SettingsAutomations from "./SettingsAutomations.svelte";

const schema = { ...schemaView, workflow: { transitions: {}, required: { done: ["points"] } } };
const rules = [
  {
    id: "pr-opened",
    on: "pr.opened",
    status: "in_progress",
    enabled: true,
    disabled_in: [{ ulid: "P1", id: "REL-1", title: "オンコール" }],
  },
  {
    id: "pr-merged",
    on: "pr.merged",
    when: "all_linked_prs_merged",
    category: "done",
    enabled: true,
    disabled_in: [],
  },
  {
    id: "pr-review",
    on: "pr.review_requested",
    status: "waiting",
    enabled: false,
    disabled_in: [],
  },
];
const stats = [
  { rule: "pr-opened", applied: 12, rejected: 0, rejected_tasks: [] },
  {
    rule: "pr-merged",
    applied: 5,
    rejected: 1,
    rejected_tasks: [{ ulid: "T1", id: "REL-11", title: "x" }],
  },
];
const fields = {
  default_sync: "link",
  push: ["title", "body", "status", "assignees", "labels"],
  mirror: ["title", "status", "assignees"],
  props: {},
};
const github = [
  {
    store: "shared",
    repo: "github.com/org-a/web",
    warnings: [],
    limit: 5000,
    remaining: 4812,
    app_missing: false,
    last_fetch: new Date(Date.now() - 180_000).toISOString(),
  },
];
const workspace = {
  store: "shared",
  ulid: "W1",
  name: "dev",
  projects: [{ ulid: "P1", id: "REL-1", title: "オンコール" }],
};

function setup(opts: { readOnly?: string } = {}) {
  const e = emitter();
  const api = {
    listAutomations: vi.fn(() => ok(rules)),
    getAutomationStats: vi.fn(() => ok(stats)),
    getAutomationFields: vi.fn(() => ok(fields)),
    getGithub: vi.fn(() => ok(github)),
    addAutomation: vi.fn(() => ok([])),
    importIssues: vi.fn((body: { dry_run?: boolean }) =>
      ok({ count: 3, skipped: 1, created: body.dry_run ? [] : ["A", "B", "C"], warnings: [] }),
    ),
    setAutomationEnabled: vi.fn(() => ok(rules)),
    deleteAutomation: vi.fn(() => ok(rules)),
    updateAutomationFields: vi.fn(() => ok(fields)),
  };
  renderWithCore(
    SettingsAutomations,
    { store: "shared" },
    {
      api,
      conn: { sync: new Map(), on: () => () => {}, subscribe: e.subscribe },
      schema: { get: () => schema, load: async () => {}, subscribe: e.subscribe },
      stores: {
        stores: () => [{ name: "shared" }],
        tone: () => "teal",
        readOnly: () => opts.readOnly ?? null,
        workspaces: () => [workspace],
        displayId: (_: string, id: string) => id,
        subscribe: e.subscribe,
      },
    },
  );
  return { api };
}

const rule = (id: string) => screen.findByRole("article", { name: id });

describe("SettingsAutomations", () => {
  it("shows the reads and the rate limit", async () => {
    setup();
    await screen.findByText("レート制限 4,812 / 5,000");
    expect(screen.getByText(/GitHub から取得: 3 分前/)).toBeTruthy();
  });

  it("shows what a rule does and how it went", async () => {
    setup();
    const opened = within(await rule("pr-opened"));
    expect(opened.getByText("PR が開かれた")).toBeTruthy();
    expect(opened.getByText("ステータス → 進行中")).toBeTruthy();
    expect(opened.getByText("id: pr-opened · on: pr.opened")).toBeTruthy();
    expect(await opened.findByText("直近 7 日 12 回")).toBeTruthy();
    expect(opened.getByRole("link", { name: "オンコール で停止" })).toHaveAttribute(
      "href",
      "/projects/P1/settings",
    );
    const merged = within(await rule("pr-merged"));
    expect(merged.getByText("リンク中の PR がすべてマージ済み")).toBeTruthy();
    expect(merged.getByText("カテゴリ done の既定（完了）")).toBeTruthy();
    expect(
      await merged.findByText(
        /完了には points が必要です.*REL-11 で、制約により 1 回拒否されました/,
      ),
    ).toBeTruthy();
    expect(within(await rule("pr-review")).getByText("無効")).toBeTruthy();
  });

  it("turns a rule off", async () => {
    const { api } = setup();
    await fireEvent.click(await screen.findByRole("checkbox", { name: "有効: pr-opened" }));
    await waitFor(() =>
      expect(api.setAutomationEnabled).toHaveBeenCalledWith(
        "pr-opened",
        { enabled: false },
        { store: "shared" },
      ),
    );
  });

  it("puts the switch back when the write fails", async () => {
    const { api } = setup();
    api.setAutomationEnabled.mockRejectedValueOnce(new Error("no"));
    const box = await screen.findByRole("checkbox", { name: "有効: pr-opened" });
    await fireEvent.click(box);
    await waitFor(() => expect(box).toBeChecked());
  });

  it("removes a rule after asking", async () => {
    const { api } = setup();
    await fireEvent.click(await screen.findByRole("button", { name: "削除: pr-review" }));
    const dialog = await screen.findByRole("dialog", { name: "ルール pr-review を削除" });
    expect(dialog.textContent).toContain("hh automation rm pr-review");
    await fireEvent.click(within(dialog).getByRole("button", { name: "削除" }));
    await waitFor(() =>
      expect(api.deleteAutomation).toHaveBeenCalledWith("pr-review", { store: "shared" }),
    );
  });

  it("adds a rule with a condition only for a merge", async () => {
    const { api } = setup();
    await rule("pr-opened");
    await fireEvent.click(screen.getByRole("button", { name: "ルールを追加" }));
    const dialog = within(await screen.findByRole("dialog", { name: "ルールを追加" }));
    const add = dialog.getByRole("button", { name: "追加" });
    expect(add).toBeDisabled();
    await fireEvent.input(dialog.getByLabelText("id"), { target: { value: "r1" } });
    await fireEvent.change(dialog.getByLabelText("イベント"), { target: { value: "pr.merged" } });
    await fireEvent.change(dialog.getByLabelText("遷移先"), {
      target: { value: "category:done" },
    });
    expect(screen.getByRole("dialog").textContent).toContain(
      "hh automation add --id r1 --on pr.merged --category done --when all_linked_prs_merged",
    );
    await fireEvent.click(add);
    await waitFor(() =>
      expect(api.addAutomation).toHaveBeenCalledWith(
        expect.objectContaining({
          id: "r1",
          on: "pr.merged",
          category: "done",
          when: "all_linked_prs_merged",
        }),
        { store: "shared" },
      ),
    );
  });

  it("changes the default mode and what a mode syncs", async () => {
    const { api } = setup();
    await fireEvent.click(await screen.findByRole("radio", { name: /mirror/ }));
    await waitFor(() =>
      expect(api.updateAutomationFields).toHaveBeenLastCalledWith(
        { default_sync: "mirror" },
        { store: "shared" },
      ),
    );
    await fireEvent.click(screen.getByRole("checkbox", { name: "mirror: 本文" }));
    await waitFor(() =>
      expect(api.updateAutomationFields).toHaveBeenLastCalledWith(
        { mirror: ["title", "body", "status", "assignees"] },
        { store: "shared" },
      ),
    );
  });

  it("asks for the key of the milestone before it can sync", async () => {
    const { api } = setup();
    const box = await screen.findByRole("checkbox", { name: "push: マイルストーン" });
    expect(box).toBeDisabled();
    await fireEvent.change(screen.getByLabelText("マイルストーンのプロパティ"), {
      target: { value: "target" },
    });
    await waitFor(() =>
      expect(api.updateAutomationFields).toHaveBeenLastCalledWith(
        { props: { milestone: "target" } },
        { store: "shared" },
      ),
    );
  });

  it("takes in issues under a project or at the root of a workspace", async () => {
    const { api } = setup();
    await fireEvent.input(await screen.findByLabelText("リポジトリ"), {
      target: { value: "org-a/web" },
    });
    await fireEvent.input(screen.getByLabelText("ラベル"), { target: { value: "v2" } });
    await fireEvent.change(screen.getByLabelText("作成先"), { target: { value: "W1" } });
    await screen.findByText("$ hh gh import org-a/web --label v2 --workspace dev");
    await fireEvent.click(screen.getByRole("button", { name: "件数を確認する" }));
    await screen.findByText(/open の Issue 3 件 · うち 1\s+件はリンク済み/);
    expect(api.importIssues).toHaveBeenLastCalledWith(
      { repo: "org-a/web", label: "v2", workspace: "dev", dry_run: true },
      { store: "shared" },
    );
    await fireEvent.click(screen.getByRole("button", { name: /3\s+件を mirror で取り込む/ }));
    await waitFor(() =>
      expect(api.importIssues).toHaveBeenLastCalledWith(
        expect.objectContaining({ dry_run: false, workspace: "dev" }),
        { store: "shared" },
      ),
    );
  });

  it("writes nothing on a Store that cannot be written", async () => {
    setup({ readOnly: "remote" });
    expect(await screen.findByRole("checkbox", { name: "有効: pr-opened" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "ルールを追加" })).toBeDisabled();
    expect(await screen.findByRole("radio", { name: /mirror/ })).toBeDisabled();
  });
});
