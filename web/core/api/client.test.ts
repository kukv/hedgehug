import { describe, expect, it, vi } from "vitest";
import { ApiError, createApi, toApiError, unwrap } from "./client";

function problem(status: number, body: Record<string, unknown>): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/problem+json" },
  });
}

describe("createApi", () => {
  it("sends the token as a bearer token", async () => {
    const f = vi.fn(async (_req: Request) => Response.json([]));
    const api = createApi({ token: () => "tok", fetch: f as unknown as typeof fetch });
    await api.GET("/api/stores");
    const req = f.mock.calls[0]?.[0] as unknown as Request;
    expect(req.headers.get("Authorization")).toBe("Bearer tok");
    expect(new URL(req.url).pathname).toBe("/api/stores");
  });

  it("sends no Authorization header without a token", async () => {
    const f = vi.fn(async (_req: Request) => Response.json([]));
    const api = createApi({ token: () => null, fetch: f as unknown as typeof fetch });
    await api.GET("/api/stores");
    const req = f.mock.calls[0]?.[0] as unknown as Request;
    expect(req.headers.has("Authorization")).toBe(false);
  });

  it("calls onUnauthorized for 401 auth.token only", async () => {
    const onUnauthorized = vi.fn();
    let res = problem(401, { type: "auth.token", title: "Unauthorized", status: 401 });
    const api = createApi({
      token: () => "bad",
      fetch: (async () => res) as unknown as typeof fetch,
      onUnauthorized,
    });
    await api.GET("/api/stores");
    expect(onUnauthorized).toHaveBeenCalledTimes(1);

    res = problem(404, { type: "task.not_found", title: "Not Found", status: 404 });
    await api.GET("/api/stores");
    expect(onUnauthorized).toHaveBeenCalledTimes(1);
  });
});

describe("unwrap", () => {
  it("returns the data", async () => {
    const api = createApi({
      token: () => "t",
      fetch: (async () => Response.json([{ name: "a" }])) as unknown as typeof fetch,
    });
    const stores = await unwrap(api.GET("/api/stores"));
    expect(stores).toEqual([{ name: "a" }]);
  });

  it("throws the problem as an ApiError", async () => {
    const body = {
      type: "task.not_found",
      title: "Not Found",
      status: 404,
      detail: 'no task "X-1"',
      args: { input: "X-1" },
    };
    const api = createApi({
      token: () => "t",
      fetch: (async () => problem(404, body)) as unknown as typeof fetch,
    });
    const err = await unwrap(api.GET("/api/stores")).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    const e = err as ApiError;
    expect(e.status).toBe(404);
    expect(e.type).toBe("task.not_found");
    expect(e.detail).toBe('no task "X-1"');
    expect(e.args).toEqual({ input: "X-1" });
    expect(e.body).toEqual(body);
  });

  it("types an error that is not a problem by its status", () => {
    const e = toApiError(502, "<html>Bad Gateway</html>");
    expect(e.type).toBe("http.502");
    expect(e.detail).toBe("<html>Bad Gateway</html>");
    expect(e.args).toEqual({});
  });
});
