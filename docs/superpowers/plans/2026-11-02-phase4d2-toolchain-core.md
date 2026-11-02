# フェーズ 4d-2: ツールチェーンと web/core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `web/` のツールチェーン（pnpm・Vite・Biome・svelte-check・Vitest・Playwright）と CI・`make web`、フレームワークに依存しない `web/core`（API クライアント、SSE と接続、キャッシュと保存の制御、CLI の文字列・Store とスキーマ・表示の整形）を作る。画面は最小の App だけ。

**Architecture:** フェーズ 4d の4回のうちの1回。決定事項・spec にない細部・ファイル構成・共通の型は概要 `docs/superpowers/plans/2026-11-02-phase4d-web.md` にあり、各 Task はそこを前提にする（細部の番号はそこを指す）。Task の番号は 4d 全体の通し番号。

**Tech Stack:** 概要のとおり。

**Spec:** `docs/superpowers/specs/2026-11-01-phase4-gui-design.md`（「4d・4e フロントエンド」「画面と範囲」「4c サーバー」）と、概要 `docs/superpowers/plans/2026-11-02-phase4d-web.md`。

**この回の Task:** Task 2〜6

**着手の前に:** 4d-1 がマージ済みであること（`web/core/api/schema.d.ts` を 4d-1 の `api/openapi.yaml` から作るため）。Task 2 の `Makefile`・`.github/workflows/ci.yaml`・`mise.toml` の変更は、PR #28（CI の短縮。Windows をやめ、Go のテストを `test-shard` の 12 ジョブとまとめの `test` にし、`make cover`・`make clean-cache` と `mise.toml` の `[env]` を足した）のあとの形に合わせてある（2026-11-02）。web のジョブは Go のジョブと並列のまま。

## Global Constraints

- 画面の文言はすべて日本語。コードの識別子・コメントは英語（既存のコードと同じ）
- コンポーネントは色・寸法の 16 進数や px を直接書かず、`web/app/styles/tokens.css` の CSS 変数を使う（Foundations: 「色はトークン名で実装し、画面に 16 進数を直書きしない」）
- ステータスは色だけでなく形で区別する（Foundations のステータスの形）
- 行とカードは ID だけを出し、ULID は ID にカーソルを合わせたときに出してコピーできるようにする。タスク詳細では ULID を常に出す。「キー未確定」・短縮 ID・「次の同期で確定」は作らない
- フォントは `@fontsource` で同梱し、外部（Google Fonts など）に通信しない
- ダークモードは作らない
- キーボード: ⌘K コマンドパレット、C タスクを作成、⇧⌘S 今すぐ同期、Enter 下に追加（ツリー）、Tab / ⇧Tab 字下げ・戻す、⌥↑↓ 並べ替え、Space 完了、S ステータス、K 種類、⌘↵ 詳細を開く・作成。Windows と Linux では ⌘ を Ctrl にする
- 保存は入力が 2 秒止まったとき、またはフォーカスが外れたときに `PATCH`。1操作1コミット
- トーストは左下に 6 秒、取り消し付きは 10 秒。取り消しにくい操作だけ確認ダイアログ。undo は確認しない
- ダイアログとフィルタ欄の足元には同じ操作の `hh` コマンドを出し、コピーできるようにする
- `svelte-check` は警告ゼロ、`biome ci` は指摘ゼロ
- spec・設計と違う挙動にしたくなったら、実装を止めて報告する（自分で決めない）
- Go のテストは `-race` を含めて通す。Go のジョブは `web/dist` なし（`.gitkeep` だけ）で通る

---

### Task 2: ツールチェーンと CI

細部 1〜7 のとおり。ここで作る設定は、すべて scratch の複製の上で `pnpm install`・`pnpm gen`・`pnpm lint`・`pnpm check`・`pnpm test`・`pnpm build`・`playwright test` を実行して動くことを確かめてある。確かめて plan の想定から変わった点は、各ステップの注に書いた（`@types/node` を足したこと、`svelte-check` が `allowJs` を要ること、Biome が `html.formatter.enabled` を要すること、pnpm 12 が `pnpm-workspace.yaml` を作ること、Vite の `emptyOutDir` が `dist/.gitkeep` を消すこと）。

Task 2 の範囲に **e2e の CI ジョブは入れない**。ジョブは `make e2e` を走らせ、spec が1つもないと Playwright は失敗する。ジョブは Task 16a で、最初の spec と一緒に足す（`make e2e` のターゲットだけここで作る）。

**Files:**
- Create: `web/package.json`、`web/pnpm-lock.yaml`（`pnpm install` が作る）、`web/pnpm-workspace.yaml`（同。pnpm 12 が作る）、`web/biome.json`、`web/tsconfig.json`、`web/vite.config.ts`、`web/vitest.config.ts`、`web/vitest.setup.ts`、`web/svelte.config.js`、`web/playwright.config.ts`、`web/index.html`、`web/app/main.ts`、`web/app/App.svelte`、`web/core/api/schema.d.ts`（`pnpm gen` の生成物）
- Test: `web/app/App.test.ts`
- Modify: `mise.toml`、`Makefile`、`.gitignore`、`.goreleaser.yaml`、`.github/workflows/ci.yaml`、`.github/workflows/release.yaml`、`renovate.json`

**Interfaces:**
- Consumes: Task 1 が作り直した `api/openapi.yaml`（`schema.d.ts` に `body`・`repos` が入る）、4c の `web/embed.go`（`//go:embed all:dist`）と `internal/server/static.go`（`web/dist/index.html` があれば配り、知らないパスは `index.html` を返す）
- Produces: `web/` の1パッケージ（scripts: `dev`・`build`・`gen`・`check`・`lint`・`fix`・`test`・`e2e`）、`make web`・`make web-check`・`make e2e`・`make build`（先に `make web`）・`make check`（`web-check` を含む）、CI の `web` ジョブ、`web/core/api/schema.d.ts`（Task 3 以降が `import type { paths, components } from "./schema"` で使う。Task 3 でもう一度 `pnpm gen` しても差分は出ない）、最小の `app/App.svelte` と `app/App.test.ts`（Task 7 が `App.svelte` を置き換え、`App.test.ts` をその画面のテストに書き換える）。次の約束をほかのタスクに課す: **コンポーネントや core のファイルを足したら、コミットの前に `cd web && pnpm fix` を実行する**（Biome は `.svelte` の `<script>` を2スペースでインデントし、文末にセミコロンを付ける。`pnpm lint` は整形の差も指摘として数える）

- [ ] **Step 1: ツールの版と無視するファイル**

`mise.toml` の `[tools]` に2行足す（`[env]` の `GOFLAGS` はそのまま）:
```toml
[tools]
"github:gotestyourself/gotestsum" = "1.13.0"
node = "24.22.1"
pnpm = "12.16.1"
```

`.gitignore` の末尾の `### build ###` の節に、`web` の節を足す。

```gitignore
### build ###
/cmd/hedgehug/hedgehug

### web ###
node_modules/
/web/test-results/
/web/playwright-report/
```

Run: `mise install`
Expected: Node 24.22.1 と pnpm 12.16.1 が入る。`node --version` が `v24.22.1`、`pnpm --version` が `12.16.1`

- [ ] **Step 2: package.json と依存の導入**

細部 1 の版の一覧に、`@codemirror/autocomplete` 6.20.4（`@`・`#` の補完。細部 45）と `@types/node` を足す（`vite.config.ts`・`playwright.config.ts`・`e2e/` が Node の API を使い、`svelte-check` が型を要するため。Node 24 に合わせて 24.19.2）。ほかは細部 1 のまま。`scripts.fix`（`biome check --write .`）も足す。

`web/package.json`:
```json
{
  "name": "hedgehug-web",
  "version": "0.0.0",
  "private": true,
  "type": "module",
  "packageManager": "pnpm@12.16.1",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "gen": "openapi-typescript ../api/openapi.yaml -o core/api/schema.d.ts",
    "check": "svelte-check --tsconfig ./tsconfig.json --fail-on-warnings",
    "lint": "biome ci .",
    "fix": "biome check --write .",
    "test": "vitest run",
    "e2e": "playwright test"
  },
  "dependencies": {
    "@atlaskit/pragmatic-drag-and-drop": "4.0.0",
    "@atlaskit/pragmatic-drag-and-drop-auto-scroll": "3.2.1",
    "@atlaskit/pragmatic-drag-and-drop-hitbox": "3.0.0",
    "@codemirror/autocomplete": "6.20.4",
    "@codemirror/lang-markdown": "6.5.2",
    "@codemirror/state": "6.7.6",
    "@codemirror/view": "6.43.15",
    "@fontsource/ibm-plex-sans-jp": "5.3.1",
    "@fontsource/jetbrains-mono": "5.3.0",
    "@tanstack/svelte-virtual": "3.13.51",
    "codemirror": "6.0.2",
    "openapi-fetch": "0.17.0"
  },
  "devDependencies": {
    "@biomejs/biome": "2.6.4",
    "@playwright/test": "1.64.1",
    "@sveltejs/vite-plugin-svelte": "7.3.1",
    "@testing-library/jest-dom": "7.0.1",
    "@testing-library/svelte": "5.4.2",
    "@types/node": "24.19.2",
    "jsdom": "30.1.3",
    "openapi-typescript": "7.13.0",
    "svelte": "5.57.3",
    "svelte-check": "4.7.7",
    "typescript": "5.9.3",
    "vite": "8.3.5",
    "vitest": "5.0.5"
  }
}
```

Run: `cd web && pnpm install`
Expected: `web/pnpm-lock.yaml` と `web/pnpm-workspace.yaml` ができる。`pnpm-workspace.yaml` は pnpm 12 の設定ファイルで、公開から日の浅い版（`@tanstack/svelte-virtual@3.13.51` など）を `minimumReleaseAgeExclude` に書く。`packages:` を持たないので workspace の宣言ではない。**そのままコミットする**（`--frozen-lockfile` の導入に要る）。中身は実行した日で変わりうる

- [ ] **Step 3: 設定ファイル**

`web/tsconfig.json`（`allowJs` は `svelte-check` が `import App from "./App.svelte"` を解くのに要る。外すと `Could not find a declaration file for module './App.svelte'` になる）:
```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "lib": ["ES2023", "DOM", "DOM.Iterable"],
    "types": ["node", "svelte", "vite/client", "@testing-library/jest-dom"],
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "skipLibCheck": true,
    "allowJs": true,
    "noEmit": true
  },
  "include": ["app/**/*.ts", "app/**/*.svelte", "core/**/*.ts", "e2e/**/*.ts", "*.ts"]
}
```

`web/biome.json`（決定事項 1 への補足: `html.experimentalFullSupportEnabled` だけでは `.svelte` の lint は効くが整形は効かない。`html.formatter.enabled` を足して整形も効かせ、`indentScriptAndStyle` で `<script>` と `<style>` の中身をインデントする。2.6 では `rules.recommended` が非推奨で、`rules.preset` に変わった。`pnpm-workspace.yaml` は pnpm が書くので対象から外す）:
```json
{
  "$schema": "https://biomejs.dev/schemas/2.6.4/schema.json",
  "files": {
    "includes": [
      "**",
      "!dist",
      "!test-results",
      "!playwright-report",
      "!core/api/schema.d.ts",
      "!pnpm-lock.yaml",
      "!pnpm-workspace.yaml"
    ]
  },
  "formatter": {
    "enabled": true,
    "indentStyle": "space",
    "indentWidth": 2,
    "lineWidth": 100
  },
  "javascript": {
    "formatter": {
      "quoteStyle": "double"
    }
  },
  "html": {
    "experimentalFullSupportEnabled": true,
    "formatter": {
      "enabled": true,
      "indentScriptAndStyle": true
    }
  },
  "linter": {
    "enabled": true,
    "rules": {
      "preset": "recommended"
    }
  },
  "assist": {
    "enabled": true,
    "actions": {
      "source": {
        "organizeImports": "on"
      }
    }
  }
}
```

`web/svelte.config.js`:
```js
import { vitePreprocess } from "@sveltejs/vite-plugin-svelte";

export default {
  preprocess: vitePreprocess(),
};
```

`web/vite.config.ts`（`codeSplitting` は CodeMirror を別のチャンクに分け、`vite build` の「500 kB 超」の警告を出さない。`pnpm build` の出力でどのチャンクも 500 kB 未満であることを Task 11 のあとに確かめる。Task 2 の時点では CodeMirror がまだ入らないので、チャンクは1つ。**分け方に注意**: グループの `includeDependenciesRecursively` を既定（真）のままにすると、依存の `@codemirror/view` などが別のチャンクに引き込まれて循環し、実行時に `c is not a constructor` で画面が真っ白になる。ここでは、土台（`@lezer/*`、`@codemirror/state`・`view`・`language`、`crelt`・`style-mod`・`w3c-keyname`）と、残り（Markdown の言語・補完・コマンドなど。土台だけに依存する）の2つに分け、どちらも依存を引き込まない。この真っ白は単体テストでは見えず、Task 16a の E2E で見つかった。`keepGitkeep`: Vite は `emptyOutDir` で `dist/` を空にするので、`dist/.gitkeep` が消えて git に削除として見える。`go build` は `dist` に1つもファイルがないと埋め込みに失敗するため、ビルドのあとに作り直す。`pnpm dev` は `HEDGEHUG_UI` があれば `/api` をそのサーバーへ中継し、開くブラウザのパスにその `#token=` を付ける）:
```ts
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

// In development, `pnpm dev` serves the page and proxies /api to the server
// that `hh ui --no-open` started. HEDGEHUG_UI is the URL it printed
// (http://127.0.0.1:<port>/#token=<token>): the proxy goes to its origin, and
// the browser the dev server opens gets its token in the hash.
const ui = process.env.HEDGEHUG_UI ? new URL(process.env.HEDGEHUG_UI) : null;

export default defineConfig({
  plugins: [svelte(), keepGitkeep()],
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
```

`web/vitest.config.ts` と `web/vitest.setup.ts`:
```ts
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { svelteTesting } from "@testing-library/svelte/vite";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [svelte(), svelteTesting()],
  test: {
    environment: "jsdom",
    include: ["core/**/*.test.ts", "app/**/*.test.ts"],
    setupFiles: ["./vitest.setup.ts"],
  },
});
```

```ts
import "@testing-library/jest-dom/vitest";
```

`web/playwright.config.ts`（`webServer` は持たない。テストごとに `hh ui` を起動する。Task 16a の `e2e/harness.ts`）:
```ts
import { defineConfig, devices } from "@playwright/test";

// Each test starts its own hh ui against its own store (e2e/harness.ts), so
// there is no webServer here and the tests can run in parallel.
export default defineConfig({
  testDir: "e2e",
  testMatch: "**/*.spec.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: process.env.CI ? [["github"], ["list"]] : "list",
  use: {
    trace: "retain-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } },
    },
  ],
});
```

`web/index.html` と `web/app/main.ts`:
```html
<!doctype html>
<html lang="ja">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Hedgehug</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/app/main.ts"></script>
  </body>
</html>
```

```ts
import { mount } from "svelte";
import App from "./App.svelte";

const target = document.getElementById("app");
if (!target) throw new Error("#app is missing from index.html");

mount(App, { target });
```

- [ ] **Step 4: 失敗するテストを書く**

`web/app/App.test.ts`:
```ts
import { render, screen } from "@testing-library/svelte";
import { expect, test } from "vitest";
import App from "./App.svelte";

test("App renders the product name", () => {
  render(App);
  expect(screen.getByRole("heading", { name: "Hedgehug" })).toBeInTheDocument();
});
```

- [ ] **Step 5: 失敗を確かめる**

Run: `cd web && pnpm test`
Expected: FAIL（`Failed to resolve import "./App.svelte" from "app/App.test.ts". Does the file exist?`）

- [ ] **Step 6: 実装する**

`web/app/App.svelte`:
```svelte
<main>
  <h1>Hedgehug</h1>
</main>
```

- [ ] **Step 7: 通ることを確かめる**

Run: `cd web && pnpm test`
Expected: PASS（1 test）

- [ ] **Step 8: API の型を作る**

Run: `cd web && pnpm gen`
Expected: `web/core/api/schema.d.ts` ができる。`components["schemas"]` に `TaskView`・`ShowView`・`StoreItem`・`SchemaView` などが入り、`paths["/api/tasks"]["post"]` のリクエストに `body`・`repos` がある（Task 1 のあと）。`syncStatus` に `push_at`、`paths["/api/people"]`、`paths["/api/tasks/{id}/history/{commit}"]` もある

- [ ] **Step 9: 整形・検査・ビルド**

Run: `cd web && pnpm fix && pnpm lint && pnpm check && pnpm test && pnpm build && ls -A dist`
Expected: `biome ci` が `No fixes applied.`、`svelte-check` が `0 ERRORS 0 WARNINGS`、テスト PASS、`dist` に `.gitkeep`・`assets`・`index.html`。続けて、リポジトリの直下で `git status --short web/dist` が何も出さない（`.gitkeep` が戻っている。`assets` と `index.html` は `.gitignore` で無視される）

- [ ] **Step 10: Makefile**

`.PHONY` の行、`build`、`check` を変え、`web`・`web-check`・`e2e` を足す。ほかのターゲットは変えない。`web-check` の `git diff` は、`pnpm gen` が作った `schema.d.ts` が git のものと違えば落ちる（`api/openapi.yaml` が変わったのに型を作り直し忘れた、を CI で見つける）。`e2e` は Task 16a・16b の spec を走らせる（ここでは `make e2e` を実行しない）。

```make
.PHONY: generate build web web-check e2e fmt fmt-check lint tidy-check test cover clean-cache check release-check
```

```make
# Build the GUI into web/dist, which the binary embeds.
web:
	cd web && pnpm install --frozen-lockfile && pnpm build

# Build the binary into bin/, with the GUI.
build: web
	go build -o bin/hedgehug ./cmd/hedgehug

# Check the GUI: the API types are up to date, biome, svelte-check and the unit tests
# (mirrors the CI web job).
web-check:
	cd web && pnpm install --frozen-lockfile
	cd web && pnpm gen
	git diff --exit-code -- web/core/api/schema.d.ts
	cd web && pnpm lint && pnpm check && pnpm test

# Run the browser tests against bin/hedgehug (mirrors the CI e2e job).
e2e: build
	cd web && pnpm exec playwright install chromium
	cd web && pnpm e2e
```

```make
# Run everything the CI checks, locally.
check: tidy-check lint fmt-check test web-check
```

- [ ] **Step 11: make で通す**

Run: `make web && make web-check`
Expected: どちらも成功。`git status --short` に `web/dist` の変更が出ない

- [ ] **Step 12: バイナリが GUI を配ることを手順で確かめる**

Go のテストではなく手で確かめる（`web/dist` は git にないので、Go のテストは `dist` なしで通ることを前提にしている）。

```bash
make build
export HEDGEHUG_HOME="$(mktemp -d)"
bin/hedgehug store init demo --prefix DM
bin/hedgehug ui --no-open &        # 1行目: Hedgehug UI: http://127.0.0.1:<port>/#token=<token>
curl -s http://127.0.0.1:<port>/ | head -5                  # index.html が返る（<title>Hedgehug</title>）
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:<port>/workspaces/x/y   # 200（知らないパスも index.html）
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:<port>/api/nope          # 404（API は index.html にしない）
kill -INT %1
```
Expected: 上の4つのとおり。`<port>` は1行目の URL のもの

- [ ] **Step 13: goreleaser と mise-action と CI**

`.goreleaser.yaml` の `before.hooks` に `make web` を足す（リリースのバイナリが GUI を埋め込むため。`web/dist` と `node_modules` は `.gitignore` なので、goreleaser の「git が dirty」の検査には引っかからない）。

```yaml
before:
  hooks:
    - go mod tidy
    # The binaries embed web/dist. It is not in git, so build it here.
    - make web
```

`.github/workflows/release.yaml` の `setup-go` と `goreleaser-action` の間に足す。SHA は ci.yaml の `jdx/mise-action` と同じ。

```yaml
      # .goreleaser.yaml の before.hooks が make web で GUI を作る。Node と pnpm が要る。
      - uses: jdx/mise-action@82a612c5ab8bcda7cc5db6b7e81a8e7584c3dc6e # v5.2.3
        with:
          install_args: node pnpm
```

`.github/workflows/ci.yaml`。`test-shard` と `test-without-git` の `jdx/mise-action` を2か所とも次に変える（12 の shard のジョブそれぞれに Node と pnpm を入れる時間を足さないため。`mise which gotestsum` はこのままで動く）。

```yaml
      # mise.toml に固定したツールのうち gotestsum だけを入れる。Node と pnpm は GUI のジョブ用で、
      # ここで入れると shard のジョブがそれぞれ遅くなる。Renovate の mise manager がバージョンを追う。
      - uses: jdx/mise-action@82a612c5ab8bcda7cc5db6b7e81a8e7584c3dc6e # v5.2.3
        with:
          install_args: gotestsum
```

同じファイルの末尾（`test-without-git` の後）に `web` ジョブを足す。必須チェックはまとめのジョブ `test` だけ（ruleset は Terraform で管理）なので、`web` は必須チェックに入れない。pnpm のキャッシュは使わない（既存の `cache: false` の方針。`actions/setup-node` も使わず、新しいアクションを増やさない）。

```yaml

  # GUI（web/）。Node は1つの OS で確かめれば足りるので ubuntu だけで回す。
  web:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    permissions:
      contents: read
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      # mise.toml に固定した Node と pnpm を入れる。
      - uses: jdx/mise-action@82a612c5ab8bcda7cc5db6b7e81a8e7584c3dc6e # v5.2.3
        with:
          install_args: node pnpm
      - name: Install dependencies
        run: pnpm install --frozen-lockfile
      # core/api/schema.d.ts は api/openapi.yaml から作る。古ければここで気付く。
      - name: Verify the API types are up to date
        run: |
          pnpm gen
          git diff --exit-code -- core/api/schema.d.ts
      - name: Lint
        run: pnpm lint
      - name: Type check
        run: pnpm check
      - name: Test
        run: pnpm test
      - name: Build
        run: pnpm build
```

