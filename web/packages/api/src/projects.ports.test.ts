import type { Project, ProjectsPort } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createProjectsPort } from "./projects";

afterEach(() => {
  vi.unstubAllGlobals();
});

// respond は、fetch を 1 つの応答に差し替え、呼び出しを返す。
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

function call(fetch: ReturnType<typeof respond>, n = 0) {
  return fetch.mock.calls[n] as unknown as [string, RequestInit];
}

const apiProject = {
  id: "P1",
  workspace: "W1",
  number: 1,
  rank: "V",
  title: "ログイン",
  owner: "taro",
  status: "Todo",
  start: "",
  end: "",
  repos: ["example-org/web"],
};

interface Case {
  name: string;
  run: (port: ProjectsPort) => Promise<Project>;
  method: string;
  url: string;
  body: unknown;
}

const cases: Case[] = [
  {
    name: "rename",
    run: (p) => p.rename("S1", "P1", "決済"),
    method: "PUT",
    url: "/api/stores/S1/projects/P1/title",
    body: { title: "決済" },
  },
  {
    name: "setStatus",
    run: (p) => p.setStatus("S1", "P1", "InProgress"),
    method: "PUT",
    url: "/api/stores/S1/projects/P1/status",
    body: { status: "InProgress" },
  },
  {
    name: "setOwner",
    run: (p) => p.setOwner("S1", "P1", ""),
    method: "PUT",
    url: "/api/stores/S1/projects/P1/owner",
    body: { owner: "" },
  },
  {
    name: "setDate",
    run: (p) => p.setDate("S1", "P1", "end", "2026-11-30"),
    method: "PUT",
    url: "/api/stores/S1/projects/P1/date",
    body: { field: "end", value: "2026-11-30" },
  },
  {
    name: "addRepo",
    run: (p) => p.addRepo("S1", "P1", "example-org/web"),
    method: "POST",
    url: "/api/stores/S1/projects/P1/repos",
    body: { repo: "example-org/web" },
  },
  {
    name: "removeRepo",
    run: (p) => p.removeRepo("S1", "P1", "example-org/web"),
    method: "POST",
    url: "/api/stores/S1/projects/P1/repos/remove",
    body: { repo: "example-org/web" },
  },
];

describe("projects port: overview edits", () => {
  it.each(cases)(
    "$name goes to its own endpoint and returns the project with its repos",
    async (c) => {
      const fetch = respond(200, apiProject);
      const project = await c.run(createProjectsPort());
      const [url, init] = call(fetch);
      expect(url).toBe(c.url);
      expect(init.method).toBe(c.method);
      expect(JSON.parse(String(init.body))).toEqual(c.body);
      expect(project.repos).toEqual(["example-org/web"]);
    },
  );

  it("reads a null list of repos as empty", async () => {
    respond(200, { ...apiProject, repos: null });
    const project = await createProjectsPort().get("S1", "P1");
    expect(project.repos).toEqual([]);
  });
});
