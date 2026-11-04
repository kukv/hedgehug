import { type Api, type TaskView, unwrap } from "../../core/api/client";
import { quoteArg } from "../../core/cli";
import { searchTasks as findTasks } from "../../core/search";
import type { WorkspaceEntry } from "../../core/stores";
import type { ViewEntry } from "../../core/views";
import { kbdLabel } from "../keys";
import type { Layout } from "../router";

export type PaletteMode = "id" | "query" | "command";

// A word of a query that has no colon: a comparison such as due<2026-11-10.
const COMPARISON = /^-?[A-Za-z_][\w-]*(<=|>=|<|>)\S/;
// Keep in sync with ID in core/stores.ts and Go model.ParseTaskID.
const ID = /^[A-Za-z][A-Za-z0-9]*-\d+$/;
const ULID = /^[0-9A-HJKMNP-TV-Za-hjkmnp-tv-z]{26}$/;
const MAX = 20;

// isStoreQualifiedId tells "<store>:<ID>" whose store is one of storeNames.
export function isStoreQualifiedId(input: string, storeNames: string[]): boolean {
  const i = input.indexOf(":");
  return i > 0 && storeNames.includes(input.slice(0, i)) && ID.test(input.slice(i + 1));
}

// detectMode tells which of the three the input is. IDs, former IDs, ULIDs and
// titles are one mode: the same input is looked up as all of them.
// With storeNames, "<store>:<ID>" for a store of the names is an ID too.
export function detectMode(input: string, storeNames?: string[]): PaletteMode {
  const s = input.trimStart();
  if (s.startsWith(">")) return "command";
  if (storeNames && isStoreQualifiedId(s.trimEnd(), storeNames)) return "id";
  const words = s.trimEnd().split(/\s+/).filter(Boolean);
  if (words.some((w) => w.includes(":") || COMPARISON.test(w))) return "query";
  return "id";
}

export type TaskItem = {
  type: "task";
  key: string;
  ulid: string;
  id: string;
  title: string;
  kind: string;
  store: string;
  note?: string;
  href: string;
};

export type CommandItem = {
  type: "command";
  key: string;
  label: string;
  icon:
    | "sync"
    | "plus"
    | "status"
    | "folder"
    | "workspace"
    | "settings"
    | "undo"
    | "checklist"
    | "bell"
    | "activity"
    | "filter";
  kbd?: string;
  note?: string;
  store?: string;
  href?: string; // where it goes, for a new tab
  keepOpen?: boolean; // the palette stays open and shows something else
  run: () => void;
};

export type PaletteItem = TaskItem | CommandItem;
export type PaletteGroup = { name: string; items: PaletteItem[] };

type TaskLike = { ulid: string; id: string; title: string; kind: string; store: string };

function taskItem(t: TaskLike, hrefOf: (ulid: string) => string, note?: string): TaskItem {
  return {
    type: "task",
    key: `task:${t.ulid}`,
    ulid: t.ulid,
    id: t.id,
    title: t.title,
    kind: t.kind,
    store: t.store,
    href: hrefOf(t.ulid),
    ...(note ? { note } : {}),
  };
}

export type SearchDeps = {
  api: Api;
  hrefOf: (ulid: string) => string;
  // The point a read begins at (take it before the request is sent), and what puts what the search found in the
  // cache, as a read of that point.
  epoch: () => number;
  prime: (tasks: TaskView[], since: number) => void;
  // The names of the stores that are hidden, read for each search: what they hold is not found. The
  // query leaves them out (-store:<name>), so they take none of the places of the list.
  hidden?: () => string[];
  // every mounted Store, also a hidden one: a "<store>:<ID>" is looked up for any of them
  storeNames?: () => string[];
};

export type SearchResult = { groups: PaletteGroup[]; error: unknown };

function isNotFound(e: unknown): boolean {
  return typeof e === "object" && e !== null && (e as { status?: number }).status === 404;
}

// matchNote says how an exact lookup matched when it was not by the current ID.
function matchNote(s: { id: string; ulid: string; former_ids?: string[] | null }, text: string) {
  const t = text.toLowerCase();
  if (s.id.toLowerCase() === t || s.ulid.toLowerCase() === t) return undefined;
  if ((s.former_ids ?? []).some((f) => f.toLowerCase() === t)) {
    return `旧 ID ${text.toUpperCase()} で一致`;
  }
  return undefined;
}

