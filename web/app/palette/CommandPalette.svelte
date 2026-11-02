<script lang="ts">
  import { tick, untrack } from "svelte";
  import type { ShowView, TaskView } from "../../core/api/client";
  import { errorMessage } from "../../core/messages";
  import { core } from "../context";
  import { openCreate } from "../create-state.svelte";
  import Icon from "../icons/Icon.svelte";
  import KindIcon from "../icons/KindIcon.svelte";
  import { kbdLabel, modKey } from "../keys";
  import { closePalette, palette } from "../palette-state.svelte";
  import { router } from "../router";
  import { selection } from "../selection.svelte";
  import { runSync } from "../sidebar/sync-run.svelte";
  import { useSubscribe } from "../svelte-store";
  import { toast } from "../toast.svelte";
  import { patchTask } from "../tree/actions";
  import StoreBadge from "../ui/StoreBadge.svelte";
  import { fetchMissing, readRecent, rememberTask } from "./recent";
  import {
    buildCommands,
    type CommandEnv,
    detectMode,
    type PaletteGroup,
    type PaletteItem,
    searchTasks,
    taskCommands,
  } from "./search";

  const c = core();
  const cacheRev = useSubscribe(c.cache, () => ({}));
  const storesRev = useSubscribe(c.stores, () => ({}));
  const schemaRev = useSubscribe(c.schema, () => ({}));

  let input = $state("");
  let active = $state(0);
  let picking = $state<TaskView | null>(null);
  let found = $state<{ groups: PaletteGroup[]; error: unknown }>({ groups: [], error: null });
  let inputEl = $state<HTMLInputElement | null>(null);
  let listEl = $state<HTMLDivElement | null>(null);
  // Why the statuses of the task picked could not be listed (the schema of its store did not load).
  let pickError = $state<unknown>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let seq = 0;

  // The tasks opened last, for the empty palette. The sidebar owns the shortcut that opens it.
  let recentUlids = $state(readRecent());
  $effect(() => {
    if (router.route.name !== "task") return;
    rememberTask(router.route.ulid);
    recentUlids = readRecent();
  });

  // Opening the palette fills the list up to eight: what the cache lacks is fetched.
  $effect(() => {
    if (!palette.open) return;
    untrack(() => {
      recentUlids = readRecent();
      void fetchMissing(
        {
          api: c.api,
          has: (u) => !!c.cache.task(u),
          epoch: () => c.cache.epochNow(),
          prime: (tasks, since) => c.cache.prime(tasks, since),
        },
        recentUlids,
      ).then(() => {
        recentUlids = readRecent();
      });
    });
  });

  // Start from an empty palette each time it opens.
  $effect(() => {
    if (!palette.open) return;
    input = "";
    active = 0;
    picking = null;
    found = { groups: [], error: null };
    void tick().then(() => inputEl?.focus());
  });

  const mode = $derived(detectMode(input));
  const modeLabel = $derived(
    picking ? "ステータス" : { id: "ID", query: "クエリ", command: "コマンド" }[mode],
  );

  const deps = {
    api: c.api,
    hrefOf: (ulid: string) => router.href({ name: "task", ulid }),
    epoch: () => c.cache.epochNow(),
    prime: (tasks: (TaskView | ShowView)[], since: number) => c.cache.prime(tasks, since),
  };

  $effect(() => {
    const text = input;
    const m = mode;
    clearTimeout(timer);
    seq++;
    if (!palette.open || picking || m === "command" || text.trim() === "") {
      found = { groups: [], error: null };
      return;
    }
    const mine = seq;
    timer = setTimeout(async () => {
      const r = await searchTasks(deps, text, m);
      if (mine === seq) found = r;
    }, 150);
    return () => clearTimeout(timer);
  });

  async function setStatus(t: TaskView, status: string) {
    try {
      await patchTask(c, t, { status });
    } catch (e) {
      toast({ message: errorMessage(e) });
    }
  }

  function pickStatus(t: TaskView) {
    picking = t;
    pickError = null;
    input = "";
    active = 0;
    // The palette finds tasks of every store; the schema of this one may not be read yet.
    c.schema.load(t.store).catch((e: unknown) => {
      if (picking?.ulid === t.ulid) pickError = e;
    });
  }

  const env = $derived.by<CommandEnv>(() => {
    cacheRev.current;
    storesRev.current;
    const sel = router.route.name === "task" ? router.route.ulid : selection.ulid;
    return {
      syncNow: () => void runSync(c.api),
      createTask: (parent) => openCreate({ kind: "task", parent }),
      createProject: () => openCreate({ kind: "project" }),
      workspaces: c.stores.workspaces(),
      go: (path) => router.go(path),
      hrefs: {
        workspace: (store, ulid) =>
          router.href({ name: "workspace", store, ulid, tab: "overview" }),
        project: (ulid) => router.href({ name: "project", ulid, tab: "tree" }),
        task: (ulid) => router.href({ name: "task", ulid }),
      },
      selected: sel ? (c.cache.task(sel) ?? null) : null,
      recent: recentUlids.flatMap((u) => c.cache.task(u) ?? []),
      pickStatus,
    };
  });

  function statusGroups(t: TaskView): PaletteGroup[] {
    const word = input.trim().toLowerCase();
    const items = (c.schema.get(t.store)?.statuses ?? [])
      .filter(
        (s) =>
          word === "" ||
          (s.label || s.name).toLowerCase().includes(word) ||
          s.name.toLowerCase().includes(word),
      )
      .map((s) => ({
        type: "command" as const,
        key: `status:${s.name}`,
        label: s.label && s.label !== s.name ? `${s.label}（${s.name}）` : s.name,
        icon: "status" as const,
        note: s.name === t.status ? "今のステータス" : undefined,
        run: () => void setStatus(t, s.name),
      }));
    return items.length ? [{ name: `${t.id} のステータス`, items }] : [];
  }

  const groups = $derived.by<PaletteGroup[]>(() => {
    cacheRev.current;
    schemaRev.current;
    if (picking) return statusGroups(picking);
    if (mode === "command" || input.trim() === "") return buildCommands(env, input);
    const out = [...found.groups];
    const first = found.groups.flatMap((g) => g.items).find((i) => i.type === "task");
    if (mode === "id" && first?.type === "task") {
      const t = c.cache.task(first.ulid);
      if (t) out.push(taskCommands(env, t));
    }
    return out;
  });

  const view = $derived.by(() => {
    let n = 0;
    return groups.map((g) => ({
      name: g.name,
      items: g.items.map((item) => ({ item, index: n++ })),
    }));
  });
  const flat = $derived(groups.flatMap((g) => g.items));
  const activeKey = $derived(flat[active]?.key);
  const error = $derived(picking ? pickError : found.error);
  const waitingSchema = $derived.by(() => {
    schemaRev.current; // the schema of the store arrives later than the pick
    return picking !== null && !c.schema.get(picking.store) && !pickError;
  });
  const empty = $derived(
    flat.length === 0 && (picking !== null || input.trim() !== "") && !error && !waitingSchema,
  );

  $effect(() => {
    if (active >= flat.length) active = Math.max(0, flat.length - 1);
  });

  function activate(it: PaletteItem, newTab: boolean) {
    if (newTab && it.href) {
      // No noopener: the new tab gets a copy of sessionStorage, which holds the token.
      window.open(new URL(it.href, window.location.origin).href, "_blank");
      closePalette();
      return;
    }
    if (it.type === "task") {
      closePalette();
      router.go(it.href);
      return;
    }
    if (!it.keepOpen) closePalette();
    it.run();
  }

  // The input is the only thing to focus in the palette: Tab keeps the focus there.
  function trapTab(e: KeyboardEvent) {
    if (e.key !== "Tab") return;
    e.preventDefault();
    inputEl?.focus();
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.isComposing || e.keyCode === 229) return;
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (flat.length === 0) return;
      active = (active + (e.key === "ArrowDown" ? 1 : -1) + flat.length) % flat.length;
      void tick().then(() =>
        listEl?.querySelector('[aria-selected="true"]')?.scrollIntoView?.({ block: "nearest" }),
      );
    } else if (e.key === "Enter") {
      e.preventDefault();
      const it = flat[active];
      if (it) activate(it, modKey(e));
    } else if (e.key === "Escape") {
      e.preventDefault();
      if (picking) {
        picking = null;
        input = "";
        active = 0;
      } else {
        closePalette();
      }
    }
  }
