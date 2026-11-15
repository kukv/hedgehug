import { NotFound, Refused, type ViewContentDto } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createViewsPort } from "./views";

afterEach(() => {
  vi.unstubAllGlobals();
});

function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async (_url: string, _init?: RequestInit) =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

const id = "01abcdefghijklmnopqrstuvwx";
const content: ViewContentDto = {
  query: "status:Todo",
  layout: "table",
  table: { columns: ["id", "title"], group: "", sort: { key: "id", desc: true } },
};
const view = { id, name: "Bugs", content };

function initOf(f: ReturnType<typeof respond>): RequestInit {
  return f.mock.calls[0]?.[1] ?? {};
}

function bodyOf(f: ReturnType<typeof respond>): unknown {
  return JSON.parse(String(initOf(f).body));
}

describe("createViewsPort", () => {
  it("lists the views", async () => {
    const f = respond(200, { views: [view] });
    expect(await createViewsPort().list()).toEqual([view]);
    expect(String(f.mock.calls[0]?.[0])).toBe("/api/views");
  });

  it("creates a view with a name and content", async () => {
    const f = respond(200, view);
    expect(await createViewsPort().create("Bugs", content)).toEqual(view);
    expect(String(f.mock.calls[0]?.[0])).toBe("/api/views");
    expect(initOf(f).method).toBe("POST");
    expect(bodyOf(f)).toEqual({ name: "Bugs", content });
  });

  it("saves the content", async () => {
    const f = respond(200, view);
    await createViewsPort().save(id, content);
    expect(String(f.mock.calls[0]?.[0])).toBe(`/api/views/${id}`);
    expect(initOf(f).method).toBe("PUT");
    expect(bodyOf(f)).toEqual({ content });
  });

  it("renames", async () => {
    const f = respond(200, view);
    await createViewsPort().rename(id, "Tasks");
    expect(String(f.mock.calls[0]?.[0])).toBe(`/api/views/${id}/rename`);
    expect(bodyOf(f)).toEqual({ name: "Tasks" });
  });

  it("duplicates", async () => {
    const f = respond(200, view);
    expect(await createViewsPort().duplicate(id)).toEqual(view);
    expect(String(f.mock.calls[0]?.[0])).toBe(`/api/views/${id}/duplicate`);
    expect(initOf(f).method).toBe("POST");
  });

  it("removes", async () => {
    const f = respond(204, undefined);
    await createViewsPort().remove(id);
    expect(String(f.mock.calls[0]?.[0])).toBe(`/api/views/${id}`);
    expect(initOf(f).method).toBe("DELETE");
  });

  it("asks the key impact with the store, workspace and key", async () => {
    const f = respond(200, { affected: true });
    expect(await createViewsPort().keyImpact("S1", "W1", "customer")).toBe(true);
    const url = new URL(String(f.mock.calls[0]?.[0]), "http://x");
    expect(url.pathname).toBe("/api/views/key-impact");
    expect(Object.fromEntries(url.searchParams)).toEqual({
      storeId: "S1",
      workspaceId: "W1",
      key: "customer",
    });
  });

  it("throws Refused with the refusal text on 422", async () => {
    respond(422, {
      title: "Unprocessable Entity",
      status: 422,
      detail: "同じ名前のビューがあります",
    });
    const err = await createViewsPort()
      .create("Bugs", content)
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(Refused);
    expect((err as Refused).message).toBe("同じ名前のビューがあります");
  });

  it("throws NotFound on 404", async () => {
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createViewsPort().remove(id)).rejects.toBeInstanceOf(NotFound);
  });
});
