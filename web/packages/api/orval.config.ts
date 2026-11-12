import { defineConfig } from "orval";

export default defineConfig({
  hedgehug: {
    input: "../../../api/openapi.yaml",
    output: {
      target: "src/generated.ts",
      client: "fetch",
      mode: "single",
      urlEncodeParameters: true,
      override: {
        fetch: { includeHttpResponseReturnType: false },
        mutator: { path: "src/fetcher.ts", name: "fetcher" },
      },
    },
  },
});
