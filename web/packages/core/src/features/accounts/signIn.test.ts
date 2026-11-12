import { describe, expect, it } from "vitest";
import type {
  Account,
  AccountRow,
  SignInCode,
  SignInFailure,
  SignInResult,
  SignInTarget,
} from "../../model/account";
import { NotFound, Refused } from "../../model/errors";
import { get } from "../../reactive/store";
import { code, fakeAccounts, fakeTimer, fakeToasts, flush, waiting } from "./fixtures.test-util";
import { createSignIn } from "./signIn";

function setup(rows: AccountRow[] = []) {
  const f = fakeAccounts(rows);
  const t = fakeTimer();
  const { toasts, texts } = fakeToasts();
  const back: { target: SignInTarget; failure: SignInFailure }[] = [];
  let done = 0;
  const signIn = createSignIn({
    accounts: f.port,
    toasts,
    timer: t.timer,
    back: (target, failure) => void back.push({ target, failure }),
    onDone: () => {
      done += 1;
    },
  });
  return { f, t, texts, back, signIn, done: () => done };
}

const add: SignInTarget = { purpose: "add", original: null, name: "work", login: "taro" };
const taro: Account = { id: "A1", name: "work", login: "taro", method: "device" };
const doneWith = (a: Account): SignInResult => ({ ...waiting, status: "done", account: a });
const mismatch = "このトークンは @jiro のものです。ログイン名を直してください";
const noKeychain =
  "このマシンでは OS のキーチェーンを使えません。gh CLI のトークンか環境変数を使ってください";
const unreachable = "GitHub に接続できません。接続を確かめて、もう一度お試しください";

async function tick(t: ReturnType<typeof fakeTimer>, ms: number) {
  t.advance(ms);
  await flush();
}

