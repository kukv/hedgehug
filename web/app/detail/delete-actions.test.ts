import { describe, expect, it } from "vitest";
import { ApiError } from "../../core/api/client";
import {
  collectDescendants,
  deleteTask,
  probePurge,
  purgeTargetsOf,
  purgeTask,
  restoreTask,
} from "./delete-actions";
import { fail, makeCore, ok, taskView } from "./testkit";

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

describe("purge", () => {
  const targets = [
    { ulid: "U6", id: "REL-6", title: "x" },
    { ulid: "U7", id: "REL-7", title: "y" },
  ];

  it("purgeTargetsOf reads the targets of a purge.unconfirmed problem only", async () => {
    const k = makeCore();
    k.api.DELETE.mockImplementation(() => fail(409, "purge.unconfirmed", { targets }));
    const err = await deleteTask({ api: k.api as never, cache: k.cache }, t).catch(
      (e: unknown) => e,
    );
    expect(err).toBeInstanceOf(ApiError);
    expect(purgeTargetsOf(err)).toEqual(targets);
    expect(purgeTargetsOf(new Error("x"))).toBeNull();
    k.api.DELETE.mockImplementation(() => fail(404, "task.not_found"));
    expect(
      purgeTargetsOf(
        await deleteTask({ api: k.api as never, cache: k.cache }, t).catch((e: unknown) => e),
      ),
    ).toBeNull();
  });

  it("probePurge asks without a confirmation and returns what must be confirmed", async () => {
    const k = makeCore();
    k.api.DELETE.mockImplementation(() => fail(409, "purge.unconfirmed", { targets }));
    expect(await probePurge({ api: k.api as never, cache: k.cache }, t)).toEqual(targets);
    expect(k.api.DELETE).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "U6" }, query: { store: "shared", purge: true } },
    });
  });

  it("probePurge rethrows other errors", async () => {
    const k = makeCore();
    k.api.DELETE.mockImplementation(() => fail(404, "task.not_found"));
    await expect(probePurge({ api: k.api as never, cache: k.cache }, t)).rejects.toBeInstanceOf(
      ApiError,
    );
  });

  it("purgeTask sends the confirmed ULIDs and tells the cache which tasks are gone", async () => {
    const k = makeCore();
    k.api.DELETE.mockImplementation(() => ok({ commit: "c3", ulids: ["U6", "U7"] }));
    await purgeTask({ api: k.api as never, cache: k.cache }, t, ["U6", "U7"]);
    expect(k.api.DELETE).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "U6" }, query: { store: "shared", purge: true, confirm: "U6,U7" } },
    });
    expect(k.cache.applyWrite).toHaveBeenCalledWith({ ulids: ["U6", "U7"], commit: "c3" });
  });
});
