import { NotFound, Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createLinksPort } from "./links";

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

const row = {
  kind: "pr",
  repo: "org/web",
  number: 41,
  title: "A",
  state: "open",
  review: "pending",
  checks: "",
  error: "",
};

describe("links port", () => {
  it("reads rows with the narrow types", async () => {
    respond(200, [row]);
    expect(await createLinksPort().list("S1", "I1")).toEqual([row]);
  });

  it("sends the add body and reads 204", async () => {
    const fetch = respond(204, undefined);
    await createLinksPort().add("S1", "I1", {
      kind: "pr",
      repo: "org/web",
      number: "41",
      addToProject: true,
    });
    const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/stores/S1/items/I1/links");
    expect(JSON.parse(String(init.body))).toEqual({
      kind: "pr",
      repo: "org/web",
      number: "41",
      addToProject: true,
    });
  });

  it("sends only the link reference to remove, from a whole row", async () => {
    const fetch = respond(204, undefined);
    await createLinksPort().remove("S1", "I1", row as never);
    const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toContain("/api/stores/S1/items/I1/links/remove");
    expect(JSON.parse(String(init.body))).toEqual({ kind: "pr", repo: "org/web", number: 41 });
  });

  it("reads the project's rows and an empty answer", async () => {
    const fetch = respond(200, [{ ...row, item: "I1", display: "HH-2" }]);
    expect(await createLinksPort().listProject("S1", "P1")).toEqual([
      { ...row, item: "I1", display: "HH-2" },
    ]);
    expect((fetch.mock.calls[0] as unknown as [string])[0]).toContain(
      "/api/stores/S1/projects/P1/links",
    );
    respond(200, null);
    expect(await createLinksPort().listProject("S1", "P1")).toEqual([]);
  });

  it("turns 422 into Refused and 404 into NotFound", async () => {
    respond(422, { detail: "番号を入力してください" });
    await expect(
      createLinksPort().add("S1", "I1", {
        kind: "pr",
        repo: "org/web",
        number: "x",
        addToProject: false,
      }),
    ).rejects.toEqual(new Refused("番号を入力してください"));
    respond(404, { detail: "not found" });
    await expect(createLinksPort().refresh("S1", "I1")).rejects.toBeInstanceOf(NotFound);
  });
});