`renovate.json`（npm と mise の manager は共有設定 `bright-room/renovate-config` で有効。`mise.toml` の `node`・`pnpm` と `package.json` の版・`packageManager` は Renovate の既定で追われるので、足すのは、`pnpm` が `mise.toml` と `packageManager` の2か所にあるため、同じ PR で上げるグループだけ。Renovate はここで実行して確かめられない）:
```json
{
  "extends": [
    "github>bright-room/renovate-config",
    "github>bright-room/renovate-config:go"
  ],
  "packageRules": [
    {
      "description": "pnpm is pinned in mise.toml and in web/package.json (packageManager); update both in one PR",
      "matchDepNames": ["pnpm"],
      "groupName": "pnpm"
    }
  ]
}
```

Run: `actionlint .github/workflows/ci.yaml .github/workflows/release.yaml && goreleaser check`
Expected: どちらもエラーなし

- [ ] **Step 14: コミット**
```bash
git add mise.toml .gitignore renovate.json web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml web/biome.json web/tsconfig.json web/vite.config.ts web/vitest.config.ts web/vitest.setup.ts web/svelte.config.js web/playwright.config.ts web/index.html web/app web/core/api/schema.d.ts
git commit -m "build(web): Svelte・Vite・Biome・Vitest・Playwright の土台を置く"
git add Makefile .goreleaser.yaml .github/workflows/ci.yaml .github/workflows/release.yaml
git commit -m "ci: GUI のビルドと検査を make と CI とリリースに足す"
```

---

## web/core: API・接続・キャッシュ・保存・表示の整形（Task 3〜6）

この節の4つのタスクは、Svelte に依存しない TypeScript だけを書く。`subscribe` は素の関数のコールバックで、描画の側（Task 7 の `useSubscribe`）が runes につなぐ。各タスクはテストを先に書き、通ってからコミットする。

**この節で注意すること**

- API のパスは `api/openapi.yaml` のとおり `/api` 付き（`/api/tasks`、`/api/tasks/{id}`、`/api/stores`、`/api/workspaces`、`/api/schema`、`/api/presence`、`/api/events`）。plan 前半の `GET /tasks` などは `/api` を略した書き方
- `core/api/schema.d.ts` は Task 2 の `pnpm gen` で作ってコミット済み（Task 1 の `SyncStatus.push_at`・`GET /api/people`・`GET /api/tasks/{id}/history/{commit}` が入った openapi.yaml から）。ない場合や、`push_at` などが型にない場合は `cd web && pnpm gen` を先に実行する
- テストは Vitest（jsdom）。`web/vitest.config.ts` の `include` が `core/**/*.test.ts` を拾うこと（Task 2 で設定済み）
- Biome の整形結果（行幅 100、ダブルクォート、import の並べ替え）で書いてある。コードをそのまま貼る

---

### Task 3: API クライアント・トークン・エラーの文言

**Files:**
- Create: `web/core/api/client.ts`
- Create: `web/core/token.ts`
- Create: `web/core/messages.ts`
- Test: `web/core/api/client.test.ts`、`web/core/token.test.ts`、`web/core/messages.test.ts`

**Interfaces:**
- Consumes: `web/core/api/schema.d.ts`（Task 2 の `pnpm gen`）、openapi-fetch
- Produces（plan 前半の「共通の型と関数」のとおり）: `Schemas`・`TaskView`・`ShowView`・`Api`・`ApiError`・`createApi`・`unwrap`、`takeToken`、`errorMessage`
- 前半に足したもの: `toApiError(status, error): ApiError`（`unwrap` が使う。problem+json でない本文は `http.<status>` の型にする）、`getToken(storage): string | null`（`takeToken` の後の読み出し）
- `createApi` の `baseUrl` は `location.origin`（jsdom のテストでも絶対 URL になる）。問題の本文を `ApiError` にするのは `unwrap`（呼び出し側の形を plan 前半の「`unwrap` で ApiError を投げる」にそろえるため）。ミドルウェアは Authorization の付与と、401 `auth.token` での `onUnauthorized` だけを受け持つ
- `messages.ts` は ops と server の Code すべての文言を持つ（`grep -rhoE 'newErr\(Kind[A-Za-z]+, *"[^"]+"' internal/ops internal/server` と `grep -rhoE 'newProblem\([^,]+, *"[^"]+"' internal/server` と `classify` の結果と突き合わせて確認済み）。args のキーは各 `newErr` の呼び出しを読んで合わせてある（例: `task.not_found` は `input`、`store.not_found` は `name`・`candidates`、`comment.not_author` は `author`、`purge.unconfirmed` は `targets`）。同じ Code が ops とサーバーで意味を分ける `auth.token` は `args.account` の有無で分ける

- [ ] **Step 1: 失敗するテストを書く**

`web/core/api/client.test.ts`

```ts
import { describe, expect, it, vi } from "vitest";
import { ApiError, createApi, toApiError, unwrap } from "./client";

function problem(status: number, body: Record<string, unknown>): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/problem+json" },
  });
}

describe("createApi", () => {
  it("sends the token as a bearer token", async () => {
    const f = vi.fn(async (_req: Request) => Response.json([]));
    const api = createApi({ token: () => "tok", fetch: f as unknown as typeof fetch });
    await api.GET("/api/stores");
    const req = f.mock.calls[0]?.[0] as unknown as Request;
    expect(req.headers.get("Authorization")).toBe("Bearer tok");
    expect(new URL(req.url).pathname).toBe("/api/stores");
  });

  it("sends no Authorization header without a token", async () => {
    const f = vi.fn(async (_req: Request) => Response.json([]));
    const api = createApi({ token: () => null, fetch: f as unknown as typeof fetch });
    await api.GET("/api/stores");
    const req = f.mock.calls[0]?.[0] as unknown as Request;
    expect(req.headers.has("Authorization")).toBe(false);
  });

  it("calls onUnauthorized for 401 auth.token only", async () => {
    const onUnauthorized = vi.fn();
    let res = problem(401, { type: "auth.token", title: "Unauthorized", status: 401 });
    const api = createApi({
      token: () => "bad",
      fetch: (async () => res) as unknown as typeof fetch,
      onUnauthorized,
    });
    await api.GET("/api/stores");
    expect(onUnauthorized).toHaveBeenCalledTimes(1);

    res = problem(404, { type: "task.not_found", title: "Not Found", status: 404 });
    await api.GET("/api/stores");
    expect(onUnauthorized).toHaveBeenCalledTimes(1);
  });
});

describe("unwrap", () => {
  it("returns the data", async () => {
    const api = createApi({
      token: () => "t",
      fetch: (async () => Response.json([{ name: "a" }])) as unknown as typeof fetch,
    });
    const stores = await unwrap(api.GET("/api/stores"));
    expect(stores).toEqual([{ name: "a" }]);
  });

  it("throws the problem as an ApiError", async () => {
    const body = {
      type: "task.not_found",
      title: "Not Found",
      status: 404,
      detail: 'no task "X-1"',
      args: { input: "X-1" },
    };
    const api = createApi({
      token: () => "t",
      fetch: (async () => problem(404, body)) as unknown as typeof fetch,
    });
    const err = await unwrap(api.GET("/api/stores")).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    const e = err as ApiError;
    expect(e.status).toBe(404);
    expect(e.type).toBe("task.not_found");
    expect(e.detail).toBe('no task "X-1"');
    expect(e.args).toEqual({ input: "X-1" });
    expect(e.body).toEqual(body);
  });

  it("types an error that is not a problem by its status", () => {
    const e = toApiError(502, "<html>Bad Gateway</html>");
    expect(e.type).toBe("http.502");
    expect(e.detail).toBe("<html>Bad Gateway</html>");
    expect(e.args).toEqual({});
  });
});
```

`web/core/token.test.ts`

```ts
import { describe, expect, it, vi } from "vitest";
import { getToken, takeToken } from "./token";

function loc(hash: string): Location {
  return { hash, pathname: "/projects/X", search: "?a=1" } as Location;
}

describe("takeToken", () => {
  it("moves #token= into the storage and clears the hash", () => {
    sessionStorage.clear();
    const history = { replaceState: vi.fn() } as unknown as History;
    expect(takeToken(loc("#token=abc123"), sessionStorage, history)).toBe("abc123");
    expect(sessionStorage.getItem("hedgehug.token")).toBe("abc123");
    expect(history.replaceState).toHaveBeenCalledWith(null, "", "/projects/X?a=1");
  });

  it("returns the stored token when the URL has none", () => {
    sessionStorage.setItem("hedgehug.token", "kept");
    const history = { replaceState: vi.fn() } as unknown as History;
    expect(takeToken(loc(""), sessionStorage, history)).toBe("kept");
    expect(history.replaceState).not.toHaveBeenCalled();
  });

  it("returns null when there is no token anywhere", () => {
    sessionStorage.clear();
    const history = { replaceState: vi.fn() } as unknown as History;
    expect(takeToken(loc("#foo=1"), sessionStorage, history)).toBeNull();
    expect(getToken(sessionStorage)).toBeNull();
  });
});
```

`web/core/messages.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { ApiError } from "./api/client";
import { errorMessage } from "./messages";

const err = (
  type: string,
  args: Record<string, unknown> = {},
  detail = "english",
  body?: unknown,
) => new ApiError({ status: 400, type, detail, args, body });

describe("errorMessage", () => {
  it("puts the args into the words", () => {
    expect(errorMessage(err("task.not_found", { input: "REL-9" }))).toBe(
      "REL-9 というタスクは見つかりません",
    );
    expect(errorMessage(err("comment.not_author", { author: "alice" }))).toBe(
      "alice が書いたコメントは変更できません",
    );
    expect(errorMessage(err("assign.bad_value", { key: "points", value: "x" }, ""))).toBe(
      "points に「x」は指定できません",
    );
    expect(errorMessage(err("store.not_found", { name: "foo", candidates: ["a", "b"] }))).toBe(
      "foo という Store はありません（登録済み: a、b）",
    );
  });

  it("words restore_at.not_found for a restore and for reading a point in time", () => {
    expect(errorMessage(err("restore_at.not_found", { at: "abc123" }))).toBe(
      "abc123 の時点にはこのタスクがありませんでした",
    );
    expect(errorMessage(err("restore_at.not_found"))).toBe(
      "その時点にはこのタスクがありませんでした",
    );
  });

  it("tells the two auth.token apart", () => {
    expect(errorMessage(err("auth.token", { account: "work" }))).toContain("work");
    expect(errorMessage(err("auth.token"))).toBe(
      "この画面は hh ui が開いた URL からだけ使えます。端末で hh ui を実行してください",
    );
  });

  it("counts the targets of purge.unconfirmed", () => {
    const targets = [{ ulid: "a" }, { ulid: "b" }];
    expect(errorMessage(err("purge.unconfirmed", { targets }))).toContain("2 件");
  });

  it("lists what a schema violation broke", () => {
    const m = errorMessage(
      err("schema.violation", {
        violations: [
          { code: "placement", field: "parent", message: "a epic cannot be placed under task" },
        ],
      }),
    );
    expect(m).toContain("この親には置けません（parent）");
  });

  it("joins the details of request.invalid", () => {
    const body = { errors: [{ message: "expected string" }, { message: "too short" }] };
    expect(errorMessage(err("request.invalid", {}, "validation failed", body))).toBe(
      "入力が正しくありません: expected string / too short",
    );
  });

  it("shows the type and the detail of an unknown type", () => {
    expect(errorMessage(err("weird.thing", {}, "boom"))).toBe(
      "操作に失敗しました（weird.thing）: boom",
    );
  });

  it("words an error that is not an ApiError", () => {
    expect(errorMessage(new TypeError("Failed to fetch"))).toBe(
      "操作に失敗しました: Failed to fetch",
    );
    expect(errorMessage("x")).toBe("操作に失敗しました: x");
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm vitest run core/api/client.test.ts core/token.test.ts core/messages.test.ts`

Expected: FAIL（モジュールが見つからない）

- [ ] **Step 3: 実装する**

`web/core/api/client.ts`

```ts
import createClient, { type Middleware } from "openapi-fetch";
import type { components, paths } from "./schema";

export type Schemas = components["schemas"];
export type TaskView = Schemas["TaskView"];
export type ShowView = Schemas["ShowView"];

export type Api = ReturnType<typeof createApi>;

// ApiError is a problem+json response of the server. type is the Code of the
// error (task.not_found...); args are its arguments; body is the whole
// problem, for what args do not hold (purge.unconfirmed lists its targets).
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

// toApiError makes the ApiError of a failed response. error is what
// openapi-fetch read from the body: the parsed problem, or the text when the
// body is not JSON (a proxy's page).
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

// createApi is the typed client of the server. It sends the token as a
// bearer token, and calls onUnauthorized when the server refuses it
// (401 auth.token).
export function createApi(opts: {
  token: () => string | null;
  fetch?: typeof fetch;
  onUnauthorized?: () => void;
}) {
  const client = createClient<paths>({
    baseUrl: typeof location !== "undefined" ? location.origin : "",
    fetch: opts.fetch,
  });
  const auth: Middleware = {
    onRequest({ request }) {
      const t = opts.token();
      if (t) request.headers.set("Authorization", `Bearer ${t}`);
      return request;
    },
    async onResponse({ response }) {
      if (response.status === 401 && opts.onUnauthorized) {
        const p: unknown = await response
          .clone()
          .json()
          .catch(() => null);
        if (isRecord(p) && p.type === "auth.token") opts.onUnauthorized();
      }
      return undefined;
    },
  };
  client.use(auth);
  return client;
}

// unwrap returns the data of a call, or throws the ApiError of its problem.
export async function unwrap<T>(
  p: Promise<{ data?: T; error?: unknown; response: Response }>,
): Promise<T> {
  const r = await p;
  if (r.error !== undefined || !r.response.ok) throw toApiError(r.response.status, r.error);
  return r.data as T;
}
```

`web/core/token.ts`

```ts
const KEY = "hedgehug.token";

// takeToken moves the token of the URL (#token=<t>, which hh ui prints) into
// sessionStorage and removes it from the address bar. It returns the token
// the tab holds, or null when there is none.
export function takeToken(loc: Location, storage: Storage, history: History): string | null {
  const t = new URLSearchParams(loc.hash.replace(/^#/, "")).get("token");
  if (!t) return getToken(storage);
  try {
    storage.setItem(KEY, t);
  } catch {
    // A storage that refuses writes still lets this page load use the token.
  }
  history.replaceState(null, "", loc.pathname + loc.search);
  return t;
}

// getToken is the token the tab holds.
export function getToken(storage: Storage): string | null {
  try {
    return storage.getItem(KEY);
  } catch {
    return null;
  }
}
```

`web/core/messages.ts`

```ts
import { ApiError } from "./api/client";

type Args = Record<string, unknown>;
type Make = (a: Args, e: ApiError) => string;

const str = (a: Args, k: string): string => {
  const v = a[k];
  return v === undefined || v === null ? "" : String(v);
};
const list = (a: Args, k: string): string => {
  const v = a[k];
  return Array.isArray(v) ? v.map(String).join("、") : "";
};
// withDetail adds the English message of the server, for errors whose cause
// only the message tells.
const withDetail = (text: string, e: ApiError): string =>
  e.detail ? `${text}: ${e.detail}` : text;

const violationLabels: Record<string, string> = {
  unknown_kind: "未定義の種類です",
  unknown_status: "未定義のステータスです",
  missing_title: "タイトルがありません",
  bad_value: "値が正しくありません",
  placement: "この親には置けません",
  transition: "このステータスには移れません",
  required: "必須の項目が空です",
  missing_ref: "参照先がありません",
  cycle: "循環しています",
};

function violations(a: Args): string {
  const vs = a.violations;
  if (!Array.isArray(vs) || vs.length === 0) return "";
  return vs
    .map((v) => {
      const x = (v ?? {}) as Record<string, unknown>;
      const label = violationLabels[String(x.code)] ?? String(x.code ?? "");
      const field = x.field ? `（${String(x.field)}）` : "";
      return `${label}${field}: ${String(x.message ?? "")}`;
    })
    .join(" / ");
}

// messages has the words for every Code of the ops errors and of the server.
const messages: Record<string, Make> = {
  // tasks and workspaces
  "task.not_found": (a) => `${str(a, "input")} というタスクは見つかりません`,
  "task.ambiguous": (a) =>
    `${str(a, "input")} に当てはまるタスクが複数あります。ULID で指定してください` +
    (list(a, "candidates") ? `（候補: ${list(a, "candidates")}）` : ""),
  not_found: (_, e) => withDetail("見つかりません", e),
  "task.gone": (a) => `${str(a, "id")} はもうストアにありません`,
  "task.deleted": (a) => `${str(a, "id")} は削除済みです。先に復元してください`,
  "task.parent_deleted": (a) => `親の ${str(a, "id")} が削除済みです。先に復元してください`,
  "task.already_deleted": (a) => `${str(a, "id")} はすでに削除されています`,
  "task.not_deleted": (a) => `${str(a, "id")} は削除されていません`,
  "task.workspace_mismatch": () => "親のワークスペースがこの Store にないか、指定と合いません",
  "workspace.not_found": (a) => `${str(a, "input")} というワークスペースは見つかりません`,
  "workspace.ambiguous": (a) => `${str(a, "input")} に当てはまるワークスペースが複数あります`,
  "workspace.taken": (_, e) => withDetail("そのワークスペースの名前はすでに使われています", e),
  "workspace.none": () =>
    "この Store にはワークスペースがありません。hh workspace add で作ってください",
  "workspace.choose": (a) =>
    `ワークスペースが複数あります。どれかを指定してください（${list(a, "names")}）`,
  "workspace.bad_name": (a) => `「${str(a, "value")}」はワークスペースの名前に使えません`,
  "workspace.bad_prefix": (a) => `「${str(a, "value")}」は ID の接頭辞に使えません`,
  "workspace.prefix_taken": (a) => `ID の接頭辞 ${str(a, "prefix")} はすでに使われています`,
  "workspace.not_empty": () =>
    "ワークスペースにまだタスクがあります。別のワークスペースへ移すか、完全に削除してください",
  "workspace.one_change": () =>
    "名前と接頭辞は同時に変えられません。どちらか一方を指定してください",

  // writes
  "patch.empty": () => "変更する項目がありません",
  "edit.changed": () => "編集している間にタスクが変わりました。開き直してください",
  "edit.invalid": (a, e) => withDetail(`${str(a, "field") || "入力"} を変更できません`, e),
  "move.invalid": (_, e) => withDetail("ここには移せません", e),
  "dep.invalid": (_, e) => withDetail("ブロックの指定を変えられません", e),
  "status.unknown": (a) => `「${str(a, "name")}」というステータスはありません`,
  "schema.no_default": (a) => {
    const what = str(a, "what");
    const name = what === "kind" ? "種類" : what === "status" ? "ステータス" : "完了のステータス";
    return `スキーマに既定の${name}がありません。指定してください`;
  },
  "schema.violation": (a, e) => {
    const t = violations(a);
    return t ? `スキーマに合わない変更です: ${t}` : withDetail("スキーマに合わない変更です", e);
  },
  "ref.cross_store": (a) =>
    `${str(a, "store")} の ${str(a, "id")} は参照できません（参照は同じ Store の中だけです）`,
  "use.not_project": (a) => `${str(a, "id")} は ${str(a, "kind")} で、プロジェクトではありません`,

  // assignments (hh set)
  "assign.form": (a) => `「${str(a, "input")}」は key=value の形ではありません`,
  "assign.readonly": (a) => `${str(a, "key")} は変更できません`,
  "assign.unknown_key": (a) =>
    `${str(a, "key")} はフィールドでも、スキーマのプロパティでもありません`,
  "assign.single": (a) => `${str(a, "key")} には値を1つだけ指定できます`,
  "assign.missing_value": (a) => `${str(a, "key")} の値がありません`,
  "assign.bad_value": (a, e) =>
    withDetail(`${str(a, "key")} に「${str(a, "value")}」は指定できません`, e),

  // deleting and restoring
  "purge.changed": () => "確認のあとにタスクが変わりました。もう一度確かめてください",
  "purge.child_not_deleted": (a) =>
    `${str(a, "id")} が削除されていないため、完全には削除できません`,
  "purge.unconfirmed": (a) => {
    const t = a.targets;
    const n = Array.isArray(t) ? t.length : 0;
    return n > 0
      ? `${n} 件のタスクを完全に削除します。対象を確かめてください`
      : "完全に削除するには確認が必要です";
  },
  "restore_at.bad_time": (a) => `「${str(a, "at")}」は時刻でもコミットでもありません`,
  "restore_at.not_found": (a) =>
    a.at
      ? `${str(a, "at")} の時点にはこのタスクがありませんでした`
      : "その時点にはこのタスクがありませんでした",
  "undo.nothing": () => "取り消せる操作がありません",
  "undo.blocked": () =>
    "ワークスペースにまだタスクがあるため取り消せません。別のワークスペースへ移すか、完全に削除してください",

  // comments
  "comment.not_found": (a) =>
    a.input ? `${str(a, "input")} というコメントは見つかりません` : "コメントが見つかりません",
  "comment.ambiguous": (a) => `${str(a, "input")} に当てはまるコメントが複数あります`,
  "comment.changed": () => "編集している間にコメントが変わりました。開き直してください",
  "comment.deleted": () => "このコメントは削除されています",
  "comment.no_author": () => "書いた人が分からないコメントは変更できません",
  "comment.not_author": (a) => `${str(a, "author")} が書いたコメントは変更できません`,
  "comment.empty": () => "コメントが空です",

  // stores and sync
  "store.read_only": (a) =>
    `${str(a, "store")} は読み取り専用です${a.reason ? `（${str(a, "reason")}）` : ""}`,
  "store.bad_name": (a) => `「${str(a, "value")}」は Store の名前に使えません`,
  "store.exists": (a) => `${str(a, "name")} という Store はすでにあります`,
  "store.not_found": (a) =>
    `${str(a, "name")} という Store はありません` +
    (list(a, "candidates") ? `（登録済み: ${list(a, "candidates")}）` : ""),
  "store.none": () => "Store がありません。hh store init で作ってください",
  "store.open": (a, e) => withDetail(`${str(a, "name")} を開けません`, e),
  "store.choose": (_, e) => withDetail("Store を選んでください", e),
  "store.conflict": (a) =>
    `${str(a, "named")} の ID ですが、指定された Store は ${str(a, "store")} です`,
  "store.locked": () => "別の hh が書き込み中です。少し待ってからやり直してください",
  "store.unpushed": (a) =>
    `${str(a, "name")} にはまだ push していない変更があります。先に同期してください`,
  "store.remote_required": () => "リモートの URL が必要です",
  "store.remote_not_empty": (_, e) => withDetail("リモートが空ではありません", e),
  "store.clone_failed": (a, e) => withDetail(`${str(a, "remote")} を取り込めません`, e),
  "store.not_hedgehug": (a) => `${str(a, "name")} は Hedgehug の Store ではありません`,
  "store.mount_failed": (a) => `追加できなかった Store があります（${list(a, "failed")}）`,
  "sync.dirty": () => "コミットしていない変更があるため同期できません",
  "sync.unrelated": () => "リモートの履歴がこの Store の履歴と無関係です",
  "sync.background_needs_store": () => "バックグラウンドの同期には Store の指定が必要です",

  // accounts and GitHub
  "account.none": () =>
    "アカウントが設定されていません。hh auth login か config.yaml の accounts で足してください",
  "account.not_found": (a) =>
    `${str(a, "name")} というアカウントはありません` +
    (list(a, "candidates") ? `（登録済み: ${list(a, "candidates")}）` : ""),
  "account.exists": (a) => `アカウント ${str(a, "account")} はすでに設定されています`,
  "account.host_mismatch": (a) =>
    `アカウント ${str(a, "account")} のホストと、リモートのホストが違います`,
  "auth.token": (a) =>
    a.account
      ? `アカウント ${str(a, "account")} にトークンがありません。hh auth login で設定してください`
      : "この画面は hh ui が開いた URL からだけ使えます。端末で hh ui を実行してください",
  "auth.unauthorized": (a) =>
    `GitHub がアカウント ${str(a, "account")} のトークンを受け付けませんでした。有効か確かめてください`,
  "auth.insecure": (a) =>
    `${str(a, "url")} は http なので、トークンを送りません。https を使ってください`,
  "auth.host": (a) =>
    `アカウント ${str(a, "account")} のホストは ${str(a, "host")} です。指定と合いません`,
  "auth.need_token": (a) =>
    `アカウント ${str(a, "account")} はキーチェーンにトークンを持ちます。標準入力で渡してください`,
  "auth.device_only": (a) =>
    `アカウント ${str(a, "account")} はデバイスフローでログインします。標準入力のトークンは使えません`,
  "auth.name": () => "アカウントの名前が必要です",
  "github.forbidden": (a) =>
    `GitHub がアカウント ${str(a, "account")} の操作を許可しませんでした。権限を確かめてください`,
  "github.not_found": (a) =>
    `アカウント ${str(a, "account")} では GitHub のリポジトリが見つかりません`,
  "github.rate_limit": (a) =>
    `GitHub の利用制限に達しました（アカウント ${str(a, "account")}）。しばらく待ってください`,

  // queries, views, schema
  "query.invalid": (_, e) => withDetail("クエリが正しくありません", e),
  "query.tree_order": () => "ツリーは rank の順に並ぶため、group: と sort: は使えません",
  "log.workspace_with_task": () => "タスクの履歴にはワークスペースを指定できません",
  "view.bad_name": (a) => `「${str(a, "value")}」はビューの名前に使えません`,
  "view.not_found": (a) => `${str(a, "name")} というビューはありません`,
  "view.name_taken": (a) =>
    a.existing && a.existing !== a.name
      ? `ビュー ${str(a, "existing")} と名前が重複します（大文字小文字は区別しません）`
      : `${str(a, "name")} というビューはすでにあります`,
  "view.broken": (a) =>
    `ビュー ${str(a, "name")} のファイルが壊れています。直すか、削除してください`,
  "view.bad_value": (_, e) => withDetail("ビューの設定が正しくありません", e),
  "view.bad_column": (a) =>
    `列 ${str(a, "column")} は、フィールドでもスキーマのプロパティでもありません`,
  "view.tree_order": () => "ツリーのビューは rank の順に並ぶため、group: と sort: は使えません",
  "migrate.invalid": (_, e) => withDetail("移行するとスキーマに合わないタスクがあります", e),

  // notifications
  "inbox.ulid": (a) => `${str(a, "input")} は完全な ULID ではありません`,
  "inbox.not_found": (a) => `${str(a, "input")} という通知は見つかりません`,
  "inbox.not_revertable": (_, e) => withDetail("この通知の操作は取り消せません", e),
  "notification.unread": () => "通知を未読には戻せません",

  // choices and confirmations (the CLI asks; the GUI picks first)
  "choice.required": (a) => `${str(a, "what")} を選んでください（${list(a, "candidates")}）`,
  "confirm.required": (a) => `${str(a, "what")} には確認が必要です`,

  // the server itself
  "request.invalid": (_, e) => {
    const body = e.body as { errors?: { message?: string }[] } | undefined;
    const ms = (body?.errors ?? []).map((x) => x.message).filter(Boolean);
    return ms.length > 0
      ? `入力が正しくありません: ${ms.join(" / ")}`
      : withDetail("入力が正しくありません", e);
  },
  "host.refused": () => "この画面は 127.0.0.1 か localhost のアドレスからだけ開けます",
  "presence.unknown_client": () => "接続が切れています。再接続を待ってください",
  "http.404": () => "存在しない API です。画面と hh のバージョンが合っていないかもしれません",
  internal: (_, e) => withDetail("サーバーで予期しないエラーが起きました", e),
};

// errorMessage is the words for an error shown to the user.
export function errorMessage(e: unknown): string {
  if (e instanceof ApiError) {
    const make = messages[e.type];
    if (make) return make(e.args, e);
    return `操作に失敗しました（${e.type}）: ${e.detail}`;
  }
  const msg = e instanceof Error ? e.message : String(e);
  return `操作に失敗しました: ${msg}`;
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm vitest run core/api/client.test.ts core/token.test.ts core/messages.test.ts`

