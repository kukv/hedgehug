import { describe, expect, it } from "vitest";
import { searchTasks, taskSearchQuery } from "./search";
import { fakeApi, tv } from "./testing";

describe("taskSearchQuery", () => {
  it("quotes each word so that it is free text", () => {
    expect(taskSearchQuery("switch dns")).toBe('"switch" "dns"');
    expect(taskSearchQuery("  status:todo  -x ")).toBe('"status:todo" "-x"');
    expect(taskSearchQuery('say "hi"')).toBe('"say" "hi"');
  });

  it("puts the extra query in front", () => {
    expect(taskSearchQuery("rel", "kind:project")).toBe('kind:project "rel"');
    expect(taskSearchQuery("", "kind:project")).toBe("kind:project");
    expect(taskSearchQuery("", "")).toBe("");
  });
});

describe("searchTasks", () => {
  it("asks for the tasks and keeps the first ones", async () => {
    const { api, calls } = fakeApi(() => ({
      tasks: [tv("A"), tv("B"), tv("C")],
    }));
    const found = await searchTasks(api, "rel 8", { store: "shared", limit: 2 });
    expect(found.map((t) => t.ulid)).toEqual(["A", "B"]);
    expect(calls[0]?.query.get("q")).toBe('"rel" "8"');
    expect(calls[0]?.query.get("store")).toBe("shared");
  });

  it("asks for nothing when there is nothing to look for", async () => {
    const { api, calls } = fakeApi(() => ({ tasks: [tv("A")] }));
    expect(await searchTasks(api, "   ")).toEqual([]);
    expect(calls).toHaveLength(0);
  });

  it("looks in every store without store", async () => {
    const { api, calls } = fakeApi(() => ({ tasks: [] }));
    await searchTasks(api, "x", { extra: "-status:canceled" });
    expect(calls[0]?.query.get("q")).toBe('-status:canceled "x"');
    expect(calls[0]?.query.has("store")).toBe(false);
  });
});
