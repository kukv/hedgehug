import type { Connect } from "./fetcher";
import * as generated from "./generated";

export { ApiError, toApiError } from "./fetcher";
export type * as Schemas from "./generated";
export type { ShowView, TaskView } from "./generated";

type Generated = typeof generated;
type Operation = (...args: never[]) => Promise<unknown>;

// Api is every operation of the server, by its operationId. The URL builders
// orval writes beside them (getGetTaskUrl) are not operations.
export type Api = {
  [K in keyof Generated as K extends `get${string}Url`
    ? never
    : Generated[K] extends Operation
      ? K
      : never]: Generated[K];
};

// createApi is the typed client of the server. A generated operation takes its
// options last, so each one is bound here to put connect in them: the operation
// then reaches the fetch and the token of this client.
export function createApi(connect: Connect): Api {
  const api: Record<string, unknown> = {};
  for (const [name, op] of Object.entries(generated)) {
    if (typeof op !== "function" || /^get.*Url$/.test(name)) continue;
    const last = op.length - 1;
    api[name] = (...args: unknown[]) => {
      const bound = [...args];
      bound[last] = { ...(args[last] as object | undefined), connect };
      return (op as (...a: unknown[]) => unknown)(...bound);
    };
  }
  return api as Api;
}
