import { afterEach, describe, expect, it, vi } from "vitest";
import { setToken } from "./fetcher";
import { createWorkspacesPort } from "./workspaces";

afterEach(() => {
  vi.unstubAllGlobals();
});

// respond は、fetch を 1 つの応答に差し替え、呼び出しを返す。
function respond(status: number, body: unknown) {
  const fetch = vi.fn(
    async () =>
      // 204 の応答は本文を持てない(Response は null の本文だけを受ける)。
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

describe("workspaces port: templates, workflow and transitions", () => {
  it("sends template values as they are and reads null lists of a template as empty", async () => {
    const fetch = respond(200, {
      id: "t1",
      name: "障害",
      kind: "Bug",
      title: "",
      description: "",
      values: null,
      children: null,
      steps: null,
    });
    const t = await createWorkspacesPort().createTemplate("S1", "W1", {
      name: "障害",
      kind: "Bug",
      title: "",
      description: "",
      values: { priority: null, memo: "x" },
      children: [],
      steps: [],
    });
    const [url, init] = call(fetch);
    expect(url).toBe("/api/stores/S1/workspaces/W1/templates");
    expect(JSON.parse(String(init.body)).values).toEqual({ priority: null, memo: "x" });
    expect(t).toMatchObject({ id: "t1", values: {}, children: [], steps: [] });
  });

  it("reads the workflow and templates of the settings with nulls as empty", async () => {
    respond(200, {
      id: "W1",
      name: "Web",
      prefix: "HH",
      settings: {
        categoryDefaults: {},
        initialPriority: "pr1",
        initialStatus: "st1",
        priorities: [],
        statuses: [],
        systems: [],
        properties: [],
        workflow: null,
        templateDefaults: null,
        templates: [
          {
            id: "t1",
            name: "n",
            kind: "Epic",
            title: "",
            description: "",
            values: null,
            children: null,
            steps: null,
          },
        ],
      },
    });
    const d = await createWorkspacesPort().get("S1", "W1");
    expect(d.settings.workflow).toEqual({});
    expect(d.settings.templateDefaults).toEqual({});
    expect(d.settings.templates[0]).toMatchObject({ values: {}, children: [], steps: [] });
  });

  it("sends the transition, template default and save-as-template operations", async () => {
    const port = createWorkspacesPort();
    const tr = respond(204, undefined);
    await port.setTransition("S1", "W1", "st1", "st2", false);
    expect(call(tr)[0]).toContain("/workspaces/W1/");
    expect(JSON.parse(String(call(tr)[1].body))).toEqual({ allowed: false });
    const def = respond(204, undefined);
    await port.setTemplateDefault("S1", "W1", "Bug", "");
    expect(JSON.parse(String(call(def)[1].body))).toEqual({ template: "" });
    const save = respond(200, {
      id: "t2",
      name: "n",
      kind: "Epic",
      title: "",
      description: "",
      values: {},
      children: [],
      steps: [],
    });
    await port.saveItemAsTemplate("S1", "I1", { name: "n", title: "", structure: true });
    expect(JSON.parse(String(call(save)[1].body))).toEqual({
      name: "n",
      title: "",
      structure: true,
    });
  });
});