</script>

{#if palette.open}
  <!-- biome-ignore lint/a11y/noStaticElementInteractions: a click on the backdrop closes the dialog; the keyboard closes it with Esc -->
  <div
    class="backdrop"
    role="presentation"
    onmousedown={(e) => {
      if (e.target === e.currentTarget) closePalette();
    }}
  >
    <div
      class="panel"
      data-palette
      role="dialog"
      aria-modal="true"
      aria-label="ID・検索・コマンド"
      tabindex="-1"
      onkeydown={trapTab}
    >
      <div class="head">
        <Icon name="search" size={16} />
        <input
          bind:this={inputEl}
          type="text"
          role="combobox"
          aria-expanded="true"
          aria-controls="palette-list"
          aria-activedescendant={activeKey ? `palette-${activeKey}` : undefined}
          aria-label="ID・検索・コマンド"
          placeholder="ID・検索・コマンド"
          spellcheck="false"
          autocomplete="off"
          value={input}
          oninput={(e) => {
            input = e.currentTarget.value;
            active = 0;
          }}
          {onkeydown}
        >
        <span class="mode" data-mode={mode}>{modeLabel}</span>
      </div>

      <div class="list" id="palette-list" role="listbox" bind:this={listEl}>
        {#each view as g (g.name)}
          <!-- biome-ignore lint/a11y/useSemanticElements: a group of options in a listbox is a div with role=group -->
          <div class="group" role="group" aria-label={g.name}>
            <div class="gname">{g.name}</div>
            {#each g.items as { item, index } (item.key)}
              <!-- svelte-ignore a11y_click_events_have_key_events -->
              <!-- biome-ignore lint/a11y/useKeyWithClickEvents: the keys are handled by the input, which keeps the focus -->
              <div
                class="item"
                class:active={index === active}
                id="palette-{item.key}"
                role="option"
                aria-selected={index === active}
                tabindex="-1"
                onclick={() => activate(item, false)}
                onmousemove={() => (active = index)}
              >
                {#if item.type === "task"}
                  <KindIcon kind={item.kind} />
                  <span class="key">{item.id}</span>
                  <span class="text">{item.title}</span>
                  {#if item.note}
                    <span class="note">{item.note}</span>
                  {/if}
                  <span class="store"
                    ><StoreBadge store={item.store} tone={c.stores.tone(item.store)} /></span
                  >
                {:else}
                  <Icon name={item.icon} size={14} />
                  <span class="text">{item.label}</span>
                  {#if item.note}
                    <span class="note">{item.note}</span>
                  {/if}
                  {#if item.store}
                    <span class="store"
                      ><StoreBadge store={item.store} tone={c.stores.tone(item.store)} /></span
                    >
                  {/if}
                  {#if item.kbd}
                    <kbd>{item.kbd}</kbd>
                  {/if}
                {/if}
              </div>
            {/each}
          </div>
        {/each}
        {#if error}
          <p class="msg error" role="alert">
            {errorMessage(error)}
          </p>
        {:else if waitingSchema}
          <p class="msg">読み込み中…</p>
        {:else if empty}
          <p class="msg">一致するものがありません</p>
        {/if}
      </div>

      <footer class="foot">
        <span><kbd>↑↓</kbd> 選ぶ</span>
        <span><kbd>↵</kbd> 開く</span>
        <span><kbd>{kbdLabel("mod+enter")}</kbd> 新しいタブで</span>
        <span><kbd>&gt;</kbd> で始めるとコマンドだけ</span>
        <span class="hint">ID・旧 ID・ULID・タイトル・クエリのどれでも</span>
      </footer>
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding-top: 12vh;
    background: var(--scrim);
  }
  .panel {
    display: flex;
    flex-direction: column;
    width: min(38.75rem, calc(100vw - 2rem));
    max-height: 70vh;
    border: var(--hairline) solid var(--line);
    border-radius: var(--radius-8);
    background: var(--surface);
    box-shadow: var(--shadow-popover);
    overflow: hidden;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    padding: 0 0.875rem;
    border-bottom: var(--hairline) solid var(--line);
    color: var(--muted);
  }
  .head input {
    flex: 1;
    min-width: 0;
    height: 3rem;
    border: 0;
    outline: 0;
    background: transparent;
    color: var(--ink);
    font: inherit;
    font-size: 0.95rem;
  }
  .mode {
    flex: 0 0 auto;
    padding: 0.125rem 0.5rem;
    border-radius: var(--radius-pill);
    background: var(--hover);
    color: var(--ink-2);
    font-size: 0.72rem;
    font-weight: 500;
  }
  .mode[data-mode="query"] {
    background: var(--accent-weak);
    color: var(--accent-ink);
  }
  .mode[data-mode="command"] {
    background: var(--ink);
    color: var(--surface);
  }
  .list {
    flex: 1 1 auto;
    min-height: 0;
    overflow: auto;
    padding: 0.375rem;
  }
  .gname {
    padding: 0.5rem 0.625rem 0.25rem;
    color: var(--muted);
    font-size: 0.72rem;
    font-weight: 600;
  }
  .item {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    min-height: 2.25rem;
    padding: 0 0.625rem;
    border-radius: var(--radius-4);
    color: var(--ink);
    cursor: pointer;
  }
  .item.active {
    background: var(--accent-weak);
    box-shadow: inset 0 0 0 var(--hairline) var(--accent);
  }
  .key {
    flex: 0 0 auto;
    font-family: var(--font-mono);
    font-size: 0.72rem;
    color: var(--muted);
  }
  .text {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .note {
    flex: 0 0 auto;
    font-size: 0.72rem;
    color: var(--muted);
  }
  .store {
    flex: 0 0 auto;
  }
  kbd {
    flex: 0 0 auto;
    padding: 0 0.3rem;
    border: var(--hairline) solid var(--line);
    border-radius: var(--radius-4);
    background: var(--sunken);
    font-family: var(--font-mono);
    font-size: 0.7rem;
    color: var(--ink-2);
  }
  .msg {
    margin: 0.75rem 0.625rem;
    color: var(--muted);
    font-size: 0.82rem;
  }
  .msg.error {
    color: var(--danger);
  }
  .foot {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem 1rem;
    padding: 0.5rem 0.875rem;
    border-top: var(--hairline) solid var(--line);
    color: var(--muted);
    font-size: 0.72rem;
  }
  .hint {
    margin-left: auto;
  }
</style>
