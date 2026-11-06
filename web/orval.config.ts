import { defineConfig } from "orval";

export default defineConfig({
  hedgehug: {
    input: "../api/openapi.yaml",
    output: {
      target: "core/api/generated.ts",
      client: "fetch",
      mode: "single",
      urlEncodeParameters: true,
      override: {
        fetch: { includeHttpResponseReturnType: false },
        mutator: { path: "core/api/fetcher.ts", name: "fetcher" },
      },
    },
  },
});
