import { describe, expect, it } from "vitest";
import {
  cliAdd,
  cliComment,
  cliLs,
  cliMv,
  cliRestore,
  cliRestoreAt,
  cliRm,
  cliSet,
  cliStoreInit,
  cliSync,
  cliWatch,
  cliWorkspaceAdd,
  Placeholder,
  quoteArg,
} from "./cli";

describe("quoteArg", () => {
  it("leaves a plain argument as it is", () => {
    expect(quoteArg("REL-8")).toBe("REL-8");
    expect(quoteArg("status:todo")).toBe("status:todo");
    expect(quoteArg("points=3")).toBe("points=3");
    expect(quoteArg("リリース作業")).toBe("リリース作業");
  });

  it("quotes an argument with a space or a symbol", () => {
    expect(quoteArg("fix the bug")).toBe('"fix the bug"');
    expect(quoteArg("a;b")).toBe('"a;b"');
    expect(quoteArg("")).toBe('""');
  });

  it("single-quotes an argument with ! or a leading =", () => {
    expect(quoteArg("Fix it!")).toBe("'Fix it!'");
    expect(quoteArg("a!b")).toBe("'a!b'");
    expect(quoteArg("it's!")).toBe("'it'\\''s!'");
    expect(quoteArg("=foo")).toBe("'=foo'");
    expect(quoteArg("a=b")).toBe("a=b");
  });

  it('escapes " \\ $ and ` with a backslash', () => {
    expect(quoteArg('say "hi"')).toBe('"say \\"hi\\""');
    expect(quoteArg("a\\b c")).toBe('"a\\\\b c"');
    expect(quoteArg("$HOME")).toBe('"\\$HOME"');
    expect(quoteArg("`date`")).toBe('"\\`date\\`"');
  });
});

describe("commands", () => {
  it("writes hh add with its options", () => {
    expect(
      cliAdd({
        title: "Switch DNS",
        store: "shared",
        workspace: "main",
        parent: "REL-8",
        kind: "task",
        body: "line 1\nline 2",
        props: { points: "2", priority: "high" },
      }),
    ).toBe(
      'hh add "Switch DNS" --store shared --workspace main --parent REL-8 --kind task --body "line 1\nline 2" -p points=2 -p priority=high',
    );
    expect(cliAdd({ title: "x" })).toBe("hh add x");
    expect(cliAdd({ title: "x", parent: "REL-8", after: "REL-9" })).toBe(
      "hh add x --parent REL-8 --after REL-9",
    );
    expect(cliAdd({ title: "x", before: "REL-9" })).toBe("hh add x --before REL-9");
  });

  it("writes hh ls", () => {
    expect(cliLs("", true)).toBe("hh ls --tree");
    expect(cliLs("status:todo assignee:alice", false)).toBe('hh ls "status:todo assignee:alice"');
    expect(cliLs("kind:project", false)).toBe("hh ls kind:project");
    expect(cliLs("workspace:main -status:canceled", true, "shared")).toBe(
      'hh ls --store shared --tree "workspace:main -status:canceled"',
    );
  });

  it("puts -- before an argument that starts with -", () => {
    expect(cliLs("-status:canceled", false)).toBe("hh ls -- -status:canceled");
    expect(cliLs("-status:canceled", true, "shared")).toBe(
      "hh ls --store shared --tree -- -status:canceled",
    );
    expect(cliAdd({ title: "-x", store: "demo" })).toBe("hh add --store demo -- -x");
    expect(cliAdd({ title: "-x", body: "-y" })).toBe("hh add --body -y -- -x");
    expect(cliStoreInit({ name: "-s", prefix: "WEB" })).toBe("hh store init --prefix WEB -- -s");
  });

  it("writes hh set, with the operators of a list", () => {
    expect(cliSet("REL-8", { status: "done", "labels+": "ops", due: "", title: "A B" })).toBe(
      'hh set REL-8 status=done labels+=ops due= "title=A B"',
    );
  });

  it("writes hh mv", () => {
    expect(cliMv("REL-8", { parent: "REL-1", after: "REL-3" })).toBe(
      "hh mv REL-8 --parent REL-1 --after REL-3",
    );
    expect(cliMv("REL-8", { root: true, before: "REL-2" })).toBe(
      "hh mv REL-8 --root --before REL-2",
    );
  });

  it("writes the others", () => {
    expect(cliRm("REL-8")).toBe("hh rm REL-8");
    expect(cliRestore("REL-8")).toBe("hh restore REL-8");
    expect(cliRestoreAt("REL-8", "abc123")).toBe("hh restore REL-8 --at abc123");
    expect(cliComment("REL-8", "add")).toBe("hh comment REL-8");
    expect(cliComment("REL-8", "edit", "01HX")).toBe("hh comment edit 01HX");
    expect(cliComment("REL-8", "rm", "01HX")).toBe("hh comment rm 01HX");
    expect(cliWatch("REL-8", true)).toBe("hh watch REL-8");
    expect(cliWatch("REL-8", false)).toBe("hh unwatch REL-8");
    expect(cliSync()).toBe("hh sync");
    expect(cliSync("shared")).toBe("hh sync --store shared");
    expect(cliWorkspaceAdd({ name: "web", prefix: "WEB", store: "shared" })).toBe(
      "hh workspace add web --prefix WEB --store shared",
    );
    expect(
      cliStoreInit({ name: "shared", prefix: "WEB", workspace: "web", remotes: ["git@x:a/b.git"] }),
    ).toBe("hh store init shared --prefix WEB --workspace web --remote git@x:a/b.git");
  });

  it("writes a placeholder as it is, so that a shell refuses the line", () => {
    const name = new Placeholder("名前");
    const prefix = new Placeholder("接頭辞");
    expect(cliStoreInit({ name, prefix })).toBe("hh store init <名前> --prefix <接頭辞>");
    expect(cliWorkspaceAdd({ name, prefix, store: "a b" })).toBe(
      'hh workspace add <名前> --prefix <接頭辞> --store "a b"',
    );
    // what a person types is still quoted
    expect(cliStoreInit({ name: "<名前>", prefix: "ME" })).toBe(
      'hh store init "<名前>" --prefix ME',
    );
  });
});
