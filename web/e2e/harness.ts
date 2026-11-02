import { type ChildProcess, execFileSync, spawn } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { test as base, expect, type Page } from "@playwright/test";

// The binary under test. `make e2e` builds it into bin/hedgehug; HEDGEHUG_BIN
// points at another one.
const BIN = process.env.HEDGEHUG_BIN ?? resolve(import.meta.dirname, "../../bin/hedgehug");

export type StoreSpec = { name: string; prefix: string; workspace?: string };

/** The store every test starts with, unless it says store: null. */
export const DEFAULT_STORE: StoreSpec = { name: "demo", prefix: "DM" };

export type TaskJson = {
  ulid: string;
  id: string;
  title: string;
  kind: string;
  status: string;
  parent: string | null;
  rank: string;
  labels: string[];
  props: Record<string, unknown>;
  deleted_at: string | null;
  body?: string;
  history?: unknown[];
};

/**
 * One hh ui process on its own HEDGEHUG_HOME, and the CLI against the same
 * home, so that a test can change the store the way a terminal would.
 */
export class Hedgehug {
  constructor(
    readonly home: string,
    /** http://127.0.0.1:<port> */
    readonly origin: string,
    readonly token: string,
    private readonly proc: ChildProcess,
    readonly stderr: () => string,
  ) {}

  /** The environment of every hh process of this test. */
  static env(home: string): NodeJS.ProcessEnv {
    return {
      ...process.env,
      HEDGEHUG_HOME: join(home, "hh"),
      HEDGEHUG_AUTO_SYNC: "0",
      // The author of the commits comes from the global git config.
      HOME: join(home, "user"),
      GIT_CONFIG_GLOBAL: join(home, "user", ".gitconfig"),
    };
  }

  /** Runs hh and returns its standard output. A non-zero exit throws. */
  hh(...args: string[]): string {
    return execFileSync(BIN, args, {
      cwd: this.home, // not inside a repository, so hh add does not pick a project from it
      env: Hedgehug.env(this.home),
      encoding: "utf8",
    });
  }

  /** Runs hh with --json and parses the output. */
  json<T>(...args: string[]): T {
    return JSON.parse(this.hh(...args, "--json")) as T;
  }

  add(title: string, ...args: string[]): TaskJson {
    return this.json<TaskJson>("add", title, ...args);
  }

  show(id: string): TaskJson {
    return this.json<TaskJson>("show", id);
  }

  /**
   * Calls the API of this server, as the page does. For a test that needs many tasks:
   * one request is much quicker than one hh process.
   */
  async post<T>(path: string, body: unknown): Promise<T> {
    const r = await fetch(`${this.origin}${path}`, {
      method: "POST",
      headers: { Authorization: `Bearer ${this.token}`, "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!r.ok) throw new Error(`POST ${path}: ${r.status} ${await r.text()}`);
    return (await r.json()) as T;
  }

  /** Opens the page the way hh ui's URL does, and waits until the page is live. */
  async open(page: Page, path = "/"): Promise<void> {
    // The page sends PUT /api/presence once the hello of the event stream
    // arrived. After that a change made by the CLI reaches the page as an event.
    const live = page.waitForRequest(
      (r) => r.method() === "PUT" && new URL(r.url()).pathname === "/api/presence",
    );
    await page.goto(`${this.origin}${path}#token=${this.token}`);
    await live;
  }

  async stop(): Promise<void> {
    if (this.proc.exitCode === null) {
      const exited = new Promise<void>((done) => this.proc.once("exit", () => done()));
      this.proc.kill("SIGINT");
      const timer = setTimeout(() => this.proc.kill("SIGKILL"), 5000);
      await exited;
      clearTimeout(timer);
    }
    rmSync(this.home, { recursive: true, force: true });
  }
}

/** Starts hh ui on a new home with the store, or with no store when it is null. */
export async function startHedgehug(store: StoreSpec | null): Promise<Hedgehug> {
  const home = mkdtempSync(join(tmpdir(), "hh-e2e-"));
  mkdirSync(join(home, "user"));
  writeFileSync(
    join(home, "user", ".gitconfig"),
    "[user]\n\tname = E2E\n\temail = e2e@example.com\n",
  );
  const env = Hedgehug.env(home);
  if (store) {
    const args = ["store", "init", store.name, "--prefix", store.prefix];
    if (store.workspace) args.push("--workspace", store.workspace);
    execFileSync(BIN, args, { cwd: home, env, encoding: "utf8" });
  }

  const proc = spawn(BIN, ["ui", "--no-open"], { cwd: home, env });
  let out = "";
  let err = "";
  proc.stderr.on("data", (c: Buffer) => {
    err += c.toString();
  });
  const url = await new Promise<string>((resolveURL, reject) => {
    const timer = setTimeout(() => reject(new Error(`hh ui printed no URL in 15s\n${err}`)), 15000);
    proc.once("exit", (code) => {
      clearTimeout(timer);
      reject(new Error(`hh ui exited with ${code} before it printed its URL\n${err}`));
    });
    proc.stdout.on("data", (c: Buffer) => {
      out += c.toString();
      const m = /^Hedgehug UI: (\S+)$/m.exec(out);
      if (m?.[1]) {
        clearTimeout(timer);
        resolveURL(m[1]);
      }
    });
  }).catch(async (e) => {
    proc.kill("SIGKILL");
    rmSync(home, { recursive: true, force: true });
    throw e;
  });

  const u = new URL(url);
  const token = new URLSearchParams(u.hash.slice(1)).get("token");
  if (!token) throw new Error(`no token in ${url}`);
  return new Hedgehug(home, u.origin, token, proc, () => err);
}

type Fixtures = {
  /** The store hh starts with; null starts hh with no store at all. */
  store: StoreSpec | null;
  hh: Hedgehug;
};

export const test = base.extend<Fixtures>({
  store: [DEFAULT_STORE, { option: true }],
  hh: async ({ store }, use) => {
    const h = await startHedgehug(store);
    try {
      await use(h);
    } finally {
      await h.stop();
    }
  },
});

export { expect };
