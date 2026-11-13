import { NotFound, Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createNotesPort } from "./notes";

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

const call = (fetch: ReturnType<typeof respond>) =>
  fetch.mock.calls[0] as unknown as [string, RequestInit];

const item = { kind: "item", storeId: "S1", id: "I1" } as const;
const project = { kind: "project", storeId: "S1", id: "P1" } as const;

describe("notes port: description", () => {
  it("reads an item's description", async () => {
    const fetch = respond(200, { text: "## 手順" });
    expect(await createNotesPort().description(item)).toBe("## 手順");
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/description");
  });

  it("reads a project's description", async () => {
    const fetch = respond(200, { text: "" });
    expect(await createNotesPort().description(project)).toBe("");
    expect(call(fetch)[0]).toBe("/api/stores/S1/projects/P1/description");
  });

  it("saves with the base and the text, and returns the merged text and the conflict", async () => {
    const fetch = respond(200, { text: "合わせた", conflict: true });
    const result = await createNotesPort().describe(item, "前", "後");
    const [url, init] = call(fetch);
    expect(url).toBe("/api/stores/S1/items/I1/description");
    expect(init.method).toBe("PUT");
    expect(JSON.parse(String(init.body))).toEqual({ base: "前", text: "後" });
    expect(result).toEqual({ text: "合わせた", conflict: true });
  });

  it("saves a project's description at its own address", async () => {
    const fetch = respond(200, { text: "後", conflict: false });
    await createNotesPort().describe(project, "前", "後");
    expect(call(fetch)[0]).toBe("/api/stores/S1/projects/P1/description");
  });

  it("turns a 422 into a refusal with the server's message", async () => {
    respond(422, { detail: "閲覧のみの Store です" });
    await expect(createNotesPort().describe(item, "", "x")).rejects.toThrow(Refused);
  });
});

const apiComment = {
  id: "K1",
  by: "hanako",
  byName: "花子",
  at: "2026-11-13T09:00:00.000Z",
  body: "見てください",
  edited: false,
};

describe("notes port: comments", () => {
  it("lists the comments of an item and of a project, reading null as empty", async () => {
    const fetch = respond(200, [apiComment]);
    await expect(createNotesPort().listComments(item)).resolves.toEqual([apiComment]);
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/comments");
    const none = respond(200, null);
    await expect(createNotesPort().listComments(project)).resolves.toEqual([]);
    expect(call(none)[0]).toBe("/api/stores/S1/projects/P1/comments");
  });

  it("posts, edits and deletes", async () => {
    const posted = respond(200, apiComment);
    await createNotesPort().postComment(item, "見てください");
    expect(call(posted)[0]).toBe("/api/stores/S1/items/I1/comments");
    expect(call(posted)[1].method).toBe("POST");
    expect(JSON.parse(String(call(posted)[1].body))).toEqual({ body: "見てください" });

    const edited = respond(200, { ...apiComment, edited: true });
    await expect(createNotesPort().editComment(project, "K1", "直した")).resolves.toMatchObject({
      edited: true,
    });
    expect(call(edited)[0]).toBe("/api/stores/S1/projects/P1/comments/K1");
    expect(call(edited)[1].method).toBe("PUT");

    const deleted = respond(204, undefined);
    await expect(createNotesPort().deleteComment(item, "K1")).resolves.toBeUndefined();
    expect(call(deleted)[0]).toBe("/api/stores/S1/items/I1/comments/K1");
    expect(call(deleted)[1].method).toBe("DELETE");
  });

  it("turns a 422 into Refused and a 404 into NotFound", async () => {
    respond(422, {
      title: "Unprocessable Entity",
      status: 422,
      detail: "コメントを入力してください",
    });
    const err = await createNotesPort()
      .postComment(item, " ")
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(Refused);
    expect((err as Refused).message).toBe("コメントを入力してください");
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createNotesPort().listComments(item)).rejects.toBeInstanceOf(NotFound);
  });
});

describe("notes port: worklogs", () => {
  const apiWorklog = {
    id: "W1",
    by: "hanako",
    byName: "花子",
    date: "2026-11-13",
    hours: "1.25",
    memo: "下書き",
  };

  it("lists the worklogs of an item, reading null as empty", async () => {
    const fetch = respond(200, [apiWorklog]);
    await expect(createNotesPort().listWorklogs("S1", "I1")).resolves.toEqual([apiWorklog]);
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/worklogs");
    respond(200, null);
    await expect(createNotesPort().listWorklogs("S1", "I1")).resolves.toEqual([]);
  });

  it("adds with the hours as typed, and deletes", async () => {
    const added = respond(200, apiWorklog);
    await createNotesPort().addWorklog("S1", "I1", "1.25", "下書き");
    expect(call(added)[1].method).toBe("POST");
    expect(JSON.parse(String(call(added)[1].body))).toEqual({ hours: "1.25", memo: "下書き" });
    const deleted = respond(204, undefined);
    await expect(createNotesPort().deleteWorklog("S1", "I1", "W1")).resolves.toBeUndefined();
    expect(call(deleted)[0]).toBe("/api/stores/S1/items/I1/worklogs/W1");
    expect(call(deleted)[1].method).toBe("DELETE");
  });

  it("turns a 422 into Refused with the words of the spec", async () => {
    respond(422, { title: "Unprocessable Entity", status: 422, detail: "時間を入力してください" });
    const err = await createNotesPort()
      .addWorklog("S1", "I1", "", "")
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(Refused);
    expect((err as Refused).message).toBe("時間を入力してください");
  });
});

describe("notes port: watch", () => {
  const apiItem = {
    id: "I1",
    workspace: "W1",
    project: "P1",
    parent: null,
    number: 13,
    kind: "Epic",
    rank: "V",
    title: "認証",
    watchers: ["hanako", "taro"],
    blocked: false,
    createdAt: "2026-11-13T09:00:00.000Z",
    createdBy: "taro",
    status: "st1",
    priority: "pr2",
    assignee: "",
    start: "",
    end: "",
    points: null,
    estimate: null,
    systems: [],
    choices: {},
    fields: {},
  };

  it("sends the state and returns the item", async () => {
    const fetch = respond(200, apiItem);
    const it = await createNotesPort().setWatch("S1", "I1", true);
    expect(it).toMatchObject({ id: "I1", storeId: "S1", watchers: ["hanako", "taro"] });
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/watch");
    expect(call(fetch)[1].method).toBe("PUT");
    expect(JSON.parse(String(call(fetch)[1].body))).toEqual({ on: true });
  });

  it("turns a 404 into NotFound", async () => {
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createNotesPort().setWatch("S1", "I1", false)).rejects.toBeInstanceOf(NotFound);
  });
});
