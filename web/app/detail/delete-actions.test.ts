import { describe, expect, it } from "vitest";
import { collectDescendants, deleteTask, restoreTask } from "./delete-actions";
import { makeCore, ok, taskView } from "./testkit";

const t = { ulid: "U6", store: "shared" };

describe("collectDescendants", () => {
  it("walks the children level by level", async () => {
    const k = makeCore();
    k.api.GET.mockImplementation((_p: string, o: { params: { query: { q: string } } }) => {
      const q = o.params.query.q;
      if (q === "parent:U6")
        return ok({ tasks: [taskView({ ulid: "A" }), taskView({ ulid: "B" })] });
      if (q === "parent:A") return ok({ tasks: [taskView({ ulid: "C" })] });
      return ok({ tasks: [] });
    });
    const ds = await collectDescendants(k.api as never, "shared", "U6");
    expect(ds.map((d) => d.ulid)).toEqual(["A", "B", "C"]);
  });
});

describe("deleteTask / restoreTask", () => {
  it("sends DELETE and puts the affected tasks into the cache", async () => {
    const k = makeCore();
    k.api.DELETE.mockImplementation(() => ok({ commit: "c1", tasks: [taskView({ ulid: "U6" })] }));
    await deleteTask({ api: k.api as never, cache: k.cache }, t);
    expect(k.api.DELETE).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "U6" }, query: { store: "shared" } },
    });
    expect(k.cache.applyWrite).toHaveBeenCalledWith({
      tasks: [expect.objectContaining({ ulid: "U6" })],
      commit: "c1",
    });
  });
  it("sends POST restore", async () => {
    const k = makeCore();
    k.api.POST.mockImplementation(() => ok({ commit: "c2", tasks: [] }));
    await restoreTask({ api: k.api as never, cache: k.cache }, t);
    expect(k.api.POST).toHaveBeenCalledWith("/api/tasks/{id}/restore", {
      params: { path: { id: "U6" }, query: { store: "shared" } },
    });
    expect(k.cache.applyWrite).toHaveBeenCalledWith({ tasks: [], commit: "c2" });
  });
});
