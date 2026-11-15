import { Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createSettingsPort } from "./settings";

afterEach(() => {
  vi.unstubAllGlobals();
});

function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async (_url: string, _init?: RequestInit) =>
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
  );
  vi.stubGlobal("fetch", fetch);
  setToken("tok");
  return fetch;
}

const sent = (fetch: ReturnType<typeof respond>) => {
  const [url, init] = fetch.mock.calls[0] as [string, RequestInit];
  return [url, init.method, init.body];
};

const timing = { auto: true, intervalMinutes: 15, afterWriteSeconds: 30, githubMinutes: 5 };
const line = "local での あなた が @taro から tarou に変わります。";

describe("settings port", () => {
  it("reads and saves the login", async () => {
    let fetch = respond(200, { login: "taro" });
    expect(await createSettingsPort().login()).toBe("taro");
    expect(sent(fetch)).toEqual(["/api/settings/login", "GET", undefined]);
    fetch = respond(204, undefined);
    await createSettingsPort().setLogin("tarou");
    expect(sent(fetch)).toEqual(["/api/settings/login", "PUT", '{"login":"tarou"}']);
  });

  it("reads the lines of the confirmation", async () => {
    const fetch = respond(200, { lines: [line] });
    expect(await createSettingsPort().previewLogin("tarou")).toEqual([line]);
    expect(sent(fetch)).toEqual(["/api/settings/login/preview", "POST", '{"login":"tarou"}']);
  });

  it("turns a 422 into a refusal with the spec text", async () => {
    const text = "英数字・ハイフン・アンダースコアのログイン名(39 文字以内)を入力してください";
    respond(422, { detail: text });
    const e = await createSettingsPort()
      .previewLogin("ta ro")
      .catch((x: unknown) => x);
    expect(e).toBeInstanceOf(Refused);
    expect((e as Refused).message).toBe(text);
  });

  it("reads and saves the sync timing", async () => {
    let fetch = respond(200, timing);
    expect(await createSettingsPort().timing()).toEqual(timing);
    expect(sent(fetch)).toEqual(["/api/sync/timing", "GET", undefined]);
    fetch = respond(204, undefined);
    await createSettingsPort().setTiming(timing);
    expect(sent(fetch)).toEqual(["/api/sync/timing", "PUT", JSON.stringify(timing)]);
  });
});
