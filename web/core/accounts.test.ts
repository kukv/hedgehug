import { describe, expect, it, vi } from "vitest";
import { AccountsCache, envVar, source } from "./accounts";
import type { Schemas } from "./api/client";
import { FakeConn, fakeApi, problem, settle } from "./testing";

type Auth = Schemas.AuthStatus;
const acct = (name: string, extra: Partial<Auth> = {}): Auth => ({
  name,
  id: `github.com/${name}`,
  login: name,
  state: "ok",
  ...extra,
});

function setup(first: () => unknown = () => [acct("alice")]) {
  let answer = first;
  const { api, calls } = fakeApi((r) => (r.path === "/api/accounts" ? answer() : {}));
  const conn = new FakeConn();
  const cache = new AccountsCache({ api, conn: conn.asConnection() });
  const asked = () => calls.filter((c) => c.path === "/api/accounts").length;
  return { cache, conn, asked, answer: (fn: () => unknown) => (answer = fn) };
}

describe("AccountsCache", () => {
  it("does not read before load, and load reads once", async () => {
    const t = setup();
    await settle();
    expect(t.asked()).toBe(0);
    expect(t.cache.loaded()).toBe(false);

    await Promise.all([t.cache.load(), t.cache.load()]);
    await t.cache.load();
    expect(t.asked()).toBe(1);
    expect(t.cache.loaded()).toBe(true);
    expect(t.cache.accounts().map((a) => a.name)).toEqual(["alice"]);
    expect(t.cache.error()).toBeNull();
  });

  it("reload reads again and tells the subscribers when the list changed", async () => {
    const t = setup();
    await t.cache.load();
    const fn = vi.fn();
    t.cache.subscribe(fn);
    await t.cache.reload();
    expect(t.asked()).toBe(2);
    expect(fn).not.toHaveBeenCalled(); // the same list

    t.answer(() => [acct("alice"), acct("bob")]);
    await t.cache.reload();
    expect(t.cache.accounts().map((a) => a.name)).toEqual(["alice", "bob"]);
    expect(fn).toHaveBeenCalledTimes(1);
  });

  it("drops the result of an older reload when a newer one has started", async () => {
    let release!: () => void;
    const gate = new Promise<void>((r) => {
      release = r;
    });
    let n = 0;
    const { api } = fakeApi(async (r) => {
      if (r.path !== "/api/accounts") return {};
      n++;
      if (n === 1) {
        await gate;
        return [acct("old")];
      }
      return [acct("new")];
    });
    const cache = new AccountsCache({ api, conn: new FakeConn().asConnection() });
    const first = cache.reload();
    await settle();
    await cache.reload();
    release();
    await first;
    expect(cache.accounts().map((a) => a.name)).toEqual(["new"]);
  });

  it("holds why the accounts could not be read, and load tries again", async () => {
    const t = setup(() => problem(502, "github.unavailable"));
    await t.cache.load();
    expect(t.cache.loaded()).toBe(false);
    expect(t.cache.error()?.type).toBe("github.unavailable");

    t.answer(() => [acct("alice")]);
    await t.cache.load();
    expect(t.asked()).toBe(2);
    expect(t.cache.loaded()).toBe(true);
    expect(t.cache.error()).toBeNull();
  });

  it("reads again at stores.changed and resync only when it was loaded", async () => {
    const t = setup();
    t.conn.emit("stores.changed");
    t.conn.emit("resync");
    await settle();
    expect(t.asked()).toBe(0);

    await t.cache.load();
    t.conn.emit("stores.changed");
    await settle();
    expect(t.asked()).toBe(2);
    t.conn.emit("resync");
    await settle();
    expect(t.asked()).toBe(3);
  });

  it("reads again at stores.changed that comes while the first load is in flight", async () => {
    let release!: () => void;
    const gate = new Promise<void>((r) => {
      release = r;
    });
    let n = 0;
    const { api } = fakeApi(async (r) => {
      if (r.path !== "/api/accounts") return {};
      n++;
      if (n === 1) {
        await gate;
        return [acct("old")];
      }
      return [acct("new")];
    });
    const conn = new FakeConn();
    const cache = new AccountsCache({ api, conn: conn.asConnection() });
    const first = cache.load();
    await settle();
    conn.emit("stores.changed");
    await settle();
    release();
    await first;
    await settle();
    expect(n).toBe(2);
    expect(cache.accounts().map((a) => a.name)).toEqual(["new"]);
  });

  it("keeps the list when a reload fails, tells the subscribers once, and a success clears the error", async () => {
    const t = setup();
    await t.cache.load();
    const fn = vi.fn();
    t.cache.subscribe(fn);

    t.answer(() => problem(502, "github.unavailable"));
    await t.cache.reload();
    await t.cache.reload();
    expect(t.cache.accounts().map((a) => a.name)).toEqual(["alice"]);
    expect(t.cache.loaded()).toBe(true);
    expect(t.cache.error()?.type).toBe("github.unavailable");
    expect(fn).toHaveBeenCalledTimes(1);

    t.answer(() => [acct("alice")]);
    await t.cache.reload();
    expect(t.cache.error()).toBeNull();
    expect(fn).toHaveBeenCalledTimes(2);
  });

  it("reads once for the events of one turn", async () => {
    const t = setup();
    await t.cache.load();
    t.conn.emit("stores.changed");
    t.conn.emit("stores.changed");
    t.conn.emit("resync");
    await settle();
    expect(t.asked()).toBe(2);
  });
});

describe("source", () => {
  it("tells where the token of an account comes from", () => {
    expect(source(acct("a", { token: "gh" }))).toBe("gh");
    expect(source(acct("a", { token: "env:GH_TOKEN" }))).toBe("env");
    expect(envVar(acct("a", { token: "env:GH_TOKEN" }))).toBe("GH_TOKEN");
    expect(source(acct("a", { token: "keychain" }))).toBe("keychain");
    expect(source(acct("a", { token: "device" }))).toBe("device");
    expect(source(acct("a"))).toBe("none");
    expect(source(acct("a", { token: "" }))).toBe("none");
    expect(envVar(acct("a", { token: "gh" }))).toBeNull();
  });
});