// searchTasks looks the input up in every store: as an ID, a former ID or a
// ULID (an exact lookup, first), and as words of an ID or a title; or, for the
// query mode, runs it as a query.
export async function searchTasks(
  d: SearchDeps,
  input: string,
  mode: "id" | "query",
): Promise<SearchResult> {
  const text = input.trim();
  if (text === "") return { groups: [], error: null };
  const since = d.epoch();
  const hidden = d.hidden?.() ?? [];
  const without = hidden.map((s) => `-store:${quoteArg(s)}`).join(" ");
  try {
    if (mode === "query") {
      const list = await unwrap(
        d.api.GET("/api/tasks", {
          params: { query: { q: [without, text].filter((t) => t !== "").join(" "), tree: false } },
        }),
      );
      const tasks = list.tasks ?? [];
      d.prime(tasks, since);
      const items = tasks.slice(0, MAX).map((t) => taskItem(t, d.hrefOf));
      return {
        groups: items.length ? [{ name: `クエリに一致 · ${tasks.length} 件`, items }] : [],
        error: null,
      };
    }
    const found = new Map<string, TaskItem>();
    // An exact lookup (an ID, a ULID, "<store>:<ID>") opens the task like a URL does, so a hidden
    // Store does not filter it; only the words below do.
    const qualified = isStoreQualifiedId(text, d.storeNames?.() ?? []);
    if (qualified || ID.test(text) || ULID.test(text)) {
      try {
        const s = await unwrap(d.api.GET("/api/tasks/{id}", { params: { path: { id: text } } }));
        found.set(s.ulid, taskItem(s, d.hrefOf, matchNote(s, text)));
      } catch (e) {
        if (!isNotFound(e)) throw e;
      }
    }
    if (qualified) {
      const items = [...found.values()];
      return { groups: items.length ? [{ name: "タスク", items }] : [], error: null };
    }
    const tasks = await findTasks(d.api, text, { extra: without, limit: MAX });
    d.prime(tasks, since);
    for (const t of tasks) {
      if (!found.has(t.ulid)) found.set(t.ulid, taskItem(t, d.hrefOf));
    }
    const items = [...found.values()].slice(0, MAX);
    return { groups: items.length ? [{ name: "タスク", items }] : [], error: null };
  } catch (e) {
    return { groups: [], error: e };
  }
}

export type CommandEnv = {
  syncNow: () => void;
  createTask: (parent?: string) => void;
  createProject: () => void;
  addWorkspace: () => void;
  undo: () => void;
  saveQuery: (q: string) => void; // opens the dialog that saves the query as a view
  workspaces: WorkspaceEntry[];
  views: ViewEntry[]; // the saved views of every store
  storeNames: string[]; // every mounted store, also one with no workspace
  go: (path: string) => void;
  hrefs: {
    workspace: (store: string, ulid: string) => string;
    workspaceSettings: (store: string, ulid: string) => string;
    settingsWorkspaces: (store: string) => string;
    settingsSchema: (store: string) => string;
    settingsGeneral: () => string;
    settingsAccounts: () => string;
    settingsStores: () => string;
    project: (ulid: string) => string;
    projectSettings: (ulid: string) => string;
    task: (ulid: string) => string;
    view: (store: string, name: string) => string;
    query: (layout: Layout, q: string) => string;
    my: () => string;
    inbox: () => string;
    activity: () => string;
  };
  selected: TaskView | null; // the task the commands for a task are about
  project: { ulid: string; title: string } | null; // the project the screen is of (its page or its detail)
  recent: TaskView[]; // the tasks opened last, the latest first
  pickStatus: (t: TaskView) => void;
  moveTask: (t: TaskView) => void;
  readOnly: (store: string) => string | null; // why a store cannot be written, or null
  displayId: (store: string, id: string) => string; // the ID with its Store where the prefix is shared
};

// taskCommands are the commands about one task.
export function taskCommands(env: CommandEnv, t: TaskView): PaletteGroup {
  const href = env.hrefs.task(t.ulid);
  // A task goes to another workspace of its own store: with none, there is nothing to offer.
  // Every one of them writes to the store of the task (細部 35): a read-only store is only read.
  const writable = env.readOnly(t.store) === null;
  const canMove = env.workspaces.some((w) => w.store === t.store && w.ulid !== t.workspace.ulid);
  return {
    name: `${env.displayId(t.store, t.id)} に対して`,
    items: [
      ...(writable
        ? [
            {
              type: "command" as const,
              key: `task-status:${t.ulid}`,
              label: "ステータスを変更…",
              icon: "status" as const,
              keepOpen: true,
              run: () => env.pickStatus(t),
            },
            {
              type: "command" as const,
              key: `task-child:${t.ulid}`,
              label: "子タスクを作る",
              icon: "plus" as const,
              kbd: "C",
              run: () => env.createTask(t.ulid),
            },
          ]
        : []),
      {
        type: "command",
        key: `task-open:${t.ulid}`,
        label: "詳細を開く",
        icon: "folder",
        kbd: kbdLabel("mod+enter"),
        href,
        run: () => env.go(href),
      },
      ...(canMove && writable
        ? [
            {
              type: "command" as const,
              key: `task-move:${t.ulid}`,
              label: "別のワークスペースへ移動…",
              icon: "workspace" as const,
              run: () => env.moveTask(t),
            },
          ]
        : []),
    ],
  };
}

