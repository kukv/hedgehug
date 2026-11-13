import { NotFound } from "../../model/errors";
import type { AutomationRule, WorkspaceAutomation } from "../../model/workspace";
import type { WorkspacesPort } from "../../port/workspaces";

const clone = <T>(x: T): T => JSON.parse(JSON.stringify(x)) as T;

type Write = <T>(name: string, fn: () => T) => Promise<T>;

// fakeAutomation は、偽の口の自動化の 5 つの操作。ルールはメモリに持つ。
export function fakeAutomation(
  calls: string[],
  write: Write,
  newId: (prefix: string) => string,
): Pick<
  WorkspacesPort,
  "automation" | "createRule" | "updateRule" | "deleteRule" | "setRuleEnabled"
> {
  const automations = new Map<string, WorkspaceAutomation>();
  const automation = (s: string, w: string) => {
    const key = `${s}/${w}`;
    let a = automations.get(key);
    if (!a) {
      a = { rules: [], recent: [] };
      automations.set(key, a);
    }
    return a;
  };
  const rule = (s: string, w: string, id: string) => {
    const r = automation(s, w).rules.find((x) => x.id === id);
    if (!r) throw new NotFound(id);
    return r;
  };
  return {
    async automation(s, w) {
      calls.push("automation");
      return clone(automation(s, w));
    },
    createRule: (s, w, input) =>
      write("createRule", (): AutomationRule => {
        const r: AutomationRule = { ...input, id: newId("R"), enabled: true };
        automation(s, w).rules.push(r);
        return clone(r);
      }),
    updateRule: (s, w, id, input) =>
      write("updateRule", () => {
        Object.assign(rule(s, w, id), input);
      }),
    deleteRule: (s, w, id) =>
      write("deleteRule", () => {
        const a = automation(s, w);
        rule(s, w, id);
        a.rules = a.rules.filter((r) => r.id !== id);
      }),
    setRuleEnabled: (s, w, id, enabled) =>
      write("setRuleEnabled", () => {
        rule(s, w, id).enabled = enabled;
      }),
  };
}
