import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig, type Plugin } from "vite";

// The Go side embeds web/dist (`//go:embed all:dist`), and git tracks only
// dist/.gitkeep so that go build works before `make web`. Vite empties the
// output directory on every build, so put the file back.
function keepGitkeep(): Plugin {
  let outDir = "";
  return {
    name: "hedgehug-keep-gitkeep",
    apply: "build",
    configResolved(config) {
      outDir = join(config.root, config.build.outDir);
    },
    closeBundle() {
      mkdirSync(outDir, { recursive: true });
      writeFileSync(join(outDir, ".gitkeep"), "");
    },
  };
}

// @fontsource lists woff2 and then woff for each font file, and Vite copies both. Every browser
// that runs this app reads woff2, so drop the woff and keep dist to woff2 only.
const woff2Only = {
  postcssPlugin: "hedgehug-woff2-only",
  Declaration: {
    src(decl: { value: string }) {
      decl.value = decl.value.replace(/,\s*url\([^)]*\.woff\)\s*format\(["']woff["']\)/g, "");
    },
  },
};

// In development, `pnpm dev` serves the page and proxies /api to the server
// that `hh ui --no-open` started. HEDGEHUG_UI is the URL it printed
// (http://127.0.0.1:<port>/#token=<token>): the proxy goes to its origin, and
// the browser the dev server opens gets its token in the hash.
const ui = process.env.HEDGEHUG_UI ? new URL(process.env.HEDGEHUG_UI) : null;

export default defineConfig({
  plugins: [svelte(), keepGitkeep()],
  css: { postcss: { plugins: [woff2Only] } },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    rolldownOptions: {
      output: {
        // CodeMirror is most of the code that is not the app's. Two chunks of its own keep every
        // chunk under the 500 kB the build warns about: the base (the parsers and what the editor
        // is built on), and the rest of the editor with the Markdown language. The base imports
        // nothing from the rest, so the chunks have no cycle.
        codeSplitting: {
          groups: [
            {
              name: "codemirror-base",
              test: /node_modules[\\/](?:\.pnpm[\\/][^\\/]+[\\/]node_modules[\\/])?(?:@lezer|@codemirror[\\/](?:state|view|language)|crelt|style-mod|w3c-keyname)[\\/]/,
              priority: 2,
              includeDependenciesRecursively: false,
            },
            {
              name: "codemirror",
              test: /node_modules[\\/](?:\.pnpm[\\/][^\\/]+[\\/]node_modules[\\/])?(?:@codemirror|codemirror)[\\/]/,
              priority: 1,
              includeDependenciesRecursively: false,
            },
          ],
        },
      },
    },
  },
  server: {
    open: ui ? `/${ui.hash}` : false,
    proxy: ui ? { "/api": { target: ui.origin, changeOrigin: true } } : undefined,
  },
});