// queryCommands are what the palette offers for the query that was typed: the screen of the
// query as a table (nothing is saved), and the dialog that saves it as a view. The bulk update
// of the matches the design shows is not made (phase 6).
export function queryCommands(env: CommandEnv, q: string): PaletteGroup {
  const href = env.hrefs.query("table", q);
  return {
    name: "このクエリで",
    items: [
      {
        type: "command",
        key: "query-table",
        label: "テーブルで開く",
        icon: "filter",
        href,
        run: () => env.go(href),
      },
      {
        type: "command",
        key: "query-save",
        label: "ビューとして保存…",
        icon: "filter",
        run: () => env.saveQuery(q),
      },
    ],
  };
}

// buildCommands lists the commands, then where to go, then the commands for
// the selected task. text is the input, with or without the leading >; a word
// in it narrows the list by its label and note.
export function buildCommands(env: CommandEnv, text: string): PaletteGroup[] {
  const word = text.replace(/^\s*>/, "").trim().toLowerCase();
  const keep = (i: CommandItem) =>
    word === "" || `${i.label} ${i.note ?? ""}`.toLowerCase().includes(word);
  // A create needs a workspace of a store that can be written, a workspace add a store (細部 99).
  const canCreate = env.workspaces.some((w) => env.readOnly(w.store) === null);
  const canAddWorkspace = env.storeNames.some((s) => env.readOnly(s) === null);

  const commands: CommandItem[] = [
    {
      type: "command",
      key: "cmd:sync",
      label: "今すぐ同期する",
      icon: "sync",
      kbd: kbdLabel("shift+mod+s"),
      run: env.syncNow,
    },
    {
      type: "command",
      key: "cmd:create-task",
      label: "タスクを作成",
      icon: "plus",
      kbd: "C",
      run: () => env.createTask(),
    },
    {
      type: "command",
      key: "cmd:create-project",
      label: "プロジェクトを作成",
      icon: "plus",
      run: env.createProject,
    },
    {
      type: "command",
      key: "cmd:workspace-add",
      label: "ワークスペースを追加",
      icon: "plus",
      run: env.addWorkspace,
    },
    {
      type: "command",
      key: "cmd:undo",
      label: "直前の操作を取り消す",
      icon: "undo",
      run: env.undo,
    },
  ];
  const writes: Record<string, boolean> = {
    "cmd:create-task": canCreate,
    "cmd:create-project": canCreate,
    "cmd:workspace-add": canAddWorkspace,
  };

  // The page asks for the diagnosis when it is opened.
  const doctorHref = env.hrefs.settingsGeneral();
  commands.push({
    type: "command",
    key: "cmd:doctor",
    label: "doctor を実行",
    icon: "checklist",
    href: doctorHref,
    run: () => env.go(doctorHref),
  });

  const myHref = env.hrefs.my();
  const my: CommandItem = {
    type: "command",
    key: "open:my",
    label: "マイタスクを開く",
    icon: "checklist",
    href: myHref,
    run: () => env.go(myHref),
  };
  const inboxHref = env.hrefs.inbox();
  const inbox: CommandItem = {
    type: "command",
    key: "open:inbox",
    label: "通知を開く",
    icon: "bell",
    href: inboxHref,
    run: () => env.go(inboxHref),
  };
  const activityHref = env.hrefs.activity();
  const activity: CommandItem = {
    type: "command",
    key: "open:activity",
    label: "アクティビティを開く",
    icon: "activity",
    href: activityHref,
    run: () => env.go(activityHref),
  };
  const saved: CommandItem[] = env.views.map((v) => {
    const href = env.hrefs.view(v.store, v.view.name);
    return {
      type: "command",
      key: `open:view:${v.store}:${v.view.name}`,
      label: `${v.view.name} を開く`,
      icon: "filter",
      note: "保存済みビュー",
      store: v.store,
      href,
      run: () => env.go(href),
    };
  });

  const places: CommandItem[] = env.workspaces.flatMap((w) => {
    const wsHref = env.hrefs.workspace(w.store, w.ulid);
    const ws: CommandItem = {
      type: "command",
      key: `open:ws:${w.store}:${w.ulid}`,
      label: `${w.name} を開く`,
      icon: "workspace",
      store: w.store,
      href: wsHref,
      run: () => env.go(wsHref),
    };
    const settingsHref = env.hrefs.workspaceSettings(w.store, w.ulid);
    const settings: CommandItem = {
      type: "command",
      key: `open:ws-settings:${w.store}:${w.ulid}`,
      label: "ワークスペースの設定を開く",
      icon: "settings",
      note: w.name,
      store: w.store,
      href: settingsHref,
      run: () => env.go(settingsHref),
    };
    const projects = w.projects.map((p): CommandItem => {
      const href = env.hrefs.project(p.ulid);
      return {
        type: "command",
        key: `open:project:${p.ulid}`,
        label: `${p.title} を開く`,
        icon: "folder",
        note: w.name,
        store: w.store,
        href,
        run: () => env.go(href),
      };
    });
    return [ws, settings, ...projects];
  });

  // The page that lists the workspaces of a store, in the settings.
  const lists: CommandItem[] = env.storeNames.map((store) => {
    const href = env.hrefs.settingsWorkspaces(store);
    return {
      type: "command",
      key: `open:settings-workspaces:${store}`,
      label: "設定のワークスペース一覧を開く",
      icon: "settings",
      store,
      href,
      run: () => env.go(href),
    };
  });

  // The schema of a store, in the settings.
  const schemas: CommandItem[] = env.storeNames.map((store) => {
    const href = env.hrefs.settingsSchema(store);
    return {
      type: "command",
      key: `open:settings-schema:${store}`,
      label: "設定のスキーマを開く",
      icon: "settings",
      store,
      href,
      run: () => env.go(href),
    };
  });

  // The sync and the upkeep of this machine: no Store, so one command.
  const generalHref = env.hrefs.settingsGeneral();
  const general: CommandItem = {
    type: "command",
    key: "open:settings-general",
    label: "設定の同期と保守を開く",
    icon: "settings",
    href: generalHref,
    run: () => env.go(generalHref),
  };

  // The accounts and the mounted Stores of this machine: no Store either.
  const accountsHref = env.hrefs.settingsAccounts();
  const accounts: CommandItem = {
    type: "command",
    key: "open:settings-accounts",
    label: "設定のアカウントと認証を開く",
    icon: "settings",
    href: accountsHref,
    run: () => env.go(accountsHref),
  };
  const storesHref = env.hrefs.settingsStores();
  const mounted: CommandItem = {
    type: "command",
    key: "open:settings-stores",
    label: "設定のマウント中の Store を開く",
    icon: "settings",
    href: storesHref,
    run: () => env.go(storesHref),
  };

  const inProject = env.project;
  const projectSettingsHref = inProject ? env.hrefs.projectSettings(inProject.ulid) : "";
  const projectSettings: CommandItem[] = inProject
    ? [
        {
          type: "command",
          key: `open:project-settings:${inProject.ulid}`,
          label: "プロジェクトの設定を開く",
          icon: "settings",
          note: inProject.title || undefined,
          href: projectSettingsHref,
          run: () => env.go(projectSettingsHref),
        },
      ]
    : [];

  const recent = env.recent
    .filter(
      (t) =>
        word === "" ||
        `${t.id} ${env.displayId(t.store, t.id)} ${t.title}`.toLowerCase().includes(word),
    )
    .map((t) => taskItem(t, env.hrefs.task));

  const groups: PaletteGroup[] = [
    { name: "コマンド", items: commands.filter((i) => writes[i.key] !== false && keep(i)) },
    { name: "最近開いたもの", items: recent },
    {
      name: "開く",
      items: [
        ...places,
        ...lists,
        ...schemas,
        accounts,
        mounted,
        general,
        ...projectSettings,
        my,
        inbox,
        activity,
        ...saved,
      ].filter(keep),
    },
  ];
  if (env.selected) {
    const g = taskCommands(env, env.selected);
    groups.push({ ...g, items: g.items.filter((i) => i.type === "command" && keep(i)) });
  }
  return groups.filter((g) => g.items.length > 0);
}