describe("createSignIn", () => {
  it("asks after the interval the server gives each time", async () => {
    const { f, t, signIn } = setup();
    signIn.begin(add, code("F1"));
    expect(get(signIn.state)).toMatchObject({ target: add, ended: "", error: "", retrying: false });
    await tick(t, 4999);
    expect(f.calls).toEqual([]);
    f.polls.push({ ...waiting, intervalSec: 10 });
    await tick(t, 1);
    expect(f.calls).toEqual(["pollSignIn F1"]);
    await tick(t, 5000);
    expect(f.calls).toEqual(["pollSignIn F1"]);
    await tick(t, 5000);
    expect(f.calls).toEqual(["pollSignIn F1", "pollSignIn F1"]);
  });

  it("asks at once when the code expires, and stops after it", async () => {
    const { f, t, signIn } = setup();
    signIn.begin(add, { ...code("F1"), expiresInSec: 3 });
    f.polls.push({ ...waiting, status: "expired" });
    await tick(t, 3000);
    expect(get(signIn.state)?.ended).toBe("expired");
    expect(f.calls).toEqual(["pollSignIn F1", "cancelSignIn F1"]);
    await tick(t, 60_000);
    expect(f.calls).toHaveLength(2);
  });

  it("closes and tells by purpose when GitHub allows it", async () => {
    const cases = [
      ["add", "@taro としてサインインしました"],
      ["edit", "アカウントを変更しました"],
      ["reauth", "work: @taro として再認証しました"],
    ] as const;
    for (const [purpose, want] of cases) {
      const { f, t, signIn, texts, done } = setup();
      signIn.begin({ ...add, purpose }, code("F1"));
      f.polls.push(doneWith(taro));
      await tick(t, 5000);
      expect(get(signIn.state)).toBeNull();
      expect(texts()).toEqual([want]);
      expect(done()).toBe(1);
    }
  });

  it("goes back to add or edit when another person allows it or the save fails", async () => {
    const { f, t, signIn, back } = setup();
    signIn.begin(add, code("F1"));
    f.polls.push({ ...waiting, status: "mismatch", message: mismatch });
    await tick(t, 5000);
    expect(get(signIn.state)).toBeNull();
    expect(back).toEqual([{ target: add, failure: { loginError: mismatch } }]);
    signIn.begin(add, code("F2"));
    f.polls.push({ ...waiting, status: "failed", message: noKeychain });
    await tick(t, 5000);
    expect(back[1]).toEqual({ target: add, failure: { error: noKeychain } });
  });

  it("keeps a failed reauth in its own dialog, and retries with a new code", async () => {
    const { f, t, signIn, back } = setup();
    const reauth: SignInTarget = { purpose: "reauth", original: taro, name: "work", login: "taro" };
    signIn.begin(reauth, code("F1"));
    f.polls.push({ ...waiting, status: "mismatch", message: mismatch });
    await tick(t, 5000);
    expect(get(signIn.state)).toMatchObject({ ended: "failed", error: mismatch });
    expect(back).toEqual([]);
    await signIn.retry();
    expect(f.calls).toContain("startSignIn A1 work taro");
    expect(get(signIn.state)).toMatchObject({ ended: "", error: "", code: { id: "S1" } });
  });

  it("goes back to add when a retry cannot get a code", async () => {
    const { f, t, signIn, back } = setup();
    signIn.begin(add, code("F1"));
    f.polls.push({ ...waiting, status: "denied" });
    await tick(t, 5000);
    expect(get(signIn.state)?.ended).toBe("denied");
    f.refuse.startSignIn = unreachable;
    await signIn.retry();
    expect(get(signIn.state)).toBeNull();
    expect(back).toEqual([{ target: add, failure: { error: unreachable } }]);
  });

  it("cancels the flow on close; a later answer that it was done only reloads the list", async () => {
    const { f, t, signIn, texts, done } = setup();
    signIn.begin(add, code("F1"));
    let answer: (r: SignInResult) => void = () => undefined;
    f.port.pollSignIn = (id) => {
      f.calls.push(`pollSignIn ${id}`);
      return new Promise((resolve) => {
        answer = resolve;
      });
    };
    await tick(t, 5000);
    signIn.close();
    expect(get(signIn.state)).toBeNull();
    expect(f.calls).toEqual(["pollSignIn F1", "cancelSignIn F1"]);
    answer(doneWith(taro));
    await flush();
    expect(texts()).toEqual([]);
    expect(done()).toBe(1);
    await tick(t, 60_000);
    expect(f.calls).toHaveLength(2);
  });

  it("keeps waiting when the server cannot be reached, and shows a gone flow as expired", async () => {
    const { f, t, signIn } = setup();
    signIn.begin(add, code("F1"));
    f.polls.push(new TypeError("Failed to fetch"), new NotFound("not found"));
    await tick(t, 5000);
    expect(get(signIn.state)?.ended).toBe("");
    await tick(t, 5000);
    expect(get(signIn.state)?.ended).toBe("expired");
  });

  it("treats a refusal of the question like a failed sign-in", async () => {
    const { f, t, signIn, back } = setup();
    signIn.begin(add, code("F1"));
    f.polls.push(new Refused(noKeychain));
    await tick(t, 5000);
    expect(back).toEqual([{ target: add, failure: { error: noKeychain } }]);
  });

  it("starts a reauth with the account, and lets the caller show a refusal", async () => {
    const { f, signIn } = setup();
    await signIn.reauth(taro);
    expect(f.calls).toEqual(["startSignIn A1 work taro"]);
    expect(get(signIn.state)?.target).toEqual({
      purpose: "reauth",
      original: taro,
      name: "work",
      login: "taro",
    });
    f.refuse.startSignIn = unreachable;
    await expect(signIn.reauth(taro)).rejects.toBeInstanceOf(Refused);
  });

  it("polls a new flow even while the closed flow's request is still pending", async () => {
    const { f, t, signIn, texts, done } = setup();
    signIn.begin(add, code("F1"));
    const held: ((r: SignInResult) => void)[] = [];
    const ok = f.port.pollSignIn;
    f.port.pollSignIn = (id) => {
      if (id !== "F1") return ok(id);
      f.calls.push(`pollSignIn ${id}`);
      return new Promise((resolve) => held.push(resolve));
    };
    await tick(t, 5000);
    signIn.close();
    signIn.begin(add, code("F2"));
    await tick(t, 5000);
    expect(f.calls).toContain("pollSignIn F2");
    f.polls.push(doneWith(taro));
    await tick(t, 5000);
    expect(get(signIn.state)).toBeNull();
    expect(texts()).toEqual(["@taro としてサインインしました"]);
    held[0]?.(waiting);
    await flush();
    expect(get(signIn.state)).toBeNull();
    expect(done()).toBe(1);
  });

  it("does not poll twice later when a poll at expiry fails", async () => {
    const { f, t, signIn } = setup();
    signIn.begin(add, { ...code("F1"), expiresInSec: 5 });
    f.polls.push(new TypeError("Failed to fetch"));
    await tick(t, 5000);
    const n = f.calls.length;
    await tick(t, 5000);
    expect(f.calls.length - n).toBe(1);
  });

  it("ignores retry while the dialog is still waiting", async () => {
    const { f, signIn } = setup();
    signIn.begin(add, code("F1"));
    await signIn.retry();
    expect(f.calls).toEqual([]);
    expect(get(signIn.state)).toMatchObject({ code: { id: "F1" }, retrying: false });
  });

  it("drops the code of a reauth that arrives after the dialog was closed", async () => {
    const { f, t, signIn } = setup();
    const held: ((c: SignInCode) => void)[] = [];
    f.port.startSignIn = () => new Promise((resolve) => held.push(resolve));
    const pending = signIn.reauth(taro);
    signIn.close();
    held[0]?.(code("R1"));
    await pending;
    await tick(t, 60_000);
    expect(get(signIn.state)).toBeNull();
    expect(f.calls).toEqual(["cancelSignIn R1"]);
  });

  it("drops the code of a retry that arrives after close and another sign-in", async () => {
    const { f, t, signIn } = setup();
    signIn.begin({ ...add, purpose: "reauth" }, code("F1"));
    f.polls.push({ ...waiting, status: "expired" });
    await tick(t, 5000);
    expect(get(signIn.state)?.ended).toBe("expired");
    const held: ((c: SignInCode) => void)[] = [];
    f.port.startSignIn = () => new Promise((resolve) => held.push(resolve));
    const pending = signIn.retry();
    signIn.close();
    signIn.begin(add, code("F2"));
    held[0]?.(code("R1"));
    await pending;
    expect(get(signIn.state)).toMatchObject({ target: add, code: { id: "F2" }, retrying: false });
    expect(f.calls).toContain("cancelSignIn R1");
    expect(f.calls).not.toContain("cancelSignIn F2");
    f.calls.length = 0;
    await tick(t, 5000);
    expect(f.calls).toEqual(["pollSignIn F2"]);
  });
});
