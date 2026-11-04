import { type ChildProcess, execFileSync, spawn } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { basename, delimiter, dirname, join, resolve } from "node:path";
import { test as base, expect, type Page } from "@playwright/test";

// The binary under test. `make e2e` builds it into bin/hedgehug-e2e (the build with the e2e tag,
// whose keyring is a file in HEDGEHUG_HOME, not the OS keychain); HEDGEHUG_BIN points at another one.
const BIN = resolve(
  process.env.HEDGEHUG_BIN ?? resolve(import.meta.dirname, "../../bin/hedgehug-e2e"),
);

// The fake of GitHub, which a test with github: true starts. `make e2e` builds it into bin/fakegh.
const FAKEGH = resolve(
  process.env.HEDGEHUG_FAKEGH ?? resolve(import.meta.dirname, "../../bin/fakegh"),
);

/** remote: the store gets a bare repository of its own as its remote, for a test with a second user. */
export type StoreSpec = { name: string; prefix: string; workspace?: string; remote?: boolean };

/** The login of the account of the page; the author of the commits the page sees. */
export const LOGIN = "e2e";

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
    /** The path of the bare repository the store pushes to, when the store has one. */
    readonly remote: string | null = null,
    /** The fake of GitHub started for this test (github: true), or null. */
    private readonly gh: FakeGitHub | null = null,
  ) {}

  /** The environment of every hh process of this test. */
  static env(home: string): NodeJS.ProcessEnv {
    return {
      ...process.env,
      PATH: `${Hedgehug.binDir(home)}${delimiter}${process.env.PATH ?? ""}`,
      HEDGEHUG_HOME: join(home, "hh"),
      HEDGEHUG_AUTO_SYNC: "0",
    };
  }

  /**
   * A directory with the binary under test as `hedgehug`: the merge driver of a Store runs it from PATH,
   * and hh doctor reports a Store whose PATH has none.
   */
  private static binDir(home: string): string {
    if (basename(BIN) === "hedgehug") return dirname(BIN);
    const dir = join(home, "bin");
    if (!existsSync(join(dir, "hedgehug"))) {
      mkdirSync(dir, { recursive: true });
      symlinkSync(BIN, join(dir, "hedgehug"));
    }
    return dir;
  }

  /** Runs hh and returns its standard output. A non-zero exit throws. */
  hh(...args: string[]): string {
    return execFileSync(BIN, args, {
      cwd: this.home, // not inside a repository, so hh add does not pick a project from it
      env: Hedgehug.env(this.home),
      encoding: "utf8",
    });
  }

  /** Runs hh with the text on its standard input and returns its standard output. A non-zero exit throws. */
  hhIn(input: string, ...args: string[]): string {
    return execFileSync(BIN, args, {
      cwd: this.home,
      env: Hedgehug.env(this.home),
      encoding: "utf8",
      input,
    });
  }

  /** The URL of the fake of GitHub (github: true). */
  get fakeGitHubURL(): string {
    if (!this.gh) throw new Error("the fake of GitHub is not running: use github: true");
    return this.gh.url;
  }

  /** Runs fakegh against the fake of this test (github: true) and returns its standard output. */
  fakegh(...args: string[]): string {
    return execFileSync(FAKEGH, [...args, "--server", this.fakeGitHubURL], {
      cwd: this.home,
      encoding: "utf8",
    });
  }

  /** Runs hh with --json and parses the output. */
  json<T>(...args: string[]): T {
    return JSON.parse(this.hh(...args, "--json")) as T;
  }

  /** GETs the API, as the page does. */
  async get<T>(path: string): Promise<T> {
    const r = await fetch(`${this.origin}${path}`, {
      headers: { Authorization: `Bearer ${this.token}` },
    });
    if (!r.ok) throw new Error(`GET ${path}: ${r.status} ${await r.text()}`);
    return (await r.json()) as T;
  }

  /** The path of the tree tab of the first workspace of the first store, which "/" is the overview of. */
  async treePath(): Promise<string> {
    const stores = await this.get<{ name: string }[]>("/api/stores");
    const store = stores[0]?.name;
    if (!store) throw new Error("hh ui has no store");
    const spaces = await this.get<{ ulid: string }[]>(
      `/api/workspaces?store=${encodeURIComponent(store)}`,
    );
    const space = spaces[0];
    if (!space) throw new Error(`the store ${store} has no workspace`);
    return `/workspaces/${encodeURIComponent(store)}/${space.ulid}/tree`;
  }

  /** Opens the tree tab of the first workspace, and waits until the page is live. */
  async openTree(page: Page): Promise<void> {
    await this.open(page, await this.treePath());
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
    if (this.proc.exitCode === null && this.proc.signalCode === null) {
      const exited = new Promise<void>((done) => this.proc.once("exit", () => done()));
      this.proc.kill("SIGINT");
      const timer = setTimeout(() => this.proc.kill("SIGKILL"), 5000);
      await exited;
      clearTimeout(timer);
    }
    await this.gh?.stop();
    rmSync(this.home, { recursive: true, force: true });
  }
}

