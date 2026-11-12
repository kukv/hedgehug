import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig, type Plugin } from "vite";

// Go は web/dist を埋め込む(`//go:embed all:dist`)。git は dist/.gitkeep だけを持ち、make web の前でも
// go build が通るようにしている。Vite はビルドのたびに出力先を空にするので、.gitkeep を置き直す。
function keepGitkeep(): Plugin {
  let outDir = "";
  return {
    name: "hedgehug-keep-gitkeep",
    apply: "build",
    configResolved(config) {
      outDir = config.build.outDir;
    },
    closeBundle() {
      mkdirSync(outDir, { recursive: true });
      writeFileSync(join(outDir, ".gitkeep"), "");
    },
  };
}

// @fontsource は woff2 と woff を並べ、Vite は両方を写す。この画面を開くブラウザはどれも woff2 を読むので、
// woff を外して dist を woff2 だけにする。
const woff2Only = {
  postcssPlugin: "hedgehug-woff2-only",
  Declaration: {
    src(decl: { value: string }) {
      decl.value = decl.value.replace(/,\s*url\([^)]*\.woff\)\s*format\(["']woff["']\)/g, "");
    },
  },
};

// 開発中は、pnpm dev が画面を出し、/api を、起動したサーバーへ送る。HEDGEHUG_UI は、サーバーが出した URL
// (http://127.0.0.1:<番号>/#token=<トークン>)。
const ui = process.env.HEDGEHUG_UI ? new URL(process.env.HEDGEHUG_UI) : null;

export default defineConfig({
  plugins: [svelte(), keepGitkeep()],
  css: { postcss: { plugins: [woff2Only] } },
  build: {
    outDir: join(import.meta.dirname, "..", "dist"),
    emptyOutDir: true,
  },
  server: {
    open: ui ? `/${ui.hash}` : false,
    proxy: ui ? { "/api": { target: ui.origin, changeOrigin: true } } : undefined,
  },
});
