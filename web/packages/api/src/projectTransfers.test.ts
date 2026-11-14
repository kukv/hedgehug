import { NotFound, Refused, TransferUndoFailed } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createTransfersPort } from "./transfers";

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

const target = { storeId: "S3", storeName: "team", workspaceId: "W1", workspaceName: "Team" };
const input = { toStore: "S3", toWorkspace: "W1", statusChoices: { st9: "st1" } };

describe("transfers port: projects (A5b-22)", () => {
  it("reads the targets of a project, null being empty", async () => {
    const fetch = respond(200, [target]);
    expect(await createTransfersPort().projectTargets("S1", "P1")).toEqual([target]);
    expect(call(fetch)[0]).toBe("/api/stores/S1/projects/P1/transfer-targets");
    respond(200, null);
    expect(await createTransfersPort().projectTargets("S1", "P1")).toEqual([]);
  });

  it("reads the preview, turning null lists into empty ones", async () => {
    const body = {
      items: 3,
      deleted: 1,
      prefix: "TM",
      source: "Web",
      destination: "team の Team",
      crossStore: true,
      remoteNotice: false,
      destinationHasRemote: true,
      hasRules: true,
      notCarried: [
        { kind: "workflow", name: "A から B", missing: ["B"], count: 0 },
        { kind: "hidden", name: "Pending", missing: null, count: 0 },
      ],
      replacements: [{ status: "st9", name: "Review", count: 2, choices: null, default: "st1" }],
    };
    const fetch = respond(200, body);
    const p = await createTransfersPort().projectPreview("S1", "P1", {
      storeId: "S3",
      workspaceId: "W1",
    });
    expect(call(fetch)[0]).toBe(
      "/api/stores/S1/projects/P1/transfer-preview?toStore=S3&toWorkspace=W1",
    );
    expect(p).toEqual({
      ...body,
      notCarried: [
        { kind: "workflow", name: "A から B", missing: ["B"], count: 0 },
        { kind: "hidden", name: "Pending", missing: [], count: 0 },
      ],
      replacements: [{ status: "st9", name: "Review", count: 2, choices: [], default: "st1" }],
    });
    respond(200, { ...body, notCarried: null, replacements: null });
    const empty = await createTransfersPort().projectPreview("S1", "P1", {
      storeId: "S3",
      workspaceId: "W1",
    });
    expect([empty.notCarried, empty.replacements]).toEqual([[], []]);
  });

  it("posts the transfer and returns the moved project", async () => {
    const result = {
      toStore: "S3",
      projectId: "P1",
      display: "TM-P4",
      destination: "team の Team",
    };
    const fetch = respond(200, result);
    expect(await createTransfersPort().transferProject("S1", "P1", input)).toEqual(result);
    expect(call(fetch)[0]).toBe("/api/stores/S1/projects/P1/transfer");
    expect(call(fetch)[1].method).toBe("POST");
    expect(JSON.parse(String(call(fetch)[1].body))).toEqual(input);
  });

  it("turns 422, 404 and 409 of the transfer into Refused, NotFound and TransferUndoFailed", async () => {
    respond(422, { title: "Unprocessable Entity", status: 422, detail: "断りの文" });
    await expect(createTransfersPort().transferProject("S1", "P1", input)).rejects.toEqual(
      new Refused("断りの文"),
    );
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createTransfersPort().transferProject("S1", "P1", input)).rejects.toBeInstanceOf(
      NotFound,
    );
    const text =
      "HH-P1 を team の TM-P4 に移しましたが、移動元から取り除けず、取り消しもできませんでした。どちらかを削除してください";
    respond(409, { title: "Conflict", status: 409, detail: text });
    await expect(createTransfersPort().transferProject("S1", "P1", input)).rejects.toEqual(
      new TransferUndoFailed(text),
    );
  });

  it("locates a project address: found with movedFrom, gone, 404 as NotFound", async () => {
    const found = {
      kind: "found",
      storeId: "S3",
      projectId: "P9",
      movedFrom: { old: "HH-P1", at: "2026-11-14T00:05:00Z", display: "TM-P4", current: "TM-P4" },
    };
    const fetch = respond(200, found);
    expect(await createTransfersPort().locateProject("work", "HH-P1")).toEqual(found);
    expect(call(fetch)[0]).toBe("/api/locate-project?store=work&id=HH-P1");
    const plain = { kind: "found", storeId: "S1", projectId: "P1" };
    respond(200, plain);
    expect(await createTransfersPort().locateProject("work", "HH-P1")).toEqual(plain);
    const hidden = {
      kind: "gone",
      reason: "hidden",
      old: "HH-P1",
      at: "2026-11-14T00:05:00Z",
      newDisplay: "TM-P4",
      storeName: "far",
    };
    respond(200, hidden);
    expect(await createTransfersPort().locateProject("work", "HH-P1")).toEqual(hidden);
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createTransfersPort().locateProject("work", "HH-P1")).rejects.toBeInstanceOf(
      NotFound,
    );
  });

  it("rejects a locate-project answer that lacks a required field", async () => {
    const bad = [
      { kind: "found", storeId: "S1" },
      { kind: "found", projectId: "P1" },
      { kind: "gone", reason: "deleted", old: "HH-P1", at: "2026-11-14T00:05:00Z" },
      { kind: "gone", reason: "elsewhere", old: "HH-P1", at: "x", newDisplay: "TM-P4" },
      { kind: "other" },
    ];
    for (const body of bad) {
      respond(200, body);
      const err = await createTransfersPort()
        .locateProject("work", "HH-P1")
        .then(
          () => null,
          (e: unknown) => e,
        );
      expect(err).toBeInstanceOf(Error);
      expect(err).not.toBeInstanceOf(NotFound);
      expect(err).not.toBeInstanceOf(Refused);
    }
  });
});