Expected: PASS（3 ファイル・17 テスト）

Run: `cd web && pnpm exec biome ci core && pnpm exec svelte-check --fail-on-warnings`

Expected: 指摘ゼロ

- [ ] **Step 5: コミット**

```bash
git add web/core/api/client.ts web/core/token.ts web/core/messages.ts web/core/api/client.test.ts web/core/token.test.ts web/core/messages.test.ts
git commit -m "feat(web): API クライアント・トークン・エラーの文言を足す"
```

---

### Task 4: SSE と接続

**Files:**
- Create: `web/core/sse.ts`
- Create: `web/core/connection.ts`
- Test: `web/core/sse.test.ts`、`web/core/connection.test.ts`

**Interfaces:**
- Consumes: Task 3 の `Api`・`Schemas`（`HelloEvent`・`TasksChanged`・`SyncStatus` は `components["schemas"]` から）
- Produces: `SseMessage`、`parseEventStream`、`ConnState`、`Connection`（plan 前半のシグネチャのとおり）
- イベントの形は `internal/server/events.go` と `api/openapi.yaml` のとおり: `hello` は `{client, stores: [{store, head, sync}], unread}`、`tasks.changed` は `{store, ulids, removed, commit, op, author}`（`ulids`・`removed` は null になりうるので `Connection` が `[]` にして渡す）、`sync.state` は `SyncStatus`（`store` を持つ）、`notifications.unread` は `{count}`、`schema.changed`・`views.changed` は `{store}`
- 状態の遷移（細部 11・36）: 最初は `connecting`。`hello` で `open`。つながったあとに切れると `reconnecting`（1 秒待つ間）。次の接続に失敗した時点で `down`（そのあとの間隔は 2・4・5・5… 秒）。一度も `hello` が来ないまま失敗しても `down`。`hello` が来れば間隔は 1 秒からやり直す
- `resync` は、最初の `hello` と、再接続の `hello` で HEAD が前と違う Store があるときに出る。`tasks.changed` で受けた `commit` も HEAD として覚えるので、イベントで追いついている HEAD は「違う」と見なさない
- `sync.state` と `hello` の `SyncStatus` は、`push_at`（Task 1 が足す。push 待ちがないときは省かれる）も含めてそのまま `conn.sync` に入れる（Task 8 の同期のカードが読む）
- presence（細部 12）: `hello` の直後と `visibilitychange` で `PUT /api/presence {client, visible}`。失敗は握りつぶす（ヒントでしかない）

- [ ] **Step 1: 失敗するテストを書く**

`web/core/sse.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { parseEventStream, type SseMessage } from "./sse";

function collect() {
  const got: SseMessage[] = [];
  return { got, feed: parseEventStream((m) => got.push(m)) };
}

describe("parseEventStream", () => {
  it("reads an event with its name and data", () => {
    const { got, feed } = collect();
    feed('event: hello\ndata: {"client":"c1"}\n\n');
    expect(got).toEqual([{ event: "hello", data: '{"client":"c1"}' }]);
  });

  it("joins a line that is split between chunks", () => {
    const { got, feed } = collect();
    feed("event: tasks.cha");
    feed('nged\ndata: {"sto');
    feed('re":"s"}\n');
    expect(got).toEqual([]);
    feed("\n");
    expect(got).toEqual([{ event: "tasks.changed", data: '{"store":"s"}' }]);
  });

  it("reads CRLF and a CRLF split between chunks", () => {
    const { got, feed } = collect();
    feed("event: a\r\ndata: 1\r");
    feed("\n\r\n");
    feed("event: b\rdata: 2\r\revent: c\n");
    expect(got).toEqual([
      { event: "a", data: "1" },
      { event: "b", data: "2" },
    ]);
  });

  it("joins several data lines with LF", () => {
    const { got, feed } = collect();
    feed("event: x\ndata: a\ndata: b\ndata:c\n\n");
    expect(got).toEqual([{ event: "x", data: "a\nb\nc" }]);
  });

  it("ignores comment lines such as : ping", () => {
    const { got, feed } = collect();
    feed(": ping\n\n");
    feed("event: x\n: ping\ndata: 1\n\n");
    expect(got).toEqual([{ event: "x", data: "1" }]);
  });

  it("names an event with no event: line message, and drops one with no data", () => {
    const { got, feed } = collect();
    feed("data: 1\n\n");
    feed("event: lonely\n\n");
    feed("id: 7\nretry: 1000\ndata: 2\n\n");
    expect(got).toEqual([
      { event: "message", data: "1" },
      { event: "message", data: "2" },
    ]);
  });

  it("reads several events in one chunk", () => {
    const { got, feed } = collect();
    feed("event: a\ndata: 1\n\nevent: b\ndata: 2\n\n");
    expect(got.map((m) => m.event)).toEqual(["a", "b"]);
  });
});
```

`web/core/connection.test.ts`

```ts
import { afterEach, describe, expect, it, vi } from "vitest";
import { createApi } from "./api/client";
import { Connection } from "./connection";

const enc = new TextEncoder();

function sseStream() {
  let c!: ReadableStreamDefaultController<Uint8Array>;
  const body = new ReadableStream<Uint8Array>({
    start(x) {
      c = x;
    },
  });
  return {
    response: new Response(body, { headers: { "Content-Type": "text/event-stream" } }),
    send: (s: string) => c.enqueue(enc.encode(s)),
    end: () => c.close(),
  };
}

const ev = (event: string, data: unknown) => `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;
const sync = (store: string, unpushed = 0) => ({ store, remotes: [], unpushed });
const hello = (client: string, heads: Record<string, string>, unread = 0) =>
  ev("hello", {
    client,
    unread,
    stores: Object.entries(heads).map(([store, head]) => ({ store, head, sync: sync(store) })),
  });

type Streams = ReturnType<typeof sseStream>;

// A server for the tests: each call of /api/events takes the next entry of
// the script (a stream, or null for a refused connection).
function setup(script: (Streams | null)[]) {
  const calls: { url: string; auth: string | null }[] = [];
  const presence: { client: string; visible: boolean }[] = [];
  const f = (async (input: RequestInfo | URL, init?: RequestInit) => {
    if (typeof input === "string") {
      calls.push({ url: input, auth: new Headers(init?.headers).get("Authorization") });
      const next = script.shift();
      if (!next) throw new TypeError("Failed to fetch");
      return next.response;
    }
    const req = input as Request;
    presence.push(await req.json());
    return new Response(null, { status: 204 });
  }) as unknown as typeof fetch;
  const waits: { ms: number; resolve: () => void }[] = [];
  const slept: number[] = [];
  const sleep = (ms: number) =>
    new Promise<void>((resolve) => {
      slept.push(ms);
      waits.push({ ms, resolve });
    });
  const api = createApi({ token: () => "tok", fetch: f });
  const conn = new Connection({ token: () => "tok", fetch: f, api, sleep, now: () => 1000 });
  return {
    conn,
    calls,
    presence,
    slept,
    wake: () => waits.shift()?.resolve(),
    until: (fn: () => boolean) => vi.waitFor(() => expect(fn()).toBe(true)),
  };
}

afterEach(() => {
  Object.defineProperty(document, "visibilityState", { value: "visible", configurable: true });
});

