import { svelte } from "@sveltejs/vite-plugin-svelte";
import { svelteTesting } from "@testing-library/svelte/vite";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [svelte(), svelteTesting()],
  test: {
    environment: "jsdom",
    include: ["core/**/*.test.ts", "app/**/*.test.ts"],
    setupFiles: ["./vitest.setup.ts"],
    // A test takes about 0.5s alone; on a busy machine (other suites at the same time) it took
    // over the default 5s. A test that hangs still fails, at 20s.
    testTimeout: 20_000,
  },
});
