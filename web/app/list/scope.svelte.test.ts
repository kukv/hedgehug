import { flushSync } from "svelte";
import { describe, expect, it, vi } from "vitest";
import { ApiError, type TaskView } from "../../core/api/client";
import type { SchemaView } from "../../core/schema";
import type { WorkspaceEntry } from "../../core/stores";
import { emitter } from "../test-utils";
import { entry } from "../workspace/testkit";
import type { ListScope } from "./scope";
import { useListScope } from "./scope.svelte";

const h = vi.hoisted(() => ({ core: undefined as unknown }));
vi.mock("../context", () => ({ core: () => h.core }));

const task = (over: Partial<TaskView> = {}) =>
  ({
    ulid: "P1",
    id: "REL-15",
    store: "shared",
    kind: "project",
    title: "リリース準備",
    workspace: { ulid: "W1", name: "release", id_prefix: "REL" },
    ...over,
  }) as TaskView;

const schema = {
  statuses: [],
  kinds: [],
  props: {},
  workflow: {},
  scoped: [],
} as unknown as SchemaView;

type World = {
  tasks?: Record<string, TaskView>;
  workspaces?: WorkspaceEntry[];
  stores?: string[];
  loaded?: boolean;
  schemas?: Record<string, SchemaView>;
  detailError?: ApiError | null;
};

function setup(world: World = {}) {
  const cache = emitter();
  const stores = emitter();
  const schemas = emitter();
  const w = { workspaces: [entry()], stores: ["shared"], loaded: true, ...world };
  const disposed = vi.fn();
  const watched: string[] = [];
  const loads: string[] = [];
  h.core = {
    cache: {
      subscribe: cache.subscribe,
      task: (u: string) => w.tasks?.[u],
      watchDetail: (u: string) => {
        watched.push(u);
        return { error: () => w.detailError ?? null, dispose: disposed };
      },
    },
    stores: {
      subscribe: stores.subscribe,
      workspace: (s: string, u: string) => w.workspaces.find((x) => x.store === s && x.ulid === u),
      workspaces: () => w.workspaces,
      readOnly: () => null,
      stores: () => w.stores.map((name) => ({ name })),
      loaded: () => w.loaded,
    },
    schema: {
      subscribe: schemas.subscribe,
      get: (s: string) => w.schemas?.[s],
      load: (s: string) => {
        loads.push(s);
        return Promise.resolve(schema);
      },
    },
  };
  return { w, cache, stores, schemas, disposed, watched, loads };
}

type Scope = ReturnType<typeof useListScope>;
// The fields as an effect last read them: a value that depends on the core is read in an effect, as a
// component does, since the core's notifications only reach what is read there.
type Snapshot = {
  -readonly [K in
    | "ready"
    | "notFound"
    | "error"
    | "apiBase"
    | "cliBase"
    | "store"
    | "workspace"
    | "project"
    | "saveBase"
    | "crumbTitle"
    | "emptyProps"
    | "notFoundProps"
    | "schemas"]: Scope[K];
};

function use(get: () => ListScope) {
  const scope = {} as Snapshot;
  const stop = $effect.root(() => {
    const s = useListScope(get);
    scope.schemas = s.schemas;
    $effect(() => {
      scope.ready = s.ready;
      scope.notFound = s.notFound;
      scope.error = s.error;
      scope.apiBase = s.apiBase;
      scope.cliBase = s.cliBase;
      scope.store = s.store;
      scope.workspace = s.workspace;
      scope.project = s.project;
      scope.saveBase = s.saveBase;
      scope.crumbTitle = s.crumbTitle;
      scope.emptyProps = s.emptyProps;
      scope.notFoundProps = s.notFoundProps;
    });
  });
  flushSync();
  return { scope, stop };
}

