import { NotFound, Refused } from "@hedgehug/core";
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

describe("transfers port", () => {
  it("reads the targets, turning null projects and epics into empty lists", async () => {
    const fetch = respond(200, [
      {
        storeId: "S1",
        storeName: "work",
        workspaceId: "W2",
        workspaceName: "Mobile",
        prefix: "MB",
        projects: [{ id: "P1", title: "アプリ", epics: null }],
      },
      {
        storeId: "S3",
        storeName: "team",
        workspaceId: "W1",
        workspaceName: "Team",
        prefix: "TM",
        projects: null,
      },
    ]);
    expect(await createTransfersPort().targets("S1", "I1")).toEqual([
      {
        storeId: "S1",
        storeName: "work",
        workspaceId: "W2",
        workspaceName: "Mobile",
        prefix: "MB",
        projects: [{ id: "P1", title: "アプリ", epics: [] }],
      },
      {
        storeId: "S3",
        storeName: "team",
        workspaceId: "W1",
        workspaceName: "Team",
        prefix: "TM",
        projects: [],
      },
    ]);
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/transfer-targets");
    respond(200, null);
    expect(await createTransfersPort().targets("S1", "I1")).toEqual([]);
  });

  it("reads the preview for a destination, with the status replacements", async () => {
    const fetch = respond(200, {
      below: 2,
      deleted: 1,
      nextDisplay: "MB-13",
      crossStore: true,
      remoteNotice: true,
      destinationHasRemote: false,
      replacements: [
        {
          status: "st9",
          name: "Review",
          count: 3,
          choices: [{ id: "st1", name: "Todo" }],
          default: "st1",
        },
        { status: "st8", name: "Hold", count: 1, choices: null, default: "" },
      ],
    });
    const p = await createTransfersPort().preview("S1", "I1", {
      storeId: "S3",
      workspaceId: "W9",
      projectId: "P9",
    });
    expect(call(fetch)[0]).toBe(
      "/api/stores/S1/items/I1/transfer-preview?toStore=S3&toWorkspace=W9&toProject=P9",
    );
    expect(p.replacements[0]).toEqual({
      status: "st9",
      name: "Review",
      count: 3,
      choices: [{ id: "st1", name: "Todo" }],
      default: "st1",
    });
    expect(p.replacements[1]?.choices).toEqual([]);
    expect(p).toMatchObject({
      below: 2,
      deleted: 1,
      nextDisplay: "MB-13",
      crossStore: true,
      remoteNotice: true,
      destinationHasRemote: false,
    });
    respond(200, { ...p, replacements: null });
    expect(
      (
        await createTransfersPort().preview("S1", "I1", {
          storeId: "S3",
          workspaceId: "W9",
          projectId: "P9",
        })
      ).replacements,
    ).toEqual([]);
  });

  it("posts the transfer and returns the new item, a null emptied list being empty", async () => {
    const fetch = respond(200, { toStore: "S3", itemId: "N1", display: "TM-4", emptied: null });
    const input = {
      toStore: "S3",
      toWorkspace: "W9",
      toProject: "P9",
      toParent: "E1",
      statusChoices: { st9: "st1" },
    };
    expect(await createTransfersPort().transfer("S1", "I1", input)).toEqual({
      toStore: "S3",
      itemId: "N1",
      display: "TM-4",
      emptied: [],
    });
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/transfer");
    expect(call(fetch)[1].method).toBe("POST");
    expect(JSON.parse(String(call(fetch)[1].body))).toEqual(input);
  });

  it("turns a 422 into Refused with the message, and a 404 into NotFound", async () => {
    respond(422, {
      title: "Unprocessable Entity",
      status: 422,
      detail: "team に送信できないため移動できません: 認証に失敗しました",
    });
    await expect(
      createTransfersPort().transfer("S1", "I1", {
        toStore: "S3",
        toWorkspace: "W",
        toProject: "P",
        statusChoices: {},
      }),
    ).rejects.toEqual(new Refused("team に送信できないため移動できません: 認証に失敗しました"));
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createTransfersPort().targets("S1", "I1")).rejects.toBeInstanceOf(NotFound);
  });

  it("locates an address: found with movedFrom, gone with the reason, 404 as NotFound", async () => {
    const found = {
      kind: "found",
      storeId: "S3",
      projectId: "P9",
      itemId: "N1",
      movedFrom: { old: "HH-5", at: "2026-11-13T00:05:00Z", display: "TM-4", current: "AR-7" },
    };
    const fetch = respond(200, found);
    expect(await createTransfersPort().locate("work", "HH-5")).toEqual(found);
    expect(call(fetch)[0]).toBe("/api/locate?store=work&id=HH-5");
    const plain = { kind: "found", storeId: "S1", projectId: "P1", itemId: "I1" };
    respond(200, plain);
    expect(await createTransfersPort().locate("work", "HH-1")).toEqual(plain);
    const deleted = {
      kind: "gone",
      reason: "deleted",
      old: "HH-5",
      at: "2026-11-13T00:05:00Z",
      newDisplay: "TM-4",
    };
    respond(200, deleted);
    expect(await createTransfersPort().locate("work", "HH-5")).toEqual(deleted);
    const elsewhere = {
      kind: "gone",
      reason: "elsewhere",
      old: "HH-5",
      at: "2026-11-13T00:05:00Z",
      newDisplay: "TM-4",
      storeName: "far",
    };
    respond(200, elsewhere);
    expect(await createTransfersPort().locate("work", "HH-5")).toEqual(elsewhere);
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createTransfersPort().locate("work", "HH-5")).rejects.toBeInstanceOf(NotFound);
  });

  it("encodes the Store name in the locate query", async () => {
    const fetch = respond(200, { kind: "found", storeId: "S1", projectId: "P1", itemId: "I1" });
    await createTransfersPort().locate("my store&x", "HH-1");
    expect(call(fetch)[0]).toBe("/api/locate?store=my+store%26x&id=HH-1");
  });

  it("rejects a locate answer that lacks a required field, as neither Refused nor NotFound", async () => {
    const bad = [
      { kind: "found", projectId: "P1", itemId: "I1" },
      { kind: "gone", old: "HH-5", at: "2026-11-13T00:05:00Z", newDisplay: "TM-4" },
      { kind: "gone", reason: "deleted", at: "2026-11-13T00:05:00Z", newDisplay: "TM-4" },
      { kind: "gone", reason: "deleted", old: "HH-5", newDisplay: "TM-4" },
      { kind: "gone", reason: "deleted", old: "HH-5", at: "2026-11-13T00:05:00Z" },
      // hidden・elsewhere は、文に Store 名を書くので storeName が要る。
      {
        kind: "gone",
        reason: "hidden",
        old: "HH-5",
        at: "2026-11-13T00:05:00Z",
        newDisplay: "TM-4",
      },
      {
        kind: "gone",
        reason: "elsewhere",
        old: "HH-5",
        at: "2026-11-13T00:05:00Z",
        newDisplay: "TM-4",
      },
      { kind: "other" },
    ];
    for (const body of bad) {
      respond(200, body);
      const err = await createTransfersPort()
        .locate("work", "HH-5")
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
