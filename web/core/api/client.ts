import createClient, { type Middleware } from "openapi-fetch";
import type { components, paths } from "./schema";

export type Schemas = components["schemas"];
export type TaskView = Schemas["TaskView"];
export type ShowView = Schemas["ShowView"];

export type Api = ReturnType<typeof createApi>;

// ApiError is a problem+json response of the server. type is the Code of the
// error (task.not_found...); args are its arguments; body is the whole
// problem, for what args do not hold (schema.violation lists what it broke).
export class ApiError extends Error {
  readonly status: number;
  readonly type: string;
  readonly detail: string;
  readonly args: Record<string, unknown>;
  readonly body: unknown;

  constructor(init: {
    status: number;
    type: string;
    detail?: string;
    args?: Record<string, unknown>;
    body?: unknown;
  }) {
    super(init.detail || init.type);
    this.name = "ApiError";
    this.status = init.status;
    this.type = init.type;
    this.detail = init.detail ?? "";
    this.args = init.args ?? {};
    this.body = init.body;
  }
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

// toApiError makes the ApiError of a failed response. error is what
// openapi-fetch read from the body: the parsed problem, or the text when the
// body is not JSON (a proxy's page).
export function toApiError(status: number, error: unknown): ApiError {
  if (isRecord(error) && typeof error.type === "string") {
    return new ApiError({
      status: typeof error.status === "number" ? error.status : status,
      type: error.type,
      detail: typeof error.detail === "string" ? error.detail : "",
      args: isRecord(error.args) ? error.args : {},
      body: error,
    });
  }
  return new ApiError({
    status,
    type: `http.${status}`,
    detail: typeof error === "string" ? error : "",
    body: error,
  });
}

// createApi is the typed client of the server. It sends the token as a
// bearer token, and calls onUnauthorized when the server refuses it
// (401 auth.token).
export function createApi(opts: {
  token: () => string | null;
  fetch?: typeof fetch;
  onUnauthorized?: () => void;
}) {
  const client = createClient<paths>({
    baseUrl: typeof location !== "undefined" ? location.origin : "",
    fetch: opts.fetch,
  });
  const auth: Middleware = {
    onRequest({ request }) {
      const t = opts.token();
      if (t) request.headers.set("Authorization", `Bearer ${t}`);
      return request;
    },
    async onResponse({ response }) {
      if (response.status === 401 && opts.onUnauthorized) {
        const p: unknown = await response
          .clone()
          .json()
          .catch(() => null);
        if (isRecord(p) && p.type === "auth.token") opts.onUnauthorized();
      }
      return undefined;
    },
  };
  client.use(auth);
  return client;
}

// unwrap returns the data of a call, or throws the ApiError of its problem.
export async function unwrap<T>(
  p: Promise<{ data?: T; error?: unknown; response: Response }>,
): Promise<T> {
  const r = await p;
  if (r.error !== undefined || !r.response.ok) throw toApiError(r.response.status, r.error);
  return r.data as T;
}
