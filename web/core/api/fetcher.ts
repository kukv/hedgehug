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

// toApiError makes the ApiError of a failed response. error is what the body
// held: the parsed problem, or the text when the body is not JSON (a proxy's
// page).
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

// Connect is what a client of the server is made from.
export type Connect = {
  token: () => string | null;
  fetch?: typeof fetch;
  onUnauthorized?: () => void;
};

function parse(text: string): unknown {
  if (text === "") return undefined;
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

// fetcher is the one request of every generated operation. It sends the token
// as a bearer token, returns the parsed body, and throws the ApiError of a
// failed response; a 401 auth.token also calls onUnauthorized. connect comes
// from createApi, which puts it in the options of each call.
export async function fetcher<T>(
  url: string,
  options: RequestInit & { connect?: Connect },
): Promise<T> {
  const { connect, ...init } = options;
  if (!connect) throw new Error(`no client for ${url}: call it through createApi`);
  const headers = new Headers(init.headers);
  const token = connect.token();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const send = connect.fetch ?? globalThis.fetch;
  const res = await send(url, { ...init, headers });
  const body = parse(await res.text());
  if (res.ok) return body as T;
  if (res.status === 401 && isRecord(body) && body.type === "auth.token") {
    connect.onUnauthorized?.();
  }
  throw toApiError(res.status, body);
}
