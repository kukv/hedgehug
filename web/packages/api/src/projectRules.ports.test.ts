import { NotFound, Refused } from "@hedgehug/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createItemActionsPort } from "./itemActions";
import { createProjectsPort } from "./projects";
import { createWorkspacesPort } from "./workspaces";

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

const rules = "/api/stores/S1/projects/P1/rules";

describe("projects port: project rules", () => {
  it("builds the paths and bodies of the six operations", async () => {
    const fetch = respond(204, undefined);
    const port = createProjectsPort();
    await port.setStatusUsed("S1", "P1", "V1", false);
    await port.setTransition("S1", "P1", "V1", "V2", true);
    await port.setTemplateUsed("S1", "P1", "T1", true);
    await port.setTemplateDefault("S1", "P1", "Task", "@none");
    await port.setRuleRuns("S1", "P1", "R1", false);
    await port.resetRules("S1", "P1", "automation");
    const sent = fetch.mock.calls.map((_, i) => {
      const [url, init] = call(fetch, i);
      return [init.method, url, init.body === undefined ? null : JSON.parse(String(init.body))];
    });
    expect(sent).toEqual([
      ["PUT", `${rules}/statuses/V1`, { used: false }],
      ["PUT", `${rules}/workflow/V1/V2`, { allowed: true }],
      ["PUT", `${rules}/templates/T1`, { used: true }],
      ["PUT", `${rules}/template-defaults/Task`, { template: "@none" }],
      ["PUT", `${rules}/automation/R1`, { runs: false }],
      ["POST", `${rules}/automation/reset`, null],
    ]);
  });

  it("sends an empty template default as it is to clear it", async () => {
    const fetch = respond(204, undefined);
    await createProjectsPort().setTemplateDefault("S1", "P1", "Task", "");
    expect(call(fetch)[0]).toBe(`${rules}/template-defaults/Task`);
    expect(JSON.parse(String(call(fetch)[1].body))).toEqual({ template: "" });
  });

  it("turns a refusal into Refused and a missing project into NotFound", async () => {
    respond(422, {
      title: "Unprocessable Entity",
      status: 422,
      detail: "初期ステータスのため外せません",
    });
    await expect(createProjectsPort().setStatusUsed("S1", "P1", "V1", false)).rejects.toThrow(
      Refused,
    );
    respond(404, { title: "Not Found", status: 404, detail: "not found" });
    await expect(createProjectsPort().resetRules("S1", "P1", "statuses")).rejects.toThrow(NotFound);
  });
});

describe("workspaces port: project rules in the settings and rule scope", () => {
  it("reads project rules with nulls as empty lists and unset kinds left out", async () => {
    respond(200, {
      id: "W1",
      name: "Web",
      prefix: "HH",
      oldPrefixes: null,
      settings: {
        properties: null,
        statuses: null,
        priorities: null,
        systems: null,
        initialStatus: "V1",
        initialPriority: "V9",
        categoryDefaults: null,
        workflow: null,
        templates: null,
        templateDefaults: null,
        projectRules: {
          P1: {
            hiddenStatuses: ["V4"],
            workflow: { V1: ["V2"], V2: null },
            templates: null,
            templateDefaults: { Task: "T1" },
            disabledRules: null,
          },
          P2: null,
        },
      },
    });
    const d = await createWorkspacesPort().get("S1", "W1");
    expect(d.settings.projectRules).toEqual({
      P1: {
        hiddenStatuses: ["V4"],
        workflow: { V1: ["V2"], V2: [] },
        templates: [],
        templateDefaults: { Task: "T1" },
        disabledRules: [],
      },
      P2: {
        hiddenStatuses: [],
        workflow: {},
        templates: [],
        templateDefaults: {},
        disabledRules: [],
      },
    });
    respond(200, {
      id: "W1",
      name: "Web",
      prefix: "HH",
      settings: { initialStatus: "V1", initialPriority: "V9" },
    });
    expect((await createWorkspacesPort().get("S1", "W1")).settings.projectRules).toEqual({});
  });

  it("reads the scope of a rule, left out for the whole workspace", async () => {
    respond(200, {
      rules: [
        {
          id: "R1",
          name: "a",
          trigger: "pr_open",
          triggerStatus: "",
          allMerged: false,
          action: "assignee",
          value: "@none",
          enabled: true,
        },
        {
          id: "R2",
          name: "b",
          trigger: "pr_open",
          triggerStatus: "",
          allMerged: false,
          action: "assignee",
          value: "@none",
          enabled: true,
          scope: "P1",
        },
      ],
      recent: [],
    });
    const a = await createWorkspacesPort().automation("S1", "W1");
    expect(a.rules.map((r) => r.scope)).toEqual([undefined, "P1"]);
  });
});

describe("item actions port: move preview and statuses", () => {
  it("reads the preview of a move to a project or a parent", async () => {
    const fetch = respond(200, {
      below: 2,
      statuses: [{ status: "V4", count: 3, default: "V1" }],
    });
    const p = await createItemActionsPort().movePreview("S1", "I1", { project: "P2" });
    expect(call(fetch)[0]).toBe("/api/stores/S1/items/I1/move-preview?project=P2");
    expect(p).toEqual({ below: 2, statuses: [{ status: "V4", count: 3, default: "V1" }] });
    respond(200, { below: 0, statuses: null });
    expect(await createItemActionsPort().movePreview("S1", "I1", { parent: "I3" })).toEqual({
      below: 0,
      statuses: [],
    });
    const parent = respond(200, { below: 0, statuses: [] });
    await createItemActionsPort().movePreview("S1", "I1", { parent: "I3" });
    expect(call(parent)[0]).toBe("/api/stores/S1/items/I1/move-preview?parent=I3");
  });

  it("sends the statuses of a move only when given", async () => {
    const fetch = respond(204, undefined);
    const port = createItemActionsPort();
    await port.move("S1", "I1", { project: "P2" }, { V4: "V1" });
    await port.move("S1", "I1", { project: "P2" });
    expect(JSON.parse(String(call(fetch)[1].body))).toEqual({
      project: "P2",
      statuses: { V4: "V1" },
    });
    expect(JSON.parse(String(call(fetch, 1)[1].body))).toEqual({ project: "P2" });
  });
});
