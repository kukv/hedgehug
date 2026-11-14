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

describe("transfers port: workspaces (B5c-19)", () => {
  it("reads the targets of a workspace, null being empty", async () => {
    const body = { targets: [{ storeId: "S3", storeName: "team" }], hiddenOnly: false };
    const fetch = respond(200, body);
    expect(await createTransfersPort().workspaceTargets("S1", "W1")).toEqual(body);
    expect(call(fetch)[0]).toBe("/api/stores/S1/workspaces/W1/transfer-targets");
    respond(200, { targets: null, hiddenOnly: true });
    expect(await createTransfersPort().workspaceTargets("S1", "W1")).toEqual({
      targets: [],
      hiddenOnly: true,
    });
  });

  it("reads the preview, turning null people into an empty list", async () => {
    const body = {
      source: "work",
      destination: "team",
      projects: 2,
      items: 5,
      deleted: 3,
      people: [{ login: "suzuki", name: "鈴木" }],
      remoteNotice: true,
      destinationHasRemote: false,
    };
    const fetch = respond(200, body);
    expect(await createTransfersPort().workspacePreview("S1", "W1", "S3")).toEqual(body);
    expect(call(fetch)[0]).toBe("/api/stores/S1/workspaces/W1/transfer-preview?toStore=S3");
    respond(200, { ...body, people: null });
    expect((await createTransfersPort().workspacePreview("S1", "W1", "S3")).people).toEqual([]);
  });

  it("turns a 422 of the preview into Refused", async () => {
    respond(422, { title: "Unprocessable Entity", status: 422, detail: "断りの文" });
    await expect(createTransfersPort().workspacePreview("S1", "W1", "S3")).rejects.toEqual(
      new Refused("断りの文"),
    );
  });

  it("posts the transfer and returns the moved workspace", async () => {
    const result = { toStore: "S3", toStoreName: "team", workspaceId: "W1", prefix: "HH" };
    const fetch = respond(200, result);
    expect(await createTransfersPort().transferWorkspace("S1", "W1", "S3")).toEqual(result);
    expect(call(fetch)[0]).toBe("/api/stores/S1/workspaces/W1/transfer");
    expect(call(fetch)[1].method).toBe("POST");
    expect(JSON.parse(String(call(fetch)[1].body))).toEqual({ toStore: "S3" });
  });

  it("turns 422, 404 and 409 of the transfer into Refused, NotFound and TransferUndoFailed", async () => {
    respond(422, { title: "Unprocessable Entity", status: 422, detail: "断りの文" });
    await expect(createTransfersPort().transferWorkspace("S1", "W1", "S3")).rejects.toEqual(
      new Refused("断りの文"),
    );
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createTransfersPort().transferWorkspace("S1", "W1", "S3")).rejects.toBeInstanceOf(
      NotFound,
    );
    const text =
      "ワークスペース「Web」を team へ移しましたが、移動元から取り除けず、取り消しもできませんでした。どちらかを削除してください";
    respond(409, { title: "Conflict", status: 409, detail: text });
    await expect(createTransfersPort().transferWorkspace("S1", "W1", "S3")).rejects.toEqual(
      new TransferUndoFailed(text),
    );
  });

  it("locates a workspace address: found with movedFrom, gone, 404 as NotFound", async () => {
    const found = {
      kind: "found",
      storeId: "S3",
      workspaceId: "W1",
      movedFrom: { old: "HH", at: "2026-11-14T00:05:00Z", display: "team", current: "team" },
    };
    const fetch = respond(200, found);
    expect(await createTransfersPort().locateWorkspace("work", "HH")).toEqual(found);
    expect(call(fetch)[0]).toBe("/api/locate-workspace?store=work&prefix=HH");
    const plain = { kind: "found", storeId: "S1", workspaceId: "W1" };
    respond(200, plain);
    expect(await createTransfersPort().locateWorkspace("work", "HH")).toEqual(plain);
    const hidden = {
      kind: "gone",
      reason: "hidden",
      old: "HH",
      at: "2026-11-14T00:05:00Z",
      newDisplay: "far",
      storeName: "far",
    };
    respond(200, hidden);
    expect(await createTransfersPort().locateWorkspace("work", "HH")).toEqual(hidden);
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createTransfersPort().locateWorkspace("work", "HH")).rejects.toBeInstanceOf(
      NotFound,
    );
  });

  it("rejects a locate-workspace answer that lacks a required field", async () => {
    const bad = [
      { kind: "found", storeId: "S1" },
      { kind: "found", workspaceId: "W1" },
      { kind: "gone", reason: "deleted", old: "HH", at: "2026-11-14T00:05:00Z" },
      { kind: "gone", reason: "elsewhere", old: "HH", at: "x", newDisplay: "team" },
      { kind: "other" },
    ];
    for (const body of bad) {
      respond(200, body);
      const err = await createTransfersPort()
        .locateWorkspace("work", "HH")
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