/** One fakegh process. */
class FakeGitHub {
  private constructor(
    readonly url: string,
    private readonly proc: ChildProcess,
  ) {}

  /** Starts fakegh on a free port with the seed and waits for its URL. */
  static async start(home: string): Promise<FakeGitHub> {
    // One user, e2e (LOGIN); device_interval 0 so that the login does not wait between polls.
    // auto_approve stays off: the test approves the code, to see the screen that waits for it.
    const seed = join(home, "fakegh-seed.json");
    writeFileSync(seed, JSON.stringify({ users: [LOGIN], device_interval: 0 }));
    const proc = spawn(FAKEGH, ["serve", "--addr", "127.0.0.1:0", "--seed", seed], { cwd: home });
    let out = "";
    let err = "";
    proc.stderr.on("data", (c: Buffer) => {
      err += c.toString();
    });
    const url = await new Promise<string>((resolveURL, reject) => {
      const timer = setTimeout(
        () => reject(new Error(`fakegh printed no URL in 15s\n${err}`)),
        15000,
      );
      proc.once("error", (e) => {
        clearTimeout(timer);
        reject(new Error(`cannot run ${FAKEGH}: ${e.message} (run make e2e-bin to build it)`));
      });
      proc.once("exit", (code) => {
        clearTimeout(timer);
        reject(new Error(`fakegh exited with ${code} before it printed its URL\n${err}`));
      });
      proc.stdout.on("data", (c: Buffer) => {
        out += c.toString();
        const m = /^fakegh listening on (\S+)$/m.exec(out);
        if (m?.[1]) {
          clearTimeout(timer);
          resolveURL(m[1]);
        }
      });
    }).catch((e) => {
      proc.kill("SIGKILL");
      throw e;
    });
    return new FakeGitHub(url, proc);
  }

  async stop(): Promise<void> {
    if (this.proc.exitCode !== null || this.proc.signalCode !== null) return;
    const exited = new Promise<void>((done) => this.proc.once("exit", () => done()));
    this.proc.kill("SIGINT");
    const timer = setTimeout(() => this.proc.kill("SIGKILL"), 5000);
    await exited;
    clearTimeout(timer);
  }
}

/**
 * A second user of the store of a Hedgehug that has a remote: the CLI on a home and an account of
 * their own (login "bob"), with the same store added from the remote.
 */
export class Peer {
  private constructor(readonly home: string) {}

