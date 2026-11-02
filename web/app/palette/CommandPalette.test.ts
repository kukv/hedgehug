import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { closeCreate, createState } from "../create-state.svelte";
import { closePalette, openPalette, palette } from "../palette-state.svelte";
import { selection } from "../selection.svelte";
import { schema, task } from "../tree/fixtures";

const h = vi.hoisted(() => ({
  core: undefined as unknown,
  route: { name: "home" } as { name: string; ulid?: string },
  go: vi.fn(),
  toast: vi.fn(),
}));

vi.mock("../context", () => ({ core: () => h.core }));
vi.mock("../router", () => ({
  router: {
    get route() {
      return h.route;
    },
    go: h.go,
    href: (r: { name: string; ulid?: string; store?: string }) =>
      r.name === "task"
        ? `/tasks/${r.ulid}`
        : r.name === "project"
          ? `/projects/${r.ulid}`
          : `/workspaces/${r.store}/${r.ulid}`,
  },
}));
vi.mock("../toast.svelte", () => ({ toast: h.toast }));

import CommandPalette from "./CommandPalette.svelte";

const ok = (data: unknown) => ({ data, response: new Response(null, { status: 200 }) });
const rel6 = task("U6", "task", null, {
  id: "REL-6",
  title: "Web の API クライアントを v2 に更新",
});

function setup() {
  const none = () => () => {};
  const ws = {
    store: "shared",
    tone: "purple",
    ulid: "W1",
    name: "release",
    prefix: "REL",
    projects: [],
  };
  const api = {
    GET: vi.fn(
      async (
        path: string,
        opts: { params: { path?: { id: string }; query?: { store?: string; q?: string } } },
      ) => {
        if (path === "/api/tasks/{id}") {
          const hit = opts.params.path?.id.toLowerCase() === "sh-17";
          return hit
            ? ok({ ...rel6, former_ids: ["SH-17"] })
            : {
                error: { type: "task.not_found", title: "NF", status: 404 },
                response: new Response(null, { status: 404 }),
              };
        }
        if (opts.params.query?.q?.startsWith("foo:")) {
          return {
            error: {
              type: "query.invalid",
              title: "Bad",
              status: 400,
              detail: "term 1 (foo:bar): unknown key",
            },
            response: new Response(null, { status: 400 }),
          };
        }
        return ok({ tasks: [] });
      },
    ),
    POST: vi.fn(async () => ok([])),
    PATCH: vi.fn(async () => ok({ task: { ...rel6, status: "done" }, commit: "c1" })),
  };
  h.core = {
    api,
    cache: {
      task: (u: string) => (u === "U6" ? rel6 : undefined),
      applyWrite: vi.fn(),
      epochNow: () => 0,
      prime: vi.fn(),
      subscribe: none,
    },
    stores: {
      stores: () => [{ name: "shared" }, { name: "backend" }],
      workspaces: () => [ws],
      tone: () => "purple",
      subscribe: none,
    },
    schema: { get: () => schema, load: async () => schema, subscribe: none },
  };
  return { api };
}

const box = () => screen.getByRole("combobox");

beforeEach(() => {
  h.go.mockReset();
  h.toast.mockReset();
  localStorage.clear();
  closePalette();
  closeCreate();
  h.route = { name: "home" };
  selection.set(null);
});

