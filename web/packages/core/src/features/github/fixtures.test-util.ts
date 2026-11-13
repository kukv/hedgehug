import type { LinkRow } from "../../model/link";
import type { LinksPort } from "../../port/links";

export const row = (p: Partial<LinkRow> = {}): LinkRow => ({
  kind: "pr",
  repo: "org/web",
  number: 41,
  title: "Add login form",
  state: "open",
  review: "pending",
  checks: "",
  error: "",
  ...p,
});

// fakeLinks は、呼び出しを覚え、答えを差し替えられる LinksPort。
export function fakeLinks() {
  const calls: string[] = [];
  const fake = {
    calls,
    rows: [row()] as LinkRow[],
    refreshed: null as LinkRow[] | null,
    fail: null as unknown,
    port: {} as LinksPort,
  };
  const check = () => {
    if (fake.fail) throw fake.fail;
  };
  fake.port = {
    async list(s, i) {
      calls.push(`list ${s} ${i}`);
      check();
      return fake.rows;
    },
    async refresh(s, i) {
      calls.push(`refresh ${s} ${i}`);
      check();
      return fake.refreshed ?? fake.rows;
    },
    async add(s, i, input) {
      calls.push(`add ${s} ${i} ${input.kind} ${input.repo} ${input.number} ${input.addToProject}`);
      check();
    },
    async remove(s, i, t) {
      calls.push(`remove ${s} ${i} ${t.kind}:${t.repo}#${t.number}`);
      check();
    },
    async listProject(s, p) {
      calls.push(`project ${s} ${p}`);
      check();
      return [];
    },
  };
  return fake;
}
