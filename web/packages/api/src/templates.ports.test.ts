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

describe("workspaces port: automation", () => {
  const apiRule = {
    id: "R1",
    name: "PR マージで完了",
    trigger: "pr_merged",
    triggerStatus: "",
    allMerged: false,
    action: "status",
    value: "st4",
    enabled: true,
  };
  const input = {
    name: apiRule.name,
    trigger: "pr_merged",
    triggerStatus: "",
    allMerged: false,
    action: "status",
    value: "st4",
  } as const;

  it("reads the rules and recent runs, with null lists as empty and the time as a Date", async () => {
    const fetch = respond(200, {
      rules: [apiRule],
      recent: [{ at: "2026-11-13T08:55:00Z", item: "I1", display: "HH-2", text: "t" }],
    });
    const a = await createWorkspacesPort().automation("S1", "W1");
    expect(call(fetch)[0]).toBe("/api/stores/S1/workspaces/W1/automation");
    expect(a.rules).toEqual([apiRule]);
    expect(a.recent[0]?.at).toEqual(new Date("2026-11-13T08:55:00Z"));
    respond(200, { rules: null, recent: null });
    await expect(createWorkspacesPort().automation("S1", "W1")).resolves.toEqual({
      rules: [],
      recent: [],
    });
  });

  it("creates, updates, deletes and switches a rule", async () => {
    const port = createWorkspacesPort();
    const create = respond(200, apiRule);
    await expect(port.createRule("S1", "W1", input)).resolves.toEqual(apiRule);
    expect(call(create)[0]).toBe("/api/stores/S1/workspaces/W1/automation/rules");
    expect(JSON.parse(String(call(create)[1].body))).toEqual(input);
    const update = respond(204, undefined);
    await port.updateRule("S1", "W1", "R1", input);
    expect(call(update)[0]).toBe("/api/stores/S1/workspaces/W1/automation/rules/R1");
    expect(call(update)[1].method).toBe("PUT");
    const del = respond(204, undefined);
    await port.deleteRule("S1", "W1", "R1");
    expect(call(del)[1].method).toBe("DELETE");
    const sw = respond(204, undefined);
    await port.setRuleEnabled("S1", "W1", "R1", false);
    expect(call(sw)[0]).toBe("/api/stores/S1/workspaces/W1/automation/rules/R1/enabled");
    expect(JSON.parse(String(call(sw)[1].body))).toEqual({ enabled: false });
  });

  it("sends only the six input fields, even when given a whole rule", async () => {
    // 入力の型は追加の項目を断る(additionalProperties: false)。id と enabled を送ると 422 になる。
    const port = createWorkspacesPort();
    const create = respond(200, apiRule);
    await port.createRule("S1", "W1", apiRule as never);
    expect(JSON.parse(String(call(create)[1].body))).toEqual(input);
    const update = respond(204, undefined);
    await port.updateRule("S1", "W1", "R1", apiRule as never);
    expect(JSON.parse(String(call(update)[1].body))).toEqual(input);
  });

  it("sends the scope of a project-only rule on create, and never on update", async () => {
    const port = createWorkspacesPort();
    const create = respond(200, { ...apiRule, scope: "P1" });
    const made = await port.createRule("S1", "W1", { ...input, scope: "P1" });
    expect(made.scope).toBe("P1");
    expect(JSON.parse(String(call(create)[1].body))).toEqual({ ...input, scope: "P1" });
    const update = respond(204, undefined);
    await port.updateRule("S1", "W1", "R1", { ...input, scope: "P1" });
    expect(JSON.parse(String(call(update)[1].body))).toEqual(input);
  });
});
