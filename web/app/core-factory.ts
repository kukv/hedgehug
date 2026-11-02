import { createApi } from "../core/api/client";
import { TaskCache } from "../core/cache";
import { Connection } from "../core/connection";
import { SaveController } from "../core/save";
import { SchemaCache } from "../core/schema";
import { StoreDirectory } from "../core/stores";
import type { AppCore } from "./context";

/** Builds the one set of core objects the whole app shares. */
export function createCore(opts: {
  token: () => string | null;
  fetch?: typeof fetch;
  onUnauthorized?: () => void;
}): AppCore {
  const api = createApi(opts);
  const conn = new Connection({
    token: opts.token,
    fetch: opts.fetch,
    api,
    onUnauthorized: opts.onUnauthorized,
  });
  const cache = new TaskCache({ api, conn });
  const save = new SaveController({ api, cache, conn });
  const stores = new StoreDirectory({ api, conn, cache });
  const schema = new SchemaCache({ api, conn });
  return { api, conn, cache, save, stores, schema };
}
