// 画面のパッケージの依存の向き(設計書 6 節の表)。テストのファイルは vitest などを使うので、from から外す。
const notTest = "\\.test\\.ts$";
const svelte = "(^|/)node_modules/svelte/";

module.exports = {
  forbidden: [
    {
      name: "core-imports-only-itself",
      comment: "core は標準の TypeScript だけを使う(Svelte も DOM のライブラリも使わない)",
      severity: "error",
      from: { path: "^packages/core/", pathNot: notTest },
      to: { pathNot: "^packages/core/" },
    },
    {
      name: "api-imports-only-core",
      comment: "api が使ってよいのは core だけ",
      severity: "error",
      from: { path: "^packages/api/", pathNot: notTest },
      to: { path: "^packages/(ui|app)/|^app/" },
    },
    {
      name: "ui-imports-only-svelte",
      comment: "ui が使ってよいのは Svelte だけ",
      severity: "error",
      from: { path: "^packages/ui/", pathNot: notTest },
      to: { pathNot: ["^packages/ui/", svelte] },
    },
    {
      name: "packages-do-not-import-app",
      severity: "error",
      from: { path: "^packages/" },
      to: { path: "^app/" },
    },
  ],
  options: {
    doNotFollow: { path: "node_modules" },
    tsConfig: { fileName: "tsconfig.base.json" },
    enhancedResolveOptions: {
      exportsFields: ["exports"],
      conditionNames: ["svelte", "import", "default"],
      extensions: [".ts", ".js", ".mjs", ".svelte"],
    },
  },
};