  static start(owner: Hedgehug, name: string): Peer {
    if (!owner.remote) throw new Error("the store has no remote: use store: { remote: true }");
    const home = mkdtempSync(join(tmpdir(), "hh-e2e-peer-"));
    mkdirSync(join(home, "hh"));
    writeFileSync(
      join(home, "hh", "config.yaml"),
      "accounts:\n  - name: bob\n    host: github.com\n    login: bob\n",
    );
    const peer = new Peer(home);
    try {
      peer.hh("store", "add", name, "--remote", owner.remote);
    } catch (e) {
      peer.stop();
      throw e;
    }
    return peer;
  }

  /** Runs hh as the second user and returns its standard output. */
  hh(...args: string[]): string {
    return execFileSync(BIN, args, {
      cwd: this.home,
      env: Hedgehug.env(this.home),
      encoding: "utf8",
    });
  }

  stop(): void {
    rmSync(this.home, { recursive: true, force: true });
  }
}

/** Starts hh ui on a new home with the store, or with no store when it is null. */
export async function startHedgehug(store: StoreSpec | null, github = false): Promise<Hedgehug> {
  const home = mkdtempSync(join(tmpdir(), "hh-e2e-"));
  // github: the fake of GitHub is the github.com of this home, before hh ui reads the config.
  let gh: FakeGitHub | null = null;
  if (github) {
    try {
      gh = await FakeGitHub.start(home);
    } catch (e) {
      rmSync(home, { recursive: true, force: true });
      throw e;
    }
  }
  const env = Hedgehug.env(home);
  let remote: string | null = null;
  try {
    // hh needs an account; the author of the commits is its login.
    mkdirSync(join(home, "hh"));
    const hosts = gh
      ? `hosts:\n  - host: github.com\n    api: ${gh.url}\n    web: ${gh.url}\n    client_id: e2e\n`
      : "";
    writeFileSync(
      join(home, "hh", "config.yaml"),
      `accounts:\n  - name: ${LOGIN}\n    host: github.com\n    login: ${LOGIN}\n${hosts}`,
    );
    if (store) {
      const args = ["store", "init", store.name, "--prefix", store.prefix];
      if (store.workspace) args.push("--workspace", store.workspace);
      if (store.remote) {
        remote = join(home, "remote.git");
        execFileSync("git", ["init", "--bare", "--quiet", remote], { encoding: "utf8" });
        args.push("--remote", remote);
      }
      execFileSync(BIN, args, { cwd: home, env, encoding: "utf8" });
      // The remote gets the first commits, so that a second user can add the store from it.
      if (remote) execFileSync(BIN, ["sync"], { cwd: home, env, encoding: "utf8" });
    }
  } catch (e) {
    await gh?.stop();
    rmSync(home, { recursive: true, force: true });
    throw e;
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
    await gh?.stop();
    rmSync(home, { recursive: true, force: true });
    throw e;
  });

  const u = new URL(url);
  const token = new URLSearchParams(u.hash.slice(1)).get("token");
  if (!token) {
    proc.kill("SIGKILL");
    await gh?.stop();
    rmSync(home, { recursive: true, force: true });
    throw new Error(`no token in ${url}`);
  }
  return new Hedgehug(home, u.origin, token, proc, () => err, remote, gh);
}

type Fixtures = {
  /** The store hh starts with; null starts hh with no store at all. */
  store: StoreSpec | null;
  /** true starts the fake of GitHub (bin/fakegh) as github.com for this test: Device Flow and pasted tokens. */
  github: boolean;
  hh: Hedgehug;
  /** A second user of the store, which needs store: { ...DEFAULT_STORE, remote: true }. */
  peer: Peer;
};

export const test = base.extend<Fixtures>({
  store: [DEFAULT_STORE, { option: true }],
  github: [false, { option: true }],
  hh: async ({ store, github }, use) => {
    const h = await startHedgehug(store, github);
    try {
      await use(h);
    } finally {
      await h.stop();
    }
  },
  peer: async ({ hh, store }, use) => {
    const p = Peer.start(hh, store?.name ?? DEFAULT_STORE.name);
    try {
      await use(p);
    } finally {
      p.stop();
    }
  },
});

export { expect };
