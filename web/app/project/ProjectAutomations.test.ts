import { fireEvent, screen, waitFor, within } from "@testing-library/svelte";
import { describe, expect, it, vi } from "vitest";
import { ok, schemaView } from "../detail/testkit";
import { emitter, renderWithCore } from "../test-utils";
import ProjectAutomations from "./ProjectAutomations.svelte";

const base = { on: "pr.opened", status: "in_progress", enabled: true, disabled_in: [] };
const here = { ulid: "P1", id: "REL-1", title: "リリース" };
const whole = [
  { ...base, id: "running" },
  { ...base, id: "stopped", disabled_in: [here] },
  { ...base, id: "off", enabled: false },
  { ...base, id: "mine", project: here },
];
const inProject = whole.map((r) => (r.id === "stopped" ? { ...r, enabled: false } : r));

function setup(off: string | null = null) {
  const e = emitter();
  const api = {
    listAutomations: vi.fn((query: { project?: string }) => ok(query.project ? inProject : whole)),
    setAutomationEnabled: vi.fn((_id?: string, _body?: unknown, _query?: unknown) => ok(inProject)),
    deleteAutomation: vi.fn((_id?: string, _query?: unknown) => ok(whole)),
    addAutomation: vi.fn((_body?: unknown, _query?: unknown) => ok(whole)),
  };
  renderWithCore(
    ProjectAutomations,
    { store: "shared", schema: schemaView, project: here, off },
    {
      api,
      conn: { sync: new Map(), on: () => () => {}, subscribe: e.subscribe },
    },
  );
  return { api };
}

const row = async (id: string) => {
  await screen.findByText(id);
  return within(document.querySelector(`[data-rule="${id}"]`) as HTMLElement);
};

describe("ProjectAutomations", () => {
  it("stops a rule of the Store in the project", async () => {
    const { api } = setup();
    await fireEvent.click(
      await screen.findByRole("checkbox", { name: "このプロジェクトで有効: running" }),
    );
    await waitFor(() =>
      expect(api.setAutomationEnabled).toHaveBeenCalledWith(
        "running",
        { enabled: false, project: "P1" },
        { store: "shared" },
      ),
    );
  });

  it("starts a rule stopped here again", async () => {
    const { api } = setup();
    await fireEvent.click(
      await screen.findByRole("checkbox", { name: "このプロジェクトで停止: stopped" }),
    );
    await waitFor(() =>
      expect(api.setAutomationEnabled).toHaveBeenCalledWith(
        "stopped",
        { enabled: true, project: "P1" },
        { store: "shared" },
      ),
    );
  });

  it("leaves a rule the Store turned off as it is", async () => {
    setup();
    expect((await row("off")).getByRole("checkbox")).toBeDisabled();
    expect((await row("off")).getByText("shared で無効")).toBeTruthy();
  });

  it("turns a rule of its own off with the rule's switch, and removes it", async () => {
    const { api } = setup();
    const mine = await row("mine");
    expect(mine.getByText("このプロジェクトだけ")).toBeTruthy();
    await fireEvent.click(mine.getByRole("checkbox"));
    await waitFor(() =>
      expect(api.setAutomationEnabled).toHaveBeenCalledWith(
        "mine",
        { enabled: false },
        { store: "shared" },
      ),
    );
    await fireEvent.click(mine.getByRole("button", { name: "削除: mine" }));
    await waitFor(() => expect(api.deleteAutomation).toHaveBeenCalled());
  });

  it("adds a rule of the project", async () => {
    const { api } = setup();
    await fireEvent.click(
      await screen.findByRole("button", { name: "このプロジェクトだけのルールを追加" }),
    );
    const dialog = within(
      await screen.findByRole("dialog", { name: "このプロジェクトだけのルールを追加" }),
    );
    await fireEvent.input(dialog.getByLabelText("id"), { target: { value: "r1" } });
    await fireEvent.change(dialog.getByLabelText("行き先"), {
      target: { value: "status:waiting" },
    });
    expect(screen.getByRole("dialog").textContent).toContain("--project REL-1");
    await fireEvent.click(dialog.getByRole("button", { name: "追加" }));
    await waitFor(() =>
      expect(api.addAutomation).toHaveBeenCalledWith(
        expect.objectContaining({ id: "r1", status: "waiting", project: "P1" }),
        { store: "shared" },
      ),
    );
  });

  it("writes nothing on a Store that cannot be written", async () => {
    setup("閲覧のみです");
    expect((await row("running")).getByRole("checkbox")).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "このプロジェクトだけのルールを追加" }),
    ).toBeDisabled();
  });
});
