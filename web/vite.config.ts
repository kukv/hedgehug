import { existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { basename, dirname, join } from "node:path";
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

// The packages whose code or fonts end up in dist are redistributed with the binary, and their
// licenses ask for their texts to go along. Write them to dist/THIRD_PARTY_LICENSES from the
// modules in the output chunks and the sources of the copied files, so a devDependency such as
// svelte (its runtime is in the bundle) counts and code that is not bundled does not.
// `make licenses` puts this file after the Go part.
const licenseFile = /^(licen[cs]e|copying|notice)([.\-_].*)?$/i;
function thirdPartyLicenses(): Plugin {
  let root = "";
  return {
    name: "hedgehug-third-party-licenses",
    apply: "build",
    configResolved(config) {
      root = config.root;
    },
    generateBundle(_, bundle) {
      const pkgs = new Map<string, { name: string; version: string }>();
      for (const out of Object.values(bundle)) {
        // A copied file such as a font is not a module; its source is in originalFileNames.
        const ids =
          out.type === "chunk" ? out.moduleIds : out.originalFileNames.map((f) => join(root, f));
        for (const id of ids) {
          if (id.startsWith("\0") || !id.includes("node_modules")) continue;
          // The package's root is the nearest directory up with a package.json that has a name: some
          // packages keep a package.json without one for a subpath.
          for (
            let dir = dirname(id.replace(/\?.*/, ""));
            basename(dir) !== "node_modules";
            dir = dirname(dir)
          ) {
            const file = join(dir, "package.json");
            if (!existsSync(file)) continue;
            const pkg = JSON.parse(readFileSync(file, "utf8"));
            if (!pkg.name) continue;
            pkgs.set(dir, pkg);
            break;
          }
        }
      }
      const sections = [...pkgs].sort(([, a], [, b]) => a.name.localeCompare(b.name));
      let text = "";
      for (const [dir, pkg] of sections) {
        const files = readdirSync(dir).filter((f) => licenseFile.test(f));
        if (!files.some((f) => !/^notice/i.test(f)))
          this.error(`no license file for ${pkg.name} in ${dir}`);
        text += `\n==== ${pkg.name} ${pkg.version} (npm) ====\n`;
        for (const f of files)
          text += `\n---- ${f} ----\n\n${readFileSync(join(dir, f), "utf8").trimEnd()}\n`;
      }
      this.emitFile({ type: "asset", fileName: "THIRD_PARTY_LICENSES", source: text });
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
  plugins: [svelte(), keepGitkeep(), thirdPartyLicenses()],
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
