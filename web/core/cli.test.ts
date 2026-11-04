import { describe, expect, it } from "vitest";
import {
  cliAdd,
  cliAuthAdd,
  cliAuthLogin,
  cliAuthLogout,
  cliComment,
  cliDoctor,
  cliInbox,
  cliLog,
  cliLs,
  cliMv,
  cliPurge,
  cliRestore,
  cliRestoreAt,
  cliRm,
  cliSchemaCheck,
  cliSchemaMigrate,
  cliScopeRm,
  cliScopeSet,
  cliSet,
  cliStoreAdd,
  cliStoreDefault,
  cliStoreDiscover,
  cliStoreInit,
  cliStoreRemove,
  cliStoreSet,
  cliSync,
  cliViewRename,
  cliViewRm,
  cliViewSave,
  cliViewShow,
  cliWatch,
  cliWorkspaceAdd,
  cliWorkspaceRename,
  cliWorkspaceRm,
  cliWorkspaceSetPrefix,
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
      'hh add "Switch DNS" --no-repo --store shared --workspace main --parent REL-8 --kind task --body "line 1\nline 2" -p points=2 -p priority=high',
    );
    expect(cliAdd({ title: "x" })).toBe("hh add x --no-repo");
    expect(cliAdd({ title: "x", parent: "REL-8", after: "REL-9" })).toBe(
      "hh add x --no-repo --parent REL-8 --after REL-9",
    );
    expect(cliAdd({ title: "x", before: "REL-9" })).toBe("hh add x --no-repo --before REL-9");
  });

  it("writes hh ls", () => {
    expect(cliLs("", true)).toBe("hh ls --tree");
    expect(cliLs("status:todo assignee:alice", false)).toBe('hh ls "status:todo assignee:alice"');
    expect(cliLs("kind:project", false)).toBe("hh ls kind:project");
    expect(cliLs("workspace:main -status:canceled", true, "shared")).toBe(
      'hh ls --store shared --tree "workspace:main -status:canceled"',
    );
  });

  it("writes hh inbox", () => {
    expect(cliInbox()).toBe("hh inbox");
    expect(cliInbox({ all: true })).toBe("hh inbox --all");
    expect(cliInbox({ all: true, store: "shared" })).toBe("hh inbox --all --store shared");
  });

  it("writes hh log, with the workspaces by name", () => {
    expect(cliLog([])).toBe("hh log");
    expect(cliLog(["release", "payments"])).toBe("hh log --workspace release,payments");
    expect(cliLog(["my work"])).toBe('hh log --workspace "my work"');
  });

  it("writes hh doctor, with --fix and --store", () => {
    expect(cliDoctor({})).toBe("hh doctor");
    expect(cliDoctor({ fix: true })).toBe("hh doctor --fix");
    expect(cliDoctor({ fix: true, store: "shared" })).toBe("hh doctor --fix --store shared");
    expect(cliDoctor({ store: "shared" })).toBe("hh doctor --store shared");
  });

  it("writes hh purge --older-than <N>d", () => {
    expect(cliPurge(90)).toBe("hh purge --older-than 90d");
    expect(cliPurge(0, "shared")).toBe("hh purge --older-than 0d --store shared");
  });

  it("puts -- before an argument that starts with -", () => {
    expect(cliLs("-status:canceled", false)).toBe("hh ls -- -status:canceled");
    expect(cliLs("-status:canceled", true, "shared")).toBe(
      "hh ls --store shared --tree -- -status:canceled",
    );
    expect(cliAdd({ title: "-x", store: "demo" })).toBe("hh add --no-repo --store demo -- -x");
    expect(cliAdd({ title: "-x", body: "-y" })).toBe("hh add --no-repo --body -y -- -x");
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

  it("writes hh mv --workspace, and hh workspace rename, set-prefix and rm", () => {
    expect(cliMv("REL-6", { workspace: "shared" })).toBe("hh mv REL-6 --workspace shared");
    expect(cliMv("REL-6", { workspace: new Placeholder("ワークスペース") })).toBe(
      "hh mv REL-6 --workspace <ワークスペース>",
    );
    expect(cliWorkspaceRename({ workspace: "release", name: "releases", store: "shared" })).toBe(
      "hh workspace rename release releases --store shared",
    );
    expect(
      cliWorkspaceRename({
        workspace: "release",
        name: new Placeholder("新しい名前"),
        store: "shared",
      }),
    ).toBe("hh workspace rename release <新しい名前> --store shared");
    expect(cliWorkspaceSetPrefix({ workspace: "release", prefix: "RLS", store: "shared" })).toBe(
      "hh workspace set-prefix release RLS --store shared",
    );
    expect(cliWorkspaceRm({ workspace: "sandbox", store: "shared" })).toBe(
      "hh workspace rm sandbox --store shared",
    );
  });

  it("writes hh view save with a flag for each setting the body holds", () => {
    expect(
      cliViewSave({
        name: "weekly",
        store: "shared",
        body: {
          layout: "table",
          query: "status:todo assignee:@me",
          columns: ["id", "title", "due"],
          group_by: "parent",
        },
      }),
    ).toBe(
      'hh view save weekly --layout table --query "status:todo assignee:@me" --columns id,title,due --group-by parent --store shared',
    );
    expect(
      cliViewSave({
        name: "board",
        body: { layout: "kanban", kanban_columns: "category", swimlane: "assignee" },
      }),
    ).toBe("hh view save board --layout kanban --kanban-columns category --swimlane assignee");
    expect(cliViewSave({ name: "board", body: {} })).toBe("hh view save board");
    expect(cliViewSave({ name: "board", body: { query: "" } })).toBe(
      'hh view save board --query ""',
    );
    expect(cliViewSave({ name: "w", body: { query: "-status:canceled" } })).toBe(
      "hh view save w --query -status:canceled",
    );
    expect(
      cliViewSave({ name: new Placeholder("名前"), store: "a b", body: { layout: "tree" } }),
    ).toBe('hh view save <名前> --layout tree --store "a b"');
    expect(cliViewSave({ name: "-x", body: { layout: "tree" } })).toBe(
      "hh view save --layout tree -- -x",
    );
  });

  it("writes hh view show, rm and rename; --store only when it is given", () => {
    expect(cliViewShow("weekly")).toBe("hh view show weekly");
    expect(cliViewShow("今週の作業", "shared")).toBe("hh view show 今週の作業 --store shared");
    expect(cliViewRm("weekly", "shared")).toBe("hh view rm weekly --store shared");
    expect(cliViewRm("weekly")).toBe("hh view rm weekly");
    expect(cliViewRename("weekly", "monthly", "shared")).toBe(
      "hh view rename weekly monthly --store shared",
    );
    expect(cliViewRename("weekly", new Placeholder("新しい名前"))).toBe(
      "hh view rename weekly <新しい名前>",
    );
  });

  it("writes the others", () => {
    expect(cliRm("REL-8")).toBe("hh rm REL-8");
    expect(cliRestore("REL-8")).toBe("hh restore REL-8");
    expect(cliRestoreAt("REL-8", "abc123")).toBe("hh restore REL-8 --at abc123");
    expect(cliComment("REL-8", "add")).toBe("hh comment add REL-8");
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

  it("writes hh store init with the account when it is chosen", () => {
    expect(cliStoreInit({ name: "shared", prefix: "WEB", account: "work" })).toBe(
      "hh store init shared --prefix WEB --account work",
    );
    expect(
      cliStoreInit({ name: "shared", prefix: "WEB", remotes: ["git@x:a/b.git"], account: "work" }),
    ).toBe("hh store init shared --prefix WEB --remote git@x:a/b.git --account work");
  });

  it("writes hh auth login; the host only when it is not github.com, and never a token", () => {
    expect(cliAuthLogin({})).toBe("hh auth login");
    expect(cliAuthLogin({ name: "work" })).toBe("hh auth login work");
    expect(cliAuthLogin({ name: "work", host: "github.com" })).toBe("hh auth login work");
    expect(cliAuthLogin({ name: "work", host: "ghe.example.com" })).toBe(
      "hh auth login work --host ghe.example.com",
    );
    expect(cliAuthLogin({ host: "ghe.example.com" })).toBe("hh auth login --host ghe.example.com");
    expect(cliAuthLogin({ name: "work", withToken: true })).toBe("hh auth login work --with-token");
    expect(cliAuthLogin({ name: new Placeholder("名前"), withToken: true, host: "h.io" })).toBe(
      "hh auth login <名前> --with-token --host h.io",
    );
    expect(cliAuthLogin({ name: "-x" })).toBe("hh auth login -- -x");
  });

  it("writes hh auth add and hh auth logout", () => {
    expect(cliAuthAdd({ name: "work", login: "alice" })).toBe("hh auth add work --login alice");
    expect(cliAuthAdd({ name: "work", login: "alice", token: "gh" })).toBe(
      "hh auth add work --login alice --token gh",
    );
    expect(
      cliAuthAdd({ name: "work", login: "alice", host: "ghe.io", token: "env:GH_TOKEN" }),
    ).toBe("hh auth add work --login alice --host ghe.io --token env:GH_TOKEN");
    expect(cliAuthAdd({ name: "work", login: "alice", host: "github.com" })).toBe(
      "hh auth add work --login alice",
    );
    expect(cliAuthAdd({ name: new Placeholder("名前"), login: new Placeholder("login") })).toBe(
      "hh auth add <名前> --login <login>",
    );
    expect(cliAuthLogout("work")).toBe("hh auth logout work");
  });

  it("writes hh store discover, add, remove, default and set", () => {
    expect(cliStoreDiscover({})).toBe("hh store discover");
    expect(cliStoreDiscover({ account: "work" })).toBe("hh store discover --account work");
    expect(cliStoreDiscover({ from: "~/dev dir" })).toBe('hh store discover --from "~/dev dir"');
    expect(cliStoreDiscover({ account: "work", from: "/src" })).toBe(
      "hh store discover --account work --from /src",
    );

    expect(cliStoreAdd({ name: "shared", remotes: ["git@x:a/b.git"] })).toBe(
      "hh store add shared --remote git@x:a/b.git",
    );
    expect(cliStoreAdd({ name: "shared", remotes: ["u1", "u2"], account: "work" })).toBe(
      "hh store add shared --remote u1 --remote u2 --account work",
    );
    expect(cliStoreAdd({ name: new Placeholder("名前"), remotes: [new Placeholder("URL")] })).toBe(
      "hh store add <名前> --remote <URL>",
    );

    expect(cliStoreRemove("shared", false)).toBe("hh store remove shared");
    expect(cliStoreRemove("shared", true)).toBe("hh store remove shared --keep");
    expect(cliStoreDefault("shared")).toBe("hh store default shared");

    expect(cliStoreSet({ name: "shared", color: "teal" })).toBe("hh store set shared --color teal");
    expect(cliStoreSet({ name: "shared", color: "" })).toBe('hh store set shared --color ""');
    expect(cliStoreSet({ name: "shared", hidden: true })).toBe("hh store set shared --hidden=true");
    expect(cliStoreSet({ name: "shared", hidden: false })).toBe(
      "hh store set shared --hidden=false",
    );
    expect(cliStoreSet({ name: "shared", color: "pink", hidden: true })).toBe(
      "hh store set shared --color pink --hidden=true",
    );
  });

  it("writes hh schema scope rm; --store only when it is given", () => {
    expect(cliScopeRm("REL-15")).toBe("hh schema scope rm REL-15");
    expect(cliScopeRm("REL-15", "shared")).toBe("hh schema scope rm REL-15 --store shared");
  });

  it("writes hh schema scope set with the parts that are given; an empty part takes it off", () => {
    expect(
      cliScopeSet({
        id: "REL-15",
        statuses: ["todo", "done"],
        transitions: { todo: ["in_progress", "done"], waiting: ["done"] },
        required: { in_progress: ["points", "assignees"] },
        store: "shared",
      }),
    ).toBe(
      "hh schema scope set REL-15 --statuses todo,done --transition todo=in_progress,done" +
        " --transition waiting=done --required in_progress=points,assignees --store shared",
    );
    expect(cliScopeSet({ id: "REL-15", statuses: [], transitions: {}, required: {} })).toBe(
      'hh schema scope set REL-15 --statuses "" --transition "" --required ""',
    );
    expect(cliScopeSet({ id: "REL-15", required: { done: ["points"] } })).toBe(
      "hh schema scope set REL-15 --required done=points",
    );
    expect(cliScopeSet({ id: "REL-15", store: "shared" })).toBe(
      "hh schema scope set REL-15 <変更> --store shared",
    );
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

  it("writes hh schema check, with --store only when asked", () => {
    expect(cliSchemaCheck()).toBe("hh schema check");
    expect(cliSchemaCheck("shared")).toBe("hh schema check --store shared");
  });

  it("writes hh schema migrate for each target; value takes the property first", () => {
    expect(cliSchemaMigrate({ target: "status", old: "triage", new: "todo" })).toBe(
      "hh schema migrate status triage todo",
    );
    expect(cliSchemaMigrate({ target: "kind", old: "bug", new: "defect", store: "a b" })).toBe(
      'hh schema migrate kind bug defect --store "a b"',
    );
    expect(cliSchemaMigrate({ target: "prop", old: "area", new: "team" })).toBe(
      "hh schema migrate prop area team",
    );
    expect(cliSchemaMigrate({ target: "value", prop: "priority", old: "mid", new: "normal" })).toBe(
      "hh schema migrate value priority mid normal",
    );
  });

  it("puts a placeholder where the new name goes while it is not typed, and guards a leading -", () => {
    const next = new Placeholder("新しい名前");
    expect(cliSchemaMigrate({ target: "status", old: "triage", new: next })).toBe(
      "hh schema migrate status triage <新しい名前>",
    );
    expect(cliSchemaMigrate({ target: "status", old: "triage", new: "-x" })).toBe(
      "hh schema migrate status -- triage -x",
    );
  });
});
