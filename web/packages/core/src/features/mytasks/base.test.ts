import { describe, expect, it, vi } from "vitest";
import type { MyTasksPort } from "../../port/myTasks";
import { get } from "../../reactive/store";
import { result } from "../search/fixtures.test-util";
import { createMyTaskBase, myTaskProjects } from "./base";

const reply = (...ids: string[]) => ({ errors: [], rows: result(...ids).rows });

describe("createMyTaskBase", () => {
  it("base ignores the filter", async () => {
    const list = vi.fn(async () => reply("a"));
    const base = createMyTaskBase({ port: { list } satisfies MyTasksPort });
    expect(get(base.rows)).toBeNull();
    await base.reload();
    expect(list).toHaveBeenCalledWith("");
    expect(get(base.rows)?.map((r) => r.itemId)).toEqual(["a"]);
  });

  it("keeps rows on failure", async () => {
    const list = vi.fn<MyTasksPort["list"]>().mockResolvedValueOnce(reply("a"));
    const base = createMyTaskBase({ port: { list } });
    await base.reload();
    list.mockRejectedValueOnce(new Error("boom"));
    await base.reload();
    expect(get(base.rows)?.map((r) => r.itemId)).toEqual(["a"]);
  });
});

describe("myTaskProjects", () => {
  it("returns storeId/projectId pairs", () => {
    const rows = result("a", "b").rows;
    const other = { ...(rows[1] as (typeof rows)[number]), storeId: "s2", projectId: "p9" };
    expect([...myTaskProjects([rows[0] as (typeof rows)[number], other])].sort()).toEqual([
      "s1/p1",
      "s2/p9",
    ]);
  });
});
