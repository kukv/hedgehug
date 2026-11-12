import { Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createAccountsPort } from "./accounts";
import { setToken } from "./fetcher";

afterEach(() => {
  vi.unstubAllGlobals();
});

function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async () =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

const sent = (fetch: ReturnType<typeof respond>) =>
  fetch.mock.calls[0] as unknown as [string, RequestInit];

const work = { id: "A1", name: "work", login: "taro", method: "device" };

describe("accounts port", () => {
  it("lists the rows with their state, and reads null as none", async () => {
    const fetch = respond(200, {
      accounts: [{ ...work, method: "gh", status: "auth_error", reason: "x" }],
    });
    expect(await createAccountsPort().list()).toEqual([
      { ...work, method: "gh", status: "auth_error", reason: "x" },
    ]);
    expect(sent(fetch)[0]).toBe("/api/accounts");
    respond(200, { accounts: null });
    expect(await createAccountsPort().list()).toEqual([]);
  });

  it("turns a 422 into a refusal with the spec text", async () => {
    respond(422, { detail: "同じ表示名のアカウントがあります" });
    const e = await createAccountsPort()
      .add({ name: "w", login: "t", method: "gh" })
      .catch((x: unknown) => x);
    expect(e).toBeInstanceOf(Refused);
    expect((e as Refused).message).toBe("同じ表示名のアカウントがあります");
  });

  it("starts a sign-in with the account and maps the code", async () => {
    const fetch = respond(201, {
      id: "F1",
      user_code: "WDJB-MJHT",
      verification_uri: "https://github.com/login/device",
      expires_in: 900,
      interval: 5,
    });
    expect(
      await createAccountsPort().startSignIn({ account: "A1", name: "work", login: "taro" }),
    ).toEqual({
      id: "F1",
      userCode: "WDJB-MJHT",
      verificationUri: "https://github.com/login/device",
      expiresInSec: 900,
      intervalSec: 5,
    });
    expect(JSON.parse(String(sent(fetch)[1].body))).toEqual({
      account: "A1",
      name: "work",
      login: "taro",
    });
  });

  it("asks the sign-in without the cache and maps the answer", async () => {
    const fetch = respond(200, { state: "done", message: "", interval: 5, account: work });
    expect(await createAccountsPort().pollSignIn("F1")).toEqual({
      status: "done",
      message: "",
      intervalSec: 5,
      account: work,
    });
    expect(sent(fetch)[0]).toBe("/api/sign-ins/F1");
    expect(sent(fetch)[1].cache).toBe("no-store");
    respond(200, { state: "waiting", message: "", interval: 10 });
    expect((await createAccountsPort().pollSignIn("F1")).account).toBeNull();
  });

  it("sends the new login to the edit impact, and reads null lines as none", async () => {
    const fetch = respond(200, { lines: null });
    expect(await createAccountsPort().editImpact("A1", "jiro")).toEqual([]);
    expect(sent(fetch)[0]).toBe("/api/accounts/A1/edit-impact?login=jiro");
  });
});