describe("useListScope: a workspace", () => {
  it("is ready with the workspace, and its query names the workspace", () => {
    setup();
    const { scope, stop } = use(() => ({ kind: "workspace", store: "shared", ulid: "W1" }));
    expect(scope.ready).toBe(true);
    expect(scope.notFound).toBe(false);
    expect(scope.store).toBe("shared");
    expect(scope.workspace?.name).toBe("release");
    expect(scope.project).toBeUndefined();
    expect(scope.apiBase).toBe("workspace:release");
    expect(scope.cliBase).toBe("workspace:release");
    expect(scope.saveBase).toBe("workspace:release");
    stop();
  });

  it("quotes a name with a space, and puts the store first when two or more are mounted", () => {
    setup({ workspaces: [entry({ name: "web app" })], stores: ["shared", "personal"] });
    const { scope, stop } = use(() => ({ kind: "workspace", store: "shared", ulid: "W1" }));
    expect(scope.cliBase).toBe('workspace:"web app"');
    expect(scope.saveBase).toBe('store:shared workspace:"web app"');
    stop();
  });

  it("is not ready while the stores are not read, and not found once they are", () => {
    const { w, stores } = setup({ workspaces: [], loaded: false });
    const { scope, stop } = use(() => ({ kind: "workspace", store: "shared", ulid: "W1" }));
    expect(scope.ready).toBe(false);
    expect(scope.notFound).toBe(false);
    expect(scope.apiBase).toBe("");
    expect(scope.saveBase).toBe("");
    w.loaded = true;
    stores.emit();
    flushSync();
    expect(scope.notFound).toBe(true);
    stop();
  });

  it("tells an effect that reads the query only when the query changes, not when the workspace is read again", () => {
    const { w, stores } = setup();
    let runs = 0;
    const stop = $effect.root(() => {
      const s = useListScope(() => ({ kind: "workspace", store: "shared", ulid: "W1" }));
      $effect(() => {
        // What the tree reads to start its query: reading it again would reload the tree.
        const read = [s.ready, s.apiBase, s.cliBase, s.store];
        if (read.length > 0) runs++;
      });
    });
    flushSync();
    const before = runs;
    // The stores are read again (a task was made): an equal workspace, a new object.
    w.workspaces = [entry()];
    stores.emit();
    flushSync();
    expect(runs).toBe(before);
    stop();
  });

  it("follows the scope it is given", () => {
    setup({ workspaces: [entry(), entry({ ulid: "W2", name: "web" })] });
    let current = $state<ListScope>({ kind: "workspace", store: "shared", ulid: "W1" });
    const { scope, stop } = use(() => current);
    expect(scope.cliBase).toBe("workspace:release");
    current = { kind: "workspace", store: "shared", ulid: "W2" };
    flushSync();
    expect(scope.cliBase).toBe("workspace:web");
    stop();
  });
});

describe("useListScope: a project", () => {
  it("reads the project from the cache: the API takes its ULID, the command its ID", () => {
    const { watched } = setup({ tasks: { P1: task() } });
    const { scope, stop } = use(() => ({ kind: "project", ulid: "P1" }));
    expect(scope.ready).toBe(true);
    expect(scope.project?.id).toBe("REL-15");
    expect(scope.store).toBe("shared");
    expect(scope.workspace?.ulid).toBe("W1");
    expect(scope.apiBase).toBe("project:P1");
    expect(scope.cliBase).toBe("project:REL-15");
    expect(scope.saveBase).toBe("project:REL-15");
    expect(watched).toEqual(["P1"]);
    stop();
  });

  it("puts the store first in saveBase when two or more are mounted", () => {
    setup({ tasks: { P1: task() }, stores: ["personal", "shared"] });
    const { scope, stop } = use(() => ({ kind: "project", ulid: "P1" }));
    expect(scope.saveBase).toBe("store:shared project:REL-15");
    stop();
  });

  it("is not ready until the project is fetched; a 404 is not found, another error is kept", () => {
    const { w, cache } = setup({ tasks: {} });
    const { scope, stop } = use(() => ({ kind: "project", ulid: "P1" }));
    expect(scope.ready).toBe(false);
    expect(scope.notFound).toBe(false);
    expect(scope.error).toBeNull();
    expect(scope.store).toBeUndefined();
    w.detailError = new ApiError({ status: 500, type: "internal" });
    cache.emit();
    flushSync();
    expect(scope.notFound).toBe(false);
    expect(scope.error?.status).toBe(500);
    w.detailError = new ApiError({ status: 404, type: "task.not_found" });
    cache.emit();
    flushSync();
    expect(scope.notFound).toBe(true);
    w.tasks = { P1: task() };
    cache.emit();
    flushSync();
    expect(scope.ready).toBe(true);
    expect(scope.error).toBeNull();
    stop();
  });

  it("lets go of the detail when it goes away", () => {
    const { disposed } = setup({ tasks: { P1: task() } });
    const { stop } = use(() => ({ kind: "project", ulid: "P1" }));
    expect(disposed).not.toHaveBeenCalled();
    stop();
    expect(disposed).toHaveBeenCalledTimes(1);
  });
});

