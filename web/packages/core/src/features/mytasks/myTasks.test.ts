import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { MyTasksPort } from "../../port/myTasks";
import type { ScreenStatesPort } from "../../port/search";
import { get } from "../../reactive/store";
import { createScreenStates } from "../screenstate/screenStates";
import { result, timer } from "../search/fixtures.test-util";
import { createToasts } from "../toasts/toasts";
import { createMyTasks } from "./myTasks";

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

type Reply = { errors: string[]; rows: ReturnType<typeof result>["rows"] };
const reply = (...ids: string[]): Reply => ({ errors: [], rows: result(...ids).rows });

function setup(saved: string | null = null) {
  const list = vi.fn<MyTasksPort["list"]>(async (f) => reply(f));
  const sp: ScreenStatesPort = { get: vi.fn(async () => saved), put: vi.fn(async () => {}) };
  const toasts = createToasts(timer);
  const m = createMyTasks({ port: { list }, screenStates: createScreenStates(sp), timer, toasts });
  return { m, list, sp, toasts };
}

describe("createMyTasks", () => {
  it("open reads the saved state", async () => {
    const t = setup('{"filter":"priority:high","groupBy":"project"}');
    await t.m.open();
    expect(get(t.m.state)).toMatchObject({
      filter: "priority:high",
      groupBy: "project",
      loaded: true,
    });
    expect(t.list).toHaveBeenCalledTimes(1);
    expect(t.list).toHaveBeenCalledWith("priority:high");
  });

  it("bad saved fields fall back", async () => {
    const t = setup('{"filter":3,"groupBy":"nope"}');
    await t.m.open();
    expect(get(t.m.state)).toMatchObject({ filter: "", groupBy: "due" });
  });

  it("setFilter waits then asks", async () => {
    const t = setup();
    await t.m.open();
    t.list.mockClear();
    vi.mocked(t.sp.put).mockClear();
    t.m.setFilter("a");
    t.m.setFilter("ab");
    expect(get(t.m.state).filter).toBe("ab");
    await vi.advanceTimersByTimeAsync(149);
    expect(t.list).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(t.list).toHaveBeenCalledTimes(1);
    expect(t.list).toHaveBeenCalledWith("ab");
    expect(t.sp.put).toHaveBeenLastCalledWith(
      "my-tasks",
      JSON.stringify({ filter: "ab", groupBy: "due" }),
    );
  });

  it("late answer is dropped", async () => {
    const t = setup();
    await t.m.open();
    const resolvers: Record<string, (r: Reply) => void> = {};
    t.list.mockImplementation((q) => new Promise<Reply>((r) => (resolvers[q] = r)));
    t.m.setFilter("a");
    await vi.advanceTimersByTimeAsync(150);
    t.m.setFilter("ab");
    await vi.advanceTimersByTimeAsync(150);
    resolvers.ab?.(reply("AB"));
    await vi.advanceTimersByTimeAsync(0);
    resolvers.a?.(reply("A"));
    await vi.advanceTimersByTimeAsync(0);
    expect(get(t.m.state).rows.map((r) => r.itemId)).toEqual(["AB"]);
  });

  it("reload keeps the typed filter", async () => {
    const t = setup();
    await t.m.open();
    t.m.setFilter("ab");
    await t.m.reload();
    expect(get(t.m.state).filter).toBe("ab");
    expect(t.list).toHaveBeenLastCalledWith("ab");
  });

  it("errors empty the rows", async () => {
    const t = setup();
    await t.m.open();
    t.list.mockResolvedValueOnce({ errors: ["x"], rows: [] });
    await t.m.reload();
    expect(get(t.m.state)).toMatchObject({ errors: ["x"], rows: [] });
  });

  it("keeps the previous rows and toasts when the read fails", async () => {
    const t = setup();
    await t.m.open();
    t.list.mockRejectedValueOnce(new Error("boom"));
    await t.m.reload();
    expect(get(t.m.state).rows.map((r) => r.itemId)).toEqual([""]);
    expect(get(t.toasts.list).map((x) => x.text)).toEqual(["boom"]);
  });

  it("setGroupBy writes without asking", async () => {
    const t = setup();
    await t.m.open();
    t.list.mockClear();
    t.m.setGroupBy("workspace");
    expect(t.list).not.toHaveBeenCalled();
    expect(get(t.m.state).groupBy).toBe("workspace");
    expect(t.sp.put).toHaveBeenLastCalledWith(
      "my-tasks",
      JSON.stringify({ filter: "", groupBy: "workspace" }),
    );
  });
});