describe("Connection", () => {
  it("opens at hello, keeps sync, unread and the clock, and asks for a resync", async () => {
    const s1 = sseStream();
    const t = setup([s1]);
    const resync = vi.fn();
    t.conn.on("resync", resync);
    t.conn.start();
    expect(t.conn.state).toBe("connecting");

    s1.send(hello("c1", { shared: "h1" }, 3));
    await t.until(() => t.conn.state === "open");
    expect(t.calls[0]).toEqual({ url: "/api/events", auth: "Bearer tok" });
    expect(t.conn.unread).toBe(3);
    expect(t.conn.sync.get("shared")?.store).toBe("shared");
    expect(t.conn.lastOpenAt).toBe(1000);
    expect(resync).toHaveBeenCalledTimes(1);
    await t.until(() => t.presence.length === 1);
    expect(t.presence[0]).toEqual({ client: "c1", visible: true });
    t.conn.stop();
  });

  it("applies sync.state and notifications.unread and tells the subscribers", async () => {
    const s1 = sseStream();
    const t = setup([s1]);
    const changed = vi.fn();
    t.conn.subscribe(changed);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    await t.until(() => t.conn.state === "open");
    changed.mockClear();

    s1.send(ev("sync.state", sync("shared", 2)));
    await t.until(() => t.conn.sync.get("shared")?.unpushed === 2);
    s1.send(ev("notifications.unread", { count: 5 }));
    await t.until(() => t.conn.unread === 5);
    expect(changed).toHaveBeenCalledTimes(2);
    t.conn.stop();
  });

  it("carries push_at of the sync state as it is", async () => {
    const s1 = sseStream();
    const t = setup([s1]);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    await t.until(() => t.conn.state === "open");
    s1.send(ev("sync.state", { ...sync("shared", 1), push_at: "2026-11-02T10:00:30Z" }));
    await t.until(() => t.conn.sync.get("shared")?.push_at === "2026-11-02T10:00:30Z");
    t.conn.stop();
  });

  it("passes tasks.changed, schema.changed and views.changed to the listeners", async () => {
    const s1 = sseStream();
    const t = setup([s1]);
    const tasks = vi.fn();
    const schema = vi.fn();
    const views = vi.fn();
    t.conn.on("tasks.changed", tasks);
    t.conn.on("schema.changed", schema);
    t.conn.on("views.changed", views);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    s1.send(
      ev("tasks.changed", {
        store: "shared",
        ulids: ["U1"],
        removed: null,
        commit: "h2",
        op: "set",
        author: "a",
      }),
    );
    s1.send(ev("schema.changed", { store: "shared" }));
    s1.send(ev("views.changed", { store: "shared" }));
    await t.until(() => views.mock.calls.length === 1);
    expect(tasks).toHaveBeenCalledWith({
      store: "shared",
      ulids: ["U1"],
      removed: [],
      commit: "h2",
      op: "set",
      author: "a",
    });
    expect(schema).toHaveBeenCalledWith({ store: "shared" });
    t.conn.stop();
  });

  it("reconnects after 1, 2, 4, 5 and 5 seconds and shows the state meanwhile", async () => {
    const s1 = sseStream();
    const s2 = sseStream();
    const t = setup([s1, null, null, null, null, s2]);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    await t.until(() => t.conn.state === "open");

    s1.end();
    await t.until(() => t.slept.length === 1);
    expect(t.conn.state).toBe("reconnecting");
    expect(t.slept).toEqual([1000]);

    t.wake(); // the first try fails
    await t.until(() => t.slept.length === 2);
    expect(t.conn.state).toBe("down");
    t.wake();
    await t.until(() => t.slept.length === 3);
    t.wake();
    await t.until(() => t.slept.length === 4);
    t.wake();
    await t.until(() => t.slept.length === 5);
    expect(t.slept).toEqual([1000, 2000, 4000, 5000, 5000]);

    t.wake(); // the sixth try gets through
    s2.send(hello("c2", { shared: "h1" }));
    await t.until(() => t.conn.state === "open");
    t.conn.stop();
  });

  it("does not resync after a reconnect when no HEAD moved", async () => {
    const s1 = sseStream();
    const s2 = sseStream();
    const t = setup([s1, s2]);
    const resync = vi.fn();
    t.conn.on("resync", resync);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    await t.until(() => resync.mock.calls.length === 1);
    s1.send(
      ev("tasks.changed", {
        store: "shared",
        ulids: [],
        removed: [],
        commit: "h2",
        op: "",
        author: "",
      }),
    );
    s1.end();
    await t.until(() => t.slept.length === 1);
    t.wake();
    s2.send(hello("c2", { shared: "h2" })); // the head the events already told
    await t.until(() => t.conn.state === "open");
    expect(resync).toHaveBeenCalledTimes(1);
    t.conn.stop();
  });

  it("resyncs after a reconnect when a HEAD moved", async () => {
    const s1 = sseStream();
    const s2 = sseStream();
    const t = setup([s1, s2]);
    const resync = vi.fn();
    t.conn.on("resync", resync);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    await t.until(() => resync.mock.calls.length === 1);
    s1.end();
    await t.until(() => t.slept.length === 1);
    t.wake();
    s2.send(hello("c2", { shared: "h9" }));
    await t.until(() => resync.mock.calls.length === 2);
    t.conn.stop();
  });

  it("sends presence when the page is hidden or shown", async () => {
    const s1 = sseStream();
    const t = setup([s1]);
    t.conn.start();
    s1.send(hello("c1", { shared: "h1" }));
    await t.until(() => t.presence.length === 1);

    Object.defineProperty(document, "visibilityState", { value: "hidden", configurable: true });
    document.dispatchEvent(new Event("visibilitychange"));
    await t.until(() => t.presence.length === 2);
    expect(t.presence[1]).toEqual({ client: "c1", visible: false });
    t.conn.stop();
  });

  it("goes down when the first connection is refused", async () => {
    const t = setup([null]);
    t.conn.start();
    await t.until(() => t.slept.length === 1);
    expect(t.conn.state).toBe("down");
    expect(t.conn.lastOpenAt).toBeNull();
    t.conn.stop();
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm vitest run core/sse.test.ts core/connection.test.ts`

Expected: FAIL（モジュールが見つからない）

- [ ] **Step 3: 実装する**

`web/core/sse.ts`

```ts
export type SseMessage = { event: string; data: string };

// parseEventStream returns the function to feed the chunks of a
// text/event-stream body to. It calls onMessage for each event, once its
// blank line has come. A line may be split between chunks; lines end with
// LF, CRLF or CR; several data: lines join with LF; lines that begin with a
// colon (the server's ": ping") are comments and are ignored.
export function parseEventStream(onMessage: (m: SseMessage) => void): (chunk: string) => void {
  let buf = "";
  let event = "";
  let data: string[] = [];

  const line = (l: string) => {
    if (l === "") {
      if (data.length > 0) onMessage({ event: event || "message", data: data.join("\n") });
      event = "";
      data = [];
      return;
    }
    if (l.startsWith(":")) return;
    const i = l.indexOf(":");
    const field = i < 0 ? l : l.slice(0, i);
    let value = i < 0 ? "" : l.slice(i + 1);
    if (value.startsWith(" ")) value = value.slice(1);
    if (field === "event") event = value;
    else if (field === "data") data.push(value);
    // id: and retry: are not used; the connection has its own backoff.
  };

  return (chunk) => {
    buf += chunk;
    let start = 0;
    for (let i = 0; i < buf.length; i++) {
      const c = buf[i];
      if (c !== "\n" && c !== "\r") continue;
      if (c === "\r" && i === buf.length - 1) break; // a "\n" may follow in the next chunk
      line(buf.slice(start, i));
      if (c === "\r" && buf[i + 1] === "\n") i++;
      start = i + 1;
    }
    buf = buf.slice(start);
  };
}
```

`web/core/connection.ts`

```ts
import type { Api, Schemas } from "./api/client";
import { parseEventStream } from "./sse";

export type ConnState = "connecting" | "open" | "reconnecting" | "down";
export type HelloEvent = Schemas["HelloEvent"];
export type TasksChanged = Schemas["TasksChanged"];
export type SyncStatus = Schemas["SyncStatus"];

// Seconds between the end of a connection (or a failed try) and the next
// try: 1, 2, 4, then 5 for ever.
const BACKOFF_MS = [1000, 2000, 4000, 5000];

type Handlers = {
  "tasks.changed": (e: TasksChanged) => void;
  "schema.changed": (e: { store: string }) => void;
  "views.changed": (e: { store: string }) => void;
  resync: () => void;
};

// Connection reads /api/events. It keeps the state of the connection, the
// sync state of each store and the unread count, and tells the caches when
// to fetch again (resync): at the first hello, and when a store's HEAD
// moved while the connection was down.
export class Connection {
  private readonly opts: {
    token: () => string | null;
    fetch: typeof fetch;
    api: Api;
    now: () => number;
    sleep: (ms: number) => Promise<void>;
    doc: Document | undefined;
  };
  private _state: ConnState = "connecting";
  private _lastOpenAt: number | null = null;
  private _unread = 0;
  private _sync = new Map<string, SyncStatus>();
  private heads = new Map<string, string>();
  private everOpened = false;
  private clientId: string | null = null;
  private running: object | null = null;
  private abort: AbortController | null = null;
  private readonly subs = new Set<() => void>();
  private readonly handlers: { [K in keyof Handlers]: Set<Handlers[K]> } = {
    "tasks.changed": new Set(),
    "schema.changed": new Set(),
    "views.changed": new Set(),
    resync: new Set(),
  };

  constructor(opts: {
    token: () => string | null;
    fetch?: typeof fetch;
    api: Api;
    now?: () => number;
    sleep?: (ms: number) => Promise<void>;
    doc?: Document;
  }) {
    this.opts = {
      token: opts.token,
      fetch: opts.fetch ?? ((input, init) => globalThis.fetch(input, init)),
      api: opts.api,
      now: opts.now ?? Date.now,
      sleep: opts.sleep ?? ((ms) => new Promise((r) => setTimeout(r, ms))),
      doc: opts.doc ?? (typeof document !== "undefined" ? document : undefined),
    };
  }

  get state(): ConnState {
    return this._state;
  }
  get lastOpenAt(): number | null {
    return this._lastOpenAt;
  }
  get sync(): Map<string, SyncStatus> {
    return this._sync;
  }
  get unread(): number {
    return this._unread;
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  on(event: "tasks.changed", fn: (e: TasksChanged) => void): () => void;
  on(event: "schema.changed" | "views.changed", fn: (e: { store: string }) => void): () => void;
  on(event: "resync", fn: () => void): () => void;
  on(event: keyof Handlers, fn: (e: never) => void): () => void {
    const set = this.handlers[event] as Set<(e: never) => void>;
    set.add(fn);
    return () => set.delete(fn);
  }

  start(): void {
    if (this.running) return;
    const run = {};
    this.running = run;
    this.opts.doc?.addEventListener("visibilitychange", this.onVisibility);
    void this.loop(run);
  }

  stop(): void {
    this.running = null;
    this.abort?.abort();
    this.opts.doc?.removeEventListener("visibilitychange", this.onVisibility);
  }

  private notify(): void {
    for (const fn of [...this.subs]) fn();
  }

  private setState(s: ConnState): void {
    if (this._state === s) return;
    this._state = s;
    this.notify();
  }

  private emit<K extends keyof Handlers>(event: K, ...args: Parameters<Handlers[K]>): void {
    for (const fn of [...this.handlers[event]] as ((...a: unknown[]) => void)[]) fn(...args);
  }

  // loop connects, reads until the stream ends, and tries again after a
  // pause. A stream that ended after its hello starts the pauses over.
  private async loop(run: object): Promise<void> {
    let failures = 0;
    while (this.running === run) {
      const opened = await this.connectOnce(run);
      if (this.running !== run) return;
      if (opened) {
        failures = 0;
        this.setState("reconnecting");
      } else {
        this.setState("down");
      }
      await this.opts.sleep(BACKOFF_MS[Math.min(failures, BACKOFF_MS.length - 1)] ?? 5000);
      failures++;
    }
  }

  // connectOnce reads one connection to its end. It returns whether the
  // hello came.
  private async connectOnce(run: object): Promise<boolean> {
    const ctl = new AbortController();
    this.abort = ctl;
    let opened = false;
    try {
      const headers: Record<string, string> = { Accept: "text/event-stream" };
      const t = this.opts.token();
      if (t) headers.Authorization = `Bearer ${t}`;
      const res = await this.opts.fetch("/api/events", { headers, signal: ctl.signal });
      if (!res.ok || !res.body) return false;
      const feed = parseEventStream((m) => {
        if (this.running !== run) return;
        if (m.event === "hello") opened = true;
        this.dispatch(m.event, m.data);
      });
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        feed(decoder.decode(value, { stream: true }));
      }
    } catch {
      // A broken connection is the same as one that ended.
    }
    return opened;
  }

  private dispatch(event: string, data: string): void {
    let body: unknown;
    try {
      body = JSON.parse(data);
    } catch {
      return;
    }
    switch (event) {
      case "hello":
        this.onHello(body as HelloEvent);
        break;
      case "tasks.changed": {
        const e = body as TasksChanged;
        this.heads.set(e.store, e.commit);
        this.emit("tasks.changed", { ...e, ulids: e.ulids ?? [], removed: e.removed ?? [] });
        break;
      }
      case "sync.state": {
        const s = body as SyncStatus;
        this._sync = new Map(this._sync).set(s.store, s);
        this.notify();
        break;
      }
      case "notifications.unread":
        this._unread = (body as { count: number }).count;
        this.notify();
        break;
      case "schema.changed":
        this.emit("schema.changed", body as { store: string });
        break;
      case "views.changed":
        this.emit("views.changed", body as { store: string });
        break;
    }
  }

  private onHello(h: HelloEvent): void {
    const stores = h.stores ?? [];
    const heads = new Map(stores.map((s) => [s.store, s.head] as const));
    const moved =
      !this.everOpened ||
      heads.size !== this.heads.size ||
      [...heads].some(([store, head]) => this.heads.get(store) !== head);
    this.heads = heads;
    this._sync = new Map(stores.map((s) => [s.store, s.sync] as const));
    this._unread = h.unread;
    this.clientId = h.client;
    this.everOpened = true;
    this._lastOpenAt = this.opts.now();
    this._state = "open";
    this.notify();
    void this.sendPresence();
    if (moved) this.emit("resync");
  }

  private onVisibility = (): void => {
    void this.sendPresence();
  };

  // sendPresence tells the server whether the page is on screen, for its
  // sync loop. It is sent after each hello too: the server counts a new
  // connection as visible, and the page may be hidden by then.
  private async sendPresence(): Promise<void> {
    if (!this.clientId) return;
    const visible = (this.opts.doc?.visibilityState ?? "visible") === "visible";
    try {
      await this.opts.api.PUT("/api/presence", { body: { client: this.clientId, visible } });
    } catch {
      // presence is a hint; a failure changes nothing for the user
    }
  }
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm vitest run core/sse.test.ts core/connection.test.ts`

Expected: PASS（2 ファイル・16 テスト）

Run: `cd web && pnpm exec biome ci core && pnpm exec svelte-check --fail-on-warnings`

Expected: 指摘ゼロ

- [ ] **Step 5: コミット**

```bash
git add web/core/sse.ts web/core/connection.ts web/core/sse.test.ts web/core/connection.test.ts
git commit -m "feat(web): SSE の解釈と再接続つきの Connection を足す"
```

---

### Task 5: キャッシュと保存の制御

**Files:**
- Create: `web/core/cache.ts`
- Create: `web/core/save.ts`
- Create: `web/core/testing.ts`（テスト用のフェイク。Task 6 のテストも使う）
- Test: `web/core/cache.test.ts`、`web/core/save.test.ts`

**Interfaces:**
- Consumes: Task 3 の `Api`・`ApiError`・`unwrap`・`TaskView`・`ShowView`・`errorMessage`、Task 4 の `Connection`（`on("tasks.changed")`・`on("resync")`・`sync`）
- Produces: `LiveMark`・`ListQuery`・`TreeNode`・`TaskCache`、`PatchField`・`SaveState`・`SaveController`（plan 前半のシグネチャのとおり）。`testing.ts` の `tv`・`showOf`・`problem`・`fakeApi`・`FakeConn`・`changed`・`settle`
- 前半に足したもの: `watchTree` の戻り値に `cycle: () => string[]`（親の循環にあるタスクの ULID。画面で警告に使える）
- 取り直しで変わらないタスクを同じオブジェクトにする方法（細部 13）: 行は `TaskView` から `children` と `ShowView` だけの項目を除いたものにして、`JSON.stringify` の文字列（Go の JSON はキーの順が決まっている）を ULID ごとに覚えて比べる。同じなら前のオブジェクトを返す。ツリーの `TreeNode` も、形が同じなら前のオブジェクトを使い回す（`reuseNodes`）。詳細（`ShowView`）も同じ比べ方。何も変わらなかった取り直しは `subscribe` に知らせない
- 印（細部 14）: 10 秒の `setTimeout` で消し、消えたときも `subscribe` に知らせる。書き込みの応答より先に `tasks.changed` が来ても誤って印を付けないよう、`noteOwnCommit` は同じ `commit` で付いた印を取り消す
- 取り直し（細部 13）: `tasks.changed` と `resync` で、表示中の問い合わせをすべて取り直す。詳細は `ulids`・`removed` に入るものだけ（`resync` はすべて）。同じ周期の複数のイベントは1回の取り直しにまとめる。`removed` のタスクはキャッシュから消し、その詳細は 404 になる
- 保存（細部 15）: 同じタスクの `PATCH` は1つずつ順に送る。失敗したら溜めた編集を残し、`flush` がやり直しになる。`editing` は、未送信と送信中（応答をキャッシュに入れ終えるまで）の両方で true。失敗したあとも、溜めた編集が残る間は true（Task 11 の本文エディタが、外からの変更で入力を上書きしない判断に使う）
- 書き込みの応答の `task` は一覧の行と同じ形（`listView`）なので、そのままキャッシュに入れてよい

- [ ] **Step 1: 失敗するテストを書く**

`web/core/testing.ts`

```ts
// Fakes for the tests of core: an API over a function, a Connection that
// the test drives, and tasks to put in answers.
import { type Api, createApi, type ShowView, type TaskView } from "./api/client";
import type { Connection, SyncStatus } from "./connection";

export function tv(ulid: string, title = ulid, extra: Partial<TaskView> = {}): TaskView {
  return {
    ulid,
    id: `T-${ulid}`,
    store: "shared",
    workspace: { ulid: "W1", name: "main", id_prefix: "T" },
    kind: "task",
    title,
    parent: null,
    rank: "a",
    status: "todo",
    assignees: [],
    labels: [],
    blocked_by: [],
    props: {},
    created: "2026-11-01T00:00:00Z",
    updated: "2026-11-01T00:00:00Z",
    deleted_at: null,
    problems: [],
    ...extra,
  };
}

export function showOf(t: TaskView, extra: Partial<ShowView> = {}): ShowView {
  return {
    ...t,
    body: "",
    watchers: [],
    repos: [],
    comments: [],
    reverse: {},
    referenced_by: [],
    history: [],
    ...extra,
  };
}

export type Req = { method: string; path: string; query: URLSearchParams; body: unknown };

// problem is the answer of the server to a request it refuses.
export function problem(status: number, type: string, args: Record<string, unknown> = {}) {
  return new Response(JSON.stringify({ type, title: "x", status, detail: type, args }), {
    status,
    headers: { "Content-Type": "application/problem+json" },
  });
}

// fakeApi is an Api over route: an object it returns is a 200 answer in
// JSON, a Response is sent as it is.
export function fakeApi(route: (r: Req) => unknown): { api: Api; calls: Req[] } {
  const calls: Req[] = [];
  const f = (async (input: RequestInfo | URL) => {
    const req = input as Request;
    const url = new URL(req.url);
    let body: unknown;
    if (req.method !== "GET" && req.method !== "DELETE") body = await req.json().catch(() => null);
    const r: Req = { method: req.method, path: url.pathname, query: url.searchParams, body };
    calls.push(r);
    const out = route(r);
    return out instanceof Response ? out : Response.json(out ?? {});
  }) as unknown as typeof fetch;
  return { api: createApi({ token: () => "tok", fetch: f }), calls };
}

type Listener = (e: never) => void;

// FakeConn stands for Connection: the test emits the events.
export class FakeConn {
  state = "open";
  lastOpenAt: number | null = 1000;
  sync = new Map<string, SyncStatus>();
  unread = 0;
  private readonly listeners = new Map<string, Set<Listener>>();
  private readonly subs = new Set<() => void>();

  on(event: string, fn: Listener): () => void {
    const set = this.listeners.get(event) ?? new Set();
    this.listeners.set(event, set);
    set.add(fn);
    return () => set.delete(fn);
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  emit(event: string, payload?: unknown): void {
    for (const fn of [...(this.listeners.get(event) ?? [])]) (fn as (e: unknown) => void)(payload);
  }

  // changeSync replaces the sync state of a store and tells the subscribers.
  changeSync(s: SyncStatus): void {
    this.sync = new Map(this.sync).set(s.store, s);
    for (const fn of [...this.subs]) fn();
  }

  asConnection(): Connection {
    return this as unknown as Connection;
  }
}

export function changed(
  store: string,
  ulids: string[],
  commit: string,
  op = "set",
  removed: string[] = [],
) {
  return { store, ulids, removed, commit, op, author: "a" };
}

// settle lets the requests that are on their way finish.
export const settle = () => new Promise<void>((r) => setTimeout(r, 0));
```

`web/core/cache.test.ts`

```ts
import { afterEach, describe, expect, it, vi } from "vitest";
import { TaskCache } from "./cache";
import { changed, FakeConn, fakeApi, problem, type Req, settle, showOf, tv } from "./testing";

afterEach(() => {
  vi.useRealTimers();
});

function setup(route: Parameters<typeof fakeApi>[0]) {
  const { api, calls } = fakeApi(route);
  const conn = new FakeConn();
  let t = 1_000_000;
  const cache = new TaskCache({ api, conn: conn.asConnection(), now: () => t });
  return { cache, conn, calls, advance: (ms: number) => (t += ms) };
}

const listCalls = (calls: Req[]) => calls.filter((c) => c.path === "/api/tasks");

describe("TaskCache lists", () => {
  it("fetches the query and keeps each task once by ULID", async () => {
    const { cache, calls } = setup(() => ({ tasks: [tv("A", "a"), tv("B", "b")] }));
    const w = cache.watchList({ q: "kind:task", tree: false, store: "shared" });
    expect(w.loading()).toBe(true);
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    expect(w.ulids()).toEqual(["A", "B"]);
    expect(cache.task("A")?.title).toBe("a");
    expect(listCalls(calls)[0]?.query.get("q")).toBe("kind:task");
    expect(listCalls(calls)[0]?.query.get("store")).toBe("shared");
    w.dispose();
  });

  it("shares one fetch between watchers of the same query", async () => {
    const { cache, calls } = setup(() => ({ tasks: [tv("A")] }));
    const w1 = cache.watchList({ q: "", tree: false });
    const w2 = cache.watchList({ q: "", tree: false });
    await vi.waitFor(() => expect(w2.loading()).toBe(false));
    expect(listCalls(calls)).toHaveLength(1);
    w1.dispose();
    w2.dispose();
  });

  it("builds the tree from the children and holds the cycle", async () => {
    const root = tv("R", "root", {
      children: [tv("C1", "c1", { children: [tv("G", "g")] }), tv("C2")],
    });
    const { cache, calls } = setup(() => ({ roots: [root], cycle: [tv("X")] }));
    const w = cache.watchTree({ q: "workspace:main", tree: true });
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    expect(listCalls(calls)[0]?.query.get("tree")).toBe("true");
    expect(w.roots()).toEqual([
      {
        ulid: "R",
        children: [
          { ulid: "C1", children: [{ ulid: "G", children: [] }] },
          { ulid: "C2", children: [] },
        ],
      },
    ]);
    expect(w.cycle()).toEqual(["X"]);
    expect(cache.task("G")?.title).toBe("g");
    expect(cache.task("R")?.children).toBeUndefined();
    w.dispose();
  });

  it("keeps the objects of the tasks that did not change when it fetches again", async () => {
    let titleB = "b";
    const { cache, conn } = setup(() => ({
      roots: [tv("R", "r", { children: [tv("A", "a"), tv("B", titleB)] })],
    }));
    const w = cache.watchTree({ q: "", tree: true });
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    const a = cache.task("A");
    const b = cache.task("B");
    const roots = w.roots();

    conn.emit("resync");
    await settle();
    await settle();
    expect(cache.task("A")).toBe(a);
    expect(cache.task("B")).toBe(b);
    expect(w.roots()).toBe(roots);

    titleB = "b2";
    conn.emit("tasks.changed", changed("shared", ["B"], "c1"));
    await vi.waitFor(() => expect(cache.task("B")?.title).toBe("b2"));
    expect(cache.task("A")).toBe(a);
    expect(cache.task("B")).not.toBe(b);
    expect(w.roots()).toBe(roots); // the shape is the same
    w.dispose();
  });

  it("notifies only when something changed", async () => {
    const { cache, conn } = setup(() => ({ tasks: [tv("A")] }));
    const w = cache.watchList({ q: "", tree: false });
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    const fn = vi.fn();
    cache.subscribe(fn);
    conn.emit("resync");
    await settle();
    await settle();
    expect(fn).not.toHaveBeenCalled();
    w.dispose();
  });

  it("fetches the queries on screen again at tasks.changed, and no others", async () => {
    const { cache, conn, calls } = setup(() => ({ tasks: [] }));
    const w1 = cache.watchList({ q: "a", tree: false });
    const w2 = cache.watchList({ q: "b", tree: false });
    await vi.waitFor(() => expect(w2.loading()).toBe(false));
    w2.dispose();
    const before = listCalls(calls).length;
    conn.emit("tasks.changed", changed("shared", ["U"], "c1"));
    await vi.waitFor(() => expect(listCalls(calls).length).toBe(before + 1));
    await settle();
    expect(listCalls(calls).length).toBe(before + 1);
    w1.dispose();
  });

  it("holds the error of a query", async () => {
    const { cache } = setup(() => problem(400, "query.invalid"));
    const w = cache.watchList({ q: "(", tree: false });
    await vi.waitFor(() => expect(w.error()).not.toBeNull());
    expect(w.error()?.type).toBe("query.invalid");
    expect(w.loading()).toBe(false);
    w.dispose();
  });
});

describe("TaskCache details", () => {
  it("fetches a task by ID and finds it by ULID", async () => {
    const t = tv("A", "a");
    const { cache, calls } = setup(() => showOf(t, { body: "hello" }));
    const w = cache.watchDetail("T-A");
    await vi.waitFor(() => expect(w.ulid()).toBe("A"));
    expect(calls[0]?.path).toBe("/api/tasks/T-A");
    expect(cache.detail("A")?.body).toBe("hello");
    expect(cache.task("A")?.title).toBe("a");
    expect(cache.task("A")).not.toHaveProperty("body");
    w.dispose();
  });

  it("fetches a detail again only when tasks.changed names it", async () => {
    const { cache, conn, calls } = setup((r) => showOf(tv(r.path.endsWith("/A") ? "A" : "B")));
    const wa = cache.watchDetail("A");
    const wb = cache.watchDetail("B");
    await vi.waitFor(() => expect(wb.ulid()).toBe("B"));
    await vi.waitFor(() => expect(wa.ulid()).toBe("A"));
    const n = calls.length;
    conn.emit("tasks.changed", changed("shared", ["A"], "c1"));
    await vi.waitFor(() => expect(calls.length).toBe(n + 1));
    await settle();
    expect(calls.slice(n).map((c) => c.path)).toEqual(["/api/tasks/A"]);
    wa.dispose();
    wb.dispose();
  });

  it("forgets a purged task and shows the 404 of its detail", async () => {
    let gone = false;
    const { cache, conn } = setup(() => (gone ? problem(404, "task.not_found") : showOf(tv("A"))));
    const w = cache.watchDetail("A");
    await vi.waitFor(() => expect(w.ulid()).toBe("A"));
    gone = true;
    conn.emit("tasks.changed", changed("shared", [], "c2", "purge", ["A"]));
    await vi.waitFor(() => expect(w.error()?.status).toBe(404));
    expect(w.ulid()).toBeNull();
    expect(cache.task("A")).toBeUndefined();
    expect(cache.detail("A")).toBeUndefined();
    w.dispose();
  });
});

describe("TaskCache marks", () => {
  it("marks a change from outside for ten seconds, and tells when it is gone", () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    const { cache, conn, advance } = setup(() => ({}));
    const fn = vi.fn();
    cache.subscribe(fn);
    conn.emit("tasks.changed", changed("shared", ["A"], "c1", "set"));
    expect(cache.live("A")).toEqual({ label: "CLI で更新 · 今", until: 1_010_000 });
    expect(cache.live("B")).toBeUndefined();
    expect(fn).toHaveBeenCalledTimes(1);

    advance(10_000);
    vi.advanceTimersByTime(10_000);
    expect(cache.live("A")).toBeUndefined();
    expect(fn).toHaveBeenCalledTimes(2);
  });

  it("says sync for merge and repair", () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    const { cache, conn } = setup(() => ({}));
    conn.emit("tasks.changed", changed("shared", ["A"], "c1", "merge"));
    conn.emit("tasks.changed", changed("shared", ["B"], "c2", "repair"));
    conn.emit("tasks.changed", changed("shared", ["C"], "c3", "move"));
    expect(cache.live("A")?.label).toBe("同期で更新 · 今");
    expect(cache.live("B")?.label).toBe("同期で更新 · 今");
    expect(cache.live("C")?.label).toBe("CLI で更新 · 今");
  });

  it("does not mark the commit this tab made, whether the event or the answer comes first", () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    const { cache, conn } = setup(() => ({}));
    cache.noteOwnCommit("own1");
    conn.emit("tasks.changed", changed("shared", ["A"], "own1"));
    expect(cache.live("A")).toBeUndefined();

    conn.emit("tasks.changed", changed("shared", ["B"], "own2"));
    expect(cache.live("B")).toBeDefined();
    cache.noteOwnCommit("own2");
    expect(cache.live("B")).toBeUndefined();
  });
});

describe("TaskCache writes", () => {
  it("takes in the task of a write, and puts it into the detail", async () => {
    const t = tv("A", "a");
    const { cache } = setup(() => showOf(t, { body: "text" }));
    const w = cache.watchDetail("A");
    await vi.waitFor(() => expect(w.ulid()).toBe("A"));
    const fn = vi.fn();
    cache.subscribe(fn);

    cache.applyWrite({ task: tv("A", "renamed"), commit: "c9" });
    expect(cache.task("A")?.title).toBe("renamed");
    expect(cache.detail("A")?.title).toBe("renamed");
    expect(cache.detail("A")?.body).toBe("text");
    expect(fn).toHaveBeenCalled();
    w.dispose();
  });

  it("forgets the tasks purged", async () => {
    const { cache } = setup(() => ({ tasks: [tv("A"), tv("B")] }));
    const w = cache.watchList({ q: "", tree: false });
    await vi.waitFor(() => expect(w.loading()).toBe(false));
    cache.applyWrite({ ulids: ["A"], commit: "c9" });
    expect(cache.task("A")).toBeUndefined();
    expect(cache.task("B")).toBeDefined();
    w.dispose();
  });
});
```

`web/core/save.test.ts`

```ts
import { describe, expect, it, vi } from "vitest";
import { TaskCache } from "./cache";
import { SaveController } from "./save";
import { FakeConn, fakeApi, problem, tv } from "./testing";

function setup(route: Parameters<typeof fakeApi>[0] = () => ({})) {
  const { api, calls } = fakeApi((r) => {
    if (r.method === "PATCH") {
      const out = route(r);
      if (out instanceof Response) return out;
      return { task: tv("A", "saved"), commit: "c1", ...(out as object) };
    }
    return route(r);
  });
  const conn = new FakeConn();
  const cache = new TaskCache({ api, conn: conn.asConnection() });
  const timers: { fn: () => void; ms: number; id: number }[] = [];
  let next = 1;
  const setTimer = ((fn: () => void, ms: number) => {
    const id = next++;
    timers.push({ fn, ms, id });
    return id;
  }) as unknown as typeof setTimeout;
  const clearTimer = ((id: number) => {
    const i = timers.findIndex((t) => t.id === id);
    if (i >= 0) timers.splice(i, 1);
  }) as unknown as typeof clearTimeout;
  const save = new SaveController({ api, cache, conn: conn.asConnection(), setTimer, clearTimer });
  const patches = () => calls.filter((c) => c.method === "PATCH");
  return { save, cache, conn, calls, patches, timers };
}

describe("SaveController", () => {
  it("waits two seconds, and starts the wait again at each edit", () => {
    const { save, timers, patches } = setup();
    save.edit("A", "shared", "title", "a");
    expect(timers).toHaveLength(1);
    expect(timers[0]?.ms).toBe(2000);
    save.edit("A", "shared", "title", "ab");
    expect(timers).toHaveLength(1); // the first wait was cleared
    expect(patches()).toHaveLength(0);
  });

  it("sends the fields edited in one PATCH, with the props together", async () => {
    const { save, timers, patches } = setup();
    save.edit("A", "shared", "title", "new title");
    save.edit("A", "shared", "title", "newer title");
    save.edit("A", "shared", "props.points", 3);
    save.edit("A", "shared", "props.due", null);
    save.edit("A", "shared", "assignees", ["alice"]);
    timers[0]?.fn();
    await vi.waitFor(() => expect(save.state("A").kind).toBe("saved"));
    expect(patches()).toHaveLength(1);
    const p = patches()[0];
    expect(p?.path).toBe("/api/tasks/A");
    expect(p?.query.get("store")).toBe("shared");
    expect(p?.body).toEqual({
      title: "newer title",
      assignees: ["alice"],
      props: { points: 3, due: null },
    });
  });

  it("sends at once at flush, and the answer goes to the cache", async () => {
    const { save, cache, timers, patches } = setup();
    save.edit("A", "shared", "title", "x");
    expect(save.editing("A", "title")).toBe(true);
    await save.flush("A");
    expect(timers).toHaveLength(0);
    expect(patches()).toHaveLength(1);
    expect(save.editing("A", "title")).toBe(false);
    expect(cache.task("A")?.title).toBe("saved");
    expect(save.state("A")).toEqual({ kind: "saved", pendingPush: false });
  });

  it("goes through saving, and says push is pending when the store has unpushed commits", async () => {
    const { save, conn } = setup();
    const kinds: string[] = [];
    save.subscribe(() => kinds.push(save.state("A").kind));
    save.edit("A", "shared", "title", "x");
    await save.flush("A");
    expect(kinds).toContain("saving");
    expect(save.state("A")).toEqual({ kind: "saved", pendingPush: false });

    conn.changeSync({ store: "shared", remotes: ["origin"], unpushed: 2 });
    expect(save.state("A")).toEqual({ kind: "saved", pendingPush: true });
    conn.changeSync({ store: "shared", remotes: ["origin"], unpushed: 0 });
    expect(save.state("A")).toEqual({ kind: "saved", pendingPush: false });
  });

  it("shows the words of the error, and sends the same edits again at the retry", async () => {
    let fail = true;
    const { save, patches } = setup(() => (fail ? problem(422, "schema.violation") : {}));
    save.edit("A", "shared", "status", "bogus");
    await save.flush("A");
    const s = save.state("A");
    expect(s.kind).toBe("error");
    expect(s.kind === "error" && s.message).toContain("スキーマに合わない変更です");
    expect(save.editing("A", "status")).toBe(true);

    fail = false;
    await save.flush("A");
    expect(patches()).toHaveLength(2);
    expect(patches()[1]?.body).toEqual({ status: "bogus" });
    expect(save.state("A").kind).toBe("saved");
  });

  it("sends one PATCH of a task at a time, in the order of the edits", async () => {
    const { save, patches } = setup();
    let second: Promise<void> | undefined;
    let fired = false;
    save.subscribe(() => {
      // An edit that comes while the first PATCH is on its way.
      if (fired || save.state("A").kind !== "saving") return;
      fired = true;
      save.edit("A", "shared", "title", "two");
      expect(save.editing("A", "title")).toBe(true);
      second = save.flush("A");
    });
    save.edit("A", "shared", "title", "one");
    await save.flush("A");
    await second;
    expect(patches().map((p) => (p.body as { title: string }).title)).toEqual(["one", "two"]);
    expect(save.editing("A", "title")).toBe(false);
  });

  it("keeps editing true until the answer is in the cache, and after a failure", async () => {
    let fail = true;
    const { save, cache } = setup(() => (fail ? problem(422, "schema.violation") : {}));
    const seen: { editing: boolean; title?: string }[] = [];
    save.subscribe(() =>
      seen.push({ editing: save.editing("A", "body"), title: cache.task("A")?.title }),
    );
    save.edit("A", "shared", "body", "text");
    expect(save.editing("A", "body")).toBe(true); // not sent
    await save.flush("A");
    expect(save.editing("A", "body")).toBe(true); // failed: the text is still the user's
    fail = false;
    seen.length = 0;
    await save.flush("A");
    expect(save.editing("A", "body")).toBe(false);
    // false is never seen before the cache has the answer
    for (const s of seen) if (!s.editing) expect(s.title).toBe("saved");
    expect(seen.some((s) => s.editing)).toBe(true); // while it was on its way
  });

  it("does nothing at flush when nothing was edited", async () => {
    const { save, patches } = setup();
    await save.flush("A");
    expect(patches()).toHaveLength(0);
    expect(save.state("A")).toEqual({ kind: "idle" });
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm vitest run core/cache.test.ts core/save.test.ts`

Expected: FAIL（`./cache`・`./save` が見つからない）

- [ ] **Step 3: 実装する**

`web/core/cache.ts`

```ts
import { type Api, ApiError, type ShowView, type TaskView, unwrap } from "./api/client";
import type { Connection, TasksChanged } from "./connection";

export type LiveMark = { label: "CLI で更新 · 今" | "同期で更新 · 今"; until: number };
export type ListQuery = { q: string; tree: boolean; store?: string };
export type TreeNode = { ulid: string; children: TreeNode[] };

// How long a task stays marked as changed from outside.
const MARK_MS = 10_000;
// The ops that sync makes: a change by one of them came from a remote.
const SYNC_OPS = new Set(["merge", "repair"]);
// How many of the commits this tab made to remember.
const OWN_COMMITS = 100;
// What ShowView has on top of TaskView; the shared row of a task has none of it.
const SHOW_ONLY = [
  "children",
  "body",
  "watchers",
  "repos",
  "comments",
  "reverse",
  "referenced_by",
  "history",
] as const;

type QueryState = {
  query: ListQuery;
  refs: number;
  ulids: string[];
  roots: TreeNode[];
  cycle: string[];
  error: ApiError | null;
  loading: boolean;
  seq: number;
};

type DetailState = {
  key: string;
  refs: number;
  ulid: string | null;
  error: ApiError | null;
  seq: number;
};

type Mark = { mark: LiveMark; commit: string; timer: ReturnType<typeof setTimeout> };

function sameList<T>(a: readonly T[], b: readonly T[]): boolean {
  return a.length === b.length && a.every((x, i) => x === b[i]);
}

// reuseNodes returns prev, or the parts of it, where next says the same, so
// that a tree that did not change is the same object.
function reuseNodes(next: TreeNode[], prev: TreeNode[]): TreeNode[] {
  const before = new Map(prev.map((n) => [n.ulid, n]));
  const out = next.map((n) => {
    const p = before.get(n.ulid);
    if (!p) return n;
    const children = reuseNodes(n.children, p.children);
    return children === p.children ? p : { ulid: n.ulid, children };
  });
  return sameList(out, prev) ? prev : out;
}

function toApiError(e: unknown): ApiError {
  if (e instanceof ApiError) return e;
  return new ApiError({
    status: 0,
    type: "network",
    detail: e instanceof Error ? e.message : String(e),
  });
}

// rowOf is the row of a task: a TaskView without its children, and without
// what a ShowView adds.
function rowOf(v: object): TaskView {
  const o = { ...v } as Record<string, unknown>;
  for (const k of SHOW_ONLY) delete o[k];
  return o as TaskView;
}

// TaskCache keeps the tasks the screen shows, once each, by ULID. A list or
// a tree is a query that holds the ULIDs of its result. A task that did not
// change when it was fetched again stays the same object (===), so that the
// views that show it do not draw it again.
export class TaskCache {
  private readonly api: Api;
  private readonly now: () => number;
  private readonly rows = new Map<string, TaskView>();
  private readonly rowSigs = new Map<string, string>();
  private readonly shows = new Map<string, ShowView>();
  private readonly showSigs = new Map<string, string>();
  private readonly queries = new Map<string, QueryState>();
  private readonly details = new Map<string, DetailState>();
  private readonly marks = new Map<string, Mark>();
  private readonly own = new Set<string>();
  private readonly subs = new Set<() => void>();
  private dirty = false;
  private refreshQueued = false;
  private refreshAll = false;
  private refreshUlids = new Set<string>();

  constructor(opts: { api: Api; conn: Connection; now?: () => number }) {
    this.api = opts.api;
    this.now = opts.now ?? Date.now;
    opts.conn.on("tasks.changed", (e) => this.onTasksChanged(e));
    opts.conn.on("resync", () => this.queueRefresh(null));
  }

  task(ulid: string): TaskView | undefined {
    return this.rows.get(ulid);
  }

  detail(ulid: string): ShowView | undefined {
    return this.shows.get(ulid);
  }

  live(ulid: string): LiveMark | undefined {
    return this.marks.get(ulid)?.mark;
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  watchList(q: ListQuery) {
    const s = this.acquire(q);
    let done = false;
    return {
      ulids: () => s.ulids,
      error: () => s.error,
      loading: () => s.loading,
      dispose: () => {
        if (!done) {
          done = true;
          this.release(s);
        }
      },
    };
  }

  watchTree(q: ListQuery) {
    const s = this.acquire({ ...q, tree: true });
    let done = false;
    return {
      roots: () => s.roots,
      cycle: () => s.cycle,
      error: () => s.error,
      loading: () => s.loading,
      dispose: () => {
        if (!done) {
          done = true;
          this.release(s);
        }
      },
    };
  }

  watchDetail(idOrUlid: string) {
    let s = this.details.get(idOrUlid);
    if (!s) {
      s = { key: idOrUlid, refs: 0, ulid: null, error: null, seq: 0 };
      this.details.set(idOrUlid, s);
      void this.fetchDetail(s);
    }
    s.refs++;
    const st = s;
    let done = false;
    return {
      ulid: () => st.ulid,
      error: () => st.error,
      dispose: () => {
        if (done) return;
        done = true;
        if (--st.refs === 0) this.details.delete(st.key);
      },
    };
  }

  // noteOwnCommit remembers a commit this tab made, so that its
  // tasks.changed is not shown as a change from outside. The event may come
  // before the answer of the write; the marks it made are taken back.
  noteOwnCommit(commit: string | null | undefined): void {
    if (!commit) return;
    this.own.add(commit);
    if (this.own.size > OWN_COMMITS) this.own.delete(this.own.values().next().value as string);
    for (const [ulid, m] of this.marks) {
      if (m.commit === commit) this.dropMark(ulid);
    }
    this.flush();
  }

  // applyWrite takes in the answer of a write: the tasks as they are now,
  // and the ULIDs purged.
  applyWrite(r: {
    task?: TaskView;
    tasks?: TaskView[];
    ulids?: string[];
    commit?: string | null;
  }): void {
    for (const t of [...(r.task ? [r.task] : []), ...(r.tasks ?? [])]) {
      const row = this.ingest(t);
      const show = this.shows.get(t.ulid);
      if (show) this.putShow({ ...show, ...row });
    }
    for (const u of r.ulids ?? []) this.forget(u);
    this.noteOwnCommit(r.commit);
    this.flush();
  }

  // ---- queries

  private acquire(q: ListQuery): QueryState {
    const key = JSON.stringify([q.q, q.tree, q.store ?? null]);
    let s = this.queries.get(key);
    if (!s) {
      s = {
        query: q,
        refs: 0,
        ulids: [],
        roots: [],
        cycle: [],
        error: null,
        loading: true,
        seq: 0,
      };
      this.queries.set(key, s);
      void this.fetchQuery(s);
    }
    s.refs++;
    return s;
  }

  private release(s: QueryState): void {
    if (--s.refs > 0) return;
    for (const [k, v] of this.queries) if (v === s) this.queries.delete(k);
  }

  private async fetchQuery(s: QueryState): Promise<void> {
    const seq = ++s.seq;
    const { q, tree, store } = s.query;
    try {
      const out = await unwrap(
        this.api.GET("/api/tasks", { params: { query: { q, tree, store } } }),
      );
      if (seq !== s.seq) return; // a newer fetch is on its way
      if (tree) {
        const roots = reuseNodes(
          (out.roots ?? []).map((t) => this.ingestTree(t)),
          s.roots,
        );
        if (roots !== s.roots) {
          s.roots = roots;
          this.dirty = true;
        }
        const cycle = (out.cycle ?? []).map((t) => this.ingestTree(t).ulid);
        if (!sameList(cycle, s.cycle)) {
          s.cycle = cycle;
          this.dirty = true;
        }
      } else {
        const ulids = (out.tasks ?? []).map((t) => this.ingest(t).ulid);
        if (!sameList(ulids, s.ulids)) {
          s.ulids = ulids;
          this.dirty = true;
        }
      }
      if (s.error) this.dirty = true;
      s.error = null;
    } catch (e) {
      if (seq !== s.seq) return;
      s.error = toApiError(e);
      this.dirty = true;
    }
    if (s.loading) {
      s.loading = false;
      this.dirty = true;
    }
    this.flush();
  }

  // ingestTree takes in a task and, under it, its children.
  private ingestTree(t: TaskView): TreeNode {
    const row = this.ingest(t);
    return { ulid: row.ulid, children: (t.children ?? []).map((c) => this.ingestTree(c)) };
  }

  private ingest(t: TaskView | ShowView): TaskView {
    const row = rowOf(t);
    const sig = JSON.stringify(row);
    const old = this.rows.get(row.ulid);
    if (old && this.rowSigs.get(row.ulid) === sig) return old;
    this.rows.set(row.ulid, row);
    this.rowSigs.set(row.ulid, sig);
    this.dirty = true;
    return row;
  }

  // ---- details

  private async fetchDetail(s: DetailState): Promise<void> {
    const seq = ++s.seq;
    try {
      const show = await unwrap(
        this.api.GET("/api/tasks/{id}", { params: { path: { id: s.key } } }),
      );
      if (seq !== s.seq) return;
      this.ingest(show);
      this.putShow(show);
      s.ulid = show.ulid;
      s.error = null;
    } catch (e) {
      if (seq !== s.seq) return;
      const err = toApiError(e);
      s.error = err;
      if (err.status === 404 && s.ulid) {
        this.shows.delete(s.ulid);
        this.showSigs.delete(s.ulid);
        s.ulid = null;
      }
    }
    this.dirty = true;
    this.flush();
  }

  private putShow(show: ShowView): void {
    const sig = JSON.stringify(show);
    if (this.showSigs.get(show.ulid) === sig) return;
    this.shows.set(show.ulid, show);
    this.showSigs.set(show.ulid, sig);
    this.dirty = true;
  }

  private forget(ulid: string): void {
    this.rows.delete(ulid);
    this.rowSigs.delete(ulid);
    this.shows.delete(ulid);
    this.showSigs.delete(ulid);
    this.dropMark(ulid);
    this.dirty = true;
  }

  // ---- events

  private onTasksChanged(e: TasksChanged): void {
    const removed = e.removed ?? [];
    const ulids = e.ulids ?? [];
    if (!this.own.has(e.commit)) {
      const label = SYNC_OPS.has(e.op) ? "同期で更新 · 今" : "CLI で更新 · 今";
      for (const u of ulids) this.setMark(u, label, e.commit);
    }
    for (const u of removed) this.forget(u);
    this.queueRefresh([...ulids, ...removed]);
    this.flush();
  }

  private setMark(ulid: string, label: LiveMark["label"], commit: string): void {
    this.dropMark(ulid);
    const timer = setTimeout(() => {
      this.dropMark(ulid);
      this.flush();
    }, MARK_MS);
    this.marks.set(ulid, { mark: { label, until: this.now() + MARK_MS }, commit, timer });
    this.dirty = true;
  }

  private dropMark(ulid: string): void {
    const m = this.marks.get(ulid);
    if (!m) return;
    clearTimeout(m.timer);
    this.marks.delete(ulid);
    this.dirty = true;
  }

  // queueRefresh fetches again what is on screen: every query (a create, a
  // move or a delete changes the order), and the details of ulids; null is
  // every detail. Events that come in one turn make one round of requests.
  private queueRefresh(ulids: string[] | null): void {
    if (ulids === null) this.refreshAll = true;
    else for (const u of ulids) this.refreshUlids.add(u);
    if (this.refreshQueued) return;
    this.refreshQueued = true;
    queueMicrotask(() => {
      this.refreshQueued = false;
      const all = this.refreshAll;
      const us = this.refreshUlids;
      this.refreshAll = false;
      this.refreshUlids = new Set();
      for (const s of this.queries.values()) void this.fetchQuery(s);
      for (const s of this.details.values()) {
        if (all || us.has(s.key) || (s.ulid !== null && us.has(s.ulid))) void this.fetchDetail(s);
      }
    });
  }

  private flush(): void {
    if (!this.dirty) return;
    this.dirty = false;
    for (const fn of [...this.subs]) fn();
  }
}
```

`web/core/save.ts`

```ts
import { type Api, type Schemas, unwrap } from "./api/client";
import type { TaskCache } from "./cache";
import type { Connection } from "./connection";
import { errorMessage } from "./messages";

export type PatchField =
  | "title"
  | "kind"
  | "status"
  | "assignees"
  | "watchers"
  | "labels"
  | "repos"
  | "blocked_by"
  | "body"
  | `props.${string}`;

export type SaveState =
  | { kind: "idle" }
  | { kind: "saving" }
  | { kind: "saved"; pendingPush: boolean }
  | { kind: "error"; message: string };

type Patch = Schemas["PatchInputBody"];

const DELAY_MS = 2000;

// patchOf makes the body of one PATCH from the fields edited: props.<name>
// goes into props, the others are fields of the body.
function patchOf(fields: Map<PatchField, unknown>): Patch {
  const body: Record<string, unknown> = {};
  const props: Record<string, unknown> = {};
  for (const [field, value] of fields) {
    if (field.startsWith("props.")) props[field.slice("props.".length)] = value;
    else body[field] = value;
  }
  if (Object.keys(props).length > 0) body.props = props;
  return body as Patch;
}

// SaveController sends the edits of a task as one PATCH (one commit): when
// the input stopped for two seconds, or at flush (the field lost focus, the
// screen is left). It keeps the state of the save for each task.
export class SaveController {
  private readonly api: Api;
  private readonly cache: TaskCache;
  private readonly conn: Connection;
  private readonly delayMs: number;
  private readonly setTimer: (fn: () => void, ms: number) => unknown;
  private readonly clearTimer: (t: unknown) => void;
  // The fields edited and not sent yet, by task.
  private readonly pending = new Map<string, Map<PatchField, unknown>>();
  // The fields of a PATCH on its way, by task.
  private readonly sending = new Map<string, Set<PatchField>>();
  private readonly timers = new Map<string, unknown>();
  private readonly stores = new Map<string, string>();
  private readonly states = new Map<string, SaveState>();
  private readonly chains = new Map<string, Promise<void>>();
  private readonly subs = new Set<() => void>();

  constructor(opts: {
    api: Api;
    cache: TaskCache;
    conn: Connection;
    delayMs?: number;
    setTimer?: typeof setTimeout;
    clearTimer?: typeof clearTimeout;
  }) {
    this.api = opts.api;
    this.cache = opts.cache;
    this.conn = opts.conn;
    this.delayMs = opts.delayMs ?? DELAY_MS;
    this.setTimer = (fn, ms) => (opts.setTimer ?? setTimeout)(fn, ms);
    this.clearTimer = (t) => (opts.clearTimer ?? clearTimeout)(t as never);
  }

  // edit takes the value of a field and starts the wait again. The latest
  // value of a field wins.
  edit(ulid: string, store: string, field: PatchField, value: unknown): void {
    let fields = this.pending.get(ulid);
    if (!fields) {
      fields = new Map();
      this.pending.set(ulid, fields);
    }
    fields.set(field, value);
    this.stores.set(ulid, store);
    const t = this.timers.get(ulid);
    if (t !== undefined) this.clearTimer(t);
    this.timers.set(
      ulid,
      this.setTimer(() => {
        this.timers.delete(ulid);
        void this.flush(ulid);
      }, this.delayMs),
    );
    this.notify();
  }

  // flush sends what is waiting for ulid now. It resolves when the save is
  // over, whether it worked or not (the state tells).
  flush(ulid: string): Promise<void> {
    const t = this.timers.get(ulid);
    if (t !== undefined) {
      this.clearTimer(t);
      this.timers.delete(ulid);
    }
    // One PATCH of a task at a time, in the order of the edits.
    const next = (this.chains.get(ulid) ?? Promise.resolve()).then(() => this.send(ulid));
    this.chains.set(ulid, next);
    return next;
  }

  state(ulid: string): SaveState {
    const s = this.states.get(ulid) ?? { kind: "idle" };
    if (s.kind !== "saved") return s;
    const store = this.stores.get(ulid);
    const unpushed = store ? (this.conn.sync.get(store)?.unpushed ?? 0) : 0;
    return { kind: "saved", pendingPush: unpushed >= 1 };
  }

  // editing is whether the field has an edit that the server has not
  // answered. The screen does not overwrite its input with a change from
  // outside while it is true.
  editing(ulid: string, field: PatchField): boolean {
    return !!this.pending.get(ulid)?.has(field) || !!this.sending.get(ulid)?.has(field);
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  private notify(): void {
    for (const fn of [...this.subs]) fn();
  }

  private async send(ulid: string): Promise<void> {
    const fields = this.pending.get(ulid);
    const store = this.stores.get(ulid);
    if (!fields || fields.size === 0 || !store) return;
    this.pending.delete(ulid);
    this.sending.set(ulid, new Set(fields.keys()));
    this.states.set(ulid, { kind: "saving" });
    this.notify();
    try {
      const w = await unwrap(
        this.api.PATCH("/api/tasks/{id}", {
          params: { path: { id: ulid }, query: { store } },
          body: patchOf(fields),
        }),
      );
      this.cache.applyWrite({ task: w.task, commit: w.commit });
      this.states.set(ulid, { kind: "saved", pendingPush: false });
    } catch (e) {
      // Keep the edits for the retry, unless the field was edited again.
      const again = this.pending.get(ulid) ?? new Map<PatchField, unknown>();
      for (const [k, v] of fields) if (!again.has(k)) again.set(k, v);
      this.pending.set(ulid, again);
      this.states.set(ulid, { kind: "error", message: errorMessage(e) });
    }
    this.sending.delete(ulid);
    this.notify();
  }
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm vitest run core/cache.test.ts core/save.test.ts`

Expected: PASS（2 ファイル・23 テスト）

Run: `cd web && pnpm exec biome ci core && pnpm exec svelte-check --fail-on-warnings`

Expected: 指摘ゼロ

- [ ] **Step 5: コミット**

```bash
git add web/core/cache.ts web/core/save.ts web/core/testing.ts web/core/cache.test.ts web/core/save.test.ts
git commit -m "feat(web): タスクのキャッシュと保存の制御を足す"
```

---

### Task 6: CLI の文字列・Store とスキーマ・表示の整形

**Files:**
- Create: `web/core/cli.ts`
- Create: `web/core/format.ts`
- Create: `web/core/schema.ts`
- Create: `web/core/stores.ts`
- Create: `web/core/people.ts`（`@` の候補）
- Create: `web/core/search.ts`（`#` などのタスク検索）
- Create: `web/core/history.ts`（時点の内容と、復元の差分）
- Test: `web/core/cli.test.ts`、`web/core/format.test.ts`、`web/core/schema.test.ts`、`web/core/stores.test.ts`、`web/core/people.test.ts`、`web/core/search.test.ts`、`web/core/history.test.ts`

**Interfaces:**
- Consumes: Task 3 の `Api`・`unwrap`・`TaskView`・`Schemas`、Task 4 の `Connection`、Task 5 の `TaskCache`（`watchList`・`task`）と `testing.ts`
- Produces（前半のシグネチャのとおり）: `quoteArg`・`cliLine`・`cli*`（`cliLs(q, tree, store?)` は `--store <名前>` を先頭に付ける）、`formatDate`・`formatDateTime`・`formatRelative`・`initials`・`avatarTone`、`SchemaView`・`SchemaCache`・`canParent`・`parentRule`・`defaultStatus`・`PropEditor`（前半の13値）・`propEditorOf`、`StoreTone`・`WorkspaceEntry`・`StoreDirectory`（plan 前半のシグネチャのとおり）
- 前半に足したもの: `StoreDirectory.error(): ApiError | null`（Store の一覧が取れなかった理由。取れたら null）。`cliSet` の `assigns` は、キーが `+` か `-` で終わると `+=`・`-=` になる（`labels+` は `labels+=x`）。値が空なら `key=`（クリア）
- CLI の文字列（細部 16）: 引数は、文字・数字と `_ - . , : / @ % + =` だけなら素のまま、それ以外（空白・記号・空文字）はダブルクォートで囲み、`"`・`\`・`$`・`` ` `` をバックスラッシュでエスケープする。`hh add` の `--body` は Task 1 で足す。`--store` は hh 全体のフラグ
- スキーマ（`internal/schema` で確かめた意味）: `KindDef.parents` が null ならどこにでも置ける、`[]` ならどこにも置けない、リストの `root` が「親なし」。`canParent(s, kind, null)` は `root` を見る。`parentRule` の文言はキャンバスの CreateTask・Main に合わせる（`エピックを置ける親はルートとプロジェクトだけです`、親がルートだけなら `プロジェクトはルートにだけ置けます`）。`PropDef.type` は `text`・`number`・`bool`・`date`・`datetime`・`duration`・`enum`・`multi_enum`・`user`・`task_ref`・`url` で、`propEditorOf` はそれぞれ同名（`task_ref` だけ `taskref`）の `PropEditor` に対応づける。組み込みのフィールドは `status`・`kind`・`assignees`/`watchers`（`user`）・`blocked_by`（`taskref`）・`title`/`labels`/`repos`/`body`（`text`）
- Store の色（細部 19）: `GET /api/stores` の順に 7 色、8 つ目以降は先頭に戻る。`conn.sync` の `read_only` がある Store と、一覧にない Store は `readonly`（灰色）。ワークスペースは Store の順・名前順、プロジェクトは rank 順（細部 18）。プロジェクトは `TaskCache.watchList({ q: "kind:project" })` で集めるので、`tasks.changed` に自動で追従する。Store とワークスペースは `resync` と `tasks.changed`（ワークスペースの追加もコミット）で取り直し、変わらなければ `subscribe` に知らせず、変わらない行は同じオブジェクトのまま
- 日付（細部 21）: `2026-11-07` のような日付だけの文字列はローカルのその日として読む（UTC の 0 時として読むと、ずれる）。`avatarTone` は名前の文字コードの 31 進ハッシュを 4 で割った余り
- `@` の候補（利用者の決定 8・細部 45）: `class PeopleCache { constructor(opts: { api: Api; conn: Connection }); get(store: string): string[] | undefined; load(store: string): Promise<string[]>; subscribe(fn: () => void): () => void }`（`GET /api/people?store=`。Store ごとに覚え、`tasks.changed` でその Store を古くして次の `load` で取り直す。`resync` は全 Store を古くする。名前が変わらなければ同じ配列）と、`matchPeople(names: readonly string[], typed: string, limit = 8): string[]`（前方一致を先、含むを後、大文字小文字を無視）
- タスク検索（`#`・パレット・作成ダイアログの親・ブロックの選択で共有）: `taskSearchQuery(text: string, extra = ""): string`（空白で分けた各語をダブルクォートで囲み、`:` や先頭の `-` をキーや否定と読ませない。`extra` は `kind:project` のような追加の条件で先頭に置く）と、`searchTasks(api: Api, text: string, opts?: { store?: string; extra?: string; limit?: number }): Promise<TaskView[]>`（`GET /api/tasks?q=`。既定の `limit` は 8。語も `extra` もなければ API を呼ばず `[]`）
- 時点の内容（利用者の決定 9・細部 46）: `showAt(api: Api, id: string, commit: string, store?: string): Promise<ShowView>`（`GET /api/tasks/{id}/history/{commit}`。そのコミットにタスクがなければ `ApiError` の `restore_at.not_found`）と、純粋関数 `diffForRestore(now: ShowView, then: ShowView, opts?: { idOf?: (ulid: string) => string; propLabel?: (name: string) => string }): RestoreChange[]`（`RestoreChange = { field: string; label: string; from: string; to: string }`。タイトル・種類・ステータス・親・担当者・ラベル・ブロック・props・本文のうち違うものだけを、今 → 戻したあとで返す。リストは順序を無視して比べる。本文は `from: "現在の本文"`・`to: "この時点の本文"` の固定で、差分の有無だけ。空配列は「この時点と同じです」）
- `restore_at.not_found` の文言は `args.at` があれば「<at> の時点にはこのタスクがありませんでした」、なければ「その時点にはこのタスクがありませんでした」（時点の取得ではコミットだけが分かるため、どちらでも自然に読める）

- [ ] **Step 1: 失敗するテストを書く**

`web/core/cli.test.ts`

```ts
import { describe, expect, it } from "vitest";
import {
  cliAdd,
  cliComment,
  cliLs,
  cliMv,
  cliPurge,
  cliRestore,
  cliRestoreAt,
  cliRm,
  cliSet,
  cliStoreInit,
  cliSync,
  cliWatch,
  quoteArg,
} from "./cli";

describe("quoteArg", () => {
  it("leaves a plain argument as it is", () => {
    expect(quoteArg("REL-8")).toBe("REL-8");
    expect(quoteArg("status:todo")).toBe("status:todo");
    expect(quoteArg("points=3")).toBe("points=3");
    expect(quoteArg("リリース作業")).toBe("リリース作業");
  });

  it("quotes an argument with a space or a symbol", () => {
    expect(quoteArg("fix the bug")).toBe('"fix the bug"');
    expect(quoteArg("a;b")).toBe('"a;b"');
    expect(quoteArg("")).toBe('""');
  });

  it('escapes " \\ $ and ` with a backslash', () => {
    expect(quoteArg('say "hi"')).toBe('"say \\"hi\\""');
    expect(quoteArg("a\\b c")).toBe('"a\\\\b c"');
    expect(quoteArg("$HOME")).toBe('"\\$HOME"');
    expect(quoteArg("`date`")).toBe('"\\`date\\`"');
  });
});

describe("commands", () => {
  it("writes hh add with its options", () => {
    expect(
      cliAdd({
        title: "Switch DNS",
        store: "shared",
        workspace: "main",
        parent: "REL-8",
        kind: "task",
        body: "line 1\nline 2",
        props: { points: "2", priority: "high" },
      }),
    ).toBe(
      'hh add "Switch DNS" --store shared --workspace main --parent REL-8 --kind task --body "line 1\nline 2" -p points=2 -p priority=high',
    );
    expect(cliAdd({ title: "x" })).toBe("hh add x");
  });

  it("writes hh ls", () => {
    expect(cliLs("", true)).toBe("hh ls --tree");
    expect(cliLs("status:todo assignee:alice", false)).toBe('hh ls "status:todo assignee:alice"');
    expect(cliLs("kind:project", false)).toBe("hh ls kind:project");
    expect(cliLs("workspace:main -status:canceled", true, "shared")).toBe(
      'hh ls --store shared --tree "workspace:main -status:canceled"',
    );
  });

  it("writes hh set, with the operators of a list", () => {
    expect(cliSet("REL-8", { status: "done", "labels+": "ops", due: "", title: "A B" })).toBe(
      'hh set REL-8 status=done labels+=ops due= "title=A B"',
    );
  });

  it("writes hh mv", () => {
    expect(cliMv("REL-8", { parent: "REL-1", after: "REL-3" })).toBe(
      "hh mv REL-8 --parent REL-1 --after REL-3",
    );
    expect(cliMv("REL-8", { root: true, before: "REL-2" })).toBe(
      "hh mv REL-8 --root --before REL-2",
    );
  });

  it("writes the others", () => {
    expect(cliRm("REL-8")).toBe("hh rm REL-8");
    expect(cliRestore("REL-8")).toBe("hh restore REL-8");
    expect(cliPurge("REL-8")).toBe("hh purge REL-8");
    expect(cliRestoreAt("REL-8", "abc123")).toBe("hh restore REL-8 --at abc123");
    expect(cliComment("REL-8", "add")).toBe("hh comment REL-8");
    expect(cliComment("REL-8", "edit", "01HX")).toBe("hh comment edit 01HX");
    expect(cliComment("REL-8", "rm", "01HX")).toBe("hh comment rm 01HX");
    expect(cliWatch("REL-8", true)).toBe("hh watch REL-8");
    expect(cliWatch("REL-8", false)).toBe("hh unwatch REL-8");
    expect(cliSync()).toBe("hh sync");
    expect(cliSync("shared")).toBe("hh sync --store shared");
    expect(
      cliStoreInit({ name: "shared", prefix: "WEB", workspace: "web", remotes: ["git@x:a/b.git"] }),
    ).toBe("hh store init shared --prefix WEB --workspace web --remote git@x:a/b.git");
  });
});
```

`web/core/format.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { avatarTone, formatDate, formatDateTime, formatRelative, initials } from "./format";

const now = new Date(2026, 10, 4, 12, 0, 0); // 2026-11-04 12:00 local
const at = (y: number, mo: number, d: number, h = 0, mi = 0, s = 0) =>
  new Date(y, mo - 1, d, h, mi, s).toISOString();

describe("formatDate", () => {
  it("leaves out the year of this year", () => {
    expect(formatDate("2026-11-07", now)).toBe("11/07");
    expect(formatDate(at(2026, 1, 5, 9), now)).toBe("01/05");
  });

  it("shows the year of another year", () => {
    expect(formatDate("2027-01-29", now)).toBe("2027/01/29");
    expect(formatDate(at(2025, 12, 31, 23), now)).toBe("2025/12/31");
  });
});

describe("formatDateTime", () => {
  it("shows the time too", () => {
    expect(formatDateTime(at(2026, 11, 4, 10, 12), now)).toBe("11/04 10:12");
    expect(formatDateTime(at(2025, 3, 2, 9, 5), now)).toBe("2025/03/02 09:05");
  });
});

describe("formatRelative", () => {
  it("says now, minutes, hours, then the date", () => {
    expect(formatRelative(at(2026, 11, 4, 11, 59, 30), now)).toBe("今");
    expect(formatRelative(at(2026, 11, 4, 12, 0, 30), now)).toBe("今"); // a clock a little ahead
    expect(formatRelative(at(2026, 11, 4, 11, 57), now)).toBe("3 分前");
    expect(formatRelative(at(2026, 11, 4, 10, 0), now)).toBe("2 時間前");
    expect(formatRelative(at(2026, 11, 3, 11, 0), now)).toBe("11/03");
  });
});

describe("initials and avatarTone", () => {
  it("takes the first two characters, in capitals", () => {
    expect(initials("alice")).toBe("AL");
    expect(initials("田中太郎")).toBe("田中");
    expect(initials("  bob ")).toBe("BO");
    expect(initials("x")).toBe("X");
  });

  it("gives the same tone for the same name, within 0 to 3", () => {
    for (const n of ["alice", "bob", "田中太郎", "x", ""]) {
      const t = avatarTone(n);
      expect([0, 1, 2, 3]).toContain(t);
      expect(avatarTone(n)).toBe(t);
    }
    expect(new Set(["a", "b", "c", "d"].map(avatarTone)).size).toBe(4);
  });
});
```

`web/core/schema.test.ts`

```ts
import { describe, expect, it, vi } from "vitest";
import type { Schemas } from "./api/client";
import {
  canParent,
  defaultStatus,
  parentRule,
  propEditorOf,
  SchemaCache,
  type SchemaView,
} from "./schema";
import { FakeConn, fakeApi, settle } from "./testing";

// The schema of a new store (internal/schema Default), as GET /api/schema
// tells it.
const kinds = [
  { name: "project", label: "プロジェクト", role: "container", parents: ["root"] },
  { name: "epic", label: "エピック", role: "container", parents: ["root", "project"] },
  { name: "story", label: "ストーリー", role: "work", parents: ["root", "project", "epic"] },
  {
    name: "task",
    label: "タスク",
    role: "work",
    parents: ["root", "project", "epic", "story", "task"],
    default: true,
  },
  { name: "bug", label: "バグ", role: "work", parents: ["root", "project", "epic", "story"] },
  { name: "note", label: "メモ", role: "work", parents: null },
  { name: "nowhere", role: "work", parents: [] },
] as Schemas["KindDef"][];

const schema: SchemaView = {
  statuses: [
    { name: "backlog", label: "バックログ", category: "todo" },
    { name: "todo", label: "未着手", category: "todo", default: true },
    { name: "in_progress", label: "進行中", category: "doing", default: true },
    { name: "done", label: "完了", category: "done", default: true },
    { name: "canceled", label: "中止", category: "canceled", default: true },
  ],
  kinds,
  props: {
    priority: { type: "enum", values: ["low", "mid", "high", "urgent"] },
    multi: { type: "multi_enum", values: ["a", "b"] },
    start: { type: "date" },
    points: { type: "number" },
    owner: { type: "user" },
    relates: { type: "task_ref", multi: true, label: "関連", symmetric: true },
    estimate: { type: "duration" },
    done_flag: { type: "bool" },
    site: { type: "url" },
    memo: { type: "text" },
    at: { type: "datetime" },
  },
  workflow: { transitions: {}, required: {} },
  scoped: [],
};

describe("canParent", () => {
  it("allows the parents the kind lists, and the top as root", () => {
    expect(canParent(schema, "epic", null)).toBe(true);
    expect(canParent(schema, "epic", "project")).toBe(true);
    expect(canParent(schema, "epic", "task")).toBe(false);
    expect(canParent(schema, "task", "task")).toBe(true);
    expect(canParent(schema, "project", "project")).toBe(false);
    expect(canParent(schema, "project", null)).toBe(true);
  });

  it("allows anywhere when the kind lists no parents, and nowhere for an empty list", () => {
    expect(canParent(schema, "note", "bug")).toBe(true);
    expect(canParent(schema, "note", null)).toBe(true);
    expect(canParent(schema, "nowhere", null)).toBe(false);
  });

  it("refuses a kind the schema does not have", () => {
    expect(canParent(schema, "unknown", null)).toBe(false);
  });
});

describe("parentRule", () => {
  it("names the kinds that can be the parent", () => {
    expect(parentRule(schema, "epic")).toBe("エピックを置ける親はルートとプロジェクトだけです");
    expect(parentRule(schema, "story")).toBe(
      "ストーリーを置ける親はルート、プロジェクトとエピックだけです",
    );
  });

  it("has its own words for the top only, anywhere and nowhere", () => {
    expect(parentRule(schema, "project")).toBe("プロジェクトはルートにだけ置けます");
    expect(parentRule(schema, "note")).toBe("メモはどこにでも置けます");
    expect(parentRule(schema, "nowhere")).toBe("nowhereは親に置けません");
  });
});

describe("defaultStatus", () => {
  it("is the default of the category", () => {
    expect(defaultStatus(schema, "todo")).toBe("todo");
    expect(defaultStatus(schema, "done")).toBe("done");
  });

  it("falls back to the first of the category, and to empty", () => {
    const s: SchemaView = {
      ...schema,
      statuses: [
        { name: "a", category: "todo" },
        { name: "b", category: "todo" },
      ],
    };
    expect(defaultStatus(s, "todo")).toBe("a");
    expect(defaultStatus(s, "done")).toBe("");
  });
});

describe("propEditorOf", () => {
  it("maps the types of the schema", () => {
    expect(propEditorOf(schema, "priority")).toBe("enum");
    expect(propEditorOf(schema, "multi")).toBe("multi_enum");
    expect(propEditorOf(schema, "start")).toBe("date");
    expect(propEditorOf(schema, "points")).toBe("number");
    expect(propEditorOf(schema, "owner")).toBe("user");
    expect(propEditorOf(schema, "relates")).toBe("taskref");
    expect(propEditorOf(schema, "estimate")).toBe("duration");
    expect(propEditorOf(schema, "done_flag")).toBe("bool");
    expect(propEditorOf(schema, "site")).toBe("url");
    expect(propEditorOf(schema, "memo")).toBe("text");
    expect(propEditorOf(schema, "at")).toBe("datetime");
  });

  it("knows the fields every task has", () => {
    expect(propEditorOf(schema, "status")).toBe("status");
    expect(propEditorOf(schema, "kind")).toBe("kind");
    expect(propEditorOf(schema, "assignees")).toBe("user");
    expect(propEditorOf(schema, "watchers")).toBe("user");
    expect(propEditorOf(schema, "blocked_by")).toBe("taskref");
    expect(propEditorOf(schema, "title")).toBe("text");
  });

  it("is null for what the schema does not define", () => {
    expect(propEditorOf(schema, "legacy")).toBeNull();
  });
});

describe("SchemaCache", () => {
  function setup() {
    let current = schema;
    const { api, calls } = fakeApi((r) => (r.path === "/api/schema" ? current : {}));
    const conn = new FakeConn();
    const cache = new SchemaCache({ api, conn: conn.asConnection() });
    return { cache, conn, calls, set: (s: SchemaView) => (current = s) };
  }

  it("loads a store once and keeps it", async () => {
    const { cache, calls } = setup();
    expect(cache.get("shared")).toBeUndefined();
    const [a, b] = await Promise.all([cache.load("shared"), cache.load("shared")]);
    expect(a).toEqual(schema);
    expect(b).toBe(a);
    expect(cache.get("shared")).toBe(a);
    await cache.load("shared");
    expect(calls).toHaveLength(1);
    expect(calls[0]?.query.get("store")).toBe("shared");
  });

  it("fetches again at schema.changed and tells the subscribers", async () => {
    const { cache, conn, set } = setup();
    const first = await cache.load("shared");
    const fn = vi.fn();
    cache.subscribe(fn);

    set({ ...schema, kinds: kinds.slice(0, 2) });
    conn.emit("schema.changed", { store: "shared" });
    await vi.waitFor(() => expect(cache.get("shared")?.kinds?.length).toBe(2));
    expect(cache.get("shared")).not.toBe(first);
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("keeps the same object when the schema did not change", async () => {
    const { cache, conn, calls } = setup();
    const first = await cache.load("shared");
    const fn = vi.fn();
    cache.subscribe(fn);
    conn.emit("resync");
    await vi.waitFor(() => expect(calls).toHaveLength(2));
    await settle();
    expect(cache.get("shared")).toBe(first);
    expect(fn).not.toHaveBeenCalled();
  });

  it("leaves alone a store nobody asked for at schema.changed", async () => {
    const { cache, conn, calls } = setup();
    conn.emit("schema.changed", { store: "other" });
    await settle();
    expect(cache.get("other")).toBeUndefined();
    expect(calls).toHaveLength(0);
  });
});
```

`web/core/stores.test.ts`

```ts
import { describe, expect, it, vi } from "vitest";
import { TaskCache } from "./cache";
import { StoreDirectory } from "./stores";
import { changed, FakeConn, fakeApi, problem, settle, tv } from "./testing";

const store = (name: string, def = false) => ({
  name,
  default: def,
  path: `/data/${name}`,
  remotes: [],
});
const ws = (ulid: string, name: string, prefix: string) => ({
  ulid,
  name,
  id_prefix: prefix,
  prefix_set: "2026-11-01T00:00:00Z",
  tasks: 0,
});
const project = (ulid: string, storeName: string, wsUlid: string, rank: string, title = ulid) =>
  tv(ulid, title, {
    kind: "project",
    store: storeName,
    rank,
    workspace: { ulid: wsUlid, name: "x", id_prefix: "X" },
  });

type World = {
  stores: ReturnType<typeof store>[];
  spaces: Record<string, ReturnType<typeof ws>[]>;
  projects: ReturnType<typeof project>[];
};

function setup(world: World) {
  const { api, calls } = fakeApi((r) => {
    if (r.path === "/api/stores") return world.stores;
    if (r.path === "/api/workspaces") return world.spaces[r.query.get("store") ?? ""] ?? [];
    if (r.path === "/api/tasks") return { tasks: world.projects };
    return {};
  });
  const conn = new FakeConn();
  const cache = new TaskCache({ api, conn: conn.asConnection() });
  const dir = new StoreDirectory({ api, conn: conn.asConnection(), cache });
  return { dir, conn, calls, cache, world };
}

const baseWorld = (): World => ({
  stores: [store("shared", true), store("personal")],
  spaces: {
    shared: [ws("W2", "web", "WEB"), ws("W1", "api", "API")],
    personal: [ws("W3", "home", "HOME")],
  },
  projects: [
    project("P2", "shared", "W1", "b"),
    project("P1", "shared", "W1", "a"),
    project("P3", "personal", "W3", "a"),
  ],
});

describe("StoreDirectory", () => {
  it("lists the workspaces by store order then name, with their projects by rank", async () => {
    const { dir, calls } = setup(baseWorld());
    expect(dir.loaded()).toBe(false);
    await vi.waitFor(() => expect(dir.workspaces().length).toBe(3));
    expect(dir.loaded()).toBe(true);
    expect(dir.workspaces().map((w) => `${w.store}/${w.name}`)).toEqual([
      "shared/api",
      "shared/web",
      "personal/home",
    ]);
    const api = dir.workspace("shared", "W1");
    expect(api?.prefix).toBe("API");
    expect(api?.projects.map((p) => p.ulid)).toEqual(["P1", "P2"]);
    expect(dir.workspace("shared", "W2")?.projects).toEqual([]);
    expect(dir.workspace("personal", "W1")).toBeUndefined();
    const asked = calls
      .filter((c) => c.path === "/api/workspaces")
      .map((c) => c.query.get("store"));
    expect(asked.sort()).toEqual(["personal", "shared"]);
    expect(calls.some((c) => c.path === "/api/tasks" && c.query.get("q") === "kind:project")).toBe(
      true,
    );
  });

  it("gives the colours in the order of the stores, grey for read-only, and wraps", async () => {
    const w = baseWorld();
    w.stores = Array.from({ length: 9 }, (_, i) => store(`s${i}`));
    w.spaces = { s1: [ws("W1", "a", "A")] };
    const { dir, conn } = setup(w);
    await vi.waitFor(() => expect(dir.loaded()).toBe(true));
    expect(dir.tone("s0")).toBe("teal");
    expect(dir.tone("s1")).toBe("orange");
    expect(dir.tone("s6")).toBe("brown");
    expect(dir.tone("s7")).toBe("teal");
    expect(dir.tone("s8")).toBe("orange");
    expect(dir.tone("nobody")).toBe("readonly");

    await vi.waitFor(() => expect(dir.workspace("s1", "W1")?.tone).toBe("orange"));
    const fn = vi.fn();
    dir.subscribe(fn);
    conn.changeSync({ store: "s1", remotes: [], read_only: "no write access" });
    expect(dir.tone("s1")).toBe("readonly");
    expect(fn).toHaveBeenCalled();
    expect(dir.workspace("s1", "W1")?.tone).toBe("readonly");
  });

  it("fetches the stores and workspaces again at tasks.changed and resync", async () => {
    const { dir, conn, world } = setup(baseWorld());
    await vi.waitFor(() => expect(dir.workspaces().length).toBe(3));
    const first = dir.workspaces()[0];
    const fn = vi.fn();
    dir.subscribe(fn);

    world.spaces.shared?.push(ws("W9", "zzz", "ZZZ"));
    conn.emit("tasks.changed", changed("shared", [], "c1", "workspace.add"));
    await vi.waitFor(() => expect(dir.workspaces().length).toBe(4));
    expect(dir.workspaces()[0]).toBe(first); // an entry that did not change is the same object
    expect(fn).toHaveBeenCalled();

    world.stores.push(store("third"));
    world.spaces.third = [ws("W7", "t", "TT")];
    conn.emit("resync");
    await vi.waitFor(() => expect(dir.workspaces().length).toBe(5));
    expect(dir.stores().map((s) => s.name)).toEqual(["shared", "personal", "third"]);
  });

  it("follows the projects through the task cache", async () => {
    const { dir, conn, world } = setup(baseWorld());
    await vi.waitFor(() => expect(dir.workspaces().length).toBe(3));
    const untouched = dir.workspace("personal", "W3");

    world.projects.push(project("P4", "shared", "W2", "a"));
    conn.emit("tasks.changed", changed("shared", ["P4"], "c2"));
    await vi.waitFor(() => expect(dir.workspace("shared", "W2")?.projects.length).toBe(1));
    expect(dir.workspace("personal", "W3")).toBe(untouched);
  });

  it("does not tell the subscribers when a reload brings nothing new", async () => {
    const { dir, conn } = setup(baseWorld());
    await vi.waitFor(() => expect(dir.workspaces().length).toBe(3));
    await settle();
    const fn = vi.fn();
    dir.subscribe(fn);
    conn.emit("resync");
    await settle();
    await settle();
    expect(fn).not.toHaveBeenCalled();
  });

  it("holds why the stores could not be listed", async () => {
    const { api } = fakeApi(() => problem(500, "internal"));
    const conn = new FakeConn();
    const cache = new TaskCache({ api, conn: conn.asConnection() });
    const dir = new StoreDirectory({ api, conn: conn.asConnection(), cache });
    await vi.waitFor(() => expect(dir.error()).not.toBeNull());
    expect(dir.error()?.type).toBe("internal");
    expect(dir.loaded()).toBe(false);
    await expect(dir.reload()).rejects.toThrow();
  });
});
```

`web/core/people.test.ts`

```ts
import { describe, expect, it, vi } from "vitest";
import { matchPeople, PeopleCache } from "./people";
import { changed, FakeConn, fakeApi } from "./testing";

function setup() {
  const world: Record<string, string[]> = { shared: ["alice", "bob"] };
  const { api, calls } = fakeApi((r) => world[r.query.get("store") ?? ""] ?? []);
  const conn = new FakeConn();
  const people = new PeopleCache({ api, conn: conn.asConnection() });
  return { people, conn, calls, world };
}

describe("PeopleCache", () => {
  it("fetches the names of a store once", async () => {
    const { people, calls } = setup();
    expect(people.get("shared")).toBeUndefined();
    const [a, b] = await Promise.all([people.load("shared"), people.load("shared")]);
    expect(a).toEqual(["alice", "bob"]);
    expect(b).toBe(a);
    await people.load("shared");
    expect(calls).toHaveLength(1);
    expect(calls[0]?.path).toBe("/api/people");
    expect(calls[0]?.query.get("store")).toBe("shared");
    expect(people.get("shared")).toBe(a);
  });

  it("keeps the names of each store apart", async () => {
    const { people, world } = setup();
    world.other = ["zed"];
    expect(await people.load("shared")).toEqual(["alice", "bob"]);
    expect(await people.load("other")).toEqual(["zed"]);
  });

  it("fetches again after tasks.changed of that store, and only then", async () => {
    const { people, conn, calls, world } = setup();
    await people.load("shared");
    conn.emit("tasks.changed", changed("other", ["U"], "c1"));
    await people.load("shared");
    expect(calls).toHaveLength(1);

    world.shared = ["alice", "bob", "carol"];
    conn.emit("tasks.changed", changed("shared", ["U"], "c2"));
    expect(people.get("shared")).toEqual(["alice", "bob"]); // stale until the next load
    const fn = vi.fn();
    people.subscribe(fn);
    expect(await people.load("shared")).toEqual(["alice", "bob", "carol"]);
    expect(calls).toHaveLength(2);
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("keeps the same array when the names did not change", async () => {
    const { people, conn } = setup();
    const first = await people.load("shared");
    conn.emit("tasks.changed", changed("shared", ["U"], "c1"));
    expect(await people.load("shared")).toBe(first);
  });

  it("makes everything stale at resync", async () => {
    const { people, conn, calls } = setup();
    await people.load("shared");
    conn.emit("resync");
    await people.load("shared");
    expect(calls).toHaveLength(2);
  });
});

describe("matchPeople", () => {
  const names = ["alice", "Bob", "malice", "carol"];
  it("puts the names that start with the text first, and ignores case", () => {
    expect(matchPeople(names, "al")).toEqual(["alice", "malice"]);
    expect(matchPeople(names, "b")).toEqual(["Bob"]);
    expect(matchPeople(names, "")).toEqual(names);
  });

  it("stops at the limit", () => {
    expect(matchPeople(names, "", 2)).toEqual(["alice", "Bob"]);
  });
});
```

`web/core/search.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { searchTasks, taskSearchQuery } from "./search";
import { fakeApi, tv } from "./testing";

describe("taskSearchQuery", () => {
  it("quotes each word so that it is free text", () => {
    expect(taskSearchQuery("switch dns")).toBe('"switch" "dns"');
    expect(taskSearchQuery("  status:todo  -x ")).toBe('"status:todo" "-x"');
    expect(taskSearchQuery('say "hi"')).toBe('"say" "hi"');
  });

  it("puts the extra query in front", () => {
    expect(taskSearchQuery("rel", "kind:project")).toBe('kind:project "rel"');
    expect(taskSearchQuery("", "kind:project")).toBe("kind:project");
    expect(taskSearchQuery("", "")).toBe("");
  });
});

describe("searchTasks", () => {
  it("asks for the tasks and keeps the first ones", async () => {
    const { api, calls } = fakeApi(() => ({
      tasks: [tv("A"), tv("B"), tv("C")],
    }));
    const found = await searchTasks(api, "rel 8", { store: "shared", limit: 2 });
    expect(found.map((t) => t.ulid)).toEqual(["A", "B"]);
    expect(calls[0]?.query.get("q")).toBe('"rel" "8"');
    expect(calls[0]?.query.get("store")).toBe("shared");
  });

  it("asks for nothing when there is nothing to look for", async () => {
    const { api, calls } = fakeApi(() => ({ tasks: [tv("A")] }));
    expect(await searchTasks(api, "   ")).toEqual([]);
    expect(calls).toHaveLength(0);
  });

  it("looks in every store without store", async () => {
    const { api, calls } = fakeApi(() => ({ tasks: [] }));
    await searchTasks(api, "x", { extra: "-status:canceled" });
    expect(calls[0]?.query.get("q")).toBe('-status:canceled "x"');
    expect(calls[0]?.query.has("store")).toBe(false);
  });
});
```

`web/core/history.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { ApiError } from "./api/client";
import { diffForRestore, showAt } from "./history";
import { fakeApi, problem, showOf, tv } from "./testing";

describe("showAt", () => {
  it("reads the task at a commit", async () => {
    const { api, calls } = fakeApi(() => showOf(tv("A", "old")));
    const s = await showAt(api, "T-A", "abc123", "shared");
    expect(s.title).toBe("old");
    expect(calls[0]?.path).toBe("/api/tasks/T-A/history/abc123");
    expect(calls[0]?.query.get("store")).toBe("shared");
  });

  it("throws restore_at.not_found when the commit has no such task", async () => {
    const { api } = fakeApi(() => problem(404, "restore_at.not_found", { at: "abc123" }));
    const err = await showAt(api, "T-A", "abc123").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).type).toBe("restore_at.not_found");
  });
});

describe("diffForRestore", () => {
  const base = showOf(
    tv("A", "title", {
      assignees: ["alice"],
      labels: ["ops", "web"],
      props: { priority: "high", points: 3 },
      blocked_by: [{ ulid: "B", id: "T-B", title: "b" }],
    }),
    { body: "text" },
  );

  it("is empty for the same task", () => {
    expect(diffForRestore(base, { ...base })).toEqual([]);
  });

  it("is empty when only the order of a list differs", () => {
    const then = { ...base, labels: ["web", "ops"] };
    expect(diffForRestore(base, then)).toEqual([]);
  });

  it("lists only the fields that differ, as now to after", () => {
    const then = showOf(
      tv("A", "older title", {
        status: "done",
        parent: "P1",
        assignees: [],
        labels: ["ops", "web"],
        props: { priority: "high", points: 5, due: "2026-11-07" },
        blocked_by: [],
      }),
      { body: "text" },
    );
    const got = diffForRestore(base, then, {
      idOf: (u) => (u === "P1" ? "REL-1" : u),
      propLabel: (n) => (n === "points" ? "ポイント" : n),
    });
    expect(got).toEqual([
      { field: "title", label: "タイトル", from: "title", to: "older title" },
      { field: "status", label: "ステータス", from: "todo", to: "done" },
      { field: "parent", label: "親", from: "ルート", to: "REL-1" },
      { field: "assignees", label: "担当者", from: "alice", to: "（なし）" },
      { field: "blocked_by", label: "ブロック", from: "T-B", to: "（なし）" },
      { field: "props.due", label: "due", from: "（なし）", to: "2026-11-07" },
      { field: "props.points", label: "ポイント", from: "3", to: "5" },
    ]);
  });

  it("shows a changed body only as changed, and a prop that goes away as none", () => {
    const then = showOf(tv("A", "title", { ...base, props: { points: 3 } }), { body: "other" });
    expect(diffForRestore(base, then)).toEqual([
      { field: "props.priority", label: "priority", from: "high", to: "（なし）" },
      { field: "body", label: "本文", from: "現在の本文", to: "この時点の本文" },
    ]);
  });

  it("joins the items of a list prop", () => {
    const a = showOf(tv("A", "t", { props: { tags: ["x", "y"] } }));
    const b = showOf(tv("A", "t", { props: { tags: ["x"] } }));
    expect(diffForRestore(a, b)).toEqual([
      { field: "props.tags", label: "tags", from: "x、y", to: "x" },
    ]);
  });
});
```

- [ ] **Step 2: 失敗を確かめる**

Run: `cd web && pnpm vitest run core/cli.test.ts core/format.test.ts core/schema.test.ts core/stores.test.ts core/people.test.ts core/search.test.ts core/history.test.ts`

Expected: FAIL（モジュールが見つからない）

- [ ] **Step 3: 実装する**

`web/core/cli.ts`

```ts
// The hh command that does what a dialog or a filter does, shown under it
// for the user to copy (Foundations: the same operation as a command).

const PLAIN = /^[\p{L}\p{N}_\-.,:/@%+=]+$/u;

// quoteArg writes s as one argument of a shell. Only an argument with a
// space or a symbol is put in double quotes; inside them " \ $ and ` are
// escaped with a backslash.
export function quoteArg(s: string): string {
  if (s !== "" && PLAIN.test(s)) return s;
  return `"${s.replace(/["\\$`]/g, "\\$&")}"`;
}

// cliLine is the command for args, without the "$ " that the screen puts in
// front.
export function cliLine(args: string[]): string {
  return ["hh", ...args.map(quoteArg)].join(" ");
}

export function cliAdd(i: {
  title: string;
  parent?: string;
  workspace?: string;
  store?: string;
  kind?: string;
  body?: string;
  props?: Record<string, string>;
}): string {
  const a = ["add", i.title];
  if (i.store) a.push("--store", i.store);
  if (i.workspace) a.push("--workspace", i.workspace);
  if (i.parent) a.push("--parent", i.parent);
  if (i.kind) a.push("--kind", i.kind);
  if (i.body) a.push("--body", i.body);
  for (const [k, v] of Object.entries(i.props ?? {})) a.push("-p", `${k}=${v}`);
  return cliLine(a);
}

export function cliLs(q: string, tree: boolean, store?: string): string {
  const a = ["ls"];
  if (store) a.push("--store", store);
  if (tree) a.push("--tree");
  if (q.trim() !== "") a.push(q);
  return cliLine(a);
}

// cliSet is hh set; a key that ends with + or - adds to or removes from a
// list (labels+ is labels+=x), an empty value clears.
export function cliSet(id: string, assigns: Record<string, string>): string {
  return cliLine(["set", id, ...Object.entries(assigns).map(([k, v]) => `${k}=${v}`)]);
}

export function cliMv(
  id: string,
  to: { parent?: string; root?: boolean; before?: string; after?: string },
): string {
  const a = ["mv", id];
  if (to.parent) a.push("--parent", to.parent);
  if (to.root) a.push("--root");
  if (to.before) a.push("--before", to.before);
  if (to.after) a.push("--after", to.after);
  return cliLine(a);
}

export const cliRm = (id: string): string => cliLine(["rm", id]);
export const cliRestore = (id: string): string => cliLine(["restore", id]);
export const cliPurge = (id: string): string => cliLine(["purge", id]);
export const cliRestoreAt = (id: string, commit: string): string =>
  cliLine(["restore", id, "--at", commit]);

// cliComment is hh comment (add), hh comment edit <comment> or hh comment rm
// <comment>; edit and rm take the ULID of the comment as cid.
export function cliComment(id: string, kind: "add" | "edit" | "rm", cid?: string): string {
  if (kind === "add") return cliLine(["comment", id]);
  return cliLine(["comment", kind, cid ?? id]);
}

export const cliWatch = (id: string, on: boolean): string =>
  cliLine([on ? "watch" : "unwatch", id]);

export function cliSync(store?: string): string {
  return cliLine(store ? ["sync", "--store", store] : ["sync"]);
}

export function cliStoreInit(i: {
  name: string;
  prefix: string;
  workspace?: string;
  remotes?: string[];
}): string {
  const a = ["store", "init", i.name, "--prefix", i.prefix];
  if (i.workspace) a.push("--workspace", i.workspace);
  for (const r of i.remotes ?? []) a.push("--remote", r);
  return cliLine(a);
}
```

`web/core/format.ts`

```ts
// How dates and names are shown (Foundations: dates are Mono; this year
// without the year).

const DATE_ONLY = /^(\d{4})-(\d{2})-(\d{2})$/;

const pad = (n: number): string => String(n).padStart(2, "0");

// parse reads an ISO date (2026-11-07, a date property) as that day in the
// local time zone, and an ISO time as the moment it is.
function parse(iso: string): Date {
  const m = DATE_ONLY.exec(iso);
  return m ? new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])) : new Date(iso);
}

// formatDate: 11/07 this year, 2027/01/29 in another year.
export function formatDate(iso: string, now: Date): string {
  const d = parse(iso);
  const md = `${pad(d.getMonth() + 1)}/${pad(d.getDate())}`;
  return d.getFullYear() === now.getFullYear() ? md : `${d.getFullYear()}/${md}`;
}

// formatDateTime: 11/04 10:12, with the year in another year.
export function formatDateTime(iso: string, now: Date): string {
  const d = parse(iso);
  return `${formatDate(iso, now)} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

// formatRelative: 今, 3 分前, 2 時間前, and the date from a day on.
export function formatRelative(iso: string, now: Date): string {
  const ms = now.getTime() - parse(iso).getTime();
  if (ms < 60_000) return "今";
  if (ms < 3_600_000) return `${Math.floor(ms / 60_000)} 分前`;
  if (ms < 86_400_000) return `${Math.floor(ms / 3_600_000)} 時間前`;
  return formatDate(iso, now);
}

// initials: the first two characters of a name, in capitals for letters.
export function initials(name: string): string {
  return Array.from(name.trim()).slice(0, 2).join("").toUpperCase();
}

// avatarTone picks one of the four colour pairs of the design (tokens
// --avatar-0 ... --avatar-3) from the name, the same each time.
export function avatarTone(name: string): 0 | 1 | 2 | 3 {
  let h = 0;
  for (const c of name) h = (h * 31 + (c.codePointAt(0) ?? 0)) >>> 0;
  return (h % 4) as 0 | 1 | 2 | 3;
}
```

`web/core/schema.ts`

```ts
import { type Api, type Schemas, unwrap } from "./api/client";
import type { Connection } from "./connection";

export type SchemaView = Schemas["SchemaView"];
export type KindDef = Schemas["KindDef"];

// The name that stands for "no parent" in the parents of a kind.
const ROOT = "root";

// SchemaCache keeps the schema.yaml of the stores the screen has asked for,
// and fetches it again when the store says it changed. A schema that did not
// change stays the same object.
export class SchemaCache {
  private readonly api: Api;
  private readonly schemas = new Map<string, SchemaView>();
  private readonly sigs = new Map<string, string>();
  private readonly loading = new Map<string, Promise<SchemaView>>();
  private readonly subs = new Set<() => void>();

  constructor(opts: { api: Api; conn: Connection }) {
    this.api = opts.api;
    opts.conn.on("schema.changed", (e) => this.refetch([e.store]));
    opts.conn.on("resync", () => this.refetch([...this.schemas.keys()]));
  }

  get(store: string): SchemaView | undefined {
    return this.schemas.get(store);
  }

  // load is the schema of the store: the one held, or the one fetched.
  load(store: string): Promise<SchemaView> {
    const have = this.schemas.get(store);
    if (have) return Promise.resolve(have);
    return this.fetch(store);
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  private fetch(store: string): Promise<SchemaView> {
    const running = this.loading.get(store);
    if (running) return running;
    const p = unwrap(this.api.GET("/api/schema", { params: { query: { store } } }))
      .then((s) => {
        const sig = JSON.stringify(s);
        if (this.sigs.get(store) === sig) return this.schemas.get(store) as SchemaView;
        this.schemas.set(store, s);
        this.sigs.set(store, sig);
        for (const fn of [...this.subs]) fn();
        return s;
      })
      .finally(() => {
        if (this.loading.get(store) === p) this.loading.delete(store);
      });
    this.loading.set(store, p);
    return p;
  }

  // refetch fetches again the schemas it holds of stores; one nobody asked
  // for is left alone. A failure leaves the schema held; the next change or
  // resync tries again.
  private refetch(stores: string[]): void {
    for (const store of stores) {
      if (!this.schemas.has(store)) continue;
      this.loading.delete(store); // a fetch on its way may have read the old file
      void this.fetch(store).catch(() => undefined);
    }
  }
}

const kindOf = (s: SchemaView, name: string): KindDef | undefined =>
  (s.kinds ?? []).find((k) => k.name === name);

const kindLabel = (s: SchemaView, name: string): string => {
  const k = kindOf(s, name);
  return k?.label || name;
};

// canParent is whether a task of childKind may sit under a parent of
// parentKind (null: at the top, with no parent). Parents that the kind does
// not list (null) mean anywhere; the top is written "root" in the list.
export function canParent(s: SchemaView, childKind: string, parentKind: string | null): boolean {
  const k = kindOf(s, childKind);
  if (!k) return false;
  if (!k.parents) return true;
  return k.parents.includes(parentKind ?? ROOT);
}

function listJa(names: string[]): string {
  if (names.length <= 1) return names.join("");
  return `${names.slice(0, -1).join("、")}と${names[names.length - 1]}`;
}

// parentRule says where a kind can be put, for the tooltip of a kind that
// cannot be chosen: 「エピックを置ける親はルートとプロジェクトだけです」.
export function parentRule(s: SchemaView, childKind: string): string {
  const label = kindLabel(s, childKind);
  const parents = kindOf(s, childKind)?.parents;
  if (!parents) return `${label}はどこにでも置けます`;
  if (parents.length === 0) return `${label}は親に置けません`;
  if (parents.length === 1 && parents[0] === ROOT) return `${label}はルートにだけ置けます`;
  const names = parents.map((p) => (p === ROOT ? "ルート" : kindLabel(s, p)));
  return `${label}を置ける親は${listJa(names)}だけです`;
}

// defaultStatus is the status a task gets when it moves to the todo or the
// done category: the one the schema marks default, else the first of the
// category. It is "" when the category has none.
export function defaultStatus(s: SchemaView, category: "todo" | "done"): string {
  const of = (s.statuses ?? []).filter((x) => x.category === category);
  return (of.find((x) => x.default) ?? of[0])?.name ?? "";
}

export type PropEditor =
  | "status"
  | "kind"
  | "enum"
  | "multi_enum"
  | "date"
  | "datetime"
  | "number"
  | "bool"
  | "duration"
  | "url"
  | "taskref"
  | "user"
  | "text";

// The fields every task has, and what edits them.
const BUILTIN: Record<string, PropEditor> = {
  status: "status",
  kind: "kind",
  assignees: "user",
  watchers: "user",
  blocked_by: "taskref",
  title: "text",
  labels: "text",
  repos: "text",
  body: "text",
};

// The types of a property of the schema (internal/schema PropType) and the
// editor each gets.
const BY_TYPE: Record<string, PropEditor> = {
  text: "text",
  number: "number",
  bool: "bool",
  date: "date",
  datetime: "datetime",
  duration: "duration",
  enum: "enum",
  multi_enum: "multi_enum",
  user: "user",
  task_ref: "taskref",
  url: "url",
};

// propEditorOf is the editor for the field or property name, or null when
// the schema does not define it.
export function propEditorOf(s: SchemaView, name: string): PropEditor | null {
  const builtin = BUILTIN[name];
  if (builtin) return builtin;
  const p = s.props[name];
  if (!p) return null;
  return BY_TYPE[p.type] ?? "text";
}
```

`web/core/stores.ts`

```ts
import { type Api, ApiError, type Schemas, type TaskView, unwrap } from "./api/client";
import type { TaskCache } from "./cache";
import type { Connection } from "./connection";

export type StoreTone =
  | "teal"
  | "orange"
  | "purple"
  | "sky"
  | "pink"
  | "olive"
  | "brown"
  | "readonly";

export type WorkspaceEntry = {
  store: string;
  tone: StoreTone;
  ulid: string;
  name: string;
  prefix: string;
  projects: TaskView[];
};

// The colours of the stores, in the order of GET /stores; the eighth store
// takes the first colour again. Grey (readonly) is for a store that is
// read-only.
const TONES: StoreTone[] = ["teal", "orange", "purple", "sky", "pink", "olive", "brown"];

const byRank = (a: TaskView, b: TaskView): number =>
  a.rank < b.rank ? -1 : a.rank > b.rank ? 1 : a.ulid < b.ulid ? -1 : a.ulid > b.ulid ? 1 : 0;

function sameEntry(a: WorkspaceEntry, b: WorkspaceEntry): boolean {
  return (
    a.store === b.store &&
    a.tone === b.tone &&
    a.ulid === b.ulid &&
    a.name === b.name &&
    a.prefix === b.prefix &&
    a.projects.length === b.projects.length &&
    a.projects.every((p, i) => p === b.projects[i])
  );
}

// StoreDirectory is what the sidebar and the dialogs list: the stores in
// the order of config.yaml, the workspaces of each, and the projects of each
// workspace. The projects come from the task cache (so they follow
// tasks.changed); the stores and workspaces are fetched again at resync and
// at tasks.changed, since a workspace is added by a commit too.
export class StoreDirectory {
  private readonly api: Api;
  private readonly conn: Connection;
  private readonly cache: TaskCache;
  private items: Schemas["StoreItem"][] = [];
  private spaces = new Map<string, Schemas["WorkspaceView"][]>();
  private isLoaded = false;
  private err: ApiError | null = null;
  private seq = 0;
  private result: WorkspaceEntry[] = [];
  private readOnlySig = "";
  private readonly projects: { ulids: () => string[] };
  private readonly subs = new Set<() => void>();

  constructor(opts: { api: Api; conn: Connection; cache: TaskCache }) {
    this.api = opts.api;
    this.conn = opts.conn;
    this.cache = opts.cache;
    this.projects = opts.cache.watchList({ q: "kind:project", tree: false });
    opts.cache.subscribe(() => this.update());
    opts.conn.subscribe(() => {
      const sig = this.readOnlyNow();
      if (sig !== this.readOnlySig) this.update();
    });
    opts.conn.on("resync", () => void this.reload().catch(() => undefined));
    opts.conn.on("tasks.changed", () => void this.reload().catch(() => undefined));
    void this.reload().catch(() => undefined);
  }

  stores(): Schemas["StoreItem"][] {
    return this.items;
  }

  // tone is the colour of a store; a store that is read-only (its sync state
  // says why) and one that is not known are grey.
  tone(store: string): StoreTone {
    const i = this.items.findIndex((s) => s.name === store);
    if (i < 0 || this.conn.sync.get(store)?.read_only) return "readonly";
    return TONES[i % TONES.length] ?? "readonly";
  }

  // workspaces lists the workspaces of every store: in the order of the
  // stores, then by name.
  workspaces(): WorkspaceEntry[] {
    return this.result;
  }

  workspace(store: string, ulid: string): WorkspaceEntry | undefined {
    return this.result.find((w) => w.store === store && w.ulid === ulid);
  }

  loaded(): boolean {
    return this.isLoaded;
  }

  // error is why the stores could not be listed, when they could not.
  error(): ApiError | null {
    return this.err;
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  // reload fetches the stores and the workspaces of each again. A store whose
  // workspaces cannot be read has none in the list.
  async reload(): Promise<void> {
    const seq = ++this.seq;
    let items: Schemas["StoreItem"][];
    try {
      items = (await unwrap(this.api.GET("/api/stores"))) ?? [];
    } catch (e) {
      if (seq === this.seq) {
        this.err =
          e instanceof ApiError
            ? e
            : new ApiError({
                status: 0,
                type: "network",
                detail: e instanceof Error ? e.message : String(e),
              });
        this.update(true);
      }
      throw e;
    }
    const got = await Promise.all(
      items.map(async (s): Promise<[string, Schemas["WorkspaceView"][]]> => {
        try {
          const ws = await unwrap(
            this.api.GET("/api/workspaces", { params: { query: { store: s.name } } }),
          );
          return [s.name, ws ?? []];
        } catch {
          return [s.name, []];
        }
      }),
    );
    if (seq !== this.seq) return; // a newer reload is on its way
    // last_success changes at each sync and nothing here shows it (the sync
    // card reads sync.state); the list stays as it was when nothing else changed
    const sig = (xs: Schemas["StoreItem"][]) =>
      JSON.stringify(xs.map(({ last_success: _, ...rest }) => rest));
    const same = sig(items) === sig(this.items);
    const spaces = new Map(got);
    const changed =
      !this.isLoaded ||
      this.err !== null ||
      !same ||
      JSON.stringify([...spaces]) !== JSON.stringify([...this.spaces]);
    if (!same) this.items = items;
    this.spaces = spaces;
    this.isLoaded = true;
    this.err = null;
    this.update(changed);
  }

  private readOnlyNow(): string {
    return this.items
      .filter((s) => this.conn.sync.get(s.name)?.read_only)
      .map((s) => s.name)
      .join("\0");
  }

  // update builds the list again, keeping the entries that did not change
  // as they were, and tells the subscribers if the list is not the same.
  private update(force = false): void {
    this.readOnlySig = this.readOnlyNow();
    const projects = this.projects
      .ulids()
      .map((u) => this.cache.task(u))
      .filter((t): t is TaskView => !!t)
      .sort(byRank);
    const next: WorkspaceEntry[] = [];
    for (const s of this.items) {
      const ws = [...(this.spaces.get(s.name) ?? [])].sort((a, b) =>
        a.name.localeCompare(b.name, "ja"),
      );
      for (const w of ws) {
        const entry: WorkspaceEntry = {
          store: s.name,
          tone: this.tone(s.name),
          ulid: w.ulid,
          name: w.name,
          prefix: w.id_prefix,
          projects: projects.filter((p) => p.store === s.name && p.workspace.ulid === w.ulid),
        };
        const prev = this.result.find((e) => e.store === entry.store && e.ulid === entry.ulid);
        next.push(prev && sameEntry(prev, entry) ? prev : entry);
      }
    }
    const same = next.length === this.result.length && next.every((e, i) => e === this.result[i]);
    if (!same) this.result = next;
    if (!same || force) for (const fn of [...this.subs]) fn();
  }
}
```

`web/core/people.ts`

```ts
import { type Api, unwrap } from "./api/client";
import type { Connection } from "./connection";

// PeopleCache keeps the names that @ offers, by store: the people on the
// tasks of the store and the authors of its commits (GET /api/people). A
// change in the store (tasks.changed: a comment or an assignee may be new)
// makes the names stale; the next load fetches them again.
export class PeopleCache {
  private readonly api: Api;
  private readonly names = new Map<string, string[]>();
  private readonly stale = new Set<string>();
  private readonly loading = new Map<string, Promise<string[]>>();
  private readonly subs = new Set<() => void>();

  constructor(opts: { api: Api; conn: Connection }) {
    this.api = opts.api;
    opts.conn.on("tasks.changed", (e) => this.stale.add(e.store));
    opts.conn.on("resync", () => {
      for (const store of this.names.keys()) this.stale.add(store);
    });
  }

  // get is the names last read, which may be stale.
  get(store: string): string[] | undefined {
    return this.names.get(store);
  }

  // load is the names of the store: the ones held while they are fresh,
  // else the ones fetched.
  load(store: string): Promise<string[]> {
    const have = this.names.get(store);
    if (have && !this.stale.has(store)) return Promise.resolve(have);
    const running = this.loading.get(store);
    if (running) return running;
    this.stale.delete(store);
    const p = unwrap(this.api.GET("/api/people", { params: { query: { store } } }))
      .then((list) => {
        const next = list ?? [];
        // A change that came while this was on its way makes the next load fetch again.
        const old = this.names.get(store);
        if (old && old.length === next.length && old.every((n, i) => n === next[i])) return old;
        this.names.set(store, next);
        for (const fn of [...this.subs]) fn();
        return next;
      })
      .finally(() => {
        if (this.loading.get(store) === p) this.loading.delete(store);
      });
    this.loading.set(store, p);
    return p;
  }

  subscribe(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }
}

// matchPeople is the names that start with, or (after the first) contain,
// what was typed after @, case ignored; the ones that start with it come
// first.
export function matchPeople(names: readonly string[], typed: string, limit = 8): string[] {
  const t = typed.toLowerCase();
  const starts: string[] = [];
  const contains: string[] = [];
  for (const n of names) {
    const i = n.toLowerCase().indexOf(t);
    if (i === 0) starts.push(n);
    else if (i > 0) contains.push(n);
  }
  return [...starts, ...contains].slice(0, limit);
}
```

`web/core/search.ts`

```ts
import { type Api, type TaskView, unwrap } from "./api/client";

// taskSearchQuery is the query for the words typed in a search box (the #
// completion, the command palette, the parent and blocker pickers): every
// word is free text that has to be in the ID or the title. Each word is
// quoted, so that a word with a colon or a leading - is not read as a key
// or as a negation. Empty input gives "".
export function taskSearchQuery(text: string, extra = ""): string {
  const words = text
    .split(/\s+/)
    .map((w) => w.replaceAll('"', ""))
    .filter((w) => w !== "");
  return [extra.trim(), ...words.map((w) => `"${w}"`)].filter((t) => t !== "").join(" ");
}

// searchTasks finds the tasks whose ID or title has all the words, in every
// store (or in one). extra is more query ("kind:project", "-status:canceled").
// Nothing is asked for empty input and an empty extra.
export async function searchTasks(
  api: Api,
  text: string,
  opts: { store?: string; extra?: string; limit?: number } = {},
): Promise<TaskView[]> {
  const q = taskSearchQuery(text, opts.extra);
  if (q === "") return [];
  const out = await unwrap(api.GET("/api/tasks", { params: { query: { q, store: opts.store } } }));
  return (out.tasks ?? []).slice(0, opts.limit ?? 8);
}
```

`web/core/history.ts`

```ts
import { type Api, type ShowView, unwrap } from "./api/client";

// showAt is the task as it was at the commit (GET /api/tasks/{id}/history/
// {commit}). The commit has no task: 404 restore_at.not_found.
export function showAt(api: Api, id: string, commit: string, store?: string): Promise<ShowView> {
  return unwrap(
    api.GET("/api/tasks/{id}/history/{commit}", {
      params: { path: { id, commit }, query: { store } },
    }),
  );
}

export type RestoreChange = {
  field: string; // title, kind, status, parent, assignees, labels, blocked_by, body, props.<name>
  label: string;
  from: string; // now
  to: string; // after the restore
};

const NONE = "（なし）";

const shown = (v: unknown): string => {
  if (v === null || v === undefined || v === "") return NONE;
  if (Array.isArray(v)) return v.length === 0 ? NONE : v.map(shown).join("、");
  if (typeof v === "object") return JSON.stringify(v);
  return String(v);
};

const sorted = (xs: readonly string[] | null | undefined): string[] => [...(xs ?? [])].sort();
const sameSet = (
  a: readonly string[] | null | undefined,
  b: readonly string[] | null | undefined,
) => JSON.stringify(sorted(a)) === JSON.stringify(sorted(b));

// diffForRestore lists what a restore to `then` changes in `now`, the fields
// that differ and no others: the title, kind, status, parent, assignees,
// labels, blockers, props, and the body (shown only as changed). Nothing in
// the list means the task is the same as at that time. idOf shows the ULID of
// a parent as its ID; propLabel is the label of a property.
export function diffForRestore(
  now: ShowView,
  then: ShowView,
  opts: { idOf?: (ulid: string) => string; propLabel?: (name: string) => string } = {},
): RestoreChange[] {
  const idOf = opts.idOf ?? ((u: string) => u);
  const propLabel = opts.propLabel ?? ((n: string) => n);
  const out: RestoreChange[] = [];
  const scalar = (field: string, label: string, a: string, b: string) => {
    if (a !== b) out.push({ field, label, from: shown(a), to: shown(b) });
  };
  scalar("title", "タイトル", now.title, then.title);
  scalar("kind", "種類", now.kind, then.kind);
  scalar("status", "ステータス", now.status, then.status);
  if ((now.parent ?? null) !== (then.parent ?? null)) {
    out.push({
      field: "parent",
      label: "親",
      from: now.parent ? idOf(now.parent) : "ルート",
      to: then.parent ? idOf(then.parent) : "ルート",
    });
  }
  const list = (field: string, label: string, a: string[] | null, b: string[] | null) => {
    if (!sameSet(a, b)) out.push({ field, label, from: shown(a), to: shown(b) });
  };
  list("assignees", "担当者", now.assignees, then.assignees);
  list("labels", "ラベル", now.labels, then.labels);
  list(
    "blocked_by",
    "ブロック",
    (now.blocked_by ?? []).map((r) => r.id),
    (then.blocked_by ?? []).map((r) => r.id),
  );
  const keys = [...new Set([...Object.keys(now.props ?? {}), ...Object.keys(then.props ?? {})])];
  for (const k of keys.sort()) {
    const a = now.props?.[k] ?? null;
    const b = then.props?.[k] ?? null;
    if (JSON.stringify(a) !== JSON.stringify(b)) {
      out.push({ field: `props.${k}`, label: propLabel(k), from: shown(a), to: shown(b) });
    }
  }
  if ((now.body ?? "") !== (then.body ?? "")) {
    out.push({ field: "body", label: "本文", from: "現在の本文", to: "この時点の本文" });
  }
  return out;
}
```

- [ ] **Step 4: 通ることを確かめる**

Run: `cd web && pnpm vitest run core/cli.test.ts core/format.test.ts core/schema.test.ts core/stores.test.ts core/people.test.ts core/search.test.ts core/history.test.ts`

Expected: PASS（7 ファイル・53 テスト）

Run: `cd web && pnpm exec biome ci core && pnpm exec svelte-check --fail-on-warnings`

Expected: 指摘ゼロ

- [ ] **Step 5: コミット**

```bash
git add web/core/cli.ts web/core/format.ts web/core/schema.ts web/core/stores.ts web/core/people.ts web/core/search.ts web/core/history.ts web/core/cli.test.ts web/core/format.test.ts web/core/schema.test.ts web/core/stores.test.ts web/core/people.test.ts web/core/search.test.ts web/core/history.test.ts
git commit -m "feat(web): CLI の文字列・Store とスキーマ・人とタスクの検索・時点の差分を足す"
```

---

### 実装で plan から変えた点（2026-11-02、レビューのあと）

この plan の各 Task のコードは plan を書いた時点のもの。レビューの指摘で次を変えた（公開の型と関数のシグネチャは変えていない）。

- `messages.ts`: `task.moved`（`store`・`id` があれば移った先と今の ID を出す）・`restore.forward`・`sync.force_push`・`network`（「サーバーに接続できません」）の文言を足した
- 購読者とイベントのリスナーは `listeners.ts` の `callAll`（と `Connection` の `emit` の try/catch）で呼ぶ。投げたリスナーは `console.error` に出すだけで、接続や保存の状態を壊さず、ほかのリスナーも呼ぶ。`/api/events` が `ok` でないときは本体を `cancel` する
- `cache.ts`: 取得を始めたあとに書き込み・purge・`tasks.changed` の `removed` があった ULID は、古い応答で上書きも復活もしない（ULID ごとの書き込みの記録）。一覧・ツリーからも外す。詳細の応答が古ければ取り直す（自分のコミットのイベントが PATCH の応答より先に届く順序）。変化のない詳細の取得では通知しない。解放した `watchList` の応答は取り込まない。`watchList` は `tree: false`、`watchTree` は `tree: true` に固定。`setBody` を足した
- `save.ts`: 本文を保存したら、送った本文を詳細に入れてから「編集中」を外す。待っている編集が残っていれば `saved` にせず `saving` のまま。`send` の途中の例外でチェーンと `sending` を残さない。4xx（408・409・429 を除く）で拒否されたフィールドは、エラーのまま残してやり直し（`flush` でそれだけが残っているとき、またはそのフィールドの編集）で送り、ほかのフィールドの保存には混ぜない。通信の失敗・5xx・408・409・429 は従来どおり混ぜてやり直す
- `stores.ts`: `tasks.changed` による取り直しを microtask でまとめる。ワークスペースの取得に失敗した Store は前のワークスペースを残す。最初の `hello` の resync は省かない（コンストラクタの取得と SSE の購読のあいだの変更を拾うため）
- `people.ts`・`schema.ts`: 取得に成功したときだけ新しいとみなし、取得中の `tasks.changed` のあとは取り直す。Store ごとの連番で、古い取得が新しい結果を上書きしない
- `cli.ts`: 利用者の入力が位置引数になるもの（`cliAdd` のタイトル、`cliLs` のクエリ、`cliStoreInit` の名前）は、フラグを先に並べ、位置引数が `-` で始まるときはその前に `--` を置く（`hh ls -status:canceled` はフラグとして拒否されるため。既定のフィルタの足元は `hh ls -- -status:canceled`）。`!` を含む引数と `=` で始まる引数は単引用符で囲む（シェルの履歴展開と zsh の `=` 展開を避ける）
- `Makefile` の `e2e` のコメントは、CI のジョブ（Task 16a で足す）を指さない文にした
- CI（PR #30 で利用者の指示）: `ci.yaml` を `ci-backend.yaml` に改名し、`web` ジョブを `ci-frontend.yaml` に移した。ワークフローのステップの間は1行空ける。`jdx/mise-action` の `install_args` は `mise.toml` のキー（`github:gotestyourself/gotestsum`）で書く。リリースの `mise-action` は `cache: false`（zizmor の cache-poisoning）
- ツールの版（同）: Node 26.13.0、TypeScript 6.0.3（7 は svelte-check が未対応。openapi-typescript の peer は `peerDependencyRules` で 6 を許す）、`@types/node` 26 系。公開から 7 日未満の版は入れない（`minimumReleaseAge: 10080`）ので、各ライブラリは 7 日たった中で最新（biome 2.6.3・svelte 5.57.2・@tanstack/svelte-virtual 3.13.48・@types/node 26.6.5）。`package.json` に `engines.node: ">=26"`
- `pnpm-workspace.yaml` と `biome.json` は kukv/koto の設定に合わせた（`trustPolicy`・`strictDepBuilds`・`blockExoticSubdeps`・`strictPeerDependencies`、Biome の `vcs.useIgnoreFile` と対象のファイルの列挙、`noNonNullAssertion: warn`）。minimatch は脆弱性（GHSA-g3hv-75wg-m249）の修正版 11.0.1 に `overrides` で上げ、7 日を待たずに入れる。修正版のない deepmerge（GHSA-gcqx-w48m-w2h4）は `web/osv-scanner.toml` で 2027-02-01 まで無視する
- Renovate は bright-room/renovate-config の既定・`:go`・`:typescript` だけを使う（自前のルールは書かない）