describe("useListScope: every store", () => {
  it("is ready at once and has no fixed query, store, workspace or project", () => {
    const { watched } = setup({ stores: ["shared", "personal"] });
    const { scope, stop } = use(() => ({ kind: "all" }));
    expect(scope.ready).toBe(true);
    expect(scope.notFound).toBe(false);
    expect(scope.error).toBeNull();
    expect(scope.apiBase).toBe("");
    expect(scope.cliBase).toBe("");
    expect(scope.saveBase).toBe("");
    expect(scope.store).toBeUndefined();
    expect(scope.workspace).toBeUndefined();
    expect(scope.project).toBeUndefined();
    expect(watched).toEqual([]);
    stop();
  });
});

describe("useListScope: schemas", () => {
  it("gives the schemas that are loaded in the order asked, and fetches the others", () => {
    const other = { ...schema, kinds: [] } as SchemaView;
    const { loads, schemas, w } = setup({ schemas: { shared: schema } });
    const { scope, stop } = use(() => ({ kind: "all" }));
    expect(scope.schemas(["personal", "shared"])).toEqual([schema]);
    expect(loads).toEqual(["personal"]);
    w.schemas = { shared: schema, personal: other };
    schemas.emit();
    expect(scope.schemas(["personal", "shared"])).toEqual([other, schema]);
    expect(scope.schemas([])).toEqual([]);
    stop();
  });

  it("is read again when the schema cache notifies", () => {
    const { schemas, w } = setup({ schemas: {} });
    const { scope, stop } = use(() => ({ kind: "all" }));
    const seen: number[] = [];
    const stopRead = $effect.root(() => {
      $effect(() => {
        seen.push(scope.schemas(["shared"]).length);
      });
    });
    flushSync();
    w.schemas = { shared: schema };
    schemas.emit();
    flushSync();
    expect(seen).toEqual([0, 1]);
    stopRead();
    stop();
  });
});

describe("useListScope: what the empty and not-found screens are told", () => {
  it("names the workspace for a workspace, and the ULID in the not-found screen", () => {
    setup();
    const { scope, stop } = use(() => ({ kind: "workspace", store: "shared", ulid: "W1" }));
    expect(scope.crumbTitle).toBe("release");
    expect(scope.emptyProps).toEqual({ scope: "workspace", name: "release" });
    expect(scope.notFoundProps).toEqual({ scope: "workspace", name: "W1" });
    stop();
  });

  it("names the project for a project, and nothing in the not-found screen", () => {
    setup({ tasks: { P1: task() } });
    const { scope, stop } = use(() => ({ kind: "project", ulid: "P1" }));
    expect(scope.crumbTitle).toBe("リリース準備");
    expect(scope.emptyProps).toEqual({
      scope: "project",
      name: "リリース準備",
      importTarget: { store: task().store, parent: task().ulid, parentId: task().id },
    });
    expect(scope.notFoundProps).toEqual({ scope: "project", name: "" });
    stop();
  });

  it("has no name before the workspace is read", () => {
    setup({ workspaces: [], loaded: false });
    const { scope, stop } = use(() => ({ kind: "workspace", store: "shared", ulid: "W9" }));
    expect(scope.crumbTitle).toBe("");
    stop();
  });
});
