import { NotFound, Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createItemActionsPort } from "./itemActions";

afterEach(() => {
  vi.unstubAllGlobals();
});

// respond は、fetch を 1 つの応答に差し替え、呼び出しを返す。
function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async () =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

function call(fetch: ReturnType<typeof respond>, n = 0) {
  return fetch.mock.calls[n] as unknown as [string, RequestInit];
}

describe("item actions port", () => {
  it("reads the delete previews, turning a null kinds into an empty list", async () => {
    const fetch = respond(200, { count: 3, kinds: ["Task", "Step"], retentionDays: 30 });
    expect(await createItemActionsPort().deletePreview("S1", "I1")).toEqual({
      count: 3,
      kinds: ["Task", "Step"],
      retentionDays: 30,
    });
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/delete-preview");
    respond(200, { count: 0, kinds: null, retentionDays: 7 });
    expect((await createItemActionsPort().deletePreview("S1", "I1")).kinds).toEqual([]);
    const project = respond(200, { items: 12, retentionDays: 30 });
    expect(await createItemActionsPort().projectDeletePreview("S1", "P1")).toEqual({
      items: 12,
      retentionDays: 30,
    });
    expect(call(project)[0]).toBe("/api/stores/S1/projects/P1/delete-preview");
  });

  it("deletes an item and a project with DELETE", async () => {
    const fetch = respond(204, undefined);
    await createItemActionsPort().deleteItem("S1", "I1");
    await createItemActionsPort().deleteProject("S1", "P1");
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1");
    expect(call(fetch)[1].method).toBe("DELETE");
    expect(call(fetch, 1)[0]).toBe("/api/stores/S1/projects/P1");
    expect(call(fetch, 1)[1].method).toBe("DELETE");
  });

  it("reads move candidates (null is empty) and moves to a parent or a project", async () => {
    const list = [
      {
        kind: "item",
        id: "I3",
        display: "HH-3",
        title: "認証",
        projectId: "P1",
        projectTitle: "Web",
      },
      {
        kind: "project",
        id: "P2",
        display: "HH-P2",
        title: "モバイル",
        projectId: "P2",
        projectTitle: "",
      },
    ];
    const fetch = respond(200, list);
    expect(await createItemActionsPort().moveCandidates("S1", "I1")).toEqual(list);
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/move-candidates");
    respond(200, null);
    expect(await createItemActionsPort().moveCandidates("S1", "I1")).toEqual([]);
    const moved = respond(204, undefined);
    await createItemActionsPort().move("S1", "I1", { parent: "I3" });
    await createItemActionsPort().move("S1", "I1", { project: "P2" });
    expect(call(moved)[0]).toBe("/api/stores/S1/items/I1/move");
    expect(JSON.parse(String(call(moved)[1].body))).toEqual({ parent: "I3" });
    expect(JSON.parse(String(call(moved, 1)[1].body))).toEqual({ project: "P2" });
  });

  it("changes the kind with PUT and returns the id of a duplicate", async () => {
    const fetch = respond(204, undefined);
    await createItemActionsPort().changeKind("S1", "I1", "Bug");
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/kind");
    expect(call(fetch)[1].method).toBe("PUT");
    expect(JSON.parse(String(call(fetch)[1].body))).toEqual({ kind: "Bug" });
    const dup = respond(200, { id: "I9" });
    expect(await createItemActionsPort().duplicate("S1", "I1")).toBe("I9");
    expect(call(dup)[0]).toBe("/api/stores/S1/items/I1/duplicate");
    expect(call(dup)[1].method).toBe("POST");
  });

  it("turns 422 into Refused with the words of the spec and 404 into NotFound", async () => {
    respond(422, {
      title: "Unprocessable Entity",
      status: 422,
      detail: "Task は Task の下に置けません",
    });
    const refused = await createItemActionsPort()
      .move("S1", "I1", { parent: "I3" })
      .catch((e: unknown) => e);
    expect(refused).toBeInstanceOf(Refused);
    expect((refused as Refused).message).toBe("Task は Task の下に置けません");
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    const missing = await createItemActionsPort()
      .deleteItem("S1", "I1")
      .catch((e: unknown) => e);
    expect(missing).toBeInstanceOf(NotFound);
  });
});
