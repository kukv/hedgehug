import { describe, expect, it, vi } from "vitest";
import { ApiError, createApi, toApiError } from "./client";
import { listStores } from "./generated";

function problem(status: number, body: Record<string, unknown>): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/problem+json" },
  });
}

function server(res: () => Response) {
  return vi.fn(async (_url: string, _init: RequestInit) => res());
}

function client(f: ReturnType<typeof server>, token: string | null = "tok") {
  return createApi({ token: () => token, fetch: f as unknown as typeof fetch });
}

describe("createApi", () => {
  it("sends the token as a bearer token", async () => {
    const f = server(() => Response.json([]));
    await client(f).listStores();
    const [url, init] = f.mock.calls[0] as [string, RequestInit];
    expect(new Headers(init.headers).get("Authorization")).toBe("Bearer tok");
    expect(url).toBe("/api/stores");
  });

  it("sends no Authorization header without a token", async () => {
    const f = server(() => Response.json([]));
    await client(f, null).listStores();
    const [, init] = f.mock.calls[0] as [string, RequestInit];
    expect(new Headers(init.headers).has("Authorization")).toBe(false);
  });

  it("calls onUnauthorized for 401 auth.token only", async () => {
    const onUnauthorized = vi.fn();
    let res = problem(401, { type: "auth.token", title: "Unauthorized", status: 401 });
    const api = createApi({
      token: () => "bad",
      fetch: (async () => res) as unknown as typeof fetch,
      onUnauthorized,
    });
    await api.listStores().catch(() => {});
    expect(onUnauthorized).toHaveBeenCalledTimes(1);

    res = problem(404, { type: "task.not_found", title: "Not Found", status: 404 });
    await api.listStores().catch(() => {});
    expect(onUnauthorized).toHaveBeenCalledTimes(1);
  });

  it("reaches the client of each instance", async () => {
    const a = server(() => Response.json([]));
    const b = server(() => Response.json([]));
    const first = client(a);
    const second = client(b);
    await first.listStores();
    await second.listStores();
    expect(a).toHaveBeenCalledTimes(1);
    expect(b).toHaveBeenCalledTimes(1);
  });

  it("binds an operation with a path, a body, a query and options", async () => {
    const f = server(() => Response.json({}));
    const ctl = new AbortController();
    await client(f).updateTask("t1", { title: "x" }, { store: "s" }, { signal: ctl.signal });
    const [url, init] = f.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/tasks/t1?store=s");
    expect(init.method).toBe("PATCH");
    expect(init.body).toBe('{"title":"x"}');
    expect(init.signal).toBe(ctl.signal);
  });

  it("binds an operation when its optional arguments are left out", async () => {
    const f = server(() => Response.json({}));
    await client(f).deleteTask("t1");
    expect(f.mock.calls[0]?.[0]).toBe("/api/tasks/t1");
  });

  it("encodes a path parameter", async () => {
    const f = server(() => Response.json({}));
    await client(f).getTemplate("a b/c#d");
    expect(f.mock.calls[0]?.[0]).toBe("/api/templates/a%20b%2Fc%23d");
  });

  it("leaves an undefined query parameter out", async () => {
    const f = server(() => Response.json({}));
    await client(f).listTasks({ store: undefined, q: "a b" });
    expect(f.mock.calls[0]?.[0]).toBe("/api/tasks?q=a+b");
  });

  it("answers a 204 with nothing", async () => {
    const f = server(() => new Response(null, { status: 204 }));
    await expect(client(f).cancelDeviceLogin("d1")).resolves.toBeUndefined();
  });

  it("refuses an operation called outside createApi", async () => {
    await expect(listStores()).rejects.toThrow("createApi");
  });
});

describe("an answer", () => {
  it("is the data of the body", async () => {
    const f = server(() => Response.json([{ name: "a" }]));
    await expect(client(f).listStores()).resolves.toEqual([{ name: "a" }]);
  });

  it("throws the problem as an ApiError", async () => {
    const body = {
      type: "task.not_found",
      title: "Not Found",
      status: 404,
      detail: 'no task "X-1"',
      args: { input: "X-1" },
    };
    const err = await client(server(() => problem(404, body)))
      .listStores()
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    const e = err as ApiError;
    expect(e.status).toBe(404);
    expect(e.type).toBe("task.not_found");
    expect(e.detail).toBe('no task "X-1"');
    expect(e.args).toEqual({ input: "X-1" });
    expect(e.body).toEqual(body);
  });

  it("throws what is not a problem by its status", async () => {
    const f = server(() => new Response("<h1>bad</h1>", { status: 502 }));
    const e = (await client(f)
      .listStores()
      .catch((x: unknown) => x)) as ApiError;
    expect(e.type).toBe("http.502");
    expect(e.detail).toBe("<h1>bad</h1>");
  });

  it("lets a network failure through", async () => {
    const f = server(() => {
      throw new TypeError("Failed to fetch");
    });
    await expect(client(f).listStores()).rejects.toThrow(TypeError);
  });

  it("types an error that is not a problem by its status", () => {
    const e = toApiError(502, "<html>Bad Gateway</html>");
    expect(e.type).toBe("http.502");
    expect(e.detail).toBe("<html>Bad Gateway</html>");
    expect(e.args).toEqual({});
  });
});