describe("CommandPalette", () => {
  it("renders nothing while closed and shows the dialog when opened", async () => {
    setup();
    render(CommandPalette);
    expect(screen.queryByRole("dialog")).toBeNull();
    openPalette();
    expect(await screen.findByRole("dialog", { name: "ID・検索・コマンド" })).toBeInTheDocument();
  });

  it("lists the tasks opened last when the input is empty", async () => {
    setup();
    localStorage.setItem("hedgehug.recent", JSON.stringify(["U6"]));
    render(CommandPalette);
    openPalette();
    expect(await screen.findByText("最近開いたもの")).toBeInTheDocument();
    expect(screen.getByText("Web の API クライアントを v2 に更新")).toBeInTheDocument();
  });

  it("closes with Escape", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.keyDown(await screen.findByRole("combobox"), { key: "Escape" });
    await waitFor(() => expect(palette.open).toBe(false));
  });

  it("runs a command: > narrows to the commands and Enter syncs", async () => {
    const { api } = setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">同期" } });
    expect(await screen.findByText("今すぐ同期する")).toBeInTheDocument();
    expect(screen.getByText("コマンド", { selector: "[data-mode]" })).toBeInTheDocument();
    await fireEvent.keyDown(box(), { key: "Enter" });
    await waitFor(() => expect(api.POST).toHaveBeenCalledWith("/api/sync"));
    expect(palette.open).toBe(false);
  });

  it("finds a task by a former ID, says so, and opens it with Enter", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: "sh-17" } });
    expect(await screen.findByText("旧 ID SH-17 で一致")).toBeInTheDocument();
    expect(screen.getByText("REL-6")).toBeInTheDocument();
    expect(screen.getByText("REL-6 に対して")).toBeInTheDocument();
    await fireEvent.keyDown(box(), { key: "Enter" });
    expect(h.go).toHaveBeenCalledWith("/tasks/U6");
    expect(palette.open).toBe(false);
  });

  it("opens the result in a new tab with the modifier", async () => {
    setup();
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: "sh-17" } });
    await screen.findByText("旧 ID SH-17 で一致");
    await fireEvent.keyDown(box(), { key: "Enter", ctrlKey: true, metaKey: true });
    expect(open).toHaveBeenCalledWith(expect.stringContaining("/tasks/U6"), "_blank");
    expect(h.go).not.toHaveBeenCalled();
    open.mockRestore();
  });

  it("goes down to the commands for the task and changes its status in the palette", async () => {
    const { api } = setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: "sh-17" } });
    await screen.findByText("REL-6 に対して");
    await fireEvent.keyDown(box(), { key: "ArrowDown" });
    expect(screen.getByRole("option", { selected: true })).toHaveTextContent("ステータスを変更…");
    await fireEvent.keyDown(box(), { key: "Enter" });
    expect(palette.open).toBe(true);
    expect(screen.getByText("ステータス", { selector: "[data-mode]" })).toBeInTheDocument();
    await fireEvent.click(await screen.findByRole("option", { name: /done/ }));
    await waitFor(() => expect(api.PATCH).toHaveBeenCalled());
    expect(api.PATCH).toHaveBeenCalledWith("/api/tasks/{id}", {
      params: { path: { id: "U6" }, query: { store: "shared" } },
      body: { status: "done" },
    });
    expect(palette.open).toBe(false);
  });

  it("wraps the selection around with the arrow keys", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    await screen.findByRole("combobox");
    await fireEvent.keyDown(box(), { key: "ArrowUp" });
    const options = screen.getAllByRole("option");
    expect(options[options.length - 1]).toHaveAttribute("aria-selected", "true");
  });

  it("shows the message of a query that does not run", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: "foo:bar" } });
    expect(await screen.findByRole("alert")).toHaveTextContent("term 1 (foo:bar): unknown key");
    expect(screen.getByText("クエリ", { selector: "[data-mode]" })).toBeInTheDocument();
  });

  it("offers the commands for the selected task", async () => {
    setup();
    selection.set("U6");
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
    expect(await screen.findByText("REL-6 に対して")).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("option", { name: /子タスクを作る/ }));
    expect(createState.request).toEqual({ kind: "task", parent: "U6" });
  });

  it("does nothing on Enter or the arrows while the IME is composing", async () => {
    const { api } = setup();
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">同期" } });
    await screen.findByText("今すぐ同期する");
    await fireEvent.keyDown(box(), { key: "Enter", isComposing: true });
    await fireEvent.keyDown(box(), { key: "Enter", keyCode: 229 });
    await fireEvent.keyDown(box(), { key: "Escape", isComposing: true });
    expect(api.POST).not.toHaveBeenCalled();
    expect(palette.open).toBe(true);
  });

  it("takes the task of the detail route before the selected row", async () => {
    setup();
    selection.set("OTHER");
    h.route = { name: "task", ulid: "U6" };
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
    expect(await screen.findByText("REL-6 に対して")).toBeInTheDocument();
  });

  it("writes the key of the new tab for the platform", async () => {
    setup();
    const platform = vi.spyOn(navigator, "platform", "get").mockReturnValue("MacIntel");
    render(CommandPalette);
    openPalette();
    expect(await screen.findByText("⌘↵", { selector: "kbd" })).toBeInTheDocument();
    platform.mockReturnValue("Linux x86_64");
    closePalette();
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    openPalette();
    expect(await screen.findByText("Ctrl+↵", { selector: "kbd" })).toBeInTheDocument();
    platform.mockRestore();
  });

  it("says which stores failed to sync, and does not start a second sync while one runs", async () => {
    const { api } = setup();
    let release: (v: unknown) => void = () => {};
    api.POST.mockImplementation(
      () =>
        new Promise((r) => {
          release = r;
        }) as never,
    );
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">同期" } });
    await fireEvent.keyDown(await screen.findByRole("combobox"), { key: "Enter" });
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">同期" } });
    await fireEvent.keyDown(box(), { key: "Enter" });
    expect(api.POST).toHaveBeenCalledTimes(1);
    release(
      ok([
        { store: "shared", ok: true },
        { store: "backend", ok: false, error: "boom" },
      ]),
    );
    await waitFor(() => expect(h.toast).toHaveBeenCalled());
    expect(h.toast.mock.calls[0]?.[0].message).toContain("backend");
  });

  it("goes back to the first item when Esc leaves the status list", async () => {
    setup();
    selection.set("U6");
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
    await fireEvent.click(await screen.findByRole("option", { name: /ステータスを変更/ }));
    await screen.findByText("ステータス", { selector: "[data-mode]" });
    await fireEvent.keyDown(box(), { key: "ArrowDown" });
    await fireEvent.keyDown(box(), { key: "Escape" });
    expect(await screen.findByText("ID", { selector: "[data-mode]" })).toBeInTheDocument();
    expect(screen.getAllByRole("option")[0]).toHaveAttribute("aria-selected", "true");
  });

  it("matches a status name in any case", async () => {
    setup();
    selection.set("U6");
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
    await fireEvent.click(await screen.findByRole("option", { name: /ステータスを変更/ }));
    await fireEvent.input(box(), { target: { value: "DOI" } });
    expect(await screen.findByRole("option", { name: /doing/ })).toBeInTheDocument();
  });

  it("lists the statuses of a task whose store schema is not read yet, once it is", async () => {
    setup();
    const core = h.core as { schema: { get: unknown; load: unknown; subscribe: unknown } };
    const subs = new Set<() => void>();
    let loaded = false;
    core.schema.get = () => (loaded ? schema : undefined);
    core.schema.subscribe = (fn: () => void) => {
      subs.add(fn);
      return () => subs.delete(fn);
    };
    const load = vi.fn(async () => {
      loaded = true;
      for (const fn of subs) fn();
      return schema;
    });
    core.schema.load = load;
    selection.set("U6");
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
    await fireEvent.click(await screen.findByRole("option", { name: /ステータスを変更/ }));
    expect(load).toHaveBeenCalledWith("shared");
    expect(await screen.findByRole("option", { name: /doing/ })).toBeInTheDocument();
  });

  it("says so when the schema of the store cannot be read", async () => {
    setup();
    const core = h.core as { schema: { get: unknown; load: unknown } };
    core.schema.get = () => undefined;
    core.schema.load = async () => {
      throw new Error("down");
    };
    selection.set("U6");
    render(CommandPalette);
    openPalette();
    await fireEvent.input(await screen.findByRole("combobox"), { target: { value: ">" } });
    await fireEvent.click(await screen.findByRole("option", { name: /ステータスを変更/ }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });

  it("keeps Tab inside the palette", async () => {
    setup();
    render(CommandPalette);
    openPalette();
    const input = await screen.findByRole("combobox");
    const e = new KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true });
    input.dispatchEvent(e);
    expect(e.defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(input);
  });

  it("fetches the recent tasks the cache lacks and drops one that is gone", async () => {
    const { api } = setup();
    const gone = {
      error: { type: "task.not_found", title: "NF", status: 404 },
      response: new Response(null, { status: 404 }),
    };
    const u7 = task("U7", "task", null, { id: "REL-7", title: "あとから届く" });
    api.GET.mockImplementation((async (
      path: string,
      opts: { params: { path?: { id: string } } },
    ) => {
      if (path === "/api/tasks/{id}") return opts.params.path?.id === "U7" ? ok(u7) : gone;
      return ok({ tasks: [] });
    }) as never);
    const core = h.core as { cache: { task: (u: string) => unknown; prime: unknown } };
    const known = core.cache.task;
    const store = new Map<string, unknown>();
    core.cache.task = (u: string) => store.get(u) ?? known(u);
    core.cache.prime = vi.fn((tasks: { ulid: string }[]) => {
      for (const t of tasks) store.set(t.ulid, t);
    });
    localStorage.setItem("hedgehug.recent", JSON.stringify(["U6", "U7", "U8"]));
    render(CommandPalette);
    openPalette();
    expect(await screen.findByText("あとから届く")).toBeInTheDocument();
    expect(api.GET).toHaveBeenCalledWith("/api/tasks/{id}", { params: { path: { id: "U7" } } });
    expect(api.GET).toHaveBeenCalledWith("/api/tasks/{id}", { params: { path: { id: "U8" } } });
    await waitFor(() =>
      expect(JSON.parse(localStorage.getItem("hedgehug.recent") ?? "[]")).toEqual(["U6", "U7"]),
    );
  });
});
